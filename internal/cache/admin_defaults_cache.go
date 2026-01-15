package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"binance-trading-bot/internal/autopilot"
)

// AdminDefaultsCacheService provides granular cache access to admin default settings
// Mirrors the user settings cache structure for consistent comparison and reset operations
type AdminDefaultsCacheService struct {
	cache  *CacheService
	logger Logger
}

// NewAdminDefaultsCacheService creates a new admin defaults cache service
func NewAdminDefaultsCacheService(cache *CacheService, logger Logger) *AdminDefaultsCacheService {
	return &AdminDefaultsCacheService{
		cache:  cache,
		logger: logger,
	}
}

// ============================================================================
// LOAD OPERATIONS (On Startup or First Access)
// ============================================================================

// LoadAdminDefaults loads all admin defaults from default-settings.json into granular cache keys
// Creates 89 keys: 80 mode keys (4 modes x 20 groups) + 4 global keys + 4 safety keys + 1 hash key
func (s *AdminDefaultsCacheService) LoadAdminDefaults(ctx context.Context) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	// Load from file
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		return fmt.Errorf("failed to load default-settings.json: %w", err)
	}

	// Calculate and store hash for change detection
	hash := s.calculateHash(defaults)
	if err := s.cache.Set(ctx, "admin:defaults:hash", hash, 0); err != nil {
		s.logger.Warn("Failed to store defaults hash", "error", err)
	}

	// Store mode defaults (80 keys = 4 modes x 20 groups)
	for _, mode := range TradingModes {
		modeConfig, exists := defaults.ModeConfigs[mode]
		if !exists || modeConfig == nil {
			s.logger.Debug("Mode not found in defaults", "mode", mode)
			continue
		}

		for _, group := range SettingGroups {
			groupData := s.extractGroupFromConfig(modeConfig, group.Key)
			if groupData == nil {
				continue
			}

			key := fmt.Sprintf("admin:defaults:mode:%s:%s", mode, group.Key)
			groupJSON, err := json.Marshal(groupData)
			if err != nil {
				s.logger.Debug("Failed to marshal group", "group", group.Key, "error", err)
				continue
			}

			if err := s.cache.Set(ctx, key, string(groupJSON), 0); err != nil {
				s.logger.Debug("Failed to cache default group", "key", key, "error", err)
			}
		}
	}

	// Store global defaults (4 keys)
	s.storeGlobalDefaults(ctx, defaults)

	// Store safety settings (4 keys - one per mode)
	s.storeSafetySettings(ctx, defaults)

	s.logger.Info("Admin defaults loaded to cache", "hash", hash[:8], "keys", 89)
	return nil
}

// storeGlobalDefaults stores the 4 cross-mode default settings
func (s *AdminDefaultsCacheService) storeGlobalDefaults(ctx context.Context, defaults *autopilot.DefaultSettingsFile) {
	// Circuit Breaker
	if defaults.CircuitBreaker.Global.Enabled || defaults.CircuitBreaker.Global.MaxDailyLoss > 0 {
		key := "admin:defaults:global:circuit_breaker"
		data, _ := json.Marshal(defaults.CircuitBreaker.Global)
		s.cache.Set(ctx, key, string(data), 0)
	}

	// LLM Config
	if defaults.LLMConfig.Global.Provider != "" {
		key := "admin:defaults:global:llm_config"
		data, _ := json.Marshal(defaults.LLMConfig.Global)
		s.cache.Set(ctx, key, string(data), 0)
	}

	// Capital Allocation
	key := "admin:defaults:global:capital_allocation"
	data, _ := json.Marshal(defaults.CapitalAllocation)
	s.cache.Set(ctx, key, string(data), 0)

	// Global Trading (risk_level, max_usd_allocation, profit_reinvest_percent, profit_reinvest_risk_level)
	if defaults.GlobalTrading.RiskLevel != "" || defaults.GlobalTrading.MaxUSDAllocation > 0 {
		key := "admin:defaults:global:global_trading"
		data, _ := json.Marshal(defaults.GlobalTrading)
		s.cache.Set(ctx, key, string(data), 0)
	}
}

