package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"binance-trading-bot/internal/backtest"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/strategy"
)

// handleGetKlines returns historical klines for charting
// GET /api/binance/klines?symbol=BTCUSDT&interval=5m&limit=500
func (s *Server) handleGetKlines(c *gin.Context) {
	symbol := c.Query("symbol")
	interval := c.Query("interval")
	limitStr := c.DefaultQuery("limit", "500")

	if symbol == "" || interval == "" {
		errorResponse(c, http.StatusBadRequest, "symbol and interval are required")
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		errorResponse(c, http.StatusBadRequest, "limit must be between 1 and 1000")
		return
	}

	// Type assert GetClient() to *binance.Client
	client, ok := s.botAPI.GetClient().(*binance.Client)
	if !ok {
		errorResponse(c, http.StatusInternalServerError, "Failed to get client")
		return
	}

	klines, err := client.GetKlines(symbol, interval, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch klines: "+err.Error())
		return
	}

	successResponse(c, klines)
}

// handleRunBacktest executes a backtest for a visual strategy
// POST /api/strategy-configs/:id/backtest
// Body: {"symbol": "BTCUSDT", "interval": "5m", "start_date": "2024-01-01", "end_date": "2024-01-31"}
func (s *Server) handleRunBacktest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy config ID")
		return
	}

	var req struct {
		Symbol    string `json:"symbol" binding:"required"`
		Interval  string `json:"interval" binding:"required"`
		StartDate string `json:"start_date" binding:"required"`
		EndDate   string `json:"end_date" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid start_date format (use YYYY-MM-DD)")
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid end_date format (use YYYY-MM-DD)")
		return
	}

	// Get strategy config from database
	config, err := s.repo.GetStrategyConfigByID(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "Strategy config not found")
		return
	}

	// Check if config has visual_flow
	visualFlowData, ok := config.ConfigParams["visual_flow"].(map[string]interface{})
	if !ok {
		errorResponse(c, http.StatusBadRequest, "Strategy config does not contain a visual flow")
		return
	}

	// Create visual strategy
	visualStrat, err := strategy.NewVisualStrategy(config.Name, visualFlowData)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Failed to create visual strategy: "+err.Error())
		return
	}

	// Type assert GetClient() to *binance.Client
	client, ok := s.botAPI.GetClient().(*binance.Client)
	if !ok {
		errorResponse(c, http.StatusInternalServerError, "Failed to get client")
		return
	}

	// Create backtest
	backtester := backtest.NewBacktest(client, s.repo, visualStrat)

	// Run backtest
	result, trades, err := backtester.Run(c.Request.Context(), backtest.Config{
		StrategyConfigID: id,
		Symbol:           req.Symbol,
		Interval:         req.Interval,
		StartDate:        startDate,
		EndDate:          endDate,
		InitialBalance:   10000.0, // Default $10,000 initial balance
	})

	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Backtest failed: "+err.Error())
		return
	}

	// Save result to database
	resultID, err := s.repo.SaveBacktestResult(c.Request.Context(), result, trades)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save backtest result: "+err.Error())
		return
	}

	result.ID = resultID

	// Return result with trades
	successResponse(c, gin.H{
		"result": result,
		"trades": trades,
	})
}

// handleGetBacktestResults returns backtest results for a strategy config
// GET /api/strategy-configs/:id/backtest-results?limit=10
func (s *Server) handleGetBacktestResults(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy config ID")
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	results, err := s.repo.GetBacktestResults(c.Request.Context(), id, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch backtest results: "+err.Error())
		return
	}

	successResponse(c, results)
}

// handleGetBacktestTrades returns trades for a specific backtest result
// GET /api/backtest-results/:id/trades
func (s *Server) handleGetBacktestTrades(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid backtest result ID")
		return
	}

	trades, err := s.repo.GetBacktestTrades(c.Request.Context(), id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch backtest trades: "+err.Error())
		return
	}

	successResponse(c, trades)
}
