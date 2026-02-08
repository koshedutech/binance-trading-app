// Package entrydecision provides real-time pattern matching for the Entry Decision System.
// Epic 14: Chain Trading System - Entry Decision Strategy Requirements & Real-Time Monitoring
package entrydecision

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"binance-trading-bot/internal/coinprofiler"
)

// ============================================================================
// REAL-TIME PATTERN MATCHER
// ============================================================================
//
// The RealtimePatternMatcher evaluates patterns in real-time as candles close.
// It integrates with the CoinProfiler via a callback mechanism:
//
// Flow:
// 1. CoinProfiler receives closed candle from Binance WebSocket
// 2. CoinProfiler calls AddClosedCandle which triggers the callback
// 3. RealtimePatternMatcher.OnCandleClose is called with the candle data
// 4. Patterns are evaluated for all enabled strategies
// 5. If pattern state changed, OnPatternUpdate callback is triggered
// 6. Handler broadcasts update via WebSocket to frontend
//
// ============================================================================

// PatternUpdate represents a change in pattern state for a coin.
// This is broadcast via WebSocket when patterns progress or complete.
type PatternUpdate struct {
	// User identification (for targeted broadcasts)
	UserID string `json:"user_id,omitempty"`

	// Coin identification
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`

	// Pattern state
	Mode        string        `json:"mode"`
	Strategy    string        `json:"strategy"`
	SubStrategy string        `json:"sub_strategy"`
	CurrentStep int           `json:"current_step"`
	TotalSteps  int           `json:"total_steps"`
	Status      PatternStatus `json:"status"`

	// Step details
	StepDetails []StepDetail `json:"step_details"`

	// Entry levels (when pattern is ready or near-ready)
	EntryLevels *EntryLevels `json:"entry_levels,omitempty"`

	// Volume progress (real-time volume ratio for UI display)
	VolumeProgress *VolumeProgress `json:"volume_progress,omitempty"`

	// Reference candle from Stage 1 (persisted into Stage 2+ for context)
	ReferenceCandle *ReferenceCandle `json:"reference_candle,omitempty"`

	// Price context for UI progress bars
	CurrentPrice    float64 `json:"current_price,omitempty"`    // Current market price
	DayHigh         float64 `json:"day_high,omitempty"`         // Day's high price
	DayLow          float64 `json:"day_low,omitempty"`          // Day's low price
	VolumeThreshold float64 `json:"volume_threshold,omitempty"` // Volume threshold (e.g., 3.0 for 3x)

	// Direction - the actual direction being tracked (long/short)
	Direction string `json:"direction,omitempty"`

	// LookingFor - what the strategy is configured to find (long/short/both)
	LookingFor string `json:"looking_for,omitempty"`

	// Countdown timer for next candle close
	NextCandleClose time.Time `json:"next_candle_close,omitempty"`

	// Last evaluation timestamp (so frontend knows data is fresh)
	LastEvaluatedAt time.Time `json:"last_evaluated_at,omitempty"`

	// Position tracking (when position is actually open on Binance)
	HasActivePosition  bool    `json:"has_active_position,omitempty"`  // Whether there's an active position for this coin
	PositionEntryPrice float64 `json:"position_entry_price,omitempty"` // Position entry price (actual fill price from Binance)
	ChainID            string  `json:"chain_id,omitempty"`             // Chain ID for the position

	// Step 3: Order filling fields (when order is placed, waiting for fill)
	OrderPrice         float64 `json:"order_price,omitempty"`          // Limit order price
	OrderQuantityUSD   float64 `json:"order_quantity_usd,omitempty"`   // Order size in USD
	FillTimeoutSeconds int     `json:"fill_timeout_seconds,omitempty"` // Remaining seconds until fill timeout
	FillTimeoutTotal   int     `json:"fill_timeout_total,omitempty"`   // Total fill timeout duration in seconds

	// Entry candle data (the breakout candle that triggered Step 3)
	EntryCandle *EntryCandle `json:"entry_candle,omitempty"`

	// Timing fields for UI display
	ReferenceDetectedAt string `json:"reference_detected_at,omitempty"` // ISO timestamp when reference candle was detected
	BreakoutDetectedAt  string `json:"breakout_detected_at,omitempty"`  // ISO timestamp when breakout was detected (Step 2→3)
	SecondsSinceReference int  `json:"seconds_since_reference,omitempty"` // Seconds elapsed since reference detection
	SecondsUntilExpiry    int  `json:"seconds_until_expiry,omitempty"`   // Seconds until ready pattern expires

	// Timestamp
	UpdatedAt time.Time `json:"updated_at"`
}

// VolumeProgress contains real-time volume progress for UI display.
type VolumeProgress struct {
	CurrentVolume      float64 `json:"current_volume"`       // Current forming candle's volume
	AverageVolume      float64 `json:"average_volume"`       // Average of last N closed candles
	CurrentRatio       float64 `json:"current_ratio"`        // CurrentVolume / AverageVolume
	RequiredRatio      float64 `json:"required_ratio"`       // Threshold to trigger (e.g., 3.0)
	ProgressPercent    float64 `json:"progress_percent"`     // (CurrentRatio / RequiredRatio) * 100
	CandleDirection    string  `json:"candle_direction"`     // "bullish" or "bearish"
	IsApproachingSpike bool    `json:"is_approaching_spike"` // True if > 50% of threshold
	TimeRemainingMs    int64   `json:"time_remaining_ms"`    // Milliseconds until candle close
	LookbackCandles    int     `json:"lookback_candles"`     // How many candles used for average
	CurrentPrice       float64 `json:"current_price"`        // Real-time current price
}

// EntryLevels contains calculated entry, stop-loss, and take-profit levels.
type EntryLevels struct {
	// Entry price (typically reference high for longs)
	EntryPrice float64 `json:"entry_price"`

	// Stop loss price
	StopLoss float64 `json:"stop_loss"`

	// Take profit price
	TakeProfit float64 `json:"take_profit"`

	// Percentage values
	RiskPercent   float64 `json:"risk_percent"`
	RewardPercent float64 `json:"reward_percent"`

	// Risk/Reward ratio
	RiskRewardRatio float64 `json:"risk_reward_ratio"`

	// Reference prices used for calculation
	ReferenceHigh float64 `json:"reference_high,omitempty"`
	ReferenceLow  float64 `json:"reference_low,omitempty"`
	CurrentPrice  float64 `json:"current_price,omitempty"`
}

// PatternUpdateCallback is called when a pattern state changes.
type PatternUpdateCallback func(update PatternUpdate)

// VolumeProgressCallback is called on every tick with volume progress data.
type VolumeProgressCallback func(progress VolumeProgress)

// BreakoutCallback is called immediately when tick-level breakout is detected.
// This enables instant order execution without waiting for scan cycle.
// Parameters: symbol, direction ("long"/"short"), mode, strategyGroup, subStrategy, timeframe, price at breakout
type BreakoutCallback func(symbol, direction, mode, strategyGroup, subStrategy, timeframe string, price float64)

// CapacityChecker is called to check if there's capacity for new entries.
// Returns (canEnter bool, currentCount int, maxCount int).
type CapacityChecker func() (bool, int, int)

// ============================================================================
// PATTERN STATE PERSISTENCE INTERFACE
// ============================================================================

// PersistedPatternState is the serializable representation of a pattern state
// for database storage. It contains all fields needed to restore a pattern
// after server restart.
type PersistedPatternState struct {
	UserID          string          `json:"user_id"`
	Symbol          string          `json:"symbol"`
	Mode            string          `json:"mode"`
	Timeframe       string          `json:"timeframe"`
	Strategy        string          `json:"strategy"`
	SubStrategy     string          `json:"sub_strategy"`
	Status          string          `json:"status"`
	CurrentStep     int             `json:"current_step"`
	Direction       string          `json:"direction"`
	ReferenceCandle json.RawMessage `json:"reference_candle,omitempty"`
	ConsolidationData json.RawMessage `json:"consolidation_data,omitempty"`
	EntryLevels     json.RawMessage `json:"entry_levels,omitempty"`
	PatternData     json.RawMessage `json:"pattern_data,omitempty"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty"`
}

// ConsolidationSnapshot captures consolidation-related state for serialization.
type ConsolidationSnapshot struct {
	ConsolidationCandles  int        `json:"consolidation_candles"`
	ConsolidationLow      float64    `json:"consolidation_low"`
	ConsolidationHigh     float64    `json:"consolidation_high"`
	ConsolidationAvgVol   float64    `json:"consolidation_avg_vol"`
	VolumeTrend           float64    `json:"volume_trend"`
	AverageVolumeAtSpike  float64    `json:"average_volume_at_spike"`
	ReferenceDetectedAt   *time.Time `json:"reference_detected_at,omitempty"`
}

// PatternStatePersister defines the interface for persisting pattern states to a database.
// This interface is implemented by the database.Repository to avoid circular imports.
type PatternStatePersister interface {
	// SavePatternStateToDB saves a pattern state (upsert by user_id, symbol, mode, timeframe).
	SavePatternStateToDB(ctx context.Context, state *PersistedPatternState) error
	// GetPatternStatesFromDB retrieves all active pattern states for a user.
	GetPatternStatesFromDB(ctx context.Context, userID string) ([]PersistedPatternState, error)
	// DeletePatternStateFromDB removes a specific pattern state.
	DeletePatternStateFromDB(ctx context.Context, userID, symbol, mode, timeframe string) error
	// DeleteAllPatternStatesFromDB removes all pattern states for a user.
	DeleteAllPatternStatesFromDB(ctx context.Context, userID string) error
	// DeleteStalePatternStatesFromDB removes all non-position_running pattern states for a user.
	DeleteStalePatternStatesFromDB(ctx context.Context, userID string) error
}

