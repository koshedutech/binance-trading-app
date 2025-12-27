package autopilot

import (
	"encoding/json"
	"fmt"
	"log"
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

// ====== COMPREHENSIVE MODE-SPECIFIC CONFIGURATION (Story 2.7) ======
// Each mode (ultra_fast, scalp, swing, position) has independent settings
// All defaults are from Story 2.7 and can be customized by users

// ModeTimeframeConfig holds timeframe settings for a mode
type ModeTimeframeConfig struct {
	TrendTimeframe    string `json:"trend_timeframe"`    // Higher TF for trend (e.g., "5m", "15m", "1h", "4h")
	EntryTimeframe    string `json:"entry_timeframe"`    // Signal timing TF (e.g., "1m", "5m", "15m", "1h")
	AnalysisTimeframe string `json:"analysis_timeframe"` // Pattern detection TF (e.g., "1m", "15m", "4h", "1d")
}

// ModeConfidenceConfig holds confidence thresholds for a mode
type ModeConfidenceConfig struct {
	MinConfidence   float64 `json:"min_confidence"`   // Minimum to enter (e.g., 50, 60, 65, 75)
	HighConfidence  float64 `json:"high_confidence"`  // Threshold for size multiplier (e.g., 70, 75, 80, 85)
	UltraConfidence float64 `json:"ultra_confidence"` // Threshold for max size (e.g., 85, 88, 90, 92)
}

// ModeSizeConfig holds position sizing settings for a mode
type ModeSizeConfig struct {
	BaseSizeUSD      float64 `json:"base_size_usd"`       // Base position size (e.g., $100, $200, $400, $600)
	MaxSizeUSD       float64 `json:"max_size_usd"`        // Max with multiplier (e.g., $200, $400, $750, $1000)
	MaxPositions     int     `json:"max_positions"`       // Max concurrent (e.g., 5, 4, 3, 2)
	Leverage         int     `json:"leverage"`            // Default leverage (e.g., 10, 8, 5, 3)
	SizeMultiplierLo float64 `json:"size_multiplier_lo"`  // Min multiplier (1.0)
	SizeMultiplierHi float64 `json:"size_multiplier_hi"`  // Max multiplier on high conf (e.g., 1.5, 1.8, 2.0, 2.5)

	// Position sizing fallbacks (Story 5: Mode-specific position sizing)
	SafetyMargin               float64 `json:"safety_margin"`                  // Default: 0.90 - Reserve 10% of balance for safety
	MinBalanceUSD              float64 `json:"min_balance_usd"`                // Default: 25.0 - Minimum balance required to trade
	MinPositionSizeUSD         float64 `json:"min_position_size_usd"`          // Default: 10.0 - Minimum position size
	RiskMultiplierConservative float64 `json:"risk_multiplier_conservative"`   // Default: 0.6 - Conservative risk scaling
	RiskMultiplierModerate     float64 `json:"risk_multiplier_moderate"`       // Default: 0.8 - Moderate risk scaling
	RiskMultiplierAggressive   float64 `json:"risk_multiplier_aggressive"`     // Default: 1.0 - Aggressive risk scaling
	ConfidenceMultiplierBase   float64 `json:"confidence_multiplier_base"`     // Default: 0.5 - Base multiplier for confidence scaling
	ConfidenceMultiplierScale  float64 `json:"confidence_multiplier_scale"`    // Default: 0.7 - Additional multiplier per confidence level
}

// ModeCircuitBreakerConfig holds circuit breaker settings for a mode
type ModeCircuitBreakerConfig struct {
	MaxLossPerHour       float64 `json:"max_loss_per_hour"`       // e.g., $20, $40, $80, $150
	MaxLossPerDay        float64 `json:"max_loss_per_day"`        // e.g., $50, $100, $200, $400
	MaxConsecutiveLosses int     `json:"max_consecutive_losses"`  // e.g., 3, 5, 7, 10
	CooldownMinutes      int     `json:"cooldown_minutes"`        // e.g., 15, 30, 60, 120
	MaxTradesPerMinute   int     `json:"max_trades_per_minute"`   // e.g., 5, 3, 2, 1
	MaxTradesPerHour     int     `json:"max_trades_per_hour"`     // e.g., 30, 20, 10, 5
	MaxTradesPerDay      int     `json:"max_trades_per_day"`      // e.g., 100, 50, 20, 10
	WinRateCheckAfter    int     `json:"win_rate_check_after"`    // Trades before evaluation (e.g., 10, 15, 20, 25)
	MinWinRate           float64 `json:"min_win_rate"`            // Threshold % (e.g., 45, 50, 55, 60)
}

// ModeSLTPConfig holds SL/TP settings for a mode
type ModeSLTPConfig struct {
	StopLossPercent         float64   `json:"stop_loss_percent"`          // Default SL % (e.g., 1.0, 1.5, 2.5, 3.5)
	TakeProfitPercent       float64   `json:"take_profit_percent"`        // Default TP % (e.g., 2.0, 3.0, 5.0, 8.0)
	TrailingStopEnabled     bool      `json:"trailing_stop_enabled"`      // Enable trailing (e.g., false, false, true, true)
	TrailingStopPercent     float64   `json:"trailing_stop_percent"`      // Trail distance (e.g., N/A, 0.5, 1.5, 2.5)
	TrailingStopActivation  float64   `json:"trailing_stop_activation"`   // Activate at profit % (e.g., N/A, 0.5, 1.0, 2.0)
	TrailingActivationPrice float64   `json:"trailing_activation_price"`  // Activate at specific price (0 = disabled)
	MaxHoldDuration         string    `json:"max_hold_duration"`          // Force exit after (e.g., "3s", "4h", "3d", "14d")
	UseSingleTP             bool      `json:"use_single_tp"`              // true = 100% at TP, false = multi-level
	SingleTPPercent         float64   `json:"single_tp_percent"`          // Gain % for single TP mode (e.g., 3.0 = 3%)
	TPGainLevels            []float64 `json:"tp_gain_levels"`             // Multi-level TP price gains (e.g., [0.3, 0.6, 1.0, 1.5])
	TPAllocation            []float64 `json:"tp_allocation"`              // Multi-level TP qty allocation (e.g., [50, 50, 0, 0] = 50% at TP1, 50% at TP2)
	TrailingActivationMode  string    `json:"trailing_activation_mode"`   // "immediate", "after_tp1", "after_breakeven", "after_tp1_and_breakeven"
	// ROI-based SL/TP settings
	UseROIBasedSLTP      bool    `json:"use_roi_based_sltp"`       // true = use ROI-based SL/TP instead of price %
	ROIStopLossPercent   float64 `json:"roi_stop_loss_percent"`    // SL based on ROI % (e.g., -5 = close at -5% ROI)
	ROITakeProfitPercent float64 `json:"roi_take_profit_percent"`  // TP based on ROI % (e.g., 10 = close at +10% ROI)
	// Margin type settings
	MarginType            string  `json:"margin_type"`              // "CROSS" or "ISOLATED" (default: "CROSS")
	IsolatedMarginPercent float64 `json:"isolated_margin_percent"`  // Margin % for isolated mode (10-100%)
	// ATR-based SL/TP settings (for LLM/ATR blending)
	ATRSLMultiplier float64 `json:"atr_sl_multiplier"` // ATR multiplier for stop-loss
	ATRTPMultiplier float64 `json:"atr_tp_multiplier"` // ATR multiplier for take-profit
	ATRSLMin        float64 `json:"atr_sl_min"`        // Min SL distance (ATR-based)
	ATRSLMax        float64 `json:"atr_sl_max"`        // Max SL distance (ATR-based)
	ATRTPMin        float64 `json:"atr_tp_min"`        // Min TP distance (ATR-based)
	ATRTPMax        float64 `json:"atr_tp_max"`        // Max TP distance (ATR-based)
	LLMWeight       float64 `json:"llm_weight"`        // Weight for LLM-suggested SL/TP
	ATRWeight       float64 `json:"atr_weight"`        // Weight for ATR-calculated SL/TP
}

// HedgeModeConfig holds hedge mode settings for a mode (LONG + SHORT simultaneously)
type HedgeModeConfig struct {
	AllowHedge                bool    `json:"allow_hedge"`                  // Enable hedging for this mode
	MinConfidenceForHedge     float64 `json:"min_confidence_for_hedge"`     // Min confidence to open hedge (e.g., 70, 75, 80, 85)
	ExistingMustBeInProfit    float64 `json:"existing_must_be_in_profit"`   // Existing position profit threshold (e.g., 0, 0, 1, 2 %)
	MaxHedgeSizePercent       float64 `json:"max_hedge_size_percent"`       // Max hedge size vs original (e.g., 100, 75, 50, 50 %)
	AllowSameModeHedge        bool    `json:"allow_same_mode_hedge"`        // Allow hedge within same mode (default: false)
	MaxTotalExposureMultiplier float64 `json:"max_total_exposure_multiplier"` // Cap total exposure (default: 2.0 = 2x normal)
}

// PositionAveragingConfig holds position averaging settings for a mode
type PositionAveragingConfig struct {
	AllowAveraging         bool    `json:"allow_averaging"`           // Enable averaging for this mode
	AverageUpProfitPercent float64 `json:"average_up_profit_percent"` // Add when profit exceeds (e.g., 0.5, 1.0, 2.0 %)
	AverageDownLossPercent float64 `json:"average_down_loss_percent"` // Add when loss within (e.g., -1, -1.5, -2 %)
	AddSizePercent         float64 `json:"add_size_percent"`          // Size to add (e.g., 50, 50, 30 % of original)
	MaxAverages            int     `json:"max_averages"`              // Max times to average (e.g., 0, 2, 3, 2)
	MinConfidenceForAverage float64 `json:"min_confidence_for_average"` // Min confidence for new signal
	UseLLMForAveraging     bool    `json:"use_llm_for_averaging"`     // Use LLM to decide if averaging is wise
}

// StalePositionReleaseConfig holds stale position release (capital liberation) settings
type StalePositionReleaseConfig struct {
	Enabled              bool    `json:"enabled"`                 // Enable stale position release
	MaxHoldDuration      string  `json:"max_hold_duration"`       // Max hold time (e.g., "10s", "6h", "5d", "21d")
	MinProfitToKeep      float64 `json:"min_profit_to_keep"`      // Min profit % to keep position (e.g., 0.3, 0.5, 1.0, 2.0)
	MaxLossToForceClose  float64 `json:"max_loss_to_force_close"` // Max loss % to force close (e.g., -0.5, -1.0, -1.5, -2.0)
	StaleZoneLo          float64 `json:"stale_zone_lo"`           // Stale zone lower bound (e.g., -0.3, -0.5, -1.0, -1.5)
	StaleZoneHi          float64 `json:"stale_zone_hi"`           // Stale zone upper bound (e.g., +0.3, +0.5, +1.0, +1.5)
	StaleZoneCloseAction string  `json:"stale_zone_close_action"` // Action: "close" or "reduce_50" or "wait_signal"
}

// ModeAssignmentConfig holds mode assignment criteria from analyzer
type ModeAssignmentConfig struct {
	VolatilityMin       string  `json:"volatility_min"`        // "low", "medium", "high", "extreme"
	VolatilityMax       string  `json:"volatility_max"`        // "low", "medium", "high", "extreme"
	ExpectedHoldMin     string  `json:"expected_hold_min"`     // Min expected hold (e.g., "0", "15m", "4h", "3d")
	ExpectedHoldMax     string  `json:"expected_hold_max"`     // Max expected hold (e.g., "5m", "4h", "3d", "30d")
	ConfidenceMin       float64 `json:"confidence_min"`        // Min confidence for assignment (e.g., 50, 60, 65, 75)
	ConfidenceMax       float64 `json:"confidence_max"`        // Max confidence (e.g., 70, 75, 85, 100)
	RiskScoreMax        int     `json:"risk_score_max"`        // Max risk score to allow (e.g., 50, 45, 40, 30)
	ProfitPotentialMin  float64 `json:"profit_potential_min"`  // Min profit % potential (e.g., 0.5, 1, 3, 5)
	ProfitPotentialMax  float64 `json:"profit_potential_max"`  // Max profit % (e.g., 2, 3, 8, 15)
	RequiresTrendAlign  bool    `json:"requires_trend_align"`  // Require trend alignment
	PriorityWeight      float64 `json:"priority_weight"`       // For conflict resolution (e.g., 0.8, 1.0, 1.2, 1.5)
}

// ModeFundingRateConfig holds funding rate awareness settings for a mode
type ModeFundingRateConfig struct {
	Enabled               bool    `json:"enabled"`                  // Enable funding rate awareness for this mode
	MaxFundingRate        float64 `json:"max_funding_rate"`         // Max funding rate threshold (e.g., 0.001 = 0.1%)
	ExitTimeMinutes       int     `json:"exit_time_minutes"`        // Minutes before funding to consider exit
	FeeThresholdPercent   float64 `json:"fee_threshold_percent"`    // Exit if fee > this % of profit
	ExtremeFundingRate    float64 `json:"extreme_funding_rate"`     // Extreme rate threshold for forced exit
	HighRateReduction     float64 `json:"high_rate_reduction"`      // Size reduction multiplier for high rates
	ElevatedRateReduction float64 `json:"elevated_rate_reduction"`  // Size reduction multiplier for elevated rates
	BlockTimeMinutes      int     `json:"block_time_minutes"`       // Minutes before funding to block new trades
}

// ModeRiskConfig holds risk management settings for a mode
type ModeRiskConfig struct {
	RiskLevel                  string  `json:"risk_level"`                   // "conservative"/"moderate"/"aggressive"
	RiskMultiplierConservative float64 `json:"risk_multiplier_conservative"` // 0.6
	RiskMultiplierModerate     float64 `json:"risk_multiplier_moderate"`     // 0.8
	RiskMultiplierAggressive   float64 `json:"risk_multiplier_aggressive"`   // 1.0
	MaxDrawdownPercent         float64 `json:"max_drawdown_percent"`         // Max allowed drawdown
	MaxDailyLossPercent        float64 `json:"max_daily_loss_percent"`       // Max daily loss limit
}

// ModeTrendDivergenceConfig holds trend divergence detection settings
type ModeTrendDivergenceConfig struct {
	Enabled              bool    `json:"enabled"`                // Enable trend divergence checks
	MinDivergencePercent float64 `json:"min_divergence_percent"` // Min divergence to detect
	BlockOnDivergence    bool    `json:"block_on_divergence"`    // Block trades on strong divergence
	DivergenceWeight     float64 `json:"divergence_weight"`      // Weight in signal scoring
}

// ModeFullConfig holds ALL settings for a single trading mode
type ModeFullConfig struct {
	ModeName       string `json:"mode_name"` // "ultra_fast", "scalp", "swing", "position"
	Enabled        bool   `json:"enabled"`   // Enable this mode

	// Sub-configurations
	Timeframe       *ModeTimeframeConfig        `json:"timeframe"`
	Confidence      *ModeConfidenceConfig       `json:"confidence"`
	Size            *ModeSizeConfig             `json:"size"`
	CircuitBreaker  *ModeCircuitBreakerConfig   `json:"circuit_breaker"`
	SLTP            *ModeSLTPConfig             `json:"sltp"`
	Hedge           *HedgeModeConfig            `json:"hedge"`
	Averaging       *PositionAveragingConfig    `json:"averaging"`
	StaleRelease    *StalePositionReleaseConfig `json:"stale_release"`
	Assignment      *ModeAssignmentConfig       `json:"assignment"`
	FundingRate     *ModeFundingRateConfig      `json:"funding_rate"`
	Risk            *ModeRiskConfig             `json:"risk"`
	TrendDivergence *ModeTrendDivergenceConfig  `json:"trend_divergence"`
}

// ====== LLM AND ADAPTIVE AI CONFIGURATION (Story 2.8) ======

// LLMConfig holds global LLM provider settings
type LLMConfig struct {
	Enabled          bool   `json:"enabled"`
	Provider         string `json:"provider"`           // deepseek, claude, openai, local
	Model            string `json:"model"`              // deepseek-chat, claude-3-haiku, gpt-4o-mini
	FallbackProvider string `json:"fallback_provider"`
	FallbackModel    string `json:"fallback_model"`
	TimeoutMs        int    `json:"timeout_ms"`         // default 5000
	RetryCount       int    `json:"retry_count"`        // default 2
	CacheDurationSec int    `json:"cache_duration_sec"` // default 300
}

// ModeLLMSettings holds per-mode LLM settings
type ModeLLMSettings struct {
	LLMEnabled          bool    `json:"llm_enabled"`
	LLMWeight           float64 `json:"llm_weight"`            // 0.0-1.0
	SkipOnTimeout       bool    `json:"skip_on_timeout"`
	MinLLMConfidence    int     `json:"min_llm_confidence"`    // 0-100
	BlockOnDisagreement bool    `json:"block_on_disagreement"`
	CacheEnabled        bool    `json:"cache_enabled"`
}

// AdaptiveAIConfig holds adaptive AI learning settings
type AdaptiveAIConfig struct {
	Enabled                  bool `json:"enabled"`
	LearningWindowTrades     int  `json:"learning_window_trades"`      // default 50
	LearningWindowHours      int  `json:"learning_window_hours"`       // default 24
	AutoAdjustEnabled        bool `json:"auto_adjust_enabled"`
	MaxAutoAdjustmentPercent int  `json:"max_auto_adjustment_percent"` // default 10
	RequireApproval          bool `json:"require_approval"`
	MinTradesForLearning     int  `json:"min_trades_for_learning"`     // default 20
	StoreDecisionContext     bool `json:"store_decision_context"`
}

// DefaultLLMConfig returns the default LLM configuration
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		Enabled:          true,
		Provider:         "deepseek",
		Model:            "deepseek-chat",
		FallbackProvider: "claude",
		FallbackModel:    "claude-3-haiku",
		TimeoutMs:        5000,
		RetryCount:       2,
		CacheDurationSec: 300,
	}
}

