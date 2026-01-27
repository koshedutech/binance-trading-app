// Package research provides data structures and services for research infrastructure.
// Story 15.7: Simple Backtest Engine - Types and condition definitions
package research

import (
	"fmt"
	"math"
	"time"
)

// Operator represents a comparison operator for conditions.
type Operator string

const (
	OpLT   Operator = "<"
	OpLTE  Operator = "<="
	OpGT   Operator = ">"
	OpGTE  Operator = ">="
	OpEQ   Operator = "=="
	OpNEQ  Operator = "!="
)

// IsValid checks if the operator is a valid supported value.
func (o Operator) IsValid() bool {
	switch o {
	case OpLT, OpLTE, OpGT, OpGTE, OpEQ, OpNEQ:
		return true
	default:
		return false
	}
}

// ParseOperator converts a string to an Operator.
func ParseOperator(s string) (Operator, error) {
	op := Operator(s)
	if !op.IsValid() {
		return "", fmt.Errorf("invalid operator: %s", s)
	}
	return op, nil
}

// EntryLogic represents how multiple conditions are combined.
type EntryLogic string

const (
	LogicAND EntryLogic = "AND"
	LogicOR  EntryLogic = "OR"
)

// IsValid checks if the logic is valid.
func (l EntryLogic) IsValid() bool {
	return l == LogicAND || l == LogicOR
}

// EntryCondition represents a single entry condition.
// Example: { "feature": "return_5c", "operator": "<", "value": -2.0 }
type EntryCondition struct {
	Feature  string   `json:"feature"`  // Feature name (e.g., "return_5c", "rsi_14")
	Operator Operator `json:"operator"` // Comparison operator
	Value    float64  `json:"value"`    // Threshold value
}

// Evaluate checks if the condition is met given a feature value.
// Returns false if the feature value is NaN.
func (c *EntryCondition) Evaluate(featureValue float64) bool {
	if math.IsNaN(featureValue) {
		return false
	}

	// Epsilon for floating point equality comparison
	const epsilon = 1e-9

	switch c.Operator {
	case OpLT:
		return featureValue < c.Value
	case OpLTE:
		return featureValue <= c.Value
	case OpGT:
		return featureValue > c.Value
	case OpGTE:
		return featureValue >= c.Value
	case OpEQ:
		// Use epsilon-based comparison for floating point equality
		return math.Abs(featureValue-c.Value) < epsilon
	case OpNEQ:
		// Use epsilon-based comparison for floating point inequality
		return math.Abs(featureValue-c.Value) >= epsilon
	default:
		return false
	}
}

// Validate checks if the condition is valid.
func (c *EntryCondition) Validate() error {
	if c.Feature == "" {
		return fmt.Errorf("feature name is required")
	}
	if !c.Operator.IsValid() {
		return fmt.Errorf("invalid operator: %s", c.Operator)
	}
	return nil
}

// TradeOutcome represents the result of a simulated trade.
type TradeOutcome string

const (
	OutcomeWin     TradeOutcome = "win"     // Take profit hit
	OutcomeLoss    TradeOutcome = "loss"    // Stop loss hit
	OutcomeTimeout TradeOutcome = "timeout" // Max hold candles reached
)

// Trade represents a single simulated trade.
type Trade struct {
	Symbol         string        `json:"symbol"`
	EntryTime      time.Time     `json:"entry_time"`
	EntryPrice     float64       `json:"entry_price"`
	EntryATR       float64       `json:"entry_atr"`       // ATR at entry for SL/TP calculation
	StopLoss       float64       `json:"stop_loss"`       // SL price
	TakeProfit     float64       `json:"take_profit"`     // TP price
	ExitTime       time.Time     `json:"exit_time"`
	ExitPrice      float64       `json:"exit_price"`
	Outcome        TradeOutcome  `json:"outcome"`
	HoldCandles    int           `json:"hold_candles"`    // Number of candles held
	PnLPercent     float64       `json:"pnl_percent"`     // Percentage P&L
	PnLATR         float64       `json:"pnl_atr"`         // P&L in ATR units
	FeesPercent    float64       `json:"fees_percent"`    // Total fees (entry + exit)
	NetPnLPercent  float64       `json:"net_pnl_percent"` // P&L after fees
}

