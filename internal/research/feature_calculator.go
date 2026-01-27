// Package research provides data structures and services for research infrastructure.
// Story 15.4: Feature Calculator Engine - Main calculator for all 70+ features
package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"binance-trading-bot/internal/cache"
	"binance-trading-bot/internal/research/indicators"
)

// CandleFeatures represents all calculated features for a single candle.
// This struct contains 70+ features across 6 categories.
type CandleFeatures struct {
	// Identification
	Symbol    string    `json:"symbol"`
	Timeframe Timeframe `json:"timeframe"`
	OpenTime  time.Time `json:"open_time"`

	// Feature categories
	Price      *indicators.PriceFeatures      `json:"price"`
	Volume     *indicators.VolumeFeatures     `json:"volume"`
	Volatility *indicators.VolatilityFeatures `json:"volatility"`
	Momentum   *indicators.MomentumFeatures   `json:"momentum"`
	Trend      *indicators.TrendFeatures      `json:"trend"`
	Time       *indicators.TimeFeatures       `json:"time"`

	// Calculation metadata
	CalculatedAt   time.Time `json:"calculated_at"`
	LookbackUsed   int       `json:"lookback_used"`   // Actual candles used
	LookbackNeeded int       `json:"lookback_needed"` // Minimum needed for all features
	IsComplete     bool      `json:"is_complete"`     // All features calculated
}

// MinimumLookback is the minimum number of candles needed to calculate all features.
// This is determined by the indicator with the longest lookback:
// - ADX needs 2*period+1 = 29 for period=14
// - SMA50 needs 50
// - MACD needs slowPeriod + signalPeriod = 26 + 9 = 35
// Using 60 to be safe and allow for warmup.
const MinimumLookback = 60

// RecommendedLookback provides additional buffer for more stable calculations.
const RecommendedLookback = 100

// FeatureCalculator calculates all features for candles.
type FeatureCalculator struct {
	cacheService *cache.CacheService
	featureCache *FeatureCache
	cacheTTL     time.Duration
}

// FeatureCalculatorConfig holds configuration for the calculator.
type FeatureCalculatorConfig struct {
	CacheTTL time.Duration // How long to cache features (legacy, used if FeatureCache not provided)
}

// DefaultFeatureCalculatorConfig returns default configuration.
func DefaultFeatureCalculatorConfig() FeatureCalculatorConfig {
	return FeatureCalculatorConfig{
		CacheTTL: 24 * time.Hour, // Cache features for 24 hours
	}
}

// NewFeatureCalculator creates a new FeatureCalculator.
func NewFeatureCalculator(cacheService *cache.CacheService, config FeatureCalculatorConfig) *FeatureCalculator {
	if config.CacheTTL == 0 {
		config.CacheTTL = 24 * time.Hour
	}

	fc := &FeatureCalculator{
		cacheService: cacheService,
		cacheTTL:     config.CacheTTL,
	}

	// Create FeatureCache if cache service is available
	if cacheService != nil {
		fc.featureCache = NewFeatureCache(cacheService, DefaultFeatureCacheConfig())
	}

	return fc
}

// NewFeatureCalculatorWithCache creates a FeatureCalculator with an explicit FeatureCache.
// This allows for custom cache configuration including version overrides.
func NewFeatureCalculatorWithCache(featureCache *FeatureCache) *FeatureCalculator {
	return &FeatureCalculator{
		featureCache: featureCache,
		cacheTTL:     24 * time.Hour,
	}
}

// GetFeatureCache returns the underlying FeatureCache for direct cache operations.
func (fc *FeatureCalculator) GetFeatureCache() *FeatureCache {
	return fc.featureCache
}

// featureCacheKey generates the cache key for features.
func featureCacheKey(symbol string, timeframe Timeframe, openTime time.Time) string {
	return fmt.Sprintf("research:features:%s:%s:%d", symbol, timeframe, openTime.Unix())
}

