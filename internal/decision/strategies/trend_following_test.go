// Package strategies provides trading strategy implementations for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.8: Trend Following Strategy Tests
package strategies

import (
	"testing"
	"time"

	"binance-trading-bot/internal/decision"
)

// TestTrendFollowingStrategy_Name tests the Name() method.
func TestTrendFollowingStrategy_Name(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	name := strategy.Name()
	if name != "trend_following" {
		t.Errorf("Expected name 'trend_following', got '%s'", name)
	}
}

// TestTrendFollowingStrategy_Description tests the Description() method.
func TestTrendFollowingStrategy_Description(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	desc := strategy.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
	if len(desc) < 20 {
		t.Error("Description should be meaningful (at least 20 chars)")
	}
}

// TestTrendFollowingStrategy_SupportedRegimes tests the SupportedRegimes() method.
func TestTrendFollowingStrategy_SupportedRegimes(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	regimes := strategy.SupportedRegimes()
	if len(regimes) == 0 {
		t.Error("SupportedRegimes should not be empty")
	}

	// Should support TRENDING regime
	found := false
	for _, r := range regimes {
		if r == decision.RegimeTrending {
			found = true
			break
		}
	}
	if !found {
		t.Error("TrendFollowingStrategy should support TRENDING regime")
	}
}

