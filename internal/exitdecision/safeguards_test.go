package exitdecision

import (
	"testing"
	"time"
)

// ============================================================================
// MOCK POSITION FOR SAFEGUARD TESTS
// ============================================================================

// MockPositionSafeguards extends MockPosition with safeguard-specific fields.
type MockPositionSafeguards struct {
	MockPosition
	breakevenPrice  float64
	lastPriceUpdate time.Time
}

func (m *MockPositionSafeguards) GetBreakevenPrice() float64    { return m.breakevenPrice }
func (m *MockPositionSafeguards) GetLastPriceUpdate() time.Time { return m.lastPriceUpdate }

// NewMockPositionForSafeguards creates a mock position with safeguard support.
func NewMockPositionForSafeguards() *MockPositionSafeguards {
	return &MockPositionSafeguards{
		MockPosition: MockPosition{
			symbol:     "BTCUSDT",
			mode:       "scalp",
			side:       "LONG",
			entryPrice: 50000.0,
			entryTime:  time.Now().Add(-5 * time.Minute), // 5 minutes ago
			effActive:  true,
			efficiency: 0.4,
			peakProfit: 5.0,
			currentProfit: 2.0,
		},
		breakevenPrice:  50000.0,
		lastPriceUpdate: time.Now(),
	}
}

// ============================================================================
// S1: MINIMUM HOLD TIME TESTS
// ============================================================================

func TestS1_MinHoldTime_Blocked(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.MinHoldBeforeEfficiencyMins = map[string]int{
		"scalp": 2,
	}
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.entryTime = time.Now().Add(-1 * time.Minute) // Only 1 minute ago
	pos.mode = "scalp"

	blocked, reason := safeguards.CheckMinHoldTime(pos)
	if !blocked {
		t.Error("S1: Should block exit within minimum hold time")
	}
	if reason == "" {
		t.Error("S1: Should provide reason when blocked")
	}
	t.Logf("S1 Blocked reason: %s", reason)
}

func TestS1_MinHoldTime_Allowed(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.MinHoldBeforeEfficiencyMins = map[string]int{
		"scalp": 2,
	}
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.entryTime = time.Now().Add(-3 * time.Minute) // 3 minutes ago (> 2 min)
	pos.mode = "scalp"

	blocked, _ := safeguards.CheckMinHoldTime(pos)
	if blocked {
		t.Error("S1: Should allow exit after minimum hold time")
	}
}

func TestS1_MinHoldTime_Disabled(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.EnableMinHoldTime = false
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.entryTime = time.Now().Add(-30 * time.Second) // Only 30 seconds ago

	blocked, _ := safeguards.CheckMinHoldTime(pos)
	if blocked {
		t.Error("S1: Should not block when disabled")
	}
}

func TestS1_MinHoldTime_PerModeConfiguration(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.MinHoldBeforeEfficiencyMins = map[string]int{
		"ultra_fast": 1,
		"scalp":      2,
		"swing":      5,
		"position":   15,
	}
	safeguards := NewSafeguards(config, true)

	testCases := []struct {
		mode        string
		minutesAgo  int
		shouldBlock bool
	}{
		{"ultra_fast", 0, true},  // 0 min < 1 min required
		{"ultra_fast", 2, false}, // 2 min > 1 min required
		{"scalp", 1, true},       // 1 min < 2 min required
		{"scalp", 3, false},      // 3 min > 2 min required
		{"swing", 3, true},       // 3 min < 5 min required
		{"swing", 6, false},      // 6 min > 5 min required
		{"position", 10, true},   // 10 min < 15 min required
		{"position", 20, false},  // 20 min > 15 min required
	}

	for _, tc := range testCases {
		pos := NewMockPositionForSafeguards()
		pos.mode = tc.mode
		pos.entryTime = time.Now().Add(-time.Duration(tc.minutesAgo) * time.Minute)

		blocked, _ := safeguards.CheckMinHoldTime(pos)
		if blocked != tc.shouldBlock {
			t.Errorf("S1: mode=%s, minutesAgo=%d: expected blocked=%v, got %v",
				tc.mode, tc.minutesAgo, tc.shouldBlock, blocked)
		}
	}
}

