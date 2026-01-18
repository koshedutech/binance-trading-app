// Package strategies provides trading strategy implementations for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.10: Breakout Strategy
package strategies

import (
	"sync"

	"binance-trading-bot/internal/decision"
)

// BreakoutStrategy implements the Strategy interface for breakout trading.
// This strategy detects and trades breakouts in volatile markets when price
// breaks key levels with volume confirmation.
//
// Entry Conditions:
//   - ATR expansion (above average - indicating volatility)
//   - Volume spike (> 2x average)
//   - Price breaks key level (support/resistance)
//   - Momentum confirmation (RSI showing strength in breakout direction)
//
// Exit Conditions:
//   - Failed breakout (price returns inside range)
//   - Target reached (1.5-2x ATR from entry)
//   - Volume dies down (losing momentum)
type BreakoutStrategy struct {
	// mu protects concurrent access to config
	mu sync.RWMutex
	// config holds the strategy-specific settings
	config *BreakoutConfig
}

// BreakoutConfig contains configurable parameters for the breakout strategy.
type BreakoutConfig struct {
	// ATR thresholds for volatility expansion
	ATRExpansionMultiplier float64 // ATR must be >= average * multiplier for breakout (default: 1.2)
	ATRTargetMultiplier    float64 // Target profit as ATR multiple (default: 1.5)

	// Volume thresholds for confirmation
	VolumeSpikeMultiplier float64 // Volume must be >= average * multiplier (default: 2.0)
	VolumeDeclineRatio    float64 // Volume below this ratio of average triggers exit (default: 0.7)

	// RSI thresholds for momentum confirmation
	RSIMomentumBullish float64 // RSI above this confirms bullish momentum (default: 55)
	RSIMomentumBearish float64 // RSI below this confirms bearish momentum (default: 45)

	// Failed breakout detection (price returning inside range)
	FailedBreakoutPct float64 // Price retracement % to consider failed (default: 0.5)

	// Scoring weights for technical components (0-40 total)
	ATRExpansionWeight    float64 // Weight for ATR expansion scoring (default: 12)
	VolumeSpikeWeight     float64 // Weight for volume spike scoring (default: 12)
	KeyLevelBreakWeight   float64 // Weight for key level break scoring (default: 10)
	MomentumConfirmWeight float64 // Weight for momentum confirmation (default: 6)

	// Scoring weights for context components (0-30 total)
	RegimeMatchWeight    float64 // Weight for regime match (default: 15)
	TimeframeAlignWeight float64 // Weight for timeframe alignment (default: 15)

	// Entry score threshold (default: 55)
	EntryScoreThreshold float64 // Minimum final score to recommend entry
}

// DefaultBreakoutConfig returns the default configuration for breakout strategy.
func DefaultBreakoutConfig() *BreakoutConfig {
	return &BreakoutConfig{
		ATRExpansionMultiplier: 1.2,
		ATRTargetMultiplier:    1.5,
		VolumeSpikeMultiplier:  2.0,
		VolumeDeclineRatio:     0.7,
		RSIMomentumBullish:     55.0,
		RSIMomentumBearish:     45.0,
		FailedBreakoutPct:      0.5,
		ATRExpansionWeight:     12.0,
		VolumeSpikeWeight:      12.0,
		KeyLevelBreakWeight:    10.0,
		MomentumConfirmWeight:  6.0,
		RegimeMatchWeight:      15.0,
		TimeframeAlignWeight:   15.0,
		EntryScoreThreshold:    55.0,
	}
}

// NewBreakoutStrategy creates a new BreakoutStrategy with default configuration.
func NewBreakoutStrategy() *BreakoutStrategy {
	return &BreakoutStrategy{
		config: DefaultBreakoutConfig(),
	}
}

// NewBreakoutStrategyWithConfig creates a new BreakoutStrategy with custom configuration.
func NewBreakoutStrategyWithConfig(config *BreakoutConfig) *BreakoutStrategy {
	if config == nil {
		config = DefaultBreakoutConfig()
	}
	return &BreakoutStrategy{
		config: config,
	}
}

// Name returns the unique identifier for this strategy.
func (bs *BreakoutStrategy) Name() string {
	return "breakout"
}

