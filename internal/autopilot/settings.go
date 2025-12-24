package autopilot

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SymbolPerformanceCategory represents a symbol's performance tier
type SymbolPerformanceCategory string

const (
	PerformanceBest     SymbolPerformanceCategory = "best"     // Top performers - can use aggressive settings
	PerformanceGood     SymbolPerformanceCategory = "good"     // Above average - use normal settings
	PerformanceNeutral  SymbolPerformanceCategory = "neutral"  // Average - use normal settings
	PerformancePoor     SymbolPerformanceCategory = "poor"     // Below average - use conservative settings
	PerformanceWorst    SymbolPerformanceCategory = "worst"    // Worst performers - very conservative or blacklist
	PerformanceBlacklist SymbolPerformanceCategory = "blacklist" // Do not trade
)

// ModeAllocationConfig holds capital allocation settings per trading mode
type ModeAllocationConfig struct {
	// Capital allocation (must sum to 100%)
	UltraFastScalpPercent float64 `json:"ultra_fast_scalp_percent"` // e.g., 20%
	ScalpPercent          float64 `json:"scalp_percent"`            // e.g., 30%
	SwingPercent          float64 `json:"swing_percent"`            // e.g., 35%
	PositionPercent       float64 `json:"position_percent"`         // e.g., 15%

	// Position limits per mode
	MaxUltraFastPositions int `json:"max_ultra_fast_positions"` // e.g., 3
	MaxScalpPositions     int `json:"max_scalp_positions"`      // e.g., 4
	MaxSwingPositions     int `json:"max_swing_positions"`      // e.g., 3
	MaxPositionPositions  int `json:"max_position_positions"`   // e.g., 2

	// Max USD per position per mode
	MaxUltraFastUSDPerPosition float64 `json:"max_ultra_fast_usd_per_position"` // e.g., $200
	MaxScalpUSDPerPosition     float64 `json:"max_scalp_usd_per_position"`      // e.g., $300
	MaxSwingUSDPerPosition     float64 `json:"max_swing_usd_per_position"`      // e.g., $500
	MaxPositionUSDPerPosition  float64 `json:"max_position_usd_per_position"`   // e.g., $750

	// Dynamic rebalancing (optional)
	AllowDynamicRebalance bool    `json:"allow_dynamic_rebalance"` // false = fixed
	RebalanceThresholdPct float64 `json:"rebalance_threshold_pct"` // e.g., 20% drift
}

// ModeAllocationState tracks runtime allocation state per mode
type ModeAllocationState struct {
	Mode                 string  // ultra_fast, scalp, swing, position
	AllocatedPercent     float64 // From config
	AllocatedUSD         float64 // Calculated from total capital
	UsedUSD              float64 // Currently in positions
	AvailableUSD         float64 // Remaining
	CurrentPositions     int
	MaxPositions         int
	CapitalUtilization   float64 // percentage
	PositionUtilization  float64 // percentage
	LastAllocation       time.Time
}

// ModeSafetyConfig holds safety settings per trading mode
type ModeSafetyConfig struct {
	// Rate limiting
	MaxTradesPerMinute int `json:"max_trades_per_minute"` // e.g., 10
	MaxTradesPerHour   int `json:"max_trades_per_hour"`   // e.g., 30
	MaxTradesPerDay    int `json:"max_trades_per_day"`    // e.g., 100

	// Cumulative profit monitoring
	EnableProfitMonitor     bool    `json:"enable_profit_monitor"`
	ProfitWindowMinutes     int     `json:"profit_window_minutes"`        // e.g., 10
	MaxLossPercentInWindow  float64 `json:"max_loss_percent_in_window"`   // e.g., -2%
	PauseCooldownMinutes    int     `json:"pause_cooldown_minutes"`       // e.g., 30

	// Win-rate monitoring
	EnableWinRateMonitor   bool    `json:"enable_win_rate_monitor"`
	WinRateSampleSize      int     `json:"win_rate_sample_size"`        // e.g., 20
	MinWinRateThreshold    float64 `json:"min_win_rate_threshold"`      // e.g., 50%
	WinRateCooldownMinutes int     `json:"win_rate_cooldown_minutes"`   // e.g., 60
}

// ModeSafetyState tracks runtime safety state per mode
type ModeSafetyState struct {
	Mode string // ultra_fast, scalp, swing, position

	// Rate limiting
	TradesLastMinute []time.Time // Sliding window of trade times
	TradesLastHour   []time.Time // Sliding window of trade times
	TradesToday      int         // Count of trades today

	// Profit monitoring
	ProfitWindow     []SafetyTradeResult // Sliding window of recent trades
	WindowProfitPct  float64             // Cumulative PnL % in window
	IsPausedProfit   bool
	ProfitPauseUntil time.Time

	// Win-rate monitoring
	RecentTrades      []SafetyTradeResult // Fixed-size circular buffer
	CurrentWinRate    float64
	IsPausedWinRate   bool
	WinRatePauseUntil time.Time

	// Overall
	IsPaused    bool
	PauseReason string
	LastUpdate  time.Time
}

// SafetyTradeResult represents a single trade for mode safety monitoring
type SafetyTradeResult struct {
	Symbol      string
	Timestamp   time.Time
	PnLUSD      float64
	PnLPercent  float64
	IsWinning   bool // Winning if PnLUSD > 0
	Mode        string
}

// SymbolSettings holds per-symbol trading configuration
type SymbolSettings struct {
	Symbol              string                    `json:"symbol"`
	Category            SymbolPerformanceCategory `json:"category"`             // Performance category
	MinConfidence       float64                   `json:"min_confidence"`       // Override min confidence (0 = use global)
	MaxPositionUSD      float64                   `json:"max_position_usd"`     // Override max position size (0 = use global)
	SizeMultiplier      float64                   `json:"size_multiplier"`      // Multiplier for position size (1.0 = normal)
	LeverageOverride    int                       `json:"leverage_override"`    // Override leverage (0 = use global)
	Enabled             bool                      `json:"enabled"`              // Whether to trade this symbol
	CustomROIPercent    float64                   `json:"custom_roi_percent"`   // Custom ROI% for early profit booking (0 = use mode defaults)
	Notes               string                    `json:"notes"`                // User notes about this symbol

	// Performance metrics (updated periodically)
	TotalTrades         int                       `json:"total_trades"`
	WinningTrades       int                       `json:"winning_trades"`
	TotalPnL            float64                   `json:"total_pnl"`
	WinRate             float64                   `json:"win_rate"`
	AvgPnL              float64                   `json:"avg_pnl"`
	LastUpdated         string                    `json:"last_updated"`
}

// SymbolPerformanceReport represents a performance analysis report
type SymbolPerformanceReport struct {
	Symbol        string  `json:"symbol"`
	Category      string  `json:"category"`
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	TotalPnL      float64 `json:"total_pnl"`
	WinRate       float64 `json:"win_rate"`
	AvgPnL        float64 `json:"avg_pnl"`
	AvgWin        float64 `json:"avg_win"`
	AvgLoss       float64 `json:"avg_loss"`

	// Current settings applied
	MinConfidence  float64 `json:"min_confidence"`
	MaxPositionUSD float64 `json:"max_position_usd"`
	SizeMultiplier float64 `json:"size_multiplier"`
	Enabled        bool    `json:"enabled"`
}

