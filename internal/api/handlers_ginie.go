package api

import (
	"binance-trading-bot/internal/autopilot"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// handleGetGinieStatus returns current Ginie status
func (s *Server) handleGetGinieStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	status := ginie.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleGetGinieConfig returns Ginie configuration
func (s *Server) handleGetGinieConfig(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	config := ginie.GetConfig()
	c.JSON(http.StatusOK, config)
}

// handleUpdateGinieConfig updates Ginie configuration
func (s *Server) handleUpdateGinieConfig(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	var config autopilot.GinieConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	ginie.SetConfig(&config)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ginie configuration updated",
		"config":  ginie.GetConfig(),
	})
}

// handleToggleGinie enables or disables Ginie
func (s *Server) handleToggleGinie(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.Enabled {
		// Enable the analyzer
		ginie.Enable()
		// CRITICAL: Start the actual autopilot trading loop
		if err := controller.StartGinieAutopilot(); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to start autopilot: "+err.Error())
			return
		}
	} else {
		// CRITICAL: Stop the autopilot trading loop
		if err := controller.StopGinieAutopilot(); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to stop autopilot: "+err.Error())
			return
		}
		// Disable the analyzer
		ginie.Disable()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ginie toggled",
		"enabled": ginie.IsEnabled(),
	})
}

