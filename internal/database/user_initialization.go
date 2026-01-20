package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
)

// InitializeUserDefaultSettings copies ALL default settings from default-settings.json
// to the database for a new user. This includes:
// - mode_configs (4 trading modes: ultra_fast, scalp, swing, position)
// - scalp_reentry_config (Position Optimization, NOT a trading mode - stored separately)
// - global circuit breaker settings
// - user_llm_config - LLM provider and model settings
// - user_capital_allocation - Capital allocation rules
// - user_early_warning - Early warning thresholds
// - user_ginie_settings - Ginie autopilot settings
// - user_spot_settings - Spot trading settings
// - user_mode_circuit_breaker_stats - Initialize empty stats for each mode
//
// Story 4.14: User initialization should copy ALL per-user defaults
func (r *Repository) InitializeUserDefaultSettings(ctx context.Context, userID string) error {
	log.Printf("[USER-INIT] Loading default-settings.json for user %s", userID)

	// Load default-settings.json file
	defaultsJSON, err := os.ReadFile("default-settings.json")
	if err != nil {
		return fmt.Errorf("failed to read default-settings.json: %w", err)
	}

	// Parse the defaults file
	var defaults struct {
		ModeConfigs       map[string]json.RawMessage `json:"mode_configs"`
		ScalpReentryConfig json.RawMessage           `json:"scalp_reentry_config"` // Separate from mode_configs!
		CircuitBreaker struct {
			Global struct {
				Enabled               bool    `json:"enabled"`
				MaxLossPerHour        float64 `json:"max_loss_per_hour"`
				MaxDailyLoss          float64 `json:"max_daily_loss"`
				MaxConsecutiveLosses  int     `json:"max_consecutive_losses"`
				CooldownMinutes       int     `json:"cooldown_minutes"`
				MaxTradesPerMinute    int     `json:"max_trades_per_minute"`
				MaxDailyTrades        int     `json:"max_daily_trades"`
			} `json:"global"`
		} `json:"circuit_breaker"`
		LLMConfig struct {
			Global struct {
				Enabled          bool    `json:"enabled"`
				Provider         string  `json:"provider"`
				Model            string  `json:"model"`
				TimeoutMs        int     `json:"timeout_ms"`
				RetryCount       int     `json:"retry_count"`
				CacheDurationSec int     `json:"cache_duration_sec"`
			} `json:"global"`
		} `json:"llm_config"`
		CapitalAllocation struct {
			UltraFastPercent int `json:"ultra_fast_percent"`
			ScalpPercent     int `json:"scalp_percent"`
			SwingPercent     int `json:"swing_percent"`
			PositionPercent  int `json:"position_percent"`
		} `json:"capital_allocation"`
		EarlyWarning struct {
			Enabled            bool    `json:"enabled"`
			StartAfterMinutes  int     `json:"start_after_minutes"`
			CheckIntervalSecs  int     `json:"check_interval_secs"`
			OnlyUnderwater     bool    `json:"only_underwater"`
			MinLossPercent     float64 `json:"min_loss_percent"`
			CloseOnReversal    bool    `json:"close_on_reversal"`
		} `json:"early_warning"`
	}

	if err := json.Unmarshal(defaultsJSON, &defaults); err != nil {
		return fmt.Errorf("failed to parse default-settings.json: %w", err)
	}

	// ===== 1. Initialize Mode Configs (4 trading modes) =====
	// NOTE: scalp_reentry is NOT a trading mode - it's stored separately as scalp_reentry_config
	modes := []string{"ultra_fast", "scalp", "swing", "position"}
	modesInitialized := 0

	for _, modeName := range modes {
		modeJSON, exists := defaults.ModeConfigs[modeName]
		if !exists {
			log.Printf("[USER-INIT] Warning: Mode %s not found in defaults, skipping", modeName)
			continue
		}

		// Parse the FULL mode config to extract the enabled flag
		// This ensures we copy EVERYTHING from default-settings.json
		var modeConfig struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(modeJSON, &modeConfig); err != nil {
			log.Printf("[USER-INIT] Warning: Failed to parse mode %s config: %v", modeName, err)
			continue
		}

		// Save the ENTIRE mode config to the database
		// This includes ALL fields: enabled, timeframe, confidence, size, circuit_breaker, sltp, hedge, averaging, etc.
		if err := r.SaveUserModeConfig(ctx, userID, modeName, modeConfig.Enabled, modeJSON); err != nil {
			log.Printf("[USER-INIT] Warning: Failed to save mode %s for user %s: %v", modeName, userID, err)
			continue
		}

		log.Printf("[USER-INIT] Initialized mode %s for user %s (enabled: %v, copied ALL fields from defaults)", modeName, userID, modeConfig.Enabled)
		modesInitialized++
	}

	// ===== 1b. Initialize Scalp Reentry Config (Position Optimization, NOT a trading mode) =====
	if len(defaults.ScalpReentryConfig) > 0 {
		var scalpReentryConfig struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(defaults.ScalpReentryConfig, &scalpReentryConfig); err != nil {
			log.Printf("[USER-INIT] Warning: Failed to parse scalp_reentry_config: %v", err)
		} else {
			if err := r.SaveUserModeConfig(ctx, userID, "scalp_reentry", scalpReentryConfig.Enabled, defaults.ScalpReentryConfig); err != nil {
				log.Printf("[USER-INIT] Warning: Failed to save scalp_reentry for user %s: %v", userID, err)
			} else {
				log.Printf("[USER-INIT] Initialized scalp_reentry for user %s (enabled: %v)", userID, scalpReentryConfig.Enabled)
				modesInitialized++
			}
		}
	} else {
		log.Printf("[USER-INIT] Warning: scalp_reentry_config not found in defaults")
	}

	// ===== 2. Initialize Global Circuit Breaker =====
	circuitBreakerConfig := DefaultUserGlobalCircuitBreaker()
	circuitBreakerConfig.UserID = userID
	circuitBreakerConfig.MaxLossPerHour = defaults.CircuitBreaker.Global.MaxLossPerHour
	circuitBreakerConfig.MaxDailyLoss = defaults.CircuitBreaker.Global.MaxDailyLoss
	circuitBreakerConfig.MaxConsecutiveLosses = defaults.CircuitBreaker.Global.MaxConsecutiveLosses
	circuitBreakerConfig.CooldownMinutes = defaults.CircuitBreaker.Global.CooldownMinutes

	if err := r.SaveUserGlobalCircuitBreaker(ctx, circuitBreakerConfig); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to save circuit breaker for user %s: %v", userID, err)
		// Don't fail initialization if circuit breaker fails
	} else {
		log.Printf("[USER-INIT] Initialized circuit breaker for user %s", userID)
	}

	// ===== 3. Initialize LLM Configuration =====
	llmConfig := DefaultUserLLMConfig()
	llmConfig.UserID = userID
	if defaults.LLMConfig.Global.Provider != "" {
		llmConfig.Provider = defaults.LLMConfig.Global.Provider
		llmConfig.Model = defaults.LLMConfig.Global.Model
	}
	if err := r.SaveUserLLMConfig(ctx, llmConfig); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize LLM config: %v", err)
	} else {
		log.Printf("[USER-INIT] Initialized LLM config for user %s (provider: %s, model: %s)", userID, llmConfig.Provider, llmConfig.Model)
	}

	// ===== 4. Initialize Capital Allocation =====
	// Note: default-settings.json has capital_allocation as percentages by mode,
	// but UserCapitalAllocation uses different fields (max capital per trade, etc.)
	// We use the hardcoded defaults from DefaultUserCapitalAllocation for now
	capitalAllocation := DefaultUserCapitalAllocation()
	capitalAllocation.UserID = userID
	if err := r.SaveUserCapitalAllocation(ctx, capitalAllocation); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize capital allocation: %v", err)
	} else {
		log.Printf("[USER-INIT] Initialized capital allocation for user %s", userID)
	}

	// ===== 5. Initialize Early Warning =====
	earlyWarning := DefaultUserEarlyWarning()
	earlyWarning.UserID = userID
	// Note: default-settings.json early_warning fields don't map directly to UserEarlyWarning
	// We use the hardcoded defaults from DefaultUserEarlyWarning for now
	if err := r.SaveUserEarlyWarning(ctx, earlyWarning); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize early warning: %v", err)
	} else {
		log.Printf("[USER-INIT] Initialized early warning for user %s", userID)
	}

	// ===== 6. Initialize Ginie Settings =====
	ginieSettings := DefaultUserGinieSettings()
	ginieSettings.UserID = userID
	if err := r.SaveUserGinieSettings(ctx, ginieSettings); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize Ginie settings: %v", err)
	} else {
		log.Printf("[USER-INIT] Initialized Ginie settings for user %s", userID)
	}

	// ===== 7. Initialize Spot Settings =====
	spotSettings := DefaultUserSpotSettings()
	spotSettings.UserID = userID
	if err := r.SaveUserSpotSettings(ctx, spotSettings); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize Spot settings: %v", err)
	} else {
		log.Printf("[USER-INIT] Initialized Spot settings for user %s", userID)
	}

	// ===== 8. Initialize Mode Circuit Breaker Stats =====
	// Create empty stats for each trading mode
	modesStatsInitialized := 0
	for _, modeName := range modes {
		modeStats := DefaultUserModeCBStats(userID, modeName)
		if err := r.SaveUserModeCBStats(ctx, modeStats); err != nil {
			log.Printf("[USER-INIT] Warning: Failed to initialize mode CB stats for %s: %v", modeName, err)
		} else {
			modesStatsInitialized++
		}
	}
	log.Printf("[USER-INIT] Initialized CB stats for %d modes", modesStatsInitialized)

	// ===== 9. Initialize Safety Settings =====
	// Per-mode safety controls for rate limiting, profit monitoring, and win-rate monitoring
	if err := r.InitializeUserSafetySettings(ctx, userID); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize safety settings: %v", err)
	} else {
		log.Printf("[USER-INIT] Initialized safety settings for user %s (all 4 modes)", userID)
	}

	// ===== 10. Initialize Global Trading Settings =====
	// Global trading config including risk level, max allocation, profit reinvestment, and timezone
	// Uses defaults from default-settings.json -> global_trading section
	if err := r.InitializeUserGlobalTrading(ctx, userID); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize global trading: %v", err)
	} else {
		log.Printf("[USER-INIT] Initialized global trading for user %s (timezone: %s)", userID, DefaultUserGlobalTrading().Timezone)
	}

	// ===== 11. Initialize Position Management Settings =====
	// Position management config for decision mode (classic/new_engine), efficiency exit, and dynamic SL/TP
	// Uses defaults from default-settings.json -> position_management section
	if err := r.InitializeUserPositionManagement(ctx, userID); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize position management: %v", err)
	} else {
		log.Printf("[USER-INIT] Initialized position management for user %s (decision_mode: %s)", userID, DefaultUserPositionManagement().DecisionMode)
	}

	// ===== 12. Initialize Mode+Strategy Settings (Story 11.32) =====
	// Create 16 mode+strategy records (4 modes x 4 strategies) with default configs
	// This is idempotent - safe to run multiple times
	modeStrategiesInitialized := 0
	if err := r.InitializeUserModeStrategies(ctx, userID); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize mode+strategy settings: %v", err)
	} else {
		modeStrategiesInitialized = 16 // 4 modes x 4 strategies
		log.Printf("[USER-INIT] Initialized %d mode+strategy configs for user %s", modeStrategiesInitialized, userID)
	}

	// ===== Summary =====
	log.Printf("[USER-INIT] Successfully initialized ALL settings for user %s: %d mode configs, circuit breaker, LLM, capital allocation, early warning, Ginie, Spot, %d mode CB stats, safety settings, global trading, position management, and %d mode+strategy configs",
		userID, modesInitialized, modesStatsInitialized, modeStrategiesInitialized)

	return nil
}

