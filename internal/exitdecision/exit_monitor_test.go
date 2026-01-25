package exitdecision

import (
	"context"
	"testing"
	"time"
)

// ============================================================================
// MOCK POSITION IMPLEMENTATION
// ============================================================================

// MockPosition implements the Position interface for testing.
type MockPosition struct {
	symbol          string
	mode            string
	side            string
	entryPrice      float64
	currentPrice    float64
	originalQty     float64
	remainingQty    float64
	entryTime       time.Time
	tpLevels        []TakeProfitLevel
	currentTPLevel  int
	slPrice         float64
	movedToBE       bool
	trailingActive  bool
	trailingPct     float64
	trailingActPct  float64
	highestPrice    float64
	lowestPrice     float64
	effActive       bool
	peakProfit      float64
	currentProfit   float64
	efficiency      float64
	breakevenPrice  float64   // S3: Breakeven verification
	lastPriceUpdate time.Time // S4: Stale data detection
}

func (m *MockPosition) GetSymbol() string            { return m.symbol }
func (m *MockPosition) GetMode() string              { return m.mode }
func (m *MockPosition) GetSide() string              { return m.side }
func (m *MockPosition) GetEntryPrice() float64       { return m.entryPrice }
func (m *MockPosition) GetCurrentPrice() float64     { return m.currentPrice }
func (m *MockPosition) GetOriginalQty() float64      { return m.originalQty }
func (m *MockPosition) GetRemainingQty() float64     { return m.remainingQty }
func (m *MockPosition) GetEntryTime() time.Time      { return m.entryTime }
func (m *MockPosition) GetTakeProfitLevels() []TakeProfitLevel { return m.tpLevels }
func (m *MockPosition) GetCurrentTPLevel() int       { return m.currentTPLevel }
func (m *MockPosition) GetStopLossPrice() float64    { return m.slPrice }
func (m *MockPosition) HasMovedToBreakeven() bool    { return m.movedToBE }
func (m *MockPosition) IsTrailingActive() bool       { return m.trailingActive }
func (m *MockPosition) GetTrailingPercent() float64  { return m.trailingPct }
func (m *MockPosition) GetTrailingActivationPct() float64 { return m.trailingActPct }
func (m *MockPosition) GetHighestPrice() float64     { return m.highestPrice }
func (m *MockPosition) GetLowestPrice() float64      { return m.lowestPrice }
func (m *MockPosition) IsEfficiencyActive() bool     { return m.effActive }
func (m *MockPosition) GetPeakProfit() float64       { return m.peakProfit }
func (m *MockPosition) GetCurrentProfit() float64    { return m.currentProfit }
func (m *MockPosition) GetEfficiency() float64       { return m.efficiency }

// Safeguard methods (Story 10.1 Phase 2)
func (m *MockPosition) GetBreakevenPrice() float64 {
	if m.breakevenPrice > 0 {
		return m.breakevenPrice
	}
	return m.entryPrice // Default to entry price
}

func (m *MockPosition) GetLastPriceUpdate() time.Time {
	if m.lastPriceUpdate.IsZero() {
		return time.Now() // Default to now for backward compatibility
	}
	return m.lastPriceUpdate
}

// ============================================================================
// EXIT MONITOR TESTS
// ============================================================================

func TestNewExitMonitor(t *testing.T) {
	monitor := NewExitMonitor(nil)
	if monitor == nil {
		t.Fatal("NewExitMonitor returned nil")
	}
	if monitor.config == nil {
		t.Error("Monitor config is nil")
	}

	config := &Config{CheckInterval: 100 * time.Millisecond}
	monitor = NewExitMonitor(config)
	if monitor.config.CheckInterval != 100*time.Millisecond {
		t.Error("Monitor config not set correctly")
	}
}

func TestCheckPosition_NilPosition(t *testing.T) {
	monitor := NewExitMonitor(nil)
	ctx := context.Background()

	signal, err := monitor.CheckPosition(ctx, nil, 100.0, "user1")
	if err == nil {
		t.Error("Expected error for nil position")
	}
	if signal != nil {
		t.Error("Signal should be nil for nil position")
	}
}

