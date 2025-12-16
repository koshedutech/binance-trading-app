package patterns

import (
	"binance-trading-bot/internal/binance"
	"testing"
)

// TestBullishEngulfing tests Bullish Engulfing pattern detection
func TestBullishEngulfing(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Valid Bullish Engulfing
	c1 := binance.Kline{Open: 100, High: 102, Low: 98, Close: 99, CloseTime: 1000000}  // Bearish
	c2 := binance.Kline{Open: 98, High: 105, Low: 97, Close: 104, CloseTime: 2000000} // Bullish engulfing

	if !detector.isBullishEngulfing(c1, c2) {
		t.Error("Should detect valid Bullish Engulfing pattern")
	}

	// Invalid - C1 not bearish
	c1Invalid := binance.Kline{Open: 99, High: 102, Low: 98, Close: 100, CloseTime: 1000000}
	if detector.isBullishEngulfing(c1Invalid, c2) {
		t.Error("Should NOT detect pattern when C1 is not bearish")
	}

	// Invalid - C2 doesn't engulf C1
	c2Invalid := binance.Kline{Open: 99, High: 101, Low: 98, Close: 100, CloseTime: 2000000}
	if detector.isBullishEngulfing(c1, c2Invalid) {
		t.Error("Should NOT detect pattern when C2 doesn't engulf C1")
	}
}

// TestBearishEngulfing tests Bearish Engulfing pattern detection
func TestBearishEngulfing(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Valid Bearish Engulfing
	c1 := binance.Kline{Open: 99, High: 102, Low: 98, Close: 100, CloseTime: 1000000}  // Bullish
	c2 := binance.Kline{Open: 101, High: 103, Low: 95, Close: 96, CloseTime: 2000000} // Bearish engulfing

	if !detector.isBearishEngulfing(c1, c2) {
		t.Error("Should detect valid Bearish Engulfing pattern")
	}
}

// TestDoji tests Doji pattern detection
func TestDoji(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Valid Doji - open and close nearly same
	doji := binance.Kline{Open: 100, High: 102, Low: 98, Close: 100.5, CloseTime: 1000000}
	if !detector.isDoji(doji) {
		t.Error("Should detect valid Doji pattern")
	}

	// Invalid - large body
	notDoji := binance.Kline{Open: 100, High: 110, Low: 98, Close: 108, CloseTime: 1000000}
	if detector.isDoji(notDoji) {
		t.Error("Should NOT detect Doji with large body")
	}
}

// TestDragonflyDoji tests Dragonfly Doji pattern
func TestDragonflyDoji(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Valid Dragonfly - long lower wick, small body at top
	dragonfly := binance.Kline{Open: 100, High: 100.5, Low: 92, Close: 100, CloseTime: 1000000}
	if !detector.isDragonflyDoji(dragonfly) {
		t.Error("Should detect valid Dragonfly Doji")
	}

	// Invalid - has upper wick
	notDragonfly := binance.Kline{Open: 100, High: 105, Low: 92, Close: 100, CloseTime: 1000000}
	if detector.isDragonflyDoji(notDragonfly) {
		t.Error("Should NOT detect Dragonfly with upper wick")
	}
}

// TestGravestoneDoji tests Gravestone Doji pattern
func TestGravestoneDoji(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Valid Gravestone - long upper wick, small body at bottom
	gravestone := binance.Kline{Open: 100, High: 108, Low: 99.5, Close: 100, CloseTime: 1000000}
	if !detector.isGravestoneDoji(gravestone) {
		t.Error("Should detect valid Gravestone Doji")
	}
}

// TestBullishHarami tests Bullish Harami pattern
func TestBullishHarami(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Valid Bullish Harami
	c1 := binance.Kline{Open: 105, High: 106, Low: 95, Close: 96, CloseTime: 1000000}  // Large bearish
	c2 := binance.Kline{Open: 98, High: 100, Low: 97, Close: 99, CloseTime: 2000000}  // Small bullish inside C1

	if !detector.isBullishHarami(c1, c2) {
		t.Error("Should detect valid Bullish Harami")
	}

	// Invalid - C2 too large
	c2Large := binance.Kline{Open: 96, High: 104, Low: 95, Close: 103, CloseTime: 2000000}
	if detector.isBullishHarami(c1, c2Large) {
		t.Error("Should NOT detect Harami when C2 is too large")
	}
}

// TestBearishHarami tests Bearish Harami pattern
func TestBearishHarami(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Valid Bearish Harami
	c1 := binance.Kline{Open: 96, High: 106, Low: 95, Close: 105, CloseTime: 1000000}  // Large bullish
	c2 := binance.Kline{Open: 103, High: 104, Low: 101, Close: 102, CloseTime: 2000000} // Small bearish inside

	if !detector.isBearishHarami(c1, c2) {
		t.Error("Should detect valid Bearish Harami")
	}
}

// TestHangingMan tests Hanging Man pattern
func TestHangingMan(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Valid Hanging Man - appears after uptrend
	prevCandle := binance.Kline{Open: 95, High: 100, Low: 94, Close: 99, CloseTime: 1000000} // Bullish
	hangingMan := binance.Kline{Open: 100, High: 101, Low: 92, Close: 100, CloseTime: 2000000}

	if !detector.isHangingMan(hangingMan, &prevCandle) {
		t.Error("Should detect valid Hanging Man after uptrend")
	}

	// Invalid - appears after downtrend
	prevBearish := binance.Kline{Open: 100, High: 101, Low: 95, Close: 96, CloseTime: 1000000}
	if detector.isHangingMan(hangingMan, &prevBearish) {
		t.Error("Should NOT detect Hanging Man after downtrend")
	}
}

// TestDetectReversalPatterns tests comprehensive reversal detection
func TestDetectReversalPatterns(t *testing.T) {
	detector := NewPatternDetector(0.5)

	candles := []binance.Kline{
		{Open: 100, High: 105, Low: 99, Close: 104, CloseTime: 1000000},  // Bullish
		{Open: 104, High: 106, Low: 98, Close: 99, CloseTime: 2000000},   // Bearish
		{Open: 98, High: 105, Low: 97, Close: 103, CloseTime: 3000000},   // Bullish Engulfing
	}

	patterns := detector.DetectReversalPatterns("BTCUSDT", "1h", candles)

	// Should find at least the engulfing pattern
	found := false
	for _, p := range patterns {
		if p.Type == BullishEngulfing {
			found = true
			if p.Direction != "bullish" {
				t.Error("Bullish Engulfing should have bullish direction")
			}
			if p.Confidence <= 0 || p.Confidence > 1 {
				t.Error("Confidence should be between 0 and 1")
			}
		}
	}

	if !found {
		t.Error("Should detect Bullish Engulfing in test candles")
	}
}

// BenchmarkPatternDetection benchmarks pattern detection performance
func BenchmarkReversalPatternDetection(b *testing.B) {
	detector := NewPatternDetector(0.5)

	// Generate 100 candles
	candles := make([]binance.Kline, 100)
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
		detector.DetectReversalPatterns("BTCUSDT", "1h", candles)
	}
}
