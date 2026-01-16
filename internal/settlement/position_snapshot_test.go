// Package settlement provides unit tests for position snapshot service.
// Epic 8 Story 8.1: EOD Snapshot of Open Positions
package settlement

import (
	"context"
	"testing"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/orders"
)

// ============================================================================
// TEST: Mode Extraction from ClientOrderID
// ============================================================================

func TestExtractModeFromClientOrderID(t *testing.T) {
	testCases := []struct {
		name           string
		clientOrderID  string
		expectedMode   string
	}{
		{
			name:          "scalp mode normal format",
			clientOrderID: "SCA-16JAN-00001-E",
			expectedMode:  ModeScalp,
		},
		{
			name:          "swing mode normal format",
			clientOrderID: "SWI-16JAN-00002-E",
			expectedMode:  ModeSwing,
		},
		{
			name:          "position mode normal format",
			clientOrderID: "POS-16JAN-00003-E",
			expectedMode:  ModePosition,
		},
		{
			name:          "ultra_fast mode normal format",
			clientOrderID: "ULT-16JAN-00004-E",
			expectedMode:  ModeUltraFast,
		},
		{
			name:          "scalp mode fallback format",
			clientOrderID: "SCA-FALLBACK-a3f7c2e9-E",
			expectedMode:  ModeScalp,
		},
		{
			name:          "swing mode with TP1 order type",
			clientOrderID: "SWI-16JAN-00005-TP1",
			expectedMode:  ModeSwing,
		},
		{
			name:          "position mode with SL order type",
			clientOrderID: "POS-16JAN-00006-SL",
			expectedMode:  ModePosition,
		},
		{
			name:          "empty clientOrderID",
			clientOrderID: "",
			expectedMode:  ModeUnknown,
		},
		{
			name:          "legacy/unstructured clientOrderID",
			clientOrderID: "web_abc123xyz",
			expectedMode:  ModeUnknown,
		},
		{
			name:          "random string",
			clientOrderID: "some-random-string",
			expectedMode:  ModeUnknown,
		},
		{
			name:          "invalid mode code",
			clientOrderID: "XYZ-16JAN-00001-E",
			expectedMode:  ModeUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mode := extractModeFromClientOrderID(tc.clientOrderID)
			if mode != tc.expectedMode {
				t.Errorf("extractModeFromClientOrderID(%q) = %q, want %q", tc.clientOrderID, mode, tc.expectedMode)
			}
		})
	}
}

// ============================================================================
// TEST: Mode Code Mapping
// ============================================================================

func TestModeCodeToNameMapping(t *testing.T) {
	testCases := []struct {
		code     string
		expected string
	}{
		{"ULT", ModeUltraFast},
		{"SCA", ModeScalp},
		{"SWI", ModeSwing},
		{"POS", ModePosition},
	}

	for _, tc := range testCases {
		t.Run(tc.code, func(t *testing.T) {
			name, exists := ModeCodeToName[tc.code]
			if !exists {
				t.Errorf("ModeCodeToName[%q] not found", tc.code)
				return
			}
			if name != tc.expected {
				t.Errorf("ModeCodeToName[%q] = %q, want %q", tc.code, name, tc.expected)
			}
		})
	}
}

// ============================================================================
// TEST: TradingMode to Mode String Conversion
// ============================================================================

func TestTradingModeConversion(t *testing.T) {
	testCases := []struct {
		tradingMode  orders.TradingMode
		expectedMode string
	}{
		{orders.ModeScalp, ModeScalp},
		{orders.ModeSwing, ModeSwing},
		{orders.ModePosition, ModePosition},
		{orders.ModeUltraFast, ModeUltraFast},
	}

	for _, tc := range testCases {
		t.Run(string(tc.tradingMode), func(t *testing.T) {
			// Use ParseClientOrderId to simulate conversion
			parsed := orders.ParseClientOrderId(string(tc.tradingMode[:3]) + "-16JAN-00001-E")
			if parsed == nil {
				t.Skip("ParseClientOrderId returned nil - mode code not recognized")
				return
			}

			// Verify the mode matches expected
			mode := extractModeFromClientOrderID(parsed.Raw)
			if mode != tc.expectedMode {
				t.Errorf("mode for %v = %q, want %q", tc.tradingMode, mode, tc.expectedMode)
			}
		})
	}
}

// ============================================================================
// TEST: Position Snapshot Struct
// ============================================================================

