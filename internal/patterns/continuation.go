package patterns

import (
	"binance-trading-bot/internal/binance"
	"time"
)

// Continuation Pattern Detection Methods

// FlagPattern represents a flag/pennant formation
type FlagPattern struct {
	TrendDirection string  // "up" or "down"
	PoleHeight     float64 // Height of the pole (trend before consolidation)
	ConsolidationBars int  // Number of candles in consolidation
	BreakoutPrice  float64 // Price at breakout
}

// TrianglePattern represents a triangle formation
type TrianglePattern struct {
	Type          string  // "ascending", "descending", "symmetrical"
	UpperTrendline float64
	LowerTrendline float64
	Duration      int     // Number of candles in formation
}

// isBullishFlag checks for Bullish Flag pattern
func (pd *PatternDetector) isBullishFlag(candles []binance.Kline, startIdx int) (*FlagPattern, bool) {
	if startIdx < 10 || startIdx+5 >= len(candles) {
		return nil, false
	}

	// Need at least 10 candles before + 5 candles for flag
	poleCandles := candles[startIdx-10 : startIdx]
	flagCandles := candles[startIdx : startIdx+5]

	// 1. Check for upward pole (strong uptrend)
	poleStart := poleCandles[0].Open
	poleEnd := poleCandles[len(poleCandles)-1].Close
	poleHeight := poleEnd - poleStart

	if poleHeight <= 0 {
		return nil, false // Not an uptrend
	}

	// Pole should be strong (most candles bullish)
	bullishCount := 0
	for _, c := range poleCandles {
		if c.Close > c.Open {
			bullishCount++
		}
	}
	if float64(bullishCount)/float64(len(poleCandles)) < 0.6 {
		return nil, false // Weak uptrend
	}

	// 2. Check for downward sloping consolidation (flag)
	flagStart := flagCandles[0].High
	flagEnd := flagCandles[len(flagCandles)-1].Low

	// Flag should slope slightly down or sideways
	if flagEnd > flagStart {
		return nil, false // Flag slopes up (not valid)
	}

	// Flag range should be smaller than pole
	flagRange := flagStart - flagEnd
	if flagRange > poleHeight*0.5 {
		return nil, false // Flag too large
	}

	// 3. Volume should decrease during flag formation
	// (We'll skip this for now, can add later with volume data)

	return &FlagPattern{
		TrendDirection:    "up",
		PoleHeight:        poleHeight,
		ConsolidationBars: len(flagCandles),
		BreakoutPrice:     flagStart,
	}, true
}

// isBearishFlag checks for Bearish Flag pattern
func (pd *PatternDetector) isBearishFlag(candles []binance.Kline, startIdx int) (*FlagPattern, bool) {
	if startIdx < 10 || startIdx+5 >= len(candles) {
		return nil, false
	}

	poleCandles := candles[startIdx-10 : startIdx]
	flagCandles := candles[startIdx : startIdx+5]

	// 1. Check for downward pole (strong downtrend)
	poleStart := poleCandles[0].Open
	poleEnd := poleCandles[len(poleCandles)-1].Close
	poleHeight := poleStart - poleEnd // Negative for downtrend

	if poleHeight <= 0 {
		return nil, false // Not a downtrend
	}

	// Pole should be strong (most candles bearish)
	bearishCount := 0
	for _, c := range poleCandles {
		if c.Close < c.Open {
			bearishCount++
		}
	}
	if float64(bearishCount)/float64(len(poleCandles)) < 0.6 {
		return nil, false
	}

	// 2. Check for upward sloping consolidation (flag)
	flagStart := flagCandles[0].Low
	flagEnd := flagCandles[len(flagCandles)-1].High

	// Flag should slope slightly up or sideways
	if flagEnd < flagStart {
		return nil, false // Flag slopes down (not valid)
	}

	// Flag range should be smaller than pole
	flagRange := flagEnd - flagStart
	if flagRange > poleHeight*0.5 {
		return nil, false
	}

	return &FlagPattern{
		TrendDirection:    "down",
		PoleHeight:        poleHeight,
		ConsolidationBars: len(flagCandles),
		BreakoutPrice:     flagStart,
	}, true
}

