// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.19: Decision Output Interface
package decision

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Action represents the decision action type
type Action string

const (
	ActionEnterLong  Action = "ENTER_LONG"
	ActionEnterShort Action = "ENTER_SHORT"
	ActionHold       Action = "HOLD"
	ActionExit       Action = "EXIT"
	ActionBlocked    Action = "BLOCKED"
)

// Confidence level thresholds
const (
	ConfidenceHighThreshold   = 75
	ConfidenceMediumThreshold = 55
)

// PositionDecision represents a trading decision from the Decision Engine.
// This is the standardized output format for the Position Decision Engine.
// Note: Named PositionDecision to avoid conflict with existing Decision type in coin_state.go
type PositionDecision struct {
	// Identification
	ID        string    `json:"id"`      // Unique decision ID (UUID)
	Symbol    string    `json:"symbol"`
	UserID    int       `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`

	// Decision
	Action         Action  `json:"action"`
	Score          int     `json:"score"`           // Raw additive score (0-100)
	CalibratedConf float64 `json:"calibrated_conf"` // Historical win rate
	Confidence     string  `json:"confidence"`      // "HIGH", "MEDIUM", "LOW"

	// Strategy Context
	Strategy   string       `json:"strategy"`
	Regime     MarketRegime `json:"regime"`
	RegimeConf float64      `json:"regime_confidence"`

	// Blocking Information
	IsBlocked       bool             `json:"is_blocked"`
	BlockingReasons []BlockingReason `json:"blocking_reasons"`

	// Technical Context
	Indicators map[string]float64 `json:"indicators"`

	// LLM Context (optional, for audit)
	LLMContext *LLMContext `json:"llm_context,omitempty"`

	// Execution Metadata
	ExecutedAt     *time.Time `json:"executed_at,omitempty"`
	OrderID        string     `json:"order_id,omitempty"`
	ExecutionError string     `json:"execution_error,omitempty"`

	// Audit Trail
	PreviousDecisionID string `json:"previous_decision_id,omitempty"`
	DecisionChain      int    `json:"decision_chain"` // Number of decisions for this symbol in session

	// Actor Tracking (Story 11.20)
	Actor     Actor      `json:"actor"`
	ActorInfo *ActorInfo `json:"actor_info,omitempty"`

	// Rollback tracking
	RolledBack     bool       `json:"rolled_back,omitempty"`
	RollbackReason string     `json:"rollback_reason,omitempty"`
	RollbackAt     *time.Time `json:"rollback_at,omitempty"`
}

// NewPositionDecision creates a new PositionDecision with generated ID and timestamp
func NewPositionDecision(symbol string, userID int) *PositionDecision {
	return &PositionDecision{
		ID:              uuid.New().String(),
		Symbol:          symbol,
		UserID:          userID,
		Timestamp:       time.Now(),
		Indicators:      make(map[string]float64),
		BlockingReasons: []BlockingReason{},
		DecisionChain:   1,
		Actor:           ActorUnknown, // Default to unknown, should be set explicitly
	}
}

// DecisionBuilder helps construct decisions with a fluent API
type DecisionBuilder struct {
	decision *PositionDecision
}

// NewDecisionBuilder creates a new builder
func NewDecisionBuilder(symbol string, userID int) *DecisionBuilder {
	return &DecisionBuilder{
		decision: NewPositionDecision(symbol, userID),
	}
}

// WithScore sets the score and determines confidence level
func (b *DecisionBuilder) WithScore(score int, calibratedConf float64) *DecisionBuilder {
	// Clamp score to valid range
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	b.decision.Score = score
	b.decision.CalibratedConf = calibratedConf
	b.decision.Confidence = GetConfidenceLevel(score)
	return b
}

// WithAction sets the action explicitly (use when not deriving from score)
func (b *DecisionBuilder) WithAction(action Action) *DecisionBuilder {
	b.decision.Action = action
	return b
}

// WithStrategy sets the strategy and regime
func (b *DecisionBuilder) WithStrategy(strategy string, regime MarketRegime, regimeConf float64) *DecisionBuilder {
	b.decision.Strategy = strategy
	b.decision.Regime = regime
	b.decision.RegimeConf = regimeConf
	return b
}