// Description returns a human-readable explanation of the strategy.
func (bs *BreakoutStrategy) Description() string {
	return "Detects and trades breakouts in volatile markets when price breaks key levels with ATR expansion and volume confirmation. Best for volatile markets with high ATR."
}

// SupportedRegimes returns the market regimes this strategy works best with.
func (bs *BreakoutStrategy) SupportedRegimes() []decision.MarketRegime {
	return []decision.MarketRegime{decision.RegimeVolatile}
}

// RequiredIndicators returns the list of indicators needed for this strategy.
func (bs *BreakoutStrategy) RequiredIndicators() []string {
	return []string{
		"atr",
		"volume",
		"momentum",
		"rsi",
		"ema_21",
		"trend_15m",
		"trend_1h",
	}
}

// CalculateScore evaluates the coin state and returns a strategy-specific score.
// The score breakdown follows the additive scoring formula:
// - Technical Score (0-40): ATR expansion, Volume spike, Key level break, Momentum
// - Context Score (0-30): Regime match, Timeframe alignment
// - LLM Score (0-20): Not calculated here (provided externally)
// - History Score (0-10): Not calculated here (provided externally)
// Thread-safe: holds read lock during calculation to ensure config consistency.
func (bs *BreakoutStrategy) CalculateScore(state *decision.CoinState) decision.StrategyScore {
	// Hold read lock during entire calculation for config consistency
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	score := decision.NewStrategyScore(bs.Name())
	entryConditions := bs.getEntryConditionsLocked()
	score.EntryConditionsTotal = len(entryConditions)

	if state == nil {
		score.Recommendation = decision.RecommendAvoid
		score.AddBlockingReason(*decision.NewBlockingReason(
			decision.ReasonRegimeMismatch,
			decision.CategoryHardBlock,
			"Nil state provided",
		))
		return *score
	}

	// Calculate Technical Score (0-40)
	technicalScore := bs.calculateTechnicalScore(state, score)
	score.TechnicalScore = technicalScore

	// Calculate Context Score (0-30)
	contextScore := bs.calculateContextScore(state, score)
	score.ContextScore = contextScore

	// LLM and History scores are provided externally
	// For now, use the values from CoinState if available
	score.LLMScore = float64(state.ScoreLLM) * 0.2        // Scale from 0-100 to 0-20
	score.HistoryScore = float64(state.ScoreHistory) * 0.1 // Scale from 0-100 to 0-10

	// Calculate final score
	score.CalculateFinal()

	// Evaluate entry/exit conditions
	bs.evaluateConditionsLocked(state, score, entryConditions)

	// Set confidence based on final score and conditions met
	if score.EntryConditionsTotal > 0 {
		conditionRatio := float64(score.EntryConditionsMet) / float64(score.EntryConditionsTotal)
		score.Confidence = (score.FinalScore + conditionRatio*100) / 2
	} else {
		score.Confidence = score.FinalScore
	}
	// Ensure confidence is capped at 100
	if score.Confidence > 100 {
		score.Confidence = 100
	}

	return *score
}

// calculateTechnicalScore computes the technical indicator score (0-40).
func (bs *BreakoutStrategy) calculateTechnicalScore(state *decision.CoinState, score *decision.StrategyScore) float64 {
	var technicalScore float64

	// ATR Expansion Score (0-12) - Higher ATR vs average indicates volatility
	atrScore := bs.scoreATRExpansion(state.ATR, state.ATRAverage)
	score.SetComponentDetail("atr_expansion_score", atrScore)
	technicalScore += atrScore

	// Volume Spike Score (0-12)
	// Note: CoinState doesn't have direct volume fields
	// In production, volume data would come from CoinState
	volumeScore := bs.scoreVolumeSpike(state)
	score.SetComponentDetail("volume_spike_score", volumeScore)
	technicalScore += volumeScore

	// Key Level Break Score (0-10)
	// Uses price distance from EMA21 as proxy for breakout level
	keyLevelScore := bs.scoreKeyLevelBreak(state)
	score.SetComponentDetail("key_level_break_score", keyLevelScore)
	technicalScore += keyLevelScore

	// Momentum Confirmation Score (0-6)
	momentumScore := bs.scoreMomentumConfirmation(state.RSI, state.Trend15M)
	score.SetComponentDetail("momentum_confirm_score", momentumScore)
	technicalScore += momentumScore

	return technicalScore
}

