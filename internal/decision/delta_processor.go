// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.2: Delta Update Processor
package decision

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// DeltaProcessor compares new state values against cached values and only updates
// changed fields in Redis. This minimizes network traffic and Redis operations.
type DeltaProcessor struct {
	cache      map[string]map[string]interface{} // userID:symbol -> field -> value
	cacheMutex sync.RWMutex                      // Thread safety for cache
	metrics    *DeltaMetrics                     // Update frequency tracking
	sm         *StateManager                     // For Redis operations

	// Cache size management to prevent unbounded growth
	maxCacheSize int // Maximum number of cache entries (0 = unlimited)
}

// DeltaMetrics tracks update frequency and processing statistics.
type DeltaMetrics struct {
	fieldUpdateCounts map[string]int64 // field name -> update count
	totalUpdates      int64            // Total number of delta updates processed
	totalProcessTime  int64            // Total processing time in nanoseconds
	processCount      int64            // Number of Process() calls
	mutex             sync.RWMutex     // Thread safety for metrics
}

// DeltaStats provides a snapshot of processing statistics.
type DeltaStats struct {
	TotalUpdates      int64         // Total number of field updates
	TotalProcessCalls int64         // Total number of Process() calls
	AvgProcessTime    time.Duration // Average processing time per call
	FieldUpdateCounts map[string]int64
}

// DeltaResult contains the result of a delta processing operation.
type DeltaResult struct {
	ChangedFields []string               // Names of fields that changed
	UpdatedValues map[string]interface{} // Field -> new value (for batched Redis update)
	ProcessTime   time.Duration          // Time taken to process
}

// DefaultMaxCacheSize is the default maximum number of cache entries.
// Set to 10,000 to support typical trading scenarios while preventing unbounded growth.
const DefaultMaxCacheSize = 10000

// NewDeltaProcessor creates a new DeltaProcessor with the given StateManager.
// Returns nil if StateManager is nil.
func NewDeltaProcessor(sm *StateManager) *DeltaProcessor {
	if sm == nil {
		return nil
	}
	return &DeltaProcessor{
		cache:        make(map[string]map[string]interface{}),
		metrics:      newDeltaMetrics(),
		sm:           sm,
		maxCacheSize: DefaultMaxCacheSize,
	}
}

// NewDeltaProcessorWithMaxCache creates a new DeltaProcessor with custom max cache size.
// Use maxCacheSize=0 for unlimited (not recommended in production).
func NewDeltaProcessorWithMaxCache(sm *StateManager, maxCacheSize int) *DeltaProcessor {
	if sm == nil {
		return nil
	}
	return &DeltaProcessor{
		cache:        make(map[string]map[string]interface{}),
		metrics:      newDeltaMetrics(),
		sm:           sm,
		maxCacheSize: maxCacheSize,
	}
}

// newDeltaMetrics creates a new DeltaMetrics instance.
func newDeltaMetrics() *DeltaMetrics {
	return &DeltaMetrics{
		fieldUpdateCounts: make(map[string]int64),
	}
}

// cacheKey generates a unique cache key for a user+symbol combination.
func cacheKey(userID, symbol string) string {
	return userID + ":" + symbol
}

