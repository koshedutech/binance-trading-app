// Package autopilot provides automated trading functionality.
// Epic 10, Story 10.1: Trend-Based Exit for New Engine Mode (Epic 11 Integration)
package autopilot

import (
	"context"
	"fmt"
	"log"
	"time"
)

// NewEngineDecisionSettings holds configuration for New Engine Mode (Epic 11) trend reversal detection.
// This mode integrates with the Position Decision Engine's strategy system.
type NewEngineDecisionSettings struct {
	// Strategy Integration
	UseActiveStrategy  bool `json:"use_active_strategy"`   // Use the active strategy from Epic 11 Decision Engine
	FallbackToClassic  bool `json:"fallback_to_classic"`   // Fall back to Classic mode if engine unavailable

	// Exit Conditions
	ExitOnRegimeChange   bool `json:"exit_on_regime_change"`   // Exit if market regime changes from entry
	UseStrategyExitRules bool `json:"use_strategy_exit_rules"` // Use the strategy's built-in exit conditions

	// Trend Score Thresholds
	TrendScoreExitThreshold float64 `json:"trend_score_exit_threshold"` // Exit if trend score crosses this threshold (default: 40)
	BearishScoreThreshold   float64 `json:"bearish_score_threshold"`    // Score below this is bearish (default: 40)
	BullishScoreThreshold   float64 `json:"bullish_score_threshold"`    // Score above this is bullish (default: 60)

	// Indicator Weights for Trend Segment (user-configurable)
	TrendIndicatorWeights map[string]float64 `json:"trend_indicator_weights"` // Custom weights for trend indicators

	// Timing
	MinHoldMinutes int `json:"min_hold_minutes"` // Minimum minutes before reversal exit (default: 5)
}

// DefaultNewEngineDecisionSettings returns sensible defaults for New Engine Mode.
func DefaultNewEngineDecisionSettings() *NewEngineDecisionSettings {
	return &NewEngineDecisionSettings{
		UseActiveStrategy:       true,
		FallbackToClassic:       true,
		ExitOnRegimeChange:      true,
		UseStrategyExitRules:    true,
		TrendScoreExitThreshold: 40.0,
		BearishScoreThreshold:   40.0,
		BullishScoreThreshold:   60.0,
		TrendIndicatorWeights:   nil, // Use default weights from strategy
		MinHoldMinutes:          5,
	}
}

// EngineExitResult holds the result of New Engine trend reversal detection.
type EngineExitResult struct {
	ShouldExit bool   `json:"should_exit"`
	Reason     string `json:"reason"`

	// Strategy Information
	ActiveStrategy string                 `json:"active_strategy,omitempty"`
	StrategyScore  *DecisionStrategyScore `json:"strategy_score,omitempty"`

	// Regime Analysis
	EntryRegime   DecisionMarketRegime `json:"entry_regime,omitempty"`
	CurrentRegime DecisionMarketRegime `json:"current_regime,omitempty"`
	RegimeChanged bool                 `json:"regime_changed"`

	// Trend Analysis
	TrendScore     float64 `json:"trend_score"`
	TrendDirection string  `json:"trend_direction"` // "BULLISH", "BEARISH", "NEUTRAL"
	ScoreFlipped   bool    `json:"score_flipped"`   // Trend score flipped against position

	// Exit Conditions Met
	ExitConditions []EngineExitCondition `json:"exit_conditions"`

	// Metadata
	FallbackUsed bool      `json:"fallback_used"`
	AnalyzedAt   time.Time `json:"analyzed_at"`
}

// EngineExitCondition represents a single exit condition evaluation.
type EngineExitCondition struct {
	Name        string `json:"name"`
	Met         bool   `json:"met"`
	Value       string `json:"value,omitempty"`
	Threshold   string `json:"threshold,omitempty"`
	Description string `json:"description"`
}

// PositionContextData holds context about a position for engine-based exit decisions.
// This is passed to the Decision Engine for evaluation.
type PositionContextData struct {
	Symbol       string
	Side         string // "LONG" or "SHORT"
	EntryPrice   float64
	CurrentPrice float64
	EntryTime    time.Time
	EntryRegime  DecisionMarketRegime
	Mode         GinieTradingMode
	UserID       string
}

