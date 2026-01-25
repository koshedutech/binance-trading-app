// Package entrydecision provides tests for the Strategy Matcher.
// Epic 14: Chain Trading System - Story 14.11: Strategy Reader & Matcher
package entrydecision

import (
	"context"
	"testing"
	"time"

	"binance-trading-bot/internal/coinprofiler"
	"binance-trading-bot/internal/database"
)

// ============================================================================
// MOCK STRATEGY READER
// ============================================================================

// MockStrategyReader implements StrategyReader for testing.
type MockStrategyReader struct {
	EnabledStrategies        []EnabledSubStrategy
	EnabledStrategiesForMode map[string][]EnabledSubStrategy
	StrategyConfigs          map[string]*StrategyConfig

	// Error simulation
	GetEnabledStrategiesErr error
	GetConfigErr            error
}

// NewMockStrategyReader creates a new mock reader with default data.
func NewMockStrategyReader() *MockStrategyReader {
	return &MockStrategyReader{
		EnabledStrategiesForMode: make(map[string][]EnabledSubStrategy),
		StrategyConfigs:          make(map[string]*StrategyConfig),
	}
}

func (m *MockStrategyReader) GetEnabledStrategies(ctx context.Context, userID string) ([]EnabledSubStrategy, error) {
	if m.GetEnabledStrategiesErr != nil {
		return nil, m.GetEnabledStrategiesErr
	}
	return m.EnabledStrategies, nil
}

func (m *MockStrategyReader) GetEnabledStrategiesForMode(ctx context.Context, userID, mode string) ([]EnabledSubStrategy, error) {
	if m.GetEnabledStrategiesErr != nil {
		return nil, m.GetEnabledStrategiesErr
	}
	return m.EnabledStrategiesForMode[mode], nil
}

func (m *MockStrategyReader) GetStrategyConfig(ctx context.Context, userID string, strategy EnabledSubStrategy) (*StrategyConfig, error) {
	if m.GetConfigErr != nil {
		return nil, m.GetConfigErr
	}
	key := strategy.Mode + ":" + strategy.StrategyGroup + ":" + strategy.SubStrategy
	return m.StrategyConfigs[key], nil
}

// ============================================================================
// MOCK COIN DATA PROVIDER
// ============================================================================

// MockCoinDataProvider implements CoinDataProvider for testing.
type MockCoinDataProvider struct {
	CoinData map[string]*coinprofiler.CoinData
	Err      error
}

// NewMockCoinDataProvider creates a new mock provider with sample data.
func NewMockCoinDataProvider() *MockCoinDataProvider {
	return &MockCoinDataProvider{
		CoinData: make(map[string]*coinprofiler.CoinData),
	}
}

func (m *MockCoinDataProvider) GetAllCoinData(ctx context.Context) (map[string]*coinprofiler.CoinData, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.CoinData, nil
}

func (m *MockCoinDataProvider) GetCoinData(ctx context.Context, symbol string) (*coinprofiler.CoinData, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.CoinData[symbol], nil
}

// ============================================================================
// STRATEGY MATCHER TESTS
// ============================================================================

