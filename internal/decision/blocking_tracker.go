// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.18: Blocking Reason Tracker
package decision

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// DefaultMaxHistory is the default maximum history entries per symbol.
const DefaultMaxHistory = 100

// BlockingTracker tracks and analyzes blocking reasons for position decisions.
// It integrates with the StateManager to persist blocking reasons in Redis.
type BlockingTracker struct {
	config     *BlockingConfig
	sm         *StateManager
	history    map[string][]BlockingReason // userID:symbol -> history
	historyMu  sync.RWMutex
	maxHistory int
	configMu   sync.RWMutex
}

// NewBlockingTracker creates a new BlockingTracker with the given StateManager and config.
// If config is nil, uses default configuration.
func NewBlockingTracker(sm *StateManager, config *BlockingConfig) *BlockingTracker {
	if config == nil {
		config = DefaultBlockingConfig()
	}
	config.Validate()

	return &BlockingTracker{
		config:     config,
		sm:         sm,
		history:    make(map[string][]BlockingReason),
		maxHistory: DefaultMaxHistory,
	}
}

// GetConfig returns a copy of the current blocking configuration.
func (bt *BlockingTracker) GetConfig() *BlockingConfig {
	bt.configMu.RLock()
	defer bt.configMu.RUnlock()
	return bt.config.Clone()
}

// SetConfig updates the blocking configuration.
func (bt *BlockingTracker) SetConfig(config *BlockingConfig) {
	if config == nil {
		return
	}
	config.Validate()
	bt.configMu.Lock()
	defer bt.configMu.Unlock()
	bt.config = config
}

// SetMaxHistory sets the maximum history entries to retain per symbol.
func (bt *BlockingTracker) SetMaxHistory(max int) {
	if max <= 0 {
		max = DefaultMaxHistory
	}
	bt.historyMu.Lock()
	bt.maxHistory = max
	bt.historyMu.Unlock()
}

// CheckForBlocks analyzes the coin state and score to detect blocking conditions.
// Returns a slice of all detected blocking reasons.
func (bt *BlockingTracker) CheckForBlocks(state *CoinState, score *ScoreBreakdown) []BlockingReason {
	if state == nil {
		return nil
	}

	bt.configMu.RLock()
	config := bt.config.Clone()
	bt.configMu.RUnlock()

	var reasons []BlockingReason

	// Hard Blocks
	if config.EnableHardBlocks {
		reasons = append(reasons, bt.checkHardBlocks(state, config)...)
	}

	// Soft Blocks
	if config.EnableSoftBlocks {
		reasons = append(reasons, bt.checkSoftBlocks(state, score, config)...)
	}

	// Warnings
	if config.EnableWarnings {
		reasons = append(reasons, bt.checkWarnings(state, config)...)
	}

	// Limit to max reasons
	if len(reasons) > config.MaxBlockingReasons {
		reasons = reasons[:config.MaxBlockingReasons]
	}

	return reasons
}

// checkHardBlocks detects hard blocking conditions that cannot be overridden.
func (bt *BlockingTracker) checkHardBlocks(state *CoinState, config *BlockingConfig) []BlockingReason {
	var reasons []BlockingReason

	// Hard Block: Trend Divergence (1H != 15M and both have direction)
	if bt.hasTrendDivergence(state) {
		reason := NewBlockingReason(
			ReasonTrendDivergence,
			CategoryHardBlock,
			fmt.Sprintf("Trend divergence: 1H=%s, 15M=%s", state.Trend1H, state.Trend15M),
		).WithValue(map[string]string{
			"trend_1h":  string(state.Trend1H),
			"trend_15m": string(state.Trend15M),
		})
		reasons = append(reasons, *reason)
	}

	// Hard Block: ADX Too Low
	if state.ADX > 0 && state.ADX < config.ADXMinimum {
		reason := NewBlockingReason(
			ReasonADXTooLow,
			CategoryHardBlock,
			fmt.Sprintf("ADX %.2f is below minimum threshold", state.ADX),
		).WithValue(state.ADX).WithThreshold(config.ADXMinimum)
		reasons = append(reasons, *reason)
	}

	// Hard Block: Timeframe Misalignment (opposing trends)
	if bt.hasTimeframeMisalignment(state) {
		reason := NewBlockingReason(
			ReasonTimeframeMisalign,
			CategoryHardBlock,
			"Timeframes show opposing directional signals",
		).WithValue(map[string]string{
			"trend_1h":  string(state.Trend1H),
			"trend_15m": string(state.Trend15M),
		})
		reasons = append(reasons, *reason)
	}

	return reasons
}

