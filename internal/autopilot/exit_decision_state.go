// Package autopilot provides automated trading functionality.
// Epic 10, Story 10.3: Exit Decision State Tracking
// This file defines types for tracking and exposing exit decision state to the UI.
package autopilot

import (
	"sync"
	"time"
)

// ExitCheckStatus represents the status of an individual exit check.
type ExitCheckStatus string

const (
	// ExitCheckStatusPassed indicates the check passed (no exit signal)
	ExitCheckStatusPassed ExitCheckStatus = "PASSED"
	// ExitCheckStatusFailed indicates the check failed (would trigger exit)
	ExitCheckStatusFailed ExitCheckStatus = "FAILED"
	// ExitCheckStatusPending indicates the check is pending evaluation
	ExitCheckStatusPending ExitCheckStatus = "PENDING"
	// ExitCheckStatusDisabled indicates the check is disabled
	ExitCheckStatusDisabled ExitCheckStatus = "DISABLED"
	// ExitCheckStatusActive indicates the check is actively monitoring (e.g., trailing SL)
	ExitCheckStatusActive ExitCheckStatus = "ACTIVE"
)

// OverallDecision represents the overall exit decision.
type OverallDecision string

const (
	// OverallDecisionHold indicates the position should be held
	OverallDecisionHold OverallDecision = "HOLD"
	// OverallDecisionExit indicates an exit has been triggered
	OverallDecisionExit OverallDecision = "EXIT"
)

// MinHoldTimeSafeguard tracks minimum hold time status.
type MinHoldTimeSafeguard struct {
	RequiredSeconds int64 `json:"required_seconds"`
	ElapsedSeconds  int64 `json:"elapsed_seconds"`
	Met             bool  `json:"met"`
}

// BreakEvenSafeguard tracks breakeven achievement status.
type BreakEvenSafeguard struct {
	Achieved   bool    `json:"achieved"`
	AchievedAt int64   `json:"achieved_at,omitempty"` // Unix timestamp in milliseconds
	BEPrice    float64 `json:"be_price,omitempty"`
}

// ConsecutiveSignalsSafeguard tracks consecutive exit signals requirement.
type ConsecutiveSignalsSafeguard struct {
	Required int `json:"required"`
	Current  int `json:"current"`
}

// HoldSafeguards contains all safeguard checks before allowing exit.
type HoldSafeguards struct {
	MinHoldTime        MinHoldTimeSafeguard        `json:"min_hold_time"`
	Breakeven          BreakEvenSafeguard          `json:"breakeven"`
	ConsecutiveSignals ConsecutiveSignalsSafeguard `json:"consecutive_signals"`
}

// ADXDetails contains ADX indicator details.
type ADXDetails struct {
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Status    string  `json:"status"`    // "strong_trend", "weak_trend", "ranging"
	Direction string  `json:"direction"` // "increasing", "decreasing", "flat"
}

// RSIDetails contains RSI indicator details.
type RSIDetails struct {
	Value      float64 `json:"value"`
	State      string  `json:"state"` // "oversold", "normal", "overbought"
	Overbought float64 `json:"overbought"`
	Oversold   float64 `json:"oversold"`
}

// EMACrossDetails contains EMA cross detection details.
type EMACrossDetails struct {
	Detected bool   `json:"detected"`
	Type     string `json:"type,omitempty"` // "bullish", "bearish", or null
}

// ReversalSignalsDetails contains reversal signal count details.
type ReversalSignalsDetails struct {
	Count    int `json:"count"`
	Required int `json:"required"`
}

// MACDDetails contains MACD histogram details.
type MACDDetails struct {
	Histogram float64 `json:"histogram"`
	Direction string  `json:"direction"` // "bullish", "bearish", "neutral"
}

// TrendReversalDetails contains all trend reversal check details for Classic mode.
type TrendReversalDetails struct {
	ADX             ADXDetails             `json:"adx"`
	RSI             RSIDetails             `json:"rsi"`
	EMACross        EMACrossDetails        `json:"ema_cross"`
	ReversalSignals ReversalSignalsDetails `json:"reversal_signals"`
	MACD            *MACDDetails           `json:"macd,omitempty"`
}

