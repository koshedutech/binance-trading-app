// Package coinprofiler provides Redis storage for Coin Profiler data.
// Epic 14: Chain Trading System
// Story 14.5: Redis Storage for Coin Profiler data
//
// This file implements the Redis storage layer for CoinData, enabling
// persistence and sharing of real-time coin data between services.
package coinprofiler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key patterns for Coin Profiler data
const (
	// PrefixCoinData is the prefix for current CoinData
	// Full key: coinprofiler:{symbol}:data
	PrefixCoinData = "coinprofiler:%s:data"

	// PrefixTimeframeData is the prefix for TimeframeData
	// Full key: coinprofiler:{symbol}:{timeframe}
	PrefixTimeframeData = "coinprofiler:%s:%s"

	// KeyTrackedSymbols is the key for the set of all tracked symbols
	// Full key: coinprofiler:symbols
	KeyTrackedSymbols = "coinprofiler:symbols"
)

// Default TTL for Coin Profiler data
const (
	// DefaultCoinDataTTL is the default TTL for CoinData (5 minutes)
	DefaultCoinDataTTL = 5 * time.Minute

	// DefaultTimeframeTTL is the default TTL for TimeframeData (5 minutes)
	DefaultTimeframeTTL = 5 * time.Minute
)

// CoinDataStorage defines the interface for storing and retrieving CoinData.
type CoinDataStorage interface {
	// SaveCoinData saves CoinData to Redis with TTL
	SaveCoinData(ctx context.Context, data *CoinData) error

	// GetCoinData retrieves CoinData for a specific symbol
	GetCoinData(ctx context.Context, symbol string) (*CoinData, error)

	// GetAllCoinData retrieves all tracked CoinData
	GetAllCoinData(ctx context.Context) (map[string]*CoinData, error)

	// DeleteCoinData removes CoinData for a symbol
	DeleteCoinData(ctx context.Context, symbol string) error

	// GetTrackedSymbols returns all currently tracked symbols
	GetTrackedSymbols(ctx context.Context) ([]string, error)

	// SaveBatch saves multiple CoinData entries atomically
	SaveBatch(ctx context.Context, data map[string]*CoinData) error

	// GetBatch retrieves multiple CoinData entries
	GetBatch(ctx context.Context, symbols []string) (map[string]*CoinData, error)

	// SaveTimeframeData saves TimeframeData for a symbol
	SaveTimeframeData(ctx context.Context, symbol string, data *TimeframeData) error

	// GetTimeframeData retrieves TimeframeData for a symbol and timeframe
	GetTimeframeData(ctx context.Context, symbol, timeframe string) (*TimeframeData, error)

	// UpdateCoinDataDelta performs a delta update on CoinData (only changed fields)
	UpdateCoinDataDelta(ctx context.Context, symbol string, updates map[string]interface{}) error

	// IsHealthy returns whether the storage is available
	IsHealthy() bool
}

// RedisClient defines the minimal interface needed for Redis operations.
// This allows for dependency injection and avoids import cycles with internal/cache.
type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
	SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	SMembers(ctx context.Context, key string) *redis.StringSliceCmd
	Ping(ctx context.Context) *redis.StatusCmd
}

// PipelineSupport is an optional interface for clients that support pipelining.
// If the client supports this, batch operations will use Redis pipelines for efficiency.
type PipelineSupport interface {
	Pipeline() redis.Pipeliner
}

// RedisCoinDataStorage implements CoinDataStorage using Redis.
type RedisCoinDataStorage struct {
	client  RedisClient
	ttl     time.Duration
	logger  Logger
	healthy bool
}

// Logger interface for the storage layer
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// StorageConfig holds configuration for the Redis storage
type StorageConfig struct {
	TTL time.Duration // TTL for CoinData (default: 5 minutes)
}

// DefaultStorageConfig returns the default storage configuration
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		TTL: DefaultCoinDataTTL,
	}
}

