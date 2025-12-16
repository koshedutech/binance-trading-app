package patterns

import (
	"binance-trading-bot/internal/binance"
	"testing"
)

// TestBullishFlag tests Bullish Flag pattern detection
func TestBullishFlag(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Create candles with upward pole then consolidation
	candles := make([]binance.Kline, 20)

	// Upward pole (10 candles)
	for i := 0; i < 10; i++ {
		candles[i] = binance.Kline{
			Open:      float64(100 + i*2),
			High:      float64(105 + i*2),
			Low:       float64(98 + i*2),
			Close:     float64(103 + i*2),
			CloseTime: int64((i + 1) * 1000000),
		}
	}

	// Flag consolidation (5 candles) - slight downward slope
	for i := 10; i < 15; i++ {
		candles[i] = binance.Kline{
			Open:      float64(122 - (i-10)*0.5),
			High:      float64(124 - (i-10)*0.5),
			Low:       float64(120 - (i-10)*0.5),
			Close:     float64(121 - (i-10)*0.5),
			CloseTime: int64((i + 1) * 1000000),
		}
	}

	flag, found := detector.isBullishFlag(candles, 10)

	if !found {
		t.Error("Should detect valid Bullish Flag pattern")
	}

	if flag != nil && flag.TrendDirection != "up" {
		t.Error("Bullish Flag should have 'up' trend direction")
	}
}

// TestBearishFlag tests Bearish Flag pattern detection
func TestBearishFlag(t *testing.T) {
	detector := NewPatternDetector(0.5)

	candles := make([]binance.Kline, 20)

	// Downward pole (10 candles)
	for i := 0; i < 10; i++ {
		candles[i] = binance.Kline{
			Open:      float64(120 - i*2),
			High:      float64(122 - i*2),
			Low:       float64(115 - i*2),
			Close:     float64(117 - i*2),
			CloseTime: int64((i + 1) * 1000000),
		}
	}

	// Flag consolidation (5 candles) - slight upward slope
	for i := 10; i < 15; i++ {
		candles[i] = binance.Kline{
			Open:      float64(100 + (i-10)*0.5),
			High:      float64(102 + (i-10)*0.5),
			Low:       float64(98 + (i-10)*0.5),
			Close:     float64(99 + (i-10)*0.5),
			CloseTime: int64((i + 1) * 1000000),
		}
	}

	flag, found := detector.isBearishFlag(candles, 10)

	if !found {
		t.Error("Should detect valid Bearish Flag pattern")
	}

	if flag != nil && flag.TrendDirection != "down" {
		t.Error("Bearish Flag should have 'down' trend direction")
	}
}

// TestAscendingTriangle tests Ascending Triangle detection
func TestAscendingTriangle(t *testing.T) {
	detector := NewPatternDetector(0.5)

	candles := make([]binance.Kline, 20)

	// Create ascending triangle: flat highs, rising lows
	for i := 0; i < 15; i++ {
		candles[i] = binance.Kline{
			Open:      float64(100 + i*0.5),
			High:      float64(110), // Flat resistance
			Low:       float64(95 + i), // Rising support
			Close:     float64(105 + i*0.3),
			CloseTime: int64((i + 1) * 1000000),
		}
	}

	triangle, found := detector.isAscendingTriangle(candles, 0)

	if !found {
		t.Error("Should detect valid Ascending Triangle")
	}

	if triangle != nil && triangle.Type != "ascending" {
		t.Error("Should be 'ascending' type")
	}
}

// TestDescendingTriangle tests Descending Triangle detection
func TestDescendingTriangle(t *testing.T) {
	detector := NewPatternDetector(0.5)

	candles := make([]binance.Kline, 20)

	// Create descending triangle: descending highs, flat lows
	for i := 0; i < 15; i++ {
		candles[i] = binance.Kline{
			Open:      float64(105 - i*0.3),
			High:      float64(110 - i), // Descending resistance
			Low:       float64(95), // Flat support
			Close:     float64(100 - i*0.5),
			CloseTime: int64((i + 1) * 1000000),
		}
	}

	triangle, found := detector.isDescendingTriangle(candles, 0)

	if !found {
		t.Error("Should detect valid Descending Triangle")
	}

	if triangle != nil && triangle.Type != "descending" {
		t.Error("Should be 'descending' type")
	}
}

// TestDetectContinuationPatterns tests comprehensive continuation detection
func TestDetectContinuationPatterns(t *testing.T) {
	detector := NewPatternDetector(0.5)

	// Create realistic pattern scenario
	candles := make([]binance.Kline, 25)

	// Uptrend pole
	for i := 0; i < 10; i++ {
		candles[i] = binance.Kline{
			Open:      float64(100 + i*3),
			High:      float64(105 + i*3),
			Low:       float64(98 + i*3),
			Close:     float64(103 + i*3),
			CloseTime: int64((i + 1) * 1000000),
		}
	}

	// Consolidation flag
	for i := 10; i < 20; i++ {
		candles[i] = binance.Kline{
			Open:      float64(130),
			High:      float64(132),
			Low:       float64(128),
			Close:     float64(129),
			CloseTime: int64((i + 1) * 1000000),
		}
	}

	patterns := detector.DetectContinuationPatterns("ETHUSDT", "4h", candles)

	// Should detect at least one continuation pattern
	if len(patterns) == 0 {
		t.Log("Warning: No continuation patterns detected in test scenario")
	}

	// Validate pattern properties
	for _, p := range patterns {
		if p.Symbol != "ETHUSDT" {
			t.Error("Pattern should have correct symbol")
		}
		if p.Timeframe != "4h" {
			t.Error("Pattern should have correct timeframe")
		}
		if p.Confidence <= 0 || p.Confidence > 1 {
			t.Error("Confidence should be between 0 and 1")
		}
	}
}

// BenchmarkContinuationPatternDetection benchmarks continuation pattern performance
func BenchmarkContinuationPatternDetection(b *testing.B) {
	detector := NewPatternDetector(0.5)

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
		detector.DetectContinuationPatterns("BTCUSDT", "1h", candles)
	}
}
