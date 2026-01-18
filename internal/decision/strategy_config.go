// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.6: Strategy Registry
package decision

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// StrategyConfigStore manages strategy configuration storage and retrieval.
// It provides a cache layer for strategy configurations per user with
// fallback to default settings.
type StrategyConfigStore interface {
	// GetStrategyConfig retrieves the configuration for a specific strategy.
	GetStrategyConfig(ctx context.Context, userID string, strategyName StrategyName) (*StrategySettings, error)

	// SetStrategyConfig stores the configuration for a specific strategy.
	SetStrategyConfig(ctx context.Context, userID string, settings *StrategySettings) error

	// GetAllStrategyConfigs retrieves all strategy configurations for a user.
	GetAllStrategyConfigs(ctx context.Context, userID string) (*DecisionEngineSettings, error)

	// SetAllStrategyConfigs stores all strategy configurations for a user.
	SetAllStrategyConfigs(ctx context.Context, userID string, settings *DecisionEngineSettings) error

	// DeleteStrategyConfig removes a strategy configuration.
	DeleteStrategyConfig(ctx context.Context, userID string, strategyName StrategyName) error

	// ResetToDefaults resets a user's strategy configuration to defaults.
	ResetToDefaults(ctx context.Context, userID string) error
}

// Redis key patterns for strategy configuration
const (
	// PrefixStrategyConfig is the Redis key pattern for user strategy configs.
	// Format: decision:strategy:{userID}:{strategyName}
	PrefixStrategyConfig = "decision:strategy:%s:%s"

	// PrefixStrategyConfigAll is the Redis key for all user strategy configs.
	// Format: decision:strategy:{userID}:all
	PrefixStrategyConfigAll = "decision:strategy:%s:all"

	// DefaultStrategyConfigTTL is the default TTL for strategy configurations.
	DefaultStrategyConfigTTL = 24 * time.Hour
)

// StrategyConfigKey generates the Redis key for a specific strategy config.
func StrategyConfigKey(userID string, strategyName StrategyName) string {
	return fmt.Sprintf(PrefixStrategyConfig, sanitizeKeyComponent(userID), sanitizeKeyComponent(string(strategyName)))
}

// StrategyConfigAllKey generates the Redis key for all strategy configs.
func StrategyConfigAllKey(userID string) string {
	return fmt.Sprintf(PrefixStrategyConfigAll, sanitizeKeyComponent(userID))
}

// RedisClient defines the minimal interface needed for Redis operations.
// This allows for easy mocking in tests.
type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	IsHealthy() bool
}

// InMemoryStrategyConfigStore provides an in-memory implementation of StrategyConfigStore.
// This is useful for testing and when Redis is unavailable.
type InMemoryStrategyConfigStore struct {
	configs  map[string]*DecisionEngineSettings
	mu       sync.RWMutex
	defaults *DecisionEngineSettings
}

// NewInMemoryStrategyConfigStore creates a new in-memory config store.
func NewInMemoryStrategyConfigStore() *InMemoryStrategyConfigStore {
	return &InMemoryStrategyConfigStore{
		configs:  make(map[string]*DecisionEngineSettings),
		defaults: DefaultDecisionEngineSettings(),
	}
}

// GetStrategyConfig retrieves a specific strategy configuration.
func (s *InMemoryStrategyConfigStore) GetStrategyConfig(ctx context.Context, userID string, strategyName StrategyName) (*StrategySettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings, exists := s.configs[userID]
	if !exists {
		// Return defaults
		if defaultSettings, ok := s.defaults.Strategies[strategyName]; ok {
			return defaultSettings.Clone(), nil
		}
		return nil, fmt.Errorf("strategy %s not found in defaults", strategyName)
	}

	if strategySettings, ok := settings.Strategies[strategyName]; ok {
		return strategySettings.Clone(), nil
	}

	// Fall back to defaults if strategy not configured for user
	if defaultSettings, ok := s.defaults.Strategies[strategyName]; ok {
		return defaultSettings.Clone(), nil
	}

	return nil, fmt.Errorf("strategy %s not found", strategyName)
}

// SetStrategyConfig stores a specific strategy configuration.
func (s *InMemoryStrategyConfigStore) SetStrategyConfig(ctx context.Context, userID string, settings *StrategySettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	if err := settings.Validate(); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	userSettings, exists := s.configs[userID]
	if !exists {
		userSettings = s.defaults.Clone()
		s.configs[userID] = userSettings
	}

	userSettings.Strategies[settings.Name] = settings.Clone()
	userSettings.LastUpdated = time.Now().UnixMilli()

	return nil
}