// NewRedisCoinDataStorage creates a new RedisCoinDataStorage instance.
// If config is nil, default configuration will be used.
func NewRedisCoinDataStorage(client RedisClient, config *StorageConfig, logger Logger) *RedisCoinDataStorage {
	if config == nil {
		config = DefaultStorageConfig()
	}

	ttl := config.TTL
	if ttl <= 0 {
		ttl = DefaultCoinDataTTL
	}

	storage := &RedisCoinDataStorage{
		client:  client,
		ttl:     ttl,
		logger:  logger,
		healthy: false,
	}

	// Check health on creation
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err == nil {
			storage.healthy = true
		}
	}

	return storage
}

// CoinDataKey generates the Redis key for CoinData
func CoinDataKey(symbol string) string {
	return fmt.Sprintf(PrefixCoinData, symbol)
}

// TimeframeDataKey generates the Redis key for TimeframeData
func TimeframeDataKey(symbol, timeframe string) string {
	return fmt.Sprintf(PrefixTimeframeData, symbol, timeframe)
}

// IsHealthy returns whether the underlying Redis client is healthy
func (s *RedisCoinDataStorage) IsHealthy() bool {
	if s.client == nil {
		return false
	}

	// Check actual connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := s.client.Ping(ctx).Err(); err != nil {
		s.healthy = false
		return false
	}

	s.healthy = true
	return true
}

// SaveCoinData saves CoinData to Redis.
// It also adds the symbol to the tracked symbols set.
func (s *RedisCoinDataStorage) SaveCoinData(ctx context.Context, data *CoinData) error {
	if s.client == nil {
		s.logWarn("Redis client unavailable, skipping SaveCoinData", "symbol", data.Symbol)
		return fmt.Errorf("redis client unavailable")
	}

	if data == nil || data.Symbol == "" {
		return fmt.Errorf("invalid coin data: nil or empty symbol")
	}

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal coin data: %w", err)
	}

	// Save the coin data with TTL
	key := CoinDataKey(data.Symbol)
	if err := s.client.Set(ctx, key, string(jsonData), s.ttl).Err(); err != nil {
		s.logError("Failed to save coin data", "symbol", data.Symbol, "error", err)
		return fmt.Errorf("failed to save coin data: %w", err)
	}

	// Add symbol to tracked symbols set
	if err := s.addToTrackedSymbols(ctx, data.Symbol); err != nil {
		s.logWarn("Failed to add to tracked symbols", "symbol", data.Symbol, "error", err)
		// Continue - main data was saved
	}

	// Save individual timeframe data if present
	for _, tfData := range data.Timeframes {
		if err := s.SaveTimeframeData(ctx, data.Symbol, tfData); err != nil {
			s.logWarn("Failed to save timeframe data", "symbol", data.Symbol, "timeframe", tfData.Timeframe, "error", err)
			// Continue - don't fail the whole operation
		}
	}

	s.logDebug("Saved coin data", "symbol", data.Symbol)
	return nil
}

// GetCoinData retrieves CoinData for a specific symbol.
// Returns nil, nil if not found (cache miss).
func (s *RedisCoinDataStorage) GetCoinData(ctx context.Context, symbol string) (*CoinData, error) {
	if s.client == nil {
		return nil, fmt.Errorf("redis client unavailable")
	}

	if symbol == "" {
		return nil, fmt.Errorf("empty symbol")
	}

	key := CoinDataKey(symbol)
	data, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		s.logDebug("Cache miss for coin data", "symbol", symbol, "error", err)
		return nil, nil
	}

	var coinData CoinData
	if err := json.Unmarshal([]byte(data), &coinData); err != nil {
		s.logError("Failed to unmarshal coin data", "symbol", symbol, "error", err)
		return nil, fmt.Errorf("failed to unmarshal coin data: %w", err)
	}

	return &coinData, nil
}

// GetAllCoinData retrieves all tracked CoinData.
func (s *RedisCoinDataStorage) GetAllCoinData(ctx context.Context) (map[string]*CoinData, error) {
	if s.client == nil {
		return nil, fmt.Errorf("redis client unavailable")
	}

	// Get all tracked symbols
	symbols, err := s.GetTrackedSymbols(ctx)
	if err != nil {
		return nil, err
	}

	if len(symbols) == 0 {
		return make(map[string]*CoinData), nil
	}

	// Use batch get for efficiency
	return s.GetBatch(ctx, symbols)
}

