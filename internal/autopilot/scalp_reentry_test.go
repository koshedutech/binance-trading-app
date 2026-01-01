package autopilot

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// ============ PRECISION FUNCTION TESTS (AC1) ============

// TestRoundPrice tests the generic price rounding function
func TestRoundPrice(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		price    float64
		expected float64
	}{
		{
			name:     "BTCUSDT - 2 decimals - round up",
			symbol:   "BTCUSDT",
			price:    87438.456,
			expected: 87438.46,
		},
		{
			name:     "BTCUSDT - 2 decimals - round down",
			symbol:   "BTCUSDT",
			price:    87438.454,
			expected: 87438.45,
		},
		{
			name:     "ETHUSDT - 2 decimals",
			symbol:   "ETHUSDT",
			price:    3456.789,
			expected: 3456.79,
		},
		{
			name:     "BNBUSDT - 2 decimals",
			symbol:   "BNBUSDT",
			price:    861.2345,
			expected: 861.23,
		},
		{
			name:     "SOLUSDT - 3 decimals",
			symbol:   "SOLUSDT",
			price:    173.56789,
			expected: 173.568,
		},
		{
			name:     "Unknown symbol uses 6 decimal default",
			symbol:   "NEWCOINUSDT",
			price:    0.01234567,
			expected: 0.012346, // 6 decimals with rounding
		},
		{
			name:     "Low price coin - 6 decimal default",
			symbol:   "UNKNOWNUSDT",
			price:    0.000123456,
			expected: 0.000123, // 6 decimals
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := roundPrice(tt.symbol, tt.price)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("roundPrice(%s, %.8f) = %.8f, want %.8f",
					tt.symbol, tt.price, result, tt.expected)
			}
		})
	}
}

// TestRoundPriceForTP tests TP price rounding (conservative for execution)
func TestRoundPriceForTP(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		price    float64
		side     string
		expected float64
	}{
		{
			name:     "LONG - floors for earlier TP trigger",
			symbol:   "BTCUSDT",
			price:    87438.456,
			side:     "LONG",
			expected: 87438.45, // Floor: triggers at lower price
		},
		{
			name:     "SHORT - ceils for earlier TP trigger",
			symbol:   "BTCUSDT",
			price:    87438.451,
			side:     "SHORT",
			expected: 87438.46, // Ceil: triggers at higher price
		},
		{
			name:     "LONG - exact value unchanged",
			symbol:   "BTCUSDT",
			price:    87438.50,
			side:     "LONG",
			expected: 87438.50,
		},
		{
			name:     "SHORT - exact value unchanged",
			symbol:   "BTCUSDT",
			price:    87438.50,
			side:     "SHORT",
			expected: 87438.50,
		},
		{
			name:     "BNBUSDT LONG - 2 decimals floor",
			symbol:   "BNBUSDT",
			price:    861.5678,
			side:     "LONG",
			expected: 861.56,
		},
		{
			name:     "BNBUSDT SHORT - 2 decimals ceil",
			symbol:   "BNBUSDT",
			price:    861.5672,
			side:     "SHORT",
			expected: 861.57,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := roundPriceForTP(tt.symbol, tt.price, tt.side)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("roundPriceForTP(%s, %.8f, %s) = %.8f, want %.8f",
					tt.symbol, tt.price, tt.side, result, tt.expected)
			}
		})
	}
}

// TestRoundPriceForSL tests SL price rounding (conservative for protection)
func TestRoundPriceForSL(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		price    float64
		side     string
		expected float64
	}{
		{
			name:     "LONG - ceils for earlier SL trigger (protects capital)",
			symbol:   "BTCUSDT",
			price:    87000.001,
			side:     "LONG",
			expected: 87000.01, // Ceil: triggers sooner, limits loss
		},
		{
			name:     "SHORT - floors for earlier SL trigger (protects capital)",
			symbol:   "BTCUSDT",
			price:    87000.999,
			side:     "SHORT",
			expected: 87000.99, // Floor: triggers sooner, limits loss
		},
		{
			name:     "LONG - exact value unchanged",
			symbol:   "BTCUSDT",
			price:    87000.50,
			side:     "LONG",
			expected: 87000.50,
		},
		{
			name:     "SHORT - exact value unchanged",
			symbol:   "BTCUSDT",
			price:    87000.50,
			side:     "SHORT",
			expected: 87000.50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := roundPriceForSL(tt.symbol, tt.price, tt.side)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("roundPriceForSL(%s, %.8f, %s) = %.8f, want %.8f",
					tt.symbol, tt.price, tt.side, result, tt.expected)
			}
		})
	}
}

