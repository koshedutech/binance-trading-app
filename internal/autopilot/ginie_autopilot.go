package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/circuit"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/logging"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

// ==================== TRADING FEE CONSTANTS ====================
// Binance Futures trading fees (VIP0 level - adjust based on actual tier)
const (
	// TakerFeeRate is the fee for market orders (0.04% = 0.0004)
	TakerFeeRate = 0.0004

	// MakerFeeRate is the fee for limit orders (0.02% = 0.0002)
	// Not used currently as we primarily use market orders
	MakerFeeRate = 0.0002
)

// calculateTradingFee returns the trading fee for a trade
// For market orders (taker), fee = notional value * TakerFeeRate
func calculateTradingFee(quantity, price float64) float64 {
	notionalValue := quantity * price
	return notionalValue * TakerFeeRate
}

// ===== EARLY PROFIT BOOKING (ROI-BASED) =====
// calculateROIAfterFees calculates the ROI% after accounting for both entry and exit trading fees
// Returns: ROI percentage (e.g., 3.5 for 3.5% gain after fees)
// IMPORTANT: For leveraged positions, ROI is calculated on actual collateral, not notional value
// entryPrice: Entry price
// currentPrice: Current/exit price
// quantity: Position quantity (notional value)
// side: "LONG" or "SHORT"
// leverage: Position leverage (e.g., 5 for 5x leverage). Default is 1 for unleveraged
func calculateROIAfterFees(entryPrice, currentPrice, quantity float64, side string, leverage int) float64 {
	// Validate leverage
	if leverage <= 0 {
		leverage = 1
	}

	// Calculate gross profit/loss (in USD)
	var grossPnl float64
	if side == "LONG" {
		grossPnl = (currentPrice - entryPrice) * quantity
	} else {
		grossPnl = (entryPrice - currentPrice) * quantity
	}

	// Calculate entry and exit fees (on notional value)
	entryFee := calculateTradingFee(quantity, entryPrice)
	exitFee := calculateTradingFee(quantity, currentPrice)
	totalFees := entryFee + exitFee

	// Net profit after fees
	netPnl := grossPnl - totalFees

	// For leveraged positions, collateral = notional / leverage
	// ROI is calculated on actual collateral invested, not notional
	// ROI% = (Net PnL / Collateral) * 100 = (Net PnL * Leverage / Notional) * 100
	notionalAtEntry := quantity * entryPrice

	if notionalAtEntry <= 0 {
		return 0
	}

	// Account for leverage: actual collateral is notional / leverage
	roiPercent := (netPnl * float64(leverage) / notionalAtEntry) * 100
	return roiPercent
}

// ==================== END TRADING FEE CONSTANTS ====================

// GinieAutopilotConfig holds configuration for Ginie autonomous trading
type GinieAutopilotConfig struct {
	Enabled            bool    `json:"enabled"`
	MaxPositions       int     `json:"max_positions"`        // Max concurrent positions
	MaxUSDPerPosition  float64 `json:"max_usd_per_position"` // Max USD per position
	TotalMaxUSD        float64 `json:"total_max_usd"`        // Total max USD allocation
	DefaultLeverage    int     `json:"default_leverage"`     // Default leverage
	DryRun             bool    `json:"dry_run"`              // Paper trading mode
	RiskLevel          string  `json:"risk_level"`           // conservative, moderate, aggressive

	// Mode-specific settings
	EnableScalpMode     bool `json:"enable_scalp_mode"`
	EnableSwingMode     bool `json:"enable_swing_mode"`
	EnablePositionMode  bool `json:"enable_position_mode"`
	EnableUltraFastMode bool `json:"enable_ultra_fast_mode"`

	// Take Profit Distribution (must total 100%)
	TP1Percent float64 `json:"tp1_percent"` // % of position to close at TP1
	TP2Percent float64 `json:"tp2_percent"` // % at TP2
	TP3Percent float64 `json:"tp3_percent"` // % at TP3
	TP4Percent float64 `json:"tp4_percent"` // % trailing at TP4

	// Breakeven settings
	MoveToBreakevenAfterTP1 bool    `json:"move_to_breakeven_after_tp1"`
	BreakevenBuffer         float64 `json:"breakeven_buffer"` // Add small buffer above entry

	// Proactive profit protection (NEW - fixes trailing stop issue)
	ProactiveBreakevenPercent  float64 `json:"proactive_breakeven_percent"`   // Move to breakeven when profit >= this % (before TP1)
	TrailingActivationPercent  float64 `json:"trailing_activation_percent"`   // Activate trailing when profit >= this %
	TrailingStepPercent        float64 `json:"trailing_step_percent"`         // Trail by this % from highest price
	TrailingSLUpdateThreshold  float64 `json:"trailing_sl_update_threshold"`  // Min improvement % before updating Binance order

	// Scan intervals (seconds)
	ScalpScanInterval    int `json:"scalp_scan_interval"`
	SwingScanInterval    int `json:"swing_scan_interval"`
	PositionScanInterval int `json:"position_scan_interval"`

	// Adaptive SL/TP LLM Update Intervals (seconds)
	AdaptiveSLTPEnabled       bool `json:"adaptive_sltp_enabled"`
	ScalpSLTPUpdateInterval   int  `json:"scalp_sltp_update_interval"`   // 1 min for scalp
	SwingSLTPUpdateInterval   int  `json:"swing_sltp_update_interval"`   // 5 min for swing
	PositionSLTPUpdateInterval int `json:"position_sltp_update_interval"` // 15 min for position

	// Confidence thresholds
	MinConfidenceToTrade float64 `json:"min_confidence_to_trade"`

	// Daily limits
	MaxDailyTrades int     `json:"max_daily_trades"`
	MaxDailyLoss   float64 `json:"max_daily_loss"`

	// Circuit breaker settings (separate from FuturesController)
	CircuitBreakerEnabled bool    `json:"circuit_breaker_enabled"`
	CBMaxLossPerHour      float64 `json:"cb_max_loss_per_hour"`
	CBMaxDailyLoss        float64 `json:"cb_max_daily_loss"`
	CBMaxConsecutiveLosses int    `json:"cb_max_consecutive_losses"`
	CBCooldownMinutes     int     `json:"cb_cooldown_minutes"`

	// === EARLY PROFIT BOOKING (ROI-BASED) ===
	// Book profits early based on ROI after trading fees to lock in gains
	EarlyProfitBookingEnabled    bool    `json:"early_profit_booking_enabled"`   // Enable early profit booking
	UltraFastScalpROIThreshold   float64 `json:"ultra_fast_scalp_roi_threshold"` // Book at 3%+ ROI (after fees)
	ScalpROIThreshold            float64 `json:"scalp_roi_threshold"`             // Book at 5%+ ROI (after fees)
	SwingROIThreshold            float64 `json:"swing_roi_threshold"`             // Book at 8%+ ROI (after fees)
	PositionROIThreshold         float64 `json:"position_roi_threshold"`          // Book at 10%+ ROI (after fees)
}

// DefaultGinieAutopilotConfig returns default configuration
func DefaultGinieAutopilotConfig() *GinieAutopilotConfig {
	return &GinieAutopilotConfig{
		Enabled:            false,
		MaxPositions:       10,
		MaxUSDPerPosition:  500,
		TotalMaxUSD:        5000,
		DefaultLeverage:    10,
		DryRun:             false, // Default to LIVE mode, not PAPER mode
		RiskLevel:          "moderate",

		EnableScalpMode:    true,
		EnableSwingMode:    true,
		EnablePositionMode: true,

		// 4-Level TP Distribution (25% each)
		TP1Percent: 25,
		TP2Percent: 25,
		TP3Percent: 25,
		TP4Percent: 25, // Trailing

		MoveToBreakevenAfterTP1: true,
		BreakevenBuffer:         0.1, // 0.1% above entry

		// Proactive profit protection
		ProactiveBreakevenPercent:  0.5,  // Move to breakeven when profit >= 0.5% (before TP1)
		TrailingActivationPercent:  0,    // DEPRECATED: Trailing now activates after TP1+breakeven (from settings)
		TrailingStepPercent:        1.5,  // Default trailing step % (overridden by per-mode settings)
		TrailingSLUpdateThreshold:  0.2,  // Update Binance order when SL improves by >= 0.2%

		// Scan intervals based on mode (reduced for testing)
		ScalpScanInterval:    60,   // 1 minute
		SwingScanInterval:    120,  // 2 minutes (testing)
		PositionScanInterval: 120,  // 2 minutes (testing)

		// Adaptive SL/TP LLM Update Intervals
		AdaptiveSLTPEnabled:        true,
		ScalpSLTPUpdateInterval:    60,   // 1 minute for scalp
		SwingSLTPUpdateInterval:    300,  // 5 minutes for swing
		PositionSLTPUpdateInterval: 900,  // 15 minutes for position

		MinConfidenceToTrade: 50.0, // FIXED: Lowered from 65% to match user expectations
		MaxDailyTrades:       1000,
		MaxDailyLoss:         500,

		// Circuit breaker defaults
		CircuitBreakerEnabled:  true,
		CBMaxLossPerHour:       100,  // $100 max loss per hour
		CBMaxDailyLoss:         300,  // $300 max daily loss
		CBMaxConsecutiveLosses: 3,    // 3 consecutive losses triggers cooldown
		CBCooldownMinutes:      30,   // 30 minute cooldown

		// Early profit booking - ENABLED, but thresholds come from mode-specific settings
		// (GinieTPPercentUltrafast, GinieTPPercentScalp, etc. in autopilot_settings.json)
		// These hardcoded values are DEPRECATED and only used as fallback if settings not loaded
		EarlyProfitBookingEnabled:  true,
		UltraFastScalpROIThreshold: 0, // DEPRECATED: Use settings.GinieTPPercentUltrafast × leverage
		ScalpROIThreshold:          0, // DEPRECATED: Use settings.GinieTPPercentScalp × leverage
		SwingROIThreshold:          0, // DEPRECATED: Use settings.GinieTPPercentSwing × leverage
		PositionROIThreshold:       0, // DEPRECATED: Use settings.GinieTPPercentPosition × leverage
	}
}

// ==================== POSITION PROTECTION STATE MACHINE ====================

// ProtectionState represents the current SL/TP protection state of a position
type ProtectionState string

const (
	// StateOpening - Position just opened, SL/TP not yet placed
	StateOpening ProtectionState = "OPENING"
	// StatePlacingSL - Attempting to place Stop Loss order
	StatePlacingSL ProtectionState = "PLACING_SL"
	// StateSLVerified - Stop Loss confirmed on Binance
	StateSLVerified ProtectionState = "SL_VERIFIED"
	// StatePlacingTP - Attempting to place Take Profit order
	StatePlacingTP ProtectionState = "PLACING_TP"
	// StateFullyProtected - Both SL and TP verified on Binance
	StateFullyProtected ProtectionState = "PROTECTED"
	// StateHealing - Reconciler is fixing missing orders
	StateHealing ProtectionState = "HEALING"
	// StateUnprotected - DANGER: Position lacks SL/TP protection
	StateUnprotected ProtectionState = "UNPROTECTED"
	// StateEmergencyClose - Position being closed due to protection failure
	StateEmergencyClose ProtectionState = "EMERGENCY"
)

// ProtectionStatus tracks the SL/TP protection state of a position
type ProtectionStatus struct {
	State           ProtectionState `json:"state"`
	SLOrderID       int64           `json:"sl_order_id"`
	SLVerified      bool            `json:"sl_verified"`
	SLVerifiedAt    time.Time       `json:"sl_verified_at,omitempty"`
	TPOrderIDs      []int64         `json:"tp_order_ids,omitempty"`
	TPVerified      bool            `json:"tp_verified"`
	TPVerifiedAt    time.Time       `json:"tp_verified_at,omitempty"`
	FailureCount    int             `json:"failure_count"`
	LastFailure     string          `json:"last_failure,omitempty"`
	LastStateChange time.Time       `json:"last_state_change"`
	HealAttempts    int             `json:"heal_attempts"`
}

// NewProtectionStatus creates a new protection status in OPENING state
func NewProtectionStatus() *ProtectionStatus {
	return &ProtectionStatus{
		State:           StateOpening,
		LastStateChange: time.Now(),
	}
}

// SetState updates the protection state with timestamp
func (ps *ProtectionStatus) SetState(state ProtectionState) {
	ps.State = state
	ps.LastStateChange = time.Now()
}

// TimeSinceStateChange returns duration since last state change
func (ps *ProtectionStatus) TimeSinceStateChange() time.Duration {
	return time.Since(ps.LastStateChange)
}

// IsProtected returns true if position has verified SL (minimum protection)
func (ps *ProtectionStatus) IsProtected() bool {
	return ps.State == StateFullyProtected || ps.State == StateSLVerified
}

// NeedsHealing returns true if position needs SL/TP repair
func (ps *ProtectionStatus) NeedsHealing() bool {
	return ps.State == StateUnprotected || ps.State == StateHealing
}

// ==================== END POSITION PROTECTION STATE MACHINE ====================

// GiniePosition represents a Ginie-managed position with multi-level TPs
type GiniePosition struct {
	Symbol          string           `json:"symbol"`
	Side            string           `json:"side"` // "LONG" or "SHORT"
	Mode            GinieTradingMode `json:"mode"` // scalp, swing, position
	EntryPrice      float64          `json:"entry_price"`
	OriginalQty     float64          `json:"original_qty"`     // Original position size
	RemainingQty    float64          `json:"remaining_qty"`    // Remaining after partial closes
	Leverage        int              `json:"leverage"`
	EntryTime       time.Time        `json:"entry_time"`

	// Take Profit Levels
	TakeProfits     []GinieTakeProfitLevel `json:"take_profits"`
	CurrentTPLevel  int                    `json:"current_tp_level"` // 0 = none hit, 1-4 = levels hit

	// Stop Loss
	StopLoss        float64 `json:"stop_loss"`
	OriginalSL      float64 `json:"original_sl"`      // Original SL before breakeven move
	MovedToBreakeven bool   `json:"moved_to_breakeven"`

	// Trailing
	TrailingActive      bool    `json:"trailing_active"`
	HighestPrice        float64 `json:"highest_price"`
	LowestPrice         float64 `json:"lowest_price"`
	TrailingPercent     float64 `json:"trailing_percent"`       // Dynamic trailing %
	TrailingActivationPct float64 `json:"trailing_activation_pct"` // % profit needed to activate trailing

	// Algo Order IDs (for Binance SL/TP orders)
	StopLossAlgoID   int64   `json:"stop_loss_algo_id,omitempty"`   // Binance algo order ID for SL
	TakeProfitAlgoIDs []int64 `json:"take_profit_algo_ids,omitempty"` // Binance algo order IDs for TPs
	LastLLMUpdate    time.Time `json:"last_llm_update,omitempty"`   // Last LLM SL/TP update time

	// Protection State Machine (bulletproof SL/TP tracking)
	Protection *ProtectionStatus `json:"protection,omitempty"` // SL/TP protection state

	// Tracking
	RealizedPnL      float64 `json:"realized_pnl"`     // From partial closes
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	DecisionReport   *GinieDecisionReport `json:"decision_report,omitempty"`
	FuturesTradeID   int64   `json:"futures_trade_id,omitempty"` // Database trade ID for lifecycle events

	// Trade Source Tracking
	Source       string  `json:"source"`                   // "ai" or "strategy"
	StrategyID   *int64  `json:"strategy_id,omitempty"`    // Strategy ID if source is "strategy"
	StrategyName *string `json:"strategy_name,omitempty"`  // Strategy name for display

	// Ultra-Fast Scalping Mode
	UltraFastSignal       *UltraFastSignal `json:"ultra_fast_signal,omitempty"`       // Signal that triggered entry
	UltraFastTargetPercent float64         `json:"ultra_fast_target_percent,omitempty"` // Fee-aware profit target %
	MaxHoldTime           time.Duration    `json:"max_hold_time,omitempty"`            // 3s for ultra-fast

	// Early Profit Booking (per-position custom ROI target)
	CustomROIPercent *float64 `json:"custom_roi_percent,omitempty"` // Custom ROI% for this position (nil = use mode defaults)
}

// GinieTradeResult tracks the result of a trade action with full signal info for study
type GinieTradeResult struct {
	Symbol      string    `json:"symbol"`
	Action      string    `json:"action"` // "open", "partial_close", "full_close"
	Side        string    `json:"side"`
	Quantity    float64   `json:"quantity"`
	Price       float64   `json:"price"`
	PnL         float64   `json:"pnl"`
	PnLPercent  float64   `json:"pnl_percent"`
	Reason      string    `json:"reason"`
	TPLevel     int       `json:"tp_level,omitempty"`
	Timestamp   time.Time `json:"timestamp"`

	// Full decision info for study purposes
	Mode            GinieTradingMode     `json:"mode,omitempty"`
	Confidence      float64              `json:"confidence,omitempty"`
	MarketConditions *GinieMarketSnapshot `json:"market_conditions,omitempty"`
	SignalSummary   *GinieSignalSummary  `json:"signal_summary,omitempty"`
	EntryParams     *GinieEntryParams    `json:"entry_params,omitempty"`

	// Trade Source Tracking
	Source       string  `json:"source"`                   // "ai" or "strategy"
	StrategyID   *int64  `json:"strategy_id,omitempty"`    // Strategy ID if source is "strategy"
	StrategyName *string `json:"strategy_name,omitempty"`  // Strategy name for display
}

// GinieMarketSnapshot captures market state at trade time
type GinieMarketSnapshot struct {
	Trend        string  `json:"trend"`
	ADX          float64 `json:"adx"`
	Volatility   string  `json:"volatility"`
	ATRPercent   float64 `json:"atr_percent"`
	Volume       string  `json:"volume"`
	BTCCorr      float64 `json:"btc_correlation"`
}

// GinieSignalSummary summarizes signals that triggered the trade
type GinieSignalSummary struct {
	Direction       string   `json:"direction"`
	Strength        string   `json:"strength"`
	StrengthScore   float64  `json:"strength_score"`
	PrimaryMet      int      `json:"primary_met"`
	PrimaryRequired int      `json:"primary_required"`
	SignalNames     []string `json:"signal_names"`
}

// GinieEntryParams captures trade entry parameters
type GinieEntryParams struct {
	EntryPrice   float64   `json:"entry_price"`
	StopLoss     float64   `json:"stop_loss"`
	StopLossPct  float64   `json:"stop_loss_pct"`
	TakeProfits  []float64 `json:"take_profits"`
	Leverage     int       `json:"leverage"`
	RiskReward   float64   `json:"risk_reward"`
}

// LLMSwitchEvent tracks when LLM enables or disables a coin
type LLMSwitchEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Symbol    string    `json:"symbol"`
	Action    string    `json:"action"`    // "enable" or "disable"
	Reason    string    `json:"reason"`    // Explanation for the switch
}

// GinieSignalLog tracks all signals generated with executed/rejected status
type GinieSignalLog struct {
	ID            string    `json:"id"`
	Symbol        string    `json:"symbol"`
	Timestamp     time.Time `json:"timestamp"`
	Direction     string    `json:"direction"`      // "LONG" or "SHORT"
	Mode          string    `json:"mode"`           // scalp, swing, position
	Confidence    float64   `json:"confidence"`
	Status        string    `json:"status"`         // "executed", "rejected", "pending"
	RejectionReason string  `json:"rejection_reason,omitempty"`

	// Detailed rejection tracking (shows WHY a good signal wasn't traded)
	RejectionDetails *SignalRejectionDetails `json:"rejection_details,omitempty"`

	// Signal details
	EntryPrice    float64   `json:"entry_price"`
	StopLoss      float64   `json:"stop_loss"`
	TakeProfit1   float64   `json:"take_profit_1"`
	Leverage      int       `json:"leverage"`
	RiskReward    float64   `json:"risk_reward"`

	// Market context at signal time
	CurrentPrice  float64   `json:"current_price"`
	ATRPercent    float64   `json:"atr_percent"`
	Trend         string    `json:"trend"`
	Volatility    string    `json:"volatility"`

	// Signals that contributed
	SignalNames   []string  `json:"signal_names"`
	PrimaryMet    int       `json:"primary_met"`
	PrimaryRequired int     `json:"primary_required"`
}

// SignalRejectionDetails provides detailed rejection breakdown for signals
// This helps users understand exactly WHY a coin with a good score isn't being traded
type SignalRejectionDetails struct {
	// All rejection reasons (ordered by severity)
	AllReasons []string `json:"all_reasons"`

	// Category-specific rejection details
	PositionLimit    *PositionLimitInfo    `json:"position_limit,omitempty"`
	InsufficientFunds *InsufficientFundsInfo `json:"insufficient_funds,omitempty"`
	CircuitBreaker   *CircuitBreakerInfo   `json:"circuit_breaker,omitempty"`
	TrendDivergence  *TrendDivergenceInfo  `json:"trend_divergence,omitempty"`
	SignalQuality    *SignalQualityInfo    `json:"signal_quality,omitempty"`
	CounterTrend     *CounterTrendInfo     `json:"counter_trend,omitempty"`
}

// PositionLimitInfo shows position limit blocking details
type PositionLimitInfo struct {
	CurrentPositions int    `json:"current_positions"`
	MaxPositions     int    `json:"max_positions"`
	ModePositions    int    `json:"mode_positions"`
	ModeName         string `json:"mode_name"`
}

// InsufficientFundsInfo shows funding blocking details
type InsufficientFundsInfo struct {
	RequiredUSD   float64 `json:"required_usd"`
	AvailableUSD  float64 `json:"available_usd"`
	PositionSize  float64 `json:"position_size"`
	Leverage      int     `json:"leverage"`
}

// CircuitBreakerInfo shows circuit breaker blocking details
type CircuitBreakerInfo struct {
	IsTripped    bool    `json:"is_tripped"`
	Reason       string  `json:"reason"`
	DailyLoss    float64 `json:"daily_loss"`
	MaxDailyLoss float64 `json:"max_daily_loss"`
	CooldownMins int     `json:"cooldown_mins"`
}

// TrendDivergenceInfo shows trend divergence blocking details
type TrendDivergenceInfo struct {
	ScanTimeframe     string `json:"scan_timeframe"`
	ScanTrend         string `json:"scan_trend"`
	DecisionTimeframe string `json:"decision_timeframe"`
	DecisionTrend     string `json:"decision_trend"`
	Severity          string `json:"severity"`
}

// SignalQualityInfo shows signal quality blocking details
type SignalQualityInfo struct {
	SignalsMet      int      `json:"signals_met"`
	SignalsRequired int      `json:"signals_required"`
	FailedSignals   []string `json:"failed_signals"`
	ConfidenceScore float64  `json:"confidence_score"`
	MinConfidence   float64  `json:"min_confidence"`
}

// CounterTrendInfo shows counter-trend blocking details
type CounterTrendInfo struct {
	SignalDirection string   `json:"signal_direction"`
	TrendDirection  string   `json:"trend_direction"`
	MissingSignals  []string `json:"missing_signals"`
}

// SLUpdateRecord tracks individual SL update attempts
type SLUpdateRecord struct {
	Timestamp     time.Time `json:"timestamp"`
	OldSL         float64   `json:"old_sl"`
	NewSL         float64   `json:"new_sl"`
	CurrentPrice  float64   `json:"current_price"`
	Status        string    `json:"status"`        // "applied", "rejected"
	RejectionRule string    `json:"rejection_rule,omitempty"` // Which rule rejected it
	Source        string    `json:"source"`        // "llm", "breakeven", "trailing"
	LLMConfidence float64   `json:"llm_confidence,omitempty"`
}

// SLUpdateHistory tracks all SL updates for a position
type SLUpdateHistory struct {
	Symbol        string           `json:"symbol"`
	TotalAttempts int              `json:"total_attempts"`
	Applied       int              `json:"applied"`
	Rejected      int              `json:"rejected"`
	Updates       []SLUpdateRecord `json:"updates"`
}

// CoinBlockInfo tracks why a coin is blocked and when it can be unblocked
type CoinBlockInfo struct {
	Symbol        string    `json:"symbol"`
	BlockReason   string    `json:"block_reason"`
	BlockTime     time.Time `json:"block_time"`
	LossAmount    float64   `json:"loss_amount"`      // Actual loss that triggered block
	LossROI       float64   `json:"loss_roi"`         // ROI % at time of block
	ConsecLosses  int       `json:"consec_losses"`    // Consecutive losses for this coin
	AutoUnblock   time.Time `json:"auto_unblock"`     // When coin will auto-unblock (zero if manual required)
	BlockCount    int       `json:"block_count"`      // How many times this coin was blocked
	ManualOnly    bool      `json:"manual_only"`      // If true, requires manual unblock
}

// ==================== Diagnostic Types ====================

// GinieDiagnostics provides comprehensive troubleshooting info
type GinieDiagnostics struct {
	Timestamp        time.Time `json:"timestamp"`
	AutopilotRunning bool      `json:"autopilot_running"`
	IsLiveMode       bool      `json:"is_live_mode"`
	CanTrade         bool      `json:"can_trade"`
	CanTradeReason   string    `json:"can_trade_reason"`

	CircuitBreaker CBDiagnostics       `json:"circuit_breaker"`
	Positions      PositionDiagnostics `json:"positions"`
	Scanning       ScanDiagnostics     `json:"scanning"`
	Signals        SignalDiagnostics   `json:"signals"`
	ProfitBooking  ProfitDiagnostics   `json:"profit_booking"`
	BlockedCoins   []*CoinBlockInfo    `json:"blocked_coins"`
	LLMStatus      LLMDiagnostics      `json:"llm_status"`
	Issues         []DiagnosticIssue   `json:"issues"`
}

// CBDiagnostics shows circuit breaker state
type CBDiagnostics struct {
	Enabled           bool    `json:"enabled"`
	State             string  `json:"state"`
	HourlyLoss        float64 `json:"hourly_loss"`
	HourlyLossLimit   float64 `json:"hourly_loss_limit"`
	DailyLoss         float64 `json:"daily_loss"`
	DailyLossLimit    float64 `json:"daily_loss_limit"`
	ConsecutiveLosses int     `json:"consecutive_losses"`
	CooldownRemaining string  `json:"cooldown_remaining"`
}

// PositionDiagnostics shows position slot usage
type PositionDiagnostics struct {
	OpenCount          int     `json:"open_count"`
	MaxAllowed         int     `json:"max_allowed"`
	SlotsAvailable     int     `json:"slots_available"`
	TotalUnrealizedPnL float64 `json:"total_unrealized_pnl"`
}

// ScanDiagnostics shows scanning activity
type ScanDiagnostics struct {
	LastScanTime         time.Time `json:"last_scan_time"`
	SecondsSinceLastScan int64     `json:"seconds_since_last_scan"`
	SymbolsInWatchlist   int       `json:"symbols_in_watchlist"`
	SymbolsScannedLast   int       `json:"symbols_scanned_last_cycle"`
	ScalpEnabled         bool      `json:"scalp_enabled"`
	SwingEnabled         bool      `json:"swing_enabled"`
	PositionEnabled      bool      `json:"position_enabled"`
	// New fields for scan status tracking (Issue 2B)
	ScanningActive       bool      `json:"scanning_active"`
	CurrentPhase         string    `json:"current_phase"`
	TimeUntilNextScan    int64     `json:"time_until_next_scan"`
	ScannedThisCycle     int       `json:"scanned_this_cycle"`
	TotalSymbols         int       `json:"total_symbols"`
	LastScanDuration     int64     `json:"last_scan_duration_ms"`
	NextScanTime         time.Time `json:"next_scan_time"`
}

// SignalDiagnostics shows signal generation stats
type SignalDiagnostics struct {
	// Last 1 hour window (existing behavior)
	TotalGenerated      int            `json:"total_generated_1h"`
	Executed            int            `json:"executed_1h"`
	Rejected            int            `json:"rejected_1h"`
	ExecutionRate       float64        `json:"execution_rate_pct_1h"`

	// All-time/session counters (NEW)
	TotalGeneratedAllTime int     `json:"total_generated"`
	ExecutedAllTime       int     `json:"executed"`
	RejectedAllTime       int     `json:"rejected"`
	ExecutionRateAllTime  float64 `json:"execution_rate_pct"`

	TopRejectionReasons map[string]int `json:"top_rejection_reasons"`
}

// ProfitDiagnostics shows profit booking status
type ProfitDiagnostics struct {
	PositionsWithPendingTP int `json:"positions_with_pending_tp"`
	TPHitsLastHour         int `json:"tp_hits_last_hour"`
	PartialClosesLastHour  int `json:"partial_closes_last_hour"`
	FailedClosesLastHour   int `json:"failed_closes_last_hour"`
	TrailingActiveCount    int `json:"trailing_active_count"`
}

// LLMDiagnostics shows LLM connection status
type LLMDiagnostics struct {
	Connected       bool      `json:"connected"`
	Provider        string    `json:"provider"`
	LastCallTime    time.Time `json:"last_call_time"`
	CoinListCached  bool      `json:"coin_list_cached"`
	CoinListAge     string    `json:"coin_list_age"`
	DisabledSymbols []string  `json:"disabled_symbols"`
}

// DiagnosticIssue represents a problem with suggested fix
type DiagnosticIssue struct {
	Severity   string `json:"severity"`   // critical, warning, info
	Category   string `json:"category"`   // scanning, trading, profit, config
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

// GinieAutopilot manages autonomous Ginie trading
type GinieAutopilot struct {
	config        *GinieAutopilotConfig
	analyzer      *GinieAnalyzer
	llmAnalyzer   *llm.Analyzer  // LLM for adaptive SL/TP
	futuresClient binance.FuturesClient
	logger        *logging.Logger
	repo          *database.Repository // Database for trade persistence
	eventLogger   *TradeEventLogger    // Trade lifecycle event logging
	userID        string               // User ID for multi-tenant PnL isolation

	// Circuit breaker (separate from FuturesController)
	circuitBreaker *circuit.CircuitBreaker

	// Dynamic risk level (can be changed without restart)
	currentRiskLevel string

	// State
	running       bool
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex

	// Position tracking
	positions     map[string]*GiniePosition

	// Per-coin blocking for big losses
	blockedCoins       map[string]*CoinBlockInfo  // Coins blocked due to big losses
	coinConsecLosses   map[string]int             // Track consecutive losses per coin
	coinBlockHistory   map[string]int             // Historical count of times each coin was blocked

	// LLM SL validation tracking (kill switch after 3 bad calls)
	badLLMCallCount    map[string]int   // symbol -> consecutive bad LLM SL calls
	llmSLDisabled      map[string]bool  // symbol -> LLM SL updates disabled (kill switch active)

	// Signal logging (all signals with executed/rejected status)
	signalLogs         []GinieSignalLog           // All generated signals
	maxSignalLogs      int                         // Max signals to keep

	// SL update history per position
	slUpdateHistory    map[string]*SLUpdateHistory // symbol -> SL update history

	// Trade history
	tradeHistory  []GinieTradeResult
	maxHistory    int

	// LLM switch tracking
	llmSwitches   []LLMSwitchEvent
	maxLLMSwitches int

	// Daily tracking
	dailyTrades   int
	dailyPnL      float64
	dayStart      time.Time

	// Mode-specific tracking
	scalpTicker    *time.Ticker
	swingTicker    *time.Ticker
	positionTicker *time.Ticker
	ultraFastTicker *time.Ticker

	// Ultra-Fast monitoring
	volatilityRegimes map[string]*VolatilityRegime // Cached volatility regimes per symbol
	lastRegimeUpdate  map[string]time.Time          // When each symbol's regime was last updated

	// Per-mode capital allocation tracking
	modeAllocationStates map[string]*ModeAllocationState // Current allocation state per mode
	modeUsedUSD          map[string]float64              // Total USD used per mode
	modePositionCounts   map[string]int                  // Current position count per mode

	// Per-mode safety controls tracking
	modeSafetyStates  map[string]*ModeSafetyState  // Runtime safety state per mode
	modeSafetyConfigs map[string]*ModeSafetyConfig // Safety config per mode
	lastDayReset      time.Time                     // When daily counters were last reset

	// Per-mode circuit breaker tracking (Story 2.7 Task 2.7.4)
	modeCircuitBreakers map[GinieTradingMode]*ModeCircuitBreaker // Mode-specific circuit breaker state

	// Performance stats
	totalTrades   int
	winningTrades int
	totalPnL      float64

	// Diagnostic tracking
	lastScalpScan           time.Time
	lastSwingScan           time.Time
	lastPositionScan        time.Time
	lastUltraFastScan       time.Time // Ultra-fast mode: 5-second scan tracking
	symbolsScannedLastCycle int
	failedOrdersLastHour   int
	tpHitsLastHour         int
	partialClosesLastHour  int
	lastLLMCallTime        time.Time

	// Scan status tracking (Issue 2B)
	lastScanTime     time.Time     // When last scan completed
	nextScanTime     time.Time     // When next scan will start
	scanningActive   bool          // Currently scanning
	currentPhase     string        // "initializing", "loading_symbols", "scanning", "idle"
	scannedThisCycle int           // Coins scanned in current cycle
	totalSymbols     int           // Total symbols to scan
	scanDuration     time.Duration // How long last scan took

	// Balance caching (to avoid blocking API calls)
	cachedAvailableBalance float64
	cachedWalletBalance    float64
	lastBalanceUpdateTime  time.Time

	// Strategy Evaluation
	strategyEvaluator *StrategyEvaluator
	lastStrategyScan  time.Time

	// SLTP Job Queue (for async recalculation)
	sltpJobQueue *SLTPJobQueue

	// Adaptive AI for dynamic parameter optimization
	adaptiveAI *AdaptiveAI
}

// NewGinieAutopilot creates a new Ginie autonomous trading system
func NewGinieAutopilot(
	analyzer *GinieAnalyzer,
	futuresClient binance.FuturesClient,
	logger *logging.Logger,
	repo *database.Repository,
	userID string,
) *GinieAutopilot {
	config := DefaultGinieAutopilotConfig()

	// Create Ginie's own circuit breaker
	cbConfig := &circuit.CircuitBreakerConfig{
		Enabled:              config.CircuitBreakerEnabled,
		MaxLossPerHour:       config.CBMaxLossPerHour,
		MaxDailyLoss:         config.CBMaxDailyLoss,
		MaxConsecutiveLosses: config.CBMaxConsecutiveLosses,
		CooldownMinutes:      config.CBCooldownMinutes,
		MaxTradesPerMinute:   10,  // Allow 10 trades per minute
		MaxDailyTrades:       100, // Allow 100 trades per day
	}

	ga := &GinieAutopilot{
		config:            config,
		analyzer:          analyzer,
		futuresClient:     futuresClient,
		logger:            logger,
		repo:              repo,
		eventLogger:       NewTradeEventLogger(repo.GetDB()),
		userID:            userID,
		circuitBreaker:    circuit.NewCircuitBreaker(cbConfig),
		currentRiskLevel:  config.RiskLevel,
		stopChan:          make(chan struct{}),
		positions:         make(map[string]*GiniePosition),
		blockedCoins:      make(map[string]*CoinBlockInfo),
		coinConsecLosses:  make(map[string]int),
		coinBlockHistory:  make(map[string]int),
		badLLMCallCount:   make(map[string]int),
		llmSLDisabled:     make(map[string]bool),
		signalLogs:        make([]GinieSignalLog, 0, 500),
		maxSignalLogs:     500,
		slUpdateHistory:   make(map[string]*SLUpdateHistory),
		tradeHistory:      make([]GinieTradeResult, 0, 1000),
		maxHistory:        1000, // Increased for study purposes
		llmSwitches:       make([]LLMSwitchEvent, 0, 500),
		maxLLMSwitches:    500, // Keep last 500 LLM switch events
		dayStart:             time.Now().Truncate(24 * time.Hour),
		volatilityRegimes:    make(map[string]*VolatilityRegime),
		lastRegimeUpdate:     make(map[string]time.Time),
		modeAllocationStates: make(map[string]*ModeAllocationState),
		modeUsedUSD:          make(map[string]float64),
		modePositionCounts:   make(map[string]int),
		modeSafetyStates:     make(map[string]*ModeSafetyState),
		modeSafetyConfigs:    make(map[string]*ModeSafetyConfig),
		lastDayReset:         time.Now().Truncate(24 * time.Hour),
		modeCircuitBreakers:  make(map[GinieTradingMode]*ModeCircuitBreaker),
	}

	// Initialize safety configs and states from settings
	settingsManager := GetSettingsManager()
	settings := settingsManager.GetCurrentSettings()

	// Initialize AdaptiveAI for dynamic parameter optimization
	ga.adaptiveAI = NewAdaptiveAI(settingsManager)

	// Load safety configs
	ga.modeSafetyConfigs["ultra_fast"] = settings.SafetyUltraFast
	ga.modeSafetyConfigs["scalp"] = settings.SafetyScalp
	ga.modeSafetyConfigs["swing"] = settings.SafetySwing
	ga.modeSafetyConfigs["position"] = settings.SafetyPosition

	// CRITICAL FIX: Load user mode enable/disable settings from ModeConfigs
	// This ensures user's mode preferences are respected (fixes swing bypass bug)
	if scalpConfig, err := settingsManager.GetModeConfig("scalp"); err == nil && scalpConfig != nil {
		ga.config.EnableScalpMode = scalpConfig.Enabled
		log.Printf("[GINIE-INIT] Scalp mode enabled from user settings: %v", scalpConfig.Enabled)
	}
	if swingConfig, err := settingsManager.GetModeConfig("swing"); err == nil && swingConfig != nil {
		ga.config.EnableSwingMode = swingConfig.Enabled
		log.Printf("[GINIE-INIT] Swing mode enabled from user settings: %v", swingConfig.Enabled)
	}
	if positionConfig, err := settingsManager.GetModeConfig("position"); err == nil && positionConfig != nil {
		ga.config.EnablePositionMode = positionConfig.Enabled
		log.Printf("[GINIE-INIT] Position mode enabled from user settings: %v", positionConfig.Enabled)
	}

	// Initialize safety states
	for _, mode := range []string{"ultra_fast", "scalp", "swing", "position"} {
		ga.modeSafetyStates[mode] = &ModeSafetyState{
			Mode:              mode,
			TradesLastMinute:  make([]time.Time, 0),
			TradesLastHour:    make([]time.Time, 0),
			TradesToday:       0,
			ProfitWindow:      make([]SafetyTradeResult, 0),
			WindowProfitPct:   0,
			RecentTrades:      make([]SafetyTradeResult, 0),
			CurrentWinRate:    0,
			IsPaused:          false,
			LastUpdate:        time.Now(),
		}
	}

	// Initialize strategy evaluator for saved strategy execution
	if repo != nil {
		ga.strategyEvaluator = NewStrategyEvaluator(repo, futuresClient, logger)
	}

	// Initialize SLTP job queue (keep last 50 jobs)
	ga.sltpJobQueue = NewSLTPJobQueue(50)

	// Initialize mode circuit breakers from default configs (Story 2.7 Task 2.7.4)
	ga.initModeCircuitBreakers()

	return ga
}

// selectEnabledModeForPosition returns the first enabled trading mode for synced/external positions.
// This replaces hardcoded GinieModeSwing defaults to respect user mode preferences.
// Priority: swing > scalp > position (swing is most common default)
func (ga *GinieAutopilot) selectEnabledModeForPosition() GinieTradingMode {
	// Check modes in order of preference for position management
	if ga.config.EnableSwingMode {
		return GinieModeSwing
	}
	if ga.config.EnableScalpMode {
		return GinieModeScalp
	}
	if ga.config.EnablePositionMode {
		return GinieModePosition
	}
	// Fallback: if no modes enabled, default to swing (should not happen normally)
	log.Printf("[GINIE-WARNING] No modes enabled, defaulting to swing for position management")
	return GinieModeSwing
}

// isModeEnabled checks if a specific trading mode is enabled in user settings.
// Used for defense-in-depth validation before executing mode-specific operations.
func (ga *GinieAutopilot) isModeEnabled(mode GinieTradingMode) bool {
	switch mode {
	case GinieModeScalp:
		return ga.config.EnableScalpMode
	case GinieModeSwing:
		return ga.config.EnableSwingMode
	case GinieModePosition:
		return ga.config.EnablePositionMode
	case GinieModeUltraFast:
		// UltraFast has its own config in settings, check there
		sm := GetSettingsManager()
		settings := sm.GetCurrentSettings()
		return settings.UltraFastEnabled
	default:
		return false
	}
}

// LoadPnLStats loads PnL statistics from database (per-user isolation)
func (ga *GinieAutopilot) LoadPnLStats() {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	// If no userID set or no repo, fall back to shared settings (legacy mode)
	if ga.userID == "" || ga.repo == nil {
		sm := GetSettingsManager()
		totalPnL, dailyPnL, totalTrades, winningTrades, dailyTrades := sm.GetGiniePnLStats()
		ga.totalPnL = totalPnL
		ga.dailyPnL = dailyPnL
		ga.totalTrades = totalTrades
		ga.winningTrades = winningTrades
		ga.dailyTrades = dailyTrades
		ga.logger.Info("Loaded Ginie PnL stats from settings (legacy mode)",
			"total_pnl", totalPnL,
			"daily_pnl", dailyPnL,
			"total_trades", totalTrades)
		return
	}

	// Load from database for multi-user isolation
	ctx := context.Background()
	db := ga.repo.GetDB()

	// Get comprehensive trading metrics for this user
	metrics, err := db.GetFuturesTradingMetricsForUser(ctx, ga.userID)
	if err != nil {
		ga.logger.Error("Failed to load PnL metrics from database", "error", err, "user_id", ga.userID)
		return
	}

	// Get daily PnL and trade count
	dailyPnL, err := db.GetDailyFuturesPnLForUser(ctx, ga.userID)
	if err != nil {
		ga.logger.Warn("Failed to get daily PnL from database", "error", err)
		dailyPnL = 0
	}

	dailyTrades, err := db.GetDailyFuturesTradeCountForUser(ctx, ga.userID)
	if err != nil {
		ga.logger.Warn("Failed to get daily trade count from database", "error", err)
		dailyTrades = 0
	}

	// Set the values from database
	ga.totalPnL = metrics.TotalRealizedPnL
	ga.dailyPnL = dailyPnL
	ga.totalTrades = metrics.TotalTrades
	ga.winningTrades = metrics.WinningTrades
	ga.dailyTrades = dailyTrades

	ga.logger.Info("Loaded Ginie PnL stats from database (per-user)",
		"user_id", ga.userID,
		"total_pnl", ga.totalPnL,
		"daily_pnl", ga.dailyPnL,
		"total_trades", ga.totalTrades,
		"winning_trades", ga.winningTrades,
		"daily_trades", ga.dailyTrades,
		"win_rate", metrics.WinRate)
}

// SavePnLStats is deprecated for multi-user mode - PnL is calculated from trades in database
// For legacy (shared) mode, still persists to settings file
func (ga *GinieAutopilot) SavePnLStats() {
	// In multi-user mode, PnL is calculated from futures_trades table automatically
	// No need to persist separately - the trades themselves are the source of truth
	if ga.userID != "" {
		ga.logger.Debug("SavePnLStats skipped - using database for multi-user PnL tracking",
			"user_id", ga.userID)
		return
	}

	// Legacy mode: persist to shared settings file
	sm := GetSettingsManager()
	if err := sm.UpdateGiniePnLStats(
		ga.totalPnL,
		ga.dailyPnL,
		ga.totalTrades,
		ga.winningTrades,
		ga.dailyTrades,
	); err != nil {
		ga.logger.Error("Failed to save Ginie PnL stats", "error", err)
	}
}

// SetConfig updates the configuration
func (ga *GinieAutopilot) SetConfig(config *GinieAutopilotConfig) {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	ga.config = config
}

// SetLLMAnalyzer sets the LLM analyzer for adaptive SL/TP
// SetFuturesClient updates the futures client (used when switching between paper/live modes)
func (ga *GinieAutopilot) SetFuturesClient(client binance.FuturesClient) {
	ga.mu.Lock()
	ga.futuresClient = client
	ga.logger.Info("Ginie futures client updated")
	wasRunning := ga.running
	ga.mu.Unlock()

	// CRITICAL: Re-sync positions and auto-start when client is updated
	// This handles the case where container starts with mock client, then real client is injected
	if client != nil {
		go func() {
			// 1. Sync existing positions from exchange
			synced, err := ga.SyncWithExchange()
			if err != nil {
				ga.logger.Error("Failed to sync positions after client update", "error", err)
			} else if synced > 0 {
				ga.logger.Info("Synced existing exchange positions after client update", "count", synced)
			}

			// 2. Auto-start Ginie if enabled in settings and not already running
			if !wasRunning {
				settingsManager := GetSettingsManager()
				settings := settingsManager.GetCurrentSettings()
				if settings.GinieAutoStart {
					ga.logger.Info("Auto-starting Ginie autopilot (ginie_auto_start=true)")
					if err := ga.Start(); err != nil {
						ga.logger.Warn("Failed to auto-start Ginie", "error", err)
					} else {
						ga.logger.Info("Ginie autopilot auto-started successfully")
						// 3. Place SL/TP orders for synced positions
						if synced > 0 {
							ga.placeSLTPOrdersForSyncedPositions()
						}
					}
				}
			}
		}()
	}
}

func (ga *GinieAutopilot) SetLLMAnalyzer(analyzer *llm.Analyzer) {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	ga.llmAnalyzer = analyzer
	if analyzer != nil {
		ga.logger.Info("LLM analyzer set for Ginie adaptive SL/TP")
	}
}

// HasLLMAnalyzer returns true if an LLM analyzer is configured and enabled
func (ga *GinieAutopilot) HasLLMAnalyzer() bool {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	return ga.llmAnalyzer != nil && ga.llmAnalyzer.IsEnabled()
}

// getEffectivePositionSide determines the correct position side based on Binance account's position mode
// Returns PositionSideBoth for ONE_WAY mode, or the provided positionSide for HEDGE mode
func (ga *GinieAutopilot) getEffectivePositionSide(positionSide binance.PositionSide) binance.PositionSide {
	posMode, err := ga.futuresClient.GetPositionMode()
	if err != nil {
		log.Printf("[GINIE] Warning: Failed to get position mode, assuming ONE_WAY: %v", err)
		return binance.PositionSideBoth
	}

	if !posMode.DualSidePosition {
		// ONE_WAY mode - must use BOTH
		log.Printf("[GINIE] One-Way mode detected, using PositionSideBoth")
		return binance.PositionSideBoth
	}

	// HEDGE mode - use the provided position side
	log.Printf("[GINIE] Hedge mode detected, using %s", positionSide)
	return positionSide
}

// GetConfig returns current configuration
func (ga *GinieAutopilot) GetConfig() *GinieAutopilotConfig {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	return ga.config
}

// SetRiskLevel changes the current risk level dynamically
func (ga *GinieAutopilot) SetRiskLevel(level string) error {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	// Validate risk level
	validLevels := map[string]bool{"conservative": true, "moderate": true, "aggressive": true}
	if !validLevels[level] {
		return fmt.Errorf("invalid risk level: %s (must be conservative, moderate, or aggressive)", level)
	}

	oldLevel := ga.currentRiskLevel
	ga.currentRiskLevel = level
	ga.config.RiskLevel = level

	// Adjust parameters based on risk level
	switch level {
	case "conservative":
		ga.config.MinConfidenceToTrade = 60.0 // FIXED: Lowered from 75% to allow more trades
		ga.config.MaxUSDPerPosition = 300
		ga.config.DefaultLeverage = 3
	case "moderate":
		ga.config.MinConfidenceToTrade = 50.0 // FIXED: Lowered from 65% to match user expectations
		ga.config.MaxUSDPerPosition = 500
		ga.config.DefaultLeverage = 5
	case "aggressive":
		ga.config.MinConfidenceToTrade = 45.0 // FIXED: Lowered from 55% to be truly aggressive
		ga.config.MaxUSDPerPosition = 800
		ga.config.DefaultLeverage = 10
	}

	ga.logger.Info("Ginie risk level changed", map[string]interface{}{
		"from": oldLevel,
		"to":   level,
	})

	return nil
}

// GetRiskLevel returns the current risk level
func (ga *GinieAutopilot) GetRiskLevel() string {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	return ga.currentRiskLevel
}

// IsRunning returns whether the autopilot is running
func (ga *GinieAutopilot) IsRunning() bool {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	return ga.running
}

// GetPositions returns all active positions
func (ga *GinieAutopilot) GetPositions() []*GiniePosition {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	positions := make([]*GiniePosition, 0, len(ga.positions))
	for _, pos := range ga.positions {
		positions = append(positions, pos)
	}
	return positions
}

// GetBalanceInfo returns available and wallet balance from Binance (uses cache to avoid blocking API calls)
func (ga *GinieAutopilot) GetBalanceInfo() (availableBalance float64, walletBalance float64) {
	ga.mu.RLock()
	cachedAvailable := ga.cachedAvailableBalance
	cachedWallet := ga.cachedWalletBalance
	timeSinceUpdate := time.Since(ga.lastBalanceUpdateTime)
	ga.mu.RUnlock()

	// Return cached value if fresh (less than 30 seconds old)
	if timeSinceUpdate < 30*time.Second {
		return cachedAvailable, cachedWallet
	}

	// Always fetch in background to update cache, but return immediately
	// This prevents API endpoints from blocking on network calls
	select {
	case <-ga.stopChan:
		// If stopping, just return cached values
		return cachedAvailable, cachedWallet
	default:
		// Spawn background fetch if cache is stale
		go func() {
			accountInfo, err := ga.futuresClient.GetFuturesAccountInfo()
			if err != nil {
				ga.logger.Error("Failed to update balance info in background", "error", err)
				return
			}

			ga.mu.Lock()
			ga.cachedAvailableBalance = accountInfo.AvailableBalance
			ga.cachedWalletBalance = accountInfo.TotalWalletBalance
			ga.lastBalanceUpdateTime = time.Now()
			ga.mu.Unlock()
		}()

		// Return immediately with cached values
		return cachedAvailable, cachedWallet
	}
}

// ClearPositions clears all tracked positions and resets stats
func (ga *GinieAutopilot) ClearPositions() {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	ga.positions = make(map[string]*GiniePosition)
	ga.tradeHistory = make([]GinieTradeResult, 0)
	ga.dailyTrades = 0
	ga.dailyPnL = 0
	ga.totalPnL = 0
	ga.totalTrades = 0
	ga.winningTrades = 0

	ga.logger.Info("Ginie autopilot positions and stats cleared", nil)
}

// GetTradeHistory returns recent trade history
func (ga *GinieAutopilot) GetTradeHistory(limit int) []GinieTradeResult {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	if limit <= 0 || limit > len(ga.tradeHistory) {
		limit = len(ga.tradeHistory)
	}

	start := len(ga.tradeHistory) - limit
	if start < 0 {
		start = 0
	}

	result := make([]GinieTradeResult, limit)
	copy(result, ga.tradeHistory[start:])
	return result
}

// GetTradeHistoryInDateRange returns trade history within a date range
func (ga *GinieAutopilot) GetTradeHistoryInDateRange(startTime, endTime time.Time) []GinieTradeResult {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	var result []GinieTradeResult
	for _, trade := range ga.tradeHistory {
		if (startTime.IsZero() || trade.Timestamp.After(startTime) || trade.Timestamp.Equal(startTime)) &&
			(endTime.IsZero() || trade.Timestamp.Before(endTime) || trade.Timestamp.Equal(endTime)) {
			result = append(result, trade)
		}
	}
	return result
}

// RecordLLMSwitch records an LLM switch event (enable/disable coin)
func (ga *GinieAutopilot) RecordLLMSwitch(symbol string, action string, reason string) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	event := LLMSwitchEvent{
		Timestamp: time.Now(),
		Symbol:    symbol,
		Action:    action,
		Reason:    reason,
	}

	ga.llmSwitches = append(ga.llmSwitches, event)

	// Keep only the last maxLLMSwitches events
	if len(ga.llmSwitches) > ga.maxLLMSwitches {
		ga.llmSwitches = ga.llmSwitches[len(ga.llmSwitches)-ga.maxLLMSwitches:]
	}
}

// GetLLMSwitches returns recent LLM switch events (limit most recent)
func (ga *GinieAutopilot) GetLLMSwitches(limit int) []LLMSwitchEvent {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	if limit <= 0 || limit > len(ga.llmSwitches) {
		limit = len(ga.llmSwitches)
	}

	start := len(ga.llmSwitches) - limit
	if start < 0 {
		start = 0
	}

	result := make([]LLMSwitchEvent, limit)
	copy(result, ga.llmSwitches[start:])
	return result
}

// GetLLMSwitchesInDateRange returns LLM switches within a date range
func (ga *GinieAutopilot) GetLLMSwitchesInDateRange(startTime, endTime time.Time) []LLMSwitchEvent {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	var result []LLMSwitchEvent
	for _, event := range ga.llmSwitches {
		if (startTime.IsZero() || event.Timestamp.After(startTime) || event.Timestamp.Equal(startTime)) &&
			(endTime.IsZero() || event.Timestamp.Before(endTime) || event.Timestamp.Equal(endTime)) {
			result = append(result, event)
		}
	}
	return result
}

// ClearLLMSwitches clears all LLM switch history
func (ga *GinieAutopilot) ClearLLMSwitches() {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	ga.llmSwitches = make([]LLMSwitchEvent, 0, 500)
}

// Start starts the Ginie autopilot
func (ga *GinieAutopilot) Start() error {
	ga.logger.Info("Ginie Start() called", "dry_run", ga.config.DryRun, "current_running", ga.running)

	// CRITICAL FIX: Acquire lock FIRST before checking running state (prevents race conditions)
	ga.mu.Lock()
	if ga.running {
		ga.mu.Unlock()
		return fmt.Errorf("Ginie autopilot already running")
	}

	ga.running = true
	ga.config.Enabled = true // Set enabled flag to reflect running state
	ga.stopChan = make(chan struct{})
	ga.logger.Info("Starting Ginie Autopilot",
		"dry_run", ga.config.DryRun,
		"max_positions", ga.config.MaxPositions,
		"modes", fmt.Sprintf("scalp=%v swing=%v position=%v",
			ga.config.EnableScalpMode,
			ga.config.EnableSwingMode,
			ga.config.EnablePositionMode))

	// Start the main trading loops
	ga.wg.Add(1)
	go ga.runMainLoop()

	// Start position monitoring loop
	ga.wg.Add(1)
	go ga.runPositionMonitor()

	// Start adaptive SL/TP monitor (uses LLM to continuously adjust SL/TP)
	if ga.config.AdaptiveSLTPEnabled && ga.llmAnalyzer != nil {
		ga.wg.Add(1)
		go ga.runAdaptiveSLTPMonitor()
		ga.logger.Info("Adaptive SL/TP monitor started with LLM")
	}

	// Start ultra-fast scalping monitor (500ms polling for rapid exits)
	settingsManager := GetSettingsManager()
	currentSettings := settingsManager.GetCurrentSettings()
	if currentSettings.UltraFastEnabled {
		ga.wg.Add(1)
		go ga.monitorUltraFastPositions()
		ga.logger.Info("Ultra-fast scalping monitor started - 500ms polling enabled")
	}

	// Start daily reset goroutine (tracked in WaitGroup for clean shutdown)
	ga.wg.Add(1)
	go ga.resetDailyCounters()

	// Start hourly reset goroutine for profit booking metrics
	ga.wg.Add(1)
	go ga.resetHourlyCounters()

	// Start periodic orphan order cleanup (every 5 minutes)
	// This prevents order accumulation from position updates and failed cancellations
	ga.wg.Add(1)
	go ga.periodicOrphanOrderCleanup()

	// Start protection guardian (bulletproof SL/TP monitoring every 5 seconds)
	// This is the core of the bulletproof protection system
	ga.wg.Add(1)
	go ga.runProtectionGuardian()

	ga.mu.Unlock()
	// CRITICAL: Release lock BEFORE doing any blocking operations (prevents API handler timeouts)

	// BACKGROUND: Run heavy initialization tasks in background goroutine
	// This prevents the API endpoint from timing out due to Binance API calls
	if !ga.config.DryRun {
		go func() {
			// Load watchlist from user's coin source settings
			// This uses configured sources: saved coins, LLM selection, market movers
			currentWatchlist := ga.analyzer.GetWatchSymbols()
			ga.logger.Info("Current watchlist size on start", "count", len(currentWatchlist))

			// Always load from user coin sources on startup
			ga.logger.Info("Loading watchlist from user coin source settings")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := ga.LoadUserCoinSources(ctx)
			cancel()
			if err != nil {
				ga.logger.Error("Failed to load user coin sources, trying fallback", "error", err)
				// Fallback to market movers if user settings fail
				err = ga.analyzer.LoadDynamicSymbols(50)
				if err != nil {
					ga.logger.Error("Failed to load market movers fallback", "error", err)
				}
			}
			updatedWatchlist := ga.analyzer.GetWatchSymbols()
			ga.logger.Info("Watchlist loaded from user config", "new_count", len(updatedWatchlist))

			// Sync positions with exchange (this can be slow)
			synced, err := ga.SyncWithExchange()
			if err != nil {
				ga.logger.Warn("Failed to sync positions on start", "error", err)
			} else if synced > 0 {
				ga.logger.Info("Synced positions from exchange on start", "count", synced)
			}

			// Place SL/TP orders for all existing positions (including those synced during initialization)
			ga.placeSLTPOrdersForSyncedPositions()

			// CRITICAL: Run comprehensive orphan order cleanup at startup
			ga.logger.Info("Running startup orphan order cleanup in background")
			ga.cleanupAllOrphanOrders()

			ga.logger.Info("Ginie startup initialization tasks completed")
		}()
	}

	ga.logger.Info("Ginie Autopilot started - initialization tasks running in background")
	return nil
}

// Stop stops the Ginie autopilot
func (ga *GinieAutopilot) Stop() error {
	ga.mu.Lock()
	if !ga.running {
		ga.mu.Unlock()
		return fmt.Errorf("Ginie autopilot not running")
	}
	ga.running = false
	ga.config.Enabled = false // Set enabled flag to reflect running state
	close(ga.stopChan)
	ga.mu.Unlock()

	// CRITICAL FIX: Don't block waiting for goroutines - return immediately
	// Let goroutines clean up in background
	go func() {
		ga.wg.Wait()
		ga.logger.Info("Ginie Autopilot stopped (background cleanup completed)")
	}()

	ga.logger.Info("Ginie Autopilot stop initiated - cleanup running in background")
	return nil
}

// runMainLoop is the main trading loop that scans for opportunities
func (ga *GinieAutopilot) runMainLoop() {
	defer ga.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			ga.logger.Error("PANIC in Ginie main loop - restarting", "panic", r)
			log.Printf("[GINIE-PANIC] Main loop panic: %v", r)
			// Restart the loop after a brief delay
			time.Sleep(5 * time.Second)
			ga.wg.Add(1)
			go ga.runMainLoop()
		}
	}()

	// Set initial phase (Issue 2B)
	ga.mu.Lock()
	ga.currentPhase = "initializing"
	ga.mu.Unlock()

	ga.logger.Info("Ginie main loop started")

	// Use the shortest enabled interval as base, then check mode-specific timing
	baseTicker := time.NewTicker(time.Duration(ga.config.ScalpScanInterval) * time.Second)
	defer baseTicker.Stop()

	// Track last scan times for each mode
	lastScalpScan := time.Now()
	lastSwingScan := time.Now()
	lastPositionScan := time.Now()
	lastStrategyScan := time.Now()
	lastUltraFastScan := time.Now() // Ultra-fast mode: 5-second scan interval
	lastWatchlistRefresh := time.Now() // Periodic watchlist refresh

	// Set phase to idle after initialization (Issue 2B)
	ga.mu.Lock()
	ga.currentPhase = "idle"
	ga.mu.Unlock()

	for {
		select {
		case <-ga.stopChan:
			ga.logger.Info("Ginie main loop stopping")
			return
		case <-baseTicker.C:
			now := time.Now()

			// Check if we can trade
			canTrade := ga.canTrade()
			log.Printf("[GINIE-SCAN] canTrade=%v, positions=%d/%d", canTrade, len(ga.positions), ga.config.MaxPositions)
			if !canTrade {
				// Set phase to idle when not trading (Issue 2B)
				ga.mu.Lock()
				ga.currentPhase = "idle"
				ga.scanningActive = false
				ga.mu.Unlock()
				continue
			}

			ga.logger.Info("Ginie canTrade passed, proceeding with scans")

			// Mode Integration Status (Story 2.5 verification)
			modesEnabled := 0
			var enabledModes []string
			if ga.config.EnableScalpMode {
				modesEnabled++
				enabledModes = append(enabledModes, "SCALP")
			}
			if ga.config.EnableSwingMode {
				modesEnabled++
				enabledModes = append(enabledModes, "SWING")
			}
			if ga.config.EnablePositionMode {
				modesEnabled++
				enabledModes = append(enabledModes, "POSITION")
			}
			currentSettings := GetSettingsManager().GetCurrentSettings()
			if currentSettings.UltraFastEnabled {
				modesEnabled++
				enabledModes = append(enabledModes, "ULTRA-FAST")
			}
			if modesEnabled > 1 {
				log.Printf("[MODE-ORCHESTRATION] Multi-mode trading active: %d modes enabled [%s]", modesEnabled, strings.Join(enabledModes, ", "))
			} else if modesEnabled == 1 {
				log.Printf("[MODE-ORCHESTRATION] Single-mode trading: %s enabled", enabledModes[0])
			} else {
				log.Printf("[MODE-ORCHESTRATION] WARNING: No trading modes enabled!")
			}

			// Track scan cycle timing (Issue 2B)
			scanCycleStart := time.Now()
			scansPerformed := 0

			// Set scanning phase (Issue 2B)
			ga.mu.Lock()
			ga.currentPhase = "scanning"
			ga.scanningActive = true
			ga.mu.Unlock()

			// Scan based on mode intervals
			if ga.config.EnableScalpMode && now.Sub(lastScalpScan) >= time.Duration(ga.config.ScalpScanInterval)*time.Second {
				log.Printf("[MODE-SCAN] Scanning SCALP mode (interval: %ds)", ga.config.ScalpScanInterval)
				ga.scanForMode(GinieModeScalp)
				lastScalpScan = now
				scansPerformed++
			}

			if ga.config.EnableSwingMode && now.Sub(lastSwingScan) >= time.Duration(ga.config.SwingScanInterval)*time.Second {
				log.Printf("[MODE-SCAN] Scanning SWING mode (interval: %ds)", ga.config.SwingScanInterval)
				ga.scanForMode(GinieModeSwing)
				lastSwingScan = now
				scansPerformed++
			}

			if ga.config.EnablePositionMode && now.Sub(lastPositionScan) >= time.Duration(ga.config.PositionScanInterval)*time.Second {
				log.Printf("[MODE-SCAN] Scanning POSITION mode (interval: %ds)", ga.config.PositionScanInterval)
				ga.scanForMode(GinieModePosition)
				lastPositionScan = now
				scansPerformed++
			}

			// Ultra-fast mode: 5-second scan for rapid scalping opportunities
			// Uses milliseconds for interval, converts to duration
			// Note: currentSettings already fetched above for mode counting
			if currentSettings.UltraFastEnabled {
				ultraFastInterval := time.Duration(currentSettings.UltraFastScanInterval) * time.Millisecond
				if now.Sub(lastUltraFastScan) >= ultraFastInterval {
					log.Printf("[ULTRA-FAST-SCAN] Starting ultra-fast scan cycle at %s", now.Format("15:04:05.000"))
					ga.scanForUltraFast()
					lastUltraFastScan = now
					scansPerformed++
				}
			}

			// Scan saved strategies (every 60 seconds)
			if ga.strategyEvaluator != nil && now.Sub(lastStrategyScan) >= 60*time.Second {
				ga.scanStrategies()
				lastStrategyScan = now
			}

			// Refresh watchlist based on user's coin source settings (every 30 minutes)
			// This uses user-configured sources: saved coins, LLM selection, market movers
			if now.Sub(lastWatchlistRefresh) >= 30*time.Minute {
				go func() {
					currentCount := len(ga.analyzer.GetWatchSymbols())
					ga.logger.Info("Refreshing watchlist from user coin sources", "current_count", currentCount)
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					err := ga.LoadUserCoinSources(ctx)
					if err != nil {
						ga.logger.Warn("Failed to refresh watchlist from user sources", "error", err)
					} else {
						newCount := len(ga.analyzer.GetWatchSymbols())
						ga.logger.Info("Watchlist refreshed from user config successfully", "new_count", newCount)
					}
				}()
				lastWatchlistRefresh = now
			}

			// Update scan status after scan cycle (Issue 2B)
			if scansPerformed > 0 {
				ga.mu.Lock()
				ga.lastScanTime = time.Now()
				ga.scanDuration = time.Since(scanCycleStart)
				// Calculate next scan time based on shortest enabled interval
				shortestInterval := time.Duration(ga.config.ScalpScanInterval) * time.Second
				if currentSettings.UltraFastEnabled {
					ultraFastInterval := time.Duration(currentSettings.UltraFastScanInterval) * time.Millisecond
					if ultraFastInterval < shortestInterval {
						shortestInterval = ultraFastInterval
					}
				}
				ga.nextScanTime = time.Now().Add(shortestInterval)
				ga.currentPhase = "idle"
				ga.scanningActive = false
				ga.mu.Unlock()
			}
		}
	}
}

// LoadUserCoinSources loads coins based on user's scan source settings
// This replaces the hardcoded coin selection with user-configurable sources
func (ga *GinieAutopilot) LoadUserCoinSources(ctx context.Context) error {
	if ga.userID == "" || ga.repo == nil {
		ga.logger.Warn("No user ID or repo, falling back to market movers")
		return ga.analyzer.LoadDynamicSymbols(50)
	}

	// Get user's scan source settings
	settings, err := ga.repo.GetUserScanSourceSettings(ctx, ga.userID)
	if err != nil {
		ga.logger.Warn("Failed to get user scan source settings, falling back to market movers", "error", err)
		return ga.analyzer.LoadDynamicSymbols(50)
	}

	// Build coin list from enabled sources
	coinSet := make(map[string]bool)

	// 1. Saved Coins (highest priority - user's explicit selections)
	if settings.UseSavedCoins && len(settings.SavedCoins) > 0 {
		for _, coin := range settings.SavedCoins {
			coinSet[coin] = true
		}
		ga.logger.Info("Loaded saved coins from user config", "count", len(settings.SavedCoins))
	}

	// 2. LLM Selection (if enabled)
	if settings.UseLLMList {
		// Try to load LLM coins (uses cache if valid)
		if err := ga.analyzer.LoadLLMSelectedCoins(); err != nil {
			ga.logger.Warn("Failed to load LLM coins", "error", err)
		} else {
			llmCoins := ga.analyzer.GetLLMSelectedCoins()
			for _, coin := range llmCoins {
				coinSet[coin] = true
			}
			ga.logger.Info("Added LLM-selected coins", "count", len(llmCoins))
		}
	}

	// 3. Market Movers (if enabled)
	if settings.UseMarketMovers {
		// Determine max limit needed
		maxLimit := settings.GainersLimit
		if settings.LosersLimit > maxLimit {
			maxLimit = settings.LosersLimit
		}
		if settings.VolumeLimit > maxLimit {
			maxLimit = settings.VolumeLimit
		}
		if settings.VolatilityLimit > maxLimit {
			maxLimit = settings.VolatilityLimit
		}
		if maxLimit < 10 {
			maxLimit = 10
		}

		movers, err := ga.analyzer.GetMarketMovers(maxLimit)
		if err == nil {
			if settings.MoverGainers {
				for i, coin := range movers.TopGainers {
					if i >= settings.GainersLimit {
						break
					}
					coinSet[coin] = true
				}
			}
			if settings.MoverLosers {
				for i, coin := range movers.TopLosers {
					if i >= settings.LosersLimit {
						break
					}
					coinSet[coin] = true
				}
			}
			if settings.MoverVolume {
				for i, coin := range movers.TopVolume {
					if i >= settings.VolumeLimit {
						break
					}
					coinSet[coin] = true
				}
			}
			if settings.MoverVolatility {
				for i, coin := range movers.HighVolatility {
					if i >= settings.VolatilityLimit {
						break
					}
					coinSet[coin] = true
				}
			}
			ga.logger.Info("Added market mover coins from user config")
		} else {
			ga.logger.Warn("Failed to get market movers", "error", err)
		}
	}

	// Convert to slice and apply max_coins limit
	coins := make([]string, 0, len(coinSet))
	for coin := range coinSet {
		coins = append(coins, coin)
	}

	// Limit to max_coins
	if len(coins) > settings.MaxCoins && settings.MaxCoins > 0 {
		coins = coins[:settings.MaxCoins]
	}

	// If no coins found from user settings, fall back to core coins
	if len(coins) == 0 {
		ga.logger.Warn("No coins from user config, using default market movers")
		return ga.analyzer.LoadDynamicSymbols(50)
	}

	// Update the analyzer's watch list
	ga.analyzer.SetWatchSymbols(coins)
	ga.logger.Info("Loaded coins from user scan source config",
		"total", len(coins),
		"saved_enabled", settings.UseSavedCoins,
		"llm_enabled", settings.UseLLMList,
		"movers_enabled", settings.UseMarketMovers,
		"max_coins", settings.MaxCoins)

	return nil
}

// canTrade checks if trading is allowed
func (ga *GinieAutopilot) canTrade() bool {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	// Check circuit breaker first (if enabled)
	if ga.config.CircuitBreakerEnabled && ga.circuitBreaker != nil {
		canTrade, reason := ga.circuitBreaker.CanTrade()
		if !canTrade {
			ga.logger.Warn("Ginie circuit breaker blocking trades", "reason", reason)
			return false
		}
	}

	// Check daily limits
	if ga.config.MaxDailyTrades > 0 && ga.dailyTrades >= ga.config.MaxDailyTrades {
		ga.logger.Warn("Ginie max daily trades reached", "current", ga.dailyTrades, "max", ga.config.MaxDailyTrades)
		return false
	}

	if ga.config.MaxDailyLoss > 0 && ga.dailyPnL <= -ga.config.MaxDailyLoss {
		ga.logger.Warn("Ginie max daily loss reached", "current_loss", ga.dailyPnL, "max_loss", -ga.config.MaxDailyLoss)
		return false
	}

	// Check max positions
	if len(ga.positions) >= ga.config.MaxPositions {
		ga.logger.Warn("Ginie max positions reached", "current", len(ga.positions), "max", ga.config.MaxPositions)
		return false
	}

	return true
}

// scanForUltraFast scans all watched symbols for ultra-fast scalping opportunities
// Uses 4-layer signal generation: Trend, Volatility, Entry, Dynamic TP
// Calls executeUltraFastEntry when confidence >= threshold
func (ga *GinieAutopilot) scanForUltraFast() {
	symbols := ga.analyzer.watchSymbols
	settingsManager := GetSettingsManager()
	currentSettings := settingsManager.GetCurrentSettings()

	// Track scan time for diagnostics
	ga.mu.Lock()
	ga.lastUltraFastScan = time.Now()
	ga.symbolsScannedLastCycle = len(symbols)
	// Initialize progress tracking (Issue 2B)
	ga.scannedThisCycle = 0
	ga.totalSymbols = len(symbols)
	ga.mu.Unlock()

	log.Printf("[ULTRA-FAST-SCAN] Scanning %d symbols, min_confidence=%d%%", len(symbols), currentSettings.UltraFastMinConfidence)

	// Count current ultra-fast positions
	ga.mu.RLock()
	currentUFCount := 0
	for _, pos := range ga.positions {
		if pos.Mode == GinieModeUltraFast {
			currentUFCount++
		}
	}
	maxUFPositions := currentSettings.UltraFastMaxPositions
	ga.mu.RUnlock()

	if currentUFCount >= maxUFPositions {
		log.Printf("[ULTRA-FAST-SCAN] Position limit reached: %d/%d, skipping scan", currentUFCount, maxUFPositions)
		return
	}

	// Check daily trade limit
	if ga.dailyTrades >= currentSettings.UltraFastMaxDailyTrades {
		log.Printf("[ULTRA-FAST-SCAN] Daily trade limit reached: %d/%d, skipping scan", ga.dailyTrades, currentSettings.UltraFastMaxDailyTrades)
		return
	}

	signalsGenerated := 0
	tradesAttempted := 0

	for _, symbol := range symbols {
		select {
		case <-ga.stopChan:
			log.Printf("[ULTRA-FAST-SCAN] Scan interrupted by stop signal")
			return
		default:
			// Skip if we already have a position for this symbol (any mode)
			ga.mu.RLock()
			_, hasPosition := ga.positions[symbol]
			ga.mu.RUnlock()

			if hasPosition {
				continue
			}

			// Generate ultra-fast signal using 4-layer analysis
			signal, err := ga.analyzer.GenerateUltraFastSignal(symbol)
			if err != nil {
				log.Printf("[ULTRA-FAST-SCAN] %s: Signal generation failed: %v", symbol, err)
				continue
			}

			signalsGenerated++

			// Log signal details
			log.Printf("[ULTRA-FAST-SCAN] %s: TrendBias=%s (%.1f%%), EntryConfidence=%.1f%%, VolatilityRegime=%s, MinTP=%.2f%%",
				symbol,
				signal.TrendBias,
				signal.TrendStrength,
				signal.EntryConfidence,
				signal.VolatilityRegime.Level,
				signal.MinProfitTarget)

			// Get current price for signal log
			currentPrice, _ := ga.futuresClient.GetFuturesCurrentPrice(symbol)

			// Build signal log entry for ultra-fast signal
			signalLog := &GinieSignalLog{
				Symbol:       symbol,
				Direction:    signal.TrendBias, // LONG or SHORT
				Mode:         "ultra_fast",
				Confidence:   signal.EntryConfidence,
				CurrentPrice: currentPrice,
				Trend:        signal.TrendBias,
				Volatility:   signal.VolatilityRegime.Level,
				ATRPercent:   signal.VolatilityRegime.ATRRatio * 100, // Convert ratio to percentage estimate
				SignalNames:  []string{"TrendBias", "EntryConfidence", "VolatilityRegime", "MinProfitTarget"},
			}

			// Check confidence threshold
			minConfidence := float64(currentSettings.UltraFastMinConfidence)
			if signal.EntryConfidence < minConfidence {
				log.Printf("[ULTRA-FAST-SCAN] %s: Confidence %.1f%% < threshold %.1f%%, SKIP",
					symbol, signal.EntryConfidence, minConfidence)
				signalLog.Status = "rejected"
				signalLog.RejectionReason = fmt.Sprintf("low_confidence (%.1f < %.1f)", signal.EntryConfidence, minConfidence)
				ga.LogSignal(signalLog)
				continue
			}

			// Check trend bias (skip NEUTRAL for entries)
			if signal.TrendBias == "NEUTRAL" {
				log.Printf("[ULTRA-FAST-SCAN] %s: Neutral trend, SKIP", symbol)
				signalLog.Status = "rejected"
				signalLog.RejectionReason = "neutral_trend"
				ga.LogSignal(signalLog)
				continue
			}

			// Confidence met - attempt entry
			log.Printf("[ULTRA-FAST-SCAN] %s: ✓ ENTRY SIGNAL - Confidence %.1f%% >= %.1f%%, Direction=%s",
				symbol, signal.EntryConfidence, minConfidence, signal.TrendBias)

			tradesAttempted++

			// Execute the ultra-fast entry
			err = ga.executeUltraFastEntry(symbol, signal)
			if err != nil {
				log.Printf("[ULTRA-FAST-SCAN] %s: Entry execution failed: %v", symbol, err)
				signalLog.Status = "rejected"
				signalLog.RejectionReason = fmt.Sprintf("execution_failed: %v", err)
				ga.LogSignal(signalLog)
			} else {
				log.Printf("[ULTRA-FAST-SCAN] %s: ✓ ENTRY EXECUTED - Now monitoring for exit", symbol)
				signalLog.Status = "executed"
				ga.LogSignal(signalLog)

				// Check if we've hit position limit after this entry
				ga.mu.RLock()
				newUFCount := 0
				for _, pos := range ga.positions {
					if pos.Mode == GinieModeUltraFast {
						newUFCount++
					}
				}
				ga.mu.RUnlock()

				if newUFCount >= maxUFPositions {
					log.Printf("[ULTRA-FAST-SCAN] Position limit reached after entry: %d/%d, stopping scan", newUFCount, maxUFPositions)
					break
				}
			}
		}
	}

	log.Printf("[ULTRA-FAST-SCAN] Scan complete: %d signals generated, %d trade attempts", signalsGenerated, tradesAttempted)

	// Update progress to show scan completed (Issue 2B)
	ga.mu.Lock()
	ga.scannedThisCycle = len(symbols)
	ga.mu.Unlock()
}

// scanForMode scans all watched symbols for a specific trading mode
func (ga *GinieAutopilot) scanForMode(mode GinieTradingMode) {
	// DEFENSE-IN-DEPTH: Verify mode is actually enabled before scanning
	// This catches any edge cases where the main loop check was bypassed
	if !ga.isModeEnabled(mode) {
		log.Printf("[%s-SCAN] Mode disabled in user settings, skipping scan", mode)
		return
	}

	symbols := ga.analyzer.watchSymbols

	// Track scan time for diagnostics
	ga.mu.Lock()
	now := time.Now()
	switch mode {
	case GinieModeScalp:
		ga.lastScalpScan = now
	case GinieModeSwing:
		ga.lastSwingScan = now
	case GinieModePosition:
		ga.lastPositionScan = now
	}
	ga.symbolsScannedLastCycle = len(symbols)
	// Initialize progress tracking (Issue 2B)
	ga.scannedThisCycle = 0
	ga.totalSymbols = len(symbols)
	ga.mu.Unlock()

	ga.logger.Info("Ginie scanning for mode", "mode", mode, "symbols", len(symbols))

	// Mode-specific variables for logging
	isScalpMode := mode == GinieModeScalp
	var scalpTrades int // Track successful scalp entries

	// Mode-specific scan cycle logging for debugging (Epic 2 Stories 2.2-2.4)
	switch mode {
	case GinieModeScalp:
		log.Printf("[SCALP-SCAN] Starting scalp scan cycle, scanning %d symbols, min_confidence=%.1f%%", len(symbols), ga.config.MinConfidenceToTrade)
	case GinieModeSwing:
		log.Printf("[SWING-SCAN] Starting swing scan cycle, scanning %d symbols, min_confidence=%.1f%%", len(symbols), ga.config.MinConfidenceToTrade)
	case GinieModePosition:
		log.Printf("[POSITION-SCAN] Starting position scan cycle, scanning %d symbols for long-term opportunities", len(symbols))
	}

	var scalpSignals, swingSignals, positionSignals int

	for _, symbol := range symbols {
		select {
		case <-ga.stopChan:
			if isScalpMode {
				log.Printf("[SCALP-SCAN] Scan interrupted by stop signal")
			}
			return
		default:
			// Skip if we already have a position
			ga.mu.RLock()
			_, hasPosition := ga.positions[symbol]
			ga.mu.RUnlock()

			if hasPosition {
				continue
			}

			// Generate decision for this symbol using the specific mode being scanned
			// This allows each mode (scalp/swing/position) to be evaluated independently
			decision, err := ga.analyzer.GenerateDecisionForMode(symbol, mode)
			if err != nil {
				if isScalpMode {
					log.Printf("[SCALP-SCAN] %s: Signal generation failed: %v", symbol, err)
				}
				ga.logger.Error("Ginie decision generation failed", "symbol", symbol, "mode", mode, "error", err)
				continue
			}

			// Scalp mode: Log detailed signal evaluation per symbol (AC-2.2.2)
			if isScalpMode {
				scalpSignals++
				// Log market conditions overview
				log.Printf("[SCALP-SCAN] %s: Trend=%s, ADX=%.1f, Volatility=%s, Confidence=%.1f%%, Action=%s, RR=%.2f",
					symbol,
					decision.MarketConditions.Trend,
					decision.MarketConditions.ADX,
					decision.MarketConditions.Volatility,
					decision.ConfidenceScore,
					decision.TradeExecution.Action,
					decision.TradeExecution.RiskReward)
				// Log signal summary with all 4 signals evaluated
				log.Printf("[SCALP-SCAN] %s: Signals %d/%d met - [%s]",
					symbol,
					decision.SignalAnalysis.PrimaryMet,
					decision.SignalAnalysis.PrimaryRequired,
					decision.SignalAnalysis.SignalStrength)
				// Log each individual signal status (AC-2.2.2 requirement)
				for i, sig := range decision.SignalAnalysis.PrimarySignals {
					statusStr := "NOT_MET"
					if sig.Met {
						statusStr = "MET"
					}
					log.Printf("[SCALP-SCAN] %s: Signal[%d] %s=%s (value=%.2f, threshold=%.2f, weight=%.1f)",
						symbol, i+1, sig.Name, statusStr, sig.Value, sig.Threshold, sig.Weight)
				}
			}

			// No need to check mode match - we explicitly requested this mode

			// Build signal log entry
			entryPrice := (decision.TradeExecution.EntryLow + decision.TradeExecution.EntryHigh) / 2
			if entryPrice == 0 {
				entryPrice = decision.TradeExecution.EntryHigh
			}
			signalLog := &GinieSignalLog{
				Symbol:      symbol,
				Direction:   decision.TradeExecution.Action,
				Mode:        string(decision.SelectedMode),
				Confidence:  decision.ConfidenceScore,
				EntryPrice:  entryPrice,
				StopLoss:    decision.TradeExecution.StopLoss,
				Leverage:    decision.TradeExecution.Leverage,
				RiskReward:  decision.TradeExecution.RiskReward,
				Trend:       decision.MarketConditions.Trend,
				Volatility:  decision.MarketConditions.Volatility,
				ATRPercent:  decision.MarketConditions.ATR,
			}
			if len(decision.TradeExecution.TakeProfits) > 0 {
				signalLog.TakeProfit1 = decision.TradeExecution.TakeProfits[0].Price
			}
			// Get signal info from SignalAnalysis
			signalLog.PrimaryMet = decision.SignalAnalysis.PrimaryMet
			signalLog.PrimaryRequired = decision.SignalAnalysis.PrimaryRequired
			// Build signal names from primary signals
			for _, sig := range decision.SignalAnalysis.PrimarySignals {
				signalLog.SignalNames = append(signalLog.SignalNames, sig.Name)
			}
			// Get current price for context
			if price, err := ga.futuresClient.GetFuturesCurrentPrice(symbol); err == nil {
				signalLog.CurrentPrice = price
			}

			// Mode-specific per-symbol signal logging (Epic 2 Stories 2.2-2.4)
			// NOTE: Scalp mode detailed logging is handled earlier (AC-2.2.2)
			switch mode {
			case GinieModeSwing:
				swingSignals++
				// AC-2.3.2: Log market conditions overview
				log.Printf("[SWING-SCAN] %s: Trend=%s, ADX=%.1f, Volatility=%s, Confidence=%.1f%%, Action=%s, RR=%.2f",
					symbol, decision.MarketConditions.Trend, decision.MarketConditions.ADX,
					decision.MarketConditions.Volatility, decision.ConfidenceScore,
					decision.TradeExecution.Action, decision.TradeExecution.RiskReward)
				// AC-2.3.2: Log signal summary
				log.Printf("[SWING-SCAN] %s: Signals %d/%d met - [%s]",
					symbol, decision.SignalAnalysis.PrimaryMet, decision.SignalAnalysis.PrimaryRequired,
					decision.SignalAnalysis.SignalStrength)
				// AC-2.3.2: Log each individual signal with detailed values
				for i, sig := range decision.SignalAnalysis.PrimarySignals {
					statusStr := "NOT_MET"
					if sig.Met {
						statusStr = "MET"
					}
					log.Printf("[SWING-SCAN] %s: Signal[%d] %s=%s (value=%.2f, threshold=%.2f, weight=%.1f)",
						symbol, i+1, sig.Name, statusStr, sig.Value, sig.Threshold, sig.Weight)
				}
			case GinieModePosition:
				positionSignals++
				log.Printf("[POSITION-SCAN] %s: Trend=%s, ADX=%.1f, Volatility=%s, Confidence=%.1f%%, Action=%s, RR=%.2f",
					symbol, decision.MarketConditions.Trend, decision.MarketConditions.ADX,
					decision.MarketConditions.Volatility, decision.ConfidenceScore,
					decision.TradeExecution.Action, decision.TradeExecution.RiskReward)
				for i, sig := range decision.SignalAnalysis.PrimarySignals {
					log.Printf("[POSITION-SCAN] %s: Signal[%d] %s=%s (weight=%.1f)", symbol, i+1, sig.Name, sig.Status, sig.Weight)
				}
			}

			// Check recommendation
			if decision.Recommendation != RecommendationExecute {
				// Scalp mode: Log recommendation rejection (AC-2.2.2)
				if isScalpMode {
					log.Printf("[SCALP-SCAN] %s: Recommendation=%s, SKIP", symbol, decision.Recommendation)
				}
				if mode == GinieModeSwing {
					log.Printf("[SWING-SCAN] %s: Not recommended (recommendation=%s), SKIP", symbol, decision.Recommendation)
				}
				// Position mode: Log recommendation rejection (AC-2.4.1)
				if mode == GinieModePosition {
					log.Printf("[POSITION-SCAN] %s: Not recommended (recommendation=%s), SKIP", symbol, decision.Recommendation)
				}
				signalLog.Status = "rejected"
				// Include actual rejection reason from decision
				if decision.RecommendationNote != "" {
					signalLog.RejectionReason = decision.RecommendationNote
				} else {
					signalLog.RejectionReason = string(decision.Recommendation)
				}
				// Copy rejection tracking details to signal log
				if decision.RejectionTracking != nil && len(decision.RejectionTracking.AllReasons) > 0 {
					signalLog.RejectionDetails = &SignalRejectionDetails{
						AllReasons: decision.RejectionTracking.AllReasons,
					}
					// Map trend divergence if present
					if decision.RejectionTracking.TrendDivergence != nil {
						signalLog.RejectionDetails.TrendDivergence = &TrendDivergenceInfo{
							ScanTimeframe:     decision.RejectionTracking.TrendDivergence.ScanTimeframe,
							ScanTrend:         decision.RejectionTracking.TrendDivergence.ScanTrend,
							DecisionTimeframe: decision.RejectionTracking.TrendDivergence.DecisionTimeframe,
							DecisionTrend:     decision.RejectionTracking.TrendDivergence.DecisionTrend,
							Severity:          decision.RejectionTracking.TrendDivergence.Severity,
						}
					}
					// Map counter-trend rejection if present
					if decision.RejectionTracking.CounterTrend != nil {
						signalLog.RejectionDetails.CounterTrend = &CounterTrendInfo{
							SignalDirection: decision.RejectionTracking.CounterTrend.SignalDirection,
							TrendDirection:  decision.RejectionTracking.CounterTrend.TrendDirection,
							MissingSignals:  decision.RejectionTracking.CounterTrend.MissingRequirements,
						}
					}
				}
				ga.LogSignal(signalLog)
				continue
			}

			// Check if symbol is enabled (per-symbol settings)
			settingsManager := GetSettingsManager()
			if !settingsManager.IsSymbolEnabled(symbol) {
				// Scalp mode: Log symbol disabled (AC-2.2.2)
				if isScalpMode {
					log.Printf("[SCALP-SCAN] %s: Symbol disabled, SKIP", symbol)
				}
				if mode == GinieModeSwing {
					log.Printf("[SWING-SCAN] %s: Symbol disabled, SKIP", symbol)
				}
				// Position mode: Log symbol disabled (AC-2.4.1)
				if mode == GinieModePosition {
					log.Printf("[POSITION-SCAN] %s: Symbol disabled, SKIP", symbol)
				}
				signalLog.Status = "rejected"
				signalLog.RejectionReason = "symbol_disabled"
				ga.LogSignal(signalLog)
				continue
			}

			// Get effective confidence threshold for this symbol (considers performance category)
			effectiveMinConfidence := settingsManager.GetEffectiveConfidence(symbol, ga.config.MinConfidenceToTrade)

			// Get symbol category for logging
			symbolSettings := settingsManager.GetSymbolSettings(symbol)
			categoryBoost := effectiveMinConfidence - ga.config.MinConfidenceToTrade

			// Check confidence threshold (both are 0-100 format)
			if decision.ConfidenceScore < effectiveMinConfidence {
				// Enhanced logging with category boost information
				boostInfo := ""
				if categoryBoost > 0 {
					boostInfo = fmt.Sprintf(" (base %.1f%% + %.1f%% boost for '%s' category)",
						ga.config.MinConfidenceToTrade, categoryBoost, symbolSettings.Category)
				} else if categoryBoost < 0 {
					boostInfo = fmt.Sprintf(" (base %.1f%% - %.1f%% bonus for '%s' category)",
						ga.config.MinConfidenceToTrade, -categoryBoost, symbolSettings.Category)
				}

				// Scalp mode: Log confidence rejection (AC-2.2.2)
				if isScalpMode {
					log.Printf("[SCALP-SCAN] %s: Confidence %.1f%% < threshold %.1f%%%s, SKIP",
						symbol, decision.ConfidenceScore, effectiveMinConfidence, boostInfo)
				}
				// Swing mode: Log confidence rejection (AC-2.3.2)
				if mode == GinieModeSwing {
					log.Printf("[SWING-SCAN] %s: Confidence %.1f%% < threshold %.1f%%%s, SKIP",
						symbol, decision.ConfidenceScore, effectiveMinConfidence, boostInfo)
				}
				// Position mode: Log confidence rejection (AC-2.4.1)
				if mode == GinieModePosition {
					log.Printf("[POSITION-SCAN] %s: Confidence %.1f%% < threshold %.1f%%%s, SKIP",
						symbol, decision.ConfidenceScore, effectiveMinConfidence, boostInfo)
				}
				ga.logger.Debug("Ginie skipping low confidence signal",
					"symbol", symbol,
					"confidence", decision.ConfidenceScore,
					"min_required", effectiveMinConfidence,
					"global_min", ga.config.MinConfidenceToTrade,
					"category", symbolSettings.Category,
					"category_boost", categoryBoost)
				signalLog.Status = "rejected"
				signalLog.RejectionReason = fmt.Sprintf("low_confidence (%.1f < %.1f)%s",
					decision.ConfidenceScore, effectiveMinConfidence, boostInfo)
				ga.LogSignal(signalLog)
				continue
			}

			// Check if coin is blocked
			if blocked, reason := ga.isCoinBlocked(symbol); blocked {
				// Scalp mode: Log coin blocked (AC-2.2.2)
				if isScalpMode {
					log.Printf("[SCALP-SCAN] %s: Coin blocked (%s), SKIP", symbol, reason)
				}
				// Swing mode: Log coin blocked (AC-2.3.2)
				if mode == GinieModeSwing {
					log.Printf("[SWING-SCAN] %s: Coin blocked (%s), SKIP", symbol, reason)
				}
				// Position mode: Log coin blocked (AC-2.4.1)
				if mode == GinieModePosition {
					log.Printf("[POSITION-SCAN] %s: Coin blocked (%s), SKIP", symbol, reason)
				}
				signalLog.Status = "rejected"
				signalLog.RejectionReason = "coin_blocked: " + reason
				ga.LogSignal(signalLog)
				continue
			}


			// CRITICAL: Skip if action is WAIT or CLOSE - these are not entry signals
			tradeAction := decision.TradeExecution.Action
			if tradeAction != "LONG" && tradeAction != "SHORT" {
				if isScalpMode {
					log.Printf("[SCALP-SCAN] %s: Action=%s (not LONG/SHORT), SKIP", symbol, tradeAction)
				}
				if mode == GinieModeSwing {
					log.Printf("[SWING-SCAN] %s: Action=%s (not LONG/SHORT), SKIP", symbol, tradeAction)
				}
				if mode == GinieModePosition {
					log.Printf("[POSITION-SCAN] %s: Action=%s (not LONG/SHORT), SKIP", symbol, tradeAction)
				}
				signalLog.Status = "rejected"
				signalLog.RejectionReason = fmt.Sprintf("invalid_action: %s", tradeAction)
				ga.LogSignal(signalLog)
				continue
			}
			// Scalp mode: Log successful entry signal (AC-2.2.2)
			if isScalpMode {
				log.Printf("[SCALP-SCAN] %s: ENTRY SIGNAL - Confidence %.1f%% >= %.1f%%, Direction=%s",
					symbol, decision.ConfidenceScore, effectiveMinConfidence, decision.TradeExecution.Action)
				scalpTrades++
			}

			// Swing mode: Log successful entry signal (AC-2.3.2)
			if mode == GinieModeSwing {
				log.Printf("[SWING-SCAN] %s: ✓ ENTRY SIGNAL - Confidence %.1f%% >= %.1f%%, Direction=%s",
					symbol, decision.ConfidenceScore, effectiveMinConfidence, decision.TradeExecution.Action)
			}

			// Position mode: Log successful entry signal (AC-2.4.1)
			if mode == GinieModePosition {
				log.Printf("[POSITION-SCAN] %s: ENTRY SIGNAL - Confidence %.1f%% >= %.1f%%, Direction=%s",
					symbol, decision.ConfidenceScore, effectiveMinConfidence, decision.TradeExecution.Action)
			}

			ga.logger.Info("Ginie found tradeable signal - attempting trade",
				"symbol", symbol,
				"confidence", decision.ConfidenceScore,
				"min_required", effectiveMinConfidence,
				"action", decision.TradeExecution.Action,
				"mode", decision.SelectedMode)

			// Check mode-specific circuit breaker before executing (Story 2.7 Task 2.7.4)
			canTrade, cbReason := ga.CheckModeCircuitBreaker(mode)
			if !canTrade {
				// Mode circuit breaker is blocking trades
				log.Printf("[MODE-CIRCUIT-BREAKER] %s: Trade for %s BLOCKED - %s", mode, symbol, cbReason)
				signalLog.Status = "rejected"
				signalLog.RejectionReason = fmt.Sprintf("mode_circuit_breaker: %s", cbReason)
				ga.LogSignal(signalLog)
				continue
			}

			// Log as executed (will be attempted)
			signalLog.Status = "executed"
			ga.LogSignal(signalLog)

			// Execute the trade
			ga.executeTrade(decision)

			// Scalp mode: Log trade execution (AC-2.2.2)
			if isScalpMode {
				log.Printf("[SCALP-SCAN] %s: Trade execution initiated", symbol)
			}

			// Swing mode: Log trade execution (AC-2.3.2)
			if mode == GinieModeSwing {
				log.Printf("[SWING-SCAN] %s: ✓ Trade execution initiated", symbol)
			}

			// Position mode: Log trade execution (AC-2.4.1)
			if mode == GinieModePosition {
				log.Printf("[POSITION-SCAN] %s: Trade execution initiated", symbol)
			}
		}
	}

	// Mode-specific scan cycle completion summary (Epic 2 Stories 2.2-2.4)
	switch mode {
	case GinieModeScalp:
		log.Printf("[SCALP-SCAN] Scan complete: %d signals evaluated, %d trade attempts", scalpSignals, scalpTrades)
	case GinieModeSwing:
		log.Printf("[SWING-SCAN] Scan complete: %d symbols scanned, %d signals evaluated", len(symbols), swingSignals)
	case GinieModePosition:
		log.Printf("[POSITION-SCAN] Scan cycle complete: %d symbols scanned, %d signals generated", len(symbols), positionSignals)
	}

	// Update progress to show scan completed (Issue 2B)
	ga.mu.Lock()
	ga.scannedThisCycle = len(symbols)
	ga.mu.Unlock()
}

// getAvailableBalance fetches the actual available balance from Binance
func (ga *GinieAutopilot) getAvailableBalance() (float64, error) {
	accountInfo, err := ga.futuresClient.GetFuturesAccountInfo()
	if err != nil {
		return 0, fmt.Errorf("failed to get account info: %w", err)
	}
	return accountInfo.AvailableBalance, nil
}

// calculateAdaptivePositionSize calculates position size based on available balance and current state
// This is a pragmatic, human-like approach that considers:
// 1. Actual available balance (not fixed config)
// 2. Number of open positions vs max positions
// 3. Confidence level of the trade
// 4. Risk level setting
// 5. Safety margin to avoid over-allocation
// 6. Per-symbol performance category (size multiplier)
// 7. Mode-specific configuration overrides
func (ga *GinieAutopilot) calculateAdaptivePositionSize(symbol string, confidence float64, currentPositionCount int, mode GinieTradingMode) (positionUSD float64, canTrade bool, reason string) {
	// Get mode configuration for mode-specific sizing parameters
	modeConfig := ga.getModeConfig(mode)

	// Get actual available balance from Binance
	availableBalance, err := ga.getAvailableBalance()
	if err != nil {
		ga.logger.Error("Failed to get available balance", "error", err)
		return 0, false, "cannot fetch balance"
	}

	// Safety margin: use mode config or fallback to 0.90 (only use 90% of available balance)
	safetyMargin := 0.90
	if modeConfig != nil && modeConfig.Size != nil && modeConfig.Size.SafetyMargin > 0 {
		safetyMargin = modeConfig.Size.SafetyMargin
	}
	usableBalance := availableBalance * safetyMargin

	// Check minimum balance threshold: use mode config or fallback to $25
	minBalanceRequired := 25.0
	if modeConfig != nil && modeConfig.Size != nil && modeConfig.Size.MinBalanceUSD > 0 {
		minBalanceRequired = modeConfig.Size.MinBalanceUSD
	}
	if usableBalance < minBalanceRequired {
		return 0, false, fmt.Sprintf("insufficient balance: $%.2f (need $%.2f)", usableBalance, minBalanceRequired)
	}

	// Use position count passed from caller (captured while holding lock)
	// Use mode-specific max positions if available, otherwise global config
	maxPositions := ga.config.MaxPositions
	if modeConfig != nil && modeConfig.Size != nil && modeConfig.Size.MaxPositions > 0 {
		maxPositions = modeConfig.Size.MaxPositions
	}
	availableSlots := maxPositions - currentPositionCount

	if availableSlots <= 0 {
		return 0, false, fmt.Sprintf("max positions reached: %d/%d", currentPositionCount, maxPositions)
	}

	// Calculate allocation per potential new position
	// Divide usable balance across available slots
	baseAllocationPerPosition := usableBalance / float64(availableSlots)

	// Get risk multipliers from mode config with fallbacks
	riskMultiplierConservative := 0.6
	riskMultiplierModerate := 0.8
	riskMultiplierAggressive := 1.0
	if modeConfig != nil && modeConfig.Size != nil {
		if modeConfig.Size.RiskMultiplierConservative > 0 {
			riskMultiplierConservative = modeConfig.Size.RiskMultiplierConservative
		}
		if modeConfig.Size.RiskMultiplierModerate > 0 {
			riskMultiplierModerate = modeConfig.Size.RiskMultiplierModerate
		}
		if modeConfig.Size.RiskMultiplierAggressive > 0 {
			riskMultiplierAggressive = modeConfig.Size.RiskMultiplierAggressive
		}
	}

	// Adjust based on risk level using mode-specific multipliers
	riskMultiplier := riskMultiplierAggressive
	switch ga.currentRiskLevel {
	case "conservative":
		riskMultiplier = riskMultiplierConservative
	case "moderate":
		riskMultiplier = riskMultiplierModerate
	case "aggressive":
		riskMultiplier = riskMultiplierAggressive
	}

	// Get confidence multipliers from mode config with fallbacks
	confidenceBase := 0.5
	confidenceScale := 0.7
	if modeConfig != nil && modeConfig.Size != nil {
		if modeConfig.Size.ConfidenceMultiplierBase > 0 {
			confidenceBase = modeConfig.Size.ConfidenceMultiplierBase
		}
		if modeConfig.Size.ConfidenceMultiplierScale > 0 {
			confidenceScale = modeConfig.Size.ConfidenceMultiplierScale
		}
	}

	// Adjust based on confidence (higher confidence = more allocation)
	// Scale: 65% confidence = 0.8x, 80% confidence = 1.0x, 95% confidence = 1.15x
	confidenceMultiplier := confidenceBase + (confidence / 100.0 * confidenceScale)

	// Get per-symbol size multiplier based on performance category
	settingsManager := GetSettingsManager()
	effectiveMaxUSD := settingsManager.GetEffectivePositionSize(symbol, ga.config.MaxUSDPerPosition)
	symbolSettings := settingsManager.GetSymbolSettings(symbol)

	// Calculate final position size
	positionUSD = baseAllocationPerPosition * riskMultiplier * confidenceMultiplier

	// Cap at mode-specific MaxSizeUSD if configured, otherwise use effective max USD
	maxSizeUSD := effectiveMaxUSD
	if modeConfig != nil && modeConfig.Size != nil && modeConfig.Size.MaxSizeUSD > 0 {
		// Use mode-specific max, but still respect per-symbol adjustments (take the lower)
		if modeConfig.Size.MaxSizeUSD < maxSizeUSD {
			maxSizeUSD = modeConfig.Size.MaxSizeUSD
		}
	}
	if positionUSD > maxSizeUSD {
		positionUSD = maxSizeUSD
	}

	// Log per-symbol or mode adjustment if different from global
	if maxSizeUSD != ga.config.MaxUSDPerPosition {
		ga.logger.Debug("Position size cap applied",
			"symbol", symbol,
			"mode", mode,
			"category", symbolSettings.Category,
			"global_max_usd", ga.config.MaxUSDPerPosition,
			"effective_max_usd", maxSizeUSD)
	}

	// Minimum position size check: use mode config or fallback to $10
	minPositionSize := 10.0
	if modeConfig != nil && modeConfig.Size != nil && modeConfig.Size.MinPositionSizeUSD > 0 {
		minPositionSize = modeConfig.Size.MinPositionSizeUSD
	}
	if positionUSD < minPositionSize {
		return 0, false, fmt.Sprintf("calculated position too small: $%.2f (min $%.2f)", positionUSD, minPositionSize)
	}

	ga.logger.Info("Adaptive position sizing",
		"mode", mode,
		"available_balance", fmt.Sprintf("$%.2f", availableBalance),
		"usable_balance", fmt.Sprintf("$%.2f", usableBalance),
		"safety_margin", fmt.Sprintf("%.0f%%", safetyMargin*100),
		"current_positions", currentPositionCount,
		"available_slots", availableSlots,
		"base_allocation", fmt.Sprintf("$%.2f", baseAllocationPerPosition),
		"risk_level", ga.currentRiskLevel,
		"risk_multiplier", fmt.Sprintf("%.2f", riskMultiplier),
		"confidence", fmt.Sprintf("%.1f%%", confidence),
		"confidence_multiplier", fmt.Sprintf("%.2f", confidenceMultiplier),
		"max_size_usd", fmt.Sprintf("$%.2f", maxSizeUSD),
		"final_position_usd", fmt.Sprintf("$%.2f", positionUSD))

	return positionUSD, true, ""
}

// ==================== FUNDING RATE AWARENESS ====================

// checkFundingRate checks if trade should be blocked due to high funding rate near funding time
// Uses mode-specific funding rate configuration if available
// Returns (shouldBlock bool, reason string)
func (ga *GinieAutopilot) checkFundingRate(symbol string, isLong bool, mode GinieTradingMode) (bool, string) {
	fundingRate, err := ga.futuresClient.GetFundingRate(symbol)
	if err != nil || fundingRate == nil {
		return false, "" // Allow if can't check
	}

	// Get mode-specific funding rate config with fallback defaults
	var maxRate float64 = 0.001       // 0.1% threshold (fallback)
	var blockTimeMinutes int = 30      // Block within 30 minutes of funding (fallback)

	modeConfig := ga.getModeConfig(mode)
	if modeConfig != nil && modeConfig.FundingRate != nil {
		// Check if funding rate awareness is disabled for this mode
		if !modeConfig.FundingRate.Enabled {
			return false, "" // Funding rate checks disabled for this mode
		}
		// Use config values if set
		if modeConfig.FundingRate.MaxFundingRate > 0 {
			maxRate = modeConfig.FundingRate.MaxFundingRate
		}
		if modeConfig.FundingRate.BlockTimeMinutes > 0 {
			blockTimeMinutes = modeConfig.FundingRate.BlockTimeMinutes
		}
	}

	// Calculate cost for our direction
	// Positive rate = longs pay shorts, negative rate = shorts pay longs
	fundingCost := fundingRate.FundingRate
	if !isLong {
		fundingCost = -fundingCost // Shorts benefit from positive rates
	}

	// Get time until next funding (every 8 hours: 00:00, 08:00, 16:00 UTC)
	now := time.Now().UTC()
	nextFunding := time.Unix(fundingRate.NextFundingTime/1000, 0)
	timeToFunding := nextFunding.Sub(now)

	// Block if funding costs us money AND rate is high AND near funding time
	blockDuration := time.Duration(blockTimeMinutes) * time.Minute
	if fundingCost > maxRate && timeToFunding > 0 && timeToFunding < blockDuration {
		return true, fmt.Sprintf("High funding %.4f%% costs us in %v", fundingCost*100, timeToFunding.Round(time.Minute))
	}

	return false, ""
}

// shouldExitBeforeFunding checks if we should close position to avoid funding fee
// Uses mode-specific funding rate configuration from the position's mode
// Returns (shouldExit bool, reason string)
func (ga *GinieAutopilot) shouldExitBeforeFunding(pos *GiniePosition) (bool, string) {
	fundingRate, err := ga.futuresClient.GetFundingRate(pos.Symbol)
	if err != nil || fundingRate == nil {
		return false, ""
	}

	// Get mode-specific funding rate config with fallback defaults
	var exitTimeMinutes int = 10           // Only consider exit within 10 minutes (fallback)
	var feeThresholdPercent float64 = 0.3  // Exit if fee > 30% of profit (fallback)
	var extremeFundingRate float64 = 0.003 // 0.3% extreme rate (fallback)

	modeConfig := ga.getModeConfig(pos.Mode)
	if modeConfig != nil && modeConfig.FundingRate != nil {
		// Check if funding rate awareness is disabled for this mode
		if !modeConfig.FundingRate.Enabled {
			return false, "" // Funding rate checks disabled for this mode
		}
		// Use config values if set
		if modeConfig.FundingRate.ExitTimeMinutes > 0 {
			exitTimeMinutes = modeConfig.FundingRate.ExitTimeMinutes
		}
		if modeConfig.FundingRate.FeeThresholdPercent > 0 {
			feeThresholdPercent = modeConfig.FundingRate.FeeThresholdPercent
		}
		if modeConfig.FundingRate.ExtremeFundingRate > 0 {
			extremeFundingRate = modeConfig.FundingRate.ExtremeFundingRate
		}
	}

	now := time.Now().UTC()
	nextFunding := time.Unix(fundingRate.NextFundingTime/1000, 0)
	timeToFunding := nextFunding.Sub(now)

	// Only consider if within exit time window before funding
	exitDuration := time.Duration(exitTimeMinutes) * time.Minute
	if timeToFunding > exitDuration || timeToFunding < 0 {
		return false, ""
	}

	// Calculate funding cost for our position direction
	fundingCost := fundingRate.FundingRate
	if pos.Side == "SHORT" {
		fundingCost = -fundingCost
	}

	// If funding costs us money
	if fundingCost > 0 {
		currentPnL := pos.UnrealizedPnL
		positionValue := pos.EntryPrice * pos.RemainingQty
		fundingFee := positionValue * fundingCost

		// Exit if profitable AND funding fee would eat more than threshold % of profit
		if currentPnL > 0 && fundingFee > currentPnL*feeThresholdPercent {
			return true, fmt.Sprintf("Exit before funding: PnL $%.2f, fee would be $%.4f (%.1f%% of profit)",
				currentPnL, fundingFee, (fundingFee/currentPnL)*100)
		}

		// Exit if funding rate is extreme
		if fundingCost > extremeFundingRate {
			return true, fmt.Sprintf("Exit: extreme funding rate %.4f%% in %v", fundingCost*100, timeToFunding.Round(time.Minute))
		}
	}

	return false, ""
}

// adjustSizeForFunding reduces position size when funding rate is costly
// Uses mode-specific funding rate configuration if available
func (ga *GinieAutopilot) adjustSizeForFunding(symbol string, baseSize float64, isLong bool, mode GinieTradingMode) float64 {
	fundingRate, err := ga.futuresClient.GetFundingRate(symbol)
	if err != nil || fundingRate == nil {
		return baseSize
	}

	// Get mode-specific funding rate config with fallback defaults
	var maxFundingRate float64 = 0.001       // 0.1% elevated threshold (fallback)
	var highRateReduction float64 = 0.5      // 50% reduction for high rates (fallback)
	var elevatedRateReduction float64 = 0.75 // 75% (25% reduction) for elevated rates (fallback)

	modeConfig := ga.getModeConfig(mode)
	if modeConfig != nil && modeConfig.FundingRate != nil {
		// Check if funding rate awareness is disabled for this mode
		if !modeConfig.FundingRate.Enabled {
			return baseSize // Funding rate adjustments disabled for this mode
		}
		// Use config values if set
		if modeConfig.FundingRate.MaxFundingRate > 0 {
			maxFundingRate = modeConfig.FundingRate.MaxFundingRate
		}
		if modeConfig.FundingRate.HighRateReduction > 0 {
			highRateReduction = modeConfig.FundingRate.HighRateReduction
		}
		if modeConfig.FundingRate.ElevatedRateReduction > 0 {
			elevatedRateReduction = modeConfig.FundingRate.ElevatedRateReduction
		}
	}

	fundingCost := fundingRate.FundingRate
	if !isLong {
		fundingCost = -fundingCost
	}

	// Only adjust if funding costs us money
	if fundingCost > 0 {
		// High rate threshold = 2x the max funding rate (e.g., 0.2% if max is 0.1%)
		highRateThreshold := maxFundingRate * 2
		if fundingCost > highRateThreshold {
			ga.logger.Info("Funding rate high - reducing position",
				"symbol", symbol, "mode", mode, "funding_rate", fundingCost*100,
				"reduction", fmt.Sprintf("%.0f%%", (1-highRateReduction)*100), "original_size", baseSize)
			return baseSize * highRateReduction
		}
		// Elevated rate = at or above max funding rate
		if fundingCost > maxFundingRate {
			ga.logger.Info("Funding rate elevated - reducing position",
				"symbol", symbol, "mode", mode, "funding_rate", fundingCost*100,
				"reduction", fmt.Sprintf("%.0f%%", (1-elevatedRateReduction)*100), "original_size", baseSize)
			return baseSize * elevatedRateReduction
		}
	}

	return baseSize
}

// ==================== END FUNDING RATE AWARENESS ====================

// ==================== ORDER FILL VERIFICATION ====================

// verifyOrderFill verifies that a market order was filled and returns fill details
// For commercial trading, we must verify orders are executed, not just accepted
func (ga *GinieAutopilot) verifyOrderFill(order *binance.FuturesOrderResponse, expectedQty float64) (actualPrice float64, actualQty float64, err error) {
	if order == nil {
		return 0, 0, fmt.Errorf("nil order response")
	}

	// Check order status
	status := binance.FuturesOrderStatus(order.Status)

	// Market orders should fill immediately
	if status == binance.FuturesOrderStatusFilled {
		// Verify executed quantity matches (allow 0.1% slippage for rounding)
		if order.ExecutedQty < expectedQty*0.999 {
			ga.logger.Warn("Order partially filled",
				"expected_qty", expectedQty,
				"executed_qty", order.ExecutedQty,
				"order_id", order.OrderId)
		}
		return order.AvgPrice, order.ExecutedQty, nil
	}

	// If not filled, we need to poll for status (market orders should not get here)
	if status == binance.FuturesOrderStatusNew || status == binance.FuturesOrderStatusPartiallyFilled {
		// Wait and poll for fill (up to 5 seconds for market order)
		for attempt := 0; attempt < 5; attempt++ {
			time.Sleep(1 * time.Second)

			// Query order status
			orders, err := ga.futuresClient.GetOpenOrders(order.Symbol)
			if err != nil {
				ga.logger.Warn("Failed to query order status", "order_id", order.OrderId, "error", err)
				continue
			}

			// Check if order is still in open orders (if not, it's filled)
			found := false
			for _, o := range orders {
				if o.OrderId == order.OrderId {
					found = true
					if binance.FuturesOrderStatus(o.Status) == binance.FuturesOrderStatusFilled {
						return o.AvgPrice, o.ExecutedQty, nil
					}
					break
				}
			}

			// Order not in open orders = filled
			if !found {
				// Get all orders to find the filled order
				allOrders, err := ga.futuresClient.GetAllOrders(order.Symbol, 10)
				if err == nil {
					for _, o := range allOrders {
						if o.OrderId == order.OrderId && binance.FuturesOrderStatus(o.Status) == binance.FuturesOrderStatusFilled {
							return o.AvgPrice, o.ExecutedQty, nil
						}
					}
				}
				// If we can't find it but it's not open, assume filled at average price
				if order.AvgPrice > 0 {
					return order.AvgPrice, order.ExecutedQty, nil
				}
			}
		}

		// After 5 seconds, if market order still not filled, something is wrong
		return 0, 0, fmt.Errorf("market order not filled after 5s, status: %s", status)
	}

	// Order was rejected or cancelled
	if status == binance.FuturesOrderStatusCanceled || status == binance.FuturesOrderStatusExpired {
		return 0, 0, fmt.Errorf("order rejected, status: %s", status)
	}

	return order.AvgPrice, order.ExecutedQty, nil
}

// ==================== END ORDER FILL VERIFICATION ====================

// executeTrade executes a trade based on Ginie decision
func (ga *GinieAutopilot) executeTrade(decision *GinieDecisionReport) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	symbol := decision.Symbol

	// Check if coin is blocked due to big losses
	if blocked, reason := ga.isCoinBlocked(symbol); blocked {
		ga.logger.Warn("Ginie skipping trade - coin is blocked",
			"symbol", symbol,
			"reason", reason)
		return
	}

	// CRITICAL: Skip trades where action is WAIT or CLOSE - these are not entry signals
	action := decision.TradeExecution.Action
	if action != "LONG" && action != "SHORT" {
		ga.logger.Warn("Ginie skipping trade - action is not LONG or SHORT",
			"symbol", symbol,
			"action", action,
			"confidence", decision.ConfidenceScore)
		return
	}

	// Check funding rate before entry - avoid high fees near funding time
	isLong := action == "LONG"
	selectedMode := decision.SelectedMode
	if blocked, reason := ga.checkFundingRate(symbol, isLong, selectedMode); blocked {
		ga.logger.Warn("Ginie skipping trade - funding rate concern",
			"symbol", symbol,
			"mode", selectedMode,
			"reason", reason,
			"side", decision.TradeExecution.Action)
		return
	}

	// Double-check we don't already have a position
	if _, exists := ga.positions[symbol]; exists {
		return
	}

	// Capture position count while holding lock for adaptive sizing
	currentPositionCount := len(ga.positions)

	// Use adaptive position sizing based on available balance (human-like approach)
	// Need to unlock temporarily for API call to get balance
	ga.mu.Unlock()
	positionUSD, canTrade, reason := ga.calculateAdaptivePositionSize(symbol, decision.ConfidenceScore, currentPositionCount, selectedMode)
	ga.mu.Lock()

	// CRITICAL: Re-check position doesn't exist after re-acquiring lock
	// Another goroutine could have opened a position for this symbol while we were unlocked
	if _, exists := ga.positions[symbol]; exists {
		ga.logger.Warn("Ginie race condition avoided - position created while sizing",
			"symbol", symbol)
		return
	}

	if !canTrade {
		ga.logger.Warn("Ginie cannot trade - adaptive sizing rejected",
			"symbol", symbol,
			"reason", reason,
			"confidence_score", decision.ConfidenceScore)
		return
	}

	// Adjust position size based on funding rate (reduce if funding costs us money)
	positionUSD = ga.adjustSizeForFunding(symbol, positionUSD, isLong, selectedMode)

	// Get current price
	price, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		ga.logger.Error("Failed to get price for trade", "symbol", symbol, "error", err)
		return
	}

	// Use leverage from decision or default
	leverage := decision.TradeExecution.Leverage
	if leverage == 0 {
		leverage = ga.config.DefaultLeverage
	}

	// Calculate quantity based on adaptive position size
	quantity := (positionUSD * float64(leverage)) / price
	quantity = roundQuantity(symbol, quantity)

	if quantity <= 0 {
		ga.logger.Warn("Ginie calculated zero quantity", "symbol", symbol, "usd", positionUSD)
		return
	}

	// Determine side
	side := "BUY"
	positionSide := binance.PositionSideLong
	if decision.TradeExecution.Action == "SHORT" {
		side = "SELL"
		positionSide = binance.PositionSideShort
	}

	// Check actual Binance position mode to avoid API error -4061
	effectivePositionSide := ga.getEffectivePositionSide(positionSide)

	// Build TP levels with prices
	takeProfits := make([]GinieTakeProfitLevel, len(decision.TradeExecution.TakeProfits))
	for i, tp := range decision.TradeExecution.TakeProfits {
		takeProfits[i] = GinieTakeProfitLevel{
			Level:   tp.Level,
			Price:   tp.Price,
			Percent: ga.getTPPercent(i + 1), // Use our configured percentages
			GainPct: tp.GainPct,
			Status:  "pending",
		}
	}

	// Ensure we have 4 TP levels
	if len(takeProfits) < 4 {
		// Generate default TPs based on mode
		takeProfits = ga.generateDefaultTPs(symbol, price, decision.SelectedMode, decision.TradeExecution.Action == "LONG")
	}

	ga.logger.Info("Ginie executing trade",
		"symbol", symbol,
		"side", decision.TradeExecution.Action,
		"mode", decision.SelectedMode,
		"quantity", quantity,
		"leverage", leverage,
		"confidence", decision.ConfidenceScore,
		"dry_run", ga.config.DryRun)

	// Variables for actual fill details
	actualPrice := price
	actualQty := quantity

	if !ga.config.DryRun {
		// Set leverage first
		_, err = ga.futuresClient.SetLeverage(symbol, leverage)
		if err != nil {
			ga.logger.Error("Failed to set leverage", "symbol", symbol, "error", err.Error())
			return
		}

		// Place market order
		orderParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         side,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     quantity,
		}

		order, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
		if err != nil {
			ga.logger.Error("Ginie trade execution failed", "symbol", symbol, "error", err.Error())
			return
		}

		// CRITICAL: Verify order fill - commercial grade trading must confirm execution
		fillPrice, fillQty, fillErr := ga.verifyOrderFill(order, quantity)
		if fillErr != nil {
			ga.logger.Error("Ginie order fill verification failed",
				"symbol", symbol,
				"order_id", order.OrderId,
				"error", fillErr.Error())
			// Don't create position if we can't verify fill
			return
		}

		// Use actual fill values for position tracking
		actualPrice = fillPrice
		actualQty = fillQty

		// CRITICAL FIX: Recalculate TPs and SL based on actual fill price
		// The decision's TP/SL were calculated with pre-execution estimated price
		// which can differ significantly from actual fill price
		if actualPrice != price && actualPrice > 0 {
			isLong := decision.TradeExecution.Action == "LONG"
			sideForRounding := "LONG"
			if !isLong {
				sideForRounding = "SHORT"
			}

			// Recalculate each TP level with actual fill price (with directional rounding)
			for i := range takeProfits {
				gainPct := takeProfits[i].GainPct
				var rawTP float64
				if isLong {
					rawTP = actualPrice * (1 + gainPct/100)
				} else {
					rawTP = actualPrice * (1 - gainPct/100)
				}
				// Apply Binance directional precision rounding for TP orders
				takeProfits[i].Price = roundPriceForTP(symbol, rawTP, sideForRounding)
			}

			// Recalculate SL with actual fill price (with directional rounding)
			slPct := decision.TradeExecution.StopLossPct
			if slPct > 0 {
				var rawSL float64
				if isLong {
					rawSL = actualPrice * (1 - slPct/100)
				} else {
					rawSL = actualPrice * (1 + slPct/100)
				}
				// Apply Binance directional precision rounding for SL orders
				decision.TradeExecution.StopLoss = roundPriceForSL(symbol, rawSL, sideForRounding)
			}

			ga.logger.Info("Recalculated TP/SL with actual fill price",
				"symbol", symbol,
				"estimated_price", price,
				"actual_price", actualPrice,
				"price_diff_pct", ((actualPrice-price)/price)*100,
				"new_sl", decision.TradeExecution.StopLoss)
		}

		ga.logger.Info("Ginie trade executed and verified",
			"symbol", symbol,
			"order_id", order.OrderId,
			"side", side,
			"requested_qty", quantity,
			"filled_qty", actualQty,
			"expected_price", price,
			"fill_price", actualPrice)
	}

	// Create position record with ACTUAL fill price and quantity
	// Get trailing stop settings from Mode Config
	trailingEnabled := ga.isTrailingEnabled(decision.SelectedMode)
	trailingPercent := 0.0
	trailingActivation := 0.0
	if trailingEnabled {
		trailingPercent = ga.getTrailingPercent(decision.SelectedMode)
		trailingActivation = ga.getTrailingActivation(decision.SelectedMode)
	}

	position := &GiniePosition{
		Symbol:               symbol,
		Side:                 decision.TradeExecution.Action,
		Mode:                 decision.SelectedMode,
		EntryPrice:           actualPrice,
		OriginalQty:          actualQty,
		RemainingQty:         actualQty,
		Leverage:             leverage,
		EntryTime:            time.Now(),
		TakeProfits:          takeProfits,
		CurrentTPLevel:       0,
		StopLoss:             decision.TradeExecution.StopLoss,
		OriginalSL:           decision.TradeExecution.StopLoss,
		MovedToBreakeven:     false,
		TrailingActive:       false,
		HighestPrice:         actualPrice,
		LowestPrice:          actualPrice,
		TrailingPercent:      trailingPercent,
		TrailingActivationPct: trailingActivation, // Now properly initialized from Mode Config
		DecisionReport:       decision,
		Source:               "ai", // AI-based trade
		Protection:           NewProtectionStatus(), // Initialize bulletproof protection tracking
	}

	ga.positions[symbol] = position
	ga.dailyTrades++
	ga.totalTrades++

	// Create initial futures trade record in database for lifecycle tracking
	if ga.repo != nil {
		trade := &database.FuturesTrade{
			Symbol:       symbol,
			PositionSide: decision.TradeExecution.Action,
			Side:         decision.TradeExecution.Action,
			EntryPrice:   actualPrice,
			Quantity:     actualQty,
			Leverage:     leverage,
			MarginType:   "CROSSED",
			Status:       "OPEN",
			EntryTime:    time.Now(),
			TradeSource:  "ginie",
		}
		if err := ga.repo.CreateFuturesTrade(context.Background(), trade); err != nil {
			ga.logger.Warn("Failed to create futures trade record", "error", err, "symbol", symbol)
		} else {
			position.FuturesTradeID = trade.ID
			ga.logger.Debug("Futures trade record created", "symbol", symbol, "trade_id", trade.ID)

			// Log position opened event to lifecycle
			if ga.eventLogger != nil {
				conditionsMet := make(map[string]interface{})
				for _, sig := range decision.SignalAnalysis.PrimarySignals {
					if sig.Met {
						conditionsMet[sig.Name] = sig.Value
					}
				}
				go ga.eventLogger.LogPositionOpened(
					context.Background(),
					trade.ID,
					symbol,
					decision.TradeExecution.Action,
					string(decision.SelectedMode),
					actualPrice,
					actualQty,
					leverage,
					decision.ConfidenceScore,
					conditionsMet,
				)
			}
		}
	}

	// Place SL/TP orders on Binance (if not dry run)
	if !ga.config.DryRun {
		position.Protection.SetState(StatePlacingSL)
		ga.placeSLTPOrders(position)

		// CRITICAL: Verify protection was established
		// Give orders time to be registered on Binance
		time.Sleep(500 * time.Millisecond)
		ga.verifyPositionProtection(position)

		if !position.Protection.SLVerified {
			log.Printf("[PROTECTION] %s: WARNING - SL not verified after initial placement, will be healed by guardian", symbol)
		}
	} else {
		// In dry run, mark as protected
		position.Protection.SetState(StateFullyProtected)
	}

	// Build signal names for summary
	signalNames := make([]string, 0)
	for _, sig := range decision.SignalAnalysis.PrimarySignals {
		if sig.Met {
			signalNames = append(signalNames, sig.Name)
		}
	}

	// Build TP prices array from recalculated takeProfits (not stale decision values)
	tpPrices := make([]float64, len(takeProfits))
	for i, tp := range takeProfits {
		tpPrices[i] = tp.Price
	}

	// Record trade with full signal info for study (using actual fill values)
	ga.recordTrade(GinieTradeResult{
		Symbol:     symbol,
		Action:     "open",
		Side:       decision.TradeExecution.Action,
		Quantity:   actualQty,
		Price:      actualPrice,
		Reason:     fmt.Sprintf("Ginie %s signal (%.1f%% confidence)", decision.SelectedMode, decision.ConfidenceScore),
		Timestamp:  time.Now(),
		Mode:       decision.SelectedMode,
		Confidence: decision.ConfidenceScore,
		MarketConditions: &GinieMarketSnapshot{
			Trend:      decision.MarketConditions.Trend,
			ADX:        decision.MarketConditions.ADX,
			Volatility: decision.MarketConditions.Volatility,
			ATRPercent: decision.MarketConditions.ATR,
			Volume:     decision.MarketConditions.Volume,
			BTCCorr:    decision.MarketConditions.BTCCorr,
		},
		SignalSummary: &GinieSignalSummary{
			Direction:       decision.SignalAnalysis.Direction,
			Strength:        decision.SignalAnalysis.SignalStrength,
			StrengthScore:   decision.SignalAnalysis.StrengthScore,
			PrimaryMet:      decision.SignalAnalysis.PrimaryMet,
			PrimaryRequired: decision.SignalAnalysis.PrimaryRequired,
			SignalNames:     signalNames,
		},
		EntryParams: &GinieEntryParams{
			EntryPrice:  actualPrice, // Use actual fill price, not estimated
			StopLoss:    decision.TradeExecution.StopLoss,
			StopLossPct: decision.TradeExecution.StopLossPct,
			TakeProfits: tpPrices,
			Leverage:    leverage,
			RiskReward:  decision.TradeExecution.RiskReward,
		},
	})
}

// runPositionMonitor monitors all positions for TP/SL hits and trailing
func (ga *GinieAutopilot) runPositionMonitor() {
	defer ga.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			ga.logger.Error("PANIC in Ginie position monitor - restarting", "panic", r)
			log.Printf("[GINIE-PANIC] Position monitor panic: %v", r)
			// Restart the monitor after a brief delay
			time.Sleep(5 * time.Second)
			ga.wg.Add(1)
			go ga.runPositionMonitor()
		}
	}()

	log.Printf("[GINIE-MONITOR] Position monitor goroutine started")
	ga.logger.Info("Ginie position monitor started - will check positions every 5 seconds")

	// Check positions every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	scanCount := 0
	for {
		select {
		case <-ga.stopChan:
			ga.logger.Info("Ginie position monitor stopping")
			return
		case <-ticker.C:
			scanCount++
			// Log every 12th scan (every minute) to show it's alive
			if scanCount%12 == 1 {
				ga.mu.RLock()
				posCount := len(ga.positions)
				ga.mu.RUnlock()
				ga.logger.Info("Ginie position monitor active",
					"scan_count", scanCount,
					"positions", posCount)
			}
			ga.monitorAllPositions()

			// Reconcile positions with Binance every 30 seconds (6 scans * 5 seconds)
			// This catches positions closed manually or modified externally
			if scanCount%6 == 0 {
				go ga.reconcilePositions()
			}

			// Clean up orphan orders every 60 seconds (12 scans * 5 seconds)
			if scanCount%12 == 0 {
				go ga.cleanupOrphanAlgoOrders()
			}
		}
	}
}

// monitorAllPositions checks all positions for TP/SL/trailing triggers
func (ga *GinieAutopilot) monitorAllPositions() {
	// PHASE 1: Copy position symbols while holding lock briefly
	ga.mu.RLock()
	posCount := len(ga.positions)
	if posCount == 0 {
		ga.mu.RUnlock()
		return
	}

	// Copy symbols and positions for processing outside lock
	type positionSnapshot struct {
		symbol string
		pos    *GiniePosition
	}
	snapshots := make([]positionSnapshot, 0, posCount)
	for sym, pos := range ga.positions {
		snapshots = append(snapshots, positionSnapshot{symbol: sym, pos: pos})
	}
	ga.mu.RUnlock()

	log.Printf("[GINIE-MONITOR] Checking %d positions for trailing/TP/SL", posCount)
	var posSymbols []string
	for _, snap := range snapshots {
		posSymbols = append(posSymbols, snap.symbol)
	}
	log.Printf("[GINIE-MONITOR-DEBUG] Positions in monitoring: %v", posSymbols)

	// PHASE 2: Fetch prices OUTSIDE the lock (network calls)
	prices := make(map[string]float64)
	for _, snap := range snapshots {
		currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(snap.symbol)
		if err != nil {
			continue
		}
		prices[snap.symbol] = currentPrice
	}

	// PHASE 3: Process positions with lock for state updates
	for _, snap := range snapshots {
		symbol := snap.symbol
		currentPrice, ok := prices[symbol]
		if !ok {
			continue
		}

		// Acquire lock for this position's state update
		ga.mu.Lock()
		pos, exists := ga.positions[symbol]
		if !exists {
			ga.mu.Unlock()
			continue
		}

		// Update high/low tracking
		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
		}
		if currentPrice < pos.LowestPrice {
			pos.LowestPrice = currentPrice
		}

		// Calculate current PnL
		var pnlPercent float64
		if pos.Side == "LONG" {
			pnlPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
			pos.UnrealizedPnL = (currentPrice - pos.EntryPrice) * pos.RemainingQty
		} else {
			pnlPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
			pos.UnrealizedPnL = (pos.EntryPrice - currentPrice) * pos.RemainingQty
		}

		// === FUNDING RATE EARLY EXIT ===
		// Check if we should exit before funding payment to save fees
		if shouldExit, reason := ga.shouldExitBeforeFunding(pos); shouldExit {
			ga.logger.Info("Exiting position before funding",
				"symbol", symbol,
				"pnl", pos.UnrealizedPnL,
				"pnl_percent", pnlPercent,
				"reason", reason)
			ga.mu.Unlock()
			ga.closePosition(symbol, pos, currentPrice, "funding_rate_exit", pos.CurrentTPLevel)
			continue
		}

		// === PROACTIVE PROFIT PROTECTION (NEW - fixes BCHUSDT-style losses) ===

		// Log position status for debugging
		if pnlPercent > 0.3 {
			log.Printf("[GINIE-MONITOR] %s: PnL=%.2f%%, TrailingActive=%v, MovedToBreakeven=%v, TPLevel=%d, SL=%.4f",
				symbol, pnlPercent, pos.TrailingActive, pos.MovedToBreakeven, pos.CurrentTPLevel, pos.StopLoss)
		}

		// 1. Proactive breakeven: Move SL to entry when profit >= threshold (before TP1)
		if !pos.MovedToBreakeven && pnlPercent >= ga.config.ProactiveBreakevenPercent && pos.CurrentTPLevel == 0 {
			log.Printf("[GINIE-MONITOR] %s: Triggering proactive breakeven at %.2f%% profit", symbol, pnlPercent)
			ga.logger.Info("Proactive breakeven triggered",
				"symbol", pos.Symbol,
				"pnl_percent", pnlPercent,
				"threshold", ga.config.ProactiveBreakevenPercent)
			ga.moveToBreakeven(pos)
			// FIX: Release lock BEFORE network call to prevent blocking GetStatus API
			ga.mu.Unlock()
			ga.updateBinanceSLOrder(pos)
			// Re-acquire lock and check if position still exists
			ga.mu.Lock()
			pos, exists = ga.positions[symbol]
			if !exists {
				ga.mu.Unlock()
				continue
			}
		}

		// 2. Trailing SL: Activate ONLY after TP1 hit AND SL moved to breakeven (for swing/position)
		// Ultra-fast and Scalp modes: NO trailing (disabled in settings)
		if !pos.TrailingActive {
			settingsManager := GetSettingsManager()
			settings := settingsManager.GetCurrentSettings()

			// Check if trailing is enabled for this mode (read from ModeConfigs)
			modeToConfigKey := map[string]string{
				string(GinieModeUltraFast): "ultra_fast",
				string(GinieModeScalp):     "scalp",
				string(GinieModeSwing):     "swing",
				string(GinieModePosition):  "position",
			}
			trailingEnabled := false
			if modeKey, ok := modeToConfigKey[string(pos.Mode)]; ok {
				if modeConfig := settings.ModeConfigs[modeKey]; modeConfig != nil {
					if modeConfig.SLTP != nil {
						trailingEnabled = modeConfig.SLTP.TrailingStopEnabled
					}
				}
			}

			if trailingEnabled {
				// Trailing activation conditions (multiple paths):
				// 1. TP1 hit AND breakeven moved (conservative)
				// 2. OR profit threshold reached (if TrailingActivationPct > 0)
				canActivate := false
				activationReason := ""

				if pos.Mode == GinieModeSwing || pos.Mode == GinieModePosition {
					// Allow activation via multiple conditions:
					// 1. TP1 hit AND breakeven moved (conservative - protects after partial TP)
					// 2. OR profit threshold reached (respects user's TrailingActivationPct setting)
					if pos.CurrentTPLevel >= 1 && pos.MovedToBreakeven {
						canActivate = true
						activationReason = "after_tp1_and_breakeven"
					} else if pos.TrailingActivationPct > 0 && pnlPercent >= pos.TrailingActivationPct {
						// FIX: Allow profit-threshold activation even before TP1
						// This prevents scenarios where price runs up significantly but trailing never activates
						canActivate = true
						activationReason = "profit_threshold"
						log.Printf("[GINIE-TRAILING] %s: Activating via profit threshold (%.2f%% >= %.2f%%)",
							symbol, pnlPercent, pos.TrailingActivationPct)
					}
				} else {
					// For other modes (ultra-fast/scalp if enabled), use profit threshold
					if pos.TrailingActivationPct > 0 && pnlPercent >= pos.TrailingActivationPct {
						canActivate = true
						activationReason = "profit_threshold"
					}
				}

				if canActivate {
					pos.TrailingActive = true
					ga.logger.Info("Trailing stop activated",
						"symbol", pos.Symbol,
						"mode", pos.Mode,
						"reason", activationReason,
						"tp_level", pos.CurrentTPLevel,
						"at_breakeven", pos.MovedToBreakeven,
						"pnl_percent", pnlPercent)

					// Log trailing activation to trade lifecycle
					if ga.eventLogger != nil && pos.FuturesTradeID > 0 {
						go ga.eventLogger.LogTrailingActivated(
							context.Background(),
							pos.FuturesTradeID,
							pos.Symbol,
							string(pos.Mode),
							activationReason,
							currentPrice,
							pnlPercent,
							pos.CurrentTPLevel,
						)
					}
				}
			}
		}

		// 3. Trail SL upward: Update SL as price moves favorably
		// Use configured trailing percent (per-mode), or fall back to global config
		trailingPercent := pos.TrailingPercent
		if trailingPercent == 0 {
			trailingPercent = ga.config.TrailingStepPercent
		}

		if pos.TrailingActive && trailingPercent > 0 {
			var newTrailingSL float64
			if pos.Side == "LONG" {
				// For longs: trail from highest price
				newTrailingSL = pos.HighestPrice * (1 - trailingPercent/100)
			} else {
				// For shorts: trail from lowest price
				newTrailingSL = pos.LowestPrice * (1 + trailingPercent/100)
			}

			// Only move SL in profitable direction (never lower for longs, never higher for shorts)
			slImproved := false
			var trailingOldSL float64
			var trailingImprovement float64

			if pos.Side == "LONG" && newTrailingSL > pos.StopLoss {
				slImprovement := (newTrailingSL - pos.StopLoss) / pos.EntryPrice * 100
				if slImprovement >= ga.config.TrailingSLUpdateThreshold {
					trailingOldSL = pos.StopLoss
					trailingImprovement = slImprovement
					pos.StopLoss = newTrailingSL
					slImproved = true
					ga.logger.Info("Trailing SL moved up (LONG)",
						"symbol", pos.Symbol,
						"old_sl", trailingOldSL,
						"new_sl", newTrailingSL,
						"highest_price", pos.HighestPrice,
						"improvement_pct", slImprovement)
				}
			} else if pos.Side == "SHORT" && newTrailingSL < pos.StopLoss {
				slImprovement := (pos.StopLoss - newTrailingSL) / pos.EntryPrice * 100
				if slImprovement >= ga.config.TrailingSLUpdateThreshold {
					trailingOldSL = pos.StopLoss
					trailingImprovement = slImprovement
					pos.StopLoss = newTrailingSL
					slImproved = true
					ga.logger.Info("Trailing SL moved down (SHORT)",
						"symbol", pos.Symbol,
						"old_sl", trailingOldSL,
						"new_sl", newTrailingSL,
						"lowest_price", pos.LowestPrice,
						"improvement_pct", slImprovement)
				}
			}

			// Update Binance order if SL improved significantly
			if slImproved {
				// Capture values needed for event logging before releasing lock
				eventLogger := ga.eventLogger
				futuresTradeID := pos.FuturesTradeID
				posSymbol := pos.Symbol
				posSide := pos.Side
				posStopLoss := pos.StopLoss
				highWaterMark := pos.HighestPrice
				if pos.Side == "SHORT" {
					highWaterMark = pos.LowestPrice
				}

				// FIX: Release lock BEFORE network call to prevent blocking GetStatus API
				ga.mu.Unlock()
				ga.updateBinanceSLOrder(pos)

				// Log trailing update to trade lifecycle (uses captured values, no lock needed)
				if eventLogger != nil && futuresTradeID > 0 {
					go eventLogger.LogTrailingUpdated(
						context.Background(),
						futuresTradeID,
						posSymbol,
						posSide,
						trailingOldSL,
						posStopLoss,
						highWaterMark,
						trailingImprovement,
					)
				}

				// Re-acquire lock and check if position still exists
				ga.mu.Lock()
				pos, exists = ga.positions[symbol]
				if !exists {
					ga.mu.Unlock()
					continue
				}
			}
		}
		// === END PROACTIVE PROFIT PROTECTION ===

		// === EARLY PROFIT BOOKING (ROI-BASED) ===
		// Close position if ROI after fees exceeds mode-specific threshold
		log.Printf("[MONITOR-EARLY-PROFIT] %s: About to check early profit booking\n", symbol)
		shouldBook, roiPercent, modeStr := ga.shouldBookEarlyProfit(pos, currentPrice)
		if shouldBook {
			ga.logger.Info("Booking profit early based on ROI threshold",
				"symbol", symbol,
				"roi_percent", roiPercent,
				"mode", modeStr,
				"threshold", roiPercent,
				"entry_price", pos.EntryPrice,
				"current_price", currentPrice)
			ga.mu.Unlock()
			ga.closePosition(symbol, pos, currentPrice, "early_profit_booking", pos.CurrentTPLevel)
			continue
		}
		// === END EARLY PROFIT BOOKING ===

		// Check Stop Loss
		if ga.checkStopLoss(pos, currentPrice) {
			ga.mu.Unlock()
			ga.closePosition(symbol, pos, currentPrice, "stop_loss", 0)
			continue
		}

		// Check Take Profit levels (process one at a time)
		// FIX: Release lock BEFORE checkTakeProfits since it makes network calls
		// (executePartialClose, updateBinanceSLOrder, placeNextTPOrder)
		ga.mu.Unlock()
		tpHit := ga.checkTakeProfits(pos, currentPrice, pnlPercent)
		if tpHit > 0 && tpHit <= len(pos.TakeProfits) {
			// Partial close for TP1-3, handled by checkTakeProfits
			// Lock already released, just continue
			continue
		}
		// Re-acquire lock for remaining checks
		ga.mu.Lock()
		pos, exists = ga.positions[symbol]
		if !exists {
			ga.mu.Unlock()
			continue
		}

		// Check trailing stop (for TP4 / final portion) - now also triggers earlier if trailing active
		if pos.TrailingActive {
			if ga.checkTrailingStop(pos, currentPrice) {
				reason := "trailing_stop"
				if pos.CurrentTPLevel >= 3 {
					reason = "trailing_stop_tp4"
				}
				ga.mu.Unlock()
				ga.closePosition(symbol, pos, currentPrice, reason, pos.CurrentTPLevel)
				continue
			}
		}

		// Release lock at end of this position's processing
		ga.mu.Unlock()
	}
}

// checkStopLoss checks if stop loss is hit
// Uses tolerance-based comparison to avoid floating point precision issues
func (ga *GinieAutopilot) checkStopLoss(pos *GiniePosition, currentPrice float64) bool {
	if pos.StopLoss <= 0 {
		return false
	}

	if pos.Side == "LONG" {
		return priceLessOrEqual(pos.Symbol, currentPrice, pos.StopLoss)
	}
	return priceGreaterOrEqual(pos.Symbol, currentPrice, pos.StopLoss)
}

// checkTakeProfits checks and executes take profit levels
// Uses tolerance-based comparison to avoid floating point precision issues
// shouldBookEarlyProfit checks if position should be closed early based on ROI threshold
// Priority order for threshold selection:
//   1. Position.CustomROIPercent (temporary, per-position override)
//   2. SymbolSettings.CustomROIPercent (persistent, per-symbol override)
//   3. Mode-based thresholds (SCALP=5%, SWING=8%, POSITION=10%, ULTRAFAST=3%)
// Returns (shouldBook, currentROI, source) where source indicates which threshold was used
func (ga *GinieAutopilot) shouldBookEarlyProfit(pos *GiniePosition, currentPrice float64) (bool, float64, string) {
	// Debug: Log early profit booking check start
	fmt.Printf("[EARLY-PROFIT-DEBUG] Checking %s | entry=%.8f current=%.8f | enabled=%v tpLevel=%d\n",
		pos.Symbol, pos.EntryPrice, currentPrice, ga.config.EarlyProfitBookingEnabled, pos.CurrentTPLevel)

	if !ga.config.EarlyProfitBookingEnabled || pos.CurrentTPLevel > 0 {
		if !ga.config.EarlyProfitBookingEnabled {
			fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Early profit booking DISABLED in config\n", pos.Symbol)
		}
		if pos.CurrentTPLevel > 0 {
			fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Skipping early profit - TP level already hit (CurrentTPLevel=%d)\n",
				pos.Symbol, pos.CurrentTPLevel)
		}
		return false, 0, ""
	}

	// Calculate ROI after fees (including leverage effect)
	roiPercent := calculateROIAfterFees(pos.EntryPrice, currentPrice, pos.RemainingQty, pos.Side, pos.Leverage)

	fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Calculated ROI after fees = %.4f%%\n", pos.Symbol, roiPercent)

	// Only book if profitable after fees
	if roiPercent <= 0 {
		fmt.Printf("[EARLY-PROFIT-DEBUG] %s: ROI not profitable (%.4f%%), skipping\n", pos.Symbol, roiPercent)
		return false, 0, ""
	}

	// Determine threshold: Custom position ROI > Custom symbol ROI > Mode-based threshold
	var threshold float64
	var source string

	// 1. Check position-level custom ROI (highest priority)
	if pos.CustomROIPercent != nil && *pos.CustomROIPercent > 0 {
		threshold = *pos.CustomROIPercent
		source = "position_custom"
		fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Using POSITION-LEVEL custom ROI = %.4f%%\n", pos.Symbol, threshold)
	} else {
		if pos.CustomROIPercent == nil {
			fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Position custom ROI is NIL (not set)\n", pos.Symbol)
		} else {
			fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Position custom ROI is <= 0 (%.4f), checking symbol settings\n",
				pos.Symbol, *pos.CustomROIPercent)
		}

		// 2. Check symbol-level custom ROI from PER-USER database (second priority)
		var userSymbolROI float64
		if ga.userID != "" && ga.repo != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			roi, err := ga.repo.GetUserSymbolROI(ctx, ga.userID, pos.Symbol)
			cancel()
			if err == nil && roi > 0 {
				userSymbolROI = roi
				fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Found PER-USER custom ROI = %.4f%% from database\n",
					pos.Symbol, userSymbolROI)
			}
		}

		if userSymbolROI > 0 {
			threshold = userSymbolROI
			source = "user_symbol_custom"
			fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Using USER-SPECIFIC custom ROI = %.4f%% from database\n",
				pos.Symbol, threshold)
		} else {
			// Fallback: Check shared symbol settings (legacy mode)
			settingsManager := GetSettingsManager()
			symbolSettings := settingsManager.GetSymbolSettings(pos.Symbol)
			if symbolSettings != nil && symbolSettings.CustomROIPercent > 0 {
				threshold = symbolSettings.CustomROIPercent
				source = "symbol_custom"
				fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Using SHARED symbol custom ROI = %.4f%% from settings\n",
					pos.Symbol, threshold)
			} else {
				if symbolSettings == nil {
					fmt.Printf("[EARLY-PROFIT-DEBUG] %s: No symbol settings found (shared or user)\n", pos.Symbol)
				} else {
					fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Symbol settings found but CustomROIPercent = %.4f (not set)\n",
						pos.Symbol, symbolSettings.CustomROIPercent)
				}

			// 3. Fallback to mode-based threshold from ModeConfigs (not hardcoded)
			// Use mode-specific TP% from settings, converted to ROI by multiplying by leverage
			settings := settingsManager.GetCurrentSettings()
			modeToConfigKey := map[string]string{
				string(GinieModeUltraFast): "ultra_fast",
				string(GinieModeScalp):     "scalp",
				string(GinieModeSwing):     "swing",
				string(GinieModePosition):  "position",
			}
			modeDefaults := map[string]float64{
				"ultra_fast": 2.0,
				"scalp":      3.0,
				"swing":      5.0,
				"position":   8.0,
			}

			var tpPercent float64
			modeKey, ok := modeToConfigKey[string(pos.Mode)]
			if !ok {
				tpPercent = 5.0
				source = "default"
			} else {
				tpPercent = modeDefaults[modeKey]
				source = modeKey + "_settings"
				if modeConfig := settings.ModeConfigs[modeKey]; modeConfig != nil {
					if modeConfig.SLTP != nil && modeConfig.SLTP.TakeProfitPercent > 0 {
						tpPercent = modeConfig.SLTP.TakeProfitPercent
					}
				}
			}

			// Convert TP% to ROI threshold: ROI = TP% × leverage
			// This ensures we book profit when price move reaches the mode's TP target
			threshold = tpPercent * float64(pos.Leverage)
			fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Using %s TP=%.2f%% × leverage=%d = ROI threshold %.2f%%\n",
				pos.Symbol, source, tpPercent, pos.Leverage, threshold)
			}
		}
	}

	// FIX: Enforce minimum threshold to prevent booking at 0% or near-zero ROI
	// This guards against misconfigured settings where threshold could be 0
	const minThreshold = 0.1 // Minimum 0.1% ROI required to book profit
	if threshold < minThreshold {
		fmt.Printf("[EARLY-PROFIT-DEBUG] %s: ⚠️ Threshold %.4f%% below minimum, using %.4f%%\n",
			pos.Symbol, threshold, minThreshold)
		threshold = minThreshold
	}

	// Check if ROI exceeds threshold
	fmt.Printf("[EARLY-PROFIT-DEBUG] %s: Comparing ROI %.4f%% >= Threshold %.4f%% (source: %s)\n",
		pos.Symbol, roiPercent, threshold, source)

	if roiPercent >= threshold {
		fmt.Printf("[EARLY-PROFIT-DEBUG] %s: ✅ YES - BOOKING EARLY PROFIT! ROI %.4f%% exceeds threshold %.4f%%\n",
			pos.Symbol, roiPercent, threshold)
		return true, roiPercent, source
	}

	fmt.Printf("[EARLY-PROFIT-DEBUG] %s: ❌ NO - ROI %.4f%% below threshold %.4f%%\n",
		pos.Symbol, roiPercent, threshold)
	return false, roiPercent, source
}

func (ga *GinieAutopilot) checkTakeProfits(pos *GiniePosition, currentPrice float64, pnlPercent float64) int {
	for i, tp := range pos.TakeProfits {
		if tp.Status == "hit" {
			continue
		}

		// Check if TP is hit (using tolerance-based comparison for commercial reliability)
		var tpHit bool
		if pos.Side == "LONG" {
			tpHit = priceGreaterOrEqual(pos.Symbol, currentPrice, tp.Price)
		} else {
			tpHit = priceLessOrEqual(pos.Symbol, currentPrice, tp.Price)
		}

		if tpHit {
			tpLevel := i + 1

			// Track TP hit for diagnostics
			ga.mu.Lock()
			ga.tpHitsLastHour++
			ga.mu.Unlock()

			// Handle all TP levels: partial close for each
			// CRITICAL FIX: TP4 now also executes partial close instead of just activating trailing
			// This prevents residual quantity from being left unsold
			ga.executePartialClose(pos, currentPrice, tpLevel)

			// After TP4 (final level), activate trailing for any dust remaining due to rounding
			if tpLevel >= 4 && pos.RemainingQty > 0 {
				pos.TrailingActive = true
				ga.logger.Info("Ginie TP4 hit - closed portion and activated trailing for dust",
					"symbol", pos.Symbol,
					"price", currentPrice,
					"remaining_qty", pos.RemainingQty)
			}

			// Mark TP as hit
			pos.TakeProfits[i].Status = "hit"
			pos.CurrentTPLevel = tpLevel

			// Move SL to breakeven after TP1 and update Binance order
			if tpLevel == 1 && ga.config.MoveToBreakevenAfterTP1 && !pos.MovedToBreakeven {
				ga.moveToBreakeven(pos)
				ga.updateBinanceSLOrder(pos) // CRITICAL: Update the actual Binance SL order
			}

			// Place the next TP order on Binance (TP2 after TP1, TP3 after TP2, etc.)
			if tpLevel < len(pos.TakeProfits) {
				ga.logger.Info("TP level hit - placing next TP order",
					"symbol", pos.Symbol,
					"current_tp_level", tpLevel,
					"next_tp_level", tpLevel+1,
					"remaining_qty", pos.RemainingQty,
					"next_tp_price", pos.TakeProfits[tpLevel].Price)
				ga.placeNextTPOrder(pos, tpLevel)
			} else {
				ga.logger.Info("Final TP level hit - no more TPs to place",
					"symbol", pos.Symbol,
					"tp_level", tpLevel,
					"total_tp_levels", len(pos.TakeProfits),
					"trailing_active", pos.TrailingActive)
			}

			// Log TP hit to trade lifecycle
			if ga.eventLogger != nil && pos.FuturesTradeID > 0 {
				// Calculate PnL for this TP level
				tpConfig := pos.TakeProfits[tpLevel-1]
				closeQty := pos.OriginalQty * (tpConfig.Percent / 100.0)
				var tpPnL float64
				if pos.Side == "LONG" {
					tpPnL = (currentPrice - pos.EntryPrice) * closeQty
				} else {
					tpPnL = (pos.EntryPrice - currentPrice) * closeQty
				}
				go ga.eventLogger.LogTPHit(
					context.Background(),
					pos.FuturesTradeID,
					pos.Symbol,
					tpLevel,
					currentPrice,
					closeQty,
					tpPnL,
					pnlPercent,
				)
			}

			return tpLevel
		}
	}

	return 0
}

// executePartialClose closes a portion of the position
func (ga *GinieAutopilot) executePartialClose(pos *GiniePosition, currentPrice float64, tpLevel int) {
	// Calculate quantity to close
	tpConfig := pos.TakeProfits[tpLevel-1]
	closePercent := tpConfig.Percent / 100.0
	closeQty := roundQuantity(pos.Symbol, pos.OriginalQty*closePercent)

	if closeQty <= 0 || closeQty > pos.RemainingQty {
		return
	}

	// CRITICAL FIX: Check if TP algo order was already triggered on Binance
	// This prevents double-execution when both the algo order triggers AND we detect TP hit via price
	if len(pos.TakeProfitAlgoIDs) > 0 && pos.TakeProfitAlgoIDs[0] > 0 {
		algoID := pos.TakeProfitAlgoIDs[0] // Current active TP is always at index 0
		// Check algo order status on Binance
		algoOrders, err := ga.futuresClient.GetAllAlgoOrders(pos.Symbol, 100)
		if err == nil {
			for _, order := range algoOrders {
				if order.AlgoId == algoID {
					if order.AlgoStatus == "TRIGGERED" || order.AlgoStatus == "FILLED" {
						ga.logger.Info("TP algo order already triggered on Binance - skipping duplicate close order",
							"symbol", pos.Symbol,
							"tp_level", tpLevel,
							"algo_id", algoID,
							"algo_status", order.AlgoStatus)
						// Algo already executed - just update internal state without placing another order
						pos.RemainingQty -= closeQty
						pos.TakeProfits[tpLevel-1].Status = "hit"
						// Update PnL tracking (algo order already closed position)
						var grossPnlAlgo float64
						if pos.Side == "LONG" {
							grossPnlAlgo = (currentPrice - pos.EntryPrice) * closeQty
						} else {
							grossPnlAlgo = (pos.EntryPrice - currentPrice) * closeQty
						}
						exitFeeAlgo := calculateTradingFee(closeQty, currentPrice)
						pnlAlgo := grossPnlAlgo - exitFeeAlgo
						pos.RealizedPnL += pnlAlgo
						ga.dailyPnL += pnlAlgo
						ga.totalPnL += pnlAlgo
						// Track for diagnostics
						ga.mu.Lock()
						ga.partialClosesLastHour++
						if pnlAlgo > 0 {
							ga.winningTrades++
						} else {
						}
						ga.mu.Unlock()
						return
					}
					break
				}
			}
		}
	}

	// Calculate PnL for this portion (both USD and percentage for circuit breaker)
	var grossPnl float64
	var pnlPercent float64
	if pos.Side == "LONG" {
		grossPnl = (currentPrice - pos.EntryPrice) * closeQty
		pnlPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
	} else {
		grossPnl = (pos.EntryPrice - currentPrice) * closeQty
		pnlPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
	}

	// Calculate and deduct trading fees (only exit fee)
	// CRITICAL: Entry fee was already paid when position opened
	// Only deduct exit fee for this partial close to avoid double-counting
	exitFee := calculateTradingFee(closeQty, currentPrice)
	totalFee := exitFee
	pnl := grossPnl - totalFee

	ga.logger.Info("Ginie partial close at TP",
		"symbol", pos.Symbol,
		"tp_level", tpLevel,
		"close_qty", closeQty,
		"close_percent", tpConfig.Percent,
		"price", currentPrice,
		"gross_pnl", grossPnl,
		"fees", totalFee,
		"net_pnl", pnl)

	if !ga.config.DryRun {
		// Place close order using LIMIT to avoid slippage
		side := "SELL"
		positionSide := binance.PositionSideLong
		if pos.Side == "SHORT" {
			side = "BUY"
			positionSide = binance.PositionSideShort
		}

		// Check actual Binance position mode to avoid API error -4061
		effectivePositionSide := ga.getEffectivePositionSide(positionSide)

		// Use LIMIT order at slightly worse price (0.1% buffer) to ensure execution
		// This avoids slippage on volatile movements during order execution
		// For LONG: sell at 0.1% below current price
		// For SHORT: buy at 0.1% above current price
		closePrice := currentPrice
		if pos.Side == "LONG" {
			closePrice = currentPrice * 0.999 // 0.1% buffer below
		} else {
			closePrice = currentPrice * 1.001 // 0.1% buffer above
		}

		orderParams := binance.FuturesOrderParams{
			Symbol:       pos.Symbol,
			Side:         side,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeLimit,
			Quantity:     closeQty,
			Price:        closePrice, // LIMIT order with 0.1% buffer
		}

		_, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
		if err != nil {
			ga.logger.Error("Ginie partial close failed", "symbol", pos.Symbol, "error", err)
			// Track failed order for diagnostics
			ga.mu.Lock()
			ga.failedOrdersLastHour++
			ga.mu.Unlock()
			return
		}

		ga.logger.Info("Ginie partial close order placed (LIMIT)",
			"symbol", pos.Symbol,
			"side", side,
			"current_price", currentPrice,
			"limit_price", closePrice,
			"quantity", closeQty)
	}

	// Track successful partial close for diagnostics
	ga.mu.Lock()
	ga.partialClosesLastHour++
	ga.mu.Unlock()

	// Update position
	pos.RemainingQty -= closeQty
	pos.RealizedPnL += pnl
	ga.dailyPnL += pnl
	ga.totalPnL += pnl

	if pnl > 0 {
		ga.winningTrades++
	}

	// Persist PnL stats
	go ga.SavePnLStats()

	// Record to circuit breaker (if enabled) - uses PERCENTAGE not USD
	if ga.config.CircuitBreakerEnabled && ga.circuitBreaker != nil {
		ga.circuitBreaker.RecordTrade(pnlPercent)
	}

	// Record trade with original signal info for study
	tradeResult := GinieTradeResult{
		Symbol:     pos.Symbol,
		Action:     "partial_close",
		Side:       pos.Side,
		Quantity:   closeQty,
		Price:      currentPrice,
		PnL:        pnl,
		PnLPercent: tpConfig.GainPct,
		Reason:     fmt.Sprintf("TP%d hit (%.0f%%)", tpLevel, tpConfig.Percent),
		TPLevel:    tpLevel,
		Timestamp:  time.Now(),
		Mode:       pos.Mode,
	}

	// Add original entry info if available
	if pos.DecisionReport != nil {
		tradeResult.Confidence = pos.DecisionReport.ConfidenceScore
		tradeResult.EntryParams = &GinieEntryParams{
			EntryPrice:  pos.EntryPrice,
			StopLoss:    pos.OriginalSL,
			Leverage:    pos.Leverage,
		}
	}

	ga.recordTrade(tradeResult)
}

// moveToBreakeven moves stop loss to entry price + buffer
func (ga *GinieAutopilot) moveToBreakeven(pos *GiniePosition) {
	buffer := pos.EntryPrice * (ga.config.BreakevenBuffer / 100)

	if pos.Side == "LONG" {
		pos.StopLoss = pos.EntryPrice + buffer
	} else {
		pos.StopLoss = pos.EntryPrice - buffer
	}

	pos.MovedToBreakeven = true

	ga.logger.Info("Ginie moved SL to breakeven",
		"symbol", pos.Symbol,
		"entry", pos.EntryPrice,
		"new_sl", pos.StopLoss,
		"buffer", ga.config.BreakevenBuffer)

	// Log breakeven event to trade lifecycle
	if ga.eventLogger != nil && pos.FuturesTradeID > 0 {
		go ga.eventLogger.LogMovedToBreakeven(
			context.Background(),
			pos.FuturesTradeID,
			pos.Symbol,
			pos.EntryPrice,
			pos.StopLoss,
			ga.config.BreakevenBuffer,
		)
	}
}

// placeNextTPOrder places the next TP order on Binance after a TP level is hit
func (ga *GinieAutopilot) placeNextTPOrder(pos *GiniePosition, currentTPLevel int) {
	nextTPIndex := currentTPLevel // currentTPLevel is 1-based, so index for next is same as level
	if nextTPIndex >= len(pos.TakeProfits) {
		ga.logger.Info("Cannot place next TP - index out of bounds",
			"symbol", pos.Symbol,
			"current_tp_level", currentTPLevel,
			"next_tp_index", nextTPIndex,
			"total_tp_levels", len(pos.TakeProfits))
		return // No more TP levels
	}

	ga.logger.Info("placeNextTPOrder called",
		"symbol", pos.Symbol,
		"current_tp_level", currentTPLevel,
		"next_tp_level", nextTPIndex+1,
		"next_tp_price", pos.TakeProfits[nextTPIndex].Price,
		"dry_run", ga.config.DryRun)

	// CRITICAL: Cancel ALL algo orders for this symbol before placing new one
	// This is aggressive but necessary to prevent order accumulation
	success, failed, err := ga.cancelAllAlgoOrdersForSymbol(pos.Symbol)
	if err != nil {
		ga.logger.Warn("Some algo orders failed to cancel before new TP placement",
			"symbol", pos.Symbol,
			"successful", success,
			"failed", failed,
			"error", err)
	} else if success > 0 {
		ga.logger.Info("Successfully cancelled all algo orders before new TP placement",
			"symbol", pos.Symbol,
			"cancelled_count", success)
	}

	time.Sleep(100 * time.Millisecond) // Wait for cancellation to process

	// Also clear tracking
	pos.TakeProfitAlgoIDs = nil
	pos.StopLossAlgoID = 0

	nextTP := pos.TakeProfits[nextTPIndex]
	if nextTP.Price <= 0 {
		ga.logger.Warn("Next TP price is invalid",
			"symbol", pos.Symbol,
			"tp_level", nextTPIndex+1,
			"price", nextTP.Price)
		return
	}

	if ga.config.DryRun {
		ga.logger.Info("Dry run: would place next TP order",
			"symbol", pos.Symbol,
			"tp_level", nextTPIndex+1,
			"price", nextTP.Price)
		return
	}

	// Calculate quantity for next TP
	tpQty := roundQuantity(pos.Symbol, pos.OriginalQty*(nextTP.Percent/100.0))

	// Ensure we don't try to close more than remaining
	if tpQty > pos.RemainingQty {
		tpQty = pos.RemainingQty
	}

	if tpQty <= 0 {
		ga.logger.Warn("Calculated TP quantity is zero or negative",
			"symbol", pos.Symbol,
			"tp_level", nextTPIndex+1,
			"remaining_qty", pos.RemainingQty)
		return
	}

	// Determine order side (opposite of position)
	closeSide := "SELL"
	positionSide := binance.PositionSideLong
	if pos.Side == "SHORT" {
		closeSide = "BUY"
		positionSide = binance.PositionSideShort
	}

	// Check actual Binance position mode to avoid API error -4061
	effectivePositionSide := ga.getEffectivePositionSide(positionSide)

	// Round TP price with directional rounding to ensure trigger fires
	roundedTPPrice := roundPriceForTP(pos.Symbol, nextTP.Price, pos.Side)

	// Get current price to check if TP is already passed
	currentPrice, priceErr := ga.futuresClient.GetFuturesCurrentPrice(pos.Symbol)
	if priceErr != nil {
		ga.logger.Warn("Failed to get current price for next TP check", "symbol", pos.Symbol, "error", priceErr)
		currentPrice = 0
	}

	// Check if price already passed this TP level
	tpAlreadyPassed := false
	if currentPrice > 0 {
		if pos.Side == "LONG" && currentPrice >= roundedTPPrice {
			tpAlreadyPassed = true
		} else if pos.Side == "SHORT" && currentPrice <= roundedTPPrice {
			tpAlreadyPassed = true
		}
	}

	if tpAlreadyPassed {
		// Price already passed this TP - execute market order immediately
		log.Printf("[GINIE] %s: TP%d already passed (price=%.4f, tp=%.4f), executing market order",
			pos.Symbol, nextTPIndex+1, currentPrice, roundedTPPrice)

		orderParams := binance.FuturesOrderParams{
			Symbol:       pos.Symbol,
			Side:         closeSide,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     tpQty,
		}

		order, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
		if err != nil {
			ga.logger.Error("Failed to execute immediate TP market order",
				"symbol", pos.Symbol,
				"tp_level", nextTPIndex+1,
				"error", err.Error())
		} else {
			// Calculate and record PnL
			var pnl float64
			if pos.Side == "LONG" {
				pnl = (currentPrice - pos.EntryPrice) * tpQty
			} else {
				pnl = (pos.EntryPrice - currentPrice) * tpQty
			}

			pos.TakeProfits[nextTPIndex].Status = "hit"
			pos.CurrentTPLevel = nextTPIndex + 1
			pos.RemainingQty -= tpQty
			pos.RealizedPnL += pnl
			ga.dailyPnL += pnl
			ga.totalPnL += pnl

			// If TP4, activate trailing
			if nextTPIndex+1 == 4 {
				pos.TrailingActive = true
			}

			ga.logger.Info("Immediate TP executed successfully",
				"symbol", pos.Symbol,
				"tp_level", nextTPIndex+1,
				"order_id", order.OrderId,
				"executed_qty", tpQty,
				"pnl", pnl)

			// Place next TP order if available
			if nextTPIndex+1 < len(pos.TakeProfits) {
				ga.placeNextTPOrder(pos, nextTPIndex+1)
			} else {
				// Last TP executed - ensure SL is placed for remaining qty if not trailing
				if pos.RemainingQty > 0 && !pos.TrailingActive {
					ga.placeSLOrder(pos)
				}
			}
		}
		return
	}

	// Normal case - place TP algo order
	// CRITICAL FIX: For final TP level, use ClosePosition=true to prevent residual quantity
	isFinalTPLevel := nextTPIndex >= len(pos.TakeProfits)-1

	var tpParams binance.AlgoOrderParams
	if isFinalTPLevel {
		// Final TP level - close entire remaining position
		ga.logger.Info("Placing final TP with ClosePosition=true",
			"symbol", pos.Symbol,
			"tp_level", nextTPIndex+1,
			"trigger_price", roundedTPPrice)
		tpParams = binance.AlgoOrderParams{
			Symbol:        pos.Symbol,
			Side:          closeSide,
			PositionSide:  effectivePositionSide,
			Type:          binance.FuturesOrderTypeTakeProfitMarket,
			ClosePosition: true, // Close entire remaining position
			TriggerPrice:  roundedTPPrice,
			WorkingType:   binance.WorkingTypeMarkPrice,
		}
	} else {
		// Intermediate TP level - use calculated quantity
		tpParams = binance.AlgoOrderParams{
			Symbol:       pos.Symbol,
			Side:         closeSide,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeTakeProfitMarket,
			Quantity:     tpQty,
			TriggerPrice: roundedTPPrice,
			WorkingType:  binance.WorkingTypeMarkPrice,
		}
	}

	// Place TP with retry logic
	const maxTPRetries = 3
	tpRetryDelay := 500 * time.Millisecond
	var tpOrderPlaced bool

	for attempt := 1; attempt <= maxTPRetries; attempt++ {
		tpOrder, err := ga.futuresClient.PlaceAlgoOrder(tpParams)
		if err == nil && tpOrder != nil && tpOrder.AlgoId > 0 {
			pos.TakeProfitAlgoIDs = append(pos.TakeProfitAlgoIDs, tpOrder.AlgoId)
			if isFinalTPLevel {
				ga.logger.Info("Final take profit order placed (ClosePosition=true)",
					"symbol", pos.Symbol,
					"tp_level", nextTPIndex+1,
					"algo_id", tpOrder.AlgoId,
					"trigger_price", roundedTPPrice,
					"close_position", true,
					"attempt", attempt)
			} else {
				ga.logger.Info("Next take profit order placed",
					"symbol", pos.Symbol,
					"tp_level", nextTPIndex+1,
					"algo_id", tpOrder.AlgoId,
					"trigger_price", roundedTPPrice,
					"quantity", tpQty,
					"attempt", attempt)
			}
			tpOrderPlaced = true
			break
		}
		ga.logger.Error("Failed to place next take profit order",
			"symbol", pos.Symbol,
			"tp_level", nextTPIndex+1,
			"tp_price", nextTP.Price,
			"attempt", attempt,
			"max_retries", maxTPRetries,
			"error", err.Error())
		if attempt < maxTPRetries {
			time.Sleep(tpRetryDelay * time.Duration(attempt))
		}
	}

	if !tpOrderPlaced {
		ga.logger.Error("CRITICAL: Next TP order NOT placed after all retries",
			"symbol", pos.Symbol,
			"tp_level", nextTPIndex+1,
			"tp_price", roundedTPPrice)
		return
	}

	// CRITICAL FIX: Place a new SL order for remaining quantity
	// Without this, the remaining position is unprotected after TP placement
	ga.placeSLOrder(pos)
}

// updateBinanceSLOrder cancels the existing SL algo order and places a new one at the updated price
// This is critical for trailing stops - without this, Binance would still trigger at the old SL price
func (ga *GinieAutopilot) updateBinanceSLOrder(pos *GiniePosition) {
	if ga.config.DryRun {
		ga.logger.Info("Dry run: would update Binance SL order",
			"symbol", pos.Symbol,
			"new_sl", pos.StopLoss)
		return
	}

	if pos.StopLossAlgoID == 0 {
		ga.logger.Warn("No existing SL algo order to update",
			"symbol", pos.Symbol)
		// Just place a new one
		ga.placeSLOrder(pos)
		return
	}

	// Cancel existing SL order
	err := ga.futuresClient.CancelAlgoOrder(pos.Symbol, pos.StopLossAlgoID)
	if err != nil {
		ga.logger.Error("Failed to cancel existing SL algo order",
			"symbol", pos.Symbol,
			"algo_id", pos.StopLossAlgoID,
			"error", err.Error())
		// Try to place new one anyway
	} else {
		ga.logger.Info("Cancelled existing SL algo order",
			"symbol", pos.Symbol,
			"old_algo_id", pos.StopLossAlgoID)
	}

	// Place new SL order at updated price
	ga.placeSLOrder(pos)
}

// placeSLOrder places a new SL algo order (helper for updateBinanceSLOrder)
// CRITICAL: Uses ClosePosition=true to ensure the ENTIRE position is closed regardless of quantity tracking
func (ga *GinieAutopilot) placeSLOrder(pos *GiniePosition) {
	closeSide := "SELL"
	positionSide := binance.PositionSideLong
	if pos.Side == "SHORT" {
		closeSide = "BUY"
		positionSide = binance.PositionSideShort
	}

	// CRITICAL FIX: Check actual Binance position mode to avoid API error -4061
	effectivePositionSide := ga.getEffectivePositionSide(positionSide)

	// Round SL price with directional rounding to ensure trigger protects capital
	roundedSL := roundPriceForSL(pos.Symbol, pos.StopLoss, pos.Side)

	// CRITICAL FIX: Use ClosePosition=true instead of specifying quantity
	// This ensures Binance closes the ENTIRE position on the exchange, avoiding residual quantity issues
	// caused by rounding mismatches between internal tracking and actual exchange position
	slParams := binance.AlgoOrderParams{
		Symbol:        pos.Symbol,
		Side:          closeSide,
		PositionSide:  effectivePositionSide,
		Type:          binance.FuturesOrderTypeStopMarket,
		ClosePosition: true, // Close entire position - no quantity needed
		TriggerPrice:  roundedSL,
		WorkingType:   binance.WorkingTypeMarkPrice,
	}

	// Place SL with retry logic - CRITICAL for position protection
	const maxSLRetries = 3
	slRetryDelay := 500 * time.Millisecond
	var slOrderPlaced bool

	for attempt := 1; attempt <= maxSLRetries; attempt++ {
		slOrder, err := ga.futuresClient.PlaceAlgoOrder(slParams)
		if err == nil && slOrder != nil && slOrder.AlgoId > 0 {
			pos.StopLossAlgoID = slOrder.AlgoId
			ga.logger.Info("Updated SL order placed (ClosePosition=true)",
				"symbol", pos.Symbol,
				"new_algo_id", slOrder.AlgoId,
				"trigger_price", roundedSL,
				"close_position", true,
				"attempt", attempt)
			slOrderPlaced = true
			break
		}
		ga.logger.Error("Failed to place updated SL order",
			"symbol", pos.Symbol,
			"sl_price", roundedSL,
			"attempt", attempt,
			"max_retries", maxSLRetries,
			"error", err.Error())
		if attempt < maxSLRetries {
			time.Sleep(slRetryDelay * time.Duration(attempt))
		}
	}

	if !slOrderPlaced {
		ga.logger.Error("CRITICAL: Updated SL order NOT placed after all retries - position unprotected!",
			"symbol", pos.Symbol,
			"sl_price", roundedSL)
	}
}

// checkTrailingStop checks if trailing stop is triggered
func (ga *GinieAutopilot) checkTrailingStop(pos *GiniePosition, currentPrice float64) bool {
	if !pos.TrailingActive {
		return false
	}

	var pullback float64
	if pos.Side == "LONG" {
		pullback = (pos.HighestPrice - currentPrice) / pos.HighestPrice * 100
	} else {
		pullback = (currentPrice - pos.LowestPrice) / pos.LowestPrice * 100
	}

	// Use tolerance for floating point precision (prevents edge case failures on small-cap coins)
	tolerance := 0.01 // 0.01% tolerance
	return pullback >= (pos.TrailingPercent - tolerance)
}

// closePosition closes the entire remaining position
func (ga *GinieAutopilot) closePosition(symbol string, pos *GiniePosition, currentPrice float64, reason string, tpLevel int) {
	// CRITICAL: Cancel all remaining algo orders FIRST to prevent orphan orders
	// This must happen before anything else to avoid race conditions
	log.Printf("[GINIE] %s: Closing position, cancelling all algo orders (SL_ID=%d, TP_IDs=%v)",
		symbol, pos.StopLossAlgoID, pos.TakeProfitAlgoIDs)
	success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
	if err != nil || failed > 0 {
		ga.logger.Warn("Failed to cancel all algo orders on position close",
			"symbol", symbol,
			"success", success,
			"failed", failed,
			"error", err)
	}

	// Calculate PnL
	var grossPnl float64
	var pnlPercent float64
	if pos.Side == "LONG" {
		grossPnl = (currentPrice - pos.EntryPrice) * pos.RemainingQty
		pnlPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
	} else {
		grossPnl = (pos.EntryPrice - currentPrice) * pos.RemainingQty
		pnlPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
	}

	// Calculate and deduct trading fees (only exit fee for remaining quantity)
	// CRITICAL: Entry fee was already paid when position opened
	// Only deduct exit fee for this final close to avoid double-counting
	exitFee := calculateTradingFee(pos.RemainingQty, currentPrice)
	totalFee := exitFee
	pnl := grossPnl - totalFee

	totalPnL := pos.RealizedPnL + pnl

	ga.logger.Info("Ginie closing position",
		"symbol", symbol,
		"reason", reason,
		"remaining_qty", pos.RemainingQty,
		"price", currentPrice,
		"partial_pnl", pos.RealizedPnL,
		"gross_pnl", grossPnl,
		"fees", totalFee,
		"net_pnl", pnl,
		"total_pnl", totalPnL)

	// Log position closed to trade lifecycle
	if ga.eventLogger != nil && pos.FuturesTradeID > 0 {
		go ga.eventLogger.LogPositionClosed(
			context.Background(),
			pos.FuturesTradeID,
			symbol,
			currentPrice,
			pos.RemainingQty,
			totalPnL,
			pnlPercent,
			reason,
			database.EventSourceGinie,
		)
	}

	if !ga.config.DryRun && pos.RemainingQty > 0 {
		// Place close order using LIMIT to avoid slippage on SL/Trailing closes
		// This is critical for SL/Trailing stop to avoid worst-case execution
		side := "SELL"
		positionSide := binance.PositionSideLong
		if pos.Side == "SHORT" {
			side = "BUY"
			positionSide = binance.PositionSideShort
		}

		// Check actual Binance position mode to avoid API error -4061
		effectivePositionSide := ga.getEffectivePositionSide(positionSide)

		// Use LIMIT order at slightly worse price (0.1% buffer) to ensure execution
		// Avoids slippage especially on volatile coins where price moves between detection and execution
		// For LONG: sell at 0.1% below current price to ensure fill
		// For SHORT: buy at 0.1% above current price to ensure fill
		closePrice := currentPrice
		if pos.Side == "LONG" {
			closePrice = currentPrice * 0.999 // 0.1% buffer below for LONG
		} else {
			closePrice = currentPrice * 1.001 // 0.1% buffer above for SHORT
		}

		// CRITICAL FIX: Round quantity and price to match Binance's precision requirements
		// Without this, orders are rejected with precision errors
		roundedQty := roundQuantity(symbol, pos.RemainingQty)
		roundedPrice := roundPriceForTP(symbol, closePrice, pos.Side)  // Ensure tick-size alignment

		ga.logger.Info("Placing close order",
			"symbol", symbol,
			"side", side,
			"qty", roundedQty,
			"price", roundedPrice,
			"current_price", currentPrice,
			"raw_qty", pos.RemainingQty,
			"raw_price", closePrice)

		orderParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         side,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeLimit,
			Quantity:     roundedQty,
			Price:        roundedPrice, // LIMIT order with 0.1% buffer
		}

		_, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
		if err != nil {
			ga.logger.Warn("LIMIT close order failed, falling back to MARKET order",
				"symbol", symbol,
				"error", err.Error(),
				"limit_price", roundedPrice,
				"qty", roundedQty)

			// FALLBACK: Use MARKET order if LIMIT fails (e.g., due to price precision issues)
			marketParams := binance.FuturesOrderParams{
				Symbol:       symbol,
				Side:         side,
				PositionSide: effectivePositionSide,
				Type:         binance.FuturesOrderTypeMarket,
				Quantity:     roundedQty,
			}

			_, marketErr := ga.futuresClient.PlaceFuturesOrder(marketParams)
			if marketErr != nil {
				ga.logger.Error("Both LIMIT and MARKET close orders failed",
					"symbol", symbol,
					"limit_error", err.Error(),
					"market_error", marketErr.Error(),
					"qty", roundedQty,
					"reason", reason)
				return
			}

			ga.logger.Info("MARKET fallback close order placed successfully",
				"symbol", symbol,
				"reason", reason,
				"qty", roundedQty)
		} else {
			ga.logger.Info("Ginie full close order placed (LIMIT - SL/Trailing)",
				"symbol", symbol,
				"reason", reason,
				"current_price", currentPrice,
				"limit_price", closePrice,
				"quantity", pos.RemainingQty)
		}
	}

	// Update tracking
	ga.dailyPnL += pnl
	ga.totalPnL += pnl

	if totalPnL > 0 {
		ga.winningTrades++
	}

	// Persist PnL stats
	go ga.SavePnLStats()

	// Record to circuit breaker (if enabled) - uses PERCENTAGE not USD
	if ga.config.CircuitBreakerEnabled && ga.circuitBreaker != nil {
		ga.circuitBreaker.RecordTrade(pnlPercent)
	}

	// Per-coin consecutive loss tracking and blocking
	ga.updateCoinLossTracking(symbol, totalPnL, pnlPercent)

	// Record trade with original signal info for study
	tradeResult := GinieTradeResult{
		Symbol:     symbol,
		Action:     "full_close",
		Side:       pos.Side,
		Quantity:   pos.RemainingQty,
		Price:      currentPrice,
		PnL:        totalPnL,
		PnLPercent: pnlPercent,
		Reason:     reason,
		TPLevel:    tpLevel,
		Timestamp:  time.Now(),
		Mode:       pos.Mode,
	}

	// Add original entry and signal info if available
	if pos.DecisionReport != nil {
		tradeResult.Confidence = pos.DecisionReport.ConfidenceScore
		tradeResult.MarketConditions = &GinieMarketSnapshot{
			Trend:      pos.DecisionReport.MarketConditions.Trend,
			ADX:        pos.DecisionReport.MarketConditions.ADX,
			Volatility: pos.DecisionReport.MarketConditions.Volatility,
			ATRPercent: pos.DecisionReport.MarketConditions.ATR,
			Volume:     pos.DecisionReport.MarketConditions.Volume,
			BTCCorr:    pos.DecisionReport.MarketConditions.BTCCorr,
		}
		tradeResult.EntryParams = &GinieEntryParams{
			EntryPrice:  pos.EntryPrice,
			StopLoss:    pos.OriginalSL,
			Leverage:    pos.Leverage,
			RiskReward:  pos.DecisionReport.TradeExecution.RiskReward,
		}
	}

	ga.recordTrade(tradeResult)

	// Remove position
	delete(ga.positions, symbol)
}

// closePositionAtMarket closes a position immediately using a TRUE MARKET order
// Used for emergency close, LLM close signals, or when immediate execution is required
// CRITICAL: Uses MARKET order type to guarantee execution regardless of price precision issues
func (ga *GinieAutopilot) closePositionAtMarket(pos *GiniePosition) error {
	if pos == nil {
		return fmt.Errorf("nil position")
	}

	symbol := pos.Symbol

	// Cancel all algo orders first to prevent orphan orders
	log.Printf("[MARKET-CLOSE] %s: Cancelling all algo orders before market close", symbol)
	success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
	if err != nil || failed > 0 {
		ga.logger.Warn("Failed to cancel all algo orders on market close",
			"symbol", symbol,
			"success", success,
			"failed", failed,
			"error", err)
	}

	// Get current price for PnL calculation
	currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get current price: %w", err)
	}

	// Calculate PnL
	var grossPnl float64
	var pnlPercent float64
	if pos.Side == "LONG" {
		grossPnl = (currentPrice - pos.EntryPrice) * pos.RemainingQty
		pnlPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
	} else {
		grossPnl = (pos.EntryPrice - currentPrice) * pos.RemainingQty
		pnlPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
	}

	// Calculate fees
	exitFee := calculateTradingFee(pos.RemainingQty, currentPrice)
	pnl := grossPnl - exitFee
	totalPnL := pos.RealizedPnL + pnl

	ga.logger.Info("Closing position with MARKET order",
		"symbol", symbol,
		"remaining_qty", pos.RemainingQty,
		"current_price", currentPrice,
		"gross_pnl", grossPnl,
		"fees", exitFee,
		"net_pnl", pnl,
		"total_pnl", totalPnL)

	if !ga.config.DryRun && pos.RemainingQty > 0 {
		// Determine order side and position side
		side := "SELL"
		positionSide := binance.PositionSideLong
		if pos.Side == "SHORT" {
			side = "BUY"
			positionSide = binance.PositionSideShort
		}

		// Check actual Binance position mode to avoid API error -4061
		effectivePositionSide := ga.getEffectivePositionSide(positionSide)

		// Round quantity only (MARKET orders don't need price)
		roundedQty := roundQuantity(symbol, pos.RemainingQty)

		ga.logger.Info("Placing MARKET close order",
			"symbol", symbol,
			"side", side,
			"qty", roundedQty,
			"raw_qty", pos.RemainingQty)

		// Use MARKET order - no price needed, guaranteed execution
		orderParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         side,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     roundedQty,
		}

		_, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
		if err != nil {
			ga.logger.Error("MARKET close order failed",
				"symbol", symbol,
				"error", err.Error(),
				"qty", roundedQty)
			return fmt.Errorf("market close failed: %w", err)
		}

		ga.logger.Info("MARKET close order executed successfully",
			"symbol", symbol,
			"qty", roundedQty,
			"estimated_price", currentPrice)
	}

	// Update tracking
	ga.dailyPnL += pnl
	ga.totalPnL += pnl

	if totalPnL > 0 {
		ga.winningTrades++
	}

	// Update per-coin loss tracking
	ga.updateCoinLossTracking(symbol, pnl, pnlPercent)

	// Create trade result
	tradeResult := GinieTradeResult{
		Symbol:    symbol,
		Mode:      pos.Mode,
		Side:      pos.Side,
		Action:    "close_market",
		Price:     currentPrice,
		Quantity:  pos.OriginalQty,
		PnL:       totalPnL,
		PnLPercent: pnlPercent,
		Reason:    "market_close",
		Timestamp: time.Now(),
	}
	ga.recordTrade(tradeResult)

	// Remove position
	ga.mu.Lock()
	delete(ga.positions, symbol)
	ga.mu.Unlock()

	log.Printf("[MARKET-CLOSE] %s: Position closed successfully via MARKET order", symbol)

	return nil
}

// updateCoinLossTracking tracks per-coin consecutive losses and blocks coins with big losses
// Consecutive losses reset on ANY profit (even small)
// Coins with >50% negative ROI get blocked
// First block: auto-unblock after 2 hours
// Second+ block: requires manual unblock
func (ga *GinieAutopilot) updateCoinLossTracking(symbol string, pnl float64, pnlPercent float64) {
	// Initialize if needed
	if ga.coinConsecLosses == nil {
		ga.coinConsecLosses = make(map[string]int)
	}
	if ga.blockedCoins == nil {
		ga.blockedCoins = make(map[string]*CoinBlockInfo)
	}
	if ga.coinBlockHistory == nil {
		ga.coinBlockHistory = make(map[string]int)
	}

	// Check if this is a profit (even small) or a loss
	if pnl > 0 {
		// ANY profit resets consecutive losses for this coin
		if ga.coinConsecLosses[symbol] > 0 {
			ga.logger.Info("Ginie per-coin consecutive losses reset on profit",
				"symbol", symbol,
				"prev_consec", ga.coinConsecLosses[symbol],
				"profit", pnl)
		}
		ga.coinConsecLosses[symbol] = 0
		return
	}

	// It's a loss - increment consecutive losses for this coin
	ga.coinConsecLosses[symbol]++
	consecLosses := ga.coinConsecLosses[symbol]

	ga.logger.Warn("Ginie per-coin loss recorded",
		"symbol", symbol,
		"consecutive_losses", consecLosses,
		"pnl", pnl,
		"roi_percent", pnlPercent)

	// Check if this is a big single loss (>50% negative ROI with leverage)
	// The pnlPercent is already the leveraged ROI from price movement
	shouldBlock := false
	blockReason := ""

	// Calculate actual ROI considering leverage effect
	// A 50% loss on leveraged position is devastating
	if pnlPercent < -50 {
		shouldBlock = true
		blockReason = fmt.Sprintf("big single loss: %.1f%% ROI", pnlPercent)
	} else if consecLosses >= 3 && pnlPercent < -20 {
		// 3+ consecutive losses with at least 20% loss each is also bad
		shouldBlock = true
		blockReason = fmt.Sprintf("%d consecutive losses, last: %.1f%% ROI", consecLosses, pnlPercent)
	}

	if shouldBlock {
		// Check historical block count for this coin
		historicalBlocks := ga.coinBlockHistory[symbol]
		isRepeatOffender := historicalBlocks >= 1

		// Increment historical block count
		ga.coinBlockHistory[symbol] = historicalBlocks + 1

		blockInfo := &CoinBlockInfo{
			Symbol:       symbol,
			BlockReason:  blockReason,
			BlockTime:    time.Now(),
			LossAmount:   pnl,
			LossROI:      pnlPercent,
			ConsecLosses: consecLosses,
			BlockCount:   historicalBlocks + 1,
		}

		if isRepeatOffender {
			// Repeat offender - requires manual unblock
			blockInfo.ManualOnly = true
			blockInfo.AutoUnblock = time.Time{} // Zero time means no auto-unblock
			ga.logger.Error("Ginie BLOCKING coin (MANUAL UNBLOCK REQUIRED - repeat offender)",
				"symbol", symbol,
				"reason", blockReason,
				"block_count", blockInfo.BlockCount,
				"loss_amount", pnl)
		} else {
			// First time block - auto-unblock after 2 hours
			blockInfo.ManualOnly = false
			blockInfo.AutoUnblock = time.Now().Add(2 * time.Hour)
			ga.logger.Warn("Ginie BLOCKING coin (auto-unblock in 2 hours)",
				"symbol", symbol,
				"reason", blockReason,
				"auto_unblock", blockInfo.AutoUnblock.Format("15:04:05"),
				"loss_amount", pnl)
		}

		ga.blockedCoins[symbol] = blockInfo

		// Reset consecutive losses since we've handled it
		ga.coinConsecLosses[symbol] = 0
	}
}

// isCoinBlocked checks if a coin is blocked and handles auto-unblock
func (ga *GinieAutopilot) isCoinBlocked(symbol string) (bool, string) {
	if ga.blockedCoins == nil {
		return false, ""
	}

	blockInfo, exists := ga.blockedCoins[symbol]
	if !exists {
		return false, ""
	}

	// Check for auto-unblock
	if !blockInfo.ManualOnly && !blockInfo.AutoUnblock.IsZero() {
		if time.Now().After(blockInfo.AutoUnblock) {
			// Auto-unblock time passed
			ga.logger.Info("Ginie auto-unblocking coin",
				"symbol", symbol,
				"was_blocked_for", blockInfo.BlockReason,
				"blocked_since", blockInfo.BlockTime.Format("15:04:05"))
			delete(ga.blockedCoins, symbol)
			return false, ""
		}

		remaining := time.Until(blockInfo.AutoUnblock).Round(time.Minute)
		return true, fmt.Sprintf("blocked: %s (auto-unblock in %v)", blockInfo.BlockReason, remaining)
	}

	// Manual unblock required
	return true, fmt.Sprintf("blocked: %s (MANUAL UNBLOCK REQUIRED, blocked %d times)",
		blockInfo.BlockReason, blockInfo.BlockCount)
}

// GetBlockedCoins returns list of currently blocked coins
func (ga *GinieAutopilot) GetBlockedCoins() []*CoinBlockInfo {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	result := make([]*CoinBlockInfo, 0, len(ga.blockedCoins))
	for _, info := range ga.blockedCoins {
		// Check auto-unblock before adding
		if !info.ManualOnly && !info.AutoUnblock.IsZero() && time.Now().After(info.AutoUnblock) {
			continue // Skip - will be auto-unblocked
		}
		result = append(result, info)
	}
	return result
}

// UnblockCoin manually unblocks a coin
func (ga *GinieAutopilot) UnblockCoin(symbol string) error {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if ga.blockedCoins == nil {
		return fmt.Errorf("no blocked coins")
	}

	blockInfo, exists := ga.blockedCoins[symbol]
	if !exists {
		return fmt.Errorf("coin %s is not blocked", symbol)
	}

	ga.logger.Info("Ginie manually unblocking coin",
		"symbol", symbol,
		"was_blocked_for", blockInfo.BlockReason,
		"was_blocked_since", blockInfo.BlockTime.Format("15:04:05"),
		"block_count", blockInfo.BlockCount)

	delete(ga.blockedCoins, symbol)
	return nil
}

// ResetCoinBlockHistory clears the block history for a coin (for fresh start)
func (ga *GinieAutopilot) ResetCoinBlockHistory(symbol string) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if ga.coinBlockHistory != nil {
		delete(ga.coinBlockHistory, symbol)
		ga.logger.Info("Ginie reset coin block history", "symbol", symbol)
	}
}

// Helper functions

func (ga *GinieAutopilot) getTPPercent(level int) float64 {
	switch level {
	case 1:
		return ga.config.TP1Percent
	case 2:
		return ga.config.TP2Percent
	case 3:
		return ga.config.TP3Percent
	case 4:
		return ga.config.TP4Percent
	default:
		return 25.0
	}
}

// getModeConfig retrieves the mode configuration from SettingsManager with fallback to nil
// This is a helper method that provides a unified way to get mode-specific configuration
func (ga *GinieAutopilot) getModeConfig(mode GinieTradingMode) *ModeFullConfig {
	sm := GetSettingsManager()
	if sm != nil {
		modeStr := string(mode)
		if modeConfig, err := sm.GetModeConfig(modeStr); err == nil && modeConfig != nil {
			return modeConfig
		}
	}
	return nil
}

// getTrailingPercent reads trailing stop percent from Mode Config
// Falls back to defaults if Mode Config is unavailable
func (ga *GinieAutopilot) getTrailingPercent(mode GinieTradingMode) float64 {
	// Try to read from Mode Config first
	sm := GetSettingsManager()
	if sm != nil {
		modeStr := string(mode)
		if modeConfig, err := sm.GetModeConfig(modeStr); err == nil && modeConfig != nil && modeConfig.SLTP != nil {
			if modeConfig.SLTP.TrailingStopPercent > 0 {
				return modeConfig.SLTP.TrailingStopPercent
			}
		}
	}

	// Fallback to defaults if Mode Config not available
	switch mode {
	case GinieModeScalp:
		return 0.3 // 0.3% trailing for scalp
	case GinieModeSwing:
		return 1.5 // 1.5% trailing for swing
	case GinieModePosition:
		return 3.0 // 3% trailing for position
	default:
		return 1.0
	}
}

// getTrailingActivation reads trailing stop activation threshold from Mode Config
// Returns the profit % at which trailing stop activates
func (ga *GinieAutopilot) getTrailingActivation(mode GinieTradingMode) float64 {
	// Try to read from Mode Config first
	sm := GetSettingsManager()
	if sm != nil {
		modeStr := string(mode)
		if modeConfig, err := sm.GetModeConfig(modeStr); err == nil && modeConfig != nil && modeConfig.SLTP != nil {
			// Return the configured activation (even if 0, that's valid = disabled)
			return modeConfig.SLTP.TrailingStopActivation
		}
	}

	// Fallback to defaults if Mode Config not available
	switch mode {
	case GinieModeScalp:
		return 0.5 // 0.5% profit to activate
	case GinieModeSwing:
		return 1.0 // 1.0% profit to activate
	case GinieModePosition:
		return 2.0 // 2.0% profit to activate
	default:
		return 1.0
	}
}

// isTrailingEnabled checks if trailing stop is enabled for the given mode from Mode Config
func (ga *GinieAutopilot) isTrailingEnabled(mode GinieTradingMode) bool {
	// Try to read from Mode Config first
	sm := GetSettingsManager()
	if sm != nil {
		modeStr := string(mode)
		if modeConfig, err := sm.GetModeConfig(modeStr); err == nil && modeConfig != nil && modeConfig.SLTP != nil {
			return modeConfig.SLTP.TrailingStopEnabled
		}
	}

	// Fallback to defaults if Mode Config not available
	switch mode {
	case GinieModeSwing, GinieModePosition:
		return true // Default enabled for swing/position
	default:
		return false // Default disabled for scalp/ultra-fast
	}
}

func (ga *GinieAutopilot) generateDefaultTPs(symbol string, entryPrice float64, mode GinieTradingMode, isLong bool) []GinieTakeProfitLevel {
	var gains []float64

	// Check if single TP mode is enabled for this mode
	sm := GetSettingsManager()
	settings := sm.GetCurrentSettings()
	useSingleTP := false

	// Also check mode config for use_single_tp
	modeConfig := ga.getModeConfig(mode)
	if modeConfig != nil && modeConfig.SLTP != nil {
		useSingleTP = modeConfig.SLTP.UseSingleTP
	}

	// For single TP modes, only use TP1 gain (first valid gain)
	if useSingleTP {
		var singleGain float64
		// Try to get from mode config
		if modeConfig != nil && modeConfig.SLTP != nil && len(modeConfig.SLTP.TPGainLevels) > 0 {
			// Find first non-zero gain
			for _, g := range modeConfig.SLTP.TPGainLevels {
				if g > 0.01 { // Skip near-zero values
					singleGain = g
					break
				}
			}
		}
		// Fallback to mode-specific defaults from ModeConfigs if no valid gain found
		if singleGain < 0.01 {
			modeToConfigKey := map[string]string{
				string(GinieModeUltraFast): "ultra_fast",
				string(GinieModeScalp):     "scalp",
				string(GinieModeSwing):     "swing",
				string(GinieModePosition):  "position",
			}
			modeDefaults := map[string]float64{
				"ultra_fast": 0.5,
				"scalp":      1.0,
				"swing":      2.0,
				"position":   3.0,
			}

			modeKey, ok := modeToConfigKey[string(mode)]
			if !ok {
				singleGain = 1.0
			} else {
				singleGain = modeDefaults[modeKey]
				if mc := settings.ModeConfigs[modeKey]; mc != nil && mc.SLTP != nil {
					if mc.SLTP.SingleTPPercent > 0 {
						singleGain = mc.SLTP.SingleTPPercent
					} else if mc.SLTP.TakeProfitPercent > 0 {
						singleGain = mc.SLTP.TakeProfitPercent
					}
				}
			}
		}
		gains = []float64{singleGain}
	} else {
		// Multi-TP mode: Try to get TP gain levels from mode config first
		if modeConfig != nil && modeConfig.SLTP != nil && len(modeConfig.SLTP.TPGainLevels) > 0 {
			gains = modeConfig.SLTP.TPGainLevels
		}

		// Validate gains - filter out 0% or near-zero values
		validGains := make([]float64, 0, len(gains))
		for _, g := range gains {
			if g >= 0.05 { // Minimum 0.05% gain to be meaningful
				validGains = append(validGains, g)
			}
		}

		// If no valid gains after filtering, use hardcoded defaults
		if len(validGains) == 0 {
			switch mode {
			case GinieModeUltraFast:
				gains = []float64{0.15, 0.3, 0.5, 0.8}
			case GinieModeScalp:
				gains = []float64{0.3, 0.6, 1.0, 1.5}
			case GinieModeSwing:
				gains = []float64{1.0, 2.0, 3.0, 4.0}
			case GinieModePosition:
				gains = []float64{2.0, 4.0, 6.0, 8.0}
			default:
				gains = []float64{1.0, 2.0, 3.0, 4.0} // Default to swing-like
			}
		} else {
			gains = validGains
		}
	}

	// Determine side string for proper rounding
	side := "LONG"
	if !isLong {
		side = "SHORT"
	}

	// Create TP levels based on gains (variable number of levels)
	tps := make([]GinieTakeProfitLevel, len(gains))
	for i, gain := range gains {
		var price float64
		if isLong {
			price = entryPrice * (1 + gain/100)
		} else {
			price = entryPrice * (1 - gain/100)
		}
		// FIX: Use roundPriceForTP with symbol and side for proper tick precision
		// This prevents TP prices from being rounded to values <= entry price
		tps[i] = GinieTakeProfitLevel{
			Level:   i + 1,
			Price:   roundPriceForTP(symbol, price, side),
			Percent: ga.getTPPercent(i + 1),
			GainPct: gain,
			Status:  "pending",
		}
	}

	return tps
}

func (ga *GinieAutopilot) calculateCurrentAllocation() float64 {
	var total float64
	for _, pos := range ga.positions {
		total += pos.EntryPrice * pos.RemainingQty / float64(pos.Leverage)
	}
	return total
}

func (ga *GinieAutopilot) recordTrade(result GinieTradeResult) {
	// Add to in-memory history
	ga.tradeHistory = append(ga.tradeHistory, result)
	if len(ga.tradeHistory) > ga.maxHistory {
		ga.tradeHistory = ga.tradeHistory[1:]
	}

	// Persist to database for analysis
	ga.persistTradeToDatabase(result)
}

// persistTradeToDatabase saves trade result with confidence to database for analysis
func (ga *GinieAutopilot) persistTradeToDatabase(result GinieTradeResult) {
	if ga.repo == nil {
		return // No database configured
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First, create an AI decision record with confidence info
	aiDecision := &database.AIDecision{
		Symbol:       result.Symbol,
		CurrentPrice: result.Price,
		Action:       result.Side,
		Confidence:   result.Confidence / 100.0, // Convert to 0-1 range if stored as percentage
		Reasoning:    fmt.Sprintf("Ginie %s: %s (Mode: %s)", result.Action, result.Reason, result.Mode),
		RiskLevel:    ga.currentRiskLevel,
		Executed:     true,
		CreatedAt:    result.Timestamp,
	}

	// Add signal component confidence if available
	if result.SignalSummary != nil {
		aiDecision.ConfluenceCount = result.SignalSummary.PrimaryMet
	}

	// Save AI decision
	if err := ga.repo.SaveAIDecision(ctx, aiDecision); err != nil {
		ga.logger.Debug("Failed to save AI decision to DB", "error", err, "symbol", result.Symbol)
		return
	}

	// For close actions, update the existing trade record
	if result.Action == "full_close" || result.Action == "partial_close" {
		// Create a trade record for tracking
		pnlPercent := result.PnLPercent
		trade := &database.FuturesTrade{
			Symbol:             result.Symbol,
			PositionSide:       result.Side,
			Side:               result.Side,
			EntryPrice:         result.Price - (result.PnL / result.Quantity), // Estimate entry from PnL
			ExitPrice:          &result.Price,
			Quantity:           result.Quantity,
			Leverage:           10, // Default, will be overwritten if we have position info
			MarginType:         "CROSSED",
			RealizedPnL:        &result.PnL,
			RealizedPnLPercent: &pnlPercent,
			Status:             "CLOSED",
			EntryTime:          result.Timestamp.Add(-1 * time.Hour), // Estimate
			ExitTime:           &result.Timestamp,
			TradeSource:        "ginie",
			AIDecisionID:       &aiDecision.ID,
		}

		if err := ga.repo.CreateFuturesTrade(ctx, trade); err != nil {
			ga.logger.Debug("Failed to save trade to DB", "error", err, "symbol", result.Symbol)
		} else {
			ga.logger.Debug("Trade persisted to DB",
				"symbol", result.Symbol,
				"confidence", result.Confidence,
				"pnl", result.PnL,
				"trade_id", trade.ID)
		}
	}
}

func (ga *GinieAutopilot) resetDailyCounters() {
	defer ga.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			ga.logger.Error("PANIC in daily reset goroutine - restarting", "panic", r)
			log.Printf("[GINIE-PANIC] Daily reset panic: %v", r)
			time.Sleep(5 * time.Second)
			ga.wg.Add(1)
			go ga.resetDailyCounters()
		}
	}()

	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		sleepDuration := time.Until(next)

		// Use select to allow goroutine to stop when autopilot stops
		select {
		case <-ga.stopChan:
			ga.logger.Info("Ginie daily reset goroutine stopping")
			return
		case <-time.After(sleepDuration):
			// Time to reset daily counters
		}

		ga.mu.Lock()
		ga.dailyTrades = 0
		ga.dailyPnL = 0
		ga.dayStart = time.Now().Truncate(24 * time.Hour)
		ga.mu.Unlock()

		ga.logger.Info("Ginie autopilot daily counters reset")
	}
}

// resetHourlyCounters resets profit booking metrics every hour
// These are "LastHour" metrics so they need to reset hourly for accuracy
func (ga *GinieAutopilot) resetHourlyCounters() {
	defer ga.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			ga.logger.Error("PANIC in hourly reset goroutine - restarting", "panic", r)
			log.Printf("[GINIE-PANIC] Hourly reset panic: %v", r)
			time.Sleep(5 * time.Second)
			ga.wg.Add(1)
			go ga.resetHourlyCounters()
		}
	}()

	for {
		// Reset at the top of every hour
		now := time.Now()
		next := now.Truncate(time.Hour).Add(time.Hour)
		sleepDuration := time.Until(next)

		select {
		case <-ga.stopChan:
			ga.logger.Info("Ginie hourly reset goroutine stopping")
			return
		case <-time.After(sleepDuration):
			// Time to reset hourly counters
		}

		ga.mu.Lock()
		ga.tpHitsLastHour = 0
		ga.partialClosesLastHour = 0
		ga.failedOrdersLastHour = 0
		ga.mu.Unlock()

		ga.logger.Info("Ginie autopilot hourly profit booking metrics reset")
	}
}

// GetStats returns current performance statistics
func (ga *GinieAutopilot) GetStats() map[string]interface{} {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	winRate := 0.0
	if ga.totalTrades > 0 {
		winRate = float64(ga.winningTrades) / float64(ga.totalTrades) * 100
	}

	// Calculate total unrealized PnL from all open positions
	unrealizedPnL := 0.0
	for _, pos := range ga.positions {
		unrealizedPnL += pos.UnrealizedPnL + pos.RealizedPnL
	}

	return map[string]interface{}{
		"running":          ga.running,
		"dry_run":          ga.config.DryRun,
		"total_trades":     ga.totalTrades,
		"winning_trades":   ga.winningTrades,
		"win_rate":         winRate,
		"total_pnl":        ga.totalPnL,
		"daily_trades":     ga.dailyTrades,
		"daily_pnl":        ga.dailyPnL,
		"unrealized_pnl":   unrealizedPnL,
		"combined_pnl":     ga.dailyPnL + unrealizedPnL, // Daily realized + current unrealized
		"active_positions": len(ga.positions),
		"max_positions":    ga.config.MaxPositions,
	}
}

// ========== Circuit Breaker Methods (Ginie-specific) ==========

// GetCircuitBreakerStatus returns the current circuit breaker status
func (ga *GinieAutopilot) GetCircuitBreakerStatus() map[string]interface{} {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	if ga.circuitBreaker == nil {
		return map[string]interface{}{
			"enabled": false,
			"error":   "circuit breaker not initialized",
		}
	}

	stats := ga.circuitBreaker.GetStats()
	canTrade, reason := ga.circuitBreaker.CanTrade()

	return map[string]interface{}{
		"enabled":             ga.config.CircuitBreakerEnabled,
		"can_trade":           canTrade,
		"block_reason":        reason,
		"state":               stats["state"],
		"hourly_loss":         stats["hourly_loss"],
		"daily_loss":          stats["daily_loss"],
		"consecutive_losses":  stats["consecutive_losses"],
		"trades_last_minute":  stats["trades_last_minute"],
		"daily_trades":        stats["daily_trades"],
		"trip_reason":         stats["trip_reason"],
		"last_trip_time":      stats["last_trip_time"],
		"max_loss_per_hour":   ga.config.CBMaxLossPerHour,
		"max_daily_loss":      ga.config.CBMaxDailyLoss,
		"max_consecutive":     ga.config.CBMaxConsecutiveLosses,
		"cooldown_minutes":    ga.config.CBCooldownMinutes,
	}
}

// ResetCircuitBreaker resets the circuit breaker
func (ga *GinieAutopilot) ResetCircuitBreaker() error {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if ga.circuitBreaker == nil {
		return fmt.Errorf("circuit breaker not initialized")
	}

	ga.circuitBreaker.ForceReset()
	ga.logger.Info("Ginie circuit breaker reset manually")
	return nil
}

// SetCircuitBreakerEnabled enables or disables the circuit breaker
func (ga *GinieAutopilot) SetCircuitBreakerEnabled(enabled bool) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	ga.config.CircuitBreakerEnabled = enabled
	ga.logger.Info("Ginie circuit breaker enabled state changed", "enabled", enabled)
}

// UpdateCircuitBreakerConfig updates circuit breaker configuration
func (ga *GinieAutopilot) UpdateCircuitBreakerConfig(maxLossPerHour, maxDailyLoss float64, maxConsecutiveLosses, cooldownMinutes int) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	ga.config.CBMaxLossPerHour = maxLossPerHour
	ga.config.CBMaxDailyLoss = maxDailyLoss
	ga.config.CBMaxConsecutiveLosses = maxConsecutiveLosses
	ga.config.CBCooldownMinutes = cooldownMinutes

	// Update the circuit breaker config using its UpdateConfig method
	if ga.circuitBreaker != nil {
		ga.circuitBreaker.UpdateConfig(&circuit.CircuitBreakerConfig{
			Enabled:              ga.config.CircuitBreakerEnabled,
			MaxLossPerHour:       maxLossPerHour,
			MaxDailyLoss:         maxDailyLoss,
			MaxConsecutiveLosses: maxConsecutiveLosses,
			CooldownMinutes:      cooldownMinutes,
		})
	}

	ga.logger.Info("Ginie circuit breaker config updated",
		"max_loss_per_hour", maxLossPerHour,
		"max_daily_loss", maxDailyLoss,
		"max_consecutive_losses", maxConsecutiveLosses,
		"cooldown_minutes", cooldownMinutes)
}

// ForceSyncWithExchange completely clears all Ginie positions and reimports from exchange
// This is useful when positions have gotten out of sync (e.g., after switching modes)
// If client is provided, it uses that client instead of the internal one
func (ga *GinieAutopilot) ForceSyncWithExchange(client ...binance.FuturesClient) (int, error) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	// Use provided client if available, otherwise use internal one
	activeClient := ga.futuresClient
	if len(client) > 0 && client[0] != nil {
		activeClient = client[0]
		ga.futuresClient = client[0] // Also update internal client
		ga.logger.Info("Using provided client for force sync")
	}

	// Get all open positions from exchange
	positions, err := activeClient.GetPositions()
	if err != nil {
		return 0, fmt.Errorf("failed to get exchange positions: %w", err)
	}

	// Clear all existing Ginie positions
	oldCount := len(ga.positions)
	ga.positions = make(map[string]*GiniePosition)
	ga.logger.Info("Cleared all Ginie positions for force sync", "old_count", oldCount)

	// Import all positions from exchange
	synced := 0
	for _, pos := range positions {
		// Skip positions with no quantity
		if pos.PositionAmt == 0 {
			continue
		}

		symbol := pos.Symbol

		// Determine side
		side := "LONG"
		if pos.PositionAmt < 0 {
			side = "SHORT"
		}

		qty := pos.PositionAmt
		if qty < 0 {
			qty = -qty
		}

		// Get current price for TP calculation
		// FIX: Do NOT use entry price as fallback - this causes 0% ROI issues
		// If price fetch fails, skip TP generation and use entry price only for tracking
		currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
		if err != nil {
			ga.logger.Error("Failed to get price during sync, using entry price for tracking only", "symbol", symbol, "error", err)
			// Use entry price for HighestPrice/LowestPrice tracking only
			// TPs will still be generated based on entry price which is correct
			currentPrice = pos.EntryPrice
		}

		// Select mode based on user's enabled modes (fixes hardcoded swing bypass)
		syncedMode := ga.selectEnabledModeForPosition()

		// Create a GiniePosition from exchange data
		position := &GiniePosition{
			Symbol:       symbol,
			Side:         side,
			Mode:         syncedMode, // Use user's enabled mode preference
			EntryPrice:   pos.EntryPrice,
			OriginalQty:  qty,
			RemainingQty: qty,
			Leverage:     pos.Leverage,
			EntryTime:    time.Now(), // We don't know actual entry time

			// Generate default TPs based on entry price
			TakeProfits:    ga.generateDefaultTPs(symbol, pos.EntryPrice, syncedMode, side == "LONG"),
			CurrentTPLevel: 0,

			// Calculate a reasonable stop loss (2% for synced positions)
			StopLoss:         ga.calculateDefaultSL(pos.EntryPrice, side == "LONG", 2.0),
			OriginalSL:       ga.calculateDefaultSL(pos.EntryPrice, side == "LONG", 2.0),
			MovedToBreakeven: false,

			// Trailing - read from Mode Config
			TrailingActive:        false,
			HighestPrice:          currentPrice,
			LowestPrice:           currentPrice,
			TrailingPercent:       ga.getTrailingPercent(syncedMode),
			TrailingActivationPct: ga.getTrailingActivation(syncedMode),

			// PnL from exchange
			UnrealizedPnL: pos.UnrealizedProfit,

			// Initialize protection tracking (will be verified by guardian)
			Protection: NewProtectionStatus(),
		}

		// Create FuturesTrade record in database for lifecycle tracking
		// IMPORTANT: Check for existing open trade first to prevent duplicates
		if ga.repo != nil {
			var tradeID int64
			var isNewTrade bool

			// Check if an open trade already exists for this symbol/user
			if ga.userID != "" {
				existingTrade, err := ga.repo.GetDB().GetOpenFuturesTradeBySymbolForUser(context.Background(), ga.userID, symbol)
				if err != nil {
					ga.logger.Warn("Failed to check for existing trade in force sync", "error", err, "symbol", symbol)
				} else if existingTrade != nil {
					// Use existing trade ID instead of creating a duplicate
					tradeID = existingTrade.ID
					isNewTrade = false
					ga.logger.Debug("Using existing open trade record for force-synced position", "symbol", symbol, "trade_id", tradeID)
				}
			}

			// Only create new trade if no existing one found
			if tradeID == 0 {
				trade := &database.FuturesTrade{
					UserID:       ga.userID, // CRITICAL: Set user ID for duplicate detection
					Symbol:       symbol,
					PositionSide: side,
					Side:         side,
					EntryPrice:   pos.EntryPrice,
					Quantity:     qty,
					Leverage:     pos.Leverage,
					MarginType:   "CROSSED",
					Status:       "OPEN",
					EntryTime:    time.Now(),
					TradeSource:  "force_sync", // Mark as force synced from exchange
				}
				if err := ga.repo.CreateFuturesTrade(context.Background(), trade); err != nil {
					ga.logger.Warn("Failed to create futures trade record for force-synced position", "error", err, "symbol", symbol)
				} else {
					tradeID = trade.ID
					isNewTrade = true
					ga.logger.Debug("Created futures trade record for force-synced position", "symbol", symbol, "trade_id", tradeID)
				}
			}

			// Set trade ID on position
			if tradeID > 0 {
				position.FuturesTradeID = tradeID

				// Log position synced event to lifecycle (only for new trades)
				if isNewTrade && ga.eventLogger != nil {
					conditionsMet := map[string]interface{}{
						"source":      "force_sync",
						"sync_reason": "manual_force_sync",
					}
					go ga.eventLogger.LogPositionOpened(
						context.Background(),
						tradeID,
						symbol,
						side,
						string(syncedMode), // Use user's enabled mode preference
						pos.EntryPrice,
						qty,
						pos.Leverage,
						0, // No confidence score for synced positions
						conditionsMet,
					)
				}
			}
		}

		ga.positions[symbol] = position
		synced++

		ga.logger.Info("Force-synced position from exchange",
			"symbol", symbol,
			"side", side,
			"qty", qty,
			"entry_price", pos.EntryPrice,
			"trade_id", position.FuturesTradeID)
	}

	ga.logger.Info("Force sync completed", "synced_count", synced)
	return synced, nil
}

// SyncWithExchange syncs Ginie tracked positions with actual exchange positions
// This is useful when server restarts or positions get lost
// IMPORTANT: This function minimizes lock holding to avoid blocking API handlers
func (ga *GinieAutopilot) SyncWithExchange() (int, error) {
	// ========== PHASE 1: Fetch exchange data OUTSIDE the lock ==========
	// Network calls can take multiple seconds - don't hold lock during this
	positions, err := ga.futuresClient.GetPositions()
	if err != nil {
		return 0, fmt.Errorf("failed to get exchange positions: %w", err)
	}

	// Build a set of symbols with open positions on exchange (pure computation, no lock needed)
	exchangePositions := make(map[string]bool)
	exchangePositionData := make(map[string]binance.FuturesPosition) // Store full data for sync
	for _, pos := range positions {
		if pos.PositionAmt != 0 {
			exchangePositions[pos.Symbol] = true
			exchangePositionData[pos.Symbol] = pos
			ga.logger.Debug("Exchange position found", "symbol", pos.Symbol, "amt", pos.PositionAmt)
		}
	}

	// ========== PHASE 2: Quick lock to identify stale and new positions ==========
	ga.mu.RLock()
	ginieCount := len(ga.positions)
	ga.logger.Info("Exchange positions check",
		"exchange_count", len(exchangePositions),
		"ginie_count", ginieCount)

	// Find stale positions (tracked by Ginie but not on exchange)
	toRemove := make([]string, 0)
	for symbol := range ga.positions {
		if !exchangePositions[symbol] {
			ga.logger.Info("Found stale position not on exchange", "symbol", symbol)
			toRemove = append(toRemove, symbol)
		}
	}

	// Find new positions (on exchange but not tracked by Ginie)
	toAdd := make([]string, 0)
	for symbol := range exchangePositions {
		if _, exists := ga.positions[symbol]; !exists {
			toAdd = append(toAdd, symbol)
		}
	}
	ga.mu.RUnlock()

	// ========== PHASE 3: Cancel algo orders for stale positions OUTSIDE lock ==========
	// This involves network calls with retry logic - must not hold lock
	removed := 0
	for _, symbol := range toRemove {
		ga.logger.Info("Cancelling orphan orders for stale position", "symbol", symbol)
		success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
		if err != nil || failed > 0 {
			ga.logger.Warn("Failed to cancel all algo orders on stale position removal",
				"symbol", symbol,
				"success", success,
				"failed", failed,
				"error", err)
		}
	}

	// ========== PHASE 4: Remove stale positions with brief lock ==========
	if len(toRemove) > 0 {
		ga.mu.Lock()
		for _, symbol := range toRemove {
			if _, exists := ga.positions[symbol]; exists {
				delete(ga.positions, symbol)
				removed++
				ga.logger.Info("Removed stale position", "symbol", symbol)
			}
		}
		ga.mu.Unlock()
		if removed > 0 {
			ga.logger.Info("Cleaned up stale positions", "removed_count", removed)
		}
	}

	// ========== PHASE 5: Fetch prices for new positions OUTSIDE lock ==========
	// Network calls for prices - can take several seconds
	type syncData struct {
		symbol       string
		pos          binance.FuturesPosition
		currentPrice float64
	}
	toSync := make([]syncData, 0, len(toAdd))

	for _, symbol := range toAdd {
		pos, exists := exchangePositionData[symbol]
		if !exists {
			continue
		}

		// Fetch current price outside the lock
		currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
		if err != nil {
			ga.logger.Error("Failed to get price during sync", "symbol", symbol, "error", err)
			continue
		}

		toSync = append(toSync, syncData{
			symbol:       symbol,
			pos:          pos,
			currentPrice: currentPrice,
		})
	}

	// ========== PHASE 6: Create positions and add to map WITH proper locking ==========
	synced := 0
	for _, data := range toSync {
		symbol := data.symbol
		pos := data.pos
		currentPrice := data.currentPrice

		// Determine side
		side := "LONG"
		if pos.PositionAmt < 0 {
			side = "SHORT"
		}

		qty := pos.PositionAmt
		if qty < 0 {
			qty = -qty
		}

		// Select mode based on user's enabled modes (fixes hardcoded swing bypass)
		syncMode := ga.selectEnabledModeForPosition()

		// Create a basic GiniePosition from exchange data
		// Note: This won't have the original decision info, but allows position monitoring
		position := &GiniePosition{
			Symbol:       symbol,
			Side:         side,
			Mode:         syncMode, // Use user's enabled mode preference
			EntryPrice:   pos.EntryPrice,
			OriginalQty:  qty,
			RemainingQty: qty,
			Leverage:     pos.Leverage,
			EntryTime:    time.Now(), // We don't know actual entry time

			// Generate default TPs based on entry price
			TakeProfits:    ga.generateDefaultTPs(symbol, pos.EntryPrice, syncMode, side == "LONG"),
			CurrentTPLevel: 0,

			// Calculate a reasonable stop loss (2% for synced positions)
			StopLoss:         ga.calculateDefaultSL(pos.EntryPrice, side == "LONG", 2.0),
			OriginalSL:       ga.calculateDefaultSL(pos.EntryPrice, side == "LONG", 2.0),
			MovedToBreakeven: false,

			// Trailing - read from Mode Config
			TrailingActive:        false,
			HighestPrice:          currentPrice,
			LowestPrice:           currentPrice,
			TrailingPercent:       ga.getTrailingPercent(syncMode),
			TrailingActivationPct: ga.getTrailingActivation(syncMode),

			// PnL from exchange
			UnrealizedPnL: pos.UnrealizedProfit,

			// Initialize protection tracking (will be verified by guardian)
			Protection: NewProtectionStatus(),
		}

		// Create FuturesTrade record in database for lifecycle tracking (outside lock)
		// IMPORTANT: Check for existing open trade first to prevent duplicates
		if ga.repo != nil {
			var tradeID int64
			var isNewTrade bool

			// Check if an open trade already exists for this symbol/user
			if ga.userID != "" {
				existingTrade, err := ga.repo.GetDB().GetOpenFuturesTradeBySymbolForUser(context.Background(), ga.userID, symbol)
				if err != nil {
					ga.logger.Warn("Failed to check for existing trade", "error", err, "symbol", symbol)
				} else if existingTrade != nil {
					// Use existing trade ID instead of creating a duplicate
					tradeID = existingTrade.ID
					isNewTrade = false
					ga.logger.Debug("Using existing open trade record for synced position", "symbol", symbol, "trade_id", tradeID)
				}
			}

			// Only create new trade if no existing one found
			if tradeID == 0 {
				trade := &database.FuturesTrade{
					UserID:       ga.userID, // CRITICAL: Set user ID for duplicate detection
					Symbol:       symbol,
					PositionSide: side,
					Side:         side,
					EntryPrice:   pos.EntryPrice,
					Quantity:     qty,
					Leverage:     pos.Leverage,
					MarginType:   "CROSSED",
					Status:       "OPEN",
					EntryTime:    time.Now(),
					TradeSource:  "sync", // Mark as synced from exchange
				}
				if err := ga.repo.CreateFuturesTrade(context.Background(), trade); err != nil {
					ga.logger.Warn("Failed to create futures trade record for synced position", "error", err, "symbol", symbol)
				} else {
					tradeID = trade.ID
					isNewTrade = true
					ga.logger.Debug("Created futures trade record for synced position", "symbol", symbol, "trade_id", tradeID)
				}
			}

			// Set trade ID on position
			if tradeID > 0 {
				position.FuturesTradeID = tradeID

				// Log position synced event to lifecycle (only for new trades)
				if isNewTrade && ga.eventLogger != nil {
					conditionsMet := map[string]interface{}{
						"source":      "exchange_sync",
						"sync_reason": "app_restart_or_manual_position",
					}
					go ga.eventLogger.LogPositionOpened(
						context.Background(),
						tradeID,
						symbol,
						side,
						string(GinieModeSwing), // Synced positions default to swing mode
						pos.EntryPrice,
						qty,
						pos.Leverage,
						0, // No confidence score for synced positions
						conditionsMet,
					)
				}
			}
		}

		// Add to positions map with brief lock
		ga.mu.Lock()
		// Double-check position wasn't added by another goroutine
		if _, exists := ga.positions[symbol]; !exists {
			ga.positions[symbol] = position
			synced++
			ga.logger.Info("Synced position from exchange",
				"symbol", symbol,
				"side", side,
				"qty", qty,
				"entry_price", pos.EntryPrice,
				"unrealized_pnl", pos.UnrealizedProfit,
				"trade_id", position.FuturesTradeID)
		}
		ga.mu.Unlock()
	}

	if synced > 0 {
		ga.logger.Info("Position sync completed", "synced_count", synced)
	}

	return synced, nil
}

// ==================== ADAPTIVE SL/TP WITH LLM ====================

// placeSLTPOrders places stop loss and take profit orders on Binance
func (ga *GinieAutopilot) placeSLTPOrders(pos *GiniePosition) {
	if pos == nil || pos.StopLoss <= 0 {
		ga.logger.Warn("Cannot place SL/TP orders - invalid position or SL", "symbol", pos.Symbol)
		return
	}

	// CRITICAL: Cancel ALL existing algo orders for this symbol FIRST
	// This prevents accumulation of orphan orders when updating SL/TP
	log.Printf("[GINIE] %s: Cancelling existing algo orders before placing new SL/TP", pos.Symbol)
	success, failed, err := ga.cancelAllAlgoOrdersForSymbol(pos.Symbol)
	if err != nil || failed > 0 {
		ga.logger.Warn("Failed to cancel all algo orders in placeSLTPOrders",
			"symbol", pos.Symbol,
			"success", success,
			"failed", failed,
			"error", err)
	}

	// Clear stored algo IDs since we cancelled all orders
	pos.StopLossAlgoID = 0
	pos.TakeProfitAlgoIDs = nil

	// Determine the side for closing orders
	closeSide := "SELL"
	positionSide := binance.PositionSideLong
	if pos.Side == "SHORT" {
		closeSide = "BUY"
		positionSide = binance.PositionSideShort
	}

	// Check actual Binance position mode to avoid API error -4061
	effectivePositionSide := ga.getEffectivePositionSide(positionSide)

	// Round SL price with directional rounding to ensure trigger protects capital
	roundedSL := roundPriceForSL(pos.Symbol, pos.StopLoss, pos.Side)
	roundedQty := roundQuantity(pos.Symbol, pos.RemainingQty)

	// Place Stop Loss order using Algo Order API (STOP_MARKET requires Algo API)
	// Note: Don't set ReduceOnly - in Hedge mode, positionSide determines direction
	slParams := binance.AlgoOrderParams{
		Symbol:       pos.Symbol,
		Side:         closeSide,
		PositionSide: effectivePositionSide,
		Type:         binance.FuturesOrderTypeStopMarket,
		Quantity:     roundedQty,
		TriggerPrice: roundedSL,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	// Place SL with retry logic - CRITICAL for position protection
	const maxSLRetries = 3
	slRetryDelay := 500 * time.Millisecond
	var slOrderPlaced bool

	for attempt := 1; attempt <= maxSLRetries; attempt++ {
		slOrder, err := ga.futuresClient.PlaceAlgoOrder(slParams)
		if err == nil && slOrder != nil && slOrder.AlgoId > 0 {
			pos.StopLossAlgoID = slOrder.AlgoId
			ga.logger.Info("Stop loss order placed",
				"symbol", pos.Symbol,
				"algo_id", slOrder.AlgoId,
				"trigger_price", roundedSL,
				"attempt", attempt)
			slOrderPlaced = true
			break
		}
		ga.logger.Error("Failed to place stop loss order",
			"symbol", pos.Symbol,
			"sl_price", roundedSL,
			"attempt", attempt,
			"max_retries", maxSLRetries,
			"error", err.Error())
		if attempt < maxSLRetries {
			time.Sleep(slRetryDelay * time.Duration(attempt))
		}
	}

	if !slOrderPlaced {
		ga.logger.Error("CRITICAL: Stop loss order NOT placed after all retries - position unprotected!",
			"symbol", pos.Symbol,
			"sl_price", roundedSL)
	}

	// Place Take Profit orders for each level (only TP1 initially, others placed as we hit levels)
	// For now, place TP1 as the first target
	if len(pos.TakeProfits) > 0 && pos.TakeProfits[0].Price > 0 {
		tp1 := pos.TakeProfits[0]
		tp1Qty := roundQuantity(pos.Symbol, pos.OriginalQty*(tp1.Percent/100.0))
		// Round TP price with directional rounding to ensure trigger fires
		roundedTP1 := roundPriceForTP(pos.Symbol, tp1.Price, pos.Side)

		if tp1Qty > 0 {
			// Get current price to check if TP is already passed
			currentPrice, priceErr := ga.futuresClient.GetFuturesCurrentPrice(pos.Symbol)
			if priceErr != nil {
				ga.logger.Warn("Failed to get current price for TP check", "symbol", pos.Symbol, "error", priceErr)
				currentPrice = 0
			}

			// Check if price already passed TP1 - execute immediately with market order
			tpAlreadyPassed := false
			if currentPrice > 0 {
				if pos.Side == "LONG" && currentPrice >= roundedTP1 {
					tpAlreadyPassed = true
				} else if pos.Side == "SHORT" && currentPrice <= roundedTP1 {
					tpAlreadyPassed = true
				}
			}

			if tpAlreadyPassed {
				// Price already passed TP1 - execute market order immediately
				log.Printf("[GINIE] %s: TP1 already passed (price=%.4f, tp1=%.4f), executing market order", pos.Symbol, currentPrice, roundedTP1)
				ga.logger.Info("TP1 already passed - executing market order immediately",
					"symbol", pos.Symbol,
					"current_price", currentPrice,
					"tp1_price", roundedTP1,
					"quantity", tp1Qty)

				// Place market order to close TP1 portion
				orderParams := binance.FuturesOrderParams{
					Symbol:       pos.Symbol,
					Side:         closeSide,
					PositionSide: effectivePositionSide,
					Type:         binance.FuturesOrderTypeMarket,
					Quantity:     tp1Qty,
				}

				order, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
				if err != nil {
					ga.logger.Error("Failed to execute immediate TP1 market order",
						"symbol", pos.Symbol,
						"error", err.Error())
				} else {
					// Calculate and record PnL
					var pnl float64
					if pos.Side == "LONG" {
						pnl = (currentPrice - pos.EntryPrice) * tp1Qty
					} else {
						pnl = (pos.EntryPrice - currentPrice) * tp1Qty
					}

					pos.TakeProfits[0].Status = "hit"
					pos.CurrentTPLevel = 1
					pos.RemainingQty -= tp1Qty
					pos.RealizedPnL += pnl
					ga.dailyPnL += pnl
					ga.totalPnL += pnl

					// Move to breakeven after TP1
					if ga.config.MoveToBreakevenAfterTP1 && !pos.MovedToBreakeven {
						ga.moveToBreakeven(pos)
						ga.updateBinanceSLOrder(pos)
					}

					ga.logger.Info("Immediate TP1 executed successfully",
						"symbol", pos.Symbol,
						"order_id", order.OrderId,
						"executed_qty", tp1Qty,
						"pnl", pnl)

					// Place TP2 order
					if len(pos.TakeProfits) > 1 {
						ga.placeNextTPOrder(pos, 1)
					}
				}
			} else {
				// Normal case - price hasn't reached TP1, place algo order
				tpParams := binance.AlgoOrderParams{
					Symbol:       pos.Symbol,
					Side:         closeSide,
					PositionSide: effectivePositionSide,
					Type:         binance.FuturesOrderTypeTakeProfitMarket,
					Quantity:     tp1Qty,
					TriggerPrice: roundedTP1,
					WorkingType:  binance.WorkingTypeMarkPrice,
				}

				// Place TP with retry logic - CRITICAL for profit protection
				const maxTPRetries = 3
				tpRetryDelay := 500 * time.Millisecond
				var tpOrderPlaced bool

				for attempt := 1; attempt <= maxTPRetries; attempt++ {
					tpOrder, err := ga.futuresClient.PlaceAlgoOrder(tpParams)
					if err == nil && tpOrder != nil && tpOrder.AlgoId > 0 {
						pos.TakeProfitAlgoIDs = append(pos.TakeProfitAlgoIDs, tpOrder.AlgoId)
						ga.logger.Info("Take profit order placed",
							"symbol", pos.Symbol,
							"tp_level", 1,
							"algo_id", tpOrder.AlgoId,
							"trigger_price", roundedTP1,
							"quantity", tp1Qty,
							"attempt", attempt)
						tpOrderPlaced = true
						break
					}
					ga.logger.Error("Failed to place take profit order",
						"symbol", pos.Symbol,
						"tp_level", 1,
						"tp_price", tp1.Price,
						"attempt", attempt,
						"max_retries", maxTPRetries,
						"error", err.Error())
					if attempt < maxTPRetries {
						time.Sleep(tpRetryDelay * time.Duration(attempt))
					}
				}

				if !tpOrderPlaced {
					ga.logger.Error("CRITICAL: Take profit order NOT placed after all retries - no profit protection!",
						"symbol", pos.Symbol,
						"tp_price", roundedTP1)
				}
			}
		}
	}

	pos.LastLLMUpdate = time.Now()

	// Log SL/TP placed event to trade lifecycle
	if ga.eventLogger != nil && pos.FuturesTradeID > 0 && slOrderPlaced {
		tpLevels := make([]float64, len(pos.TakeProfits))
		for i, tp := range pos.TakeProfits {
			tpLevels[i] = tp.Price
		}
		go ga.eventLogger.LogSLTPPlaced(
			context.Background(),
			pos.FuturesTradeID,
			pos.Symbol,
			string(pos.Mode),
			roundedSL,
			tpLevels,
		)
	}
}

// placeSLTPOrdersForSyncedPositions places SL/TP orders for all synced positions
// This is called on startup after syncing positions from the exchange
func (ga *GinieAutopilot) placeSLTPOrdersForSyncedPositions() {
	ga.mu.RLock()
	positions := make([]*GiniePosition, 0, len(ga.positions))
	for _, pos := range ga.positions {
		positions = append(positions, pos)
	}
	ga.mu.RUnlock()

	if len(positions) == 0 {
		return
	}

	ga.logger.Info("Placing SL/TP orders for synced positions", "count", len(positions))

	for _, pos := range positions {
		// ALWAYS cancel existing algo orders from Binance first (from previous sessions)
		success, failed, err := ga.cancelAllAlgoOrdersForSymbol(pos.Symbol)
		if err != nil {
			ga.logger.Warn("Failed to fully cancel existing algo orders",
				"symbol", pos.Symbol,
				"successful", success,
				"failed", failed,
				"error", err)
		} else if success > 0 {
			ga.logger.Info("Cleaned up existing algo orders from previous session",
				"symbol", pos.Symbol,
				"cancelled_count", success)
		}

		// Ensure position has valid SL/TP levels
		if pos.StopLoss <= 0 {
			ga.logger.Warn("Skipping position - no stop loss set",
				"symbol", pos.Symbol)
			continue
		}

		ga.placeSLTPOrders(pos)

		// Small delay between API calls to avoid rate limits
		time.Sleep(100 * time.Millisecond)
	}

	ga.logger.Info("Finished placing SL/TP orders for synced positions")
}

// ensureSLTPOrdersExist checks if SLTP orders exist on Binance for a position
// and recreates them if missing (e.g., user manually deleted them)
func (ga *GinieAutopilot) ensureSLTPOrdersExist(symbol string, pos *GiniePosition) {
	if pos == nil || pos.StopLoss <= 0 {
		return
	}

	// Get all algo orders for this symbol from Binance
	algoOrders, err := ga.futuresClient.GetOpenAlgoOrders(symbol)
	if err != nil {
		ga.logger.Debug("Failed to get algo orders for SLTP check",
			"symbol", symbol,
			"error", err)
		return
	}

	// Check for existing SL and TP orders
	hasSL := false
	hasTP := false

	expectedPosSide := pos.Side
	for _, order := range algoOrders {
		if order.PositionSide != expectedPosSide {
			continue
		}

		// Check if it's a SL order
		if order.OrderType == "STOP_MARKET" || order.OrderType == "STOP" {
			hasSL = true
		}

		// Check if it's a TP order
		if order.OrderType == "TAKE_PROFIT_MARKET" || order.OrderType == "TAKE_PROFIT" {
			hasTP = true
		}
	}

	// If both exist, nothing to do
	if hasSL && hasTP {
		return
	}

	// Missing orders detected - recreate them
	if !hasSL || !hasTP {
		log.Printf("[SLTP-RECONCILE] %s: Missing orders detected (SL=%v, TP=%v) - recreating", symbol, hasSL, hasTP)
		ga.logger.Warn("Missing SLTP orders detected - recreating",
			"symbol", symbol,
			"has_sl", hasSL,
			"has_tp", hasTP,
			"position_side", pos.Side)

		// Cancel any partial orders first, then recreate all
		successCount, failureCount, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
		if err != nil {
			ga.logger.Error("Failed to cancel existing algo orders before recreating SLTP",
				"symbol", symbol,
				"error", err)
			return // Don't proceed if cancellation failed
		}
		if failureCount > 0 {
			ga.logger.Warn("Some algo orders failed to cancel",
				"symbol", symbol,
				"success_count", successCount,
				"failure_count", failureCount)
		}

		// Wait for cancellation to propagate on exchange
		time.Sleep(200 * time.Millisecond)

		// Recreate SLTP orders
		ga.placeSLTPOrders(pos)
	}
}

// ==================== BULLETPROOF POSITION PROTECTION SYSTEM ====================

// verifySLOrderExists checks if a Stop Loss order exists on Binance for the position
// Returns true if SL order found and verified, false otherwise
func (ga *GinieAutopilot) verifySLOrderExists(symbol string, expectedSide string) (bool, int64) {
	if ga.config.DryRun {
		return true, 0 // In dry run, assume SL exists
	}

	algoOrders, err := ga.futuresClient.GetOpenAlgoOrders(symbol)
	if err != nil {
		ga.logger.Debug("Failed to get algo orders for SL verification",
			"symbol", symbol, "error", err)
		return false, 0
	}

	for _, order := range algoOrders {
		if order.PositionSide != expectedSide {
			continue
		}
		if order.OrderType == "STOP_MARKET" || order.OrderType == "STOP" {
			return true, order.AlgoId
		}
	}

	return false, 0
}

// verifyTPOrderExists checks if a Take Profit order exists on Binance for the position
// Returns true if TP order found and verified, false otherwise
func (ga *GinieAutopilot) verifyTPOrderExists(symbol string, expectedSide string) (bool, []int64) {
	if ga.config.DryRun {
		return true, nil // In dry run, assume TP exists
	}

	algoOrders, err := ga.futuresClient.GetOpenAlgoOrders(symbol)
	if err != nil {
		ga.logger.Debug("Failed to get algo orders for TP verification",
			"symbol", symbol, "error", err)
		return false, nil
	}

	var tpOrderIDs []int64
	for _, order := range algoOrders {
		if order.PositionSide != expectedSide {
			continue
		}
		if order.OrderType == "TAKE_PROFIT_MARKET" || order.OrderType == "TAKE_PROFIT" {
			tpOrderIDs = append(tpOrderIDs, order.AlgoId)
		}
	}

	return len(tpOrderIDs) > 0, tpOrderIDs
}

// verifyPositionProtection checks if position has valid SL/TP orders on Binance
// and updates the protection status accordingly
func (ga *GinieAutopilot) verifyPositionProtection(pos *GiniePosition) {
	if pos == nil {
		return
	}

	// Initialize protection status if nil
	if pos.Protection == nil {
		pos.Protection = NewProtectionStatus()
	}

	// Verify SL exists on Binance
	slExists, slOrderID := ga.verifySLOrderExists(pos.Symbol, pos.Side)
	if slExists {
		pos.Protection.SLOrderID = slOrderID
		pos.Protection.SLVerified = true
		pos.Protection.SLVerifiedAt = time.Now()
	} else {
		pos.Protection.SLVerified = false
	}

	// Verify TP exists on Binance
	tpExists, tpOrderIDs := ga.verifyTPOrderExists(pos.Symbol, pos.Side)
	if tpExists {
		pos.Protection.TPOrderIDs = tpOrderIDs
		pos.Protection.TPVerified = true
		pos.Protection.TPVerifiedAt = time.Now()
	} else {
		pos.Protection.TPVerified = false
	}

	// Update protection state based on verification
	if pos.Protection.SLVerified && pos.Protection.TPVerified {
		if pos.Protection.State != StateFullyProtected {
			pos.Protection.SetState(StateFullyProtected)
			log.Printf("[PROTECTION] %s: Position FULLY PROTECTED (SL+TP verified)", pos.Symbol)
		}
	} else if pos.Protection.SLVerified {
		if pos.Protection.State != StateSLVerified && pos.Protection.State != StateFullyProtected {
			pos.Protection.SetState(StateSLVerified)
			log.Printf("[PROTECTION] %s: SL VERIFIED (TP missing)", pos.Symbol)
		}
	} else {
		// No SL = UNPROTECTED
		if pos.Protection.State != StateUnprotected && pos.Protection.State != StateHealing && pos.Protection.State != StateEmergencyClose {
			pos.Protection.SetState(StateUnprotected)
			log.Printf("[PROTECTION] %s: UNPROTECTED - SL missing!", pos.Symbol)
		}
	}
}

// healPosition attempts to repair SL/TP orders for an unprotected position
func (ga *GinieAutopilot) healPosition(pos *GiniePosition) {
	if pos == nil || pos.Protection == nil {
		return
	}

	// Don't heal if already protected or in emergency close
	if pos.Protection.State == StateFullyProtected || pos.Protection.State == StateEmergencyClose {
		return
	}

	pos.Protection.SetState(StateHealing)
	pos.Protection.HealAttempts++

	log.Printf("[PROTECTION-HEAL] %s: Attempting heal (attempt #%d)", pos.Symbol, pos.Protection.HealAttempts)

	// Cancel any orphan orders and recreate
	_, _, err := ga.cancelAllAlgoOrdersForSymbol(pos.Symbol)
	if err != nil {
		log.Printf("[PROTECTION-HEAL] %s: Failed to cancel existing orders: %v", pos.Symbol, err)
		pos.Protection.LastFailure = fmt.Sprintf("cancel failed: %v", err)
		pos.Protection.FailureCount++
		pos.Protection.SetState(StateUnprotected)
		return
	}

	// Wait for cancellation to propagate
	time.Sleep(300 * time.Millisecond)

	// Recreate SL/TP orders
	ga.placeSLTPOrders(pos)

	// Wait for orders to be placed
	time.Sleep(500 * time.Millisecond)

	// Verify the orders were created
	ga.verifyPositionProtection(pos)

	if pos.Protection.SLVerified {
		log.Printf("[PROTECTION-HEAL] %s: Heal SUCCESSFUL - SL verified", pos.Symbol)
		pos.Protection.HealAttempts = 0 // Reset on success
	} else {
		log.Printf("[PROTECTION-HEAL] %s: Heal FAILED - SL still not verified", pos.Symbol)
		pos.Protection.FailureCount++
		pos.Protection.LastFailure = "SL placement failed after heal attempt"
		pos.Protection.SetState(StateUnprotected)
	}
}

// emergencyClosePosition closes a position immediately due to protection failure
func (ga *GinieAutopilot) emergencyClosePosition(pos *GiniePosition, reason string) error {
	if pos == nil {
		return fmt.Errorf("nil position")
	}

	pos.Protection.SetState(StateEmergencyClose)

	log.Printf("[EMERGENCY-CLOSE] %s: CLOSING POSITION - Reason: %s", pos.Symbol, reason)
	ga.logger.Error("Emergency position close triggered",
		"symbol", pos.Symbol,
		"reason", reason,
		"protection_failures", pos.Protection.FailureCount,
		"heal_attempts", pos.Protection.HealAttempts,
		"unprotected_duration", pos.Protection.TimeSinceStateChange())

	// Close at market
	return ga.closePositionAtMarket(pos)
}

// runProtectionGuardian runs a continuous loop that monitors and heals position protection
// This is the core of the bulletproof SL/TP system
func (ga *GinieAutopilot) runProtectionGuardian() {
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	log.Printf("[PROTECTION-GUARDIAN] Starting position protection guardian (5s interval)")

	for {
		select {
		case <-ga.stopChan:
			log.Printf("[PROTECTION-GUARDIAN] Stopping protection guardian")
			return
		case <-ticker.C:
			ga.checkAllPositionsProtection()
		}
	}
}

// checkAllPositionsProtection verifies protection for all active positions
func (ga *GinieAutopilot) checkAllPositionsProtection() {
	ga.mu.RLock()
	positions := make([]*GiniePosition, 0, len(ga.positions))
	for _, pos := range ga.positions {
		positions = append(positions, pos)
	}
	ga.mu.RUnlock()

	for _, pos := range positions {
		ga.checkSinglePositionProtection(pos)
	}
}

// checkSinglePositionProtection checks and handles protection for a single position
func (ga *GinieAutopilot) checkSinglePositionProtection(pos *GiniePosition) {
	if pos == nil {
		return
	}

	// Initialize protection if nil
	if pos.Protection == nil {
		pos.Protection = NewProtectionStatus()
		pos.Protection.SetState(StateOpening)
	}

	// Skip if in emergency close
	if pos.Protection.State == StateEmergencyClose {
		return
	}

	// Verify current protection status
	ga.verifyPositionProtection(pos)

	// Handle unprotected positions
	if pos.Protection.State == StateUnprotected {
		unprotectedDuration := pos.Protection.TimeSinceStateChange()

		// Configuration: Max unprotected time before emergency close
		const maxUnprotectedTime = 30 * time.Second
		const maxHealAttempts = 3

		if unprotectedDuration > maxUnprotectedTime || pos.Protection.HealAttempts >= maxHealAttempts {
			// EMERGENCY: Position has been unprotected too long or too many heal attempts
			reason := fmt.Sprintf("Unprotected for %v, heal attempts: %d", unprotectedDuration.Round(time.Second), pos.Protection.HealAttempts)
			ga.emergencyClosePosition(pos, reason)
			return
		}

		// Try to heal (only if not already healing)
		if pos.Protection.State != StateHealing {
			ga.healPosition(pos)
		}
	}

	// Handle partially protected (SL only, no TP)
	if pos.Protection.State == StateSLVerified && !pos.Protection.TPVerified {
		// TP missing but SL in place - try to add TP without canceling SL
		if pos.Protection.TimeSinceStateChange() > 10*time.Second {
			log.Printf("[PROTECTION] %s: TP missing for %v, attempting to add TP", pos.Symbol, pos.Protection.TimeSinceStateChange().Round(time.Second))
			// Only place TP, don't cancel SL
			ga.placeTPOrderOnly(pos)
		}
	}
}

// placeTPOrderOnly places only the TP order without touching SL
// Used when SL is verified but TP is missing
func (ga *GinieAutopilot) placeTPOrderOnly(pos *GiniePosition) {
	if pos == nil {
		log.Printf("[PROTECTION-TP] placeTPOrderOnly called with nil position")
		return
	}
	if len(pos.TakeProfits) == 0 {
		log.Printf("[PROTECTION-TP] %s: No TakeProfits defined (len=0), cannot place TP", pos.Symbol)
		return
	}

	// Determine the current TP level to place
	tpLevel := pos.CurrentTPLevel
	if tpLevel >= len(pos.TakeProfits) {
		log.Printf("[PROTECTION-TP] %s: All TPs already hit (level=%d, total=%d)", pos.Symbol, tpLevel, len(pos.TakeProfits))
		return // All TPs already hit
	}

	tp := pos.TakeProfits[tpLevel]
	if tp.Price <= 0 {
		log.Printf("[PROTECTION-TP] %s: TP price is invalid (price=%.8f)", pos.Symbol, tp.Price)
		return
	}
	if tp.Status == "hit" {
		log.Printf("[PROTECTION-TP] %s: TP level %d already hit", pos.Symbol, tpLevel)
		return
	}

	closeSide := "SELL"
	positionSide := binance.PositionSideLong
	if pos.Side == "SHORT" {
		closeSide = "BUY"
		positionSide = binance.PositionSideShort
	}

	// CRITICAL FIX: Check actual Binance position mode to avoid API error -4061
	effectivePositionSide := ga.getEffectivePositionSide(positionSide)

	// Determine if this is the final TP level - use ClosePosition=true to avoid residual
	isFinalTPLevel := tpLevel >= len(pos.TakeProfits)-1

	roundedTP := roundPriceForTP(pos.Symbol, tp.Price, pos.Side)

	var tpParams binance.AlgoOrderParams

	if isFinalTPLevel {
		// CRITICAL FIX: For final TP level, use ClosePosition=true to close entire remaining position
		// This prevents residual quantity issues from rounding mismatches
		log.Printf("[PROTECTION-TP] %s: Final TP level %d - using ClosePosition=true",
			pos.Symbol, tpLevel)
		tpParams = binance.AlgoOrderParams{
			Symbol:        pos.Symbol,
			Side:          closeSide,
			PositionSide:  effectivePositionSide,
			Type:          binance.FuturesOrderTypeTakeProfitMarket,
			ClosePosition: true, // Close entire remaining position
			TriggerPrice:  roundedTP,
			WorkingType:   binance.WorkingTypeMarkPrice,
		}
	} else {
		// For intermediate TP levels (TP1-3), calculate quantity for partial close
		tpQty := roundQuantity(pos.Symbol, pos.RemainingQty*(tp.Percent/100.0))
		if tpQty <= 0 {
			// For small positions, use full remaining quantity (converts to single TP mode)
			tpQty = roundQuantity(pos.Symbol, pos.RemainingQty)
			if tpQty <= 0 {
				log.Printf("[PROTECTION-TP] %s: Even full quantity rounds to 0 (remainingQty=%.8f)", pos.Symbol, pos.RemainingQty)
				return
			}
			log.Printf("[PROTECTION-TP] %s: Using full remaining qty for TP (small position, %.8f -> %.8f)",
				pos.Symbol, pos.RemainingQty, tpQty)
		}

		log.Printf("[PROTECTION-TP] %s: Placing TP order (level=%d, price=%.8f, qty=%.8f, side=%s)",
			pos.Symbol, tpLevel, tp.Price, tpQty, closeSide)

		tpParams = binance.AlgoOrderParams{
			Symbol:       pos.Symbol,
			Side:         closeSide,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeTakeProfitMarket,
			Quantity:     tpQty,
			TriggerPrice: roundedTP,
			WorkingType:  binance.WorkingTypeMarkPrice,
		}
	}

	tpOrder, err := ga.futuresClient.PlaceAlgoOrder(tpParams)
	if err == nil && tpOrder != nil && tpOrder.AlgoId > 0 {
		pos.TakeProfitAlgoIDs = append(pos.TakeProfitAlgoIDs, tpOrder.AlgoId)
		pos.Protection.TPOrderIDs = append(pos.Protection.TPOrderIDs, tpOrder.AlgoId)
		pos.Protection.TPVerified = true
		pos.Protection.TPVerifiedAt = time.Now()
		pos.Protection.SetState(StateFullyProtected)
		log.Printf("[PROTECTION] %s: TP order placed successfully (algoID: %d)", pos.Symbol, tpOrder.AlgoId)
	} else {
		log.Printf("[PROTECTION] %s: Failed to place TP order: %v", pos.Symbol, err)
	}
}

// initializePositionProtection initializes protection tracking for a new position
func (ga *GinieAutopilot) initializePositionProtection(pos *GiniePosition) {
	if pos == nil {
		return
	}
	pos.Protection = NewProtectionStatus()
	pos.Protection.SetState(StateOpening)
}

// GetPositionProtectionStatus returns protection status for all positions (for API/UI)
func (ga *GinieAutopilot) GetPositionProtectionStatus() []map[string]interface{} {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(ga.positions))
	for symbol, pos := range ga.positions {
		status := map[string]interface{}{
			"symbol":     symbol,
			"side":       pos.Side,
			"entry_time": pos.EntryTime,
		}

		if pos.Protection != nil {
			status["protection_state"] = string(pos.Protection.State)
			status["sl_verified"] = pos.Protection.SLVerified
			status["tp_verified"] = pos.Protection.TPVerified
			status["failure_count"] = pos.Protection.FailureCount
			status["heal_attempts"] = pos.Protection.HealAttempts
			status["last_failure"] = pos.Protection.LastFailure
			status["time_in_state"] = pos.Protection.TimeSinceStateChange().String()
			status["is_protected"] = pos.Protection.IsProtected()
		} else {
			status["protection_state"] = "UNKNOWN"
			status["is_protected"] = false
		}

		result = append(result, status)
	}

	return result
}

// ==================== END BULLETPROOF POSITION PROTECTION SYSTEM ====================

// cancelAllAlgoOrdersForSymbol queries Binance for all open algo orders for a symbol and cancels them
// Returns (successCount, failureCount, error)
func (ga *GinieAutopilot) cancelAllAlgoOrdersForSymbol(symbol string) (int, int, error) {
	if ga.config.DryRun {
		return 0, 0, nil
	}

	successCount := 0
	failureCount := 0

	// Get all open algo orders from Binance
	openOrders, err := ga.futuresClient.GetOpenAlgoOrders(symbol)
	if err != nil {
		ga.logger.Warn("Failed to get open algo orders for cancellation",
			"symbol", symbol,
			"error", err)
		return 0, 0, err
	}

	if len(openOrders) == 0 {
		return 0, 0, nil
	}

	ga.logger.Info("Starting cancellation of existing algo orders",
		"symbol", symbol,
		"order_count", len(openOrders))

	// Cancel each order with retry logic
	for idx, order := range openOrders {
		cancelled := false
		var lastErr error

		// Retry up to 3 times with backoff
		for attempt := 1; attempt <= 3; attempt++ {
			err := ga.futuresClient.CancelAlgoOrder(symbol, order.AlgoId)
			if err == nil {
				cancelled = true
				successCount++
				ga.logger.Info("✓ Cancelled algo order successfully",
					"symbol", symbol,
					"order_num", fmt.Sprintf("%d/%d", idx+1, len(openOrders)),
					"algo_id", order.AlgoId,
					"order_type", order.OrderType,
					"quantity", order.Quantity,
					"attempt", attempt)
				break
			}

			lastErr = err
			if attempt < 3 {
				// Exponential backoff: 50ms, 100ms, 150ms
				backoffMs := time.Duration((attempt * 50)) * time.Millisecond
				ga.logger.Warn("✗ Failed to cancel algo order, retrying",
					"symbol", symbol,
					"order_num", fmt.Sprintf("%d/%d", idx+1, len(openOrders)),
					"algo_id", order.AlgoId,
					"attempt", attempt,
					"error", err,
					"retry_in_ms", backoffMs.Milliseconds())
				time.Sleep(backoffMs)
			}
		}

		if !cancelled {
			failureCount++
			ga.logger.Error("✗✗ Failed to cancel algo order after 3 attempts",
				"symbol", symbol,
				"order_num", fmt.Sprintf("%d/%d", idx+1, len(openOrders)),
				"algo_id", order.AlgoId,
				"order_type", order.OrderType,
				"quantity", order.Quantity,
				"trigger_price", order.TriggerPrice,
				"final_error", lastErr)
		}

		// Small delay between cancellations to avoid rate limits
		time.Sleep(50 * time.Millisecond)
	}

	ga.logger.Info("Algo order cancellation batch complete",
		"symbol", symbol,
		"total_attempted", len(openOrders),
		"successful", successCount,
		"failed", failureCount)

	if failureCount > 0 {
		return successCount, failureCount, fmt.Errorf("cancelled %d/%d orders for %s (%d failures)",
			successCount, len(openOrders), symbol, failureCount)
	}

	return successCount, failureCount, nil
}

// cleanupOrphanAlgoOrders finds and cancels algo orders that don't have corresponding positions
// This prevents orphan orders from triggering and opening unwanted positions
// ==================== POSITION RECONCILIATION ====================

// getClosingPnLFromBinance fetches actual closing trades from Binance to calculate realized PnL
// Returns: closePrice, realizedPnL, pnlPercent
func (ga *GinieAutopilot) getClosingPnLFromBinance(symbol string, pos *GiniePosition) (float64, float64, float64) {
	// Default values if we can't get trade history
	closePrice := 0.0
	realizedPnL := 0.0
	pnlPercent := 0.0

	// Fetch recent trades from Binance for this symbol
	trades, err := ga.futuresClient.GetTradeHistory(symbol, 50)
	if err != nil {
		ga.logger.Warn("Failed to fetch trade history for PnL calculation",
			"symbol", symbol,
			"error", err)
		return closePrice, realizedPnL, pnlPercent
	}

	if len(trades) == 0 {
		ga.logger.Debug("No trades found for symbol", "symbol", symbol)
		return closePrice, realizedPnL, pnlPercent
	}

	// Find closing trades (trades that reduce position)
	// For LONG positions: closing trades are SELL
	// For SHORT positions: closing trades are BUY
	closingSide := "SELL"
	if pos.Side == "SHORT" {
		closingSide = "BUY"
	}

	// Calculate time window: look for trades in the last 5 minutes
	// Position reconciliation runs every minute, so recent trades are relevant
	cutoffTime := time.Now().Add(-5 * time.Minute).UnixMilli()

	totalCloseQty := 0.0
	weightedPriceSum := 0.0

	for _, trade := range trades {
		// Check if this is a recent closing trade
		if trade.Time < cutoffTime {
			continue
		}

		// Match position side and closing direction
		expectedPosSide := pos.Side
		if trade.PositionSide != expectedPosSide {
			continue
		}

		if trade.Side != closingSide {
			continue
		}

		// This is a closing trade - accumulate PnL
		realizedPnL += trade.RealizedPnl
		totalCloseQty += trade.Qty
		weightedPriceSum += trade.Price * trade.Qty

		ga.logger.Debug("Found closing trade",
			"symbol", symbol,
			"trade_id", trade.ID,
			"side", trade.Side,
			"qty", trade.Qty,
			"price", trade.Price,
			"realized_pnl", trade.RealizedPnl)
	}

	// Calculate weighted average close price
	if totalCloseQty > 0 {
		closePrice = weightedPriceSum / totalCloseQty

		// Calculate PnL percentage based on entry price and close price
		if pos.EntryPrice > 0 {
			if pos.Side == "LONG" {
				pnlPercent = ((closePrice - pos.EntryPrice) / pos.EntryPrice) * 100
			} else {
				pnlPercent = ((pos.EntryPrice - closePrice) / pos.EntryPrice) * 100
			}
		}
	}

	ga.logger.Info("Calculated closing PnL from Binance trades",
		"symbol", symbol,
		"close_price", closePrice,
		"realized_pnl", realizedPnL,
		"pnl_percent", pnlPercent,
		"trades_found", len(trades),
		"closing_trades_qty", totalCloseQty)

	return closePrice, realizedPnL, pnlPercent
}

// reconcilePositions syncs internal position state with Binance exchange
// This handles cases where positions are closed manually or modified externally
func (ga *GinieAutopilot) reconcilePositions() {
	if ga.config.DryRun {
		return // Only reconcile in live mode
	}

	// Get all positions from Binance
	exchangePositions, err := ga.futuresClient.GetPositions()
	if err != nil {
		ga.logger.Debug("Failed to get exchange positions for reconciliation", "error", err)
		return
	}

	// Build map of exchange positions by symbol+side for quick lookup
	type posKey struct {
		symbol string
		side   string
	}
	exchangeMap := make(map[posKey]*binance.FuturesPosition)
	for i := range exchangePositions {
		pos := &exchangePositions[i]
		if pos.PositionAmt != 0 {
			key := posKey{symbol: pos.Symbol, side: pos.PositionSide}
			exchangeMap[key] = pos
		}
	}

	// Check each internal position against Binance
	ga.mu.Lock()
	defer ga.mu.Unlock()

	positionsToRemove := make([]string, 0)

	for symbol, internalPos := range ga.positions {
		// Determine position side for lookup
		exchangeSide := "LONG"
		if internalPos.Side == "SHORT" {
			exchangeSide = "SHORT"
		}

		key := posKey{symbol: symbol, side: exchangeSide}
		exchangePos, exists := exchangeMap[key]

		if !exists {
			// Position no longer exists on Binance - was closed externally
			ga.logger.Warn("Position reconciliation: position closed externally",
				"symbol", symbol,
				"side", internalPos.Side,
				"internal_qty", internalPos.RemainingQty)

			// Calculate realized PnL if we know the close price
			positionsToRemove = append(positionsToRemove, symbol)
			continue
		}

		// Position exists - check for quantity mismatch
		exchangeQty := math.Abs(exchangePos.PositionAmt)
		internalQty := internalPos.RemainingQty

		// If quantities differ by more than 1%, update internal state
		qtyDiff := math.Abs(exchangeQty-internalQty) / internalQty
		if qtyDiff > 0.01 {
			if exchangeQty < internalQty*0.5 {
				// Significant reduction - likely partial close we missed
				ga.logger.Warn("Position reconciliation: quantity mismatch detected",
					"symbol", symbol,
					"internal_qty", internalQty,
					"exchange_qty", exchangeQty,
					"diff_pct", qtyDiff*100)

				// Update internal quantity to match exchange
				internalPos.RemainingQty = exchangeQty

				// If quantity is very small (< 1% of original), treat as closed
				if exchangeQty < internalPos.OriginalQty*0.01 {
					ga.logger.Info("Position reconciliation: position nearly closed",
						"symbol", symbol,
						"remaining_qty", exchangeQty)
					positionsToRemove = append(positionsToRemove, symbol)
				}
			} else if exchangeQty > internalQty {
				// Exchange has more than we thought - DCA or averaging happened externally
				ga.logger.Warn("Position reconciliation: external position increase detected",
					"symbol", symbol,
					"internal_qty", internalQty,
					"exchange_qty", exchangeQty)
				// Update to match exchange
				internalPos.RemainingQty = exchangeQty
				// Also update entry price if available
				if exchangePos.EntryPrice > 0 {
					internalPos.EntryPrice = exchangePos.EntryPrice
				}
			}
		}

		// Update mark price for PnL calculation
		if exchangePos.MarkPrice > 0 {
			internalPos.UnrealizedPnL = exchangePos.UnrealizedProfit
		}

		// CHECK FOR MISSING SLTP ORDERS - FIX: recreate if deleted manually
		// This runs during reconciliation to ensure positions always have protection
		// NOTE: We copy the position to avoid race conditions - the original may be modified
		// by other goroutines during reconciliation
		posCopy := *internalPos // Create a copy to avoid data race
		go func(sym string, pos *GiniePosition) {
			ga.ensureSLTPOrdersExist(sym, pos)
		}(symbol, &posCopy)
	}

	// Remove positions that were closed externally
	for _, symbol := range positionsToRemove {
		pos := ga.positions[symbol]

		// Fetch actual closing price and PnL from Binance trade history
		closePrice, realizedPnL, pnlPercent := ga.getClosingPnLFromBinance(symbol, pos)

		// Determine close reason based on trade data
		closeReason := "Position closed externally (reconciliation)"
		if realizedPnL != 0 {
			if realizedPnL > 0 {
				closeReason = fmt.Sprintf("take_profit (reconciliation, PnL: $%.2f)", realizedPnL)
			} else {
				closeReason = fmt.Sprintf("stop_loss (reconciliation, PnL: $%.2f)", realizedPnL)
			}
		}

		// Record with actual close price and PnL from Binance
		ga.recordTrade(GinieTradeResult{
			Symbol:     symbol,
			Action:     "full_close",
			Side:       pos.Side,
			Quantity:   pos.RemainingQty,
			Price:      closePrice,
			PnL:        realizedPnL,
			PnLPercent: pnlPercent,
			Reason:     closeReason,
			Timestamp:  time.Now(),
			Mode:       pos.Mode,
		})

		ga.logger.Info("Position reconciliation: recorded close with actual PnL",
			"symbol", symbol,
			"side", pos.Side,
			"close_price", closePrice,
			"realized_pnl", realizedPnL,
			"pnl_percent", pnlPercent)

		// Log external close to trade lifecycle
		if ga.eventLogger != nil && pos.FuturesTradeID > 0 {
			go ga.eventLogger.LogExternalClose(
				context.Background(),
				pos.FuturesTradeID,
				symbol,
				closePrice,
				pos.RemainingQty,
				realizedPnL,
				pnlPercent,
			)
		}

		delete(ga.positions, symbol)

		// Cancel any remaining algo orders
		go func(sym string) {
			success, failed, err := ga.cancelAllAlgoOrdersForSymbol(sym)
			if err != nil {
				ga.logger.Warn("Failed to cancel all orders for externally closed position",
					"symbol", sym,
					"successful", success,
					"failed", failed,
					"error", err)
			} else if success > 0 {
				ga.logger.Info("Cancelled orders for externally closed position",
					"symbol", sym,
					"cancelled_count", success)
			}
		}(symbol)
	}

	if len(positionsToRemove) > 0 {
		ga.logger.Info("Position reconciliation completed",
			"removed_count", len(positionsToRemove),
			"remaining_positions", len(ga.positions))
	}

	// PHASE 2: Add positions that exist on Binance but not in internal tracking
	// This handles positions opened externally or from previous sessions
	positionsToAdd := make([]*binance.FuturesPosition, 0)

	for key, exchangePos := range exchangeMap {
		// Skip if we already have this position tracked
		if _, exists := ga.positions[key.symbol]; exists {
			continue
		}

		// This is a new position on Binance that we're not tracking
		ga.logger.Info("Position reconciliation: found untracked Binance position",
			"symbol", key.symbol,
			"side", key.side,
			"quantity", math.Abs(exchangePos.PositionAmt),
			"entry_price", exchangePos.EntryPrice)

		positionsToAdd = append(positionsToAdd, exchangePos)
	}

	// Add discovered positions to tracking (release lock briefly for placeSLTPOrders)
	if len(positionsToAdd) > 0 {
		for _, exchangePos := range positionsToAdd {
			side := "LONG"
			if exchangePos.PositionAmt < 0 {
				side = "SHORT"
			}
			qty := math.Abs(exchangePos.PositionAmt)

			// Select mode based on user's enabled modes (fixes hardcoded swing bypass)
			externalMode := ga.selectEnabledModeForPosition()

			// Create internal position entry
			newPos := &GiniePosition{
				Symbol:       exchangePos.Symbol,
				Side:         side,
				EntryPrice:   exchangePos.EntryPrice,
				OriginalQty:  qty,
				RemainingQty: qty,
				EntryTime:    time.Now(), // Approximate
				Mode:         externalMode, // Use user's enabled mode preference
				HighestPrice: exchangePos.MarkPrice,
				LowestPrice:  exchangePos.MarkPrice,
				Protection:   NewProtectionStatus(), // Initialize protection tracking
			}

			ga.positions[exchangePos.Symbol] = newPos

			ga.logger.Info("Position reconciliation: added untracked position to Ginie",
				"symbol", exchangePos.Symbol,
				"side", side,
				"quantity", qty,
				"entry_price", exchangePos.EntryPrice)
		}

		// Place SL/TP orders for newly added positions (in background to avoid lock issues)
		ga.logger.Info("Position reconciliation: placing SL/TP for newly discovered positions",
			"count", len(positionsToAdd))

		go func() {
			// Small delay to ensure positions are saved
			time.Sleep(500 * time.Millisecond)
			ga.placeSLTPOrdersForSyncedPositions()
		}()
	}

	if len(positionsToAdd) > 0 || len(positionsToRemove) > 0 {
		ga.logger.Info("Position reconciliation summary",
			"added", len(positionsToAdd),
			"removed", len(positionsToRemove),
			"total_tracked", len(ga.positions))
	}
}

// ==================== END POSITION RECONCILIATION ====================

func (ga *GinieAutopilot) cleanupOrphanAlgoOrders() {
	if ga.config.DryRun {
		return // Only cleanup in live mode
	}

	// Get all symbols with tracked positions
	ga.mu.RLock()
	trackedSymbols := make(map[string]bool)
	for symbol := range ga.positions {
		trackedSymbols[symbol] = true
	}
	ga.mu.RUnlock()

	// Get all open positions from Binance
	exchangePositions, err := ga.futuresClient.GetPositions()
	if err != nil {
		ga.logger.Debug("Failed to get exchange positions for orphan cleanup", "error", err)
		return
	}

	// Build set of symbols with actual positions on exchange
	exchangeSymbols := make(map[string]bool)
	for _, pos := range exchangePositions {
		if pos.PositionAmt != 0 {
			exchangeSymbols[pos.Symbol] = true
		}
	}

	// For each tracked symbol, check if it has orders but no exchange position
	for symbol := range trackedSymbols {
		if !exchangeSymbols[symbol] {
			// Position in Ginie but not on exchange - cancel orphan orders
			log.Printf("[GINIE-CLEANUP] Orphan orders detected for %s (position closed on exchange)", symbol)
			success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
			if err != nil {
				log.Printf("[GINIE-CLEANUP] WARNING: Failed to fully cancel orders for %s: %v (success=%d, failed=%d)", symbol, err, success, failed)
			} else if success > 0 {
				log.Printf("[GINIE-CLEANUP] Successfully cancelled %d orphan orders for %s", success, symbol)
			}
		}
	}

	// Also check for algo orders on symbols we don't track at all
	// CRITICAL: Must check ALL symbols with orders, not just a hardcoded list
	// Get all open algo orders first to identify which symbols have orphans
	allOpenOrders, err := ga.futuresClient.GetOpenAlgoOrders("")
	if err != nil {
		ga.logger.Debug("Failed to get all open algo orders for orphan check", "error", err)
	} else {
		// Build set of symbols that have open orders
		ordersMap := make(map[string][]binance.AlgoOrder)
		for _, order := range allOpenOrders {
			ordersMap[order.Symbol] = append(ordersMap[order.Symbol], order)
		}

		// For each symbol with orders, check if it has a corresponding position
		for symbol, orders := range ordersMap {
			if trackedSymbols[symbol] || exchangeSymbols[symbol] {
				// This symbol has a position - check order count
				if len(orders) > 4 {
					// More than 4 orders is suspicious (should be max 2-4 for SL/TP)
					log.Printf("[GINIE-CLEANUP] %s has %d orders (may be orphans), cleaning up", symbol, len(orders))
					success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
					if err != nil {
						log.Printf("[GINIE-CLEANUP] WARNING: Failed to fully cancel orders for %s: %v (success=%d, failed=%d)", symbol, err, success, failed)
					} else {
						log.Printf("[GINIE-CLEANUP] Successfully cancelled %d orders for %s", success, symbol)
					}
				}
				continue // Skip symbols with positions
			}

			// Symbol has no tracked position and no exchange position (TRUE ORPHAN)
			// Cancel all orders for this symbol
			if len(orders) > 0 {
				log.Printf("[GINIE-CLEANUP] Found %d ORPHAN orders for %s (no position), cancelling all", len(orders), symbol)
				success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
				if err != nil {
					log.Printf("[GINIE-CLEANUP] ERROR: Failed to fully cancel %d orphan orders for %s: %v (success=%d, failed=%d)", len(orders), symbol, err, success, failed)
				} else {
					log.Printf("[GINIE-CLEANUP] Successfully cancelled %d ORPHAN orders for %s", success, symbol)
				}
			}
		}
	}
}

// cleanupAllOrphanOrders does a comprehensive cleanup of ALL orphan orders at startup
// It checks every position on exchange and cancels orders for symbols without positions
func (ga *GinieAutopilot) cleanupAllOrphanOrders() {
	if ga.config.DryRun {
		return
	}

	// Get all open positions from Binance
	exchangePositions, err := ga.futuresClient.GetPositions()
	if err != nil {
		ga.logger.Warn("Failed to get exchange positions for orphan cleanup", "error", err)
		return
	}

	// Build set of symbols with actual positions on exchange
	symbolsWithPositions := make(map[string]bool)
	for _, pos := range exchangePositions {
		if pos.PositionAmt != 0 {
			symbolsWithPositions[pos.Symbol] = true
		}
	}

	log.Printf("[GINIE-CLEANUP] Found %d positions on exchange", len(symbolsWithPositions))

	// For each position, verify it has at most 2 algo orders (1 SL + 1 TP)
	orderCount := 0
	cancelledCount := 0
	for symbol := range symbolsWithPositions {
		orders, err := ga.futuresClient.GetOpenAlgoOrders(symbol)
		if err != nil {
			continue
		}
		orderCount += len(orders)

		// If more than 2 orders for a position, cancel all and let the system re-create them
		if len(orders) > 2 {
			log.Printf("[GINIE-CLEANUP] %s has %d orders (expected max 2), cancelling all to reset", symbol, len(orders))
			success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
			if err != nil {
				log.Printf("[GINIE-CLEANUP] WARNING: Failed to fully cancel orders for %s: %v (success=%d, failed=%d)", symbol, err, success, failed)
			} else {
				log.Printf("[GINIE-CLEANUP] Successfully cancelled %d orders for %s", success, symbol)
			}
			cancelledCount += success
			time.Sleep(100 * time.Millisecond) // Rate limit protection
		}
	}

	// CRITICAL: Check ALL symbols with open orders, not just a hardcoded list
	// This ensures we catch orphan orders on any symbol
	allOpenOrders, err := ga.futuresClient.GetOpenAlgoOrders("")
	if err != nil {
		log.Printf("[GINIE-CLEANUP] Warning: Failed to get all open orders for comprehensive cleanup: %v", err)
	} else {
		// Build map of all symbols with open orders
		ordersMap := make(map[string][]binance.AlgoOrder)
		for _, order := range allOpenOrders {
			ordersMap[order.Symbol] = append(ordersMap[order.Symbol], order)
		}

		// Check each symbol with orders
		for symbol, orders := range ordersMap {
			if symbolsWithPositions[symbol] {
				// Symbol has a position - check if too many orders
				if len(orders) > 4 {
					log.Printf("[GINIE-CLEANUP] %s has %d orders (expected max 4), too many - cancelling all to reset", symbol, len(orders))
					success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
					if err != nil {
						log.Printf("[GINIE-CLEANUP] WARNING: Failed to fully cancel orders for %s: %v (success=%d, failed=%d)", symbol, err, success, failed)
					} else {
						log.Printf("[GINIE-CLEANUP] Successfully cancelled %d orders for %s", success, symbol)
					}
					cancelledCount += success
					time.Sleep(50 * time.Millisecond)
				}
				continue
			}

			// No position for this symbol - cancel all orders (ORPHAN ORDERS)
			log.Printf("[GINIE-CLEANUP] Found %d ORPHAN orders for %s (no position), cancelling", len(orders), symbol)
			success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
			if err != nil {
				log.Printf("[GINIE-CLEANUP] ERROR: Failed to fully cancel %d orphan orders for %s: %v (success=%d, failed=%d)", len(orders), symbol, err, success, failed)
			} else {
				log.Printf("[GINIE-CLEANUP] Successfully cancelled %d ORPHAN orders for %s", success, symbol)
			}
			cancelledCount += success
			time.Sleep(50 * time.Millisecond)
		}
	}

	log.Printf("[GINIE-CLEANUP] Cleanup complete: %d total orders checked, %d successfully cancelled", orderCount, cancelledCount)
}

// periodicOrphanOrderCleanup runs orphan order cleanup every 5 minutes
// This is critical to prevent order accumulation from repeated position updates
func (ga *GinieAutopilot) periodicOrphanOrderCleanup() {
	defer ga.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			ga.logger.Error("PANIC in orphan order cleanup - restarting", "panic", r)
			log.Printf("[GINIE-PANIC] Orphan order cleanup panic: %v", r)
			time.Sleep(5 * time.Second)
			ga.wg.Add(1)
			go ga.periodicOrphanOrderCleanup()
		}
	}()

	ga.logger.Info("Periodic orphan order cleanup goroutine started - runs every 5 minutes")

	// Run cleanup immediately on startup
	ga.cleanupAllOrphanOrders()

	// Then run periodically every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ga.stopChan:
			ga.logger.Info("Periodic orphan order cleanup stopping")
			return
		case <-ticker.C:
			ga.logger.Info("Running periodic orphan order cleanup...")
			ga.cleanupAllOrphanOrders()
		}
	}
}

// cancelExistingSLTPOrders cancels all existing SL/TP algo orders for a position
func (ga *GinieAutopilot) cancelExistingSLTPOrders(pos *GiniePosition) {
	if pos == nil {
		return
	}

	// Cancel stop loss algo order
	if pos.StopLossAlgoID > 0 {
		err := ga.futuresClient.CancelAlgoOrder(pos.Symbol, pos.StopLossAlgoID)
		if err != nil {
			ga.logger.Debug("Failed to cancel SL algo order (may already be filled/cancelled)",
				"symbol", pos.Symbol,
				"algo_id", pos.StopLossAlgoID,
				"error", err.Error())
		}
		pos.StopLossAlgoID = 0
	}

	// Cancel take profit algo orders
	for _, tpAlgoID := range pos.TakeProfitAlgoIDs {
		if tpAlgoID > 0 {
			err := ga.futuresClient.CancelAlgoOrder(pos.Symbol, tpAlgoID)
			if err != nil {
				ga.logger.Debug("Failed to cancel TP algo order (may already be filled/cancelled)",
					"symbol", pos.Symbol,
					"algo_id", tpAlgoID,
					"error", err.Error())
			}
		}
	}
	pos.TakeProfitAlgoIDs = nil
}

// modifySLTPOrders updates SL/TP orders with new prices
func (ga *GinieAutopilot) modifySLTPOrders(pos *GiniePosition, newSL float64, newTPs []float64) {
	if pos == nil {
		return
	}

	// Cancel ALL existing algo orders from Binance (more robust than stored IDs)
	success, failed, err := ga.cancelAllAlgoOrdersForSymbol(pos.Symbol)
	if err != nil || failed > 0 {
		ga.logger.Warn("Failed to cancel all algo orders in modifySLTPOrders",
			"symbol", pos.Symbol,
			"success", success,
			"failed", failed,
			"error", err)
	}

	// Clear stored algo IDs since we cancelled all orders
	pos.StopLossAlgoID = 0
	pos.TakeProfitAlgoIDs = nil

	// Update position with new SL/TP values
	if newSL > 0 {
		pos.StopLoss = newSL
	}

	// Update TP prices if provided
	for i, newTP := range newTPs {
		if i < len(pos.TakeProfits) && newTP > 0 {
			pos.TakeProfits[i].Price = newTP
		}
	}

	// Place new orders
	ga.placeSLTPOrders(pos)

	ga.logger.Info("SL/TP orders modified",
		"symbol", pos.Symbol,
		"new_sl", newSL,
		"new_tps", newTPs)
}

// runAdaptiveSLTPMonitor runs the adaptive SL/TP monitoring loop
func (ga *GinieAutopilot) runAdaptiveSLTPMonitor() {
	defer ga.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			ga.logger.Error("PANIC in adaptive SLTP monitor - restarting", "panic", r)
			log.Printf("[GINIE-PANIC] Adaptive SLTP monitor panic: %v", r)
			time.Sleep(5 * time.Second)
			ga.wg.Add(1)
			go ga.runAdaptiveSLTPMonitor()
		}
	}()

	// Use 10 second base interval, then check mode-specific timing
	baseTicker := time.NewTicker(10 * time.Second)
	defer baseTicker.Stop()

	for {
		select {
		case <-ga.stopChan:
			return
		case <-baseTicker.C:
			ga.checkAndUpdateAdaptiveSLTP()
		}
	}
}

// checkAndUpdateAdaptiveSLTP checks each position for SL/TP updates based on mode intervals
func (ga *GinieAutopilot) checkAndUpdateAdaptiveSLTP() {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if ga.llmAnalyzer == nil || !ga.llmAnalyzer.IsEnabled() {
		return
	}

	now := time.Now()

	for symbol, pos := range ga.positions {
		// Determine update interval based on mode
		var updateInterval time.Duration
		switch pos.Mode {
		case GinieModeScalp:
			updateInterval = time.Duration(ga.config.ScalpSLTPUpdateInterval) * time.Second
		case GinieModeSwing:
			updateInterval = time.Duration(ga.config.SwingSLTPUpdateInterval) * time.Second
		case GinieModePosition:
			updateInterval = time.Duration(ga.config.PositionSLTPUpdateInterval) * time.Second
		default:
			updateInterval = time.Duration(ga.config.SwingSLTPUpdateInterval) * time.Second
		}

		// Check if it's time to update
		if now.Sub(pos.LastLLMUpdate) >= updateInterval {
			// Update SL/TP from LLM (run without lock to avoid blocking)
			ga.mu.Unlock()
			ga.updatePositionSLTPFromLLM(symbol, pos)
			ga.mu.Lock()
		}
	}
}

// updatePositionSLTPFromLLM gets updated SL/TP suggestions from LLM and modifies orders
func (ga *GinieAutopilot) updatePositionSLTPFromLLM(symbol string, pos *GiniePosition) {
	if pos == nil || ga.llmAnalyzer == nil {
		return
	}

	// Rule 4: Check if LLM SL updates are disabled for this symbol (kill switch active)
	if ga.llmSLDisabled[symbol] {
		ga.logger.Debug("LLM SL updates disabled for symbol (kill switch active)",
			"symbol", symbol)
		return
	}

	// Get market data for LLM analysis - timeframe based on mode
	timeframe := "5m" // Default
	switch pos.Mode {
	case GinieModeScalp:
		timeframe = "1m"
	case GinieModeSwing:
		timeframe = "1h"
	case GinieModePosition:
		timeframe = "4h"
	}

	klines, err := ga.futuresClient.GetFuturesKlines(symbol, timeframe, 50)
	if err != nil {
		ga.logger.Debug("Failed to get klines for LLM SL/TP update",
			"symbol", symbol,
			"error", err.Error())
		return
	}

	// Get current price
	currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		ga.logger.Debug("Failed to get current price",
			"symbol", symbol,
			"error", err.Error())
		return
	}

	// Calculate P&L percentage
	pnlPercent := 0.0
	if pos.EntryPrice > 0 {
		if pos.Side == "LONG" {
			pnlPercent = ((currentPrice - pos.EntryPrice) / pos.EntryPrice) * 100
		} else {
			pnlPercent = ((pos.EntryPrice - currentPrice) / pos.EntryPrice) * 100
		}
	}

	// Calculate hold duration
	holdDuration := time.Since(pos.EntryTime).Round(time.Minute).String()

	// Get current TP price (TP1 if available)
	currentTP := 0.0
	if len(pos.TakeProfits) > 0 {
		currentTP = pos.TakeProfits[0].Price
	}

	// Build position info for LLM with FULL context
	posInfo := &llm.PositionInfo{
		Symbol:        symbol,
		Side:          pos.Side,
		EntryPrice:    pos.EntryPrice,
		CurrentPrice:  currentPrice,
		Quantity:      pos.RemainingQty,
		UnrealizedPnL: pos.UnrealizedPnL,
		PnLPercent:    pnlPercent,
		CurrentSL:     pos.StopLoss,
		CurrentTP:     currentTP,
		HoldDuration:  holdDuration,
		Mode:          string(pos.Mode),
	}

	// Call LLM analyzer with FULL position context
	sltpAnalysis, err := ga.llmAnalyzer.AnalyzePositionSLTP(posInfo, klines)

	// Track LLM call time for diagnostics
	ga.mu.Lock()
	ga.lastLLMCallTime = time.Now()
	ga.mu.Unlock()

	if err != nil {
		ga.logger.Debug("LLM SL/TP analysis failed",
			"symbol", symbol,
			"error", err.Error())
		return
	}

	ga.logger.Info("LLM position analysis received",
		"symbol", symbol,
		"action", sltpAnalysis.Action,
		"confidence", sltpAnalysis.Confidence,
		"recommended_sl", sltpAnalysis.RecommendedSL,
		"recommended_tp", sltpAnalysis.RecommendedTP,
		"urgency", sltpAnalysis.Urgency,
		"sl_reasoning", sltpAnalysis.SLReasoning)

	// Only update if LLM confidence is high enough
	if sltpAnalysis.Confidence < 0.5 {
		ga.logger.Debug("LLM confidence too low, skipping update",
			"symbol", symbol,
			"confidence", sltpAnalysis.Confidence)
		return
	}

	// Extract recommended values
	llmSL := sltpAnalysis.RecommendedSL
	llmTP := sltpAnalysis.RecommendedTP

	// Check if LLM action is "close_now" or if SL would immediately trigger
	// If SL is on the wrong side of current price, close position at market
	shouldCloseNow := sltpAnalysis.Action == "close_now"

	if llmSL > 0 && !shouldCloseNow {
		// Check if LLM SL would immediately trigger (already breached)
		if pos.Side == "LONG" && llmSL >= currentPrice {
			// For LONG: SL should be below current price. If SL >= current price, price already hit SL
			ga.logger.Warn("LLM SL would immediately trigger for LONG, closing position at market",
				"symbol", symbol,
				"llm_sl", llmSL,
				"current_price", currentPrice)
			shouldCloseNow = true
		} else if pos.Side == "SHORT" && llmSL <= currentPrice {
			// For SHORT: SL should be above current price. If SL <= current price, price already hit SL
			ga.logger.Warn("LLM SL would immediately trigger for SHORT, closing position at market",
				"symbol", symbol,
				"llm_sl", llmSL,
				"current_price", currentPrice)
			shouldCloseNow = true
		}
	}

	// If LLM says close now or SL is breached, close position at market
	if shouldCloseNow && !ga.config.DryRun {
		ga.logger.Info("Closing position at market based on LLM analysis",
			"symbol", symbol,
			"action", sltpAnalysis.Action,
			"sl_reasoning", sltpAnalysis.SLReasoning)

		// Cancel existing algo orders first
		success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
		if err != nil || failed > 0 {
			ga.logger.Warn("Failed to cancel all algo orders before LLM-triggered close",
				"symbol", symbol,
				"success", success,
				"failed", failed,
				"error", err)
		}

		// Close position at market
		err = ga.closePositionAtMarket(pos)
		if err != nil {
			ga.logger.Error("Failed to close position at market",
				"symbol", symbol,
				"error", err.Error())
		} else {
			ga.logger.Info("Position closed at market successfully",
				"symbol", symbol,
				"close_price", currentPrice)
		}
		return
	}

	// Only update if LLM provided valid values and they're reasonable
	var newSL float64
	var newTPs []float64

	if llmSL > 0 {
		// First check basic direction (SL should be on correct side of current price)
		validDirection := false
		if pos.Side == "LONG" && llmSL < currentPrice {
			validDirection = true
		} else if pos.Side == "SHORT" && llmSL > currentPrice {
			validDirection = true
		}

		if !validDirection {
			ga.logger.Debug("LLM SL on wrong side of price, skipping",
				"symbol", symbol,
				"llm_sl", llmSL,
				"current_price", currentPrice,
				"side", pos.Side)
			// Record rejected SL update (wrong direction)
			ga.RecordSLUpdate(symbol, pos.StopLoss, llmSL, currentPrice, "rejected", "wrong_direction", "llm", sltpAnalysis.Confidence)
		} else {
			// Apply our strict SL validation rules (never widen, max 10% move, min ATR distance)
			valid, reason := ga.validateSLUpdate(pos, llmSL, currentPrice, klines)
			if !valid {
				// Record bad LLM call (activates kill switch after 3 consecutive failures)
				ga.recordBadLLMCall(symbol)
				ga.logger.Warn("LLM SL rejected by validation rules",
					"symbol", symbol,
					"reason", reason,
					"llm_sl", llmSL,
					"current_sl", pos.StopLoss,
					"current_price", currentPrice,
					"bad_call_count", ga.badLLMCallCount[symbol])
				// Record rejected SL update with the rule that rejected it
				ga.RecordSLUpdate(symbol, pos.StopLoss, llmSL, currentPrice, "rejected", reason, "llm", sltpAnalysis.Confidence)
			} else {
				// Valid SL update - reset bad call counter
				ga.resetBadLLMCount(symbol)
				newSL = llmSL
				// Record applied SL update
				ga.RecordSLUpdate(symbol, pos.StopLoss, llmSL, currentPrice, "applied", "", "llm", sltpAnalysis.Confidence)
			}
		}
	}

	// For TP, use LLM suggestion for TP1 but keep our 4-level structure
	if llmTP > 0 && len(pos.TakeProfits) > 0 {
		// Validate TP is in the right direction
		if pos.Side == "LONG" && llmTP > currentPrice {
			// Update TP1 with LLM suggestion, scale others proportionally
			tpRatio := llmTP / pos.TakeProfits[0].Price
			newTPs = make([]float64, len(pos.TakeProfits))
			for i := range pos.TakeProfits {
				newTPs[i] = pos.TakeProfits[i].Price * tpRatio
			}
		} else if pos.Side == "SHORT" && llmTP < currentPrice {
			tpRatio := llmTP / pos.TakeProfits[0].Price
			newTPs = make([]float64, len(pos.TakeProfits))
			for i := range pos.TakeProfits {
				newTPs[i] = pos.TakeProfits[i].Price * tpRatio
			}
		}
	}

	// Apply updates if we have valid new values
	if newSL > 0 || len(newTPs) > 0 {
		ga.mu.Lock()
		// Check if we should move to breakeven (after TP1 hit)
		if pos.CurrentTPLevel >= 1 && !pos.MovedToBreakeven && ga.config.MoveToBreakevenAfterTP1 {
			// Move SL to breakeven (entry price + small buffer)
			breakevenSL := pos.EntryPrice
			if pos.Side == "LONG" {
				breakevenSL = pos.EntryPrice * (1 + ga.config.BreakevenBuffer/100)
			} else {
				breakevenSL = pos.EntryPrice * (1 - ga.config.BreakevenBuffer/100)
			}
			newSL = breakevenSL
			pos.MovedToBreakeven = true
			ga.logger.Info("Moving SL to breakeven after TP1",
				"symbol", symbol,
				"breakeven_sl", breakevenSL)
		}

		if !ga.config.DryRun {
			ga.modifySLTPOrders(pos, newSL, newTPs)
		} else {
			// In dry run, just update the local position
			if newSL > 0 {
				pos.StopLoss = newSL
			}
			for i, newTP := range newTPs {
				if i < len(pos.TakeProfits) && newTP > 0 {
					pos.TakeProfits[i].Price = newTP
				}
			}
		}

		pos.LastLLMUpdate = time.Now()
		ga.mu.Unlock()

		ga.logger.Info("Adaptive SL/TP updated from LLM",
			"symbol", symbol,
			"mode", pos.Mode,
			"new_sl", newSL,
			"new_tps", newTPs,
			"action", sltpAnalysis.Action,
			"confidence", sltpAnalysis.Confidence)

		// Log SL revised event to trade lifecycle (only if SL was actually updated)
		if newSL > 0 && ga.eventLogger != nil && pos.FuturesTradeID > 0 {
			revisionCount, _ := ga.repo.GetDB().CountSLRevisions(context.Background(), pos.FuturesTradeID)
			go ga.eventLogger.LogSLRevised(
				context.Background(),
				pos.FuturesTradeID,
				symbol,
				pos.OriginalSL, // Use original SL as old value for reference
				newSL,
				fmt.Sprintf("LLM adaptive update (confidence: %.1f%%)", sltpAnalysis.Confidence),
				revisionCount+1,
			)
		}
	} else {
		// Update timestamp even if no changes to prevent continuous retry
		ga.mu.Lock()
		pos.LastLLMUpdate = time.Now()
		ga.mu.Unlock()
	}
}

// calculateDefaultSL calculates a default stop loss for synced positions
func (ga *GinieAutopilot) calculateDefaultSL(entryPrice float64, isLong bool, slPercent float64) float64 {
	if isLong {
		return entryPrice * (1 - slPercent/100)
	}
	return entryPrice * (1 + slPercent/100)
}

// RecalculateAdaptiveSLTP recalculates SL/TP for all positions using adaptive logic
// This applies LLM + ATR based SL/TP to existing/naked positions
func (ga *GinieAutopilot) RecalculateAdaptiveSLTP() (int, error) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if len(ga.positions) == 0 {
		return 0, nil
	}

	updated := 0

	for symbol, pos := range ga.positions {
		// Get klines for ATR calculation
		klines, err := ga.futuresClient.GetFuturesKlines(symbol, "1m", 50)
		if err != nil {
			ga.logger.Error("Failed to get klines for SL/TP recalc", "symbol", symbol, "error", err)
			continue
		}

		if len(klines) < 20 {
			ga.logger.Warn("Insufficient klines for SL/TP recalc", "symbol", symbol, "count", len(klines))
			continue
		}

		// Get current price
		currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
		if err != nil {
			ga.logger.Error("Failed to get price for SL/TP recalc", "symbol", symbol, "error", err)
			continue
		}

		// Calculate ATR
		atrPct := ga.calculateATRPercent(klines)
		if atrPct == 0 {
			atrPct = 1.0 // Fallback
		}

		// Get LLM analysis if available
		var llmSLPct, llmTPPct float64
		var llmUsed bool
		if ga.analyzer != nil && ga.analyzer.signalAggregator != nil {
			llmAnalysis := ga.analyzer.signalAggregator.GetCachedLLMAnalysis(symbol)
			if llmAnalysis != nil {
				isLong := pos.Side == "LONG"
				if llmAnalysis.StopLoss != nil && *llmAnalysis.StopLoss > 0 {
					if isLong {
						llmSLPct = ((currentPrice - *llmAnalysis.StopLoss) / currentPrice) * 100
					} else {
						llmSLPct = ((*llmAnalysis.StopLoss - currentPrice) / currentPrice) * 100
					}
					if llmSLPct > 0 {
						llmUsed = true
					}
				}
				if llmAnalysis.TakeProfit != nil && *llmAnalysis.TakeProfit > 0 {
					if isLong {
						llmTPPct = ((*llmAnalysis.TakeProfit - currentPrice) / currentPrice) * 100
					} else {
						llmTPPct = ((currentPrice - *llmAnalysis.TakeProfit) / currentPrice) * 100
					}
					if llmTPPct > 0 {
						llmUsed = true
					}
				}
			}
		}

		// Check for manual SL/TP override from ModeConfigs
		sm := GetSettingsManager()
		settings := sm.GetCurrentSettings()
		var manualSLPct, manualTPPct float64

		modeToConfigKey := map[string]string{
			string(GinieModeUltraFast): "ultra_fast",
			string(GinieModeScalp):     "scalp",
			string(GinieModeSwing):     "swing",
			string(GinieModePosition):  "position",
		}
		if modeKey, ok := modeToConfigKey[string(pos.Mode)]; ok {
			if modeConfig := settings.ModeConfigs[modeKey]; modeConfig != nil {
				if modeConfig.SLTP != nil {
					manualSLPct = modeConfig.SLTP.StopLossPercent
					manualTPPct = modeConfig.SLTP.TakeProfitPercent
				}
			}
		}

		// Manual override takes precedence if set (> 0)
		var finalSLPct, finalTPPct float64
		if manualSLPct > 0 && manualTPPct > 0 {
			finalSLPct = manualSLPct
			finalTPPct = manualTPPct
			ga.logger.Info("Using manual SL/TP override for position",
				"symbol", symbol,
				"mode", pos.Mode,
				"sl_pct", finalSLPct,
				"tp_pct", finalTPPct)
		} else {
			// Mode-specific limits for ATR/LLM blend - read from ModeConfig with fallback defaults
			var baseSLMult, baseTPMult float64
			var minSL, maxSL, minTP, maxTP float64
			var llmWeight, atrWeight float64

			// Try to get mode config for ATR/LLM blending parameters
			modeConfig := ga.getModeConfig(pos.Mode)
			if modeConfig != nil && modeConfig.SLTP != nil {
				// Use config values if available
				baseSLMult = modeConfig.SLTP.ATRSLMultiplier
				baseTPMult = modeConfig.SLTP.ATRTPMultiplier
				minSL = modeConfig.SLTP.ATRSLMin
				maxSL = modeConfig.SLTP.ATRSLMax
				minTP = modeConfig.SLTP.ATRTPMin
				maxTP = modeConfig.SLTP.ATRTPMax
				llmWeight = modeConfig.SLTP.LLMWeight
				atrWeight = modeConfig.SLTP.ATRWeight
			}

			// Apply fallback defaults if config values are zero/not set
			if baseSLMult == 0 || baseTPMult == 0 || maxSL == 0 || maxTP == 0 {
				switch pos.Mode {
				case GinieModeUltraFast:
					// Ultra-fast: Very tight SL/TP for quick momentum trades
					if baseSLMult == 0 {
						baseSLMult = 0.3
					}
					if baseTPMult == 0 {
						baseTPMult = 0.6
					}
					if minSL == 0 {
						minSL = 0.3
					}
					if maxSL == 0 {
						maxSL = 1.5
					}
					if minTP == 0 {
						minTP = 0.5
					}
					if maxTP == 0 {
						maxTP = 3.0
					}
				case GinieModeScalp:
					if baseSLMult == 0 {
						baseSLMult = 0.5
					}
					if baseTPMult == 0 {
						baseTPMult = 1.0
					}
					if minSL == 0 {
						minSL = 0.2
					}
					if maxSL == 0 {
						maxSL = 0.8
					}
					if minTP == 0 {
						minTP = 0.3
					}
					if maxTP == 0 {
						maxTP = 2.0
					}
				case GinieModeSwing:
					if baseSLMult == 0 {
						baseSLMult = 1.5
					}
					if baseTPMult == 0 {
						baseTPMult = 3.0
					}
					if minSL == 0 {
						minSL = 1.0
					}
					if maxSL == 0 {
						maxSL = 5.0
					}
					if minTP == 0 {
						minTP = 2.0
					}
					if maxTP == 0 {
						maxTP = 15.0
					}
				case GinieModePosition:
					if baseSLMult == 0 {
						baseSLMult = 2.5
					}
					if baseTPMult == 0 {
						baseTPMult = 5.0
					}
					if minSL == 0 {
						minSL = 3.0
					}
					if maxSL == 0 {
						maxSL = 15.0
					}
					if minTP == 0 {
						minTP = 5.0
					}
					if maxTP == 0 {
						maxTP = 50.0
					}
				default:
					// Default to swing
					if baseSLMult == 0 {
						baseSLMult = 1.5
					}
					if baseTPMult == 0 {
						baseTPMult = 3.0
					}
					if minSL == 0 {
						minSL = 1.0
					}
					if maxSL == 0 {
						maxSL = 5.0
					}
					if minTP == 0 {
						minTP = 2.0
					}
					if maxTP == 0 {
						maxTP = 15.0
					}
				}
			}

			// Apply fallback defaults for LLM/ATR weights if not configured
			if llmWeight == 0 {
				llmWeight = 0.7 // Default 70% LLM weight
			}
			if atrWeight == 0 {
				atrWeight = 0.3 // Default 30% ATR weight
			}

			// Calculate ATR-based SL/TP
			atrSLPct := atrPct * baseSLMult
			atrTPPct := atrPct * baseTPMult

			// Blend LLM and ATR using configured weights (default: 70% LLM, 30% ATR if LLM available)
			if llmUsed && llmSLPct > 0 {
				finalSLPct = llmSLPct*llmWeight + atrSLPct*atrWeight
			} else {
				finalSLPct = atrSLPct
			}
			if llmUsed && llmTPPct > 0 {
				finalTPPct = llmTPPct*llmWeight + atrTPPct*atrWeight
			} else {
				finalTPPct = atrTPPct
			}

			// Clamp to limits
			if finalSLPct < minSL {
				finalSLPct = minSL
			}
			if finalSLPct > maxSL {
				finalSLPct = maxSL
			}
			if finalTPPct < minTP {
				finalTPPct = minTP
			}
			if finalTPPct > maxTP {
				finalTPPct = maxTP
			}
		}

		// Apply to position
		isLong := pos.Side == "LONG"
		direction := 1.0
		if !isLong {
			direction = -1.0
		}

		// Update Stop Loss (from current price, not entry)
		oldSL := pos.StopLoss
		pos.StopLoss = pos.EntryPrice * (1 - direction*finalSLPct/100)
		pos.OriginalSL = pos.StopLoss

		// Generate TP levels based on TP mode (single vs multi)
		// Use modeToConfigKey already declared above
		useSingleTP := false
		if modeKey, ok := modeToConfigKey[string(pos.Mode)]; ok {
			if modeConfig := settings.ModeConfigs[modeKey]; modeConfig != nil {
				if modeConfig.SLTP != nil {
					useSingleTP = modeConfig.SLTP.UseSingleTP
				}
			}
		}

		if useSingleTP {
			// Single TP mode: Close 100% at one level (ultra-fast and scalp modes)
			// Use mode-specific TP% (from finalTPPct) for the single TP target
			// This ensures ultra-fast uses GinieTPPercentUltrafast, scalp uses GinieTPPercentScalp
			singleTPPrice := pos.EntryPrice * (1 + direction*finalTPPct/100)
			pos.TakeProfits = []GinieTakeProfitLevel{
				{Level: 1, Percent: 100, GainPct: finalTPPct, Price: singleTPPrice, Status: "pending"},
			}
			ga.logger.Info("Single TP mode applied (100% at TP)",
				"symbol", symbol,
				"mode", pos.Mode,
				"tp_percent", finalTPPct,
				"tp_price", singleTPPrice,
				"note", "no TP1/TP2/TP3/TP4 split - full position closes at TP")
		} else {
			// Multi-TP mode: Use per-mode TPAllocation from mode_configs, fallback to global settings
			var tp1Pct, tp2Pct, tp3Pct, tp4Pct float64

			// Try to get from mode_configs first
			modeConfig := ga.getModeConfig(pos.Mode)
			if modeConfig != nil && modeConfig.SLTP != nil && len(modeConfig.SLTP.TPAllocation) >= 4 {
				tp1Pct = modeConfig.SLTP.TPAllocation[0]
				tp2Pct = modeConfig.SLTP.TPAllocation[1]
				tp3Pct = modeConfig.SLTP.TPAllocation[2]
				tp4Pct = modeConfig.SLTP.TPAllocation[3]
				ga.logger.Debug("Using mode_configs TPAllocation",
					"symbol", symbol,
					"mode", pos.Mode,
					"allocation", modeConfig.SLTP.TPAllocation)
			} else {
				// Fallback to defaults (legacy fields removed)
				tp1Pct, tp2Pct, tp3Pct, tp4Pct = 25, 25, 25, 25
			}

			// Ensure allocation sums to 100%
			totalAlloc := tp1Pct + tp2Pct + tp3Pct + tp4Pct
			if totalAlloc < 99 || totalAlloc > 101 {
				ga.logger.Warn("TP allocation doesn't sum to 100%, using defaults",
					"symbol", symbol,
					"total", totalAlloc)
				tp1Pct, tp2Pct, tp3Pct, tp4Pct = 25, 25, 25, 25
			}

			// Calculate actual gain % for each level based on allocation
			tp1Gain := finalTPPct * (tp1Pct / 100)
			tp2Gain := finalTPPct * (tp2Pct / 100)
			tp3Gain := finalTPPct * (tp3Pct / 100)
			tp4Gain := finalTPPct * (tp4Pct / 100)

			pos.TakeProfits = []GinieTakeProfitLevel{
				{Level: 1, Percent: tp1Pct, GainPct: tp1Gain, Price: pos.EntryPrice * (1 + direction*tp1Gain/100), Status: "pending"},
				{Level: 2, Percent: tp2Pct, GainPct: tp2Gain, Price: pos.EntryPrice * (1 + direction*tp2Gain/100), Status: "pending"},
				{Level: 3, Percent: tp3Pct, GainPct: tp3Gain, Price: pos.EntryPrice * (1 + direction*tp3Gain/100), Status: "pending"},
				{Level: 4, Percent: tp4Pct, GainPct: tp4Gain, Price: pos.EntryPrice * (1 + direction*tp4Gain/100), Status: "pending"},
			}

			ga.logger.Debug("Multi-TP mode applied",
				"symbol", symbol,
				"allocation", fmt.Sprintf("%.0f%%/%.0f%%/%.0f%%/%.0f%%", tp1Pct, tp2Pct, tp3Pct, tp4Pct))
		}

		// Apply configured trailing stop settings
		var trailingEnabled bool
		var trailingPercent, trailingActivation float64

		// Read trailing stop config from ModeConfigs (use modeToConfigKey already declared)
		// Defaults
		trailingEnabled = true
		trailingPercent = 1.5
		trailingActivation = 1.0

		if modeKey, ok := modeToConfigKey[string(pos.Mode)]; ok {
			if modeConfig := settings.ModeConfigs[modeKey]; modeConfig != nil {
				if modeConfig.SLTP != nil {
					trailingEnabled = modeConfig.SLTP.TrailingStopEnabled
					if modeConfig.SLTP.TrailingStopPercent > 0 {
						trailingPercent = modeConfig.SLTP.TrailingStopPercent
					}
					if modeConfig.SLTP.TrailingStopActivation > 0 {
						trailingActivation = modeConfig.SLTP.TrailingStopActivation
					}
				}
			}
		}

		if trailingEnabled {
			pos.TrailingPercent = trailingPercent
			pos.TrailingActivationPct = trailingActivation
			ga.logger.Debug("Trailing stop configured",
				"symbol", symbol,
				"mode", pos.Mode,
				"trailing_pct", trailingPercent,
				"activation_pct", trailingActivation)
		} else {
			pos.TrailingPercent = 0
			pos.TrailingActivationPct = 0
			ga.logger.Debug("Trailing stop disabled", "symbol", symbol, "mode", pos.Mode)
		}

		// Place SLTP orders on Binance in background
		posSymbol := symbol
		slPrice := roundPrice(posSymbol, pos.StopLoss)

		// Build TP prices array dynamically based on configured TP levels
		var tpPrices []float64
		var tpQuantities []float64
		for _, tpLevel := range pos.TakeProfits {
			tpPrices = append(tpPrices, roundPrice(posSymbol, tpLevel.Price))
			// Calculate quantity for this TP level based on its percent allocation
			tpQtyForLevel := roundQuantity(posSymbol, pos.RemainingQty*tpLevel.Percent/100)
			tpQuantities = append(tpQuantities, tpQtyForLevel)
		}

		qty := roundQuantity(posSymbol, pos.RemainingQty)
		posSide := pos.Side

		go func() {
			// Cancel existing orders with proper error logging
			if pos.StopLossAlgoID > 0 {
				if err := ga.futuresClient.CancelAlgoOrder(posSymbol, pos.StopLossAlgoID); err != nil {
					ga.logger.Warn("SLTP: Failed to cancel existing SL order (may already be cancelled)",
						"symbol", posSymbol,
						"algo_id", pos.StopLossAlgoID,
						"error", err.Error())
				}
			}
			for _, tpID := range pos.TakeProfitAlgoIDs {
				if tpID > 0 {
					if err := ga.futuresClient.CancelAlgoOrder(posSymbol, tpID); err != nil {
						ga.logger.Warn("SLTP: Failed to cancel existing TP order (may already be triggered)",
							"symbol", posSymbol,
							"algo_id", tpID,
							"error", err.Error())
					}
				}
			}

			slSide := "SELL"
			if posSide == "SHORT" {
				slSide = "BUY"
			}

			slParams := binance.AlgoOrderParams{
				Symbol:       posSymbol,
				Side:         slSide,
				Type:         "STOP_MARKET",
				TriggerPrice: slPrice,
				Quantity:     qty,
				ReduceOnly:   true,
			}

			// Place SL with retry logic - CRITICAL for position protection
			const maxSLRetries = 3
			slRetryDelay := 500 * time.Millisecond
			var slOrderPlaced bool

			for attempt := 1; attempt <= maxSLRetries; attempt++ {
				if slOrder, err := ga.futuresClient.PlaceAlgoOrder(slParams); err == nil && slOrder != nil && slOrder.AlgoId > 0 {
					pos.StopLossAlgoID = slOrder.AlgoId
					ga.logger.Info("SLTP: SL order placed", "symbol", posSymbol, "price", slPrice, "attempt", attempt)
					slOrderPlaced = true
					break
				} else {
					ga.logger.Error("SLTP: Failed to place SL order",
						"symbol", posSymbol,
						"attempt", attempt,
						"max_retries", maxSLRetries,
						"error", err.Error())
					if attempt < maxSLRetries {
						time.Sleep(slRetryDelay * time.Duration(attempt))
					}
				}
			}

			if !slOrderPlaced {
				ga.logger.Error("CRITICAL: SL order NOT placed after all retries - position unprotected!",
					"symbol", posSymbol,
					"sl_price", slPrice)
			}

			tpSide := "SELL"
			if posSide == "SHORT" {
				tpSide = "BUY"
			}

			newTPIDs := []int64{}

			// Place TP orders with retry logic
			const maxTPRetries = 3
			tpRetryDelay := 500 * time.Millisecond

			for i, tpPrice := range tpPrices {
				tpQty := tpQuantities[i]

				tpParams := binance.AlgoOrderParams{
					Symbol:       posSymbol,
					Side:         tpSide,
					Type:         "TAKE_PROFIT_MARKET",
					TriggerPrice: tpPrice,
					Quantity:     tpQty,
					ReduceOnly:   true,
				}

				var tpOrderPlaced bool
				for attempt := 1; attempt <= maxTPRetries; attempt++ {
					if tpOrder, err := ga.futuresClient.PlaceAlgoOrder(tpParams); err == nil && tpOrder != nil && tpOrder.AlgoId > 0 {
						newTPIDs = append(newTPIDs, tpOrder.AlgoId)
						ga.logger.Info("SLTP: TP order placed", "symbol", posSymbol, "level", i+1, "price", tpPrice, "qty", tpQty, "attempt", attempt)
						tpOrderPlaced = true
						break
					} else {
						ga.logger.Error("SLTP: Failed to place TP order",
							"symbol", posSymbol,
							"level", i+1,
							"attempt", attempt,
							"max_retries", maxTPRetries,
							"error", err.Error())
						if attempt < maxTPRetries {
							time.Sleep(tpRetryDelay * time.Duration(attempt))
						}
					}
				}

				if !tpOrderPlaced {
					ga.logger.Error("CRITICAL: TP order NOT placed after all retries - missing profit protection!",
						"symbol", posSymbol,
						"tp_level", i+1,
						"tp_price", tpPrice)
				}
			}

			pos.TakeProfitAlgoIDs = newTPIDs
		}()

		updated++

		ga.logger.Info("Adaptive SL/TP applied to position",
			"symbol", symbol,
			"side", pos.Side,
			"mode", pos.Mode,
			"entry", pos.EntryPrice,
			"old_sl", oldSL,
			"new_sl", pos.StopLoss,
			"sl_pct", fmt.Sprintf("%.2f%%", finalSLPct),
			"tp_pct", fmt.Sprintf("%.2f%%", finalTPPct),
			"llm_used", llmUsed)

	// Log individual TP levels dynamically to support both single TP and multi-TP modes
	for i, tp := range pos.TakeProfits {
		ga.logger.Debug("TP level configured", "symbol", symbol, "level", i+1, "price", fmt.Sprintf("%.2f", tp.Price), "percent", fmt.Sprintf("%.0f%%", tp.Percent))
	}
	}

	ga.logger.Info("Adaptive SL/TP recalculation completed", "updated", updated, "total_positions", len(ga.positions))
	return updated, nil
}

// GetSLTPJobQueue returns the SLTP job queue
func (ga *GinieAutopilot) GetSLTPJobQueue() *SLTPJobQueue {
	return ga.sltpJobQueue
}

// RecalculateAdaptiveSLTPAsync starts an async SLTP recalculation job and returns immediately with job ID
// The actual processing happens in background with progress tracking
func (ga *GinieAutopilot) RecalculateAdaptiveSLTPAsync() string {
	ga.mu.RLock()
	positions := make([]*GiniePosition, 0, len(ga.positions))
	for _, pos := range ga.positions {
		positions = append(positions, pos)
	}
	ga.mu.RUnlock()

	// Create job
	job := ga.sltpJobQueue.CreateJob(len(positions))

	// Process in background
	go ga.processAsyncSLTPRecalculation(job.ID, positions)

	return job.ID
}

// processAsyncSLTPRecalculation processes SLTP recalculation in background with progress tracking
func (ga *GinieAutopilot) processAsyncSLTPRecalculation(jobID string, positions []*GiniePosition) {
	// Start the job
	ga.sltpJobQueue.StartJob(jobID)

	successCount := 0
	failedCount := 0
	results := make([]*GiniePosition, 0, len(positions))

	// Process positions sequentially (could be parallelized for better performance)
	for idx, pos := range positions {
		if pos == nil {
			continue
		}

		// Update progress
		ga.sltpJobQueue.UpdateJobProgress(jobID, pos.Symbol, idx, successCount, failedCount)

		// Process this position
		if err := ga.recalculateSinglePositionSLTP(pos); err != nil {
			ga.logger.Error("Failed to recalculate SL/TP for position",
				"symbol", pos.Symbol,
				"error", err.Error())
			failedCount++
		} else {
			successCount++
			results = append(results, pos)
		}
	}

	// Mark job as completed
	ga.sltpJobQueue.CompleteJob(jobID, results)

	ga.logger.Info("Async SLTP recalculation completed",
		"job_id", jobID,
		"total", len(positions),
		"success", successCount,
		"failed", failedCount,
		"elapsed_seconds", ga.sltpJobQueue.GetJob(jobID).ElapsedSeconds)
}

// recalculateSinglePositionSLTP recalculates SL/TP for a single position
func (ga *GinieAutopilot) recalculateSinglePositionSLTP(pos *GiniePosition) error {
	symbol := pos.Symbol

	// Get klines
	klines, err := ga.futuresClient.GetFuturesKlines(symbol, "1h", 200)
	if err != nil {
		return fmt.Errorf("failed to fetch klines: %w", err)
	}

	if len(klines) < 50 {
		return fmt.Errorf("insufficient klines for analysis")
	}

	// Get settings
	sm := GetSettingsManager()
	settings := sm.GetCurrentSettings()

	// Determine mode for this position
	mode := pos.Mode
	if mode == "" {
		mode = "swing" // Default
	}

	// Get manual SL/TP override if set from ModeConfigs
	var manualSL, manualTP float64
	modeToConfigKey := map[string]string{
		"ultra_fast": "ultra_fast",
		"scalp":      "scalp",
		"swing":      "swing",
		"position":   "position",
	}
	if modeKey, ok := modeToConfigKey[string(mode)]; ok {
		if modeConfig := settings.ModeConfigs[modeKey]; modeConfig != nil {
			if modeConfig.SLTP != nil {
				manualSL = modeConfig.SLTP.StopLossPercent
				manualTP = modeConfig.SLTP.TakeProfitPercent
			}
		}
	}

	var finalSLPct, finalTPPct float64

	// Use manual override if set, otherwise calculate ATR-based
	if manualSL > 0 && manualTP > 0 {
		finalSLPct = manualSL
		finalTPPct = manualTP
	} else {
		// Simple ATR calculation as fallback
		atrPct := ga.calculateATRPercent(klines)
		finalSLPct = atrPct * 1.5
		finalTPPct = atrPct * 3.0
	}

	// Calculate SL price
	if pos.Side == "LONG" {
		pos.StopLoss = pos.EntryPrice * (1 - finalSLPct/100)
	} else {
		pos.StopLoss = pos.EntryPrice * (1 + finalSLPct/100)
	}

	// Calculate TP levels based on TP mode (single vs multi)
	// Direction: +1 for LONG (price goes up for profit), -1 for SHORT (price goes down)
	direction := 1.0
	if pos.Side == "SHORT" {
		direction = -1.0
	}

	// Check single vs multi TP mode from ModeConfigs
	useSingleTP := false
	if modeKey, ok := modeToConfigKey[string(mode)]; ok {
		if modeConfig := settings.ModeConfigs[modeKey]; modeConfig != nil {
			if modeConfig.SLTP != nil {
				useSingleTP = modeConfig.SLTP.UseSingleTP
			}
		}
	}

	if useSingleTP {
		// Single TP mode: Close 100% at one level
		tpPrice := pos.EntryPrice * (1 + direction*finalTPPct/100)
		pos.TakeProfits = []GinieTakeProfitLevel{
			{Level: 1, Percent: 100, GainPct: finalTPPct, Price: tpPrice, Status: "pending"},
		}
		ga.logger.Debug("Single TP mode applied (async)",
			"symbol", pos.Symbol, "mode", mode, "tp_price", tpPrice)
	} else {
		// Multi-TP mode: Use per-mode TPAllocation from mode_configs, fallback to global settings
		var tp1Pct, tp2Pct, tp3Pct, tp4Pct float64

		// Try to get from mode_configs first
		modeConfig := ga.getModeConfig(pos.Mode)
		if modeConfig != nil && modeConfig.SLTP != nil && len(modeConfig.SLTP.TPAllocation) >= 4 {
			tp1Pct = modeConfig.SLTP.TPAllocation[0]
			tp2Pct = modeConfig.SLTP.TPAllocation[1]
			tp3Pct = modeConfig.SLTP.TPAllocation[2]
			tp4Pct = modeConfig.SLTP.TPAllocation[3]
			ga.logger.Debug("Using mode_configs TPAllocation (async)",
				"symbol", pos.Symbol,
				"mode", pos.Mode,
				"allocation", modeConfig.SLTP.TPAllocation)
		} else {
			// Fallback to defaults (legacy fields removed)
			tp1Pct, tp2Pct, tp3Pct, tp4Pct = 25, 25, 25, 25
		}

		// Ensure allocation sums to 100%
		totalAlloc := tp1Pct + tp2Pct + tp3Pct + tp4Pct
		if totalAlloc < 99 || totalAlloc > 101 {
			ga.logger.Warn("TP allocation doesn't sum to 100%, using defaults (async)",
				"symbol", pos.Symbol,
				"total", totalAlloc)
			tp1Pct, tp2Pct, tp3Pct, tp4Pct = 25, 25, 25, 25
		}

		// Calculate actual gain % for each level based on allocation
		tp1Gain := finalTPPct * (tp1Pct / 100)
		tp2Gain := finalTPPct * (tp2Pct / 100)
		tp3Gain := finalTPPct * (tp3Pct / 100)
		tp4Gain := finalTPPct * (tp4Pct / 100)

		pos.TakeProfits = []GinieTakeProfitLevel{
			{Level: 1, Percent: tp1Pct, GainPct: tp1Gain, Price: pos.EntryPrice * (1 + direction*tp1Gain/100), Status: "pending"},
			{Level: 2, Percent: tp2Pct, GainPct: tp2Gain, Price: pos.EntryPrice * (1 + direction*tp2Gain/100), Status: "pending"},
			{Level: 3, Percent: tp3Pct, GainPct: tp3Gain, Price: pos.EntryPrice * (1 + direction*tp3Gain/100), Status: "pending"},
			{Level: 4, Percent: tp4Pct, GainPct: tp4Gain, Price: pos.EntryPrice * (1 + direction*tp4Gain/100), Status: "pending"},
		}
		ga.logger.Debug("Multi-TP mode applied (async)",
			"symbol", pos.Symbol, "allocation", fmt.Sprintf("%.0f%%/%.0f%%/%.0f%%/%.0f%%", tp1Pct, tp2Pct, tp3Pct, tp4Pct))
	}

	// Actually place the SL/TP orders on Binance
	ga.placeSLTPOrders(pos)

	return nil
}

// calculateATRPercent calculates ATR as a percentage of current price
func (ga *GinieAutopilot) calculateATRPercent(klines []binance.Kline) float64 {
	if len(klines) < 14 {
		return 1.0 // Default 1%
	}

	// Calculate ATR (14 period)
	var trSum float64
	for i := 1; i < len(klines) && i <= 14; i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := high - prevClose
		if tr2 < 0 {
			tr2 = -tr2
		}
		tr3 := low - prevClose
		if tr3 < 0 {
			tr3 = -tr3
		}

		tr := tr1
		if tr2 > tr {
			tr = tr2
		}
		if tr3 > tr {
			tr = tr3
		}
		trSum += tr
	}

	atr := trSum / 14.0
	currentPrice := klines[len(klines)-1].Close
	if currentPrice == 0 {
		return 1.0
	}

	return (atr / currentPrice) * 100
}

// calculateATR returns absolute ATR value (not percentage)
func (ga *GinieAutopilot) calculateATR(klines []binance.Kline) float64 {
	if len(klines) < 14 {
		return 0
	}

	var trSum float64
	for i := 1; i < len(klines) && i <= 14; i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		tr := math.Max(tr1, math.Max(tr2, tr3))
		trSum += tr
	}

	return trSum / 14.0
}

// validateSLUpdate validates LLM SL recommendation against our strict rules
// Returns (isValid, rejectionReason)
// Rules:
// 1. Never widen SL (LONG: new SL >= current SL, SHORT: new SL <= current SL)
// 2. Max SL move per update < 10%
// 3. Min distance from current price = ATR * 0.5
func (ga *GinieAutopilot) validateSLUpdate(pos *GiniePosition, newSL, currentPrice float64, klines []binance.Kline) (bool, string) {
	currentSL := pos.StopLoss

	// Skip validation if this is initial SL setup
	if currentSL <= 0 {
		return true, ""
	}

	// Rule 1: Never widen SL
	if pos.Side == "LONG" {
		// For LONG: SL must move UP (tighter), so new SL >= current SL
		if newSL < currentSL {
			return false, fmt.Sprintf("Rule 1: Cannot widen SL for LONG (new %.6f < current %.6f)", newSL, currentSL)
		}
	} else {
		// For SHORT: SL must move DOWN (tighter), so new SL <= current SL
		if newSL > currentSL {
			return false, fmt.Sprintf("Rule 1: Cannot widen SL for SHORT (new %.6f > current %.6f)", newSL, currentSL)
		}
	}

	// Rule 2: Max 10% move per update
	movePercent := math.Abs(newSL-currentSL) / currentSL * 100
	if movePercent > 10.0 {
		return false, fmt.Sprintf("Rule 2: SL move %.2f%% exceeds 10%% max", movePercent)
	}

	// Rule 3: Min distance = ATR * 0.5
	atr := ga.calculateATR(klines)
	if atr <= 0 {
		// If we can't calculate ATR, skip this rule
		return true, ""
	}

	minDistance := atr * 0.5 // Half ATR minimum buffer

	if pos.Side == "LONG" {
		distance := currentPrice - newSL
		if distance < minDistance {
			return false, fmt.Sprintf("Rule 3: SL too close to price (distance %.6f < min %.6f ATR)", distance, minDistance)
		}
	} else {
		distance := newSL - currentPrice
		if distance < minDistance {
			return false, fmt.Sprintf("Rule 3: SL too close to price (distance %.6f < min %.6f ATR)", distance, minDistance)
		}
	}

	return true, ""
}

// recordBadLLMCall records a bad LLM SL recommendation and activates kill switch if threshold reached
func (ga *GinieAutopilot) recordBadLLMCall(symbol string) {
	ga.badLLMCallCount[symbol]++
	count := ga.badLLMCallCount[symbol]

	if count >= 3 {
		ga.llmSLDisabled[symbol] = true
		ga.logger.Warn("Kill switch ACTIVATED - LLM SL updates disabled for symbol",
			"symbol", symbol,
			"consecutive_bad_calls", count)
	} else {
		ga.logger.Info("Bad LLM SL call recorded",
			"symbol", symbol,
			"bad_calls", count,
			"threshold", 3)
	}
}

// resetBadLLMCount resets the bad LLM call counter for a symbol (called on successful update)
func (ga *GinieAutopilot) resetBadLLMCount(symbol string) {
	if ga.badLLMCallCount[symbol] > 0 {
		ga.logger.Debug("Resetting bad LLM call counter", "symbol", symbol, "was", ga.badLLMCallCount[symbol])
		ga.badLLMCallCount[symbol] = 0
	}
	// Note: Does NOT auto-enable llmSLDisabled - requires manual intervention via API
}

// ResetLLMSLForSymbol resets the kill switch for a specific symbol (manual intervention)
func (ga *GinieAutopilot) ResetLLMSLForSymbol(symbol string) bool {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	wasDisabled := ga.llmSLDisabled[symbol]
	ga.llmSLDisabled[symbol] = false
	ga.badLLMCallCount[symbol] = 0

	if wasDisabled {
		ga.logger.Info("Kill switch RESET - LLM SL updates re-enabled for symbol", "symbol", symbol)
	}

	return wasDisabled
}

// GetLLMSLStatus returns the LLM SL status for all symbols (for API)
func (ga *GinieAutopilot) GetLLMSLStatus() map[string]interface{} {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	status := make(map[string]interface{})
	disabledSymbols := []string{}
	badCallCounts := make(map[string]int)

	for symbol, disabled := range ga.llmSLDisabled {
		if disabled {
			disabledSymbols = append(disabledSymbols, symbol)
		}
	}

	for symbol, count := range ga.badLLMCallCount {
		if count > 0 {
			badCallCounts[symbol] = count
		}
	}

	status["disabled_symbols"] = disabledSymbols
	status["bad_call_counts"] = badCallCounts
	status["kill_switch_threshold"] = 3

	return status
}

// ==================== Signal Logging Functions ====================

// LogSignal logs a new signal with its status (executed/rejected)
func (ga *GinieAutopilot) LogSignal(signal *GinieSignalLog) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	// Generate unique ID
	signal.ID = fmt.Sprintf("%s_%d", signal.Symbol, time.Now().UnixNano())
	signal.Timestamp = time.Now()

	// Add to logs
	ga.signalLogs = append(ga.signalLogs, *signal)

	// Trim if over limit
	if len(ga.signalLogs) > ga.maxSignalLogs {
		ga.signalLogs = ga.signalLogs[len(ga.signalLogs)-ga.maxSignalLogs:]
	}

	// Log to system logger as well
	status := signal.Status
	if signal.Status == "rejected" && signal.RejectionReason != "" {
		status = fmt.Sprintf("rejected (%s)", signal.RejectionReason)
	}

	ga.logger.Info("Signal logged",
		"symbol", signal.Symbol,
		"direction", signal.Direction,
		"mode", signal.Mode,
		"confidence", signal.Confidence,
		"status", status,
		"entry", signal.EntryPrice,
		"sl", signal.StopLoss,
		"tp1", signal.TakeProfit1)
}

// GetSignalLogs returns recent signal logs
func (ga *GinieAutopilot) GetSignalLogs(limit int) []GinieSignalLog {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	if limit <= 0 || limit > len(ga.signalLogs) {
		limit = len(ga.signalLogs)
	}

	// Return most recent signals (reversed order - newest first)
	result := make([]GinieSignalLog, limit)
	for i := 0; i < limit; i++ {
		result[i] = ga.signalLogs[len(ga.signalLogs)-1-i]
	}

	return result
}

// GetSignalStats returns signal statistics for the last hour (consistent with diagnostics)
func (ga *GinieAutopilot) GetSignalStats() map[string]interface{} {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	stats := make(map[string]interface{})
	total := 0
	executed := 0
	rejected := 0
	rejectionReasons := make(map[string]int)

	// Only count signals from the last hour to be consistent with diagnostics
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	for _, sig := range ga.signalLogs {
		// Skip signals older than 1 hour
		if sig.Timestamp.Before(oneHourAgo) {
			continue
		}
		total++
		switch sig.Status {
		case "executed":
			executed++
		case "rejected":
			rejected++
			if sig.RejectionReason != "" {
				rejectionReasons[sig.RejectionReason]++
			}
		}
	}

	stats["total"] = total
	stats["executed"] = executed
	stats["rejected"] = rejected
	stats["execution_rate"] = 0.0
	if total > 0 {
		stats["execution_rate"] = float64(executed) / float64(total) * 100
	}
	stats["rejection_reasons"] = rejectionReasons

	return stats
}

// ==================== SL Update History Functions ====================

// RecordSLUpdate records an SL update attempt for a position
func (ga *GinieAutopilot) RecordSLUpdate(symbol string, oldSL, newSL, currentPrice float64, status, rejectionRule, source string, llmConfidence float64) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	// Initialize history if needed
	if ga.slUpdateHistory[symbol] == nil {
		ga.slUpdateHistory[symbol] = &SLUpdateHistory{
			Symbol:  symbol,
			Updates: make([]SLUpdateRecord, 0, 100),
		}
	}

	history := ga.slUpdateHistory[symbol]

	// Add record
	record := SLUpdateRecord{
		Timestamp:     time.Now(),
		OldSL:         oldSL,
		NewSL:         newSL,
		CurrentPrice:  currentPrice,
		Status:        status,
		RejectionRule: rejectionRule,
		Source:        source,
		LLMConfidence: llmConfidence,
	}

	history.Updates = append(history.Updates, record)
	history.TotalAttempts++

	if status == "applied" {
		history.Applied++
	} else {
		history.Rejected++
	}

	// Trim if over limit (keep last 100 updates per symbol)
	if len(history.Updates) > 100 {
		history.Updates = history.Updates[len(history.Updates)-100:]
	}
}

// GetSLUpdateHistory returns SL update history for a symbol
func (ga *GinieAutopilot) GetSLUpdateHistory(symbol string) *SLUpdateHistory {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	if history := ga.slUpdateHistory[symbol]; history != nil {
		// Return a copy
		copy := &SLUpdateHistory{
			Symbol:        history.Symbol,
			TotalAttempts: history.TotalAttempts,
			Applied:       history.Applied,
			Rejected:      history.Rejected,
			Updates:       make([]SLUpdateRecord, len(history.Updates)),
		}
		for i, u := range history.Updates {
			copy.Updates[i] = u
		}
		return copy
	}

	return nil
}

// GetAllSLUpdateHistory returns SL update history for all positions
func (ga *GinieAutopilot) GetAllSLUpdateHistory() map[string]*SLUpdateHistory {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	result := make(map[string]*SLUpdateHistory)
	for symbol, history := range ga.slUpdateHistory {
		// Return copies
		copy := &SLUpdateHistory{
			Symbol:        history.Symbol,
			TotalAttempts: history.TotalAttempts,
			Applied:       history.Applied,
			Rejected:      history.Rejected,
			Updates:       make([]SLUpdateRecord, len(history.Updates)),
		}
		for i, u := range history.Updates {
			copy.Updates[i] = u
		}
		result[symbol] = copy
	}

	return result
}

// GetSLUpdateStats returns overall SL update statistics
func (ga *GinieAutopilot) GetSLUpdateStats() map[string]interface{} {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	stats := make(map[string]interface{})
	totalAttempts := 0
	totalApplied := 0
	totalRejected := 0
	rejectionsByRule := make(map[string]int)

	for _, history := range ga.slUpdateHistory {
		totalAttempts += history.TotalAttempts
		totalApplied += history.Applied
		totalRejected += history.Rejected

		for _, update := range history.Updates {
			if update.Status == "rejected" && update.RejectionRule != "" {
				rejectionsByRule[update.RejectionRule]++
			}
		}
	}

	stats["total_attempts"] = totalAttempts
	stats["applied"] = totalApplied
	stats["rejected"] = totalRejected
	stats["approval_rate"] = 0.0
	if totalAttempts > 0 {
		stats["approval_rate"] = float64(totalApplied) / float64(totalAttempts) * 100
	}
	stats["rejections_by_rule"] = rejectionsByRule
	stats["symbols_tracked"] = len(ga.slUpdateHistory)

	return stats
}

// CloseAllPositions closes all Ginie-managed positions (Ginie panic button)
func (ga *GinieAutopilot) CloseAllPositions() (int, float64, error) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	closedCount := 0
	totalPnL := 0.0

	for symbol, pos := range ga.positions {
		// Get current price
		currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
		if err != nil {
			ga.logger.Error("Failed to get price for panic close", "symbol", symbol, "error", err)
			continue
		}

		// Calculate PnL (both USD and percentage for circuit breaker)
		var pnl float64
		var pnlPercent float64
		if pos.Side == "LONG" {
			pnl = (currentPrice - pos.EntryPrice) * pos.RemainingQty
			pnlPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
		} else {
			pnl = (pos.EntryPrice - currentPrice) * pos.RemainingQty
			pnlPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
		}
		pnl += pos.RealizedPnL

		ga.logger.Info("Ginie panic closing position",
			"symbol", symbol,
			"remaining_qty", pos.RemainingQty,
			"pnl", pnl)

		if !ga.config.DryRun && pos.RemainingQty > 0 {
			// Place close order
			side := "SELL"
			positionSide := binance.PositionSideLong
			if pos.Side == "SHORT" {
				side = "BUY"
				positionSide = binance.PositionSideShort
			}

			// Check actual Binance position mode to avoid API error -4061
			effectivePositionSide := ga.getEffectivePositionSide(positionSide)

			orderParams := binance.FuturesOrderParams{
				Symbol:       symbol,
				Side:         side,
				PositionSide: effectivePositionSide,
				Type:         binance.FuturesOrderTypeMarket,
				Quantity:     pos.RemainingQty,
			}

			_, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
			if err != nil {
				ga.logger.Error("Ginie panic close failed", "symbol", symbol, "error", err)
				continue
			}
		}

		// Update tracking
		ga.dailyPnL += pnl - pos.RealizedPnL
		ga.totalPnL += pnl - pos.RealizedPnL
		totalPnL += pnl

		// Record to circuit breaker - uses PERCENTAGE not USD
		if ga.config.CircuitBreakerEnabled && ga.circuitBreaker != nil {
			ga.circuitBreaker.RecordTrade(pnlPercent)
		}

		// Record trade
		ga.recordTrade(GinieTradeResult{
			Symbol:    symbol,
			Action:    "panic_close",
			Side:      pos.Side,
			Quantity:  pos.RemainingQty,
			Price:     currentPrice,
			PnL:       pnl,
			Reason:    "Ginie panic button - manual close all",
			Timestamp: time.Now(),
			Mode:      pos.Mode,
		})

		closedCount++
	}

	// Clear all positions
	ga.positions = make(map[string]*GiniePosition)

	// Persist PnL stats after panic close
	go ga.SavePnLStats()

	ga.logger.Info("Ginie panic close complete",
		"positions_closed", closedCount,
		"total_pnl", totalPnL)

	return closedCount, totalPnL, nil
}

// ==================== Diagnostic Methods ====================

// GetDiagnostics returns comprehensive troubleshooting information
func (ga *GinieAutopilot) GetDiagnostics() *GinieDiagnostics {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	diag := &GinieDiagnostics{
		Timestamp:        time.Now(),
		AutopilotRunning: ga.running,
		IsLiveMode:       !ga.config.DryRun,
	}

	// Check canTrade and capture reason
	canTrade, reason := ga.canTradeWithReasonLocked()
	diag.CanTrade = canTrade
	diag.CanTradeReason = reason

	// Circuit breaker status
	diag.CircuitBreaker = ga.getCircuitBreakerDiagnosticsLocked()

	// Position status
	diag.Positions = PositionDiagnostics{
		OpenCount:      len(ga.positions),
		MaxAllowed:     ga.config.MaxPositions,
		SlotsAvailable: ga.config.MaxPositions - len(ga.positions),
	}
	// Calculate total unrealized PnL
	for _, pos := range ga.positions {
		diag.Positions.TotalUnrealizedPnL += pos.UnrealizedPnL
	}

	// Scanning status
	diag.Scanning = ga.getScanDiagnosticsLocked()

	// Signal stats
	diag.Signals = ga.getSignalDiagnosticsLocked()

	// Profit booking
	diag.ProfitBooking = ga.getProfitDiagnosticsLocked()

	// Blocked coins
	for _, info := range ga.blockedCoins {
		diag.BlockedCoins = append(diag.BlockedCoins, info)
	}

	// LLM status
	diag.LLMStatus = ga.getLLMDiagnosticsLocked()

	// Generate issue recommendations
	diag.Issues = ga.generateIssueRecommendationsLocked(diag)

	return diag
}

// canTradeWithReasonLocked returns whether trading is allowed and why (must hold lock)
func (ga *GinieAutopilot) canTradeWithReasonLocked() (bool, string) {
	// Check if autopilot is running
	if !ga.running {
		return false, "autopilot_stopped: Ginie autopilot is not running"
	}

	// Circuit breaker check
	if ga.config.CircuitBreakerEnabled && ga.circuitBreaker != nil {
		canTrade, reason := ga.circuitBreaker.CanTrade()
		if !canTrade {
			return false, "circuit_breaker: " + reason
		}
	}

	// Position limit check
	if len(ga.positions) >= ga.config.MaxPositions {
		return false, fmt.Sprintf("max_positions: %d/%d slots used",
			len(ga.positions), ga.config.MaxPositions)
	}

	// Daily trade limit check
	if ga.dailyTrades >= ga.config.MaxDailyTrades {
		return false, fmt.Sprintf("daily_trades: %d/%d limit reached",
			ga.dailyTrades, ga.config.MaxDailyTrades)
	}

	// Daily loss limit check
	if ga.dailyPnL <= -ga.config.MaxDailyLoss {
		return false, fmt.Sprintf("daily_loss: $%.2f exceeds limit $%.2f",
			-ga.dailyPnL, ga.config.MaxDailyLoss)
	}

	// Check if any mode is enabled
	if !ga.config.EnableScalpMode && !ga.config.EnableSwingMode && !ga.config.EnablePositionMode {
		return false, "no_modes: No trading modes enabled (scalp/swing/position)"
	}

	return true, "ok"
}

// getCircuitBreakerDiagnosticsLocked returns CB status (must hold lock)
func (ga *GinieAutopilot) getCircuitBreakerDiagnosticsLocked() CBDiagnostics {
	diag := CBDiagnostics{
		Enabled:         ga.config.CircuitBreakerEnabled,
		State:           "closed",
		HourlyLossLimit: ga.config.CBMaxLossPerHour,
		DailyLossLimit:  ga.config.CBMaxDailyLoss,
	}

	if ga.circuitBreaker == nil {
		return diag
	}

	// Get circuit breaker state from string
	diag.State = string(ga.circuitBreaker.GetState())

	// Get detailed stats
	stats := ga.circuitBreaker.GetStats()
	if stats != nil {
		if hourlyLoss, ok := stats["hourly_loss"].(float64); ok {
			diag.HourlyLoss = hourlyLoss
		}
		if dailyLoss, ok := stats["daily_loss"].(float64); ok {
			diag.DailyLoss = dailyLoss
		}
		if consecLosses, ok := stats["consecutive_losses"].(int); ok {
			diag.ConsecutiveLosses = consecLosses
		}

		// Check for cooldown if state is open
		if diag.State == "open" {
			if lastTripTime, ok := stats["last_trip_time"].(time.Time); ok && !lastTripTime.IsZero() {
				cooldownMins := ga.config.CBCooldownMinutes
				cooldownEnd := lastTripTime.Add(time.Duration(cooldownMins) * time.Minute)
				if cooldownEnd.After(time.Now()) {
					remaining := time.Until(cooldownEnd)
					diag.CooldownRemaining = fmt.Sprintf("%dm %ds", int(remaining.Minutes()), int(remaining.Seconds())%60)
				}
			}
		}
	}

	return diag
}

// getScanDiagnosticsLocked returns scan activity info (must hold lock)
func (ga *GinieAutopilot) getScanDiagnosticsLocked() ScanDiagnostics {
	// Get the most recent scan time
	lastScan := ga.lastScalpScan
	if ga.lastSwingScan.After(lastScan) {
		lastScan = ga.lastSwingScan
	}
	if ga.lastPositionScan.After(lastScan) {
		lastScan = ga.lastPositionScan
	}

	symbolsCount := 0
	if ga.analyzer != nil {
		symbolsCount = len(ga.analyzer.watchSymbols)
	}

	// Handle uninitialized timestamps (zero time) - show 0 seconds since last scan
	// instead of garbage values (billions of seconds since epoch)
	var secondsSinceLastScan int64 = 0
	if !lastScan.IsZero() {
		secondsSinceLastScan = int64(time.Since(lastScan).Seconds())
	}

	// Calculate time until next scan
	var timeUntilNext int64
	if !ga.nextScanTime.IsZero() && ga.nextScanTime.After(time.Now()) {
		timeUntilNext = int64(time.Until(ga.nextScanTime).Seconds())
	}

	return ScanDiagnostics{
		LastScanTime:         lastScan,
		SecondsSinceLastScan: secondsSinceLastScan,
		SymbolsInWatchlist:   symbolsCount,
		SymbolsScannedLast:   ga.symbolsScannedLastCycle,
		ScalpEnabled:         ga.config.EnableScalpMode,
		SwingEnabled:         ga.config.EnableSwingMode,
		PositionEnabled:      ga.config.EnablePositionMode,
		// New scan status fields (Issue 2B)
		ScanningActive:    ga.scanningActive,
		CurrentPhase:      ga.currentPhase,
		TimeUntilNextScan: timeUntilNext,
		ScannedThisCycle:  ga.scannedThisCycle,
		TotalSymbols:      ga.totalSymbols,
		LastScanDuration:  ga.scanDuration.Milliseconds(),
		NextScanTime:      ga.nextScanTime,
	}
}

// GetScanStatus returns current scan status information (Issue 2B)
// This method is thread-safe and can be called from API handlers
func (ga *GinieAutopilot) GetScanStatus() map[string]interface{} {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	var timeUntilNext int64
	if !ga.nextScanTime.IsZero() && ga.nextScanTime.After(time.Now()) {
		timeUntilNext = int64(time.Until(ga.nextScanTime).Seconds())
	}

	return map[string]interface{}{
		"scanning_active":             ga.scanningActive,
		"current_phase":               ga.currentPhase,
		"time_until_next_scan_seconds": timeUntilNext,
		"progress":                    fmt.Sprintf("%d/%d", ga.scannedThisCycle, ga.totalSymbols),
		"last_scan_duration_ms":       ga.scanDuration.Milliseconds(),
		"last_scan_time":              ga.lastScanTime,
		"next_scan_time":              ga.nextScanTime,
	}
}

// getSignalDiagnosticsLocked returns signal generation stats (must hold lock)
func (ga *GinieAutopilot) getSignalDiagnosticsLocked() SignalDiagnostics {
	diag := SignalDiagnostics{
		TopRejectionReasons: make(map[string]int),
	}

	// Time filter for 1-hour window
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	for _, sig := range ga.signalLogs {
		// Count for all-time/session (no time filter)
		diag.TotalGeneratedAllTime++
		switch sig.Status {
		case "executed":
			diag.ExecutedAllTime++
		case "rejected":
			diag.RejectedAllTime++
		}

		// Count for last 1 hour only
		if sig.Timestamp.Before(oneHourAgo) {
			continue
		}
		diag.TotalGenerated++
		switch sig.Status {
		case "executed":
			diag.Executed++
		case "rejected":
			diag.Rejected++
			if sig.RejectionReason != "" {
				diag.TopRejectionReasons[sig.RejectionReason]++
			}
		}
	}

	// Calculate execution rates for both windows
	if diag.TotalGenerated > 0 {
		diag.ExecutionRate = float64(diag.Executed) / float64(diag.TotalGenerated) * 100
	}
	if diag.TotalGeneratedAllTime > 0 {
		diag.ExecutionRateAllTime = float64(diag.ExecutedAllTime) / float64(diag.TotalGeneratedAllTime) * 100
	}

	return diag
}

// getProfitDiagnosticsLocked returns profit booking status (must hold lock)
func (ga *GinieAutopilot) getProfitDiagnosticsLocked() ProfitDiagnostics {
	diag := ProfitDiagnostics{
		TPHitsLastHour:        ga.tpHitsLastHour,
		PartialClosesLastHour: ga.partialClosesLastHour,
		FailedClosesLastHour:  ga.failedOrdersLastHour,
	}

	// Count positions with pending TPs and trailing active
	for _, pos := range ga.positions {
		// Check if any TP is still pending
		for _, tp := range pos.TakeProfits {
			if tp.Status != "hit" {
				diag.PositionsWithPendingTP++
				break
			}
		}
		if pos.TrailingActive {
			diag.TrailingActiveCount++
		}
	}

	return diag
}

// getLLMDiagnosticsLocked returns LLM connection status (must hold lock)
func (ga *GinieAutopilot) getLLMDiagnosticsLocked() LLMDiagnostics {
	diag := LLMDiagnostics{
		DisabledSymbols: make([]string, 0),
	}

	// Check LLM availability
	if ga.llmAnalyzer != nil && ga.llmAnalyzer.IsEnabled() {
		diag.Connected = true
		if client := ga.llmAnalyzer.GetClient(); client != nil {
			diag.Provider = string(client.GetProvider())
		}
	}

	diag.LastCallTime = ga.lastLLMCallTime

	// Check analyzer coin list cache
	if ga.analyzer != nil {
		diag.CoinListCached = len(ga.analyzer.watchSymbols) > 0
		// Approximate cache age based on last scan
		if !ga.lastScalpScan.IsZero() {
			age := time.Since(ga.lastScalpScan)
			diag.CoinListAge = fmt.Sprintf("%dm", int(age.Minutes()))
		}
	}

	// Get LLM SL disabled symbols
	for symbol, disabled := range ga.llmSLDisabled {
		if disabled {
			diag.DisabledSymbols = append(diag.DisabledSymbols, symbol)
		}
	}

	return diag
}

// formatDurationHMS formats seconds into a human-readable duration string (e.g., "1h 30m 45s")
func formatDurationHMS(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		mins := seconds / 60
		secs := seconds % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := seconds / 3600
	mins := (seconds % 3600) / 60
	secs := seconds % 60
	if secs == 0 {
		if mins == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dh %dm %ds", hours, mins, secs)
}

// generateIssueRecommendationsLocked identifies problems and suggests fixes (must hold lock)
func (ga *GinieAutopilot) generateIssueRecommendationsLocked(diag *GinieDiagnostics) []DiagnosticIssue {
	var issues []DiagnosticIssue

	// Critical: Autopilot not running
	if !diag.AutopilotRunning {
		issues = append(issues, DiagnosticIssue{
			Severity:   "critical",
			Category:   "trading",
			Message:    "Ginie autopilot is not running",
			Suggestion: "Start Ginie autopilot from the UI or API",
		})
	}

	// Critical: Circuit breaker open
	if diag.CircuitBreaker.State == "open" {
		issues = append(issues, DiagnosticIssue{
			Severity:   "critical",
			Category:   "trading",
			Message:    fmt.Sprintf("Circuit breaker is OPEN (daily loss: $%.2f)", diag.CircuitBreaker.DailyLoss),
			Suggestion: fmt.Sprintf("Wait for cooldown (%s) or manually reset circuit breaker", diag.CircuitBreaker.CooldownRemaining),
		})
	}

	// Critical: All slots full
	if diag.Positions.SlotsAvailable == 0 {
		issues = append(issues, DiagnosticIssue{
			Severity:   "critical",
			Category:   "trading",
			Message:    fmt.Sprintf("All position slots full (%d/%d)", diag.Positions.OpenCount, diag.Positions.MaxAllowed),
			Suggestion: "Wait for positions to close or increase max_positions config",
		})
	}

	// Critical: No modes enabled
	if !diag.Scanning.ScalpEnabled && !diag.Scanning.SwingEnabled && !diag.Scanning.PositionEnabled {
		issues = append(issues, DiagnosticIssue{
			Severity:   "critical",
			Category:   "config",
			Message:    "No trading modes enabled",
			Suggestion: "Enable at least one mode: scalp, swing, or position trading",
		})
	}

	// Warning: Paper mode
	if !diag.IsLiveMode {
		issues = append(issues, DiagnosticIssue{
			Severity:   "warning",
			Category:   "config",
			Message:    "Running in PAPER mode - no real trades",
			Suggestion: "Switch to LIVE mode if you want real trading",
		})
	}

	// Warning: Many blocked coins
	if len(diag.BlockedCoins) > 5 {
		issues = append(issues, DiagnosticIssue{
			Severity:   "warning",
			Category:   "trading",
			Message:    fmt.Sprintf("%d coins are blocked from trading", len(diag.BlockedCoins)),
			Suggestion: "Review blocked coins and unblock if appropriate",
		})
	}

	// Warning: Low execution rate
	if diag.Signals.TotalGenerated > 10 && diag.Signals.ExecutionRate < 10 {
		issues = append(issues, DiagnosticIssue{
			Severity:   "warning",
			Category:   "signals",
			Message:    fmt.Sprintf("Low signal execution rate: %.1f%% (%d/%d)", diag.Signals.ExecutionRate, diag.Signals.Executed, diag.Signals.TotalGenerated),
			Suggestion: "Consider lowering confidence threshold or adjusting signal requirements",
		})
	}

	// Warning: No scanning for > 5 minutes when running
	// Note: SecondsSinceLastScan is 0 when no scans have happened yet (just started)
	if diag.AutopilotRunning && diag.Scanning.SecondsSinceLastScan > 300 && !diag.Scanning.LastScanTime.IsZero() {
		issues = append(issues, DiagnosticIssue{
			Severity:   "warning",
			Category:   "scanning",
			Message:    fmt.Sprintf("No scan activity for %s", formatDurationHMS(diag.Scanning.SecondsSinceLastScan)),
			Suggestion: "Check if autopilot loop is running correctly",
		})
	}

	// Info: Autopilot just started, waiting for first scan
	if diag.AutopilotRunning && diag.Scanning.LastScanTime.IsZero() {
		issues = append(issues, DiagnosticIssue{
			Severity:   "info",
			Category:   "scanning",
			Message:    "Waiting for first scan cycle",
			Suggestion: "First scan should complete within 30-60 seconds",
		})
	}

	// Warning: Empty watchlist
	if diag.Scanning.SymbolsInWatchlist == 0 {
		issues = append(issues, DiagnosticIssue{
			Severity:   "warning",
			Category:   "scanning",
			Message:    "No symbols in watchlist",
			Suggestion: "Add symbols to watch or enable auto coin discovery",
		})
	}

	// Warning: Failed orders
	if diag.ProfitBooking.FailedClosesLastHour > 0 {
		issues = append(issues, DiagnosticIssue{
			Severity:   "warning",
			Category:   "profit",
			Message:    fmt.Sprintf("%d failed close orders in last hour", diag.ProfitBooking.FailedClosesLastHour),
			Suggestion: "Check Binance API connectivity and order validation",
		})
	}

	// Warning: LLM disabled for symbols
	if len(diag.LLMStatus.DisabledSymbols) > 0 {
		issues = append(issues, DiagnosticIssue{
			Severity:   "warning",
			Category:   "config",
			Message:    fmt.Sprintf("LLM SL updates disabled for %d symbols (kill switch active)", len(diag.LLMStatus.DisabledSymbols)),
			Suggestion: "Reset LLM kill switch for affected symbols if LLM issues are resolved",
		})
	}

	// Info: Positions with pending TPs
	if diag.ProfitBooking.PositionsWithPendingTP > 0 {
		issues = append(issues, DiagnosticIssue{
			Severity:   "info",
			Category:   "profit",
			Message:    fmt.Sprintf("%d positions waiting for TP levels", diag.ProfitBooking.PositionsWithPendingTP),
			Suggestion: "TPs will trigger automatically when price reaches targets",
		})
	}

	// Info: LLM not connected
	if !diag.LLMStatus.Connected {
		issues = append(issues, DiagnosticIssue{
			Severity:   "info",
			Category:   "config",
			Message:    "LLM analyzer not connected",
			Suggestion: "Configure AI_LLM_PROVIDER and API key for adaptive SL/TP",
		})
	}

	// Info: Trailing stops active
	if diag.ProfitBooking.TrailingActiveCount > 0 {
		issues = append(issues, DiagnosticIssue{
			Severity:   "info",
			Category:   "profit",
			Message:    fmt.Sprintf("%d positions have trailing stops active", diag.ProfitBooking.TrailingActiveCount),
			Suggestion: "Trailing stops will close at trailing % below highest price",
		})
	}

	return issues
}

// scanStrategies evaluates all enabled saved strategies and executes trades for triggered signals
func (ga *GinieAutopilot) scanStrategies() {
	if ga.strategyEvaluator == nil {
		return
	}

	// Check if we can trade before scanning
	if !ga.canTrade() {
		return
	}

	// Evaluate all enabled strategies
	signals, err := ga.strategyEvaluator.EvaluateAllStrategies()
	if err != nil {
		ga.logger.Error("Failed to evaluate strategies", "error", err)
		return
	}

	if len(signals) == 0 {
		return
	}

	ga.logger.Info("Strategy evaluation complete", "triggered_signals", len(signals))

	// Execute trades for each triggered signal
	for _, signal := range signals {
		// Check if we already have a position for this symbol
		ga.mu.RLock()
		_, hasPosition := ga.positions[signal.Symbol]
		ga.mu.RUnlock()

		if hasPosition {
			ga.logger.Debug("Skipping strategy signal - position already exists",
				"symbol", signal.Symbol,
				"strategy", signal.StrategyName)
			continue
		}

		// Check circuit breaker
		if ga.config.CircuitBreakerEnabled && ga.circuitBreaker != nil {
			canTrade, reason := ga.circuitBreaker.CanTrade()
			if !canTrade {
				ga.logger.Warn("Strategy signal blocked by circuit breaker",
					"symbol", signal.Symbol,
					"strategy", signal.StrategyName,
					"reason", reason)
				continue
			}
		}

		// Check if we have room for more positions
		ga.mu.RLock()
		posCount := len(ga.positions)
		ga.mu.RUnlock()

		if posCount >= ga.config.MaxPositions {
			ga.logger.Warn("Strategy signal blocked - max positions reached",
				"symbol", signal.Symbol,
				"strategy", signal.StrategyName,
				"current", posCount,
				"max", ga.config.MaxPositions)
			break
		}

		// Execute the strategy trade
		ga.executeStrategyTrade(&signal)
	}
}

// executeStrategyTrade executes a trade based on a strategy signal
func (ga *GinieAutopilot) executeStrategyTrade(signal *StrategySignal) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	symbol := signal.Symbol

	// Double-check we don't already have a position
	if _, exists := ga.positions[symbol]; exists {
		return
	}

	// Check if coin is blocked due to big losses
	if blocked, reason := ga.isCoinBlocked(symbol); blocked {
		ga.logger.Warn("Strategy trade skipped - coin is blocked",
			"symbol", symbol,
			"strategy", signal.StrategyName,
			"reason", reason)
		return
	}

	// Check funding rate before entry (use user's enabled mode preference)
	isLong := signal.Side == "LONG"
	strategyMode := ga.selectEnabledModeForPosition() // Use user's enabled mode instead of hardcoded swing
	if blocked, reason := ga.checkFundingRate(symbol, isLong, strategyMode); blocked {
		ga.logger.Warn("Strategy trade skipped - funding rate concern",
			"symbol", symbol,
			"strategy", signal.StrategyName,
			"mode", strategyMode,
			"reason", reason,
			"side", signal.Side)
		return
	}

	// Calculate position size from strategy's configured percentage
	ga.mu.Unlock()
	availableBalance, err := ga.getAvailableBalance()
	ga.mu.Lock()

	// Re-check position after unlock
	if _, exists := ga.positions[symbol]; exists {
		ga.logger.Warn("Strategy race condition avoided - position created while sizing",
			"symbol", symbol)
		return
	}

	if err != nil {
		ga.logger.Error("Failed to get balance for strategy trade",
			"symbol", symbol,
			"error", err)
		return
	}

	// Use strategy's position size percentage (default to 5% if not set)
	positionPct := signal.PositionSize
	if positionPct <= 0 {
		positionPct = 5.0 // Default 5% of account
	}
	positionUSD := availableBalance * (positionPct / 100.0)

	// Apply max USD per position limit from Ginie config
	if ga.config.MaxUSDPerPosition > 0 && positionUSD > ga.config.MaxUSDPerPosition {
		positionUSD = ga.config.MaxUSDPerPosition
	}

	// Get current price
	price := signal.EntryPrice
	if price <= 0 {
		ga.mu.Unlock()
		priceResult, priceErr := ga.futuresClient.GetFuturesCurrentPrice(symbol)
		ga.mu.Lock()

		if priceErr != nil {
			ga.logger.Error("Failed to get price for strategy trade",
				"symbol", symbol,
				"error", priceErr)
			return
		}
		price = priceResult

		// Re-check position after unlock
		if _, exists := ga.positions[symbol]; exists {
			return
		}
	}

	// Use Ginie's default leverage
	leverage := ga.config.DefaultLeverage
	if leverage <= 0 {
		leverage = 10 // Default to 10x
	}

	// Calculate quantity
	quantity := (positionUSD * float64(leverage)) / price
	quantity = roundQuantity(symbol, quantity)

	if quantity <= 0 {
		ga.logger.Warn("Strategy calculated zero quantity",
			"symbol", symbol,
			"strategy", signal.StrategyName,
			"usd", positionUSD)
		return
	}

	// Determine side for Binance
	side := "BUY"
	positionSide := binance.PositionSideLong
	if signal.Side == "SHORT" {
		side = "SELL"
		positionSide = binance.PositionSideShort
	}

	// Check actual Binance position mode to avoid API error -4061
	effectivePositionSide := ga.getEffectivePositionSide(positionSide)

	ga.logger.Info("Executing strategy trade",
		"symbol", symbol,
		"strategy", signal.StrategyName,
		"side", signal.Side,
		"quantity", quantity,
		"leverage", leverage,
		"dry_run", ga.config.DryRun)

	// Variables for actual fill details
	actualPrice := price
	actualQty := quantity

	if !ga.config.DryRun {
		// Need to unlock for API calls
		ga.mu.Unlock()

		// Set leverage first
		_, err = ga.futuresClient.SetLeverage(symbol, leverage)
		if err != nil {
			ga.logger.Error("Failed to set leverage for strategy trade",
				"symbol", symbol,
				"error", err.Error())
			ga.mu.Lock()
			return
		}

		// Place market order
		orderParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         side,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     quantity,
		}

		order, orderErr := ga.futuresClient.PlaceFuturesOrder(orderParams)
		if orderErr != nil {
			ga.logger.Error("Strategy trade execution failed",
				"symbol", symbol,
				"strategy", signal.StrategyName,
				"error", orderErr.Error())
			ga.mu.Lock()
			return
		}

		// Verify order fill
		fillPrice, fillQty, fillErr := ga.verifyOrderFill(order, quantity)
		ga.mu.Lock()

		if fillErr != nil {
			ga.logger.Error("Strategy order fill verification failed",
				"symbol", symbol,
				"order_id", order.OrderId,
				"error", fillErr.Error())
			return
		}

		actualPrice = fillPrice
		actualQty = fillQty

		ga.logger.Info("Strategy trade executed and verified",
			"symbol", symbol,
			"strategy", signal.StrategyName,
			"order_id", order.OrderId,
			"side", side,
			"filled_qty", actualQty,
			"fill_price", actualPrice)
	} else {
		ga.logger.Info("Strategy trade (DRY RUN)",
			"symbol", symbol,
			"strategy", signal.StrategyName,
			"side", side,
			"quantity", quantity,
			"price", price)
	}

	// Generate default TPs based on mode (use swing as default for strategies)
	takeProfits := ga.generateDefaultTPs(symbol, actualPrice, GinieModeSwing, isLong)

	// Create strategy ID and name pointers
	stratID := signal.StrategyID
	stratName := signal.StrategyName

	// Create position record using user's enabled mode preference
	position := &GiniePosition{
		Symbol:                symbol,
		Side:                  signal.Side,
		Mode:                  strategyMode, // Use user's enabled mode preference
		EntryPrice:            actualPrice,
		OriginalQty:           actualQty,
		RemainingQty:          actualQty,
		Leverage:              leverage,
		EntryTime:             time.Now(),
		TakeProfits:           takeProfits,
		CurrentTPLevel:        0,
		StopLoss:              signal.StopLoss,
		OriginalSL:            signal.StopLoss,
		MovedToBreakeven:      false,
		TrailingActive:        false,
		HighestPrice:          actualPrice,
		LowestPrice:           actualPrice,
		TrailingPercent:       ga.getTrailingPercent(strategyMode),
		TrailingActivationPct: ga.getTrailingActivation(strategyMode),
		DecisionReport:        nil, // No AI decision report for strategy trades
		Source:                "strategy",
		StrategyID:            &stratID,
		StrategyName:          &stratName,
		Protection:            NewProtectionStatus(), // Initialize protection tracking
	}

	ga.positions[symbol] = position
	ga.dailyTrades++
	ga.totalTrades++

	// Place SL/TP orders on Binance (if not dry run)
	if !ga.config.DryRun {
		ga.mu.Unlock()
		ga.placeSLTPOrders(position)
		ga.mu.Lock()
	}

	// Build TP prices array
	tpPrices := make([]float64, len(takeProfits))
	for i, tp := range takeProfits {
		tpPrices[i] = tp.Price
	}

	// Calculate SL percent from entry
	slPercent := 0.0
	if isLong && signal.StopLoss > 0 {
		slPercent = ((actualPrice - signal.StopLoss) / actualPrice) * 100
	} else if !isLong && signal.StopLoss > 0 {
		slPercent = ((signal.StopLoss - actualPrice) / actualPrice) * 100
	}

	// Record trade with strategy info using user's enabled mode
	ga.recordTrade(GinieTradeResult{
		Symbol:    symbol,
		Action:    "open",
		Side:      signal.Side,
		Quantity:  actualQty,
		Price:     actualPrice,
		Reason:    fmt.Sprintf("Strategy: %s - %s", signal.StrategyName, signal.Reason),
		Timestamp: time.Now(),
		Mode:      strategyMode, // Use user's enabled mode preference
		EntryParams: &GinieEntryParams{
			EntryPrice:  actualPrice,
			StopLoss:    signal.StopLoss,
			StopLossPct: slPercent,
			TakeProfits: tpPrices,
			Leverage:    leverage,
		},
		Source:       "strategy",
		StrategyID:   &stratID,
		StrategyName: &stratName,
	})
}

// === PER-MODE CAPITAL ALLOCATION METHODS ===

// canAllocateForMode checks if capital can be allocated for a mode
// Returns (canTrade, reason)
func (ga *GinieAutopilot) canAllocateForMode(mode GinieTradingMode, requestedUSD float64) (bool, string) {
	settings := GetSettingsManager()
	allocationConfig := settings.GetModeAllocation()

	// Get balance
	balance, err := ga.getAvailableBalance()
	if err != nil {
		return false, fmt.Sprintf("balance fetch failed: %v", err)
	}

	if balance <= 0 {
		return false, "insufficient balance"
	}

	// Get current positions and capital usage per mode
	currentPositions := make(map[string]int)
	currentUsedUSD := make(map[string]float64)

	ga.mu.RLock()
	for _, pos := range ga.positions {
		// Calculate position USD cost
		posUSD := pos.EntryPrice * pos.RemainingQty / float64(pos.Leverage)
		modeStr := string(pos.Mode)
		currentUsedUSD[modeStr] += posUSD
		currentPositions[modeStr]++
	}
	ga.mu.RUnlock()

	// Get allocation state
	allocationState := settings.GetModeAllocationState(string(mode), balance, currentPositions, currentUsedUSD)

	// Check 1: Position limit
	if allocationState.CurrentPositions >= allocationState.MaxPositions {
		return false, fmt.Sprintf("position limit reached: %d/%d", allocationState.CurrentPositions, allocationState.MaxPositions)
	}

	// Check 2: Capital limit for mode
	if allocationState.UsedUSD+requestedUSD > allocationState.AllocatedUSD {
		return false, fmt.Sprintf("mode capital limit reached: %.2f USD allocated, %.2f used, %.2f requested",
			allocationState.AllocatedUSD, allocationState.UsedUSD, requestedUSD)
	}

	// Check 3: Per-position max
	maxPerPosition := 0.0
	switch mode {
	case GinieModeUltraFast:
		maxPerPosition = allocationConfig.MaxUltraFastUSDPerPosition
	case GinieModeScalp:
		maxPerPosition = allocationConfig.MaxScalpUSDPerPosition
	case GinieModeSwing:
		maxPerPosition = allocationConfig.MaxSwingUSDPerPosition
	case GinieModePosition:
		maxPerPosition = allocationConfig.MaxPositionUSDPerPosition
	}

	if requestedUSD > maxPerPosition {
		return false, fmt.Sprintf("exceeds max per-position: %.2f requested > %.2f max", requestedUSD, maxPerPosition)
	}

	return true, ""
}

// allocateCapital allocates capital for a position
func (ga *GinieAutopilot) allocateCapital(mode GinieTradingMode, positionUSD float64) {
	modeStr := string(mode)

	ga.mu.Lock()
	ga.modeUsedUSD[modeStr] += positionUSD
	ga.modePositionCounts[modeStr]++
	ga.mu.Unlock()

	ga.logger.Info("Capital allocated",
		"mode", mode,
		"position_usd", positionUSD,
		"total_used", ga.modeUsedUSD[modeStr],
		"position_count", ga.modePositionCounts[modeStr])
}

// releaseCapital releases capital from a closed position
func (ga *GinieAutopilot) releaseCapital(mode GinieTradingMode, positionUSD float64) {
	modeStr := string(mode)

	ga.mu.Lock()
	ga.modeUsedUSD[modeStr] -= positionUSD
	if ga.modeUsedUSD[modeStr] < 0 {
		ga.modeUsedUSD[modeStr] = 0
	}
	ga.modePositionCounts[modeStr]--
	if ga.modePositionCounts[modeStr] < 0 {
		ga.modePositionCounts[modeStr] = 0
	}
	ga.mu.Unlock()

	ga.logger.Info("Capital released",
		"mode", mode,
		"position_usd", positionUSD,
		"total_used", ga.modeUsedUSD[modeStr],
		"position_count", ga.modePositionCounts[modeStr])
}

// GetModeAllocationStatus returns the current allocation status for all modes
func (ga *GinieAutopilot) GetModeAllocationStatus() map[string]interface{} {
	settings := GetSettingsManager()
	balance, _ := ga.getAvailableBalance()

	ga.mu.RLock()
	defer ga.mu.RUnlock()

	allocations := make(map[string]interface{})

	for _, mode := range []GinieTradingMode{GinieModeUltraFast, GinieModeScalp, GinieModeSwing, GinieModePosition} {
		state := settings.GetModeAllocationState(string(mode), balance, ga.modePositionCounts, ga.modeUsedUSD)

		allocations[string(mode)] = map[string]interface{}{
			"allocated_percent":    state.AllocatedPercent,
			"allocated_usd":        state.AllocatedUSD,
			"used_usd":             state.UsedUSD,
			"available_usd":        state.AvailableUSD,
			"current_positions":    state.CurrentPositions,
			"max_positions":        state.MaxPositions,
			"capital_utilization":  state.CapitalUtilization,
			"position_utilization": state.PositionUtilization,
		}
	}

	return allocations
}

// GetModeAllocationStatusWithBalance returns the current allocation status for all modes
// using an externally provided balance instead of the internal client's balance.
// This is used when the API needs to show allocations based on a user's real Binance balance.
func (ga *GinieAutopilot) GetModeAllocationStatusWithBalance(balance float64) map[string]interface{} {
	settings := GetSettingsManager()

	ga.mu.RLock()
	defer ga.mu.RUnlock()

	allocations := make(map[string]interface{})

	for _, mode := range []GinieTradingMode{GinieModeUltraFast, GinieModeScalp, GinieModeSwing, GinieModePosition} {
		state := settings.GetModeAllocationState(string(mode), balance, ga.modePositionCounts, ga.modeUsedUSD)

		allocations[string(mode)] = map[string]interface{}{
			"allocated_percent":    state.AllocatedPercent,
			"allocated_usd":        state.AllocatedUSD,
			"used_usd":             state.UsedUSD,
			"available_usd":        state.AvailableUSD,
			"current_positions":    state.CurrentPositions,
			"max_positions":        state.MaxPositions,
			"capital_utilization":  state.CapitalUtilization,
			"position_utilization": state.PositionUtilization,
		}
	}

	return allocations
}

// === SAFETY CONTROL METHODS ===

// checkRateLimit validates rate limiting for a mode
// Returns (allowed, reason) based on trades in sliding windows
func (ga *GinieAutopilot) checkRateLimit(mode GinieTradingMode) (bool, string) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	modeStr := string(mode)
	state, exists := ga.modeSafetyStates[modeStr]
	if !exists {
		return true, "" // No safety state = no restrictions
	}

	config, exists := ga.modeSafetyConfigs[modeStr]
	if !exists || config == nil {
		return true, "" // No config = no restrictions
	}

	now := time.Now()

	// Reset daily counter at midnight
	if ga.lastDayReset.Day() != now.Day() {
		ga.lastDayReset = now
		for _, s := range ga.modeSafetyStates {
			s.TradesToday = 0
		}
	}

	// Clean sliding windows - keep only trades within time windows
	oneMinuteAgo := now.Add(-1 * time.Minute)
	oneHourAgo := now.Add(-1 * time.Hour)

	// Filter trades from last minute
	state.TradesLastMinute = make([]time.Time, 0)
	for _, t := range state.TradesLastMinute {
		if t.After(oneMinuteAgo) {
			state.TradesLastMinute = append(state.TradesLastMinute, t)
		}
	}

	// Filter trades from last hour
	state.TradesLastHour = make([]time.Time, 0)
	for _, t := range state.TradesLastHour {
		if t.After(oneHourAgo) {
			state.TradesLastHour = append(state.TradesLastHour, t)
		}
	}

	// Check rate limits
	if len(state.TradesLastMinute) >= config.MaxTradesPerMinute {
		return false, fmt.Sprintf("rate limit: %d trades in last minute (max %d)",
			len(state.TradesLastMinute), config.MaxTradesPerMinute)
	}

	if len(state.TradesLastHour) >= config.MaxTradesPerHour {
		return false, fmt.Sprintf("rate limit: %d trades in last hour (max %d)",
			len(state.TradesLastHour), config.MaxTradesPerHour)
	}

	if state.TradesToday >= config.MaxTradesPerDay {
		return false, fmt.Sprintf("rate limit: %d trades today (max %d)",
			state.TradesToday, config.MaxTradesPerDay)
	}

	return true, ""
}

// checkProfitThreshold validates cumulative profit/loss threshold
// Returns (allowed, reason) based on sliding window P&L
func (ga *GinieAutopilot) checkProfitThreshold(mode GinieTradingMode) (bool, string) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	modeStr := string(mode)
	state, exists := ga.modeSafetyStates[modeStr]
	if !exists {
		return true, "" // No safety state = no restrictions
	}

	config, exists := ga.modeSafetyConfigs[modeStr]
	if !exists || config == nil || !config.EnableProfitMonitor {
		return true, "" // Profit monitoring disabled
	}

	// Check if currently paused by profit threshold
	if state.IsPausedProfit && time.Now().Before(state.ProfitPauseUntil) {
		timeRemaining := time.Until(state.ProfitPauseUntil).Seconds()
		return false, fmt.Sprintf("profit threshold pause active (%.0f seconds remaining)", timeRemaining)
	}

	// Clean sliding window - keep only recent trades
	windowStart := time.Now().Add(-time.Duration(config.ProfitWindowMinutes) * time.Minute)
	state.ProfitWindow = make([]SafetyTradeResult, 0)
	for _, trade := range state.ProfitWindow {
		if trade.Timestamp.After(windowStart) {
			state.ProfitWindow = append(state.ProfitWindow, trade)
		}
	}

	// Calculate cumulative P&L in window
	windowProfitPct := 0.0
	for _, trade := range state.ProfitWindow {
		windowProfitPct += trade.PnLPercent
	}
	state.WindowProfitPct = windowProfitPct

	// Check threshold
	if windowProfitPct < config.MaxLossPercentInWindow {
		state.IsPausedProfit = true
		state.ProfitPauseUntil = time.Now().Add(time.Duration(config.PauseCooldownMinutes) * time.Minute)
		state.IsPaused = true
		state.PauseReason = fmt.Sprintf("cumulative loss %.2f%% in %d min (threshold: %.2f%%)",
			windowProfitPct, config.ProfitWindowMinutes, config.MaxLossPercentInWindow)
		ga.logger.Warn("Profit threshold triggered", "mode", mode, "reason", state.PauseReason)
		return false, state.PauseReason
	}

	// Clear pause flag if threshold is no longer breached
	if state.IsPausedProfit {
		state.IsPausedProfit = false
		state.IsPaused = false
		ga.logger.Info("Profit threshold pause cleared", "mode", mode)
	}

	return true, ""
}

// checkWinRate validates minimum win rate threshold
// Returns (allowed, reason) based on recent trade results
func (ga *GinieAutopilot) checkWinRate(mode GinieTradingMode) (bool, string) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	modeStr := string(mode)
	state, exists := ga.modeSafetyStates[modeStr]
	if !exists {
		return true, "" // No safety state = no restrictions
	}

	config, exists := ga.modeSafetyConfigs[modeStr]
	if !exists || config == nil || !config.EnableWinRateMonitor {
		return true, "" // Win-rate monitoring disabled
	}

	// Check if currently paused by win-rate
	if state.IsPausedWinRate && time.Now().Before(state.WinRatePauseUntil) {
		timeRemaining := time.Until(state.WinRatePauseUntil).Seconds()
		return false, fmt.Sprintf("win-rate pause active (%.0f seconds remaining)", timeRemaining)
	}

	// Need minimum sample size
	if len(state.RecentTrades) < config.WinRateSampleSize {
		return true, "" // Not enough data yet
	}

	// Calculate win rate from most recent trades
	winCount := 0
	for i := 0; i < config.WinRateSampleSize && i < len(state.RecentTrades); i++ {
		if state.RecentTrades[i].IsWinning {
			winCount++
		}
	}
	winRate := (float64(winCount) / float64(config.WinRateSampleSize)) * 100
	state.CurrentWinRate = winRate

	// Check threshold
	if winRate < config.MinWinRateThreshold {
		state.IsPausedWinRate = true
		state.WinRatePauseUntil = time.Now().Add(time.Duration(config.WinRateCooldownMinutes) * time.Minute)
		state.IsPaused = true
		state.PauseReason = fmt.Sprintf("win-rate %.1f%% below threshold (%.1f%%)",
			winRate, config.MinWinRateThreshold)
		ga.logger.Warn("Win-rate threshold triggered", "mode", mode, "reason", state.PauseReason)
		return false, state.PauseReason
	}

	// Clear pause flag if threshold is no longer breached
	if state.IsPausedWinRate {
		state.IsPausedWinRate = false
		state.IsPaused = false
		ga.logger.Info("Win-rate pause cleared", "mode", mode)
	}

	return true, ""
}

// canTradeMode performs comprehensive safety checks before allowing a trade
// Checks (in order):
// 1. Capital allocation (position limit, capital limit, per-position max)
// 2. Rate limiting (trades/minute, hour, day)
// 3. Profit threshold (cumulative loss monitoring)
// 4. Win-rate threshold (minimum win rate requirement)
// 5. Global circuit breaker (final safety net)
func (ga *GinieAutopilot) canTradeMode(mode GinieTradingMode, requestedUSD float64) (bool, string) {
	// Step 1: Capital allocation check
	if ok, reason := ga.canAllocateForMode(mode, requestedUSD); !ok {
		return false, "allocation: " + reason
	}

	// Step 2: Rate limiting check
	if ok, reason := ga.checkRateLimit(mode); !ok {
		return false, "rate_limit: " + reason
	}

	// Step 3: Profit threshold check
	if ok, reason := ga.checkProfitThreshold(mode); !ok {
		return false, "profit_monitor: " + reason
	}

	// Step 4: Win-rate check
	if ok, reason := ga.checkWinRate(mode); !ok {
		return false, "win_rate: " + reason
	}

	// Step 5: Global circuit breaker (if configured)
	if ga.circuitBreaker != nil {
		if ok, reason := ga.circuitBreaker.CanTrade(); !ok {
			return false, "circuit_breaker: " + reason
		}
	}

	return true, ""
}

// recordModeTradeOpening records a new trade initiation for rate limiting
// Should be called when a position is opened
func (ga *GinieAutopilot) recordModeTradeOpening(mode GinieTradingMode) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	modeStr := string(mode)
	state, exists := ga.modeSafetyStates[modeStr]
	if !exists {
		return
	}

	now := time.Now()
	state.TradesLastMinute = append(state.TradesLastMinute, now)
	state.TradesLastHour = append(state.TradesLastHour, now)
	state.TradesToday++
}

// recordModeTradeClosure records a completed trade for win-rate and profit tracking
// Should be called when a position is closed
func (ga *GinieAutopilot) recordModeTradeClosure(mode GinieTradingMode, symbol string, pnlUSD, pnlPercent float64) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	modeStr := string(mode)
	state, exists := ga.modeSafetyStates[modeStr]
	if !exists {
		return
	}

	tradeResult := SafetyTradeResult{
		Symbol:      symbol,
		PnLUSD:      pnlUSD,
		PnLPercent:  pnlPercent,
		IsWinning:   pnlUSD > 0,
		Timestamp:   time.Now(),
		Mode:        modeStr,
	}

	// Add to profit window
	state.ProfitWindow = append(state.ProfitWindow, tradeResult)

	// Add to recent trades (keep only last N trades for win-rate calculation)
	config, exists := ga.modeSafetyConfigs[modeStr]
	if exists && config != nil {
		state.RecentTrades = append(state.RecentTrades, tradeResult)
		// Keep only the most recent trades (circular buffer)
		if len(state.RecentTrades) > config.WinRateSampleSize*2 {
			state.RecentTrades = state.RecentTrades[1:]
		}
	}
}

// === ULTRA-FAST SCALPING MODE METHODS ===

// monitorUltraFastPositions runs a configurable polling loop to monitor ultra-fast positions
// for profit targets and time-based exits
func (ga *GinieAutopilot) monitorUltraFastPositions() {
	defer ga.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			ga.logger.Error("PANIC in ultra-fast position monitor - restarting", "panic", r)
			log.Printf("[GINIE-PANIC] Ultra-fast position monitor panic: %v", r)
			time.Sleep(2 * time.Second)
			ga.wg.Add(1)
			go ga.monitorUltraFastPositions()
		}
	}()

	// Get configurable interval from settings (default 2000ms)
	settings := GetSettingsManager().GetCurrentSettings()
	monitorInterval := settings.UltraFastMonitorInterval
	if monitorInterval <= 0 {
		monitorInterval = 2000 // Default to 2 seconds
	}

	ga.logger.Info("Ultra-fast position monitor started", "interval_ms", monitorInterval)

	ticker := time.NewTicker(time.Duration(monitorInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ga.stopChan:
			ga.logger.Info("Ultra-fast position monitor stopping")
			return
		case <-ticker.C:
			// Check rate limiter - skip if circuit breaker is open
			if binance.GetRateLimiter().IsCircuitOpen() {
				log.Printf("[ULTRA-FAST] Skipping cycle - rate limiter circuit breaker is open")
				continue
			}
			ga.checkUltraFastExits()
		}
	}
}

// checkUltraFastExits checks all ultra-fast positions for exit conditions
// Exit priority:
// 1. STOP LOSS hit → EXIT immediately (100% loss booking)
// 2. Profit target hit → EXIT immediately (100% profit booking)
// 3. Trailing stop triggered → EXIT (capture max profit with pullback protection)
// 4. Time > 1s AND profitable → EXIT to secure profit
// 5. Time > 3s → FORCE EXIT (emergency timeout)
func (ga *GinieAutopilot) checkUltraFastExits() {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	now := time.Now()

	// Find all ultra-fast positions
	ultraFastPositions := make([]*GiniePosition, 0)
	for _, pos := range ga.positions {
		if pos.Mode == GinieModeUltraFast && pos.UltraFastSignal != nil {
			ultraFastPositions = append(ultraFastPositions, pos)
		}
	}

	if len(ultraFastPositions) == 0 {
		return
	}

	for _, pos := range ultraFastPositions {
		// Get current price
		currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(pos.Symbol)
		if err != nil {
			ga.logger.Warn("Failed to get price for ultra-fast exit check",
				"symbol", pos.Symbol,
				"error", err)
			continue
		}

		// Calculate PnL %
		var pnlPercent float64
		if pos.Side == "LONG" {
			pnlPercent = ((currentPrice - pos.EntryPrice) / pos.EntryPrice) * 100
		} else {
			pnlPercent = ((pos.EntryPrice - currentPrice) / pos.EntryPrice) * 100
		}

		holdTime := now.Sub(pos.EntryTime)

		// Exit Condition 1: STOP LOSS HIT → 100% LOSS BOOKING (priority 1)
		if pos.StopLoss > 0 && ga.checkStopLossHit(pos, currentPrice) {
			ga.logger.Warn("Ultra-fast: STOP LOSS HIT - closing entire position (100% loss booking)",
				"symbol", pos.Symbol,
				"stop_loss", pos.StopLoss,
				"current_price", currentPrice,
				"pnl_pct", pnlPercent)
			ga.executeUltraFastExit(pos, currentPrice, "stop_loss_hit")
			continue
		}

		// Exit Condition 2: Profit target hit → 100% PROFIT BOOKING (priority 2)
		if pos.UltraFastTargetPercent > 0 && pnlPercent >= pos.UltraFastTargetPercent {
			ga.logger.Info("Ultra-fast: Profit target hit - 100% profit booking - exiting",
				"symbol", pos.Symbol,
				"target_pct", pos.UltraFastTargetPercent,
				"current_pnl_pct", pnlPercent,
				"hold_time_ms", holdTime.Milliseconds())
			ga.executeUltraFastExit(pos, currentPrice, "target_hit")
			continue
		}

		// Exit Condition 3: Trailing stop triggered (priority 3)
		if pos.TrailingActive && ga.checkTrailingStop(pos, currentPrice) {
			ga.logger.Info("Ultra-fast: Trailing stop hit - exiting with profit protection",
				"symbol", pos.Symbol,
				"highest_price", pos.HighestPrice,
				"current_price", currentPrice,
				"trailing_pct", pos.TrailingPercent,
				"pnl_pct", pnlPercent)
			ga.executeUltraFastExit(pos, currentPrice, "trailing_stop_hit")
			continue
		}

		// Update trailing stop if position is profitable (activate and trail upward)
		if pnlPercent > 0 {
			ga.updateUltraFastTrailingStop(pos, currentPrice)
		}

		// Exit Condition 4: Time > 1s AND profitable → EXIT to secure profit
		if holdTime >= 1*time.Second && pnlPercent > 0 {
			ga.logger.Info("Ultra-fast: Time limit with profit - exiting",
				"symbol", pos.Symbol,
				"hold_time_ms", holdTime.Milliseconds(),
				"pnl_pct", pnlPercent)
			ga.executeUltraFastExit(pos, currentPrice, "time_limit_profit")
			continue
		}

		// Exit Condition 5: Time > 3s → FORCE EXIT (emergency)
		if holdTime >= 3*time.Second {
			ga.logger.Warn("Ultra-fast: Force exit after 3s timeout",
				"symbol", pos.Symbol,
				"hold_time_ms", holdTime.Milliseconds(),
				"pnl_pct", pnlPercent)
			ga.executeUltraFastExit(pos, currentPrice, "force_exit_timeout")
		}
	}
}

// checkStopLossHit checks if price has hit the stop loss level
// For LONG positions: price <= SL
// For SHORT positions: price >= SL
func (ga *GinieAutopilot) checkStopLossHit(pos *GiniePosition, currentPrice float64) bool {
	if pos.StopLoss <= 0 {
		return false
	}

	if pos.Side == "LONG" {
		return currentPrice <= pos.StopLoss
	} else {
		return currentPrice >= pos.StopLoss
	}
}

// updateUltraFastTrailingStop updates the trailing stop for ultra-fast positions
// Activates trailing stop when position becomes profitable and trails upward as price rises
func (ga *GinieAutopilot) updateUltraFastTrailingStop(pos *GiniePosition, currentPrice float64) {
	// Initialize highest/lowest price on first profit
	if pos.HighestPrice == 0 {
		pos.HighestPrice = currentPrice
	}
	if pos.LowestPrice == 0 {
		pos.LowestPrice = currentPrice
	}

	// Update high water mark for LONG, low water mark for SHORT
	if pos.Side == "LONG" {
		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice

			// Activate trailing stop once position shows profit
			if !pos.TrailingActive {
				pos.TrailingActive = true
				pos.TrailingPercent = 0.5 // Trail by 0.5% from high
				ga.logger.Info("Ultra-fast: Trailing stop activated",
					"symbol", pos.Symbol,
					"highest_price", pos.HighestPrice,
					"trailing_pct", pos.TrailingPercent)
			}

			// Update trailing stop price (0.5% below highest)
			pos.StopLoss = pos.HighestPrice * (1 - pos.TrailingPercent/100)
		}
	} else { // SHORT
		if currentPrice < pos.LowestPrice {
			pos.LowestPrice = currentPrice

			// Activate trailing stop once position shows profit
			if !pos.TrailingActive {
				pos.TrailingActive = true
				pos.TrailingPercent = 0.5 // Trail by 0.5% from low
				ga.logger.Info("Ultra-fast: Trailing stop activated (SHORT)",
					"symbol", pos.Symbol,
					"lowest_price", pos.LowestPrice,
					"trailing_pct", pos.TrailingPercent)
			}

			// Update trailing stop price (0.5% above lowest)
			pos.StopLoss = pos.LowestPrice * (1 + pos.TrailingPercent/100)
		}
	}
}

// executeUltraFastExit closes an ultra-fast position with fee-aware PnL calculation
// Updates capital allocation and records trade result
// Uses LIMIT orders instead of MARKET to reduce slippage on exits
func (ga *GinieAutopilot) executeUltraFastExit(pos *GiniePosition, currentPrice float64, reason string) {
	symbol := pos.Symbol

	// Calculate quantity to close (all remaining)
	closeQty := pos.RemainingQty

	// Calculate PnL with accurate fee handling
	var pnlUSD, pnlPercent float64
	var exitFeeUSD float64

	if pos.Side == "LONG" {
		pnlBeforeFees := (currentPrice - pos.EntryPrice) * closeQty
		// Only count exit fee (entry fee already deducted at position open)
		exitFeeUSD = currentPrice * closeQty * 0.0004 // 0.04% taker fee
		pnlUSD = pnlBeforeFees - exitFeeUSD
		pnlPercent = ((currentPrice - pos.EntryPrice) / pos.EntryPrice) * 100
	} else {
		pnlBeforeFees := (pos.EntryPrice - currentPrice) * closeQty
		exitFeeUSD = currentPrice * closeQty * 0.0004 // 0.04% taker fee
		pnlUSD = pnlBeforeFees - exitFeeUSD
		pnlPercent = ((pos.EntryPrice - currentPrice) / pos.EntryPrice) * 100
	}

	ga.logger.Info("Ultra-fast position closing",
		"symbol", symbol,
		"side", pos.Side,
		"qty", closeQty,
		"entry_price", pos.EntryPrice,
		"exit_price", currentPrice,
		"pnl_usd", pnlUSD,
		"pnl_pct", pnlPercent,
		"exit_fee", exitFeeUSD,
		"reason", reason,
		"hold_time_ms", time.Since(pos.EntryTime).Milliseconds())

	if !ga.config.DryRun {
		// Close position using LIMIT order with 0.1% buffer to reduce slippage
		// LIMIT orders are preferred for ultra-fast exits to avoid worst-case execution
		// Especially important for stop-loss exits where slippage can be significant
		side := "SELL"
		positionSide := binance.PositionSideLong
		if pos.Side == "SHORT" {
			side = "BUY"
			positionSide = binance.PositionSideShort
		}

		// Check actual Binance position mode to avoid API error -4061
		effectivePositionSide := ga.getEffectivePositionSide(positionSide)

		// Use LIMIT order at slightly worse price (0.1% buffer) to ensure execution
		// For LONG: sell at 0.1% below current price (ensures fill on pullback)
		// For SHORT: buy at 0.1% above current price (ensures fill on pullback)
		limitPrice := currentPrice
		if pos.Side == "LONG" {
			limitPrice = currentPrice * 0.999 // 0.1% buffer below
		} else {
			limitPrice = currentPrice * 1.001 // 0.1% buffer above
		}

		orderParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         side,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeLimit,
			Quantity:     closeQty,
			Price:        limitPrice,
		}

		order, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
		if err != nil {
			ga.logger.Error("Ultra-fast exit LIMIT order failed",
				"symbol", symbol,
				"current_price", currentPrice,
				"limit_price", limitPrice,
				"error", err.Error())
			return
		}

		ga.logger.Info("Ultra-fast exit LIMIT order placed",
			"symbol", symbol,
			"order_id", order.OrderId,
			"current_price", currentPrice,
			"limit_price", limitPrice,
			"reason", reason)
	}

	// Remove position from tracking
	delete(ga.positions, symbol)

	// Update daily tracking
	ga.dailyTrades++
	ga.dailyPnL += pnlUSD
	ga.totalTrades++
	if pnlUSD > 0 {
		ga.winningTrades++
	}
	ga.totalPnL += pnlUSD

	// Record trade result
	ga.recordTrade(GinieTradeResult{
		Symbol:     symbol,
		Action:     "close",
		Side:       pos.Side,
		Quantity:   closeQty,
		Price:      currentPrice,
		PnL:        pnlUSD,
		PnLPercent: pnlPercent,
		Reason:     fmt.Sprintf("ultra_fast_%s", reason),
		Timestamp:  time.Now(),
		Mode:       GinieModeUltraFast,
		Confidence: pos.UltraFastSignal.EntryConfidence,
	})

	// Check for re-entry opportunity based on volatility regime
	if pos.UltraFastSignal != nil && pos.UltraFastSignal.VolatilityRegime != nil {
		reEntryDelay := pos.UltraFastSignal.VolatilityRegime.ReEntryDelay
		ga.logger.Info("Ultra-fast: Re-entry available after delay",
			"symbol", symbol,
			"delay_seconds", reEntryDelay.Seconds(),
			"max_trades_per_hour", pos.UltraFastSignal.VolatilityRegime.MaxTradesPerHour)
	}
}

// executeUltraFastEntry opens an ultra-fast position with signal-derived parameters
// Validates safety checks and applies fee-aware profit targets
func (ga *GinieAutopilot) executeUltraFastEntry(symbol string, signal *UltraFastSignal) error {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	// Safety checks
	if _, exists := ga.positions[symbol]; exists {
		return fmt.Errorf("position already exists for %s", symbol)
	}

	// Check position limits
	currentUltraFastCount := 0
	for _, pos := range ga.positions {
		if pos.Mode == GinieModeUltraFast {
			currentUltraFastCount++
		}
	}

	settingsManager := GetSettingsManager()
	currentSettings := settingsManager.GetCurrentSettings()
	maxUltraFastPositions := currentSettings.UltraFastMaxPositions
	if currentUltraFastCount >= maxUltraFastPositions {
		return fmt.Errorf("ultra-fast position limit reached: %d/%d", currentUltraFastCount, maxUltraFastPositions)
	}

	// Get current price
	price, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get price for %s: %w", symbol, err)
	}

	// Calculate position size (ultra-fast conservative sizing)
	maxUSDPerPos := currentSettings.UltraFastMaxUSDPerPos
	positionUSD := maxUSDPerPos // Fixed for ultra-fast (not adaptive like scalp)

	// Get leverage from style config
	styleConfig := GetDefaultStyleConfig(StyleUltraFast)
	leverage := styleConfig.DefaultLeverage

	// Calculate quantity
	quantity := (positionUSD * float64(leverage)) / price
	quantity = roundQuantity(symbol, quantity)

	if quantity <= 0 {
		return fmt.Errorf("calculated zero quantity for %s", symbol)
	}

	// Determine side
	side := "BUY"
	positionSide := binance.PositionSideLong
	isLong := signal.TrendBias == "LONG"
	if !isLong {
		side = "SELL"
		positionSide = binance.PositionSideShort
	}

	// Check actual Binance position mode to avoid API error -4061
	effectivePositionSide := ga.getEffectivePositionSide(positionSide)

	ga.logger.Info("Ultra-fast entry executing",
		"symbol", symbol,
		"trend_bias", signal.TrendBias,
		"trend_strength", signal.TrendStrength,
		"entry_confidence", signal.EntryConfidence,
		"volatility_regime", signal.VolatilityRegime.Level,
		"quantity", quantity,
		"leverage", leverage,
		"position_usd", positionUSD,
		"target_percent", signal.MinProfitTarget)

	// Variables for actual fill details
	actualPrice := price
	actualQty := quantity

	if !ga.config.DryRun {
		// Set leverage
		_, err = ga.futuresClient.SetLeverage(symbol, leverage)
		if err != nil {
			return fmt.Errorf("failed to set leverage: %w", err)
		}

		// Place market order
		orderParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         side,
			PositionSide: effectivePositionSide,
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     quantity,
		}

		order, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
		if err != nil {
			return fmt.Errorf("failed to place order: %w", err)
		}

		// Verify fill
		fillPrice, fillQty, fillErr := ga.verifyOrderFill(order, quantity)
		if fillErr != nil {
			return fmt.Errorf("order fill verification failed: %w", fillErr)
		}

		actualPrice = fillPrice
		actualQty = fillQty

		ga.logger.Info("Ultra-fast order filled",
			"symbol", symbol,
			"order_id", order.OrderId,
			"fill_price", actualPrice,
			"fill_qty", actualQty)
	}

	// Create ultra-fast position
	position := &GiniePosition{
		Symbol:                 symbol,
		Side:                   signal.TrendBias,
		Mode:                   GinieModeUltraFast,
		EntryPrice:             actualPrice,
		OriginalQty:            actualQty,
		RemainingQty:           actualQty,
		Leverage:               leverage,
		EntryTime:              time.Now(),
		TakeProfits:            []GinieTakeProfitLevel{}, // Ultra-fast uses profit target, not multi-level TPs
		CurrentTPLevel:         0,
		StopLoss:               0, // SL managed by circuit breaker, not individual positions
		OriginalSL:             0,
		MovedToBreakeven:       false,
		TrailingActive:         false,
		HighestPrice:           actualPrice,
		LowestPrice:            actualPrice,
		TrailingPercent:        0,
		Source:                 "ai",
		UltraFastSignal:        signal,
		UltraFastTargetPercent: signal.MinProfitTarget,
		MaxHoldTime:            3 * time.Second,
		Protection:             NewProtectionStatus(), // Initialize protection tracking
	}

	ga.positions[symbol] = position
	ga.dailyTrades++
	ga.totalTrades++

	// Update ultra-fast stats
	sm := GetSettingsManager()
	sm.IncrementUltraFastTrade()

	// Record trade opening
	ga.recordTrade(GinieTradeResult{
		Symbol:     symbol,
		Action:     "open",
		Side:       signal.TrendBias,
		Quantity:   actualQty,
		Price:      actualPrice,
		Reason:     fmt.Sprintf("ultra_fast_entry: %s, confidence=%.1f%%", signal.TrendBias, signal.EntryConfidence*100),
		Timestamp:  time.Now(),
		Mode:       GinieModeUltraFast,
		Confidence: signal.EntryConfidence,
	})

	ga.logger.Info("Ultra-fast position opened successfully",
		"symbol", symbol,
		"position_id", fmt.Sprintf("%s_%d", symbol, time.Now().UnixNano()))

	return nil
}

// ========== Mode-Specific Circuit Breaker Methods (Story 2.7 Task 2.7.4) ==========

// initModeCircuitBreakers initializes mode circuit breakers from default configs
func (ga *GinieAutopilot) initModeCircuitBreakers() {
	modes := []GinieTradingMode{GinieModeUltraFast, GinieModeScalp, GinieModeSwing, GinieModePosition}

	for _, mode := range modes {
		config := ga.GetDefaultModeConfig(mode)
		if config != nil && config.CircuitBreaker != nil {
			ga.modeCircuitBreakers[mode] = &ModeCircuitBreaker{
				// Copy configuration values
				MaxLossPerHour:     config.CircuitBreaker.MaxLossPerHour,
				MaxLossPerDay:      config.CircuitBreaker.MaxLossPerDay,
				MaxConsecutiveLoss: config.CircuitBreaker.MaxConsecutiveLosses,
				MaxTradesPerMinute: config.CircuitBreaker.MaxTradesPerMinute,
				MaxTradesPerHour:   config.CircuitBreaker.MaxTradesPerHour,
				MaxTradesPerDay:    config.CircuitBreaker.MaxTradesPerDay,
				WinRateCheckAfter:  config.CircuitBreaker.WinRateCheckAfter,
				MinWinRatePercent:  config.CircuitBreaker.MinWinRate,
				CooldownMinutes:    config.CircuitBreaker.CooldownMinutes,
				AutoRecovery:       true,
				// Initialize state values
				CurrentHourLoss:   0,
				CurrentDayLoss:    0,
				ConsecutiveLosses: 0,
				TradesThisMinute:  0,
				TradesThisHour:    0,
				TradesThisDay:     0,
				TotalWins:         0,
				TotalTrades:       0,
				IsPaused:          false,
			}
			log.Printf("[MODE-CIRCUIT-BREAKER] Initialized for mode %s: MaxLoss/hr=$%.2f, MaxLoss/day=$%.2f, MaxConsecLoss=%d, Cooldown=%dm",
				mode, config.CircuitBreaker.MaxLossPerHour, config.CircuitBreaker.MaxLossPerDay,
				config.CircuitBreaker.MaxConsecutiveLosses, config.CircuitBreaker.CooldownMinutes)
		}
	}
}

// GetDefaultModeConfig returns the default configuration for a trading mode
// This retrieves the mode config from DefaultModeConfigs() in settings.go
func (ga *GinieAutopilot) GetDefaultModeConfig(mode GinieTradingMode) *ModeFullConfig {
	configs := DefaultModeConfigs()
	modeStr := string(mode)
	if config, exists := configs[modeStr]; exists {
		return config
	}
	return nil
}

// getModeCircuitBreaker returns the circuit breaker for a mode, creating if needed
func (ga *GinieAutopilot) getModeCircuitBreaker(mode GinieTradingMode) *ModeCircuitBreaker {
	if cb, exists := ga.modeCircuitBreakers[mode]; exists {
		return cb
	}

	// Create default circuit breaker if not exists
	config := ga.GetDefaultModeConfig(mode)
	if config == nil || config.CircuitBreaker == nil {
		// Fallback defaults
		return &ModeCircuitBreaker{
			MaxLossPerHour:     100.0,
			MaxLossPerDay:      300.0,
			MaxConsecutiveLoss: 5,
			MaxTradesPerMinute: 5,
			MaxTradesPerHour:   30,
			MaxTradesPerDay:    100,
			WinRateCheckAfter:  10,
			MinWinRatePercent:  45.0,
			CooldownMinutes:    30,
			AutoRecovery:       true,
		}
	}

	cb := &ModeCircuitBreaker{
		MaxLossPerHour:     config.CircuitBreaker.MaxLossPerHour,
		MaxLossPerDay:      config.CircuitBreaker.MaxLossPerDay,
		MaxConsecutiveLoss: config.CircuitBreaker.MaxConsecutiveLosses,
		MaxTradesPerMinute: config.CircuitBreaker.MaxTradesPerMinute,
		MaxTradesPerHour:   config.CircuitBreaker.MaxTradesPerHour,
		MaxTradesPerDay:    config.CircuitBreaker.MaxTradesPerDay,
		WinRateCheckAfter:  config.CircuitBreaker.WinRateCheckAfter,
		MinWinRatePercent:  config.CircuitBreaker.MinWinRate,
		CooldownMinutes:    config.CircuitBreaker.CooldownMinutes,
		AutoRecovery:       true,
	}
	ga.modeCircuitBreakers[mode] = cb
	return cb
}

// CheckModeCircuitBreaker checks if the mode's circuit breaker allows trading
// Returns (true, "") if trading is allowed, (false, reason) if blocked
func (ga *GinieAutopilot) CheckModeCircuitBreaker(mode GinieTradingMode) (canTrade bool, reason string) {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	cb := ga.getModeCircuitBreaker(mode)
	if cb == nil {
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: No circuit breaker found, allowing trade", mode)
		return true, ""
	}

	// Check if mode is currently paused
	if cb.IsPaused {
		if time.Now().Before(cb.PausedUntil) {
			remainingTime := time.Until(cb.PausedUntil).Round(time.Second)
			reason := fmt.Sprintf("mode_paused: %s (remaining: %v)", cb.PauseReason, remainingTime)
			log.Printf("[MODE-CIRCUIT-BREAKER] %s: BLOCKED - %s", mode, reason)
			return false, reason
		}
		// Auto-recovery: pause has expired
		if cb.AutoRecovery {
			log.Printf("[MODE-CIRCUIT-BREAKER] %s: Auto-recovering from pause (was: %s)", mode, cb.PauseReason)
			cb.IsPaused = false
			cb.PauseReason = ""
		}
	}

	// Check trades per minute limit
	if cb.MaxTradesPerMinute > 0 && cb.TradesThisMinute >= cb.MaxTradesPerMinute {
		reason := fmt.Sprintf("max_trades_per_minute: %d/%d", cb.TradesThisMinute, cb.MaxTradesPerMinute)
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: BLOCKED - %s", mode, reason)
		return false, reason
	}

	// Check trades per hour limit
	if cb.MaxTradesPerHour > 0 && cb.TradesThisHour >= cb.MaxTradesPerHour {
		reason := fmt.Sprintf("max_trades_per_hour: %d/%d", cb.TradesThisHour, cb.MaxTradesPerHour)
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: BLOCKED - %s", mode, reason)
		return false, reason
	}

	// Check trades per day limit
	if cb.MaxTradesPerDay > 0 && cb.TradesThisDay >= cb.MaxTradesPerDay {
		reason := fmt.Sprintf("max_trades_per_day: %d/%d", cb.TradesThisDay, cb.MaxTradesPerDay)
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: BLOCKED - %s", mode, reason)
		return false, reason
	}

	// Check hourly loss limit
	if cb.MaxLossPerHour > 0 && cb.CurrentHourLoss >= cb.MaxLossPerHour {
		reason := fmt.Sprintf("max_loss_per_hour: $%.2f/$%.2f", cb.CurrentHourLoss, cb.MaxLossPerHour)
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: BLOCKED - %s", mode, reason)
		return false, reason
	}

	// Check daily loss limit
	if cb.MaxLossPerDay > 0 && cb.CurrentDayLoss >= cb.MaxLossPerDay {
		reason := fmt.Sprintf("max_loss_per_day: $%.2f/$%.2f", cb.CurrentDayLoss, cb.MaxLossPerDay)
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: BLOCKED - %s", mode, reason)
		return false, reason
	}

	// Check consecutive losses limit
	if cb.MaxConsecutiveLoss > 0 && cb.ConsecutiveLosses >= cb.MaxConsecutiveLoss {
		reason := fmt.Sprintf("max_consecutive_losses: %d/%d", cb.ConsecutiveLosses, cb.MaxConsecutiveLoss)
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: BLOCKED - %s", mode, reason)
		return false, reason
	}

	// Check win rate after sufficient trades
	if cb.WinRateCheckAfter > 0 && cb.TotalTrades >= cb.WinRateCheckAfter {
		winRate := 0.0
		if cb.TotalTrades > 0 {
			winRate = float64(cb.TotalWins) / float64(cb.TotalTrades) * 100.0
		}
		if cb.MinWinRatePercent > 0 && winRate < cb.MinWinRatePercent {
			reason := fmt.Sprintf("low_win_rate: %.1f%% < %.1f%% (after %d trades)", winRate, cb.MinWinRatePercent, cb.TotalTrades)
			log.Printf("[MODE-CIRCUIT-BREAKER] %s: BLOCKED - %s", mode, reason)
			return false, reason
		}
	}

	// All checks passed
	log.Printf("[MODE-CIRCUIT-BREAKER] %s: ALLOWED - trades=%d/%d (min), loss=$%.2f/$%.2f (hr), consec=%d/%d",
		mode, cb.TradesThisMinute, cb.MaxTradesPerMinute, cb.CurrentHourLoss, cb.MaxLossPerHour,
		cb.ConsecutiveLosses, cb.MaxConsecutiveLoss)
	return true, ""
}

// TriggerModeCircuitBreaker triggers the circuit breaker for a mode
func (ga *GinieAutopilot) TriggerModeCircuitBreaker(mode GinieTradingMode, reason string) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	cb := ga.getModeCircuitBreaker(mode)
	if cb == nil {
		log.Printf("[CIRCUIT-BREAKER-TRIGGERED] %s: Cannot trigger - no circuit breaker found", mode)
		return
	}

	// Get cooldown duration from config
	cooldownMinutes := cb.CooldownMinutes
	if cooldownMinutes <= 0 {
		cooldownMinutes = 30 // Default 30 minutes
	}

	cb.IsPaused = true
	cb.PausedUntil = time.Now().Add(time.Duration(cooldownMinutes) * time.Minute)
	cb.PauseReason = reason

	log.Printf("[CIRCUIT-BREAKER-TRIGGERED] Mode=%s, Reason=%s, PausedUntil=%s, CooldownMinutes=%d",
		mode, reason, cb.PausedUntil.Format(time.RFC3339), cooldownMinutes)

	ga.logger.Warn("Mode circuit breaker triggered",
		"mode", mode,
		"reason", reason,
		"paused_until", cb.PausedUntil,
		"cooldown_minutes", cooldownMinutes)
}

// RecordModeTradeResult records a trade result and updates the circuit breaker state
func (ga *GinieAutopilot) RecordModeTradeResult(mode GinieTradingMode, pnl float64) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	cb := ga.getModeCircuitBreaker(mode)
	if cb == nil {
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: Cannot record trade - no circuit breaker found", mode)
		return
	}

	// Update trade counters
	cb.TradesThisMinute++
	cb.TradesThisHour++
	cb.TradesThisDay++
	cb.TotalTrades++

	// Update win/loss tracking
	if pnl > 0 {
		// Winning trade
		cb.TotalWins++
		cb.ConsecutiveLosses = 0
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: Recorded WIN +$%.2f (wins=%d, total=%d, consec_loss=0)",
			mode, pnl, cb.TotalWins, cb.TotalTrades)
	} else if pnl < 0 {
		// Losing trade
		cb.ConsecutiveLosses++
		absLoss := -pnl // Convert to positive for loss tracking
		cb.CurrentHourLoss += absLoss
		cb.CurrentDayLoss += absLoss
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: Recorded LOSS -$%.2f (hr_loss=$%.2f, day_loss=$%.2f, consec=%d)",
			mode, absLoss, cb.CurrentHourLoss, cb.CurrentDayLoss, cb.ConsecutiveLosses)
	}

	// Check if any threshold is now exceeded and trigger circuit breaker if needed
	triggered := false
	triggerReason := ""

	// Check consecutive losses
	if cb.MaxConsecutiveLoss > 0 && cb.ConsecutiveLosses >= cb.MaxConsecutiveLoss {
		triggered = true
		triggerReason = fmt.Sprintf("max_consecutive_losses_exceeded: %d/%d", cb.ConsecutiveLosses, cb.MaxConsecutiveLoss)
	}

	// Check hourly loss
	if !triggered && cb.MaxLossPerHour > 0 && cb.CurrentHourLoss >= cb.MaxLossPerHour {
		triggered = true
		triggerReason = fmt.Sprintf("max_hourly_loss_exceeded: $%.2f/$%.2f", cb.CurrentHourLoss, cb.MaxLossPerHour)
	}

	// Check daily loss
	if !triggered && cb.MaxLossPerDay > 0 && cb.CurrentDayLoss >= cb.MaxLossPerDay {
		triggered = true
		triggerReason = fmt.Sprintf("max_daily_loss_exceeded: $%.2f/$%.2f", cb.CurrentDayLoss, cb.MaxLossPerDay)
	}

	// Check win rate after sufficient trades
	if !triggered && cb.WinRateCheckAfter > 0 && cb.TotalTrades >= cb.WinRateCheckAfter {
		winRate := 0.0
		if cb.TotalTrades > 0 {
			winRate = float64(cb.TotalWins) / float64(cb.TotalTrades) * 100.0
		}
		if cb.MinWinRatePercent > 0 && winRate < cb.MinWinRatePercent {
			triggered = true
			triggerReason = fmt.Sprintf("low_win_rate: %.1f%% < %.1f%%", winRate, cb.MinWinRatePercent)
		}
	}

	if triggered {
		// Trigger without lock since we already hold it
		cooldownMinutes := cb.CooldownMinutes
		if cooldownMinutes <= 0 {
			cooldownMinutes = 30
		}
		cb.IsPaused = true
		cb.PausedUntil = time.Now().Add(time.Duration(cooldownMinutes) * time.Minute)
		cb.PauseReason = triggerReason

		log.Printf("[CIRCUIT-BREAKER-TRIGGERED] Mode=%s, Reason=%s, PausedUntil=%s",
			mode, triggerReason, cb.PausedUntil.Format(time.RFC3339))

		ga.logger.Warn("Mode circuit breaker auto-triggered after trade",
			"mode", mode,
			"reason", triggerReason,
			"paused_until", cb.PausedUntil)
	}
}

// ResetModeCircuitBreakerStats resets circuit breaker stats for a mode based on period
// period can be: "minute", "hour", "day"
func (ga *GinieAutopilot) ResetModeCircuitBreakerStats(mode GinieTradingMode, period string) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	cb := ga.getModeCircuitBreaker(mode)
	if cb == nil {
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: Cannot reset stats - no circuit breaker found", mode)
		return
	}

	switch period {
	case "minute":
		oldValue := cb.TradesThisMinute
		cb.TradesThisMinute = 0
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: Reset minute stats (trades_this_minute: %d -> 0)", mode, oldValue)

	case "hour":
		oldTrades := cb.TradesThisHour
		oldLoss := cb.CurrentHourLoss
		cb.TradesThisHour = 0
		cb.CurrentHourLoss = 0
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: Reset hour stats (trades: %d -> 0, loss: $%.2f -> $0)", mode, oldTrades, oldLoss)

	case "day":
		oldTrades := cb.TradesThisDay
		oldLoss := cb.CurrentDayLoss
		oldWins := cb.TotalWins
		oldTotal := cb.TotalTrades
		cb.TradesThisDay = 0
		cb.CurrentDayLoss = 0
		cb.TotalWins = 0
		cb.TotalTrades = 0
		// Also reset consecutive losses at day reset
		cb.ConsecutiveLosses = 0
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: Reset day stats (trades: %d -> 0, loss: $%.2f -> $0, wins: %d -> 0, total: %d -> 0)",
			mode, oldTrades, oldLoss, oldWins, oldTotal)

	default:
		log.Printf("[MODE-CIRCUIT-BREAKER] %s: Unknown reset period '%s' (valid: minute, hour, day)", mode, period)
	}
}

// GetModeCircuitBreakerStatus returns the current status of a mode's circuit breaker
func (ga *GinieAutopilot) GetModeCircuitBreakerStatus(mode GinieTradingMode) map[string]interface{} {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	cb := ga.getModeCircuitBreaker(mode)
	if cb == nil {
		return map[string]interface{}{
			"mode":    string(mode),
			"enabled": false,
			"error":   "circuit breaker not initialized",
		}
	}

	winRate := 0.0
	if cb.TotalTrades > 0 {
		winRate = float64(cb.TotalWins) / float64(cb.TotalTrades) * 100.0
	}

	cooldownRemaining := ""
	if cb.IsPaused && time.Now().Before(cb.PausedUntil) {
		cooldownRemaining = time.Until(cb.PausedUntil).Round(time.Second).String()
	}

	return map[string]interface{}{
		"mode":               string(mode),
		"enabled":            true,
		"is_paused":          cb.IsPaused,
		"pause_reason":       cb.PauseReason,
		"paused_until":       cb.PausedUntil,
		"cooldown_remaining": cooldownRemaining,
		"limits": map[string]interface{}{
			"max_trades_per_minute": cb.MaxTradesPerMinute,
			"max_trades_per_hour":   cb.MaxTradesPerHour,
			"max_trades_per_day":    cb.MaxTradesPerDay,
			"max_loss_per_hour":     cb.MaxLossPerHour,
			"max_loss_per_day":      cb.MaxLossPerDay,
			"max_consecutive_loss":  cb.MaxConsecutiveLoss,
			"min_win_rate":          cb.MinWinRatePercent,
			"win_rate_check_after":  cb.WinRateCheckAfter,
			"cooldown_minutes":      cb.CooldownMinutes,
		},
		"current_state": map[string]interface{}{
			"trades_this_minute":  cb.TradesThisMinute,
			"trades_this_hour":    cb.TradesThisHour,
			"trades_this_day":     cb.TradesThisDay,
			"current_hour_loss":   cb.CurrentHourLoss,
			"current_day_loss":    cb.CurrentDayLoss,
			"consecutive_losses":  cb.ConsecutiveLosses,
			"total_wins":          cb.TotalWins,
			"total_trades":        cb.TotalTrades,
			"current_win_rate":    winRate,
		},
	}
}

// ResetModeCircuitBreaker manually resets a mode's circuit breaker (clears pause)
func (ga *GinieAutopilot) ResetModeCircuitBreaker(mode GinieTradingMode) error {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	cb := ga.getModeCircuitBreaker(mode)
	if cb == nil {
		return fmt.Errorf("circuit breaker not found for mode %s", mode)
	}

	cb.IsPaused = false
	cb.PauseReason = ""
	cb.PausedUntil = time.Time{}

	log.Printf("[MODE-CIRCUIT-BREAKER] %s: Manually reset - trading resumed", mode)
	ga.logger.Info("Mode circuit breaker manually reset", "mode", mode)

	return nil
}

// ==================== Adaptive AI & LLM Diagnostics (Story 2.8) ====================

// AdaptiveAIData holds adaptive AI recommendations and statistics
type AdaptiveAIData struct {
	Recommendations []AdaptiveRecommendationData `json:"recommendations"`
	Statistics      map[string]ModeStatisticsData `json:"statistics"`
	LastAnalysis    time.Time                     `json:"last_analysis"`
	TotalOutcomes   int                           `json:"total_outcomes"`
}

// AdaptiveRecommendationData represents a single adaptive AI recommendation
type AdaptiveRecommendationData struct {
	ID             string    `json:"id"`
	Mode           string    `json:"mode"`
	Parameter      string    `json:"parameter"`
	CurrentValue   float64   `json:"current_value"`
	SuggestedValue float64   `json:"suggested_value"`
	Reasoning      string    `json:"reasoning"`
	Confidence     float64   `json:"confidence"`
	Impact         string    `json:"impact"`
	CreatedAt      time.Time `json:"created_at"`
	Status         string    `json:"status"`
}

// ModeStatisticsData represents statistics for a trading mode
type ModeStatisticsData struct {
	TotalTrades int       `json:"total_trades"`
	WinCount    int       `json:"win_count"`
	LossCount   int       `json:"loss_count"`
	WinRate     float64   `json:"win_rate"`
	TotalPnL    float64   `json:"total_pnl"`
	AvgPnL      float64   `json:"avg_pnl"`
	AvgHoldTime string    `json:"avg_hold_time"`
	LLMAccuracy float64   `json:"llm_accuracy"`
	LastUpdated time.Time `json:"last_updated"`
}

// LLMDiagnosticsData holds LLM call statistics
type LLMDiagnosticsData struct {
	TotalCalls      int64              `json:"total_calls"`
	CacheHits       int64              `json:"cache_hits"`
	CacheMisses     int64              `json:"cache_misses"`
	CacheHitRate    float64            `json:"cache_hit_rate"`
	AvgLatencyMs    float64            `json:"avg_latency_ms"`
	ErrorCount      int64              `json:"error_count"`
	ErrorRate       float64            `json:"error_rate"`
	CallsByProvider map[string]int64   `json:"calls_by_provider"`
	RecentErrors    []LLMErrorData     `json:"recent_errors"`
	LastResetAt     time.Time          `json:"last_reset_at"`
}

// LLMErrorData represents a recent LLM error
type LLMErrorData struct {
	Timestamp time.Time `json:"timestamp"`
	Provider  string    `json:"provider"`
	ErrorType string    `json:"error_type"`
	Message   string    `json:"message"`
	Symbol    string    `json:"symbol,omitempty"`
}

// TradeWithAIContextData represents a trade with AI decision context
type TradeWithAIContextData struct {
	TradeID       string     `json:"trade_id"`
	Symbol        string     `json:"symbol"`
	Side          string     `json:"side"`
	Mode          string     `json:"mode"`
	EntryPrice    float64    `json:"entry_price"`
	ExitPrice     float64    `json:"exit_price,omitempty"`
	PnL           float64    `json:"pnl"`
	PnLPercent    float64    `json:"pnl_percent"`
	Status        string     `json:"status"`
	OpenedAt      time.Time  `json:"opened_at"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
	AIReasoning   string     `json:"ai_reasoning"`
	LLMConfidence float64    `json:"llm_confidence"`
	LLMProvider   string     `json:"llm_provider,omitempty"`
	SignalSource  string     `json:"signal_source"`
}

// GetAdaptiveAIData returns adaptive AI recommendations and statistics
func (ga *GinieAutopilot) GetAdaptiveAIData() *AdaptiveAIData {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	// Build mode statistics from trade history
	statistics := make(map[string]ModeStatisticsData)
	modeTradeCount := make(map[string]int)
	modeWinCount := make(map[string]int)
	modePnL := make(map[string]float64)

	for _, trade := range ga.tradeHistory {
		mode := string(trade.Mode)
		modeTradeCount[mode]++
		modePnL[mode] += trade.PnL
		if trade.PnL > 0 {
			modeWinCount[mode]++
		}
	}

	for mode, count := range modeTradeCount {
		winRate := 0.0
		if count > 0 {
			winRate = float64(modeWinCount[mode]) / float64(count) * 100
		}
		avgPnL := 0.0
		if count > 0 {
			avgPnL = modePnL[mode] / float64(count)
		}

		statistics[mode] = ModeStatisticsData{
			TotalTrades: count,
			WinCount:    modeWinCount[mode],
			LossCount:   count - modeWinCount[mode],
			WinRate:     winRate,
			TotalPnL:    modePnL[mode],
			AvgPnL:      avgPnL,
			AvgHoldTime: "N/A", // TODO: Calculate from trade durations
			LLMAccuracy: 0,     // TODO: Track LLM prediction accuracy
			LastUpdated: time.Now(),
		}
	}

	// Get real recommendations from AdaptiveAI engine
	var recommendations []AdaptiveRecommendationData
	var lastAnalysis time.Time
	var totalOutcomes int

	if ga.adaptiveAI != nil {
		// Get pending recommendations from AdaptiveAI
		pendingRecs := ga.adaptiveAI.GetPendingRecommendations()
		recommendations = make([]AdaptiveRecommendationData, 0, len(pendingRecs))

		for _, rec := range pendingRecs {
			// Convert CurrentValue to float64
			currentVal := 0.0
			switch v := rec.CurrentValue.(type) {
			case float64:
				currentVal = v
			case float32:
				currentVal = float64(v)
			case int:
				currentVal = float64(v)
			case int64:
				currentVal = float64(v)
			}

			// Convert SuggestedValue to float64
			suggestedVal := 0.0
			switch v := rec.SuggestedValue.(type) {
			case float64:
				suggestedVal = v
			case float32:
				suggestedVal = float64(v)
			case int:
				suggestedVal = float64(v)
			case int64:
				suggestedVal = float64(v)
			}

			// Determine status
			status := "pending"
			if rec.AppliedAt != nil {
				status = "applied"
			} else if rec.Dismissed {
				status = "dismissed"
			}

			recommendations = append(recommendations, AdaptiveRecommendationData{
				ID:             rec.ID,
				Mode:           string(rec.Mode),
				Parameter:      rec.Type,
				CurrentValue:   currentVal,
				SuggestedValue: suggestedVal,
				Reasoning:      rec.Reason,
				Confidence:     0.0, // AdaptiveRecommendation doesn't have confidence field
				Impact:         rec.ExpectedImprovement,
				CreatedAt:      rec.CreatedAt,
				Status:         status,
			})
		}

		// Get statistics from AdaptiveAI
		analysisState := ga.adaptiveAI.GetAnalysisState()
		if la, ok := analysisState["last_analysis"].(time.Time); ok {
			lastAnalysis = la
		} else {
			lastAnalysis = time.Now()
		}
		if to, ok := analysisState["total_outcomes"].(int); ok {
			totalOutcomes = to
		} else {
			totalOutcomes = len(ga.tradeHistory)
		}
	} else {
		recommendations = []AdaptiveRecommendationData{}
		lastAnalysis = time.Now()
		totalOutcomes = len(ga.tradeHistory)
	}

	return &AdaptiveAIData{
		Recommendations: recommendations,
		Statistics:      statistics,
		LastAnalysis:    lastAnalysis,
		TotalOutcomes:   totalOutcomes,
	}
}

// ApplyAdaptiveRecommendation applies a specific recommendation by ID
func (ga *GinieAutopilot) ApplyAdaptiveRecommendation(recommendationID string) error {
	if ga.adaptiveAI == nil {
		return errors.New("AdaptiveAI engine not initialized")
	}
	return ga.adaptiveAI.ApplyRecommendation(recommendationID)
}

// DismissAdaptiveRecommendation dismisses a specific recommendation
func (ga *GinieAutopilot) DismissAdaptiveRecommendation(recommendationID string) error {
	if ga.adaptiveAI == nil {
		return errors.New("AdaptiveAI engine not initialized")
	}
	return ga.adaptiveAI.DismissRecommendation(recommendationID)
}

// ApplyAllAdaptiveRecommendations applies all pending recommendations
func (ga *GinieAutopilot) ApplyAllAdaptiveRecommendations() (int, error) {
	if ga.adaptiveAI == nil {
		return 0, errors.New("AdaptiveAI engine not initialized")
	}

	pending := ga.adaptiveAI.GetPendingRecommendations()
	applied := 0
	var lastErr error

	for _, rec := range pending {
		if err := ga.adaptiveAI.ApplyRecommendation(rec.ID); err != nil {
			lastErr = err
		} else {
			applied++
		}
	}

	if applied == 0 && lastErr != nil {
		return 0, lastErr
	}
	return applied, nil
}

// GetLLMDiagnosticsData returns LLM call statistics
func (ga *GinieAutopilot) GetLLMDiagnosticsData() *LLMDiagnosticsData {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	// Calculate statistics from LLM switch events
	var totalCalls int64
	var totalLatency float64
	callsByProvider := make(map[string]int64)

	for _, event := range ga.llmSwitches {
		totalCalls++
		// Use Action as provider category since LLMSwitchEvent doesn't have Provider field
		callsByProvider[event.Action]++
		_ = event // Note: Latency not tracked in LLMSwitchEvent currently
	}

	avgLatency := 0.0
	if totalCalls > 0 {
		avgLatency = totalLatency / float64(totalCalls)
	}

	return &LLMDiagnosticsData{
		TotalCalls:      totalCalls,
		CacheHits:       0, // TODO: Track cache hits when caching is implemented
		CacheMisses:     totalCalls,
		CacheHitRate:    0,
		AvgLatencyMs:    avgLatency,
		ErrorCount:      0, // TODO: Track errors when error tracking is implemented
		ErrorRate:       0,
		CallsByProvider: callsByProvider,
		RecentErrors:    []LLMErrorData{},
		LastResetAt:     time.Now().Add(-24 * time.Hour), // Placeholder
	}
}

// ResetLLMDiagnostics resets LLM diagnostic counters
func (ga *GinieAutopilot) ResetLLMDiagnostics() {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	// Clear LLM switch history
	ga.llmSwitches = make([]LLMSwitchEvent, 0)

	ga.logger.Info("LLM diagnostics reset")
}

// GetTradeHistoryWithAIContext returns trade history with AI decision context
func (ga *GinieAutopilot) GetTradeHistoryWithAIContext(limit, offset int) []TradeWithAIContextData {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	trades := make([]TradeWithAIContextData, 0)

	// Get trades in reverse order (newest first)
	historyLen := len(ga.tradeHistory)
	startIdx := historyLen - 1 - offset
	endIdx := startIdx - limit
	if endIdx < -1 {
		endIdx = -1
	}

	for i := startIdx; i > endIdx && i >= 0; i-- {
		trade := ga.tradeHistory[i]

		// Build AI reasoning from signal summary if available
		aiReasoning := "No AI reasoning available"
		llmConfidence := trade.Confidence
		if trade.SignalSummary != nil {
			aiReasoning = fmt.Sprintf("Signal Strength: %s, Direction: %s", trade.SignalSummary.Strength, trade.SignalSummary.Direction)
		}
		if trade.Reason != "" {
			aiReasoning = trade.Reason
		}

		// Use Price as both entry and exit for now (trade result only has one price)
		tradeData := TradeWithAIContextData{
			TradeID:       fmt.Sprintf("%s-%d", trade.Symbol, trade.Timestamp.Unix()),
			Symbol:        trade.Symbol,
			Side:          trade.Side,
			Mode:          string(trade.Mode),
			EntryPrice:    trade.Price,
			ExitPrice:     trade.Price,
			PnL:           trade.PnL,
			PnLPercent:    trade.PnLPercent,
			Status:        "closed",
			OpenedAt:      trade.Timestamp,
			AIReasoning:   aiReasoning,
			LLMConfidence: llmConfidence,
			SignalSource:  trade.Source,
		}

		// Mark as closed since trade result represents completed trade
		closedAt := trade.Timestamp
		tradeData.ClosedAt = &closedAt

		trades = append(trades, tradeData)
	}

	return trades
}

// GetTradeHistoryCount returns total number of trades in history
func (ga *GinieAutopilot) GetTradeHistoryCount() int {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	return len(ga.tradeHistory)
}
