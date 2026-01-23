package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/database"

	"github.com/redis/go-redis/v9"
)

// SettingsCacheService provides granular cache access to user settings
// ALL reads go through this service - never bypass to DB directly
type SettingsCacheService struct {
	cache  *CacheService
	repo   *database.Repository
	logger Logger
}

// NewSettingsCacheService creates a new settings cache service
func NewSettingsCacheService(cache *CacheService, repo *database.Repository, logger Logger) *SettingsCacheService {
	return &SettingsCacheService{
		cache:  cache,
		repo:   repo,
		logger: logger,
	}
}

// ============================================================================
// LOAD OPERATIONS (On User Login)
// ============================================================================

// LoadUserSettings loads ALL user settings (88 keys) on login
// This MUST succeed for user to trade - returns error if Redis unavailable
// Key breakdown: 80 mode keys + 4 global keys + 4 safety keys = 88 total
func (s *SettingsCacheService) LoadUserSettings(ctx context.Context, userID string) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	var errs []error

	// Load mode settings (80 keys = 4 modes x 20 groups)
	for _, mode := range TradingModes {
		if err := s.loadModeToCache(ctx, userID, mode); err != nil {
			errs = append(errs, fmt.Errorf("mode %s: %w", mode, err))
		}
	}

	// Load global settings (4 keys: circuit_breaker, llm_config, capital_allocation, global_trading)
	if err := s.loadGlobalSettings(ctx, userID); err != nil {
		errs = append(errs, fmt.Errorf("global: %w", err))
	}

	// Load safety settings (4 keys: one per mode)
	if err := s.loadSafetySettings(ctx, userID); err != nil {
		errs = append(errs, fmt.Errorf("safety: %w", err))
	}

	// Load mode+strategy settings (16 keys: 4 modes x 4 strategies) - Story 11.32
	if err := s.PopulateModeStrategiesFromDB(ctx, userID); err != nil {
		errs = append(errs, fmt.Errorf("mode+strategy: %w", err))
	}

	// Load symbol settings (Story 9.12 Phase 1)
	if err := s.LoadSymbolSettings(ctx, userID); err != nil {
		errs = append(errs, fmt.Errorf("symbol_settings: %w", err))
	}

	// Load confluence configs (Story 9.12 Phase 3)
	if err := s.LoadCoinConfluenceConfigs(ctx, userID); err != nil {
		errs = append(errs, fmt.Errorf("confluence_configs: %w", err))
	}

	if len(errs) > 0 {
		s.logger.Warn("Some settings failed to load", "userID", userID, "errors", errs)
	}

	return nil
}

// parseUserIDStringToInt converts a string user ID to integer
// Handles UUID format for default admin user
func parseUserIDStringToInt(userID string) (int, error) {
	if userID == "" {
		return 0, fmt.Errorf("user ID is empty")
	}

	// Handle UUID format (00000000-0000-0000-0000-000000000000)
	// Default admin UUID should map to user ID 1
	if userID == "00000000-0000-0000-0000-000000000000" {
		return 1, nil
	}

	// Try parsing as integer
	id, err := strconv.Atoi(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID format: %s", userID)
	}

	if id <= 0 {
		return 0, fmt.Errorf("user ID must be positive: %d", id)
	}

	return id, nil
}

// loadModeToCache loads a single mode's settings into granular cache keys
func (s *SettingsCacheService) loadModeToCache(ctx context.Context, userID, mode string) error {
	// Get full mode config from database INCLUDING the enabled column (source of truth)
	enabled, configJSON, err := s.repo.GetUserModeConfigWithEnabled(ctx, userID, mode)
	if err != nil {
		return fmt.Errorf("failed to get mode config: %w", err)
	}
	if configJSON == nil {
		return nil // No config in DB, skip caching
	}

	// Parse into ModeFullConfig
	var config autopilot.ModeFullConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return fmt.Errorf("failed to parse mode config: %w", err)
	}

	// Extract and cache each group
	for _, group := range SettingGroups {
		var groupData interface{}
		var groupJSON []byte

		// CRITICAL: For "enabled" group, use the database COLUMN value (source of truth)
		// not config.Enabled from JSON (which may be stale after toggle)
		if group.Key == "enabled" {
			groupData = map[string]interface{}{"enabled": enabled}
			groupJSON, _ = json.Marshal(groupData)
		} else {
			groupData = s.extractGroupFromConfig(&config, group.Key)
			if groupData == nil {
				continue
			}
			groupJSON, _ = json.Marshal(groupData)
		}

		key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group.Key)

		if err := s.cache.Set(ctx, key, string(groupJSON), 0); err != nil {
			s.logger.Debug("Failed to cache group", "key", key, "error", err)
		}
	}

	return nil
}

// loadGlobalSettings loads all 5 global settings (circuit_breaker, llm_config, capital_allocation, global_trading, position_management)
// Story 6.2: Part of 88-key architecture (now 89 keys with position_management)
func (s *SettingsCacheService) loadGlobalSettings(ctx context.Context, userID string) error {
	// Circuit Breaker
	if cb, err := s.repo.GetUserGlobalCircuitBreaker(ctx, userID); err == nil && cb != nil {
		key := fmt.Sprintf("user:%s:circuit_breaker", userID)
		data, _ := json.Marshal(cb)
		s.cache.Set(ctx, key, string(data), 0)
	}

	// LLM Config
	if llm, err := s.repo.GetUserLLMConfig(ctx, userID); err == nil && llm != nil {
		key := fmt.Sprintf("user:%s:llm_config", userID)
		data, _ := json.Marshal(llm)
		s.cache.Set(ctx, key, string(data), 0)
	}

	// Capital Allocation
	if cap, err := s.repo.GetUserCapitalAllocation(ctx, userID); err == nil && cap != nil {
		key := fmt.Sprintf("user:%s:capital_allocation", userID)
		data, _ := json.Marshal(cap)
		s.cache.Set(ctx, key, string(data), 0)
	}

	// Global Trading (Story 6.2 fix: was missing from 88-key load)
	if gt, err := s.repo.GetUserGlobalTrading(ctx, userID); err == nil && gt != nil {
		key := fmt.Sprintf("user:%s:global_trading", userID)
		data, _ := json.Marshal(gt)
		s.cache.Set(ctx, key, string(data), 0)
	}

	// Position Management (Story 10.1: Position Decision Configuration)
	if pm, err := s.repo.GetUserPositionManagement(ctx, userID); err == nil && pm != nil {
		key := fmt.Sprintf("user:%s:position_management", userID)
		data, _ := json.Marshal(pm)
		s.cache.Set(ctx, key, string(data), 0)
	}

	return nil
}

// loadSafetySettings loads all 4 safety settings (one per mode)
// Story 6.2: Part of 88-key architecture
func (s *SettingsCacheService) loadSafetySettings(ctx context.Context, userID string) error {
	for _, mode := range TradingModes {
		if safety, err := s.repo.GetUserSafetySettings(ctx, userID, mode); err == nil && safety != nil {
			key := fmt.Sprintf("user:%s:safety:%s", userID, mode)
			data, _ := json.Marshal(safety)
			s.cache.Set(ctx, key, string(data), 0)
		}
	}
	return nil
}

// ============================================================================
// READ OPERATIONS (Cache-Only with Auto-Populate)
// ============================================================================

// GetModeGroup retrieves a single settings group
// NEVER bypasses cache - if miss, populates cache first then returns from cache
func (s *SettingsCacheService) GetModeGroup(ctx context.Context, userID, mode, group string) ([]byte, error) {
	// RULE: Redis must be healthy - no bypass allowed
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group)

	// Try cache first
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		return []byte(cached), nil
	}

	// Cache miss - populate cache from DB, then return FROM CACHE
	if err := s.populateModeGroupFromDB(ctx, userID, mode, group); err != nil {
		return nil, err
	}

	// Now read from cache (NOT from DB directly)
	cached, err = s.cache.Get(ctx, key)
	if err != nil || cached == "" {
		return nil, ErrSettingNotFound
	}

	return []byte(cached), nil
}

// populateModeGroupFromDB loads a single group from DB into cache
func (s *SettingsCacheService) populateModeGroupFromDB(ctx context.Context, userID, mode, group string) error {
	// Use GetUserModeConfigWithEnabled to get BOTH the enabled column AND config_json
	// The enabled column is the source of truth for mode enabled status (updated by toggle)
	// The config_json contains all other settings
	enabled, configJSON, err := s.repo.GetUserModeConfigWithEnabled(ctx, userID, mode)
	if err != nil {
		return fmt.Errorf("failed to get mode config from DB: %w", err)
	}
	if configJSON == nil {
		return ErrSettingNotFound
	}

	var config autopilot.ModeFullConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return fmt.Errorf("failed to parse mode config: %w", err)
	}

	// CRITICAL FIX: For the "enabled" group, use the database COLUMN value (source of truth)
	// not the config.Enabled from JSON (which may be stale after toggle)
	if group == "enabled" {
		groupData := map[string]interface{}{"enabled": enabled}
		key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group)
		groupJSON, _ := json.Marshal(groupData)
		return s.cache.Set(ctx, key, string(groupJSON), 0)
	}

	// For all other groups, extract from config_json as before
	groupData := s.extractGroupFromConfig(&config, group)
	if groupData == nil {
		return ErrSettingNotFound
	}

	key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group)
	groupJSON, _ := json.Marshal(groupData)

	return s.cache.Set(ctx, key, string(groupJSON), 0)
}

// GetModeEnabled checks if a mode is enabled (fast path)
func (s *SettingsCacheService) GetModeEnabled(ctx context.Context, userID, mode string) (bool, error) {
	data, err := s.GetModeGroup(ctx, userID, mode, "enabled")
	if err != nil {
		return false, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return false, err
	}

	if enabled, ok := result["enabled"].(bool); ok {
		return enabled, nil
	}
	return false, nil
}

// GetModeConfidence retrieves confidence settings for a mode
func (s *SettingsCacheService) GetModeConfidence(ctx context.Context, userID, mode string) (*autopilot.ModeConfidenceConfig, error) {
	data, err := s.GetModeGroup(ctx, userID, mode, "confidence")
	if err != nil {
		return nil, err
	}

	var config autopilot.ModeConfidenceConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetModeSLTP retrieves SLTP settings for a mode
func (s *SettingsCacheService) GetModeSLTP(ctx context.Context, userID, mode string) (*autopilot.ModeSLTPConfig, error) {
	data, err := s.GetModeGroup(ctx, userID, mode, "sltp")
	if err != nil {
		return nil, err
	}

	var config autopilot.ModeSLTPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetPositionOptimization retrieves position optimization settings
func (s *SettingsCacheService) GetPositionOptimization(ctx context.Context, userID, mode string) (*autopilot.PositionOptimizationConfig, error) {
	data, err := s.GetModeGroup(ctx, userID, mode, "position_optimization")
	if err != nil {
		return nil, err
	}

	var config autopilot.PositionOptimizationConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetModeConfig assembles full ModeFullConfig from all cached groups
// Uses MGET for atomic read of all 20 groups
func (s *SettingsCacheService) GetModeConfig(ctx context.Context, userID, mode string) (*autopilot.ModeFullConfig, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	// Build keys for all groups
	keys := make([]string, len(SettingGroups))
	for i, group := range SettingGroups {
		keys[i] = fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group.Key)
	}

	// Atomic read of all 20 groups using MGET
	values, err := s.cache.MGet(ctx, keys...)
	if err != nil {
		return nil, fmt.Errorf("failed to get mode config: %w", err)
	}

	config := &autopilot.ModeFullConfig{ModeName: mode}

	// Check for any misses and handle
	for i, val := range values {
		if val == nil {
			// Cache miss for this group - populate it
			if err := s.populateModeGroupFromDB(ctx, userID, mode, SettingGroups[i].Key); err != nil {
				s.logger.Debug("Group not found", "group", SettingGroups[i].Key, "error", err)
				continue
			}
			// Re-fetch this single key
			cached, _ := s.cache.Get(ctx, keys[i])
			if cached != "" {
				val = cached
			}
		}

		if val != nil {
			// Handle both string and interface{} types from MGET
			var valStr string
			switch v := val.(type) {
			case string:
				valStr = v
			default:
				continue
			}
			if valStr != "" {
				s.mergeGroupIntoConfig(config, SettingGroups[i].Key, []byte(valStr))
			}
		}
	}

	return config, nil
}

// ============================================================================
// WRITE OPERATIONS (Write-Through: DB First, Then Cache)
// ============================================================================

// UpdateModeGroup updates a single settings group with write-through
// DB FIRST for durability, then cache
func (s *SettingsCacheService) UpdateModeGroup(ctx context.Context, userID, mode, group string, data []byte) error {
	// STEP 1: Write to durable storage first
	if err := s.repo.UpdateUserModeConfigGroup(ctx, userID, mode, group, data); err != nil {
		return fmt.Errorf("failed to persist to DB: %w", err)
	}

	// STEP 2: Update cache (best effort - DB has the truth)
	key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group)
	if s.cache.IsHealthy() {
		if err := s.cache.Set(ctx, key, string(data), 0); err != nil {
			// Log warning but don't fail - DB has the truth
			// Next read will repopulate cache from DB
			s.logger.Warn("Failed to update cache, will repopulate on next read",
				"key", key, "error", err)
		}
	}

	return nil
}

// ============================================================================
// CROSS-MODE SETTINGS (Circuit Breaker, LLM Config, Capital Allocation)
// ============================================================================

// GetCircuitBreaker retrieves global circuit breaker (cache-only with auto-populate)
func (s *SettingsCacheService) GetCircuitBreaker(ctx context.Context, userID string) (*database.UserGlobalCircuitBreaker, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := fmt.Sprintf("user:%s:circuit_breaker", userID)

	// Try cache first
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var cb database.UserGlobalCircuitBreaker
		if err := json.Unmarshal([]byte(cached), &cb); err == nil {
			return &cb, nil
		}
	}

	// Cache miss - load from DB, populate cache, return from cache
	cb, err := s.repo.GetUserGlobalCircuitBreaker(ctx, userID)
	if err != nil {
		return nil, err
	}
	if cb == nil {
		return nil, ErrSettingNotFound
	}

	// Populate cache
	data, _ := json.Marshal(cb)
	s.cache.Set(ctx, key, string(data), 0)

	return cb, nil
}

