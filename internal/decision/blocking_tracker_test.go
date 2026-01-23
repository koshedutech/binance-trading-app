// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.18: Blocking Reason Tracker Tests
package decision

import (
	"testing"
)

// TestBlockingTracker_TrendDivergence tests trend divergence detection
func TestBlockingTracker_TrendDivergence(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)

	testCases := []struct {
		name           string
		trend1H        TrendDirection
		trend15M       TrendDirection
		expectDivergence bool
	}{
		{
			name:           "bullish vs bearish = divergence",
			trend1H:        TrendBullish,
			trend15M:       TrendBearish,
			expectDivergence: true,
		},
		{
			name:           "bearish vs bullish = divergence",
			trend1H:        TrendBearish,
			trend15M:       TrendBullish,
			expectDivergence: true,
		},
		{
			name:           "bullish vs bullish = no divergence",
			trend1H:        TrendBullish,
			trend15M:       TrendBullish,
			expectDivergence: false,
		},
		{
			name:           "bearish vs bearish = no divergence",
			trend1H:        TrendBearish,
			trend15M:       TrendBearish,
			expectDivergence: false,
		},
		{
			name:           "neutral vs bullish = no divergence",
			trend1H:        TrendNeutral,
			trend15M:       TrendBullish,
			expectDivergence: false,
		},
		{
			name:           "bullish vs neutral = no divergence",
			trend1H:        TrendBullish,
			trend15M:       TrendNeutral,
			expectDivergence: false,
		},
		{
			name:           "empty vs bullish = no divergence",
			trend1H:        "",
			trend15M:       TrendBullish,
			expectDivergence: false,
		},
		{
			name:           "neutral vs neutral = no divergence",
			trend1H:        TrendNeutral,
			trend15M:       TrendNeutral,
			expectDivergence: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := &CoinState{
				Symbol:   "BTCUSDT",
				Trend1H:  tc.trend1H,
				Trend15M: tc.trend15M,
			}

			reasons := tracker.CheckForBlocks(state, nil)
			hasDivergence := tracker.GetReasonByCode(reasons, ReasonTrendDivergence) != nil

			if hasDivergence != tc.expectDivergence {
				t.Errorf("expected divergence=%v, got %v", tc.expectDivergence, hasDivergence)
			}

			if hasDivergence {
				reason := tracker.GetReasonByCode(reasons, ReasonTrendDivergence)
				if reason.Category != CategoryHardBlock {
					t.Errorf("trend divergence should be HARD_BLOCK, got %s", reason.Category)
				}
				if reason.Overridable {
					t.Error("hard block should not be overridable")
				}
			}
		})
	}
}

// TestBlockingTracker_ADXTooLow tests ADX threshold blocking
func TestBlockingTracker_ADXTooLow(t *testing.T) {
	testCases := []struct {
		name        string
		adx         float64
		adxMinimum  float64
		expectBlock bool
	}{
		{
			name:        "ADX below threshold = block",
			adx:         10,
			adxMinimum:  15,
			expectBlock: true,
		},
		{
			name:        "ADX at threshold = no block",
			adx:         15,
			adxMinimum:  15,
			expectBlock: false,
		},
		{
			name:        "ADX above threshold = no block",
			adx:         25,
			adxMinimum:  15,
			expectBlock: false,
		},
		{
			name:        "ADX zero = no block (no data)",
			adx:         0,
			adxMinimum:  15,
			expectBlock: false,
		},
		{
			name:        "ADX just below threshold = block",
			adx:         14.9,
			adxMinimum:  15,
			expectBlock: true,
		},
		{
			name:        "Custom high threshold",
			adx:         20,
			adxMinimum:  25,
			expectBlock: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &BlockingConfig{
				ADXMinimum:       tc.adxMinimum,
				ScoreMinimum:     55,
				EnableHardBlocks: true,
				EnableSoftBlocks: true,
				EnableWarnings:   true,
				MaxBlockingReasons: 10,
			}
			tracker := NewBlockingTracker(nil, config)

			state := &CoinState{
				Symbol: "ETHUSDT",
				ADX:    tc.adx,
			}

			reasons := tracker.CheckForBlocks(state, nil)
			hasADXBlock := tracker.GetReasonByCode(reasons, ReasonADXTooLow) != nil

			if hasADXBlock != tc.expectBlock {
				t.Errorf("ADX=%.1f, threshold=%.1f: expected block=%v, got %v",
					tc.adx, tc.adxMinimum, tc.expectBlock, hasADXBlock)
			}

			if hasADXBlock {
				reason := tracker.GetReasonByCode(reasons, ReasonADXTooLow)
				if reason.Category != CategoryHardBlock {
					t.Errorf("ADX too low should be HARD_BLOCK, got %s", reason.Category)
				}
				if reason.Value != tc.adx {
					t.Errorf("expected value %.1f, got %v", tc.adx, reason.Value)
				}
				if reason.Threshold != tc.adxMinimum {
					t.Errorf("expected threshold %.1f, got %v", tc.adxMinimum, reason.Threshold)
				}
			}
		})
	}
}

