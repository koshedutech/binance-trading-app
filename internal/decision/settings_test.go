// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.23: Decision Engine Settings Tests
package decision

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestStrategyName_IsValid tests strategy name validation.
func TestStrategyName_IsValid(t *testing.T) {
	testCases := []struct {
		name     string
		strategy StrategyName
		expected bool
	}{
		{"trend_following valid", StrategyTrendFollowing, true},
		{"mean_reversion valid", StrategyMeanReversion, true},
		{"breakout valid", StrategyBreakout, true},
		{"range_trading valid", StrategyRangeTrading, true},
		{"empty invalid", StrategyName(""), false},
		{"unknown invalid", StrategyName("unknown"), false},
		{"typo invalid", StrategyName("trend-following"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.strategy.IsValid()
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestAllStrategies tests the AllStrategies function.
func TestAllStrategies(t *testing.T) {
	strategies := AllStrategies()

	if len(strategies) != 4 {
		t.Errorf("expected 4 strategies, got %d", len(strategies))
	}

	// Verify all strategies are valid
	for _, s := range strategies {
		if !s.IsValid() {
			t.Errorf("strategy %s should be valid", s)
		}
	}
}

// TestDefaultDecisionEngineSettings tests default settings creation.
func TestDefaultDecisionEngineSettings(t *testing.T) {
	settings := DefaultDecisionEngineSettings()

	// Check basic structure
	if settings == nil {
		t.Fatal("settings should not be nil")
	}
	if settings.Strategies == nil {
		t.Error("strategies map should not be nil")
	}
	if settings.ActiveStrategy != StrategyTrendFollowing {
		t.Errorf("expected active strategy %s, got %s", StrategyTrendFollowing, settings.ActiveStrategy)
	}
	if settings.Version != 1 {
		t.Errorf("expected version 1, got %d", settings.Version)
	}
	if settings.LastUpdated <= 0 {
		t.Error("last_updated should be set")
	}

	// Check all strategies are present
	for _, strategyName := range AllStrategies() {
		strategy := settings.GetStrategy(strategyName)
		if strategy == nil {
			t.Errorf("strategy %s should be present", strategyName)
			continue
		}
		if strategy.Name != strategyName {
			t.Errorf("strategy name mismatch: expected %s, got %s", strategyName, strategy.Name)
		}
		if !strategy.Enabled {
			t.Errorf("strategy %s should be enabled by default", strategyName)
		}
	}

	// Validate the settings
	if err := settings.Validate(); err != nil {
		t.Errorf("default settings should be valid: %v", err)
	}
}

// TestDefaultTrendFollowingSettings tests trend following defaults.
func TestDefaultTrendFollowingSettings(t *testing.T) {
	settings := DefaultTrendFollowingSettings()

	if settings.Name != StrategyTrendFollowing {
		t.Errorf("expected name %s, got %s", StrategyTrendFollowing, settings.Name)
	}
	if !settings.Enabled {
		t.Error("should be enabled by default")
	}
	if settings.Description == "" {
		t.Error("description should not be empty")
	}
	if settings.Scoring == nil {
		t.Error("scoring config should not be nil")
	}
	if settings.Blocking == nil {
		t.Error("blocking config should not be nil")
	}
	if settings.Entry == nil {
		t.Error("entry conditions should not be nil")
	}
	if settings.Exit == nil {
		t.Error("exit conditions should not be nil")
	}

	// Check specific trend following values
	if settings.Entry.RequireTrendAlign != true {
		t.Error("trend following should require trend alignment")
	}
	if settings.Exit.TrailingStop != true {
		t.Error("trend following should use trailing stop")
	}
	if settings.PrimaryTimeframe != "1h" {
		t.Errorf("expected primary timeframe 1h, got %s", settings.PrimaryTimeframe)
	}
}

// TestDefaultMeanReversionSettings tests mean reversion defaults.
func TestDefaultMeanReversionSettings(t *testing.T) {
	settings := DefaultMeanReversionSettings()

	if settings.Name != StrategyMeanReversion {
		t.Errorf("expected name %s, got %s", StrategyMeanReversion, settings.Name)
	}

	// Mean reversion should not require trend alignment
	if settings.Entry.RequireTrendAlign {
		t.Error("mean reversion should not require trend alignment")
	}
	// Mean reversion uses fixed targets
	if settings.Exit.TrailingStop {
		t.Error("mean reversion should not use trailing stop")
	}
}

// TestDefaultBreakoutSettings tests breakout defaults.
func TestDefaultBreakoutSettings(t *testing.T) {
	settings := DefaultBreakoutSettings()

	if settings.Name != StrategyBreakout {
		t.Errorf("expected name %s, got %s", StrategyBreakout, settings.Name)
	}

	// Breakout should require higher volume
	if settings.Entry.VolumeMultiplier < 1.5 {
		t.Errorf("breakout should require higher volume, got %.2f", settings.Entry.VolumeMultiplier)
	}
	// Breakout uses larger targets
	if settings.Exit.TakeProfitPercent < 2.5 {
		t.Errorf("breakout should have larger take profit, got %.2f", settings.Exit.TakeProfitPercent)
	}
}

// TestDefaultRangeTradingSettings tests range trading defaults.
func TestDefaultRangeTradingSettings(t *testing.T) {
	settings := DefaultRangeTradingSettings()

	if settings.Name != StrategyRangeTrading {
		t.Errorf("expected name %s, got %s", StrategyRangeTrading, settings.Name)
	}

	// Range trading doesn't need ADX
	if settings.Entry.MinADX > 5 {
		t.Errorf("range trading should not require high ADX, got %.2f", settings.Entry.MinADX)
	}
	// Range trading uses fixed targets
	if settings.Exit.TrailingStop {
		t.Error("range trading should not use trailing stop")
	}
}

// TestStrategySettings_Validation tests strategy settings validation.
func TestStrategySettings_Validation(t *testing.T) {
	testCases := []struct {
		name      string
		modify    func(*StrategySettings)
		expectErr bool
	}{
		{
			name:      "valid settings",
			modify:    func(s *StrategySettings) {},
			expectErr: false,
		},
		{
			name: "invalid strategy name",
			modify: func(s *StrategySettings) {
				s.Name = "invalid"
			},
			expectErr: true,
		},
		{
			name: "entry min_score too low",
			modify: func(s *StrategySettings) {
				s.Entry.MinScore = -1
			},
			expectErr: true,
		},
		{
			name: "entry min_score too high",
			modify: func(s *StrategySettings) {
				s.Entry.MinScore = 150
			},
			expectErr: true,
		},
		{
			name: "exit take_profit negative",
			modify: func(s *StrategySettings) {
				s.Exit.TakeProfitPercent = -5
			},
			expectErr: true,
		},
		{
			name: "trailing stop enabled but zero percent",
			modify: func(s *StrategySettings) {
				s.Exit.TrailingStop = true
				s.Exit.TrailingPercent = 0
			},
			expectErr: true,
		},
		{
			name: "trailing stop disabled with zero percent",
			modify: func(s *StrategySettings) {
				s.Exit.TrailingStop = false
				s.Exit.TrailingPercent = 0
			},
			expectErr: false,
		},
		{
			name: "invalid primary timeframe",
			modify: func(s *StrategySettings) {
				s.PrimaryTimeframe = "invalid"
			},
			expectErr: true,
		},
		{
			name: "invalid secondary timeframe",
			modify: func(s *StrategySettings) {
				s.SecondaryTimeframe = "2x"
			},
			expectErr: true,
		},
		{
			name: "valid timeframes",
			modify: func(s *StrategySettings) {
				s.PrimaryTimeframe = "4h"
				s.SecondaryTimeframe = "15m"
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			settings := DefaultTrendFollowingSettings()
			tc.modify(settings)

			err := settings.Validate()
			if tc.expectErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestEntryConditions_Validation tests entry conditions validation.
func TestEntryConditions_Validation(t *testing.T) {
	testCases := []struct {
		name      string
		entry     *EntryConditions
		expectErr bool
	}{
		{
			name: "valid entry",
			entry: &EntryConditions{
				MinScore:          55.0,
				MinADX:            25.0,
				RequireTrendAlign: true,
				VolumeMultiplier:  1.2,
				MinConfidence:     60.0,
				MaxSpread:         0.1,
			},
			expectErr: false,
		},
		{
			name: "negative min_score",
			entry: &EntryConditions{
				MinScore: -10,
			},
			expectErr: true,
		},
		{
			name: "min_adx too high",
			entry: &EntryConditions{
				MinScore: 50,
				MinADX:   150,
			},
			expectErr: true,
		},
		{
			name: "negative volume multiplier",
			entry: &EntryConditions{
				MinScore:         50,
				MinADX:           25,
				VolumeMultiplier: -1,
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expectErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestExitConditions_Validation tests exit conditions validation.
func TestExitConditions_Validation(t *testing.T) {
	testCases := []struct {
		name      string
		exit      *ExitConditions
		expectErr bool
	}{
		{
			name: "valid exit",
			exit: &ExitConditions{
				TakeProfitPercent: 2.0,
				StopLossPercent:   1.0,
				TrailingStop:      true,
				TrailingPercent:   0.5,
				MaxHoldTime:       "4h",
			},
			expectErr: false,
		},
		{
			name: "negative take profit",
			exit: &ExitConditions{
				TakeProfitPercent: -5,
			},
			expectErr: true,
		},
		{
			name: "trailing stop without percent",
			exit: &ExitConditions{
				TakeProfitPercent: 2.0,
				StopLossPercent:   1.0,
				TrailingStop:      true,
				TrailingPercent:   0,
			},
			expectErr: true,
		},
		{
			name: "trailing percent too high",
			exit: &ExitConditions{
				TakeProfitPercent: 2.0,
				StopLossPercent:   1.0,
				TrailingStop:      false,
				TrailingPercent:   150,
			},
			expectErr: true,
		},
		{
			name: "invalid max_hold_time format",
			exit: &ExitConditions{
				TakeProfitPercent: 2.0,
				StopLossPercent:   1.0,
				MaxHoldTime:       "invalid",
			},
			expectErr: true,
		},
		{
			name: "invalid min_hold_time format",
			exit: &ExitConditions{
				TakeProfitPercent: 2.0,
				StopLossPercent:   1.0,
				MinHoldTime:       "abc",
			},
			expectErr: true,
		},
		{
			name: "valid duration formats",
			exit: &ExitConditions{
				TakeProfitPercent: 2.0,
				StopLossPercent:   1.0,
				MaxHoldTime:       "4h",
				MinHoldTime:       "5m",
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.exit.Validate()
			if tc.expectErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestDecisionEngineSettings_Validation tests full settings validation.
func TestDecisionEngineSettings_Validation(t *testing.T) {
	testCases := []struct {
		name      string
		modify    func(*DecisionEngineSettings)
		expectErr bool
	}{
		{
			name:      "valid defaults",
			modify:    func(s *DecisionEngineSettings) {},
			expectErr: false,
		},
		{
			name: "nil strategies",
			modify: func(s *DecisionEngineSettings) {
				s.Strategies = nil
			},
			expectErr: true,
		},
		{
			name: "invalid active strategy",
			modify: func(s *DecisionEngineSettings) {
				s.ActiveStrategy = "invalid"
			},
			expectErr: true,
		},
		{
			name: "active strategy not in map",
			modify: func(s *DecisionEngineSettings) {
				delete(s.Strategies, StrategyTrendFollowing)
			},
			expectErr: true,
		},
		{
			name: "nil strategy settings",
			modify: func(s *DecisionEngineSettings) {
				s.Strategies[StrategyBreakout] = nil
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			settings := DefaultDecisionEngineSettings()
			tc.modify(settings)

			err := settings.Validate()
			if tc.expectErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestDecisionEngineSettings_Clone tests deep cloning.
func TestDecisionEngineSettings_Clone(t *testing.T) {
	original := DefaultDecisionEngineSettings()
	clone := original.Clone()

	// Verify clone is not nil
	if clone == nil {
		t.Fatal("clone should not be nil")
	}

	// Verify deep copy - modifying clone doesn't affect original
	clone.ActiveStrategy = StrategyBreakout
	clone.Strategies[StrategyTrendFollowing].Entry.MinScore = 99.0

	if original.ActiveStrategy != StrategyTrendFollowing {
		t.Error("original active strategy should not change")
	}
	if original.Strategies[StrategyTrendFollowing].Entry.MinScore == 99.0 {
		t.Error("original entry min score should not change")
	}

	// Verify nil clone
	var nilSettings *DecisionEngineSettings
	nilClone := nilSettings.Clone()
	if nilClone != nil {
		t.Error("nil clone should be nil")
	}
}

// TestStrategySettings_Clone tests strategy settings cloning.
func TestStrategySettings_Clone(t *testing.T) {
	original := DefaultTrendFollowingSettings()
	clone := original.Clone()

	if clone == nil {
		t.Fatal("clone should not be nil")
	}

	// Modify clone
	clone.Entry.MinScore = 88.0
	clone.Exit.TakeProfitPercent = 5.0
	clone.Scoring.TechnicalWeight = 2.0

	// Verify original unchanged
	if original.Entry.MinScore == 88.0 {
		t.Error("original entry should not change")
	}
	if original.Exit.TakeProfitPercent == 5.0 {
		t.Error("original exit should not change")
	}
	if original.Scoring.TechnicalWeight == 2.0 {
		t.Error("original scoring should not change")
	}
}

// TestDecisionEngineSettings_JSON tests JSON serialization.
func TestDecisionEngineSettings_JSON(t *testing.T) {
	original := DefaultDecisionEngineSettings()

	// Serialize
	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	// Verify it's valid JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Deserialize
	restored, err := FromJSONSettings(data)
	if err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	// Verify restored settings match
	if restored.ActiveStrategy != original.ActiveStrategy {
		t.Error("active strategy mismatch after JSON round-trip")
	}
	if len(restored.Strategies) != len(original.Strategies) {
		t.Error("strategies count mismatch after JSON round-trip")
	}

	// Verify validation still passes
	if err := restored.Validate(); err != nil {
		t.Errorf("restored settings should be valid: %v", err)
	}
}

// TestDecisionEngineSettings_GetSetStrategy tests get/set strategy operations.
func TestDecisionEngineSettings_GetSetStrategy(t *testing.T) {
	settings := DefaultDecisionEngineSettings()

	// Get existing strategy
	strategy := settings.GetStrategy(StrategyTrendFollowing)
	if strategy == nil {
		t.Fatal("should get trend following strategy")
	}

	// Get non-existent strategy
	nonExistent := settings.GetStrategy(StrategyName("unknown"))
	if nonExistent != nil {
		t.Error("should return nil for unknown strategy")
	}

	// Set new strategy
	custom := &StrategySettings{
		Name:        StrategyBreakout,
		Enabled:     true,
		Description: "Custom breakout",
		Entry: &EntryConditions{
			MinScore: 70.0,
		},
	}
	settings.SetStrategy(custom)

	// Verify it was set
	retrieved := settings.GetStrategy(StrategyBreakout)
	if retrieved.Entry.MinScore != 70.0 {
		t.Error("strategy was not updated correctly")
	}
}

// TestSettingsService_GetSettings tests settings retrieval.
func TestSettingsService_GetSettings(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-1"

	// Get settings for new user - should return defaults
	settings, err := service.GetSettings(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	if settings == nil {
		t.Fatal("settings should not be nil")
	}
	if settings.ActiveStrategy != StrategyTrendFollowing {
		t.Error("should have default active strategy")
	}

	// Verify empty userID fails
	_, err = service.GetSettings(ctx, "")
	if err == nil {
		t.Error("should fail with empty userID")
	}
}

// TestSettingsService_SaveSettings tests settings persistence.
func TestSettingsService_SaveSettings(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-2"

	// Save settings
	settings := DefaultDecisionEngineSettings()
	settings.ActiveStrategy = StrategyBreakout

	err := service.SaveSettings(ctx, userID, settings)
	if err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	// Retrieve and verify
	retrieved, err := service.GetSettings(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	if retrieved.ActiveStrategy != StrategyBreakout {
		t.Error("active strategy should be breakout")
	}

	// Verify empty userID fails
	err = service.SaveSettings(ctx, "", settings)
	if err == nil {
		t.Error("should fail with empty userID")
	}

	// Verify nil settings fails
	err = service.SaveSettings(ctx, userID, nil)
	if err == nil {
		t.Error("should fail with nil settings")
	}
}

// TestSettingsService_GetStrategySettings tests individual strategy retrieval.
func TestSettingsService_GetStrategySettings(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-3"

	// Get strategy for user (will use defaults)
	strategy, err := service.GetStrategySettings(ctx, userID, StrategyMeanReversion)
	if err != nil {
		t.Fatalf("failed to get strategy: %v", err)
	}
	if strategy.Name != StrategyMeanReversion {
		t.Error("should get mean reversion strategy")
	}

	// Invalid strategy should fail
	_, err = service.GetStrategySettings(ctx, userID, StrategyName("invalid"))
	if err == nil {
		t.Error("should fail with invalid strategy")
	}
}

// TestSettingsService_UpdateStrategySettings tests strategy updates.
func TestSettingsService_UpdateStrategySettings(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-4"

	// Update a strategy
	updated := DefaultBreakoutSettings()
	updated.Entry.MinScore = 75.0

	err := service.UpdateStrategySettings(ctx, userID, StrategyBreakout, updated)
	if err != nil {
		t.Fatalf("failed to update strategy: %v", err)
	}

	// Verify update
	strategy, _ := service.GetStrategySettings(ctx, userID, StrategyBreakout)
	if strategy.Entry.MinScore != 75.0 {
		t.Error("strategy should be updated")
	}

	// Test name mismatch
	updated.Name = StrategyTrendFollowing
	err = service.UpdateStrategySettings(ctx, userID, StrategyBreakout, updated)
	if err == nil {
		t.Error("should fail with name mismatch")
	}
}

// TestSettingsService_ActiveStrategy tests active strategy management.
func TestSettingsService_ActiveStrategy(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-5"

	// Get active strategy (defaults to trend following)
	active, err := service.GetActiveStrategy(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get active strategy: %v", err)
	}
	if active.Name != StrategyTrendFollowing {
		t.Error("default active should be trend following")
	}

	// Change active strategy
	err = service.SetActiveStrategy(ctx, userID, StrategyMeanReversion)
	if err != nil {
		t.Fatalf("failed to set active strategy: %v", err)
	}

	// Verify change
	active, _ = service.GetActiveStrategy(ctx, userID)
	if active.Name != StrategyMeanReversion {
		t.Error("active should now be mean reversion")
	}

	// Invalid strategy should fail
	err = service.SetActiveStrategy(ctx, userID, StrategyName("invalid"))
	if err == nil {
		t.Error("should fail with invalid strategy")
	}
}

// TestSettingsService_ResetToDefaults tests reset functionality.
func TestSettingsService_ResetToDefaults(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-6"

	// Customize settings
	settings := DefaultDecisionEngineSettings()
	settings.ActiveStrategy = StrategyBreakout
	settings.Strategies[StrategyTrendFollowing].Entry.MinScore = 99.0
	service.SaveSettings(ctx, userID, settings)

	// Reset to defaults
	err := service.ResetToDefaults(ctx, userID)
	if err != nil {
		t.Fatalf("failed to reset: %v", err)
	}

	// Verify reset
	retrieved, _ := service.GetSettings(ctx, userID)
	if retrieved.ActiveStrategy != StrategyTrendFollowing {
		t.Error("active strategy should be reset to default")
	}
	if retrieved.Strategies[StrategyTrendFollowing].Entry.MinScore == 99.0 {
		t.Error("entry min score should be reset")
	}
}

// TestSettingsService_EnableDisableStrategy tests strategy enable/disable.
func TestSettingsService_EnableDisableStrategy(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-7"

	// First save defaults to establish the user
	defaults := DefaultDecisionEngineSettings()
	service.SaveSettings(ctx, userID, defaults)

	// Disable a non-active strategy
	err := service.DisableStrategy(ctx, userID, StrategyBreakout)
	if err != nil {
		t.Fatalf("failed to disable strategy: %v", err)
	}

	// Verify disabled
	strategy, _ := service.GetStrategySettings(ctx, userID, StrategyBreakout)
	if strategy.Enabled {
		t.Error("strategy should be disabled")
	}

	// Cannot disable active strategy
	err = service.DisableStrategy(ctx, userID, StrategyTrendFollowing)
	if err == nil {
		t.Error("should not be able to disable active strategy")
	}

	// Re-enable
	err = service.EnableStrategy(ctx, userID, StrategyBreakout)
	if err != nil {
		t.Fatalf("failed to enable strategy: %v", err)
	}

	strategy, _ = service.GetStrategySettings(ctx, userID, StrategyBreakout)
	if !strategy.Enabled {
		t.Error("strategy should be enabled")
	}
}

// TestSettingsService_GetEnabledStrategies tests getting enabled strategies.
func TestSettingsService_GetEnabledStrategies(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-8"

	// All strategies enabled by default
	enabled, err := service.GetEnabledStrategies(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get enabled strategies: %v", err)
	}
	if len(enabled) != 4 {
		t.Errorf("expected 4 enabled strategies, got %d", len(enabled))
	}

	// Disable one and save
	defaults := DefaultDecisionEngineSettings()
	defaults.Strategies[StrategyRangeTrading].Enabled = false
	service.SaveSettings(ctx, userID, defaults)

	enabled, _ = service.GetEnabledStrategies(ctx, userID)
	if len(enabled) != 3 {
		t.Errorf("expected 3 enabled strategies, got %d", len(enabled))
	}
}

// TestSettingsService_GetScoringConfigForUser tests scoring config retrieval.
func TestSettingsService_GetScoringConfigForUser(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-9"

	// Get scoring config
	scoring, err := service.GetScoringConfigForUser(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get scoring config: %v", err)
	}
	if scoring == nil {
		t.Fatal("scoring config should not be nil")
	}

	// Verify it matches trend following defaults
	expected := DefaultScoringConfig()
	if scoring.TechnicalWeight != expected.TechnicalWeight {
		t.Error("technical weight should match default")
	}
}

// TestSettingsService_GetBlockingConfigForUser tests blocking config retrieval.
func TestSettingsService_GetBlockingConfigForUser(t *testing.T) {
	service := NewSettingsService(nil)

	ctx := context.Background()
	userID := "test-user-10"

	// Get blocking config
	blocking, err := service.GetBlockingConfigForUser(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get blocking config: %v", err)
	}
	if blocking == nil {
		t.Fatal("blocking config should not be nil")
	}

	// Verify it matches trend following defaults
	expected := DefaultBlockingConfig()
	if blocking.ADXMinimum != expected.ADXMinimum {
		t.Error("ADX minimum should match default")
	}
}

// TestSettingsService_CacheCount tests cache counting.
func TestSettingsService_CacheCount(t *testing.T) {
	service := NewSettingsService(nil)
	ctx := context.Background()

	// Initially empty
	if service.CacheCount() != 0 {
		t.Error("cache should be empty initially")
	}

	// Add some users
	for i := 1; i <= 3; i++ {
		userID := string(rune('0'+i)) + "-user"
		settings := DefaultDecisionEngineSettings()
		service.SaveSettings(ctx, userID, settings)
	}

	if service.CacheCount() != 3 {
		t.Errorf("expected 3 cached users, got %d", service.CacheCount())
	}
}

// TestSettingsService_InvalidateCache tests cache invalidation.
func TestSettingsService_InvalidateCache(t *testing.T) {
	service := NewSettingsService(nil)
	ctx := context.Background()
	userID := "test-user-11"

	// Save settings
	settings := DefaultDecisionEngineSettings()
	settings.ActiveStrategy = StrategyBreakout
	service.SaveSettings(ctx, userID, settings)

	// Verify cached
	retrieved, _ := service.GetSettings(ctx, userID)
	if retrieved.ActiveStrategy != StrategyBreakout {
		t.Fatal("settings should be cached")
	}

	// Invalidate
	service.InvalidateCache(userID)

	// Cache miss - should get defaults
	retrieved, _ = service.GetSettings(ctx, userID)
	if retrieved.ActiveStrategy != StrategyTrendFollowing {
		t.Error("should get defaults after cache invalidation (no Redis)")
	}
}

// TestSettingsService_DeleteSettings tests settings deletion.
func TestSettingsService_DeleteSettings(t *testing.T) {
	service := NewSettingsService(nil)
	ctx := context.Background()
	userID := "test-user-12"

	// Save settings
	settings := DefaultDecisionEngineSettings()
	service.SaveSettings(ctx, userID, settings)

	// Delete
	err := service.DeleteSettings(ctx, userID)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify deleted from cache
	if service.CacheCount() != 0 {
		t.Error("cache should be empty after delete")
	}

	// Empty userID should fail
	err = service.DeleteSettings(ctx, "")
	if err == nil {
		t.Error("should fail with empty userID")
	}
}

// TestSettingsService_CreateSnapshot tests snapshot creation.
func TestSettingsService_CreateSnapshot(t *testing.T) {
	service := NewSettingsService(nil)
	ctx := context.Background()
	userID := "test-user-13"

	// Create snapshot
	before := time.Now()
	snapshot, err := service.CreateSnapshot(ctx, userID)
	after := time.Now()

	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}
	if snapshot == nil {
		t.Fatal("snapshot should not be nil")
	}
	if snapshot.UserID != userID {
		t.Error("snapshot userID mismatch")
	}
	if snapshot.Settings == nil {
		t.Error("snapshot settings should not be nil")
	}
	if snapshot.Timestamp.Before(before) || snapshot.Timestamp.After(after) {
		t.Error("snapshot timestamp should be recent")
	}
}

// TestDecisionSettingsKey tests Redis key generation.
func TestDecisionSettingsKey(t *testing.T) {
	testCases := []struct {
		userID   string
		expected string
	}{
		{"user123", "decision:settings:user123"},
		{"test-user", "decision:settings:test-user"},
		{"user_with_underscore", "decision:settings:user_with_underscore"},
	}

	for _, tc := range testCases {
		key := DecisionSettingsKey(tc.userID)
		if key != tc.expected {
			t.Errorf("expected key %s, got %s", tc.expected, key)
		}
	}
}

// TestScoringConfigIntegration tests that ScoringConfig from scoring.go integrates properly.
func TestScoringConfigIntegration(t *testing.T) {
	// Verify ScoringConfig used in strategy settings
	settings := DefaultTrendFollowingSettings()

	if settings.Scoring == nil {
		t.Fatal("scoring should not be nil")
	}

	// Verify it's the same type from scoring.go
	scoringClone := settings.Scoring.Clone()
	if scoringClone.TechnicalWeight != settings.Scoring.TechnicalWeight {
		t.Error("ScoringConfig Clone should work correctly")
	}

	// Verify Validate works
	if err := settings.Scoring.Validate(); err != nil {
		t.Errorf("ScoringConfig Validate should pass: %v", err)
	}
}

// TestBlockingConfigIntegration tests that BlockingConfig from blocking.go integrates properly.
func TestBlockingConfigIntegration(t *testing.T) {
	// Verify BlockingConfig used in strategy settings
	settings := DefaultMeanReversionSettings()

	if settings.Blocking == nil {
		t.Fatal("blocking should not be nil")
	}

	// Verify it's the same type from blocking.go
	blockingClone := settings.Blocking.Clone()
	if blockingClone.ADXMinimum != settings.Blocking.ADXMinimum {
		t.Error("BlockingConfig Clone should work correctly")
	}

	// Verify Validate (clamping) works
	settings.Blocking.Validate()
	// After validation, values should be in valid ranges
	if settings.Blocking.ADXMinimum < 0 || settings.Blocking.ADXMinimum > 100 {
		t.Error("ADXMinimum should be clamped to valid range")
	}
}
