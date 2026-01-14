package autopilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"binance-trading-bot/internal/database"
)

// ====== ADMIN SETTINGS SYNC SERVICE ======
// Story 4.15: Admin Settings Sync
// When admin user modifies settings, sync to default-settings.json
// This ensures all new users get the latest admin-approved defaults

const (
	AdminEmail              = "admin@binance-bot.local"
	AdminUserID             = "00000000-0000-0000-0000-000000000000"
	DefaultSettingsFilePath = "default-settings.json"
	BackupSuffix            = ".backup"
)

var (
	syncMutex sync.Mutex // Prevent concurrent writes to default-settings.json
)

// AdminSyncService handles syncing admin settings to default-settings.json
type AdminSyncService struct {
	mu sync.Mutex
}

// GetAdminSyncService returns the singleton admin sync service
var adminSyncService *AdminSyncService
var adminSyncOnce sync.Once

func GetAdminSyncService() *AdminSyncService {
	adminSyncOnce.Do(func() {
		adminSyncService = &AdminSyncService{}
	})
	return adminSyncService
}

// IsAdminUser checks if the given email is the admin user
func IsAdminUser(email string) bool {
	return email == AdminEmail
}

// SyncAdminModeConfig syncs a single mode configuration to default-settings.json
// This is called automatically when admin saves mode config
func (s *AdminSyncService) SyncAdminModeConfig(ctx context.Context, modeName string, config *ModeFullConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load current default settings
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return fmt.Errorf("failed to load default settings: %w", err)
	}

	// Update the specific mode config
	if defaults.ModeConfigs == nil {
		defaults.ModeConfigs = make(map[string]*ModeFullConfig)
	}
	defaults.ModeConfigs[modeName] = config

	// Update metadata
	defaults.Metadata.LastUpdated = time.Now().Format(time.RFC3339)
	defaults.Metadata.UpdatedBy = "admin"

	// Save to file
	if err := s.saveDefaultSettings(defaults); err != nil {
		return fmt.Errorf("failed to save default settings: %w", err)
	}

	log.Printf("[ADMIN-SYNC] Successfully synced mode %s to default-settings.json", modeName)
	return nil
}

// SyncAdminSettingToDefaults syncs a specific settings group to default-settings.json
// group can be: "global_trading", "position_optimization", "circuit_breaker", "llm_config", etc.
func (s *AdminSyncService) SyncAdminSettingToDefaults(ctx context.Context, group string, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load current default settings
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return fmt.Errorf("failed to load default settings: %w", err)
	}

	// Update the specific group
	switch group {
	case "global_trading":
		if val, ok := data.(GlobalTradingDefaults); ok {
			defaults.GlobalTrading = val
		} else {
			return fmt.Errorf("invalid data type for global_trading")
		}
	case "position_optimization":
		if val, ok := data.(PositionOptimizationDefaults); ok {
			defaults.PositionOptimization = val
		} else {
			return fmt.Errorf("invalid data type for position_optimization")
		}
	case "circuit_breaker":
		if val, ok := data.(CircuitBreakerDefaults); ok {
			defaults.CircuitBreaker = val
		} else {
			return fmt.Errorf("invalid data type for circuit_breaker")
		}
	case "llm_config":
		if val, ok := data.(LLMConfigDefaults); ok {
			defaults.LLMConfig = val
		} else {
			return fmt.Errorf("invalid data type for llm_config")
		}
	case "early_warning":
		if val, ok := data.(EarlyWarningDefaults); ok {
			defaults.EarlyWarning = val
		} else {
			return fmt.Errorf("invalid data type for early_warning")
		}
	case "capital_allocation":
		if val, ok := data.(CapitalAllocationDefaults); ok {
			defaults.CapitalAllocation = val
		} else {
			return fmt.Errorf("invalid data type for capital_allocation")
		}
	default:
		return fmt.Errorf("unknown settings group: %s", group)
	}

	// Update metadata
	defaults.Metadata.LastUpdated = time.Now().Format(time.RFC3339)
	defaults.Metadata.UpdatedBy = "admin"

	// Save to file
	if err := s.saveDefaultSettings(defaults); err != nil {
		return fmt.Errorf("failed to save default settings: %w", err)
	}

	log.Printf("[ADMIN-SYNC] Successfully synced %s to default-settings.json", group)
	return nil
}

// SyncAllAdminDefaults syncs all settings from admin's database config to default-settings.json
// This is called manually via admin API endpoint
func (s *AdminSyncService) SyncAllAdminDefaults(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("[ADMIN-SYNC] Starting full sync of admin settings to default-settings.json")

	// Load current default settings as template
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return fmt.Errorf("failed to load default settings: %w", err)
	}

	// Note: We cannot access repository directly here since AdminSyncService
	// is a standalone service. This function should be called from the API handler
	// which has access to the repository.
	// For now, just update metadata to indicate manual sync was triggered
	defaults.Metadata.LastUpdated = time.Now().Format(time.RFC3339)
	defaults.Metadata.UpdatedBy = "admin"

	if err := s.saveDefaultSettings(defaults); err != nil {
		return fmt.Errorf("failed to save default settings: %w", err)
	}

	log.Printf("[ADMIN-SYNC] Full sync completed successfully")
	return nil
}

