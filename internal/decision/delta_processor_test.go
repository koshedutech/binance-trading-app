// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.2: Delta Update Processor Tests
package decision

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// Mock StateManager for Testing
// ============================================================================

// MockStateManager provides a mock implementation for testing DeltaProcessor
// without requiring actual Redis connectivity.
type MockStateManager struct {
	data         map[string]map[string]interface{} // key -> fields
	mu           sync.RWMutex
	updateCalls  []UpdateCall
	updateErr    error
	getCalls     []GetCall
	getErr       error
}

// UpdateCall tracks UpdateCoinState invocations
type UpdateCall struct {
	UserID  string
	Symbol  string
	Updates map[string]interface{}
}

// GetCall tracks GetCoinState invocations
type GetCall struct {
	UserID string
	Symbol string
}

func newMockStateManager() *MockStateManager {
	return &MockStateManager{
		data: make(map[string]map[string]interface{}),
	}
}

// MockUpdateCoinState simulates StateManager.UpdateCoinState
func (m *MockStateManager) MockUpdateCoinState(ctx context.Context, userID, symbol string, updates map[string]interface{}) error {
	if m.updateErr != nil {
		return m.updateErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateCalls = append(m.updateCalls, UpdateCall{UserID: userID, Symbol: symbol, Updates: updates})

	key := userID + ":" + symbol
	if m.data[key] == nil {
		m.data[key] = make(map[string]interface{})
	}
	for k, v := range updates {
		m.data[key][k] = v
	}

	return nil
}

// GetUpdateCalls returns recorded update calls
func (m *MockStateManager) GetUpdateCalls() []UpdateCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.updateCalls
}

// ============================================================================
// DeltaProcessor Unit Tests (No Redis Required)
// ============================================================================

// testDeltaProcessor creates a DeltaProcessor for unit testing without Redis
func setupTestDeltaProcessor(t *testing.T) *DeltaProcessor {
	t.Helper()
	// DeltaProcessor requires a non-nil StateManager, but for unit tests
	// we only use ProcessWithoutRedis which doesn't call StateManager methods.
	// We create a minimal StateManager with nil client that we won't actually use.
	return &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
		sm:      nil, // Not used in ProcessWithoutRedis
	}
}

func TestNewDeltaProcessor(t *testing.T) {
	t.Run("returns nil with nil StateManager", func(t *testing.T) {
		dp := NewDeltaProcessor(nil)
		if dp != nil {
			t.Error("Expected nil DeltaProcessor with nil StateManager")
		}
	})
}

func TestDeltaProcessor_NoChanges(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// First call - all fields are new
	state := map[string]interface{}{
		"price":  50000.0,
		"rsi":    65.5,
		"symbol": "BTCUSDT",
	}

	result1, err := dp.ProcessWithoutRedis(userID, symbol, state)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result1.ChangedFields) != 3 {
		t.Errorf("Expected 3 changed fields on first call, got %d", len(result1.ChangedFields))
	}

	// Second call with same values - no changes expected
	result2, err := dp.ProcessWithoutRedis(userID, symbol, state)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result2.ChangedFields) != 0 {
		t.Errorf("Expected 0 changed fields on second call with same values, got %d: %v",
			len(result2.ChangedFields), result2.ChangedFields)
	}
}

func TestDeltaProcessor_SingleFieldChange(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Initial state
	state := map[string]interface{}{
		"price":  50000.0,
		"rsi":    65.5,
		"symbol": "BTCUSDT",
	}
	_, _ = dp.ProcessWithoutRedis(userID, symbol, state)

	// Change only price
	state["price"] = 51000.0
	result, err := dp.ProcessWithoutRedis(userID, symbol, state)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.ChangedFields) != 1 {
		t.Errorf("Expected 1 changed field, got %d: %v", len(result.ChangedFields), result.ChangedFields)
	}
	if result.ChangedFields[0] != "price" {
		t.Errorf("Expected 'price' to be changed, got %s", result.ChangedFields[0])
	}
}

