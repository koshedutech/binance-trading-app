package coinprofiler

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
)

// ============================================================================
// TEST: GetStrategyRequirements
// ============================================================================

func TestGetStrategyRequirements(t *testing.T) {
	t.Run("returns requirements for known strategy - volume imbalance", func(t *testing.T) {
		req := GetStrategyRequirements("scalp", "breakout", "ravindra_volume_imbalance")

		if req == nil {
			t.Fatal("expected non-nil requirements")
		}
		if req.Mode != "scalp" {
			t.Errorf("expected mode 'scalp', got %s", req.Mode)
		}
		if req.Strategy != "breakout" {
			t.Errorf("expected strategy 'breakout', got %s", req.Strategy)
		}
		if req.SubStrategy != "ravindra_volume_imbalance" {
			t.Errorf("expected sub-strategy 'ravindra_volume_imbalance', got %s", req.SubStrategy)
		}

		// Check timeframes - scalp should use 3m (backtested Dec 2025 - Jan 2026)
		if len(req.Timeframes) == 0 {
			t.Error("expected non-empty timeframes")
		}
		if req.Timeframes[0] != "3m" {
			t.Errorf("expected 3m timeframe for scalp mode (backtested), got %v", req.Timeframes)
		}

		// Check data fields
		hasVolume := false
		hasTakerBuy := false
		hasOHLC := false
		for _, df := range req.DataFields {
			if df == "volume" {
				hasVolume = true
			}
			if df == "taker_buy_volume" {
				hasTakerBuy = true
			}
			if df == "ohlc" {
				hasOHLC = true
			}
		}
		if !hasVolume {
			t.Error("expected 'volume' in data fields")
		}
		if !hasTakerBuy {
			t.Error("expected 'taker_buy_volume' in data fields")
		}
		if !hasOHLC {
			t.Error("expected 'ohlc' in data fields")
		}
	})

	t.Run("returns different timeframes for different modes", func(t *testing.T) {
		scalpReq := GetStrategyRequirements("scalp", "breakout", "ravindra_volume_imbalance")
		swingReq := GetStrategyRequirements("swing", "breakout", "ravindra_volume_imbalance")
		positionReq := GetStrategyRequirements("position", "breakout", "ravindra_volume_imbalance")

		// Scalp uses 3m (backtested Dec 2025 - Jan 2026)
		if scalpReq.Timeframes[0] != "3m" {
			t.Errorf("scalp should use 3m (backtested), got %v", scalpReq.Timeframes)
		}
		if swingReq.Timeframes[0] != "1h" {
			t.Errorf("swing should use 1h, got %v", swingReq.Timeframes)
		}
		if positionReq.Timeframes[0] != "4h" {
			t.Errorf("position should use 4h, got %v", positionReq.Timeframes)
		}
	})

	t.Run("returns requirements for classic breakout", func(t *testing.T) {
		req := GetStrategyRequirements("scalp", "breakout", "classic_breakout")

		if req == nil {
			t.Fatal("expected non-nil requirements")
		}
		if len(req.Timeframes) == 0 {
			t.Error("expected non-empty timeframes")
		}

		// Classic breakout should have ohlc and volume
		hasOHLC := false
		hasVolume := false
		for _, df := range req.DataFields {
			if df == "ohlc" {
				hasOHLC = true
			}
			if df == "volume" {
				hasVolume = true
			}
		}
		if !hasOHLC {
			t.Error("expected 'ohlc' in data fields")
		}
		if !hasVolume {
			t.Error("expected 'volume' in data fields")
		}
	})

	t.Run("returns default requirements for unknown strategy", func(t *testing.T) {
		req := GetStrategyRequirements("scalp", "unknown_group", "unknown_strategy")

		if req == nil {
			t.Fatal("expected non-nil requirements for unknown strategy")
		}
		if req.Mode != "scalp" {
			t.Errorf("expected mode 'scalp', got %s", req.Mode)
		}
		if req.SubStrategy != "unknown_strategy" {
			t.Errorf("expected sub-strategy 'unknown_strategy', got %s", req.SubStrategy)
		}
		if len(req.Timeframes) == 0 {
			t.Error("expected default timeframes for unknown strategy")
		}
		if len(req.DataFields) == 0 {
			t.Error("expected default data fields for unknown strategy")
		}
	})
}

// ============================================================================
// TEST: GetRequirementsForStrategies
// ============================================================================

func TestGetRequirementsForStrategies(t *testing.T) {
	t.Run("extracts requirements for multiple strategies", func(t *testing.T) {
		strategies := []EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
			{Mode: "swing", StrategyGroup: "trending", SubStrategy: "trend_following"},
		}

		requirements := GetRequirementsForStrategies(strategies)

		if len(requirements) != 2 {
			t.Errorf("expected 2 requirements, got %d", len(requirements))
		}

		// Verify first strategy
		if requirements[0].Mode != "scalp" {
			t.Errorf("expected first requirement mode 'scalp', got %s", requirements[0].Mode)
		}
		if requirements[0].SubStrategy != "ravindra_volume_imbalance" {
			t.Errorf("expected first sub-strategy 'ravindra_volume_imbalance', got %s", requirements[0].SubStrategy)
		}

		// Verify second strategy
		if requirements[1].Mode != "swing" {
			t.Errorf("expected second requirement mode 'swing', got %s", requirements[1].Mode)
		}
		if requirements[1].SubStrategy != "trend_following" {
			t.Errorf("expected second sub-strategy 'trend_following', got %s", requirements[1].SubStrategy)
		}
	})

	t.Run("handles empty strategy list", func(t *testing.T) {
		requirements := GetRequirementsForStrategies(nil)

		if requirements == nil {
			t.Error("expected non-nil result for empty input")
		}
		if len(requirements) != 0 {
			t.Errorf("expected 0 requirements, got %d", len(requirements))
		}
	})

	t.Run("handles same strategy in different modes", func(t *testing.T) {
		strategies := []EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
			{Mode: "swing", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
			{Mode: "position", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
		}

		requirements := GetRequirementsForStrategies(strategies)

		if len(requirements) != 3 {
			t.Errorf("expected 3 requirements, got %d", len(requirements))
		}

		// Verify different timeframes for same strategy in different modes
		modes := make(map[string][]string)
		for _, req := range requirements {
			modes[req.Mode] = req.Timeframes
		}

		if modes["scalp"][0] == modes["swing"][0] {
			t.Error("expected different timeframes for scalp and swing modes")
		}
		if modes["swing"][0] == modes["position"][0] {
			t.Error("expected different timeframes for swing and position modes")
		}
	})
}

// ============================================================================
// TEST: AggregateRequirements
// ============================================================================