// UpdateCircuitBreaker updates with write-through (DB first)
func (s *SettingsCacheService) UpdateCircuitBreaker(ctx context.Context, userID string, cb *database.UserGlobalCircuitBreaker) error {
	// DB first
	cb.UserID = userID
	if err := s.repo.SaveUserGlobalCircuitBreaker(ctx, cb); err != nil {
		return err
	}

	// Then cache
	key := fmt.Sprintf("user:%s:circuit_breaker", userID)
	if s.cache.IsHealthy() {
		data, _ := json.Marshal(cb)
		s.cache.Set(ctx, key, string(data), 0)
	}

	return nil
}

// GetLLMConfig retrieves LLM configuration (cache-only with auto-populate)
func (s *SettingsCacheService) GetLLMConfig(ctx context.Context, userID string) (*database.UserLLMConfig, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := fmt.Sprintf("user:%s:llm_config", userID)

	// Try cache first
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var llm database.UserLLMConfig
		if err := json.Unmarshal([]byte(cached), &llm); err == nil {
			return &llm, nil
		}
	}

	// Cache miss - load from DB, populate cache
	llm, err := s.repo.GetUserLLMConfig(ctx, userID)
	if err != nil {
		return nil, err
	}
	if llm == nil {
		return nil, ErrSettingNotFound
	}

	data, _ := json.Marshal(llm)
	s.cache.Set(ctx, key, string(data), 0)

	return llm, nil
}

// UpdateLLMConfig updates with write-through (DB first)
func (s *SettingsCacheService) UpdateLLMConfig(ctx context.Context, userID string, llm *database.UserLLMConfig) error {
	llm.UserID = userID
	if err := s.repo.SaveUserLLMConfig(ctx, llm); err != nil {
		return err
	}

	key := fmt.Sprintf("user:%s:llm_config", userID)
	if s.cache.IsHealthy() {
		data, _ := json.Marshal(llm)
		s.cache.Set(ctx, key, string(data), 0)
	}

	return nil
}

// GetCapitalAllocation retrieves capital allocation (cache-only with auto-populate)
func (s *SettingsCacheService) GetCapitalAllocation(ctx context.Context, userID string) (*database.UserCapitalAllocation, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := fmt.Sprintf("user:%s:capital_allocation", userID)

	// Try cache first
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var cap database.UserCapitalAllocation
		if err := json.Unmarshal([]byte(cached), &cap); err == nil {
			return &cap, nil
		}
	}

	// Cache miss - load from DB, populate cache
	cap, err := s.repo.GetUserCapitalAllocation(ctx, userID)
	if err != nil {
		return nil, err
	}
	if cap == nil {
		return nil, ErrSettingNotFound
	}

	data, _ := json.Marshal(cap)
	s.cache.Set(ctx, key, string(data), 0)

	return cap, nil
}

// UpdateCapitalAllocation updates with write-through (DB first, then invalidate cache)
func (s *SettingsCacheService) UpdateCapitalAllocation(ctx context.Context, userID string, cap *database.UserCapitalAllocation) error {
	cap.UserID = userID
	if err := s.repo.SaveUserCapitalAllocation(ctx, cap); err != nil {
		return err
	}

	key := fmt.Sprintf("user:%s:capital_allocation", userID)
	if s.cache.IsHealthy() {
		// CRITICAL: Delete old cache entry first to ensure stale data is removed
		if err := s.cache.Delete(ctx, key); err != nil {
			s.logger.Warn("[SETTINGS-CACHE] Failed to invalidate cache key %s: %v", key, err)
		}
		// Set new value with error logging
		data, _ := json.Marshal(cap)
		if err := s.cache.Set(ctx, key, string(data), 0); err != nil {
			s.logger.Warn("[SETTINGS-CACHE] Failed to update cache key %s: %v", key, err)
			// Don't fail - DB is source of truth, cache will be repopulated on next read
		}
	}

	return nil
}

// GetGlobalTrading retrieves global trading config (cache-first)
// Story 6.5: Cache-First Read Pattern
func (s *SettingsCacheService) GetGlobalTrading(ctx context.Context, userID string) (*database.UserGlobalTrading, error) {
	key := fmt.Sprintf("user:%s:global_trading", userID)

	// Cache-first
	if s.cache.IsHealthy() {
		if cached, err := s.cache.Get(ctx, key); err == nil && cached != "" {
			var config database.UserGlobalTrading
			if json.Unmarshal([]byte(cached), &config) == nil {
				return &config, nil
			}
		}
	}

	// Cache miss - load from DB
	config, err := s.repo.GetUserGlobalTrading(ctx, userID)
	if err != nil {
		return nil, err
	}
	if config == nil {
		config = database.DefaultUserGlobalTrading()
		config.UserID = userID
	}

	// Populate cache
	if s.cache.IsHealthy() {
		data, _ := json.Marshal(config)
		s.cache.Set(ctx, key, string(data), 0)
	}

	return config, nil
}

// UpdateGlobalTrading updates with write-through (DB first, then cache)
// Story 6.5: Write-Through Pattern
func (s *SettingsCacheService) UpdateGlobalTrading(ctx context.Context, userID string, config *database.UserGlobalTrading) error {
	config.UserID = userID
	if err := s.repo.SaveUserGlobalTrading(ctx, config); err != nil {
		return err
	}

	key := fmt.Sprintf("user:%s:global_trading", userID)
	if s.cache.IsHealthy() {
		data, _ := json.Marshal(config)
		s.cache.Set(ctx, key, string(data), 0)
	}

	return nil
}

// GetCacheService returns the underlying CacheService for direct access
// Story 6.5: Expose cache for handlers that need direct key access
func (s *SettingsCacheService) GetCacheService() *CacheService {
	return s.cache
}

// GetRedisClient returns the underlying Redis client for services that need it.
// Story 11.26: Calibration Data Lifecycle - needed for CalibrationService initialization.
func (s *SettingsCacheService) GetRedisClient() *redis.Client {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.GetClient()
}

// ============================================================================
// RESET AND INVALIDATION OPERATIONS
// ============================================================================

// ResetModeGroup resets a single group to admin defaults
func (s *SettingsCacheService) ResetModeGroup(ctx context.Context, userID, mode, group string) error {
	// Get default value from admin defaults cache or JSON file
	defaultKey := fmt.Sprintf("admin:defaults:mode:%s:%s", mode, group)
	defaultData, err := s.cache.Get(ctx, defaultKey)
	if err != nil || defaultData == "" {
		defaultData, err = s.loadGroupFromDefaults(mode, group)
		if err != nil {
			return fmt.Errorf("failed to get default for group %s: %w", group, err)
		}
	}

	return s.UpdateModeGroup(ctx, userID, mode, group, []byte(defaultData))
}

// loadGroupFromDefaults loads a group's default value from default-settings.json
func (s *SettingsCacheService) loadGroupFromDefaults(mode, group string) (string, error) {
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		return "", fmt.Errorf("default settings not available: %w", err)
	}

	modeConfig, exists := defaults.ModeConfigs[mode]
	if !exists || modeConfig == nil {
		return "", fmt.Errorf("mode %s not found in defaults", mode)
	}

	groupData := s.extractGroupFromConfig(modeConfig, group)
	if groupData == nil {
		return "", fmt.Errorf("group %s not found in mode config", group)
	}

	data, err := json.Marshal(groupData)
	return string(data), err
}

// InvalidateModeGroup removes a single group from cache
func (s *SettingsCacheService) InvalidateModeGroup(ctx context.Context, userID, mode, group string) error {
	key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group)
	return s.cache.Delete(ctx, key)
}

// InvalidateMode removes all groups for a mode from cache
func (s *SettingsCacheService) InvalidateMode(ctx context.Context, userID, mode string) error {
	pattern := fmt.Sprintf("user:%s:mode:%s:*", userID, mode)
	return s.cache.DeletePattern(ctx, pattern)
}

// InvalidateAllModes removes all mode settings from cache for a user
func (s *SettingsCacheService) InvalidateAllModes(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("user:%s:mode:*", userID)
	return s.cache.DeletePattern(ctx, pattern)
}

// InvalidateCrossModeSetting removes a single cross-mode setting from cache
func (s *SettingsCacheService) InvalidateCrossModeSetting(ctx context.Context, userID, setting string) error {
	key := fmt.Sprintf("user:%s:%s", userID, setting)
	return s.cache.Delete(ctx, key)
}

// InvalidateAllUserSettings removes ALL user settings from cache (88 keys)
// Story 6.2: 80 mode keys + 4 global keys + 4 safety keys = 88 total
func (s *SettingsCacheService) InvalidateAllUserSettings(ctx context.Context, userID string) error {
	// Invalidate mode settings (80 keys)
	if err := s.InvalidateAllModes(ctx, userID); err != nil {
		s.logger.Warn("Failed to invalidate mode settings", "error", err)
	}

	// Invalidate global settings (4 keys)
	for _, setting := range CrossModeSettings {
		s.InvalidateCrossModeSetting(ctx, userID, setting)
	}

	// Invalidate global_trading (Story 6.2 fix)
	s.InvalidateCrossModeSetting(ctx, userID, "global_trading")

	// Invalidate safety settings (4 keys)
	if err := s.InvalidateAllSafetySettings(ctx, userID); err != nil {
		s.logger.Warn("Failed to invalidate safety settings", "error", err)
	}

	return nil
}