// TestBlockingTracker_ScoreBelowThreshold tests score threshold soft blocking
func TestBlockingTracker_ScoreBelowThreshold(t *testing.T) {
	testCases := []struct {
		name        string
		score       float64
		threshold   float64
		expectBlock bool
	}{
		{
			name:        "score below threshold = soft block",
			score:       45,
			threshold:   55,
			expectBlock: true,
		},
		{
			name:        "score at threshold = no block",
			score:       55,
			threshold:   55,
			expectBlock: false,
		},
		{
			name:        "score above threshold = no block",
			score:       70,
			threshold:   55,
			expectBlock: false,
		},
		{
			name:        "score just below threshold",
			score:       54.9,
			threshold:   55,
			expectBlock: true,
		},
		{
			name:        "high threshold",
			score:       65,
			threshold:   70,
			expectBlock: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &BlockingConfig{
				ADXMinimum:       15,
				ScoreMinimum:     tc.threshold,
				EnableHardBlocks: true,
				EnableSoftBlocks: true,
				EnableWarnings:   true,
				MaxBlockingReasons: 10,
			}
			tracker := NewBlockingTracker(nil, config)

			state := &CoinState{
				Symbol: "BNBUSDT",
			}

			score := &ScoreBreakdown{
				Symbol:          "BNBUSDT",
				NormalizedScore: tc.score,
			}

			reasons := tracker.CheckForBlocks(state, score)
			hasScoreBlock := tracker.GetReasonByCode(reasons, ReasonScoreBelowThreshold) != nil

			if hasScoreBlock != tc.expectBlock {
				t.Errorf("score=%.1f, threshold=%.1f: expected block=%v, got %v",
					tc.score, tc.threshold, tc.expectBlock, hasScoreBlock)
			}

			if hasScoreBlock {
				reason := tracker.GetReasonByCode(reasons, ReasonScoreBelowThreshold)
				if reason.Category != CategorySoftBlock {
					t.Errorf("score below threshold should be SOFT_BLOCK, got %s", reason.Category)
				}
				if !reason.Overridable {
					t.Error("soft block should be overridable")
				}
			}
		})
	}
}

// TestBlockingTracker_HasHardBlock tests hard block detection
func TestBlockingTracker_HasHardBlock(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)

	testCases := []struct {
		name     string
		reasons  []BlockingReason
		expected bool
	}{
		{
			name:     "empty reasons",
			reasons:  []BlockingReason{},
			expected: false,
		},
		{
			name: "only warnings",
			reasons: []BlockingReason{
				{Code: ReasonHighRSI, Category: CategoryWarning},
				{Code: ReasonLowVolume, Category: CategoryWarning},
			},
			expected: false,
		},
		{
			name: "only soft blocks",
			reasons: []BlockingReason{
				{Code: ReasonScoreBelowThreshold, Category: CategorySoftBlock},
			},
			expected: false,
		},
		{
			name: "has hard block",
			reasons: []BlockingReason{
				{Code: ReasonTrendDivergence, Category: CategoryHardBlock},
			},
			expected: true,
		},
		{
			name: "mixed with hard block",
			reasons: []BlockingReason{
				{Code: ReasonHighRSI, Category: CategoryWarning},
				{Code: ReasonADXTooLow, Category: CategoryHardBlock},
				{Code: ReasonScoreBelowThreshold, Category: CategorySoftBlock},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tracker.HasHardBlock(tc.reasons)
			if result != tc.expected {
				t.Errorf("expected HasHardBlock=%v, got %v", tc.expected, result)
			}
		})
	}
}

// TestBlockingTracker_HasSoftBlock tests soft block detection
func TestBlockingTracker_HasSoftBlock(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)

	testCases := []struct {
		name     string
		reasons  []BlockingReason
		expected bool
	}{
		{
			name:     "empty reasons",
			reasons:  []BlockingReason{},
			expected: false,
		},
		{
			name: "only warnings",
			reasons: []BlockingReason{
				{Code: ReasonHighRSI, Category: CategoryWarning},
			},
			expected: false,
		},
		{
			name: "has soft block",
			reasons: []BlockingReason{
				{Code: ReasonScoreBelowThreshold, Category: CategorySoftBlock},
			},
			expected: true,
		},
		{
			name: "only hard blocks",
			reasons: []BlockingReason{
				{Code: ReasonTrendDivergence, Category: CategoryHardBlock},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tracker.HasSoftBlock(tc.reasons)
			if result != tc.expected {
				t.Errorf("expected HasSoftBlock=%v, got %v", tc.expected, result)
			}
		})
	}
}

