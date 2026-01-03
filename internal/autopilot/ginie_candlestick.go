package autopilot

import (
	"math"
	"time"

	"binance-trading-bot/internal/binance"
)

// ===== CANDLESTICK PATTERN DETECTION =====
// This file contains functions for detecting classic candlestick patterns
// to improve entry timing and reversal confirmation.

// CandlestickPatternType represents the type of candlestick pattern
type CandlestickPatternType string

const (
	PatternNone             CandlestickPatternType = "none"
	PatternHammer           CandlestickPatternType = "hammer"
	PatternInvertedHammer   CandlestickPatternType = "inverted_hammer"
	PatternBullishEngulfing CandlestickPatternType = "bullish_engulfing"
	PatternBearishEngulfing CandlestickPatternType = "bearish_engulfing"
	PatternDoji             CandlestickPatternType = "doji"
	PatternDragonfly        CandlestickPatternType = "dragonfly_doji"
	PatternGravestone       CandlestickPatternType = "gravestone_doji"
	PatternPinBarBullish    CandlestickPatternType = "pin_bar_bullish"
	PatternPinBarBearish    CandlestickPatternType = "pin_bar_bearish"
	PatternMorningStar      CandlestickPatternType = "morning_star"
	PatternEveningStar      CandlestickPatternType = "evening_star"
	PatternThreeWhite       CandlestickPatternType = "three_white_soldiers"
	PatternThreeBlack       CandlestickPatternType = "three_black_crows"
	PatternTweezerBottom    CandlestickPatternType = "tweezer_bottom"
	PatternTweezerTop       CandlestickPatternType = "tweezer_top"
)

// CandlestickSignal represents the trading signal from a pattern
type CandlestickSignal string

const (
	SignalBullish CandlestickSignal = "BULLISH"
	SignalBearish CandlestickSignal = "BEARISH"
	SignalNeutral CandlestickSignal = "NEUTRAL"
)

// CandlestickPattern represents a detected candlestick pattern
type CandlestickPattern struct {
	Type        CandlestickPatternType `json:"type"`
	Signal      CandlestickSignal      `json:"signal"`
	Confidence  float64                `json:"confidence"`  // 0-100
	Strength    string                 `json:"strength"`    // weak, moderate, strong
	Description string                 `json:"description"`
	CandleCount int                    `json:"candle_count"` // Number of candles in pattern
	DetectedAt  time.Time              `json:"detected_at"`
}

// CandlestickAnalysis contains the results of candlestick pattern analysis
type CandlestickAnalysis struct {
	Symbol           string                `json:"symbol"`
	Timeframe        string                `json:"timeframe"`
	PatternsDetected []CandlestickPattern  `json:"patterns_detected"`
	PrimarySignal    CandlestickSignal     `json:"primary_signal"`
	SignalStrength   float64               `json:"signal_strength"` // 0-100
	BullishPatterns  int                   `json:"bullish_patterns"`
	BearishPatterns  int                   `json:"bearish_patterns"`
	BestPattern      *CandlestickPattern   `json:"best_pattern,omitempty"`
	AnalyzedAt       time.Time             `json:"analyzed_at"`
}

// CandlestickConfig contains configuration for pattern detection
type CandlestickConfig struct {
	Enabled              bool    `json:"enabled"`
	MinConfidence        float64 `json:"min_confidence"`         // Minimum confidence to report (default: 60)
	DojiBodyRatio        float64 `json:"doji_body_ratio"`        // Max body/range ratio for doji (default: 0.1)
	HammerWickRatio      float64 `json:"hammer_wick_ratio"`      // Min lower wick / body ratio (default: 2.0)
	EngulfingMinOverlap  float64 `json:"engulfing_min_overlap"`  // Min % body must exceed previous (default: 1.0 = 100%)
	PinBarWickRatio      float64 `json:"pin_bar_wick_ratio"`     // Min wick / total range ratio (default: 0.6)
	RequirePriorTrend    bool    `json:"require_prior_trend"`    // Require trend before reversal patterns
	PriorTrendCandles    int     `json:"prior_trend_candles"`    // Candles to check for prior trend (default: 5)
}