// GetEnabledModes returns list of enabled mode names for a user
func (s *SettingsCacheService) GetEnabledModes(ctx context.Context, userID string) ([]string, error) {
	var enabled []string

	for _, mode := range TradingModes {
		isEnabled, err := s.GetModeEnabled(ctx, userID, mode)
		if err != nil {
			return nil, err
		}
		if isEnabled {
			enabled = append(enabled, mode)
		}
	}

	return enabled, nil
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// extractGroupFromConfig extracts a specific group's data from ModeFullConfig
func (s *SettingsCacheService) extractGroupFromConfig(config *autopilot.ModeFullConfig, groupKey string) interface{} {
	switch groupKey {
	case "enabled":
		return map[string]interface{}{"enabled": config.Enabled}
	case "timeframe":
		return config.Timeframe
	case "confidence":
		return config.Confidence
	case "size":
		return config.Size
	case "sltp":
		return config.SLTP
	case "risk":
		return config.Risk
	case "circuit_breaker":
		return config.CircuitBreaker
	case "hedge":
		return config.Hedge
	case "averaging":
		return config.Averaging
	case "stale_release":
		return config.StaleRelease
	case "assignment":
		return config.Assignment
	case "mtf":
		return config.MTF
	case "dynamic_ai_exit":
		return config.DynamicAIExit
	case "reversal":
		return config.Reversal
	case "funding_rate":
		return config.FundingRate
	case "trend_divergence":
		return config.TrendDivergence
	case "position_optimization":
		return config.PositionOptimization
	case "trend_filters":
		return config.TrendFilters
	case "early_warning":
		return config.EarlyWarning
	case "entry":
		return config.Entry
	default:
		return nil
	}
}

// mergeGroupIntoConfig merges a group's JSON data into ModeFullConfig
func (s *SettingsCacheService) mergeGroupIntoConfig(config *autopilot.ModeFullConfig, groupKey string, data []byte) error {
	var err error
	switch groupKey {
	case "enabled":
		var m map[string]interface{}
		if err = json.Unmarshal(data, &m); err == nil {
			if enabled, ok := m["enabled"].(bool); ok {
				config.Enabled = enabled
			}
		}
	case "timeframe":
		var t autopilot.ModeTimeframeConfig
		if err = json.Unmarshal(data, &t); err == nil {
			config.Timeframe = &t
		}
	case "confidence":
		var c autopilot.ModeConfidenceConfig
		if err = json.Unmarshal(data, &c); err == nil {
			config.Confidence = &c
		}
	case "size":
		var sz autopilot.ModeSizeConfig
		if err = json.Unmarshal(data, &sz); err == nil {
			config.Size = &sz
		}
	case "sltp":
		var sl autopilot.ModeSLTPConfig
		if err = json.Unmarshal(data, &sl); err == nil {
			config.SLTP = &sl
		}
	case "risk":
		var r autopilot.ModeRiskConfig
		if err = json.Unmarshal(data, &r); err == nil {
			config.Risk = &r
		}
	case "circuit_breaker":
		var cb autopilot.ModeCircuitBreakerConfig
		if err = json.Unmarshal(data, &cb); err == nil {
			config.CircuitBreaker = &cb
		}
	case "hedge":
		var h autopilot.HedgeModeConfig
		if err = json.Unmarshal(data, &h); err == nil {
			config.Hedge = &h
		}
	case "averaging":
		var a autopilot.PositionAveragingConfig
		if err = json.Unmarshal(data, &a); err == nil {
			config.Averaging = &a
		}
	case "stale_release":
		var sr autopilot.StalePositionReleaseConfig
		if err = json.Unmarshal(data, &sr); err == nil {
			config.StaleRelease = &sr
		}
	case "assignment":
		var as autopilot.ModeAssignmentConfig
		if err = json.Unmarshal(data, &as); err == nil {
			config.Assignment = &as
		}
	case "mtf":
		var m autopilot.ModeMTFConfig
		if err = json.Unmarshal(data, &m); err == nil {
			config.MTF = &m
		}
	case "dynamic_ai_exit":
		var d autopilot.ModeDynamicAIExitConfig
		if err = json.Unmarshal(data, &d); err == nil {
			config.DynamicAIExit = &d
		}
	case "reversal":
		var rv autopilot.ModeReversalConfig
		if err = json.Unmarshal(data, &rv); err == nil {
			config.Reversal = &rv
		}
	case "funding_rate":
		var f autopilot.ModeFundingRateConfig
		if err = json.Unmarshal(data, &f); err == nil {
			config.FundingRate = &f
		}
	case "trend_divergence":
		var td autopilot.ModeTrendDivergenceConfig
		if err = json.Unmarshal(data, &td); err == nil {
			config.TrendDivergence = &td
		}
	case "position_optimization":
		var p autopilot.PositionOptimizationConfig
		if err = json.Unmarshal(data, &p); err == nil {
			config.PositionOptimization = &p
		}
	case "trend_filters":
		var tf autopilot.TrendFiltersConfig
		if err = json.Unmarshal(data, &tf); err == nil {
			config.TrendFilters = &tf
		}
	case "early_warning":
		var e autopilot.ModeEarlyWarningConfig
		if err = json.Unmarshal(data, &e); err == nil {
			config.EarlyWarning = &e
		}
	case "entry":
		var en autopilot.ModeEntryConfig
		if err = json.Unmarshal(data, &en); err == nil {
			config.Entry = &en
		}
	}
	return err
}

// ============================================================================
// FULL SETTINGS LOAD (For Settings Comparison)
// ============================================================================

// GetAllUserSettings loads all user settings from cache into a DefaultSettingsFile structure
// Story 6.5: Cache-First Read Pattern for settings comparison
// Returns HTTP 503-compatible error if cache is unavailable
func (s *SettingsCacheService) GetAllUserSettings(ctx context.Context, userID string) (*autopilot.DefaultSettingsFile, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	settings := &autopilot.DefaultSettingsFile{
		ModeConfigs: make(map[string]*autopilot.ModeFullConfig),
	}

	// Load all mode configs from cache
	for _, mode := range TradingModes {
		modeConfig, err := s.GetModeConfig(ctx, userID, mode)
		if err != nil {
			if err == ErrCacheUnavailable {
				return nil, err
			}
			// Mode not configured yet, skip
			s.logger.Debug("Mode config not found in cache", "mode", mode, "userID", userID)
			continue
		}
		settings.ModeConfigs[mode] = modeConfig
	}

	// Load cross-mode settings
	// Global Trading
	if globalTrading, err := s.GetGlobalTrading(ctx, userID); err == nil && globalTrading != nil {
		settings.GlobalTrading = autopilot.GlobalTradingDefaults{
			RiskLevel:               globalTrading.RiskLevel,
			MaxUSDAllocation:        globalTrading.MaxUSDAllocation,
			ProfitReinvestPercent:   globalTrading.ProfitReinvestPercent,
			ProfitReinvestRiskLevel: globalTrading.ProfitReinvestRiskLevel,
			Timezone:                globalTrading.Timezone,
			TimezoneOffset:          globalTrading.TimezoneOffset,
		}
	}

	// Circuit Breaker
	if cb, err := s.GetCircuitBreaker(ctx, userID); err == nil && cb != nil {
		settings.CircuitBreaker = autopilot.CircuitBreakerDefaults{
			Global: autopilot.GlobalCircuitBreakerConfig{
				Enabled:      cb.Enabled,
				MaxDailyLoss: cb.MaxDailyLoss,
			},
		}
	}

	// LLM Config
	if llm, err := s.GetLLMConfig(ctx, userID); err == nil && llm != nil {
		settings.LLMConfig = autopilot.LLMConfigDefaults{
			Global: autopilot.GlobalLLMConfig{
				Enabled:  llm.Enabled,
				Provider: llm.Provider,
			},
		}
	}

	// Capital Allocation
	if cap, err := s.GetCapitalAllocation(ctx, userID); err == nil && cap != nil {
		settings.CapitalAllocation = autopilot.CapitalAllocationDefaults{
			UltraFastPercent: cap.UltraFastPercent,
			ScalpPercent:     cap.ScalpPercent,
			SwingPercent:     cap.SwingPercent,
			PositionPercent:  cap.PositionPercent,
		}
	}

	return settings, nil
}

// IsHealthy returns whether the underlying cache is healthy
func (s *SettingsCacheService) IsHealthy() bool {
	return s.cache.IsHealthy()
}

// ============================================================================
// SAFETY SETTINGS (Per-Mode Safety Controls)
// Story 6.5: Cache-First Read Pattern for Safety Settings
// ============================================================================

// GetSafetySettings retrieves safety settings for a specific mode (cache-only with auto-populate)
func (s *SettingsCacheService) GetSafetySettings(ctx context.Context, userID, mode string) (*database.UserSafetySettings, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := fmt.Sprintf("user:%s:safety:%s", userID, mode)

	// Try cache first
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var settings database.UserSafetySettings
		if err := json.Unmarshal([]byte(cached), &settings); err == nil {
			return &settings, nil
		}
	}

	// Cache miss - load from DB, populate cache
	settings, err := s.repo.GetUserSafetySettings(ctx, userID, mode)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		// Return default settings if not found in DB
		settings = database.DefaultUserSafetySettings(mode)
		settings.UserID = userID
	}

	// Populate cache
	data, _ := json.Marshal(settings)
	s.cache.Set(ctx, key, string(data), 0)

	return settings, nil
}

// GetAllSafetySettings retrieves safety settings for all modes (cache-only with auto-populate)
func (s *SettingsCacheService) GetAllSafetySettings(ctx context.Context, userID string) (map[string]*database.UserSafetySettings, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	modes := []string{"ultra_fast", "scalp", "swing", "position"}
	result := make(map[string]*database.UserSafetySettings)

	for _, mode := range modes {
		settings, err := s.GetSafetySettings(ctx, userID, mode)
		if err != nil {
			// If cache unavailable, propagate that error
			if err == ErrCacheUnavailable {
				return nil, err
			}
			// For other errors, use defaults
			settings = database.DefaultUserSafetySettings(mode)
			settings.UserID = userID
		}
		result[mode] = settings
	}

	return result, nil
}

// UpdateSafetySettings updates safety settings with write-through (DB first, then cache)
func (s *SettingsCacheService) UpdateSafetySettings(ctx context.Context, userID, mode string, settings *database.UserSafetySettings) error {
	// DB first for durability
	settings.UserID = userID
	settings.Mode = mode
	if err := s.repo.SaveUserSafetySettings(ctx, settings); err != nil {
		return err
	}

	// Then update cache (best effort - DB has the truth)
	key := fmt.Sprintf("user:%s:safety:%s", userID, mode)
	if s.cache.IsHealthy() {
		data, _ := json.Marshal(settings)
		if err := s.cache.Set(ctx, key, string(data), 0); err != nil {
			s.logger.Warn("Failed to update safety settings cache, will repopulate on next read",
				"key", key, "error", err)
		}
	}

	return nil
}

// InvalidateSafetySettings removes safety settings from cache for a mode
func (s *SettingsCacheService) InvalidateSafetySettings(ctx context.Context, userID, mode string) error {
	key := fmt.Sprintf("user:%s:safety:%s", userID, mode)
	return s.cache.Delete(ctx, key)
}

// InvalidateAllSafetySettings removes all safety settings from cache for a user
func (s *SettingsCacheService) InvalidateAllSafetySettings(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("user:%s:safety:*", userID)
	return s.cache.DeletePattern(ctx, pattern)
}

// ============================================================================
// POSITION MANAGEMENT SETTINGS (Story 10.1: Position Decision Configuration)
// Cache-First Read Pattern for Position Management
// ============================================================================

// GetPositionManagement retrieves position management settings (cache-only with auto-populate)
func (s *SettingsCacheService) GetPositionManagement(ctx context.Context, userID string) (*database.UserPositionManagement, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := fmt.Sprintf("user:%s:position_management", userID)

	// Try cache first
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var config database.UserPositionManagement
		if err := json.Unmarshal([]byte(cached), &config); err == nil {
			return &config, nil
		}
	}

	// Cache miss - load from DB, populate cache
	config, err := s.repo.GetUserPositionManagement(ctx, userID)
	if err != nil {
		return nil, err
	}
	if config == nil {
		// Return default settings if not found in DB
		config = database.DefaultUserPositionManagement()
		config.UserID = userID
	}

	// Populate cache
	data, _ := json.Marshal(config)
	s.cache.Set(ctx, key, string(data), 0)

	return config, nil
}

// UpdatePositionManagement updates position management with write-through (DB first, then cache)
func (s *SettingsCacheService) UpdatePositionManagement(ctx context.Context, userID string, config *database.UserPositionManagement) error {
	// DB first for durability
	config.UserID = userID
	if err := s.repo.SaveUserPositionManagement(ctx, config); err != nil {
		return err
	}

	// Then update cache (best effort - DB has the truth)
	key := fmt.Sprintf("user:%s:position_management", userID)
	if s.cache.IsHealthy() {
		data, _ := json.Marshal(config)
		if err := s.cache.Set(ctx, key, string(data), 0); err != nil {
			s.logger.Warn("Failed to update position management cache, will repopulate on next read",
				"key", key, "error", err)
		}
	}

	return nil
}