// RealtimePatternMatcher handles real-time pattern evaluation on candle close events.
type RealtimePatternMatcher struct {
	// Pattern matcher for Volume Imbalance strategy
	patternMatcher *VolumeImbalancePatternMatcher

	// Callback for pattern state changes
	onPatternUpdate PatternUpdateCallback

	// Callback for real-time volume progress updates
	onVolumeProgress VolumeProgressCallback

	// Callback for immediate breakout order execution
	onBreakout BreakoutCallback

	// Capacity checker - called before triggering breakout to check if entry is allowed
	// This enables proactive capacity management: when at limit, breakouts are skipped
	capacityChecker CapacityChecker

	// User identification for targeted broadcasts
	userID string

	// Configuration
	defaultMode string
	riskReward  float64 // Default R:R ratio for entry level calculations

	// State tracking - last known states for change detection
	lastStates map[string]*PatternProgress // key: symbol:mode:timeframe

	// Volume progress storage - latest volume progress per symbol
	volumeProgress map[string]*VolumeProgress // key: symbol:timeframe

	// Database persistence for pattern state recovery after restart
	persister PatternStatePersister

	// Suppressed symbols - symbols with active positions that should NOT have new patterns created.
	// Key: "symbol:mode:timeframe", set when fill completes, removed when position closes.
	suppressedSymbols map[string]bool

	mu sync.RWMutex
}

// RealtimePatternMatcherConfig holds configuration for the realtime matcher.
type RealtimePatternMatcherConfig struct {
	// Default trading mode if not specified
	DefaultMode string

	// Default risk/reward ratio for entry level calculations
	RiskRewardRatio float64
}

// DefaultRealtimePatternMatcherConfig returns the default configuration.
func DefaultRealtimePatternMatcherConfig() *RealtimePatternMatcherConfig {
	return &RealtimePatternMatcherConfig{
		DefaultMode:     "scalp",
		RiskRewardRatio: 4.0, // 4:1 R:R per strategy requirements
	}
}

// NewRealtimePatternMatcher creates a new real-time pattern matcher.
func NewRealtimePatternMatcher(
	patternMatcher *VolumeImbalancePatternMatcher,
	config *RealtimePatternMatcherConfig,
) *RealtimePatternMatcher {
	if config == nil {
		config = DefaultRealtimePatternMatcherConfig()
	}

	return &RealtimePatternMatcher{
		patternMatcher:    patternMatcher,
		defaultMode:       config.DefaultMode,
		riskReward:        config.RiskRewardRatio,
		lastStates:        make(map[string]*PatternProgress),
		volumeProgress:    make(map[string]*VolumeProgress),
		suppressedSymbols: make(map[string]bool),
	}
}

// SetPatternUpdateCallback sets the callback for pattern state changes.
func (r *RealtimePatternMatcher) SetPatternUpdateCallback(callback PatternUpdateCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onPatternUpdate = callback
}

// SetUserID sets the user ID for this matcher (used in pattern update broadcasts).
func (r *RealtimePatternMatcher) SetUserID(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userID = userID
}

// SetVolumeProgressCallback sets the callback for real-time volume progress updates.
func (r *RealtimePatternMatcher) SetVolumeProgressCallback(callback VolumeProgressCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onVolumeProgress = callback
}

// SetBreakoutCallback sets the callback for immediate breakout order execution.
// This callback is triggered the moment price breaks out, enabling instant order placement.
func (r *RealtimePatternMatcher) SetBreakoutCallback(callback BreakoutCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onBreakout = callback
}

// SetCapacityChecker sets the capacity checker function.
// This function is called before triggering a breakout to check if new entries are allowed.
// If the checker returns false (at capacity), the breakout is logged but not executed.
// This enables proactive capacity management - pattern matching continues for UI display,
// but no orders are placed when at max_concurrent_trades limit.
func (r *RealtimePatternMatcher) SetCapacityChecker(checker CapacityChecker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.capacityChecker = checker
	log.Printf("[REALTIME-PATTERN] Capacity checker registered for proactive limit enforcement")
}

// SetPersister sets the database persister for saving/restoring pattern states across restarts.
// This should be called during initialization before any pattern processing begins.
func (r *RealtimePatternMatcher) SetPersister(persister PatternStatePersister) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.persister = persister
	log.Printf("[REALTIME-PATTERN] Pattern state persister registered for DB persistence")
}

// RestorePatternStates loads persisted pattern states from the database and restores them
// into the in-memory pattern matcher. This should be called during initialization after
// the persister has been set and the user ID is known.
func (r *RealtimePatternMatcher) RestorePatternStates() {
	r.mu.RLock()
	persister := r.persister
	userID := r.userID
	r.mu.RUnlock()

	if persister == nil || userID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	states, err := persister.GetPatternStatesFromDB(ctx, userID)
	if err != nil {
		log.Printf("[REALTIME-PATTERN] Failed to restore pattern states from DB: %v", err)
		return
	}

	if len(states) == 0 {
		log.Printf("[REALTIME-PATTERN] No persisted pattern states to restore for user %s", userID)
		return
	}

	restored := 0
	for _, ps := range states {
		if err := r.restoreSinglePattern(ps); err != nil {
			log.Printf("[REALTIME-PATTERN] Failed to restore pattern %s:%s:%s: %v",
				ps.Symbol, ps.Mode, ps.Timeframe, err)
			continue
		}
		restored++
	}

	log.Printf("[REALTIME-PATTERN] Restored %d/%d pattern states from DB for user %s",
		restored, len(states), userID)

	// After restoring, immediately broadcast all restored pattern states to UI
	// This ensures Step 2 data is visible without waiting for next candle close
	r.mu.RLock()
	callback := r.onPatternUpdate
	userID = r.userID
	r.mu.RUnlock()

	if callback != nil && restored > 0 {
		for _, ps := range states {
			progress := r.patternMatcher.GetPattern(ps.Symbol, ps.Mode, ps.Timeframe)
			if progress == nil {
				continue
			}
			update := PatternUpdate{
				UserID:      userID,
				Symbol:      ps.Symbol,
				Timeframe:   ps.Timeframe,
				Mode:        ps.Mode,
				Strategy:    ps.Strategy,
				SubStrategy: ps.SubStrategy,
				CurrentStep: progress.CurrentStep,
				TotalSteps:  progress.TotalSteps,
				Status:      progress.Status,
				StepDetails: progress.StepDetails,
				LookingFor:  r.getLookingForDirection(),
				UpdatedAt:   time.Now(),
			}
			// Include reference candle and entry levels if available
			state := r.getPatternState(ps.Symbol, ps.Mode, ps.Timeframe)
			if state != nil {
				update.Direction = state.Direction
				if state.ReferenceCandle != nil {
					volumeMultiplier := 0.0
					if state.AverageVolumeAtSpike > 0 {
						volumeMultiplier = state.ReferenceCandle.Volume / state.AverageVolumeAtSpike
					}
					update.ReferenceCandle = &ReferenceCandle{
						OpenTime:         state.ReferenceCandle.OpenTime,
						CloseTime:        state.ReferenceCandle.Time,
						Open:             state.ReferenceCandle.Open,
						High:             state.ReferenceCandle.High,
						Low:              state.ReferenceCandle.Low,
						Close:            state.ReferenceCandle.Close,
						Volume:           state.ReferenceCandle.Volume,
						VolumeMultiplier: volumeMultiplier,
					}
					update.EntryLevels = r.calculateEntryLevels(state, state.ReferenceCandle.Close)
				}
				r.addTimingFields(&update, state)
			}
			callback(update)
		}
		log.Printf("[REALTIME-PATTERN] Broadcast %d restored pattern states to UI", restored)
	}
}

