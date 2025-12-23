package binance

import (
	"strconv"
	"sync"
	"time"
)

// CachedMarkPrice holds mark price data with timestamp
type CachedMarkPrice struct {
	Data      *MarkPrice
	UpdatedAt time.Time
}

// CachedKlines holds kline data with timestamp
type CachedKlines struct {
	Data      []Kline
	Interval  string
	UpdatedAt time.Time
}

// CachedOrderBook holds order book data with timestamp
type CachedOrderBook struct {
	Data      *OrderBookDepth
	UpdatedAt time.Time
}

// MarketDataCache provides thread-safe caching for market data from WebSocket streams
type MarketDataCache struct {
	markPrices sync.Map // symbol -> *CachedMarkPrice
	klines     sync.Map // "symbol:interval" -> *CachedKlines
	orderBooks sync.Map // symbol -> *CachedOrderBook

	// Statistics
	hitCount  int64
	missCount int64
	statsMu   sync.RWMutex
}

// NewMarketDataCache creates a new market data cache
func NewMarketDataCache() *MarketDataCache {
	return &MarketDataCache{}
}

// ==================== MARK PRICE ====================

// GetMarkPrice returns cached mark price for a symbol
func (c *MarketDataCache) GetMarkPrice(symbol string) *MarkPrice {
	if val, ok := c.markPrices.Load(symbol); ok {
		cached := val.(*CachedMarkPrice)
		// Check if data is stale (older than 30 seconds for REST, WebSocket updates more frequently)
		if time.Since(cached.UpdatedAt) < 30*time.Second {
			c.recordHit()
			return cached.Data
		}
	}
	c.recordMiss()
	return nil
}

// UpdateMarkPrice updates the cached mark price from WebSocket data
func (c *MarketDataCache) UpdateMarkPrice(symbol string, markPrice float64, indexPrice float64, fundingRate float64, nextFundingTime int64) {
	cached := &CachedMarkPrice{
		Data: &MarkPrice{
			Symbol:          symbol,
			MarkPrice:       markPrice,
			IndexPrice:      indexPrice,
			LastFundingRate: fundingRate,
			NextFundingTime: nextFundingTime,
			Time:            time.Now().UnixMilli(),
		},
		UpdatedAt: time.Now(),
	}
	c.markPrices.Store(symbol, cached)
}

// UpdateMarkPriceFromStrings updates mark price from WebSocket string values
func (c *MarketDataCache) UpdateMarkPriceFromStrings(symbol, markPriceStr, indexPriceStr, fundingRateStr string, nextFundingTime int64) {
	markPrice, _ := strconv.ParseFloat(markPriceStr, 64)
	indexPrice, _ := strconv.ParseFloat(indexPriceStr, 64)
	fundingRate, _ := strconv.ParseFloat(fundingRateStr, 64)
	c.UpdateMarkPrice(symbol, markPrice, indexPrice, fundingRate, nextFundingTime)
}

// GetCurrentPrice returns just the current price for a symbol (from mark price)
func (c *MarketDataCache) GetCurrentPrice(symbol string) (float64, bool) {
	mp := c.GetMarkPrice(symbol)
	if mp != nil {
		return mp.MarkPrice, true
	}
	return 0, false
}

// ==================== KLINES ====================

// GetKlines returns cached klines for a symbol and interval
func (c *MarketDataCache) GetKlines(symbol, interval string, limit int) []Kline {
	key := symbol + ":" + interval
	if val, ok := c.klines.Load(key); ok {
		cached := val.(*CachedKlines)
		// Check if data is stale (older than 60 seconds for klines)
		if time.Since(cached.UpdatedAt) < 60*time.Second {
			c.recordHit()
			data := cached.Data
			if len(data) > limit {
				return data[len(data)-limit:]
			}
			return data
		}
	}
	c.recordMiss()
	return nil
}

