// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.13: Indicator Calculation Engine
package indicators

import (
	"sync"
	"time"
)

// CacheEntry holds a cached indicator value with its metadata.
type CacheEntry struct {
	// Value is the cached IndicatorValue.
	Value *IndicatorValue
	// ExpiresAt is when this entry expires (Unix nanoseconds).
	ExpiresAt int64
	// CreatedAt is when this entry was created (Unix nanoseconds).
	CreatedAt int64
}

// IsExpired checks if the cache entry has expired.
func (e *CacheEntry) IsExpired() bool {
	return time.Now().UnixNano() > e.ExpiresAt
}

// Age returns how long ago this entry was created.
func (e *CacheEntry) Age() time.Duration {
	return time.Duration(time.Now().UnixNano() - e.CreatedAt)
}

// IndicatorCache provides an in-memory cache for calculated indicator values.
// It is thread-safe for concurrent access.
// Key hierarchy: symbol -> indicator name -> CacheEntry
type IndicatorCache struct {
	mu sync.RWMutex
	// cache stores entries by symbol and indicator name
	// map[symbol]map[indicatorName]*CacheEntry
	cache map[string]map[string]*CacheEntry
	// defaultTTL is the default time-to-live for cache entries
	defaultTTL time.Duration
	// stats tracks cache performance (use atomic operations or statsMu)
	statsMu sync.Mutex
	stats   CacheStats
	// stopCh signals cleanup goroutine to stop
	stopCh chan struct{}
	// stopped indicates if the cache has been closed
	stopped bool
}

// CacheStats holds cache performance metrics.
type CacheStats struct {
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Evictions  int64 `json:"evictions"`
	Size       int   `json:"size"`
	LastClean  int64 `json:"last_clean"`
}

// CacheConfig holds configuration for the IndicatorCache.
type CacheConfig struct {
	// DefaultTTL is the default time-to-live for entries.
	DefaultTTL time.Duration
	// CleanupInterval is how often to run cleanup (0 = no automatic cleanup).
	CleanupInterval time.Duration
	// MaxEntriesPerSymbol limits entries per symbol (0 = unlimited).
	MaxEntriesPerSymbol int
}

// DefaultCacheConfig returns sensible defaults for the cache.
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		DefaultTTL:          5 * time.Second, // Short TTL for real-time trading
		CleanupInterval:     30 * time.Second,
		MaxEntriesPerSymbol: 50, // Enough for all indicator types
	}
}

// NewIndicatorCache creates a new IndicatorCache with the given TTL.
// If ttl is 0, defaults to 5 seconds.
func NewIndicatorCache(ttl time.Duration) *IndicatorCache {
	if ttl <= 0 {
		ttl = 5 * time.Second
	}

	return &IndicatorCache{
		cache:      make(map[string]map[string]*CacheEntry),
		defaultTTL: ttl,
		stopCh:     make(chan struct{}),
	}
}

// NewIndicatorCacheWithConfig creates a new IndicatorCache with the given config.
// If config is nil, uses default config.
func NewIndicatorCacheWithConfig(config *CacheConfig) *IndicatorCache {
	if config == nil {
		config = DefaultCacheConfig()
	}

	cache := &IndicatorCache{
		cache:      make(map[string]map[string]*CacheEntry),
		defaultTTL: config.DefaultTTL,
		stopCh:     make(chan struct{}),
	}

	// Start cleanup goroutine if interval is set
	if config.CleanupInterval > 0 {
		go cache.cleanupLoop(config.CleanupInterval)
	}

	return cache
}

// Close stops the cleanup goroutine and releases resources.
// Safe to call multiple times.
func (c *IndicatorCache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}
	c.stopped = true
	close(c.stopCh)
}