// Calculate calculates all features for a single candle.
// The candles slice should contain the target candle as the last element,
// with enough historical candles for lookback calculations.
func (fc *FeatureCalculator) Calculate(candles []*ResearchCandle) (*CandleFeatures, error) {
	if len(candles) == 0 {
		return nil, fmt.Errorf("no candles provided")
	}

	// Target candle is the last one
	target := candles[len(candles)-1]

	// Convert to OHLCV for indicator calculations
	ohlcv := make([]indicators.OHLCV, len(candles))
	for i, c := range candles {
		ohlcv[i] = indicators.OHLCV{
			Open:           c.Open,
			High:           c.High,
			Low:            c.Low,
			Close:          c.Close,
			Volume:         c.Volume,
			QuoteVolume:    c.QuoteVolume,
			TakerBuyVolume: c.TakerBuyVolume,
			TradeCount:     c.TradeCount,
		}
	}

	// Calculate all feature categories
	features := &CandleFeatures{
		Symbol:         target.Symbol,
		Timeframe:      target.Timeframe,
		OpenTime:       target.OpenTime,
		Price:          indicators.CalculatePriceFeatures(ohlcv),
		Volume:         indicators.CalculateVolumeFeatures(ohlcv),
		Volatility:     indicators.CalculateVolatilityFeatures(ohlcv),
		Momentum:       indicators.CalculateMomentumFeatures(ohlcv),
		Trend:          indicators.CalculateTrendFeatures(ohlcv),
		Time:           indicators.CalculateTimeFeatures(target.OpenTime),
		CalculatedAt:   time.Now(),
		LookbackUsed:   len(candles),
		LookbackNeeded: MinimumLookback,
		IsComplete:     len(candles) >= MinimumLookback,
	}

	return features, nil
}

// CalculateBatch calculates features for multiple candles efficiently.
// Returns features for each candle where the index represents the offset from
// the start of lookbackCandles (after the minimum lookback period).
//
// lookbackCandles: All candles including lookback period
// targetCount: Number of target candles to calculate features for (from the end)
func (fc *FeatureCalculator) CalculateBatch(lookbackCandles []*ResearchCandle, targetCount int) ([]*CandleFeatures, error) {
	totalCandles := len(lookbackCandles)

	if totalCandles < MinimumLookback {
		return nil, fmt.Errorf("insufficient candles: have %d, need at least %d", totalCandles, MinimumLookback)
	}

	// Adjust target count if needed
	maxTargets := totalCandles - MinimumLookback + 1
	if targetCount > maxTargets {
		targetCount = maxTargets
	}

	if targetCount <= 0 {
		return nil, fmt.Errorf("no targets to calculate after lookback requirement")
	}

	results := make([]*CandleFeatures, targetCount)

	// Calculate features for each target candle
	// Start from the earliest target that has enough lookback
	// Ensure startIdx is never negative
	startIdx := totalCandles - targetCount
	if startIdx < MinimumLookback-1 {
		startIdx = MinimumLookback - 1
	}

	for i := 0; i < targetCount; i++ {
		endIdx := startIdx + i + 1 // Include this candle
		candleSlice := lookbackCandles[:endIdx]

		features, err := fc.Calculate(candleSlice)
		if err != nil {
			log.Printf("[FEATURE_CALC] Error calculating features for candle %d: %v", i, err)
			continue
		}
		results[i] = features
	}

	return results, nil
}

// CalculateAndCache calculates features and stores them in Redis cache.
// Uses the optimized FeatureCache (HASH storage) if available, falls back to legacy JSON storage.
func (fc *FeatureCalculator) CalculateAndCache(ctx context.Context, candles []*ResearchCandle) (*CandleFeatures, error) {
	features, err := fc.Calculate(candles)
	if err != nil {
		return nil, err
	}

	// Cache the features using FeatureCache (preferred) or legacy method
	if fc.featureCache != nil {
		if err := fc.featureCache.Set(ctx, features); err != nil {
			log.Printf("[FEATURE_CALC] Failed to cache features (HASH): %v", err)
			// Don't return error, features are still valid
		}
	} else if fc.cacheService != nil {
		// Legacy fallback
		key := featureCacheKey(features.Symbol, features.Timeframe, features.OpenTime)
		if err := fc.cacheService.SetJSON(ctx, key, features, fc.cacheTTL); err != nil {
			log.Printf("[FEATURE_CALC] Failed to cache features (JSON): %v", err)
		}
	}

	return features, nil
}

// GetCachedFeatures retrieves features from cache if available.
// Uses the optimized FeatureCache (HASH storage) if available, falls back to legacy JSON storage.
func (fc *FeatureCalculator) GetCachedFeatures(ctx context.Context, symbol string, timeframe Timeframe, openTime time.Time) (*CandleFeatures, error) {
	// Try FeatureCache first (preferred)
	if fc.featureCache != nil {
		features, err := fc.featureCache.Get(ctx, symbol, timeframe, openTime)
		if err != nil {
			return nil, err
		}
		if features != nil {
			return features, nil
		}
		// Cache miss from HASH storage
		return nil, nil
	}

	// Legacy fallback
	if fc.cacheService == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	key := featureCacheKey(symbol, timeframe, openTime)
	var features CandleFeatures
	if err := fc.cacheService.GetJSON(ctx, key, &features); err != nil {
		return nil, err
	}

	return &features, nil
}