// restoreSinglePattern restores a single pattern state from a persisted record.
func (r *RealtimePatternMatcher) restoreSinglePattern(ps PersistedPatternState) error {
	if r.patternMatcher == nil {
		return fmt.Errorf("pattern matcher not initialized")
	}

	patternKey := fmt.Sprintf("%s:%s:%s", ps.Symbol, ps.Mode, ps.Timeframe)

	// Deserialize reference candle
	var refCandle *Candle
	if len(ps.ReferenceCandle) > 0 {
		refCandle = &Candle{}
		if err := json.Unmarshal(ps.ReferenceCandle, refCandle); err != nil {
			return fmt.Errorf("failed to unmarshal reference candle: %w", err)
		}
	}

	// Deserialize consolidation data
	var consolidation ConsolidationSnapshot
	if len(ps.ConsolidationData) > 0 {
		if err := json.Unmarshal(ps.ConsolidationData, &consolidation); err != nil {
			return fmt.Errorf("failed to unmarshal consolidation data: %w", err)
		}
	}

	// Reconstruct PatternState
	state := &PatternState{
		ReferenceCandle:      refCandle,
		AverageVolumeAtSpike: consolidation.AverageVolumeAtSpike,
		ConsolidationCandles: consolidation.ConsolidationCandles,
		ConsolidationLow:     consolidation.ConsolidationLow,
		ConsolidationHigh:    consolidation.ConsolidationHigh,
		ConsolidationAvgVol:  consolidation.ConsolidationAvgVol,
		VolumeTrend:          consolidation.VolumeTrend,
		Direction:            ps.Direction,
	}
	// Restore ReferenceDetectedAt from persisted consolidation data
	if consolidation.ReferenceDetectedAt != nil {
		state.ReferenceDetectedAt = *consolidation.ReferenceDetectedAt
	}

	// Restore pattern data (breakout candle, entry price, filling info)
	if len(ps.PatternData) > 0 {
		var patternData map[string]interface{}
		if err := json.Unmarshal(ps.PatternData, &patternData); err == nil {
			if breakoutJSON, ok := patternData["breakout_candle"]; ok {
				breakoutBytes, _ := json.Marshal(breakoutJSON)
				var candle Candle
				if err := json.Unmarshal(breakoutBytes, &candle); err == nil {
					state.BreakoutCandle = &candle
				}
			}
			if v, ok := patternData["entry_price"].(float64); ok {
				state.EntryPrice = v
			}
			if v, ok := patternData["breakout_volume_multiplier"].(float64); ok {
				state.BreakoutVolumeMultiplier = v
			}
			if readyAtStr, ok := patternData["ready_at"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, readyAtStr); err == nil {
					state.ReadyAt = t
				}
			}
			if v, ok := patternData["filling_order_price"].(float64); ok {
				state.FillingOrderPrice = v
			}
			if v, ok := patternData["filling_order_quantity_usd"].(float64); ok {
				state.FillingOrderQuantityUSD = v
			}
			if v, ok := patternData["filling_timeout_total"].(float64); ok {
				state.FillingTimeoutTotal = int(v)
			}
		}
	}

	// Reconstruct PatternProgress
	totalSteps := 2
	progress := NewPatternProgress(ps.Symbol, ps.Strategy, ps.SubStrategy, ps.Mode, ps.Timeframe, totalSteps)
	progress.CurrentStep = ps.CurrentStep
	progress.Status = PatternStatus(ps.Status)
	if ps.StartedAt != nil {
		progress.StartedAt = *ps.StartedAt
	}
	progress.UpdatedAt = ps.UpdatedAt
	if ps.ExpiresAt != nil {
		progress.ExpiresAt = *ps.ExpiresAt
	}

	// Restore step details based on current step
	if ps.CurrentStep >= 2 && refCandle != nil {
		// Step 1 was completed (volume spike detected)
		volMultiplier := 0.0
		if consolidation.AverageVolumeAtSpike > 0 {
			volMultiplier = refCandle.Volume / consolidation.AverageVolumeAtSpike
		}
		directionLabel := "Long Setup"
		if ps.Direction == "short" {
			directionLabel = "Short Setup"
		}
		progress.StepDetails[0] = StepDetail{
			StepNumber: 1,
			Name:       "Volume Spike",
			Completed:  true,
			Progress:   fmt.Sprintf("%.1fx avg (%s) [restored]", volMultiplier, directionLabel),
			Details:    fmt.Sprintf("Reference: H=%.6f L=%.6f", refCandle.High, refCandle.Low),
		}
	}

	// Store in pattern matcher
	r.patternMatcher.mu.Lock()
	r.patternMatcher.patterns[patternKey] = progress
	r.patternMatcher.states[patternKey] = state
	r.patternMatcher.mu.Unlock()

	// Also store in our last states cache
	r.mu.Lock()
	r.lastStates[patternKey] = progress
	r.mu.Unlock()

	log.Printf("[REALTIME-PATTERN] Restored pattern %s step=%d status=%s direction=%s",
		patternKey, ps.CurrentStep, ps.Status, ps.Direction)

	return nil
}

// savePatternStateAsync saves the current pattern state to the database asynchronously.
// This is fire-and-forget to avoid blocking pattern processing.
// Only saves patterns that are in step 2+ (have a reference candle).
func (r *RealtimePatternMatcher) savePatternStateAsync(symbol, mode, timeframe string) {
	r.mu.RLock()
	persister := r.persister
	userID := r.userID
	r.mu.RUnlock()

	if persister == nil || userID == "" {
		return
	}

	// Get pattern progress and state
	progress := r.patternMatcher.GetPattern(symbol, mode, timeframe)
	if progress == nil {
		return
	}

	// Only save patterns at step 2+ (skip "watching" states)
	if progress.CurrentStep < 2 {
		return
	}

	// Get internal state
	state := r.getPatternState(symbol, mode, timeframe)
	if state == nil {
		return
	}

	// Build the persisted state
	ps := r.buildPersistedState(userID, symbol, mode, timeframe, progress, state)
	if ps == nil {
		return
	}

	// Save asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := persister.SavePatternStateToDB(ctx, ps); err != nil {
			log.Printf("[REALTIME-PATTERN] Failed to save pattern state %s:%s:%s to DB: %v",
				symbol, mode, timeframe, err)
		}
	}()
}

// deletePatternStateAsync deletes a pattern state from the database asynchronously.
func (r *RealtimePatternMatcher) deletePatternStateAsync(symbol, mode, timeframe string) {
	r.mu.RLock()
	persister := r.persister
	userID := r.userID
	r.mu.RUnlock()

	if persister == nil || userID == "" {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := persister.DeletePatternStateFromDB(ctx, userID, symbol, mode, timeframe); err != nil {
			log.Printf("[REALTIME-PATTERN] Failed to delete pattern state %s:%s:%s from DB: %v",
				symbol, mode, timeframe, err)
		}
	}()
}

// deleteStalePatternStatesAsync deletes non-position_running pattern states from the database asynchronously.
func (r *RealtimePatternMatcher) deleteStalePatternStatesAsync() {
	r.mu.RLock()
	persister := r.persister
	userID := r.userID
	r.mu.RUnlock()

	if persister == nil || userID == "" {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := persister.DeleteStalePatternStatesFromDB(ctx, userID); err != nil {
			log.Printf("[REALTIME-PATTERN] Error deleting stale pattern states: %v", err)
		}
	}()
}

// deleteAllPatternStatesAsync deletes all pattern states from the database asynchronously.
func (r *RealtimePatternMatcher) deleteAllPatternStatesAsync() {
	r.mu.RLock()
	persister := r.persister
	userID := r.userID
	r.mu.RUnlock()

	if persister == nil || userID == "" {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := persister.DeleteAllPatternStatesFromDB(ctx, userID); err != nil {
			log.Printf("[REALTIME-PATTERN] Failed to delete all pattern states from DB: %v", err)
		}
	}()
}

// buildPersistedState creates a PersistedPatternState from the current pattern state.
func (r *RealtimePatternMatcher) buildPersistedState(
	userID, symbol, mode, timeframe string,
	progress *PatternProgress,
	state *PatternState,
) *PersistedPatternState {
	ps := &PersistedPatternState{
		UserID:      userID,
		Symbol:      symbol,
		Mode:        mode,
		Timeframe:   timeframe,
		Strategy:    progress.Strategy,
		SubStrategy: progress.SubStrategy,
		Status:      string(progress.Status),
		CurrentStep: progress.CurrentStep,
		Direction:   state.Direction,
		UpdatedAt:   time.Now(),
	}

	// Set started_at
	if !progress.StartedAt.IsZero() {
		startedAt := progress.StartedAt
		ps.StartedAt = &startedAt
	}

	// Set expires_at
	if !progress.ExpiresAt.IsZero() {
		expiresAt := progress.ExpiresAt
		ps.ExpiresAt = &expiresAt
	}

	// Serialize reference candle
	if state.ReferenceCandle != nil {
		data, err := json.Marshal(state.ReferenceCandle)
		if err != nil {
			log.Printf("[REALTIME-PATTERN] Failed to marshal reference candle: %v", err)
			return nil
		}
		ps.ReferenceCandle = data
	}

	// Serialize consolidation data
	consolidation := ConsolidationSnapshot{
		ConsolidationCandles: state.ConsolidationCandles,
		ConsolidationLow:     state.ConsolidationLow,
		ConsolidationHigh:    state.ConsolidationHigh,
		ConsolidationAvgVol:  state.ConsolidationAvgVol,
		VolumeTrend:          state.VolumeTrend,
		AverageVolumeAtSpike: state.AverageVolumeAtSpike,
	}
	if !state.ReferenceDetectedAt.IsZero() {
		refDetectedAt := state.ReferenceDetectedAt
		consolidation.ReferenceDetectedAt = &refDetectedAt
	}
	data, err := json.Marshal(consolidation)
	if err != nil {
		log.Printf("[REALTIME-PATTERN] Failed to marshal consolidation data: %v", err)
		return nil
	}
	ps.ConsolidationData = data

	// Serialize entry levels (computed from current state)
	if state.ReferenceCandle != nil {
		entryLevels := r.calculateEntryLevels(state, state.ReferenceCandle.Close)
		if entryLevels != nil {
			entryLevelsJSON, err := json.Marshal(entryLevels)
			if err == nil {
				ps.EntryLevels = entryLevelsJSON
			}
		}
	}

	// Serialize pattern data (breakout candle, entry price, filling info)
	patternExtra := map[string]interface{}{}
	if state.BreakoutCandle != nil {
		patternExtra["breakout_candle"] = state.BreakoutCandle
	}
	if state.EntryPrice > 0 {
		patternExtra["entry_price"] = state.EntryPrice
	}
	if state.BreakoutVolumeMultiplier > 0 {
		patternExtra["breakout_volume_multiplier"] = state.BreakoutVolumeMultiplier
	}
	if !state.ReadyAt.IsZero() {
		patternExtra["ready_at"] = state.ReadyAt
	}
	if state.FillingOrderPrice > 0 {
		patternExtra["filling_order_price"] = state.FillingOrderPrice
		patternExtra["filling_order_quantity_usd"] = state.FillingOrderQuantityUSD
		patternExtra["filling_timeout_total"] = state.FillingTimeoutTotal
	}
	if len(patternExtra) > 0 {
		patternDataJSON, err := json.Marshal(patternExtra)
		if err == nil {
			ps.PatternData = patternDataJSON
		}
	}

	return ps
}

