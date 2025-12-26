package autopilot

import "time"

// GinieTradingMode represents the trading mode selected by Ginie
type GinieTradingMode string

const (
	GinieModeScalp     GinieTradingMode = "scalp"
	GinieModeSwing     GinieTradingMode = "swing"
	GinieModePosition  GinieTradingMode = "position"
	GinieModeUltraFast GinieTradingMode = "ultra_fast"
)

// GinieScanStatus represents the coin scan classification
type GinieScanStatus string

const (
	ScanStatusScalpReady    GinieScanStatus = "SCALP-READY"
	ScanStatusSwingReady    GinieScanStatus = "SWING-READY"
	ScanStatusPositionReady GinieScanStatus = "POSITION-READY"
	ScanStatusHedgeRequired GinieScanStatus = "HEDGE-REQUIRED"
	ScanStatusAvoid         GinieScanStatus = "AVOID"
)

// GenieRecommendation represents overall recommendation
type GenieRecommendation string

const (
	RecommendationExecute GenieRecommendation = "EXECUTE"
	RecommendationWait    GenieRecommendation = "WAIT"
	RecommendationSkip    GenieRecommendation = "SKIP"
)

// GinieCoinScan represents the pre-trade coin scan results
type GinieCoinScan struct {
	Symbol    string          `json:"symbol"`
	Timestamp time.Time       `json:"timestamp"`
	Status    GinieScanStatus `json:"status"`

	// Liquidity Check
	Liquidity LiquidityCheck `json:"liquidity"`

	// Volatility Profile
	Volatility VolatilityProfile `json:"volatility"`

	// Trend Health
	Trend TrendHealth `json:"trend"`

	// Market Structure
	Structure MarketStructure `json:"structure"`

	// Correlation Check
	Correlation CorrelationCheck `json:"correlation"`

	// Overall Score
	Score       float64 `json:"score"`
	TradeReady  bool    `json:"trade_ready"`
	Reason      string  `json:"reason"`
}

// LiquidityCheck contains liquidity assessment
type LiquidityCheck struct {
	Volume24h       float64 `json:"volume_24h"`
	VolumeUSD       float64 `json:"volume_usd"`
	BidAskSpread    float64 `json:"bid_ask_spread"`
	SpreadPercent   float64 `json:"spread_percent"`
	SlippageRisk    string  `json:"slippage_risk"` // low, medium, high
	OrderBookDepth  float64 `json:"order_book_depth"`
	LiquidityScore  float64 `json:"liquidity_score"` // 0-100
	PassedScalp     bool    `json:"passed_scalp"`    // Volume > $5M
	PassedSwing     bool    `json:"passed_swing"`    // Volume > $1M
}

// VolatilityProfile contains volatility assessment
type VolatilityProfile struct {
	ATR14           float64 `json:"atr_14"`
	ATRPercent      float64 `json:"atr_percent"`
	AvgATR20        float64 `json:"avg_atr_20"`
	ATRRatio        float64 `json:"atr_ratio"` // Current / Avg
	BBWidth         float64 `json:"bb_width"`
	BBWidthPercent  float64 `json:"bb_width_percent"`
	Volatility7d    float64 `json:"volatility_7d"`
	Volatility30d   float64 `json:"volatility_30d"`
	Regime          string  `json:"regime"` // Low, Medium, High, Extreme
	VolatilityScore float64 `json:"volatility_score"`
}

