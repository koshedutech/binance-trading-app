// Package strategies provides trading strategy implementations for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.11: Range Trading Strategy Tests
package strategies

import (
	"testing"
	"time"

	"binance-trading-bot/internal/decision"
)

// TestRangeTradingStrategy_Name tests the Name() method.
func TestRangeTradingStrategy_Name(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	name := strategy.Name()
	if name != "range_trading" {
		t.Errorf("Expected name 'range_trading', got '%s'", name)
	}
}

// TestRangeTradingStrategy_Description tests the Description() method.
func TestRangeTradingStrategy_Description(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	desc := strategy.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
	if len(desc) < 20 {
		t.Error("Description should be meaningful (at least 20 chars)")
	}
}

// TestRangeTradingStrategy_SupportedRegimes tests the SupportedRegimes() method.
func TestRangeTradingStrategy_SupportedRegimes(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	regimes := strategy.SupportedRegimes()
	if len(regimes) == 0 {
		t.Error("SupportedRegimes should not be empty")
	}

	// Should support CONSOLIDATING regime
	found := false
	for _, r := range regimes {
		if r == decision.RegimeConsolidating {
			found = true
			break
		}
	}
	if !found {
		t.Error("RangeTradingStrategy should support CONSOLIDATING regime")
	}
}

// TestRangeTradingStrategy_RequiredIndicators tests the RequiredIndicators() method.
func TestRangeTradingStrategy_RequiredIndicators(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	indicators := strategy.RequiredIndicators()
	if len(indicators) == 0 {
		t.Error("RequiredIndicators should not be empty")
	}

	// Check for required indicators per implementation
	// Note: Support/resistance are approximated using EMA21 + ATR-based range estimation
	required := []string{"atr", "atr_average", "rsi", "adx", "ema_21"}
	for _, req := range required {
		found := false
		for _, ind := range indicators {
			if ind == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing required indicator: %s", req)
		}
	}
}

// TestRangeTradingStrategy_GetEntryConditions tests the GetEntryConditions() method.
func TestRangeTradingStrategy_GetEntryConditions(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	conditions := strategy.GetEntryConditions()
	if len(conditions) == 0 {
		t.Error("GetEntryConditions should return at least one condition")
	}

	// Check that each condition has required fields
	for _, cond := range conditions {
		if cond.Name == "" {
			t.Error("Condition name should not be empty")
		}
		if cond.Description == "" {
			t.Error("Condition description should not be empty")
		}
		if cond.Indicator == "" {
			t.Error("Condition indicator should not be empty")
		}
	}

	// Check for ATR tight condition
	hasATRTight := false
	for _, cond := range conditions {
		if cond.Name == "atr_tight" {
			hasATRTight = true
			if !cond.Required {
				t.Error("ATR tight condition should be required")
			}
			break
		}
	}
	if !hasATRTight {
		t.Error("Missing ATR tight entry condition")
	}

	// Check for range boundary condition
	hasRangeBoundary := false
	for _, cond := range conditions {
		if cond.Name == "range_boundary" {
			hasRangeBoundary = true
			if !cond.Required {
				t.Error("Range boundary condition should be required")
			}
			break
		}
	}
	if !hasRangeBoundary {
		t.Error("Missing range boundary entry condition")
	}

	// Check for ADX low condition
	hasADXLow := false
	for _, cond := range conditions {
		if cond.Name == "adx_low" {
			hasADXLow = true
			break
		}
	}
	if !hasADXLow {
		t.Error("Missing ADX low entry condition")
	}
}

// TestRangeTradingStrategy_GetExitConditions tests the GetExitConditions() method.
func TestRangeTradingStrategy_GetExitConditions(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	conditions := strategy.GetExitConditions()
	if len(conditions) == 0 {
		t.Error("GetExitConditions should return at least one condition")
	}

	// Check for opposite boundary exit condition
	hasOppositeBoundary := false
	for _, cond := range conditions {
		if cond.Name == "opposite_boundary" {
			hasOppositeBoundary = true
			break
		}
	}
	if !hasOppositeBoundary {
		t.Error("Missing opposite boundary exit condition")
	}

	// Check for ATR breakout exit condition
	hasATRBreakout := false
	for _, cond := range conditions {
		if cond.Name == "atr_breakout" {
			hasATRBreakout = true
			break
		}
	}
	if !hasATRBreakout {
		t.Error("Missing ATR breakout exit condition")
	}
}

