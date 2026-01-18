// Package strategies provides trading strategy implementations for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.9: Mean Reversion Strategy
package strategies

import (
	"sync"

	"binance-trading-bot/internal/decision"
)

// MeanReversionStrategy implements the Strategy interface for mean reversion trading.
// This strategy trades reversals in ranging/sideways markets when price reaches extremes.
//
// Entry Conditions:
//   - ADX < 20 (no strong trend - ranging market)
//   - RSI < 30 (oversold) or RSI > 70 (overbought)
//   - Price at Bollinger Band extremes (near upper/lower band)
//   - Support/resistance proximity
//
// Exit Conditions:
//   - Return to mean (price near middle Bollinger Band / EMA21)
//   - RSI returns to neutral (40-60)
//   - Time-based exit (ranging trades shouldn't last long)
type MeanReversionStrategy struct {
	// mu protects concurrent access to config
	mu sync.RWMutex
	// config holds the strategy-specific settings
	config *MeanReversionConfig
}

// MeanReversionConfig contains configurable parameters for the mean reversion strategy.
type MeanReversionConfig struct {
	// ADX thresholds - ADX must be LOW for mean reversion (indicating no trend)
	ADXMaxEntry float64 // Maximum ADX for entry (default: 20 - no strong trend)
	ADXExitMax  float64 // ADX above this suggests trend forming, consider exit (default: 25)

	// RSI thresholds for overbought/oversold extremes
	RSIOversold   float64 // RSI below this is oversold, potential long (default: 30)
	RSIOverbought float64 // RSI above this is overbought, potential short (default: 70)
	RSINeutralMin float64 // RSI entering this range triggers exit (default: 40)
	RSINeutralMax float64 // RSI entering this range triggers exit (default: 60)

	// Bollinger Band position (as percentage distance from middle band)
	BBExtremePct float64 // Percentage of BB width for extreme position (default: 0.8 = 80%)
	BBMeanPct    float64 // Percentage of BB width considered "at mean" (default: 0.2 = 20%)

	// ATR for support/resistance proximity (as ATR multiplier)
	SRProximityATR float64 // Within this many ATRs of S/R is valid (default: 1.5)

	// Scoring weights for technical components (0-40 total)
	ADXWeight          float64 // Weight for ADX scoring (default: 10)
	RSIExtremeWeight   float64 // Weight for RSI extreme scoring (default: 12)
	BBPositionWeight   float64 // Weight for BB position scoring (default: 10)
	SRProximityWeight  float64 // Weight for S/R proximity (default: 8)

	// Scoring weights for context components (0-30 total)
	RegimeMatchWeight    float64 // Weight for regime match (default: 15)
	TimeframeAlignWeight float64 // Weight for timeframe alignment (default: 15)

	// Entry score threshold (default: 55)
	EntryScoreThreshold float64 // Minimum final score to recommend entry

	// Time-based exit (maximum position hold time in minutes)
	MaxHoldMinutes int64 // Maximum hold time before forced exit (default: 240 = 4 hours)
}

// DefaultMeanReversionConfig returns the default configuration for mean reversion.
func DefaultMeanReversionConfig() *MeanReversionConfig {
	return &MeanReversionConfig{
		ADXMaxEntry:          20.0,
		ADXExitMax:           25.0,
		RSIOversold:          30.0,
		RSIOverbought:        70.0,
		RSINeutralMin:        40.0,
		RSINeutralMax:        60.0,
		BBExtremePct:         0.8,
		BBMeanPct:            0.2,
		SRProximityATR:       1.5,
		ADXWeight:            10.0,
		RSIExtremeWeight:     12.0,
		BBPositionWeight:     10.0,
		SRProximityWeight:    8.0,
		RegimeMatchWeight:    15.0,
		TimeframeAlignWeight: 15.0,
		EntryScoreThreshold:  55.0,
		MaxHoldMinutes:       240,
	}
}