// TrendHealth contains trend assessment
type TrendHealth struct {
	Timeframe       string   `json:"timeframe"`        // 1h, 4h, 1d - the timeframe used for analysis
	ADXValue        float64  `json:"adx_value"`
	ADXStrength     string   `json:"adx_strength"` // weak, moderate, strong, very_strong
	IsTrending      bool     `json:"is_trending"`  // ADX > 25
	IsRanging       bool     `json:"is_ranging"`   // ADX < 20
	TrendDirection  string   `json:"trend_direction"` // bullish, bearish, neutral
	EMA20Distance   float64  `json:"ema_20_distance"`
	EMA50Distance   float64  `json:"ema_50_distance"`
	EMA200Distance  float64  `json:"ema_200_distance"`
	MTFAlignment    bool     `json:"mtf_alignment"`    // Multi-timeframe alignment
	AlignedTFs      []string `json:"aligned_tfs"`
	TrendAge        int      `json:"trend_age"`        // Candles since trend started
	TrendMaturity   string   `json:"trend_maturity"`   // early, mature, late
	TrendScore      float64  `json:"trend_score"`
}

// MarketStructure contains structure assessment
type MarketStructure struct {
	Pattern           string    `json:"pattern"` // HH/HL, LH/LL, ranging
	KeyResistances    []float64 `json:"key_resistances"`
	KeySupports       []float64 `json:"key_supports"`
	NearestResistance float64   `json:"nearest_resistance"`
	NearestSupport    float64   `json:"nearest_support"`
	BreakoutPotential float64   `json:"breakout_potential"`
	BreakdownPotential float64  `json:"breakdown_potential"`
	ConsolidationDays int       `json:"consolidation_days"`
	StructureScore    float64   `json:"structure_score"`
}

// CorrelationCheck contains correlation assessment
type CorrelationCheck struct {
	BTCCorrelation     float64 `json:"btc_correlation"`
	ETHCorrelation     float64 `json:"eth_correlation"`
	SectorCorrelation  float64 `json:"sector_correlation"`
	IndependentCapable bool    `json:"independent_capable"` // Can move independently
	CorrelationScore   float64 `json:"correlation_score"`
}

// GinieSignalSet contains signals for a specific mode
type GinieSignalSet struct {
	Mode                GinieTradingMode `json:"mode"`
	PrimaryTimeframe    string           `json:"primary_timeframe"`
	ConfirmTimeframe    string           `json:"confirm_timeframe"`

	// Primary Signals
	PrimarySignals      []GinieSignal    `json:"primary_signals"`
	PrimaryMet          int              `json:"primary_met"`
	PrimaryRequired     int              `json:"primary_required"`
	PrimaryPassed       bool             `json:"primary_passed"`

	// Secondary Confirmations
	SecondarySignals    []GinieSignal    `json:"secondary_signals"`
	SecondaryMet        int              `json:"secondary_met"`

	// Overall
	SignalStrength      string           `json:"signal_strength"` // Weak, Moderate, Strong, Very Strong
	StrengthScore       float64          `json:"strength_score"`
	Direction           string           `json:"direction"`       // long, short, neutral
}

// GinieSignal represents an individual signal
type GinieSignal struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Status      string  `json:"status"` // met, not_met, partial
	Value       float64 `json:"value"`
	Threshold   float64 `json:"threshold"`
	Weight      float64 `json:"weight"`
	Met         bool    `json:"met"`
}

// GinieTakeProfitLevel represents a TP level
type GinieTakeProfitLevel struct {
	Level      int     `json:"level"`
	Price      float64 `json:"price"`
	Percent    float64 `json:"percent"`    // Portion of position
	GainPct    float64 `json:"gain_pct"`   // % gain from entry
	Status     string  `json:"status"`     // pending, hit
}

// GinieHedgeRecommendation contains hedging advice
type GinieHedgeRecommendation struct {
	Required    bool    `json:"required"`
	HedgeType   string  `json:"hedge_type"`   // direct, correlation, options, stablecoin
	HedgeSize   float64 `json:"hedge_size"`   // % of position
	EntryRule   string  `json:"entry_rule"`
	ExitRule    string  `json:"exit_rule"`
	Reason      string  `json:"reason"`
}

