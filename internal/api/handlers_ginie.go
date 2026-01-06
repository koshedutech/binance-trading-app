package api

import (
	"binance-trading-bot/internal/auth"
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"context"
	"encoding/json"
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
	if err := sm.UpdateGinieAutoStart(req.Enabled, userID); err != nil {
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
		currentSettings := sm.GetDefaultSettings()
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

	// Fetch P/L directly from Binance Income History for accuracy
	// Uses the autopilot's futures client which is already authenticated
	dailyPnL, totalPnL := s.GetBinancePnLForAutopilot(autopilot)

	// Override internal counter values with actual Binance values
	stats["daily_pnl"] = dailyPnL
	stats["total_pnl"] = totalPnL
	// Recalculate combined_pnl with actual daily P/L
	if unrealizedPnL, ok := stats["unrealized_pnl"].(float64); ok {
		stats["combined_pnl"] = dailyPnL + unrealizedPnL
	}

	// Get stuck positions that need manual intervention
	stuckPositions := autopilot.GetStuckPositions()

	c.JSON(http.StatusOK, gin.H{
		"stats":             stats,
		"config":            config,
		"positions":         positions,
		"trade_history":     history,
		"available_balance": availableBalance,
		"wallet_balance":    walletBalance,
		"blocked_coins":     blockedCoins,
		"stuck_positions":   stuckPositions,
		"has_stuck_positions": len(stuckPositions) > 0,
	})
}

// handleGetStuckPositions returns positions that need manual intervention
func (s *Server) handleGetStuckPositions(c *gin.Context) {
	autopilot := s.getGinieAutopilotForUser(c)
	if autopilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	stuckPositions := autopilot.GetStuckPositions()

	c.JSON(http.StatusOK, gin.H{
		"stuck_positions": stuckPositions,
		"count":           len(stuckPositions),
		"has_alerts":      len(stuckPositions) > 0,
		"message":         getStuckPositionsMessage(len(stuckPositions)),
	})
}

func getStuckPositionsMessage(count int) string {
	if count == 0 {
		return "No positions need manual intervention"
	}
	if count == 1 {
		return "1 position needs manual intervention - please close manually"
	}
	return fmt.Sprintf("%d positions need manual intervention - please close manually", count)
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
	if v, ok := updates["enable_ultra_fast_mode"].(bool); ok {
		currentConfig.EnableUltraFastMode = v
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
		if err := sm.UpdateGinieAutoStart(true, userID); err != nil {
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
		if err := sm.UpdateGinieAutoStart(false, userID); err != nil {
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

// ==================== Global Circuit Breaker Handlers (Story 5.3) ====================

// handleGetGlobalCircuitBreaker returns user's global circuit breaker config
// GET /api/user/global-circuit-breaker
func (s *Server) handleGetGlobalCircuitBreaker(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	config, err := s.repo.GetUserGlobalCircuitBreaker(c.Request.Context(), userID)
	if err != nil {
		log.Printf("[GLOBAL-CIRCUIT-BREAKER] Failed to get config for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get global circuit breaker config")
		return
	}

	// If no config exists, return defaults
	if config == nil {
		config = database.DefaultGlobalCircuitBreakerConfig()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  config,
	})
}

// handleUpdateGlobalCircuitBreaker updates user's global circuit breaker config
// PUT /api/user/global-circuit-breaker
func (s *Server) handleUpdateGlobalCircuitBreaker(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var config database.GlobalCircuitBreakerConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate config
	if err := config.Validate(); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Save to database
	if err := s.repo.SaveUserGlobalCircuitBreaker(c.Request.Context(), userID, &config); err != nil {
		log.Printf("[GLOBAL-CIRCUIT-BREAKER] Failed to save config for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to save global circuit breaker config")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Global circuit breaker config updated successfully",
		"config":  config,
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
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
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

// handleGetAllMarketMovers returns ALL market movers WITHOUT volume filtering
// This shows the real top gainers/losers including low-volume coins (like Binance app shows)
func (s *Server) handleGetAllMarketMovers(c *gin.Context) {
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

	movers, err := ginie.GetAllMarketMovers(topN)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to get all market movers: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"top_n":           topN,
		"top_gainers":     movers.TopGainers,
		"top_losers":      movers.TopLosers,
		"top_volume":      movers.TopVolume,
		"high_volatility": movers.HighVolatility,
		"note":            "No volume filter applied - includes all coins",
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
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	stats := giniePilot.GetSignalStats()
	c.JSON(http.StatusOK, stats)
}

// ==================== Ginie SL Update History Handlers ====================

// handleGetGinieSLHistory returns SL update history for all or specific symbol
func (s *Server) handleGetGinieSLHistory(c *gin.Context) {
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	stats := giniePilot.GetSLUpdateStats()
	c.JSON(http.StatusOK, stats)
}

// ==================== Ginie LLM SL Validation Handlers ====================

// handleGetGinieLLMSLStatus returns LLM SL kill switch status
func (s *Server) handleGetGinieLLMSLStatus(c *gin.Context) {
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	status := giniePilot.GetLLMSLStatus()
	c.JSON(http.StatusOK, status)
}

// handleResetGinieLLMSL resets the LLM SL kill switch for a specific symbol
func (s *Server) handleResetGinieLLMSL(c *gin.Context) {
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
	// Use per-user GinieAutopilot for accurate "autopilot_running" status
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
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
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Get filter from query param
	source := c.DefaultQuery("source", "all")

	positions := giniePilot.GetPositions()

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
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Get filter from query param
	source := c.DefaultQuery("source", "all")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	history := giniePilot.GetTradeHistory(limit * 2) // Get more to filter

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

// ==================== Mode Configuration CRUD Handlers (Story 2.7 Task 2.7.10) ====================

// handleGetModeConfigs returns all 4 mode configurations (ultrafast, scalp, swing, position)
// GET /api/futures/ginie/mode-configs
// DATABASE ONLY: Reads from database for authenticated user. No JSON fallback for existing users.
func (s *Server) handleGetModeConfigs(c *gin.Context) {
	log.Println("[MODE-CONFIG] Getting all mode configurations (DATABASE ONLY)")

	// Get userID from context (JWT auth)
	userID, exists := c.Get("user_id")
	if !exists {
		// No auth - return error (user must be authenticated)
		errorResponse(c, http.StatusUnauthorized, "Authentication required to access mode configs")
		return
	}
	userIDStr := userID.(string)

	// DATABASE ONLY: Get configs from database for this user - NO JSON FALLBACK
	sm := autopilot.GetSettingsManager()
	ctx := context.Background()
	dbConfigs, err := sm.GetAllUserModeConfigsFromDB(ctx, s.repo, userIDStr)
	if err != nil {
		log.Printf("[MODE-CONFIG] ERROR: Failed to get DB configs for user %s: %v", userIDStr, err)
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to load mode configs from database: %v", err))
		return
	}

	// Ensure all modes are present - if user is missing a mode, log warning
	// This should NOT happen for properly initialized users
	defaultConfigs := autopilot.DefaultModeConfigs()
	mergedConfigs := make(map[string]*autopilot.ModeFullConfig)
	for mode, defaultCfg := range defaultConfigs {
		if dbCfg, ok := dbConfigs[mode]; ok {
			mergedConfigs[mode] = dbCfg
			log.Printf("[MODE-CONFIG] %s: loaded from DB (enabled=%v)", mode, dbCfg.Enabled)
		} else {
			// This should only happen for new users or incomplete DB setup
			log.Printf("[MODE-CONFIG] WARNING: User %s missing mode %s in DB - using default (this may indicate incomplete initialization)", userIDStr, mode)
			mergedConfigs[mode] = defaultCfg
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"mode_configs": mergedConfigs,
		"valid_modes":  []string{"ultra_fast", "scalp", "swing", "position"},
		"source":       "database",
	})
}

// handleGetModeConfig returns configuration for a specific mode
// GET /api/futures/ginie/mode-config/:mode
// DATABASE ONLY: Reads from database for authenticated user. No JSON fallback for existing users.
func (s *Server) handleGetModeConfig(c *gin.Context) {
	mode := c.Param("mode")
	log.Printf("[MODE-CONFIG] Getting configuration for mode: %s (DATABASE ONLY)", mode)

	// Validate mode name
	validModes := map[string]bool{
		"ultra_fast": true, "scalp": true, "swing": true,
		"position": true, "scalp_reentry": true,
	}
	if !validModes[mode] {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("invalid mode: %s", mode))
		return
	}

	// Get userID from context (JWT auth)
	userID, exists := c.Get("user_id")
	if !exists {
		// No auth - return error (user must be authenticated)
		errorResponse(c, http.StatusUnauthorized, "Authentication required to access mode config")
		return
	}
	userIDStr := userID.(string)

	// DATABASE ONLY: Get config from database for this user - NO JSON FALLBACK
	sm := autopilot.GetSettingsManager()
	ctx := context.Background()
	config, err := sm.GetUserModeConfigFromDB(ctx, s.repo, userIDStr, mode)
	if err != nil {
		log.Printf("[MODE-CONFIG] ERROR: Failed to get DB config for user %s mode %s: %v", userIDStr, mode, err)

		// Check if this is a missing mode (user not initialized) vs database error
		if err.Error() == "mode config not found" {
			log.Printf("[MODE-CONFIG] WARNING: User %s missing mode %s - incomplete initialization, using default", userIDStr, mode)
			// Only in this specific case, use default to fill the gap
			defaultConfig, defaultErr := sm.GetDefaultModeConfig(mode)
			if defaultErr != nil {
				errorResponse(c, http.StatusBadRequest, defaultErr.Error())
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"mode":    mode,
				"config":  defaultConfig,
				"source":  "default_fallback_initialization_incomplete",
			})
			return
		}

		// For other errors, return error response
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to load mode config from database: %v", err))
		return
	}

	log.Printf("[MODE-CONFIG] Loaded %s from DB for user %s (enabled=%v)", mode, userIDStr, config.Enabled)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mode":    mode,
		"config":  config,
		"source":  "database",
	})
}

// handleUpdateModeConfig updates configuration for a specific mode
// PUT /api/futures/ginie/mode-config/:mode
func (s *Server) handleUpdateModeConfig(c *gin.Context) {
	mode := c.Param("mode")

	// Validate mode name
	validModes := map[string]bool{
		"ultra_fast": true, "scalp": true, "swing": true,
		"position": true, "scalp_reentry": true,
	}
	if !validModes[mode] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid mode: %s", mode)})
		return
	}

	var config autopilot.ModeFullConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.ModeName = mode

	// Get userID from context (JWT auth)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userIDStr := userID.(string)

	// Marshal config to JSON for database storage
	configJSON, err := json.Marshal(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal config"})
		return
	}

	// CRITICAL: Save to DATABASE (not just JSON file)
	ctx := context.Background()
	err = s.repo.SaveUserModeConfig(ctx, userIDStr, mode, config.Enabled, configJSON)
	if err != nil {
		log.Printf("[MODE-CONFIG] Failed to save %s config to database for user %s: %v", mode, userIDStr, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save mode config to database"})
		return
	}

	log.Printf("[MODE-CONFIG] Saved %s config to database for user %s (enabled=%v)", mode, userIDStr, config.Enabled)

	// Also update in-memory settings for backwards compatibility
	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateModeConfig(mode, &config); err != nil {
		log.Printf("[MODE-CONFIG] Warning: failed to update in-memory config: %v", err)
		// Don't fail - database is the source of truth now
	}

	// Story 4.15: Auto-sync to default-settings.json if admin user
	// Get user email to check if admin
	user, err := s.repo.GetUserByID(ctx, userIDStr)
	if err == nil && user != nil {
		SyncAdminModeConfigIfAdmin(ctx, user.Email, mode, &config)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mode":    mode,
		"enabled": config.Enabled,
		"message": "Mode config saved to database",
	})
}

// handleToggleModeEnabled toggles mode enabled status (quick toggle without full config)
// POST /api/futures/ginie/mode-config/:mode/toggle
func (s *Server) handleToggleModeEnabled(c *gin.Context) {
	mode := c.Param("mode")

	// Validate mode name
	validModes := map[string]bool{
		"ultra_fast": true, "scalp": true, "swing": true,
		"position": true, "scalp_reentry": true,
	}
	if !validModes[mode] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid mode: %s", mode)})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get userID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}
	userIDStr := userID.(string)

	// Update enabled status in database
	ctx := context.Background()
	err := s.repo.UpdateUserModeEnabled(ctx, userIDStr, mode, req.Enabled)
	if err != nil {
		log.Printf("[MODE-TOGGLE] Failed to toggle %s for user %s: %v", mode, userIDStr, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to toggle mode"})
		return
	}

	log.Printf("[MODE-TOGGLE] %s mode %s for user %s (immediate effect)", mode,
		map[bool]string{true: "ENABLED", false: "DISABLED"}[req.Enabled], userIDStr)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mode":    mode,
		"enabled": req.Enabled,
		"message": "Mode toggled - takes effect on next scan cycle",
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

	configs := sm.GetDefaultModeConfigs()

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

	// Get runtime state from Ginie autopilot if available (per-user)
	giniePilot := s.getGinieAutopilotForUser(c)
	var modeStatus map[string]interface{}
	var globalStatus map[string]interface{}

	if giniePilot != nil {
		// Get per-mode circuit breaker runtime status
		modeStatus = giniePilot.GetAllModeCircuitBreakerStatus()
		// Get global circuit breaker status
		globalStatus = giniePilot.GetCircuitBreakerStatus()
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                  true,
		"circuit_breaker_configs":  cbConfigs,
		"mode_status":              modeStatus,
		"global_status":            globalStatus,
		"valid_modes":              []string{"ultra_fast", "scalp", "swing", "position"},
	})
}

// handleResetModeCircuitBreaker resets the circuit breaker for a specific mode
// POST /api/futures/ginie/mode-circuit-breaker/:mode/reset
func (s *Server) handleResetModeCircuitBreaker(c *gin.Context) {
	mode := c.Param("mode")
	log.Printf("[MODE-CONFIG] Resetting circuit breaker for mode: %s", mode)

	// Validate mode
	validModes := map[string]autopilot.GinieTradingMode{
		"ultra_fast": autopilot.GinieModeUltraFast,
		"scalp":      autopilot.GinieModeScalp,
		"swing":      autopilot.GinieModeSwing,
		"position":   autopilot.GinieModePosition,
	}

	ginieMode, ok := validModes[mode]
	if !ok {
		errorResponse(c, http.StatusBadRequest, "Invalid mode. Must be one of: ultra_fast, scalp, swing, position")
		return
	}

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Reset the mode circuit breaker
	err := giniePilot.ResetModeCircuitBreaker(ginieMode)
	if err != nil {
		log.Printf("[MODE-CONFIG] Failed to reset circuit breaker for mode %s: %v", mode, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to reset circuit breaker: "+err.Error())
		return
	}

	log.Printf("[MODE-CONFIG] Successfully reset circuit breaker for mode: %s", mode)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Circuit breaker for %s mode has been reset", mode),
	})
}

// handleGetGinieTradeHistoryWithDateRange returns trade history filtered by date range
func (s *Server) handleGetGinieTradeHistoryWithDateRange(c *gin.Context) {
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
		historyTrades := giniePilot.GetTradeHistory(50)
		for _, trade := range historyTrades {
			trades = append(trades, trade)
		}
	} else {
		historyTrades := giniePilot.GetTradeHistoryInDateRange(startTime, endTime)
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
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
		trades = giniePilot.GetTradeHistory(1000)
	} else {
		trades = giniePilot.GetTradeHistoryInDateRange(startTime, endTime)
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
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Get all LLM switches
	switches := giniePilot.GetLLMSwitches(500)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"switches": switches,
		"count":   len(switches),
	})
}