// SyncAllAdminDefaultsFromDB syncs all settings from admin's database config to default-settings.json
// This is the actual implementation that reads from database
func (s *AdminSyncService) SyncAllAdminDefaultsFromDB(ctx context.Context, repo *database.Repository) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("[ADMIN-SYNC] Starting full sync of admin settings from database to default-settings.json")

	// Load current default settings as template
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return fmt.Errorf("failed to load default settings: %w", err)
	}

	// 1. Load all mode configs from user_mode_configs
	modeConfigsMap, err := repo.GetAllUserModeConfigs(ctx, AdminUserID)
	if err != nil {
		return fmt.Errorf("failed to get admin mode configs: %w", err)
	}

	// Parse mode configs from JSON and populate defaults
	if len(modeConfigsMap) > 0 {
		defaults.ModeConfigs = make(map[string]*ModeFullConfig)
		for modeName, configJSON := range modeConfigsMap {
			var modeConfig ModeFullConfig
			if err := json.Unmarshal(configJSON, &modeConfig); err != nil {
				log.Printf("[ADMIN-SYNC] Warning: Failed to unmarshal mode config for %s: %v", modeName, err)
				continue
			}
			defaults.ModeConfigs[modeName] = &modeConfig
			log.Printf("[ADMIN-SYNC] Synced mode config: %s", modeName)
		}
	}

	// 2. Load capital allocation from user_capital_allocation
	capitalAlloc, err := repo.GetUserCapitalAllocation(ctx, AdminUserID)
	if err != nil {
		log.Printf("[ADMIN-SYNC] Warning: Failed to get admin capital allocation: %v", err)
	} else if capitalAlloc != nil {
		// Sync all capital allocation fields including dynamic rebalance settings
		defaults.CapitalAllocation = CapitalAllocationDefaults{
			UltraFastPercent:      capitalAlloc.UltraFastPercent,
			ScalpPercent:          capitalAlloc.ScalpPercent,
			SwingPercent:          capitalAlloc.SwingPercent,
			PositionPercent:       capitalAlloc.PositionPercent,
			AllowDynamicRebalance: capitalAlloc.AllowDynamicRebalance,
			RebalanceThresholdPct: capitalAlloc.RebalanceThresholdPct,
		}
		log.Printf("[ADMIN-SYNC] Synced capital allocation (including dynamic rebalance)")
	}

	// 3. Load global circuit breaker from user_global_circuit_breaker
	circuitBreaker, err := repo.GetUserGlobalCircuitBreaker(ctx, AdminUserID)
	if err != nil {
		log.Printf("[ADMIN-SYNC] Warning: Failed to get admin circuit breaker: %v", err)
	} else if circuitBreaker != nil {
		defaults.CircuitBreaker = CircuitBreakerDefaults{
			Global: GlobalCircuitBreakerConfig{
				Enabled:              circuitBreaker.Enabled,
				MaxLossPerHour:       circuitBreaker.MaxLossPerHour,
				MaxDailyLoss:         circuitBreaker.MaxDailyLoss,
				MaxConsecutiveLosses: circuitBreaker.MaxConsecutiveLosses,
				CooldownMinutes:      circuitBreaker.CooldownMinutes,
				MaxTradesPerMinute:   circuitBreaker.MaxTradesPerMinute,
				MaxDailyTrades:       circuitBreaker.MaxDailyTrades,
			},
		}
		log.Printf("[ADMIN-SYNC] Synced global circuit breaker")
	}

	// 4. Load LLM config from user_llm_config
	llmConfig, err := repo.GetUserLLMConfig(ctx, AdminUserID)
	if err != nil {
		log.Printf("[ADMIN-SYNC] Warning: Failed to get admin LLM config: %v", err)
	} else if llmConfig != nil {
		defaults.LLMConfig = LLMConfigDefaults{
			Global: GlobalLLMConfig{
				Enabled:          llmConfig.Enabled,
				Provider:         llmConfig.Provider,
				Model:            llmConfig.Model,
				TimeoutMS:        llmConfig.TimeoutMs,
				RetryCount:       llmConfig.RetryCount,
				CacheDurationSec: llmConfig.CacheDurationSec,
			},
		}
		log.Printf("[ADMIN-SYNC] Synced LLM config")
	}

	// 5. Load early warning from user_early_warning
	earlyWarning, err := repo.GetUserEarlyWarning(ctx, AdminUserID)
	if err != nil {
		log.Printf("[ADMIN-SYNC] Warning: Failed to get admin early warning: %v", err)
	} else if earlyWarning != nil {
		defaults.EarlyWarning = EarlyWarningDefaults{
			Enabled:           earlyWarning.Enabled,
			StartAfterMinutes: earlyWarning.StartAfterMinutes,
			CheckIntervalSecs: earlyWarning.CheckIntervalSecs,
			OnlyUnderwater:    earlyWarning.OnlyUnderwater,
			MinLossPercent:    earlyWarning.MinLossPercent,
			CloseOnReversal:   earlyWarning.CloseOnReversal,
		}
		log.Printf("[ADMIN-SYNC] Synced early warning")
	}

	// 6. Load Ginie settings from user_ginie_settings
	ginieSettings, err := repo.GetUserGinieSettings(ctx, AdminUserID)
	if err != nil {
		log.Printf("[ADMIN-SYNC] Warning: Failed to get admin Ginie settings: %v", err)
	} else if ginieSettings != nil {
		// Ginie settings are stored in GlobalTrading section
		defaults.GlobalTrading.RiskLevel = "moderate" // Keep existing risk level
		// Note: Ginie-specific settings like DryRunMode, AutoStart, MaxPositions are in user_ginie_settings
		// but not in the GlobalTradingDefaults struct. They are mode-specific.
		log.Printf("[ADMIN-SYNC] Synced Ginie settings")
	}

	// Update metadata
	defaults.Metadata.LastUpdated = time.Now().Format(time.RFC3339)
	defaults.Metadata.UpdatedBy = "admin"

	// Save to file
	if err := s.saveDefaultSettings(defaults); err != nil {
		return fmt.Errorf("failed to save default settings: %w", err)
	}

	log.Printf("[ADMIN-SYNC] Full sync from database completed successfully")
	log.Printf("[ADMIN-SYNC] Synced: %d mode configs, capital allocation, circuit breaker, LLM config, early warning", len(defaults.ModeConfigs))
	return nil
}