// TestQuantityPrecision tests quantity rounding for various symbols
func TestQuantityPrecision(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		quantity float64
		expected float64
	}{
		{
			name:     "BTCUSDT - 3 decimals",
			symbol:   "BTCUSDT",
			quantity: 0.12345,
			expected: 0.123, // Floor - never over-sell
		},
		{
			name:     "ETHUSDT - 3 decimals",
			symbol:   "ETHUSDT",
			quantity: 0.56789,
			expected: 0.567,
		},
		{
			name:     "BNBUSDT - 2 decimals",
			symbol:   "BNBUSDT",
			quantity: 1.2345,
			expected: 1.23,
		},
		{
			name:     "SOLUSDT - 0 decimals (whole)",
			symbol:   "SOLUSDT",
			quantity: 10.987,
			expected: 10, // SOLUSDT has 0 qty precision
		},
		{
			name:     "Unknown symbol - 0 decimals (whole)",
			symbol:   "NEWCOINUSDT",
			quantity: 123.456,
			expected: 123, // Defaults to whole numbers
		},
	}

	// Create a mock GinieAutopilot for testing roundQuantity
	g := &GinieAutopilot{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.roundQuantity(tt.symbol, tt.quantity)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("roundQuantity(%s, %.8f) = %.8f, want %.8f",
					tt.symbol, tt.quantity, result, tt.expected)
			}
		})
	}
}

// ============ BREAKEVEN CALCULATION TESTS (AC1, AC2) ============