// handleResetGinieLLMDiagnostics clears LLM diagnostic data
func (s *Server) handleResetGinieLLMDiagnostics(c *gin.Context) {
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Clear LLM switch history
	giniePilot.ClearLLMSwitches()

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
	settings := sm.GetDefaultSettings()

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
	settings := sm.GetDefaultSettings()

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
	settings := sm.GetDefaultSettings()

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

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
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

	c.JSON(http.StatusOK, gin.H{
		"config": settings,
	})
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
	settings, err := s.repo.GetUserScanSourceSettings(c.Request.Context(), userID)
	if err != nil {
		log.Printf("[handleGetSavedCoins] Warning: failed to get scan source settings for user %s: %v", userID, err)
	}

	c.JSON(http.StatusOK, gin.H{
		"saved_coins": coins,
		"count":       len(coins),
		"enabled":     settings != nil && settings.UseSavedCoins,
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

	// Track coins with their sources: map[symbol][]sources
	coinSources := make(map[string][]string)

	// 1. Saved Coins (if enabled)
	if settings.UseSavedCoins && len(settings.SavedCoins) > 0 {
		for _, coin := range settings.SavedCoins {
			coinSources[coin] = append(coinSources[coin], "saved")
		}
	}

	// 2. LLM Selection - get actual LLM coins if available
	if settings.UseLLMList {
		controller := s.getFuturesAutopilot()
		if controller != nil {
			if ginie := controller.GetGinieAnalyzer(); ginie != nil {
				llmCoins := ginie.GetLLMSelectedCoins()
				for _, coin := range llmCoins {
					coinSources[coin] = append(coinSources[coin], "llm")
				}
			}
		}
	}

	// 3. Market Movers (if enabled) - get actual market movers
	if settings.UseMarketMovers {
		controller := s.getFuturesAutopilot()
		if controller != nil {
			if ginie := controller.GetGinieAnalyzer(); ginie != nil {
				// Get max limit from settings
				maxLimit := settings.GainersLimit
				if settings.LosersLimit > maxLimit {
					maxLimit = settings.LosersLimit
				}
				if settings.VolumeLimit > maxLimit {
					maxLimit = settings.VolumeLimit
				}
				if settings.VolatilityLimit > maxLimit {
					maxLimit = settings.VolatilityLimit
				}
				if maxLimit < 10 {
					maxLimit = 10
				}

				// Use GetAllMarketMovers to include ALL top gainers (no volume filter)
				// This allows trading coins like BUSDT, USELESSUSDT, PIEVERSEUSDT etc.
				movers, err := ginie.GetAllMarketMovers(maxLimit)
				if err == nil {
					if settings.MoverGainers && len(movers.TopGainers) > 0 {
						for i, coin := range movers.TopGainers {
							if i >= settings.GainersLimit {
								break
							}
							coinSources[coin] = append(coinSources[coin], "gainers")
						}
					}
					if settings.MoverLosers && len(movers.TopLosers) > 0 {
						for i, coin := range movers.TopLosers {
							if i >= settings.LosersLimit {
								break
							}
							coinSources[coin] = append(coinSources[coin], "losers")
						}
					}
					if settings.MoverVolume && len(movers.TopVolume) > 0 {
						for i, coin := range movers.TopVolume {
							if i >= settings.VolumeLimit {
								break
							}
							coinSources[coin] = append(coinSources[coin], "volume")
						}
					}
					if settings.MoverVolatility && len(movers.HighVolatility) > 0 {
						for i, coin := range movers.HighVolatility {
							if i >= settings.VolatilityLimit {
								break
							}
							coinSources[coin] = append(coinSources[coin], "volatility")
						}
					}
				}
			}
		}
	}

	// Build coins array with sources for frontend
	type coinPreview struct {
		Symbol  string   `json:"symbol"`
		Sources []string `json:"sources"`
	}

	coins := make([]coinPreview, 0, len(coinSources))
	for symbol, sources := range coinSources {
		coins = append(coins, coinPreview{
			Symbol:  symbol,
			Sources: sources,
		})
	}

	// Limit to max_coins
	totalCount := len(coins)
	if len(coins) > settings.MaxCoins {
		coins = coins[:settings.MaxCoins]
	}

	c.JSON(http.StatusOK, gin.H{
		"coins":       coins,
		"total_count": totalCount,
		"max_coins":   settings.MaxCoins,
	})
}

// ============================================================================
// SYMBOL BLOCKING HANDLERS (Worst Performer Daily Blocking)
// ============================================================================

// handleBlockSymbolForDay blocks a symbol for the rest of the day
// POST /api/futures/autopilot/symbols/:symbol/block-day
func (s *Server) handleBlockSymbolForDay(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	var req struct {
		Reason string `json:"reason"` // Optional custom reason
	}
	c.ShouldBindJSON(&req)

	reason := req.Reason
	if reason == "" {
		reason = "manual_block"
	}

	sm := autopilot.GetSettingsManager()
	err := sm.BlockSymbolForDay(symbol, reason)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to block symbol: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"symbol":        symbol,
		"blocked_until": time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02 23:59:59 UTC"),
		"reason":        reason,
	})
}