// OnVolumeProgress is called on every tick with volume progress data from CoinProfiler.
// It stores the latest volume progress and broadcasts it via WebSocket in real-time.
// CRITICAL: Preserves existing pattern state (Step 2+) to prevent UI flicker.
func (r *RealtimePatternMatcher) OnVolumeProgress(data coinprofiler.VolumeProgressData) {
	// Skip suppressed symbols (active position exists)
	suppKey := fmt.Sprintf("%s:%s:%s", data.Symbol, r.defaultMode, data.Timeframe)
	r.mu.RLock()
	suppressed := r.suppressedSymbols[suppKey]
	r.mu.RUnlock()
	if suppressed {
		return
	}

	// Convert CoinProfiler data to our VolumeProgress type
	progress := &VolumeProgress{
		CurrentVolume:      data.CurrentVolume,
		AverageVolume:      data.AverageVolume,
		CurrentRatio:       data.CurrentRatio,
		RequiredRatio:      data.RequiredRatio,
		ProgressPercent:    data.ProgressPercent,
		CandleDirection:    data.CandleDirection,
		IsApproachingSpike: data.IsApproachingSpike,
		TimeRemainingMs:    data.TimeRemainingMs,
		LookbackCandles:    data.LookbackCandles,
		CurrentPrice:       data.CurrentPrice,
	}

	// Store latest volume progress
	key := fmt.Sprintf("%s:%s", data.Symbol, data.Timeframe)
	r.mu.Lock()
	r.volumeProgress[key] = progress
	callback := r.onVolumeProgress
	userID := r.userID
	r.mu.Unlock()

	// Broadcast volume progress update
	if callback != nil {
		callback(*progress)
	}

	// Also broadcast as a pattern update with volume progress attached
	// This keeps the frontend in sync with real-time volume data
	r.mu.RLock()
	patternCallback := r.onPatternUpdate
	r.mu.RUnlock()

	if patternCallback != nil {
		// CRITICAL FIX: Check existing pattern state before broadcasting
		// If the coin is already in Step 2+ (has reference_candle), preserve that state
		// to prevent UI flickering from "Accumulating" back to "Watching"
		mode := r.defaultMode
		existingProgress := r.patternMatcher.GetPattern(data.Symbol, mode, data.Timeframe)
		existingState := r.getPatternState(data.Symbol, mode, data.Timeframe)

		// Default values for Step 1 (no pattern yet)
		currentStep := 1
		totalSteps := 2
		status := PatternStatusWatching
		var stepDetails []StepDetail
		var referenceCandle *ReferenceCandle
		var entryLevels *EntryLevels
		direction := ""

		// If we have existing pattern progress in Step 2+, preserve it
		if existingProgress != nil && existingProgress.CurrentStep >= 2 {
			currentStep = existingProgress.CurrentStep
			totalSteps = existingProgress.TotalSteps
			status = existingProgress.Status
			stepDetails = existingProgress.StepDetails

			// Get state for reference candle and direction
			if existingState != nil {
				direction = existingState.Direction

				// Include reference candle for Step 2+ context
				if existingState.ReferenceCandle != nil {
					volumeMultiplier := 0.0
					if existingState.AverageVolumeAtSpike > 0 {
						volumeMultiplier = existingState.ReferenceCandle.Volume / existingState.AverageVolumeAtSpike
					}
					referenceCandle = &ReferenceCandle{
						OpenTime:         existingState.ReferenceCandle.OpenTime,
						CloseTime:        existingState.ReferenceCandle.Time,
						Open:             existingState.ReferenceCandle.Open,
						High:             existingState.ReferenceCandle.High,
						Low:              existingState.ReferenceCandle.Low,
						Close:            existingState.ReferenceCandle.Close,
						Volume:           existingState.ReferenceCandle.Volume,
						VolumeMultiplier: volumeMultiplier,
					}

					// Calculate entry levels with current price
					entryLevels = r.calculateEntryLevels(existingState, data.CurrentPrice)
				}
			}
		}

		update := PatternUpdate{
			UserID:          userID,
			Symbol:          data.Symbol,
			Timeframe:       data.Timeframe,
			Mode:            mode,
			Strategy:        "volume_imbalance",
			SubStrategy:     "ravindra_volume_imbalance",
			CurrentStep:     currentStep,
			TotalSteps:      totalSteps,
			Status:          status,
			StepDetails:     stepDetails,
			VolumeProgress:  progress,
			ReferenceCandle: referenceCandle,
			EntryLevels:     entryLevels,
			Direction:       direction,
			CurrentPrice:    data.CurrentPrice,
			LookingFor:      r.getLookingForDirection(),
			UpdatedAt:       time.Now(),
		}

		// Add timing fields for Step 2+ patterns
		if existingState != nil {
			r.addTimingFields(&update, existingState)
		}

		patternCallback(update)
	}
}

// GetVolumeProgress returns the latest volume progress for a symbol/timeframe.
func (r *RealtimePatternMatcher) GetVolumeProgress(symbol, timeframe string) *VolumeProgress {
	key := fmt.Sprintf("%s:%s", symbol, timeframe)
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.volumeProgress[key]
}

// GetPatternMatcher returns the underlying VolumeImbalancePatternMatcher.
// This allows the Entry Decision API to access the same pattern matcher
// that is connected to the CoinProfiler for real-time updates.
func (r *RealtimePatternMatcher) GetPatternMatcher() *VolumeImbalancePatternMatcher {
	return r.patternMatcher
}

// ReloadPatternMatcherConfig updates the underlying pattern matcher's configuration.
// This allows dynamic configuration updates without resetting pattern progress.
func (r *RealtimePatternMatcher) ReloadPatternMatcherConfig(newConfig *PatternMatcherConfig) {
	if r.patternMatcher == nil {
		log.Printf("[REALTIME-PATTERN] ReloadPatternMatcherConfig: no pattern matcher, ignoring")
		return
	}
	r.patternMatcher.ReloadConfig(newConfig)
}

// ClearAllPatterns clears all pattern state from both the underlying pattern matcher
// and the realtime matcher's cached states. This should be called when:
// - The CoinProfiler restarts (e.g., browser refresh triggers stop/start)
// - Subscriptions are refreshed
// - A fresh pattern detection session is needed
//
// This prevents "pattern timeout/expired" issues after profiler restart because
// old patterns with stale ExpiresAt timestamps are cleared before new candle data arrives.
func (r *RealtimePatternMatcher) ClearAllPatterns() {
	// Clear underlying pattern matcher's patterns
	if r.patternMatcher != nil {
		r.patternMatcher.ClearAllPatterns()
	}

	// Clear our cached states and suppression list
	r.mu.Lock()
	r.lastStates = make(map[string]*PatternProgress)
	r.volumeProgress = make(map[string]*VolumeProgress)
	r.suppressedSymbols = make(map[string]bool)
	r.mu.Unlock()

	// Delete all persisted pattern states from DB (async)
	r.deleteAllPatternStatesAsync()

	log.Printf("[REALTIME-PATTERN] Cleared all pattern states for fresh start")
}

// ClearStalePatterns clears expired/stale patterns but preserves position_running patterns.
// Used on coin profiler start to avoid destroying active position tracking.
func (r *RealtimePatternMatcher) ClearStalePatterns() {
	// Clear stale patterns in underlying matcher (preserves position_running)
	if r.patternMatcher != nil {
		r.patternMatcher.ClearStalePatterns()
	}

	// Clear cached states and volume progress for non-position patterns only
	r.mu.Lock()
	for key, state := range r.lastStates {
		if state.Status != PatternStatusPositionRunning {
			delete(r.lastStates, key)
			delete(r.volumeProgress, key)
		}
	}
	// Keep suppressedSymbols intact - position symbols should stay suppressed
	r.mu.Unlock()

	// Delete non-position pattern states from DB
	r.deleteStalePatternStatesAsync()

	log.Printf("[REALTIME-PATTERN] Cleared stale patterns (preserved position_running)")
}

// ClearPatternForSymbol clears the pattern for a specific symbol when a position is opened.
// This notifies the system to stop looking for new entry patterns on this symbol until
// the position is closed. It also suppresses the symbol so OnCandleClose won't re-create
// the pattern while a position exists.
func (r *RealtimePatternMatcher) ClearPatternForSymbol(symbol, mode, timeframe string) {
	if r.patternMatcher == nil {
		return
	}

	// Clear from underlying pattern matcher
	r.patternMatcher.ClearPatternForSymbol(symbol, mode, timeframe)

	// Clear from our cached states and suppress re-creation
	r.mu.Lock()
	stateKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
	delete(r.lastStates, stateKey)
	volKey := fmt.Sprintf("%s:%s", symbol, timeframe)
	delete(r.volumeProgress, volKey)
	r.suppressedSymbols[stateKey] = true
	r.mu.Unlock()

	// Delete persisted pattern state from DB (async)
	r.deletePatternStateAsync(symbol, mode, timeframe)

	log.Printf("[REALTIME-PATTERN] Cleared and suppressed pattern for %s:%s:%s (position opened)", symbol, mode, timeframe)
}

// SetPatternPositionRunning transitions a pattern to "position_running" status and broadcasts Step 4.
// This is called when an entry order fills successfully. Instead of clearing the pattern (which would
// make the coin disappear from the UI), this keeps it visible as Step 4 with position_running status.
// The symbol is also suppressed to prevent new pattern detection while the position is active.
func (r *RealtimePatternMatcher) SetPatternPositionRunning(symbol, mode, timeframe string) {
	if r.patternMatcher == nil {
		return
	}

	// Set position_running status on the underlying pattern matcher
	r.patternMatcher.SetPatternPositionRunning(symbol, mode, timeframe)

	// Suppress the symbol to prevent new pattern detection
	r.mu.Lock()
	stateKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
	r.suppressedSymbols[stateKey] = true
	r.mu.Unlock()

	// Save the position_running state to DB for persistence across restarts
	r.savePatternStateAsync(symbol, mode, timeframe)

	// Broadcast Step 4 update to UI
	r.mu.RLock()
	callback := r.onPatternUpdate
	userID := r.userID
	r.mu.RUnlock()

	if callback != nil {
		update := PatternUpdate{
			UserID:      userID,
			Symbol:      symbol,
			Timeframe:   timeframe,
			Mode:        mode,
			Strategy:    "volume_imbalance",
			SubStrategy: "ravindra_volume_imbalance",
			CurrentStep: 4,
			TotalSteps:  4,
			Status:      PatternStatusPositionRunning,
			UpdatedAt:   time.Now(),
		}
		callback(update)
		log.Printf("[REALTIME-PATTERN] Set position_running and broadcast Step 4 for %s:%s:%s (callback wired)", symbol, mode, timeframe)
	} else {
		log.Printf("[REALTIME-PATTERN] Set position_running for %s:%s:%s (NO callback - broadcast skipped, will appear in periodic broadcast)", symbol, mode, timeframe)
	}
}