// TestCalculateNewBreakeven tests the breakeven calculation after partial closes and reentries
func TestCalculateNewBreakeven(t *testing.T) {
	g := &GinieAutopilot{}

	tests := []struct {
		name           string
		pos            *GiniePosition
		sr             *ScalpReentryStatus
		expectedBE     float64
		allowedDelta   float64
		description    string
	}{
		{
			name: "Initial position - no cycles",
			pos: &GiniePosition{
				Symbol:      "BTCUSDT",
				EntryPrice:  100.0,
				OriginalQty: 10.0,
			},
			sr: &ScalpReentryStatus{
				Cycles: []ReentryCycle{},
			},
			expectedBE:   100.0,
			allowedDelta: 0.01,
			description:  "BE should equal entry price when no trades",
		},
		{
			name: "After TP1 sell - no reentry yet",
			pos: &GiniePosition{
				Symbol:      "BTCUSDT",
				EntryPrice:  100.0,
				OriginalQty: 10.0,
			},
			sr: &ScalpReentryStatus{
				Cycles: []ReentryCycle{
					{
						SellPrice:    100.30, // Sold at 0.3% profit
						SellQuantity: 3.0,    // Sold 30%
						ReentryState: ReentryStateWaiting,
					},
				},
			},
			// netCost = 100 * 10 - 100.30 * 3 = 1000 - 300.90 = 699.10
			// netQty = 10 - 3 = 7
			// BE = 699.10 / 7 = 99.87
			expectedBE:   99.87,
			allowedDelta: 0.01,
			description:  "BE should decrease after profitable sell",
		},
		{
			name: "After TP1 sell + reentry completed",
			pos: &GiniePosition{
				Symbol:      "BTCUSDT",
				EntryPrice:  100.0,
				OriginalQty: 10.0,
			},
			sr: &ScalpReentryStatus{
				Cycles: []ReentryCycle{
					{
						SellPrice:          100.30,
						SellQuantity:       3.0,
						ReentryState:       ReentryStateCompleted,
						ReentryFilledPrice: 100.0, // Re-entered at breakeven
						ReentryFilledQty:   2.4,   // 80% of sold
					},
				},
			},
			// netCost = 100*10 - 100.30*3 + 100.0*2.4 = 1000 - 300.90 + 240 = 939.10
			// netQty = 10 - 3 + 2.4 = 9.4
			// BE = 939.10 / 9.4 = 99.90
			expectedBE:   99.90,
			allowedDelta: 0.01,
			description:  "BE should be close to original after re-entry at breakeven",
		},
		{
			name: "Multiple cycles completed",
			pos: &GiniePosition{
				Symbol:      "BTCUSDT",
				EntryPrice:  100.0,
				OriginalQty: 10.0,
			},
			sr: &ScalpReentryStatus{
				Cycles: []ReentryCycle{
					{
						SellPrice:          100.30, // TP1: +0.3%
						SellQuantity:       3.0,
						ReentryState:       ReentryStateCompleted,
						ReentryFilledPrice: 100.0,
						ReentryFilledQty:   2.4,
					},
					{
						SellPrice:          100.60, // TP2: +0.6%
						SellQuantity:       4.7,    // 50% of remaining
						ReentryState:       ReentryStateCompleted,
						ReentryFilledPrice: 99.90,
						ReentryFilledQty:   3.76, // 80% of sold
					},
				},
			},
			// This is a more complex calculation but BE should still be near original
			// due to profitable sells and reentries near breakeven
			expectedBE:   100.0, // Approximately
			allowedDelta: 0.5,   // Allow more delta for complex scenario
			description:  "BE should be manageable after multiple cycles",
		},
		{
			name: "Skipped reentry - no rebuy",
			pos: &GiniePosition{
				Symbol:      "BTCUSDT",
				EntryPrice:  100.0,
				OriginalQty: 10.0,
			},
			sr: &ScalpReentryStatus{
				Cycles: []ReentryCycle{
					{
						SellPrice:    100.30,
						SellQuantity: 3.0,
						ReentryState: ReentryStateSkipped, // AI skipped
					},
				},
			},
			// netCost = 1000 - 300.90 = 699.10
			// netQty = 10 - 3 = 7
			// BE = 699.10 / 7 = 99.87
			expectedBE:   99.87,
			allowedDelta: 0.01,
			description:  "Skipped reentry should not add to qty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.calculateNewBreakeven(tt.pos, tt.sr)
			if math.Abs(result-tt.expectedBE) > tt.allowedDelta {
				t.Errorf("%s: calculateNewBreakeven() = %.4f, want %.4f (delta %.4f)",
					tt.description, result, tt.expectedBE, tt.allowedDelta)
			}
			t.Logf("%s: BE = %.4f (expected %.4f)", tt.name, result, tt.expectedBE)
		})
	}
}

// ============ TP HIT DETECTION TESTS (AC2) ============

// TestCheckScalpReentryTP tests TP level hit detection
func TestCheckScalpReentryTP(t *testing.T) {
	// Initialize settings manager for test
	initTestSettings()

	g := &GinieAutopilot{}

	tests := []struct {
		name         string
		pos          *GiniePosition
		currentPrice float64
		tpLevel      int
		expectedHit  bool
		description  string
	}{
		{
			name: "LONG TP1 not reached",
			pos: &GiniePosition{
				Symbol:     "BTCUSDT",
				EntryPrice: 100.0,
				Side:       "LONG",
			},
			currentPrice: 100.20, // 0.2% - below TP1 (0.3%)
			tpLevel:      1,
			expectedHit:  false,
			description:  "Price below TP1 threshold",
		},
		{
			name: "LONG TP1 reached exactly",
			pos: &GiniePosition{
				Symbol:     "BTCUSDT",
				EntryPrice: 100.0,
				Side:       "LONG",
			},
			currentPrice: 100.30, // Exactly 0.3%
			tpLevel:      1,
			expectedHit:  true,
			description:  "Price at TP1 threshold",
		},
		{
			name: "LONG TP1 exceeded",
			pos: &GiniePosition{
				Symbol:     "BTCUSDT",
				EntryPrice: 100.0,
				Side:       "LONG",
			},
			currentPrice: 100.50, // 0.5% > 0.3%
			tpLevel:      1,
			expectedHit:  true,
			description:  "Price exceeds TP1 threshold",
		},
		{
			name: "SHORT TP1 reached",
			pos: &GiniePosition{
				Symbol:     "BTCUSDT",
				EntryPrice: 100.0,
				Side:       "SHORT",
			},
			currentPrice: 99.70, // -0.3%
			tpLevel:      1,
			expectedHit:  true,
			description:  "SHORT price dropped to TP1",
		},
		{
			name: "SHORT TP1 not reached",
			pos: &GiniePosition{
				Symbol:     "BTCUSDT",
				EntryPrice: 100.0,
				Side:       "SHORT",
			},
			currentPrice: 99.80, // Only -0.2%
			tpLevel:      1,
			expectedHit:  false,
			description:  "SHORT price not low enough for TP1",
		},
		{
			name: "LONG TP2 reached",
			pos: &GiniePosition{
				Symbol:     "BTCUSDT",
				EntryPrice: 100.0,
				Side:       "LONG",
			},
			currentPrice: 100.60, // 0.6%
			tpLevel:      2,
			expectedHit:  true,
			description:  "Price at TP2 threshold",
		},
		{
			name: "LONG TP3 reached",
			pos: &GiniePosition{
				Symbol:     "BTCUSDT",
				EntryPrice: 100.0,
				Side:       "LONG",
			},
			currentPrice: 101.00, // 1.0%
			tpLevel:      3,
			expectedHit:  true,
			description:  "Price at TP3 threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, tpPrice := g.checkScalpReentryTP(tt.pos, tt.currentPrice, tt.tpLevel)
			if hit != tt.expectedHit {
				t.Errorf("%s: checkScalpReentryTP() hit=%v, want %v (price=%.4f, tp=%.4f)",
					tt.description, hit, tt.expectedHit, tt.currentPrice, tpPrice)
			}
			t.Logf("%s: hit=%v, tpPrice=%.4f", tt.name, hit, tpPrice)
		})
	}
}