// scoreATRExpansion calculates ATR expansion contribution to technical score.
// For breakouts, HIGHER ATR (above average) is better - indicates increased volatility.
func (bs *BreakoutStrategy) scoreATRExpansion(atr, atrAverage float64) float64 {
	maxScore := bs.config.ATRExpansionWeight

	// Need both ATR and ATRAverage to calculate expansion
	if atr <= 0 || atrAverage <= 0 {
		// If we have ATR but no average, give partial score based on ATR alone
		if atr > 0 {
			return maxScore * 0.5 // Baseline when average unavailable
		}
		return 0
	}

	// Calculate ATR expansion ratio
	expansionRatio := atr / atrAverage

	// ATR scoring for breakout (higher is better):
	// < 1.0: Below average volatility (0-3 points) - not good for breakouts
	// 1.0-1.2: Normal volatility (3-6 points) - borderline
	// 1.2-1.5: Elevated volatility (6-9 points) - good for breakouts
	// 1.5-2.0: High volatility (9-11 points) - excellent for breakouts
	// > 2.0: Very high volatility (11-12 points) - ideal, but be cautious

	if expansionRatio < 1.0 {
		// Below average - not ideal for breakouts
		return maxScore * (0.25 * expansionRatio)
	} else if expansionRatio < 1.2 {
		// Normal range approaching breakout threshold
		return maxScore * (0.25 + 0.25*(expansionRatio-1.0)/0.2)
	} else if expansionRatio < 1.5 {
		// Good breakout volatility
		return maxScore * (0.5 + 0.25*(expansionRatio-1.2)/0.3)
	} else if expansionRatio < 2.0 {
		// Excellent breakout volatility
		return maxScore * (0.75 + 0.17*(expansionRatio-1.5)/0.5)
	}
	// Very high volatility - near maximum score
	return maxScore * 0.95
}

// scoreVolumeSpike calculates volume contribution to technical score.
// For breakouts, volume spike confirms the move.
// Note: CoinState doesn't have direct volume fields, using RSI as proxy for now.
func (bs *BreakoutStrategy) scoreVolumeSpike(state *decision.CoinState) float64 {
	maxScore := bs.config.VolumeSpikeWeight

	// Without actual volume data, we use ATR expansion as a volatility proxy
	// and RSI extremes as a momentum proxy (which often correlates with volume)
	// In a full implementation, volume data would come from state

	// Use ATR expansion as proxy for market activity
	if state.ATR > 0 && state.ATRAverage > 0 {
		expansionRatio := state.ATR / state.ATRAverage
		if expansionRatio >= bs.config.ATRExpansionMultiplier {
			// High ATR often correlates with volume spikes
			return maxScore * 0.7
		} else if expansionRatio >= 1.0 {
			return maxScore * 0.5
		}
	}

	// Baseline when no data available
	return maxScore * 0.4
}

// scoreKeyLevelBreak calculates key level break contribution to technical score.
// Uses price distance from EMA21 and trend direction to assess breakout.
func (bs *BreakoutStrategy) scoreKeyLevelBreak(state *decision.CoinState) float64 {
	maxScore := bs.config.KeyLevelBreakWeight

	if state.EMA21 <= 0 || state.Price <= 0 {
		return maxScore * 0.3 // Baseline when EMA unavailable
	}

	// Calculate price distance from EMA21 as percentage
	distance := (state.Price - state.EMA21) / state.EMA21 * 100
	absDistance := distance
	if absDistance < 0 {
		absDistance = -absDistance
	}

	// Key level break scoring:
	// Price significantly above/below EMA indicates potential breakout
	// 0-0.5%: Near EMA - no breakout (0-2 points)
	// 0.5-1.5%: Moderate distance - potential breakout (2-5 points)
	// 1.5-3%: Good breakout distance (5-8 points)
	// 3-5%: Strong breakout (8-10 points)
	// > 5%: Very extended - may be overextended (8 points)

	// Check if direction aligns with trend
	directionAligned := false
	if distance > 0 && (state.Trend15M == decision.TrendBullish || state.Trend1H == decision.TrendBullish) {
		directionAligned = true
	} else if distance < 0 && (state.Trend15M == decision.TrendBearish || state.Trend1H == decision.TrendBearish) {
		directionAligned = true
	}

	var baseScore float64
	if absDistance < 0.5 {
		baseScore = maxScore * (0.2 * absDistance / 0.5)
	} else if absDistance < 1.5 {
		baseScore = maxScore * (0.2 + 0.3*(absDistance-0.5)/1.0)
	} else if absDistance < 3.0 {
		baseScore = maxScore * (0.5 + 0.3*(absDistance-1.5)/1.5)
	} else if absDistance < 5.0 {
		baseScore = maxScore * (0.8 + 0.2*(absDistance-3.0)/2.0)
	} else {
		// Overextended - cap the score
		baseScore = maxScore * 0.8
	}

	// Bonus for direction alignment
	if directionAligned {
		baseScore *= 1.1
		if baseScore > maxScore {
			baseScore = maxScore
		}
	}

	return baseScore
}