// GetAllStrategyConfigs retrieves all strategy configurations for a user.
func (s *InMemoryStrategyConfigStore) GetAllStrategyConfigs(ctx context.Context, userID string) (*DecisionEngineSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings, exists := s.configs[userID]
	if !exists {
		return s.defaults.Clone(), nil
	}

	return settings.Clone(), nil
}

// SetAllStrategyConfigs stores all strategy configurations for a user.
func (s *InMemoryStrategyConfigStore) SetAllStrategyConfigs(ctx context.Context, userID string, settings *DecisionEngineSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	if err := settings.Validate(); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs[userID] = settings.Clone()

	return nil
}

// DeleteStrategyConfig removes a strategy configuration for a user.
func (s *InMemoryStrategyConfigStore) DeleteStrategyConfig(ctx context.Context, userID string, strategyName StrategyName) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, exists := s.configs[userID]
	if !exists {
		return nil // Nothing to delete
	}

	delete(settings.Strategies, strategyName)
	settings.LastUpdated = time.Now().UnixMilli()

	return nil
}

// ResetToDefaults resets a user's strategy configuration to defaults.
func (s *InMemoryStrategyConfigStore) ResetToDefaults(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs[userID] = s.defaults.Clone()

	return nil
}

// SetDefaults sets the default settings used for new users or reset.
func (s *InMemoryStrategyConfigStore) SetDefaults(defaults *DecisionEngineSettings) {
	if defaults == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.defaults = defaults.Clone()
}

// GetDefaults returns the current default settings.
func (s *InMemoryStrategyConfigStore) GetDefaults() *DecisionEngineSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.defaults.Clone()
}

// ClearUser removes all configurations for a user.
func (s *InMemoryStrategyConfigStore) ClearUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.configs, userID)
}

// ClearAll removes all configurations.
func (s *InMemoryStrategyConfigStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs = make(map[string]*DecisionEngineSettings)
}

// StrategyConfigManager provides high-level strategy configuration management.
// It coordinates between the registry and config store.
// Thread safety is delegated to the underlying registry and store implementations.
type StrategyConfigManager struct {
	registry *StrategyRegistry
	store    StrategyConfigStore
}

// NewStrategyConfigManager creates a new StrategyConfigManager.
func NewStrategyConfigManager(registry *StrategyRegistry, store StrategyConfigStore) *StrategyConfigManager {
	if registry == nil {
		registry = NewStrategyRegistry()
	}
	if store == nil {
		store = NewInMemoryStrategyConfigStore()
	}

	return &StrategyConfigManager{
		registry: registry,
		store:    store,
	}
}

// GetRegistry returns the strategy registry.
func (m *StrategyConfigManager) GetRegistry() *StrategyRegistry {
	return m.registry
}

// GetStore returns the config store.
func (m *StrategyConfigManager) GetStore() StrategyConfigStore {
	return m.store
}

// GetUserStrategyConfig gets a user's strategy configuration.
// Returns defaults if user has no custom configuration.
func (m *StrategyConfigManager) GetUserStrategyConfig(ctx context.Context, userID string, strategyName StrategyName) (*StrategySettings, error) {
	return m.store.GetStrategyConfig(ctx, userID, strategyName)
}

// GetUserSettings gets all user settings.
func (m *StrategyConfigManager) GetUserSettings(ctx context.Context, userID string) (*DecisionEngineSettings, error) {
	return m.store.GetAllStrategyConfigs(ctx, userID)
}

// UpdateUserStrategyConfig updates a user's strategy configuration.
func (m *StrategyConfigManager) UpdateUserStrategyConfig(ctx context.Context, userID string, settings *StrategySettings) error {
	return m.store.SetStrategyConfig(ctx, userID, settings)
}

// UpdateUserSettings updates all user settings.
func (m *StrategyConfigManager) UpdateUserSettings(ctx context.Context, userID string, settings *DecisionEngineSettings) error {
	return m.store.SetAllStrategyConfigs(ctx, userID, settings)
}

// ResetUserToDefaults resets a user's strategy settings to defaults.
func (m *StrategyConfigManager) ResetUserToDefaults(ctx context.Context, userID string) error {
	return m.store.ResetToDefaults(ctx, userID)
}