func TestDeltaProcessor_MultipleFieldChanges(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Initial state
	state := map[string]interface{}{
		"price":           50000.0,
		"rsi":             65.5,
		"adx":             25.0,
		"regime":          RegimeTrending,
		"decision":        DecisionReady,
		"score_technical": 75,
	}
	_, _ = dp.ProcessWithoutRedis(userID, symbol, state)

	// Change multiple fields
	state["price"] = 51000.0
	state["rsi"] = 70.0
	state["decision"] = DecisionBlocked
	result, err := dp.ProcessWithoutRedis(userID, symbol, state)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.ChangedFields) != 3 {
		t.Errorf("Expected 3 changed fields, got %d: %v", len(result.ChangedFields), result.ChangedFields)
	}

	// Verify the changed fields
	changed := make(map[string]bool)
	for _, field := range result.ChangedFields {
		changed[field] = true
	}

	if !changed["price"] {
		t.Error("Expected 'price' to be in changed fields")
	}
	if !changed["rsi"] {
		t.Error("Expected 'rsi' to be in changed fields")
	}
	if !changed["decision"] {
		t.Error("Expected 'decision' to be in changed fields")
	}
}

func TestDeltaProcessor_NewSymbol(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"

	// First symbol
	state1 := map[string]interface{}{
		"price":  50000.0,
		"symbol": "BTCUSDT",
	}
	result1, err := dp.ProcessWithoutRedis(userID, "BTCUSDT", state1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result1.ChangedFields) != 2 {
		t.Errorf("Expected 2 changed fields for new symbol, got %d", len(result1.ChangedFields))
	}

	// Second symbol (new, should have all fields changed)
	state2 := map[string]interface{}{
		"price":  2000.0,
		"symbol": "ETHUSDT",
	}
	result2, err := dp.ProcessWithoutRedis(userID, "ETHUSDT", state2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result2.ChangedFields) != 2 {
		t.Errorf("Expected 2 changed fields for new symbol, got %d", len(result2.ChangedFields))
	}

	// Verify cache has both symbols
	if dp.CacheSize() != 2 {
		t.Errorf("Expected cache size 2, got %d", dp.CacheSize())
	}
}

func TestDeltaProcessor_ConcurrentAccess(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"

	// Run concurrent updates
	var wg sync.WaitGroup
	numGoroutines := 100
	numIterations := 50

	// Use atomic counter for error tracking instead of t.Errorf in goroutines
	var errCount int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			symbol := "BTCUSDT"
			if goroutineID%2 == 0 {
				symbol = "ETHUSDT"
			}

			for j := 0; j < numIterations; j++ {
				state := map[string]interface{}{
					"price": float64(50000 + goroutineID*100 + j),
					"rsi":   float64(50 + j%50),
				}
				_, err := dp.ProcessWithoutRedis(userID, symbol, state)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Check for errors after all goroutines complete
	if errCount > 0 {
		t.Errorf("Concurrent access had %d errors", errCount)
	}

	// Verify metrics were recorded
	stats := dp.GetProcessingStats()
	expectedCalls := int64(numGoroutines * numIterations)
	if stats.TotalProcessCalls != expectedCalls {
		t.Errorf("Expected %d process calls, got %d", expectedCalls, stats.TotalProcessCalls)
	}
}

func TestDeltaProcessor_CompareValues_Float64(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	tests := []struct {
		name     string
		oldVal   interface{}
		newVal   interface{}
		expected bool
	}{
		{"equal floats", 50000.0, 50000.0, true},
		{"different floats", 50000.0, 51000.0, false},
		{"float vs string", 50000.0, "50000", true},
		{"small difference (epsilon)", 1.0000000001, 1.0, true},
		{"larger difference", 1.001, 1.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dp.compareValues(tt.oldVal, tt.newVal)
			if result != tt.expected {
				t.Errorf("compareValues(%v, %v) = %v, expected %v",
					tt.oldVal, tt.newVal, result, tt.expected)
			}
		})
	}
}

func TestDeltaProcessor_CompareValues_Int(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	tests := []struct {
		name     string
		oldVal   interface{}
		newVal   interface{}
		expected bool
	}{
		{"equal ints", 75, 75, true},
		{"different ints", 75, 80, false},
		{"int vs int64", 75, int64(75), true},
		{"int vs float64", 75, 75.0, true},
		{"int vs string", 75, "75", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dp.compareValues(tt.oldVal, tt.newVal)
			if result != tt.expected {
				t.Errorf("compareValues(%v, %v) = %v, expected %v",
					tt.oldVal, tt.newVal, result, tt.expected)
			}
		})
	}
}