// scoreMomentumConfirmation calculates momentum contribution to technical score.
// RSI should confirm the breakout direction.
func (bs *BreakoutStrategy) scoreMomentumConfirmation(rsi float64, trend15m decision.TrendDirection) float64 {
	maxScore := bs.config.MomentumConfirmWeight

	if rsi <= 0 || rsi > 100 {
		return 0
	}

	// Momentum confirmation scoring:
	// For bullish breakout (trend up): RSI > 55 confirms momentum
	// For bearish breakout (trend down): RSI < 45 confirms momentum
	// Neutral zone (45-55): Weak momentum signal

	switch trend15m {
	case decision.TrendBullish:
		if rsi >= bs.config.RSIMomentumBullish {
			// Strong bullish momentum confirmation
			if rsi >= 60 {
				return maxScore * 0.9
			}
			return maxScore * (0.6 + 0.3*(rsi-55)/5)
		}
		// Weak bullish confirmation
		return maxScore * (rsi / 55) * 0.5

	case decision.TrendBearish:
		if rsi <= bs.config.RSIMomentumBearish {
			// Strong bearish momentum confirmation
			if rsi <= 40 {
				return maxScore * 0.9
			}
			return maxScore * (0.6 + 0.3*(45-rsi)/5)
		}
		// Weak bearish confirmation
		return maxScore * ((100 - rsi) / 55) * 0.5

	default: // Neutral
		// Look for divergence from neutral as potential breakout setup
		distFromCenter := rsi - 50
		if distFromCenter < 0 {
			distFromCenter = -distFromCenter
		}
		return maxScore * (0.3 + 0.2*(distFromCenter/50))
	}
}

// calculateContextScore computes the market context score (0-30).
func (bs *BreakoutStrategy) calculateContextScore(state *decision.CoinState, score *decision.StrategyScore) float64 {
	var contextScore float64

	// Regime Match Score (0-15)
	regimeScore := bs.scoreRegimeMatch(state.Regime)
	score.SetComponentDetail("regime_match_score", regimeScore)
	contextScore += regimeScore

	// Timeframe Alignment Score (0-15)
	alignmentScore := bs.scoreTimeframeAlignment(state.Trend15M, state.Trend1H)
	score.SetComponentDetail("timeframe_align_score", alignmentScore)
	contextScore += alignmentScore

	return contextScore
}

// scoreRegimeMatch calculates regime match contribution to context score.
func (bs *BreakoutStrategy) scoreRegimeMatch(regime decision.MarketRegime) float64 {
	maxScore := bs.config.RegimeMatchWeight

	switch regime {
	case decision.RegimeVolatile:
		return maxScore // Perfect match - breakouts thrive in volatility
	case decision.RegimeConsolidating:
		return maxScore * 0.6 // Good - consolidation often precedes breakouts
	case decision.RegimeTrending:
		return maxScore * 0.4 // Can work for continuation breakouts
	case decision.RegimeRanging:
		return maxScore * 0.2 // Poor - ranging markets often see false breakouts
	default:
		return 0
	}
}