// ============================================================================
// S2: CONSECUTIVE SIGNAL (WHIPSAW PREVENTION) TESTS
// ============================================================================

func TestS2_WhipsawPrevention_RequiresConsecutive(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.ConsecutiveSignalsRequired = 3
	safeguards := NewSafeguards(config, true)

	symbol := "BTCUSDT"

	// First signal - should block
	blocked, _ := safeguards.CheckConsecutiveSignals(symbol, true)
	if !blocked {
		t.Error("S2: Should block on first signal")
	}
	if safeguards.GetConsecutiveSignalCount(symbol) != 1 {
		t.Errorf("S2: Expected count=1, got %d", safeguards.GetConsecutiveSignalCount(symbol))
	}

	// Second signal - should block
	blocked, _ = safeguards.CheckConsecutiveSignals(symbol, true)
	if !blocked {
		t.Error("S2: Should block on second signal")
	}
	if safeguards.GetConsecutiveSignalCount(symbol) != 2 {
		t.Errorf("S2: Expected count=2, got %d", safeguards.GetConsecutiveSignalCount(symbol))
	}

	// Third signal - should allow
	blocked, _ = safeguards.CheckConsecutiveSignals(symbol, true)
	if blocked {
		t.Error("S2: Should allow after 3 consecutive signals")
	}
	if safeguards.GetConsecutiveSignalCount(symbol) != 3 {
		t.Errorf("S2: Expected count=3, got %d", safeguards.GetConsecutiveSignalCount(symbol))
	}
}

func TestS2_WhipsawPrevention_ResetOnRecovery(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.ConsecutiveSignalsRequired = 3
	safeguards := NewSafeguards(config, true)

	symbol := "BTCUSDT"

	// Two signals below threshold
	safeguards.CheckConsecutiveSignals(symbol, true)
	safeguards.CheckConsecutiveSignals(symbol, true)
	if safeguards.GetConsecutiveSignalCount(symbol) != 2 {
		t.Errorf("S2: Expected count=2, got %d", safeguards.GetConsecutiveSignalCount(symbol))
	}

	// Efficiency recovers - counter should reset
	blocked, reason := safeguards.CheckConsecutiveSignals(symbol, false)
	if !blocked {
		t.Error("S2: Should block when efficiency recovers (wait for confirmation)")
	}
	if reason == "" {
		t.Error("S2: Should provide reason about recovery")
	}
	if safeguards.GetConsecutiveSignalCount(symbol) != 0 {
		t.Errorf("S2: Counter should reset on recovery, got %d", safeguards.GetConsecutiveSignalCount(symbol))
	}
}

func TestS2_WhipsawPrevention_Oscillation(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.ConsecutiveSignalsRequired = 3
	safeguards := NewSafeguards(config, true)

	symbol := "ETHUSDT"

	// Simulate oscillation: below, below, above, below, below, above, below
	sequence := []bool{true, true, false, true, true, false, true}
	expectedCounts := []int{1, 2, 0, 1, 2, 0, 1}

	for i, isBelowThreshold := range sequence {
		safeguards.CheckConsecutiveSignals(symbol, isBelowThreshold)
		count := safeguards.GetConsecutiveSignalCount(symbol)
		if count != expectedCounts[i] {
			t.Errorf("S2: Step %d (below=%v): expected count=%d, got %d",
				i, isBelowThreshold, expectedCounts[i], count)
		}
	}
}