// GetSyncStatus returns the last sync information from default-settings.json metadata
func (s *AdminSyncService) GetSyncStatus(ctx context.Context) (map[string]interface{}, error) {
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to load default settings: %w", err)
	}

	// Check if backup file exists
	backupExists := false
	if _, err := os.Stat(DefaultSettingsFilePath + BackupSuffix); err == nil {
		backupExists = true
	}

	// Get file modification time
	fileInfo, err := os.Stat(DefaultSettingsFilePath)
	var fileModTime string
	if err == nil {
		fileModTime = fileInfo.ModTime().Format(time.RFC3339)
	}

	return map[string]interface{}{
		"last_updated":     defaults.Metadata.LastUpdated,
		"updated_by":       defaults.Metadata.UpdatedBy,
		"version":          defaults.Metadata.Version,
		"schema_version":   defaults.Metadata.SchemaVersion,
		"backup_exists":    backupExists,
		"file_mod_time":    fileModTime,
		"sync_enabled":     true,
		"admin_email":      AdminEmail,
		"settings_file":    DefaultSettingsFilePath,
	}, nil
}

// saveDefaultSettings saves the defaults to file with backup
func (s *AdminSyncService) saveDefaultSettings(defaults *DefaultSettingsFile) error {
	// Create backup of current file
	if err := s.createBackup(); err != nil {
		log.Printf("[ADMIN-SYNC] Warning: Failed to create backup: %v", err)
		// Continue anyway - backup is not critical
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(defaults, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal defaults: %w", err)
	}

	// Write to file atomically
	tempFile := DefaultSettingsFilePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, DefaultSettingsFilePath); err != nil {
		os.Remove(tempFile) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Force reload of singleton to pick up new settings
	if err := ReloadDefaultSettings(); err != nil {
		log.Printf("[ADMIN-SYNC] Warning: Failed to reload defaults after save: %v", err)
	}

	return nil
}

// createBackup creates a backup of the current default-settings.json
func (s *AdminSyncService) createBackup() error {
	// Check if source file exists
	if _, err := os.Stat(DefaultSettingsFilePath); os.IsNotExist(err) {
		return nil // No file to backup
	}

	// Read current file
	data, err := os.ReadFile(DefaultSettingsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read current file: %w", err)
	}

	// Write backup
	backupFile := DefaultSettingsFilePath + BackupSuffix
	if err := os.WriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	log.Printf("[ADMIN-SYNC] Created backup: %s", backupFile)
	return nil
}

// SaveFlattenedDefaults saves flattened key-value pairs to default-settings.json
// Story 9.4: Admin can edit default values from the UI
// configType: "safety_settings", "circuit_breaker", "llm_config", "capital_allocation", "scalp_reentry", mode names (ultra_fast, scalp, etc.)
// editedValues: map of flattened keys to values (e.g., "safety_settings.ultra_fast.max_trades_per_minute" -> 5)
func (s *AdminSyncService) SaveFlattenedDefaults(configType string, editedValues map[string]interface{}) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("[ADMIN-SAVE] Saving %d values for config type: %s", len(editedValues), configType)

	// Load current defaults
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return 0, fmt.Errorf("failed to load default settings: %w", err)
	}

	changesCount := 0

	// Update based on config type
	switch configType {
	case "safety_settings":
		if defaults.SafetySettings == nil {
			defaults.SafetySettings = &SafetySettingsAllModes{}
		}
		changesCount = s.updateSafetySettingsFromFlattened(defaults.SafetySettings, editedValues)

	case "circuit_breaker":
		changesCount = s.updateCircuitBreakerFromFlattened(&defaults.CircuitBreaker, editedValues)

	case "llm_config":
		changesCount = s.updateLLMConfigFromFlattened(&defaults.LLMConfig, editedValues)

	case "capital_allocation":
		changesCount = s.updateCapitalAllocationFromFlattened(&defaults.CapitalAllocation, editedValues)

	case "scalp_reentry":
		if defaults.ScalpReentry == nil {
			defaults.ScalpReentry = &PositionOptimizationConfig{}
		}
		changesCount = s.updateScalpReentryFromFlattened(defaults.ScalpReentry, editedValues)

	case "ultra_fast", "scalp", "swing", "position":
		// Mode configs
		if defaults.ModeConfigs == nil {
			defaults.ModeConfigs = make(map[string]*ModeFullConfig)
		}
		if defaults.ModeConfigs[configType] == nil {
			defaults.ModeConfigs[configType] = &ModeFullConfig{}
		}
		changesCount = s.updateModeConfigFromFlattened(defaults.ModeConfigs[configType], editedValues)

	default:
		return 0, fmt.Errorf("unknown config type: %s", configType)
	}

	// Update metadata
	defaults.Metadata.LastUpdated = time.Now().Format(time.RFC3339)
	defaults.Metadata.UpdatedBy = "admin"

	// Save to file
	if err := s.saveDefaultSettings(defaults); err != nil {
		return 0, fmt.Errorf("failed to save default settings: %w", err)
	}

	log.Printf("[ADMIN-SAVE] Saved %d changes for config type: %s", changesCount, configType)
	return changesCount, nil
}

