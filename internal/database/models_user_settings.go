package database

import (
	"fmt"
	"time"
)

// ====== USER-SPECIFIC SETTINGS MODELS ======
// These structs represent per-user configuration tables for the trading bot.
// They mirror the structure from default-settings.json but are stored per-user in the database.
//
// Story 4.13: User Settings Database Tables
// - Each user has their own copy of settings in these tables
// - Settings are initialized from default-settings.json on user creation
// - Users can customize their settings via API without affecting others
// - All runtime settings come from these per-user tables

// ====== LLM CONFIGURATION ======

// UserLLMConfig represents per-user LLM/AI configuration
type UserLLMConfig struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Basic LLM Settings
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"` // deepseek, claude, openai
	Model    string `json:"model"`    // deepseek-chat, claude-3-haiku, gpt-4, etc.

	// Fallback Configuration
	FallbackProvider string `json:"fallback_provider,omitempty"`
	FallbackModel    string `json:"fallback_model,omitempty"`

	// Performance Settings
	TimeoutMs        int `json:"timeout_ms"`         // Request timeout in milliseconds
	RetryCount       int `json:"retry_count"`        // Number of retries on failure
	CacheDurationSec int `json:"cache_duration_sec"` // Cache duration for LLM responses

	// Adaptive AI Learning (optional)
	AdaptiveEnabled      bool `json:"adaptive_enabled"`       // Enable adaptive AI learning
	LearningWindowTrades int  `json:"learning_window_trades"` // Number of trades to analyze
	LearningWindowHours  int  `json:"learning_window_hours"`  // Time window for learning

	// Cost Control
	MaxDailyCost      float64 `json:"max_daily_cost"`       // Max daily LLM API cost in USD
	CostTrackingReset string  `json:"cost_tracking_reset"`  // Time to reset daily cost (e.g., "00:00 UTC")

	// Feature Flags
	UseForEntrySignals bool `json:"use_for_entry_signals"` // Use LLM for entry decisions
	UseForExitSignals  bool `json:"use_for_exit_signals"`  // Use LLM for exit decisions
	UseForAveraging    bool `json:"use_for_averaging"`     // Use LLM for averaging decisions
	UseForHedging      bool `json:"use_for_hedging"`       // Use LLM for hedging decisions

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserLLMConfig returns default LLM configuration from default-settings.json
func DefaultUserLLMConfig() *UserLLMConfig {
	return &UserLLMConfig{
		Enabled:              true,
		Provider:             "deepseek",
		Model:                "deepseek-chat",
		FallbackProvider:     "claude",
		FallbackModel:        "claude-3-haiku",
		TimeoutMs:            5000,
		RetryCount:           2,
		CacheDurationSec:     300,
		AdaptiveEnabled:      false,
		LearningWindowTrades: 100,
		LearningWindowHours:  24,
		MaxDailyCost:         10.0,
		CostTrackingReset:    "00:00 UTC",
		UseForEntrySignals:   true,
		UseForExitSignals:    true,
		UseForAveraging:      true,
		UseForHedging:        false,
	}
}

// ====== GLOBAL TRADING ======

// UserGlobalTrading represents per-user global trading configuration
type UserGlobalTrading struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Risk Management
	RiskLevel        string  `json:"risk_level"`         // low, medium, high, aggressive
	MaxUSDAllocation float64 `json:"max_usd_allocation"` // Maximum USD to allocate for trading

	// Profit Reinvestment
	ProfitReinvestPercent   float64 `json:"profit_reinvest_percent"`    // % of profits to reinvest
	ProfitReinvestRiskLevel string  `json:"profit_reinvest_risk_level"` // Risk level for reinvested profits

	// Timezone Settings
	Timezone       string `json:"timezone"`        // IANA timezone name (e.g., "Asia/Kolkata", "UTC")
	TimezoneOffset string `json:"timezone_offset"` // UTC offset (e.g., "+05:30", "-05:00")

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserGlobalTrading returns default global trading config values
// Values match default-settings.json -> global_trading section
func DefaultUserGlobalTrading() *UserGlobalTrading {
	return &UserGlobalTrading{
		RiskLevel:               "moderate",
		MaxUSDAllocation:        2500.0,
		ProfitReinvestPercent:   50.0,
		ProfitReinvestRiskLevel: "aggressive",
		Timezone:                "Asia/Phnom_Penh",
		TimezoneOffset:          "+07:00",
	}
}

// ====== CAPITAL ALLOCATION ======

// UserCapitalAllocation represents per-user capital allocation across trading modes
type UserCapitalAllocation struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Capital Allocation Percentages (must sum to 100%)
	// NOTE: Only 4 trading modes - scalp_reentry is a Position Optimization method
	UltraFastPercent float64 `json:"ultra_fast_percent"` // e.g., 20%
	ScalpPercent     float64 `json:"scalp_percent"`      // e.g., 30%
	SwingPercent     float64 `json:"swing_percent"`      // e.g., 35%
	PositionPercent  float64 `json:"position_percent"`   // e.g., 15%

	// Max Positions Per Mode
	MaxUltraFastPositions int `json:"max_ultra_fast_positions"` // e.g., 3
	MaxScalpPositions     int `json:"max_scalp_positions"`      // e.g., 10
	MaxSwingPositions     int `json:"max_swing_positions"`      // e.g., 5
	MaxPositionPositions  int `json:"max_position_positions"`   // e.g., 2

	// Max USD Per Position Per Mode
	MaxUltraFastUSDPerPosition float64 `json:"max_ultra_fast_usd_per_position"` // e.g., $200
	MaxScalpUSDPerPosition     float64 `json:"max_scalp_usd_per_position"`      // e.g., $600
	MaxSwingUSDPerPosition     float64 `json:"max_swing_usd_per_position"`      // e.g., $500
	MaxPositionUSDPerPosition  float64 `json:"max_position_usd_per_position"`   // e.g., $600

	// Dynamic Rebalancing
	AllowDynamicRebalance bool    `json:"allow_dynamic_rebalance"` // false = fixed allocation
	RebalanceThresholdPct float64 `json:"rebalance_threshold_pct"` // e.g., 20% drift triggers rebalance

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserCapitalAllocation returns default capital allocation from default-settings.json
func DefaultUserCapitalAllocation() *UserCapitalAllocation {
	return &UserCapitalAllocation{
		UltraFastPercent:              20,
		ScalpPercent:                  30,
		SwingPercent:                  35,
		PositionPercent:            15,
		MaxUltraFastPositions:      3,
		MaxScalpPositions:          10,
		MaxSwingPositions:          5,
		MaxPositionPositions:       2,
		MaxUltraFastUSDPerPosition: 200,
		MaxScalpUSDPerPosition:     600,
		MaxSwingUSDPerPosition:     500,
		MaxPositionUSDPerPosition:  600,
		AllowDynamicRebalance:         false,
		RebalanceThresholdPct:         20,
	}
}

// ====== GLOBAL CIRCUIT BREAKER ======

// UserGlobalCircuitBreaker represents per-user global circuit breaker settings and state
type UserGlobalCircuitBreaker struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Global Circuit Breaker Configuration
	Enabled              bool    `json:"enabled"`
	MaxLossPerHour       float64 `json:"max_loss_per_hour"`       // Max loss per hour in USD
	MaxDailyLoss         float64 `json:"max_daily_loss"`          // Max loss per day in USD
	MaxConsecutiveLosses int     `json:"max_consecutive_losses"`  // Max consecutive losses before pause
	CooldownMinutes      int     `json:"cooldown_minutes"`        // Cooldown period after circuit breaker trips
	MaxTradesPerMinute   int     `json:"max_trades_per_minute"`   // Rate limit per minute
	MaxDailyTrades       int     `json:"max_daily_trades"`        // Max trades per day

	// Win Rate Monitoring
	WinRateCheckAfter int     `json:"win_rate_check_after"` // Check win rate after N trades
	MinWinRate        float64 `json:"min_win_rate"`         // Minimum win rate percentage (e.g., 50.0)

	// Runtime State (tracked by repository_user_circuit_breaker.go)
	IsTripped        bool       `json:"is_tripped"`                   // Whether circuit breaker is currently tripped
	TrippedReason    string     `json:"tripped_reason,omitempty"`     // Reason for tripping
	TrippedAt        *time.Time `json:"tripped_at,omitempty"`         // When the circuit breaker tripped
	ResetAt          *time.Time `json:"reset_at,omitempty"`           // When it was last reset
	HourlyLoss       float64    `json:"hourly_loss"`                  // Current hourly loss tracking
	DailyLoss        float64    `json:"daily_loss"`                   // Current daily loss tracking
	ConsecutiveLosses int       `json:"consecutive_losses"`           // Current consecutive loss count
	LastResetAt      *time.Time `json:"last_reset_at,omitempty"`      // When stats were last reset

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserGlobalCircuitBreaker returns default global circuit breaker from default-settings.json
func DefaultUserGlobalCircuitBreaker() *UserGlobalCircuitBreaker {
	return &UserGlobalCircuitBreaker{
		Enabled:               true,
		MaxLossPerHour:        100,
		MaxDailyLoss:          500,
		MaxConsecutiveLosses:  15,
		CooldownMinutes:       30,
		MaxTradesPerMinute:    10,
		MaxDailyTrades:        1000,
		WinRateCheckAfter:     20,
		MinWinRate:            50.0,
	}
}

