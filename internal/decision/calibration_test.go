// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.17: Calibration Layer Tests
package decision

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupTestRedis creates a mini-redis instance for testing
func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, client
}

// TestGetBucketForScore tests score to bucket mapping
func TestGetBucketForScore(t *testing.T) {
	testCases := []struct {
		score    int
		expected string
	}{
		// Boundary tests
		{-10, BucketLow},
		{0, BucketLow},
		{24, BucketLow},
		{25, BucketMediumLow},
		{49, BucketMediumLow},
		{50, BucketMediumHigh},
		{74, BucketMediumHigh},
		{75, BucketHigh},
		{100, BucketHigh},
		{150, BucketHigh}, // Over 100 should clamp to high
		// Mid-range tests
		{10, BucketLow},
		{35, BucketMediumLow},
		{62, BucketMediumHigh},
		{88, BucketHigh},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("score_%d", tc.score), func(t *testing.T) {
			result := GetBucketForScore(tc.score)
			if result != tc.expected {
				t.Errorf("GetBucketForScore(%d) = %s, expected %s", tc.score, result, tc.expected)
			}
		})
	}
}

// TestCalculateIndicatorHash tests indicator hash generation
func TestCalculateIndicatorHash(t *testing.T) {
	testCases := []struct {
		name       string
		indicators []string
		expectLen  int
		expectSame bool // Compare with another set that should produce same hash
		sameAs     []string
	}{
		{
			name:       "empty indicators",
			indicators: []string{},
			expectLen:  8,
		},
		{
			name:       "single indicator",
			indicators: []string{"RSI"},
			expectLen:  8,
		},
		{
			name:       "multiple indicators",
			indicators: []string{"RSI", "MACD", "EMA"},
			expectLen:  8,
		},
		{
			name:       "same indicators different order should match",
			indicators: []string{"MACD", "EMA", "RSI"},
			expectLen:  8,
			expectSame: true,
			sameAs:     []string{"RSI", "MACD", "EMA"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash := CalculateIndicatorHash(tc.indicators)

			if len(hash) != tc.expectLen {
				t.Errorf("expected hash length %d, got %d (%s)", tc.expectLen, len(hash), hash)
			}

			if tc.expectSame {
				sameHash := CalculateIndicatorHash(tc.sameAs)
				if hash != sameHash {
					t.Errorf("expected same hash for sorted indicators, got %s vs %s", hash, sameHash)
				}
			}
		})
	}

	// Test deterministic hashing
	t.Run("deterministic", func(t *testing.T) {
		indicators := []string{"RSI", "MACD", "ADX"}
		hash1 := CalculateIndicatorHash(indicators)
		hash2 := CalculateIndicatorHash(indicators)
		if hash1 != hash2 {
			t.Errorf("hash should be deterministic: %s vs %s", hash1, hash2)
		}
	})
}

// TestScoreBucketRecordOutcome tests bucket outcome recording
func TestScoreBucketRecordOutcome(t *testing.T) {
	bucket := &ScoreBucket{
		MinScore: 50,
		MaxScore: 74,
	}

	// Record a winning trade
	bucket.RecordOutcome(true, 5.5)

	if bucket.TotalTrades != 1 {
		t.Errorf("expected 1 total trade, got %d", bucket.TotalTrades)
	}
	if bucket.WinningTrades != 1 {
		t.Errorf("expected 1 winning trade, got %d", bucket.WinningTrades)
	}
	if bucket.WinRate != 1.0 {
		t.Errorf("expected win rate 1.0, got %f", bucket.WinRate)
	}
	if bucket.AvgPnLPercent != 5.5 {
		t.Errorf("expected avg pnl 5.5, got %f", bucket.AvgPnLPercent)
	}

	// Record a losing trade
	bucket.RecordOutcome(false, -2.0)

	if bucket.TotalTrades != 2 {
		t.Errorf("expected 2 total trades, got %d", bucket.TotalTrades)
	}
	if bucket.WinningTrades != 1 {
		t.Errorf("expected 1 winning trade, got %d", bucket.WinningTrades)
	}
	if bucket.WinRate != 0.5 {
		t.Errorf("expected win rate 0.5, got %f", bucket.WinRate)
	}
	expectedAvgPnL := 3.5 / 2 // (5.5 - 2.0) / 2 = 1.75
	if bucket.AvgPnLPercent != expectedAvgPnL {
		t.Errorf("expected avg pnl %f, got %f", expectedAvgPnL, bucket.AvgPnLPercent)
	}

	// Ensure timestamp is set
	if bucket.LastUpdated.IsZero() {
		t.Error("LastUpdated should not be zero after recording outcome")
	}
}