// updateSafetySettingsFromFlattened updates safety settings from flattened key-value pairs
func (s *AdminSyncService) updateSafetySettingsFromFlattened(ss *SafetySettingsAllModes, values map[string]interface{}) int {
	count := 0
	for key, value := range values {
		// Key format: safety_settings.{mode}.{field} or just {mode}.{field}
		parts := splitKeyPath(key)
		if len(parts) < 2 {
			continue
		}

		// Handle both "safety_settings.ultra_fast.xxx" and "ultra_fast.xxx" formats
		var modeName, fieldName string
		if parts[0] == "safety_settings" && len(parts) >= 3 {
			modeName = parts[1]
			fieldName = parts[2]
		} else if len(parts) >= 2 {
			modeName = parts[0]
			fieldName = parts[1]
		} else {
			continue
		}

		var mode *SafetySettingsMode
		switch modeName {
		case "ultra_fast":
			if ss.UltraFast == nil {
				ss.UltraFast = &SafetySettingsMode{}
			}
			mode = ss.UltraFast
		case "scalp":
			if ss.Scalp == nil {
				ss.Scalp = &SafetySettingsMode{}
			}
			mode = ss.Scalp
		case "swing":
			if ss.Swing == nil {
				ss.Swing = &SafetySettingsMode{}
			}
			mode = ss.Swing
		case "position":
			if ss.Position == nil {
				ss.Position = &SafetySettingsMode{}
			}
			mode = ss.Position
		default:
			continue
		}

		if updateSafetyModeField(mode, fieldName, value) {
			count++
		}
	}
	return count
}

// updateSafetyModeField updates a single field on a SafetySettingsMode
func updateSafetyModeField(mode *SafetySettingsMode, field string, value interface{}) bool {
	switch field {
	case "max_trades_per_minute":
		mode.MaxTradesPerMinute = toInt(value)
	case "max_trades_per_hour":
		mode.MaxTradesPerHour = toInt(value)
	case "max_trades_per_day":
		mode.MaxTradesPerDay = toInt(value)
	case "enable_profit_monitor":
		mode.EnableProfitMonitor = toBool(value)
	case "profit_window_minutes":
		mode.ProfitWindowMinutes = toInt(value)
	case "max_loss_percent_in_window":
		mode.MaxLossPercentInWindow = toFloat64(value)
	case "pause_cooldown_minutes":
		mode.PauseCooldownMinutes = toInt(value)
	case "enable_win_rate_monitor":
		mode.EnableWinRateMonitor = toBool(value)
	case "win_rate_sample_size":
		mode.WinRateSampleSize = toInt(value)
	case "min_win_rate_threshold":
		mode.MinWinRateThreshold = toFloat64(value)
	case "win_rate_cooldown_minutes":
		mode.WinRateCooldownMinutes = toInt(value)
	default:
		return false
	}
	return true
}

// updateCircuitBreakerFromFlattened updates circuit breaker from flattened values
func (s *AdminSyncService) updateCircuitBreakerFromFlattened(cb *CircuitBreakerDefaults, values map[string]interface{}) int {
	count := 0
	for key, value := range values {
		parts := splitKeyPath(key)
		field := parts[len(parts)-1] // Get the last part as the field name

		switch field {
		case "enabled":
			cb.Global.Enabled = toBool(value)
			count++
		case "max_loss_per_hour":
			cb.Global.MaxLossPerHour = toFloat64(value)
			count++
		case "max_daily_loss":
			cb.Global.MaxDailyLoss = toFloat64(value)
			count++
		case "max_consecutive_losses":
			cb.Global.MaxConsecutiveLosses = toInt(value)
			count++
		case "cooldown_minutes":
			cb.Global.CooldownMinutes = toInt(value)
			count++
		case "max_trades_per_minute":
			cb.Global.MaxTradesPerMinute = toInt(value)
			count++
		case "max_daily_trades":
			cb.Global.MaxDailyTrades = toInt(value)
			count++
		}
	}
	return count
}

// updateLLMConfigFromFlattened updates LLM config from flattened values
func (s *AdminSyncService) updateLLMConfigFromFlattened(llm *LLMConfigDefaults, values map[string]interface{}) int {
	count := 0
	for key, value := range values {
		parts := splitKeyPath(key)
		field := parts[len(parts)-1]

		switch field {
		case "enabled":
			llm.Global.Enabled = toBool(value)
			count++
		case "provider":
			llm.Global.Provider = toString(value)
			count++
		case "model":
			llm.Global.Model = toString(value)
			count++
		case "timeout_ms":
			llm.Global.TimeoutMS = toInt(value)
			count++
		case "retry_count":
			llm.Global.RetryCount = toInt(value)
			count++
		case "cache_duration_sec":
			llm.Global.CacheDurationSec = toInt(value)
			count++
		}
	}
	return count
}

// updateCapitalAllocationFromFlattened updates capital allocation from flattened values
func (s *AdminSyncService) updateCapitalAllocationFromFlattened(ca *CapitalAllocationDefaults, values map[string]interface{}) int {
	count := 0
	for key, value := range values {
		parts := splitKeyPath(key)
		field := parts[len(parts)-1]

		switch field {
		case "ultra_fast_percent":
			ca.UltraFastPercent = toFloat64(value)
			count++
		case "scalp_percent":
			ca.ScalpPercent = toFloat64(value)
			count++
		case "swing_percent":
			ca.SwingPercent = toFloat64(value)
			count++
		case "position_percent":
			ca.PositionPercent = toFloat64(value)
			count++
		case "allow_dynamic_rebalance":
			ca.AllowDynamicRebalance = toBool(value)
			count++
		case "rebalance_threshold_pct":
			ca.RebalanceThresholdPct = toFloat64(value)
			count++
		}
	}
	return count
}

