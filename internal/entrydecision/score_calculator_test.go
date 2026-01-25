// Package entrydecision provides tests for the Score Calculator.
// Epic 14: Chain Trading System - Story 14.10: Score Calculator for Trend Following
package entrydecision

import (
	"testing"
	"time"

	"binance-trading-bot/internal/coinprofiler"
)

// ============================================================================
// Score Weights Tests
// ============================================================================

func TestDefaultScoreWeights(t *testing.T) {
	weights := DefaultScoreWeights()

	if weights.TrendStrength != DefaultWeightTrendStrength {
		t.Errorf("TrendStrength = %d, want %d", weights.TrendStrength, DefaultWeightTrendStrength)
	}
	if weights.Momentum != DefaultWeightMomentum {
		t.Errorf("Momentum = %d, want %d", weights.Momentum, DefaultWeightMomentum)
	}
	if weights.Volume != DefaultWeightVolume {
		t.Errorf("Volume = %d, want %d", weights.Volume, DefaultWeightVolume)
	}
	if weights.PriceAction != DefaultWeightPriceAction {
		t.Errorf("PriceAction = %d, want %d", weights.PriceAction, DefaultWeightPriceAction)
	}
	if weights.SRProximity != DefaultWeightSRProximity {
		t.Errorf("SRProximity = %d, want %d", weights.SRProximity, DefaultWeightSRProximity)
	}
}

func TestScoreWeights_Sum(t *testing.T) {
	tests := []struct {
		name     string
		weights  *ScoreWeights
		expected int
	}{
		{
			name:     "default weights sum to 100",
			weights:  DefaultScoreWeights(),
			expected: 100,
		},
		{
			name: "custom weights",
			weights: &ScoreWeights{
				TrendStrength: 40,
				Momentum:      30,
				Volume:        15,
				PriceAction:   10,
				SRProximity:   5,
			},
			expected: 100,
		},
		{
			name:     "nil weights",
			weights:  nil,
			expected: 0,
		},
		{
			name: "zero weights",
			weights: &ScoreWeights{
				TrendStrength: 0,
				Momentum:      0,
				Volume:        0,
				PriceAction:   0,
				SRProximity:   0,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.weights.Sum(); got != tt.expected {
				t.Errorf("Sum() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestScoreWeights_Validate(t *testing.T) {
	tests := []struct {
		name      string
		weights   *ScoreWeights
		expectErr bool
	}{
		{
			name:      "valid default weights",
			weights:   DefaultScoreWeights(),
			expectErr: false,
		},
		{
			name:      "nil weights",
			weights:   nil,
			expectErr: true,
		},
		{
			name: "negative weight",
			weights: &ScoreWeights{
				TrendStrength: -10,
				Momentum:      50,
				Volume:        30,
				PriceAction:   20,
				SRProximity:   10,
			},
			expectErr: true,
		},
		{
			name: "sum not 100",
			weights: &ScoreWeights{
				TrendStrength: 20,
				Momentum:      20,
				Volume:        20,
				PriceAction:   20,
				SRProximity:   10, // Sum = 90
			},
			expectErr: true,
		},
		{
			name: "valid custom weights",
			weights: &ScoreWeights{
				TrendStrength: 50,
				Momentum:      20,
				Volume:        15,
				PriceAction:   10,
				SRProximity:   5,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.weights.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Validate() error = %v, expectErr = %v", err, tt.expectErr)
			}
		})
	}
}

// ============================================================================
// Score Calculator Config Tests
// ============================================================================

func TestDefaultScoreCalculatorConfig(t *testing.T) {
	config := DefaultScoreCalculatorConfig()

	if config.Threshold != DefaultScoreThreshold {
		t.Errorf("Threshold = %d, want %d", config.Threshold, DefaultScoreThreshold)
	}
	if config.Weights == nil {
		t.Error("Weights should not be nil")
	}
	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

func TestScoreCalculatorConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *ScoreCalculatorConfig
		expectErr bool
	}{
		{
			name:      "valid default config",
			config:    DefaultScoreCalculatorConfig(),
			expectErr: false,
		},
		{
			name:      "nil config",
			config:    nil,
			expectErr: true,
		},
		{
			name: "threshold too low",
			config: &ScoreCalculatorConfig{
				Threshold: -1,
				Weights:   DefaultScoreWeights(),
			},
			expectErr: true,
		},
		{
			name: "threshold too high",
			config: &ScoreCalculatorConfig{
				Threshold: 101,
				Weights:   DefaultScoreWeights(),
			},
			expectErr: true,
		},
		{
			name: "nil weights",
			config: &ScoreCalculatorConfig{
				Threshold: 55,
				Weights:   nil,
			},
			expectErr: true,
		},
		{
			name: "threshold at boundaries",
			config: &ScoreCalculatorConfig{
				Threshold: 0,
				Weights:   DefaultScoreWeights(),
			},
			expectErr: false,
		},
		{
			name: "threshold at max",
			config: &ScoreCalculatorConfig{
				Threshold: 100,
				Weights:   DefaultScoreWeights(),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Validate() error = %v, expectErr = %v", err, tt.expectErr)
			}
		})
	}
}

// ============================================================================
// Score Calculator Creation Tests
// ============================================================================

func TestNewTrendFollowingScoreCalculator(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	if calc == nil {
		t.Fatal("Calculator should not be nil")
	}
	if calc.config == nil {
		t.Error("Config should not be nil")
	}
	if calc.config.Threshold != DefaultScoreThreshold {
		t.Errorf("Threshold = %d, want %d", calc.config.Threshold, DefaultScoreThreshold)
	}
	if calc.components == nil {
		t.Error("Components should not be nil")
	}
}

func TestNewTrendFollowingScoreCalculatorWithConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &ScoreCalculatorConfig{
			Threshold: 70,
			Weights:   DefaultScoreWeights(),
		}

		calc, err := NewTrendFollowingScoreCalculatorWithConfig(config)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if calc.config.Threshold != 70 {
			t.Errorf("Threshold = %d, want 70", calc.config.Threshold)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		config := &ScoreCalculatorConfig{
			Threshold: 150, // Invalid
			Weights:   DefaultScoreWeights(),
		}

		_, err := NewTrendFollowingScoreCalculatorWithConfig(config)
		if err == nil {
			t.Error("Expected error for invalid config")
		}
	})
}

