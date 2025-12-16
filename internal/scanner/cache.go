package scanner

import (
	"sync"
	"time"
)

// ScannerCache manages cached proximity results with TTL
type ScannerCache struct {
	mu    sync.RWMutex
	cache map[string]*CachedProximity // key: symbol_strategyName
	ttl   time.Duration
}

// NewScannerCache creates a new cache with specified TTL
func NewScannerCache(ttl time.Duration) *ScannerCache {
	return &ScannerCache{
		cache: make(map[string]*CachedProximity),
		ttl:   ttl,
	}
}

// Get retrieves a proximity result from cache if not expired
func (sc *ScannerCache) Get(key string) *ProximityResult {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	cached, exists := sc.cache[key]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		return nil
	}

	return cached.Result
}

// Set stores a proximity result in cache with TTL
func (sc *ScannerCache) Set(key string, result *ProximityResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.cache[key] = &CachedProximity{
		Result:    result,
		ExpiresAt: time.Now().Add(sc.ttl),
	}
}

// Clear removes all cached results
func (sc *ScannerCache) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.cache = make(map[string]*CachedProximity)
}

// CleanupExpired removes expired cache entries
func (sc *ScannerCache) CleanupExpired() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()
	for key, cached := range sc.cache {
		if now.After(cached.ExpiresAt) {
			delete(sc.cache, key)
		}
	}
}
