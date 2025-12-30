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
	ScanStatusScalpReady     GinieScanStatus = "SCALP-READY"
	ScanStatusSwingReady     GinieScanStatus = "SWING-READY"
	ScanStatusPositionReady  GinieScanStatus = "POSITION-READY"
	ScanStatusUltraFastReady GinieScanStatus = "ULTRAFAST-READY"
	ScanStatusHedgeRequired  GinieScanStatus = "HEDGE-REQUIRED"
	ScanStatusAvoid          GinieScanStatus = "AVOID"
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

	// Price Action Analysis (FVG + Order Blocks)
	PriceAction PriceActionAnalysis `json:"price_action"`

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
	// 24h volatility for mode selection
	PriceChange24h  float64 `json:"price_change_24h"`  // 24h price change percent (absolute)
	HighLowRange24h float64 `json:"high_low_range_24h"` // (High - Low) / Low * 100
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

// FairValueGap represents a price imbalance (3-candle pattern where wicks don't overlap)
type FairValueGap struct {
	Type       string    `json:"type"`        // "bullish" or "bearish"
	TopPrice   float64   `json:"top_price"`   // Upper boundary of the gap
	BottomPrice float64  `json:"bottom_price"` // Lower boundary of the gap
	MidPrice   float64   `json:"mid_price"`   // Middle of the gap (50% level)
	GapSize    float64   `json:"gap_size"`    // Size in price units
	GapPercent float64   `json:"gap_percent"` // Size as percentage of price
	CandleIndex int      `json:"candle_index"` // Index of the middle candle (imbalance candle)
	Timestamp  time.Time `json:"timestamp"`   // When the FVG was created
	Filled     bool      `json:"filled"`      // Whether price has returned to fill it
	Tested     bool      `json:"tested"`      // Whether price has touched the zone
	Strength   string    `json:"strength"`    // "strong", "moderate", "weak"
}

// FVGAnalysis contains all detected Fair Value Gaps
type FVGAnalysis struct {
	BullishFVGs     []FairValueGap `json:"bullish_fvgs"`     // Gaps expecting price to come down to fill
	BearishFVGs     []FairValueGap `json:"bearish_fvgs"`     // Gaps expecting price to come up to fill
	NearestBullish  *FairValueGap  `json:"nearest_bullish"`  // Closest unfilled bullish FVG below price
	NearestBearish  *FairValueGap  `json:"nearest_bearish"`  // Closest unfilled bearish FVG above price
	TotalUnfilled   int            `json:"total_unfilled"`   // Count of unfilled FVGs
	InFVGZone       bool           `json:"in_fvg_zone"`      // Is current price inside an FVG?
	FVGZoneType     string         `json:"fvg_zone_type"`    // "bullish", "bearish", or ""
	FVGConfluence   bool           `json:"fvg_confluence"`   // FVG aligns with trade direction
}

// OrderBlock represents an institutional order flow zone
type OrderBlock struct {
	Type        string    `json:"type"`         // "bullish" (demand) or "bearish" (supply)
	HighPrice   float64   `json:"high_price"`   // Upper boundary of the OB
	LowPrice    float64   `json:"low_price"`    // Lower boundary of the OB
	MidPrice    float64   `json:"mid_price"`    // Middle of the OB (50% level)
	OpenPrice   float64   `json:"open_price"`   // Open of the OB candle
	ClosePrice  float64   `json:"close_price"`  // Close of the OB candle
	Volume      float64   `json:"volume"`       // Volume of the OB candle
	CandleIndex int       `json:"candle_index"` // Index of the OB candle
	Timestamp   time.Time `json:"timestamp"`    // When the OB was created
	Mitigated   bool      `json:"mitigated"`    // Whether price has returned and mitigated it
	Tested      bool      `json:"tested"`       // Whether price has touched the zone
	TestCount   int       `json:"test_count"`   // Number of times tested
	Strength    string    `json:"strength"`     // "strong", "moderate", "weak" based on move after
	MovePercent float64   `json:"move_percent"` // % move that followed this OB
}