// Validate validates UserGlobalCircuitBreaker configuration
func (c *UserGlobalCircuitBreaker) Validate() error {
	if c.MaxLossPerHour < 0 {
		return fmt.Errorf("max_loss_per_hour must be non-negative")
	}
	if c.MaxDailyLoss < 0 {
		return fmt.Errorf("max_daily_loss must be non-negative")
	}
	if c.MaxConsecutiveLosses < 0 {
		return fmt.Errorf("max_consecutive_losses must be non-negative")
	}
	if c.CooldownMinutes < 0 {
		return fmt.Errorf("cooldown_minutes must be non-negative")
	}
	if c.MaxTradesPerMinute < 0 {
		return fmt.Errorf("max_trades_per_minute must be non-negative")
	}
	if c.MaxDailyTrades < 0 {
		return fmt.Errorf("max_daily_trades must be non-negative")
	}
	if c.WinRateCheckAfter < 0 {
		return fmt.Errorf("win_rate_check_after must be non-negative")
	}
	if c.MinWinRate < 0 || c.MinWinRate > 100 {
		return fmt.Errorf("min_win_rate must be between 0 and 100")
	}
	return nil
}

// ====== EARLY WARNING SYSTEM ======

// UserEarlyWarning represents per-user early warning system settings
type UserEarlyWarning struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Early Warning System
	Enabled           bool    `json:"enabled"`
	StartAfterMinutes int     `json:"start_after_minutes"` // Start monitoring after N minutes
	CheckIntervalSecs int     `json:"check_interval_secs"` // Check interval in seconds
	OnlyUnderwater    bool    `json:"only_underwater"`     // Only check positions in loss
	MinLossPercent    float64 `json:"min_loss_percent"`    // Minimum loss % to trigger warning
	CloseOnReversal   bool    `json:"close_on_reversal"`   // Auto-close on reversal detection

	// Extended Early Warning Fields (Story 9.4 Phase 4)
	TightenSLOnWarning      bool    `json:"tighten_sl_on_warning"`       // Tighten SL if warning detected (default: true)
	MinConfidence           float64 `json:"min_confidence"`              // Min LLM confidence to act (default: 0.7)
	MaxLLMCallsPerPos       int     `json:"max_llm_calls_per_pos"`       // Max LLM calls per position per cycle (default: 3)
	CloseMinHoldMins        int     `json:"close_min_hold_mins"`         // Min hold time before close_now allowed (default: 15)
	CloseMinConfidence      float64 `json:"close_min_confidence"`        // Higher confidence for close_now action (default: 0.85)
	CloseRequireConsecutive int     `json:"close_require_consecutive"`   // Require X consecutive warnings before close (default: 2)
	CloseSLProximityPct     int     `json:"close_sl_proximity_pct"`      // Only close if within X% of SL distance (default: 50)

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserEarlyWarning returns default early warning settings from default-settings.json
func DefaultUserEarlyWarning() *UserEarlyWarning {
	return &UserEarlyWarning{
		Enabled:           true,
		StartAfterMinutes: 1,
		CheckIntervalSecs: 30,
		OnlyUnderwater:    true,
		MinLossPercent:    0.3,
		CloseOnReversal:   true,
		// Extended fields (Story 9.4 Phase 4)
		TightenSLOnWarning:      true,
		MinConfidence:           0.7,
		MaxLLMCallsPerPos:       3,
		CloseMinHoldMins:        15,
		CloseMinConfidence:      0.85,
		CloseRequireConsecutive: 2,
		CloseSLProximityPct:     50,
	}
}

// ====== GINIE AUTOPILOT SETTINGS ======

// UserGinieSettings represents per-user Ginie autopilot configuration
// Matches migration 017_user_ginie_settings.sql + 043_user_ginie_morning_auto_block.sql
type UserGinieSettings struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Global settings
	DryRunMode   bool `json:"dry_run_mode"`   // Paper trading mode
	AutoStart    bool `json:"auto_start"`     // Auto-start on server restart
	MaxPositions int  `json:"max_positions"`  // Max concurrent positions (1-100)

	// Auto mode settings (LLM-driven trading)
	AutoModeEnabled         bool    `json:"auto_mode_enabled"`           // Enable LLM auto trading
	AutoModeMaxPositions    int     `json:"auto_mode_max_positions"`     // Max positions in auto mode (1-50)
	AutoModeMaxLeverage     int     `json:"auto_mode_max_leverage"`      // Max leverage (1-125)
	AutoModeMaxPositionSize float64 `json:"auto_mode_max_position_size"` // Max position size USD ($10-$100k)
	AutoModeMaxTotalUSD     float64 `json:"auto_mode_max_total_usd"`     // Max total USD ($10-$1M)
	AutoModeAllowAveraging  bool    `json:"auto_mode_allow_averaging"`   // Allow averaging in auto mode
	AutoModeMaxAverages     int     `json:"auto_mode_max_averages"`      // Max averages (1-10)
	AutoModeMinHoldMinutes  int     `json:"auto_mode_min_hold_minutes"`  // Min hold time (1-1440)
	AutoModeQuickProfitMode bool    `json:"auto_mode_quick_profit_mode"` // Quick profit mode
	AutoModeMinProfitExit   float64 `json:"auto_mode_min_profit_exit"`   // Min profit % to exit (0.1-20%)

	// Morning auto-block settings (Story 9.12 Phase 4)
	MorningAutoBlockEnabled bool `json:"morning_auto_block_enabled"` // Enable morning auto-block (default: false)
	MorningAutoBlockHourUTC int  `json:"morning_auto_block_hour_utc"` // Hour in UTC (0-23, default: 0)
	MorningAutoBlockMinUTC  int  `json:"morning_auto_block_min_utc"`  // Minute in UTC (0-59, default: 5)

	// PnL statistics (persisted)
	TotalPnL      float64    `json:"total_pnl"`       // Lifetime realized PnL USD
	DailyPnL      float64    `json:"daily_pnl"`       // Today's realized PnL USD
	TotalTrades   int        `json:"total_trades"`    // Lifetime trade count
	WinningTrades int        `json:"winning_trades"`  // Lifetime winning trades
	DailyTrades   int        `json:"daily_trades"`    // Today's trade count
	PnLLastUpdate *time.Time `json:"pnl_last_update"` // Last PnL update timestamp

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserGinieSettings returns default Ginie settings matching migration defaults
func DefaultUserGinieSettings() *UserGinieSettings {
	return &UserGinieSettings{
		DryRunMode:              false,
		AutoStart:               false,
		MaxPositions:            10,
		AutoModeEnabled:         false,
		AutoModeMaxPositions:    5,
		AutoModeMaxLeverage:     10,
		AutoModeMaxPositionSize: 1000.0,
		AutoModeMaxTotalUSD:     5000.0,
		AutoModeAllowAveraging:  true,
		AutoModeMaxAverages:     3,
		AutoModeMinHoldMinutes:  5,
		AutoModeQuickProfitMode: false,
		AutoModeMinProfitExit:   1.5,
		// Morning auto-block defaults (Story 9.12 Phase 4)
		MorningAutoBlockEnabled: false,
		MorningAutoBlockHourUTC: 0,
		MorningAutoBlockMinUTC:  5,
		// PnL statistics
		TotalPnL:                0,
		DailyPnL:                0,
		TotalTrades:             0,
		WinningTrades:           0,
		DailyTrades:             0,
		PnLLastUpdate:           nil,
	}
}

// ====== SPOT TRADING SETTINGS ======