// DeleteCoinData removes CoinData for a symbol.
// It also removes the symbol from the tracked symbols set.
func (s *RedisCoinDataStorage) DeleteCoinData(ctx context.Context, symbol string) error {
	if s.client == nil {
		s.logWarn("Redis client unavailable, skipping DeleteCoinData", "symbol", symbol)
		return nil
	}

	if symbol == "" {
		return fmt.Errorf("empty symbol")
	}

	// Get existing data to find timeframes to delete
	existingData, _ := s.GetCoinData(ctx, symbol)

	// Delete main coin data
	key := CoinDataKey(symbol)
	if err := s.client.Del(ctx, key).Err(); err != nil {
		s.logWarn("Failed to delete coin data", "symbol", symbol, "error", err)
	}

	// Remove from tracked symbols set
	if err := s.removeFromTrackedSymbols(ctx, symbol); err != nil {
		s.logWarn("Failed to remove from tracked symbols", "symbol", symbol, "error", err)
	}

	// Delete associated timeframe data
	if existingData != nil {
		for _, tfData := range existingData.Timeframes {
			tfKey := TimeframeDataKey(symbol, tfData.Timeframe)
			if err := s.client.Del(ctx, tfKey).Err(); err != nil {
				s.logWarn("Failed to delete timeframe data", "symbol", symbol, "timeframe", tfData.Timeframe, "error", err)
			}
		}
	}

	s.logDebug("Deleted coin data", "symbol", symbol)
	return nil
}

// GetTrackedSymbols returns all currently tracked symbols.
func (s *RedisCoinDataStorage) GetTrackedSymbols(ctx context.Context) ([]string, error) {
	if s.client == nil {
		return nil, fmt.Errorf("redis client unavailable")
	}

	symbols, err := s.client.SMembers(ctx, KeyTrackedSymbols).Result()
	if err != nil {
		s.logError("Failed to get tracked symbols", "error", err)
		return nil, fmt.Errorf("failed to get tracked symbols: %w", err)
	}

	return symbols, nil
}

// SaveBatch saves multiple CoinData entries.
// This is more efficient than saving individually for bulk updates.
// If the client supports pipelining (PipelineSupport interface), it will use Redis pipelines.
// Otherwise, it falls back to individual save operations.
func (s *RedisCoinDataStorage) SaveBatch(ctx context.Context, data map[string]*CoinData) error {
	if s.client == nil {
		return fmt.Errorf("redis client unavailable")
	}

	if len(data) == 0 {
		return nil
	}

	// Check if client supports pipelining
	pipeClient, supportsPipeline := s.client.(PipelineSupport)
	if supportsPipeline {
		return s.saveBatchWithPipeline(ctx, data, pipeClient)
	}

	// Fall back to individual operations
	return s.saveBatchIndividually(ctx, data)
}

// saveBatchWithPipeline saves batch data using Redis pipeline for efficiency.
func (s *RedisCoinDataStorage) saveBatchWithPipeline(ctx context.Context, data map[string]*CoinData, pipeClient PipelineSupport) error {
	pipe := pipeClient.Pipeline()

	symbols := make([]string, 0, len(data))
	for symbol, coinData := range data {
		if coinData == nil || symbol == "" {
			continue
		}

		jsonData, err := json.Marshal(coinData)
		if err != nil {
			s.logWarn("Failed to marshal coin data in batch", "symbol", symbol, "error", err)
			continue
		}

		key := CoinDataKey(symbol)
		pipe.Set(ctx, key, string(jsonData), s.ttl)
		symbols = append(symbols, symbol)

		// Also save timeframe data
		for _, tfData := range coinData.Timeframes {
			tfKey := TimeframeDataKey(symbol, tfData.Timeframe)
			tfJSON, err := json.Marshal(tfData)
			if err != nil {
				continue
			}
			pipe.Set(ctx, tfKey, string(tfJSON), s.ttl)
		}
	}

	// Add all symbols to tracked set
	if len(symbols) > 0 {
		symbolInterfaces := make([]interface{}, len(symbols))
		for i, sym := range symbols {
			symbolInterfaces[i] = sym
		}
		pipe.SAdd(ctx, KeyTrackedSymbols, symbolInterfaces...)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		s.logError("Failed to execute batch save", "error", err)
		return fmt.Errorf("failed to execute batch save: %w", err)
	}

	s.logDebug("Batch saved coin data (pipeline)", "count", len(symbols))
	return nil
}