func TestDeltaProcessor_CompareValues_String(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	tests := []struct {
		name     string
		oldVal   interface{}
		newVal   interface{}
		expected bool
	}{
		{"equal strings", "BTCUSDT", "BTCUSDT", true},
		{"different strings", "BTCUSDT", "ETHUSDT", false},
		{"empty strings", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dp.compareValues(tt.oldVal, tt.newVal)
			if result != tt.expected {
				t.Errorf("compareValues(%v, %v) = %v, expected %v",
					tt.oldVal, tt.newVal, result, tt.expected)
			}
		})
	}
}

func TestDeltaProcessor_CompareValues_StringSlice(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	tests := []struct {
		name     string
		oldVal   interface{}
		newVal   interface{}
		expected bool
	}{
		{"equal slices", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different slices", []string{"a", "b"}, []string{"a", "c"}, false},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"empty slices", []string{}, []string{}, true},
		{"slice vs json", []string{"a", "b"}, `["a","b"]`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dp.compareValues(tt.oldVal, tt.newVal)
			if result != tt.expected {
				t.Errorf("compareValues(%v, %v) = %v, expected %v",
					tt.oldVal, tt.newVal, result, tt.expected)
			}
		})
	}
}

func TestDeltaProcessor_CompareValues_Enums(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	tests := []struct {
		name     string
		oldVal   interface{}
		newVal   interface{}
		expected bool
	}{
		{"equal MarketRegime", RegimeTrending, RegimeTrending, true},
		{"different MarketRegime", RegimeTrending, RegimeRanging, false},
		{"MarketRegime vs string", RegimeTrending, "TRENDING", true},
		{"equal TrendDirection", TrendBullish, TrendBullish, true},
		{"TrendDirection vs string", TrendBullish, "BULLISH", true},
		{"equal Decision", DecisionReady, DecisionReady, true},
		{"Decision vs string", DecisionBlocked, "BLOCKED", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dp.compareValues(tt.oldVal, tt.newVal)
			if result != tt.expected {
				t.Errorf("compareValues(%v, %v) = %v, expected %v",
					tt.oldVal, tt.newVal, result, tt.expected)
			}
		})
	}
}

func TestDeltaProcessor_CompareValues_Nil(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	tests := []struct {
		name     string
		oldVal   interface{}
		newVal   interface{}
		expected bool
	}{
		{"both nil", nil, nil, true},
		{"old nil", nil, "value", false},
		{"new nil", "value", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dp.compareValues(tt.oldVal, tt.newVal)
			if result != tt.expected {
				t.Errorf("compareValues(%v, %v) = %v, expected %v",
					tt.oldVal, tt.newVal, result, tt.expected)
			}
		})
	}
}

func TestDeltaProcessor_CompareValues_Bool(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	tests := []struct {
		name     string
		oldVal   interface{}
		newVal   interface{}
		expected bool
	}{
		{"equal true", true, true, true},
		{"equal false", false, false, true},
		{"different bool", true, false, false},
		{"bool vs string true", true, "true", true},
		{"bool vs string false", false, "false", true},
		{"bool true vs string false", true, "false", false},
		{"bool false vs string true", false, "true", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dp.compareValues(tt.oldVal, tt.newVal)
			if result != tt.expected {
				t.Errorf("compareValues(%v, %v) = %v, expected %v",
					tt.oldVal, tt.newVal, result, tt.expected)
			}
		})
	}
}