// DefaultModeLLMSettings returns the default per-mode LLM settings for all modes
func DefaultModeLLMSettings() map[GinieTradingMode]ModeLLMSettings {
	return map[GinieTradingMode]ModeLLMSettings{
		GinieModeUltraFast: {
			LLMEnabled:          true,
			LLMWeight:           0.10,
			SkipOnTimeout:       true,
			MinLLMConfidence:    40,
			BlockOnDisagreement: false,
			CacheEnabled:        true,
		},
		GinieModeScalp: {
			LLMEnabled:          true,
			LLMWeight:           0.20,
			SkipOnTimeout:       true,
			MinLLMConfidence:    50,
			BlockOnDisagreement: false,
			CacheEnabled:        true,
		},
		GinieModeSwing: {
			LLMEnabled:          true,
			LLMWeight:           0.40,
			SkipOnTimeout:       false,
			MinLLMConfidence:    60,
			BlockOnDisagreement: true,
			CacheEnabled:        false,
		},
		GinieModePosition: {
			LLMEnabled:          true,
			LLMWeight:           0.50,
			SkipOnTimeout:       false,
			MinLLMConfidence:    65,
			BlockOnDisagreement: true,
			CacheEnabled:        false,
		},
	}
}

// DefaultAdaptiveAIConfig returns the default adaptive AI configuration
func DefaultAdaptiveAIConfig() AdaptiveAIConfig {
	return AdaptiveAIConfig{
		Enabled:                  true,
		LearningWindowTrades:     50,
		LearningWindowHours:      24,
		AutoAdjustEnabled:        false,
		MaxAutoAdjustmentPercent: 10,
		RequireApproval:          true,
		MinTradesForLearning:     20,
		StoreDecisionContext:     true,
	}
}