// Process compares new state values against cached values and returns changed fields.
// It updates the internal cache and writes changes to Redis via StateManager.
// This is the main entry point for delta processing.
func (dp *DeltaProcessor) Process(ctx context.Context, userID, symbol string, newState map[string]interface{}) (*DeltaResult, error) {
	startTime := time.Now()

	if userID == "" || symbol == "" {
		return nil, fmt.Errorf("userID and symbol are required")
	}
	if newState == nil {
		return nil, fmt.Errorf("newState cannot be nil")
	}

	// Get cached state for this symbol - copy to avoid race conditions
	key := cacheKey(userID, symbol)

	dp.cacheMutex.RLock()
	cachedState, exists := dp.cache[key]
	var cachedStateCopy map[string]interface{}
	if exists {
		cachedStateCopy = make(map[string]interface{}, len(cachedState))
		for k, v := range cachedState {
			cachedStateCopy[k] = v
		}
	}
	dp.cacheMutex.RUnlock()

	// Identify changed fields
	changedFields := make([]string, 0)
	updatedValues := make(map[string]interface{})

	if !exists {
		// New symbol - all fields are considered changed
		for field, value := range newState {
			changedFields = append(changedFields, field)
			updatedValues[field] = value
		}
	} else {
		// Compare against cached values (using copied state)
		for field, newVal := range newState {
			oldVal, hasOld := cachedStateCopy[field]
			if !hasOld || !dp.compareValues(oldVal, newVal) {
				changedFields = append(changedFields, field)
				updatedValues[field] = newVal
			}
		}
	}

	// Record metrics for changed fields and write to Redis BEFORE updating cache
	// This ensures cache consistency: if Redis write fails, cache remains unchanged
	if len(changedFields) > 0 {
		dp.recordFieldUpdates(changedFields)

		// Write changes to Redis using batched HSET
		if err := dp.sm.UpdateCoinState(ctx, userID, symbol, updatedValues); err != nil {
			return nil, fmt.Errorf("failed to update Redis: %w", err)
		}
	}

	// Update cache with new state ONLY AFTER successful Redis write
	dp.cacheMutex.Lock()
	// Check cache size limit to prevent unbounded growth
	if dp.maxCacheSize > 0 && len(dp.cache) >= dp.maxCacheSize && dp.cache[key] == nil {
		// Cache is full and this is a new entry - evict oldest (simple FIFO via map iteration)
		// Note: map iteration order is random, so this is approximate LRU
		for evictKey := range dp.cache {
			delete(dp.cache, evictKey)
			break
		}
	}
	if dp.cache[key] == nil {
		dp.cache[key] = make(map[string]interface{})
	}
	for field, value := range newState {
		dp.cache[key][field] = value
	}
	dp.cacheMutex.Unlock()

	processTime := time.Since(startTime)
	dp.recordProcessTime(processTime)

	return &DeltaResult{
		ChangedFields: changedFields,
		UpdatedValues: updatedValues,
		ProcessTime:   processTime,
	}, nil
}

// ProcessWithoutRedis compares new state values against cached values without writing to Redis.
// Useful for testing or when Redis write is handled separately.
func (dp *DeltaProcessor) ProcessWithoutRedis(userID, symbol string, newState map[string]interface{}) (*DeltaResult, error) {
	startTime := time.Now()

	if userID == "" || symbol == "" {
		return nil, fmt.Errorf("userID and symbol are required")
	}
	if newState == nil {
		return nil, fmt.Errorf("newState cannot be nil")
	}

	key := cacheKey(userID, symbol)

	// Get cached state with proper locking - copy to avoid race conditions
	dp.cacheMutex.RLock()
	cachedState, exists := dp.cache[key]
	var cachedStateCopy map[string]interface{}
	if exists {
		cachedStateCopy = make(map[string]interface{}, len(cachedState))
		for k, v := range cachedState {
			cachedStateCopy[k] = v
		}
	}
	dp.cacheMutex.RUnlock()

	changedFields := make([]string, 0)
	updatedValues := make(map[string]interface{})

	if !exists {
		for field, value := range newState {
			changedFields = append(changedFields, field)
			updatedValues[field] = value
		}
	} else {
		for field, newVal := range newState {
			oldVal, hasOld := cachedStateCopy[field]
			if !hasOld || !dp.compareValues(oldVal, newVal) {
				changedFields = append(changedFields, field)
				updatedValues[field] = newVal
			}
		}
	}

	if len(changedFields) > 0 {
		dp.recordFieldUpdates(changedFields)
	}

	// Update cache
	dp.cacheMutex.Lock()
	// Check cache size limit to prevent unbounded growth
	if dp.maxCacheSize > 0 && len(dp.cache) >= dp.maxCacheSize && dp.cache[key] == nil {
		// Cache is full and this is a new entry - evict one
		for evictKey := range dp.cache {
			delete(dp.cache, evictKey)
			break
		}
	}
	if dp.cache[key] == nil {
		dp.cache[key] = make(map[string]interface{})
	}
	for field, value := range newState {
		dp.cache[key][field] = value
	}
	dp.cacheMutex.Unlock()

	processTime := time.Since(startTime)
	dp.recordProcessTime(processTime)

	return &DeltaResult{
		ChangedFields: changedFields,
		UpdatedValues: updatedValues,
		ProcessTime:   processTime,
	}, nil
}