// AutopilotSettings holds persistent settings that survive restarts
type AutopilotSettings struct {
	// Dynamic SL/TP settings
	DynamicSLTPEnabled  bool    `json:"dynamic_sltp_enabled"`
	ATRPeriod           int     `json:"atr_period"`
	ATRMultiplierSL     float64 `json:"atr_multiplier_sl"`
	ATRMultiplierTP     float64 `json:"atr_multiplier_tp"`
	LLMSLTPWeight       float64 `json:"llm_sltp_weight"`
	MinSLPercent        float64 `json:"min_sl_percent"`
	MaxSLPercent        float64 `json:"max_sl_percent"`
	MinTPPercent        float64 `json:"min_tp_percent"`
	MaxTPPercent        float64 `json:"max_tp_percent"`

	// Scalping mode settings
	ScalpingModeEnabled     bool    `json:"scalping_mode_enabled"`
	ScalpingMinProfit       float64 `json:"scalping_min_profit"`
	ScalpingQuickReentry    bool    `json:"scalping_quick_reentry"`
	ScalpingReentryDelaySec int     `json:"scalping_reentry_delay_sec"`
	ScalpingMaxTradesPerDay int     `json:"scalping_max_trades_per_day"`

	// Circuit breaker settings
	CircuitBreakerEnabled    bool    `json:"circuit_breaker_enabled"`
	MaxLossPerHour           float64 `json:"max_loss_per_hour"`
	MaxDailyLoss             float64 `json:"max_daily_loss"`
	MaxConsecutiveLosses     int     `json:"max_consecutive_losses"`
	CooldownMinutes          int     `json:"cooldown_minutes"`
	MaxTradesPerMinute       int     `json:"max_trades_per_minute"`
	MaxDailyTrades           int     `json:"max_daily_trades"`

	// Autopilot mode settings (risk level, dry run, etc.)
	RiskLevel               string  `json:"risk_level"`                // conservative/moderate/aggressive
	DryRunMode              bool    `json:"dry_run_mode"`              // Paper trading mode
	MaxUSDAllocation        float64 `json:"max_usd_allocation"`        // Max USD to allocate
	ProfitReinvestPercent   float64 `json:"profit_reinvest_percent"`   // % of profit to reinvest
	ProfitReinvestRiskLevel string  `json:"profit_reinvest_risk_level"` // Risk level for reinvested profits

	// Ginie-specific settings
	GinieRiskLevel     string  `json:"ginie_risk_level"`      // conservative/moderate/aggressive
	GinieDryRunMode    bool    `json:"ginie_dry_run_mode"`    // Paper trading mode for Ginie
	GinieAutoStart     bool    `json:"ginie_auto_start"`      // Auto-start Ginie on server restart
	GinieMaxUSD        float64 `json:"ginie_max_usd"`         // Max USD per position for Ginie
	GinieLeverage      int     `json:"ginie_leverage"`        // Default leverage for Ginie
	GinieMinConfidence float64 `json:"ginie_min_confidence"`  // Min confidence to trade
	GinieMaxPositions  int     `json:"ginie_max_positions"`   // Max concurrent positions for Ginie

	// Ginie trend detection timeframes (per mode)
	GinieTrendTimeframeUltrafast string `json:"ginie_trend_timeframe_ultrafast"` // e.g., "5m"
	GinieTrendTimeframeScalp    string `json:"ginie_trend_timeframe_scalp"`    // e.g., "15m"
	GinieTrendTimeframeSwing    string `json:"ginie_trend_timeframe_swing"`    // e.g., "1h"
	GinieTrendTimeframePosition string `json:"ginie_trend_timeframe_position"` // e.g., "4h"

	// Ginie divergence detection
	GinieBlockOnDivergence bool `json:"ginie_block_on_divergence"` // Block trades when timeframe divergence detected

	// Ginie SL/TP manual overrides (per mode) - if set (> 0), override ATR/LLM calculations
	GinieSLPercentUltrafast    float64 `json:"ginie_sl_percent_ultrafast"`    // e.g., 0.5 (0.5%)
	GinieTPPercentUltrafast    float64 `json:"ginie_tp_percent_ultrafast"`    // e.g., 1.0 (1%)
	GinieSLPercentScalp        float64 `json:"ginie_sl_percent_scalp"`        // e.g., 1.0 (1%)
	GinieTPPercentScalp        float64 `json:"ginie_tp_percent_scalp"`        // e.g., 2.0 (2%)
	GinieSLPercentSwing        float64 `json:"ginie_sl_percent_swing"`        // e.g., 2.0 (2%)
	GinieTPPercentSwing        float64 `json:"ginie_tp_percent_swing"`        // e.g., 6.0 (6%)
	GinieSLPercentPosition     float64 `json:"ginie_sl_percent_position"`     // e.g., 3.0 (3%)
	GinieTPPercentPosition     float64 `json:"ginie_tp_percent_position"`     // e.g., 10.0 (10%)

	// Trailing stop configuration (per mode)
	GinieTrailingStopEnabledUltrafast       bool    `json:"ginie_trailing_stop_enabled_ultrafast"`
	GinieTrailingStopPercentUltrafast       float64 `json:"ginie_trailing_stop_percent_ultrafast"`       // e.g., 0.1%
	GinieTrailingStopActivationUltrafast    float64 `json:"ginie_trailing_stop_activation_ultrafast"`    // e.g., 0.2% profit

	GinieTrailingStopEnabledScalp           bool    `json:"ginie_trailing_stop_enabled_scalp"`
	GinieTrailingStopPercentScalp           float64 `json:"ginie_trailing_stop_percent_scalp"`       // e.g., 0.3%
	GinieTrailingStopActivationScalp        float64 `json:"ginie_trailing_stop_activation_scalp"`    // e.g., 0.5% profit

	GinieTrailingStopEnabledSwing           bool    `json:"ginie_trailing_stop_enabled_swing"`
	GinieTrailingStopPercentSwing           float64 `json:"ginie_trailing_stop_percent_swing"`       // e.g., 1.5%
	GinieTrailingStopActivationSwing        float64 `json:"ginie_trailing_stop_activation_swing"`    // e.g., 1.0% profit

	GinieTrailingStopEnabledPosition        bool    `json:"ginie_trailing_stop_enabled_position"`
	GinieTrailingStopPercentPosition        float64 `json:"ginie_trailing_stop_percent_position"`    // e.g., 3.0%
	GinieTrailingStopActivationPosition     float64 `json:"ginie_trailing_stop_activation_position"` // e.g., 2.0% profit

	// TP mode configuration
	GinieUseSingleTP      bool    `json:"ginie_use_single_tp"`       // true = 100% at TP1, false = 4-level
	GinieSingleTPPercent  float64 `json:"ginie_single_tp_percent"`   // If single TP, this is the gain %

	// TP allocation for multi-TP mode (if not using single TP)
	GinieTP1Percent float64 `json:"ginie_tp1_percent"` // e.g., 25.0 (25%)
	GinieTP2Percent float64 `json:"ginie_tp2_percent"` // e.g., 25.0 (25%)
	GinieTP3Percent float64 `json:"ginie_tp3_percent"` // e.g., 25.0 (25%)
	GinieTP4Percent float64 `json:"ginie_tp4_percent"` // e.g., 25.0 (25%)

	// Ginie PnL statistics (persisted)
	GinieTotalPnL      float64 `json:"ginie_total_pnl"`       // Lifetime realized PnL
	GinieDailyPnL      float64 `json:"ginie_daily_pnl"`       // Today's realized PnL
	GinieTotalTrades   int     `json:"ginie_total_trades"`    // Lifetime trade count
	GinieWinningTrades int     `json:"ginie_winning_trades"`  // Lifetime winning trades
	GinieDailyTrades   int     `json:"ginie_daily_trades"`    // Today's trade count
	GiniePnLLastUpdate string  `json:"ginie_pnl_last_update"` // Last update date for daily reset

	// ====== AUTO MODE SETTINGS (LLM-DRIVEN TRADING) ======
	// When enabled, LLM decides position size, leverage, coins to trade, averaging decisions
	AutoModeEnabled         bool    `json:"auto_mode_enabled"`          // Master toggle for auto mode
	AutoModeMaxPositions    int     `json:"auto_mode_max_positions"`    // Max concurrent positions (LLM will decide within this limit)
	AutoModeMaxLeverage     int     `json:"auto_mode_max_leverage"`     // Max leverage LLM can use (hard limit)
	AutoModeMaxPositionSize float64 `json:"auto_mode_max_position_size"` // Max USD per position (hard limit)
	AutoModeMaxTotalUSD     float64 `json:"auto_mode_max_total_usd"`    // Max total USD across all positions
	AutoModeAllowAveraging  bool    `json:"auto_mode_allow_averaging"`  // Allow LLM to decide averaging
	AutoModeMaxAverages     int     `json:"auto_mode_max_averages"`     // Max times to average per position
	AutoModeMinHoldMinutes  int     `json:"auto_mode_min_hold_minutes"` // Min time before exit (prevents flip-flopping)
	AutoModeQuickProfitMode bool    `json:"auto_mode_quick_profit"`     // Take quick profits and re-enter
	AutoModeMinProfitForExit float64 `json:"auto_mode_min_profit_exit"` // Min profit % to trigger quick exit

	// ====== SPOT AUTOPILOT SETTINGS ======
	// Spot trading mode settings
	SpotAutopilotEnabled  bool    `json:"spot_autopilot_enabled"`
	SpotDryRunMode        bool    `json:"spot_dry_run_mode"`
	SpotRiskLevel         string  `json:"spot_risk_level"`          // conservative/moderate/aggressive
	SpotMaxPositions      int     `json:"spot_max_positions"`
	SpotMaxUSDPerPosition float64 `json:"spot_max_usd_per_position"`
	SpotTakeProfitPercent float64 `json:"spot_take_profit_percent"`
	SpotStopLossPercent   float64 `json:"spot_stop_loss_percent"`
	SpotMinConfidence     float64 `json:"spot_min_confidence"`

	// Spot circuit breaker settings
	SpotCircuitBreakerEnabled    bool    `json:"spot_circuit_breaker_enabled"`
	SpotMaxLossPerHour           float64 `json:"spot_max_loss_per_hour"`
	SpotMaxDailyLoss             float64 `json:"spot_max_daily_loss"`
	SpotMaxConsecutiveLosses     int     `json:"spot_max_consecutive_losses"`
	SpotCooldownMinutes          int     `json:"spot_cooldown_minutes"`
	SpotMaxTradesPerMinute       int     `json:"spot_max_trades_per_minute"`
	SpotMaxDailyTrades           int     `json:"spot_max_daily_trades"`

	// Spot coin preferences
	SpotCoinBlacklist  []string `json:"spot_coin_blacklist"`
	SpotCoinWhitelist  []string `json:"spot_coin_whitelist"`
	SpotUseWhitelist   bool     `json:"spot_use_whitelist"`

	// Spot PnL statistics (persisted)
	SpotTotalPnL      float64 `json:"spot_total_pnl"`
	SpotDailyPnL      float64 `json:"spot_daily_pnl"`
	SpotTotalTrades   int     `json:"spot_total_trades"`
	SpotWinningTrades int     `json:"spot_winning_trades"`
	SpotDailyTrades   int     `json:"spot_daily_trades"`
	SpotPnLLastUpdate string  `json:"spot_pnl_last_update"`

	// ====== ULTRA-FAST SCALPING SETTINGS ======
	// 1-3 second exits with fee-aware profit targets and volatility-based re-entry
	UltraFastEnabled        bool    `json:"ultra_fast_enabled"`         // Master toggle for ultra-fast mode
	UltraFastScanInterval   int     `json:"ultra_fast_scan_interval"`   // Interval to scan for signals (milliseconds)
	UltraFastMonitorInterval int   `json:"ultra_fast_monitor_interval"` // Interval to check ultra-fast positions (milliseconds)
	UltraFastMaxPositions   int     `json:"ultra_fast_max_positions"`   // Max concurrent ultra-fast positions
	UltraFastMaxUSDPerPos   float64 `json:"ultra_fast_max_usd_per_pos"` // Max USD per ultra-fast position
	UltraFastMinConfidence  float64 `json:"ultra_fast_min_confidence"`  // Min confidence for ultra-fast trades
	UltraFastMinProfitPct   float64 `json:"ultra_fast_min_profit_pct"`  // Min profit % to exit (calculated dynamically if 0)
	UltraFastMaxHoldMS      int     `json:"ultra_fast_max_hold_ms"`     // Force exit after this many milliseconds

	// Ultra-fast specific counters for rate limiting
	UltraFastTodayTrades    int     `json:"ultra_fast_today_trades"`    // Trades executed today
	UltraFastMaxDailyTrades int     `json:"ultra_fast_max_daily_trades"` // Max daily trades
	UltraFastDailyPnL       float64 `json:"ultra_fast_daily_pnl"`       // Today's PnL
	UltraFastTotalPnL       float64 `json:"ultra_fast_total_pnl"`       // Lifetime PnL
	UltraFastWinRate        float64 `json:"ultra_fast_win_rate"`        // Current win rate %
	UltraFastLastUpdate     string  `json:"ultra_fast_last_update"`     // Last update date

	// ====== PER-MODE SAFETY CONTROLS ======
	// Independent safety settings per trading mode (rate limiting, profit monitoring, win-rate)
	SafetyUltraFast  *ModeSafetyConfig `json:"safety_ultra_fast"`
	SafetyScalp      *ModeSafetyConfig `json:"safety_scalp"`
	SafetySwing      *ModeSafetyConfig `json:"safety_swing"`
	SafetyPosition   *ModeSafetyConfig `json:"safety_position"`

	// ====== PER-MODE CAPITAL ALLOCATION ======
	// Independent capital budgets and position limits per trading mode
	ModeAllocation *ModeAllocationConfig `json:"mode_allocation"`

	// ====== PER-SYMBOL SETTINGS ======
	// Symbol-specific trading configuration based on performance
	SymbolSettings map[string]*SymbolSettings `json:"symbol_settings"`

	// Default adjustments for each performance category
	CategoryConfidenceBoost  map[string]float64 `json:"category_confidence_boost"`  // Extra confidence required by category
	CategorySizeMultiplier   map[string]float64 `json:"category_size_multiplier"`   // Size multiplier by category
}

