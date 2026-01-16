package settlement

import (
	"math"
	"testing"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/orders"
)

// floatEquals compares two floats with tolerance
func floatEquals(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

// ============================================================================
// TEST: Aggregation with multiple trades per mode
// ============================================================================

func TestAggregatePnLForTrades_MultipleTrades(t *testing.T) {
	trades := []binance.FuturesTrade{
		{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0},
		{OrderId: 2, RealizedPnl: 50.0, QuoteQty: 500.0},
		{OrderId: 3, RealizedPnl: -30.0, QuoteQty: 300.0},
	}

	orderClientIDs := map[int64]string{
		1: "SCA-16JAN-00001-E",
		2: "SCA-16JAN-00002-E",
		3: "SCA-16JAN-00003-E",
	}

	result := AggregatePnLForTrades(trades, orderClientIDs)

	// Check scalp mode aggregation
	scalp, ok := result[ModeScalp]
	if !ok {
		t.Fatal("Expected scalp mode to exist")
	}

	if scalp.TradeCount != 3 {
		t.Errorf("Expected 3 trades for scalp, got %d", scalp.TradeCount)
	}

	expectedPnL := 100.0 + 50.0 - 30.0
	if scalp.RealizedPnL != expectedPnL {
		t.Errorf("Expected RealizedPnL %.2f, got %.2f", expectedPnL, scalp.RealizedPnL)
	}

	expectedVolume := 1000.0 + 500.0 + 300.0
	if scalp.TotalVolume != expectedVolume {
		t.Errorf("Expected TotalVolume %.2f, got %.2f", expectedVolume, scalp.TotalVolume)
	}

	// Check ALL mode
	all, ok := result[ModeAll]
	if !ok {
		t.Fatal("Expected ALL mode to exist")
	}

	if all.TradeCount != 3 {
		t.Errorf("Expected 3 trades for ALL, got %d", all.TradeCount)
	}

	if all.RealizedPnL != expectedPnL {
		t.Errorf("Expected ALL RealizedPnL %.2f, got %.2f", expectedPnL, all.RealizedPnL)
	}
}

// ============================================================================
// TEST: Win rate calculation (various win/loss ratios)
// ============================================================================

func TestAggregatePnLForTrades_WinRateCalculation(t *testing.T) {
	testCases := []struct {
		name            string
		trades          []binance.FuturesTrade
		expectedWinRate float64
	}{
		{
			name: "100% win rate (2 wins, 0 losses)",
			trades: []binance.FuturesTrade{
				{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0},
				{OrderId: 2, RealizedPnl: 50.0, QuoteQty: 500.0},
			},
			expectedWinRate: 100.0,
		},
		{
			name: "50% win rate (1 win, 1 loss)",
			trades: []binance.FuturesTrade{
				{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0},
				{OrderId: 2, RealizedPnl: -50.0, QuoteQty: 500.0},
			},
			expectedWinRate: 50.0,
		},
		{
			name: "0% win rate (0 wins, 2 losses)",
			trades: []binance.FuturesTrade{
				{OrderId: 1, RealizedPnl: -100.0, QuoteQty: 1000.0},
				{OrderId: 2, RealizedPnl: -50.0, QuoteQty: 500.0},
			},
			expectedWinRate: 0.0,
		},
		{
			name: "66.67% win rate (2 wins, 1 loss)",
			trades: []binance.FuturesTrade{
				{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0},
				{OrderId: 2, RealizedPnl: 50.0, QuoteQty: 500.0},
				{OrderId: 3, RealizedPnl: -30.0, QuoteQty: 300.0},
			},
			expectedWinRate: 66.66666666666667,
		},
		{
			name: "break-even trade (0 P&L) - not counted as win or loss",
			trades: []binance.FuturesTrade{
				{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0},
				{OrderId: 2, RealizedPnl: 0.0, QuoteQty: 500.0},
				{OrderId: 3, RealizedPnl: -50.0, QuoteQty: 300.0},
			},
			expectedWinRate: 33.33333333333333, // 1 win out of 3 trades
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			orderClientIDs := make(map[int64]string)
			for _, trade := range tc.trades {
				orderClientIDs[trade.OrderId] = "SCA-16JAN-00001-E"
			}

			result := AggregatePnLForTrades(tc.trades, orderClientIDs)
			scalp := result[ModeScalp]

			if !floatEquals(scalp.WinRate, tc.expectedWinRate, 0.01) {
				t.Errorf("Expected WinRate %.2f, got %.2f", tc.expectedWinRate, scalp.WinRate)
			}
		})
	}
}

// ============================================================================
// TEST: "ALL" mode total calculation
// ============================================================================

