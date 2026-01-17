package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/database"
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

	if len(errs) > 0 {
		s.logger.Warn("Some settings failed to load", "userID", userID, "errors", errs)
	}

	return nil
}

// loadModeToCache loads a single mode's settings into granular cache keys
func (s *SettingsCacheService) loadModeToCache(ctx context.Context, userID, mode string) error {
	// Get full mode config from database
	configJSON, err := s.repo.GetUserModeConfig(ctx, userID, mode)
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
		groupData := s.extractGroupFromConfig(&config, group.Key)
		if groupData == nil {
			continue
		}

		key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group.Key)
		groupJSON, _ := json.Marshal(groupData)

		if err := s.cache.Set(ctx, key, string(groupJSON), 0); err != nil {
			s.logger.Debug("Failed to cache group", "key", key, "error", err)
		}
	}

	return nil
}

// loadGlobalSettings loads all 4 global settings (circuit_breaker, llm_config, capital_allocation, global_trading)
// Story 6.2: Part of 88-key architecture
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
	configJSON, err := s.repo.GetUserModeConfig(ctx, userID, mode)
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
// SEQUENCE PROVIDER (Epic 7: Client Order ID Generation)
// Implements orders.SequenceProvider interface for atomic daily sequence numbers
// ============================================================================

// IncrementDailySequence atomically increments and returns the daily sequence for a user.
// dateKey is in YYYYMMDD format (e.g., "20260115")
// Delegates to underlying CacheService.
func (s *SettingsCacheService) IncrementDailySequence(ctx context.Context, userID, dateKey string) (int64, error) {
	return s.cache.IncrementDailySequence(ctx, userID, dateKey)
}
