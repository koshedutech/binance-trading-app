package patterns

import (
	"binance-trading-bot/internal/binance"
	"time"
)

// PatternType represents different chart patterns
type PatternType string

const (
	// Reversal Patterns
	MorningStar      PatternType = "morning_star"
	EveningStar      PatternType = "evening_star"
	ShootingStar     PatternType = "shooting_star"
	Hammer           PatternType = "hammer"
	HangingMan       PatternType = "hanging_man"
	BullishEngulfing PatternType = "bullish_engulfing"
	BearishEngulfing PatternType = "bearish_engulfing"
	Doji             PatternType = "doji"
	DragonflyDoji    PatternType = "dragonfly_doji"
	GravestoneDoji   PatternType = "gravestone_doji"
	BullishHarami    PatternType = "bullish_harami"
	BearishHarami    PatternType = "bearish_harami"

	// Continuation Patterns
	BullishFlag      PatternType = "bullish_flag"
	BearishFlag      PatternType = "bearish_flag"
	Pennant          PatternType = "pennant"
	AscendingTriangle PatternType = "ascending_triangle"
	DescendingTriangle PatternType = "descending_triangle"
)

// DetectedPattern represents a detected chart pattern
type DetectedPattern struct {
	Type              PatternType
	Symbol            string
	Timeframe         string
	DetectedAt        time.Time
	CandleIndex       int
	Confidence        float64 // 0.0 to 1.0
	Direction         string  // "bullish" or "bearish"
	VolumeConfirmed   bool    // Whether volume confirms the pattern
	TrendAligned      bool    // Whether pattern aligns with trend
	VolumeRatio       float64 // Current volume vs average
	TrendStrength     float64 // Strength of prevailing trend
}

// PatternDetector detects chart patterns in candlestick data
type PatternDetector struct {
	minBodySize      float64 // Minimum candle body size (% of price)
	volumeLookback   int     // Candles to look back for average volume
	trendLookback    int     // Candles to look back for trend analysis
}

// NewPatternDetector creates a new pattern detector
func NewPatternDetector(minBodySize float64) *PatternDetector {
	if minBodySize <= 0 {
		minBodySize = 0.5 // Default 0.5%
	}
	return &PatternDetector{
		minBodySize:    minBodySize,
		volumeLookback: 20, // Default 20 candles for volume average
		trendLookback:  14, // Default 14 candles for trend analysis
	}
}

// NewPatternDetectorWithConfig creates a pattern detector with custom config
func NewPatternDetectorWithConfig(minBodySize float64, volumeLookback, trendLookback int) *PatternDetector {
	if minBodySize <= 0 {
		minBodySize = 0.5
	}
	if volumeLookback <= 0 {
		volumeLookback = 20
	}
	if trendLookback <= 0 {
		trendLookback = 14
	}
	return &PatternDetector{
		minBodySize:    minBodySize,
		volumeLookback: volumeLookback,
		trendLookback:  trendLookback,
	}
}