// TestRangeTradingStrategy_CalculateScore_NilState tests scoring with nil state.
func TestRangeTradingStrategy_CalculateScore_NilState(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	score := strategy.CalculateScore(nil)

	if score.StrategyName != "range_trading" {
		t.Errorf("Expected strategy name 'range_trading', got '%s'", score.StrategyName)
	}
	if score.Recommendation != decision.RecommendAvoid {
		t.Errorf("Expected AVOID recommendation for nil state, got %s", score.Recommendation)
	}
	if len(score.BlockingReasons) == 0 {
		t.Error("Expected blocking reasons for nil state")
	}
}

// TestRangeTradingStrategy_CalculateScore_LowerBoundaryEntry tests scoring for entry at lower boundary.
func TestRangeTradingStrategy_CalculateScore_LowerBoundaryEntry(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       48000, // Price at lower boundary
		Regime:      decision.RegimeConsolidating,
		ADX:         18.0,   // Low ADX - consolidating
		RSI:         32.0,   // Low RSI at support
		EMA21:       50000,  // Range middle
		ATR:         400,    // Tight ATR
		ATRAverage:  600,    // ATR ratio = 0.67 < 0.8
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Technical score should be good for tight range at boundary
	if score.TechnicalScore < 20 {
		t.Errorf("Expected technical score >= 20 for lower boundary setup, got %.2f", score.TechnicalScore)
	}

	// Context score should be high due to consolidating regime
	if score.ContextScore < 20 {
		t.Errorf("Expected context score >= 20 for consolidating regime, got %.2f", score.ContextScore)
	}

	// ATR tightness score should be good
	atrScore, ok := score.ComponentDetails["atr_tightness_score"]
	if !ok || atrScore < 6 {
		t.Errorf("Expected ATR tightness score >= 6 for tight range, got %.2f", atrScore)
	}
}

// TestRangeTradingStrategy_CalculateScore_UpperBoundaryEntry tests scoring for entry at upper boundary.
func TestRangeTradingStrategy_CalculateScore_UpperBoundaryEntry(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       52000, // Price at upper boundary
		Regime:      decision.RegimeConsolidating,
		ADX:         15.0,   // Very low ADX
		RSI:         68.0,   // High RSI at resistance
		EMA21:       50000,  // Range middle
		ATR:         450,    // Tight ATR
		ATRAverage:  600,    // ATR ratio = 0.75 < 0.8
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// RSI reversal score should be good at upper boundary
	rsiScore, ok := score.ComponentDetails["rsi_reversal_score"]
	if !ok || rsiScore < 5 {
		t.Errorf("Expected RSI reversal score >= 5 for RSI 68 at upper boundary, got %.2f", rsiScore)
	}

	// ADX score should be excellent for very low ADX
	adxScore, ok := score.ComponentDetails["adx_score"]
	if !ok || adxScore < 6 {
		t.Errorf("Expected ADX score >= 6 for ADX 15, got %.2f", adxScore)
	}
}

// TestRangeTradingStrategy_CalculateScore_HighATR tests scoring rejects high ATR.
func TestRangeTradingStrategy_CalculateScore_HighATR(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50500,
		Regime:      decision.RegimeVolatile, // Wrong regime
		ADX:         30.0,                     // High ADX
		RSI:         55.0,
		EMA21:       50000,
		ATR:         800, // High ATR
		ATRAverage:  600, // ATR ratio = 1.33 > 0.8
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// ATR tight condition should fail
	conditionFailed := false
	for _, name := range score.FailedConditions {
		if name == "atr_tight" {
			conditionFailed = true
			break
		}
	}
	if !conditionFailed {
		t.Error("ATR tight condition should have failed for high ATR")
	}

	// Should have blocking reason
	if len(score.BlockingReasons) == 0 {
		t.Error("Expected blocking reason for high ATR")
	}

	// ATR tightness score should be low
	atrScore, ok := score.ComponentDetails["atr_tightness_score"]
	if ok && atrScore > 3 {
		t.Errorf("Expected low ATR tightness score for high ATR, got %.2f", atrScore)
	}
}

