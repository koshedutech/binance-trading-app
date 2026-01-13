package autopilot

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/strategy"
)

const BTCCacheTTL = 5 * time.Minute

// BTCTrendCache caches BTC trend to avoid repeated API calls
type BTCTrendCache struct {
	trend     string // "bullish" or "bearish" or "sideways"
	timestamp time.Time
	timeframe string
	mu        sync.RWMutex
}

// GetOrFetch returns cached trend or fetches fresh if expired
func (c *BTCTrendCache) GetOrFetch(timeframe string, fetchFunc func() (string, error)) (string, error) {
	c.mu.RLock()
	if time.Since(c.timestamp) < BTCCacheTTL && c.timeframe == timeframe {
		trend := c.trend
		c.mu.RUnlock()
		return trend, nil
	}
	c.mu.RUnlock()

	// Fetch fresh
	trend, err := fetchFunc()
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.trend = trend
	c.timestamp = time.Now()
	c.timeframe = timeframe
	c.mu.Unlock()

	return trend, nil
}

// HigherTFCache caches higher timeframe trend data per symbol
type HigherTFCache struct {
	cache map[string]*TrendCacheEntry
	mu    sync.RWMutex
}

// TrendCacheEntry stores cached trend data for a symbol+timeframe
type TrendCacheEntry struct {
	trend     string
	timestamp time.Time
}

// NewHigherTFCache creates a new higher timeframe cache
func NewHigherTFCache() *HigherTFCache {
	return &HigherTFCache{
		cache: make(map[string]*TrendCacheEntry),
	}
}

// Get returns cached trend if still valid (5 minute TTL)
func (c *HigherTFCache) Get(symbol, timeframe string) (string, bool) {
	key := symbol + ":" + timeframe
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists || time.Since(entry.timestamp) > BTCCacheTTL {
		return "", false
	}
	return entry.trend, true
}

// Set stores trend in cache
func (c *HigherTFCache) Set(symbol, timeframe, trend string) {
	key := symbol + ":" + timeframe
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &TrendCacheEntry{
		trend:     trend,
		timestamp: time.Now(),
	}
}

// TrendFilterValidator validates trend filters before trade entry
type TrendFilterValidator struct {
	config        *TrendFiltersConfig
	mtfConfig     *ModeMTFConfig
	btcCache      *BTCTrendCache
	higherTFCache *HigherTFCache
	futuresClient binance.FuturesClient
	logger        *slog.Logger
}

// NewTrendFilterValidator creates a new validator
func NewTrendFilterValidator(
	config *TrendFiltersConfig,
	mtfConfig *ModeMTFConfig,
	futuresClient binance.FuturesClient,
	logger *slog.Logger,
) *TrendFilterValidator {
	return &TrendFilterValidator{
		config:        config,
		mtfConfig:     mtfConfig,
		btcCache:      &BTCTrendCache{},
		higherTFCache: NewHigherTFCache(),
		futuresClient: futuresClient,
		logger:        logger,
	}
}

// ValidateAll checks all enabled filters in order (fastest first)
// Returns (passed, rejectionReason)
// Filter evaluation order: Price/EMA -> VWAP -> Higher TF -> BTC
func (v *TrendFilterValidator) ValidateAll(
	symbol string,
	direction string,
	currentPrice float64,
	ema float64,
	vwap float64,
) (bool, string) {
	// Normalize direction to uppercase for comparison
	dir := strings.ToUpper(direction)

	// 1. Price vs EMA check (instant - FASTEST)
	if v.config != nil && v.config.PriceVsEMA != nil && v.config.PriceVsEMA.Enabled {
		passed, reason := v.checkPriceVsEMA(symbol, dir, currentPrice, ema)
		if !passed {
			return false, reason
		}
	}

	// 2. VWAP check (instant - skip if vwap=0)
	if v.config != nil && v.config.VWAPFilter != nil && v.config.VWAPFilter.Enabled {
		passed, reason := v.checkVWAP(symbol, dir, currentPrice, vwap)
		if !passed {
			return false, reason
		}
	}

	// 3. Higher TF check (may need API call)
	if v.mtfConfig != nil && v.mtfConfig.Enabled {
		passed, reason := v.checkHigherTF(symbol, dir)
		if !passed {
			return false, reason
		}
	}

	// 4. BTC trend check (may need API call - uses cache, skip for BTCUSDT)
	if v.config != nil && v.config.BTCTrendCheck != nil && v.config.BTCTrendCheck.Enabled {
		passed, reason := v.checkBTCTrend(symbol, dir)
		if !passed {
			return false, reason
		}
	}

	v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: All filters PASSED for %s entry", symbol, dir))
	return true, ""
}