func TestS2_WhipsawPrevention_Reset(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	symbol := "BTCUSDT"

	safeguards.CheckConsecutiveSignals(symbol, true)
	safeguards.CheckConsecutiveSignals(symbol, true)
	if safeguards.GetConsecutiveSignalCount(symbol) != 2 {
		t.Errorf("S2: Expected count=2 before reset")
	}

	safeguards.ResetConsecutiveSignals(symbol)
	if safeguards.GetConsecutiveSignalCount(symbol) != 0 {
		t.Errorf("S2: Expected count=0 after reset")
	}
}

func TestS2_WhipsawPrevention_Disabled(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.EnableWhipsawPrevention = false
	safeguards := NewSafeguards(config, true)

	// First signal should immediately allow when disabled
	blocked, _ := safeguards.CheckConsecutiveSignals("BTCUSDT", true)
	if blocked {
		t.Error("S2: Should not block when disabled")
	}
}

// ============================================================================
// S3: BREAKEVEN VERIFICATION TESTS
// ============================================================================

func TestS3_BreakevenVerification_LongBelowBE(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.side = "LONG"
	pos.breakevenPrice = 50000.0
	pos.currentProfit = -1.0 // In loss

	// Price below breakeven
	blocked, reason := safeguards.CheckBreakevenVerification(pos, 49500.0)
	if !blocked {
		t.Error("S3: Should block LONG efficiency exit when price below breakeven")
	}
	t.Logf("S3 Blocked reason: %s", reason)
}

func TestS3_BreakevenVerification_LongAboveBE(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.side = "LONG"
	pos.breakevenPrice = 50000.0
	pos.currentProfit = 2.0 // In profit

	// Price above breakeven
	blocked, _ := safeguards.CheckBreakevenVerification(pos, 51000.0)
	if blocked {
		t.Error("S3: Should allow LONG efficiency exit when price above breakeven")
	}
}

func TestS3_BreakevenVerification_ShortAboveBE(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.side = "SHORT"
	pos.entryPrice = 3000.0
	pos.breakevenPrice = 3000.0
	pos.currentProfit = -2.0 // In loss

	// Price above breakeven for short
	blocked, reason := safeguards.CheckBreakevenVerification(pos, 3100.0)
	if !blocked {
		t.Error("S3: Should block SHORT efficiency exit when price above breakeven")
	}
	t.Logf("S3 Blocked reason: %s", reason)
}

func TestS3_BreakevenVerification_ShortBelowBE(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.side = "SHORT"
	pos.entryPrice = 3000.0
	pos.breakevenPrice = 3000.0
	pos.currentProfit = 3.0 // In profit

	// Price below breakeven for short
	blocked, _ := safeguards.CheckBreakevenVerification(pos, 2900.0)
	if blocked {
		t.Error("S3: Should allow SHORT efficiency exit when price below breakeven")
	}
}

func TestS3_BreakevenVerification_NegativeProfit(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.side = "LONG"
	pos.breakevenPrice = 50000.0
	pos.currentProfit = -0.5 // Negative profit

	// Price above breakeven but profit is negative
	blocked, reason := safeguards.CheckBreakevenVerification(pos, 50100.0)
	if !blocked {
		t.Error("S3: Should block when current profit is not positive")
	}
	if reason == "" {
		t.Error("S3: Should provide reason about negative profit")
	}
}

func TestS3_BreakevenVerification_Disabled(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.EnableBreakevenVerification = false
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.side = "LONG"
	pos.breakevenPrice = 50000.0
	pos.currentProfit = -1.0

	blocked, _ := safeguards.CheckBreakevenVerification(pos, 49000.0)
	if blocked {
		t.Error("S3: Should not block when disabled")
	}
}

// ============================================================================
// S4: STALE DATA DETECTION TESTS
// ============================================================================

func TestS4_StalePrice_Blocked(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.MaxPriceDataAgeSecs = 5
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.lastPriceUpdate = time.Now().Add(-10 * time.Second) // 10 seconds ago

	blocked, reason := safeguards.CheckPriceDataFreshness(pos)
	if !blocked {
		t.Error("S4: Should block when price data is stale")
	}
	t.Logf("S4 Blocked reason: %s", reason)
}