// GinieTradeExecution contains trade parameters
type GinieTradeExecution struct {
	Action       string                 `json:"action"` // LONG, SHORT, WAIT, CLOSE
	EntryLow     float64                `json:"entry_low"`
	EntryHigh    float64                `json:"entry_high"`
	PositionPct  float64                `json:"position_pct"`  // % of capital
	RiskUSD      float64                `json:"risk_usd"`
	Leverage     int                    `json:"leverage"`
	TakeProfits  []GinieTakeProfitLevel `json:"take_profits"`
	StopLoss     float64                `json:"stop_loss"`
	StopLossPct  float64                `json:"stop_loss_pct"`
	RiskReward   float64                `json:"risk_reward"`
	TrailingStop float64                `json:"trailing_stop"`
}

// GinieDecisionReport is the full structured decision output
type GinieDecisionReport struct {
	// Header
	Symbol      string           `json:"symbol"`
	Timestamp   time.Time        `json:"timestamp"`
	ScanStatus  GinieScanStatus  `json:"scan_status"`
	SelectedMode GinieTradingMode `json:"selected_mode"`

	// Market Conditions
	MarketConditions struct {
		Trend        string  `json:"trend"`
		ADX          float64 `json:"adx"`
		Volatility   string  `json:"volatility"`
		ATR          float64 `json:"atr"`
		Volume       string  `json:"volume"`
		BTCCorr      float64 `json:"btc_correlation"`
		Sentiment    string  `json:"sentiment"`
		SentimentVal float64 `json:"sentiment_value"`
	} `json:"market_conditions"`

	// Signal Analysis
	SignalAnalysis GinieSignalSet `json:"signal_analysis"`

	// Trade Execution
	TradeExecution GinieTradeExecution `json:"trade_execution"`

	// Hedge Recommendation
	Hedge GinieHedgeRecommendation `json:"hedge"`

	// Invalidation & Alerts
	InvalidationConditions []string `json:"invalidation_conditions"`
	ReEvaluateConditions   []string `json:"re_evaluate_conditions"`
	NextReview             string   `json:"next_review"`

	// Divergence Detection
	TrendDivergence *TrendDivergence `json:"trend_divergence,omitempty"`

	// LLM Decision Context (for analysis tracking)
	DecisionContext *DecisionContext `json:"decision_context,omitempty"`

	// Final Scores
	ConfidenceScore    float64             `json:"confidence_score"`
	Recommendation     GenieRecommendation `json:"recommendation"`
	RecommendationNote string              `json:"recommendation_note"`
}

// TrendConfirmation contains LLM trend analysis
type TrendConfirmation struct {
	Trend      string  `json:"trend"`      // BULLISH, BEARISH, NEUTRAL
	Strength   float64 `json:"strength"`   // 0-100
	Confidence float64 `json:"confidence"` // 0-100
	Reasoning  string  `json:"reasoning"`
}

// TrendDivergence represents a detected divergence between scan and decision timeframes
type TrendDivergence struct {
	Detected           bool    `json:"detected"`
	ScanTimeframe      string  `json:"scan_timeframe"`
	ScanTrend          string  `json:"scan_trend"`          // bullish/bearish/neutral
	DecisionTimeframe  string  `json:"decision_timeframe"`
	DecisionTrend      string  `json:"decision_trend"`
	Severity           string  `json:"severity"`            // minor/moderate/severe
	ShouldBlock        bool    `json:"should_block"`
	Reason             string  `json:"reason"`
}

