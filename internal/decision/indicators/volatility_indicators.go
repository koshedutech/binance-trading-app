// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"fmt"
	"math"
)

// ATRIndicator wraps the Average True Range for volatility assessment.
type ATRIndicator struct {
	BaseIndicator
	Period int
}

// NewATRIndicator creates a new ATR indicator with default 14 period.
func NewATRIndicator() *ATRIndicator {
	return &ATRIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "atr",
			IndicatorSegment: SegmentVolatility,
			IndicatorDeps:    []string{},
		},
		Period: 14,
	}
}

// Description returns a human-readable explanation.
func (a *ATRIndicator) Description() string {
	return fmt.Sprintf("ATR %d period. Measures price volatility as average range.", a.Period)
}

// Calculate computes the ATR value.
func (a *ATRIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated ATR
	if atr, hasATR := data.GetOtherValue("atr"); hasATR {
		// Normalize relative to current price
		normalizedATR := a.normalizeWithPrice(atr, data.Price)
		return NewIndicatorValue(a.Name(), atr, normalizedATR), nil
	}

	if !data.HasSufficientData(a.Period + 1) {
		return nil, fmt.Errorf("insufficient data for ATR calculation: need %d periods", a.Period+1)
	}

	// Need high, low, close for True Range calculation
	if len(data.Highs) < a.Period+1 || len(data.Lows) < a.Period+1 {
		return nil, fmt.Errorf("high/low data required for ATR calculation")
	}

	closes := data.Closes
	if len(closes) == 0 {
		closes = data.Prices
	}

	atr := calculateATR(data.Highs, data.Lows, closes, a.Period)
	normalizedATR := a.normalizeWithPrice(atr, data.Price)

	return NewIndicatorValue(a.Name(), atr, normalizedATR), nil
}

// normalizeWithPrice normalizes ATR as percentage of price.
func (a *ATRIndicator) normalizeWithPrice(atr, price float64) float64 {
	if price <= 0 {
		return 50
	}

	// ATR as percentage of price
	atrPercent := atr / price * 100

	return a.Normalize(atrPercent)
}

// Normalize converts ATR percentage to 0-100 volatility score.
// Higher volatility = higher score (for volatility segment).
func (a *ATRIndicator) Normalize(value float64) float64 {
	// ATR as % of price typically ranges from 0.5% to 5% for crypto
	// Map to 0-100 where higher volatility = higher score

	const minATR = 0.5
	const maxATR = 5.0

	if value <= minATR {
		return 10 // Very low volatility
	}
	if value >= maxATR {
		return 100 // Very high volatility
	}

	// Linear mapping
	return 10 + (value-minATR)/(maxATR-minATR)*90
}

// BollingerWidthIndicator calculates Bollinger Band width as percentage of price.
type BollingerWidthIndicator struct {
	BaseIndicator
	Period     int
	StdDevMult float64
}

// NewBollingerWidthIndicator creates a new Bollinger Width indicator.
func NewBollingerWidthIndicator() *BollingerWidthIndicator {
	return &BollingerWidthIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "bollinger_width",
			IndicatorSegment: SegmentVolatility,
			IndicatorDeps:    []string{},
		},
		Period:     20,
		StdDevMult: 2.0,
	}
}

// Description returns a human-readable explanation.
func (b *BollingerWidthIndicator) Description() string {
	return fmt.Sprintf("Bollinger Band width (%d, %.1f). Narrow = low volatility, wide = high volatility.", b.Period, b.StdDevMult)
}

