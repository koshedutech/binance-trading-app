// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.23: Decision Engine Settings Service
package decision

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// SettingsService manages decision engine settings for all users.
// It provides in-memory caching with Redis persistence following the
// existing project patterns from settings_cache_service.go.
type SettingsService struct {
	// settings stores in-memory cache of user settings.
	settings map[string]*DecisionEngineSettings
	// settingsMu protects concurrent access to settings map.
	settingsMu sync.RWMutex
	// redis is the Redis client for persistence (optional).
	redis *redis.Client
	// sm is the StateManager for integration (optional).
	sm *StateManager
}

// NewSettingsService creates a new SettingsService.
// The StateManager is optional and used for integration with other decision engine components.
func NewSettingsService(sm *StateManager) *SettingsService {
	ss := &SettingsService{
		settings: make(map[string]*DecisionEngineSettings),
		sm:       sm,
	}

	// If StateManager has a Redis client, use it for persistence
	if sm != nil && sm.client != nil {
		ss.redis = sm.client
	}

	return ss
}

// NewSettingsServiceWithRedis creates a SettingsService with explicit Redis client.
// Use this when you need Redis persistence without a StateManager.
func NewSettingsServiceWithRedis(client *redis.Client) *SettingsService {
	return &SettingsService{
		settings: make(map[string]*DecisionEngineSettings),
		redis:    client,
	}
}

// GetSettings retrieves the decision engine settings for a user.
// It checks in-memory cache first, then Redis, then returns defaults if not found.
func (ss *SettingsService) GetSettings(ctx context.Context, userID string) (*DecisionEngineSettings, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	// Check in-memory cache first
	ss.settingsMu.RLock()
	if settings, exists := ss.settings[userID]; exists {
		ss.settingsMu.RUnlock()
		return settings.Clone(), nil
	}
	ss.settingsMu.RUnlock()

	// Try to load from Redis if available
	if ss.redis != nil {
		settings, err := ss.loadFromRedis(ctx, userID)
		if err == nil && settings != nil {
			// Cache in memory
			ss.settingsMu.Lock()
			ss.settings[userID] = settings
			ss.settingsMu.Unlock()
			return settings.Clone(), nil
		}
		// Log Redis errors but don't fail - fall through to defaults
		if err != nil && err != redis.Nil {
			log.Printf("[DECISION-SETTINGS] Redis load failed for user %s: %v", userID, err)
		}
	}

	// Return default settings (but don't persist yet - wait for SaveSettings)
	defaults := DefaultDecisionEngineSettings()
	return defaults, nil
}

// SaveSettings persists the decision engine settings for a user.
// It writes to both in-memory cache and Redis (if available).
func (ss *SettingsService) SaveSettings(ctx context.Context, userID string, settings *DecisionEngineSettings) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	// Validate before saving
	if err := settings.Validate(); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	// Update timestamp
	settings.UpdateTimestamp()

	// Clone for storage to prevent external modification
	storedSettings := settings.Clone()

	// Update in-memory cache
	ss.settingsMu.Lock()
	ss.settings[userID] = storedSettings
	ss.settingsMu.Unlock()

	// Persist to Redis if available
	if ss.redis != nil {
		if err := ss.saveToRedis(ctx, userID, storedSettings); err != nil {
			log.Printf("[DECISION-SETTINGS] Redis save failed for user %s: %v", userID, err)
			// Don't fail - in-memory is updated and will be persisted on next attempt
		}
	}

	return nil
}

// GetStrategySettings retrieves settings for a specific strategy for a user.
func (ss *SettingsService) GetStrategySettings(ctx context.Context, userID string, strategy StrategyName) (*StrategySettings, error) {
	if !strategy.IsValid() {
		return nil, fmt.Errorf("invalid strategy: %s", strategy)
	}

	settings, err := ss.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	strategySettings := settings.GetStrategy(strategy)
	if strategySettings == nil {
		// Return default for this strategy
		return ss.getDefaultStrategySettings(strategy), nil
	}

	return strategySettings.Clone(), nil
}