// NewEngineExitEvaluator provides methods for evaluating exits using Epic 11 Decision Engine.
type NewEngineExitEvaluator struct {
	stateManager     DecisionStateManagerInterface
	strategySelector StrategySelectorInterface
	registry         StrategyRegistryInterface
	settings         *NewEngineDecisionSettings
}

// NewEngineExitEvaluatorFromInterfaces creates an evaluator from interface types.
// This avoids importing the decision package directly.
func NewEngineExitEvaluatorFromInterfaces(
	stateManager DecisionStateManagerInterface,
	selector StrategySelectorInterface,
	registry StrategyRegistryInterface,
	settings *NewEngineDecisionSettings,
) *NewEngineExitEvaluator {
	if settings == nil {
		settings = DefaultNewEngineDecisionSettings()
	}
	return &NewEngineExitEvaluator{
		stateManager:     stateManager,
		strategySelector: selector,
		registry:         registry,
		settings:         settings,
	}
}

// IsAvailable checks if the Engine evaluator has all required components.
func (e *NewEngineExitEvaluator) IsAvailable() bool {
	return e != nil && e.stateManager != nil && e.registry != nil
}

// detectTrendReversalNewEngine detects trend reversal signals using Epic 11 Decision Engine.
// Returns result indicating if position should exit due to trend reversal.
func detectTrendReversalNewEngine(
	ctx context.Context,
	pos *GiniePosition,
	posCtx *PositionContextData,
	evaluator *NewEngineExitEvaluator,
	settings *NewEngineDecisionSettings,
) *EngineExitResult {

	result := &EngineExitResult{
		ShouldExit:     false,
		ExitConditions: make([]EngineExitCondition, 0),
		AnalyzedAt:     time.Now(),
	}

	// Validate inputs
	if pos == nil || posCtx == nil {
		result.Reason = "Invalid position or context"
		return result
	}

	if settings == nil {
		settings = DefaultNewEngineDecisionSettings()
	}

	// Check minimum hold time
	holdDuration := time.Since(pos.EntryTime)
	if holdDuration.Minutes() < float64(settings.MinHoldMinutes) {
		result.Reason = "Position too young for trend reversal exit"
		return result
	}

	// Check if evaluator is available
	if evaluator == nil || !evaluator.IsAvailable() {
		if settings.FallbackToClassic {
			result.FallbackUsed = true
			result.Reason = "Decision Engine unavailable, fallback to Classic mode required"
			// The caller should use Classic mode instead
			return result
		}
		result.Reason = "Decision Engine unavailable and fallback disabled"
		return result
	}

	// Get coin state from Decision Engine
	coinState, err := evaluator.stateManager.GetCoinStateInterface(ctx, posCtx.UserID, posCtx.Symbol)
	if err != nil || coinState == nil {
		if settings.FallbackToClassic {
			result.FallbackUsed = true
			result.Reason = "Coin state not available, fallback to Classic mode required"
			return result
		}
		result.Reason = fmt.Sprintf("Failed to get coin state: %v", err)
		return result
	}

	result.CurrentRegime = coinState.GetRegime()
	result.EntryRegime = posCtx.EntryRegime
	result.ActiveStrategy = coinState.GetActiveStrategy()

	// === CONDITION 1: Regime Change ===
	if settings.ExitOnRegimeChange && posCtx.EntryRegime != "" {
		regimeChanged := posCtx.EntryRegime != coinState.GetRegime()
		result.RegimeChanged = regimeChanged

		condition := EngineExitCondition{
			Name:        "REGIME_CHANGE",
			Met:         regimeChanged,
			Value:       string(coinState.GetRegime()),
			Threshold:   string(posCtx.EntryRegime),
			Description: "Market regime changed from entry",
		}
		result.ExitConditions = append(result.ExitConditions, condition)
	}

	// === CONDITION 2: Strategy Exit Conditions ===
	if settings.UseStrategyExitRules && evaluator.registry != nil && coinState.GetActiveStrategy() != "" {
		strategy, err := evaluator.registry.GetStrategyReader(coinState.GetActiveStrategy())
		if err == nil && strategy != nil {
			exitConditions := strategy.GetExitConditions()
			for _, cond := range exitConditions {
				// Get indicator value from coin state
				value := getIndicatorValueFromCoinStateReader(coinState, cond.Indicator)
				met := cond.Evaluate(value)

				condition := EngineExitCondition{
					Name:        cond.Name,
					Met:         met,
					Value:       fmt.Sprintf("%.2f", value),
					Threshold:   fmt.Sprintf("%.2f", cond.Threshold),
					Description: cond.Description,
				}
				result.ExitConditions = append(result.ExitConditions, condition)
			}
		}
	}

	// === CONDITION 3: Trend Score Analysis ===
	// Calculate averaged trend score from indicators
	trendScore := calculateTrendScoreFromCoinStateReader(coinState, settings.TrendIndicatorWeights)
	result.TrendScore = trendScore

	// Determine trend direction based on score
	if trendScore < settings.BearishScoreThreshold {
		result.TrendDirection = "BEARISH"
	} else if trendScore > settings.BullishScoreThreshold {
		result.TrendDirection = "BULLISH"
	} else {
		result.TrendDirection = "NEUTRAL"
	}

	// Check if trend score flipped against position
	if pos.Side == "LONG" && result.TrendDirection == "BEARISH" {
		result.ScoreFlipped = true
		condition := EngineExitCondition{
			Name:        "TREND_SCORE_BEARISH",
			Met:         true,
			Value:       fmt.Sprintf("%.1f", trendScore),
			Threshold:   fmt.Sprintf("< %.1f", settings.BearishScoreThreshold),
			Description: "Trend score turned bearish for LONG position",
		}
		result.ExitConditions = append(result.ExitConditions, condition)
	} else if pos.Side == "SHORT" && result.TrendDirection == "BULLISH" {
		result.ScoreFlipped = true
		condition := EngineExitCondition{
			Name:        "TREND_SCORE_BULLISH",
			Met:         true,
			Value:       fmt.Sprintf("%.1f", trendScore),
			Threshold:   fmt.Sprintf("> %.1f", settings.BullishScoreThreshold),
			Description: "Trend score turned bullish for SHORT position",
		}
		result.ExitConditions = append(result.ExitConditions, condition)
	}

	// === DETERMINE EXIT DECISION ===
	// Exit if any of these conditions are met:
	// 1. Regime change (if enabled)
	// 2. Trend score flipped against position
	// 3. Multiple strategy exit conditions met

	exitReasons := []string{}

	if result.RegimeChanged && settings.ExitOnRegimeChange {
		exitReasons = append(exitReasons, fmt.Sprintf("Regime changed: %s -> %s", posCtx.EntryRegime, coinState.GetRegime()))
	}

	if result.ScoreFlipped {
		exitReasons = append(exitReasons, fmt.Sprintf("Trend flipped to %s (score: %.1f)", result.TrendDirection, trendScore))
	}

	// Count met exit conditions from strategy
	metConditions := 0
	for _, cond := range result.ExitConditions {
		if cond.Met && cond.Name != "REGIME_CHANGE" && cond.Name != "TREND_SCORE_BEARISH" && cond.Name != "TREND_SCORE_BULLISH" {
			metConditions++
		}
	}
	if metConditions >= 2 {
		exitReasons = append(exitReasons, fmt.Sprintf("%d strategy exit conditions met", metConditions))
	}

	if len(exitReasons) > 0 {
		result.ShouldExit = true
		result.Reason = formatEngineExitReasons(exitReasons)
		log.Printf("[ENGINE-EXIT] %s %s: TREND REVERSAL DETECTED! %s",
			pos.Symbol, pos.Side, result.Reason)
	} else {
		result.Reason = "No exit conditions triggered"
	}

	return result
}

