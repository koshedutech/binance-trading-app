// Package database provides Redis-based position state persistence.
// Phase 2 of Story 9.6: Shared Redis Infrastructure - Active/Standby Container Control
//
// This repository stores Ginie position state in Redis for sharing between
// dev and prod containers. When Redis is unavailable, it falls back to
// an in-memory cache to ensure trading continues without interruption.
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key prefixes for position state
const (
	// PositionKeyPrefix is the prefix for individual position state keys
	// Format: ginie:position:{userID}:{symbol}
	PositionKeyPrefix = "ginie:position"

	// PositionListKeyPrefix is the prefix for the list of active positions per user
	// Format: ginie:positions:{userID}:list
	PositionListKeyPrefix = "ginie:positions"

	// PositionStateTTL is the TTL for position state keys (7 days)
	// Positions typically close within hours/days, but we keep state longer for safety
	PositionStateTTL = 7 * 24 * time.Hour
)

// PersistedPositionState stores critical state that must survive restarts.
// This is imported from autopilot package to avoid circular dependency.
// Note: The actual struct is defined in internal/autopilot/ginie_autopilot.go
// We define a compatible struct here to avoid import cycles.
type PersistedPositionState struct {
	Symbol         string                 `json:"symbol"`
	Side           string                 `json:"side"`
	Mode           string                 `json:"mode"` // GinieTradingMode as string
	CurrentTPLevel int                    `json:"current_tp_level"`
	ScalpReentry   *ScalpReentryStateData `json:"scalp_reentry,omitempty"` // Full scalp_reentry state
	SavedAt        time.Time              `json:"saved_at"`
}

// ScalpReentryStateData is a simplified version of ScalpReentryStatus for persistence.
// Contains only the essential fields needed for state restoration.
type ScalpReentryStateData struct {
	Enabled           bool                `json:"enabled"`
	CurrentCycle      int                 `json:"current_cycle"`
	Cycles            []ReentryCycleData  `json:"cycles"`
	AccumulatedProfit float64             `json:"accumulated_profit"`
	TPLevelUnlocked   int                 `json:"tp_level_unlocked"`
	NextTPBlocked     bool                `json:"next_tp_blocked"`
	OriginalEntryPrice float64            `json:"original_entry_price"`
	CurrentBreakeven  float64             `json:"current_breakeven"`
	RemainingQuantity float64             `json:"remaining_quantity"`
	DynamicSLActive   bool                `json:"dynamic_sl_active"`
	DynamicSLPrice    float64             `json:"dynamic_sl_price"`
	ProtectedProfit   float64             `json:"protected_profit"`
}

// ReentryCycleData represents a single reentry cycle for persistence.
type ReentryCycleData struct {
	CycleNumber      int       `json:"cycle_number"`
	EntryPrice       float64   `json:"entry_price"`
	EntryQuantity    float64   `json:"entry_quantity"`
	TPHitPrice       float64   `json:"tp_hit_price,omitempty"`
	TPHitQuantity    float64   `json:"tp_hit_quantity,omitempty"`
	RealizedProfit   float64   `json:"realized_profit"`
	ReentryTriggered bool      `json:"reentry_triggered"`
	ReentryPrice     float64   `json:"reentry_price,omitempty"`
	ReentryQuantity  float64   `json:"reentry_quantity,omitempty"`
	CycleStartTime   time.Time `json:"cycle_start_time"`
	CycleEndTime     time.Time `json:"cycle_end_time,omitempty"`
}

// RedisPositionStateRepository provides Redis-based storage for position state
// with an in-memory fallback cache when Redis is unavailable.
type RedisPositionStateRepository struct {
	client         *redis.Client
	inMemoryCache  map[string]*PersistedPositionState // Fallback cache: key = "{userID}:{symbol}"
	cacheMu        sync.RWMutex
	redisAvailable atomic.Bool
}