// TestCalibrationDataRecordOutcome tests CalibrationData outcome recording
func TestCalibrationDataRecordOutcome(t *testing.T) {
	data := NewCalibrationData(1, "trend_following", "abc12345")

	// Verify default buckets exist
	if len(data.Buckets) != 4 {
		t.Errorf("expected 4 default buckets, got %d", len(data.Buckets))
	}

	// Record outcomes in different buckets
	data.RecordOutcome(10, true, 3.0)   // Bucket 0-24
	data.RecordOutcome(30, false, -1.0) // Bucket 25-49
	data.RecordOutcome(60, true, 2.0)   // Bucket 50-74
	data.RecordOutcome(80, true, 4.0)   // Bucket 75-100

	if data.TotalTrades != 4 {
		t.Errorf("expected 4 total trades, got %d", data.TotalTrades)
	}

	// Check bucket 0-24
	if bucket := data.Buckets[BucketLow]; bucket.TotalTrades != 1 || bucket.WinningTrades != 1 {
		t.Errorf("bucket 0-24: expected 1/1, got %d/%d", bucket.TotalTrades, bucket.WinningTrades)
	}

	// Check bucket 25-49
	if bucket := data.Buckets[BucketMediumLow]; bucket.TotalTrades != 1 || bucket.WinningTrades != 0 {
		t.Errorf("bucket 25-49: expected 1/0, got %d/%d", bucket.TotalTrades, bucket.WinningTrades)
	}

	// Check bucket 50-74
	if bucket := data.Buckets[BucketMediumHigh]; bucket.TotalTrades != 1 || bucket.WinningTrades != 1 {
		t.Errorf("bucket 50-74: expected 1/1, got %d/%d", bucket.TotalTrades, bucket.WinningTrades)
	}

	// Check bucket 75-100
	if bucket := data.Buckets[BucketHigh]; bucket.TotalTrades != 1 || bucket.WinningTrades != 1 {
		t.Errorf("bucket 75-100: expected 1/1, got %d/%d", bucket.TotalTrades, bucket.WinningTrades)
	}
}

// TestCalibrationDataGetCalibratedConfidence tests confidence retrieval
func TestCalibrationDataGetCalibratedConfidence(t *testing.T) {
	data := NewCalibrationData(1, "trend_following", "abc12345")

	// No trades - should return 0
	conf := data.GetCalibratedConfidence(50)
	if conf != 0 {
		t.Errorf("expected 0 confidence with no trades, got %f", conf)
	}

	// Add some trades
	data.RecordOutcome(60, true, 2.0)
	data.RecordOutcome(65, true, 1.5)
	data.RecordOutcome(70, false, -1.0)

	// Win rate should be 2/3 = 0.6667
	conf = data.GetCalibratedConfidence(60)
	expected := 2.0 / 3.0
	if !almostEqual(conf, expected, 0.001) {
		t.Errorf("expected confidence %f, got %f", expected, conf)
	}

	// Check a bucket with no trades
	conf = data.GetCalibratedConfidence(10) // Bucket 0-24 has no trades
	if conf != 0 {
		t.Errorf("expected 0 confidence for empty bucket, got %f", conf)
	}
}

// TestCalibrationDataIsReliable tests reliability checking
func TestCalibrationDataIsReliable(t *testing.T) {
	data := NewCalibrationData(1, "trend_following", "abc12345")

	// No trades - not reliable
	if data.IsReliable(50) {
		t.Error("expected not reliable with 0 trades")
	}

	// Add 49 trades - still not reliable
	for i := 0; i < 49; i++ {
		data.RecordOutcome(60, true, 1.0)
	}
	if data.IsReliable(50) {
		t.Error("expected not reliable with 49 trades")
	}

	// Add 1 more trade - now reliable
	data.RecordOutcome(60, true, 1.0)
	if !data.IsReliable(50) {
		t.Error("expected reliable with 50 trades")
	}

	// Custom minimum
	if data.IsReliable(100) {
		t.Error("expected not reliable with minTrades=100")
	}
}

