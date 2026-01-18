// Package strategies provides trading strategy implementations for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.10: Breakout Strategy Tests
package strategies

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"binance-trading-bot/internal/decision"
)

// TestBreakoutStrategy_Name tests the Name() method.
func TestBreakoutStrategy_Name(t *testing.T) {
	strategy := NewBreakoutStrategy()

	name := strategy.Name()
	if name != "breakout" {
		t.Errorf("Expected name 'breakout', got '%s'", name)
	}
}

// TestBreakoutStrategy_Description tests the Description() method.
func TestBreakoutStrategy_Description(t *testing.T) {
	strategy := NewBreakoutStrategy()

	desc := strategy.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
	if len(desc) < 20 {
		t.Error("Description should be meaningful (at least 20 chars)")
	}
}

// TestBreakoutStrategy_SupportedRegimes tests the SupportedRegimes() method.
func TestBreakoutStrategy_SupportedRegimes(t *testing.T) {
	strategy := NewBreakoutStrategy()

	regimes := strategy.SupportedRegimes()
	if len(regimes) == 0 {
		t.Error("SupportedRegimes should not be empty")
	}

	// Should support VOLATILE regime
	found := false
	for _, r := range regimes {
		if r == decision.RegimeVolatile {
			found = true
			break
		}
	}
	if !found {
		t.Error("BreakoutStrategy should support VOLATILE regime")
	}
}

