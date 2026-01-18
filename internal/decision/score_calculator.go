// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.15: Additive Score Calculator
package decision

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"
)

// ScoreCalculator computes additive scores for position decisions.
// It uses the formula: FINAL_SCORE = Technical(0-40) + Context(0-30) + LLM(0-20) + History(0-10)
type ScoreCalculator struct {
	config      *ScoringConfig
	sm          *StateManager
	history     map[string]*ScoreHistory // symbol -> history
	mutex       sync.RWMutex             // Protects history map
	configMutex sync.RWMutex             // Protects config access
}

// NewScoreCalculator creates a new ScoreCalculator with the given StateManager and config.
// If config is nil, uses default configuration.
func NewScoreCalculator(sm *StateManager, config *ScoringConfig) *ScoreCalculator {
	if config == nil {
		config = DefaultScoringConfig()
	}
	config.Validate()

	return &ScoreCalculator{
		config:  config,
		sm:      sm,
		history: make(map[string]*ScoreHistory),
	}
}

// GetConfig returns a copy of the current scoring configuration
func (sc *ScoreCalculator) GetConfig() *ScoringConfig {
	sc.configMutex.RLock()
	defer sc.configMutex.RUnlock()
	return sc.config.Clone()
}

// SetConfig updates the scoring configuration
func (sc *ScoreCalculator) SetConfig(config *ScoringConfig) {
	if config == nil {
		return
	}
	config.Validate()
	sc.configMutex.Lock()
	defer sc.configMutex.Unlock()
	sc.config = config
}

// CalculateTechnicalScore computes the technical indicators score (0-40 points).
// Sub-components:
//   - Trend alignment (1h == 15m): 0-15 points
//   - Momentum (RSI): 0-10 points
//   - Volatility (ADX): 0-10 points
//   - Volume: 0-5 points (placeholder)
func (sc *ScoreCalculator) CalculateTechnicalScore(state *CoinState) ComponentScore {
	cs := NewComponentScore(DimensionTechnical, MaxTechnicalScore)
	sc.configMutex.RLock()
	technicalWeight := sc.config.TechnicalWeight
	sc.configMutex.RUnlock()
	cs.SetWeight(technicalWeight)

	if state == nil {
		return *cs
	}

	// 1. Trend Alignment (0-15 points)
	trendScore := sc.calculateTrendAlignment(state.Trend1H, state.Trend15M)
	cs.Details["trend_alignment"] = trendScore

	// 2. Momentum - RSI (0-10 points)
	momentumScore := sc.calculateMomentumScore(state.RSI)
	cs.Details["momentum"] = momentumScore

	// 3. Volatility - ADX (0-10 points)
	volatilityScore := sc.calculateVolatilityScore(state.ADX)
	cs.Details["volatility"] = volatilityScore

	// 4. Volume (0-5 points) - placeholder, can be enhanced later
	volumeScore := sc.calculateVolumeScore(state)
	cs.Details["volume"] = volumeScore

	// Sum sub-components
	totalScore := trendScore + momentumScore + volatilityScore + volumeScore
	cs.SetScore(totalScore)

	return *cs
}

// calculateTrendAlignment scores how well 1H and 15M trends align.
// Perfect alignment = 15 points, partial alignment = 7 points, conflict = 0 points
func (sc *ScoreCalculator) calculateTrendAlignment(trend1H, trend15M TrendDirection) float64 {
	// Both neutral or both empty = partial alignment
	if trend1H == TrendNeutral || trend1H == "" {
		if trend15M == TrendNeutral || trend15M == "" {
			return 7.0 // Both neutral
		}
		return 5.0 // One neutral, one directional
	}

	if trend15M == TrendNeutral || trend15M == "" {
		return 5.0 // One neutral, one directional
	}

	// Both have direction
	if trend1H == trend15M {
		return MaxTrendAlignmentScore // Perfect alignment = 15 points
	}

	// Opposing trends = no alignment points
	return 0.0
}

