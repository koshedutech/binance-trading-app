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
// This shows what timeframes and data fields are needed based on enabled strategies and positions.
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
			"all_timeframes":    []string{},
			"all_data_fields":   []string{},
			"all_symbols":       []string{},
			"strategy_count":    0,
			"position_count":    0,
			"from_strategies":   []interface{}{},
			"from_positions":    []interface{}{},
			"subscriptions":     map[string]interface{}{},
		})
		return
	}

	// Get combined requirements which has detailed strategy and position info
	combined := profiler.GetCombinedRequirements()

	// Build response with detailed breakdown
	response := gin.H{
		"all_timeframes":  []string{},
		"all_data_fields": []string{"ohlc", "volume", "taker_buy_volume"},
		"all_symbols":     []string{},
		"strategy_count":  0,
		"position_count":  0,
		"from_strategies": []interface{}{},
		"from_positions":  []interface{}{},
		"subscriptions":   map[string]interface{}{},
	}

	if combined != nil {
		response["all_timeframes"] = combined.AllTimeframes
		response["all_data_fields"] = combined.AllDataFields
		response["all_symbols"] = combined.AllSymbols
		response["strategy_count"] = combined.StrategyCount
		response["position_count"] = combined.PositionCount

		// Extract strategy sources (symbols tracked for entry decisions)
		fromStrategies := []interface{}{}
		fromPositions := []interface{}{}

		for _, symReq := range combined.BySymbol {
			if symReq.Source == "strategy" || symReq.Source == "both" {
				// Build strategy info
				stratInfo := map[string]interface{}{
					"symbol":     symReq.Symbol,
					"timeframes": symReq.Timeframes,
					"strategies": symReq.Strategies,
				}
				fromStrategies = append(fromStrategies, stratInfo)
			}

			if symReq.Source == "position" || symReq.Source == "both" {
				// Build position info
				for _, pos := range symReq.Positions {
					posInfo := map[string]interface{}{
						"symbol":     pos.Symbol,
						"mode":       pos.Mode,
						"side":       pos.Side,
						"exit_mode":  pos.ExitMode,
						"timeframes": pos.Timeframes,
					}
					fromPositions = append(fromPositions, posInfo)
				}
			}
		}

		response["from_strategies"] = fromStrategies
		response["from_positions"] = fromPositions

		// Build subscriptions map
		subsMap := make(map[string]interface{})
		for symbol, symReq := range combined.BySymbol {
			subsMap[symbol] = map[string]interface{}{
				"timeframes":  symReq.Timeframes,
				"data_fields": symReq.DataFields,
				"source":      symReq.Source,
				"strategies":  symReq.Strategies,
				"positions":   symReq.Positions,
			}
		}
		response["subscriptions"] = subsMap
	} else {
		// Fallback to subscriptions if combined requirements not available
		subscriptions := profiler.GetSubscriptions()
		subsMap := make(map[string]interface{})
		timeframeSet := make(map[string]bool)

		for symbol, sub := range subscriptions {
			for _, tf := range sub.Timeframes {
				timeframeSet[tf] = true
			}
			subsMap[symbol] = map[string]interface{}{
				"timeframes": sub.Timeframes,
				"source":     sub.Source,
			}
		}

		timeframes := make([]string, 0, len(timeframeSet))
		for tf := range timeframeSet {
			timeframes = append(timeframes, tf)
		}
		response["all_timeframes"] = timeframes
		response["subscriptions"] = subsMap
	}

	c.JSON(http.StatusOK, response)
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
