package strategy

import (
	"binance-trading-bot/internal/analysis"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/confluence"
	"binance-trading-bot/internal/patterns"
	"fmt"
	"log"
)

// PatternConfluenceStrategy combines patterns, FVG, volume, and trend analysis
type PatternConfluenceStrategy struct {
	symbol           string
	interval         string
	stopLossPercent  float64
	takeProfitPercent float64
	minConfluenceScore float64

	// Analysis components
	tfManager    *analysis.TimeframeManager
	fvgDetector  *analysis.FVGDetector
	volumeAnalyzer *analysis.VolumeAnalyzer
	trendAnalyzer  *analysis.TrendAnalyzer
	patternDetector *patterns.PatternDetector
	confluenceScorer *confluence.ConfluenceScorer
}

// PatternConfluenceConfig holds strategy configuration
type PatternConfluenceConfig struct {
	Symbol             string
	Interval           string
	StopLossPercent    float64
	TakeProfitPercent  float64
	MinConfluenceScore float64
	FVGProximityPercent float64
}

// NewPatternConfluenceStrategy creates a new integrated strategy
func NewPatternConfluenceStrategy(client *binance.Client, config *PatternConfluenceConfig) *PatternConfluenceStrategy {
	return &PatternConfluenceStrategy{
		symbol:            config.Symbol,
		interval:          config.Interval,
		stopLossPercent:   config.StopLossPercent,
		takeProfitPercent: config.TakeProfitPercent,
		minConfluenceScore: config.MinConfluenceScore,

		tfManager:        analysis.NewTimeframeManager(client),
		fvgDetector:      analysis.NewFVGDetector(0.1),
		volumeAnalyzer:   analysis.NewVolumeAnalyzer(20),
		trendAnalyzer:    analysis.NewTrendAnalyzer(5),
		patternDetector:  patterns.NewPatternDetector(0.5),
		confluenceScorer: confluence.NewConfluenceScorer(),
	}
}

// Name returns the strategy name
func (pcs *PatternConfluenceStrategy) Name() string {
	return "pattern_confluence_" + pcs.symbol
}

// GetSymbol returns the trading symbol
func (pcs *PatternConfluenceStrategy) GetSymbol() string {
	return pcs.symbol
}

// GetInterval returns the timeframe
func (pcs *PatternConfluenceStrategy) GetInterval() string {
	return pcs.interval
}