// calculateMomentumScore scores RSI position.
// Optimal RSI for entry: 40-60 range gets full points.
// Extended RSI (overbought/oversold) gets fewer points for new entries.
func (sc *ScoreCalculator) calculateMomentumScore(rsi float64) float64 {
	if rsi <= 0 {
		return 0 // No RSI data
	}

	// Optimal zone for entry: RSI 40-60
	if rsi >= 40 && rsi <= 60 {
		return MaxMomentumScore // 10 points
	}

	// Moderate zones: 30-40 and 60-70
	if (rsi >= 30 && rsi < 40) || (rsi > 60 && rsi <= 70) {
		return 7.0
	}

	// Extended zones: 20-30 and 70-80
	if (rsi >= 20 && rsi < 30) || (rsi > 70 && rsi <= 80) {
		return 4.0
	}

	// Extreme zones: < 20 or > 80
	// Still tradeable but risky for new entries
	return 2.0
}

// calculateVolatilityScore scores ADX (trend strength indicator).
// ADX > 25 indicates a trending market = good for trend strategies.
func (sc *ScoreCalculator) calculateVolatilityScore(adx float64) float64 {
	if adx <= 0 {
		return 0 // No ADX data
	}

	// Strong trend: ADX > 40
	if adx > 40 {
		return MaxVolatilityScore // 10 points
	}

	// Moderate trend: ADX 25-40
	if adx >= 25 {
		return 7.0
	}

	// Weak trend: ADX 20-25
	if adx >= 20 {
		return 4.0
	}

	// No trend / consolidation: ADX < 20
	// Still some points for range strategies
	return 2.0
}

// calculateVolumeScore is a placeholder for volume-based scoring.
// Can be enhanced when volume data is available in CoinState.
func (sc *ScoreCalculator) calculateVolumeScore(state *CoinState) float64 {
	// Placeholder: return middle value until volume data is integrated
	// Future: compare current volume to average volume
	return MaxVolumeScore / 2 // 2.5 points default
}

// CalculateContextScore computes the market context score (0-30 points).
// Sub-components:
//   - Market regime match: 0-10 points
//   - Timeframe alignment: 0-10 points
//   - BTC/market trend: 0-10 points
func (sc *ScoreCalculator) CalculateContextScore(state *CoinState) ComponentScore {
	cs := NewComponentScore(DimensionContext, MaxContextScore)
	sc.configMutex.RLock()
	contextWeight := sc.config.ContextWeight
	sc.configMutex.RUnlock()
	cs.SetWeight(contextWeight)

	if state == nil {
		return *cs
	}

	// 1. Market Regime Match (0-10 points)
	regimeScore := sc.calculateRegimeMatchScore(state.Regime, state.ActiveStrategy)
	cs.Details["regime_match"] = regimeScore

	// 2. Timeframe Alignment (0-10 points)
	timeframeScore := sc.calculateTimeframeAlignmentScore(state.Trend1H, state.Trend15M)
	cs.Details["timeframe_alignment"] = timeframeScore

	// 3. BTC Trend (0-10 points) - uses state if it's BTC, otherwise placeholder
	btcScore := sc.calculateBTCTrendScore(state)
	cs.Details["btc_trend"] = btcScore

	// Sum sub-components
	totalScore := regimeScore + timeframeScore + btcScore
	cs.SetScore(totalScore)

	return *cs
}

// calculateRegimeMatchScore scores how well the active strategy matches the market regime.
func (sc *ScoreCalculator) calculateRegimeMatchScore(regime MarketRegime, strategy string) float64 {
	if strategy == "" || regime == "" {
		return MaxRegimeMatchScore / 2 // 5 points default when unknown
	}

	// Define strategy-regime compatibility
	// These mappings should be configurable in production
	switch regime {
	case RegimeTrending:
		// Trend-following strategies work best
		if containsStrategy(strategy, "trend", "momentum", "breakout") {
			return MaxRegimeMatchScore // 10 points
		}
		if containsStrategy(strategy, "swing") {
			return 7.0
		}
		return 3.0 // Range strategies in trending = poor match

	case RegimeRanging:
		// Mean-reversion strategies work best
		if containsStrategy(strategy, "range", "reversion", "scalp") {
			return MaxRegimeMatchScore
		}
		if containsStrategy(strategy, "swing") {
			return 7.0
		}
		return 3.0

	case RegimeVolatile:
		// Volatile markets need careful strategies
		if containsStrategy(strategy, "volatility", "hedge", "conservative") {
			return MaxRegimeMatchScore
		}
		if containsStrategy(strategy, "scalp") {
			return 6.0
		}
		return 2.0 // Most strategies struggle in high volatility

	case RegimeConsolidating:
		// Consolidating markets - wait for breakout or use range strategies
		if containsStrategy(strategy, "range", "accumulation") {
			return 8.0
		}
		if containsStrategy(strategy, "breakout") {
			return 5.0 // Waiting for breakout confirmation
		}
		return 4.0

	default:
		return MaxRegimeMatchScore / 2
	}
}

