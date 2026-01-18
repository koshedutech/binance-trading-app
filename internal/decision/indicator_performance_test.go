// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.14: Indicator Performance Tracker Tests
package decision

import (
	"context"
	"testing"
	"time"
)

// TestHashIndicatorList tests the indicator combination hashing function.
func TestHashIndicatorList(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string // Just checking it's 8 chars and deterministic
	}{
		{
			name:     "empty list",
			input:    []string{},
			expected: "00000000",
		},
		{
			name:     "single indicator",
			input:    []string{"adx"},
			expected: "", // Will be computed
		},
		{
			name:     "multiple indicators - order independent",
			input:    []string{"atr", "adx", "volume"},
			expected: "", // Will be computed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashIndicatorList(tt.input)

			// Check length is 8
			if len(result) != 8 {
				t.Errorf("hashIndicatorList() = %v, want length 8", result)
			}

			// Check empty list returns "00000000"
			if len(tt.input) == 0 && result != "00000000" {
				t.Errorf("hashIndicatorList() for empty list = %v, want 00000000", result)
			}

			// Check determinism - same input produces same output
			result2 := hashIndicatorList(tt.input)
			if result != result2 {
				t.Errorf("hashIndicatorList() not deterministic: %v != %v", result, result2)
			}
		})
	}

	// Test order independence
	t.Run("order independence", func(t *testing.T) {
		list1 := []string{"adx", "atr", "volume"}
		list2 := []string{"volume", "adx", "atr"}

		hash1 := hashIndicatorList(list1)
		hash2 := hashIndicatorList(list2)

		if hash1 != hash2 {
			t.Errorf("hashIndicatorList() not order-independent: %v != %v", hash1, hash2)
		}
	})
}

// TestCalculatePearsonCorrelation tests the correlation calculation.
func TestCalculatePearsonCorrelation(t *testing.T) {
	tests := []struct {
		name      string
		x         []float64
		y         []float64
		expected  float64
		tolerance float64
	}{
		{
			name:      "perfect positive correlation",
			x:         []float64{1, 2, 3, 4, 5},
			y:         []float64{2, 4, 6, 8, 10},
			expected:  1.0,
			tolerance: 0.001,
		},
		{
			name:      "perfect negative correlation",
			x:         []float64{1, 2, 3, 4, 5},
			y:         []float64{10, 8, 6, 4, 2},
			expected:  -1.0,
			tolerance: 0.001,
		},
		{
			name:      "no correlation",
			x:         []float64{1, 2, 3, 4, 5},
			y:         []float64{3, 3, 3, 3, 3}, // Constant - no variance
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "mismatched lengths",
			x:         []float64{1, 2, 3},
			y:         []float64{1, 2},
			expected:  0.0, // Should return 0 for invalid input
			tolerance: 0.001,
		},
		{
			name:      "too few points",
			x:         []float64{1},
			y:         []float64{2},
			expected:  0.0, // Need at least 2 points
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePearsonCorrelation(tt.x, tt.y)

			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}

			if diff > tt.tolerance {
				t.Errorf("calculatePearsonCorrelation() = %v, want %v (tolerance %v)",
					result, tt.expected, tt.tolerance)
			}
		})
	}
}