func TestCheckPosition_EmptySymbol(t *testing.T) {
	monitor := NewExitMonitor(nil)
	ctx := context.Background()

	pos := &MockPosition{symbol: ""}
	signal, err := monitor.CheckPosition(ctx, pos, 100.0, "user1")
	if err == nil {
		t.Error("Expected error for empty symbol")
	}
	if signal != nil {
		t.Error("Signal should be nil for empty symbol")
	}
}

// ============================================================================
// STOP LOSS TESTS
// ============================================================================

func TestCheckStopLoss_LongPosition_Triggered(t *testing.T) {
	monitor := NewExitMonitor(nil)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		slPrice:    49000.0,
	}

	// Price below SL - should trigger
	signal, err := monitor.CheckPosition(ctx, pos, 48900.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected SL signal")
	}
	if signal.ExitType != ExitTypeSL {
		t.Errorf("Expected SL exit type, got %s", signal.ExitType)
	}
	if signal.Urgency != UrgencyImmediate {
		t.Errorf("Expected immediate urgency, got %s", signal.Urgency)
	}
	if signal.Symbol != "BTCUSDT" {
		t.Errorf("Wrong symbol: %s", signal.Symbol)
	}
}

func TestCheckStopLoss_LongPosition_NotTriggered(t *testing.T) {
	monitor := NewExitMonitor(nil)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		slPrice:    49000.0,
	}

	// Price above SL - should not trigger
	signal, err := monitor.CheckPosition(ctx, pos, 50500.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// No immediate SL signal expected
	if signal != nil && signal.ExitType == ExitTypeSL && signal.Urgency == UrgencyImmediate {
		t.Error("Should not trigger SL when price is above SL")
	}
}

func TestCheckStopLoss_ShortPosition_Triggered(t *testing.T) {
	monitor := NewExitMonitor(nil)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:     "ETHUSDT",
		mode:       "swing",
		side:       "SHORT",
		entryPrice: 3000.0,
		slPrice:    3100.0,
	}

	// Price above SL for short - should trigger
	signal, err := monitor.CheckPosition(ctx, pos, 3150.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected SL signal")
	}
	if signal.ExitType != ExitTypeSL {
		t.Errorf("Expected SL exit type, got %s", signal.ExitType)
	}
	if signal.Urgency != UrgencyImmediate {
		t.Errorf("Expected immediate urgency, got %s", signal.Urgency)
	}
}

func TestCheckStopLoss_ApproachingSL(t *testing.T) {
	monitor := NewExitMonitor(nil)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		slPrice:    49000.0,
	}

	// Price very close to SL (within 0.5%)
	signal, err := monitor.CheckPosition(ctx, pos, 49100.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected warning signal")
	}
	if signal.Urgency != UrgencyWarning {
		t.Errorf("Expected warning urgency, got %s", signal.Urgency)
	}
}

// ============================================================================
// TAKE PROFIT TESTS
// ============================================================================

func TestCheckTakeProfit_LongPosition_Triggered(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false // Disable SL to test TP
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		tpLevels: []TakeProfitLevel{
			{Level: 1, Price: 51000.0, Status: "pending"},
			{Level: 2, Price: 52000.0, Status: "pending"},
		},
		currentTPLevel: 0,
	}

	// Price above TP1 - should trigger
	signal, err := monitor.CheckPosition(ctx, pos, 51100.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected TP signal")
	}
	if signal.ExitType != ExitTypeTP {
		t.Errorf("Expected TP exit type, got %s", signal.ExitType)
	}
	if signal.TPLevel != 1 {
		t.Errorf("Expected TP level 1, got %d", signal.TPLevel)
	}
	if signal.Urgency != UrgencyImmediate {
		t.Errorf("Expected immediate urgency, got %s", signal.Urgency)
	}
}

func TestCheckTakeProfit_ShortPosition_Triggered(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:     "ETHUSDT",
		mode:       "swing",
		side:       "SHORT",
		entryPrice: 3000.0,
		tpLevels: []TakeProfitLevel{
			{Level: 1, Price: 2900.0, Status: "pending"},
		},
		currentTPLevel: 0,
	}

	// Price below TP1 for short - should trigger
	signal, err := monitor.CheckPosition(ctx, pos, 2850.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected TP signal")
	}
	if signal.ExitType != ExitTypeTP {
		t.Errorf("Expected TP exit type, got %s", signal.ExitType)
	}
}