// handleGinieScanCoin scans a specific coin
func (s *Server) handleGinieScanCoin(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	symbol := c.Query("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	scan, err := ginie.ScanCoin(symbol)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to scan coin: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, scan)
}

// handleGinieGenerateDecision generates a trading decision for a symbol
func (s *Server) handleGinieGenerateDecision(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	symbol := c.Query("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	decision, err := ginie.GenerateDecision(symbol)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to generate decision: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, decision)
}

// handleGinieGetDecisions returns recent decisions
func (s *Server) handleGinieGetDecisions(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	decisions := ginie.GetRecentDecisions(20)

	c.JSON(http.StatusOK, gin.H{
		"decisions": decisions,
		"count":     len(decisions),
	})
}

// handleGinieScanAll scans all watched symbols
func (s *Server) handleGinieScanAll(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	status := ginie.GetStatus()
	results := make([]*autopilot.GinieCoinScan, 0)

	for _, symbol := range status.WatchedSymbols {
		scan, err := ginie.ScanCoin(symbol)
		if err != nil {
			continue
		}
		results = append(results, scan)
	}

	c.JSON(http.StatusOK, gin.H{
		"scans":   results,
		"count":   len(results),
		"symbols": status.WatchedSymbols,
	})
}

// handleGinieAnalyzeAll generates decisions for all watched symbols
func (s *Server) handleGinieAnalyzeAll(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	status := ginie.GetStatus()
	results := make([]*autopilot.GinieDecisionReport, 0)

	for _, symbol := range status.WatchedSymbols {
		decision, err := ginie.GenerateDecision(symbol)
		if err != nil {
			continue
		}
		results = append(results, decision)
	}

	// Find best opportunities
	var bestLong, bestShort *autopilot.GinieDecisionReport
	for _, d := range results {
		if d.Recommendation == autopilot.RecommendationExecute {
			if d.TradeExecution.Action == "LONG" {
				if bestLong == nil || d.ConfidenceScore > bestLong.ConfidenceScore {
					bestLong = d
				}
			} else if d.TradeExecution.Action == "SHORT" {
				if bestShort == nil || d.ConfidenceScore > bestShort.ConfidenceScore {
					bestShort = d
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"decisions":  results,
		"count":      len(results),
		"best_long":  bestLong,
		"best_short": bestShort,
	})
}

// ==================== Ginie Autopilot Handlers ====================

// handleGetGinieAutopilotStatus returns current Ginie autopilot status
func (s *Server) handleGetGinieAutopilotStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	autopilot := controller.GetGinieAutopilot()
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	stats := autopilot.GetStats()
	config := autopilot.GetConfig()
	positions := autopilot.GetPositions()
	history := autopilot.GetTradeHistory(20)
	blockedCoins := autopilot.GetBlockedCoins()

	// Get available balance for adaptive sizing info
	availableBalance, walletBalance := autopilot.GetBalanceInfo()

	c.JSON(http.StatusOK, gin.H{
		"stats":             stats,
		"config":            config,
		"positions":         positions,
		"trade_history":     history,
		"available_balance": availableBalance,
		"wallet_balance":    walletBalance,
		"blocked_coins":     blockedCoins,
	})
}

// handleGetGinieAutopilotConfig returns Ginie autopilot configuration
func (s *Server) handleGetGinieAutopilotConfig(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	autopilot := controller.GetGinieAutopilot()
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	config := autopilot.GetConfig()
	c.JSON(http.StatusOK, config)
}

// handleUpdateGinieAutopilotConfig updates Ginie autopilot configuration
func (s *Server) handleUpdateGinieAutopilotConfig(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Get current config and merge with incoming updates
	currentConfig := giniePilot.GetConfig()

	// Parse incoming updates as a map to detect which fields are provided
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Apply updates to current config
	if v, ok := updates["enabled"].(bool); ok {
		currentConfig.Enabled = v
	}
	if v, ok := updates["max_positions"].(float64); ok {
		currentConfig.MaxPositions = int(v)
	}
	if v, ok := updates["max_usd_per_position"].(float64); ok {
		currentConfig.MaxUSDPerPosition = v
	}
	if v, ok := updates["total_max_usd"].(float64); ok {
		currentConfig.TotalMaxUSD = v
	}
	if v, ok := updates["default_leverage"].(float64); ok {
		currentConfig.DefaultLeverage = int(v)
	}
	if v, ok := updates["dry_run"].(bool); ok {
		currentConfig.DryRun = v
	}
	if v, ok := updates["enable_scalp_mode"].(bool); ok {
		currentConfig.EnableScalpMode = v
	}
	if v, ok := updates["enable_swing_mode"].(bool); ok {
		currentConfig.EnableSwingMode = v
	}
	if v, ok := updates["enable_position_mode"].(bool); ok {
		currentConfig.EnablePositionMode = v
	}
	if v, ok := updates["tp1_percent"].(float64); ok {
		currentConfig.TP1Percent = v
	}
	if v, ok := updates["tp2_percent"].(float64); ok {
		currentConfig.TP2Percent = v
	}
	if v, ok := updates["tp3_percent"].(float64); ok {
		currentConfig.TP3Percent = v
	}
	if v, ok := updates["tp4_percent"].(float64); ok {
		currentConfig.TP4Percent = v
	}
	if v, ok := updates["move_to_breakeven_after_tp1"].(bool); ok {
		currentConfig.MoveToBreakevenAfterTP1 = v
	}
	if v, ok := updates["breakeven_buffer"].(float64); ok {
		currentConfig.BreakevenBuffer = v
	}
	if v, ok := updates["scalp_scan_interval"].(float64); ok {
		currentConfig.ScalpScanInterval = int(v)
	}
	if v, ok := updates["swing_scan_interval"].(float64); ok {
		currentConfig.SwingScanInterval = int(v)
	}
	if v, ok := updates["position_scan_interval"].(float64); ok {
		currentConfig.PositionScanInterval = int(v)
	}
	if v, ok := updates["min_confidence_to_trade"].(float64); ok {
		currentConfig.MinConfidenceToTrade = v
	}
	if v, ok := updates["max_daily_trades"].(float64); ok {
		currentConfig.MaxDailyTrades = int(v)
	}
	if v, ok := updates["max_daily_loss"].(float64); ok {
		currentConfig.MaxDailyLoss = v
	}
	// Circuit breaker config fields
	if v, ok := updates["circuit_breaker_enabled"].(bool); ok {
		currentConfig.CircuitBreakerEnabled = v
	}
	if v, ok := updates["cb_max_loss_per_hour"].(float64); ok {
		currentConfig.CBMaxLossPerHour = v
	}
	if v, ok := updates["cb_max_daily_loss"].(float64); ok {
		currentConfig.CBMaxDailyLoss = v
	}
	if v, ok := updates["cb_max_consecutive_losses"].(float64); ok {
		currentConfig.CBMaxConsecutiveLosses = int(v)
	}
	if v, ok := updates["cb_cooldown_minutes"].(float64); ok {
		currentConfig.CBCooldownMinutes = int(v)
	}

	giniePilot.SetConfig(currentConfig)

	// Persist Ginie settings to file so they survive restarts
	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateGinieSettings(
		currentConfig.RiskLevel,
		currentConfig.DryRun,
		currentConfig.MaxUSDPerPosition,
		currentConfig.DefaultLeverage,
		currentConfig.MinConfidenceToTrade,
		currentConfig.MaxPositions,
	); err != nil {
		log.Printf("Warning: Failed to persist Ginie settings: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ginie autopilot configuration updated",
		"config":  giniePilot.GetConfig(),
	})
}

// handleStartGinieAutopilot starts the Ginie autonomous trading
func (s *Server) handleStartGinieAutopilot(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Get Ginie autopilot to check its mode
	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Check mode for response message
	isLiveMode := !giniePilot.GetConfig().DryRun

	err := controller.StartGinieAutopilot()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to start Ginie autopilot: "+err.Error())
		return
	}

	// Persist auto-start setting so Ginie restarts automatically after server restart
	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateGinieAutoStart(true); err != nil {
		// Log but don't fail - the start was successful
		log.Printf("Failed to persist Ginie auto-start setting: %v", err)
	}

	modeStr := "PAPER"
	if isLiveMode {
		modeStr = "LIVE"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ginie autopilot started in " + modeStr + " mode (will auto-start on server restart)",
		"running": true,
		"mode":    modeStr,
	})
}

// handleStopGinieAutopilot stops the Ginie autonomous trading
func (s *Server) handleStopGinieAutopilot(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	err := controller.StopGinieAutopilot()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to stop Ginie autopilot: "+err.Error())
		return
	}

	// Clear auto-start setting so Ginie doesn't restart after server restart
	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateGinieAutoStart(false); err != nil {
		// Log but don't fail - the stop was successful
		log.Printf("Failed to clear Ginie auto-start setting: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ginie autopilot stopped (will NOT auto-start on server restart)",
		"running": false,
	})
}

