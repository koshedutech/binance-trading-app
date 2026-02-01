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

	// Add price from coin match if available
	if coinMatch != nil {
		state.Price = coinMatch.CurrentPrice

		// If entry candle has entry price, use that
		if coinMatch.EntryCandle != nil && coinMatch.EntryCandle.EntryPrice > 0 {
			state.Price = coinMatch.EntryCandle.EntryPrice
		}
	}

	// Get strategy settings for budget/SL/TP configuration
	if p.repo != nil {
		ctx := context.Background()

		// Get strategy group settings for position size and leverage
		groupSettings, err := p.repo.GetStrategyGroupSettings(ctx, p.userID, pattern.Mode, pattern.Strategy)
		if err == nil && groupSettings != nil {
			// PositionSizePercent is stored as percentage (e.g., 2.0 = 2%)
			// For now, use it as a rough USD equivalent (will need account balance for proper calculation)
			// The sub-strategy settings may have a budget_usd override
			state.BudgetUSD = groupSettings.PositionSizePercent * 5 // Default: treat 2% as ~$10 base
			state.MaxLeverage = groupSettings.MaxLeverage

			log.Printf("[PATTERN-STATE-PROVIDER] Strategy settings: mode=%s, group=%s, posSize=%f%%, leverage=%d",
				pattern.Mode, pattern.Strategy, groupSettings.PositionSizePercent, groupSettings.MaxLeverage)
		}

		// Get sub-strategy specific settings for custom SL/TP
		subSettings, err := p.repo.GetSubStrategySettings(ctx, p.userID, pattern.Mode, pattern.Strategy, pattern.SubStrategy)
		if err == nil && subSettings != nil && len(subSettings.Settings) > 0 {
			// Parse settings JSON for custom budget, SL, TP
			var settingsMap map[string]interface{}
			if err := json.Unmarshal(subSettings.Settings, &settingsMap); err == nil {
				// Check for budget_usd override
				if budgetUSD, ok := settingsMap["budget_usd"].(float64); ok && budgetUSD > 0 {
					state.BudgetUSD = budgetUSD
					log.Printf("[PATTERN-STATE-PROVIDER] Sub-strategy budget override: %.2f USD", budgetUSD)
				}
				// Check for SL/TP overrides
				if slPercent, ok := settingsMap["sl_percent"].(float64); ok && slPercent > 0 {
					state.SLPercent = slPercent
				}
				if tpPercent, ok := settingsMap["tp_percent"].(float64); ok && tpPercent > 0 {
					state.TPPercent = tpPercent
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