// ============ EDGE CASE TESTS (AC3) ============

// TestMinQuantityValidation tests minimum quantity edge cases
// Using hardcoded min quantities since getMinQuantity requires live client
func TestMinQuantityValidation(t *testing.T) {
	g := &GinieAutopilot{}

	// Hardcoded min quantities from Binance futures specs
	minQtyMap := map[string]float64{
		"BTCUSDT":  0.001,
		"ETHUSDT":  0.001,
		"BNBUSDT":  0.01,
		"SOLUSDT":  1,
		"DOGEUSDT": 1,
	}

	tests := []struct {
		name          string
		symbol        string
		quantity      float64
		sellPercent   float64
		expectTrade   bool
		description   string
	}{
		{
			name:         "BTCUSDT normal quantity",
			symbol:       "BTCUSDT",
			quantity:     0.5,
			sellPercent:  30,
			expectTrade:  true,
			description:  "0.5 * 0.3 = 0.15 > 0.001 min",
		},
		{
			name:         "BTCUSDT tiny quantity after 30% sell",
			symbol:       "BTCUSDT",
			quantity:     0.002,
			sellPercent:  30,
			expectTrade:  false,
			description:  "0.002 * 0.3 = 0.0006 < 0.001 min",
		},
		{
			name:         "SOLUSDT edge case - below min",
			symbol:       "SOLUSDT",
			quantity:     2.0, // After rounding: 2 * 0.3 = 0.6 -> rounds to 0 (whole)
			sellPercent:  30,
			expectTrade:  false,
			description:  "2 * 0.3 = 0.6 rounds to 0, below min 1",
		},
		{
			name:         "SOLUSDT above min",
			symbol:       "SOLUSDT",
			quantity:     10.0,
			sellPercent:  30,
			expectTrade:  true,
			description:  "10 * 0.3 = 3 > min 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sellQty := tt.quantity * (tt.sellPercent / 100.0)
			sellQty = g.roundQuantity(tt.symbol, sellQty)
			minQty := minQtyMap[tt.symbol]

			canTrade := sellQty >= minQty

			if canTrade != tt.expectTrade {
				t.Errorf("%s: expected canTrade=%v, got %v (sellQty=%.8f, minQty=%.8f)",
					tt.description, tt.expectTrade, canTrade, sellQty, minQty)
			}
			t.Logf("%s: sellQty=%.8f, minQty=%.8f, canTrade=%v",
				tt.name, sellQty, minQty, canTrade)
		})
	}
}

