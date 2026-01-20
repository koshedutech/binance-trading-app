// Package autopilot provides automated trading functionality.
// Epic 10, Story 10.1: Decision Engine Interface Definitions
//
// This file defines interfaces for the Decision Engine integration to avoid
// import cycles between autopilot and decision packages.
// The actual implementations are in the decision package.
package autopilot

import (
	"context"
	"time"
)

// --- Market Regime Types ---

// DecisionMarketRegime represents the current market condition.
// Mirrors decision.MarketRegime.
type DecisionMarketRegime string

const (
	DecisionRegimeTrending      DecisionMarketRegime = "TRENDING"
	DecisionRegimeRanging       DecisionMarketRegime = "RANGING"
	DecisionRegimeVolatile      DecisionMarketRegime = "VOLATILE"
	DecisionRegimeConsolidating DecisionMarketRegime = "CONSOLIDATING"
)

// DecisionTrendDirection represents the trend direction on a timeframe.
// Mirrors decision.TrendDirection.
type DecisionTrendDirection string

const (
	DecisionTrendBullish DecisionTrendDirection = "BULLISH"
	DecisionTrendBearish DecisionTrendDirection = "BEARISH"
	DecisionTrendNeutral DecisionTrendDirection = "NEUTRAL"
)

// --- Coin State Interface ---

// CoinStateReader provides read access to coin state data.
// This allows autopilot to read coin state without importing decision package.
type CoinStateReader interface {
	// GetSymbol returns the trading pair symbol
	GetSymbol() string

	// GetPrice returns the current price
	GetPrice() float64

	// GetRegime returns the current market regime
	GetRegime() DecisionMarketRegime

	// GetActiveStrategy returns the active strategy name
	GetActiveStrategy() string

	// Technical indicators
	GetADX() float64
	GetATR() float64
	GetRSI() float64
	GetEMA9() float64
	GetEMA21() float64

	// Trend analysis
	GetTrend1H() DecisionTrendDirection
	GetTrend15M() DecisionTrendDirection

	// Scores
	GetScoreTechnical() int
	GetScoreFinal() int
}

// DecisionStateManagerInterface provides methods for accessing coin state.
// This is the interface autopilot uses to access Decision Engine state for exit decisions.
// Named differently from StateManagerInterface to avoid conflict with the existing
// StateManagerInterface in ginie_autopilot.go which is used for writing state.
type DecisionStateManagerInterface interface {
	// GetCoinState retrieves the full coin state for a user and symbol
	GetCoinStateInterface(ctx context.Context, userID, symbol string) (CoinStateReader, error)
}

// --- Strategy Interfaces ---

// StrategyExitCondition represents an exit condition from a strategy.
type StrategyExitCondition struct {
	Name        string
	Indicator   string
	Operator    string // "lt", "gt", "eq"
	Threshold   float64
	Description string
}

// Evaluate checks if the condition is met for the given value.
func (c *StrategyExitCondition) Evaluate(value float64) bool {
	switch c.Operator {
	case "lt":
		return value < c.Threshold
	case "gt":
		return value > c.Threshold
	case "eq":
		return value == c.Threshold
	case "lte":
		return value <= c.Threshold
	case "gte":
		return value >= c.Threshold
	default:
		return false
	}
}

// StrategyReader provides read access to strategy information.
type StrategyReader interface {
	// GetName returns the strategy name
	GetName() string

	// GetExitConditions returns the exit conditions for this strategy
	GetExitConditions() []StrategyExitCondition
}

// StrategyRegistryInterface provides methods for accessing strategies.
type StrategyRegistryInterface interface {
	// GetStrategy retrieves a strategy by name
	GetStrategyReader(name string) (StrategyReader, error)

	// HasStrategy checks if a strategy exists
	HasStrategy(name string) bool
}

// StrategySelectorInterface provides methods for selecting strategies.
type StrategySelectorInterface interface {
	// GetRegistry returns the underlying registry
	GetRegistryInterface() StrategyRegistryInterface
}

// --- Strategy Score ---

// DecisionStrategyScore holds the evaluation score for a strategy.
type DecisionStrategyScore struct {
	StrategyName   string  `json:"strategy_name"`
	TotalScore     float64 `json:"total_score"`
	RegimeScore    float64 `json:"regime_score"`
	IndicatorScore float64 `json:"indicator_score"`
	Confidence     float64 `json:"confidence"`
}

// --- Indicator Segment Types ---

// IndicatorSegment represents the segment an indicator belongs to.
type IndicatorSegment string

