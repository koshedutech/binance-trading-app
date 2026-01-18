// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.13: Indicator Calculation Engine
package indicators

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestIndicatorCache tests the IndicatorCache functionality.
func TestIndicatorCache(t *testing.T) {
	t.Run("BasicOperations", func(t *testing.T) {
		cache := NewIndicatorCache(1 * time.Second)

		// Test Set and Get
		value := NewIndicatorValue("test", 50.0, 75.0)
		cache.Set("BTCUSDT", "test", value)

		retrieved, ok := cache.Get("BTCUSDT", "test")
		if !ok {
			t.Error("Expected to find cached value")
		}
		if retrieved.Value != 50.0 {
			t.Errorf("Expected value 50.0, got %f", retrieved.Value)
		}
		if retrieved.Normalized != 75.0 {
			t.Errorf("Expected normalized 75.0, got %f", retrieved.Normalized)
		}
	})

	t.Run("TTLExpiration", func(t *testing.T) {
		cache := NewIndicatorCache(50 * time.Millisecond)

		value := NewIndicatorValue("test", 50.0, 75.0)
		cache.Set("BTCUSDT", "test", value)

		// Should be found immediately
		_, ok := cache.Get("BTCUSDT", "test")
		if !ok {
			t.Error("Expected to find cached value before expiration")
		}

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should be expired now
		_, ok = cache.Get("BTCUSDT", "test")
		if ok {
			t.Error("Expected cache entry to be expired")
		}
	})

	t.Run("InvalidateBySymbol", func(t *testing.T) {
		cache := NewIndicatorCache(1 * time.Second)

		cache.Set("BTCUSDT", "rsi", NewIndicatorValue("rsi", 50.0, 50.0))
		cache.Set("BTCUSDT", "macd", NewIndicatorValue("macd", 10.0, 60.0))
		cache.Set("ETHUSDT", "rsi", NewIndicatorValue("rsi", 55.0, 55.0))

		cache.InvalidateBySymbol("BTCUSDT")

		// BTCUSDT entries should be gone
		_, ok := cache.Get("BTCUSDT", "rsi")
		if ok {
			t.Error("Expected BTCUSDT rsi to be invalidated")
		}
		_, ok = cache.Get("BTCUSDT", "macd")
		if ok {
			t.Error("Expected BTCUSDT macd to be invalidated")
		}

		// ETHUSDT should still exist
		_, ok = cache.Get("ETHUSDT", "rsi")
		if !ok {
			t.Error("Expected ETHUSDT rsi to still exist")
		}
	})

	t.Run("InvalidateByIndicator", func(t *testing.T) {
		cache := NewIndicatorCache(1 * time.Second)

		cache.Set("BTCUSDT", "rsi", NewIndicatorValue("rsi", 50.0, 50.0))
		cache.Set("ETHUSDT", "rsi", NewIndicatorValue("rsi", 55.0, 55.0))
		cache.Set("BTCUSDT", "macd", NewIndicatorValue("macd", 10.0, 60.0))

		cache.InvalidateByIndicator("rsi")

		// All RSI entries should be gone
		_, ok := cache.Get("BTCUSDT", "rsi")
		if ok {
			t.Error("Expected BTCUSDT rsi to be invalidated")
		}
		_, ok = cache.Get("ETHUSDT", "rsi")
		if ok {
			t.Error("Expected ETHUSDT rsi to be invalidated")
		}

		// MACD should still exist
		_, ok = cache.Get("BTCUSDT", "macd")
		if !ok {
			t.Error("Expected BTCUSDT macd to still exist")
		}
	})

	t.Run("GetMultiple", func(t *testing.T) {
		cache := NewIndicatorCache(1 * time.Second)

		cache.Set("BTCUSDT", "rsi", NewIndicatorValue("rsi", 50.0, 50.0))
		cache.Set("BTCUSDT", "macd", NewIndicatorValue("macd", 10.0, 60.0))

		values := cache.GetMultiple("BTCUSDT", []string{"rsi", "macd", "missing"})

		if len(values) != 2 {
			t.Errorf("Expected 2 values, got %d", len(values))
		}

		if _, ok := values["rsi"]; !ok {
			t.Error("Expected rsi in results")
		}
		if _, ok := values["macd"]; !ok {
			t.Error("Expected macd in results")
		}
		if _, ok := values["missing"]; ok {
			t.Error("Did not expect missing in results")
		}
	})

	t.Run("SetMultiple", func(t *testing.T) {
		cache := NewIndicatorCache(1 * time.Second)

		values := map[string]*IndicatorValue{
			"rsi":  NewIndicatorValue("rsi", 50.0, 50.0),
			"macd": NewIndicatorValue("macd", 10.0, 60.0),
		}

		cache.SetMultiple("BTCUSDT", values)

		if cache.Size() != 2 {
			t.Errorf("Expected cache size 2, got %d", cache.Size())
		}

		retrieved := cache.GetMultiple("BTCUSDT", []string{"rsi", "macd"})
		if len(retrieved) != 2 {
			t.Errorf("Expected 2 retrieved values, got %d", len(retrieved))
		}
	})

	t.Run("Clean", func(t *testing.T) {
		cache := NewIndicatorCache(50 * time.Millisecond)

		// Add some entries
		cache.Set("BTCUSDT", "rsi", NewIndicatorValue("rsi", 50.0, 50.0))
		cache.Set("BTCUSDT", "macd", NewIndicatorValue("macd", 10.0, 60.0))

		if cache.Size() != 2 {
			t.Errorf("Expected initial size 2, got %d", cache.Size())
		}

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Clean should remove expired entries
		removed := cache.Clean()
		if removed != 2 {
			t.Errorf("Expected 2 removed, got %d", removed)
		}

		if cache.Size() != 0 {
			t.Errorf("Expected size 0 after clean, got %d", cache.Size())
		}
	})

	t.Run("HitRate", func(t *testing.T) {
		cache := NewIndicatorCache(1 * time.Second)
		cache.ResetStats()

		cache.Set("BTCUSDT", "rsi", NewIndicatorValue("rsi", 50.0, 50.0))

		// 1 hit
		cache.Get("BTCUSDT", "rsi")
		// 1 miss
		cache.Get("BTCUSDT", "missing")

		hitRate := cache.HitRate()
		if hitRate != 50.0 {
			t.Errorf("Expected 50%% hit rate, got %.2f%%", hitRate)
		}
	})

	t.Run("ThreadSafety", func(t *testing.T) {
		cache := NewIndicatorCache(1 * time.Second)
		var wg sync.WaitGroup

		// Concurrent writes
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				symbol := fmt.Sprintf("SYMBOL%d", idx%10)
				indicator := fmt.Sprintf("ind%d", idx%5)
				cache.Set(symbol, indicator, NewIndicatorValue(indicator, float64(idx), float64(idx)))
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				symbol := fmt.Sprintf("SYMBOL%d", idx%10)
				indicator := fmt.Sprintf("ind%d", idx%5)
				cache.Get(symbol, indicator)
			}(i)
		}

		wg.Wait()
		// If we get here without deadlock/race, the test passes
	})

	t.Run("InvalidateForUpdate", func(t *testing.T) {
		cache := NewIndicatorCache(1 * time.Second)

		// Set up some values
		cache.Set("BTCUSDT", "vwap", NewIndicatorValue("vwap", 100.0, 50.0))
		cache.Set("BTCUSDT", "rsi", NewIndicatorValue("rsi", 50.0, 50.0))
		cache.Set("BTCUSDT", "macd", NewIndicatorValue("macd", 10.0, 60.0))

		// Price tick should only invalidate price-sensitive indicators
		cache.InvalidateForUpdate("BTCUSDT", UpdatePriceTick)

		// VWAP is price-sensitive, should be invalidated
		_, ok := cache.Get("BTCUSDT", "vwap")
		if ok {
			t.Error("Expected vwap to be invalidated on price tick")
		}

		// RSI should still exist (candle-based)
		_, ok = cache.Get("BTCUSDT", "rsi")
		if !ok {
			t.Error("Expected rsi to still exist after price tick")
		}

		// Candle close should invalidate everything
		cache.InvalidateForUpdate("BTCUSDT", UpdateCandleClose)

		_, ok = cache.Get("BTCUSDT", "rsi")
		if ok {
			t.Error("Expected rsi to be invalidated on candle close")
		}
	})
}