// hasTrendDivergence checks if 1H and 15M trends are divergent (opposing directions).
func (bt *BlockingTracker) hasTrendDivergence(state *CoinState) bool {
	// Both must have directional trends (not neutral/empty)
	if state.Trend1H == "" || state.Trend1H == TrendNeutral {
		return false
	}
	if state.Trend15M == "" || state.Trend15M == TrendNeutral {
		return false
	}

	// Check for opposing directions
	return state.Trend1H != state.Trend15M
}

// hasTimeframeMisalignment is similar to trend divergence but used for
// checking specific misalignment scenarios.
func (bt *BlockingTracker) hasTimeframeMisalignment(state *CoinState) bool {
	// For now, this is the same as trend divergence
	// Can be extended to check more timeframes or specific patterns
	return false // Avoid duplicate with TrendDivergence - extend as needed
}

// checkSoftBlocks detects soft blocking conditions that can be overridden.
func (bt *BlockingTracker) checkSoftBlocks(state *CoinState, score *ScoreBreakdown, config *BlockingConfig) []BlockingReason {
	var reasons []BlockingReason

	// Soft Block: Score Below Threshold
	if score != nil && score.NormalizedScore < config.ScoreMinimum {
		reason := NewBlockingReason(
			ReasonScoreBelowThreshold,
			CategorySoftBlock,
			fmt.Sprintf("Score %.1f is below minimum threshold", score.NormalizedScore),
		).WithValue(score.NormalizedScore).WithThreshold(config.ScoreMinimum)
		reasons = append(reasons, *reason)
	}

	// Soft Block: Low LLM Confidence (if LLM component is low)
	if score != nil {
		llmComp := score.GetComponent(DimensionLLM)
		if llmComp != nil && llmComp.Percentage() < 30 {
			reason := NewBlockingReason(
				ReasonLowConfidence,
				CategorySoftBlock,
				fmt.Sprintf("LLM confidence %.1f%% is low", llmComp.Percentage()),
			).WithValue(llmComp.Percentage()).WithThreshold(30.0)
			reasons = append(reasons, *reason)
		}
	}

	// Soft Block: Weak Momentum
	if score != nil {
		techComp := score.GetComponent(DimensionTechnical)
		if techComp != nil {
			if momentum, ok := techComp.Details["momentum"]; ok && momentum < 4 {
				reason := NewBlockingReason(
					ReasonWeakMomentum,
					CategorySoftBlock,
					fmt.Sprintf("Momentum score %.1f is weak", momentum),
				).WithValue(momentum).WithThreshold(4.0)
				reasons = append(reasons, *reason)
			}
		}
	}

	return reasons
}

// checkWarnings detects warning conditions that are informational.
func (bt *BlockingTracker) checkWarnings(state *CoinState, config *BlockingConfig) []BlockingReason {
	var reasons []BlockingReason

	// Warning: High RSI (overbought)
	if state.RSI > config.RSIUpperLimit {
		reason := NewBlockingReason(
			ReasonHighRSI,
			CategoryWarning,
			fmt.Sprintf("RSI %.1f indicates overbought conditions", state.RSI),
		).WithValue(state.RSI).WithThreshold(config.RSIUpperLimit)
		reasons = append(reasons, *reason)
	}

	// Warning: Low RSI (oversold)
	if state.RSI > 0 && state.RSI < config.RSILowerLimit {
		reason := NewBlockingReason(
			ReasonLowRSI,
			CategoryWarning,
			fmt.Sprintf("RSI %.1f indicates oversold conditions", state.RSI),
		).WithValue(state.RSI).WithThreshold(config.RSILowerLimit)
		reasons = append(reasons, *reason)
	}

	// Warning: Low Volume (placeholder - can be enhanced when volume data available)
	// This is a placeholder for future enhancement when CoinState includes volume data

	// Warning: Wide Spread (placeholder - can be enhanced when spread data available)
	// This is a placeholder for future enhancement when spread data available

	return reasons
}