// GinieConfig contains Ginie configuration
type GinieConfig struct {
	Enabled             bool    `json:"enabled"`

	// Mode Selection Thresholds
	ScalpADXMax         float64 `json:"scalp_adx_max"`         // Prefer ranging (ADX < 20)
	SwingADXMin         float64 `json:"swing_adx_min"`         // 25-45
	SwingADXMax         float64 `json:"swing_adx_max"`
	PositionADXMin      float64 `json:"position_adx_min"`      // > 35

	// Volatility Thresholds
	HighVolatilityRatio float64 `json:"high_volatility_ratio"` // ATR ratio > 1.5

	// Liquidity Thresholds
	MinScalpVolume      float64 `json:"min_scalp_volume"`      // $5M
	MinSwingVolume      float64 `json:"min_swing_volume"`      // $1M
	MaxBidAskSpread     float64 `json:"max_bid_ask_spread"`    // 0.1%

	// Signal Requirements
	ScalpSignalsRequired    int `json:"scalp_signals_required"`    // 3/4
	SwingSignalsRequired    int `json:"swing_signals_required"`    // 4/5
	PositionSignalsRequired int `json:"position_signals_required"` // 4/5

	// Risk Parameters
	MaxDailyDrawdown    float64 `json:"max_daily_drawdown"`    // 3%
	MaxWeeklyDrawdown   float64 `json:"max_weekly_drawdown"`   // 7%
	MaxMonthlyDrawdown  float64 `json:"max_monthly_drawdown"`  // 15%

	// Mode Limits
	MaxScalpPositions   int `json:"max_scalp_positions"`   // 3-5
	MaxSwingPositions   int `json:"max_swing_positions"`   // 2-4
	MaxPositionPositions int `json:"max_position_positions"` // 1-3

	// Auto Mode Override
	AutoOverrideEnabled bool `json:"auto_override_enabled"`

	// Monitoring Intervals (seconds)
	ScalpMonitorInterval    int `json:"scalp_monitor_interval"`    // 900 (15 min)
	SwingMonitorInterval    int `json:"swing_monitor_interval"`    // 14400 (4 hours)
	PositionMonitorInterval int `json:"position_monitor_interval"` // 86400 (1 day)
}

// DefaultGinieConfig returns default configuration
func DefaultGinieConfig() *GinieConfig {
	return &GinieConfig{
		Enabled:             true,

		// Mode Selection
		ScalpADXMax:         20,
		SwingADXMin:         25,
		SwingADXMax:         45,
		PositionADXMin:      35,

		// Volatility
		HighVolatilityRatio: 1.5,

		// Liquidity
		MinScalpVolume:      5000000,
		MinSwingVolume:      1000000,
		MaxBidAskSpread:     0.1,

		// Signals
		ScalpSignalsRequired:    3,
		SwingSignalsRequired:    4,
		PositionSignalsRequired: 4,

		// Risk
		MaxDailyDrawdown:   3.0,
		MaxWeeklyDrawdown:  7.0,
		MaxMonthlyDrawdown: 15.0,

		// Positions
		MaxScalpPositions:    5,
		MaxSwingPositions:    4,
		MaxPositionPositions: 3,

		// Override
		AutoOverrideEnabled: true,

		// Monitoring
		ScalpMonitorInterval:    900,
		SwingMonitorInterval:    14400,
		PositionMonitorInterval: 86400,
	}
}

// GinieStatus represents current Ginie status
type GinieStatus struct {
	Enabled           bool              `json:"enabled"`
	ActiveMode        GinieTradingMode  `json:"active_mode"`
	ActivePositions   int               `json:"active_positions"`
	MaxPositions      int               `json:"max_positions"`
	LastScanTime      time.Time         `json:"last_scan_time"`
	LastDecisionTime  time.Time         `json:"last_decision_time"`
	DailyPnL          float64           `json:"daily_pnl"`
	DailyTrades       int               `json:"daily_trades"`
	WinRate           float64           `json:"win_rate"`
	Config            *GinieConfig      `json:"config"`
	RecentDecisions   []GinieDecisionReport `json:"recent_decisions,omitempty"`
	WatchedSymbols    []string          `json:"watched_symbols"`
	ScannedSymbols    int               `json:"scanned_symbols"`
}

