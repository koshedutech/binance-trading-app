package strategy

import (
	"binance-trading-bot/internal/binance"
	"math"
)

// ============================================================================
// MOVING AVERAGES
// ============================================================================

// CalculateSMA calculates Simple Moving Average
func CalculateSMA(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	sum := 0.0
	startIdx := len(klines) - period

	for i := startIdx; i < len(klines); i++ {
		sum += klines[i].Close
	}

	return sum / float64(period)
}

// CalculateEMA calculates Exponential Moving Average
func CalculateEMA(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// Calculate initial SMA as starting point
	sma := CalculateSMA(klines[:period], period)

	// Calculate multiplier
	multiplier := 2.0 / float64(period+1)

	// Calculate EMA
	ema := sma
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close * multiplier) + (ema * (1 - multiplier))
	}

	return ema
}

// ============================================================================
// RSI (Relative Strength Index)
// ============================================================================

// CalculateRSI calculates the Relative Strength Index
func CalculateRSI(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 50.0 // Neutral RSI
	}

	gains := 0.0
	losses := 0.0

	// Calculate initial average gain and loss
	for i := len(klines) - period; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// ============================================================================
// MACD (Moving Average Convergence Divergence)
// ============================================================================

// MACDResult holds MACD indicator values
type MACDResult struct {
	MACD      float64
	Signal    float64
	Histogram float64
}

// CalculateMACD calculates MACD, Signal line, and Histogram
func CalculateMACD(klines []binance.Kline, fastPeriod, slowPeriod, signalPeriod int) *MACDResult {
	if len(klines) < slowPeriod+signalPeriod {
		return &MACDResult{0, 0, 0}
	}

	// Calculate fast and slow EMAs
	fastEMA := CalculateEMA(klines, fastPeriod)
	slowEMA := CalculateEMA(klines, slowPeriod)

	// MACD line
	macdLine := fastEMA - slowEMA

	// For signal line, we need to calculate EMA of MACD values
	// Simplified: using the current MACD as approximation
	// In production, you'd maintain MACD history
	signalLine := macdLine * 0.8 // Simplified approximation

	// Histogram
	histogram := macdLine - signalLine

	return &MACDResult{
		MACD:      macdLine,
		Signal:    signalLine,
		Histogram: histogram,
	}
}

// ============================================================================
// BOLLINGER BANDS
// ============================================================================

// BollingerBandsResult holds Bollinger Bands values
type BollingerBandsResult struct {
	Upper  float64
	Middle float64
	Lower  float64
}

// CalculateBollingerBands calculates Bollinger Bands
func CalculateBollingerBands(klines []binance.Kline, period int, stdDevMultiplier float64) *BollingerBandsResult {
	if len(klines) < period {
		return &BollingerBandsResult{0, 0, 0}
	}

	// Middle band is SMA
	middle := CalculateSMA(klines, period)

	// Calculate standard deviation
	variance := 0.0
	startIdx := len(klines) - period

	for i := startIdx; i < len(klines); i++ {
		diff := klines[i].Close - middle
		variance += diff * diff
	}

	stdDev := math.Sqrt(variance / float64(period))

	// Upper and lower bands
	upper := middle + (stdDev * stdDevMultiplier)
	lower := middle - (stdDev * stdDevMultiplier)

	return &BollingerBandsResult{
		Upper:  upper,
		Middle: middle,
		Lower:  lower,
	}
}

// ============================================================================
// ATR (Average True Range)
// ============================================================================

// CalculateATR calculates Average True Range
func CalculateATR(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}

	trSum := 0.0
	startIdx := len(klines) - period

	for i := startIdx; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr := math.Max(
			high-low,
			math.Max(
				math.Abs(high-prevClose),
				math.Abs(low-prevClose),
			),
		)

		trSum += tr
	}

	return trSum / float64(period)
}

// ============================================================================
// STOCHASTIC OSCILLATOR
// ============================================================================

// StochasticResult holds Stochastic Oscillator values
type StochasticResult struct {
	K float64
	D float64
}

// CalculateStochastic calculates Stochastic Oscillator (%K and %D)
func CalculateStochastic(klines []binance.Kline, kPeriod, dPeriod int) *StochasticResult {
	if len(klines) < kPeriod {
		return &StochasticResult{50, 50}
	}

	// Find highest high and lowest low in the period
	startIdx := len(klines) - kPeriod
	highestHigh := klines[startIdx].High
	lowestLow := klines[startIdx].Low

	for i := startIdx; i < len(klines); i++ {
		if klines[i].High > highestHigh {
			highestHigh = klines[i].High
		}
		if klines[i].Low < lowestLow {
			lowestLow = klines[i].Low
		}
	}

	// Calculate %K
	currentClose := klines[len(klines)-1].Close
	percentK := 0.0
	if highestHigh != lowestLow {
		percentK = ((currentClose - lowestLow) / (highestHigh - lowestLow)) * 100
	}

	// %D is SMA of %K (simplified as current %K for now)
	percentD := percentK * 0.9 // Simplified

	return &StochasticResult{
		K: percentK,
		D: percentD,
	}
}