func TestCheckTakeProfit_SkipsHitLevels(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		tpLevels: []TakeProfitLevel{
			{Level: 1, Price: 51000.0, Status: "hit"},
			{Level: 2, Price: 52000.0, Status: "pending"},
		},
		currentTPLevel: 1,
	}

	// Price between TP1 and TP2 - should not trigger (TP1 already hit)
	signal, err := monitor.CheckPosition(ctx, pos, 51500.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// No immediate signal expected
	if signal != nil && signal.ExitType == ExitTypeTP && signal.TPLevel == 1 {
		t.Error("Should not trigger TP1 that is already hit")
	}
}

func TestCheckTakeProfit_ApproachingTP(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		tpLevels: []TakeProfitLevel{
			{Level: 1, Price: 51000.0, Status: "pending"},
		},
		currentTPLevel: 0,
	}

	// Price very close to TP1 (within 0.5%)
	signal, err := monitor.CheckPosition(ctx, pos, 50950.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected monitor signal")
	}
	if signal.Urgency != UrgencyMonitor {
		t.Errorf("Expected monitor urgency, got %s", signal.Urgency)
	}
}

// ============================================================================
// TRAILING STOP TESTS
// ============================================================================

func TestCheckTrailingStop_LongPosition_Triggered(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	config.EnableTPMonitoring = false
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:         "BTCUSDT",
		mode:           "scalp",
		side:           "LONG",
		entryPrice:     50000.0,
		trailingActive: true,
		trailingPct:    2.0,
		highestPrice:   52000.0,
	}

	// Trailing SL for long = highestPrice * (1 - 2%) = 52000 * 0.98 = 50960
	// Price below trailing SL - should trigger
	signal, err := monitor.CheckPosition(ctx, pos, 50900.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected trailing stop signal")
	}
	if signal.ExitType != ExitTypeTrailing {
		t.Errorf("Expected trailing exit type, got %s", signal.ExitType)
	}
	if signal.Urgency != UrgencyImmediate {
		t.Errorf("Expected immediate urgency, got %s", signal.Urgency)
	}
}

func TestCheckTrailingStop_ShortPosition_Triggered(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	config.EnableTPMonitoring = false
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:         "ETHUSDT",
		mode:           "swing",
		side:           "SHORT",
		entryPrice:     3000.0,
		trailingActive: true,
		trailingPct:    2.0,
		lowestPrice:    2800.0,
	}

	// Trailing SL for short = lowestPrice * (1 + 2%) = 2800 * 1.02 = 2856
	// Price above trailing SL - should trigger
	signal, err := monitor.CheckPosition(ctx, pos, 2860.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected trailing stop signal")
	}
	if signal.ExitType != ExitTypeTrailing {
		t.Errorf("Expected trailing exit type, got %s", signal.ExitType)
	}
}

func TestCheckTrailingStop_NotActive(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	config.EnableTPMonitoring = false
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:         "BTCUSDT",
		mode:           "scalp",
		side:           "LONG",
		entryPrice:     50000.0,
		trailingActive: false,
		trailingPct:    2.0,
		highestPrice:   52000.0,
	}

	// Trailing not active - should not trigger
	signal, err := monitor.CheckPosition(ctx, pos, 50900.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal != nil && signal.ExitType == ExitTypeTrailing {
		t.Error("Should not trigger trailing stop when not active")
	}
}

// ============================================================================
// EFFICIENCY EXIT TESTS
// ============================================================================

func TestCheckEfficiencyExit_Triggered(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	config.EnableTPMonitoring = false
	config.EnableTrailingMonitoring = false
	config.DefaultEfficiencyThreshold = 0.5
	// Disable safeguards to test basic efficiency exit behavior
	config.Safeguards = &SafeguardsConfig{
		EnableMinHoldTime:            false,
		EnableWhipsawPrevention:      false,
		EnableBreakevenVerification:  false,
		EnableStaleDataDetection:     false,
		EnableNewEngineFallback:      false,
		BlockModeChangeWithOpenPositions: false,
	}
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:        "BTCUSDT",
		mode:          "scalp",
		side:          "LONG",
		entryPrice:    50000.0,
		effActive:     true,
		peakProfit:    5.0,
		currentProfit: 2.0,
		efficiency:    0.4, // 40% < 50% threshold
	}

	signal, err := monitor.CheckPosition(ctx, pos, 51000.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected efficiency exit signal")
	}
	if signal.ExitType != ExitTypeEfficiency {
		t.Errorf("Expected efficiency exit type, got %s", signal.ExitType)
	}
	if signal.Urgency != UrgencyImmediate {
		t.Errorf("Expected immediate urgency, got %s", signal.Urgency)
	}
}