// WithBlocking sets blocking information
func (b *DecisionBuilder) WithBlocking(reasons []BlockingReason) *DecisionBuilder {
	if reasons == nil {
		b.decision.BlockingReasons = []BlockingReason{}
		b.decision.IsBlocked = false
	} else {
		b.decision.BlockingReasons = reasons
		b.decision.IsBlocked = len(reasons) > 0
	}
	return b
}

// WithIndicators sets the technical indicators
func (b *DecisionBuilder) WithIndicators(indicators map[string]float64) *DecisionBuilder {
	if indicators == nil {
		b.decision.Indicators = make(map[string]float64)
	} else {
		// Copy map to prevent external mutation
		b.decision.Indicators = make(map[string]float64, len(indicators))
		for k, v := range indicators {
			b.decision.Indicators[k] = v
		}
	}
	return b
}

// WithLLMContext attaches the full LLM context
func (b *DecisionBuilder) WithLLMContext(ctx *LLMContext) *DecisionBuilder {
	b.decision.LLMContext = ctx
	return b
}

// WithPreviousDecision links to the previous decision for audit trail
func (b *DecisionBuilder) WithPreviousDecision(prevID string, chainCount int) *DecisionBuilder {
	b.decision.PreviousDecisionID = prevID
	if chainCount < 1 {
		chainCount = 1
	}
	b.decision.DecisionChain = chainCount
	return b
}

// WithActor sets the actor who initiated this decision
func (b *DecisionBuilder) WithActor(actor Actor, info *ActorInfo) *DecisionBuilder {
	b.decision.Actor = actor
	if info != nil {
		b.decision.ActorInfo = info.Copy()
	}
	return b
}

// Build finalizes and returns the Decision.
// It determines the action if not explicitly set.
func (b *DecisionBuilder) Build() *PositionDecision {
	// If action not set, determine it
	if b.decision.Action == "" {
		b.decision.Action = DetermineAction(b.decision.Score, b.decision.IsBlocked, b.decision.Regime)
	}
	return b.decision
}

// DetermineAction determines the action based on score and blocking
func DetermineAction(score int, isBlocked bool, regime MarketRegime) Action {
	// Blocked always takes precedence
	if isBlocked {
		return ActionBlocked
	}

	// High confidence scores lead to entry
	if score >= ConfidenceHighThreshold {
		// In trending markets, trend following is preferred
		if regime == RegimeTrending {
			return ActionEnterLong // Could be short based on trend direction
		}
		// In volatile markets, consider breakout
		if regime == RegimeVolatile {
			return ActionEnterLong
		}
		// Otherwise, cautious approach
		return ActionHold
	}

	// Medium confidence - hold
	if score >= ConfidenceMediumThreshold {
		return ActionHold
	}

	// Low confidence - no action
	return ActionHold
}

// DetermineActionWithDirection determines the action with explicit direction
func DetermineActionWithDirection(score int, isBlocked bool, isLong bool, scoreThreshold int) Action {
	if isBlocked {
		return ActionBlocked
	}

	if score >= scoreThreshold {
		if isLong {
			return ActionEnterLong
		}
		return ActionEnterShort
	}

	return ActionHold
}

// GetConfidenceLevel returns "HIGH", "MEDIUM", "LOW" based on score
func GetConfidenceLevel(score int) string {
	if score >= ConfidenceHighThreshold {
		return "HIGH"
	}
	if score >= ConfidenceMediumThreshold {
		return "MEDIUM"
	}
	return "LOW"
}

// MarkExecuted updates the decision with execution info
func (d *PositionDecision) MarkExecuted(orderID string) {
	now := time.Now()
	d.ExecutedAt = &now
	d.OrderID = orderID
}

// MarkError records an execution error
func (d *PositionDecision) MarkError(errMsg string) {
	d.ExecutionError = errMsg
}

// MarkRolledBack marks the decision as rolled back
func (d *PositionDecision) MarkRolledBack(reason string) {
	d.RolledBack = true
	d.RollbackReason = reason
	now := time.Now()
	d.RollbackAt = &now
}

// IsExecuted returns true if the decision was executed
func (d *PositionDecision) IsExecuted() bool {
	return d.ExecutedAt != nil && d.OrderID != ""
}

// HasError returns true if the decision has an execution error
func (d *PositionDecision) HasError() bool {
	return d.ExecutionError != ""
}

