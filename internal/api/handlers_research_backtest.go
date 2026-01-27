// Package api provides HTTP API handlers for the trading bot.
// Epic 15, Story 15.9: Backtest API Endpoints
// Provides endpoints for running backtests, walk-forward validation,
// and querying available features for the Pattern Discovery Agent.
package api

import (
	"context"
	"net/http"
	"time"

	"binance-trading-bot/internal/research"

	"github.com/gin-gonic/gin"
)

// BacktestAPIRequest represents the request body for running a backtest.
// POST /api/research/backtest
type BacktestAPIRequest struct {
	Coins           []string                   `json:"coins" binding:"required"`
	Timeframe       string                     `json:"timeframe" binding:"required"`
	From            string                     `json:"from" binding:"required"`          // Format: "2023-01-01"
	To              string                     `json:"to" binding:"required"`            // Format: "2023-12-31"
	EntryConditions []research.EntryCondition  `json:"entry_conditions" binding:"required"`
	EntryLogic      string                     `json:"entry_logic"`                      // "AND" or "OR", defaults to "AND"
	SLATRMultiplier float64                    `json:"sl_atr_multiplier" binding:"required"`
	TPATRMultiplier float64                    `json:"tp_atr_multiplier" binding:"required"`
	ATRPeriod       int                        `json:"atr_period"`                       // Defaults to 14
	MaxHoldCandles  int                        `json:"max_hold_candles" binding:"required"`
	IncludeFees     bool                       `json:"include_fees"`
	FeePercent      float64                    `json:"fee_percent"`                      // Defaults to 0.04%
	IncludeTrades   bool                       `json:"include_trades"`                   // Whether to include individual trades in response
	MaxTrades       int                        `json:"max_trades"`                       // Max trades to return (0 = all)
}

// BacktestAPIResponse represents the response for a backtest.
type BacktestAPIResponse struct {
	Summary         research.BacktestSummary   `json:"summary"`
	ByCoin          []research.CoinBreakdown   `json:"by_coin"`
	Validation      research.ValidationResult  `json:"validation"`
	Trades          []research.Trade           `json:"trades,omitempty"`
	ExecutionTimeMs int64                      `json:"execution_time_ms"`
}

// WalkForwardAPIRequest represents the request body for walk-forward validation.
// POST /api/research/walk-forward
type WalkForwardAPIRequest struct {
	Coins           []string                   `json:"coins" binding:"required"`
	Timeframe       string                     `json:"timeframe" binding:"required"`
	From            string                     `json:"from" binding:"required"`
	To              string                     `json:"to" binding:"required"`
	EntryConditions []research.EntryCondition  `json:"entry_conditions" binding:"required"`
	EntryLogic      string                     `json:"entry_logic"`
	SLATRMultiplier float64                    `json:"sl_atr_multiplier" binding:"required"`
	TPATRMultiplier float64                    `json:"tp_atr_multiplier" binding:"required"`
	ATRPeriod       int                        `json:"atr_period"`
	MaxHoldCandles  int                        `json:"max_hold_candles" binding:"required"`
	IncludeFees     bool                       `json:"include_fees"`
	FeePercent      float64                    `json:"fee_percent"`

	// Walk-forward specific parameters
	TrainMonths    int `json:"train_months"`    // Defaults to 6
	TestMonths     int `json:"test_months"`     // Defaults to 3
	StepMonths     int `json:"step_months"`     // Defaults to 3
	HoldoutPercent int `json:"holdout_percent"` // Defaults to 20 (0-50)
}

// FeatureCategoryResponse represents features grouped by category.
type FeatureCategoryResponse struct {
	Price      []string `json:"price"`
	Volume     []string `json:"volume"`
	Volatility []string `json:"volatility"`
	Momentum   []string `json:"momentum"`
	Trend      []string `json:"trend"`
	Time       []string `json:"time"`
}

// FeaturesResponse represents the response for available features.
type FeaturesResponse struct {
	Features   []string                 `json:"features"`
	Count      int                      `json:"count"`
	Categories FeatureCategoryResponse  `json:"categories"`
	Operators  []string                 `json:"operators"`
}