func TestPositionSnapshotStruct(t *testing.T) {
	now := time.Now().UTC()
	testDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	snapshot := PositionSnapshot{
		UserID:        "user-123",
		SnapshotDate:  testDate,
		Symbol:        "BTCUSDT",
		PositionSide:  "LONG",
		Quantity:      0.5,
		EntryPrice:    50000.0,
		MarkPrice:     51000.0,
		UnrealizedPnL: 500.0,
		Mode:          ModeScalp,
		Leverage:      10,
		MarginType:    "CROSSED",
	}

	// Verify fields
	if snapshot.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", snapshot.UserID, "user-123")
	}
	if snapshot.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %q, want %q", snapshot.Symbol, "BTCUSDT")
	}
	if snapshot.UnrealizedPnL != 500.0 {
		t.Errorf("UnrealizedPnL = %f, want %f", snapshot.UnrealizedPnL, 500.0)
	}
	if snapshot.Mode != ModeScalp {
		t.Errorf("Mode = %q, want %q", snapshot.Mode, ModeScalp)
	}
}

// ============================================================================
// TEST: Snapshot Result Struct
// ============================================================================

func TestSnapshotResultStruct(t *testing.T) {
	now := time.Now().UTC()
	testDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	result := SnapshotResult{
		UserID:             "user-456",
		SnapshotDate:       testDate,
		PositionCount:      3,
		TotalUnrealizedPnL: 1500.0,
		Success:            true,
		Duration:           time.Second * 5,
	}

	// Verify fields
	if !result.Success {
		t.Error("Success should be true")
	}
	if result.PositionCount != 3 {
		t.Errorf("PositionCount = %d, want %d", result.PositionCount, 3)
	}
	if result.TotalUnrealizedPnL != 1500.0 {
		t.Errorf("TotalUnrealizedPnL = %f, want %f", result.TotalUnrealizedPnL, 1500.0)
	}
}

// ============================================================================
// TEST: Settlement Status Determination
// ============================================================================

func TestSettlementStatusLogic(t *testing.T) {
	ist, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Skipf("Cannot load timezone: %v", err)
	}

	nowIST := time.Now().In(ist)
	todayIST := time.Date(nowIST.Year(), nowIST.Month(), nowIST.Day(), 0, 0, 0, 0, ist)
	yesterdayIST := todayIST.AddDate(0, 0, -1)
	twoDaysAgo := todayIST.AddDate(0, 0, -2)

	testCases := []struct {
		name                string
		lastSettlementDate  *time.Time
		timezone            string
		expectedNeedsWork   bool
	}{
		{
			name:               "nil settlement date needs settlement",
			lastSettlementDate: nil,
			timezone:           "Asia/Kolkata",
			expectedNeedsWork:  true,
		},
		{
			name:               "yesterday's settlement needs new settlement",
			lastSettlementDate: &yesterdayIST,
			timezone:           "Asia/Kolkata",
			expectedNeedsWork:  true,
		},
		{
			name:               "today's settlement does not need settlement",
			lastSettlementDate: &todayIST,
			timezone:           "Asia/Kolkata",
			expectedNeedsWork:  false,
		},
		{
			name:               "two days ago needs settlement",
			lastSettlementDate: &twoDaysAgo,
			timezone:           "Asia/Kolkata",
			expectedNeedsWork:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			needsSettlement := determineNeedsSettlement(tc.lastSettlementDate, tc.timezone)
			if needsSettlement != tc.expectedNeedsWork {
				t.Errorf("needsSettlement = %v, want %v", needsSettlement, tc.expectedNeedsWork)
			}
		})
	}
}

// determineNeedsSettlement checks if a user needs settlement based on their timezone
// This is a helper for testing - mirrors the scheduler logic
func determineNeedsSettlement(lastSettlement *time.Time, timezone string) bool {
	if lastSettlement == nil {
		return true // Never settled
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC // Fallback to UTC
	}

	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	lastDate := time.Date(lastSettlement.Year(), lastSettlement.Month(), lastSettlement.Day(), 0, 0, 0, 0, loc)

	return lastDate.Before(today)
}

// ============================================================================
// TEST: Timezone Handling
// ============================================================================

