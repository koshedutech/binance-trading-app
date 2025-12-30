package autopilot

import (
	"log"
	"math"
	"time"

	"binance-trading-bot/internal/binance"
)

// ===== REVERSAL PATTERN DETECTION =====
// This file contains functions for detecting reversal entry patterns
// based on consecutive Lower Lows (for LONG) or Higher Highs (for SHORT)
// across multiple timeframes (5m, 15m, 1h).

// DetectLowerLows checks if the last N candles (excluding current) are making consecutive lower lows.
// This indicates a potential bullish reversal setup (exhaustion of sellers).
// Returns nil if pattern is not detected.
func (g *GinieAnalyzer) DetectLowerLows(klines []binance.Kline, count int) *ReversalPattern {
	log.Printf("[REVERSAL] DetectLowerLows: checking %d klines for %d consecutive lower lows", len(klines), count)

	// Need at least count+1 candles (count candles to analyze + 1 current to exclude)
	if len(klines) < count+1 {
		log.Printf("[REVERSAL] DetectLowerLows: insufficient klines (%d < %d)", len(klines), count+1)
		return nil
	}

	// Get last N candles EXCLUDING the current candle
	// For 3 candles: indices [-4, -3, -2] (not -1 which is current)
	startIdx := len(klines) - count - 1
	endIdx := len(klines) - 1 // Exclude current candle
	candles := klines[startIdx:endIdx]

	// Log the candle lows being analyzed
	lows := make([]float64, len(candles))
	for i, c := range candles {
		lows[i] = c.Low
	}
	log.Printf("[REVERSAL] DetectLowerLows: analyzing lows: %v", lows)

	// Check each candle makes a LOWER LOW than the previous
	for i := 1; i < len(candles); i++ {
		if candles[i].Low >= candles[i-1].Low {
			log.Printf("[REVERSAL] DetectLowerLows: pattern broken at candle %d (%.4f >= %.4f)", i, candles[i].Low, candles[i-1].Low)
			return nil // Not a lower low - pattern broken
		}
	}

	log.Printf("[REVERSAL] ✓ DetectLowerLows: PATTERN DETECTED! %d consecutive lower lows", count)

	// Pattern detected! Extract candle data
	candleData := make([]ReversalCandleData, len(candles))
	for i, k := range candles {
		candleData[i] = ReversalCandleData{
			Open:   k.Open,
			High:   k.High,
			Low:    k.Low,
			Close:  k.Close,
			Volume: k.Volume,
			Time:   time.Unix(k.OpenTime/1000, 0),
		}
	}

	// Previous candle (the one right before current) - this is our entry zone
	prevCandle := klines[len(klines)-2]

	// Calculate confidence based on pattern strength
	// - Larger drops = more confidence (exhaustion move)
	// - Increasing volume on drops = more confidence
	totalDrop := (candles[0].Low - candles[len(candles)-1].Low) / candles[0].Low * 100
	confidence := 50.0 // Base confidence

	// Bonus for larger drop (up to +30)
	if totalDrop > 0.5 {
		confidence += math.Min(totalDrop*10, 30)
	}

	// Bonus for volume pattern (exhaustion often has decreasing volume)
	if len(candles) >= 3 {
		if candles[2].Volume < candles[1].Volume && candles[1].Volume < candles[0].Volume {
			confidence += 15 // Decreasing volume = exhaustion
		}
	}

	return &ReversalPattern{
		PatternType:    "lower_lows",
		Direction:      "LONG",
		CandleCount:    count,
		Candles:        candleData,
		PrevCandleLow:  prevCandle.Low,
		PrevCandleHigh: prevCandle.High,
		Confidence:     math.Min(confidence, 100),
		DetectedAt:     time.Now(),
	}
}