// ResetPatternForSymbol clears the pattern for a specific symbol WITHOUT suppressing it.
// This should be called when an entry order fails (timeout, rejected, etc.) to allow
// the pattern matcher to immediately start looking for new patterns on this symbol.
// Unlike ClearPatternForSymbol, this does NOT add the symbol to suppressedSymbols.
func (r *RealtimePatternMatcher) ResetPatternForSymbol(symbol, mode, timeframe string) {
	if r.patternMatcher == nil {
		return
	}

	// Clear from underlying pattern matcher
	r.patternMatcher.ClearPatternForSymbol(symbol, mode, timeframe)

	// Clear from our cached states but do NOT suppress re-creation
	r.mu.Lock()
	stateKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
	delete(r.lastStates, stateKey)
	volKey := fmt.Sprintf("%s:%s", symbol, timeframe)
	delete(r.volumeProgress, volKey)
	// Explicitly remove any existing suppression (in case it was suppressed)
	delete(r.suppressedSymbols, stateKey)
	r.mu.Unlock()

	// Delete persisted pattern state from DB (async)
	r.deletePatternStateAsync(symbol, mode, timeframe)

	log.Printf("[REALTIME-PATTERN] Reset pattern for %s:%s:%s (entry failed - ready for new detection)", symbol, mode, timeframe)

	// Broadcast a "watching" status so the frontend resets to Step 1
	r.mu.RLock()
	callback := r.onPatternUpdate
	userID := r.userID
	r.mu.RUnlock()

	if callback != nil {
		update := PatternUpdate{
			UserID:      userID,
			Symbol:      symbol,
			Timeframe:   timeframe,
			Mode:        mode,
			Strategy:    "volume_imbalance",
			SubStrategy: "ravindra_volume_imbalance",
			CurrentStep: 1,
			TotalSteps:  2,
			Status:      PatternStatusWatching,
			LookingFor:  r.getLookingForDirection(),
			UpdatedAt:   time.Now(),
		}
		callback(update)
	}
}

// UnsuppressSymbol removes the suppression for a symbol, allowing new pattern detection.
// This should be called when a position is closed for a symbol.
func (r *RealtimePatternMatcher) UnsuppressSymbol(symbol, mode, timeframe string) {
	r.mu.Lock()
	stateKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
	delete(r.suppressedSymbols, stateKey)
	r.mu.Unlock()

	log.Printf("[REALTIME-PATTERN] Unsuppressed pattern for %s:%s:%s (position closed)", symbol, mode, timeframe)
}

// SetPatternFillingStatus transitions a pattern to "filling" status and broadcasts Step 3 UI data.
// Called when a LIMIT order has been placed and we're waiting for fill.
func (r *RealtimePatternMatcher) SetPatternFillingStatus(symbol, mode, timeframe string, orderPrice, orderQuantityUSD float64, fillTimeoutSecs int) {
	if r.patternMatcher == nil {
		return
	}

	// Update pattern status to filling and store order data on state for fill progress broadcasts
	r.patternMatcher.mu.Lock()
	patternKey := r.patternMatcher.patternKey(symbol, mode, timeframe)
	progress := r.patternMatcher.patterns[patternKey]
	if progress != nil {
		progress.SetStatus(PatternStatusFilling)
		progress.CurrentStep = 3
		progress.UpdatedAt = time.Now()
		// Update Step 3 details
		if len(progress.StepDetails) >= 3 {
			progress.StepDetails[2] = StepDetail{
				StepNumber: 3,
				Name:       "Order Filling",
				Completed:  false,
				Progress:   "FILLING",
				Details:    fmt.Sprintf("LIMIT order @ %.6f, waiting for fill (%ds timeout)", orderPrice, fillTimeoutSecs),
			}
		}
	}
	// Store filling data on PatternState so UpdateFillProgress can read it
	state := r.patternMatcher.states[patternKey]
	if state != nil {
		state.FillingOrderPrice = orderPrice
		state.FillingOrderQuantityUSD = orderQuantityUSD
		state.FillingTimeoutTotal = fillTimeoutSecs
	}
	r.patternMatcher.mu.Unlock()

	if progress == nil {
		log.Printf("[REALTIME-PATTERN] Cannot set filling status - no pattern found for %s:%s:%s", symbol, mode, timeframe)
		return
	}

	log.Printf("[REALTIME-PATTERN] Set filling status for %s:%s:%s: price=%.6f, qty=$%.2f, timeout=%ds",
		symbol, mode, timeframe, orderPrice, orderQuantityUSD, fillTimeoutSecs)

	// Broadcast Step 3 update to UI
	r.mu.RLock()
	callback := r.onPatternUpdate
	userID := r.userID
	r.mu.RUnlock()

	if callback != nil {
		// Get reference candle and entry levels from internal pattern state for context
		var refCandle *ReferenceCandle
		var entryLevels *EntryLevels
		var direction string

		state := r.getPatternState(symbol, mode, timeframe)
		if state != nil {
			direction = state.Direction

			if state.ReferenceCandle != nil {
				volumeMultiplier := 0.0
				if state.AverageVolumeAtSpike > 0 {
					volumeMultiplier = state.ReferenceCandle.Volume / state.AverageVolumeAtSpike
				}
				refCandle = &ReferenceCandle{
					OpenTime:         state.ReferenceCandle.OpenTime,
					CloseTime:        state.ReferenceCandle.Time,
					Open:             state.ReferenceCandle.Open,
					High:             state.ReferenceCandle.High,
					Low:              state.ReferenceCandle.Low,
					Close:            state.ReferenceCandle.Close,
					Volume:           state.ReferenceCandle.Volume,
					VolumeMultiplier: volumeMultiplier,
				}

				entryLevels = r.calculateEntryLevels(state, orderPrice)
			}
		}

		update := PatternUpdate{
			UserID:             userID,
			Symbol:             symbol,
			Timeframe:          timeframe,
			Mode:               mode,
			Strategy:           "breakout",
			SubStrategy:        "ravindra_volume_imbalance",
			CurrentStep:        3,
			TotalSteps:         3,
			Status:             PatternStatusFilling,
			StepDetails:        progress.StepDetails,
			EntryLevels:        entryLevels,
			ReferenceCandle:    refCandle,
			Direction:          direction,
			OrderPrice:         orderPrice,
			OrderQuantityUSD:   orderQuantityUSD,
			FillTimeoutSeconds: fillTimeoutSecs,
			FillTimeoutTotal:   fillTimeoutSecs,
			UpdatedAt:          time.Now(),
		}

		// Include entry candle (breakout candle) and timing data
		if state != nil {
			update.EntryCandle = r.buildEntryCandle(state)
			r.addTimingFields(&update, state)
		}

		callback(update)
	}
}

// UpdateFillProgress broadcasts updated fill timeout remaining for the Step 3 UI countdown.
// Called periodically (every 2 seconds) by the chain entry runner during waitForLimitFill.
func (r *RealtimePatternMatcher) UpdateFillProgress(symbol, mode, timeframe string, remainingSecs int) {
	r.mu.RLock()
	callback := r.onPatternUpdate
	userID := r.userID
	r.mu.RUnlock()

	if callback == nil {
		return
	}

	if r.patternMatcher == nil {
		return
	}

	// Read pattern progress and state under lock
	r.patternMatcher.mu.RLock()
	patternKey := r.patternMatcher.patternKey(symbol, mode, timeframe)
	progress := r.patternMatcher.patterns[patternKey]
	state := r.patternMatcher.states[patternKey]
	r.patternMatcher.mu.RUnlock()

	if progress == nil || progress.Status != PatternStatusFilling {
		return // Only broadcast during filling state
	}

	// Read filling data from state
	var refCandle *ReferenceCandle
	var entryLevels *EntryLevels
	var direction string
	var orderPrice float64
	var orderQtyUSD float64
	var fillTimeoutTotal int

	if state != nil {
		direction = state.Direction
		orderPrice = state.FillingOrderPrice
		orderQtyUSD = state.FillingOrderQuantityUSD
		fillTimeoutTotal = state.FillingTimeoutTotal

		if state.ReferenceCandle != nil {
			volumeMultiplier := 0.0
			if state.AverageVolumeAtSpike > 0 {
				volumeMultiplier = state.ReferenceCandle.Volume / state.AverageVolumeAtSpike
			}
			refCandle = &ReferenceCandle{
				OpenTime:         state.ReferenceCandle.OpenTime,
				CloseTime:        state.ReferenceCandle.Time,
				Open:             state.ReferenceCandle.Open,
				High:             state.ReferenceCandle.High,
				Low:              state.ReferenceCandle.Low,
				Close:            state.ReferenceCandle.Close,
				Volume:           state.ReferenceCandle.Volume,
				VolumeMultiplier: volumeMultiplier,
			}

			entryLevels = r.calculateEntryLevels(state, orderPrice)
		}
	}

	update := PatternUpdate{
		UserID:             userID,
		Symbol:             symbol,
		Timeframe:          timeframe,
		Mode:               mode,
		Strategy:           "breakout",
		SubStrategy:        "ravindra_volume_imbalance",
		CurrentStep:        3,
		TotalSteps:         3,
		Status:             PatternStatusFilling,
		StepDetails:        progress.StepDetails,
		EntryLevels:        entryLevels,
		ReferenceCandle:    refCandle,
		Direction:          direction,
		OrderPrice:         orderPrice,
		OrderQuantityUSD:   orderQtyUSD,
		FillTimeoutSeconds: remainingSecs,
		FillTimeoutTotal:   fillTimeoutTotal,
		UpdatedAt:          time.Now(),
	}

	// Include entry candle (breakout candle) and timing data
	if state != nil {
		update.EntryCandle = r.buildEntryCandle(state)
		r.addTimingFields(&update, state)
	}

	callback(update)
}