// UpdateKline updates a single kline in the cache (from WebSocket stream)
func (c *MarketDataCache) UpdateKline(symbol, interval string, kline Kline) {
	key := symbol + ":" + interval

	var cached *CachedKlines
	if val, ok := c.klines.Load(key); ok {
		cached = val.(*CachedKlines)
	} else {
		cached = &CachedKlines{
			Data:     make([]Kline, 0, 100),
			Interval: interval,
		}
	}

	// Check if we should update the last kline or append new one
	if len(cached.Data) > 0 {
		lastIdx := len(cached.Data) - 1
		if cached.Data[lastIdx].OpenTime == kline.OpenTime {
			// Update existing kline
			cached.Data[lastIdx] = kline
		} else {
			// Append new kline
			cached.Data = append(cached.Data, kline)
			// Keep only last 100 klines
			if len(cached.Data) > 100 {
				cached.Data = cached.Data[1:]
			}
		}
	} else {
		cached.Data = append(cached.Data, kline)
	}

	cached.UpdatedAt = time.Now()
	c.klines.Store(key, cached)
}

// SetKlines sets the full klines cache (from REST API fallback)
func (c *MarketDataCache) SetKlines(symbol, interval string, klines []Kline) {
	key := symbol + ":" + interval
	cached := &CachedKlines{
		Data:      klines,
		Interval:  interval,
		UpdatedAt: time.Now(),
	}
	c.klines.Store(key, cached)
}

// ==================== ORDER BOOK ====================

// GetOrderBook returns cached order book for a symbol
func (c *MarketDataCache) GetOrderBook(symbol string) *OrderBookDepth {
	if val, ok := c.orderBooks.Load(symbol); ok {
		cached := val.(*CachedOrderBook)
		// Check if data is stale (older than 30 seconds - order book for UI display doesn't need to be real-time)
		if time.Since(cached.UpdatedAt) < 30*time.Second {
			c.recordHit()
			return cached.Data
		}
	}
	c.recordMiss()
	return nil
}

// GetFundingRate returns cached funding rate for a symbol (from mark price stream)
func (c *MarketDataCache) GetFundingRate(symbol string) *FundingRate {
	if val, ok := c.markPrices.Load(symbol); ok {
		cached := val.(*CachedMarkPrice)
		// Funding rate updates every 8 hours, so 5 minute cache is fine
		if time.Since(cached.UpdatedAt) < 5*time.Minute {
			c.recordHit()
			return &FundingRate{
				Symbol:          cached.Data.Symbol,
				FundingRate:     cached.Data.LastFundingRate,
				FundingTime:     cached.Data.Time,
				NextFundingTime: cached.Data.NextFundingTime,
			}
		}
	}
	c.recordMiss()
	return nil
}

// UpdateOrderBook updates the cached order book
func (c *MarketDataCache) UpdateOrderBook(symbol string, bids, asks [][]string) {
	cached := &CachedOrderBook{
		Data: &OrderBookDepth{
			Bids: bids,
			Asks: asks,
		},
		UpdatedAt: time.Now(),
	}
	c.orderBooks.Store(symbol, cached)
}

// ==================== STATISTICS ====================

func (c *MarketDataCache) recordHit() {
	c.statsMu.Lock()
	c.hitCount++
	c.statsMu.Unlock()
}

func (c *MarketDataCache) recordMiss() {
	c.statsMu.Lock()
	c.missCount++
	c.statsMu.Unlock()
}

// GetStats returns cache hit/miss statistics
func (c *MarketDataCache) GetStats() (hits, misses int64, hitRate float64) {
	c.statsMu.RLock()
	defer c.statsMu.RUnlock()
	hits = c.hitCount
	misses = c.missCount
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}
	return
}

// ClearStats resets the cache statistics
func (c *MarketDataCache) ClearStats() {
	c.statsMu.Lock()
	c.hitCount = 0
	c.missCount = 0
	c.statsMu.Unlock()
}

// Clear removes all cached data
func (c *MarketDataCache) Clear() {
	c.markPrices = sync.Map{}
	c.klines = sync.Map{}
	c.orderBooks = sync.Map{}
}
