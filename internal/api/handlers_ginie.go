package api

import (
	"binance-trading-bot/internal/autopilot"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

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

	// CRITICAL: Check if dry_run mode is being updated
	// ALWAYS use centralized SetDryRunMode() when dry_run is in updates
	// because the main config and Ginie config may be out of sync
	var dryRunUpdated bool
	var newDryRunValue bool
	if v, ok := updates["dry_run"].(bool); ok {
		dryRunUpdated = true
		newDryRunValue = v
		fmt.Printf("[GINIE-MODE] Dry run update requested: %v (will always sync to main config)\n", v)
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

	// CRITICAL: If dry_run is being updated, use centralized SetDryRunMode() to ensure consistency
	if dryRunUpdated {
		fmt.Printf("[GINIE-MODE] Syncing dry_run to main config: %v\n", newDryRunValue)
		settingsAPI := s.getSettingsAPI()
		if settingsAPI != nil {
			fmt.Printf("[GINIE-MODE] Using centralized SetDryRunMode() method\n")
			if err := settingsAPI.SetDryRunMode(newDryRunValue); err != nil {
				fmt.Printf("[GINIE-MODE] ERROR: SetDryRunMode failed: %v\n", err)
				errorResponse(c, http.StatusInternalServerError, "Failed to sync trading mode: "+err.Error())
				return
			}
			fmt.Printf("[GINIE-MODE] Mode synced successfully via settingsAPI\n")
		} else {
			// Fallback: if getSettingsAPI() is nil, directly update FuturesController
			fmt.Printf("[GINIE-MODE] WARNING: getSettingsAPI() returned nil, using fallback method\n")
			controller.SetDryRun(newDryRunValue)
			fmt.Printf("[GINIE-MODE] Called SetDryRun directly\n")

			// Also persist to settings file
			sm := autopilot.GetSettingsManager()
			if err := sm.UpdateDryRunMode(newDryRunValue); err != nil {
				fmt.Printf("[GINIE-MODE] ERROR: Failed to persist mode to settings: %v\n", err)
			} else {
				fmt.Printf("[GINIE-MODE] Mode synced successfully via fallback\n")
			}
		}
	} else {
		// If dry_run didn't change, just persist other Ginie-specific settings
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

// ==================== Per-Position ROI Target Handlers ====================

// handleSetPositionROITarget sets custom ROI% target for a specific position
func (s *Server) handleSetPositionROITarget(c *gin.Context) {
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

	var req struct {
		ROIPercent    float64 `json:"roi_percent"`      // Custom ROI% (0.1-1000)
		SaveForFuture bool    `json:"save_for_future"`  // If true, save to SymbolSettings
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validation
	if req.ROIPercent < 0 || req.ROIPercent > 1000 {
		errorResponse(c, http.StatusBadRequest, "ROI percent must be between 0-1000%")
		return
	}

	// Get the position
	positions := giniePilot.GetPositions()
	var targetPos *autopilot.GiniePosition
	for _, pos := range positions {
		if pos.Symbol == symbol {
			targetPos = pos
			break
		}
	}

	if targetPos == nil {
		errorResponse(c, http.StatusNotFound, fmt.Sprintf("Position not found: %s", symbol))
		return
	}

	// Set custom ROI on position (in-memory)
	if req.ROIPercent > 0 {
		targetPos.CustomROIPercent = &req.ROIPercent
	} else {
		targetPos.CustomROIPercent = nil // Clear custom ROI
	}

	// Optionally save to SymbolSettings for future positions
	if req.SaveForFuture {
		settingsManager := autopilot.GetSettingsManager()
		if err := settingsManager.UpdateSymbolROITarget(symbol, req.ROIPercent); err != nil {
			logger.Printf("[API] Failed to save symbol ROI target: %v", err)
			errorResponse(c, http.StatusInternalServerError, "Failed to save symbol ROI target")
			return
		}
		logger.Printf("[API] Saved custom ROI %.2f%% for symbol %s to settings", req.ROIPercent, symbol)
	}

	logger.Printf("[API] Set custom ROI %.2f%% for position %s (save_for_future=%v)",
		req.ROIPercent, symbol, req.SaveForFuture)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         fmt.Sprintf("Custom ROI target set for %s", symbol),
		"symbol":          symbol,
		"roi_percent":     req.ROIPercent,
		"save_for_future": req.SaveForFuture,
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

// handleGetGinieTrendTimeframes returns current trend timeframe configuration
func (s *Server) handleGetGinieTrendTimeframes(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"timeframes": gin.H{
			"ultrafast":          settings.GinieTrendTimeframeUltrafast,
			"scalp":              settings.GinieTrendTimeframeScalp,
			"swing":              settings.GinieTrendTimeframeSwing,
			"position":           settings.GinieTrendTimeframePosition,
			"block_on_divergence": settings.GinieBlockOnDivergence,
		},
		"valid_timeframes": []string{
			"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w", "1M",
		},
	})
}

// handleUpdateGinieTrendTimeframes updates trend timeframe configuration
func (s *Server) handleUpdateGinieTrendTimeframes(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	var req struct {
		UltrafastTimeframe string `json:"ultrafast_timeframe"`
		ScalpTimeframe    string `json:"scalp_timeframe"`
		SwingTimeframe    string `json:"swing_timeframe"`
		PositionTimeframe string `json:"position_timeframe"`
		BlockOnDivergence *bool  `json:"block_on_divergence"` // Pointer to detect if provided
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate timeframes if provided
	if req.UltrafastTimeframe != "" {
		if err := autopilot.ValidateTimeframe(req.UltrafastTimeframe); err != nil {
			errorResponse(c, http.StatusBadRequest, "Invalid ultrafast timeframe: "+err.Error())
			return
		}
	}
	if req.ScalpTimeframe != "" {
		if err := autopilot.ValidateTimeframe(req.ScalpTimeframe); err != nil {
			errorResponse(c, http.StatusBadRequest, "Invalid scalp timeframe: "+err.Error())
			return
		}
	}
	if req.SwingTimeframe != "" {
		if err := autopilot.ValidateTimeframe(req.SwingTimeframe); err != nil {
			errorResponse(c, http.StatusBadRequest, "Invalid swing timeframe: "+err.Error())
			return
		}
	}
	if req.PositionTimeframe != "" {
		if err := autopilot.ValidateTimeframe(req.PositionTimeframe); err != nil {
			errorResponse(c, http.StatusBadRequest, "Invalid position timeframe: "+err.Error())
			return
		}
	}

	// Update settings
	sm := autopilot.GetSettingsManager()
	blockOnDiv := false
	if req.BlockOnDivergence != nil {
		blockOnDiv = *req.BlockOnDivergence
	} else {
		blockOnDiv = sm.GetCurrentSettings().GinieBlockOnDivergence
	}

	if err := sm.UpdateGinieTrendTimeframes(
		req.UltrafastTimeframe,
		req.ScalpTimeframe,
		req.SwingTimeframe,
		req.PositionTimeframe,
		blockOnDiv,
	); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update timeframes: "+err.Error())
		return
	}

	// Refresh GinieAnalyzer settings to pick up changes immediately
	ginie := controller.GetGinieAnalyzer()
	if ginie != nil {
		ginie.RefreshSettings()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Trend timeframes updated successfully",
		"timeframes": gin.H{
			"scalp":              sm.GetCurrentSettings().GinieTrendTimeframeScalp,
			"swing":              sm.GetCurrentSettings().GinieTrendTimeframeSwing,
			"position":           sm.GetCurrentSettings().GinieTrendTimeframePosition,
			"block_on_divergence": sm.GetCurrentSettings().GinieBlockOnDivergence,
		},
	})
}

// handleGetGinieSLTPConfig returns current SL/TP configuration
func (s *Server) handleGetGinieSLTPConfig(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"sltp_config": gin.H{
			"ultrafast": gin.H{
				"sl_percent":             settings.GinieSLPercentUltrafast,
				"tp_percent":             settings.GinieTPPercentUltrafast,
				"trailing_enabled":       settings.GinieTrailingStopEnabledUltrafast,
				"trailing_percent":       settings.GinieTrailingStopPercentUltrafast,
				"trailing_activation":    settings.GinieTrailingStopActivationUltrafast,
			},
			"scalp": gin.H{
				"sl_percent":             settings.GinieSLPercentScalp,
				"tp_percent":             settings.GinieTPPercentScalp,
				"trailing_enabled":       settings.GinieTrailingStopEnabledScalp,
				"trailing_percent":       settings.GinieTrailingStopPercentScalp,
				"trailing_activation":    settings.GinieTrailingStopActivationScalp,
			},
			"swing": gin.H{
				"sl_percent":             settings.GinieSLPercentSwing,
				"tp_percent":             settings.GinieTPPercentSwing,
				"trailing_enabled":       settings.GinieTrailingStopEnabledSwing,
				"trailing_percent":       settings.GinieTrailingStopPercentSwing,
				"trailing_activation":    settings.GinieTrailingStopActivationSwing,
			},
			"position": gin.H{
				"sl_percent":             settings.GinieSLPercentPosition,
				"tp_percent":             settings.GinieTPPercentPosition,
				"trailing_enabled":       settings.GinieTrailingStopEnabledPosition,
				"trailing_percent":       settings.GinieTrailingStopPercentPosition,
				"trailing_activation":    settings.GinieTrailingStopActivationPosition,
			},
		},
		"tp_mode": gin.H{
			"use_single_tp":     settings.GinieUseSingleTP,
			"single_tp_percent": settings.GinieSingleTPPercent,
			"tp1_percent":       settings.GinieTP1Percent,
			"tp2_percent":       settings.GinieTP2Percent,
			"tp3_percent":       settings.GinieTP3Percent,
			"tp4_percent":       settings.GinieTP4Percent,
		},
	})
}