func TestDeltaProcessor_MaxCacheSize(t *testing.T) {
	// Create processor with small max cache size
	dp := &DeltaProcessor{
		cache:        make(map[string]map[string]interface{}),
		metrics:      newDeltaMetrics(),
		sm:           nil, // Not used in ProcessWithoutRedis
		maxCacheSize: 3,   // Small limit for testing
	}

	// Add 3 entries (should all fit)
	_, _ = dp.ProcessWithoutRedis("user1", "BTC", map[string]interface{}{"price": 1.0})
	_, _ = dp.ProcessWithoutRedis("user1", "ETH", map[string]interface{}{"price": 2.0})
	_, _ = dp.ProcessWithoutRedis("user1", "LTC", map[string]interface{}{"price": 3.0})

	if dp.CacheSize() != 3 {
		t.Errorf("Expected cache size 3, got %d", dp.CacheSize())
	}

	// Add 4th entry - should evict one
	_, _ = dp.ProcessWithoutRedis("user1", "XRP", map[string]interface{}{"price": 4.0})

	if dp.CacheSize() != 3 {
		t.Errorf("Expected cache size to remain 3 after eviction, got %d", dp.CacheSize())
	}

	// XRP should be in cache (newest entry)
	if dp.GetCachedState("user1", "XRP") == nil {
		t.Error("Expected XRP to be in cache")
	}
}

func TestDeltaProcessor_GetFieldMetrics(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Process several updates
	for i := 0; i < 5; i++ {
		state := map[string]interface{}{
			"price": float64(50000 + i*100),
		}
		_, _ = dp.ProcessWithoutRedis(userID, symbol, state)
	}

	metrics := dp.GetFieldMetrics()
	if metrics["price"] != 5 {
		t.Errorf("Expected price update count 5, got %d", metrics["price"])
	}
}

func TestDeltaProcessor_GetHotFields(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Initial state
	state := map[string]interface{}{
		"price":  50000.0,
		"rsi":    65.5,
		"adx":    25.0,
		"symbol": "BTCUSDT",
	}
	_, _ = dp.ProcessWithoutRedis(userID, symbol, state)

	// Update price more frequently
	for i := 0; i < 10; i++ {
		state["price"] = float64(50000 + i*100)
		_, _ = dp.ProcessWithoutRedis(userID, symbol, state)
	}

	// Update RSI less frequently
	for i := 0; i < 5; i++ {
		state["rsi"] = float64(60 + i)
		_, _ = dp.ProcessWithoutRedis(userID, symbol, state)
	}

	hotFields := dp.GetHotFields(2)
	if len(hotFields) != 2 {
		t.Fatalf("Expected 2 hot fields, got %d", len(hotFields))
	}

	// Price should be first (most updated)
	if hotFields[0] != "price" {
		t.Errorf("Expected 'price' to be hottest field, got %s", hotFields[0])
	}
}

func TestDeltaProcessor_GetHotFields_EdgeCases(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	// Test with no updates
	hotFields := dp.GetHotFields(5)
	if len(hotFields) != 0 {
		t.Errorf("Expected 0 hot fields with no updates, got %d", len(hotFields))
	}

	// Test with negative topN
	hotFields = dp.GetHotFields(-1)
	if len(hotFields) != 0 {
		t.Errorf("Expected 0 hot fields with negative topN, got %d", len(hotFields))
	}

	// Test with zero topN
	hotFields = dp.GetHotFields(0)
	if len(hotFields) != 0 {
		t.Errorf("Expected 0 hot fields with zero topN, got %d", len(hotFields))
	}
}

func TestDeltaProcessor_GetProcessingStats(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Process some updates
	for i := 0; i < 10; i++ {
		state := map[string]interface{}{
			"price": float64(50000 + i*100),
		}
		_, _ = dp.ProcessWithoutRedis(userID, symbol, state)
	}

	stats := dp.GetProcessingStats()

	if stats.TotalProcessCalls != 10 {
		t.Errorf("Expected 10 process calls, got %d", stats.TotalProcessCalls)
	}
	if stats.TotalUpdates != 10 {
		t.Errorf("Expected 10 total updates, got %d", stats.TotalUpdates)
	}
	if stats.AvgProcessTime <= 0 {
		t.Error("Expected positive average process time")
	}
}

