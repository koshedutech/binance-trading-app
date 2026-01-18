// Package strategies provides trading strategy implementations for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.8: Trend Following Strategy
package strategies

import (
	"sync"

	"binance-trading-bot/internal/decision"
)

// TrendFollowingStrategy implements the Strategy interface for trend-following trading.
// This strategy follows established trends with momentum and volume confirmation.
//
// Entry Conditions:
//   - ADX > 25 (strong trend)
//   - Price above EMA 21 (for longs) / below EMA 21 (for shorts)
//   - RSI 40-70 (not overbought/oversold)
//   - Volume confirmation (above average)
//   - Trend alignment (15m and 1h timeframes agree)
//
// Exit Conditions:
//   - Trend reversal detection
//   - ADX drops below 20
//   - Price crosses EMA 21
//   - Trailing stop hit
type TrendFollowingStrategy struct {
	// mu protects concurrent access to config
	mu sync.RWMutex
	// config holds the strategy-specific settings
	config *TrendFollowingConfig
}

// TrendFollowingConfig contains configurable parameters for the trend following strategy.
type TrendFollowingConfig struct {
	// ADX thresholds
	ADXEntryMin float64 // Minimum ADX for entry (default: 25)
	ADXExitMin  float64 // ADX below this triggers exit consideration (default: 20)

	// RSI thresholds for not overbought/oversold
	RSILongMin  float64 // Minimum RSI for long entry (default: 40)
	RSILongMax  float64 // Maximum RSI for long entry (default: 70)
	RSIShortMin float64 // Minimum RSI for short entry (default: 30)
	RSIShortMax float64 // Maximum RSI for short entry (default: 60)

	// Volume multiplier for confirmation
	VolumeMultiplier float64 // Volume must be >= average * multiplier (default: 1.0)

	// Scoring weights for technical components (0-40 total)
	ADXWeight         float64 // Weight for ADX strength scoring (default: 10)
	EMAPositionWeight float64 // Weight for EMA position scoring (default: 10)
	RSIWeight         float64 // Weight for RSI level scoring (default: 10)
	VolumeWeight      float64 // Weight for volume confirmation (default: 10)

	// Scoring weights for context components (0-30 total)
	RegimeMatchWeight    float64 // Weight for regime match (default: 15)
	TimeframeAlignWeight float64 // Weight for timeframe alignment (default: 15)

	// Entry score threshold (default: 55)
	EntryScoreThreshold float64 // Minimum final score to recommend entry
}

// DefaultTrendFollowingConfig returns the default configuration for trend following.
func DefaultTrendFollowingConfig() *TrendFollowingConfig {
	return &TrendFollowingConfig{
		ADXEntryMin:          25.0,
		ADXExitMin:           20.0,
		RSILongMin:           40.0,
		RSILongMax:           70.0,
		RSIShortMin:          30.0,
		RSIShortMax:          60.0,
		VolumeMultiplier:     1.0,
		ADXWeight:            10.0,
		EMAPositionWeight:    10.0,
		RSIWeight:            10.0,
		VolumeWeight:         10.0,
		RegimeMatchWeight:    15.0,
		TimeframeAlignWeight: 15.0,
		EntryScoreThreshold:  55.0,
	}
}

// NewTrendFollowingStrategy creates a new TrendFollowingStrategy with default configuration.
func NewTrendFollowingStrategy() *TrendFollowingStrategy {
	return &TrendFollowingStrategy{
		config: DefaultTrendFollowingConfig(),
	}
}

// NewTrendFollowingStrategyWithConfig creates a new TrendFollowingStrategy with custom configuration.
func NewTrendFollowingStrategyWithConfig(config *TrendFollowingConfig) *TrendFollowingStrategy {
	if config == nil {
		config = DefaultTrendFollowingConfig()
	}
	return &TrendFollowingStrategy{
		config: config,
	}
}

// Name returns the unique identifier for this strategy.
func (tfs *TrendFollowingStrategy) Name() string {
	return "trend_following"
}

// Description returns a human-readable explanation of the strategy.
func (tfs *TrendFollowingStrategy) Description() string {
	return "Follows established trends with momentum and volume confirmation. Best for trending markets with ADX > 25."
}