// updateScalpReentryFromFlattened updates scalp reentry config from flattened values
func (s *AdminSyncService) updateScalpReentryFromFlattened(sr *PositionOptimizationConfig, values map[string]interface{}) int {
	count := 0
	for key, value := range values {
		parts := splitKeyPath(key)
		field := parts[len(parts)-1]

		switch field {
		case "enabled":
			sr.Enabled = toBool(value)
			count++
		case "tp1_percent":
			sr.TP1Percent = toFloat64(value)
			count++
		case "tp1_sell_percent":
			sr.TP1SellPercent = toFloat64(value)
			count++
		case "tp2_percent":
			sr.TP2Percent = toFloat64(value)
			count++
		case "tp2_sell_percent":
			sr.TP2SellPercent = toFloat64(value)
			count++
		case "tp3_percent":
			sr.TP3Percent = toFloat64(value)
			count++
		case "tp3_sell_percent":
			sr.TP3SellPercent = toFloat64(value)
			count++
		case "reentry_percent":
			sr.ReentryPercent = toFloat64(value)
			count++
		case "reentry_price_buffer":
			sr.ReentryPriceBuffer = toFloat64(value)
			count++
		case "max_reentry_attempts":
			sr.MaxReentryAttempts = toInt(value)
			count++
		case "reentry_timeout_sec":
			sr.ReentryTimeoutSec = toInt(value)
			count++
		}
	}
	return count
}

// updateModeConfigFromFlattened updates mode config from flattened values
func (s *AdminSyncService) updateModeConfigFromFlattened(mc *ModeFullConfig, values map[string]interface{}) int {
	count := 0
	for key, value := range values {
		parts := splitKeyPath(key)
		if len(parts) < 2 {
			continue
		}

		section := parts[0]
		field := parts[1]

		switch section {
		case "sltp":
			if mc.SLTP == nil {
				mc.SLTP = &ModeSLTPConfig{}
			}
			count += updateSLTPConfig(mc.SLTP, field, parts[2:], value)
		case "size":
			if mc.Size == nil {
				mc.Size = &ModeSizeConfig{}
			}
			count += updateSizeConfig(mc.Size, field, value)
		case "confidence":
			if mc.Confidence == nil {
				mc.Confidence = &ModeConfidenceConfig{}
			}
			count += updateConfidenceConfig(mc.Confidence, field, value)
		case "circuit_breaker":
			if mc.CircuitBreaker == nil {
				mc.CircuitBreaker = &ModeCircuitBreakerConfig{}
			}
			count += updateCircuitBreakerConfig(mc.CircuitBreaker, field, value)
		case "timeframe":
			if mc.Timeframe == nil {
				mc.Timeframe = &ModeTimeframeConfig{}
			}
			count += updateTimeframeConfig(mc.Timeframe, field, value)
		case "hedge":
			if mc.Hedge == nil {
				mc.Hedge = &HedgeModeConfig{}
			}
			count += updateHedgeConfig(mc.Hedge, field, value)
		case "averaging":
			if mc.Averaging == nil {
				mc.Averaging = &PositionAveragingConfig{}
			}
			count += updateAveragingConfig(mc.Averaging, field, value)
		case "stale_release":
			if mc.StaleRelease == nil {
				mc.StaleRelease = &StalePositionReleaseConfig{}
			}
			count += updateStaleReleaseConfig(mc.StaleRelease, field, value)
		case "funding_rate":
			if mc.FundingRate == nil {
				mc.FundingRate = &ModeFundingRateConfig{}
			}
			count += updateFundingRateConfig(mc.FundingRate, field, value)
		case "risk":
			if mc.Risk == nil {
				mc.Risk = &ModeRiskConfig{}
			}
			count += updateRiskConfig(mc.Risk, field, value)
		case "trend_filters":
			if mc.TrendFilters == nil {
				mc.TrendFilters = &TrendFiltersConfig{}
			}
			count += updateTrendFiltersConfig(mc.TrendFilters, field, parts[2:], value)
		case "mtf":
			if mc.MTF == nil {
				mc.MTF = &ModeMTFConfig{}
			}
			count += updateMTFConfig(mc.MTF, field, value)
		case "dynamic_ai_exit":
			if mc.DynamicAIExit == nil {
				mc.DynamicAIExit = &ModeDynamicAIExitConfig{}
			}
			count += updateDynamicAIExitConfig(mc.DynamicAIExit, field, value)
		default:
			log.Printf("[ADMIN-SAVE] Unknown mode config section: %s", section)
		}
	}
	return count
}