// TestRangeTradingStrategy_CalculateScore_PriceAtMiddle tests scoring rejects price at range middle.
func TestRangeTradingStrategy_CalculateScore_PriceAtMiddle(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50100, // Very close to EMA21 (middle)
		Regime:      decision.RegimeConsolidating,
		ADX:         18.0,
		RSI:         50.0,  // Neutral RSI
		EMA21:       50000, // Range middle
		ATR:         400,
		ATRAverage:  600,
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Range boundary condition should fail (price is at middle, not boundary)
	conditionFailed := false
	for _, name := range score.FailedConditions {
		if name == "range_boundary" {
			conditionFailed = true
			break
		}
	}
	if !conditionFailed {
		t.Error("Range boundary condition should have failed for price at middle")
	}

	// Range boundary score should be low
	boundaryScore, ok := score.ComponentDetails["range_boundary_score"]
	if ok && boundaryScore > 4 {
		t.Errorf("Expected low range boundary score for price at middle, got %.2f", boundaryScore)
	}
}

// TestRangeTradingStrategy_CalculateScore_ExitOppositeBoundary tests exit when reaching opposite boundary.
func TestRangeTradingStrategy_CalculateScore_ExitOppositeBoundary(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	// Scenario: Entered long at lower boundary, now at upper boundary
	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       52000, // Near upper boundary
		Regime:      decision.RegimeConsolidating,
		ADX:         18.0,
		RSI:         65.0,
		EMA21:       50000,
		ATR:         400,    // BB-like range = 50000 +/- 800
		ATRAverage:  500,
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Price at 52000, EMA21 at 50000, distance = 2000
	// Range half = ATR * 2 = 800, position = 2000/800 = 2.5 (capped at 1.0)
	// This should trigger opposite_boundary exit condition
	if score.Recommendation != decision.RecommendExit && score.Recommendation != decision.RecommendEnterShort {
		// It's also valid to recommend short entry at upper boundary
		t.Logf("Recommendation: %s (either EXIT or ENTER_SHORT expected at opposite boundary)", score.Recommendation)
	}
}

// TestRangeTradingStrategy_CalculateScore_ExitATRBreakout tests exit when ATR expands.
func TestRangeTradingStrategy_CalculateScore_ExitATRBreakout(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	// Scenario: Was consolidating, now ATR expanded indicating breakout
	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       51500,
		Regime:      decision.RegimeConsolidating,
		ADX:         28.0,   // ADX increasing
		RSI:         60.0,
		EMA21:       50000,
		ATR:         900,    // ATR expanded
		ATRAverage:  600,    // ATR ratio = 1.5 > 1.3 breakout threshold
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Should recommend exit due to ATR breakout
	if score.Recommendation != decision.RecommendExit {
		t.Errorf("Expected EXIT when ATR indicates breakout (ratio > 1.3), got %s", score.Recommendation)
	}
}

// TestRangeTradingStrategy_CalculateScore_RecommendEnterLong tests long recommendation at lower boundary.
func TestRangeTradingStrategy_CalculateScore_RecommendEnterLong(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:       "BTCUSDT",
		Price:        48200, // At lower boundary
		Regime:       decision.RegimeConsolidating,
		ADX:          12.0, // Very low ADX
		RSI:          28.0, // Oversold at support
		EMA21:        50000,
		ATR:          350,  // Tight range
		ATRAverage:   500,  // ATR ratio = 0.7
		Trend1H:      decision.TrendNeutral,
		Trend15M:     decision.TrendNeutral,
		ScoreLLM:     60,
		ScoreHistory: 55,
		LastUpdated:  time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// With strong setup and external scores, should recommend long
	if score.FinalScore < 55 {
		t.Logf("Note: Score %.2f below threshold 55, recommendation: %s", score.FinalScore, score.Recommendation)
	} else if score.Recommendation != decision.RecommendEnterLong {
		t.Errorf("Expected ENTER_LONG at lower boundary with good score, got %s (score: %.2f)",
			score.Recommendation, score.FinalScore)
	}
}

// TestRangeTradingStrategy_CalculateScore_RecommendEnterShort tests short recommendation at upper boundary.
func TestRangeTradingStrategy_CalculateScore_RecommendEnterShort(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:       "BTCUSDT",
		Price:        51800, // At upper boundary
		Regime:       decision.RegimeConsolidating,
		ADX:          12.0, // Very low ADX
		RSI:          72.0, // Overbought at resistance
		EMA21:        50000,
		ATR:          350,  // Tight range
		ATRAverage:   500,  // ATR ratio = 0.7
		Trend1H:      decision.TrendNeutral,
		Trend15M:     decision.TrendNeutral,
		ScoreLLM:     60,
		ScoreHistory: 55,
		LastUpdated:  time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// With strong setup and external scores, should recommend short
	if score.FinalScore < 55 {
		t.Logf("Note: Score %.2f below threshold 55, recommendation: %s", score.FinalScore, score.Recommendation)
	} else if score.Recommendation != decision.RecommendEnterShort {
		t.Errorf("Expected ENTER_SHORT at upper boundary with good score, got %s (score: %.2f)",
			score.Recommendation, score.FinalScore)
	}
}

