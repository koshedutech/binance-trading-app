package api

import (
	"binance-trading-bot/internal/auth"
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGetGinieStatus returns current Ginie status
// NOTE: This returns status from the user-specific GinieAutopilot via the shared GinieAnalyzer.
// The diagnostics endpoint should be used for user-specific LLM status.
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

	// CRITICAL FIX: Override with per-user GinieAutopilot's state and stats
	// Multi-user isolation: status must show user-specific data, not shared analyzer data
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot != nil {
		status.Enabled = giniePilot.IsRunning()

		// Override stats with per-user data from GinieAutopilot
		userStats := giniePilot.GetStats()
		if winRate, ok := userStats["win_rate"].(float64); ok {
			status.WinRate = winRate
		}
		if dailyPnL, ok := userStats["daily_pnl"].(float64); ok {
			status.DailyPnL = dailyPnL
		}
		if dailyTrades, ok := userStats["daily_trades"].(int); ok {
			status.DailyTrades = dailyTrades
		}
		if activePos, ok := userStats["active_positions"].(int); ok {
			status.ActivePositions = activePos
		}
	}

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
	userID := s.getUserID(c)
	ctx := c.Request.Context()

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// CRITICAL FIX: Persist the auto-start setting so Ginie restarts after server reboot
	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateGinieAutoStart(req.Enabled); err != nil {
		log.Printf("[GINIE-TOGGLE] Failed to persist auto-start setting: %v", err)
	}

	// MULTI-USER MODE: Use per-user autopilot if manager is available
	if s.userAutopilotManager != nil && userID != "" {
		var enabled bool
		var dryRun bool

		if req.Enabled {
			// Check if already running for this user
			if s.userAutopilotManager.IsRunning(userID) {
				log.Printf("[GINIE-TOGGLE] Ginie already running for user %s, continuing", userID)
				enabled = true
			} else {
				// Start the user's autopilot
				if err := s.userAutopilotManager.StartAutopilot(ctx, userID); err != nil {
					errorResponse(c, http.StatusInternalServerError, "Failed to start Ginie: "+err.Error())
					return
				}
				log.Printf("[MULTI-USER] User %s enabled Ginie", userID)
				enabled = true
			}
		} else {
			// Check if running for this user
			if !s.userAutopilotManager.IsRunning(userID) {
				log.Printf("[GINIE-TOGGLE] Ginie already stopped for user %s", userID)
				enabled = false
			} else {
				// Stop the user's autopilot
				if err := s.userAutopilotManager.StopAutopilot(userID); err != nil {
					log.Printf("[GINIE-TOGGLE] Stop returned: %v (this is OK if already stopped)", err)
				}
				log.Printf("[MULTI-USER] User %s disabled Ginie", userID)
				enabled = false
			}
		}

		// Get dry_run mode from settings
		currentSettings := sm.GetCurrentSettings()
		dryRun = currentSettings.GinieDryRunMode

		// Broadcast autopilot status change to the SPECIFIC user via WebSocket (multi-user safe)
		if userWSHub != nil {
			userWSHub.BroadcastToUser(userID, events.Event{
				Type:      events.EventAutopilotToggled,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"enabled":  enabled,
					"dry_run":  dryRun,
					"source":   "ginie",
					"user_id":  userID,
				},
			})
			log.Printf("[GINIE-TOGGLE] Broadcasted AUTOPILOT_TOGGLED event to user %s: enabled=%v, dry_run=%v", userID, enabled, dryRun)
		}

		status := s.userAutopilotManager.GetStatus(userID)
		c.JSON(http.StatusOK, gin.H{
			"success":         true,
			"message":         "Ginie toggled",
			"enabled":         enabled,
			"dry_run":         dryRun,
			"user_id":         userID,
			"multi_user_mode": true,
			"status":          status,
		})
		return
	}

	// NO fallback - user authentication required
	errorResponse(c, http.StatusUnauthorized, "User authentication required")
}

// handleGinieScanCoin scans a specific coin
func (s *Server) handleGinieScanCoin(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Initialize user-specific clients (LLM + Binance) from database
	s.tryInitializeUserClients(c, controller)

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

	// Initialize user-specific clients (LLM + Binance) from database
	s.tryInitializeUserClients(c, controller)

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

// handleGinieGetDecisions returns recent decisions/signals for the current user
// Multi-user isolation: Uses per-user GinieAutopilot signal logs instead of shared analyzer
func (s *Server) handleGinieGetDecisions(c *gin.Context) {
	// First try per-user GinieAutopilot for user-specific signal logs
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot != nil {
		signals := giniePilot.GetSignalLogs(20)
		c.JSON(http.StatusOK, gin.H{
			"signals": signals,
			"count":   len(signals),
			"source":  "user_autopilot",
		})
		return
	}

	// Fallback to shared analyzer (for unauthenticated or legacy mode)
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
		"source":    "shared_analyzer",
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
	autopilot := s.getGinieAutopilotForUser(c)
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	stats := autopilot.GetStats()
	config := autopilot.GetConfig()
	positions := autopilot.GetPositions()
	history := autopilot.GetTradeHistory(20)
	blockedCoins := autopilot.GetBlockedCoins()

	// Get available balance for adaptive sizing info
	availableBalance, walletBalance := autopilot.GetBalanceInfo()

	// Reuse cached P/L from Futures Dashboard metrics (no duplicate API calls)
	// The Futures Dashboard already fetches and caches this data with 5-minute TTL
	dailyPnL, totalPnL := s.GetCachedDailyPnL()

	// Override internal counter values with cached Binance API values
	stats["daily_pnl"] = dailyPnL
	stats["total_pnl"] = totalPnL
	// Recalculate combined_pnl with actual daily P/L
	if unrealizedPnL, ok := stats["unrealized_pnl"].(float64); ok {
		stats["combined_pnl"] = dailyPnL + unrealizedPnL
	}

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
	autopilot := s.getGinieAutopilotForUser(c)
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	config := autopilot.GetConfig()
	c.JSON(http.StatusOK, config)
}

// handleUpdateGinieAutopilotConfig updates Ginie autopilot configuration
func (s *Server) handleUpdateGinieAutopilotConfig(c *gin.Context) {
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
			if controller := s.getFuturesAutopilot(); controller != nil {
				controller.SetDryRun(newDryRunValue)
				fmt.Printf("[GINIE-MODE] Called SetDryRun directly\n")
			}

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
	userID := s.getUserID(c)
	ctx := c.Request.Context()

	// MULTI-USER MODE: Use per-user autopilot if manager is available
	if s.userAutopilotManager != nil && userID != "" {
		// Check if already running for this user
		if s.userAutopilotManager.IsRunning(userID) {
			status := s.userAutopilotManager.GetStatus(userID)
			c.JSON(http.StatusOK, gin.H{
				"success":         true,
				"message":         "Your Ginie autopilot is already running",
				"running":         true,
				"user_id":         userID,
				"multi_user_mode": true,
				"status":          status,
			})
			return
		}

		// Start the user's autopilot
		if err := s.userAutopilotManager.StartAutopilot(ctx, userID); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to start Ginie autopilot: "+err.Error())
			return
		}

		log.Printf("[MULTI-USER] User %s started their personal Ginie autopilot", userID)

		// Persist auto-start setting so Ginie restarts automatically after server restart
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateGinieAutoStart(true); err != nil {
			// Log but don't fail - the start was successful
			log.Printf("Failed to persist Ginie auto-start setting: %v", err)
		}

		status := s.userAutopilotManager.GetStatus(userID)
		modeStr := "PAPER"
		if !status.DryRun {
			modeStr = "LIVE"
		}

		c.JSON(http.StatusOK, gin.H{
			"success":         true,
			"message":         "Your personal Ginie autopilot started in " + modeStr + " mode",
			"running":         true,
			"mode":            modeStr,
			"user_id":         userID,
			"multi_user_mode": true,
			"status":          status,
		})
		return
	}

	// NO fallback - user authentication required
	errorResponse(c, http.StatusUnauthorized, "User authentication required")
}

