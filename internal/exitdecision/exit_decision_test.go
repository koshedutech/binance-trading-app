package exitdecision

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// MOCK IMPLEMENTATIONS
// ============================================================================

// MockPriceProvider implements PriceProvider for testing.
type MockPriceProvider struct {
	prices map[string]float64
	mu     sync.RWMutex
	err    error
}

func NewMockPriceProvider() *MockPriceProvider {
	return &MockPriceProvider{
		prices: make(map[string]float64),
	}
}

func (m *MockPriceProvider) SetPrice(symbol string, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prices[symbol] = price
}

func (m *MockPriceProvider) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockPriceProvider) GetPrice(ctx context.Context, symbol string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return 0, m.err
	}
	return m.prices[symbol], nil
}

func (m *MockPriceProvider) GetPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	result := make(map[string]float64)
	for _, s := range symbols {
		if price, ok := m.prices[s]; ok {
			result[s] = price
		}
	}
	return result, nil
}

// MockPositionProvider implements PositionProvider for testing.
type MockPositionProvider struct {
	positions map[string][]Position
	mu        sync.RWMutex
}

func NewMockPositionProvider() *MockPositionProvider {
	return &MockPositionProvider{
		positions: make(map[string][]Position),
	}
}

func (m *MockPositionProvider) AddPosition(userID string, pos Position) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.positions[userID] = append(m.positions[userID], pos)
}

func (m *MockPositionProvider) ClearPositions(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.positions[userID] = nil
}

func (m *MockPositionProvider) GetOpenPositions(ctx context.Context, userID string) ([]Position, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// If userID is empty, return all positions
	if userID == "" {
		var all []Position
		for _, positions := range m.positions {
			all = append(all, positions...)
		}
		return all, nil
	}
	return m.positions[userID], nil
}

func (m *MockPositionProvider) GetPosition(ctx context.Context, userID, symbol string) (Position, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, pos := range m.positions[userID] {
		if pos.GetSymbol() == symbol {
			return pos, nil
		}
	}
	return nil, nil
}

// ============================================================================
// SERVICE TESTS
// ============================================================================

func TestNewService(t *testing.T) {
	priceProvider := NewMockPriceProvider()
	positionProvider := NewMockPositionProvider()

	service := NewService(nil, priceProvider, positionProvider)

	if service == nil {
		t.Fatal("NewService returned nil")
	}

	if service.config == nil {
		t.Error("Service config is nil")
	}

	if service.monitor == nil {
		t.Error("Service monitor is nil")
	}

	if service.IsRunning() {
		t.Error("Service should not be running initially")
	}
}

func TestServiceStartStop(t *testing.T) {
	priceProvider := NewMockPriceProvider()
	positionProvider := NewMockPositionProvider()
	config := DefaultConfig()
	config.CheckInterval = 100 * time.Millisecond

	service := NewService(config, priceProvider, positionProvider)

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	if !service.IsRunning() {
		t.Error("Service should be running after Start")
	}

	// Try to start again - should fail
	err = service.Start(ctx)
	if err == nil {
		t.Error("Starting already running service should return error")
	}

	// Stop service
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	if service.IsRunning() {
		t.Error("Service should not be running after Stop")
	}

	// Try to stop again - should fail
	err = service.Stop()
	if err == nil {
		t.Error("Stopping already stopped service should return error")
	}
}

func TestGetStatus(t *testing.T) {
	priceProvider := NewMockPriceProvider()
	positionProvider := NewMockPositionProvider()
	config := DefaultConfig()

	service := NewService(config, priceProvider, positionProvider)

	status := service.GetStatus()
	if status.Running {
		t.Error("Status should show not running")
	}

	ctx := context.Background()
	_ = service.Start(ctx)
	defer service.Stop()

	status = service.GetStatus()
	if !status.Running {
		t.Error("Status should show running")
	}

	if status.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}

	if !status.PriceProviderHealthy {
		t.Error("PriceProviderHealthy should be true")
	}
}

