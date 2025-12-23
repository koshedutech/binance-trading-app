package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/circuit"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/logging"
	"context"
	"fmt"
	"log"
	"math"
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
	EnableScalpMode    bool `json:"enable_scalp_mode"`
	EnableSwingMode    bool `json:"enable_swing_mode"`
	EnablePositionMode bool `json:"enable_position_mode"`

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

		// Proactive profit protection (NEW - prevents BCHUSDT-style losses)
		ProactiveBreakevenPercent:  0.5,  // Move to breakeven when profit >= 0.5% (before TP1)
		TrailingActivationPercent:  2.0,  // Activate trailing when profit >= 2.0% (increased to align with TP1 gain of 1.5%)
		TrailingStepPercent:        0.5,  // Trail by 0.5% from highest price
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

		MinConfidenceToTrade: 65.0,
		MaxDailyTrades:       1000,
		MaxDailyLoss:         500,

		// Circuit breaker defaults
		CircuitBreakerEnabled:  true,
		CBMaxLossPerHour:       100,  // $100 max loss per hour
		CBMaxDailyLoss:         300,  // $300 max daily loss
		CBMaxConsecutiveLosses: 3,    // 3 consecutive losses triggers cooldown
		CBCooldownMinutes:      30,   // 30 minute cooldown
	}
}

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
	TrailingActive   bool    `json:"trailing_active"`
	HighestPrice     float64 `json:"highest_price"`
	LowestPrice      float64 `json:"lowest_price"`
	TrailingPercent  float64 `json:"trailing_percent"` // Dynamic trailing %

	// Algo Order IDs (for Binance SL/TP orders)
	StopLossAlgoID   int64   `json:"stop_loss_algo_id,omitempty"`   // Binance algo order ID for SL
	TakeProfitAlgoIDs []int64 `json:"take_profit_algo_ids,omitempty"` // Binance algo order IDs for TPs
	LastLLMUpdate    time.Time `json:"last_llm_update,omitempty"`   // Last LLM SL/TP update time

	// Tracking
	RealizedPnL      float64 `json:"realized_pnl"`     // From partial closes
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	DecisionReport   *GinieDecisionReport `json:"decision_report,omitempty"`

	// Trade Source Tracking
	Source       string  `json:"source"`                   // "ai" or "strategy"
	StrategyID   *int64  `json:"strategy_id,omitempty"`    // Strategy ID if source is "strategy"
	StrategyName *string `json:"strategy_name,omitempty"`  // Strategy name for display

	// Ultra-Fast Scalping Mode
	UltraFastSignal       *UltraFastSignal `json:"ultra_fast_signal,omitempty"`       // Signal that triggered entry
	UltraFastTargetPercent float64         `json:"ultra_fast_target_percent,omitempty"` // Fee-aware profit target %
	MaxHoldTime           time.Duration    `json:"max_hold_time,omitempty"`            // 3s for ultra-fast
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
}

// SignalDiagnostics shows signal generation stats
type SignalDiagnostics struct {
	TotalGenerated      int            `json:"total_generated"`
	Executed            int            `json:"executed"`
	Rejected            int            `json:"rejected"`
	ExecutionRate       float64        `json:"execution_rate_pct"`
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

	// Performance stats
	totalTrades   int
	winningTrades int
	totalPnL      float64

	// Diagnostic tracking
	lastScalpScan          time.Time
	lastSwingScan          time.Time
	lastPositionScan       time.Time
	symbolsScannedLastCycle int
	failedOrdersLastHour   int
	tpHitsLastHour         int
	partialClosesLastHour  int
	lastLLMCallTime        time.Time

	// Balance caching (to avoid blocking API calls)
	cachedAvailableBalance float64
	cachedWalletBalance    float64
	lastBalanceUpdateTime  time.Time

	// Strategy Evaluation
	strategyEvaluator *StrategyEvaluator
	lastStrategyScan  time.Time
}

// NewGinieAutopilot creates a new Ginie autonomous trading system
func NewGinieAutopilot(
	analyzer *GinieAnalyzer,
	futuresClient binance.FuturesClient,
	logger *logging.Logger,
	repo *database.Repository,
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
		dayStart:             time.Now().Truncate(24 * time.Hour),
		volatilityRegimes:    make(map[string]*VolatilityRegime),
		lastRegimeUpdate:     make(map[string]time.Time),
		modeAllocationStates: make(map[string]*ModeAllocationState),
		modeUsedUSD:          make(map[string]float64),
		modePositionCounts:   make(map[string]int),
		modeSafetyStates:     make(map[string]*ModeSafetyState),
		modeSafetyConfigs:    make(map[string]*ModeSafetyConfig),
		lastDayReset:         time.Now().Truncate(24 * time.Hour),
	}

	// Initialize safety configs and states from settings
	settingsManager := GetSettingsManager()
	settings := settingsManager.GetCurrentSettings()

	// Load safety configs
	ga.modeSafetyConfigs["ultra_fast"] = settings.SafetyUltraFast
	ga.modeSafetyConfigs["scalp"] = settings.SafetyScalp
	ga.modeSafetyConfigs["swing"] = settings.SafetySwing
	ga.modeSafetyConfigs["position"] = settings.SafetyPosition

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

	return ga
}

// LoadPnLStats loads persisted PnL statistics from settings
func (ga *GinieAutopilot) LoadPnLStats() {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	sm := GetSettingsManager()
	totalPnL, dailyPnL, totalTrades, winningTrades, dailyTrades := sm.GetGiniePnLStats()

	ga.totalPnL = totalPnL
	ga.dailyPnL = dailyPnL
	ga.totalTrades = totalTrades
	ga.winningTrades = winningTrades
	ga.dailyTrades = dailyTrades

	ga.logger.Info("Loaded Ginie PnL stats from settings",
		"total_pnl", totalPnL,
		"daily_pnl", dailyPnL,
		"total_trades", totalTrades,
		"winning_trades", winningTrades,
		"daily_trades", dailyTrades)
}

// SavePnLStats persists current PnL statistics to settings
func (ga *GinieAutopilot) SavePnLStats() {
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
	defer ga.mu.Unlock()
	ga.futuresClient = client
	ga.logger.Info("Ginie futures client updated")
}

