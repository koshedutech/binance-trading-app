// Package research provides data structures and services for research infrastructure.
// Story 15.4: Feature Calculator Engine - Tests
package research

import (
	"math"
	"testing"
	"time"
)

// createTestCandles creates a slice of test candles for feature calculation.
func createTestCandles(count int) []*ResearchCandle {
	candles := make([]*ResearchCandle, count)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	basePrice := 100.0

	for i := 0; i < count; i++ {
		// Create some price variation
		priceChange := float64(i%10) * 0.5
		if i%3 == 0 {
			priceChange = -priceChange
		}

		open := basePrice + priceChange
		close := open + (float64(i%5) - 2) * 0.3
		high := math.Max(open, close) + float64(i%3)*0.2
		low := math.Min(open, close) - float64(i%3)*0.2

		volume := 1000.0 + float64(i%20)*100
		takerBuyVolume := volume * 0.5 // 50% buy ratio

		candles[i] = &ResearchCandle{
			ID:             int64(i + 1),
			Symbol:         "BTCUSDT",
			Timeframe:      Timeframe15m,
			OpenTime:       baseTime.Add(time.Duration(i) * 15 * time.Minute),
			CloseTime:      baseTime.Add(time.Duration(i+1) * 15 * time.Minute),
			Open:           open,
			High:           high,
			Low:            low,
			Close:          close,
			Volume:         volume,
			QuoteVolume:    volume * close,
			TradeCount:     100 + i%50,
			TakerBuyVolume: takerBuyVolume,
			TakerBuyQuote:  takerBuyVolume * close,
			CreatedAt:      time.Now(),
		}
	}

	return candles
}

func TestFeatureCalculator_Calculate(t *testing.T) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())

	t.Run("with sufficient candles", func(t *testing.T) {
		candles := createTestCandles(100)
		features, err := fc.Calculate(candles)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if features == nil {
			t.Fatal("Expected features, got nil")
		}

		// Check identification
		if features.Symbol != "BTCUSDT" {
			t.Errorf("Expected symbol BTCUSDT, got %s", features.Symbol)
		}
		if features.Timeframe != Timeframe15m {
			t.Errorf("Expected timeframe 15m, got %s", features.Timeframe)
		}

		// Check that feature categories are populated
		if features.Price == nil {
			t.Error("Price features should not be nil")
		}
		if features.Volume == nil {
			t.Error("Volume features should not be nil")
		}
		if features.Volatility == nil {
			t.Error("Volatility features should not be nil")
		}
		if features.Momentum == nil {
			t.Error("Momentum features should not be nil")
		}
		if features.Trend == nil {
			t.Error("Trend features should not be nil")
		}
		if features.Time == nil {
			t.Error("Time features should not be nil")
		}

		// Check IsComplete flag
		if !features.IsComplete {
			t.Error("Features should be complete with 100 candles")
		}
	})

	t.Run("with minimum candles", func(t *testing.T) {
		candles := createTestCandles(MinimumLookback)
		features, err := fc.Calculate(candles)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if !features.IsComplete {
			t.Error("Features should be complete with minimum lookback")
		}
	})

	t.Run("with insufficient candles", func(t *testing.T) {
		candles := createTestCandles(10)
		features, err := fc.Calculate(candles)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Features should still be calculated but marked incomplete
		if features.IsComplete {
			t.Error("Features should not be complete with only 10 candles")
		}
	})

	t.Run("with empty candles", func(t *testing.T) {
		_, err := fc.Calculate([]*ResearchCandle{})

		if err == nil {
			t.Error("Expected error for empty candles")
		}
	})
}