// EfficiencyExitDetails contains efficiency-based exit check details.
type EfficiencyExitDetails struct {
	PeakProfitPercent    float64 `json:"peak_profit_percent"`
	CurrentProfitPercent float64 `json:"current_profit_percent"`
	EfficiencyPercent    float64 `json:"efficiency_percent"`
	ThresholdPercent     float64 `json:"threshold_percent"`
}

// TrailingSLDetails contains trailing stop loss details.
type TrailingSLDetails struct {
	Active          bool    `json:"active"`
	SLPrice         float64 `json:"sl_price"`
	CurrentPrice    float64 `json:"current_price"`
	DistancePercent float64 `json:"distance_percent"`
}

// DynamicTPDetails contains dynamic take profit details.
type DynamicTPDetails struct {
	Active          bool    `json:"active"`
	TPPrice         float64 `json:"tp_price"`
	CurrentPrice    float64 `json:"current_price"`
	ProgressPercent float64 `json:"progress_percent"`
}

// NewEngineDetails contains New Engine mode specific details.
type NewEngineDetails struct {
	Strategy        string  `json:"strategy"`
	EntryRegime     string  `json:"entry_regime"`
	CurrentRegime   string  `json:"current_regime"`
	RegimeChanged   bool    `json:"regime_changed"`
	TechnicalScore  float64 `json:"technical_score"`
	ExitSignal      float64 `json:"exit_signal"` // 0-100 exit signal strength
	StrategyRules   []string `json:"strategy_rules,omitempty"`
}

// ExitCheck represents an individual exit condition check.
type ExitCheck struct {
	Name      string          `json:"name"`      // "TREND_REVERSAL", "EFFICIENCY_EXIT", "TRAILING_SL", "DYNAMIC_TP"
	Priority  int             `json:"priority"`  // 1-4 (P1 highest)
	Status    ExitCheckStatus `json:"status"`    // "PASSED", "FAILED", "PENDING", "DISABLED", "ACTIVE"
	WouldExit bool            `json:"would_exit"` // Whether this check would trigger exit

	// Details - only one will be populated based on check type
	TrendReversalDetails  *TrendReversalDetails  `json:"trend_reversal_details,omitempty"`
	EfficiencyDetails     *EfficiencyExitDetails `json:"efficiency_details,omitempty"`
	TrailingSLDetails     *TrailingSLDetails     `json:"trailing_sl_details,omitempty"`
	DynamicTPDetails      *DynamicTPDetails      `json:"dynamic_tp_details,omitempty"`
	NewEngineDetails      *NewEngineDetails      `json:"new_engine_details,omitempty"`
}

// ExitDecisionState represents the complete exit decision state for a position.
type ExitDecisionState struct {
	mu sync.RWMutex `json:"-"` // Mutex for thread-safe access

	Symbol          string          `json:"symbol"`
	DecisionMode    DecisionMode    `json:"decision_mode"` // "classic" or "new_engine"
	OverallDecision OverallDecision `json:"overall_decision"`
	DecisionReason  string          `json:"decision_reason"`
	LastCheckTime   int64           `json:"last_check_time"` // Unix timestamp in milliseconds

	HoldSafeguards HoldSafeguards `json:"hold_safeguards"`
	ExitChecks     []ExitCheck    `json:"exit_checks"`
}

// NewExitDecisionState creates a new exit decision state for a symbol.
func NewExitDecisionState(symbol string, mode DecisionMode) *ExitDecisionState {
	return &ExitDecisionState{
		Symbol:          symbol,
		DecisionMode:    mode,
		OverallDecision: OverallDecisionHold,
		DecisionReason:  "Initializing exit decision monitoring",
		LastCheckTime:   time.Now().UnixMilli(),
		HoldSafeguards: HoldSafeguards{
			MinHoldTime: MinHoldTimeSafeguard{
				RequiredSeconds: 300, // Default 5 minutes
				ElapsedSeconds:  0,
				Met:             false,
			},
			Breakeven: BreakEvenSafeguard{
				Achieved: false,
			},
			ConsecutiveSignals: ConsecutiveSignalsSafeguard{
				Required: 2, // Default 2 consecutive signals
				Current:  0,
			},
		},
		ExitChecks: make([]ExitCheck, 0, 4),
	}
}