// scoreTimeframeAlignment calculates timeframe alignment contribution.
// For breakouts, aligned directional trends are ideal for confirmation.
func (bs *BreakoutStrategy) scoreTimeframeAlignment(trend15m, trend1h decision.TrendDirection) float64 {
	maxScore := bs.config.TimeframeAlignWeight

	// For breakouts, we want:
	// - Both aligned in direction: best (15 points) - confirmed breakout
	// - One directional, one neutral: good (10-12 points) - emerging breakout
	// - Both neutral: moderate (7-9 points) - potential breakout setup
	// - Opposite directions: risky (3-5 points) - potential false breakout

	// Full alignment in a directional trend
	if trend15m == trend1h {
		if trend15m == decision.TrendBullish || trend15m == decision.TrendBearish {
			return maxScore // Perfect alignment
		}
		// Both neutral - waiting for direction
		return maxScore * 0.55
	}

	// One neutral, one directional - emerging breakout
	if trend15m == decision.TrendNeutral || trend1h == decision.TrendNeutral {
		return maxScore * 0.7
	}

	// Opposite directions - higher risk of false breakout
	return maxScore * 0.25
}

// evaluateConditionsLocked checks entry and exit conditions.
// Must be called with read lock held.
func (bs *BreakoutStrategy) evaluateConditionsLocked(state *decision.CoinState, score *decision.StrategyScore, entryConditions []decision.Condition) {
	exitConditions := bs.getExitConditionsLocked()

	// Evaluate entry conditions
	allEntryMet := true
	for _, cond := range entryConditions {
		value := bs.getIndicatorValue(state, cond.Indicator)
		if cond.Evaluate(value) {
			score.EntryConditionsMet++
		} else {
			allEntryMet = false
			score.AddFailedCondition(cond.Name)
			if cond.BlockCategory != "" {
				score.AddBlockingReason(*decision.NewBlockingReason(
					decision.BlockingReasonCode(cond.Name),
					cond.BlockCategory,
					cond.Description,
				).WithValue(value).WithThreshold(cond.Threshold))
			}
		}
	}

	// Evaluate exit conditions
	exitTriggered := false
	for _, cond := range exitConditions {
		value := bs.getIndicatorValue(state, cond.Indicator)
		if cond.Evaluate(value) {
			exitTriggered = true
			break
		}
	}

	// Determine recommendation
	if exitTriggered {
		score.Recommendation = decision.RecommendExit
	} else if allEntryMet && score.FinalScore >= bs.config.EntryScoreThreshold {
		// Determine long or short based on trend direction
		if state.Trend15M == decision.TrendBullish || state.Trend1H == decision.TrendBullish {
			score.Recommendation = decision.RecommendEnterLong
		} else if state.Trend15M == decision.TrendBearish || state.Trend1H == decision.TrendBearish {
			score.Recommendation = decision.RecommendEnterShort
		} else {
			score.Recommendation = decision.RecommendHold
		}
	} else if score.IsBlocked() {
		score.Recommendation = decision.RecommendAvoid
	} else {
		score.Recommendation = decision.RecommendHold
	}
}

// getIndicatorValue retrieves the value for a specific indicator from CoinState.
func (bs *BreakoutStrategy) getIndicatorValue(state *decision.CoinState, indicator string) float64 {
	switch indicator {
	case "atr":
		return state.ATR
	case "atr_average":
		return state.ATRAverage
	case "rsi":
		return state.RSI
	case "ema_21":
		return state.EMA21
	case "price":
		return state.Price
	case "atr_expansion":
		// Returns 1.0 if ATR is expanded above average
		if state.ATR > 0 && state.ATRAverage > 0 {
			if state.ATR >= state.ATRAverage*bs.config.ATRExpansionMultiplier {
				return 1.0
			}
		}
		return 0.0
	case "volume_spike":
		// Placeholder - returns 1.0 based on ATR expansion proxy
		// In production, this would check actual volume data
		if state.ATR > 0 && state.ATRAverage > 0 {
			if state.ATR >= state.ATRAverage*bs.config.ATRExpansionMultiplier {
				return 1.0
			}
		}
		return 0.0
	case "momentum_confirmed":
		// Returns 1.0 if RSI confirms the breakout direction
		if state.RSI <= 0 || state.RSI > 100 {
			return 0.0
		}
		if state.Trend15M == decision.TrendBullish && state.RSI >= bs.config.RSIMomentumBullish {
			return 1.0
		}
		if state.Trend15M == decision.TrendBearish && state.RSI <= bs.config.RSIMomentumBearish {
			return 1.0
		}
		return 0.0
	case "failed_breakout":
		// Returns 1.0 if price has returned inside the range (failed breakout)
		// Uses EMA proximity as proxy - if price near EMA after breakout, may be failing
		if state.EMA21 > 0 && state.Price > 0 {
			distance := (state.Price - state.EMA21) / state.EMA21 * 100
			if distance < 0 {
				distance = -distance
			}
			if distance < bs.config.FailedBreakoutPct {
				return 1.0 // Price returned to mean - potential failed breakout
			}
		}
		return 0.0
	case "volume_declining":
		// Placeholder - in production would check actual volume
		// Returns 0.0 as we can't detect volume decline without volume data
		return 0.0
	default:
		return 0
	}
}

