// Package strategies provides trading strategy implementations for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.11: Range Trading Strategy
package strategies

import (
	"sync"

	"binance-trading-bot/internal/decision"
)

// RangeTradingStrategy implements the Strategy interface for range trading in consolidating markets.
// This strategy trades reversals at established range boundaries with tight ATR.
//
// Entry Conditions:
//   - Low ATR (< average * 0.8) indicating tight range/consolidation
//   - Price at range boundary (estimated using EMA21 +/- 2*ATR)
//   - Low ADX (< 25) confirming no strong trend
//   - RSI at extremes at boundaries (oversold at support, overbought at resistance)
//
// Exit Conditions:
//   - Opposite range boundary reached with RSI normalization
//   - Range breakout detected (ATR expansion > 1.3x average)
type RangeTradingStrategy struct {
	// mu protects concurrent access to config
	mu sync.RWMutex
	// config holds the strategy-specific settings
	config *RangeTradingConfig
}

// RangeTradingConfig contains configurable parameters for the range trading strategy.
type RangeTradingConfig struct {
	// ATR thresholds - ATR must be LOW for range trading (tight consolidation)
	ATRMaxRatio float64 // Max ATR/ATRAverage ratio for tight range (default: 0.8)
	ATRBreakout float64 // ATR expansion ratio that triggers exit (default: 1.3)

	// Range boundary detection (using EMA and ATR)
	RangeBoundaryPct float64 // Price within this % of range edge for entry (default: 0.15 = 15%)
	RangeExitPct     float64 // Price reaching this % of opposite edge triggers exit (default: 0.85)

	// RSI thresholds for reversal detection
	RSIUpperReversal float64 // RSI above this at upper boundary suggests reversal (default: 65)
	RSILowerReversal float64 // RSI below this at lower boundary suggests reversal (default: 35)
	RSINeutralMin    float64 // RSI entering neutral zone (default: 45)
	RSINeutralMax    float64 // RSI leaving neutral zone (default: 55)

	// ADX threshold - low ADX confirms consolidation
	ADXMax float64 // Maximum ADX for ranging market (default: 25)

	// Scoring weights for technical components (0-40 total)
	ATRTightnessWeight    float64 // Weight for ATR tightness scoring (default: 10)
	RangeBoundaryWeight   float64 // Weight for range boundary proximity (default: 12)
	RSIReversalWeight     float64 // Weight for RSI reversal signal (default: 10)
	ADXWeight             float64 // Weight for ADX confirmation (default: 8)

	// Scoring weights for context components (0-30 total)
	RegimeMatchWeight    float64 // Weight for regime match (default: 15)
	TimeframeAlignWeight float64 // Weight for timeframe alignment (default: 15)

	// Entry score threshold (default: 55)
	EntryScoreThreshold float64 // Minimum final score to recommend entry

	// Time-based exit (maximum position hold time in minutes)
	MaxHoldMinutes int64 // Maximum hold time before forced exit consideration (default: 360 = 6 hours)
}

// DefaultRangeTradingConfig returns the default configuration for range trading.
func DefaultRangeTradingConfig() *RangeTradingConfig {
	return &RangeTradingConfig{
		ATRMaxRatio:          0.8,
		ATRBreakout:          1.3,
		RangeBoundaryPct:     0.15,
		RangeExitPct:         0.85,
		RSIUpperReversal:     65.0,
		RSILowerReversal:     35.0,
		RSINeutralMin:        45.0,
		RSINeutralMax:        55.0,
		ADXMax:               25.0,
		ATRTightnessWeight:   10.0,
		RangeBoundaryWeight:  12.0,
		RSIReversalWeight:    10.0,
		ADXWeight:            8.0,
		RegimeMatchWeight:    15.0,
		TimeframeAlignWeight: 15.0,
		EntryScoreThreshold:  55.0,
		MaxHoldMinutes:       360,
	}
}

// NewRangeTradingStrategy creates a new RangeTradingStrategy with default configuration.
func NewRangeTradingStrategy() *RangeTradingStrategy {
	return &RangeTradingStrategy{
		config: DefaultRangeTradingConfig(),
	}
}

// NewRangeTradingStrategyWithConfig creates a new RangeTradingStrategy with custom configuration.
func NewRangeTradingStrategyWithConfig(config *RangeTradingConfig) *RangeTradingStrategy {
	if config == nil {
		config = DefaultRangeTradingConfig()
	}
	return &RangeTradingStrategy{
		config: config,
	}
}

