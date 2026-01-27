// Package indicators provides technical indicator calculations for research infrastructure.
// Story 15.4: Feature Calculator Engine - Volatility Features
package indicators

// VolatilityFeatures holds all volatility-related features for a candle.
// These features capture market volatility and price bands.
type VolatilityFeatures struct {
	// Average True Range
	ATR7       float64 `json:"atr_7"`       // 7-period ATR
	ATR14      float64 `json:"atr_14"`      // 14-period ATR
	ATR21      float64 `json:"atr_21"`      // 21-period ATR
	ATRPercent float64 `json:"atr_percent"` // ATR as % of close price

	// Bollinger Bands
	BollingerUpper    float64 `json:"bollinger_upper"`    // Upper band (SMA + 2*StdDev)
	BollingerLower    float64 `json:"bollinger_lower"`    // Lower band (SMA - 2*StdDev)
	BollingerWidth    float64 `json:"bollinger_width"`    // Band width as % of middle
	BollingerPosition float64 `json:"bollinger_position"` // Price position in bands (0-1)

	// Additional volatility metrics
	RangeExpansion  float64 `json:"range_expansion"`  // Current range / Average range
	VolatilityRatio float64 `json:"volatility_ratio"` // Short-term vol / Long-term vol
}

// CalculateTrueRange calculates the True Range for a single candle.
// TR = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
func CalculateTrueRange(high, low, prevClose float64) float64 {
	tr1 := high - low
	tr2 := Abs(high - prevClose)
	tr3 := Abs(low - prevClose)

	return Max(tr1, Max(tr2, tr3))
}

// CalculateATR calculates the Average True Range.
// Uses simple moving average of True Range.
func CalculateATR(highs, lows, closes []float64, period int) float64 {
	if len(highs) < period+1 || len(lows) < period+1 || len(closes) < period+1 {
		return NaN
	}

	n := len(closes)
	if len(highs) != n || len(lows) != n {
		return NaN
	}

	// Calculate True Range for the period
	trSum := 0.0
	startIdx := n - period

	for i := startIdx; i < n; i++ {
		tr := CalculateTrueRange(highs[i], lows[i], closes[i-1])
		trSum += tr
	}

	return trSum / float64(period)
}

// CalculateATRPercent calculates ATR as a percentage of the current close.
func CalculateATRPercent(atr, close float64) float64 {
	if close == 0 {
		return NaN
	}
	return (atr / close) * 100
}

// BollingerBands holds Bollinger Band values.
type BollingerBands struct {
	Upper  float64
	Middle float64
	Lower  float64
	Width  float64
}

// CalculateBollingerBands calculates Bollinger Bands.
// period: typically 20, stdDevMultiplier: typically 2.0
func CalculateBollingerBands(closes []float64, period int, stdDevMultiplier float64) *BollingerBands {
	if len(closes) < period {
		return &BollingerBands{
			Upper:  NaN,
			Middle: NaN,
			Lower:  NaN,
			Width:  NaN,
		}
	}

	// Calculate SMA (middle band)
	startIdx := len(closes) - period
	sum := 0.0
	for i := startIdx; i < len(closes); i++ {
		sum += closes[i]
	}
	middle := sum / float64(period)

	// Calculate standard deviation using sample variance (n-1 denominator)
	// This provides an unbiased estimator for the population variance
	variance := 0.0
	for i := startIdx; i < len(closes); i++ {
		diff := closes[i] - middle
		variance += diff * diff
	}
	// Use sample standard deviation (period-1) for unbiased estimation
	// Guard against period=1 edge case
	divisor := float64(period - 1)
	if divisor < 1 {
		divisor = 1
	}
	stdDev := Sqrt(variance / divisor)

	// Calculate bands
	upper := middle + (stdDev * stdDevMultiplier)
	lower := middle - (stdDev * stdDevMultiplier)

	// Calculate width as percentage of middle
	width := NaN
	if middle != 0 {
		width = ((upper - lower) / middle) * 100
	}

	return &BollingerBands{
		Upper:  upper,
		Middle: middle,
		Lower:  lower,
		Width:  width,
	}
}

// CalculateBollingerPosition calculates where price is within the bands.
// Returns 0 at lower band, 1 at upper band, can be outside [0,1].
func CalculateBollingerPosition(price float64, bb *BollingerBands) float64 {
	bandRange := bb.Upper - bb.Lower
	if bandRange == 0 || IsNaN(bb.Upper) || IsNaN(bb.Lower) {
		return NaN
	}
	return (price - bb.Lower) / bandRange
}

// CalculateRangeExpansion calculates current range relative to average range.
func CalculateRangeExpansion(highs, lows []float64, period int) float64 {
	n := len(highs)
	if n < period+1 || len(lows) != n {
		return NaN
	}

	// Calculate average range for previous 'period' candles
	startIdx := n - period - 1
	endIdx := n - 1

	rangeSum := 0.0
	for i := startIdx; i < endIdx; i++ {
		rangeSum += highs[i] - lows[i]
	}
	avgRange := rangeSum / float64(period)

	if avgRange == 0 {
		return NaN
	}

	// Current range
	currentRange := highs[n-1] - lows[n-1]

	return currentRange / avgRange
}

// CalculateVolatilityRatio calculates short-term vs long-term volatility.
// Uses ATR ratio.
func CalculateVolatilityRatio(highs, lows, closes []float64, shortPeriod, longPeriod int) float64 {
	shortATR := CalculateATR(highs, lows, closes, shortPeriod)
	longATR := CalculateATR(highs, lows, closes, longPeriod)

	if IsNaN(shortATR) || IsNaN(longATR) || longATR == 0 {
		return NaN
	}

	return shortATR / longATR
}

// CalculateVolatilityFeatures calculates all volatility features.
func CalculateVolatilityFeatures(candles []OHLCV) *VolatilityFeatures {
	if len(candles) == 0 {
		return &VolatilityFeatures{
			ATR7:              NaN,
			ATR14:             NaN,
			ATR21:             NaN,
			ATRPercent:        NaN,
			BollingerUpper:    NaN,
			BollingerLower:    NaN,
			BollingerWidth:    NaN,
			BollingerPosition: NaN,
			RangeExpansion:    NaN,
			VolatilityRatio:   NaN,
		}
	}

	// Extract data arrays
	n := len(candles)
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)

	for i, c := range candles {
		closes[i] = c.Close
		highs[i] = c.High
		lows[i] = c.Low
	}

	current := candles[n-1]

	// Calculate ATRs
	atr7 := CalculateATR(highs, lows, closes, 7)
	atr14 := CalculateATR(highs, lows, closes, 14)
	atr21 := CalculateATR(highs, lows, closes, 21)

	// Use ATR14 for percent calculation (standard)
	atrPercent := CalculateATRPercent(atr14, current.Close)

	// Calculate Bollinger Bands (20 period, 2 std dev)
	bb := CalculateBollingerBands(closes, 20, 2.0)

	features := &VolatilityFeatures{
		// ATR
		ATR7:       atr7,
		ATR14:      atr14,
		ATR21:      atr21,
		ATRPercent: atrPercent,

		// Bollinger Bands
		BollingerUpper:    bb.Upper,
		BollingerLower:    bb.Lower,
		BollingerWidth:    bb.Width,
		BollingerPosition: CalculateBollingerPosition(current.Close, bb),

		// Additional metrics
		RangeExpansion:  CalculateRangeExpansion(highs, lows, 14),
		VolatilityRatio: CalculateVolatilityRatio(highs, lows, closes, 7, 21),
	}

	return features
}