// ============================================================================
// ADX (Average Directional Index)
// ============================================================================

// CalculateADX calculates Average Directional Index
func CalculateADX(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}

	// Simplified ADX calculation
	// Full implementation would require +DI and -DI calculations
	atr := CalculateATR(klines, period)

	// Approximate ADX based on price movement volatility
	priceRange := klines[len(klines)-1].High - klines[len(klines)-1].Low

	if atr == 0 {
		return 0
	}

	adx := (priceRange / atr) * 25 // Scaled approximation
	if adx > 100 {
		adx = 100
	}

	return adx
}

// ============================================================================
// VOLUME ANALYSIS
// ============================================================================

// CalculateAverageVolume calculates average volume over a period
func CalculateAverageVolume(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		period = len(klines)
	}

	sum := 0.0
	startIdx := len(klines) - period

	for i := startIdx; i < len(klines); i++ {
		sum += klines[i].Volume
	}

	return sum / float64(period)
}

// IsVolumeSpike checks if current volume is significantly higher than average
func IsVolumeSpike(klines []binance.Kline, period int, multiplier float64) bool {
	if len(klines) < period+1 {
		return false
	}

	avgVolume := CalculateAverageVolume(klines[:len(klines)-1], period)
	currentVolume := klines[len(klines)-1].Volume

	return currentVolume >= avgVolume*multiplier
}

// ============================================================================
// MOMENTUM INDICATORS
// ============================================================================

// CalculateMomentum calculates price momentum
func CalculateMomentum(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}

	currentPrice := klines[len(klines)-1].Close
	pastPrice := klines[len(klines)-period-1].Close

	return ((currentPrice - pastPrice) / pastPrice) * 100
}

// CalculateROC calculates Rate of Change
func CalculateROC(klines []binance.Kline, period int) float64 {
	return CalculateMomentum(klines, period)
}

// ============================================================================
// FIBONACCI RETRACEMENT LEVELS
// ============================================================================

// FibonacciLevels holds Fibonacci retracement levels
type FibonacciLevels struct {
	Level0    float64 // 0% (High)
	Level236  float64 // 23.6%
	Level382  float64 // 38.2%
	Level50   float64 // 50%
	Level618  float64 // 61.8%
	Level100  float64 // 100% (Low)
}

// CalculateFibonacciLevels calculates Fibonacci retracement levels
func CalculateFibonacciLevels(klines []binance.Kline, period int) *FibonacciLevels {
	if len(klines) < period {
		return &FibonacciLevels{}
	}

	// Find high and low in the period
	startIdx := len(klines) - period
	high := klines[startIdx].High
	low := klines[startIdx].Low

	for i := startIdx; i < len(klines); i++ {
		if klines[i].High > high {
			high = klines[i].High
		}
		if klines[i].Low < low {
			low = klines[i].Low
		}
	}

	diff := high - low

	return &FibonacciLevels{
		Level0:    high,
		Level236:  high - (diff * 0.236),
		Level382:  high - (diff * 0.382),
		Level50:   high - (diff * 0.50),
		Level618:  high - (diff * 0.618),
		Level100:  low,
	}
}

// ============================================================================
// SUPPORT AND RESISTANCE
// ============================================================================

// FindSupportResistance identifies support and resistance levels
func FindSupportResistance(klines []binance.Kline, period int) (support float64, resistance float64) {
	if len(klines) < period {
		return 0, 0
	}

	startIdx := len(klines) - period
	high := klines[startIdx].High
	low := klines[startIdx].Low

	for i := startIdx; i < len(klines); i++ {
		if klines[i].High > high {
			high = klines[i].High
		}
		if klines[i].Low < low {
			low = klines[i].Low
		}
	}

	return low, high
}

// ============================================================================
// TREND DETECTION
// ============================================================================

// TrendDirection represents the current trend
type TrendDirection string

const (
	TrendUp      TrendDirection = "UPTREND"
	TrendDown    TrendDirection = "DOWNTREND"
	TrendSideways TrendDirection = "SIDEWAYS"
)

// DetectTrend detects the current trend using EMAs
func DetectTrend(klines []binance.Kline, fastPeriod, slowPeriod int) TrendDirection {
	if len(klines) < slowPeriod {
		return TrendSideways
	}

	fastEMA := CalculateEMA(klines, fastPeriod)
	slowEMA := CalculateEMA(klines, slowPeriod)

	difference := math.Abs(fastEMA-slowEMA) / slowEMA * 100

	// If EMAs are very close (within 0.5%), consider it sideways
	if difference < 0.5 {
		return TrendSideways
	}

	if fastEMA > slowEMA {
		return TrendUp
	}

	return TrendDown
}