// saveBatchIndividually saves batch data using individual operations.
// This is used when the client doesn't support pipelining.
func (s *RedisCoinDataStorage) saveBatchIndividually(ctx context.Context, data map[string]*CoinData) error {
	var lastErr error
	count := 0

	for symbol, coinData := range data {
		if coinData == nil || symbol == "" {
			continue
		}

		jsonData, err := json.Marshal(coinData)
		if err != nil {
			s.logWarn("Failed to marshal coin data in batch", "symbol", symbol, "error", err)
			continue
		}

		key := CoinDataKey(symbol)
		if err := s.client.Set(ctx, key, string(jsonData), s.ttl).Err(); err != nil {
			s.logWarn("Failed to save coin data in batch", "symbol", symbol, "error", err)
			lastErr = err
			continue
		}

		// Add to tracked symbols
		if err := s.client.SAdd(ctx, KeyTrackedSymbols, symbol).Err(); err != nil {
			s.logWarn("Failed to add to tracked symbols in batch", "symbol", symbol, "error", err)
		}

		// Save timeframe data
		for _, tfData := range coinData.Timeframes {
			tfKey := TimeframeDataKey(symbol, tfData.Timeframe)
			tfJSON, err := json.Marshal(tfData)
			if err != nil {
				continue
			}
			if err := s.client.Set(ctx, tfKey, string(tfJSON), s.ttl).Err(); err != nil {
				s.logWarn("Failed to save timeframe data in batch", "symbol", symbol, "timeframe", tfData.Timeframe, "error", err)
			}
		}

		count++
	}

	s.logDebug("Batch saved coin data (individual)", "count", count)
	return lastErr
}

// GetBatch retrieves multiple CoinData entries.
func (s *RedisCoinDataStorage) GetBatch(ctx context.Context, symbols []string) (map[string]*CoinData, error) {
	if s.client == nil {
		return nil, fmt.Errorf("redis client unavailable")
	}

	if len(symbols) == 0 {
		return make(map[string]*CoinData), nil
	}

	// Build keys for MGet
	keys := make([]string, len(symbols))
	for i, symbol := range symbols {
		keys[i] = CoinDataKey(symbol)
	}

	values, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		s.logError("Failed to get batch coin data", "error", err)
		return nil, fmt.Errorf("failed to get batch coin data: %w", err)
	}

	result := make(map[string]*CoinData, len(symbols))
	for i, v := range values {
		if v == nil {
			continue
		}

		str, ok := v.(string)
		if !ok {
			continue
		}

		var coinData CoinData
		if err := json.Unmarshal([]byte(str), &coinData); err != nil {
			s.logWarn("Failed to unmarshal coin data in batch", "symbol", symbols[i], "error", err)
			continue
		}

		result[symbols[i]] = &coinData
	}

	return result, nil
}

// SaveTimeframeData saves TimeframeData for a symbol.
func (s *RedisCoinDataStorage) SaveTimeframeData(ctx context.Context, symbol string, data *TimeframeData) error {
	if s.client == nil {
		return fmt.Errorf("redis client unavailable")
	}

	if data == nil || symbol == "" || data.Timeframe == "" {
		return fmt.Errorf("invalid timeframe data")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal timeframe data: %w", err)
	}

	key := TimeframeDataKey(symbol, data.Timeframe)
	if err := s.client.Set(ctx, key, string(jsonData), s.ttl).Err(); err != nil {
		s.logError("Failed to save timeframe data", "symbol", symbol, "timeframe", data.Timeframe, "error", err)
		return fmt.Errorf("failed to save timeframe data: %w", err)
	}

	s.logDebug("Saved timeframe data", "symbol", symbol, "timeframe", data.Timeframe)
	return nil
}

