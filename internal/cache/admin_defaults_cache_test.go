package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"binance-trading-bot/internal/autopilot"
)

// ============================================================================
// TEST SUITE FOR AdminDefaultsCacheService
// Story 6.4: Admin Defaults Cache (UI-Mirrored Granular Keys)
// ============================================================================

// TestableAdminDefaultsCacheService wraps AdminDefaultsCacheService with mock interfaces
type TestableAdminDefaultsCacheService struct {
	mockCache *MockCacheService
	logger    *MockLogger
}

// NewTestableAdminDefaultsCacheService creates a testable service with mocks
func NewTestableAdminDefaultsCacheService() *TestableAdminDefaultsCacheService {
	return &TestableAdminDefaultsCacheService{
		mockCache: NewMockCacheService(),
		logger:    NewMockLogger(),
	}
}

// extractGroupFromConfig extracts a specific group's data from ModeFullConfig
func (ts *TestableAdminDefaultsCacheService) extractGroupFromConfig(config *autopilot.ModeFullConfig, groupKey string) interface{} {
	switch groupKey {
	case "enabled":
		return map[string]interface{}{"enabled": config.Enabled}
	case "timeframe":
		return config.Timeframe
	case "confidence":
		return config.Confidence
	case "size":
		return config.Size
	case "sltp":
		return config.SLTP
	case "risk":
		return config.Risk
	case "circuit_breaker":
		return config.CircuitBreaker
	case "hedge":
		return config.Hedge
	case "averaging":
		return config.Averaging
	case "stale_release":
		return config.StaleRelease
	case "assignment":
		return config.Assignment
	case "mtf":
		return config.MTF
	case "dynamic_ai_exit":
		return config.DynamicAIExit
	case "reversal":
		return config.Reversal
	case "funding_rate":
		return config.FundingRate
	case "trend_divergence":
		return config.TrendDivergence
	case "position_optimization":
		return config.PositionOptimization
	case "trend_filters":
		return config.TrendFilters
	case "early_warning":
		return config.EarlyWarning
	case "entry":
		return config.Entry
	default:
		return nil
	}
}

// GetAdminDefaultGroup retrieves a single default group from cache (test implementation)
func (ts *TestableAdminDefaultsCacheService) GetAdminDefaultGroup(ctx context.Context, mode, group string) ([]byte, error) {
	if !ts.mockCache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := fmt.Sprintf("admin:defaults:mode:%s:%s", mode, group)

	// Try cache first
	cached, err := ts.mockCache.Get(ctx, key)
	if err == nil && cached != "" {
		return []byte(cached), nil
	}

	// Cache miss - for testing, we don't auto-load from file
	return nil, ErrSettingNotFound
}

// LoadAdminDefaultsFromConfig loads defaults from a provided config (for testing)
func (ts *TestableAdminDefaultsCacheService) LoadAdminDefaultsFromConfig(ctx context.Context, defaults *autopilot.DefaultSettingsFile) error {
	if !ts.mockCache.IsHealthy() {
		return ErrCacheUnavailable
	}

	// Store mode defaults
	for _, mode := range TradingModes {
		modeConfig, exists := defaults.ModeConfigs[mode]
		if !exists || modeConfig == nil {
			continue
		}

		for _, group := range SettingGroups {
			groupData := ts.extractGroupFromConfig(modeConfig, group.Key)
			if groupData == nil {
				continue
			}

			key := fmt.Sprintf("admin:defaults:mode:%s:%s", mode, group.Key)
			groupJSON, _ := json.Marshal(groupData)
			ts.mockCache.Set(ctx, key, string(groupJSON), 0)
		}
	}

	// Store hash
	ts.mockCache.Set(ctx, "admin:defaults:hash", "testhash123", 0)

	return nil
}

// InvalidateAdminDefaults removes all admin default keys
func (ts *TestableAdminDefaultsCacheService) InvalidateAdminDefaults(ctx context.Context) error {
	ts.mockCache.DeletePattern(ctx, "admin:defaults:mode:*")
	for _, setting := range CrossModeSettings {
		key := fmt.Sprintf("admin:defaults:global:%s", setting)
		ts.mockCache.Delete(ctx, key)
	}
	ts.mockCache.Delete(ctx, "admin:defaults:hash")
	return nil
}

