// Package entrydecision provides data structures and logic for the Entry Decision System.
// This system uses a strategy-first view where coins are matched to enabled strategies,
// supporting both pattern-based (step progress) and score-based (weighted scoring) approaches.
//
// Epic 14: Chain Trading System - Story 14.8: Strategy-First Data Structure
package entrydecision

import (
	"fmt"
	"time"
)

// StrategyType represents the type of strategy evaluation method.
type StrategyType string

const (
	// StrategyTypePattern indicates a pattern-based strategy that requires
	// all conditions to be met in sequence (e.g., Volume Imbalance 3-step pattern).
	// Display shows step progress, not scores.
	StrategyTypePattern StrategyType = "pattern"

	// StrategyTypeScore indicates a score-based strategy that uses
	// weighted criteria for partial matching (e.g., Trend Following).
	// Display shows score values (0-100) and threshold comparison.
	StrategyTypeScore StrategyType = "score"
)

// String returns the string representation of the strategy type.
func (st StrategyType) String() string {
	return string(st)
}

// IsValid checks if the strategy type is valid.
func (st StrategyType) IsValid() bool {
	return st == StrategyTypePattern || st == StrategyTypeScore
}

// PatternStatus represents the current status of a pattern-based coin match.
type PatternStatus string

const (
	// PatternStatusWatching indicates the coin is being monitored but no pattern started.
	PatternStatusWatching PatternStatus = "watching"

	// PatternStatusAccumulation indicates Step 1 is in progress (volume spike detected).
	PatternStatusAccumulation PatternStatus = "accumulation"

	// PatternStatusConsolidating indicates Step 2 is in progress (price consolidation).
	PatternStatusConsolidating PatternStatus = "consolidating"

	// PatternStatusReady indicates all steps are complete and entry is ready.
	PatternStatusReady PatternStatus = "ready"

	// PatternStatusFailed indicates the pattern broke/failed.
	PatternStatusFailed PatternStatus = "failed"

	// PatternStatusExpired indicates the pattern window has expired.
	PatternStatusExpired PatternStatus = "expired"

	// PatternStatusPositionRunning indicates a position is currently open from this pattern.
	// The coin should preserve its entry candle info and show position stage instead of
	// resetting to "watching" state. This status is set when there's an active position
	// on Binance for this symbol.
	PatternStatusPositionRunning PatternStatus = "position_running"
)

// String returns the string representation of the pattern status.
func (ps PatternStatus) String() string {
	return string(ps)
}

// IsEntryReady returns true if the pattern status indicates entry is ready.
func (ps PatternStatus) IsEntryReady() bool {
	return ps == PatternStatusReady
}

// IsActive returns true if the pattern is still being tracked (not failed/expired).
func (ps PatternStatus) IsActive() bool {
	return ps == PatternStatusWatching ||
		ps == PatternStatusAccumulation ||
		ps == PatternStatusConsolidating ||
		ps == PatternStatusReady ||
		ps == PatternStatusPositionRunning
}