// handleUnblockSymbol removes the block from a symbol
// POST /api/futures/autopilot/symbols/:symbol/unblock
func (s *Server) handleUnblockSymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	sm := autopilot.GetSettingsManager()
	err := sm.UnblockSymbol(symbol)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to unblock symbol: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"symbol":  symbol,
		"message": "Symbol unblocked successfully",
	})
}

// handleGetBlockedSymbols returns all currently blocked symbols
// GET /api/futures/autopilot/symbols/blocked
func (s *Server) handleGetBlockedSymbols(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	blocked := sm.GetAllBlockedSymbols()

	// Convert to JSON-friendly format
	result := make([]map[string]interface{}, 0)
	for symbol, info := range blocked {
		result = append(result, map[string]interface{}{
			"symbol":        symbol,
			"blocked_until": info.Until.Format(time.RFC3339),
			"reason":        info.Reason,
			"remaining":     time.Until(info.Until).String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"blocked_symbols": result,
		"total":           len(result),
	})
}

// handleAutoBlockWorstPerformers blocks all "worst" category symbols for the day
// POST /api/futures/autopilot/symbols/auto-block-worst
func (s *Server) handleAutoBlockWorstPerformers(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	blockedSymbols, err := sm.AutoBlockWorstPerformers()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to auto-block worst performers: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"blocked_symbols": blockedSymbols,
		"count":           len(blockedSymbols),
		"message":         fmt.Sprintf("Blocked %d worst performing symbols for the day", len(blockedSymbols)),
	})
}

// handleClearExpiredBlocks removes expired blocks from all symbols
// POST /api/futures/autopilot/symbols/clear-expired-blocks
func (s *Server) handleClearExpiredBlocks(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	cleared := sm.ClearExpiredBlocks()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"cleared": cleared,
		"message": fmt.Sprintf("Cleared %d expired blocks", cleared),
	})
}

// handleGetSymbolBlockStatus checks if a specific symbol is blocked
// GET /api/futures/autopilot/symbols/:symbol/block-status
func (s *Server) handleGetSymbolBlockStatus(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	sm := autopilot.GetSettingsManager()
	reason, until, isBlocked := sm.GetBlockedReason(symbol)

	response := gin.H{
		"symbol":     symbol,
		"is_blocked": isBlocked,
	}

	if isBlocked {
		response["blocked_until"] = until.Format(time.RFC3339)
		response["reason"] = reason
		response["remaining"] = time.Until(until).String()
	}

	c.JSON(http.StatusOK, response)
}

// handleGetMorningAutoBlockConfig returns morning auto-block configuration
// GET /api/futures/autopilot/morning-auto-block/config
func (s *Server) handleGetMorningAutoBlockConfig(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	settings := sm.GetDefaultSettings()

	// Calculate next scheduled time
	hour := settings.MorningAutoBlockHourUTC
	minute := settings.MorningAutoBlockMinUTC
	if hour < 0 || hour > 23 {
		hour = 0
	}
	if minute < 0 || minute > 59 {
		minute = 5
	}

	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"enabled":        settings.MorningAutoBlockEnabled,
		"hour_utc":       hour,
		"minute_utc":     minute,
		"next_run":       next.Format(time.RFC3339),
		"time_until":     time.Until(next).String(),
	})
}