// DetectPatterns scans for all supported patterns
func (pd *PatternDetector) DetectPatterns(symbol, timeframe string, candles []binance.Kline) []DetectedPattern {
	var patterns []DetectedPattern

	if len(candles) < 3 {
		return patterns
	}

	// Check for 3-candle patterns
	for i := 2; i < len(candles); i++ {
		c1, c2, c3 := candles[i-2], candles[i-1], candles[i]

		// Morning Star
		if pd.isMorningStar(c1, c2, c3) {
			volumeConfirmed, volumeRatio := pd.calculateVolumeConfirmation(candles, i, "bullish")
			trendAligned, trendStrength := pd.calculateTrendAlignment(candles, i, "bullish")
			confidence := pd.calculateConfidenceWithContext(c1, c2, c3, MorningStar, volumeConfirmed, trendAligned, volumeRatio, trendStrength)

			patterns = append(patterns, DetectedPattern{
				Type:            MorningStar,
				Symbol:          symbol,
				Timeframe:       timeframe,
				DetectedAt:      time.Unix(c3.CloseTime/1000, 0),
				CandleIndex:     i,
				Confidence:      confidence,
				Direction:       "bullish",
				VolumeConfirmed: volumeConfirmed,
				TrendAligned:    trendAligned,
				VolumeRatio:     volumeRatio,
				TrendStrength:   trendStrength,
			})
		}

		// Evening Star
		if pd.isEveningStar(c1, c2, c3) {
			volumeConfirmed, volumeRatio := pd.calculateVolumeConfirmation(candles, i, "bearish")
			trendAligned, trendStrength := pd.calculateTrendAlignment(candles, i, "bearish")
			confidence := pd.calculateConfidenceWithContext(c1, c2, c3, EveningStar, volumeConfirmed, trendAligned, volumeRatio, trendStrength)

			patterns = append(patterns, DetectedPattern{
				Type:            EveningStar,
				Symbol:          symbol,
				Timeframe:       timeframe,
				DetectedAt:      time.Unix(c3.CloseTime/1000, 0),
				CandleIndex:     i,
				Confidence:      confidence,
				Direction:       "bearish",
				VolumeConfirmed: volumeConfirmed,
				TrendAligned:    trendAligned,
				VolumeRatio:     volumeRatio,
				TrendStrength:   trendStrength,
			})
		}
	}

	// Check for single candle patterns
	for i := 0; i < len(candles); i++ {
		candle := candles[i]
		var prevCandle *binance.Kline
		if i > 0 {
			prevCandle = &candles[i-1]
		}

		// Shooting Star
		if pd.isShootingStar(candle, prevCandle) {
			volumeConfirmed, volumeRatio := pd.calculateVolumeConfirmation(candles, i, "bearish")
			trendAligned, trendStrength := pd.calculateTrendAlignment(candles, i, "bearish")
			confidence := pd.calculateSingleCandleConfidenceWithContext(candle, ShootingStar, volumeConfirmed, trendAligned, volumeRatio, trendStrength)

			patterns = append(patterns, DetectedPattern{
				Type:            ShootingStar,
				Symbol:          symbol,
				Timeframe:       timeframe,
				DetectedAt:      time.Unix(candle.CloseTime/1000, 0),
				CandleIndex:     i,
				Confidence:      confidence,
				Direction:       "bearish",
				VolumeConfirmed: volumeConfirmed,
				TrendAligned:    trendAligned,
				VolumeRatio:     volumeRatio,
				TrendStrength:   trendStrength,
			})
		}

		// Hammer
		if pd.isHammer(candle, prevCandle) {
			volumeConfirmed, volumeRatio := pd.calculateVolumeConfirmation(candles, i, "bullish")
			trendAligned, trendStrength := pd.calculateTrendAlignment(candles, i, "bullish")
			confidence := pd.calculateSingleCandleConfidenceWithContext(candle, Hammer, volumeConfirmed, trendAligned, volumeRatio, trendStrength)

			patterns = append(patterns, DetectedPattern{
				Type:            Hammer,
				Symbol:          symbol,
				Timeframe:       timeframe,
				DetectedAt:      time.Unix(candle.CloseTime/1000, 0),
				CandleIndex:     i,
				Confidence:      confidence,
				Direction:       "bullish",
				VolumeConfirmed: volumeConfirmed,
				TrendAligned:    trendAligned,
				VolumeRatio:     volumeRatio,
				TrendStrength:   trendStrength,
			})
		}
	}

	return patterns
}

// isMorningStar checks for Morning Star pattern (bullish reversal)
func (pd *PatternDetector) isMorningStar(c1, c2, c3 binance.Kline) bool {
	// Candle 1: Long bearish candle
	if c1.Close >= c1.Open {
		return false
	}
	body1 := c1.Open - c1.Close
	range1 := c1.High - c1.Low
	if body1 < range1*0.6 {
		return false // Body too small
	}

	// Candle 2: Small body (indecision)
	body2 := abs(c2.Close - c2.Open)
	if body2 > body1*0.4 {
		return false // Body too large
	}

	// Candle 3: Long bullish candle
	if c3.Close <= c3.Open {
		return false
	}
	body3 := c3.Close - c3.Open
	range3 := c3.High - c3.Low
	if body3 < range3*0.6 {
		return false // Body too small
	}

	// C3 should close above midpoint of C1
	midpoint := (c1.Open + c1.Close) / 2
	if c3.Close < midpoint {
		return false
	}

	return true
}