// UserSpotSettings represents per-user spot trading configuration
type UserSpotSettings struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Core spot settings
	Enabled            bool    `json:"enabled"`
	DryRunMode         bool    `json:"dry_run_mode"`
	RiskLevel          string  `json:"risk_level"`
	MaxPositions       int     `json:"max_positions"`
	MaxUSDPerPosition  float64 `json:"max_usd_per_position"`
	TakeProfitPercent  float64 `json:"take_profit_percent"`
	StopLossPercent    float64 `json:"stop_loss_percent"`
	MinConfidence      float64 `json:"min_confidence"`

	// Circuit breaker settings
	CircuitBreakerEnabled   bool    `json:"circuit_breaker_enabled"`
	CBMaxLossPerHour        float64 `json:"cb_max_loss_per_hour"`
	CBMaxDailyLoss          float64 `json:"cb_max_daily_loss"`
	CBMaxConsecutiveLosses  int     `json:"cb_max_consecutive_losses"`
	CBCooldownMinutes       int     `json:"cb_cooldown_minutes"`
	CBMaxTradesPerMinute    int     `json:"cb_max_trades_per_minute"`
	CBMaxDailyTrades        int     `json:"cb_max_daily_trades"`

	// Coin preferences
	CoinBlacklist []string `json:"coin_blacklist"`
	CoinWhitelist []string `json:"coin_whitelist"`
	UseWhitelist  bool     `json:"use_whitelist"`

	// PnL statistics
	TotalPnL      float64   `json:"total_pnl"`
	DailyPnL      float64   `json:"daily_pnl"`
	TotalTrades   int       `json:"total_trades"`
	WinningTrades int       `json:"winning_trades"`
	DailyTrades   int       `json:"daily_trades"`
	PnLLastUpdate time.Time `json:"pnl_last_update"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserSpotSettings returns default spot trading settings matching migration defaults
func DefaultUserSpotSettings() *UserSpotSettings {
	return &UserSpotSettings{
		Enabled:                 false,
		DryRunMode:              false,
		RiskLevel:               "moderate",
		MaxPositions:            5,
		MaxUSDPerPosition:       500.0,
		TakeProfitPercent:       3.0,
		StopLossPercent:         2.0,
		MinConfidence:           70.0,
		CircuitBreakerEnabled:   true,
		CBMaxLossPerHour:        50.0,
		CBMaxDailyLoss:          200.0,
		CBMaxConsecutiveLosses:  5,
		CBCooldownMinutes:       30,
		CBMaxTradesPerMinute:    5,
		CBMaxDailyTrades:        50,
		CoinBlacklist:           []string{},
		CoinWhitelist:           []string{},
		UseWhitelist:            false,
		TotalPnL:                0.0,
		DailyPnL:                0.0,
		TotalTrades:             0,
		WinningTrades:           0,
		DailyTrades:             0,
	}
}

// ====== MODE-SPECIFIC CIRCUIT BREAKER STATS ======

// UserModeCBStats represents per-user per-mode circuit breaker statistics
// This table tracks runtime stats for circuit breaker enforcement
// IMPORTANT: Field names MUST match migration 019_user_mode_circuit_breaker_stats.sql
type UserModeCBStats struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	ModeName string `json:"mode_name"` // ultra_fast, scalp, scalp_reentry, swing, position

	// Trade Counters
	TradesThisMinute  int `json:"trades_this_minute"`
	TradesThisHour    int `json:"trades_this_hour"`
	TradesThisDay     int `json:"trades_this_day"`
	TotalTrades       int `json:"total_trades"`
	TotalWins         int `json:"total_wins"`
	ConsecutiveLosses int `json:"consecutive_losses"`

	// Loss Tracking
	CurrentHourLoss float64 `json:"current_hour_loss"`
	CurrentDayLoss  float64 `json:"current_day_loss"`

	// Pause State (replaces IsTripped/TripReason)
	IsPaused    bool      `json:"is_paused"`
	PausedUntil time.Time `json:"paused_until,omitempty"`
	PauseReason string    `json:"pause_reason,omitempty"`

	// Timestamps for time-based resets
	LastMinuteReset time.Time `json:"last_minute_reset"`
	LastHourReset   time.Time `json:"last_hour_reset"`
	LastDayReset    time.Time `json:"last_day_reset"` // DATE type in DB

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserModeCBStats returns default circuit breaker stats (all zeros)
// Takes userID and modeName as parameters to initialize the struct
func DefaultUserModeCBStats(userID, modeName string) *UserModeCBStats {
	now := time.Now()
	return &UserModeCBStats{
		UserID:            userID,
		ModeName:          modeName,
		TradesThisMinute:  0,
		TradesThisHour:    0,
		TradesThisDay:     0,
		TotalTrades:       0,
		TotalWins:         0,
		ConsecutiveLosses: 0,
		CurrentHourLoss:   0,
		CurrentDayLoss:    0,
		IsPaused:          false,
		LastMinuteReset:   now,
		LastHourReset:     now,
		LastDayReset:      now,
	}
}

// ====== POSITION MANAGEMENT SETTINGS ======

// UserPositionManagement represents per-user position management configuration
// Story 10.1: Position Decision Configuration
type UserPositionManagement struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Decision Mode: "classic" or "new_engine"
	DecisionMode string `json:"decision_mode"`

	// Classic Decision Engine Settings
	ClassicADXReversalThreshold  float64 `json:"classic_adx_reversal_threshold"`
	ClassicEMAReversalPeriods    []int   `json:"classic_ema_reversal_periods"`
	ClassicRSIOverbought         float64 `json:"classic_rsi_overbought"`
	ClassicRSIOversold           float64 `json:"classic_rsi_oversold"`
	ClassicReversalConfirmations int     `json:"classic_reversal_confirmations"`

	// New Engine Settings
	NewEngineUseActiveStrategy    bool   `json:"new_engine_use_active_strategy"`
	NewEngineStrategyName         string `json:"new_engine_strategy_name"`
	NewEngineUseStrategyExitRules bool   `json:"new_engine_use_strategy_exit_rules"`
	NewEngineExitOnRegimeChange   bool   `json:"new_engine_exit_on_regime_change"`

	// Efficiency Exit Settings
	EfficiencyExitEnabled                    bool `json:"efficiency_exit_enabled"`
	EfficiencyExitHistoricalWindowHours      int  `json:"efficiency_exit_historical_window_hours"`
	EfficiencyExitMinimumHoldMinutes         int  `json:"efficiency_exit_minimum_hold_minutes"`
	EfficiencyExitConsecutiveSignalsRequired int  `json:"efficiency_exit_consecutive_signals_required"`

	// Dynamic SL/TP Settings
	DynamicSLTPEnabled        bool    `json:"dynamic_sltp_enabled"`
	DynamicSLTPSLATRMultiplier float64 `json:"dynamic_sltp_sl_atr_multiplier"`
	DynamicSLTPTPATRMultiplier float64 `json:"dynamic_sltp_tp_atr_multiplier"`
	DynamicSLTPUpdateOnBinance bool    `json:"dynamic_sltp_update_on_binance"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserPositionManagement returns default position management settings
// Values match default-settings.json -> position_management section
func DefaultUserPositionManagement() *UserPositionManagement {
	return &UserPositionManagement{
		DecisionMode:                             "classic",
		ClassicADXReversalThreshold:              20,
		ClassicEMAReversalPeriods:                []int{9, 21},
		ClassicRSIOverbought:                     70,
		ClassicRSIOversold:                       30,
		ClassicReversalConfirmations:             2,
		NewEngineUseActiveStrategy:               true,
		NewEngineStrategyName:                    "",
		NewEngineUseStrategyExitRules:            true,
		NewEngineExitOnRegimeChange:              true,
		EfficiencyExitEnabled:                    true,
		EfficiencyExitHistoricalWindowHours:      6,
		EfficiencyExitMinimumHoldMinutes:         2,
		EfficiencyExitConsecutiveSignalsRequired: 3,
		DynamicSLTPEnabled:                       true,
		DynamicSLTPSLATRMultiplier:               1.5,
		DynamicSLTPTPATRMultiplier:               3.0,
		DynamicSLTPUpdateOnBinance:               true,
	}
}

// ====== MODE-STRATEGY CONFIGURATION (Story 11.28) ======
// New hierarchical structure: modes.{mode}.strategies.{strategy}
// Each mode has 4 strategies with independent settings.
// This replaces the flat mode_configs structure.

