package decision

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockListener is a test helper that records regime change notifications.
type mockListener struct {
	changes []struct {
		symbol string
		from   MarketRegime
		to     MarketRegime
	}
	mu sync.Mutex
}

func newMockListener() *mockListener {
	return &mockListener{
		changes: make([]struct {
			symbol string
			from   MarketRegime
			to     MarketRegime
		}, 0),
	}
}

func (m *mockListener) OnRegimeChange(symbol string, from, to MarketRegime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.changes = append(m.changes, struct {
		symbol string
		from   MarketRegime
		to     MarketRegime
	}{symbol, from, to})
}

func (m *mockListener) getChanges() []struct {
	symbol string
	from   MarketRegime
	to     MarketRegime
} {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]struct {
		symbol string
		from   MarketRegime
		to     MarketRegime
	}, len(m.changes))
	copy(result, m.changes)
	return result
}

// TestTransitionHandler_ConfirmedTransition tests that transitions are confirmed after N candles.
func TestTransitionHandler_ConfirmedTransition(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 3
	config.MinRegimeDuration = 0 // Disable for this test
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "BTCUSDT"

	// Initial classification to establish baseline (TRENDING)
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	// First candle detecting change to RANGING - should not confirm yet
	state1 := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	change1, err := handler.ProcessCandle(ctx, "user1", symbol, state1)
	if err != nil {
		t.Fatalf("ProcessCandle failed: %v", err)
	}
	if change1 != nil {
		t.Error("First candle should not confirm transition")
	}
	if !handler.IsPendingTransition(symbol) {
		t.Error("Should have pending transition after first candle")
	}
	pending := handler.GetPendingTransition(symbol)
	if pending.Candles != 1 {
		t.Errorf("Pending candles = %d, want 1", pending.Candles)
	}

	// Second candle confirming - still pending
	state2 := &CoinState{Symbol: symbol, ADX: 16.0, ATR: 100.0, ATRAverage: 100.0}
	change2, err := handler.ProcessCandle(ctx, "user1", symbol, state2)
	if err != nil {
		t.Fatalf("ProcessCandle failed: %v", err)
	}
	if change2 != nil {
		t.Error("Second candle should not confirm transition")
	}
	pending = handler.GetPendingTransition(symbol)
	if pending.Candles != 2 {
		t.Errorf("Pending candles = %d, want 2", pending.Candles)
	}

	// Third candle - should confirm
	state3 := &CoinState{Symbol: symbol, ADX: 17.0, ATR: 100.0, ATRAverage: 100.0}
	change3, err := handler.ProcessCandle(ctx, "user1", symbol, state3)
	if err != nil {
		t.Fatalf("ProcessCandle failed: %v", err)
	}
	if change3 == nil {
		t.Fatal("Third candle should confirm transition")
	}
	if change3.From != RegimeTrending {
		t.Errorf("Change.From = %s, want TRENDING", change3.From)
	}
	if change3.To != RegimeRanging {
		t.Errorf("Change.To = %s, want RANGING", change3.To)
	}
	if handler.IsPendingTransition(symbol) {
		t.Error("Should not have pending transition after confirmation")
	}
}

// TestTransitionHandler_WhipsawPrevention tests that regime reversions cancel pending transitions.
func TestTransitionHandler_WhipsawPrevention(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 3
	config.MinRegimeDuration = 0
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "ETHUSDT"

	// Initial classification to TRENDING
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	// First candle detecting change to RANGING
	state1 := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state1)
	if !handler.IsPendingTransition(symbol) {
		t.Error("Should have pending transition")
	}

	// Second candle - reverts back to TRENDING (whipsaw)
	state2 := &CoinState{Symbol: symbol, ADX: 30.0, ATR: 100.0, ATRAverage: 100.0}
	change, err := handler.ProcessCandle(ctx, "user1", symbol, state2)
	if err != nil {
		t.Fatalf("ProcessCandle failed: %v", err)
	}
	if change != nil {
		t.Error("Whipsaw should not produce a confirmed change")
	}
	if handler.IsPendingTransition(symbol) {
		t.Error("Pending transition should be cancelled on whipsaw")
	}
}

