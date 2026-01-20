// Package cache provides market data caching for Epic 10 Story 10.1
// Caches candles and indicators in Redis for millisecond access during position monitoring
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// MarketDataCache provides Redis-based caching for market data
type MarketDataCache struct {
	cacheService *CacheService
}

// Redis key patterns for market data
const (
	// Kline/Candle keys
	PrefixKline        = "kline:%s:%s"        // kline:{symbol}:{interval}
	PrefixKlineCurrent = "kline:%s:%s:current" // Current forming candle

	// Indicator keys
	PrefixIndicator = "ind:%s:%s" // ind:{symbol}:{indicator_name}
	PrefixPrice     = "price:%s"  // Current price + timestamp

	// Active positions index
	PrefixActivePositions = "active:%s" // active:{user_id} -> Set of symbols
)

// Default TTLs for market data
const (
	IndicatorTTL = 5 * time.Minute  // Indicators refresh frequently
	PriceTTL     = 30 * time.Second // Price is very short-lived
	KlineTTL     = 24 * time.Hour   // Candles kept for 24 hours
)

// MaxCandles defines rolling window sizes per interval
var MaxCandles = map[string]int{
	"1m":  1440, // 24 hours
	"5m":  288,  // 24 hours
	"15m": 96,   // 24 hours
	"1h":  24,   // 24 hours
	"4h":  6,    // 24 hours
}

// CachedKline represents a cached candlestick
type CachedKline struct {
	OpenTime  int64   `json:"ot"`
	Open      float64 `json:"o"`
	High      float64 `json:"h"`
	Low       float64 `json:"l"`
	Close     float64 `json:"c"`
	Volume    float64 `json:"v"`
	CloseTime int64   `json:"ct"`
	IsFinal   bool    `json:"f"`
}

// CachedIndicators represents cached technical indicators for a symbol
type CachedIndicators struct {
	Symbol    string  `json:"symbol"`
	Timestamp int64   `json:"ts"`
	RSI       float64 `json:"rsi"`
	ADX       float64 `json:"adx"`
	PlusDI    float64 `json:"plus_di"`
	MinusDI   float64 `json:"minus_di"`
	ATR       float64 `json:"atr"`
	ATRPct    float64 `json:"atr_pct"` // ATR as percentage of price
	EMA9      float64 `json:"ema9"`
	EMA21     float64 `json:"ema21"`
	MACD      float64 `json:"macd"`
	MACDSignal float64 `json:"macd_sig"`
	MACDHist  float64 `json:"macd_hist"`
}

