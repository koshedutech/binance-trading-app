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
	Type        PatternType
	Symbol      string
	Timeframe   string
	DetectedAt  time.Time
	CandleIndex int
	Confidence  float64 // 0.0 to 1.0
	Direction   string  // "bullish" or "bearish"
}

// PatternDetector detects chart patterns in candlestick data
type PatternDetector struct {
	minBodySize float64 // Minimum candle body size (% of price)
}

// NewPatternDetector creates a new pattern detector
func NewPatternDetector(minBodySize float64) *PatternDetector {
	if minBodySize <= 0 {
		minBodySize = 0.5 // Default 0.5%
	}
	return &PatternDetector{
		minBodySize: minBodySize,
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
			patterns = append(patterns, DetectedPattern{
				Type:        MorningStar,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(c3.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  pd.calculateConfidence(c1, c2, c3, MorningStar),
				Direction:   "bullish",
			})
		}

		// Evening Star
		if pd.isEveningStar(c1, c2, c3) {
			patterns = append(patterns, DetectedPattern{
				Type:        EveningStar,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(c3.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  pd.calculateConfidence(c1, c2, c3, EveningStar),
				Direction:   "bearish",
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
			patterns = append(patterns, DetectedPattern{
				Type:        ShootingStar,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(candle.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  pd.calculateSingleCandleConfidence(candle, ShootingStar),
				Direction:   "bearish",
			})
		}

		// Hammer
		if pd.isHammer(candle, prevCandle) {
			patterns = append(patterns, DetectedPattern{
				Type:        Hammer,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(candle.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  pd.calculateSingleCandleConfidence(candle, Hammer),
				Direction:   "bullish",
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

// calculateConfidence calculates pattern confidence score
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

	// TODO: Add volume confirmation
	// TODO: Add trend context

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// calculateSingleCandleConfidence calculates confidence for single-candle patterns
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