func TestDeltaProcessor_CacheOperations(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"

	// Process state for two symbols
	_, _ = dp.ProcessWithoutRedis(userID, "BTCUSDT", map[string]interface{}{"price": 50000.0})
	_, _ = dp.ProcessWithoutRedis(userID, "ETHUSDT", map[string]interface{}{"price": 2000.0})

	if dp.CacheSize() != 2 {
		t.Errorf("Expected cache size 2, got %d", dp.CacheSize())
	}

	// Get cached state
	cached := dp.GetCachedState(userID, "BTCUSDT")
	if cached == nil {
		t.Fatal("Expected cached state for BTCUSDT")
	}
	if cached["price"] != 50000.0 {
		t.Errorf("Expected cached price 50000, got %v", cached["price"])
	}

	// Clear cache for one symbol
	dp.ClearCacheForSymbol(userID, "BTCUSDT")
	if dp.CacheSize() != 1 {
		t.Errorf("Expected cache size 1 after clear, got %d", dp.CacheSize())
	}

	// Clear all cache
	dp.ClearCache()
	if dp.CacheSize() != 0 {
		t.Errorf("Expected cache size 0 after clear all, got %d", dp.CacheSize())
	}
}

func TestDeltaProcessor_ClearCacheForUser(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	// Process for multiple users
	_, _ = dp.ProcessWithoutRedis("user1", "BTCUSDT", map[string]interface{}{"price": 50000.0})
	_, _ = dp.ProcessWithoutRedis("user1", "ETHUSDT", map[string]interface{}{"price": 2000.0})
	_, _ = dp.ProcessWithoutRedis("user2", "BTCUSDT", map[string]interface{}{"price": 51000.0})

	if dp.CacheSize() != 3 {
		t.Errorf("Expected cache size 3, got %d", dp.CacheSize())
	}

	// Clear for user1 only
	dp.ClearCacheForUser("user1")

	if dp.CacheSize() != 1 {
		t.Errorf("Expected cache size 1 after clearing user1, got %d", dp.CacheSize())
	}

	// Verify user2's state is still cached
	cached := dp.GetCachedState("user2", "BTCUSDT")
	if cached == nil {
		t.Error("Expected user2's state to still be cached")
	}
}

func TestDeltaProcessor_ResetMetrics(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Process some updates
	for i := 0; i < 5; i++ {
		state := map[string]interface{}{
			"price": float64(50000 + i*100),
		}
		_, _ = dp.ProcessWithoutRedis(userID, symbol, state)
	}

	// Verify metrics exist
	stats := dp.GetProcessingStats()
	if stats.TotalProcessCalls != 5 {
		t.Errorf("Expected 5 process calls before reset, got %d", stats.TotalProcessCalls)
	}

	// Reset metrics
	dp.ResetMetrics()

	// Verify metrics are cleared
	stats = dp.GetProcessingStats()
	if stats.TotalProcessCalls != 0 {
		t.Errorf("Expected 0 process calls after reset, got %d", stats.TotalProcessCalls)
	}
	if stats.TotalUpdates != 0 {
		t.Errorf("Expected 0 total updates after reset, got %d", stats.TotalUpdates)
	}
}