// NewMeanReversionStrategy creates a new MeanReversionStrategy with default configuration.
func NewMeanReversionStrategy() *MeanReversionStrategy {
	return &MeanReversionStrategy{
		config: DefaultMeanReversionConfig(),
	}
}

// NewMeanReversionStrategyWithConfig creates a new MeanReversionStrategy with custom configuration.
func NewMeanReversionStrategyWithConfig(config *MeanReversionConfig) *MeanReversionStrategy {
	if config == nil {
		config = DefaultMeanReversionConfig()
	}
	return &MeanReversionStrategy{
		config: config,
	}
}

// Name returns the unique identifier for this strategy.
func (mrs *MeanReversionStrategy) Name() string {
	return "mean_reversion"
}

// Description returns a human-readable explanation of the strategy.
func (mrs *MeanReversionStrategy) Description() string {
	return "Trades reversals when price reaches extremes in ranging markets. Best for ranging/sideways markets with ADX < 20."
}

// SupportedRegimes returns the market regimes this strategy works best with.
func (mrs *MeanReversionStrategy) SupportedRegimes() []decision.MarketRegime {
	return []decision.MarketRegime{decision.RegimeRanging}
}

// RequiredIndicators returns the list of indicators needed for this strategy.
func (mrs *MeanReversionStrategy) RequiredIndicators() []string {
	return []string{
		"adx",
		"rsi",
		"bollinger",   // Bollinger Bands (upper, middle, lower)
		"atr",         // For volatility assessment
		"ema_21",      // For mean reference
		"trend_15m",
		"trend_1h",
	}
}