func TestAggregatePnLForTrades_AllModeTotal(t *testing.T) {
	trades := []binance.FuturesTrade{
		{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0}, // scalp
		{OrderId: 2, RealizedPnl: 200.0, QuoteQty: 2000.0}, // swing
		{OrderId: 3, RealizedPnl: -50.0, QuoteQty: 500.0},  // position
		{OrderId: 4, RealizedPnl: 75.0, QuoteQty: 750.0},   // ultra_fast
	}

	orderClientIDs := map[int64]string{
		1: "SCA-16JAN-00001-E",
		2: "SWI-16JAN-00001-E",
		3: "POS-16JAN-00001-E",
		4: "ULT-16JAN-00001-E",
	}

	result := AggregatePnLForTrades(trades, orderClientIDs)

	// Verify individual modes exist
	if _, ok := result[ModeScalp]; !ok {
		t.Error("Expected scalp mode")
	}
	if _, ok := result[ModeSwing]; !ok {
		t.Error("Expected swing mode")
	}
	if _, ok := result[ModePosition]; !ok {
		t.Error("Expected position mode")
	}
	if _, ok := result[ModeUltraFast]; !ok {
		t.Error("Expected ultra_fast mode")
	}

	// Verify ALL mode totals
	all := result[ModeAll]
	expectedTotalPnL := 100.0 + 200.0 - 50.0 + 75.0
	expectedTotalVolume := 1000.0 + 2000.0 + 500.0 + 750.0

	if all.TradeCount != 4 {
		t.Errorf("Expected ALL TradeCount 4, got %d", all.TradeCount)
	}

	if all.RealizedPnL != expectedTotalPnL {
		t.Errorf("Expected ALL RealizedPnL %.2f, got %.2f", expectedTotalPnL, all.RealizedPnL)
	}

	if all.TotalVolume != expectedTotalVolume {
		t.Errorf("Expected ALL TotalVolume %.2f, got %.2f", expectedTotalVolume, all.TotalVolume)
	}

	if all.WinCount != 3 {
		t.Errorf("Expected ALL WinCount 3, got %d", all.WinCount)
	}

	if all.LossCount != 1 {
		t.Errorf("Expected ALL LossCount 1, got %d", all.LossCount)
	}
}

// ============================================================================
// TEST: Handling of trades without clientOrderId (UNKNOWN mode)
// ============================================================================

func TestAggregatePnLForTrades_UnknownMode(t *testing.T) {
	trades := []binance.FuturesTrade{
		{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0}, // has clientOrderId
		{OrderId: 2, RealizedPnl: 50.0, QuoteQty: 500.0},   // missing clientOrderId
		{OrderId: 3, RealizedPnl: -30.0, QuoteQty: 300.0},  // invalid clientOrderId format
	}

	orderClientIDs := map[int64]string{
		1: "SCA-16JAN-00001-E",
		// 2 is missing
		3: "invalid-format",
	}

	result := AggregatePnLForTrades(trades, orderClientIDs)

	// Check scalp mode (1 trade)
	scalp, ok := result[ModeScalp]
	if !ok {
		t.Fatal("Expected scalp mode to exist")
	}

	if scalp.TradeCount != 1 {
		t.Errorf("Expected 1 scalp trade, got %d", scalp.TradeCount)
	}

	// Check UNKNOWN mode (2 trades)
	unknown, ok := result[ModeUnknown]
	if !ok {
		t.Fatal("Expected UNKNOWN mode to exist")
	}

	if unknown.TradeCount != 2 {
		t.Errorf("Expected 2 UNKNOWN trades, got %d", unknown.TradeCount)
	}

	expectedUnknownPnL := 50.0 - 30.0
	if unknown.RealizedPnL != expectedUnknownPnL {
		t.Errorf("Expected UNKNOWN RealizedPnL %.2f, got %.2f", expectedUnknownPnL, unknown.RealizedPnL)
	}
}

// ============================================================================
// TEST: Largest win/loss tracking
// ============================================================================

func TestAggregatePnLForTrades_LargestWinLoss(t *testing.T) {
	trades := []binance.FuturesTrade{
		{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0},  // win
		{OrderId: 2, RealizedPnl: 250.0, QuoteQty: 2500.0},  // largest win
		{OrderId: 3, RealizedPnl: 50.0, QuoteQty: 500.0},    // win
		{OrderId: 4, RealizedPnl: -30.0, QuoteQty: 300.0},   // loss
		{OrderId: 5, RealizedPnl: -150.0, QuoteQty: 1500.0}, // largest loss
		{OrderId: 6, RealizedPnl: -20.0, QuoteQty: 200.0},   // loss
	}

	orderClientIDs := make(map[int64]string)
	for _, trade := range trades {
		orderClientIDs[trade.OrderId] = "SCA-16JAN-00001-E"
	}

	result := AggregatePnLForTrades(trades, orderClientIDs)
	scalp := result[ModeScalp]

	if scalp.LargestWin != 250.0 {
		t.Errorf("Expected LargestWin 250.0, got %.2f", scalp.LargestWin)
	}

	if scalp.LargestLoss != -150.0 {
		t.Errorf("Expected LargestLoss -150.0, got %.2f", scalp.LargestLoss)
	}

	// Verify ALL mode also tracks largest win/loss
	all := result[ModeAll]
	if all.LargestWin != 250.0 {
		t.Errorf("Expected ALL LargestWin 250.0, got %.2f", all.LargestWin)
	}

	if all.LargestLoss != -150.0 {
		t.Errorf("Expected ALL LargestLoss -150.0, got %.2f", all.LargestLoss)
	}
}