func (ga *GinieAutopilot) SetLLMAnalyzer(analyzer *llm.Analyzer) {
	ga.mu.Lock()
	defer ga.mu.Unlock()
	ga.llmAnalyzer = analyzer
	if analyzer != nil {
		ga.logger.Info("LLM analyzer set for Ginie adaptive SL/TP")
	}
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
		ga.config.MinConfidenceToTrade = 75.0
		ga.config.MaxUSDPerPosition = 300
		ga.config.DefaultLeverage = 3
	case "moderate":
		ga.config.MinConfidenceToTrade = 65.0
		ga.config.MaxUSDPerPosition = 500
		ga.config.DefaultLeverage = 5
	case "aggressive":
		ga.config.MinConfidenceToTrade = 55.0
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

// Start starts the Ginie autopilot
func (ga *GinieAutopilot) Start() error {
	ga.logger.Info("Ginie Start() called", "dry_run", ga.config.DryRun, "current_running", ga.running)

	// Sync positions with exchange before starting (don't hold main lock)
	if !ga.config.DryRun {
		synced, err := ga.SyncWithExchange()
		if err != nil {
			ga.logger.Warn("Failed to sync positions on start", "error", err)
		} else if synced > 0 {
			ga.logger.Info("Synced positions from exchange on start", "count", synced)
		}

		// Place SL/TP orders for all existing positions (including those synced during initialization)
		ga.placeSLTPOrdersForSyncedPositions()

		// CRITICAL: Run comprehensive orphan order cleanup at startup
		log.Printf("[GINIE] Running startup orphan order cleanup...")
		ga.cleanupAllOrphanOrders()
	}

	ga.mu.Lock()
	defer ga.mu.Unlock()

	if ga.running {
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

	// Start periodic orphan order cleanup (every 5 minutes)
	// This prevents order accumulation from position updates and failed cancellations
	ga.wg.Add(1)
	go ga.periodicOrphanOrderCleanup()

	ga.logger.Info("Ginie Autopilot fully started - all monitors running")
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

	ga.wg.Wait()
	ga.logger.Info("Ginie Autopilot stopped")
	return nil
}

// runMainLoop is the main trading loop that scans for opportunities
func (ga *GinieAutopilot) runMainLoop() {
	defer ga.wg.Done()

	ga.logger.Info("Ginie main loop started")

	// Use the shortest enabled interval as base, then check mode-specific timing
	baseTicker := time.NewTicker(time.Duration(ga.config.ScalpScanInterval) * time.Second)
	defer baseTicker.Stop()

	// Track last scan times for each mode
	lastScalpScan := time.Now()
	lastSwingScan := time.Now()
	lastPositionScan := time.Now()
	lastStrategyScan := time.Now()

	for {
		select {
		case <-ga.stopChan:
			ga.logger.Info("Ginie main loop stopping")
			return
		case <-baseTicker.C:
			now := time.Now()

			// Check if we can trade
			if !ga.canTrade() {
				continue
			}

			ga.logger.Debug("Ginie canTrade passed, proceeding with scans")

			// Scan based on mode intervals
			if ga.config.EnableScalpMode && now.Sub(lastScalpScan) >= time.Duration(ga.config.ScalpScanInterval)*time.Second {
				ga.scanForMode(GinieModeScalp)
				lastScalpScan = now
			}

			if ga.config.EnableSwingMode && now.Sub(lastSwingScan) >= time.Duration(ga.config.SwingScanInterval)*time.Second {
				ga.scanForMode(GinieModeSwing)
				lastSwingScan = now
			}

			if ga.config.EnablePositionMode && now.Sub(lastPositionScan) >= time.Duration(ga.config.PositionScanInterval)*time.Second {
				ga.scanForMode(GinieModePosition)
				lastPositionScan = now
			}

			// Scan saved strategies (every 60 seconds)
			if ga.strategyEvaluator != nil && now.Sub(lastStrategyScan) >= 60*time.Second {
				ga.scanStrategies()
				lastStrategyScan = now
			}
		}
	}
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

// scanForMode scans all watched symbols for a specific trading mode
func (ga *GinieAutopilot) scanForMode(mode GinieTradingMode) {
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
	ga.mu.Unlock()

	ga.logger.Info("Ginie scanning for mode", "mode", mode, "symbols", len(symbols))

	for _, symbol := range symbols {
		select {
		case <-ga.stopChan:
			return
		default:
			// Skip if we already have a position
			ga.mu.RLock()
			_, hasPosition := ga.positions[symbol]
			ga.mu.RUnlock()

			if hasPosition {
				continue
			}

			// Generate decision for this symbol
			decision, err := ga.analyzer.GenerateDecision(symbol)
			if err != nil {
				ga.logger.Error("Ginie decision generation failed", "symbol", symbol, "error", err)
				continue
			}

			// Check if the mode matches what we're looking for
			if decision.SelectedMode != mode {
				continue
			}

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

			// Check recommendation
			if decision.Recommendation != RecommendationExecute {
				signalLog.Status = "rejected"
				signalLog.RejectionReason = "not_recommended"
				ga.LogSignal(signalLog)
				continue
			}

			// Check if symbol is enabled (per-symbol settings)
			settingsManager := GetSettingsManager()
			if !settingsManager.IsSymbolEnabled(symbol) {
				signalLog.Status = "rejected"
				signalLog.RejectionReason = "symbol_disabled"
				ga.LogSignal(signalLog)
				continue
			}

			// Get effective confidence threshold for this symbol (considers performance category)
			effectiveMinConfidence := settingsManager.GetEffectiveConfidence(symbol, ga.config.MinConfidenceToTrade)

			// Check confidence threshold (both are 0-100 format)
			if decision.ConfidenceScore < effectiveMinConfidence {
				ga.logger.Debug("Ginie skipping low confidence signal",
					"symbol", symbol,
					"confidence", decision.ConfidenceScore,
					"min_required", effectiveMinConfidence,
					"global_min", ga.config.MinConfidenceToTrade)
				signalLog.Status = "rejected"
				signalLog.RejectionReason = fmt.Sprintf("low_confidence (%.1f < %.1f)", decision.ConfidenceScore, effectiveMinConfidence)
				ga.LogSignal(signalLog)
				continue
			}

			// Check if coin is blocked
			if blocked, reason := ga.isCoinBlocked(symbol); blocked {
				signalLog.Status = "rejected"
				signalLog.RejectionReason = "coin_blocked: " + reason
				ga.LogSignal(signalLog)
				continue
			}

			ga.logger.Info("Ginie found tradeable signal - attempting trade",
				"symbol", symbol,
				"confidence", decision.ConfidenceScore,
				"min_required", effectiveMinConfidence,
				"action", decision.TradeExecution.Action,
				"mode", decision.SelectedMode)

			// Log as executed (will be attempted)
			signalLog.Status = "executed"
			ga.LogSignal(signalLog)

			// Execute the trade
			ga.executeTrade(decision)
		}
	}
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
func (ga *GinieAutopilot) calculateAdaptivePositionSize(symbol string, confidence float64, currentPositionCount int) (positionUSD float64, canTrade bool, reason string) {
	// Get actual available balance from Binance
	availableBalance, err := ga.getAvailableBalance()
	if err != nil {
		ga.logger.Error("Failed to get available balance", "error", err)
		return 0, false, "cannot fetch balance"
	}

	// Safety margin: only use 90% of available balance to avoid margin issues
	safetyMargin := 0.90
	usableBalance := availableBalance * safetyMargin

	// Check minimum balance threshold
	minBalanceRequired := 25.0 // At least $25 to trade
	if usableBalance < minBalanceRequired {
		return 0, false, fmt.Sprintf("insufficient balance: $%.2f (need $%.2f)", usableBalance, minBalanceRequired)
	}

	// Use position count passed from caller (captured while holding lock)
	maxPositions := ga.config.MaxPositions
	availableSlots := maxPositions - currentPositionCount

	if availableSlots <= 0 {
		return 0, false, fmt.Sprintf("max positions reached: %d/%d", currentPositionCount, maxPositions)
	}

	// Calculate allocation per potential new position
	// Divide usable balance across available slots
	baseAllocationPerPosition := usableBalance / float64(availableSlots)

	// Adjust based on risk level
	riskMultiplier := 1.0
	switch ga.currentRiskLevel {
	case "conservative":
		riskMultiplier = 0.6 // 60% of base allocation
	case "moderate":
		riskMultiplier = 0.8 // 80% of base allocation
	case "aggressive":
		riskMultiplier = 1.0 // 100% of base allocation
	}

	// Adjust based on confidence (higher confidence = more allocation)
	// Scale: 65% confidence = 0.8x, 80% confidence = 1.0x, 95% confidence = 1.15x
	confidenceMultiplier := 0.5 + (confidence / 100.0 * 0.7) // Range: 0.5 to 1.2

	// Get per-symbol size multiplier based on performance category
	settingsManager := GetSettingsManager()
	effectiveMaxUSD := settingsManager.GetEffectivePositionSize(symbol, ga.config.MaxUSDPerPosition)
	symbolSettings := settingsManager.GetSymbolSettings(symbol)

	// Calculate final position size
	positionUSD = baseAllocationPerPosition * riskMultiplier * confidenceMultiplier

	// Cap at effective max USD per position (adjusted for symbol performance)
	if positionUSD > effectiveMaxUSD {
		positionUSD = effectiveMaxUSD
	}

	// Log per-symbol adjustment if different from global
	if effectiveMaxUSD != ga.config.MaxUSDPerPosition {
		ga.logger.Debug("Per-symbol size adjustment applied",
			"symbol", symbol,
			"category", symbolSettings.Category,
			"global_max_usd", ga.config.MaxUSDPerPosition,
			"effective_max_usd", effectiveMaxUSD)
	}

	// Minimum position size check
	minPositionSize := 10.0 // At least $10 per position
	if positionUSD < minPositionSize {
		return 0, false, fmt.Sprintf("calculated position too small: $%.2f (min $%.2f)", positionUSD, minPositionSize)
	}

	ga.logger.Info("Adaptive position sizing",
		"available_balance", fmt.Sprintf("$%.2f", availableBalance),
		"usable_balance", fmt.Sprintf("$%.2f", usableBalance),
		"current_positions", currentPositionCount,
		"available_slots", availableSlots,
		"base_allocation", fmt.Sprintf("$%.2f", baseAllocationPerPosition),
		"risk_level", ga.currentRiskLevel,
		"confidence", fmt.Sprintf("%.1f%%", confidence),
		"final_position_usd", fmt.Sprintf("$%.2f", positionUSD))

	return positionUSD, true, ""
}

// ==================== FUNDING RATE AWARENESS ====================

// checkFundingRate checks if trade should be blocked due to high funding rate near funding time
// Returns (shouldBlock bool, reason string)
func (ga *GinieAutopilot) checkFundingRate(symbol string, isLong bool) (bool, string) {
	fundingRate, err := ga.futuresClient.GetFundingRate(symbol)
	if err != nil || fundingRate == nil {
		return false, "" // Allow if can't check
	}

	maxRate := 0.001 // 0.1% threshold

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

	// Block if funding costs us money AND rate is high AND near funding time (30 min)
	if fundingCost > maxRate && timeToFunding > 0 && timeToFunding < 30*time.Minute {
		return true, fmt.Sprintf("High funding %.4f%% costs us in %v", fundingCost*100, timeToFunding.Round(time.Minute))
	}

	return false, ""
}

// shouldExitBeforeFunding checks if we should close position to avoid funding fee
// Returns (shouldExit bool, reason string)
func (ga *GinieAutopilot) shouldExitBeforeFunding(pos *GiniePosition) (bool, string) {
	fundingRate, err := ga.futuresClient.GetFundingRate(pos.Symbol)
	if err != nil || fundingRate == nil {
		return false, ""
	}

	now := time.Now().UTC()
	nextFunding := time.Unix(fundingRate.NextFundingTime/1000, 0)
	timeToFunding := nextFunding.Sub(now)

	// Only consider if within 10 minutes of funding
	if timeToFunding > 10*time.Minute || timeToFunding < 0 {
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

		// Exit if profitable AND funding fee would eat >30% of profit
		if currentPnL > 0 && fundingFee > currentPnL*0.3 {
			return true, fmt.Sprintf("Exit before funding: PnL $%.2f, fee would be $%.4f (%.1f%% of profit)",
				currentPnL, fundingFee, (fundingFee/currentPnL)*100)
		}

		// Exit if funding rate is extreme (>0.3%)
		if fundingCost > 0.003 {
			return true, fmt.Sprintf("Exit: extreme funding rate %.4f%% in %v", fundingCost*100, timeToFunding.Round(time.Minute))
		}
	}

	return false, ""
}

// adjustSizeForFunding reduces position size when funding rate is costly
func (ga *GinieAutopilot) adjustSizeForFunding(symbol string, baseSize float64, isLong bool) float64 {
	fundingRate, err := ga.futuresClient.GetFundingRate(symbol)
	if err != nil || fundingRate == nil {
		return baseSize
	}

	fundingCost := fundingRate.FundingRate
	if !isLong {
		fundingCost = -fundingCost
	}

	// Only adjust if funding costs us money
	if fundingCost > 0 {
		if fundingCost > 0.002 { // > 0.2%
			ga.logger.Info("Funding rate high - reducing position 50%",
				"symbol", symbol, "funding_rate", fundingCost*100, "original_size", baseSize)
			return baseSize * 0.5
		}
		if fundingCost > 0.001 { // > 0.1%
			ga.logger.Info("Funding rate elevated - reducing position 25%",
				"symbol", symbol, "funding_rate", fundingCost*100, "original_size", baseSize)
			return baseSize * 0.75
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

	// Check funding rate before entry - avoid high fees near funding time
	isLong := decision.TradeExecution.Action == "LONG"
	if blocked, reason := ga.checkFundingRate(symbol, isLong); blocked {
		ga.logger.Warn("Ginie skipping trade - funding rate concern",
			"symbol", symbol,
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
	positionUSD, canTrade, reason := ga.calculateAdaptivePositionSize(symbol, decision.ConfidenceScore, currentPositionCount)
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
	positionUSD = ga.adjustSizeForFunding(symbol, positionUSD, isLong)

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
		takeProfits = ga.generateDefaultTPs(price, decision.SelectedMode, decision.TradeExecution.Action == "LONG")
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
			PositionSide: positionSide,
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
	position := &GiniePosition{
		Symbol:          symbol,
		Side:            decision.TradeExecution.Action,
		Mode:            decision.SelectedMode,
		EntryPrice:      actualPrice,
		OriginalQty:     actualQty,
		RemainingQty:    actualQty,
		Leverage:        leverage,
		EntryTime:       time.Now(),
		TakeProfits:     takeProfits,
		CurrentTPLevel:  0,
		StopLoss:        decision.TradeExecution.StopLoss,
		OriginalSL:      decision.TradeExecution.StopLoss,
		MovedToBreakeven: false,
		TrailingActive:  false,
		HighestPrice:    actualPrice,
		LowestPrice:     actualPrice,
		TrailingPercent: ga.getTrailingPercent(decision.SelectedMode),
		DecisionReport:  decision,
		Source:          "ai", // AI-based trade
	}

	ga.positions[symbol] = position
	ga.dailyTrades++
	ga.totalTrades++

	// Place SL/TP orders on Binance (if not dry run)
	if !ga.config.DryRun {
		ga.placeSLTPOrders(position)
	}

	// Build signal names for summary
	signalNames := make([]string, 0)
	for _, sig := range decision.SignalAnalysis.PrimarySignals {
		if sig.Met {
			signalNames = append(signalNames, sig.Name)
		}
	}

	// Build TP prices array
	tpPrices := make([]float64, len(decision.TradeExecution.TakeProfits))
	for i, tp := range decision.TradeExecution.TakeProfits {
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
			EntryPrice:  price,
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
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if len(ga.positions) > 0 {
		log.Printf("[GINIE-MONITOR] Checking %d positions for trailing/TP/SL", len(ga.positions))
	}

	for symbol, pos := range ga.positions {
		// Get current price
		currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
		if err != nil {
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
			ga.updateBinanceSLOrder(pos)
		}

		// 2. Trailing SL: Activate early trailing when profit >= threshold
		if !pos.TrailingActive && pnlPercent >= ga.config.TrailingActivationPercent {
			pos.TrailingActive = true
			ga.logger.Info("Early trailing activated",
				"symbol", pos.Symbol,
				"pnl_percent", pnlPercent,
				"threshold", ga.config.TrailingActivationPercent)
		}

		// 3. Trail SL upward: Update SL as price moves favorably
		if pos.TrailingActive && ga.config.TrailingStepPercent > 0 {
			var newTrailingSL float64
			if pos.Side == "LONG" {
				// For longs: trail from highest price
				newTrailingSL = pos.HighestPrice * (1 - ga.config.TrailingStepPercent/100)
			} else {
				// For shorts: trail from lowest price
				newTrailingSL = pos.LowestPrice * (1 + ga.config.TrailingStepPercent/100)
			}

			// Only move SL in profitable direction (never lower for longs, never higher for shorts)
			slImproved := false
			if pos.Side == "LONG" && newTrailingSL > pos.StopLoss {
				slImprovement := (newTrailingSL - pos.StopLoss) / pos.EntryPrice * 100
				if slImprovement >= ga.config.TrailingSLUpdateThreshold {
					oldSL := pos.StopLoss
					pos.StopLoss = newTrailingSL
					slImproved = true
					ga.logger.Info("Trailing SL moved up (LONG)",
						"symbol", pos.Symbol,
						"old_sl", oldSL,
						"new_sl", newTrailingSL,
						"highest_price", pos.HighestPrice,
						"improvement_pct", slImprovement)
				}
			} else if pos.Side == "SHORT" && newTrailingSL < pos.StopLoss {
				slImprovement := (pos.StopLoss - newTrailingSL) / pos.EntryPrice * 100
				if slImprovement >= ga.config.TrailingSLUpdateThreshold {
					oldSL := pos.StopLoss
					pos.StopLoss = newTrailingSL
					slImproved = true
					ga.logger.Info("Trailing SL moved down (SHORT)",
						"symbol", pos.Symbol,
						"old_sl", oldSL,
						"new_sl", newTrailingSL,
						"lowest_price", pos.LowestPrice,
						"improvement_pct", slImprovement)
				}
			}

			// Update Binance order if SL improved significantly
			if slImproved {
				ga.updateBinanceSLOrder(pos)
			}
		}
		// === END PROACTIVE PROFIT PROTECTION ===

		// Check Stop Loss
		if ga.checkStopLoss(pos, currentPrice) {
			ga.closePosition(symbol, pos, currentPrice, "stop_loss", 0)
			continue
		}

		// Check Take Profit levels (process one at a time)
		tpHit := ga.checkTakeProfits(pos, currentPrice, pnlPercent)
		if tpHit > 0 && tpHit <= len(pos.TakeProfits) {
			// Partial close for TP1-3, handled by checkTakeProfits
			continue
		}

		// Check trailing stop (for TP4 / final portion) - now also triggers earlier if trailing active
		if pos.TrailingActive {
			if ga.checkTrailingStop(pos, currentPrice) {
				reason := "trailing_stop"
				if pos.CurrentTPLevel >= 3 {
					reason = "trailing_stop_tp4"
				}
				ga.closePosition(symbol, pos, currentPrice, reason, pos.CurrentTPLevel)
			}
		}
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

			// Handle TP1-3: partial close
			if tpLevel <= 3 {
				ga.executePartialClose(pos, currentPrice, tpLevel)
			} else {
				// TP4: activate trailing for remaining position
				pos.TrailingActive = true
				ga.logger.Info("Ginie TP4 hit - trailing activated",
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
				ga.placeNextTPOrder(pos, tpLevel)
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
	closeQty := roundQuantity(pos.Symbol, pos.OriginalQty * closePercent)

	if closeQty <= 0 || closeQty > pos.RemainingQty {
		return
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
			PositionSide: positionSide,
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
}

// placeNextTPOrder places the next TP order on Binance after a TP level is hit
func (ga *GinieAutopilot) placeNextTPOrder(pos *GiniePosition, currentTPLevel int) {
	nextTPIndex := currentTPLevel // currentTPLevel is 1-based, so index for next is same as level
	if nextTPIndex >= len(pos.TakeProfits) {
		return // No more TP levels
	}

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
			PositionSide: positionSide,
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
			}
		}
		return
	}

	// Normal case - place TP algo order
	tpParams := binance.AlgoOrderParams{
		Symbol:       pos.Symbol,
		Side:         closeSide,
		PositionSide: positionSide,
		Type:         binance.FuturesOrderTypeTakeProfitMarket,
		Quantity:     tpQty,
		TriggerPrice: roundedTPPrice,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	tpOrder, err := ga.futuresClient.PlaceAlgoOrder(tpParams)
	if err != nil {
		ga.logger.Error("Failed to place next take profit order",
			"symbol", pos.Symbol,
			"tp_level", nextTPIndex+1,
			"tp_price", nextTP.Price,
			"error", err.Error())
		return
	}

	pos.TakeProfitAlgoIDs = append(pos.TakeProfitAlgoIDs, tpOrder.AlgoId)
	ga.logger.Info("Next take profit order placed",
		"symbol", pos.Symbol,
		"tp_level", nextTPIndex+1,
		"algo_id", tpOrder.AlgoId,
		"trigger_price", roundedTPPrice,
		"quantity", tpQty)
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
func (ga *GinieAutopilot) placeSLOrder(pos *GiniePosition) {
	closeSide := "SELL"
	positionSide := binance.PositionSideLong
	if pos.Side == "SHORT" {
		closeSide = "BUY"
		positionSide = binance.PositionSideShort
	}

	// Round SL price with directional rounding to ensure trigger protects capital
	roundedSL := roundPriceForSL(pos.Symbol, pos.StopLoss, pos.Side)
	roundedQty := roundQuantity(pos.Symbol, pos.RemainingQty)

	slParams := binance.AlgoOrderParams{
		Symbol:       pos.Symbol,
		Side:         closeSide,
		PositionSide: positionSide,
		Type:         binance.FuturesOrderTypeStopMarket,
		Quantity:     roundedQty,
		TriggerPrice: roundedSL,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	slOrder, err := ga.futuresClient.PlaceAlgoOrder(slParams)
	if err != nil {
		ga.logger.Error("Failed to place updated SL order",
			"symbol", pos.Symbol,
			"sl_price", roundedSL,
			"error", err.Error())
		return
	}

	pos.StopLossAlgoID = slOrder.AlgoId
	ga.logger.Info("Updated SL order placed",
		"symbol", pos.Symbol,
		"new_algo_id", slOrder.AlgoId,
		"trigger_price", roundedSL,
		"quantity", roundedQty)
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

	if !ga.config.DryRun && pos.RemainingQty > 0 {
		// Place close order using LIMIT to avoid slippage on SL/Trailing closes
		// This is critical for SL/Trailing stop to avoid worst-case execution
		side := "SELL"
		positionSide := binance.PositionSideLong
		if pos.Side == "SHORT" {
			side = "BUY"
			positionSide = binance.PositionSideShort
		}

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

		orderParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         side,
			PositionSide: positionSide,
			Type:         binance.FuturesOrderTypeLimit,
			Quantity:     pos.RemainingQty,
			Price:        closePrice, // LIMIT order with 0.1% buffer
		}

		_, err := ga.futuresClient.PlaceFuturesOrder(orderParams)
		if err != nil {
			ga.logger.Error("Ginie full close failed", "symbol", symbol, "error", err)
			return
		}

		ga.logger.Info("Ginie full close order placed (LIMIT - SL/Trailing)",
			"symbol", symbol,
			"reason", reason,
			"current_price", currentPrice,
			"limit_price", closePrice,
			"quantity", pos.RemainingQty)
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

// closePositionAtMarket closes a position immediately at market price
// Used when LLM recommends closing or when SL would immediately trigger
func (ga *GinieAutopilot) closePositionAtMarket(pos *GiniePosition) error {
	if pos == nil {
		return fmt.Errorf("nil position")
	}

	// Get current price
	currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(pos.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get current price: %w", err)
	}

	// Use the existing closePosition logic
	ga.closePosition(pos.Symbol, pos, currentPrice, "LLM_CLOSE_SIGNAL", 0)

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

func (ga *GinieAutopilot) getTrailingPercent(mode GinieTradingMode) float64 {
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

func (ga *GinieAutopilot) generateDefaultTPs(entryPrice float64, mode GinieTradingMode, isLong bool) []GinieTakeProfitLevel {
	var gains []float64
	switch mode {
	case GinieModeScalp:
		gains = []float64{0.3, 0.6, 1.0, 1.5}
	case GinieModeSwing:
		gains = []float64{3.0, 6.0, 10.0, 15.0}
	case GinieModePosition:
		gains = []float64{10.0, 20.0, 35.0, 50.0}
	}

	tps := make([]GinieTakeProfitLevel, 4)
	for i, gain := range gains {
		var price float64
		if isLong {
			price = entryPrice * (1 + gain/100)
		} else {
			price = entryPrice * (1 - gain/100)
		}
		tps[i] = GinieTakeProfitLevel{
			Level:   i + 1,
			Price:   roundPrice("", price),
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
		currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
		if err != nil {
			ga.logger.Error("Failed to get price during sync", "symbol", symbol, "error", err)
			currentPrice = pos.EntryPrice // Use entry as fallback
		}

		// Create a GiniePosition from exchange data
		position := &GiniePosition{
			Symbol:       symbol,
			Side:         side,
			Mode:         GinieModeSwing, // Default to swing mode for synced positions
			EntryPrice:   pos.EntryPrice,
			OriginalQty:  qty,
			RemainingQty: qty,
			Leverage:     pos.Leverage,
			EntryTime:    time.Now(), // We don't know actual entry time

			// Generate default TPs based on entry price
			TakeProfits:    ga.generateDefaultTPs(pos.EntryPrice, GinieModeSwing, side == "LONG"),
			CurrentTPLevel: 0,

			// Calculate a reasonable stop loss (2% for synced positions)
			StopLoss:         ga.calculateDefaultSL(pos.EntryPrice, side == "LONG", 2.0),
			OriginalSL:       ga.calculateDefaultSL(pos.EntryPrice, side == "LONG", 2.0),
			MovedToBreakeven: false,

			// Trailing
			TrailingActive:  false,
			HighestPrice:    currentPrice,
			LowestPrice:     currentPrice,
			TrailingPercent: ga.getTrailingPercent(GinieModeSwing),

			// PnL from exchange
			UnrealizedPnL: pos.UnrealizedProfit,
		}

		ga.positions[symbol] = position
		synced++

		ga.logger.Info("Force-synced position from exchange",
			"symbol", symbol,
			"side", side,
			"qty", qty,
			"entry_price", pos.EntryPrice)
	}

	ga.logger.Info("Force sync completed", "synced_count", synced)
	return synced, nil
}

// SyncWithExchange syncs Ginie tracked positions with actual exchange positions
// This is useful when server restarts or positions get lost
func (ga *GinieAutopilot) SyncWithExchange() (int, error) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	// Get all open positions from exchange
	positions, err := ga.futuresClient.GetPositions()
	if err != nil {
		return 0, fmt.Errorf("failed to get exchange positions: %w", err)
	}

	// Build a set of symbols with open positions on exchange
	exchangePositions := make(map[string]bool)
	for _, pos := range positions {
		if pos.PositionAmt != 0 {
			exchangePositions[pos.Symbol] = true
			ga.logger.Debug("Exchange position found", "symbol", pos.Symbol, "amt", pos.PositionAmt)
		}
	}
	ga.logger.Info("Exchange positions check",
		"exchange_count", len(exchangePositions),
		"ginie_count", len(ga.positions))

	// Remove Ginie positions that no longer exist on exchange
	removed := 0
	toRemove := make([]string, 0)
	for symbol := range ga.positions {
		if !exchangePositions[symbol] {
			ga.logger.Info("Found stale position not on exchange", "symbol", symbol)
			toRemove = append(toRemove, symbol)
		}
	}
	for _, symbol := range toRemove {
		ga.logger.Info("Removing stale position and cancelling orphan orders", "symbol", symbol)
		// CRITICAL: Cancel all algo orders for this symbol to prevent orphan orders
		success, failed, err := ga.cancelAllAlgoOrdersForSymbol(symbol)
		if err != nil || failed > 0 {
			ga.logger.Warn("Failed to cancel all algo orders on stale position removal",
				"symbol", symbol,
				"success", success,
				"failed", failed,
				"error", err)
		}
		delete(ga.positions, symbol)
		removed++
	}
	if removed > 0 {
		ga.logger.Info("Cleaned up stale positions", "removed_count", removed)
	}

	synced := 0
	for _, pos := range positions {
		// Skip positions with no quantity
		if pos.PositionAmt == 0 {
			continue
		}

		symbol := pos.Symbol

		// Skip if already tracked
		if _, exists := ga.positions[symbol]; exists {
			continue
		}

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
		currentPrice, err := ga.futuresClient.GetFuturesCurrentPrice(symbol)
		if err != nil {
			ga.logger.Error("Failed to get price during sync", "symbol", symbol, "error", err)
			continue
		}

		// Create a basic GiniePosition from exchange data
		// Note: This won't have the original decision info, but allows position monitoring
		position := &GiniePosition{
			Symbol:       symbol,
			Side:         side,
			Mode:         GinieModeSwing, // Default to swing mode for synced positions
			EntryPrice:   pos.EntryPrice,
			OriginalQty:  qty,
			RemainingQty: qty,
			Leverage:     pos.Leverage,
			EntryTime:    time.Now(), // We don't know actual entry time

			// Generate default TPs based on entry price
			TakeProfits:    ga.generateDefaultTPs(pos.EntryPrice, GinieModeSwing, side == "LONG"),
			CurrentTPLevel: 0,

			// Calculate a reasonable stop loss (2% for synced positions)
			StopLoss:         ga.calculateDefaultSL(pos.EntryPrice, side == "LONG", 2.0),
			OriginalSL:       ga.calculateDefaultSL(pos.EntryPrice, side == "LONG", 2.0),
			MovedToBreakeven: false,

			// Trailing
			TrailingActive:  false,
			HighestPrice:    currentPrice,
			LowestPrice:     currentPrice,
			TrailingPercent: ga.getTrailingPercent(GinieModeSwing),

			// PnL from exchange
			UnrealizedPnL: pos.UnrealizedProfit,
		}

		ga.positions[symbol] = position
		synced++

		ga.logger.Info("Synced position from exchange",
			"symbol", symbol,
			"side", side,
			"qty", qty,
			"entry_price", pos.EntryPrice,
			"unrealized_pnl", pos.UnrealizedProfit)
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

	// Round SL price with directional rounding to ensure trigger protects capital
	roundedSL := roundPriceForSL(pos.Symbol, pos.StopLoss, pos.Side)
	roundedQty := roundQuantity(pos.Symbol, pos.RemainingQty)

	// Place Stop Loss order using Algo Order API (STOP_MARKET requires Algo API)
	// Note: Don't set ReduceOnly - in Hedge mode, positionSide determines direction
	slParams := binance.AlgoOrderParams{
		Symbol:       pos.Symbol,
		Side:         closeSide,
		PositionSide: positionSide,
		Type:         binance.FuturesOrderTypeStopMarket,
		Quantity:     roundedQty,
		TriggerPrice: roundedSL,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	slOrder, err := ga.futuresClient.PlaceAlgoOrder(slParams)
	if err != nil {
		ga.logger.Error("Failed to place stop loss order",
			"symbol", pos.Symbol,
			"sl_price", roundedSL,
			"error", err.Error())
	} else {
		pos.StopLossAlgoID = slOrder.AlgoId
		ga.logger.Info("Stop loss order placed",
			"symbol", pos.Symbol,
			"algo_id", slOrder.AlgoId,
			"trigger_price", roundedSL)
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
					PositionSide: positionSide,
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
					PositionSide: positionSide,
					Type:         binance.FuturesOrderTypeTakeProfitMarket,
					Quantity:     tp1Qty,
					TriggerPrice: roundedTP1,
					WorkingType:  binance.WorkingTypeMarkPrice,
				}

				tpOrder, err := ga.futuresClient.PlaceAlgoOrder(tpParams)
				if err != nil {
					ga.logger.Error("Failed to place take profit order",
						"symbol", pos.Symbol,
						"tp_level", 1,
						"tp_price", tp1.Price,
						"error", err.Error())
				} else {
					pos.TakeProfitAlgoIDs = append(pos.TakeProfitAlgoIDs, tpOrder.AlgoId)
					ga.logger.Info("Take profit order placed",
						"symbol", pos.Symbol,
						"tp_level", 1,
						"algo_id", tpOrder.AlgoId,
						"trigger_price", roundedTP1,
						"quantity", tp1Qty)
				}
			}
		}
	}

	pos.LastLLMUpdate = time.Now()
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
	}

	// Remove positions that were closed externally
	for _, symbol := range positionsToRemove {
		pos := ga.positions[symbol]

		// Record as closed (with unknown close price - use 0)
		ga.recordTrade(GinieTradeResult{
			Symbol:    symbol,
			Action:    "close",
			Side:      pos.Side,
			Quantity:  pos.RemainingQty,
			Price:     0, // Unknown - closed externally
			Reason:    "Position closed externally (reconciliation)",
			Timestamp: time.Now(),
			Mode:      pos.Mode,
		})

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

		// Mode-specific limits
		var baseSLMult, baseTPMult float64
		var minSL, maxSL, minTP, maxTP float64

		switch pos.Mode {
		case GinieModeScalp:
			baseSLMult, baseTPMult = 0.5, 1.0
			minSL, maxSL = 0.2, 0.8
			minTP, maxTP = 0.3, 2.0
		case GinieModeSwing:
			baseSLMult, baseTPMult = 1.5, 3.0
			minSL, maxSL = 1.0, 5.0
			minTP, maxTP = 2.0, 15.0
		case GinieModePosition:
			baseSLMult, baseTPMult = 2.5, 5.0
			minSL, maxSL = 3.0, 15.0
			minTP, maxTP = 5.0, 50.0
		default:
			// Default to swing
			baseSLMult, baseTPMult = 1.5, 3.0
			minSL, maxSL = 1.0, 5.0
			minTP, maxTP = 2.0, 15.0
		}

		// Calculate ATR-based SL/TP
		atrSLPct := atrPct * baseSLMult
		atrTPPct := atrPct * baseTPMult

		// Blend LLM and ATR (70% LLM, 30% ATR if LLM available)
		var finalSLPct, finalTPPct float64
		if llmUsed && llmSLPct > 0 {
			finalSLPct = llmSLPct*0.7 + atrSLPct*0.3
		} else {
			finalSLPct = atrSLPct
		}
		if llmUsed && llmTPPct > 0 {
			finalTPPct = llmTPPct*0.7 + atrTPPct*0.3
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

		// Generate 4 TP levels
		pos.TakeProfits = []GinieTakeProfitLevel{
			{Level: 1, Percent: 25, GainPct: finalTPPct * 0.25, Price: pos.EntryPrice * (1 + direction*finalTPPct*0.25/100), Status: "pending"},
			{Level: 2, Percent: 25, GainPct: finalTPPct * 0.50, Price: pos.EntryPrice * (1 + direction*finalTPPct*0.50/100), Status: "pending"},
			{Level: 3, Percent: 25, GainPct: finalTPPct * 0.75, Price: pos.EntryPrice * (1 + direction*finalTPPct*0.75/100), Status: "pending"},
			{Level: 4, Percent: 25, GainPct: finalTPPct * 1.00, Price: pos.EntryPrice * (1 + direction*finalTPPct*1.00/100), Status: "pending"},
		}

		// Update trailing percent based on mode
		pos.TrailingPercent = ga.getTrailingPercent(pos.Mode)

		// Place SLTP orders on Binance in background
		slPrice := pos.StopLoss
		tpPrices := []float64{
			pos.TakeProfits[0].Price,
			pos.TakeProfits[1].Price,
			pos.TakeProfits[2].Price,
			pos.TakeProfits[3].Price,
		}
		qty := pos.RemainingQty
		posSymbol := symbol
		posSide := pos.Side

		go func() {
			if pos.StopLossAlgoID > 0 {
				_ = ga.futuresClient.CancelAlgoOrder(posSymbol, pos.StopLossAlgoID)
			}
			for _, tpID := range pos.TakeProfitAlgoIDs {
				if tpID > 0 {
					_ = ga.futuresClient.CancelAlgoOrder(posSymbol, tpID)
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

			if slOrder, err := ga.futuresClient.PlaceAlgoOrder(slParams); err == nil {
				pos.StopLossAlgoID = slOrder.AlgoId
			} else {
				ga.logger.Error("SLTP: Failed to place SL order", "symbol", posSymbol, "error", err)
			}

			tpSide := "SELL"
			if posSide == "SHORT" {
				tpSide = "BUY"
			}

			newTPIDs := []int64{}
			tpQty := qty / 4.0

			for i, tpPrice := range tpPrices {
				tpParams := binance.AlgoOrderParams{
					Symbol:       posSymbol,
					Side:         tpSide,
					Type:         "TAKE_PROFIT_MARKET",
					TriggerPrice: tpPrice,
					Quantity:     tpQty,
					ReduceOnly:   true,
				}

				if tpOrder, err := ga.futuresClient.PlaceAlgoOrder(tpParams); err == nil {
					newTPIDs = append(newTPIDs, tpOrder.AlgoId)
					ga.logger.Info("SLTP: TP order placed", "symbol", posSymbol, "level", i+1, "price", tpPrice)
				} else {
					ga.logger.Error("SLTP: Failed to place TP order", "symbol", posSymbol, "level", i+1, "error", err)
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
			"tp1", pos.TakeProfits[0].Price,
			"tp2", pos.TakeProfits[1].Price,
			"tp3", pos.TakeProfits[2].Price,
			"tp4", pos.TakeProfits[3].Price,
			"llm_used", llmUsed)
	}

	ga.logger.Info("Adaptive SL/TP recalculation completed", "updated", updated, "total_positions", len(ga.positions))
	return updated, nil
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

// GetSignalStats returns signal statistics
func (ga *GinieAutopilot) GetSignalStats() map[string]interface{} {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	stats := make(map[string]interface{})
	total := len(ga.signalLogs)
	executed := 0
	rejected := 0
	pending := 0
	rejectionReasons := make(map[string]int)

	for _, sig := range ga.signalLogs {
		switch sig.Status {
		case "executed":
			executed++
		case "rejected":
			rejected++
			if sig.RejectionReason != "" {
				rejectionReasons[sig.RejectionReason]++
			}
		case "pending":
			pending++
		}
	}

	stats["total"] = total
	stats["executed"] = executed
	stats["rejected"] = rejected
	stats["pending"] = pending
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

			orderParams := binance.FuturesOrderParams{
				Symbol:       symbol,
				Side:         side,
				PositionSide: positionSide,
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

	return ScanDiagnostics{
		LastScanTime:         lastScan,
		SecondsSinceLastScan: int64(time.Since(lastScan).Seconds()),
		SymbolsInWatchlist:   symbolsCount,
		SymbolsScannedLast:   ga.symbolsScannedLastCycle,
		ScalpEnabled:         ga.config.EnableScalpMode,
		SwingEnabled:         ga.config.EnableSwingMode,
		PositionEnabled:      ga.config.EnablePositionMode,
	}
}

// getSignalDiagnosticsLocked returns signal generation stats (must hold lock)
func (ga *GinieAutopilot) getSignalDiagnosticsLocked() SignalDiagnostics {
	diag := SignalDiagnostics{
		TopRejectionReasons: make(map[string]int),
	}

	// Only look at signals from the last hour
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	for _, sig := range ga.signalLogs {
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

	if diag.TotalGenerated > 0 {
		diag.ExecutionRate = float64(diag.Executed) / float64(diag.TotalGenerated) * 100
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
	if diag.AutopilotRunning && diag.Scanning.SecondsSinceLastScan > 300 {
		issues = append(issues, DiagnosticIssue{
			Severity:   "warning",
			Category:   "scanning",
			Message:    fmt.Sprintf("No scan activity for %d seconds", diag.Scanning.SecondsSinceLastScan),
			Suggestion: "Check if autopilot loop is running correctly",
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

	// Check funding rate before entry
	isLong := signal.Side == "LONG"
	if blocked, reason := ga.checkFundingRate(symbol, isLong); blocked {
		ga.logger.Warn("Strategy trade skipped - funding rate concern",
			"symbol", symbol,
			"strategy", signal.StrategyName,
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
			PositionSide: positionSide,
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
	takeProfits := ga.generateDefaultTPs(actualPrice, GinieModeSwing, isLong)

	// Create strategy ID and name pointers
	stratID := signal.StrategyID
	stratName := signal.StrategyName

	// Create position record
	position := &GiniePosition{
		Symbol:           symbol,
		Side:             signal.Side,
		Mode:             GinieModeSwing, // Default mode for strategy trades
		EntryPrice:       actualPrice,
		OriginalQty:      actualQty,
		RemainingQty:     actualQty,
		Leverage:         leverage,
		EntryTime:        time.Now(),
		TakeProfits:      takeProfits,
		CurrentTPLevel:   0,
		StopLoss:         signal.StopLoss,
		OriginalSL:       signal.StopLoss,
		MovedToBreakeven: false,
		TrailingActive:   false,
		HighestPrice:     actualPrice,
		LowestPrice:      actualPrice,
		TrailingPercent:  ga.getTrailingPercent(GinieModeSwing),
		DecisionReport:   nil, // No AI decision report for strategy trades
		Source:           "strategy",
		StrategyID:       &stratID,
		StrategyName:     &stratName,
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

	// Record trade with strategy info
	ga.recordTrade(GinieTradeResult{
		Symbol:    symbol,
		Action:    "open",
		Side:      signal.Side,
		Quantity:  actualQty,
		Price:     actualPrice,
		Reason:    fmt.Sprintf("Strategy: %s - %s", signal.StrategyName, signal.Reason),
		Timestamp: time.Now(),
		Mode:      GinieModeSwing,
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

// monitorUltraFastPositions runs a 500ms polling loop to monitor ultra-fast positions
// for profit targets and time-based exits
func (ga *GinieAutopilot) monitorUltraFastPositions() {
	defer ga.wg.Done()

	ga.logger.Info("Ultra-fast position monitor started - 500ms polling interval")

	// 500ms ticker for rapid exit checking
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ga.stopChan:
			ga.logger.Info("Ultra-fast position monitor stopping")
			return
		case <-ticker.C:
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
			PositionSide: positionSide,
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
			PositionSide: positionSide,
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