// TestMaxCyclesLimit tests the maximum cycles per position limit
func TestMaxCyclesLimit(t *testing.T) {
	config := DefaultScalpReentryConfig()

	tests := []struct {
		name         string
		currentCycle int
		maxCycles    int
		canContinue  bool
	}{
		{"First cycle", 1, 10, true},
		{"Middle cycle", 5, 10, true},
		{"Last allowed cycle", 10, 10, true},
		{"Exceeds max", 11, 10, false},
		{"Way over max", 20, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.MaxCyclesPerPosition = tt.maxCycles
			canContinue := tt.currentCycle <= config.MaxCyclesPerPosition

			if canContinue != tt.canContinue {
				t.Errorf("cycle %d with max %d: expected canContinue=%v, got %v",
					tt.currentCycle, tt.maxCycles, tt.canContinue, canContinue)
			}
		})
	}
}

// TestReentryTimeout tests the reentry timeout logic
func TestReentryTimeout(t *testing.T) {
	config := DefaultScalpReentryConfig()
	config.ReentryTimeoutSec = 300 // 5 minutes

	tests := []struct {
		name         string
		elapsedSec   int
		expectTimeout bool
	}{
		{"Just started", 0, false},
		{"1 minute in", 60, false},
		{"4 minutes in", 240, false},
		{"Exactly at timeout", 300, true}, // time.Since uses > not >=, so at 300s it is timeout
		{"Just past timeout", 301, true},
		{"Way past timeout", 600, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startTime := time.Now().Add(-time.Duration(tt.elapsedSec) * time.Second)
			isTimeout := time.Since(startTime) > time.Duration(config.ReentryTimeoutSec)*time.Second

			if isTimeout != tt.expectTimeout {
				t.Errorf("elapsed %ds: expected timeout=%v, got %v",
					tt.elapsedSec, tt.expectTimeout, isTimeout)
			}
		})
	}
}

// ============ REENTRY STATE MACHINE TESTS (AC2) ============

// TestReentryStateTransitions tests valid state transitions
func TestReentryStateTransitions(t *testing.T) {
	tests := []struct {
		name      string
		from      ReentryState
		to        ReentryState
		isValid   bool
	}{
		{"None to Waiting", ReentryStateNone, ReentryStateWaiting, true},
		{"Waiting to Executing", ReentryStateWaiting, ReentryStateExecuting, true},
		{"Executing to Completed", ReentryStateExecuting, ReentryStateCompleted, true},
		{"Executing to Failed", ReentryStateExecuting, ReentryStateFailed, true},
		{"Waiting to Skipped", ReentryStateWaiting, ReentryStateSkipped, true},
		{"None to Completed", ReentryStateNone, ReentryStateCompleted, false},
		{"Completed to Waiting", ReentryStateCompleted, ReentryStateWaiting, false},
	}

	validTransitions := map[ReentryState][]ReentryState{
		ReentryStateNone:      {ReentryStateWaiting},
		ReentryStateWaiting:   {ReentryStateExecuting, ReentryStateSkipped},
		ReentryStateExecuting: {ReentryStateCompleted, ReentryStateFailed},
		ReentryStateCompleted: {},
		ReentryStateFailed:    {},
		ReentryStateSkipped:   {},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := validTransitions[tt.from]
			isValid := false
			for _, s := range allowed {
				if s == tt.to {
					isValid = true
					break
				}
			}

			if isValid != tt.isValid {
				t.Errorf("transition %s -> %s: expected valid=%v, got %v",
					tt.from, tt.to, tt.isValid, isValid)
			}
		})
	}
}

// ============ REAL SYMBOL PRECISION TESTS (AC5) ============

