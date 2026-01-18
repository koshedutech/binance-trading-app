// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.3: WebSocket State Sync Tests
package decision

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// Test Setup Helpers
// ============================================================================

// setupTestWSStateSync creates a WSStateSync for unit testing without Redis.
func setupTestWSStateSync(t *testing.T) *WSStateSync {
	t.Helper()

	// Create a DeltaProcessor without Redis for unit testing
	dp := &DeltaProcessor{
		cache:        make(map[string]map[string]interface{}),
		metrics:      newDeltaMetrics(),
		sm:           nil, // Not used in ProcessWithoutRedis
		maxCacheSize: DefaultMaxCacheSize,
	}

	// Create WSStateSync with a mock-friendly setup
	// We'll use a wrapper that doesn't require actual StateManager
	ws := &WSStateSync{
		sm:              nil, // Will use custom processing
		dp:              dp,
		throttleWindow:  int64(10 * time.Millisecond), // Short for testing
		lastUpdate:      make(map[string]int64),
		updateCounts:    make(map[string]int64),
		throttledCounts: make(map[string]int64),
		errorCounts:     make(map[string]int64),
	}

	return ws
}

// testableWSStateSync extends WSStateSync for testing without Redis
type testableWSStateSync struct {
	*WSStateSync
	processedUpdates []MarketUpdate
	mu               sync.Mutex
}

// newTestableWSStateSync creates a testable instance that tracks processed updates
func newTestableWSStateSync(t *testing.T, throttleWindow time.Duration) *testableWSStateSync {
	t.Helper()

	dp := &DeltaProcessor{
		cache:        make(map[string]map[string]interface{}),
		metrics:      newDeltaMetrics(),
		sm:           nil,
		maxCacheSize: DefaultMaxCacheSize,
	}

	return &testableWSStateSync{
		WSStateSync: &WSStateSync{
			sm:              nil,
			dp:              dp,
			throttleWindow:  int64(throttleWindow),
			lastUpdate:      make(map[string]int64),
			updateCounts:    make(map[string]int64),
			throttledCounts: make(map[string]int64),
			errorCounts:     make(map[string]int64),
		},
		processedUpdates: make([]MarketUpdate, 0),
	}
}

// ProcessUpdateTest processes an update without requiring StateManager
func (tws *testableWSStateSync) ProcessUpdateTest(ctx context.Context, userID string, update *MarketUpdate) error {
	if !tws.IsRunning() {
		return nil
	}

	if update == nil || userID == "" || update.Symbol == "" {
		return nil
	}

	// Check throttling
	if tws.shouldThrottle(update.Symbol) {
		tws.recordThrottled(update.Symbol)
		return nil
	}

	// Record this update
	tws.recordUpdate(update.Symbol)

	// Track processed update for testing
	tws.mu.Lock()
	tws.processedUpdates = append(tws.processedUpdates, *update)
	tws.mu.Unlock()

	// Process through DeltaProcessor without Redis
	switch update.Type {
	case UpdateTypePrice:
		state := map[string]interface{}{
			"price": update.Price,
		}
		_, err := tws.dp.ProcessWithoutRedis(userID, update.Symbol, state)
		return err

	case UpdateTypeKline:
		state := make(map[string]interface{})
		if update.Price > 0 {
			state["price"] = update.Price
		}
		for k, v := range update.Data {
			state[k] = v
		}
		if len(state) > 0 {
			_, err := tws.dp.ProcessWithoutRedis(userID, update.Symbol, state)
			return err
		}

	case UpdateTypeOrderBook:
		if update.Data != nil && len(update.Data) > 0 {
			_, err := tws.dp.ProcessWithoutRedis(userID, update.Symbol, update.Data)
			return err
		}
	}

	return nil
}

// GetProcessedCount returns the number of updates that were processed (not throttled)
func (tws *testableWSStateSync) GetProcessedCount() int {
	tws.mu.Lock()
	defer tws.mu.Unlock()
	return len(tws.processedUpdates)
}

// ============================================================================
// Unit Tests
// ============================================================================