func TestNewTrendFollowingScoreCalculatorWithComponents(t *testing.T) {
	t.Run("valid components", func(t *testing.T) {
		config := DefaultScoreCalculatorConfig()
		components := NewStubComponentCalculator()

		calc, err := NewTrendFollowingScoreCalculatorWithComponents(config, components)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if calc == nil {
			t.Error("Calculator should not be nil")
		}
	})

	t.Run("nil components", func(t *testing.T) {
		config := DefaultScoreCalculatorConfig()

		_, err := NewTrendFollowingScoreCalculatorWithComponents(config, nil)
		if err == nil {
			t.Error("Expected error for nil components")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		config := &ScoreCalculatorConfig{
			Threshold: -1,
			Weights:   DefaultScoreWeights(),
		}
		components := NewStubComponentCalculator()

		_, err := NewTrendFollowingScoreCalculatorWithComponents(config, components)
		if err == nil {
			t.Error("Expected error for invalid config")
		}
	})
}

// ============================================================================
// Calculate Score Tests
// ============================================================================

func TestCalculateScore_Basic(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	data := &coinprofiler.CoinData{
		Symbol:    "BTCUSDT",
		Price:     43500.00,
		Volume24h: 50000000,
		UpdatedAt: time.Now(),
	}

	result, err := calc.CalculateScore("BTCUSDT", data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %s, want BTCUSDT", result.Symbol)
	}
	if result.Score < 0 || result.Score > 100 {
		t.Errorf("Score = %d, should be between 0 and 100", result.Score)
	}
	if result.Direction == "" {
		t.Error("Direction should not be empty")
	}
	if result.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
	if len(result.Components) != 5 {
		t.Errorf("Components count = %d, want 5", len(result.Components))
	}
}

func TestCalculateScore_EmptySymbol(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	_, err := calc.CalculateScore("", nil)
	if err == nil {
		t.Error("Expected error for empty symbol")
	}
}

func TestCalculateScore_NilData(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	result, err := calc.CalculateScore("BTCUSDT", nil)
	if err != nil {
		t.Fatalf("Unexpected error for nil data: %v", err)
	}

	if result.Score != 0 {
		t.Errorf("Score = %d, want 0 for nil data", result.Score)
	}
	if result.Direction != DirectionNeutral {
		t.Errorf("Direction = %s, want neutral for nil data", result.Direction)
	}
	if result.Ready {
		t.Error("Ready should be false for nil data")
	}
}

func TestCalculateScore_WithFixedScores(t *testing.T) {
	// Create calculator with fixed scores for deterministic testing
	stubCalc := NewStubComponentCalculator().
		WithFixedScores(80, 70, 60, 50, 40).
		WithFixedDirections(DirectionLong, DirectionLong, DirectionLong)

	config := DefaultScoreCalculatorConfig()
	config.Threshold = 50

	calc, err := NewTrendFollowingScoreCalculatorWithComponents(config, stubCalc)
	if err != nil {
		t.Fatalf("Failed to create calculator: %v", err)
	}

	data := &coinprofiler.CoinData{
		Symbol: "BTCUSDT",
		Price:  43500.00,
	}

	result, err := calc.CalculateScore("BTCUSDT", data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Calculate expected weighted average:
	// (80*30 + 70*25 + 60*20 + 50*15 + 40*10) / 100
	// = (2400 + 1750 + 1200 + 750 + 400) / 100
	// = 6500 / 100 = 65
	expectedScore := 65

	if result.Score != expectedScore {
		t.Errorf("Score = %d, want %d", result.Score, expectedScore)
	}
	if result.Direction != DirectionLong {
		t.Errorf("Direction = %s, want long", result.Direction)
	}
	if !result.Ready {
		t.Error("Ready should be true (65 >= 50)")
	}

	// Verify individual components
	if result.Components[string(ScoreComponentTrendStrength)] != 80 {
		t.Errorf("TrendStrength = %d, want 80", result.Components[string(ScoreComponentTrendStrength)])
	}
	if result.Components[string(ScoreComponentMomentum)] != 70 {
		t.Errorf("Momentum = %d, want 70", result.Components[string(ScoreComponentMomentum)])
	}
	if result.Components[string(ScoreComponentVolume)] != 60 {
		t.Errorf("Volume = %d, want 60", result.Components[string(ScoreComponentVolume)])
	}
	if result.Components[string(ScoreComponentPriceAction)] != 50 {
		t.Errorf("PriceAction = %d, want 50", result.Components[string(ScoreComponentPriceAction)])
	}
	if result.Components[string(ScoreComponentSRProximity)] != 40 {
		t.Errorf("SRProximity = %d, want 40", result.Components[string(ScoreComponentSRProximity)])
	}
}

func TestCalculateScore_ThresholdComparison(t *testing.T) {
	tests := []struct {
		name         string
		scores       []int // trend, momentum, volume, priceAction, sr
		threshold    int
		expectedReady bool
	}{
		{
			name:          "all high scores above threshold",
			scores:        []int{100, 100, 100, 100, 100},
			threshold:     55,
			expectedReady: true,
		},
		{
			name:          "all low scores below threshold",
			scores:        []int{20, 20, 20, 20, 20},
			threshold:     55,
			expectedReady: false,
		},
		{
			name:          "exact threshold",
			scores:        []int{55, 55, 55, 55, 55},
			threshold:     55,
			expectedReady: true,
		},
		{
			name:          "just below threshold",
			scores:        []int{54, 54, 54, 54, 54},
			threshold:     55,
			expectedReady: false,
		},
		{
			name:          "zero threshold",
			scores:        []int{0, 0, 0, 0, 0},
			threshold:     0,
			expectedReady: true,
		},
		{
			name:          "max threshold",
			scores:        []int{99, 99, 99, 99, 99},
			threshold:     100,
			expectedReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCalc := NewStubComponentCalculator().
				WithFixedScores(tt.scores[0], tt.scores[1], tt.scores[2], tt.scores[3], tt.scores[4])

			config := &ScoreCalculatorConfig{
				Threshold: tt.threshold,
				Weights:   DefaultScoreWeights(),
			}

			calc, _ := NewTrendFollowingScoreCalculatorWithComponents(config, stubCalc)

			result, err := calc.CalculateScore("BTCUSDT", &coinprofiler.CoinData{Symbol: "BTCUSDT"})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Ready != tt.expectedReady {
				t.Errorf("Ready = %v, want %v (score: %d, threshold: %d)",
					result.Ready, tt.expectedReady, result.Score, tt.threshold)
			}
		})
	}
}

// ============================================================================
// Direction Detection Tests
// ============================================================================

func TestCalculateScore_DirectionDetection(t *testing.T) {
	tests := []struct {
		name             string
		trendDir         string
		momentumDir      string
		priceActionDir   string
		expectedDirection string
	}{
		{
			name:             "all long",
			trendDir:         DirectionLong,
			momentumDir:      DirectionLong,
			priceActionDir:   DirectionLong,
			expectedDirection: DirectionLong,
		},
		{
			name:             "all short",
			trendDir:         DirectionShort,
			momentumDir:      DirectionShort,
			priceActionDir:   DirectionShort,
			expectedDirection: DirectionShort,
		},
		{
			name:             "all neutral",
			trendDir:         DirectionNeutral,
			momentumDir:      DirectionNeutral,
			priceActionDir:   DirectionNeutral,
			expectedDirection: DirectionNeutral,
		},
		{
			name:             "trend long overrides others",
			trendDir:         DirectionLong,
			momentumDir:      DirectionShort,
			priceActionDir:   DirectionShort,
			// TrendStrength(30) + long = 30
			// Momentum(25) + PriceAction(15) = 40 short
			// 40 > 30, so short wins
			expectedDirection: DirectionShort,
		},
		{
			name:             "trend and momentum agree against price action",
			trendDir:         DirectionLong,
			momentumDir:      DirectionLong,
			priceActionDir:   DirectionShort,
			// Long: 30 + 25 = 55
			// Short: 15
			// Long wins
			expectedDirection: DirectionLong,
		},
		{
			name:             "equal long and short (neutral)",
			trendDir:         DirectionLong,
			momentumDir:      DirectionShort,
			priceActionDir:   DirectionNeutral,
			// Long: 30
			// Short: 25
			// 30 > 25, long wins
			expectedDirection: DirectionLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCalc := NewStubComponentCalculator().
				WithFixedScores(70, 70, 70, 70, 70).
				WithFixedDirections(tt.trendDir, tt.momentumDir, tt.priceActionDir)

			calc, _ := NewTrendFollowingScoreCalculatorWithComponents(
				DefaultScoreCalculatorConfig(), stubCalc)

			result, err := calc.CalculateScore("BTCUSDT", &coinprofiler.CoinData{Symbol: "BTCUSDT"})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Direction != tt.expectedDirection {
				t.Errorf("Direction = %s, want %s", result.Direction, tt.expectedDirection)
			}
		})
	}
}