// UpdateStrategySettings updates settings for a specific strategy.
func (ss *SettingsService) UpdateStrategySettings(ctx context.Context, userID string, strategy StrategyName, strategySettings *StrategySettings) error {
	if !strategy.IsValid() {
		return fmt.Errorf("invalid strategy: %s", strategy)
	}
	if strategySettings == nil {
		return fmt.Errorf("strategy settings cannot be nil")
	}
	if strategySettings.Name != strategy {
		return fmt.Errorf("strategy name mismatch: expected %s, got %s", strategy, strategySettings.Name)
	}

	// Validate the strategy settings
	if err := strategySettings.Validate(); err != nil {
		return fmt.Errorf("invalid strategy settings: %w", err)
	}

	// Get current settings (will create defaults if not exists)
	settings, err := ss.GetSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Update the strategy
	settings.SetStrategy(strategySettings.Clone())

	// Save back
	return ss.SaveSettings(ctx, userID, settings)
}

// GetActiveStrategy returns the currently active strategy for a user.
func (ss *SettingsService) GetActiveStrategy(ctx context.Context, userID string) (*StrategySettings, error) {
	settings, err := ss.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	activeSettings := settings.GetActiveStrategySettings()
	if activeSettings == nil {
		// Fallback to trend following
		return DefaultTrendFollowingSettings(), nil
	}

	return activeSettings.Clone(), nil
}

// SetActiveStrategy changes the active strategy for a user.
func (ss *SettingsService) SetActiveStrategy(ctx context.Context, userID string, strategy StrategyName) error {
	if !strategy.IsValid() {
		return fmt.Errorf("invalid strategy: %s", strategy)
	}

	settings, err := ss.GetSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Check that the strategy exists
	if settings.GetStrategy(strategy) == nil {
		return fmt.Errorf("strategy %s not found in user settings", strategy)
	}

	// Check that the strategy is enabled
	strategySettings := settings.GetStrategy(strategy)
	if !strategySettings.Enabled {
		return fmt.Errorf("strategy %s is not enabled", strategy)
	}

	settings.ActiveStrategy = strategy
	return ss.SaveSettings(ctx, userID, settings)
}

// ResetToDefaults resets all settings for a user to default values.
func (ss *SettingsService) ResetToDefaults(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	defaults := DefaultDecisionEngineSettings()
	return ss.SaveSettings(ctx, userID, defaults)
}

// ResetStrategyToDefaults resets a specific strategy to its default values.
func (ss *SettingsService) ResetStrategyToDefaults(ctx context.Context, userID string, strategy StrategyName) error {
	if !strategy.IsValid() {
		return fmt.Errorf("invalid strategy: %s", strategy)
	}

	defaultSettings := ss.getDefaultStrategySettings(strategy)
	if defaultSettings == nil {
		return fmt.Errorf("no defaults found for strategy: %s", strategy)
	}

	return ss.UpdateStrategySettings(ctx, userID, strategy, defaultSettings)
}

// DeleteSettings removes all settings for a user from cache and Redis.
func (ss *SettingsService) DeleteSettings(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	// Remove from in-memory cache
	ss.settingsMu.Lock()
	delete(ss.settings, userID)
	ss.settingsMu.Unlock()

	// Remove from Redis if available
	if ss.redis != nil {
		key := DecisionSettingsKey(userID)
		if err := ss.redis.Del(ctx, key).Err(); err != nil && err != redis.Nil {
			log.Printf("[DECISION-SETTINGS] Redis delete failed for user %s: %v", userID, err)
			// Don't fail - in-memory is cleared
		}
	}

	return nil
}

// EnableStrategy enables a strategy for a user.
func (ss *SettingsService) EnableStrategy(ctx context.Context, userID string, strategy StrategyName) error {
	strategySettings, err := ss.GetStrategySettings(ctx, userID, strategy)
	if err != nil {
		return err
	}

	strategySettings.Enabled = true
	return ss.UpdateStrategySettings(ctx, userID, strategy, strategySettings)
}

// DisableStrategy disables a strategy for a user.
// If the strategy is currently active, returns an error.
func (ss *SettingsService) DisableStrategy(ctx context.Context, userID string, strategy StrategyName) error {
	settings, err := ss.GetSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Cannot disable the active strategy
	if settings.ActiveStrategy == strategy {
		return fmt.Errorf("cannot disable active strategy %s - change active strategy first", strategy)
	}

	strategySettings := settings.GetStrategy(strategy)
	if strategySettings == nil {
		return fmt.Errorf("strategy %s not found", strategy)
	}

	strategySettings.Enabled = false
	settings.SetStrategy(strategySettings)

	return ss.SaveSettings(ctx, userID, settings)
}