// containsStrategy checks if the strategy name contains any of the keywords (case-insensitive).
func containsStrategy(strategy string, keywords ...string) bool {
	strategyLower := toLowerCase(strategy)
	for _, kw := range keywords {
		if containsSubstring(strategyLower, kw) {
			return true
		}
	}
	return false
}

// toLowerCase converts a string to lowercase without importing strings package
func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// containsSubstring checks if s contains substr (simple implementation)
func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// calculateTimeframeAlignmentScore scores multi-timeframe confirmation.
func (sc *ScoreCalculator) calculateTimeframeAlignmentScore(trend1H, trend15M TrendDirection) float64 {
	// Similar to trend alignment but focused on confirmation
	if trend1H == "" && trend15M == "" {
		return 0 // No data
	}

	// Both have direction and match = strong confirmation
	if trend1H != "" && trend1H != TrendNeutral &&
		trend15M != "" && trend15M != TrendNeutral {
		if trend1H == trend15M {
			return MaxTimeframeAlignScore // 10 points
		}
		return 0 // Conflicting signals = no confirmation
	}

	// One has direction, other neutral = partial confirmation
	if (trend1H != "" && trend1H != TrendNeutral) ||
		(trend15M != "" && trend15M != TrendNeutral) {
		return 5.0
	}

	// Both neutral
	return 3.0
}

// calculateBTCTrendScore scores how BTC trend supports the position.
// For non-BTC pairs, this is a placeholder until BTC state is available.
func (sc *ScoreCalculator) calculateBTCTrendScore(state *CoinState) float64 {
	// If this IS BTC, use its own trend
	if state.Symbol == "BTCUSDT" || state.Symbol == "BTCBUSD" {
		if state.Trend1H == TrendBullish {
			return MaxBTCTrendScore
		}
		if state.Trend1H == TrendNeutral {
			return 5.0
		}
		// Bearish
		return 2.0
	}

	// For other pairs, placeholder score until we can access BTC state
	// In production, this would query BTC's state and compare
	return MaxBTCTrendScore / 2 // 5 points default
}

// CalculateLLMScore computes the AI/LLM confidence score (0-20 points).
// Simply maps llmConfidence (0-100) to (0-20) range.
func (sc *ScoreCalculator) CalculateLLMScore(llmConfidence float64) ComponentScore {
	cs := NewComponentScore(DimensionLLM, MaxLLMScore)
	sc.configMutex.RLock()
	llmWeight := sc.config.LLMWeight
	sc.configMutex.RUnlock()
	cs.SetWeight(llmWeight)

	// Clamp input to valid range
	if llmConfidence < 0 {
		llmConfidence = 0
	}
	if llmConfidence > 100 {
		llmConfidence = 100
	}

	// Map 0-100 to 0-20
	score := (llmConfidence / 100) * MaxLLMScore
	cs.Details["llm_confidence"] = llmConfidence
	cs.Details["mapped_score"] = score
	cs.SetScore(score)

	return *cs
}