// TestBlockingTracker_NoBlocks tests scenarios with no blocking conditions
func TestBlockingTracker_NoBlocks(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)

	// Optimal state with no blocking conditions
	state := &CoinState{
		Symbol:   "BTCUSDT",
		Trend1H:  TrendBullish,
		Trend15M: TrendBullish, // Aligned
		ADX:      30,           // Above threshold
		RSI:      50,           // Normal range
	}

	score := &ScoreBreakdown{
		Symbol:          "BTCUSDT",
		NormalizedScore: 75, // Above threshold
		Components: []ComponentScore{
			{
				Dimension: DimensionLLM,
				Score:     15,
				MaxScore:  20,
				Details:   map[string]float64{},
			},
			{
				Dimension: DimensionTechnical,
				Score:     35,
				MaxScore:  40,
				Details:   map[string]float64{"momentum": 8},
			},
		},
	}

	reasons := tracker.CheckForBlocks(state, score)

	// Should have no hard or soft blocks
	if tracker.HasHardBlock(reasons) {
		t.Error("optimal state should have no hard blocks")
	}
	if tracker.HasSoftBlock(reasons) {
		t.Error("optimal state should have no soft blocks")
	}
	if tracker.IsBlocked(reasons) {
		t.Error("optimal state should not be blocked")
	}
}

// TestBlockingTracker_History tests history tracking
func TestBlockingTracker_History(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)
	tracker.SetMaxHistory(5)

	userID := "test-user"
	symbol := "BTCUSDT"

	// Add some blocking reasons to history
	for i := 0; i < 7; i++ {
		reason := BlockingReason{
			Code:        ReasonTrendDivergence,
			Category:    CategoryHardBlock,
			Description: "Test divergence",
			Timestamp:   int64(i),
		}
		tracker.addToHistory(userID, symbol, reason)
	}

	// Check history is limited to maxHistory
	history := tracker.GetHistory(userID, symbol)
	if len(history) != 5 {
		t.Errorf("expected 5 history entries, got %d", len(history))
	}

	// Oldest entries should be trimmed (0, 1 should be gone)
	if history[0].Timestamp != 2 {
		t.Errorf("expected oldest remaining entry timestamp=2, got %d", history[0].Timestamp)
	}

	// Test clear history
	tracker.ClearHistory(userID, symbol)
	history = tracker.GetHistory(userID, symbol)
	if len(history) != 0 {
		t.Errorf("expected empty history after clear, got %d entries", len(history))
	}
}

// TestBlockingTracker_ConfigurableThresholds tests config modification
func TestBlockingTracker_ConfigurableThresholds(t *testing.T) {
	// Test with default config
	defaultTracker := NewBlockingTracker(nil, nil)
	defaultConfig := defaultTracker.GetConfig()

	if defaultConfig.ADXMinimum != 15 {
		t.Errorf("expected default ADXMinimum=15, got %.1f", defaultConfig.ADXMinimum)
	}
	if defaultConfig.ScoreMinimum != 55 {
		t.Errorf("expected default ScoreMinimum=55, got %.1f", defaultConfig.ScoreMinimum)
	}

	// Test with custom config
	customConfig := &BlockingConfig{
		ADXMinimum:         20,
		ScoreMinimum:       60,
		RSIUpperLimit:      85,
		RSILowerLimit:      15,
		MaxBlockingReasons: 5,
		EnableHardBlocks:   true,
		EnableSoftBlocks:   false, // Disable soft blocks
		EnableWarnings:     true,
	}
	customTracker := NewBlockingTracker(nil, customConfig)

	state := &CoinState{
		Symbol: "ETHUSDT",
		ADX:    18, // Between 15 and 20
	}

	score := &ScoreBreakdown{
		NormalizedScore: 58, // Between 55 and 60
	}

	reasons := customTracker.CheckForBlocks(state, score)

	// Should trigger ADX block with custom threshold
	adxBlock := customTracker.GetReasonByCode(reasons, ReasonADXTooLow)
	if adxBlock == nil {
		t.Error("expected ADX block with custom threshold")
	}

	// Should NOT trigger score block (soft blocks disabled)
	scoreBlock := customTracker.GetReasonByCode(reasons, ReasonScoreBelowThreshold)
	if scoreBlock != nil {
		t.Error("soft blocks disabled, should not have score block")
	}
}