// checkPriceVsEMA validates price position relative to EMA
// For LONG: price must be >= EMA
// For SHORT: price must be <= EMA
func (v *TrendFilterValidator) checkPriceVsEMA(symbol, direction string, currentPrice, ema float64) (bool, string) {
	cfg := v.config.PriceVsEMA

	// Skip if EMA is 0 (not calculated)
	if ema <= 0 {
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: price_vs_ema SKIPPED - EMA not available", symbol))
		return true, ""
	}

	if direction == "LONG" && cfg.RequirePriceAboveEMAForLong {
		if currentPrice < ema {
			reason := fmt.Sprintf("Price %.4f below EMA%d (%.4f) for LONG entry",
				currentPrice, cfg.EMAPeriod, ema)
			v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: price_vs_ema BLOCKED - %s", symbol, reason))
			return false, reason
		}
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: price_vs_ema PASSED - Price %.4f >= EMA%d (%.4f)",
			symbol, currentPrice, cfg.EMAPeriod, ema))
	}

	if direction == "SHORT" && cfg.RequirePriceBelowEMAForShort {
		if currentPrice > ema {
			reason := fmt.Sprintf("Price %.4f above EMA%d (%.4f) for SHORT entry",
				currentPrice, cfg.EMAPeriod, ema)
			v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: price_vs_ema BLOCKED - %s", symbol, reason))
			return false, reason
		}
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: price_vs_ema PASSED - Price %.4f <= EMA%d (%.4f)",
			symbol, currentPrice, cfg.EMAPeriod, ema))
	}

	return true, ""
}

// checkVWAP validates price position relative to VWAP
// For LONG: price must be above VWAP (or within tolerance)
// For SHORT: price must be below VWAP (or within tolerance)
// Skip if VWAP is 0 or unavailable (don't block)
func (v *TrendFilterValidator) checkVWAP(symbol, direction string, currentPrice, vwap float64) (bool, string) {
	cfg := v.config.VWAPFilter

	// Skip if VWAP is 0 (not available) - don't block
	if vwap <= 0 {
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: vwap_filter SKIPPED - VWAP not available", symbol))
		return true, ""
	}

	// Calculate tolerance band
	toleranceAmount := vwap * (cfg.NearVWAPTolerancePercent / 100.0)
	vwapLower := vwap - toleranceAmount
	vwapUpper := vwap + toleranceAmount

	if direction == "LONG" && cfg.RequirePriceAboveVWAPForLong {
		// Price must be above VWAP lower band (VWAP - tolerance)
		if currentPrice < vwapLower {
			reason := fmt.Sprintf("Price %.4f below VWAP (%.4f, tolerance %.2f%%) for LONG entry",
				currentPrice, vwap, cfg.NearVWAPTolerancePercent)
			v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: vwap_filter BLOCKED - %s", symbol, reason))
			return false, reason
		}
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: vwap_filter PASSED - Price %.4f >= VWAP band (%.4f)",
			symbol, currentPrice, vwapLower))
	}

	if direction == "SHORT" && cfg.RequirePriceBelowVWAPForShort {
		// Price must be below VWAP upper band (VWAP + tolerance)
		if currentPrice > vwapUpper {
			reason := fmt.Sprintf("Price %.4f above VWAP (%.4f, tolerance %.2f%%) for SHORT entry",
				currentPrice, vwap, cfg.NearVWAPTolerancePercent)
			v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: vwap_filter BLOCKED - %s", symbol, reason))
			return false, reason
		}
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: vwap_filter PASSED - Price %.4f <= VWAP band (%.4f)",
			symbol, currentPrice, vwapUpper))
	}

	return true, ""
}

// checkHigherTF validates higher timeframe trend alignment
// Uses the primary timeframe from MTF config (e.g., 1h for scalp)
func (v *TrendFilterValidator) checkHigherTF(symbol, direction string) (bool, string) {
	// MTF must be enabled and have a primary timeframe configured
	if v.mtfConfig == nil || !v.mtfConfig.Enabled || v.mtfConfig.PrimaryTimeframe == "" {
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: higher_tf SKIPPED - MTF not configured", symbol))
		return true, ""
	}

	higherTF := v.mtfConfig.PrimaryTimeframe

	// Check cache first
	if cachedTrend, found := v.higherTFCache.Get(symbol, higherTF); found {
		return v.evaluateHigherTFTrend(symbol, direction, higherTF, cachedTrend)
	}

	// Fetch klines for higher timeframe
	klines, err := v.futuresClient.GetFuturesKlines(symbol, higherTF, 100)
	if err != nil {
		v.logger.Error(fmt.Sprintf("[TREND-FILTER] %s: higher_tf ERROR fetching %s klines: %v", symbol, higherTF, err))
		// On error, allow trade (don't block due to API issues)
		return true, ""
	}

	if len(klines) < 50 {
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: higher_tf SKIPPED - insufficient klines (%d)", symbol, len(klines)))
		return true, ""
	}

	// Use EMA 20/50 for trend detection (matching story requirements)
	trend := strategy.DetectTrend(klines, 20, 50)
	trendStr := string(trend)

	// Cache the result
	v.higherTFCache.Set(symbol, higherTF, trendStr)

	return v.evaluateHigherTFTrend(symbol, direction, higherTF, trendStr)
}

