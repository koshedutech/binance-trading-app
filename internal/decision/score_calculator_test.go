// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.15: Additive Score Calculator Tests
package decision

import (
	"math"
	"testing"
)

// TestCalculateTechnicalScore tests the technical score calculation
func TestCalculateTechnicalScore(t *testing.T) {
	calc := NewScoreCalculator(nil, nil)

	testCases := []struct {
		name        string
		state       *CoinState
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "nil state returns zero",
			state:       nil,
			minExpected: 0,
			maxExpected: 0,
		},
		{
			name: "perfect alignment bullish with optimal RSI and ADX",
			state: &CoinState{
				Symbol:   "BTCUSDT",
				Trend1H:  TrendBullish,
				Trend15M: TrendBullish,
				RSI:      50, // Optimal for entry
				ADX:      45, // Strong trend
			},
			minExpected: 35, // Should be close to max
			maxExpected: 40,
		},
		{
			name: "conflicting trends",
			state: &CoinState{
				Symbol:   "ETHUSDT",
				Trend1H:  TrendBullish,
				Trend15M: TrendBearish, // Conflict
				RSI:      50,
				ADX:      30,
			},
			minExpected: 15, // Trend alignment = 0, but RSI and ADX contribute
			maxExpected: 25,
		},
		{
			name: "neutral trends partial alignment",
			state: &CoinState{
				Symbol:   "BNBUSDT",
				Trend1H:  TrendNeutral,
				Trend15M: TrendNeutral,
				RSI:      45,
				ADX:      25,
			},
			minExpected: 18, // 7 (partial) + ~10 (RSI) + ~7 (ADX) + 2.5 (volume)
			maxExpected: 30,
		},
		{
			name: "extreme RSI oversold",
			state: &CoinState{
				Symbol:   "SOLUSDT",
				Trend1H:  TrendBearish,
				Trend15M: TrendBearish,
				RSI:      15, // Extreme oversold
				ADX:      50,
			},
			minExpected: 25, // 15 (aligned) + 2 (extreme RSI) + 10 (strong ADX)
			maxExpected: 35,
		},
		{
			name: "no indicator data",
			state: &CoinState{
				Symbol: "XRPUSDT",
			},
			minExpected: 0,
			maxExpected: 10, // Only placeholder volume score
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calc.CalculateTechnicalScore(tc.state)

			if result.Score < tc.minExpected || result.Score > tc.maxExpected {
				t.Errorf("expected score between %.1f and %.1f, got %.1f",
					tc.minExpected, tc.maxExpected, result.Score)
			}

			if result.Dimension != DimensionTechnical {
				t.Errorf("expected dimension %s, got %s", DimensionTechnical, result.Dimension)
			}

			if result.MaxScore != MaxTechnicalScore {
				t.Errorf("expected max score %.1f, got %.1f", MaxTechnicalScore, result.MaxScore)
			}

			// Check sub-components exist
			if tc.state != nil {
				if _, ok := result.Details["trend_alignment"]; !ok {
					t.Error("missing trend_alignment in details")
				}
				if _, ok := result.Details["momentum"]; !ok {
					t.Error("missing momentum in details")
				}
				if _, ok := result.Details["volatility"]; !ok {
					t.Error("missing volatility in details")
				}
				if _, ok := result.Details["volume"]; !ok {
					t.Error("missing volume in details")
				}
			}
		})
	}
}

