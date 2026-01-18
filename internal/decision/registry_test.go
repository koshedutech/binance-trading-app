// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.6: Strategy Registry - Tests
package decision

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewStrategyRegistry tests registry creation
func TestNewStrategyRegistry(t *testing.T) {
	registry := NewStrategyRegistry()

	if registry == nil {
		t.Fatal("NewStrategyRegistry should not return nil")
	}

	if registry.Count() != 0 {
		t.Errorf("New registry should be empty, got count %d", registry.Count())
	}

	if registry.GetDefaultStrategyName() != "" {
		t.Error("New registry should have no default strategy")
	}
}

// TestRegisterStrategy tests strategy registration
func TestRegisterStrategy(t *testing.T) {
	registry := NewStrategyRegistry()
	strategy := NewMockStrategy("trend_following", []MarketRegime{RegimeTrending})

	err := registry.RegisterStrategy(strategy)
	if err != nil {
		t.Fatalf("RegisterStrategy failed: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Count = %d, expected 1", registry.Count())
	}

	if !registry.HasStrategy("trend_following") {
		t.Error("Registry should have trend_following strategy")
	}

	// First registered strategy should become default
	if registry.GetDefaultStrategyName() != "trend_following" {
		t.Errorf("Default strategy = %s, expected trend_following", registry.GetDefaultStrategyName())
	}
}

// TestRegisterStrategyDuplicate tests duplicate registration
func TestRegisterStrategyDuplicate(t *testing.T) {
	registry := NewStrategyRegistry()
	strategy := NewMockStrategy("trend_following", []MarketRegime{RegimeTrending})

	err := registry.RegisterStrategy(strategy)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	err = registry.RegisterStrategy(strategy)
	if err != ErrStrategyAlreadyExists {
		t.Errorf("Expected ErrStrategyAlreadyExists, got %v", err)
	}
}

// TestRegisterStrategyNil tests nil strategy registration
func TestRegisterStrategyNil(t *testing.T) {
	registry := NewStrategyRegistry()

	err := registry.RegisterStrategy(nil)
	if err != ErrStrategyNil {
		t.Errorf("Expected ErrStrategyNil, got %v", err)
	}
}

// TestRegisterStrategyEmptyName tests empty name registration
func TestRegisterStrategyEmptyName(t *testing.T) {
	registry := NewStrategyRegistry()
	strategy := NewMockStrategy("", []MarketRegime{RegimeTrending})

	err := registry.RegisterStrategy(strategy)
	if err == nil {
		t.Error("Expected error for empty strategy name")
	}
}

// TestUnregisterStrategy tests strategy unregistration
func TestUnregisterStrategy(t *testing.T) {
	registry := NewStrategyRegistry()
	strategy := NewMockStrategy("trend_following", []MarketRegime{RegimeTrending})

	registry.RegisterStrategy(strategy)

	err := registry.UnregisterStrategy("trend_following")
	if err != nil {
		t.Fatalf("UnregisterStrategy failed: %v", err)
	}

	if registry.Count() != 0 {
		t.Errorf("Count = %d, expected 0", registry.Count())
	}

	if registry.HasStrategy("trend_following") {
		t.Error("Registry should not have trend_following after unregistration")
	}
}

// TestUnregisterStrategyNotFound tests unregistering non-existent strategy
func TestUnregisterStrategyNotFound(t *testing.T) {
	registry := NewStrategyRegistry()

	err := registry.UnregisterStrategy("nonexistent")
	if err != ErrStrategyNotFound {
		t.Errorf("Expected ErrStrategyNotFound, got %v", err)
	}
}

// TestGetStrategy tests strategy retrieval
func TestGetStrategy(t *testing.T) {
	registry := NewStrategyRegistry()
	strategy := NewMockStrategy("trend_following", []MarketRegime{RegimeTrending})

	registry.RegisterStrategy(strategy)

	retrieved, err := registry.GetStrategy("trend_following")
	if err != nil {
		t.Fatalf("GetStrategy failed: %v", err)
	}

	if retrieved.Name() != "trend_following" {
		t.Errorf("Retrieved strategy name = %s, expected trend_following", retrieved.Name())
	}
}