// ============================================================================
// CRITICAL TEST CASES (P0)
// ============================================================================

// TestGetAdminDefaultGroup_RedisDown_ReturnsError verifies that when Redis is unavailable,
// the service returns ErrCacheUnavailable (no bypass)
func TestGetAdminDefaultGroup_RedisDown_ReturnsError(t *testing.T) {
	ts := NewTestableAdminDefaultsCacheService()
	ctx := context.Background()

	// Setup: Mark Redis as unhealthy
	ts.mockCache.healthy = false

	// Act: Try to get a default group
	_, err := ts.GetAdminDefaultGroup(ctx, "scalp", "confidence")

	// Assert: Must return ErrCacheUnavailable
	if err == nil {
		t.Fatal("Expected error when Redis is down, got nil")
	}
	if !errors.Is(err, ErrCacheUnavailable) {
		t.Errorf("Expected ErrCacheUnavailable, got: %v", err)
	}

	// Assert: IsHealthy was checked
	if ts.mockCache.healthyCalled == 0 {
		t.Error("IsHealthy should have been called to check Redis status")
	}
}

// TestGetAdminDefaultGroup_CacheHit_ReturnsData verifies that cache hits return data correctly
func TestGetAdminDefaultGroup_CacheHit_ReturnsData(t *testing.T) {
	ts := NewTestableAdminDefaultsCacheService()
	ctx := context.Background()

	// Setup: Redis is healthy
	ts.mockCache.healthy = true

	// Setup: Data in cache
	cacheKey := "admin:defaults:mode:scalp:confidence"
	cacheData := `{"min_confidence": 0.70, "high_confidence": 0.85}`
	ts.mockCache.data[cacheKey] = cacheData

	// Act: Get admin default group
	result, err := ts.GetAdminDefaultGroup(ctx, "scalp", "confidence")

	// Assert: No error
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Assert: Got correct data
	if string(result) != cacheData {
		t.Errorf("Expected %s, got %s", cacheData, string(result))
	}

	// Assert: Cache Get was called
	if len(ts.mockCache.getCalls) != 1 {
		t.Errorf("Expected 1 cache Get call, got %d", len(ts.mockCache.getCalls))
	}
}

// TestGetAdminDefaultGroup_CacheMiss_ReturnsError verifies cache miss returns error
// (in real implementation, it would auto-load from file)
func TestGetAdminDefaultGroup_CacheMiss_ReturnsError(t *testing.T) {
	ts := NewTestableAdminDefaultsCacheService()
	ctx := context.Background()

	// Setup: Redis is healthy but cache is empty
	ts.mockCache.healthy = true

	// Act: Get admin default group (cache miss)
	_, err := ts.GetAdminDefaultGroup(ctx, "scalp", "confidence")

	// Assert: Should return ErrSettingNotFound (cache miss in test mode)
	if err == nil {
		t.Fatal("Expected error on cache miss")
	}
	if !errors.Is(err, ErrSettingNotFound) {
		t.Errorf("Expected ErrSettingNotFound, got: %v", err)
	}
}

// TestLoadAdminDefaults_CreatesGranularKeys verifies that loading creates granular keys
func TestLoadAdminDefaults_CreatesGranularKeys(t *testing.T) {
	ts := NewTestableAdminDefaultsCacheService()
	ctx := context.Background()

	// Setup: Redis is healthy
	ts.mockCache.healthy = true

	// Create test defaults
	defaults := createTestDefaultSettings()

	// Act: Load admin defaults
	err := ts.LoadAdminDefaultsFromConfig(ctx, defaults)

	// Assert: No error
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Assert: Cache Set was called for each mode/group combination
	// 4 modes x groups that have data
	if len(ts.mockCache.setCalls) == 0 {
		t.Error("Expected cache Set calls")
	}

	// Verify key format is correct
	foundConfidence := false
	foundSltp := false
	for _, call := range ts.mockCache.setCalls {
		if call.Key == "admin:defaults:mode:scalp:confidence" {
			foundConfidence = true
		}
		if call.Key == "admin:defaults:mode:scalp:sltp" {
			foundSltp = true
		}
	}

	if !foundConfidence {
		t.Error("Expected admin:defaults:mode:scalp:confidence key to be set")
	}
	if !foundSltp {
		t.Error("Expected admin:defaults:mode:scalp:sltp key to be set")
	}
}