// CoinMatch represents a single coin's match status against a strategy.
// This is a unified struct that supports both pattern-based and score-based strategies.
type CoinMatch struct {
	// Common fields for all strategy types
	Symbol    string    `json:"symbol"`     // e.g., "BTCUSDT"
	UpdatedAt time.Time `json:"updated_at"` // When this match was last updated

	// Pattern-based fields (used when strategy type is "pattern")
	Step        int           `json:"step,omitempty"`         // Current step (1, 2, 3, etc.)
	TotalSteps  int           `json:"total_steps,omitempty"`  // Total steps in pattern (e.g., 2 for Volume Imbalance)
	Status      PatternStatus `json:"status,omitempty"`       // Current pattern status
	Details     string        `json:"details,omitempty"`      // Human-readable details (e.g., "3.2x avg")
	StepDetails []StepDetail  `json:"step_details,omitempty"` // Detailed info for each step

	// Score-based fields (used when strategy type is "score")
	Score int  `json:"score,omitempty"` // Score value (0-100)
	Ready bool `json:"ready,omitempty"` // Whether score meets threshold

	// Pattern tracking metrics (for pattern strategies with active tracking)
	ReferenceCandle        *ReferenceCandle `json:"reference_candle,omitempty"`          // Reference candle data (for step 1+)
	ReferenceDetectedAt    *time.Time       `json:"reference_detected_at,omitempty"`     // When reference candle was detected
	CandlesSinceReference  int              `json:"candles_since_reference,omitempty"`   // Number of candles since reference
	SecondsSinceReference  int              `json:"seconds_since_reference,omitempty"`   // Seconds elapsed since reference detection
	ProximityToBreakout    float64          `json:"proximity_to_breakout,omitempty"`     // Current price proximity to breakout (%) - negative = below
	PotentialBreakout      bool             `json:"potential_breakout,omitempty"`        // Whether current candle looks like potential breakout

	// Entry/Breakout candle (when pattern is ready)
	EntryCandle     *EntryCandle `json:"entry_candle,omitempty"`      // Entry candle data when breakout detected
	ReadyAt         *time.Time   `json:"ready_at,omitempty"`          // When pattern became ready (UTC)
	SecondsUntilExpiry int       `json:"seconds_until_expiry,omitempty"` // Seconds until ready pattern expires

	// Additional context (shared)
	Direction    string  `json:"direction,omitempty"`     // "long", "short", or "neutral"
	CurrentPrice float64 `json:"current_price,omitempty"` // Current price for reference
	Volume24h    float64 `json:"volume_24h,omitempty"`    // 24h volume for reference

	// Volume tracking (for real-time display)
	CurrentVolume         float64 `json:"current_volume,omitempty"`          // Current candle volume
	VolumeThreshold       float64 `json:"volume_threshold,omitempty"`        // Volume threshold to trigger (e.g., 3.0 for 3x avg)
	AvgVolume             float64 `json:"avg_volume,omitempty"`              // Average volume used for comparison
	VolumeMultiplier      float64 `json:"volume_multiplier,omitempty"`       // Current volume multiplier vs average
	VolumeDistancePercent float64 `json:"volume_distance_percent,omitempty"` // Distance to volume threshold (negative = below threshold)
	PriceDistancePercent  float64 `json:"price_distance_percent,omitempty"`  // Distance to breakout price (negative = below entry)

	// Price context tracking (for UI price progress bar)
	DayHigh   float64 `json:"day_high,omitempty"`    // Day's highest price
	DayLow    float64 `json:"day_low,omitempty"`     // Day's lowest price
	AvgPrice5 float64 `json:"avg_price_5,omitempty"` // 5-candle average price (same lookback as volume)

	// Position tracking (when position is actually open on Binance)
	HasActivePosition    bool       `json:"has_active_position,omitempty"`    // Whether there's an active position for this coin
	PositionOpenedAt     *time.Time `json:"position_opened_at,omitempty"`     // When position was opened (entry filled)
	PositionEntryPrice   float64    `json:"position_entry_price,omitempty"`   // Position entry price (actual fill price from Binance)
	PositionQuantity     float64    `json:"position_quantity,omitempty"`      // Position quantity
	ChainID              string     `json:"chain_id,omitempty"`               // Chain ID for the position

	// Entry levels from pattern detection (for actual SL/TP prices, not percentages)
	// These are calculated from Support Price (Consolidation Low) at pattern detection time
	SupportPrice    float64 `json:"support_price,omitempty"`     // Support price (ConsolidationLow) - SL for longs
	ResistancePrice float64 `json:"resistance_price,omitempty"`  // Resistance price (ConsolidationHigh) - SL for shorts
	EntryLevelPrice float64 `json:"entry_level_price,omitempty"` // Calculated entry price from pattern
	StopLossPrice   float64 `json:"stop_loss_price,omitempty"`   // Actual SL price (Support with buffer)
	TakeProfitPrice float64 `json:"take_profit_price,omitempty"` // Actual TP price (Entry + Risk × R:R)
	RiskAmount      float64 `json:"risk_amount,omitempty"`       // Risk in price terms (Entry - SL for long)
	RiskPercent     float64 `json:"risk_percent,omitempty"`      // Risk as percentage of entry
}