// handleRunResearchBacktest runs a simple research backtest.
// POST /api/research/backtest
func (s *Server) handleRunResearchBacktest(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	var req BacktestAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Parse dates
	fromDate, err := time.Parse("2006-01-02", req.From)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid from date format (use YYYY-MM-DD)")
		return
	}

	toDate, err := time.Parse("2006-01-02", req.To)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid to date format (use YYYY-MM-DD)")
		return
	}

	// Validate date range
	if toDate.Before(fromDate) {
		errorResponse(c, http.StatusBadRequest, "to date must be after from date")
		return
	}
	if toDate.After(time.Now()) {
		errorResponse(c, http.StatusBadRequest, "to date cannot be in the future")
		return
	}
	maxDateRange := 5 * 365 * 24 * time.Hour // 5 years max
	if toDate.Sub(fromDate) > maxDateRange {
		errorResponse(c, http.StatusBadRequest, "Date range exceeds maximum allowed (5 years)")
		return
	}

	// Validate timeframe
	tf := research.Timeframe(req.Timeframe)
	if !tf.IsValid() {
		errorResponse(c, http.StatusBadRequest, "Invalid timeframe: "+req.Timeframe+". Valid options: 1m, 5m, 15m, 30m, 1h, 4h, 1d")
		return
	}

	// Convert entry logic
	entryLogic := research.LogicAND
	if req.EntryLogic != "" {
		entryLogic = research.EntryLogic(req.EntryLogic)
		if !entryLogic.IsValid() {
			errorResponse(c, http.StatusBadRequest, "Invalid entry_logic: must be AND or OR")
			return
		}
	}

	// Validate entry conditions (feature names and operators)
	for i, cond := range req.EntryConditions {
		if cond.Feature == "" {
			errorResponse(c, http.StatusBadRequest, "Entry condition "+string(rune('0'+i))+": feature name is required")
			return
		}
		if !research.ValidateFeatureName(cond.Feature) {
			errorResponse(c, http.StatusBadRequest, "Invalid feature name in condition: "+cond.Feature+". Use GET /api/research/features for available features")
			return
		}
		op := research.Operator(cond.Operator)
		if !op.IsValid() {
			errorResponse(c, http.StatusBadRequest, "Invalid operator in condition: "+string(cond.Operator)+". Valid operators: <, <=, >, >=, ==, !=")
			return
		}
	}

	// Validate numeric parameters are within reasonable bounds
	if req.SLATRMultiplier <= 0 || req.SLATRMultiplier > 10 {
		errorResponse(c, http.StatusBadRequest, "sl_atr_multiplier must be between 0 and 10")
		return
	}
	if req.TPATRMultiplier <= 0 || req.TPATRMultiplier > 10 {
		errorResponse(c, http.StatusBadRequest, "tp_atr_multiplier must be between 0 and 10")
		return
	}
	if req.MaxHoldCandles <= 0 || req.MaxHoldCandles > 10000 {
		errorResponse(c, http.StatusBadRequest, "max_hold_candles must be between 1 and 10000")
		return
	}
	if req.ATRPeriod < 0 || req.ATRPeriod > 200 {
		errorResponse(c, http.StatusBadRequest, "atr_period must be between 0 and 200")
		return
	}
	if req.FeePercent < 0 || req.FeePercent > 1 {
		errorResponse(c, http.StatusBadRequest, "fee_percent must be between 0 and 1")
		return
	}
	if len(req.Coins) > 100 {
		errorResponse(c, http.StatusBadRequest, "Maximum 100 coins allowed per backtest request")
		return
	}

	// Check if backtest engine is available
	if s.backtestEngine == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Backtest engine not available")
		return
	}

	// Create backtest request
	backtestReq := &research.BacktestRequest{
		Coins:           req.Coins,
		Timeframe:       tf,
		From:            fromDate,
		To:              toDate,
		EntryConditions: req.EntryConditions,
		EntryLogic:      entryLogic,
		SLATRMultiplier: req.SLATRMultiplier,
		TPATRMultiplier: req.TPATRMultiplier,
		ATRPeriod:       req.ATRPeriod,
		MaxHoldCandles:  req.MaxHoldCandles,
		IncludeFees:     req.IncludeFees,
		FeePercent:      req.FeePercent,
	}

	// Use a timeout for backtest execution
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	// Run backtest with options
	// Limit max_trades to prevent excessive response size
	maxTrades := req.MaxTrades
	if maxTrades <= 0 || maxTrades > 10000 {
		maxTrades = 10000 // Cap at 10000 trades to prevent memory issues
	}
	opts := research.BacktestOptions{
		IncludeTrades: req.IncludeTrades,
		MaxTrades:     maxTrades,
	}

	result, err := s.backtestEngine.RunBacktestWithOptions(ctx, backtestReq, opts)
	if err != nil {
		// Check if error is due to context timeout/cancellation
		if ctx.Err() == context.DeadlineExceeded {
			errorResponse(c, http.StatusGatewayTimeout, "Backtest timed out after 5 minutes - try reducing date range or number of coins")
			return
		}
		if ctx.Err() == context.Canceled {
			errorResponse(c, http.StatusRequestTimeout, "Backtest request was cancelled")
			return
		}
		errorResponse(c, http.StatusBadRequest, "Backtest failed: "+err.Error())
		return
	}

	// Build response
	response := BacktestAPIResponse{
		Summary:         result.Summary,
		ByCoin:          result.ByCoin,
		Validation:      result.Validation,
		Trades:          result.Trades,
		ExecutionTimeMs: result.ExecutionTimeMs,
	}

	successResponse(c, response)
}