// InvalidatePositionManagement removes position management from cache for a user
func (s *SettingsCacheService) InvalidatePositionManagement(ctx context.Context, userID string) error {
	key := fmt.Sprintf("user:%s:position_management", userID)
	return s.cache.Delete(ctx, key)
}

// ============================================================================
// SEQUENCE PROVIDER (Epic 7: Client Order ID Generation)
// Implements orders.SequenceProvider interface for atomic daily sequence numbers
// ============================================================================

// IncrementDailySequence atomically increments and returns the daily sequence for a user.
// dateKey is in YYYYMMDD format (e.g., "20260115")
// Delegates to underlying CacheService.
func (s *SettingsCacheService) IncrementDailySequence(ctx context.Context, userID, dateKey string) (int64, error) {
	return s.cache.IncrementDailySequence(ctx, userID, dateKey)
}

// ============================================================================
// MODE-STRATEGY CACHE LAYER (Story 11.30)
// Hierarchical cache for Mode+Strategy configuration
// Key pattern: mode:{userID}:{mode}:{strategy}
// TTL: 24 hours (same as other settings)
// ============================================================================

// ModeStrategyCacheTTL is the TTL for mode+strategy cache entries (24 hours)
const ModeStrategyCacheTTL = 24 * time.Hour

// ValidStrategies lists the valid strategy names for cache operations
var ValidStrategies = []string{"trend_following", "mean_reversion", "breakout", "range_trading"}

// modeStrategyKey generates the cache key for a mode+strategy configuration
// Format: mode:{userID}:{mode}:{strategy}
func modeStrategyKey(userID string, mode, strategy string) string {
	return fmt.Sprintf("mode:%s:%s:%s", userID, mode, strategy)
}

// modeStrategiesPattern generates a pattern for all strategies under a mode
// Format: mode:{userID}:{mode}:*
func modeStrategiesPattern(userID string, mode string) string {
	return fmt.Sprintf("mode:%s:%s:*", userID, mode)
}

// allModeStrategiesPattern generates a pattern for all mode+strategy keys for a user
// Format: mode:{userID}:*
func allModeStrategiesPattern(userID string) string {
	return fmt.Sprintf("mode:%s:*", userID)
}

// GetModeStrategyConfig retrieves settings for a specific mode+strategy
// Cache-first with auto-populate from DB on miss
func (s *SettingsCacheService) GetModeStrategyConfig(ctx context.Context, userID string, mode, strategy string) (*database.ModeStrategyConfig, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := modeStrategyKey(userID, mode, strategy)

	// Try cache first
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var config database.ModeStrategyConfig
		if err := json.Unmarshal([]byte(cached), &config); err == nil {
			return &config, nil
		}
		// Invalid JSON in cache, fall through to DB
		s.logger.Warn("Invalid JSON in mode strategy cache, repopulating", "key", key)
	}

	// Cache miss - load from DB, populate cache
	row, err := s.repo.GetModeStrategySettings(ctx, userID, mode, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to get mode strategy from DB: %w", err)
	}

	// If not in DB, return default config
	if row == nil {
		config := database.DefaultModeStrategyConfig(mode, strategy)
		// Cache the default
		data, _ := json.Marshal(config)
		s.cache.Set(ctx, key, string(data), ModeStrategyCacheTTL)
		return config, nil
	}

	// Parse the settings JSON into ModeStrategyConfig
	var config database.ModeStrategyConfig
	if err := json.Unmarshal(row.Settings, &config); err != nil {
		return nil, fmt.Errorf("failed to parse mode strategy settings: %w", err)
	}

	// Apply row-level fields that aren't in the settings JSON
	config.Enabled = row.Enabled
	config.Priority = row.Priority

	// Populate cache
	data, _ := json.Marshal(config)
	s.cache.Set(ctx, key, string(data), ModeStrategyCacheTTL)

	return &config, nil
}

// SetModeStrategyConfig stores settings for a specific mode+strategy
// Write-through: DB first, then cache
func (s *SettingsCacheService) SetModeStrategyConfig(ctx context.Context, userID string, mode, strategy string, config *database.ModeStrategyConfig) error {
	// Serialize the config for DB storage
	settingsJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal mode strategy config: %w", err)
	}

	// Check if record exists to decide INSERT vs UPDATE
	existing, err := s.repo.GetModeStrategySettings(ctx, userID, mode, strategy)
	if err != nil {
		return fmt.Errorf("failed to check existing mode strategy: %w", err)
	}

	if existing == nil {
		// Create new record
		if err := s.repo.CreateModeStrategySettings(ctx, userID, mode, strategy, config.Enabled, config.Priority, settingsJSON); err != nil {
			return fmt.Errorf("failed to create mode strategy in DB: %w", err)
		}
	} else {
		// Update existing record
		if err := s.repo.UpdateModeStrategySettings(ctx, userID, mode, strategy, settingsJSON); err != nil {
			return fmt.Errorf("failed to update mode strategy in DB: %w", err)
		}
		// Also update enabled/priority if they changed
		if existing.Enabled != config.Enabled {
			if err := s.repo.UpdateModeStrategyEnabled(ctx, userID, mode, strategy, config.Enabled); err != nil {
				s.logger.Warn("Failed to update enabled status", "error", err)
			}
		}
	}

	// Update cache (best effort - DB has the truth)
	key := modeStrategyKey(userID, mode, strategy)
	if s.cache.IsHealthy() {
		data, _ := json.Marshal(config)
		if err := s.cache.Set(ctx, key, string(data), ModeStrategyCacheTTL); err != nil {
			s.logger.Warn("Failed to update mode strategy cache, will repopulate on next read",
				"key", key, "error", err)
		}
	}

	return nil
}

// GetAllStrategiesForMode retrieves all strategy configs for a mode
// Returns map[strategyName]*ModeStrategyConfig
func (s *SettingsCacheService) GetAllStrategiesForMode(ctx context.Context, userID string, mode string) (map[string]*database.ModeStrategyConfig, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	result := make(map[string]*database.ModeStrategyConfig)

	// Try to get all strategies from cache first using MGET
	keys := make([]string, len(ValidStrategies))
	for i, strategy := range ValidStrategies {
		keys[i] = modeStrategyKey(userID, mode, strategy)
	}

	values, err := s.cache.MGet(ctx, keys...)
	if err != nil {
		// Cache error, fall back to individual DB queries
		s.logger.Warn("MGET failed for mode strategies, falling back to DB", "error", err)
		return s.getAllStrategiesFromDB(ctx, userID, mode)
	}

	// Process cache results and identify misses
	misses := make([]string, 0)
	for i, val := range values {
		strategy := ValidStrategies[i]
		if val == nil {
			misses = append(misses, strategy)
			continue
		}

		valStr, ok := val.(string)
		if !ok || valStr == "" {
			misses = append(misses, strategy)
			continue
		}

		var config database.ModeStrategyConfig
		if err := json.Unmarshal([]byte(valStr), &config); err != nil {
			misses = append(misses, strategy)
			continue
		}

		result[strategy] = &config
	}

	// Populate misses from DB
	for _, strategy := range misses {
		config, err := s.GetModeStrategyConfig(ctx, userID, mode, strategy)
		if err != nil {
			s.logger.Warn("Failed to get strategy config", "mode", mode, "strategy", strategy, "error", err)
			// Use default on error
			config = database.DefaultModeStrategyConfig(mode, strategy)
		}
		result[strategy] = config
	}

	return result, nil
}

// getAllStrategiesFromDB loads all strategies for a mode directly from DB
func (s *SettingsCacheService) getAllStrategiesFromDB(ctx context.Context, userID string, mode string) (map[string]*database.ModeStrategyConfig, error) {
	result := make(map[string]*database.ModeStrategyConfig)

	rows, err := s.repo.GetModeStrategies(ctx, userID, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get mode strategies from DB: %w", err)
	}

	// Convert rows to map
	for _, row := range rows {
		var config database.ModeStrategyConfig
		if err := json.Unmarshal(row.Settings, &config); err != nil {
			s.logger.Warn("Failed to parse strategy settings", "strategy", row.Strategy, "error", err)
			continue
		}
		config.Enabled = row.Enabled
		config.Priority = row.Priority
		result[row.Strategy] = &config

		// Also populate cache for future reads
		if s.cache.IsHealthy() {
			key := modeStrategyKey(userID, mode, row.Strategy)
			data, _ := json.Marshal(config)
			s.cache.Set(ctx, key, string(data), ModeStrategyCacheTTL)
		}
	}

	// Fill in missing strategies with defaults
	for _, strategy := range ValidStrategies {
		if _, exists := result[strategy]; !exists {
			result[strategy] = database.DefaultModeStrategyConfig(mode, strategy)
		}
	}

	return result, nil
}

// InvalidateModeStrategyConfig removes a specific mode+strategy from cache
func (s *SettingsCacheService) InvalidateModeStrategyConfig(ctx context.Context, userID string, mode, strategy string) error {
	key := modeStrategyKey(userID, mode, strategy)
	return s.cache.Delete(ctx, key)
}

// InvalidateAllModeStrategies removes all mode+strategy configs for a user
func (s *SettingsCacheService) InvalidateAllModeStrategies(ctx context.Context, userID string) error {
	pattern := allModeStrategiesPattern(userID)
	return s.cache.DeletePattern(ctx, pattern)
}

// PopulateModeStrategiesFromDB loads all mode+strategy configs from DB to cache
// Called on user login to pre-warm the cache
func (s *SettingsCacheService) PopulateModeStrategiesFromDB(ctx context.Context, userID string) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	// Get all mode+strategy settings from DB
	rows, err := s.repo.GetAllModeStrategySettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get all mode strategy settings: %w", err)
	}

	// Cache each configuration
	for _, row := range rows {
		var config database.ModeStrategyConfig
		if err := json.Unmarshal(row.Settings, &config); err != nil {
			s.logger.Warn("Failed to parse mode strategy settings during populate",
				"mode", row.Mode, "strategy", row.Strategy, "error", err)
			continue
		}
		config.Enabled = row.Enabled
		config.Priority = row.Priority

		key := modeStrategyKey(userID, row.Mode, row.Strategy)
		data, _ := json.Marshal(config)
		if err := s.cache.Set(ctx, key, string(data), ModeStrategyCacheTTL); err != nil {
			s.logger.Warn("Failed to cache mode strategy during populate",
				"key", key, "error", err)
		}
	}

	// For any mode+strategy combinations not in DB, cache defaults
	dbConfigKeys := make(map[string]bool)
	for _, row := range rows {
		dbConfigKeys[row.Mode+":"+row.Strategy] = true
	}

	for _, mode := range TradingModes {
		for _, strategy := range ValidStrategies {
			configKey := mode + ":" + strategy
			if !dbConfigKeys[configKey] {
				config := database.DefaultModeStrategyConfig(mode, strategy)
				key := modeStrategyKey(userID, mode, strategy)
				data, _ := json.Marshal(config)
				s.cache.Set(ctx, key, string(data), ModeStrategyCacheTTL)
			}
		}
	}

	s.logger.Info("Populated mode strategy cache", "userID", userID, "dbRows", len(rows))
	return nil
}

// GetActiveStrategyForMode retrieves the currently active strategy for a mode based on regime
// Returns the config, the strategy name, and any error
// Selection logic:
// 1. Find strategies that support the given regime
// 2. Among those, select the enabled one with highest priority
// 3. If none match, return the mode's default strategy
func (s *SettingsCacheService) GetActiveStrategyForMode(ctx context.Context, userID string, mode string, regime string) (*database.ModeStrategyConfig, string, error) {
	// Get all strategies for this mode
	strategies, err := s.GetAllStrategiesForMode(ctx, userID, mode)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get strategies for mode %s: %w", mode, err)
	}

	// Find the best matching strategy
	var bestConfig *database.ModeStrategyConfig
	var bestStrategy string
	bestPriority := -1

	for strategyName, config := range strategies {
		// Skip disabled strategies
		if !config.Enabled {
			continue
		}

		// Check if this strategy supports the current regime
		supportsRegime := false
		for _, supportedRegime := range config.SupportedRegimes {
			if supportedRegime == regime {
				supportsRegime = true
				break
			}
		}

		if !supportsRegime {
			continue
		}

		// Select highest priority (higher number = higher priority)
		if config.Priority > bestPriority {
			bestPriority = config.Priority
			bestConfig = config
			bestStrategy = strategyName
		}
	}

	// If no matching strategy found, return the first enabled strategy or default
	if bestConfig == nil {
		// Try to find any enabled strategy
		for strategyName, config := range strategies {
			if config.Enabled && config.Priority > bestPriority {
				bestPriority = config.Priority
				bestConfig = config
				bestStrategy = strategyName
			}
		}
	}

	// If still no match, return trend_following as default
	if bestConfig == nil {
		defaultStrategy := "trend_following"
		bestConfig = strategies[defaultStrategy]
		if bestConfig == nil {
			bestConfig = database.DefaultModeStrategyConfig(mode, defaultStrategy)
		}
		bestStrategy = defaultStrategy
	}

	return bestConfig, bestStrategy, nil
}