// Copy creates a deep copy of the PositionDecision
func (d *PositionDecision) Copy() *PositionDecision {
	if d == nil {
		return nil
	}

	result := &PositionDecision{
		ID:                 d.ID,
		Symbol:             d.Symbol,
		UserID:             d.UserID,
		Timestamp:          d.Timestamp,
		Action:             d.Action,
		Score:              d.Score,
		CalibratedConf:     d.CalibratedConf,
		Confidence:         d.Confidence,
		Strategy:           d.Strategy,
		Regime:             d.Regime,
		RegimeConf:         d.RegimeConf,
		IsBlocked:          d.IsBlocked,
		OrderID:            d.OrderID,
		ExecutionError:     d.ExecutionError,
		PreviousDecisionID: d.PreviousDecisionID,
		DecisionChain:      d.DecisionChain,
		Actor:              d.Actor,
		RolledBack:         d.RolledBack,
		RollbackReason:     d.RollbackReason,
	}

	// Deep copy actor info
	if d.ActorInfo != nil {
		result.ActorInfo = d.ActorInfo.Copy()
	}

	// Deep copy blocking reasons
	if d.BlockingReasons != nil {
		result.BlockingReasons = make([]BlockingReason, len(d.BlockingReasons))
		for i, r := range d.BlockingReasons {
			result.BlockingReasons[i] = r
		}
	}

	// Deep copy indicators
	if d.Indicators != nil {
		result.Indicators = make(map[string]float64, len(d.Indicators))
		for k, v := range d.Indicators {
			result.Indicators[k] = v
		}
	}

	// Deep copy LLM context to prevent mutation affecting original
	if d.LLMContext != nil {
		llmCopy := *d.LLMContext
		// Deep copy the BlockingReasons slice
		if d.LLMContext.BlockingReasons != nil {
			llmCopy.BlockingReasons = make([]string, len(d.LLMContext.BlockingReasons))
			copy(llmCopy.BlockingReasons, d.LLMContext.BlockingReasons)
		}
		result.LLMContext = &llmCopy
	}

	// Copy time pointers
	if d.ExecutedAt != nil {
		t := *d.ExecutedAt
		result.ExecutedAt = &t
	}
	if d.RollbackAt != nil {
		t := *d.RollbackAt
		result.RollbackAt = &t
	}

	return result
}

// ========== API Response Format ==========

// DecisionResponse is the API response format for decisions
type DecisionResponse struct {
	Decision    *PositionDecision `json:"decision"`
	Recommended bool              `json:"recommended"`  // true if score >= threshold
	Message     string            `json:"message"`      // Human-readable summary
	NextAction  string            `json:"next_action"`  // "execute", "review", "skip"
	Actor       ActorResponse     `json:"actor"`        // Actor info for display
}

// ToResponse converts a PositionDecision to API response format
func (d *PositionDecision) ToResponse(threshold int) *DecisionResponse {
	if d == nil {
		return nil
	}

	// Build actor response
	actorResp := d.Actor.ToResponse()
	if d.ActorInfo != nil {
		actorResp.Description = d.ActorInfo.Description
	}

	resp := &DecisionResponse{
		Decision:    d,
		Recommended: d.Score >= threshold && !d.IsBlocked,
		Actor:       actorResp,
	}

	// Determine message and next action
	if d.IsBlocked {
		resp.Message = formatBlockedMessage(d)
		resp.NextAction = "skip"
	} else if d.Score >= threshold {
		resp.Message = formatRecommendedMessage(d)
		resp.NextAction = "execute"
	} else {
		resp.Message = formatReviewMessage(d)
		resp.NextAction = "review"
	}

	return resp
}

// formatBlockedMessage generates a human-readable message for blocked decisions
func formatBlockedMessage(d *PositionDecision) string {
	if len(d.BlockingReasons) == 0 {
		return "Position entry blocked due to market conditions"
	}

	// Get the primary blocking reason
	primaryReason := d.BlockingReasons[0]
	msg := "Position entry blocked: " + primaryReason.Description

	if len(d.BlockingReasons) > 1 {
		msg += " (and " + strconv.Itoa(len(d.BlockingReasons)-1) + " more reasons)"
	}

	return msg
}

