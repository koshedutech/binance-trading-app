package decision

import (
	"context"
	"testing"
)

// TestRegimeClassifier_TrendingADX tests that high ADX values result in TRENDING regime.
func TestRegimeClassifier_TrendingADX(t *testing.T) {
	config := DefaultRegimeConfig()
	classifier := NewRegimeClassifier(config, nil)

	tests := []struct {
		name     string
		adx      float64
		expected MarketRegime
	}{
		{"ADX at threshold", 25.0, RegimeTrending},
		{"ADX above threshold", 30.0, RegimeTrending},
		{"ADX well above threshold", 50.0, RegimeTrending},
		{"ADX just below threshold", 24.9, RegimeRanging}, // Should NOT be trending
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyWithIndicators(tt.adx, 0, 0)
			if result != tt.expected {
				t.Errorf("ClassifyWithIndicators(adx=%.2f) = %s, want %s", tt.adx, result, tt.expected)
			}
		})
	}
}

// TestRegimeClassifier_RangingADX tests that low ADX with normal ATR results in RANGING regime.
func TestRegimeClassifier_RangingADX(t *testing.T) {
	config := DefaultRegimeConfig()
	classifier := NewRegimeClassifier(config, nil)

	tests := []struct {
		name       string
		adx        float64
		atr        float64
		atrAverage float64
		expected   MarketRegime
	}{
		{"Low ADX no ATR data", 15.0, 0, 0, RegimeRanging},
		{"Low ADX with normal ATR", 18.0, 100.0, 100.0, RegimeRanging},
		{"Low ADX with slightly elevated ATR", 15.0, 120.0, 100.0, RegimeRanging}, // < 1.5x
		{"Low ADX with slightly low ATR", 15.0, 60.0, 100.0, RegimeRanging},       // > 0.5x
		{"Mid-range ADX (between thresholds)", 22.0, 100.0, 100.0, RegimeRanging},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyWithIndicators(tt.adx, tt.atr, tt.atrAverage)
			if result != tt.expected {
				t.Errorf("ClassifyWithIndicators(adx=%.2f, atr=%.2f, atrAvg=%.2f) = %s, want %s",
					tt.adx, tt.atr, tt.atrAverage, result, tt.expected)
			}
		})
	}
}

// TestRegimeClassifier_Volatile tests that high ATR relative to average results in VOLATILE regime.
func TestRegimeClassifier_Volatile(t *testing.T) {
	config := DefaultRegimeConfig()
	classifier := NewRegimeClassifier(config, nil)

	tests := []struct {
		name       string
		adx        float64
		atr        float64
		atrAverage float64
		expected   MarketRegime
	}{
		{"Low ADX with high ATR", 15.0, 160.0, 100.0, RegimeVolatile},       // 1.6x > 1.5x
		{"Low ADX with very high ATR", 10.0, 200.0, 100.0, RegimeVolatile},  // 2.0x > 1.5x
		{"Low ADX at ATR threshold", 15.0, 151.0, 100.0, RegimeVolatile},    // 1.51x > 1.5x
		{"Low ADX just below threshold", 15.0, 149.0, 100.0, RegimeRanging}, // 1.49x < 1.5x
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyWithIndicators(tt.adx, tt.atr, tt.atrAverage)
			if result != tt.expected {
				t.Errorf("ClassifyWithIndicators(adx=%.2f, atr=%.2f, atrAvg=%.2f) = %s, want %s",
					tt.adx, tt.atr, tt.atrAverage, result, tt.expected)
			}
		})
	}
}