// TestCalibrationServiceRecordTradeOutcome tests service-level outcome recording
func TestCalibrationServiceRecordTradeOutcome(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	if svc == nil {
		t.Fatal("failed to create calibration service")
	}

	ctx := context.Background()

	// Record an outcome
	err := svc.RecordTradeOutcome(ctx, 1, "trend_following", "abc12345", 67, true, 2.5)
	if err != nil {
		t.Fatalf("failed to record trade outcome: %v", err)
	}

	// Verify data was stored
	data, err := svc.GetCalibrationData(ctx, 1, "trend_following", "abc12345")
	if err != nil {
		t.Fatalf("failed to get calibration data: %v", err)
	}
	if data == nil {
		t.Fatal("expected calibration data to be stored")
	}
	if data.TotalTrades != 1 {
		t.Errorf("expected 1 total trade, got %d", data.TotalTrades)
	}

	// Record more outcomes
	err = svc.RecordTradeOutcome(ctx, 1, "trend_following", "abc12345", 72, false, -1.5)
	if err != nil {
		t.Fatalf("failed to record second outcome: %v", err)
	}

	data, _ = svc.GetCalibrationData(ctx, 1, "trend_following", "abc12345")
	if data.TotalTrades != 2 {
		t.Errorf("expected 2 total trades, got %d", data.TotalTrades)
	}
}

// TestCalibrationServiceGetCalibratedConfidence tests confidence retrieval
func TestCalibrationServiceGetCalibratedConfidence(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	ctx := context.Background()

	// No data - should fallback to raw score
	conf, err := svc.GetCalibratedConfidence(ctx, 1, "trend_following", "abc12345", 72)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conf != 0.72 {
		t.Errorf("expected fallback confidence 0.72, got %f", conf)
	}

	// Add data but not enough for reliability
	for i := 0; i < 30; i++ {
		svc.RecordTradeOutcome(ctx, 1, "trend_following", "abc12345", 60, true, 1.0)
	}

	// Still should fallback because not enough trades
	conf, _ = svc.GetCalibratedConfidence(ctx, 1, "trend_following", "abc12345", 60)
	if conf != 0.60 {
		t.Errorf("expected fallback confidence 0.60 with insufficient data, got %f", conf)
	}

	// Add more trades to make reliable
	for i := 0; i < 25; i++ {
		svc.RecordTradeOutcome(ctx, 1, "trend_following", "abc12345", 60, true, 1.0)
	}
	for i := 0; i < 20; i++ {
		svc.RecordTradeOutcome(ctx, 1, "trend_following", "abc12345", 60, false, -0.5)
	}

	// Now should return calibrated confidence (55/75 = 0.733...)
	conf, _ = svc.GetCalibratedConfidence(ctx, 1, "trend_following", "abc12345", 60)
	expected := 55.0 / 75.0
	if !almostEqual(conf, expected, 0.01) {
		t.Errorf("expected calibrated confidence %f, got %f", expected, conf)
	}
}

// TestCalibrationServiceRedisStorage tests Redis persistence
func TestCalibrationServiceRedisStorage(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	ctx := context.Background()

	// Record outcome
	err := svc.RecordTradeOutcome(ctx, 1, "trend_following", "abc12345", 80, true, 3.0)
	if err != nil {
		t.Fatalf("failed to record outcome: %v", err)
	}

	// Clear memory cache
	svc.ClearMemoryCache()

	// Load from Redis
	err = svc.LoadFromRedis(ctx, 1, "trend_following", "abc12345")
	if err != nil {
		t.Fatalf("failed to load from Redis: %v", err)
	}

	// Verify data is available
	data, err := svc.GetCalibrationData(ctx, 1, "trend_following", "abc12345")
	if err != nil {
		t.Fatalf("failed to get calibration data: %v", err)
	}
	if data == nil {
		t.Fatal("expected calibration data to be loaded from Redis")
	}
	if data.TotalTrades != 1 {
		t.Errorf("expected 1 trade after Redis reload, got %d", data.TotalTrades)
	}
}