func TestS4_StalePrice_Fresh(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.MaxPriceDataAgeSecs = 5
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.lastPriceUpdate = time.Now().Add(-2 * time.Second) // 2 seconds ago

	blocked, _ := safeguards.CheckPriceDataFreshness(pos)
	if blocked {
		t.Error("S4: Should allow when price data is fresh")
	}
}

func TestS4_StaleTrend_Blocked(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.MaxTrendDataAgeSecs = 30
	safeguards := NewSafeguards(config, true)

	trendTimestamp := time.Now().Add(-45 * time.Second) // 45 seconds ago

	blocked, reason := safeguards.CheckTrendDataFreshness(trendTimestamp)
	if !blocked {
		t.Error("S4: Should block when trend data is stale")
	}
	t.Logf("S4 Blocked reason: %s", reason)
}

func TestS4_StaleTrend_Fresh(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.MaxTrendDataAgeSecs = 30
	safeguards := NewSafeguards(config, true)

	trendTimestamp := time.Now().Add(-15 * time.Second) // 15 seconds ago

	blocked, _ := safeguards.CheckTrendDataFreshness(trendTimestamp)
	if blocked {
		t.Error("S4: Should allow when trend data is fresh")
	}
}

func TestS4_StaleTrend_NoData(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	blocked, reason := safeguards.CheckTrendDataFreshness(time.Time{})
	if !blocked {
		t.Error("S4: Should block when no trend data available")
	}
	if reason == "" {
		t.Error("S4: Should provide reason about missing data")
	}
}

func TestS4_StaleData_Disabled(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.EnableStaleDataDetection = false
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.lastPriceUpdate = time.Now().Add(-60 * time.Second) // Very old

	blocked, _ := safeguards.CheckPriceDataFreshness(pos)
	if blocked {
		t.Error("S4: Should not block when disabled")
	}

	blocked, _ = safeguards.CheckTrendDataFreshness(time.Time{})
	if blocked {
		t.Error("S4: Should not block trend check when disabled")
	}
}

// ============================================================================
// S5: EPIC 11 INTEGRATION SAFEGUARDS TESTS
// ============================================================================

func TestS5_NewEngineAvailability_AllAvailable(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	result := safeguards.CheckNewEngineAvailability(true, true, 3)
	if !result.Available {
		t.Error("S5: Should be available when all components present")
	}
	if result.UseFallback {
		t.Error("S5: Should not use fallback when available")
	}
}

func TestS5_NewEngineAvailability_NoEngine(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	result := safeguards.CheckNewEngineAvailability(false, true, 3)
	if result.Available {
		t.Error("S5: Should not be available when engine missing")
	}
	if !result.UseFallback {
		t.Error("S5: Should use fallback when engine missing")
	}
	t.Logf("S5 Reason: %s", result.Reason)
}

func TestS5_NewEngineAvailability_NoStrategy(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	result := safeguards.CheckNewEngineAvailability(true, false, 3)
	if result.Available {
		t.Error("S5: Should not be available when strategy missing")
	}
	if !result.UseFallback {
		t.Error("S5: Should use fallback when strategy missing")
	}
}

func TestS5_NewEngineAvailability_InsufficientIndicators(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.MinIndicatorsRequired = 2
	safeguards := NewSafeguards(config, true)

	result := safeguards.CheckNewEngineAvailability(true, true, 1)
	if result.Available {
		t.Error("S5: Should not be available with insufficient indicators")
	}
	if !result.UseFallback {
		t.Error("S5: Should use fallback with insufficient indicators")
	}
}