// TestRegimeClassifier_Consolidating tests that low ATR relative to average results in CONSOLIDATING regime.
func TestRegimeClassifier_Consolidating(t *testing.T) {
	config := DefaultRegimeConfig()
	classifier := NewRegimeClassifier(config, nil)

	tests := []struct {
		name       string
		adx        float64
		atr        float64
		atrAverage float64
		expected   MarketRegime
	}{
		{"Low ADX with low ATR", 15.0, 40.0, 100.0, RegimeConsolidating},       // 0.4x < 0.5x
		{"Low ADX with very low ATR", 10.0, 20.0, 100.0, RegimeConsolidating},  // 0.2x < 0.5x
		{"Low ADX at ATR threshold", 15.0, 49.0, 100.0, RegimeConsolidating},   // 0.49x < 0.5x
		{"Low ADX just above threshold", 15.0, 51.0, 100.0, RegimeRanging},     // 0.51x > 0.5x
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyWithIndicators(tt.adx, tt.atr, tt.atrAverage)
			if result != tt.expected {
				t.Errorf("ClassifyWithIndicators(adx=%.2f, atr=%.2f, atrAvg=%.2f) = %s, want %s",
					tt.adx, tt.atr, tt.atrAverage, result, tt.expected)
			}
		})
	}
}

// TestRegimeClassifier_History tests that regime changes are properly recorded.
func TestRegimeClassifier_History(t *testing.T) {
	config := DefaultRegimeConfig()
	config.MinRegimeDuration = 1 // Allow immediate changes for testing
	classifier := NewRegimeClassifier(config, nil)

	symbol := "BTCUSDT"

	// First classification (initial)
	regime1, changed1 := classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	if regime1 != RegimeTrending {
		t.Errorf("First classification = %s, want TRENDING", regime1)
	}
	if !changed1 {
		t.Error("First classification should indicate a change")
	}

	// Second classification (same regime - no change)
	regime2, changed2 := classifier.ClassifyAndRecord(symbol, 35.0, 0, 0)
	if regime2 != RegimeTrending {
		t.Errorf("Second classification = %s, want TRENDING", regime2)
	}
	if changed2 {
		t.Error("Second classification should NOT indicate a change (same regime)")
	}

	// Third classification (change to ranging)
	regime3, changed3 := classifier.ClassifyAndRecord(symbol, 15.0, 100, 100)
	if regime3 != RegimeRanging {
		t.Errorf("Third classification = %s, want RANGING", regime3)
	}
	if !changed3 {
		t.Error("Third classification should indicate a change")
	}

	// Verify history
	history := classifier.GetHistory(symbol)
	if len(history) != 2 { // Initial + one change
		t.Errorf("History length = %d, want 2", len(history))
	}

	// Verify first change (initial classification)
	if history[0].To != RegimeTrending {
		t.Errorf("First history entry To = %s, want TRENDING", history[0].To)
	}

	// Verify second change
	if history[1].From != RegimeTrending || history[1].To != RegimeRanging {
		t.Errorf("Second history entry = %s -> %s, want TRENDING -> RANGING",
			history[1].From, history[1].To)
	}
}

// TestRegimeClassifier_MinDuration tests that regime changes respect minimum duration.
func TestRegimeClassifier_MinDuration(t *testing.T) {
	config := DefaultRegimeConfig()
	config.MinRegimeDuration = 3
	classifier := NewRegimeClassifier(config, nil)

	symbol := "ETHUSDT"

	// Initial classification to TRENDING
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)

	// Try to change to RANGING immediately (should be blocked)
	regime, changed := classifier.ClassifyAndRecord(symbol, 15.0, 100, 100)
	if changed {
		t.Error("Change should be blocked (only 1 candle in current regime)")
	}
	if regime != RegimeTrending {
		t.Errorf("Should still be TRENDING, got %s", regime)
	}

	// Second attempt (still blocked)
	regime, changed = classifier.ClassifyAndRecord(symbol, 15.0, 100, 100)
	if changed {
		t.Error("Change should be blocked (only 2 candles in current regime)")
	}
	if regime != RegimeTrending {
		t.Errorf("Should still be TRENDING, got %s", regime)
	}

	// Third attempt (now allowed - 3 candles elapsed)
	regime, changed = classifier.ClassifyAndRecord(symbol, 15.0, 100, 100)
	if !changed {
		t.Error("Change should be allowed (3 candles in current regime)")
	}
	if regime != RegimeRanging {
		t.Errorf("Should now be RANGING, got %s", regime)
	}
}