// OnCandleClose is called when a candle closes. This is the main entry point
// for real-time pattern evaluation.
// It evaluates patterns for the given symbol/timeframe and ALWAYS broadcasts
// the current state (so frontend knows the system is alive and when next update will occur).
func (r *RealtimePatternMatcher) OnCandleClose(symbol, timeframe string, candles []coinprofiler.HistoricalCandle) {
	if r.patternMatcher == nil {
		return
	}

	r.mu.RLock()
	callback := r.onPatternUpdate
	userID := r.userID
	r.mu.RUnlock()

	// Calculate next candle close time for countdown display
	nextCandleClose := calculateNextCandleClose(timeframe)

	if len(candles) < 25 {
		log.Printf("[REALTIME-PATTERN] %s:%s - Not enough candles (%d < 25), skipping", symbol, timeframe, len(candles))
		// Still broadcast a status update so frontend knows we're alive
		if callback != nil {
			callback(PatternUpdate{
				UserID:          userID,
				Symbol:          symbol,
				Timeframe:       timeframe,
				Mode:            r.defaultMode,
				Strategy:        "volume_imbalance",
				SubStrategy:     "ravindra_volume_imbalance",
				Status:          PatternStatusWatching,
				LookingFor:      r.getLookingForDirection(),
				NextCandleClose: nextCandleClose,
				LastEvaluatedAt: time.Now(),
				UpdatedAt:       time.Now(),
			})
		}
		return
	}

	// Check if this symbol is suppressed (has active position, pattern should not be re-created)
	mode := r.defaultMode
	stateKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
	r.mu.RLock()
	suppressed := r.suppressedSymbols[stateKey]
	r.mu.RUnlock()

	if suppressed {
		log.Printf("[REALTIME-PATTERN] %s:%s - Suppressed (active position), skipping pattern evaluation", symbol, timeframe)
		return
	}

	// Convert historical candles to pattern matcher format
	matcherCandles := make([]Candle, len(candles))
	for i, hc := range candles {
		matcherCandles[i] = Candle{
			OpenTime:       hc.OpenTime,  // Candle open time (use to identify the candle)
			Time:           hc.CloseTime, // Candle close time (backward compat)
			Open:           hc.Open,
			High:           hc.High,
			Low:            hc.Low,
			Close:          hc.Close,
			Volume:         hc.Volume,
			TakerBuyVolume: hc.TakerBuyVol,
		}
	}

	// Evaluate pattern for default mode
	coinMatch := r.patternMatcher.MatchPattern(symbol, mode, timeframe, matcherCandles)

	// Get full pattern progress (even if coinMatch is nil, we want current state)
	progress := r.patternMatcher.GetPattern(symbol, mode, timeframe)

	if coinMatch != nil {
		log.Printf("[REALTIME-PATTERN] %s:%s - Pattern match found! Step %d, Status: %s",
			symbol, timeframe, coinMatch.Step, coinMatch.Status)
	}

	// Check if state has changed (for logging purposes)
	// stateKey already defined above for suppression check
	stateChanged := r.hasStateChanged(stateKey, progress)

	if stateChanged && progress != nil {
		// Store new state
		r.mu.Lock()
		r.lastStates[stateKey] = progress
		r.mu.Unlock()

		// Persist pattern state to DB (async, fire-and-forget)
		r.savePatternStateAsync(symbol, mode, timeframe)
	}

	// ALWAYS broadcast update (so frontend knows system is alive)
	if callback != nil {
		var update PatternUpdate

		if progress != nil {
			update = r.buildPatternUpdate(symbol, timeframe, progress, matcherCandles)
		} else {
			// No pattern yet, create a basic watching state
			update = PatternUpdate{
				UserID:      userID,
				Symbol:      symbol,
				Timeframe:   timeframe,
				Mode:        mode,
				Strategy:    "volume_imbalance",
				SubStrategy: "ravindra_volume_imbalance",
				CurrentStep: 1,
				TotalSteps:  2,
				Status:      PatternStatusWatching,
				UpdatedAt:   time.Now(),
			}
		}

		// Add countdown timer and evaluation timestamp
		update.NextCandleClose = nextCandleClose
		update.LastEvaluatedAt = time.Now()
		update.LookingFor = r.getLookingForDirection()

		log.Printf("[REALTIME-PATTERN] Broadcasting update for %s:%s status=%s looking_for=%s next_close=%v",
			symbol, timeframe, update.Status, update.LookingFor, update.NextCandleClose)
		callback(update)
	} else {
		log.Printf("[REALTIME-PATTERN] WARNING: No callback set for %s:%s, cannot broadcast", symbol, timeframe)
	}
}

// calculateNextCandleClose calculates when the next candle will close based on timeframe.
// Aligns to Binance candle boundaries which start at 00:00:00 UTC.
// For 3m candles: closes at :00, :03, :06, :09... :51, :54, :57
func calculateNextCandleClose(timeframe string) time.Time {
	now := time.Now().UTC()

	// Parse timeframe in minutes
	var durationMinutes int
	switch timeframe {
	case "1m":
		durationMinutes = 1
	case "3m":
		durationMinutes = 3
	case "5m":
		durationMinutes = 5
	case "15m":
		durationMinutes = 15
	case "30m":
		durationMinutes = 30
	case "1h":
		durationMinutes = 60
	case "4h":
		durationMinutes = 240
	case "1d":
		durationMinutes = 1440
	default:
		durationMinutes = 3 // Default to 3m
	}

	// Calculate based on current minute alignment (Binance candles align to clock)
	currentMinute := now.Minute()
	currentSecond := now.Second()

	// Find the next candle close minute
	// For 3m candles starting at minute 0: close at 3, 6, 9, 12... 51, 54, 57, 0
	currentCandleStart := currentMinute - (currentMinute % durationMinutes)
	nextCandleClose := currentCandleStart + durationMinutes

	// Handle hour rollover
	hoursToAdd := 0
	if nextCandleClose >= 60 {
		nextCandleClose = nextCandleClose % 60
		hoursToAdd = 1
	}

	// Build the next close time
	nextClose := time.Date(
		now.Year(), now.Month(), now.Day(),
		now.Hour()+hoursToAdd, nextCandleClose, 0, 0,
		time.UTC,
	)

	// If the calculated time is in the past (edge case at second 0), add duration
	if nextClose.Before(now) || (nextClose.Equal(now) && currentSecond == 0) {
		nextClose = nextClose.Add(time.Duration(durationMinutes) * time.Minute)
	}

	return nextClose
}

// getLookingForDirection returns the configured direction setting for display.
func (r *RealtimePatternMatcher) getLookingForDirection() string {
	if r.patternMatcher == nil || r.patternMatcher.config == nil {
		return "long"
	}
	return r.patternMatcher.config.Direction
}

// hasStateChanged checks if the pattern state has changed from the last known state.
func (r *RealtimePatternMatcher) hasStateChanged(stateKey string, current *PatternProgress) bool {
	r.mu.RLock()
	last, exists := r.lastStates[stateKey]
	r.mu.RUnlock()

	if !exists {
		return true // New pattern, always report
	}

	// Check for meaningful changes
	if last.CurrentStep != current.CurrentStep {
		return true
	}
	if last.Status != current.Status {
		return true
	}

	return false
}

// buildEntryCandle creates an EntryCandle from the breakout candle in PatternState.
// Returns nil if no breakout candle is available.
func (r *RealtimePatternMatcher) buildEntryCandle(state *PatternState) *EntryCandle {
	if state == nil || state.BreakoutCandle == nil {
		return nil
	}

	return &EntryCandle{
		OpenTime:         state.BreakoutCandle.OpenTime,
		CloseTime:        state.BreakoutCandle.Time,
		Open:             state.BreakoutCandle.Open,
		High:             state.BreakoutCandle.High,
		Low:              state.BreakoutCandle.Low,
		Close:            state.BreakoutCandle.Close,
		Volume:           state.BreakoutCandle.Volume,
		VolumeMultiplier: state.BreakoutVolumeMultiplier,
		EntryPrice:       state.EntryPrice,
		DetectedAt:       state.ReadyAt,
		Direction:        state.Direction,
	}
}

// addTimingFields populates the timing fields on a PatternUpdate from PatternState.
func (r *RealtimePatternMatcher) addTimingFields(update *PatternUpdate, state *PatternState) {
	if state == nil {
		return
	}
	if !state.ReferenceDetectedAt.IsZero() {
		update.ReferenceDetectedAt = state.ReferenceDetectedAt.UTC().Format(time.RFC3339)
		update.SecondsSinceReference = int(time.Since(state.ReferenceDetectedAt).Seconds())
	}
	if !state.ReadyAt.IsZero() {
		update.BreakoutDetectedAt = state.ReadyAt.UTC().Format(time.RFC3339)
	}
}