// BacktestRequest represents the input parameters for a backtest.
type BacktestRequest struct {
	// Coins to test (supports multiple)
	Coins []string `json:"coins"`

	// Timeframe for candle data
	Timeframe Timeframe `json:"timeframe"`

	// Date range
	From time.Time `json:"from"`
	To   time.Time `json:"to"`

	// Entry conditions
	EntryConditions []EntryCondition `json:"entry_conditions"`
	EntryLogic      EntryLogic       `json:"entry_logic"` // AND or OR

	// Risk management
	SLATRMultiplier float64 `json:"sl_atr_multiplier"` // Stop loss = Entry - (ATR * multiplier)
	TPATRMultiplier float64 `json:"tp_atr_multiplier"` // Take profit = Entry + (ATR * multiplier)
	ATRPeriod       int     `json:"atr_period"`        // ATR calculation period (default 14)
	MaxHoldCandles  int     `json:"max_hold_candles"`  // Maximum candles to hold (timeout)

	// Fees
	IncludeFees bool    `json:"include_fees"`
	FeePercent  float64 `json:"fee_percent"` // Fee per trade (entry or exit), typically 0.04%
}

// Validate checks if the request is valid.
func (r *BacktestRequest) Validate() error {
	if len(r.Coins) == 0 {
		return fmt.Errorf("at least one coin is required")
	}
	if !r.Timeframe.IsValid() {
		return fmt.Errorf("invalid timeframe: %s", r.Timeframe)
	}
	if r.From.IsZero() {
		return fmt.Errorf("from date is required")
	}
	if r.To.IsZero() {
		return fmt.Errorf("to date is required")
	}
	if r.To.Before(r.From) {
		return fmt.Errorf("to date must be after from date")
	}
	if len(r.EntryConditions) == 0 {
		return fmt.Errorf("at least one entry condition is required")
	}
	for i, cond := range r.EntryConditions {
		if err := cond.Validate(); err != nil {
			return fmt.Errorf("invalid condition %d: %w", i, err)
		}
	}
	if !r.EntryLogic.IsValid() {
		return fmt.Errorf("invalid entry logic: %s (must be AND or OR)", r.EntryLogic)
	}
	if r.SLATRMultiplier <= 0 {
		return fmt.Errorf("sl_atr_multiplier must be positive")
	}
	if r.TPATRMultiplier <= 0 {
		return fmt.Errorf("tp_atr_multiplier must be positive")
	}
	if r.MaxHoldCandles <= 0 {
		return fmt.Errorf("max_hold_candles must be positive")
	}
	return nil
}

// SetDefaults fills in default values for optional fields.
func (r *BacktestRequest) SetDefaults() {
	if r.ATRPeriod <= 0 {
		r.ATRPeriod = 14
	}
	if r.EntryLogic == "" {
		r.EntryLogic = LogicAND
	}
	if r.FeePercent == 0 && r.IncludeFees {
		r.FeePercent = 0.04 // Default Binance fee
	}
}

// BacktestSummary holds aggregate statistics for the backtest.
type BacktestSummary struct {
	TotalTrades        int     `json:"total_trades"`
	Wins               int     `json:"wins"`
	Losses             int     `json:"losses"`
	Timeouts           int     `json:"timeouts"`
	WinRate            float64 `json:"win_rate"`             // Percentage
	ExpectedATR        float64 `json:"expected_atr"`         // Expected value in ATR units
	ExpectedPercent    float64 `json:"expected_percent"`     // Expected value in percent
	SharpeRatio        float64 `json:"sharpe_ratio"`         // Risk-adjusted return
	ProfitFactor       float64 `json:"profit_factor"`        // Gross profit / Gross loss
	MaxDrawdownPercent float64 `json:"max_drawdown_percent"` // Maximum peak-to-trough decline
	AvgHoldCandles     float64 `json:"avg_hold_candles"`     // Average trade duration

	// Additional useful metrics
	AvgWinPercent       float64 `json:"avg_win_percent"`       // Average winning trade
	AvgLossPercent      float64 `json:"avg_loss_percent"`      // Average losing trade
	LargestWinPercent   float64 `json:"largest_win_percent"`   // Best trade
	LargestLossPercent  float64 `json:"largest_loss_percent"`  // Worst trade
	ConsecutiveWins     int     `json:"consecutive_wins"`      // Max consecutive wins
	ConsecutiveLosses   int     `json:"consecutive_losses"`    // Max consecutive losses
	TotalGrossProfit    float64 `json:"total_gross_profit"`    // Sum of winning trades
	TotalGrossLoss      float64 `json:"total_gross_loss"`      // Sum of losing trades (negative)
	TotalNetProfit      float64 `json:"total_net_profit"`      // Net profit after fees
	TotalFeesPercent    float64 `json:"total_fees_percent"`    // Total fees paid
}