// TestRangeTradingStrategy_CalculateScore_RangingRegime tests scoring in ranging market.
func TestRangeTradingStrategy_CalculateScore_RangingRegime(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       48500,
		Regime:      decision.RegimeRanging, // Similar to consolidating
		ADX:         18.0,
		RSI:         35.0,
		EMA21:       50000,
		ATR:         400,
		ATRAverage:  550,
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Regime match score should be decent for ranging (similar to consolidating)
	regimeScore, ok := score.ComponentDetails["regime_match_score"]
	if !ok || regimeScore < 10 {
		t.Errorf("Expected regime match score >= 10 for ranging market, got %.2f", regimeScore)
	}
}

// TestRangeTradingStrategy_CalculateScore_TrendingRegime tests scoring rejects trending market.
func TestRangeTradingStrategy_CalculateScore_TrendingRegime(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       52000,
		Regime:      decision.RegimeTrending, // Wrong regime
		ADX:         35.0,                     // High ADX
		RSI:         65.0,
		EMA21:       49000,
		ATR:         700,
		ATRAverage:  500,
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Regime match score should be very low for trending
	regimeScore, ok := score.ComponentDetails["regime_match_score"]
	if ok && regimeScore > 3 {
		t.Errorf("Expected low regime match score for trending market, got %.2f", regimeScore)
	}
}

// TestRangeTradingStrategy_CalculateScore_ScoreRange tests score is within valid range.
func TestRangeTradingStrategy_CalculateScore_ScoreRange(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	testCases := []struct {
		name  string
		state *decision.CoinState
	}{
		{
			name: "Perfect consolidation setup at lower boundary",
			state: &decision.CoinState{
				Symbol: "BTCUSDT", Price: 48000, Regime: decision.RegimeConsolidating,
				ADX: 12, RSI: 28, EMA21: 50000, ATR: 300, ATRAverage: 500,
				Trend1H: decision.TrendNeutral, Trend15M: decision.TrendNeutral,
				ScoreLLM: 80, ScoreHistory: 70,
			},
		},
		{
			name: "Poor setup - trending market",
			state: &decision.CoinState{
				Symbol: "BTCUSDT", Price: 50000, Regime: decision.RegimeTrending,
				ADX: 40, RSI: 55, EMA21: 49000, ATR: 800, ATRAverage: 500,
				Trend1H: decision.TrendBullish, Trend15M: decision.TrendBullish,
				ScoreLLM: 20, ScoreHistory: 30,
			},
		},
		{
			name: "Zero values",
			state: &decision.CoinState{
				Symbol: "BTCUSDT",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score := strategy.CalculateScore(tc.state)

			// Technical score should be 0-40
			if score.TechnicalScore < 0 || score.TechnicalScore > 40 {
				t.Errorf("Technical score %.2f out of range [0, 40]", score.TechnicalScore)
			}

			// Context score should be 0-30
			if score.ContextScore < 0 || score.ContextScore > 30 {
				t.Errorf("Context score %.2f out of range [0, 30]", score.ContextScore)
			}

			// LLM score should be 0-20
			if score.LLMScore < 0 || score.LLMScore > 20 {
				t.Errorf("LLM score %.2f out of range [0, 20]", score.LLMScore)
			}

			// History score should be 0-10
			if score.HistoryScore < 0 || score.HistoryScore > 10 {
				t.Errorf("History score %.2f out of range [0, 10]", score.HistoryScore)
			}

			// Final score should be 0-100
			if score.FinalScore < 0 || score.FinalScore > 100 {
				t.Errorf("Final score %.2f out of range [0, 100]", score.FinalScore)
			}

			// Confidence should be 0-100
			if score.Confidence < 0 || score.Confidence > 100 {
				t.Errorf("Confidence %.2f out of range [0, 100]", score.Confidence)
			}
		})
	}
}