func TestCheckPosition(t *testing.T) {
	priceProvider := NewMockPriceProvider()
	positionProvider := NewMockPositionProvider()

	service := NewService(nil, priceProvider, positionProvider)

	// Test with nil position
	ctx := context.Background()
	signal, err := service.CheckPosition(ctx, nil)
	if err == nil {
		t.Error("CheckPosition with nil position should return error")
	}
	if signal != nil {
		t.Error("Signal should be nil for nil position")
	}

	// Test with valid position
	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		slPrice:    49000.0,
		tpLevels:   []TakeProfitLevel{{Level: 1, Price: 51000.0, Status: "pending"}},
	}

	priceProvider.SetPrice("BTCUSDT", 50500.0)

	signal, err = service.CheckPosition(ctx, pos)
	if err != nil {
		t.Errorf("CheckPosition returned error: %v", err)
	}
	// No signal expected as price is between SL and TP
	if signal != nil && signal.Urgency == UrgencyImmediate {
		t.Errorf("Unexpected immediate signal: %+v", signal)
	}
}

func TestGetExitSignals(t *testing.T) {
	priceProvider := NewMockPriceProvider()
	positionProvider := NewMockPositionProvider()

	service := NewService(nil, priceProvider, positionProvider)

	ctx := context.Background()

	// Initially no signals
	signals, err := service.GetExitSignals(ctx, "user1")
	if err != nil {
		t.Errorf("GetExitSignals returned error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("Expected 0 signals, got %d", len(signals))
	}

	// Add a signal manually
	service.mu.Lock()
	service.pendingSignals["user1"] = []ExitSignal{
		{
			Symbol:   "BTCUSDT",
			ExitType: ExitTypeSL,
			Urgency:  UrgencyImmediate,
		},
	}
	service.mu.Unlock()

	signals, err = service.GetExitSignals(ctx, "user1")
	if err != nil {
		t.Errorf("GetExitSignals returned error: %v", err)
	}
	if len(signals) != 1 {
		t.Errorf("Expected 1 signal, got %d", len(signals))
	}

	// Immediate signals should be cleared after retrieval
	signals, err = service.GetExitSignals(ctx, "user1")
	if err != nil {
		t.Errorf("GetExitSignals returned error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("Expected 0 signals after retrieval, got %d", len(signals))
	}
}

func TestMonitoringCycle(t *testing.T) {
	priceProvider := NewMockPriceProvider()
	positionProvider := NewMockPositionProvider()
	config := DefaultConfig()
	config.CheckInterval = 50 * time.Millisecond
	config.DebugMode = true

	service := NewService(config, priceProvider, positionProvider)

	// Add a position that should trigger SL
	pos := &MockPosition{
		symbol:     "BTCUSDT",
		mode:       "scalp",
		side:       "LONG",
		entryPrice: 50000.0,
		slPrice:    49000.0,
		tpLevels:   []TakeProfitLevel{{Level: 1, Price: 51000.0, Status: "pending"}},
	}
	positionProvider.AddPosition("", pos)

	// Set price below SL to trigger
	priceProvider.SetPrice("BTCUSDT", 48900.0)

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for monitoring cycle
	time.Sleep(200 * time.Millisecond)

	service.Stop()

	// Check that signal was generated
	service.mu.RLock()
	signalCount := service.signalsTotal
	service.mu.RUnlock()

	if signalCount == 0 {
		t.Error("Expected at least one signal to be generated")
	}
}

func TestSetProviders(t *testing.T) {
	service := NewService(nil, nil, nil)

	priceProvider := NewMockPriceProvider()
	positionProvider := NewMockPositionProvider()

	service.SetPriceProvider(priceProvider)
	service.SetPositionProvider(positionProvider)

	status := service.GetStatus()
	if !status.PriceProviderHealthy {
		t.Error("PriceProviderHealthy should be true after setting provider")
	}
}

func TestSetConfig(t *testing.T) {
	service := NewService(nil, nil, nil)

	newConfig := &Config{
		CheckInterval:                  200 * time.Millisecond,
		EnableTPMonitoring:             false,
		DefaultEfficiencyThreshold:     0.7,
	}

	service.SetConfig(newConfig)

	service.mu.RLock()
	configInterval := service.config.CheckInterval
	tpEnabled := service.config.EnableTPMonitoring
	service.mu.RUnlock()

	if configInterval != 200*time.Millisecond {
		t.Errorf("Config interval not updated: got %v", configInterval)
	}

	if tpEnabled {
		t.Error("TP monitoring should be disabled")
	}

	// Test nil config
	service.SetConfig(nil)
	service.mu.RLock()
	afterNil := service.config.CheckInterval
	service.mu.RUnlock()

	if afterNil != 200*time.Millisecond {
		t.Error("Config should not change when setting nil")
	}
}

func TestSubscribeToSignals(t *testing.T) {
	service := NewService(nil, nil, nil)

	var received ExitSignal
	var wg sync.WaitGroup
	wg.Add(1)

	service.SubscribeToSignals(func(signal ExitSignal) {
		received = signal
		wg.Done()
	})

	// Notify subscribers
	testSignal := ExitSignal{
		Symbol:   "ETHUSDT",
		ExitType: ExitTypeTP,
		TPLevel:  1,
	}

	notifySubscribers(testSignal)

	// Wait for notification
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if received.Symbol != "ETHUSDT" {
			t.Errorf("Received wrong signal: %+v", received)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for signal notification")
	}
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestServiceWithMultiplePositions(t *testing.T) {
	priceProvider := NewMockPriceProvider()
	positionProvider := NewMockPositionProvider()
	config := DefaultConfig()
	config.CheckInterval = 50 * time.Millisecond

	service := NewService(config, priceProvider, positionProvider)

	// Add multiple positions
	positions := []*MockPosition{
		{symbol: "BTCUSDT", mode: "scalp", side: "LONG", entryPrice: 50000.0, slPrice: 49000.0},
		{symbol: "ETHUSDT", mode: "swing", side: "SHORT", entryPrice: 3000.0, slPrice: 3200.0},
		{symbol: "SOLUSDT", mode: "position", side: "LONG", entryPrice: 100.0, slPrice: 95.0},
	}

	for _, pos := range positions {
		positionProvider.AddPosition("", pos)
	}

	// Set prices - ETHUSDT should trigger SL (above entry for short)
	priceProvider.SetPrice("BTCUSDT", 50500.0) // In profit
	priceProvider.SetPrice("ETHUSDT", 3250.0)  // Above SL, should trigger
	priceProvider.SetPrice("SOLUSDT", 102.0)   // In profit

	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for monitoring cycles
	time.Sleep(200 * time.Millisecond)

	service.Stop()

	// Check status
	status := service.GetStatus()
	if status.PositionsMonitored != 3 {
		t.Errorf("Expected 3 positions monitored, got %d", status.PositionsMonitored)
	}

	if status.SignalsGenerated == 0 {
		t.Error("Expected at least one signal to be generated")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.CheckInterval != 500*time.Millisecond {
		t.Errorf("Wrong default CheckInterval: %v", config.CheckInterval)
	}

	if !config.EnableTPMonitoring {
		t.Error("TP monitoring should be enabled by default")
	}

	if !config.EnableSLMonitoring {
		t.Error("SL monitoring should be enabled by default")
	}

	if !config.EnableTrailingMonitoring {
		t.Error("Trailing monitoring should be enabled by default")
	}

	if !config.EnableEfficiencyMonitoring {
		t.Error("Efficiency monitoring should be enabled by default")
	}

	if config.DefaultEfficiencyThreshold != 0.5 {
		t.Errorf("Wrong default efficiency threshold: %v", config.DefaultEfficiencyThreshold)
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{3665 * time.Second, "1h 1m 5s"},
		{90061 * time.Second, "1d 1h 1m 1s"},
	}

	for _, tt := range tests {
		result := formatUptime(tt.duration)
		if result != tt.expected {
			t.Errorf("formatUptime(%v) = %s, expected %s", tt.duration, result, tt.expected)
		}
	}
}
