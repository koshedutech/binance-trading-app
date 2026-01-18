// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.7: Strategy Selector - Tests
package decision

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestNewStrategySelector tests selector creation
func TestNewStrategySelector(t *testing.T) {
	registry := NewStrategyRegistry()
	selector := NewStrategySelector(registry, nil, nil)

	if selector == nil {
		t.Fatal("NewStrategySelector should not return nil with valid registry")
	}

	if selector.GetRegistry() != registry {
		t.Error("GetRegistry should return the provided registry")
	}

	config := selector.GetConfig()
	if config.CooldownDuration != 5*time.Minute {
		t.Errorf("Default CooldownDuration = %v, expected 5m", config.CooldownDuration)
	}
	if config.DefaultStrategy != "trend_following" {
		t.Errorf("Default DefaultStrategy = %s, expected trend_following", config.DefaultStrategy)
	}
}

// TestNewStrategySelectorNilRegistry tests nil registry handling
func TestNewStrategySelectorNilRegistry(t *testing.T) {
	selector := NewStrategySelector(nil, nil, nil)
	if selector != nil {
		t.Error("NewStrategySelector with nil registry should return nil")
	}
}

// TestDefaultSelectorConfig tests default configuration
func TestDefaultSelectorConfig(t *testing.T) {
	config := DefaultSelectorConfig()

	if config.CooldownDuration != 5*time.Minute {
		t.Errorf("CooldownDuration = %v, expected 5m", config.CooldownDuration)
	}
	if config.DefaultStrategy != "trend_following" {
		t.Errorf("DefaultStrategy = %s, expected trend_following", config.DefaultStrategy)
	}
	if config.SelectionHistoryTTL != 24*time.Hour {
		t.Errorf("SelectionHistoryTTL = %v, expected 24h", config.SelectionHistoryTTL)
	}
	if config.MaxHistoryEntries != 100 {
		t.Errorf("MaxHistoryEntries = %d, expected 100", config.MaxHistoryEntries)
	}
	if !config.EnableLogging {
		t.Error("EnableLogging should default to true")
	}
}

// TestSelectStrategyRegimeMatch tests regime-based selection
func TestSelectStrategyRegimeMatch(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("trend_following", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("range_trading", []MarketRegime{RegimeRanging, RegimeConsolidating}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	selection, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("SelectStrategy failed: %v", err)
	}

	if selection.SelectedStrategy != "trend_following" {
		t.Errorf("SelectedStrategy = %s, expected trend_following", selection.SelectedStrategy)
	}
	if selection.Reason != ReasonRegimeMatch {
		t.Errorf("Reason = %s, expected REGIME_MATCH", selection.Reason)
	}
	if selection.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %s, expected BTCUSDT", selection.Symbol)
	}
	if selection.UserID != "user1" {
		t.Errorf("UserID = %s, expected user1", selection.UserID)
	}
}

// TestSelectStrategyBestScore tests best-score selection when multiple strategies match
func TestSelectStrategyBestScore(t *testing.T) {
	registry := NewStrategyRegistry()

	lowScore := NewMockStrategy("low_score", []MarketRegime{RegimeTrending})
	lowScore.SetScore(50.0)

	highScore := NewMockStrategy("high_score", []MarketRegime{RegimeTrending})
	highScore.SetScore(90.0)

	registry.RegisterStrategy(lowScore)
	registry.RegisterStrategy(highScore)

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	selection, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("SelectStrategy failed: %v", err)
	}

	if selection.SelectedStrategy != "high_score" {
		t.Errorf("SelectedStrategy = %s, expected high_score (best scoring)", selection.SelectedStrategy)
	}
	if selection.Reason != ReasonBestScore {
		t.Errorf("Reason = %s, expected BEST_SCORE", selection.Reason)
	}
	if selection.Score != 90.0 {
		t.Errorf("Score = %f, expected 90.0", selection.Score)
	}
}

// TestSelectStrategyFallback tests fallback to default strategy
func TestSelectStrategyFallback(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("trend_following", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeVolatile // No strategy for this regime

	selection, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("SelectStrategy failed: %v", err)
	}

	if selection.SelectedStrategy != "trend_following" {
		t.Errorf("SelectedStrategy = %s, expected trend_following (fallback/default)", selection.SelectedStrategy)
	}
	if selection.Reason != ReasonFallback {
		t.Errorf("Reason = %s, expected FALLBACK", selection.Reason)
	}
}