// TestGetStrategyNotFound tests retrieval of non-existent strategy
func TestGetStrategyNotFound(t *testing.T) {
	registry := NewStrategyRegistry()

	_, err := registry.GetStrategy("nonexistent")
	if err != ErrStrategyNotFound {
		t.Errorf("Expected ErrStrategyNotFound, got %v", err)
	}
}

// TestGetStrategyInfo tests strategy info retrieval
func TestGetStrategyInfo(t *testing.T) {
	registry := NewStrategyRegistry()
	strategy := NewMockStrategy("trend_following", []MarketRegime{RegimeTrending, RegimeVolatile})

	registry.RegisterStrategy(strategy)

	info, err := registry.GetStrategyInfo("trend_following")
	if err != nil {
		t.Fatalf("GetStrategyInfo failed: %v", err)
	}

	if info.Name != "trend_following" {
		t.Errorf("Info.Name = %s, expected trend_following", info.Name)
	}

	if len(info.SupportedRegimes) != 2 {
		t.Errorf("SupportedRegimes length = %d, expected 2", len(info.SupportedRegimes))
	}

	if !info.Enabled {
		t.Error("Strategy should be enabled by default")
	}
}

// TestGetStrategiesForRegime tests regime-based lookup
func TestGetStrategiesForRegime(t *testing.T) {
	registry := NewStrategyRegistry()

	// Register strategies with different regimes
	trendStrategy := NewMockStrategy("trend_following", []MarketRegime{RegimeTrending})
	rangeStrategy := NewMockStrategy("range_trading", []MarketRegime{RegimeRanging, RegimeConsolidating})
	multiStrategy := NewMockStrategy("multi_regime", []MarketRegime{RegimeTrending, RegimeVolatile})

	registry.RegisterStrategy(trendStrategy)
	registry.RegisterStrategy(rangeStrategy)
	registry.RegisterStrategy(multiStrategy)

	// Test TRENDING regime
	trending := registry.GetStrategiesForRegime(RegimeTrending)
	if len(trending) != 2 {
		t.Errorf("TRENDING strategies = %d, expected 2", len(trending))
	}

	// Test RANGING regime
	ranging := registry.GetStrategiesForRegime(RegimeRanging)
	if len(ranging) != 1 {
		t.Errorf("RANGING strategies = %d, expected 1", len(ranging))
	}

	// Test VOLATILE regime
	volatile := registry.GetStrategiesForRegime(RegimeVolatile)
	if len(volatile) != 1 {
		t.Errorf("VOLATILE strategies = %d, expected 1", len(volatile))
	}

	// Test regime with no strategies
	names := registry.GetStrategyNamesForRegime(MarketRegime("UNKNOWN"))
	if len(names) != 0 {
		t.Errorf("Unknown regime should return empty slice, got %d strategies", len(names))
	}
}

