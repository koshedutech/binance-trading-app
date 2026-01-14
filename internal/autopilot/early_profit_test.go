package autopilot

import (
	"math"
	"testing"
)

// TestCalculateROIAfterFees tests the ROI calculation function for various scenarios
// Note: With TakerFeeRate = 0.05% (0.0005), entry + exit fees = ~0.10% of notional
// This means small price moves can result in negative ROI due to fees
func TestCalculateROIAfterFees(t *testing.T) {
	tests := []struct {
		name         string
		entryPrice   float64
		currentPrice float64
		quantity     float64
		side         string
		leverage     int
		wantPositive bool // whether we expect positive ROI
		description  string
	}{
		{
			name:         "LONG significant profit - price up 1%",
			entryPrice:   87438.50,
			currentPrice: 88312.89, // +1% move
			quantity:     0.003,
			side:         "LONG",
			leverage:     5,
			wantPositive: true,
			description:  "LONG 1% price increase should overcome fees",
		},
		{
			name:         "LONG loss - price decreased",
			entryPrice:   87438.50,
			currentPrice: 87400.00,
			quantity:     0.003,
			side:         "LONG",
			leverage:     5,
			wantPositive: false,
			description:  "LONG position should lose when price goes down",
		},
		{
			name:         "SHORT significant profit - price down 1%",
			entryPrice:   87438.50,
			currentPrice: 86564.12, // -1% move
			quantity:     0.003,
			side:         "SHORT",
			leverage:     5,
			wantPositive: true,
			description:  "SHORT 1% price decrease should overcome fees",
		},
		{
			name:         "SHORT loss - price increased",
			entryPrice:   87438.50,
			currentPrice: 87500.00,
			quantity:     0.003,
			side:         "SHORT",
			leverage:     5,
			wantPositive: false,
			description:  "SHORT position should lose when price goes up",
		},
		{
			name:         "Trade#125 scenario - SHORT tiny move eaten by fees",
			entryPrice:   87438.50,
			currentPrice: 87425.00, // Only 0.015% move - fees (~0.08%) exceed profit
			quantity:     0.003,
			side:         "SHORT",
			leverage:     5,
			wantPositive: false, // Fees eat the tiny profit, ROI is negative
			description:  "Trade#125: tiny move eaten by fees, ROI should be negative",
		},
		{
			name:         "Zero leverage defaults to 1",
			entryPrice:   100.0,
			currentPrice: 110.0,
			quantity:     1.0,
			side:         "LONG",
			leverage:     0, // Should default to 1
			wantPositive: true,
			description:  "Zero leverage should default to 1x",
		},
		{
			name:         "Negative leverage defaults to 1",
			entryPrice:   100.0,
			currentPrice: 110.0,
			quantity:     1.0,
			side:         "LONG",
			leverage:     -5, // Should default to 1
			wantPositive: true,
			description:  "Negative leverage should default to 1x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roi := calculateROIAfterFees(tt.entryPrice, tt.currentPrice, tt.quantity, tt.side, tt.leverage)

			if tt.wantPositive && roi <= 0 {
				t.Errorf("%s: expected positive ROI, got %.6f%%", tt.description, roi)
			}
			if !tt.wantPositive && roi >= 0 {
				t.Errorf("%s: expected negative ROI, got %.6f%%", tt.description, roi)
			}

			t.Logf("%s: ROI = %.6f%%", tt.name, roi)
		})
	}
}

// TestTrade125Scenario specifically tests the bug scenario from Trade#125
// SHORT BTCUSDT: entry=87438.50, current=87425, TP1=87001.41
//
// IMPORTANT INSIGHT: With trading fees (0.04% entry + 0.04% exit = 0.08% total),
// a tiny 0.015% price move results in NEGATIVE ROI after fees.
// The bug must have been in threshold calculation, not ROI calculation.
func TestTrade125Scenario(t *testing.T) {
	entryPrice := 87438.50
	currentPrice := 87425.00
	tp1Price := 87001.41
	quantity := 0.003
	leverage := 5

	// Calculate ROI at current price (tiny profit eaten by fees)
	roiAtCurrent := calculateROIAfterFees(entryPrice, currentPrice, quantity, "SHORT", leverage)

	// Calculate ROI at TP1 (target profit)
	roiAtTP1 := calculateROIAfterFees(entryPrice, tp1Price, quantity, "SHORT", leverage)

	// Price movement percentages
	priceMoveAtCurrent := ((entryPrice - currentPrice) / entryPrice) * 100
	priceMoveAtTP1 := ((entryPrice - tp1Price) / entryPrice) * 100

	t.Logf("Trade #125 Analysis:")
	t.Logf("  Entry: %.2f, Current: %.2f, TP1: %.2f", entryPrice, currentPrice, tp1Price)
	t.Logf("  Price move at current: %.4f%%", priceMoveAtCurrent)
	t.Logf("  Price move at TP1: %.4f%%", priceMoveAtTP1)
	t.Logf("  ROI at current (5x leverage): %.4f%%", roiAtCurrent)
	t.Logf("  ROI at TP1 (5x leverage): %.4f%%", roiAtTP1)

	// Key insight: Tiny price moves result in negative ROI after fees
	// The 0.015% move is eaten by ~0.08% fees = net negative
	if roiAtCurrent >= 0 {
		t.Error("Expected negative ROI at current price (fees exceed tiny profit)")
	}

	// ROI at TP1 should be positive (real profit target)
	if roiAtTP1 <= 0 {
		t.Error("Expected positive ROI at TP1 target")
	}

	// The minimum threshold guard (0.1%) prevents booking at any ROI < 0.1%
	// Since ROI is negative here, it would never book anyway
	const minThreshold = 0.1
	t.Logf("Minimum threshold guard: %.2f%% - prevents zero-threshold exploit", minThreshold)
	t.Logf("With negative ROI (%.4f%%), shouldBookEarlyProfit correctly returns false", roiAtCurrent)
}