// TestTransitionHandler_MinDuration tests that transitions respect minimum regime duration.
func TestTransitionHandler_MinDuration(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 1  // Immediate confirmation for this test
	config.MinRegimeDuration = time.Hour // One hour minimum
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "SOLUSDT"

	// Initial classification to TRENDING
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)

	// Set regime start time to recent (within MinRegimeDuration)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-30*time.Minute).UnixMilli())

	// Try to transition - should be blocked
	state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	change, err := handler.ProcessCandle(ctx, "user1", symbol, state)
	if err != nil {
		t.Fatalf("ProcessCandle failed: %v", err)
	}
	if change != nil {
		t.Error("Transition should be blocked due to MinRegimeDuration")
	}
	if handler.IsPendingTransition(symbol) {
		t.Error("Should not create pending when blocked by MinRegimeDuration")
	}

	// Now set regime start time to beyond MinRegimeDuration
	handler.SetRegimeStartTime(symbol, time.Now().Add(-2*time.Hour).UnixMilli())

	// Try to transition again - should work
	change, err = handler.ProcessCandle(ctx, "user1", symbol, state)
	if err != nil {
		t.Fatalf("ProcessCandle failed: %v", err)
	}
	if change == nil {
		t.Error("Transition should be allowed after MinRegimeDuration")
	}
}

// TestTransitionHandler_Notifications tests that listeners are notified on confirmed transitions.
func TestTransitionHandler_Notifications(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 1
	config.MinRegimeDuration = 0
	config.EnableNotifications = true
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "ADAUSDT"

	// Add a listener
	listener := newMockListener()
	handler.AddListener(listener)

	// Initial classification
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	// Process a transition
	state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	change, err := handler.ProcessCandle(ctx, "user1", symbol, state)
	if err != nil {
		t.Fatalf("ProcessCandle failed: %v", err)
	}
	if change == nil {
		t.Fatal("Expected confirmed transition")
	}

	// Wait for async notification
	time.Sleep(100 * time.Millisecond)

	// Check listener was notified
	changes := listener.getChanges()
	if len(changes) != 1 {
		t.Fatalf("Listener should have 1 notification, got %d", len(changes))
	}
	if changes[0].symbol != symbol {
		t.Errorf("Notification symbol = %s, want %s", changes[0].symbol, symbol)
	}
	if changes[0].from != RegimeTrending {
		t.Errorf("Notification from = %s, want TRENDING", changes[0].from)
	}
	if changes[0].to != RegimeRanging {
		t.Errorf("Notification to = %s, want RANGING", changes[0].to)
	}
}

// TestTransitionHandler_DisabledNotifications tests that listeners are not notified when disabled.
func TestTransitionHandler_DisabledNotifications(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 1
	config.MinRegimeDuration = 0
	config.EnableNotifications = false // Disabled
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "DOTUSDT"

	// Add a listener
	listener := newMockListener()
	handler.AddListener(listener)

	// Initial classification and transition
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state)

	// Wait briefly
	time.Sleep(100 * time.Millisecond)

	// Listener should NOT be notified
	changes := listener.getChanges()
	if len(changes) != 0 {
		t.Errorf("Listener should not be notified when disabled, got %d notifications", len(changes))
	}
}

// TestTransitionHandler_ConcurrentAccess tests thread safety of the handler.
func TestTransitionHandler_ConcurrentAccess(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 2
	config.MinRegimeDuration = 0
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()

	// Initialize multiple symbols
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "ADAUSDT", "DOTUSDT"}
	for _, symbol := range symbols {
		classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
		handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())
	}

	// Add multiple listeners
	listeners := make([]*mockListener, 3)
	for i := range listeners {
		listeners[i] = newMockListener()
		handler.AddListener(listeners[i])
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	var errors int32

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			symbol := symbols[id%len(symbols)]

			// Alternate between different operations
			switch id % 5 {
			case 0:
				// Process candle with regime change
				state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
				_, err := handler.ProcessCandle(ctx, "user1", symbol, state)
				if err != nil {
					atomic.AddInt32(&errors, 1)
				}
			case 1:
				// Process candle with same regime
				state := &CoinState{Symbol: symbol, ADX: 30.0, ATR: 100.0, ATRAverage: 100.0}
				_, err := handler.ProcessCandle(ctx, "user1", symbol, state)
				if err != nil {
					atomic.AddInt32(&errors, 1)
				}
			case 2:
				// Read pending transitions
				_ = handler.GetPendingTransitions()
				_ = handler.IsPendingTransition(symbol)
			case 3:
				// Check transition log
				_ = handler.GetTransitionLog(symbol)
				_ = handler.GetRecentTransitionCount(symbol)
			case 4:
				// Get/set regime start time
				_ = handler.GetRegimeStartTime(symbol)
				handler.SetRegimeStartTime(symbol, time.Now().UnixMilli())
			}
		}(i)
	}

	wg.Wait()

	if errors > 0 {
		t.Errorf("Concurrent operations had %d errors", errors)
	}
}