// handleGetGinieAutopilotPositions returns active Ginie positions
func (s *Server) handleGetGinieAutopilotPositions(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	autopilot := controller.GetGinieAutopilot()
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	positions := autopilot.GetPositions()

	c.JSON(http.StatusOK, gin.H{
		"positions": positions,
		"count":     len(positions),
	})
}

// handleGetGinieAutopilotTradeHistory returns Ginie trade history
func (s *Server) handleGetGinieAutopilotTradeHistory(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	autopilot := controller.GetGinieAutopilot()
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	history := autopilot.GetTradeHistory(50)

	c.JSON(http.StatusOK, gin.H{
		"trades": history,
		"count":  len(history),
	})
}

// handleClearGinieAutopilotPositions clears all tracked positions
func (s *Server) handleClearGinieAutopilotPositions(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	autopilot := controller.GetGinieAutopilot()
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	autopilot.ClearPositions()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All positions and stats cleared",
	})
}

// handleRefreshGinieSymbols refreshes the watched symbols list
func (s *Server) handleRefreshGinieSymbols(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	count, err := ginie.RefreshSymbols()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"message":  "Using fallback symbol list",
			"count":    count,
			"symbols":  ginie.GetWatchSymbols(),
			"fallback": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Symbols refreshed from Binance",
		"count":    count,
		"symbols":  ginie.GetWatchSymbols(),
		"fallback": false,
	})
}

// ==================== Ginie Circuit Breaker Handlers ====================

// handleGetGinieCircuitBreakerStatus returns Ginie circuit breaker status
func (s *Server) handleGetGinieCircuitBreakerStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	status := giniePilot.GetCircuitBreakerStatus()
	c.JSON(http.StatusOK, status)
}