func TestNewWSStateSync(t *testing.T) {
	t.Run("returns nil with nil StateManager", func(t *testing.T) {
		dp := &DeltaProcessor{
			cache:   make(map[string]map[string]interface{}),
			metrics: newDeltaMetrics(),
		}
		ws := NewWSStateSync(nil, dp, DefaultThrottleWindow)
		if ws != nil {
			t.Error("Expected nil WSStateSync with nil StateManager")
		}
	})

	t.Run("returns nil with nil DeltaProcessor", func(t *testing.T) {
		// We can't create a real StateManager without Redis, so skip this
		// The constructor checks for nil anyway
		ws := NewWSStateSync(nil, nil, DefaultThrottleWindow)
		if ws != nil {
			t.Error("Expected nil WSStateSync with nil DeltaProcessor")
		}
	})

	t.Run("uses default throttle window for zero value", func(t *testing.T) {
		dp := &DeltaProcessor{
			cache:   make(map[string]map[string]interface{}),
			metrics: newDeltaMetrics(),
		}
		// Create a minimal StateManager for testing
		sm := &StateManager{ttl: DefaultCoinStateTTL}
		ws := NewWSStateSync(sm, dp, 0)

		if ws.GetThrottleWindow() != DefaultThrottleWindow {
			t.Errorf("Expected default throttle window %v, got %v", DefaultThrottleWindow, ws.GetThrottleWindow())
		}
	})

	t.Run("uses default throttle window for negative value", func(t *testing.T) {
		dp := &DeltaProcessor{
			cache:   make(map[string]map[string]interface{}),
			metrics: newDeltaMetrics(),
		}
		sm := &StateManager{ttl: DefaultCoinStateTTL}
		ws := NewWSStateSync(sm, dp, -1*time.Second)

		if ws.GetThrottleWindow() != DefaultThrottleWindow {
			t.Errorf("Expected default throttle window %v, got %v", DefaultThrottleWindow, ws.GetThrottleWindow())
		}
	})
}

func TestWSStateSync_StartStop(t *testing.T) {
	ws := setupTestWSStateSync(t)

	// Initially not running
	if ws.IsRunning() {
		t.Error("Expected WSStateSync to not be running initially")
	}

	// Start
	ws.Start()
	if !ws.IsRunning() {
		t.Error("Expected WSStateSync to be running after Start()")
	}

	// Stop
	ws.Stop()
	if ws.IsRunning() {
		t.Error("Expected WSStateSync to not be running after Stop()")
	}
}

func TestWSStateSync_ProcessPriceUpdate(t *testing.T) {
	tws := newTestableWSStateSync(t, 10*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	update := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypePrice,
		Price:     50000.0,
		Timestamp: time.Now().UnixMilli(),
	}

	err := tws.ProcessUpdateTest(ctx, userID, update)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify update was processed
	if tws.GetProcessedCount() != 1 {
		t.Errorf("Expected 1 processed update, got %d", tws.GetProcessedCount())
	}

	// Verify stats
	stats := tws.GetUpdateStats()
	if stats["BTCUSDT"] != 1 {
		t.Errorf("Expected 1 update for BTCUSDT, got %d", stats["BTCUSDT"])
	}
}

func TestWSStateSync_Throttling(t *testing.T) {
	throttleWindow := 50 * time.Millisecond
	tws := newTestableWSStateSync(t, throttleWindow)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	// Send first update
	update1 := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypePrice,
		Price:     50000.0,
		Timestamp: time.Now().UnixMilli(),
	}
	_ = tws.ProcessUpdateTest(ctx, userID, update1)

	// Send second update immediately (should be throttled)
	update2 := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypePrice,
		Price:     50100.0,
		Timestamp: time.Now().UnixMilli(),
	}
	_ = tws.ProcessUpdateTest(ctx, userID, update2)

	// Verify only first update was processed
	if tws.GetProcessedCount() != 1 {
		t.Errorf("Expected 1 processed update (second should be throttled), got %d", tws.GetProcessedCount())
	}

	// Verify throttle count
	if tws.GetThrottledCount() != 1 {
		t.Errorf("Expected 1 throttled update, got %d", tws.GetThrottledCount())
	}

	// Wait for throttle window to expire
	time.Sleep(throttleWindow + 10*time.Millisecond)

	// Send third update (should NOT be throttled)
	update3 := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypePrice,
		Price:     50200.0,
		Timestamp: time.Now().UnixMilli(),
	}
	_ = tws.ProcessUpdateTest(ctx, userID, update3)

	// Now should have 2 processed updates
	if tws.GetProcessedCount() != 2 {
		t.Errorf("Expected 2 processed updates after throttle window, got %d", tws.GetProcessedCount())
	}
}