func TestAggregateRequirements(t *testing.T) {
	t.Run("aggregates and deduplicates timeframes", func(t *testing.T) {
		requirements := []StrategyRequirements{
			{
				Mode:        "scalp",
				Strategy:    "breakout",
				SubStrategy: "ravindra_volume_imbalance",
				Timeframes:  []string{"15m", "1h"},
				DataFields:  []string{"volume", "ohlc"},
			},
			{
				Mode:        "swing",
				Strategy:    "trending",
				SubStrategy: "trend_following",
				Timeframes:  []string{"1h", "4h"}, // 1h is duplicate
				DataFields:  []string{"ohlc", "volume"},
			},
		}

		agg := AggregateRequirements(requirements)

		if agg == nil {
			t.Fatal("expected non-nil aggregated requirements")
		}

		// Should have 3 unique timeframes: 15m, 1h, 4h
		if len(agg.AllTimeframes) != 3 {
			t.Errorf("expected 3 unique timeframes, got %d: %v", len(agg.AllTimeframes), agg.AllTimeframes)
		}

		// Check that 1h is only listed once
		oneHCount := 0
		for _, tf := range agg.AllTimeframes {
			if tf == "1h" {
				oneHCount++
			}
		}
		if oneHCount != 1 {
			t.Errorf("expected 1h to appear once, appeared %d times", oneHCount)
		}
	})

	t.Run("aggregates and deduplicates data fields", func(t *testing.T) {
		requirements := []StrategyRequirements{
			{
				Mode:        "scalp",
				Strategy:    "breakout",
				SubStrategy: "test1",
				Timeframes:  []string{"15m"},
				DataFields:  []string{"volume", "ohlc", "taker_buy_volume"},
			},
			{
				Mode:        "swing",
				Strategy:    "trending",
				SubStrategy: "test2",
				Timeframes:  []string{"1h"},
				DataFields:  []string{"ohlc", "volume"}, // duplicates
			},
		}

		agg := AggregateRequirements(requirements)

		// Should have 3 unique data fields
		if len(agg.AllDataFields) != 3 {
			t.Errorf("expected 3 unique data fields, got %d: %v", len(agg.AllDataFields), agg.AllDataFields)
		}
	})

	t.Run("builds ByTimeframe mapping correctly", func(t *testing.T) {
		requirements := []StrategyRequirements{
			{
				Mode:        "scalp",
				Strategy:    "breakout",
				SubStrategy: "strat1",
				Timeframes:  []string{"15m", "1h"},
				DataFields:  []string{"ohlc"},
			},
			{
				Mode:        "swing",
				Strategy:    "trending",
				SubStrategy: "strat2",
				Timeframes:  []string{"1h", "4h"},
				DataFields:  []string{"ohlc"},
			},
		}

		agg := AggregateRequirements(requirements)

		// 15m should have 1 strategy
		if len(agg.ByTimeframe["15m"]) != 1 {
			t.Errorf("expected 1 strategy for 15m, got %d", len(agg.ByTimeframe["15m"]))
		}

		// 1h should have 2 strategies (both use it)
		if len(agg.ByTimeframe["1h"]) != 2 {
			t.Errorf("expected 2 strategies for 1h, got %d", len(agg.ByTimeframe["1h"]))
		}

		// 4h should have 1 strategy
		if len(agg.ByTimeframe["4h"]) != 1 {
			t.Errorf("expected 1 strategy for 4h, got %d", len(agg.ByTimeframe["4h"]))
		}
	})

	t.Run("preserves original requirements in ByStrategy", func(t *testing.T) {
		requirements := []StrategyRequirements{
			{
				Mode:        "scalp",
				Strategy:    "breakout",
				SubStrategy: "test",
				Timeframes:  []string{"15m"},
				DataFields:  []string{"ohlc"},
				Filters:     map[string]interface{}{"min_volume": 1000000},
			},
		}

		agg := AggregateRequirements(requirements)

		if len(agg.ByStrategy) != 1 {
			t.Errorf("expected 1 strategy in ByStrategy, got %d", len(agg.ByStrategy))
		}
		if agg.ByStrategy[0].Filters["min_volume"] != 1000000 {
			t.Error("expected filters to be preserved")
		}
	})

	t.Run("sets TotalStrategies correctly", func(t *testing.T) {
		requirements := []StrategyRequirements{
			{Mode: "scalp", Strategy: "a", SubStrategy: "a1", Timeframes: []string{"5m"}, DataFields: []string{"ohlc"}},
			{Mode: "swing", Strategy: "b", SubStrategy: "b1", Timeframes: []string{"1h"}, DataFields: []string{"ohlc"}},
			{Mode: "position", Strategy: "c", SubStrategy: "c1", Timeframes: []string{"4h"}, DataFields: []string{"ohlc"}},
		}

		agg := AggregateRequirements(requirements)

		if agg.TotalStrategies != 3 {
			t.Errorf("expected TotalStrategies=3, got %d", agg.TotalStrategies)
		}
	})

	t.Run("handles empty requirements list", func(t *testing.T) {
		agg := AggregateRequirements(nil)

		if agg == nil {
			t.Fatal("expected non-nil result for empty input")
		}
		if len(agg.AllTimeframes) != 0 {
			t.Error("expected empty AllTimeframes")
		}
		if len(agg.AllDataFields) != 0 {
			t.Error("expected empty AllDataFields")
		}
		if agg.TotalStrategies != 0 {
			t.Error("expected TotalStrategies=0")
		}
	})
}

// ============================================================================
// TEST: RequirementAggregator.Aggregate
// ============================================================================

// mockStrategyReader implements EnabledStrategyReader for testing
type mockStrategyReader struct {
	strategies []EnabledSubStrategy
	err        error
}

func (m *mockStrategyReader) GetEnabledStrategies(ctx context.Context, userID string) ([]EnabledSubStrategy, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.strategies, nil
}