// TestBlockingTracker_DisabledCategories tests enabling/disabling block categories
func TestBlockingTracker_DisabledCategories(t *testing.T) {
	testCases := []struct {
		name            string
		enableHard      bool
		enableSoft      bool
		enableWarnings  bool
		expectedHard    bool
		expectedSoft    bool
		expectedWarning bool
	}{
		{
			name:            "all enabled",
			enableHard:      true,
			enableSoft:      true,
			enableWarnings:  true,
			expectedHard:    true,
			expectedSoft:    true,
			expectedWarning: true,
		},
		{
			name:            "only hard blocks",
			enableHard:      true,
			enableSoft:      false,
			enableWarnings:  false,
			expectedHard:    true,
			expectedSoft:    false,
			expectedWarning: false,
		},
		{
			name:            "only soft blocks",
			enableHard:      false,
			enableSoft:      true,
			enableWarnings:  false,
			expectedHard:    false,
			expectedSoft:    true,
			expectedWarning: false,
		},
		{
			name:            "only warnings",
			enableHard:      false,
			enableSoft:      false,
			enableWarnings:  true,
			expectedHard:    false,
			expectedSoft:    false,
			expectedWarning: true,
		},
		{
			name:            "all disabled",
			enableHard:      false,
			enableSoft:      false,
			enableWarnings:  false,
			expectedHard:    false,
			expectedSoft:    false,
			expectedWarning: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &BlockingConfig{
				ADXMinimum:         15,
				ScoreMinimum:       55,
				RSIUpperLimit:      80,
				RSILowerLimit:      20,
				MaxBlockingReasons: 10,
				EnableHardBlocks:   tc.enableHard,
				EnableSoftBlocks:   tc.enableSoft,
				EnableWarnings:     tc.enableWarnings,
			}
			tracker := NewBlockingTracker(nil, config)

			// State that should trigger all block types
			state := &CoinState{
				Symbol:   "TESTUSDT",
				Trend1H:  TrendBullish,
				Trend15M: TrendBearish, // Hard block: divergence
				ADX:      10,           // Hard block: ADX too low
				RSI:      85,           // Warning: high RSI
			}

			score := &ScoreBreakdown{
				NormalizedScore: 45, // Soft block: below threshold
			}

			reasons := tracker.CheckForBlocks(state, score)

			hasHard := tracker.HasHardBlock(reasons)
			hasSoft := tracker.HasSoftBlock(reasons)
			hasWarning := tracker.HasWarning(reasons)

			if hasHard != tc.expectedHard {
				t.Errorf("expected hasHard=%v, got %v", tc.expectedHard, hasHard)
			}
			if hasSoft != tc.expectedSoft {
				t.Errorf("expected hasSoft=%v, got %v", tc.expectedSoft, hasSoft)
			}
			if hasWarning != tc.expectedWarning {
				t.Errorf("expected hasWarning=%v, got %v", tc.expectedWarning, hasWarning)
			}
		})
	}
}

// TestBlockingTracker_Summary tests the blocking summary generation
func TestBlockingTracker_Summary(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)

	reasons := []BlockingReason{
		{Code: ReasonTrendDivergence, Category: CategoryHardBlock},
		{Code: ReasonADXTooLow, Category: CategoryHardBlock},
		{Code: ReasonScoreBelowThreshold, Category: CategorySoftBlock},
		{Code: ReasonHighRSI, Category: CategoryWarning},
		{Code: ReasonLowVolume, Category: CategoryWarning},
	}

	summary := tracker.GetBlockingSummary(reasons)

	if summary.TotalReasons != 5 {
		t.Errorf("expected 5 total reasons, got %d", summary.TotalReasons)
	}
	if summary.HardBlockCount != 2 {
		t.Errorf("expected 2 hard blocks, got %d", summary.HardBlockCount)
	}
	if summary.SoftBlockCount != 1 {
		t.Errorf("expected 1 soft block, got %d", summary.SoftBlockCount)
	}
	if summary.WarningCount != 2 {
		t.Errorf("expected 2 warnings, got %d", summary.WarningCount)
	}
	if !summary.IsBlocked {
		t.Error("expected IsBlocked=true")
	}
	if summary.CanOverride {
		t.Error("expected CanOverride=false (has hard blocks)")
	}
	if summary.PrimaryReason == nil {
		t.Error("expected primary reason to be set")
	}
	if summary.PrimaryReason.Code != ReasonTrendDivergence {
		t.Errorf("expected primary reason to be first hard block, got %s", summary.PrimaryReason.Code)
	}
}