// getIndicatorValueFromCoinStateReader extracts an indicator value from the CoinStateReader.
func getIndicatorValueFromCoinStateReader(state CoinStateReader, indicatorName string) float64 {
	if state == nil {
		return 0
	}

	switch indicatorName {
	case "adx":
		return state.GetADX()
	case "rsi":
		return state.GetRSI()
	case "ema_9":
		return state.GetEMA9()
	case "ema_21":
		return state.GetEMA21()
	case "atr":
		return state.GetATR()
	case "score_technical":
		return float64(state.GetScoreTechnical())
	case "score_final":
		return float64(state.GetScoreFinal())
	default:
		return 0
	}
}

// calculateTrendScoreFromCoinStateReader calculates an aggregated trend score.
// Uses the coin state's indicators with optional custom weights.
func calculateTrendScoreFromCoinStateReader(state CoinStateReader, weights map[string]float64) float64 {
	if state == nil {
		return 50 // Neutral default
	}

	// Default weights for trend indicators
	defaultWeights := map[string]float64{
		"adx":       0.25, // Trend strength
		"rsi":       0.20, // Momentum
		"ema_cross": 0.25, // Trend direction
		"trend_1h":  0.15, // Higher timeframe trend
		"trend_15m": 0.15, // Lower timeframe trend
	}

	// Use custom weights if provided
	if weights == nil {
		weights = defaultWeights
	}

	score := 0.0
	totalWeight := 0.0

	// ADX contribution (0-100 score based on trend strength)
	if w, ok := weights["adx"]; ok && w > 0 {
		adxScore := normalizeADXToScore(state.GetADX())
		score += adxScore * w
		totalWeight += w
	}

	// RSI contribution (normalized to 0-100)
	if w, ok := weights["rsi"]; ok && w > 0 {
		rsiScore := state.GetRSI() // Already 0-100
		score += rsiScore * w
		totalWeight += w
	}

	// EMA Cross contribution
	if w, ok := weights["ema_cross"]; ok && w > 0 {
		var emaScore float64
		if state.GetEMA9() > state.GetEMA21() {
			emaScore = 70 // Bullish
		} else if state.GetEMA9() < state.GetEMA21() {
			emaScore = 30 // Bearish
		} else {
			emaScore = 50 // Neutral
		}
		score += emaScore * w
		totalWeight += w
	}

	// 1H Trend contribution
	if w, ok := weights["trend_1h"]; ok && w > 0 {
		trendScore := trendDirectionToScoreValue(state.GetTrend1H())
		score += trendScore * w
		totalWeight += w
	}

	// 15M Trend contribution
	if w, ok := weights["trend_15m"]; ok && w > 0 {
		trendScore := trendDirectionToScoreValue(state.GetTrend15M())
		score += trendScore * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 50
	}

	return score / totalWeight
}

