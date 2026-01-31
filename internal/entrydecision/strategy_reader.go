// Package entrydecision provides the Strategy Reader for the Entry Decision System.
// Epic 14: Chain Trading System - Story 14.11: Strategy Reader & Matcher
package entrydecision

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"binance-trading-bot/internal/database"
)

// ============================================================================
// STRATEGY READER INTERFACE & TYPES
// ============================================================================

// StrategyReader reads enabled strategies from the database/cache.
// It provides the list of enabled sub-strategies for a user that need
// to be evaluated by the Entry Decision System.
type StrategyReader interface {
	// GetEnabledStrategies returns all enabled sub-strategies for a user.
	// Each strategy includes mode, strategy group, sub-strategy, and timeframe.
	GetEnabledStrategies(ctx context.Context, userID string) ([]EnabledSubStrategy, error)

	// GetEnabledStrategiesForMode returns enabled sub-strategies for a specific mode.
	GetEnabledStrategiesForMode(ctx context.Context, userID, mode string) ([]EnabledSubStrategy, error)

	// GetStrategyConfig returns the full configuration for a specific sub-strategy.
	GetStrategyConfig(ctx context.Context, userID string, strategy EnabledSubStrategy) (*StrategyConfig, error)
}

// EnabledSubStrategy is a compact representation of an enabled strategy.
// This aligns with database.EnabledSubStrategy but adds timeframe information.
type EnabledSubStrategy struct {
	Mode          string `json:"mode"`           // "scalp", "swing", "position", "ultra_fast"
	StrategyGroup string `json:"strategy_group"` // "breakout", "trending", "range", "volatile"
	SubStrategy   string `json:"sub_strategy"`   // "ravindra_volume_imbalance", "classic_breakout"
	Timeframe     string `json:"timeframe"`      // Primary timeframe from strategy group settings
}

// StrategyConfig holds the full configuration for a sub-strategy.
// This combines data from strategy group and sub-strategy settings.
type StrategyConfig struct {
	EnabledSubStrategy

	// From StrategyGroupSettings
	PositionSizePercent float64 `json:"position_size_percent"`
	MaxLeverage         int     `json:"max_leverage"`
	MaxPositions        int     `json:"max_positions"`
	MinVolumeUSDT       float64 `json:"min_volume_usdt"`

	// Sub-strategy specific settings (parsed from JSON)
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// StrategyType classification for pattern vs score-based strategies.
const (
	// StrategyGroupBreakout includes pattern-based strategies like Volume Imbalance.
	StrategyGroupBreakout = "breakout"

	// StrategyGroupTrending includes score-based strategies like Trend Following.
	StrategyGroupTrending = "trending"

	// StrategyGroupRange includes range-trading strategies.
	StrategyGroupRange = "range"

	// StrategyGroupVolatile includes volatility-based strategies.
	StrategyGroupVolatile = "volatile"
)

// GetStrategyType returns whether a strategy is pattern-based or score-based.
// Pattern-based strategies use step tracking (e.g., Volume Imbalance).
// Score-based strategies use weighted scoring (e.g., Trend Following).
func GetStrategyType(strategyGroup, subStrategy string) StrategyType {
	// Volume Imbalance is a pattern-based strategy
	if subStrategy == "ravindra_volume_imbalance" {
		return StrategyTypePattern
	}

	// By strategy group:
	// - breakout strategies are typically pattern-based
	// - trending strategies are typically score-based
	// - range strategies are typically score-based
	// - volatile strategies are typically pattern-based
	switch strategyGroup {
	case StrategyGroupBreakout:
		return StrategyTypePattern
	case StrategyGroupTrending, StrategyGroupRange:
		return StrategyTypeScore
	case StrategyGroupVolatile:
		return StrategyTypePattern
	default:
		return StrategyTypeScore
	}
}

// ============================================================================
// STRATEGY REPOSITORY INTERFACE
// ============================================================================

// StrategyRepository defines the database operations needed by the StrategyReader.
// This is a subset of database.Repository methods for dependency injection.
type StrategyRepository interface {
	// GetEnabledStrategies returns all enabled sub-strategies for a user.
	GetEnabledStrategies(ctx context.Context, userID string) ([]database.EnabledSubStrategy, error)

	// GetEnabledStrategiesForMode returns enabled sub-strategies for a specific mode.
	GetEnabledStrategiesForMode(ctx context.Context, userID, mode string) ([]database.EnabledSubStrategy, error)

	// GetStrategyGroupSettings retrieves strategy group settings.
	GetStrategyGroupSettings(ctx context.Context, userID, mode, group string) (*database.StrategyGroupSettings, error)

	// GetSubStrategySettings retrieves sub-strategy settings.
	GetSubStrategySettings(ctx context.Context, userID, mode, group, subStrategy string) (*database.SubStrategySettings, error)
}

// ============================================================================
// DEFAULT STRATEGY READER IMPLEMENTATION
// ============================================================================

// DefaultStrategyReader implements StrategyReader using database repository.
type DefaultStrategyReader struct {
	repo  StrategyRepository
	mu    sync.RWMutex
	cache map[string][]EnabledSubStrategy // userID -> cached strategies
}

// NewDefaultStrategyReader creates a new DefaultStrategyReader.
func NewDefaultStrategyReader(repo StrategyRepository) *DefaultStrategyReader {
	return &DefaultStrategyReader{
		repo:  repo,
		cache: make(map[string][]EnabledSubStrategy),
	}
}

// GetEnabledStrategies returns all enabled sub-strategies for a user.
func (r *DefaultStrategyReader) GetEnabledStrategies(ctx context.Context, userID string) ([]EnabledSubStrategy, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}

	// Get enabled strategies from repository
	dbStrategies, err := r.repo.GetEnabledStrategies(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled strategies: %w", err)
	}

	// Convert and enrich with timeframe information
	result := make([]EnabledSubStrategy, 0, len(dbStrategies))
	for _, dbStrat := range dbStrategies {
		// Get timeframe from strategy group settings
		timeframe := r.getTimeframeForStrategy(ctx, userID, dbStrat.Mode, dbStrat.StrategyGroup)

		result = append(result, EnabledSubStrategy{
			Mode:          dbStrat.Mode,
			StrategyGroup: dbStrat.StrategyGroup,
			SubStrategy:   dbStrat.SubStrategy,
			Timeframe:     timeframe,
		})
	}

	return result, nil
}

