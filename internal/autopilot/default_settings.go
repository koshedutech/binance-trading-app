package autopilot

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
)

// ====== DEFAULT SETTINGS FILE STRUCTURE ======
// This file provides helper functions to load the default-settings.json file
// which is the SINGLE SOURCE OF TRUTH for all default configuration values.
//
// Story 4.13: Default Settings JSON Foundation
// - default-settings.json is template-only (never used at runtime for active trading)
// - Used ONLY for: (a) new user creation, (b) "Load Defaults" feature, (c) admin sync
// - All runtime settings come from database per-user

// DefaultSettingsFile represents the complete default-settings.json structure
type DefaultSettingsFile struct {
	Metadata             DefaultMetadata                `json:"metadata"`
	GlobalTrading        GlobalTradingDefaults          `json:"global_trading"`
	ModeConfigs          map[string]*ModeFullConfig     `json:"mode_configs"`
	PositionOptimization PositionOptimizationDefaults   `json:"position_optimization"`
	CircuitBreaker       CircuitBreakerDefaults         `json:"circuit_breaker"`
	LLMConfig            LLMConfigDefaults              `json:"llm_config"`
	CapitalAllocation    CapitalAllocationDefaults      `json:"capital_allocation"`
	ScalpReentry         *PositionOptimizationConfig            `json:"scalp_reentry_config,omitempty"`
	SafetySettings       *SafetySettingsAllModes        `json:"safety_settings,omitempty"`
	SettingsRiskIndex    SettingsRiskIndex              `json:"_settings_risk_index"`
	// NOTE: Global EarlyWarning removed - early warning is now per-mode only (see ModeEarlyWarningConfig)
}

// SafetySettingsAllModes contains safety settings for all trading modes
type SafetySettingsAllModes struct {
	Description string               `json:"_description,omitempty"`
	UltraFast   *SafetySettingsMode  `json:"ultra_fast,omitempty"`
	Scalp       *SafetySettingsMode  `json:"scalp,omitempty"`
	Swing       *SafetySettingsMode  `json:"swing,omitempty"`
	Position    *SafetySettingsMode  `json:"position,omitempty"`
}

// SafetySettingsMode contains per-mode safety settings
type SafetySettingsMode struct {
	MaxTradesPerMinute     int     `json:"max_trades_per_minute"`
	MaxTradesPerHour       int     `json:"max_trades_per_hour"`
	MaxTradesPerDay        int     `json:"max_trades_per_day"`
	EnableProfitMonitor    bool    `json:"enable_profit_monitor"`
	ProfitWindowMinutes    int     `json:"profit_window_minutes"`
	MaxLossPercentInWindow float64 `json:"max_loss_percent_in_window"`
	PauseCooldownMinutes   int     `json:"pause_cooldown_minutes"`
	EnableWinRateMonitor   bool    `json:"enable_win_rate_monitor"`
	WinRateSampleSize      int     `json:"win_rate_sample_size"`
	MinWinRateThreshold    float64 `json:"min_win_rate_threshold"`
	WinRateCooldownMinutes int     `json:"win_rate_cooldown_minutes"`
}

// DefaultMetadata holds version and update information
type DefaultMetadata struct {
	Version       string `json:"version"`
	SchemaVersion int    `json:"schema_version"`
	LastUpdated   string `json:"last_updated"`
	UpdatedBy     string `json:"updated_by"`
	Description   string `json:"description"`
}

// GlobalTradingDefaults holds global trading settings
type GlobalTradingDefaults struct {
	RiskLevel              string              `json:"risk_level"`
	MaxUSDAllocation       float64             `json:"max_usd_allocation"`
	ProfitReinvestPercent  float64             `json:"profit_reinvest_percent"`
	ProfitReinvestRiskLevel string             `json:"profit_reinvest_risk_level"`
	RiskInfo               map[string]RiskInfo `json:"_risk_info"`
}

// PositionOptimizationDefaults holds position optimization settings
type PositionOptimizationDefaults struct {
	Averaging AveragingDefaults `json:"averaging"`
	Hedging   HedgingDefaults   `json:"hedging"`
	RiskInfo  map[string]RiskInfo `json:"_risk_info"`
}

// AveragingDefaults holds averaging configuration
type AveragingDefaults struct {
	Enabled                  bool      `json:"enabled"`
	MaxEntries               int       `json:"max_entries"`
	EntrySpacingPercent      float64   `json:"entry_spacing_percent"`
	SizeMultiplierPerLevel   float64   `json:"size_multiplier_per_level"`
	StagedEntryEnabled       bool      `json:"staged_entry_enabled"`
	StagedEntryLevels        int       `json:"staged_entry_levels"`
	StagedEntryPercent       []float64 `json:"staged_entry_percent"`
	StagedEntryCooldownSec   int       `json:"staged_entry_cooldown_sec"`
}

