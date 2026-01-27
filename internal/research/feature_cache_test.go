// Package research provides data structures and services for research infrastructure.
// Story 15.5: Feature Cache Management - Tests
package research

import (
	"context"
	"testing"
	"time"

	"binance-trading-bot/internal/research/indicators"
)

// createTestCandleFeatures creates test CandleFeatures for testing
func createTestCandleFeatures(symbol string, timeframe Timeframe, openTime time.Time) *CandleFeatures {
	return &CandleFeatures{
		Symbol:    symbol,
		Timeframe: timeframe,
		OpenTime:  openTime,
		Price: &indicators.PriceFeatures{
			Return1c:       0.5,
			Return3c:       1.2,
			Return5c:       2.0,
			BodyRatio:      0.7,
			UpperWickRatio: 0.15,
			LowerWickRatio: 0.15,
			PositionInRange: 0.6,
			ConsecutiveUp:  3,
			ConsecutiveDown: 0,
			HigherHigh:     true,
			LowerLow:       false,
		},
		Volume: &indicators.VolumeFeatures{
			VolumeRatio5:  1.5,
			VolumeRatio10: 1.2,
			TakerBuyRatio: 0.55,
			OBV:           1000000,
		},
		Volatility: &indicators.VolatilityFeatures{
			ATR7:            50.0,
			ATR14:           48.0,
			ATRPercent:      1.2,
			BollingerWidth:  100.0,
			BollingerPosition: 0.7,
		},
		Momentum: &indicators.MomentumFeatures{
			RSI7:    65.0,
			RSI14:   58.0,
			StochK:  75.0,
			StochD:  70.0,
			MACDLine: 10.0,
			MACDSignal: 8.0,
			MACDHistogram: 2.0,
		},
		Trend: &indicators.TrendFeatures{
			SMA10:         40000.0,
			SMA20:         39500.0,
			SMA50:         38000.0,
			EMA9:          40100.0,
			EMA21:         39800.0,
			PriceVsSMA20:  1.5,
			ADX:           28.0,
			TrendStrength: 0.7,
		},
		Time: &indicators.TimeFeatures{
			HourOfDay:   14,
			DayOfWeek:   3,
			IsWeekend:   false,
			Session:     indicators.SessionAsian,
			DayOfMonth:  15,
			IsMonthEnd:  false,
		},
		CalculatedAt:   time.Now(),
		LookbackUsed:   100,
		LookbackNeeded: 60,
		IsComplete:     true,
	}
}

func TestFeaturesToMap(t *testing.T) {
	openTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	features := createTestCandleFeatures("BTCUSDT", Timeframe15m, openTime)

	m := featuresToMap(features, 1)

	// Check metadata
	if m["_version"] != "1" {
		t.Errorf("Expected version '1', got '%v'", m["_version"])
	}
	if m["symbol"] != "BTCUSDT" {
		t.Errorf("Expected symbol 'BTCUSDT', got '%v'", m["symbol"])
	}
	if m["timeframe"] != "15m" {
		t.Errorf("Expected timeframe '15m', got '%v'", m["timeframe"])
	}
	if m["is_complete"] != "1" {
		t.Errorf("Expected is_complete '1', got '%v'", m["is_complete"])
	}

	// Check that feature categories are JSON strings
	if _, ok := m["price"].(string); !ok {
		t.Error("Expected price to be a JSON string")
	}
	if _, ok := m["volume"].(string); !ok {
		t.Error("Expected volume to be a JSON string")
	}
}

func TestFeaturesFromMap(t *testing.T) {
	openTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	original := createTestCandleFeatures("BTCUSDT", Timeframe15m, openTime)

	// Convert to map and back
	m := featuresToMap(original, 1)

	// Convert map[string]interface{} to map[string]string
	stringMap := make(map[string]string)
	for k, v := range m {
		stringMap[k] = v.(string)
	}

	restored, err := featuresFromMap(stringMap)
	if err != nil {
		t.Fatalf("Failed to restore from map: %v", err)
	}

	// Verify restoration
	if restored.Symbol != original.Symbol {
		t.Errorf("Symbol mismatch: got %s, want %s", restored.Symbol, original.Symbol)
	}
	if restored.Timeframe != original.Timeframe {
		t.Errorf("Timeframe mismatch: got %s, want %s", restored.Timeframe, original.Timeframe)
	}
	if !restored.OpenTime.Equal(original.OpenTime) {
		t.Errorf("OpenTime mismatch: got %v, want %v", restored.OpenTime, original.OpenTime)
	}
	if restored.IsComplete != original.IsComplete {
		t.Errorf("IsComplete mismatch: got %v, want %v", restored.IsComplete, original.IsComplete)
	}

	// Check Price features
	if restored.Price == nil {
		t.Fatal("Price features should not be nil")
	}
	if restored.Price.Return1c != original.Price.Return1c {
		t.Errorf("Price.Return1c mismatch: got %f, want %f", restored.Price.Return1c, original.Price.Return1c)
	}
	if restored.Price.ConsecutiveUp != original.Price.ConsecutiveUp {
		t.Errorf("Price.ConsecutiveUp mismatch: got %d, want %d", restored.Price.ConsecutiveUp, original.Price.ConsecutiveUp)
	}

	// Check Momentum features
	if restored.Momentum == nil {
		t.Fatal("Momentum features should not be nil")
	}
	if restored.Momentum.RSI14 != original.Momentum.RSI14 {
		t.Errorf("Momentum.RSI14 mismatch: got %f, want %f", restored.Momentum.RSI14, original.Momentum.RSI14)
	}

	// Check Time features
	if restored.Time == nil {
		t.Fatal("Time features should not be nil")
	}
	if restored.Time.HourOfDay != original.Time.HourOfDay {
		t.Errorf("Time.HourOfDay mismatch: got %d, want %d", restored.Time.HourOfDay, original.Time.HourOfDay)
	}
}