// TestTrendFollowingStrategy_RequiredIndicators tests the RequiredIndicators() method.
func TestTrendFollowingStrategy_RequiredIndicators(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	indicators := strategy.RequiredIndicators()
	if len(indicators) == 0 {
		t.Error("RequiredIndicators should not be empty")
	}

	// Check for required indicators
	required := []string{"adx", "ema_21", "rsi", "volume", "trend_15m", "trend_1h"}
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

// TestTrendFollowingStrategy_GetEntryConditions tests the GetEntryConditions() method.
func TestTrendFollowingStrategy_GetEntryConditions(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

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

	// Check for ADX condition
	hasADX := false
	for _, cond := range conditions {
		if cond.Name == "adx_strength" {
			hasADX = true
			if cond.Threshold != 25.0 {
				t.Errorf("ADX threshold should be 25.0, got %f", cond.Threshold)
			}
			if !cond.Required {
				t.Error("ADX condition should be required")
			}
			break
		}
	}
	if !hasADX {
		t.Error("Missing ADX strength entry condition")
	}
}

// TestTrendFollowingStrategy_GetExitConditions tests the GetExitConditions() method.
func TestTrendFollowingStrategy_GetExitConditions(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	conditions := strategy.GetExitConditions()
	if len(conditions) == 0 {
		t.Error("GetExitConditions should return at least one condition")
	}

	// Check for ADX weakness condition
	hasADXWeakness := false
	for _, cond := range conditions {
		if cond.Name == "adx_weakness" {
			hasADXWeakness = true
			if cond.Threshold != 20.0 {
				t.Errorf("ADX exit threshold should be 20.0, got %f", cond.Threshold)
			}
			break
		}
	}
	if !hasADXWeakness {
		t.Error("Missing ADX weakness exit condition")
	}
}

// TestTrendFollowingStrategy_CalculateScore_NilState tests scoring with nil state.
func TestTrendFollowingStrategy_CalculateScore_NilState(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	score := strategy.CalculateScore(nil)

	if score.StrategyName != "trend_following" {
		t.Errorf("Expected strategy name 'trend_following', got '%s'", score.StrategyName)
	}
	if score.Recommendation != decision.RecommendAvoid {
		t.Errorf("Expected AVOID recommendation for nil state, got %s", score.Recommendation)
	}
	if len(score.BlockingReasons) == 0 {
		t.Error("Expected blocking reasons for nil state")
	}
}

// TestTrendFollowingStrategy_CalculateScore_StrongBullishTrend tests scoring in strong bullish trend.
func TestTrendFollowingStrategy_CalculateScore_StrongBullishTrend(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	state := &decision.CoinState{
		Symbol:       "BTCUSDT",
		Price:        50000,
		Regime:       decision.RegimeTrending,
		ADX:          35.0, // Strong trend
		RSI:          55.0, // In range
		EMA21:        49500, // Price above EMA
		Trend1H:      decision.TrendBullish,
		Trend15M:     decision.TrendBullish, // Aligned
		LastUpdated:  time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Technical score should be good
	if score.TechnicalScore < 20 {
		t.Errorf("Expected technical score >= 20 for strong bullish trend, got %.2f", score.TechnicalScore)
	}

	// Context score should be high due to regime match and alignment
	if score.ContextScore < 20 {
		t.Errorf("Expected context score >= 20 for trending regime with alignment, got %.2f", score.ContextScore)
	}

	// Entry conditions should mostly pass
	if score.EntryConditionsMet < 2 {
		t.Errorf("Expected at least 2 entry conditions met, got %d", score.EntryConditionsMet)
	}

	// Should recommend entering long
	if score.Recommendation != decision.RecommendEnterLong && score.Recommendation != decision.RecommendHold {
		t.Errorf("Expected ENTER_LONG or HOLD for strong bullish trend, got %s", score.Recommendation)
	}
}

// TestTrendFollowingStrategy_CalculateScore_StrongBearishTrend tests scoring in strong bearish trend.
func TestTrendFollowingStrategy_CalculateScore_StrongBearishTrend(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       48000,
		Regime:      decision.RegimeTrending,
		ADX:         40.0, // Very strong trend
		RSI:         45.0, // In range for shorts
		EMA21:       49000, // Price below EMA
		Trend1H:     decision.TrendBearish,
		Trend15M:    decision.TrendBearish, // Aligned
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// ADX score should be high
	adxScore, ok := score.ComponentDetails["adx_score"]
	if !ok || adxScore < 8 {
		t.Errorf("Expected ADX score >= 8 for ADX 40, got %.2f", adxScore)
	}

	// Should have trend alignment
	alignScore, ok := score.ComponentDetails["timeframe_align_score"]
	if !ok || alignScore < 10 {
		t.Errorf("Expected alignment score >= 10 for aligned trends, got %.2f", alignScore)
	}
}

// TestTrendFollowingStrategy_CalculateScore_WeakTrend tests scoring in weak trend.
func TestTrendFollowingStrategy_CalculateScore_WeakTrend(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50000,
		Regime:      decision.RegimeTrending,
		ADX:         18.0, // Below entry threshold
		RSI:         50.0,
		EMA21:       49800,
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// ADX condition should fail
	conditionFailed := false
	for _, name := range score.FailedConditions {
		if name == "adx_strength" {
			conditionFailed = true
			break
		}
	}
	if !conditionFailed {
		t.Error("ADX strength condition should have failed for ADX < 25")
	}

	// Should have blocking reason
	if len(score.BlockingReasons) == 0 {
		t.Error("Expected blocking reason for weak ADX")
	}
}

// TestTrendFollowingStrategy_CalculateScore_TrendDivergence tests scoring with divergent trends.
func TestTrendFollowingStrategy_CalculateScore_TrendDivergence(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50000,
		Regime:      decision.RegimeTrending,
		ADX:         30.0,
		RSI:         50.0,
		EMA21:       49500,
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBearish, // Divergent!
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Alignment score should be low
	alignScore, ok := score.ComponentDetails["timeframe_align_score"]
	if ok && alignScore > 5 {
		t.Errorf("Expected low alignment score for divergent trends, got %.2f", alignScore)
	}

	// Trend alignment condition should fail
	conditionFailed := false
	for _, name := range score.FailedConditions {
		if name == "trend_alignment" {
			conditionFailed = true
			break
		}
	}
	if !conditionFailed {
		t.Error("Trend alignment condition should have failed for divergent trends")
	}
}

// TestTrendFollowingStrategy_CalculateScore_OverboughtRSI tests scoring with overbought RSI.
func TestTrendFollowingStrategy_CalculateScore_OverboughtRSI(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50000,
		Regime:      decision.RegimeTrending,
		ADX:         30.0,
		RSI:         78.0, // Overbought
		EMA21:       49500,
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// RSI range condition should fail
	conditionFailed := false
	for _, name := range score.FailedConditions {
		if name == "rsi_range" {
			conditionFailed = true
			break
		}
	}
	if !conditionFailed {
		t.Error("RSI range condition should have failed for overbought RSI > 70")
	}
}

// TestTrendFollowingStrategy_CalculateScore_RangingMarket tests scoring in ranging market.
func TestTrendFollowingStrategy_CalculateScore_RangingMarket(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50000,
		Regime:      decision.RegimeRanging, // Wrong regime for trend following
		ADX:         35.0,
		RSI:         50.0,
		EMA21:       49800,
		Trend1H:     decision.TrendNeutral,
		Trend15M:    decision.TrendNeutral,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Regime match score should be low
	regimeScore, ok := score.ComponentDetails["regime_match_score"]
	if ok && regimeScore > 5 {
		t.Errorf("Expected low regime match score for ranging market, got %.2f", regimeScore)
	}

	// Should not recommend entry
	if score.Recommendation == decision.RecommendEnterLong || score.Recommendation == decision.RecommendEnterShort {
		t.Errorf("Should not recommend entry in ranging market, got %s", score.Recommendation)
	}
}

// TestTrendFollowingStrategy_CalculateScore_ExitConditions tests exit condition triggering.
func TestTrendFollowingStrategy_CalculateScore_ExitConditions(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	// State where ADX has dropped below exit threshold
	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       48000,
		Regime:      decision.RegimeTrending,
		ADX:         15.0, // Below exit threshold of 20
		RSI:         50.0,
		EMA21:       50000, // Price crossed below EMA
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Should recommend exit
	if score.Recommendation != decision.RecommendExit && score.Recommendation != decision.RecommendAvoid {
		t.Errorf("Expected EXIT or AVOID when ADX < 20, got %s", score.Recommendation)
	}
}

// TestTrendFollowingStrategy_CalculateScore_EmaCrossAgainstTrend tests EMA cross exit for both directions.
func TestTrendFollowingStrategy_CalculateScore_EmaCrossAgainstTrend(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	// Test 1: Bullish trend, price below EMA (should trigger exit)
	bullishState := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       48000,
		Regime:      decision.RegimeTrending,
		ADX:         30.0, // Strong trend
		RSI:         50.0,
		EMA21:       50000, // Price below EMA in bullish trend
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	bullishScore := strategy.CalculateScore(bullishState)
	if bullishScore.Recommendation != decision.RecommendExit {
		t.Errorf("Expected EXIT for bullish trend with price below EMA, got %s", bullishScore.Recommendation)
	}

	// Test 2: Bearish trend, price above EMA (should trigger exit)
	bearishState := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       52000,
		Regime:      decision.RegimeTrending,
		ADX:         30.0, // Strong trend
		RSI:         45.0,
		EMA21:       50000, // Price above EMA in bearish trend
		Trend1H:     decision.TrendBearish,
		Trend15M:    decision.TrendBearish,
		LastUpdated: time.Now().UnixMilli(),
	}

	bearishScore := strategy.CalculateScore(bearishState)
	if bearishScore.Recommendation != decision.RecommendExit {
		t.Errorf("Expected EXIT for bearish trend with price above EMA, got %s", bearishScore.Recommendation)
	}

	// Test 3: Bearish trend, price below EMA (should NOT trigger exit)
	bearishCorrectState := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       48000,
		Regime:      decision.RegimeTrending,
		ADX:         30.0,
		RSI:         45.0,
		EMA21:       50000, // Price below EMA in bearish trend (correct)
		Trend1H:     decision.TrendBearish,
		Trend15M:    decision.TrendBearish,
		LastUpdated: time.Now().UnixMilli(),
	}

	bearishCorrectScore := strategy.CalculateScore(bearishCorrectState)
	if bearishCorrectScore.Recommendation == decision.RecommendExit {
		t.Errorf("Should NOT recommend EXIT for bearish trend with price below EMA, got %s", bearishCorrectScore.Recommendation)
	}
}