// TestListStrategies tests listing all strategies
func TestListStrategies(t *testing.T) {
	registry := NewStrategyRegistry()

	// Empty registry
	list := registry.ListStrategies()
	if len(list) != 0 {
		t.Errorf("Empty registry should return empty list, got %d", len(list))
	}

	// Add strategies
	registry.RegisterStrategy(NewMockStrategy("strategy1", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("strategy2", []MarketRegime{RegimeRanging}))
	registry.RegisterStrategy(NewMockStrategy("strategy3", []MarketRegime{RegimeVolatile}))

	list = registry.ListStrategies()
	if len(list) != 3 {
		t.Errorf("List length = %d, expected 3", len(list))
	}

	names := registry.ListStrategyNames()
	if len(names) != 3 {
		t.Errorf("Names length = %d, expected 3", len(names))
	}
}

// TestDefaultStrategy tests default strategy management
func TestDefaultStrategy(t *testing.T) {
	registry := NewStrategyRegistry()

	// No default initially
	_, err := registry.GetDefaultStrategy()
	if err == nil {
		t.Error("Expected error when getting default from empty registry")
	}

	// First strategy becomes default
	registry.RegisterStrategy(NewMockStrategy("first", []MarketRegime{RegimeTrending}))
	if registry.GetDefaultStrategyName() != "first" {
		t.Error("First strategy should become default")
	}

	// Second strategy does not change default
	registry.RegisterStrategy(NewMockStrategy("second", []MarketRegime{RegimeRanging}))
	if registry.GetDefaultStrategyName() != "first" {
		t.Error("Second strategy should not change default")
	}

	// Explicitly set default
	err = registry.SetDefaultStrategy("second")
	if err != nil {
		t.Fatalf("SetDefaultStrategy failed: %v", err)
	}
	if registry.GetDefaultStrategyName() != "second" {
		t.Error("Default should be 'second' after SetDefaultStrategy")
	}

	// Try to set non-existent strategy as default
	err = registry.SetDefaultStrategy("nonexistent")
	if err != ErrStrategyNotFound {
		t.Errorf("Expected ErrStrategyNotFound, got %v", err)
	}
}

// TestDefaultStrategyAfterUnregister tests default update after unregistration
func TestDefaultStrategyAfterUnregister(t *testing.T) {
	registry := NewStrategyRegistry()

	registry.RegisterStrategy(NewMockStrategy("first", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("second", []MarketRegime{RegimeRanging}))

	// Unregister default
	registry.UnregisterStrategy("first")

	// Default should update to remaining strategy
	defaultName := registry.GetDefaultStrategyName()
	if defaultName != "second" {
		t.Errorf("Default should update to 'second' after unregistering 'first', got '%s'", defaultName)
	}
}

// TestEnableDisableStrategy tests enabling/disabling strategies
func TestEnableDisableStrategy(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("test", []MarketRegime{RegimeTrending}))

	// Initially enabled
	if !registry.IsEnabled("test") {
		t.Error("Strategy should be enabled by default")
	}

	// Disable
	err := registry.DisableStrategy("test")
	if err != nil {
		t.Fatalf("DisableStrategy failed: %v", err)
	}
	if registry.IsEnabled("test") {
		t.Error("Strategy should be disabled")
	}

	// Re-enable
	err = registry.EnableStrategy("test")
	if err != nil {
		t.Fatalf("EnableStrategy failed: %v", err)
	}
	if !registry.IsEnabled("test") {
		t.Error("Strategy should be re-enabled")
	}

	// Test non-existent strategy
	err = registry.DisableStrategy("nonexistent")
	if err != ErrStrategyNotFound {
		t.Errorf("Expected ErrStrategyNotFound, got %v", err)
	}
}

// TestGetEnabledStrategies tests filtered enabled strategy retrieval
func TestGetEnabledStrategies(t *testing.T) {
	registry := NewStrategyRegistry()

	registry.RegisterStrategy(NewMockStrategy("enabled1", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("enabled2", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("disabled", []MarketRegime{RegimeTrending}))

	registry.DisableStrategy("disabled")

	enabled := registry.GetEnabledStrategies()
	if len(enabled) != 2 {
		t.Errorf("Enabled strategies = %d, expected 2", len(enabled))
	}

	enabledForRegime := registry.GetEnabledStrategiesForRegime(RegimeTrending)
	if len(enabledForRegime) != 2 {
		t.Errorf("Enabled strategies for regime = %d, expected 2", len(enabledForRegime))
	}
}

// TestSelectBestStrategy tests best strategy selection
func TestSelectBestStrategy(t *testing.T) {
	registry := NewStrategyRegistry()

	// Create strategies with different scores
	lowScore := NewMockStrategy("low", []MarketRegime{RegimeTrending})
	lowScore.SetScore(50.0)

	highScore := NewMockStrategy("high", []MarketRegime{RegimeTrending})
	highScore.SetScore(85.0)

	registry.RegisterStrategy(lowScore)
	registry.RegisterStrategy(highScore)

	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	name, score, err := registry.SelectBestStrategy(state, RegimeTrending)
	if err != nil {
		t.Fatalf("SelectBestStrategy failed: %v", err)
	}

	if name != "high" {
		t.Errorf("Best strategy = %s, expected 'high'", name)
	}

	if score.FinalScore != 85.0 {
		t.Errorf("Best score = %f, expected 85.0", score.FinalScore)
	}
}

// TestSelectBestStrategyNoMatch tests fallback to default
func TestSelectBestStrategyNoMatch(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("default", []MarketRegime{RegimeTrending}))

	state := NewCoinState("BTCUSDT")

	// Request regime with no strategies - should fall back to default
	name, _, err := registry.SelectBestStrategy(state, RegimeRanging)
	if err != nil {
		t.Fatalf("SelectBestStrategy should fall back to default: %v", err)
	}

	if name != "default" {
		t.Errorf("Should fall back to default strategy, got %s", name)
	}
}

// TestSelectBestStrategyNoStrategies tests error when no strategies available
func TestSelectBestStrategyNoStrategies(t *testing.T) {
	registry := NewStrategyRegistry()
	state := NewCoinState("BTCUSDT")

	_, _, err := registry.SelectBestStrategy(state, RegimeTrending)
	if err != ErrNoStrategiesForRegime {
		t.Errorf("Expected ErrNoStrategiesForRegime, got %v", err)
	}
}

// TestSelectBestStrategyNilState tests error when nil state is passed
func TestSelectBestStrategyNilState(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("test", []MarketRegime{RegimeTrending}))

	_, _, err := registry.SelectBestStrategy(nil, RegimeTrending)
	if err != ErrNilCoinState {
		t.Errorf("Expected ErrNilCoinState, got %v", err)
	}
}

// TestEvaluateAllStrategiesNilState tests nil state handling
func TestEvaluateAllStrategiesNilState(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("test", []MarketRegime{RegimeTrending}))

	results := registry.EvaluateAllStrategies(nil)
	if len(results) != 0 {
		t.Errorf("Expected empty map for nil state, got %d entries", len(results))
	}
}