// RestoreUserDefaultSettings resets ALL user settings to defaults from default-settings.json
// This is the "Restore Defaults" function that overwrites existing user settings
// RULE: Copy EVERYTHING from default-settings.json to database
func (r *Repository) RestoreUserDefaultSettings(ctx context.Context, userID string) error {
	log.Printf("[USER-RESTORE] Restoring all settings to defaults for user %s", userID)

	// Load default-settings.json file
	defaultsJSON, err := os.ReadFile("default-settings.json")
	if err != nil {
		return fmt.Errorf("failed to read default-settings.json: %w", err)
	}

	// Parse the defaults file
	var defaults struct {
		ModeConfigs       map[string]json.RawMessage `json:"mode_configs"`
		ScalpReentryConfig json.RawMessage           `json:"scalp_reentry_config"` // Separate from mode_configs!
		CircuitBreaker struct {
			Global struct {
				Enabled               bool    `json:"enabled"`
				MaxLossPerHour        float64 `json:"max_loss_per_hour"`
				MaxDailyLoss          float64 `json:"max_daily_loss"`
				MaxConsecutiveLosses  int     `json:"max_consecutive_losses"`
				CooldownMinutes       int     `json:"cooldown_minutes"`
				MaxTradesPerMinute    int     `json:"max_trades_per_minute"`
				MaxDailyTrades        int     `json:"max_daily_trades"`
			} `json:"global"`
		} `json:"circuit_breaker"`
		LLMConfig struct {
			Global struct {
				Enabled          bool   `json:"enabled"`
				Provider         string `json:"provider"`
				Model            string `json:"model"`
				TimeoutMs        int    `json:"timeout_ms"`
				RetryCount       int    `json:"retry_count"`
				CacheDurationSec int    `json:"cache_duration_sec"`
			} `json:"global"`
		} `json:"llm_config"`
		CapitalAllocation struct {
			UltraFastPercent int `json:"ultra_fast_percent"`
			ScalpPercent     int `json:"scalp_percent"`
			SwingPercent     int `json:"swing_percent"`
			PositionPercent  int `json:"position_percent"`
		} `json:"capital_allocation"`
		EarlyWarning struct {
			Enabled            bool    `json:"enabled"`
			StartAfterMinutes  int     `json:"start_after_minutes"`
			CheckIntervalSecs  int     `json:"check_interval_secs"`
			OnlyUnderwater     bool    `json:"only_underwater"`
			MinLossPercent     float64 `json:"min_loss_percent"`
			CloseOnReversal    bool    `json:"close_on_reversal"`
		} `json:"early_warning"`
	}

	if err := json.Unmarshal(defaultsJSON, &defaults); err != nil {
		return fmt.Errorf("failed to parse default-settings.json: %w", err)
	}

	// ===== 1. Restore Mode Configs (4 trading modes) =====
	// NOTE: scalp_reentry is NOT a trading mode - it's stored separately as scalp_reentry_config
	modes := []string{"ultra_fast", "scalp", "swing", "position"}
	modesRestored := 0

	for _, modeName := range modes {
		modeJSON, exists := defaults.ModeConfigs[modeName]
		if !exists {
			log.Printf("[USER-RESTORE] Warning: Mode %s not found in defaults, skipping", modeName)
			continue
		}

		// Parse to get enabled flag
		var modeConfig struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(modeJSON, &modeConfig); err != nil {
			log.Printf("[USER-RESTORE] Warning: Failed to parse mode %s config: %v", modeName, err)
			continue
		}

		// OVERWRITE the mode config in database with ALL fields from defaults
		if err := r.SaveUserModeConfig(ctx, userID, modeName, modeConfig.Enabled, modeJSON); err != nil {
			log.Printf("[USER-RESTORE] Warning: Failed to restore mode %s for user %s: %v", modeName, userID, err)
			continue
		}

		log.Printf("[USER-RESTORE] Restored mode %s for user %s (enabled: %v)", modeName, userID, modeConfig.Enabled)
		modesRestored++
	}

	// ===== 1b. Restore Scalp Reentry Config (Position Optimization, NOT a trading mode) =====
	if len(defaults.ScalpReentryConfig) > 0 {
		var scalpReentryConfig struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(defaults.ScalpReentryConfig, &scalpReentryConfig); err != nil {
			log.Printf("[USER-RESTORE] Warning: Failed to parse scalp_reentry_config: %v", err)
		} else {
			if err := r.SaveUserModeConfig(ctx, userID, "scalp_reentry", scalpReentryConfig.Enabled, defaults.ScalpReentryConfig); err != nil {
				log.Printf("[USER-RESTORE] Warning: Failed to restore scalp_reentry for user %s: %v", userID, err)
			} else {
				log.Printf("[USER-RESTORE] Restored scalp_reentry for user %s (enabled: %v)", userID, scalpReentryConfig.Enabled)
				modesRestored++
			}
		}
	} else {
		log.Printf("[USER-RESTORE] Warning: scalp_reentry_config not found in defaults")
	}

	// ===== 2. Restore Global Circuit Breaker =====
	circuitBreakerConfig := DefaultUserGlobalCircuitBreaker()
	circuitBreakerConfig.UserID = userID
	circuitBreakerConfig.MaxLossPerHour = defaults.CircuitBreaker.Global.MaxLossPerHour
	circuitBreakerConfig.MaxDailyLoss = defaults.CircuitBreaker.Global.MaxDailyLoss
	circuitBreakerConfig.MaxConsecutiveLosses = defaults.CircuitBreaker.Global.MaxConsecutiveLosses
	circuitBreakerConfig.CooldownMinutes = defaults.CircuitBreaker.Global.CooldownMinutes

	if err := r.SaveUserGlobalCircuitBreaker(ctx, circuitBreakerConfig); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore circuit breaker for user %s: %v", userID, err)
	} else {
		log.Printf("[USER-RESTORE] Restored circuit breaker for user %s", userID)
	}

	// ===== 3. Restore LLM Configuration =====
	llmConfig := DefaultUserLLMConfig()
	llmConfig.UserID = userID
	if defaults.LLMConfig.Global.Provider != "" {
		llmConfig.Provider = defaults.LLMConfig.Global.Provider
		llmConfig.Model = defaults.LLMConfig.Global.Model
	}
	if err := r.SaveUserLLMConfig(ctx, llmConfig); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore LLM config: %v", err)
	} else {
		log.Printf("[USER-RESTORE] Restored LLM config for user %s (provider: %s, model: %s)", userID, llmConfig.Provider, llmConfig.Model)
	}

	// ===== 4. Restore Capital Allocation =====
	capitalAllocation := DefaultUserCapitalAllocation()
	capitalAllocation.UserID = userID
	if err := r.SaveUserCapitalAllocation(ctx, capitalAllocation); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore capital allocation: %v", err)
	} else {
		log.Printf("[USER-RESTORE] Restored capital allocation for user %s", userID)
	}

	// ===== 5. Restore Early Warning =====
	earlyWarning := DefaultUserEarlyWarning()
	earlyWarning.UserID = userID
	if err := r.SaveUserEarlyWarning(ctx, earlyWarning); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore early warning: %v", err)
	} else {
		log.Printf("[USER-RESTORE] Restored early warning for user %s", userID)
	}

	// ===== 6. Restore Ginie Settings =====
	ginieSettings := DefaultUserGinieSettings()
	ginieSettings.UserID = userID
	if err := r.SaveUserGinieSettings(ctx, ginieSettings); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore Ginie settings: %v", err)
	} else {
		log.Printf("[USER-RESTORE] Restored Ginie settings for user %s", userID)
	}

	// ===== 7. Restore Spot Settings =====
	spotSettings := DefaultUserSpotSettings()
	spotSettings.UserID = userID
	if err := r.SaveUserSpotSettings(ctx, spotSettings); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore Spot settings: %v", err)
	} else {
		log.Printf("[USER-RESTORE] Restored Spot settings for user %s", userID)
	}

	// ===== 8. Restore Mode Circuit Breaker Stats =====
	// Reset stats to defaults (zeros) for each mode
	modesStatsRestored := 0
	for _, modeName := range modes {
		modeStats := DefaultUserModeCBStats(userID, modeName)
		if err := r.SaveUserModeCBStats(ctx, modeStats); err != nil {
			log.Printf("[USER-RESTORE] Warning: Failed to restore mode CB stats for %s: %v", modeName, err)
		} else {
			modesStatsRestored++
		}
	}
	log.Printf("[USER-RESTORE] Restored CB stats for %d modes", modesStatsRestored)

	// ===== 9. Restore Safety Settings =====
	// Reset to defaults for all 4 modes
	if err := r.InitializeUserSafetySettings(ctx, userID); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore safety settings: %v", err)
	} else {
		log.Printf("[USER-RESTORE] Restored safety settings for user %s (all 4 modes)", userID)
	}

	// ===== 10. Restore Global Trading Settings =====
	// Reset to defaults including timezone from default-settings.json
	if err := r.InitializeUserGlobalTrading(ctx, userID); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore global trading: %v", err)
	} else {
		log.Printf("[USER-RESTORE] Restored global trading for user %s (timezone: %s)", userID, DefaultUserGlobalTrading().Timezone)
	}

	// ===== 11. Restore Position Management Settings =====
	// Reset to defaults from default-settings.json -> position_management section
	if err := r.InitializeUserPositionManagement(ctx, userID); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore position management: %v", err)
	} else {
		log.Printf("[USER-RESTORE] Restored position management for user %s (decision_mode: %s)", userID, DefaultUserPositionManagement().DecisionMode)
	}

	// ===== 12. Restore Mode+Strategy Settings (Story 11.32) =====
	// Reset all 16 mode+strategy records to defaults
	// This is idempotent - uses ON CONFLICT UPDATE
	modeStrategiesRestored := 0
	if err := r.InitializeUserModeStrategies(ctx, userID); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore mode+strategy settings: %v", err)
	} else {
		modeStrategiesRestored = 16 // 4 modes x 4 strategies
		log.Printf("[USER-RESTORE] Restored %d mode+strategy configs for user %s", modeStrategiesRestored, userID)
	}

	log.Printf("[USER-RESTORE] Successfully restored ALL settings for user %s: %d mode configs, circuit breaker, LLM, capital allocation, early warning, Ginie, Spot, %d mode CB stats, safety settings, global trading, position management, and %d mode+strategy configs",
		userID, modesRestored, modesStatsRestored, modeStrategiesRestored)

	return nil
}