// ============================================================================
// TEST: Average trade size calculation
// ============================================================================

func TestAggregatePnLForTrades_AvgTradeSize(t *testing.T) {
	trades := []binance.FuturesTrade{
		{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0},
		{OrderId: 2, RealizedPnl: 50.0, QuoteQty: 2000.0},
		{OrderId: 3, RealizedPnl: -30.0, QuoteQty: 3000.0},
	}

	orderClientIDs := make(map[int64]string)
	for _, trade := range trades {
		orderClientIDs[trade.OrderId] = "SCA-16JAN-00001-E"
	}

	result := AggregatePnLForTrades(trades, orderClientIDs)
	scalp := result[ModeScalp]

	expectedAvgSize := (1000.0 + 2000.0 + 3000.0) / 3
	if scalp.AvgTradeSize != expectedAvgSize {
		t.Errorf("Expected AvgTradeSize %.2f, got %.2f", expectedAvgSize, scalp.AvgTradeSize)
	}
}

// ============================================================================
// TEST: Edge case - zero trades (no division by zero)
// ============================================================================

func TestAggregatePnLForTrades_ZeroTrades(t *testing.T) {
	trades := []binance.FuturesTrade{}
	orderClientIDs := map[int64]string{}

	result := AggregatePnLForTrades(trades, orderClientIDs)

	// Should have ALL mode with zero values
	all, ok := result[ModeAll]
	if !ok {
		t.Fatal("Expected ALL mode to exist even with zero trades")
	}

	if all.TradeCount != 0 {
		t.Errorf("Expected TradeCount 0, got %d", all.TradeCount)
	}

	if all.WinRate != 0 {
		t.Errorf("Expected WinRate 0, got %.2f", all.WinRate)
	}

	if all.AvgTradeSize != 0 {
		t.Errorf("Expected AvgTradeSize 0, got %.2f", all.AvgTradeSize)
	}

	if all.RealizedPnL != 0 {
		t.Errorf("Expected RealizedPnL 0, got %.2f", all.RealizedPnL)
	}
}

// ============================================================================
// TEST: Mode extraction from ParsedOrderId
// ============================================================================

func TestExtractModeFromParsedOrderID(t *testing.T) {
	testCases := []struct {
		name         string
		parsed       *orders.ParsedOrderId
		expectedMode string
	}{
		{
			name:         "nil parsed",
			parsed:       nil,
			expectedMode: ModeUnknown,
		},
		{
			name:         "scalp mode",
			parsed:       &orders.ParsedOrderId{Mode: orders.ModeScalp},
			expectedMode: ModeScalp,
		},
		{
			name:         "swing mode",
			parsed:       &orders.ParsedOrderId{Mode: orders.ModeSwing},
			expectedMode: ModeSwing,
		},
		{
			name:         "position mode",
			parsed:       &orders.ParsedOrderId{Mode: orders.ModePosition},
			expectedMode: ModePosition,
		},
		{
			name:         "ultra_fast mode",
			parsed:       &orders.ParsedOrderId{Mode: orders.ModeUltraFast},
			expectedMode: ModeUltraFast,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractModeFromParsedOrderID(tc.parsed)
			if result != tc.expectedMode {
				t.Errorf("Expected mode %s, got %s", tc.expectedMode, result)
			}
		})
	}
}

// ============================================================================
// TEST: Win rate is within valid range (0-100%)
// ============================================================================