func TestCheckEfficiencyExit_TriggeredWithSafeguards(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	config.EnableTPMonitoring = false
	config.EnableTrailingMonitoring = false
	config.DefaultEfficiencyThreshold = 0.5
	// Enable safeguards with minimal requirements
	config.Safeguards = &SafeguardsConfig{
		EnableMinHoldTime:            true,
		MinHoldBeforeEfficiencyMins:  map[string]int{"scalp": 1},
		EnableWhipsawPrevention:      true,
		ConsecutiveSignalsRequired:   1, // Only 1 signal required
		EnableBreakevenVerification:  true,
		EnableStaleDataDetection:     true,
		MaxPriceDataAgeSecs:          60,
		EnableNewEngineFallback:      false,
		BlockModeChangeWithOpenPositions: false,
	}
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	// Position that passes all safeguards
	pos := &MockPosition{
		symbol:          "BTCUSDT",
		mode:            "scalp",
		side:            "LONG",
		entryPrice:      50000.0,
		entryTime:       time.Now().Add(-5 * time.Minute), // Past min hold
		effActive:       true,
		peakProfit:      5.0,
		currentProfit:   2.0, // Positive profit
		efficiency:      0.4, // Below threshold
		breakevenPrice:  50000.0,
		lastPriceUpdate: time.Now(), // Fresh data
	}

	// Price above breakeven
	currentPrice := 51000.0

	signal, err := monitor.CheckPosition(ctx, pos, currentPrice, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected efficiency exit signal")
	}
	if signal.ExitType != ExitTypeEfficiency {
		t.Errorf("Expected efficiency exit type, got %s", signal.ExitType)
	}
	if signal.Urgency != UrgencyImmediate {
		t.Errorf("Expected immediate urgency when all safeguards pass, got %s", signal.Urgency)
	}
}

func TestCheckEfficiencyExit_Warning(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	config.EnableTPMonitoring = false
	config.EnableTrailingMonitoring = false
	config.DefaultEfficiencyThreshold = 0.5
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:        "BTCUSDT",
		mode:          "scalp",
		side:          "LONG",
		entryPrice:    50000.0,
		effActive:     true,
		peakProfit:    5.0,
		currentProfit: 2.8,
		efficiency:    0.55, // 55% - between threshold (50%) and warning (60%)
	}

	signal, err := monitor.CheckPosition(ctx, pos, 51400.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected warning signal")
	}
	if signal.ExitType != ExitTypeEfficiency {
		t.Errorf("Expected efficiency exit type, got %s", signal.ExitType)
	}
	if signal.Urgency != UrgencyWarning {
		t.Errorf("Expected warning urgency, got %s", signal.Urgency)
	}
}

func TestCheckEfficiencyExit_NotActive(t *testing.T) {
	config := DefaultConfig()
	config.EnableSLMonitoring = false
	config.EnableTPMonitoring = false
	config.EnableTrailingMonitoring = false
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		effActive:  false,
		efficiency: 0.4,
	}

	signal, err := monitor.CheckPosition(ctx, pos, 51000.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal != nil && signal.ExitType == ExitTypeEfficiency {
		t.Error("Should not trigger efficiency exit when not active")
	}
}

// ============================================================================
// UTILITY FUNCTION TESTS
// ============================================================================

func TestCalculatePnLPercent(t *testing.T) {
	monitor := NewExitMonitor(nil)

	tests := []struct {
		name       string
		entry      float64
		current    float64
		side       string
		expectedPct float64
	}{
		{"Long profit", 100.0, 105.0, "LONG", 5.0},
		{"Long loss", 100.0, 95.0, "LONG", -5.0},
		{"Short profit", 100.0, 95.0, "SHORT", 5.0},
		{"Short loss", 100.0, 105.0, "SHORT", -5.0},
		{"Zero entry", 0.0, 100.0, "LONG", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := monitor.calculatePnLPercent(tt.entry, tt.current, tt.side)
			if result != tt.expectedPct {
				t.Errorf("Expected %.2f%%, got %.2f%%", tt.expectedPct, result)
			}
		})
	}
}