// handleResetGinieCircuitBreaker resets Ginie circuit breaker
func (s *Server) handleResetGinieCircuitBreaker(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	err := giniePilot.ResetCircuitBreaker()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to reset circuit breaker: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ginie circuit breaker reset successfully",
		"status":  giniePilot.GetCircuitBreakerStatus(),
	})
}

// handleToggleGinieCircuitBreaker enables or disables Ginie circuit breaker
func (s *Server) handleToggleGinieCircuitBreaker(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	giniePilot.SetCircuitBreakerEnabled(req.Enabled)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ginie circuit breaker toggled",
		"enabled": req.Enabled,
		"status":  giniePilot.GetCircuitBreakerStatus(),
	})
}

// handleUpdateGinieCircuitBreakerConfig updates Ginie circuit breaker config
func (s *Server) handleUpdateGinieCircuitBreakerConfig(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	var req struct {
		MaxLossPerHour       float64 `json:"max_loss_per_hour"`
		MaxDailyLoss         float64 `json:"max_daily_loss"`
		MaxConsecutiveLosses int     `json:"max_consecutive_losses"`
		CooldownMinutes      int     `json:"cooldown_minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate
	if req.MaxLossPerHour <= 0 {
		req.MaxLossPerHour = 100 // default
	}
	if req.MaxDailyLoss <= 0 {
		req.MaxDailyLoss = 300 // default
	}
	if req.MaxConsecutiveLosses <= 0 {
		req.MaxConsecutiveLosses = 3 // default
	}
	if req.CooldownMinutes <= 0 {
		req.CooldownMinutes = 30 // default
	}

	giniePilot.UpdateCircuitBreakerConfig(
		req.MaxLossPerHour,
		req.MaxDailyLoss,
		req.MaxConsecutiveLosses,
		req.CooldownMinutes,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ginie circuit breaker config updated",
		"status":  giniePilot.GetCircuitBreakerStatus(),
	})
}

// ==================== Ginie Panic Button Handler ====================

// handleSyncGiniePositions syncs Ginie positions with exchange
func (s *Server) handleSyncGiniePositions(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Get the current futures client based on mode (paper/live)
	futuresClient := s.getFuturesClient()

	// Force full resync: clear all positions and reimport from exchange
	// Pass the client directly to ensure we use the right one
	synced, err := giniePilot.ForceSyncWithExchange(futuresClient)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to sync positions: "+err.Error())
		return
	}

	// Return updated positions
	positions := giniePilot.GetPositions()

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "Synced positions with exchange",
		"synced_count":   synced,
		"total_positions": len(positions),
		"positions":      positions,
	})
}

// handleCloseAllGiniePositions closes all Ginie-managed positions (panic button)
func (s *Server) handleCloseAllGiniePositions(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	closedCount, totalPnL, err := giniePilot.CloseAllPositions()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to close positions: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          "All Ginie positions closed",
		"positions_closed": closedCount,
		"total_pnl":        totalPnL,
	})
}

// handleRecalculateAdaptiveSLTP applies adaptive SL/TP to all naked positions
func (s *Server) handleRecalculateAdaptiveSLTP(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Recalculate adaptive SL/TP for all positions
	updated, err := giniePilot.RecalculateAdaptiveSLTP()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to recalculate SL/TP: "+err.Error())
		return
	}

	// Return updated positions
	positions := giniePilot.GetPositions()

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"message":           "Adaptive SL/TP applied to positions",
		"positions_updated": updated,
		"positions":         positions,
	})
}

// ==================== Ginie Risk Level Handlers ====================

// handleGetGinieRiskLevel returns the current Ginie risk level
func (s *Server) handleGetGinieRiskLevel(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	config := giniePilot.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"risk_level":      giniePilot.GetRiskLevel(),
		"min_confidence":  config.MinConfidenceToTrade,
		"max_usd":         config.MaxUSDPerPosition,
		"leverage":        config.DefaultLeverage,
	})
}

// handleSetGinieRiskLevel updates the Ginie risk level
func (s *Server) handleSetGinieRiskLevel(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	var req struct {
		RiskLevel string `json:"risk_level" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Set the risk level
	if err := giniePilot.SetRiskLevel(req.RiskLevel); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Persist to settings
	settingsManager := autopilot.GetSettingsManager()
	if err := settingsManager.UpdateGinieRiskLevel(req.RiskLevel); err != nil {
		// Log but don't fail - the in-memory change was successful
		log.Printf("Warning: Failed to persist Ginie risk level setting: %v", err)
	}

	config := giniePilot.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "Ginie risk level updated",
		"risk_level":      req.RiskLevel,
		"min_confidence":  config.MinConfidenceToTrade,
		"max_usd":         config.MaxUSDPerPosition,
		"leverage":        config.DefaultLeverage,
	})
}