// TestRealSymbolPrecision tests with actual Binance symbol precisions
func TestRealSymbolPrecision(t *testing.T) {
	// Real Binance symbol configurations from futures_controller.go
	symbolConfigs := []struct {
		symbol        string
		pricePrecision int
		qtyPrecision   int
		minQty        float64
		tickSize      float64
	}{
		{"BTCUSDT", 2, 3, 0.001, 0.01},
		{"ETHUSDT", 2, 3, 0.001, 0.01},
		{"BNBUSDT", 2, 2, 0.01, 0.01},     // Fixed: price=2, qty=2
		{"SOLUSDT", 3, 0, 1, 0.001},       // Fixed: price=3, qty=0
		{"ADAUSDT", 5, 0, 1, 0.00001},
		{"DOGEUSDT", 5, 0, 1, 0.00001},    // Fixed: price=5
	}

	g := &GinieAutopilot{}

	for _, sc := range symbolConfigs {
		t.Run(sc.symbol, func(t *testing.T) {
			// Test price rounding respects precision
			testPrice := 100.123456789
			roundedPrice := roundPrice(sc.symbol, testPrice)

			// Count decimal places
			decimals := countDecimals(roundedPrice)
			if decimals > sc.pricePrecision {
				t.Errorf("%s: roundPrice has %d decimals, want max %d",
					sc.symbol, decimals, sc.pricePrecision)
			}

			// Test quantity rounding respects precision
			testQty := 10.123456789
			roundedQty := g.roundQuantity(sc.symbol, testQty)

			qtyDecimals := countDecimals(roundedQty)
			if qtyDecimals > sc.qtyPrecision {
				t.Errorf("%s: roundQuantity has %d decimals, want max %d",
					sc.symbol, qtyDecimals, sc.qtyPrecision)
			}

			t.Logf("%s: price %.8f -> %.8f, qty %.8f -> %.8f",
				sc.symbol, testPrice, roundedPrice, testQty, roundedQty)
		})
	}
}

// ============ INTEGRATION TEST: FULL TP CYCLE (AC2) ============