// TestCalculationEngine tests the CalculationEngine functionality.
func TestCalculationEngine(t *testing.T) {
	t.Run("BasicCalculation", func(t *testing.T) {
		engine := NewCalculationEngineSimple()

		data := createTestIndicatorData("BTCUSDT")

		result, err := engine.CalculateForSymbol("BTCUSDT", data)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if result.Symbol != "BTCUSDT" {
			t.Errorf("Expected symbol BTCUSDT, got %s", result.Symbol)
		}

		if len(result.SegmentResults) == 0 {
			t.Error("Expected segment results")
		}

		// Should have duration tracked
		if result.Duration == 0 {
			t.Error("Expected duration to be tracked")
		}
	})

	t.Run("CacheIntegration", func(t *testing.T) {
		engine := NewCalculationEngineSimple()

		data := createTestIndicatorData("BTCUSDT")

		// First calculation - all cache misses
		result1, _ := engine.CalculateForSymbol("BTCUSDT", data)
		initialMisses := result1.CacheMisses

		// Second calculation - should have cache hits
		result2, _ := engine.CalculateForSymbol("BTCUSDT", data)

		if result2.CacheHits == 0 {
			t.Error("Expected cache hits on second calculation")
		}

		if result2.CacheMisses >= initialMisses {
			t.Error("Expected fewer cache misses on second calculation")
		}
	})

	t.Run("IncrementalUpdate_PriceTick", func(t *testing.T) {
		engine := NewCalculationEngineSimple()

		data := createTestIndicatorData("BTCUSDT")

		// Initial calculation
		engine.CalculateForSymbol("BTCUSDT", data)

		// Price tick should only invalidate some indicators
		result, _ := engine.OnPriceTick("BTCUSDT", data)

		// Should have some cache hits (non-price-sensitive indicators)
		if result.CacheHits == 0 {
			t.Error("Expected some cache hits after price tick")
		}
	})

	t.Run("IncrementalUpdate_CandleClose", func(t *testing.T) {
		engine := NewCalculationEngineSimple()

		data := createTestIndicatorData("BTCUSDT")

		// Initial calculation
		engine.CalculateForSymbol("BTCUSDT", data)

		// Candle close should invalidate all
		result, _ := engine.OnCandleClose("BTCUSDT", data)

		// All cache misses since everything was invalidated
		if result.CacheHits != 0 {
			t.Errorf("Expected 0 cache hits after candle close, got %d", result.CacheHits)
		}
	})

	t.Run("CalculateSegment", func(t *testing.T) {
		engine := NewCalculationEngineSimple()

		data := createTestIndicatorData("BTCUSDT")

		result, err := engine.CalculateSegment("BTCUSDT", SegmentTrend, data)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if result.Segment != SegmentTrend {
			t.Errorf("Expected segment %s, got %s", SegmentTrend, result.Segment)
		}

		// Score should be in 0-100 range
		if result.Score < 0 || result.Score > 100 {
			t.Errorf("Score %f out of 0-100 range", result.Score)
		}
	})

	t.Run("SettingsUpdate", func(t *testing.T) {
		engine := NewCalculationEngineSimple()

		// Get current settings
		settings1 := engine.GetSettings()

		// Modify settings
		newSettings := DefaultIndicatorSettings()
		newSettings.Trend.SelectedIndicators = []string{"ema_cross"}

		engine.SetSettings(newSettings)

		// Get updated settings
		settings2 := engine.GetSettings()

		if len(settings2.Trend.SelectedIndicators) != 1 {
			t.Errorf("Expected 1 trend indicator, got %d", len(settings2.Trend.SelectedIndicators))
		}

		// Original should be unchanged (clone verification)
		if len(settings1.Trend.SelectedIndicators) == 1 {
			t.Error("Original settings should not be modified")
		}
	})

	t.Run("BatchCalculation", func(t *testing.T) {
		engine := NewCalculationEngineSimple()

		requests := []BatchCalculation{
			{Symbol: "BTCUSDT", Data: createTestIndicatorData("BTCUSDT")},
			{Symbol: "ETHUSDT", Data: createTestIndicatorData("ETHUSDT")},
			{Symbol: "BNBUSDT", Data: createTestIndicatorData("BNBUSDT")},
		}

		results := engine.CalculateBatch(requests)

		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}

		for _, symbol := range []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"} {
			if _, ok := results[symbol]; !ok {
				t.Errorf("Expected result for %s", symbol)
			}
		}
	})

	t.Run("EnginePerformanceStats", func(t *testing.T) {
		engine := NewCalculationEngineSimple()
		engine.ResetStats()

		data := createTestIndicatorData("BTCUSDT")

		// Perform a few calculations
		for i := 0; i < 5; i++ {
			engine.CalculateForSymbol("BTCUSDT", data)
		}

		stats := engine.GetPerformanceStats()

		if stats.TotalCalculations != 5 {
			t.Errorf("Expected 5 calculations, got %d", stats.TotalCalculations)
		}

		if stats.AverageDuration == 0 {
			t.Error("Expected non-zero average duration")
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		engine := NewCalculationEngineSimple()

		data := createTestIndicatorData("BTCUSDT")
		engine.CalculateForSymbol("BTCUSDT", data)

		health := engine.CheckHealth()

		if health["status"] == nil {
			t.Error("Expected status in health check")
		}

		if health["meets_5ms_target"] == nil {
			t.Error("Expected meets_5ms_target in health check")
		}
	})
}

