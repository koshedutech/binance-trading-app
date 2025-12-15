package strategy

import (
	"binance-trading-bot/internal/binance"
	"math"
)

// CandlestickPattern represents a detected candlestick pattern
type CandlestickPattern struct {
	Name        string
	Type        string // "BULLISH" or "BEARISH"
	Reliability string // "HIGH", "MEDIUM", "LOW"
	Description string
}

// ============================================================================
// SINGLE CANDLE PATTERNS
// ============================================================================

// IsHammer detects Hammer pattern (bullish reversal)
func IsHammer(candle binance.Kline) bool {
	body := math.Abs(candle.Close - candle.Open)
	upperShadow := candle.High - math.Max(candle.Open, candle.Close)
	lowerShadow := math.Min(candle.Open, candle.Close) - candle.Low

	// Hammer criteria:
	// 1. Small body at the upper end
	// 2. Long lower shadow (at least 2x body)
	// 3. Little or no upper shadow
	if body == 0 {
		return false
	}

	return lowerShadow >= body*2 && upperShadow <= body*0.5
}

// IsInvertedHammer detects Inverted Hammer pattern (bullish reversal)
func IsInvertedHammer(candle binance.Kline) bool {
	body := math.Abs(candle.Close - candle.Open)
	upperShadow := candle.High - math.Max(candle.Open, candle.Close)
	lowerShadow := math.Min(candle.Open, candle.Close) - candle.Low

	// Inverted Hammer criteria:
	// 1. Small body at the lower end
	// 2. Long upper shadow (at least 2x body)
	// 3. Little or no lower shadow
	if body == 0 {
		return false
	}

	return upperShadow >= body*2 && lowerShadow <= body*0.5
}

// IsShootingStar detects Shooting Star pattern (bearish reversal)
func IsShootingStar(candle binance.Kline) bool {
	// Shooting star is same as inverted hammer but in uptrend (bearish)
	return IsInvertedHammer(candle)
}

// IsHangingMan detects Hanging Man pattern (bearish reversal)
func IsHangingMan(candle binance.Kline) bool {
	// Hanging man is same as hammer but in uptrend (bearish)
	return IsHammer(candle)
}

// IsDoji detects Doji pattern (indecision)
func IsDoji(candle binance.Kline) bool {
	body := math.Abs(candle.Close - candle.Open)
	totalRange := candle.High - candle.Low

	if totalRange == 0 {
		return false
	}

	// Doji has very small body (less than 5% of total range)
	return body <= totalRange*0.05
}

// IsMarubozu detects Marubozu pattern (strong momentum)
func IsMarubozu(candle binance.Kline) bool {
	body := math.Abs(candle.Close - candle.Open)
	upperShadow := candle.High - math.Max(candle.Open, candle.Close)
	lowerShadow := math.Min(candle.Open, candle.Close) - candle.Low

	if body == 0 {
		return false
	}

	// Marubozu has little to no shadows (shadows < 5% of body)
	return upperShadow <= body*0.05 && lowerShadow <= body*0.05
}

// IsBullishMarubozu detects Bullish Marubozu (strong bullish momentum)
func IsBullishMarubozu(candle binance.Kline) bool {
	return IsMarubozu(candle) && candle.Close > candle.Open
}

// IsBearishMarubozu detects Bearish Marubozu (strong bearish momentum)
func IsBearishMarubozu(candle binance.Kline) bool {
	return IsMarubozu(candle) && candle.Close < candle.Open
}

// IsSpinningTop detects Spinning Top pattern (indecision)
func IsSpinningTop(candle binance.Kline) bool {
	body := math.Abs(candle.Close - candle.Open)
	upperShadow := candle.High - math.Max(candle.Open, candle.Close)
	lowerShadow := math.Min(candle.Open, candle.Close) - candle.Low

	if body == 0 {
		return false
	}

	// Spinning top has small body with equal upper and lower shadows
	avgShadow := (upperShadow + lowerShadow) / 2
	return body < avgShadow && math.Abs(upperShadow-lowerShadow) <= body
}

// ============================================================================
// TWO CANDLE PATTERNS
// ============================================================================

// IsBullishEngulfing detects Bullish Engulfing pattern
func IsBullishEngulfing(prev, current binance.Kline) bool {
	// Previous candle is bearish
	prevBearish := prev.Close < prev.Open
	// Current candle is bullish
	currentBullish := current.Close > current.Open

	if !prevBearish || !currentBullish {
		return false
	}

	// Current candle's body engulfs previous candle's body
	return current.Open <= prev.Close && current.Close >= prev.Open
}

