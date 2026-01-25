// Package entrydecision provides tests for the Strategy Reader.
// Epic 14: Chain Trading System - Story 14.11: Strategy Reader & Matcher
package entrydecision

import (
	"context"
	"encoding/json"
	"testing"

	"binance-trading-bot/internal/database"
)

// ============================================================================
// MOCK STRATEGY REPOSITORY
// ============================================================================

// MockStrategyRepository implements StrategyRepository for testing.
type MockStrategyRepository struct {
	EnabledStrategies        []database.EnabledSubStrategy
	EnabledStrategiesForMode map[string][]database.EnabledSubStrategy
	StrategyGroups           map[string]*database.StrategyGroupSettings
	SubStrategies            map[string]*database.SubStrategySettings

	// Error simulation
	GetEnabledStrategiesErr error
	GetGroupSettingsErr     error
	GetSubStrategyErr       error
}

// NewMockStrategyRepository creates a new mock repository with default data.
func NewMockStrategyRepository() *MockStrategyRepository {
	return &MockStrategyRepository{
		EnabledStrategiesForMode: make(map[string][]database.EnabledSubStrategy),
		StrategyGroups:           make(map[string]*database.StrategyGroupSettings),
		SubStrategies:            make(map[string]*database.SubStrategySettings),
	}
}

func (m *MockStrategyRepository) GetEnabledStrategies(ctx context.Context, userID string) ([]database.EnabledSubStrategy, error) {
	if m.GetEnabledStrategiesErr != nil {
		return nil, m.GetEnabledStrategiesErr
	}
	return m.EnabledStrategies, nil
}

func (m *MockStrategyRepository) GetEnabledStrategiesForMode(ctx context.Context, userID, mode string) ([]database.EnabledSubStrategy, error) {
	if m.GetEnabledStrategiesErr != nil {
		return nil, m.GetEnabledStrategiesErr
	}
	return m.EnabledStrategiesForMode[mode], nil
}

func (m *MockStrategyRepository) GetStrategyGroupSettings(ctx context.Context, userID, mode, group string) (*database.StrategyGroupSettings, error) {
	if m.GetGroupSettingsErr != nil {
		return nil, m.GetGroupSettingsErr
	}
	key := mode + ":" + group
	return m.StrategyGroups[key], nil
}

func (m *MockStrategyRepository) GetSubStrategySettings(ctx context.Context, userID, mode, group, subStrategy string) (*database.SubStrategySettings, error) {
	if m.GetSubStrategyErr != nil {
		return nil, m.GetSubStrategyErr
	}
	key := mode + ":" + group + ":" + subStrategy
	return m.SubStrategies[key], nil
}

// ============================================================================
// STRATEGY READER TESTS
// ============================================================================

func TestDefaultStrategyReader_GetEnabledStrategies(t *testing.T) {
	t.Run("returns enabled strategies", func(t *testing.T) {
		mockRepo := NewMockStrategyRepository()
		mockRepo.EnabledStrategies = []database.EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
			{Mode: "swing", StrategyGroup: "trending", SubStrategy: "trend_following"},
		}
		mockRepo.StrategyGroups["scalp:breakout"] = &database.StrategyGroupSettings{
			Timeframe: "5m",
		}
		mockRepo.StrategyGroups["swing:trending"] = &database.StrategyGroupSettings{
			Timeframe: "15m",
		}

		reader := NewDefaultStrategyReader(mockRepo)
		ctx := context.Background()

		strategies, err := reader.GetEnabledStrategies(ctx, "user123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(strategies) != 2 {
			t.Errorf("expected 2 strategies, got %d", len(strategies))
		}

		// Check first strategy
		if strategies[0].Mode != "scalp" {
			t.Errorf("expected mode 'scalp', got '%s'", strategies[0].Mode)
		}
		if strategies[0].StrategyGroup != "breakout" {
			t.Errorf("expected strategy group 'breakout', got '%s'", strategies[0].StrategyGroup)
		}
		if strategies[0].SubStrategy != "ravindra_volume_imbalance" {
			t.Errorf("expected sub-strategy 'ravindra_volume_imbalance', got '%s'", strategies[0].SubStrategy)
		}
		if strategies[0].Timeframe != "5m" {
			t.Errorf("expected timeframe '5m', got '%s'", strategies[0].Timeframe)
		}
	})

	t.Run("returns error for empty userID", func(t *testing.T) {
		mockRepo := NewMockStrategyRepository()
		reader := NewDefaultStrategyReader(mockRepo)
		ctx := context.Background()

		_, err := reader.GetEnabledStrategies(ctx, "")
		if err == nil {
			t.Error("expected error for empty userID")
		}
	})

	t.Run("returns empty list when no strategies enabled", func(t *testing.T) {
		mockRepo := NewMockStrategyRepository()
		mockRepo.EnabledStrategies = []database.EnabledSubStrategy{}

		reader := NewDefaultStrategyReader(mockRepo)
		ctx := context.Background()

		strategies, err := reader.GetEnabledStrategies(ctx, "user123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(strategies) != 0 {
			t.Errorf("expected 0 strategies, got %d", len(strategies))
		}
	})

	t.Run("uses default timeframe when group settings not found", func(t *testing.T) {
		mockRepo := NewMockStrategyRepository()
		mockRepo.EnabledStrategies = []database.EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
		}
		// No strategy group settings configured

		reader := NewDefaultStrategyReader(mockRepo)
		ctx := context.Background()

		strategies, err := reader.GetEnabledStrategies(ctx, "user123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(strategies) != 1 {
			t.Fatalf("expected 1 strategy, got %d", len(strategies))
		}

		// Should use default timeframe for scalp mode
		if strategies[0].Timeframe != "5m" {
			t.Errorf("expected default timeframe '5m' for scalp, got '%s'", strategies[0].Timeframe)
		}
	})
}