// handleUpdateGinieSLTP updates SL/TP configuration for a mode
func (s *Server) handleUpdateGinieSLTP(c *gin.Context) {
	mode := c.Param("mode") // scalp, swing, or position

	var req struct {
		SLPercent           *float64 `json:"sl_percent"`
		TPPercent           *float64 `json:"tp_percent"`
		TrailingEnabled     *bool    `json:"trailing_enabled"`
		TrailingPercent     *float64 `json:"trailing_percent"`
		TrailingActivation  *float64 `json:"trailing_activation"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	// Get current values as defaults
	var slPct, tpPct, trailPct, trailAct float64
	var trailEnabled bool

	switch mode {
	case "ultrafast":
		slPct = settings.GinieSLPercentUltrafast
		tpPct = settings.GinieTPPercentUltrafast
		trailEnabled = settings.GinieTrailingStopEnabledUltrafast
		trailPct = settings.GinieTrailingStopPercentUltrafast
		trailAct = settings.GinieTrailingStopActivationUltrafast
	case "scalp":
		slPct = settings.GinieSLPercentScalp
		tpPct = settings.GinieTPPercentScalp
		trailEnabled = settings.GinieTrailingStopEnabledScalp
		trailPct = settings.GinieTrailingStopPercentScalp
		trailAct = settings.GinieTrailingStopActivationScalp
	case "swing":
		slPct = settings.GinieSLPercentSwing
		tpPct = settings.GinieTPPercentSwing
		trailEnabled = settings.GinieTrailingStopEnabledSwing
		trailPct = settings.GinieTrailingStopPercentSwing
		trailAct = settings.GinieTrailingStopActivationSwing
	case "position":
		slPct = settings.GinieSLPercentPosition
		tpPct = settings.GinieTPPercentPosition
		trailEnabled = settings.GinieTrailingStopEnabledPosition
		trailPct = settings.GinieTrailingStopPercentPosition
		trailAct = settings.GinieTrailingStopActivationPosition
	default:
		errorResponse(c, http.StatusBadRequest, "Invalid mode: must be ultrafast, scalp, swing, or position")
		return
	}

	// Update with provided values
	if req.SLPercent != nil {
		slPct = *req.SLPercent
	}
	if req.TPPercent != nil {
		tpPct = *req.TPPercent
	}
	if req.TrailingEnabled != nil {
		trailEnabled = *req.TrailingEnabled
	}
	if req.TrailingPercent != nil {
		trailPct = *req.TrailingPercent
	}
	if req.TrailingActivation != nil {
		trailAct = *req.TrailingActivation
	}

	// Update settings
	if err := sm.UpdateGinieSLTPSettings(mode, slPct, tpPct, trailEnabled, trailPct, trailAct); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("SL/TP config updated for %s mode", mode),
		"config": gin.H{
			"sl_percent":          slPct,
			"tp_percent":          tpPct,
			"trailing_enabled":    trailEnabled,
			"trailing_percent":    trailPct,
			"trailing_activation": trailAct,
		},
	})
}

// handleUpdateGinieTPMode updates TP mode (single vs multi)
func (s *Server) handleUpdateGinieTPMode(c *gin.Context) {
	var req struct {
		UseSingleTP     *bool    `json:"use_single_tp"`
		SingleTPPercent *float64 `json:"single_tp_percent"`
		TP1Percent      *float64 `json:"tp1_percent"`
		TP2Percent      *float64 `json:"tp2_percent"`
		TP3Percent      *float64 `json:"tp3_percent"`
		TP4Percent      *float64 `json:"tp4_percent"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	useSingle := settings.GinieUseSingleTP
	singlePct := settings.GinieSingleTPPercent
	tp1 := settings.GinieTP1Percent
	tp2 := settings.GinieTP2Percent
	tp3 := settings.GinieTP3Percent
	tp4 := settings.GinieTP4Percent

	if req.UseSingleTP != nil {
		useSingle = *req.UseSingleTP
	}
	if req.SingleTPPercent != nil {
		singlePct = *req.SingleTPPercent
	}
	if req.TP1Percent != nil {
		tp1 = *req.TP1Percent
	}
	if req.TP2Percent != nil {
		tp2 = *req.TP2Percent
	}
	if req.TP3Percent != nil {
		tp3 = *req.TP3Percent
	}
	if req.TP4Percent != nil {
		tp4 = *req.TP4Percent
	}

	if err := sm.UpdateGinieTPMode(useSingle, singlePct, tp1, tp2, tp3, tp4); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "TP mode updated successfully",
		"config": gin.H{
			"use_single_tp":     useSingle,
			"single_tp_percent": singlePct,
			"tp1_percent":       tp1,
			"tp2_percent":       tp2,
			"tp3_percent":       tp3,
			"tp4_percent":       tp4,
		},
	})
}