// DetectHigherHighs checks if the last N candles (excluding current) are making consecutive higher highs.
// This indicates a potential bearish reversal setup (exhaustion of buyers).
// Returns nil if pattern is not detected.
func (g *GinieAnalyzer) DetectHigherHighs(klines []binance.Kline, count int) *ReversalPattern {
	log.Printf("[REVERSAL] DetectHigherHighs: checking %d klines for %d consecutive higher highs", len(klines), count)

	// Need at least count+1 candles (count candles to analyze + 1 current to exclude)
	if len(klines) < count+1 {
		log.Printf("[REVERSAL] DetectHigherHighs: insufficient klines (%d < %d)", len(klines), count+1)
		return nil
	}

	// Get last N candles EXCLUDING the current candle
	startIdx := len(klines) - count - 1
	endIdx := len(klines) - 1 // Exclude current candle
	candles := klines[startIdx:endIdx]

	// Log the candle highs being analyzed
	highs := make([]float64, len(candles))
	for i, c := range candles {
		highs[i] = c.High
	}
	log.Printf("[REVERSAL] DetectHigherHighs: analyzing highs: %v", highs)

	// Check each candle makes a HIGHER HIGH than the previous
	for i := 1; i < len(candles); i++ {
		if candles[i].High <= candles[i-1].High {
			log.Printf("[REVERSAL] DetectHigherHighs: pattern broken at candle %d (%.4f <= %.4f)", i, candles[i].High, candles[i-1].High)
			return nil // Not a higher high - pattern broken
		}
	}

	log.Printf("[REVERSAL] ✓ DetectHigherHighs: PATTERN DETECTED! %d consecutive higher highs", count)

	// Pattern detected! Extract candle data
	candleData := make([]ReversalCandleData, len(candles))
	for i, k := range candles {
		candleData[i] = ReversalCandleData{
			Open:   k.Open,
			High:   k.High,
			Low:    k.Low,
			Close:  k.Close,
			Volume: k.Volume,
			Time:   time.Unix(k.OpenTime/1000, 0),
		}
	}

	// Previous candle (the one right before current) - this is our entry zone
	prevCandle := klines[len(klines)-2]

	// Calculate confidence based on pattern strength
	totalRise := (candles[len(candles)-1].High - candles[0].High) / candles[0].High * 100
	confidence := 50.0 // Base confidence

	// Bonus for larger rise (up to +30)
	if totalRise > 0.5 {
		confidence += math.Min(totalRise*10, 30)
	}

	// Bonus for decreasing volume on rise (exhaustion)
	if len(candles) >= 3 {
		if candles[2].Volume < candles[1].Volume && candles[1].Volume < candles[0].Volume {
			confidence += 15 // Decreasing volume = exhaustion
		}
	}

	return &ReversalPattern{
		PatternType:    "higher_highs",
		Direction:      "SHORT",
		CandleCount:    count,
		Candles:        candleData,
		PrevCandleLow:  prevCandle.Low,
		PrevCandleHigh: prevCandle.High,
		Confidence:     math.Min(confidence, 100),
		DetectedAt:     time.Now(),
	}
}

// DetectReversalPattern detects either Lower Lows or Higher Highs pattern
// and returns the pattern with direction (LONG for LL, SHORT for HH)
func (g *GinieAnalyzer) DetectReversalPattern(klines []binance.Kline, count int) *ReversalPattern {
	// Try to detect Lower Lows first (LONG reversal)
	if pattern := g.DetectLowerLows(klines, count); pattern != nil {
		return pattern
	}

	// Try to detect Higher Highs (SHORT reversal)
	if pattern := g.DetectHigherHighs(klines, count); pattern != nil {
		return pattern
	}

	return nil
}