// TestIndicatorCombinationStats tests the stats recording.
func TestIndicatorCombinationStats(t *testing.T) {
	stats := &IndicatorCombinationStats{
		CombinationHash: "12345678",
		Indicators:      []string{"adx", "atr"},
		Strategy:        "trend_following",
		CreatedAt:       time.Now(),
	}

	// Record first trade - win
	stats.RecordTrade(true, 75, 2.5)

	if stats.TradeCount != 1 {
		t.Errorf("TradeCount = %d, want 1", stats.TradeCount)
	}
	if stats.WinCount != 1 {
		t.Errorf("WinCount = %d, want 1", stats.WinCount)
	}
	if stats.WinRate != 100.0 {
		t.Errorf("WinRate = %f, want 100.0", stats.WinRate)
	}
	if stats.AvgPnLPercent != 2.5 {
		t.Errorf("AvgPnLPercent = %f, want 2.5", stats.AvgPnLPercent)
	}
	if stats.AvgEntryScore != 75.0 {
		t.Errorf("AvgEntryScore = %f, want 75.0", stats.AvgEntryScore)
	}

	// Record second trade - loss
	stats.RecordTrade(false, 65, -1.0)

	if stats.TradeCount != 2 {
		t.Errorf("TradeCount = %d, want 2", stats.TradeCount)
	}
	if stats.WinCount != 1 {
		t.Errorf("WinCount = %d, want 1", stats.WinCount)
	}
	if stats.WinRate != 50.0 {
		t.Errorf("WinRate = %f, want 50.0", stats.WinRate)
	}
	expectedAvgPnL := (2.5 - 1.0) / 2.0 // 0.75
	if stats.AvgPnLPercent != expectedAvgPnL {
		t.Errorf("AvgPnLPercent = %f, want %f", stats.AvgPnLPercent, expectedAvgPnL)
	}
	expectedAvgScore := (75.0 + 65.0) / 2.0 // 70.0
	if stats.AvgEntryScore != expectedAvgScore {
		t.Errorf("AvgEntryScore = %f, want %f", stats.AvgEntryScore, expectedAvgScore)
	}
}

// TestNewIndicatorPerformanceTracker tests tracker creation.
func TestNewIndicatorPerformanceTracker(t *testing.T) {
	// Test creation without cache (nil cache is valid for testing)
	tracker := NewIndicatorPerformanceTracker(nil)

	if tracker == nil {
		t.Error("NewIndicatorPerformanceTracker() returned nil")
	}

	if tracker.pendingTrades == nil {
		t.Error("pendingTrades map not initialized")
	}

	if tracker.ttl != DefaultPerformanceTTL {
		t.Errorf("ttl = %v, want %v", tracker.ttl, DefaultPerformanceTTL)
	}
}

// TestRecordTradeEntry tests entry recording without cache.
func TestRecordTradeEntry(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil) // No cache for unit test

	snapshot := &TradeIndicatorSnapshot{
		TradeID:  "test-trade-1",
		UserID:   1,
		Symbol:   "BTCUSDT",
		Strategy: "trend_following",
		Indicators: map[string]float64{
			"adx": 35.0,
			"atr": 1.5,
		},
		EntryScore:   80,
		EntryTime:    time.Now(),
		MarketRegime: "BULLISH",
		Side:         "LONG",
	}

	ctx := context.Background()
	err := tracker.RecordTradeEntry(ctx, snapshot)

	if err != nil {
		t.Errorf("RecordTradeEntry() error = %v", err)
	}

	// Verify it's in pending trades
	if tracker.GetPendingTradeCount() != 1 {
		t.Errorf("GetPendingTradeCount() = %d, want 1", tracker.GetPendingTradeCount())
	}
}

// TestRecordTradeEntryValidation tests validation of entry parameters.
func TestRecordTradeEntryValidation(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	tests := []struct {
		name      string
		snapshot  *TradeIndicatorSnapshot
		expectErr bool
	}{
		{
			name:      "nil snapshot",
			snapshot:  nil,
			expectErr: true,
		},
		{
			name: "empty trade ID",
			snapshot: &TradeIndicatorSnapshot{
				TradeID: "",
				UserID:  1,
			},
			expectErr: true,
		},
		{
			name: "invalid user ID",
			snapshot: &TradeIndicatorSnapshot{
				TradeID: "test-1",
				UserID:  0,
			},
			expectErr: true,
		},
		{
			name: "valid snapshot",
			snapshot: &TradeIndicatorSnapshot{
				TradeID: "test-1",
				UserID:  1,
				Symbol:  "BTCUSDT",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tracker.RecordTradeEntry(ctx, tt.snapshot)
			hasErr := err != nil

			if hasErr != tt.expectErr {
				t.Errorf("RecordTradeEntry() error = %v, expectErr = %v", err, tt.expectErr)
			}
		})
	}
}