func TestRequirementAggregator_Aggregate(t *testing.T) {
	t.Run("aggregates requirements from database", func(t *testing.T) {
		reader := &mockStrategyReader{
			strategies: []EnabledSubStrategy{
				{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
				{Mode: "swing", StrategyGroup: "trending", SubStrategy: "trend_following"},
			},
		}

		agg := NewRequirementAggregator(reader)
		result, err := agg.Aggregate(context.Background(), "user123")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.TotalStrategies != 2 {
			t.Errorf("expected 2 strategies, got %d", result.TotalStrategies)
		}
	})

	t.Run("propagates database errors", func(t *testing.T) {
		expectedErr := errors.New("database connection failed")
		reader := &mockStrategyReader{
			err: expectedErr,
		}

		agg := NewRequirementAggregator(reader)
		_, err := agg.Aggregate(context.Background(), "user123")

		if err == nil {
			t.Error("expected error, got nil")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("expected error '%v', got '%v'", expectedErr, err)
		}
	})

	t.Run("handles empty strategy list", func(t *testing.T) {
		reader := &mockStrategyReader{
			strategies: []EnabledSubStrategy{},
		}

		agg := NewRequirementAggregator(reader)
		result, err := agg.Aggregate(context.Background(), "user123")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalStrategies != 0 {
			t.Errorf("expected 0 strategies, got %d", result.TotalStrategies)
		}
	})
}

// ============================================================================
// TEST: Utility Functions
// ============================================================================

func TestTimeframePriority(t *testing.T) {
	t.Run("returns correct priority order", func(t *testing.T) {
		if TimeframePriority("1m") >= TimeframePriority("5m") {
			t.Error("1m should have lower priority than 5m")
		}
		if TimeframePriority("5m") >= TimeframePriority("15m") {
			t.Error("5m should have lower priority than 15m")
		}
		if TimeframePriority("15m") >= TimeframePriority("1h") {
			t.Error("15m should have lower priority than 1h")
		}
		if TimeframePriority("1h") >= TimeframePriority("4h") {
			t.Error("1h should have lower priority than 4h")
		}
		if TimeframePriority("4h") >= TimeframePriority("1d") {
			t.Error("4h should have lower priority than 1d")
		}
	})

	t.Run("returns high priority for unknown timeframe", func(t *testing.T) {
		unknownPriority := TimeframePriority("unknown")
		knownPriority := TimeframePriority("1d")

		if unknownPriority <= knownPriority {
			t.Error("unknown timeframe should have higher priority than known")
		}
	})
}

func TestSortTimeframes(t *testing.T) {
	t.Run("sorts timeframes by priority", func(t *testing.T) {
		input := []string{"4h", "1m", "1h", "15m", "5m"}
		expected := []string{"1m", "5m", "15m", "1h", "4h"}

		result := SortTimeframes(input)

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("does not modify original slice", func(t *testing.T) {
		input := []string{"4h", "1m", "1h"}
		originalFirst := input[0]

		SortTimeframes(input)

		if input[0] != originalFirst {
			t.Error("original slice should not be modified")
		}
	})

	t.Run("handles empty slice", func(t *testing.T) {
		result := SortTimeframes([]string{})
		if len(result) != 0 {
			t.Error("expected empty result")
		}
	})
}

func TestGetRegisteredStrategies(t *testing.T) {
	t.Run("returns known strategies", func(t *testing.T) {
		strategies := GetRegisteredStrategies()

		if len(strategies) == 0 {
			t.Error("expected at least one registered strategy")
		}

		// Should include ravindra_volume_imbalance
		found := false
		for _, s := range strategies {
			if s == "ravindra_volume_imbalance" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'ravindra_volume_imbalance' in registered strategies")
		}
	})

	t.Run("returns sorted list", func(t *testing.T) {
		strategies := GetRegisteredStrategies()

		sorted := make([]string, len(strategies))
		copy(sorted, strategies)
		sort.Strings(sorted)

		if !reflect.DeepEqual(strategies, sorted) {
			t.Error("strategies should be returned in sorted order")
		}
	})
}

func TestIsStrategyRegistered(t *testing.T) {
	t.Run("returns true for known strategy", func(t *testing.T) {
		if !IsStrategyRegistered("ravindra_volume_imbalance") {
			t.Error("expected ravindra_volume_imbalance to be registered")
		}
		if !IsStrategyRegistered("classic_breakout") {
			t.Error("expected classic_breakout to be registered")
		}
	})

	t.Run("returns false for unknown strategy", func(t *testing.T) {
		if IsStrategyRegistered("unknown_strategy") {
			t.Error("expected unknown_strategy to not be registered")
		}
	})
}

func TestGetStrategyConfig(t *testing.T) {
	t.Run("returns config for known strategy", func(t *testing.T) {
		config := GetStrategyConfig("ravindra_volume_imbalance")

		if config == nil {
			t.Fatal("expected non-nil config")
		}
		if len(config.TimeframesByMode) == 0 {
			t.Error("expected non-empty TimeframesByMode")
		}
		if len(config.DataFields) == 0 {
			t.Error("expected non-empty DataFields")
		}
	})

	t.Run("returns nil for unknown strategy", func(t *testing.T) {
		config := GetStrategyConfig("unknown_strategy")

		if config != nil {
			t.Error("expected nil config for unknown strategy")
		}
	})
}

func TestRegisterStrategy(t *testing.T) {
	t.Run("registers new strategy", func(t *testing.T) {
		customConfig := &StrategyDataConfig{
			TimeframesByMode: map[string][]string{
				"scalp": {"5m"},
			},
			DataFields: []string{"ohlc"},
		}

		RegisterStrategy("test_custom_strategy", customConfig)

		if !IsStrategyRegistered("test_custom_strategy") {
			t.Error("expected custom strategy to be registered")
		}

		// Clean up
		delete(strategyRegistry, "test_custom_strategy")
	})

	t.Run("does not register nil config", func(t *testing.T) {
		RegisterStrategy("nil_strategy", nil)

		if IsStrategyRegistered("nil_strategy") {
			t.Error("should not register nil config")
		}
	})
}

func TestGetDefaultTimeframesForMode(t *testing.T) {
	t.Run("returns correct timeframes for each mode", func(t *testing.T) {
		ultraFast := getDefaultTimeframesForMode("ultra_fast")
		if len(ultraFast) == 0 || ultraFast[0] != "1m" {
			t.Errorf("ultra_fast should start with 1m, got %v", ultraFast)
		}

		scalp := getDefaultTimeframesForMode("scalp")
		if len(scalp) == 0 || scalp[0] != "5m" {
			t.Errorf("scalp should start with 5m, got %v", scalp)
		}

		swing := getDefaultTimeframesForMode("swing")
		if len(swing) == 0 || swing[0] != "1h" {
			t.Errorf("swing should start with 1h, got %v", swing)
		}

		position := getDefaultTimeframesForMode("position")
		if len(position) == 0 || position[0] != "4h" {
			t.Errorf("position should start with 4h, got %v", position)
		}
	})

	t.Run("returns default for unknown mode", func(t *testing.T) {
		unknown := getDefaultTimeframesForMode("unknown_mode")

		if len(unknown) == 0 {
			t.Error("expected default timeframes for unknown mode")
		}
	})
}

// ============================================================================
// TEST: Mode + Strategy Combinations (AC5)
// ============================================================================

func TestModeStrategyCombinations(t *testing.T) {
	t.Run("same strategy different modes have different timeframes", func(t *testing.T) {
		strategies := []EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
			{Mode: "swing", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
			{Mode: "position", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
		}

		requirements := GetRequirementsForStrategies(strategies)
		agg := AggregateRequirements(requirements)

		// Should have 3 different strategies tracked
		if agg.TotalStrategies != 3 {
			t.Errorf("expected 3 total strategies, got %d", agg.TotalStrategies)
		}

		// Verify timeframes are aggregated correctly
		// 15m (scalp), 1h (swing), 4h (position) = 3 unique timeframes
		if len(agg.AllTimeframes) != 3 {
			t.Errorf("expected 3 unique timeframes, got %d: %v", len(agg.AllTimeframes), agg.AllTimeframes)
		}
	})

	t.Run("deduplicates when same timeframe across modes", func(t *testing.T) {
		// Classic breakout uses 15m for scalp and 15m+1h for swing
		strategies := []EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "classic_breakout"},
			{Mode: "swing", StrategyGroup: "breakout", SubStrategy: "classic_breakout"},
		}

		requirements := GetRequirementsForStrategies(strategies)
		agg := AggregateRequirements(requirements)

		// 15m should appear in ByTimeframe with both strategies
		strategiesFor15m := agg.ByTimeframe["15m"]
		if len(strategiesFor15m) != 2 {
			t.Errorf("expected 2 strategies for 15m, got %d", len(strategiesFor15m))
		}
	})
}

// ============================================================================
// TEST: Integration - Complete Flow
// ============================================================================

func TestIntegration_CompleteFlow(t *testing.T) {
	t.Run("complete aggregation flow", func(t *testing.T) {
		// Simulate reading from database
		reader := &mockStrategyReader{
			strategies: []EnabledSubStrategy{
				{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
				{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "classic_breakout"},
				{Mode: "swing", StrategyGroup: "trending", SubStrategy: "trend_following"},
			},
		}

		aggregator := NewRequirementAggregator(reader)
		result, err := aggregator.Aggregate(context.Background(), "test-user")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify aggregation
		if result.TotalStrategies != 3 {
			t.Errorf("expected 3 strategies, got %d", result.TotalStrategies)
		}

		// Verify timeframes are deduplicated
		timeframeSet := make(map[string]bool)
		for _, tf := range result.AllTimeframes {
			if timeframeSet[tf] {
				t.Errorf("timeframe %s appears more than once", tf)
			}
			timeframeSet[tf] = true
		}

		// Verify data fields are deduplicated
		dataFieldSet := make(map[string]bool)
		for _, df := range result.AllDataFields {
			if dataFieldSet[df] {
				t.Errorf("data field %s appears more than once", df)
			}
			dataFieldSet[df] = true
		}

		// Verify ByTimeframe mapping
		for tf, strats := range result.ByTimeframe {
			if len(strats) == 0 {
				t.Errorf("timeframe %s has no strategies", tf)
			}
		}
	})
}

// ============================================================================
// STORY 14.3: Position Requirement Aggregation Tests
// ============================================================================

// ============================================================================
// TEST: SimplePosition
// ============================================================================

func TestSimplePosition(t *testing.T) {
	t.Run("implements Position interface", func(t *testing.T) {
		pos := &SimplePosition{
			Symbol:         "BTCUSDT",
			Mode:           "scalp",
			Side:           "LONG",
			TakeProfit:     true,
			StopLoss:       true,
			TrailingActive: false,
		}

		// Verify it implements Position interface
		var _ Position = pos

		if pos.GetSymbol() != "BTCUSDT" {
			t.Errorf("expected symbol BTCUSDT, got %s", pos.GetSymbol())
		}
		if pos.GetMode() != "scalp" {
			t.Errorf("expected mode scalp, got %s", pos.GetMode())
		}
		if pos.GetSide() != "LONG" {
			t.Errorf("expected side LONG, got %s", pos.GetSide())
		}
		if !pos.HasTakeProfit() {
			t.Error("expected HasTakeProfit to be true")
		}
		if !pos.HasStopLoss() {
			t.Error("expected HasStopLoss to be true")
		}
		if pos.IsTrailingActive() {
			t.Error("expected IsTrailingActive to be false")
		}
	})
}

// ============================================================================
// TEST: getExitTimeframesForMode
// ============================================================================

func TestGetExitTimeframesForMode(t *testing.T) {
	t.Run("returns correct timeframes for ultra_fast mode", func(t *testing.T) {
		tf := getExitTimeframesForMode("ultra_fast")
		expected := []string{"1m", "5m"}
		if !reflect.DeepEqual(tf, expected) {
			t.Errorf("expected %v, got %v", expected, tf)
		}
	})

	t.Run("returns correct timeframes for scalp mode", func(t *testing.T) {
		tf := getExitTimeframesForMode("scalp")
		expected := []string{"5m", "15m"}
		if !reflect.DeepEqual(tf, expected) {
			t.Errorf("expected %v, got %v", expected, tf)
		}
	})

	t.Run("returns correct timeframes for swing mode", func(t *testing.T) {
		tf := getExitTimeframesForMode("swing")
		expected := []string{"15m", "1h"}
		if !reflect.DeepEqual(tf, expected) {
			t.Errorf("expected %v, got %v", expected, tf)
		}
	})

	t.Run("returns correct timeframes for position mode", func(t *testing.T) {
		tf := getExitTimeframesForMode("position")
		expected := []string{"1h", "4h"}
		if !reflect.DeepEqual(tf, expected) {
			t.Errorf("expected %v, got %v", expected, tf)
		}
	})

	t.Run("returns scalp timeframes for unknown mode", func(t *testing.T) {
		tf := getExitTimeframesForMode("unknown_mode")
		expected := []string{"5m", "15m"}
		if !reflect.DeepEqual(tf, expected) {
			t.Errorf("expected default %v, got %v", expected, tf)
		}
	})
}

// ============================================================================
// TEST: detectExitMode
// ============================================================================

func TestDetectExitMode(t *testing.T) {
	t.Run("returns tp_sl for position with TP only", func(t *testing.T) {
		pos := &SimplePosition{
			Symbol:         "BTCUSDT",
			Mode:           "scalp",
			TakeProfit:     true,
			StopLoss:       false,
			TrailingActive: false,
		}

		exitMode := detectExitMode(pos)
		if exitMode != ExitModeTPSL {
			t.Errorf("expected %s, got %s", ExitModeTPSL, exitMode)
		}
	})

	t.Run("returns tp_sl for position with SL only", func(t *testing.T) {
		pos := &SimplePosition{
			Symbol:         "BTCUSDT",
			Mode:           "scalp",
			TakeProfit:     false,
			StopLoss:       true,
			TrailingActive: false,
		}

		exitMode := detectExitMode(pos)
		if exitMode != ExitModeTPSL {
			t.Errorf("expected %s, got %s", ExitModeTPSL, exitMode)
		}
	})

	t.Run("returns tp_sl for position with both TP and SL", func(t *testing.T) {
		pos := &SimplePosition{
			Symbol:         "BTCUSDT",
			Mode:           "scalp",
			TakeProfit:     true,
			StopLoss:       true,
			TrailingActive: false,
		}

		exitMode := detectExitMode(pos)
		if exitMode != ExitModeTPSL {
			t.Errorf("expected %s, got %s", ExitModeTPSL, exitMode)
		}
	})

	t.Run("returns trailing for position with trailing only", func(t *testing.T) {
		pos := &SimplePosition{
			Symbol:         "BTCUSDT",
			Mode:           "scalp",
			TakeProfit:     false,
			StopLoss:       false,
			TrailingActive: true,
		}

		exitMode := detectExitMode(pos)
		if exitMode != ExitModeTrailing {
			t.Errorf("expected %s, got %s", ExitModeTrailing, exitMode)
		}
	})

	t.Run("returns both for position with TP/SL and trailing", func(t *testing.T) {
		pos := &SimplePosition{
			Symbol:         "BTCUSDT",
			Mode:           "scalp",
			TakeProfit:     true,
			StopLoss:       true,
			TrailingActive: true,
		}

		exitMode := detectExitMode(pos)
		if exitMode != ExitModeBoth {
			t.Errorf("expected %s, got %s", ExitModeBoth, exitMode)
		}
	})

	t.Run("returns tp_sl for position with no exit settings", func(t *testing.T) {
		pos := &SimplePosition{
			Symbol:         "BTCUSDT",
			Mode:           "scalp",
			TakeProfit:     false,
			StopLoss:       false,
			TrailingActive: false,
		}

		exitMode := detectExitMode(pos)
		if exitMode != ExitModeTPSL {
			t.Errorf("expected %s (default), got %s", ExitModeTPSL, exitMode)
		}
	})
}

// ============================================================================
// TEST: GetPositionRequirements
// ============================================================================

func TestGetPositionRequirements(t *testing.T) {
	t.Run("extracts requirements from single scalp position", func(t *testing.T) {
		positions := []Position{
			&SimplePosition{
				Symbol:     "BTCUSDT",
				Mode:       "scalp",
				Side:       "LONG",
				TakeProfit: true,
				StopLoss:   true,
			},
		}

		reqs := GetPositionRequirements(positions)

		if len(reqs) != 1 {
			t.Fatalf("expected 1 requirement, got %d", len(reqs))
		}
		if reqs[0].Symbol != "BTCUSDT" {
			t.Errorf("expected symbol BTCUSDT, got %s", reqs[0].Symbol)
		}
		if reqs[0].Mode != "scalp" {
			t.Errorf("expected mode scalp, got %s", reqs[0].Mode)
		}
		if reqs[0].Side != "LONG" {
			t.Errorf("expected side LONG, got %s", reqs[0].Side)
		}
		// Scalp should have 5m and 15m timeframes
		if len(reqs[0].Timeframes) != 2 {
			t.Errorf("expected 2 timeframes, got %d", len(reqs[0].Timeframes))
		}
	})

	t.Run("extracts requirements from multiple positions in different modes", func(t *testing.T) {
		positions := []Position{
			&SimplePosition{Symbol: "BTCUSDT", Mode: "scalp", Side: "LONG", StopLoss: true},
			&SimplePosition{Symbol: "ETHUSDT", Mode: "swing", Side: "SHORT", TakeProfit: true},
			&SimplePosition{Symbol: "SOLUSDT", Mode: "position", Side: "LONG", TrailingActive: true},
		}

		reqs := GetPositionRequirements(positions)

		if len(reqs) != 3 {
			t.Fatalf("expected 3 requirements, got %d", len(reqs))
		}

		// Verify each position has correct mode-specific timeframes
		modeTimeframes := map[string][]string{
			"scalp":    {"5m", "15m"},
			"swing":    {"15m", "1h"},
			"position": {"1h", "4h"},
		}

		for _, req := range reqs {
			expected := modeTimeframes[req.Mode]
			if !reflect.DeepEqual(req.Timeframes, expected) {
				t.Errorf("mode %s: expected timeframes %v, got %v", req.Mode, expected, req.Timeframes)
			}
		}
	})

	t.Run("handles empty position list", func(t *testing.T) {
		reqs := GetPositionRequirements([]Position{})

		if reqs == nil {
			t.Error("expected non-nil result for empty input")
		}
		if len(reqs) != 0 {
			t.Errorf("expected 0 requirements, got %d", len(reqs))
		}
	})

	t.Run("handles nil position list", func(t *testing.T) {
		reqs := GetPositionRequirements(nil)

		if reqs == nil {
			t.Error("expected non-nil result for nil input")
		}
		if len(reqs) != 0 {
			t.Errorf("expected 0 requirements, got %d", len(reqs))
		}
	})

	t.Run("skips nil positions in list", func(t *testing.T) {
		positions := []Position{
			&SimplePosition{Symbol: "BTCUSDT", Mode: "scalp", Side: "LONG", StopLoss: true},
			nil,
			&SimplePosition{Symbol: "ETHUSDT", Mode: "swing", Side: "SHORT", TakeProfit: true},
		}

		reqs := GetPositionRequirements(positions)

		if len(reqs) != 2 {
			t.Errorf("expected 2 requirements (skipping nil), got %d", len(reqs))
		}
	})

	t.Run("skips positions with empty symbol", func(t *testing.T) {
		positions := []Position{
			&SimplePosition{Symbol: "BTCUSDT", Mode: "scalp", Side: "LONG", StopLoss: true},
			&SimplePosition{Symbol: "", Mode: "swing", Side: "SHORT", TakeProfit: true},
		}

		reqs := GetPositionRequirements(positions)

		if len(reqs) != 1 {
			t.Errorf("expected 1 requirement (skipping empty symbol), got %d", len(reqs))
		}
	})

	t.Run("defaults to scalp for empty mode", func(t *testing.T) {
		positions := []Position{
			&SimplePosition{Symbol: "BTCUSDT", Mode: "", Side: "LONG", StopLoss: true},
		}

		reqs := GetPositionRequirements(positions)

		if len(reqs) != 1 {
			t.Fatalf("expected 1 requirement, got %d", len(reqs))
		}
		if reqs[0].Mode != "scalp" {
			t.Errorf("expected default mode 'scalp', got %s", reqs[0].Mode)
		}
		// Should have scalp timeframes
		expected := []string{"5m", "15m"}
		if !reflect.DeepEqual(reqs[0].Timeframes, expected) {
			t.Errorf("expected scalp timeframes %v, got %v", expected, reqs[0].Timeframes)
		}
	})

	t.Run("correctly detects exit modes", func(t *testing.T) {
		positions := []Position{
			&SimplePosition{Symbol: "BTCUSDT", Mode: "scalp", TakeProfit: true, StopLoss: true},
			&SimplePosition{Symbol: "ETHUSDT", Mode: "scalp", TrailingActive: true},
			&SimplePosition{Symbol: "SOLUSDT", Mode: "scalp", TakeProfit: true, TrailingActive: true},
		}

		reqs := GetPositionRequirements(positions)

		if len(reqs) != 3 {
			t.Fatalf("expected 3 requirements, got %d", len(reqs))
		}

		if reqs[0].ExitMode != ExitModeTPSL {
			t.Errorf("BTCUSDT: expected exit mode %s, got %s", ExitModeTPSL, reqs[0].ExitMode)
		}
		if reqs[1].ExitMode != ExitModeTrailing {
			t.Errorf("ETHUSDT: expected exit mode %s, got %s", ExitModeTrailing, reqs[1].ExitMode)
		}
		if reqs[2].ExitMode != ExitModeBoth {
			t.Errorf("SOLUSDT: expected exit mode %s, got %s", ExitModeBoth, reqs[2].ExitMode)
		}
	})
}

// ============================================================================
// TEST: CombineRequirements
// ============================================================================

func TestCombineRequirements(t *testing.T) {
	t.Run("combines strategy and position requirements", func(t *testing.T) {
		strategyReqs := &AggregatedRequirements{
			AllTimeframes:   []string{"15m", "1h"},
			AllDataFields:   []string{"ohlc", "volume", "taker_buy_volume"},
			TotalStrategies: 2,
		}

		positionReqs := []PositionRequirements{
			{Symbol: "BTCUSDT", Timeframes: []string{"5m", "15m"}, Mode: "scalp", ExitMode: ExitModeTPSL},
			{Symbol: "ETHUSDT", Timeframes: []string{"15m", "1h"}, Mode: "swing", ExitMode: ExitModeTrailing},
		}

		combined := CombineRequirements(strategyReqs, positionReqs)

		if combined == nil {
			t.Fatal("expected non-nil combined requirements")
		}
		if combined.StrategyCount != 2 {
			t.Errorf("expected strategy count 2, got %d", combined.StrategyCount)
		}
		if combined.PositionCount != 2 {
			t.Errorf("expected position count 2, got %d", combined.PositionCount)
		}
		if len(combined.AllSymbols) != 2 {
			t.Errorf("expected 2 symbols, got %d", len(combined.AllSymbols))
		}
	})

	t.Run("deduplicates timeframes across sources", func(t *testing.T) {
		strategyReqs := &AggregatedRequirements{
			AllTimeframes: []string{"15m", "1h"},
			AllDataFields: []string{"ohlc"},
		}

		positionReqs := []PositionRequirements{
			{Symbol: "BTCUSDT", Timeframes: []string{"5m", "15m"}, Mode: "scalp"}, // 15m is duplicate
		}

		combined := CombineRequirements(strategyReqs, positionReqs)

		// Should have 5m, 15m, 1h (3 unique)
		if len(combined.AllTimeframes) != 3 {
			t.Errorf("expected 3 unique timeframes, got %d: %v", len(combined.AllTimeframes), combined.AllTimeframes)
		}

		// Verify no duplicates
		tfSet := make(map[string]bool)
		for _, tf := range combined.AllTimeframes {
			if tfSet[tf] {
				t.Errorf("duplicate timeframe found: %s", tf)
			}
			tfSet[tf] = true
		}
	})

	t.Run("handles nil strategy requirements", func(t *testing.T) {
		positionReqs := []PositionRequirements{
			{Symbol: "BTCUSDT", Timeframes: []string{"5m", "15m"}, Mode: "scalp"},
		}

		combined := CombineRequirements(nil, positionReqs)

		if combined == nil {
			t.Fatal("expected non-nil result")
		}
		if combined.StrategyCount != 0 {
			t.Errorf("expected 0 strategies, got %d", combined.StrategyCount)
		}
		if combined.PositionCount != 1 {
			t.Errorf("expected 1 position, got %d", combined.PositionCount)
		}
	})

	t.Run("handles empty position requirements", func(t *testing.T) {
		strategyReqs := &AggregatedRequirements{
			AllTimeframes:   []string{"15m", "1h"},
			AllDataFields:   []string{"ohlc"},
			TotalStrategies: 1,
		}

		combined := CombineRequirements(strategyReqs, []PositionRequirements{})

		if combined == nil {
			t.Fatal("expected non-nil result")
		}
		if combined.StrategyCount != 1 {
			t.Errorf("expected 1 strategy, got %d", combined.StrategyCount)
		}
		if combined.PositionCount != 0 {
			t.Errorf("expected 0 positions, got %d", combined.PositionCount)
		}
	})

	t.Run("creates BySymbol entries for positions", func(t *testing.T) {
		positionReqs := []PositionRequirements{
			{Symbol: "BTCUSDT", Timeframes: []string{"5m", "15m"}, Mode: "scalp", ExitMode: ExitModeTPSL},
		}

		combined := CombineRequirements(nil, positionReqs)

		symReq, exists := combined.BySymbol["BTCUSDT"]
		if !exists {
			t.Fatal("expected BTCUSDT in BySymbol")
		}
		if symReq.Symbol != "BTCUSDT" {
			t.Errorf("expected symbol BTCUSDT, got %s", symReq.Symbol)
		}
		if symReq.Source != DataSourcePosition {
			t.Errorf("expected source %s, got %s", DataSourcePosition, symReq.Source)
		}
		if len(symReq.Positions) != 1 {
			t.Errorf("expected 1 position, got %d", len(symReq.Positions))
		}
	})

	t.Run("merges timeframes for same symbol from multiple positions", func(t *testing.T) {
		positionReqs := []PositionRequirements{
			{Symbol: "BTCUSDT", Timeframes: []string{"5m", "15m"}, Mode: "scalp"},
			{Symbol: "BTCUSDT", Timeframes: []string{"1h", "4h"}, Mode: "swing"},
		}

		combined := CombineRequirements(nil, positionReqs)

		symReq, exists := combined.BySymbol["BTCUSDT"]
		if !exists {
			t.Fatal("expected BTCUSDT in BySymbol")
		}

		// Should have all 4 timeframes merged
		if len(symReq.Timeframes) != 4 {
			t.Errorf("expected 4 merged timeframes, got %d: %v", len(symReq.Timeframes), symReq.Timeframes)
		}

		// Should have 2 positions
		if len(symReq.Positions) != 2 {
			t.Errorf("expected 2 positions, got %d", len(symReq.Positions))
		}
	})

	t.Run("includes default data fields for positions", func(t *testing.T) {
		positionReqs := []PositionRequirements{
			{Symbol: "BTCUSDT", Timeframes: []string{"5m"}, Mode: "scalp"},
		}

		combined := CombineRequirements(nil, positionReqs)

		// Should include ohlc and volume
		hasOHLC := false
		hasVolume := false
		for _, df := range combined.AllDataFields {
			if df == "ohlc" {
				hasOHLC = true
			}
			if df == "volume" {
				hasVolume = true
			}
		}
		if !hasOHLC {
			t.Error("expected 'ohlc' in data fields")
		}
		if !hasVolume {
			t.Error("expected 'volume' in data fields")
		}
	})
}

// ============================================================================
// TEST: AddSymbolFromStrategy
// ============================================================================

func TestAddSymbolFromStrategy(t *testing.T) {
	t.Run("adds new symbol from strategy", func(t *testing.T) {
		combined := &CombinedRequirements{
			AllSymbols: []string{},
			BySymbol:   make(map[string]*SymbolRequirements),
		}

		strategies := []StrategyRef{
			{Mode: "scalp", Strategy: "breakout", SubStrategy: "volume_imbalance"},
		}

		combined.AddSymbolFromStrategy("BTCUSDT", strategies, []string{"15m"}, []string{"ohlc"})

		if len(combined.AllSymbols) != 1 {
			t.Errorf("expected 1 symbol, got %d", len(combined.AllSymbols))
		}
		if combined.AllSymbols[0] != "BTCUSDT" {
			t.Errorf("expected BTCUSDT, got %s", combined.AllSymbols[0])
		}

		symReq, exists := combined.BySymbol["BTCUSDT"]
		if !exists {
			t.Fatal("expected BTCUSDT in BySymbol")
		}
		if symReq.Source != DataSourceStrategy {
			t.Errorf("expected source %s, got %s", DataSourceStrategy, symReq.Source)
		}
		if len(symReq.Strategies) != 1 {
			t.Errorf("expected 1 strategy, got %d", len(symReq.Strategies))
		}
	})

	t.Run("updates source to both when symbol exists from position", func(t *testing.T) {
		combined := &CombinedRequirements{
			AllSymbols: []string{"BTCUSDT"},
			BySymbol: map[string]*SymbolRequirements{
				"BTCUSDT": {
					Symbol:     "BTCUSDT",
					Timeframes: []string{"5m"},
					Source:     DataSourcePosition,
				},
			},
		}

		strategies := []StrategyRef{
			{Mode: "scalp", Strategy: "breakout", SubStrategy: "volume_imbalance"},
		}

		combined.AddSymbolFromStrategy("BTCUSDT", strategies, []string{"15m"}, []string{"ohlc"})

		symReq := combined.BySymbol["BTCUSDT"]
		if symReq.Source != DataSourceBoth {
			t.Errorf("expected source %s, got %s", DataSourceBoth, symReq.Source)
		}
	})

	t.Run("merges timeframes when adding to existing symbol", func(t *testing.T) {
		combined := &CombinedRequirements{
			AllSymbols: []string{"BTCUSDT"},
			BySymbol: map[string]*SymbolRequirements{
				"BTCUSDT": {
					Symbol:     "BTCUSDT",
					Timeframes: []string{"5m", "15m"},
					DataFields: []string{"ohlc"},
					Source:     DataSourcePosition,
				},
			},
		}

		combined.AddSymbolFromStrategy("BTCUSDT", []StrategyRef{}, []string{"15m", "1h"}, []string{"volume"})

		symReq := combined.BySymbol["BTCUSDT"]
		// Should have 5m, 15m, 1h (3 unique)
		if len(symReq.Timeframes) != 3 {
			t.Errorf("expected 3 timeframes, got %d: %v", len(symReq.Timeframes), symReq.Timeframes)
		}
		// Should have ohlc, volume (2 unique)
		if len(symReq.DataFields) != 2 {
			t.Errorf("expected 2 data fields, got %d: %v", len(symReq.DataFields), symReq.DataFields)
		}
	})

	t.Run("ignores empty symbol", func(t *testing.T) {
		combined := &CombinedRequirements{
			AllSymbols: []string{},
			BySymbol:   make(map[string]*SymbolRequirements),
		}

		combined.AddSymbolFromStrategy("", []StrategyRef{}, []string{"15m"}, []string{"ohlc"})

		if len(combined.AllSymbols) != 0 {
			t.Errorf("expected 0 symbols after adding empty, got %d", len(combined.AllSymbols))
		}
	})
}

// ============================================================================
// TEST: GetSymbolsForSource
// ============================================================================

func TestGetSymbolsForSource(t *testing.T) {
	t.Run("returns symbols for position source", func(t *testing.T) {
		combined := &CombinedRequirements{
			BySymbol: map[string]*SymbolRequirements{
				"BTCUSDT": {Symbol: "BTCUSDT", Source: DataSourcePosition},
				"ETHUSDT": {Symbol: "ETHUSDT", Source: DataSourceStrategy},
				"SOLUSDT": {Symbol: "SOLUSDT", Source: DataSourceBoth},
			},
		}

		symbols := combined.GetSymbolsForSource(DataSourcePosition)

		// Should return BTCUSDT (position) and SOLUSDT (both)
		if len(symbols) != 2 {
			t.Errorf("expected 2 symbols for position source, got %d: %v", len(symbols), symbols)
		}
	})

	t.Run("returns symbols for strategy source", func(t *testing.T) {
		combined := &CombinedRequirements{
			BySymbol: map[string]*SymbolRequirements{
				"BTCUSDT": {Symbol: "BTCUSDT", Source: DataSourcePosition},
				"ETHUSDT": {Symbol: "ETHUSDT", Source: DataSourceStrategy},
				"SOLUSDT": {Symbol: "SOLUSDT", Source: DataSourceBoth},
			},
		}

		symbols := combined.GetSymbolsForSource(DataSourceStrategy)

		// Should return ETHUSDT (strategy) and SOLUSDT (both)
		if len(symbols) != 2 {
			t.Errorf("expected 2 symbols for strategy source, got %d: %v", len(symbols), symbols)
		}
	})

	t.Run("returns all symbols for both source", func(t *testing.T) {
		combined := &CombinedRequirements{
			BySymbol: map[string]*SymbolRequirements{
				"BTCUSDT": {Symbol: "BTCUSDT", Source: DataSourcePosition},
				"ETHUSDT": {Symbol: "ETHUSDT", Source: DataSourceStrategy},
				"SOLUSDT": {Symbol: "SOLUSDT", Source: DataSourceBoth},
			},
		}

		symbols := combined.GetSymbolsForSource(DataSourceBoth)

		// Should return all 3 symbols
		if len(symbols) != 3 {
			t.Errorf("expected 3 symbols for both source, got %d: %v", len(symbols), symbols)
		}
	})

	t.Run("returns sorted symbols", func(t *testing.T) {
		combined := &CombinedRequirements{
			BySymbol: map[string]*SymbolRequirements{
				"SOLUSDT": {Symbol: "SOLUSDT", Source: DataSourcePosition},
				"BTCUSDT": {Symbol: "BTCUSDT", Source: DataSourcePosition},
				"ETHUSDT": {Symbol: "ETHUSDT", Source: DataSourcePosition},
			},
		}

		symbols := combined.GetSymbolsForSource(DataSourcePosition)

		expected := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
		if !reflect.DeepEqual(symbols, expected) {
			t.Errorf("expected sorted %v, got %v", expected, symbols)
		}
	})
}

// ============================================================================
// TEST: Integration - Position + Strategy Requirements
// ============================================================================

func TestIntegration_PositionAndStrategyRequirements(t *testing.T) {
	t.Run("complete flow with strategies and positions", func(t *testing.T) {
		// Simulate strategy requirements
		strategyReqs := &AggregatedRequirements{
			AllTimeframes:   []string{"15m", "1h", "4h"},
			AllDataFields:   []string{"ohlc", "volume", "taker_buy_volume"},
			TotalStrategies: 3,
			ByStrategy: []StrategyRequirements{
				{Mode: "scalp", Strategy: "breakout", SubStrategy: "volume_imbalance"},
				{Mode: "swing", Strategy: "trending", SubStrategy: "trend_following"},
				{Mode: "position", Strategy: "range", SubStrategy: "mean_reversion"},
			},
		}

		// Simulate open positions
		positions := []Position{
			&SimplePosition{Symbol: "BTCUSDT", Mode: "scalp", Side: "LONG", TakeProfit: true, StopLoss: true},
			&SimplePosition{Symbol: "ETHUSDT", Mode: "swing", Side: "SHORT", TrailingActive: true},
		}

		// Get position requirements
		positionReqs := GetPositionRequirements(positions)

		// Combine everything
		combined := CombineRequirements(strategyReqs, positionReqs)

		// Verify combined results
		if combined.StrategyCount != 3 {
			t.Errorf("expected 3 strategies, got %d", combined.StrategyCount)
		}
		if combined.PositionCount != 2 {
			t.Errorf("expected 2 positions, got %d", combined.PositionCount)
		}
		if len(combined.AllSymbols) != 2 {
			t.Errorf("expected 2 symbols from positions, got %d", len(combined.AllSymbols))
		}

		// Verify timeframes include both strategy and position needs
		timeframeSet := make(map[string]bool)
		for _, tf := range combined.AllTimeframes {
			timeframeSet[tf] = true
		}
		// Should have: 5m, 15m from scalp position; 15m, 1h from swing position; 15m, 1h, 4h from strategies
		if !timeframeSet["5m"] {
			t.Error("expected 5m timeframe from scalp position")
		}
		if !timeframeSet["15m"] {
			t.Error("expected 15m timeframe")
		}
		if !timeframeSet["1h"] {
			t.Error("expected 1h timeframe")
		}
		if !timeframeSet["4h"] {
			t.Error("expected 4h timeframe from strategy")
		}

		// Verify BTCUSDT has correct data
		btcReq := combined.BySymbol["BTCUSDT"]
		if btcReq == nil {
			t.Fatal("expected BTCUSDT in BySymbol")
		}
		if btcReq.Source != DataSourcePosition {
			t.Errorf("expected BTCUSDT source %s, got %s", DataSourcePosition, btcReq.Source)
		}
		if len(btcReq.Positions) != 1 {
			t.Errorf("expected 1 position for BTCUSDT, got %d", len(btcReq.Positions))
		}
		if btcReq.Positions[0].ExitMode != ExitModeTPSL {
			t.Errorf("expected BTCUSDT exit mode %s, got %s", ExitModeTPSL, btcReq.Positions[0].ExitMode)
		}
	})

	t.Run("trading OFF - positions only", func(t *testing.T) {
		// When trading is OFF, only position requirements matter
		positions := []Position{
			&SimplePosition{Symbol: "BTCUSDT", Mode: "scalp", Side: "LONG", StopLoss: true},
		}

		positionReqs := GetPositionRequirements(positions)
		combined := CombineRequirements(nil, positionReqs)

		if combined.StrategyCount != 0 {
			t.Errorf("expected 0 strategies (trading OFF), got %d", combined.StrategyCount)
		}
		if combined.PositionCount != 1 {
			t.Errorf("expected 1 position, got %d", combined.PositionCount)
		}
		if len(combined.AllSymbols) != 1 {
			t.Errorf("expected 1 symbol, got %d", len(combined.AllSymbols))
		}
	})
}

// ============================================================================
// TEST: GiniePositionAdapter (Story 14.3)
// ============================================================================

// mockGiniePositionSource implements GiniePositionSource for testing
type mockGiniePositionSource struct {
	symbol       string
	mode         string
	side         string
	tpPrice      float64
	slPrice      float64
	trailingStop bool
}

func (m *mockGiniePositionSource) GetSymbol() string           { return m.symbol }
func (m *mockGiniePositionSource) GetMode() string             { return m.mode }
func (m *mockGiniePositionSource) GetSide() string             { return m.side }
func (m *mockGiniePositionSource) GetTakeProfitPrice() float64 { return m.tpPrice }
func (m *mockGiniePositionSource) GetStopLossPrice() float64   { return m.slPrice }
func (m *mockGiniePositionSource) IsTrailingStopActive() bool  { return m.trailingStop }

func TestGiniePositionAdapter(t *testing.T) {
	t.Run("implements Position interface", func(t *testing.T) {
		source := &mockGiniePositionSource{
			symbol:       "BTCUSDT",
			mode:         "scalp",
			side:         "LONG",
			tpPrice:      45000.0,
			slPrice:      40000.0,
			trailingStop: true,
		}

		adapter := NewGiniePositionAdapter(source)

		// Verify it implements Position interface
		var _ Position = adapter

		if adapter.GetSymbol() != "BTCUSDT" {
			t.Errorf("expected symbol BTCUSDT, got %s", adapter.GetSymbol())
		}
		if adapter.GetMode() != "scalp" {
			t.Errorf("expected mode scalp, got %s", adapter.GetMode())
		}
		if adapter.GetSide() != "LONG" {
			t.Errorf("expected side LONG, got %s", adapter.GetSide())
		}
		if !adapter.HasTakeProfit() {
			t.Error("expected HasTakeProfit to be true")
		}
		if !adapter.HasStopLoss() {
			t.Error("expected HasStopLoss to be true")
		}
		if !adapter.IsTrailingActive() {
			t.Error("expected IsTrailingActive to be true")
		}
	})

	t.Run("handles nil adapter gracefully", func(t *testing.T) {
		var adapter *GiniePositionAdapter

		if adapter.GetSymbol() != "" {
			t.Error("expected empty symbol for nil adapter")
		}
		if adapter.GetMode() != "" {
			t.Error("expected empty mode for nil adapter")
		}
		if adapter.GetSide() != "" {
			t.Error("expected empty side for nil adapter")
		}
		if adapter.HasTakeProfit() {
			t.Error("expected false for nil adapter")
		}
		if adapter.HasStopLoss() {
			t.Error("expected false for nil adapter")
		}
		if adapter.IsTrailingActive() {
			t.Error("expected false for nil adapter")
		}
	})

	t.Run("handles nil source gracefully", func(t *testing.T) {
		adapter := NewGiniePositionAdapter(nil)

		if adapter.GetSymbol() != "" {
			t.Error("expected empty symbol for nil source")
		}
		if adapter.HasTakeProfit() {
			t.Error("expected false for nil source")
		}
	})

	t.Run("TP/SL detection based on price", func(t *testing.T) {
		// No TP set (price = 0)
		source1 := &mockGiniePositionSource{
			symbol:  "BTCUSDT",
			tpPrice: 0,
			slPrice: 40000.0,
		}
		adapter1 := NewGiniePositionAdapter(source1)
		if adapter1.HasTakeProfit() {
			t.Error("expected HasTakeProfit false when price is 0")
		}
		if !adapter1.HasStopLoss() {
			t.Error("expected HasStopLoss true when price > 0")
		}

		// No SL set (price = 0)
		source2 := &mockGiniePositionSource{
			symbol:  "BTCUSDT",
			tpPrice: 45000.0,
			slPrice: 0,
		}
		adapter2 := NewGiniePositionAdapter(source2)
		if !adapter2.HasTakeProfit() {
			t.Error("expected HasTakeProfit true when price > 0")
		}
		if adapter2.HasStopLoss() {
			t.Error("expected HasStopLoss false when price is 0")
		}
	})
}

func TestAdaptGiniePositions(t *testing.T) {
	t.Run("converts slice of sources to positions", func(t *testing.T) {
		sources := []GiniePositionSource{
			&mockGiniePositionSource{symbol: "BTCUSDT", mode: "scalp"},
			&mockGiniePositionSource{symbol: "ETHUSDT", mode: "swing"},
		}

		positions := AdaptGiniePositions(sources)

		if len(positions) != 2 {
			t.Errorf("expected 2 positions, got %d", len(positions))
		}
		if positions[0].GetSymbol() != "BTCUSDT" {
			t.Errorf("expected first symbol BTCUSDT, got %s", positions[0].GetSymbol())
		}
		if positions[1].GetSymbol() != "ETHUSDT" {
			t.Errorf("expected second symbol ETHUSDT, got %s", positions[1].GetSymbol())
		}
	})

	t.Run("skips nil sources", func(t *testing.T) {
		sources := []GiniePositionSource{
			&mockGiniePositionSource{symbol: "BTCUSDT"},
			nil,
			&mockGiniePositionSource{symbol: "ETHUSDT"},
		}

		positions := AdaptGiniePositions(sources)

		if len(positions) != 2 {
			t.Errorf("expected 2 positions (skipping nil), got %d", len(positions))
		}
	})

	t.Run("handles empty slice", func(t *testing.T) {
		positions := AdaptGiniePositions([]GiniePositionSource{})
		if len(positions) != 0 {
			t.Errorf("expected 0 positions, got %d", len(positions))
		}
	})
}

// ============================================================================
// TEST: Thread Safety (Story 14.2)
// ============================================================================

func TestStrategyRegistry_ThreadSafety(t *testing.T) {
	t.Run("concurrent reads are safe", func(t *testing.T) {
		done := make(chan bool, 10)

		// Spawn multiple goroutines reading from registry
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				for j := 0; j < 100; j++ {
					_ = GetRegisteredStrategies()
					_ = IsStrategyRegistered("ravindra_volume_imbalance")
					_ = GetStrategyConfig("classic_breakout")
					_ = GetStrategyRequirements("scalp", "breakout", "ravindra_volume_imbalance")
				}
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("returned slices are safe to modify", func(t *testing.T) {
		// Get requirements and modify the returned slices
		req := GetStrategyRequirements("scalp", "breakout", "ravindra_volume_imbalance")

		// Modify returned timeframes
		if len(req.Timeframes) > 0 {
			req.Timeframes[0] = "MODIFIED"
		}

		// Get fresh requirements and verify original is unchanged
		req2 := GetStrategyRequirements("scalp", "breakout", "ravindra_volume_imbalance")
		if len(req2.Timeframes) > 0 && req2.Timeframes[0] == "MODIFIED" {
			t.Error("modifying returned slice should not affect registry")
		}
	})

	t.Run("returned config is safe to modify", func(t *testing.T) {
		config := GetStrategyConfig("ravindra_volume_imbalance")
		if config == nil {
			t.Skip("ravindra_volume_imbalance not registered")
		}

		// Modify returned config
		config.DataFields = append(config.DataFields, "MODIFIED")

		// Get fresh config and verify original is unchanged
		config2 := GetStrategyConfig("ravindra_volume_imbalance")
		for _, df := range config2.DataFields {
			if df == "MODIFIED" {
				t.Error("modifying returned config should not affect registry")
			}
		}
	})
}