// SettingsManager handles persistent settings storage
type SettingsManager struct {
	settingsPath string
	mu           sync.RWMutex
}

var (
	settingsManager *SettingsManager
	managerOnce     sync.Once
)

// GetSettingsManager returns the singleton settings manager
func GetSettingsManager() *SettingsManager {
	managerOnce.Do(func() {
		// Use a settings file in the config directory
		settingsPath := getSettingsFilePath()
		settingsManager = &SettingsManager{
			settingsPath: settingsPath,
		}
	})
	return settingsManager
}

// getSettingsFilePath returns the path to the settings file
func getSettingsFilePath() string {
	const settingsFileName = "autopilot_settings.json"

	// First, try the current working directory
	if cwd, err := os.Getwd(); err == nil {
		cwdPath := filepath.Join(cwd, settingsFileName)
		if _, err := os.Stat(cwdPath); err == nil {
			return cwdPath
		}
	}

	// Then try the executable directory (useful for compiled binaries)
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		execDirPath := filepath.Join(execDir, settingsFileName)
		if _, err := os.Stat(execDirPath); err == nil {
			return execDirPath
		}
	}

	// Default to current directory relative path (will be created if doesn't exist)
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, settingsFileName)
	}

	return settingsFileName
}

// DefaultSettings returns the default settings
func DefaultSettings() *AutopilotSettings {
	return &AutopilotSettings{
		// Dynamic SL/TP defaults
		DynamicSLTPEnabled:  false,
		ATRPeriod:           14,
		ATRMultiplierSL:     1.5,
		ATRMultiplierTP:     2.0,
		LLMSLTPWeight:       0.3,
		MinSLPercent:        0.3,
		MaxSLPercent:        3.0,
		MinTPPercent:        0.5,
		MaxTPPercent:        5.0,

		// Scalping defaults
		ScalpingModeEnabled:     false,
		ScalpingMinProfit:       0.2,
		ScalpingQuickReentry:    false,
		ScalpingReentryDelaySec: 5,
		ScalpingMaxTradesPerDay: 0,

		// Circuit breaker defaults
		CircuitBreakerEnabled:    true,
		MaxLossPerHour:           100,
		MaxDailyLoss:             500,
		MaxConsecutiveLosses:     5,
		CooldownMinutes:          30,
		MaxTradesPerMinute:       10,
		MaxDailyTrades:           100,

		// Autopilot mode defaults
		RiskLevel:               "moderate",
		DryRunMode:              true,
		MaxUSDAllocation:        2500,
		ProfitReinvestPercent:   50,
		ProfitReinvestRiskLevel: "aggressive",

		// Ginie defaults
		GinieRiskLevel:     "moderate",
		GinieDryRunMode:    false, // Default to LIVE mode, not PAPER mode
		GinieAutoStart:     false, // Don't auto-start by default
		GinieMaxUSD:        500,
		GinieLeverage:      10,
		GinieMinConfidence: 65.0,
		GinieMaxPositions:  10,

		// Ginie trend timeframe defaults (per mode)
		GinieTrendTimeframeUltrafast: "5m",
		GinieTrendTimeframeScalp:    "15m",
		GinieTrendTimeframeSwing:    "1h",
		GinieTrendTimeframePosition: "4h",

		// Ginie divergence detection
		GinieBlockOnDivergence: true, // Default to safest mode (block on severe divergence)

		// Ginie SL/TP manual overrides (0 = use ATR/LLM blend)
		GinieSLPercentUltrafast:    0,   // Disabled, use ATR/LLM
		GinieTPPercentUltrafast:    0,   // Disabled, use ATR/LLM
		GinieSLPercentScalp:        0,   // Disabled, use ATR/LLM
		GinieTPPercentScalp:        0,   // Disabled, use ATR/LLM
		GinieSLPercentSwing:        0,   // Disabled, use ATR/LLM
		GinieTPPercentSwing:        0,   // Disabled, use ATR/LLM
		GinieSLPercentPosition:     0,   // Disabled, use ATR/LLM
		GinieTPPercentPosition:     0,   // Disabled, use ATR/LLM

		// Ginie trailing stop defaults (match current hardcoded values)
		GinieTrailingStopEnabledUltrafast:       true,
		GinieTrailingStopPercentUltrafast:       0.1,  // 0.1%
		GinieTrailingStopActivationUltrafast:    0.2,  // After 0.2% profit

		GinieTrailingStopEnabledScalp:           true,
		GinieTrailingStopPercentScalp:           0.3,  // 0.3%
		GinieTrailingStopActivationScalp:        0.5,  // After 0.5% profit

		GinieTrailingStopEnabledSwing:           true,
		GinieTrailingStopPercentSwing:           1.5,  // 1.5%
		GinieTrailingStopActivationSwing:        1.0,  // After 1% profit

		GinieTrailingStopEnabledPosition:        true,
		GinieTrailingStopPercentPosition:        3.0,  // 3.0%
		GinieTrailingStopActivationPosition:     2.0,  // After 2% profit

		// Ginie TP mode (default to 4-level system)
		GinieUseSingleTP:     false, // Use multi-TP
		GinieSingleTPPercent: 5.0,   // If single TP enabled, 5% gain

		// Ginie TP allocation (current default: 25% each)
		GinieTP1Percent: 25.0,
		GinieTP2Percent: 25.0,
		GinieTP3Percent: 25.0,
		GinieTP4Percent: 25.0,

		// Auto Mode defaults (LLM-driven trading)
		AutoModeEnabled:          false, // Disabled by default - user must opt in
		AutoModeMaxPositions:     5,     // Max 5 concurrent positions
		AutoModeMaxLeverage:      10,    // Hard limit on leverage
		AutoModeMaxPositionSize:  500,   // Max $500 per position
		AutoModeMaxTotalUSD:      2000,  // Max $2000 total across all positions
		AutoModeAllowAveraging:   true,  // Allow averaging by default
		AutoModeMaxAverages:      2,     // Max 2 averages per position
		AutoModeMinHoldMinutes:   5,     // Hold at least 5 minutes
		AutoModeQuickProfitMode:  false, // Disabled by default
		AutoModeMinProfitForExit: 1.0,   // 1% min profit for quick exit

		// Ultra-fast scalping defaults (1-3 second exits)
		UltraFastEnabled:         false, // Disabled by default - high risk mode
		UltraFastScanInterval:    5000,  // Scan every 5 seconds
		UltraFastMonitorInterval: 500,   // Monitor every 500ms for exits
		UltraFastMaxPositions:    3,     // Conservative - max 3 concurrent positions
		UltraFastMaxUSDPerPos:    200,   // Conservative - $200 per position
		UltraFastMinConfidence:   50.0,  // Lower confidence for quick moves
		UltraFastMinProfitPct:    0.0,   // Dynamic calculation (fee-aware)
		UltraFastMaxHoldMS:       3000,  // Force exit after 3 seconds
		UltraFastTodayTrades:     0,
		UltraFastMaxDailyTrades:  50,    // Max 50 trades per day
		UltraFastDailyPnL:        0,
		UltraFastTotalPnL:        0,
		UltraFastWinRate:         0,
		UltraFastLastUpdate:      "",

		// Spot autopilot defaults
		SpotAutopilotEnabled:  false,
		SpotDryRunMode:        true,
		SpotRiskLevel:         "moderate",
		SpotMaxPositions:      5,
		SpotMaxUSDPerPosition: 100,
		SpotTakeProfitPercent: 3.0,
		SpotStopLossPercent:   2.0,
		SpotMinConfidence:     65.0,

		// Spot circuit breaker defaults
		SpotCircuitBreakerEnabled:    true,
		SpotMaxLossPerHour:           50,
		SpotMaxDailyLoss:             200,
		SpotMaxConsecutiveLosses:     5,
		SpotCooldownMinutes:          30,
		SpotMaxTradesPerMinute:       5,
		SpotMaxDailyTrades:           50,

		// Spot coin preferences defaults
		SpotCoinBlacklist: []string{},
		SpotCoinWhitelist: []string{},
		SpotUseWhitelist:  false,

		// Per-mode safety controls defaults
		SafetyUltraFast: &ModeSafetyConfig{
			// Rate limiting - ultra-fast is aggressive
			MaxTradesPerMinute: 5,   // Max 5 per minute
			MaxTradesPerHour:   20,  // Max 20 per hour
			MaxTradesPerDay:    50,  // Max 50 per day

			// Profit monitoring - very sensitive
			EnableProfitMonitor:    true,
			ProfitWindowMinutes:    10,   // Monitor last 10 minutes
			MaxLossPercentInWindow: -1.5, // Max -1.5% loss in window
			PauseCooldownMinutes:   30,   // Pause for 30 min if threshold hit

			// Win-rate monitoring - moderate sensitivity
			EnableWinRateMonitor:   true,
			WinRateSampleSize:      15, // Last 15 trades
			MinWinRateThreshold:    50, // Need 50% win rate
			WinRateCooldownMinutes: 60, // Pause for 60 min if below threshold
		},
		SafetyScalp: &ModeSafetyConfig{
			// Rate limiting - moderate
			MaxTradesPerMinute: 8,
			MaxTradesPerHour:   30,
			MaxTradesPerDay:    100,

			// Profit monitoring
			EnableProfitMonitor:    true,
			ProfitWindowMinutes:    15,
			MaxLossPercentInWindow: -2.0,
			PauseCooldownMinutes:   30,

			// Win-rate monitoring
			EnableWinRateMonitor:   true,
			WinRateSampleSize:      20,
			MinWinRateThreshold:    50,
			WinRateCooldownMinutes: 60,
		},
		SafetySwing: &ModeSafetyConfig{
			// Rate limiting - conservative
			MaxTradesPerMinute: 10,
			MaxTradesPerHour:   30,
			MaxTradesPerDay:    80,

			// Profit monitoring
			EnableProfitMonitor:    true,
			ProfitWindowMinutes:    60,   // Monitor last hour
			MaxLossPercentInWindow: -3.0, // Max -3% loss in window
			PauseCooldownMinutes:   60,

			// Win-rate monitoring
			EnableWinRateMonitor:   true,
			WinRateSampleSize:      25,
			MinWinRateThreshold:    55,
			WinRateCooldownMinutes: 120,
		},
		SafetyPosition: &ModeSafetyConfig{
			// Rate limiting - very conservative
			MaxTradesPerMinute: 5,
			MaxTradesPerHour:   15,
			MaxTradesPerDay:    50,

			// Profit monitoring
			EnableProfitMonitor:    true,
			ProfitWindowMinutes:    120,  // Monitor last 2 hours
			MaxLossPercentInWindow: -5.0, // Max -5% loss in window
			PauseCooldownMinutes:   120,

			// Win-rate monitoring
			EnableWinRateMonitor:   true,
			WinRateSampleSize:      30,
			MinWinRateThreshold:    60,
			WinRateCooldownMinutes: 180,
		},

		// Per-mode capital allocation defaults (conservative)
		ModeAllocation: &ModeAllocationConfig{
			// Capital allocation: must sum to 100%
			UltraFastScalpPercent: 20.0, // 20% to ultra-fast scalping
			ScalpPercent:          30.0, // 30% to scalping
			SwingPercent:          35.0, // 35% to swing trading
			PositionPercent:       15.0, // 15% to position trading

			// Position limits per mode
			MaxUltraFastPositions: 3, // Max 3 concurrent ultra-fast positions
			MaxScalpPositions:     4, // Max 4 concurrent scalp positions
			MaxSwingPositions:     3, // Max 3 concurrent swing positions
			MaxPositionPositions:  2, // Max 2 concurrent position trades

			// Max USD per position per mode
			MaxUltraFastUSDPerPosition: 200,  // $200 per ultra-fast position
			MaxScalpUSDPerPosition:     300,  // $300 per scalp position
			MaxSwingUSDPerPosition:     500,  // $500 per swing position
			MaxPositionUSDPerPosition:  750,  // $750 per position trade

			// Dynamic rebalancing
			AllowDynamicRebalance: false,   // Fixed allocation by default
			RebalanceThresholdPct: 20.0,    // Allow 20% drift before rebalancing
		},

		// Per-symbol settings defaults
		SymbolSettings: make(map[string]*SymbolSettings),

		// Default category adjustments
		// These define how much to boost confidence or reduce size for each category
		CategoryConfidenceBoost: map[string]float64{
			"best":      -5.0,  // Can use 5% lower confidence for best performers
			"good":      0.0,   // Normal confidence
			"neutral":   0.0,   // Normal confidence
			"poor":      10.0,  // Require 10% higher confidence for poor performers
			"worst":     20.0,  // Require 20% higher confidence for worst performers
			"blacklist": 100.0, // Effectively disable (impossible confidence)
		},
		CategorySizeMultiplier: map[string]float64{
			"best":      1.5,  // 50% larger positions for best performers
			"good":      1.2,  // 20% larger positions
			"neutral":   1.0,  // Normal size
			"poor":      0.5,  // 50% smaller positions for poor performers
			"worst":     0.25, // 75% smaller positions for worst performers
			"blacklist": 0.0,  // No positions
		},
	}
}