// GinieModeConfig defines the complete configuration for a specific trading mode.
// Each mode (scalp, swing, position, ultra_fast) has its own configuration that
// controls timeframes, thresholds, position sizing, risk management, and circuit breakers.
type GinieModeConfig struct {
	// Mode Identity
	Mode    GinieTradingMode `json:"mode"`    // The trading mode this config applies to
	Enabled bool             `json:"enabled"` // Whether this mode is enabled

	// Timeframe Configuration
	TrendTimeframe    string `json:"trend_timeframe"`    // Timeframe for trend analysis (e.g., "4h", "1d")
	EntryTimeframe    string `json:"entry_timeframe"`    // Timeframe for entry signals (e.g., "15m", "1h")
	AnalysisTimeframe string `json:"analysis_timeframe"` // Timeframe for detailed analysis

	// Confidence Thresholds
	MinConfidence   float64 `json:"min_confidence"`   // Minimum confidence to consider a trade (0-100)
	HighConfidence  float64 `json:"high_confidence"`  // Threshold for high confidence trades
	UltraConfidence float64 `json:"ultra_confidence"` // Threshold for maximum confidence trades

	// Position Sizing
	BaseSizeUSD    float64 `json:"base_size_usd"`    // Base position size in USD
	MaxSizeUSD     float64 `json:"max_size_usd"`     // Maximum position size in USD
	MaxPositions   int     `json:"max_positions"`    // Maximum concurrent positions for this mode
	Leverage       int     `json:"leverage"`         // Leverage to use for positions
	SizeMultiplier float64 `json:"size_multiplier"`  // Multiplier applied based on confidence

	// SL/TP Configuration
	StopLossPercent   float64 `json:"stop_loss_percent"`   // Stop loss percentage from entry
	TakeProfitPercent float64 `json:"take_profit_percent"` // Take profit percentage from entry
	TrailingEnabled   bool    `json:"trailing_enabled"`    // Whether trailing stop is enabled
	TrailingPercent   float64 `json:"trailing_percent"`    // Trailing stop percentage
	TrailingActivation float64 `json:"trailing_activation"` // Profit % to activate trailing (e.g., 1.0 = activate after 1% profit)
	TrailingActivationPrice float64 `json:"trailing_activation_price"` // Specific price to activate trailing (0 = use profit %)
	MaxHoldDuration   string  `json:"max_hold_duration"`   // Maximum time to hold position (e.g., "4h", "1d")

	// ROI-based SL/TP (alternative to price-based)
	UseROIBasedSLTP      bool    `json:"use_roi_based_sltp"`       // Use ROI % instead of price % for SL/TP
	ROIStopLossPercent   float64 `json:"roi_stop_loss_percent"`    // Close at this ROI % loss (e.g., -10 = -10% ROI)
	ROITakeProfitPercent float64 `json:"roi_take_profit_percent"`  // Close at this ROI % profit (e.g., 25 = +25% ROI)

	// Margin Configuration
	MarginType            string  `json:"margin_type"`              // "CROSS" or "ISOLATED" (default: "CROSS")
	IsolatedMarginPercent float64 `json:"isolated_margin_percent"`  // Margin % for isolated mode (10-100%)

	// Circuit Breaker - nested struct for mode-specific circuit breaker configuration
	CircuitBreaker ModeCircuitBreaker `json:"circuit_breaker"`
}

