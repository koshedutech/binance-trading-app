package analysis

import (
	"fmt"
	"sync"
	"time"
	"binance-trading-bot/internal/binance"
)

// TimeframeManager handles multi-timeframe candlestick data
type TimeframeManager struct {
	client    *binance.Client
	cache     *CandleCache
	mu        sync.RWMutex
}

// Timeframe represents different chart timeframes
type Timeframe string

const (
	TF1m  Timeframe = "1m"
	TF5m  Timeframe = "5m"
	TF15m Timeframe = "15m"
	TF1h  Timeframe = "1h"
	TF4h  Timeframe = "4h"
	TF1d  Timeframe = "1d"
)

// MultiTimeframeData holds candles across different timeframes
type MultiTimeframeData struct {
	Symbol    string
	Timestamp time.Time
	Data      map[Timeframe][]binance.Kline
}

// CandleCache provides caching for candle data
type CandleCache struct {
	data map[string]*CacheEntry
	mu   sync.RWMutex
}

// CacheEntry represents a cached candle dataset
type CacheEntry struct {
	Candles   []binance.Kline
	ExpiresAt time.Time
}

// NewTimeframeManager creates a new multi-timeframe data manager
func NewTimeframeManager(client *binance.Client) *TimeframeManager {
	return &TimeframeManager{
		client: client,
		cache:  NewCandleCache(),
	}
}

// NewCandleCache creates a new candle cache
func NewCandleCache() *CandleCache {
	return &CandleCache{
		data: make(map[string]*CacheEntry),
	}
}

// GetMultiTimeframeData fetches candles for multiple timeframes in parallel
func (tm *TimeframeManager) GetMultiTimeframeData(symbol string, timeframes []Timeframe, limit int) (*MultiTimeframeData, error) {
	result := &MultiTimeframeData{
		Symbol:    symbol,
		Timestamp: time.Now(),
		Data:      make(map[Timeframe][]binance.Kline),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(timeframes))

	// Fetch all timeframes in parallel
	for _, tf := range timeframes {
		wg.Add(1)
		go func(timeframe Timeframe) {
			defer wg.Done()

			candles, err := tm.GetCandles(symbol, string(timeframe), limit)
			if err != nil {
				errChan <- fmt.Errorf("failed to fetch %s %s: %w", symbol, timeframe, err)
				return
			}

			mu.Lock()
			result.Data[timeframe] = candles
			mu.Unlock()
		}(tf)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if err := <-errChan; err != nil {
		return nil, err
	}

	return result, nil
}

// GetCandles fetches candles with caching
func (tm *TimeframeManager) GetCandles(symbol, interval string, limit int) ([]binance.Kline, error) {
	cacheKey := fmt.Sprintf("%s:%s:%d", symbol, interval, limit)

	// Check cache first
	if cached := tm.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	// Fetch from API
	candles, err := tm.client.GetKlines(symbol, interval, limit)
	if err != nil {
		return nil, err
	}

	// Cache with appropriate TTL based on timeframe
	ttl := tm.getCacheTTL(interval)
	tm.cache.Set(cacheKey, candles, ttl)

	return candles, nil
}

// getCacheTTL returns appropriate cache TTL based on timeframe
func (tm *TimeframeManager) getCacheTTL(interval string) time.Duration {
	switch interval {
	case "1m":
		return 30 * time.Second
	case "5m":
		return 2 * time.Minute
	case "15m":
		return 5 * time.Minute
	case "1h":
		return 30 * time.Minute
	case "4h":
		return 2 * time.Hour
	case "1d":
		return 12 * time.Hour
	default:
		return 1 * time.Minute
	}
}

// Get retrieves cached candles if not expired
func (c *CandleCache) Get(key string) []binance.Kline {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.data[key]
	if !exists {
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry.Candles
}

// Set stores candles in cache with expiration
func (c *CandleCache) Set(key string, candles []binance.Kline, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = &CacheEntry{
		Candles:   candles,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Clear removes expired entries from cache
func (c *CandleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.data {
		if now.After(entry.ExpiresAt) {
			delete(c.data, key)
		}
	}
}