// TestRecordTradeExit tests exit recording and correlation with entry.
func TestRecordTradeExit(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	// First record an entry
	snapshot := &TradeIndicatorSnapshot{
		TradeID:  "test-trade-2",
		UserID:   1,
		Symbol:   "ETHUSDT",
		Strategy: "scalping",
		Indicators: map[string]float64{
			"adx": 25.0,
		},
		EntryScore: 70,
		EntryTime:  time.Now(),
		Side:       "SHORT",
	}
	err := tracker.RecordTradeEntry(ctx, snapshot)
	if err != nil {
		t.Fatalf("RecordTradeEntry() failed: %v", err)
	}

	// Then record the exit
	outcome := &TradeOutcome{
		TradeID:    "test-trade-2",
		IsWin:      true,
		PnLPercent: 1.5,
		ExitTime:   time.Now(),
		ExitReason: "TP_HIT",
	}

	err = tracker.RecordTradeExit(ctx, outcome)
	if err != nil {
		t.Errorf("RecordTradeExit() error = %v", err)
	}

	// Verify the pending trade was removed
	if tracker.GetPendingTradeCount() != 0 {
		t.Errorf("GetPendingTradeCount() = %d, want 0", tracker.GetPendingTradeCount())
	}
}

// TestRecordTradeExitUnknownTrade tests exit for trade not in pending.
func TestRecordTradeExitUnknownTrade(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	outcome := &TradeOutcome{
		TradeID:    "unknown-trade",
		IsWin:      true,
		PnLPercent: 1.0,
		ExitTime:   time.Now(),
		ExitReason: "MANUAL",
	}

	// Should not error - just logs and continues
	err := tracker.RecordTradeExit(ctx, outcome)
	if err != nil {
		t.Errorf("RecordTradeExit() for unknown trade error = %v, want nil", err)
	}
}

// TestClearPendingTrades tests clearing pending trades.
func TestClearPendingTrades(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	// Add some trades
	for i := 1; i <= 3; i++ {
		snapshot := &TradeIndicatorSnapshot{
			TradeID: "test-" + string(rune('0'+i)),
			UserID:  1,
			Symbol:  "BTCUSDT",
		}
		_ = tracker.RecordTradeEntry(ctx, snapshot)
	}

	if tracker.GetPendingTradeCount() != 3 {
		t.Errorf("GetPendingTradeCount() = %d, want 3", tracker.GetPendingTradeCount())
	}

	// Clear
	tracker.ClearPendingTrades()

	if tracker.GetPendingTradeCount() != 0 {
		t.Errorf("GetPendingTradeCount() after clear = %d, want 0", tracker.GetPendingTradeCount())
	}
}

// TestAutoOptimizeFlag tests the auto-optimize flag.
func TestAutoOptimizeFlag(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)

	// Default should be false
	if tracker.IsAutoOptimizeEnabled() {
		t.Error("IsAutoOptimizeEnabled() = true, want false by default")
	}

	// Enable
	tracker.SetAutoOptimize(true)
	if !tracker.IsAutoOptimizeEnabled() {
		t.Error("IsAutoOptimizeEnabled() = false after SetAutoOptimize(true)")
	}

	// Disable
	tracker.SetAutoOptimize(false)
	if tracker.IsAutoOptimizeEnabled() {
		t.Error("IsAutoOptimizeEnabled() = true after SetAutoOptimize(false)")
	}
}

// TestRecommendationType tests recommendation type constants.
func TestRecommendationType(t *testing.T) {
	types := []RecommendationType{
		RecommendationAddIndicator,
		RecommendationRemoveIndicator,
		RecommendationAdjustWeight,
		RecommendationChangeStrategy,
		RecommendationKeepCurrent,
	}

	for _, rt := range types {
		if rt == "" {
			t.Error("RecommendationType should not be empty")
		}
	}
}

// TestAnalyzeIndicatorPerformanceEmptyData tests recommendations with no data.
func TestAnalyzeIndicatorPerformanceEmptyData(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)

	correlations := make(map[string]*IndicatorCorrelation)
	combinations := []IndicatorCombinationStats{}

	recommendations := tracker.analyzeIndicatorPerformance(correlations, combinations, 20)

	// Should return at least one recommendation
	if len(recommendations) == 0 {
		t.Error("analyzeIndicatorPerformance() returned no recommendations")
	}

	// The default should be KEEP_CURRENT
	if recommendations[0].Type != RecommendationKeepCurrent {
		t.Errorf("default recommendation Type = %s, want %s",
			recommendations[0].Type, RecommendationKeepCurrent)
	}
}