// NewRedisPositionStateRepository creates a new RedisPositionStateRepository.
// If client is nil, the repository operates in memory-only mode.
func NewRedisPositionStateRepository(client *redis.Client) *RedisPositionStateRepository {
	repo := &RedisPositionStateRepository{
		client:        client,
		inMemoryCache: make(map[string]*PersistedPositionState),
	}

	// Check initial Redis availability
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			log.Printf("[REDIS-POSITION] Redis unavailable at startup: %v, using in-memory cache", err)
			repo.redisAvailable.Store(false)
		} else {
			log.Printf("[REDIS-POSITION] Redis connected successfully")
			repo.redisAvailable.Store(true)
		}
	} else {
		log.Printf("[REDIS-POSITION] No Redis client provided, using in-memory cache only")
		repo.redisAvailable.Store(false)
	}

	return repo
}

// GetClient returns the underlying Redis client.
// Returns nil if operating in memory-only mode.
func (r *RedisPositionStateRepository) GetClient() *redis.Client {
	return r.client
}

// positionKey generates the Redis key for a position state.
// Format: ginie:position:{userID}:{symbol}
func (r *RedisPositionStateRepository) positionKey(userID, symbol string) string {
	return fmt.Sprintf("%s:%s:%s", PositionKeyPrefix, userID, symbol)
}

// positionListKey generates the Redis key for the position list.
// Format: ginie:positions:{userID}:list
func (r *RedisPositionStateRepository) positionListKey(userID string) string {
	return fmt.Sprintf("%s:%s:list", PositionListKeyPrefix, userID)
}

// cacheKey generates the in-memory cache key.
func (r *RedisPositionStateRepository) cacheKey(userID, symbol string) string {
	return fmt.Sprintf("%s:%s", userID, symbol)
}

// SavePositionState saves position state to Redis with fallback to in-memory cache.
// This is called after TP hits, state changes, or periodically during position monitoring.
func (r *RedisPositionStateRepository) SavePositionState(ctx context.Context, userID, symbol string, state *PersistedPositionState) error {
	if state == nil {
		return fmt.Errorf("cannot save nil position state")
	}

	// Ensure SavedAt is set
	state.SavedAt = time.Now()

	// Marshal state to JSON
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal position state: %w", err)
	}

	// Always update in-memory cache
	r.updateCache(userID, symbol, state)

	// Try to save to Redis
	if r.client != nil && r.redisAvailable.Load() {
		key := r.positionKey(userID, symbol)
		listKey := r.positionListKey(userID)

		// Use pipeline for atomic updates
		pipe := r.client.TxPipeline()
		pipe.Set(ctx, key, data, PositionStateTTL)
		pipe.SAdd(ctx, listKey, symbol)
		pipe.Expire(ctx, listKey, PositionStateTTL)

		_, err = pipe.Exec(ctx)
		if err != nil {
			log.Printf("[REDIS-POSITION] Failed to save to Redis: %v, using in-memory cache", err)
			r.redisAvailable.Store(false)
			// Don't return error - in-memory cache is already updated
			return nil
		}

		log.Printf("[REDIS-POSITION] Saved position state: %s/%s (mode=%s, tp_level=%d)",
			userID, symbol, state.Mode, state.CurrentTPLevel)
	} else {
		log.Printf("[REDIS-POSITION] Redis unavailable, saved to in-memory cache: %s/%s", userID, symbol)
	}

	return nil
}

// LoadPositionState loads position state from Redis with fallback to in-memory cache.
// Returns nil if the position doesn't exist (not an error).
func (r *RedisPositionStateRepository) LoadPositionState(ctx context.Context, userID, symbol string) (*PersistedPositionState, error) {
	// Try Redis first if available
	if r.client != nil && r.redisAvailable.Load() {
		key := r.positionKey(userID, symbol)
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				// Position doesn't exist in Redis, try in-memory cache
				return r.getFromCache(userID, symbol), nil
			}
			// Redis error - mark unavailable and fall back to cache
			log.Printf("[REDIS-POSITION] Redis read error: %v, using in-memory cache", err)
			r.redisAvailable.Store(false)
			return r.getFromCache(userID, symbol), nil
		}

		// Mark Redis as available (recovered)
		r.redisAvailable.Store(true)

		// Parse state
		var state PersistedPositionState
		if err := json.Unmarshal([]byte(data), &state); err != nil {
			return nil, fmt.Errorf("failed to unmarshal position state: %w", err)
		}

		// Update in-memory cache with Redis data
		r.updateCache(userID, symbol, &state)

		return &state, nil
	}

	// Redis unavailable - use in-memory cache
	return r.getFromCache(userID, symbol), nil
}