// ReferenceCandle holds information about the reference candle for pattern tracking.
type ReferenceCandle struct {
	OpenTime         time.Time `json:"open_time"`         // Reference candle open time
	CloseTime        time.Time `json:"close_time"`        // Reference candle close time
	Open             float64   `json:"open"`              // Open price
	High             float64   `json:"high"`              // High price - breakout level for longs
	Low              float64   `json:"low"`               // Low price
	Close            float64   `json:"close"`             // Close price
	Volume           float64   `json:"volume"`            // Volume
	VolumeMultiplier float64   `json:"volume_multiplier"` // Volume multiplier vs average (e.g., 3.2x)
}

// EntryCandle holds information about the entry/breakout candle when pattern completes.
type EntryCandle struct {
	OpenTime         time.Time `json:"open_time"`         // Entry candle open time (UTC)
	CloseTime        time.Time `json:"close_time"`        // Entry candle close time (UTC)
	Open             float64   `json:"open"`              // Open price
	High             float64   `json:"high"`              // High price
	Low              float64   `json:"low"`               // Low price
	Close            float64   `json:"close"`             // Close price at breakout detection
	Volume           float64   `json:"volume"`            // Volume at breakout
	VolumeMultiplier float64   `json:"volume_multiplier"` // Volume multiplier vs consolidation avg
	EntryPrice       float64   `json:"entry_price"`       // Actual entry price sent for order
	DetectedAt       time.Time `json:"detected_at"`       // When breakout was detected (UTC)
	Direction        string    `json:"direction"`         // "long" or "short"
}

// NewPatternCoinMatch creates a new CoinMatch for pattern-based strategies.
func NewPatternCoinMatch(symbol string, step int, status PatternStatus, details string) *CoinMatch {
	return &CoinMatch{
		Symbol:    symbol,
		Step:      step,
		Status:    status,
		Details:   details,
		UpdatedAt: time.Now(),
	}
}

// NewScoreCoinMatch creates a new CoinMatch for score-based strategies.
// Scores are clamped to valid range [0, 100].
func NewScoreCoinMatch(symbol string, score int, threshold int) *CoinMatch {
	// Clamp score to valid range
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	// Clamp threshold to valid range
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 100 {
		threshold = 100
	}

	return &CoinMatch{
		Symbol:    symbol,
		Score:     score,
		Ready:     score >= threshold,
		UpdatedAt: time.Now(),
	}
}

// IsReady returns true if this coin match indicates entry is ready.
// For pattern-based: Status == "ready"
// For score-based: Ready == true (score >= threshold)
// Safe to call on nil receiver - returns false.
func (cm *CoinMatch) IsReady() bool {
	if cm == nil {
		return false
	}
	// Pattern-based check
	if cm.Status != "" {
		return cm.Status.IsEntryReady()
	}
	// Score-based check
	return cm.Ready
}

// String returns a human-readable representation of the coin match.
// Safe to call on nil receiver - returns empty string.
func (cm *CoinMatch) String() string {
	if cm == nil {
		return ""
	}
	if cm.Status != "" {
		// Pattern-based
		return fmt.Sprintf("%s: Step %d (%s) - %s", cm.Symbol, cm.Step, cm.Status, cm.Details)
	}
	// Score-based
	readyStr := "not ready"
	if cm.Ready {
		readyStr = "READY"
	}
	return fmt.Sprintf("%s: Score %d (%s)", cm.Symbol, cm.Score, readyStr)
}