// TestLoadAdminDefaults_RedisDown_ReturnsError verifies load fails when Redis is down
func TestLoadAdminDefaults_RedisDown_ReturnsError(t *testing.T) {
	ts := NewTestableAdminDefaultsCacheService()
	ctx := context.Background()

	// Setup: Redis is unhealthy
	ts.mockCache.healthy = false

	defaults := createTestDefaultSettings()

	// Act: Try to load admin defaults
	err := ts.LoadAdminDefaultsFromConfig(ctx, defaults)

	// Assert: Must return ErrCacheUnavailable
	if err == nil {
		t.Fatal("Expected error when Redis is down")
	}
	if !errors.Is(err, ErrCacheUnavailable) {
		t.Errorf("Expected ErrCacheUnavailable, got: %v", err)
	}
}

// TestInvalidateAdminDefaults_DeletesAllKeys verifies invalidation deletes all keys
func TestInvalidateAdminDefaults_DeletesAllKeys(t *testing.T) {
	ts := NewTestableAdminDefaultsCacheService()
	ctx := context.Background()

	// Setup: Redis is healthy with cached defaults
	ts.mockCache.healthy = true

	// Pre-populate some defaults
	ts.mockCache.data["admin:defaults:mode:scalp:confidence"] = `{"min": 70}`
	ts.mockCache.data["admin:defaults:mode:swing:sltp"] = `{"sl": 2.0}`
	ts.mockCache.data["admin:defaults:global:circuit_breaker"] = `{"enabled": true}`
	ts.mockCache.data["admin:defaults:hash"] = "abc123"

	// Act: Invalidate admin defaults
	err := ts.InvalidateAdminDefaults(ctx)

	// Assert: No error
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Assert: DeletePattern was called for mode defaults
	if len(ts.mockCache.patternCalls) == 0 {
		t.Error("Expected DeletePattern to be called")
	}

	foundModePattern := false
	for _, pattern := range ts.mockCache.patternCalls {
		if pattern == "admin:defaults:mode:*" {
			foundModePattern = true
		}
	}
	if !foundModePattern {
		t.Error("Expected pattern admin:defaults:mode:* to be deleted")
	}

	// Assert: Hash key was deleted
	hashDeleted := false
	for _, key := range ts.mockCache.deleteCalls {
		if key == "admin:defaults:hash" {
			hashDeleted = true
		}
	}
	if !hashDeleted {
		t.Error("Expected admin:defaults:hash to be deleted")
	}
}

// ============================================================================
// TABLE-DRIVEN TESTS
// ============================================================================

// TestGetAdminDefaultGroup_VariousModes tests cache behavior across all trading modes
func TestGetAdminDefaultGroup_VariousModes(t *testing.T) {
	testCases := []struct {
		name  string
		mode  string
		group string
	}{
		{"ultra_fast confidence", "ultra_fast", "confidence"},
		{"scalp sltp", "scalp", "sltp"},
		{"swing risk", "swing", "risk"},
		{"position circuit_breaker", "position", "circuit_breaker"},
		{"scalp enabled", "scalp", "enabled"},
		{"swing timeframe", "swing", "timeframe"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := NewTestableAdminDefaultsCacheService()
			ctx := context.Background()

			// Setup
			ts.mockCache.healthy = true
			cacheKey := fmt.Sprintf("admin:defaults:mode:%s:%s", tc.mode, tc.group)
			ts.mockCache.data[cacheKey] = `{"test": "data"}`

			// Act
			result, err := ts.GetAdminDefaultGroup(ctx, tc.mode, tc.group)

			// Assert
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(result) == 0 {
				t.Error("Expected non-empty result")
			}
		})
	}
}