func TestFeatureCacheConfig(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		config := DefaultFeatureCacheConfig()

		if config.Version != FeatureVersion {
			t.Errorf("Expected version %d, got %d", FeatureVersion, config.Version)
		}
		if config.DefaultTTL != 24*time.Hour {
			t.Errorf("Expected default TTL 24h, got %v", config.DefaultTTL)
		}
	})

	t.Run("TTL by timeframe", func(t *testing.T) {
		fc := NewFeatureCache(nil, DefaultFeatureCacheConfig())

		// Check default TTLs
		if ttl := fc.getTTL(Timeframe1m); ttl != 6*time.Hour {
			t.Errorf("1m TTL should be 6h, got %v", ttl)
		}
		if ttl := fc.getTTL(Timeframe15m); ttl != 24*time.Hour {
			t.Errorf("15m TTL should be 24h, got %v", ttl)
		}
		if ttl := fc.getTTL(Timeframe1d); ttl != 336*time.Hour {
			t.Errorf("1d TTL should be 336h, got %v", ttl)
		}
	})

	t.Run("TTL overrides", func(t *testing.T) {
		config := DefaultFeatureCacheConfig()
		config.TTLOverrides[Timeframe15m] = 48 * time.Hour

		fc := NewFeatureCache(nil, config)

		if ttl := fc.getTTL(Timeframe15m); ttl != 48*time.Hour {
			t.Errorf("15m TTL should be 48h (override), got %v", ttl)
		}
	})
}

func TestFeatureCacheKeyGeneration(t *testing.T) {
	config := DefaultFeatureCacheConfig()
	fc := NewFeatureCache(nil, config)

	openTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	key := fc.featureCacheKey("BTCUSDT", Timeframe15m, openTime)

	expected := "research:features:v1:BTCUSDT:15m:1705327200"
	if key != expected {
		t.Errorf("Key mismatch: got '%s', want '%s'", key, expected)
	}
}

func TestFeatureCacheWithNilCache(t *testing.T) {
	fc := NewFeatureCache(nil, DefaultFeatureCacheConfig())
	ctx := context.Background()

	// Operations should gracefully handle nil cache
	openTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	features := createTestCandleFeatures("BTCUSDT", Timeframe15m, openTime)

	// Set should not panic
	err := fc.Set(ctx, features)
	if err != nil {
		t.Errorf("Set with nil cache should return nil, got %v", err)
	}

	// Get should return nil, nil
	result, err := fc.Get(ctx, "BTCUSDT", Timeframe15m, openTime)
	if err != nil || result != nil {
		t.Errorf("Get with nil cache should return nil, nil - got %v, %v", result, err)
	}

	// GetBatch should return nil, nil
	start := openTime.Add(-1 * time.Hour)
	end := openTime.Add(1 * time.Hour)
	results, err := fc.GetBatch(ctx, "BTCUSDT", Timeframe15m, start, end)
	if err != nil || results != nil {
		t.Errorf("GetBatch with nil cache should return nil, nil - got %v, %v", results, err)
	}
}

func TestFeatureCacheHealthCheck(t *testing.T) {
	t.Run("nil cache service", func(t *testing.T) {
		fc := NewFeatureCache(nil, DefaultFeatureCacheConfig())
		if fc.IsHealthy() {
			t.Error("IsHealthy should return false for nil cache service")
		}
	})
}

func TestDefaultFeatureTTLs(t *testing.T) {
	// Verify all timeframes have default TTLs
	expectedTTLs := map[Timeframe]time.Duration{
		Timeframe1m:  6 * time.Hour,
		Timeframe5m:  12 * time.Hour,
		Timeframe15m: 24 * time.Hour,
		Timeframe30m: 48 * time.Hour,
		Timeframe1h:  72 * time.Hour,
		Timeframe4h:  168 * time.Hour,
		Timeframe1d:  336 * time.Hour,
	}

	for tf, expectedTTL := range expectedTTLs {
		if ttl, ok := DefaultFeatureTTLs[tf]; !ok || ttl != expectedTTL {
			t.Errorf("Timeframe %s: expected TTL %v, got %v", tf, expectedTTL, ttl)
		}
	}
}