// StrategyMatch represents a strategy with all matching coins.
// This is the core data structure for the strategy-first view in the Entry Decision System.
type StrategyMatch struct {
	// Strategy identification
	Mode        string `json:"mode"`         // Trading mode: "scalp", "swing", "position"
	Strategy    string `json:"strategy"`     // Strategy name: "volume_imbalance", "trend_following"
	SubStrategy string `json:"sub_strategy"` // Sub-strategy name: "ravindra_volume_imbalance"

	// Strategy configuration
	Type       StrategyType `json:"type"`                 // "pattern" or "score"
	Timeframe  string       `json:"timeframe"`            // Primary timeframe: "5m", "15m", "1h"
	Timeframes []string     `json:"timeframes,omitempty"` // All required timeframes
	Threshold  int          `json:"threshold,omitempty"`  // Score threshold (for score-based only)
	Enabled    bool         `json:"enabled"`              // Whether this strategy is enabled

	// Matching coins
	Coins []CoinMatch `json:"coins"` // All coins being tracked by this strategy

	// Strategy requirements (for UI display)
	Requirements *StrategyRequirements `json:"requirements,omitempty"`

	// Real-time countdown timer (per-strategy, based on timeframe)
	NextCandleClose time.Time `json:"next_candle_close,omitempty"` // When next candle closes
	LookingFor      string    `json:"looking_for,omitempty"`       // Direction looking for ("long", "short", "both")

	// Metadata
	UpdatedAt time.Time `json:"updated_at"` // When this match data was last updated
}

// StrategyRequirements holds the data requirements for a strategy.
// This is used for display in the UI and for Coin Profiler aggregation.
type StrategyRequirements struct {
	Timeframes  []string               `json:"timeframes"`            // Required timeframes
	DataFields  []string               `json:"data_fields,omitempty"` // Required data fields (e.g., "volume", "ohlc")
	Filters     map[string]interface{} `json:"filters,omitempty"`     // Filters (e.g., min_volume: 1000000)
	Description string                 `json:"description,omitempty"` // Human-readable description
}

// NewStrategyMatch creates a new StrategyMatch with the given parameters.
func NewStrategyMatch(mode, strategy, subStrategy string, strategyType StrategyType, timeframe string) *StrategyMatch {
	return &StrategyMatch{
		Mode:        mode,
		Strategy:    strategy,
		SubStrategy: subStrategy,
		Type:        strategyType,
		Timeframe:   timeframe,
		Enabled:     true,
		Coins:       []CoinMatch{},
		UpdatedAt:   time.Now(),
	}
}

// WithThreshold sets the score threshold for score-based strategies.
func (sm *StrategyMatch) WithThreshold(threshold int) *StrategyMatch {
	sm.Threshold = threshold
	return sm
}

// WithTimeframes sets all required timeframes for this strategy.
func (sm *StrategyMatch) WithTimeframes(timeframes []string) *StrategyMatch {
	sm.Timeframes = timeframes
	return sm
}

// WithRequirements sets the strategy requirements.
func (sm *StrategyMatch) WithRequirements(req *StrategyRequirements) *StrategyMatch {
	sm.Requirements = req
	return sm
}

// AddCoin adds a coin match to this strategy.
func (sm *StrategyMatch) AddCoin(coin CoinMatch) {
	sm.Coins = append(sm.Coins, coin)
	sm.UpdatedAt = time.Now()
}

// GetReadyCoins returns all coins that are ready for entry.
func (sm *StrategyMatch) GetReadyCoins() []CoinMatch {
	var ready []CoinMatch
	for _, coin := range sm.Coins {
		if coin.IsReady() {
			ready = append(ready, coin)
		}
	}
	return ready
}

// GetReadyCount returns the count of coins ready for entry.
func (sm *StrategyMatch) GetReadyCount() int {
	count := 0
	for _, coin := range sm.Coins {
		if coin.IsReady() {
			count++
		}
	}
	return count
}

// GetWatchingCount returns the count of coins being watched (not ready).
func (sm *StrategyMatch) GetWatchingCount() int {
	return len(sm.Coins) - sm.GetReadyCount()
}