// ============================================================================
// Weighted Average Calculation Tests
// ============================================================================

func TestCalculateScore_WeightedAverage(t *testing.T) {
	tests := []struct {
		name          string
		scores        []int
		weights       *ScoreWeights
		expectedScore int
	}{
		{
			name:   "default weights with uniform scores",
			scores: []int{100, 100, 100, 100, 100},
			weights: DefaultScoreWeights(),
			expectedScore: 100,
		},
		{
			name:   "default weights with zero scores",
			scores: []int{0, 0, 0, 0, 0},
			weights: DefaultScoreWeights(),
			expectedScore: 0,
		},
		{
			name:   "only trend strength counts",
			scores: []int{100, 0, 0, 0, 0},
			weights: DefaultScoreWeights(), // 30% for trend
			expectedScore: 30,
		},
		{
			name:   "custom weights - heavy on trend",
			scores: []int{100, 50, 50, 50, 50},
			weights: &ScoreWeights{
				TrendStrength: 60,
				Momentum:      10,
				Volume:        10,
				PriceAction:   10,
				SRProximity:   10,
			},
			// (100*60 + 50*10 + 50*10 + 50*10 + 50*10) / 100
			// = (6000 + 500 + 500 + 500 + 500) / 100 = 8000/100 = 80
			expectedScore: 80,
		},
		{
			name:   "varied scores with default weights",
			scores: []int{90, 80, 70, 60, 50},
			weights: DefaultScoreWeights(),
			// (90*30 + 80*25 + 70*20 + 60*15 + 50*10) / 100
			// = (2700 + 2000 + 1400 + 900 + 500) / 100 = 7500/100 = 75
			expectedScore: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCalc := NewStubComponentCalculator().
				WithFixedScores(tt.scores[0], tt.scores[1], tt.scores[2], tt.scores[3], tt.scores[4])

			config := &ScoreCalculatorConfig{
				Threshold: 0,
				Weights:   tt.weights,
			}

			calc, err := NewTrendFollowingScoreCalculatorWithComponents(config, stubCalc)
			if err != nil {
				t.Fatalf("Failed to create calculator: %v", err)
			}

			result, err := calc.CalculateScore("BTCUSDT", &coinprofiler.CoinData{Symbol: "BTCUSDT"})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Score != tt.expectedScore {
				t.Errorf("Score = %d, want %d", result.Score, tt.expectedScore)
			}
		})
	}
}