// DefaultModeConfigs returns the default configurations for all 4 modes (Story 2.7 defaults)
func DefaultModeConfigs() map[string]*ModeFullConfig {
	return map[string]*ModeFullConfig{
		"ultra_fast": {
			ModeName: "ultra_fast",
			Enabled:  true,
			Timeframe: &ModeTimeframeConfig{
				TrendTimeframe:    "5m",
				EntryTimeframe:    "1m",
				AnalysisTimeframe: "1m",
			},
			Confidence: &ModeConfidenceConfig{
				MinConfidence:   50.0,
				HighConfidence:  70.0,
				UltraConfidence: 85.0,
			},
			Size: &ModeSizeConfig{
				BaseSizeUSD:      100.0,
				MaxSizeUSD:       200.0,
				MaxPositions:     5,
				Leverage:         10,
				SizeMultiplierLo: 1.0,
				SizeMultiplierHi: 1.5,
			},
			CircuitBreaker: &ModeCircuitBreakerConfig{
				MaxLossPerHour:       20.0,
				MaxLossPerDay:        50.0,
				MaxConsecutiveLosses: 3,
				CooldownMinutes:      15,
				MaxTradesPerMinute:   5,
				MaxTradesPerHour:     30,
				MaxTradesPerDay:      100,
				WinRateCheckAfter:    10,
				MinWinRate:           45.0,
			},
			SLTP: &ModeSLTPConfig{
				StopLossPercent:         1.0,
				TakeProfitPercent:       2.0,
				TrailingStopEnabled:     false,
				TrailingStopPercent:     0,
				TrailingStopActivation:  0,
				TrailingActivationPrice: 0,
				MaxHoldDuration:         "3s",
				UseSingleTP:             true,
				SingleTPPercent:         2.0,
				TPAllocation:            []float64{100, 0, 0, 0}, // 100% at TP1 (single TP mode)
				TrailingActivationMode:  "immediate",
				UseROIBasedSLTP:         false,
				ROIStopLossPercent:      -5,
				ROITakeProfitPercent:    10,
				MarginType:              "CROSS",
				IsolatedMarginPercent:   25,
			},
			Hedge: &HedgeModeConfig{
				AllowHedge:                true,
				MinConfidenceForHedge:     70.0,
				ExistingMustBeInProfit:    0, // Any
				MaxHedgeSizePercent:       100.0,
				AllowSameModeHedge:        false,
				MaxTotalExposureMultiplier: 2.0,
			},
			Averaging: &PositionAveragingConfig{
				AllowAveraging:         false, // Ultra-fast: NO averaging
				AverageUpProfitPercent: 0,
				AverageDownLossPercent: 0,
				AddSizePercent:         0,
				MaxAverages:            0,
				MinConfidenceForAverage: 0,
				UseLLMForAveraging:     false,
			},
			StaleRelease: &StalePositionReleaseConfig{
				Enabled:              true,
				MaxHoldDuration:      "10s",
				MinProfitToKeep:      0.3,
				MaxLossToForceClose:  -0.5,
				StaleZoneLo:          -0.3,
				StaleZoneHi:          0.3,
				StaleZoneCloseAction: "close",
			},
			Assignment: &ModeAssignmentConfig{
				VolatilityMin:       "high",
				VolatilityMax:       "extreme",
				ExpectedHoldMin:     "0",
				ExpectedHoldMax:     "5m",
				ConfidenceMin:       50.0,
				ConfidenceMax:       70.0,
				RiskScoreMax:        50,
				ProfitPotentialMin:  0.5,
				ProfitPotentialMax:  2.0,
				RequiresTrendAlign:  false,
				PriorityWeight:      0.8,
			},
		},
		"scalp": {
			ModeName: "scalp",
			Enabled:  true,
			Timeframe: &ModeTimeframeConfig{
				TrendTimeframe:    "15m",
				EntryTimeframe:    "5m",
				AnalysisTimeframe: "15m",
			},
			Confidence: &ModeConfidenceConfig{
				MinConfidence:   60.0,
				HighConfidence:  75.0,
				UltraConfidence: 88.0,
			},
			Size: &ModeSizeConfig{
				BaseSizeUSD:      200.0,
				MaxSizeUSD:       400.0,
				MaxPositions:     4,
				Leverage:         8,
				SizeMultiplierLo: 1.0,
				SizeMultiplierHi: 1.8,
			},
			CircuitBreaker: &ModeCircuitBreakerConfig{
				MaxLossPerHour:       40.0,
				MaxLossPerDay:        100.0,
				MaxConsecutiveLosses: 5,
				CooldownMinutes:      30,
				MaxTradesPerMinute:   3,
				MaxTradesPerHour:     20,
				MaxTradesPerDay:      50,
				WinRateCheckAfter:    15,
				MinWinRate:           50.0,
			},
			SLTP: &ModeSLTPConfig{
				StopLossPercent:         1.5,
				TakeProfitPercent:       3.0,
				TrailingStopEnabled:     false,
				TrailingStopPercent:     0.5,
				TrailingStopActivation:  0.5,
				TrailingActivationPrice: 0,
				MaxHoldDuration:         "4h",
				UseSingleTP:             true,
				SingleTPPercent:         3.0,
				TPAllocation:            []float64{100, 0, 0, 0}, // 100% at TP1 (single TP mode)
				TrailingActivationMode:  "after_tp1",
				UseROIBasedSLTP:         false,
				ROIStopLossPercent:      -8,
				ROITakeProfitPercent:    15,
				MarginType:              "CROSS",
				IsolatedMarginPercent:   30,
			},
			Hedge: &HedgeModeConfig{
				AllowHedge:                true,
				MinConfidenceForHedge:     75.0,
				ExistingMustBeInProfit:    0.0, // > 0%
				MaxHedgeSizePercent:       75.0,
				AllowSameModeHedge:        false,
				MaxTotalExposureMultiplier: 2.0,
			},
			Averaging: &PositionAveragingConfig{
				AllowAveraging:         true,
				AverageUpProfitPercent: 0.5, // Add when profit > 0.5%
				AverageDownLossPercent: -1.0, // Or loss < -1%
				AddSizePercent:         50.0, // Add 50% of original
				MaxAverages:            2,
				MinConfidenceForAverage: 65.0,
				UseLLMForAveraging:     true,
			},
			StaleRelease: &StalePositionReleaseConfig{
				Enabled:              true,
				MaxHoldDuration:      "6h",
				MinProfitToKeep:      0.5,
				MaxLossToForceClose:  -1.0,
				StaleZoneLo:          -0.5,
				StaleZoneHi:          0.5,
				StaleZoneCloseAction: "close",
			},
			Assignment: &ModeAssignmentConfig{
				VolatilityMin:       "medium",
				VolatilityMax:       "high",
				ExpectedHoldMin:     "15m",
				ExpectedHoldMax:     "4h",
				ConfidenceMin:       60.0,
				ConfidenceMax:       75.0,
				RiskScoreMax:        45,
				ProfitPotentialMin:  1.0,
				ProfitPotentialMax:  3.0,
				RequiresTrendAlign:  false,
				PriorityWeight:      1.0,
			},
		},
		"swing": {
			ModeName: "swing",
			Enabled:  true,
			Timeframe: &ModeTimeframeConfig{
				TrendTimeframe:    "1h",
				EntryTimeframe:    "15m",
				AnalysisTimeframe: "4h",
			},
			Confidence: &ModeConfidenceConfig{
				MinConfidence:   65.0,
				HighConfidence:  80.0,
				UltraConfidence: 90.0,
			},
			Size: &ModeSizeConfig{
				BaseSizeUSD:      400.0,
				MaxSizeUSD:       750.0,
				MaxPositions:     3,
				Leverage:         5,
				SizeMultiplierLo: 1.0,
				SizeMultiplierHi: 2.0,
			},
			CircuitBreaker: &ModeCircuitBreakerConfig{
				MaxLossPerHour:       80.0,
				MaxLossPerDay:        200.0,
				MaxConsecutiveLosses: 7,
				CooldownMinutes:      60,
				MaxTradesPerMinute:   2,
				MaxTradesPerHour:     10,
				MaxTradesPerDay:      20,
				WinRateCheckAfter:    20,
				MinWinRate:           55.0,
			},
			SLTP: &ModeSLTPConfig{
				StopLossPercent:         2.5,
				TakeProfitPercent:       5.0,
				TrailingStopEnabled:     true,
				TrailingStopPercent:     1.5,
				TrailingStopActivation:  1.0,
				TrailingActivationPrice: 0,
				MaxHoldDuration:         "3d",
				UseSingleTP:             false,
				SingleTPPercent:         5.0,
				TPAllocation:            []float64{50, 50, 0, 0}, // 50% at TP1, 50% at TP2
				TrailingActivationMode:  "after_tp1_and_breakeven",
				UseROIBasedSLTP:         false,
				ROIStopLossPercent:      -12,
				ROITakeProfitPercent:    20,
				MarginType:              "CROSS",
				IsolatedMarginPercent:   40,
			},
			Hedge: &HedgeModeConfig{
				AllowHedge:                true,
				MinConfidenceForHedge:     80.0,
				ExistingMustBeInProfit:    1.0, // > 1%
				MaxHedgeSizePercent:       50.0,
				AllowSameModeHedge:        false,
				MaxTotalExposureMultiplier: 2.0,
			},
			Averaging: &PositionAveragingConfig{
				AllowAveraging:         true,
				AverageUpProfitPercent: 1.0, // Add when profit > 1%
				AverageDownLossPercent: -1.5, // Or loss < -1.5%
				AddSizePercent:         50.0,
				MaxAverages:            3,
				MinConfidenceForAverage: 70.0,
				UseLLMForAveraging:     true,
			},
			StaleRelease: &StalePositionReleaseConfig{
				Enabled:              true,
				MaxHoldDuration:      "5d",
				MinProfitToKeep:      1.0,
				MaxLossToForceClose:  -1.5,
				StaleZoneLo:          -1.0,
				StaleZoneHi:          1.0,
				StaleZoneCloseAction: "close",
			},
			Assignment: &ModeAssignmentConfig{
				VolatilityMin:       "low",
				VolatilityMax:       "medium",
				ExpectedHoldMin:     "4h",
				ExpectedHoldMax:     "3d",
				ConfidenceMin:       65.0,
				ConfidenceMax:       85.0,
				RiskScoreMax:        40,
				ProfitPotentialMin:  3.0,
				ProfitPotentialMax:  8.0,
				RequiresTrendAlign:  true,
				PriorityWeight:      1.2,
			},
		},
		"position": {
			ModeName: "position",
			Enabled:  true,
			Timeframe: &ModeTimeframeConfig{
				TrendTimeframe:    "4h",
				EntryTimeframe:    "1h",
				AnalysisTimeframe: "1d",
			},
			Confidence: &ModeConfidenceConfig{
				MinConfidence:   75.0,
				HighConfidence:  85.0,
				UltraConfidence: 92.0,
			},
			Size: &ModeSizeConfig{
				BaseSizeUSD:      600.0,
				MaxSizeUSD:       1000.0,
				MaxPositions:     2,
				Leverage:         3,
				SizeMultiplierLo: 1.0,
				SizeMultiplierHi: 2.5,
			},
			CircuitBreaker: &ModeCircuitBreakerConfig{
				MaxLossPerHour:       150.0,
				MaxLossPerDay:        400.0,
				MaxConsecutiveLosses: 10,
				CooldownMinutes:      120,
				MaxTradesPerMinute:   1,
				MaxTradesPerHour:     5,
				MaxTradesPerDay:      10,
				WinRateCheckAfter:    25,
				MinWinRate:           60.0,
			},
			SLTP: &ModeSLTPConfig{
				StopLossPercent:         3.5,
				TakeProfitPercent:       8.0,
				TrailingStopEnabled:     true,
				TrailingStopPercent:     2.5,
				TrailingStopActivation:  2.0,
				TrailingActivationPrice: 0,
				MaxHoldDuration:         "14d",
				UseSingleTP:             false,
				SingleTPPercent:         8.0,
				TPAllocation:            []float64{40, 30, 20, 10}, // 40% TP1, 30% TP2, 20% TP3, 10% TP4
				TrailingActivationMode:  "after_tp1_and_breakeven",
				UseROIBasedSLTP:         false,
				ROIStopLossPercent:      -15,
				ROITakeProfitPercent:    30,
				MarginType:              "ISOLATED", // Position mode uses isolated for safety
				IsolatedMarginPercent:   50,
			},
			Hedge: &HedgeModeConfig{
				AllowHedge:                true, // Cautious
				MinConfidenceForHedge:     85.0,
				ExistingMustBeInProfit:    2.0, // > 2%
				MaxHedgeSizePercent:       50.0,
				AllowSameModeHedge:        false,
				MaxTotalExposureMultiplier: 2.0,
			},
			Averaging: &PositionAveragingConfig{
				AllowAveraging:         true,
				AverageUpProfitPercent: 2.0, // Add when profit > 2%
				AverageDownLossPercent: -2.0, // Or loss < -2%
				AddSizePercent:         30.0, // Add 30% of original
				MaxAverages:            2,
				MinConfidenceForAverage: 80.0,
				UseLLMForAveraging:     true,
			},
			StaleRelease: &StalePositionReleaseConfig{
				Enabled:              true,
				MaxHoldDuration:      "21d",
				MinProfitToKeep:      2.0,
				MaxLossToForceClose:  -2.0,
				StaleZoneLo:          -1.5,
				StaleZoneHi:          1.5,
				StaleZoneCloseAction: "close",
			},
			Assignment: &ModeAssignmentConfig{
				VolatilityMin:       "low",
				VolatilityMax:       "low",
				ExpectedHoldMin:     "3d",
				ExpectedHoldMax:     "30d",
				ConfidenceMin:       75.0,
				ConfidenceMax:       100.0,
				RiskScoreMax:        30,
				ProfitPotentialMin:  5.0,
				ProfitPotentialMax:  15.0,
				RequiresTrendAlign:  true,
				PriorityWeight:      1.5,
			},
		},
	}
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
	// Settings version for migration (v1=legacy, v2=consolidated ModeConfigs)
	SettingsVersion int `json:"settings_version"`

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
	// Deprecated: Use ModeConfigs[mode].Risk.RiskLevel instead. Will be migrated automatically.
	GinieRiskLevel     string  `json:"ginie_risk_level"`      // conservative/moderate/aggressive
	GinieDryRunMode    bool    `json:"ginie_dry_run_mode"`    // Paper trading mode for Ginie
	GinieAutoStart     bool    `json:"ginie_auto_start"`      // Auto-start Ginie on server restart
	GinieMaxUSD        float64 `json:"ginie_max_usd"`         // Max USD per position for Ginie
	GinieLeverage      int     `json:"ginie_leverage"`        // Default leverage for Ginie
	GinieMinConfidence float64 `json:"ginie_min_confidence"`  // Min confidence to trade
	GinieMaxPositions  int     `json:"ginie_max_positions"`   // Max concurrent positions for Ginie

	// Ginie trend detection timeframes (per mode)
	// Deprecated: Use ModeConfigs["ultrafast"].Timeframe.TrendTimeframe instead. Will be migrated automatically.
	GinieTrendTimeframeUltrafast string `json:"ginie_trend_timeframe_ultrafast"` // e.g., "5m"
	// Deprecated: Use ModeConfigs["scalp"].Timeframe.TrendTimeframe instead. Will be migrated automatically.
	GinieTrendTimeframeScalp    string `json:"ginie_trend_timeframe_scalp"`    // e.g., "15m"
	// Deprecated: Use ModeConfigs["swing"].Timeframe.TrendTimeframe instead. Will be migrated automatically.
	GinieTrendTimeframeSwing    string `json:"ginie_trend_timeframe_swing"`    // e.g., "1h"
	// Deprecated: Use ModeConfigs["position"].Timeframe.TrendTimeframe instead. Will be migrated automatically.
	GinieTrendTimeframePosition string `json:"ginie_trend_timeframe_position"` // e.g., "4h"

	// Ginie divergence detection
	// Deprecated: Use ModeConfigs[mode].TrendDivergence.BlockOnDivergence instead. Will be migrated automatically.
	GinieBlockOnDivergence bool `json:"ginie_block_on_divergence"` // Block trades when timeframe divergence detected

	// Ginie SL/TP manual overrides (per mode) - if set (> 0), override ATR/LLM calculations
	// Deprecated: Use ModeConfigs["ultrafast"].SLTP.StopLossPercent instead. Will be migrated automatically.
	GinieSLPercentUltrafast    float64 `json:"ginie_sl_percent_ultrafast"`    // e.g., 0.5 (0.5%)
	// Deprecated: Use ModeConfigs["ultrafast"].SLTP.TakeProfitPercent instead. Will be migrated automatically.
	GinieTPPercentUltrafast    float64 `json:"ginie_tp_percent_ultrafast"`    // e.g., 1.0 (1%)
	// Deprecated: Use ModeConfigs["scalp"].SLTP.StopLossPercent instead. Will be migrated automatically.
	GinieSLPercentScalp        float64 `json:"ginie_sl_percent_scalp"`        // e.g., 1.0 (1%)
	// Deprecated: Use ModeConfigs["scalp"].SLTP.TakeProfitPercent instead. Will be migrated automatically.
	GinieTPPercentScalp        float64 `json:"ginie_tp_percent_scalp"`        // e.g., 2.0 (2%)
	// Deprecated: Use ModeConfigs["swing"].SLTP.StopLossPercent instead. Will be migrated automatically.
	GinieSLPercentSwing        float64 `json:"ginie_sl_percent_swing"`        // e.g., 2.0 (2%)
	// Deprecated: Use ModeConfigs["swing"].SLTP.TakeProfitPercent instead. Will be migrated automatically.
	GinieTPPercentSwing        float64 `json:"ginie_tp_percent_swing"`        // e.g., 6.0 (6%)
	// Deprecated: Use ModeConfigs["position"].SLTP.StopLossPercent instead. Will be migrated automatically.
	GinieSLPercentPosition     float64 `json:"ginie_sl_percent_position"`     // e.g., 3.0 (3%)
	// Deprecated: Use ModeConfigs["position"].SLTP.TakeProfitPercent instead. Will be migrated automatically.
	GinieTPPercentPosition     float64 `json:"ginie_tp_percent_position"`     // e.g., 10.0 (10%)

	// Trailing stop configuration (per mode)
	// Deprecated: Use ModeConfigs["ultrafast"].SLTP.TrailingStopEnabled instead. Will be migrated automatically.
	GinieTrailingStopEnabledUltrafast       bool    `json:"ginie_trailing_stop_enabled_ultrafast"`
	// Deprecated: Use ModeConfigs["ultrafast"].SLTP.TrailingStopPercent instead. Will be migrated automatically.
	GinieTrailingStopPercentUltrafast       float64 `json:"ginie_trailing_stop_percent_ultrafast"`       // e.g., 0.1%
	// Deprecated: Use ModeConfigs["ultrafast"].SLTP.TrailingStopActivation instead. Will be migrated automatically.
	GinieTrailingStopActivationUltrafast    float64 `json:"ginie_trailing_stop_activation_ultrafast"`    // e.g., 0.2% profit

	// Deprecated: Use ModeConfigs["scalp"].SLTP.TrailingStopEnabled instead. Will be migrated automatically.
	GinieTrailingStopEnabledScalp           bool    `json:"ginie_trailing_stop_enabled_scalp"`
	// Deprecated: Use ModeConfigs["scalp"].SLTP.TrailingStopPercent instead. Will be migrated automatically.
	GinieTrailingStopPercentScalp           float64 `json:"ginie_trailing_stop_percent_scalp"`       // e.g., 0.3%
	// Deprecated: Use ModeConfigs["scalp"].SLTP.TrailingStopActivation instead. Will be migrated automatically.
	GinieTrailingStopActivationScalp        float64 `json:"ginie_trailing_stop_activation_scalp"`    // e.g., 0.5% profit

	// Deprecated: Use ModeConfigs["swing"].SLTP.TrailingStopEnabled instead. Will be migrated automatically.
	GinieTrailingStopEnabledSwing           bool    `json:"ginie_trailing_stop_enabled_swing"`
	// Deprecated: Use ModeConfigs["swing"].SLTP.TrailingStopPercent instead. Will be migrated automatically.
	GinieTrailingStopPercentSwing           float64 `json:"ginie_trailing_stop_percent_swing"`       // e.g., 1.5%
	// Deprecated: Use ModeConfigs["swing"].SLTP.TrailingStopActivation instead. Will be migrated automatically.
	GinieTrailingStopActivationSwing        float64 `json:"ginie_trailing_stop_activation_swing"`    // e.g., 1.0% profit

	// Deprecated: Use ModeConfigs["position"].SLTP.TrailingStopEnabled instead. Will be migrated automatically.
	GinieTrailingStopEnabledPosition        bool    `json:"ginie_trailing_stop_enabled_position"`
	// Deprecated: Use ModeConfigs["position"].SLTP.TrailingStopPercent instead. Will be migrated automatically.
	GinieTrailingStopPercentPosition        float64 `json:"ginie_trailing_stop_percent_position"`    // e.g., 3.0%
	// Deprecated: Use ModeConfigs["position"].SLTP.TrailingStopActivation instead. Will be migrated automatically.
	GinieTrailingStopActivationPosition     float64 `json:"ginie_trailing_stop_activation_position"` // e.g., 2.0% profit

	// TP mode configuration (global and per-mode)
	GinieUseSingleTP          bool    `json:"ginie_use_single_tp"`           // true = 100% at TP1, false = 4-level (global fallback)
	GinieUseSingleTPUltrafast bool    `json:"ginie_use_single_tp_ultrafast"` // Ultra-fast: always single TP
	GinieUseSingleTPScalp     bool    `json:"ginie_use_single_tp_scalp"`     // Scalp: single TP mode
	GinieSingleTPPercent      float64 `json:"ginie_single_tp_percent"`       // If single TP, this is the gain %

	// Trailing stop activation mode: "immediate" (default), "after_tp1", "after_breakeven", "after_tp1_and_breakeven"
	GinieTrailingActivationMode string `json:"ginie_trailing_activation_mode"` // When to activate trailing

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

	// ====== COMPREHENSIVE MODE CONFIGURATIONS (Story 2.7) ======
	// Full configuration for each trading mode with all settings
	// User can customize any setting - defaults provided from Story 2.7
	ModeConfigs map[string]*ModeFullConfig `json:"mode_configs"`

	// ====== LLM AND ADAPTIVE AI CONFIGURATION (Story 2.8) ======
	// Global LLM provider settings
	LLMConfig LLMConfig `json:"llm_config"`
	// Per-mode LLM settings
	ModeLLMSettings map[GinieTradingMode]ModeLLMSettings `json:"mode_llm_settings"`
	// Adaptive AI learning configuration
	AdaptiveAIConfig AdaptiveAIConfig `json:"adaptive_ai_config"`
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
		// Settings version (v2 = consolidated ModeConfigs)
		SettingsVersion: SettingsVersionConsolidated,

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
		GinieTrailingStopEnabledUltrafast:       false, // Ultra-fast: NO trailing, only SL/TP
		GinieTrailingStopPercentUltrafast:       0,    // Disabled
		GinieTrailingStopActivationUltrafast:    0,    // Disabled

		GinieTrailingStopEnabledScalp:           false, // Scalp: NO trailing, only SL/TP
		GinieTrailingStopPercentScalp:           0,     // Disabled
		GinieTrailingStopActivationScalp:        0,     // Disabled

		GinieTrailingStopEnabledSwing:           true,
		GinieTrailingStopPercentSwing:           1.5,  // 1.5%
		GinieTrailingStopActivationSwing:        1.0,  // After 1% profit

		GinieTrailingStopEnabledPosition:        true,
		GinieTrailingStopPercentPosition:        3.0,  // 3.0%
		GinieTrailingStopActivationPosition:     2.0,  // After 2% profit

		// Ginie TP mode (per-mode single TP settings)
		GinieUseSingleTP:          false, // Global fallback: Use multi-TP
		GinieUseSingleTPUltrafast: true,  // Ultra-fast: ALWAYS single TP (100% at TP)
		GinieUseSingleTPScalp:     true,  // Scalp: Single TP for quick exits
		GinieSingleTPPercent:      5.0,   // If single TP enabled, 5% gain

		// Trailing activation mode: trailing only activates after TP1 AND breakeven
		GinieTrailingActivationMode: "after_tp1_and_breakeven",

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
		UltraFastMonitorInterval: 2000,  // Monitor every 2 seconds for exits (reduced from 500ms to lower API load)
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

		// Comprehensive mode configurations (Story 2.7 defaults)
		ModeConfigs: DefaultModeConfigs(),

		// LLM and Adaptive AI configuration (Story 2.8 defaults)
		LLMConfig:        DefaultLLMConfig(),
		ModeLLMSettings:  DefaultModeLLMSettings(),
		AdaptiveAIConfig: DefaultAdaptiveAIConfig(),
	}
}