// LoadAllPositions loads all position states for a user from Redis.
// Returns an empty map if no positions exist (not an error).
func (r *RedisPositionStateRepository) LoadAllPositions(ctx context.Context, userID string) (map[string]*PersistedPositionState, error) {
	positions := make(map[string]*PersistedPositionState)

	// Try Redis first if available
	if r.client != nil && r.redisAvailable.Load() {
		listKey := r.positionListKey(userID)

		// Get all symbols from the set
		symbols, err := r.client.SMembers(ctx, listKey).Result()
		if err != nil {
			if err == redis.Nil {
				// No positions exist
				return r.getAllFromCache(userID), nil
			}
			// Redis error - mark unavailable and fall back to cache
			log.Printf("[REDIS-POSITION] Redis read error: %v, using in-memory cache", err)
			r.redisAvailable.Store(false)
			return r.getAllFromCache(userID), nil
		}

		// Mark Redis as available
		r.redisAvailable.Store(true)

		// Load each position
		for _, symbol := range symbols {
			state, err := r.LoadPositionState(ctx, userID, symbol)
			if err != nil {
				log.Printf("[REDIS-POSITION] Failed to load position %s/%s: %v", userID, symbol, err)
				continue
			}
			if state != nil {
				positions[symbol] = state
			}
		}

		if len(positions) > 0 {
			log.Printf("[REDIS-POSITION] Loaded %d positions for user %s from Redis", len(positions), userID)
		}

		return positions, nil
	}

	// Redis unavailable - use in-memory cache
	return r.getAllFromCache(userID), nil
}

// DeletePosition removes position state from Redis and in-memory cache.
// Called when a position is fully closed.
func (r *RedisPositionStateRepository) DeletePosition(ctx context.Context, userID, symbol string) error {
	// Remove from in-memory cache
	r.removeFromCache(userID, symbol)

	// Remove from Redis if available
	if r.client != nil && r.redisAvailable.Load() {
		key := r.positionKey(userID, symbol)
		listKey := r.positionListKey(userID)

		// Use pipeline for atomic updates
		pipe := r.client.TxPipeline()
		pipe.Del(ctx, key)
		pipe.SRem(ctx, listKey, symbol)

		_, err := pipe.Exec(ctx)
		if err != nil {
			log.Printf("[REDIS-POSITION] Failed to delete from Redis: %v", err)
			r.redisAvailable.Store(false)
			// Don't return error - in-memory cache is already updated
			return nil
		}

		log.Printf("[REDIS-POSITION] Deleted position: %s/%s", userID, symbol)
	}

	return nil
}

// IsRedisAvailable returns whether Redis is currently available.
// Useful for monitoring and debugging.
func (r *RedisPositionStateRepository) IsRedisAvailable() bool {
	return r.redisAvailable.Load()
}

// CheckRedisConnection performs a health check and updates availability status.
func (r *RedisPositionStateRepository) CheckRedisConnection(ctx context.Context) error {
	if r.client == nil {
		return fmt.Errorf("no Redis client configured")
	}

	err := r.client.Ping(ctx).Err()
	if err != nil {
		r.redisAvailable.Store(false)
		return fmt.Errorf("redis ping failed: %w", err)
	}

	wasUnavailable := !r.redisAvailable.Load()
	r.redisAvailable.Store(true)

	if wasUnavailable {
		log.Printf("[REDIS-POSITION] Redis connection recovered")
	}

	return nil
}

