package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Epic 14: Coin Profiler API Endpoints
// Story 14.6: API Endpoints for Coin Profiler
// ============================================================================
// This file provides REST API endpoints for the Coin Profiler service.
// The Coin Profiler collects real-time WebSocket data for the Chain Trading System.
//
// Endpoints:
// - GET  /api/futures/coin-profiler/status       - Get profiler status
// - GET  /api/futures/coin-profiler/coins        - Get all tracked coins
// - GET  /api/futures/coin-profiler/coins/:symbol - Get specific coin data
// - GET  /api/futures/coin-profiler/requirements - Get aggregated requirements
// - POST /api/futures/coin-profiler/start        - Start profiler
// - POST /api/futures/coin-profiler/stop         - Stop profiler
// ============================================================================

// handleGetCoinProfilerStatus returns the current status of the Coin Profiler.
// GET /api/futures/coin-profiler/status
func (s *Server) handleGetCoinProfilerStatus(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	status := s.userAutopilotManager.GetCoinProfilerStatus(userID)
	c.JSON(http.StatusOK, status)
}

// handleGetCoinProfilerCoins returns all tracked coins with current data.
// GET /api/futures/coin-profiler/coins
func (s *Server) handleGetCoinProfilerCoins(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	profiler := s.userAutopilotManager.GetCoinProfiler(userID)
	if profiler == nil {
		c.JSON(http.StatusOK, gin.H{
			"coins": []interface{}{},
			"total": 0,
		})
		return
	}

	allCoinData := profiler.GetAllCoinData()

	// Convert map to slice for JSON response
	coins := make([]interface{}, 0, len(allCoinData))
	for _, coin := range allCoinData {
		coins = append(coins, coin)
	}

	c.JSON(http.StatusOK, gin.H{
		"coins": coins,
		"total": len(coins),
	})
}

// handleGetCoinProfilerCoin returns data for a specific coin/symbol.
// GET /api/futures/coin-profiler/coins/:symbol
func (s *Server) handleGetCoinProfilerCoin(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_REQUEST",
			"message": "Symbol parameter is required",
		})
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	profiler := s.userAutopilotManager.GetCoinProfiler(userID)
	if profiler == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "NOT_FOUND",
			"message": "Coin profiler not initialized",
		})
		return
	}

	coinData := profiler.GetCoinData(symbol)
	if coinData == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "NOT_FOUND",
			"message": "No data found for symbol: " + symbol,
		})
		return
	}

	c.JSON(http.StatusOK, coinData)
}

// handleGetCoinProfilerRequirements returns the current aggregated data requirements.
// This shows what timeframes and data fields are needed based on enabled strategies.
// GET /api/futures/coin-profiler/requirements
func (s *Server) handleGetCoinProfilerRequirements(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	profiler := s.userAutopilotManager.GetCoinProfiler(userID)
	if profiler == nil {
		c.JSON(http.StatusOK, gin.H{
			"all_timeframes": []string{},
			"all_data_fields": []string{},
			"total_strategies": 0,
			"subscriptions": map[string]interface{}{},
		})
		return
	}

	// Get current subscriptions as a proxy for requirements
	subscriptions := profiler.GetSubscriptions()

	// Build timeframe and data field sets from subscriptions
	timeframeSet := make(map[string]bool)
	for _, sub := range subscriptions {
		for _, tf := range sub.Timeframes {
			timeframeSet[tf] = true
		}
	}

	// Convert set to sorted slice
	timeframes := make([]string, 0, len(timeframeSet))
	for tf := range timeframeSet {
		timeframes = append(timeframes, tf)
	}

	// Convert subscriptions map to JSON-friendly format
	subsMap := make(map[string]interface{})
	for symbol, sub := range subscriptions {
		subsMap[symbol] = map[string]interface{}{
			"timeframes": sub.Timeframes,
			"source":     sub.Source,
			"strategy":   sub.Strategy,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"all_timeframes":   timeframes,
		"all_data_fields":  []string{"ohlc", "volume", "taker_buy_volume"}, // Standard fields
		"total_strategies": len(subscriptions),
		"subscriptions":    subsMap,
	})
}

// handleStartCoinProfiler starts the Coin Profiler for the authenticated user.
// POST /api/futures/coin-profiler/start
func (s *Server) handleStartCoinProfiler(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	if err := s.userAutopilotManager.StartCoinProfiler(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "START_FAILED",
			"message": err.Error(),
		})
		return
	}

	status := s.userAutopilotManager.GetCoinProfilerStatus(userID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Coin profiler started",
		"status":  status,
	})
}

// handleStopCoinProfiler stops the Coin Profiler for the authenticated user.
// POST /api/futures/coin-profiler/stop
func (s *Server) handleStopCoinProfiler(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	if err := s.userAutopilotManager.StopCoinProfiler(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "STOP_FAILED",
			"message": err.Error(),
		})
		return
	}

	status := s.userAutopilotManager.GetCoinProfilerStatus(userID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Coin profiler stopped",
		"status":  status,
	})
}