// parseIntParam is a helper to parse integer query parameters
func parseIntParam(s string) (int, error) {
	return strconv.Atoi(s)
}

// ====== ULTRAFAST SCALPING MODE ======

// handleGetUltraFastConfig returns current ultrafast scalping configuration
func (s *Server) handleGetUltraFastConfig(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"ultrafast_config": gin.H{
			"enabled":           settings.UltraFastEnabled,
			"scan_interval_ms":  settings.UltraFastScanInterval,
			"monitor_interval_ms": settings.UltraFastMonitorInterval,
			"max_positions":     settings.UltraFastMaxPositions,
			"max_usd_per_pos":   settings.UltraFastMaxUSDPerPos,
			"min_confidence":    settings.UltraFastMinConfidence,
			"min_profit_pct":    settings.UltraFastMinProfitPct,
			"max_hold_ms":       settings.UltraFastMaxHoldMS,
			"max_daily_trades":  settings.UltraFastMaxDailyTrades,
		},
		"ultrafast_stats": gin.H{
			"today_trades":  settings.UltraFastTodayTrades,
			"daily_pnl":     settings.UltraFastDailyPnL,
			"total_pnl":     settings.UltraFastTotalPnL,
			"win_rate":      settings.UltraFastWinRate,
			"last_update":   settings.UltraFastLastUpdate,
		},
	})
}

