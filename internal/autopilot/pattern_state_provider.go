// Package autopilot provides the PatternStateProvider which bridges the pattern matcher
// to the ChainEntryRunner for automatic order execution.
// Epic 14: Chain Trading System - Pattern to Order Execution Bridge
package autopilot

import (
	"context"
	"encoding/json"
	"log"

	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/entrydecision"
)

// PatternStateProvider implements ChainStateProvider interface by wrapping the
// RealtimePatternMatcher to provide ready patterns as ChainCoinState objects.
//
// This is the critical bridge between:
// - Pattern detection (RealtimePatternMatcher detecting volume spikes → breakouts)
// - Order execution (ChainEntryRunner placing market orders)
//
// Flow:
// 1. RealtimePatternMatcher detects pattern becomes "ready" (breakout confirmed)
// 2. ChainEntryRunner.executeScanCycle() calls GetAllChainCoinStates()
// 3. PatternStateProvider converts ready PatternProgress to ChainCoinState
// 4. ChainEntryRunner executes entry order for the ready state
type PatternStateProvider struct {
	realtimeMatcher *entrydecision.RealtimePatternMatcher
	userID          string
	repo            *database.Repository // Database access for strategy settings
}

// NewPatternStateProvider creates a new PatternStateProvider for a user.
func NewPatternStateProvider(matcher *entrydecision.RealtimePatternMatcher, userID string) *PatternStateProvider {
	return &PatternStateProvider{
		realtimeMatcher: matcher,
		userID:          userID,
	}
}

// SetRepository sets the database repository for accessing strategy settings.
func (p *PatternStateProvider) SetRepository(repo *database.Repository) {
	p.repo = repo
}

// GetAllChainCoinStates returns all coin states that are ready for entry.
// This implements the ChainStateProvider interface.
//
// The ChainEntryRunner calls this method every scan cycle (30s default) to find
// patterns that have reached "ready" status and should trigger order placement.
func (p *PatternStateProvider) GetAllChainCoinStates(ctx context.Context, userID string) ([]*ChainCoinState, error) {
	if p.realtimeMatcher == nil {
		return nil, nil
	}

	// Verify user ID matches (security check)
	if userID != p.userID {
		log.Printf("[PATTERN-STATE-PROVIDER] User ID mismatch: expected %s, got %s", p.userID, userID)
		return nil, nil
	}

	// Get the underlying pattern matcher
	patternMatcher := p.realtimeMatcher.GetPatternMatcher()
	if patternMatcher == nil {
		return nil, nil
	}

	// Get all ready patterns
	readyPatterns := patternMatcher.GetReadyPatterns()
	if len(readyPatterns) == 0 {
		return nil, nil
	}

	log.Printf("[PATTERN-STATE-PROVIDER] Found %d ready patterns for user %s", len(readyPatterns), userID)

	// Convert PatternProgress to ChainCoinState
	states := make([]*ChainCoinState, 0, len(readyPatterns))
	for _, pattern := range readyPatterns {
		state := p.patternToChainState(pattern, patternMatcher)
		if state != nil {
			states = append(states, state)
			log.Printf("[PATTERN-STATE-PROVIDER] Ready state: %s, mode=%s, direction=%s, score=%d",
				state.Symbol, state.ActiveStrategy, p.getDirection(pattern, patternMatcher), state.ScoreFinal)
		}
	}

	return states, nil
}

// GetCoinStateForEntry returns a coin state for a specific symbol with actual SL/TP prices
// from pattern detection. This is used by ExecuteImmediateEntry for tick-level breakout orders.
// If pattern is not found or not ready, returns nil.
func (p *PatternStateProvider) GetCoinStateForEntry(ctx context.Context, userID, symbol, mode, timeframe string) (*ChainCoinState, error) {
	if p.realtimeMatcher == nil {
		return nil, nil
	}

	// Get the pattern matcher
	patternMatcher := p.realtimeMatcher.GetPatternMatcher()
	if patternMatcher == nil {
		return nil, nil
	}

	// Get pattern for this specific symbol
	pattern := patternMatcher.GetPattern(symbol, mode, timeframe)
	if pattern == nil {
		log.Printf("[PATTERN-STATE-PROVIDER] GetCoinStateForEntry: No pattern found for %s:%s:%s", symbol, mode, timeframe)
		return nil, nil
	}

	// Convert to ChainCoinState (includes loading SL/TP prices)
	state := p.patternToChainState(pattern, patternMatcher)
	if state != nil {
		log.Printf("[PATTERN-STATE-PROVIDER] GetCoinStateForEntry: Got state for %s with SLPrice=%.6f, TPPrice=%.6f, RiskAmount=%.6f",
			symbol, state.SLPrice, state.TPPrice, state.RiskAmount)
	}

	return state, nil
}