func TestFeatureVersionConstant(t *testing.T) {
	if FeatureVersion < 1 {
		t.Error("FeatureVersion should be at least 1")
	}
}

func TestBoolToStr(t *testing.T) {
	if boolToStr(true) != "1" {
		t.Error("boolToStr(true) should return '1'")
	}
	if boolToStr(false) != "0" {
		t.Error("boolToStr(false) should return '0'")
	}
}

func TestSplitCacheKey(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantNil   bool
		version   int
		symbol    string
		timeframe string
		timestamp int64
	}{
		{
			name:      "valid key",
			key:       "research:features:v1:BTCUSDT:15m:1705327200",
			wantNil:   false,
			version:   1,
			symbol:    "BTCUSDT",
			timeframe: "15m",
			timestamp: 1705327200,
		},
		{
			name:      "valid key with double-digit version",
			key:       "research:features:v10:ETHUSDT:1h:1705327200",
			wantNil:   false,
			version:   10,
			symbol:    "ETHUSDT",
			timeframe: "1h",
			timestamp: 1705327200,
		},
		{
			name:      "long symbol name",
			key:       "research:features:v1:VERYLONGSYMBOLUSDT:4h:1705327200",
			wantNil:   false,
			version:   1,
			symbol:    "VERYLONGSYMBOLUSDT",
			timeframe: "4h",
			timestamp: 1705327200,
		},
		{
			name:    "empty string",
			key:     "",
			wantNil: true,
		},
		{
			name:    "wrong prefix",
			key:     "other:prefix:v1:BTCUSDT:15m:1705327200",
			wantNil: true,
		},
		{
			name:    "missing version",
			key:     "research:features:v:BTCUSDT:15m:1705327200",
			wantNil: true,
		},
		{
			name:    "invalid version",
			key:     "research:features:vABC:BTCUSDT:15m:1705327200",
			wantNil: true,
		},
		{
			name:    "missing parts",
			key:     "research:features:v1:BTCUSDT",
			wantNil: true,
		},
		{
			name:    "invalid timestamp",
			key:     "research:features:v1:BTCUSDT:15m:invalid",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitCacheKey(tt.key)

			if tt.wantNil {
				if result != nil {
					t.Errorf("splitCacheKey(%q) = %+v, want nil", tt.key, result)
				}
				return
			}

			if result == nil {
				t.Fatalf("splitCacheKey(%q) = nil, want non-nil", tt.key)
			}

			if result.version != tt.version {
				t.Errorf("version = %d, want %d", result.version, tt.version)
			}
			if result.symbol != tt.symbol {
				t.Errorf("symbol = %s, want %s", result.symbol, tt.symbol)
			}
			if result.timeframe != tt.timeframe {
				t.Errorf("timeframe = %s, want %s", result.timeframe, tt.timeframe)
			}
			if result.timestamp != tt.timestamp {
				t.Errorf("timestamp = %d, want %d", result.timestamp, tt.timestamp)
			}
		})
	}
}

func TestFeatureCalculatorWithFeatureCache(t *testing.T) {
	t.Run("NewFeatureCalculator creates FeatureCache when cache service provided", func(t *testing.T) {
		// With nil cache service
		fc := NewFeatureCalculator(nil, DefaultFeatureCalculatorConfig())
		if fc.GetFeatureCache() != nil {
			t.Error("FeatureCache should be nil when cache service is nil")
		}
	})

	t.Run("NewFeatureCalculatorWithCache", func(t *testing.T) {
		featureCache := NewFeatureCache(nil, DefaultFeatureCacheConfig())
		fc := NewFeatureCalculatorWithCache(featureCache)

		if fc.GetFeatureCache() != featureCache {
			t.Error("GetFeatureCache should return the injected FeatureCache")
		}
	})
}

// BenchmarkFeaturesToMap measures serialization performance
func BenchmarkFeaturesToMap(b *testing.B) {
	openTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	features := createTestCandleFeatures("BTCUSDT", Timeframe15m, openTime)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		featuresToMap(features, 1)
	}
}

// BenchmarkFeaturesFromMap measures deserialization performance
func BenchmarkFeaturesFromMap(b *testing.B) {
	openTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	features := createTestCandleFeatures("BTCUSDT", Timeframe15m, openTime)

	m := featuresToMap(features, 1)
	stringMap := make(map[string]string)
	for k, v := range m {
		stringMap[k] = v.(string)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		featuresFromMap(stringMap)
	}
}
