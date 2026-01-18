// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"fmt"
	"math"
)

// EMACrossIndicator detects EMA 9/21 crossover signals.
// Returns: bullish=100, bearish=0, neutral=50
type EMACrossIndicator struct {
	BaseIndicator
	ShortPeriod int
	LongPeriod  int
}

// NewEMACrossIndicator creates a new EMA cross indicator with default 9/21 periods.
func NewEMACrossIndicator() *EMACrossIndicator {
	return &EMACrossIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "ema_cross",
			IndicatorSegment: SegmentTrend,
			IndicatorDeps:    []string{},
		},
		ShortPeriod: 9,
		LongPeriod:  21,
	}
}

// Description returns a human-readable explanation.
func (e *EMACrossIndicator) Description() string {
	return fmt.Sprintf("EMA %d/%d crossover detection. Bullish when short EMA > long EMA.", e.ShortPeriod, e.LongPeriod)
}

// Calculate computes the EMA cross value.
func (e *EMACrossIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated EMA values
	shortEMA, hasShort := data.GetOtherValue(fmt.Sprintf("ema_%d", e.ShortPeriod))
	longEMA, hasLong := data.GetOtherValue(fmt.Sprintf("ema_%d", e.LongPeriod))

	// If not pre-calculated, calculate from prices
	if !hasShort || !hasLong {
		if !data.HasSufficientData(e.LongPeriod + 1) {
			return nil, fmt.Errorf("insufficient data for EMA calculation: need %d periods", e.LongPeriod+1)
		}

		prices := data.Closes
		if len(prices) == 0 {
			prices = data.Prices
		}

		shortEMA = calculateEMA(prices, e.ShortPeriod)
		longEMA = calculateEMA(prices, e.LongPeriod)
	}

	// Calculate raw value: difference as percentage
	var rawValue float64
	if longEMA > 0 {
		rawValue = (shortEMA - longEMA) / longEMA * 100
	}

	normalized := e.Normalize(rawValue)

	return NewIndicatorValue(e.Name(), rawValue, normalized), nil
}

// Normalize converts the EMA cross value to 0-100 scale.
// Positive difference (bullish) -> 50-100
// Negative difference (bearish) -> 0-50
func (e *EMACrossIndicator) Normalize(value float64) float64 {
	// Map percentage difference to 0-100
	// -2% or worse = 0, +2% or better = 100
	const maxDiff = 2.0

	if value >= maxDiff {
		return 100
	}
	if value <= -maxDiff {
		return 0
	}

	// Linear mapping: -2 to +2 -> 0 to 100
	return 50 + (value/maxDiff)*50
}

// MACDIndicator calculates MACD histogram for trend direction and strength.
type MACDIndicator struct {
	BaseIndicator
	FastPeriod   int
	SlowPeriod   int
	SignalPeriod int
}

// NewMACDIndicator creates a new MACD indicator with standard 12/26/9 periods.
func NewMACDIndicator() *MACDIndicator {
	return &MACDIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "macd",
			IndicatorSegment: SegmentTrend,
			IndicatorDeps:    []string{},
		},
		FastPeriod:   12,
		SlowPeriod:   26,
		SignalPeriod: 9,
	}
}

// Description returns a human-readable explanation.
func (m *MACDIndicator) Description() string {
	return fmt.Sprintf("MACD %d/%d/%d histogram. Positive = bullish, negative = bearish.", m.FastPeriod, m.SlowPeriod, m.SignalPeriod)
}

// Calculate computes the MACD histogram value.
func (m *MACDIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated MACD
	if histogram, hasMACD := data.GetOtherValue("macd_histogram"); hasMACD {
		return NewIndicatorValue(m.Name(), histogram, m.Normalize(histogram)), nil
	}

	minPeriods := m.SlowPeriod + m.SignalPeriod
	if !data.HasSufficientData(minPeriods) {
		return nil, fmt.Errorf("insufficient data for MACD calculation: need %d periods", minPeriods)
	}

	prices := data.Closes
	if len(prices) == 0 {
		prices = data.Prices
	}

	// Calculate MACD line (fast EMA - slow EMA)
	fastEMA := calculateEMA(prices, m.FastPeriod)
	slowEMA := calculateEMA(prices, m.SlowPeriod)
	macdLine := fastEMA - slowEMA

	// Calculate signal line (EMA of MACD line values over signal period)
	// We need to calculate MACD values over the signal period to get the signal line
	signalLine := m.calculateSignalLine(prices)

	// Histogram = MACD line - Signal line
	histogram := macdLine - signalLine

	normalized := m.Normalize(histogram)

	return NewIndicatorValue(m.Name(), histogram, normalized), nil
}

// calculateSignalLine calculates the signal line (EMA of MACD line values).
func (m *MACDIndicator) calculateSignalLine(prices []float64) float64 {
	if len(prices) < m.SlowPeriod+m.SignalPeriod {
		return 0
	}

	// Calculate MACD line values for the signal period
	macdValues := make([]float64, m.SignalPeriod)

	for i := 0; i < m.SignalPeriod; i++ {
		// Calculate MACD at each point going back from the end
		endIdx := len(prices) - (m.SignalPeriod - 1 - i)
		if endIdx < m.SlowPeriod {
			macdValues[i] = 0
			continue
		}

		subPrices := prices[:endIdx]
		fastEMA := calculateEMA(subPrices, m.FastPeriod)
		slowEMA := calculateEMA(subPrices, m.SlowPeriod)
		macdValues[i] = fastEMA - slowEMA
	}

	// Calculate EMA of MACD values (this is the signal line)
	return calculateEMA(macdValues, m.SignalPeriod)
}