// Name returns the unique identifier for this strategy.
func (rts *RangeTradingStrategy) Name() string {
	return "range_trading"
}

// Description returns a human-readable explanation of the strategy.
func (rts *RangeTradingStrategy) Description() string {
	return "Trades reversals at established range boundaries in consolidating markets with tight ATR. Best for consolidating/ranging markets with clear support/resistance levels."
}

// SupportedRegimes returns the market regimes this strategy works best with.
func (rts *RangeTradingStrategy) SupportedRegimes() []decision.MarketRegime {
	return []decision.MarketRegime{decision.RegimeConsolidating}
}

// RequiredIndicators returns the list of indicators needed for this strategy.
// Note: Support/resistance detection is approximated using EMA21 and ATR-based range estimation.
func (rts *RangeTradingStrategy) RequiredIndicators() []string {
	return []string{
		"atr",
		"atr_average",
		"rsi",
		"adx",
		"ema_21",
		"trend_15m",
		"trend_1h",
	}
}

// CalculateScore evaluates the coin state and returns a strategy-specific score.
// The score breakdown follows the additive scoring formula:
// - Technical Score (0-40): ATR tightness, Range boundary, RSI reversal, ADX
// - Context Score (0-30): Regime match, Timeframe alignment
// - LLM Score (0-20): Not calculated here (provided externally)
// - History Score (0-10): Not calculated here (provided externally)
// Thread-safe: holds read lock during calculation to ensure config consistency.
func (rts *RangeTradingStrategy) CalculateScore(state *decision.CoinState) decision.StrategyScore {
	// Hold read lock during entire calculation for config consistency
	rts.mu.RLock()
	defer rts.mu.RUnlock()

	score := decision.NewStrategyScore(rts.Name())
	entryConditions := rts.getEntryConditionsLocked()
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
	technicalScore := rts.calculateTechnicalScore(state, score)
	score.TechnicalScore = technicalScore

	// Calculate Context Score (0-30)
	contextScore := rts.calculateContextScore(state, score)
	score.ContextScore = contextScore

	// LLM and History scores are provided externally
	// For now, use the values from CoinState if available
	score.LLMScore = float64(state.ScoreLLM) * 0.2        // Scale from 0-100 to 0-20
	score.HistoryScore = float64(state.ScoreHistory) * 0.1 // Scale from 0-100 to 0-10

	// Calculate final score
	score.CalculateFinal()

	// Evaluate entry/exit conditions
	rts.evaluateConditionsLocked(state, score, entryConditions)

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
func (rts *RangeTradingStrategy) calculateTechnicalScore(state *decision.CoinState, score *decision.StrategyScore) float64 {
	var technicalScore float64

	// ATR Tightness Score (0-10) - Lower ATR relative to average is better
	atrScore := rts.scoreATRTightness(state.ATR, state.ATRAverage)
	score.SetComponentDetail("atr_tightness_score", atrScore)
	technicalScore += atrScore

	// Range Boundary Score (0-12) - Price near S/R levels is good
	boundaryScore := rts.scoreRangeBoundary(state)
	score.SetComponentDetail("range_boundary_score", boundaryScore)
	technicalScore += boundaryScore

	// RSI Reversal Score (0-10) - RSI at extremes at boundaries suggests reversal
	rsiScore := rts.scoreRSIReversal(state)
	score.SetComponentDetail("rsi_reversal_score", rsiScore)
	technicalScore += rsiScore

	// ADX Score (0-8) - Low ADX confirms consolidation
	adxScore := rts.scoreADX(state.ADX)
	score.SetComponentDetail("adx_score", adxScore)
	technicalScore += adxScore

	return technicalScore
}

// scoreATRTightness calculates ATR tightness contribution to technical score.
// For range trading, LOWER ATR (tight range) is better.
func (rts *RangeTradingStrategy) scoreATRTightness(atr, atrAverage float64) float64 {
	maxScore := rts.config.ATRTightnessWeight

	// Need both ATR and ATRAverage to calculate tightness
	if atr <= 0 || atrAverage <= 0 {
		// If we have ATR but no average, give partial score
		if atr > 0 {
			return maxScore * 0.5
		}
		return 0
	}

	// Calculate ATR ratio (lower is better for range trading)
	atrRatio := atr / atrAverage

	// ATR scoring for range trading (lower is better):
	// < 0.6: Very tight range (10 points) - excellent for range trading
	// 0.6-0.8: Tight range (7-10 points) - good for range trading
	// 0.8-1.0: Normal volatility (4-7 points) - acceptable
	// 1.0-1.2: Elevated volatility (2-4 points) - borderline
	// > 1.2: High volatility (0-2 points) - not suitable for range trading

	if atrRatio < 0.6 {
		return maxScore // Perfect - very tight range
	} else if atrRatio < 0.8 {
		// Good range: score decreases as ATR ratio approaches 0.8
		return maxScore * (0.7 + 0.3*(0.8-atrRatio)/0.2)
	} else if atrRatio < 1.0 {
		// Acceptable range: score decreases more
		return maxScore * (0.4 + 0.3*(1.0-atrRatio)/0.2)
	} else if atrRatio < 1.2 {
		// Borderline: low score
		return maxScore * (0.2 + 0.2*(1.2-atrRatio)/0.2)
	}
	// High volatility - not suitable for range trading
	return maxScore * 0.1
}

// scoreRangeBoundary calculates range boundary proximity score.
// Price at range boundaries (near S/R) is favorable for entry.
func (rts *RangeTradingStrategy) scoreRangeBoundary(state *decision.CoinState) float64 {
	maxScore := rts.config.RangeBoundaryWeight

	// Use EMA21 as range midpoint and ATR to estimate range
	// In production, actual S/R levels would come from state
	if state.EMA21 <= 0 || state.ATR <= 0 {
		return maxScore * 0.3 // Baseline when data unavailable
	}

	// Estimate range using 2x ATR around EMA21
	rangeHalf := state.ATR * 2
	distanceFromMiddle := state.Price - state.EMA21
	if distanceFromMiddle < 0 {
		distanceFromMiddle = -distanceFromMiddle
	}

	// Calculate position as percentage of half-range
	positionPct := distanceFromMiddle / rangeHalf
	if positionPct > 1.0 {
		positionPct = 1.0 // Cap at range boundary
	}

	// Scoring: closer to boundary (positionPct near 1.0) is better for entry
	// Position 0.85-1.0: At boundary - excellent (12 points)
	// Position 0.7-0.85: Near boundary - good (8-12 points)
	// Position 0.5-0.7: Middle zone - moderate (4-8 points)
	// Position 0-0.5: Center - poor for entry (0-4 points)

	if positionPct >= rts.config.RangeExitPct {
		return maxScore // At boundary
	} else if positionPct >= 0.7 {
		// Near boundary
		rangeWidth := rts.config.RangeExitPct - 0.7
		if rangeWidth <= 0 {
			return maxScore * 0.8 // Safe fallback
		}
		return maxScore * (0.67 + 0.33*(positionPct-0.7)/rangeWidth)
	} else if positionPct >= 0.5 {
		// Middle zone
		return maxScore * (0.33 + 0.34*(positionPct-0.5)/0.2)
	}
	// Center zone - not ideal for range trading entry
	return maxScore * (positionPct / 0.5) * 0.33
}

// scoreRSIReversal calculates RSI reversal signal contribution.
// At range boundaries, RSI at extremes suggests reversal potential.
func (rts *RangeTradingStrategy) scoreRSIReversal(state *decision.CoinState) float64 {
	maxScore := rts.config.RSIReversalWeight

	// RSI can be 0 (extremely oversold) but not negative
	if state.RSI < 0 || state.RSI > 100 {
		return 0 // Invalid RSI
	}

	// Calculate price position relative to EMA21 (as range proxy)
	priceAboveMiddle := true
	if state.EMA21 > 0 {
		priceAboveMiddle = state.Price >= state.EMA21
	}

	// RSI scoring for reversal:
	// At upper boundary (price above middle): Lower RSI is better (suggests reversal down)
	// At lower boundary (price below middle): Higher RSI is better (suggests reversal up)
	// Actually inverted: at upper boundary high RSI that's starting to turn = reversal
	// At lower boundary low RSI that's starting to turn = reversal

	if priceAboveMiddle {
		// At upper boundary - looking for RSI showing weakness or overbought
		if state.RSI >= rts.config.RSIUpperReversal {
			// Overbought at resistance - excellent reversal signal
			if state.RSI >= 70 {
				return maxScore * 0.95
			}
			// Guard against division by zero when RSIUpperReversal >= 70
			denominator := 70 - rts.config.RSIUpperReversal
			if denominator <= 0 {
				return maxScore * 0.85 // Safe fallback for edge config
			}
			return maxScore * (0.7 + 0.25*(state.RSI-rts.config.RSIUpperReversal)/denominator)
		} else if state.RSI >= 55 {
			// Moderate RSI at resistance
			return maxScore * (0.4 + 0.3*(state.RSI-55)/10)
		}
		// Low RSI at resistance - not typical reversal pattern
		return maxScore * 0.3
	}

	// At lower boundary - looking for RSI showing strength or oversold
	if state.RSI <= rts.config.RSILowerReversal {
		// Oversold at support - excellent reversal signal
		if state.RSI <= 30 {
			return maxScore * 0.95
		}
		// Guard against division by zero when RSILowerReversal <= 30
		denominator := rts.config.RSILowerReversal - 30
		if denominator <= 0 {
			return maxScore * 0.85 // Safe fallback for edge config
		}
		return maxScore * (0.7 + 0.25*(rts.config.RSILowerReversal-state.RSI)/denominator)
	} else if state.RSI <= 45 {
		// Moderate RSI at support
		return maxScore * (0.4 + 0.3*(45-state.RSI)/10)
	}
	// High RSI at support - not typical reversal pattern
	return maxScore * 0.3
}

// scoreADX calculates ADX contribution to technical score.
// For range trading, LOWER ADX (no trend) is better - confirms consolidation.
func (rts *RangeTradingStrategy) scoreADX(adx float64) float64 {
	maxScore := rts.config.ADXWeight

	if adx <= 0 {
		// Zero or invalid ADX - return 0 to avoid false signals
		return 0
	}

	// ADX scoring for range trading (lower is better):
	// < 15: Very low ADX (8 points) - perfect consolidation
	// 15-20: Low ADX (6-8 points) - good consolidation
	// 20-25: Moderate ADX (3-6 points) - acceptable
	// 25-30: Elevated ADX (1-3 points) - trend forming
	// > 30: High ADX (0-1 points) - trending, not suitable

	if adx < 15 {
		return maxScore // Perfect consolidation
	} else if adx < 20 {
		return maxScore * (0.75 + 0.25*(20-adx)/5)
	} else if adx < 25 {
		return maxScore * (0.375 + 0.375*(25-adx)/5)
	} else if adx < 30 {
		return maxScore * (0.125 + 0.25*(30-adx)/5)
	}
	// High ADX - not suitable
	return maxScore * 0.05
}

// calculateContextScore computes the market context score (0-30).
func (rts *RangeTradingStrategy) calculateContextScore(state *decision.CoinState, score *decision.StrategyScore) float64 {
	var contextScore float64

	// Regime Match Score (0-15)
	regimeScore := rts.scoreRegimeMatch(state.Regime)
	score.SetComponentDetail("regime_match_score", regimeScore)
	contextScore += regimeScore

	// Timeframe Alignment Score (0-15)
	alignmentScore := rts.scoreTimeframeAlignment(state.Trend15M, state.Trend1H)
	score.SetComponentDetail("timeframe_align_score", alignmentScore)
	contextScore += alignmentScore

	return contextScore
}

// scoreRegimeMatch calculates regime match contribution to context score.
func (rts *RangeTradingStrategy) scoreRegimeMatch(regime decision.MarketRegime) float64 {
	maxScore := rts.config.RegimeMatchWeight

	switch regime {
	case decision.RegimeConsolidating:
		return maxScore // Perfect match - consolidating markets
	case decision.RegimeRanging:
		return maxScore * 0.8 // Good - ranging is similar to consolidating
	case decision.RegimeVolatile:
		return maxScore * 0.3 // Poor - volatile markets not good for range trading
	case decision.RegimeTrending:
		return maxScore * 0.1 // Very poor - trending opposes range trading
	default:
		return 0
	}
}

// scoreTimeframeAlignment calculates timeframe alignment contribution.
// For range trading, neutral/consolidating timeframes are preferred.
func (rts *RangeTradingStrategy) scoreTimeframeAlignment(trend15m, trend1h decision.TrendDirection) float64 {
	maxScore := rts.config.TimeframeAlignWeight

	// For range trading, we want:
	// - Both neutral: best (15 points) - confirms consolidation
	// - One neutral, one directional: good (10-12 points) - mixed signals
	// - Opposite directions: moderate (7-10 points) - chop, can work
	// - Same direction (aligned trend): poor (3-5 points) - trending, avoid

	if trend15m == decision.TrendNeutral && trend1h == decision.TrendNeutral {
		return maxScore // Perfect - confirms consolidation
	}

	// One neutral
	if trend15m == decision.TrendNeutral || trend1h == decision.TrendNeutral {
		return maxScore * 0.75
	}

	// Opposite directions - choppy market, can work for range trading
	if (trend15m == decision.TrendBullish && trend1h == decision.TrendBearish) ||
		(trend15m == decision.TrendBearish && trend1h == decision.TrendBullish) {
		return maxScore * 0.55
	}

	// Same direction - trending, not ideal for range trading
	return maxScore * 0.25
}

// evaluateConditionsLocked checks entry and exit conditions.
// Must be called with read lock held.
func (rts *RangeTradingStrategy) evaluateConditionsLocked(state *decision.CoinState, score *decision.StrategyScore, entryConditions []decision.Condition) {
	exitConditions := rts.getExitConditionsLocked()

	// Evaluate entry conditions
	allEntryMet := true
	for _, cond := range entryConditions {
		value := rts.getIndicatorValue(state, cond.Indicator)
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
		value := rts.getIndicatorValue(state, cond.Indicator)
		if cond.Evaluate(value) {
			exitTriggered = true
			break
		}
	}

	// Determine recommendation
	if exitTriggered {
		score.Recommendation = decision.RecommendExit
	} else if allEntryMet && score.FinalScore >= rts.config.EntryScoreThreshold {
		// Determine long or short based on price position relative to range middle
		if state.EMA21 > 0 {
			if state.Price < state.EMA21 {
				// Price at lower boundary - long entry
				score.Recommendation = decision.RecommendEnterLong
			} else {
				// Price at upper boundary - short entry
				score.Recommendation = decision.RecommendEnterShort
			}
		} else {
			// Use RSI as fallback for direction
			if state.RSI <= rts.config.RSILowerReversal {
				score.Recommendation = decision.RecommendEnterLong
			} else if state.RSI >= rts.config.RSIUpperReversal {
				score.Recommendation = decision.RecommendEnterShort
			} else {
				score.Recommendation = decision.RecommendHold
			}
		}
	} else if score.IsBlocked() {
		score.Recommendation = decision.RecommendAvoid
	} else {
		score.Recommendation = decision.RecommendHold
	}
}

// getIndicatorValue retrieves the value for a specific indicator from CoinState.
func (rts *RangeTradingStrategy) getIndicatorValue(state *decision.CoinState, indicator string) float64 {
	switch indicator {
	case "atr":
		return state.ATR
	case "atr_average":
		return state.ATRAverage
	case "rsi":
		return state.RSI
	case "adx":
		return state.ADX
	case "ema_21":
		return state.EMA21
	case "price":
		return state.Price
	case "atr_tight":
		// Returns 1.0 if ATR is tight (below threshold ratio)
		if state.ATR > 0 && state.ATRAverage > 0 {
			ratio := state.ATR / state.ATRAverage
			if ratio <= rts.config.ATRMaxRatio {
				return 1.0
			}
		}
		return 0.0
	case "range_boundary":
		// Returns 1.0 if price is at range boundary
		if state.EMA21 > 0 && state.ATR > 0 {
			rangeHalf := state.ATR * 2
			distanceFromMiddle := state.Price - state.EMA21
			if distanceFromMiddle < 0 {
				distanceFromMiddle = -distanceFromMiddle
			}
			positionPct := distanceFromMiddle / rangeHalf
			if positionPct >= (1.0 - rts.config.RangeBoundaryPct) {
				return 1.0
			}
		}
		return 0.0
	case "adx_low":
		// Returns 1.0 if ADX is low (consolidating)
		if state.ADX > 0 && state.ADX <= rts.config.ADXMax {
			return 1.0
		}
		return 0.0
	case "opposite_boundary":
		// Returns 1.0 if price has reached the opposite range boundary with neutral RSI
		// For exit to trigger, price must be at boundary AND RSI must have reverted toward neutral
		// This prevents exit from triggering at entry (where RSI is extreme)
		if state.EMA21 > 0 && state.ATR > 0 && state.RSI > 0 && state.RSI <= 100 {
			rangeHalf := state.ATR * 2
			distanceFromMiddle := state.Price - state.EMA21
			priceAboveMiddle := distanceFromMiddle >= 0
			if distanceFromMiddle < 0 {
				distanceFromMiddle = -distanceFromMiddle
			}
			positionPct := distanceFromMiddle / rangeHalf

			// Must be at boundary AND RSI should have moved toward neutral
			// At upper boundary with RSI not too high = reversal from oversold played out
			// At lower boundary with RSI not too low = reversal from overbought played out
			if positionPct >= rts.config.RangeExitPct {
				if priceAboveMiddle && state.RSI < rts.config.RSIUpperReversal {
					// At upper boundary but RSI is not overbought - previous long target reached
					return 1.0
				}
				if !priceAboveMiddle && state.RSI > rts.config.RSILowerReversal {
					// At lower boundary but RSI is not oversold - previous short target reached
					return 1.0
				}
			}
		}
		return 0.0
	case "atr_breakout":
		// Returns 1.0 if ATR has expanded (breakout occurring)
		if state.ATR > 0 && state.ATRAverage > 0 {
			ratio := state.ATR / state.ATRAverage
			if ratio >= rts.config.ATRBreakout {
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
func (rts *RangeTradingStrategy) GetEntryConditions() []decision.Condition {
	rts.mu.RLock()
	defer rts.mu.RUnlock()
	return rts.getEntryConditionsLocked()
}

// getEntryConditionsLocked returns entry conditions without acquiring the lock.
// Must be called with read lock already held.
func (rts *RangeTradingStrategy) getEntryConditionsLocked() []decision.Condition {
	return []decision.Condition{
		*decision.NewCondition(
			"atr_tight",
			"ATR must be tight (below average) indicating consolidation",
			"atr_tight",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(true).
			WithBlockCategory(decision.CategoryHardBlock),

		*decision.NewCondition(
			"range_boundary",
			"Price must be at range boundary (near support or resistance)",
			"range_boundary",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(true).
			WithBlockCategory(decision.CategoryHardBlock),

		*decision.NewCondition(
			"adx_low",
			"ADX must be low indicating no strong trend (consolidation)",
			"adx_low",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false).
			WithWeight(0.8).
			WithBlockCategory(decision.CategorySoftBlock),
	}
}

// GetExitConditions returns the conditions that trigger position exit.
// Thread-safe: holds read lock to prevent race conditions with SetConfig.
func (rts *RangeTradingStrategy) GetExitConditions() []decision.Condition {
	rts.mu.RLock()
	defer rts.mu.RUnlock()
	return rts.getExitConditionsLocked()
}

// getExitConditionsLocked returns exit conditions without acquiring the lock.
// Must be called with read lock already held.
func (rts *RangeTradingStrategy) getExitConditionsLocked() []decision.Condition {
	return []decision.Condition{
		*decision.NewCondition(
			"opposite_boundary",
			"Price has reached opposite range boundary (target achieved)",
			"opposite_boundary",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false),

		*decision.NewCondition(
			"atr_breakout",
			"ATR has expanded indicating range breakout",
			"atr_breakout",
		).WithThreshold(decision.OperatorEqual, 1.0).
			WithRequired(false),
	}
}

// GetConfig returns the current strategy configuration.
// Note: Returns a copy to prevent external modification without SetConfig.
func (rts *RangeTradingStrategy) GetConfig() *RangeTradingConfig {
	rts.mu.RLock()
	defer rts.mu.RUnlock()
	// Return a shallow copy of the config
	configCopy := *rts.config
	return &configCopy
}

// SetConfig updates the strategy configuration.
// Thread-safe: uses mutex to prevent race conditions.
func (rts *RangeTradingStrategy) SetConfig(config *RangeTradingConfig) {
	if config != nil {
		rts.mu.Lock()
		rts.config = config
		rts.mu.Unlock()
	}
}

// init registers the strategy with the global registry on package load.
func init() {
	strategy := NewRangeTradingStrategy()
	if err := decision.RegisterGlobalStrategy(strategy); err != nil {
		// Strategy may already be registered if init is called multiple times
		// This is not an error in normal operation
	}
}

// RegisterRangeTradingStrategy explicitly registers the range trading strategy
// with the global registry. This can be called manually if package init is not used.
func RegisterRangeTradingStrategy() error {
	return decision.RegisterGlobalStrategy(NewRangeTradingStrategy())
}

// RegisterRangeTradingStrategyWithConfig registers the strategy with custom config.
func RegisterRangeTradingStrategyWithConfig(registry *decision.StrategyRegistry, config *RangeTradingConfig) error {
	strategy := NewRangeTradingStrategyWithConfig(config)
	return registry.RegisterStrategy(strategy)
}