// Get retrieves a cached indicator value for a symbol.
// Returns the value and true if found and not expired, nil and false otherwise.
func (c *IndicatorCache) Get(symbol, indicatorName string) (*IndicatorValue, bool) {
	c.mu.RLock()
	symbolCache, exists := c.cache[symbol]
	if !exists {
		c.mu.RUnlock()
		c.recordMiss()
		return nil, false
	}

	entry, exists := symbolCache[indicatorName]
	if !exists || entry.IsExpired() {
		c.mu.RUnlock()
		c.recordMiss()
		return nil, false
	}

	value := entry.Value
	c.mu.RUnlock()
	c.recordHit()
	return value, true
}

// recordHit safely increments the hit counter.
func (c *IndicatorCache) recordHit() {
	c.statsMu.Lock()
	c.stats.Hits++
	c.statsMu.Unlock()
}

// recordMiss safely increments the miss counter.
func (c *IndicatorCache) recordMiss() {
	c.statsMu.Lock()
	c.stats.Misses++
	c.statsMu.Unlock()
}

// GetMultiple retrieves multiple cached indicator values for a symbol.
// Returns a map of indicator name to value for all found (non-expired) entries.
func (c *IndicatorCache) GetMultiple(symbol string, indicatorNames []string) map[string]*IndicatorValue {
	c.mu.RLock()
	result := make(map[string]*IndicatorValue)
	hits := 0
	misses := 0

	symbolCache, exists := c.cache[symbol]
	if !exists {
		c.mu.RUnlock()
		// All are misses
		c.statsMu.Lock()
		c.stats.Misses += int64(len(indicatorNames))
		c.statsMu.Unlock()
		return result
	}

	for _, name := range indicatorNames {
		if entry, ok := symbolCache[name]; ok && !entry.IsExpired() {
			result[name] = entry.Value
			hits++
		} else {
			misses++
		}
	}
	c.mu.RUnlock()

	// Update stats outside of RLock
	c.statsMu.Lock()
	c.stats.Hits += int64(hits)
	c.stats.Misses += int64(misses)
	c.statsMu.Unlock()

	return result
}

// Set stores an indicator value in the cache with the default TTL.
func (c *IndicatorCache) Set(symbol, indicatorName string, value *IndicatorValue) {
	c.SetWithTTL(symbol, indicatorName, value, c.defaultTTL)
}

// SetWithTTL stores an indicator value with a custom TTL.
func (c *IndicatorCache) SetWithTTL(symbol, indicatorName string, value *IndicatorValue, ttl time.Duration) {
	if value == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()

	// Initialize symbol cache if needed
	if _, exists := c.cache[symbol]; !exists {
		c.cache[symbol] = make(map[string]*CacheEntry)
	}

	c.cache[symbol][indicatorName] = &CacheEntry{
		Value:     value,
		ExpiresAt: now + int64(ttl),
		CreatedAt: now,
	}
}

// SetMultiple stores multiple indicator values at once.
func (c *IndicatorCache) SetMultiple(symbol string, values map[string]*IndicatorValue) {
	if len(values) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	expiresAt := now + int64(c.defaultTTL)

	// Initialize symbol cache if needed
	if _, exists := c.cache[symbol]; !exists {
		c.cache[symbol] = make(map[string]*CacheEntry)
	}

	for name, value := range values {
		if value != nil {
			c.cache[symbol][name] = &CacheEntry{
				Value:     value,
				ExpiresAt: expiresAt,
				CreatedAt: now,
			}
		}
	}
}

// Invalidate removes a specific indicator entry from the cache.
func (c *IndicatorCache) Invalidate(symbol, indicatorName string) {
	c.mu.Lock()
	evicted := false
	if symbolCache, exists := c.cache[symbol]; exists {
		if _, ok := symbolCache[indicatorName]; ok {
			delete(symbolCache, indicatorName)
			evicted = true
		}
	}
	c.mu.Unlock()

	if evicted {
		c.statsMu.Lock()
		c.stats.Evictions++
		c.statsMu.Unlock()
	}
}