// GetEnabledStrategies returns a list of enabled strategies for a user.
func (ss *SettingsService) GetEnabledStrategies(ctx context.Context, userID string) ([]StrategyName, error) {
	settings, err := ss.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	var enabled []StrategyName
	for name, strategy := range settings.Strategies {
		if strategy.Enabled {
			enabled = append(enabled, name)
		}
	}

	return enabled, nil
}

// LoadFromRedis loads settings from Redis for a specific user.
// This is called during cache miss or initialization.
func (ss *SettingsService) LoadFromRedis(ctx context.Context, userID string) error {
	if ss.redis == nil {
		return fmt.Errorf("Redis client not available")
	}

	settings, err := ss.loadFromRedis(ctx, userID)
	if err != nil {
		return err
	}

	if settings != nil {
		ss.settingsMu.Lock()
		ss.settings[userID] = settings
		ss.settingsMu.Unlock()
	}

	return nil
}

// InvalidateCache removes a user's settings from in-memory cache.
// The next GetSettings call will reload from Redis or use defaults.
func (ss *SettingsService) InvalidateCache(userID string) {
	ss.settingsMu.Lock()
	delete(ss.settings, userID)
	ss.settingsMu.Unlock()
}

// CacheCount returns the number of users with cached settings.
func (ss *SettingsService) CacheCount() int {
	ss.settingsMu.RLock()
	defer ss.settingsMu.RUnlock()
	return len(ss.settings)
}

// GetScoringConfigForUser returns the scoring config for the user's active strategy.
// This integrates with the existing ScoringConfig from scoring.go.
func (ss *SettingsService) GetScoringConfigForUser(ctx context.Context, userID string) (*ScoringConfig, error) {
	activeStrategy, err := ss.GetActiveStrategy(ctx, userID)
	if err != nil {
		return nil, err
	}

	if activeStrategy.Scoring != nil {
		return activeStrategy.Scoring.Clone(), nil
	}

	return DefaultScoringConfig(), nil
}

// GetBlockingConfigForUser returns the blocking config for the user's active strategy.
// This integrates with the existing BlockingConfig from blocking.go.
func (ss *SettingsService) GetBlockingConfigForUser(ctx context.Context, userID string) (*BlockingConfig, error) {
	activeStrategy, err := ss.GetActiveStrategy(ctx, userID)
	if err != nil {
		return nil, err
	}

	if activeStrategy.Blocking != nil {
		return activeStrategy.Blocking.Clone(), nil
	}

	return DefaultBlockingConfig(), nil
}

// Internal helper methods

// loadFromRedis retrieves settings from Redis.
func (ss *SettingsService) loadFromRedis(ctx context.Context, userID string) (*DecisionEngineSettings, error) {
	key := DecisionSettingsKey(userID)

	data, err := ss.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get settings from Redis: %w", err)
	}

	settings, err := FromJSONSettings([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse settings from Redis: %w", err)
	}

	return settings, nil
}

// saveToRedis persists settings to Redis.
func (ss *SettingsService) saveToRedis(ctx context.Context, userID string, settings *DecisionEngineSettings) error {
	key := DecisionSettingsKey(userID)

	data, err := settings.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize settings: %w", err)
	}

	// No TTL - settings are persistent until explicitly deleted
	if err := ss.redis.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save settings to Redis: %w", err)
	}

	return nil
}

// getDefaultStrategySettings returns the default settings for a strategy.
func (ss *SettingsService) getDefaultStrategySettings(strategy StrategyName) *StrategySettings {
	switch strategy {
	case StrategyTrendFollowing:
		return DefaultTrendFollowingSettings()
	case StrategyMeanReversion:
		return DefaultMeanReversionSettings()
	case StrategyBreakout:
		return DefaultBreakoutSettings()
	case StrategyRangeTrading:
		return DefaultRangeTradingSettings()
	default:
		return nil
	}
}

// SettingsSnapshot represents a point-in-time snapshot of settings for audit/comparison.
type SettingsSnapshot struct {
	UserID    string                   `json:"user_id"`
	Settings  *DecisionEngineSettings  `json:"settings"`
	Timestamp time.Time                `json:"timestamp"`
}

// CreateSnapshot creates a snapshot of current settings for a user.
func (ss *SettingsService) CreateSnapshot(ctx context.Context, userID string) (*SettingsSnapshot, error) {
	settings, err := ss.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &SettingsSnapshot{
		UserID:    userID,
		Settings:  settings,
		Timestamp: time.Now(),
	}, nil
}