// buildPatternUpdate constructs a PatternUpdate from pattern progress.
func (r *RealtimePatternMatcher) buildPatternUpdate(
	symbol, timeframe string,
	progress *PatternProgress,
	candles []Candle,
) PatternUpdate {
	update := PatternUpdate{
		UserID:      r.userID, // Include userID for targeted broadcasts
		Symbol:      symbol,
		Timeframe:   timeframe,
		Mode:        progress.Mode,
		Strategy:    progress.Strategy,
		SubStrategy: progress.SubStrategy,
		CurrentStep: progress.CurrentStep,
		TotalSteps:  progress.TotalSteps,
		Status:      progress.Status,
		StepDetails: progress.StepDetails,
		LookingFor:  r.getLookingForDirection(), // What direction we're configured to find
		UpdatedAt:   time.Now(),
	}

	// Calculate current price and day high/low from candles
	var currentPrice float64
	var dayHigh, dayLow float64
	if len(candles) > 0 {
		currentPrice = candles[len(candles)-1].Close
		update.CurrentPrice = currentPrice

		// Calculate day high/low from recent candles (last 24 for daily context)
		lookback := min(len(candles), 96) // ~24h worth of 15m candles or ~8h of 5m candles
		dayHigh = candles[len(candles)-1].High
		dayLow = candles[len(candles)-1].Low
		for i := len(candles) - lookback; i < len(candles); i++ {
			if candles[i].High > dayHigh {
				dayHigh = candles[i].High
			}
			if candles[i].Low < dayLow {
				dayLow = candles[i].Low
			}
		}
		update.DayHigh = dayHigh
		update.DayLow = dayLow
	}

	// Get pattern state for entry levels and reference candle
	state := r.getPatternState(symbol, progress.Mode, timeframe)
	if state != nil {
		update.Direction = state.Direction

		// Get volume threshold from config
		if r.patternMatcher != nil {
			update.VolumeThreshold = r.patternMatcher.config.MinVolumeSpikeMultiplier
		}

		// Include reference candle data for Stage 2+ context display
		// This shows where the volume spike occurred
		if state.ReferenceCandle != nil && progress.CurrentStep >= 2 {
			// Convert internal Candle type to ReferenceCandle type for the update
			volumeMultiplier := 0.0
			if state.AverageVolumeAtSpike > 0 {
				volumeMultiplier = state.ReferenceCandle.Volume / state.AverageVolumeAtSpike
			}
			update.ReferenceCandle = &ReferenceCandle{
				OpenTime:         state.ReferenceCandle.OpenTime,
				CloseTime:        state.ReferenceCandle.Time, // Time is close time
				Open:             state.ReferenceCandle.Open,
				High:             state.ReferenceCandle.High,
				Low:              state.ReferenceCandle.Low,
				Close:            state.ReferenceCandle.Close,
				Volume:           state.ReferenceCandle.Volume,
				VolumeMultiplier: volumeMultiplier,
			}
		}

		// Calculate entry levels if we have enough state
		if state.ReferenceCandle != nil && currentPrice > 0 {
			update.EntryLevels = r.calculateEntryLevels(state, currentPrice)
		}

		// Include entry candle data (breakout candle) for Step 3+ display
		update.EntryCandle = r.buildEntryCandle(state)

		// Add timing fields
		r.addTimingFields(&update, state)
	}

	// Attach latest volume progress if available
	update.VolumeProgress = r.GetVolumeProgress(symbol, timeframe)

	return update
}

// getPatternState retrieves the internal pattern state from the matcher.
func (r *RealtimePatternMatcher) getPatternState(symbol, mode, timeframe string) *PatternState {
	if r.patternMatcher == nil {
		return nil
	}

	// Access pattern matcher's internal state (thread-safe)
	r.patternMatcher.mu.RLock()
	defer r.patternMatcher.mu.RUnlock()

	key := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
	return r.patternMatcher.states[key]
}

// calculateEntryLevels computes entry, SL, and TP levels based on pattern state.
// Direction-aware: LONG uses reference HIGH as entry, SHORT uses reference LOW.
func (r *RealtimePatternMatcher) calculateEntryLevels(state *PatternState, currentPrice float64) *EntryLevels {
	if state == nil || state.ReferenceCandle == nil {
		return nil
	}

	var entryPrice, stopLoss, risk, takeProfit float64

	if state.Direction == "short" {
		// SHORT: Entry at/below reference candle low, SL above consolidation high
		entryPrice = state.ReferenceCandle.Low

		stopLossBase := state.ConsolidationHigh
		if stopLossBase <= 0 {
			stopLossBase = state.ReferenceCandle.High
		}
		stopLoss = stopLossBase * 1.001 // 0.1% above resistance

		risk = stopLoss - entryPrice
		if risk <= 0 {
			return nil // Invalid risk
		}

		takeProfit = entryPrice - (risk * r.riskReward)
	} else {
		// LONG: Entry at/above reference candle high, SL below consolidation low
		entryPrice = state.ReferenceCandle.High

		stopLossBase := state.ConsolidationLow
		if stopLossBase <= 0 {
			stopLossBase = state.ReferenceCandle.Low
		}
		stopLoss = stopLossBase * 0.999 // 0.1% below support

		risk = entryPrice - stopLoss
		if risk <= 0 {
			return nil // Invalid risk
		}

		takeProfit = entryPrice + (risk * r.riskReward)
	}

	// Calculate percentages (direction-independent using absolute risk)
	riskPercent := (risk / entryPrice) * 100
	rewardPercent := (risk * r.riskReward / entryPrice) * 100

	return &EntryLevels{
		EntryPrice:      entryPrice,
		StopLoss:        stopLoss,
		TakeProfit:      takeProfit,
		RiskPercent:     riskPercent,
		RewardPercent:   rewardPercent,
		RiskRewardRatio: r.riskReward,
		ReferenceHigh:   state.ReferenceCandle.High,
		ReferenceLow:    state.ReferenceCandle.Low,
		CurrentPrice:    currentPrice,
	}
}

// GetAllPatternUpdates returns current state of all tracked patterns as updates.
// Useful for initial page load to populate UI.
func (r *RealtimePatternMatcher) GetAllPatternUpdates() []PatternUpdate {
	if r.patternMatcher == nil {
		return nil
	}

	patterns := r.patternMatcher.GetAllPatterns()
	updates := make([]PatternUpdate, 0, len(patterns))

	for _, progress := range patterns {
		update := PatternUpdate{
			Symbol:      progress.Symbol,
			Timeframe:   progress.Timeframe,
			Mode:        progress.Mode,
			Strategy:    progress.Strategy,
			SubStrategy: progress.SubStrategy,
			CurrentStep: progress.CurrentStep,
			TotalSteps:  progress.TotalSteps,
			Status:      progress.Status,
			StepDetails: progress.StepDetails,
			UpdatedAt:   progress.UpdatedAt,
		}

		// Get state for additional info
		state := r.getPatternState(progress.Symbol, progress.Mode, progress.Timeframe)
		if state != nil {
			update.Direction = state.Direction
			if state.ReferenceCandle != nil {
				update.EntryLevels = r.calculateEntryLevels(state, state.ReferenceCandle.Close)
			}
		}

		updates = append(updates, update)
	}

	return updates
}

// GetPatternUpdate returns the current pattern update for a specific symbol.
func (r *RealtimePatternMatcher) GetPatternUpdate(symbol, mode, timeframe string) *PatternUpdate {
	if r.patternMatcher == nil {
		return nil
	}

	progress := r.patternMatcher.GetPattern(symbol, mode, timeframe)
	if progress == nil {
		return nil
	}

	update := PatternUpdate{
		Symbol:      progress.Symbol,
		Timeframe:   progress.Timeframe,
		Mode:        progress.Mode,
		Strategy:    progress.Strategy,
		SubStrategy: progress.SubStrategy,
		CurrentStep: progress.CurrentStep,
		TotalSteps:  progress.TotalSteps,
		Status:      progress.Status,
		StepDetails: progress.StepDetails,
		UpdatedAt:   progress.UpdatedAt,
	}

	// Get state for additional info
	state := r.getPatternState(symbol, mode, timeframe)
	if state != nil {
		update.Direction = state.Direction
		if state.ReferenceCandle != nil {
			update.EntryLevels = r.calculateEntryLevels(state, state.ReferenceCandle.Close)
		}
	}

	return &update
}

// ============================================================================
// INTEGRATION HELPERS
// ============================================================================

// RegisterWithCoinProfiler registers the realtime matcher with a CoinProfiler
// to receive candle close events.
func (r *RealtimePatternMatcher) RegisterWithCoinProfiler(cp *coinprofiler.CoinProfiler) {
	if cp == nil {
		return
	}

	cp.SetCandleCloseCallback(r.OnCandleClose)
	// Also register for price updates (tick-level breakout detection)
	cp.SetPriceUpdateCallback(r.OnPriceUpdate)

	// Register for volume progress updates (real-time volume ratio display)
	cp.SetVolumeProgressCallback(r.OnVolumeProgress)

	// Configure volume progress calculation parameters from pattern matcher config
	if r.patternMatcher != nil && r.patternMatcher.config != nil {
		cp.SetVolumeProgressConfig(
			r.patternMatcher.config.MinVolumeSpikeMultiplier,
			r.patternMatcher.config.LookbackPeriod,
		)
	}
}

// ============================================================================
// TICK-LEVEL BREAKOUT DETECTION AND REAL-TIME PROXIMITY UPDATES
// ============================================================================