// UpdateSafeguards updates the hold safeguards state.
func (s *ExitDecisionState) UpdateSafeguards(safeguards HoldSafeguards) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.HoldSafeguards = safeguards
}

// SetOverallDecision sets the overall decision and reason.
func (s *ExitDecisionState) SetOverallDecision(decision OverallDecision, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OverallDecision = decision
	s.DecisionReason = reason
	s.LastCheckTime = time.Now().UnixMilli()
}

// SetExitChecks replaces all exit checks.
func (s *ExitDecisionState) SetExitChecks(checks []ExitCheck) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ExitChecks = checks
}

// AddExitCheck adds an exit check to the state.
func (s *ExitDecisionState) AddExitCheck(check ExitCheck) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ExitChecks = append(s.ExitChecks, check)
}

// ClearExitChecks clears all exit checks.
func (s *ExitDecisionState) ClearExitChecks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ExitChecks = s.ExitChecks[:0]
}

// Clone creates a thread-safe copy of the state.
func (s *ExitDecisionState) Clone() *ExitDecisionState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := &ExitDecisionState{
		Symbol:          s.Symbol,
		DecisionMode:    s.DecisionMode,
		OverallDecision: s.OverallDecision,
		DecisionReason:  s.DecisionReason,
		LastCheckTime:   s.LastCheckTime,
		HoldSafeguards:  s.HoldSafeguards,
		ExitChecks:      make([]ExitCheck, len(s.ExitChecks)),
	}
	copy(clone.ExitChecks, s.ExitChecks)
	return clone
}

// AllSafeguardsMet returns true if all hold safeguards are satisfied.
func (s *ExitDecisionState) AllSafeguardsMet() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.HoldSafeguards.MinHoldTime.Met && s.HoldSafeguards.Breakeven.Achieved
}

// HasExitSignal returns true if any exit check would trigger an exit.
func (s *ExitDecisionState) HasExitSignal() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, check := range s.ExitChecks {
		if check.WouldExit {
			return true
		}
	}
	return false
}

// GetTriggeredCheck returns the highest priority check that would trigger an exit.
func (s *ExitDecisionState) GetTriggeredCheck() *ExitCheck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var triggered *ExitCheck
	for i := range s.ExitChecks {
		if s.ExitChecks[i].WouldExit {
			if triggered == nil || s.ExitChecks[i].Priority < triggered.Priority {
				triggered = &s.ExitChecks[i]
			}
		}
	}
	return triggered
}

// ExitDecisionStateManager manages exit decision states for multiple positions.
type ExitDecisionStateManager struct {
	mu     sync.RWMutex
	states map[string]*ExitDecisionState // Keyed by symbol
}

// NewExitDecisionStateManager creates a new state manager.
func NewExitDecisionStateManager() *ExitDecisionStateManager {
	return &ExitDecisionStateManager{
		states: make(map[string]*ExitDecisionState),
	}
}

// GetState returns the exit decision state for a symbol.
func (m *ExitDecisionStateManager) GetState(symbol string) *ExitDecisionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[symbol]
}

// GetOrCreateState returns existing state or creates a new one.
func (m *ExitDecisionStateManager) GetOrCreateState(symbol string, mode DecisionMode) *ExitDecisionState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.states[symbol]; exists {
		return state
	}

	state := NewExitDecisionState(symbol, mode)
	m.states[symbol] = state
	return state
}

// SetState sets the exit decision state for a symbol.
func (m *ExitDecisionStateManager) SetState(symbol string, state *ExitDecisionState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[symbol] = state
}

// RemoveState removes the exit decision state for a symbol.
func (m *ExitDecisionStateManager) RemoveState(symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, symbol)
}

// GetAllStates returns a copy of all states.
func (m *ExitDecisionStateManager) GetAllStates() map[string]*ExitDecisionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ExitDecisionState, len(m.states))
	for k, v := range m.states {
		result[k] = v.Clone()
	}
	return result
}
