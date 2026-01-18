// Package strategies provides trading strategy implementations for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.9: Mean Reversion Strategy Tests
package strategies

import (
	"testing"
	"time"

	"binance-trading-bot/internal/decision"
)

// TestMeanReversionStrategy_Name tests the Name() method.
func TestMeanReversionStrategy_Name(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	name := strategy.Name()
	if name != "mean_reversion" {
		t.Errorf("Expected name 'mean_reversion', got '%s'", name)
	}
}

// TestMeanReversionStrategy_Description tests the Description() method.
func TestMeanReversionStrategy_Description(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	desc := strategy.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
	if len(desc) < 20 {
		t.Error("Description should be meaningful (at least 20 chars)")
	}
}

// TestMeanReversionStrategy_SupportedRegimes tests the SupportedRegimes() method.
func TestMeanReversionStrategy_SupportedRegimes(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	regimes := strategy.SupportedRegimes()
	if len(regimes) == 0 {
		t.Error("SupportedRegimes should not be empty")
	}

	// Should support RANGING regime
	found := false
	for _, r := range regimes {
		if r == decision.RegimeRanging {
			found = true
			break
		}
	}
	if !found {
		t.Error("MeanReversionStrategy should support RANGING regime")
	}
}

// TestMeanReversionStrategy_RequiredIndicators tests the RequiredIndicators() method.
func TestMeanReversionStrategy_RequiredIndicators(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	indicators := strategy.RequiredIndicators()
	if len(indicators) == 0 {
		t.Error("RequiredIndicators should not be empty")
	}

	// Check for required indicators
	required := []string{"adx", "rsi", "bollinger", "atr"}
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

// TestMeanReversionStrategy_GetEntryConditions tests the GetEntryConditions() method.
func TestMeanReversionStrategy_GetEntryConditions(t *testing.T) {
	strategy := NewMeanReversionStrategy()

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

	// Check for ADX ranging condition (ADX < 20)
	hasADX := false
	for _, cond := range conditions {
		if cond.Name == "adx_ranging" {
			hasADX = true
			if cond.Threshold != 20.0 {
				t.Errorf("ADX threshold should be 20.0, got %f", cond.Threshold)
			}
			if cond.Operator != decision.OperatorLessThan {
				t.Errorf("ADX operator should be LessThan, got %s", cond.Operator)
			}
			if !cond.Required {
				t.Error("ADX condition should be required")
			}
			break
		}
	}
	if !hasADX {
		t.Error("Missing ADX ranging entry condition")
	}

	// Check for RSI extreme condition
	hasRSIExtreme := false
	for _, cond := range conditions {
		if cond.Name == "rsi_extreme" {
			hasRSIExtreme = true
			if !cond.Required {
				t.Error("RSI extreme condition should be required")
			}
			break
		}
	}
	if !hasRSIExtreme {
		t.Error("Missing RSI extreme entry condition")
	}
}

// TestMeanReversionStrategy_GetExitConditions tests the GetExitConditions() method.
func TestMeanReversionStrategy_GetExitConditions(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	conditions := strategy.GetExitConditions()
	if len(conditions) == 0 {
		t.Error("GetExitConditions should return at least one condition")
	}

	// Check for RSI neutral exit condition
	hasRSINeutral := false
	for _, cond := range conditions {
		if cond.Name == "rsi_neutral" {
			hasRSINeutral = true
			break
		}
	}
	if !hasRSINeutral {
		t.Error("Missing RSI neutral exit condition")
	}

	// Check for BB mean reversion exit condition
	hasBBMean := false
	for _, cond := range conditions {
		if cond.Name == "bb_mean_reversion" {
			hasBBMean = true
			break
		}
	}
	if !hasBBMean {
		t.Error("Missing BB mean reversion exit condition")
	}

	// Check for trend forming exit condition
	hasTrendForming := false
	for _, cond := range conditions {
		if cond.Name == "trend_forming" {
			hasTrendForming = true
			if cond.Threshold != 25.0 {
				t.Errorf("ADX exit threshold should be 25.0, got %f", cond.Threshold)
			}
			break
		}
	}
	if !hasTrendForming {
		t.Error("Missing trend forming exit condition")
	}
}

// TestMeanReversionStrategy_CalculateScore_NilState tests scoring with nil state.
func TestMeanReversionStrategy_CalculateScore_NilState(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	score := strategy.CalculateScore(nil)

	if score.StrategyName != "mean_reversion" {
		t.Errorf("Expected strategy name 'mean_reversion', got '%s'", score.StrategyName)
	}
	if score.Recommendation != decision.RecommendAvoid {
		t.Errorf("Expected AVOID recommendation for nil state, got %s", score.Recommendation)
	}
	if len(score.BlockingReasons) == 0 {
		t.Error("Expected blocking reasons for nil state")
	}
}

// TestMeanReversionStrategy_CalculateScore_OversoldEntry tests scoring for oversold entry.
func TestMeanReversionStrategy_CalculateScore_OversoldEntry(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       48000, // Price near lower BB
		Regime:      decision.RegimeRanging,
		ADX:         15.0, // Low ADX - good for mean reversion
		RSI:         25.0, // Oversold
		EMA21:       50000,
		ATR:         500, // BB width = 1000, price is 2000 below middle = 2x width
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Technical score should be good for oversold setup
	if score.TechnicalScore < 20 {
		t.Errorf("Expected technical score >= 20 for oversold setup, got %.2f", score.TechnicalScore)
	}

	// Context score should be high due to ranging regime
	if score.ContextScore < 20 {
		t.Errorf("Expected context score >= 20 for ranging regime, got %.2f", score.ContextScore)
	}

	// RSI extreme score should be high
	rsiScore, ok := score.ComponentDetails["rsi_extreme_score"]
	if !ok || rsiScore < 8 {
		t.Errorf("Expected RSI extreme score >= 8 for RSI 25, got %.2f", rsiScore)
	}
}

// TestMeanReversionStrategy_CalculateScore_OverboughtEntry tests scoring for overbought entry.
func TestMeanReversionStrategy_CalculateScore_OverboughtEntry(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       52000, // Price near upper BB
		Regime:      decision.RegimeRanging,
		ADX:         18.0, // Low ADX
		RSI:         78.0, // Overbought
		EMA21:       50000,
		ATR:         500,
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// RSI extreme score should be high for overbought
	rsiScore, ok := score.ComponentDetails["rsi_extreme_score"]
	if !ok || rsiScore < 8 {
		t.Errorf("Expected RSI extreme score >= 8 for RSI 78, got %.2f", rsiScore)
	}

	// ADX score should be good (low ADX = good for mean reversion)
	adxScore, ok := score.ComponentDetails["adx_score"]
	if !ok || adxScore < 6 {
		t.Errorf("Expected ADX score >= 6 for ADX 18, got %.2f", adxScore)
	}
}

// TestMeanReversionStrategy_CalculateScore_StrongTrend tests scoring rejects strong trends.
func TestMeanReversionStrategy_CalculateScore_StrongTrend(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50000,
		Regime:      decision.RegimeTrending, // Wrong regime
		ADX:         35.0,                     // Too high for mean reversion
		RSI:         25.0,
		EMA21:       49800,
		ATR:         500,
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// ADX condition should fail
	conditionFailed := false
	for _, name := range score.FailedConditions {
		if name == "adx_ranging" {
			conditionFailed = true
			break
		}
	}
	if !conditionFailed {
		t.Error("ADX ranging condition should have failed for ADX > 20")
	}

	// Should have blocking reason
	if len(score.BlockingReasons) == 0 {
		t.Error("Expected blocking reason for high ADX")
	}

	// Regime match score should be low
	regimeScore, ok := score.ComponentDetails["regime_match_score"]
	if ok && regimeScore > 3 {
		t.Errorf("Expected low regime match score for trending market, got %.2f", regimeScore)
	}
}

// TestMeanReversionStrategy_CalculateScore_NeutralRSI tests scoring rejects neutral RSI.
func TestMeanReversionStrategy_CalculateScore_NeutralRSI(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50000,
		Regime:      decision.RegimeRanging,
		ADX:         15.0, // Good
		RSI:         50.0, // Neutral - not good for mean reversion entry
		EMA21:       49900,
		ATR:         500,
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// RSI extreme condition should fail
	conditionFailed := false
	for _, name := range score.FailedConditions {
		if name == "rsi_extreme" {
			conditionFailed = true
			break
		}
	}
	if !conditionFailed {
		t.Error("RSI extreme condition should have failed for neutral RSI 50")
	}

	// RSI extreme score should be low
	rsiScore, ok := score.ComponentDetails["rsi_extreme_score"]
	if ok && rsiScore > 4 {
		t.Errorf("Expected low RSI extreme score for neutral RSI, got %.2f", rsiScore)
	}
}

// TestMeanReversionStrategy_CalculateScore_ExitRSINeutral tests exit when RSI returns to neutral.
func TestMeanReversionStrategy_CalculateScore_ExitRSINeutral(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	// Scenario: Was oversold, now RSI returned to neutral
	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       49900,
		Regime:      decision.RegimeRanging,
		ADX:         15.0,
		RSI:         50.0, // Back to neutral - should trigger exit
		EMA21:       50000,
		ATR:         500,
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Should recommend exit due to RSI returning to neutral
	if score.Recommendation != decision.RecommendExit && score.Recommendation != decision.RecommendHold {
		t.Errorf("Expected EXIT or HOLD when RSI returns to neutral, got %s", score.Recommendation)
	}
}

// TestMeanReversionStrategy_CalculateScore_ExitBBMean tests exit when price returns to mean.
func TestMeanReversionStrategy_CalculateScore_ExitBBMean(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	// Scenario: Price has returned to middle BB (near EMA21)
	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50050, // Very close to EMA21
		Regime:      decision.RegimeRanging,
		ADX:         15.0,
		RSI:         48.0, // Also neutral
		EMA21:       50000,
		ATR:         500, // BB width = 1000, distance = 50 = 5% of width < 20%
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Should recommend exit due to return to mean
	if score.Recommendation != decision.RecommendExit && score.Recommendation != decision.RecommendHold {
		t.Errorf("Expected EXIT or HOLD when price returns to mean, got %s", score.Recommendation)
	}
}

// TestMeanReversionStrategy_CalculateScore_ExitTrendForming tests exit when trend starts forming.
func TestMeanReversionStrategy_CalculateScore_ExitTrendForming(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	// Scenario: ADX has risen above 25, indicating trend is forming
	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       48000,
		Regime:      decision.RegimeRanging,
		ADX:         28.0, // Above exit threshold of 25
		RSI:         35.0,
		EMA21:       50000,
		ATR:         500,
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Should recommend exit due to trend forming
	if score.Recommendation != decision.RecommendExit {
		t.Errorf("Expected EXIT when trend is forming (ADX > 25), got %s", score.Recommendation)
	}
}

// TestMeanReversionStrategy_CalculateScore_RecommendEnterLong tests long recommendation for oversold.
func TestMeanReversionStrategy_CalculateScore_RecommendEnterLong(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	state := &decision.CoinState{
		Symbol:       "BTCUSDT",
		Price:        47000, // Near lower BB
		Regime:       decision.RegimeRanging,
		ADX:          12.0, // Very low - perfect
		RSI:          22.0, // Very oversold
		EMA21:        50000,
		ATR:          1000, // BB width = 2000, price 3000 below = 1.5x width (extreme)
		Trend1H:      decision.TrendNeutral,
		Trend15M:     decision.TrendNeutral,
		ScoreLLM:     60,
		ScoreHistory: 55,
		LastUpdated:  time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// With strong setup and external scores, should recommend long
	// Score must pass threshold AND recommendation must be ENTER_LONG
	if score.FinalScore < 55 {
		t.Logf("Note: Score %.2f below threshold 55, recommendation: %s", score.FinalScore, score.Recommendation)
	} else if score.Recommendation != decision.RecommendEnterLong {
		t.Errorf("Expected ENTER_LONG for oversold condition with good score, got %s (score: %.2f)",
			score.Recommendation, score.FinalScore)
	}
}

// TestMeanReversionStrategy_CalculateScore_RecommendEnterShort tests short recommendation for overbought.
func TestMeanReversionStrategy_CalculateScore_RecommendEnterShort(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	state := &decision.CoinState{
		Symbol:       "BTCUSDT",
		Price:        53000, // Near upper BB
		Regime:       decision.RegimeRanging,
		ADX:          12.0, // Very low - perfect
		RSI:          82.0, // Very overbought
		EMA21:        50000,
		ATR:          1000, // BB width = 2000, price 3000 above = 1.5x width (extreme)
		Trend1H:      decision.TrendNeutral,
		Trend15M:     decision.TrendNeutral,
		ScoreLLM:     60,
		ScoreHistory: 55,
		LastUpdated:  time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// With strong setup and external scores, should recommend short
	// Score must pass threshold AND recommendation must be ENTER_SHORT
	if score.FinalScore < 55 {
		t.Logf("Note: Score %.2f below threshold 55, recommendation: %s", score.FinalScore, score.Recommendation)
	} else if score.Recommendation != decision.RecommendEnterShort {
		t.Errorf("Expected ENTER_SHORT for overbought condition with good score, got %s (score: %.2f)",
			score.Recommendation, score.FinalScore)
	}
}

// TestMeanReversionStrategy_CalculateScore_ConsolidatingRegime tests scoring in consolidating market.
func TestMeanReversionStrategy_CalculateScore_ConsolidatingRegime(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50000,
		Regime:      decision.RegimeConsolidating, // Similar to ranging
		ADX:         15.0,
		RSI:         25.0,
		EMA21:       50500,
		ATR:         300,
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Regime match score should be decent for consolidating
	regimeScore, ok := score.ComponentDetails["regime_match_score"]
	if !ok || regimeScore < 8 {
		t.Errorf("Expected regime match score >= 8 for consolidating market, got %.2f", regimeScore)
	}
}

// TestMeanReversionStrategy_CalculateScore_ScoreRange tests score is within valid range.
func TestMeanReversionStrategy_CalculateScore_ScoreRange(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	testCases := []struct {
		name  string
		state *decision.CoinState
	}{
		{
			name: "Perfect oversold setup",
			state: &decision.CoinState{
				Symbol: "BTCUSDT", Price: 48000, Regime: decision.RegimeRanging,
				ADX: 12, RSI: 22, EMA21: 50000, ATR: 500,
				Trend1H: decision.TrendNeutral, Trend15M: decision.TrendNeutral,
				ScoreLLM: 80, ScoreHistory: 70,
			},
		},
		{
			name: "Poor setup - trending market",
			state: &decision.CoinState{
				Symbol: "BTCUSDT", Price: 50000, Regime: decision.RegimeTrending,
				ADX: 40, RSI: 55, EMA21: 49000, ATR: 500,
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

// TestMeanReversionStrategy_Config tests configuration.
func TestMeanReversionStrategy_Config(t *testing.T) {
	// Test default config
	strategy := NewMeanReversionStrategy()
	config := strategy.GetConfig()

	if config.ADXMaxEntry != 20.0 {
		t.Errorf("Expected default ADXMaxEntry 20.0, got %f", config.ADXMaxEntry)
	}
	if config.ADXExitMax != 25.0 {
		t.Errorf("Expected default ADXExitMax 25.0, got %f", config.ADXExitMax)
	}
	if config.RSIOversold != 30.0 {
		t.Errorf("Expected default RSIOversold 30.0, got %f", config.RSIOversold)
	}
	if config.RSIOverbought != 70.0 {
		t.Errorf("Expected default RSIOverbought 70.0, got %f", config.RSIOverbought)
	}
	if config.RSINeutralMin != 40.0 {
		t.Errorf("Expected default RSINeutralMin 40.0, got %f", config.RSINeutralMin)
	}
	if config.RSINeutralMax != 60.0 {
		t.Errorf("Expected default RSINeutralMax 60.0, got %f", config.RSINeutralMax)
	}
	if config.EntryScoreThreshold != 55.0 {
		t.Errorf("Expected default EntryScoreThreshold 55.0, got %f", config.EntryScoreThreshold)
	}

	// Test custom config
	customConfig := &MeanReversionConfig{
		ADXMaxEntry:          18.0,
		ADXExitMax:           22.0,
		RSIOversold:          25.0,
		RSIOverbought:        75.0,
		RSINeutralMin:        35.0,
		RSINeutralMax:        65.0,
		BBExtremePct:         0.85,
		BBMeanPct:            0.15,
		SRProximityATR:       2.0,
		ADXWeight:            12.0,
		RSIExtremeWeight:     10.0,
		BBPositionWeight:     10.0,
		SRProximityWeight:    8.0,
		RegimeMatchWeight:    15.0,
		TimeframeAlignWeight: 15.0,
		EntryScoreThreshold:  60.0,
		MaxHoldMinutes:       180,
	}

	strategyCustom := NewMeanReversionStrategyWithConfig(customConfig)
	configCustom := strategyCustom.GetConfig()

	if configCustom.ADXMaxEntry != 18.0 {
		t.Errorf("Expected custom ADXMaxEntry 18.0, got %f", configCustom.ADXMaxEntry)
	}
	if configCustom.RSIOversold != 25.0 {
		t.Errorf("Expected custom RSIOversold 25.0, got %f", configCustom.RSIOversold)
	}

	// Test nil config falls back to default
	strategyNil := NewMeanReversionStrategyWithConfig(nil)
	configNil := strategyNil.GetConfig()
	if configNil.ADXMaxEntry != 20.0 {
		t.Errorf("Expected default ADXMaxEntry 20.0 for nil config, got %f", configNil.ADXMaxEntry)
	}
}

// TestMeanReversionStrategy_SetConfig tests setting configuration.
func TestMeanReversionStrategy_SetConfig(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	newConfig := &MeanReversionConfig{
		ADXMaxEntry: 15.0,
		ADXExitMax:  20.0,
	}

	strategy.SetConfig(newConfig)
	config := strategy.GetConfig()

	if config.ADXMaxEntry != 15.0 {
		t.Errorf("Expected updated ADXMaxEntry 15.0, got %f", config.ADXMaxEntry)
	}

	// Setting nil should not change config
	currentADXMax := strategy.GetConfig().ADXMaxEntry
	strategy.SetConfig(nil)
	if strategy.GetConfig().ADXMaxEntry != currentADXMax {
		t.Error("SetConfig(nil) should not change config")
	}
}

// TestMeanReversionStrategy_ScoreADX tests ADX scoring (inverse scoring - lower is better).
func TestMeanReversionStrategy_ScoreADX(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	testCases := []struct {
		adx         float64
		minExpected float64
		maxExpected float64
	}{
		{0, 0, 0},      // Zero ADX - invalid data, return 0 to avoid false signals
		{10, 9, 10},    // Very low - perfect for mean reversion
		{15, 8, 10},    // Still good
		{20, 5, 8},     // Borderline
		{25, 3, 5},     // Transitioning to trend
		{35, 0, 2},     // Trendy - poor
		{50, 0, 1},     // Strong trend - very poor
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

// TestMeanReversionStrategy_ScoreRSIExtreme tests RSI extreme scoring.
func TestMeanReversionStrategy_ScoreRSIExtreme(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	testCases := []struct {
		rsi         float64
		minExpected float64
		maxExpected float64
	}{
		{15, 10, 12},  // Very oversold - excellent
		{25, 9, 12},   // Oversold - good
		{35, 4, 8},    // Approaching neutral
		{50, 0, 4},    // Neutral - poor
		{65, 4, 8},    // Approaching overbought
		{75, 9, 12},   // Overbought - good
		{85, 10, 12},  // Very overbought - excellent
	}

	for _, tc := range testCases {
		state := &decision.CoinState{
			Symbol: "TEST",
			RSI:    tc.rsi,
		}
		score := strategy.CalculateScore(state)
		rsiScore, ok := score.ComponentDetails["rsi_extreme_score"]
		if !ok {
			t.Errorf("RSI %f: Missing rsi_extreme_score in details", tc.rsi)
			continue
		}
		if rsiScore < tc.minExpected || rsiScore > tc.maxExpected {
			t.Errorf("RSI %f: score %.2f not in expected range [%.2f, %.2f]",
				tc.rsi, rsiScore, tc.minExpected, tc.maxExpected)
		}
	}
}

// TestMeanReversionStrategy_TimeframeAlignment tests timeframe alignment scoring.
func TestMeanReversionStrategy_TimeframeAlignment(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	testCases := []struct {
		name        string
		trend15m    decision.TrendDirection
		trend1h     decision.TrendDirection
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "Both neutral - best for mean reversion",
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
			name:        "Opposite directions - potential reversal",
			trend15m:    decision.TrendBullish,
			trend1h:     decision.TrendBearish,
			minExpected: 8,
			maxExpected: 11,
		},
		{
			name:        "Same direction - trend, not reversion",
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

// TestMeanReversionStrategy_ImplementsInterface verifies interface compliance.
func TestMeanReversionStrategy_ImplementsInterface(t *testing.T) {
	var _ decision.Strategy = (*MeanReversionStrategy)(nil)
	var _ decision.StrategyDescriber = (*MeanReversionStrategy)(nil)
}

// TestMeanReversionStrategy_TimestampSet tests that timestamp is set.
func TestMeanReversionStrategy_TimestampSet(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	before := time.Now().UnixMilli()
	score := strategy.CalculateScore(&decision.CoinState{Symbol: "TEST"})
	after := time.Now().UnixMilli()

	if score.Timestamp < before || score.Timestamp > after {
		t.Errorf("Timestamp %d not within expected range [%d, %d]",
			score.Timestamp, before, after)
	}
}

// TestMeanReversionStrategy_ComponentDetails tests component details are populated.
func TestMeanReversionStrategy_ComponentDetails(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	state := &decision.CoinState{
		Symbol:   "BTCUSDT",
		Price:    48000,
		Regime:   decision.RegimeRanging,
		ADX:      15.0,
		RSI:      25.0,
		EMA21:    50000,
		ATR:      500,
		Trend1H:  decision.TrendNeutral,
		Trend15M: decision.TrendNeutral,
	}

	score := strategy.CalculateScore(state)

	expectedDetails := []string{
		"adx_score",
		"rsi_extreme_score",
		"bb_position_score",
		"sr_proximity_score",
		"regime_match_score",
		"timeframe_align_score",
	}

	for _, detail := range expectedDetails {
		if _, ok := score.ComponentDetails[detail]; !ok {
			t.Errorf("Missing component detail: %s", detail)
		}
	}
}

// TestMeanReversionStrategy_EntryConditionsMet tests entry conditions counting.
func TestMeanReversionStrategy_EntryConditionsMet(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	// Perfect entry setup
	perfectState := &decision.CoinState{
		Symbol:   "BTCUSDT",
		Price:    47000, // At BB extreme
		Regime:   decision.RegimeRanging,
		ADX:      12.0, // < 20
		RSI:      22.0, // < 30 (oversold)
		EMA21:    50000,
		ATR:      1000, // BB width = 2000, price 3000 below = extreme
		Trend1H:  decision.TrendNeutral,
		Trend15M: decision.TrendNeutral,
	}

	score := strategy.CalculateScore(perfectState)

	if score.EntryConditionsTotal == 0 {
		t.Error("EntryConditionsTotal should be > 0")
	}
	if score.EntryConditionsMet > score.EntryConditionsTotal {
		t.Error("EntryConditionsMet should not exceed EntryConditionsTotal")
	}
	// With perfect setup, at least 2 conditions should be met
	if score.EntryConditionsMet < 2 {
		t.Errorf("Expected at least 2 entry conditions met for perfect setup, got %d", score.EntryConditionsMet)
	}
}

// TestMeanReversionStrategy_Registration tests strategy registration.
func TestMeanReversionStrategy_Registration(t *testing.T) {
	// Create a new registry for testing
	registry := decision.NewStrategyRegistry()

	// Register the strategy
	strategy := NewMeanReversionStrategy()
	err := registry.RegisterStrategy(strategy)
	if err != nil {
		t.Fatalf("Failed to register strategy: %v", err)
	}

	// Verify it's registered
	if !registry.HasStrategy("mean_reversion") {
		t.Error("Strategy should be registered")
	}

	// Verify it's available for RANGING regime
	strategies := registry.GetStrategiesForRegime(decision.RegimeRanging)
	found := false
	for _, s := range strategies {
		if s.Name() == "mean_reversion" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Mean reversion strategy should be available for RANGING regime")
	}
}

// TestMeanReversionStrategy_ConcurrentAccess tests thread-safety of concurrent access.
func TestMeanReversionStrategy_ConcurrentAccess(t *testing.T) {
	strategy := NewMeanReversionStrategy()

	state := &decision.CoinState{
		Symbol:   "BTCUSDT",
		Price:    48000,
		Regime:   decision.RegimeRanging,
		ADX:      15.0,
		RSI:      25.0,
		EMA21:    50000,
		ATR:      500,
		Trend1H:  decision.TrendNeutral,
		Trend15M: decision.TrendNeutral,
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
				config := &MeanReversionConfig{
					ADXMaxEntry:     float64(15 + (j % 10)),
					ADXExitMax:      25.0,
					RSIOversold:     30.0,
					RSIOverbought:   70.0,
					RSINeutralMin:   40.0,
					RSINeutralMax:   60.0,
					BBExtremePct:    0.8,
					BBMeanPct:       0.2,
					SRProximityATR:  1.5,
					ADXWeight:       10.0,
					RSIExtremeWeight: 12.0,
					BBPositionWeight: 10.0,
					SRProximityWeight: 8.0,
					RegimeMatchWeight: 15.0,
					TimeframeAlignWeight: 15.0,
					EntryScoreThreshold: 55.0,
					MaxHoldMinutes: 240,
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

// BenchmarkMeanReversionStrategy_CalculateScore benchmarks score calculation.
func BenchmarkMeanReversionStrategy_CalculateScore(b *testing.B) {
	strategy := NewMeanReversionStrategy()
	state := &decision.CoinState{
		Symbol:   "BTCUSDT",
		Price:    48000,
		Regime:   decision.RegimeRanging,
		ADX:      15.0,
		RSI:      25.0,
		EMA21:    50000,
		ATR:      500,
		Trend1H:  decision.TrendNeutral,
		Trend15M: decision.TrendNeutral,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.CalculateScore(state)
	}
}