// ====== SETTINGS MIGRATION (Story 12) ======
// Migration constants
const (
	SettingsVersionLegacy       = 1 // Original flat structure with GinieTrailing*, GinieTrendTimeframe*, etc.
	SettingsVersionConsolidated = 2 // Consolidated ModeConfigs structure
)

// patchMigrateTPAllocation is a patch migration that runs on v2 settings to add the
// TPAllocation field which was added after initial v2 migration.
// This ensures existing v2 settings get TPAllocation populated from global settings.
func patchMigrateTPAllocation(settings *AutopilotSettings) bool {
	if settings.ModeConfigs == nil {
		return false
	}

	modes := []string{"ultra_fast", "scalp", "swing", "position"}
	migrated := false

	// Build global TP allocation from global settings
	globalTPAllocation := []float64{
		settings.GinieTP1Percent,
		settings.GinieTP2Percent,
		settings.GinieTP3Percent,
		settings.GinieTP4Percent,
	}

	// Default allocations per mode (if global is all zeros)
	defaultAllocations := map[string][]float64{
		"ultra_fast": {100, 0, 0, 0},  // 100% at TP1
		"scalp":      {100, 0, 0, 0},  // 100% at TP1
		"swing":      {50, 50, 0, 0},  // 50% at TP1, 50% at TP2
		"position":   {40, 30, 20, 10}, // Multi-level
	}

	hasGlobalAllocation := settings.GinieTP1Percent > 0 || settings.GinieTP2Percent > 0 ||
		settings.GinieTP3Percent > 0 || settings.GinieTP4Percent > 0

	for _, mode := range modes {
		cfg := settings.ModeConfigs[mode]
		if cfg == nil || cfg.SLTP == nil {
			continue
		}

		// Check if TPAllocation is missing or empty
		if cfg.SLTP.TPAllocation == nil || len(cfg.SLTP.TPAllocation) == 0 {
			if hasGlobalAllocation {
				// Use global allocation for multi-TP modes
				if !cfg.SLTP.UseSingleTP {
					cfg.SLTP.TPAllocation = globalTPAllocation
					log.Printf("[PATCH-MIGRATION] Applied global TPAllocation to %s: %v", mode, globalTPAllocation)
					migrated = true
				} else {
					// For single-TP modes, set 100% at TP1
					cfg.SLTP.TPAllocation = []float64{100, 0, 0, 0}
					log.Printf("[PATCH-MIGRATION] Applied single-TP allocation to %s: [100, 0, 0, 0]", mode)
					migrated = true
				}
			} else {
				// Use defaults for each mode
				cfg.SLTP.TPAllocation = defaultAllocations[mode]
				log.Printf("[PATCH-MIGRATION] Applied default TPAllocation to %s: %v", mode, defaultAllocations[mode])
				migrated = true
			}
		}
	}

	return migrated
}