// TestTransitionHandler_MultipleListeners tests that multiple listeners all receive notifications.
func TestTransitionHandler_MultipleListeners(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 1
	config.MinRegimeDuration = 0
	config.EnableNotifications = true
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "LINKUSDT"

	// Add multiple listeners
	listener1 := newMockListener()
	listener2 := newMockListener()
	listener3 := newMockListener()
	handler.AddListener(listener1)
	handler.AddListener(listener2)
	handler.AddListener(listener3)

	// Initial classification and transition
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state)

	// Wait for async notifications
	time.Sleep(200 * time.Millisecond)

	// All listeners should be notified
	if len(listener1.getChanges()) != 1 {
		t.Errorf("Listener1 should have 1 notification, got %d", len(listener1.getChanges()))
	}
	if len(listener2.getChanges()) != 1 {
		t.Errorf("Listener2 should have 1 notification, got %d", len(listener2.getChanges()))
	}
	if len(listener3.getChanges()) != 1 {
		t.Errorf("Listener3 should have 1 notification, got %d", len(listener3.getChanges()))
	}
}

// TestTransitionHandler_RemoveListener tests removing a listener.
func TestTransitionHandler_RemoveListener(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 1
	config.MinRegimeDuration = 0
	config.EnableNotifications = true
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "MATICUSDT"

	// Add and then remove a listener
	listener1 := newMockListener()
	listener2 := newMockListener()
	handler.AddListener(listener1)
	handler.AddListener(listener2)
	handler.RemoveListener(listener1)

	// Initial classification and transition
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state)

	// Wait for async notification
	time.Sleep(100 * time.Millisecond)

	// Only listener2 should be notified
	if len(listener1.getChanges()) != 0 {
		t.Errorf("Removed listener should have 0 notifications, got %d", len(listener1.getChanges()))
	}
	if len(listener2.getChanges()) != 1 {
		t.Errorf("Active listener should have 1 notification, got %d", len(listener2.getChanges()))
	}
}

// TestTransitionHandler_TransitionLog tests that transitions are logged for analysis.
func TestTransitionHandler_TransitionLog(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 1
	config.MinRegimeDuration = 0
	config.WhipsawWindow = time.Hour
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "AVAXUSDT"

	// Initial classification
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	// First transition: TRENDING -> RANGING
	state1 := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state1)

	// Verify log entry
	log1 := handler.GetTransitionLog(symbol)
	if len(log1) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(log1))
	}
	if log1[0].From != RegimeTrending || log1[0].To != RegimeRanging {
		t.Errorf("Log entry = %s -> %s, want TRENDING -> RANGING", log1[0].From, log1[0].To)
	}
	if log1[0].Candles != 1 {
		t.Errorf("Log entry candles = %d, want 1", log1[0].Candles)
	}

	// Verify count
	count := handler.GetRecentTransitionCount(symbol)
	if count != 1 {
		t.Errorf("Recent transition count = %d, want 1", count)
	}
}

// TestTransitionHandler_WhipsawDetection tests the whipsaw detection feature.
func TestTransitionHandler_WhipsawDetection(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 1
	config.MinRegimeDuration = 0
	config.WhipsawWindow = time.Hour
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "NEARUSDT"

	// Create multiple transitions to trigger whipsaw detection
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0) // TRENDING
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	// Transition 1: TRENDING -> RANGING
	state1 := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state1)

	// Transition 2: RANGING -> TRENDING
	state2 := &CoinState{Symbol: symbol, ADX: 30.0, ATR: 100.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state2)

	// Transition 3: TRENDING -> VOLATILE
	state3 := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 200.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state3)

	// Check whipsaw detection with threshold of 3
	if !handler.IsWhipsawDetected(symbol, 3) {
		t.Error("Should detect whipsaw with threshold 3 after 3 transitions")
	}

	// Check whipsaw detection with higher threshold
	if handler.IsWhipsawDetected(symbol, 5) {
		t.Error("Should not detect whipsaw with threshold 5 after only 3 transitions")
	}
}