// TestBlockingTracker_CanOverride tests override capability detection
func TestBlockingTracker_CanOverride(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)

	testCases := []struct {
		name        string
		reasons     []BlockingReason
		canOverride bool
	}{
		{
			name:        "no blocks",
			reasons:     []BlockingReason{},
			canOverride: true,
		},
		{
			name: "only warnings",
			reasons: []BlockingReason{
				{Code: ReasonHighRSI, Category: CategoryWarning},
			},
			canOverride: true,
		},
		{
			name: "only soft blocks",
			reasons: []BlockingReason{
				{Code: ReasonScoreBelowThreshold, Category: CategorySoftBlock},
			},
			canOverride: true,
		},
		{
			name: "has hard block",
			reasons: []BlockingReason{
				{Code: ReasonTrendDivergence, Category: CategoryHardBlock},
			},
			canOverride: false,
		},
		{
			name: "soft block + hard block",
			reasons: []BlockingReason{
				{Code: ReasonScoreBelowThreshold, Category: CategorySoftBlock},
				{Code: ReasonADXTooLow, Category: CategoryHardBlock},
			},
			canOverride: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tracker.CanOverride(tc.reasons)
			if result != tc.canOverride {
				t.Errorf("expected CanOverride=%v, got %v", tc.canOverride, result)
			}
		})
	}
}

// TestBlockingTracker_Stats tests statistics calculation
func TestBlockingTracker_Stats(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)

	userID := "stats-test-user"

	// Add blocking reasons for different symbols
	reasons1 := []BlockingReason{
		{Code: ReasonTrendDivergence, Category: CategoryHardBlock},
		{Code: ReasonTrendDivergence, Category: CategoryHardBlock},
		{Code: ReasonADXTooLow, Category: CategoryHardBlock},
	}
	for _, r := range reasons1 {
		tracker.addToHistory(userID, "BTCUSDT", r)
	}

	reasons2 := []BlockingReason{
		{Code: ReasonScoreBelowThreshold, Category: CategorySoftBlock},
		{Code: ReasonHighRSI, Category: CategoryWarning},
	}
	for _, r := range reasons2 {
		tracker.addToHistory(userID, "ETHUSDT", r)
	}

	stats := tracker.GetBlockingStats(userID)

	if stats.TotalBlocks != 5 {
		t.Errorf("expected 5 total blocks, got %d", stats.TotalBlocks)
	}
	if stats.SymbolsAffected != 2 {
		t.Errorf("expected 2 symbols affected, got %d", stats.SymbolsAffected)
	}
	if stats.BlocksByCode[ReasonTrendDivergence] != 2 {
		t.Errorf("expected 2 trend divergence blocks, got %d", stats.BlocksByCode[ReasonTrendDivergence])
	}
	if stats.BlocksByCategory[CategoryHardBlock] != 3 {
		t.Errorf("expected 3 hard blocks, got %d", stats.BlocksByCategory[CategoryHardBlock])
	}
	if stats.MostFrequentBlock != ReasonTrendDivergence {
		t.Errorf("expected most frequent block to be TREND_DIVERGENCE, got %s", stats.MostFrequentBlock)
	}
}

// TestBlockingReason_String tests the string representation
func TestBlockingReason_String(t *testing.T) {
	reason := NewBlockingReason(
		ReasonADXTooLow,
		CategoryHardBlock,
		"ADX is below threshold",
	).WithValue(10.5).WithThreshold(15.0)

	str := reason.String()
	if str == "" {
		t.Error("expected non-empty string")
	}

	// Check it contains expected parts
	if !containsSubstring(str, "HARD_BLOCK") {
		t.Error("string should contain category")
	}
	if !containsSubstring(str, "ADX_TOO_LOW") {
		t.Error("string should contain code")
	}
	if !containsSubstring(str, "10.5") {
		t.Error("string should contain value")
	}
	if !containsSubstring(str, "15") {
		t.Error("string should contain threshold")
	}
}

// TestBlockingConfig_Validate tests config validation
func TestBlockingConfig_Validate(t *testing.T) {
	config := &BlockingConfig{
		ADXMinimum:         -10,  // Invalid
		ScoreMinimum:       150,  // Invalid
		RSIUpperLimit:      30,   // Invalid (too low)
		RSILowerLimit:      80,   // Invalid (too high)
		WinRateMinimum:     -5,   // Invalid
		MaxBlockingReasons: -1,   // Invalid
	}

	config.Validate()

	if config.ADXMinimum != 0 {
		t.Errorf("ADXMinimum should clamp to 0, got %.1f", config.ADXMinimum)
	}
	if config.ScoreMinimum != 100 {
		t.Errorf("ScoreMinimum should clamp to 100, got %.1f", config.ScoreMinimum)
	}
	if config.RSIUpperLimit != 50 {
		t.Errorf("RSIUpperLimit should clamp to 50, got %.1f", config.RSIUpperLimit)
	}
	if config.RSILowerLimit != 50 {
		t.Errorf("RSILowerLimit should clamp to 50, got %.1f", config.RSILowerLimit)
	}
	if config.WinRateMinimum != 0 {
		t.Errorf("WinRateMinimum should clamp to 0, got %.1f", config.WinRateMinimum)
	}
	if config.MaxBlockingReasons != 10 {
		t.Errorf("MaxBlockingReasons should clamp to 10, got %d", config.MaxBlockingReasons)
	}
}