// ============================================================================
// MODE-STRATEGY SECTION-LEVEL OPERATIONS (Story 11.41)
// Get/Set specific sections (e.g., "sltp", "mtf", "circuit_breaker") within a strategy
// ============================================================================

// ModeStrategySectionNames lists all valid section names for mode-strategy configs
var ModeStrategySectionNames = []string{
	"position_sizing",
	"timeframe",
	"mtf",
	"sltp",
	"confidence",
	"entry_conditions",
	"exit_conditions",
	"scoring",
	"circuit_breaker",
	"hedge",
	"averaging",
	"stale_release",
	"position_optimization",
	"funding_rate",
	"risk",
	"trend_divergence",
	"dynamic_ai_exit",
	"early_warning",
}

// GetModeStrategySection retrieves a specific section from a mode+strategy config
// Returns the section data as JSON bytes, or nil if section doesn't exist
func (s *SettingsCacheService) GetModeStrategySection(ctx context.Context, userID, mode, strategy, section string) ([]byte, error) {
	// First get the full config
	config, err := s.GetModeStrategyConfig(ctx, userID, mode, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to get mode strategy config: %w", err)
	}

	// Extract the requested section
	sectionData, err := extractModeStrategySection(config, section)
	if err != nil {
		return nil, fmt.Errorf("failed to extract section %s: %w", section, err)
	}

	// Marshal the section data to JSON
	sectionJSON, err := json.Marshal(sectionData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal section %s: %w", section, err)
	}

	return sectionJSON, nil
}

// SetModeStrategySection updates a specific section within a mode+strategy config
// Write-through: DB first, then cache
func (s *SettingsCacheService) SetModeStrategySection(ctx context.Context, userID, mode, strategy, section string, sectionData []byte) error {
	// First get the full config
	config, err := s.GetModeStrategyConfig(ctx, userID, mode, strategy)
	if err != nil {
		return fmt.Errorf("failed to get mode strategy config: %w", err)
	}

	// Merge the section data into the config
	if err := mergeModeStrategySection(config, section, sectionData); err != nil {
		return fmt.Errorf("failed to merge section %s: %w", section, err)
	}

	// Save the updated config (write-through to DB and cache)
	if err := s.SetModeStrategyConfig(ctx, userID, mode, strategy, config); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	return nil
}

// extractModeStrategySection extracts a specific section from ModeStrategyConfig
func extractModeStrategySection(config *database.ModeStrategyConfig, section string) (interface{}, error) {
	switch section {
	case "position_sizing":
		if config.PositionSizing != nil {
			return config.PositionSizing, nil
		}
		// Return legacy fields if PositionSizing is nil
		return &database.StrategyPositionSizing{
			Leverage:     config.Leverage,
			MaxPositions: config.MaxPositions,
			BaseSizeUSD:  config.BaseSizeUSD,
		}, nil
	case "timeframe":
		return &config.Timeframe, nil
	case "mtf":
		return config.MTF, nil
	case "sltp":
		return &config.SLTP, nil
	case "confidence":
		return &config.Confidence, nil
	case "entry_conditions":
		// Return v2 if available, otherwise legacy map
		if config.EntryConditionsV2 != nil {
			return config.EntryConditionsV2, nil
		}
		return config.EntryConditions, nil
	case "exit_conditions":
		return &config.ExitConditions, nil
	case "scoring":
		return &config.Scoring, nil
	case "circuit_breaker":
		return config.CircuitBreaker, nil
	case "hedge":
		return config.Hedge, nil
	case "averaging":
		return config.Averaging, nil
	case "stale_release":
		return config.StaleRelease, nil
	case "position_optimization":
		return config.PositionOptimization, nil
	case "funding_rate":
		return config.FundingRate, nil
	case "risk":
		return config.Risk, nil
	case "trend_divergence":
		return config.TrendDivergence, nil
	case "dynamic_ai_exit":
		return config.DynamicAIExit, nil
	case "early_warning":
		return config.EarlyWarning, nil
	default:
		return nil, fmt.Errorf("unknown section: %s", section)
	}
}

// mergeModeStrategySection merges section data into a ModeStrategyConfig
func mergeModeStrategySection(config *database.ModeStrategyConfig, section string, data []byte) error {
	var err error
	switch section {
	case "position_sizing":
		var ps database.StrategyPositionSizing
		if err = json.Unmarshal(data, &ps); err == nil {
			config.PositionSizing = &ps
			// Also update legacy fields for backward compatibility
			config.Leverage = ps.Leverage
			config.MaxPositions = ps.MaxPositions
			config.BaseSizeUSD = ps.BaseSizeUSD
		}
	case "timeframe":
		err = json.Unmarshal(data, &config.Timeframe)
	case "mtf":
		var mtf database.StrategyMTF
		if err = json.Unmarshal(data, &mtf); err == nil {
			config.MTF = &mtf
		}
	case "sltp":
		err = json.Unmarshal(data, &config.SLTP)
	case "confidence":
		err = json.Unmarshal(data, &config.Confidence)
	case "entry_conditions":
		// Try to parse as v2 first
		var ec database.StrategyEntryConditions
		if err = json.Unmarshal(data, &ec); err == nil {
			config.EntryConditionsV2 = &ec
		} else {
			// Fall back to legacy map
			var ecMap map[string]interface{}
			if err = json.Unmarshal(data, &ecMap); err == nil {
				config.EntryConditions = ecMap
			}
		}
	case "exit_conditions":
		err = json.Unmarshal(data, &config.ExitConditions)
	case "scoring":
		err = json.Unmarshal(data, &config.Scoring)
	case "circuit_breaker":
		var cb database.StrategyCircuitBreaker
		if err = json.Unmarshal(data, &cb); err == nil {
			config.CircuitBreaker = &cb
		}
	case "hedge":
		var h database.StrategyHedge
		if err = json.Unmarshal(data, &h); err == nil {
			config.Hedge = &h
		}
	case "averaging":
		var a database.StrategyAveraging
		if err = json.Unmarshal(data, &a); err == nil {
			config.Averaging = &a
		}
	case "stale_release":
		var sr database.StrategyStaleRelease
		if err = json.Unmarshal(data, &sr); err == nil {
			config.StaleRelease = &sr
		}
	case "position_optimization":
		var po database.StrategyPositionOptimization
		if err = json.Unmarshal(data, &po); err == nil {
			config.PositionOptimization = &po
		}
	case "funding_rate":
		var fr database.StrategyFundingRate
		if err = json.Unmarshal(data, &fr); err == nil {
			config.FundingRate = &fr
		}
	case "risk":
		var r database.StrategyRisk
		if err = json.Unmarshal(data, &r); err == nil {
			config.Risk = &r
		}
	case "trend_divergence":
		var td database.StrategyTrendDivergence
		if err = json.Unmarshal(data, &td); err == nil {
			config.TrendDivergence = &td
		}
	case "dynamic_ai_exit":
		var dae database.StrategyDynamicAIExit
		if err = json.Unmarshal(data, &dae); err == nil {
			config.DynamicAIExit = &dae
		}
	case "early_warning":
		var ew database.StrategyEarlyWarning
		if err = json.Unmarshal(data, &ew); err == nil {
			config.EarlyWarning = &ew
		}
	default:
		return fmt.Errorf("unknown section: %s", section)
	}

	if err != nil {
		return fmt.Errorf("failed to unmarshal section %s: %w", section, err)
	}

	return nil
}

// ResetModeStrategySection resets a specific section to default values
// Loads defaults from default-settings.json "modes" section
func (s *SettingsCacheService) ResetModeStrategySection(ctx context.Context, userID, mode, strategy, section string) error {
	// Load default config from default-settings.json
	defaultConfig, err := s.loadDefaultModeStrategyConfig(mode, strategy)
	if err != nil {
		return fmt.Errorf("failed to load default config: %w", err)
	}

	// Extract the default section data
	defaultSectionData, err := extractModeStrategySection(defaultConfig, section)
	if err != nil {
		return fmt.Errorf("failed to extract default section %s: %w", section, err)
	}

	// Marshal to JSON
	sectionJSON, err := json.Marshal(defaultSectionData)
	if err != nil {
		return fmt.Errorf("failed to marshal default section %s: %w", section, err)
	}

	// Set the section with default values
	return s.SetModeStrategySection(ctx, userID, mode, strategy, section, sectionJSON)
}

// loadDefaultModeStrategyConfig loads a default ModeStrategyConfig from default-settings.json
func (s *SettingsCacheService) loadDefaultModeStrategyConfig(mode, strategy string) (*database.ModeStrategyConfig, error) {
	// Try to load from default-settings.json "modes" section first
	configJSON, err := s.loadDefaultModeStrategyConfigJSON(mode, strategy)
	if err == nil && configJSON != nil {
		var config database.ModeStrategyConfig
		if err := json.Unmarshal(configJSON, &config); err == nil {
			return &config, nil
		}
		s.logger.Warn("Failed to parse default mode strategy config from JSON", "mode", mode, "strategy", strategy, "error", err)
	}

	// Fallback to hardcoded defaults
	return database.DefaultModeStrategyConfig(mode, strategy), nil
}

// loadDefaultModeStrategyConfigJSON loads raw JSON for a mode+strategy from default-settings.json
func (s *SettingsCacheService) loadDefaultModeStrategyConfigJSON(mode, strategy string) (json.RawMessage, error) {
	// Try admin defaults cache first
	adminKey := fmt.Sprintf("admin:defaults:mode:%s:strategy:%s", mode, strategy)
	cached, err := s.cache.Get(context.Background(), adminKey)
	if err == nil && cached != "" {
		return json.RawMessage(cached), nil
	}

	// Load directly from default-settings.json
	data, err := autopilot.GetDefaultModeStrategyConfigJSON(mode, strategy)
	if err != nil {
		return nil, err
	}

	// Cache for future use
	if s.cache.IsHealthy() && data != nil {
		s.cache.Set(context.Background(), adminKey, string(data), 24*time.Hour)
	}

	return data, nil
}

// ============================================================================
// SYMBOL SETTINGS CACHE (Story 9.12 Phase 1)
// ============================================================================

// Symbol settings cache key patterns
const (
	symbolSettingsKeyPrefix = "user:%s:symbol_settings"       // Hash key for all symbols
	symbolSettingsTTL       = 24 * time.Hour                  // Cache for 24 hours
)

// CachedSymbolSettings represents symbol settings stored in cache
type CachedSymbolSettings struct {
	Symbol           string  `json:"symbol"`
	Category         string  `json:"category"`
	MinConfidence    float64 `json:"min_confidence"`
	MaxPositionUSD   float64 `json:"max_position_usd"`
	SizeMultiplier   float64 `json:"size_multiplier"`
	LeverageOverride int     `json:"leverage_override"`
	Enabled          bool    `json:"enabled"`
	CustomROIPercent float64 `json:"custom_roi_percent"`
	Notes            string  `json:"notes"`
	TotalTrades      int     `json:"total_trades"`
	WinningTrades    int     `json:"winning_trades"`
	TotalPnL         float64 `json:"total_pnl"`
	WinRate          float64 `json:"win_rate"`
	AvgPnL           float64 `json:"avg_pnl"`
	LastUpdated      string  `json:"last_updated"`
}