// CoinBreakdown holds statistics for a single coin.
type CoinBreakdown struct {
	Symbol       string          `json:"symbol"`
	TotalTrades  int             `json:"total_trades"`
	Wins         int             `json:"wins"`
	Losses       int             `json:"losses"`
	Timeouts     int             `json:"timeouts"`
	WinRate      float64         `json:"win_rate"`
	ExpectedATR  float64         `json:"expected_atr"`
	AvgHoldCandles float64       `json:"avg_hold_candles"`
	NetProfit    float64         `json:"net_profit"`
}

// ValidationResult holds statistical validation information.
type ValidationResult struct {
	PValue             float64 `json:"p_value"`             // Statistical significance
	Significant        bool    `json:"significant"`         // Is p_value < 0.05?
	SampleSizeAdequate bool    `json:"sample_size_adequate"` // Enough trades for reliability
	MinSampleSize      int     `json:"min_sample_size"`     // Recommended minimum (typically 30)
}

// BacktestResult represents the complete result of a backtest.
type BacktestResult struct {
	Summary         BacktestSummary   `json:"summary"`
	ByCoin          []CoinBreakdown   `json:"by_coin"`
	Validation      ValidationResult  `json:"validation"`
	ExecutionTimeMs int64             `json:"execution_time_ms"`

	// Optional: Include trades for detailed analysis
	Trades []Trade `json:"trades,omitempty"`

	// Request echo for reference
	Request BacktestRequest `json:"request,omitempty"`
}

// ConditionEvaluator evaluates entry conditions against feature data.
type ConditionEvaluator struct {
	conditions     []EntryCondition
	logic          EntryLogic
	neededFeatures map[string]bool // Cache of features needed for conditions
}

// NewConditionEvaluator creates a new evaluator with the given conditions.
func NewConditionEvaluator(conditions []EntryCondition, logic EntryLogic) *ConditionEvaluator {
	// Pre-compute needed features for memory efficiency
	needed := make(map[string]bool, len(conditions))
	for _, cond := range conditions {
		needed[cond.Feature] = true
	}

	return &ConditionEvaluator{
		conditions:     conditions,
		logic:          logic,
		neededFeatures: needed,
	}
}

// Evaluate checks if all conditions are met for the given features.
// Returns true if entry should occur based on the logic (AND/OR).
func (e *ConditionEvaluator) Evaluate(features *CandleFeatures) bool {
	if features == nil || len(e.conditions) == 0 {
		return false
	}

	if e.logic == LogicAND {
		// All conditions must be true
		for _, cond := range e.conditions {
			value, ok := e.getFeatureValue(features, cond.Feature)
			if !ok || !cond.Evaluate(value) {
				return false
			}
		}
		return true
	}

	// OR logic: at least one condition must be true
	for _, cond := range e.conditions {
		value, ok := e.getFeatureValue(features, cond.Feature)
		if ok && cond.Evaluate(value) {
			return true
		}
	}
	return false
}