// GetCachedFeaturesBatch retrieves multiple features for a time range.
// This is optimized for backtest queries using Redis pipeline.
// Returns features in chronological order. Missing entries are nil.
func (fc *FeatureCalculator) GetCachedFeaturesBatch(ctx context.Context, symbol string, timeframe Timeframe, startTime, endTime time.Time) ([]*CandleFeatures, error) {
	if fc.featureCache == nil {
		return nil, fmt.Errorf("feature cache not available for batch retrieval")
	}

	return fc.featureCache.GetBatch(ctx, symbol, timeframe, startTime, endTime)
}

// CalculateBatchAndCache calculates and caches features for multiple candles.
// Uses the optimized FeatureCache (HASH storage with pipeline) if available.
func (fc *FeatureCalculator) CalculateBatchAndCache(ctx context.Context, lookbackCandles []*ResearchCandle, targetCount int) ([]*CandleFeatures, error) {
	features, err := fc.CalculateBatch(lookbackCandles, targetCount)
	if err != nil {
		return nil, err
	}

	// Cache all features using FeatureCache (preferred) or legacy method
	if fc.featureCache != nil {
		// Filter out nil features
		validFeatures := make([]*CandleFeatures, 0, len(features))
		for _, f := range features {
			if f != nil {
				validFeatures = append(validFeatures, f)
			}
		}
		if err := fc.featureCache.SetBatch(ctx, validFeatures); err != nil {
			log.Printf("[FEATURE_CALC] Failed to cache batch features: %v", err)
		}
	} else if fc.cacheService != nil {
		// Legacy fallback
		for _, f := range features {
			if f == nil {
				continue
			}
			key := featureCacheKey(f.Symbol, f.Timeframe, f.OpenTime)
			if err := fc.cacheService.SetJSON(ctx, key, f, fc.cacheTTL); err != nil {
				log.Printf("[FEATURE_CALC] Failed to cache features for %s: %v", f.OpenTime, err)
			}
		}
	}

	return features, nil
}