// GetEntryConditions returns the conditions that must be met to enter a position.
// Thread-safe: holds read lock to prevent race conditions with SetConfig.
func (bs *BreakoutStrategy) GetEntryConditions() []decision.Condition {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.getEntryConditionsLocked()
}

// getEntryConditionsLocked returns entry conditions without acquiring the lock.
// Must be called with read lock already held.
func (bs *BreakoutStrategy) getEntryConditionsLocked() []decision.Condition {
	return []decision.Condition{
		*decision.NewCondition(
			"atr_expansion",
			"ATR must be expanded above average (high volatility)",
			"atr_expansion",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(true).
			WithBlockCategory(decision.CategoryHardBlock),

		*decision.NewCondition(
			"volume_spike",
			"Volume must spike above 2x average (breakout confirmation)",
			"volume_spike",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false). // Soft requirement since we use proxy
			WithWeight(0.8).
			WithBlockCategory(decision.CategorySoftBlock),

		*decision.NewCondition(
			"momentum_confirmed",
			"RSI must confirm breakout direction momentum",
			"momentum_confirmed",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(true).
			WithBlockCategory(decision.CategoryHardBlock),
	}
}

// GetExitConditions returns the conditions that trigger position exit.
// Thread-safe: holds read lock to prevent race conditions with SetConfig.
func (bs *BreakoutStrategy) GetExitConditions() []decision.Condition {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.getExitConditionsLocked()
}

// getExitConditionsLocked returns exit conditions without acquiring the lock.
// Must be called with read lock already held.
func (bs *BreakoutStrategy) getExitConditionsLocked() []decision.Condition {
	return []decision.Condition{
		*decision.NewCondition(
			"failed_breakout",
			"Price has returned inside range (failed breakout)",
			"failed_breakout",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false),

		*decision.NewCondition(
			"volume_declining",
			"Volume has dropped below threshold (losing momentum)",
			"volume_declining",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false),
	}
}

// GetConfig returns the current strategy configuration.
// Note: Returns a copy to prevent external modification without SetConfig.
func (bs *BreakoutStrategy) GetConfig() *BreakoutConfig {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	// Return a shallow copy of the config
	configCopy := *bs.config
	return &configCopy
}

// SetConfig updates the strategy configuration.
// Thread-safe: uses mutex to prevent race conditions.
func (bs *BreakoutStrategy) SetConfig(config *BreakoutConfig) {
	if config != nil {
		bs.mu.Lock()
		bs.config = config
		bs.mu.Unlock()
	}
}

// init registers the strategy with the global registry on package load.
func init() {
	strategy := NewBreakoutStrategy()
	if err := decision.RegisterGlobalStrategy(strategy); err != nil {
		// Strategy may already be registered if init is called multiple times
		// This is not an error in normal operation
	}
}

// RegisterBreakoutStrategy explicitly registers the breakout strategy
// with the global registry. This can be called manually if package init is not used.
func RegisterBreakoutStrategy() error {
	return decision.RegisterGlobalStrategy(NewBreakoutStrategy())
}

// RegisterBreakoutStrategyWithConfig registers the strategy with custom config.
func RegisterBreakoutStrategyWithConfig(registry *decision.StrategyRegistry, config *BreakoutConfig) error {
	strategy := NewBreakoutStrategyWithConfig(config)
	return registry.RegisterStrategy(strategy)
}