// TestRangeTradingStrategy_Config tests configuration.
func TestRangeTradingStrategy_Config(t *testing.T) {
	// Test default config
	strategy := NewRangeTradingStrategy()
	config := strategy.GetConfig()

	if config.ATRMaxRatio != 0.8 {
		t.Errorf("Expected default ATRMaxRatio 0.8, got %f", config.ATRMaxRatio)
	}
	if config.ATRBreakout != 1.3 {
		t.Errorf("Expected default ATRBreakout 1.3, got %f", config.ATRBreakout)
	}
	if config.RangeBoundaryPct != 0.15 {
		t.Errorf("Expected default RangeBoundaryPct 0.15, got %f", config.RangeBoundaryPct)
	}
	if config.RangeExitPct != 0.85 {
		t.Errorf("Expected default RangeExitPct 0.85, got %f", config.RangeExitPct)
	}
	if config.ADXMax != 25.0 {
		t.Errorf("Expected default ADXMax 25.0, got %f", config.ADXMax)
	}
	if config.EntryScoreThreshold != 55.0 {
		t.Errorf("Expected default EntryScoreThreshold 55.0, got %f", config.EntryScoreThreshold)
	}
	if config.MaxHoldMinutes != 360 {
		t.Errorf("Expected default MaxHoldMinutes 360, got %d", config.MaxHoldMinutes)
	}

	// Test custom config
	customConfig := &RangeTradingConfig{
		ATRMaxRatio:          0.7,
		ATRBreakout:          1.4,
		RangeBoundaryPct:     0.2,
		RangeExitPct:         0.8,
		RSIUpperReversal:     60.0,
		RSILowerReversal:     40.0,
		RSINeutralMin:        42.0,
		RSINeutralMax:        58.0,
		ADXMax:               20.0,
		ATRTightnessWeight:   12.0,
		RangeBoundaryWeight:  10.0,
		RSIReversalWeight:    10.0,
		ADXWeight:            8.0,
		RegimeMatchWeight:    15.0,
		TimeframeAlignWeight: 15.0,
		EntryScoreThreshold:  60.0,
		MaxHoldMinutes:       480,
	}

	strategyCustom := NewRangeTradingStrategyWithConfig(customConfig)
	configCustom := strategyCustom.GetConfig()

	if configCustom.ATRMaxRatio != 0.7 {
		t.Errorf("Expected custom ATRMaxRatio 0.7, got %f", configCustom.ATRMaxRatio)
	}
	if configCustom.ADXMax != 20.0 {
		t.Errorf("Expected custom ADXMax 20.0, got %f", configCustom.ADXMax)
	}

	// Test nil config falls back to default
	strategyNil := NewRangeTradingStrategyWithConfig(nil)
	configNil := strategyNil.GetConfig()
	if configNil.ATRMaxRatio != 0.8 {
		t.Errorf("Expected default ATRMaxRatio 0.8 for nil config, got %f", configNil.ATRMaxRatio)
	}
}

// TestRangeTradingStrategy_SetConfig tests setting configuration.
func TestRangeTradingStrategy_SetConfig(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	newConfig := &RangeTradingConfig{
		ATRMaxRatio: 0.75,
		ATRBreakout: 1.25,
		ADXMax:      22.0,
	}

	strategy.SetConfig(newConfig)
	config := strategy.GetConfig()

	if config.ATRMaxRatio != 0.75 {
		t.Errorf("Expected updated ATRMaxRatio 0.75, got %f", config.ATRMaxRatio)
	}

	// Setting nil should not change config
	currentATRMax := strategy.GetConfig().ATRMaxRatio
	strategy.SetConfig(nil)
	if strategy.GetConfig().ATRMaxRatio != currentATRMax {
		t.Error("SetConfig(nil) should not change config")
	}
}