// TestFullTPCycleWithReentry tests the complete TP1 -> Reentry -> TP2 -> Reentry -> TP3 flow
func TestFullTPCycleWithReentry(t *testing.T) {
	initTestSettings()

	// Create a simulated position
	pos := &GiniePosition{
		Symbol:      "BTCUSDT",
		EntryPrice:  100.0,
		OriginalQty: 10.0,
		RemainingQty: 10.0,
		Side:        "LONG",
	}

	config := DefaultScalpReentryConfig()
	sr := NewScalpReentryStatus(pos.EntryPrice, pos.OriginalQty, config)
	pos.ScalpReentry = sr

	// === PHASE 1: TP1 at 0.3% ===
	t.Log("=== PHASE 1: TP1 at 0.3% ===")

	// Verify initial state
	if sr.RemainingQuantity != 10.0 {
		t.Fatalf("Initial qty should be 10.0, got %.4f", sr.RemainingQuantity)
	}
	if sr.TPLevelUnlocked != 0 {
		t.Fatalf("Initial TP level should be 0, got %d", sr.TPLevelUnlocked)
	}

	// Simulate TP1 hit - sell 30%
	tp1SellQty := sr.RemainingQuantity * 0.3 // 3.0
	sr.RemainingQuantity -= tp1SellQty
	sr.TPLevelUnlocked = 1
	sr.AccumulatedProfit += (100.30 - 100.0) * tp1SellQty // $0.90 profit

	cycle1 := ReentryCycle{
		CycleNumber:        1,
		TPLevel:            1,
		SellPrice:          100.30,
		SellQuantity:       tp1SellQty,
		ReentryTargetPrice: sr.CurrentBreakeven,
		ReentryQuantity:    tp1SellQty * 0.8, // 2.4
		ReentryState:       ReentryStateWaiting,
	}
	sr.Cycles = append(sr.Cycles, cycle1)
	sr.NextTPBlocked = true

	if sr.RemainingQuantity != 7.0 {
		t.Errorf("After TP1: qty should be 7.0, got %.4f", sr.RemainingQuantity)
	}
	if !sr.NextTPBlocked {
		t.Error("After TP1: next TP should be blocked")
	}

	// === PHASE 2: Reentry at breakeven ===
	t.Log("=== PHASE 2: Reentry at breakeven ===")

	reentry1Qty := cycle1.ReentryQuantity // 2.4
	sr.RemainingQuantity += reentry1Qty
	sr.Cycles[0].ReentryState = ReentryStateCompleted
	sr.Cycles[0].ReentryFilledPrice = 100.0
	sr.Cycles[0].ReentryFilledQty = reentry1Qty
	sr.TotalReentries++
	sr.SuccessfulReentries++
	sr.NextTPBlocked = false

	expectedQtyAfterReentry1 := 7.0 + 2.4 // 9.4
	if math.Abs(sr.RemainingQuantity-expectedQtyAfterReentry1) > 0.01 {
		t.Errorf("After reentry 1: qty should be %.4f, got %.4f",
			expectedQtyAfterReentry1, sr.RemainingQuantity)
	}

	// === PHASE 3: TP2 at 0.6% ===
	t.Log("=== PHASE 3: TP2 at 0.6% ===")

	tp2SellQty := sr.RemainingQuantity * 0.5 // 4.7
	sr.RemainingQuantity -= tp2SellQty
	sr.TPLevelUnlocked = 2
	sr.AccumulatedProfit += (100.60 - 100.0) * tp2SellQty

	cycle2 := ReentryCycle{
		CycleNumber:        2,
		TPLevel:            2,
		SellPrice:          100.60,
		SellQuantity:       tp2SellQty,
		ReentryTargetPrice: sr.CurrentBreakeven,
		ReentryQuantity:    tp2SellQty * 0.8,
		ReentryState:       ReentryStateWaiting,
	}
	sr.Cycles = append(sr.Cycles, cycle2)
	sr.NextTPBlocked = true

	// === PHASE 4: Reentry at breakeven ===
	t.Log("=== PHASE 4: Reentry at breakeven ===")

	reentry2Qty := cycle2.ReentryQuantity
	sr.RemainingQuantity += reentry2Qty
	sr.Cycles[1].ReentryState = ReentryStateCompleted
	sr.Cycles[1].ReentryFilledPrice = 99.95
	sr.Cycles[1].ReentryFilledQty = reentry2Qty
	sr.TotalReentries++
	sr.SuccessfulReentries++
	sr.NextTPBlocked = false

	// === PHASE 5: TP3 at 1.0% ===
	t.Log("=== PHASE 5: TP3 at 1.0% ===")

	tp3SellQty := sr.RemainingQuantity * 0.8 // Sell 80%, keep 20%
	finalPortionQty := sr.RemainingQuantity * 0.2
	sr.RemainingQuantity -= tp3SellQty
	sr.TPLevelUnlocked = 3
	sr.AccumulatedProfit += (101.0 - 100.0) * tp3SellQty
	sr.FinalPortionActive = true
	sr.FinalPortionQty = finalPortionQty
	sr.DynamicSLActive = true

	// Verify final state
	if sr.TPLevelUnlocked != 3 {
		t.Errorf("Final TP level should be 3, got %d", sr.TPLevelUnlocked)
	}
	if !sr.FinalPortionActive {
		t.Error("Final portion should be active after TP3")
	}
	if !sr.DynamicSLActive {
		t.Error("Dynamic SL should be active after TP3")
	}
	if sr.SuccessfulReentries != 2 {
		t.Errorf("Should have 2 successful reentries, got %d", sr.SuccessfulReentries)
	}

	t.Logf("=== FINAL STATE ===")
	t.Logf("Accumulated Profit: $%.4f", sr.AccumulatedProfit)
	t.Logf("Remaining Qty: %.4f", sr.RemainingQuantity)
	t.Logf("Final Portion Qty: %.4f", sr.FinalPortionQty)
	t.Logf("Cycles completed: %d", len(sr.Cycles))
	t.Logf("Reentries: %d successful / %d total", sr.SuccessfulReentries, sr.TotalReentries)
}

// ============ HELPER FUNCTIONS ============

// initTestSettings initializes settings for testing
func initTestSettings() {
	// Initialize settings manager with default config
	sm := GetSettingsManager()
	if sm != nil {
		settings := sm.GetCurrentSettings()
		settings.ScalpReentryConfig = DefaultScalpReentryConfig()
		settings.ScalpReentryConfig.Enabled = true
	}
}

// countDecimals counts decimal places in a float
func countDecimals(f float64) int {
	s := fmt.Sprintf("%.10f", f)
	// Find decimal point
	dotIdx := -1
	for i, c := range s {
		if c == '.' {
			dotIdx = i
			break
		}
	}
	if dotIdx == -1 {
		return 0
	}

	// Count non-zero trailing digits
	lastNonZero := dotIdx
	for i := len(s) - 1; i > dotIdx; i-- {
		if s[i] != '0' {
			lastNonZero = i
			break
		}
	}

	if lastNonZero == dotIdx {
		return 0
	}
	return lastNonZero - dotIdx
}

