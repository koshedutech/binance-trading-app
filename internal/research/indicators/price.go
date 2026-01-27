// Package indicators provides technical indicator calculations for research infrastructure.
// Story 15.4: Feature Calculator Engine - Price Features
package indicators

// PriceFeatures holds all price-related features for a candle.
// These features capture price action characteristics.
type PriceFeatures struct {
	// Returns - Price change percentages over different periods
	Return1c  float64 `json:"return_1c"`  // 1-candle return (%)
	Return3c  float64 `json:"return_3c"`  // 3-candle return (%)
	Return5c  float64 `json:"return_5c"`  // 5-candle return (%)
	Return10c float64 `json:"return_10c"` // 10-candle return (%)
	Return20c float64 `json:"return_20c"` // 20-candle return (%)

	// Candle structure
	BodyRatio       float64 `json:"body_ratio"`        // Body size / Range (0-1)
	UpperWickRatio  float64 `json:"upper_wick_ratio"`  // Upper wick / Range (0-1)
	LowerWickRatio  float64 `json:"lower_wick_ratio"`  // Lower wick / Range (0-1)
	PositionInRange float64 `json:"position_in_range"` // Close position in range (0-1)

	// Gap and range
	GapPercent   float64 `json:"gap_percent"`   // Gap from previous close (%)
	RangePercent float64 `json:"range_percent"` // (High-Low)/Open (%)

	// Pattern detection
	ConsecutiveUp   int  `json:"consecutive_up"`   // Count of consecutive bullish candles
	ConsecutiveDown int  `json:"consecutive_down"` // Count of consecutive bearish candles
	HigherHigh      bool `json:"higher_high"`      // Current high > previous high
	LowerLow        bool `json:"lower_low"`        // Current low < previous low
}

// CalculateReturn calculates the percentage return over n candles.
// Returns NaN if insufficient data.
func CalculateReturn(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return NaN
	}
	currentIdx := len(closes) - 1
	pastIdx := currentIdx - period

	return SafePercent(closes[currentIdx], closes[pastIdx])
}

// CalculateBodyRatio calculates the body ratio (body/range).
// Returns 0 for doji candles, NaN for invalid data.
func CalculateBodyRatio(open, high, low, close float64) float64 {
	candleRange := high - low
	if candleRange == 0 {
		return 0 // Doji
	}
	body := Abs(close - open)
	return body / candleRange
}

// CalculateUpperWickRatio calculates the upper wick ratio.
// Returns 0 if no range.
func CalculateUpperWickRatio(open, high, low, close float64) float64 {
	candleRange := high - low
	if candleRange == 0 {
		return 0
	}

	var upperWick float64
	if close > open { // Bullish
		upperWick = high - close
	} else { // Bearish
		upperWick = high - open
	}

	return upperWick / candleRange
}

// CalculateLowerWickRatio calculates the lower wick ratio.
// Returns 0 if no range.
func CalculateLowerWickRatio(open, high, low, close float64) float64 {
	candleRange := high - low
	if candleRange == 0 {
		return 0
	}

	var lowerWick float64
	if close > open { // Bullish
		lowerWick = open - low
	} else { // Bearish
		lowerWick = close - low
	}

	return lowerWick / candleRange
}

// CalculatePositionInRange calculates where the close is within the candle range.
// Returns 1 for highest point, 0 for lowest, 0.5 for middle.
func CalculatePositionInRange(high, low, close float64) float64 {
	candleRange := high - low
	if candleRange == 0 {
		return 0.5 // No range, consider it middle
	}
	return (close - low) / candleRange
}

// CalculateGapPercent calculates the gap from the previous close.
func CalculateGapPercent(currentOpen, previousClose float64) float64 {
	return SafePercent(currentOpen, previousClose)
}

// CalculateRangePercent calculates the range as percentage of open.
func CalculateRangePercent(open, high, low float64) float64 {
	if open == 0 {
		return NaN
	}
	return (high - low) / open * 100
}

// CalculateConsecutive calculates consecutive up/down candle counts.
// Returns (consecutiveUp, consecutiveDown)
func CalculateConsecutive(opens, closes []float64) (int, int) {
	if len(opens) == 0 || len(closes) == 0 || len(opens) != len(closes) {
		return 0, 0
	}

	consecutiveUp := 0
	consecutiveDown := 0

	// Count from most recent candle backwards
	for i := len(closes) - 1; i >= 0; i-- {
		if closes[i] > opens[i] {
			// Bullish candle
			if consecutiveDown > 0 {
				break // Streak ended
			}
			consecutiveUp++
		} else if closes[i] < opens[i] {
			// Bearish candle
			if consecutiveUp > 0 {
				break // Streak ended
			}
			consecutiveDown++
		} else {
			// Doji - doesn't break streak but doesn't count
			if consecutiveUp > 0 || consecutiveDown > 0 {
				break
			}
		}
	}

	return consecutiveUp, consecutiveDown
}

// CalculateHigherHigh checks if current high is higher than previous high.
func CalculateHigherHigh(highs []float64) bool {
	if len(highs) < 2 {
		return false
	}
	return highs[len(highs)-1] > highs[len(highs)-2]
}

// CalculateLowerLow checks if current low is lower than previous low.
func CalculateLowerLow(lows []float64) bool {
	if len(lows) < 2 {
		return false
	}
	return lows[len(lows)-1] < lows[len(lows)-2]
}

// CalculatePriceFeatures calculates all price features for the current candle.
// Requires OHLCV data for multiple candles (lookback).
func CalculatePriceFeatures(candles []OHLCV) *PriceFeatures {
	if len(candles) == 0 {
		return &PriceFeatures{
			Return1c:  NaN,
			Return3c:  NaN,
			Return5c:  NaN,
			Return10c: NaN,
			Return20c: NaN,
		}
	}

	// Extract closes for return calculations
	closes := make([]float64, len(candles))
	opens := make([]float64, len(candles))
	highs := make([]float64, len(candles))
	lows := make([]float64, len(candles))

	for i, c := range candles {
		closes[i] = c.Close
		opens[i] = c.Open
		highs[i] = c.High
		lows[i] = c.Low
	}

	current := candles[len(candles)-1]
	features := &PriceFeatures{
		// Returns
		Return1c:  CalculateReturn(closes, 1),
		Return3c:  CalculateReturn(closes, 3),
		Return5c:  CalculateReturn(closes, 5),
		Return10c: CalculateReturn(closes, 10),
		Return20c: CalculateReturn(closes, 20),

		// Candle structure
		BodyRatio:       CalculateBodyRatio(current.Open, current.High, current.Low, current.Close),
		UpperWickRatio:  CalculateUpperWickRatio(current.Open, current.High, current.Low, current.Close),
		LowerWickRatio:  CalculateLowerWickRatio(current.Open, current.High, current.Low, current.Close),
		PositionInRange: CalculatePositionInRange(current.High, current.Low, current.Close),

		// Range
		RangePercent: CalculateRangePercent(current.Open, current.High, current.Low),

		// Pattern detection
		HigherHigh: CalculateHigherHigh(highs),
		LowerLow:   CalculateLowerLow(lows),
	}

	// Gap from previous candle
	if len(candles) >= 2 {
		previousClose := candles[len(candles)-2].Close
		features.GapPercent = CalculateGapPercent(current.Open, previousClose)
	} else {
		features.GapPercent = NaN
	}

	// Consecutive candle counts
	features.ConsecutiveUp, features.ConsecutiveDown = CalculateConsecutive(opens, closes)

	return features
}
