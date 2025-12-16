package analysis

import (
	"testing"
	"binance-trading-bot/internal/binance"
)

// TestDetectBullishFVG tests detection of bullish Fair Value Gaps
func TestDetectBullishFVG(t *testing.T) {
	detector := NewFVGDetector(0.1)

	candles := []binance.Kline{
		// Candle 1: High at 100
		{Open: 95, High: 100, Low: 94, Close: 98, CloseTime: 1000000},
		// Candle 2: Gap creator (middle candle)
		{Open: 98, High: 105, Low: 97, Close: 104, CloseTime: 2000000},
		// Candle 3: Low at 101 (gap between 100 and 101)
		{Open: 104, High: 108, Low: 101, Close: 106, CloseTime: 3000000},
	}

	fvgs := detector.DetectFVGs("BTCUSDT", "1h", candles)

	if len(fvgs) != 1 {
		t.Fatalf("Expected 1 FVG, got %d", len(fvgs))
	}

	fvg := fvgs[0]

	if fvg.Type != BullishFVG {
		t.Errorf("Expected BullishFVG, got %s", fvg.Type)
	}

	if fvg.BottomPrice != 100 {
		t.Errorf("Expected BottomPrice 100, got %f", fvg.BottomPrice)
	}

	if fvg.TopPrice != 101 {
		t.Errorf("Expected TopPrice 101, got %f", fvg.TopPrice)
	}

	if fvg.Filled {
		t.Error("FVG should not be marked as filled initially")
	}
}

// TestDetectBearishFVG tests detection of bearish Fair Value Gaps
func TestDetectBearishFVG(t *testing.T) {
	detector := NewFVGDetector(0.1)

	candles := []binance.Kline{
		// Candle 1: Low at 100
		{Open: 105, High: 106, Low: 100, Close: 102, CloseTime: 1000000},
		// Candle 2: Gap creator
		{Open: 102, High: 103, Low: 95, Close: 96, CloseTime: 2000000},
		// Candle 3: High at 99 (gap between 99 and 100)
		{Open: 96, High: 99, Low: 92, Close: 94, CloseTime: 3000000},
	}

	fvgs := detector.DetectFVGs("BTCUSDT", "1h", candles)

	if len(fvgs) != 1 {
		t.Fatalf("Expected 1 FVG, got %d", len(fvgs))
	}

	fvg := fvgs[0]

	if fvg.Type != BearishFVG {
		t.Errorf("Expected BearishFVG, got %s", fvg.Type)
	}

	if fvg.BottomPrice != 99 {
		t.Errorf("Expected BottomPrice 99, got %f", fvg.BottomPrice)
	}

	if fvg.TopPrice != 100 {
		t.Errorf("Expected TopPrice 100, got %f", fvg.TopPrice)
	}
}

// TestNoFVGDetection tests that no FVG is detected when candles overlap
func TestNoFVGDetection(t *testing.T) {
	detector := NewFVGDetector(0.1)

	candles := []binance.Kline{
		// Overlapping candles - no gap
		{Open: 95, High: 100, Low: 94, Close: 98, CloseTime: 1000000},
		{Open: 98, High: 102, Low: 97, Close: 100, CloseTime: 2000000},
		{Open: 100, High: 104, Low: 99, Close: 102, CloseTime: 3000000},
	}

	fvgs := detector.DetectFVGs("BTCUSDT", "1h", candles)

	if len(fvgs) != 0 {
		t.Errorf("Expected 0 FVGs for overlapping candles, got %d", len(fvgs))
	}
}

// TestIsPriceInFVG tests price proximity detection
func TestIsPriceInFVG(t *testing.T) {
	detector := NewFVGDetector(0.1)

	fvg := FVG{
		Type:        BullishFVG,
		TopPrice:    105,
		BottomPrice: 100,
	}

	tests := []struct {
		price    float64
		expected bool
	}{
		{102.5, true},  // Inside FVG
		{100, true},    // At bottom
		{105, true},    // At top
		{99, false},    // Below FVG
		{106, false},   // Above FVG
	}

	for _, tt := range tests {
		result := detector.IsPriceInFVG(tt.price, fvg)
		if result != tt.expected {
			t.Errorf("IsPriceInFVG(%f) = %v, expected %v", tt.price, result, tt.expected)
		}
	}
}

// TestUpdateFVGStatus tests FVG fill detection
func TestUpdateFVGStatus_BullishFVGFilled(t *testing.T) {
	detector := NewFVGDetector(0.1)

	fvg := FVG{
		Type:        BullishFVG,
		TopPrice:    105,
		BottomPrice: 100,
		Filled:      false,
	}

	// Candle that wicks down into the FVG
	newCandles := []binance.Kline{
		{Open: 110, High: 112, Low: 102, Close: 108, CloseTime: 4000000},
	}

	detector.UpdateFVGStatus(&fvg, newCandles)

	if !fvg.Filled {
		t.Error("FVG should be marked as filled after price entered the zone")
	}

	if fvg.FilledPrice == nil {
		t.Error("FilledPrice should be set")
	} else if *fvg.FilledPrice != 102 {
		t.Errorf("Expected FilledPrice 102, got %f", *fvg.FilledPrice)
	}
}

// TestMinGapPercent tests minimum gap size filtering
func TestMinGapPercent(t *testing.T) {
	detector := NewFVGDetector(5.0) // 5% minimum gap

	candles := []binance.Kline{
		// Small gap (< 5%)
		{Open: 100, High: 100.5, Low: 99.5, Close: 100, CloseTime: 1000000},
		{Open: 100, High: 102, Low: 99, Close: 101, CloseTime: 2000000},
		{Open: 101, High: 102, Low: 100.6, Close: 101.5, CloseTime: 3000000}, // Gap of 0.1
	}

	fvgs := detector.DetectFVGs("BTCUSDT", "1h", candles)

	if len(fvgs) != 0 {
		t.Errorf("Expected 0 FVGs with small gap, got %d", len(fvgs))
	}
}

// BenchmarkDetectFVGs benchmarks FVG detection performance
func BenchmarkDetectFVGs(b *testing.B) {
	detector := NewFVGDetector(0.1)

	// Generate 1000 candles
	candles := make([]binance.Kline, 1000)
	for i := range candles {
		candles[i] = binance.Kline{
			Open:      float64(100 + i),
			High:      float64(105 + i),
			Low:       float64(95 + i),
			Close:     float64(102 + i),
			CloseTime: int64((i + 1) * 1000000),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectFVGs("BTCUSDT", "1h", candles)
	}
}
