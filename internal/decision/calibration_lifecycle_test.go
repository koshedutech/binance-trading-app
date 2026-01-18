// Package decision provides tests for CalibrationLifecycleService.
// Epic 11: Position Decision Engine - Story 11.26: Calibration Data Lifecycle
package decision

import (
	"testing"
)

// TestConfidenceLevelCalculation tests the confidence level calculation logic.
func TestConfidenceLevelCalculation(t *testing.T) {
	tests := []struct {
		name           string
		totalTrades    int64
		expectedLevel  ConfidenceLevel
		expectedReliable bool
	}{
		{
			name:           "Zero trades - insufficient",
			totalTrades:    0,
			expectedLevel:  ConfidenceInsufficient,
			expectedReliable: false,
		},
		{
			name:           "5 trades - insufficient",
			totalTrades:    5,
			expectedLevel:  ConfidenceInsufficient,
			expectedReliable: false,
		},
		{
			name:           "10 trades - low confidence",
			totalTrades:    10,
			expectedLevel:  ConfidenceLow,
			expectedReliable: false,
		},
		{
			name:           "25 trades - low confidence",
			totalTrades:    25,
			expectedLevel:  ConfidenceLow,
			expectedReliable: false,
		},
		{
			name:           "30 trades - medium confidence",
			totalTrades:    30,
			expectedLevel:  ConfidenceMedium,
			expectedReliable: true,
		},
		{
			name:           "45 trades - medium confidence",
			totalTrades:    45,
			expectedLevel:  ConfidenceMedium,
			expectedReliable: true,
		},
		{
			name:           "50 trades - high confidence",
			totalTrades:    50,
			expectedLevel:  ConfidenceHigh,
			expectedReliable: true,
		},
		{
			name:           "100 trades - high confidence",
			totalTrades:    100,
			expectedLevel:  ConfidenceHigh,
			expectedReliable: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock calibration data
			data := &CalibrationData{
				TotalTrades: tc.totalTrades,
				Buckets:     GetDefaultBuckets(),
			}

			// Create lifecycle service with nil calibration service for unit testing
			cls := &CalibrationLifecycleService{}
			confidence := cls.calculateConfidence(data)

			if confidence.Level != tc.expectedLevel {
				t.Errorf("Expected level %s, got %s", tc.expectedLevel, confidence.Level)
			}

			if confidence.IsReliable != tc.expectedReliable {
				t.Errorf("Expected reliable=%v, got %v", tc.expectedReliable, confidence.IsReliable)
			}

			if confidence.TotalTrades != tc.totalTrades {
				t.Errorf("Expected totalTrades=%d, got %d", tc.totalTrades, confidence.TotalTrades)
			}
		})
	}
}

// TestTradesNeededCalculation tests the calculation of trades needed for next level.
func TestTradesNeededCalculation(t *testing.T) {
	tests := []struct {
		totalTrades    int64
		expectedNeeded int64
		expectedNext   ConfidenceLevel
	}{
		{0, 10, ConfidenceLow},      // 10 trades to reach low
		{5, 5, ConfidenceLow},       // 5 trades to reach low
		{10, 20, ConfidenceMedium},  // 20 trades to reach medium
		{25, 5, ConfidenceMedium},   // 5 trades to reach medium
		{30, 20, ConfidenceHigh},    // 20 trades to reach high
		{45, 5, ConfidenceHigh},     // 5 trades to reach high
		{50, 0, ConfidenceHigh},     // Already at high
		{100, 0, ConfidenceHigh},    // Already at high
	}

	for _, tc := range tests {
		data := &CalibrationData{
			TotalTrades: tc.totalTrades,
			Buckets:     GetDefaultBuckets(),
		}

		cls := &CalibrationLifecycleService{}
		confidence := cls.calculateConfidence(data)

		if confidence.TradesNeeded != tc.expectedNeeded {
			t.Errorf("totalTrades=%d: expected tradesNeeded=%d, got %d",
				tc.totalTrades, tc.expectedNeeded, confidence.TradesNeeded)
		}

		if confidence.NextLevel != tc.expectedNext {
			t.Errorf("totalTrades=%d: expected nextLevel=%s, got %s",
				tc.totalTrades, tc.expectedNext, confidence.NextLevel)
		}
	}
}

// TestReliabilityPercentage tests the reliability percentage calculation.
func TestReliabilityPercentage(t *testing.T) {
	tests := []struct {
		totalTrades int64
		minPct      float64
		maxPct      float64
	}{
		{0, 0, 1},       // 0%
		{5, 10, 15},     // ~12.5% (halfway to low)
		{10, 0, 1},      // 0% for low start
		{20, 20, 30},    // ~25% (halfway through low)
		{30, 49, 52},    // ~50% (start of medium)
		{40, 70, 80},    // ~75% (halfway through medium)
		{50, 99, 101},   // 100% (high confidence)
		{100, 99, 101},  // 100% (stays at high)
	}

	for _, tc := range tests {
		data := &CalibrationData{
			TotalTrades: tc.totalTrades,
			Buckets:     GetDefaultBuckets(),
		}

		cls := &CalibrationLifecycleService{}
		confidence := cls.calculateConfidence(data)

		if confidence.ReliabilityPct < tc.minPct || confidence.ReliabilityPct > tc.maxPct {
			t.Errorf("totalTrades=%d: expected reliability in [%.1f, %.1f], got %.1f",
				tc.totalTrades, tc.minPct, tc.maxPct, confidence.ReliabilityPct)
		}
	}
}