// Normalize converts MACD histogram to 0-100 scale.
func (m *MACDIndicator) Normalize(value float64) float64 {
	// Typical histogram range is -50 to +50 for most assets
	// Map this to 0-100
	const maxHistogram = 50.0

	if value >= maxHistogram {
		return 100
	}
	if value <= -maxHistogram {
		return 0
	}

	return 50 + (value/maxHistogram)*50
}

// SuperTrendIndicator calculates SuperTrend direction.
type SuperTrendIndicator struct {
	BaseIndicator
	Period     int
	Multiplier float64
}

// NewSuperTrendIndicator creates a new SuperTrend indicator.
func NewSuperTrendIndicator() *SuperTrendIndicator {
	return &SuperTrendIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "supertrend",
			IndicatorSegment: SegmentTrend,
			IndicatorDeps:    []string{"atr"},
		},
		Period:     10,
		Multiplier: 3.0,
	}
}

// Description returns a human-readable explanation.
func (s *SuperTrendIndicator) Description() string {
	return fmt.Sprintf("SuperTrend %d period, %.1f multiplier. 100 = bullish, 0 = bearish.", s.Period, s.Multiplier)
}

// Calculate computes the SuperTrend direction.
func (s *SuperTrendIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated SuperTrend
	if supertrend, hasST := data.GetOtherValue("supertrend_direction"); hasST {
		return NewIndicatorValue(s.Name(), supertrend, s.Normalize(supertrend)), nil
	}

	// Need ATR and price data
	atr, hasATR := data.GetOtherValue("atr")
	if !hasATR {
		return nil, fmt.Errorf("ATR is required for SuperTrend calculation")
	}

	if data.Price <= 0 {
		return nil, fmt.Errorf("current price is required")
	}

	// Simplified SuperTrend: check if price is above/below the bands
	// Upper band = (High + Low) / 2 + Multiplier * ATR
	// Lower band = (High + Low) / 2 - Multiplier * ATR

	var midPrice float64
	if len(data.Highs) > 0 && len(data.Lows) > 0 {
		midPrice = (data.Highs[len(data.Highs)-1] + data.Lows[len(data.Lows)-1]) / 2
	} else {
		midPrice = data.Price
	}

	upperBand := midPrice + s.Multiplier*atr
	lowerBand := midPrice - s.Multiplier*atr

	// Determine direction: price above lower band = bullish
	var rawValue float64
	if data.Price > upperBand {
		rawValue = 1.0 // Strong bullish
	} else if data.Price > lowerBand {
		rawValue = 0.5 // Neutral/weak
	} else {
		rawValue = 0.0 // Bearish
	}

	normalized := s.Normalize(rawValue)

	return NewIndicatorValue(s.Name(), rawValue, normalized), nil
}

// Normalize converts SuperTrend direction to 0-100 scale.
func (s *SuperTrendIndicator) Normalize(value float64) float64 {
	// 0 = bearish (0), 0.5 = neutral (50), 1 = bullish (100)
	return value * 100
}

// ADXTrendIndicator wraps ADX as a trend indicator.
// While ADX is typically used for trend strength, it can also indicate trend presence.
type ADXTrendIndicator struct {
	BaseIndicator
	Period int
}

// NewADXTrendIndicator creates a new ADX trend indicator.
func NewADXTrendIndicator() *ADXTrendIndicator {
	return &ADXTrendIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "adx_trend",
			IndicatorSegment: SegmentTrend,
			IndicatorDeps:    []string{},
		},
		Period: 14,
	}
}

// Description returns a human-readable explanation.
func (a *ADXTrendIndicator) Description() string {
	return "ADX trend strength indicator. >25 = trending, <20 = ranging."
}

// Calculate computes the ADX value.
func (a *ADXTrendIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated ADX
	adx, hasADX := data.GetOtherValue("adx")
	if !hasADX {
		// ADX calculation is complex; return default if not pre-calculated
		return NewIndicatorValue(a.Name(), 0, 50).WithConfidence(0.5), nil
	}

	normalized := a.Normalize(adx)

	return NewIndicatorValue(a.Name(), adx, normalized), nil
}

// Normalize converts ADX to 0-100 trend score.
// High ADX = strong trend = high score
func (a *ADXTrendIndicator) Normalize(value float64) float64 {
	// ADX range: 0-100, but typically 10-60
	// Map to 0-100 with emphasis on trending range
	if value < 15 {
		return value / 15 * 30 // Weak trend zone
	}
	if value < 25 {
		return 30 + (value-15)/10*20 // Transition zone
	}
	if value < 50 {
		return 50 + (value-25)/25*40 // Strong trend zone
	}
	return 90 + math.Min(10, (value-50)/10*10) // Very strong trend
}

// Helper function to calculate Exponential Moving Average
func calculateEMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)

	// Start with SMA for the first EMA value
	var sum float64
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	ema := sum / float64(period)

	// Calculate EMA for remaining values
	for i := period; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}

	return ema
}

// init registers trend indicators with the global registry.
func init() {
	registry := GetGlobalIndicatorRegistry()

	// Register all trend indicators, log errors if any
	indicators := []Indicator{
		NewEMACrossIndicator(),
		NewMACDIndicator(),
		NewSuperTrendIndicator(),
		NewADXTrendIndicator(),
	}

	for _, ind := range indicators {
		if err := registry.Register(ind); err != nil {
			// Log registration errors - in production this would use a proper logger
			// For now, panic on startup to surface misconfigurations early
			panic(fmt.Sprintf("failed to register trend indicator %s: %v", ind.Name(), err))
		}
	}
}