// handleUpdateMorningAutoBlockConfig updates morning auto-block configuration
// POST /api/futures/autopilot/morning-auto-block/config
func (s *Server) handleUpdateMorningAutoBlockConfig(c *gin.Context) {
	var req struct {
		Enabled   *bool `json:"enabled"`
		HourUTC   *int  `json:"hour_utc"`
		MinuteUTC *int  `json:"minute_utc"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetDefaultSettings()

	// Update only provided fields
	if req.Enabled != nil {
		settings.MorningAutoBlockEnabled = *req.Enabled
	}
	if req.HourUTC != nil {
		if *req.HourUTC < 0 || *req.HourUTC > 23 {
			errorResponse(c, http.StatusBadRequest, "hour_utc must be 0-23")
			return
		}
		settings.MorningAutoBlockHourUTC = *req.HourUTC
	}
	if req.MinuteUTC != nil {
		if *req.MinuteUTC < 0 || *req.MinuteUTC > 59 {
			errorResponse(c, http.StatusBadRequest, "minute_utc must be 0-59")
			return
		}
		settings.MorningAutoBlockMinUTC = *req.MinuteUTC
	}

	// Save settings
	if err := sm.SaveSettings(settings); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save settings: "+err.Error())
		return
	}

	// Calculate next scheduled time with new settings
	hour := settings.MorningAutoBlockHourUTC
	minute := settings.MorningAutoBlockMinUTC
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"enabled":        settings.MorningAutoBlockEnabled,
		"hour_utc":       hour,
		"minute_utc":     minute,
		"next_run":       next.Format(time.RFC3339),
		"time_until":     time.Until(next).String(),
		"message":        "Morning auto-block configuration updated",
	})
}

// ==================== SLTP Configuration Endpoints ====================

// handleGetGinieSLTPConfig returns SL/TP configuration for all modes
// GET /api/futures/ginie/sltp-config
func (s *Server) handleGetGinieSLTPConfig(c *gin.Context) {
	log.Println("[SLTP-CONFIG] Getting SL/TP configuration for all modes")

	sm := autopilot.GetSettingsManager()
	configs := sm.GetDefaultModeConfigs()

	// Extract just the SLTP config from each mode
	sltpConfigs := make(map[string]interface{})
	for modeName, config := range configs {
		if config.SLTP != nil {
			sltpConfigs[modeName] = config.SLTP
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"sltp_configs": sltpConfigs,
		"valid_modes":  []string{"ultra_fast", "scalp", "swing", "position"},
	})
}

// handleUpdateGinieSLTPConfig updates SL/TP configuration for a specific mode
// POST /api/futures/ginie/sltp-config/:mode
func (s *Server) handleUpdateGinieSLTPConfig(c *gin.Context) {
	mode := c.Param("mode")
	log.Printf("[SLTP-CONFIG] Updating SL/TP configuration for mode: %s", mode)

	// Validate mode parameter
	if !autopilot.ValidModes[mode] {
		errorResponse(c, http.StatusBadRequest,
			fmt.Sprintf("Invalid mode '%s': must be ultra_fast, scalp, swing, or position", mode))
		return
	}

	var sltpConfig autopilot.ModeSLTPConfig
	if err := c.ShouldBindJSON(&sltpConfig); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()

	// Get current config and update SLTP section
	config, err := sm.GetDefaultModeConfig(mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to get mode config: "+err.Error())
		return
	}

	config.SLTP = &sltpConfig

	if err := sm.UpdateModeConfig(mode, config); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update SLTP config: "+err.Error())
		return
	}

	log.Printf("[SLTP-CONFIG] Successfully updated SL/TP configuration for mode: %s", mode)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("SL/TP configuration updated for %s", mode),
		"mode":    mode,
		"sltp":    sltpConfig,
	})
}

// ==================== Trend Timeframe Endpoints ====================

// handleGetGinieTrendTimeframes returns trend timeframe configuration for all modes
// GET /api/futures/ginie/trend-timeframes
func (s *Server) handleGetGinieTrendTimeframes(c *gin.Context) {
	log.Println("[TREND-TF] Getting trend timeframe configuration for all modes")

	sm := autopilot.GetSettingsManager()
	configs := sm.GetDefaultModeConfigs()

	// Extract timeframe config from each mode
	timeframeConfigs := make(map[string]interface{})
	for modeName, config := range configs {
		if config.Timeframe != nil {
			timeframeConfigs[modeName] = config.Timeframe
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"timeframes": timeframeConfigs,
		"valid_modes": []string{"ultra_fast", "scalp", "swing", "position"},
	})
}

// handleUpdateGinieTrendTimeframes updates trend timeframe configuration
// POST /api/futures/ginie/trend-timeframes
func (s *Server) handleUpdateGinieTrendTimeframes(c *gin.Context) {
	log.Println("[TREND-TF] Updating trend timeframe configuration")

	var req struct {
		Mode              string `json:"mode"`               // Optional: if provided, update only this mode
		TrendTimeframe    string `json:"trend_timeframe"`    // e.g., "5m", "15m", "1h", "4h"
		EntryTimeframe    string `json:"entry_timeframe"`    // e.g., "1m", "5m", "15m"
		AnalysisTimeframe string `json:"analysis_timeframe"` // e.g., "1m", "15m", "4h"
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()

	// If mode is specified, update only that mode
	if req.Mode != "" {
		if !autopilot.ValidModes[req.Mode] {
			errorResponse(c, http.StatusBadRequest,
				fmt.Sprintf("Invalid mode '%s': must be ultra_fast, scalp, swing, or position", req.Mode))
			return
		}

		config, err := sm.GetDefaultModeConfig(req.Mode)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to get mode config: "+err.Error())
			return
		}

		if config.Timeframe == nil {
			config.Timeframe = &autopilot.ModeTimeframeConfig{}
		}

		if req.TrendTimeframe != "" {
			config.Timeframe.TrendTimeframe = req.TrendTimeframe
		}
		if req.EntryTimeframe != "" {
			config.Timeframe.EntryTimeframe = req.EntryTimeframe
		}
		if req.AnalysisTimeframe != "" {
			config.Timeframe.AnalysisTimeframe = req.AnalysisTimeframe
		}

		if err := sm.UpdateModeConfig(req.Mode, config); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to update timeframe config: "+err.Error())
			return
		}

		log.Printf("[TREND-TF] Successfully updated timeframe configuration for mode: %s", req.Mode)

		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   fmt.Sprintf("Timeframe configuration updated for %s", req.Mode),
			"mode":      req.Mode,
			"timeframe": config.Timeframe,
		})
		return
	}

	// No mode specified - return current configs
	configs := sm.GetDefaultModeConfigs()
	timeframeConfigs := make(map[string]interface{})
	for modeName, config := range configs {
		if config.Timeframe != nil {
			timeframeConfigs[modeName] = config.Timeframe
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Provide 'mode' parameter to update a specific mode's timeframes",
		"timeframes": timeframeConfigs,
	})
}

// ==================== Ultra-Fast Mode Configuration Endpoints ====================

// handleGetUltraFastConfig returns ultra-fast mode configuration
// GET /api/futures/ultrafast/config
// DB-FIRST: Reads enabled status from database, not from Ginie config
func (s *Server) handleGetUltraFastConfig(c *gin.Context) {
	log.Println("[ULTRAFAST] Getting ultra-fast mode configuration (DB-first)")

	// Get user ID from JWT
	userID, exists := c.Get("user_id")
	if !exists {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}
	userIDStr := userID.(string)

	sm := autopilot.GetSettingsManager()
	ctx := context.Background()

	// DB-FIRST: Read from database first
	modeConfig, err := sm.GetUserModeConfigFromDB(ctx, s.repo, userIDStr, "ultra_fast")
	source := "database"
	if err != nil || modeConfig == nil {
		// Fallback to defaults
		modeConfig, _ = sm.GetDefaultModeConfig("ultra_fast")
		source = "defaults"
	}

	// Enabled comes from database config, not from Ginie in-memory state
	enabled := false
	if modeConfig != nil {
		enabled = modeConfig.Enabled
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mode":    "ultra_fast",
		"enabled": enabled,
		"config":  modeConfig,
		"source":  source,
	})
}

// handleUpdateUltraFastConfig updates ultra-fast mode configuration
// POST /api/futures/ultrafast/config
// DB-FIRST: Saves all changes to database AND updates in-memory for instant effect
func (s *Server) handleUpdateUltraFastConfig(c *gin.Context) {
	log.Println("[ULTRAFAST] Updating ultra-fast mode configuration (DB-first)")

	var req struct {
		Enabled *bool                     `json:"enabled"` // Enable/disable ultra-fast mode globally
		Config  *autopilot.ModeFullConfig `json:"config"`  // Full config update (optional)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Get user ID from JWT
	userID, exists := c.Get("user_id")
	if !exists {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}
	userIDStr := userID.(string)

	sm := autopilot.GetSettingsManager()
	ctx := context.Background()

	// Get current config from database (or defaults)
	currentConfig, err := sm.GetUserModeConfigFromDB(ctx, s.repo, userIDStr, "ultra_fast")
	if err != nil || currentConfig == nil {
		currentConfig, _ = sm.GetDefaultModeConfig("ultra_fast")
	}

	// Update enabled flag if provided
	if req.Enabled != nil {
		currentConfig.Enabled = *req.Enabled
		log.Printf("[ULTRAFAST] Setting enabled=%v", *req.Enabled)
	}

	// Merge full config if provided
	if req.Config != nil {
		req.Config.ModeName = "ultra_fast"
		// Preserve enabled status from explicit flag or current state
		if req.Enabled != nil {
			req.Config.Enabled = *req.Enabled
		} else {
			req.Config.Enabled = currentConfig.Enabled
		}
		currentConfig = req.Config
		log.Println("[ULTRAFAST] Merged full config update")
	}

	// Save to database (source of truth)
	configJSON, err := json.Marshal(currentConfig)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to marshal config: "+err.Error())
		return
	}
	err = s.repo.SaveUserModeConfig(ctx, userIDStr, "ultra_fast", currentConfig.Enabled, configJSON)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save to database: "+err.Error())
		return
	}
	log.Printf("[ULTRAFAST] Saved ultra_fast config to database for user %s", userIDStr)

	// Update in-memory Ginie config for instant effect
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot != nil {
		ginieConfig := giniePilot.GetConfig()
		ginieConfig.EnableUltraFastMode = currentConfig.Enabled
		giniePilot.SetConfig(ginieConfig)
	}

	// Also update SettingsManager in-memory
	sm.UpdateModeConfig("ultra_fast", currentConfig)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ultra-fast configuration updated - takes effect immediately",
		"mode":    "ultra_fast",
		"enabled": currentConfig.Enabled,
		"config":  currentConfig,
		"source":  "database",
	})
}

// handleToggleUltraFast toggles ultra-fast mode on/off
// POST /api/futures/ultrafast/toggle
// DB-FIRST: Saves enabled status to database AND updates in-memory for instant effect
func (s *Server) handleToggleUltraFast(c *gin.Context) {
	log.Println("[ULTRAFAST] Toggling ultra-fast mode (DB-first)")

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Get user ID from JWT
	userID, exists := c.Get("user_id")
	if !exists {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}
	userIDStr := userID.(string)

	// 1. Get current config from database (or defaults)
	sm := autopilot.GetSettingsManager()
	ctx := context.Background()
	config, err := sm.GetUserModeConfigFromDB(ctx, s.repo, userIDStr, "ultra_fast")
	if err != nil || config == nil {
		// No DB config, use defaults
		config, _ = sm.GetDefaultModeConfig("ultra_fast")
	}

	// 2. Update the enabled status
	config.Enabled = req.Enabled

	// 3. Save to database (source of truth)
	configJSON, err := json.Marshal(config)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to marshal config: "+err.Error())
		return
	}
	err = s.repo.SaveUserModeConfig(ctx, userIDStr, "ultra_fast", req.Enabled, configJSON)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save to database: "+err.Error())
		return
	}
	log.Printf("[ULTRAFAST] Saved ultra_fast enabled=%v to database for user %s", req.Enabled, userIDStr)

	// 4. Update in-memory Ginie config for instant effect (no restart needed)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot != nil {
		ginieConfig := giniePilot.GetConfig()
		ginieConfig.EnableUltraFastMode = req.Enabled
		giniePilot.SetConfig(ginieConfig)
		log.Printf("[ULTRAFAST] Updated in-memory Ginie config for instant effect")
	}

	// 5. Also update SettingsManager in-memory for consistency
	sm.UpdateModeConfig("ultra_fast", config)

	log.Printf("[ULTRAFAST] Ultra-fast mode toggled to: %v (DB + in-memory)", req.Enabled)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"enabled": req.Enabled,
		"source":  "database",
		"message": fmt.Sprintf("Ultra-fast mode %s - takes effect immediately", map[bool]string{true: "enabled", false: "disabled"}[req.Enabled]),
	})
}

// ==================== Scalp Re-entry Configuration ====================

// handleGetScalpReentryConfig returns the current scalp re-entry mode configuration
// GET /api/futures/ginie/scalp-reentry-config
func (s *Server) handleGetScalpReentryConfig(c *gin.Context) {
	userID := s.getUserID(c)
	log.Printf("[SCALP-REENTRY] Getting scalp re-entry configuration for user %s", userID)

	// Try to get user's config from database
	configJSON, err := s.repo.GetUserScalpReentryConfig(c.Request.Context(), userID)
	if err != nil {
		log.Printf("[SCALP-REENTRY] Database error: %v", err)
		errorResponse(c, http.StatusInternalServerError, "Failed to retrieve configuration")
		return
	}

	var config autopilot.ScalpReentryConfig

	if configJSON == nil {
		// No config in database - use defaults
		log.Println("[SCALP-REENTRY] No config in database, using defaults")
		config = autopilot.DefaultScalpReentryConfig()
	} else {
		// Parse the config from database
		if err := json.Unmarshal(configJSON, &config); err != nil {
			log.Printf("[SCALP-REENTRY] Failed to parse config from database: %v", err)
			errorResponse(c, http.StatusInternalServerError, "Failed to parse configuration")
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  config,
	})
}

// handleUpdateScalpReentryConfig updates the scalp re-entry mode configuration
// POST /api/futures/ginie/scalp-reentry-config
func (s *Server) handleUpdateScalpReentryConfig(c *gin.Context) {
	userID := s.getUserID(c)
	log.Printf("[SCALP-REENTRY] Updating scalp re-entry configuration for user %s", userID)

	var req autopilot.ScalpReentryConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate TP levels
	if req.TP1Percent <= 0 || req.TP1Percent >= req.TP2Percent {
		errorResponse(c, http.StatusBadRequest, "TP1 percent must be positive and less than TP2")
		return
	}
	if req.TP2Percent <= req.TP1Percent || req.TP2Percent >= req.TP3Percent {
		errorResponse(c, http.StatusBadRequest, "TP2 percent must be between TP1 and TP3")
		return
	}
	if req.TP3Percent <= req.TP2Percent || req.TP3Percent > 10 {
		errorResponse(c, http.StatusBadRequest, "TP3 percent must be greater than TP2 and at most 10%")
		return
	}

	// Validate sell percentages
	if req.TP1SellPercent <= 0 || req.TP1SellPercent > 50 {
		errorResponse(c, http.StatusBadRequest, "TP1 sell percent must be 1-50%")
		return
	}
	if req.TP2SellPercent <= 0 || req.TP2SellPercent > 80 {
		errorResponse(c, http.StatusBadRequest, "TP2 sell percent must be 1-80%")
		return
	}
	if req.TP3SellPercent <= 0 || req.TP3SellPercent > 100 {
		errorResponse(c, http.StatusBadRequest, "TP3 sell percent must be 1-100%")
		return
	}

	// Validate re-entry config
	if req.ReentryPercent < 50 || req.ReentryPercent > 100 {
		errorResponse(c, http.StatusBadRequest, "Re-entry percent must be 50-100%")
		return
	}
	if req.ReentryPriceBuffer < 0 || req.ReentryPriceBuffer > 1 {
		errorResponse(c, http.StatusBadRequest, "Re-entry price buffer must be 0-1%")
		return
	}

	// Validate dynamic SL config
	if req.DynamicSLProtectPct+req.DynamicSLMaxLossPct != 100 {
		errorResponse(c, http.StatusBadRequest, "Dynamic SL protect + max loss must equal 100%")
		return
	}

	// Marshal config to JSON
	configJSON, err := json.Marshal(req)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to serialize configuration")
		return
	}

	// Save to database
	if err := s.repo.SaveUserScalpReentryConfig(c.Request.Context(), userID, configJSON); err != nil {
		log.Printf("[SCALP-REENTRY] Failed to save config to database: %v", err)
		errorResponse(c, http.StatusInternalServerError, "Failed to save configuration")
		return
	}

	log.Printf("[SCALP-REENTRY] Configuration updated - Enabled: %v, TP levels: %.2f%%, %.2f%%, %.2f%%",
		req.Enabled, req.TP1Percent, req.TP2Percent, req.TP3Percent)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  req,
		"message": "Scalp re-entry configuration updated",
	})
}

// handleToggleScalpReentry toggles the scalp re-entry mode on/off
// POST /api/futures/ginie/scalp-reentry/toggle
func (s *Server) handleToggleScalpReentry(c *gin.Context) {
	log.Println("[SCALP-REENTRY] Toggling scalp re-entry mode")

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetDefaultSettings()
	settings.ScalpReentryConfig.Enabled = req.Enabled

	if err := sm.SaveSettings(settings); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save settings: "+err.Error())
		return
	}

	status := "disabled"
	if req.Enabled {
		status = "enabled"
	}
	log.Printf("[SCALP-REENTRY] Mode toggled to: %s", status)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"enabled": req.Enabled,
		"message": fmt.Sprintf("Scalp re-entry mode %s", status),
	})
}

// ==================== Hedge Mode Configuration Endpoints ====================

// HedgeModeConfigRequest represents the hedge mode configuration for API
type HedgeModeConfigRequest struct {
	HedgeModeEnabled      bool    `json:"hedge_mode_enabled"`
	TriggerOnProfitTP     bool    `json:"trigger_on_profit_tp"`
	TriggerOnLossTP       bool    `json:"trigger_on_loss_tp"`
	DCAOnLoss             bool    `json:"dca_on_loss"`
	MaxPositionMultiple   float64 `json:"max_position_multiple"`
	CombinedROIExitPct    float64 `json:"combined_roi_exit_pct"`
	WideSLATRMultiplier   float64 `json:"wide_sl_atr_multiplier"`
	DisableAISL           bool    `json:"disable_ai_sl"`
	RallyExitEnabled      bool    `json:"rally_exit_enabled"`
	RallyADXThreshold     float64 `json:"rally_adx_threshold"`
	RallySustainedMovePct float64 `json:"rally_sustained_move_pct"`
	NegTP1Percent         float64 `json:"neg_tp1_percent"`
	NegTP1AddPercent      float64 `json:"neg_tp1_add_percent"`
	NegTP2Percent         float64 `json:"neg_tp2_percent"`
	NegTP2AddPercent      float64 `json:"neg_tp2_add_percent"`
	NegTP3Percent         float64 `json:"neg_tp3_percent"`
	NegTP3AddPercent      float64 `json:"neg_tp3_add_percent"`
}

// handleGetHedgeModeConfig returns the current hedge mode configuration
// GET /api/futures/ginie/hedge-config
func (s *Server) handleGetHedgeModeConfig(c *gin.Context) {
	log.Println("[HEDGE-MODE] Getting hedge mode configuration")

	sm := autopilot.GetSettingsManager()
	settings := sm.GetDefaultSettings()
	config := settings.ScalpReentryConfig

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config": HedgeModeConfigRequest{
			HedgeModeEnabled:      config.HedgeModeEnabled,
			TriggerOnProfitTP:     config.TriggerOnProfitTP,
			TriggerOnLossTP:       config.TriggerOnLossTP,
			DCAOnLoss:             config.DCAOnLoss,
			MaxPositionMultiple:   config.MaxPositionMultiple,
			CombinedROIExitPct:    config.CombinedROIExitPct,
			WideSLATRMultiplier:   config.WideSLATRMultiplier,
			DisableAISL:           config.DisableAISL,
			RallyExitEnabled:      config.RallyExitEnabled,
			RallyADXThreshold:     config.RallyADXThreshold,
			RallySustainedMovePct: config.RallySustainedMovePct,
			NegTP1Percent:         config.NegTP1Percent,
			NegTP1AddPercent:      config.NegTP1AddPercent,
			NegTP2Percent:         config.NegTP2Percent,
			NegTP2AddPercent:      config.NegTP2AddPercent,
			NegTP3Percent:         config.NegTP3Percent,
			NegTP3AddPercent:      config.NegTP3AddPercent,
		},
	})
}

// handleUpdateHedgeModeConfig updates the hedge mode configuration
// POST /api/futures/ginie/hedge-config
func (s *Server) handleUpdateHedgeModeConfig(c *gin.Context) {
	log.Println("[HEDGE-MODE] Updating hedge mode configuration")

	// Parse request as raw JSON to detect which fields are set
	var rawReq map[string]interface{}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Get existing settings
	sm := autopilot.GetSettingsManager()
	settings := sm.GetDefaultSettings()
	cfg := &settings.ScalpReentryConfig

	// Helper to check if field was provided
	hasField := func(name string) bool {
		_, ok := rawReq[name]
		return ok
	}

	// Update only provided fields with validation
	if hasField("hedge_mode_enabled") {
		cfg.HedgeModeEnabled = rawReq["hedge_mode_enabled"].(bool)
	}
	if hasField("trigger_on_profit_tp") {
		cfg.TriggerOnProfitTP = rawReq["trigger_on_profit_tp"].(bool)
	}
	if hasField("trigger_on_loss_tp") {
		cfg.TriggerOnLossTP = rawReq["trigger_on_loss_tp"].(bool)
	}
	if hasField("dca_on_loss") {
		cfg.DCAOnLoss = rawReq["dca_on_loss"].(bool)
	}
	if hasField("disable_ai_sl") {
		cfg.DisableAISL = rawReq["disable_ai_sl"].(bool)
	}
	if hasField("rally_exit_enabled") {
		cfg.RallyExitEnabled = rawReq["rally_exit_enabled"].(bool)
	}
	if hasField("max_position_multiple") {
		v := rawReq["max_position_multiple"].(float64)
		if v < 1.5 || v > 10.0 {
			errorResponse(c, http.StatusBadRequest, "max_position_multiple must be between 1.5 and 10.0")
			return
		}
		cfg.MaxPositionMultiple = v
	}
	if hasField("combined_roi_exit_pct") {
		v := rawReq["combined_roi_exit_pct"].(float64)
		if v < 0.5 || v > 20.0 {
			errorResponse(c, http.StatusBadRequest, "combined_roi_exit_pct must be between 0.5 and 20.0")
			return
		}
		cfg.CombinedROIExitPct = v
	}
	if hasField("wide_sl_atr_multiplier") {
		v := rawReq["wide_sl_atr_multiplier"].(float64)
		if v < 1.5 || v > 5.0 {
			errorResponse(c, http.StatusBadRequest, "wide_sl_atr_multiplier must be between 1.5 and 5.0")
			return
		}
		cfg.WideSLATRMultiplier = v
	}
	if hasField("rally_adx_threshold") {
		v := rawReq["rally_adx_threshold"].(float64)
		if v < 15.0 || v > 50.0 {
			errorResponse(c, http.StatusBadRequest, "rally_adx_threshold must be between 15.0 and 50.0")
			return
		}
		cfg.RallyADXThreshold = v
	}
	if hasField("rally_sustained_move_pct") {
		v := rawReq["rally_sustained_move_pct"].(float64)
		if v < 1.0 || v > 10.0 {
			errorResponse(c, http.StatusBadRequest, "rally_sustained_move_pct must be between 1.0 and 10.0")
			return
		}
		cfg.RallySustainedMovePct = v
	}
	if hasField("neg_tp1_percent") {
		cfg.NegTP1Percent = rawReq["neg_tp1_percent"].(float64)
	}
	if hasField("neg_tp1_add_percent") {
		cfg.NegTP1AddPercent = rawReq["neg_tp1_add_percent"].(float64)
	}
	if hasField("neg_tp2_percent") {
		cfg.NegTP2Percent = rawReq["neg_tp2_percent"].(float64)
	}
	if hasField("neg_tp2_add_percent") {
		cfg.NegTP2AddPercent = rawReq["neg_tp2_add_percent"].(float64)
	}
	if hasField("neg_tp3_percent") {
		cfg.NegTP3Percent = rawReq["neg_tp3_percent"].(float64)
	}
	if hasField("neg_tp3_add_percent") {
		cfg.NegTP3AddPercent = rawReq["neg_tp3_add_percent"].(float64)
	}

	if err := sm.SaveSettings(settings); err != nil {
		log.Printf("[HEDGE-MODE] Failed to save hedge mode configuration: %v", err)
		errorResponse(c, http.StatusInternalServerError, "Failed to save configuration: "+err.Error())
		return
	}

	log.Printf("[HEDGE-MODE] Configuration updated: enabled=%v, combined_roi_exit=%.2f%%, max_pos=%.1fx",
		cfg.HedgeModeEnabled, cfg.CombinedROIExitPct, cfg.MaxPositionMultiple)

	// Return updated config
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Hedge mode configuration updated",
		"config": gin.H{
			"hedge_mode_enabled":      cfg.HedgeModeEnabled,
			"trigger_on_profit_tp":    cfg.TriggerOnProfitTP,
			"trigger_on_loss_tp":      cfg.TriggerOnLossTP,
			"dca_on_loss":             cfg.DCAOnLoss,
			"max_position_multiple":   cfg.MaxPositionMultiple,
			"combined_roi_exit_pct":   cfg.CombinedROIExitPct,
			"wide_sl_atr_multiplier":  cfg.WideSLATRMultiplier,
			"disable_ai_sl":           cfg.DisableAISL,
			"rally_exit_enabled":      cfg.RallyExitEnabled,
			"rally_adx_threshold":     cfg.RallyADXThreshold,
			"rally_sustained_move_pct": cfg.RallySustainedMovePct,
			"neg_tp1_percent":         cfg.NegTP1Percent,
			"neg_tp1_add_percent":     cfg.NegTP1AddPercent,
			"neg_tp2_percent":         cfg.NegTP2Percent,
			"neg_tp2_add_percent":     cfg.NegTP2AddPercent,
			"neg_tp3_percent":         cfg.NegTP3Percent,
			"neg_tp3_add_percent":     cfg.NegTP3AddPercent,
		},
	})
}

// handleToggleHedgeMode toggles the hedge mode on/off
// POST /api/futures/ginie/hedge-mode/toggle
func (s *Server) handleToggleHedgeMode(c *gin.Context) {
	log.Println("[HEDGE-MODE] Toggling hedge mode")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetDefaultSettings()
	settings.ScalpReentryConfig.HedgeModeEnabled = req.Enabled

	if err := sm.SaveSettings(settings); err != nil {
		log.Printf("[HEDGE-MODE] Failed to toggle hedge mode: %v", err)
		errorResponse(c, http.StatusInternalServerError, "Failed to save settings: "+err.Error())
		return
	}

	status := "disabled"
	if req.Enabled {
		status = "enabled"
	}
	log.Printf("[HEDGE-MODE] Mode toggled to: %s", status)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"enabled": req.Enabled,
		"message": fmt.Sprintf("Hedge mode %s", status),
	})
}

// handleGetHedgeModePositions returns positions with active hedge mode state
// GET /api/futures/ginie/hedge-mode/positions
func (s *Server) handleGetHedgeModePositions(c *gin.Context) {
	log.Println("[HEDGE-MODE] Getting hedge mode positions")

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available")
		return
	}

	allPositions := giniePilot.GetPositions()
	var hedgePositions []gin.H

	for _, pos := range allPositions {
		if pos.ScalpReentry == nil || pos.ScalpReentry.HedgeMode == nil {
			continue
		}

		hm := pos.ScalpReentry.HedgeMode
		sr := pos.ScalpReentry
		currentPrice := giniePilot.GetCurrentPrice(pos.Symbol)

		// Calculate current PnL for both sides
		var originalUnrealized float64
		if pos.Side == "LONG" {
			originalUnrealized = (currentPrice - sr.CurrentBreakeven) * sr.RemainingQuantity
		} else {
			originalUnrealized = (sr.CurrentBreakeven - currentPrice) * sr.RemainingQuantity
		}

		var hedgeUnrealized float64
		if hm.HedgeActive {
			if hm.HedgeSide == "LONG" {
				hedgeUnrealized = (currentPrice - hm.HedgeCurrentBE) * hm.HedgeRemainingQty
			} else {
				hedgeUnrealized = (hm.HedgeCurrentBE - currentPrice) * hm.HedgeRemainingQty
			}
		}

		hedgePositions = append(hedgePositions, gin.H{
			"symbol":        pos.Symbol,
			"original_side": pos.Side,
			"entry_price":   pos.EntryPrice,
			"current_price": currentPrice,
			"original": gin.H{
				"remaining_qty":    sr.RemainingQuantity,
				"current_be":       sr.CurrentBreakeven,
				"tp_level":         sr.TPLevelUnlocked,
				"accum_profit":     sr.AccumulatedProfit,
				"unrealized_pnl":   originalUnrealized,
			},
			"hedge": gin.H{
				"active":           hm.HedgeActive,
				"side":             hm.HedgeSide,
				"entry_price":      hm.HedgeEntryPrice,
				"remaining_qty":    hm.HedgeRemainingQty,
				"current_be":       hm.HedgeCurrentBE,
				"tp_level":         hm.HedgeTPLevel,
				"accum_profit":     hm.HedgeAccumProfit,
				"unrealized_pnl":   hedgeUnrealized,
				"trigger_type":     hm.TriggerType,
			},
			"combined": gin.H{
				"roi_percent":      hm.CombinedROIPercent,
				"realized_pnl":     hm.CombinedRealizedPnL,
				"unrealized_pnl":   hm.CombinedUnrealizedPnL,
				"total_pnl":        hm.CombinedRealizedPnL + hm.CombinedUnrealizedPnL,
			},
			"dca": gin.H{
				"enabled":          hm.DCAEnabled,
				"additions_count":  len(hm.DCAAdditions),
				"total_qty":        hm.OriginalTotalQty,
				"neg_tp_triggered": hm.NegTPLevelTriggered,
			},
			"wide_sl": gin.H{
				"price":            hm.WideSLPrice,
				"atr_multiplier":   hm.WideSLATRMultiplier,
				"ai_blocked":       hm.AICannotTriggerSL,
			},
			"debug_log": hm.DebugLog,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"count":     len(hedgePositions),
		"positions": hedgePositions,
	})
}

// ==================== Scalp Re-entry Monitor Endpoints ====================

// ScalpReentryPositionStatus provides enhanced status for a position in scalp_reentry mode
type ScalpReentryPositionStatus struct {
	Symbol             string                       `json:"symbol"`
	Side               string                       `json:"side"`
	Mode               string                       `json:"mode"`
	EntryPrice         float64                      `json:"entry_price"`
	CurrentPrice       float64                      `json:"current_price"`
	UnrealizedPnL      float64                      `json:"unrealized_pnl"`
	UnrealizedPnLPct   float64                      `json:"unrealized_pnl_pct"`

	// Scalp Re-entry specific fields
	ScalpReentryActive bool                         `json:"scalp_reentry_active"`
	TPLevelUnlocked    int                          `json:"tp_level_unlocked"`     // 0, 1, 2, or 3
	NextTPLevel        int                          `json:"next_tp_level"`         // Next target TP
	NextTPPercent      float64                      `json:"next_tp_percent"`       // Target % for next TP
	NextTPBlocked      bool                         `json:"next_tp_blocked"`       // Waiting for reentry

	// Current cycle info
	CurrentCycleNum    int                          `json:"current_cycle_num"`
	CurrentCycleState  string                       `json:"current_cycle_state"`   // WAITING, EXECUTING, COMPLETED, etc
	ReentryTargetPrice float64                      `json:"reentry_target_price"`  // Breakeven target
	DistanceToReentry  float64                      `json:"distance_to_reentry"`   // % distance to reentry price

	// Accumulated stats
	AccumulatedProfit  float64                      `json:"accumulated_profit"`
	TotalCycles        int                          `json:"total_cycles"`
	SuccessfulReentries int                         `json:"successful_reentries"`
	SkippedReentries   int                          `json:"skipped_reentries"`

	// Final portion tracking
	FinalPortionActive bool                         `json:"final_portion_active"`
	FinalPortionQty    float64                      `json:"final_portion_qty"`
	FinalTrailingPeak  float64                      `json:"final_trailing_peak"`
	DynamicSLActive    bool                         `json:"dynamic_sl_active"`
	DynamicSLPrice     float64                      `json:"dynamic_sl_price"`

	// Cycle history
	Cycles             []ScalpReentryCycleInfo      `json:"cycles"`

	// Debug info
	LastUpdate         string                       `json:"last_update"`

	// Hedge mode status
	HedgeModeActive    bool                         `json:"hedge_mode_active"`
	HedgeSide          string                       `json:"hedge_side,omitempty"`
}

// ScalpReentryCycleInfo provides info about a single sell->buyback cycle
type ScalpReentryCycleInfo struct {
	CycleNumber    int     `json:"cycle_number"`
	TPLevel        int     `json:"tp_level"`
	State          string  `json:"state"`           // NONE, WAITING, EXECUTING, COMPLETED, FAILED, SKIPPED

	// Sell info
	SellPrice      float64 `json:"sell_price"`
	SellQuantity   float64 `json:"sell_quantity"`
	SellPnL        float64 `json:"sell_pnl"`
	SellTime       string  `json:"sell_time"`

	// Reentry info
	ReentryTarget  float64 `json:"reentry_target"`
	ReentryFilled  float64 `json:"reentry_filled"`
	ReentryPrice   float64 `json:"reentry_price"`
	ReentryTime    string  `json:"reentry_time"`

	// Outcome
	Outcome        string  `json:"outcome"`         // profit, loss, skipped, pending
	OutcomePnL     float64 `json:"outcome_pnl"`
	OutcomeReason  string  `json:"outcome_reason"`

	// AI Decision
	AIReasoning    string  `json:"ai_reasoning,omitempty"`
	AIConfidence   float64 `json:"ai_confidence,omitempty"`
}

// handleGetScalpReentryPositions returns all positions in scalp_reentry mode with enhanced status
// GET /api/futures/ginie/scalp-reentry/positions
func (s *Server) handleGetScalpReentryPositions(c *gin.Context) {
	log.Println("[SCALP-REENTRY-MONITOR] Getting scalp re-entry positions")

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available")
		return
	}

	// Get all positions
	allPositions := giniePilot.GetPositions()

	// Get scalp reentry config for TP level info
	sm := autopilot.GetSettingsManager()
	settings := sm.GetDefaultSettings()
	config := settings.ScalpReentryConfig

	// Filter and enhance positions with scalp_reentry mode or ScalpReentry status
	var scalpPositions []ScalpReentryPositionStatus

	for _, pos := range allPositions {
		// Include if mode is scalp_reentry OR has ScalpReentry tracking
		if pos.Mode != autopilot.GinieModeScalpReentry && pos.ScalpReentry == nil {
			continue
		}

		// Get current price from Ginie
		currentPrice := giniePilot.GetCurrentPrice(pos.Symbol)
		if currentPrice == 0 {
			currentPrice = pos.EntryPrice // Fallback
		}

		// Calculate unrealized PnL
		var unrealizedPnL, unrealizedPnLPct float64
		if pos.Side == "LONG" {
			unrealizedPnL = (currentPrice - pos.EntryPrice) * pos.RemainingQty
			unrealizedPnLPct = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
		} else {
			unrealizedPnL = (pos.EntryPrice - currentPrice) * pos.RemainingQty
			unrealizedPnLPct = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
		}

		status := ScalpReentryPositionStatus{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			Mode:             string(pos.Mode),
			EntryPrice:       pos.EntryPrice,
			CurrentPrice:     currentPrice,
			UnrealizedPnL:    unrealizedPnL,
			UnrealizedPnLPct: unrealizedPnLPct,
		}

		// Populate scalp reentry specific fields if available
		if pos.ScalpReentry != nil {
			sr := pos.ScalpReentry
			status.ScalpReentryActive = sr.Enabled
			status.TPLevelUnlocked = sr.TPLevelUnlocked
			status.NextTPLevel = sr.TPLevelUnlocked + 1
			status.NextTPBlocked = sr.NextTPBlocked
			status.CurrentCycleNum = sr.CurrentCycle
			status.AccumulatedProfit = sr.AccumulatedProfit
			status.TotalCycles = len(sr.Cycles)
			status.SuccessfulReentries = sr.SuccessfulReentries
			status.SkippedReentries = sr.SkippedReentries
			status.FinalPortionActive = sr.FinalPortionActive
			status.FinalPortionQty = sr.FinalPortionQty
			status.FinalTrailingPeak = sr.FinalTrailingPeak
			status.DynamicSLActive = sr.DynamicSLActive
			status.DynamicSLPrice = sr.DynamicSLPrice
			status.LastUpdate = sr.LastUpdate.Format("2006-01-02 15:04:05")

			// Get next TP percent from config
			if status.NextTPLevel <= 3 {
				tpPct, _ := config.GetTPConfig(status.NextTPLevel)
				status.NextTPPercent = tpPct
			}

			// Get current cycle state
			if cycle := sr.GetCurrentCycle(); cycle != nil {
				status.CurrentCycleState = string(cycle.ReentryState)
				status.ReentryTargetPrice = cycle.ReentryTargetPrice

				// Calculate distance to reentry
				if cycle.ReentryTargetPrice > 0 {
					if pos.Side == "LONG" {
						status.DistanceToReentry = (currentPrice - cycle.ReentryTargetPrice) / cycle.ReentryTargetPrice * 100
					} else {
						status.DistanceToReentry = (cycle.ReentryTargetPrice - currentPrice) / cycle.ReentryTargetPrice * 100
					}
				}
			}

			// Convert cycles to simplified info
			for _, cycle := range sr.Cycles {
				cycleInfo := ScalpReentryCycleInfo{
					CycleNumber:   cycle.CycleNumber,
					TPLevel:       cycle.TPLevel,
					State:         string(cycle.ReentryState),
					SellPrice:     cycle.SellPrice,
					SellQuantity:  cycle.SellQuantity,
					SellPnL:       cycle.SellPnL,
					ReentryTarget: cycle.ReentryTargetPrice,
					ReentryFilled: cycle.ReentryFilledQty,
					ReentryPrice:  cycle.ReentryFilledPrice,
					Outcome:       cycle.Outcome,
					OutcomePnL:    cycle.OutcomePnL,
					OutcomeReason: cycle.OutcomeReason,
				}

				if !cycle.SellTime.IsZero() {
					cycleInfo.SellTime = cycle.SellTime.Format("15:04:05")
				}
				if !cycle.ReentryFillTime.IsZero() {
					cycleInfo.ReentryTime = cycle.ReentryFillTime.Format("15:04:05")
				}

				// Include AI decision info if available
				if cycle.AIDecision != nil {
					cycleInfo.AIReasoning = cycle.AIDecision.Reasoning
					cycleInfo.AIConfidence = cycle.AIDecision.Confidence
				}

				status.Cycles = append(status.Cycles, cycleInfo)
			}
		} else {
			// Position is in scalp_reentry mode but not yet initialized
			status.ScalpReentryActive = false
			status.CurrentCycleState = "INITIALIZING"
		}

		// Check hedge mode status
		if pos.ScalpReentry != nil && pos.ScalpReentry.HedgeMode != nil {
			hm := pos.ScalpReentry.HedgeMode
			status.HedgeModeActive = hm.HedgeActive
			if hm.HedgeActive {
				status.HedgeSide = hm.HedgeSide
			}
		}

		scalpPositions = append(scalpPositions, status)
	}

	// Get global stats
	totalAccumulatedProfit := 0.0
	totalCycles := 0
	totalReentries := 0
	for _, pos := range scalpPositions {
		totalAccumulatedProfit += pos.AccumulatedProfit
		totalCycles += pos.TotalCycles
		totalReentries += pos.SuccessfulReentries
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"positions": scalpPositions,
		"count":     len(scalpPositions),
		"summary": gin.H{
			"total_positions":      len(scalpPositions),
			"total_accumulated_pnl": totalAccumulatedProfit,
			"total_cycles":         totalCycles,
			"total_reentries":      totalReentries,
			"config_enabled":       config.Enabled,
		},
	})
}

// handleGetScalpReentryPositionStatus returns detailed status for a single position
// GET /api/futures/ginie/scalp-reentry/positions/:symbol
func (s *Server) handleGetScalpReentryPositionStatus(c *gin.Context) {
	symbol := c.Param("symbol")
	log.Printf("[SCALP-REENTRY-MONITOR] Getting status for %s", symbol)

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available")
		return
	}

	// Find the position
	positions := giniePilot.GetPositions()
	var targetPos *autopilot.GiniePosition
	for _, pos := range positions {
		if pos.Symbol == symbol {
			targetPos = pos
			break
		}
	}

	if targetPos == nil {
		errorResponse(c, http.StatusNotFound, fmt.Sprintf("Position %s not found", symbol))
		return
	}

	if targetPos.ScalpReentry == nil {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Position %s is not in scalp_reentry mode", symbol))
		return
	}

	sr := targetPos.ScalpReentry
	sm := autopilot.GetSettingsManager()
	config := sm.GetDefaultSettings().ScalpReentryConfig
	currentPrice := giniePilot.GetCurrentPrice(symbol)

	// Build detailed response
	var unrealizedPnL, unrealizedPnLPct float64
	if targetPos.Side == "LONG" {
		unrealizedPnL = (currentPrice - targetPos.EntryPrice) * targetPos.RemainingQty
		unrealizedPnLPct = (currentPrice - targetPos.EntryPrice) / targetPos.EntryPrice * 100
	} else {
		unrealizedPnL = (targetPos.EntryPrice - currentPrice) * targetPos.RemainingQty
		unrealizedPnLPct = (targetPos.EntryPrice - currentPrice) / targetPos.EntryPrice * 100
	}

	response := gin.H{
		"success": true,
		"symbol":  symbol,
		"side":    targetPos.Side,
		"mode":    string(targetPos.Mode),
		"entry_price": targetPos.EntryPrice,
		"current_price": currentPrice,
		"unrealized_pnl": unrealizedPnL,
		"unrealized_pnl_pct": unrealizedPnLPct,
		"original_qty": targetPos.OriginalQty,
		"remaining_qty": sr.RemainingQuantity,

		// Scalp Re-entry Status
		"scalp_reentry": gin.H{
			"enabled": sr.Enabled,
			"tp_level_unlocked": sr.TPLevelUnlocked,
			"next_tp_blocked": sr.NextTPBlocked,
			"current_cycle": sr.CurrentCycle,
			"accumulated_profit": sr.AccumulatedProfit,
			"original_entry_price": sr.OriginalEntryPrice,
			"current_breakeven": sr.CurrentBreakeven,
			"remaining_quantity": sr.RemainingQuantity,

			// Dynamic SL
			"dynamic_sl_active": sr.DynamicSLActive,
			"dynamic_sl_price": sr.DynamicSLPrice,
			"protected_profit": sr.ProtectedProfit,
			"max_allowable_loss": sr.MaxAllowableLoss,

			// Final portion
			"final_portion_active": sr.FinalPortionActive,
			"final_portion_qty": sr.FinalPortionQty,
			"final_trailing_peak": sr.FinalTrailingPeak,
			"final_trailing_percent": sr.FinalTrailingPercent,
			"final_trailing_active": sr.FinalTrailingActive,

			// Stats
			"total_cycles_completed": sr.TotalCyclesCompleted,
			"total_reentries": sr.TotalReentries,
			"successful_reentries": sr.SuccessfulReentries,
			"skipped_reentries": sr.SkippedReentries,
			"total_cycle_pnl": sr.TotalCyclePnL,

			// Timestamps
			"started_at": sr.StartedAt.Format("2006-01-02 15:04:05"),
			"last_update": sr.LastUpdate.Format("2006-01-02 15:04:05"),

			// Debug log (last 10 entries)
			"debug_log": func() []string {
				if len(sr.DebugLog) > 10 {
					return sr.DebugLog[len(sr.DebugLog)-10:]
				}
				return sr.DebugLog
			}(),
		},

		// TP Level configurations
		"tp_levels": gin.H{
			"tp1": gin.H{"percent": config.TP1Percent, "sell_percent": config.TP1SellPercent, "hit": sr.TPLevelUnlocked >= 1},
			"tp2": gin.H{"percent": config.TP2Percent, "sell_percent": config.TP2SellPercent, "hit": sr.TPLevelUnlocked >= 2},
			"tp3": gin.H{"percent": config.TP3Percent, "sell_percent": config.TP3SellPercent, "hit": sr.TPLevelUnlocked >= 3},
		},

		// Cycles detail
		"cycles": sr.Cycles,
	}

	c.JSON(http.StatusOK, response)
}

// handleConvertPositionMode converts a position from one trading mode to another
// POST /api/futures/ginie/positions/:symbol/convert-mode
func (s *Server) handleConvertPositionMode(c *gin.Context) {
	symbol := c.Param("symbol")
	log.Printf("[MODE-CONVERT] Mode conversion request for position: %s", symbol)

	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Parse request body
	var req struct {
		TargetMode string                 `json:"target_mode" binding:"required"`
		Leverage   int                    `json:"leverage,omitempty"`
		Options    map[string]interface{} `json:"options,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate target mode
	validModes := map[string]autopilot.GinieTradingMode{
		"ultra_fast":    autopilot.GinieModeUltraFast,
		"scalp":         autopilot.GinieModeScalp,
		"scalp_reentry": autopilot.GinieModeScalpReentry,
		"swing":         autopilot.GinieModeSwing,
		"position":      autopilot.GinieModePosition,
	}

	targetMode, ok := validModes[req.TargetMode]
	if !ok {
		errorResponse(c, http.StatusBadRequest,
			"Invalid target_mode. Must be one of: ultra_fast, scalp, scalp_reentry, swing, position")
		return
	}

	// Get current position mode
	currentMode, err := giniePilot.GetPositionMode(symbol)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "Position not found: "+err.Error())
		return
	}

	// Build options map
	options := make(map[string]interface{})
	if req.Leverage > 0 {
		options["leverage"] = req.Leverage
	}
	if req.Options != nil {
		for k, v := range req.Options {
			options[k] = v
		}
	}

	// Perform conversion
	updatedPos, err := giniePilot.ConvertPositionMode(symbol, targetMode, options)
	if err != nil {
		log.Printf("[MODE-CONVERT] Conversion failed for %s: %v", symbol, err)
		errorResponse(c, http.StatusBadRequest, "Mode conversion failed: "+err.Error())
		return
	}

	log.Printf("[MODE-CONVERT] Successfully converted %s from %s to %s", symbol, currentMode, targetMode)

	// Build response with position details
	tpLevels := make([]map[string]interface{}, len(updatedPos.TakeProfits))
	for i, tp := range updatedPos.TakeProfits {
		tpLevels[i] = map[string]interface{}{
			"level":    tp.Level,
			"price":    tp.Price,
			"percent":  tp.Percent,
			"gain_pct": tp.GainPct,
			"status":   tp.Status,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  fmt.Sprintf("Position %s converted from %s to %s", symbol, currentMode, targetMode),
		"symbol":   symbol,
		"old_mode": string(currentMode),
		"new_mode": string(targetMode),
		"position": gin.H{
			"symbol":              updatedPos.Symbol,
			"side":                updatedPos.Side,
			"mode":                string(updatedPos.Mode),
			"entry_price":         updatedPos.EntryPrice,
			"remaining_qty":       updatedPos.RemainingQty,
			"leverage":            updatedPos.Leverage,
			"stop_loss":           updatedPos.StopLoss,
			"take_profits":        tpLevels,
			"trailing_pct":        updatedPos.TrailingPercent,
			"trailing_activation": updatedPos.TrailingActivationPct,
		},
		"scalp_reentry": updatedPos.ScalpReentry != nil,
	})
}