// storeSafetySettings stores the 4 per-mode safety settings (rate limits, profit monitor, win rate monitor)
func (s *AdminDefaultsCacheService) storeSafetySettings(ctx context.Context, defaults *autopilot.DefaultSettingsFile) {
	if defaults.SafetySettings == nil {
		return
	}

	// Store safety settings for each mode
	safetyModes := map[string]*autopilot.SafetySettingsMode{
		"ultra_fast": defaults.SafetySettings.UltraFast,
		"scalp":      defaults.SafetySettings.Scalp,
		"swing":      defaults.SafetySettings.Swing,
		"position":   defaults.SafetySettings.Position,
	}

	for mode, safetyConfig := range safetyModes {
		if safetyConfig == nil {
			continue
		}
		key := fmt.Sprintf("admin:defaults:safety:%s", mode)
		data, err := json.Marshal(safetyConfig)
		if err != nil {
			s.logger.Debug("Failed to marshal safety settings", "mode", mode, "error", err)
			continue
		}
		s.cache.Set(ctx, key, string(data), 0)
	}
}

// ============================================================================
// READ OPERATIONS (Cache-First with Auto-Populate)
// ============================================================================

// GetAdminDefaultGroup retrieves a single default group from cache
// Auto-populates from file on cache miss
func (s *AdminDefaultsCacheService) GetAdminDefaultGroup(ctx context.Context, mode, group string) ([]byte, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := fmt.Sprintf("admin:defaults:mode:%s:%s", mode, group)

	// Try cache first
	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		return []byte(cached), nil
	}

	// Cache miss - load all defaults (they come as a set from the JSON file)
	if err := s.LoadAdminDefaults(ctx); err != nil {
		return nil, fmt.Errorf("failed to load admin defaults: %w", err)
	}

	// Retry cache read
	cached, err = s.cache.Get(ctx, key)
	if err != nil || cached == "" {
		return nil, ErrSettingNotFound
	}

	return []byte(cached), nil
}