// CalculateHistoryScore computes the historical performance score (0-10 points).
// Sub-components:
//   - Symbol win rate: 0-5 points
//   - Strategy win rate: 0-5 points
func (sc *ScoreCalculator) CalculateHistoryScore(symbolWinRate, strategyWinRate float64) ComponentScore {
	cs := NewComponentScore(DimensionHistory, MaxHistoryScore)
	sc.configMutex.RLock()
	historyWeight := sc.config.HistoryWeight
	sc.configMutex.RUnlock()
	cs.SetWeight(historyWeight)

	// 1. Symbol Win Rate (0-5 points)
	symbolScore := sc.mapWinRateToScore(symbolWinRate, MaxSymbolWinRateScore)
	cs.Details["symbol_win_rate"] = symbolWinRate
	cs.Details["symbol_score"] = symbolScore

	// 2. Strategy Win Rate (0-5 points)
	strategyScore := sc.mapWinRateToScore(strategyWinRate, MaxStrategyWinRateScore)
	cs.Details["strategy_win_rate"] = strategyWinRate
	cs.Details["strategy_score"] = strategyScore

	// Sum sub-components
	totalScore := symbolScore + strategyScore
	cs.SetScore(totalScore)

	return *cs
}

// mapWinRateToScore converts a win rate (0-100%) to score points.
// Uses a linear mapping with a minimum threshold.
func (sc *ScoreCalculator) mapWinRateToScore(winRate, maxScore float64) float64 {
	// Clamp to valid range
	if winRate < 0 {
		winRate = 0
	}
	if winRate > 100 {
		winRate = 100
	}

	// Linear mapping from 0-100% to 0-maxScore
	// Could be enhanced with non-linear mapping (e.g., bonus for high win rates)
	return (winRate / 100) * maxScore
}

// CalculateFinalScore computes the complete score breakdown for a coin.
// This is the main entry point for score calculation.
func (sc *ScoreCalculator) CalculateFinalScore(
	state *CoinState,
	llmConfidence float64,
	symbolWinRate float64,
	strategyWinRate float64,
) *ScoreBreakdown {
	if state == nil {
		return nil
	}

	breakdown := NewScoreBreakdown(state.Symbol)

	// Calculate each component
	technicalScore := sc.CalculateTechnicalScore(state)
	contextScore := sc.CalculateContextScore(state)
	llmScore := sc.CalculateLLMScore(llmConfidence)
	historyScore := sc.CalculateHistoryScore(symbolWinRate, strategyWinRate)

	// Add to breakdown
	breakdown.AddComponent(technicalScore)
	breakdown.AddComponent(contextScore)
	breakdown.AddComponent(llmScore)
	breakdown.AddComponent(historyScore)

	// Calculate final score - get threshold with lock
	sc.configMutex.RLock()
	threshold := sc.config.MinThreshold
	sc.configMutex.RUnlock()
	breakdown.CalculateFinal(threshold)

	// Store in history
	sc.addToHistory(state.Symbol, *breakdown)

	return breakdown
}

// CalculateAndUpdateState computes scores and updates the CoinState in Redis.
// This integrates with the StateManager for delta updates.
func (sc *ScoreCalculator) CalculateAndUpdateState(
	ctx context.Context,
	userID string,
	state *CoinState,
	llmConfidence float64,
	symbolWinRate float64,
	strategyWinRate float64,
) (*ScoreBreakdown, error) {
	if sc.sm == nil {
		return nil, fmt.Errorf("StateManager is not configured")
	}

	breakdown := sc.CalculateFinalScore(state, llmConfidence, symbolWinRate, strategyWinRate)
	if breakdown == nil {
		return nil, fmt.Errorf("failed to calculate score: state is nil")
	}

	// Extract integer scores for CoinState (0-100 scale stored in int fields)
	// The component scores need to be converted to percentages for the int fields
	technicalPct := 0
	contextPct := 0
	llmPct := 0
	historyPct := 0

	if comp := breakdown.GetComponent(DimensionTechnical); comp != nil {
		technicalPct = int(math.Round(comp.Percentage()))
	}
	if comp := breakdown.GetComponent(DimensionContext); comp != nil {
		contextPct = int(math.Round(comp.Percentage()))
	}
	if comp := breakdown.GetComponent(DimensionLLM); comp != nil {
		llmPct = int(math.Round(comp.Percentage()))
	}
	if comp := breakdown.GetComponent(DimensionHistory); comp != nil {
		historyPct = int(math.Round(comp.Percentage()))
	}
	finalNormalized := int(math.Round(breakdown.NormalizedScore))

	// Update Redis via StateManager
	err := sc.sm.UpdateScores(ctx, userID, state.Symbol,
		technicalPct, contextPct, llmPct, historyPct, finalNormalized)
	if err != nil {
		return breakdown, fmt.Errorf("failed to update scores in Redis: %w", err)
	}

	// Update local CoinState object
	state.ScoreTechnical = technicalPct
	state.ScoreContext = contextPct
	state.ScoreLLM = llmPct
	state.ScoreHistory = historyPct
	state.ScoreFinal = finalNormalized

	return breakdown, nil
}