// TestCalculateContextScore tests the context score calculation
func TestCalculateContextScore(t *testing.T) {
	calc := NewScoreCalculator(nil, nil)

	testCases := []struct {
		name        string
		state       *CoinState
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "nil state returns zero",
			state:       nil,
			minExpected: 0,
			maxExpected: 0,
		},
		{
			name: "trending regime with trend strategy",
			state: &CoinState{
				Symbol:         "BTCUSDT",
				Regime:         RegimeTrending,
				ActiveStrategy: "trend-following",
				Trend1H:        TrendBullish,
				Trend15M:       TrendBullish,
			},
			minExpected: 25, // 10 (regime) + 10 (timeframe) + 10 (BTC is itself)
			maxExpected: 30,
		},
		{
			name: "ranging regime with range strategy",
			state: &CoinState{
				Symbol:         "ETHUSDT",
				Regime:         RegimeRanging,
				ActiveStrategy: "mean-reversion-scalp",
				Trend1H:        TrendNeutral,
				Trend15M:       TrendNeutral,
			},
			minExpected: 15, // 10 (regime) + 3 (both neutral) + 5 (BTC placeholder)
			maxExpected: 25,
		},
		{
			name: "mismatched regime and strategy",
			state: &CoinState{
				Symbol:         "BNBUSDT",
				Regime:         RegimeTrending,
				ActiveStrategy: "range-bound",
				Trend1H:        TrendBullish,
				Trend15M:       TrendNeutral,
			},
			minExpected: 8, // 3 (poor match) + 5 (partial align) + 5 (BTC placeholder)
			maxExpected: 18,
		},
		{
			name: "volatile regime",
			state: &CoinState{
				Symbol:         "DOGEUSDT",
				Regime:         RegimeVolatile,
				ActiveStrategy: "aggressive",
				Trend1H:        TrendBearish,
				Trend15M:       TrendBearish,
			},
			minExpected: 12, // 2 (poor volatile match) + 10 (aligned) + 5 (BTC)
			maxExpected: 20,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calc.CalculateContextScore(tc.state)

			if result.Score < tc.minExpected || result.Score > tc.maxExpected {
				t.Errorf("expected score between %.1f and %.1f, got %.1f",
					tc.minExpected, tc.maxExpected, result.Score)
			}

			if result.Dimension != DimensionContext {
				t.Errorf("expected dimension %s, got %s", DimensionContext, result.Dimension)
			}

			// Check sub-components exist
			if tc.state != nil {
				if _, ok := result.Details["regime_match"]; !ok {
					t.Error("missing regime_match in details")
				}
				if _, ok := result.Details["timeframe_alignment"]; !ok {
					t.Error("missing timeframe_alignment in details")
				}
				if _, ok := result.Details["btc_trend"]; !ok {
					t.Error("missing btc_trend in details")
				}
			}
		})
	}
}

// TestCalculateLLMScore tests the LLM confidence score mapping
func TestCalculateLLMScore(t *testing.T) {
	calc := NewScoreCalculator(nil, nil)

	testCases := []struct {
		name          string
		llmConfidence float64
		expected      float64
	}{
		{"0% confidence", 0, 0},
		{"50% confidence", 50, 10},
		{"100% confidence", 100, 20},
		{"75% confidence", 75, 15},
		{"negative confidence clamped", -10, 0},
		{"over 100% clamped", 150, 20},
		{"25% confidence", 25, 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calc.CalculateLLMScore(tc.llmConfidence)

			if math.Abs(result.Score-tc.expected) > 0.001 {
				t.Errorf("expected score %.1f, got %.1f", tc.expected, result.Score)
			}

			if result.Dimension != DimensionLLM {
				t.Errorf("expected dimension %s, got %s", DimensionLLM, result.Dimension)
			}

			if result.MaxScore != MaxLLMScore {
				t.Errorf("expected max score %.1f, got %.1f", MaxLLMScore, result.MaxScore)
			}
		})
	}
}

// TestCalculateHistoryScore tests the historical performance score
func TestCalculateHistoryScore(t *testing.T) {
	calc := NewScoreCalculator(nil, nil)

	testCases := []struct {
		name            string
		symbolWinRate   float64
		strategyWinRate float64
		expected        float64
	}{
		{"both 0%", 0, 0, 0},
		{"both 100%", 100, 100, 10},
		{"both 50%", 50, 50, 5},
		{"symbol 60%, strategy 80%", 60, 80, 7},
		{"negative rates clamped", -10, -20, 0},
		{"over 100% clamped", 150, 200, 10},
		{"mixed rates", 40, 70, 5.5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calc.CalculateHistoryScore(tc.symbolWinRate, tc.strategyWinRate)

			if math.Abs(result.Score-tc.expected) > 0.001 {
				t.Errorf("expected score %.1f, got %.1f", tc.expected, result.Score)
			}

			if result.Dimension != DimensionHistory {
				t.Errorf("expected dimension %s, got %s", DimensionHistory, result.Dimension)
			}

			if result.MaxScore != MaxHistoryScore {
				t.Errorf("expected max score %.1f, got %.1f", MaxHistoryScore, result.MaxScore)
			}
		})
	}
}