func TestFeatureCalculator_CalculateBatch(t *testing.T) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())

	t.Run("batch calculation", func(t *testing.T) {
		candles := createTestCandles(100)
		targetCount := 10

		features, err := fc.CalculateBatch(candles, targetCount)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(features) != targetCount {
			t.Errorf("Expected %d features, got %d", targetCount, len(features))
		}

		// Check that all features are populated
		for i, f := range features {
			if f == nil {
				t.Errorf("Feature at index %d is nil", i)
				continue
			}
			if f.Price == nil {
				t.Errorf("Feature at index %d has nil Price", i)
			}
		}
	})

	t.Run("insufficient candles for batch", func(t *testing.T) {
		candles := createTestCandles(30)
		_, err := fc.CalculateBatch(candles, 10)

		if err == nil {
			t.Error("Expected error for insufficient candles")
		}
	})

	t.Run("target count exceeds available", func(t *testing.T) {
		candles := createTestCandles(70) // Just above minimum
		features, err := fc.CalculateBatch(candles, 100) // Request more than available

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Should get fewer than requested
		if len(features) >= 100 {
			t.Errorf("Expected fewer than 100 features, got %d", len(features))
		}
	})
}

func TestCandleFeatures_ToFeatureVector(t *testing.T) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
	candles := createTestCandles(100)

	features, err := fc.Calculate(candles)
	if err != nil {
		t.Fatalf("Failed to calculate features: %v", err)
	}

	values, names := features.ToFeatureVector()

	// Check that we have the expected number of features
	// Price: 15, Volume: 12, Volatility: 10, Momentum: 14 (williams_r is 1 feature), Trend: 12, Time: 6
	// Total expected: 15 + 12 + 10 + 14 + 12 + 6 = 69
	// Note: Actual count depends on implementation
	expectedMinFeatures := 60
	if len(values) < expectedMinFeatures {
		t.Errorf("Expected at least %d features, got %d", expectedMinFeatures, len(values))
	}

	// Values and names should have same length
	if len(values) != len(names) {
		t.Errorf("Values length (%d) != names length (%d)", len(values), len(names))
	}

	// Check that NaN values are converted to 0
	for i, v := range values {
		if math.IsNaN(v) {
			t.Errorf("Feature %s has NaN value, should be 0", names[i])
		}
	}

	// Check some specific feature names exist
	expectedNames := []string{"return_1c", "rsi_14", "atr_14", "macd_line", "sma_20", "hour_of_day"}
	for _, expected := range expectedNames {
		found := false
		for _, name := range names {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected feature name %s not found", expected)
		}
	}
}

func TestCandleFeatures_GetSummary(t *testing.T) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
	candles := createTestCandles(100)

	features, err := fc.Calculate(candles)
	if err != nil {
		t.Fatalf("Failed to calculate features: %v", err)
	}

	summary := features.GetSummary()

	if summary.TotalFeatures == 0 {
		t.Error("Total features should be > 0")
	}

	if !summary.HasPriceFeatures {
		t.Error("Should have price features")
	}
	if !summary.HasVolumeFeatures {
		t.Error("Should have volume features")
	}
	if !summary.HasVolatilityFeatures {
		t.Error("Should have volatility features")
	}
	if !summary.HasMomentumFeatures {
		t.Error("Should have momentum features")
	}
	if !summary.HasTrendFeatures {
		t.Error("Should have trend features")
	}
	if !summary.HasTimeFeatures {
		t.Error("Should have time features")
	}

	// Most features should be complete with 100 candles
	completionRate := float64(summary.CompleteFeatures) / float64(summary.TotalFeatures)
	if completionRate < 0.9 {
		t.Errorf("Expected >90%% completion rate, got %.1f%%", completionRate*100)
	}
}

func TestCandleFeatures_JSONSerialization(t *testing.T) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
	candles := createTestCandles(100)

	features, err := fc.Calculate(candles)
	if err != nil {
		t.Fatalf("Failed to calculate features: %v", err)
	}

	// Serialize to JSON
	jsonData, err := features.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize to JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("JSON data should not be empty")
	}

	// Deserialize from JSON
	restored, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("Failed to deserialize from JSON: %v", err)
	}

	// Check that key fields match
	if restored.Symbol != features.Symbol {
		t.Errorf("Symbol mismatch: %s != %s", restored.Symbol, features.Symbol)
	}
	if restored.Timeframe != features.Timeframe {
		t.Errorf("Timeframe mismatch: %s != %s", restored.Timeframe, features.Timeframe)
	}
	if !restored.OpenTime.Equal(features.OpenTime) {
		t.Errorf("OpenTime mismatch: %v != %v", restored.OpenTime, features.OpenTime)
	}
}