// migrateSettings migrates legacy autopilot_settings.json to the consolidated ModeConfigs structure.
// This function is idempotent - safe to run multiple times.
// Legacy fields remain populated for rollback safety.
// Returns true if migration was performed, false if already migrated or no migration needed.
func migrateSettings(settings *AutopilotSettings) bool {
	// Always run patch migrations to fix missing fields in v2
	patchMigrated := patchMigrateTPAllocation(settings)

	// Already migrated to v2 or higher - no full migration needed
	if settings.SettingsVersion >= SettingsVersionConsolidated {
		return patchMigrated
	}

	// Ensure ModeConfigs map exists
	if settings.ModeConfigs == nil {
		settings.ModeConfigs = make(map[string]*ModeFullConfig)
	}

	// Initialize mode configs if they don't exist
	modes := []string{"ultra_fast", "scalp", "swing", "position"}
	for _, mode := range modes {
		if settings.ModeConfigs[mode] == nil {
			settings.ModeConfigs[mode] = &ModeFullConfig{
				ModeName: mode,
				Enabled:  true,
			}
		}
		// Ensure sub-configs exist
		ensureModeSubConfigs(settings.ModeConfigs[mode])
	}

	// ====== MIGRATE TRAILING STOP SETTINGS ======
	// Scalp trailing stop
	if settings.GinieTrailingStopPercentScalp > 0 {
		settings.ModeConfigs["scalp"].SLTP.TrailingStopPercent = settings.GinieTrailingStopPercentScalp
	}
	if settings.GinieTrailingStopActivationScalp > 0 {
		settings.ModeConfigs["scalp"].SLTP.TrailingStopActivation = settings.GinieTrailingStopActivationScalp
	}
	settings.ModeConfigs["scalp"].SLTP.TrailingStopEnabled = settings.GinieTrailingStopEnabledScalp

	// Swing trailing stop
	if settings.GinieTrailingStopPercentSwing > 0 {
		settings.ModeConfigs["swing"].SLTP.TrailingStopPercent = settings.GinieTrailingStopPercentSwing
	}
	if settings.GinieTrailingStopActivationSwing > 0 {
		settings.ModeConfigs["swing"].SLTP.TrailingStopActivation = settings.GinieTrailingStopActivationSwing
	}
	settings.ModeConfigs["swing"].SLTP.TrailingStopEnabled = settings.GinieTrailingStopEnabledSwing

	// Position trailing stop
	if settings.GinieTrailingStopPercentPosition > 0 {
		settings.ModeConfigs["position"].SLTP.TrailingStopPercent = settings.GinieTrailingStopPercentPosition
	}
	if settings.GinieTrailingStopActivationPosition > 0 {
		settings.ModeConfigs["position"].SLTP.TrailingStopActivation = settings.GinieTrailingStopActivationPosition
	}
	settings.ModeConfigs["position"].SLTP.TrailingStopEnabled = settings.GinieTrailingStopEnabledPosition

	// Ultra-fast trailing stop
	if settings.GinieTrailingStopPercentUltrafast > 0 {
		settings.ModeConfigs["ultra_fast"].SLTP.TrailingStopPercent = settings.GinieTrailingStopPercentUltrafast
	}
	if settings.GinieTrailingStopActivationUltrafast > 0 {
		settings.ModeConfigs["ultra_fast"].SLTP.TrailingStopActivation = settings.GinieTrailingStopActivationUltrafast
	}
	settings.ModeConfigs["ultra_fast"].SLTP.TrailingStopEnabled = settings.GinieTrailingStopEnabledUltrafast

	// ====== MIGRATE TIMEFRAME SETTINGS ======
	// Scalp timeframe -> TrendTimeframe
	if settings.GinieTrendTimeframeScalp != "" {
		settings.ModeConfigs["scalp"].Timeframe.TrendTimeframe = settings.GinieTrendTimeframeScalp
	}
	// Swing timeframe -> TrendTimeframe
	if settings.GinieTrendTimeframeSwing != "" {
		settings.ModeConfigs["swing"].Timeframe.TrendTimeframe = settings.GinieTrendTimeframeSwing
	}
	// Position timeframe -> TrendTimeframe
	if settings.GinieTrendTimeframePosition != "" {
		settings.ModeConfigs["position"].Timeframe.TrendTimeframe = settings.GinieTrendTimeframePosition
	}
	// Ultra-fast timeframe -> TrendTimeframe
	if settings.GinieTrendTimeframeUltrafast != "" {
		settings.ModeConfigs["ultra_fast"].Timeframe.TrendTimeframe = settings.GinieTrendTimeframeUltrafast
	}

	// ====== MIGRATE SL/TP PERCENT SETTINGS ======
	// These override ATR/LLM calculations if set (> 0)
	if settings.GinieSLPercentScalp > 0 {
		settings.ModeConfigs["scalp"].SLTP.StopLossPercent = settings.GinieSLPercentScalp
	}
	if settings.GinieTPPercentScalp > 0 {
		settings.ModeConfigs["scalp"].SLTP.TakeProfitPercent = settings.GinieTPPercentScalp
	}
	if settings.GinieSLPercentSwing > 0 {
		settings.ModeConfigs["swing"].SLTP.StopLossPercent = settings.GinieSLPercentSwing
	}
	if settings.GinieTPPercentSwing > 0 {
		settings.ModeConfigs["swing"].SLTP.TakeProfitPercent = settings.GinieTPPercentSwing
	}
	if settings.GinieSLPercentPosition > 0 {
		settings.ModeConfigs["position"].SLTP.StopLossPercent = settings.GinieSLPercentPosition
	}
	if settings.GinieTPPercentPosition > 0 {
		settings.ModeConfigs["position"].SLTP.TakeProfitPercent = settings.GinieTPPercentPosition
	}
	if settings.GinieSLPercentUltrafast > 0 {
		settings.ModeConfigs["ultra_fast"].SLTP.StopLossPercent = settings.GinieSLPercentUltrafast
	}
	if settings.GinieTPPercentUltrafast > 0 {
		settings.ModeConfigs["ultra_fast"].SLTP.TakeProfitPercent = settings.GinieTPPercentUltrafast
	}

	// ====== MIGRATE SINGLE TP MODE SETTINGS ======
	settings.ModeConfigs["ultra_fast"].SLTP.UseSingleTP = settings.GinieUseSingleTPUltrafast
	settings.ModeConfigs["scalp"].SLTP.UseSingleTP = settings.GinieUseSingleTPScalp
	// Swing and position default to multi-TP, but can be overridden by global setting
	if settings.GinieUseSingleTP {
		settings.ModeConfigs["swing"].SLTP.UseSingleTP = settings.GinieUseSingleTP
		settings.ModeConfigs["position"].SLTP.UseSingleTP = settings.GinieUseSingleTP
	}

	// ====== MIGRATE SINGLE TP PERCENT (new field) ======
	if settings.GinieSingleTPPercent > 0 {
		// Apply to all modes as fallback (will be overwritten by mode-specific TP% below)
		for _, mode := range modes {
			if settings.ModeConfigs[mode].SLTP.SingleTPPercent == 0 {
				settings.ModeConfigs[mode].SLTP.SingleTPPercent = settings.GinieSingleTPPercent
			}
		}
	}
	// Override with mode-specific TP percentages
	if settings.GinieTPPercentUltrafast > 0 {
		settings.ModeConfigs["ultra_fast"].SLTP.SingleTPPercent = settings.GinieTPPercentUltrafast
	}
	if settings.GinieTPPercentScalp > 0 {
		settings.ModeConfigs["scalp"].SLTP.SingleTPPercent = settings.GinieTPPercentScalp
	}
	if settings.GinieTPPercentSwing > 0 {
		settings.ModeConfigs["swing"].SLTP.SingleTPPercent = settings.GinieTPPercentSwing
	}
	if settings.GinieTPPercentPosition > 0 {
		settings.ModeConfigs["position"].SLTP.SingleTPPercent = settings.GinieTPPercentPosition
	}

	// ====== MIGRATE TRAILING ACTIVATION MODE (new field) ======
	if settings.GinieTrailingActivationMode != "" {
		for _, mode := range modes {
			if settings.ModeConfigs[mode].SLTP.TrailingActivationMode == "" {
				settings.ModeConfigs[mode].SLTP.TrailingActivationMode = settings.GinieTrailingActivationMode
			}
		}
	}

	// ====== MIGRATE TP ALLOCATION (new field) ======
	// Convert global TP1/TP2/TP3/TP4 percent to per-mode TPAllocation array
	if settings.GinieTP1Percent > 0 || settings.GinieTP2Percent > 0 ||
		settings.GinieTP3Percent > 0 || settings.GinieTP4Percent > 0 {
		globalTPAllocation := []float64{
			settings.GinieTP1Percent,
			settings.GinieTP2Percent,
			settings.GinieTP3Percent,
			settings.GinieTP4Percent,
		}
		// Apply to modes that use multi-TP and don't have allocation set
		for _, mode := range modes {
			if !settings.ModeConfigs[mode].SLTP.UseSingleTP &&
				(settings.ModeConfigs[mode].SLTP.TPAllocation == nil ||
					len(settings.ModeConfigs[mode].SLTP.TPAllocation) == 0) {
				settings.ModeConfigs[mode].SLTP.TPAllocation = globalTPAllocation
			}
		}
	}

	// Bump version to v2 after successful migration
	settings.SettingsVersion = SettingsVersionConsolidated

	return true
}

// ensureModeSubConfigs ensures all sub-configurations exist for a mode
func ensureModeSubConfigs(config *ModeFullConfig) {
	if config.Timeframe == nil {
		config.Timeframe = &ModeTimeframeConfig{}
	}
	if config.Confidence == nil {
		config.Confidence = &ModeConfidenceConfig{}
	}
	if config.Size == nil {
		config.Size = &ModeSizeConfig{}
	}
	if config.CircuitBreaker == nil {
		config.CircuitBreaker = &ModeCircuitBreakerConfig{}
	}
	if config.SLTP == nil {
		config.SLTP = &ModeSLTPConfig{}
	}
	if config.Hedge == nil {
		config.Hedge = &HedgeModeConfig{}
	}
	if config.Averaging == nil {
		config.Averaging = &PositionAveragingConfig{}
	}
	if config.StaleRelease == nil {
		config.StaleRelease = &StalePositionReleaseConfig{}
	}
	if config.Assignment == nil {
		config.Assignment = &ModeAssignmentConfig{}
	}
	if config.FundingRate == nil {
		config.FundingRate = &ModeFundingRateConfig{}
	}
}

// LoadSettings loads settings from file, returns defaults if file doesn't exist.
// If migration is needed, it performs migration and saves the updated settings.
// Also runs patch migrations to fix missing fields in existing v2 settings.
func (sm *SettingsManager) LoadSettings() (*AutopilotSettings, error) {
	// First, try to load with read lock to check if migration is needed
	settings, needsMigration, err := sm.loadSettingsInternal()
	if err != nil {
		return settings, err
	}

	// If full migration is needed (v1 -> v2), perform it with write lock and save
	if needsMigration {
		return sm.loadAndMigrateSettings()
	}

	// Even if version is already v2, run patch migrations for missing fields
	// This handles the case where new fields were added after initial v2 migration
	patchMigrated := patchMigrateTPAllocation(settings)
	if patchMigrated {
		// Need to save the patch-migrated settings
		log.Println("[SETTINGS] Patch migration applied, saving updated settings...")
		if saveErr := sm.SaveSettings(settings); saveErr != nil {
			log.Printf("[SETTINGS] Warning: Failed to save patch-migrated settings: %v", saveErr)
		}
	}

	return settings, nil
}

// loadSettingsInternal loads settings and returns whether migration is needed
func (sm *SettingsManager) loadSettingsInternal() (*AutopilotSettings, bool, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	settings := DefaultSettings()

	data, err := os.ReadFile(sm.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return defaults (already at latest version)
			return settings, false, nil
		}
		return settings, false, err
	}

	if err := json.Unmarshal(data, settings); err != nil {
		return DefaultSettings(), false, err
	}

	// Check if migration is needed (version < 2 means legacy settings)
	needsMigration := settings.SettingsVersion < SettingsVersionConsolidated

	return settings, needsMigration, nil
}