// ToFeatureVector converts CandleFeatures to a flat slice of float64 values.
// This is useful for ML models that expect numeric arrays.
// Returns the values and a slice of feature names in the same order.
func (cf *CandleFeatures) ToFeatureVector() ([]float64, []string) {
	values := make([]float64, 0, 70)
	names := make([]string, 0, 70)

	// Helper to add feature safely
	addFeature := func(name string, value float64) {
		// Replace NaN with 0 for ML compatibility
		if math.IsNaN(value) {
			value = 0
		}
		values = append(values, value)
		names = append(names, name)
	}

	addFeatureInt := func(name string, value int) {
		values = append(values, float64(value))
		names = append(names, name)
	}

	addFeatureBool := func(name string, value bool) {
		v := 0.0
		if value {
			v = 1.0
		}
		values = append(values, v)
		names = append(names, name)
	}

	// Price features (15)
	if cf.Price != nil {
		addFeature("return_1c", cf.Price.Return1c)
		addFeature("return_3c", cf.Price.Return3c)
		addFeature("return_5c", cf.Price.Return5c)
		addFeature("return_10c", cf.Price.Return10c)
		addFeature("return_20c", cf.Price.Return20c)
		addFeature("body_ratio", cf.Price.BodyRatio)
		addFeature("upper_wick_ratio", cf.Price.UpperWickRatio)
		addFeature("lower_wick_ratio", cf.Price.LowerWickRatio)
		addFeature("position_in_range", cf.Price.PositionInRange)
		addFeature("gap_percent", cf.Price.GapPercent)
		addFeature("range_percent", cf.Price.RangePercent)
		addFeatureInt("consecutive_up", cf.Price.ConsecutiveUp)
		addFeatureInt("consecutive_down", cf.Price.ConsecutiveDown)
		addFeatureBool("higher_high", cf.Price.HigherHigh)
		addFeatureBool("lower_low", cf.Price.LowerLow)
	}

	// Volume features (12)
	if cf.Volume != nil {
		addFeature("volume_ratio_5", cf.Volume.VolumeRatio5)
		addFeature("volume_ratio_10", cf.Volume.VolumeRatio10)
		addFeature("volume_ratio_20", cf.Volume.VolumeRatio20)
		addFeature("volume_trend_5", cf.Volume.VolumeTrend5)
		addFeature("taker_buy_ratio", cf.Volume.TakerBuyRatio)
		addFeature("trade_count_value", cf.Volume.TradeCountValue)
		addFeature("avg_trade_size", cf.Volume.AvgTradeSize)
		addFeature("obv", cf.Volume.OBV)
		addFeature("obv_trend", cf.Volume.OBVTrend)
		addFeature("volume_price_trend", cf.Volume.VolumePriceTrend)
		addFeature("accum_dist", cf.Volume.AccumDist)
		addFeature("money_flow", cf.Volume.MoneyFlow)
	}

	// Volatility features (10)
	if cf.Volatility != nil {
		addFeature("atr_7", cf.Volatility.ATR7)
		addFeature("atr_14", cf.Volatility.ATR14)
		addFeature("atr_21", cf.Volatility.ATR21)
		addFeature("atr_percent", cf.Volatility.ATRPercent)
		addFeature("bollinger_upper", cf.Volatility.BollingerUpper)
		addFeature("bollinger_lower", cf.Volatility.BollingerLower)
		addFeature("bollinger_width", cf.Volatility.BollingerWidth)
		addFeature("bollinger_position", cf.Volatility.BollingerPosition)
		addFeature("range_expansion", cf.Volatility.RangeExpansion)
		addFeature("volatility_ratio", cf.Volatility.VolatilityRatio)
	}

	// Momentum features (15)
	if cf.Momentum != nil {
		addFeature("rsi_7", cf.Momentum.RSI7)
		addFeature("rsi_14", cf.Momentum.RSI14)
		addFeature("rsi_21", cf.Momentum.RSI21)
		addFeature("stoch_k", cf.Momentum.StochK)
		addFeature("stoch_d", cf.Momentum.StochD)
		addFeature("macd_line", cf.Momentum.MACDLine)
		addFeature("macd_signal", cf.Momentum.MACDSignal)
		addFeature("macd_histogram", cf.Momentum.MACDHistogram)
		addFeature("roc_5", cf.Momentum.ROC5)
		addFeature("roc_10", cf.Momentum.ROC10)
		addFeature("roc_20", cf.Momentum.ROC20)
		addFeature("momentum_5", cf.Momentum.Momentum5)
		addFeature("momentum_10", cf.Momentum.Momentum10)
		addFeature("williams_r", cf.Momentum.WilliamsR)
	}

	// Trend features (12)
	if cf.Trend != nil {
		addFeature("sma_10", cf.Trend.SMA10)
		addFeature("sma_20", cf.Trend.SMA20)
		addFeature("sma_50", cf.Trend.SMA50)
		addFeature("ema_9", cf.Trend.EMA9)
		addFeature("ema_21", cf.Trend.EMA21)
		addFeature("price_vs_sma_20", cf.Trend.PriceVsSMA20)
		addFeatureInt("sma_cross", cf.Trend.SMACross)
		addFeature("adx", cf.Trend.ADX)
		addFeature("plus_di", cf.Trend.PlusDI)
		addFeature("minus_di", cf.Trend.MinusDI)
		addFeature("trend_strength", cf.Trend.TrendStrength)
		addFeatureInt("trend_direction", cf.Trend.TrendDirection)
	}

	// Time features (6)
	if cf.Time != nil {
		addFeatureInt("hour_of_day", cf.Time.HourOfDay)
		addFeatureInt("day_of_week", cf.Time.DayOfWeek)
		addFeatureBool("is_weekend", cf.Time.IsWeekend)
		addFeatureInt("session", indicators.SessionToInt(cf.Time.Session))
		addFeatureInt("day_of_month", cf.Time.DayOfMonth)
		addFeatureBool("is_month_end", cf.Time.IsMonthEnd)
	}

	return values, names
}

// ToJSON serializes CandleFeatures to JSON.
func (cf *CandleFeatures) ToJSON() ([]byte, error) {
	return json.Marshal(cf)
}

// FromJSON deserializes JSON to CandleFeatures.
func FromJSON(data []byte) (*CandleFeatures, error) {
	var cf CandleFeatures
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	return &cf, nil
}

// FeatureSummary provides a summary of the calculated features.
type FeatureSummary struct {
	TotalFeatures   int  `json:"total_features"`
	CompleteFeatures int `json:"complete_features"` // Non-NaN features
	HasPriceFeatures bool `json:"has_price_features"`
	HasVolumeFeatures bool `json:"has_volume_features"`
	HasVolatilityFeatures bool `json:"has_volatility_features"`
	HasMomentumFeatures bool `json:"has_momentum_features"`
	HasTrendFeatures bool `json:"has_trend_features"`
	HasTimeFeatures bool `json:"has_time_features"`
}