func TestPriceFeatures(t *testing.T) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
	candles := createTestCandles(100)

	features, err := fc.Calculate(candles)
	if err != nil {
		t.Fatalf("Failed to calculate features: %v", err)
	}

	price := features.Price

	// Check that ratios are in valid range
	if price.BodyRatio < 0 || price.BodyRatio > 1 {
		if !math.IsNaN(price.BodyRatio) {
			t.Errorf("BodyRatio should be in [0,1], got %f", price.BodyRatio)
		}
	}

	if price.UpperWickRatio < 0 || price.UpperWickRatio > 1 {
		if !math.IsNaN(price.UpperWickRatio) {
			t.Errorf("UpperWickRatio should be in [0,1], got %f", price.UpperWickRatio)
		}
	}

	if price.LowerWickRatio < 0 || price.LowerWickRatio > 1 {
		if !math.IsNaN(price.LowerWickRatio) {
			t.Errorf("LowerWickRatio should be in [0,1], got %f", price.LowerWickRatio)
		}
	}

	// Check that consecutive counts are non-negative
	if price.ConsecutiveUp < 0 || price.ConsecutiveDown < 0 {
		t.Error("Consecutive counts should be non-negative")
	}
}

func TestMomentumFeatures(t *testing.T) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
	candles := createTestCandles(100)

	features, err := fc.Calculate(candles)
	if err != nil {
		t.Fatalf("Failed to calculate features: %v", err)
	}

	momentum := features.Momentum

	// RSI should be in [0, 100]
	for _, rsi := range []float64{momentum.RSI7, momentum.RSI14, momentum.RSI21} {
		if !math.IsNaN(rsi) && (rsi < 0 || rsi > 100) {
			t.Errorf("RSI should be in [0,100], got %f", rsi)
		}
	}

	// Stochastic should be in [0, 100]
	if !math.IsNaN(momentum.StochK) && (momentum.StochK < 0 || momentum.StochK > 100) {
		t.Errorf("Stochastic K should be in [0,100], got %f", momentum.StochK)
	}

	// Williams %R should be in [-100, 0]
	if !math.IsNaN(momentum.WilliamsR) && (momentum.WilliamsR < -100 || momentum.WilliamsR > 0) {
		t.Errorf("Williams R should be in [-100,0], got %f", momentum.WilliamsR)
	}
}

func TestTimeFeatures(t *testing.T) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
	candles := createTestCandles(100)

	features, err := fc.Calculate(candles)
	if err != nil {
		t.Fatalf("Failed to calculate features: %v", err)
	}

	timeFeatures := features.Time

	// Hour should be in [0, 23]
	if timeFeatures.HourOfDay < 0 || timeFeatures.HourOfDay > 23 {
		t.Errorf("HourOfDay should be in [0,23], got %d", timeFeatures.HourOfDay)
	}

	// Day of week should be in [0, 6]
	if timeFeatures.DayOfWeek < 0 || timeFeatures.DayOfWeek > 6 {
		t.Errorf("DayOfWeek should be in [0,6], got %d", timeFeatures.DayOfWeek)
	}

	// Day of month should be in [1, 31]
	if timeFeatures.DayOfMonth < 1 || timeFeatures.DayOfMonth > 31 {
		t.Errorf("DayOfMonth should be in [1,31], got %d", timeFeatures.DayOfMonth)
	}
}

func BenchmarkFeatureCalculator_Calculate(b *testing.B) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
	candles := createTestCandles(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fc.Calculate(candles)
	}
}

func BenchmarkFeatureCalculator_CalculateBatch(b *testing.B) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
	candles := createTestCandles(200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fc.CalculateBatch(candles, 100)
	}
}

func BenchmarkCandleFeatures_ToFeatureVector(b *testing.B) {
	fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
	candles := createTestCandles(100)
	features, _ := fc.Calculate(candles)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		features.ToFeatureVector()
	}
}
