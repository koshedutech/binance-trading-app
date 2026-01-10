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
	}
}

// ====== GINIE AUTOPILOT SETTINGS ======

// UserGinieSettings represents per-user Ginie autopilot configuration
// Matches migration 017_user_ginie_settings.sql
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