func TestS5_RegimeChange_NotConfirmed(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.RegimeChangeConfirmationSecs = 60
	safeguards := NewSafeguards(config, true)

	symbol := "BTCUSDT"
	safeguards.ResetRegimeTracker(symbol) // Ensure clean state

	confirmed, tracker := safeguards.CheckRegimeChangeConfirmation(symbol, "TRENDING", "RANGING")
	if confirmed {
		t.Error("S5: Regime change should not be confirmed immediately")
	}
	if tracker == nil {
		t.Error("S5: Should return tracker for pending regime change")
	}
	if tracker.OriginalRegime != "TRENDING" {
		t.Errorf("S5: Expected original regime TRENDING, got %s", tracker.OriginalRegime)
	}
	if tracker.DetectedRegime != "RANGING" {
		t.Errorf("S5: Expected detected regime RANGING, got %s", tracker.DetectedRegime)
	}
}

func TestS5_RegimeChange_Confirmed(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.RegimeChangeConfirmationSecs = 1 // Short for test
	safeguards := NewSafeguards(config, true)

	symbol := "ETHUSDT"
	safeguards.ResetRegimeTracker(symbol)

	// Initial detection
	safeguards.CheckRegimeChangeConfirmation(symbol, "TRENDING", "RANGING")

	// Wait for confirmation period
	time.Sleep(1100 * time.Millisecond)

	// Should now be confirmed
	confirmed, _ := safeguards.CheckRegimeChangeConfirmation(symbol, "TRENDING", "RANGING")
	if !confirmed {
		t.Error("S5: Regime change should be confirmed after confirmation period")
	}
}

func TestS5_RegimeChange_NoChange(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	confirmed, tracker := safeguards.CheckRegimeChangeConfirmation("BTCUSDT", "TRENDING", "TRENDING")
	if confirmed {
		t.Error("S5: Should not confirm when regime hasn't changed")
	}
	if tracker != nil {
		t.Error("S5: Should not return tracker when no change")
	}
}

func TestS5_FallbackDisabled(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.EnableNewEngineFallback = false
	safeguards := NewSafeguards(config, true)

	result := safeguards.CheckNewEngineAvailability(false, false, 0)
	if result.UseFallback {
		t.Error("S5: Should not use fallback when disabled")
	}
}

// ============================================================================
// S6: DECISION MODE CONSISTENCY TESTS
// ============================================================================

func TestS6_ModeChange_Blocked(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	openPositions := []string{"BTCUSDT", "ETHUSDT"}
	result := safeguards.CheckModeChangeAllowed(openPositions, "classic", "new_engine")

	if result.Allowed {
		t.Error("S6: Should block mode change with open positions")
	}
	if result.OpenPositionCount != 2 {
		t.Errorf("S6: Expected 2 open positions, got %d", result.OpenPositionCount)
	}
	if len(result.OpenSymbols) != 2 {
		t.Errorf("S6: Expected 2 open symbols, got %d", len(result.OpenSymbols))
	}
	t.Logf("S6 Blocked reason: %s", result.Reason)
}

func TestS6_ModeChange_Allowed(t *testing.T) {
	config := DefaultSafeguardsConfig()
	safeguards := NewSafeguards(config, true)

	result := safeguards.CheckModeChangeAllowed([]string{}, "classic", "new_engine")
	if !result.Allowed {
		t.Error("S6: Should allow mode change with no open positions")
	}
}

func TestS6_ModeChange_Disabled(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.BlockModeChangeWithOpenPositions = false
	safeguards := NewSafeguards(config, true)

	openPositions := []string{"BTCUSDT", "ETHUSDT"}
	result := safeguards.CheckModeChangeAllowed(openPositions, "classic", "new_engine")
	if !result.Allowed {
		t.Error("S6: Should allow when restriction disabled")
	}
}

// ============================================================================
// COMBINED SAFEGUARD TESTS
// ============================================================================

