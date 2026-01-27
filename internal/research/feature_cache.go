// Package research provides data structures and services for research infrastructure.
// Story 15.5: Feature Cache Management - Optimized Redis caching for calculated features
package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"binance-trading-bot/internal/cache"
	"binance-trading-bot/internal/research/indicators"

	"github.com/redis/go-redis/v9"
)

// FeatureVersion is the current version of feature calculations.
// Increment this when the feature calculation logic changes to invalidate old cache entries.
const FeatureVersion = 1

// Cache key prefixes and patterns
const (
	// FeatureCachePrefix is the base prefix for all feature cache keys
	// Format: research:features:v{version}:{symbol}:{timeframe}:{openTimeUnix}
	FeatureCachePrefix = "research:features:v%d:%s:%s:%d"

	// FeatureCachePattern for scanning features by symbol/timeframe
	// Format: research:features:v{version}:{symbol}:{timeframe}:*
	FeatureCachePattern = "research:features:v%d:%s:%s:*"

	// AllFeaturesCachePattern for scanning all features of any version
	AllFeaturesCachePattern = "research:features:*"

	// VersionedFeaturesCachePattern for scanning all features of a specific version
	VersionedFeaturesCachePattern = "research:features:v%d:*"
)

// Default TTLs by timeframe
var DefaultFeatureTTLs = map[Timeframe]time.Duration{
	Timeframe1m:  6 * time.Hour,   // Short-lived for 1m data
	Timeframe5m:  12 * time.Hour,  // Half day for 5m
	Timeframe15m: 24 * time.Hour,  // 1 day for 15m
	Timeframe30m: 48 * time.Hour,  // 2 days for 30m
	Timeframe1h:  72 * time.Hour,  // 3 days for 1h
	Timeframe4h:  168 * time.Hour, // 1 week for 4h
	Timeframe1d:  336 * time.Hour, // 2 weeks for daily
}

// FeatureCacheConfig holds configuration for the feature cache.
type FeatureCacheConfig struct {
	// TTLOverrides allows overriding default TTLs per timeframe
	TTLOverrides map[Timeframe]time.Duration

	// DefaultTTL is used when no timeframe-specific TTL is configured
	DefaultTTL time.Duration

	// Version override (default: FeatureVersion)
	Version int
}

// DefaultFeatureCacheConfig returns the default cache configuration.
func DefaultFeatureCacheConfig() FeatureCacheConfig {
	return FeatureCacheConfig{
		TTLOverrides: make(map[Timeframe]time.Duration),
		DefaultTTL:   24 * time.Hour,
		Version:      FeatureVersion,
	}
}

// FeatureCache provides optimized Redis caching for CandleFeatures.
// Uses Redis HASH storage for efficient partial reads and updates.
type FeatureCache struct {
	cacheService *cache.CacheService
	config       FeatureCacheConfig
}

// NewFeatureCache creates a new FeatureCache instance.
func NewFeatureCache(cacheService *cache.CacheService, config FeatureCacheConfig) *FeatureCache {
	if config.Version == 0 {
		config.Version = FeatureVersion
	}
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 24 * time.Hour
	}

	return &FeatureCache{
		cacheService: cacheService,
		config:       config,
	}
}

// featureCacheKey generates the versioned cache key for a feature.
func (fc *FeatureCache) featureCacheKey(symbol string, timeframe Timeframe, openTime time.Time) string {
	return fmt.Sprintf(FeatureCachePrefix, fc.config.Version, symbol, timeframe, openTime.Unix())
}

// getTTL returns the appropriate TTL for a timeframe.
func (fc *FeatureCache) getTTL(timeframe Timeframe) time.Duration {
	// Check config overrides first
	if ttl, ok := fc.config.TTLOverrides[timeframe]; ok {
		return ttl
	}
	// Check defaults
	if ttl, ok := DefaultFeatureTTLs[timeframe]; ok {
		return ttl
	}
	// Fallback to default
	return fc.config.DefaultTTL
}

// IsHealthy returns whether the cache service is available.
func (fc *FeatureCache) IsHealthy() bool {
	return fc.cacheService != nil && fc.cacheService.IsHealthy()
}