// formatRecommendedMessage generates a message for recommended actions
func formatRecommendedMessage(d *PositionDecision) string {
	actionStr := "entry"
	switch d.Action {
	case ActionEnterLong:
		actionStr = "LONG entry"
	case ActionEnterShort:
		actionStr = "SHORT entry"
	case ActionExit:
		actionStr = "EXIT"
	}

	return d.Symbol + ": " + actionStr + " recommended with " + d.Confidence + " confidence (score: " + strconv.Itoa(d.Score) + ")"
}

// formatReviewMessage generates a message for decisions that need review
func formatReviewMessage(d *PositionDecision) string {
	return d.Symbol + ": Score " + strconv.Itoa(d.Score) + " is below threshold - review conditions before entry"
}

// DecisionListResponse is the API response for listing multiple decisions
type DecisionListResponse struct {
	Decisions []*PositionDecision `json:"decisions"`
	Total     int                 `json:"total"`
	Offset    int                 `json:"offset"`
	Limit     int                 `json:"limit"`
}

// DecisionSummary provides a lightweight summary of a decision
type DecisionSummary struct {
	ID         string        `json:"id"`
	Symbol     string        `json:"symbol"`
	Action     Action        `json:"action"`
	Score      int           `json:"score"`
	Confidence string        `json:"confidence"`
	IsBlocked  bool          `json:"is_blocked"`
	Regime     MarketRegime  `json:"regime"`
	Timestamp  time.Time     `json:"timestamp"`
	Executed   bool          `json:"executed"`
	Actor      ActorResponse `json:"actor"` // Actor info for display
}

// ToSummary converts a PositionDecision to a lightweight summary
func (d *PositionDecision) ToSummary() *DecisionSummary {
	if d == nil {
		return nil
	}
	return &DecisionSummary{
		ID:         d.ID,
		Symbol:     d.Symbol,
		Action:     d.Action,
		Score:      d.Score,
		Confidence: d.Confidence,
		IsBlocked:  d.IsBlocked,
		Regime:     d.Regime,
		Timestamp:  d.Timestamp,
		Executed:   d.IsExecuted(),
		Actor:      d.Actor.ToResponse(),
	}
}

// ========== Integration Helpers ==========

// BuildFromCoinState creates a PositionDecision from a CoinState
// This is a convenience method for integration with the existing state management
func BuildFromCoinState(state *CoinState, userID int, score int, calibratedConf float64) *PositionDecision {
	if state == nil {
		return nil
	}

	builder := NewDecisionBuilder(state.Symbol, userID).
		WithScore(score, calibratedConf).
		WithStrategy(state.ActiveStrategy, state.Regime, 0.0).
		WithIndicators(map[string]float64{
			"adx":   state.ADX,
			"atr":   state.ATR,
			"rsi":   state.RSI,
			"ema9":  state.EMA9,
			"ema21": state.EMA21,
			"price": state.Price,
		})

	// Check if blocked
	if state.IsBlocked() {
		// Convert string reasons to BlockingReason objects
		reasons := make([]BlockingReason, 0)
		for _, r := range state.BlockingReasons {
			reason := BlockingReason{
				Description: r,
				Timestamp:   time.Now().UnixMilli(),
			}
			reasons = append(reasons, reason)
		}
		builder.WithBlocking(reasons)
	}

	return builder.Build()
}

// BuildFromLLMContext creates a PositionDecision using LLM context
func BuildFromLLMContext(ctx *LLMContext, userID int) *PositionDecision {
	if ctx == nil {
		return nil
	}

	builder := NewDecisionBuilder(ctx.Symbol, userID).
		WithScore(ctx.FinalScore, float64(ctx.FinalScore)/100.0).
		WithStrategy(ctx.ActiveStrategy, ctx.MarketRegime, ctx.RegimeConfidence).
		WithIndicators(map[string]float64{
			"adx":   ctx.ADXValue,
			"rsi":   ctx.RSI,
			"ema9":  ctx.EMA9,
			"ema21": ctx.EMA21,
			"price": ctx.CurrentPrice,
		}).
		WithLLMContext(ctx)

	if ctx.IsBlocked && len(ctx.BlockingReasons) > 0 {
		reasons := make([]BlockingReason, len(ctx.BlockingReasons))
		for i, r := range ctx.BlockingReasons {
			reasons[i] = BlockingReason{
				Description: r,
				Timestamp:   time.Now().UnixMilli(),
			}
		}
		builder.WithBlocking(reasons)
	}

	return builder.Build()
}