// LoadSettings loads settings from file, returns defaults if file doesn't exist
func (sm *SettingsManager) LoadSettings() (*AutopilotSettings, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	settings := DefaultSettings()

	data, err := os.ReadFile(sm.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return defaults
			return settings, nil
		}
		return settings, err
	}

	if err := json.Unmarshal(data, settings); err != nil {
		return DefaultSettings(), err
	}

	return settings, nil
}

// SaveSettings saves settings to file using atomic write pattern
// Uses temp file + rename for crash-safe persistence
func (sm *SettingsManager) SaveSettings(settings *AutopilotSettings) error {
	// Marshal data outside the lock to avoid holding lock during I/O
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write pattern: write to temp file, then rename
	// This prevents corruption if the process crashes during write
	tempPath := sm.settingsPath + ".tmp"

	// Write to temp file
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return err
	}

	// Sync to disk to ensure data is written
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return err
	}

	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}

	// Now acquire lock for the file replacement
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Atomic rename (replace existing file)
	if err := os.Rename(tempPath, sm.settingsPath); err != nil {
		os.Remove(tempPath)
		return err
	}

	return nil
}

// GetCurrentSettings gets current settings (loading from file if needed)
func (sm *SettingsManager) GetCurrentSettings() *AutopilotSettings {
	settings, _ := sm.LoadSettings()
	return settings
}

// UpdateDynamicSLTP updates dynamic SL/TP settings and saves to file
func (sm *SettingsManager) UpdateDynamicSLTP(
	enabled bool,
	atrPeriod int,
	atrMultiplierSL float64,
	atrMultiplierTP float64,
	llmWeight float64,
	minSL float64,
	maxSL float64,
	minTP float64,
	maxTP float64,
) error {
	settings := sm.GetCurrentSettings()

	settings.DynamicSLTPEnabled = enabled
	if atrPeriod > 0 {
		settings.ATRPeriod = atrPeriod
	}
	if atrMultiplierSL > 0 {
		settings.ATRMultiplierSL = atrMultiplierSL
	}
	if atrMultiplierTP > 0 {
		settings.ATRMultiplierTP = atrMultiplierTP
	}
	if llmWeight >= 0 && llmWeight <= 1 {
		settings.LLMSLTPWeight = llmWeight
	}
	if minSL > 0 {
		settings.MinSLPercent = minSL
	}
	if maxSL > 0 {
		settings.MaxSLPercent = maxSL
	}
	if minTP > 0 {
		settings.MinTPPercent = minTP
	}
	if maxTP > 0 {
		settings.MaxTPPercent = maxTP
	}

	return sm.SaveSettings(settings)
}

// UpdateScalping updates scalping settings and saves to file
func (sm *SettingsManager) UpdateScalping(
	enabled bool,
	minProfit float64,
	quickReentry bool,
	reentryDelaySec int,
	maxTradesPerDay int,
) error {
	settings := sm.GetCurrentSettings()

	settings.ScalpingModeEnabled = enabled
	if minProfit > 0 {
		settings.ScalpingMinProfit = minProfit
	}
	settings.ScalpingQuickReentry = quickReentry
	if reentryDelaySec > 0 {
		settings.ScalpingReentryDelaySec = reentryDelaySec
	}
	if maxTradesPerDay >= 0 {
		settings.ScalpingMaxTradesPerDay = maxTradesPerDay
	}

	return sm.SaveSettings(settings)
}

// UpdateCircuitBreaker updates circuit breaker settings and saves to file
func (sm *SettingsManager) UpdateCircuitBreaker(
	enabled bool,
	maxLossPerHour float64,
	maxDailyLoss float64,
	maxConsecutiveLosses int,
	cooldownMinutes int,
	maxTradesPerMinute int,
	maxDailyTrades int,
) error {
	settings := sm.GetCurrentSettings()

	settings.CircuitBreakerEnabled = enabled
	if maxLossPerHour > 0 {
		settings.MaxLossPerHour = maxLossPerHour
	}
	if maxDailyLoss > 0 {
		settings.MaxDailyLoss = maxDailyLoss
	}
	if maxConsecutiveLosses > 0 {
		settings.MaxConsecutiveLosses = maxConsecutiveLosses
	}
	if cooldownMinutes > 0 {
		settings.CooldownMinutes = cooldownMinutes
	}
	if maxTradesPerMinute > 0 {
		settings.MaxTradesPerMinute = maxTradesPerMinute
	}
	if maxDailyTrades > 0 {
		settings.MaxDailyTrades = maxDailyTrades
	}

	return sm.SaveSettings(settings)
}

// UpdateAutopilotMode updates autopilot mode settings and saves to file
func (sm *SettingsManager) UpdateAutopilotMode(
	riskLevel string,
	dryRun bool,
	maxAllocation float64,
	profitReinvestPercent float64,
	profitReinvestRiskLevel string,
) error {
	settings := sm.GetCurrentSettings()

	if riskLevel != "" {
		settings.RiskLevel = riskLevel
	}
	settings.DryRunMode = dryRun
	if maxAllocation > 0 {
		settings.MaxUSDAllocation = maxAllocation
	}
	if profitReinvestPercent >= 0 {
		settings.ProfitReinvestPercent = profitReinvestPercent
	}
	if profitReinvestRiskLevel != "" {
		settings.ProfitReinvestRiskLevel = profitReinvestRiskLevel
	}

	return sm.SaveSettings(settings)
}