// InvalidateMultiple removes multiple indicator entries from the cache.
func (c *IndicatorCache) InvalidateMultiple(symbol string, indicatorNames []string) {
	c.mu.Lock()
	evicted := 0
	if symbolCache, exists := c.cache[symbol]; exists {
		for _, name := range indicatorNames {
			if _, ok := symbolCache[name]; ok {
				delete(symbolCache, name)
				evicted++
			}
		}
	}
	c.mu.Unlock()

	if evicted > 0 {
		c.statsMu.Lock()
		c.stats.Evictions += int64(evicted)
		c.statsMu.Unlock()
	}
}

// InvalidateBySymbol removes all cached entries for a symbol.
func (c *IndicatorCache) InvalidateBySymbol(symbol string) {
	c.mu.Lock()
	evicted := 0
	if symbolCache, exists := c.cache[symbol]; exists {
		evicted = len(symbolCache)
		delete(c.cache, symbol)
	}
	c.mu.Unlock()

	if evicted > 0 {
		c.statsMu.Lock()
		c.stats.Evictions += int64(evicted)
		c.statsMu.Unlock()
	}
}

// InvalidateByIndicator removes all cached entries for a specific indicator across all symbols.
func (c *IndicatorCache) InvalidateByIndicator(indicatorName string) {
	c.mu.Lock()
	evicted := 0
	for _, symbolCache := range c.cache {
		if _, ok := symbolCache[indicatorName]; ok {
			delete(symbolCache, indicatorName)
			evicted++
		}
	}
	c.mu.Unlock()

	if evicted > 0 {
		c.statsMu.Lock()
		c.stats.Evictions += int64(evicted)
		c.statsMu.Unlock()
	}
}

// InvalidateAll clears the entire cache.
func (c *IndicatorCache) InvalidateAll() {
	c.mu.Lock()
	evicted := 0
	// Count all entries for eviction stats
	for _, symbolCache := range c.cache {
		evicted += len(symbolCache)
	}
	c.cache = make(map[string]map[string]*CacheEntry)
	c.mu.Unlock()

	if evicted > 0 {
		c.statsMu.Lock()
		c.stats.Evictions += int64(evicted)
		c.statsMu.Unlock()
	}
}

// Clean removes all expired entries from the cache.
// Returns the number of entries removed.
func (c *IndicatorCache) Clean() int {
	c.mu.Lock()
	now := time.Now().UnixNano()
	removed := 0

	for symbol, symbolCache := range c.cache {
		for name, entry := range symbolCache {
			if now > entry.ExpiresAt {
				delete(symbolCache, name)
				removed++
			}
		}
		// Remove empty symbol caches
		if len(symbolCache) == 0 {
			delete(c.cache, symbol)
		}
	}
	c.mu.Unlock()

	// Update stats outside of cache lock
	c.statsMu.Lock()
	c.stats.Evictions += int64(removed)
	c.stats.LastClean = time.Now().UnixMilli()
	c.statsMu.Unlock()

	return removed
}

// cleanupLoop runs periodic cleanup in a background goroutine.
func (c *IndicatorCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Clean()
		case <-c.stopCh:
			return
		}
	}
}

// Size returns the total number of cached entries.
func (c *IndicatorCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := 0
	for _, symbolCache := range c.cache {
		total += len(symbolCache)
	}
	return total
}

// SizeBySymbol returns the number of cached entries for a symbol.
func (c *IndicatorCache) SizeBySymbol(symbol string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if symbolCache, exists := c.cache[symbol]; exists {
		return len(symbolCache)
	}
	return 0
}

// Symbols returns all symbols with cached entries.
func (c *IndicatorCache) Symbols() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	symbols := make([]string, 0, len(c.cache))
	for symbol := range c.cache {
		symbols = append(symbols, symbol)
	}
	return symbols
}

// GetStats returns cache performance statistics.
func (c *IndicatorCache) GetStats() CacheStats {
	// Get stats atomically
	c.statsMu.Lock()
	stats := c.stats
	c.statsMu.Unlock()

	// Get size from cache
	c.mu.RLock()
	stats.Size = 0
	for _, symbolCache := range c.cache {
		stats.Size += len(symbolCache)
	}
	c.mu.RUnlock()

	return stats
}