// CalculateScore evaluates the coin state and returns a strategy-specific score.
// The score breakdown follows the additive scoring formula:
// - Technical Score (0-40): ADX weakness, RSI extremes, BB position, S/R proximity
// - Context Score (0-30): Regime match, Timeframe alignment
// - LLM Score (0-20): Not calculated here (provided externally)
// - History Score (0-10): Not calculated here (provided externally)
// Thread-safe: holds read lock during calculation to ensure config consistency.
func (mrs *MeanReversionStrategy) CalculateScore(state *decision.CoinState) decision.StrategyScore {
	// Hold read lock during entire calculation for config consistency
	mrs.mu.RLock()
	defer mrs.mu.RUnlock()

	score := decision.NewStrategyScore(mrs.Name())
	score.EntryConditionsTotal = len(mrs.getEntryConditionsLocked())

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
	technicalScore := mrs.calculateTechnicalScore(state, score)
	score.TechnicalScore = technicalScore

	// Calculate Context Score (0-30)
	contextScore := mrs.calculateContextScore(state, score)
	score.ContextScore = contextScore

	// LLM and History scores are provided externally
	// For now, use the values from CoinState if available
	score.LLMScore = float64(state.ScoreLLM) * 0.2        // Scale from 0-100 to 0-20
	score.HistoryScore = float64(state.ScoreHistory) * 0.1 // Scale from 0-100 to 0-10

	// Calculate final score
	score.CalculateFinal()

	// Evaluate entry/exit conditions
	mrs.evaluateConditions(state, score)

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
func (mrs *MeanReversionStrategy) calculateTechnicalScore(state *decision.CoinState, score *decision.StrategyScore) float64 {
	var technicalScore float64

	// ADX Weakness Score (0-10) - Lower ADX is better for mean reversion
	adxScore := mrs.scoreADX(state.ADX)
	score.SetComponentDetail("adx_score", adxScore)
	technicalScore += adxScore

	// RSI Extreme Score (0-12) - Look for oversold/overbought
	rsiScore := mrs.scoreRSIExtreme(state.RSI)
	score.SetComponentDetail("rsi_extreme_score", rsiScore)
	technicalScore += rsiScore

	// Bollinger Band Position Score (0-10)
	bbScore := mrs.scoreBBPosition(state)
	score.SetComponentDetail("bb_position_score", bbScore)
	technicalScore += bbScore

	// Support/Resistance Proximity Score (0-8)
	// Note: S/R data is not directly in CoinState, using ATR-based approximation
	srScore := mrs.scoreSRProximity(state)
	score.SetComponentDetail("sr_proximity_score", srScore)
	technicalScore += srScore

	return technicalScore
}

// scoreADX calculates ADX contribution to technical score.
// For mean reversion, LOWER ADX is better (no trend = good for reversion).
func (mrs *MeanReversionStrategy) scoreADX(adx float64) float64 {
	maxScore := mrs.config.ADXWeight

	if adx <= 0 {
		// Zero or negative ADX indicates missing/invalid data - return low score
		// to avoid false signals. Consistent with other indicator handling.
		return 0
	}

	// ADX scoring for mean reversion (inverse of trend following):
	// < 15: Perfect for mean reversion (10 points)
	// 15-20: Good for mean reversion (7-10 points)
	// 20-25: Borderline (4-7 points)
	// 25-35: Trend forming, risky (1-4 points)
	// > 35: Too trendy, avoid (0-1 points)

	if adx < 15 {
		return maxScore // Perfect - no trend
	} else if adx < 20 {
		// Good range: score decreases as ADX approaches 20
		return maxScore * (0.7 + 0.3*(20-adx)/5)
	} else if adx < 25 {
		// Borderline: score decreases more rapidly
		return maxScore * (0.4 + 0.3*(25-adx)/5)
	} else if adx < 35 {
		// Trend forming: low score
		return maxScore * (0.1 + 0.3*(35-adx)/10)
	}
	// Strong trend - not suitable for mean reversion
	return maxScore * 0.05
}

// scoreRSIExtreme calculates RSI extreme contribution to technical score.
// Looking for oversold (< 30) or overbought (> 70) conditions.
func (mrs *MeanReversionStrategy) scoreRSIExtreme(rsi float64) float64 {
	maxScore := mrs.config.RSIExtremeWeight

	if rsi <= 0 || rsi > 100 {
		return 0
	}

	// RSI scoring for mean reversion - extremes are good:
	// < 20: Very oversold (12 points) - strong long signal
	// 20-30: Oversold (9-12 points) - good long signal
	// 30-40: Approaching neutral (4-8 points) - weak signal
	// 40-60: Neutral (0-3 points) - no signal
	// 60-70: Approaching neutral (4-8 points) - weak signal
	// 70-80: Overbought (9-12 points) - good short signal
	// > 80: Very overbought (12 points) - strong short signal

	if rsi < 20 {
		return maxScore // Extremely oversold
	} else if rsi < 30 {
		// Oversold range
		return maxScore * (0.75 + 0.25*(30-rsi)/10)
	} else if rsi < 40 {
		// Approaching neutral from oversold
		return maxScore * (0.3 + 0.4*(40-rsi)/10)
	} else if rsi <= 60 {
		// Neutral zone - poor for mean reversion
		distFromCenter := 50 - rsi
		if distFromCenter < 0 {
			distFromCenter = -distFromCenter
		}
		return maxScore * (0.1 + 0.15*(distFromCenter/10))
	} else if rsi < 70 {
		// Approaching neutral from overbought
		return maxScore * (0.3 + 0.4*(rsi-60)/10)
	} else if rsi < 80 {
		// Overbought range
		return maxScore * (0.75 + 0.25*(rsi-70)/10)
	}
	// Extremely overbought
	return maxScore
}

// scoreBBPosition calculates Bollinger Band position contribution.
// For mean reversion, price at band extremes is favorable.
func (mrs *MeanReversionStrategy) scoreBBPosition(state *decision.CoinState) float64 {
	maxScore := mrs.config.BBPositionWeight

	// Note: CoinState doesn't have direct Bollinger Band fields
	// We use EMA21 as the middle band and ATR to estimate band width
	// In a full implementation, BB values would come from state

	if state.EMA21 <= 0 || state.ATR <= 0 {
		// No data for BB estimation
		return maxScore * 0.5
	}

	// Estimate BB position using price distance from EMA21 relative to ATR
	// Typical BB uses 2 * stddev, we approximate with 2 * ATR
	bbWidth := state.ATR * 2
	distanceFromMean := state.Price - state.EMA21
	if distanceFromMean < 0 {
		distanceFromMean = -distanceFromMean
	}

	// Calculate position as percentage of half-width
	positionPct := distanceFromMean / bbWidth

	// Scoring: closer to extremes (>80% of half-width) is better
	if positionPct >= mrs.config.BBExtremePct {
		return maxScore // At band extreme
	} else if positionPct >= 0.6 {
		// Near extreme - guard against division by zero if BBExtremePct <= 0.6
		denominator := mrs.config.BBExtremePct - 0.6
		if denominator <= 0 {
			return maxScore * 0.8 // Safe fallback for edge case
		}
		return maxScore * (0.6 + 0.4*(positionPct-0.6)/denominator)
	} else if positionPct >= 0.4 {
		// Middle zone
		return maxScore * (0.3 + 0.3*(positionPct-0.4)/0.2)
	}
	// Near mean - not good for entry, but could be exit zone
	return maxScore * (0.1 + 0.2*(positionPct/0.4))
}

// scoreSRProximity calculates support/resistance proximity score.
// Note: S/R levels aren't in CoinState, so we use ATR-based estimation.
func (mrs *MeanReversionStrategy) scoreSRProximity(state *decision.CoinState) float64 {
	maxScore := mrs.config.SRProximityWeight

	// Without actual S/R levels, we give a moderate baseline score
	// In a full implementation, this would check against calculated S/R levels

	// Use RSI extreme as a proxy - extreme RSI often occurs near S/R
	rsi := state.RSI
	if rsi <= 0 || rsi > 100 {
		return maxScore * 0.4
	}

	// If RSI is extreme, assume we're likely near S/R
	if rsi < 30 || rsi > 70 {
		return maxScore * 0.8
	} else if rsi < 35 || rsi > 65 {
		return maxScore * 0.6
	}
	return maxScore * 0.4
}

// calculateContextScore computes the market context score (0-30).
func (mrs *MeanReversionStrategy) calculateContextScore(state *decision.CoinState, score *decision.StrategyScore) float64 {
	var contextScore float64

	// Regime Match Score (0-15)
	regimeScore := mrs.scoreRegimeMatch(state.Regime)
	score.SetComponentDetail("regime_match_score", regimeScore)
	contextScore += regimeScore

	// Timeframe Alignment Score (0-15)
	// For mean reversion, neutral/ranging timeframes are preferred
	alignmentScore := mrs.scoreTimeframeAlignment(state.Trend15M, state.Trend1H)
	score.SetComponentDetail("timeframe_align_score", alignmentScore)
	contextScore += alignmentScore

	return contextScore
}

// scoreRegimeMatch calculates regime match contribution to context score.
func (mrs *MeanReversionStrategy) scoreRegimeMatch(regime decision.MarketRegime) float64 {
	maxScore := mrs.config.RegimeMatchWeight

	switch regime {
	case decision.RegimeRanging:
		return maxScore // Perfect match
	case decision.RegimeConsolidating:
		return maxScore * 0.7 // Good - tight range is okay
	case decision.RegimeVolatile:
		return maxScore * 0.4 // Risky - volatile can work but dangerous
	case decision.RegimeTrending:
		return maxScore * 0.1 // Poor - trend opposes mean reversion
	default:
		return 0
	}
}

// scoreTimeframeAlignment calculates timeframe alignment contribution.
// For mean reversion, neutral or opposing short-term vs long-term is actually good.
func (mrs *MeanReversionStrategy) scoreTimeframeAlignment(trend15m, trend1h decision.TrendDirection) float64 {
	maxScore := mrs.config.TimeframeAlignWeight

	// For mean reversion, we want:
	// - Both neutral: best (15 points) - no trend at all
	// - One neutral, one directional: good (10-12 points) - mixed signals
	// - Opposite directions: okay (8-10 points) - potential reversal
	// - Same direction (aligned trend): poor (3-5 points) - trend, not reversion

	if trend15m == decision.TrendNeutral && trend1h == decision.TrendNeutral {
		return maxScore // Perfect - no trend on either timeframe
	}

	// One neutral
	if trend15m == decision.TrendNeutral || trend1h == decision.TrendNeutral {
		return maxScore * 0.75
	}

	// Opposite directions - potential reversal setup
	if (trend15m == decision.TrendBullish && trend1h == decision.TrendBearish) ||
		(trend15m == decision.TrendBearish && trend1h == decision.TrendBullish) {
		return maxScore * 0.6
	}

	// Same direction - not ideal for mean reversion
	return maxScore * 0.25
}

// evaluateConditions checks entry and exit conditions.
// Must be called with read lock held.
func (mrs *MeanReversionStrategy) evaluateConditions(state *decision.CoinState, score *decision.StrategyScore) {
	entryConditions := mrs.getEntryConditionsLocked()
	exitConditions := mrs.getExitConditionsLocked()

	// Evaluate entry conditions
	allEntryMet := true
	for _, cond := range entryConditions {
		value := mrs.getIndicatorValue(state, cond.Indicator)
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
		value := mrs.getIndicatorValue(state, cond.Indicator)
		if cond.Evaluate(value) {
			exitTriggered = true
			break
		}
	}

	// Determine recommendation
	if exitTriggered {
		score.Recommendation = decision.RecommendExit
	} else if allEntryMet && score.FinalScore >= mrs.config.EntryScoreThreshold {
		// Determine long or short based on RSI (oversold = long, overbought = short)
		if state.RSI < mrs.config.RSIOversold {
			score.Recommendation = decision.RecommendEnterLong
		} else if state.RSI > mrs.config.RSIOverbought {
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
func (mrs *MeanReversionStrategy) getIndicatorValue(state *decision.CoinState, indicator string) float64 {
	switch indicator {
	case "adx":
		return state.ADX
	case "rsi":
		return state.RSI
	case "ema_21":
		return state.EMA21
	case "atr":
		return state.ATR
	case "price":
		return state.Price
	case "rsi_extreme":
		// Returns 1.0 if RSI is in extreme range (oversold or overbought)
		// RSI must be valid (0 < RSI <= 100) to avoid false signals on missing data
		if state.RSI <= 0 || state.RSI > 100 {
			return 0.0 // Invalid RSI - not a valid extreme
		}
		if state.RSI < mrs.config.RSIOversold || state.RSI > mrs.config.RSIOverbought {
			return 1.0
		}
		return 0.0
	case "rsi_neutral":
		// Returns 1.0 if RSI is in neutral range (exit signal)
		if state.RSI >= mrs.config.RSINeutralMin && state.RSI <= mrs.config.RSINeutralMax {
			return 1.0
		}
		return 0.0
	case "bb_extreme":
		// Check if price is at Bollinger Band extreme
		if state.EMA21 > 0 && state.ATR > 0 {
			bbWidth := state.ATR * 2
			distanceFromMean := state.Price - state.EMA21
			if distanceFromMean < 0 {
				distanceFromMean = -distanceFromMean
			}
			positionPct := distanceFromMean / bbWidth
			if positionPct >= mrs.config.BBExtremePct {
				return 1.0
			}
		}
		return 0.0
	case "bb_mean":
		// Check if price has returned to mean (middle BB / EMA21)
		if state.EMA21 > 0 && state.ATR > 0 {
			bbWidth := state.ATR * 2
			distanceFromMean := state.Price - state.EMA21
			if distanceFromMean < 0 {
				distanceFromMean = -distanceFromMean
			}
			positionPct := distanceFromMean / bbWidth
			if positionPct <= mrs.config.BBMeanPct {
				return 1.0
			}
		}
		return 0.0
	default:
		return 0
	}
}

// GetEntryConditions returns the conditions that must be met to enter a position.
// Thread-safe: holds read lock to prevent race conditions with SetConfig.
func (mrs *MeanReversionStrategy) GetEntryConditions() []decision.Condition {
	mrs.mu.RLock()
	defer mrs.mu.RUnlock()
	return mrs.getEntryConditionsLocked()
}

// getEntryConditionsLocked returns entry conditions without acquiring the lock.
// Must be called with read lock already held.
func (mrs *MeanReversionStrategy) getEntryConditionsLocked() []decision.Condition {
	return []decision.Condition{
		*decision.NewCondition(
			"adx_ranging",
			"ADX must be < 20 indicating no strong trend (ranging market)",
			"adx",
		).WithThreshold(decision.OperatorLessThan, mrs.config.ADXMaxEntry).
			WithRequired(true).
			WithBlockCategory(decision.CategoryHardBlock),

		*decision.NewCondition(
			"rsi_extreme",
			"RSI must be at extreme (< 30 oversold or > 70 overbought)",
			"rsi_extreme",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(true).
			WithBlockCategory(decision.CategoryHardBlock),

		*decision.NewCondition(
			"bb_extreme_position",
			"Price should be at Bollinger Band extreme",
			"bb_extreme",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false).
			WithWeight(0.8).
			WithBlockCategory(decision.CategorySoftBlock),
	}
}

// GetExitConditions returns the conditions that trigger position exit.
// Thread-safe: holds read lock to prevent race conditions with SetConfig.
func (mrs *MeanReversionStrategy) GetExitConditions() []decision.Condition {
	mrs.mu.RLock()
	defer mrs.mu.RUnlock()
	return mrs.getExitConditionsLocked()
}

// getExitConditionsLocked returns exit conditions without acquiring the lock.
// Must be called with read lock already held.
func (mrs *MeanReversionStrategy) getExitConditionsLocked() []decision.Condition {
	return []decision.Condition{
		*decision.NewCondition(
			"rsi_neutral",
			"RSI returned to neutral range (40-60)",
			"rsi_neutral",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false),

		*decision.NewCondition(
			"bb_mean_reversion",
			"Price has returned to mean (middle Bollinger Band)",
			"bb_mean",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false),

		*decision.NewCondition(
			"trend_forming",
			"ADX rising above 25 indicating trend is forming",
			"adx",
		).WithThreshold(decision.OperatorGreaterThan, mrs.config.ADXExitMax).
			WithRequired(false),
	}
}

// GetConfig returns the current strategy configuration.
// Note: Returns a copy to prevent external modification without SetConfig.
func (mrs *MeanReversionStrategy) GetConfig() *MeanReversionConfig {
	mrs.mu.RLock()
	defer mrs.mu.RUnlock()
	// Return a shallow copy of the config
	configCopy := *mrs.config
	return &configCopy
}

// SetConfig updates the strategy configuration.
// Thread-safe: uses mutex to prevent race conditions.
func (mrs *MeanReversionStrategy) SetConfig(config *MeanReversionConfig) {
	if config != nil {
		mrs.mu.Lock()
		mrs.config = config
		mrs.mu.Unlock()
	}
}

// init registers the strategy with the global registry on package load.
func init() {
	strategy := NewMeanReversionStrategy()
	if err := decision.RegisterGlobalStrategy(strategy); err != nil {
		// Strategy may already be registered if init is called multiple times
		// This is not an error in normal operation
	}
}

// RegisterMeanReversionStrategy explicitly registers the mean reversion strategy
// with the global registry. This can be called manually if package init is not used.
func RegisterMeanReversionStrategy() error {
	return decision.RegisterGlobalStrategy(NewMeanReversionStrategy())
}

// RegisterMeanReversionStrategyWithConfig registers the strategy with custom config.
func RegisterMeanReversionStrategyWithConfig(registry *decision.StrategyRegistry, config *MeanReversionConfig) error {
	strategy := NewMeanReversionStrategyWithConfig(config)
	return registry.RegisterStrategy(strategy)
}