// DefaultCandlestickConfig returns default configuration
func DefaultCandlestickConfig() *CandlestickConfig {
	return &CandlestickConfig{
		Enabled:              true,
		MinConfidence:        60.0,
		DojiBodyRatio:        0.1,   // Body must be < 10% of range
		HammerWickRatio:      2.0,   // Lower wick at least 2x body
		EngulfingMinOverlap:  1.0,   // Must fully engulf previous body
		PinBarWickRatio:      0.6,   // Wick at least 60% of total range
		RequirePriorTrend:    true,  // Require prior trend for reversal patterns
		PriorTrendCandles:    5,     // Check 5 candles for prior trend
	}
}

// AnalyzeCandlestickPatterns performs comprehensive candlestick pattern analysis
func (g *GinieAnalyzer) AnalyzeCandlestickPatterns(symbol string, klines []binance.Kline, config *CandlestickConfig) *CandlestickAnalysis {
	if config == nil {
		config = DefaultCandlestickConfig()
	}

	analysis := &CandlestickAnalysis{
		Symbol:           symbol,
		Timeframe:        "1m", // Default, can be overridden
		PatternsDetected: make([]CandlestickPattern, 0),
		AnalyzedAt:       time.Now(),
	}

	if !config.Enabled || len(klines) < 5 {
		analysis.PrimarySignal = SignalNeutral
		return analysis
	}

	// Detect all pattern types
	patterns := []CandlestickPattern{}

	// Single candle patterns (check last candle)
	if p := g.detectHammer(klines, config); p != nil {
		patterns = append(patterns, *p)
	}
	if p := g.detectInvertedHammer(klines, config); p != nil {
		patterns = append(patterns, *p)
	}
	if p := g.detectDoji(klines, config); p != nil {
		patterns = append(patterns, *p)
	}
	if p := g.detectPinBar(klines, config); p != nil {
		patterns = append(patterns, *p)
	}

	// Two candle patterns
	if p := g.detectEngulfing(klines, config); p != nil {
		patterns = append(patterns, *p)
	}
	if p := g.detectTweezer(klines, config); p != nil {
		patterns = append(patterns, *p)
	}

	// Three candle patterns
	if p := g.detectMorningEveningStar(klines, config); p != nil {
		patterns = append(patterns, *p)
	}
	if p := g.detectThreeSoldiersCrows(klines, config); p != nil {
		patterns = append(patterns, *p)
	}

	// Filter by minimum confidence
	for _, p := range patterns {
		if p.Confidence >= config.MinConfidence {
			analysis.PatternsDetected = append(analysis.PatternsDetected, p)
			if p.Signal == SignalBullish {
				analysis.BullishPatterns++
			} else if p.Signal == SignalBearish {
				analysis.BearishPatterns++
			}
		}
	}

	// Determine primary signal and best pattern
	if len(analysis.PatternsDetected) > 0 {
		// Find best pattern by confidence
		var best *CandlestickPattern
		for i := range analysis.PatternsDetected {
			if best == nil || analysis.PatternsDetected[i].Confidence > best.Confidence {
				best = &analysis.PatternsDetected[i]
			}
		}
		analysis.BestPattern = best

		// Primary signal based on pattern count and strength
		bullishScore := float64(analysis.BullishPatterns)
		bearishScore := float64(analysis.BearishPatterns)

		// Add weight for best pattern
		if best.Signal == SignalBullish {
			bullishScore += best.Confidence / 50
		} else if best.Signal == SignalBearish {
			bearishScore += best.Confidence / 50
		}

		if bullishScore > bearishScore {
			analysis.PrimarySignal = SignalBullish
			analysis.SignalStrength = math.Min(bullishScore*20, 100)
		} else if bearishScore > bullishScore {
			analysis.PrimarySignal = SignalBearish
			analysis.SignalStrength = math.Min(bearishScore*20, 100)
		} else {
			analysis.PrimarySignal = SignalNeutral
			analysis.SignalStrength = 50
		}
	} else {
		analysis.PrimarySignal = SignalNeutral
		analysis.SignalStrength = 0
	}

	return analysis
}

// ===== SINGLE CANDLE PATTERNS =====