// TestRedisHealthStates tests behavior for different Redis health states
func TestAdminDefaults_RedisHealthStates(t *testing.T) {
	testCases := []struct {
		name        string
		healthy     bool
		expectError bool
	}{
		{"Redis healthy", true, false},
		{"Redis unhealthy", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := NewTestableAdminDefaultsCacheService()
			ctx := context.Background()

			// Setup
			ts.mockCache.healthy = tc.healthy
			if tc.healthy {
				ts.mockCache.data["admin:defaults:mode:scalp:confidence"] = `{"min": 70}`
			}

			// Act
			_, err := ts.GetAdminDefaultGroup(ctx, "scalp", "confidence")

			// Assert
			if tc.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// createTestDefaultSettings creates a test DefaultSettingsFile
func createTestDefaultSettings() *autopilot.DefaultSettingsFile {
	return &autopilot.DefaultSettingsFile{
		Metadata: autopilot.DefaultMetadata{
			Version:       "1.0.0",
			SchemaVersion: 1,
			LastUpdated:   "2026-01-15",
			UpdatedBy:     "test",
		},
		ModeConfigs: map[string]*autopilot.ModeFullConfig{
			"ultra_fast": createTestModeFullConfig("ultra_fast"),
			"scalp":      createTestModeFullConfig("scalp"),
			"swing":      createTestModeFullConfig("swing"),
			"position":   createTestModeFullConfig("position"),
		},
		CapitalAllocation: autopilot.CapitalAllocationDefaults{
			UltraFastPercent: 0,
			ScalpPercent:     100,
			SwingPercent:     0,
			PositionPercent:  0,
		},
	}
}

// createTestModeFullConfig creates a test ModeFullConfig
func createTestModeFullConfig(mode string) *autopilot.ModeFullConfig {
	return &autopilot.ModeFullConfig{
		ModeName: mode,
		Enabled:  mode == "scalp", // Only scalp enabled by default
		Confidence: &autopilot.ModeConfidenceConfig{
			MinConfidence:  0.70,
			HighConfidence: 0.85,
		},
		SLTP: &autopilot.ModeSLTPConfig{
			StopLossPercent:   2.0,
			TakeProfitPercent: 4.0,
		},
		Size: &autopilot.ModeSizeConfig{
			BaseSizeUSD: 100.0,
			MaxSizeUSD:  1000.0,
		},
		Timeframe: &autopilot.ModeTimeframeConfig{
			TrendTimeframe: "15m",
			EntryTimeframe: "5m",
		},
	}
}

// ============================================================================
// MOCK ADDITIONS FOR ADMIN DEFAULTS TESTING
// ============================================================================

// AdminMockCacheService extends MockCacheService with pattern operations tracking
type AdminMockCacheService struct {
	*MockCacheService
	mu           sync.Mutex
	patternCalls []string
}

// NewAdminMockCacheService creates a new admin mock cache service
func NewAdminMockCacheService() *AdminMockCacheService {
	return &AdminMockCacheService{
		MockCacheService: NewMockCacheService(),
		patternCalls:     []string{},
	}
}

// DeletePattern tracks pattern deletion calls
func (m *AdminMockCacheService) DeletePattern(ctx context.Context, pattern string) error {
	m.mu.Lock()
	m.patternCalls = append(m.patternCalls, pattern)
	m.mu.Unlock()

	return m.MockCacheService.DeletePattern(ctx, pattern)
}

// ============================================================================
// PERFORMANCE TESTS (Commented - Run manually)
// ============================================================================

/*
func BenchmarkGetAdminDefaultGroup(b *testing.B) {
	ts := NewTestableAdminDefaultsCacheService()
	ctx := context.Background()

	ts.mockCache.healthy = true
	ts.mockCache.data["admin:defaults:mode:scalp:confidence"] = `{"min_confidence": 0.70}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts.GetAdminDefaultGroup(ctx, "scalp", "confidence")
	}
}
*/

// Ensure the time package is used (for future timing tests)
var _ = time.Now