// GetEnabledStrategiesForMode returns enabled sub-strategies for a specific mode.
func (r *DefaultStrategyReader) GetEnabledStrategiesForMode(ctx context.Context, userID, mode string) ([]EnabledSubStrategy, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}
	if mode == "" {
		return nil, fmt.Errorf("mode cannot be empty")
	}

	// Get enabled strategies for mode from repository
	dbStrategies, err := r.repo.GetEnabledStrategiesForMode(ctx, userID, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled strategies for mode %s: %w", mode, err)
	}

	// Convert and enrich with timeframe information
	result := make([]EnabledSubStrategy, 0, len(dbStrategies))
	for _, dbStrat := range dbStrategies {
		// Get timeframe from strategy group settings
		timeframe := r.getTimeframeForStrategy(ctx, userID, dbStrat.Mode, dbStrat.StrategyGroup)

		result = append(result, EnabledSubStrategy{
			Mode:          dbStrat.Mode,
			StrategyGroup: dbStrat.StrategyGroup,
			SubStrategy:   dbStrat.SubStrategy,
			Timeframe:     timeframe,
		})
	}

	return result, nil
}

// GetStrategyConfig returns the full configuration for a specific sub-strategy.
func (r *DefaultStrategyReader) GetStrategyConfig(ctx context.Context, userID string, strategy EnabledSubStrategy) (*StrategyConfig, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}

	// Get strategy group settings for position sizing, timeframe, etc.
	groupSettings, err := r.repo.GetStrategyGroupSettings(ctx, userID, strategy.Mode, strategy.StrategyGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy group settings: %w", err)
	}

	// Get sub-strategy specific settings
	subSettings, err := r.repo.GetSubStrategySettings(ctx, userID, strategy.Mode, strategy.StrategyGroup, strategy.SubStrategy)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-strategy settings: %w", err)
	}

	// Build StrategyConfig
	config := &StrategyConfig{
		EnabledSubStrategy: EnabledSubStrategy{
			Mode:          strategy.Mode,
			StrategyGroup: strategy.StrategyGroup,
			SubStrategy:   strategy.SubStrategy,
		},
	}

	// Apply group settings if available
	if groupSettings != nil {
		config.Timeframe = groupSettings.Timeframe
		config.PositionSizePercent = groupSettings.PositionSizePercent
		config.MaxLeverage = groupSettings.MaxLeverage
		config.MaxPositions = groupSettings.MaxPositions
		config.MinVolumeUSDT = groupSettings.MinVolumeUSDT
	} else {
		// Apply defaults
		config.Timeframe = getDefaultTimeframe(strategy.Mode)
		config.PositionSizePercent = 2.0
		config.MaxLeverage = 5
		config.MaxPositions = 3
		config.MinVolumeUSDT = 1000000
	}

	// Parse sub-strategy settings if available
	if subSettings != nil && len(subSettings.Settings) > 0 {
		config.Settings = parseSubStrategySettings(subSettings.Settings)
	}

	return config, nil
}

// getTimeframeForStrategy gets the timeframe for a strategy from user's settings (cache/database).
// User-configured settings take priority over defaults.
func (r *DefaultStrategyReader) getTimeframeForStrategy(ctx context.Context, userID, mode, strategyGroup string) string {
	// Get user's configured timeframe from strategy group settings (Redis cache -> database)
	groupSettings, err := r.repo.GetStrategyGroupSettings(ctx, userID, mode, strategyGroup)
	if err == nil && groupSettings != nil && groupSettings.Timeframe != "" {
		return groupSettings.Timeframe
	}

	// Fall back to mode-based defaults only if user has no configured setting
	return getDefaultTimeframe(mode)
}