// normalizeADXToScore converts ADX value to a 0-100 score.
// ADX < 20 = weak trend (30), 20-40 = moderate (50), 40-60 = strong (70), > 60 = very strong (90)
func normalizeADXToScore(adx float64) float64 {
	if adx < 20 {
		return 30 + (adx/20)*20 // 30-50
	} else if adx < 40 {
		return 50 + ((adx-20)/20)*20 // 50-70
	} else if adx < 60 {
		return 70 + ((adx-40)/20)*20 // 70-90
	}
	return 90 // Very strong trend
}

// trendDirectionToScoreValue converts trend direction to a score.
func trendDirectionToScoreValue(trend DecisionTrendDirection) float64 {
	switch trend {
	case DecisionTrendBullish:
		return 75
	case DecisionTrendBearish:
		return 25
	case DecisionTrendNeutral:
		return 50
	default:
		return 50
	}
}

// formatEngineExitReasons formats multiple exit reasons into a single string.
func formatEngineExitReasons(reasons []string) string {
	if len(reasons) == 0 {
		return "No reasons"
	}
	if len(reasons) == 1 {
		return reasons[0]
	}
	return fmt.Sprintf("%s (and %d more)", reasons[0], len(reasons)-1)
}

// GetTrendIndicatorConfig returns the default indicator configuration for trend segment.
// This can be customized per user through settings.
func GetTrendIndicatorConfig() map[string]IndicatorSegment {
	return map[string]IndicatorSegment{
		"adx":       IndicatorSegmentTrend,
		"ema_cross": IndicatorSegmentTrend,
		"rsi":       IndicatorSegmentMomentum,
		"macd":      IndicatorSegmentMomentum,
	}
}