// UpdateRiskLevel updates just the risk level setting
func (sm *SettingsManager) UpdateRiskLevel(riskLevel string) error {
	settings := sm.GetCurrentSettings()
	settings.RiskLevel = riskLevel
	return sm.SaveSettings(settings)
}

// UpdateDryRunMode updates just the dry run mode setting
func (sm *SettingsManager) UpdateDryRunMode(dryRun bool) error {
	settings := sm.GetCurrentSettings()
	settings.GinieDryRunMode = dryRun
	return sm.SaveSettings(settings)
}

// UpdateMaxAllocation updates just the max USD allocation setting
func (sm *SettingsManager) UpdateMaxAllocation(maxAllocation float64) error {
	settings := sm.GetCurrentSettings()
	settings.MaxUSDAllocation = maxAllocation
	return sm.SaveSettings(settings)
}

// UpdateProfitReinvest updates profit reinvestment settings
func (sm *SettingsManager) UpdateProfitReinvest(percent float64, riskLevel string) error {
	settings := sm.GetCurrentSettings()
	settings.ProfitReinvestPercent = percent
	settings.ProfitReinvestRiskLevel = riskLevel
	return sm.SaveSettings(settings)
}

// UpdateGinieRiskLevel updates Ginie risk level and related settings
func (sm *SettingsManager) UpdateGinieRiskLevel(riskLevel string) error {
	settings := sm.GetCurrentSettings()
	settings.GinieRiskLevel = riskLevel

	// Adjust related settings based on risk level
	switch riskLevel {
	case "conservative":
		settings.GinieMinConfidence = 75.0
		settings.GinieMaxUSD = 300
		settings.GinieLeverage = 3
	case "moderate":
		settings.GinieMinConfidence = 65.0
		settings.GinieMaxUSD = 500
		settings.GinieLeverage = 5
	case "aggressive":
		settings.GinieMinConfidence = 55.0
		settings.GinieMaxUSD = 800
		settings.GinieLeverage = 10
	}

	return sm.SaveSettings(settings)
}

// UpdateGinieSettings updates all Ginie-specific settings
func (sm *SettingsManager) UpdateGinieSettings(
	riskLevel string,
	dryRun bool,
	maxUSD float64,
	leverage int,
	minConfidence float64,
	maxPositions int,
) error {
	settings := sm.GetCurrentSettings()

	if riskLevel != "" {
		settings.GinieRiskLevel = riskLevel
	}
	settings.GinieDryRunMode = dryRun
	if maxUSD > 0 {
		settings.GinieMaxUSD = maxUSD
	}
	if leverage > 0 {
		settings.GinieLeverage = leverage
	}
	if minConfidence > 0 {
		settings.GinieMinConfidence = minConfidence
	}
	if maxPositions > 0 {
		settings.GinieMaxPositions = maxPositions
	}

	return sm.SaveSettings(settings)
}

// UpdateGinieAutoStart updates the Ginie auto-start setting
func (sm *SettingsManager) UpdateGinieAutoStart(autoStart bool) error {
	settings := sm.GetCurrentSettings()
	settings.GinieAutoStart = autoStart
	return sm.SaveSettings(settings)
}

// GetGinieAutoStart returns whether Ginie should auto-start
func (sm *SettingsManager) GetGinieAutoStart() bool {
	settings := sm.GetCurrentSettings()
	return settings.GinieAutoStart
}

// ValidBinanceTimeframes lists all valid Binance timeframe intervals
var ValidBinanceTimeframes = map[string]bool{
	"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
	"1h": true, "2h": true, "4h": true, "6h": true, "8h": true,
	"12h": true, "1d": true, "3d": true, "1w": true, "1M": true,
}

// ValidateTimeframe checks if a timeframe string is valid for Binance API
func ValidateTimeframe(tf string) error {
	if !ValidBinanceTimeframes[tf] {
		return fmt.Errorf("invalid timeframe '%s': must be one of: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M", tf)
	}
	return nil
}

// UpdateGinieTrendTimeframes updates the trend timeframe settings for each mode
func (sm *SettingsManager) UpdateGinieTrendTimeframes(
	ultrafastTF string,
	scalpTF string,
	swingTF string,
	positionTF string,
	blockOnDivergence bool,
) error {
	// Validate all timeframes before saving
	if ultrafastTF != "" {
		if err := ValidateTimeframe(ultrafastTF); err != nil {
			return fmt.Errorf("ultrafast timeframe: %w", err)
		}
	}
	if scalpTF != "" {
		if err := ValidateTimeframe(scalpTF); err != nil {
			return fmt.Errorf("scalp timeframe: %w", err)
		}
	}
	if swingTF != "" {
		if err := ValidateTimeframe(swingTF); err != nil {
			return fmt.Errorf("swing timeframe: %w", err)
		}
	}
	if positionTF != "" {
		if err := ValidateTimeframe(positionTF); err != nil {
			return fmt.Errorf("position timeframe: %w", err)
		}
	}

	// Get current settings BEFORE acquiring the lock to avoid deadlock
	// (GetCurrentSettings acquires RLock internally)
	settings := sm.GetCurrentSettings()

	if ultrafastTF != "" {
		settings.GinieTrendTimeframeUltrafast = ultrafastTF
	}
	if scalpTF != "" {
		settings.GinieTrendTimeframeScalp = scalpTF
	}
	if swingTF != "" {
		settings.GinieTrendTimeframeSwing = swingTF
	}
	if positionTF != "" {
		settings.GinieTrendTimeframePosition = positionTF
	}
	settings.GinieBlockOnDivergence = blockOnDivergence

	return sm.SaveSettings(settings)
}

// ValidateSLTPPercents validates SL/TP percentage settings
func ValidateSLTPPercents(slPct, tpPct float64) error {
	if slPct < 0 || slPct > 20 {
		return fmt.Errorf("SL percent must be between 0-20%%, got: %.2f", slPct)
	}
	if tpPct < 0 || tpPct > 50 {
		return fmt.Errorf("TP percent must be between 0-50%%, got: %.2f", tpPct)
	}
	if slPct > 0 && tpPct > 0 && tpPct <= slPct {
		return fmt.Errorf("TP (%.2f%%) must be greater than SL (%.2f%%)", tpPct, slPct)
	}
	return nil
}

// ValidateTPAllocation validates TP allocation percentages sum to 100
func ValidateTPAllocation(tp1, tp2, tp3, tp4 float64) error {
	total := tp1 + tp2 + tp3 + tp4
	if math.Abs(total-100.0) > 0.1 {
		return fmt.Errorf("TP allocation must sum to 100%%, got: %.2f%%", total)
	}
	if tp1 < 0 || tp2 < 0 || tp3 < 0 || tp4 < 0 {
		return fmt.Errorf("TP percentages cannot be negative")
	}
	return nil
}

// UpdateGinieSLTPSettings updates SL/TP configuration for a specific mode
func (sm *SettingsManager) UpdateGinieSLTPSettings(
	mode string, // "scalp", "swing", "position"
	slPercent, tpPercent float64,
	trailingEnabled bool,
	trailingPercent, trailingActivation float64,
) error {
	// Validate inputs
	if err := ValidateSLTPPercents(slPercent, tpPercent); err != nil {
		return err
	}

	if trailingPercent < 0 || trailingPercent > 10 {
		return fmt.Errorf("trailing percent must be 0-10%%, got: %.2f", trailingPercent)
	}

	if trailingActivation < 0 || trailingActivation > 20 {
		return fmt.Errorf("trailing activation must be 0-20%%, got: %.2f", trailingActivation)
	}

	// Get current settings BEFORE acquiring the lock to avoid deadlock
	// (GetCurrentSettings acquires RLock internally)
	settings := sm.GetCurrentSettings()

	switch mode {
	case "scalp":
		settings.GinieSLPercentScalp = slPercent
		settings.GinieTPPercentScalp = tpPercent
		settings.GinieTrailingStopEnabledScalp = trailingEnabled
		settings.GinieTrailingStopPercentScalp = trailingPercent
		settings.GinieTrailingStopActivationScalp = trailingActivation
	case "swing":
		settings.GinieSLPercentSwing = slPercent
		settings.GinieTPPercentSwing = tpPercent
		settings.GinieTrailingStopEnabledSwing = trailingEnabled
		settings.GinieTrailingStopPercentSwing = trailingPercent
		settings.GinieTrailingStopActivationSwing = trailingActivation
	case "position":
		settings.GinieSLPercentPosition = slPercent
		settings.GinieTPPercentPosition = tpPercent
		settings.GinieTrailingStopEnabledPosition = trailingEnabled
		settings.GinieTrailingStopPercentPosition = trailingPercent
		settings.GinieTrailingStopActivationPosition = trailingActivation
	default:
		return fmt.Errorf("invalid mode: %s (must be scalp, swing, or position)", mode)
	}

	return sm.SaveSettings(settings)
}

// UpdateGinieTPMode updates the TP mode (single vs multi) and allocation
func (sm *SettingsManager) UpdateGinieTPMode(
	useSingleTP bool,
	singleTPPercent float64,
	tp1, tp2, tp3, tp4 float64,
) error {
	if useSingleTP {
		if singleTPPercent <= 0 || singleTPPercent > 50 {
			return fmt.Errorf("single TP percent must be 0-50%%, got: %.2f", singleTPPercent)
		}
	} else {
		if err := ValidateTPAllocation(tp1, tp2, tp3, tp4); err != nil {
			return err
		}
	}

	// Get current settings BEFORE acquiring the lock to avoid deadlock
	// (GetCurrentSettings acquires RLock internally)
	settings := sm.GetCurrentSettings()
	settings.GinieUseSingleTP = useSingleTP
	settings.GinieSingleTPPercent = singleTPPercent
	settings.GinieTP1Percent = tp1
	settings.GinieTP2Percent = tp2
	settings.GinieTP3Percent = tp3
	settings.GinieTP4Percent = tp4

	return sm.SaveSettings(settings)
}