// LoadSymbolSettings loads all symbol settings for a user into cache
// Called during user login as part of LoadUserSettings
func (s *SettingsCacheService) LoadSymbolSettings(ctx context.Context, userID string) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	// Get all symbol settings from database
	settings, err := s.repo.GetAllUserSymbolSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to load symbol settings from DB: %w", err)
	}

	if len(settings) == 0 {
		return nil // No settings to cache
	}

	// Store each symbol as a hash field
	hashKey := fmt.Sprintf(symbolSettingsKeyPrefix, userID)
	pipe := s.cache.client.Pipeline()

	for symbol, setting := range settings {
		cached := &CachedSymbolSettings{
			Symbol:           setting.Symbol,
			Category:         setting.Category,
			MinConfidence:    setting.MinConfidence,
			MaxPositionUSD:   setting.MaxPositionUSD,
			SizeMultiplier:   setting.SizeMultiplier,
			LeverageOverride: setting.LeverageOverride,
			Enabled:          setting.Enabled,
			CustomROIPercent: setting.CustomROIPercent,
			Notes:            setting.Notes,
			TotalTrades:      setting.TotalTrades,
			WinningTrades:    setting.WinningTrades,
			TotalPnL:         setting.TotalPnL,
			WinRate:          setting.WinRate,
			AvgPnL:           setting.AvgPnL,
			LastUpdated:      setting.LastUpdated,
		}

		data, _ := json.Marshal(cached)
		pipe.HSet(ctx, hashKey, symbol, string(data))
	}

	// Set TTL on the hash
	pipe.Expire(ctx, hashKey, symbolSettingsTTL)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to cache symbol settings: %w", err)
	}

	s.logger.Debug("Symbol settings cached", "userID", userID, "count", len(settings))
	return nil
}

// GetSymbolSettings retrieves symbol settings for a specific symbol (cache-first)
// Returns nil if symbol not found (use defaults)
func (s *SettingsCacheService) GetSymbolSettings(ctx context.Context, userID, symbol string) (*CachedSymbolSettings, error) {
	if !s.cache.IsHealthy() {
		// Fallback to DB
		return s.getSymbolSettingsFromDB(ctx, userID, symbol)
	}

	hashKey := fmt.Sprintf(symbolSettingsKeyPrefix, userID)
	data, err := s.cache.client.HGet(ctx, hashKey, symbol).Result()

	if err == redis.Nil {
		// Not in cache - try DB
		return s.getSymbolSettingsFromDB(ctx, userID, symbol)
	}
	if err != nil {
		s.logger.Debug("Cache read error for symbol settings", "error", err)
		return s.getSymbolSettingsFromDB(ctx, userID, symbol)
	}

	var settings CachedSymbolSettings
	if err := json.Unmarshal([]byte(data), &settings); err != nil {
		return s.getSymbolSettingsFromDB(ctx, userID, symbol)
	}

	return &settings, nil
}

// getSymbolSettingsFromDB retrieves symbol settings from database and caches it
func (s *SettingsCacheService) getSymbolSettingsFromDB(ctx context.Context, userID, symbol string) (*CachedSymbolSettings, error) {
	dbSettings, err := s.repo.GetUserSymbolSettings(ctx, userID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbol settings from DB: %w", err)
	}
	if dbSettings == nil {
		return nil, nil // Symbol not configured
	}

	cached := &CachedSymbolSettings{
		Symbol:           dbSettings.Symbol,
		Category:         dbSettings.Category,
		MinConfidence:    dbSettings.MinConfidence,
		MaxPositionUSD:   dbSettings.MaxPositionUSD,
		SizeMultiplier:   dbSettings.SizeMultiplier,
		LeverageOverride: dbSettings.LeverageOverride,
		Enabled:          dbSettings.Enabled,
		CustomROIPercent: dbSettings.CustomROIPercent,
		Notes:            dbSettings.Notes,
		TotalTrades:      dbSettings.TotalTrades,
		WinningTrades:    dbSettings.WinningTrades,
		TotalPnL:         dbSettings.TotalPnL,
		WinRate:          dbSettings.WinRate,
		AvgPnL:           dbSettings.AvgPnL,
		LastUpdated:      dbSettings.LastUpdated,
	}

	// Cache for future reads
	if s.cache.IsHealthy() {
		hashKey := fmt.Sprintf(symbolSettingsKeyPrefix, userID)
		data, _ := json.Marshal(cached)
		s.cache.client.HSet(ctx, hashKey, symbol, string(data))
	}

	return cached, nil
}

// GetAllSymbolSettings retrieves all symbol settings for a user (cache-first)
func (s *SettingsCacheService) GetAllSymbolSettings(ctx context.Context, userID string) (map[string]*CachedSymbolSettings, error) {
	if !s.cache.IsHealthy() {
		return s.getAllSymbolSettingsFromDB(ctx, userID)
	}

	hashKey := fmt.Sprintf(symbolSettingsKeyPrefix, userID)
	data, err := s.cache.client.HGetAll(ctx, hashKey).Result()

	if err != nil || len(data) == 0 {
		return s.getAllSymbolSettingsFromDB(ctx, userID)
	}

	result := make(map[string]*CachedSymbolSettings)
	for symbol, jsonData := range data {
		var settings CachedSymbolSettings
		if err := json.Unmarshal([]byte(jsonData), &settings); err != nil {
			continue
		}
		result[symbol] = &settings
	}

	return result, nil
}

// getAllSymbolSettingsFromDB retrieves all symbol settings from database
func (s *SettingsCacheService) getAllSymbolSettingsFromDB(ctx context.Context, userID string) (map[string]*CachedSymbolSettings, error) {
	dbSettings, err := s.repo.GetAllUserSymbolSettings(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all symbol settings from DB: %w", err)
	}

	result := make(map[string]*CachedSymbolSettings)
	for symbol, setting := range dbSettings {
		result[symbol] = &CachedSymbolSettings{
			Symbol:           setting.Symbol,
			Category:         setting.Category,
			MinConfidence:    setting.MinConfidence,
			MaxPositionUSD:   setting.MaxPositionUSD,
			SizeMultiplier:   setting.SizeMultiplier,
			LeverageOverride: setting.LeverageOverride,
			Enabled:          setting.Enabled,
			CustomROIPercent: setting.CustomROIPercent,
			Notes:            setting.Notes,
			TotalTrades:      setting.TotalTrades,
			WinningTrades:    setting.WinningTrades,
			TotalPnL:         setting.TotalPnL,
			WinRate:          setting.WinRate,
			AvgPnL:           setting.AvgPnL,
			LastUpdated:      setting.LastUpdated,
		}
	}

	// Populate cache for future reads
	if s.cache.IsHealthy() && len(result) > 0 {
		_ = s.LoadSymbolSettings(ctx, userID)
	}

	return result, nil
}

// SetSymbolSettings updates symbol settings (write-through: DB first, then cache)
func (s *SettingsCacheService) SetSymbolSettings(ctx context.Context, userID string, settings *database.UserSymbolSettings) error {
	// Write to DB first
	settings.UserID = userID
	if err := s.repo.UpsertUserSymbolSettings(ctx, settings); err != nil {
		return fmt.Errorf("failed to save symbol settings to DB: %w", err)
	}

	// Update cache
	if s.cache.IsHealthy() {
		cached := &CachedSymbolSettings{
			Symbol:           settings.Symbol,
			Category:         settings.Category,
			MinConfidence:    settings.MinConfidence,
			MaxPositionUSD:   settings.MaxPositionUSD,
			SizeMultiplier:   settings.SizeMultiplier,
			LeverageOverride: settings.LeverageOverride,
			Enabled:          settings.Enabled,
			CustomROIPercent: settings.CustomROIPercent,
			Notes:            settings.Notes,
			TotalTrades:      settings.TotalTrades,
			WinningTrades:    settings.WinningTrades,
			TotalPnL:         settings.TotalPnL,
			WinRate:          settings.WinRate,
			AvgPnL:           settings.AvgPnL,
			LastUpdated:      settings.LastUpdated,
		}

		hashKey := fmt.Sprintf(symbolSettingsKeyPrefix, userID)
		data, _ := json.Marshal(cached)
		s.cache.client.HSet(ctx, hashKey, settings.Symbol, string(data))
	}

	return nil
}

// InvalidateSymbolSettings removes a specific symbol's settings from cache
func (s *SettingsCacheService) InvalidateSymbolSettings(ctx context.Context, userID, symbol string) error {
	if !s.cache.IsHealthy() {
		return nil
	}

	hashKey := fmt.Sprintf(symbolSettingsKeyPrefix, userID)
	return s.cache.client.HDel(ctx, hashKey, symbol).Err()
}

// InvalidateAllSymbolSettings removes all symbol settings from cache for a user
func (s *SettingsCacheService) InvalidateAllSymbolSettings(ctx context.Context, userID string) error {
	if !s.cache.IsHealthy() {
		return nil
	}

	hashKey := fmt.Sprintf(symbolSettingsKeyPrefix, userID)
	return s.cache.client.Del(ctx, hashKey).Err()
}

// GetEffectiveConfidence returns the effective minimum confidence for a symbol
// Uses symbol-specific override if set, otherwise returns the global value
func (s *SettingsCacheService) GetEffectiveConfidence(ctx context.Context, userID, symbol string, globalMinConfidence float64) float64 {
	settings, err := s.GetSymbolSettings(ctx, userID, symbol)
	if err != nil || settings == nil {
		return globalMinConfidence
	}

	// If symbol has a custom min confidence > 0, use it
	if settings.MinConfidence > 0 {
		return settings.MinConfidence
	}

	return globalMinConfidence
}

// GetEffectivePositionSize returns the effective max position size for a symbol
// Uses symbol-specific override if set, otherwise returns the global value
// Also applies size multiplier if set
func (s *SettingsCacheService) GetEffectivePositionSize(ctx context.Context, userID, symbol string, globalMaxUSD float64) float64 {
	settings, err := s.GetSymbolSettings(ctx, userID, symbol)
	if err != nil || settings == nil {
		return globalMaxUSD
	}

	// Start with global or symbol-specific max
	effectiveMax := globalMaxUSD
	if settings.MaxPositionUSD > 0 {
		effectiveMax = settings.MaxPositionUSD
	}

	// Apply size multiplier
	if settings.SizeMultiplier > 0 && settings.SizeMultiplier != 1.0 {
		effectiveMax *= settings.SizeMultiplier
	}

	return effectiveMax
}

// IsSymbolEnabled checks if a symbol is enabled for trading (cache-first)
func (s *SettingsCacheService) IsSymbolEnabled(ctx context.Context, userID, symbol string) bool {
	settings, err := s.GetSymbolSettings(ctx, userID, symbol)
	if err != nil || settings == nil {
		return true // Default to enabled if no settings
	}

	return settings.Enabled
}

// ============================================================================
// COIN CONFLUENCE CONFIG CACHE (Story 9.12 Phase 3)
// Per-coin entry confluence settings (ADX, VWAP, Volume, Pivots, EMA thresholds)
// ============================================================================

// Confluence config cache key patterns
const (
	confluenceConfigKeyPrefix = "user:%s:confluence_config" // Hash key for all symbols
	confluenceConfigTTL       = 24 * time.Hour              // Cache for 24 hours
)

// CachedConfluenceConfig represents confluence config stored in cache
// Mirrors autopilot.CoinConfluenceConfig for cache storage
type CachedConfluenceConfig struct {
	Symbol             string                 `json:"symbol"`
	Tier               string                 `json:"tier"`
	Enabled            bool                   `json:"enabled"`
	ADXMultiplier      float64                `json:"adx_multiplier"`
	ScalpADX           float64                `json:"scalp_adx"`
	SwingADX           float64                `json:"swing_adx"`
	PositionADX        float64                `json:"position_adx"`
	VolumeMultiplier   float64                `json:"volume_multiplier"`
	VolumePeriod       int                    `json:"volume_period"`
	VWAPPeriod         int                    `json:"vwap_period"`
	VWAPEnabled        bool                   `json:"vwap_enabled"`
	PivotProximity     float64                `json:"pivot_proximity"`
	PivotEnabled       bool                   `json:"pivot_enabled"`
	EMAFastPeriod      int                    `json:"ema_fast_period"`
	EMASlowPeriod      int                    `json:"ema_slow_period"`
	EMAEnabled         bool                   `json:"ema_enabled"`
	MinConfluence      int                    `json:"min_confluence"`
	ScalpMinConfluence int                    `json:"scalp_min_confluence"`
	CustomWeights      map[string]interface{} `json:"custom_weights,omitempty"`
	Notes              string                 `json:"notes"`
}