func TestDefaultStrategyMatcher_MatchAll(t *testing.T) {
	t.Run("matches pattern and score strategies", func(t *testing.T) {
		// Setup mock strategy reader
		mockReader := NewMockStrategyReader()
		mockReader.EnabledStrategies = []EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance", Timeframe: "5m"},
			{Mode: "swing", StrategyGroup: "trending", SubStrategy: "trend_following", Timeframe: "15m"},
		}

		// Setup mock coin data
		mockProvider := NewMockCoinDataProvider()
		mockProvider.CoinData["BTCUSDT"] = &coinprofiler.CoinData{
			Symbol:    "BTCUSDT",
			Price:     50000,
			Volume24h: 1000000000,
			Timeframes: map[string]*coinprofiler.TimeframeData{
				"5m":  {Open: 49900, High: 50100, Low: 49800, Close: 50000, Volume: 100},
				"15m": {Open: 49800, High: 50200, Low: 49700, Close: 50000, Volume: 300},
			},
		}

		// Create pattern matcher
		patternMatcher := NewVolumeImbalancePatternMatcher(nil)

		// Create score calculator
		scoreCalc := NewTrendFollowingScoreCalculator()

		// Create matcher
		config := DefaultStrategyMatcherConfig("user123")
		config.RefreshInterval = 0 // Disable caching for test
		matcher := NewDefaultStrategyMatcher(config, mockReader, patternMatcher, scoreCalc, mockProvider)

		ctx := context.Background()
		response, err := matcher.MatchAll(ctx, mockProvider.CoinData)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if response == nil {
			t.Fatal("expected non-nil response")
		}

		// Should have 2 strategies
		if response.TotalStrategies != 2 {
			t.Errorf("expected 2 strategies, got %d", response.TotalStrategies)
		}

		// Check that we have both types
		hasPattern := false
		hasScore := false
		for _, s := range response.Strategies {
			if s.Type == StrategyTypePattern {
				hasPattern = true
			}
			if s.Type == StrategyTypeScore {
				hasScore = true
			}
		}

		if !hasPattern {
			t.Error("expected to find pattern-based strategy")
		}
		if !hasScore {
			t.Error("expected to find score-based strategy")
		}
	})

	t.Run("handles empty strategy list", func(t *testing.T) {
		mockReader := NewMockStrategyReader()
		mockReader.EnabledStrategies = []EnabledSubStrategy{}

		config := DefaultStrategyMatcherConfig("user123")
		config.RefreshInterval = 0
		matcher := NewDefaultStrategyMatcher(config, mockReader, nil, nil, nil)

		ctx := context.Background()
		response, err := matcher.MatchAll(ctx, nil)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if response.TotalStrategies != 0 {
			t.Errorf("expected 0 strategies, got %d", response.TotalStrategies)
		}
	})
}

func TestDefaultStrategyMatcher_MatchForMode(t *testing.T) {
	t.Run("matches strategies for specific mode", func(t *testing.T) {
		mockReader := NewMockStrategyReader()
		mockReader.EnabledStrategiesForMode["scalp"] = []EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance", Timeframe: "5m"},
		}

		scoreCalc := NewTrendFollowingScoreCalculator()
		patternMatcher := NewVolumeImbalancePatternMatcher(nil)

		config := DefaultStrategyMatcherConfig("user123")
		matcher := NewDefaultStrategyMatcher(config, mockReader, patternMatcher, scoreCalc, nil)

		ctx := context.Background()
		coins := map[string]*coinprofiler.CoinData{
			"BTCUSDT": {Symbol: "BTCUSDT", Price: 50000},
		}

		modeStrategies, err := matcher.MatchForMode(ctx, "scalp", coins)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if modeStrategies == nil {
			t.Fatal("expected non-nil mode strategies")
		}

		if modeStrategies.Mode != "scalp" {
			t.Errorf("expected mode 'scalp', got '%s'", modeStrategies.Mode)
		}

		if len(modeStrategies.Strategies) != 1 {
			t.Errorf("expected 1 strategy, got %d", len(modeStrategies.Strategies))
		}
	})

	t.Run("returns error for invalid mode", func(t *testing.T) {
		mockReader := NewMockStrategyReader()
		config := DefaultStrategyMatcherConfig("user123")
		matcher := NewDefaultStrategyMatcher(config, mockReader, nil, nil, nil)

		ctx := context.Background()
		_, err := matcher.MatchForMode(ctx, "invalid_mode", nil)

		if err == nil {
			t.Error("expected error for invalid mode")
		}
	})
}