// TestCalculationResult tests the CalculationResult functionality.
func TestCalculationResult(t *testing.T) {
	t.Run("GetSegmentScore", func(t *testing.T) {
		result := NewCalculationResult("BTCUSDT")

		// Non-existent segment should return neutral
		score := result.GetSegmentScore(SegmentTrend)
		if score != 50 {
			t.Errorf("Expected neutral score 50, got %f", score)
		}

		// Add a segment result
		result.SegmentResults[SegmentTrend] = &SegmentResult{
			Segment: SegmentTrend,
			Score:   75.0,
		}

		score = result.GetSegmentScore(SegmentTrend)
		if score != 75.0 {
			t.Errorf("Expected score 75, got %f", score)
		}
	})

	t.Run("GetTotalScore", func(t *testing.T) {
		result := NewCalculationResult("BTCUSDT")

		result.SegmentResults[SegmentTrend] = &SegmentResult{Score: 60.0}
		result.SegmentResults[SegmentMomentum] = &SegmentResult{Score: 70.0}
		result.SegmentResults[SegmentVolatility] = &SegmentResult{Score: 50.0}
		result.SegmentResults[SegmentVolume] = &SegmentResult{Score: 80.0}

		total := result.GetTotalScore()
		expected := 60.0 + 70.0 + 50.0 + 80.0

		if total != expected {
			t.Errorf("Expected total %f, got %f", expected, total)
		}
	})

	t.Run("ErrorTracking", func(t *testing.T) {
		result := NewCalculationResult("BTCUSDT")

		if result.HasErrors() {
			t.Error("Expected no errors initially")
		}

		result.AddError("test error 1")
		result.AddError("test error 2")

		if !result.HasErrors() {
			t.Error("Expected errors after adding")
		}

		if len(result.Errors) != 2 {
			t.Errorf("Expected 2 errors, got %d", len(result.Errors))
		}
	})
}