// LoadCoinConfluenceConfigs loads all confluence configs for a user into cache
// Called during user login as part of LoadUserSettings
func (s *SettingsCacheService) LoadCoinConfluenceConfigs(ctx context.Context, userID string) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	// Get all confluence configs from database
	configs, err := s.repo.GetAllUserCoinConfluenceConfigs(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to load confluence configs from DB: %w", err)
	}

	if len(configs) == 0 {
		return nil // No configs to cache
	}

	// Store each config as a hash field
	hashKey := fmt.Sprintf(confluenceConfigKeyPrefix, userID)
	pipe := s.cache.client.Pipeline()

	for symbol, dbConfig := range configs {
		cached := &CachedConfluenceConfig{
			Symbol:             dbConfig.Symbol,
			Tier:               dbConfig.Tier,
			Enabled:            dbConfig.Enabled,
			ADXMultiplier:      dbConfig.ADXMultiplier,
			ScalpADX:           dbConfig.ScalpADX,
			SwingADX:           dbConfig.SwingADX,
			PositionADX:        dbConfig.PositionADX,
			VolumeMultiplier:   dbConfig.VolumeMultiplier,
			VolumePeriod:       dbConfig.VolumePeriod,
			VWAPPeriod:         dbConfig.VWAPPeriod,
			VWAPEnabled:        dbConfig.VWAPEnabled,
			PivotProximity:     dbConfig.PivotProximity,
			PivotEnabled:       dbConfig.PivotEnabled,
			EMAFastPeriod:      dbConfig.EMAFastPeriod,
			EMASlowPeriod:      dbConfig.EMASlowPeriod,
			EMAEnabled:         dbConfig.EMAEnabled,
			MinConfluence:      dbConfig.MinConfluence,
			ScalpMinConfluence: dbConfig.ScalpMinConfluence,
			CustomWeights:      dbConfig.CustomWeights,
			Notes:              dbConfig.Notes,
		}

		data, _ := json.Marshal(cached)
		pipe.HSet(ctx, hashKey, symbol, string(data))
	}

	// Set TTL on the hash
	pipe.Expire(ctx, hashKey, confluenceConfigTTL)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to cache confluence configs: %w", err)
	}

	s.logger.Debug("Confluence configs cached", "userID", userID, "count", len(configs))
	return nil
}

// GetCoinConfluenceConfig retrieves confluence config for a specific symbol (cache-first)
// Returns nil if symbol not found (caller should use tier-based defaults)
func (s *SettingsCacheService) GetCoinConfluenceConfig(ctx context.Context, userID, symbol string) (*CachedConfluenceConfig, error) {
	if !s.cache.IsHealthy() {
		// Fallback to DB
		return s.getCoinConfluenceConfigFromDB(ctx, userID, symbol)
	}

	hashKey := fmt.Sprintf(confluenceConfigKeyPrefix, userID)
	data, err := s.cache.client.HGet(ctx, hashKey, symbol).Result()

	if err == redis.Nil {
		// Not in cache - try DB
		return s.getCoinConfluenceConfigFromDB(ctx, userID, symbol)
	}
	if err != nil {
		s.logger.Debug("Cache read error for confluence config", "error", err)
		return s.getCoinConfluenceConfigFromDB(ctx, userID, symbol)
	}

	var config CachedConfluenceConfig
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return s.getCoinConfluenceConfigFromDB(ctx, userID, symbol)
	}

	return &config, nil
}

// getCoinConfluenceConfigFromDB retrieves confluence config from database and caches it
func (s *SettingsCacheService) getCoinConfluenceConfigFromDB(ctx context.Context, userID, symbol string) (*CachedConfluenceConfig, error) {
	dbConfig, err := s.repo.GetUserCoinConfluenceConfig(ctx, userID, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get confluence config from DB: %w", err)
	}
	if dbConfig == nil {
		return nil, nil // Symbol not configured - use tier defaults
	}

	cached := &CachedConfluenceConfig{
		Symbol:             dbConfig.Symbol,
		Tier:               dbConfig.Tier,
		Enabled:            dbConfig.Enabled,
		ADXMultiplier:      dbConfig.ADXMultiplier,
		ScalpADX:           dbConfig.ScalpADX,
		SwingADX:           dbConfig.SwingADX,
		PositionADX:        dbConfig.PositionADX,
		VolumeMultiplier:   dbConfig.VolumeMultiplier,
		VolumePeriod:       dbConfig.VolumePeriod,
		VWAPPeriod:         dbConfig.VWAPPeriod,
		VWAPEnabled:        dbConfig.VWAPEnabled,
		PivotProximity:     dbConfig.PivotProximity,
		PivotEnabled:       dbConfig.PivotEnabled,
		EMAFastPeriod:      dbConfig.EMAFastPeriod,
		EMASlowPeriod:      dbConfig.EMASlowPeriod,
		EMAEnabled:         dbConfig.EMAEnabled,
		MinConfluence:      dbConfig.MinConfluence,
		ScalpMinConfluence: dbConfig.ScalpMinConfluence,
		CustomWeights:      dbConfig.CustomWeights,
		Notes:              dbConfig.Notes,
	}

	// Cache for future reads
	if s.cache.IsHealthy() {
		hashKey := fmt.Sprintf(confluenceConfigKeyPrefix, userID)
		data, _ := json.Marshal(cached)
		s.cache.client.HSet(ctx, hashKey, symbol, string(data))
	}

	return cached, nil
}

// GetAllCoinConfluenceConfigs retrieves all confluence configs for a user (cache-first)
func (s *SettingsCacheService) GetAllCoinConfluenceConfigs(ctx context.Context, userID string) (map[string]*CachedConfluenceConfig, error) {
	if !s.cache.IsHealthy() {
		return s.getAllCoinConfluenceConfigsFromDB(ctx, userID)
	}

	hashKey := fmt.Sprintf(confluenceConfigKeyPrefix, userID)
	data, err := s.cache.client.HGetAll(ctx, hashKey).Result()

	if err != nil || len(data) == 0 {
		return s.getAllCoinConfluenceConfigsFromDB(ctx, userID)
	}

	result := make(map[string]*CachedConfluenceConfig)
	for symbol, jsonData := range data {
		var config CachedConfluenceConfig
		if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
			continue
		}
		result[symbol] = &config
	}

	return result, nil
}

// getAllCoinConfluenceConfigsFromDB retrieves all confluence configs from database
func (s *SettingsCacheService) getAllCoinConfluenceConfigsFromDB(ctx context.Context, userID string) (map[string]*CachedConfluenceConfig, error) {
	dbConfigs, err := s.repo.GetAllUserCoinConfluenceConfigs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all confluence configs from DB: %w", err)
	}

	result := make(map[string]*CachedConfluenceConfig)
	for symbol, dbConfig := range dbConfigs {
		result[symbol] = &CachedConfluenceConfig{
			Symbol:             dbConfig.Symbol,
			Tier:               dbConfig.Tier,
			Enabled:            dbConfig.Enabled,
			ADXMultiplier:      dbConfig.ADXMultiplier,
			ScalpADX:           dbConfig.ScalpADX,
			SwingADX:           dbConfig.SwingADX,
			PositionADX:        dbConfig.PositionADX,
			VolumeMultiplier:   dbConfig.VolumeMultiplier,
			VolumePeriod:       dbConfig.VolumePeriod,
			VWAPPeriod:         dbConfig.VWAPPeriod,
			VWAPEnabled:        dbConfig.VWAPEnabled,
			PivotProximity:     dbConfig.PivotProximity,
			PivotEnabled:       dbConfig.PivotEnabled,
			EMAFastPeriod:      dbConfig.EMAFastPeriod,
			EMASlowPeriod:      dbConfig.EMASlowPeriod,
			EMAEnabled:         dbConfig.EMAEnabled,
			MinConfluence:      dbConfig.MinConfluence,
			ScalpMinConfluence: dbConfig.ScalpMinConfluence,
			CustomWeights:      dbConfig.CustomWeights,
			Notes:              dbConfig.Notes,
		}
	}

	// Populate cache for future reads
	if s.cache.IsHealthy() && len(result) > 0 {
		_ = s.LoadCoinConfluenceConfigs(ctx, userID)
	}

	return result, nil
}

// SetCoinConfluenceConfig updates confluence config (write-through: DB first, then cache)
func (s *SettingsCacheService) SetCoinConfluenceConfig(ctx context.Context, userID string, config *database.UserCoinConfluenceConfig) error {
	// Write to DB first
	config.UserID = userID
	if err := s.repo.UpsertUserCoinConfluenceConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to save confluence config to DB: %w", err)
	}

	// Update cache
	if s.cache.IsHealthy() {
		cached := &CachedConfluenceConfig{
			Symbol:             config.Symbol,
			Tier:               config.Tier,
			Enabled:            config.Enabled,
			ADXMultiplier:      config.ADXMultiplier,
			ScalpADX:           config.ScalpADX,
			SwingADX:           config.SwingADX,
			PositionADX:        config.PositionADX,
			VolumeMultiplier:   config.VolumeMultiplier,
			VolumePeriod:       config.VolumePeriod,
			VWAPPeriod:         config.VWAPPeriod,
			VWAPEnabled:        config.VWAPEnabled,
			PivotProximity:     config.PivotProximity,
			PivotEnabled:       config.PivotEnabled,
			EMAFastPeriod:      config.EMAFastPeriod,
			EMASlowPeriod:      config.EMASlowPeriod,
			EMAEnabled:         config.EMAEnabled,
			MinConfluence:      config.MinConfluence,
			ScalpMinConfluence: config.ScalpMinConfluence,
			CustomWeights:      config.CustomWeights,
			Notes:              config.Notes,
		}

		hashKey := fmt.Sprintf(confluenceConfigKeyPrefix, userID)
		data, _ := json.Marshal(cached)
		s.cache.client.HSet(ctx, hashKey, config.Symbol, string(data))
	}

	return nil
}

// InvalidateCoinConfluenceConfig removes a specific symbol's confluence config from cache
func (s *SettingsCacheService) InvalidateCoinConfluenceConfig(ctx context.Context, userID, symbol string) error {
	if !s.cache.IsHealthy() {
		return nil
	}

	hashKey := fmt.Sprintf(confluenceConfigKeyPrefix, userID)
	return s.cache.client.HDel(ctx, hashKey, symbol).Err()
}

// InvalidateAllCoinConfluenceConfigs removes all confluence configs from cache for a user
func (s *SettingsCacheService) InvalidateAllCoinConfluenceConfigs(ctx context.Context, userID string) error {
	if !s.cache.IsHealthy() {
		return nil
	}

	hashKey := fmt.Sprintf(confluenceConfigKeyPrefix, userID)
	return s.cache.client.Del(ctx, hashKey).Err()
}

// DeleteCoinConfluenceConfig deletes a confluence config (DB + cache)
// Used when reverting to tier-based defaults
func (s *SettingsCacheService) DeleteCoinConfluenceConfig(ctx context.Context, userID, symbol string) error {
	// Delete from DB first
	if err := s.repo.DeleteUserCoinConfluenceConfig(ctx, userID, symbol); err != nil {
		return fmt.Errorf("failed to delete confluence config from DB: %w", err)
	}

	// Then invalidate cache
	return s.InvalidateCoinConfluenceConfig(ctx, userID, symbol)
}

// GetCoinConfluenceConfigCached implements autopilot.ConfluenceConfigCache interface
// Returns per-coin confluence config converted to autopilot.CoinConfluenceConfig
// Returns nil if not found (caller should use tier-based defaults via autopilot.DefaultCoinConfluenceConfig)
func (s *SettingsCacheService) GetCoinConfluenceConfigCached(ctx context.Context, userID, symbol string) (*autopilot.CoinConfluenceConfig, error) {
	cached, err := s.GetCoinConfluenceConfig(ctx, userID, symbol)
	if err != nil {
		return nil, err
	}
	if cached == nil {
		return nil, nil // Not found - caller uses tier defaults
	}

	// Convert CachedConfluenceConfig to autopilot.CoinConfluenceConfig
	return &autopilot.CoinConfluenceConfig{
		Symbol:             cached.Symbol,
		Tier:               autopilot.CoinTier(cached.Tier),
		Enabled:            cached.Enabled,
		ADXMultiplier:      cached.ADXMultiplier,
		ScalpADX:           cached.ScalpADX,
		SwingADX:           cached.SwingADX,
		PositionADX:        cached.PositionADX,
		VolumeMultiplier:   cached.VolumeMultiplier,
		VolumePeriod:       cached.VolumePeriod,
		VWAPPeriod:         cached.VWAPPeriod,
		VWAPEnabled:        cached.VWAPEnabled,
		PivotProximity:     cached.PivotProximity,
		PivotEnabled:       cached.PivotEnabled,
		EMAFastPeriod:      cached.EMAFastPeriod,
		EMASlowPeriod:      cached.EMASlowPeriod,
		EMAEnabled:         cached.EMAEnabled,
		MinConfluence:      cached.MinConfluence,
		ScalpMinConfluence: cached.ScalpMinConfluence,
		Notes:              cached.Notes,
	}, nil
}