// compareValues compares two values and returns true if they are equal.
// Handles different types: float64, int, int64, string, []string, enums (stored as strings).
func (dp *DeltaProcessor) compareValues(oldVal, newVal interface{}) bool {
	// Handle nil values
	if oldVal == nil && newVal == nil {
		return true
	}
	if oldVal == nil || newVal == nil {
		return false
	}

	// Type-specific comparison
	switch old := oldVal.(type) {
	case float64:
		if new, ok := newVal.(float64); ok {
			// Use tolerance for float comparison to avoid floating point errors
			const epsilon = 1e-9
			return math.Abs(old-new) < epsilon
		}
		// Handle string-encoded float (from Redis)
		if newStr, ok := newVal.(string); ok {
			newFloat, err := strconv.ParseFloat(newStr, 64)
			if err == nil {
				const epsilon = 1e-9
				return math.Abs(old-newFloat) < epsilon
			}
		}
		return false

	case int:
		switch new := newVal.(type) {
		case int:
			return old == new
		case int64:
			return int64(old) == new
		case float64:
			return float64(old) == new
		case string:
			newInt, err := strconv.Atoi(new)
			if err == nil {
				return old == newInt
			}
		}
		return false

	case int64:
		switch new := newVal.(type) {
		case int64:
			return old == new
		case int:
			return old == int64(new)
		case float64:
			return float64(old) == new
		case string:
			newInt, err := strconv.ParseInt(new, 10, 64)
			if err == nil {
				return old == newInt
			}
		}
		return false

	case string:
		if new, ok := newVal.(string); ok {
			return old == new
		}
		// Handle enum types that might be stored as their underlying string
		return fmt.Sprintf("%v", oldVal) == fmt.Sprintf("%v", newVal)

	case []string:
		if new, ok := newVal.([]string); ok {
			return compareStringSlices(old, new)
		}
		// Handle JSON-encoded slice from Redis
		if newStr, ok := newVal.(string); ok {
			var newSlice []string
			if err := json.Unmarshal([]byte(newStr), &newSlice); err == nil {
				return compareStringSlices(old, newSlice)
			}
		}
		return false

	case MarketRegime:
		if new, ok := newVal.(MarketRegime); ok {
			return old == new
		}
		if new, ok := newVal.(string); ok {
			return string(old) == new
		}
		return false

	case TrendDirection:
		if new, ok := newVal.(TrendDirection); ok {
			return old == new
		}
		if new, ok := newVal.(string); ok {
			return string(old) == new
		}
		return false

	case Decision:
		if new, ok := newVal.(Decision); ok {
			return old == new
		}
		if new, ok := newVal.(string); ok {
			return string(old) == new
		}
		return false

	case bool:
		if new, ok := newVal.(bool); ok {
			return old == new
		}
		// Handle string representations from Redis
		if new, ok := newVal.(string); ok {
			return (old && new == "true") || (!old && new == "false")
		}
		return false

	default:
		// Fallback: compare string representations
		return fmt.Sprintf("%v", oldVal) == fmt.Sprintf("%v", newVal)
	}
}

// compareStringSlices compares two string slices for equality.
func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// recordFieldUpdates increments update counts for the given fields.
func (dp *DeltaProcessor) recordFieldUpdates(fields []string) {
	dp.metrics.mutex.Lock()
	defer dp.metrics.mutex.Unlock()

	for _, field := range fields {
		dp.metrics.fieldUpdateCounts[field]++
	}
	atomic.AddInt64(&dp.metrics.totalUpdates, int64(len(fields)))
}

// recordProcessTime records the time taken for a Process() call.
func (dp *DeltaProcessor) recordProcessTime(duration time.Duration) {
	atomic.AddInt64(&dp.metrics.totalProcessTime, int64(duration))
	atomic.AddInt64(&dp.metrics.processCount, 1)
}

// GetFieldMetrics returns a copy of the field update counts.
func (dp *DeltaProcessor) GetFieldMetrics() map[string]int64 {
	dp.metrics.mutex.RLock()
	defer dp.metrics.mutex.RUnlock()

	result := make(map[string]int64, len(dp.metrics.fieldUpdateCounts))
	for field, count := range dp.metrics.fieldUpdateCounts {
		result[field] = count
	}
	return result
}