// TestRegimeClassifier_CurrentStreak tests the GetCurrentStreak method.
func TestRegimeClassifier_CurrentStreak(t *testing.T) {
	config := DefaultRegimeConfig()
	config.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(config, nil)

	symbol := "SOLUSDT"

	// No data yet
	regime, streak := classifier.GetCurrentStreak(symbol)
	if regime != RegimeConsolidating || streak != 0 {
		t.Errorf("Initial streak = (%s, %d), want (CONSOLIDATING, 0)", regime, streak)
	}

	// First classification
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	regime, streak = classifier.GetCurrentStreak(symbol)
	if regime != RegimeTrending || streak != 1 {
		t.Errorf("After first = (%s, %d), want (TRENDING, 1)", regime, streak)
	}

	// Same regime again
	classifier.ClassifyAndRecord(symbol, 32.0, 0, 0)
	regime, streak = classifier.GetCurrentStreak(symbol)
	if regime != RegimeTrending || streak != 2 {
		t.Errorf("After second = (%s, %d), want (TRENDING, 2)", regime, streak)
	}

	// Change regime
	classifier.ClassifyAndRecord(symbol, 15.0, 100, 100)
	regime, streak = classifier.GetCurrentStreak(symbol)
	if regime != RegimeRanging || streak != 1 {
		t.Errorf("After change = (%s, %d), want (RANGING, 1)", regime, streak)
	}
}

// TestRegimeClassifier_ConfigurableThresholds tests custom threshold configuration.
func TestRegimeClassifier_ConfigurableThresholds(t *testing.T) {
	// Custom configuration with stricter thresholds
	config := &RegimeConfig{
		ADXTrendingThreshold:    30.0, // Higher threshold for trending
		ADXRangingThreshold:     15.0, // Lower threshold for ranging detection
		ATRVolatileMultiplier:   2.0,  // Need 2x average for volatile
		ATRConsolidatingDivisor: 0.3,  // Need < 0.3x for consolidating
		MinRegimeDuration:       1,
		HistoryMaxSize:          50,
	}
	classifier := NewRegimeClassifier(config, nil)

	tests := []struct {
		name       string
		adx        float64
		atr        float64
		atrAverage float64
		expected   MarketRegime
	}{
		{"ADX 25 not trending with strict config", 25.0, 0, 0, RegimeRanging},
		{"ADX 30 is trending with strict config", 30.0, 0, 0, RegimeTrending},
		{"1.5x ATR not volatile with strict config", 10.0, 150.0, 100.0, RegimeRanging},
		{"2x ATR is volatile with strict config", 10.0, 201.0, 100.0, RegimeVolatile}, // 2.01x > 2.0x threshold
		{"0.4x ATR not consolidating with strict config", 10.0, 40.0, 100.0, RegimeRanging},
		{"0.25x ATR is consolidating with strict config", 10.0, 25.0, 100.0, RegimeConsolidating},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyWithIndicators(tt.adx, tt.atr, tt.atrAverage)
			if result != tt.expected {
				t.Errorf("ClassifyWithIndicators(adx=%.2f, atr=%.2f, atrAvg=%.2f) = %s, want %s",
					tt.adx, tt.atr, tt.atrAverage, result, tt.expected)
			}
		})
	}
}

// TestRegimeClassifier_HistoryMaxSize tests that history is trimmed when exceeding max size.
func TestRegimeClassifier_HistoryMaxSize(t *testing.T) {
	config := DefaultRegimeConfig()
	config.HistoryMaxSize = 5
	config.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(config, nil)

	symbol := "ADAUSDT"

	// Generate more changes than max size
	for i := 0; i < 10; i++ {
		var adx float64
		if i%2 == 0 {
			adx = 30.0 // Trending
		} else {
			adx = 15.0 // Ranging
		}
		classifier.ClassifyAndRecord(symbol, adx, 100, 100)
	}

	history := classifier.GetHistory(symbol)
	if len(history) > config.HistoryMaxSize {
		t.Errorf("History length = %d, should be <= %d", len(history), config.HistoryMaxSize)
	}
}