// HedgingDefaults holds hedging configuration
type HedgingDefaults struct {
	Enabled                bool    `json:"enabled"`
	MinConfidenceForHedge  float64 `json:"min_confidence_for_hedge"`
	ExistingMustBeInProfit float64 `json:"existing_must_be_in_profit"`
	MaxHedgeSizePercent    float64 `json:"max_hedge_size_percent"`
	AllowSameModeHedge     bool    `json:"allow_same_mode_hedge"`
}

// CircuitBreakerDefaults holds global circuit breaker settings
type CircuitBreakerDefaults struct {
	Global   GlobalCircuitBreakerConfig `json:"global"`
	RiskInfo map[string]RiskInfo        `json:"_risk_info"`
}

// GlobalCircuitBreakerConfig holds the global circuit breaker configuration
type GlobalCircuitBreakerConfig struct {
	Enabled                bool    `json:"enabled"`
	MaxLossPerHour         float64 `json:"max_loss_per_hour"`
	MaxDailyLoss           float64 `json:"max_daily_loss"`
	MaxConsecutiveLosses   int     `json:"max_consecutive_losses"`
	CooldownMinutes        int     `json:"cooldown_minutes"`
	MaxTradesPerMinute     int     `json:"max_trades_per_minute"`
	MaxDailyTrades         int     `json:"max_daily_trades"`
}

// LLMConfigDefaults holds LLM configuration settings
type LLMConfigDefaults struct {
	Global   GlobalLLMConfig     `json:"global"`
	RiskInfo map[string]RiskInfo `json:"_risk_info"`
}

// GlobalLLMConfig holds the global LLM configuration
type GlobalLLMConfig struct {
	Enabled          bool   `json:"enabled"`
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	TimeoutMS        int    `json:"timeout_ms"`
	RetryCount       int    `json:"retry_count"`
	CacheDurationSec int    `json:"cache_duration_sec"`
}

// EarlyWarningDefaults holds early warning system settings
type EarlyWarningDefaults struct {
	Enabled            bool    `json:"enabled"`
	StartAfterMinutes  int     `json:"start_after_minutes"`
	CheckIntervalSecs  int     `json:"check_interval_secs"`
	OnlyUnderwater     bool    `json:"only_underwater"`
	MinLossPercent     float64 `json:"min_loss_percent"`
	CloseOnReversal    bool    `json:"close_on_reversal"`
	// Extended early warning fields (Story 9.4 Phase 1)
	TightenSLOnWarning      bool    `json:"tighten_sl_on_warning"`
	MinConfidence           float64 `json:"min_confidence"`
	MaxLLMCallsPerPos       int     `json:"max_llm_calls_per_pos"`
	CloseMinHoldMins        int     `json:"close_min_hold_mins"`
	CloseMinConfidence      float64 `json:"close_min_confidence"`
	CloseRequireConsecutive int     `json:"close_require_consecutive"`
	CloseSLProximityPct     int     `json:"close_sl_proximity_pct"`
}

// CapitalAllocationDefaults holds capital allocation percentages per mode
type CapitalAllocationDefaults struct {
	UltraFastPercent      float64 `json:"ultra_fast_percent"`
	ScalpPercent          float64 `json:"scalp_percent"`
	SwingPercent          float64 `json:"swing_percent"`
	PositionPercent       float64 `json:"position_percent"`
	AllowDynamicRebalance bool    `json:"allow_dynamic_rebalance"`
	RebalanceThresholdPct float64 `json:"rebalance_threshold_pct"`
}

// SettingsRiskIndex categorizes settings by risk level
type SettingsRiskIndex struct {
	HighRiskSettings   []string `json:"high_risk_settings"`
	MediumRiskSettings []string `json:"medium_risk_settings"`
	LowRiskSettings    []string `json:"low_risk_settings"`
}

// RiskInfo provides risk information for a specific setting
type RiskInfo struct {
	Impact         string `json:"impact"`
	Recommendation string `json:"recommendation"`
}

// ====== SINGLETON LOADER ======

var (
	defaultSettings     *DefaultSettingsFile
	defaultSettingsOnce sync.Once
	defaultSettingsErr  error
)