// ============================================================================
// PER-COIN CONFLUENCE CONFIGURATION ENDPOINTS
// ============================================================================

// handleGetAllCoinConfluenceConfigs returns all coin confluence configurations
// GET /api/futures/ginie/coin-confluence
func (s *Server) handleGetAllCoinConfluenceConfigs(c *gin.Context) {
	sm := autopilot.GetSettingsManager()

	// Get all configs including tier defaults for common coins
	configs := sm.GetCoinConfluenceConfigWithDefaults()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"configs": configs,
		"tiers": gin.H{
			"blue_chip": "BTC, ETH - Lower ADX threshold, standard volume",
			"major_alt": "SOL, XRP, BNB, ADA, etc. - Standard settings",
			"mid_cap":   "DOGE, SHIB, etc. - Higher ADX, more volume confirmation",
			"small_cap": "Other coins - Strictest settings, all filters required",
		},
	})
}

// handleGetCoinConfluenceConfig returns confluence config for a specific coin
// GET /api/futures/ginie/coin-confluence/:symbol
func (s *Server) handleGetCoinConfluenceConfig(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	sm := autopilot.GetSettingsManager()
	config := sm.GetCoinConfluenceConfig(symbol)

	// Check if this is a custom config or tier default
	allConfigs := sm.GetAllCoinConfluenceConfigs()
	_, isCustom := allConfigs[symbol]

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"symbol":    symbol,
		"config":    config,
		"is_custom": isCustom,
		"tier":      config.Tier,
	})
}