// OrderBlockAnalysis contains all detected Order Blocks
type OrderBlockAnalysis struct {
	BullishOBs      []OrderBlock `json:"bullish_obs"`      // Demand zones (last bearish before up move)
	BearishOBs      []OrderBlock `json:"bearish_obs"`      // Supply zones (last bullish before down move)
	NearestBullish  *OrderBlock  `json:"nearest_bullish"`  // Closest unmitigated bullish OB below price
	NearestBearish  *OrderBlock  `json:"nearest_bearish"`  // Closest unmitigated bearish OB above price
	TotalUnmitigated int         `json:"total_unmitigated"` // Count of unmitigated OBs
	InOBZone        bool         `json:"in_ob_zone"`       // Is current price inside an OB?
	OBZoneType      string       `json:"ob_zone_type"`     // "bullish", "bearish", or ""
	OBConfluence    bool         `json:"ob_confluence"`    // OB aligns with trade direction
}

// PriceActionAnalysis combines FVG, Order Block, and Chart Pattern analysis
type PriceActionAnalysis struct {
	FVG              FVGAnalysis          `json:"fvg"`
	OrderBlocks      OrderBlockAnalysis   `json:"order_blocks"`
	ChartPatterns    ChartPatternAnalysis `json:"chart_patterns"`     // Chart pattern detection
	HasBullishSetup  bool                 `json:"has_bullish_setup"`  // Price at demand zone with bullish FVG
	HasBearishSetup  bool                 `json:"has_bearish_setup"`  // Price at supply zone with bearish FVG
	SetupQuality     string               `json:"setup_quality"`      // "premium", "good", "average", "poor"
	ConfluenceScore  float64              `json:"confluence_score"`   // 0-100 based on FVG+OB alignment
}

// ============ CHART PATTERN TYPES ============

// SwingPoint represents a significant price swing (high or low) with full context
type SwingPoint struct {
	Price     float64   `json:"price"`
	Index     int       `json:"index"`
	Timestamp time.Time `json:"timestamp"`
	Volume    float64   `json:"volume,omitempty"`
}

// Trendline represents a line connecting swing points
type Trendline struct {
	StartPrice  float64 `json:"start_price"`
	EndPrice    float64 `json:"end_price"`
	StartIndex  int     `json:"start_index"`
	EndIndex    int     `json:"end_index"`
	Slope       float64 `json:"slope"`        // Price change per bar
	TouchPoints []int   `json:"touch_points"` // Indices where price touches line
}

// HeadAndShouldersPattern represents a head and shoulders or inverse H&S pattern
type HeadAndShouldersPattern struct {
	Type          string     `json:"type"`           // "head_and_shoulders" or "inverse_head_and_shoulders"
	LeftShoulder  SwingPoint `json:"left_shoulder"`
	Head          SwingPoint `json:"head"`
	RightShoulder SwingPoint `json:"right_shoulder"`
	NecklineLeft  SwingPoint `json:"neckline_left"`
	NecklineRight SwingPoint `json:"neckline_right"`
	NecklineSlope float64    `json:"neckline_slope"`  // Slope of neckline
	NecklinePrice float64    `json:"neckline_price"`  // Current neckline price at right shoulder
	TargetPrice   float64    `json:"target_price"`    // Projected price target
	PatternHeight float64    `json:"pattern_height"`  // Head to neckline distance
	PatternPercent float64   `json:"pattern_percent"` // Height as % of price
	SymmetryScore float64    `json:"symmetry_score"`  // 0-100 based on shoulder symmetry
	VolumeConfirmed bool     `json:"volume_confirmed"` // Volume pattern matches
	Completed     bool       `json:"completed"`       // Neckline broken
	CandleIndex   int        `json:"candle_index"`    // Index of right shoulder
	Timestamp     time.Time  `json:"timestamp"`
	Strength      string     `json:"strength"`        // "strong", "moderate", "weak"
}