// LoadDefaultSettings loads the default-settings.json file (singleton pattern)
// This function is called once and the result is cached for the lifetime of the process.
// Returns the parsed DefaultSettingsFile or an error if loading/parsing fails.
func LoadDefaultSettings() (*DefaultSettingsFile, error) {
	defaultSettingsOnce.Do(func() {
		data, err := os.ReadFile("default-settings.json")
		if err != nil {
			defaultSettingsErr = fmt.Errorf("failed to read default-settings.json: %w", err)
			return
		}

		defaultSettings = &DefaultSettingsFile{}
		if err := json.Unmarshal(data, defaultSettings); err != nil {
			defaultSettingsErr = fmt.Errorf("failed to parse default-settings.json: %w", err)
			return
		}

		log.Printf("[DEFAULT-SETTINGS] Loaded version %s (schema: %d, updated: %s by %s)",
			defaultSettings.Metadata.Version,
			defaultSettings.Metadata.SchemaVersion,
			defaultSettings.Metadata.LastUpdated,
			defaultSettings.Metadata.UpdatedBy)
	})

	return defaultSettings, defaultSettingsErr
}

// GetDefaultModeFullConfig returns default ModeFullConfig for a specific mode from default-settings.json
// Returns a deep copy to prevent accidental mutation of the cached defaults
func GetDefaultModeFullConfig(modeName string) (*ModeFullConfig, error) {
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return nil, err
	}

	config, exists := defaults.ModeConfigs[modeName]
	if !exists {
		return nil, fmt.Errorf("mode %s not found in default settings", modeName)
	}

	// Return a deep copy to prevent mutation
	configCopy := deepCopyModeConfig(config)
	return configCopy, nil
}

// GetAllDefaultModeFullConfigs returns all default ModeFullConfig as a map from default-settings.json
// Returns deep copies to prevent accidental mutation of the cached defaults
func GetAllDefaultModeFullConfigs() (map[string]*ModeFullConfig, error) {
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return nil, err
	}

	// Return copies of all mode configs
	result := make(map[string]*ModeFullConfig)
	for name, config := range defaults.ModeConfigs {
		result[name] = deepCopyModeConfig(config)
	}
	return result, nil
}

// GetDefaultSettingsJSON returns the entire defaults as JSON bytes
// Useful for API responses or admin panel display
func GetDefaultSettingsJSON() ([]byte, error) {
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(defaults, "", "  ")
}

// deepCopyModeConfig creates a deep copy of ModeFullConfig using JSON marshaling
// This ensures the original cached config is never mutated
func deepCopyModeConfig(src *ModeFullConfig) *ModeFullConfig {
	if src == nil {
		return nil
	}

	// Use JSON marshaling for deep copy
	data, err := json.Marshal(src)
	if err != nil {
		log.Printf("[DEFAULT-SETTINGS] ERROR: Failed to marshal ModeFullConfig for deep copy: %v", err)
		return nil
	}

	var dst ModeFullConfig
	if err := json.Unmarshal(data, &dst); err != nil {
		log.Printf("[DEFAULT-SETTINGS] ERROR: Failed to unmarshal ModeFullConfig for deep copy: %v", err)
		return nil
	}

	return &dst
}

// GetDefaultPositionOptimizationConfig returns default PositionOptimizationConfig from default-settings.json
// ScalpReentry is stored separately from ModeConfigs because it's a Position Optimization
// feature, not a trading mode
func GetDefaultPositionOptimizationConfig() (*PositionOptimizationConfig, error) {
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return nil, err
	}

	if defaults.ScalpReentry == nil {
		return nil, fmt.Errorf("scalp_reentry_config not found in default settings")
	}

	// Return a deep copy to prevent mutation
	configCopy := deepCopyPositionOptimizationConfig(defaults.ScalpReentry)
	return configCopy, nil
}

// deepCopyPositionOptimizationConfig creates a deep copy of PositionOptimizationConfig using JSON marshaling
func deepCopyPositionOptimizationConfig(src *PositionOptimizationConfig) *PositionOptimizationConfig {
	if src == nil {
		return nil
	}

	// Use JSON marshaling for deep copy
	data, err := json.Marshal(src)
	if err != nil {
		log.Printf("[DEFAULT-SETTINGS] ERROR: Failed to marshal PositionOptimizationConfig for deep copy: %v", err)
		return nil
	}

	var dst PositionOptimizationConfig
	if err := json.Unmarshal(data, &dst); err != nil {
		log.Printf("[DEFAULT-SETTINGS] ERROR: Failed to unmarshal PositionOptimizationConfig for deep copy: %v", err)
		return nil
	}

	return &dst
}

