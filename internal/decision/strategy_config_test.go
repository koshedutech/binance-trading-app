// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.6: Strategy Registry - Config Tests
package decision

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestNewInMemoryStrategyConfigStore tests store creation
func TestNewInMemoryStrategyConfigStore(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()

	if store == nil {
		t.Fatal("NewInMemoryStrategyConfigStore should not return nil")
	}

	defaults := store.GetDefaults()
	if defaults == nil {
		t.Fatal("Defaults should not be nil")
	}

	if len(defaults.Strategies) != 4 {
		t.Errorf("Defaults should have 4 strategies, got %d", len(defaults.Strategies))
	}
}

// TestGetStrategyConfigDefaults tests getting default config
func TestGetStrategyConfigDefaults(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// Get config for user without custom settings
	config, err := store.GetStrategyConfig(ctx, "user1", StrategyTrendFollowing)
	if err != nil {
		t.Fatalf("GetStrategyConfig failed: %v", err)
	}

	if config.Name != StrategyTrendFollowing {
		t.Errorf("Config name = %s, expected trend_following", config.Name)
	}

	if !config.Enabled {
		t.Error("Default strategy should be enabled")
	}

	if config.Entry == nil || config.Exit == nil {
		t.Error("Entry and Exit conditions should not be nil")
	}
}

// TestSetAndGetStrategyConfig tests setting and getting config
func TestSetAndGetStrategyConfig(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// Create custom config
	customConfig := DefaultTrendFollowingSettings()
	customConfig.Entry.MinScore = 70.0
	customConfig.Entry.MinADX = 30.0

	// Set config
	err := store.SetStrategyConfig(ctx, "user1", customConfig)
	if err != nil {
		t.Fatalf("SetStrategyConfig failed: %v", err)
	}

	// Get config
	retrieved, err := store.GetStrategyConfig(ctx, "user1", StrategyTrendFollowing)
	if err != nil {
		t.Fatalf("GetStrategyConfig failed: %v", err)
	}

	if retrieved.Entry.MinScore != 70.0 {
		t.Errorf("MinScore = %f, expected 70.0", retrieved.Entry.MinScore)
	}

	if retrieved.Entry.MinADX != 30.0 {
		t.Errorf("MinADX = %f, expected 30.0", retrieved.Entry.MinADX)
	}
}