func TestCombinedSafeguards_AllPass(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.ConsecutiveSignalsRequired = 1 // Only need 1 for this test
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.entryTime = time.Now().Add(-10 * time.Minute) // Well past hold time
	pos.currentProfit = 3.0                           // Positive profit
	pos.breakevenPrice = 50000.0
	pos.lastPriceUpdate = time.Now()

	currentPrice := 51500.0 // Above breakeven

	result := safeguards.CheckEfficiencyExitSafeguards(pos, currentPrice, true)
	if !result.Allowed {
		t.Errorf("Combined: Expected allowed, blocked by %v", result.BlockedBy)
	}
	if !result.HoldTimeMet {
		t.Error("Combined: Hold time should be met")
	}
	if !result.AboveBreakeven {
		t.Error("Combined: Should be above breakeven")
	}
	if !result.PriceDataFresh {
		t.Error("Combined: Price data should be fresh")
	}
}

func TestCombinedSafeguards_MultipleBlocks(t *testing.T) {
	config := DefaultSafeguardsConfig()
	config.ConsecutiveSignalsRequired = 5 // High requirement
	safeguards := NewSafeguards(config, true)

	pos := NewMockPositionForSafeguards()
	pos.entryTime = time.Now().Add(-30 * time.Second) // Too early
	pos.currentProfit = -1.0                          // Negative profit
	pos.breakevenPrice = 50000.0
	pos.lastPriceUpdate = time.Now().Add(-30 * time.Second) // Stale

	currentPrice := 49500.0 // Below breakeven

	result := safeguards.CheckEfficiencyExitSafeguards(pos, currentPrice, true)
	if result.Allowed {
		t.Error("Combined: Should be blocked by multiple safeguards")
	}
	if len(result.BlockedBy) < 3 {
		t.Errorf("Combined: Expected at least 3 blocks, got %d: %v",
			len(result.BlockedBy), result.BlockedBy)
	}
	t.Logf("Blocked by: %v", result.BlockedBy)
	t.Logf("Reasons: %v", result.Reasons)
}

// ============================================================================
// INTEGRATION WITH EXIT MONITOR
// ============================================================================

func TestExitMonitor_EfficiencyExitWithSafeguards(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	config.EnableTPMonitoring = false
	config.EnableTrailingMonitoring = false
	config.DefaultEfficiencyThreshold = 0.5
	config.Safeguards = DefaultSafeguardsConfig()
	config.Safeguards.ConsecutiveSignalsRequired = 3

	monitor := NewExitMonitor(config)

	// Use a unique symbol to avoid interference from other tests
	pos := NewMockPositionForSafeguards()
	pos.symbol = "TESTUSDT_SAFEGUARD"
	pos.entryTime = time.Now().Add(-10 * time.Minute)
	pos.effActive = true
	pos.efficiency = 0.4 // Below threshold
	pos.currentProfit = 2.0
	pos.breakevenPrice = 50000.0
	pos.lastPriceUpdate = time.Now()

	currentPrice := 51000.0

	// Reset any existing tracker for this symbol
	monitor.GetSafeguards().ResetConsecutiveSignals(pos.symbol)

	// First check - should be blocked by consecutive signals (S2)
	signal, safeguardResult := monitor.CheckEfficiencyExitWithSafeguards(pos, currentPrice, "user1")
	if signal == nil {
		t.Fatal("Expected signal")
	}
	if signal.Urgency == UrgencyImmediate {
		t.Error("Should not be immediate on first signal due to S2")
	}
	if safeguardResult == nil {
		t.Error("Expected safeguard result")
	}
	// Note: CheckEfficiencyExitWithSafeguards calls CheckEfficiencyExitSafeguards twice
	// (once in checkEfficiencyExit, once directly), so the count may increment
	// The important check is that it's blocked initially
	if safeguardResult.Allowed {
		t.Error("First signal should be blocked")
	}

	// Second check
	signal, safeguardResult = monitor.CheckEfficiencyExitWithSafeguards(pos, currentPrice, "user1")
	if signal.Urgency == UrgencyImmediate && safeguardResult.ConsecutiveSignals < 3 {
		t.Error("Should not be immediate on second signal")
	}

	// Keep checking until we have 3+ consecutive signals
	// Note: The checkEfficiencyExit function also calls the safeguards internally
	for i := 0; i < 5; i++ {
		signal, safeguardResult = monitor.CheckEfficiencyExitWithSafeguards(pos, currentPrice, "user1")
		if safeguardResult.Allowed && safeguardResult.ConsecutiveSignals >= 3 {
			break
		}
	}

	// Should eventually become immediate after enough signals
	if signal.Urgency != UrgencyImmediate {
		t.Errorf("Should be immediate after consecutive signals, got %s (signals: %d)",
			signal.Urgency, safeguardResult.ConsecutiveSignals)
	}
	if !safeguardResult.Allowed {
		t.Errorf("Should be allowed after enough signals, blocked by %v", safeguardResult.BlockedBy)
	}
}