// ============================================================================
// Calculate Scores (Batch) Tests
// ============================================================================

func TestCalculateScores_Multiple(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	coins := map[string]*coinprofiler.CoinData{
		"BTCUSDT": {Symbol: "BTCUSDT", Price: 43500},
		"ETHUSDT": {Symbol: "ETHUSDT", Price: 2300},
		"SOLUSDT": {Symbol: "SOLUSDT", Price: 95},
	}

	results, err := calc.CalculateScores(coins)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Results count = %d, want 3", len(results))
	}

	// Verify all symbols are present
	symbolsFound := make(map[string]bool)
	for _, r := range results {
		symbolsFound[r.Symbol] = true
	}

	for symbol := range coins {
		if !symbolsFound[symbol] {
			t.Errorf("Symbol %s not found in results", symbol)
		}
	}
}

func TestCalculateScores_NilInput(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	results, err := calc.CalculateScores(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if results != nil {
		t.Error("Results should be nil for nil input")
	}
}

func TestCalculateScores_EmptyInput(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	coins := map[string]*coinprofiler.CoinData{}

	results, err := calc.CalculateScores(coins)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Results count = %d, want 0", len(results))
	}
}

// ============================================================================
// Configuration Tests
// ============================================================================

func TestGetConfig(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	config := calc.GetConfig()
	if config == nil {
		t.Error("Config should not be nil")
	}
	if config.Threshold != DefaultScoreThreshold {
		t.Errorf("Threshold = %d, want %d", config.Threshold, DefaultScoreThreshold)
	}
}