// handleUpdateUltraFastConfig updates ultrafast scalping configuration
func (s *Server) handleUpdateUltraFastConfig(c *gin.Context) {
	var req struct {
		Enabled          *bool    `json:"enabled"`
		ScanIntervalMs   *int     `json:"scan_interval_ms"`
		MonitorIntervalMs *int    `json:"monitor_interval_ms"`
		MaxPositions     *int     `json:"max_positions"`
		MaxUSDPerPos     *float64 `json:"max_usd_per_pos"`
		MinConfidence    *float64 `json:"min_confidence"`
		MinProfitPct     *float64 `json:"min_profit_pct"`
		MaxHoldMs        *int     `json:"max_hold_ms"`
		MaxDailyTrades   *int     `json:"max_daily_trades"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	// Update with provided values
	if req.Enabled != nil {
		settings.UltraFastEnabled = *req.Enabled
	}
	if req.ScanIntervalMs != nil {
		if *req.ScanIntervalMs < 1000 || *req.ScanIntervalMs > 30000 {
			errorResponse(c, http.StatusBadRequest, "Scan interval must be between 1000-30000ms")
			return
		}
		settings.UltraFastScanInterval = *req.ScanIntervalMs
	}
	if req.MonitorIntervalMs != nil {
		if *req.MonitorIntervalMs < 100 || *req.MonitorIntervalMs > 5000 {
			errorResponse(c, http.StatusBadRequest, "Monitor interval must be between 100-5000ms")
			return
		}
		settings.UltraFastMonitorInterval = *req.MonitorIntervalMs
	}
	if req.MaxPositions != nil {
		if *req.MaxPositions < 1 || *req.MaxPositions > 10 {
			errorResponse(c, http.StatusBadRequest, "Max positions must be between 1-10")
			return
		}
		settings.UltraFastMaxPositions = *req.MaxPositions
	}
	if req.MaxUSDPerPos != nil {
		if *req.MaxUSDPerPos < 50 || *req.MaxUSDPerPos > 5000 {
			errorResponse(c, http.StatusBadRequest, "Max USD per position must be between $50-$5000")
			return
		}
		settings.UltraFastMaxUSDPerPos = *req.MaxUSDPerPos
	}
	if req.MinConfidence != nil {
		if *req.MinConfidence < 10 || *req.MinConfidence > 99 {
			errorResponse(c, http.StatusBadRequest, "Min confidence must be between 10-99%")
			return
		}
		settings.UltraFastMinConfidence = *req.MinConfidence
	}
	if req.MinProfitPct != nil {
		if *req.MinProfitPct < 0 || *req.MinProfitPct > 5 {
			errorResponse(c, http.StatusBadRequest, "Min profit must be between 0-5%")
			return
		}
		settings.UltraFastMinProfitPct = *req.MinProfitPct
	}
	if req.MaxHoldMs != nil {
		if *req.MaxHoldMs < 500 || *req.MaxHoldMs > 60000 {
			errorResponse(c, http.StatusBadRequest, "Max hold time must be between 500-60000ms")
			return
		}
		settings.UltraFastMaxHoldMS = *req.MaxHoldMs
	}
	if req.MaxDailyTrades != nil {
		if *req.MaxDailyTrades < 10 || *req.MaxDailyTrades > 500 {
			errorResponse(c, http.StatusBadRequest, "Max daily trades must be between 10-500")
			return
		}
		settings.UltraFastMaxDailyTrades = *req.MaxDailyTrades
	}

	if err := sm.SaveSettings(settings); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update ultrafast config: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ultrafast scalping configuration updated",
		"config": gin.H{
			"enabled":           settings.UltraFastEnabled,
			"scan_interval_ms":  settings.UltraFastScanInterval,
			"monitor_interval_ms": settings.UltraFastMonitorInterval,
			"max_positions":     settings.UltraFastMaxPositions,
			"max_usd_per_pos":   settings.UltraFastMaxUSDPerPos,
			"min_confidence":    settings.UltraFastMinConfidence,
			"min_profit_pct":    settings.UltraFastMinProfitPct,
			"max_hold_ms":       settings.UltraFastMaxHoldMS,
			"max_daily_trades":  settings.UltraFastMaxDailyTrades,
		},
	})
}

// handleToggleUltraFast toggles ultrafast mode on/off
func (s *Server) handleToggleUltraFast(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()
	settings.UltraFastEnabled = req.Enabled

	if err := sm.SaveSettings(settings); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to toggle ultrafast: "+err.Error())
		return
	}

	status := "disabled"
	if req.Enabled {
		status = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Ultrafast scalping mode %s", status),
		"enabled": req.Enabled,
	})
}

// handleResetUltraFastStats resets daily ultrafast statistics
func (s *Server) handleResetUltraFastStats(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	// Reset daily stats
	settings.UltraFastTodayTrades = 0
	settings.UltraFastDailyPnL = 0
	settings.UltraFastLastUpdate = time.Now().Format("2006-01-02")

	if err := sm.SaveSettings(settings); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to reset stats: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ultrafast daily statistics reset",
		"stats": gin.H{
			"today_trades": 0,
			"daily_pnl":    0,
		},
	})
}

// handleGetGinieTradeHistoryWithDateRange returns trade history filtered by date range
func (s *Server) handleGetGinieTradeHistoryWithDateRange(c *gin.Context) {
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

	// Parse query parameters for date range
	startStr := c.Query("start")
	endStr := c.Query("end")

	var startTime, endTime time.Time
	var err error

	// Parse start date if provided (format: 2025-12-24)
	if startStr != "" {
		startTime, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "Invalid start date format. Use YYYY-MM-DD: "+err.Error())
			return
		}
	}

	// Parse end date if provided (format: 2025-12-24)
	if endStr != "" {
		endTime, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "Invalid end date format. Use YYYY-MM-DD: "+err.Error())
			return
		}
		// Include entire end date by moving to start of next day
		endTime = endTime.Add(24 * time.Hour)
	}

	// Get trades in date range
	var trades []interface{}
	if startTime.IsZero() && endTime.IsZero() {
		// No date filter, return recent trades
		historyTrades := autopilot.GetTradeHistory(50)
		for _, trade := range historyTrades {
			trades = append(trades, trade)
		}
	} else {
		historyTrades := autopilot.GetTradeHistoryInDateRange(startTime, endTime)
		for _, trade := range historyTrades {
			trades = append(trades, trade)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"trades":  trades,
		"count":   len(trades),
		"filters": gin.H{
			"start_date": startStr,
			"end_date":   endStr,
		},
	})
}

// handleGetGiniePerformanceMetrics returns performance metrics with optional date filtering
func (s *Server) handleGetGiniePerformanceMetrics(c *gin.Context) {
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

	// Parse query parameters for date range
	startStr := c.Query("start")
	endStr := c.Query("end")

	var startTime, endTime time.Time
	var err error

	if startStr != "" {
		startTime, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "Invalid start date format. Use YYYY-MM-DD: "+err.Error())
			return
		}
	}

	if endStr != "" {
		endTime, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "Invalid end date format. Use YYYY-MM-DD: "+err.Error())
			return
		}
		endTime = endTime.Add(24 * time.Hour)
	}

	// Get trades in date range
	var trades interface{}
	if startTime.IsZero() && endTime.IsZero() {
		trades = autopilot.GetTradeHistory(1000)
	} else {
		trades = autopilot.GetTradeHistoryInDateRange(startTime, endTime)
	}

	// Return basic performance data
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"trades":  trades,
		"date_filters": gin.H{
			"start_date": startStr,
			"end_date":   endStr,
		},
	})
}

// handleGetGinieLLMDiagnostics returns LLM switch history
func (s *Server) handleGetGinieLLMDiagnostics(c *gin.Context) {
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

	// Get all LLM switches
	switches := autopilot.GetLLMSwitches(500)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"switches": switches,
		"count":   len(switches),
	})
}

// handleResetGinieLLMDiagnostics clears LLM diagnostic data
func (s *Server) handleResetGinieLLMDiagnostics(c *gin.Context) {
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

	// Clear LLM switch history
	autopilot.ClearLLMSwitches()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "LLM diagnostic data cleared",
	})
}