// AnalyzeMTFReversal performs multi-timeframe reversal pattern detection
// Checks 5m, 15m, and 1h timeframes for consistent reversal patterns.
// Returns analysis with alignment status and entry price from 5m candle.
func (g *GinieAnalyzer) AnalyzeMTFReversal(symbol string, consecutiveCandles int) *MTFReversalAnalysis {
	log.Printf("[REVERSAL] ========== MTF REVERSAL ANALYSIS START ==========")
	log.Printf("[REVERSAL] AnalyzeMTFReversal: symbol=%s, consecutiveCandles=%d", symbol, consecutiveCandles)

	result := &MTFReversalAnalysis{
		Symbol:     symbol,
		Aligned:    false,
		AnalyzedAt: time.Now(),
	}

	// Fetch klines for each timeframe (need at least consecutiveCandles + 5 for safety)
	klineLimit := consecutiveCandles + 10

	// Fetch 5m klines (primary for scalp mode)
	log.Printf("[REVERSAL] Fetching 5m klines for %s...", symbol)
	klines5m, err := g.futuresClient.GetFuturesKlines(symbol, "5m", klineLimit)
	if err != nil || len(klines5m) < consecutiveCandles+1 {
		log.Printf("[REVERSAL] Failed to fetch 5m klines: err=%v, len=%d", err, len(klines5m))
		result.Reason = "Failed to fetch 5m klines"
		return result
	}
	log.Printf("[REVERSAL] Got %d 5m klines", len(klines5m))

	// Fetch 15m klines (secondary confirmation)
	klines15m, err := g.futuresClient.GetFuturesKlines(symbol, "15m", klineLimit)
	if err != nil || len(klines15m) < consecutiveCandles+1 {
		result.Reason = "Failed to fetch 15m klines"
		return result
	}

	// Fetch 1h klines (tertiary confirmation)
	klines1h, err := g.futuresClient.GetFuturesKlines(symbol, "1h", klineLimit)
	if err != nil || len(klines1h) < consecutiveCandles+1 {
		result.Reason = "Failed to fetch 1h klines"
		return result
	}

	// Detect patterns on each timeframe
	log.Printf("[REVERSAL] --- Detecting patterns on 5m ---")
	result.Pattern5m = g.DetectReversalPattern(klines5m, consecutiveCandles)
	if result.Pattern5m != nil {
		result.Pattern5m.Timeframe = "5m"
		log.Printf("[REVERSAL] 5m: %s pattern, confidence=%.1f%%, entry_low=%.6f, entry_high=%.6f",
			result.Pattern5m.Direction, result.Pattern5m.Confidence, result.Pattern5m.PrevCandleLow, result.Pattern5m.PrevCandleHigh)
	} else {
		log.Printf("[REVERSAL] 5m: NO PATTERN DETECTED")
	}

	log.Printf("[REVERSAL] --- Detecting patterns on 15m ---")
	result.Pattern15m = g.DetectReversalPattern(klines15m, consecutiveCandles)
	if result.Pattern15m != nil {
		result.Pattern15m.Timeframe = "15m"
		log.Printf("[REVERSAL] 15m: %s pattern, confidence=%.1f%%", result.Pattern15m.Direction, result.Pattern15m.Confidence)
	} else {
		log.Printf("[REVERSAL] 15m: NO PATTERN DETECTED")
	}

	log.Printf("[REVERSAL] --- Detecting patterns on 1h ---")
	result.Pattern1h = g.DetectReversalPattern(klines1h, consecutiveCandles)
	if result.Pattern1h != nil {
		result.Pattern1h.Timeframe = "1h"
		log.Printf("[REVERSAL] 1h: %s pattern, confidence=%.1f%%", result.Pattern1h.Direction, result.Pattern1h.Confidence)
	} else {
		log.Printf("[REVERSAL] 1h: NO PATTERN DETECTED")
	}

	// Count aligned patterns and determine direction
	longCount := 0
	shortCount := 0
	totalScore := 0.0

	if result.Pattern5m != nil {
		if result.Pattern5m.Direction == "LONG" {
			longCount++
		} else {
			shortCount++
		}
		totalScore += result.Pattern5m.Confidence * 0.5 // 5m has 50% weight
	}

	if result.Pattern15m != nil {
		if result.Pattern15m.Direction == "LONG" {
			longCount++
		} else {
			shortCount++
		}
		totalScore += result.Pattern15m.Confidence * 0.3 // 15m has 30% weight
	}

	if result.Pattern1h != nil {
		if result.Pattern1h.Direction == "LONG" {
			longCount++
		} else {
			shortCount++
		}
		totalScore += result.Pattern1h.Confidence * 0.2 // 1h has 20% weight
	}

	// Determine alignment (need 2+ timeframes agreeing on same direction)
	alignedCount := longCount
	if shortCount > longCount {
		alignedCount = shortCount
	}
	result.AlignedCount = alignedCount

	if alignedCount >= 2 {
		result.Aligned = true
		result.AlignmentScore = totalScore

		if longCount > shortCount {
			result.Direction = "LONG"
			// Entry at previous 5m candle's LOW for LONG
			if result.Pattern5m != nil {
				result.EntryPrice = result.Pattern5m.PrevCandleLow
			}
			result.Reason = buildAlignmentReason("LONG", longCount, result.Pattern5m, result.Pattern15m, result.Pattern1h)
		} else {
			result.Direction = "SHORT"
			// Entry at previous 5m candle's HIGH for SHORT
			if result.Pattern5m != nil {
				result.EntryPrice = result.Pattern5m.PrevCandleHigh
			}
			result.Reason = buildAlignmentReason("SHORT", shortCount, result.Pattern5m, result.Pattern15m, result.Pattern1h)
		}

		log.Printf("[REVERSAL] %s: MTF ALIGNED - %s (%d/3 TFs), Score=%.1f, Entry=%.6f",
			symbol, result.Direction, alignedCount, result.AlignmentScore, result.EntryPrice)
	} else {
		result.Reason = "Insufficient timeframe alignment"
		if alignedCount == 1 {
			result.Reason = "Only 1 timeframe shows pattern - need 2+ for confirmation"
		} else if alignedCount == 0 {
			result.Reason = "No reversal pattern detected on any timeframe"
		}
		log.Printf("[REVERSAL] %s: NOT ALIGNED - longCount=%d, shortCount=%d, reason=%s",
			symbol, longCount, shortCount, result.Reason)
	}

	log.Printf("[REVERSAL] ========== MTF REVERSAL ANALYSIS END (aligned=%v) ==========", result.Aligned)
	return result
}

// buildAlignmentReason creates a human-readable explanation of the alignment
func buildAlignmentReason(direction string, count int, p5m, p15m, p1h *ReversalPattern) string {
	patterns := []string{}

	if p5m != nil && p5m.Direction == direction {
		patterns = append(patterns, "5m")
	}
	if p15m != nil && p15m.Direction == direction {
		patterns = append(patterns, "15m")
	}
	if p1h != nil && p1h.Direction == direction {
		patterns = append(patterns, "1h")
	}

	patternType := "lower_lows"
	if direction == "SHORT" {
		patternType = "higher_highs"
	}

	return patternType + " detected on: " + joinStrings(patterns, ", ")
}

// joinStrings joins strings with a separator (simple helper)
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