// Calculate computes the Bollinger Band width.
func (b *BollingerWidthIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated values
	if width, hasBB := data.GetOtherValue("bollinger_width"); hasBB {
		return NewIndicatorValue(b.Name(), width, b.Normalize(width)), nil
	}

	if !data.HasSufficientData(b.Period) {
		return nil, fmt.Errorf("insufficient data for Bollinger calculation: need %d periods", b.Period)
	}

	prices := data.Closes
	if len(prices) == 0 {
		prices = data.Prices
	}

	// Calculate middle band (SMA)
	n := len(prices)
	start := n - b.Period

	var sum float64
	for i := start; i < n; i++ {
		sum += prices[i]
	}
	sma := sum / float64(b.Period)

	// Calculate standard deviation
	var sumSquares float64
	for i := start; i < n; i++ {
		diff := prices[i] - sma
		sumSquares += diff * diff
	}
	stdDev := math.Sqrt(sumSquares / float64(b.Period))

	// Calculate band width as percentage of middle band
	upperBand := sma + b.StdDevMult*stdDev
	lowerBand := sma - b.StdDevMult*stdDev

	var width float64
	if sma > 0 {
		width = (upperBand - lowerBand) / sma * 100
	}

	normalized := b.Normalize(width)

	return NewIndicatorValue(b.Name(), width, normalized), nil
}

// Normalize converts Bollinger width to 0-100 scale.
func (b *BollingerWidthIndicator) Normalize(value float64) float64 {
	// Bollinger width typically ranges from 2% to 15% for crypto
	// Map to 0-100

	const minWidth = 2.0
	const maxWidth = 15.0

	if value <= minWidth {
		return 10
	}
	if value >= maxWidth {
		return 100
	}

	return 10 + (value-minWidth)/(maxWidth-minWidth)*90
}

// KeltnerWidthIndicator calculates Keltner Channel width.
type KeltnerWidthIndicator struct {
	BaseIndicator
	EMAPeriod  int
	ATRPeriod  int
	ATRMult    float64
}

// NewKeltnerWidthIndicator creates a new Keltner Width indicator.
func NewKeltnerWidthIndicator() *KeltnerWidthIndicator {
	return &KeltnerWidthIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "keltner_width",
			IndicatorSegment: SegmentVolatility,
			IndicatorDeps:    []string{"atr"},
		},
		EMAPeriod: 20,
		ATRPeriod: 10,
		ATRMult:   2.0,
	}
}

// Description returns a human-readable explanation.
func (k *KeltnerWidthIndicator) Description() string {
	return fmt.Sprintf("Keltner Channel width (EMA %d, ATR %d x %.1f). ATR-based volatility measure.", k.EMAPeriod, k.ATRPeriod, k.ATRMult)
}

// Calculate computes the Keltner Channel width.
func (k *KeltnerWidthIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated values
	if width, hasKW := data.GetOtherValue("keltner_width"); hasKW {
		return NewIndicatorValue(k.Name(), width, k.Normalize(width)), nil
	}

	// Need ATR
	atr, hasATR := data.GetOtherValue("atr")
	if !hasATR {
		return nil, fmt.Errorf("ATR is required for Keltner calculation")
	}

	if !data.HasSufficientData(k.EMAPeriod) {
		return nil, fmt.Errorf("insufficient data for Keltner calculation: need %d periods", k.EMAPeriod)
	}

	prices := data.Closes
	if len(prices) == 0 {
		prices = data.Prices
	}

	// Calculate EMA (middle line)
	ema := calculateEMA(prices, k.EMAPeriod)

	// Calculate channel width as percentage of EMA
	channelWidth := 2 * k.ATRMult * atr

	var widthPercent float64
	if ema > 0 {
		widthPercent = channelWidth / ema * 100
	}

	normalized := k.Normalize(widthPercent)

	return NewIndicatorValue(k.Name(), widthPercent, normalized), nil
}

// Normalize converts Keltner width to 0-100 scale.
func (k *KeltnerWidthIndicator) Normalize(value float64) float64 {
	// Similar to Bollinger, typically 2% to 12% for crypto
	const minWidth = 2.0
	const maxWidth = 12.0

	if value <= minWidth {
		return 10
	}
	if value >= maxWidth {
		return 100
	}

	return 10 + (value-minWidth)/(maxWidth-minWidth)*90
}