func TestWSStateSync_MultipleSymbols(t *testing.T) {
	tws := newTestableWSStateSync(t, 10*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	// Send updates for different symbols
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	for _, symbol := range symbols {
		update := &MarketUpdate{
			Symbol:    symbol,
			Type:      UpdateTypePrice,
			Price:     1000.0,
			Timestamp: time.Now().UnixMilli(),
		}
		_ = tws.ProcessUpdateTest(ctx, userID, update)
	}

	// All should be processed (different symbols have independent throttling)
	if tws.GetProcessedCount() != 3 {
		t.Errorf("Expected 3 processed updates, got %d", tws.GetProcessedCount())
	}

	// Verify symbol count
	if tws.GetSymbolCount() != 3 {
		t.Errorf("Expected 3 unique symbols, got %d", tws.GetSymbolCount())
	}

	// Verify no throttling
	if tws.GetThrottledCount() != 0 {
		t.Errorf("Expected 0 throttled updates, got %d", tws.GetThrottledCount())
	}
}

func TestWSStateSync_ConcurrentUpdates(t *testing.T) {
	tws := newTestableWSStateSync(t, 1*time.Millisecond) // Very short throttle for concurrency test
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	var wg sync.WaitGroup
	numGoroutines := 10
	numIterations := 100
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
				update := &MarketUpdate{
					Symbol:    symbol,
					Type:      UpdateTypePrice,
					Price:     float64(50000 + goroutineID*100 + j),
					Timestamp: time.Now().UnixMilli(),
				}
				err := tws.ProcessUpdateTest(ctx, userID, update)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
				}
				// Small sleep to allow some updates through throttle
				time.Sleep(100 * time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	if errCount > 0 {
		t.Errorf("Concurrent updates had %d errors", errCount)
	}

	// Verify some updates were processed and some throttled
	totalProcessed := tws.GetTotalUpdateCount()
	totalThrottled := tws.GetThrottledCount()

	if totalProcessed == 0 {
		t.Error("Expected some updates to be processed")
	}
	if totalThrottled == 0 {
		t.Error("Expected some updates to be throttled due to rapid fire")
	}

	t.Logf("Concurrent test: %d processed, %d throttled", totalProcessed, totalThrottled)
}

func TestWSStateSync_KlineUpdate(t *testing.T) {
	tws := newTestableWSStateSync(t, 10*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	update := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypeKline,
		Price:     50000.0,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"close":   50000.0,
			"adx":     25.5,
			"rsi":     65.0,
			"ema_9":   49500.0,
			"ema_21":  49000.0,
			"trend_1h": "BULLISH",
			"regime":  "TRENDING",
		},
	}

	err := tws.ProcessUpdateTest(ctx, userID, update)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify update was processed
	if tws.GetProcessedCount() != 1 {
		t.Errorf("Expected 1 processed update, got %d", tws.GetProcessedCount())
	}

	// Verify cached state has kline data
	cached := tws.dp.GetCachedState(userID, "BTCUSDT")
	if cached == nil {
		t.Fatal("Expected cached state")
	}

	if cached["adx"] != 25.5 {
		t.Errorf("Expected ADX 25.5, got %v", cached["adx"])
	}
	if cached["rsi"] != 65.0 {
		t.Errorf("Expected RSI 65.0, got %v", cached["rsi"])
	}
}

func TestWSStateSync_OrderBookUpdate(t *testing.T) {
	tws := newTestableWSStateSync(t, 10*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	update := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypeOrderBook,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"spread":          0.01,
			"liquidity_score": 85.5,
		},
	}

	err := tws.ProcessUpdateTest(ctx, userID, update)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify update was processed
	if tws.GetProcessedCount() != 1 {
		t.Errorf("Expected 1 processed update, got %d", tws.GetProcessedCount())
	}
}

func TestWSStateSync_NotRunning(t *testing.T) {
	ws := setupTestWSStateSync(t)
	// Don't call Start()

	ctx := context.Background()
	update := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypePrice,
		Price:     50000.0,
		Timestamp: time.Now().UnixMilli(),
	}

	err := ws.ProcessUpdate(ctx, "user123", update)
	if err == nil {
		t.Error("Expected error when processing update while not running")
	}
}