// GetHotFields returns the top N most frequently updated field names.
func (dp *DeltaProcessor) GetHotFields(topN int) []string {
	dp.metrics.mutex.RLock()
	defer dp.metrics.mutex.RUnlock()

	if topN <= 0 {
		return []string{}
	}

	// Create slice of field-count pairs for sorting
	type fieldCount struct {
		field string
		count int64
	}
	pairs := make([]fieldCount, 0, len(dp.metrics.fieldUpdateCounts))
	for field, count := range dp.metrics.fieldUpdateCounts {
		pairs = append(pairs, fieldCount{field, count})
	}

	// Sort by count descending
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})

	// Take top N
	if topN > len(pairs) {
		topN = len(pairs)
	}

	result := make([]string, topN)
	for i := 0; i < topN; i++ {
		result[i] = pairs[i].field
	}

	return result
}

// GetProcessingStats returns a snapshot of processing statistics.
func (dp *DeltaProcessor) GetProcessingStats() DeltaStats {
	totalUpdates := atomic.LoadInt64(&dp.metrics.totalUpdates)
	totalProcessTime := atomic.LoadInt64(&dp.metrics.totalProcessTime)
	processCount := atomic.LoadInt64(&dp.metrics.processCount)

	var avgProcessTime time.Duration
	if processCount > 0 {
		avgProcessTime = time.Duration(totalProcessTime / processCount)
	}

	return DeltaStats{
		TotalUpdates:      totalUpdates,
		TotalProcessCalls: processCount,
		AvgProcessTime:    avgProcessTime,
		FieldUpdateCounts: dp.GetFieldMetrics(),
	}
}

// GetCachedState returns a copy of the cached state for a user+symbol.
// Returns nil if no state is cached.
func (dp *DeltaProcessor) GetCachedState(userID, symbol string) map[string]interface{} {
	key := cacheKey(userID, symbol)

	dp.cacheMutex.RLock()
	defer dp.cacheMutex.RUnlock()

	cached, exists := dp.cache[key]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	result := make(map[string]interface{}, len(cached))
	for k, v := range cached {
		result[k] = v
	}
	return result
}

// ClearCache removes all cached state entries.
func (dp *DeltaProcessor) ClearCache() {
	dp.cacheMutex.Lock()
	defer dp.cacheMutex.Unlock()

	dp.cache = make(map[string]map[string]interface{})
}

// ClearCacheForSymbol removes the cached state for a specific user+symbol.
func (dp *DeltaProcessor) ClearCacheForSymbol(userID, symbol string) {
	key := cacheKey(userID, symbol)

	dp.cacheMutex.Lock()
	defer dp.cacheMutex.Unlock()

	delete(dp.cache, key)
}

// ClearCacheForUser removes all cached states for a specific user.
func (dp *DeltaProcessor) ClearCacheForUser(userID string) {
	prefix := userID + ":"

	dp.cacheMutex.Lock()
	defer dp.cacheMutex.Unlock()

	for key := range dp.cache {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			delete(dp.cache, key)
		}
	}
}

// ResetMetrics clears all metrics.
func (dp *DeltaProcessor) ResetMetrics() {
	// Reset all metrics atomically while holding the mutex
	// to prevent inconsistent state during concurrent access
	dp.metrics.mutex.Lock()
	dp.metrics.fieldUpdateCounts = make(map[string]int64)
	atomic.StoreInt64(&dp.metrics.totalUpdates, 0)
	atomic.StoreInt64(&dp.metrics.totalProcessTime, 0)
	atomic.StoreInt64(&dp.metrics.processCount, 0)
	dp.metrics.mutex.Unlock()
}

// CacheSize returns the number of symbol states currently cached.
func (dp *DeltaProcessor) CacheSize() int {
	dp.cacheMutex.RLock()
	defer dp.cacheMutex.RUnlock()

	return len(dp.cache)
}

// PreloadCache populates the cache from an existing CoinState.
// Useful for initializing the cache from Redis data.
func (dp *DeltaProcessor) PreloadCache(userID, symbol string, state *CoinState) {
	if state == nil {
		return
	}

	key := cacheKey(userID, symbol)
	stateMap := state.ToMap()

	dp.cacheMutex.Lock()
	defer dp.cacheMutex.Unlock()

	dp.cache[key] = make(map[string]interface{}, len(stateMap))
	for k, v := range stateMap {
		dp.cache[key][k] = v
	}
}