// GetBestStrategyForState selects the best strategy for the given coin state.
// It considers the market regime, user preferences, and strategy scores.
func (m *StrategyConfigManager) GetBestStrategyForState(ctx context.Context, userID string, state *CoinState) (Strategy, *StrategyScore, error) {
	if state == nil {
		return nil, nil, fmt.Errorf("state cannot be nil")
	}

	// Get user settings to check which strategies are enabled
	userSettings, err := m.store.GetAllStrategyConfigs(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user settings: %w", err)
	}

	// Get strategies for the current regime
	strategies := m.registry.GetStrategiesForRegime(state.Regime)
	if len(strategies) == 0 {
		// Fall back to default strategy
		strategy, err := m.registry.GetDefaultStrategy()
		if err != nil {
			return nil, nil, fmt.Errorf("no strategies available for regime %s", state.Regime)
		}
		strategies = []Strategy{strategy}
	}

	var bestStrategy Strategy
	var bestScore *StrategyScore

	for _, strategy := range strategies {
		strategyName := StrategyName(strategy.Name())

		// Check if strategy is enabled for this user
		if stratSettings, exists := userSettings.Strategies[strategyName]; exists {
			if !stratSettings.Enabled {
				continue
			}
		}

		// Calculate score
		score := strategy.CalculateScore(state)
		if bestScore == nil || score.FinalScore > bestScore.FinalScore {
			bestStrategy = strategy
			bestScore = &score
		}
	}

	if bestStrategy == nil {
		return nil, nil, fmt.Errorf("no enabled strategies available")
	}

	return bestStrategy, bestScore, nil
}

// ValidateStrategyConfig validates strategy settings without storing them.
func (m *StrategyConfigManager) ValidateStrategyConfig(settings *StrategySettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	return settings.Validate()
}

// ValidateAllConfigs validates all strategy settings.
func (m *StrategyConfigManager) ValidateAllConfigs(settings *DecisionEngineSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	return settings.Validate()
}

// StrategyConfigJSON represents strategy config in JSON format for API responses.
type StrategyConfigJSON struct {
	UserID           string                              `json:"user_id"`
	ActiveStrategy   string                              `json:"active_strategy"`
	Strategies       map[string]*StrategySettings        `json:"strategies"`
	LastUpdated      int64                               `json:"last_updated"`
	AvailableStrategies []*StrategyInfo                  `json:"available_strategies"`
}

// ToJSON converts settings to JSON-friendly format.
func (m *StrategyConfigManager) ToJSON(ctx context.Context, userID string) (*StrategyConfigJSON, error) {
	settings, err := m.store.GetAllStrategyConfigs(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Convert strategies map to string keys
	strategies := make(map[string]*StrategySettings)
	for name, s := range settings.Strategies {
		strategies[string(name)] = s
	}

	return &StrategyConfigJSON{
		UserID:              userID,
		ActiveStrategy:      string(settings.ActiveStrategy),
		Strategies:          strategies,
		LastUpdated:         settings.LastUpdated,
		AvailableStrategies: m.registry.ListStrategies(),
	}, nil
}

// FromJSON parses JSON and updates user settings.
func (m *StrategyConfigManager) FromJSON(ctx context.Context, userID string, data []byte) error {
	var config StrategyConfigJSON
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	settings := NewDecisionEngineSettings()
	settings.ActiveStrategy = StrategyName(config.ActiveStrategy)
	settings.LastUpdated = time.Now().UnixMilli()

	for name, s := range config.Strategies {
		settings.Strategies[StrategyName(name)] = s
	}

	if err := settings.Validate(); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}

	return m.store.SetAllStrategyConfigs(ctx, userID, settings)
}

// Global config manager instance
var (
	globalConfigManager   *StrategyConfigManager
	configManagerOnce     sync.Once
	configManagerMu       sync.RWMutex
)

// GetGlobalConfigManager returns the global strategy config manager.
// Creates one with defaults if not already initialized.
func GetGlobalConfigManager() *StrategyConfigManager {
	configManagerOnce.Do(func() {
		configManagerMu.Lock()
		defer configManagerMu.Unlock()
		globalConfigManager = NewStrategyConfigManager(GetGlobalRegistry(), NewInMemoryStrategyConfigStore())
	})
	configManagerMu.RLock()
	defer configManagerMu.RUnlock()
	return globalConfigManager
}

// SetGlobalConfigManager sets the global config manager.
// This is thread-safe and can be used to replace the manager after initialization.
func SetGlobalConfigManager(manager *StrategyConfigManager) {
	configManagerMu.Lock()
	defer configManagerMu.Unlock()
	globalConfigManager = manager
}