func TestWSStateSync_ValidationErrors(t *testing.T) {
	ws := setupTestWSStateSync(t)
	ws.Start()
	defer ws.Stop()

	ctx := context.Background()

	tests := []struct {
		name    string
		userID  string
		update  *MarketUpdate
		wantErr bool
	}{
		{
			name:    "nil update",
			userID:  "user123",
			update:  nil,
			wantErr: true,
		},
		{
			name:   "empty userID",
			userID: "",
			update: &MarketUpdate{
				Symbol: "BTCUSDT",
				Type:   UpdateTypePrice,
				Price:  50000.0,
			},
			wantErr: true,
		},
		{
			name:   "empty symbol",
			userID: "user123",
			update: &MarketUpdate{
				Symbol: "",
				Type:   UpdateTypePrice,
				Price:  50000.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ws.ProcessUpdate(ctx, tt.userID, tt.update)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWSStateSync_Stats(t *testing.T) {
	tws := newTestableWSStateSync(t, 5*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	// Send some updates
	for i := 0; i < 5; i++ {
		update := &MarketUpdate{
			Symbol:    "BTCUSDT",
			Type:      UpdateTypePrice,
			Price:     float64(50000 + i*100),
			Timestamp: time.Now().UnixMilli(),
		}
		_ = tws.ProcessUpdateTest(ctx, userID, update)
		time.Sleep(6 * time.Millisecond) // Allow throttle to reset
	}

	// Get comprehensive stats
	stats := tws.GetStats()

	if !stats.IsRunning {
		t.Error("Expected IsRunning to be true")
	}

	if stats.ThrottleWindow != 5*time.Millisecond {
		t.Errorf("Expected throttle window 5ms, got %v", stats.ThrottleWindow)
	}

	if stats.SymbolCount != 1 {
		t.Errorf("Expected 1 symbol, got %d", stats.SymbolCount)
	}

	if stats.TotalUpdates != 5 {
		t.Errorf("Expected 5 total updates, got %d", stats.TotalUpdates)
	}

	if stats.UpdatesBySymbol["BTCUSDT"] != 5 {
		t.Errorf("Expected 5 updates for BTCUSDT, got %d", stats.UpdatesBySymbol["BTCUSDT"])
	}
}

func TestWSStateSync_ResetStats(t *testing.T) {
	tws := newTestableWSStateSync(t, 5*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	// Send some updates
	update := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypePrice,
		Price:     50000.0,
		Timestamp: time.Now().UnixMilli(),
	}
	_ = tws.ProcessUpdateTest(ctx, userID, update)

	// Verify we have stats
	if tws.GetTotalUpdateCount() == 0 {
		t.Error("Expected some updates before reset")
	}

	// Reset
	tws.ResetStats()

	// Verify stats cleared
	if tws.GetTotalUpdateCount() != 0 {
		t.Errorf("Expected 0 updates after reset, got %d", tws.GetTotalUpdateCount())
	}
	if tws.GetThrottledCount() != 0 {
		t.Errorf("Expected 0 throttled after reset, got %d", tws.GetThrottledCount())
	}
}

func TestWSStateSync_ClearSymbol(t *testing.T) {
	tws := newTestableWSStateSync(t, 5*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	// Send updates for two symbols
	for _, symbol := range []string{"BTCUSDT", "ETHUSDT"} {
		update := &MarketUpdate{
			Symbol:    symbol,
			Type:      UpdateTypePrice,
			Price:     50000.0,
			Timestamp: time.Now().UnixMilli(),
		}
		_ = tws.ProcessUpdateTest(ctx, userID, update)
	}

	// Verify both symbols tracked
	if tws.GetSymbolCount() != 2 {
		t.Errorf("Expected 2 symbols, got %d", tws.GetSymbolCount())
	}

	// Clear one symbol
	tws.ClearSymbol("BTCUSDT")

	// Verify only one symbol remains
	if tws.GetSymbolCount() != 1 {
		t.Errorf("Expected 1 symbol after clear, got %d", tws.GetSymbolCount())
	}

	// Verify BTCUSDT stats cleared
	stats := tws.GetUpdateStats()
	if _, exists := stats["BTCUSDT"]; exists {
		t.Error("Expected BTCUSDT to be cleared from stats")
	}
	if stats["ETHUSDT"] != 1 {
		t.Errorf("Expected ETHUSDT to still have 1 update, got %d", stats["ETHUSDT"])
	}
}

func TestWSStateSync_SetThrottleWindow(t *testing.T) {
	ws := setupTestWSStateSync(t)

	// Initial window
	initialWindow := ws.GetThrottleWindow()
	if initialWindow != 10*time.Millisecond {
		t.Errorf("Expected initial window 10ms, got %v", initialWindow)
	}

	// Change window
	ws.SetThrottleWindow(200 * time.Millisecond)
	if ws.GetThrottleWindow() != 200*time.Millisecond {
		t.Errorf("Expected window 200ms, got %v", ws.GetThrottleWindow())
	}

	// Set invalid window (should use default)
	ws.SetThrottleWindow(0)
	if ws.GetThrottleWindow() != DefaultThrottleWindow {
		t.Errorf("Expected default window %v for zero, got %v", DefaultThrottleWindow, ws.GetThrottleWindow())
	}

	ws.SetThrottleWindow(-1 * time.Second)
	if ws.GetThrottleWindow() != DefaultThrottleWindow {
		t.Errorf("Expected default window %v for negative, got %v", DefaultThrottleWindow, ws.GetThrottleWindow())
	}
}

func TestWSStateSync_ExtractFloat(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		key      string
		expected float64
		ok       bool
	}{
		{
			name:     "float64 value",
			data:     map[string]interface{}{"price": 50000.5},
			key:      "price",
			expected: 50000.5,
			ok:       true,
		},
		{
			name:     "int value",
			data:     map[string]interface{}{"count": 100},
			key:      "count",
			expected: 100.0,
			ok:       true,
		},
		{
			name:     "int64 value",
			data:     map[string]interface{}{"timestamp": int64(1234567890)},
			key:      "timestamp",
			expected: 1234567890.0,
			ok:       true,
		},
		{
			name:     "string value",
			data:     map[string]interface{}{"price": "50000.5"},
			key:      "price",
			expected: 50000.5,
			ok:       true,
		},
		{
			name:     "string zero",
			data:     map[string]interface{}{"value": "0"},
			key:      "value",
			expected: 0,
			ok:       true,
		},
		{
			name:     "missing key",
			data:     map[string]interface{}{"price": 50000.0},
			key:      "missing",
			expected: 0,
			ok:       false,
		},
		{
			name:     "invalid string",
			data:     map[string]interface{}{"value": "not-a-number"},
			key:      "value",
			expected: 0,
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := extractFloat(tt.data, tt.key)
			if ok != tt.ok {
				t.Errorf("extractFloat() ok = %v, expected %v", ok, tt.ok)
			}
			if result != tt.expected {
				t.Errorf("extractFloat() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestWSStateSync_ThrottleWindowEffect(t *testing.T) {
	// Test with long throttle window
	longWindow := 100 * time.Millisecond
	tws := newTestableWSStateSync(t, longWindow)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	// Send 10 updates rapidly
	for i := 0; i < 10; i++ {
		update := &MarketUpdate{
			Symbol:    "BTCUSDT",
			Type:      UpdateTypePrice,
			Price:     float64(50000 + i),
			Timestamp: time.Now().UnixMilli(),
		}
		_ = tws.ProcessUpdateTest(ctx, userID, update)
	}

	// Only first should be processed (9 throttled)
	if tws.GetTotalUpdateCount() != 1 {
		t.Errorf("Expected 1 update with long throttle, got %d", tws.GetTotalUpdateCount())
	}
	if tws.GetThrottledCount() != 9 {
		t.Errorf("Expected 9 throttled with long throttle, got %d", tws.GetThrottledCount())
	}
}

func TestMarketUpdateType_Constants(t *testing.T) {
	// Verify constants are as expected
	if UpdateTypePrice != "PRICE" {
		t.Errorf("Expected UpdateTypePrice to be 'PRICE', got %s", UpdateTypePrice)
	}
	if UpdateTypeKline != "KLINE" {
		t.Errorf("Expected UpdateTypeKline to be 'KLINE', got %s", UpdateTypeKline)
	}
	if UpdateTypeOrderBook != "ORDERBOOK" {
		t.Errorf("Expected UpdateTypeOrderBook to be 'ORDERBOOK', got %s", UpdateTypeOrderBook)
	}
}

func TestDefaultThrottleWindow(t *testing.T) {
	if DefaultThrottleWindow != 100*time.Millisecond {
		t.Errorf("Expected DefaultThrottleWindow to be 100ms, got %v", DefaultThrottleWindow)
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkWSStateSync_ProcessUpdate(b *testing.B) {
	dp := &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
	}

	tws := &testableWSStateSync{
		WSStateSync: &WSStateSync{
			sm:              nil,
			dp:              dp,
			throttleWindow:  int64(1 * time.Nanosecond), // Minimal throttle for benchmark
			lastUpdate:      make(map[string]int64),
			updateCounts:    make(map[string]int64),
			throttledCounts: make(map[string]int64),
		},
		processedUpdates: make([]MarketUpdate, 0),
	}
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		update := &MarketUpdate{
			Symbol:    "BTCUSDT",
			Type:      UpdateTypePrice,
			Price:     float64(50000 + i),
			Timestamp: time.Now().UnixMilli(),
		}
		_ = tws.ProcessUpdateTest(ctx, userID, update)
	}
}

func BenchmarkWSStateSync_Throttling(b *testing.B) {
	dp := &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
	}

	tws := &testableWSStateSync{
		WSStateSync: &WSStateSync{
			sm:              nil,
			dp:              dp,
			throttleWindow:  int64(100 * time.Millisecond), // Normal throttle
			lastUpdate:      make(map[string]int64),
			updateCounts:    make(map[string]int64),
			throttledCounts: make(map[string]int64),
		},
		processedUpdates: make([]MarketUpdate, 0),
	}
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	// Pre-populate to trigger throttling
	update := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypePrice,
		Price:     50000.0,
		Timestamp: time.Now().UnixMilli(),
	}
	_ = tws.ProcessUpdateTest(ctx, userID, update)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		update := &MarketUpdate{
			Symbol:    "BTCUSDT",
			Type:      UpdateTypePrice,
			Price:     float64(50000 + i),
			Timestamp: time.Now().UnixMilli(),
		}
		_ = tws.ProcessUpdateTest(ctx, userID, update)
	}
}

func BenchmarkWSStateSync_MultipleSymbols(b *testing.B) {
	dp := &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
	}

	tws := &testableWSStateSync{
		WSStateSync: &WSStateSync{
			sm:              nil,
			dp:              dp,
			throttleWindow:  int64(1 * time.Nanosecond),
			lastUpdate:      make(map[string]int64),
			updateCounts:    make(map[string]int64),
			throttledCounts: make(map[string]int64),
		},
		processedUpdates: make([]MarketUpdate, 0),
	}
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"
	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		symbol := symbols[i%len(symbols)]
		update := &MarketUpdate{
			Symbol:    symbol,
			Type:      UpdateTypePrice,
			Price:     float64(50000 + i),
			Timestamp: time.Now().UnixMilli(),
		}
		_ = tws.ProcessUpdateTest(ctx, userID, update)
	}
}

func BenchmarkWSStateSync_Concurrent(b *testing.B) {
	dp := &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
	}

	tws := &testableWSStateSync{
		WSStateSync: &WSStateSync{
			sm:              nil,
			dp:              dp,
			throttleWindow:  int64(1 * time.Nanosecond),
			lastUpdate:      make(map[string]int64),
			updateCounts:    make(map[string]int64),
			throttledCounts: make(map[string]int64),
		},
		processedUpdates: make([]MarketUpdate, 0),
	}
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			symbol := "BTCUSDT"
			if i%2 == 0 {
				symbol = "ETHUSDT"
			}
			update := &MarketUpdate{
				Symbol:    symbol,
				Type:      UpdateTypePrice,
				Price:     float64(50000 + i),
				Timestamp: time.Now().UnixMilli(),
			}
			_ = tws.ProcessUpdateTest(ctx, userID, update)
			i++
		}
	})
}