// getFeatureValue retrieves a single feature value by name.
// More memory efficient than building a full map for every evaluation.
func (e *ConditionEvaluator) getFeatureValue(f *CandleFeatures, name string) (float64, bool) {
	switch name {
	// Price features
	case "return_1c":
		if f.Price != nil {
			return f.Price.Return1c, true
		}
	case "return_3c":
		if f.Price != nil {
			return f.Price.Return3c, true
		}
	case "return_5c":
		if f.Price != nil {
			return f.Price.Return5c, true
		}
	case "return_10c":
		if f.Price != nil {
			return f.Price.Return10c, true
		}
	case "return_20c":
		if f.Price != nil {
			return f.Price.Return20c, true
		}
	case "body_ratio":
		if f.Price != nil {
			return f.Price.BodyRatio, true
		}
	case "upper_wick_ratio":
		if f.Price != nil {
			return f.Price.UpperWickRatio, true
		}
	case "lower_wick_ratio":
		if f.Price != nil {
			return f.Price.LowerWickRatio, true
		}
	case "position_in_range":
		if f.Price != nil {
			return f.Price.PositionInRange, true
		}
	case "gap_percent":
		if f.Price != nil {
			return f.Price.GapPercent, true
		}
	case "range_percent":
		if f.Price != nil {
			return f.Price.RangePercent, true
		}
	case "consecutive_up":
		if f.Price != nil {
			return float64(f.Price.ConsecutiveUp), true
		}
	case "consecutive_down":
		if f.Price != nil {
			return float64(f.Price.ConsecutiveDown), true
		}
	case "higher_high":
		if f.Price != nil {
			if f.Price.HigherHigh {
				return 1.0, true
			}
			return 0.0, true
		}
	case "lower_low":
		if f.Price != nil {
			if f.Price.LowerLow {
				return 1.0, true
			}
			return 0.0, true
		}

	// Volume features
	case "volume_ratio_5":
		if f.Volume != nil {
			return f.Volume.VolumeRatio5, true
		}
	case "volume_ratio_10":
		if f.Volume != nil {
			return f.Volume.VolumeRatio10, true
		}
	case "volume_ratio_20":
		if f.Volume != nil {
			return f.Volume.VolumeRatio20, true
		}
	case "volume_trend_5":
		if f.Volume != nil {
			return f.Volume.VolumeTrend5, true
		}
	case "taker_buy_ratio":
		if f.Volume != nil {
			return f.Volume.TakerBuyRatio, true
		}
	case "trade_count_value":
		if f.Volume != nil {
			return f.Volume.TradeCountValue, true
		}
	case "avg_trade_size":
		if f.Volume != nil {
			return f.Volume.AvgTradeSize, true
		}
	case "obv":
		if f.Volume != nil {
			return f.Volume.OBV, true
		}
	case "obv_trend":
		if f.Volume != nil {
			return f.Volume.OBVTrend, true
		}
	case "volume_price_trend":
		if f.Volume != nil {
			return f.Volume.VolumePriceTrend, true
		}
	case "accum_dist":
		if f.Volume != nil {
			return f.Volume.AccumDist, true
		}
	case "money_flow":
		if f.Volume != nil {
			return f.Volume.MoneyFlow, true
		}

	// Volatility features
	case "atr_7":
		if f.Volatility != nil {
			return f.Volatility.ATR7, true
		}
	case "atr_14":
		if f.Volatility != nil {
			return f.Volatility.ATR14, true
		}
	case "atr_21":
		if f.Volatility != nil {
			return f.Volatility.ATR21, true
		}
	case "atr_percent":
		if f.Volatility != nil {
			return f.Volatility.ATRPercent, true
		}
	case "bollinger_upper":
		if f.Volatility != nil {
			return f.Volatility.BollingerUpper, true
		}
	case "bollinger_lower":
		if f.Volatility != nil {
			return f.Volatility.BollingerLower, true
		}
	case "bollinger_width":
		if f.Volatility != nil {
			return f.Volatility.BollingerWidth, true
		}
	case "bollinger_position":
		if f.Volatility != nil {
			return f.Volatility.BollingerPosition, true
		}
	case "range_expansion":
		if f.Volatility != nil {
			return f.Volatility.RangeExpansion, true
		}
	case "volatility_ratio":
		if f.Volatility != nil {
			return f.Volatility.VolatilityRatio, true
		}

	// Momentum features
	case "rsi_7":
		if f.Momentum != nil {
			return f.Momentum.RSI7, true
		}
	case "rsi_14":
		if f.Momentum != nil {
			return f.Momentum.RSI14, true
		}
	case "rsi_21":
		if f.Momentum != nil {
			return f.Momentum.RSI21, true
		}
	case "stoch_k":
		if f.Momentum != nil {
			return f.Momentum.StochK, true
		}
	case "stoch_d":
		if f.Momentum != nil {
			return f.Momentum.StochD, true
		}
	case "macd_line":
		if f.Momentum != nil {
			return f.Momentum.MACDLine, true
		}
	case "macd_signal":
		if f.Momentum != nil {
			return f.Momentum.MACDSignal, true
		}
	case "macd_histogram":
		if f.Momentum != nil {
			return f.Momentum.MACDHistogram, true
		}
	case "roc_5":
		if f.Momentum != nil {
			return f.Momentum.ROC5, true
		}
	case "roc_10":
		if f.Momentum != nil {
			return f.Momentum.ROC10, true
		}
	case "roc_20":
		if f.Momentum != nil {
			return f.Momentum.ROC20, true
		}
	case "momentum_5":
		if f.Momentum != nil {
			return f.Momentum.Momentum5, true
		}
	case "momentum_10":
		if f.Momentum != nil {
			return f.Momentum.Momentum10, true
		}
	case "williams_r":
		if f.Momentum != nil {
			return f.Momentum.WilliamsR, true
		}

	// Trend features
	case "sma_10":
		if f.Trend != nil {
			return f.Trend.SMA10, true
		}
	case "sma_20":
		if f.Trend != nil {
			return f.Trend.SMA20, true
		}
	case "sma_50":
		if f.Trend != nil {
			return f.Trend.SMA50, true
		}
	case "ema_9":
		if f.Trend != nil {
			return f.Trend.EMA9, true
		}
	case "ema_21":
		if f.Trend != nil {
			return f.Trend.EMA21, true
		}
	case "price_vs_sma_20":
		if f.Trend != nil {
			return f.Trend.PriceVsSMA20, true
		}
	case "sma_cross":
		if f.Trend != nil {
			return float64(f.Trend.SMACross), true
		}
	case "adx":
		if f.Trend != nil {
			return f.Trend.ADX, true
		}
	case "plus_di":
		if f.Trend != nil {
			return f.Trend.PlusDI, true
		}
	case "minus_di":
		if f.Trend != nil {
			return f.Trend.MinusDI, true
		}
	case "trend_strength":
		if f.Trend != nil {
			return f.Trend.TrendStrength, true
		}
	case "trend_direction":
		if f.Trend != nil {
			return float64(f.Trend.TrendDirection), true
		}

	// Time features
	case "hour_of_day":
		if f.Time != nil {
			return float64(f.Time.HourOfDay), true
		}
	case "day_of_week":
		if f.Time != nil {
			return float64(f.Time.DayOfWeek), true
		}
	case "is_weekend":
		if f.Time != nil {
			if f.Time.IsWeekend {
				return 1.0, true
			}
			return 0.0, true
		}
	case "day_of_month":
		if f.Time != nil {
			return float64(f.Time.DayOfMonth), true
		}
	case "is_month_end":
		if f.Time != nil {
			if f.Time.IsMonthEnd {
				return 1.0, true
			}
			return 0.0, true
		}
	}

	return 0, false
}