// TestCalibrationServiceResetCalibration tests calibration reset
func TestCalibrationServiceResetCalibration(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	ctx := context.Background()

	// Record some outcomes
	for i := 0; i < 10; i++ {
		svc.RecordTradeOutcome(ctx, 1, "trend_following", "abc12345", 60, true, 1.0)
	}

	// Verify data exists
	data, _ := svc.GetCalibrationData(ctx, 1, "trend_following", "abc12345")
	if data == nil || data.TotalTrades != 10 {
		t.Fatal("expected 10 trades before reset")
	}

	// Reset
	err := svc.ResetCalibration(ctx, 1, "trend_following", "abc12345")
	if err != nil {
		t.Fatalf("failed to reset calibration: %v", err)
	}

	// Verify data is gone from memory
	data, _ = svc.GetCalibrationData(ctx, 1, "trend_following", "abc12345")
	if data != nil {
		t.Error("expected calibration data to be nil after reset")
	}

	// Verify data is gone from Redis
	svc.ClearMemoryCache()
	err = svc.LoadFromRedis(ctx, 1, "trend_following", "abc12345")
	if err != nil {
		t.Logf("expected no error or nil data: %v", err)
	}
	data, _ = svc.GetCalibrationData(ctx, 1, "trend_following", "abc12345")
	if data != nil {
		t.Error("expected calibration data to be nil after Redis reset")
	}
}

// TestCalibrationServiceInputValidation tests input validation
func TestCalibrationServiceInputValidation(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	ctx := context.Background()

	// Invalid userID
	err := svc.RecordTradeOutcome(ctx, 0, "trend_following", "abc12345", 50, true, 1.0)
	if err == nil {
		t.Error("expected error for invalid userID")
	}

	// Empty strategy
	err = svc.RecordTradeOutcome(ctx, 1, "", "abc12345", 50, true, 1.0)
	if err == nil {
		t.Error("expected error for empty strategy")
	}

	// Empty indicatorHash should default to "00000000"
	err = svc.RecordTradeOutcome(ctx, 1, "trend_following", "", 50, true, 1.0)
	if err != nil {
		t.Errorf("expected empty indicatorHash to be accepted: %v", err)
	}
}

// TestCalibrationServiceNilClient tests nil client handling
func TestCalibrationServiceNilClient(t *testing.T) {
	svc := NewCalibrationService(nil)
	if svc != nil {
		t.Error("expected nil service when client is nil")
	}
}

// TestCalibrationServiceInputValidationAllMethods tests input validation for all service methods
func TestCalibrationServiceInputValidationAllMethods(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	ctx := context.Background()

	// Test GetCalibratedConfidence with invalid inputs
	t.Run("GetCalibratedConfidence_invalid_userID", func(t *testing.T) {
		conf, err := svc.GetCalibratedConfidence(ctx, -1, "strategy", "hash", 50)
		if err == nil {
			t.Error("expected error for invalid userID")
		}
		// Should still return fallback confidence
		if conf != 0.50 {
			t.Errorf("expected fallback confidence 0.50, got %f", conf)
		}
	})

	t.Run("GetCalibratedConfidence_empty_strategy", func(t *testing.T) {
		conf, err := svc.GetCalibratedConfidence(ctx, 1, "", "hash", 50)
		if err == nil {
			t.Error("expected error for empty strategy")
		}
		if conf != 0.50 {
			t.Errorf("expected fallback confidence 0.50, got %f", conf)
		}
	})

	// Test GetCalibrationData with invalid inputs
	t.Run("GetCalibrationData_invalid_userID", func(t *testing.T) {
		_, err := svc.GetCalibrationData(ctx, 0, "strategy", "hash")
		if err == nil {
			t.Error("expected error for invalid userID")
		}
	})

	t.Run("GetCalibrationData_empty_strategy", func(t *testing.T) {
		_, err := svc.GetCalibrationData(ctx, 1, "", "hash")
		if err == nil {
			t.Error("expected error for empty strategy")
		}
	})

	// Test ResetCalibration with invalid inputs
	t.Run("ResetCalibration_invalid_userID", func(t *testing.T) {
		err := svc.ResetCalibration(ctx, -5, "strategy", "hash")
		if err == nil {
			t.Error("expected error for invalid userID")
		}
	})

	t.Run("ResetCalibration_empty_strategy", func(t *testing.T) {
		err := svc.ResetCalibration(ctx, 1, "", "hash")
		if err == nil {
			t.Error("expected error for empty strategy")
		}
	})
}