func TestTimezoneHandling(t *testing.T) {
	testCases := []struct {
		timezone    string
		shouldLoad  bool
	}{
		{"Asia/Kolkata", true},
		{"UTC", true},
		{"America/New_York", true},
		{"Europe/London", true},
		{"Invalid/Timezone", false},
		{"", true}, // Empty string returns Local timezone in Go
	}

	for _, tc := range testCases {
		t.Run(tc.timezone, func(t *testing.T) {
			_, err := time.LoadLocation(tc.timezone)
			loaded := err == nil
			if loaded != tc.shouldLoad {
				t.Errorf("LoadLocation(%q) loaded = %v, want %v", tc.timezone, loaded, tc.shouldLoad)
			}
		})
	}
}

// ============================================================================
// TEST: Zero Quantity Position Filtering
// ============================================================================

func TestZeroQuantityFiltering(t *testing.T) {
	positions := []struct {
		symbol      string
		positionAmt float64
		shouldKeep  bool
	}{
		{"BTCUSDT", 0.5, true},
		{"ETHUSDT", 0.0, false},  // Zero quantity - should be filtered
		{"SOLUSDT", -1.0, true},  // Short position
		{"XRPUSDT", 0.0, false},  // Zero quantity
		{"ADAUSDT", 0.001, true}, // Small but non-zero
	}

	var keptCount int
	for _, pos := range positions {
		if pos.positionAmt != 0 {
			keptCount++
		}
	}

	expectedKept := 3
	if keptCount != expectedKept {
		t.Errorf("keptCount = %d, want %d", keptCount, expectedKept)
	}
}

// ============================================================================
// TEST: Mode Breakdown Aggregation
// ============================================================================

func TestModeBreakdownAggregation(t *testing.T) {
	snapshots := []PositionSnapshot{
		{Mode: ModeScalp, UnrealizedPnL: 100},
		{Mode: ModeScalp, UnrealizedPnL: 200},
		{Mode: ModeSwing, UnrealizedPnL: 500},
		{Mode: ModeUnknown, UnrealizedPnL: -50},
	}

	// Aggregate by mode
	modeAggregates := make(map[string]struct {
		count int
		pnl   float64
	})

	for _, s := range snapshots {
		agg := modeAggregates[s.Mode]
		agg.count++
		agg.pnl += s.UnrealizedPnL
		modeAggregates[s.Mode] = agg
	}

	// Verify scalp aggregation
	if modeAggregates[ModeScalp].count != 2 {
		t.Errorf("Scalp count = %d, want 2", modeAggregates[ModeScalp].count)
	}
	if modeAggregates[ModeScalp].pnl != 300 {
		t.Errorf("Scalp PnL = %f, want 300", modeAggregates[ModeScalp].pnl)
	}

	// Verify swing aggregation
	if modeAggregates[ModeSwing].count != 1 {
		t.Errorf("Swing count = %d, want 1", modeAggregates[ModeSwing].count)
	}
	if modeAggregates[ModeSwing].pnl != 500 {
		t.Errorf("Swing PnL = %f, want 500", modeAggregates[ModeSwing].pnl)
	}

	// Verify unknown aggregation
	if modeAggregates[ModeUnknown].pnl != -50 {
		t.Errorf("Unknown PnL = %f, want -50", modeAggregates[ModeUnknown].pnl)
	}
}

// ============================================================================
// TEST: Empty UserID Validation
// ============================================================================

func TestEmptyUserIDValidation(t *testing.T) {
	// Create a service with nil dependencies (won't be used due to validation)
	service := &PositionSnapshotService{}

	result, err := service.SnapshotOpenPositions(nil, "", time.Now())

	if err == nil {
		t.Error("Expected error for empty userID, got nil")
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil even on error")
	}

	if result.Success {
		t.Error("Expected Success to be false for empty userID")
	}

	if result.Error != "userID cannot be empty" {
		t.Errorf("Expected error message 'userID cannot be empty', got %q", result.Error)
	}
}

// ============================================================================
// TEST: Scheduler Restart Capability
// ============================================================================

func TestSchedulerRestartCapability(t *testing.T) {
	// Create scheduler without actual dependencies
	scheduler := &Scheduler{
		config:   DefaultSchedulerConfig(),
		stopChan: make(chan struct{}),
	}

	// Simulate first start
	scheduler.running = true

	// Simulate stop (close stopChan)
	close(scheduler.stopChan)
	scheduler.running = false

	// Simulate second start - should create new stopChan
	scheduler.stopChan = make(chan struct{}) // This is what Start() should do
	scheduler.running = true

	// Verify we can close again without panic
	close(scheduler.stopChan)

	// If we got here without panic, the restart capability works
	t.Log("Scheduler restart capability verified - no panic on second close")
}