// IsBearishEngulfing detects Bearish Engulfing pattern
func IsBearishEngulfing(prev, current binance.Kline) bool {
	// Previous candle is bullish
	prevBullish := prev.Close > prev.Open
	// Current candle is bearish
	currentBearish := current.Close < current.Open

	if !prevBullish || !currentBearish {
		return false
	}

	// Current candle's body engulfs previous candle's body
	return current.Open >= prev.Close && current.Close <= prev.Open
}

// IsPiercing detects Piercing Pattern (bullish reversal)
func IsPiercing(prev, current binance.Kline) bool {
	// Previous candle is bearish
	prevBearish := prev.Close < prev.Open
	// Current candle is bullish
	currentBullish := current.Close > current.Open

	if !prevBearish || !currentBullish {
		return false
	}

	prevMidpoint := (prev.Open + prev.Close) / 2

	// Current opens below previous low and closes above midpoint
	return current.Open < prev.Close && current.Close > prevMidpoint && current.Close < prev.Open
}

// IsDarkCloudCover detects Dark Cloud Cover (bearish reversal)
func IsDarkCloudCover(prev, current binance.Kline) bool {
	// Previous candle is bullish
	prevBullish := prev.Close > prev.Open
	// Current candle is bearish
	currentBearish := current.Close < current.Open

	if !prevBullish || !currentBearish {
		return false
	}

	prevMidpoint := (prev.Open + prev.Close) / 2

	// Current opens above previous high and closes below midpoint
	return current.Open > prev.Close && current.Close < prevMidpoint && current.Close > prev.Open
}

// IsTweezerTop detects Tweezer Top (bearish reversal)
func IsTweezerTop(prev, current binance.Kline) bool {
	// Both candles have similar highs (within 0.1%)
	tolerance := prev.High * 0.001
	return math.Abs(prev.High-current.High) <= tolerance && prev.Close > prev.Open && current.Close < current.Open
}

// IsTweezerBottom detects Tweezer Bottom (bullish reversal)
func IsTweezerBottom(prev, current binance.Kline) bool {
	// Both candles have similar lows (within 0.1%)
	tolerance := prev.Low * 0.001
	return math.Abs(prev.Low-current.Low) <= tolerance && prev.Close < prev.Open && current.Close > current.Open
}

// ============================================================================
// THREE CANDLE PATTERNS
// ============================================================================

// IsMorningStar detects Morning Star pattern (bullish reversal)
func IsMorningStar(first, second, third binance.Kline) bool {
	// First candle: long bearish
	firstBearish := first.Close < first.Open
	firstBody := math.Abs(first.Close - first.Open)

	// Second candle: small body (star)
	secondBody := math.Abs(second.Close - second.Open)

	// Third candle: long bullish
	thirdBullish := third.Close > third.Open
	thirdBody := math.Abs(third.Close - third.Open)

	if !firstBearish || !thirdBullish {
		return false
	}

	// Second candle should gap down and have small body
	// Third candle should close above midpoint of first candle
	firstMidpoint := (first.Open + first.Close) / 2

	return secondBody < firstBody*0.3 &&
		secondBody < thirdBody*0.3 &&
		third.Close > firstMidpoint
}

// IsEveningStar detects Evening Star pattern (bearish reversal)
func IsEveningStar(first, second, third binance.Kline) bool {
	// First candle: long bullish
	firstBullish := first.Close > first.Open
	firstBody := math.Abs(first.Close - first.Open)

	// Second candle: small body (star)
	secondBody := math.Abs(second.Close - second.Open)

	// Third candle: long bearish
	thirdBearish := third.Close < third.Open
	thirdBody := math.Abs(third.Close - third.Open)

	if !firstBullish || !thirdBearish {
		return false
	}

	// Second candle should gap up and have small body
	// Third candle should close below midpoint of first candle
	firstMidpoint := (first.Open + first.Close) / 2

	return secondBody < firstBody*0.3 &&
		secondBody < thirdBody*0.3 &&
		third.Close < firstMidpoint
}

// IsThreeWhiteSoldiers detects Three White Soldiers (bullish continuation)
func IsThreeWhiteSoldiers(first, second, third binance.Kline) bool {
	// All three candles are bullish
	allBullish := first.Close > first.Open &&
		second.Close > second.Open &&
		third.Close > third.Open

	if !allBullish {
		return false
	}

	// Each candle opens within the body of the previous candle
	// Each candle closes progressively higher
	return second.Open > first.Open && second.Open < first.Close &&
		third.Open > second.Open && third.Open < second.Close &&
		second.Close > first.Close &&
		third.Close > second.Close
}