// TestCalibrationServiceGetStats tests statistics retrieval
func TestCalibrationServiceGetStats(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	ctx := context.Background()

	// Initial stats
	stats := svc.GetStats()
	if stats.CachedCalibrations != 0 || stats.TotalTradesTracked != 0 {
		t.Errorf("expected empty stats, got %+v", stats)
	}

	// Add some data
	svc.RecordTradeOutcome(ctx, 1, "strategy1", "hash1", 50, true, 1.0)
	svc.RecordTradeOutcome(ctx, 1, "strategy1", "hash1", 60, false, -0.5)
	svc.RecordTradeOutcome(ctx, 1, "strategy2", "hash2", 70, true, 2.0)

	stats = svc.GetStats()
	if stats.CachedCalibrations != 2 {
		t.Errorf("expected 2 cached calibrations, got %d", stats.CachedCalibrations)
	}
	if stats.TotalTradesTracked != 3 {
		t.Errorf("expected 3 total trades tracked, got %d", stats.TotalTradesTracked)
	}
}

// TestCalibrationServiceGetAllUserCalibrations tests user calibration retrieval
func TestCalibrationServiceGetAllUserCalibrations(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	ctx := context.Background()

	// Add data for user 1
	svc.RecordTradeOutcome(ctx, 1, "strategy1", "hash1", 50, true, 1.0)
	svc.RecordTradeOutcome(ctx, 1, "strategy2", "hash2", 60, false, -0.5)

	// Add data for user 2
	svc.RecordTradeOutcome(ctx, 2, "strategy1", "hash1", 70, true, 2.0)

	// Get user 1's calibrations
	user1Cals := svc.GetAllUserCalibrations(1)
	if len(user1Cals) != 2 {
		t.Errorf("expected 2 calibrations for user 1, got %d", len(user1Cals))
	}

	// Get user 2's calibrations
	user2Cals := svc.GetAllUserCalibrations(2)
	if len(user2Cals) != 1 {
		t.Errorf("expected 1 calibration for user 2, got %d", len(user2Cals))
	}
}

// TestCalibrationDataToResponse tests API response conversion
func TestCalibrationDataToResponse(t *testing.T) {
	data := NewCalibrationData(1, "trend_following", "abc12345")

	// Add some trades
	data.RecordOutcome(60, true, 2.0)
	data.RecordOutcome(65, false, -1.0)
	data.RecordOutcome(80, true, 3.0)

	resp := data.ToResponse(50)

	if resp.UserID != 1 {
		t.Errorf("expected userID 1, got %d", resp.UserID)
	}
	if resp.Strategy != "trend_following" {
		t.Errorf("expected strategy trend_following, got %s", resp.Strategy)
	}
	if resp.TotalTrades != 3 {
		t.Errorf("expected 3 total trades, got %d", resp.TotalTrades)
	}
	if resp.IsReliable {
		t.Error("expected not reliable with only 3 trades")
	}
	if len(resp.Buckets) != 4 {
		t.Errorf("expected 4 buckets in response, got %d", len(resp.Buckets))
	}

	// Check bucket 50-74
	bucket, ok := resp.Buckets[BucketMediumHigh]
	if !ok {
		t.Fatal("expected bucket 50-74 in response")
	}
	if bucket.TotalTrades != 2 {
		t.Errorf("expected 2 trades in bucket 50-74, got %d", bucket.TotalTrades)
	}
	if bucket.WinningTrades != 1 {
		t.Errorf("expected 1 winning trade in bucket 50-74, got %d", bucket.WinningTrades)
	}
}