// BenchmarkCalculationEngine_FullSet benchmarks full indicator set calculation.
// Story 11.13 requirement: < 5ms for full indicator set.
func BenchmarkCalculationEngine_FullSet(b *testing.B) {
	engine := NewCalculationEngineSimple()
	data := createTestIndicatorData("BTCUSDT")

	// Warmup
	engine.CalculateForSymbol("BTCUSDT", data)

	// Clear cache to benchmark fresh calculations
	engine.InvalidateCacheAll()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.InvalidateCacheAll() // Ensure fresh calculation each time
		engine.CalculateForSymbol("BTCUSDT", data)
	}

	// Report performance
	stats := engine.GetPerformanceStats()
	b.ReportMetric(float64(stats.AverageDuration.Nanoseconds())/1e6, "ms/op")
}

// BenchmarkCalculationEngine_CacheHit benchmarks calculation with cache hits.
func BenchmarkCalculationEngine_CacheHit(b *testing.B) {
	engine := NewCalculationEngineSimple()
	data := createTestIndicatorData("BTCUSDT")

	// Warmup and populate cache
	engine.CalculateForSymbol("BTCUSDT", data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.CalculateForSymbol("BTCUSDT", data)
	}
}

// BenchmarkCalculationEngine_SingleSegment benchmarks single segment calculation.
func BenchmarkCalculationEngine_SingleSegment(b *testing.B) {
	engine := NewCalculationEngineSimple()
	data := createTestIndicatorData("BTCUSDT")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.CalculateSegment("BTCUSDT", SegmentTrend, data)
	}
}

// BenchmarkCalculationEngine_IncrementalPriceTick benchmarks price tick updates.
func BenchmarkCalculationEngine_IncrementalPriceTick(b *testing.B) {
	engine := NewCalculationEngineSimple()
	data := createTestIndicatorData("BTCUSDT")

	// Initial calculation
	engine.CalculateForSymbol("BTCUSDT", data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.OnPriceTick("BTCUSDT", data)
	}
}

// BenchmarkIndicatorCache_Get benchmarks cache get operations.
func BenchmarkIndicatorCache_Get(b *testing.B) {
	cache := NewIndicatorCache(1 * time.Second)
	cache.Set("BTCUSDT", "rsi", NewIndicatorValue("rsi", 50.0, 50.0))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("BTCUSDT", "rsi")
	}
}