// TestBreakoutStrategy_RequiredIndicators tests the RequiredIndicators() method.
func TestBreakoutStrategy_RequiredIndicators(t *testing.T) {
	strategy := NewBreakoutStrategy()

	indicators := strategy.RequiredIndicators()
	if len(indicators) == 0 {
		t.Error("RequiredIndicators should not be empty")
	}

	// Check for required indicators
	required := []string{"atr", "volume", "momentum", "rsi"}
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

// TestBreakoutStrategy_GetEntryConditions tests the GetEntryConditions() method.
func TestBreakoutStrategy_GetEntryConditions(t *testing.T) {
	strategy := NewBreakoutStrategy()

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

	// Check for ATR expansion condition
	hasATRExpansion := false
	for _, cond := range conditions {
		if cond.Name == "atr_expansion" {
			hasATRExpansion = true
			if !cond.Required {
				t.Error("ATR expansion condition should be required")
			}
			break
		}
	}
	if !hasATRExpansion {
		t.Error("Missing ATR expansion entry condition")
	}

	// Check for momentum confirmed condition
	hasMomentum := false
	for _, cond := range conditions {
		if cond.Name == "momentum_confirmed" {
			hasMomentum = true
			break
		}
	}
	if !hasMomentum {
		t.Error("Missing momentum confirmed entry condition")
	}
}

// TestBreakoutStrategy_GetExitConditions tests the GetExitConditions() method.
func TestBreakoutStrategy_GetExitConditions(t *testing.T) {
	strategy := NewBreakoutStrategy()

	conditions := strategy.GetExitConditions()
	if len(conditions) == 0 {
		t.Error("GetExitConditions should return at least one condition")
	}

	// Check for failed breakout condition
	hasFailedBreakout := false
	for _, cond := range conditions {
		if cond.Name == "failed_breakout" {
			hasFailedBreakout = true
			break
		}
	}
	if !hasFailedBreakout {
		t.Error("Missing failed breakout exit condition")
	}

	// Check for volume declining condition
	hasVolumeDeclining := false
	for _, cond := range conditions {
		if cond.Name == "volume_declining" {
			hasVolumeDeclining = true
			break
		}
	}
	if !hasVolumeDeclining {
		t.Error("Missing volume declining exit condition")
	}
}

// TestBreakoutStrategy_CalculateScore_NilState tests scoring with nil state.
func TestBreakoutStrategy_CalculateScore_NilState(t *testing.T) {
	strategy := NewBreakoutStrategy()

	score := strategy.CalculateScore(nil)

	if score.StrategyName != "breakout" {
		t.Errorf("Expected strategy name 'breakout', got '%s'", score.StrategyName)
	}
	if score.Recommendation != decision.RecommendAvoid {
		t.Errorf("Expected AVOID recommendation for nil state, got %s", score.Recommendation)
	}
	if len(score.BlockingReasons) == 0 {
		t.Error("Expected blocking reasons for nil state")
	}
}

// TestBreakoutStrategy_CalculateScore_StrongBullishBreakout tests scoring in strong bullish breakout.
func TestBreakoutStrategy_CalculateScore_StrongBullishBreakout(t *testing.T) {
	strategy := NewBreakoutStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       52000,       // Price broke above EMA
		Regime:      decision.RegimeVolatile,
		ATR:         500,         // High ATR
		ATRAverage:  300,         // ATR > 1.5x average = expansion
		RSI:         62.0,        // Bullish momentum confirmed (> 55)
		EMA21:       50000,       // Price above EMA (4% breakout)
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish, // Aligned
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Technical score should be good for a breakout
	if score.TechnicalScore < 20 {
		t.Errorf("Expected technical score >= 20 for strong bullish breakout, got %.2f", score.TechnicalScore)
	}

	// Context score should be high due to regime match and alignment
	if score.ContextScore < 20 {
		t.Errorf("Expected context score >= 20 for volatile regime with alignment, got %.2f", score.ContextScore)
	}

	// Should recommend entering long
	if score.Recommendation != decision.RecommendEnterLong && score.Recommendation != decision.RecommendHold {
		t.Errorf("Expected ENTER_LONG or HOLD for strong bullish breakout, got %s", score.Recommendation)
	}
}

// TestBreakoutStrategy_CalculateScore_StrongBearishBreakout tests scoring in strong bearish breakout.
func TestBreakoutStrategy_CalculateScore_StrongBearishBreakout(t *testing.T) {
	strategy := NewBreakoutStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       48000,       // Price broke below EMA
		Regime:      decision.RegimeVolatile,
		ATR:         600,         // Very high ATR
		ATRAverage:  350,         // ATR > 1.7x average
		RSI:         38.0,        // Bearish momentum (< 45)
		EMA21:       50000,       // Price below EMA
		Trend1H:     decision.TrendBearish,
		Trend15M:    decision.TrendBearish, // Aligned
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// ATR expansion score should be high
	atrScore, ok := score.ComponentDetails["atr_expansion_score"]
	if !ok || atrScore < 8 {
		t.Errorf("Expected ATR expansion score >= 8 for high ATR ratio, got %.2f", atrScore)
	}

	// Should have trend alignment
	alignScore, ok := score.ComponentDetails["timeframe_align_score"]
	if !ok || alignScore < 10 {
		t.Errorf("Expected alignment score >= 10 for aligned bearish trends, got %.2f", alignScore)
	}
}

// TestBreakoutStrategy_CalculateScore_LowVolatility tests scoring when ATR is not expanded.
func TestBreakoutStrategy_CalculateScore_LowVolatility(t *testing.T) {
	strategy := NewBreakoutStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50000,
		Regime:      decision.RegimeVolatile,
		ATR:         250,         // Below average
		ATRAverage:  300,         // ATR < average = no expansion
		RSI:         55.0,
		EMA21:       49800,
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// ATR expansion condition should fail
	conditionFailed := false
	for _, name := range score.FailedConditions {
		if name == "atr_expansion" {
			conditionFailed = true
			break
		}
	}
	if !conditionFailed {
		t.Error("ATR expansion condition should have failed for ATR < average")
	}

	// Should have blocking reason
	if len(score.BlockingReasons) == 0 {
		t.Error("Expected blocking reason for low ATR")
	}
}

// TestBreakoutStrategy_CalculateScore_NoMomentumConfirmation tests scoring without momentum.
func TestBreakoutStrategy_CalculateScore_NoMomentumConfirmation(t *testing.T) {
	strategy := NewBreakoutStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       51000,
		Regime:      decision.RegimeVolatile,
		ATR:         400,
		ATRAverage:  300,
		RSI:         50.0,        // Neutral RSI - no momentum confirmation
		EMA21:       50000,
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBullish,
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Momentum condition should fail
	conditionFailed := false
	for _, name := range score.FailedConditions {
		if name == "momentum_confirmed" {
			conditionFailed = true
			break
		}
	}
	if !conditionFailed {
		t.Error("Momentum confirmed condition should have failed for neutral RSI")
	}
}

// TestBreakoutStrategy_CalculateScore_RangingMarket tests scoring in ranging market.
func TestBreakoutStrategy_CalculateScore_RangingMarket(t *testing.T) {
	strategy := NewBreakoutStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50000,
		Regime:      decision.RegimeRanging, // Wrong regime for breakouts
		ATR:         400,
		ATRAverage:  300,
		RSI:         55.0,
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

// TestBreakoutStrategy_CalculateScore_FailedBreakout tests exit condition for failed breakout.
func TestBreakoutStrategy_CalculateScore_FailedBreakout(t *testing.T) {
	strategy := NewBreakoutStrategy()

	// State where price has returned to EMA (failed breakout)
	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       50010,       // Very close to EMA (0.02% distance)
		Regime:      decision.RegimeVolatile,
		ATR:         400,
		ATRAverage:  300,
		RSI:         50.0,
		EMA21:       50000,       // Price at EMA = failed breakout
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendNeutral, // Lost momentum
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Should recommend exit due to failed breakout
	if score.Recommendation != decision.RecommendExit && score.Recommendation != decision.RecommendHold && score.Recommendation != decision.RecommendAvoid {
		t.Errorf("Expected EXIT, HOLD, or AVOID when price returns to EMA, got %s", score.Recommendation)
	}
}

// TestBreakoutStrategy_CalculateScore_TrendDivergence tests scoring with divergent trends.
func TestBreakoutStrategy_CalculateScore_TrendDivergence(t *testing.T) {
	strategy := NewBreakoutStrategy()

	state := &decision.CoinState{
		Symbol:      "BTCUSDT",
		Price:       51000,
		Regime:      decision.RegimeVolatile,
		ATR:         400,
		ATRAverage:  300,
		RSI:         55.0,
		EMA21:       50000,
		Trend1H:     decision.TrendBullish,
		Trend15M:    decision.TrendBearish, // Divergent!
		LastUpdated: time.Now().UnixMilli(),
	}

	score := strategy.CalculateScore(state)

	// Alignment score should be low
	alignScore, ok := score.ComponentDetails["timeframe_align_score"]
	if ok && alignScore > 6 {
		t.Errorf("Expected low alignment score for divergent trends, got %.2f", alignScore)
	}
}

// TestBreakoutStrategy_CalculateScore_ScoreRange tests score is within valid range.
func TestBreakoutStrategy_CalculateScore_ScoreRange(t *testing.T) {
	strategy := NewBreakoutStrategy()

	testCases := []struct {
		name  string
		state *decision.CoinState
	}{
		{
			name: "Perfect breakout setup",
			state: &decision.CoinState{
				Symbol: "BTCUSDT", Price: 52000, Regime: decision.RegimeVolatile,
				ATR: 500, ATRAverage: 300, RSI: 62, EMA21: 50000,
				Trend1H: decision.TrendBullish, Trend15M: decision.TrendBullish,
				ScoreLLM: 80, ScoreHistory: 70,
			},
		},
		{
			name: "Weak setup",
			state: &decision.CoinState{
				Symbol: "BTCUSDT", Price: 50000, Regime: decision.RegimeRanging,
				ATR: 200, ATRAverage: 300, RSI: 50, EMA21: 50000,
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

// TestBreakoutStrategy_Config tests configuration.
func TestBreakoutStrategy_Config(t *testing.T) {
	// Test default config
	strategy := NewBreakoutStrategy()
	config := strategy.GetConfig()

	if config.ATRExpansionMultiplier != 1.2 {
		t.Errorf("Expected default ATRExpansionMultiplier 1.2, got %f", config.ATRExpansionMultiplier)
	}
	if config.VolumeSpikeMultiplier != 2.0 {
		t.Errorf("Expected default VolumeSpikeMultiplier 2.0, got %f", config.VolumeSpikeMultiplier)
	}
	if config.RSIMomentumBullish != 55.0 {
		t.Errorf("Expected default RSIMomentumBullish 55.0, got %f", config.RSIMomentumBullish)
	}
	if config.RSIMomentumBearish != 45.0 {
		t.Errorf("Expected default RSIMomentumBearish 45.0, got %f", config.RSIMomentumBearish)
	}
	if config.EntryScoreThreshold != 55.0 {
		t.Errorf("Expected default EntryScoreThreshold 55.0, got %f", config.EntryScoreThreshold)
	}

	// Test custom config
	customConfig := &BreakoutConfig{
		ATRExpansionMultiplier: 1.5,
		VolumeSpikeMultiplier:  2.5,
		RSIMomentumBullish:     60.0,
		RSIMomentumBearish:     40.0,
		VolumeDeclineRatio:     0.6,
		FailedBreakoutPct:      0.3,
		ATRExpansionWeight:     15.0,
		VolumeSpikeWeight:      10.0,
		KeyLevelBreakWeight:    10.0,
		MomentumConfirmWeight:  5.0,
		RegimeMatchWeight:      18.0,
		TimeframeAlignWeight:   12.0,
		EntryScoreThreshold:    60.0,
	}

	strategyCustom := NewBreakoutStrategyWithConfig(customConfig)
	configCustom := strategyCustom.GetConfig()

	if configCustom.ATRExpansionMultiplier != 1.5 {
		t.Errorf("Expected custom ATRExpansionMultiplier 1.5, got %f", configCustom.ATRExpansionMultiplier)
	}

	// Test nil config falls back to default
	strategyNil := NewBreakoutStrategyWithConfig(nil)
	configNil := strategyNil.GetConfig()
	if configNil.ATRExpansionMultiplier != 1.2 {
		t.Errorf("Expected default ATRExpansionMultiplier 1.2 for nil config, got %f", configNil.ATRExpansionMultiplier)
	}
}

// TestBreakoutStrategy_SetConfig tests setting configuration.
func TestBreakoutStrategy_SetConfig(t *testing.T) {
	strategy := NewBreakoutStrategy()

	newConfig := &BreakoutConfig{
		ATRExpansionMultiplier: 1.4,
		VolumeSpikeMultiplier:  2.2,
	}

	strategy.SetConfig(newConfig)
	config := strategy.GetConfig()

	if config.ATRExpansionMultiplier != 1.4 {
		t.Errorf("Expected updated ATRExpansionMultiplier 1.4, got %f", config.ATRExpansionMultiplier)
	}

	// Setting nil should not change config
	currentATRMultiplier := strategy.GetConfig().ATRExpansionMultiplier
	strategy.SetConfig(nil)
	if strategy.GetConfig().ATRExpansionMultiplier != currentATRMultiplier {
		t.Error("SetConfig(nil) should not change config")
	}
}

// TestBreakoutStrategy_ScoreATRExpansion tests ATR expansion scoring.
func TestBreakoutStrategy_ScoreATRExpansion(t *testing.T) {
	strategy := NewBreakoutStrategy()

	testCases := []struct {
		atr         float64
		atrAverage  float64
		minExpected float64
		maxExpected float64
		description string
	}{
		{0, 0, 0, 0, "Zero values"},
		{100, 0, 0, 6, "Zero average"},
		{200, 300, 0, 3, "Below average"},
		{300, 300, 2, 5, "At average"},
		{360, 300, 5, 8, "1.2x expansion"},
		{450, 300, 7, 10, "1.5x expansion"},
		{600, 300, 9, 12, "2x expansion"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			state := &decision.CoinState{
				Symbol:     "TEST",
				ATR:        tc.atr,
				ATRAverage: tc.atrAverage,
			}
			score := strategy.CalculateScore(state)
			atrScore, ok := score.ComponentDetails["atr_expansion_score"]
			if !ok && tc.atr > 0 {
				t.Errorf("ATR %.1f/%.1f: Missing atr_expansion_score in details", tc.atr, tc.atrAverage)
				return
			}
			if atrScore < tc.minExpected || atrScore > tc.maxExpected {
				t.Errorf("ATR %.1f/%.1f: score %.2f not in expected range [%.2f, %.2f]",
					tc.atr, tc.atrAverage, atrScore, tc.minExpected, tc.maxExpected)
			}
		})
	}
}

// TestBreakoutStrategy_ImplementsInterface verifies interface compliance.
func TestBreakoutStrategy_ImplementsInterface(t *testing.T) {
	var _ decision.Strategy = (*BreakoutStrategy)(nil)
	var _ decision.StrategyDescriber = (*BreakoutStrategy)(nil)
}

// TestBreakoutStrategy_TimestampSet tests that timestamp is set.
func TestBreakoutStrategy_TimestampSet(t *testing.T) {
	strategy := NewBreakoutStrategy()

	before := time.Now().UnixMilli()
	score := strategy.CalculateScore(&decision.CoinState{Symbol: "TEST"})
	after := time.Now().UnixMilli()

	if score.Timestamp < before || score.Timestamp > after {
		t.Errorf("Timestamp %d not within expected range [%d, %d]",
			score.Timestamp, before, after)
	}
}

// TestBreakoutStrategy_ComponentDetails tests component details are populated.
func TestBreakoutStrategy_ComponentDetails(t *testing.T) {
	strategy := NewBreakoutStrategy()

	state := &decision.CoinState{
		Symbol:     "BTCUSDT",
		Price:      51000,
		Regime:     decision.RegimeVolatile,
		ATR:        400,
		ATRAverage: 300,
		RSI:        58.0,
		EMA21:      50000,
		Trend1H:    decision.TrendBullish,
		Trend15M:   decision.TrendBullish,
	}

	score := strategy.CalculateScore(state)

	expectedDetails := []string{
		"atr_expansion_score",
		"volume_spike_score",
		"key_level_break_score",
		"momentum_confirm_score",
		"regime_match_score",
		"timeframe_align_score",
	}

	for _, detail := range expectedDetails {
		if _, ok := score.ComponentDetails[detail]; !ok {
			t.Errorf("Missing component detail: %s", detail)
		}
	}
}

// TestBreakoutStrategy_EntryConditionsMet tests entry conditions counting.
func TestBreakoutStrategy_EntryConditionsMet(t *testing.T) {
	strategy := NewBreakoutStrategy()

	// Perfect entry setup
	perfectState := &decision.CoinState{
		Symbol:     "BTCUSDT",
		Price:      52000,
		Regime:     decision.RegimeVolatile,
		ATR:        400,         // > average * 1.2
		ATRAverage: 300,
		RSI:        60.0,        // > 55 for bullish
		EMA21:      50000,
		Trend1H:    decision.TrendBullish,
		Trend15M:   decision.TrendBullish,
	}

	score := strategy.CalculateScore(perfectState)

	if score.EntryConditionsTotal == 0 {
		t.Error("EntryConditionsTotal should be > 0")
	}
	if score.EntryConditionsMet > score.EntryConditionsTotal {
		t.Error("EntryConditionsMet should not exceed EntryConditionsTotal")
	}
}

// TestBreakoutStrategy_ThreadSafety tests concurrent access.
func TestBreakoutStrategy_ThreadSafety(t *testing.T) {
	strategy := NewBreakoutStrategy()
	state := &decision.CoinState{
		Symbol:     "BTCUSDT",
		Price:      51000,
		Regime:     decision.RegimeVolatile,
		ATR:        400,
		ATRAverage: 300,
		RSI:        58.0,
		EMA21:      50000,
		Trend1H:    decision.TrendBullish,
		Trend15M:   decision.TrendBullish,
	}

	var wg sync.WaitGroup
	concurrency := 100
	errChan := make(chan error, concurrency*2)

	// Launch concurrent score calculations
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			score := strategy.CalculateScore(state)
			if score.StrategyName != "breakout" {
				errChan <- fmt.Errorf("unexpected strategy name: %s", score.StrategyName)
			}
		}()
	}

	// Launch concurrent config reads/writes
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				_ = strategy.GetConfig()
			} else {
				strategy.SetConfig(&BreakoutConfig{
					ATRExpansionMultiplier: 1.3,
				})
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			t.Errorf("Concurrent access error: %v", err)
		}
	}
}

// TestBreakoutStrategy_ScoreMomentumConfirmation tests momentum scoring.
func TestBreakoutStrategy_ScoreMomentumConfirmation(t *testing.T) {
	strategy := NewBreakoutStrategy()

	testCases := []struct {
		name        string
		rsi         float64
		trend       decision.TrendDirection
		minExpected float64
		maxExpected float64
	}{
		{"Bullish with high RSI", 65.0, decision.TrendBullish, 4, 6},
		{"Bullish with borderline RSI", 55.0, decision.TrendBullish, 3, 6},
		{"Bullish with weak RSI", 45.0, decision.TrendBullish, 0, 3},
		{"Bearish with low RSI", 35.0, decision.TrendBearish, 4, 6},
		{"Bearish with borderline RSI", 45.0, decision.TrendBearish, 3, 6},
		{"Bearish with weak RSI", 55.0, decision.TrendBearish, 0, 3},
		{"Neutral with RSI 50", 50.0, decision.TrendNeutral, 1, 4},
		{"Invalid RSI zero", 0, decision.TrendBullish, 0, 0},
		{"Invalid RSI negative", -10.0, decision.TrendBullish, 0, 0},
		{"Invalid RSI above 100", 105.0, decision.TrendBearish, 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := &decision.CoinState{
				Symbol:   "TEST",
				RSI:      tc.rsi,
				Trend15M: tc.trend,
			}
			score := strategy.CalculateScore(state)
			momentumScore, ok := score.ComponentDetails["momentum_confirm_score"]
			if !ok && tc.rsi > 0 {
				t.Errorf("%s: Missing momentum_confirm_score in details", tc.name)
				return
			}
			if momentumScore < tc.minExpected || momentumScore > tc.maxExpected {
				t.Errorf("%s: score %.2f not in expected range [%.2f, %.2f]",
					tc.name, momentumScore, tc.minExpected, tc.maxExpected)
			}
		})
	}
}