// TestSetStrategyConfigValidation tests validation on set
func TestSetStrategyConfigValidation(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// Test nil config
	err := store.SetStrategyConfig(ctx, "user1", nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}

	// Test invalid config
	invalidConfig := &StrategySettings{
		Name: "invalid_name", // Invalid strategy name
	}
	err = store.SetStrategyConfig(ctx, "user1", invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

// TestGetAllStrategyConfigs tests getting all configs
func TestGetAllStrategyConfigs(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// Get all configs for user without custom settings
	settings, err := store.GetAllStrategyConfigs(ctx, "user1")
	if err != nil {
		t.Fatalf("GetAllStrategyConfigs failed: %v", err)
	}

	if len(settings.Strategies) != 4 {
		t.Errorf("Should have 4 strategies, got %d", len(settings.Strategies))
	}

	// Check all strategy types are present
	strategies := []StrategyName{StrategyTrendFollowing, StrategyMeanReversion, StrategyBreakout, StrategyRangeTrading}
	for _, name := range strategies {
		if _, exists := settings.Strategies[name]; !exists {
			t.Errorf("Strategy %s should be present", name)
		}
	}
}

// TestSetAllStrategyConfigs tests setting all configs
func TestSetAllStrategyConfigs(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// Create custom settings
	customSettings := DefaultDecisionEngineSettings()
	customSettings.ActiveStrategy = StrategyBreakout
	customSettings.Strategies[StrategyTrendFollowing].Enabled = false

	// Set all configs
	err := store.SetAllStrategyConfigs(ctx, "user1", customSettings)
	if err != nil {
		t.Fatalf("SetAllStrategyConfigs failed: %v", err)
	}

	// Get and verify
	retrieved, err := store.GetAllStrategyConfigs(ctx, "user1")
	if err != nil {
		t.Fatalf("GetAllStrategyConfigs failed: %v", err)
	}

	if retrieved.ActiveStrategy != StrategyBreakout {
		t.Errorf("ActiveStrategy = %s, expected breakout", retrieved.ActiveStrategy)
	}

	if retrieved.Strategies[StrategyTrendFollowing].Enabled {
		t.Error("TrendFollowing should be disabled")
	}
}

// TestDeleteStrategyConfig tests deleting config
func TestDeleteStrategyConfig(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// First set some custom config
	customConfig := DefaultTrendFollowingSettings()
	customConfig.Entry.MinScore = 80.0
	store.SetStrategyConfig(ctx, "user1", customConfig)

	// Delete it
	err := store.DeleteStrategyConfig(ctx, "user1", StrategyTrendFollowing)
	if err != nil {
		t.Fatalf("DeleteStrategyConfig failed: %v", err)
	}

	// Getting should now return default (since strategy is removed from user config)
	settings, _ := store.GetAllStrategyConfigs(ctx, "user1")
	if _, exists := settings.Strategies[StrategyTrendFollowing]; exists {
		t.Error("TrendFollowing should be deleted from user config")
	}
}

// TestResetToDefaults tests resetting to defaults
func TestResetToDefaults(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// First set custom config
	customSettings := DefaultDecisionEngineSettings()
	customSettings.ActiveStrategy = StrategyBreakout
	customSettings.Strategies[StrategyTrendFollowing].Entry.MinScore = 90.0
	store.SetAllStrategyConfigs(ctx, "user1", customSettings)

	// Reset to defaults
	err := store.ResetToDefaults(ctx, "user1")
	if err != nil {
		t.Fatalf("ResetToDefaults failed: %v", err)
	}

	// Verify reset
	retrieved, _ := store.GetAllStrategyConfigs(ctx, "user1")
	if retrieved.ActiveStrategy != StrategyTrendFollowing {
		t.Errorf("ActiveStrategy should be reset to default (trend_following), got %s", retrieved.ActiveStrategy)
	}

	if retrieved.Strategies[StrategyTrendFollowing].Entry.MinScore != 55.0 {
		t.Errorf("MinScore should be reset to default (55.0), got %f", retrieved.Strategies[StrategyTrendFollowing].Entry.MinScore)
	}
}

// TestSetDefaults tests setting custom defaults
func TestSetDefaults(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()

	// Create custom defaults
	customDefaults := DefaultDecisionEngineSettings()
	customDefaults.Strategies[StrategyTrendFollowing].Entry.MinScore = 60.0

	store.SetDefaults(customDefaults)

	defaults := store.GetDefaults()
	if defaults.Strategies[StrategyTrendFollowing].Entry.MinScore != 60.0 {
		t.Errorf("Custom default MinScore = %f, expected 60.0", defaults.Strategies[StrategyTrendFollowing].Entry.MinScore)
	}
}

// TestClearUser tests clearing user config
func TestClearUser(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// Set custom config
	customConfig := DefaultTrendFollowingSettings()
	customConfig.Entry.MinScore = 80.0
	store.SetStrategyConfig(ctx, "user1", customConfig)

	// Clear user
	store.ClearUser("user1")

	// Should get defaults now
	retrieved, _ := store.GetStrategyConfig(ctx, "user1", StrategyTrendFollowing)
	if retrieved.Entry.MinScore != 55.0 {
		t.Errorf("After clear, should get default MinScore (55.0), got %f", retrieved.Entry.MinScore)
	}
}

// TestClearAll tests clearing all configs
func TestClearAll(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// Set configs for multiple users
	store.SetStrategyConfig(ctx, "user1", DefaultTrendFollowingSettings())
	store.SetStrategyConfig(ctx, "user2", DefaultMeanReversionSettings())

	// Clear all
	store.ClearAll()

	// Both users should get defaults now
	config1, _ := store.GetStrategyConfig(ctx, "user1", StrategyTrendFollowing)
	if config1.Entry.MinScore != 55.0 {
		t.Error("User1 should get defaults after ClearAll")
	}
}

// TestStrategyConfigManager tests the config manager
func TestStrategyConfigManager(t *testing.T) {
	registry := NewStrategyRegistry()
	store := NewInMemoryStrategyConfigStore()
	manager := NewStrategyConfigManager(registry, store)

	if manager.GetRegistry() != registry {
		t.Error("GetRegistry should return the same registry")
	}

	if manager.GetStore() != store {
		t.Error("GetStore should return the same store")
	}
}

// TestStrategyConfigManagerNilParams tests nil parameter handling
func TestStrategyConfigManagerNilParams(t *testing.T) {
	// Should create defaults if nil
	manager := NewStrategyConfigManager(nil, nil)

	if manager.GetRegistry() == nil {
		t.Error("Registry should not be nil")
	}

	if manager.GetStore() == nil {
		t.Error("Store should not be nil")
	}
}

// TestGetBestStrategyForState tests best strategy selection
func TestGetBestStrategyForState(t *testing.T) {
	registry := NewStrategyRegistry()
	store := NewInMemoryStrategyConfigStore()
	manager := NewStrategyConfigManager(registry, store)

	// Register strategies
	trendStrategy := NewMockStrategy("trend_following", []MarketRegime{RegimeTrending})
	trendStrategy.SetScore(80.0)

	rangeStrategy := NewMockStrategy("range_trading", []MarketRegime{RegimeTrending})
	rangeStrategy.SetScore(60.0)

	registry.RegisterStrategy(trendStrategy)
	registry.RegisterStrategy(rangeStrategy)

	ctx := context.Background()
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	strategy, score, err := manager.GetBestStrategyForState(ctx, "user1", state)
	if err != nil {
		t.Fatalf("GetBestStrategyForState failed: %v", err)
	}

	if strategy.Name() != "trend_following" {
		t.Errorf("Best strategy = %s, expected trend_following", strategy.Name())
	}

	if score.FinalScore != 80.0 {
		t.Errorf("Best score = %f, expected 80.0", score.FinalScore)
	}
}

// TestGetBestStrategyForStateNilState tests nil state handling
func TestGetBestStrategyForStateNilState(t *testing.T) {
	manager := NewStrategyConfigManager(nil, nil)
	ctx := context.Background()

	_, _, err := manager.GetBestStrategyForState(ctx, "user1", nil)
	if err == nil {
		t.Error("Expected error for nil state")
	}
}

// TestGetBestStrategyDisabledStrategy tests disabled strategy filtering
func TestGetBestStrategyDisabledStrategy(t *testing.T) {
	registry := NewStrategyRegistry()
	store := NewInMemoryStrategyConfigStore()
	manager := NewStrategyConfigManager(registry, store)

	// Register a high-scoring strategy
	highScore := NewMockStrategy("trend_following", []MarketRegime{RegimeTrending})
	highScore.SetScore(90.0)
	registry.RegisterStrategy(highScore)

	// Register a lower-scoring strategy
	lowScore := NewMockStrategy("mean_reversion", []MarketRegime{RegimeTrending})
	lowScore.SetScore(70.0)
	registry.RegisterStrategy(lowScore)

	ctx := context.Background()

	// Disable the high-scoring strategy in user settings
	userSettings := DefaultDecisionEngineSettings()
	userSettings.Strategies[StrategyTrendFollowing] = &StrategySettings{
		Name:    StrategyTrendFollowing,
		Enabled: false,
	}
	store.SetAllStrategyConfigs(ctx, "user1", userSettings)

	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	strategy, _, err := manager.GetBestStrategyForState(ctx, "user1", state)
	if err != nil {
		t.Fatalf("GetBestStrategyForState failed: %v", err)
	}

	// Should select the lower-scoring but enabled strategy
	if strategy.Name() != "mean_reversion" {
		t.Errorf("Should select enabled strategy, got %s", strategy.Name())
	}
}

// TestValidateStrategyConfig tests config validation
func TestValidateStrategyConfig(t *testing.T) {
	manager := NewStrategyConfigManager(nil, nil)

	// Valid config
	validConfig := DefaultTrendFollowingSettings()
	err := manager.ValidateStrategyConfig(validConfig)
	if err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	// Nil config
	err = manager.ValidateStrategyConfig(nil)
	if err == nil {
		t.Error("Nil config should fail validation")
	}
}

// TestValidateAllConfigs tests all config validation
func TestValidateAllConfigs(t *testing.T) {
	manager := NewStrategyConfigManager(nil, nil)

	// Valid settings
	validSettings := DefaultDecisionEngineSettings()
	err := manager.ValidateAllConfigs(validSettings)
	if err != nil {
		t.Errorf("Valid settings should pass validation: %v", err)
	}

	// Nil settings
	err = manager.ValidateAllConfigs(nil)
	if err == nil {
		t.Error("Nil settings should fail validation")
	}
}

// TestStrategyConfigJSON tests JSON conversion
func TestStrategyConfigJSON(t *testing.T) {
	registry := NewStrategyRegistry()
	registry.RegisterStrategy(NewMockStrategy("trend_following", []MarketRegime{RegimeTrending}))

	store := NewInMemoryStrategyConfigStore()
	manager := NewStrategyConfigManager(registry, store)

	ctx := context.Background()

	// Test ToJSON
	jsonConfig, err := manager.ToJSON(ctx, "user1")
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if jsonConfig.UserID != "user1" {
		t.Errorf("UserID = %s, expected user1", jsonConfig.UserID)
	}

	if len(jsonConfig.Strategies) != 4 {
		t.Errorf("Should have 4 strategies, got %d", len(jsonConfig.Strategies))
	}

	if len(jsonConfig.AvailableStrategies) != 1 {
		t.Errorf("Should have 1 available strategy, got %d", len(jsonConfig.AvailableStrategies))
	}
}

// TestFromJSON tests JSON parsing
func TestFromJSON(t *testing.T) {
	registry := NewStrategyRegistry()
	store := NewInMemoryStrategyConfigStore()
	manager := NewStrategyConfigManager(registry, store)

	ctx := context.Background()

	// Create JSON data - include all required strategies including breakout
	jsonData := `{
		"active_strategy": "breakout",
		"strategies": {
			"trend_following": {
				"name": "trend_following",
				"enabled": true,
				"entry": {
					"min_score": 65,
					"min_adx": 28,
					"require_trend_align": true,
					"volume_multiplier": 1.2,
					"min_confidence": 60,
					"max_spread": 0.1
				},
				"exit": {
					"take_profit_percent": 2,
					"stop_loss_percent": 1,
					"trailing_stop": true,
					"trailing_percent": 0.5,
					"max_hold_time": "4h",
					"exit_on_trend_reversal": true,
					"min_hold_time": "5m"
				},
				"primary_timeframe": "1h",
				"secondary_timeframe": "15m"
			},
			"breakout": {
				"name": "breakout",
				"enabled": true,
				"entry": {
					"min_score": 60,
					"min_adx": 15,
					"require_trend_align": true,
					"volume_multiplier": 1.5,
					"min_confidence": 65,
					"max_spread": 0.08
				},
				"exit": {
					"take_profit_percent": 3,
					"stop_loss_percent": 0.8,
					"trailing_stop": true,
					"trailing_percent": 0.6,
					"max_hold_time": "6h",
					"exit_on_trend_reversal": true,
					"min_hold_time": "10m"
				},
				"primary_timeframe": "4h",
				"secondary_timeframe": "1h"
			}
		}
	}`

	// Parse JSON
	err := manager.FromJSON(ctx, "user1", []byte(jsonData))
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	// Verify parsing
	settings, _ := manager.GetUserSettings(ctx, "user1")
	if settings.ActiveStrategy != StrategyBreakout {
		t.Errorf("ActiveStrategy = %s, expected breakout", settings.ActiveStrategy)
	}

	if settings.Strategies[StrategyTrendFollowing].Entry.MinScore != 65.0 {
		t.Errorf("MinScore = %f, expected 65.0", settings.Strategies[StrategyTrendFollowing].Entry.MinScore)
	}
}

// TestFromJSONInvalid tests invalid JSON handling
func TestFromJSONInvalid(t *testing.T) {
	manager := NewStrategyConfigManager(nil, nil)
	ctx := context.Background()

	// Invalid JSON
	err := manager.FromJSON(ctx, "user1", []byte("not valid json"))
	if err == nil {
		t.Error("Invalid JSON should fail")
	}
}

// TestGlobalConfigManager tests global config manager
func TestGlobalConfigManager(t *testing.T) {
	manager := GetGlobalConfigManager()
	if manager == nil {
		t.Fatal("GetGlobalConfigManager should not return nil")
	}

	// Same instance
	manager2 := GetGlobalConfigManager()
	if manager != manager2 {
		t.Error("GetGlobalConfigManager should return the same instance")
	}
}

// TestSetGlobalConfigManager tests setting global manager
func TestSetGlobalConfigManager(t *testing.T) {
	originalManager := GetGlobalConfigManager()

	customManager := NewStrategyConfigManager(nil, nil)
	SetGlobalConfigManager(customManager)

	if GetGlobalConfigManager() != customManager {
		t.Error("SetGlobalConfigManager should update the global manager")
	}

	// Restore original
	SetGlobalConfigManager(originalManager)
}

// TestStrategyConfigKeyGeneration tests key generation
func TestStrategyConfigKeyGeneration(t *testing.T) {
	key := StrategyConfigKey("user123", StrategyTrendFollowing)
	expected := "decision:strategy:user123:trend_following"
	if key != expected {
		t.Errorf("Key = %s, expected %s", key, expected)
	}

	allKey := StrategyConfigAllKey("user123")
	expectedAll := "decision:strategy:user123:all"
	if allKey != expectedAll {
		t.Errorf("AllKey = %s, expected %s", allKey, expectedAll)
	}
}

// TestStrategyConfigCloning tests that returned configs are clones
func TestStrategyConfigCloning(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	// Get config
	config1, _ := store.GetStrategyConfig(ctx, "user1", StrategyTrendFollowing)

	// Modify it
	config1.Entry.MinScore = 99.0

	// Get again
	config2, _ := store.GetStrategyConfig(ctx, "user1", StrategyTrendFollowing)

	// Should not be affected by modification
	if config2.Entry.MinScore == 99.0 {
		t.Error("Returned config should be a clone, modifications should not affect stored data")
	}
}

// Benchmark tests

func BenchmarkGetStrategyConfig(b *testing.B) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		store.GetStrategyConfig(ctx, "user1", StrategyTrendFollowing)
	}
}

