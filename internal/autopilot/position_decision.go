// Package autopilot provides automated trading functionality.
// Epic 10, Story 10.1: Unified Position Decision Routing
// This file provides the unified interface for position exit decisions,
// routing between Classic Mode and New Engine Mode (Epic 11 integration).
package autopilot

import (
	"context"
	"fmt"
	"log"
	"time"
)

// DecisionMode specifies which exit decision algorithm to use.
type DecisionMode string

const (
	// DecisionModeClassic uses the classic indicator-based exit detection.
	DecisionModeClassic DecisionMode = "classic"
	// DecisionModeNewEngine uses Epic 11 Position Decision Engine integration.
	DecisionModeNewEngine DecisionMode = "new_engine"
	// DecisionModeAuto automatically selects based on availability.
	DecisionModeAuto DecisionMode = "auto"
)

// ExitPriority defines the priority levels for exit decisions.
type ExitPriority int

const (
	// ExitPriorityTrendReversal is highest priority - immediate exit on trend reversal
	ExitPriorityTrendReversal ExitPriority = 1
	// ExitPriorityEfficiency is second priority - exit when efficiency drops
	ExitPriorityEfficiency ExitPriority = 2
	// ExitPriorityTrailingSL is third - Binance handles trailing SL
	ExitPriorityTrailingSL ExitPriority = 3
	// ExitPriorityDynamicTP is fourth - Binance handles dynamic TP
	ExitPriorityDynamicTP ExitPriority = 4
)

// PositionDecisionConfig holds configuration for position exit decisions.
type PositionDecisionConfig struct {
	// Mode Selection
	DecisionMode DecisionMode `json:"decision_mode"` // "classic" | "new_engine" | "auto"

	// Classic Mode Settings
	ClassicSettings *ClassicDecisionSettings `json:"classic,omitempty"`

	// New Engine Mode Settings
	NewEngineSettings *NewEngineDecisionSettings `json:"new_engine,omitempty"`

	// Exit Priority Settings
	EnableTrendReversalExit bool    `json:"enable_trend_reversal_exit"` // Enable trend-based exit
	EnableEfficiencyExit    bool    `json:"enable_efficiency_exit"`     // Enable efficiency-based exit
	EfficiencyThreshold     float64 `json:"efficiency_threshold"`       // Min efficiency to stay in position

	// Timeframe for indicator analysis
	AnalysisTimeframe string `json:"analysis_timeframe"` // e.g., "15m", "1h"
}

// DefaultPositionDecisionConfig returns default configuration.
func DefaultPositionDecisionConfig() *PositionDecisionConfig {
	return &PositionDecisionConfig{
		DecisionMode:            DecisionModeAuto,
		ClassicSettings:         DefaultClassicDecisionSettings(),
		NewEngineSettings:       DefaultNewEngineDecisionSettings(),
		EnableTrendReversalExit: true,
		EnableEfficiencyExit:    true,
		EfficiencyThreshold:     0.3, // 30% efficiency minimum
		AnalysisTimeframe:       "15m",
	}
}