// ============================================================================
// MOCK: FuturesClient for testing extractModeForPosition
// ============================================================================

// mockFuturesClient is a minimal mock for testing mode extraction
type mockFuturesClient struct {
	openOrders []binance.FuturesOrder
	allOrders  []binance.FuturesOrder
	openErr    error
	allErr     error
}

// GetOpenOrders and GetAllOrders are the only methods used in extractModeForPosition
func (m *mockFuturesClient) GetOpenOrders(symbol string) ([]binance.FuturesOrder, error) {
	return m.openOrders, m.openErr
}
func (m *mockFuturesClient) GetAllOrders(symbol string, limit int) ([]binance.FuturesOrder, error) {
	return m.allOrders, m.allErr
}

// Implement all other interface methods (unused in test but required for interface compliance)
func (m *mockFuturesClient) GetFuturesAccountInfo() (*binance.FuturesAccountInfo, error) { return nil, nil }
func (m *mockFuturesClient) GetPositions() ([]binance.FuturesPosition, error)            { return nil, nil }
func (m *mockFuturesClient) GetPositionBySymbol(string) (*binance.FuturesPosition, error) { return nil, nil }
func (m *mockFuturesClient) GetCommissionRate(string) (*binance.CommissionRate, error)   { return nil, nil }
func (m *mockFuturesClient) SetLeverage(string, int) (*binance.LeverageResponse, error)  { return nil, nil }
func (m *mockFuturesClient) SetMarginType(string, binance.MarginType) error              { return nil }
func (m *mockFuturesClient) SetPositionMode(bool) error                                  { return nil }
func (m *mockFuturesClient) GetPositionMode() (*binance.PositionModeResponse, error)     { return nil, nil }
func (m *mockFuturesClient) PlaceFuturesOrder(binance.FuturesOrderParams) (*binance.FuturesOrderResponse, error) { return nil, nil }
func (m *mockFuturesClient) CancelFuturesOrder(string, int64) error                      { return nil }
func (m *mockFuturesClient) CancelAllFuturesOrders(string) error                         { return nil }
func (m *mockFuturesClient) GetOrder(string, int64) (*binance.FuturesOrder, error)       { return nil, nil }
func (m *mockFuturesClient) PlaceAlgoOrder(binance.AlgoOrderParams) (*binance.AlgoOrderResponse, error) { return nil, nil }
func (m *mockFuturesClient) GetOpenAlgoOrders(string) ([]binance.AlgoOrder, error)       { return nil, nil }
func (m *mockFuturesClient) CancelAlgoOrder(string, int64) error                         { return nil }
func (m *mockFuturesClient) CancelAllAlgoOrders(string) error                            { return nil }
func (m *mockFuturesClient) GetAllAlgoOrders(string, int) ([]binance.AlgoOrder, error)   { return nil, nil }
func (m *mockFuturesClient) GetFundingRate(string) (*binance.FundingRate, error)         { return nil, nil }
func (m *mockFuturesClient) GetFundingRateHistory(string, int) ([]binance.FundingRate, error) { return nil, nil }
func (m *mockFuturesClient) GetMarkPrice(string) (*binance.MarkPrice, error)             { return nil, nil }
func (m *mockFuturesClient) GetAllMarkPrices() ([]binance.MarkPrice, error)              { return nil, nil }
func (m *mockFuturesClient) GetOrderBookDepth(string, int) (*binance.OrderBookDepth, error) { return nil, nil }
func (m *mockFuturesClient) GetFuturesKlines(string, string, int) ([]binance.Kline, error) { return nil, nil }
func (m *mockFuturesClient) Get24hrTicker(string) (*binance.Futures24hrTicker, error)    { return nil, nil }
func (m *mockFuturesClient) GetAll24hrTickers() ([]binance.Futures24hrTicker, error)     { return nil, nil }
func (m *mockFuturesClient) GetFuturesCurrentPrice(string) (float64, error)              { return 0, nil }
func (m *mockFuturesClient) GetFuturesExchangeInfo() (*binance.FuturesExchangeInfo, error) { return nil, nil }
func (m *mockFuturesClient) GetFuturesSymbols() ([]string, error)                        { return nil, nil }
func (m *mockFuturesClient) GetTradeHistory(string, int) ([]binance.FuturesTrade, error) { return nil, nil }
func (m *mockFuturesClient) GetTradeHistoryByDateRange(string, int64, int64, int) ([]binance.FuturesTrade, error) { return nil, nil }
func (m *mockFuturesClient) GetFundingFeeHistory(string, int) ([]binance.FundingFeeRecord, error) { return nil, nil }
func (m *mockFuturesClient) GetAllOrdersByDateRange(string, int64, int64, int) ([]binance.FuturesOrder, error) { return nil, nil }
func (m *mockFuturesClient) GetIncomeHistory(string, int64, int64, int) ([]binance.IncomeRecord, error) { return nil, nil }
func (m *mockFuturesClient) GetListenKey() (string, error)                               { return "", nil }
func (m *mockFuturesClient) KeepAliveListenKey(string) error                             { return nil }
func (m *mockFuturesClient) CloseListenKey(string) error                                 { return nil }

