package scanner

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/strategy"
	"fmt"
	"math"
	"time"
)

// ProximityEvaluator calculates proximity metrics for strategies
type ProximityEvaluator struct {
	cache *ScannerCache
}

// NewProximityEvaluator creates a new proximity evaluator
func NewProximityEvaluator(cacheTTL time.Duration) *ProximityEvaluator {
	return &ProximityEvaluator{
		cache: NewScannerCache(cacheTTL),
	}
}

// EvaluateProximity calculates how close a symbol is to triggering
func (pe *ProximityEvaluator) EvaluateProximity(
	strat strategy.Strategy,
	klines []binance.Kline,
	currentPrice float64,
) (*ProximityResult, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s_%s", strat.GetSymbol(), strat.Name())
	if cached := pe.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	result := &ProximityResult{
		Symbol:        strat.GetSymbol(),
		StrategyName:  strat.Name(),
		CurrentPrice:  currentPrice,
		LastEvaluated: time.Now(),
		Timestamp:     time.Now(),
	}

	// Strategy-specific proximity calculation
	switch s := strat.(type) {
	case *strategy.BreakoutStrategy:
		pe.evaluateBreakoutProximity(s, klines, currentPrice, result)
	case *strategy.SupportStrategy:
		pe.evaluateSupportProximity(s, klines, currentPrice, result)
	default:
		// Generic proximity for unknown strategies
		pe.evaluateGenericProximity(strat, klines, currentPrice, result)
	}

	// Calculate time prediction based on price velocity
	result.TimePrediction = pe.calculateTimePrediction(klines, currentPrice, result)

	// Cache the result
	pe.cache.Set(cacheKey, result)

	return result, nil
}

// evaluateBreakoutProximity calculates proximity for breakout strategy
func (pe *ProximityEvaluator) evaluateBreakoutProximity(
	s *strategy.BreakoutStrategy,
	klines []binance.Kline,
	currentPrice float64,
	result *ProximityResult,
) {
	if len(klines) < 2 {
		result.ReadinessScore = 0
		result.TrendDirection = "NEUTRAL"
		return
	}

	lastCandle := klines[len(klines)-2]
	targetPrice := lastCandle.High

	result.TargetPrice = targetPrice
	result.DistanceAbsolute = targetPrice - currentPrice
	result.DistancePercent = (result.DistanceAbsolute / currentPrice) * 100

	// Readiness score: 100% when at target, decreases with distance
	maxDistance := 5.0 // 5% away = 0% readiness
	if result.DistancePercent <= 0 {
		result.ReadinessScore = 100
	} else {
		result.ReadinessScore = math.Max(0, 100-(result.DistancePercent/maxDistance*100))
	}

	// Determine trend
	if currentPrice > lastCandle.Close {
		result.TrendDirection = "BULLISH"
	} else {
		result.TrendDirection = "BEARISH"
	}

	// Conditions checklist
	volumeConditionMet := true // Simplified - would check against MinVolume config
	priceNearHigh := result.DistancePercent < 1.0

	result.Conditions = ConditionsChecklist{
		TotalConditions: 2,
		Details: []ConditionDetail{
			{
				Name:        "Price Near High",
				Description: "Current price approaching previous candle high",
				Met:         priceNearHigh,
				Value:       currentPrice,
				Target:      targetPrice,
				Distance:    result.DistancePercent,
			},
			{
				Name:        "Volume Adequate",
				Description: "Volume meets minimum requirement",
				Met:         volumeConditionMet,
				Value:       lastCandle.Volume,
			},
		},
	}

	// Count met conditions
	for _, c := range result.Conditions.Details {
		if c.Met {
			result.Conditions.MetConditions++
		} else {
			result.Conditions.FailedConditions++
		}
	}
}