// loadAndMigrateSettings loads settings with write lock, performs migration, and saves
func (sm *SettingsManager) loadAndMigrateSettings() (*AutopilotSettings, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	settings := DefaultSettings()

	data, err := os.ReadFile(sm.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return defaults with latest version
			return settings, nil
		}
		return settings, err
	}

	if err := json.Unmarshal(data, settings); err != nil {
		return DefaultSettings(), err
	}

	// Perform migration (idempotent - safe to run multiple times)
	migrated := migrateSettings(settings)
	if migrated {
		// Save the migrated settings to persist the version bump
		// We already hold the write lock, so we can save directly
		saveData, marshalErr := json.MarshalIndent(settings, "", "  ")
		if marshalErr != nil {
			// Migration succeeded but save failed - still return migrated settings
			// Next load will re-migrate (idempotent)
			return settings, nil
		}

		// Atomic write: temp file + rename
		tempPath := sm.settingsPath + ".tmp"
		if writeErr := os.WriteFile(tempPath, saveData, 0644); writeErr != nil {
			// Save failed - still return migrated settings
			return settings, nil
		}

		if renameErr := os.Rename(tempPath, sm.settingsPath); renameErr != nil {
			os.Remove(tempPath) // Clean up temp file
			// Save failed - still return migrated settings
			return settings, nil
		}
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
// Deprecated: This function updates deprecated legacy fields. Use UpdateGinieModeConfig to update
// ModeConfigs[mode].Risk.RiskLevel instead. Legacy fields are kept for backwards compatibility
// and will be migrated to ModeConfigs automatically.
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
// Deprecated: This function updates deprecated legacy fields. Use UpdateGinieModeConfig to update
// ModeConfigs[mode].Timeframe.TrendTimeframe and ModeConfigs[mode].TrendDivergence.BlockOnDivergence
// instead. Legacy fields are kept for backwards compatibility and will be migrated automatically.
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
// Deprecated: This function updates deprecated legacy fields. Use UpdateGinieModeConfig to update
// ModeConfigs[mode].SLTP settings instead. Legacy fields are kept for backwards compatibility
// and will be migrated to ModeConfigs automatically.
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

// === MODE CONFIGURATION METHODS (Story 2.7 Task 2.7.10) ===

// ValidModes lists all valid trading mode names
var ValidModes = map[string]bool{
	"ultra_fast": true,
	"scalp":      true,
	"swing":      true,
	"position":   true,
}

// ValidateModeConfig validates a ModeFullConfig for consistency and bounds
func ValidateModeConfig(config *ModeFullConfig) error {
	if config == nil {
		return fmt.Errorf("mode config cannot be nil")
	}

	// Validate mode name
	if !ValidModes[config.ModeName] {
		return fmt.Errorf("invalid mode name '%s': must be ultra_fast, scalp, swing, or position", config.ModeName)
	}

	// Validate timeframe config if present
	if config.Timeframe != nil {
		if config.Timeframe.TrendTimeframe != "" {
			if err := ValidateTimeframe(config.Timeframe.TrendTimeframe); err != nil {
				return fmt.Errorf("timeframe.trend_timeframe: %w", err)
			}
		}
		if config.Timeframe.EntryTimeframe != "" {
			if err := ValidateTimeframe(config.Timeframe.EntryTimeframe); err != nil {
				return fmt.Errorf("timeframe.entry_timeframe: %w", err)
			}
		}
		if config.Timeframe.AnalysisTimeframe != "" {
			if err := ValidateTimeframe(config.Timeframe.AnalysisTimeframe); err != nil {
				return fmt.Errorf("timeframe.analysis_timeframe: %w", err)
			}
		}
	}

	// Validate confidence config if present
	if config.Confidence != nil {
		if config.Confidence.MinConfidence < 0 || config.Confidence.MinConfidence > 100 {
			return fmt.Errorf("confidence.min_confidence must be between 0 and 100")
		}
		if config.Confidence.HighConfidence < 0 || config.Confidence.HighConfidence > 100 {
			return fmt.Errorf("confidence.high_confidence must be between 0 and 100")
		}
		if config.Confidence.UltraConfidence < 0 || config.Confidence.UltraConfidence > 100 {
			return fmt.Errorf("confidence.ultra_confidence must be between 0 and 100")
		}
	}

	// Validate size config if present
	if config.Size != nil {
		if config.Size.BaseSizeUSD < 0 {
			return fmt.Errorf("size.base_size_usd must be positive")
		}
		if config.Size.MaxSizeUSD < 0 {
			return fmt.Errorf("size.max_size_usd must be positive")
		}
		if config.Size.MaxPositions < 0 {
			return fmt.Errorf("size.max_positions must be non-negative")
		}
		if config.Size.Leverage < 1 || config.Size.Leverage > 125 {
			return fmt.Errorf("size.leverage must be between 1 and 125")
		}
	}

	// Validate circuit breaker config if present
	if config.CircuitBreaker != nil {
		if config.CircuitBreaker.MaxLossPerHour < 0 {
			return fmt.Errorf("circuit_breaker.max_loss_per_hour must be non-negative")
		}
		if config.CircuitBreaker.MaxLossPerDay < 0 {
			return fmt.Errorf("circuit_breaker.max_loss_per_day must be non-negative")
		}
		if config.CircuitBreaker.MaxConsecutiveLosses < 0 {
			return fmt.Errorf("circuit_breaker.max_consecutive_losses must be non-negative")
		}
		if config.CircuitBreaker.CooldownMinutes < 0 {
			return fmt.Errorf("circuit_breaker.cooldown_minutes must be non-negative")
		}
		if config.CircuitBreaker.MinWinRate < 0 || config.CircuitBreaker.MinWinRate > 100 {
			return fmt.Errorf("circuit_breaker.min_win_rate must be between 0 and 100")
		}
	}

	// Validate SLTP config if present
	if config.SLTP != nil {
		if config.SLTP.StopLossPercent < 0 || config.SLTP.StopLossPercent > 100 {
			return fmt.Errorf("sltp.stop_loss_percent must be between 0 and 100")
		}
		if config.SLTP.TakeProfitPercent < 0 || config.SLTP.TakeProfitPercent > 100 {
			return fmt.Errorf("sltp.take_profit_percent must be between 0 and 100")
		}
		if config.SLTP.TrailingStopPercent < 0 || config.SLTP.TrailingStopPercent > 100 {
			return fmt.Errorf("sltp.trailing_stop_percent must be between 0 and 100")
		}
		if config.SLTP.TrailingStopActivation < 0 || config.SLTP.TrailingStopActivation > 100 {
			return fmt.Errorf("sltp.trailing_stop_activation must be between 0 and 100")
		}
	}

	return nil
}

// GetAllModeConfigs returns all 4 mode configurations
func (sm *SettingsManager) GetAllModeConfigs() map[string]*ModeFullConfig {
	settings := sm.GetCurrentSettings()
	if settings.ModeConfigs == nil || len(settings.ModeConfigs) == 0 {
		return DefaultModeConfigs()
	}
	// Merge with defaults to ensure new fields have proper values
	return mergeWithDefaultConfigs(settings.ModeConfigs)
}

// mergeWithDefaultConfigs ensures saved configs have new fields populated from defaults
func mergeWithDefaultConfigs(saved map[string]*ModeFullConfig) map[string]*ModeFullConfig {
	defaults := DefaultModeConfigs()
	result := make(map[string]*ModeFullConfig)

	for mode, savedConfig := range saved {
		if savedConfig == nil {
			if def, ok := defaults[mode]; ok {
				result[mode] = def
			}
			continue
		}

		// Get default for this mode
		def, ok := defaults[mode]
		if !ok {
			result[mode] = savedConfig
			continue
		}

		// Merge all sub-configs - use defaults for any nil sub-configs
		// This prevents nil pointer panics when accessing config fields
		if savedConfig.Timeframe == nil && def.Timeframe != nil {
			savedConfig.Timeframe = def.Timeframe
		}
		if savedConfig.Confidence == nil && def.Confidence != nil {
			savedConfig.Confidence = def.Confidence
		}
		if savedConfig.Size == nil && def.Size != nil {
			savedConfig.Size = def.Size
		}
		if savedConfig.CircuitBreaker == nil && def.CircuitBreaker != nil {
			savedConfig.CircuitBreaker = def.CircuitBreaker
		}
		if savedConfig.Hedge == nil && def.Hedge != nil {
			savedConfig.Hedge = def.Hedge
		}
		if savedConfig.Averaging == nil && def.Averaging != nil {
			savedConfig.Averaging = def.Averaging
		}
		if savedConfig.StaleRelease == nil && def.StaleRelease != nil {
			savedConfig.StaleRelease = def.StaleRelease
		}
		if savedConfig.Assignment == nil && def.Assignment != nil {
			savedConfig.Assignment = def.Assignment
		}

		// Merge SLTP config - fill in zero values from defaults
		if savedConfig.SLTP != nil && def.SLTP != nil {
			// Only fill in fields that are zero/empty (not explicitly set)
			if savedConfig.SLTP.MarginType == "" {
				savedConfig.SLTP.MarginType = def.SLTP.MarginType
			}
			if savedConfig.SLTP.IsolatedMarginPercent == 0 {
				savedConfig.SLTP.IsolatedMarginPercent = def.SLTP.IsolatedMarginPercent
			}
			// Note: ROI fields default to 0/false which is valid, so only set MarginType default
			// TrailingActivationPrice 0 means disabled, which is valid default
		} else if savedConfig.SLTP == nil && def.SLTP != nil {
			savedConfig.SLTP = def.SLTP
		}

		result[mode] = savedConfig
	}

	// Add any modes that exist in defaults but not in saved
	for mode, defConfig := range defaults {
		if _, exists := result[mode]; !exists {
			result[mode] = defConfig
		}
	}

	return result
}

// GetModeConfig returns the configuration for a specific mode
func (sm *SettingsManager) GetModeConfig(mode string) (*ModeFullConfig, error) {
	if !ValidModes[mode] {
		return nil, fmt.Errorf("invalid mode '%s': must be ultra_fast, scalp, swing, or position", mode)
	}

	configs := sm.GetAllModeConfigs()
	if config, exists := configs[mode]; exists {
		return config, nil
	}

	// Return default if not found
	defaults := DefaultModeConfigs()
	if config, exists := defaults[mode]; exists {
		return config, nil
	}

	return nil, fmt.Errorf("mode config not found for '%s'", mode)
}

// UpdateModeConfig updates the configuration for a specific mode
func (sm *SettingsManager) UpdateModeConfig(mode string, config *ModeFullConfig) error {
	if !ValidModes[mode] {
		return fmt.Errorf("invalid mode '%s': must be ultra_fast, scalp, swing, or position", mode)
	}

	// Ensure mode name matches
	config.ModeName = mode

	// Validate the config
	if err := ValidateModeConfig(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	settings := sm.GetCurrentSettings()
	if settings.ModeConfigs == nil {
		settings.ModeConfigs = DefaultModeConfigs()
	}

	settings.ModeConfigs[mode] = config

	// CRITICAL: Sync max_positions to ModeAllocation as well
	// This ensures the allocation system uses the updated limits
	if config.Size != nil && config.Size.MaxPositions > 0 {
		if settings.ModeAllocation == nil {
			settings.ModeAllocation = &ModeAllocationConfig{}
		}
		switch mode {
		case "ultra_fast":
			settings.ModeAllocation.MaxUltraFastPositions = config.Size.MaxPositions
		case "scalp":
			settings.ModeAllocation.MaxScalpPositions = config.Size.MaxPositions
		case "swing":
			settings.ModeAllocation.MaxSwingPositions = config.Size.MaxPositions
		case "position":
			settings.ModeAllocation.MaxPositionPositions = config.Size.MaxPositions
		}
		fmt.Printf("[MODE-CONFIG] Synced max_positions=%d to ModeAllocation for %s\n", config.Size.MaxPositions, mode)
	}

	return sm.SaveSettings(settings)
}

// ResetModeConfigs resets all mode configurations to defaults
func (sm *SettingsManager) ResetModeConfigs() error {
	settings := sm.GetCurrentSettings()
	settings.ModeConfigs = DefaultModeConfigs()
	return sm.SaveSettings(settings)
}

// GetModeCircuitBreakerStatus returns circuit breaker status for all modes
// This is a read-only status view, not runtime state
func (sm *SettingsManager) GetModeCircuitBreakerConfigs() map[string]*ModeCircuitBreakerConfig {
	configs := sm.GetAllModeConfigs()
	result := make(map[string]*ModeCircuitBreakerConfig)

	for mode, config := range configs {
		if config.CircuitBreaker != nil {
			result[mode] = config.CircuitBreaker
		}
	}

	return result
}

// ============================================================================
// GINIE MODE CONFIG MANAGEMENT WITH GinieTradingMode AND GinieModeConfig TYPES
// (Story 2.7 Task 2.7.2)
// ============================================================================

// GetDefaultModeConfig returns the default GinieModeConfig for a specific GinieTradingMode.
// Each mode (ultra_fast, scalp, swing, position) has tailored defaults optimized
// for its trading style, timeframe, and risk profile.
func GetDefaultModeConfig(mode GinieTradingMode) GinieModeConfig {
	switch mode {
	case GinieModeUltraFast:
		return GinieModeConfig{
			Mode:    GinieModeUltraFast,
			Enabled: true,

			// Timeframes - very short for ultra-fast scalping
			TrendTimeframe:    "5m",
			EntryTimeframe:    "1m",
			AnalysisTimeframe: "1m",

			// Confidence - lower thresholds for quick entry
			MinConfidence:   50,
			HighConfidence:  70,
			UltraConfidence: 85,

			// Position Sizing - small, leveraged positions
			BaseSizeUSD:    100,
			MaxSizeUSD:     200,
			MaxPositions:   5,
			Leverage:       10,
			SizeMultiplier: 1.5,

			// SL/TP - tight stops, quick targets
			StopLossPercent:   1.0,
			TakeProfitPercent: 2.0,
			TrailingEnabled:   false,
			TrailingPercent:   0,
			TrailingActivation: 0.5,
			TrailingActivationPrice: 0, // Use profit % based
			MaxHoldDuration:   "3s",

			// ROI-based SL/TP (disabled by default, uses price % above)
			UseROIBasedSLTP:      false,
			ROIStopLossPercent:   -10, // -10% ROI
			ROITakeProfitPercent: 20,  // +20% ROI

			// Margin - cross by default for ultra-fast
			MarginType:            "CROSS",
			IsolatedMarginPercent: 50,

			// Circuit Breaker - aggressive limits for high-frequency trading
			CircuitBreaker: ModeCircuitBreaker{
				MaxLossPerHour:     20,
				MaxLossPerDay:      50,
				MaxConsecutiveLoss: 3,
				MaxTradesPerMinute: 5,
				MaxTradesPerHour:   30,
				MaxTradesPerDay:    100,
				WinRateCheckAfter:  10,
				MinWinRatePercent:  45,
				CooldownMinutes:    15,
				AutoRecovery:       true,
			},
		}

	case GinieModeScalp:
		return GinieModeConfig{
			Mode:    GinieModeScalp,
			Enabled: true,

			// Timeframes - short-term scalping
			TrendTimeframe:    "15m",
			EntryTimeframe:    "5m",
			AnalysisTimeframe: "15m",

			// Confidence - moderate thresholds
			MinConfidence:   60,
			HighConfidence:  75,
			UltraConfidence: 88,

			// Position Sizing - moderate positions
			BaseSizeUSD:    200,
			MaxSizeUSD:     400,
			MaxPositions:   4,
			Leverage:       8,
			SizeMultiplier: 1.8,

			// SL/TP - reasonable stops and targets
			StopLossPercent:   1.5,
			TakeProfitPercent: 3.0,
			TrailingEnabled:   false,
			TrailingPercent:   0.5,
			TrailingActivation: 1.0,
			TrailingActivationPrice: 0,
			MaxHoldDuration:   "4h",

			// ROI-based SL/TP
			UseROIBasedSLTP:      false,
			ROIStopLossPercent:   -12,
			ROITakeProfitPercent: 24,

			// Margin
			MarginType:            "CROSS",
			IsolatedMarginPercent: 50,

			// Circuit Breaker - balanced for frequent trading
			CircuitBreaker: ModeCircuitBreaker{
				MaxLossPerHour:     40,
				MaxLossPerDay:      100,
				MaxConsecutiveLoss: 5,
				MaxTradesPerMinute: 3,
				MaxTradesPerHour:   20,
				MaxTradesPerDay:    50,
				WinRateCheckAfter:  15,
				MinWinRatePercent:  50,
				CooldownMinutes:    30,
				AutoRecovery:       true,
			},
		}

	case GinieModeSwing:
		return GinieModeConfig{
			Mode:    GinieModeSwing,
			Enabled: true,

			// Timeframes - medium-term swing trading
			TrendTimeframe:    "1h",
			EntryTimeframe:    "15m",
			AnalysisTimeframe: "4h",

			// Confidence - higher thresholds for quality trades
			MinConfidence:   65,
			HighConfidence:  80,
			UltraConfidence: 90,

			// Position Sizing - larger positions, lower leverage
			BaseSizeUSD:    400,
			MaxSizeUSD:     750,
			MaxPositions:   3,
			Leverage:       5,
			SizeMultiplier: 2.0,

			// SL/TP - wider stops with trailing
			StopLossPercent:   2.5,
			TakeProfitPercent: 5.0,
			TrailingEnabled:   true,
			TrailingPercent:   1.5,
			TrailingActivation: 2.0,
			TrailingActivationPrice: 0,
			MaxHoldDuration:   "3d",

			// ROI-based SL/TP
			UseROIBasedSLTP:      false,
			ROIStopLossPercent:   -15,
			ROITakeProfitPercent: 25,

			// Margin
			MarginType:            "CROSS",
			IsolatedMarginPercent: 40,

			// Circuit Breaker - conservative for swing trading
			CircuitBreaker: ModeCircuitBreaker{
				MaxLossPerHour:     80,
				MaxLossPerDay:      200,
				MaxConsecutiveLoss: 7,
				MaxTradesPerMinute: 2,
				MaxTradesPerHour:   10,
				MaxTradesPerDay:    20,
				WinRateCheckAfter:  20,
				MinWinRatePercent:  55,
				CooldownMinutes:    60,
				AutoRecovery:       true,
			},
		}

	case GinieModePosition:
		return GinieModeConfig{
			Mode:    GinieModePosition,
			Enabled: true,

			// Timeframes - long-term position trading
			TrendTimeframe:    "4h",
			EntryTimeframe:    "1h",
			AnalysisTimeframe: "1d",

			// Confidence - highest thresholds for conviction trades
			MinConfidence:   75,
			HighConfidence:  85,
			UltraConfidence: 92,

			// Position Sizing - largest positions, lowest leverage
			BaseSizeUSD:    600,
			MaxSizeUSD:     1000,
			MaxPositions:   2,
			Leverage:       3,
			SizeMultiplier: 2.5,

			// SL/TP - widest stops with trailing
			StopLossPercent:   3.5,
			TakeProfitPercent: 8.0,
			TrailingEnabled:   true,
			TrailingPercent:   2.5,
			TrailingActivation: 3.0,
			TrailingActivationPrice: 0,
			MaxHoldDuration:   "14d",

			// ROI-based SL/TP
			UseROIBasedSLTP:      false,
			ROIStopLossPercent:   -20,
			ROITakeProfitPercent: 40,

			// Margin - isolated by default for position (safer)
			MarginType:            "ISOLATED",
			IsolatedMarginPercent: 30,

			// Circuit Breaker - very conservative for position trading
			CircuitBreaker: ModeCircuitBreaker{
				MaxLossPerHour:     150,
				MaxLossPerDay:      400,
				MaxConsecutiveLoss: 10,
				MaxTradesPerMinute: 1,
				MaxTradesPerHour:   5,
				MaxTradesPerDay:    10,
				WinRateCheckAfter:  25,
				MinWinRatePercent:  60,
				CooldownMinutes:    120,
				AutoRecovery:       false,
			},
		}

	default:
		// Return scalp as default if unknown mode
		return GetDefaultModeConfig(GinieModeScalp)
	}
}

// GetAllDefaultModeConfigs returns a map of all 4 trading mode configurations with their defaults.
// This is useful for displaying all mode configurations in the UI or for initialization.
// Returns map keyed by GinieTradingMode containing GinieModeConfig values.
func GetAllDefaultModeConfigs() map[GinieTradingMode]GinieModeConfig {
	return map[GinieTradingMode]GinieModeConfig{
		GinieModeUltraFast: GetDefaultModeConfig(GinieModeUltraFast),
		GinieModeScalp:     GetDefaultModeConfig(GinieModeScalp),
		GinieModeSwing:     GetDefaultModeConfig(GinieModeSwing),
		GinieModePosition:  GetDefaultModeConfig(GinieModePosition),
	}
}

// UpdateGinieModeConfig saves a custom GinieModeConfig for a specific GinieTradingMode.
// This allows users to override the defaults with their own settings.
// The configuration is validated before saving.
func (sm *SettingsManager) UpdateGinieModeConfig(mode GinieTradingMode, config GinieModeConfig) error {
	// Validate mode
	if mode != GinieModeUltraFast && mode != GinieModeScalp && mode != GinieModeSwing && mode != GinieModePosition {
		return fmt.Errorf("invalid trading mode: %s", mode)
	}

	// Validate timeframes
	if config.TrendTimeframe != "" {
		if err := ValidateTimeframe(config.TrendTimeframe); err != nil {
			return fmt.Errorf("invalid trend timeframe: %w", err)
		}
	}
	if config.EntryTimeframe != "" {
		if err := ValidateTimeframe(config.EntryTimeframe); err != nil {
			return fmt.Errorf("invalid entry timeframe: %w", err)
		}
	}
	if config.AnalysisTimeframe != "" {
		if err := ValidateTimeframe(config.AnalysisTimeframe); err != nil {
			return fmt.Errorf("invalid analysis timeframe: %w", err)
		}
	}

	// Validate confidence thresholds
	if config.MinConfidence < 0 || config.MinConfidence > 100 {
		return fmt.Errorf("min_confidence must be between 0 and 100, got: %.2f", config.MinConfidence)
	}
	if config.HighConfidence < 0 || config.HighConfidence > 100 {
		return fmt.Errorf("high_confidence must be between 0 and 100, got: %.2f", config.HighConfidence)
	}
	if config.UltraConfidence < 0 || config.UltraConfidence > 100 {
		return fmt.Errorf("ultra_confidence must be between 0 and 100, got: %.2f", config.UltraConfidence)
	}
	if config.MinConfidence > config.HighConfidence {
		return fmt.Errorf("min_confidence (%.2f) cannot be greater than high_confidence (%.2f)", config.MinConfidence, config.HighConfidence)
	}
	if config.HighConfidence > config.UltraConfidence {
		return fmt.Errorf("high_confidence (%.2f) cannot be greater than ultra_confidence (%.2f)", config.HighConfidence, config.UltraConfidence)
	}

	// Validate position sizing
	if config.BaseSizeUSD < 0 {
		return fmt.Errorf("base_size_usd cannot be negative: %.2f", config.BaseSizeUSD)
	}
	if config.MaxSizeUSD < 0 {
		return fmt.Errorf("max_size_usd cannot be negative: %.2f", config.MaxSizeUSD)
	}
	if config.BaseSizeUSD > config.MaxSizeUSD && config.MaxSizeUSD > 0 {
		return fmt.Errorf("base_size_usd (%.2f) cannot be greater than max_size_usd (%.2f)", config.BaseSizeUSD, config.MaxSizeUSD)
	}
	if config.MaxPositions < 0 {
		return fmt.Errorf("max_positions cannot be negative: %d", config.MaxPositions)
	}
	if config.Leverage < 1 || config.Leverage > 125 {
		return fmt.Errorf("leverage must be between 1 and 125, got: %d", config.Leverage)
	}
	if config.SizeMultiplier < 0 {
		return fmt.Errorf("size_multiplier cannot be negative: %.2f", config.SizeMultiplier)
	}

	// Validate SL/TP
	if config.StopLossPercent < 0 || config.StopLossPercent > 100 {
		return fmt.Errorf("stop_loss_percent must be between 0 and 100, got: %.2f", config.StopLossPercent)
	}
	if config.TakeProfitPercent < 0 || config.TakeProfitPercent > 100 {
		return fmt.Errorf("take_profit_percent must be between 0 and 100, got: %.2f", config.TakeProfitPercent)
	}
	if config.TrailingPercent < 0 || config.TrailingPercent > 100 {
		return fmt.Errorf("trailing_percent must be between 0 and 100, got: %.2f", config.TrailingPercent)
	}

	// Validate circuit breaker
	cb := config.CircuitBreaker
	if cb.MaxLossPerHour < 0 {
		return fmt.Errorf("circuit_breaker.max_loss_per_hour cannot be negative: %.2f", cb.MaxLossPerHour)
	}
	if cb.MaxLossPerDay < 0 {
		return fmt.Errorf("circuit_breaker.max_loss_per_day cannot be negative: %.2f", cb.MaxLossPerDay)
	}
	if cb.MaxConsecutiveLoss < 0 {
		return fmt.Errorf("circuit_breaker.max_consecutive_loss cannot be negative: %d", cb.MaxConsecutiveLoss)
	}
	if cb.MaxTradesPerMinute < 0 {
		return fmt.Errorf("circuit_breaker.max_trades_per_minute cannot be negative: %d", cb.MaxTradesPerMinute)
	}
	if cb.MaxTradesPerHour < 0 {
		return fmt.Errorf("circuit_breaker.max_trades_per_hour cannot be negative: %d", cb.MaxTradesPerHour)
	}
	if cb.MaxTradesPerDay < 0 {
		return fmt.Errorf("circuit_breaker.max_trades_per_day cannot be negative: %d", cb.MaxTradesPerDay)
	}
	if cb.WinRateCheckAfter < 0 {
		return fmt.Errorf("circuit_breaker.win_rate_check_after cannot be negative: %d", cb.WinRateCheckAfter)
	}
	if cb.MinWinRatePercent < 0 || cb.MinWinRatePercent > 100 {
		return fmt.Errorf("circuit_breaker.min_win_rate_percent must be between 0 and 100, got: %.2f", cb.MinWinRatePercent)
	}
	if cb.CooldownMinutes < 0 {
		return fmt.Errorf("circuit_breaker.cooldown_minutes cannot be negative: %d", cb.CooldownMinutes)
	}

	// Ensure mode is set correctly
	config.Mode = mode

	// Get current settings and update the mode config
	settings := sm.GetCurrentSettings()

	// Initialize ModeConfigs if nil
	if settings.ModeConfigs == nil {
		settings.ModeConfigs = DefaultModeConfigs()
	}

	// Convert GinieModeConfig to ModeFullConfig for storage
	modeStr := string(mode)
	if settings.ModeConfigs[modeStr] == nil {
		settings.ModeConfigs[modeStr] = &ModeFullConfig{ModeName: modeStr}
	}

	fullConfig := settings.ModeConfigs[modeStr]
	fullConfig.Enabled = config.Enabled

	// Update Timeframe
	if fullConfig.Timeframe == nil {
		fullConfig.Timeframe = &ModeTimeframeConfig{}
	}
	fullConfig.Timeframe.TrendTimeframe = config.TrendTimeframe
	fullConfig.Timeframe.EntryTimeframe = config.EntryTimeframe
	fullConfig.Timeframe.AnalysisTimeframe = config.AnalysisTimeframe

	// Update Confidence
	if fullConfig.Confidence == nil {
		fullConfig.Confidence = &ModeConfidenceConfig{}
	}
	fullConfig.Confidence.MinConfidence = config.MinConfidence
	fullConfig.Confidence.HighConfidence = config.HighConfidence
	fullConfig.Confidence.UltraConfidence = config.UltraConfidence

	// Update Size
	if fullConfig.Size == nil {
		fullConfig.Size = &ModeSizeConfig{}
	}
	fullConfig.Size.BaseSizeUSD = config.BaseSizeUSD
	fullConfig.Size.MaxSizeUSD = config.MaxSizeUSD
	fullConfig.Size.MaxPositions = config.MaxPositions
	fullConfig.Size.Leverage = config.Leverage
	fullConfig.Size.SizeMultiplierHi = config.SizeMultiplier

	// Update SLTP
	if fullConfig.SLTP == nil {
		fullConfig.SLTP = &ModeSLTPConfig{}
	}
	fullConfig.SLTP.StopLossPercent = config.StopLossPercent
	fullConfig.SLTP.TakeProfitPercent = config.TakeProfitPercent
	fullConfig.SLTP.TrailingStopEnabled = config.TrailingEnabled
	fullConfig.SLTP.TrailingStopPercent = config.TrailingPercent
	fullConfig.SLTP.MaxHoldDuration = config.MaxHoldDuration

	// Update CircuitBreaker
	if fullConfig.CircuitBreaker == nil {
		fullConfig.CircuitBreaker = &ModeCircuitBreakerConfig{}
	}
	fullConfig.CircuitBreaker.MaxLossPerHour = config.CircuitBreaker.MaxLossPerHour
	fullConfig.CircuitBreaker.MaxLossPerDay = config.CircuitBreaker.MaxLossPerDay
	fullConfig.CircuitBreaker.MaxConsecutiveLosses = config.CircuitBreaker.MaxConsecutiveLoss
	fullConfig.CircuitBreaker.MaxTradesPerMinute = config.CircuitBreaker.MaxTradesPerMinute
	fullConfig.CircuitBreaker.MaxTradesPerHour = config.CircuitBreaker.MaxTradesPerHour
	fullConfig.CircuitBreaker.MaxTradesPerDay = config.CircuitBreaker.MaxTradesPerDay
	fullConfig.CircuitBreaker.WinRateCheckAfter = config.CircuitBreaker.WinRateCheckAfter
	fullConfig.CircuitBreaker.MinWinRate = config.CircuitBreaker.MinWinRatePercent
	fullConfig.CircuitBreaker.CooldownMinutes = config.CircuitBreaker.CooldownMinutes

	return sm.SaveSettings(settings)
}

// GetGinieModeConfig retrieves the current GinieModeConfig for a specific GinieTradingMode.
// If no custom config exists, returns the default configuration.
func (sm *SettingsManager) GetGinieModeConfig(mode GinieTradingMode) GinieModeConfig {
	settings := sm.GetCurrentSettings()

	modeStr := string(mode)
	if settings.ModeConfigs != nil && settings.ModeConfigs[modeStr] != nil {
		fullConfig := settings.ModeConfigs[modeStr]
		return convertModeFullConfigToGinieModeConfig(mode, fullConfig)
	}

	// Return defaults if no custom config exists
	return GetDefaultModeConfig(mode)
}

// convertModeFullConfigToGinieModeConfig converts ModeFullConfig to GinieModeConfig
func convertModeFullConfigToGinieModeConfig(mode GinieTradingMode, fc *ModeFullConfig) GinieModeConfig {
	config := GinieModeConfig{
		Mode:    mode,
		Enabled: fc.Enabled,
	}

	if fc.Timeframe != nil {
		config.TrendTimeframe = fc.Timeframe.TrendTimeframe
		config.EntryTimeframe = fc.Timeframe.EntryTimeframe
		config.AnalysisTimeframe = fc.Timeframe.AnalysisTimeframe
	}

	if fc.Confidence != nil {
		config.MinConfidence = fc.Confidence.MinConfidence
		config.HighConfidence = fc.Confidence.HighConfidence
		config.UltraConfidence = fc.Confidence.UltraConfidence
	}

	if fc.Size != nil {
		config.BaseSizeUSD = fc.Size.BaseSizeUSD
		config.MaxSizeUSD = fc.Size.MaxSizeUSD
		config.MaxPositions = fc.Size.MaxPositions
		config.Leverage = fc.Size.Leverage
		config.SizeMultiplier = fc.Size.SizeMultiplierHi
	}

	if fc.SLTP != nil {
		config.StopLossPercent = fc.SLTP.StopLossPercent
		config.TakeProfitPercent = fc.SLTP.TakeProfitPercent
		config.TrailingEnabled = fc.SLTP.TrailingStopEnabled
		config.TrailingPercent = fc.SLTP.TrailingStopPercent
		config.MaxHoldDuration = fc.SLTP.MaxHoldDuration
	}

	if fc.CircuitBreaker != nil {
		config.CircuitBreaker = ModeCircuitBreaker{
			MaxLossPerHour:     fc.CircuitBreaker.MaxLossPerHour,
			MaxLossPerDay:      fc.CircuitBreaker.MaxLossPerDay,
			MaxConsecutiveLoss: fc.CircuitBreaker.MaxConsecutiveLosses,
			MaxTradesPerMinute: fc.CircuitBreaker.MaxTradesPerMinute,
			MaxTradesPerHour:   fc.CircuitBreaker.MaxTradesPerHour,
			MaxTradesPerDay:    fc.CircuitBreaker.MaxTradesPerDay,
			WinRateCheckAfter:  fc.CircuitBreaker.WinRateCheckAfter,
			MinWinRatePercent:  fc.CircuitBreaker.MinWinRate,
			CooldownMinutes:    fc.CircuitBreaker.CooldownMinutes,
			AutoRecovery:       true, // Default to true
		}
	}

	return config
}

// ====== LLM AND ADAPTIVE AI CRUD METHODS (Story 2.8) ======

// GetLLMConfig returns the current LLM configuration.
// If no custom config exists, returns the default configuration.
func (sm *SettingsManager) GetLLMConfig() LLMConfig {
	settings := sm.GetCurrentSettings()

	// Check if config is empty (zero value)
	if settings.LLMConfig.Provider == "" {
		return DefaultLLMConfig()
	}
	return settings.LLMConfig
}

// UpdateLLMConfig updates the global LLM configuration.
func (sm *SettingsManager) UpdateLLMConfig(config LLMConfig) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	settings, err := sm.LoadSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	settings.LLMConfig = config
	return sm.SaveSettings(settings)
}

// GetModeLLMSettings returns the LLM settings for a specific trading mode.
// If no custom settings exist, returns the default settings for that mode.
func (sm *SettingsManager) GetModeLLMSettings(mode GinieTradingMode) ModeLLMSettings {
	settings := sm.GetCurrentSettings()

	if settings.ModeLLMSettings != nil {
		if modeSettings, ok := settings.ModeLLMSettings[mode]; ok {
			return modeSettings
		}
	}

	// Return defaults if no custom settings exist
	defaults := DefaultModeLLMSettings()
	if defaultSettings, ok := defaults[mode]; ok {
		return defaultSettings
	}

	// Fallback to ultra_fast defaults if mode not found
	return defaults[GinieModeUltraFast]
}

// UpdateModeLLMSettings updates the LLM settings for a specific trading mode.
// Validates LLMWeight (0.0-1.0) and MinLLMConfidence (0-100).
func (sm *SettingsManager) UpdateModeLLMSettings(mode GinieTradingMode, modeSettings ModeLLMSettings) error {
	// Validate LLMWeight (0.0-1.0)
	if modeSettings.LLMWeight < 0.0 || modeSettings.LLMWeight > 1.0 {
		return fmt.Errorf("LLMWeight must be between 0.0 and 1.0, got %f", modeSettings.LLMWeight)
	}

	// Validate MinLLMConfidence (0-100)
	if modeSettings.MinLLMConfidence < 0 || modeSettings.MinLLMConfidence > 100 {
		return fmt.Errorf("MinLLMConfidence must be between 0 and 100, got %d", modeSettings.MinLLMConfidence)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	settings, err := sm.LoadSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// Initialize map if nil
	if settings.ModeLLMSettings == nil {
		settings.ModeLLMSettings = DefaultModeLLMSettings()
	}

	settings.ModeLLMSettings[mode] = modeSettings
	return sm.SaveSettings(settings)
}

// GetAdaptiveAIConfig returns the current adaptive AI configuration.
// If no custom config exists, returns the default configuration.
func (sm *SettingsManager) GetAdaptiveAIConfig() AdaptiveAIConfig {
	settings := sm.GetCurrentSettings()

	// Check if config is empty (zero value) by checking a required field
	if settings.AdaptiveAIConfig.LearningWindowTrades == 0 &&
		settings.AdaptiveAIConfig.LearningWindowHours == 0 &&
		settings.AdaptiveAIConfig.MinTradesForLearning == 0 {
		return DefaultAdaptiveAIConfig()
	}
	return settings.AdaptiveAIConfig
}

// UpdateAdaptiveAIConfig updates the adaptive AI configuration.
func (sm *SettingsManager) UpdateAdaptiveAIConfig(config AdaptiveAIConfig) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	settings, err := sm.LoadSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	settings.AdaptiveAIConfig = config
	return sm.SaveSettings(settings)
}

// GetAllModeLLMSettings returns the LLM settings for all trading modes.
func (sm *SettingsManager) GetAllModeLLMSettings() map[GinieTradingMode]ModeLLMSettings {
	settings := sm.GetCurrentSettings()

	if settings.ModeLLMSettings != nil && len(settings.ModeLLMSettings) > 0 {
		return settings.ModeLLMSettings
	}

	return DefaultModeLLMSettings()
}