// TestTransitionHandler_ClearSymbol tests clearing data for a specific symbol.
func TestTransitionHandler_ClearSymbol(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 2
	config.MinRegimeDuration = 0
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "ATOMUSDT"

	// Set up data
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	// Create a pending transition
	state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state)

	// Verify data exists
	if !handler.IsPendingTransition(symbol) {
		t.Error("Should have pending transition")
	}
	if handler.GetRegimeStartTime(symbol) == 0 {
		t.Error("Should have regime start time")
	}

	// Clear the symbol
	handler.ClearSymbol(symbol)

	// Verify data is cleared
	if handler.IsPendingTransition(symbol) {
		t.Error("Pending transition should be cleared")
	}
	if handler.GetRegimeStartTime(symbol) != 0 {
		t.Error("Regime start time should be cleared")
	}
	if len(handler.GetTransitionLog(symbol)) != 0 {
		t.Error("Transition log should be cleared")
	}
}

// TestTransitionHandler_ClearAll tests clearing all data.
func TestTransitionHandler_ClearAll(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 2
	config.MinRegimeDuration = 0
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()

	// Set up data for multiple symbols
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	for _, symbol := range symbols {
		classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
		handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())
		state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
		handler.ProcessCandle(ctx, "user1", symbol, state)
	}

	// Verify data exists
	pending := handler.GetPendingTransitions()
	if len(pending) != 3 {
		t.Errorf("Should have 3 pending transitions, got %d", len(pending))
	}

	// Clear all
	handler.ClearAll()

	// Verify all data is cleared
	pending = handler.GetPendingTransitions()
	if len(pending) != 0 {
		t.Errorf("Should have 0 pending transitions after clear, got %d", len(pending))
	}
	for _, symbol := range symbols {
		if handler.GetRegimeStartTime(symbol) != 0 {
			t.Errorf("Regime start time for %s should be 0", symbol)
		}
	}
}

// TestTransitionHandler_DirectionChange tests changing direction mid-confirmation.
func TestTransitionHandler_DirectionChange(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 3
	config.MinRegimeDuration = 0
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "XLMUSDT"

	// Initial classification to TRENDING
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	// First candle: TRENDING -> RANGING (pending)
	state1 := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state1)
	pending1 := handler.GetPendingTransition(symbol)
	if pending1.To != RegimeRanging {
		t.Errorf("Pending.To = %s, want RANGING", pending1.To)
	}

	// Second candle: Switches to VOLATILE instead (different direction)
	state2 := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 200.0, ATRAverage: 100.0}
	handler.ProcessCandle(ctx, "user1", symbol, state2)

	// Should have new pending for VOLATILE, not RANGING
	pending2 := handler.GetPendingTransition(symbol)
	if pending2 == nil {
		t.Fatal("Should have pending transition")
	}
	if pending2.To != RegimeVolatile {
		t.Errorf("Pending.To = %s, want VOLATILE", pending2.To)
	}
	if pending2.Candles != 1 {
		t.Errorf("Pending.Candles = %d, want 1 (reset)", pending2.Candles)
	}
}

// TestTransitionHandler_NilState tests error handling for nil state.
func TestTransitionHandler_NilState(t *testing.T) {
	classifier := NewRegimeClassifier(nil, nil)
	handler := NewTransitionHandler(classifier, nil)
	ctx := context.Background()

	_, err := handler.ProcessCandle(ctx, "user1", "BTCUSDT", nil)
	if err == nil {
		t.Error("ProcessCandle should return error for nil state")
	}
}

// TestTransitionHandler_NilClassifier tests error handling for nil classifier.
func TestTransitionHandler_NilClassifier(t *testing.T) {
	handler := NewTransitionHandler(nil, nil)
	ctx := context.Background()

	state := &CoinState{Symbol: "BTCUSDT", ADX: 30.0}
	_, err := handler.ProcessCandle(ctx, "user1", "BTCUSDT", state)
	if err == nil {
		t.Error("ProcessCandle should return error for nil classifier")
	}
}