// HistoricalVolatilityIndicator calculates price volatility over a period.
type HistoricalVolatilityIndicator struct {
	BaseIndicator
	Period int
}

// NewHistoricalVolatilityIndicator creates a new historical volatility indicator.
func NewHistoricalVolatilityIndicator() *HistoricalVolatilityIndicator {
	return &HistoricalVolatilityIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "historical_volatility",
			IndicatorSegment: SegmentVolatility,
			IndicatorDeps:    []string{},
		},
		Period: 20,
	}
}

// Description returns a human-readable explanation.
func (h *HistoricalVolatilityIndicator) Description() string {
	return fmt.Sprintf("Historical volatility over %d periods (annualized).", h.Period)
}

// Calculate computes historical volatility.
func (h *HistoricalVolatilityIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	if !data.HasSufficientData(h.Period + 1) {
		return nil, fmt.Errorf("insufficient data for volatility calculation: need %d periods", h.Period+1)
	}

	prices := data.Closes
	if len(prices) == 0 {
		prices = data.Prices
	}

	n := len(prices)
	start := n - h.Period

	// Calculate log returns
	returns := make([]float64, h.Period-1)
	for i := 0; i < h.Period-1; i++ {
		if prices[start+i] > 0 {
			returns[i] = math.Log(prices[start+i+1] / prices[start+i])
		}
	}

	// Calculate standard deviation of returns
	var sumReturns float64
	for _, r := range returns {
		sumReturns += r
	}
	meanReturn := sumReturns / float64(len(returns))

	var sumSquares float64
	for _, r := range returns {
		diff := r - meanReturn
		sumSquares += diff * diff
	}
	stdDev := math.Sqrt(sumSquares / float64(len(returns)))

	// Annualize (assuming daily data, ~365 trading days for crypto)
	annualizedVol := stdDev * math.Sqrt(365) * 100

	normalized := h.Normalize(annualizedVol)

	return NewIndicatorValue(h.Name(), annualizedVol, normalized), nil
}

// Normalize converts historical volatility to 0-100 scale.
func (h *HistoricalVolatilityIndicator) Normalize(value float64) float64 {
	// Annualized volatility for crypto typically 20% to 150%
	const minVol = 20.0
	const maxVol = 150.0

	if value <= minVol {
		return 10
	}
	if value >= maxVol {
		return 100
	}

	return 10 + (value-minVol)/(maxVol-minVol)*90
}

// Helper function to calculate ATR
func calculateATR(highs, lows, closes []float64, period int) float64 {
	if len(highs) < period+1 || len(lows) < period+1 || len(closes) < period+1 {
		return 0
	}

	n := len(closes)
	// Ensure highs and lows arrays are at least as long as closes
	if len(highs) < n || len(lows) < n {
		return 0
	}

	start := n - period

	var sumTR float64
	for i := start; i < n; i++ {
		// True Range = max(H-L, |H-C_prev|, |L-C_prev|)
		hl := highs[i] - lows[i]
		hc := math.Abs(highs[i] - closes[i-1])
		lc := math.Abs(lows[i] - closes[i-1])

		tr := hl
		if hc > tr {
			tr = hc
		}
		if lc > tr {
			tr = lc
		}

		sumTR += tr
	}

	return sumTR / float64(period)
}

// init registers volatility indicators with the global registry.
func init() {
	registry := GetGlobalIndicatorRegistry()

	// Register all volatility indicators, log errors if any
	indicators := []Indicator{
		NewATRIndicator(),
		NewBollingerWidthIndicator(),
		NewKeltnerWidthIndicator(),
		NewHistoricalVolatilityIndicator(),
	}

	for _, ind := range indicators {
		if err := registry.Register(ind); err != nil {
			// Log registration errors - in production this would use a proper logger
			// For now, panic on startup to surface misconfigurations early
			panic(fmt.Sprintf("failed to register volatility indicator %s: %v", ind.Name(), err))
		}
	}
}