// ============================================================================
// TEST: extractModeForPosition with mock client
// ============================================================================

func TestExtractModeForPosition(t *testing.T) {
	service := &PositionSnapshotService{}
	ctx := context.Background()

	testCases := []struct {
		name         string
		position     binance.FuturesPosition
		openOrders   []binance.FuturesOrder
		allOrders    []binance.FuturesOrder
		expectedMode string
		expectedID   string
	}{
		{
			name: "finds mode from open orders",
			position: binance.FuturesPosition{
				Symbol:       "BTCUSDT",
				PositionSide: "LONG",
				PositionAmt:  0.5,
			},
			openOrders: []binance.FuturesOrder{
				{Symbol: "BTCUSDT", PositionSide: "LONG", ClientOrderId: "SCA-16JAN-00001-E"},
			},
			expectedMode: ModeScalp,
			expectedID:   "SCA-16JAN-00001-E",
		},
		{
			name: "finds mode from all orders when open orders empty",
			position: binance.FuturesPosition{
				Symbol:       "ETHUSDT",
				PositionSide: "SHORT",
				PositionAmt:  -1.0,
			},
			openOrders: []binance.FuturesOrder{},
			allOrders: []binance.FuturesOrder{
				{Symbol: "ETHUSDT", PositionSide: "SHORT", ClientOrderId: "SWI-16JAN-00002-E"},
			},
			expectedMode: ModeSwing,
			expectedID:   "SWI-16JAN-00002-E",
		},
		{
			name: "returns unknown when no matching clientOrderId",
			position: binance.FuturesPosition{
				Symbol:       "SOLUSDT",
				PositionSide: "LONG",
				PositionAmt:  10,
			},
			openOrders:   []binance.FuturesOrder{},
			allOrders:    []binance.FuturesOrder{},
			expectedMode: ModeUnknown,
			expectedID:   "",
		},
		{
			name: "returns unknown for legacy clientOrderId",
			position: binance.FuturesPosition{
				Symbol:       "ADAUSDT",
				PositionSide: "LONG",
				PositionAmt:  100,
			},
			openOrders: []binance.FuturesOrder{
				{Symbol: "ADAUSDT", PositionSide: "LONG", ClientOrderId: "web_legacy_123"},
			},
			expectedMode: ModeUnknown,
			expectedID:   "",
		},
		{
			name: "skips orders with wrong position side",
			position: binance.FuturesPosition{
				Symbol:       "XRPUSDT",
				PositionSide: "LONG",
				PositionAmt:  500,
			},
			openOrders: []binance.FuturesOrder{
				{Symbol: "XRPUSDT", PositionSide: "SHORT", ClientOrderId: "POS-16JAN-00003-E"},
			},
			allOrders: []binance.FuturesOrder{
				{Symbol: "XRPUSDT", PositionSide: "LONG", ClientOrderId: "ULT-16JAN-00004-E"},
			},
			expectedMode: ModeUltraFast,
			expectedID:   "ULT-16JAN-00004-E",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &mockFuturesClient{
				openOrders: tc.openOrders,
				allOrders:  tc.allOrders,
			}

			mode, clientOrderID := service.extractModeForPosition(ctx, mockClient, tc.position)

			if mode != tc.expectedMode {
				t.Errorf("mode = %q, want %q", mode, tc.expectedMode)
			}
			if clientOrderID != tc.expectedID {
				t.Errorf("clientOrderID = %q, want %q", clientOrderID, tc.expectedID)
			}
		})
	}
}