// --- Single Feature Operations ---

// Set stores a CandleFeatures in Redis as a HASH.
func (fc *FeatureCache) Set(ctx context.Context, features *CandleFeatures) error {
	if !fc.IsHealthy() {
		return nil // Graceful degradation
	}

	if features == nil {
		return fmt.Errorf("features cannot be nil")
	}

	key := fc.featureCacheKey(features.Symbol, features.Timeframe, features.OpenTime)
	data := featuresToMap(features, fc.config.Version)
	ttl := fc.getTTL(features.Timeframe)

	client := fc.cacheService.GetClient()
	if client == nil {
		return fmt.Errorf("redis client unavailable")
	}

	// Use pipeline for atomic HSET + EXPIRE
	pipe := client.Pipeline()
	pipe.HSet(ctx, key, data)
	pipe.Expire(ctx, key, ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to cache features: %w", err)
	}

	return nil
}

// Get retrieves a CandleFeatures from Redis HASH.
// Returns nil, nil if not found (cache miss).
func (fc *FeatureCache) Get(ctx context.Context, symbol string, timeframe Timeframe, openTime time.Time) (*CandleFeatures, error) {
	if !fc.IsHealthy() {
		return nil, nil // Graceful degradation
	}

	key := fc.featureCacheKey(symbol, timeframe, openTime)

	client := fc.cacheService.GetClient()
	if client == nil {
		return nil, nil
	}

	result, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get cached features: %w", err)
	}

	// HGetAll returns empty map for non-existent keys
	if len(result) == 0 {
		return nil, nil
	}

	// Check version - REQUIRED field to prevent cache corruption
	versionStr, ok := result["_version"]
	if !ok {
		// Missing version field indicates corrupted or legacy entry - treat as cache miss
		log.Printf("[FEATURE_CACHE] Missing _version field for %s - treating as cache miss", key)
		return nil, nil
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		// Invalid version format - corrupted entry
		log.Printf("[FEATURE_CACHE] Invalid _version format for %s: %s - treating as cache miss", key, versionStr)
		return nil, nil
	}

	if version != fc.config.Version {
		// Version mismatch - treat as cache miss
		log.Printf("[FEATURE_CACHE] Version mismatch for %s: cached=%d, current=%d", key, version, fc.config.Version)
		return nil, nil
	}

	return featuresFromMap(result)
}

// Delete removes a feature from the cache.
func (fc *FeatureCache) Delete(ctx context.Context, symbol string, timeframe Timeframe, openTime time.Time) error {
	if !fc.IsHealthy() {
		return nil
	}

	key := fc.featureCacheKey(symbol, timeframe, openTime)
	return fc.cacheService.Delete(ctx, key)
}

// --- Batch Operations ---

// GetBatch retrieves multiple features for a time range.
// Uses Redis pipeline for efficiency.
// Returns features in chronological order. Missing entries are nil.
func (fc *FeatureCache) GetBatch(ctx context.Context, symbol string, timeframe Timeframe, startTime, endTime time.Time) ([]*CandleFeatures, error) {
	if !fc.IsHealthy() {
		return nil, nil
	}

	// Calculate the expected candle timestamps
	interval := timeframe.Duration()
	if interval == 0 {
		return nil, fmt.Errorf("invalid timeframe: %s", timeframe)
	}

	// Align start time to candle boundary
	startUnix := startTime.Unix()
	intervalSecs := int64(interval.Seconds())
	alignedStart := time.Unix(startUnix-(startUnix%intervalSecs), 0)

	// Generate keys for all candles in range
	var keys []string
	var timestamps []time.Time
	for t := alignedStart; !t.After(endTime); t = t.Add(interval) {
		keys = append(keys, fc.featureCacheKey(symbol, timeframe, t))
		timestamps = append(timestamps, t)
	}

	if len(keys) == 0 {
		return nil, nil
	}

	client := fc.cacheService.GetClient()
	if client == nil {
		return nil, nil
	}

	// Use pipeline for batch retrieval
	pipe := client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get batch features: %w", err)
	}

	// Process results
	results := make([]*CandleFeatures, len(keys))
	hitCount := 0
	for i, cmd := range cmds {
		result, err := cmd.Result()
		if err != nil || len(result) == 0 {
			continue // Cache miss
		}

		// Check version
		if versionStr, ok := result["_version"]; ok {
			version, _ := strconv.Atoi(versionStr)
			if version != fc.config.Version {
				continue // Version mismatch
			}
		}

		features, err := featuresFromMap(result)
		if err != nil {
			log.Printf("[FEATURE_CACHE] Failed to parse cached features at %v: %v", timestamps[i], err)
			continue
		}
		results[i] = features
		hitCount++
	}

	// Log hit rate (guard against division by zero, though len(keys) > 0 is guaranteed here)
	if len(keys) > 0 {
		log.Printf("[FEATURE_CACHE] GetBatch: %s/%s %d/%d hits (%.1f%%)",
			symbol, timeframe, hitCount, len(keys), float64(hitCount)/float64(len(keys))*100)
	}

	return results, nil
}