func TestShouldActivateTrailing(t *testing.T) {
	monitor := NewExitMonitor(nil)

	pos := &MockPosition{
		symbol:         "BTCUSDT",
		side:           "LONG",
		entryPrice:     100.0,
		trailingActPct: 2.0, // Activate at 2% profit
	}

	// Below activation threshold
	if monitor.ShouldActivateTrailing(pos, 101.0) {
		t.Error("Should not activate at 1% profit")
	}

	// At activation threshold
	if !monitor.ShouldActivateTrailing(pos, 102.0) {
		t.Error("Should activate at 2% profit")
	}

	// Above activation threshold
	if !monitor.ShouldActivateTrailing(pos, 105.0) {
		t.Error("Should activate at 5% profit")
	}
}

func TestGetMonitoringStatus(t *testing.T) {
	monitor := NewExitMonitor(nil)

	pos := &MockPosition{
		symbol:         "BTCUSDT",
		tpLevels:       []TakeProfitLevel{{Level: 1, Price: 51000}},
		slPrice:        49000.0,
		trailingActive: true,
		trailingPct:    2.0,
		effActive:      true,
		currentTPLevel: 0,
	}

	status := monitor.GetMonitoringStatus(pos)

	if !status["tp_monitoring"].(bool) {
		t.Error("TP monitoring should be enabled")
	}
	if !status["sl_monitoring"].(bool) {
		t.Error("SL monitoring should be enabled")
	}
	if !status["trailing_monitoring"].(bool) {
		t.Error("Trailing monitoring should be enabled")
	}
	if !status["efficiency_monitoring"].(bool) {
		t.Error("Efficiency monitoring should be enabled")
	}
	if status["tp_levels_count"].(int) != 1 {
		t.Error("Wrong TP levels count")
	}
	if status["stop_loss_price"].(float64) != 49000.0 {
		t.Error("Wrong SL price")
	}
}

// ============================================================================
// PRIORITY ORDER TESTS
// ============================================================================

func TestCheckPosition_SLPriorityOverTP(t *testing.T) {
	monitor := NewExitMonitor(nil)
	ctx := context.Background()

	// Position where both SL and TP would trigger
	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		slPrice:    48000.0,
		tpLevels: []TakeProfitLevel{
			{Level: 1, Price: 47000.0, Status: "pending"}, // Below current price
		},
		currentTPLevel: 0,
	}

	// Price below SL
	signal, err := monitor.CheckPosition(ctx, pos, 47500.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal == nil {
		t.Fatal("Expected signal")
	}
	// SL should have priority
	if signal.ExitType != ExitTypeSL {
		t.Errorf("Expected SL (priority), got %s", signal.ExitType)
	}
}

// ============================================================================
// CONFIG-BASED ENABLE/DISABLE TESTS
// ============================================================================

func TestCheckPosition_DisabledMonitoring(t *testing.T) {
	config := &Config{
		EnableTPMonitoring:         false,
		EnableSLMonitoring:         false,
		EnableTrailingMonitoring:   false,
		EnableEfficiencyMonitoring: false,
	}
	monitor := NewExitMonitor(config)
	ctx := context.Background()

	pos := &MockPosition{
		symbol:         "BTCUSDT",
		mode:           "scalp",
		side:           "LONG",
		entryPrice:     50000.0,
		slPrice:        49000.0,
		tpLevels:       []TakeProfitLevel{{Level: 1, Price: 51000.0}},
		trailingActive: true,
		trailingPct:    2.0,
		highestPrice:   52000.0,
		effActive:      true,
		efficiency:     0.3,
	}

	// All monitoring disabled - should not trigger any signal
	signal, err := monitor.CheckPosition(ctx, pos, 48000.0, "user1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if signal != nil {
		t.Errorf("Should not trigger any signal when all monitoring disabled: %+v", signal)
	}
}