// ResetToDefaults resets all settings to defaults and saves to file
func (sm *SettingsManager) ResetToDefaults() error {
	return sm.SaveSettings(DefaultSettings())
}

// UpdateGiniePnLStats updates Ginie PnL statistics and saves to file
func (sm *SettingsManager) UpdateGiniePnLStats(
	totalPnL float64,
	dailyPnL float64,
	totalTrades int,
	winningTrades int,
	dailyTrades int,
) error {
	settings := sm.GetCurrentSettings()

	settings.GinieTotalPnL = totalPnL
	settings.GinieDailyPnL = dailyPnL
	settings.GinieTotalTrades = totalTrades
	settings.GinieWinningTrades = winningTrades
	settings.GinieDailyTrades = dailyTrades
	settings.GiniePnLLastUpdate = time.Now().Format("2006-01-02")

	return sm.SaveSettings(settings)
}

// GetGiniePnLStats returns Ginie PnL statistics, resetting daily values if date changed
func (sm *SettingsManager) GetGiniePnLStats() (totalPnL, dailyPnL float64, totalTrades, winningTrades, dailyTrades int) {
	settings := sm.GetCurrentSettings()

	today := time.Now().Format("2006-01-02")
	if settings.GiniePnLLastUpdate != today {
		// New day - reset daily counters
		settings.GinieDailyPnL = 0
		settings.GinieDailyTrades = 0
		settings.GiniePnLLastUpdate = today
		sm.SaveSettings(settings)
	}

	return settings.GinieTotalPnL, settings.GinieDailyPnL,
		settings.GinieTotalTrades, settings.GinieWinningTrades, settings.GinieDailyTrades
}

// ============================================================================
// SPOT AUTOPILOT SETTINGS METHODS
// ============================================================================

// UpdateSpotDryRunMode updates the spot dry run mode setting
func (sm *SettingsManager) UpdateSpotDryRunMode(dryRun bool) error {
	settings := sm.GetCurrentSettings()
	settings.SpotDryRunMode = dryRun
	return sm.SaveSettings(settings)
}

// UpdateSpotRiskLevel updates the spot risk level setting
func (sm *SettingsManager) UpdateSpotRiskLevel(riskLevel string) error {
	settings := sm.GetCurrentSettings()
	settings.SpotRiskLevel = riskLevel

	// Adjust related settings based on risk level
	switch riskLevel {
	case "conservative":
		settings.SpotMinConfidence = 75.0
		settings.SpotMaxUSDPerPosition = 50
		settings.SpotTakeProfitPercent = 2.0
		settings.SpotStopLossPercent = 1.0
	case "moderate":
		settings.SpotMinConfidence = 65.0
		settings.SpotMaxUSDPerPosition = 100
		settings.SpotTakeProfitPercent = 3.0
		settings.SpotStopLossPercent = 2.0
	case "aggressive":
		settings.SpotMinConfidence = 55.0
		settings.SpotMaxUSDPerPosition = 200
		settings.SpotTakeProfitPercent = 5.0
		settings.SpotStopLossPercent = 3.0
	}

	return sm.SaveSettings(settings)
}

// UpdateSpotMaxAllocation updates the spot max USD per position setting
func (sm *SettingsManager) UpdateSpotMaxAllocation(maxUSD float64) error {
	settings := sm.GetCurrentSettings()
	settings.SpotMaxUSDPerPosition = maxUSD
	return sm.SaveSettings(settings)
}

// UpdateSpotSettings updates all spot-specific settings
func (sm *SettingsManager) UpdateSpotSettings(
	enabled bool,
	dryRun bool,
	riskLevel string,
	maxPositions int,
	maxUSDPerPosition float64,
	takeProfitPercent float64,
	stopLossPercent float64,
	minConfidence float64,
) error {
	settings := sm.GetCurrentSettings()

	settings.SpotAutopilotEnabled = enabled
	settings.SpotDryRunMode = dryRun
	if riskLevel != "" {
		settings.SpotRiskLevel = riskLevel
	}
	if maxPositions > 0 {
		settings.SpotMaxPositions = maxPositions
	}
	if maxUSDPerPosition > 0 {
		settings.SpotMaxUSDPerPosition = maxUSDPerPosition
	}
	if takeProfitPercent > 0 {
		settings.SpotTakeProfitPercent = takeProfitPercent
	}
	if stopLossPercent > 0 {
		settings.SpotStopLossPercent = stopLossPercent
	}
	if minConfidence > 0 {
		settings.SpotMinConfidence = minConfidence
	}

	return sm.SaveSettings(settings)
}

// UpdateSpotCircuitBreaker updates spot circuit breaker settings
func (sm *SettingsManager) UpdateSpotCircuitBreaker(
	enabled bool,
	maxLossPerHour float64,
	maxDailyLoss float64,
	maxConsecutiveLosses int,
	cooldownMinutes int,
	maxTradesPerMinute int,
	maxDailyTrades int,
) error {
	settings := sm.GetCurrentSettings()

	settings.SpotCircuitBreakerEnabled = enabled
	if maxLossPerHour > 0 {
		settings.SpotMaxLossPerHour = maxLossPerHour
	}
	if maxDailyLoss > 0 {
		settings.SpotMaxDailyLoss = maxDailyLoss
	}
	if maxConsecutiveLosses > 0 {
		settings.SpotMaxConsecutiveLosses = maxConsecutiveLosses
	}
	if cooldownMinutes > 0 {
		settings.SpotCooldownMinutes = cooldownMinutes
	}
	if maxTradesPerMinute > 0 {
		settings.SpotMaxTradesPerMinute = maxTradesPerMinute
	}
	if maxDailyTrades > 0 {
		settings.SpotMaxDailyTrades = maxDailyTrades
	}

	return sm.SaveSettings(settings)
}

// UpdateSpotCoinPreferences updates spot coin preference lists
func (sm *SettingsManager) UpdateSpotCoinPreferences(blacklist, whitelist []string, useWhitelist bool) error {
	settings := sm.GetCurrentSettings()

	settings.SpotCoinBlacklist = blacklist
	settings.SpotCoinWhitelist = whitelist
	settings.SpotUseWhitelist = useWhitelist

	return sm.SaveSettings(settings)
}

// UpdateSpotPnLStats updates Spot PnL statistics
func (sm *SettingsManager) UpdateSpotPnLStats(
	totalPnL float64,
	dailyPnL float64,
	totalTrades int,
	winningTrades int,
	dailyTrades int,
) error {
	settings := sm.GetCurrentSettings()

	settings.SpotTotalPnL = totalPnL
	settings.SpotDailyPnL = dailyPnL
	settings.SpotTotalTrades = totalTrades
	settings.SpotWinningTrades = winningTrades
	settings.SpotDailyTrades = dailyTrades
	settings.SpotPnLLastUpdate = time.Now().Format("2006-01-02")

	return sm.SaveSettings(settings)
}

// GetSpotPnLStats returns Spot PnL statistics, resetting daily values if date changed
func (sm *SettingsManager) GetSpotPnLStats() (totalPnL, dailyPnL float64, totalTrades, winningTrades, dailyTrades int) {
	settings := sm.GetCurrentSettings()

	today := time.Now().Format("2006-01-02")
	if settings.SpotPnLLastUpdate != today {
		// New day - reset daily counters
		settings.SpotDailyPnL = 0
		settings.SpotDailyTrades = 0
		settings.SpotPnLLastUpdate = today
		sm.SaveSettings(settings)
	}

	return settings.SpotTotalPnL, settings.SpotDailyPnL,
		settings.SpotTotalTrades, settings.SpotWinningTrades, settings.SpotDailyTrades
}

// GetSpotSettings returns all spot settings
func (sm *SettingsManager) GetSpotSettings() map[string]interface{} {
	settings := sm.GetCurrentSettings()

	return map[string]interface{}{
		"enabled":              settings.SpotAutopilotEnabled,
		"dry_run":              settings.SpotDryRunMode,
		"risk_level":           settings.SpotRiskLevel,
		"max_positions":        settings.SpotMaxPositions,
		"max_usd_per_position": settings.SpotMaxUSDPerPosition,
		"take_profit_percent":  settings.SpotTakeProfitPercent,
		"stop_loss_percent":    settings.SpotStopLossPercent,
		"min_confidence":       settings.SpotMinConfidence,
		"circuit_breaker": map[string]interface{}{
			"enabled":                settings.SpotCircuitBreakerEnabled,
			"max_loss_per_hour":      settings.SpotMaxLossPerHour,
			"max_daily_loss":         settings.SpotMaxDailyLoss,
			"max_consecutive_losses": settings.SpotMaxConsecutiveLosses,
			"cooldown_minutes":       settings.SpotCooldownMinutes,
			"max_trades_per_minute":  settings.SpotMaxTradesPerMinute,
			"max_daily_trades":       settings.SpotMaxDailyTrades,
		},
		"coin_preferences": map[string]interface{}{
			"blacklist":     settings.SpotCoinBlacklist,
			"whitelist":     settings.SpotCoinWhitelist,
			"use_whitelist": settings.SpotUseWhitelist,
		},
	}
}

// ============================================================================
// AUTO MODE SETTINGS METHODS (LLM-DRIVEN TRADING)
// ============================================================================

// UpdateAutoModeEnabled toggles auto mode on/off
func (sm *SettingsManager) UpdateAutoModeEnabled(enabled bool) error {
	settings := sm.GetCurrentSettings()
	settings.AutoModeEnabled = enabled
	return sm.SaveSettings(settings)
}