// TestDefaultTransitionConfig tests default configuration values.
func TestDefaultTransitionConfig(t *testing.T) {
	config := DefaultTransitionConfig()

	if config.ConfirmationCandles != 3 {
		t.Errorf("ConfirmationCandles = %d, want 3", config.ConfirmationCandles)
	}
	if config.MinRegimeDuration != 15*time.Minute {
		t.Errorf("MinRegimeDuration = %v, want 15m", config.MinRegimeDuration)
	}
	if config.WhipsawWindow != 30*time.Minute {
		t.Errorf("WhipsawWindow = %v, want 30m", config.WhipsawWindow)
	}
	if !config.EnableNotifications {
		t.Error("EnableNotifications should be true by default")
	}
	if !config.EnableLogging {
		t.Error("EnableLogging should be true by default")
	}
}

// TestNewTransitionHandler_NilConfig tests that nil config uses defaults.
func TestNewTransitionHandler_NilConfig(t *testing.T) {
	classifier := NewRegimeClassifier(nil, nil)
	handler := NewTransitionHandler(classifier, nil)

	if handler.config == nil {
		t.Fatal("Config should not be nil")
	}
	if handler.config.ConfirmationCandles != 3 {
		t.Errorf("Should use default config, ConfirmationCandles = %d", handler.config.ConfirmationCandles)
	}
}

// TestTransitionHandler_GetConfig tests the GetConfig method.
func TestTransitionHandler_GetConfig(t *testing.T) {
	classifier := NewRegimeClassifier(nil, nil)
	config := &TransitionConfig{
		ConfirmationCandles: 5,
	}
	handler := NewTransitionHandler(classifier, config)

	returnedConfig := handler.GetConfig()
	if returnedConfig.ConfirmationCandles != 5 {
		t.Errorf("GetConfig returned wrong value: %d", returnedConfig.ConfirmationCandles)
	}

	// Verify that modifying the returned config doesn't affect the handler
	returnedConfig.ConfirmationCandles = 10
	returnedConfig2 := handler.GetConfig()
	if returnedConfig2.ConfirmationCandles != 5 {
		t.Errorf("GetConfig should return a copy, but internal config was modified to: %d", returnedConfig2.ConfirmationCandles)
	}
}

// TestTransitionHandler_GetClassifier tests the GetClassifier method.
func TestTransitionHandler_GetClassifier(t *testing.T) {
	classifier := NewRegimeClassifier(nil, nil)
	handler := NewTransitionHandler(classifier, nil)

	if handler.GetClassifier() != classifier {
		t.Error("GetClassifier should return the same classifier")
	}
}

// TestTransitionHandler_SingleCandleConfirmation tests immediate confirmation when ConfirmationCandles=1.
func TestTransitionHandler_SingleCandleConfirmation(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 1 // Immediate confirmation
	config.MinRegimeDuration = 0
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "FTMUSDT"

	// Initial classification
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	// Single candle should confirm immediately
	state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	change, err := handler.ProcessCandle(ctx, "user1", symbol, state)
	if err != nil {
		t.Fatalf("ProcessCandle failed: %v", err)
	}
	if change == nil {
		t.Error("Single candle should confirm immediately when ConfirmationCandles=1")
	}
	if handler.IsPendingTransition(symbol) {
		t.Error("Should not have pending transition after immediate confirmation")
	}
}

// TestTransitionHandler_NilListener tests that nil listener is safely ignored.
func TestTransitionHandler_NilListener(t *testing.T) {
	classifier := NewRegimeClassifier(nil, nil)
	handler := NewTransitionHandler(classifier, nil)

	// Should not panic
	handler.AddListener(nil)
	handler.RemoveListener(nil)
}