// TestBreakoutStrategy_VolatileRegimePreference tests regime preference.
func TestBreakoutStrategy_VolatileRegimePreference(t *testing.T) {
	strategy := NewBreakoutStrategy()

	regimes := []decision.MarketRegime{
		decision.RegimeVolatile,
		decision.RegimeConsolidating,
		decision.RegimeTrending,
		decision.RegimeRanging,
	}

	var volatileScore float64

	// Scores should decrease from volatile to ranging
	for _, regime := range regimes {
		state := &decision.CoinState{
			Symbol:     "BTCUSDT",
			Price:      51000,
			Regime:     regime,
			ATR:        400,
			ATRAverage: 300,
			RSI:        58.0,
			EMA21:      50000,
			Trend1H:    decision.TrendBullish,
			Trend15M:   decision.TrendBullish,
		}

		score := strategy.CalculateScore(state)
		regimeScore, _ := score.ComponentDetails["regime_match_score"]

		if regime == decision.RegimeVolatile {
			// Volatile should have highest regime score
			if regimeScore < 14 {
				t.Errorf("VOLATILE regime should have score >= 14, got %.2f", regimeScore)
			}
			volatileScore = regimeScore
		} else if regime == decision.RegimeRanging {
			// Ranging should have lowest regime score
			if regimeScore > 5 {
				t.Errorf("RANGING regime should have score <= 5, got %.2f", regimeScore)
			}
			// Ensure volatile is higher than ranging
			if volatileScore <= regimeScore {
				t.Errorf("VOLATILE score (%.2f) should be higher than RANGING score (%.2f)", volatileScore, regimeScore)
			}
		}
	}
}

// BenchmarkBreakoutStrategy_CalculateScore benchmarks score calculation.
func BenchmarkBreakoutStrategy_CalculateScore(b *testing.B) {
	strategy := NewBreakoutStrategy()
	state := &decision.CoinState{
		Symbol:     "BTCUSDT",
		Price:      51000,
		Regime:     decision.RegimeVolatile,
		ATR:        400,
		ATRAverage: 300,
		RSI:        58.0,
		EMA21:      50000,
		Trend1H:    decision.TrendBullish,
		Trend15M:   decision.TrendBullish,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.CalculateScore(state)
	}
}