// TestBlockingTracker_RSIWarnings tests RSI warning detection
func TestBlockingTracker_RSIWarnings(t *testing.T) {
	config := &BlockingConfig{
		RSIUpperLimit:      80,
		RSILowerLimit:      20,
		EnableHardBlocks:   true,
		EnableSoftBlocks:   true,
		EnableWarnings:     true,
		MaxBlockingReasons: 10,
	}
	tracker := NewBlockingTracker(nil, config)

	testCases := []struct {
		name            string
		rsi             float64
		expectHighRSI   bool
		expectLowRSI    bool
	}{
		{
			name:          "high RSI warning",
			rsi:           85,
			expectHighRSI: true,
			expectLowRSI:  false,
		},
		{
			name:          "low RSI warning",
			rsi:           15,
			expectHighRSI: false,
			expectLowRSI:  true,
		},
		{
			name:          "normal RSI",
			rsi:           50,
			expectHighRSI: false,
			expectLowRSI:  false,
		},
		{
			name:          "at upper boundary",
			rsi:           80,
			expectHighRSI: false,
			expectLowRSI:  false,
		},
		{
			name:          "just above upper",
			rsi:           80.1,
			expectHighRSI: true,
			expectLowRSI:  false,
		},
		{
			name:          "zero RSI (no data)",
			rsi:           0,
			expectHighRSI: false,
			expectLowRSI:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := &CoinState{
				Symbol: "TESTUSDT",
				RSI:    tc.rsi,
			}

			reasons := tracker.CheckForBlocks(state, nil)

			hasHighRSI := tracker.GetReasonByCode(reasons, ReasonHighRSI) != nil
			hasLowRSI := tracker.GetReasonByCode(reasons, ReasonLowRSI) != nil

			if hasHighRSI != tc.expectHighRSI {
				t.Errorf("RSI=%.1f: expected highRSI=%v, got %v", tc.rsi, tc.expectHighRSI, hasHighRSI)
			}
			if hasLowRSI != tc.expectLowRSI {
				t.Errorf("RSI=%.1f: expected lowRSI=%v, got %v", tc.rsi, tc.expectLowRSI, hasLowRSI)
			}
		})
	}
}

// TestBlockingTracker_GetReasonsByCategory tests category filtering
func TestBlockingTracker_GetReasonsByCategory(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)

	reasons := []BlockingReason{
		{Code: ReasonTrendDivergence, Category: CategoryHardBlock},
		{Code: ReasonADXTooLow, Category: CategoryHardBlock},
		{Code: ReasonScoreBelowThreshold, Category: CategorySoftBlock},
		{Code: ReasonHighRSI, Category: CategoryWarning},
		{Code: ReasonLowVolume, Category: CategoryWarning},
	}

	hardBlocks := tracker.GetReasonsByCategory(reasons, CategoryHardBlock)
	if len(hardBlocks) != 2 {
		t.Errorf("expected 2 hard blocks, got %d", len(hardBlocks))
	}

	softBlocks := tracker.GetReasonsByCategory(reasons, CategorySoftBlock)
	if len(softBlocks) != 1 {
		t.Errorf("expected 1 soft block, got %d", len(softBlocks))
	}

	warnings := tracker.GetReasonsByCategory(reasons, CategoryWarning)
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(warnings))
	}
}

// TestBlockingTracker_NilState tests handling of nil state
func TestBlockingTracker_NilState(t *testing.T) {
	tracker := NewBlockingTracker(nil, nil)

	reasons := tracker.CheckForBlocks(nil, nil)
	if reasons != nil {
		t.Error("expected nil reasons for nil state")
	}
}

// TestBlockingTracker_MaxBlockingReasons tests reason count limiting
func TestBlockingTracker_MaxBlockingReasons(t *testing.T) {
	config := &BlockingConfig{
		ADXMinimum:         50,  // High threshold to trigger
		ScoreMinimum:       90,  // High threshold to trigger
		RSIUpperLimit:      30,  // Low threshold to trigger (inverted)
		RSILowerLimit:      70,  // High threshold to trigger (inverted)
		EnableHardBlocks:   true,
		EnableSoftBlocks:   true,
		EnableWarnings:     true,
		MaxBlockingReasons: 2, // Only allow 2 reasons
	}
	config.Validate() // Note: this will adjust the RSI limits

	// Use valid config with low max reasons
	config2 := &BlockingConfig{
		ADXMinimum:         50,
		ScoreMinimum:       90,
		RSIUpperLimit:      80,
		RSILowerLimit:      20,
		EnableHardBlocks:   true,
		EnableSoftBlocks:   true,
		EnableWarnings:     true,
		MaxBlockingReasons: 2,
	}
	tracker := NewBlockingTracker(nil, config2)

	state := &CoinState{
		Symbol:   "TESTUSDT",
		Trend1H:  TrendBullish,
		Trend15M: TrendBearish, // Divergence
		ADX:      10,           // Below 50
		RSI:      85,           // Above 80
	}

	score := &ScoreBreakdown{
		NormalizedScore: 50, // Below 90
	}

	reasons := tracker.CheckForBlocks(state, score)

	if len(reasons) > 2 {
		t.Errorf("expected max 2 reasons, got %d", len(reasons))
	}
}