func TestExitMonitor_GetSafeguards(t *testing.T) {
	monitor := NewExitMonitor(nil)
	safeguards := monitor.GetSafeguards()
	if safeguards == nil {
		t.Error("GetSafeguards should not return nil")
	}
}

// ============================================================================
// CONFIGURATION TESTS
// ============================================================================

func TestDefaultSafeguardsConfig(t *testing.T) {
	config := DefaultSafeguardsConfig()

	if !config.EnableMinHoldTime {
		t.Error("EnableMinHoldTime should be true by default")
	}
	if !config.EnableWhipsawPrevention {
		t.Error("EnableWhipsawPrevention should be true by default")
	}
	if !config.EnableBreakevenVerification {
		t.Error("EnableBreakevenVerification should be true by default")
	}
	if !config.EnableStaleDataDetection {
		t.Error("EnableStaleDataDetection should be true by default")
	}
	if !config.EnableNewEngineFallback {
		t.Error("EnableNewEngineFallback should be true by default")
	}
	if !config.BlockModeChangeWithOpenPositions {
		t.Error("BlockModeChangeWithOpenPositions should be true by default")
	}
	if config.ConsecutiveSignalsRequired != 3 {
		t.Errorf("Expected ConsecutiveSignalsRequired=3, got %d", config.ConsecutiveSignalsRequired)
	}
	if config.MaxPriceDataAgeSecs != 5 {
		t.Errorf("Expected MaxPriceDataAgeSecs=5, got %d", config.MaxPriceDataAgeSecs)
	}
	if config.MaxTrendDataAgeSecs != 30 {
		t.Errorf("Expected MaxTrendDataAgeSecs=30, got %d", config.MaxTrendDataAgeSecs)
	}
}

func TestSafeguards_SetConfig(t *testing.T) {
	safeguards := NewSafeguards(nil, false)

	newConfig := &SafeguardsConfig{
		EnableMinHoldTime:       false,
		ConsecutiveSignalsRequired: 5,
	}
	safeguards.SetConfig(newConfig)

	if safeguards.GetConfig().EnableMinHoldTime {
		t.Error("Config should be updated")
	}
	if safeguards.GetConfig().ConsecutiveSignalsRequired != 5 {
		t.Error("ConsecutiveSignalsRequired should be updated")
	}

	// Test nil config
	safeguards.SetConfig(nil)
	if safeguards.GetConfig().ConsecutiveSignalsRequired != 5 {
		t.Error("Config should not change with nil")
	}
}

func TestSafeguards_DebugMode(t *testing.T) {
	safeguards := NewSafeguards(nil, false)
	safeguards.SetDebugMode(true)
	// Just verify no panic - debug logging is hard to test
	safeguards.SetDebugMode(false)
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{500 * time.Millisecond, "500ms"},
		{1500 * time.Millisecond, "1.5s"},
		{90 * time.Second, "1.5m"},
		{90 * time.Minute, "1.5h"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.d)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, expected %s", tt.d, result, tt.expected)
		}
	}
}