// handleStopGinieAutopilot stops the Ginie autonomous trading
func (s *Server) handleStopGinieAutopilot(c *gin.Context) {
	userID := s.getUserID(c)

	// MULTI-USER MODE: Use per-user autopilot if manager is available
	if s.userAutopilotManager != nil && userID != "" {
		// Check if running for this user
		if !s.userAutopilotManager.IsRunning(userID) {
			status := s.userAutopilotManager.GetStatus(userID)
			c.JSON(http.StatusOK, gin.H{
				"success":         true,
				"message":         "Your Ginie autopilot is already stopped",
				"running":         false,
				"user_id":         userID,
				"multi_user_mode": true,
				"status":          status,
			})
			return
		}

		// Stop the user's autopilot
		if err := s.userAutopilotManager.StopAutopilot(userID); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to stop Ginie autopilot: "+err.Error())
			return
		}

		log.Printf("[MULTI-USER] User %s stopped their personal Ginie autopilot", userID)

		// Clear auto-start setting so Ginie doesn't restart after server restart
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateGinieAutoStart(false); err != nil {
			// Log but don't fail - the stop was successful
			log.Printf("Failed to clear Ginie auto-start setting: %v", err)
		}

		status := s.userAutopilotManager.GetStatus(userID)
		c.JSON(http.StatusOK, gin.H{
			"success":         true,
			"message":         "Your personal Ginie autopilot stopped (will NOT auto-start on server restart)",
			"running":         false,
			"user_id":         userID,
			"multi_user_mode": true,
			"status":          status,
		})
		return
	}

	// NO fallback - user authentication required
	errorResponse(c, http.StatusUnauthorized, "User authentication required")
}

// handleGetGinieAutopilotPositions returns active Ginie positions
func (s *Server) handleGetGinieAutopilotPositions(c *gin.Context) {
	autopilot := s.getGinieAutopilotForUser(c)
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	autopilot := s.getGinieAutopilotForUser(c)
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	autopilot := s.getGinieAutopilotForUser(c)
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	status := giniePilot.GetCircuitBreakerStatus()
	c.JSON(http.StatusOK, status)
}

// handleResetGinieCircuitBreaker resets Ginie circuit breaker
func (s *Server) handleResetGinieCircuitBreaker(c *gin.Context) {
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Get the current futures client based on mode (paper/live)
	futuresClient := s.getFuturesClientForUser(c)

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
		"success":         true,
		"message":         "Synced positions with exchange",
		"synced_count":    synced,
		"total_positions": len(positions),
		"positions":       positions,
	})
}

// handleCloseAllGiniePositions closes all Ginie-managed positions (panic button)
func (s *Server) handleCloseAllGiniePositions(c *gin.Context) {
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Default to async (non-blocking) - return immediately with job ID
	jobID := giniePilot.RecalculateAdaptiveSLTPAsync()
	c.JSON(http.StatusAccepted, gin.H{
		"success":    true,
		"message":    "SLTP recalculation started in background",
		"job_id":     jobID,
		"status_url": "/api/futures/ginie/positions/recalc-sltp/status/" + jobID,
	})
}

// handleGetSLTPJobStatus returns the status of a SLTP recalculation job
func (s *Server) handleGetSLTPJobStatus(c *gin.Context) {
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	jobID := c.Param("job_id")
	if jobID == "" {
		errorResponse(c, http.StatusBadRequest, "Job ID is required")
		return
	}

	queue := giniePilot.GetSLTPJobQueue()
	job := queue.GetJob(jobID)

	if job == nil {
		errorResponse(c, http.StatusNotFound, "Job not found: "+jobID)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"job":     job,
	})
}

// handleListSLTPJobs returns recent SLTP recalculation jobs
func (s *Server) handleListSLTPJobs(c *gin.Context) {
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Get limit from query param (default 10)
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	queue := giniePilot.GetSLTPJobQueue()
	jobs := queue.GetRecentJobs(limit)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"jobs":    jobs,
		"count":   len(jobs),
	})
}

// ==================== Per-Position ROI Target Handlers ====================