// TestTrendFollowingStrategy_CalculateScore_ScoreRange tests score is within valid range.
func TestTrendFollowingStrategy_CalculateScore_ScoreRange(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	testCases := []struct {
		name  string
		state *decision.CoinState
	}{
		{
			name: "Perfect bullish setup",
			state: &decision.CoinState{
				Symbol: "BTCUSDT", Price: 50000, Regime: decision.RegimeTrending,
				ADX: 50, RSI: 55, EMA21: 49000,
				Trend1H: decision.TrendBullish, Trend15M: decision.TrendBullish,
				ScoreLLM: 80, ScoreHistory: 70,
			},
		},
		{
			name: "Weak setup",
			state: &decision.CoinState{
				Symbol: "BTCUSDT", Price: 50000, Regime: decision.RegimeRanging,
				ADX: 10, RSI: 85, EMA21: 51000,
				Trend1H: decision.TrendBearish, Trend15M: decision.TrendBullish,
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

// TestTrendFollowingStrategy_Config tests configuration.
func TestTrendFollowingStrategy_Config(t *testing.T) {
	// Test default config
	strategy := NewTrendFollowingStrategy()
	config := strategy.GetConfig()

	if config.ADXEntryMin != 25.0 {
		t.Errorf("Expected default ADXEntryMin 25.0, got %f", config.ADXEntryMin)
	}
	if config.ADXExitMin != 20.0 {
		t.Errorf("Expected default ADXExitMin 20.0, got %f", config.ADXExitMin)
	}
	if config.RSILongMin != 40.0 {
		t.Errorf("Expected default RSILongMin 40.0, got %f", config.RSILongMin)
	}
	if config.RSILongMax != 70.0 {
		t.Errorf("Expected default RSILongMax 70.0, got %f", config.RSILongMax)
	}
	if config.EntryScoreThreshold != 55.0 {
		t.Errorf("Expected default EntryScoreThreshold 55.0, got %f", config.EntryScoreThreshold)
	}

	// Test custom config
	customConfig := &TrendFollowingConfig{
		ADXEntryMin:       30.0,
		ADXExitMin:        25.0,
		RSILongMin:        35.0,
		RSILongMax:        75.0,
		RSIShortMin:       25.0,
		RSIShortMax:       65.0,
		VolumeMultiplier:  1.5,
		ADXWeight:         12.0,
		EMAPositionWeight: 8.0,
		RSIWeight:         12.0,
		VolumeWeight:      8.0,
		RegimeMatchWeight: 18.0,
		TimeframeAlignWeight: 12.0,
	}

	strategyCustom := NewTrendFollowingStrategyWithConfig(customConfig)
	configCustom := strategyCustom.GetConfig()

	if configCustom.ADXEntryMin != 30.0 {
		t.Errorf("Expected custom ADXEntryMin 30.0, got %f", configCustom.ADXEntryMin)
	}

	// Test nil config falls back to default
	strategyNil := NewTrendFollowingStrategyWithConfig(nil)
	configNil := strategyNil.GetConfig()
	if configNil.ADXEntryMin != 25.0 {
		t.Errorf("Expected default ADXEntryMin 25.0 for nil config, got %f", configNil.ADXEntryMin)
	}
}

// TestTrendFollowingStrategy_SetConfig tests setting configuration.
func TestTrendFollowingStrategy_SetConfig(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	newConfig := &TrendFollowingConfig{
		ADXEntryMin: 28.0,
		ADXExitMin:  22.0,
	}

	strategy.SetConfig(newConfig)
	config := strategy.GetConfig()

	if config.ADXEntryMin != 28.0 {
		t.Errorf("Expected updated ADXEntryMin 28.0, got %f", config.ADXEntryMin)
	}

	// Setting nil should not change config
	currentADXEntry := strategy.GetConfig().ADXEntryMin
	strategy.SetConfig(nil)
	if strategy.GetConfig().ADXEntryMin != currentADXEntry {
		t.Error("SetConfig(nil) should not change config")
	}
}

// TestTrendFollowingStrategy_ScoreADX tests ADX scoring.
func TestTrendFollowingStrategy_ScoreADX(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	testCases := []struct {
		adx         float64
		minExpected float64
		maxExpected float64
	}{
		{0, 0, 0},
		{10, 0, 3},     // Weak trend
		{20, 2, 5},     // Transitioning
		{25, 4, 6},     // Entry threshold
		{35, 6, 9},     // Strong
		{50, 8, 10},    // Very strong
		{70, 9, 10},    // Extremely strong
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

// TestTrendFollowingStrategy_ImplementsInterface verifies interface compliance.
func TestTrendFollowingStrategy_ImplementsInterface(t *testing.T) {
	var _ decision.Strategy = (*TrendFollowingStrategy)(nil)
	var _ decision.StrategyDescriber = (*TrendFollowingStrategy)(nil)
}

// TestTrendFollowingStrategy_TimestampSet tests that timestamp is set.
func TestTrendFollowingStrategy_TimestampSet(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	before := time.Now().UnixMilli()
	score := strategy.CalculateScore(&decision.CoinState{Symbol: "TEST"})
	after := time.Now().UnixMilli()

	if score.Timestamp < before || score.Timestamp > after {
		t.Errorf("Timestamp %d not within expected range [%d, %d]",
			score.Timestamp, before, after)
	}
}

// TestTrendFollowingStrategy_ComponentDetails tests component details are populated.
func TestTrendFollowingStrategy_ComponentDetails(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	state := &decision.CoinState{
		Symbol:   "BTCUSDT",
		Price:    50000,
		Regime:   decision.RegimeTrending,
		ADX:      30.0,
		RSI:      55.0,
		EMA21:    49500,
		Trend1H:  decision.TrendBullish,
		Trend15M: decision.TrendBullish,
	}

	score := strategy.CalculateScore(state)

	expectedDetails := []string{
		"adx_score",
		"ema_position_score",
		"rsi_score",
		"volume_score",
		"regime_match_score",
		"timeframe_align_score",
	}

	for _, detail := range expectedDetails {
		if _, ok := score.ComponentDetails[detail]; !ok {
			t.Errorf("Missing component detail: %s", detail)
		}
	}
}

// TestTrendFollowingStrategy_EntryConditionsMet tests entry conditions counting.
func TestTrendFollowingStrategy_EntryConditionsMet(t *testing.T) {
	strategy := NewTrendFollowingStrategy()

	// Perfect entry setup
	perfectState := &decision.CoinState{
		Symbol:   "BTCUSDT",
		Price:    50500,
		Regime:   decision.RegimeTrending,
		ADX:      30.0, // > 25
		RSI:      55.0, // 40-70
		EMA21:    50000,
		Trend1H:  decision.TrendBullish,
		Trend15M: decision.TrendBullish, // Aligned
	}

	score := strategy.CalculateScore(perfectState)

	if score.EntryConditionsTotal == 0 {
		t.Error("EntryConditionsTotal should be > 0")
	}
	if score.EntryConditionsMet > score.EntryConditionsTotal {
		t.Error("EntryConditionsMet should not exceed EntryConditionsTotal")
	}
}

// BenchmarkTrendFollowingStrategy_CalculateScore benchmarks score calculation.
func BenchmarkTrendFollowingStrategy_CalculateScore(b *testing.B) {
	strategy := NewTrendFollowingStrategy()
	state := &decision.CoinState{
		Symbol:   "BTCUSDT",
		Price:    50000,
		Regime:   decision.RegimeTrending,
		ADX:      30.0,
		RSI:      55.0,
		EMA21:    49500,
		Trend1H:  decision.TrendBullish,
		Trend15M: decision.TrendBullish,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.CalculateScore(state)
	}
}
