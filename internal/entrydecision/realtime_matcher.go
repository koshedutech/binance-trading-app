// Package entrydecision provides real-time pattern matching for the Entry Decision System.
// Epic 14: Chain Trading System - Entry Decision Strategy Requirements & Real-Time Monitoring
package entrydecision

import (
	"context"
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

	// Direction - the actual direction being tracked (long/short)
	Direction string `json:"direction,omitempty"`

	// LookingFor - what the strategy is configured to find (long/short/both)
	LookingFor string `json:"looking_for,omitempty"`

	// Countdown timer for next candle close
	NextCandleClose time.Time `json:"next_candle_close,omitempty"`

	// Last evaluation timestamp (so frontend knows data is fresh)
	LastEvaluatedAt time.Time `json:"last_evaluated_at,omitempty"`

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
// Parameters: symbol, direction ("long"/"short"), mode, price at breakout
type BreakoutCallback func(symbol, direction, mode string, price float64)

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

	// User identification for targeted broadcasts
	userID string

	// Configuration
	defaultMode string
	riskReward  float64 // Default R:R ratio for entry level calculations

	// State tracking - last known states for change detection
	lastStates map[string]*PatternProgress // key: symbol:mode:timeframe

	// Volume progress storage - latest volume progress per symbol
	volumeProgress map[string]*VolumeProgress // key: symbol:timeframe

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
		patternMatcher: patternMatcher,
		defaultMode:    config.DefaultMode,
		riskReward:     config.RiskRewardRatio,
		lastStates:     make(map[string]*PatternProgress),
		volumeProgress: make(map[string]*VolumeProgress),
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

// OnVolumeProgress is called on every tick with volume progress data from CoinProfiler.
// It stores the latest volume progress and broadcasts it via WebSocket in real-time.
func (r *RealtimePatternMatcher) OnVolumeProgress(data coinprofiler.VolumeProgressData) {
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
		update := PatternUpdate{
			UserID:         userID,
			Symbol:         data.Symbol,
			Timeframe:      data.Timeframe,
			Mode:           r.defaultMode,
			Strategy:       "volume_imbalance",
			SubStrategy:    "ravindra_volume_imbalance",
			CurrentStep:    1,
			TotalSteps:     2,
			Status:         PatternStatusWatching,
			VolumeProgress: progress,
			LookingFor:     r.getLookingForDirection(),
			UpdatedAt:      time.Now(),
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

	// Clear our cached states
	r.mu.Lock()
	r.lastStates = make(map[string]*PatternProgress)
	r.volumeProgress = make(map[string]*VolumeProgress)
	r.mu.Unlock()

	log.Printf("[REALTIME-PATTERN] Cleared all pattern states for fresh start")
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
	mode := r.defaultMode
	coinMatch := r.patternMatcher.MatchPattern(symbol, mode, timeframe, matcherCandles)

	// Get full pattern progress (even if coinMatch is nil, we want current state)
	progress := r.patternMatcher.GetPattern(symbol, mode, timeframe)

	if coinMatch != nil {
		log.Printf("[REALTIME-PATTERN] %s:%s - Pattern match found! Step %d, Status: %s",
			symbol, timeframe, coinMatch.Step, coinMatch.Status)
	}

	// Check if state has changed (for logging purposes)
	stateKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
	stateChanged := r.hasStateChanged(stateKey, progress)

	if stateChanged && progress != nil {
		// Store new state
		r.mu.Lock()
		r.lastStates[stateKey] = progress
		r.mu.Unlock()
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

	// Get pattern state for entry levels
	state := r.getPatternState(symbol, progress.Mode, timeframe)
	if state != nil {
		update.Direction = state.Direction

		// Calculate entry levels if we have enough state
		if state.ReferenceCandle != nil && len(candles) > 0 {
			currentPrice := candles[len(candles)-1].Close
			update.EntryLevels = r.calculateEntryLevels(state, currentPrice)
		}
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
func (r *RealtimePatternMatcher) calculateEntryLevels(state *PatternState, currentPrice float64) *EntryLevels {
	if state == nil || state.ReferenceCandle == nil {
		return nil
	}

	// Entry: At/above reference candle high
	entryPrice := state.ReferenceCandle.High

	// Stop Loss: Below consolidation low (or reference low) - 0.1%
	stopLossBase := state.ConsolidationLow
	if stopLossBase <= 0 {
		stopLossBase = state.ReferenceCandle.Low
	}
	stopLoss := stopLossBase * 0.999 // 0.1% below

	// Risk calculation
	risk := entryPrice - stopLoss
	if risk <= 0 {
		return nil // Invalid risk
	}

	// Take Profit: Entry + (Risk × R:R ratio)
	takeProfit := entryPrice + (risk * r.riskReward)

	// Calculate percentages
	riskPercent := (risk / entryPrice) * 100
	rewardPercent := ((takeProfit - entryPrice) / entryPrice) * 100

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

	// Get pattern state for this symbol
	mode := r.defaultMode
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
	userID := r.userID
	r.mu.RUnlock()

	if isBreakout {
		// Breakout detected! Log and trigger update
		log.Printf("[REALTIME-BREAKOUT] %s:%s - TICK-LEVEL BREAKOUT! Direction: %s, Price: %.6f",
			symbol, timeframe, state.Direction, price)

		// Mark pattern as ready (breakout detected)
		r.patternMatcher.mu.Lock()
		patternKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
		internalProgress := r.patternMatcher.patterns[patternKey]
		if internalProgress != nil && internalProgress.Status != PatternStatusReady {
			// Mark Step 2 as complete
			internalProgress.StepDetails[1] = StepDetail{
				StepNumber:  2,
				Name:        "Breakout",
				Completed:   true,
				CompletedAt: time.Now(),
				Progress:    "LIVE",
				Details:     fmt.Sprintf("Tick-level breakout! %s @ %.6f", state.Direction, price),
			}
			internalProgress.SetStatus(PatternStatusReady)
			internalProgress.UpdatedAt = time.Now()
		}
		r.patternMatcher.mu.Unlock()

		// CRITICAL: Trigger immediate order execution callback
		// This enables instant order placement the moment breakout occurs
		if breakoutCallback != nil {
			log.Printf("[REALTIME-BREAKOUT] Triggering immediate entry callback for %s: %s @ %.6f",
				symbol, state.Direction, price)
			go breakoutCallback(symbol, state.Direction, mode, price)
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

		update := PatternUpdate{
			UserID:      userID,
			Symbol:      symbol,
			Timeframe:   timeframe,
			Mode:        progress.Mode,
			Strategy:    progress.Strategy,
			SubStrategy: progress.SubStrategy,
			CurrentStep: progress.CurrentStep,
			TotalSteps:  progress.TotalSteps,
			Status:      progress.Status,
			StepDetails: stepDetails,
			Direction:   state.Direction,
			LookingFor:  r.getLookingForDirection(),
			UpdatedAt:   time.Now(),
		}

		// Calculate and include entry levels with current price
		update.EntryLevels = r.calculateEntryLevels(state, price)
		if update.EntryLevels != nil {
			update.EntryLevels.CurrentPrice = price
		}

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