// handleSetPositionROITarget sets custom ROI% target for a specific position
func (s *Server) handleSetPositionROITarget(c *gin.Context) {
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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

	// Optionally save to per-user SymbolSettings for future positions
	if req.SaveForFuture {
		userID := s.getUserID(c)
		if userID == "" {
			errorResponse(c, http.StatusUnauthorized, "User authentication required to save symbol settings")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.repo.SetUserSymbolROI(ctx, userID, symbol, req.ROIPercent); err != nil {
			fmt.Printf("[API] Failed to save symbol ROI target for user %s: %v\n", userID, err)
			errorResponse(c, http.StatusInternalServerError, "Failed to save symbol ROI target")
			return
		}
		fmt.Printf("[API] Saved custom ROI %.2f%% for symbol %s to user %s database\n", req.ROIPercent, symbol, userID)
	}

	fmt.Printf("[API] Set custom ROI %.2f%% for position %s (save_for_future=%v)\n",
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
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	// Use per-user Ginie autopilot instance (multi-user safe)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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

	// Get status filter from query param (optional)
	statusFilter := c.Query("status")

	// Get more signals than limit if filtering, to ensure we have enough results
	fetchLimit := limit
	if statusFilter != "" {
		fetchLimit = limit * 5 // Fetch 5x to account for filtering
		if fetchLimit > 1000 {
			fetchLimit = 1000
		}
	}

	allSignals := giniePilot.GetSignalLogs(fetchLimit)

	// Apply status filter if provided
	var signals []autopilot.GinieSignalLog
	if statusFilter != "" {
		signals = make([]autopilot.GinieSignalLog, 0, limit)
		for _, sig := range allSignals {
			if sig.Status == statusFilter {
				signals = append(signals, sig)
				if len(signals) >= limit {
					break
				}
			}
		}
	} else {
		signals = allSignals
		if len(signals) > limit {
			signals = signals[:limit]
		}
	}

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
// Multi-user isolation: Uses per-user GinieAutopilot for accurate running state
func (s *Server) handleGetGinieDiagnostics(c *gin.Context) {
	// CRITICAL FIX: Use per-user GinieAutopilot for accurate "autopilot_running" status
	// The shared system autopilot may not be running even if user's autopilot is running
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		// Fallback to shared controller for unauthenticated or legacy mode
		controller := s.getFuturesAutopilot()
		if controller == nil {
			errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
			return
		}
		giniePilot = controller.GetGinieAutopilot()
		if giniePilot == nil {
			errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
			return
		}
	}

	diagnostics := giniePilot.GetDiagnostics()

	// Override LLM status with user-specific AI key info
	// The global autopilot has no LLM analyzer (uses per-user keys from database)
	// So we need to check if the current user has an active AI key configured
	userID := s.getUserID(c)
	if userID != "" && s.apiKeyService != nil {
		ctx := c.Request.Context()
		aiKey, err := s.apiKeyService.GetActiveAIKey(ctx, userID)
		if err == nil && aiKey != nil && aiKey.APIKey != "" {
			// User has an active AI key - update diagnostics to show connected
			diagnostics.LLMStatus.Connected = true
			diagnostics.LLMStatus.Provider = string(aiKey.Provider)

			// CRITICAL FIX: Remove "LLM analyzer not connected" issue since user has API key
			// The issues array was generated before the LLM status override, so we need to filter it
			filteredIssues := make([]autopilot.DiagnosticIssue, 0, len(diagnostics.Issues))
			for _, issue := range diagnostics.Issues {
				if issue.Message != "LLM analyzer not connected" {
					filteredIssues = append(filteredIssues, issue)
				}
			}
			diagnostics.Issues = filteredIssues
		}
	}

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

// ==================== Mode Configuration CRUD Handlers (Story 2.7 Task 2.7.10) ====================

// handleGetModeConfigs returns all 4 mode configurations (ultrafast, scalp, swing, position)
// GET /api/futures/ginie/mode-configs
func (s *Server) handleGetModeConfigs(c *gin.Context) {
	log.Println("[MODE-CONFIG] Getting all mode configurations")

	sm := autopilot.GetSettingsManager()
	configs := sm.GetAllModeConfigs()

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"mode_configs": configs,
		"valid_modes":  []string{"ultra_fast", "scalp", "swing", "position"},
	})
}

// handleGetModeConfig returns configuration for a specific mode
// GET /api/futures/ginie/mode-config/:mode
func (s *Server) handleGetModeConfig(c *gin.Context) {
	mode := c.Param("mode")
	log.Printf("[MODE-CONFIG] Getting configuration for mode: %s", mode)

	sm := autopilot.GetSettingsManager()
	config, err := sm.GetModeConfig(mode)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mode":    mode,
		"config":  config,
	})
}

// handleUpdateModeConfig updates configuration for a specific mode
// PUT /api/futures/ginie/mode-config/:mode
func (s *Server) handleUpdateModeConfig(c *gin.Context) {
	mode := c.Param("mode")
	log.Printf("[MODE-CONFIG] Updating configuration for mode: %s", mode)

	// Validate mode parameter
	if !autopilot.ValidModes[mode] {
		errorResponse(c, http.StatusBadRequest,
			fmt.Sprintf("Invalid mode '%s': must be ultra_fast, scalp, swing, or position", mode))
		return
	}

	var config autopilot.ModeFullConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Ensure mode name matches URL parameter
	config.ModeName = mode

	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateModeConfig(mode, &config); err != nil {
		errorResponse(c, http.StatusBadRequest, "Failed to update mode config: "+err.Error())
		return
	}

	log.Printf("[MODE-CONFIG] Successfully updated configuration for mode: %s", mode)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Mode configuration updated for %s", mode),
		"mode":    mode,
		"config":  config,
	})
}

// handleResetModeConfigs resets all modes to default configurations
// POST /api/futures/ginie/mode-config/reset
func (s *Server) handleResetModeConfigs(c *gin.Context) {
	log.Println("[MODE-CONFIG] Resetting all mode configurations to defaults")

	sm := autopilot.GetSettingsManager()
	if err := sm.ResetModeConfigs(); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to reset mode configs: "+err.Error())
		return
	}

	configs := sm.GetAllModeConfigs()

	log.Println("[MODE-CONFIG] Successfully reset all mode configurations to defaults")

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "All mode configurations reset to defaults",
		"mode_configs": configs,
	})
}