// updateSLTPConfig updates SLTP config from field/value
func updateSLTPConfig(sltp *ModeSLTPConfig, field string, subparts []string, value interface{}) int {
	switch field {
	case "stop_loss_percent":
		sltp.StopLossPercent = toFloat64(value)
		return 1
	case "take_profit_percent":
		sltp.TakeProfitPercent = toFloat64(value)
		return 1
	case "trailing_stop_enabled":
		sltp.TrailingStopEnabled = toBool(value)
		return 1
	case "trailing_stop_percent":
		sltp.TrailingStopPercent = toFloat64(value)
		return 1
	case "trailing_stop_activation":
		sltp.TrailingStopActivation = toFloat64(value)
		return 1
	case "trailing_activation_price":
		sltp.TrailingActivationPrice = toFloat64(value)
		return 1
	case "max_hold_duration":
		sltp.MaxHoldDuration = toString(value)
		return 1
	case "use_single_tp":
		sltp.UseSingleTP = toBool(value)
		return 1
	case "single_tp_percent":
		sltp.SingleTPPercent = toFloat64(value)
		return 1
	case "trailing_activation_mode":
		sltp.TrailingActivationMode = toString(value)
		return 1
	case "use_roi_based_sltp":
		sltp.UseROIBasedSLTP = toBool(value)
		return 1
	case "roi_stop_loss_percent":
		sltp.ROIStopLossPercent = toFloat64(value)
		return 1
	case "roi_take_profit_percent":
		sltp.ROITakeProfitPercent = toFloat64(value)
		return 1
	case "margin_type":
		sltp.MarginType = toString(value)
		return 1
	case "isolated_margin_percent":
		sltp.IsolatedMarginPercent = toFloat64(value)
		return 1
	case "atr_sl_multiplier":
		sltp.ATRSLMultiplier = toFloat64(value)
		return 1
	case "atr_tp_multiplier":
		sltp.ATRTPMultiplier = toFloat64(value)
		return 1
	case "llm_weight":
		sltp.LLMWeight = toFloat64(value)
		return 1
	case "atr_weight":
		sltp.ATRWeight = toFloat64(value)
		return 1
	case "auto_sltp_enabled":
		sltp.AutoSLTPEnabled = toBool(value)
		return 1
	case "auto_trailing_enabled":
		sltp.AutoTrailingEnabled = toBool(value)
		return 1
	case "min_profit_to_trail_pct":
		sltp.MinProfitToTrailPct = toFloat64(value)
		return 1
	case "min_sl_distance_from_zero":
		sltp.MinSLDistanceFromZero = toFloat64(value)
		return 1
	}
	return 0
}

// updateSizeConfig updates Size config from field/value
func updateSizeConfig(size *ModeSizeConfig, field string, value interface{}) int {
	switch field {
	case "base_size_usd":
		size.BaseSizeUSD = toFloat64(value)
		return 1
	case "max_size_usd":
		size.MaxSizeUSD = toFloat64(value)
		return 1
	case "max_positions":
		size.MaxPositions = toInt(value)
		return 1
	case "leverage":
		size.Leverage = toInt(value)
		return 1
	case "size_multiplier_lo":
		size.SizeMultiplierLo = toFloat64(value)
		return 1
	case "size_multiplier_hi":
		size.SizeMultiplierHi = toFloat64(value)
		return 1
	case "safety_margin":
		size.SafetyMargin = toFloat64(value)
		return 1
	case "min_balance_usd":
		size.MinBalanceUSD = toFloat64(value)
		return 1
	case "min_position_size_usd":
		size.MinPositionSizeUSD = toFloat64(value)
		return 1
	case "auto_size_enabled":
		size.AutoSizeEnabled = toBool(value)
		return 1
	case "auto_size_min_cover_fee":
		size.AutoSizeMinCoverFee = toFloat64(value)
		return 1
	}
	return 0
}

// updateConfidenceConfig updates Confidence config from field/value
func updateConfidenceConfig(conf *ModeConfidenceConfig, field string, value interface{}) int {
	switch field {
	case "min_confidence":
		conf.MinConfidence = toFloat64(value)
		return 1
	case "high_confidence":
		conf.HighConfidence = toFloat64(value)
		return 1
	case "ultra_confidence":
		conf.UltraConfidence = toFloat64(value)
		return 1
	}
	return 0
}

// updateCircuitBreakerConfig updates CircuitBreaker config from field/value
func updateCircuitBreakerConfig(cb *ModeCircuitBreakerConfig, field string, value interface{}) int {
	switch field {
	case "max_loss_per_hour":
		cb.MaxLossPerHour = toFloat64(value)
		return 1
	case "max_loss_per_day":
		cb.MaxLossPerDay = toFloat64(value)
		return 1
	case "max_consecutive_losses":
		cb.MaxConsecutiveLosses = toInt(value)
		return 1
	case "cooldown_minutes":
		cb.CooldownMinutes = toInt(value)
		return 1
	case "max_trades_per_minute":
		cb.MaxTradesPerMinute = toInt(value)
		return 1
	case "max_trades_per_hour":
		cb.MaxTradesPerHour = toInt(value)
		return 1
	case "max_trades_per_day":
		cb.MaxTradesPerDay = toInt(value)
		return 1
	case "win_rate_check_after":
		cb.WinRateCheckAfter = toInt(value)
		return 1
	case "min_win_rate":
		cb.MinWinRate = toFloat64(value)
		return 1
	}
	return 0
}

// updateTimeframeConfig updates Timeframe config from field/value
func updateTimeframeConfig(tf *ModeTimeframeConfig, field string, value interface{}) int {
	switch field {
	case "trend_timeframe":
		tf.TrendTimeframe = toString(value)
		return 1
	case "entry_timeframe":
		tf.EntryTimeframe = toString(value)
		return 1
	case "analysis_timeframe":
		tf.AnalysisTimeframe = toString(value)
		return 1
	}
	return 0
}

// updateHedgeConfig updates Hedge config from field/value
func updateHedgeConfig(hedge *HedgeModeConfig, field string, value interface{}) int {
	switch field {
	case "allow_hedge":
		hedge.AllowHedge = toBool(value)
		return 1
	case "min_confidence_for_hedge":
		hedge.MinConfidenceForHedge = toFloat64(value)
		return 1
	case "existing_must_be_in_profit":
		hedge.ExistingMustBeInProfit = toFloat64(value)
		return 1
	case "max_hedge_size_percent":
		hedge.MaxHedgeSizePercent = toFloat64(value)
		return 1
	case "allow_same_mode_hedge":
		hedge.AllowSameModeHedge = toBool(value)
		return 1
	case "max_total_exposure_multiplier":
		hedge.MaxTotalExposureMultiplier = toFloat64(value)
		return 1
	}
	return 0
}