// DoubleTopBottomPattern represents a double top or double bottom pattern
type DoubleTopBottomPattern struct {
	Type            string     `json:"type"`              // "double_top" or "double_bottom"
	FirstPeak       SwingPoint `json:"first_peak"`
	SecondPeak      SwingPoint `json:"second_peak"`
	Neckline        SwingPoint `json:"neckline"`          // Trough between peaks (or peak between bottoms)
	NecklinePrice   float64    `json:"neckline_price"`
	TargetPrice     float64    `json:"target_price"`       // Projected target
	PatternHeight   float64    `json:"pattern_height"`     // Distance from peaks to neckline
	PatternPercent  float64    `json:"pattern_percent"`
	PeakDifference  float64    `json:"peak_difference"`    // % difference between peaks
	BarsBetween     int        `json:"bars_between"`       // Bars between the two peaks
	VolumeConfirmed bool       `json:"volume_confirmed"`   // V1 > V2 confirmation
	Status          string     `json:"status"`             // "forming", "confirmed", "invalid"
	Completed       bool       `json:"completed"`          // Neckline broken
	CandleIndex     int        `json:"candle_index"`
	Timestamp       time.Time  `json:"timestamp"`
	Strength        string     `json:"strength"`
}

// TrianglePattern represents an ascending, descending, or symmetrical triangle
type TrianglePattern struct {
	Type            string    `json:"type"`              // "ascending", "descending", "symmetrical"
	UpperTrendline  Trendline `json:"upper_trendline"`
	LowerTrendline  Trendline `json:"lower_trendline"`
	ApexPrice       float64   `json:"apex_price"`        // Convergence point price
	ApexIndex       int       `json:"apex_index"`        // Estimated convergence bar
	PatternStart    int       `json:"pattern_start"`     // Start index
	PatternWidth    int       `json:"pattern_width"`     // Number of bars
	BaseHeight      float64   `json:"base_height"`       // Height at pattern start
	BasePercent     float64   `json:"base_percent"`      // As % of price
	CurrentHeight   float64   `json:"current_height"`    // Current contracted height
	Contraction     float64   `json:"contraction_pct"`   // How much pattern has contracted
	VolumeDecline   bool      `json:"volume_decline"`    // Volume contracting
	BreakoutBias    string    `json:"breakout_bias"`     // "up", "down", "neutral"
	BreakoutTarget  float64   `json:"breakout_target"`
	TouchesUpper    int       `json:"touches_upper"`
	TouchesLower    int       `json:"touches_lower"`
	Completed       bool      `json:"completed"`         // Breakout occurred
	BreakoutDir     string    `json:"breakout_dir"`      // "up" or "down" after break
	CandleIndex     int       `json:"candle_index"`
	Timestamp       time.Time `json:"timestamp"`
	Strength        string    `json:"strength"`
}

// WedgePattern represents a rising or falling wedge
type WedgePattern struct {
	Type            string    `json:"type"`              // "rising_wedge" or "falling_wedge"
	UpperTrendline  Trendline `json:"upper_trendline"`
	LowerTrendline  Trendline `json:"lower_trendline"`
	ApexPrice       float64   `json:"apex_price"`        // Convergence price
	ApexIndex       int       `json:"apex_index"`
	PatternStart    int       `json:"pattern_start"`
	PatternWidth    int       `json:"pattern_width"`
	SlopeRatio      float64   `json:"slope_ratio"`       // Ratio of slopes
	BaseHeight      float64   `json:"base_height"`
	BasePercent     float64   `json:"base_percent"`
	BreakoutBias    string    `json:"breakout_bias"`     // "up" for falling, "down" for rising
	BreakoutTarget  float64   `json:"breakout_target"`
	TouchesUpper    int       `json:"touches_upper"`
	TouchesLower    int       `json:"touches_lower"`
	VolumeDecline   bool      `json:"volume_decline"`
	Completed       bool      `json:"completed"`
	BreakoutDir     string    `json:"breakout_dir"`
	CandleIndex     int       `json:"candle_index"`
	Timestamp       time.Time `json:"timestamp"`
	Strength        string    `json:"strength"`
}