// evaluateHigherTFTrend checks if higher TF trend aligns with trade direction
func (v *TrendFilterValidator) evaluateHigherTFTrend(symbol, direction, timeframe, trend string) (bool, string) {
	// UPTREND = bullish, DOWNTREND = bearish, SIDEWAYS = neutral
	isBullish := trend == string(strategy.TrendUp)
	isBearish := trend == string(strategy.TrendDown)

	if direction == "LONG" && isBearish {
		reason := fmt.Sprintf("%s trend bearish (%s), blocking LONG entry", timeframe, trend)
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: higher_tf BLOCKED - %s", symbol, reason))
		return false, reason
	}

	if direction == "SHORT" && isBullish {
		reason := fmt.Sprintf("%s trend bullish (%s), blocking SHORT entry", timeframe, trend)
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: higher_tf BLOCKED - %s", symbol, reason))
		return false, reason
	}

	v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: higher_tf PASSED - %s trend %s aligns with %s",
		symbol, timeframe, trend, direction))
	return true, ""
}

// checkBTCTrend validates BTC trend for altcoin trades
// Block altcoin LONG when BTC is bearish
// Block altcoin SHORT when BTC is bullish
// Bypass for BTCUSDT itself
func (v *TrendFilterValidator) checkBTCTrend(symbol, direction string) (bool, string) {
	cfg := v.config.BTCTrendCheck

	// Bypass for BTC itself
	if strings.HasPrefix(strings.ToUpper(symbol), "BTC") {
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: btc_trend SKIPPED - BTC symbol bypasses BTC check", symbol))
		return true, ""
	}

	btcSymbol := cfg.BTCSymbol
	if btcSymbol == "" {
		btcSymbol = "BTCUSDT"
	}

	timeframe := cfg.BTCTrendTimeframe
	if timeframe == "" {
		timeframe = "15m"
	}

	// Use cache to fetch BTC trend
	btcTrend, err := v.btcCache.GetOrFetch(timeframe, func() (string, error) {
		return v.fetchBTCTrend(btcSymbol, timeframe)
	})

	if err != nil {
		v.logger.Error(fmt.Sprintf("[TREND-FILTER] %s: btc_trend ERROR fetching BTC trend: %v", symbol, err))
		// On error, allow trade (don't block due to API issues)
		return true, ""
	}

	// Evaluate BTC trend vs direction
	btcBullish := btcTrend == string(strategy.TrendUp)
	btcBearish := btcTrend == string(strategy.TrendDown)

	if direction == "LONG" && cfg.BlockAltLongWhenBTCBearish && btcBearish {
		reason := fmt.Sprintf("BTC trend bearish on %s, blocking altcoin LONG", timeframe)
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: btc_trend BLOCKED - %s", symbol, reason))
		return false, reason
	}

	if direction == "SHORT" && cfg.BlockAltShortWhenBTCBullish && btcBullish {
		reason := fmt.Sprintf("BTC trend bullish on %s, blocking altcoin SHORT", timeframe)
		v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: btc_trend BLOCKED - %s", symbol, reason))
		return false, reason
	}

	v.logger.Info(fmt.Sprintf("[TREND-FILTER] %s: btc_trend PASSED - BTC %s trend %s compatible with %s",
		symbol, timeframe, btcTrend, direction))
	return true, ""
}

// fetchBTCTrend fetches and analyzes BTC trend from klines
func (v *TrendFilterValidator) fetchBTCTrend(btcSymbol, timeframe string) (string, error) {
	klines, err := v.futuresClient.GetFuturesKlines(btcSymbol, timeframe, 100)
	if err != nil {
		return "", fmt.Errorf("failed to fetch BTC klines: %w", err)
	}

	if len(klines) < 50 {
		return string(strategy.TrendSideways), nil
	}

	// Use EMA 20/50 for BTC trend detection
	trend := strategy.DetectTrend(klines, 20, 50)
	return string(trend), nil
}

// UpdateConfig updates the validator's configuration
// Called when settings are reloaded
func (v *TrendFilterValidator) UpdateConfig(config *TrendFiltersConfig, mtfConfig *ModeMTFConfig) {
	v.config = config
	v.mtfConfig = mtfConfig
}

// ClearCache clears all cached trend data
// Useful when settings change or for testing
func (v *TrendFilterValidator) ClearCache() {
	v.btcCache.mu.Lock()
	v.btcCache.trend = ""
	v.btcCache.timestamp = time.Time{}
	v.btcCache.mu.Unlock()

	v.higherTFCache.mu.Lock()
	v.higherTFCache.cache = make(map[string]*TrendCacheEntry)
	v.higherTFCache.mu.Unlock()
}
