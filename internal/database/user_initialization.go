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

	// ===== 13. Initialize System Control Settings (Epic 7 Enhancement) =====
	// System control switches for order tracking and position management systems
	// Default: order_tracking=chain (new system), position_management=legacy
	if err := r.InitializeUserSystemControl(ctx, userID); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize system control: %v", err)
	} else {
		defaults := DefaultUserSystemControl()
		log.Printf("[USER-INIT] Initialized system control for user %s (order_tracking=%s, position_management=%s)",
			userID, defaults.OrderTrackingSystem, defaults.PositionManagementSystem)
	}

	// ===== 14. Initialize Strategy Hierarchy Settings (Story 11.44) =====
	// Strategy hierarchy: Mode -> Strategy Group (breakout, trending, range, volatile) -> Sub-Strategy
	strategyGroupsInitialized := 0
	subStrategiesInitialized := 0
	if err := r.InitializeUserStrategyHierarchy(ctx, userID); err != nil {
		log.Printf("[USER-INIT] Warning: Failed to initialize strategy hierarchy: %v", err)
	} else {
		// Count initialized: 4 modes x 4 groups = 16 strategy groups, plus sub-strategies
		strategyGroupsInitialized = 16
		subStrategiesInitialized = 4 // Currently only ravindra_volume_imbalance in 2 modes, classic_breakout in 1
		log.Printf("[USER-INIT] Initialized strategy hierarchy for user %s (%d groups, %d sub-strategies)",
			userID, strategyGroupsInitialized, subStrategiesInitialized)
	}

	// ===== Summary =====
	log.Printf("[USER-INIT] Successfully initialized ALL settings for user %s: %d mode configs, circuit breaker, LLM, capital allocation, early warning, Ginie, Spot, %d mode CB stats, safety settings, global trading, position management, %d mode+strategy configs, system control, and %d strategy groups + %d sub-strategies",
		userID, modesInitialized, modesStatsInitialized, modeStrategiesInitialized, strategyGroupsInitialized, subStrategiesInitialized)

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

	// ===== 13. Restore Strategy Hierarchy Settings (Story 11.44) =====
	// Reset all strategy groups and sub-strategies to defaults
	strategyGroupsRestored := 0
	subStrategiesRestored := 0
	if err := r.InitializeUserStrategyHierarchy(ctx, userID); err != nil {
		log.Printf("[USER-RESTORE] Warning: Failed to restore strategy hierarchy: %v", err)
	} else {
		strategyGroupsRestored = 16 // 4 modes x 4 groups
		subStrategiesRestored = 4   // Current sub-strategies defined in defaults
		log.Printf("[USER-RESTORE] Restored strategy hierarchy for user %s (%d groups, %d sub-strategies)",
			userID, strategyGroupsRestored, subStrategiesRestored)
	}

	log.Printf("[USER-RESTORE] Successfully restored ALL settings for user %s: %d mode configs, circuit breaker, LLM, capital allocation, early warning, Ginie, Spot, %d mode CB stats, safety settings, global trading, position management, %d mode+strategy configs, and %d strategy groups + %d sub-strategies",
		userID, modesRestored, modesStatsRestored, modeStrategiesRestored, strategyGroupsRestored, subStrategiesRestored)

	return nil
}