// detectHammer detects hammer pattern (bullish reversal)
// Characteristics: Small body at top, long lower wick (2x+ body), little/no upper wick
func (g *GinieAnalyzer) detectHammer(klines []binance.Kline, config *CandlestickConfig) *CandlestickPattern {
	if len(klines) < 2 {
		return nil
	}

	candle := klines[len(klines)-1]
	body := math.Abs(candle.Close - candle.Open)
	totalRange := candle.High - candle.Low

	if totalRange == 0 {
		return nil
	}

	lowerWick := math.Min(candle.Open, candle.Close) - candle.Low
	upperWick := candle.High - math.Max(candle.Open, candle.Close)
	bodyRatio := body / totalRange

	// Hammer criteria:
	// 1. Small body (< 35% of range)
	// 2. Long lower wick (at least 2x body)
	// 3. Little upper wick (< 20% of range)
	// 4. Ideally after a downtrend
	if bodyRatio < 0.35 &&
	   lowerWick >= body*config.HammerWickRatio &&
	   upperWick < totalRange*0.2 {

		confidence := 60.0

		// Bonus for prior downtrend
		if config.RequirePriorTrend && g.hasPriorTrend(klines, "down", config.PriorTrendCandles) {
			confidence += 20
		}

		// Bonus for longer wick
		if lowerWick >= body*3 {
			confidence += 10
		}

		// Bonus for bullish close (green candle)
		if candle.Close > candle.Open {
			confidence += 5
		}

		return &CandlestickPattern{
			Type:        PatternHammer,
			Signal:      SignalBullish,
			Confidence:  math.Min(confidence, 100),
			Strength:    classifyPatternStrength(confidence),
			Description: "Hammer: Long lower wick rejection, potential bullish reversal",
			CandleCount: 1,
			DetectedAt:  time.Now(),
		}
	}
	return nil
}

// detectInvertedHammer detects inverted hammer (bullish reversal after downtrend)
// Characteristics: Small body at bottom, long upper wick, little/no lower wick
func (g *GinieAnalyzer) detectInvertedHammer(klines []binance.Kline, config *CandlestickConfig) *CandlestickPattern {
	if len(klines) < 2 {
		return nil
	}

	candle := klines[len(klines)-1]
	body := math.Abs(candle.Close - candle.Open)
	totalRange := candle.High - candle.Low

	if totalRange == 0 {
		return nil
	}

	lowerWick := math.Min(candle.Open, candle.Close) - candle.Low
	upperWick := candle.High - math.Max(candle.Open, candle.Close)
	bodyRatio := body / totalRange

	// Inverted Hammer criteria:
	// 1. Small body (< 35% of range)
	// 2. Long upper wick (at least 2x body)
	// 3. Little lower wick (< 20% of range)
	// 4. After a downtrend
	if bodyRatio < 0.35 &&
	   upperWick >= body*config.HammerWickRatio &&
	   lowerWick < totalRange*0.2 {

		confidence := 55.0 // Slightly less reliable than hammer

		if config.RequirePriorTrend && g.hasPriorTrend(klines, "down", config.PriorTrendCandles) {
			confidence += 20
		}

		if upperWick >= body*3 {
			confidence += 10
		}

		return &CandlestickPattern{
			Type:        PatternInvertedHammer,
			Signal:      SignalBullish,
			Confidence:  math.Min(confidence, 100),
			Strength:    classifyPatternStrength(confidence),
			Description: "Inverted Hammer: Upper wick rejection after downtrend",
			CandleCount: 1,
			DetectedAt:  time.Now(),
		}
	}
	return nil
}

// detectDoji detects doji patterns (indecision, potential reversal)
func (g *GinieAnalyzer) detectDoji(klines []binance.Kline, config *CandlestickConfig) *CandlestickPattern {
	if len(klines) < 2 {
		return nil
	}

	candle := klines[len(klines)-1]
	body := math.Abs(candle.Close - candle.Open)
	totalRange := candle.High - candle.Low

	if totalRange == 0 {
		return nil
	}

	bodyRatio := body / totalRange
	lowerWick := math.Min(candle.Open, candle.Close) - candle.Low
	upperWick := candle.High - math.Max(candle.Open, candle.Close)

	// Doji: Body is very small relative to range
	if bodyRatio <= config.DojiBodyRatio {
		patternType := PatternDoji
		signal := SignalNeutral
		description := "Doji: Indecision candle, watch for direction"
		confidence := 55.0

		// Classify doji type
		if lowerWick > upperWick*2 {
			// Dragonfly Doji (bullish)
			patternType = PatternDragonfly
			signal = SignalBullish
			description = "Dragonfly Doji: Long lower wick, bullish reversal signal"
			confidence = 70.0
			if g.hasPriorTrend(klines, "down", config.PriorTrendCandles) {
				confidence += 15
			}
		} else if upperWick > lowerWick*2 {
			// Gravestone Doji (bearish)
			patternType = PatternGravestone
			signal = SignalBearish
			description = "Gravestone Doji: Long upper wick, bearish reversal signal"
			confidence = 70.0
			if g.hasPriorTrend(klines, "up", config.PriorTrendCandles) {
				confidence += 15
			}
		}

		return &CandlestickPattern{
			Type:        patternType,
			Signal:      signal,
			Confidence:  math.Min(confidence, 100),
			Strength:    classifyPatternStrength(confidence),
			Description: description,
			CandleCount: 1,
			DetectedAt:  time.Now(),
		}
	}
	return nil
}

