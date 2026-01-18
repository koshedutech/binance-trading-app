// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.6: Strategy Registry
package decision

import (
	"fmt"
	"math"
	"time"
)

// floatEpsilon is the tolerance for float64 equality comparisons.
const floatEpsilon = 1e-9

// floatEqual compares two float64 values with epsilon tolerance.
func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < floatEpsilon
}

// Strategy defines the interface for trading strategies.
// Each strategy encapsulates its own scoring logic, entry/exit conditions,
// and market regime preferences.
type Strategy interface {
	// Name returns the unique identifier for this strategy.
	Name() string

	// SupportedRegimes returns the market regimes this strategy works best with.
	// A strategy may support multiple regimes with different effectiveness.
	SupportedRegimes() []MarketRegime

	// RequiredIndicators returns the list of indicators needed for this strategy.
	// These indicators must be available in the CoinState for proper scoring.
	RequiredIndicators() []string

	// CalculateScore evaluates the coin state and returns a strategy-specific score.
	// The score breakdown includes component scores for transparency.
	CalculateScore(state *CoinState) StrategyScore

	// GetEntryConditions returns the conditions that must be met to enter a position.
	// These are evaluated against the current coin state.
	GetEntryConditions() []Condition

	// GetExitConditions returns the conditions that trigger position exit.
	// These are evaluated continuously while a position is open.
	GetExitConditions() []Condition
}

// ConditionType represents the type of condition evaluation.
type ConditionType string

const (
	// ConditionTypeThreshold compares a value against a threshold.
	ConditionTypeThreshold ConditionType = "THRESHOLD"
	// ConditionTypeComparison compares two values.
	ConditionTypeComparison ConditionType = "COMPARISON"
	// ConditionTypeBoolean checks a boolean state.
	ConditionTypeBoolean ConditionType = "BOOLEAN"
	// ConditionTypeRange checks if a value is within a range.
	ConditionTypeRange ConditionType = "RANGE"
)

// ConditionOperator represents the comparison operator for conditions.
type ConditionOperator string

const (
	// OperatorGreaterThan checks if value > threshold.
	OperatorGreaterThan ConditionOperator = ">"
	// OperatorGreaterOrEqual checks if value >= threshold.
	OperatorGreaterOrEqual ConditionOperator = ">="
	// OperatorLessThan checks if value < threshold.
	OperatorLessThan ConditionOperator = "<"
	// OperatorLessOrEqual checks if value <= threshold.
	OperatorLessOrEqual ConditionOperator = "<="
	// OperatorEqual checks if value == threshold.
	OperatorEqual ConditionOperator = "=="
	// OperatorNotEqual checks if value != threshold.
	OperatorNotEqual ConditionOperator = "!="
	// OperatorInRange checks if min <= value <= max.
	OperatorInRange ConditionOperator = "IN_RANGE"
)

// Condition represents a single entry or exit condition.
// Conditions are evaluated against the current market state to determine
// whether to enter or exit a position.
type Condition struct {
	// Name is a unique identifier for this condition (e.g., "adx_strength").
	Name string `json:"name"`

	// Description is a human-readable explanation of the condition.
	Description string `json:"description"`

	// Type specifies how the condition is evaluated.
	Type ConditionType `json:"type"`

	// Operator defines the comparison operation.
	Operator ConditionOperator `json:"operator"`

	// Indicator is the indicator/field name to evaluate (e.g., "adx", "rsi").
	Indicator string `json:"indicator"`

	// Threshold is the value to compare against for threshold conditions.
	Threshold float64 `json:"threshold,omitempty"`

	// MinValue is the minimum value for range conditions.
	MinValue float64 `json:"min_value,omitempty"`

	// MaxValue is the maximum value for range conditions.
	MaxValue float64 `json:"max_value,omitempty"`

	// Required indicates if this condition must pass (vs. being optional/weighted).
	Required bool `json:"required"`

	// Weight is the importance of this condition in weighted scoring (0.0-1.0).
	Weight float64 `json:"weight"`

	// BlockCategory indicates what type of block this creates if failed.
	// Empty string means no block (just affects score).
	BlockCategory BlockingCategory `json:"block_category,omitempty"`
}

// NewCondition creates a new Condition with sensible defaults.
func NewCondition(name, description, indicator string) *Condition {
	return &Condition{
		Name:        name,
		Description: description,
		Indicator:   indicator,
		Type:        ConditionTypeThreshold,
		Operator:    OperatorGreaterOrEqual,
		Required:    true,
		Weight:      1.0,
	}
}