// UpdateAutoModeSettings updates all auto mode settings
func (sm *SettingsManager) UpdateAutoModeSettings(
	enabled bool,
	maxPositions int,
	maxLeverage int,
	maxPositionSize float64,
	maxTotalUSD float64,
	allowAveraging bool,
	maxAverages int,
	minHoldMinutes int,
	quickProfitMode bool,
	minProfitForExit float64,
) error {
	settings := sm.GetCurrentSettings()

	settings.AutoModeEnabled = enabled
	if maxPositions > 0 {
		settings.AutoModeMaxPositions = maxPositions
	}
	if maxLeverage > 0 {
		settings.AutoModeMaxLeverage = maxLeverage
	}
	if maxPositionSize > 0 {
		settings.AutoModeMaxPositionSize = maxPositionSize
	}
	if maxTotalUSD > 0 {
		settings.AutoModeMaxTotalUSD = maxTotalUSD
	}
	settings.AutoModeAllowAveraging = allowAveraging
	if maxAverages >= 0 {
		settings.AutoModeMaxAverages = maxAverages
	}
	if minHoldMinutes >= 0 {
		settings.AutoModeMinHoldMinutes = minHoldMinutes
	}
	settings.AutoModeQuickProfitMode = quickProfitMode
	if minProfitForExit > 0 {
		settings.AutoModeMinProfitForExit = minProfitForExit
	}

	return sm.SaveSettings(settings)
}

// GetAutoModeSettings returns all auto mode settings
func (sm *SettingsManager) GetAutoModeSettings() map[string]interface{} {
	settings := sm.GetCurrentSettings()

	return map[string]interface{}{
		"enabled":            settings.AutoModeEnabled,
		"max_positions":      settings.AutoModeMaxPositions,
		"max_leverage":       settings.AutoModeMaxLeverage,
		"max_position_size":  settings.AutoModeMaxPositionSize,
		"max_total_usd":      settings.AutoModeMaxTotalUSD,
		"allow_averaging":    settings.AutoModeAllowAveraging,
		"max_averages":       settings.AutoModeMaxAverages,
		"min_hold_minutes":   settings.AutoModeMinHoldMinutes,
		"quick_profit_mode":  settings.AutoModeQuickProfitMode,
		"min_profit_for_exit": settings.AutoModeMinProfitForExit,
	}
}

// ============================================================================
// PER-SYMBOL SETTINGS METHODS
// ============================================================================

// GetSymbolSettings returns settings for a specific symbol
func (sm *SettingsManager) GetSymbolSettings(symbol string) *SymbolSettings {
	settings := sm.GetCurrentSettings()

	if settings.SymbolSettings == nil {
		settings.SymbolSettings = make(map[string]*SymbolSettings)
	}

	if s, exists := settings.SymbolSettings[symbol]; exists {
		return s
	}

	// Return default settings for unknown symbol
	return &SymbolSettings{
		Symbol:         symbol,
		Category:       PerformanceNeutral,
		MinConfidence:  0, // Use global
		MaxPositionUSD: 0, // Use global
		SizeMultiplier: 1.0,
		Enabled:        true,
	}
}

// UpdateSymbolROITarget updates the custom ROI% for a symbol
// If roiPercent is 0, removes the custom ROI (reverts to mode defaults)
func (sm *SettingsManager) UpdateSymbolROITarget(symbol string, roiPercent float64) error {
	settings := sm.GetCurrentSettings()

	if settings.SymbolSettings == nil {
		settings.SymbolSettings = make(map[string]*SymbolSettings)
	}

	// Get or create symbol settings
	if settings.SymbolSettings[symbol] == nil {
		settings.SymbolSettings[symbol] = &SymbolSettings{
			Symbol:         symbol,
			Enabled:        true,
			Category:       PerformanceNeutral,
			SizeMultiplier: 1.0,
		}
	}

	settings.SymbolSettings[symbol].CustomROIPercent = roiPercent

	return sm.SaveSettings(settings)
}

// GetEffectiveConfidence returns the effective min confidence for a symbol
// taking into account global settings and per-symbol overrides
func (sm *SettingsManager) GetEffectiveConfidence(symbol string, globalMinConfidence float64) float64 {
	symbolSettings := sm.GetSymbolSettings(symbol)
	settings := sm.GetCurrentSettings()

	// If symbol has explicit override, use it
	if symbolSettings.MinConfidence > 0 {
		return symbolSettings.MinConfidence
	}

	// Otherwise apply category boost to global confidence
	categoryBoost := 0.0
	if settings.CategoryConfidenceBoost != nil {
		if boost, exists := settings.CategoryConfidenceBoost[string(symbolSettings.Category)]; exists {
			categoryBoost = boost
		}
	}

	return globalMinConfidence + categoryBoost
}

// GetEffectivePositionSize returns the effective max position size for a symbol
func (sm *SettingsManager) GetEffectivePositionSize(symbol string, globalMaxUSD float64) float64 {
	symbolSettings := sm.GetSymbolSettings(symbol)
	settings := sm.GetCurrentSettings()

	// If symbol has explicit override, use it
	if symbolSettings.MaxPositionUSD > 0 {
		return symbolSettings.MaxPositionUSD
	}

	// Apply category multiplier and symbol-specific multiplier
	categoryMultiplier := 1.0
	if settings.CategorySizeMultiplier != nil {
		if mult, exists := settings.CategorySizeMultiplier[string(symbolSettings.Category)]; exists {
			categoryMultiplier = mult
		}
	}

	return globalMaxUSD * categoryMultiplier * symbolSettings.SizeMultiplier
}

// IsSymbolEnabled returns whether trading is enabled for a symbol
func (sm *SettingsManager) IsSymbolEnabled(symbol string) bool {
	symbolSettings := sm.GetSymbolSettings(symbol)
	return symbolSettings.Enabled && symbolSettings.Category != PerformanceBlacklist
}

// UpdateSymbolSettings updates settings for a specific symbol
func (sm *SettingsManager) UpdateSymbolSettings(symbol string, update *SymbolSettings) error {
	settings := sm.GetCurrentSettings()

	if settings.SymbolSettings == nil {
		settings.SymbolSettings = make(map[string]*SymbolSettings)
	}

	update.Symbol = symbol
	update.LastUpdated = time.Now().Format("2006-01-02 15:04:05")
	settings.SymbolSettings[symbol] = update

	return sm.SaveSettings(settings)
}

// UpdateSymbolCategory updates just the category for a symbol
func (sm *SettingsManager) UpdateSymbolCategory(symbol string, category SymbolPerformanceCategory) error {
	settings := sm.GetCurrentSettings()

	if settings.SymbolSettings == nil {
		settings.SymbolSettings = make(map[string]*SymbolSettings)
	}

	if _, exists := settings.SymbolSettings[symbol]; !exists {
		settings.SymbolSettings[symbol] = &SymbolSettings{
			Symbol:         symbol,
			SizeMultiplier: 1.0,
			Enabled:        category != PerformanceBlacklist,
		}
	}

	settings.SymbolSettings[symbol].Category = category
	settings.SymbolSettings[symbol].Enabled = category != PerformanceBlacklist
	settings.SymbolSettings[symbol].LastUpdated = time.Now().Format("2006-01-02 15:04:05")

	return sm.SaveSettings(settings)
}

// UpdateSymbolPerformance updates performance metrics for a symbol
func (sm *SettingsManager) UpdateSymbolPerformance(symbol string, totalTrades, winningTrades int, totalPnL float64) error {
	settings := sm.GetCurrentSettings()

	if settings.SymbolSettings == nil {
		settings.SymbolSettings = make(map[string]*SymbolSettings)
	}

	if _, exists := settings.SymbolSettings[symbol]; !exists {
		settings.SymbolSettings[symbol] = &SymbolSettings{
			Symbol:         symbol,
			Category:       PerformanceNeutral,
			SizeMultiplier: 1.0,
			Enabled:        true,
		}
	}

	s := settings.SymbolSettings[symbol]
	s.TotalTrades = totalTrades
	s.WinningTrades = winningTrades
	s.TotalPnL = totalPnL
	if totalTrades > 0 {
		s.WinRate = float64(winningTrades) / float64(totalTrades) * 100
		s.AvgPnL = totalPnL / float64(totalTrades)
	}
	s.LastUpdated = time.Now().Format("2006-01-02 15:04:05")

	// Auto-categorize based on performance
	s.Category = categorizePerformance(totalTrades, s.WinRate, totalPnL, s.AvgPnL)

	// Auto-disable blacklisted symbols
	if s.Category == PerformanceBlacklist {
		s.Enabled = false
	}

	return sm.SaveSettings(settings)
}

// categorizePerformance determines the performance category based on metrics
func categorizePerformance(totalTrades int, winRate, totalPnL, avgPnL float64) SymbolPerformanceCategory {
	// Need minimum trades for meaningful categorization
	if totalTrades < 5 {
		return PerformanceNeutral
	}

	// Blacklist: Very poor performance
	if totalPnL < -20 && winRate < 30 {
		return PerformanceBlacklist
	}

	// Worst: Poor performance that should be restricted
	if totalPnL < -10 && winRate < 40 {
		return PerformanceWorst
	}

	// Poor: Below average
	if totalPnL < 0 || winRate < 45 {
		return PerformancePoor
	}

	// Best: Excellent performance
	if totalPnL > 10 && winRate > 55 {
		return PerformanceBest
	}

	// Good: Above average
	if totalPnL > 5 || winRate > 50 {
		return PerformanceGood
	}

	return PerformanceNeutral
}

// GetAllSymbolSettings returns all symbol settings
func (sm *SettingsManager) GetAllSymbolSettings() map[string]*SymbolSettings {
	settings := sm.GetCurrentSettings()
	if settings.SymbolSettings == nil {
		return make(map[string]*SymbolSettings)
	}
	return settings.SymbolSettings
}

// GetSymbolsByCategory returns all symbols in a given category
func (sm *SettingsManager) GetSymbolsByCategory(category SymbolPerformanceCategory) []string {
	settings := sm.GetCurrentSettings()
	var symbols []string

	if settings.SymbolSettings == nil {
		return symbols
	}

	for symbol, s := range settings.SymbolSettings {
		if s.Category == category {
			symbols = append(symbols, symbol)
		}
	}

	return symbols
}