func TestDefaultStrategyReader_GetEnabledStrategiesForMode(t *testing.T) {
	t.Run("returns strategies for specific mode", func(t *testing.T) {
		mockRepo := NewMockStrategyRepository()
		mockRepo.EnabledStrategiesForMode["scalp"] = []database.EnabledSubStrategy{
			{Mode: "scalp", StrategyGroup: "breakout", SubStrategy: "ravindra_volume_imbalance"},
			{Mode: "scalp", StrategyGroup: "trending", SubStrategy: "trend_following"},
		}
		mockRepo.StrategyGroups["scalp:breakout"] = &database.StrategyGroupSettings{
			Timeframe: "5m",
		}
		mockRepo.StrategyGroups["scalp:trending"] = &database.StrategyGroupSettings{
			Timeframe: "5m",
		}

		reader := NewDefaultStrategyReader(mockRepo)
		ctx := context.Background()

		strategies, err := reader.GetEnabledStrategiesForMode(ctx, "user123", "scalp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(strategies) != 2 {
			t.Errorf("expected 2 strategies, got %d", len(strategies))
		}

		for _, s := range strategies {
			if s.Mode != "scalp" {
				t.Errorf("expected mode 'scalp', got '%s'", s.Mode)
			}
		}
	})

	t.Run("returns error for empty mode", func(t *testing.T) {
		mockRepo := NewMockStrategyRepository()
		reader := NewDefaultStrategyReader(mockRepo)
		ctx := context.Background()

		_, err := reader.GetEnabledStrategiesForMode(ctx, "user123", "")
		if err == nil {
			t.Error("expected error for empty mode")
		}
	})
}