const (
	IndicatorSegmentTrend      IndicatorSegment = "trend"
	IndicatorSegmentMomentum   IndicatorSegment = "momentum"
	IndicatorSegmentVolatility IndicatorSegment = "volatility"
	IndicatorSegmentVolume     IndicatorSegment = "volume"
)

// --- Adapter Functions ---
// These help convert between the interface types and the actual decision types
// when initializing the evaluator with real decision package components.

// DecisionEngineComponents holds the actual decision package components
// that will be wrapped by the interfaces.
type DecisionEngineComponents struct {
	// These will be set by the caller using the actual decision types
	// through type assertions at initialization time
	StateManager     interface{}
	StrategySelector interface{}
	Registry         interface{}
}

// EngineAvailabilityChecker checks if the decision engine components are available.
type EngineAvailabilityChecker interface {
	IsEngineAvailable() bool
}

// --- Adapter Structs for Runtime Bridging ---

// CoinStateAdapter wraps a decision.CoinState to implement CoinStateReader.
// This is populated at runtime by the caller.
type CoinStateAdapter struct {
	Symbol         string
	Price          float64
	Regime         DecisionMarketRegime
	ActiveStrategy string
	ADX            float64
	ATR            float64
	RSI            float64
	EMA9           float64
	EMA21          float64
	Trend1H        DecisionTrendDirection
	Trend15M       DecisionTrendDirection
	ScoreTechnical int
	ScoreFinal     int
}

func (a *CoinStateAdapter) GetSymbol() string                 { return a.Symbol }
func (a *CoinStateAdapter) GetPrice() float64                 { return a.Price }
func (a *CoinStateAdapter) GetRegime() DecisionMarketRegime   { return a.Regime }
func (a *CoinStateAdapter) GetActiveStrategy() string         { return a.ActiveStrategy }
func (a *CoinStateAdapter) GetADX() float64                   { return a.ADX }
func (a *CoinStateAdapter) GetATR() float64                   { return a.ATR }
func (a *CoinStateAdapter) GetRSI() float64                   { return a.RSI }
func (a *CoinStateAdapter) GetEMA9() float64                  { return a.EMA9 }
func (a *CoinStateAdapter) GetEMA21() float64                 { return a.EMA21 }
func (a *CoinStateAdapter) GetTrend1H() DecisionTrendDirection  { return a.Trend1H }
func (a *CoinStateAdapter) GetTrend15M() DecisionTrendDirection { return a.Trend15M }
func (a *CoinStateAdapter) GetScoreTechnical() int            { return a.ScoreTechnical }
func (a *CoinStateAdapter) GetScoreFinal() int                { return a.ScoreFinal }

// DecisionStateManagerAdapter wraps a decision.StateManager to implement DecisionStateManagerInterface.
// The actual conversion happens at the call site.
type DecisionStateManagerAdapter struct {
	// GetCoinStateFn is the function that fetches and converts coin state
	GetCoinStateFn func(ctx context.Context, userID, symbol string) (CoinStateReader, error)
}

func (a *DecisionStateManagerAdapter) GetCoinStateInterface(ctx context.Context, userID, symbol string) (CoinStateReader, error) {
	if a.GetCoinStateFn == nil {
		return nil, nil
	}
	return a.GetCoinStateFn(ctx, userID, symbol)
}

// StrategyAdapter wraps a decision.Strategy to implement StrategyReader.
type StrategyAdapter struct {
	Name           string
	ExitConditions []StrategyExitCondition
}

func (a *StrategyAdapter) GetName() string                       { return a.Name }
func (a *StrategyAdapter) GetExitConditions() []StrategyExitCondition { return a.ExitConditions }

// RegistryAdapter wraps a decision.StrategyRegistry to implement StrategyRegistryInterface.
type RegistryAdapter struct {
	GetStrategyFn func(name string) (StrategyReader, error)
	HasStrategyFn func(name string) bool
}

func (a *RegistryAdapter) GetStrategyReader(name string) (StrategyReader, error) {
	if a.GetStrategyFn == nil {
		return nil, nil
	}
	return a.GetStrategyFn(name)
}

func (a *RegistryAdapter) HasStrategy(name string) bool {
	if a.HasStrategyFn == nil {
		return false
	}
	return a.HasStrategyFn(name)
}

// SelectorAdapter wraps a decision.StrategySelector to implement StrategySelectorInterface.
type SelectorAdapter struct {
	GetRegistryFn func() StrategyRegistryInterface
}

func (a *SelectorAdapter) GetRegistryInterface() StrategyRegistryInterface {
	if a.GetRegistryFn == nil {
		return nil
	}
	return a.GetRegistryFn()
}

// Helper function to convert time duration for minimum hold time checks
func MinutesSince(t time.Time) float64 {
	return time.Since(t).Minutes()
}