// isEveningStar checks for Evening Star pattern (bearish reversal)
func (pd *PatternDetector) isEveningStar(c1, c2, c3 binance.Kline) bool {
	// Candle 1: Long bullish candle
	if c1.Close <= c1.Open {
		return false
	}
	body1 := c1.Close - c1.Open
	range1 := c1.High - c1.Low
	if body1 < range1*0.6 {
		return false
	}

	// Candle 2: Small body
	body2 := abs(c2.Close - c2.Open)
	if body2 > body1*0.4 {
		return false
	}

	// Candle 3: Long bearish candle
	if c3.Close >= c3.Open {
		return false
	}
	body3 := c3.Open - c3.Close
	range3 := c3.High - c3.Low
	if body3 < range3*0.6 {
		return false
	}

	// C3 should close below midpoint of C1
	midpoint := (c1.Open + c1.Close) / 2
	if c3.Close > midpoint {
		return false
	}

	return true
}

// isShootingStar checks for Shooting Star pattern (bearish reversal)
func (pd *PatternDetector) isShootingStar(candle binance.Kline, prevCandle *binance.Kline) bool {
	body := abs(candle.Close - candle.Open)
	upperWick := candle.High - max(candle.Open, candle.Close)
	lowerWick := min(candle.Open, candle.Close) - candle.Low

	// Long upper wick (at least 2x body)
	if upperWick < body*2 {
		return false
	}

	// Small or no lower wick
	if lowerWick > body*0.3 {
		return false
	}

	// Should appear after uptrend (if previous candle available)
	if prevCandle != nil && prevCandle.Close <= prevCandle.Open {
		return false
	}

	return true
}

// isHammer checks for Hammer pattern (bullish reversal)
func (pd *PatternDetector) isHammer(candle binance.Kline, prevCandle *binance.Kline) bool {
	body := abs(candle.Close - candle.Open)
	upperWick := candle.High - max(candle.Open, candle.Close)
	lowerWick := min(candle.Open, candle.Close) - candle.Low

	// Long lower wick (at least 2x body)
	if lowerWick < body*2 {
		return false
	}

	// Small or no upper wick
	if upperWick > body*0.3 {
		return false
	}

	// Should appear after downtrend (if previous candle available)
	if prevCandle != nil && prevCandle.Close >= prevCandle.Open {
		return false
	}

	return true
}

// calculateVolumeConfirmation checks if volume confirms the pattern
// Returns (isConfirmed, volumeRatio)
func (pd *PatternDetector) calculateVolumeConfirmation(candles []binance.Kline, index int, direction string) (bool, float64) {
	if index < pd.volumeLookback {
		return false, 1.0 // Not enough data
	}

	// Calculate average volume over lookback period
	var totalVolume float64
	for i := index - pd.volumeLookback; i < index; i++ {
		totalVolume += candles[i].Volume
	}
	avgVolume := totalVolume / float64(pd.volumeLookback)

	if avgVolume == 0 {
		return false, 1.0
	}

	// Current candle volume
	currentVolume := candles[index].Volume
	volumeRatio := currentVolume / avgVolume

	// Volume confirmation thresholds
	// For bullish reversals: we want to see increasing volume on the reversal candle
	// For bearish reversals: we want to see increasing volume on the breakdown
	isConfirmed := volumeRatio >= 1.5 // 50% above average volume confirms the pattern

	return isConfirmed, volumeRatio
}

// calculateTrendAlignment checks if the pattern aligns with the prevailing trend
// Returns (isAligned, trendStrength) where trendStrength is -1 to 1 (negative = downtrend, positive = uptrend)
func (pd *PatternDetector) calculateTrendAlignment(candles []binance.Kline, index int, patternDirection string) (bool, float64) {
	if index < pd.trendLookback {
		return false, 0 // Not enough data
	}

	// Calculate trend using simple linear regression on closing prices
	startIdx := index - pd.trendLookback
	endIdx := index

	// Calculate slope using least squares method
	var sumX, sumY, sumXY, sumX2 float64
	n := float64(pd.trendLookback)

	for i := startIdx; i < endIdx; i++ {
		x := float64(i - startIdx)
		y := candles[i].Close
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return false, 0
	}

	slope := (n*sumXY - sumX*sumY) / denominator

	// Normalize slope as percentage of average price
	avgPrice := sumY / n
	if avgPrice == 0 {
		return false, 0
	}
	normalizedSlope := (slope / avgPrice) * 100 // Percentage per candle

	// Determine trend strength (-1 to 1 scale)
	// Strong trend is > 0.5% per candle
	trendStrength := normalizedSlope / 0.5
	if trendStrength > 1 {
		trendStrength = 1
	} else if trendStrength < -1 {
		trendStrength = -1
	}

	// Reversal patterns work best when they occur at the end of a trend
	// Bullish reversal (like Morning Star, Hammer) should appear after downtrend
	// Bearish reversal (like Evening Star, Shooting Star) should appear after uptrend
	var isAligned bool
	if patternDirection == "bullish" {
		// Bullish reversal needs prior downtrend
		isAligned = trendStrength < -0.3 // Moderate downtrend
	} else {
		// Bearish reversal needs prior uptrend
		isAligned = trendStrength > 0.3 // Moderate uptrend
	}

	return isAligned, trendStrength
}