func TestDefaultStrategyReader_GetStrategyConfig(t *testing.T) {
	t.Run("returns full strategy config", func(t *testing.T) {
		mockRepo := NewMockStrategyRepository()
		mockRepo.StrategyGroups["scalp:breakout"] = &database.StrategyGroupSettings{
			Timeframe:           "5m",
			PositionSizePercent: 2.5,
			MaxLeverage:         10,
			MaxPositions:        5,
			MinVolumeUSDT:       1000000,
		}

		settings := map[string]interface{}{
			"volume_spike_multiplier": 2.0,
			"consolidation_candles":   4,
		}
		settingsJSON, _ := json.Marshal(settings)
		mockRepo.SubStrategies["scalp:breakout:ravindra_volume_imbalance"] = &database.SubStrategySettings{
			Settings: settingsJSON,
		}

		reader := NewDefaultStrategyReader(mockRepo)
		ctx := context.Background()

		strategy := EnabledSubStrategy{
			Mode:          "scalp",
			StrategyGroup: "breakout",
			SubStrategy:   "ravindra_volume_imbalance",
		}

		config, err := reader.GetStrategyConfig(ctx, "user123", strategy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.Timeframe != "5m" {
			t.Errorf("expected timeframe '5m', got '%s'", config.Timeframe)
		}
		if config.PositionSizePercent != 2.5 {
			t.Errorf("expected position size 2.5, got %f", config.PositionSizePercent)
		}
		if config.MaxLeverage != 10 {
			t.Errorf("expected max leverage 10, got %d", config.MaxLeverage)
		}
		if config.MaxPositions != 5 {
			t.Errorf("expected max positions 5, got %d", config.MaxPositions)
		}
	})

	t.Run("uses defaults when group settings not found", func(t *testing.T) {
		mockRepo := NewMockStrategyRepository()
		// No strategy group or sub-strategy settings configured

		reader := NewDefaultStrategyReader(mockRepo)
		ctx := context.Background()

		strategy := EnabledSubStrategy{
			Mode:          "swing",
			StrategyGroup: "trending",
			SubStrategy:   "trend_following",
		}

		config, err := reader.GetStrategyConfig(ctx, "user123", strategy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should use defaults
		if config.Timeframe != "15m" { // Default for swing mode
			t.Errorf("expected default timeframe '15m' for swing, got '%s'", config.Timeframe)
		}
		if config.PositionSizePercent != 2.0 {
			t.Errorf("expected default position size 2.0, got %f", config.PositionSizePercent)
		}
		if config.MaxLeverage != 5 {
			t.Errorf("expected default max leverage 5, got %d", config.MaxLeverage)
		}
	})
}

// ============================================================================
// STRATEGY TYPE TESTS
// ============================================================================

func TestGetStrategyType(t *testing.T) {
	tests := []struct {
		name          string
		strategyGroup string
		subStrategy   string
		expected      StrategyType
	}{
		{
			name:          "volume imbalance is pattern-based",
			strategyGroup: "breakout",
			subStrategy:   "ravindra_volume_imbalance",
			expected:      StrategyTypePattern,
		},
		{
			name:          "breakout group is pattern-based",
			strategyGroup: "breakout",
			subStrategy:   "classic_breakout",
			expected:      StrategyTypePattern,
		},
		{
			name:          "trending group is score-based",
			strategyGroup: "trending",
			subStrategy:   "trend_following",
			expected:      StrategyTypeScore,
		},
		{
			name:          "range group is score-based",
			strategyGroup: "range",
			subStrategy:   "range_trading",
			expected:      StrategyTypeScore,
		},
		{
			name:          "volatile group is pattern-based",
			strategyGroup: "volatile",
			subStrategy:   "volatility_breakout",
			expected:      StrategyTypePattern,
		},
		{
			name:          "unknown group defaults to score-based",
			strategyGroup: "unknown",
			subStrategy:   "unknown_strategy",
			expected:      StrategyTypeScore,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStrategyType(tt.strategyGroup, tt.subStrategy)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// HELPER FUNCTION TESTS
// ============================================================================

func TestIsValidMode(t *testing.T) {
	validModes := []string{"scalp", "swing", "position", "ultra_fast"}
	invalidModes := []string{"invalid", "day_trading", ""}

	for _, mode := range validModes {
		if !IsValidMode(mode) {
			t.Errorf("expected '%s' to be valid", mode)
		}
	}

	for _, mode := range invalidModes {
		if IsValidMode(mode) {
			t.Errorf("expected '%s' to be invalid", mode)
		}
	}
}

func TestIsValidStrategyGroup(t *testing.T) {
	validGroups := []string{"breakout", "trending", "range", "volatile"}
	invalidGroups := []string{"invalid", "momentum", ""}

	for _, group := range validGroups {
		if !IsValidStrategyGroup(group) {
			t.Errorf("expected '%s' to be valid", group)
		}
	}

	for _, group := range invalidGroups {
		if IsValidStrategyGroup(group) {
			t.Errorf("expected '%s' to be invalid", group)
		}
	}
}

func TestGetDefaultTimeframe(t *testing.T) {
	tests := []struct {
		mode     string
		expected string
	}{
		{"ultra_fast", "1m"},
		{"scalp", "5m"},
		{"swing", "15m"},
		{"position", "1h"},
		{"unknown", "5m"}, // Default fallback
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			result := getDefaultTimeframe(tt.mode)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestParseSubStrategySettings(t *testing.T) {
	t.Run("parses valid JSON", func(t *testing.T) {
		data := []byte(`{"key": "value", "number": 42}`)
		result := parseSubStrategySettings(data)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result["key"] != "value" {
			t.Errorf("expected key='value', got '%v'", result["key"])
		}
		if result["number"].(float64) != 42 {
			t.Errorf("expected number=42, got '%v'", result["number"])
		}
	})

	t.Run("returns nil for empty data", func(t *testing.T) {
		result := parseSubStrategySettings([]byte{})
		if result != nil {
			t.Error("expected nil for empty data")
		}
	})

	t.Run("returns nil for empty JSON object", func(t *testing.T) {
		result := parseSubStrategySettings([]byte("{}"))
		if result != nil {
			t.Error("expected nil for empty JSON object")
		}
	})

	t.Run("returns nil for null JSON", func(t *testing.T) {
		result := parseSubStrategySettings([]byte("null"))
		if result != nil {
			t.Error("expected nil for null JSON")
		}
	})

	t.Run("returns nil for invalid JSON", func(t *testing.T) {
		result := parseSubStrategySettings([]byte("invalid json"))
		if result != nil {
			t.Error("expected nil for invalid JSON")
		}
	})
}