// GetSymbolPerformanceReport generates a performance report for all symbols
func (sm *SettingsManager) GetSymbolPerformanceReport() []SymbolPerformanceReport {
	settings := sm.GetCurrentSettings()
	var reports []SymbolPerformanceReport

	if settings.SymbolSettings == nil {
		return reports
	}

	for _, s := range settings.SymbolSettings {
		report := SymbolPerformanceReport{
			Symbol:         s.Symbol,
			Category:       string(s.Category),
			TotalTrades:    s.TotalTrades,
			WinningTrades:  s.WinningTrades,
			LosingTrades:   s.TotalTrades - s.WinningTrades,
			TotalPnL:       s.TotalPnL,
			WinRate:        s.WinRate,
			AvgPnL:         s.AvgPnL,
			MinConfidence:  sm.GetEffectiveConfidence(s.Symbol, settings.GinieMinConfidence),
			MaxPositionUSD: sm.GetEffectivePositionSize(s.Symbol, settings.GinieMaxUSD),
			SizeMultiplier: s.SizeMultiplier,
			Enabled:        s.Enabled,
		}
		reports = append(reports, report)
	}

	return reports
}

// BlacklistSymbol adds a symbol to the blacklist
func (sm *SettingsManager) BlacklistSymbol(symbol string, reason string) error {
	settings := sm.GetCurrentSettings()

	if settings.SymbolSettings == nil {
		settings.SymbolSettings = make(map[string]*SymbolSettings)
	}

	if _, exists := settings.SymbolSettings[symbol]; !exists {
		settings.SymbolSettings[symbol] = &SymbolSettings{
			Symbol:         symbol,
			SizeMultiplier: 1.0,
		}
	}

	settings.SymbolSettings[symbol].Category = PerformanceBlacklist
	settings.SymbolSettings[symbol].Enabled = false
	settings.SymbolSettings[symbol].Notes = reason
	settings.SymbolSettings[symbol].LastUpdated = time.Now().Format("2006-01-02 15:04:05")

	return sm.SaveSettings(settings)
}

// UnblacklistSymbol removes a symbol from the blacklist
func (sm *SettingsManager) UnblacklistSymbol(symbol string) error {
	settings := sm.GetCurrentSettings()

	if settings.SymbolSettings == nil || settings.SymbolSettings[symbol] == nil {
		return nil
	}

	settings.SymbolSettings[symbol].Category = PerformanceNeutral
	settings.SymbolSettings[symbol].Enabled = true
	settings.SymbolSettings[symbol].LastUpdated = time.Now().Format("2006-01-02 15:04:05")

	return sm.SaveSettings(settings)
}

// GetCategorySettings returns the confidence boost and size multiplier for each category
func (sm *SettingsManager) GetCategorySettings() map[string]map[string]float64 {
	settings := sm.GetCurrentSettings()

	return map[string]map[string]float64{
		"confidence_boost": settings.CategoryConfidenceBoost,
		"size_multiplier":  settings.CategorySizeMultiplier,
	}
}

// UpdateCategorySettings updates the default adjustments for categories
func (sm *SettingsManager) UpdateCategorySettings(confidenceBoost, sizeMultiplier map[string]float64) error {
	settings := sm.GetCurrentSettings()

	if confidenceBoost != nil {
		settings.CategoryConfidenceBoost = confidenceBoost
	}
	if sizeMultiplier != nil {
		settings.CategorySizeMultiplier = sizeMultiplier
	}

	return sm.SaveSettings(settings)
}

// ============================================================================
// ULTRA-FAST SCALPING SETTINGS METHODS
// ============================================================================

// UpdateUltraFastSettings updates all ultra-fast settings
func (sm *SettingsManager) UpdateUltraFastSettings(
	enabled bool,
	scanInterval int,
	monitorInterval int,
	maxPositions int,
	maxUSDPerPos float64,
	minConfidence float64,
	maxDailyTrades int,
) error {
	settings := sm.GetCurrentSettings()

	settings.UltraFastEnabled = enabled
	if scanInterval > 0 {
		settings.UltraFastScanInterval = scanInterval
	}
	if monitorInterval > 0 {
		settings.UltraFastMonitorInterval = monitorInterval
	}
	if maxPositions > 0 {
		settings.UltraFastMaxPositions = maxPositions
	}
	if maxUSDPerPos > 0 {
		settings.UltraFastMaxUSDPerPos = maxUSDPerPos
	}
	if minConfidence > 0 {
		settings.UltraFastMinConfidence = minConfidence
	}
	if maxDailyTrades > 0 {
		settings.UltraFastMaxDailyTrades = maxDailyTrades
	}

	return sm.SaveSettings(settings)
}

// GetUltraFastSettings returns all ultra-fast settings as a map
func (sm *SettingsManager) GetUltraFastSettings() map[string]interface{} {
	settings := sm.GetCurrentSettings()

	return map[string]interface{}{
		"enabled":          settings.UltraFastEnabled,
		"scan_interval":    settings.UltraFastScanInterval,
		"monitor_interval": settings.UltraFastMonitorInterval,
		"max_positions":    settings.UltraFastMaxPositions,
		"max_usd_per_pos":  settings.UltraFastMaxUSDPerPos,
		"min_confidence":   settings.UltraFastMinConfidence,
		"max_daily_trades": settings.UltraFastMaxDailyTrades,
		"today_trades":     settings.UltraFastTodayTrades,
		"daily_pnl":        settings.UltraFastDailyPnL,
		"total_pnl":        settings.UltraFastTotalPnL,
		"win_rate":         settings.UltraFastWinRate,
	}
}

// UpdateUltraFastStats updates ultra-fast trading statistics
func (sm *SettingsManager) UpdateUltraFastStats(
	dailyPnL float64,
	totalPnL float64,
	dailyTrades int,
	winRate float64,
) error {
	settings := sm.GetCurrentSettings()

	settings.UltraFastDailyPnL = dailyPnL
	settings.UltraFastTotalPnL = totalPnL
	settings.UltraFastTodayTrades = dailyTrades
	settings.UltraFastWinRate = winRate
	settings.UltraFastLastUpdate = time.Now().Format("2006-01-02")

	return sm.SaveSettings(settings)
}

// ResetUltraFastDaily resets daily statistics (called at start of day)
func (sm *SettingsManager) ResetUltraFastDaily() error {
	settings := sm.GetCurrentSettings()

	today := time.Now().Format("2006-01-02")
	if settings.UltraFastLastUpdate != today {
		// New day - reset daily counters
		settings.UltraFastDailyPnL = 0
		settings.UltraFastTodayTrades = 0
		settings.UltraFastLastUpdate = today
		return sm.SaveSettings(settings)
	}

	return nil
}

// IncrementUltraFastTrade increments the today's trade count
func (sm *SettingsManager) IncrementUltraFastTrade() error {
	settings := sm.GetCurrentSettings()

	today := time.Now().Format("2006-01-02")
	if settings.UltraFastLastUpdate != today {
		settings.UltraFastDailyPnL = 0
		settings.UltraFastTodayTrades = 0
		settings.UltraFastLastUpdate = today
	}

	settings.UltraFastTodayTrades++
	return sm.SaveSettings(settings)
}

// === PER-MODE CAPITAL ALLOCATION METHODS ===

// GetModeAllocation returns the current mode allocation configuration
func (sm *SettingsManager) GetModeAllocation() *ModeAllocationConfig {
	settings := sm.GetCurrentSettings()
	if settings.ModeAllocation == nil {
		// Return default if not set
		return DefaultSettings().ModeAllocation
	}
	return settings.ModeAllocation
}

// UpdateModeAllocation updates the mode allocation configuration
func (sm *SettingsManager) UpdateModeAllocation(config *ModeAllocationConfig) error {
	settings := sm.GetCurrentSettings()

	// Validate that allocation percentages sum to 100%
	total := config.UltraFastScalpPercent + config.ScalpPercent + config.SwingPercent + config.PositionPercent
	if total < 99.0 || total > 101.0 { // Allow 1% rounding error
		return fmt.Errorf("mode allocation percentages must sum to 100%%, got %.1f%%", total)
	}

	settings.ModeAllocation = config
	return sm.SaveSettings(settings)
}

// GetModeAllocationState calculates the current allocation state for a mode
func (sm *SettingsManager) GetModeAllocationState(mode string, totalCapital float64, currentPositions map[string]int, currentUsedUSD map[string]float64) *ModeAllocationState {
	config := sm.GetModeAllocation()

	var allocatedPercent, maxPositions float64

	// Get mode-specific allocation
	switch mode {
	case "ultra_fast":
		allocatedPercent = config.UltraFastScalpPercent
		maxPositions = float64(config.MaxUltraFastPositions)
	case "scalp":
		allocatedPercent = config.ScalpPercent
		maxPositions = float64(config.MaxScalpPositions)
	case "swing":
		allocatedPercent = config.SwingPercent
		maxPositions = float64(config.MaxSwingPositions)
	case "position":
		allocatedPercent = config.PositionPercent
		maxPositions = float64(config.MaxPositionPositions)
	default:
		allocatedPercent = 0
	}

	// Calculate allocation
	allocatedUSD := totalCapital * (allocatedPercent / 100.0)
	usedUSD := 0.0
	if val, exists := currentUsedUSD[mode]; exists {
		usedUSD = val
	}
	availableUSD := allocatedUSD - usedUSD

	// Get current position count
	posCount := 0
	if val, exists := currentPositions[mode]; exists {
		posCount = val
	}

	// Calculate utilization percentages
	var capitalUtil, posUtil float64
	if allocatedUSD > 0 {
		capitalUtil = (usedUSD / allocatedUSD) * 100.0
	}
	if maxPositions > 0 {
		posUtil = (float64(posCount) / maxPositions) * 100.0
	}

	return &ModeAllocationState{
		Mode:                mode,
		AllocatedPercent:    allocatedPercent,
		AllocatedUSD:        allocatedUSD,
		UsedUSD:             usedUSD,
		AvailableUSD:        availableUSD,
		CurrentPositions:    posCount,
		MaxPositions:        int(maxPositions),
		CapitalUtilization:  capitalUtil,
		PositionUtilization: posUtil,
		LastAllocation:      time.Now(),
	}
}