// calculateConfidenceWithContext calculates confidence with volume and trend context
func (pd *PatternDetector) calculateConfidenceWithContext(c1, c2, c3 binance.Kline, patternType PatternType,
	volumeConfirmed, trendAligned bool, volumeRatio, trendStrength float64) float64 {

	// Start with base confidence
	confidence := 0.5

	// Adjust based on pattern strength
	body1 := abs(c1.Close - c1.Open)
	body3 := abs(c3.Close - c3.Open)

	// Stronger confirmation candle = higher confidence
	if body3 > body1*1.2 {
		confidence += 0.1
	}
	if body3 > body1*1.5 {
		confidence += 0.05
	}

	// Volume confirmation adds significant confidence
	if volumeConfirmed {
		confidence += 0.15
	} else if volumeRatio > 1.2 {
		confidence += 0.05 // Slightly above average volume
	}

	// Trend alignment is crucial for reversal patterns
	if trendAligned {
		confidence += 0.15
	}

	// Strong trend in the right direction adds extra confidence
	absStrength := abs(trendStrength)
	if trendAligned && absStrength > 0.6 {
		confidence += 0.05
	}

	// Cap confidence at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// calculateSingleCandleConfidenceWithContext calculates confidence for single-candle patterns with context
func (pd *PatternDetector) calculateSingleCandleConfidenceWithContext(candle binance.Kline, patternType PatternType,
	volumeConfirmed, trendAligned bool, volumeRatio, trendStrength float64) float64 {

	// Base confidence for single candle patterns is lower
	confidence := 0.45

	// Calculate wick-to-body ratio for pattern quality
	body := abs(candle.Close - candle.Open)
	upperWick := candle.High - max(candle.Open, candle.Close)
	lowerWick := min(candle.Open, candle.Close) - candle.Low

	// Better formed patterns (longer wicks, smaller bodies)
	if patternType == Hammer || patternType == HangingMan {
		wickRatio := lowerWick / (body + 0.0001) // Avoid division by zero
		if wickRatio > 3 {
			confidence += 0.1
		} else if wickRatio > 2.5 {
			confidence += 0.05
		}
	} else if patternType == ShootingStar {
		wickRatio := upperWick / (body + 0.0001)
		if wickRatio > 3 {
			confidence += 0.1
		} else if wickRatio > 2.5 {
			confidence += 0.05
		}
	}

	// Volume confirmation
	if volumeConfirmed {
		confidence += 0.15
	} else if volumeRatio > 1.2 {
		confidence += 0.05
	}

	// Trend alignment
	if trendAligned {
		confidence += 0.15
	}

	// Strong trend adds extra confidence
	absStrength := abs(trendStrength)
	if trendAligned && absStrength > 0.6 {
		confidence += 0.05
	}

	// Cap confidence
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// calculateConfidence calculates pattern confidence score (legacy method)
func (pd *PatternDetector) calculateConfidence(c1, c2, c3 binance.Kline, patternType PatternType) float64 {
	// Start with base confidence
	confidence := 0.7

	// Adjust based on pattern strength
	body1 := abs(c1.Close - c1.Open)
	body3 := abs(c3.Close - c3.Open)

	// Stronger candles = higher confidence
	if body3 > body1*1.2 {
		confidence += 0.1
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// calculateSingleCandleConfidence calculates confidence for single-candle patterns (legacy method)
func (pd *PatternDetector) calculateSingleCandleConfidence(candle binance.Kline, patternType PatternType) float64 {
	return 0.65 // Base confidence for single candle patterns
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