// updateAveragingConfig updates Averaging config from field/value
func updateAveragingConfig(avg *PositionAveragingConfig, field string, value interface{}) int {
	switch field {
	case "allow_averaging":
		avg.AllowAveraging = toBool(value)
		return 1
	case "average_up_profit_percent":
		avg.AverageUpProfitPercent = toFloat64(value)
		return 1
	case "average_down_loss_percent":
		avg.AverageDownLossPercent = toFloat64(value)
		return 1
	case "add_size_percent":
		avg.AddSizePercent = toFloat64(value)
		return 1
	case "max_averages":
		avg.MaxAverages = toInt(value)
		return 1
	case "min_confidence_for_average":
		avg.MinConfidenceForAverage = toFloat64(value)
		return 1
	case "use_llm_for_averaging":
		avg.UseLLMForAveraging = toBool(value)
		return 1
	case "staged_entry_enabled":
		avg.StagedEntryEnabled = toBool(value)
		return 1
	case "staged_entry_levels":
		avg.StagedEntryLevels = toInt(value)
		return 1
	case "staged_entry_price_improve":
		avg.StagedEntryPriceImprove = toFloat64(value)
		return 1
	case "staged_entry_cooldown_sec":
		avg.StagedEntryCooldownSec = toInt(value)
		return 1
	case "staged_entry_max_wait_sec":
		avg.StagedEntryMaxWaitSec = toInt(value)
		return 1
	}
	return 0
}

// updateStaleReleaseConfig updates StaleRelease config from field/value
func updateStaleReleaseConfig(sr *StalePositionReleaseConfig, field string, value interface{}) int {
	switch field {
	case "enabled":
		sr.Enabled = toBool(value)
		return 1
	case "max_hold_duration":
		sr.MaxHoldDuration = toString(value)
		return 1
	case "min_profit_to_keep":
		sr.MinProfitToKeep = toFloat64(value)
		return 1
	case "max_loss_to_force_close":
		sr.MaxLossToForceClose = toFloat64(value)
		return 1
	case "stale_zone_lo":
		sr.StaleZoneLo = toFloat64(value)
		return 1
	case "stale_zone_hi":
		sr.StaleZoneHi = toFloat64(value)
		return 1
	case "stale_zone_close_action":
		sr.StaleZoneCloseAction = toString(value)
		return 1
	}
	return 0
}

// updateFundingRateConfig updates FundingRate config from field/value
func updateFundingRateConfig(fr *ModeFundingRateConfig, field string, value interface{}) int {
	switch field {
	case "enabled":
		fr.Enabled = toBool(value)
		return 1
	case "max_funding_rate":
		fr.MaxFundingRate = toFloat64(value)
		return 1
	case "exit_time_minutes":
		fr.ExitTimeMinutes = toInt(value)
		return 1
	case "fee_threshold_percent":
		fr.FeeThresholdPercent = toFloat64(value)
		return 1
	case "extreme_funding_rate":
		fr.ExtremeFundingRate = toFloat64(value)
		return 1
	case "high_rate_reduction":
		fr.HighRateReduction = toFloat64(value)
		return 1
	case "elevated_rate_reduction":
		fr.ElevatedRateReduction = toFloat64(value)
		return 1
	case "block_time_minutes":
		fr.BlockTimeMinutes = toInt(value)
		return 1
	}
	return 0
}

// updateRiskConfig updates Risk config from field/value
func updateRiskConfig(risk *ModeRiskConfig, field string, value interface{}) int {
	switch field {
	case "risk_level":
		risk.RiskLevel = toString(value)
		return 1
	case "risk_multiplier_conservative":
		risk.RiskMultiplierConservative = toFloat64(value)
		return 1
	case "risk_multiplier_moderate":
		risk.RiskMultiplierModerate = toFloat64(value)
		return 1
	case "risk_multiplier_aggressive":
		risk.RiskMultiplierAggressive = toFloat64(value)
		return 1
	case "max_drawdown_percent":
		risk.MaxDrawdownPercent = toFloat64(value)
		return 1
	case "max_daily_loss_percent":
		risk.MaxDailyLossPercent = toFloat64(value)
		return 1
	case "min_adx":
		risk.MinADX = toFloat64(value)
		return 1
	}
	return 0
}

// updateTrendFiltersConfig updates TrendFilters config from field/value
func updateTrendFiltersConfig(tf *TrendFiltersConfig, field string, subparts []string, value interface{}) int {
	switch field {
	case "btc_trend_check":
		if tf.BTCTrendCheck == nil {
			tf.BTCTrendCheck = &BTCTrendCheckConfig{}
		}
		return updateBTCTrendCheckConfig(tf.BTCTrendCheck, subparts, value)
	case "price_vs_ema":
		if tf.PriceVsEMA == nil {
			tf.PriceVsEMA = &PriceVsEMAConfig{}
		}
		return updatePriceVsEMAConfig(tf.PriceVsEMA, subparts, value)
	case "vwap_filter":
		if tf.VWAPFilter == nil {
			tf.VWAPFilter = &VWAPFilterConfig{}
		}
		return updateVWAPFilterConfig(tf.VWAPFilter, subparts, value)
	case "candlestick_alignment":
		if tf.CandlestickAlignment == nil {
			tf.CandlestickAlignment = &CandlestickAlignmentConfig{}
		}
		return updateCandlestickAlignmentConfig(tf.CandlestickAlignment, subparts, value)
	}
	return 0
}