// handleRunResearchWalkForward runs a walk-forward validation backtest.
// POST /api/research/walk-forward
func (s *Server) handleRunResearchWalkForward(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	var req WalkForwardAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Parse dates
	fromDate, err := time.Parse("2006-01-02", req.From)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid from date format (use YYYY-MM-DD)")
		return
	}

	toDate, err := time.Parse("2006-01-02", req.To)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid to date format (use YYYY-MM-DD)")
		return
	}

	// Validate date range
	if toDate.Before(fromDate) {
		errorResponse(c, http.StatusBadRequest, "to date must be after from date")
		return
	}
	if toDate.After(time.Now()) {
		errorResponse(c, http.StatusBadRequest, "to date cannot be in the future")
		return
	}
	maxDateRange := 5 * 365 * 24 * time.Hour // 5 years max
	if toDate.Sub(fromDate) > maxDateRange {
		errorResponse(c, http.StatusBadRequest, "Date range exceeds maximum allowed (5 years)")
		return
	}

	// Validate timeframe
	tf := research.Timeframe(req.Timeframe)
	if !tf.IsValid() {
		errorResponse(c, http.StatusBadRequest, "Invalid timeframe: "+req.Timeframe+". Valid options: 1m, 5m, 15m, 30m, 1h, 4h, 1d")
		return
	}

	// Convert entry logic
	entryLogic := research.LogicAND
	if req.EntryLogic != "" {
		entryLogic = research.EntryLogic(req.EntryLogic)
		if !entryLogic.IsValid() {
			errorResponse(c, http.StatusBadRequest, "Invalid entry_logic: must be AND or OR")
			return
		}
	}

	// Validate entry conditions (feature names and operators)
	for i, cond := range req.EntryConditions {
		if cond.Feature == "" {
			errorResponse(c, http.StatusBadRequest, "Entry condition "+string(rune('0'+i))+": feature name is required")
			return
		}
		if !research.ValidateFeatureName(cond.Feature) {
			errorResponse(c, http.StatusBadRequest, "Invalid feature name in condition: "+cond.Feature+". Use GET /api/research/features for available features")
			return
		}
		op := research.Operator(cond.Operator)
		if !op.IsValid() {
			errorResponse(c, http.StatusBadRequest, "Invalid operator in condition: "+string(cond.Operator)+". Valid operators: <, <=, >, >=, ==, !=")
			return
		}
	}

	// Validate numeric parameters are within reasonable bounds
	if req.SLATRMultiplier <= 0 || req.SLATRMultiplier > 10 {
		errorResponse(c, http.StatusBadRequest, "sl_atr_multiplier must be between 0 and 10")
		return
	}
	if req.TPATRMultiplier <= 0 || req.TPATRMultiplier > 10 {
		errorResponse(c, http.StatusBadRequest, "tp_atr_multiplier must be between 0 and 10")
		return
	}
	if req.MaxHoldCandles <= 0 || req.MaxHoldCandles > 10000 {
		errorResponse(c, http.StatusBadRequest, "max_hold_candles must be between 1 and 10000")
		return
	}
	if req.ATRPeriod < 0 || req.ATRPeriod > 200 {
		errorResponse(c, http.StatusBadRequest, "atr_period must be between 0 and 200")
		return
	}
	if req.FeePercent < 0 || req.FeePercent > 1 {
		errorResponse(c, http.StatusBadRequest, "fee_percent must be between 0 and 1")
		return
	}
	if len(req.Coins) > 100 {
		errorResponse(c, http.StatusBadRequest, "Maximum 100 coins allowed per walk-forward request")
		return
	}
	// Validate walk-forward specific parameters
	if req.HoldoutPercent < 0 || req.HoldoutPercent > 50 {
		errorResponse(c, http.StatusBadRequest, "holdout_percent must be between 0 and 50")
		return
	}

	// Check if walk-forward engine is available
	if s.walkForwardEngine == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Walk-forward engine not available")
		return
	}

	// Create walk-forward request
	walkForwardReq := &research.WalkForwardRequest{
		Coins:           req.Coins,
		Timeframe:       tf,
		From:            fromDate,
		To:              toDate,
		EntryConditions: req.EntryConditions,
		EntryLogic:      entryLogic,
		SLATRMultiplier: req.SLATRMultiplier,
		TPATRMultiplier: req.TPATRMultiplier,
		ATRPeriod:       req.ATRPeriod,
		MaxHoldCandles:  req.MaxHoldCandles,
		IncludeFees:     req.IncludeFees,
		FeePercent:      req.FeePercent,
		WalkForwardConfig: research.WalkForwardConfig{
			TrainMonths:    req.TrainMonths,
			TestMonths:     req.TestMonths,
			StepMonths:     req.StepMonths,
			HoldoutPercent: req.HoldoutPercent,
		},
	}

	// Use a timeout for walk-forward execution (longer than simple backtest)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()

	result, err := s.walkForwardEngine.RunWalkForward(ctx, walkForwardReq)
	if err != nil {
		// Check if error is due to context timeout/cancellation
		if ctx.Err() == context.DeadlineExceeded {
			errorResponse(c, http.StatusGatewayTimeout, "Walk-forward backtest timed out after 10 minutes - try reducing date range or number of coins")
			return
		}
		if ctx.Err() == context.Canceled {
			errorResponse(c, http.StatusRequestTimeout, "Walk-forward request was cancelled")
			return
		}
		errorResponse(c, http.StatusBadRequest, "Walk-forward backtest failed: "+err.Error())
		return
	}

	successResponse(c, result)
}