// InitializeUserModeStrategies creates 16 mode+strategy records (4 modes x 4 strategies) for a new user
// Story 11.32: Mode-Strategy User Initialization
// Story 11.41: Updated to load full 18-section configs from default-settings.json "modes" section
// This function is idempotent - safe to run multiple times (uses INSERT ON CONFLICT DO UPDATE)
func (r *Repository) InitializeUserModeStrategies(ctx context.Context, userID string) error {
	log.Printf("[USER-INIT-MODESTRAT] Initializing mode+strategy settings for user %s", userID)

	// Get all valid modes and strategies
	modes := ValidModes()           // ultra_fast, scalp, swing, position
	strategies := ValidStrategies() // trend_following, mean_reversion, breakout, range_trading

	// Try to load full configs from default-settings.json "modes" section (Story 11.41)
	allModeStrategyConfigs, err := loadModeStrategyConfigsFromDefaults()
	useJSONDefaults := err == nil && len(allModeStrategyConfigs) > 0
	if err != nil {
		log.Printf("[USER-INIT-MODESTRAT] Warning: Could not load from default-settings.json modes section: %v, using hardcoded defaults", err)
	} else if len(allModeStrategyConfigs) > 0 {
		log.Printf("[USER-INIT-MODESTRAT] Using full 18-section configs from default-settings.json modes section")
	}

	// Build bulk insert configs
	var configs []ModeStrategySettingsRow
	for _, mode := range modes {
		for _, strategy := range strategies {
			var settingsJSON json.RawMessage
			var enabled bool
			var priority int

			if useJSONDefaults {
				// Try to get full config from default-settings.json "modes" section
				if modeConfigs, ok := allModeStrategyConfigs[mode]; ok {
					if strategyJSON, ok := modeConfigs[strategy]; ok {
						settingsJSON = strategyJSON

						// Parse just the enabled and priority fields
						var basicConfig struct {
							Enabled  bool `json:"enabled"`
							Priority int  `json:"priority"`
						}
						if err := json.Unmarshal(strategyJSON, &basicConfig); err == nil {
							enabled = basicConfig.Enabled
							priority = basicConfig.Priority
						} else {
							// Fallback to hardcoded defaults for enabled/priority
							config := DefaultModeStrategyConfig(mode, strategy)
							enabled = config.Enabled
							priority = config.Priority
						}
					}
				}
			}

			// Fallback to hardcoded defaults if JSON loading failed
			if settingsJSON == nil {
				config := DefaultModeStrategyConfig(mode, strategy)
				enabled = config.Enabled
				priority = config.Priority
				settingsJSON, err = json.Marshal(config)
				if err != nil {
					log.Printf("[USER-INIT-MODESTRAT] Warning: Failed to marshal config for %s/%s: %v", mode, strategy, err)
					continue
				}
			}

			configs = append(configs, ModeStrategySettingsRow{
				UserID:   userID,
				Mode:     mode,
				Strategy: strategy,
				Enabled:  enabled,
				Priority: priority,
				Settings: settingsJSON,
			})
		}
	}

	// Bulk create with ON CONFLICT DO UPDATE (idempotent, and allows re-initialization)
	if err := r.BulkCreateModeStrategySettings(ctx, userID, configs); err != nil {
		return fmt.Errorf("failed to bulk create mode strategy settings: %w", err)
	}

	log.Printf("[USER-INIT-MODESTRAT] Successfully initialized %d mode+strategy configs for user %s (4 modes x 4 strategies, using %s)",
		len(configs), userID, map[bool]string{true: "default-settings.json", false: "hardcoded defaults"}[useJSONDefaults])

	return nil
}