// SyncCacheToRedis syncs all in-memory cached positions to Redis.
// Called when Redis becomes available after being unavailable.
func (r *RedisPositionStateRepository) SyncCacheToRedis(ctx context.Context) error {
	if r.client == nil || !r.redisAvailable.Load() {
		return fmt.Errorf("redis not available for sync")
	}

	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()

	syncCount := 0
	for key, state := range r.inMemoryCache {
		// Parse userID and symbol from cache key
		// Key format: "{userID}:{symbol}"
		var userID, symbol string
		for i := len(key) - 1; i >= 0; i-- {
			if key[i] == ':' {
				userID = key[:i]
				symbol = key[i+1:]
				break
			}
		}

		if userID == "" || symbol == "" {
			log.Printf("[REDIS-POSITION] Invalid cache key format: %s", key)
			continue
		}

		// Save to Redis
		data, err := json.Marshal(state)
		if err != nil {
			log.Printf("[REDIS-POSITION] Failed to marshal state for %s: %v", key, err)
			continue
		}

		redisKey := r.positionKey(userID, symbol)
		listKey := r.positionListKey(userID)

		pipe := r.client.TxPipeline()
		pipe.Set(ctx, redisKey, data, PositionStateTTL)
		pipe.SAdd(ctx, listKey, symbol)
		pipe.Expire(ctx, listKey, PositionStateTTL)

		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("[REDIS-POSITION] Failed to sync %s to Redis: %v", key, err)
			continue
		}

		syncCount++
	}

	if syncCount > 0 {
		log.Printf("[REDIS-POSITION] Synced %d positions from in-memory cache to Redis", syncCount)
	}

	return nil
}

// GetStats returns statistics about the position state repository.
type PositionStateStats struct {
	RedisAvailable    bool `json:"redis_available"`
	InMemoryCacheSize int  `json:"in_memory_cache_size"`
}

func (r *RedisPositionStateRepository) GetStats() PositionStateStats {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()

	return PositionStateStats{
		RedisAvailable:    r.redisAvailable.Load(),
		InMemoryCacheSize: len(r.inMemoryCache),
	}
}

// --- In-memory cache operations ---

// updateCache adds or updates a position in the in-memory cache.
func (r *RedisPositionStateRepository) updateCache(userID, symbol string, state *PersistedPositionState) {
	if state == nil {
		return
	}

	key := r.cacheKey(userID, symbol)

	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	// Deep copy to avoid shared state issues
	stateCopy := *state
	r.inMemoryCache[key] = &stateCopy
}

// getFromCache retrieves a position from the in-memory cache.
func (r *RedisPositionStateRepository) getFromCache(userID, symbol string) *PersistedPositionState {
	key := r.cacheKey(userID, symbol)

	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()

	if state, exists := r.inMemoryCache[key]; exists {
		// Return a copy to avoid shared state issues
		stateCopy := *state
		return &stateCopy
	}

	return nil
}

// getAllFromCache retrieves all positions for a user from the in-memory cache.
func (r *RedisPositionStateRepository) getAllFromCache(userID string) map[string]*PersistedPositionState {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()

	positions := make(map[string]*PersistedPositionState)
	prefix := userID + ":"

	for key, state := range r.inMemoryCache {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			symbol := key[len(prefix):]
			// Return a copy to avoid shared state issues
			stateCopy := *state
			positions[symbol] = &stateCopy
		}
	}

	if len(positions) > 0 {
		log.Printf("[REDIS-POSITION] Loaded %d positions from in-memory cache for user %s", len(positions), userID)
	}

	return positions
}

// removeFromCache removes a position from the in-memory cache.
func (r *RedisPositionStateRepository) removeFromCache(userID, symbol string) {
	key := r.cacheKey(userID, symbol)

	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	delete(r.inMemoryCache, key)
}

// ClearCache clears all entries from the in-memory cache.
// Primarily used for testing.
func (r *RedisPositionStateRepository) ClearCache() {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	r.inMemoryCache = make(map[string]*PersistedPositionState)
}