// BenchmarkCheckForBlocks benchmarks block detection
func BenchmarkCheckForBlocks(b *testing.B) {
	tracker := NewBlockingTracker(nil, nil)

	state := &CoinState{
		Symbol:   "BTCUSDT",
		Trend1H:  TrendBullish,
		Trend15M: TrendBearish,
		ADX:      10,
		RSI:      85,
	}

	score := &ScoreBreakdown{
		Symbol:          "BTCUSDT",
		NormalizedScore: 45,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.CheckForBlocks(state, score)
	}
}

// BenchmarkGetBlockingSummary benchmarks summary generation
func BenchmarkGetBlockingSummary(b *testing.B) {
	tracker := NewBlockingTracker(nil, nil)

	reasons := []BlockingReason{
		{Code: ReasonTrendDivergence, Category: CategoryHardBlock},
		{Code: ReasonADXTooLow, Category: CategoryHardBlock},
		{Code: ReasonScoreBelowThreshold, Category: CategorySoftBlock},
		{Code: ReasonHighRSI, Category: CategoryWarning},
		{Code: ReasonLowVolume, Category: CategoryWarning},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.GetBlockingSummary(reasons)
	}
}

// ==================== Story 11.40 Gap Analysis Tests ====================

// TestCalculateGap tests the CalculateGap function for gap analysis UI
func TestCalculateGap(t *testing.T) {
	testCases := []struct {
		name           string
		currentValue   float64
		targetValue    float64
		targetRangeEnd float64
		expectedGap    float64
		expectedDir    GapDirection
		expectDisplay  string
	}{
		// Single target tests
		{
			name:           "below single target - needs to go up",
			currentValue:   45,
			targetValue:    55,
			targetRangeEnd: 0,
			expectedGap:    10,
			expectedDir:    GapDirectionUp,
			expectDisplay:  "↑10.0",
		},
		{
			name:           "at single target - met",
			currentValue:   55,
			targetValue:    55,
			targetRangeEnd: 0,
			expectedGap:    0,
			expectedDir:    GapDirectionInRange,
			expectDisplay:  "✓ Met",
		},
		{
			name:           "above single target - needs to go down",
			currentValue:   70,
			targetValue:    55,
			targetRangeEnd: 0,
			expectedGap:    15,
			expectedDir:    GapDirectionDown,
			expectDisplay:  "↓15.0",
		},
		// Range target tests (e.g., RSI optimal range 40-60)
		{
			name:           "below range - needs to go up",
			currentValue:   30,
			targetValue:    40,
			targetRangeEnd: 60,
			expectedGap:    10,
			expectedDir:    GapDirectionUp,
			expectDisplay:  "↑10.0",
		},
		{
			name:           "in range - met",
			currentValue:   50,
			targetValue:    40,
			targetRangeEnd: 60,
			expectedGap:    0,
			expectedDir:    GapDirectionInRange,
			expectDisplay:  "✓ In Range",
		},
		{
			name:           "above range - needs to go down",
			currentValue:   75,
			targetValue:    40,
			targetRangeEnd: 60,
			expectedGap:    15,
			expectedDir:    GapDirectionDown,
			expectDisplay:  "↓15.0",
		},
		{
			name:           "at range lower boundary - in range",
			currentValue:   40,
			targetValue:    40,
			targetRangeEnd: 60,
			expectedGap:    0,
			expectedDir:    GapDirectionInRange,
			expectDisplay:  "✓ In Range",
		},
		{
			name:           "at range upper boundary - in range",
			currentValue:   60,
			targetValue:    40,
			targetRangeEnd: 60,
			expectedGap:    0,
			expectedDir:    GapDirectionInRange,
			expectDisplay:  "✓ In Range",
		},
		// Edge cases
		{
			name:           "small gap with decimal",
			currentValue:   52.3,
			targetValue:    55,
			targetRangeEnd: 0,
			expectedGap:    2.7,
			expectedDir:    GapDirectionUp,
			expectDisplay:  "↑2.7",
		},
		{
			name:           "zero values",
			currentValue:   0,
			targetValue:    0,
			targetRangeEnd: 0,
			expectedGap:    0,
			expectedDir:    GapDirectionInRange,
			expectDisplay:  "✓ Met",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gap, dir, display := CalculateGap(tc.currentValue, tc.targetValue, tc.targetRangeEnd)

			// Check gap value (allow small floating point tolerance)
			if diff := gap - tc.expectedGap; diff > 0.01 || diff < -0.01 {
				t.Errorf("expected gap=%.2f, got %.2f", tc.expectedGap, gap)
			}

			if dir != tc.expectedDir {
				t.Errorf("expected direction=%s, got %s", tc.expectedDir, dir)
			}

			if display != tc.expectDisplay {
				t.Errorf("expected display=%q, got %q", tc.expectDisplay, display)
			}
		})
	}
}