// FlagPennantPattern represents a flag or pennant continuation pattern
type FlagPennantPattern struct {
	Type            string    `json:"type"`              // "bull_flag", "bear_flag", "bull_pennant", "bear_pennant"
	Direction       string    `json:"direction"`         // "bullish" or "bearish"

	// Flagpole
	FlagpoleStart   SwingPoint `json:"flagpole_start"`
	FlagpoleEnd     SwingPoint `json:"flagpole_end"`
	FlagpoleHeight  float64   `json:"flagpole_height"`   // In price units
	FlagpolePercent float64   `json:"flagpole_percent"`  // As % move
	FlagpoleBars    int       `json:"flagpole_bars"`
	FlagpoleVolume  float64   `json:"flagpole_volume"`   // Average volume during impulse

	// Consolidation
	ConsolidationType string  `json:"consolidation_type"` // "channel" or "triangle"
	ConsolidationHigh float64 `json:"consolidation_high"`
	ConsolidationLow  float64 `json:"consolidation_low"`
	ConsolidationBars int     `json:"consolidation_bars"`
	RetracementPct    float64 `json:"retracement_pct"`   // How much flagpole was retraced
	ConsolidationVol  float64 `json:"consolidation_vol"` // Average volume during consolidation

	// Targets
	BreakoutLevel   float64   `json:"breakout_level"`
	TargetPrice     float64   `json:"target_price"`
	StopLoss        float64   `json:"stop_loss"`

	// Status
	Completed       bool      `json:"completed"`
	VolumeConfirmed bool      `json:"volume_confirmed"`
	CandleIndex     int       `json:"candle_index"`
	Timestamp       time.Time `json:"timestamp"`
	Strength        string    `json:"strength"`
}

// PatternSummary provides a quick overview of the most significant pattern
type PatternSummary struct {
	Type           string  `json:"type"`
	Direction      string  `json:"direction"`      // "bullish" or "bearish"
	Strength       string  `json:"strength"`
	TargetPrice    float64 `json:"target_price"`
	BreakoutLevel  float64 `json:"breakout_level"`
	CompletionPct  float64 `json:"completion_pct"` // How close to breakout (0-100)
}

// ChartPatternAnalysis contains all detected chart patterns
type ChartPatternAnalysis struct {
	// Detected patterns
	HeadAndShoulders  []HeadAndShouldersPattern  `json:"head_and_shoulders"`
	DoubleTopsBottoms []DoubleTopBottomPattern   `json:"double_tops_bottoms"`
	Triangles         []TrianglePattern          `json:"triangles"`
	Wedges            []WedgePattern             `json:"wedges"`
	FlagsPennants     []FlagPennantPattern       `json:"flags_pennants"`

	// Active/nearest pattern summary
	ActivePattern     *PatternSummary            `json:"active_pattern"`

	// Scoring
	PatternScore      float64                    `json:"pattern_score"`      // 0-100
	PatternBias       string                     `json:"pattern_bias"`       // "bullish", "bearish", "neutral"
	PatternConfluence bool                       `json:"pattern_confluence"` // Aligns with trend

	// Trading signals
	HasBullishPattern bool                       `json:"has_bullish_pattern"`
	HasBearishPattern bool                       `json:"has_bearish_pattern"`
	NearBreakout      bool                       `json:"near_breakout"`
	EstimatedTarget   float64                    `json:"estimated_target"`

	// Pattern counts for quick reference
	TotalPatterns     int                        `json:"total_patterns"`
	ReversalPatterns  int                        `json:"reversal_patterns"`  // H&S, Double Top/Bottom
	ContinuationPatterns int                     `json:"continuation_patterns"` // Flags, Pennants
	ConsolidationPatterns int                    `json:"consolidation_patterns"` // Triangles, Wedges
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

	// AI/LLM Sizing - suggested position size from LLM analysis
	LLMSuggestedSizeUSD float64 `json:"llm_suggested_size_usd,omitempty"` // LLM recommended position size in USD
	LLMSizeReasoning    string  `json:"llm_size_reasoning,omitempty"`     // LLM reasoning for size recommendation

	// Reversal Entry - for LIMIT order entries at previous candle's extreme
	UseReversal     bool    `json:"use_reversal,omitempty"`      // Whether this is a reversal entry
	EntryType       string  `json:"entry_type,omitempty"`        // "MARKET" or "LIMIT"
	LimitEntryPrice float64 `json:"limit_entry_price,omitempty"` // Price for LIMIT order (prev candle low/high)
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

	// Rejection Tracking (helps users understand WHY a coin isn't being traded)
	RejectionTracking *RejectionTracker `json:"rejection_tracking,omitempty"`

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

	// Counter-Trend Trading Configuration
	AllowCounterTrend              bool    `json:"allow_counter_trend"`                // Enable counter-trend trading
	CounterTrendMinConfidence      float64 `json:"counter_trend_min_confidence"`       // Min confidence for counter-trend trades (0-100)
	CounterTrendRequireReversal    bool    `json:"counter_trend_require_reversal"`     // Require reversal pattern confirmation
	CounterTrendRequireRSIExtreme  bool    `json:"counter_trend_require_rsi_extreme"`  // Require RSI in extreme zone
	CounterTrendRequireADXWeakening bool   `json:"counter_trend_require_adx_weakening"` // Require ADX weakening

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
		SwingADXMin:         30,
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

		// Counter-Trend Trading (more permissive defaults)
		AllowCounterTrend:              true,
		CounterTrendMinConfidence:      50.0, // 50% instead of 80% - allow contrarian trades with decent signals
		CounterTrendRequireReversal:    true,  // Still require reversal pattern for safety
		CounterTrendRequireRSIExtreme:  false, // Don't require extreme RSI (too restrictive)
		CounterTrendRequireADXWeakening: false, // Don't require ADX weakening (too restrictive)

		// Monitoring
		ScalpMonitorInterval:    900,
		SwingMonitorInterval:    14400,
		PositionMonitorInterval: 86400,
	}
}