// WithThreshold sets the threshold and operator for the condition.
func (c *Condition) WithThreshold(op ConditionOperator, threshold float64) *Condition {
	c.Type = ConditionTypeThreshold
	c.Operator = op
	c.Threshold = threshold
	return c
}

// WithRange sets the range values for the condition.
func (c *Condition) WithRange(min, max float64) *Condition {
	c.Type = ConditionTypeRange
	c.Operator = OperatorInRange
	c.MinValue = min
	c.MaxValue = max
	return c
}

// WithWeight sets the weight for the condition.
func (c *Condition) WithWeight(weight float64) *Condition {
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	c.Weight = weight
	return c
}

// WithRequired sets whether the condition is required.
func (c *Condition) WithRequired(required bool) *Condition {
	c.Required = required
	return c
}

// WithBlockCategory sets the blocking category for failed conditions.
func (c *Condition) WithBlockCategory(category BlockingCategory) *Condition {
	c.BlockCategory = category
	return c
}

// Evaluate checks if the condition is met given the current value.
// Returns true if the condition passes, false otherwise.
func (c *Condition) Evaluate(value float64) bool {
	switch c.Operator {
	case OperatorGreaterThan:
		return value > c.Threshold
	case OperatorGreaterOrEqual:
		return value >= c.Threshold
	case OperatorLessThan:
		return value < c.Threshold
	case OperatorLessOrEqual:
		return value <= c.Threshold
	case OperatorEqual:
		return floatEqual(value, c.Threshold)
	case OperatorNotEqual:
		return !floatEqual(value, c.Threshold)
	case OperatorInRange:
		return value >= c.MinValue && value <= c.MaxValue
	default:
		return false
	}
}

// StrategyScore represents the scoring result from a strategy.
// It provides a breakdown of component scores for transparency.
type StrategyScore struct {
	// StrategyName is the name of the strategy that generated this score.
	StrategyName string `json:"strategy_name"`

	// FinalScore is the aggregated score (0-100).
	FinalScore float64 `json:"final_score"`

	// TechnicalScore is the technical indicator component (0-40).
	TechnicalScore float64 `json:"technical_score"`

	// ContextScore is the market context component (0-30).
	ContextScore float64 `json:"context_score"`

	// LLMScore is the AI/LLM confidence component (0-20).
	LLMScore float64 `json:"llm_score"`

	// HistoryScore is the historical performance component (0-10).
	HistoryScore float64 `json:"history_score"`

	// ComponentDetails provides additional breakdown information.
	ComponentDetails map[string]float64 `json:"component_details,omitempty"`

	// EntryConditionsMet is the count of entry conditions passed.
	EntryConditionsMet int `json:"entry_conditions_met"`

	// EntryConditionsTotal is the total number of entry conditions.
	EntryConditionsTotal int `json:"entry_conditions_total"`

	// FailedConditions lists the names of conditions that failed.
	FailedConditions []string `json:"failed_conditions,omitempty"`

	// BlockingReasons contains any blocking reasons identified.
	BlockingReasons []BlockingReason `json:"blocking_reasons,omitempty"`

	// Recommendation is the strategy's action recommendation.
	Recommendation StrategyRecommendation `json:"recommendation"`

	// Confidence is the strategy's confidence level (0-100).
	Confidence float64 `json:"confidence"`

	// Timestamp is when this score was calculated (Unix milliseconds).
	Timestamp int64 `json:"timestamp"`
}

// StrategyRecommendation represents the strategy's action recommendation.
type StrategyRecommendation string

const (
	// RecommendEnterLong suggests opening a long position.
	RecommendEnterLong StrategyRecommendation = "ENTER_LONG"
	// RecommendEnterShort suggests opening a short position.
	RecommendEnterShort StrategyRecommendation = "ENTER_SHORT"
	// RecommendHold suggests maintaining current position.
	RecommendHold StrategyRecommendation = "HOLD"
	// RecommendExit suggests closing the current position.
	RecommendExit StrategyRecommendation = "EXIT"
	// RecommendAvoid suggests avoiding this symbol entirely.
	RecommendAvoid StrategyRecommendation = "AVOID"
)