// TestGetDefaultBuckets tests default bucket initialization
func TestGetDefaultBuckets(t *testing.T) {
	buckets := GetDefaultBuckets()

	if len(buckets) != 4 {
		t.Errorf("expected 4 buckets, got %d", len(buckets))
	}

	// Verify each bucket
	expectedBuckets := map[string][2]int{
		BucketLow:        {0, 24},
		BucketMediumLow:  {25, 49},
		BucketMediumHigh: {50, 74},
		BucketHigh:       {75, 100},
	}

	for name, bounds := range expectedBuckets {
		bucket, ok := buckets[name]
		if !ok {
			t.Errorf("missing bucket %s", name)
			continue
		}
		if bucket.MinScore != bounds[0] || bucket.MaxScore != bounds[1] {
			t.Errorf("bucket %s: expected bounds [%d, %d], got [%d, %d]",
				name, bounds[0], bounds[1], bucket.MinScore, bucket.MaxScore)
		}
		if bucket.TotalTrades != 0 || bucket.WinningTrades != 0 {
			t.Errorf("bucket %s: expected zero trades, got %d/%d",
				name, bucket.TotalTrades, bucket.WinningTrades)
		}
	}
}

// TestCalibrationServiceConcurrency tests concurrent access
func TestCalibrationServiceConcurrency(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	ctx := context.Background()

	// Run concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			for j := 0; j < 100; j++ {
				score := (idx*100 + j) % 100
				won := j%2 == 0
				svc.RecordTradeOutcome(ctx, 1, "concurrent_test", "hash1", score, won, 1.0)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify data consistency
	data, err := svc.GetCalibrationData(ctx, 1, "concurrent_test", "hash1")
	if err != nil {
		t.Fatalf("failed to get calibration data: %v", err)
	}
	if data == nil {
		t.Fatal("expected calibration data to exist")
	}

	// Should have 1000 total trades
	if data.TotalTrades != 1000 {
		t.Errorf("expected 1000 trades, got %d", data.TotalTrades)
	}

	// Each bucket should have trades
	for name, bucket := range data.Buckets {
		if bucket.TotalTrades == 0 {
			t.Errorf("bucket %s: expected trades but got 0", name)
		}
	}
}

// TestCalibrationServiceWithCustomTTL tests custom TTL setting
func TestCalibrationServiceWithCustomTTL(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	customTTL := 1 * time.Hour
	svc := NewCalibrationServiceWithTTL(client, customTTL)
	if svc == nil {
		t.Fatal("failed to create calibration service with custom TTL")
	}
	if svc.ttl != customTTL {
		t.Errorf("expected TTL %v, got %v", customTTL, svc.ttl)
	}

	// Test with zero TTL (should use default)
	svc = NewCalibrationServiceWithTTL(client, 0)
	if svc == nil || svc.ttl != DefaultCalibrationTTL {
		t.Error("expected default TTL when zero is provided")
	}
}

// TestIsCalibrationReliable tests the standalone reliability check
func TestIsCalibrationReliable(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)

	// Nil data
	if svc.IsCalibrationReliable(nil, 50) {
		t.Error("expected false for nil data")
	}

	// Data with trades
	data := NewCalibrationData(1, "test", "hash")
	for i := 0; i < 60; i++ {
		data.RecordOutcome(50, true, 1.0)
	}

	if !svc.IsCalibrationReliable(data, 50) {
		t.Error("expected true for data with 60 trades")
	}

	if svc.IsCalibrationReliable(data, 100) {
		t.Error("expected false for data with 60 trades when minTrades=100")
	}

	// Default minTrades (50)
	if !svc.IsCalibrationReliable(data, 0) {
		t.Error("expected true with default minTrades")
	}
}