// TestSelectStrategyUserPreference tests user preference override
func TestSelectStrategyUserPreference(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("trend_following", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("preferred_strategy", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// Set user preference
	err := selector.SetUserPreference(ctx, "user1", "BTCUSDT", "preferred_strategy")
	if err != nil {
		t.Fatalf("SetUserPreference failed: %v", err)
	}

	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending // Would normally select trend_following

	selection, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("SelectStrategy failed: %v", err)
	}

	if selection.SelectedStrategy != "preferred_strategy" {
		t.Errorf("SelectedStrategy = %s, expected preferred_strategy (user preference)", selection.SelectedStrategy)
	}
	if selection.Reason != ReasonUserPreference {
		t.Errorf("Reason = %s, expected USER_PREFERENCE", selection.Reason)
	}
}

// TestSelectStrategyCooldown tests cooldown behavior
func TestSelectStrategyCooldown(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy_a", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("strategy_b", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.CooldownDuration = 100 * time.Millisecond // Short for testing
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// First selection - TRENDING regime (initial selection, no cooldown yet)
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	selection1, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("First SelectStrategy failed: %v", err)
	}
	if selection1.SelectedStrategy != "strategy_a" {
		t.Errorf("First selection = %s, expected strategy_a", selection1.SelectedStrategy)
	}

	// Change regime and set active strategy to simulate a change
	state.Regime = RegimeRanging
	state.ActiveStrategy = "strategy_a"

	// Second selection - regime changed, strategy should change, triggering cooldown
	selection2, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("Second SelectStrategy failed: %v", err)
	}
	if selection2.SelectedStrategy != "strategy_b" {
		t.Errorf("Second selection = %s, expected strategy_b (strategy change triggers cooldown)", selection2.SelectedStrategy)
	}
	// This selection triggers the cooldown for future switches

	// Third selection immediately - cooldown should now be active
	state.Regime = RegimeTrending
	state.ActiveStrategy = "strategy_b"

	selection3, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("Third SelectStrategy failed: %v", err)
	}
	// Cooldown should keep us on strategy_b
	if selection3.SelectedStrategy != "strategy_b" {
		t.Errorf("Third selection = %s, expected strategy_b (cooldown active)", selection3.SelectedStrategy)
	}
	if selection3.Reason != ReasonCooldown {
		t.Errorf("Third reason = %s, expected COOLDOWN", selection3.Reason)
	}
	if !selection3.CooldownActive {
		t.Error("CooldownActive should be true")
	}

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	// Fourth selection - cooldown expired, should select for new regime
	selection4, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("Fourth SelectStrategy failed: %v", err)
	}
	if selection4.SelectedStrategy != "strategy_a" {
		t.Errorf("Fourth selection = %s, expected strategy_a (after cooldown)", selection4.SelectedStrategy)
	}
	if selection4.Reason == ReasonCooldown {
		t.Error("Fourth reason should not be COOLDOWN")
	}
}