func TestSetConfig(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	t.Run("valid config", func(t *testing.T) {
		newConfig := &ScoreCalculatorConfig{
			Threshold: 70,
			Weights:   DefaultScoreWeights(),
		}

		err := calc.SetConfig(newConfig)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if calc.GetConfig().Threshold != 70 {
			t.Errorf("Threshold = %d, want 70", calc.GetConfig().Threshold)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		newConfig := &ScoreCalculatorConfig{
			Threshold: -1, // Invalid
			Weights:   DefaultScoreWeights(),
		}

		err := calc.SetConfig(newConfig)
		if err == nil {
			t.Error("Expected error for invalid config")
		}
	})
}

func TestSetComponentCalculator(t *testing.T) {
	calc := NewTrendFollowingScoreCalculator()

	t.Run("valid component calculator", func(t *testing.T) {
		newComponents := NewStubComponentCalculator().WithFixedScores(90, 90, 90, 90, 90)

		err := calc.SetComponentCalculator(newComponents)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Verify new calculator is used
		result, _ := calc.CalculateScore("BTCUSDT", &coinprofiler.CoinData{Symbol: "BTCUSDT"})
		if result.Score != 90 {
			t.Errorf("Score = %d, want 90 (should use new calculator)", result.Score)
		}
	})

	t.Run("nil component calculator", func(t *testing.T) {
		err := calc.SetComponentCalculator(nil)
		if err == nil {
			t.Error("Expected error for nil component calculator")
		}
	})
}

// ============================================================================
// Score Result Tests
// ============================================================================

func TestScoreResult_ToCoinMatch(t *testing.T) {
	t.Run("valid result", func(t *testing.T) {
		result := &ScoreResult{
			Symbol:    "BTCUSDT",
			Score:     82,
			Direction: DirectionLong,
			Ready:     true,
			Components: map[string]int{
				"trend_strength": 90,
				"momentum":       80,
			},
			Threshold: 55,
			UpdatedAt: time.Now(),
		}

		coinMatch := result.ToCoinMatch()
		if coinMatch == nil {
			t.Fatal("CoinMatch should not be nil")
		}

		if coinMatch.Symbol != "BTCUSDT" {
			t.Errorf("Symbol = %s, want BTCUSDT", coinMatch.Symbol)
		}
		if coinMatch.Score != 82 {
			t.Errorf("Score = %d, want 82", coinMatch.Score)
		}
		if coinMatch.Direction != DirectionLong {
			t.Errorf("Direction = %s, want long", coinMatch.Direction)
		}
		if !coinMatch.Ready {
			t.Error("Ready should be true")
		}
	})

	t.Run("nil result", func(t *testing.T) {
		var result *ScoreResult
		coinMatch := result.ToCoinMatch()
		if coinMatch != nil {
			t.Error("CoinMatch should be nil for nil result")
		}
	})
}

// ============================================================================
// Edge Cases and Error Handling
// ============================================================================

func TestCalculateScore_ScoreClamping(t *testing.T) {
	// Test that component scores are clamped to 0-100
	stubCalc := NewStubComponentCalculator().
		WithFixedScores(150, -10, 200, -50, 300) // Invalid scores

	calc, _ := NewTrendFollowingScoreCalculatorWithComponents(
		DefaultScoreCalculatorConfig(), stubCalc)

	result, err := calc.CalculateScore("BTCUSDT", &coinprofiler.CoinData{Symbol: "BTCUSDT"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// All scores should be clamped
	if result.Components[string(ScoreComponentTrendStrength)] > 100 {
		t.Error("TrendStrength should be clamped to 100")
	}
	if result.Components[string(ScoreComponentMomentum)] < 0 {
		t.Error("Momentum should be clamped to 0")
	}
	if result.Score < 0 || result.Score > 100 {
		t.Errorf("Final score should be between 0 and 100, got %d", result.Score)
	}
}

func TestClampScore(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{50, 50},
		{0, 0},
		{100, 100},
		{-10, 0},
		{150, 100},
		{-100, 0},
		{200, 100},
	}

	for _, tt := range tests {
		if got := clampScore(tt.input); got != tt.expected {
			t.Errorf("clampScore(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// ============================================================================
// Stub Component Calculator Tests
// ============================================================================

func TestStubComponentCalculator_Defaults(t *testing.T) {
	stub := NewStubComponentCalculator()
	data := &coinprofiler.CoinData{Symbol: "BTCUSDT"}

	// Test default values
	trend, dir, err := stub.CalculateTrendStrength(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if trend != 70 {
		t.Errorf("Default trend score = %d, want 70", trend)
	}
	if dir != DirectionLong {
		t.Errorf("Default trend direction = %s, want long", dir)
	}

	momentum, momDir, err := stub.CalculateMomentum(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if momentum != 65 {
		t.Errorf("Default momentum score = %d, want 65", momentum)
	}
	if momDir != DirectionLong {
		t.Errorf("Default momentum direction = %s, want long", momDir)
	}

	volume, err := stub.CalculateVolume(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if volume != 60 {
		t.Errorf("Default volume score = %d, want 60", volume)
	}

	priceAction, paDir, err := stub.CalculatePriceAction(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if priceAction != 55 {
		t.Errorf("Default price action score = %d, want 55", priceAction)
	}
	if paDir != DirectionLong {
		t.Errorf("Default price action direction = %s, want long", paDir)
	}

	sr, err := stub.CalculateSRProximity(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if sr != 50 {
		t.Errorf("Default SR score = %d, want 50", sr)
	}
}

func TestStubComponentCalculator_WithHighVolume(t *testing.T) {
	stub := NewStubComponentCalculator()

	// High volume should increase score
	highVolumeData := &coinprofiler.CoinData{
		Symbol:    "BTCUSDT",
		Volume24h: 150000000, // 150M
	}

	volume, err := stub.CalculateVolume(highVolumeData)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Default 60 + 15 (for >100M) = 75
	if volume != 75 {
		t.Errorf("High volume score = %d, want 75", volume)
	}
}

func TestStubComponentCalculator_WithHighVolatility(t *testing.T) {
	stub := NewStubComponentCalculator()

	highVolData := &coinprofiler.CoinData{
		Symbol:     "BTCUSDT",
		Price:      43500,
		Volatility: 3.5, // >2.0
	}

	trend, _, err := stub.CalculateTrendStrength(highVolData)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Default 70 + 10 (for >2.0 volatility) = 80
	if trend != 80 {
		t.Errorf("High volatility trend score = %d, want 80", trend)
	}
}

// ============================================================================
// Interface Compliance Tests
// ============================================================================

func TestScoreCalculator_InterfaceCompliance(t *testing.T) {
	// Ensure TrendFollowingScoreCalculator implements ScoreCalculator
	var _ ScoreCalculator = (*TrendFollowingScoreCalculator)(nil)
}

func TestComponentCalculator_InterfaceCompliance(t *testing.T) {
	// Ensure StubComponentCalculator implements ComponentCalculator
	var _ ComponentCalculator = (*StubComponentCalculator)(nil)
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestCalculateScore_FullIntegration(t *testing.T) {
	// Test full flow with realistic data
	config := &ScoreCalculatorConfig{
		Threshold: 60,
		Weights: &ScoreWeights{
			TrendStrength: 35,
			Momentum:      25,
			Volume:        20,
			PriceAction:   12,
			SRProximity:   8,
		},
	}

	stubCalc := NewStubComponentCalculator().
		WithFixedScores(85, 72, 68, 60, 55).
		WithFixedDirections(DirectionLong, DirectionLong, DirectionShort)

	calc, err := NewTrendFollowingScoreCalculatorWithComponents(config, stubCalc)
	if err != nil {
		t.Fatalf("Failed to create calculator: %v", err)
	}

	data := &coinprofiler.CoinData{
		Symbol:    "BTCUSDT",
		Price:     43500.00,
		Volume24h: 75000000,
		Timeframes: map[string]*coinprofiler.TimeframeData{
			"1h": {
				Timeframe: "1h",
				Open:      43400,
				High:      43600,
				Low:       43300,
				Close:     43500,
				Volume:    1500000,
			},
		},
		UpdatedAt: time.Now(),
	}

	result, err := calc.CalculateScore("BTCUSDT", data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify result structure
	if result.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %s, want BTCUSDT", result.Symbol)
	}

	// Calculate expected score:
	// (85*35 + 72*25 + 68*20 + 60*12 + 55*8) / 100
	// = (2975 + 1800 + 1360 + 720 + 440) / 100
	// = 7295 / 100 = 72 (integer division)
	expectedScore := 72
	if result.Score != expectedScore {
		t.Errorf("Score = %d, want %d", result.Score, expectedScore)
	}

	// Threshold 60, score 72 - should be ready
	if !result.Ready {
		t.Errorf("Ready = %v, want true (score %d >= threshold %d)", result.Ready, result.Score, config.Threshold)
	}

	// Direction: trend(35) + momentum(25) = 60 long, priceAction(12) = 12 short
	// Long wins
	if result.Direction != DirectionLong {
		t.Errorf("Direction = %s, want long", result.Direction)
	}

	// Verify threshold is recorded
	if result.Threshold != 60 {
		t.Errorf("Threshold = %d, want 60", result.Threshold)
	}

	// Convert to CoinMatch
	coinMatch := result.ToCoinMatch()
	if coinMatch == nil {
		t.Fatal("CoinMatch should not be nil")
	}
	if coinMatch.Score != result.Score {
		t.Errorf("CoinMatch.Score = %d, want %d", coinMatch.Score, result.Score)
	}
}