// UniqueKey returns a unique identifier for this strategy match.
// Format: "{mode}:{strategy}:{sub_strategy}:{timeframe}"
func (sm *StrategyMatch) UniqueKey() string {
	if sm.SubStrategy != "" {
		return fmt.Sprintf("%s:%s:%s:%s", sm.Mode, sm.Strategy, sm.SubStrategy, sm.Timeframe)
	}
	return fmt.Sprintf("%s:%s:%s", sm.Mode, sm.Strategy, sm.Timeframe)
}

// String returns a human-readable summary of the strategy match.
func (sm *StrategyMatch) String() string {
	return fmt.Sprintf("[%s] %s/%s (%s) - %d coins (%d ready)",
		sm.Mode, sm.Strategy, sm.SubStrategy, sm.Type, len(sm.Coins), sm.GetReadyCount())
}

// ModeStrategies groups strategies by trading mode for the strategy-first UI view.
type ModeStrategies struct {
	Mode       string          `json:"mode"`       // "scalp", "swing", "position"
	Strategies []StrategyMatch `json:"strategies"` // All strategies in this mode
	Enabled    bool            `json:"enabled"`    // Whether this mode is enabled
}

// NewModeStrategies creates a new ModeStrategies group.
func NewModeStrategies(mode string, enabled bool) *ModeStrategies {
	return &ModeStrategies{
		Mode:       mode,
		Strategies: []StrategyMatch{},
		Enabled:    enabled,
	}
}

// AddStrategy adds a strategy to this mode group.
func (ms *ModeStrategies) AddStrategy(strategy StrategyMatch) {
	ms.Strategies = append(ms.Strategies, strategy)
}

// GetStrategyCount returns the total number of strategies in this mode.
func (ms *ModeStrategies) GetStrategyCount() int {
	return len(ms.Strategies)
}

// GetEnabledCount returns the count of enabled strategies in this mode.
func (ms *ModeStrategies) GetEnabledCount() int {
	count := 0
	for _, s := range ms.Strategies {
		if s.Enabled {
			count++
		}
	}
	return count
}

// GetTotalReadyCoins returns the total count of ready coins across all strategies.
func (ms *ModeStrategies) GetTotalReadyCoins() int {
	count := 0
	for _, s := range ms.Strategies {
		count += s.GetReadyCount()
	}
	return count
}

// EntryDecisionResponse is the API response structure for entry decision strategies.
// This is the top-level response returned by GET /api/entry-decision/strategies.
type EntryDecisionResponse struct {
	// All enabled strategies with their matching coins, grouped by strategy
	Strategies []StrategyMatch `json:"strategies"`

	// Alternative view: strategies grouped by mode for hierarchical UI
	ByMode []ModeStrategies `json:"by_mode,omitempty"`

	// Summary statistics
	TotalStrategies   int `json:"total_strategies"`    // Total number of strategies
	EnabledStrategies int `json:"enabled_strategies"`  // Number of enabled strategies
	TotalCoinsReady   int `json:"total_coins_ready"`   // Total coins ready for entry
	TotalCoinsWatching int `json:"total_coins_watching"` // Total coins being watched

	// Metadata
	UpdatedAt time.Time `json:"updated_at"` // When this response was generated
}

// NewEntryDecisionResponse creates a new EntryDecisionResponse.
func NewEntryDecisionResponse() *EntryDecisionResponse {
	return &EntryDecisionResponse{
		Strategies: []StrategyMatch{},
		ByMode:     []ModeStrategies{},
		UpdatedAt:  time.Now(),
	}
}

// AddStrategy adds a strategy match to the response.
func (edr *EntryDecisionResponse) AddStrategy(strategy StrategyMatch) {
	edr.Strategies = append(edr.Strategies, strategy)
	edr.TotalStrategies++
	if strategy.Enabled {
		edr.EnabledStrategies++
	}
	edr.TotalCoinsReady += strategy.GetReadyCount()
	edr.TotalCoinsWatching += strategy.GetWatchingCount()
}