// IsThreeBlackCrows detects Three Black Crows (bearish continuation)
func IsThreeBlackCrows(first, second, third binance.Kline) bool {
	// All three candles are bearish
	allBearish := first.Close < first.Open &&
		second.Close < second.Open &&
		third.Close < third.Open

	if !allBearish {
		return false
	}

	// Each candle opens within the body of the previous candle
	// Each candle closes progressively lower
	return second.Open < first.Open && second.Open > first.Close &&
		third.Open < second.Open && third.Open > second.Close &&
		second.Close < first.Close &&
		third.Close < second.Close
}

// ============================================================================
// PATTERN DETECTION FUNCTIONS
// ============================================================================

// DetectSingleCandlePatterns detects patterns in a single candle
func DetectSingleCandlePatterns(candle binance.Kline) []CandlestickPattern {
	var patterns []CandlestickPattern

	if IsHammer(candle) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Hammer",
			Type:        "BULLISH",
			Reliability: "MEDIUM",
			Description: "Bullish reversal pattern with long lower shadow",
		})
	}

	if IsInvertedHammer(candle) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Inverted Hammer",
			Type:        "BULLISH",
			Reliability: "MEDIUM",
			Description: "Bullish reversal pattern with long upper shadow",
		})
	}

	if IsBullishMarubozu(candle) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Bullish Marubozu",
			Type:        "BULLISH",
			Reliability: "HIGH",
			Description: "Strong bullish momentum with no shadows",
		})
	}

	if IsBearishMarubozu(candle) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Bearish Marubozu",
			Type:        "BEARISH",
			Reliability: "HIGH",
			Description: "Strong bearish momentum with no shadows",
		})
	}

	if IsDoji(candle) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Doji",
			Type:        "NEUTRAL",
			Reliability: "MEDIUM",
			Description: "Indecision pattern with very small body",
		})
	}

	return patterns
}

// DetectTwoCandlePatterns detects patterns in two candles
func DetectTwoCandlePatterns(prev, current binance.Kline) []CandlestickPattern {
	var patterns []CandlestickPattern

	if IsBullishEngulfing(prev, current) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Bullish Engulfing",
			Type:        "BULLISH",
			Reliability: "HIGH",
			Description: "Strong bullish reversal - current candle engulfs previous",
		})
	}

	if IsBearishEngulfing(prev, current) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Bearish Engulfing",
			Type:        "BEARISH",
			Reliability: "HIGH",
			Description: "Strong bearish reversal - current candle engulfs previous",
		})
	}

	if IsPiercing(prev, current) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Piercing Pattern",
			Type:        "BULLISH",
			Reliability: "MEDIUM",
			Description: "Bullish reversal with penetration above midpoint",
		})
	}

	if IsDarkCloudCover(prev, current) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Dark Cloud Cover",
			Type:        "BEARISH",
			Reliability: "MEDIUM",
			Description: "Bearish reversal with penetration below midpoint",
		})
	}

	return patterns
}

// DetectThreeCandlePatterns detects patterns in three candles
func DetectThreeCandlePatterns(first, second, third binance.Kline) []CandlestickPattern {
	var patterns []CandlestickPattern

	if IsMorningStar(first, second, third) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Morning Star",
			Type:        "BULLISH",
			Reliability: "HIGH",
			Description: "Strong bullish reversal with star formation",
		})
	}

	if IsEveningStar(first, second, third) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Evening Star",
			Type:        "BEARISH",
			Reliability: "HIGH",
			Description: "Strong bearish reversal with star formation",
		})
	}

	if IsThreeWhiteSoldiers(first, second, third) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Three White Soldiers",
			Type:        "BULLISH",
			Reliability: "HIGH",
			Description: "Strong bullish continuation with three consecutive bullish candles",
		})
	}

	if IsThreeBlackCrows(first, second, third) {
		patterns = append(patterns, CandlestickPattern{
			Name:        "Three Black Crows",
			Type:        "BEARISH",
			Reliability: "HIGH",
			Description: "Strong bearish continuation with three consecutive bearish candles",
		})
	}

	return patterns
}

// DetectAllPatterns detects all candlestick patterns in the given klines
func DetectAllPatterns(klines []binance.Kline) []CandlestickPattern {
	var allPatterns []CandlestickPattern

	if len(klines) == 0 {
		return allPatterns
	}

	// Single candle patterns (current candle)
	current := klines[len(klines)-1]
	allPatterns = append(allPatterns, DetectSingleCandlePatterns(current)...)

	// Two candle patterns
	if len(klines) >= 2 {
		prev := klines[len(klines)-2]
		allPatterns = append(allPatterns, DetectTwoCandlePatterns(prev, current)...)
	}

	// Three candle patterns
	if len(klines) >= 3 {
		first := klines[len(klines)-3]
		second := klines[len(klines)-2]
		third := klines[len(klines)-1]
		allPatterns = append(allPatterns, DetectThreeCandlePatterns(first, second, third)...)
	}

	return allPatterns
}
