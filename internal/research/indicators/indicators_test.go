// Package indicators provides technical indicator calculations for research infrastructure.
// Story 15.4: Feature Calculator Engine - Indicator Tests
package indicators

import (
	"math"
	"testing"
	"time"
)

// createTestOHLCV creates test OHLCV data for indicator calculations.
func createTestOHLCV(count int) []OHLCV {
	data := make([]OHLCV, count)
	basePrice := 100.0

	for i := 0; i < count; i++ {
		priceChange := float64(i%10) * 0.5
		if i%3 == 0 {
			priceChange = -priceChange
		}

		open := basePrice + priceChange
		close := open + (float64(i%5)-2)*0.3
		high := math.Max(open, close) + float64(i%3)*0.2
		low := math.Min(open, close) - float64(i%3)*0.2

		data[i] = OHLCV{
			Open:           open,
			High:           high,
			Low:            low,
			Close:          close,
			Volume:         1000.0 + float64(i%20)*100,
			TakerBuyVolume: 500.0 + float64(i%10)*50,
			TradeCount:     100 + i%50,
		}
	}

	return data
}

// TestSafeDiv tests the safe division function.
func TestSafeDiv(t *testing.T) {
	tests := []struct {
		name        string
		numerator   float64
		denominator float64
		wantNaN     bool
		want        float64
	}{
		{"normal division", 10, 2, false, 5},
		{"division by zero", 10, 0, true, 0},
		{"zero divided", 0, 5, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeDiv(tt.numerator, tt.denominator)
			if tt.wantNaN {
				if !IsNaN(result) {
					t.Errorf("Expected NaN, got %f", result)
				}
			} else {
				if result != tt.want {
					t.Errorf("Expected %f, got %f", tt.want, result)
				}
			}
		})
	}
}

// TestCalculateReturn tests the return calculation.
func TestCalculateReturn(t *testing.T) {
	closes := []float64{100, 105, 110, 108, 112, 115}

	tests := []struct {
		name     string
		period   int
		wantNaN  bool
		wantSign int // -1 negative, 0 zero, 1 positive
	}{
		{"1-period return", 1, false, 1},  // 115/112 - 1 > 0
		{"3-period return", 3, false, 1},  // 115/110 - 1 > 0
		{"5-period return", 5, false, 1},  // 115/100 - 1 > 0
		{"period too long", 10, true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateReturn(closes, tt.period)
			if tt.wantNaN {
				if !IsNaN(result) {
					t.Errorf("Expected NaN, got %f", result)
				}
			} else {
				if IsNaN(result) {
					t.Error("Expected non-NaN result")
				} else if tt.wantSign > 0 && result <= 0 {
					t.Errorf("Expected positive return, got %f", result)
				} else if tt.wantSign < 0 && result >= 0 {
					t.Errorf("Expected negative return, got %f", result)
				}
			}
		})
	}
}