// TestTransitionHandler_PanicRecovery tests that listener panics don't crash the handler.
func TestTransitionHandler_PanicRecovery(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 1
	config.MinRegimeDuration = 0
	config.EnableNotifications = true
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()
	symbol := "ALGOUSDT"

	// Add a listener that panics
	panicListener := &panicingListener{}
	handler.AddListener(panicListener)

	// Add a normal listener
	normalListener := newMockListener()
	handler.AddListener(normalListener)

	// Initial classification and transition
	classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
	handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())

	// Process - should not panic
	state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
	change, err := handler.ProcessCandle(ctx, "user1", symbol, state)
	if err != nil {
		t.Fatalf("ProcessCandle failed: %v", err)
	}
	if change == nil {
		t.Error("Expected confirmed transition")
	}

	// Wait for notifications
	time.Sleep(200 * time.Millisecond)

	// Normal listener should still be notified
	if len(normalListener.getChanges()) != 1 {
		t.Errorf("Normal listener should have 1 notification despite panic, got %d", len(normalListener.getChanges()))
	}
}

// panicingListener is a test helper that panics on notification.
type panicingListener struct{}

func (p *panicingListener) OnRegimeChange(symbol string, from, to MarketRegime) {
	panic("intentional panic for testing")
}

// TestTransitionHandler_ZeroConfirmationCandles tests that zero ConfirmationCandles is normalized to 1.
func TestTransitionHandler_ZeroConfirmationCandles(t *testing.T) {
	classifier := NewRegimeClassifier(nil, nil)
	config := &TransitionConfig{
		ConfirmationCandles: 0, // Invalid value
	}
	handler := NewTransitionHandler(classifier, config)

	// Should be normalized to 1
	returnedConfig := handler.GetConfig()
	if returnedConfig.ConfirmationCandles != 1 {
		t.Errorf("Zero ConfirmationCandles should be normalized to 1, got %d", returnedConfig.ConfirmationCandles)
	}
}

// TestTransitionHandler_NegativeConfirmationCandles tests that negative ConfirmationCandles is normalized to 1.
func TestTransitionHandler_NegativeConfirmationCandles(t *testing.T) {
	classifier := NewRegimeClassifier(nil, nil)
	config := &TransitionConfig{
		ConfirmationCandles: -5, // Invalid value
	}
	handler := NewTransitionHandler(classifier, config)

	// Should be normalized to 1
	returnedConfig := handler.GetConfig()
	if returnedConfig.ConfirmationCandles != 1 {
		t.Errorf("Negative ConfirmationCandles should be normalized to 1, got %d", returnedConfig.ConfirmationCandles)
	}
}

// TestTransitionHandler_ContextCancellation tests that context cancellation is respected.
func TestTransitionHandler_ContextCancellation(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	classifier := NewRegimeClassifier(regimeConfig, nil)
	config := DefaultTransitionConfig()
	handler := NewTransitionHandler(classifier, config)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := &CoinState{Symbol: "BTCUSDT", ADX: 30.0}
	_, err := handler.ProcessCandle(ctx, "user1", "BTCUSDT", state)
	if err == nil {
		t.Error("ProcessCandle should return error for cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// TestTransitionHandler_CancelAllPendingTransitions tests cancelling all pending transitions.
func TestTransitionHandler_CancelAllPendingTransitions(t *testing.T) {
	regimeConfig := DefaultRegimeConfig()
	regimeConfig.MinRegimeDuration = 1
	classifier := NewRegimeClassifier(regimeConfig, nil)

	config := DefaultTransitionConfig()
	config.ConfirmationCandles = 3
	config.MinRegimeDuration = 0
	config.EnableLogging = false

	handler := NewTransitionHandler(classifier, config)
	ctx := context.Background()

	// Create pending transitions for multiple symbols
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	for _, symbol := range symbols {
		classifier.ClassifyAndRecord(symbol, 30.0, 0, 0)
		handler.SetRegimeStartTime(symbol, time.Now().Add(-time.Hour).UnixMilli())
		state := &CoinState{Symbol: symbol, ADX: 15.0, ATR: 100.0, ATRAverage: 100.0}
		handler.ProcessCandle(ctx, "user1", symbol, state)
	}

	// Verify pending transitions exist
	pending := handler.GetPendingTransitions()
	if len(pending) != 3 {
		t.Errorf("Should have 3 pending transitions, got %d", len(pending))
	}

	// Cancel all
	handler.CancelAllPendingTransitions()

	// Verify all cleared
	pending = handler.GetPendingTransitions()
	if len(pending) != 0 {
		t.Errorf("Should have 0 pending transitions after cancel, got %d", len(pending))
	}
}