func TestDeltaProcessor_PreloadCache(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Create a CoinState
	state := NewCoinState(symbol)
	state.Price = 50000.0
	state.RSI = 65.5
	state.Regime = RegimeTrending

	// Preload cache
	dp.PreloadCache(userID, symbol, state)

	// Verify cache is populated
	cached := dp.GetCachedState(userID, symbol)
	if cached == nil {
		t.Fatal("Expected cached state after preload")
	}

	// Process same values - should have no changes
	result, err := dp.ProcessWithoutRedis(userID, symbol, map[string]interface{}{
		"price":  "50000", // String representation (as stored in Redis)
		"rsi":    "65.5",
		"regime": "TRENDING",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result.ChangedFields) != 0 {
		t.Errorf("Expected 0 changes after preload with same values, got %d: %v",
			len(result.ChangedFields), result.ChangedFields)
	}
}

func TestDeltaProcessor_PreloadCache_Nil(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	// Should not panic with nil state
	dp.PreloadCache("user123", "BTCUSDT", nil)

	if dp.CacheSize() != 0 {
		t.Errorf("Expected cache size 0 after nil preload, got %d", dp.CacheSize())
	}
}

func TestDeltaProcessor_ProcessWithoutRedis_ValidationErrors(t *testing.T) {
	dp := setupTestDeltaProcessor(t)

	tests := []struct {
		name    string
		userID  string
		symbol  string
		state   map[string]interface{}
		wantErr bool
	}{
		{"empty userID", "", "BTCUSDT", map[string]interface{}{"price": 50000.0}, true},
		{"empty symbol", "user123", "", map[string]interface{}{"price": 50000.0}, true},
		{"nil state", "user123", "BTCUSDT", nil, true},
		{"valid input", "user123", "BTCUSDT", map[string]interface{}{"price": 50000.0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dp.ProcessWithoutRedis(tt.userID, tt.symbol, tt.state)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessWithoutRedis() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeltaProcessor_UpdatedValuesInResult(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Initial state
	state := map[string]interface{}{
		"price":  50000.0,
		"rsi":    65.5,
		"symbol": "BTCUSDT",
	}
	result1, _ := dp.ProcessWithoutRedis(userID, symbol, state)

	// Verify UpdatedValues contains all fields on first call
	if len(result1.UpdatedValues) != 3 {
		t.Errorf("Expected 3 updated values on first call, got %d", len(result1.UpdatedValues))
	}

	// Update price
	state["price"] = 51000.0
	result2, _ := dp.ProcessWithoutRedis(userID, symbol, state)

	// Verify only price is in UpdatedValues
	if len(result2.UpdatedValues) != 1 {
		t.Errorf("Expected 1 updated value, got %d", len(result2.UpdatedValues))
	}
	if result2.UpdatedValues["price"] != 51000.0 {
		t.Errorf("Expected price to be 51000.0, got %v", result2.UpdatedValues["price"])
	}
}

func TestDeltaProcessor_ProcessTime(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	state := map[string]interface{}{
		"price":  50000.0,
		"rsi":    65.5,
		"symbol": "BTCUSDT",
	}

	result, err := dp.ProcessWithoutRedis(userID, symbol, state)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// ProcessTime should be > 0
	if result.ProcessTime <= 0 {
		t.Error("Expected positive ProcessTime")
	}
}

func TestDeltaProcessor_NewField(t *testing.T) {
	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Initial state with 2 fields
	state1 := map[string]interface{}{
		"price":  50000.0,
		"symbol": "BTCUSDT",
	}
	_, _ = dp.ProcessWithoutRedis(userID, symbol, state1)

	// Add a new field
	state2 := map[string]interface{}{
		"price":  50000.0,
		"symbol": "BTCUSDT",
		"rsi":    65.5, // New field
	}
	result, _ := dp.ProcessWithoutRedis(userID, symbol, state2)

	// Only the new field should be in changed fields
	if len(result.ChangedFields) != 1 {
		t.Errorf("Expected 1 changed field (new field), got %d: %v",
			len(result.ChangedFields), result.ChangedFields)
	}
	if result.ChangedFields[0] != "rsi" {
		t.Errorf("Expected 'rsi' to be changed, got %s", result.ChangedFields[0])
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkDeltaProcessor_ProcessWithoutRedis_NoChanges(b *testing.B) {
	dp := &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
	}

	state := map[string]interface{}{
		"price":           50000.0,
		"rsi":             65.5,
		"adx":             25.0,
		"regime":          "TRENDING",
		"decision":        "READY",
		"symbol":          "BTCUSDT",
		"score_technical": 75,
		"score_context":   80,
	}
	_, _ = dp.ProcessWithoutRedis("user123", "BTCUSDT", state)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dp.ProcessWithoutRedis("user123", "BTCUSDT", state)
	}
}

func BenchmarkDeltaProcessor_ProcessWithoutRedis_SingleChange(b *testing.B) {
	dp := &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
	}

	state := map[string]interface{}{
		"price":           50000.0,
		"rsi":             65.5,
		"adx":             25.0,
		"regime":          "TRENDING",
		"decision":        "READY",
		"symbol":          "BTCUSDT",
		"score_technical": 75,
		"score_context":   80,
	}
	_, _ = dp.ProcessWithoutRedis("user123", "BTCUSDT", state)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state["price"] = float64(50000 + i)
		_, _ = dp.ProcessWithoutRedis("user123", "BTCUSDT", state)
	}
}

func BenchmarkDeltaProcessor_CompareValues(b *testing.B) {
	dp := &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dp.compareValues(50000.0, 50000.0)
		dp.compareValues("BTCUSDT", "BTCUSDT")
		dp.compareValues(75, 75)
		dp.compareValues([]string{"a", "b"}, []string{"a", "b"})
		dp.compareValues(RegimeTrending, "TRENDING")
	}
}

func BenchmarkDeltaProcessor_Concurrent(b *testing.B) {
	dp := &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			state := map[string]interface{}{
				"price": float64(50000 + i),
				"rsi":   float64(50 + i%50),
			}
			_, _ = dp.ProcessWithoutRedis("user123", "BTCUSDT", state)
			i++
		}
	})
}