// ModeStrategyConfig represents a complete strategy configuration within a mode
// Path: modes.{mode}.strategies.{strategy}
// Story 11.41: Expanded to include all 18 configuration sections per strategy
type ModeStrategyConfig struct {
	// Strategy identification
	Enabled          bool     `json:"enabled"`
	Priority         int      `json:"priority"`
	SupportedRegimes []string `json:"supported_regimes"`

	// 1. Position sizing (replaces top-level leverage, max_positions, base_size_usd)
	PositionSizing *StrategyPositionSizing `json:"position_sizing,omitempty"`
	// Legacy fields for backward compatibility (deprecated, use PositionSizing)
	Leverage     int     `json:"leverage,omitempty"`
	MaxPositions int     `json:"max_positions,omitempty"`
	BaseSizeUSD  float64 `json:"base_size_usd,omitempty"`

	// 2. Timeframes
	Timeframe StrategyTimeframe `json:"timeframe"`

	// 3. Multi-timeframe analysis
	MTF *StrategyMTF `json:"mtf,omitempty"`

	// 4. SL/TP configuration (expanded)
	SLTP StrategySLTP `json:"sltp"`

	// 5. Confidence thresholds
	Confidence StrategyConfidence `json:"confidence"`

	// 6. Entry conditions (expanded, can also use legacy map)
	EntryConditions     map[string]interface{}   `json:"entry_conditions,omitempty"`
	EntryConditionsV2   *StrategyEntryConditions `json:"entry_conditions_v2,omitempty"`

	// 7. Exit conditions (expanded)
	ExitConditions StrategyExitConditions `json:"exit_conditions"`

	// 8. Scoring weights (expanded)
	Scoring StrategyScoring `json:"scoring"`

	// 9. Per-strategy circuit breaker
	CircuitBreaker *StrategyCircuitBreaker `json:"circuit_breaker,omitempty"`

	// 10. Hedging configuration
	Hedge *StrategyHedge `json:"hedge,omitempty"`

	// 11. Position averaging
	Averaging *StrategyAveraging `json:"averaging,omitempty"`

	// 12. Stale position release
	StaleRelease *StrategyStaleRelease `json:"stale_release,omitempty"`

	// 13. Position optimization (re-entry, dynamic SL, profit protection)
	PositionOptimization *StrategyPositionOptimization `json:"position_optimization,omitempty"`

	// 14. Funding rate management
	FundingRate *StrategyFundingRate `json:"funding_rate,omitempty"`

	// 15. Risk management
	Risk *StrategyRisk `json:"risk,omitempty"`

	// 16. Trend divergence detection
	TrendDivergence *StrategyTrendDivergence `json:"trend_divergence,omitempty"`

	// 17. Dynamic AI exit
	DynamicAIExit *StrategyDynamicAIExit `json:"dynamic_ai_exit,omitempty"`

	// 18. Per-strategy early warning
	EarlyWarning *StrategyEarlyWarning `json:"early_warning,omitempty"`
}

// StrategyTimeframe contains timeframe settings for a strategy
type StrategyTimeframe struct {
	TrendTimeframe    string `json:"trend_timeframe"`
	EntryTimeframe    string `json:"entry_timeframe"`
	AnalysisTimeframe string `json:"analysis_timeframe"`
}

// StrategySLTP contains stop-loss and take-profit settings (expanded for Story 11.41)
type StrategySLTP struct {
	SLPercent              float64 `json:"sl_percent"`
	TP1Percent             float64 `json:"tp1_percent"`
	TP1SellPercent         int     `json:"tp1_sell_percent,omitempty"`
	TP2Percent             float64 `json:"tp2_percent"`
	TP2SellPercent         int     `json:"tp2_sell_percent,omitempty"`
	TP3Percent             float64 `json:"tp3_percent"`
	TP3SellPercent         int     `json:"tp3_sell_percent,omitempty"`
	TrailingEnabled        bool    `json:"trailing_enabled"`
	TrailingActivationPct  float64 `json:"trailing_activation_pct"`
	TrailingStopPct        float64 `json:"trailing_stop_pct"`
	UseATRBased            bool    `json:"use_atr_based,omitempty"`
	ATRSLMultiplier        float64 `json:"atr_sl_multiplier,omitempty"`
	ATRTPMultiplier        float64 `json:"atr_tp_multiplier,omitempty"`
	MinSLDistancePct       float64 `json:"min_sl_distance_pct,omitempty"`
}

// StrategyPositionSizing contains position sizing configuration (Story 11.41)
type StrategyPositionSizing struct {
	Leverage            int     `json:"leverage"`
	MaxPositions        int     `json:"max_positions"`
	BaseSizeUSD         float64 `json:"base_size_usd"`
	MaxSizeUSD          float64 `json:"max_size_usd,omitempty"`
	MinPositionSizeUSD  float64 `json:"min_position_size_usd,omitempty"`
	SafetyMargin        float64 `json:"safety_margin,omitempty"`
	AutoSizeEnabled     bool    `json:"auto_size_enabled,omitempty"`
	AutoSizeMinCoverFee float64 `json:"auto_size_min_cover_fee,omitempty"`
}

// StrategyMTF contains multi-timeframe analysis settings (Story 11.41)
type StrategyMTF struct {
	Enabled             bool    `json:"enabled"`
	PrimaryTimeframe    string  `json:"primary_timeframe"`
	PrimaryWeight       float64 `json:"primary_weight"`
	SecondaryTimeframe  string  `json:"secondary_timeframe"`
	SecondaryWeight     float64 `json:"secondary_weight"`
	TertiaryTimeframe   string  `json:"tertiary_timeframe"`
	TertiaryWeight      float64 `json:"tertiary_weight"`
	MinConsensus        int     `json:"min_consensus"`
	MinWeightedStrength float64 `json:"min_weighted_strength"`
	TrendStabilityCheck bool    `json:"trend_stability_check"`
}

// StrategyCircuitBreaker contains per-strategy circuit breaker settings (Story 11.41)
type StrategyCircuitBreaker struct {
	MaxLossPerHourUSD    float64 `json:"max_loss_per_hour_usd"`
	MaxLossPerDayUSD     float64 `json:"max_loss_per_day_usd"`
	MaxConsecutiveLosses int     `json:"max_consecutive_losses"`
	CooldownMinutes      int     `json:"cooldown_minutes"`
	MaxTradesPerHour     int     `json:"max_trades_per_hour"`
	MaxTradesPerDay      int     `json:"max_trades_per_day"`
	WinRateCheckAfter    int     `json:"win_rate_check_after"`
	MinWinRatePct        float64 `json:"min_win_rate_pct"`
}

// StrategyHedge contains hedging configuration (Story 11.41)
type StrategyHedge struct {
	AllowHedge                  bool    `json:"allow_hedge"`
	MinConfidenceForHedge       int     `json:"min_confidence_for_hedge"`
	ExistingMustBeInProfitPct   float64 `json:"existing_must_be_in_profit_pct"`
	MaxHedgeSizePercent         int     `json:"max_hedge_size_percent"`
	AllowSameModeHedge          bool    `json:"allow_same_mode_hedge"`
	MaxTotalExposureMultiplier  float64 `json:"max_total_exposure_multiplier"`
}

// StrategyAveraging contains position averaging configuration (Story 11.41)
type StrategyAveraging struct {
	AllowAveraging          bool    `json:"allow_averaging"`
	AverageUpProfitPercent  float64 `json:"average_up_profit_percent"`
	AverageDownLossPercent  float64 `json:"average_down_loss_percent"`
	AddSizePercent          int     `json:"add_size_percent"`
	MaxAverages             int     `json:"max_averages"`
	MinConfidenceForAverage int     `json:"min_confidence_for_average"`
	UseLLMForAveraging      bool    `json:"use_llm_for_averaging"`
	StagedEntryEnabled      bool    `json:"staged_entry_enabled"`
	StagedEntryLevels       int     `json:"staged_entry_levels"`
	StagedEntryPercent      []int   `json:"staged_entry_percent,omitempty"`
}

// StrategyStaleRelease contains stale position release settings (Story 11.41)
type StrategyStaleRelease struct {
	Enabled                 bool    `json:"enabled"`
	MaxHoldDurationMinutes  int     `json:"max_hold_duration_minutes"`
	MinProfitToKeepPct      float64 `json:"min_profit_to_keep_pct"`
	MaxLossToForceClosePct  float64 `json:"max_loss_to_force_close_pct"`
	StaleZoneLoPct          float64 `json:"stale_zone_lo_pct"`
	StaleZoneHiPct          float64 `json:"stale_zone_hi_pct"`
	StaleZoneAction         string  `json:"stale_zone_action"`
}