// Evaluate performs comprehensive multi-factor analysis
func (pcs *PatternConfluenceStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
	if len(klines) < 100 {
		return nil, fmt.Errorf("insufficient candles for analysis (need 100+, got %d)", len(klines))
	}

	log.Printf("[%s] Evaluating confluence strategy at price %.2f", pcs.symbol, currentPrice)

	// STEP 1: TREND ANALYSIS (Higher Timeframe Context)
	structure := pcs.trendAnalyzer.AnalyzeStructure(klines)
	if structure == nil {
		return nil, fmt.Errorf("failed to analyze market structure")
	}

	trendScore := structure.TrendStrength
	log.Printf("[%s] Trend: %s (Strength: %.2f)", pcs.symbol, structure.Trend, trendScore)

	// STEP 2: PATTERN DETECTION
	allPatterns := pcs.patternDetector.DetectPatterns(pcs.symbol, pcs.interval, klines)
	reversalPatterns := pcs.patternDetector.DetectReversalPatterns(pcs.symbol, pcs.interval, klines)
	allPatterns = append(allPatterns, reversalPatterns...)

	var bestPattern *patterns.DetectedPattern
	for i := range allPatterns {
		if bestPattern == nil || allPatterns[i].Confidence > bestPattern.Confidence {
			bestPattern = &allPatterns[i]
		}
	}

	if bestPattern != nil {
		log.Printf("[%s] Pattern detected: %s (Confidence: %.2f, Direction: %s)",
			pcs.symbol, bestPattern.Type, bestPattern.Confidence, bestPattern.Direction)
	}

	// STEP 3: FVG ANALYSIS
	fvgs := pcs.fvgDetector.DetectFVGs(pcs.symbol, pcs.interval, klines)
	unfilledFVGs := pcs.fvgDetector.GetUnfilledFVGs(fvgs)

	var nearestFVG *analysis.FVG
	minDistance := 100.0
	for i := range unfilledFVGs {
		if pcs.fvgDetector.IsPriceNearFVG(currentPrice, unfilledFVGs[i], 5.0) {
			distance := pcs.calculateFVGDistance(currentPrice, unfilledFVGs[i])
			if distance < minDistance {
				minDistance = distance
				nearestFVG = &unfilledFVGs[i]
			}
		}
	}

	fvgPresent := nearestFVG != nil
	if fvgPresent {
		log.Printf("[%s] FVG detected: %s at %.2f-%.2f (Distance: %.2f%%)",
			pcs.symbol, nearestFVG.Type, nearestFVG.BottomPrice, nearestFVG.TopPrice, minDistance)
	}

	// STEP 4: VOLUME ANALYSIS
	volumeProfile := pcs.volumeAnalyzer.AnalyzeVolume(klines)
	if volumeProfile != nil {
		log.Printf("[%s] Volume: %.2fx average (%s, %s)",
			pcs.symbol, volumeProfile.VolumeRatio, volumeProfile.VolumeType,
			map[bool]string{true: "HIGH", false: "NORMAL"}[volumeProfile.IsHighVolume])
	}

	// STEP 5: INDICATOR ANALYSIS AND CONFLUENCE SCORING
	indicatorScore := pcs.calculateIndicatorScore(klines, currentPrice)
	log.Printf("[%s] Indicator Score: %.2f (RSI/MACD/Stochastic composite)", pcs.symbol, indicatorScore)

	confluenceResult := pcs.confluenceScorer.CalculateConfluence(
		trendScore,
		bestPattern,
		volumeProfile,
		fvgPresent,
		minDistance,
		indicatorScore,
	)

	log.Printf("[%s] Confluence Score: %.2f%% (Grade: %s, Confidence: %s)",
		pcs.symbol, confluenceResult.TotalScore*100, confluenceResult.Grade, confluenceResult.Confidence)

	// STEP 6: SIGNAL GENERATION
	if !pcs.confluenceScorer.ShouldTrade(confluenceResult) {
		log.Printf("[%s] Confluence score too low (%.2f < %.2f) - NO SIGNAL",
			pcs.symbol, confluenceResult.TotalScore, pcs.minConfluenceScore)
		return nil, nil
	}

	// Only trade if pattern and trend align
	if bestPattern == nil {
		log.Printf("[%s] No pattern detected - NO SIGNAL", pcs.symbol)
		return nil, nil
	}

	// Bullish signal conditions
	if bestPattern.Direction == "bullish" && structure.Trend != analysis.TrendBearish {
		signal := &Signal{
			Symbol:     pcs.symbol,
			Type:       SignalBuy,
			EntryPrice: currentPrice,
			StopLoss:   currentPrice * (1 - pcs.stopLossPercent),
			TakeProfit: currentPrice * (1 + pcs.takeProfitPercent),
			Reason:     pcs.buildSignalReason(bestPattern, confluenceResult, fvgPresent, volumeProfile),
		}

		log.Printf("[%s] 🟢 BUY SIGNAL GENERATED! Confluence: %.2f%%, Pattern: %s, Reasoning: %s",
			pcs.symbol, confluenceResult.TotalScore*100, bestPattern.Type, signal.Reason)

		return signal, nil
	}

	// Bearish signal conditions (for closing longs or SELL signals)
	if bestPattern.Direction == "bearish" && structure.Trend != analysis.TrendBullish {
		signal := &Signal{
			Symbol:     pcs.symbol,
			Type:       SignalSell,
			Side:       "SELL",
			EntryPrice: currentPrice,
			StopLoss:   currentPrice * (1 + pcs.stopLossPercent),  // Stop loss above for shorts
			TakeProfit: currentPrice * (1 - pcs.takeProfitPercent), // Take profit below for shorts
			Reason:     pcs.buildSignalReason(bestPattern, confluenceResult, fvgPresent, volumeProfile),
		}

		log.Printf("[%s] 🔴 SELL SIGNAL GENERATED! Confluence: %.2f%%, Pattern: %s, Reasoning: %s",
			pcs.symbol, confluenceResult.TotalScore*100, bestPattern.Type, signal.Reason)

		return signal, nil
	}

	return nil, nil
}