// patternToChainState converts a PatternProgress to a ChainCoinState for order execution.
func (p *PatternStateProvider) patternToChainState(
	pattern *entrydecision.PatternProgress,
	matcher *entrydecision.VolumeImbalancePatternMatcher,
) *ChainCoinState {
	if pattern == nil {
		return nil
	}

	// Get the full coin match with entry details
	coinMatch := matcher.GetCoinMatch(pattern.Symbol, pattern.Mode, pattern.Timeframe)

	// Determine direction from pattern state
	direction := "long" // default
	if coinMatch != nil && coinMatch.Direction != "" {
		direction = coinMatch.Direction
	}

	// Determine trend based on direction
	trend := "BULLISH"
	if direction == "short" {
		trend = "BEARISH"
	}

	// Calculate a score for the pattern
	// Ready patterns get a high score to pass the threshold
	// Score is based on pattern completion + volume multiplier
	score := 80 // Base score for ready patterns

	// Bonus for volume confirmation
	if coinMatch != nil && coinMatch.EntryCandle != nil {
		if coinMatch.EntryCandle.VolumeMultiplier >= 2.0 {
			score += 10
		}
		if coinMatch.EntryCandle.VolumeMultiplier >= 3.0 {
			score += 5
		}
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	state := &ChainCoinState{
		Symbol:         pattern.Symbol,
		ActiveStrategy: pattern.Mode, // scalp, swing, position, ultra_fast
		Decision:       "READY",      // Pattern is ready for entry
		Regime:         "TRENDING",   // Assume trending for breakout patterns
		Trend1H:        trend,
		Trend15M:       trend,
		ScoreFinal:     score,
		ScoreTechnical: score,
		ScoreContext:   0,
		ScoreLLM:       0,
		ScoreHistory:   0,
		// Strategy identification
		StrategyGroup: pattern.Strategy,
		SubStrategy:   pattern.SubStrategy,
		Timeframe:     pattern.Timeframe,
	}

	// Add price and entry levels from coin match if available
	if coinMatch != nil {
		state.Price = coinMatch.CurrentPrice

		// If entry candle has entry price, use that
		if coinMatch.EntryCandle != nil && coinMatch.EntryCandle.EntryPrice > 0 {
			state.Price = coinMatch.EntryCandle.EntryPrice
		}

		// Pass actual SL/TP prices from pattern detection (preferred over percentages)
		// These are calculated from Support Price (Consolidation Low) at runtime
		if coinMatch.EntryLevelPrice > 0 {
			state.EntryPrice = coinMatch.EntryLevelPrice
		}
		if coinMatch.StopLossPrice > 0 {
			state.SLPrice = coinMatch.StopLossPrice
			log.Printf("[PATTERN-STATE-PROVIDER] Actual SL price from pattern: %.6f", coinMatch.StopLossPrice)
		}
		if coinMatch.TakeProfitPrice > 0 {
			state.TPPrice = coinMatch.TakeProfitPrice
			log.Printf("[PATTERN-STATE-PROVIDER] Actual TP price from pattern: %.6f", coinMatch.TakeProfitPrice)
		}
		if coinMatch.RiskAmount > 0 {
			state.RiskAmount = coinMatch.RiskAmount
			log.Printf("[PATTERN-STATE-PROVIDER] Risk amount: %.6f (%.2f%%)", coinMatch.RiskAmount, coinMatch.RiskPercent)
		}
	}

	// Get strategy settings for budget/SL/TP configuration
	if p.repo != nil {
		ctx := context.Background()

		// Get strategy group settings for leverage (budget comes from sub-strategy)
		groupSettings, err := p.repo.GetStrategyGroupSettings(ctx, p.userID, pattern.Mode, pattern.Strategy)
		if err == nil && groupSettings != nil {
			// Only get leverage from strategy group - budget should come from sub-strategy
			state.MaxLeverage = groupSettings.MaxLeverage

			log.Printf("[PATTERN-STATE-PROVIDER] Strategy group settings: mode=%s, group=%s, leverage=%d",
				pattern.Mode, pattern.Strategy, groupSettings.MaxLeverage)
		}

		// Get sub-strategy specific settings for custom SL/TP
		subSettings, err := p.repo.GetSubStrategySettings(ctx, p.userID, pattern.Mode, pattern.Strategy, pattern.SubStrategy)
		if err == nil && subSettings != nil && len(subSettings.Settings) > 0 {
			// Parse settings JSON for custom budget, SL, TP
			var settingsMap map[string]interface{}
			if err := json.Unmarshal(subSettings.Settings, &settingsMap); err == nil {
				// Check for budget_allocation.assigned_budget_usd (nested object)
				if budgetAlloc, ok := settingsMap["budget_allocation"].(map[string]interface{}); ok {
					assignedBudget := 0.0
					currentEquity := 0.0
					useIncremental := false
					positionSizing := ""

					// Get assigned budget
					if budgetUSD, ok := budgetAlloc["assigned_budget_usd"].(float64); ok && budgetUSD > 0 {
						assignedBudget = budgetUSD
					}

					// Get current equity (compounded value from wins/losses)
					if equity, ok := budgetAlloc["current_equity"].(float64); ok && equity > 0 {
						currentEquity = equity
					}

					// Get position sizing mode
					if sizing, ok := budgetAlloc["position_sizing"].(string); ok {
						positionSizing = sizing
					}

					// Get incremental equity flag
					if incremental, ok := budgetAlloc["use_incremental_equity"].(bool); ok {
						useIncremental = incremental
					}

					// Determine which budget to use:
					// - "all_in" with use_incremental_equity: use current_equity (compounding)
					// - Otherwise: use assigned_budget_usd (fixed)
					if positionSizing == "all_in" && useIncremental && currentEquity > 0 {
						state.BudgetUSD = currentEquity
						log.Printf("[PATTERN-STATE-PROVIDER] Using compounded budget: %.2f USD (assigned: %.2f, gain: %.2f)",
							currentEquity, assignedBudget, currentEquity-assignedBudget)
					} else if assignedBudget > 0 {
						state.BudgetUSD = assignedBudget
						log.Printf("[PATTERN-STATE-PROVIDER] Using assigned budget: %.2f USD", assignedBudget)
					}

					// Also get max_concurrent_trades for position limit enforcement
					if maxTrades, ok := budgetAlloc["max_concurrent_trades"].(float64); ok && maxTrades > 0 {
						log.Printf("[PATTERN-STATE-PROVIDER] Sub-strategy max concurrent trades: %.0f", maxTrades)
					}
				}

				// Check for risk_reward settings (used for R:R based SL/TP)
				if riskReward, ok := settingsMap["risk_reward"].(map[string]interface{}); ok {
					if risk, ok := riskReward["risk"].(float64); ok && risk > 0 {
						if reward, ok := riskReward["reward"].(float64); ok && reward > 0 {
							// Store R:R ratio for TP calculation (Entry + Risk × Reward)
							state.TPPercent = reward / risk // R:R ratio (e.g., 4.0 for 1:4)
							log.Printf("[PATTERN-STATE-PROVIDER] Sub-strategy R:R ratio: 1:%.0f", reward/risk)
						}
					}
				}

				// Check for pattern_detection.max_sl_percent (caps SL percentage)
				if patternDet, ok := settingsMap["pattern_detection"].(map[string]interface{}); ok {
					if maxSL, ok := patternDet["max_sl_percent"].(float64); ok && maxSL > 0 {
						state.SLPercent = maxSL
						log.Printf("[PATTERN-STATE-PROVIDER] Sub-strategy max SL: %.2f%%", maxSL)
					}
				}
			}
		}
	}

	return state
}

// getDirection extracts direction from pattern state.
func (p *PatternStateProvider) getDirection(
	pattern *entrydecision.PatternProgress,
	matcher *entrydecision.VolumeImbalancePatternMatcher,
) string {
	if matcher == nil || pattern == nil {
		return "unknown"
	}

	coinMatch := matcher.GetCoinMatch(pattern.Symbol, pattern.Mode, pattern.Timeframe)
	if coinMatch != nil && coinMatch.Direction != "" {
		return coinMatch.Direction
	}
	return "unknown"
}