// PassesThreshold checks if a score meets the minimum threshold.
func (sc *ScoreCalculator) PassesThreshold(score float64) bool {
	sc.configMutex.RLock()
	defer sc.configMutex.RUnlock()
	return score >= sc.config.MinThreshold
}

// GetThreshold returns the current minimum threshold.
func (sc *ScoreCalculator) GetThreshold() float64 {
	sc.configMutex.RLock()
	defer sc.configMutex.RUnlock()
	return sc.config.MinThreshold
}

// SetThreshold updates the minimum threshold.
func (sc *ScoreCalculator) SetThreshold(threshold float64) {
	if threshold < 0 {
		threshold = 0
	}
	if threshold > MaxTotalScore {
		threshold = MaxTotalScore
	}
	sc.configMutex.Lock()
	defer sc.configMutex.Unlock()
	sc.config.MinThreshold = threshold
}

// addToHistory adds a score breakdown to the symbol's history.
func (sc *ScoreCalculator) addToHistory(symbol string, breakdown ScoreBreakdown) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	history, exists := sc.history[symbol]
	if !exists {
		history = NewScoreHistory(symbol, 100) // Keep last 100 scores
		sc.history[symbol] = history
	}

	history.Add(breakdown)
}

// GetHistory returns the score history for a symbol.
func (sc *ScoreCalculator) GetHistory(symbol string) *ScoreHistory {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	history, exists := sc.history[symbol]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	histCopy := &ScoreHistory{
		Symbol:  history.Symbol,
		MaxSize: history.MaxSize,
		Scores:  make([]ScoreBreakdown, len(history.Scores)),
	}
	copy(histCopy.Scores, history.Scores)

	return histCopy
}

// GetAverageScore returns the average score for a symbol from history.
func (sc *ScoreCalculator) GetAverageScore(symbol string) float64 {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	history, exists := sc.history[symbol]
	if !exists {
		return 0
	}

	return history.Average()
}

// ClearHistory removes all score history for a symbol.
func (sc *ScoreCalculator) ClearHistory(symbol string) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	if history, exists := sc.history[symbol]; exists {
		history.Clear()
	}
}

// ClearAllHistory removes all score history.
func (sc *ScoreCalculator) ClearAllHistory() {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.history = make(map[string]*ScoreHistory)
}

// ScoreFromBreakdown creates a score map for use with StateManager.UpdateCoinState
func (sc *ScoreCalculator) ScoreFromBreakdown(breakdown *ScoreBreakdown) map[string]interface{} {
	if breakdown == nil {
		return nil
	}

	// Extract component percentages
	var techPct, ctxPct, llmPct, histPct float64
	for _, comp := range breakdown.Components {
		switch comp.Dimension {
		case DimensionTechnical:
			techPct = comp.Percentage()
		case DimensionContext:
			ctxPct = comp.Percentage()
		case DimensionLLM:
			llmPct = comp.Percentage()
		case DimensionHistory:
			histPct = comp.Percentage()
		}
	}

	return map[string]interface{}{
		"score_technical": strconv.Itoa(int(math.Round(techPct))),
		"score_context":   strconv.Itoa(int(math.Round(ctxPct))),
		"score_llm":       strconv.Itoa(int(math.Round(llmPct))),
		"score_history":   strconv.Itoa(int(math.Round(histPct))),
		"score_final":     strconv.Itoa(int(math.Round(breakdown.NormalizedScore))),
		"last_updated":    strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
}