// ============================================================================
// PIVOT POINTS
// ============================================================================

// PivotPoints holds pivot point levels
type PivotPoints struct {
	PP  float64 // Pivot Point
	R1  float64 // Resistance 1
	R2  float64 // Resistance 2
	R3  float64 // Resistance 3
	S1  float64 // Support 1
	S2  float64 // Support 2
	S3  float64 // Support 3
}

// CalculateStandardPivotPoints calculates standard pivot points
func CalculateStandardPivotPoints(klines []binance.Kline) *PivotPoints {
	if len(klines) == 0 {
		return &PivotPoints{}
	}

	// Use previous day's candle (or last completed candle)
	lastCandle := klines[len(klines)-1]
	high := lastCandle.High
	low := lastCandle.Low
	close := lastCandle.Close

	// Calculate Pivot Point
	pp := (high + low + close) / 3

	// Calculate resistance and support levels
	r1 := (2 * pp) - low
	s1 := (2 * pp) - high
	r2 := pp + (high - low)
	s2 := pp - (high - low)
	r3 := high + 2*(pp-low)
	s3 := low - 2*(high-pp)

	return &PivotPoints{
		PP: pp,
		R1: r1,
		R2: r2,
		R3: r3,
		S1: s1,
		S2: s2,
		S3: s3,
	}
}

// CalculateFibonacciPivotPoints calculates Fibonacci pivot points
func CalculateFibonacciPivotPoints(klines []binance.Kline) *PivotPoints {
	if len(klines) == 0 {
		return &PivotPoints{}
	}

	lastCandle := klines[len(klines)-1]
	high := lastCandle.High
	low := lastCandle.Low
	close := lastCandle.Close

	// Calculate Pivot Point
	pp := (high + low + close) / 3
	range_ := high - low

	// Calculate Fibonacci levels
	r1 := pp + (range_ * 0.382)
	r2 := pp + (range_ * 0.618)
	r3 := pp + (range_ * 1.000)
	s1 := pp - (range_ * 0.382)
	s2 := pp - (range_ * 0.618)
	s3 := pp - (range_ * 1.000)

	return &PivotPoints{
		PP: pp,
		R1: r1,
		R2: r2,
		R3: r3,
		S1: s1,
		S2: s2,
		S3: s3,
	}
}

// CheckPivotBreakout checks if price has broken above/below pivot levels
func CheckPivotBreakout(currentPrice float64, pivots *PivotPoints, threshold float64) (bool, string) {
	tolerance := currentPrice * threshold // e.g., 0.001 = 0.1%

	// Check resistance breakouts (bullish)
	if currentPrice >= pivots.R1-tolerance && currentPrice <= pivots.R1+tolerance {
		return true, "R1 Breakout"
	}
	if currentPrice >= pivots.R2-tolerance && currentPrice <= pivots.R2+tolerance {
		return true, "R2 Breakout"
	}
	if currentPrice >= pivots.R3-tolerance && currentPrice <= pivots.R3+tolerance {
		return true, "R3 Breakout"
	}

	// Check support bounces (bullish)
	if currentPrice >= pivots.S1-tolerance && currentPrice <= pivots.S1+tolerance {
		return true, "S1 Bounce"
	}
	if currentPrice >= pivots.S2-tolerance && currentPrice <= pivots.S2+tolerance {
		return true, "S2 Bounce"
	}
	if currentPrice >= pivots.S3-tolerance && currentPrice <= pivots.S3+tolerance {
		return true, "S3 Bounce"
	}

	// Check pivot point itself
	if currentPrice >= pivots.PP-tolerance && currentPrice <= pivots.PP+tolerance {
		return true, "Pivot Point"
	}

	return false, ""
}

// IsPriceAbovePivot checks if price is trading above the pivot point
func IsPriceAbovePivot(currentPrice float64, pivots *PivotPoints) bool {
	return currentPrice > pivots.PP
}

// GetNearestPivotLevel returns the nearest pivot level to current price
func GetNearestPivotLevel(currentPrice float64, pivots *PivotPoints) (float64, string) {
	levels := map[string]float64{
		"R3": pivots.R3,
		"R2": pivots.R2,
		"R1": pivots.R1,
		"PP": pivots.PP,
		"S1": pivots.S1,
		"S2": pivots.S2,
		"S3": pivots.S3,
	}

	minDiff := math.MaxFloat64
	nearestLevel := pivots.PP
	nearestName := "PP"

	for name, level := range levels {
		diff := math.Abs(currentPrice - level)
		if diff < minDiff {
			minDiff = diff
			nearestLevel = level
			nearestName = name
		}
	}

	return nearestLevel, nearestName
}