// ReloadDefaultSettings forces a reload of the defaults from disk
// This is useful for admin sync operations when default-settings.json is updated
// WARNING: This resets the singleton, so the next call to LoadDefaultSettings will re-read the file
func ReloadDefaultSettings() error {
	// Reset the sync.Once to allow re-initialization
	defaultSettingsOnce = sync.Once{}
	defaultSettings = nil
	defaultSettingsErr = nil

	// Immediately reload to validate the file
	_, err := LoadDefaultSettings()
	if err != nil {
		return fmt.Errorf("failed to reload default settings: %w", err)
	}

	log.Printf("[DEFAULT-SETTINGS] Successfully reloaded default-settings.json from disk")
	return nil
}

// ValidateDefaultSettings validates the default-settings.json structure
// Returns error if any required sections are missing or invalid
func ValidateDefaultSettings() error {
	defaults, err := LoadDefaultSettings()
	if err != nil {
		return err
	}

	// Validate metadata
	if defaults.Metadata.Version == "" {
		return fmt.Errorf("metadata.version is required")
	}
	if defaults.Metadata.SchemaVersion < 1 {
		return fmt.Errorf("metadata.schema_version must be >= 1")
	}

	// Validate mode configs (4 trading modes - scalp_reentry is NOT a mode, it's Position Optimization)
	if len(defaults.ModeConfigs) == 0 {
		return fmt.Errorf("mode_configs is empty - at least one mode must be defined")
	}

	// NOTE: scalp_reentry is NOT a trading mode - it's stored separately as ScalpReentry
	requiredModes := []string{"ultra_fast", "scalp", "swing", "position"}
	for _, mode := range requiredModes {
		config, exists := defaults.ModeConfigs[mode]
		if !exists {
			return fmt.Errorf("mode_configs.%s is required", mode)
		}

		// Validate trend_filters exists in each mode config (Story 9.5)
		if config.TrendFilters == nil {
			return fmt.Errorf("mode_configs.%s.trend_filters is required (Story 9.5)", mode)
		}

		// Validate trend_filters sub-configs exist
		if config.TrendFilters.BTCTrendCheck == nil {
			return fmt.Errorf("mode_configs.%s.trend_filters.btc_trend_check is required", mode)
		}
		if config.TrendFilters.PriceVsEMA == nil {
			return fmt.Errorf("mode_configs.%s.trend_filters.price_vs_ema is required", mode)
		}
		if config.TrendFilters.VWAPFilter == nil {
			return fmt.Errorf("mode_configs.%s.trend_filters.vwap_filter is required", mode)
		}
	}

	// Validate scalp_reentry_config exists separately
	if defaults.ScalpReentry == nil {
		return fmt.Errorf("scalp_reentry_config is required (stored separately from mode_configs)")
	}

	// Validate capital allocation sums to 100%
	// NOTE: Only 4 modes - scalp_reentry is a Position Optimization method, NOT a trading mode
	total := defaults.CapitalAllocation.UltraFastPercent +
		defaults.CapitalAllocation.ScalpPercent +
		defaults.CapitalAllocation.SwingPercent +
		defaults.CapitalAllocation.PositionPercent

	// Allow 1% tolerance for floating point precision
	if total < 99.0 || total > 101.0 {
		return fmt.Errorf("capital_allocation percentages must sum to 100%% (got %.1f%%, expected 4 modes: ultra_fast, scalp, swing, position)", total)
	}

	log.Printf("[DEFAULT-SETTINGS] Validation passed for default-settings.json")
	return nil
}

// GetDefaultTrendFiltersForMode returns default TrendFiltersConfig for a mode
// Used when database has NULL trend_filters (backward compatibility - Story 9.5)
func GetDefaultTrendFiltersForMode(modeName string) *TrendFiltersConfig {
	defaults, err := LoadDefaultSettings()
	if err != nil {
		log.Printf("[DEFAULT-SETTINGS] ERROR: Failed to load defaults for trend_filters: %v", err)
		return nil
	}

	config, exists := defaults.ModeConfigs[modeName]
	if !exists || config.TrendFilters == nil {
		log.Printf("[DEFAULT-SETTINGS] WARNING: No trend_filters found for mode %s", modeName)
		return nil
	}

	// Deep copy to prevent mutation of cached defaults
	data, err := json.Marshal(config.TrendFilters)
	if err != nil {
		log.Printf("[DEFAULT-SETTINGS] ERROR: Failed to marshal TrendFiltersConfig for deep copy: %v", err)
		return nil
	}

	var copy TrendFiltersConfig
	if err := json.Unmarshal(data, &copy); err != nil {
		log.Printf("[DEFAULT-SETTINGS] ERROR: Failed to unmarshal TrendFiltersConfig for deep copy: %v", err)
		return nil
	}

	return &copy
}