// ModeCircuitBreaker provides comprehensive risk management and rate limiting
// for a specific trading mode. It tracks losses, trade counts, win rates,
// and can automatically pause trading when limits are exceeded.
type ModeCircuitBreaker struct {
	// Loss Limits - maximum allowed losses before pausing
	MaxLossPerHour      float64 `json:"max_loss_per_hour"`      // Maximum USD loss allowed per hour
	MaxLossPerDay       float64 `json:"max_loss_per_day"`       // Maximum USD loss allowed per day
	MaxConsecutiveLoss  int     `json:"max_consecutive_loss"`   // Maximum consecutive losing trades before pause

	// Rate Limits - maximum trades allowed in time windows
	MaxTradesPerMinute int `json:"max_trades_per_minute"` // Maximum trades per minute
	MaxTradesPerHour   int `json:"max_trades_per_hour"`   // Maximum trades per hour
	MaxTradesPerDay    int `json:"max_trades_per_day"`    // Maximum trades per day

	// Win Rate Monitoring - minimum win rate requirements
	WinRateCheckAfter int     `json:"win_rate_check_after"` // Number of trades before checking win rate
	MinWinRatePercent float64 `json:"min_win_rate_percent"` // Minimum acceptable win rate percentage

	// Cooldown & Recovery - pause and resume configuration
	CooldownMinutes int  `json:"cooldown_minutes"` // Minutes to pause after circuit breaker trips
	AutoRecovery    bool `json:"auto_recovery"`    // Whether to automatically resume after cooldown

	// Current State (tracked at runtime) - these fields track the current circuit breaker state
	CurrentHourLoss    float64   `json:"current_hour_loss"`    // Current hour's cumulative loss
	CurrentDayLoss     float64   `json:"current_day_loss"`     // Current day's cumulative loss
	ConsecutiveLosses  int       `json:"consecutive_losses"`   // Current consecutive loss count
	TradesThisMinute   int       `json:"trades_this_minute"`   // Trades executed this minute
	TradesThisHour     int       `json:"trades_this_hour"`     // Trades executed this hour
	TradesThisDay      int       `json:"trades_this_day"`      // Trades executed today
	TotalWins          int       `json:"total_wins"`           // Total winning trades (for win rate)
	TotalTrades        int       `json:"total_trades"`         // Total trades executed (for win rate)
	IsPaused           bool      `json:"is_paused"`            // Whether circuit breaker has tripped
	PausedUntil        time.Time `json:"paused_until"`         // When the pause will end
	PauseReason        string    `json:"pause_reason"`         // Reason for the current pause
}

// ===== LLM INTEGRATION TYPES =====

// LLMAnalysisRequest contains data sent to LLM for trading analysis
type LLMAnalysisRequest struct {
	Symbol           string                 `json:"symbol"`
	CurrentPrice     float64                `json:"current_price"`
	PriceChange1h    float64                `json:"price_change_1h"`
	PriceChange24h   float64                `json:"price_change_24h"`
	Volume24h        float64                `json:"volume_24h"`
	VolumeChange     float64                `json:"volume_change"`
	TechnicalSignals map[string]interface{} `json:"technical_signals"`
	MarketContext    map[string]interface{} `json:"market_context"`
	RecentNews       []string               `json:"recent_news"`
}

// LLMAnalysisResponse contains the parsed response from LLM trading analysis
type LLMAnalysisResponse struct {
	Recommendation     string   `json:"recommendation"`       // LONG, SHORT, HOLD
	Confidence         int      `json:"confidence"`           // 0-100
	Reasoning          string   `json:"reasoning"`
	KeyFactors         []string `json:"key_factors"`
	RiskLevel          string   `json:"risk_level"`           // low, moderate, high
	SuggestedSLPercent float64  `json:"suggested_sl_percent"`
	SuggestedTPPercent float64  `json:"suggested_tp_percent"`
	TimeHorizon        string   `json:"time_horizon"`
}

// DecisionContext stores the context and reasoning behind a trading decision
// This is used for tracking, analysis, and audit purposes
type DecisionContext struct {
	TechnicalConfidence int      `json:"technical_confidence"`
	LLMConfidence       int      `json:"llm_confidence"`
	FinalConfidence     int      `json:"final_confidence"`
	TechnicalDirection  string   `json:"technical_direction"`
	LLMDirection        string   `json:"llm_direction"`
	Agreement           bool     `json:"agreement"`
	LLMReasoning        string   `json:"llm_reasoning"`
	LLMKeyFactors       []string `json:"llm_key_factors"`
	LLMProvider         string   `json:"llm_provider"`
	LLMModel            string   `json:"llm_model"`
	LLMLatencyMs        int64    `json:"llm_latency_ms"`
	UsedCache           bool     `json:"used_cache"`
	SkippedLLM          bool     `json:"skipped_llm"`
	SkipReason          string   `json:"skip_reason,omitempty"`
}