// StrategyPositionOptimization contains position optimization settings (Story 11.41)
type StrategyPositionOptimization struct {
	ReentryEnabled             bool    `json:"reentry_enabled"`
	ReentryAfterTP1            bool    `json:"reentry_after_tp1"`
	ReentryMinPullbackPct      float64 `json:"reentry_min_pullback_pct"`
	MaxReentriesPerPosition    int     `json:"max_reentries_per_position"`
	DynamicSLEnabled           bool    `json:"dynamic_sl_enabled"`
	DynamicSLAtBreakevenPct    float64 `json:"dynamic_sl_at_breakeven_pct"`
	ProfitProtectionEnabled    bool    `json:"profit_protection_enabled"`
	ProfitProtectionTriggerPct float64 `json:"profit_protection_trigger_pct"`
	ProfitProtectionLockPct    int     `json:"profit_protection_lock_pct"`
}

// StrategyFundingRate contains funding rate settings (Story 11.41)
type StrategyFundingRate struct {
	Enabled                  bool    `json:"enabled"`
	MaxFundingRatePct        float64 `json:"max_funding_rate_pct"`
	ExitBeforeFundingMinutes int     `json:"exit_before_funding_minutes"`
	BlockEntryAboveRatePct   float64 `json:"block_entry_above_rate_pct"`
}

// StrategyRisk contains risk management settings (Story 11.41)
type StrategyRisk struct {
	RiskLevel            string  `json:"risk_level"`
	MaxDrawdownPercent   float64 `json:"max_drawdown_percent"`
	MaxDailyLossPercent  float64 `json:"max_daily_loss_percent"`
	PositionRiskPercent  float64 `json:"position_risk_percent"`
}

// StrategyTrendDivergence contains trend divergence detection settings (Story 11.41)
type StrategyTrendDivergence struct {
	Enabled              bool    `json:"enabled"`
	MinDivergencePercent int     `json:"min_divergence_percent"`
	BlockOnDivergence    bool    `json:"block_on_divergence"`
	DivergenceWeight     float64 `json:"divergence_weight"`
}

// StrategyDynamicAIExit contains dynamic AI-based exit settings (Story 11.41)
type StrategyDynamicAIExit struct {
	Enabled            bool  `json:"enabled"`
	MinHoldBeforeAIMs  int64 `json:"min_hold_before_ai_ms"`
	AICheckIntervalMs  int64 `json:"ai_check_interval_ms"`
	UseLLMForLoss      bool  `json:"use_llm_for_loss"`
	UseLLMForProfit    bool  `json:"use_llm_for_profit"`
	MaxHoldTimeMs      int64 `json:"max_hold_time_ms"`
}

// StrategyEarlyWarning contains per-strategy early warning settings (Story 11.41)
type StrategyEarlyWarning struct {
	Enabled            bool    `json:"enabled"`
	StartAfterMinutes  int     `json:"start_after_minutes"`
	MinLossPercent     float64 `json:"min_loss_percent"`
	CheckIntervalSecs  int     `json:"check_interval_secs"`
	CloseMinHoldMins   int     `json:"close_min_hold_mins"`
}

// StrategyEntryConditions contains expanded entry condition settings (Story 11.41)
type StrategyEntryConditions struct {
	ADXMin               float64 `json:"adx_min,omitempty"`
	ADXMax               float64 `json:"adx_max,omitempty"`
	RSIMin               int     `json:"rsi_min,omitempty"`
	RSIMax               int     `json:"rsi_max,omitempty"`
	RequireTrendAlign    bool    `json:"require_trend_align,omitempty"`
	MinVolumeMultiplier  float64 `json:"min_volume_multiplier,omitempty"`
	UseLimitEntry        bool    `json:"use_limit_entry,omitempty"`
	LimitOrderGapPercent float64 `json:"limit_order_gap_percent,omitempty"`
	MaxLimitGapPercent   float64 `json:"max_limit_gap_percent,omitempty"`
	// Strategy-specific fields (mean_reversion, breakout, range_trading)
	RSIOversold            int     `json:"rsi_oversold,omitempty"`
	RSIOverbought          int     `json:"rsi_overbought,omitempty"`
	BollingerStd           float64 `json:"bollinger_std,omitempty"`
	RequirePriceAtBand     bool    `json:"require_price_at_band,omitempty"`
	BreakoutATRMultiplier  float64 `json:"breakout_atr_multiplier,omitempty"`
	VolumeSpikeMultiplier  float64 `json:"volume_spike_multiplier,omitempty"`
	RequireConsolidation   bool    `json:"require_consolidation,omitempty"`
	ConsolidationBars      int     `json:"consolidation_bars,omitempty"`
	RangeHighTouch         bool    `json:"range_high_touch,omitempty"`
	RangeLowTouch          bool    `json:"range_low_touch,omitempty"`
	RangeWidthATR          float64 `json:"range_width_atr,omitempty"`
	MinRangeDurationBars   int     `json:"min_range_duration_bars,omitempty"`
}

// StrategyConfidence contains confidence thresholds
type StrategyConfidence struct {
	MinConfidence   int `json:"min_confidence"`
	HighConfidence  int `json:"high_confidence"`
	UltraConfidence int `json:"ultra_confidence"`
}

// StrategyExitConditions contains exit rule settings (expanded for Story 11.41)
type StrategyExitConditions struct {
	UseAIExit            bool `json:"use_ai_exit,omitempty"`
	ExitAtMean           bool `json:"exit_at_mean,omitempty"`
	ExitAtRangeBoundary  bool `json:"exit_at_range_boundary,omitempty"`
	MaxHoldMinutes       int  `json:"max_hold_minutes"`
	EarlyWarningEnabled  bool `json:"early_warning_enabled"`
	ExitOnTrendReversal  bool `json:"exit_on_trend_reversal,omitempty"`
	ADXExitThreshold     int  `json:"adx_exit_threshold,omitempty"`
}

// StrategyScoring contains scoring weight configuration (expanded for Story 11.41)
type StrategyScoring struct {
	TechnicalWeight int `json:"technical_weight"`
	MomentumWeight  int `json:"momentum_weight"`
	VolumeWeight    int `json:"volume_weight"`
	SentimentWeight int `json:"sentiment_weight"`
	MinScore        int `json:"min_score,omitempty"`
	HighScore       int `json:"high_score,omitempty"`
}

// ModeConfig represents a complete mode configuration with all strategies
// Path: modes.{mode}
type ModeConfig struct {
	Name               string                        `json:"name"`
	Enabled            bool                          `json:"enabled"`
	DefaultStrategy    string                        `json:"default_strategy"`
	AutoSelectStrategy bool                          `json:"auto_select_strategy"`
	Strategies         map[string]ModeStrategyConfig `json:"strategies"`
}

// ModesConfig is the top-level container for all modes
// Path: modes
type ModesConfig struct {
	Description string                `json:"_description,omitempty"`
	Version     string                `json:"_version,omitempty"`
	Scalp       ModeConfig            `json:"scalp"`
	Swing       ModeConfig            `json:"swing"`
	Position    ModeConfig            `json:"position"`
	UltraFast   ModeConfig            `json:"ultra_fast"`
}