// GetGlobalCircuitBreakerDefault retrieves global circuit breaker defaults
func (s *AdminDefaultsCacheService) GetGlobalCircuitBreakerDefault(ctx context.Context) (*autopilot.GlobalCircuitBreakerConfig, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := "admin:defaults:global:circuit_breaker"

	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var config autopilot.GlobalCircuitBreakerConfig
		if err := json.Unmarshal([]byte(cached), &config); err == nil {
			return &config, nil
		}
	}

	// Cache miss - load defaults
	if err := s.LoadAdminDefaults(ctx); err != nil {
		return nil, err
	}

	cached, _ = s.cache.Get(ctx, key)
	if cached == "" {
		return nil, ErrSettingNotFound
	}

	var config autopilot.GlobalCircuitBreakerConfig
	if err := json.Unmarshal([]byte(cached), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetGlobalLLMConfigDefault retrieves global LLM config defaults
func (s *AdminDefaultsCacheService) GetGlobalLLMConfigDefault(ctx context.Context) (*autopilot.GlobalLLMConfig, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := "admin:defaults:global:llm_config"

	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var config autopilot.GlobalLLMConfig
		if err := json.Unmarshal([]byte(cached), &config); err == nil {
			return &config, nil
		}
	}

	// Cache miss - load defaults
	if err := s.LoadAdminDefaults(ctx); err != nil {
		return nil, err
	}

	cached, _ = s.cache.Get(ctx, key)
	if cached == "" {
		return nil, ErrSettingNotFound
	}

	var config autopilot.GlobalLLMConfig
	if err := json.Unmarshal([]byte(cached), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetCapitalAllocationDefault retrieves capital allocation defaults
func (s *AdminDefaultsCacheService) GetCapitalAllocationDefault(ctx context.Context) (*autopilot.CapitalAllocationDefaults, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := "admin:defaults:global:capital_allocation"

	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var config autopilot.CapitalAllocationDefaults
		if err := json.Unmarshal([]byte(cached), &config); err == nil {
			return &config, nil
		}
	}

	// Cache miss - load defaults
	if err := s.LoadAdminDefaults(ctx); err != nil {
		return nil, err
	}

	cached, _ = s.cache.Get(ctx, key)
	if cached == "" {
		return nil, ErrSettingNotFound
	}

	var config autopilot.CapitalAllocationDefaults
	if err := json.Unmarshal([]byte(cached), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetGlobalTradingDefault retrieves global trading defaults (risk_level, max_usd_allocation, etc.)
func (s *AdminDefaultsCacheService) GetGlobalTradingDefault(ctx context.Context) (*autopilot.GlobalTradingDefaults, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := "admin:defaults:global:global_trading"

	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var config autopilot.GlobalTradingDefaults
		if err := json.Unmarshal([]byte(cached), &config); err == nil {
			return &config, nil
		}
	}

	// Cache miss - load defaults
	if err := s.LoadAdminDefaults(ctx); err != nil {
		return nil, err
	}

	cached, _ = s.cache.Get(ctx, key)
	if cached == "" {
		return nil, ErrSettingNotFound
	}

	var config autopilot.GlobalTradingDefaults
	if err := json.Unmarshal([]byte(cached), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetSafetySettingsDefault retrieves per-mode safety settings defaults (rate limits, profit monitor, win rate monitor)
func (s *AdminDefaultsCacheService) GetSafetySettingsDefault(ctx context.Context, mode string) (*autopilot.SafetySettingsMode, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := fmt.Sprintf("admin:defaults:safety:%s", mode)

	cached, err := s.cache.Get(ctx, key)
	if err == nil && cached != "" {
		var config autopilot.SafetySettingsMode
		if err := json.Unmarshal([]byte(cached), &config); err == nil {
			return &config, nil
		}
	}

	// Cache miss - load defaults
	if err := s.LoadAdminDefaults(ctx); err != nil {
		return nil, err
	}

	cached, _ = s.cache.Get(ctx, key)
	if cached == "" {
		return nil, ErrSettingNotFound
	}

	var config autopilot.SafetySettingsMode
	if err := json.Unmarshal([]byte(cached), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// ============================================================================
// CHANGE DETECTION AND INVALIDATION
// ============================================================================

// CheckAndRefreshIfChanged checks if default-settings.json has changed
// Returns true if cache was refreshed, false if no change
func (s *AdminDefaultsCacheService) CheckAndRefreshIfChanged(ctx context.Context) (bool, error) {
	if !s.cache.IsHealthy() {
		return false, ErrCacheUnavailable
	}

	// Load current file
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		return false, fmt.Errorf("failed to load default-settings.json: %w", err)
	}

	// Calculate current hash
	currentHash := s.calculateHash(defaults)

	// Get cached hash
	cachedHash, err := s.cache.Get(ctx, "admin:defaults:hash")
	if err != nil || cachedHash != currentHash {
		// Hash changed or missing - reload
		s.logger.Info("Admin defaults changed, reloading cache",
			"cachedHash", cachedHash[:8],
			"currentHash", currentHash[:8])

		if err := s.LoadAdminDefaults(ctx); err != nil {
			return false, err
		}
		return true, nil // Refreshed
	}

	return false, nil // No change
}

// InvalidateAdminDefaults removes all admin default keys from cache
// Called when admin updates default-settings.json via API
func (s *AdminDefaultsCacheService) InvalidateAdminDefaults(ctx context.Context) error {
	// Delete all mode defaults using pattern
	if err := s.cache.DeletePattern(ctx, "admin:defaults:mode:*"); err != nil {
		s.logger.Warn("Failed to delete mode defaults", "error", err)
	}

	// Delete global defaults (4 keys)
	for _, setting := range CrossModeSettings {
		key := fmt.Sprintf("admin:defaults:global:%s", setting)
		s.cache.Delete(ctx, key)
	}

	// Delete safety settings (4 keys)
	for _, mode := range SafetySettingsModes {
		key := fmt.Sprintf("admin:defaults:safety:%s", mode)
		s.cache.Delete(ctx, key)
	}

	// Delete hash
	s.cache.Delete(ctx, "admin:defaults:hash")

	s.logger.Info("Admin defaults cache invalidated", "keys_deleted", 89)
	return nil
}

// ============================================================================
// COMPARISON OPERATIONS (For Reset Settings Page)
// ============================================================================

// CompareUserGroupToDefault compares a user's setting group to the admin default
// Returns both values for UI comparison
func (s *AdminDefaultsCacheService) CompareUserGroupToDefault(
	ctx context.Context,
	settingsCache *SettingsCacheService,
	userID, mode, group string,
) (userValue, defaultValue []byte, err error) {
	// Get user's current value from user cache
	userValue, err = settingsCache.GetModeGroup(ctx, userID, mode, group)
	if err != nil && err != ErrSettingNotFound {
		return nil, nil, fmt.Errorf("failed to get user setting: %w", err)
	}

	// Get admin default from admin defaults cache
	defaultValue, err = s.GetAdminDefaultGroup(ctx, mode, group)
	if err != nil && err != ErrSettingNotFound {
		return nil, nil, fmt.Errorf("failed to get default: %w", err)
	}

	return userValue, defaultValue, nil
}

// ============================================================================
// NEW USER CREATION SUPPORT
// ============================================================================

// CopyDefaultsToNewUser copies all 89 default keys to a new user
// Used during user registration to initialize user settings from admin defaults
func (s *AdminDefaultsCacheService) CopyDefaultsToNewUser(
	ctx context.Context,
	settingsCache *SettingsCacheService,
	userID string,
) error {
	// Ensure defaults are loaded
	if _, err := s.CheckAndRefreshIfChanged(ctx); err != nil {
		// If cache check fails, try loading directly
		if err := s.LoadAdminDefaults(ctx); err != nil {
			return fmt.Errorf("failed to load admin defaults: %w", err)
		}
	}

	var copyErrors []error

	// Copy mode defaults (80 keys)
	for _, mode := range TradingModes {
		for _, group := range SettingGroups {
			defaultData, err := s.GetAdminDefaultGroup(ctx, mode, group.Key)
			if err != nil {
				// Skip missing groups - not all groups may have defaults
				continue
			}

			// Write to user cache (uses write-through from Story 6.2)
			if err := settingsCache.UpdateModeGroup(ctx, userID, mode, group.Key, defaultData); err != nil {
				copyErrors = append(copyErrors, fmt.Errorf("%s.%s: %w", mode, group.Key, err))
			}
		}
	}

	// Copy global defaults (4 keys)
	s.copyGlobalDefaultsToUser(ctx, settingsCache, userID)

	// Copy safety settings (4 keys)
	s.copySafetySettingsToUser(ctx, userID)

	if len(copyErrors) > 0 {
		s.logger.Warn("Some defaults failed to copy to new user",
			"userID", userID, "errorCount", len(copyErrors))
	}

	s.logger.Info("Copied defaults to new user", "userID", userID, "keys", 89)
	return nil
}

// copyGlobalDefaultsToUser copies the 4 cross-mode defaults to a user's cache
func (s *AdminDefaultsCacheService) copyGlobalDefaultsToUser(
	ctx context.Context,
	settingsCache *SettingsCacheService,
	userID string,
) {
	// Circuit Breaker
	if data, err := s.cache.Get(ctx, "admin:defaults:global:circuit_breaker"); err == nil && data != "" {
		key := fmt.Sprintf("user:%s:circuit_breaker", userID)
		s.cache.Set(ctx, key, data, 0)
	}

	// LLM Config
	if data, err := s.cache.Get(ctx, "admin:defaults:global:llm_config"); err == nil && data != "" {
		key := fmt.Sprintf("user:%s:llm_config", userID)
		s.cache.Set(ctx, key, data, 0)
	}

	// Capital Allocation
	if data, err := s.cache.Get(ctx, "admin:defaults:global:capital_allocation"); err == nil && data != "" {
		key := fmt.Sprintf("user:%s:capital_allocation", userID)
		s.cache.Set(ctx, key, data, 0)
	}

	// Global Trading
	if data, err := s.cache.Get(ctx, "admin:defaults:global:global_trading"); err == nil && data != "" {
		key := fmt.Sprintf("user:%s:global_trading", userID)
		s.cache.Set(ctx, key, data, 0)
	}
}

// copySafetySettingsToUser copies the 4 per-mode safety settings to a user's cache
func (s *AdminDefaultsCacheService) copySafetySettingsToUser(
	ctx context.Context,
	userID string,
) {
	for _, mode := range SafetySettingsModes {
		adminKey := fmt.Sprintf("admin:defaults:safety:%s", mode)
		if data, err := s.cache.Get(ctx, adminKey); err == nil && data != "" {
			userKey := fmt.Sprintf("user:%s:safety:%s", userID, mode)
			s.cache.Set(ctx, userKey, data, 0)
		}
	}
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// calculateHash calculates MD5 hash of defaults for change detection
func (s *AdminDefaultsCacheService) calculateHash(defaults *autopilot.DefaultSettingsFile) string {
	data, _ := json.Marshal(defaults)
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// extractGroupFromConfig extracts a specific group's data from ModeFullConfig
// Same logic as SettingsCacheService for consistency
func (s *AdminDefaultsCacheService) extractGroupFromConfig(config *autopilot.ModeFullConfig, groupKey string) interface{} {
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

// IsHealthy returns whether the underlying cache is healthy
func (s *AdminDefaultsCacheService) IsHealthy() bool {
	return s.cache.IsHealthy()
}

// ============================================================================
// FULL DEFAULTS LOAD (For Settings Comparison)
// ============================================================================

// GetAllAdminDefaults loads all admin defaults from cache into a DefaultSettingsFile structure
// Story 6.5: Cache-First Read Pattern for settings comparison
// Returns HTTP 503-compatible error if cache is unavailable
func (s *AdminDefaultsCacheService) GetAllAdminDefaults(ctx context.Context) (*autopilot.DefaultSettingsFile, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	// Ensure defaults are loaded
	if _, err := s.CheckAndRefreshIfChanged(ctx); err != nil {
		// Try loading directly
		if loadErr := s.LoadAdminDefaults(ctx); loadErr != nil {
			return nil, fmt.Errorf("failed to load admin defaults: %w", loadErr)
		}
	}

	settings := &autopilot.DefaultSettingsFile{
		ModeConfigs: make(map[string]*autopilot.ModeFullConfig),
	}

	// Load all mode configs from cache
	for _, mode := range TradingModes {
		modeConfig, err := s.GetModeFullConfig(ctx, mode)
		if err != nil {
			if err == ErrCacheUnavailable {
				return nil, err
			}
			s.logger.Debug("Mode config not found in admin defaults cache", "mode", mode)
			continue
		}
		settings.ModeConfigs[mode] = modeConfig
	}

	// Load global settings
	// Global Trading
	if globalTrading, err := s.GetGlobalTradingDefault(ctx); err == nil && globalTrading != nil {
		settings.GlobalTrading = *globalTrading
	}

	// Circuit Breaker
	if cb, err := s.GetGlobalCircuitBreakerDefault(ctx); err == nil && cb != nil {
		settings.CircuitBreaker = autopilot.CircuitBreakerDefaults{
			Global: *cb,
		}
	}

	// LLM Config
	if llm, err := s.GetGlobalLLMConfigDefault(ctx); err == nil && llm != nil {
		settings.LLMConfig = autopilot.LLMConfigDefaults{
			Global: *llm,
		}
	}

	// Capital Allocation
	if cap, err := s.GetCapitalAllocationDefault(ctx); err == nil && cap != nil {
		settings.CapitalAllocation = *cap
	}

	// Load risk index from file (not cached separately)
	if defaults, err := autopilot.LoadDefaultSettings(); err == nil {
		settings.SettingsRiskIndex = defaults.SettingsRiskIndex
	}

	return settings, nil
}

// GetModeFullConfig assembles a full ModeFullConfig from cached groups
// Story 6.5: Public method for use by handlers
func (s *AdminDefaultsCacheService) GetModeFullConfig(ctx context.Context, mode string) (*autopilot.ModeFullConfig, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	config := &autopilot.ModeFullConfig{ModeName: mode}

	// Load each group and merge into config
	for _, group := range SettingGroups {
		data, err := s.GetAdminDefaultGroup(ctx, mode, group.Key)
		if err != nil {
			continue // Skip missing groups
		}

		s.mergeGroupIntoConfig(config, group.Key, data)
	}

	return config, nil
}

// mergeGroupIntoConfig merges a group's JSON data into ModeFullConfig
// Same logic as SettingsCacheService for consistency
func (s *AdminDefaultsCacheService) mergeGroupIntoConfig(config *autopilot.ModeFullConfig, groupKey string, data []byte) error {
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