// GetSummary returns a summary of the calculated features.
// Note: This counts raw feature values before NaN-to-0 conversion.
func (cf *CandleFeatures) GetSummary() *FeatureSummary {
	// Count features directly from structs to get accurate NaN count
	// (ToFeatureVector converts NaN to 0, so we can't use it for counting)
	totalFeatures := 0
	completeFeatures := 0

	// Helper to count non-NaN float64 values
	countValid := func(values ...float64) (total, valid int) {
		for _, v := range values {
			total++
			if !math.IsNaN(v) {
				valid++
			}
		}
		return total, valid
	}

	// Price features (15 total: 5 floats returns, 4 floats ratios, 2 floats gap/range, 2 ints, 2 bools)
	if cf.Price != nil {
		t, v := countValid(cf.Price.Return1c, cf.Price.Return3c, cf.Price.Return5c,
			cf.Price.Return10c, cf.Price.Return20c, cf.Price.BodyRatio,
			cf.Price.UpperWickRatio, cf.Price.LowerWickRatio, cf.Price.PositionInRange,
			cf.Price.GapPercent, cf.Price.RangePercent)
		totalFeatures += t + 4 // +4 for ConsecutiveUp, ConsecutiveDown, HigherHigh, LowerLow (never NaN)
		completeFeatures += v + 4
	}

	// Volume features (12 total)
	if cf.Volume != nil {
		t, v := countValid(cf.Volume.VolumeRatio5, cf.Volume.VolumeRatio10, cf.Volume.VolumeRatio20,
			cf.Volume.VolumeTrend5, cf.Volume.TakerBuyRatio, cf.Volume.TradeCountValue,
			cf.Volume.AvgTradeSize, cf.Volume.OBV, cf.Volume.OBVTrend,
			cf.Volume.VolumePriceTrend, cf.Volume.AccumDist, cf.Volume.MoneyFlow)
		totalFeatures += t
		completeFeatures += v
	}

	// Volatility features (10 total)
	if cf.Volatility != nil {
		t, v := countValid(cf.Volatility.ATR7, cf.Volatility.ATR14, cf.Volatility.ATR21,
			cf.Volatility.ATRPercent, cf.Volatility.BollingerUpper, cf.Volatility.BollingerLower,
			cf.Volatility.BollingerWidth, cf.Volatility.BollingerPosition,
			cf.Volatility.RangeExpansion, cf.Volatility.VolatilityRatio)
		totalFeatures += t
		completeFeatures += v
	}

	// Momentum features (14 total)
	if cf.Momentum != nil {
		t, v := countValid(cf.Momentum.RSI7, cf.Momentum.RSI14, cf.Momentum.RSI21,
			cf.Momentum.StochK, cf.Momentum.StochD, cf.Momentum.MACDLine,
			cf.Momentum.MACDSignal, cf.Momentum.MACDHistogram, cf.Momentum.ROC5,
			cf.Momentum.ROC10, cf.Momentum.ROC20, cf.Momentum.Momentum5,
			cf.Momentum.Momentum10, cf.Momentum.WilliamsR)
		totalFeatures += t
		completeFeatures += v
	}

	// Trend features (12 total: 5 floats MAs, 1 float PriceVsSMA, 1 int SMACross, 4 floats ADX/DI, 1 float TrendStrength, 1 int TrendDirection)
	if cf.Trend != nil {
		t, v := countValid(cf.Trend.SMA10, cf.Trend.SMA20, cf.Trend.SMA50,
			cf.Trend.EMA9, cf.Trend.EMA21, cf.Trend.PriceVsSMA20,
			cf.Trend.ADX, cf.Trend.PlusDI, cf.Trend.MinusDI, cf.Trend.TrendStrength)
		totalFeatures += t + 2 // +2 for SMACross, TrendDirection (ints, never NaN)
		completeFeatures += v + 2
	}

	// Time features (6 total: all ints/bools, never NaN)
	if cf.Time != nil {
		totalFeatures += 6
		completeFeatures += 6
	}

	return &FeatureSummary{
		TotalFeatures:         totalFeatures,
		CompleteFeatures:      completeFeatures,
		HasPriceFeatures:      cf.Price != nil,
		HasVolumeFeatures:     cf.Volume != nil,
		HasVolatilityFeatures: cf.Volatility != nil,
		HasMomentumFeatures:   cf.Momentum != nil,
		HasTrendFeatures:      cf.Trend != nil,
		HasTimeFeatures:       cf.Time != nil,
	}
}