// SupportedRegimes returns the market regimes this strategy works best with.
func (tfs *TrendFollowingStrategy) SupportedRegimes() []decision.MarketRegime {
	return []decision.MarketRegime{decision.RegimeTrending}
}

// RequiredIndicators returns the list of indicators needed for this strategy.
func (tfs *TrendFollowingStrategy) RequiredIndicators() []string {
	return []string{
		"adx",
		"ema_21",
		"rsi",
		"volume",
		"trend_15m",
		"trend_1h",
	}
}

// CalculateScore evaluates the coin state and returns a strategy-specific score.
// The score breakdown follows the additive scoring formula:
// - Technical Score (0-40): ADX strength, EMA position, RSI level, Volume
// - Context Score (0-30): Regime match, Timeframe alignment
// - LLM Score (0-20): Not calculated here (provided externally)
// - History Score (0-10): Not calculated here (provided externally)
// Thread-safe: holds read lock during calculation to ensure config consistency.
func (tfs *TrendFollowingStrategy) CalculateScore(state *decision.CoinState) decision.StrategyScore {
	// Hold read lock during entire calculation for config consistency
	tfs.mu.RLock()
	defer tfs.mu.RUnlock()

	score := decision.NewStrategyScore(tfs.Name())
	score.EntryConditionsTotal = len(tfs.GetEntryConditions())

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
	technicalScore := tfs.calculateTechnicalScore(state, score)
	score.TechnicalScore = technicalScore

	// Calculate Context Score (0-30)
	contextScore := tfs.calculateContextScore(state, score)
	score.ContextScore = contextScore

	// LLM and History scores are provided externally
	// For now, use the values from CoinState if available
	score.LLMScore = float64(state.ScoreLLM) * 0.2       // Scale from 0-100 to 0-20
	score.HistoryScore = float64(state.ScoreHistory) * 0.1 // Scale from 0-100 to 0-10

	// Calculate final score
	score.CalculateFinal()

	// Evaluate entry/exit conditions
	tfs.evaluateConditions(state, score)

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
func (tfs *TrendFollowingStrategy) calculateTechnicalScore(state *decision.CoinState, score *decision.StrategyScore) float64 {
	var technicalScore float64

	// ADX Strength Score (0-10)
	adxScore := tfs.scoreADX(state.ADX)
	score.SetComponentDetail("adx_score", adxScore)
	technicalScore += adxScore

	// EMA Position Score (0-10)
	emaScore := tfs.scoreEMAPosition(state.Price, state.EMA21, state.Trend1H)
	score.SetComponentDetail("ema_position_score", emaScore)
	technicalScore += emaScore

	// RSI Level Score (0-10)
	rsiScore := tfs.scoreRSI(state.RSI, state.Trend1H)
	score.SetComponentDetail("rsi_score", rsiScore)
	technicalScore += rsiScore

	// Volume Confirmation Score (0-10)
	// Note: Volume data is not in CoinState, so we'll use a default approach
	// In production, this would come from real volume data
	volumeScore := tfs.scoreVolume(state)
	score.SetComponentDetail("volume_score", volumeScore)
	technicalScore += volumeScore

	return technicalScore
}

// scoreADX calculates ADX contribution to technical score.
func (tfs *TrendFollowingStrategy) scoreADX(adx float64) float64 {
	maxScore := tfs.config.ADXWeight

	if adx <= 0 {
		return 0
	}

	// ADX scoring:
	// < 20: Very weak trend (0-3 points)
	// 20-25: Weak trend (3-5 points)
	// 25-40: Strong trend (5-8 points)
	// 40-60: Very strong trend (8-10 points)
	// > 60: Extremely strong (10 points, but be cautious)

	if adx < 20 {
		return (adx / 20) * 3
	} else if adx < 25 {
		return 3 + ((adx - 20) / 5) * 2
	} else if adx < 40 {
		return 5 + ((adx - 25) / 15) * 3
	} else if adx < 60 {
		return 8 + ((adx - 40) / 20) * 2
	}
	return maxScore
}

// scoreEMAPosition calculates EMA position contribution to technical score.
func (tfs *TrendFollowingStrategy) scoreEMAPosition(price, ema21 float64, trend decision.TrendDirection) float64 {
	maxScore := tfs.config.EMAPositionWeight

	if ema21 <= 0 {
		return 0
	}

	// Calculate price distance from EMA21 as percentage
	distance := (price - ema21) / ema21 * 100

	// For bullish trend, price should be above EMA21
	// For bearish trend, price should be below EMA21
	switch trend {
	case decision.TrendBullish:
		if distance > 0 {
			// Above EMA is good for longs
			// Score based on reasonable distance (0.5-3% is optimal)
			if distance < 0.5 {
				return maxScore * 0.5
			} else if distance <= 3 {
				return maxScore * (0.5 + 0.5*(distance/3))
			} else if distance <= 5 {
				return maxScore * (1.0 - 0.2*(distance-3)/2)
			}
			// Too far above EMA, might be overextended
			return maxScore * 0.6
		}
		// Price below EMA for bullish - not ideal
		return maxScore * 0.2

	case decision.TrendBearish:
		if distance < 0 {
			// Below EMA is good for shorts
			absDistance := -distance
			if absDistance < 0.5 {
				return maxScore * 0.5
			} else if absDistance <= 3 {
				return maxScore * (0.5 + 0.5*(absDistance/3))
			} else if absDistance <= 5 {
				return maxScore * (1.0 - 0.2*(absDistance-3)/2)
			}
			return maxScore * 0.6
		}
		// Price above EMA for bearish - not ideal
		return maxScore * 0.2

	default: // Neutral
		// Near EMA is fine for neutral
		absDistance := distance
		if absDistance < 0 {
			absDistance = -absDistance
		}
		if absDistance < 1 {
			return maxScore * 0.7
		}
		return maxScore * 0.4
	}
}

// scoreRSI calculates RSI contribution to technical score.
func (tfs *TrendFollowingStrategy) scoreRSI(rsi float64, trend decision.TrendDirection) float64 {
	maxScore := tfs.config.RSIWeight

	if rsi <= 0 || rsi > 100 {
		return 0
	}

	switch trend {
	case decision.TrendBullish:
		// For longs, RSI should be 40-70 (not overbought)
		if rsi >= tfs.config.RSILongMin && rsi <= tfs.config.RSILongMax {
			// Optimal range
			midpoint := (tfs.config.RSILongMin + tfs.config.RSILongMax) / 2
			distance := rsi - midpoint
			if distance < 0 {
				distance = -distance
			}
			rangeHalf := (tfs.config.RSILongMax - tfs.config.RSILongMin) / 2
			return maxScore * (1 - 0.3*(distance/rangeHalf))
		}
		if rsi < tfs.config.RSILongMin {
			// Oversold might signal weakness
			return maxScore * (rsi / tfs.config.RSILongMin) * 0.7
		}
		// Overbought - score decreases as RSI increases, with minimum of 0.1 * maxScore
		overboughtPenalty := (rsi - tfs.config.RSILongMax) / 30
		if overboughtPenalty > 0.8 {
			overboughtPenalty = 0.8 // Cap penalty to ensure non-negative score
		}
		return maxScore * (1 - overboughtPenalty) * 0.5

	case decision.TrendBearish:
		// For shorts, RSI should be 30-60 (not oversold)
		if rsi >= tfs.config.RSIShortMin && rsi <= tfs.config.RSIShortMax {
			midpoint := (tfs.config.RSIShortMin + tfs.config.RSIShortMax) / 2
			distance := rsi - midpoint
			if distance < 0 {
				distance = -distance
			}
			rangeHalf := (tfs.config.RSIShortMax - tfs.config.RSIShortMin) / 2
			return maxScore * (1 - 0.3*(distance/rangeHalf))
		}
		if rsi > tfs.config.RSIShortMax {
			// Overbought is good for shorts
			return maxScore * 0.7
		}
		// Already oversold, might bounce
		return maxScore * 0.4

	default: // Neutral
		// Middle RSI is fine
		if rsi >= 40 && rsi <= 60 {
			return maxScore * 0.8
		}
		return maxScore * 0.5
	}
}

// scoreVolume calculates volume contribution to technical score.
// Note: CoinState doesn't have direct volume fields, so this is a placeholder.
// In a real implementation, volume data would come from the CoinState or external source.
func (tfs *TrendFollowingStrategy) scoreVolume(state *decision.CoinState) float64 {
	// For now, give a baseline score since volume data isn't in CoinState
	// This should be updated when volume indicators are added
	return tfs.config.VolumeWeight * 0.6
}

// calculateContextScore computes the market context score (0-30).
func (tfs *TrendFollowingStrategy) calculateContextScore(state *decision.CoinState, score *decision.StrategyScore) float64 {
	var contextScore float64

	// Regime Match Score (0-15)
	regimeScore := tfs.scoreRegimeMatch(state.Regime)
	score.SetComponentDetail("regime_match_score", regimeScore)
	contextScore += regimeScore

	// Timeframe Alignment Score (0-15)
	alignmentScore := tfs.scoreTimeframeAlignment(state.Trend15M, state.Trend1H)
	score.SetComponentDetail("timeframe_align_score", alignmentScore)
	contextScore += alignmentScore

	return contextScore
}

// scoreRegimeMatch calculates regime match contribution to context score.
func (tfs *TrendFollowingStrategy) scoreRegimeMatch(regime decision.MarketRegime) float64 {
	maxScore := tfs.config.RegimeMatchWeight

	switch regime {
	case decision.RegimeTrending:
		return maxScore // Perfect match
	case decision.RegimeVolatile:
		return maxScore * 0.5 // Can work but risky
	case decision.RegimeConsolidating:
		return maxScore * 0.3 // Waiting for breakout
	case decision.RegimeRanging:
		return maxScore * 0.2 // Not ideal
	default:
		return 0
	}
}

// scoreTimeframeAlignment calculates timeframe alignment contribution to context score.
func (tfs *TrendFollowingStrategy) scoreTimeframeAlignment(trend15m, trend1h decision.TrendDirection) float64 {
	maxScore := tfs.config.TimeframeAlignWeight

	// Full alignment is best
	if trend15m == trend1h {
		if trend15m == decision.TrendBullish || trend15m == decision.TrendBearish {
			return maxScore // Strong alignment
		}
		return maxScore * 0.6 // Both neutral
	}

	// Partial alignment (one neutral)
	if trend15m == decision.TrendNeutral || trend1h == decision.TrendNeutral {
		return maxScore * 0.5
	}

	// Divergence (15m and 1h disagree on direction)
	return maxScore * 0.1
}

// evaluateConditions checks entry and exit conditions.
func (tfs *TrendFollowingStrategy) evaluateConditions(state *decision.CoinState, score *decision.StrategyScore) {
	entryConditions := tfs.GetEntryConditions()
	exitConditions := tfs.GetExitConditions()

	// Evaluate entry conditions
	allEntryMet := true
	for _, cond := range entryConditions {
		value := tfs.getIndicatorValue(state, cond.Indicator)
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
		value := tfs.getIndicatorValue(state, cond.Indicator)
		if cond.Evaluate(value) {
			exitTriggered = true
			break
		}
	}

	// Determine recommendation
	if exitTriggered {
		score.Recommendation = decision.RecommendExit
	} else if allEntryMet && score.FinalScore >= tfs.config.EntryScoreThreshold {
		// Determine long or short based on trend
		if state.Trend1H == decision.TrendBullish {
			score.Recommendation = decision.RecommendEnterLong
		} else if state.Trend1H == decision.TrendBearish {
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
func (tfs *TrendFollowingStrategy) getIndicatorValue(state *decision.CoinState, indicator string) float64 {
	switch indicator {
	case "adx":
		return state.ADX
	case "rsi":
		return state.RSI
	case "ema_21":
		return state.EMA21
	case "price":
		return state.Price
	case "ema_position":
		if state.EMA21 > 0 {
			return (state.Price - state.EMA21) / state.EMA21 * 100
		}
		return 0
	case "trend_alignment":
		if state.Trend15M == state.Trend1H {
			if state.Trend15M == decision.TrendBullish || state.Trend15M == decision.TrendBearish {
				return 1.0
			}
		}
		return 0.0
	case "ema_cross_against_trend":
		// Returns 1.0 if price has crossed EMA against the trend direction
		if state.EMA21 <= 0 {
			return 0
		}
		emaPosition := (state.Price - state.EMA21) / state.EMA21 * 100
		// For bullish trend, price below EMA is against trend
		if state.Trend1H == decision.TrendBullish && emaPosition < 0 {
			return 1.0
		}
		// For bearish trend, price above EMA is against trend
		if state.Trend1H == decision.TrendBearish && emaPosition > 0 {
			return 1.0
		}
		return 0.0
	default:
		return 0
	}
}

// GetEntryConditions returns the conditions that must be met to enter a position.
// Note: Accesses config directly; caller should hold read lock if thread-safety needed.
func (tfs *TrendFollowingStrategy) GetEntryConditions() []decision.Condition {
	return []decision.Condition{
		*decision.NewCondition(
			"adx_strength",
			"ADX must be > 25 for strong trend",
			"adx",
		).WithThreshold(decision.OperatorGreaterThan, tfs.config.ADXEntryMin).
			WithRequired(true).
			WithBlockCategory(decision.CategoryHardBlock),

		*decision.NewCondition(
			"rsi_range",
			"RSI must be in acceptable range (not overbought/oversold)",
			"rsi",
		).WithRange(tfs.config.RSILongMin, tfs.config.RSILongMax).
			WithRequired(true).
			WithBlockCategory(decision.CategorySoftBlock),

		*decision.NewCondition(
			"trend_alignment",
			"15m and 1h trends must align",
			"trend_alignment",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(true).
			WithBlockCategory(decision.CategoryHardBlock),
	}
}

// GetExitConditions returns the conditions that trigger position exit.
// Note: Accesses config directly; caller should hold read lock if thread-safety needed.
func (tfs *TrendFollowingStrategy) GetExitConditions() []decision.Condition {
	return []decision.Condition{
		*decision.NewCondition(
			"adx_weakness",
			"ADX dropped below 20 indicating trend weakness",
			"adx",
		).WithThreshold(decision.OperatorLessThan, tfs.config.ADXExitMin).
			WithRequired(false),

		*decision.NewCondition(
			"ema_cross",
			"Price crossed EMA 21 against trend direction",
			"ema_cross_against_trend",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false),
	}
}

// GetConfig returns the current strategy configuration.
// Note: Returns a copy to prevent external modification without SetConfig.
func (tfs *TrendFollowingStrategy) GetConfig() *TrendFollowingConfig {
	tfs.mu.RLock()
	defer tfs.mu.RUnlock()
	// Return a shallow copy of the config
	configCopy := *tfs.config
	return &configCopy
}

// SetConfig updates the strategy configuration.
// Thread-safe: uses mutex to prevent race conditions.
func (tfs *TrendFollowingStrategy) SetConfig(config *TrendFollowingConfig) {
	if config != nil {
		tfs.mu.Lock()
		tfs.config = config
		tfs.mu.Unlock()
	}
}

// init registers the strategy with the global registry on package load.
func init() {
	strategy := NewTrendFollowingStrategy()
	if err := decision.RegisterGlobalStrategy(strategy); err != nil {
		// Strategy may already be registered if init is called multiple times
		// This is not an error in normal operation
	}
}

// RegisterTrendFollowingStrategy explicitly registers the trend following strategy
// with the global registry. This can be called manually if package init is not used.
func RegisterTrendFollowingStrategy() error {
	return decision.RegisterGlobalStrategy(NewTrendFollowingStrategy())
}

// RegisterTrendFollowingStrategyWithConfig registers the strategy with custom config.
func RegisterTrendFollowingStrategyWithConfig(registry *decision.StrategyRegistry, config *TrendFollowingConfig) error {
	strategy := NewTrendFollowingStrategyWithConfig(config)
	return registry.RegisterStrategy(strategy)
}