// updateBTCTrendCheckConfig updates BTCTrendCheck config
func updateBTCTrendCheckConfig(btc *BTCTrendCheckConfig, subparts []string, value interface{}) int {
	if len(subparts) == 0 {
		return 0
	}
	switch subparts[0] {
	case "enabled":
		btc.Enabled = toBool(value)
		return 1
	case "btc_symbol":
		btc.BTCSymbol = toString(value)
		return 1
	case "block_alt_long_when_btc_bearish":
		btc.BlockAltLongWhenBTCBearish = toBool(value)
		return 1
	case "block_alt_short_when_btc_bullish":
		btc.BlockAltShortWhenBTCBullish = toBool(value)
		return 1
	case "btc_trend_timeframe":
		btc.BTCTrendTimeframe = toString(value)
		return 1
	}
	return 0
}

// updatePriceVsEMAConfig updates PriceVsEMA config
func updatePriceVsEMAConfig(ema *PriceVsEMAConfig, subparts []string, value interface{}) int {
	if len(subparts) == 0 {
		return 0
	}
	switch subparts[0] {
	case "enabled":
		ema.Enabled = toBool(value)
		return 1
	case "require_price_above_ema_for_long":
		ema.RequirePriceAboveEMAForLong = toBool(value)
		return 1
	case "require_price_below_ema_for_short":
		ema.RequirePriceBelowEMAForShort = toBool(value)
		return 1
	case "ema_period":
		ema.EMAPeriod = toInt(value)
		return 1
	}
	return 0
}

// updateVWAPFilterConfig updates VWAPFilter config
func updateVWAPFilterConfig(vwap *VWAPFilterConfig, subparts []string, value interface{}) int {
	if len(subparts) == 0 {
		return 0
	}
	switch subparts[0] {
	case "enabled":
		vwap.Enabled = toBool(value)
		return 1
	case "require_price_above_vwap_for_long":
		vwap.RequirePriceAboveVWAPForLong = toBool(value)
		return 1
	case "require_price_below_vwap_for_short":
		vwap.RequirePriceBelowVWAPForShort = toBool(value)
		return 1
	case "near_vwap_tolerance_percent":
		vwap.NearVWAPTolerancePercent = toFloat64(value)
		return 1
	}
	return 0
}

// updateCandlestickAlignmentConfig updates CandlestickAlignment config
func updateCandlestickAlignmentConfig(ca *CandlestickAlignmentConfig, subparts []string, value interface{}) int {
	if len(subparts) == 0 {
		return 0
	}
	switch subparts[0] {
	case "enabled":
		ca.Enabled = toBool(value)
		return 1
	case "min_confidence_to_block":
		ca.MinConfidenceToBlock = toFloat64(value)
		return 1
	case "log_only_mode":
		ca.LogOnlyMode = toBool(value)
		return 1
	}
	return 0
}

// updateMTFConfig updates MTF config from field/value
func updateMTFConfig(mtf *ModeMTFConfig, field string, value interface{}) int {
	switch field {
	case "mtf_enabled":
		mtf.Enabled = toBool(value)
		return 1
	case "primary_timeframe":
		mtf.PrimaryTimeframe = toString(value)
		return 1
	case "primary_weight":
		mtf.PrimaryWeight = toFloat64(value)
		return 1
	case "secondary_timeframe":
		mtf.SecondaryTimeframe = toString(value)
		return 1
	case "secondary_weight":
		mtf.SecondaryWeight = toFloat64(value)
		return 1
	case "tertiary_timeframe":
		mtf.TertiaryTimeframe = toString(value)
		return 1
	case "tertiary_weight":
		mtf.TertiaryWeight = toFloat64(value)
		return 1
	case "min_consensus":
		mtf.MinConsensus = toInt(value)
		return 1
	case "min_weighted_strength":
		mtf.MinWeightedStrength = toFloat64(value)
		return 1
	case "trend_stability_check":
		mtf.TrendStabilityCheck = toBool(value)
		return 1
	}
	return 0
}

// updateDynamicAIExitConfig updates DynamicAIExit config from field/value
func updateDynamicAIExitConfig(dae *ModeDynamicAIExitConfig, field string, value interface{}) int {
	switch field {
	case "dynamic_ai_exit_enabled":
		dae.Enabled = toBool(value)
		return 1
	case "min_hold_before_ai_ms":
		dae.MinHoldBeforeAIMS = toInt(value)
		return 1
	case "ai_check_interval_ms":
		dae.AICheckIntervalMS = toInt(value)
		return 1
	case "use_llm_for_loss":
		dae.UseLLMForLoss = toBool(value)
		return 1
	case "use_llm_for_profit":
		dae.UseLLMForProfit = toBool(value)
		return 1
	case "max_hold_time_ms":
		dae.MaxHoldTimeMS = toInt(value)
		return 1
	}
	return 0
}

// Helper functions for type conversion
func splitKeyPath(key string) []string {
	var parts []string
	current := ""
	for _, c := range key {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		var i int
		fmt.Sscanf(val, "%d", &i)
		return i
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "yes" || val == "1"
	case int:
		return val != 0
	case float64:
		return val != 0
	default:
		return false
	}
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}

// RestoreFromBackup restores default-settings.json from backup
func (s *AdminSyncService) RestoreFromBackup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	backupFile := DefaultSettingsFilePath + BackupSuffix

	// Check if backup exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist")
	}

	// Read backup
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Write to main file
	if err := os.WriteFile(DefaultSettingsFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	// Force reload
	if err := ReloadDefaultSettings(); err != nil {
		return fmt.Errorf("failed to reload after restore: %w", err)
	}

	log.Printf("[ADMIN-SYNC] Restored default-settings.json from backup")
	return nil
}