// AddBlockingReason adds a blocking reason to a symbol's state and history.
func (bt *BlockingTracker) AddBlockingReason(ctx context.Context, userID, symbol string, reason BlockingReason) error {
	if bt.sm == nil {
		return fmt.Errorf("StateManager not configured")
	}

	// Get current state
	state, err := bt.sm.GetCoinState(ctx, userID, symbol)
	if err != nil {
		return fmt.Errorf("failed to get coin state: %w", err)
	}

	if state == nil {
		return fmt.Errorf("coin state not found for %s", symbol)
	}

	// Add to state's blocking reasons
	reasonStr := reason.ToJSON()
	state.AddBlockingReason(reasonStr)

	// Update Redis
	if err := bt.sm.SetBlockingReasons(ctx, userID, symbol, state.BlockingReasons); err != nil {
		return fmt.Errorf("failed to update blocking reasons: %w", err)
	}

	// Add to history
	bt.addToHistory(userID, symbol, reason)

	return nil
}

// AddBlockingReasons adds multiple blocking reasons at once.
func (bt *BlockingTracker) AddBlockingReasons(ctx context.Context, userID, symbol string, reasons []BlockingReason) error {
	if bt.sm == nil {
		return fmt.Errorf("StateManager not configured")
	}

	if len(reasons) == 0 {
		return nil
	}

	// Convert reasons to string slice
	reasonStrs := make([]string, len(reasons))
	for i, r := range reasons {
		reasonStrs[i] = r.ToJSON()
	}

	// Update Redis
	if err := bt.sm.SetBlockingReasons(ctx, userID, symbol, reasonStrs); err != nil {
		return fmt.Errorf("failed to update blocking reasons: %w", err)
	}

	// Add to history
	for _, reason := range reasons {
		bt.addToHistory(userID, symbol, reason)
	}

	return nil
}

// ClearBlockingReasons removes all blocking reasons for a symbol.
func (bt *BlockingTracker) ClearBlockingReasons(ctx context.Context, userID, symbol string) error {
	if bt.sm == nil {
		return fmt.Errorf("StateManager not configured")
	}

	return bt.sm.SetBlockingReasons(ctx, userID, symbol, []string{})
}

// GetBlockingReasons retrieves current blocking reasons for a symbol.
func (bt *BlockingTracker) GetBlockingReasons(ctx context.Context, userID, symbol string) ([]BlockingReason, error) {
	if bt.sm == nil {
		return nil, fmt.Errorf("StateManager not configured")
	}

	state, err := bt.sm.GetCoinState(ctx, userID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get coin state: %w", err)
	}

	if state == nil {
		return nil, nil
	}

	return bt.parseBlockingReasons(state.BlockingReasons), nil
}

// parseBlockingReasons converts string slice to BlockingReason slice.
func (bt *BlockingTracker) parseBlockingReasons(strs []string) []BlockingReason {
	reasons := make([]BlockingReason, 0, len(strs))
	for _, s := range strs {
		var reason BlockingReason
		if err := json.Unmarshal([]byte(s), &reason); err == nil {
			reasons = append(reasons, reason)
		}
	}
	return reasons
}

// HasHardBlock checks if any of the reasons are hard blocks.
func (bt *BlockingTracker) HasHardBlock(reasons []BlockingReason) bool {
	for _, r := range reasons {
		if r.Category == CategoryHardBlock {
			return true
		}
	}
	return false
}

// HasSoftBlock checks if any of the reasons are soft blocks.
func (bt *BlockingTracker) HasSoftBlock(reasons []BlockingReason) bool {
	for _, r := range reasons {
		if r.Category == CategorySoftBlock {
			return true
		}
	}
	return false
}

// HasWarning checks if any of the reasons are warnings.
func (bt *BlockingTracker) HasWarning(reasons []BlockingReason) bool {
	for _, r := range reasons {
		if r.Category == CategoryWarning {
			return true
		}
	}
	return false
}

// IsBlocked returns true if there are any hard or soft blocks.
func (bt *BlockingTracker) IsBlocked(reasons []BlockingReason) bool {
	return bt.HasHardBlock(reasons) || bt.HasSoftBlock(reasons)
}

// CanOverride returns true if all blocks can be overridden (no hard blocks).
func (bt *BlockingTracker) CanOverride(reasons []BlockingReason) bool {
	return !bt.HasHardBlock(reasons)
}