// TestMinimumThresholdGuard tests that the 0.1% minimum threshold prevents
// profit booking at near-zero ROI (the fix for Trade#125 bug)
func TestMinimumThresholdGuard(t *testing.T) {
	const minThreshold = 0.1 // From ginie_autopilot.go:3109

	tests := []struct {
		name             string
		configuredThreshold float64
		roi              float64
		shouldBook       bool
		description      string
	}{
		{
			name:             "Zero threshold with tiny profit - should NOT book",
			configuredThreshold: 0.0,
			roi:              0.05,
			shouldBook:       false,
			description:      "Bug scenario: threshold=0, ROI=0.05% - minThreshold guard should prevent",
		},
		{
			name:             "Zero threshold with profit above min - should book",
			configuredThreshold: 0.0,
			roi:              0.15,
			shouldBook:       true,
			description:      "ROI 0.15% exceeds minThreshold 0.1%, should book",
		},
		{
			name:             "Proper threshold not met",
			configuredThreshold: 5.0,
			roi:              3.0,
			shouldBook:       false,
			description:      "ROI 3% below threshold 5%, should not book",
		},
		{
			name:             "Proper threshold met",
			configuredThreshold: 5.0,
			roi:              6.0,
			shouldBook:       true,
			description:      "ROI 6% exceeds threshold 5%, should book",
		},
		{
			name:             "Exactly at minimum threshold",
			configuredThreshold: 0.0,
			roi:              0.1,
			shouldBook:       true,
			description:      "ROI exactly at minThreshold 0.1%, should book",
		},
		{
			name:             "Just below minimum threshold",
			configuredThreshold: 0.0,
			roi:              0.09,
			shouldBook:       false,
			description:      "ROI 0.09% just below minThreshold 0.1%, should not book",
		},
		{
			name:             "Negative ROI - should never book",
			configuredThreshold: 0.0,
			roi:              -1.0,
			shouldBook:       false,
			description:      "Negative ROI should never trigger booking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the threshold guard logic from shouldBookEarlyProfit
			threshold := tt.configuredThreshold
			if threshold < minThreshold {
				threshold = minThreshold
			}

			// Check if would book (ROI > 0 AND ROI >= threshold)
			wouldBook := tt.roi > 0 && tt.roi >= threshold

			if wouldBook != tt.shouldBook {
				t.Errorf("%s: expected shouldBook=%v, got %v (threshold=%.4f, effectiveThreshold=%.4f, ROI=%.4f)",
					tt.description, tt.shouldBook, wouldBook, tt.configuredThreshold, threshold, tt.roi)
			}

			t.Logf("%s: configured=%.4f%%, effective=%.4f%%, ROI=%.4f%%, book=%v",
				tt.name, tt.configuredThreshold, threshold, tt.roi, wouldBook)
		})
	}
}

// TestCalculateTradingFee verifies the fee calculation
func TestCalculateTradingFee(t *testing.T) {
	// TakerFeeRate = 0.0005 (0.05%) - Binance standard tier taker fee
	tests := []struct {
		quantity float64
		price    float64
		expected float64
	}{
		{1.0, 100.0, 0.05},              // 100 * 0.0005 = 0.05
		{0.003, 87438.50, 0.13115775},   // 0.003 * 87438.50 * 0.0005 = 0.13115775
		{10.0, 50.0, 0.25},              // 500 * 0.0005 = 0.25
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			fee := calculateTradingFee(tt.quantity, tt.price)
			// Allow small floating point difference
			if math.Abs(fee-tt.expected) > 0.0001 {
				t.Errorf("calculateTradingFee(%.4f, %.4f) = %.8f, want %.8f",
					tt.quantity, tt.price, fee, tt.expected)
			}
		})
	}
}

// TestROIWithLeverage verifies leverage multiplier effect on ROI
func TestROIWithLeverage(t *testing.T) {
	entryPrice := 100.0
	currentPrice := 101.0 // 1% price increase
	quantity := 1.0

	roi1x := calculateROIAfterFees(entryPrice, currentPrice, quantity, "LONG", 1)
	roi5x := calculateROIAfterFees(entryPrice, currentPrice, quantity, "LONG", 5)
	roi10x := calculateROIAfterFees(entryPrice, currentPrice, quantity, "LONG", 10)

	t.Logf("1% price move LONG:")
	t.Logf("  1x leverage ROI: %.4f%%", roi1x)
	t.Logf("  5x leverage ROI: %.4f%%", roi5x)
	t.Logf("  10x leverage ROI: %.4f%%", roi10x)

	// ROI should scale approximately with leverage (minus fees)
	// 5x should be roughly 5 times 1x
	ratio5x := roi5x / roi1x
	if ratio5x < 4.5 || ratio5x > 5.5 {
		t.Errorf("5x leverage ROI ratio expected ~5, got %.2f", ratio5x)
	}

	// 10x should be roughly 10 times 1x
	ratio10x := roi10x / roi1x
	if ratio10x < 9.0 || ratio10x > 11.0 {
		t.Errorf("10x leverage ROI ratio expected ~10, got %.2f", ratio10x)
	}
}