// OnPriceUpdate is called on every price tick. This enables:
// 1. Real-time proximity updates for consolidating coins (UI shows live distance to entry)
// 2. Immediate breakout detection without waiting for candle close
// It only evaluates coins that have completed Step 1 (have a reference candle).
func (r *RealtimePatternMatcher) OnPriceUpdate(symbol, timeframe string, price, currentHigh, currentLow float64) {
	if r.patternMatcher == nil {
		return
	}

	// Skip suppressed symbols (active position exists)
	mode := r.defaultMode
	suppKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
	r.mu.RLock()
	suppressed := r.suppressedSymbols[suppKey]
	r.mu.RUnlock()
	if suppressed {
		return
	}

	// Get pattern state for this symbol
	state := r.getPatternState(symbol, mode, timeframe)
	if state == nil || state.ReferenceCandle == nil {
		return // No reference candle yet, nothing to check
	}

	// Check if pattern is in a state where updates are relevant
	progress := r.patternMatcher.GetPattern(symbol, mode, timeframe)
	if progress == nil {
		return
	}

	// Only process if we're in accumulation/consolidation phase
	if progress.Status != PatternStatusAccumulation && progress.Status != PatternStatusConsolidating {
		return
	}

	// Calculate proximity to breakout for real-time display
	var proximityPercent float64
	var isBreakout bool

	switch state.Direction {
	case "long":
		// For LONG: Check distance to reference high
		if state.ReferenceCandle.High > 0 {
			proximityPercent = ((price - state.ReferenceCandle.High) / state.ReferenceCandle.High) * 100
		}
		// Check if current high breaks reference high
		if currentHigh >= state.ReferenceCandle.High {
			isBreakout = true
		}
	case "short":
		// For SHORT: Check distance to consolidation low
		if state.ConsolidationLow > 0 {
			proximityPercent = ((state.ConsolidationLow - price) / state.ConsolidationLow) * 100
		}
		if state.ConsolidationLow > 0 && currentLow <= state.ConsolidationLow {
			isBreakout = true
		}
	}

	r.mu.RLock()
	callback := r.onPatternUpdate
	breakoutCallback := r.onBreakout
	capacityChecker := r.capacityChecker
	userID := r.userID
	r.mu.RUnlock()

	if isBreakout {
		// Breakout detected! Log and trigger update
		log.Printf("[REALTIME-BREAKOUT] %s:%s - TICK-LEVEL BREAKOUT! Direction: %s, Price: %.6f",
			symbol, timeframe, state.Direction, price)

		// PROACTIVE CAPACITY CHECK: Before triggering entry, check if we have capacity
		// This prevents wasteful order attempts when max_concurrent_trades is reached
		canEnter := true
		if capacityChecker != nil {
			var currentCount, maxCount int
			canEnter, currentCount, maxCount = capacityChecker()
			if !canEnter {
				log.Printf("[REALTIME-BREAKOUT] BLOCKED: max_concurrent_trades reached (%d/%d) - skipping breakout entry for %s",
					currentCount, maxCount, symbol)
				// Pattern is still marked as ready (breakout detected) but no order is placed
				// When capacity opens, user can manually trigger or wait for next pattern
			}
		}

		// Mark pattern as ready (breakout detected)
		r.patternMatcher.mu.Lock()
		patternKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
		internalProgress := r.patternMatcher.patterns[patternKey]
		internalState := r.patternMatcher.states[patternKey]
		if internalProgress != nil && internalProgress.Status != PatternStatusReady {
			// Mark Step 2 as complete
			statusDetail := fmt.Sprintf("Tick-level breakout! %s @ %.6f", state.Direction, price)
			if !canEnter {
				statusDetail = fmt.Sprintf("BREAKOUT DETECTED but blocked by max_concurrent_trades limit! %s @ %.6f", state.Direction, price)
			}
			internalProgress.StepDetails[1] = StepDetail{
				StepNumber:  2,
				Name:        "Breakout",
				Completed:   true,
				CompletedAt: time.Now(),
				Progress:    "LIVE",
				Details:     statusDetail,
			}
			internalProgress.SetStatus(PatternStatusReady)
			internalProgress.UpdatedAt = time.Now()

			// Store breakout candle data on state for entry candle display
			if internalState != nil {
				now := time.Now().UTC()
				internalState.ReadyAt = now
				// Create a synthetic breakout candle from tick data
				internalState.BreakoutCandle = &Candle{
					OpenTime: now, // Approximate - tick-level breakout doesn't have candle boundaries
					Time:     now,
					Open:     price,
					High:     currentHigh,
					Low:      currentLow,
					Close:    price,
				}
				// Entry price depends on direction
				if state.Direction == "long" {
					internalState.EntryPrice = state.ReferenceCandle.High
				} else {
					internalState.EntryPrice = state.ReferenceCandle.Low
				}
			}
		}
		r.patternMatcher.mu.Unlock()

		// CRITICAL: Only trigger entry callback if capacity is available
		// This enables instant order placement the moment breakout occurs
		if breakoutCallback != nil && canEnter {
			log.Printf("[REALTIME-BREAKOUT] Triggering immediate entry callback for %s: %s @ %.6f (timeframe=%s)",
				symbol, state.Direction, price, timeframe)
			// Pass strategy identifiers and timeframe for proper budget/leverage loading
			// This is the Ravindra Volume Imbalance strategy which is always: breakout/ravindra_volume_imbalance
			go breakoutCallback(symbol, state.Direction, mode, "breakout", "ravindra_volume_imbalance", timeframe, price)
		}

		// Get updated progress for broadcast
		progress = r.patternMatcher.GetPattern(symbol, mode, timeframe)
		if progress == nil {
			return
		}

		// Store new state
		stateKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
		r.mu.Lock()
		r.lastStates[stateKey] = progress
		r.mu.Unlock()

		// Persist breakout state to DB (async, fire-and-forget)
		r.savePatternStateAsync(symbol, mode, timeframe)
	}

	// ALWAYS broadcast update for consolidating coins - even without breakout
	// This enables real-time proximity display in the UI
	if callback != nil {
		// Create a copy of step details to update with real-time proximity
		stepDetails := make([]StepDetail, len(progress.StepDetails))
		copy(stepDetails, progress.StepDetails)

		// Update Step 2 details with real-time proximity
		if len(stepDetails) >= 2 && !isBreakout {
			stepDetails[1] = StepDetail{
				StepNumber: 2,
				Name:       "Breakout",
				Completed:  false,
				Progress:   fmt.Sprintf("%.2f%%", proximityPercent),
				Details:    fmt.Sprintf("Price: %.4f → Entry: %.4f", price, state.ReferenceCandle.High),
			}
		}

		// Get volume threshold from config
		volumeThreshold := 3.0
		if r.patternMatcher != nil {
			volumeThreshold = r.patternMatcher.config.MinVolumeSpikeMultiplier
		}

		update := PatternUpdate{
			UserID:          userID,
			Symbol:          symbol,
			Timeframe:       timeframe,
			Mode:            progress.Mode,
			Strategy:        progress.Strategy,
			SubStrategy:     progress.SubStrategy,
			CurrentStep:     progress.CurrentStep,
			TotalSteps:      progress.TotalSteps,
			Status:          progress.Status,
			StepDetails:     stepDetails,
			Direction:       state.Direction,
			LookingFor:      r.getLookingForDirection(),
			CurrentPrice:    price,
			VolumeThreshold: volumeThreshold,
			UpdatedAt:       time.Now(),
		}

		// Include reference candle for Stage 2 context display
		if state.ReferenceCandle != nil && progress.CurrentStep >= 2 {
			// Convert internal Candle to ReferenceCandle type
			volumeMultiplier := 0.0
			if state.AverageVolumeAtSpike > 0 {
				volumeMultiplier = state.ReferenceCandle.Volume / state.AverageVolumeAtSpike
			}
			update.ReferenceCandle = &ReferenceCandle{
				OpenTime:         state.ReferenceCandle.OpenTime,
				CloseTime:        state.ReferenceCandle.Time,
				Open:             state.ReferenceCandle.Open,
				High:             state.ReferenceCandle.High,
				Low:              state.ReferenceCandle.Low,
				Close:            state.ReferenceCandle.Close,
				Volume:           state.ReferenceCandle.Volume,
				VolumeMultiplier: volumeMultiplier,
			}
		}

		// Calculate and include entry levels with current price
		update.EntryLevels = r.calculateEntryLevels(state, price)
		if update.EntryLevels != nil {
			update.EntryLevels.CurrentPrice = price
		}

		// Include entry candle and timing fields
		update.EntryCandle = r.buildEntryCandle(state)
		r.addTimingFields(&update, state)

		callback(update)
	}
}

// ============================================================================
// ENTRY DECISION HANDLER INTEGRATION
// ============================================================================

// PatternUpdateBroadcaster defines the interface for broadcasting pattern updates.
type PatternUpdateBroadcaster interface {
	BroadcastPatternUpdate(update PatternUpdate) error
}

// WirePatternBroadcaster connects the realtime matcher to a broadcaster.
// This is typically called during server initialization.
func (r *RealtimePatternMatcher) WirePatternBroadcaster(broadcaster PatternUpdateBroadcaster) {
	if broadcaster == nil {
		return
	}

	r.SetPatternUpdateCallback(func(update PatternUpdate) {
		_ = broadcaster.BroadcastPatternUpdate(update)
	})
}

// ============================================================================
// CONTEXT-AWARE OPERATIONS
// ============================================================================

// EvaluateAllPatterns evaluates patterns for all tracked symbols.
// This is useful for batch evaluation (e.g., on startup or refresh).
func (r *RealtimePatternMatcher) EvaluateAllPatterns(
	ctx context.Context,
	coinProvider CoinDataProvider,
	historyProvider CoinProfilerHistoryProvider,
) error {
	if coinProvider == nil || historyProvider == nil {
		return fmt.Errorf("providers are required")
	}

	// Get all coin data
	coins, err := coinProvider.GetAllCoinDataCtx(ctx)
	if err != nil {
		return fmt.Errorf("failed to get coin data: %w", err)
	}

	// Evaluate each coin
	for symbol := range coins {
		// Get historical candles for each timeframe we track
		// IMPORTANT: Include ALL timeframes used by strategies (1m for ultra_fast, 3m for scalp/swing)
		for _, timeframe := range []string{"1m", "3m", "5m", "15m", "1h"} {
			candles := historyProvider.GetCandleHistory(symbol, timeframe)
			if len(candles) >= 25 {
				r.OnCandleClose(symbol, timeframe, candles)
			}
		}
	}

	return nil
}