// detectPinBar detects pin bar patterns (strong reversal signal)
func (g *GinieAnalyzer) detectPinBar(klines []binance.Kline, config *CandlestickConfig) *CandlestickPattern {
	if len(klines) < 2 {
		return nil
	}

	candle := klines[len(klines)-1]
	body := math.Abs(candle.Close - candle.Open)
	totalRange := candle.High - candle.Low

	if totalRange == 0 {
		return nil
	}

	lowerWick := math.Min(candle.Open, candle.Close) - candle.Low
	upperWick := candle.High - math.Max(candle.Open, candle.Close)

	// Pin Bar: One wick dominates (> 60% of range), small body
	lowerWickRatio := lowerWick / totalRange
	upperWickRatio := upperWick / totalRange
	bodyRatio := body / totalRange

	if bodyRatio < 0.3 { // Small body requirement
		if lowerWickRatio >= config.PinBarWickRatio {
			// Bullish Pin Bar
			confidence := 70.0
			if g.hasPriorTrend(klines, "down", config.PriorTrendCandles) {
				confidence += 15
			}
			if lowerWickRatio >= 0.7 {
				confidence += 10
			}

			return &CandlestickPattern{
				Type:        PatternPinBarBullish,
				Signal:      SignalBullish,
				Confidence:  math.Min(confidence, 100),
				Strength:    classifyPatternStrength(confidence),
				Description: "Bullish Pin Bar: Strong lower wick rejection",
				CandleCount: 1,
				DetectedAt:  time.Now(),
			}
		} else if upperWickRatio >= config.PinBarWickRatio {
			// Bearish Pin Bar
			confidence := 70.0
			if g.hasPriorTrend(klines, "up", config.PriorTrendCandles) {
				confidence += 15
			}
			if upperWickRatio >= 0.7 {
				confidence += 10
			}

			return &CandlestickPattern{
				Type:        PatternPinBarBearish,
				Signal:      SignalBearish,
				Confidence:  math.Min(confidence, 100),
				Strength:    classifyPatternStrength(confidence),
				Description: "Bearish Pin Bar: Strong upper wick rejection",
				CandleCount: 1,
				DetectedAt:  time.Now(),
			}
		}
	}
	return nil
}

// ===== TWO CANDLE PATTERNS =====

// detectEngulfing detects bullish and bearish engulfing patterns
func (g *GinieAnalyzer) detectEngulfing(klines []binance.Kline, config *CandlestickConfig) *CandlestickPattern {
	if len(klines) < 3 {
		return nil
	}

	prev := klines[len(klines)-2]
	curr := klines[len(klines)-1]

	prevBody := prev.Close - prev.Open // Positive = bullish, negative = bearish
	currBody := curr.Close - curr.Open
	prevBodyAbs := math.Abs(prevBody)
	currBodyAbs := math.Abs(currBody)

	// Engulfing requires opposite colors and current body engulfs previous
	if prevBody < 0 && currBody > 0 {
		// Potential Bullish Engulfing (prev red, curr green)
		// Current body must fully engulf previous body
		if curr.Close > prev.Open && curr.Open < prev.Close {
			confidence := 65.0

			// More engulfing = higher confidence
			engulfRatio := currBodyAbs / prevBodyAbs
			if engulfRatio >= 1.5 {
				confidence += 15
			} else if engulfRatio >= 1.2 {
				confidence += 8
			}

			if g.hasPriorTrend(klines, "down", config.PriorTrendCandles) {
				confidence += 15
			}

			return &CandlestickPattern{
				Type:        PatternBullishEngulfing,
				Signal:      SignalBullish,
				Confidence:  math.Min(confidence, 100),
				Strength:    classifyPatternStrength(confidence),
				Description: "Bullish Engulfing: Green candle fully engulfs previous red",
				CandleCount: 2,
				DetectedAt:  time.Now(),
			}
		}
	} else if prevBody > 0 && currBody < 0 {
		// Potential Bearish Engulfing (prev green, curr red)
		if curr.Open > prev.Close && curr.Close < prev.Open {
			confidence := 65.0

			engulfRatio := currBodyAbs / prevBodyAbs
			if engulfRatio >= 1.5 {
				confidence += 15
			} else if engulfRatio >= 1.2 {
				confidence += 8
			}

			if g.hasPriorTrend(klines, "up", config.PriorTrendCandles) {
				confidence += 15
			}

			return &CandlestickPattern{
				Type:        PatternBearishEngulfing,
				Signal:      SignalBearish,
				Confidence:  math.Min(confidence, 100),
				Strength:    classifyPatternStrength(confidence),
				Description: "Bearish Engulfing: Red candle fully engulfs previous green",
				CandleCount: 2,
				DetectedAt:  time.Now(),
			}
		}
	}
	return nil
}