// SetBatch stores multiple features efficiently using pipeline.
// Batches pipeline execution to avoid memory spikes with large feature sets.
const setBatchChunkSize = 500 // Process 500 features per pipeline execution

func (fc *FeatureCache) SetBatch(ctx context.Context, features []*CandleFeatures) error {
	if !fc.IsHealthy() || len(features) == 0 {
		return nil
	}

	client := fc.cacheService.GetClient()
	if client == nil {
		return fmt.Errorf("redis client unavailable")
	}

	totalCached := 0
	var lastErr error

	// Process in chunks to avoid memory spikes
	for i := 0; i < len(features); i += setBatchChunkSize {
		end := i + setBatchChunkSize
		if end > len(features) {
			end = len(features)
		}
		chunk := features[i:end]

		pipe := client.Pipeline()
		chunkCount := 0

		for _, f := range chunk {
			if f == nil {
				continue
			}
			key := fc.featureCacheKey(f.Symbol, f.Timeframe, f.OpenTime)
			data := featuresToMap(f, fc.config.Version)
			ttl := fc.getTTL(f.Timeframe)

			pipe.HSet(ctx, key, data)
			pipe.Expire(ctx, key, ttl)
			chunkCount++
		}

		if chunkCount > 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				log.Printf("[FEATURE_CACHE] SetBatch chunk error (offset %d): %v", i, err)
				lastErr = err
				// Continue with next chunk instead of failing entirely
			} else {
				totalCached += chunkCount
			}
		}
	}

	log.Printf("[FEATURE_CACHE] SetBatch: cached %d/%d features", totalCached, len(features))

	if lastErr != nil && totalCached == 0 {
		return fmt.Errorf("failed to cache batch features: %w", lastErr)
	}
	return nil
}

// --- Cleanup Operations ---

// DeleteBySymbolTimeframe removes all cached features for a symbol/timeframe.
func (fc *FeatureCache) DeleteBySymbolTimeframe(ctx context.Context, symbol string, timeframe Timeframe) (int64, error) {
	if !fc.IsHealthy() {
		return 0, nil
	}

	pattern := fmt.Sprintf(FeatureCachePattern, fc.config.Version, symbol, timeframe)
	return fc.deleteByPattern(ctx, pattern)
}

// InvalidateOldVersions removes all cached features with version < current.
func (fc *FeatureCache) InvalidateOldVersions(ctx context.Context) (int64, error) {
	if !fc.IsHealthy() {
		return 0, nil
	}

	client := fc.cacheService.GetClient()
	if client == nil {
		return 0, nil
	}

	var totalDeleted int64

	// Scan all feature keys and delete those with old versions
	iter := client.Scan(ctx, 0, AllFeaturesCachePattern, 1000).Iterator()
	var keysToDelete []string

	for iter.Next(ctx) {
		key := iter.Val()
		// Extract version from key using robust parsing
		parts := splitCacheKey(key)
		if parts == nil {
			continue
		}

		if parts.version < fc.config.Version {
			keysToDelete = append(keysToDelete, key)

			// Delete in batches of 100
			if len(keysToDelete) >= 100 {
				deleted, err := client.Del(ctx, keysToDelete...).Result()
				if err != nil {
					log.Printf("[FEATURE_CACHE] Batch delete error: %v", err)
				}
				totalDeleted += deleted
				keysToDelete = keysToDelete[:0]
			}
		}
	}

	// Delete remaining keys
	if len(keysToDelete) > 0 {
		deleted, err := client.Del(ctx, keysToDelete...).Result()
		if err != nil {
			log.Printf("[FEATURE_CACHE] Final batch delete error: %v", err)
		}
		totalDeleted += deleted
	}

	if err := iter.Err(); err != nil {
		return totalDeleted, fmt.Errorf("scan error: %w", err)
	}

	if totalDeleted > 0 {
		log.Printf("[FEATURE_CACHE] Invalidated %d keys with old versions (current: v%d)", totalDeleted, fc.config.Version)
	}

	return totalDeleted, nil
}