// TestNewBlockingReasonWithGap tests BlockingReasonWithGap creation
func TestNewBlockingReasonWithGap(t *testing.T) {
	t.Run("nil BlockingReason returns nil", func(t *testing.T) {
		result := NewBlockingReasonWithGap(nil, 0)
		if result != nil {
			t.Error("expected nil result for nil input")
		}
	})

	t.Run("creates gap from BlockingReason with float64 values", func(t *testing.T) {
		br := &BlockingReason{
			Code:        ReasonADXTooLow,
			Category:    CategoryHardBlock,
			Description: "ADX is below threshold",
			Value:       float64(10),
			Threshold:   float64(25),
			Timestamp:   1234567890,
			Overridable: false,
		}

		result := NewBlockingReasonWithGap(br, 0)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Code != "ADX_TOO_LOW" {
			t.Errorf("expected code=ADX_TOO_LOW, got %s", result.Code)
		}
		if result.Category != "HARD_BLOCK" {
			t.Errorf("expected category=HARD_BLOCK, got %s", result.Category)
		}
		if result.CurrentValue != 10 {
			t.Errorf("expected currentValue=10, got %.1f", result.CurrentValue)
		}
		if result.TargetValue != 25 {
			t.Errorf("expected targetValue=25, got %.1f", result.TargetValue)
		}
		if result.Gap != 15 {
			t.Errorf("expected gap=15, got %.1f", result.Gap)
		}
		if result.GapDirection != GapDirectionUp {
			t.Errorf("expected direction=up, got %s", result.GapDirection)
		}
		if !result.Overridable != true {
			t.Error("expected overridable=false")
		}
	})

	t.Run("creates gap from BlockingReason with int values", func(t *testing.T) {
		br := &BlockingReason{
			Code:        ReasonScoreBelowThreshold,
			Category:    CategorySoftBlock,
			Description: "Score is below threshold",
			Value:       int(45),
			Threshold:   int(55),
			Timestamp:   1234567890,
			Overridable: true,
		}

		result := NewBlockingReasonWithGap(br, 0)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.CurrentValue != 45 {
			t.Errorf("expected currentValue=45, got %.1f", result.CurrentValue)
		}
		if result.TargetValue != 55 {
			t.Errorf("expected targetValue=55, got %.1f", result.TargetValue)
		}
	})

	t.Run("handles range target (RSI)", func(t *testing.T) {
		br := &BlockingReason{
			Code:        ReasonWeakMomentum,
			Category:    CategorySoftBlock,
			Description: "RSI not in optimal range",
			Value:       float64(30),
			Threshold:   float64(40),
			Timestamp:   1234567890,
			Overridable: true,
		}

		result := NewBlockingReasonWithGap(br, 60) // Range 40-60

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.TargetRangeEnd != 60 {
			t.Errorf("expected targetRangeEnd=60, got %.1f", result.TargetRangeEnd)
		}
		if result.Gap != 10 {
			t.Errorf("expected gap=10 (30 to 40), got %.1f", result.Gap)
		}
		if result.GapDirection != GapDirectionUp {
			t.Errorf("expected direction=up, got %s", result.GapDirection)
		}
	})
}

// TestGetBlockingReasonTargetRange tests target range lookup
func TestGetBlockingReasonTargetRange(t *testing.T) {
	testCases := []struct {
		code          BlockingReasonCode
		expectedRange float64
	}{
		{ReasonHighRSI, 0},
		{ReasonLowRSI, 0},
		{ReasonWeakMomentum, 60},
		{ReasonADXTooLow, 0},
		{ReasonScoreBelowThreshold, 0},
		{ReasonTrendDivergence, 0},
	}

	for _, tc := range testCases {
		t.Run(string(tc.code), func(t *testing.T) {
			result := GetBlockingReasonTargetRange(tc.code)
			if result != tc.expectedRange {
				t.Errorf("expected range=%.1f, got %.1f", tc.expectedRange, result)
			}
		})
	}
}