// BenchmarkIndicatorCache_Set benchmarks cache set operations.
func BenchmarkIndicatorCache_Set(b *testing.B) {
	cache := NewIndicatorCache(1 * time.Second)
	value := NewIndicatorValue("rsi", 50.0, 50.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("BTCUSDT", "rsi", value)
	}
}

// TestPerformanceTarget tests that the engine meets the < 5ms performance target.
func TestPerformanceTarget(t *testing.T) {
	engine := NewCalculationEngineSimple()
	data := createTestIndicatorData("BTCUSDT")

	// Run multiple calculations to get average
	numRuns := 100
	var totalDuration time.Duration

	for i := 0; i < numRuns; i++ {
		engine.InvalidateCacheAll() // Fresh calculation each time
		start := time.Now()
		engine.CalculateForSymbol("BTCUSDT", data)
		totalDuration += time.Since(start)
	}

	avgDuration := totalDuration / time.Duration(numRuns)

	t.Logf("Average calculation time: %v", avgDuration)

	if avgDuration > 5*time.Millisecond {
		t.Errorf("Performance target not met: average %v > 5ms", avgDuration)
	} else {
		t.Logf("Performance target MET: average %v < 5ms", avgDuration)
	}
}

// TestCacheClose tests that the cleanup goroutine stops when Close is called.
func TestCacheClose(t *testing.T) {
	config := &CacheConfig{
		DefaultTTL:      100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
	}

	cache := NewIndicatorCacheWithConfig(config)

	// Add some data
	cache.Set("BTCUSDT", "rsi", NewIndicatorValue("rsi", 50.0, 50.0))

	// Close should stop the goroutine
	cache.Close()

	// Calling Close again should be safe
	cache.Close()

	// Cache should still be usable (though cleanup won't run)
	cache.Set("BTCUSDT", "macd", NewIndicatorValue("macd", 10.0, 60.0))
	_, ok := cache.Get("BTCUSDT", "macd")
	if !ok {
		t.Error("Cache should still work after Close")
	}
}

// TestConcurrentCalculations tests thread safety of batch calculations.
func TestConcurrentCalculations(t *testing.T) {
	engine := NewCalculationEngineSimple()

	// Run multiple concurrent calculations
	var wg sync.WaitGroup
	results := make(chan *CalculationResult, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			symbol := fmt.Sprintf("SYMBOL%d", idx%5)
			data := createTestIndicatorData(symbol)
			result, err := engine.CalculateForSymbol(symbol, data)
			if err != nil {
				t.Errorf("Calculation error for %s: %v", symbol, err)
				return
			}
			results <- result
		}(i)
	}

	wg.Wait()
	close(results)

	// Verify all results are valid
	count := 0
	for result := range results {
		count++
		if result.Symbol == "" {
			t.Error("Result has empty symbol")
		}
	}

	if count != 50 {
		t.Errorf("Expected 50 results, got %d", count)
	}

	// Verify engine stats are consistent
	stats := engine.GetPerformanceStats()
	if stats.TotalCalculations != 50 {
		t.Errorf("Expected 50 total calculations, got %d", stats.TotalCalculations)
	}
}

// createTestIndicatorData creates test data for indicator calculations.
func createTestIndicatorData(symbol string) *IndicatorData {
	data := NewIndicatorData(symbol)
	data.Price = 50000.0
	data.Timeframe = "15m"

	// Generate 100 candles of test data
	numCandles := 100
	data.Opens = make([]float64, numCandles)
	data.Highs = make([]float64, numCandles)
	data.Lows = make([]float64, numCandles)
	data.Closes = make([]float64, numCandles)
	data.Prices = make([]float64, numCandles)
	data.Volumes = make([]float64, numCandles)
	data.Timestamps = make([]int64, numCandles)

	basePrice := 50000.0
	for i := 0; i < numCandles; i++ {
		// Simulate slight price variations
		variation := float64(i%10-5) * 10.0
		price := basePrice + variation

		data.Opens[i] = price - 5
		data.Highs[i] = price + 20
		data.Lows[i] = price - 20
		data.Closes[i] = price
		data.Prices[i] = price
		data.Volumes[i] = 1000000 + float64(i*1000)
		data.Timestamps[i] = time.Now().Add(-time.Duration(numCandles-i) * 15 * time.Minute).UnixMilli()
	}

	// Set pre-calculated values that some indicators depend on
	data.SetOtherValue("adx", 25.0)
	data.SetOtherValue("atr", 100.0)
	data.SetOtherValue("atr_average", 95.0)
	data.SetOtherValue("rsi", 55.0)
	data.SetOtherValue("ema_9", 50050.0)
	data.SetOtherValue("ema_21", 49950.0)

	return data
}