// CleanupExpired is a placeholder for manual cleanup.
// Note: Redis handles TTL-based expiration automatically.
// This method can be used for manual cleanup of orphaned data.
func (fc *FeatureCache) CleanupExpired(ctx context.Context) error {
	// Redis automatically handles TTL expiration
	// This method exists for explicit cleanup scenarios if needed
	log.Printf("[FEATURE_CACHE] CleanupExpired: Redis TTL handles expiration automatically")
	return nil
}

// deleteByPattern deletes all keys matching a pattern.
func (fc *FeatureCache) deleteByPattern(ctx context.Context, pattern string) (int64, error) {
	client := fc.cacheService.GetClient()
	if client == nil {
		return 0, nil
	}

	var totalDeleted int64
	iter := client.Scan(ctx, 0, pattern, 100).Iterator()
	var keysToDelete []string

	for iter.Next(ctx) {
		keysToDelete = append(keysToDelete, iter.Val())

		if len(keysToDelete) >= 100 {
			deleted, err := client.Del(ctx, keysToDelete...).Result()
			if err != nil {
				log.Printf("[FEATURE_CACHE] Pattern delete batch error: %v", err)
			}
			totalDeleted += deleted
			keysToDelete = keysToDelete[:0]
		}
	}

	if len(keysToDelete) > 0 {
		deleted, err := client.Del(ctx, keysToDelete...).Result()
		if err != nil {
			log.Printf("[FEATURE_CACHE] Pattern delete final batch error: %v", err)
		}
		totalDeleted += deleted
	}

	if err := iter.Err(); err != nil {
		return totalDeleted, fmt.Errorf("scan error: %w", err)
	}

	return totalDeleted, nil
}

// --- Statistics ---

// CacheStats holds statistics about the feature cache.
type CacheStats struct {
	TotalKeys       int64            `json:"total_keys"`
	KeysByVersion   map[int]int64    `json:"keys_by_version"`
	KeysByTimeframe map[string]int64 `json:"keys_by_timeframe"`
	CurrentVersion  int              `json:"current_version"`
}

// GetStats returns statistics about cached features.
func (fc *FeatureCache) GetStats(ctx context.Context) (*CacheStats, error) {
	if !fc.IsHealthy() {
		return nil, fmt.Errorf("cache unavailable")
	}

	client := fc.cacheService.GetClient()
	if client == nil {
		return nil, fmt.Errorf("redis client unavailable")
	}

	stats := &CacheStats{
		KeysByVersion:   make(map[int]int64),
		KeysByTimeframe: make(map[string]int64),
		CurrentVersion:  fc.config.Version,
	}

	iter := client.Scan(ctx, 0, AllFeaturesCachePattern, 1000).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		stats.TotalKeys++

		// Parse version from key using string splitting for robustness
		// Key format: research:features:v{version}:{symbol}:{timeframe}:{timestamp}
		parts := splitCacheKey(key)
		if parts != nil {
			stats.KeysByVersion[parts.version]++
			stats.KeysByTimeframe[parts.timeframe]++
		}
	}

	return stats, iter.Err()
}