// TestAnalyzeIndicatorPerformanceNegativeCorrelation tests detection of poor indicators.
func TestAnalyzeIndicatorPerformanceNegativeCorrelation(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)

	// Create a correlation with negative win correlation
	correlations := map[string]*IndicatorCorrelation{
		"bad_indicator": {
			IndicatorName:  "bad_indicator",
			WinCorrelation: -0.5, // Strong negative correlation
			SampleSize:     50,
		},
	}
	combinations := []IndicatorCombinationStats{}

	recommendations := tracker.analyzeIndicatorPerformance(correlations, combinations, 20)

	// Should suggest removing the indicator
	found := false
	for _, rec := range recommendations {
		if rec.Type == RecommendationRemoveIndicator && rec.Indicator == "bad_indicator" {
			found = true
			break
		}
	}

	if !found {
		t.Error("analyzeIndicatorPerformance() did not recommend removing bad indicator")
	}
}

// TestAnalyzeIndicatorPerformancePositiveCorrelation tests detection of good indicators.
func TestAnalyzeIndicatorPerformancePositiveCorrelation(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)

	// Create a correlation with positive win correlation
	correlations := map[string]*IndicatorCorrelation{
		"good_indicator": {
			IndicatorName:  "good_indicator",
			WinCorrelation: 0.6, // Strong positive correlation
			SampleSize:     50,
		},
	}
	combinations := []IndicatorCombinationStats{}

	recommendations := tracker.analyzeIndicatorPerformance(correlations, combinations, 20)

	// Should suggest adjusting weight
	found := false
	for _, rec := range recommendations {
		if rec.Type == RecommendationAdjustWeight && rec.Indicator == "good_indicator" {
			found = true
			break
		}
	}

	if !found {
		t.Error("analyzeIndicatorPerformance() did not recommend adjusting good indicator weight")
	}
}

// TestPriorityFromCorrelation tests priority assignment.
func TestPriorityFromCorrelation(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)

	tests := []struct {
		corr     float64
		expected string
	}{
		{0.8, "HIGH"},
		{-0.7, "HIGH"},
		{0.5, "MEDIUM"},
		{-0.45, "MEDIUM"},
		{0.2, "LOW"},
		{-0.1, "LOW"},
	}

	for _, tt := range tests {
		result := tracker.priorityFromCorrelation(tt.corr)
		if result != tt.expected {
			t.Errorf("priorityFromCorrelation(%f) = %s, want %s", tt.corr, result, tt.expected)
		}
	}
}

// TestConfidenceFromSampleSize tests confidence calculation.
func TestConfidenceFromSampleSize(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)

	tests := []struct {
		size     int
		expected float64
	}{
		{100, 0.9},
		{50, 0.7},
		{20, 0.5},
		{10, 0.3},
		{5, 0.3},
	}

	for _, tt := range tests {
		result := tracker.confidenceFromSampleSize(tt.size)
		if result != tt.expected {
			t.Errorf("confidenceFromSampleSize(%d) = %f, want %f", tt.size, result, tt.expected)
		}
	}
}

// TestGetPerformanceStatsNoCache tests GetPerformanceStats with nil cache.
func TestGetPerformanceStatsNoCache(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	stats, err := tracker.GetPerformanceStats(ctx, 1, "trend_following")

	// Should return error since cache is nil
	if err == nil {
		t.Error("GetPerformanceStats() with nil cache should return error")
	}

	if stats != nil {
		t.Error("GetPerformanceStats() with nil cache should return nil stats")
	}
}

// TestGetIndicatorCorrelationsNoCache tests GetIndicatorCorrelations with nil cache.
func TestGetIndicatorCorrelationsNoCache(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	correlations, err := tracker.GetIndicatorCorrelations(ctx, 1, "trend_following")

	// Should return error since cache is nil
	if err == nil {
		t.Error("GetIndicatorCorrelations() with nil cache should return error")
	}

	if correlations != nil {
		t.Error("GetIndicatorCorrelations() with nil cache should return nil")
	}
}