// handleGetModeCircuitBreakerStatus returns current circuit breaker state for all modes
// GET /api/futures/ginie/mode-circuit-breaker-status
func (s *Server) handleGetModeCircuitBreakerStatus(c *gin.Context) {
	log.Println("[MODE-CONFIG] Getting circuit breaker status for all modes")

	sm := autopilot.GetSettingsManager()
	cbConfigs := sm.GetModeCircuitBreakerConfigs()

	// Get runtime state from Ginie autopilot if available
	controller := s.getFuturesAutopilot()
	var runtimeStatus map[string]interface{}

	if controller != nil {
		giniePilot := controller.GetGinieAutopilot()
		if giniePilot != nil {
			// Get runtime circuit breaker status if method exists
			runtimeStatus = map[string]interface{}{
				"ginie_cb_status": giniePilot.GetCircuitBreakerStatus(),
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                  true,
		"circuit_breaker_configs":  cbConfigs,
		"runtime_status":           runtimeStatus,
		"valid_modes":              []string{"ultra_fast", "scalp", "swing", "position"},
	})
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

// ==================== LLM & Adaptive AI Endpoints (Story 2.8) ====================

// LLMConfig represents global LLM configuration
type LLMConfig struct {
	Provider     string `json:"provider"`      // deepseek, claude, openai, local
	Model        string `json:"model"`         // Model name/ID
	TimeoutMs    int    `json:"timeout_ms"`    // 1000-30000
	MaxRetries   int    `json:"max_retries"`   // Number of retries on failure
	FallbackEnabled bool `json:"fallback_enabled"` // Use fallback provider on failure
	FallbackProvider string `json:"fallback_provider"` // Fallback provider name
}

// ModeLLMSettings represents LLM settings for a specific trading mode
type ModeLLMSettings struct {
	Mode               string  `json:"mode"`                 // ultra_fast, scalp, swing, position
	LLMEnabled         bool    `json:"llm_enabled"`          // Enable LLM for this mode
	LLMWeight          float64 `json:"llm_weight"`           // 0.0-1.0 weight in decision
	SkipOnTimeout      bool    `json:"skip_on_timeout"`      // Skip LLM if timeout
	MinLLMConfidence   int     `json:"min_llm_confidence"`   // 0-100 minimum confidence
	BlockOnDisagreement bool   `json:"block_on_disagreement"` // Block trade if LLM disagrees
	CacheEnabled       bool    `json:"cache_enabled"`        // Cache LLM responses
	CacheTTLSeconds    int     `json:"cache_ttl_seconds"`    // Cache TTL
}

// AdaptiveConfig represents adaptive AI configuration
type AdaptiveConfig struct {
	Enabled              bool    `json:"enabled"`
	LearningRate         float64 `json:"learning_rate"`          // 0.0-1.0
	MinSampleSize        int     `json:"min_sample_size"`        // Minimum trades before adapting
	AnalysisWindowDays   int     `json:"analysis_window_days"`   // Days to analyze
	AutoApplyRecommend   bool    `json:"auto_apply_recommendations"` // Auto-apply approved recommendations
	ConfidenceThreshold  float64 `json:"confidence_threshold"`   // Min confidence for recommendations
}

// AdaptiveRecommendation represents a single recommendation from AdaptiveAI
type AdaptiveRecommendation struct {
	ID           string    `json:"id"`
	Mode         string    `json:"mode"`          // ultra_fast, scalp, swing, position
	Parameter    string    `json:"parameter"`     // Parameter being adjusted
	CurrentValue float64   `json:"current_value"` // Current setting value
	SuggestedValue float64 `json:"suggested_value"` // Recommended new value
	Reasoning    string    `json:"reasoning"`     // Why this change is recommended
	Confidence   float64   `json:"confidence"`    // 0-100 confidence in recommendation
	Impact       string    `json:"impact"`        // Expected impact description
	CreatedAt    time.Time `json:"created_at"`
	Status       string    `json:"status"`        // pending, applied, dismissed
}

// ModeStatistics represents statistics for a trading mode
type ModeStatistics struct {
	Mode          string  `json:"mode"`
	TotalTrades   int     `json:"total_trades"`
	WinCount      int     `json:"win_count"`
	LossCount     int     `json:"loss_count"`
	WinRate       float64 `json:"win_rate"`
	TotalPnL      float64 `json:"total_pnl"`
	AvgPnL        float64 `json:"avg_pnl"`
	AvgHoldTime   string  `json:"avg_hold_time"`
	LLMAccuracy   float64 `json:"llm_accuracy"`
	LastUpdated   time.Time `json:"last_updated"`
}

// LLMDiagnostics represents LLM call statistics
type LLMDiagnostics struct {
	TotalCalls       int64              `json:"total_calls"`
	CacheHits        int64              `json:"cache_hits"`
	CacheMisses      int64              `json:"cache_misses"`
	CacheHitRate     float64            `json:"cache_hit_rate"`
	AvgLatencyMs     float64            `json:"avg_latency_ms"`
	ErrorCount       int64              `json:"error_count"`
	ErrorRate        float64            `json:"error_rate"`
	CallsByProvider  map[string]int64   `json:"calls_by_provider"`
	RecentErrors     []LLMError         `json:"recent_errors"`
	LastResetAt      time.Time          `json:"last_reset_at"`
}

// LLMError represents a recent LLM error
type LLMError struct {
	Timestamp   time.Time `json:"timestamp"`
	Provider    string    `json:"provider"`
	ErrorType   string    `json:"error_type"`
	Message     string    `json:"message"`
	Symbol      string    `json:"symbol,omitempty"`
}

// TradeWithAIContext represents a trade with its AI decision context
type TradeWithAIContext struct {
	TradeID       string    `json:"trade_id"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`           // LONG or SHORT
	Mode          string    `json:"mode"`           // ultra_fast, scalp, swing, position
	EntryPrice    float64   `json:"entry_price"`
	ExitPrice     float64   `json:"exit_price,omitempty"`
	PnL           float64   `json:"pnl"`
	PnLPercent    float64   `json:"pnl_percent"`
	Status        string    `json:"status"`         // open, closed, liquidated
	OpenedAt      time.Time `json:"opened_at"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
	AIReasoning   string    `json:"ai_reasoning"`
	LLMConfidence float64   `json:"llm_confidence"`
	LLMProvider   string    `json:"llm_provider,omitempty"`
	SignalSource  string    `json:"signal_source"`  // ai, strategy, manual
}

// Valid LLM providers
var validLLMProviders = map[string]bool{
	"deepseek": true,
	"claude":   true,
	"openai":   true,
	"local":    true,
}

// handleGetLLMConfig returns global LLM config and all mode settings
// GET /api/futures/ginie/llm-config
func (s *Server) handleGetLLMConfig(c *gin.Context) {
	log.Println("[LLM-CONFIG] Getting LLM configuration")

	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	// Get the nested LLMConfig from settings
	llmCfg := settings.LLMConfig

	// Build global LLM config for response
	llmConfig := LLMConfig{
		Provider:         llmCfg.Provider,
		Model:            llmCfg.Model,
		TimeoutMs:        llmCfg.TimeoutMs,
		MaxRetries:       llmCfg.RetryCount,
		FallbackEnabled:  llmCfg.FallbackProvider != "",
		FallbackProvider: llmCfg.FallbackProvider,
	}

	// Build mode-specific settings from the map
	modeSettings := make(map[string]ModeLLMSettings)

	// Get mode settings from the settings map, with defaults if not present
	modeLLM := settings.ModeLLMSettings
	if modeLLM == nil {
		modeLLM = autopilot.DefaultModeLLMSettings()
	}

	for mode, modeSetting := range modeLLM {
		modeSettings[string(mode)] = ModeLLMSettings{
			Mode:               string(mode),
			LLMEnabled:         modeSetting.LLMEnabled,
			LLMWeight:          modeSetting.LLMWeight,
			SkipOnTimeout:      modeSetting.SkipOnTimeout,
			MinLLMConfidence:   modeSetting.MinLLMConfidence,
			BlockOnDisagreement: modeSetting.BlockOnDisagreement,
			CacheEnabled:       modeSetting.CacheEnabled,
			CacheTTLSeconds:    llmCfg.CacheDurationSec, // Use global cache duration
		}
	}

	// Ensure all modes are present
	for _, mode := range []string{"ultra_fast", "scalp", "swing", "position"} {
		if _, exists := modeSettings[mode]; !exists {
			defaults := autopilot.DefaultModeLLMSettings()
			if def, ok := defaults[autopilot.GinieTradingMode(mode)]; ok {
				modeSettings[mode] = ModeLLMSettings{
					Mode:               mode,
					LLMEnabled:         def.LLMEnabled,
					LLMWeight:          def.LLMWeight,
					SkipOnTimeout:      def.SkipOnTimeout,
					MinLLMConfidence:   def.MinLLMConfidence,
					BlockOnDisagreement: def.BlockOnDisagreement,
					CacheEnabled:       def.CacheEnabled,
					CacheTTLSeconds:    llmCfg.CacheDurationSec,
				}
			}
		}
	}

	// Build adaptive config from nested struct
	adaptiveCfg := settings.AdaptiveAIConfig

	adaptiveConfig := AdaptiveConfig{
		Enabled:             adaptiveCfg.Enabled,
		LearningRate:        float64(adaptiveCfg.MaxAutoAdjustmentPercent) / 100.0, // Convert to 0-1 scale
		MinSampleSize:       adaptiveCfg.MinTradesForLearning,
		AnalysisWindowDays:  adaptiveCfg.LearningWindowHours / 24, // Convert hours to days
		AutoApplyRecommend:  adaptiveCfg.AutoAdjustEnabled,
		ConfidenceThreshold: 70.0, // Default confidence threshold
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"llm_config":      llmConfig,
		"mode_settings":   modeSettings,
		"adaptive_config": adaptiveConfig,
	})
}

// handleUpdateLLMConfig updates global LLM configuration
// PUT /api/futures/ginie/llm-config
func (s *Server) handleUpdateLLMConfig(c *gin.Context) {
	log.Println("[LLM-CONFIG] Updating global LLM configuration")

	var req LLMConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate provider
	if req.Provider != "" && !validLLMProviders[req.Provider] {
		errorResponse(c, http.StatusBadRequest,
			fmt.Sprintf("Invalid provider '%s': must be deepseek, claude, openai, or local", req.Provider))
		return
	}

	// Validate timeout
	if req.TimeoutMs > 0 && (req.TimeoutMs < 1000 || req.TimeoutMs > 30000) {
		errorResponse(c, http.StatusBadRequest, "Timeout must be between 1000-30000ms")
		return
	}

	// Validate fallback provider if specified
	if req.FallbackEnabled && req.FallbackProvider != "" && !validLLMProviders[req.FallbackProvider] {
		errorResponse(c, http.StatusBadRequest,
			fmt.Sprintf("Invalid fallback provider '%s': must be deepseek, claude, openai, or local", req.FallbackProvider))
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	// Update the nested LLMConfig struct
	if req.Provider != "" {
		settings.LLMConfig.Provider = req.Provider
	}
	if req.Model != "" {
		settings.LLMConfig.Model = req.Model
	}
	if req.TimeoutMs > 0 {
		settings.LLMConfig.TimeoutMs = req.TimeoutMs
	}
	if req.MaxRetries >= 0 {
		settings.LLMConfig.RetryCount = req.MaxRetries
	}
	if req.FallbackEnabled {
		if req.FallbackProvider != "" {
			settings.LLMConfig.FallbackProvider = req.FallbackProvider
		}
	} else {
		settings.LLMConfig.FallbackProvider = ""
	}

	if err := sm.SaveSettings(settings); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save LLM config: "+err.Error())
		return
	}

	log.Printf("[LLM-CONFIG] Updated global LLM config: provider=%s, model=%s, timeout=%dms",
		settings.LLMConfig.Provider, settings.LLMConfig.Model, settings.LLMConfig.TimeoutMs)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Global LLM configuration updated",
		"config": LLMConfig{
			Provider:         settings.LLMConfig.Provider,
			Model:            settings.LLMConfig.Model,
			TimeoutMs:        settings.LLMConfig.TimeoutMs,
			MaxRetries:       settings.LLMConfig.RetryCount,
			FallbackEnabled:  settings.LLMConfig.FallbackProvider != "",
			FallbackProvider: settings.LLMConfig.FallbackProvider,
		},
	})
}

// handleUpdateModeLLMSettings updates LLM settings for a specific mode
// PUT /api/futures/ginie/llm-config/:mode
func (s *Server) handleUpdateModeLLMSettings(c *gin.Context) {
	mode := c.Param("mode")
	log.Printf("[LLM-CONFIG] Updating LLM settings for mode: %s", mode)

	// Validate mode using the Ginie trading mode types
	validModes := map[string]autopilot.GinieTradingMode{
		"ultra_fast": autopilot.GinieModeUltraFast,
		"scalp":      autopilot.GinieModeScalp,
		"swing":      autopilot.GinieModeSwing,
		"position":   autopilot.GinieModePosition,
	}

	ginieMode, valid := validModes[mode]
	if !valid {
		errorResponse(c, http.StatusBadRequest,
			fmt.Sprintf("Invalid mode '%s': must be ultra_fast, scalp, swing, or position", mode))
		return
	}

	var req ModeLLMSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate llm_weight
	if req.LLMWeight < 0.0 || req.LLMWeight > 1.0 {
		errorResponse(c, http.StatusBadRequest, "LLM weight must be between 0.0 and 1.0")
		return
	}

	// Validate min_llm_confidence
	if req.MinLLMConfidence < 0 || req.MinLLMConfidence > 100 {
		errorResponse(c, http.StatusBadRequest, "Min LLM confidence must be between 0 and 100")
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetCurrentSettings()

	// Initialize the map if nil
	if settings.ModeLLMSettings == nil {
		settings.ModeLLMSettings = autopilot.DefaultModeLLMSettings()
	}

	// Update the mode-specific settings in the map
	settings.ModeLLMSettings[ginieMode] = autopilot.ModeLLMSettings{
		LLMEnabled:          req.LLMEnabled,
		LLMWeight:           req.LLMWeight,
		SkipOnTimeout:       req.SkipOnTimeout,
		MinLLMConfidence:    req.MinLLMConfidence,
		BlockOnDisagreement: req.BlockOnDisagreement,
		CacheEnabled:        req.CacheEnabled,
	}

	// Update global cache duration if specified
	if req.CacheTTLSeconds > 0 {
		settings.LLMConfig.CacheDurationSec = req.CacheTTLSeconds
	}

	if err := sm.SaveSettings(settings); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save mode LLM settings: "+err.Error())
		return
	}

	log.Printf("[LLM-CONFIG] Updated LLM settings for mode %s: enabled=%v, weight=%.2f, min_confidence=%d",
		mode, req.LLMEnabled, req.LLMWeight, req.MinLLMConfidence)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  fmt.Sprintf("LLM settings updated for %s mode", mode),
		"mode":     mode,
		"settings": req,
	})
}

// handleGetAdaptiveRecommendations returns pending recommendations from AdaptiveAI
// GET /api/futures/ginie/adaptive-recommendations
func (s *Server) handleGetAdaptiveRecommendations(c *gin.Context) {
	log.Println("[ADAPTIVE-AI] Getting adaptive recommendations")

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

	// Get recommendations from AdaptiveAI (if available)
	recommendations := make([]AdaptiveRecommendation, 0)
	statistics := make(map[string]ModeStatistics)
	var lastAnalysis time.Time
	var totalOutcomes int

	// Try to get adaptive AI data from autopilot
	adaptiveData := giniePilot.GetAdaptiveAIData()
	if adaptiveData != nil {
		// Convert to our response types
		for _, rec := range adaptiveData.Recommendations {
			recommendations = append(recommendations, AdaptiveRecommendation{
				ID:             rec.ID,
				Mode:           rec.Mode,
				Parameter:      rec.Parameter,
				CurrentValue:   rec.CurrentValue,
				SuggestedValue: rec.SuggestedValue,
				Reasoning:      rec.Reasoning,
				Confidence:     rec.Confidence,
				Impact:         rec.Impact,
				CreatedAt:      rec.CreatedAt,
				Status:         rec.Status,
			})
		}

		for mode, stats := range adaptiveData.Statistics {
			statistics[mode] = ModeStatistics{
				Mode:        mode,
				TotalTrades: stats.TotalTrades,
				WinCount:    stats.WinCount,
				LossCount:   stats.LossCount,
				WinRate:     stats.WinRate,
				TotalPnL:    stats.TotalPnL,
				AvgPnL:      stats.AvgPnL,
				AvgHoldTime: stats.AvgHoldTime,
				LLMAccuracy: stats.LLMAccuracy,
				LastUpdated: stats.LastUpdated,
			}
		}

		lastAnalysis = adaptiveData.LastAnalysis
		totalOutcomes = adaptiveData.TotalOutcomes
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                true,
		"recommendations":        recommendations,
		"statistics":             statistics,
		"last_analysis":          lastAnalysis,
		"total_outcomes_analyzed": totalOutcomes,
	})
}

// handleApplyRecommendation applies a specific recommendation by ID
// POST /api/futures/ginie/adaptive-recommendations/:id/apply
func (s *Server) handleApplyRecommendation(c *gin.Context) {
	recID := c.Param("id")
	log.Printf("[ADAPTIVE-AI] Applying recommendation: %s", recID)

	if recID == "" {
		errorResponse(c, http.StatusBadRequest, "Recommendation ID is required")
		return
	}

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

	// Apply the recommendation
	err := giniePilot.ApplyAdaptiveRecommendation(recID)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Failed to apply recommendation: "+err.Error())
		return
	}

	log.Printf("[ADAPTIVE-AI] Applied recommendation %s successfully", recID)

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"message":           fmt.Sprintf("Recommendation %s applied successfully", recID),
		"recommendation_id": recID,
	})
}

// handleDismissRecommendation dismisses a specific recommendation
// POST /api/futures/ginie/adaptive-recommendations/:id/dismiss
func (s *Server) handleDismissRecommendation(c *gin.Context) {
	recID := c.Param("id")
	log.Printf("[ADAPTIVE-AI] Dismissing recommendation: %s", recID)

	if recID == "" {
		errorResponse(c, http.StatusBadRequest, "Recommendation ID is required")
		return
	}

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

	// Dismiss the recommendation
	if err := giniePilot.DismissAdaptiveRecommendation(recID); err != nil {
		errorResponse(c, http.StatusBadRequest, "Failed to dismiss recommendation: "+err.Error())
		return
	}

	log.Printf("[ADAPTIVE-AI] Dismissed recommendation %s", recID)

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          fmt.Sprintf("Recommendation %s dismissed", recID),
		"recommendation_id": recID,
	})
}

// handleApplyAllRecommendations applies all pending recommendations
// POST /api/futures/ginie/adaptive-recommendations/apply-all
func (s *Server) handleApplyAllRecommendations(c *gin.Context) {
	log.Println("[ADAPTIVE-AI] Applying all pending recommendations")

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

	// Apply all recommendations
	appliedCount, err := giniePilot.ApplyAllAdaptiveRecommendations()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to apply recommendations: "+err.Error())
		return
	}

	log.Printf("[ADAPTIVE-AI] Applied %d recommendations", appliedCount)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       fmt.Sprintf("Applied %d recommendations", appliedCount),
		"applied_count": appliedCount,
	})
}

// handleGetLLMDiagnosticsV2 returns LLM call statistics (Story 2.8 version)
// GET /api/futures/ginie/llm-diagnostics (enhanced version)
func (s *Server) handleGetLLMDiagnosticsV2(c *gin.Context) {
	log.Println("[LLM-DIAG] Getting LLM diagnostics")

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

	// Get LLM diagnostics from autopilot
	diagData := giniePilot.GetLLMDiagnosticsData()

	var diagnostics LLMDiagnostics
	if diagData != nil {
		recentErrors := make([]LLMError, 0, len(diagData.RecentErrors))
		for _, e := range diagData.RecentErrors {
			recentErrors = append(recentErrors, LLMError{
				Timestamp: e.Timestamp,
				Provider:  e.Provider,
				ErrorType: e.ErrorType,
				Message:   e.Message,
				Symbol:    e.Symbol,
			})
		}

		diagnostics = LLMDiagnostics{
			TotalCalls:      diagData.TotalCalls,
			CacheHits:       diagData.CacheHits,
			CacheMisses:     diagData.CacheMisses,
			CacheHitRate:    diagData.CacheHitRate,
			AvgLatencyMs:    diagData.AvgLatencyMs,
			ErrorCount:      diagData.ErrorCount,
			ErrorRate:       diagData.ErrorRate,
			CallsByProvider: diagData.CallsByProvider,
			RecentErrors:    recentErrors,
			LastResetAt:     diagData.LastResetAt,
		}
	} else {
		// Return empty diagnostics if not available
		diagnostics = LLMDiagnostics{
			CallsByProvider: make(map[string]int64),
			RecentErrors:    make([]LLMError, 0),
			LastResetAt:     time.Now(),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"diagnostics": diagnostics,
	})
}

// handleResetLLMDiagnosticsV2 resets LLM diagnostic counters (Story 2.8 version)
// POST /api/futures/ginie/llm-diagnostics/reset
func (s *Server) handleResetLLMDiagnosticsV2(c *gin.Context) {
	log.Println("[LLM-DIAG] Resetting LLM diagnostics")

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

	// Reset diagnostics
	giniePilot.ResetLLMDiagnostics()

	log.Println("[LLM-DIAG] LLM diagnostics reset successfully")

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "LLM diagnostics reset successfully",
		"reset_at":  time.Now(),
	})
}

// handleGetTradeHistoryWithAI returns recent trade outcomes with AI decision context
// GET /api/futures/ginie/trade-history-ai
func (s *Server) handleGetTradeHistoryWithAI(c *gin.Context) {
	log.Println("[TRADE-HISTORY] Getting trade history with AI context")

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

	// Parse pagination parameters
	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get trades with AI context
	tradesData := giniePilot.GetTradeHistoryWithAIContext(limit, offset)

	trades := make([]TradeWithAIContext, 0, len(tradesData))
	for _, t := range tradesData {
		trade := TradeWithAIContext{
			TradeID:       t.TradeID,
			Symbol:        t.Symbol,
			Side:          t.Side,
			Mode:          t.Mode,
			EntryPrice:    t.EntryPrice,
			ExitPrice:     t.ExitPrice,
			PnL:           t.PnL,
			PnLPercent:    t.PnLPercent,
			Status:        t.Status,
			OpenedAt:      t.OpenedAt,
			AIReasoning:   t.AIReasoning,
			LLMConfidence: t.LLMConfidence,
			LLMProvider:   t.LLMProvider,
			SignalSource:  t.SignalSource,
		}
		if t.ClosedAt != nil {
			trade.ClosedAt = t.ClosedAt
		}
		trades = append(trades, trade)
	}

	// Get total count for pagination
	totalCount := giniePilot.GetTradeHistoryCount()

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"trades":      trades,
		"count":       len(trades),
		"total_count": totalCount,
		"limit":       limit,
		"offset":      offset,
		"has_more":    offset+len(trades) < totalCount,
	})
}

// ==================== BULLETPROOF PROTECTION STATUS ====================

// handleGetProtectionStatus returns the protection status of all active positions
// This endpoint is used by the UI to display real-time SL/TP protection health
func (s *Server) handleGetProtectionStatus(c *gin.Context) {
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

	// Get protection status for all positions
	protectionStatus := giniePilot.GetPositionProtectionStatus()

	// Calculate summary statistics
	totalPositions := len(protectionStatus)
	protectedCount := 0
	unprotectedCount := 0
	healingCount := 0
	emergencyCount := 0

	for _, status := range protectionStatus {
		state, ok := status["protection_state"].(string)
		if !ok {
			continue
		}
		switch state {
		case "PROTECTED", "SL_VERIFIED":
			protectedCount++
		case "UNPROTECTED":
			unprotectedCount++
		case "HEALING":
			healingCount++
		case "EMERGENCY":
			emergencyCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"positions": protectionStatus,
		"summary": gin.H{
			"total":       totalPositions,
			"protected":   protectedCount,
			"unprotected": unprotectedCount,
			"healing":     healingCount,
			"emergency":   emergencyCount,
			"health_pct":  calculateHealthPercent(protectedCount, totalPositions),
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// calculateHealthPercent returns the percentage of protected positions
func calculateHealthPercent(protected, total int) float64 {
	if total == 0 {
		return 100.0 // No positions = 100% healthy
	}
	return float64(protected) / float64(total) * 100.0
}

// handleGetRateLimiterStatus returns the current rate limiter status
func (s *Server) handleGetRateLimiterStatus(c *gin.Context) {
	status := binance.GetRateLimiter().GetStatus()
	c.JSON(http.StatusOK, status)
}

// ========================================
// Scan Source Configuration Handlers
// ========================================

// handleGetScanSourceConfig returns the user's scan source configuration
// GET /api/futures/ginie/scan-config
func (s *Server) handleGetScanSourceConfig(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	settings, err := s.repo.GetUserScanSourceSettings(c.Request.Context(), userID)
	if err != nil {
		log.Printf("[handleGetScanSourceConfig] Error getting scan source settings for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get scan source settings")
		return
	}

	c.JSON(http.StatusOK, settings)
}

// handleUpdateScanSourceConfig updates the user's scan source configuration
// POST /api/futures/ginie/scan-config
func (s *Server) handleUpdateScanSourceConfig(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req database.UserScanSourceSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Set user ID from auth context
	req.UserID = userID

	// Validate settings
	if req.MaxCoins < 5 {
		req.MaxCoins = 5
	}
	if req.MaxCoins > 100 {
		req.MaxCoins = 100
	}

	if err := s.repo.UpsertUserScanSourceSettings(c.Request.Context(), &req); err != nil {
		log.Printf("[handleUpdateScanSourceConfig] Error saving scan source settings for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to save scan source settings")
		return
	}

	log.Printf("[handleUpdateScanSourceConfig] User %s updated scan source config: max_coins=%d, saved=%v, llm=%v, movers=%v",
		userID, req.MaxCoins, req.UseSavedCoins, req.UseLLMList, req.UseMarketMovers)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Scan source configuration saved",
	})
}

// handleGetSavedCoins returns the user's saved coins list
// GET /api/futures/ginie/saved-coins
func (s *Server) handleGetSavedCoins(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	coins, err := s.repo.GetUserSavedCoins(c.Request.Context(), userID)
	if err != nil {
		log.Printf("[handleGetSavedCoins] Error getting saved coins for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get saved coins")
		return
	}

	// Get the full settings to check if saved coins is enabled
	settings, _ := s.repo.GetUserScanSourceSettings(c.Request.Context(), userID)

	c.JSON(http.StatusOK, gin.H{
		"coins":   coins,
		"count":   len(coins),
		"enabled": settings != nil && settings.UseSavedCoins,
	})
}

// handleUpdateSavedCoins updates the user's saved coins list
// POST /api/futures/ginie/saved-coins
func (s *Server) handleUpdateSavedCoins(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		Coins []string `json:"coins"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Normalize coin symbols (uppercase, add USDT if missing)
	normalizedCoins := make([]string, 0, len(req.Coins))
	seen := make(map[string]bool)
	for _, coin := range req.Coins {
		coin = strings.TrimSpace(strings.ToUpper(coin))
		if coin == "" {
			continue
		}
		if !strings.HasSuffix(coin, "USDT") {
			coin = coin + "USDT"
		}
		if !seen[coin] {
			normalizedCoins = append(normalizedCoins, coin)
			seen[coin] = true
		}
	}

	if err := s.repo.UpdateUserSavedCoins(c.Request.Context(), userID, normalizedCoins); err != nil {
		log.Printf("[handleUpdateSavedCoins] Error saving coins for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to save coins")
		return
	}

	log.Printf("[handleUpdateSavedCoins] User %s saved %d coins: %v", userID, len(normalizedCoins), normalizedCoins)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"coins":   normalizedCoins,
		"count":   len(normalizedCoins),
	})
}