// DefaultModeStrategyConfig returns default strategy config for a given mode and strategy
func DefaultModeStrategyConfig(modeName, strategyName string) *ModeStrategyConfig {
	// Base configuration that varies by mode and strategy
	configs := map[string]map[string]*ModeStrategyConfig{
		"scalp": {
			"trend_following": {
				Enabled:          true,
				Priority:         1,
				SupportedRegimes: []string{"TRENDING", "VOLATILE_TRENDING"},
				Leverage:         10,
				MaxPositions:     10,
				BaseSizeUSD:      200,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "15m",
					EntryTimeframe:    "5m",
					AnalysisTimeframe: "15m",
				},
				SLTP: StrategySLTP{
					SLPercent:              2.0,
					TP1Percent:             0.5,
					TP2Percent:             1.0,
					TP3Percent:             1.5,
					TrailingEnabled:        true,
					TrailingActivationPct:  0.5,
					TrailingStopPct:        0.3,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   55,
					HighConfidence:  75,
					UltraConfidence: 85,
				},
				ExitConditions: StrategyExitConditions{
					UseAIExit:           true,
					MaxHoldMinutes:      240,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 40,
					MomentumWeight:  30,
					VolumeWeight:    15,
					SentimentWeight: 15,
				},
			},
			"mean_reversion": {
				Enabled:          false,
				Priority:         2,
				SupportedRegimes: []string{"RANGING", "MEAN_REVERTING"},
				Leverage:         8,
				MaxPositions:     8,
				BaseSizeUSD:      150,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "15m",
					EntryTimeframe:    "5m",
					AnalysisTimeframe: "1h",
				},
				SLTP: StrategySLTP{
					SLPercent:  1.5,
					TP1Percent: 0.3,
					TP2Percent: 0.6,
					TP3Percent: 1.0,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   60,
					HighConfidence:  80,
					UltraConfidence: 90,
				},
				ExitConditions: StrategyExitConditions{
					ExitAtMean:          true,
					MaxHoldMinutes:      120,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 50,
					MomentumWeight:  20,
					VolumeWeight:    20,
					SentimentWeight: 10,
				},
			},
			"breakout": {
				Enabled:          false,
				Priority:         3,
				SupportedRegimes: []string{"BREAKOUT", "VOLATILE_TRENDING"},
				Leverage:         10,
				MaxPositions:     6,
				BaseSizeUSD:      200,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "3m",
					EntryTimeframe:    "3m",
					AnalysisTimeframe: "3m",
				},
				SLTP: StrategySLTP{
					SLPercent:              1.5,
					TP1Percent:             0.8,
					TP2Percent:             1.5,
					TP3Percent:             2.5,
					TrailingEnabled:        true,
					TrailingActivationPct:  0.8,
					TrailingStopPct:        0.4,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   65,
					HighConfidence:  80,
					UltraConfidence: 90,
				},
				ExitConditions: StrategyExitConditions{
					UseAIExit:           true,
					MaxHoldMinutes:      180,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 35,
					MomentumWeight:  35,
					VolumeWeight:    20,
					SentimentWeight: 10,
				},
			},
			"range_trading": {
				Enabled:          false,
				Priority:         4,
				SupportedRegimes: []string{"RANGING", "LOW_VOLATILITY"},
				Leverage:         6,
				MaxPositions:     5,
				BaseSizeUSD:      100,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "1h",
					EntryTimeframe:    "15m",
					AnalysisTimeframe: "4h",
				},
				SLTP: StrategySLTP{
					SLPercent:  1.0,
					TP1Percent: 0.3,
					TP2Percent: 0.5,
					TP3Percent: 0.8,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   55,
					HighConfidence:  70,
					UltraConfidence: 85,
				},
				ExitConditions: StrategyExitConditions{
					ExitAtRangeBoundary: true,
					MaxHoldMinutes:      360,
					EarlyWarningEnabled: false,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 45,
					MomentumWeight:  15,
					VolumeWeight:    25,
					SentimentWeight: 15,
				},
			},
		},
		"swing": {
			"trend_following": {
				Enabled:          true,
				Priority:         1,
				SupportedRegimes: []string{"TRENDING", "VOLATILE_TRENDING"},
				Leverage:         10,
				MaxPositions:     5,
				BaseSizeUSD:      300,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "1h",
					EntryTimeframe:    "15m",
					AnalysisTimeframe: "4h",
				},
				SLTP: StrategySLTP{
					SLPercent:  2.0,
					TP1Percent: 1.0,
					TP2Percent: 2.0,
					TP3Percent: 3.0,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   55,
					HighConfidence:  80,
					UltraConfidence: 90,
				},
				ExitConditions: StrategyExitConditions{
					UseAIExit:           true,
					MaxHoldMinutes:      4320,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 40,
					MomentumWeight:  30,
					VolumeWeight:    15,
					SentimentWeight: 15,
				},
			},
			"mean_reversion": {
				Enabled:          false,
				Priority:         2,
				SupportedRegimes: []string{"RANGING", "MEAN_REVERTING"},
				Leverage:         8,
				MaxPositions:     4,
				BaseSizeUSD:      250,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "1h",
					EntryTimeframe:    "15m",
					AnalysisTimeframe: "4h",
				},
				SLTP: StrategySLTP{
					SLPercent:  2.0,
					TP1Percent: 0.5,
					TP2Percent: 1.0,
					TP3Percent: 1.5,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   60,
					HighConfidence:  80,
					UltraConfidence: 90,
				},
				ExitConditions: StrategyExitConditions{
					ExitAtMean:          true,
					MaxHoldMinutes:      1440,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 50,
					MomentumWeight:  20,
					VolumeWeight:    20,
					SentimentWeight: 10,
				},
			},
			"breakout": {
				Enabled:          false,
				Priority:         3,
				SupportedRegimes: []string{"BREAKOUT", "VOLATILE_TRENDING"},
				Leverage:         10,
				MaxPositions:     4,
				BaseSizeUSD:      300,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "1h",
					EntryTimeframe:    "15m",
					AnalysisTimeframe: "4h",
				},
				SLTP: StrategySLTP{
					SLPercent:             2.0,
					TP1Percent:            1.5,
					TP2Percent:            2.5,
					TP3Percent:            4.0,
					TrailingEnabled:       true,
					TrailingActivationPct: 1.5,
					TrailingStopPct:       0.8,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   65,
					HighConfidence:  80,
					UltraConfidence: 90,
				},
				ExitConditions: StrategyExitConditions{
					UseAIExit:           true,
					MaxHoldMinutes:      2880,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 35,
					MomentumWeight:  35,
					VolumeWeight:    20,
					SentimentWeight: 10,
				},
			},
			"range_trading": {
				Enabled:          false,
				Priority:         4,
				SupportedRegimes: []string{"RANGING", "LOW_VOLATILITY"},
				Leverage:         6,
				MaxPositions:     3,
				BaseSizeUSD:      200,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "4h",
					EntryTimeframe:    "1h",
					AnalysisTimeframe: "1d",
				},
				SLTP: StrategySLTP{
					SLPercent:  1.5,
					TP1Percent: 0.5,
					TP2Percent: 1.0,
					TP3Percent: 1.5,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   55,
					HighConfidence:  70,
					UltraConfidence: 85,
				},
				ExitConditions: StrategyExitConditions{
					ExitAtRangeBoundary: true,
					MaxHoldMinutes:      1440,
					EarlyWarningEnabled: false,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 45,
					MomentumWeight:  15,
					VolumeWeight:    25,
					SentimentWeight: 15,
				},
			},
		},
		"position": {
			"trend_following": {
				Enabled:          true,
				Priority:         1,
				SupportedRegimes: []string{"TRENDING"},
				Leverage:         3,
				MaxPositions:     2,
				BaseSizeUSD:      600,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "4h",
					EntryTimeframe:    "1h",
					AnalysisTimeframe: "1d",
				},
				SLTP: StrategySLTP{
					SLPercent:  3.5,
					TP1Percent: 2.0,
					TP2Percent: 5.0,
					TP3Percent: 8.0,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   55,
					HighConfidence:  85,
					UltraConfidence: 92,
				},
				ExitConditions: StrategyExitConditions{
					UseAIExit:           true,
					MaxHoldMinutes:      30240,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 40,
					MomentumWeight:  25,
					VolumeWeight:    20,
					SentimentWeight: 15,
				},
			},
			"mean_reversion": {
				Enabled:          false,
				Priority:         2,
				SupportedRegimes: []string{"RANGING", "MEAN_REVERTING"},
				Leverage:         3,
				MaxPositions:     2,
				BaseSizeUSD:      500,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "4h",
					EntryTimeframe:    "1h",
					AnalysisTimeframe: "1d",
				},
				SLTP: StrategySLTP{
					SLPercent:  4.0,
					TP1Percent: 1.5,
					TP2Percent: 3.0,
					TP3Percent: 5.0,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   65,
					HighConfidence:  85,
					UltraConfidence: 92,
				},
				ExitConditions: StrategyExitConditions{
					ExitAtMean:          true,
					MaxHoldMinutes:      10080,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 50,
					MomentumWeight:  20,
					VolumeWeight:    20,
					SentimentWeight: 10,
				},
			},
			"breakout": {
				Enabled:          false,
				Priority:         3,
				SupportedRegimes: []string{"BREAKOUT"},
				Leverage:         3,
				MaxPositions:     2,
				BaseSizeUSD:      600,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "4h",
					EntryTimeframe:    "1h",
					AnalysisTimeframe: "1d",
				},
				SLTP: StrategySLTP{
					SLPercent:             4.0,
					TP1Percent:            3.0,
					TP2Percent:            6.0,
					TP3Percent:            10.0,
					TrailingEnabled:       true,
					TrailingActivationPct: 3.0,
					TrailingStopPct:       1.5,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   70,
					HighConfidence:  85,
					UltraConfidence: 92,
				},
				ExitConditions: StrategyExitConditions{
					UseAIExit:           true,
					MaxHoldMinutes:      20160,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 35,
					MomentumWeight:  35,
					VolumeWeight:    20,
					SentimentWeight: 10,
				},
			},
			"range_trading": {
				Enabled:          false,
				Priority:         4,
				SupportedRegimes: []string{"RANGING", "LOW_VOLATILITY"},
				Leverage:         2,
				MaxPositions:     1,
				BaseSizeUSD:      400,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "1d",
					EntryTimeframe:    "4h",
					AnalysisTimeframe: "1w",
				},
				SLTP: StrategySLTP{
					SLPercent:  3.0,
					TP1Percent: 1.0,
					TP2Percent: 2.0,
					TP3Percent: 3.0,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   60,
					HighConfidence:  80,
					UltraConfidence: 90,
				},
				ExitConditions: StrategyExitConditions{
					ExitAtRangeBoundary: true,
					MaxHoldMinutes:      10080,
					EarlyWarningEnabled: false,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 45,
					MomentumWeight:  15,
					VolumeWeight:    25,
					SentimentWeight: 15,
				},
			},
		},
		"ultra_fast": {
			"trend_following": {
				Enabled:          true,
				Priority:         1,
				SupportedRegimes: []string{"TRENDING", "VOLATILE_TRENDING"},
				Leverage:         10,
				MaxPositions:     1,
				BaseSizeUSD:      200,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "5m",
					EntryTimeframe:    "1m",
					AnalysisTimeframe: "5m",
				},
				SLTP: StrategySLTP{
					SLPercent:  0.5,
					TP1Percent: 0.2,
					TP2Percent: 0.4,
					TP3Percent: 0.8,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   55,
					HighConfidence:  80,
					UltraConfidence: 90,
				},
				ExitConditions: StrategyExitConditions{
					UseAIExit:           true,
					MaxHoldMinutes:      5,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 30,
					MomentumWeight:  40,
					VolumeWeight:    20,
					SentimentWeight: 10,
				},
			},
			"mean_reversion": {
				Enabled:          false,
				Priority:         2,
				SupportedRegimes: []string{"RANGING", "MEAN_REVERTING"},
				Leverage:         8,
				MaxPositions:     1,
				BaseSizeUSD:      150,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "5m",
					EntryTimeframe:    "1m",
					AnalysisTimeframe: "15m",
				},
				SLTP: StrategySLTP{
					SLPercent:  0.4,
					TP1Percent: 0.15,
					TP2Percent: 0.25,
					TP3Percent: 0.4,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   60,
					HighConfidence:  80,
					UltraConfidence: 90,
				},
				ExitConditions: StrategyExitConditions{
					ExitAtMean:          true,
					MaxHoldMinutes:      3,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 50,
					MomentumWeight:  25,
					VolumeWeight:    15,
					SentimentWeight: 10,
				},
			},
			"breakout": {
				Enabled:          false,
				Priority:         3,
				SupportedRegimes: []string{"BREAKOUT", "VOLATILE_TRENDING"},
				Leverage:         10,
				MaxPositions:     1,
				BaseSizeUSD:      200,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "5m",
					EntryTimeframe:    "1m",
					AnalysisTimeframe: "15m",
				},
				SLTP: StrategySLTP{
					SLPercent:             0.4,
					TP1Percent:            0.3,
					TP2Percent:            0.6,
					TP3Percent:            1.0,
					TrailingEnabled:       true,
					TrailingActivationPct: 0.3,
					TrailingStopPct:       0.15,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   65,
					HighConfidence:  80,
					UltraConfidence: 90,
				},
				ExitConditions: StrategyExitConditions{
					UseAIExit:           true,
					MaxHoldMinutes:      5,
					EarlyWarningEnabled: true,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 30,
					MomentumWeight:  40,
					VolumeWeight:    20,
					SentimentWeight: 10,
				},
			},
			"range_trading": {
				Enabled:          false,
				Priority:         4,
				SupportedRegimes: []string{"RANGING", "LOW_VOLATILITY"},
				Leverage:         5,
				MaxPositions:     1,
				BaseSizeUSD:      100,
				Timeframe: StrategyTimeframe{
					TrendTimeframe:    "5m",
					EntryTimeframe:    "1m",
					AnalysisTimeframe: "15m",
				},
				SLTP: StrategySLTP{
					SLPercent:  0.3,
					TP1Percent: 0.1,
					TP2Percent: 0.2,
					TP3Percent: 0.3,
				},
				Confidence: StrategyConfidence{
					MinConfidence:   55,
					HighConfidence:  70,
					UltraConfidence: 85,
				},
				ExitConditions: StrategyExitConditions{
					ExitAtRangeBoundary: true,
					MaxHoldMinutes:      3,
					EarlyWarningEnabled: false,
				},
				Scoring: StrategyScoring{
					TechnicalWeight: 40,
					MomentumWeight:  20,
					VolumeWeight:    25,
					SentimentWeight: 15,
				},
			},
		},
	}

	// Return the config if it exists, otherwise return a default
	if modeConfigs, ok := configs[modeName]; ok {
		if config, ok := modeConfigs[strategyName]; ok {
			applyDefaultNewSections(config)
			return config
		}
	}

	// Return a minimal default config
	config := &ModeStrategyConfig{
		Enabled:          false,
		Priority:         99,
		SupportedRegimes: []string{},
		Leverage:         5,
		MaxPositions:     1,
		BaseSizeUSD:      100,
		Timeframe: StrategyTimeframe{
			TrendTimeframe:    "1h",
			EntryTimeframe:    "15m",
			AnalysisTimeframe: "4h",
		},
		SLTP: StrategySLTP{
			SLPercent:  2.0,
			TP1Percent: 1.0,
			TP2Percent: 2.0,
			TP3Percent: 3.0,
		},
		Confidence: StrategyConfidence{
			MinConfidence:   60,
			HighConfidence:  80,
			UltraConfidence: 90,
		},
		ExitConditions: StrategyExitConditions{
			MaxHoldMinutes:      360,
			EarlyWarningEnabled: true,
		},
		Scoring: StrategyScoring{
			TechnicalWeight: 40,
			MomentumWeight:  30,
			VolumeWeight:    15,
			SentimentWeight: 15,
		},
	}
	applyDefaultNewSections(config)
	return config
}