// TestCalibrationDataCopy tests that Copy creates an isolated deep copy
func TestCalibrationDataCopy(t *testing.T) {
	original := &CalibrationData{
		UserID:         1,
		Strategy:       "test",
		IndicatorHash:  "abc12345",
		TotalTrades:    100,
		FirstTradeDate: time.Now().Add(-24 * time.Hour),
		LastUpdated:    time.Now(),
		Buckets: map[string]*ScoreBucket{
			"0-24": {MinScore: 0, MaxScore: 24, TotalTrades: 10, WinningTrades: 3, WinRate: 0.3, AvgPnLPercent: -0.5},
		},
	}

	copied := original.Copy()

	// Verify values are copied correctly
	if copied.UserID != original.UserID {
		t.Errorf("expected UserID %d, got %d", original.UserID, copied.UserID)
	}
	if copied.Strategy != original.Strategy {
		t.Errorf("expected Strategy %s, got %s", original.Strategy, copied.Strategy)
	}
	if copied.IndicatorHash != original.IndicatorHash {
		t.Errorf("expected IndicatorHash %s, got %s", original.IndicatorHash, copied.IndicatorHash)
	}
	if copied.TotalTrades != original.TotalTrades {
		t.Errorf("expected TotalTrades %d, got %d", original.TotalTrades, copied.TotalTrades)
	}

	// Verify bucket was copied
	if _, exists := copied.Buckets["0-24"]; !exists {
		t.Fatal("expected bucket 0-24 to exist in copy")
	}
	if copied.Buckets["0-24"].TotalTrades != 10 {
		t.Errorf("expected bucket TotalTrades 10, got %d", copied.Buckets["0-24"].TotalTrades)
	}

	// Modify copy and ensure original is unchanged
	copied.TotalTrades = 200
	copied.Strategy = "modified"
	copied.Buckets["0-24"].TotalTrades = 50
	copied.Buckets["0-24"].WinRate = 0.9

	// Verify original remains unchanged
	if original.TotalTrades != 100 {
		t.Errorf("original TotalTrades should still be 100, got %d", original.TotalTrades)
	}
	if original.Strategy != "test" {
		t.Errorf("original Strategy should still be test, got %s", original.Strategy)
	}
	if original.Buckets["0-24"].TotalTrades != 10 {
		t.Errorf("original bucket TotalTrades should still be 10, got %d", original.Buckets["0-24"].TotalTrades)
	}
	if original.Buckets["0-24"].WinRate != 0.3 {
		t.Errorf("original bucket WinRate should still be 0.3, got %f", original.Buckets["0-24"].WinRate)
	}
}

// TestCalibrationDataCopyNil tests that Copy handles nil correctly
func TestCalibrationDataCopyNil(t *testing.T) {
	var data *CalibrationData
	copied := data.Copy()
	if copied != nil {
		t.Error("expected Copy of nil to return nil")
	}
}

// TestGetCalibrationDataReturnsCopy tests that GetCalibrationData returns isolated copies
func TestGetCalibrationDataReturnsCopy(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	svc := NewCalibrationService(client)
	ctx := context.Background()

	// Record some outcomes to create calibration data
	for i := 0; i < 10; i++ {
		svc.RecordTradeOutcome(ctx, 1, "test_strategy", "hash123", 60, true, 1.5)
	}

	// Get calibration data twice
	data1, err := svc.GetCalibrationData(ctx, 1, "test_strategy", "hash123")
	if err != nil {
		t.Fatalf("failed to get calibration data: %v", err)
	}

	data2, err := svc.GetCalibrationData(ctx, 1, "test_strategy", "hash123")
	if err != nil {
		t.Fatalf("failed to get calibration data: %v", err)
	}

	// Verify initial values
	if data1.TotalTrades != 10 {
		t.Fatalf("expected 10 trades, got %d", data1.TotalTrades)
	}

	// Modify data1 (this should NOT affect cached data or data2)
	data1.TotalTrades = 999
	data1.Buckets[BucketMediumHigh].TotalTrades = 888

	// data2 should be unaffected
	if data2.TotalTrades != 10 {
		t.Errorf("data2 TotalTrades should be 10, got %d (copy isolation failed)", data2.TotalTrades)
	}
	if data2.Buckets[BucketMediumHigh].TotalTrades == 888 {
		t.Error("data2 bucket was modified when data1 was changed (copy isolation failed)")
	}

	// Get fresh data from cache - should still be original value
	data3, _ := svc.GetCalibrationData(ctx, 1, "test_strategy", "hash123")
	if data3.TotalTrades != 10 {
		t.Errorf("cached data was mutated, expected 10 trades, got %d", data3.TotalTrades)
	}
}

// almostEqual compares two floats with tolerance
func almostEqual(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}