// CountFeatures returns the number of cached features for a specific symbol and timeframe.
// Returns 0 if the cache is unavailable or the scan fails.
// Note: This method is slow for large datasets. Use GetAllFeatureCounts for batch operations.
func (fc *FeatureCache) CountFeatures(ctx context.Context, symbol string, timeframe Timeframe) int64 {
	if !fc.IsHealthy() {
		return 0
	}

	client := fc.cacheService.GetClient()
	if client == nil {
		return 0
	}

	// Use the versioned pattern to only count current version features
	pattern := fmt.Sprintf(FeatureCachePattern, fc.config.Version, symbol, timeframe)

	var count int64
	iter := client.Scan(ctx, 0, pattern, 1000).Iterator()
	for iter.Next(ctx) {
		count++
	}

	return count
}

// HasFeatures checks if there are any cached features for a specific symbol and timeframe.
// Returns true if at least one feature is cached.
func (fc *FeatureCache) HasFeatures(ctx context.Context, symbol string, timeframe Timeframe) bool {
	return fc.CountFeatures(ctx, symbol, timeframe) > 0
}

// FeatureCountResult holds the count for a symbol/timeframe combination
type FeatureCountResult struct {
	Symbol    string
	Timeframe string
	Count     int64
}

// GetAllFeatureCounts returns feature counts for all symbol/timeframe combinations in a single scan.
// This is much more efficient than calling CountFeatures for each combination individually.
func (fc *FeatureCache) GetAllFeatureCounts(ctx context.Context) (map[string]map[string]int64, error) {
	result := make(map[string]map[string]int64)

	if !fc.IsHealthy() {
		return result, nil
	}

	client := fc.cacheService.GetClient()
	if client == nil {
		return result, nil
	}

	// Scan all features with current version in a single pass
	pattern := fmt.Sprintf(VersionedFeaturesCachePattern, fc.config.Version)

	iter := client.Scan(ctx, 0, pattern, 10000).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		parts := splitCacheKey(key)
		if parts == nil {
			continue
		}

		// Initialize symbol map if needed
		if _, ok := result[parts.symbol]; !ok {
			result[parts.symbol] = make(map[string]int64)
		}
		result[parts.symbol][parts.timeframe]++
	}

	if err := iter.Err(); err != nil {
		return result, fmt.Errorf("scan error: %w", err)
	}

	return result, nil
}

// --- Serialization ---

// featuresToMap converts CandleFeatures to a map suitable for Redis HSET.
// Nested feature structs are serialized as JSON strings.
func featuresToMap(f *CandleFeatures, version int) map[string]interface{} {
	m := map[string]interface{}{
		// Metadata
		"_version":      strconv.Itoa(version),
		"symbol":        f.Symbol,
		"timeframe":     string(f.Timeframe),
		"open_time":     strconv.FormatInt(f.OpenTime.Unix(), 10),
		"calculated_at": strconv.FormatInt(f.CalculatedAt.Unix(), 10),
		"lookback_used": strconv.Itoa(f.LookbackUsed),
		"lookback_needed": strconv.Itoa(f.LookbackNeeded),
		"is_complete":   boolToStr(f.IsComplete),
	}

	// Serialize feature categories as JSON (more efficient than 70+ individual fields)
	if f.Price != nil {
		if data, err := json.Marshal(f.Price); err == nil {
			m["price"] = string(data)
		}
	}
	if f.Volume != nil {
		if data, err := json.Marshal(f.Volume); err == nil {
			m["volume"] = string(data)
		}
	}
	if f.Volatility != nil {
		if data, err := json.Marshal(f.Volatility); err == nil {
			m["volatility"] = string(data)
		}
	}
	if f.Momentum != nil {
		if data, err := json.Marshal(f.Momentum); err == nil {
			m["momentum"] = string(data)
		}
	}
	if f.Trend != nil {
		if data, err := json.Marshal(f.Trend); err == nil {
			m["trend"] = string(data)
		}
	}
	if f.Time != nil {
		if data, err := json.Marshal(f.Time); err == nil {
			m["time"] = string(data)
		}
	}

	return m
}