// GinieStatus represents current Ginie status
type GinieStatus struct {
	Enabled           bool              `json:"enabled"`
	DryRun            bool              `json:"dry_run"`
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
	MinSizeUSD     float64 `json:"min_size_usd"`     // Minimum recommended position size in USD (to cover fees)
	MaxSizeUSD     float64 `json:"max_size_usd"`     // Maximum position size in USD
	MaxPositions   int     `json:"max_positions"`    // Maximum concurrent positions for this mode
	Leverage       int     `json:"leverage"`         // Leverage to use for positions
	SizeMultiplier float64 `json:"size_multiplier"`  // Multiplier applied based on confidence

	// Fee Configuration (for user awareness)
	TakerFeePercent float64 `json:"taker_fee_percent"` // Taker fee percentage (default 0.05%)
	MakerFeePercent float64 `json:"maker_fee_percent"` // Maker fee percentage (default 0.02%)

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

// RejectionTracker tracks all rejection reasons for a trade decision
// This helps users understand WHY a coin with a good score isn't being traded
type RejectionTracker struct {
	// Overall rejection status
	IsBlocked      bool     `json:"is_blocked"`       // True if trade is blocked
	BlockReason    string   `json:"block_reason"`     // Primary reason for blocking
	AllReasons     []string `json:"all_reasons"`      // All rejection reasons accumulated

	// Trend divergence rejection
	TrendDivergence *TrendDivergenceRejection `json:"trend_divergence,omitempty"`

	// Signal strength rejection
	SignalStrength *SignalStrengthRejection `json:"signal_strength,omitempty"`

	// Liquidity rejection
	Liquidity *LiquidityRejection `json:"liquidity,omitempty"`

	// ADX/Trend strength rejection
	ADXStrength *ADXStrengthRejection `json:"adx_strength,omitempty"`

	// Counter-trend rejection
	CounterTrend *CounterTrendRejection `json:"counter_trend,omitempty"`

	// Confidence rejection
	Confidence *ConfidenceRejection `json:"confidence,omitempty"`

	// Position limit rejection
	PositionLimit *PositionLimitRejection `json:"position_limit,omitempty"`

	// Fund/Balance rejection
	InsufficientFunds *InsufficientFundsRejection `json:"insufficient_funds,omitempty"`

	// Circuit breaker rejection
	CircuitBreaker *CircuitBreakerRejection `json:"circuit_breaker,omitempty"`

	// Scan quality rejection
	ScanQuality *ScanQualityRejection `json:"scan_quality,omitempty"`
}

// TrendDivergenceRejection tracks trend divergence blocking
type TrendDivergenceRejection struct {
	Blocked          bool   `json:"blocked"`
	ScanTimeframe    string `json:"scan_timeframe"`
	ScanTrend        string `json:"scan_trend"`
	DecisionTimeframe string `json:"decision_timeframe"`
	DecisionTrend    string `json:"decision_trend"`
	Severity         string `json:"severity"`
	Reason           string `json:"reason"`
}

// SignalStrengthRejection tracks insufficient signals
type SignalStrengthRejection struct {
	Blocked        bool     `json:"blocked"`
	SignalsMet     int      `json:"signals_met"`
	SignalsRequired int     `json:"signals_required"`
	FailedSignals  []string `json:"failed_signals"`
	Reason         string   `json:"reason"`
}

// LiquidityRejection tracks liquidity failures
type LiquidityRejection struct {
	Blocked        bool    `json:"blocked"`
	Volume24h      float64 `json:"volume_24h"`
	RequiredVolume float64 `json:"required_volume"`
	BidAskSpread   float64 `json:"bid_ask_spread"`
	MaxSpread      float64 `json:"max_spread"`
	Reason         string  `json:"reason"`
}

// ADXStrengthRejection tracks weak trend blocking
type ADXStrengthRejection struct {
	Blocked   bool    `json:"blocked"`
	ADXValue  float64 `json:"adx_value"`
	Threshold float64 `json:"threshold"`
	Penalty   float64 `json:"penalty"`      // Penalty applied (0.0-1.0)
	Reason    string  `json:"reason"`
}

// CounterTrendRejection tracks counter-trend trade blocking
type CounterTrendRejection struct {
	Blocked            bool   `json:"blocked"`
	SignalDirection    string `json:"signal_direction"`
	TrendDirection     string `json:"trend_direction"`
	MissingRequirements []string `json:"missing_requirements"`
	Reason             string `json:"reason"`
}

// ConfidenceRejection tracks low confidence blocking
type ConfidenceRejection struct {
	Blocked           bool    `json:"blocked"`
	ConfidenceScore   float64 `json:"confidence_score"`
	ExecuteThreshold  float64 `json:"execute_threshold"`
	WaitThreshold     float64 `json:"wait_threshold"`
	Reason            string  `json:"reason"`
}

// PositionLimitRejection tracks position limit blocking
type PositionLimitRejection struct {
	Blocked          bool   `json:"blocked"`
	CurrentPositions int    `json:"current_positions"`
	MaxPositions     int    `json:"max_positions"`
	Mode             string `json:"mode"`
	Reason           string `json:"reason"`
}

// InsufficientFundsRejection tracks fund/balance blocking
type InsufficientFundsRejection struct {
	Blocked         bool    `json:"blocked"`
	RequiredUSD     float64 `json:"required_usd"`
	AvailableUSD    float64 `json:"available_usd"`
	PositionSizeUSD float64 `json:"position_size_usd"`
	Reason          string  `json:"reason"`
}

// CircuitBreakerRejection tracks circuit breaker blocking
type CircuitBreakerRejection struct {
	Blocked      bool   `json:"blocked"`
	TripReason   string `json:"trip_reason"`
	CooldownMins int    `json:"cooldown_mins"`
	ResumeAt     string `json:"resume_at"`
	Reason       string `json:"reason"`
}

// ScanQualityRejection tracks poor scan quality blocking
type ScanQualityRejection struct {
	Blocked       bool    `json:"blocked"`
	ScanScore     float64 `json:"scan_score"`
	MinScore      float64 `json:"min_score"`
	TradeReady    bool    `json:"trade_ready"`
	ScanStatus    string  `json:"scan_status"`
	Reason        string  `json:"reason"`
}

// NewRejectionTracker creates a new rejection tracker
func NewRejectionTracker() *RejectionTracker {
	return &RejectionTracker{
		IsBlocked:  false,
		AllReasons: []string{},
	}
}

// AddRejection adds a rejection reason to the tracker
func (rt *RejectionTracker) AddRejection(reason string) {
	rt.AllReasons = append(rt.AllReasons, reason)
	if !rt.IsBlocked {
		rt.IsBlocked = true
		rt.BlockReason = reason // First rejection becomes primary reason
	}
}

// ===== REVERSAL ENTRY TYPES =====

// ReversalCandleData holds OHLCV data for a single candle in reversal analysis
type ReversalCandleData struct {
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume float64   `json:"volume"`
	Time   time.Time `json:"time"`
}

// ReversalPattern represents a detected Lower Lows or Higher Highs pattern
type ReversalPattern struct {
	PatternType    string               `json:"pattern_type"`     // "lower_lows" or "higher_highs"
	Direction      string               `json:"direction"`        // "LONG" for LL (reversal up), "SHORT" for HH (reversal down)
	Timeframe      string               `json:"timeframe"`        // "5m", "15m", "1h"
	CandleCount    int                  `json:"candle_count"`     // Number of consecutive candles analyzed (default: 3)
	Candles        []ReversalCandleData `json:"candles"`          // The analyzed candles
	PrevCandleLow  float64              `json:"prev_candle_low"`  // Entry price for LONG (previous candle's low)
	PrevCandleHigh float64              `json:"prev_candle_high"` // Entry price for SHORT (previous candle's high)
	Confidence     float64              `json:"confidence"`       // Pattern confidence 0-100
	DetectedAt     time.Time            `json:"detected_at"`
}

// MTFReversalAnalysis holds multi-timeframe reversal pattern confirmation
type MTFReversalAnalysis struct {
	Symbol         string           `json:"symbol"`
	Pattern5m      *ReversalPattern `json:"pattern_5m"`      // 5-minute pattern (primary for scalp)
	Pattern15m     *ReversalPattern `json:"pattern_15m"`     // 15-minute pattern (secondary)
	Pattern1h      *ReversalPattern `json:"pattern_1h"`      // 1-hour pattern (tertiary)
	Aligned        bool             `json:"aligned"`         // True if 2+ timeframes agree on direction
	AlignedCount   int              `json:"aligned_count"`   // Number of aligned timeframes (0-3)
	AlignmentScore float64          `json:"alignment_score"` // Weighted alignment score 0-100
	Direction      string           `json:"direction"`       // Final direction if aligned ("LONG" or "SHORT")
	EntryPrice     float64          `json:"entry_price"`     // Entry price from 5m previous candle
	Reason         string           `json:"reason"`          // Explanation of alignment
	AnalyzedAt     time.Time        `json:"analyzed_at"`
}

// LLMReversalConfirmation holds LLM analysis result for reversal probability
type LLMReversalConfirmation struct {
	IsReversal      bool     `json:"is_reversal"`       // Whether LLM confirms reversal
	Confidence      float64  `json:"confidence"`        // LLM confidence 0.0-1.0
	ReversalType    string   `json:"reversal_type"`     // "exhaustion", "capitulation", "structural", "false_signal"
	EntryPrice      float64  `json:"entry_price"`       // LLM recommended entry
	StopLossPrice   float64  `json:"stop_loss_price"`   // LLM recommended SL
	TakeProfitPrice float64  `json:"take_profit_price"` // LLM recommended TP
	Reasoning       string   `json:"reasoning"`         // LLM reasoning
	CautionFlags    []string `json:"caution_flags"`     // Warning flags
	NearestSupport  float64  `json:"nearest_support"`   // Key support level
	NearestResist   float64  `json:"nearest_resistance"` // Key resistance level
}

// PendingLimitOrder tracks unfilled LIMIT orders for reversal entries
type PendingLimitOrder struct {
	OrderID      int64       `json:"order_id"`
	Symbol       string      `json:"symbol"`
	Side         string      `json:"side"`         // "BUY" or "SELL"
	PositionSide string      `json:"position_side"` // "LONG" or "SHORT"
	Price        float64     `json:"price"`
	Quantity     float64     `json:"quantity"`
	PlacedAt     time.Time   `json:"placed_at"`
	TimeoutAt    time.Time   `json:"timeout_at"`
	Source       string      `json:"source"`       // "reversal_entry"
	Mode         GinieTradingMode `json:"mode"`
}
