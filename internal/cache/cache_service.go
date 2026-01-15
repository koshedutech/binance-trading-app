// Package cache provides Redis-based caching for settings and configurations.
// Epic 6: Redis Caching Infrastructure
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"binance-trading-bot/config"

	"github.com/redis/go-redis/v9"
)

// CacheService provides Redis-based caching with graceful degradation.
// When Redis is unavailable, operations return errors that callers should handle
// by falling back to database queries.
type CacheService struct {
	client       *redis.Client
	config       config.RedisConfig
	mu           sync.RWMutex
	healthy      bool
	failureCount int
	lastCheck    time.Time

	// Circuit breaker settings
	maxFailures     int
	checkInterval   time.Duration
	recoveryBackoff time.Duration
}

// Key prefixes for different cache types
const (
	PrefixUserSettings    = "user:%s:settings:all"
	PrefixModeConfig      = "user:%s:mode:%s"
	PrefixAdminDefaults   = "admin:defaults:all"
	PrefixScalpReentry    = "user:%s:scalp_reentry"
	PrefixCircuitBreaker  = "user:%s:circuit_breaker"
	PrefixDailySequence   = "user:%s:sequence:%s" // For Epic 7: clientOrderId sequences
)

// Default TTLs
const (
	DefaultSettingsTTL = 24 * time.Hour
	DefaultSequenceTTL = 48 * time.Hour // 48h for daily sequences (handles timezone edge cases)
)

// NewCacheService creates a new CacheService with the provided configuration.
// It attempts to connect to Redis and verifies connectivity.
func NewCacheService(cfg config.RedisConfig) (*CacheService, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("redis is not enabled in configuration")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: 2,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	cs := &CacheService{
		client:          client,
		config:          cfg,
		healthy:         false,
		failureCount:    0,
		maxFailures:     3,
		checkInterval:   30 * time.Second,
		recoveryBackoff: 5 * time.Second,
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[CACHE] Initial Redis connection failed: %v", err)
		return cs, nil // Return service in degraded mode
	}

	cs.healthy = true
	cs.lastCheck = time.Now()
	log.Printf("[CACHE] Redis connected successfully at %s", cfg.Address)

	return cs, nil
}

// IsHealthy returns whether Redis is currently available.
func (cs *CacheService) IsHealthy() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.healthy
}

// recordFailure tracks a Redis operation failure for circuit breaker.
func (cs *CacheService) recordFailure() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.failureCount++
	if cs.failureCount >= cs.maxFailures {
		if cs.healthy {
			log.Printf("[CACHE] Circuit breaker OPEN: Redis marked unhealthy after %d failures", cs.failureCount)
		}
		cs.healthy = false
	}
}

// recordSuccess resets the failure counter on successful operation.
func (cs *CacheService) recordSuccess() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.healthy {
		log.Printf("[CACHE] Circuit breaker CLOSED: Redis recovered")
	}
	cs.healthy = true
	cs.failureCount = 0
	cs.lastCheck = time.Now()
}

// checkHealth performs a background health check if enough time has passed.
func (cs *CacheService) checkHealth(ctx context.Context) {
	cs.mu.RLock()
	timeSinceCheck := time.Since(cs.lastCheck)
	shouldCheck := !cs.healthy && timeSinceCheck >= cs.checkInterval
	cs.mu.RUnlock()

	if !shouldCheck {
		return
	}

	go func() {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := cs.client.Ping(pingCtx).Err(); err == nil {
			cs.recordSuccess()
		}
	}()
}

// Get retrieves a value from cache.
func (cs *CacheService) Get(ctx context.Context, key string) (string, error) {
	cs.checkHealth(ctx)

	if !cs.IsHealthy() {
		return "", fmt.Errorf("redis unavailable (circuit breaker open)")
	}

	result, err := cs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", err // Cache miss, not a failure
		}
		cs.recordFailure()
		return "", fmt.Errorf("redis get failed: %w", err)
	}

	cs.recordSuccess()
	return result, nil
}

// MGet retrieves multiple keys atomically.
func (cs *CacheService) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	cs.checkHealth(ctx)

	if !cs.IsHealthy() {
		return nil, fmt.Errorf("redis unavailable (circuit breaker open)")
	}

	result, err := cs.client.MGet(ctx, keys...).Result()
	if err != nil {
		cs.recordFailure()
		return nil, fmt.Errorf("redis mget failed: %w", err)
	}

	cs.recordSuccess()
	return result, nil
}

// Set stores a value in cache with TTL.
func (cs *CacheService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	cs.checkHealth(ctx)

	if !cs.IsHealthy() {
		return fmt.Errorf("redis unavailable (circuit breaker open)")
	}

	var data string
	switch v := value.(type) {
	case string:
		data = v
	case []byte:
		data = string(v)
	default:
		jsonData, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		data = string(jsonData)
	}

	if err := cs.client.Set(ctx, key, data, ttl).Err(); err != nil {
		cs.recordFailure()
		return fmt.Errorf("redis set failed: %w", err)
	}

	cs.recordSuccess()
	return nil
}