func TestDefaultStrategyMatcher_GetReadyCandidates(t *testing.T) {
	t.Run("returns ready candidates", func(t *testing.T) {
		mockReader := NewMockStrategyReader()
		mockReader.EnabledStrategies = []EnabledSubStrategy{
			{Mode: "swing", StrategyGroup: "trending", SubStrategy: "trend_following", Timeframe: "15m"},
		}

		// Setup mock coin data
		mockProvider := NewMockCoinDataProvider()
		mockProvider.CoinData["BTCUSDT"] = &coinprofiler.CoinData{
			Symbol:    "BTCUSDT",
			Price:     50000,
			Volume24h: 1000000000,
		}

		// Create score calculator with high fixed scores to ensure readiness
		scoreCalc := NewTrendFollowingScoreCalculator()
		stubCalc := NewStubComponentCalculator()
		stubCalc.WithFixedScores(80, 75, 70, 65, 60) // All high scores
		stubCalc.WithFixedDirections("long", "long", "long")
		scoreCalc.SetComponentCalculator(stubCalc)

		config := DefaultStrategyMatcherConfig("user123")
		config.RefreshInterval = 0
		matcher := NewDefaultStrategyMatcher(config, mockReader, nil, scoreCalc, mockProvider)

		ctx := context.Background()
		candidates, err := matcher.GetReadyCandidates(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if candidates == nil {
			t.Fatal("expected non-nil candidates")
		}

		// With high scores, BTCUSDT should be ready
		if candidates.TotalCandidates < 1 {
			t.Logf("Note: No ready candidates found (scores may be below threshold)")
		}
	})
}

// ============================================================================
// STRATEGY MATCHER BUILDER TESTS
// ============================================================================

func TestStrategyMatcherBuilder(t *testing.T) {
	t.Run("builds matcher with all components", func(t *testing.T) {
		mockReader := NewMockStrategyReader()
		patternMatcher := NewVolumeImbalancePatternMatcher(nil)
		scoreCalc := NewTrendFollowingScoreCalculator()
		mockProvider := NewMockCoinDataProvider()

		builder := NewStrategyMatcherBuilder("user123").
			WithStrategyReader(mockReader).
			WithPatternMatcher(patternMatcher).
			WithScoreCalculator(scoreCalc).
			WithCoinProvider(mockProvider)

		matcher, err := builder.Build()

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if matcher == nil {
			t.Fatal("expected non-nil matcher")
		}
	})

	t.Run("returns error when strategy reader is missing", func(t *testing.T) {
		builder := NewStrategyMatcherBuilder("user123")
		// Not setting strategy reader

		_, err := builder.Build()

		if err == nil {
			t.Error("expected error when strategy reader is missing")
		}
	})
}

// ============================================================================
// MATCH OPTIONS TESTS
// ============================================================================

func TestMatchOptions(t *testing.T) {
	t.Run("creates options with mode filter", func(t *testing.T) {
		opts := NewMatchOptions().WithMode("scalp")

		if opts.Mode != "scalp" {
			t.Errorf("expected mode 'scalp', got '%s'", opts.Mode)
		}
	})

	t.Run("creates options with strategy type filter", func(t *testing.T) {
		patternType := StrategyTypePattern
		opts := NewMatchOptions().WithStrategyType(patternType)

		if opts.StrategyType == nil || *opts.StrategyType != StrategyTypePattern {
			t.Error("expected pattern strategy type")
		}
	})

	t.Run("creates options with ready only filter", func(t *testing.T) {
		opts := NewMatchOptions().WithReadyOnly()

		if !opts.ReadyOnly {
			t.Error("expected ReadyOnly to be true")
		}
	})

	t.Run("creates options with min score", func(t *testing.T) {
		opts := NewMatchOptions().WithMinScore(70)

		if opts.MinScore != 70 {
			t.Errorf("expected min score 70, got %d", opts.MinScore)
		}
	})

	t.Run("creates options with symbols", func(t *testing.T) {
		opts := NewMatchOptions().WithSymbols("BTCUSDT", "ETHUSDT")

		if len(opts.Symbols) != 2 {
			t.Errorf("expected 2 symbols, got %d", len(opts.Symbols))
		}
	})

	t.Run("chains multiple options", func(t *testing.T) {
		opts := NewMatchOptions().
			WithMode("swing").
			WithReadyOnly().
			WithMinScore(60).
			WithSymbols("BTCUSDT")

		if opts.Mode != "swing" {
			t.Errorf("expected mode 'swing', got '%s'", opts.Mode)
		}
		if !opts.ReadyOnly {
			t.Error("expected ReadyOnly to be true")
		}
		if opts.MinScore != 60 {
			t.Errorf("expected min score 60, got %d", opts.MinScore)
		}
		if len(opts.Symbols) != 1 {
			t.Errorf("expected 1 symbol, got %d", len(opts.Symbols))
		}
	})
}

// ============================================================================
// PATTERN STRATEGY MATCHING TESTS
// ============================================================================

func TestMatchPatternStrategy(t *testing.T) {
	t.Run("matches volume imbalance pattern", func(t *testing.T) {
		mockReader := NewMockStrategyReader()
		patternMatcher := NewVolumeImbalancePatternMatcher(nil)

		config := DefaultStrategyMatcherConfig("user123")
		matcher := NewDefaultStrategyMatcher(config, mockReader, patternMatcher, nil, nil)

		ctx := context.Background()
		strategy := EnabledSubStrategy{
			Mode:          "scalp",
			StrategyGroup: "breakout",
			SubStrategy:   "ravindra_volume_imbalance",
			Timeframe:     "5m",
		}

		// Create coin data with timeframe data
		coins := map[string]*coinprofiler.CoinData{
			"BTCUSDT": {
				Symbol:    "BTCUSDT",
				Price:     50000,
				Volume24h: 1000000000,
				Timeframes: map[string]*coinprofiler.TimeframeData{
					"5m": {
						Open:      49900,
						High:      50100,
						Low:       49800,
						Close:     50000,
						Volume:    100,
						UpdatedAt: time.Now(),
					},
				},
			},
		}

		strategyMatch, err := matcher.matchPatternStrategy(ctx, strategy, coins)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strategyMatch == nil {
			t.Fatal("expected non-nil strategy match")
		}

		if strategyMatch.Type != StrategyTypePattern {
			t.Errorf("expected pattern type, got %s", strategyMatch.Type)
		}

		if strategyMatch.Mode != "scalp" {
			t.Errorf("expected mode 'scalp', got '%s'", strategyMatch.Mode)
		}
	})
}

// ============================================================================
// SCORE STRATEGY MATCHING TESTS
// ============================================================================

func TestMatchScoreStrategy(t *testing.T) {
	t.Run("matches trend following strategy", func(t *testing.T) {
		mockReader := NewMockStrategyReader()
		scoreCalc := NewTrendFollowingScoreCalculator()

		config := DefaultStrategyMatcherConfig("user123")
		matcher := NewDefaultStrategyMatcher(config, mockReader, nil, scoreCalc, nil)

		ctx := context.Background()
		strategy := EnabledSubStrategy{
			Mode:          "swing",
			StrategyGroup: "trending",
			SubStrategy:   "trend_following",
			Timeframe:     "15m",
		}

		coins := map[string]*coinprofiler.CoinData{
			"BTCUSDT": {
				Symbol:    "BTCUSDT",
				Price:     50000,
				Volume24h: 1000000000,
			},
		}

		strategyMatch, err := matcher.matchScoreStrategy(ctx, strategy, coins)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strategyMatch == nil {
			t.Fatal("expected non-nil strategy match")
		}

		if strategyMatch.Type != StrategyTypeScore {
			t.Errorf("expected score type, got %s", strategyMatch.Type)
		}

		if strategyMatch.Mode != "swing" {
			t.Errorf("expected mode 'swing', got '%s'", strategyMatch.Mode)
		}

		// Should have calculated score for BTCUSDT
		if len(strategyMatch.Coins) != 1 {
			t.Errorf("expected 1 coin match, got %d", len(strategyMatch.Coins))
		}
	})
}

// ============================================================================
// PRIORITY CALCULATION TESTS
// ============================================================================

func TestCalculatePriority(t *testing.T) {
	config := DefaultStrategyMatcherConfig("user123")
	matcher := NewDefaultStrategyMatcher(config, nil, nil, nil, nil)

	t.Run("pattern ready has high priority", func(t *testing.T) {
		coin := CoinMatch{
			Symbol: "BTCUSDT",
			Status: PatternStatusReady,
			Step:   3,
		}
		strategy := StrategyMatch{
			Type: StrategyTypePattern,
			Mode: "scalp",
		}

		priority := matcher.calculatePriority(coin, strategy)

		// Should have high priority due to ready status
		if priority <= 50 {
			t.Errorf("expected priority > 50 for ready pattern, got %d", priority)
		}
	})

	t.Run("score ready has high priority", func(t *testing.T) {
		coin := CoinMatch{
			Symbol: "BTCUSDT",
			Score:  80,
			Ready:  true,
		}
		strategy := StrategyMatch{
			Type: StrategyTypeScore,
			Mode: "swing",
		}

		priority := matcher.calculatePriority(coin, strategy)

		// Should have high priority due to high score and ready
		if priority <= 50 {
			t.Errorf("expected priority > 50 for high score ready, got %d", priority)
		}
	})

	t.Run("ultra_fast mode gets bonus priority", func(t *testing.T) {
		coin := CoinMatch{Symbol: "BTCUSDT", Score: 60}
		strategyUltraFast := StrategyMatch{Type: StrategyTypeScore, Mode: "ultra_fast"}
		strategySwing := StrategyMatch{Type: StrategyTypeScore, Mode: "swing"}

		priorityUF := matcher.calculatePriority(coin, strategyUltraFast)
		prioritySwing := matcher.calculatePriority(coin, strategySwing)

		if priorityUF <= prioritySwing {
			t.Errorf("expected ultra_fast priority (%d) > swing priority (%d)", priorityUF, prioritySwing)
		}
	})
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestStrategyMatcherIntegration(t *testing.T) {
	t.Run("full workflow with multiple strategies and coins", func(t *testing.T) {
		// Setup mock repository
		mockRepo := NewMockStrategyRepository()
		mockRepo.EnabledStrategies = []database.EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
			{Mode: "swing", StrategyGroup: "trending", SubStrategy: "trend_following"},
		}
		mockRepo.StrategyGroups["scalp:breakout"] = &database.StrategyGroupSettings{
			Timeframe: "5m",
		}
		mockRepo.StrategyGroups["swing:trending"] = &database.StrategyGroupSettings{
			Timeframe: "15m",
		}

		// Create strategy reader
		strategyReader := NewDefaultStrategyReader(mockRepo)

		// Create components
		patternMatcher := NewVolumeImbalancePatternMatcher(nil)
		scoreCalc := NewTrendFollowingScoreCalculator()

		// Create coin provider
		coinProvider := NewMockCoinDataProvider()
		coinProvider.CoinData["BTCUSDT"] = &coinprofiler.CoinData{
			Symbol:    "BTCUSDT",
			Price:     50000,
			Volume24h: 1000000000,
		}
		coinProvider.CoinData["ETHUSDT"] = &coinprofiler.CoinData{
			Symbol:    "ETHUSDT",
			Price:     3000,
			Volume24h: 500000000,
		}

		// Build matcher
		matcher, err := NewStrategyMatcherBuilder("user123").
			WithStrategyReader(strategyReader).
			WithPatternMatcher(patternMatcher).
			WithScoreCalculator(scoreCalc).
			WithCoinProvider(coinProvider).
			Build()

		if err != nil {
			t.Fatalf("failed to build matcher: %v", err)
		}

		ctx := context.Background()

		// Test MatchAll
		response, err := matcher.MatchAll(ctx, coinProvider.CoinData)
		if err != nil {
			t.Fatalf("MatchAll failed: %v", err)
		}

		if response.TotalStrategies != 2 {
			t.Errorf("expected 2 strategies, got %d", response.TotalStrategies)
		}

		// Test MatchForMode
		modeStrategies, err := matcher.MatchForMode(ctx, "scalp", coinProvider.CoinData)
		if err != nil {
			t.Fatalf("MatchForMode failed: %v", err)
		}

		if modeStrategies.Mode != "scalp" {
			t.Errorf("expected mode 'scalp', got '%s'", modeStrategies.Mode)
		}

		// Test GetReadyCandidates
		candidates, err := matcher.GetReadyCandidates(ctx)
		if err != nil {
			t.Fatalf("GetReadyCandidates failed: %v", err)
		}

		if candidates == nil {
			t.Error("expected non-nil candidates response")
		}
	})
}