// detectTweezer detects tweezer top and bottom patterns
func (g *GinieAnalyzer) detectTweezer(klines []binance.Kline, config *CandlestickConfig) *CandlestickPattern {
	if len(klines) < 3 {
		return nil
	}

	prev := klines[len(klines)-2]
	curr := klines[len(klines)-1]

	// Tolerance for "equal" highs/lows (0.1% of price)
	tolerance := curr.Close * 0.001

	prevBody := prev.Close - prev.Open
	currBody := curr.Close - curr.Open

	// Tweezer Bottom: Two candles with nearly equal lows, opposite colors
	if math.Abs(prev.Low-curr.Low) <= tolerance && prevBody < 0 && currBody > 0 {
		confidence := 60.0
		if g.hasPriorTrend(klines, "down", config.PriorTrendCandles) {
			confidence += 15
		}

		return &CandlestickPattern{
			Type:        PatternTweezerBottom,
			Signal:      SignalBullish,
			Confidence:  math.Min(confidence, 100),
			Strength:    classifyPatternStrength(confidence),
			Description: "Tweezer Bottom: Equal lows with reversal, bullish signal",
			CandleCount: 2,
			DetectedAt:  time.Now(),
		}
	}

	// Tweezer Top: Two candles with nearly equal highs, opposite colors
	if math.Abs(prev.High-curr.High) <= tolerance && prevBody > 0 && currBody < 0 {
		confidence := 60.0
		if g.hasPriorTrend(klines, "up", config.PriorTrendCandles) {
			confidence += 15
		}

		return &CandlestickPattern{
			Type:        PatternTweezerTop,
			Signal:      SignalBearish,
			Confidence:  math.Min(confidence, 100),
			Strength:    classifyPatternStrength(confidence),
			Description: "Tweezer Top: Equal highs with reversal, bearish signal",
			CandleCount: 2,
			DetectedAt:  time.Now(),
		}
	}

	return nil
}

// ===== THREE CANDLE PATTERNS =====

// detectMorningEveningStar detects morning star (bullish) and evening star (bearish)
func (g *GinieAnalyzer) detectMorningEveningStar(klines []binance.Kline, config *CandlestickConfig) *CandlestickPattern {
	if len(klines) < 4 {
		return nil
	}

	first := klines[len(klines)-3]
	middle := klines[len(klines)-2]
	last := klines[len(klines)-1]

	firstBody := first.Close - first.Open
	middleBody := math.Abs(middle.Close - middle.Open)
	lastBody := last.Close - last.Open
	middleRange := middle.High - middle.Low

	// Middle candle should be small (star)
	middleBodyRatio := 0.0
	if middleRange > 0 {
		middleBodyRatio = middleBody / middleRange
	}

	if middleBodyRatio > 0.3 {
		return nil // Middle candle too large
	}

	// Morning Star: Downtrend, big red, small star, big green
	if firstBody < 0 && lastBody > 0 {
		// First candle is bearish, last is bullish
		// Star gaps down from first, last gaps up from star (or close to it)
		if math.Abs(firstBody) > middleBody*2 && lastBody > middleBody*2 {
			confidence := 70.0
			if g.hasPriorTrend(klines, "down", config.PriorTrendCandles) {
				confidence += 15
			}
			// Bonus if last candle closes above first candle's midpoint
			firstMid := (first.Open + first.Close) / 2
			if last.Close > firstMid {
				confidence += 10
			}

			return &CandlestickPattern{
				Type:        PatternMorningStar,
				Signal:      SignalBullish,
				Confidence:  math.Min(confidence, 100),
				Strength:    classifyPatternStrength(confidence),
				Description: "Morning Star: Three-candle bullish reversal pattern",
				CandleCount: 3,
				DetectedAt:  time.Now(),
			}
		}
	}

	// Evening Star: Uptrend, big green, small star, big red
	if firstBody > 0 && lastBody < 0 {
		if firstBody > middleBody*2 && math.Abs(lastBody) > middleBody*2 {
			confidence := 70.0
			if g.hasPriorTrend(klines, "up", config.PriorTrendCandles) {
				confidence += 15
			}
			// Bonus if last candle closes below first candle's midpoint
			firstMid := (first.Open + first.Close) / 2
			if last.Close < firstMid {
				confidence += 10
			}

			return &CandlestickPattern{
				Type:        PatternEveningStar,
				Signal:      SignalBearish,
				Confidence:  math.Min(confidence, 100),
				Strength:    classifyPatternStrength(confidence),
				Description: "Evening Star: Three-candle bearish reversal pattern",
				CandleCount: 3,
				DetectedAt:  time.Now(),
			}
		}
	}

	return nil
}