// handleUpdateCoinConfluenceConfig updates confluence config for a specific coin
// POST /api/futures/ginie/coin-confluence/:symbol
func (s *Server) handleUpdateCoinConfluenceConfig(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	var config autopilot.CoinConfluenceConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate config values
	if config.ADXMultiplier < 0.1 || config.ADXMultiplier > 3.0 {
		errorResponse(c, http.StatusBadRequest, "ADX multiplier must be between 0.1 and 3.0")
		return
	}
	if config.VolumeMultiplier < 0.1 || config.VolumeMultiplier > 5.0 {
		errorResponse(c, http.StatusBadRequest, "Volume multiplier must be between 0.1 and 5.0")
		return
	}
	if config.MinConfluence < 1 || config.MinConfluence > 5 {
		errorResponse(c, http.StatusBadRequest, "Min confluence must be between 1 and 5")
		return
	}

	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateCoinConfluenceConfig(symbol, &config); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update config: "+err.Error())
		return
	}

	log.Printf("[COIN-CONFLUENCE] Updated config for %s: ADX×%.2f, Vol×%.2f, MinConf=%d",
		symbol, config.ADXMultiplier, config.VolumeMultiplier, config.MinConfluence)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Updated confluence config for %s", symbol),
		"symbol":  symbol,
		"config":  config,
	})
}