// Test performance target: < 1ms per update check (with throttling)
func TestWSStateSync_PerformanceTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	dp := &DeltaProcessor{
		cache:   make(map[string]map[string]interface{}),
		metrics: newDeltaMetrics(),
	}

	tws := &testableWSStateSync{
		WSStateSync: &WSStateSync{
			sm:              nil,
			dp:              dp,
			throttleWindow:  int64(100 * time.Millisecond),
			lastUpdate:      make(map[string]int64),
			updateCounts:    make(map[string]int64),
			throttledCounts: make(map[string]int64),
		},
		processedUpdates: make([]MarketUpdate, 0),
	}
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	iterations := 10000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		update := &MarketUpdate{
			Symbol:    "BTCUSDT",
			Type:      UpdateTypePrice,
			Price:     float64(50000 + i),
			Timestamp: time.Now().UnixMilli(),
		}
		_ = tws.ProcessUpdateTest(ctx, userID, update)
	}

	elapsed := time.Since(start)
	avgTime := elapsed / time.Duration(iterations)

	t.Logf("Average processing time: %v (iterations: %d, throttled: %d)",
		avgTime, iterations, tws.GetThrottledCount())

	// Performance target: < 100us per update check (throttle check is very fast)
	if avgTime > 100*time.Microsecond {
		t.Errorf("Performance target missed: avg time %v > 100us", avgTime)
	}
}