// handleGetResearchFeatures returns the list of available features for backtesting.
// GET /api/research/features
func (s *Server) handleGetResearchFeatures(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	// Get all available features
	features := research.GetAvailableFeatures()

	// Organize features by category
	categories := FeatureCategoryResponse{
		Price: []string{
			"return_1c", "return_3c", "return_5c", "return_10c", "return_20c",
			"body_ratio", "upper_wick_ratio", "lower_wick_ratio", "position_in_range",
			"gap_percent", "range_percent", "consecutive_up", "consecutive_down",
			"higher_high", "lower_low",
		},
		Volume: []string{
			"volume_ratio_5", "volume_ratio_10", "volume_ratio_20", "volume_trend_5",
			"taker_buy_ratio", "trade_count_value", "avg_trade_size", "obv", "obv_trend",
			"volume_price_trend", "accum_dist", "money_flow",
		},
		Volatility: []string{
			"atr_7", "atr_14", "atr_21", "atr_percent",
			"bollinger_upper", "bollinger_lower", "bollinger_width", "bollinger_position",
			"range_expansion", "volatility_ratio",
		},
		Momentum: []string{
			"rsi_7", "rsi_14", "rsi_21", "stoch_k", "stoch_d",
			"macd_line", "macd_signal", "macd_histogram",
			"roc_5", "roc_10", "roc_20", "momentum_5", "momentum_10", "williams_r",
		},
		Trend: []string{
			"sma_10", "sma_20", "sma_50", "ema_9", "ema_21", "price_vs_sma_20",
			"sma_cross", "adx", "plus_di", "minus_di", "trend_strength", "trend_direction",
		},
		Time: []string{
			"hour_of_day", "day_of_week", "is_weekend", "day_of_month", "is_month_end",
		},
	}

	// Available operators
	operators := []string{"<", "<=", ">", ">=", "==", "!="}

	response := FeaturesResponse{
		Features:   features,
		Count:      len(features),
		Categories: categories,
		Operators:  operators,
	}

	successResponse(c, response)
}
