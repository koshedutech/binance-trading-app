// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"math"
	"testing"
)

func TestNewIndicatorPerformance(t *testing.T) {
	indicators := []string{"rsi", "macd", "ema_cross"}
	perf := NewIndicatorPerformance(indicators, SegmentMomentum)

	// Check that indicators are sorted
	if len(perf.Indicators) != 3 {
		t.Errorf("Expected 3 indicators, got %d", len(perf.Indicators))
	}
	if perf.Indicators[0] != "ema_cross" {
		t.Errorf("Expected first indicator to be 'ema_cross', got '%s'", perf.Indicators[0])
	}
	if perf.Indicators[1] != "macd" {
		t.Errorf("Expected second indicator to be 'macd', got '%s'", perf.Indicators[1])
	}
	if perf.Indicators[2] != "rsi" {
		t.Errorf("Expected third indicator to be 'rsi', got '%s'", perf.Indicators[2])
	}

	// Check initial values
	if perf.TradeCount != 0 {
		t.Errorf("Expected TradeCount 0, got %d", perf.TradeCount)
	}
	if perf.WinCount != 0 {
		t.Errorf("Expected WinCount 0, got %d", perf.WinCount)
	}
	if perf.WinRate != 0 {
		t.Errorf("Expected WinRate 0, got %f", perf.WinRate)
	}
	if perf.Segment != SegmentMomentum {
		t.Errorf("Expected segment SegmentMomentum, got %s", perf.Segment)
	}
	if perf.IndicatorHash == "" {
		t.Error("Expected non-empty IndicatorHash")
	}
}

func TestIndicatorPerformance_RecordTrade(t *testing.T) {
	perf := NewIndicatorPerformance([]string{"rsi"}, SegmentMomentum)

	// Record first trade (win)
	perf.RecordTrade(true, 75.0, 100.0)

	if perf.TradeCount != 1 {
		t.Errorf("Expected TradeCount 1, got %d", perf.TradeCount)
	}
	if perf.WinCount != 1 {
		t.Errorf("Expected WinCount 1, got %d", perf.WinCount)
	}
	if perf.WinRate != 100.0 {
		t.Errorf("Expected WinRate 100.0, got %f", perf.WinRate)
	}
	if perf.AvgScore != 75.0 {
		t.Errorf("Expected AvgScore 75.0, got %f", perf.AvgScore)
	}
	if perf.TotalPnL != 100.0 {
		t.Errorf("Expected TotalPnL 100.0, got %f", perf.TotalPnL)
	}
	if perf.AvgPnL != 100.0 {
		t.Errorf("Expected AvgPnL 100.0, got %f", perf.AvgPnL)
	}

	// Record second trade (loss)
	perf.RecordTrade(false, 50.0, -50.0)

	if perf.TradeCount != 2 {
		t.Errorf("Expected TradeCount 2, got %d", perf.TradeCount)
	}
	if perf.WinCount != 1 {
		t.Errorf("Expected WinCount 1, got %d", perf.WinCount)
	}
	if perf.WinRate != 50.0 {
		t.Errorf("Expected WinRate 50.0, got %f", perf.WinRate)
	}
	// Average score = (75 + 50) / 2 = 62.5
	if math.Abs(perf.AvgScore-62.5) > 0.01 {
		t.Errorf("Expected AvgScore 62.5, got %f", perf.AvgScore)
	}
	if perf.TotalPnL != 50.0 {
		t.Errorf("Expected TotalPnL 50.0, got %f", perf.TotalPnL)
	}
	if perf.AvgPnL != 25.0 {
		t.Errorf("Expected AvgPnL 25.0, got %f", perf.AvgPnL)
	}
}

func TestIndicatorPerformance_Clone(t *testing.T) {
	perf := NewIndicatorPerformance([]string{"rsi", "macd"}, SegmentMomentum)
	perf.RecordTrade(true, 80.0, 150.0)
	perf.RecordTrade(false, 60.0, -50.0)

	clone := perf.Clone()

	// Verify clone has same values
	if clone.TradeCount != perf.TradeCount {
		t.Errorf("Clone TradeCount mismatch: got %d, want %d", clone.TradeCount, perf.TradeCount)
	}
	if clone.WinCount != perf.WinCount {
		t.Errorf("Clone WinCount mismatch: got %d, want %d", clone.WinCount, perf.WinCount)
	}
	if clone.IndicatorHash != perf.IndicatorHash {
		t.Errorf("Clone IndicatorHash mismatch")
	}

	// Verify clone is independent
	clone.RecordTrade(true, 90.0, 200.0)
	if clone.TradeCount == perf.TradeCount {
		t.Error("Clone should be independent of original")
	}
}

func TestHashIndicators(t *testing.T) {
	// Same indicators in different order should produce same hash
	hash1 := hashIndicators([]string{"rsi", "macd", "ema"})
	hash2 := hashIndicators([]string{"ema", "rsi", "macd"})
	hash3 := hashIndicators([]string{"macd", "ema", "rsi"})

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Same indicators in different order should produce same hash")
	}

	// Different indicators should produce different hash
	hash4 := hashIndicators([]string{"rsi", "atr"})
	if hash1 == hash4 {
		t.Error("Different indicators should produce different hashes")
	}

	// Hash should be 16 characters
	if len(hash1) != 16 {
		t.Errorf("Expected hash length 16, got %d", len(hash1))
	}
}

func TestNewPerformanceTracker(t *testing.T) {
	// Test with nil cache (memory-only mode)
	tracker := NewPerformanceTracker(nil)
	if tracker == nil {
		t.Error("Expected non-nil tracker")
	}
	if tracker.ttl != DefaultPerfTTL {
		t.Errorf("Expected default TTL %v, got %v", DefaultPerfTTL, tracker.ttl)
	}
}

func TestPerformanceStats(t *testing.T) {
	stats := &PerformanceStats{
		TotalCombinations: 5,
		BySegment:         make(map[IndicatorSegment]SegmentStats),
	}

	stats.BySegment[SegmentTrend] = SegmentStats{
		CombinationCount: 2,
		TotalTrades:      10,
		TotalWins:        6,
		OverallWinRate:   60.0,
		TotalPnL:         500.0,
		BestWinRate:      80.0,
		BestCombination:  []string{"ema_cross", "macd"},
	}

	if stats.TotalCombinations != 5 {
		t.Errorf("Expected TotalCombinations 5, got %d", stats.TotalCombinations)
	}

	trendStats := stats.BySegment[SegmentTrend]
	if trendStats.CombinationCount != 2 {
		t.Errorf("Expected CombinationCount 2, got %d", trendStats.CombinationCount)
	}
	if trendStats.OverallWinRate != 60.0 {
		t.Errorf("Expected OverallWinRate 60.0, got %f", trendStats.OverallWinRate)
	}
}