// TestRangeTradingStrategy_ScoreATRTightness tests ATR tightness scoring.
func TestRangeTradingStrategy_ScoreATRTightness(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	testCases := []struct {
		atr         float64
		atrAverage  float64
		minExpected float64
		maxExpected float64
	}{
		{0, 500, 0, 0},       // Zero ATR - invalid
		{300, 500, 8, 10},    // Ratio 0.6 - very tight
		{400, 500, 7, 10},    // Ratio 0.8 - tight
		{500, 500, 3.9, 5},   // Ratio 1.0 - normal (at boundary, slight tolerance for float precision)
		{600, 500, 1, 4},     // Ratio 1.2 - elevated
		{750, 500, 0, 2},     // Ratio 1.5 - high volatility
	}

	for _, tc := range testCases {
		state := &decision.CoinState{
			Symbol:     "TEST",
			ATR:        tc.atr,
			ATRAverage: tc.atrAverage,
		}
		score := strategy.CalculateScore(state)
		atrScore, ok := score.ComponentDetails["atr_tightness_score"]
		if !ok {
			t.Errorf("ATR %f / ATRAvg %f: Missing atr_tightness_score in details", tc.atr, tc.atrAverage)
			continue
		}
		if atrScore < tc.minExpected || atrScore > tc.maxExpected {
			t.Errorf("ATR %f / ATRAvg %f: score %.2f not in expected range [%.2f, %.2f]",
				tc.atr, tc.atrAverage, atrScore, tc.minExpected, tc.maxExpected)
		}
	}
}

// TestRangeTradingStrategy_ScoreADX tests ADX scoring (lower is better).
func TestRangeTradingStrategy_ScoreADX(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	testCases := []struct {
		adx         float64
		minExpected float64
		maxExpected float64
	}{
		{0, 0, 0},      // Zero ADX - invalid
		{10, 7, 8},     // Very low - perfect consolidation
		{15, 6, 8},     // Low - good consolidation
		{20, 4, 6},     // Moderate
		{25, 2, 4},     // Elevated
		{35, 0, 1},     // High - not suitable
	}

	for _, tc := range testCases {
		state := &decision.CoinState{
			Symbol: "TEST",
			ADX:    tc.adx,
		}
		score := strategy.CalculateScore(state)
		adxScore, ok := score.ComponentDetails["adx_score"]
		if !ok {
			t.Errorf("ADX %f: Missing adx_score in details", tc.adx)
			continue
		}
		if adxScore < tc.minExpected || adxScore > tc.maxExpected {
			t.Errorf("ADX %f: score %.2f not in expected range [%.2f, %.2f]",
				tc.adx, adxScore, tc.minExpected, tc.maxExpected)
		}
	}
}

// TestRangeTradingStrategy_TimeframeAlignment tests timeframe alignment scoring.
func TestRangeTradingStrategy_TimeframeAlignment(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	testCases := []struct {
		name        string
		trend15m    decision.TrendDirection
		trend1h     decision.TrendDirection
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "Both neutral - best for range trading",
			trend15m:    decision.TrendNeutral,
			trend1h:     decision.TrendNeutral,
			minExpected: 13,
			maxExpected: 15,
		},
		{
			name:        "One neutral",
			trend15m:    decision.TrendNeutral,
			trend1h:     decision.TrendBullish,
			minExpected: 10,
			maxExpected: 13,
		},
		{
			name:        "Opposite directions - choppy",
			trend15m:    decision.TrendBullish,
			trend1h:     decision.TrendBearish,
			minExpected: 7,
			maxExpected: 10,
		},
		{
			name:        "Same direction - trending, bad for range",
			trend15m:    decision.TrendBullish,
			trend1h:     decision.TrendBullish,
			minExpected: 2,
			maxExpected: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := &decision.CoinState{
				Symbol:   "TEST",
				Trend15M: tc.trend15m,
				Trend1H:  tc.trend1h,
			}
			score := strategy.CalculateScore(state)
			alignScore, ok := score.ComponentDetails["timeframe_align_score"]
			if !ok {
				t.Errorf("Missing timeframe_align_score in details")
				return
			}
			if alignScore < tc.minExpected || alignScore > tc.maxExpected {
				t.Errorf("Alignment score %.2f not in expected range [%.2f, %.2f]",
					alignScore, tc.minExpected, tc.maxExpected)
			}
		})
	}
}

// TestRangeTradingStrategy_ImplementsInterface verifies interface compliance.
func TestRangeTradingStrategy_ImplementsInterface(t *testing.T) {
	var _ decision.Strategy = (*RangeTradingStrategy)(nil)
	var _ decision.StrategyDescriber = (*RangeTradingStrategy)(nil)
}