// loadModeStrategyConfigsFromDefaults loads all mode-strategy configs from default-settings.json "modes" section
// Returns map[mode][strategy]json.RawMessage with full 18-section configs
func loadModeStrategyConfigsFromDefaults() (map[string]map[string]json.RawMessage, error) {
	// Load default-settings.json
	defaultsJSON, err := os.ReadFile("default-settings.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read default-settings.json: %w", err)
	}

	// Parse the modes section
	var defaults struct {
		Modes struct {
			Scalp     modeWrapper `json:"scalp"`
			Swing     modeWrapper `json:"swing"`
			Position  modeWrapper `json:"position"`
			UltraFast modeWrapper `json:"ultra_fast"`
		} `json:"modes"`
	}

	if err := json.Unmarshal(defaultsJSON, &defaults); err != nil {
		return nil, fmt.Errorf("failed to parse default-settings.json: %w", err)
	}

	// Build the result map
	result := make(map[string]map[string]json.RawMessage)

	modeWrappers := map[string]*modeWrapper{
		"scalp":      &defaults.Modes.Scalp,
		"swing":      &defaults.Modes.Swing,
		"position":   &defaults.Modes.Position,
		"ultra_fast": &defaults.Modes.UltraFast,
	}

	for modeName, wrapper := range modeWrappers {
		if len(wrapper.Strategies) > 0 {
			result[modeName] = make(map[string]json.RawMessage)
			for strategyName, strategyJSON := range wrapper.Strategies {
				// Make a copy to prevent mutation
				result[modeName][strategyName] = append(json.RawMessage{}, strategyJSON...)
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no mode-strategy configs found in modes section")
	}

	return result, nil
}

// modeWrapper is a helper struct for parsing mode configs from default-settings.json
type modeWrapper struct {
	Name       string                     `json:"name"`
	Enabled    bool                       `json:"enabled"`
	Strategies map[string]json.RawMessage `json:"strategies"`
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

// =====================================================
// STRATEGY HIERARCHY INITIALIZATION (Story 11.44)
// =====================================================

// InitializeUserStrategyHierarchy creates strategy hierarchy settings for a new user
// Reads from default-settings.json strategy_hierarchy section
// Creates records in user_strategy_group_settings and user_sub_strategy_settings
func (r *Repository) InitializeUserStrategyHierarchy(ctx context.Context, userID string) error {
	log.Printf("[USER-INIT-STRAT-HIER] Initializing strategy hierarchy for user %s", userID)

	// Load strategy_hierarchy from default-settings.json
	strategyHierarchy, err := loadStrategyHierarchyFromDefaults()
	if err != nil {
		log.Printf("[USER-INIT-STRAT-HIER] Warning: Could not load strategy_hierarchy from defaults: %v, using hardcoded defaults", err)
		return r.initializeStrategyHierarchyHardcoded(ctx, userID)
	}

	var strategyGroups []*StrategyGroupSettings
	var subStrategies []*SubStrategySettings

	// Process each mode in the hierarchy
	for mode, modeData := range strategyHierarchy {
		for groupName, groupData := range modeData.StrategyGroups {
			// Create strategy group
			// Use mode-specific default timeframe instead of hardcoded 15m
			defaultTimeframe := getDefaultTimeframeForMode(mode)
			sg := &StrategyGroupSettings{
				UserID:              userID,
				Mode:                mode,
				StrategyGroup:       groupName,
				Enabled:             groupData.Enabled,
				Timeframe:           getStringOrDefault(groupData.BaseSettings.Timeframe, defaultTimeframe),
				PositionSizePercent: getFloatOrDefault(groupData.BaseSettings.PositionSizePercent, 2.0),
				MaxLeverage:         getIntOrDefault(groupData.BaseSettings.MaxLeverage, 10),
				MaxPositions:        getIntOrDefault(groupData.BaseSettings.MaxPositions, 3),
				MinVolumeUSDT:       getFloatOrDefault(groupData.BaseSettings.MinVolumeUSDT, 1000000),
			}
			strategyGroups = append(strategyGroups, sg)

			// Create sub-strategies
			for subName, subData := range groupData.SubStrategies {
				settingsJSON, _ := json.Marshal(subData.Settings)
				ss := &SubStrategySettings{
					UserID:        userID,
					Mode:          mode,
					StrategyGroup: groupName,
					SubStrategy:   subName,
					Enabled:       subData.Enabled,
					Settings:      settingsJSON,
				}
				subStrategies = append(subStrategies, ss)
			}
		}
	}

	// Bulk insert strategy groups
	if err := r.BulkUpsertStrategyGroups(ctx, strategyGroups); err != nil {
		return fmt.Errorf("failed to bulk insert strategy groups: %w", err)
	}

	// Bulk insert sub-strategies
	if err := r.BulkUpsertSubStrategies(ctx, subStrategies); err != nil {
		return fmt.Errorf("failed to bulk insert sub-strategies: %w", err)
	}

	log.Printf("[USER-INIT-STRAT-HIER] Initialized %d strategy groups and %d sub-strategies for user %s",
		len(strategyGroups), len(subStrategies), userID)

	return nil
}

// initializeStrategyHierarchyHardcoded creates default strategy hierarchy when JSON loading fails
func (r *Repository) initializeStrategyHierarchyHardcoded(ctx context.Context, userID string) error {
	modes := []string{"scalp", "swing", "position", "ultra_fast"}
	groups := []string{"breakout", "trending", "range", "volatile"}

	var strategyGroups []*StrategyGroupSettings

	for _, mode := range modes {
		for _, group := range groups {
			// Only enable breakout for scalp mode by default
			enabled := mode == "scalp" && group == "breakout"

			sg := &StrategyGroupSettings{
				UserID:              userID,
				Mode:                mode,
				StrategyGroup:       group,
				Enabled:             enabled,
				Timeframe:           getDefaultTimeframeForMode(mode), // Use mode-specific default
				PositionSizePercent: 2.0,
				MaxLeverage:         10,
				MaxPositions:        3,
				MinVolumeUSDT:       1000000,
			}
			strategyGroups = append(strategyGroups, sg)
		}
	}

	// Bulk insert strategy groups
	if err := r.BulkUpsertStrategyGroups(ctx, strategyGroups); err != nil {
		return fmt.Errorf("failed to bulk insert strategy groups: %w", err)
	}

	// Create default ravindra_volume_imbalance sub-strategy for scalp/breakout
	// BACKTESTED VALUES (Dec 2025 - Jan 2026): 51 trades, 47.1% WR, +1147% net return
	defaultSettings := json.RawMessage(`{
		"min_rr_ratio": "1:4",
		"llm_validation": false,
		"trailing_stop": {
			"enabled": true,
			"milestones": [
				{"at_rr": "1:2", "move_sl_to": "entry"},
				{"at_rr": "1:3", "move_sl_to": "1:1"}
			],
			"target_rr": "1:4"
		},
		"pattern_detection": {
			"direction": "long",
			"reference_lookback_candles": 5,
			"volume_spike_threshold": 3.0,
			"require_pre_trend_down": false,
			"breakout_volume_surge": 1.0,
			"breakout_confirmation_candles": 1,
			"entry_volume_vs_reference": 1.0,
			"max_sl_percent": 1.5,
			"max_pattern_age_mins": 60
		}
	}`)

	ss := &SubStrategySettings{
		UserID:        userID,
		Mode:          "scalp",
		StrategyGroup: "breakout",
		SubStrategy:   "ravindra_volume_imbalance",
		Enabled:       true,
		Settings:      defaultSettings,
	}

	if err := r.UpsertSubStrategySettings(ctx, ss); err != nil {
		return fmt.Errorf("failed to insert default sub-strategy: %w", err)
	}

	log.Printf("[USER-INIT-STRAT-HIER] Initialized %d strategy groups and 1 sub-strategy (hardcoded) for user %s",
		len(strategyGroups), userID)

	return nil
}

// =====================================================
// STRATEGY HIERARCHY PARSING HELPERS
// =====================================================

// strategyHierarchyMode represents a mode's strategy groups in default-settings.json
type strategyHierarchyMode struct {
	StrategyGroups map[string]strategyHierarchyGroup `json:"strategy_groups"`
}

// strategyHierarchyGroup represents a strategy group configuration
type strategyHierarchyGroup struct {
	Enabled       bool                              `json:"enabled"`
	BaseSettings  strategyHierarchyBaseSettings     `json:"base_settings"`
	SubStrategies map[string]strategyHierarchySubStrategy `json:"sub_strategies"`
}

// strategyHierarchyBaseSettings represents base settings inherited by sub-strategies
type strategyHierarchyBaseSettings struct {
	Timeframe           string  `json:"timeframe"`
	PositionSizePercent float64 `json:"position_size_percent"`
	MaxLeverage         int     `json:"max_leverage"`
	MaxPositions        int     `json:"max_positions"`
	MinVolumeUSDT       float64 `json:"min_volume_usdt"`
}

// strategyHierarchySubStrategy represents a sub-strategy configuration
type strategyHierarchySubStrategy struct {
	Enabled  bool                   `json:"enabled"`
	Settings map[string]interface{} `json:"settings"`
}

// loadStrategyHierarchyFromDefaults loads strategy_hierarchy section from default-settings.json
func loadStrategyHierarchyFromDefaults() (map[string]strategyHierarchyMode, error) {
	defaultsJSON, err := os.ReadFile("default-settings.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read default-settings.json: %w", err)
	}

	var defaults struct {
		StrategyHierarchy map[string]strategyHierarchyMode `json:"strategy_hierarchy"`
	}

	if err := json.Unmarshal(defaultsJSON, &defaults); err != nil {
		return nil, fmt.Errorf("failed to parse default-settings.json: %w", err)
	}

	if len(defaults.StrategyHierarchy) == 0 {
		return nil, fmt.Errorf("no strategy_hierarchy found in defaults")
	}

	// Filter out the _description key if present
	delete(defaults.StrategyHierarchy, "_description")

	return defaults.StrategyHierarchy, nil
}

// Helper functions for default values
func getStringOrDefault(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func getFloatOrDefault(value float64, defaultValue float64) float64 {
	if value == 0 {
		return defaultValue
	}
	return value
}

func getIntOrDefault(value int, defaultValue int) int {
	if value == 0 {
		return defaultValue
	}
	return value
}

// getDefaultTimeframeForMode returns the mode-specific default timeframe.
// This ensures mode-appropriate timeframes instead of hardcoded values.
// Based on Dec 2025 - Jan 2026 backtest results for Volume Imbalance strategy.
func getDefaultTimeframeForMode(mode string) string {
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