// handleDeleteCoinConfluenceConfig removes custom config for a coin (reverts to tier defaults)
// DELETE /api/futures/ginie/coin-confluence/:symbol
func (s *Server) handleDeleteCoinConfluenceConfig(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	sm := autopilot.GetSettingsManager()
	if err := sm.DeleteCoinConfluenceConfig(symbol); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to delete config: "+err.Error())
		return
	}

	// Get the tier default that will now be used
	tierDefault := autopilot.DefaultCoinConfluenceConfig(symbol)

	log.Printf("[COIN-CONFLUENCE] Deleted custom config for %s, reverting to %s tier defaults", symbol, tierDefault.Tier)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         fmt.Sprintf("Deleted custom config for %s, reverting to %s tier defaults", symbol, tierDefault.Tier),
		"symbol":          symbol,
		"tier":            tierDefault.Tier,
		"default_config":  tierDefault,
	})
}

// handleGetCoinTier returns the tier classification for a coin
// GET /api/futures/ginie/coin-tier/:symbol
func (s *Server) handleGetCoinTier(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	tier := autopilot.GetCoinTier(symbol)
	defaultConfig := autopilot.DefaultCoinConfluenceConfig(symbol)

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"symbol":         symbol,
		"tier":           tier,
		"default_config": defaultConfig,
		"description": map[autopilot.CoinTier]string{
			autopilot.TierBlueChip: "Blue chip (BTC, ETH) - Lower ADX threshold, trends at lower values",
			autopilot.TierMajorAlt: "Major altcoin - Standard settings, good liquidity",
			autopilot.TierMidCap:   "Mid-cap coin - Needs stronger confirmation signals",
			autopilot.TierSmallCap: "Small cap - Strictest settings, all filters required",
		}[tier],
	})
}