// ResetStats resets the cache statistics.
func (c *IndicatorCache) ResetStats() {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()

	c.stats = CacheStats{}
}

// HitRate returns the cache hit rate as a percentage (0-100).
func (c *IndicatorCache) HitRate() float64 {
	c.statsMu.Lock()
	hits := c.stats.Hits
	misses := c.stats.Misses
	c.statsMu.Unlock()

	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

// GetDefaultTTL returns the default TTL for cache entries.
func (c *IndicatorCache) GetDefaultTTL() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.defaultTTL
}

// SetDefaultTTL updates the default TTL for new cache entries.
func (c *IndicatorCache) SetDefaultTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultTTL = ttl
}

// Contains checks if a cache entry exists and is not expired.
func (c *IndicatorCache) Contains(symbol, indicatorName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if symbolCache, exists := c.cache[symbol]; exists {
		if entry, ok := symbolCache[indicatorName]; ok {
			return !entry.IsExpired()
		}
	}
	return false
}

// GetAge returns the age of a cache entry, or 0 if not found.
func (c *IndicatorCache) GetAge(symbol, indicatorName string) time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if symbolCache, exists := c.cache[symbol]; exists {
		if entry, ok := symbolCache[indicatorName]; ok && !entry.IsExpired() {
			return entry.Age()
		}
	}
	return 0
}

// GetAllForSymbol returns all non-expired cached values for a symbol.
func (c *IndicatorCache) GetAllForSymbol(symbol string) map[string]*IndicatorValue {
	c.mu.RLock()
	result := make(map[string]*IndicatorValue)
	hits := 0

	if symbolCache, exists := c.cache[symbol]; exists {
		for name, entry := range symbolCache {
			if !entry.IsExpired() {
				result[name] = entry.Value
				hits++
			}
		}
	}
	c.mu.RUnlock()

	// Update stats outside of RLock
	if hits > 0 {
		c.statsMu.Lock()
		c.stats.Hits += int64(hits)
		c.statsMu.Unlock()
	}

	return result
}

// UpdateType indicates what kind of data update occurred.
type UpdateType int

const (
	// UpdatePriceTick indicates a price tick update (minor change).
	UpdatePriceTick UpdateType = iota
	// UpdateCandleClose indicates a candle close (requires full recalculation).
	UpdateCandleClose
)

// String returns the string representation of UpdateType.
func (ut UpdateType) String() string {
	switch ut {
	case UpdatePriceTick:
		return "price_tick"
	case UpdateCandleClose:
		return "candle_close"
	default:
		return "unknown"
	}
}

// InvalidateForUpdate invalidates cache entries based on update type.
// Price ticks only invalidate price-sensitive indicators.
// Candle closes invalidate all indicators for the symbol.
func (c *IndicatorCache) InvalidateForUpdate(symbol string, updateType UpdateType) {
	switch updateType {
	case UpdatePriceTick:
		// Price ticks only invalidate certain indicators
		c.InvalidateMultiple(symbol, PriceSensitiveIndicators())
	case UpdateCandleClose:
		// Candle close invalidates everything for this symbol
		c.InvalidateBySymbol(symbol)
	}
}

// PriceSensitiveIndicators returns indicators that change with price ticks.
// These are invalidated on every price tick.
func PriceSensitiveIndicators() []string {
	return []string{
		"vwap",           // Volume Weighted Average Price
		"supertrend",     // Uses current price
		"price_momentum", // Direct price calculation
	}
}

// CandleSensitiveIndicators returns indicators that only change on candle close.
// These are only invalidated when a new candle completes.
func CandleSensitiveIndicators() []string {
	return []string{
		"ema_cross",
		"macd",
		"rsi",
		"stochastic",
		"atr",
		"bollinger_width",
		"adx_trend",
		"volume_sma",
		"obv",
		"mfi",
		"chaikin_mf",
	}
}