// calculateFVGDistance calculates distance from price to FVG center as percentage
func (pcs *PatternConfluenceStrategy) calculateFVGDistance(price float64, fvg analysis.FVG) float64 {
	fvgCenter := (fvg.TopPrice + fvg.BottomPrice) / 2
	distance := abs(price - fvgCenter)
	return (distance / price) * 100
}

// buildSignalReason constructs a human-readable signal explanation
func (pcs *PatternConfluenceStrategy) buildSignalReason(
	pattern *patterns.DetectedPattern,
	confluenceResult *confluence.SignalConfluence,
	fvgPresent bool,
	volumeProfile *analysis.VolumeProfile,
) string {
	reason := fmt.Sprintf("Pattern: %s (%.0f%% confidence)", pattern.Type, pattern.Confidence*100)

	if fvgPresent {
		reason += " + FVG zone"
	}

	if volumeProfile != nil && volumeProfile.IsHighVolume {
		reason += fmt.Sprintf(" + High volume (%.1fx)", volumeProfile.VolumeRatio)
	}

	reason += fmt.Sprintf(" | Confluence: %s (%.0f%%)", confluenceResult.Grade, confluenceResult.TotalScore*100)

	return reason
}

// calculateIndicatorScore computes a composite score from multiple technical indicators
func (pcs *PatternConfluenceStrategy) calculateIndicatorScore(klines []binance.Kline, currentPrice float64) float64 {
	if len(klines) < 26 {
		return 0.5 // Neutral if insufficient data
	}

	var totalScore float64
	var weights float64

	// 1. RSI Analysis (weight: 0.3)
	rsi := CalculateRSI(klines, 14)
	var rsiScore float64
	if rsi < 30 {
		rsiScore = 0.8 + (30-rsi)/30*0.2 // Oversold = bullish (0.8-1.0)
	} else if rsi > 70 {
		rsiScore = 0.2 - (rsi-70)/30*0.2 // Overbought = bearish (0.0-0.2)
	} else {
		rsiScore = 0.5 // Neutral zone
	}
	totalScore += rsiScore * 0.3
	weights += 0.3

	// 2. MACD Analysis (weight: 0.3)
	macd := CalculateMACD(klines, 12, 26, 9)
	var macdScore float64
	if macd.Histogram > 0 {
		macdScore = 0.5 + min(macd.Histogram/currentPrice*1000, 0.5) // Bullish momentum
	} else {
		macdScore = 0.5 - min(-macd.Histogram/currentPrice*1000, 0.5) // Bearish momentum
	}
	totalScore += macdScore * 0.3
	weights += 0.3

	// 3. Stochastic Analysis (weight: 0.2)
	stoch := CalculateStochastic(klines, 14, 3)
	var stochScore float64
	if stoch.K < 20 && stoch.D < 20 {
		stochScore = 0.9 // Oversold = strong bullish
	} else if stoch.K > 80 && stoch.D > 80 {
		stochScore = 0.1 // Overbought = strong bearish
	} else if stoch.K > stoch.D {
		stochScore = 0.6 // Bullish crossover
	} else {
		stochScore = 0.4 // Bearish crossover
	}
	totalScore += stochScore * 0.2
	weights += 0.2

	// 4. EMA Trend Alignment (weight: 0.2)
	ema9 := CalculateEMA(klines, 9)
	ema21 := CalculateEMA(klines, 21)
	var emaScore float64
	if currentPrice > ema9 && ema9 > ema21 {
		emaScore = 0.9 // Strong bullish alignment
	} else if currentPrice < ema9 && ema9 < ema21 {
		emaScore = 0.1 // Strong bearish alignment
	} else if currentPrice > ema21 {
		emaScore = 0.6 // Above longer MA
	} else {
		emaScore = 0.4 // Below longer MA
	}
	totalScore += emaScore * 0.2
	weights += 0.2

	// Normalize score to 0-1 range
	if weights > 0 {
		return totalScore / weights
	}
	return 0.5
}

// min returns the smaller of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Helper function
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