// PositionExitDecision represents a complete exit decision with priority.
type PositionExitDecision struct {
	ShouldExit   bool         `json:"should_exit"`
	Priority     ExitPriority `json:"priority"`
	ExitReason   string       `json:"exit_reason"`
	ExitType     string       `json:"exit_type"` // "trend_reversal", "efficiency", "trailing_sl", "dynamic_tp"
	DecisionMode DecisionMode `json:"decision_mode"`

	// Detailed results from sub-systems
	ClassicResult *ClassicExitResult `json:"classic_result,omitempty"`
	EngineResult  *EngineExitResult  `json:"engine_result,omitempty"`

	// Metadata
	AnalyzedAt time.Time `json:"analyzed_at"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
}

// PositionDecisionEvaluator handles unified position exit evaluation.
type PositionDecisionEvaluator struct {
	analyzer        *GinieAnalyzer
	engineEvaluator *NewEngineExitEvaluator
	config          *PositionDecisionConfig
}

// NewPositionDecisionEvaluator creates a new unified decision evaluator.
// stateManager, selector, and registry are interface types from decision_interface.go
func NewPositionDecisionEvaluator(
	analyzer *GinieAnalyzer,
	stateManager DecisionStateManagerInterface,
	selector StrategySelectorInterface,
	registry StrategyRegistryInterface,
	config *PositionDecisionConfig,
) *PositionDecisionEvaluator {
	if config == nil {
		config = DefaultPositionDecisionConfig()
	}

	var engineEval *NewEngineExitEvaluator
	if stateManager != nil {
		engineEval = NewEngineExitEvaluatorFromInterfaces(
			stateManager,
			selector,
			registry,
			config.NewEngineSettings,
		)
	}

	return &PositionDecisionEvaluator{
		analyzer:        analyzer,
		engineEvaluator: engineEval,
		config:          config,
	}
}

// shouldExitOnTrendReversal evaluates whether a position should exit due to trend reversal.
// This is the main entry point that routes to the appropriate mode.
func shouldExitOnTrendReversal(
	ctx context.Context,
	pos *GiniePosition,
	analyzer *GinieAnalyzer,
	config *PositionDecisionConfig,
	engineEvaluator *NewEngineExitEvaluator,
	userID string,
) *PositionExitDecision {
	if pos == nil {
		return &PositionExitDecision{
			ShouldExit: false,
			ExitReason: "Invalid position",
			AnalyzedAt: time.Now(),
		}
	}

	if config == nil {
		config = DefaultPositionDecisionConfig()
	}

	decision := &PositionExitDecision{
		ShouldExit: false,
		Priority:   ExitPriorityTrendReversal,
		ExitType:   "trend_reversal",
		AnalyzedAt: time.Now(),
		Symbol:     pos.Symbol,
		Side:       pos.Side,
	}

	// Determine which mode to use
	mode := determineDecisionMode(config.DecisionMode, engineEvaluator)
	decision.DecisionMode = mode

	switch mode {
	case DecisionModeNewEngine:
		decision = evaluateWithNewEngine(ctx, pos, config, engineEvaluator, userID)

	case DecisionModeClassic:
		decision = evaluateWithClassic(pos, analyzer, config)

	default:
		// Auto mode: try New Engine first, fallback to Classic
		decision = evaluateWithNewEngine(ctx, pos, config, engineEvaluator, userID)
		if decision.EngineResult != nil && decision.EngineResult.FallbackUsed {
			decision = evaluateWithClassic(pos, analyzer, config)
		}
	}

	if decision.ShouldExit {
		log.Printf("[POSITION-DECISION] %s %s: EXIT RECOMMENDED (Priority %d, Mode: %s) - %s",
			pos.Symbol, pos.Side, decision.Priority, decision.DecisionMode, decision.ExitReason)
	}

	return decision
}

// determineDecisionMode selects the appropriate decision mode based on config and availability.
func determineDecisionMode(requested DecisionMode, engineEval *NewEngineExitEvaluator) DecisionMode {
	switch requested {
	case DecisionModeNewEngine:
		if engineEval != nil && engineEval.IsAvailable() {
			return DecisionModeNewEngine
		}
		log.Printf("[POSITION-DECISION] New Engine requested but unavailable, falling back to Classic")
		return DecisionModeClassic

	case DecisionModeClassic:
		return DecisionModeClassic

	case DecisionModeAuto:
		if engineEval != nil && engineEval.IsAvailable() {
			return DecisionModeNewEngine
		}
		return DecisionModeClassic

	default:
		return DecisionModeClassic
	}
}

// evaluateWithNewEngine evaluates exit using Epic 11 Decision Engine.
func evaluateWithNewEngine(
	ctx context.Context,
	pos *GiniePosition,
	config *PositionDecisionConfig,
	engineEvaluator *NewEngineExitEvaluator,
	userID string,
) *PositionExitDecision {
	decision := &PositionExitDecision{
		ShouldExit:   false,
		Priority:     ExitPriorityTrendReversal,
		ExitType:     "trend_reversal",
		DecisionMode: DecisionModeNewEngine,
		AnalyzedAt:   time.Now(),
		Symbol:       pos.Symbol,
		Side:         pos.Side,
	}

	// Create position context
	posCtx := &PositionContextData{
		Symbol:       pos.Symbol,
		Side:         pos.Side,
		EntryPrice:   pos.EntryPrice,
		CurrentPrice: 0, // Will be fetched by evaluator
		EntryTime:    pos.EntryTime,
		Mode:         pos.Mode,
		UserID:       userID,
	}

	// Detect trend reversal using New Engine
	result := detectTrendReversalNewEngine(ctx, pos, posCtx, engineEvaluator, config.NewEngineSettings)
	decision.EngineResult = result

	if result != nil {
		decision.ShouldExit = result.ShouldExit
		decision.ExitReason = result.Reason

		if result.FallbackUsed {
			// Mark that we need to use Classic mode
			decision.DecisionMode = DecisionModeAuto
		}
	}

	return decision
}

// evaluateWithClassic evaluates exit using Classic indicator mode.
func evaluateWithClassic(
	pos *GiniePosition,
	analyzer *GinieAnalyzer,
	config *PositionDecisionConfig,
) *PositionExitDecision {
	decision := &PositionExitDecision{
		ShouldExit:   false,
		Priority:     ExitPriorityTrendReversal,
		ExitType:     "trend_reversal",
		DecisionMode: DecisionModeClassic,
		AnalyzedAt:   time.Now(),
		Symbol:       pos.Symbol,
		Side:         pos.Side,
	}

	// Calculate indicators
	timeframe := config.AnalysisTimeframe
	if timeframe == "" {
		timeframe = "15m"
	}

	indicators, err := calculateClassicTrendIndicators(analyzer, pos.Symbol, timeframe)
	if err != nil {
		decision.ExitReason = "Failed to calculate indicators: " + err.Error()
		return decision
	}

	// Detect trend reversal
	result := detectTrendReversalClassic(pos, indicators, config.ClassicSettings)
	decision.ClassicResult = result

	if result != nil {
		decision.ShouldExit = result.ShouldExit
		decision.ExitReason = result.Reason
	}

	return decision
}

// EvaluateExitPriorities evaluates all exit conditions and returns the highest priority exit.
// Priority order: 1. Trend Reversal, 2. Efficiency, 3. Trailing SL, 4. Dynamic TP
func (e *PositionDecisionEvaluator) EvaluateExitPriorities(
	ctx context.Context,
	pos *GiniePosition,
	userID string,
	currentPrice float64,
) *PositionExitDecision {
	if pos == nil {
		return &PositionExitDecision{
			ShouldExit: false,
			ExitReason: "Invalid position",
		}
	}

	// Priority 1: Check Trend Reversal (highest priority - immediate exit)
	if e.config.EnableTrendReversalExit {
		trendDecision := shouldExitOnTrendReversal(ctx, pos, e.analyzer, e.config, e.engineEvaluator, userID)
		if trendDecision.ShouldExit {
			return trendDecision
		}
	}

	// Priority 2: Check Efficiency
	if e.config.EnableEfficiencyExit {
		efficiencyDecision := e.checkEfficiencyExit(pos, currentPrice)
		if efficiencyDecision.ShouldExit {
			return efficiencyDecision
		}
	}

	// Priority 3 & 4: Trailing SL and Dynamic TP are handled by Binance orders
	// These don't require our evaluation - Binance will trigger them automatically

	// No exit conditions met
	return &PositionExitDecision{
		ShouldExit: false,
		ExitReason: "No exit conditions triggered",
		AnalyzedAt: time.Now(),
		Symbol:     pos.Symbol,
		Side:       pos.Side,
	}
}

// checkEfficiencyExit checks if position efficiency has dropped below threshold.
func (e *PositionDecisionEvaluator) checkEfficiencyExit(pos *GiniePosition, currentPrice float64) *PositionExitDecision {
	decision := &PositionExitDecision{
		ShouldExit: false,
		Priority:   ExitPriorityEfficiency,
		ExitType:   "efficiency",
		AnalyzedAt: time.Now(),
		Symbol:     pos.Symbol,
		Side:       pos.Side,
	}

	if currentPrice <= 0 || pos.EntryPrice <= 0 {
		return decision
	}

	// Calculate current PnL percentage
	var pnlPercent float64
	if pos.Side == "LONG" {
		pnlPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
	} else {
		pnlPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
	}

	// Calculate position efficiency (how well position is performing vs expectation)
	// Efficiency = current PnL / (time held in hours) / expected hourly return
	holdHours := time.Since(pos.EntryTime).Hours()
	if holdHours < 0.1 {
		holdHours = 0.1 // Minimum 6 minutes
	}

	// Expected hourly return varies by mode
	expectedHourlyReturn := 0.1 // 0.1% per hour default
	switch pos.Mode {
	case GinieModeScalp, GinieModeUltraFast:
		expectedHourlyReturn = 0.5 // Scalps expect higher returns
	case GinieModeSwing:
		expectedHourlyReturn = 0.1
	case GinieModePosition:
		expectedHourlyReturn = 0.05
	}

	expectedPnL := holdHours * expectedHourlyReturn
	if expectedPnL > 0 {
		efficiency := pnlPercent / expectedPnL
		if efficiency < e.config.EfficiencyThreshold {
			decision.ShouldExit = true
			decision.ExitReason = fmt.Sprintf("Efficiency %.2f below threshold %.2f (PnL: %.2f%%, Expected: %.2f%%)",
				efficiency, e.config.EfficiencyThreshold, pnlPercent, expectedPnL)
		}
	}

	return decision
}

// ShouldExitOnTrendReversal is a convenience function for quick trend reversal checks.
// This is the main public API for position monitoring loops.
func ShouldExitOnTrendReversal(
	ctx context.Context,
	pos *GiniePosition,
	analyzer *GinieAnalyzer,
	config *PositionDecisionConfig,
	engineEvaluator *NewEngineExitEvaluator,
	userID string,
) bool {
	decision := shouldExitOnTrendReversal(ctx, pos, analyzer, config, engineEvaluator, userID)
	return decision.ShouldExit
}

// GetExitDecisionDetails returns detailed exit decision information.
// Useful for logging and debugging.
func GetExitDecisionDetails(
	ctx context.Context,
	pos *GiniePosition,
	analyzer *GinieAnalyzer,
	config *PositionDecisionConfig,
	engineEvaluator *NewEngineExitEvaluator,
	userID string,
) *PositionExitDecision {
	return shouldExitOnTrendReversal(ctx, pos, analyzer, config, engineEvaluator, userID)
}

// Helper to format log output
func init() {
	// Ensure log package is properly configured
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}