// evaluateSupportProximity calculates proximity for support strategy
func (pe *ProximityEvaluator) evaluateSupportProximity(
	s *strategy.SupportStrategy,
	klines []binance.Kline,
	currentPrice float64,
	result *ProximityResult,
) {
	if len(klines) < 2 {
		result.ReadinessScore = 0
		result.TrendDirection = "NEUTRAL"
		return
	}

	lastCandle := klines[len(klines)-2]
	touchDistance := 0.005 // Default 0.5% - would come from config
	touchThreshold := lastCandle.Low * (1 + touchDistance)

	result.TargetPrice = lastCandle.Low
	result.DistanceAbsolute = currentPrice - result.TargetPrice
	result.DistancePercent = (result.DistanceAbsolute / currentPrice) * 100

	// Readiness score based on proximity to support zone
	inZone := currentPrice >= lastCandle.Low && currentPrice <= touchThreshold
	if inZone {
		result.ReadinessScore = 100
	} else {
		maxDistance := 3.0 // 3% away = 0% readiness
		result.ReadinessScore = math.Max(0, 100-(math.Abs(result.DistancePercent)/maxDistance*100))
	}

	result.TrendDirection = "BEARISH" // Moving toward support

	result.Conditions = ConditionsChecklist{
		TotalConditions: 2,
		Details: []ConditionDetail{
			{
				Name:        "Near Support",
				Description: "Price within touch distance of support",
				Met:         inZone,
				Value:       currentPrice,
				Target:      lastCandle.Low,
				Distance:    result.DistancePercent,
			},
			{
				Name:        "Above Support",
				Description: "Price has not broken below support",
				Met:         currentPrice >= lastCandle.Low,
				Value:       currentPrice,
				Target:      lastCandle.Low,
			},
		},
	}

	for _, c := range result.Conditions.Details {
		if c.Met {
			result.Conditions.MetConditions++
		} else {
			result.Conditions.FailedConditions++
		}
	}
}

// evaluateGenericProximity provides basic proximity calculation for unknown strategies
func (pe *ProximityEvaluator) evaluateGenericProximity(
	strat strategy.Strategy,
	klines []binance.Kline,
	currentPrice float64,
	result *ProximityResult,
) {
	// Evaluate the strategy normally
	signal, _ := strat.Evaluate(klines, currentPrice)

	if signal != nil && signal.Type != strategy.SignalNone {
		// Signal is triggered
		result.ReadinessScore = 100
		result.TargetPrice = currentPrice
		result.DistancePercent = 0
		result.DistanceAbsolute = 0
		result.TrendDirection = "BULLISH"
	} else {
		// Signal not triggered - minimal readiness
		result.ReadinessScore = 0
		result.TargetPrice = currentPrice
		result.DistancePercent = 0
		result.DistanceAbsolute = 0
		result.TrendDirection = "NEUTRAL"
	}

	result.Conditions = ConditionsChecklist{
		TotalConditions: 1,
		Details: []ConditionDetail{
			{
				Name:        "Signal Triggered",
				Description: "Strategy conditions met",
				Met:         signal != nil && signal.Type != strategy.SignalNone,
			},
		},
	}

	if result.Conditions.Details[0].Met {
		result.Conditions.MetConditions = 1
	} else {
		result.Conditions.FailedConditions = 1
	}
}

// calculateTimePrediction estimates when signal might trigger based on price velocity
func (pe *ProximityEvaluator) calculateTimePrediction(
	klines []binance.Kline,
	currentPrice float64,
	result *ProximityResult,
) *TimePrediction {
	if result.DistancePercent <= 0 || len(klines) < 10 {
		return nil
	}

	// Calculate price velocity (average % change per period)
	velocities := []float64{}
	for i := len(klines) - 10; i < len(klines)-1; i++ {
		change := (klines[i+1].Close - klines[i].Close) / klines[i].Close * 100
		velocities = append(velocities, change)
	}

	avgVelocity := average(velocities)
	if avgVelocity <= 0 {
		return nil // Price moving away from target
	}

	// Estimate time: distance / velocity
	periodsNeeded := result.DistancePercent / avgVelocity

	// Assume 5-minute candles (adjust based on interval)
	minutesNeeded := int(periodsNeeded * 5)

	if minutesNeeded <= 0 || minutesNeeded > 1440 { // Cap at 24 hours
		return nil
	}

	return &TimePrediction{
		MinMinutes: minutesNeeded,
		MaxMinutes: minutesNeeded * 2, // Add uncertainty
		Confidence: 0.6,                 // Medium confidence
		BasedOn:    "price_velocity",
	}
}

// Helper function to calculate average
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