// TestSetUserPreferenceInvalidStrategy tests setting invalid preference
func TestSetUserPreferenceInvalidStrategy(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("valid_strategy", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	err := selector.SetUserPreference(ctx, "user1", "BTCUSDT", "nonexistent_strategy")
	if err == nil {
		t.Error("SetUserPreference should fail for nonexistent strategy")
	}
}

// TestClearUserPreference tests clearing user preference
func TestClearUserPreference(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("trend_following", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("preferred", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// Set preference
	selector.SetUserPreference(ctx, "user1", "BTCUSDT", "preferred")
	if !selector.HasUserPreference(ctx, "user1", "BTCUSDT") {
		t.Error("Preference should exist after set")
	}

	// Clear preference
	err := selector.ClearUserPreference(ctx, "user1", "BTCUSDT")
	if err != nil {
		t.Fatalf("ClearUserPreference failed: %v", err)
	}

	if selector.HasUserPreference(ctx, "user1", "BTCUSDT") {
		t.Error("Preference should not exist after clear")
	}
}

// TestClearCooldown tests manually clearing cooldown
func TestClearCooldown(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy_a", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("strategy_b", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.CooldownDuration = 10 * time.Minute // Long cooldown
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// First selection - TRENDING regime (initial, no cooldown)
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending
	selector.SelectStrategy(ctx, state, "user1")

	// Second selection - change regime, triggers cooldown on strategy switch
	state.Regime = RegimeRanging
	state.ActiveStrategy = "strategy_a"
	selector.SelectStrategy(ctx, state, "user1")

	// Now cooldown should be active (we just switched from strategy_a to strategy_b)
	remaining := selector.GetCooldownRemaining(ctx, "user1", "BTCUSDT")
	if remaining == 0 {
		t.Error("Cooldown should be active after strategy switch")
	}

	// Clear cooldown
	err := selector.ClearCooldown(ctx, "user1", "BTCUSDT")
	if err != nil {
		t.Fatalf("ClearCooldown failed: %v", err)
	}

	// Verify cooldown is cleared
	remaining = selector.GetCooldownRemaining(ctx, "user1", "BTCUSDT")
	if remaining != 0 {
		t.Errorf("Cooldown remaining = %v, expected 0 after clear", remaining)
	}

	// Change regime back and set active strategy
	state.Regime = RegimeTrending
	state.ActiveStrategy = "strategy_b"

	// Selection should now work without cooldown
	selection, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("SelectStrategy after clear failed: %v", err)
	}
	if selection.Reason == ReasonCooldown {
		t.Error("Selection should not be blocked by cooldown after clear")
	}
}

// TestGetCooldownRemaining tests cooldown remaining time
func TestGetCooldownRemaining(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy_a", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("strategy_b", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.CooldownDuration = 200 * time.Millisecond
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// No cooldown initially
	remaining := selector.GetCooldownRemaining(ctx, "user1", "BTCUSDT")
	if remaining != 0 {
		t.Errorf("Initial cooldown remaining = %v, expected 0", remaining)
	}

	// First selection - TRENDING regime (initial, no cooldown triggered)
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending
	selector.SelectStrategy(ctx, state, "user1")

	// Second selection - switch to different strategy to trigger cooldown
	state.Regime = RegimeRanging
	state.ActiveStrategy = "strategy_a"
	selector.SelectStrategy(ctx, state, "user1")

	// Check remaining (should be close to 200ms)
	remaining = selector.GetCooldownRemaining(ctx, "user1", "BTCUSDT")
	if remaining < 100*time.Millisecond || remaining > 200*time.Millisecond {
		t.Errorf("Cooldown remaining = %v, expected ~200ms", remaining)
	}

	// Wait and check again
	time.Sleep(100 * time.Millisecond)
	remaining = selector.GetCooldownRemaining(ctx, "user1", "BTCUSDT")
	if remaining > 110*time.Millisecond {
		t.Errorf("After 100ms, remaining = %v, expected ~100ms", remaining)
	}
}

// TestSelectStrategyNilState tests nil state handling
func TestSelectStrategyNilState(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	_, err := selector.SelectStrategy(context.Background(), nil, "user1")
	if err == nil {
		t.Error("SelectStrategy should fail with nil state")
	}
}

// TestSelectStrategyEmptyUserID tests empty userID handling
func TestSelectStrategyEmptyUserID(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	_, err := selector.SelectStrategy(context.Background(), state, "")
	if err == nil {
		t.Error("SelectStrategy should fail with empty userID")
	}
}

// TestSelectStrategyNoStrategies tests error when no strategies available
func TestSelectStrategyNoStrategies(t *testing.T) {
	registry := NewStrategyRegistry()
	// No strategies registered

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	_, err := selector.SelectStrategy(ctx, state, "user1")
	if err == nil {
		t.Error("SelectStrategy should fail when no strategies available")
	}
}

// TestSelectionHistory tests selection history (in-memory, no Redis)
func TestSelectionHistoryWithoutRedis(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// Without Redis, history operations should return empty/no error
	history, err := selector.GetSelectionHistory(ctx, "user1", "BTCUSDT", 10)
	if err != nil {
		t.Fatalf("GetSelectionHistory failed: %v", err)
	}
	if len(history) != 0 {
		t.Error("History should be empty without Redis")
	}

	count, err := selector.GetSelectionHistoryCount(ctx, "user1", "BTCUSDT")
	if err != nil {
		t.Fatalf("GetSelectionHistoryCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Count = %d, expected 0 without Redis", count)
	}

	err = selector.ClearSelectionHistory(ctx, "user1", "BTCUSDT")
	if err != nil {
		t.Fatalf("ClearSelectionHistory failed: %v", err)
	}
}

// TestNewStrategySelection tests selection creation
func TestNewStrategySelection(t *testing.T) {
	selection := NewStrategySelection("BTCUSDT", "user1", RegimeTrending, "trend_following", ReasonRegimeMatch)

	if selection.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %s, expected BTCUSDT", selection.Symbol)
	}
	if selection.UserID != "user1" {
		t.Errorf("UserID = %s, expected user1", selection.UserID)
	}
	if selection.Regime != RegimeTrending {
		t.Errorf("Regime = %s, expected TRENDING", selection.Regime)
	}
	if selection.SelectedStrategy != "trend_following" {
		t.Errorf("SelectedStrategy = %s, expected trend_following", selection.SelectedStrategy)
	}
	if selection.Reason != ReasonRegimeMatch {
		t.Errorf("Reason = %s, expected REGIME_MATCH", selection.Reason)
	}
	if selection.Timestamp == 0 {
		t.Error("Timestamp should be set")
	}
}

// TestStrategySelectionToJSON tests JSON serialization
func TestStrategySelectionToJSON(t *testing.T) {
	selection := NewStrategySelection("BTCUSDT", "user1", RegimeTrending, "trend_following", ReasonRegimeMatch)
	selection.Score = 75.5
	selection.AvailableStrategies = []string{"trend_following", "range_trading"}

	json := selection.ToJSON()
	if json == "" {
		t.Error("ToJSON should return non-empty string")
	}

	// Check that key fields are in JSON
	if !testContainsSubstring(json, "BTCUSDT") {
		t.Error("JSON should contain symbol")
	}
	if !testContainsSubstring(json, "trend_following") {
		t.Error("JSON should contain strategy name")
	}
	if !testContainsSubstring(json, "REGIME_MATCH") {
		t.Error("JSON should contain reason")
	}
}

// TestSelectionReasons tests reason constants
func TestSelectionReasons(t *testing.T) {
	reasons := []SelectionReason{
		ReasonRegimeMatch,
		ReasonUserPreference,
		ReasonFallback,
		ReasonCooldown,
		ReasonBestScore,
	}

	for _, reason := range reasons {
		if reason == "" {
			t.Error("SelectionReason should not be empty")
		}
	}
}

// TestConcurrentSelection tests thread safety
func TestConcurrentSelection(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy_a", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("strategy_b", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	config.CooldownDuration = 1 * time.Millisecond // Very short for testing
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	var wg sync.WaitGroup
	iterations := 50

	// Concurrent selections for same symbol
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			state := NewCoinState("BTCUSDT")
			if idx%2 == 0 {
				state.Regime = RegimeTrending
			} else {
				state.Regime = RegimeRanging
			}
			selector.SelectStrategy(ctx, state, "user1")
		}(i)
	}

	// Concurrent selections for different symbols
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			symbol := "SYMBOL" + string(rune('A'+idx%10))
			state := NewCoinState(symbol)
			state.Regime = RegimeTrending
			selector.SelectStrategy(ctx, state, "user1")
		}(i)
	}

	// Concurrent preference operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			symbol := "PREF" + string(rune('A'+idx))
			selector.SetUserPreference(ctx, "user1", symbol, "strategy_a")
			selector.HasUserPreference(ctx, "user1", symbol)
			selector.ClearUserPreference(ctx, "user1", symbol)
		}(i)
	}

	wg.Wait()
	// If we get here without panic, concurrent access is safe
}