// getDefaultTimeframe returns the default timeframe for a mode.
// Updated: Volume Imbalance strategy uses 3m timeframe for scalp/swing modes
// based on Dec 2025 - Jan 2026 backtest results.
func getDefaultTimeframe(mode string) string {
	switch mode {
	case "ultra_fast":
		return "1m"
	case "scalp":
		return "3m" // Volume Imbalance uses 3m for scalp
	case "swing":
		return "3m" // Volume Imbalance uses 3m for swing
	case "position":
		return "1h"
	default:
		return "3m"
	}
}

// parseSubStrategySettings parses JSON settings into a map.
func parseSubStrategySettings(data []byte) map[string]interface{} {
	if len(data) == 0 || string(data) == "{}" || string(data) == "null" {
		return nil
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}
	return settings
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// IsValidMode checks if the mode is valid.
func IsValidMode(mode string) bool {
	validModes := []string{"scalp", "swing", "position", "ultra_fast"}
	for _, valid := range validModes {
		if mode == valid {
			return true
		}
	}
	return false
}

// IsValidStrategyGroup checks if the strategy group is valid.
func IsValidStrategyGroup(group string) bool {
	validGroups := []string{"breakout", "trending", "range", "volatile"}
	for _, valid := range validGroups {
		if group == valid {
			return true
		}
	}
	return false
}

// ============================================================================
// CACHE-BASED STRATEGY READER
// ============================================================================

// CacheSettingsReader defines the cache interface for reading strategy settings.
// This aligns with SettingsCacheReader from autopilot package.
type CacheSettingsReader interface {
	// GetAllStrategiesForMode retrieves all strategy configs for a mode.
	GetAllStrategiesForMode(ctx context.Context, userID string, mode string) (map[string]*database.ModeStrategyConfig, error)
}

// CachedStrategyReader implements StrategyReader using cache-first pattern.
type CachedStrategyReader struct {
	cache CacheSettingsReader
	repo  StrategyRepository
	mu    sync.RWMutex
}

// NewCachedStrategyReader creates a new CachedStrategyReader.
func NewCachedStrategyReader(cache CacheSettingsReader, repo StrategyRepository) *CachedStrategyReader {
	return &CachedStrategyReader{
		cache: cache,
		repo:  repo,
	}
}

// GetEnabledStrategies returns all enabled sub-strategies for a user.
// Uses cache-first pattern for better performance.
func (r *CachedStrategyReader) GetEnabledStrategies(ctx context.Context, userID string) ([]EnabledSubStrategy, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}

	// Collect from all modes
	modes := []string{"scalp", "swing", "position", "ultra_fast"}
	var result []EnabledSubStrategy

	for _, mode := range modes {
		modeStrategies, err := r.GetEnabledStrategiesForMode(ctx, userID, mode)
		if err != nil {
			// Log but continue with other modes
			continue
		}
		result = append(result, modeStrategies...)
	}

	return result, nil
}

// GetEnabledStrategiesForMode returns enabled sub-strategies for a specific mode.
func (r *CachedStrategyReader) GetEnabledStrategiesForMode(ctx context.Context, userID, mode string) ([]EnabledSubStrategy, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}
	if mode == "" {
		return nil, fmt.Errorf("mode cannot be empty")
	}

	// Fall back to repository for sub-strategy data
	dbStrategies, err := r.repo.GetEnabledStrategiesForMode(ctx, userID, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled strategies for mode %s: %w", mode, err)
	}

	// Convert and enrich with timeframe information from user's settings
	result := make([]EnabledSubStrategy, 0, len(dbStrategies))
	for _, dbStrat := range dbStrategies {
		// Get user's configured timeframe from strategy group settings (Redis cache -> database)
		groupSettings, _ := r.repo.GetStrategyGroupSettings(ctx, userID, dbStrat.Mode, dbStrat.StrategyGroup)
		timeframe := getDefaultTimeframe(dbStrat.Mode)
		if groupSettings != nil && groupSettings.Timeframe != "" {
			timeframe = groupSettings.Timeframe
		}

		result = append(result, EnabledSubStrategy{
			Mode:          dbStrat.Mode,
			StrategyGroup: dbStrat.StrategyGroup,
			SubStrategy:   dbStrat.SubStrategy,
			Timeframe:     timeframe,
		})
	}

	return result, nil
}

// GetStrategyConfig returns the full configuration for a specific sub-strategy.
func (r *CachedStrategyReader) GetStrategyConfig(ctx context.Context, userID string, strategy EnabledSubStrategy) (*StrategyConfig, error) {
	// Delegate to repository-based reader for now
	defaultReader := &DefaultStrategyReader{repo: r.repo}
	return defaultReader.GetStrategyConfig(ctx, userID, strategy)
}