// handleGetScanPreview returns a preview of coins that would be scanned based on current config
// GET /api/futures/ginie/scan-preview
func (s *Server) handleGetScanPreview(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Get user's scan source settings
	settings, err := s.repo.GetUserScanSourceSettings(c.Request.Context(), userID)
	if err != nil {
		log.Printf("[handleGetScanPreview] Error getting settings for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get scan source settings")
		return
	}

	preview := make(map[string][]string)
	totalCoins := make(map[string]bool)

	// 1. Saved Coins (if enabled)
	if settings.UseSavedCoins && len(settings.SavedCoins) > 0 {
		preview["saved"] = settings.SavedCoins
		for _, coin := range settings.SavedCoins {
			totalCoins[coin] = true
		}
	}

	// 2. LLM Selection (placeholder - actual selection happens during scan)
	if settings.UseLLMList {
		preview["llm"] = []string{"(AI will select during scan)"}
	}

	// 3. Market Movers (if enabled)
	if settings.UseMarketMovers {
		movers := s.getMarketMoversPreview(settings)
		for category, coins := range movers {
			preview["movers_"+category] = coins
			for _, coin := range coins {
				totalCoins[coin] = true
			}
		}
	}

	// Compile unique coins
	uniqueCoins := make([]string, 0, len(totalCoins))
	for coin := range totalCoins {
		uniqueCoins = append(uniqueCoins, coin)
	}

	// Calculate will_scan count
	willScan := len(uniqueCoins)
	if willScan > settings.MaxCoins {
		willScan = settings.MaxCoins
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"sources":       preview,
		"unique_coins":  uniqueCoins,
		"total_unique":  len(uniqueCoins),
		"max_coins":     settings.MaxCoins,
		"will_scan":     willScan,
		"config_active": settings.UseSavedCoins || settings.UseLLMList || settings.UseMarketMovers,
	})
}

// getMarketMoversPreview gets a preview of market mover coins based on user settings
func (s *Server) getMarketMoversPreview(settings *database.UserScanSourceSettings) map[string][]string {
	result := make(map[string][]string)

	// Get cached market movers from existing system
	// For now, return placeholders - the actual data comes from the existing market movers endpoint
	if settings.MoverGainers {
		result["gainers"] = []string{fmt.Sprintf("(Top %d gainers)", settings.GainersLimit)}
	}
	if settings.MoverLosers {
		result["losers"] = []string{fmt.Sprintf("(Top %d losers)", settings.LosersLimit)}
	}
	if settings.MoverVolume {
		result["volume"] = []string{fmt.Sprintf("(Top %d by volume)", settings.VolumeLimit)}
	}
	if settings.MoverVolatility {
		result["volatility"] = []string{fmt.Sprintf("(Top %d by volatility)", settings.VolatilityLimit)}
	}
	if settings.MoverNewListings {
		result["new"] = []string{fmt.Sprintf("(Last %d new listings)", settings.NewListingsLimit)}
	}

	return result
}