// applyDefaultNewSections adds default values for the 12 new sections from Story 11.41
// if they are nil. This ensures the UI always has data to display.
func applyDefaultNewSections(config *ModeStrategyConfig) {
	// 1. Position Sizing (structured format)
	if config.PositionSizing == nil {
		config.PositionSizing = &StrategyPositionSizing{
			Leverage:     config.Leverage,
			MaxPositions: config.MaxPositions,
			BaseSizeUSD:  config.BaseSizeUSD,
		}
	}

	// 3. MTF (Multi-Timeframe)
	if config.MTF == nil {
		config.MTF = &StrategyMTF{
			Enabled:            false,
			PrimaryTimeframe:   "15m",
			PrimaryWeight:      50,
			SecondaryTimeframe: "1h",
			SecondaryWeight:    30,
			TertiaryTimeframe:  "4h",
			TertiaryWeight:     20,
			MinConsensus:       2,
			MinWeightedStrength: 60,
			TrendStabilityCheck: true,
		}
	}

	// 9. Circuit Breaker
	if config.CircuitBreaker == nil {
		config.CircuitBreaker = &StrategyCircuitBreaker{
			MaxLossPerHourUSD:     50,
			MaxLossPerDayUSD:      150,
			MaxConsecutiveLosses:  3,
			CooldownMinutes:       30,
			MaxTradesPerHour:      20,
			MaxTradesPerDay:       100,
			WinRateCheckAfter:     10,
			MinWinRatePct:         35,
		}
	}

	// 10. Hedge
	if config.Hedge == nil {
		config.Hedge = &StrategyHedge{
			AllowHedge:                false,
			MinConfidenceForHedge:     80,
			ExistingMustBeInProfitPct: 1.0,
			MaxHedgeSizePercent:       50,
			AllowSameModeHedge:        false,
			MaxTotalExposureMultiplier: 1.5,
		}
	}

	// 11. Averaging
	if config.Averaging == nil {
		config.Averaging = &StrategyAveraging{
			AllowAveraging:          false,
			AverageUpProfitPercent:  2.0,
			AverageDownLossPercent:  2.0,
			AddSizePercent:          50,
			MaxAverages:             2,
			MinConfidenceForAverage: 70,
			UseLLMForAveraging:      true,
			StagedEntryEnabled:      false,
			StagedEntryLevels:       3,
		}
	}

	// 12. Stale Release
	if config.StaleRelease == nil {
		config.StaleRelease = &StrategyStaleRelease{
			Enabled:                 true,
			MaxHoldDurationMinutes:  1440,
			MinProfitToKeepPct:      0.5,
			MaxLossToForceClosePct:  5.0,
			StaleZoneLoPct:          -1.0,
			StaleZoneHiPct:          0.5,
			StaleZoneAction:         "llm_decide",
		}
	}

	// 13. Position Optimization
	if config.PositionOptimization == nil {
		config.PositionOptimization = &StrategyPositionOptimization{
			ReentryEnabled:              false,
			ReentryAfterTP1:             true,
			ReentryMinPullbackPct:       0.5,
			MaxReentriesPerPosition:     2,
			DynamicSLEnabled:            true,
			DynamicSLAtBreakevenPct:     1.0,
			ProfitProtectionEnabled:     true,
			ProfitProtectionTriggerPct:  2.0,
			ProfitProtectionLockPct:     1.0,
		}
	}

	// 14. Funding Rate
	if config.FundingRate == nil {
		config.FundingRate = &StrategyFundingRate{
			Enabled:                 true,
			MaxFundingRatePct:       0.1,
			ExitBeforeFundingMinutes: 5,
			BlockEntryAboveRatePct:  0.05,
		}
	}

	// 15. Risk
	if config.Risk == nil {
		config.Risk = &StrategyRisk{
			RiskLevel:          "medium",
			MaxDrawdownPercent: 10,
			MaxDailyLossPercent: 5,
			PositionRiskPercent: 2,
		}
	}

	// 16. Trend Divergence
	if config.TrendDivergence == nil {
		config.TrendDivergence = &StrategyTrendDivergence{
			Enabled:              true,
			MinDivergencePercent: 5,
			BlockOnDivergence:    false,
			DivergenceWeight:     20,
		}
	}

	// 17. Dynamic AI Exit
	if config.DynamicAIExit == nil {
		config.DynamicAIExit = &StrategyDynamicAIExit{
			Enabled:             true,
			MinHoldBeforeAIMs:   30000,
			AICheckIntervalMs:   10000,
			UseLLMForLoss:       true,
			UseLLMForProfit:     true,
			MaxHoldTimeMs:       3600000,
		}
	}

	// 18. Early Warning
	if config.EarlyWarning == nil {
		config.EarlyWarning = &StrategyEarlyWarning{
			Enabled:           true,
			StartAfterMinutes: 5,
			MinLossPercent:    1.0,
			CheckIntervalSecs: 30,
			CloseMinHoldMins:  2,
		}
	}
}

