package patterns

import (
	"binance-trading-bot/internal/binance"
	"time"
)

// Additional Reversal Pattern Detection Methods

// isBullishEngulfing checks for Bullish Engulfing pattern
func (pd *PatternDetector) isBullishEngulfing(c1, c2 binance.Kline) bool {
	// C1: Bearish (red) candle
	if c1.Close >= c1.Open {
		return false
	}

	// C2: Bullish (green) candle
	if c2.Close <= c2.Open {
		return false
	}

	// C2 body must completely engulf C1 body
	if c2.Open > c1.Open || c2.Close < c1.Close {
		return false
	}

	// C2 should open at or below C1 close
	if c2.Open > c1.Close {
		return false
	}

	// C2 should close at or above C1 open
	if c2.Close < c1.Open {
		return false
	}

	return true
}

// isBearishEngulfing checks for Bearish Engulfing pattern
func (pd *PatternDetector) isBearishEngulfing(c1, c2 binance.Kline) bool {
	// C1: Bullish (green) candle
	if c1.Close <= c1.Open {
		return false
	}

	// C2: Bearish (red) candle
	if c2.Close >= c2.Open {
		return false
	}

	// C2 body must completely engulf C1 body
	if c2.Open < c1.Close || c2.Close > c1.Open {
		return false
	}

	// C2 should open at or above C1 close
	if c2.Open < c1.Close {
		return false
	}

	// C2 should close at or below C1 open
	if c2.Close > c1.Open {
		return false
	}

	return true
}

// isDoji checks for Doji pattern (indecision)
func (pd *PatternDetector) isDoji(candle binance.Kline) bool {
	body := abs(candle.Close - candle.Open)
	range_ := candle.High - candle.Low

	if range_ == 0 {
		return false
	}

	// Doji: body is very small relative to range (< 10%)
	return (body / range_) < 0.10
}

// isDragonflyDoji checks for Dragonfly Doji (bullish)
func (pd *PatternDetector) isDragonflyDoji(candle binance.Kline) bool {
	if !pd.isDoji(candle) {
		return false
	}

	body := abs(candle.Close - candle.Open)
	lowerWick := min(candle.Open, candle.Close) - candle.Low
	upperWick := candle.High - max(candle.Open, candle.Close)

	// Long lower wick, little to no upper wick
	return lowerWick > body*3 && upperWick < body*0.3
}

// isGravestoneDoji checks for Gravestone Doji (bearish)
func (pd *PatternDetector) isGravestoneDoji(candle binance.Kline) bool {
	if !pd.isDoji(candle) {
		return false
	}

	body := abs(candle.Close - candle.Open)
	lowerWick := min(candle.Open, candle.Close) - candle.Low
	upperWick := candle.High - max(candle.Open, candle.Close)

	// Long upper wick, little to no lower wick
	return upperWick > body*3 && lowerWick < body*0.3
}

// isBullishHarami checks for Bullish Harami pattern
func (pd *PatternDetector) isBullishHarami(c1, c2 binance.Kline) bool {
	// C1: Large bearish candle
	if c1.Close >= c1.Open {
		return false
	}
	body1 := c1.Open - c1.Close
	range1 := c1.High - c1.Low
	if body1 < range1*0.6 {
		return false // Body too small
	}

	// C2: Small bullish candle inside C1
	if c2.Close <= c2.Open {
		return false
	}

	// C2 must be contained within C1 body
	if c2.Open < c1.Close || c2.Close > c1.Open {
		return false
	}

	// C2 should be significantly smaller than C1
	body2 := c2.Close - c2.Open
	if body2 > body1*0.5 {
		return false
	}

	return true
}

// isBearishHarami checks for Bearish Harami pattern
func (pd *PatternDetector) isBearishHarami(c1, c2 binance.Kline) bool {
	// C1: Large bullish candle
	if c1.Close <= c1.Open {
		return false
	}
	body1 := c1.Close - c1.Open
	range1 := c1.High - c1.Low
	if body1 < range1*0.6 {
		return false
	}

	// C2: Small bearish candle inside C1
	if c2.Close >= c2.Open {
		return false
	}

	// C2 must be contained within C1 body
	if c2.Open > c1.Close || c2.Close < c1.Open {
		return false
	}

	// C2 should be significantly smaller than C1
	body2 := c2.Open - c2.Close
	if body2 > body1*0.5 {
		return false
	}

	return true
}

// isHangingMan checks for Hanging Man pattern (bearish)
func (pd *PatternDetector) isHangingMan(candle binance.Kline, prevCandle *binance.Kline) bool {
	// Same shape as hammer but appears after uptrend
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

	// Should appear after uptrend (if previous candle available)
	if prevCandle != nil && prevCandle.Close <= prevCandle.Open {
		return false // Not in uptrend
	}

	return true
}

// DetectReversalPatterns scans for all reversal patterns
func (pd *PatternDetector) DetectReversalPatterns(symbol, timeframe string, candles []binance.Kline) []DetectedPattern {
	var patterns []DetectedPattern

	if len(candles) < 2 {
		return patterns
	}

	// Two-candle patterns (Engulfing, Harami)
	for i := 1; i < len(candles); i++ {
		c1, c2 := candles[i-1], candles[i]

		// Bullish Engulfing
		if pd.isBullishEngulfing(c1, c2) {
			patterns = append(patterns, DetectedPattern{
				Type:        BullishEngulfing,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(c2.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.75,
				Direction:   "bullish",
			})
		}

		// Bearish Engulfing
		if pd.isBearishEngulfing(c1, c2) {
			patterns = append(patterns, DetectedPattern{
				Type:        BearishEngulfing,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(c2.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.75,
				Direction:   "bearish",
			})
		}

		// Bullish Harami
		if pd.isBullishHarami(c1, c2) {
			patterns = append(patterns, DetectedPattern{
				Type:        BullishHarami,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(c2.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.68,
				Direction:   "bullish",
			})
		}

		// Bearish Harami
		if pd.isBearishHarami(c1, c2) {
			patterns = append(patterns, DetectedPattern{
				Type:        BearishHarami,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(c2.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.68,
				Direction:   "bearish",
			})
		}
	}

	// Single candle patterns (Doji variations)
	for i := 0; i < len(candles); i++ {
		candle := candles[i]

		// Dragonfly Doji
		if pd.isDragonflyDoji(candle) {
			patterns = append(patterns, DetectedPattern{
				Type:        DragonflyDoji,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(candle.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.62,
				Direction:   "bullish",
			})
		}

		// Gravestone Doji
		if pd.isGravestoneDoji(candle) {
			patterns = append(patterns, DetectedPattern{
				Type:        GravestoneDoji,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(candle.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.62,
				Direction:   "bearish",
			})
		}

		// Regular Doji
		if pd.isDoji(candle) && !pd.isDragonflyDoji(candle) && !pd.isGravestoneDoji(candle) {
			patterns = append(patterns, DetectedPattern{
				Type:        Doji,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(candle.CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.50,
				Direction:   "neutral",
			})
		}
	}

	return patterns
}