// Test to verify < 1ms performance target
func TestDeltaProcessor_PerformanceTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	dp := setupTestDeltaProcessor(t)
	userID := "user123"
	symbol := "BTCUSDT"

	// Pre-populate cache
	state := map[string]interface{}{
		"price":           50000.0,
		"rsi":             65.5,
		"adx":             25.0,
		"regime":          "TRENDING",
		"decision":        "READY",
		"symbol":          "BTCUSDT",
		"score_technical": 75,
		"score_context":   80,
	}
	_, _ = dp.ProcessWithoutRedis(userID, symbol, state)

	// Measure average time over multiple iterations
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		state["price"] = float64(50000 + i)
		_, _ = dp.ProcessWithoutRedis(userID, symbol, state)
	}

	elapsed := time.Since(start)
	avgTime := elapsed / time.Duration(iterations)

	t.Logf("Average processing time: %v", avgTime)

	// Performance target: < 1ms per update (without Redis I/O)
	if avgTime > 1*time.Millisecond {
		t.Errorf("Performance target missed: avg time %v > 1ms", avgTime)
	}
}

// ============================================================================
// Process() with Redis Integration (requires live Redis)
// ============================================================================

// TestDeltaProcessor_Process_Integration tests the full Process() flow
// This test requires actual StateManager/Redis connectivity.
// It's marked with an integration tag and can be skipped in unit test runs.
func TestDeltaProcessor_Process_Integration(t *testing.T) {
	// This test requires a real StateManager, which we don't have in unit tests.
	// It would be run separately in integration tests with actual Redis.
	t.Skip("Integration test - requires Redis connectivity")
}

// ============================================================================
// DeltaResult Tests
// ============================================================================

func TestDeltaResult_Structure(t *testing.T) {
	result := &DeltaResult{
		ChangedFields: []string{"price", "rsi"},
		UpdatedValues: map[string]interface{}{
			"price": 50000.0,
			"rsi":   65.5,
		},
		ProcessTime: 100 * time.Microsecond,
	}

	if len(result.ChangedFields) != 2 {
		t.Errorf("Expected 2 changed fields, got %d", len(result.ChangedFields))
	}
	if len(result.UpdatedValues) != 2 {
		t.Errorf("Expected 2 updated values, got %d", len(result.UpdatedValues))
	}
	if result.ProcessTime != 100*time.Microsecond {
		t.Errorf("Expected process time 100us, got %v", result.ProcessTime)
	}
}

// ============================================================================
// DeltaStats Tests
// ============================================================================

func TestDeltaStats_Structure(t *testing.T) {
	stats := DeltaStats{
		TotalUpdates:      100,
		TotalProcessCalls: 50,
		AvgProcessTime:    500 * time.Microsecond,
		FieldUpdateCounts: map[string]int64{
			"price": 50,
			"rsi":   30,
		},
	}

	if stats.TotalUpdates != 100 {
		t.Errorf("Expected 100 total updates, got %d", stats.TotalUpdates)
	}
	if stats.TotalProcessCalls != 50 {
		t.Errorf("Expected 50 process calls, got %d", stats.TotalProcessCalls)
	}
	if stats.AvgProcessTime != 500*time.Microsecond {
		t.Errorf("Expected avg time 500us, got %v", stats.AvgProcessTime)
	}
	if len(stats.FieldUpdateCounts) != 2 {
		t.Errorf("Expected 2 field counts, got %d", len(stats.FieldUpdateCounts))
	}
}