// TestCalculateFinalScore tests the complete score calculation
func TestCalculateFinalScore(t *testing.T) {
	calc := NewScoreCalculator(nil, nil)

	testCases := []struct {
		name            string
		state           *CoinState
		llmConfidence   float64
		symbolWinRate   float64
		strategyWinRate float64
		minExpected     float64
		maxExpected     float64
		shouldPass      bool // Should pass threshold (55)
	}{
		{
			name:        "nil state returns nil",
			state:       nil,
			minExpected: 0,
			maxExpected: 0,
			shouldPass:  false,
		},
		{
			name: "optimal conditions",
			state: &CoinState{
				Symbol:         "BTCUSDT",
				Trend1H:        TrendBullish,
				Trend15M:       TrendBullish,
				RSI:            50,
				ADX:            45,
				Regime:         RegimeTrending,
				ActiveStrategy: "trend-momentum",
			},
			llmConfidence:   90,
			symbolWinRate:   70,
			strategyWinRate: 80,
			minExpected:     75,
			maxExpected:     100,
			shouldPass:      true,
		},
		{
			name: "poor conditions",
			state: &CoinState{
				Symbol:         "SHITCOIN",
				Trend1H:        TrendBullish,
				Trend15M:       TrendBearish, // Conflict
				RSI:            85,           // Overbought
				ADX:            15,           // No trend
				Regime:         RegimeVolatile,
				ActiveStrategy: "aggressive",
			},
			llmConfidence:   20,
			symbolWinRate:   30,
			strategyWinRate: 25,
			minExpected:     15,
			maxExpected:     40,
			shouldPass:      false,
		},
		{
			name: "mediocre conditions",
			state: &CoinState{
				Symbol:         "ETHUSDT",
				Trend1H:        TrendNeutral,
				Trend15M:       TrendBullish,
				RSI:            55,
				ADX:            28,
				Regime:         RegimeConsolidating,
				ActiveStrategy: "swing",
			},
			llmConfidence:   50,
			symbolWinRate:   55,
			strategyWinRate: 50,
			minExpected:     45,
			maxExpected:     65,
			shouldPass:      false, // Borderline - actually just below threshold
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calc.CalculateFinalScore(tc.state, tc.llmConfidence, tc.symbolWinRate, tc.strategyWinRate)

			if tc.state == nil {
				if result != nil {
					t.Error("expected nil result for nil state")
				}
				return
			}

			if result == nil {
				t.Fatal("unexpected nil result")
			}

			if result.NormalizedScore < tc.minExpected || result.NormalizedScore > tc.maxExpected {
				t.Errorf("expected normalized score between %.1f and %.1f, got %.1f",
					tc.minExpected, tc.maxExpected, result.NormalizedScore)
			}

			if result.PassesThreshold != tc.shouldPass {
				t.Errorf("expected PassesThreshold=%v, got %v (score: %.1f, threshold: %.1f)",
					tc.shouldPass, result.PassesThreshold, result.NormalizedScore, result.Threshold)
			}

			// Verify all 4 components present
			if len(result.Components) != 4 {
				t.Errorf("expected 4 components, got %d", len(result.Components))
			}

			// Verify symbol is set
			if result.Symbol != tc.state.Symbol {
				t.Errorf("expected symbol %s, got %s", tc.state.Symbol, result.Symbol)
			}

			// Verify timestamp is recent
			if result.Timestamp == 0 {
				t.Error("timestamp should not be zero")
			}
		})
	}
}

// TestConfigurableWeights tests that custom weights affect scoring
func TestConfigurableWeights(t *testing.T) {
	state := &CoinState{
		Symbol:         "BTCUSDT",
		Trend1H:        TrendBullish,
		Trend15M:       TrendBullish,
		RSI:            50,
		ADX:            40,
		Regime:         RegimeTrending,
		ActiveStrategy: "trend",
	}

	// Test with default weights
	defaultCalc := NewScoreCalculator(nil, nil)
	defaultResult := defaultCalc.CalculateFinalScore(state, 80, 60, 60)

	// Test with custom weights - emphasize technical
	customConfig := &ScoringConfig{
		TechnicalWeight: 2.0, // Double technical
		ContextWeight:   1.0,
		LLMWeight:       0.5, // Half LLM
		HistoryWeight:   0.5, // Half history
		MinThreshold:    55,
	}
	customCalc := NewScoreCalculator(nil, customConfig)
	customResult := customCalc.CalculateFinalScore(state, 80, 60, 60)

	// With higher technical weight and good technical signals,
	// the weighted score should be different
	if defaultResult.FinalScore == customResult.FinalScore {
		t.Error("custom weights should produce different final score")
	}

	// Technical component should have higher weighted score
	defaultTech := defaultResult.GetComponent(DimensionTechnical)
	customTech := customResult.GetComponent(DimensionTechnical)

	if customTech.WeightedScore <= defaultTech.WeightedScore {
		t.Error("custom technical weight should produce higher weighted score")
	}

	// LLM component should have lower weighted score
	defaultLLM := defaultResult.GetComponent(DimensionLLM)
	customLLM := customResult.GetComponent(DimensionLLM)

	if customLLM.WeightedScore >= defaultLLM.WeightedScore {
		t.Error("custom LLM weight should produce lower weighted score")
	}
}