// TestRegimeClassifier_ClearHistory tests clearing history for a symbol.
func TestRegimeClassifier_ClearHistory(t *testing.T) {
	config := DefaultRegimeConfig()
	config.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(config, nil)

	symbol := "DOGEUSDT"

	// Create some history
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	classifier.ClassifyAndRecord(symbol, 15.0, 100, 100)

	// Verify history exists
	if len(classifier.GetHistory(symbol)) == 0 {
		t.Error("History should exist before clearing")
	}

	// Clear history
	classifier.ClearHistory(symbol)

	// Verify history is cleared
	if len(classifier.GetHistory(symbol)) != 0 {
		t.Error("History should be empty after clearing")
	}

	// Verify streak is reset
	regime, streak := classifier.GetCurrentStreak(symbol)
	if streak != 0 {
		t.Errorf("Streak should be 0 after clearing, got %d", streak)
	}
	if regime != RegimeConsolidating {
		t.Errorf("Default regime should be CONSOLIDATING, got %s", regime)
	}
}

// TestRegimeClassifier_GetStrategyForRegime tests strategy recommendations.
func TestRegimeClassifier_GetStrategyForRegime(t *testing.T) {
	tests := []struct {
		regime   MarketRegime
		expected string
	}{
		{RegimeTrending, "trend_following"},
		{RegimeRanging, "mean_reversion"},
		{RegimeVolatile, "breakout"},
		{RegimeConsolidating, "range_trading"},
		{MarketRegime("UNKNOWN"), "mean_reversion"}, // Default
	}

	for _, tt := range tests {
		t.Run(string(tt.regime), func(t *testing.T) {
			result := GetStrategyForRegime(tt.regime)
			if result != tt.expected {
				t.Errorf("GetStrategyForRegime(%s) = %s, want %s", tt.regime, result, tt.expected)
			}
		})
	}
}