// GroupByMode organizes strategies into mode groups for hierarchical UI.
// This populates the ByMode field with strategies grouped by trading mode.
func (edr *EntryDecisionResponse) GroupByMode() {
	modeMap := make(map[string]*ModeStrategies)

	for _, strategy := range edr.Strategies {
		mode := strategy.Mode
		if _, exists := modeMap[mode]; !exists {
			modeMap[mode] = NewModeStrategies(mode, true)
		}
		modeMap[mode].AddStrategy(strategy)
	}

	// Convert map to slice with consistent ordering
	modeOrder := []string{"scalp", "swing", "position", "ultra_fast"}
	edr.ByMode = []ModeStrategies{}

	for _, mode := range modeOrder {
		if ms, exists := modeMap[mode]; exists {
			edr.ByMode = append(edr.ByMode, *ms)
			delete(modeMap, mode)
		}
	}

	// Add any remaining modes not in the predefined order
	for _, ms := range modeMap {
		edr.ByMode = append(edr.ByMode, *ms)
	}
}

// CalculateSummary recalculates the summary statistics from the strategies.
func (edr *EntryDecisionResponse) CalculateSummary() {
	edr.TotalStrategies = len(edr.Strategies)
	edr.EnabledStrategies = 0
	edr.TotalCoinsReady = 0
	edr.TotalCoinsWatching = 0

	for _, strategy := range edr.Strategies {
		if strategy.Enabled {
			edr.EnabledStrategies++
		}
		edr.TotalCoinsReady += strategy.GetReadyCount()
		edr.TotalCoinsWatching += strategy.GetWatchingCount()
	}

	edr.UpdatedAt = time.Now()
}