// GetTimeframeData retrieves TimeframeData for a symbol and timeframe.
func (s *RedisCoinDataStorage) GetTimeframeData(ctx context.Context, symbol, timeframe string) (*TimeframeData, error) {
	if s.client == nil {
		return nil, fmt.Errorf("redis client unavailable")
	}

	if symbol == "" || timeframe == "" {
		return nil, fmt.Errorf("empty symbol or timeframe")
	}

	key := TimeframeDataKey(symbol, timeframe)
	data, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, nil
	}

	var tfData TimeframeData
	if err := json.Unmarshal([]byte(data), &tfData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal timeframe data: %w", err)
	}

	return &tfData, nil
}

// UpdateCoinDataDelta performs a delta update on CoinData.
// Only the specified fields are updated, preserving existing data.
// Supported update fields: price, volume_24h, volatility, updated_at
func (s *RedisCoinDataStorage) UpdateCoinDataDelta(ctx context.Context, symbol string, updates map[string]interface{}) error {
	if s.client == nil {
		return fmt.Errorf("redis client unavailable")
	}

	if symbol == "" || len(updates) == 0 {
		return nil
	}

	// Get existing data
	existingData, err := s.GetCoinData(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to get existing data for delta update: %w", err)
	}

	if existingData == nil {
		// No existing data, can't do delta update
		return fmt.Errorf("no existing data for symbol: %s", symbol)
	}

	// Apply updates
	for key, value := range updates {
		switch key {
		case "price":
			if v, ok := value.(float64); ok {
				existingData.Price = v
			}
		case "volume_24h":
			if v, ok := value.(float64); ok {
				existingData.Volume24h = v
			}
		case "volatility":
			if v, ok := value.(float64); ok {
				existingData.Volatility = v
			}
		case "updated_at":
			if v, ok := value.(time.Time); ok {
				existingData.UpdatedAt = v
			}
		}
	}

	// Save updated data
	return s.SaveCoinData(ctx, existingData)
}

// addToTrackedSymbols adds a symbol to the tracked symbols set.
func (s *RedisCoinDataStorage) addToTrackedSymbols(ctx context.Context, symbol string) error {
	return s.client.SAdd(ctx, KeyTrackedSymbols, symbol).Err()
}

// removeFromTrackedSymbols removes a symbol from the tracked symbols set.
func (s *RedisCoinDataStorage) removeFromTrackedSymbols(ctx context.Context, symbol string) error {
	return s.client.SRem(ctx, KeyTrackedSymbols, symbol).Err()
}

// logDebug logs a debug message if logger is available.
func (s *RedisCoinDataStorage) logDebug(msg string, keysAndValues ...interface{}) {
	if s.logger != nil {
		s.logger.Debug(msg, keysAndValues...)
	}
}

// logWarn logs a warning message if logger is available.
func (s *RedisCoinDataStorage) logWarn(msg string, keysAndValues ...interface{}) {
	if s.logger != nil {
		s.logger.Warn(msg, keysAndValues...)
	}
}

// logError logs an error message if logger is available.
func (s *RedisCoinDataStorage) logError(msg string, keysAndValues ...interface{}) {
	if s.logger != nil {
		s.logger.Error(msg, keysAndValues...)
	}
}

// GetTTL returns the current TTL for CoinData.
func (s *RedisCoinDataStorage) GetTTL() time.Duration {
	return s.ttl
}

// SetTTL updates the TTL for CoinData.
func (s *RedisCoinDataStorage) SetTTL(ttl time.Duration) {
	if ttl > 0 {
		s.ttl = ttl
	}
}

// GetClient returns the underlying Redis client.
// This is useful for advanced operations or when integrating with cache.CacheService.
func (s *RedisCoinDataStorage) GetClient() RedisClient {
	return s.client
}