// TestRegimeClassifier_ClassifyWithCoinState tests classification from CoinState.
func TestRegimeClassifier_ClassifyWithCoinState(t *testing.T) {
	config := DefaultRegimeConfig()
	classifier := NewRegimeClassifier(config, nil)

	tests := []struct {
		name     string
		state    *CoinState
		expected MarketRegime
	}{
		{
			name:     "Nil state",
			state:    nil,
			expected: RegimeConsolidating,
		},
		{
			name: "High ADX trending",
			state: &CoinState{
				Symbol: "BTCUSDT",
				ADX:    30.0,
			},
			expected: RegimeTrending,
		},
		{
			name: "Low ADX ranging",
			state: &CoinState{
				Symbol: "ETHUSDT",
				ADX:    15.0,
			},
			expected: RegimeRanging,
		},
		{
			name: "Low ADX with high ATR volatile",
			state: &CoinState{
				Symbol:     "SOLUSDT",
				ADX:        15.0,
				ATR:        200.0,
				ATRAverage: 100.0, // ATR is 2x average (> 1.5x threshold)
			},
			expected: RegimeVolatile,
		},
		{
			name: "Low ADX with low ATR consolidating",
			state: &CoinState{
				Symbol:     "ADAUSDT",
				ADX:        15.0,
				ATR:        40.0,
				ATRAverage: 100.0, // ATR is 0.4x average (< 0.5x threshold)
			},
			expected: RegimeConsolidating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.Classify(tt.state)
			if result != tt.expected {
				t.Errorf("Classify() = %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestRegimeClassifier_GetLatestChange tests retrieving the most recent change.
func TestRegimeClassifier_GetLatestChange(t *testing.T) {
	config := DefaultRegimeConfig()
	config.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(config, nil)

	symbol := "LINKUSDT"

	// No changes yet
	if change := classifier.GetLatestChange(symbol); change != nil {
		t.Error("Should return nil when no history exists")
	}

	// Add some changes
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0) // TRENDING
	classifier.ClassifyAndRecord(symbol, 15.0, 100, 100) // RANGING

	latest := classifier.GetLatestChange(symbol)
	if latest == nil {
		t.Fatal("Should return latest change")
	}
	if latest.From != RegimeTrending || latest.To != RegimeRanging {
		t.Errorf("Latest change = %s -> %s, want TRENDING -> RANGING",
			latest.From, latest.To)
	}
}

// TestRegimeClassifier_GetAllKnownSymbols tests listing all tracked symbols.
func TestRegimeClassifier_GetAllKnownSymbols(t *testing.T) {
	config := DefaultRegimeConfig()
	classifier := NewRegimeClassifier(config, nil)

	// No symbols yet
	symbols := classifier.GetAllKnownSymbols()
	if len(symbols) != 0 {
		t.Errorf("Should have 0 symbols, got %d", len(symbols))
	}

	// Add some symbols
	classifier.ClassifyAndRecord("BTCUSDT", 30.0, 0, 0)
	classifier.ClassifyAndRecord("ETHUSDT", 25.0, 0, 0)
	classifier.ClassifyAndRecord("SOLUSDT", 20.0, 0, 0)

	symbols = classifier.GetAllKnownSymbols()
	if len(symbols) != 3 {
		t.Errorf("Should have 3 symbols, got %d", len(symbols))
	}
}

// TestRegimeClassifier_UpdateRegime tests the full update cycle with state.
func TestRegimeClassifier_UpdateRegime(t *testing.T) {
	config := DefaultRegimeConfig()
	config.MinRegimeDuration = 1
	// No StateManager - classification-only mode
	classifier := NewRegimeClassifier(config, nil)

	ctx := context.Background()
	state := &CoinState{
		Symbol: "BTCUSDT",
		ADX:    30.0,
	}

	regime, strategy, err := classifier.UpdateRegime(ctx, "user1", "BTCUSDT", state)
	if err != nil {
		t.Fatalf("UpdateRegime failed: %v", err)
	}

	if regime != RegimeTrending {
		t.Errorf("Regime = %s, want TRENDING", regime)
	}
	if strategy != "trend_following" {
		t.Errorf("Strategy = %s, want trend_following", strategy)
	}

	// Verify state was updated
	if state.Regime != RegimeTrending {
		t.Errorf("State.Regime = %s, want TRENDING", state.Regime)
	}
	if state.ActiveStrategy != "trend_following" {
		t.Errorf("State.ActiveStrategy = %s, want trend_following", state.ActiveStrategy)
	}
}

// TestRegimeClassifier_UpdateRegime_NilState tests error handling for nil state.
func TestRegimeClassifier_UpdateRegime_NilState(t *testing.T) {
	classifier := NewRegimeClassifier(nil, nil)
	ctx := context.Background()

	_, _, err := classifier.UpdateRegime(ctx, "user1", "BTCUSDT", nil)
	if err == nil {
		t.Error("UpdateRegime should return error for nil state")
	}
}

// TestRegimeClassifier_ConcurrentAccess tests thread safety.
func TestRegimeClassifier_ConcurrentAccess(t *testing.T) {
	config := DefaultRegimeConfig()
	config.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(config, nil)

	symbol := "BTCUSDT"

	// Run concurrent classifications
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			adx := float64(15 + id*3) // Different ADX values
			classifier.ClassifyAndRecord(symbol, adx, 100, 100)
			_ = classifier.GetHistory(symbol)
			_, _ = classifier.GetCurrentStreak(symbol)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no panic occurred and data is consistent
	history := classifier.GetHistory(symbol)
	if len(history) == 0 {
		t.Error("History should have entries after concurrent access")
	}
}

// TestDefaultRegimeConfig tests default configuration values.
func TestDefaultRegimeConfig(t *testing.T) {
	config := DefaultRegimeConfig()

	if config.ADXTrendingThreshold != 25.0 {
		t.Errorf("ADXTrendingThreshold = %.2f, want 25.0", config.ADXTrendingThreshold)
	}
	if config.ADXRangingThreshold != 20.0 {
		t.Errorf("ADXRangingThreshold = %.2f, want 20.0", config.ADXRangingThreshold)
	}
	if config.ATRVolatileMultiplier != 1.5 {
		t.Errorf("ATRVolatileMultiplier = %.2f, want 1.5", config.ATRVolatileMultiplier)
	}
	if config.ATRConsolidatingDivisor != 0.5 {
		t.Errorf("ATRConsolidatingDivisor = %.2f, want 0.5", config.ATRConsolidatingDivisor)
	}
	if config.MinRegimeDuration != 3 {
		t.Errorf("MinRegimeDuration = %d, want 3", config.MinRegimeDuration)
	}
	if config.HistoryMaxSize != 100 {
		t.Errorf("HistoryMaxSize = %d, want 100", config.HistoryMaxSize)
	}
}

// TestNewRegimeClassifier_NilConfig tests that nil config uses defaults.
func TestNewRegimeClassifier_NilConfig(t *testing.T) {
	classifier := NewRegimeClassifier(nil, nil)

	if classifier.config == nil {
		t.Fatal("Config should not be nil")
	}
	if classifier.config.ADXTrendingThreshold != 25.0 {
		t.Errorf("Should use default config, ADXTrendingThreshold = %.2f", classifier.config.ADXTrendingThreshold)
	}
}

// TestRegimeClassifier_GetConfig tests the GetConfig method.
func TestRegimeClassifier_GetConfig(t *testing.T) {
	config := &RegimeConfig{
		ADXTrendingThreshold: 30.0,
	}
	classifier := NewRegimeClassifier(config, nil)

	returnedConfig := classifier.GetConfig()
	if returnedConfig.ADXTrendingThreshold != 30.0 {
		t.Errorf("GetConfig returned wrong threshold: %.2f", returnedConfig.ADXTrendingThreshold)
	}
}

// TestRegimeClassifier_ClearAllHistory tests clearing history for all symbols.
func TestRegimeClassifier_ClearAllHistory(t *testing.T) {
	config := DefaultRegimeConfig()
	config.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(config, nil)

	// Create history for multiple symbols
	classifier.ClassifyAndRecord("BTCUSDT", 30.0, 0, 0)
	classifier.ClassifyAndRecord("ETHUSDT", 25.0, 0, 0)
	classifier.ClassifyAndRecord("SOLUSDT", 20.0, 100, 100)

	// Verify history exists for all symbols
	if len(classifier.GetAllKnownSymbols()) != 3 {
		t.Error("Should have 3 symbols before clearing")
	}

	// Clear all history
	classifier.ClearAllHistory()

	// Verify all history is cleared
	symbols := classifier.GetAllKnownSymbols()
	if len(symbols) != 0 {
		t.Errorf("Should have 0 symbols after clearing, got %d", len(symbols))
	}

	// Verify individual histories are empty
	if len(classifier.GetHistory("BTCUSDT")) != 0 {
		t.Error("BTCUSDT history should be empty")
	}
	if len(classifier.GetHistory("ETHUSDT")) != 0 {
		t.Error("ETHUSDT history should be empty")
	}

	// Verify streaks are reset
	regime, streak := classifier.GetCurrentStreak("BTCUSDT")
	if streak != 0 || regime != RegimeConsolidating {
		t.Errorf("BTCUSDT streak should be reset, got (%s, %d)", regime, streak)
	}
}

// TestRegimeClassifier_ChangeReason tests that change reasons are descriptive.
func TestRegimeClassifier_ChangeReason(t *testing.T) {
	config := DefaultRegimeConfig()
	config.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(config, nil)

	symbol := "AVAXUSDT"

	// Initial classification
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0) // TRENDING

	// Change to VOLATILE
	classifier.ClassifyAndRecord(symbol, 15.0, 200.0, 100.0) // VOLATILE

	history := classifier.GetHistory(symbol)
	if len(history) < 2 {
		t.Fatalf("Expected at least 2 history entries, got %d", len(history))
	}

	// Check that reason is not empty
	lastChange := history[len(history)-1]
	if lastChange.Reason == "" {
		t.Error("Change reason should not be empty")
	}

	// Check that indicators are recorded
	if lastChange.Indicators.ADX != 15.0 {
		t.Errorf("Indicators.ADX = %.2f, want 15.0", lastChange.Indicators.ADX)
	}
	if lastChange.Indicators.ATR != 200.0 {
		t.Errorf("Indicators.ATR = %.2f, want 200.0", lastChange.Indicators.ATR)
	}
}