// TestThreshold tests score vs threshold comparison
func TestThreshold(t *testing.T) {
	testCases := []struct {
		name       string
		threshold  float64
		score      float64
		shouldPass bool
	}{
		{"score equals threshold", 55, 55, true},
		{"score above threshold", 55, 60, true},
		{"score below threshold", 55, 50, false},
		{"zero threshold always passes", 0, 0.001, true},
		{"max threshold", 100, 99, false},
		{"max threshold exact", 100, 100, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &ScoringConfig{
				TechnicalWeight: 1.0,
				ContextWeight:   1.0,
				LLMWeight:       1.0,
				HistoryWeight:   1.0,
				MinThreshold:    tc.threshold,
			}
			calc := NewScoreCalculator(nil, config)

			result := calc.PassesThreshold(tc.score)
			if result != tc.shouldPass {
				t.Errorf("expected PassesThreshold(%v)=%v, got %v",
					tc.score, tc.shouldPass, result)
			}
		})
	}
}

// TestScoreHistory tests the score history tracking
func TestScoreHistory(t *testing.T) {
	calc := NewScoreCalculator(nil, nil)
	symbol := "BTCUSDT"

	state := &CoinState{
		Symbol:         symbol,
		Trend1H:        TrendBullish,
		Trend15M:       TrendBullish,
		RSI:            50,
		ADX:            30,
		Regime:         RegimeTrending,
		ActiveStrategy: "trend",
	}

	// Add multiple scores
	for i := 0; i < 5; i++ {
		calc.CalculateFinalScore(state, float64(50+i*10), 60, 60)
	}

	// Check history
	history := calc.GetHistory(symbol)
	if history == nil {
		t.Fatal("expected history to exist")
	}

	if history.Count() != 5 {
		t.Errorf("expected 5 entries, got %d", history.Count())
	}

	// Average should be reasonable
	avg := calc.GetAverageScore(symbol)
	if avg < 50 || avg > 100 {
		t.Errorf("average score %.1f out of expected range", avg)
	}

	// Clear history
	calc.ClearHistory(symbol)
	history = calc.GetHistory(symbol)
	if history != nil && history.Count() != 0 {
		t.Error("history should be cleared")
	}
}

// TestScoreBreakdownMethods tests ScoreBreakdown helper methods
func TestScoreBreakdownMethods(t *testing.T) {
	breakdown := NewScoreBreakdown("BTCUSDT")

	// Add components
	techScore := NewComponentScore(DimensionTechnical, 40)
	techScore.SetScore(30)
	breakdown.AddComponent(*techScore)

	ctxScore := NewComponentScore(DimensionContext, 30)
	ctxScore.SetScore(20)
	breakdown.AddComponent(*ctxScore)

	// Test GetComponent
	tech := breakdown.GetComponent(DimensionTechnical)
	if tech == nil {
		t.Fatal("GetComponent returned nil for existing dimension")
	}
	if tech.Score != 30 {
		t.Errorf("expected score 30, got %.1f", tech.Score)
	}

	// Test GetComponent for non-existent
	llm := breakdown.GetComponent(DimensionLLM)
	if llm != nil {
		t.Error("GetComponent should return nil for non-existent dimension")
	}

	// Test CalculateFinal
	breakdown.CalculateFinal(55)
	if breakdown.FinalScore != 50 { // 30 + 20
		t.Errorf("expected final score 50, got %.1f", breakdown.FinalScore)
	}
}

