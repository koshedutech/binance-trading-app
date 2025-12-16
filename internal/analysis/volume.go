package analysis

import (
	"binance-trading-bot/internal/binance"
	"math"
)

// VolumeAnalyzer provides volume-based technical analysis
type VolumeAnalyzer struct {
	avgPeriod int // Period for average volume calculation
}

// VolumeProfile represents volume analysis results
type VolumeProfile struct {
	CurrentVolume  float64
	AverageVolume  float64
	VolumeRatio    float64 // Current / Average
	IsHighVolume   bool    // Volume > 2x average
	IsClimaxVolume bool    // Volume > 3x average
	OBV            float64 // On-Balance Volume
	VolumeType     string  // "buying", "selling", "neutral"
}

// NewVolumeAnalyzer creates a new volume analyzer
func NewVolumeAnalyzer(avgPeriod int) *VolumeAnalyzer {
	if avgPeriod <= 0 {
		avgPeriod = 20 // Default 20-period average
	}
	return &VolumeAnalyzer{
		avgPeriod: avgPeriod,
	}
}

// AnalyzeVolume performs comprehensive volume analysis
func (va *VolumeAnalyzer) AnalyzeVolume(candles []binance.Kline) *VolumeProfile {
	if len(candles) == 0 {
		return nil
	}

	currentCandle := candles[len(candles)-1]
	currentVolume := currentCandle.Volume

	// Calculate average volume
	avgVolume := va.CalculateAverageVolume(candles)

	// Calculate volume ratio
	var volumeRatio float64
	if avgVolume > 0 {
		volumeRatio = currentVolume / avgVolume
	}

	// Determine volume type
	volumeType := va.DetermineVolumeType(currentCandle)

	// Calculate OBV
	obv := va.CalculateOBV(candles)

	return &VolumeProfile{
		CurrentVolume:  currentVolume,
		AverageVolume:  avgVolume,
		VolumeRatio:    volumeRatio,
		IsHighVolume:   volumeRatio > 2.0,
		IsClimaxVolume: volumeRatio > 3.0,
		OBV:            obv,
		VolumeType:     volumeType,
	}
}

// CalculateAverageVolume calculates the average volume over the specified period
func (va *VolumeAnalyzer) CalculateAverageVolume(candles []binance.Kline) float64 {
	if len(candles) == 0 {
		return 0
	}

	period := va.avgPeriod
	if len(candles) < period {
		period = len(candles)
	}

	sum := 0.0
	for i := len(candles) - period; i < len(candles); i++ {
		sum += candles[i].Volume
	}

	return sum / float64(period)
}

// IsVolumeSpikePresent checks if there's a volume spike
func (va *VolumeAnalyzer) IsVolumeSpikePresent(candles []binance.Kline, threshold float64) bool {
	profile := va.AnalyzeVolume(candles)
	if profile == nil {
		return false
	}

	return profile.VolumeRatio >= threshold
}

// DetermineVolumeType identifies if volume is buying or selling pressure
func (va *VolumeAnalyzer) DetermineVolumeType(candle binance.Kline) string {
	// Calculate candle body and direction
	bodySize := math.Abs(candle.Close - candle.Open)
	upperWick := candle.High - math.Max(candle.Open, candle.Close)
	lowerWick := math.Min(candle.Open, candle.Close) - candle.Low

	// If candle closed higher, it's buying volume
	if candle.Close > candle.Open {
		// Strong buying if small upper wick
		if upperWick < bodySize*0.2 {
			return "buying"
		}
		return "neutral"
	} else if candle.Close < candle.Open {
		// Strong selling if small lower wick
		if lowerWick < bodySize*0.2 {
			return "selling"
		}
		return "neutral"
	}

	return "neutral"
}

// CalculateOBV calculates On-Balance Volume
// OBV = Previous OBV + Current Volume (if close up) or - Current Volume (if close down)
func (va *VolumeAnalyzer) CalculateOBV(candles []binance.Kline) float64 {
	if len(candles) == 0 {
		return 0
	}

	obv := 0.0

	for i := 1; i < len(candles); i++ {
		if candles[i].Close > candles[i-1].Close {
			obv += candles[i].Volume
		} else if candles[i].Close < candles[i-1].Close {
			obv -= candles[i].Volume
		}
		// If close == previous close, OBV unchanged
	}

	return obv
}

// IsOBVBullish checks if OBV is trending up (bullish)
func (va *VolumeAnalyzer) IsOBVBullish(candles []binance.Kline, period int) bool {
	if len(candles) < period+1 {
		return false
	}

	// Calculate OBV for last 'period' candles
	recentCandles := candles[len(candles)-period:]
	currentOBV := va.CalculateOBV(recentCandles)

	// Calculate OBV for previous period
	previousCandles := candles[len(candles)-period-1 : len(candles)-1]
	previousOBV := va.CalculateOBV(previousCandles)

	return currentOBV > previousOBV
}

// DetectVolumeDryUp identifies consolidation with declining volume
func (va *VolumeAnalyzer) DetectVolumeDryUp(candles []binance.Kline, period int) bool {
	if len(candles) < period {
		return false
	}

	recentCandles := candles[len(candles)-period:]

	// Check if volume is declining over the period
	firstHalfAvg := 0.0
	secondHalfAvg := 0.0

	mid := period / 2
	for i := 0; i < mid; i++ {
		firstHalfAvg += recentCandles[i].Volume
	}
	for i := mid; i < period; i++ {
		secondHalfAvg += recentCandles[i].Volume
	}

	firstHalfAvg /= float64(mid)
	secondHalfAvg /= float64(period - mid)

	// Volume dry-up: second half has significantly less volume
	return secondHalfAvg < firstHalfAvg*0.7
}

// GetVolumeConfirmation checks if volume confirms a breakout or reversal
func (va *VolumeAnalyzer) GetVolumeConfirmation(candles []binance.Kline, priceBreakout bool) bool {
	if len(candles) == 0 {
		return false
	}

	profile := va.AnalyzeVolume(candles)
	if profile == nil {
		return false
	}

	if priceBreakout {
		// Breakout should have high volume (>2x average)
		return profile.IsHighVolume
	} else {
		// Reversal should have volume spike
		return profile.VolumeRatio > 1.5
	}
}

// CalculateVolumeWeightedAveragePrice calculates VWAP for the given candles
func (va *VolumeAnalyzer) CalculateVolumeWeightedAveragePrice(candles []binance.Kline) float64 {
	if len(candles) == 0 {
		return 0
	}

	totalVolumePrice := 0.0
	totalVolume := 0.0

	for _, candle := range candles {
		typicalPrice := (candle.High + candle.Low + candle.Close) / 3
		totalVolumePrice += typicalPrice * candle.Volume
		totalVolume += candle.Volume
	}

	if totalVolume == 0 {
		return 0
	}

	return totalVolumePrice / totalVolume
}