// TestBucketBreakdown tests that bucket breakdown is populated.
func TestBucketBreakdown(t *testing.T) {
	data := &CalibrationData{
		TotalTrades: 100,
		Buckets: map[string]*ScoreBucket{
			BucketLow:        {TotalTrades: 20},
			BucketMediumLow:  {TotalTrades: 25},
			BucketMediumHigh: {TotalTrades: 35},
			BucketHigh:       {TotalTrades: 20},
		},
	}

	cls := &CalibrationLifecycleService{}
	confidence := cls.calculateConfidence(data)

	if len(confidence.BucketBreakdown) != 4 {
		t.Errorf("Expected 4 buckets in breakdown, got %d", len(confidence.BucketBreakdown))
	}

	if confidence.BucketBreakdown[BucketLow] != 20 {
		t.Errorf("Expected BucketLow=20, got %d", confidence.BucketBreakdown[BucketLow])
	}

	if confidence.BucketBreakdown[BucketMediumHigh] != 35 {
		t.Errorf("Expected BucketMediumHigh=35, got %d", confidence.BucketBreakdown[BucketMediumHigh])
	}
}

// TestGetIndicatorHash tests the hash calculation wrapper.
func TestGetIndicatorHash(t *testing.T) {
	cls := &CalibrationLifecycleService{}

	// Empty indicators should return default hash
	hash := cls.GetIndicatorHash([]string{})
	if hash != "00000000" {
		t.Errorf("Expected default hash for empty indicators, got %s", hash)
	}

	// Same indicators in different order should produce same hash
	hash1 := cls.GetIndicatorHash([]string{"rsi", "macd", "ema_cross"})
	hash2 := cls.GetIndicatorHash([]string{"macd", "ema_cross", "rsi"})
	if hash1 != hash2 {
		t.Errorf("Expected same hash for same indicators in different order, got %s vs %s", hash1, hash2)
	}

	// Different indicators should produce different hashes
	hash3 := cls.GetIndicatorHash([]string{"rsi", "macd"})
	if hash1 == hash3 {
		t.Errorf("Expected different hash for different indicators, got same: %s", hash1)
	}
}

// TestTradeCloseEvent tests the TradeCloseEvent structure.
func TestTradeCloseEvent(t *testing.T) {
	event := &TradeCloseEvent{
		UserID:        1,
		Strategy:      "trend_following",
		IndicatorHash: "abc12345",
		Indicators:    []string{"rsi", "macd"},
		EntryScore:    75,
		IsWin:         true,
		PnLPercent:    2.5,
		Symbol:        "BTCUSDT",
	}

	if event.UserID != 1 {
		t.Errorf("Expected UserID=1, got %d", event.UserID)
	}
	if event.Strategy != "trend_following" {
		t.Errorf("Expected Strategy=trend_following, got %s", event.Strategy)
	}
	if event.EntryScore != 75 {
		t.Errorf("Expected EntryScore=75, got %d", event.EntryScore)
	}
	if !event.IsWin {
		t.Error("Expected IsWin=true")
	}
}

// TestIndicatorChangeEvent tests the IndicatorChangeEvent structure.
func TestIndicatorChangeEvent(t *testing.T) {
	event := &IndicatorChangeEvent{
		UserID:        1,
		Strategy:      "trend_following",
		OldIndicators: []string{"rsi", "macd"},
		NewIndicators: []string{"supertrend"},
		OldHash:       "abc12345",
		NewHash:       "def67890",
	}

	if event.UserID != 1 {
		t.Errorf("Expected UserID=1, got %d", event.UserID)
	}
	if len(event.OldIndicators) != 2 {
		t.Errorf("Expected 2 old indicators, got %d", len(event.OldIndicators))
	}
	if len(event.NewIndicators) != 1 {
		t.Errorf("Expected 1 new indicator, got %d", len(event.NewIndicators))
	}
	if event.OldHash == event.NewHash {
		t.Error("Expected different hashes for different indicators")
	}
}

// TestNilServiceHandling tests that nil services are handled gracefully.
func TestNilServiceHandling(t *testing.T) {
	var cls *CalibrationLifecycleService = nil

	// These should not panic
	hash := ""
	if cls != nil {
		hash = cls.GetIndicatorHash([]string{"rsi"})
	}
	if hash != "" {
		t.Error("Expected empty hash for nil service")
	}

	// CreateNil lifecycle service
	nilCls := NewCalibrationLifecycleService(nil)
	if nilCls != nil {
		t.Error("Expected nil lifecycle service when created with nil calibration service")
	}
}