// TestEvaluateAllStrategies tests evaluating all strategies
func TestEvaluateAllStrategies(t *testing.T) {
	registry := NewStrategyRegistry()

	strategy1 := NewMockStrategy("strategy1", []MarketRegime{RegimeTrending})
	strategy1.SetScore(60.0)

	strategy2 := NewMockStrategy("strategy2", []MarketRegime{RegimeRanging})
	strategy2.SetScore(80.0)

	registry.RegisterStrategy(strategy1)
	registry.RegisterStrategy(strategy2)

	state := NewCoinState("BTCUSDT")
	results := registry.EvaluateAllStrategies(state)

	if len(results) != 2 {
		t.Errorf("Results count = %d, expected 2", len(results))
	}

	if results["strategy1"].FinalScore != 60.0 {
		t.Errorf("strategy1 score = %f, expected 60.0", results["strategy1"].FinalScore)
	}

	if results["strategy2"].FinalScore != 80.0 {
		t.Errorf("strategy2 score = %f, expected 80.0", results["strategy2"].FinalScore)
	}
}

// TestGetStats tests registry statistics
func TestGetStats(t *testing.T) {
	registry := NewStrategyRegistry()

	registry.RegisterStrategy(NewMockStrategy("s1", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("s2", []MarketRegime{RegimeTrending, RegimeRanging}))
	registry.RegisterStrategy(NewMockStrategy("s3", []MarketRegime{RegimeVolatile}))

	registry.DisableStrategy("s3")

	stats := registry.GetStats()

	if stats.TotalStrategies != 3 {
		t.Errorf("TotalStrategies = %d, expected 3", stats.TotalStrategies)
	}

	if stats.EnabledStrategies != 2 {
		t.Errorf("EnabledStrategies = %d, expected 2", stats.EnabledStrategies)
	}

	if stats.StrategiesByRegime[RegimeTrending] != 2 {
		t.Errorf("TRENDING strategies = %d, expected 2", stats.StrategiesByRegime[RegimeTrending])
	}

	if stats.DefaultStrategy != "s1" {
		t.Errorf("DefaultStrategy = %s, expected s1", stats.DefaultStrategy)
	}
}

// TestClear tests clearing the registry
func TestClear(t *testing.T) {
	registry := NewStrategyRegistry()

	registry.RegisterStrategy(NewMockStrategy("s1", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("s2", []MarketRegime{RegimeRanging}))

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("Registry should be empty after clear, got %d", registry.Count())
	}

	if registry.GetDefaultStrategyName() != "" {
		t.Error("Default strategy should be empty after clear")
	}
}

// TestOnRegisterCallback tests registration callbacks
func TestOnRegisterCallback(t *testing.T) {
	registry := NewStrategyRegistry()

	var callbackCalled int32
	callback := func(s Strategy) {
		atomic.AddInt32(&callbackCalled, 1)
	}

	registry.OnRegister(callback)

	// Register a strategy
	registry.RegisterStrategy(NewMockStrategy("test", []MarketRegime{RegimeTrending}))

	// Wait a bit for async callback
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&callbackCalled) != 1 {
		t.Error("Callback should have been called once")
	}
}