// EntryCandidatesResponse represents all entry candidates across all strategies.
// This is used by GET /api/entry-decision/candidates.
type EntryCandidatesResponse struct {
	// All coins ready for entry, with their strategy context
	Candidates []EntryCandidate `json:"candidates"`

	// Summary
	TotalCandidates int       `json:"total_candidates"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// EntryCandidate represents a single entry opportunity.
type EntryCandidate struct {
	// Coin information
	Symbol       string  `json:"symbol"`
	CurrentPrice float64 `json:"current_price"`

	// Strategy context
	Mode        string       `json:"mode"`
	Strategy    string       `json:"strategy"`
	SubStrategy string       `json:"sub_strategy,omitempty"`
	Type        StrategyType `json:"type"`
	Timeframe   string       `json:"timeframe"`

	// Match details
	Step      int           `json:"step,omitempty"`      // For pattern-based
	Status    PatternStatus `json:"status,omitempty"`    // For pattern-based
	Details   string        `json:"details,omitempty"`   // For pattern-based
	Score     int           `json:"score,omitempty"`     // For score-based
	Threshold int           `json:"threshold,omitempty"` // For score-based

	// Entry recommendation
	Direction  string    `json:"direction"` // "long" or "short"
	Priority   int       `json:"priority"`  // Higher = more urgent (for sorting)
	Confidence float64   `json:"confidence,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NewEntryCandidatesResponse creates a new EntryCandidatesResponse.
func NewEntryCandidatesResponse() *EntryCandidatesResponse {
	return &EntryCandidatesResponse{
		Candidates: []EntryCandidate{},
		UpdatedAt:  time.Now(),
	}
}

// AddCandidate adds an entry candidate to the response.
func (ecr *EntryCandidatesResponse) AddCandidate(candidate EntryCandidate) {
	ecr.Candidates = append(ecr.Candidates, candidate)
	ecr.TotalCandidates = len(ecr.Candidates)
}

// PatternProgress tracks the progress of a multi-step pattern for a coin.
// This is used internally for pattern-based strategy matching.
type PatternProgress struct {
	Symbol      string        `json:"symbol"`
	Strategy    string        `json:"strategy"`    // Which pattern strategy
	SubStrategy string        `json:"sub_strategy"`
	Mode        string        `json:"mode"`
	Timeframe   string        `json:"timeframe"`

	// Step tracking
	CurrentStep int           `json:"current_step"` // Current step (1-based)
	TotalSteps  int           `json:"total_steps"`  // Total steps in pattern
	Status      PatternStatus `json:"status"`

	// Step details
	StepDetails []StepDetail `json:"step_details"`

	// Timing
	StartedAt   time.Time `json:"started_at"`   // When pattern tracking started
	UpdatedAt   time.Time `json:"updated_at"`   // Last update
	ExpiresAt   time.Time `json:"expires_at"`   // When pattern expires if not completed
}

// StepDetail holds information about a specific step in a pattern.
type StepDetail struct {
	StepNumber  int       `json:"step_number"`
	Name        string    `json:"name"`        // e.g., "Volume Spike", "Consolidation"
	Completed   bool      `json:"completed"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Progress    string    `json:"progress"`    // e.g., "3/6 candles"
	Details     string    `json:"details"`     // Additional context
}

// NewPatternProgress creates a new PatternProgress tracker.
func NewPatternProgress(symbol, strategy, subStrategy, mode, timeframe string, totalSteps int) *PatternProgress {
	return &PatternProgress{
		Symbol:      symbol,
		Strategy:    strategy,
		SubStrategy: subStrategy,
		Mode:        mode,
		Timeframe:   timeframe,
		CurrentStep: 1,
		TotalSteps:  totalSteps,
		Status:      PatternStatusWatching,
		StepDetails: make([]StepDetail, totalSteps),
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// AdvanceStep moves the pattern to the next step.
// Returns true if the step was advanced, false if already complete.
func (pp *PatternProgress) AdvanceStep(details string) bool {
	// Already past completion - cannot advance further
	if pp.CurrentStep > pp.TotalSteps {
		return false
	}

	// Mark current step as completed
	if pp.CurrentStep > 0 && pp.CurrentStep <= len(pp.StepDetails) {
		pp.StepDetails[pp.CurrentStep-1].Completed = true
		pp.StepDetails[pp.CurrentStep-1].CompletedAt = time.Now()
		pp.StepDetails[pp.CurrentStep-1].Details = details
	}

	pp.CurrentStep++
	pp.UpdatedAt = time.Now()

	// Check if pattern is now complete (just passed the last step)
	if pp.CurrentStep > pp.TotalSteps {
		pp.Status = PatternStatusReady
	}

	return true
}

// SetStatus updates the pattern status.
func (pp *PatternProgress) SetStatus(status PatternStatus) {
	pp.Status = status
	pp.UpdatedAt = time.Now()
}

// IsComplete returns true if the pattern is fully matched.
func (pp *PatternProgress) IsComplete() bool {
	return pp.Status == PatternStatusReady
}

// IsExpired returns true if the pattern has expired.
func (pp *PatternProgress) IsExpired() bool {
	return pp.Status == PatternStatusExpired ||
		(!pp.ExpiresAt.IsZero() && time.Now().After(pp.ExpiresAt))
}

// ToCoinMatch converts the pattern progress to a CoinMatch for API responses.
// Safe to call on nil receiver - returns empty CoinMatch.
// Includes step_details for detailed UI display of tracking stages.
func (pp *PatternProgress) ToCoinMatch() CoinMatch {
	if pp == nil {
		return CoinMatch{}
	}

	details := fmt.Sprintf("Step %d/%d", pp.CurrentStep, pp.TotalSteps)

	// Safe bounds check: CurrentStep must be >= 1 and <= len(StepDetails)
	stepIdx := pp.CurrentStep - 1 // Convert to 0-based index
	if stepIdx >= 0 && stepIdx < len(pp.StepDetails) && pp.StepDetails[stepIdx].Progress != "" {
		details = pp.StepDetails[stepIdx].Progress
	}

	return CoinMatch{
		Symbol:      pp.Symbol,
		Step:        pp.CurrentStep,
		TotalSteps:  pp.TotalSteps,
		Status:      pp.Status,
		Details:     details,
		StepDetails: pp.StepDetails, // Include full step details for UI display
		UpdatedAt:   pp.UpdatedAt,
	}
}