// TestCalculateBodyRatio tests the body ratio calculation.
func TestCalculateBodyRatio(t *testing.T) {
	tests := []struct {
		name     string
		open     float64
		high     float64
		low      float64
		close    float64
		expected float64
	}{
		{"full body candle", 100, 105, 100, 105, 1.0},
		{"doji", 100, 101, 99, 100, 0.0},
		{"half body", 100, 102, 98, 102, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBodyRatio(tt.open, tt.high, tt.low, tt.close)
			if math.Abs(result-tt.expected) > 0.01 {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

// TestCalculateRSI tests the RSI calculation.
func TestCalculateRSI(t *testing.T) {
	// Create trending data for RSI test
	upTrend := make([]float64, 20)
	for i := 0; i < 20; i++ {
		upTrend[i] = 100 + float64(i)*2 // Steady uptrend
	}

	downTrend := make([]float64, 20)
	for i := 0; i < 20; i++ {
		downTrend[i] = 200 - float64(i)*2 // Steady downtrend
	}

	t.Run("uptrend RSI", func(t *testing.T) {
		rsi := CalculateRSI(upTrend, 14)
		if IsNaN(rsi) {
			t.Error("Expected non-NaN RSI")
			return
		}
		if rsi <= 50 {
			t.Errorf("Expected RSI > 50 for uptrend, got %f", rsi)
		}
	})

	t.Run("downtrend RSI", func(t *testing.T) {
		rsi := CalculateRSI(downTrend, 14)
		if IsNaN(rsi) {
			t.Error("Expected non-NaN RSI")
			return
		}
		if rsi >= 50 {
			t.Errorf("Expected RSI < 50 for downtrend, got %f", rsi)
		}
	})

	t.Run("RSI bounds", func(t *testing.T) {
		data := createTestOHLCV(50)
		closes := make([]float64, len(data))
		for i, d := range data {
			closes[i] = d.Close
		}
		rsi := CalculateRSI(closes, 14)
		if !IsNaN(rsi) && (rsi < 0 || rsi > 100) {
			t.Errorf("RSI should be in [0,100], got %f", rsi)
		}
	})
}

// TestCalculateATR tests the ATR calculation.
func TestCalculateATR(t *testing.T) {
	data := createTestOHLCV(50)
	highs := make([]float64, len(data))
	lows := make([]float64, len(data))
	closes := make([]float64, len(data))

	for i, d := range data {
		highs[i] = d.High
		lows[i] = d.Low
		closes[i] = d.Close
	}

	atr := CalculateATR(highs, lows, closes, 14)

	if IsNaN(atr) {
		t.Error("Expected non-NaN ATR")
		return
	}

	if atr <= 0 {
		t.Errorf("ATR should be positive, got %f", atr)
	}
}

// TestCalculateBollingerBands tests the Bollinger Bands calculation.
func TestCalculateBollingerBands(t *testing.T) {
	data := createTestOHLCV(50)
	closes := make([]float64, len(data))
	for i, d := range data {
		closes[i] = d.Close
	}

	bb := CalculateBollingerBands(closes, 20, 2.0)

	if IsNaN(bb.Upper) || IsNaN(bb.Middle) || IsNaN(bb.Lower) {
		t.Error("Expected non-NaN Bollinger Bands")
		return
	}

	// Upper should be > middle > lower
	if bb.Upper <= bb.Middle {
		t.Errorf("Upper (%f) should be > middle (%f)", bb.Upper, bb.Middle)
	}
	if bb.Middle <= bb.Lower {
		t.Errorf("Middle (%f) should be > lower (%f)", bb.Middle, bb.Lower)
	}
}

// TestCalculateStochastic tests the Stochastic Oscillator calculation.
func TestCalculateStochastic(t *testing.T) {
	data := createTestOHLCV(50)
	highs := make([]float64, len(data))
	lows := make([]float64, len(data))
	closes := make([]float64, len(data))

	for i, d := range data {
		highs[i] = d.High
		lows[i] = d.Low
		closes[i] = d.Close
	}

	k, d := CalculateStochastic(highs, lows, closes, 14, 3)

	if IsNaN(k) || IsNaN(d) {
		t.Error("Expected non-NaN Stochastic values")
		return
	}

	// Stochastic values should be in [0, 100]
	if k < 0 || k > 100 {
		t.Errorf("Stochastic K should be in [0,100], got %f", k)
	}
	if d < 0 || d > 100 {
		t.Errorf("Stochastic D should be in [0,100], got %f", d)
	}
}

// TestCalculateMACD tests the MACD calculation.
func TestCalculateMACD(t *testing.T) {
	data := createTestOHLCV(50)
	closes := make([]float64, len(data))
	for i, d := range data {
		closes[i] = d.Close
	}

	macdLine, signal, histogram := CalculateMACD(closes, 12, 26, 9)

	if IsNaN(macdLine) || IsNaN(signal) || IsNaN(histogram) {
		t.Error("Expected non-NaN MACD values")
		return
	}

	// Histogram should equal MACD - Signal
	expectedHist := macdLine - signal
	if math.Abs(histogram-expectedHist) > 0.001 {
		t.Errorf("Histogram (%f) should equal MACD - Signal (%f)", histogram, expectedHist)
	}
}

// TestCalculateVolumeFeatures tests the volume features calculation.
func TestCalculateVolumeFeatures(t *testing.T) {
	data := createTestOHLCV(50)
	features := CalculateVolumeFeatures(data)

	if features == nil {
		t.Fatal("Expected non-nil volume features")
	}

	// Taker buy ratio should be in [0, 1]
	if !IsNaN(features.TakerBuyRatio) {
		if features.TakerBuyRatio < 0 || features.TakerBuyRatio > 1 {
			t.Errorf("TakerBuyRatio should be in [0,1], got %f", features.TakerBuyRatio)
		}
	}

	// Volume ratios should be positive (when valid)
	for _, ratio := range []float64{features.VolumeRatio5, features.VolumeRatio10} {
		if !IsNaN(ratio) && ratio < 0 {
			t.Errorf("Volume ratio should be positive, got %f", ratio)
		}
	}
}

// TestCalculateTrendFeatures tests the trend features calculation.
func TestCalculateTrendFeatures(t *testing.T) {
	data := createTestOHLCV(100)
	features := CalculateTrendFeatures(data)

	if features == nil {
		t.Fatal("Expected non-nil trend features")
	}

	// SMA values should be in reasonable range
	lastClose := data[len(data)-1].Close
	for _, sma := range []float64{features.SMA10, features.SMA20} {
		if !IsNaN(sma) {
			// SMA should be within 50% of current price
			diff := math.Abs(sma-lastClose) / lastClose
			if diff > 0.5 {
				t.Errorf("SMA (%f) seems too far from close (%f)", sma, lastClose)
			}
		}
	}

	// Trend direction should be -1, 0, or 1
	if features.TrendDirection < -1 || features.TrendDirection > 1 {
		t.Errorf("TrendDirection should be in [-1,1], got %d", features.TrendDirection)
	}
}

// TestCalculateTimeFeatures tests the time features calculation.
func TestCalculateTimeFeatures(t *testing.T) {
	// Test with a known timestamp
	testTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC) // Monday 14:30 UTC

	features := CalculateTimeFeatures(testTime)

	if features.HourOfDay != 14 {
		t.Errorf("Expected hour 14, got %d", features.HourOfDay)
	}

	if features.DayOfWeek != 1 { // Monday
		t.Errorf("Expected day of week 1 (Monday), got %d", features.DayOfWeek)
	}

	if features.IsWeekend {
		t.Error("Monday should not be weekend")
	}

	if features.DayOfMonth != 15 {
		t.Errorf("Expected day of month 15, got %d", features.DayOfMonth)
	}
}

// TestGetTradingSession tests the trading session detection.
func TestGetTradingSession(t *testing.T) {
	tests := []struct {
		name      string
		hour      int
		dayOfWeek int
		expected  TradingSession
	}{
		{"Asian session", 3, 1, SessionAsian},
		{"European session", 10, 2, SessionEurope},
		{"American session", 18, 3, SessionAmerica},
		{"Overlap period", 14, 4, SessionOverlap},
		{"Weekend Saturday", 12, 6, SessionOff},
		{"Weekend Sunday", 12, 0, SessionOff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTradingSession(tt.hour, tt.dayOfWeek)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestIsMonthEnd tests the month end detection.
func TestIsMonthEnd(t *testing.T) {
	tests := []struct {
		name     string
		date     time.Time
		expected bool
	}{
		{"January 31", time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), true},
		{"January 30", time.Date(2024, 1, 30, 0, 0, 0, 0, time.UTC), true},
		{"January 29", time.Date(2024, 1, 29, 0, 0, 0, 0, time.UTC), true},
		{"January 28", time.Date(2024, 1, 28, 0, 0, 0, 0, time.UTC), false},
		{"January 15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), false},
		{"February 29 (leap year)", time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMonthEnd(tt.date)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestOBV tests the On-Balance Volume calculation.
func TestOBV(t *testing.T) {
	// Create data with alternating up/down days
	closes := []float64{100, 102, 101, 103, 102, 104}
	volumes := []float64{1000, 1500, 1200, 1800, 1100, 2000}

	obv := CalculateOBV(closes, volumes)

	if IsNaN(obv) {
		t.Error("Expected non-NaN OBV")
		return
	}

	// OBV should be positive because there are more up moves
	// 102>100: +1500, 101<102: -1200, 103>101: +1800, 102<103: -1100, 104>102: +2000
	// Net: 1500 - 1200 + 1800 - 1100 + 2000 = 3000
	expectedOBV := 3000.0
	if math.Abs(obv-expectedOBV) > 0.01 {
		t.Errorf("Expected OBV %f, got %f", expectedOBV, obv)
	}
}

// TestAccumulationDistribution tests the A/D calculation.
func TestAccumulationDistribution(t *testing.T) {
	// Test case: close at the top of the range
	ad1 := CalculateAccumulationDistribution(110, 100, 110, 1000)
	if ad1 <= 0 {
		t.Errorf("A/D should be positive when close = high, got %f", ad1)
	}

	// Test case: close at the bottom of the range
	ad2 := CalculateAccumulationDistribution(110, 100, 100, 1000)
	if ad2 >= 0 {
		t.Errorf("A/D should be negative when close = low, got %f", ad2)
	}

	// Test case: close in the middle
	ad3 := CalculateAccumulationDistribution(110, 100, 105, 1000)
	if math.Abs(ad3) > 1 {
		// Should be close to zero
		t.Logf("A/D for mid-range close: %f", ad3)
	}
}

// BenchmarkCalculateRSI benchmarks the RSI calculation.
func BenchmarkCalculateRSI(b *testing.B) {
	data := createTestOHLCV(100)
	closes := make([]float64, len(data))
	for i, d := range data {
		closes[i] = d.Close
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateRSI(closes, 14)
	}
}

// BenchmarkCalculateMACD benchmarks the MACD calculation.
func BenchmarkCalculateMACD(b *testing.B) {
	data := createTestOHLCV(100)
	closes := make([]float64, len(data))
	for i, d := range data {
		closes[i] = d.Close
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateMACD(closes, 12, 26, 9)
	}
}

// BenchmarkCalculateBollingerBands benchmarks the Bollinger Bands calculation.
func BenchmarkCalculateBollingerBands(b *testing.B) {
	data := createTestOHLCV(100)
	closes := make([]float64, len(data))
	for i, d := range data {
		closes[i] = d.Close
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateBollingerBands(closes, 20, 2.0)
	}
}

// TestCyclicalEncoding tests the cyclical hour/day encoding functions.
func TestCyclicalEncoding(t *testing.T) {
	// Test cyclical hour encoding
	t.Run("cyclical hour encoding", func(t *testing.T) {
		// Hour 0 and hour 24 should produce the same values (cyclical)
		// Since we use hour 0-23, hour 0 represents the cycle start

		// sin(0) should be 0, cos(0) should be 1
		sin0 := CyclicalHourSin(0)
		cos0 := CyclicalHourCos(0)
		if math.Abs(sin0) > 0.01 {
			t.Errorf("sin(hour=0) should be ~0, got %f", sin0)
		}
		if math.Abs(cos0-1.0) > 0.01 {
			t.Errorf("cos(hour=0) should be ~1, got %f", cos0)
		}

		// Hour 6 (1/4 of day): sin should be ~1, cos should be ~0
		sin6 := CyclicalHourSin(6)
		cos6 := CyclicalHourCos(6)
		if math.Abs(sin6-1.0) > 0.01 {
			t.Errorf("sin(hour=6) should be ~1, got %f", sin6)
		}
		if math.Abs(cos6) > 0.01 {
			t.Errorf("cos(hour=6) should be ~0, got %f", cos6)
		}

		// Hour 12 (1/2 of day): sin should be ~0, cos should be ~-1
		sin12 := CyclicalHourSin(12)
		cos12 := CyclicalHourCos(12)
		if math.Abs(sin12) > 0.01 {
			t.Errorf("sin(hour=12) should be ~0, got %f", sin12)
		}
		if math.Abs(cos12+1.0) > 0.01 {
			t.Errorf("cos(hour=12) should be ~-1, got %f", cos12)
		}

		// Verify sin^2 + cos^2 = 1 for several hours
		for h := 0; h < 24; h++ {
			sinH := CyclicalHourSin(h)
			cosH := CyclicalHourCos(h)
			sumSquares := sinH*sinH + cosH*cosH
			if math.Abs(sumSquares-1.0) > 0.01 {
				t.Errorf("sin^2 + cos^2 should be 1 for hour %d, got %f", h, sumSquares)
			}
		}
	})

	// Test cyclical day encoding
	t.Run("cyclical day encoding", func(t *testing.T) {
		// Day 0 (Sunday): sin should be ~0, cos should be ~1
		sin0 := CyclicalDaySin(0)
		cos0 := CyclicalDayCos(0)
		if math.Abs(sin0) > 0.01 {
			t.Errorf("sin(day=0) should be ~0, got %f", sin0)
		}
		if math.Abs(cos0-1.0) > 0.01 {
			t.Errorf("cos(day=0) should be ~1, got %f", cos0)
		}

		// Verify sin^2 + cos^2 = 1 for all days
		for d := 0; d < 7; d++ {
			sinD := CyclicalDaySin(d)
			cosD := CyclicalDayCos(d)
			sumSquares := sinD*sinD + cosD*cosD
			if math.Abs(sumSquares-1.0) > 0.01 {
				t.Errorf("sin^2 + cos^2 should be 1 for day %d, got %f", d, sumSquares)
			}
		}
	})
}

// TestSafePercentEdgeCases tests SafePercent with edge cases.
func TestSafePercentEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		current  float64
		previous float64
		wantNaN  bool
	}{
		{"normal percentage", 110, 100, false},
		{"zero previous", 100, 0, true},
		{"tiny previous causing overflow", 1e300, 1e-300, true},
		{"both zero", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafePercent(tt.current, tt.previous)
			if tt.wantNaN && !math.IsNaN(result) && !math.IsInf(result, 0) {
				t.Errorf("Expected NaN or Inf handling, got %f", result)
			}
			if !tt.wantNaN && (math.IsNaN(result) || math.IsInf(result, 0)) {
				t.Errorf("Expected valid result, got NaN/Inf")
			}
		})
	}
}