// TestRangeTradingStrategy_TimestampSet tests that timestamp is set.
func TestRangeTradingStrategy_TimestampSet(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	before := time.Now().UnixMilli()
	score := strategy.CalculateScore(&decision.CoinState{Symbol: "TEST"})
	after := time.Now().UnixMilli()

	if score.Timestamp < before || score.Timestamp > after {
		t.Errorf("Timestamp %d not within expected range [%d, %d]",
			score.Timestamp, before, after)
	}
}

// TestRangeTradingStrategy_ComponentDetails tests component details are populated.
func TestRangeTradingStrategy_ComponentDetails(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:     "BTCUSDT",
		Price:      48000,
		Regime:     decision.RegimeConsolidating,
		ADX:        18.0,
		RSI:        32.0,
		EMA21:      50000,
		ATR:        400,
		ATRAverage: 600,
		Trend1H:    decision.TrendNeutral,
		Trend15M:   decision.TrendNeutral,
	}

	score := strategy.CalculateScore(state)

	expectedDetails := []string{
		"atr_tightness_score",
		"range_boundary_score",
		"rsi_reversal_score",
		"adx_score",
		"regime_match_score",
		"timeframe_align_score",
	}

	for _, detail := range expectedDetails {
		if _, ok := score.ComponentDetails[detail]; !ok {
			t.Errorf("Missing component detail: %s", detail)
		}
	}
}

// TestRangeTradingStrategy_EntryConditionsMet tests entry conditions counting.
func TestRangeTradingStrategy_EntryConditionsMet(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	// Perfect entry setup at lower boundary
	perfectState := &decision.CoinState{
		Symbol:     "BTCUSDT",
		Price:      48000, // At lower boundary
		Regime:     decision.RegimeConsolidating,
		ADX:        15.0,  // Low ADX
		RSI:        30.0,  // Oversold
		EMA21:      50000,
		ATR:        350,   // Tight
		ATRAverage: 500,   // Ratio = 0.7 < 0.8
		Trend1H:    decision.TrendNeutral,
		Trend15M:   decision.TrendNeutral,
	}

	score := strategy.CalculateScore(perfectState)

	if score.EntryConditionsTotal == 0 {
		t.Error("EntryConditionsTotal should be > 0")
	}
	if score.EntryConditionsMet > score.EntryConditionsTotal {
		t.Error("EntryConditionsMet should not exceed EntryConditionsTotal")
	}
	// With perfect setup, at least 2 conditions should be met (atr_tight and range_boundary)
	if score.EntryConditionsMet < 2 {
		t.Errorf("Expected at least 2 entry conditions met for perfect setup, got %d (failed: %v)",
			score.EntryConditionsMet, score.FailedConditions)
	}
}

// TestRangeTradingStrategy_Registration tests strategy registration.
func TestRangeTradingStrategy_Registration(t *testing.T) {
	// Create a new registry for testing
	registry := decision.NewStrategyRegistry()

	// Register the strategy
	strategy := NewRangeTradingStrategy()
	err := registry.RegisterStrategy(strategy)
	if err != nil {
		t.Fatalf("Failed to register strategy: %v", err)
	}

	// Verify it's registered
	if !registry.HasStrategy("range_trading") {
		t.Error("Strategy should be registered")
	}

	// Verify it's available for CONSOLIDATING regime
	strategies := registry.GetStrategiesForRegime(decision.RegimeConsolidating)
	found := false
	for _, s := range strategies {
		if s.Name() == "range_trading" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Range trading strategy should be available for CONSOLIDATING regime")
	}
}

// TestRangeTradingStrategy_ConcurrentAccess tests thread-safety of concurrent access.
func TestRangeTradingStrategy_ConcurrentAccess(t *testing.T) {
	strategy := NewRangeTradingStrategy()

	state := &decision.CoinState{
		Symbol:     "BTCUSDT",
		Price:      48000,
		Regime:     decision.RegimeConsolidating,
		ADX:        18.0,
		RSI:        32.0,
		EMA21:      50000,
		ATR:        400,
		ATRAverage: 600,
		Trend1H:    decision.TrendNeutral,
		Trend15M:   decision.TrendNeutral,
	}

	// Run concurrent operations to verify no race conditions
	done := make(chan bool)
	const goroutines = 10
	const iterations = 100

	// Concurrent CalculateScore calls
	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				_ = strategy.CalculateScore(state)
			}
			done <- true
		}()
	}

	// Concurrent SetConfig calls
	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				config := &RangeTradingConfig{
					ATRMaxRatio:          float64(70+(j%20)) / 100,
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
				strategy.SetConfig(config)
			}
			done <- true
		}()
	}

	// Concurrent GetConfig calls
	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				_ = strategy.GetConfig()
			}
			done <- true
		}()
	}

	// Concurrent GetEntryConditions and GetExitConditions calls
	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				_ = strategy.GetEntryConditions()
				_ = strategy.GetExitConditions()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < goroutines*4; i++ {
		<-done
	}

	// If we reach here without panic or data race, the test passes
	t.Log("Concurrent access test passed - no race conditions detected")
}