func TestAggregatePnLForTrades_WinRateRange(t *testing.T) {
	// Test various scenarios to ensure win rate stays between 0-100
	testCases := []struct {
		name   string
		trades []binance.FuturesTrade
	}{
		{
			name:   "all wins",
			trades: []binance.FuturesTrade{{OrderId: 1, RealizedPnl: 100.0}, {OrderId: 2, RealizedPnl: 50.0}},
		},
		{
			name:   "all losses",
			trades: []binance.FuturesTrade{{OrderId: 1, RealizedPnl: -100.0}, {OrderId: 2, RealizedPnl: -50.0}},
		},
		{
			name:   "mixed",
			trades: []binance.FuturesTrade{{OrderId: 1, RealizedPnl: 100.0}, {OrderId: 2, RealizedPnl: -50.0}},
		},
		{
			name:   "single trade win",
			trades: []binance.FuturesTrade{{OrderId: 1, RealizedPnl: 100.0}},
		},
		{
			name:   "single trade loss",
			trades: []binance.FuturesTrade{{OrderId: 1, RealizedPnl: -100.0}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			orderClientIDs := make(map[int64]string)
			for _, trade := range tc.trades {
				orderClientIDs[trade.OrderId] = "SCA-16JAN-00001-E"
			}

			result := AggregatePnLForTrades(tc.trades, orderClientIDs)
			scalp := result[ModeScalp]

			if scalp.WinRate < 0 || scalp.WinRate > 100 {
				t.Errorf("WinRate %.2f is outside valid range 0-100", scalp.WinRate)
			}
		})
	}
}

// ============================================================================
// TEST: Multiple modes in same aggregation
// ============================================================================

func TestAggregatePnLForTrades_MultipleModes(t *testing.T) {
	trades := []binance.FuturesTrade{
		{OrderId: 1, RealizedPnl: 100.0, QuoteQty: 1000.0},
		{OrderId: 2, RealizedPnl: 200.0, QuoteQty: 2000.0},
		{OrderId: 3, RealizedPnl: 300.0, QuoteQty: 3000.0},
		{OrderId: 4, RealizedPnl: 400.0, QuoteQty: 4000.0},
	}

	orderClientIDs := map[int64]string{
		1: "SCA-16JAN-00001-E",
		2: "SWI-16JAN-00001-E",
		3: "POS-16JAN-00002-E",
		4: "ULT-16JAN-00001-E",
	}

	result := AggregatePnLForTrades(trades, orderClientIDs)

	// Should have 5 modes: scalp, swing, position, ultra_fast, ALL
	if len(result) != 5 {
		t.Errorf("Expected 5 modes, got %d", len(result))
	}

	// Verify each mode has correct single trade
	modes := []string{ModeScalp, ModeSwing, ModePosition, ModeUltraFast}
	for _, mode := range modes {
		if modePnL, ok := result[mode]; !ok {
			t.Errorf("Expected mode %s to exist", mode)
		} else if modePnL.TradeCount != 1 {
			t.Errorf("Expected mode %s to have 1 trade, got %d", mode, modePnL.TradeCount)
		}
	}

	// ALL should have 4 trades
	if result[ModeAll].TradeCount != 4 {
		t.Errorf("Expected ALL to have 4 trades, got %d", result[ModeAll].TradeCount)
	}
}

// ============================================================================
// TEST: Trade deduplication by ID
// ============================================================================

func TestDeduplicateTradesByID(t *testing.T) {
	trades := []binance.FuturesTrade{
		{ID: 1, OrderId: 100, RealizedPnl: 100.0, QuoteQty: 1000.0, Symbol: "BTCUSDT"},
		{ID: 2, OrderId: 101, RealizedPnl: 50.0, QuoteQty: 500.0, Symbol: "BTCUSDT"},
		{ID: 1, OrderId: 100, RealizedPnl: 100.0, QuoteQty: 1000.0, Symbol: "BTCUSDT"}, // Duplicate of first
		{ID: 3, OrderId: 102, RealizedPnl: -30.0, QuoteQty: 300.0, Symbol: "ETHUSDT"},
		{ID: 2, OrderId: 101, RealizedPnl: 50.0, QuoteQty: 500.0, Symbol: "BTCUSDT"},   // Duplicate of second
	}

	result := deduplicateTradesByID(trades)

	if len(result) != 3 {
		t.Errorf("Expected 3 unique trades, got %d", len(result))
	}

	// Verify the unique trade IDs are present
	ids := make(map[int64]bool)
	for _, trade := range result {
		ids[trade.ID] = true
	}

	if !ids[1] || !ids[2] || !ids[3] {
		t.Errorf("Expected trade IDs 1, 2, 3 to be present, got %v", ids)
	}
}

func TestDeduplicateTradesByID_Empty(t *testing.T) {
	trades := []binance.FuturesTrade{}
	result := deduplicateTradesByID(trades)

	if len(result) != 0 {
		t.Errorf("Expected 0 trades, got %d", len(result))
	}
}

func TestDeduplicateTradesByID_NoDuplicates(t *testing.T) {
	trades := []binance.FuturesTrade{
		{ID: 1, OrderId: 100, RealizedPnl: 100.0},
		{ID: 2, OrderId: 101, RealizedPnl: 50.0},
		{ID: 3, OrderId: 102, RealizedPnl: -30.0},
	}

	result := deduplicateTradesByID(trades)

	if len(result) != 3 {
		t.Errorf("Expected 3 trades (no duplicates), got %d", len(result))
	}
}