// GetAvailableFeatures returns a list of all available feature names.
func GetAvailableFeatures() []string {
	return []string{
		// Price features (15)
		"return_1c", "return_3c", "return_5c", "return_10c", "return_20c",
		"body_ratio", "upper_wick_ratio", "lower_wick_ratio", "position_in_range",
		"gap_percent", "range_percent", "consecutive_up", "consecutive_down",
		"higher_high", "lower_low",

		// Volume features (12)
		"volume_ratio_5", "volume_ratio_10", "volume_ratio_20", "volume_trend_5",
		"taker_buy_ratio", "trade_count_value", "avg_trade_size", "obv", "obv_trend",
		"volume_price_trend", "accum_dist", "money_flow",

		// Volatility features (10)
		"atr_7", "atr_14", "atr_21", "atr_percent",
		"bollinger_upper", "bollinger_lower", "bollinger_width", "bollinger_position",
		"range_expansion", "volatility_ratio",

		// Momentum features (14)
		"rsi_7", "rsi_14", "rsi_21", "stoch_k", "stoch_d",
		"macd_line", "macd_signal", "macd_histogram",
		"roc_5", "roc_10", "roc_20", "momentum_5", "momentum_10", "williams_r",

		// Trend features (12)
		"sma_10", "sma_20", "sma_50", "ema_9", "ema_21", "price_vs_sma_20",
		"sma_cross", "adx", "plus_di", "minus_di", "trend_strength", "trend_direction",

		// Time features (5)
		"hour_of_day", "day_of_week", "is_weekend", "day_of_month", "is_month_end",
	}
}

// ValidateFeatureName checks if a feature name is valid.
func ValidateFeatureName(name string) bool {
	features := GetAvailableFeatures()
	for _, f := range features {
		if f == name {
			return true
		}
	}
	return false
}