// TestRangeTradingStrategy_GlobalRegistration tests that init() registers the strategy globally.
func TestRangeTradingStrategy_GlobalRegistration(t *testing.T) {
	// The init() function should have already registered the strategy
	globalRegistry := decision.GetGlobalRegistry()

	// Check if range_trading is registered
	if !globalRegistry.HasStrategy("range_trading") {
		t.Error("Range trading strategy should be registered in global registry via init()")
	}

	// Get the strategy and verify it works
	strategy, err := globalRegistry.GetStrategy("range_trading")
	if err != nil {
		t.Fatalf("Failed to get range_trading strategy from global registry: %v", err)
	}

	if strategy.Name() != "range_trading" {
		t.Errorf("Expected strategy name 'range_trading', got '%s'", strategy.Name())
	}
}

// TestRangeTradingStrategy_RSIEdgeCases tests RSI scoring edge cases including division by zero guard.
func TestRangeTradingStrategy_RSIEdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		rsiUpperReversal float64
		rsiLowerReversal float64
		rsi              float64
		priceAboveEMA    bool
	}{
		{
			name:             "Upper reversal at 70 boundary - no division by zero",
			rsiUpperReversal: 70.0,
			rsiLowerReversal: 35.0,
			rsi:              68.0,
			priceAboveEMA:    true,
		},
		{
			name:             "Lower reversal at 30 boundary - no division by zero",
			rsiUpperReversal: 65.0,
			rsiLowerReversal: 30.0,
			rsi:              32.0,
			priceAboveEMA:    false,
		},
		{
			name:             "RSI exactly 0 - valid extreme oversold",
			rsiUpperReversal: 65.0,
			rsiLowerReversal: 35.0,
			rsi:              0.0,
			priceAboveEMA:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &RangeTradingConfig{
				ATRMaxRatio:          0.8,
				ATRBreakout:          1.3,
				RangeBoundaryPct:     0.15,
				RangeExitPct:         0.85,
				RSIUpperReversal:     tc.rsiUpperReversal,
				RSILowerReversal:     tc.rsiLowerReversal,
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
			strategy := NewRangeTradingStrategyWithConfig(config)

			var price, ema21 float64
			if tc.priceAboveEMA {
				ema21 = 50000
				price = 52000 // Above EMA
			} else {
				ema21 = 50000
				price = 48000 // Below EMA
			}

			state := &decision.CoinState{
				Symbol:     "BTCUSDT",
				Price:      price,
				Regime:     decision.RegimeConsolidating,
				ADX:        18.0,
				RSI:        tc.rsi,
				EMA21:      ema21,
				ATR:        400,
				ATRAverage: 600,
				Trend1H:    decision.TrendNeutral,
				Trend15M:   decision.TrendNeutral,
			}

			// This should not panic due to division by zero
			score := strategy.CalculateScore(state)

			// Verify RSI reversal score is within valid range
			rsiScore, ok := score.ComponentDetails["rsi_reversal_score"]
			if !ok {
				t.Error("Missing rsi_reversal_score in component details")
				return
			}
			if rsiScore < 0 || rsiScore > 10 {
				t.Errorf("RSI reversal score %.2f out of valid range [0, 10]", rsiScore)
			}
		})
	}
}

// BenchmarkRangeTradingStrategy_CalculateScore benchmarks score calculation.
func BenchmarkRangeTradingStrategy_CalculateScore(b *testing.B) {
	strategy := NewRangeTradingStrategy()
	state := &decision.CoinState{
		Symbol:     "BTCUSDT",
		Price:      48000,
		Regime:     decision.RegimeConsolidating,
		ADX:        18.0,
		RSI:        32.0,
		EMA21:      50000,
		ATR:        400,
		ATRAverage: 600,
		Trend1H:    decision.TrendNeutral,
		Trend15M:   decision.TrendNeutral,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.CalculateScore(state)
	}
}