// NewStrategyScore creates a new StrategyScore with the given strategy name.
func NewStrategyScore(strategyName string) *StrategyScore {
	return &StrategyScore{
		StrategyName:     strategyName,
		ComponentDetails: make(map[string]float64),
		FailedConditions: []string{},
		BlockingReasons:  []BlockingReason{},
		Recommendation:   RecommendHold,
		Timestamp:        time.Now().UnixMilli(),
	}
}

// CalculateFinal computes the final score from components.
func (ss *StrategyScore) CalculateFinal() {
	// Standard weights: Technical(40) + Context(30) + LLM(20) + History(10) = 100
	ss.FinalScore = ss.TechnicalScore + ss.ContextScore + ss.LLMScore + ss.HistoryScore

	// Clamp to valid range
	if ss.FinalScore < 0 {
		ss.FinalScore = 0
	}
	if ss.FinalScore > 100 {
		ss.FinalScore = 100
	}
}

// IsBlocked returns true if there are any hard blocking reasons.
func (ss *StrategyScore) IsBlocked() bool {
	for _, reason := range ss.BlockingReasons {
		if reason.Category == CategoryHardBlock {
			return true
		}
	}
	return false
}

// PassesThreshold checks if the final score meets the minimum threshold.
func (ss *StrategyScore) PassesThreshold(threshold float64) bool {
	return ss.FinalScore >= threshold && !ss.IsBlocked()
}

// AddBlockingReason adds a blocking reason to the score.
func (ss *StrategyScore) AddBlockingReason(reason BlockingReason) {
	ss.BlockingReasons = append(ss.BlockingReasons, reason)
}

// AddFailedCondition records a failed condition.
func (ss *StrategyScore) AddFailedCondition(conditionName string) {
	ss.FailedConditions = append(ss.FailedConditions, conditionName)
}

// SetComponentDetail sets a detail value for the score breakdown.
func (ss *StrategyScore) SetComponentDetail(key string, value float64) {
	if ss.ComponentDetails == nil {
		ss.ComponentDetails = make(map[string]float64)
	}
	ss.ComponentDetails[key] = value
}

// String returns a human-readable summary of the score.
func (ss *StrategyScore) String() string {
	return fmt.Sprintf(
		"[%s] Score: %.1f (Tech: %.1f, Ctx: %.1f, LLM: %.1f, Hist: %.1f) | %s | Conf: %.0f%% | Conditions: %d/%d",
		ss.StrategyName,
		ss.FinalScore,
		ss.TechnicalScore,
		ss.ContextScore,
		ss.LLMScore,
		ss.HistoryScore,
		ss.Recommendation,
		ss.Confidence,
		ss.EntryConditionsMet,
		ss.EntryConditionsTotal,
	)
}

// StrategyInfo provides metadata about a registered strategy.
type StrategyInfo struct {
	// Name is the strategy's unique identifier.
	Name string `json:"name"`

	// Description is a human-readable explanation.
	Description string `json:"description"`

	// SupportedRegimes lists the market regimes this strategy works with.
	SupportedRegimes []MarketRegime `json:"supported_regimes"`

	// RequiredIndicators lists the indicators needed by this strategy.
	RequiredIndicators []string `json:"required_indicators"`

	// EntryConditionCount is the number of entry conditions.
	EntryConditionCount int `json:"entry_condition_count"`

	// ExitConditionCount is the number of exit conditions.
	ExitConditionCount int `json:"exit_condition_count"`

	// RegisteredAt is when this strategy was registered (Unix milliseconds).
	RegisteredAt int64 `json:"registered_at"`

	// Enabled indicates if this strategy is available for use.
	Enabled bool `json:"enabled"`
}

// StrategyDescriber is an optional interface that strategies can implement
// to provide a human-readable description.
type StrategyDescriber interface {
	Description() string
}

// NewStrategyInfo creates a StrategyInfo from a Strategy.
func NewStrategyInfo(strategy Strategy) *StrategyInfo {
	info := &StrategyInfo{
		Name:                strategy.Name(),
		SupportedRegimes:    strategy.SupportedRegimes(),
		RequiredIndicators:  strategy.RequiredIndicators(),
		EntryConditionCount: len(strategy.GetEntryConditions()),
		ExitConditionCount:  len(strategy.GetExitConditions()),
		RegisteredAt:        time.Now().UnixMilli(),
		Enabled:             true,
	}

	// Check if strategy implements StrategyDescriber for description
	if describer, ok := strategy.(StrategyDescriber); ok {
		info.Description = describer.Description()
	}

	return info
}