// ==================== Ginie Market Movers Handlers ====================

// handleGetMarketMovers returns current market movers (gainers, losers, volume, volatility)
func (s *Server) handleGetMarketMovers(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	// Get topN from query param, default to 20
	topN := 20
	if v := c.Query("top"); v != "" {
		if n, err := parseIntParam(v); err == nil && n > 0 {
			topN = n
		}
	}

	movers, err := ginie.GetMarketMovers(topN)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to get market movers: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"top_n":           topN,
		"top_gainers":     movers.TopGainers,
		"top_losers":      movers.TopLosers,
		"top_volume":      movers.TopVolume,
		"high_volatility": movers.HighVolatility,
	})
}

// handleRefreshDynamicSymbols refreshes watch list using market movers
func (s *Server) handleRefreshDynamicSymbols(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAnalyzer()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie analyzer not initialized")
		return
	}

	// Get topN from JSON body or default to 15
	var req struct {
		TopN int `json:"top_n"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TopN <= 0 {
		req.TopN = 15
	}

	err := ginie.LoadDynamicSymbols(req.TopN)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load dynamic symbols: "+err.Error())
		return
	}

	symbols := ginie.GetWatchSymbols()

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Watch list updated with market movers",
		"top_n":         req.TopN,
		"symbol_count":  len(symbols),
		"symbols":       symbols,
	})
}

// ==================== Ginie Blocked Coins Handlers ====================

// handleGetGinieBlockedCoins returns list of blocked coins
func (s *Server) handleGetGinieBlockedCoins(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	blockedCoins := giniePilot.GetBlockedCoins()

	c.JSON(http.StatusOK, gin.H{
		"blocked_coins": blockedCoins,
		"count":         len(blockedCoins),
	})
}

// handleUnblockGinieCoin manually unblocks a specific coin
func (s *Server) handleUnblockGinieCoin(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	if err := giniePilot.UnblockCoin(symbol); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Coin unblocked successfully",
		"symbol":  symbol,
	})
}

// handleResetGinieCoinBlockHistory resets block history for a coin (allows auto-unblock again)
func (s *Server) handleResetGinieCoinBlockHistory(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	// Reset the block history so next block will auto-unblock again
	giniePilot.ResetCoinBlockHistory(symbol)

	// Also unblock if currently blocked
	giniePilot.UnblockCoin(symbol)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Coin block history reset and unblocked",
		"symbol":  symbol,
	})
}

// ==================== Ginie Signal Logs Handlers ====================

// handleGetGinieSignalLogs returns recent signal logs
func (s *Server) handleGetGinieSignalLogs(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Get limit from query param, default to 100
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := parseIntParam(v); err == nil && n > 0 {
			limit = n
		}
	}

	signals := giniePilot.GetSignalLogs(limit)
	stats := giniePilot.GetSignalStats()

	c.JSON(http.StatusOK, gin.H{
		"signals": signals,
		"count":   len(signals),
		"stats":   stats,
	})
}

// handleGetGinieSignalStats returns signal statistics
func (s *Server) handleGetGinieSignalStats(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	stats := giniePilot.GetSignalStats()
	c.JSON(http.StatusOK, stats)
}

// ==================== Ginie SL Update History Handlers ====================

// handleGetGinieSLHistory returns SL update history for all or specific symbol
func (s *Server) handleGetGinieSLHistory(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Check if specific symbol requested
	symbol := c.Query("symbol")
	if symbol != "" {
		history := giniePilot.GetSLUpdateHistory(symbol)
		if history == nil {
			c.JSON(http.StatusOK, gin.H{
				"symbol":  symbol,
				"history": nil,
				"message": "No SL update history for this symbol",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"symbol":  symbol,
			"history": history,
		})
		return
	}

	// Return all SL history
	allHistory := giniePilot.GetAllSLUpdateHistory()
	stats := giniePilot.GetSLUpdateStats()

	c.JSON(http.StatusOK, gin.H{
		"histories": allHistory,
		"count":     len(allHistory),
		"stats":     stats,
	})
}

// handleGetGinieSLStats returns SL update statistics
func (s *Server) handleGetGinieSLStats(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	stats := giniePilot.GetSLUpdateStats()
	c.JSON(http.StatusOK, stats)
}

// ==================== Ginie LLM SL Validation Handlers ====================

// handleGetGinieLLMSLStatus returns LLM SL kill switch status
func (s *Server) handleGetGinieLLMSLStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	status := giniePilot.GetLLMSLStatus()
	c.JSON(http.StatusOK, status)
}

// handleResetGinieLLMSL resets the LLM SL kill switch for a specific symbol
func (s *Server) handleResetGinieLLMSL(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	wasDisabled := giniePilot.ResetLLMSLForSymbol(symbol)

	message := "LLM SL kill switch reset for " + symbol
	if !wasDisabled {
		message = "LLM SL was not disabled for " + symbol + " (no action needed)"
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      message,
		"symbol":       symbol,
		"was_disabled": wasDisabled,
		"status":       giniePilot.GetLLMSLStatus(),
	})
}

// handleGetGinieDiagnostics returns comprehensive diagnostic info for troubleshooting
func (s *Server) handleGetGinieDiagnostics(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	giniePilot := controller.GetGinieAutopilot()
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	diagnostics := giniePilot.GetDiagnostics()
	c.JSON(http.StatusOK, diagnostics)
}

// handleGetSourcePerformance returns performance stats grouped by trade source (AI vs Strategy)
func (s *Server) handleGetSourcePerformance(c *gin.Context) {
	if s.repo == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	statsManager := autopilot.NewStrategyStatsManager(s.repo, nil)
	performance, err := statsManager.GetSourcePerformance(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to get source performance: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": performance,
	})
}

// handleGetPositionsBySource returns Ginie positions filtered by source (ai, strategy, or all)
func (s *Server) handleGetPositionsBySource(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	autopilotInst := controller.GetGinieAutopilot()
	if autopilotInst == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Get filter from query param
	source := c.DefaultQuery("source", "all")

	positions := autopilotInst.GetPositions()

	// Filter by source if specified
	if source != "all" && source != "" {
		filtered := make([]*autopilot.GiniePosition, 0)
		for _, pos := range positions {
			if pos.Source == source {
				filtered = append(filtered, pos)
			}
		}
		positions = filtered
	}

	c.JSON(http.StatusOK, gin.H{
		"positions": positions,
		"count":     len(positions),
		"filter":    source,
	})
}

// handleGetTradeHistoryBySource returns Ginie trade history filtered by source
func (s *Server) handleGetTradeHistoryBySource(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	autopilotInst := controller.GetGinieAutopilot()
	if autopilotInst == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Get filter from query param
	source := c.DefaultQuery("source", "all")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	history := autopilotInst.GetTradeHistory(limit * 2) // Get more to filter

	// Filter by source if specified
	if source != "all" && source != "" {
		filtered := make([]autopilot.GinieTradeResult, 0)
		for _, trade := range history {
			if trade.Source == source {
				filtered = append(filtered, trade)
				if len(filtered) >= limit {
					break
				}
			}
		}
		history = filtered
	} else if len(history) > limit {
		history = history[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"trades": history,
		"count":  len(history),
		"filter": source,
	})
}

// parseIntParam is a helper to parse integer query parameters
func parseIntParam(s string) (int, error) {
	return strconv.Atoi(s)
}