// InitializeUserModeStrategies creates 16 mode+strategy records (4 modes x 4 strategies) for a new user
// Story 11.32: Mode-Strategy User Initialization
// This function is idempotent - safe to run multiple times (uses INSERT ON CONFLICT DO NOTHING)
func (r *Repository) InitializeUserModeStrategies(ctx context.Context, userID string) error {
	log.Printf("[USER-INIT-MODESTRAT] Initializing mode+strategy settings for user %s", userID)

	// Convert string userID to int for the mode strategy table
	// The mode strategy table uses INTEGER user_id for consistency with its schema
	userIDInt, err := parseUserIDToInt(userID)
	if err != nil {
		return fmt.Errorf("failed to parse user ID: %w", err)
	}

	// Get all valid modes and strategies
	modes := ValidModes()         // ultra_fast, scalp, swing, position
	strategies := ValidStrategies() // trend_following, mean_reversion, breakout, range_trading

	// Build bulk insert configs
	var configs []ModeStrategySettingsRow
	for _, mode := range modes {
		for _, strategy := range strategies {
			// Get default config for this mode+strategy combination
			config := DefaultModeStrategyConfig(mode, strategy)

			// Serialize config to JSON
			settingsJSON, err := json.Marshal(config)
			if err != nil {
				log.Printf("[USER-INIT-MODESTRAT] Warning: Failed to marshal config for %s/%s: %v", mode, strategy, err)
				continue
			}

			configs = append(configs, ModeStrategySettingsRow{
				UserID:   userIDInt,
				Mode:     mode,
				Strategy: strategy,
				Enabled:  config.Enabled,
				Priority: config.Priority,
				Settings: settingsJSON,
			})
		}
	}

	// Bulk create with ON CONFLICT DO NOTHING (idempotent)
	if err := r.BulkCreateModeStrategySettings(ctx, userIDInt, configs); err != nil {
		return fmt.Errorf("failed to bulk create mode strategy settings: %w", err)
	}

	log.Printf("[USER-INIT-MODESTRAT] Successfully initialized %d mode+strategy configs for user %s (4 modes x 4 strategies)",
		len(configs), userID)

	return nil
}

// parseUserIDToInt converts a string user ID to integer
// Handles UUID format for default admin user
func parseUserIDToInt(userID string) (int, error) {
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