// TestGetTopPerformingCombinationsNoCache tests GetTopPerformingCombinations with nil cache.
func TestGetTopPerformingCombinationsNoCache(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	combinations, err := tracker.GetTopPerformingCombinations(ctx, 1, "trend_following", 10)

	// Should return error since cache is nil
	if err == nil {
		t.Error("GetTopPerformingCombinations() with nil cache should return error")
	}

	if combinations != nil {
		t.Error("GetTopPerformingCombinations() with nil cache should return nil")
	}
}

// TestGetRecommendationsNoCache tests GetRecommendations with nil cache.
func TestGetRecommendationsNoCache(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	recommendations, err := tracker.GetRecommendations(ctx, 1, "trend_following")

	// Should return error since cache is nil
	if err == nil {
		t.Error("GetRecommendations() with nil cache should return error")
	}

	if recommendations != nil {
		t.Error("GetRecommendations() with nil cache should return nil")
	}
}

// TestUpdateCombinationStatsNoCache tests updateCombinationStats with nil cache.
func TestUpdateCombinationStatsNoCache(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	snapshot := &TradeIndicatorSnapshot{
		TradeID:    "test-1",
		UserID:     1,
		Symbol:     "BTCUSDT",
		Strategy:   "trend_following",
		Indicators: map[string]float64{"adx": 35.0},
	}
	outcome := &TradeOutcome{
		TradeID:    "test-1",
		IsWin:      true,
		PnLPercent: 2.5,
	}

	err := tracker.updateCombinationStats(ctx, snapshot, outcome)
	if err == nil {
		t.Error("updateCombinationStats() with nil cache should return error")
	}
}

// TestStoreCompletedTradeNoCache tests storeCompletedTrade with nil cache.
func TestStoreCompletedTradeNoCache(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	trade := &CompletedTrade{
		Snapshot: &TradeIndicatorSnapshot{
			TradeID:  "test-1",
			UserID:   1,
			Strategy: "trend_following",
		},
		Outcome: &TradeOutcome{
			TradeID: "test-1",
			IsWin:   true,
		},
	}

	err := tracker.storeCompletedTrade(ctx, trade)
	if err == nil {
		t.Error("storeCompletedTrade() with nil cache should return error")
	}
}

// TestFindSnapshotInRedisNoCache tests findSnapshotInRedis with nil cache.
func TestFindSnapshotInRedisNoCache(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	snapshot := tracker.findSnapshotInRedis(ctx, "test-trade-id")
	if snapshot != nil {
		t.Error("findSnapshotInRedis() with nil cache should return nil")
	}
}

// TestRecordTradeExitWithNilOutcome tests RecordTradeExit validation.
func TestRecordTradeExitWithNilOutcome(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	err := tracker.RecordTradeExit(ctx, nil)
	if err == nil {
		t.Error("RecordTradeExit() with nil outcome should return error")
	}
}

// TestRecordTradeExitWithEmptyTradeID tests RecordTradeExit validation.
func TestRecordTradeExitWithEmptyTradeID(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)
	ctx := context.Background()

	outcome := &TradeOutcome{
		TradeID: "",
		IsWin:   true,
	}

	err := tracker.RecordTradeExit(ctx, outcome)
	if err == nil {
		t.Error("RecordTradeExit() with empty trade_id should return error")
	}
}

// TestAnalyzeIndicatorPerformanceTopCombinations tests recommendations from top combinations.
func TestAnalyzeIndicatorPerformanceTopCombinations(t *testing.T) {
	tracker := NewIndicatorPerformanceTracker(nil)

	correlations := make(map[string]*IndicatorCorrelation)
	combinations := []IndicatorCombinationStats{
		{
			CombinationHash: "12345678",
			Indicators:      []string{"adx", "atr"},
			Strategy:        "trend_following",
			TradeCount:      30,
			WinRate:         65.0,
		},
	}

	recommendations := tracker.analyzeIndicatorPerformance(correlations, combinations, 20)

	// Should have at least one recommendation about the top combination
	found := false
	for _, rec := range recommendations {
		if rec.Type == RecommendationKeepCurrent && rec.BasedOnTrades == 30 {
			found = true
			break
		}
	}

	if !found {
		t.Error("analyzeIndicatorPerformance() should recommend keeping current for good win rate")
	}
}