// TestComponentScoreMethods tests ComponentScore helper methods
func TestComponentScoreMethods(t *testing.T) {
	cs := NewComponentScore(DimensionTechnical, 40)

	// Test SetScore clamping
	cs.SetScore(-10)
	if cs.Score != 0 {
		t.Errorf("score should clamp to 0, got %.1f", cs.Score)
	}

	cs.SetScore(100)
	if cs.Score != 40 {
		t.Errorf("score should clamp to max (40), got %.1f", cs.Score)
	}

	cs.SetScore(30)
	if cs.Score != 30 {
		t.Errorf("expected score 30, got %.1f", cs.Score)
	}

	// Test Percentage
	pct := cs.Percentage()
	if pct != 75 { // 30/40 * 100
		t.Errorf("expected percentage 75, got %.1f", pct)
	}

	// Test SetWeight
	cs.SetWeight(2.0)
	if cs.WeightedScore != 60 { // 30 * 2.0
		t.Errorf("expected weighted score 60, got %.1f", cs.WeightedScore)
	}

	cs.SetWeight(-1)
	if cs.Weight != 0 {
		t.Errorf("weight should clamp to 0, got %.1f", cs.Weight)
	}
}

// TestScoringConfigValidation tests config validation
func TestScoringConfigValidation(t *testing.T) {
	config := &ScoringConfig{
		TechnicalWeight: -1,
		ContextWeight:   -2,
		LLMWeight:       -3,
		HistoryWeight:   -4,
		MinThreshold:    150,
	}

	config.Validate()

	if config.TechnicalWeight != 0 {
		t.Errorf("negative weight should clamp to 0, got %.1f", config.TechnicalWeight)
	}
	if config.ContextWeight != 0 {
		t.Errorf("negative weight should clamp to 0, got %.1f", config.ContextWeight)
	}
	if config.MinThreshold != 100 {
		t.Errorf("threshold over 100 should clamp, got %.1f", config.MinThreshold)
	}
}

// TestScoreHistoryRollingWindow tests that history respects max size
func TestScoreHistoryRollingWindow(t *testing.T) {
	history := NewScoreHistory("BTCUSDT", 3)

	// Add 5 entries
	for i := 0; i < 5; i++ {
		breakdown := ScoreBreakdown{
			Symbol:          "BTCUSDT",
			NormalizedScore: float64(i * 10),
		}
		history.Add(breakdown)
	}

	// Should only have 3 entries
	if history.Count() != 3 {
		t.Errorf("expected 3 entries after rolling, got %d", history.Count())
	}

	// Latest should be score 40 (index 4)
	latest := history.Latest()
	if latest == nil || latest.NormalizedScore != 40 {
		t.Error("latest should be most recent entry with score 40")
	}

	// Average should be (20 + 30 + 40) / 3 = 30
	avg := history.Average()
	if avg != 30 {
		t.Errorf("expected average 30, got %.1f", avg)
	}
}

// BenchmarkScoreCalculation benchmarks the full score calculation
// Performance target: < 1ms per calculation
func BenchmarkScoreCalculation(b *testing.B) {
	calc := NewScoreCalculator(nil, nil)

	state := &CoinState{
		Symbol:         "BTCUSDT",
		Trend1H:        TrendBullish,
		Trend15M:       TrendBullish,
		RSI:            52.5,
		ADX:            35.2,
		Regime:         RegimeTrending,
		ActiveStrategy: "momentum-trend-v2",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateFinalScore(state, 75.5, 62.3, 58.7)
	}
}

// BenchmarkTechnicalScore benchmarks technical score calculation
func BenchmarkTechnicalScore(b *testing.B) {
	calc := NewScoreCalculator(nil, nil)

	state := &CoinState{
		Symbol:   "BTCUSDT",
		Trend1H:  TrendBullish,
		Trend15M: TrendBullish,
		RSI:      52.5,
		ADX:      35.2,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateTechnicalScore(state)
	}
}

// BenchmarkContextScore benchmarks context score calculation
func BenchmarkContextScore(b *testing.B) {
	calc := NewScoreCalculator(nil, nil)

	state := &CoinState{
		Symbol:         "BTCUSDT",
		Trend1H:        TrendBullish,
		Trend15M:       TrendBullish,
		Regime:         RegimeTrending,
		ActiveStrategy: "momentum-trend-v2",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateContextScore(state)
	}
}