// featuresFromMap parses a Redis hash into CandleFeatures.
func featuresFromMap(m map[string]string) (*CandleFeatures, error) {
	f := &CandleFeatures{}

	// Parse metadata
	f.Symbol = m["symbol"]
	f.Timeframe = Timeframe(m["timeframe"])

	if ts, err := strconv.ParseInt(m["open_time"], 10, 64); err == nil {
		f.OpenTime = time.Unix(ts, 0)
	}
	if ts, err := strconv.ParseInt(m["calculated_at"], 10, 64); err == nil {
		f.CalculatedAt = time.Unix(ts, 0)
	}
	if v, err := strconv.Atoi(m["lookback_used"]); err == nil {
		f.LookbackUsed = v
	}
	if v, err := strconv.Atoi(m["lookback_needed"]); err == nil {
		f.LookbackNeeded = v
	}
	f.IsComplete = m["is_complete"] == "1"

	// Parse feature categories from JSON with error logging
	var parseErrors []string

	if priceJSON := m["price"]; priceJSON != "" {
		var price indicators.PriceFeatures
		if err := json.Unmarshal([]byte(priceJSON), &price); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("price: %v", err))
		} else {
			f.Price = &price
		}
	}
	if volumeJSON := m["volume"]; volumeJSON != "" {
		var volume indicators.VolumeFeatures
		if err := json.Unmarshal([]byte(volumeJSON), &volume); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("volume: %v", err))
		} else {
			f.Volume = &volume
		}
	}
	if volatilityJSON := m["volatility"]; volatilityJSON != "" {
		var volatility indicators.VolatilityFeatures
		if err := json.Unmarshal([]byte(volatilityJSON), &volatility); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("volatility: %v", err))
		} else {
			f.Volatility = &volatility
		}
	}
	if momentumJSON := m["momentum"]; momentumJSON != "" {
		var momentum indicators.MomentumFeatures
		if err := json.Unmarshal([]byte(momentumJSON), &momentum); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("momentum: %v", err))
		} else {
			f.Momentum = &momentum
		}
	}
	if trendJSON := m["trend"]; trendJSON != "" {
		var trend indicators.TrendFeatures
		if err := json.Unmarshal([]byte(trendJSON), &trend); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("trend: %v", err))
		} else {
			f.Trend = &trend
		}
	}
	if timeJSON := m["time"]; timeJSON != "" {
		var timeFeatures indicators.TimeFeatures
		if err := json.Unmarshal([]byte(timeJSON), &timeFeatures); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("time: %v", err))
		} else {
			f.Time = &timeFeatures
		}
	}

	// Log warnings for any parse errors (data may be partially corrupted)
	if len(parseErrors) > 0 {
		log.Printf("[FEATURE_CACHE] Parse warnings for %s/%s at %v: %v",
			f.Symbol, f.Timeframe, f.OpenTime, parseErrors)
	}

	return f, nil
}

// Helper functions
func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// cacheKeyParts holds parsed components of a feature cache key
type cacheKeyParts struct {
	version   int
	symbol    string
	timeframe string
	timestamp int64
}

// splitCacheKey parses a feature cache key into its components.
// Key format: research:features:v{version}:{symbol}:{timeframe}:{timestamp}
// Returns nil if the key format is invalid.
func splitCacheKey(key string) *cacheKeyParts {
	// Expected format: research:features:v1:BTCUSDT:15m:1705327200
	// Split by colon
	const prefix = "research:features:v"
	if len(key) < len(prefix) {
		return nil
	}

	// Find version end
	rest := key[len(prefix):]
	colonIdx := 0
	for i, c := range rest {
		if c == ':' {
			colonIdx = i
			break
		}
	}
	if colonIdx == 0 {
		return nil
	}

	version, err := strconv.Atoi(rest[:colonIdx])
	if err != nil {
		return nil
	}

	// Parse remaining: {symbol}:{timeframe}:{timestamp}
	remaining := rest[colonIdx+1:]
	parts := make([]string, 0, 3)
	start := 0
	for i, c := range remaining {
		if c == ':' {
			parts = append(parts, remaining[start:i])
			start = i + 1
		}
	}
	parts = append(parts, remaining[start:])

	if len(parts) != 3 {
		return nil
	}

	timestamp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil
	}

	return &cacheKeyParts{
		version:   version,
		symbol:    parts[0],
		timeframe: parts[1],
		timestamp: timestamp,
	}
}