// GetStrategyConfig returns the strategy config for a given mode and strategy name
// Returns nil if the mode or strategy doesn't exist
func (m *ModesConfig) GetStrategyConfig(modeName, strategyName string) *ModeStrategyConfig {
	var modeConfig *ModeConfig

	switch modeName {
	case "scalp":
		modeConfig = &m.Scalp
	case "swing":
		modeConfig = &m.Swing
	case "position":
		modeConfig = &m.Position
	case "ultra_fast":
		modeConfig = &m.UltraFast
	default:
		return nil
	}

	if config, ok := modeConfig.Strategies[strategyName]; ok {
		return &config
	}
	return nil
}

// GetModeConfig returns the mode configuration by name
func (m *ModesConfig) GetModeConfig(modeName string) *ModeConfig {
	switch modeName {
	case "scalp":
		return &m.Scalp
	case "swing":
		return &m.Swing
	case "position":
		return &m.Position
	case "ultra_fast":
		return &m.UltraFast
	default:
		return nil
	}
}

// ValidModes returns the list of valid trading mode names
func ValidModes() []string {
	return []string{"scalp", "swing", "position", "ultra_fast"}
}

// ValidStrategies returns the list of valid strategy names
func ValidStrategies() []string {
	return []string{"trend_following", "mean_reversion", "breakout", "range_trading"}
}

// ====== SYSTEM CONTROL SWITCHES ======

// System control constants
const (
	// Order tracking systems
	OrderTrackingChain  = "chain"  // New chain-based system (order_chains, position_states)
	OrderTrackingLegacy = "legacy" // Old system (trade_lifecycle_events with source: ginie)
	OrderTrackingBoth   = "both"   // Log to both systems (for migration/testing)

	// Position management systems
	PositionManagementLegacy = "legacy" // Old Ginie position management
	PositionManagementChain  = "chain"  // New chain-based position tracking

	// Entry decision systems
	EntryDecisionLegacy = "legacy" // Old Ginie confluence system
	EntryDecisionChain  = "chain"  // New chain-based entry decision system
)

// UserSystemControl represents per-user system control switches
// Epic 7 Enhancement: Control switches for selecting between legacy and new trading system components
type UserSystemControl struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`

	// Order Tracking System
	// 'chain' = New chain-based system (order_chains, order_chain_events, position_states)
	// 'legacy' = Old system (trade_lifecycle_events with source: ginie)
	// 'both' = Log to both systems (for migration/testing)
	OrderTrackingSystem string `json:"order_tracking_system"`

	// Position Management System
	// 'legacy' = Old Ginie position management (current default)
	// 'chain' = New chain-based position tracking
	PositionManagementSystem string `json:"position_management_system"`

	// Entry Decision System
	// 'legacy' = Old Ginie confluence system (current default)
	// 'chain' = New chain-based entry decision system
	EntryDecisionSystem string `json:"entry_decision_system"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserSystemControl returns default system control settings
// Default: chain for all systems (order tracking, position management, entry decision)
func DefaultUserSystemControl() *UserSystemControl {
	return &UserSystemControl{
		OrderTrackingSystem:      OrderTrackingChain,      // Use new chain system by default
		PositionManagementSystem: PositionManagementChain, // Use new chain system by default
		EntryDecisionSystem:      EntryDecisionChain,      // Use new chain system by default
	}
}

// IsOrderTrackingChain returns true if order tracking should use the new chain-based system
func (u *UserSystemControl) IsOrderTrackingChain() bool {
	return u.OrderTrackingSystem == OrderTrackingChain || u.OrderTrackingSystem == OrderTrackingBoth
}

// IsOrderTrackingLegacy returns true if order tracking should use the legacy system
func (u *UserSystemControl) IsOrderTrackingLegacy() bool {
	return u.OrderTrackingSystem == OrderTrackingLegacy || u.OrderTrackingSystem == OrderTrackingBoth
}

// IsPositionManagementChain returns true if position management should use the new chain-based system
func (u *UserSystemControl) IsPositionManagementChain() bool {
	return u.PositionManagementSystem == PositionManagementChain
}

// IsPositionManagementLegacy returns true if position management should use the legacy system
func (u *UserSystemControl) IsPositionManagementLegacy() bool {
	return u.PositionManagementSystem == PositionManagementLegacy
}

// IsEntryDecisionChain returns true if entry decision should use the new chain-based system
func (u *UserSystemControl) IsEntryDecisionChain() bool {
	return u.EntryDecisionSystem == EntryDecisionChain
}

// IsEntryDecisionLegacy returns true if entry decision should use the legacy Ginie confluence system
func (u *UserSystemControl) IsEntryDecisionLegacy() bool {
	return u.EntryDecisionSystem == EntryDecisionLegacy
}

// ValidOrderTrackingSystems returns valid order tracking system values
func ValidOrderTrackingSystems() []string {
	return []string{OrderTrackingChain, OrderTrackingLegacy, OrderTrackingBoth}
}

// ValidPositionManagementSystems returns valid position management system values
func ValidPositionManagementSystems() []string {
	return []string{PositionManagementLegacy, PositionManagementChain}
}

// ValidEntryDecisionSystems returns valid entry decision system values
func ValidEntryDecisionSystems() []string {
	return []string{EntryDecisionLegacy, EntryDecisionChain}
}