// detectThreeSoldiersCrows detects three white soldiers (bullish) and three black crows (bearish)
func (g *GinieAnalyzer) detectThreeSoldiersCrows(klines []binance.Kline, config *CandlestickConfig) *CandlestickPattern {
	if len(klines) < 4 {
		return nil
	}

	c1 := klines[len(klines)-3]
	c2 := klines[len(klines)-2]
	c3 := klines[len(klines)-1]

	body1 := c1.Close - c1.Open
	body2 := c2.Close - c2.Open
	body3 := c3.Close - c3.Open

	// Three White Soldiers: Three consecutive bullish candles with higher closes
	if body1 > 0 && body2 > 0 && body3 > 0 {
		// Each candle opens within previous body and closes higher
		if c2.Open >= c1.Open && c2.Close > c1.Close &&
		   c3.Open >= c2.Open && c3.Close > c2.Close {
			confidence := 75.0
			if g.hasPriorTrend(klines, "down", config.PriorTrendCandles) {
				confidence += 15
			}

			return &CandlestickPattern{
				Type:        PatternThreeWhite,
				Signal:      SignalBullish,
				Confidence:  math.Min(confidence, 100),
				Strength:    classifyPatternStrength(confidence),
				Description: "Three White Soldiers: Strong bullish continuation/reversal",
				CandleCount: 3,
				DetectedAt:  time.Now(),
			}
		}
	}

	// Three Black Crows: Three consecutive bearish candles with lower closes
	if body1 < 0 && body2 < 0 && body3 < 0 {
		if c2.Open <= c1.Open && c2.Close < c1.Close &&
		   c3.Open <= c2.Open && c3.Close < c2.Close {
			confidence := 75.0
			if g.hasPriorTrend(klines, "up", config.PriorTrendCandles) {
				confidence += 15
			}

			return &CandlestickPattern{
				Type:        PatternThreeBlack,
				Signal:      SignalBearish,
				Confidence:  math.Min(confidence, 100),
				Strength:    classifyPatternStrength(confidence),
				Description: "Three Black Crows: Strong bearish continuation/reversal",
				CandleCount: 3,
				DetectedAt:  time.Now(),
			}
		}
	}

	return nil
}

// ===== HELPER FUNCTIONS =====

// hasPriorTrend checks if there was a trend in the specified direction
func (g *GinieAnalyzer) hasPriorTrend(klines []binance.Kline, direction string, lookback int) bool {
	if len(klines) < lookback+2 {
		return false
	}

	// Get candles before the pattern (excluding last 1-2 candles)
	start := len(klines) - lookback - 2
	if start < 0 {
		start = 0
	}
	end := len(klines) - 2

	upCandles := 0
	downCandles := 0

	for i := start; i < end; i++ {
		if klines[i].Close > klines[i].Open {
			upCandles++
		} else if klines[i].Close < klines[i].Open {
			downCandles++
		}
	}

	total := end - start
	if total == 0 {
		return false
	}

	threshold := 0.6 // 60% of candles in one direction

	if direction == "up" {
		return float64(upCandles)/float64(total) >= threshold
	} else if direction == "down" {
		return float64(downCandles)/float64(total) >= threshold
	}

	return false
}

// classifyPatternStrength classifies pattern confidence into strength category
func classifyPatternStrength(confidence float64) string {
	if confidence >= 80 {
		return "strong"
	} else if confidence >= 65 {
		return "moderate"
	}
	return "weak"
}