// CachedPrice represents the latest price for a symbol
type CachedPrice struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"ts"`
}

// NewMarketDataCache creates a new market data cache
func NewMarketDataCache(cs *CacheService) *MarketDataCache {
	return &MarketDataCache{
		cacheService: cs,
	}
}

// =====================================================
// PRICE CACHING
// =====================================================

// SetPrice caches the current price for a symbol
func (m *MarketDataCache) SetPrice(ctx context.Context, symbol string, price float64) error {
	key := fmt.Sprintf(PrefixPrice, symbol)

	data := CachedPrice{
		Symbol:    symbol,
		Price:     price,
		Timestamp: time.Now().UnixMilli(),
	}

	return m.cacheService.Set(ctx, key, data, PriceTTL)
}

// GetPrice retrieves the cached price for a symbol
func (m *MarketDataCache) GetPrice(ctx context.Context, symbol string) (*CachedPrice, error) {
	key := fmt.Sprintf(PrefixPrice, symbol)

	result, err := m.cacheService.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var price CachedPrice
	if err := json.Unmarshal([]byte(result), &price); err != nil {
		return nil, fmt.Errorf("failed to unmarshal price: %w", err)
	}

	return &price, nil
}

// =====================================================
// INDICATOR CACHING
// =====================================================

// SetIndicator caches a single indicator value
func (m *MarketDataCache) SetIndicator(ctx context.Context, symbol, indicator string, value float64) error {
	key := fmt.Sprintf(PrefixIndicator, symbol, indicator)
	return m.cacheService.Set(ctx, key, value, IndicatorTTL)
}

// GetIndicator retrieves a single cached indicator value
func (m *MarketDataCache) GetIndicator(ctx context.Context, symbol, indicator string) (float64, error) {
	key := fmt.Sprintf(PrefixIndicator, symbol, indicator)

	result, err := m.cacheService.Get(ctx, key)
	if err != nil {
		return 0, err
	}

	var value float64
	if err := json.Unmarshal([]byte(result), &value); err != nil {
		return 0, fmt.Errorf("failed to unmarshal indicator: %w", err)
	}

	return value, nil
}

// SetIndicators caches all indicators for a symbol
func (m *MarketDataCache) SetIndicators(ctx context.Context, ind *CachedIndicators) error {
	// Cache individual indicators for fast single-value access
	indicators := map[string]float64{
		"rsi":      ind.RSI,
		"adx":      ind.ADX,
		"plus_di":  ind.PlusDI,
		"minus_di": ind.MinusDI,
		"atr":      ind.ATR,
		"atr_pct":  ind.ATRPct,
		"ema9":     ind.EMA9,
		"ema21":    ind.EMA21,
		"macd":     ind.MACD,
		"macd_sig": ind.MACDSignal,
		"macd_hist": ind.MACDHist,
	}

	for name, value := range indicators {
		if err := m.SetIndicator(ctx, ind.Symbol, name, value); err != nil {
			log.Printf("[MARKET-CACHE] Failed to cache %s for %s: %v", name, ind.Symbol, err)
		}
	}

	// Also cache the complete struct for batch access
	key := fmt.Sprintf(PrefixIndicator, ind.Symbol, "all")
	return m.cacheService.Set(ctx, key, ind, IndicatorTTL)
}

// GetIndicators retrieves all cached indicators for a symbol
func (m *MarketDataCache) GetIndicators(ctx context.Context, symbol string) (*CachedIndicators, error) {
	key := fmt.Sprintf(PrefixIndicator, symbol, "all")

	result, err := m.cacheService.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var ind CachedIndicators
	if err := json.Unmarshal([]byte(result), &ind); err != nil {
		return nil, fmt.Errorf("failed to unmarshal indicators: %w", err)
	}

	return &ind, nil
}

// =====================================================
// KLINE CACHING (Using Redis Sorted Sets)
// =====================================================

// AddKline adds a closed candle to the sorted set
func (m *MarketDataCache) AddKline(ctx context.Context, symbol, interval string, kline *CachedKline) error {
	if !m.cacheService.IsHealthy() {
		return fmt.Errorf("redis unavailable")
	}

	key := fmt.Sprintf(PrefixKline, symbol, interval)

	data, err := json.Marshal(kline)
	if err != nil {
		return fmt.Errorf("failed to marshal kline: %w", err)
	}

	// Add to sorted set with close time as score
	err = m.cacheService.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(kline.CloseTime),
		Member: string(data),
	}).Err()
	if err != nil {
		m.cacheService.recordFailure()
		return fmt.Errorf("failed to add kline: %w", err)
	}

	// Set TTL on first add
	m.cacheService.client.Expire(ctx, key, KlineTTL)

	// Trim to max size
	maxCandles := MaxCandles[interval]
	if maxCandles == 0 {
		maxCandles = 100 // Default
	}
	m.cacheService.client.ZRemRangeByRank(ctx, key, 0, int64(-maxCandles-1))

	m.cacheService.recordSuccess()
	return nil
}

// GetKlines retrieves the last N candles for a symbol/interval
func (m *MarketDataCache) GetKlines(ctx context.Context, symbol, interval string, count int) ([]CachedKline, error) {
	if !m.cacheService.IsHealthy() {
		return nil, fmt.Errorf("redis unavailable")
	}

	key := fmt.Sprintf(PrefixKline, symbol, interval)

	// Get last N entries (highest scores = most recent)
	results, err := m.cacheService.client.ZRevRange(ctx, key, 0, int64(count-1)).Result()
	if err != nil {
		m.cacheService.recordFailure()
		return nil, fmt.Errorf("failed to get klines: %w", err)
	}

	m.cacheService.recordSuccess()

	candles := make([]CachedKline, 0, len(results))
	for _, data := range results {
		var kline CachedKline
		if err := json.Unmarshal([]byte(data), &kline); err != nil {
			continue // Skip malformed entries
		}
		candles = append(candles, kline)
	}

	return candles, nil
}

// SetCurrentKline caches the currently forming candle
func (m *MarketDataCache) SetCurrentKline(ctx context.Context, symbol, interval string, kline *CachedKline) error {
	key := fmt.Sprintf(PrefixKlineCurrent, symbol, interval)
	return m.cacheService.Set(ctx, key, kline, 2*time.Minute) // Short TTL for current candle
}

// GetCurrentKline retrieves the currently forming candle
func (m *MarketDataCache) GetCurrentKline(ctx context.Context, symbol, interval string) (*CachedKline, error) {
	key := fmt.Sprintf(PrefixKlineCurrent, symbol, interval)

	result, err := m.cacheService.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var kline CachedKline
	if err := json.Unmarshal([]byte(result), &kline); err != nil {
		return nil, fmt.Errorf("failed to unmarshal current kline: %w", err)
	}

	return &kline, nil
}

// =====================================================
// ACTIVE POSITIONS TRACKING
// =====================================================

// AddActivePosition adds a symbol to user's active positions set
func (m *MarketDataCache) AddActivePosition(ctx context.Context, userID, symbol string) error {
	if !m.cacheService.IsHealthy() {
		return fmt.Errorf("redis unavailable")
	}

	key := fmt.Sprintf(PrefixActivePositions, userID)

	err := m.cacheService.client.SAdd(ctx, key, symbol).Err()
	if err != nil {
		m.cacheService.recordFailure()
		return fmt.Errorf("failed to add active position: %w", err)
	}

	// Set 7-day TTL
	m.cacheService.client.Expire(ctx, key, 7*24*time.Hour)

	m.cacheService.recordSuccess()
	return nil
}

// RemoveActivePosition removes a symbol from user's active positions set
func (m *MarketDataCache) RemoveActivePosition(ctx context.Context, userID, symbol string) error {
	if !m.cacheService.IsHealthy() {
		return fmt.Errorf("redis unavailable")
	}

	key := fmt.Sprintf(PrefixActivePositions, userID)

	err := m.cacheService.client.SRem(ctx, key, symbol).Err()
	if err != nil {
		m.cacheService.recordFailure()
		return fmt.Errorf("failed to remove active position: %w", err)
	}

	m.cacheService.recordSuccess()
	return nil
}

// GetActivePositions retrieves all active position symbols for a user
func (m *MarketDataCache) GetActivePositions(ctx context.Context, userID string) ([]string, error) {
	if !m.cacheService.IsHealthy() {
		return nil, fmt.Errorf("redis unavailable")
	}

	key := fmt.Sprintf(PrefixActivePositions, userID)

	symbols, err := m.cacheService.client.SMembers(ctx, key).Result()
	if err != nil {
		m.cacheService.recordFailure()
		return nil, fmt.Errorf("failed to get active positions: %w", err)
	}

	m.cacheService.recordSuccess()
	return symbols, nil
}

// =====================================================
// HELPER FUNCTIONS
// =====================================================

// GetIndicatorsForDecision retrieves indicators needed for exit decisions
// Returns a map for easy access: rsi, adx, atr_pct, etc.
func (m *MarketDataCache) GetIndicatorsForDecision(ctx context.Context, symbol string) (map[string]float64, error) {
	indicators := make(map[string]float64)

	names := []string{"rsi", "adx", "plus_di", "minus_di", "atr_pct", "ema9", "ema21"}

	for _, name := range names {
		value, err := m.GetIndicator(ctx, symbol, name)
		if err != nil {
			// Use 0 as default if not found
			indicators[name] = 0
			continue
		}
		indicators[name] = value
	}

	return indicators, nil
}

// InvalidateSymbolCache removes all cached data for a symbol
func (m *MarketDataCache) InvalidateSymbolCache(ctx context.Context, symbol string) error {
	if !m.cacheService.IsHealthy() {
		return fmt.Errorf("redis unavailable")
	}

	// Keys to delete
	keys := []string{
		fmt.Sprintf(PrefixPrice, symbol),
		fmt.Sprintf(PrefixIndicator, symbol, "all"),
		fmt.Sprintf(PrefixIndicator, symbol, "rsi"),
		fmt.Sprintf(PrefixIndicator, symbol, "adx"),
		fmt.Sprintf(PrefixIndicator, symbol, "atr_pct"),
	}

	// Add kline keys for all intervals
	for interval := range MaxCandles {
		keys = append(keys, fmt.Sprintf(PrefixKline, symbol, interval))
		keys = append(keys, fmt.Sprintf(PrefixKlineCurrent, symbol, interval))
	}

	for _, key := range keys {
		m.cacheService.client.Del(ctx, key)
	}

	return nil
}