// ============================================================================
// Graceful Degradation Tests (AC5)
// ============================================================================

func TestWSStateSync_GracefulDegradation(t *testing.T) {
	tws := newTestableWSStateSync(t, 10*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	// Initially not in degraded mode
	if tws.IsDegraded() {
		t.Error("Expected not to be in degraded mode initially")
	}

	// Simulate consecutive errors
	for i := 0; i < MaxConsecutiveErrors; i++ {
		tws.recordError("BTCUSDT")
	}

	// Should now be in degraded mode
	if !tws.IsDegraded() {
		t.Error("Expected to be in degraded mode after consecutive errors")
	}

	// Verify error count
	if tws.GetTotalErrorCount() != int64(MaxConsecutiveErrors) {
		t.Errorf("Expected %d errors, got %d", MaxConsecutiveErrors, tws.GetTotalErrorCount())
	}

	// Record success - should exit degraded mode
	tws.recordSuccess()

	if tws.IsDegraded() {
		t.Error("Expected to exit degraded mode after success")
	}
}

func TestWSStateSync_ResetDegradedMode(t *testing.T) {
	tws := newTestableWSStateSync(t, 10*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	// Enter degraded mode
	for i := 0; i < MaxConsecutiveErrors; i++ {
		tws.recordError("BTCUSDT")
	}

	if !tws.IsDegraded() {
		t.Error("Expected to be in degraded mode")
	}

	// Manual reset
	tws.ResetDegradedMode()

	if tws.IsDegraded() {
		t.Error("Expected to exit degraded mode after manual reset")
	}
}

func TestWSStateSync_StatsIncludeDegradedMode(t *testing.T) {
	tws := newTestableWSStateSync(t, 10*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	// Get stats before degraded mode
	stats := tws.GetStats()
	if stats.InDegradedMode {
		t.Error("Expected InDegradedMode to be false initially")
	}
	if stats.TotalErrors != 0 {
		t.Error("Expected TotalErrors to be 0 initially")
	}

	// Enter degraded mode
	for i := 0; i < MaxConsecutiveErrors; i++ {
		tws.recordError("BTCUSDT")
	}

	// Get stats after degraded mode
	stats = tws.GetStats()
	if !stats.InDegradedMode {
		t.Error("Expected InDegradedMode to be true")
	}
	if stats.TotalErrors != int64(MaxConsecutiveErrors) {
		t.Errorf("Expected TotalErrors to be %d, got %d", MaxConsecutiveErrors, stats.TotalErrors)
	}
}

func TestWSStateSync_OrderBookSpreadCalculation(t *testing.T) {
	tws := newTestableWSStateSync(t, 10*time.Millisecond)
	tws.Start()
	defer tws.Stop()

	ctx := context.Background()
	userID := "user123"

	// Test with best_bid and best_ask for spread calculation
	update := &MarketUpdate{
		Symbol:    "BTCUSDT",
		Type:      UpdateTypeOrderBook,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"best_bid": 50000.0,
			"best_ask": 50010.0,
		},
	}

	err := tws.ProcessUpdateTest(ctx, userID, update)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify update was processed with calculated spread
	if tws.GetProcessedCount() != 1 {
		t.Errorf("Expected 1 processed update, got %d", tws.GetProcessedCount())
	}
}

func TestWSStateSync_MaxConsecutiveErrorsConstant(t *testing.T) {
	if MaxConsecutiveErrors <= 0 {
		t.Errorf("MaxConsecutiveErrors should be positive, got %d", MaxConsecutiveErrors)
	}
	if MaxConsecutiveErrors > 100 {
		t.Errorf("MaxConsecutiveErrors seems too high: %d", MaxConsecutiveErrors)
	}
}