// ============================================================================
// CATEGORY SETTINGS CACHE (Story 9.12 Phase 5b)
// Per-performance-category confidence/size adjustments
// ============================================================================

// CategorySettingsKey is the cache key pattern for category settings
const categorySettingsKeyPrefix = "user:%s:category_settings"

// DefaultCategorySettings returns the default category adjustments
// These are global defaults used when no per-user customization exists
func DefaultCategorySettings() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"confidence_boost": {
			"best":      -10, // Reduce confidence requirement (more trades)
			"good":      -5,
			"neutral":   0,
			"poor":      5,  // Increase confidence requirement (fewer trades)
			"worst":     10,
			"blacklist": 100, // Effectively disable
		},
		"size_multiplier": {
			"best":      1.5, // Increase position size
			"good":      1.2,
			"neutral":   1.0,
			"poor":      0.7,
			"worst":     0.5,
			"blacklist": 0.0, // No position
		},
	}
}

// GetCategorySettings retrieves category settings for a user (cache-first)
// Returns default settings if none customized
func (s *SettingsCacheService) GetCategorySettings(ctx context.Context, userID string) (map[string]map[string]float64, error) {
	if !s.cache.IsHealthy() {
		// Return defaults if cache unavailable
		return DefaultCategorySettings(), nil
	}

	key := fmt.Sprintf(categorySettingsKeyPrefix, userID)
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var settings map[string]map[string]float64
		if err := json.Unmarshal([]byte(cached), &settings); err == nil {
			return settings, nil
		}
	}

	// Cache miss - return defaults (could also check DB here if we add a table)
	defaults := DefaultCategorySettings()

	// Cache for future reads
	data, _ := json.Marshal(defaults)
	s.cache.Set(ctx, key, string(data), symbolSettingsTTL)

	return defaults, nil
}

// SetCategorySettings updates category settings (cache-first for now, DB integration pending)
// Story 9.12: This will be migrated to DB in a future phase when user_category_settings table is added
func (s *SettingsCacheService) SetCategorySettings(ctx context.Context, userID string, settings map[string]map[string]float64) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	key := fmt.Sprintf(categorySettingsKeyPrefix, userID)
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal category settings: %w", err)
	}

	return s.cache.Set(ctx, key, string(data), symbolSettingsTTL)
}

// ============================================================================
// SYMBOL PERFORMANCE REPORT (Story 9.12 Phase 5b)
// Calculated from user_symbol_settings table
// ============================================================================

// SymbolPerformanceReport represents a symbol's trading performance summary
type SymbolPerformanceReport struct {
	Symbol         string  `json:"symbol"`
	Category       string  `json:"category"`
	TotalTrades    int     `json:"total_trades"`
	WinningTrades  int     `json:"winning_trades"`
	LosingTrades   int     `json:"losing_trades"`
	TotalPnL       float64 `json:"total_pnl"`
	WinRate        float64 `json:"win_rate"`
	AvgPnL         float64 `json:"avg_pnl"`
	MinConfidence  float64 `json:"min_confidence"`
	MaxPositionUSD float64 `json:"max_position_usd"`
	SizeMultiplier float64 `json:"size_multiplier"`
	Enabled        bool    `json:"enabled"`
}

// GetSymbolPerformanceReport generates performance reports from cached symbol settings
func (s *SettingsCacheService) GetSymbolPerformanceReport(ctx context.Context, userID string, globalMinConfidence, globalMaxUSD float64) ([]SymbolPerformanceReport, error) {
	allSettings, err := s.GetAllSymbolSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	var reports []SymbolPerformanceReport
	for _, settings := range allSettings {
		report := SymbolPerformanceReport{
			Symbol:         settings.Symbol,
			Category:       settings.Category,
			TotalTrades:    settings.TotalTrades,
			WinningTrades:  settings.WinningTrades,
			LosingTrades:   settings.TotalTrades - settings.WinningTrades,
			TotalPnL:       settings.TotalPnL,
			WinRate:        settings.WinRate,
			AvgPnL:         settings.AvgPnL,
			MinConfidence:  s.GetEffectiveConfidence(ctx, userID, settings.Symbol, globalMinConfidence),
			MaxPositionUSD: s.GetEffectivePositionSize(ctx, userID, settings.Symbol, globalMaxUSD),
			SizeMultiplier: settings.SizeMultiplier,
			Enabled:        settings.Enabled,
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// GetSymbolsByCategory returns all symbols in a given category
func (s *SettingsCacheService) GetSymbolsByCategory(ctx context.Context, userID, category string) ([]string, error) {
	allSettings, err := s.GetAllSymbolSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	var symbols []string
	for symbol, settings := range allSettings {
		if settings.Category == category {
			symbols = append(symbols, symbol)
		}
	}

	return symbols, nil
}

// BlacklistSymbol adds a symbol to the blacklist category
func (s *SettingsCacheService) BlacklistSymbol(ctx context.Context, userID, symbol, reason string) error {
	// Get existing settings or create new
	existing, _ := s.GetSymbolSettings(ctx, userID, symbol)

	dbSettings := &database.UserSymbolSettings{
		UserID:         userID,
		Symbol:         symbol,
		Category:       "blacklist",
		Enabled:        false,
		Notes:          reason,
		SizeMultiplier: 1.0,
		LastUpdated:    time.Now().Format("2006-01-02 15:04:05"),
	}

	// Preserve existing performance data if available
	if existing != nil {
		dbSettings.TotalTrades = existing.TotalTrades
		dbSettings.WinningTrades = existing.WinningTrades
		dbSettings.TotalPnL = existing.TotalPnL
		dbSettings.WinRate = existing.WinRate
		dbSettings.AvgPnL = existing.AvgPnL
		dbSettings.MinConfidence = existing.MinConfidence
		dbSettings.MaxPositionUSD = existing.MaxPositionUSD
		if existing.SizeMultiplier > 0 {
			dbSettings.SizeMultiplier = existing.SizeMultiplier
		}
	}

	return s.SetSymbolSettings(ctx, userID, dbSettings)
}

// UnblacklistSymbol removes a symbol from the blacklist
func (s *SettingsCacheService) UnblacklistSymbol(ctx context.Context, userID, symbol string) error {
	existing, _ := s.GetSymbolSettings(ctx, userID, symbol)
	if existing == nil {
		return nil // Symbol doesn't exist, nothing to unblacklist
	}

	dbSettings := &database.UserSymbolSettings{
		UserID:           userID,
		Symbol:           symbol,
		Category:         "neutral", // Reset to neutral
		Enabled:          true,
		Notes:            existing.Notes,
		MinConfidence:    existing.MinConfidence,
		MaxPositionUSD:   existing.MaxPositionUSD,
		SizeMultiplier:   existing.SizeMultiplier,
		LeverageOverride: existing.LeverageOverride,
		CustomROIPercent: existing.CustomROIPercent,
		TotalTrades:      existing.TotalTrades,
		WinningTrades:    existing.WinningTrades,
		TotalPnL:         existing.TotalPnL,
		WinRate:          existing.WinRate,
		AvgPnL:           existing.AvgPnL,
		LastUpdated:      time.Now().Format("2006-01-02 15:04:05"),
	}

	if dbSettings.SizeMultiplier == 0 {
		dbSettings.SizeMultiplier = 1.0
	}

	return s.SetSymbolSettings(ctx, userID, dbSettings)
}

// RecalculateSymbolPerformance updates symbol performance from database stats
// and recategorizes based on performance thresholds
func (s *SettingsCacheService) RecalculateSymbolPerformance(ctx context.Context, userID string, dbStats map[string]interface{}) (int, error) {
	updated := 0
	now := time.Now().Format("2006-01-02 15:04:05")

	for symbol, statsRaw := range dbStats {
		stats, ok := statsRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Get existing settings or create new
		existing, _ := s.GetSymbolSettings(ctx, userID, symbol)

		dbSettings := &database.UserSymbolSettings{
			UserID:         userID,
			Symbol:         symbol,
			SizeMultiplier: 1.0,
			Enabled:        true,
			LastUpdated:    now,
		}

		// Preserve existing settings
		if existing != nil {
			dbSettings.Category = existing.Category
			dbSettings.MinConfidence = existing.MinConfidence
			dbSettings.MaxPositionUSD = existing.MaxPositionUSD
			dbSettings.LeverageOverride = existing.LeverageOverride
			dbSettings.Notes = existing.Notes
			if existing.SizeMultiplier > 0 {
				dbSettings.SizeMultiplier = existing.SizeMultiplier
			}
			dbSettings.Enabled = existing.Enabled
		}

		// Update performance metrics from DB stats
		if v, ok := stats["total_trades"].(int); ok {
			dbSettings.TotalTrades = v
		}
		if v, ok := stats["winning_trades"].(int); ok {
			dbSettings.WinningTrades = v
		}
		if v, ok := stats["total_pnl"].(float64); ok {
			dbSettings.TotalPnL = v
		}
		if v, ok := stats["avg_pnl"].(float64); ok {
			dbSettings.AvgPnL = v
		}

		// Calculate win rate
		if dbSettings.TotalTrades > 0 {
			dbSettings.WinRate = float64(dbSettings.WinningTrades) / float64(dbSettings.TotalTrades) * 100
		}

		// Recategorize based on performance (unless blacklisted)
		if dbSettings.Category != "blacklist" {
			dbSettings.Category = calculatePerformanceCategory(dbSettings.TotalPnL, dbSettings.WinRate, dbSettings.TotalTrades)
		}

		if err := s.SetSymbolSettings(ctx, userID, dbSettings); err != nil {
			s.logger.Warn("Failed to update symbol settings", "symbol", symbol, "error", err)
			continue
		}
		updated++
	}

	return updated, nil
}

// calculatePerformanceCategory determines category based on performance metrics
func calculatePerformanceCategory(totalPnL, winRate float64, totalTrades int) string {
	// Need minimum trades for categorization
	if totalTrades < 3 {
		return "neutral"
	}

	// Score-based categorization
	score := 0.0

	// PnL contribution (weighted heavily)
	if totalPnL > 100 {
		score += 3
	} else if totalPnL > 50 {
		score += 2
	} else if totalPnL > 0 {
		score += 1
	} else if totalPnL > -50 {
		score -= 1
	} else if totalPnL > -100 {
		score -= 2
	} else {
		score -= 3
	}

	// Win rate contribution
	if winRate > 70 {
		score += 2
	} else if winRate > 55 {
		score += 1
	} else if winRate < 40 {
		score -= 1
	} else if winRate < 30 {
		score -= 2
	}

	// Map score to category
	switch {
	case score >= 4:
		return "best"
	case score >= 2:
		return "good"
	case score >= -1:
		return "neutral"
	case score >= -3:
		return "poor"
	default:
		return "worst"
	}
}

// ============================================================================
// GINIE SETTINGS CACHE (Story 9.12)
// User Ginie autopilot settings (dry run, auto-start, morning auto-block, etc.)
// NOTE: Currently Ginie settings are NOT cached in Redis - they read directly from DB.
// This method exists for consistency and future cache support.
// ============================================================================

// Ginie settings cache key pattern
const ginieSettingsKeyPrefix = "user:%s:ginie_settings"

// InvalidateGinieSettings removes Ginie settings from cache for a user
// Should be called after updating any Ginie settings in the database.
// Currently a no-op since Ginie settings are not cached, but establishes the pattern.
func (s *SettingsCacheService) InvalidateGinieSettings(ctx context.Context, userID string) error {
	if !s.cache.IsHealthy() {
		return nil
	}

	key := fmt.Sprintf(ginieSettingsKeyPrefix, userID)
	return s.cache.client.Del(ctx, key).Err()
}