// BenchmarkLLMScore benchmarks LLM score calculation
func BenchmarkLLMScore(b *testing.B) {
	calc := NewScoreCalculator(nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateLLMScore(75.5)
	}
}

// BenchmarkHistoryScore benchmarks history score calculation
func BenchmarkHistoryScore(b *testing.B) {
	calc := NewScoreCalculator(nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateHistoryScore(62.3, 58.7)
	}
}

// TestRegimeMatchingScenarios tests various regime-strategy combinations
func TestRegimeMatchingScenarios(t *testing.T) {
	calc := NewScoreCalculator(nil, nil)

	scenarios := []struct {
		name            string
		regime          MarketRegime
		strategy        string
		expectedMin     float64
		expectedMax     float64
	}{
		{"trending-trend", RegimeTrending, "trend-following", 10, 10},
		{"trending-momentum", RegimeTrending, "momentum-breakout", 10, 10},
		{"trending-range", RegimeTrending, "range-bound", 0, 5},
		{"ranging-reversion", RegimeRanging, "mean-reversion", 10, 10},
		{"ranging-scalp", RegimeRanging, "scalping-strategy", 10, 10},
		{"ranging-trend", RegimeRanging, "trend-following", 0, 5},
		{"volatile-hedge", RegimeVolatile, "hedge-conservative", 10, 10},
		{"volatile-aggressive", RegimeVolatile, "aggressive-long", 0, 5},
		{"consolidating-range", RegimeConsolidating, "range-accumulation", 7, 10},
		{"consolidating-breakout", RegimeConsolidating, "breakout-watch", 4, 6},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			state := &CoinState{
				Symbol:         "TESTUSDT",
				Regime:         s.regime,
				ActiveStrategy: s.strategy,
			}
			result := calc.CalculateContextScore(state)
			regimeScore := result.Details["regime_match"]

			if regimeScore < s.expectedMin || regimeScore > s.expectedMax {
				t.Errorf("%s: expected regime score %.1f-%.1f, got %.1f",
					s.name, s.expectedMin, s.expectedMax, regimeScore)
			}
		})
	}
}

// TestRSIScoringZones tests RSI scoring at zone boundaries
func TestRSIScoringZones(t *testing.T) {
	calc := NewScoreCalculator(nil, nil)

	zones := []struct {
		rsi           float64
		expectedScore float64
	}{
		{0, 0},      // No data
		{15, 2},     // Extreme oversold
		{25, 4},     // Extended zone
		{35, 7},     // Moderate zone
		{45, 10},    // Optimal zone
		{55, 10},    // Optimal zone
		{65, 7},     // Moderate zone
		{75, 4},     // Extended zone
		{85, 2},     // Extreme overbought
	}

	for _, z := range zones {
		t.Run("RSI_"+string(rune(int(z.rsi))), func(t *testing.T) {
			state := &CoinState{
				Symbol: "TESTUSDT",
				RSI:    z.rsi,
			}
			result := calc.CalculateTechnicalScore(state)
			momentumScore := result.Details["momentum"]

			if momentumScore != z.expectedScore {
				t.Errorf("RSI %.1f: expected momentum score %.1f, got %.1f",
					z.rsi, z.expectedScore, momentumScore)
			}
		})
	}
}

// TestADXScoringZones tests ADX scoring at zone boundaries
func TestADXScoringZones(t *testing.T) {
	calc := NewScoreCalculator(nil, nil)

	zones := []struct {
		adx           float64
		expectedScore float64
	}{
		{0, 0},   // No data
		{10, 2},  // No trend
		{22, 4},  // Weak trend
		{30, 7},  // Moderate trend
		{45, 10}, // Strong trend
	}

	for _, z := range zones {
		t.Run("ADX_"+string(rune(int(z.adx))), func(t *testing.T) {
			state := &CoinState{
				Symbol: "TESTUSDT",
				ADX:    z.adx,
			}
			result := calc.CalculateTechnicalScore(state)
			volatilityScore := result.Details["volatility"]

			if volatilityScore != z.expectedScore {
				t.Errorf("ADX %.1f: expected volatility score %.1f, got %.1f",
					z.adx, z.expectedScore, volatilityScore)
			}
		})
	}
}