// GetBlockingSummary provides an aggregate view of blocking reasons.
func (bt *BlockingTracker) GetBlockingSummary(reasons []BlockingReason) *BlockingSummary {
	summary := NewBlockingSummary()
	summary.TotalReasons = len(reasons)
	summary.AllReasons = reasons

	for _, r := range reasons {
		switch r.Category {
		case CategoryHardBlock:
			summary.HardBlockCount++
			summary.CanOverride = false
			if summary.PrimaryReason == nil {
				reasonCopy := r
				summary.PrimaryReason = &reasonCopy
			}
		case CategorySoftBlock:
			summary.SoftBlockCount++
			if summary.PrimaryReason == nil && summary.HardBlockCount == 0 {
				reasonCopy := r
				summary.PrimaryReason = &reasonCopy
			}
		case CategoryWarning:
			summary.WarningCount++
		}
	}

	summary.IsBlocked = summary.HardBlockCount > 0 || summary.SoftBlockCount > 0
	return summary
}

// addToHistory adds a blocking reason to the history.
func (bt *BlockingTracker) addToHistory(userID, symbol string, reason BlockingReason) {
	bt.historyMu.Lock()
	defer bt.historyMu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, symbol)
	history := bt.history[key]

	// Add new reason
	history = append(history, reason)

	// Trim to max history
	if len(history) > bt.maxHistory {
		history = history[len(history)-bt.maxHistory:]
	}

	bt.history[key] = history
}

// GetHistory returns the blocking history for a symbol.
func (bt *BlockingTracker) GetHistory(userID, symbol string) []BlockingReason {
	bt.historyMu.RLock()
	defer bt.historyMu.RUnlock()

	key := fmt.Sprintf("%s:%s", userID, symbol)
	history := bt.history[key]

	// Return a copy
	result := make([]BlockingReason, len(history))
	copy(result, history)
	return result
}

// ClearHistory clears the blocking history for a symbol.
func (bt *BlockingTracker) ClearHistory(userID, symbol string) {
	bt.historyMu.Lock()
	defer bt.historyMu.Unlock()

	key := fmt.Sprintf("%s:%s", userID, symbol)
	delete(bt.history, key)
}

// ClearAllHistory clears all blocking history.
func (bt *BlockingTracker) ClearAllHistory() {
	bt.historyMu.Lock()
	defer bt.historyMu.Unlock()

	bt.history = make(map[string][]BlockingReason)
}

// GetBlockingStats calculates statistics about blocking reasons for a user.
func (bt *BlockingTracker) GetBlockingStats(userID string) *BlockingStats {
	bt.historyMu.RLock()
	defer bt.historyMu.RUnlock()

	stats := NewBlockingStats(userID)
	symbolSet := make(map[string]bool)

	prefix := userID + ":"
	for key, reasons := range bt.history {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			symbol := key[len(prefix):]
			symbolSet[symbol] = true

			for _, reason := range reasons {
				stats.TotalBlocks++
				stats.BlocksByCode[reason.Code]++
				stats.BlocksByCategory[reason.Category]++
			}
		}
	}

	stats.SymbolsAffected = len(symbolSet)

	// Find most frequent block
	maxCount := 0
	for code, count := range stats.BlocksByCode {
		if count > maxCount {
			maxCount = count
			stats.MostFrequentBlock = code
		}
	}

	stats.LastUpdated = time.Now().UnixMilli()
	return stats
}

// EvaluateAndUpdate evaluates a coin state, detects blocks, and updates Redis.
// This is the main integration point for the blocking tracker.
func (bt *BlockingTracker) EvaluateAndUpdate(ctx context.Context, userID string, state *CoinState, score *ScoreBreakdown) (*BlockingSummary, error) {
	if state == nil {
		return nil, fmt.Errorf("state cannot be nil")
	}

	// Detect blocking conditions
	reasons := bt.CheckForBlocks(state, score)

	// Update Redis if we have a StateManager
	if bt.sm != nil {
		if err := bt.AddBlockingReasons(ctx, userID, state.Symbol, reasons); err != nil {
			return nil, fmt.Errorf("failed to update blocking reasons: %w", err)
		}
	}

	// Return summary
	return bt.GetBlockingSummary(reasons), nil
}

// GetReasonsByCategory returns all reasons of a specific category.
func (bt *BlockingTracker) GetReasonsByCategory(reasons []BlockingReason, category BlockingCategory) []BlockingReason {
	result := make([]BlockingReason, 0)
	for _, r := range reasons {
		if r.Category == category {
			result = append(result, r)
		}
	}
	return result
}

// GetReasonByCode finds a specific reason by code.
func (bt *BlockingTracker) GetReasonByCode(reasons []BlockingReason, code BlockingReasonCode) *BlockingReason {
	for i := range reasons {
		if reasons[i].Code == code {
			return &reasons[i]
		}
	}
	return nil
}
