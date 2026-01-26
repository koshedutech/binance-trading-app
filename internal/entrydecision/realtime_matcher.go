// Package entrydecision provides real-time pattern matching for the Entry Decision System.
// Epic 14: Chain Trading System - Entry Decision Strategy Requirements & Real-Time Monitoring
package entrydecision

import (
	"context"
	"fmt"
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

	// Direction
	Direction string `json:"direction,omitempty"`

	// Timestamp
	UpdatedAt time.Time `json:"updated_at"`
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

// RealtimePatternMatcher handles real-time pattern evaluation on candle close events.
type RealtimePatternMatcher struct {
	// Pattern matcher for Volume Imbalance strategy
	patternMatcher *VolumeImbalancePatternMatcher

	// Callback for pattern state changes
	onPatternUpdate PatternUpdateCallback

	// Configuration
	defaultMode string
	riskReward  float64 // Default R:R ratio for entry level calculations

	// State tracking - last known states for change detection
	lastStates map[string]*PatternProgress // key: symbol:mode:timeframe
	mu         sync.RWMutex
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
	}
}

// SetPatternUpdateCallback sets the callback for pattern state changes.
func (r *RealtimePatternMatcher) SetPatternUpdateCallback(callback PatternUpdateCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onPatternUpdate = callback
}

// OnCandleClose is called when a candle closes. This is the main entry point
// for real-time pattern evaluation.
// It evaluates patterns for the given symbol/timeframe and triggers callbacks
// if the pattern state has changed.
func (r *RealtimePatternMatcher) OnCandleClose(symbol, timeframe string, candles []coinprofiler.HistoricalCandle) {
	if r.patternMatcher == nil || len(candles) < 25 {
		return // Not enough data for pattern detection
	}

	// Convert historical candles to pattern matcher format
	matcherCandles := make([]Candle, len(candles))
	for i, hc := range candles {
		matcherCandles[i] = Candle{
			Time:           hc.CloseTime,
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
	if coinMatch == nil {
		return
	}

	// Get full pattern progress
	progress := r.patternMatcher.GetPattern(symbol, mode, timeframe)
	if progress == nil {
		return
	}

	// Check if state has changed
	stateKey := fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
	if !r.hasStateChanged(stateKey, progress) {
		return // No change, skip update
	}

	// Store new state
	r.mu.Lock()
	r.lastStates[stateKey] = progress
	callback := r.onPatternUpdate
	r.mu.Unlock()

	// Build update
	update := r.buildPatternUpdate(symbol, timeframe, progress, matcherCandles)

	// Trigger callback if set
	if callback != nil {
		callback(update)
	}
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
		Symbol:      symbol,
		Timeframe:   timeframe,
		Mode:        progress.Mode,
		Strategy:    progress.Strategy,
		SubStrategy: progress.SubStrategy,
		CurrentStep: progress.CurrentStep,
		TotalSteps:  progress.TotalSteps,
		Status:      progress.Status,
		StepDetails: progress.StepDetails,
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
	coins, err := coinProvider.GetAllCoinData(ctx)
	if err != nil {
		return fmt.Errorf("failed to get coin data: %w", err)
	}

	// Evaluate each coin
	for symbol := range coins {
		// Get historical candles for each timeframe we track
		for _, timeframe := range []string{"5m", "15m", "1h"} {
			candles := historyProvider.GetCandleHistory(symbol, timeframe)
			if len(candles) >= 25 {
				r.OnCandleClose(symbol, timeframe, candles)
			}
		}
	}

	return nil
}