func BenchmarkSetStrategyConfig(b *testing.B) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()
	config := DefaultTrendFollowingSettings()

	for i := 0; i < b.N; i++ {
		store.SetStrategyConfig(ctx, "user1", config)
	}
}

func BenchmarkGetBestStrategyForState(b *testing.B) {
	registry := NewStrategyRegistry()
	store := NewInMemoryStrategyConfigStore()
	manager := NewStrategyConfigManager(registry, store)

	for i := 0; i < 5; i++ {
		name := "strategy_" + string(rune('a'+i))
		registry.RegisterStrategy(NewMockStrategy(name, []MarketRegime{RegimeTrending}))
	}

	ctx := context.Background()
	state := NewCoinState("BTCUSDT")
	state.Regime = RegimeTrending

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetBestStrategyForState(ctx, "user1", state)
	}
}

// TestStrategyConfigJSONSerialization tests full JSON round-trip
func TestStrategyConfigJSONSerialization(t *testing.T) {
	original := DefaultDecisionEngineSettings()
	original.ActiveStrategy = StrategyBreakout
	original.Strategies[StrategyTrendFollowing].Entry.MinScore = 72.5

	// Serialize
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Deserialize
	var parsed DecisionEngineSettings
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify
	if parsed.ActiveStrategy != StrategyBreakout {
		t.Errorf("ActiveStrategy = %s, expected breakout", parsed.ActiveStrategy)
	}

	if parsed.Strategies[StrategyTrendFollowing].Entry.MinScore != 72.5 {
		t.Errorf("MinScore = %f, expected 72.5", parsed.Strategies[StrategyTrendFollowing].Entry.MinScore)
	}
}

// TestConcurrentConfigAccess tests thread safety of config store
func TestConcurrentConfigAccess(t *testing.T) {
	store := NewInMemoryStrategyConfigStore()
	ctx := context.Background()

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				store.GetStrategyConfig(ctx, "user1", StrategyTrendFollowing)
				store.GetAllStrategyConfigs(ctx, "user1")
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				config := DefaultTrendFollowingSettings()
				config.Entry.MinScore = float64(j)
				store.SetStrategyConfig(ctx, "user1", config)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}