// TestDisabledStrategySelection tests that disabled strategies are not selected
func TestDisabledStrategySelection(t *testing.T) {
	registry := NewStrategyRegistry()
	enabledStrategy := NewMockStrategy("enabled", []MarketRegime{RegimeTrending})
	enabledStrategy.SetScore(50.0)

	disabledStrategy := NewMockStrategy("disabled", []MarketRegime{RegimeTrending})
	disabledStrategy.SetScore(100.0) // Higher score but disabled

	registry.RegisterStrategy(enabledStrategy)
	registry.RegisterStrategy(disabledStrategy)
	registry.DisableStrategy("disabled")

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	selection, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("SelectStrategy failed: %v", err)
	}

	if selection.SelectedStrategy != "enabled" {
		t.Errorf("SelectedStrategy = %s, expected enabled (disabled strategy should be skipped)", selection.SelectedStrategy)
	}
}

// TestUserPreferenceClearedWhenInvalid tests that invalid preferences are auto-cleared
func TestUserPreferenceClearedWhenInvalid(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("valid_strategy", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("to_be_unregistered", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// Set preference to a valid strategy
	selector.SetUserPreference(ctx, "user1", "BTCUSDT", "to_be_unregistered")

	// Unregister the preferred strategy
	registry.UnregisterStrategy("to_be_unregistered")

	// Selection should fall back to regime-based selection
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	selection, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("SelectStrategy failed: %v", err)
	}

	// Should select valid_strategy since preference is now invalid
	if selection.SelectedStrategy != "valid_strategy" {
		t.Errorf("SelectedStrategy = %s, expected valid_strategy", selection.SelectedStrategy)
	}
	if selection.Reason == ReasonUserPreference {
		t.Error("Reason should not be USER_PREFERENCE for invalid preference")
	}

	// Preference should be auto-cleared
	if selector.HasUserPreference(ctx, "user1", "BTCUSDT") {
		t.Error("Invalid preference should be auto-cleared")
	}
}

// TestSetUserPreferenceEmptyClears tests that empty preference clears
func TestSetUserPreferenceEmptyClears(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// Set preference
	selector.SetUserPreference(ctx, "user1", "BTCUSDT", "strategy")
	if !selector.HasUserPreference(ctx, "user1", "BTCUSDT") {
		t.Error("Preference should exist")
	}

	// Set empty preference (should clear)
	selector.SetUserPreference(ctx, "user1", "BTCUSDT", "")
	if selector.HasUserPreference(ctx, "user1", "BTCUSDT") {
		t.Error("Empty preference should clear existing preference")
	}
}

// TestAvailableStrategiesInSelection tests that available strategies are listed
func TestAvailableStrategiesInSelection(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy_a", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("strategy_b", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("strategy_c", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	selection, err := selector.SelectStrategy(ctx, state, "user1")
	if err != nil {
		t.Fatalf("SelectStrategy failed: %v", err)
	}

	// Should list only strategies that support TRENDING
	if len(selection.AvailableStrategies) != 2 {
		t.Errorf("AvailableStrategies count = %d, expected 2", len(selection.AvailableStrategies))
	}
}

// TestGetSelectionStats tests statistics gathering without Redis
func TestGetSelectionStatsWithoutRedis(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	stats, err := selector.GetSelectionStats(ctx, "user1", []string{"BTCUSDT", "ETHUSDT"})
	if err != nil {
		t.Fatalf("GetSelectionStats failed: %v", err)
	}

	// Without Redis, stats should be empty but not nil
	if stats == nil {
		t.Error("Stats should not be nil")
	}
	if stats.UserID != "user1" {
		t.Errorf("UserID = %s, expected user1", stats.UserID)
	}
	if stats.TotalSelections != 0 {
		t.Errorf("TotalSelections = %d, expected 0 without Redis", stats.TotalSelections)
	}
}

// TestClearAllUserData tests clearing all user data
func TestClearAllUserData(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// Set up some data
	selector.SetUserPreference(ctx, "user1", "BTCUSDT", "strategy")
	selector.SetUserPreference(ctx, "user1", "ETHUSDT", "strategy")

	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending
	selector.SelectStrategy(ctx, state, "user1")

	// Clear all data
	err := selector.ClearAllUserData(ctx, "user1", []string{"BTCUSDT", "ETHUSDT"})
	if err != nil {
		t.Fatalf("ClearAllUserData failed: %v", err)
	}

	// Verify preferences cleared
	if selector.HasUserPreference(ctx, "user1", "BTCUSDT") {
		t.Error("Preference should be cleared")
	}
	if selector.HasUserPreference(ctx, "user1", "ETHUSDT") {
		t.Error("Preference should be cleared")
	}
}

// Benchmark tests

func BenchmarkSelectStrategy(b *testing.B) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("trend_following", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("range_trading", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	config.CooldownDuration = 0 // Disable cooldown for benchmark
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selector.SelectStrategy(ctx, state, "user1")
	}
}

func BenchmarkSelectStrategyWithPreference(b *testing.B) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("trend_following", []MarketRegime{RegimeTrending}))
	registry.RegisterStrategy(NewMockStrategy("preferred", []MarketRegime{RegimeRanging}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()
	selector.SetUserPreference(ctx, "user1", "BTCUSDT", "preferred")

	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selector.SelectStrategy(ctx, state, "user1")
	}
}

func BenchmarkCooldownCheck(b *testing.B) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// Trigger cooldown
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending
	selector.SelectStrategy(ctx, state, "user1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selector.isCooldownActive(ctx, "user1", "BTCUSDT")
	}
}

// TestGetSelectionHistoryCountValidation tests validation in GetSelectionHistoryCount
func TestGetSelectionHistoryCountValidation(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("strategy", []MarketRegime{RegimeTrending}))

	config := DefaultSelectorConfig()
	config.EnableLogging = false
	selector := NewStrategySelector(registry, nil, config)

	ctx := context.Background()

	// Test empty userID
	_, err := selector.GetSelectionHistoryCount(ctx, "", "BTCUSDT")
	if err == nil {
		t.Error("GetSelectionHistoryCount should fail with empty userID")
	}

	// Test empty symbol
	_, err = selector.GetSelectionHistoryCount(ctx, "user1", "")
	if err == nil {
		t.Error("GetSelectionHistoryCount should fail with empty symbol")
	}
}

// Note: testContainsSubstring helper is defined in strategy_test.go