// Delete removes a key from cache.
func (cs *CacheService) Delete(ctx context.Context, key string) error {
	cs.checkHealth(ctx)

	if !cs.IsHealthy() {
		return fmt.Errorf("redis unavailable (circuit breaker open)")
	}

	if err := cs.client.Del(ctx, key).Err(); err != nil {
		cs.recordFailure()
		return fmt.Errorf("redis delete failed: %w", err)
	}

	cs.recordSuccess()
	return nil
}

// DeletePattern deletes all keys matching a pattern (use with caution).
func (cs *CacheService) DeletePattern(ctx context.Context, pattern string) error {
	cs.checkHealth(ctx)

	if !cs.IsHealthy() {
		return fmt.Errorf("redis unavailable (circuit breaker open)")
	}

	iter := cs.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := cs.client.Del(ctx, iter.Val()).Err(); err != nil {
			cs.recordFailure()
			return fmt.Errorf("redis delete pattern failed: %w", err)
		}
	}

	if err := iter.Err(); err != nil {
		cs.recordFailure()
		return fmt.Errorf("redis scan failed: %w", err)
	}

	cs.recordSuccess()
	return nil
}

// IncrementDailySequence atomically increments a daily sequence counter.
// Used by Epic 7 for clientOrderId generation.
// Returns the new sequence number (1-indexed).
func (cs *CacheService) IncrementDailySequence(ctx context.Context, userID, dateKey string) (int64, error) {
	cs.checkHealth(ctx)

	if !cs.IsHealthy() {
		return 0, fmt.Errorf("redis unavailable (circuit breaker open)")
	}

	key := fmt.Sprintf(PrefixDailySequence, userID, dateKey)

	// INCR is atomic - perfect for sequence generation
	val, err := cs.client.Incr(ctx, key).Result()
	if err != nil {
		cs.recordFailure()
		return 0, fmt.Errorf("redis incr failed: %w", err)
	}

	// Set TTL on first increment (val == 1)
	if val == 1 {
		cs.client.Expire(ctx, key, DefaultSequenceTTL)
	}

	cs.recordSuccess()
	return val, nil
}

// GetCurrentSequence returns the current sequence value for a user on a given date.
// Used for monitoring - doesn't increment the sequence.
func (cs *CacheService) GetCurrentSequence(ctx context.Context, userID, dateKey string) (int64, error) {
	cs.checkHealth(ctx)

	if !cs.IsHealthy() {
		return 0, fmt.Errorf("redis unavailable (circuit breaker open)")
	}

	key := fmt.Sprintf(PrefixDailySequence, userID, dateKey)

	val, err := cs.client.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			// Key not found - no sequences generated yet for this date
			return 0, nil
		}
		cs.recordFailure()
		return 0, fmt.Errorf("redis get sequence failed: %w", err)
	}

	cs.recordSuccess()
	return val, nil
}

// GetJSON retrieves and unmarshals a JSON value from cache.
func (cs *CacheService) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := cs.Get(ctx, key)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return fmt.Errorf("failed to unmarshal cached value: %w", err)
	}

	return nil
}

// SetJSON marshals and stores a JSON value in cache.
func (cs *CacheService) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return cs.Set(ctx, key, value, ttl)
}

// Close closes the Redis connection.
func (cs *CacheService) Close() error {
	if cs.client != nil {
		return cs.client.Close()
	}
	return nil
}

// Ping checks Redis connectivity.
func (cs *CacheService) Ping(ctx context.Context) error {
	if err := cs.client.Ping(ctx).Err(); err != nil {
		cs.recordFailure()
		return err
	}
	cs.recordSuccess()
	return nil
}

// GetClient returns the underlying Redis client for advanced operations.
// Use with caution - prefer using CacheService methods.
func (cs *CacheService) GetClient() *redis.Client {
	return cs.client
}

// Stats returns cache statistics for monitoring.
type Stats struct {
	Healthy      bool   `json:"healthy"`
	FailureCount int    `json:"failure_count"`
	Address      string `json:"address"`
	PoolSize     int    `json:"pool_size"`
}

// GetStats returns current cache statistics.
func (cs *CacheService) GetStats() Stats {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return Stats{
		Healthy:      cs.healthy,
		FailureCount: cs.failureCount,
		Address:      cs.config.Address,
		PoolSize:     cs.config.PoolSize,
	}
}

// UserSettingsKey generates a cache key for user settings.
func UserSettingsKey(userID string) string {
	return fmt.Sprintf(PrefixUserSettings, userID)
}

// ModeConfigKey generates a cache key for mode configuration.
func ModeConfigKey(userID, mode string) string {
	return fmt.Sprintf(PrefixModeConfig, userID, mode)
}

// AdminDefaultsKey returns the cache key for admin defaults.
func AdminDefaultsKey() string {
	return PrefixAdminDefaults
}

// ScalpReentryKey generates a cache key for scalp reentry config.
func ScalpReentryKey(userID string) string {
	return fmt.Sprintf(PrefixScalpReentry, userID)
}

// CircuitBreakerKey generates a cache key for circuit breaker config.
func CircuitBreakerKey(userID string) string {
	return fmt.Sprintf(PrefixCircuitBreaker, userID)
}

// DailySequenceKey generates a cache key for daily order sequences.
func DailySequenceKey(userID, dateStr string) string {
	return fmt.Sprintf(PrefixDailySequence, userID, dateStr)
}