// TestOnRegisterCallbackNil tests nil callback handling
func TestOnRegisterCallbackNil(t *testing.T) {
	registry := NewStrategyRegistry()

	// Should not panic
	registry.OnRegister(nil)

	registry.RegisterStrategy(NewMockStrategy("test", []MarketRegime{RegimeTrending}))
}

// TestConcurrentAccess tests thread safety
func TestConcurrentAccess(t *testing.T) {
	registry := NewStrategyRegistry()

	// Pre-register some strategies
	for i := 0; i < 5; i++ {
		name := string(rune('a' + i))
		registry.RegisterStrategy(NewMockStrategy(name, []MarketRegime{RegimeTrending}))
	}

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			registry.GetStrategy("a")
			registry.GetStrategiesForRegime(RegimeTrending)
			registry.ListStrategies()
			registry.GetStats()
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "concurrent_" + string(rune('a'+idx))
			registry.RegisterStrategy(NewMockStrategy(name, []MarketRegime{RegimeRanging}))
		}(i)
	}

	wg.Wait()

	// Should not have panicked and registry should be consistent
	if registry.Count() < 5 {
		t.Errorf("Registry should have at least 5 strategies after concurrent ops, got %d", registry.Count())
	}
}

// TestGlobalRegistry tests the global registry functions
func TestGlobalRegistry(t *testing.T) {
	// Clear global registry first
	GetGlobalRegistry().Clear()

	strategy := NewMockStrategy("global_test", []MarketRegime{RegimeTrending})
	err := RegisterGlobalStrategy(strategy)
	if err != nil {
		t.Fatalf("RegisterGlobalStrategy failed: %v", err)
	}

	retrieved, err := GetGlobalStrategy("global_test")
	if err != nil {
		t.Fatalf("GetGlobalStrategy failed: %v", err)
	}

	if retrieved.Name() != "global_test" {
		t.Errorf("Retrieved strategy name = %s, expected global_test", retrieved.Name())
	}

	// Cleanup
	GetGlobalRegistry().Clear()
}

// TestRegistryErrorString tests error formatting
func TestRegistryErrorString(t *testing.T) {
	err := &RegistryError{Code: "TEST_CODE", Message: "Test message"}
	str := err.Error()

	if str != "[TEST_CODE] Test message" {
		t.Errorf("Error string = %s, expected [TEST_CODE] Test message", str)
	}
}

// Benchmark tests

func BenchmarkRegisterStrategy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		registry := NewStrategyRegistry()
		registry.RegisterStrategy(NewMockStrategy("test", []MarketRegime{RegimeTrending}))
	}
}

func BenchmarkGetStrategy(b *testing.B) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("test", []MarketRegime{RegimeTrending}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.GetStrategy("test")
	}
}

func BenchmarkGetStrategiesForRegime(b *testing.B) {
	registry := NewStrategyRegistry()
	for i := 0; i < 10; i++ {
		name := "strategy_" + string(rune('a'+i))
		registry.RegisterStrategy(NewMockStrategy(name, []MarketRegime{RegimeTrending}))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.GetStrategiesForRegime(RegimeTrending)
	}
}

func BenchmarkEvaluateAllStrategies(b *testing.B) {
	registry := NewStrategyRegistry()
	for i := 0; i < 5; i++ {
		name := "strategy_" + string(rune('a'+i))
		registry.RegisterStrategy(NewMockStrategy(name, []MarketRegime{RegimeTrending}))
	}

	state := NewCoinState("BTCUSDT")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.EvaluateAllStrategies(state)
	}
}