// isAscendingTriangle checks for Ascending Triangle pattern
func (pd *PatternDetector) isAscendingTriangle(candles []binance.Kline, startIdx int) (*TrianglePattern, bool) {
	if startIdx+10 >= len(candles) {
		return nil, false
	}

	triangleCandles := candles[startIdx : startIdx+10]

	// Find highs and lows
	var highs, lows []float64
	for _, c := range triangleCandles {
		highs = append(highs, c.High)
		lows = append(lows, c.Low)
	}

	// Calculate average of highs and lows
	avgHigh := average(highs)
	_ = average(lows) // avgLow not used but calculated for consistency

	// Check if highs are relatively flat (resistance)
	highVariance := variance(highs)
	if highVariance > avgHigh*0.02 {
		return nil, false // Too much variance in highs
	}

	// Check if lows are rising (ascending support)
	if !isRising(lows) {
		return nil, false
	}

	return &TrianglePattern{
		Type:           "ascending",
		UpperTrendline: avgHigh,
		LowerTrendline: lows[0], // Start of ascending line
		Duration:       len(triangleCandles),
	}, true
}

// isDescendingTriangle checks for Descending Triangle pattern
func (pd *PatternDetector) isDescendingTriangle(candles []binance.Kline, startIdx int) (*TrianglePattern, bool) {
	if startIdx+10 >= len(candles) {
		return nil, false
	}

	triangleCandles := candles[startIdx : startIdx+10]

	var highs, lows []float64
	for _, c := range triangleCandles {
		highs = append(highs, c.High)
		lows = append(lows, c.Low)
	}

	_ = average(highs) // avgHigh not used but calculated for consistency
	avgLow := average(lows)

	// Check if lows are relatively flat (support)
	lowVariance := variance(lows)
	if lowVariance > avgLow*0.02 {
		return nil, false
	}

	// Check if highs are descending (descending resistance)
	if !isDescending(highs) {
		return nil, false
	}

	return &TrianglePattern{
		Type:           "descending",
		UpperTrendline: highs[0], // Start of descending line
		LowerTrendline: avgLow,
		Duration:       len(triangleCandles),
	}, true
}

// DetectContinuationPatterns scans for continuation patterns
func (pd *PatternDetector) DetectContinuationPatterns(symbol, timeframe string, candles []binance.Kline) []DetectedPattern {
	var patterns []DetectedPattern

	if len(candles) < 15 {
		return patterns
	}

	// Scan for flags and triangles
	for i := 10; i < len(candles)-5; i++ {
		// Bullish Flag
		if _, found := pd.isBullishFlag(candles, i); found {
			patterns = append(patterns, DetectedPattern{
				Type:        BullishFlag,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(candles[i].CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.70,
				Direction:   "bullish",
			})
		}

		// Bearish Flag
		if _, found := pd.isBearishFlag(candles, i); found {
			patterns = append(patterns, DetectedPattern{
				Type:        BearishFlag,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(candles[i].CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.70,
				Direction:   "bearish",
			})
		}

		// Ascending Triangle
		if _, found := pd.isAscendingTriangle(candles, i); found {
			patterns = append(patterns, DetectedPattern{
				Type:        AscendingTriangle,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(candles[i].CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.72,
				Direction:   "bullish",
			})
		}

		// Descending Triangle
		if _, found := pd.isDescendingTriangle(candles, i); found {
			patterns = append(patterns, DetectedPattern{
				Type:        DescendingTriangle,
				Symbol:      symbol,
				Timeframe:   timeframe,
				DetectedAt:  time.Unix(candles[i].CloseTime/1000, 0),
				CandleIndex: i,
				Confidence:  0.72,
				Direction:   "bearish",
			})
		}
	}

	return patterns
}

// Helper functions for continuation patterns

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func variance(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	avg := average(values)
	sum := 0.0
	for _, v := range values {
		diff := v - avg
		sum += diff * diff
	}
	return sum / float64(len(values))
}

func isRising(values []float64) bool {
	if len(values) < 2 {
		return false
	}
	// Check if generally trending up
	start := average(values[:len(values)/2])
	end := average(values[len(values)/2:])
	return end > start
}

func isDescending(values []float64) bool {
	if len(values) < 2 {
		return false
	}
	// Check if generally trending down
	start := average(values[:len(values)/2])
	end := average(values[len(values)/2:])
	return end < start
}
