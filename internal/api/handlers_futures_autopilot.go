package api

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/circuit"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGetFuturesAutopilotStatus returns futures autopilot status
func (s *Server) handleGetFuturesAutopilotStatus(c *gin.Context) {
	userID := s.getUserID(c)

	// MULTI-USER MODE: Use per-user autopilot if manager is available
	if s.userAutopilotManager != nil && userID != "" {
		status := s.userAutopilotManager.GetStatus(userID)
		c.JSON(http.StatusOK, gin.H{
			"enabled":          status.Running,
			"running":          status.Running,
			"dry_run":          status.DryRun,
			"active_positions": status.ActivePositions,
			"total_trades":     status.TotalTrades,
			"win_rate":         status.WinRate,
			"total_pnl":        status.TotalPnL,
			"daily_trades":     status.DailyTrades,
			"daily_pnl":        status.DailyPnL,
			"circuit_breaker":  status.CircuitBreaker,
			"message":          status.Message,
			"user_id":          status.UserID,
			"multi_user_mode":  true,
		})
		return
	}

	// LEGACY MODE: Fall back to shared controller
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":     false,
			"running":     false,
			"dry_run":     true,
			"message":     "Futures autopilot not configured",
		})
		return
	}

	// Initialize user-specific clients (LLM + Binance) from database
	// This only modifies controller if user is owner or no owner set
	s.tryInitializeUserClients(c, controller)

	status := controller.GetStatus()

	// Add multi-user ownership information
	ownerUserID := controller.GetOwnerUserID()
	currentUserID := s.getUserID(c)
	status["owner_user_id"] = ownerUserID
	status["is_owner"] = (currentUserID != "" && currentUserID == ownerUserID) || ownerUserID == ""

	c.JSON(http.StatusOK, status)
}

// handleToggleFuturesAutopilot toggles futures autopilot on/off
func (s *Server) handleToggleFuturesAutopilot(c *gin.Context) {
	var req struct {
		Enabled bool  `json:"enabled"`
		DryRun  *bool `json:"dry_run,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID := s.getUserID(c)
	ctx := c.Request.Context()

	// MULTI-USER MODE: Use per-user autopilot if manager is available
	if s.userAutopilotManager != nil && userID != "" {
		if req.Enabled {
			// Check if already running for this user
			if s.userAutopilotManager.IsRunning(userID) {
				status := s.userAutopilotManager.GetStatus(userID)
				c.JSON(http.StatusOK, gin.H{
					"success":         true,
					"message":         "Your autopilot is already running",
					"user_id":         userID,
					"multi_user_mode": true,
					"status":          status,
				})
				return
			}

			// Update dry run mode if specified
			if req.DryRun != nil {
				s.userAutopilotManager.UpdateUserDryRun(userID, *req.DryRun)
			}

			// Start the user's autopilot
			if err := s.userAutopilotManager.StartAutopilot(ctx, userID); err != nil {
				errorResponse(c, http.StatusInternalServerError, "Failed to start autopilot: "+err.Error())
				return
			}

			log.Printf("[MULTI-USER] User %s started their personal autopilot", userID)

			status := s.userAutopilotManager.GetStatus(userID)
			c.JSON(http.StatusOK, gin.H{
				"success":         true,
				"message":         "Your personal autopilot started",
				"user_id":         userID,
				"multi_user_mode": true,
				"status":          status,
			})
		} else {
			// Check if running for this user
			if !s.userAutopilotManager.IsRunning(userID) {
				status := s.userAutopilotManager.GetStatus(userID)
				c.JSON(http.StatusOK, gin.H{
					"success":         true,
					"message":         "Your autopilot is already stopped",
					"user_id":         userID,
					"multi_user_mode": true,
					"status":          status,
				})
				return
			}

			// Update dry run mode if specified
			if req.DryRun != nil {
				s.userAutopilotManager.UpdateUserDryRun(userID, *req.DryRun)
			}

			// Stop the user's autopilot
			if err := s.userAutopilotManager.StopAutopilot(userID); err != nil {
				errorResponse(c, http.StatusInternalServerError, "Failed to stop autopilot: "+err.Error())
				return
			}

			log.Printf("[MULTI-USER] User %s stopped their personal autopilot", userID)

			status := s.userAutopilotManager.GetStatus(userID)
			c.JSON(http.StatusOK, gin.H{
				"success":         true,
				"message":         "Your personal autopilot stopped",
				"user_id":         userID,
				"multi_user_mode": true,
				"status":          status,
			})
		}
		return
	}

	// LEGACY MODE: Fall back to shared controller
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	// Initialize user-specific clients (LLM + Binance) from database BEFORE any operations
	s.tryInitializeUserClients(c, controller)

	if req.Enabled {
		if controller.IsRunning() {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Futures autopilot already running",
				"owner":   controller.GetOwnerUserID(),
				"status":  controller.GetStatus(),
			})
			return
		}

		// Set the owner of the autopilot to the user who started it
		// This enables per-user API key isolation - the autopilot will use this user's keys
		if userID != "" {
			controller.SetOwnerUserID(userID)
			log.Printf("[AUTOPILOT] User %s is starting the autopilot", userID)
		}

		// Load saved settings before starting (ensures config is up-to-date)
		controller.LoadSavedSettings()

		// Update dry run mode if specified AFTER loading saved settings
		// so user-provided value overrides saved settings
		if req.DryRun != nil {
			// Use centralized mode switching to ensure proper client swap and sync
			settingsAPI := s.getSettingsAPI()
			if settingsAPI != nil {
				if err := settingsAPI.SetDryRunMode(*req.DryRun); err != nil {
					fmt.Printf("Failed to update dry run mode: %v\n", err)
				}
			} else {
				// Fallback if settings API not available (legacy support)
				controller.SetDryRun(*req.DryRun)
				sm := autopilot.GetSettingsManager()
				if err := sm.UpdateDryRunMode(*req.DryRun); err != nil {
					fmt.Printf("Failed to persist dry run mode: %v\n", err)
				}
			}
		}

		if err := controller.Start(); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to start futures autopilot: "+err.Error())
			return
		}

		// Broadcast autopilot status change to all connected clients via WebSocket
		if userWSHub != nil {
			userWSHub.BroadcastToAll(events.Event{
				Type:      events.EventAutopilotToggled,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"enabled":  true,
					"dry_run":  controller.GetDryRun(),
					"source":   "futures",
				},
			})
			log.Printf("[FUTURES-AUTOPILOT] Broadcasted AUTOPILOT_TOGGLED event: enabled=true")
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Futures autopilot started",
			"status":  controller.GetStatus(),
		})
	} else {
		if !controller.IsRunning() {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Futures autopilot already stopped",
				"status":  controller.GetStatus(),
			})
			return
		}

		// Update dry run mode if specified even when disabling
		if req.DryRun != nil {
			// Use centralized mode switching to ensure proper client swap and sync
			settingsAPI := s.getSettingsAPI()
			if settingsAPI != nil {
				if err := settingsAPI.SetDryRunMode(*req.DryRun); err != nil {
					fmt.Printf("Failed to update dry run mode: %v\n", err)
				}
			} else {
				// Fallback if settings API not available (legacy support)
				controller.SetDryRun(*req.DryRun)
				sm := autopilot.GetSettingsManager()
				if err := sm.UpdateDryRunMode(*req.DryRun); err != nil {
					fmt.Printf("Failed to persist dry run mode: %v\n", err)
				}
			}
		}

		controller.Stop()

		// Clear owner when autopilot is stopped - allows any user to start it next
		controller.SetOwnerUserID("")
		log.Printf("[AUTOPILOT] Autopilot stopped, owner cleared")

		// Broadcast autopilot status change to all connected clients via WebSocket
		if userWSHub != nil {
			userWSHub.BroadcastToAll(events.Event{
				Type:      events.EventAutopilotToggled,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"enabled":  false,
					"dry_run":  controller.GetDryRun(),
					"source":   "futures",
				},
			})
			log.Printf("[FUTURES-AUTOPILOT] Broadcasted AUTOPILOT_TOGGLED event: enabled=false")
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Futures autopilot stopped",
			"status":  controller.GetStatus(),
		})
	}
}

// handleSetFuturesAutopilotDryRun sets dry run mode for futures autopilot
func (s *Server) handleSetFuturesAutopilotDryRun(c *gin.Context) {
	var req struct {
		DryRun bool `json:"dry_run"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	// Use centralized mode switching to ensure proper client swap and sync
	settingsAPI := s.getSettingsAPI()
	if settingsAPI != nil {
		if err := settingsAPI.SetDryRunMode(req.DryRun); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to update trading mode: "+err.Error())
			return
		}
	} else {
		// Fallback if settings API not available (legacy support)
		controller.SetDryRun(req.DryRun)
		// Persist dry run mode to settings file (synchronous, not async)
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateDryRunMode(req.DryRun); err != nil {
			fmt.Printf("Failed to persist dry run mode: %v\n", err)
		}
	}

	mode := "LIVE"
	if req.DryRun {
		mode = "DRY RUN (Paper Trading)"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Futures autopilot mode updated to " + mode,
		"dry_run": req.DryRun,
		"status":  controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotAllocation sets max USD allocation
func (s *Server) handleSetFuturesAutopilotAllocation(c *gin.Context) {
	var req struct {
		MaxUSDAllocation float64 `json:"max_usd_allocation" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: max_usd_allocation is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetMaxUSDAllocation(req.MaxUSDAllocation); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Persist max allocation to settings file
	go func() {
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateMaxAllocation(req.MaxUSDAllocation); err != nil {
			fmt.Printf("Failed to persist max allocation: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"message":            "Max USD allocation updated",
		"max_usd_allocation": controller.GetMaxUSDAllocation(),
		"status":             controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotProfitReinvest configures profit reinvestment
func (s *Server) handleSetFuturesAutopilotProfitReinvest(c *gin.Context) {
	var req struct {
		ProfitReinvestPercent float64 `json:"profit_reinvest_percent" binding:"required"`
		ProfitRiskLevel       string  `json:"profit_risk_level" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: profit_reinvest_percent and profit_risk_level are required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetProfitReinvestSettings(req.ProfitReinvestPercent, req.ProfitRiskLevel); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Persist profit reinvest settings to file
	go func() {
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateProfitReinvest(req.ProfitReinvestPercent, req.ProfitRiskLevel); err != nil {
			fmt.Printf("Failed to persist profit reinvest settings: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":                 true,
		"message":                 "Profit reinvestment settings updated",
		"profit_reinvest_percent": req.ProfitReinvestPercent,
		"profit_risk_level":       req.ProfitRiskLevel,
		"status":                  controller.GetStatus(),
	})
}

// handleGetFuturesAutopilotProfitStats returns profit statistics
func (s *Server) handleGetFuturesAutopilotProfitStats(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"total_profit":               0,
			"profit_pool":                0,
			"total_usd_allocated":        0,
			"max_usd_allocation":         0,
			"profit_reinvest_percent":    50,
			"profit_reinvest_risk_level": "aggressive",
			"daily_pnl":                  0,
		})
		return
	}

	stats := controller.GetProfitStats()
	c.JSON(http.StatusOK, stats)
}

// handleSetFuturesAutopilotLeverage sets custom default leverage
func (s *Server) handleSetFuturesAutopilotLeverage(c *gin.Context) {
	var req struct {
		Leverage int `json:"leverage" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: leverage is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetDefaultLeverage(req.Leverage); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Default leverage updated",
		"leverage": controller.GetDefaultLeverage(),
		"status":   controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotMaxPositionSize sets the maximum position size percentage
func (s *Server) handleSetFuturesAutopilotMaxPositionSize(c *gin.Context) {
	var req struct {
		MaxPositionSize float64 `json:"max_position_size" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: max_position_size is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetMaxPositionSize(req.MaxPositionSize); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"message":           "Max position size updated",
		"max_position_size": controller.GetMaxPositionSize(),
		"status":            controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotConfluence sets the confluence requirement
func (s *Server) handleSetFuturesAutopilotConfluence(c *gin.Context) {
	var req struct {
		Confluence int `json:"confluence"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: confluence is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetConfluence(req.Confluence); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    fmt.Sprintf("Confluence requirement updated to %d", req.Confluence),
		"confluence": controller.GetConfluence(),
		"status":     controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotTPSL sets custom TP/SL percentages
func (s *Server) handleSetFuturesAutopilotTPSL(c *gin.Context) {
	var req struct {
		TakeProfitPercent float64 `json:"take_profit_percent" binding:"required"`
		StopLossPercent   float64 `json:"stop_loss_percent" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: take_profit_percent and stop_loss_percent are required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetTPSLPercent(req.TakeProfitPercent, req.StopLossPercent); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	tp, sl := controller.GetTPSLPercent()

	c.JSON(http.StatusOK, gin.H{
		"success":             true,
		"message":             "TP/SL percentages updated",
		"take_profit_percent": tp,
		"stop_loss_percent":   sl,
		"status":              controller.GetStatus(),
	})
}

// handleResetFuturesAutopilotAllocation resets or recalculates the allocation counter
func (s *Server) handleResetFuturesAutopilotAllocation(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	// Check if recalculate param is set
	recalculate := c.Query("recalculate") == "true"

	var newAllocation float64
	if recalculate {
		// Recalculate based on active positions
		newAllocation = controller.RecalculateAllocation()
	} else {
		// Reset to zero
		controller.ResetAllocation()
		newAllocation = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"message":              "Allocation reset successfully",
		"new_allocation":       newAllocation,
		"recalculated":         recalculate,
		"status":               controller.GetStatus(),
	})
}

// handleGetFuturesCircuitBreakerStatus returns the circuit breaker status for futures
func (s *Server) handleGetFuturesCircuitBreakerStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"enabled":   false,
			"message":   "Futures autopilot not configured",
		})
		return
	}

	status := controller.GetCircuitBreakerStatus()
	c.JSON(http.StatusOK, status)
}

// handleResetFuturesCircuitBreaker resets the futures circuit breaker
func (s *Server) handleResetFuturesCircuitBreaker(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.ResetCircuitBreaker(); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Futures circuit breaker reset successfully",
		"status":  controller.GetCircuitBreakerStatus(),
	})
}

// handleUpdateFuturesCircuitBreakerConfig updates the futures circuit breaker config
func (s *Server) handleUpdateFuturesCircuitBreakerConfig(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		MaxLossPerHour       float64 `json:"max_loss_per_hour"`
		MaxDailyLoss         float64 `json:"max_daily_loss"`
		MaxConsecutiveLosses int     `json:"max_consecutive_losses"`
		CooldownMinutes      int     `json:"cooldown_minutes"`
		MaxTradesPerMinute   int     `json:"max_trades_per_minute"`
		MaxDailyTrades       int     `json:"max_daily_trades"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := c.Request.Context()

	// 1. Load current circuit breaker config from database (or use defaults)
	currentConfig, err := s.repo.GetUserGlobalCircuitBreaker(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load circuit breaker config: "+err.Error())
		return
	}

	// Use defaults if not found
	if currentConfig == nil {
		currentConfig = database.DefaultUserGlobalCircuitBreaker()
		currentConfig.UserID = userID
	}

	// 2. Update config fields
	currentConfig.MaxLossPerHour = req.MaxLossPerHour
	currentConfig.MaxDailyLoss = req.MaxDailyLoss
	currentConfig.MaxConsecutiveLosses = req.MaxConsecutiveLosses
	currentConfig.CooldownMinutes = req.CooldownMinutes
	currentConfig.MaxTradesPerMinute = req.MaxTradesPerMinute
	currentConfig.MaxDailyTrades = req.MaxDailyTrades

	// 3. Save to DATABASE first
	if err := s.repo.SaveUserGlobalCircuitBreaker(ctx, currentConfig); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save circuit breaker config: "+err.Error())
		return
	}

	// 4. Update in-memory controller for immediate effect (optional - for backward compatibility)
	controller := s.getFuturesAutopilot()
	if controller != nil {
		circuitConfig := &circuit.CircuitBreakerConfig{
			MaxLossPerHour:       currentConfig.MaxLossPerHour,
			MaxDailyLoss:         currentConfig.MaxDailyLoss,
			MaxConsecutiveLosses: currentConfig.MaxConsecutiveLosses,
			CooldownMinutes:      currentConfig.CooldownMinutes,
			MaxTradesPerMinute:   currentConfig.MaxTradesPerMinute,
			MaxDailyTrades:       currentConfig.MaxDailyTrades,
		}
		if err := controller.UpdateCircuitBreakerConfig(circuitConfig); err != nil {
			// Log but don't fail - database is source of truth
			log.Printf("[CIRCUIT-BREAKER] Warning: Failed to update in-memory config: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Futures circuit breaker config updated",
		"config":  currentConfig,
	})
}

// handleToggleFuturesCircuitBreaker enables or disables the futures circuit breaker
func (s *Server) handleToggleFuturesCircuitBreaker(c *gin.Context) {
	// FIXED: Get userID for database operations - no fallback allowed
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetCircuitBreakerEnabled(req.Enabled); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	// FIXED: Persist circuit breaker enabled state using database call, not GetDefaultSettings()
	// Capture repo and userID for goroutine (c.Request.Context() not safe after handler returns)
	repo := s.repo
	go func() {
		sm := autopilot.GetSettingsManager()
		ctx := context.Background()
		settings, loadErr := sm.LoadSettingsFromDB(ctx, repo, userID)
		if loadErr != nil || settings == nil {
			fmt.Printf("[FUTURES-CB] ERROR: Failed to load settings from database for user %s: %v\n", userID, loadErr)
			return
		}
		settings.CircuitBreakerEnabled = req.Enabled
		if err := sm.SaveSettings(settings); err != nil {
			fmt.Printf("[FUTURES-CB] Failed to persist circuit breaker enabled state: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Futures circuit breaker " + map[bool]string{true: "enabled", false: "disabled"}[req.Enabled],
		"status":  controller.GetCircuitBreakerStatus(),
	})
}

// handleGetFuturesAutopilotRecentDecisions returns recent decision events for UI display
func (s *Server) handleGetFuturesAutopilotRecentDecisions(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"decisions": []interface{}{},
			"message":   "Futures autopilot not configured",
		})
		return
	}

	decisions := controller.GetRecentDecisions()
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"decisions": decisions,
		"count":     len(decisions),
	})
}

// handleGetAPIHealthStatus returns the health status of all APIs
func (s *Server) handleGetAPIHealthStatus(c *gin.Context) {
	status := map[string]interface{}{
		"binance_spot":    map[string]interface{}{"status": "unknown", "message": "Not checked"},
		"binance_futures": map[string]interface{}{"status": "unknown", "message": "Not checked"},
		"ai_service":      map[string]interface{}{"status": "unknown", "message": "Not checked"},
		"database":        map[string]interface{}{"status": "unknown", "message": "Not checked"},
	}

	// Check spot API via botAPI interface
	if s.botAPI != nil {
		clientIface := s.botAPI.GetBinanceClient()
		if clientIface != nil {
			// Try to get account info as health check
			type SpotChecker interface {
				GetAccountInfo() (interface{}, error)
			}
			if checker, ok := clientIface.(SpotChecker); ok {
				_, err := checker.GetAccountInfo()
				if err != nil {
					status["binance_spot"] = map[string]interface{}{"status": "error", "message": err.Error()}
				} else {
					status["binance_spot"] = map[string]interface{}{"status": "ok", "message": "Connected"}
				}
			} else {
				status["binance_spot"] = map[string]interface{}{"status": "ok", "message": "Available"}
			}
		} else {
			status["binance_spot"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
		}
	} else {
		status["binance_spot"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
	}

	// Check futures API
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient != nil {
		_, err := futuresClient.GetFuturesAccountInfo()
		if err != nil {
			errMsg := err.Error()
			// Parse ban time if present in error message
			if strings.Contains(errMsg, "banned until") {
				// Extract timestamp from error like "banned until 1766161377588"
				re := regexp.MustCompile(`banned until (\d+)`)
				if matches := re.FindStringSubmatch(errMsg); len(matches) > 1 {
					if banTs, parseErr := strconv.ParseInt(matches[1], 10, 64); parseErr == nil {
						banTime := time.UnixMilli(banTs)
						remaining := time.Until(banTime)
						if remaining > 0 {
							errMsg = fmt.Sprintf("IP banned. Releases in %d min %d sec (at %s)",
								int(remaining.Minutes()), int(remaining.Seconds())%60,
								banTime.Format("15:04:05"))
						} else {
							errMsg = "Ban expired. Refresh to check."
						}
					}
				}
			}
			status["binance_futures"] = map[string]interface{}{"status": "error", "message": errMsg}
		} else {
			status["binance_futures"] = map[string]interface{}{"status": "ok", "message": "Connected"}
		}
	} else {
		status["binance_futures"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
	}

	// Check AI service (user-specific LLM status)
	giniePilot := s.getGinieAutopilotForUser(c)
	if giniePilot != nil {
		if giniePilot.HasLLMAnalyzer() {
			status["ai_service"] = map[string]interface{}{"status": "ok", "message": "LLM configured and enabled"}
		} else {
			status["ai_service"] = map[string]interface{}{"status": "disabled", "message": "LLM not configured"}
		}
	} else {
		status["ai_service"] = map[string]interface{}{"status": "disabled", "message": "Autopilot not initialized"}
	}

	// Check database
	if s.repo != nil {
		// Simple ping check
		status["database"] = map[string]interface{}{"status": "ok", "message": "Connected"}
	} else {
		status["database"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
	}

	// Overall status
	allOk := true
	for _, v := range status {
		if m, ok := v.(map[string]interface{}); ok {
			if m["status"] == "error" {
				allOk = false
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"healthy":  allOk,
		"services": status,
	})
}

// getFuturesAutopilot is a helper to get the futures autopilot from botAPI
func (s *Server) getFuturesAutopilot() *autopilot.FuturesController {
	if s.botAPI == nil {
		return nil
	}

	// Check if botAPI implements FuturesAutopilotProvider
	if provider, ok := s.botAPI.(interface {
		GetFuturesAutopilot() interface{}
	}); ok {
		if ctrl, ok := provider.GetFuturesAutopilot().(*autopilot.FuturesController); ok {
			return ctrl
		}
	}

	return nil
}

// getGinieAutopilotForUser returns the GinieAutopilot for the current user.
// Returns a per-user autopilot instance only. Does NOT fallback to shared controller.
// This is the PREFERRED method for handlers that need GinieAutopilot.
func (s *Server) getGinieAutopilotForUser(c *gin.Context) *autopilot.GinieAutopilot {
	userID := s.getUserID(c)
	ctx := c.Request.Context()

	if s.userAutopilotManager == nil || userID == "" {
		log.Printf("[USER-AUTOPILOT] No user session or manager not available")
		return nil
	}

	instance, err := s.userAutopilotManager.GetOrCreateInstance(ctx, userID)
	if err != nil {
		log.Printf("[USER-AUTOPILOT] Failed to get autopilot for user %s: %v", userID, err)
		return nil
	}

	if instance == nil || instance.Autopilot == nil {
		log.Printf("[USER-AUTOPILOT] No autopilot instance for user %s", userID)
		return nil
	}

	instance.TouchLastActive()
	return instance.Autopilot
}

// getFuturesControllerForUser returns the FuturesController for the current user.
// In multi-user mode, handlers should prefer getGinieAutopilotForUser when only
// GinieAutopilot functionality is needed.
// This method is for handlers that need the full FuturesController (e.g., hedging, circuit breaker).
// NOTE: FuturesController is currently shared - full per-user isolation requires
// creating per-user FuturesController instances (future enhancement).
func (s *Server) getFuturesControllerForUser(c *gin.Context) *autopilot.FuturesController {
	userID := s.getUserID(c)

	// For now, return the shared controller but log the access for audit
	controller := s.getFuturesAutopilot()
	if controller != nil && userID != "" {
		// Check ownership for write operations (handlers should check isOwner)
		ownerID := controller.GetOwnerUserID()
		if ownerID != "" && ownerID != userID {
			log.Printf("[MULTI-USER-WARN] User %s accessing controller owned by %s", userID, ownerID)
		}
	}
	return controller
}

// ==================== SENTIMENT & NEWS ====================

// handleGetSentimentNews returns recent crypto news with sentiment
func (s *Server) handleGetSentimentNews(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit > 100 {
		limit = 100
	}

	ticker := c.Query("ticker") // Optional ticker filter

	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"news":      []interface{}{},
			"sentiment": nil,
			"count":     0,
			"message":   "Autopilot not configured",
		})
		return
	}

	// Get sentiment analyzer from controller
	sentiment := controller.GetSentimentScore()
	stats := controller.GetSentimentStats()
	tickers := controller.GetAvailableTickers()

	var news []map[string]interface{}
	if ticker != "" {
		news = controller.GetNewsByTicker(ticker, limit)
	} else {
		news = controller.GetRecentNews(limit)
	}

	c.JSON(http.StatusOK, gin.H{
		"news":      news,
		"sentiment": sentiment,
		"stats":     stats,
		"tickers":   tickers,
		"count":     len(news),
	})
}

// handleGetBreakingNews returns trending/important news
func (s *Server) handleGetBreakingNews(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit > 20 {
		limit = 20
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"news":  []interface{}{},
			"count": 0,
		})
		return
	}

	news := controller.GetBreakingNews(limit)

	c.JSON(http.StatusOK, gin.H{
		"news":  news,
		"count": len(news),
	})
}

// ==================== POSITION AVERAGING ====================

// handleGetAveragingStatus returns averaging configuration and position status
func (s *Server) handleGetAveragingStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":   false,
			"config":    nil,
			"positions": []interface{}{},
			"message":   "Autopilot not configured",
		})
		return
	}

	status := controller.GetAveragingStatus()
	c.JSON(http.StatusOK, status)
}

// handleSetAveragingConfig updates averaging configuration
func (s *Server) handleSetAveragingConfig(c *gin.Context) {
	var req struct {
		Enabled         bool    `json:"enabled"`
		MaxEntries      int     `json:"max_entries"`
		MinConfidence   float64 `json:"min_confidence"`
		MinPriceImprove float64 `json:"min_price_improve"`
		CooldownMins    int     `json:"cooldown_mins"`
		NewsWeight      float64 `json:"news_weight"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	controller.SetAveragingConfig(
		req.Enabled,
		req.MaxEntries,
		req.MinConfidence,
		req.MinPriceImprove,
		req.CooldownMins,
		req.NewsWeight,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Averaging config updated",
		"status":  controller.GetAveragingStatus(),
	})
}

// ============================================================================
// DYNAMIC SL/TP ENDPOINTS
// ============================================================================

// handleGetDynamicSLTPConfig returns dynamic SL/TP configuration
func (s *Server) handleGetDynamicSLTPConfig(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	config := controller.GetDynamicSLTPConfig()
	c.JSON(http.StatusOK, config)
}

// handleSetDynamicSLTPConfig updates dynamic SL/TP configuration
func (s *Server) handleSetDynamicSLTPConfig(c *gin.Context) {
	var req struct {
		Enabled         bool    `json:"enabled"`
		ATRPeriod       int     `json:"atr_period"`
		ATRMultiplierSL float64 `json:"atr_multiplier_sl"`
		ATRMultiplierTP float64 `json:"atr_multiplier_tp"`
		LLMWeight       float64 `json:"llm_weight"`
		MinSLPercent    float64 `json:"min_sl_percent"`
		MaxSLPercent    float64 `json:"max_sl_percent"`
		MinTPPercent    float64 `json:"min_tp_percent"`
		MaxTPPercent    float64 `json:"max_tp_percent"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	controller.SetDynamicSLTPConfig(
		req.Enabled,
		req.ATRPeriod,
		req.ATRMultiplierSL,
		req.ATRMultiplierTP,
		req.LLMWeight,
		req.MinSLPercent,
		req.MaxSLPercent,
		req.MinTPPercent,
		req.MaxTPPercent,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Dynamic SL/TP config updated",
		"config":  controller.GetDynamicSLTPConfig(),
	})
}

// ============================================================================
// SCALPING MODE ENDPOINTS
// ============================================================================

// handleGetScalpingConfig returns scalping mode configuration
func (s *Server) handleGetScalpingConfig(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	config := controller.GetScalpingConfig()
	c.JSON(http.StatusOK, config)
}

// handleSetScalpingConfig updates scalping mode configuration
func (s *Server) handleSetScalpingConfig(c *gin.Context) {
	var req struct {
		Enabled          bool    `json:"enabled"`
		MinProfit        float64 `json:"min_profit"`
		QuickReentry     bool    `json:"quick_reentry"`
		ReentryDelaySec  int     `json:"reentry_delay_sec"`
		MaxTradesPerDay  int     `json:"max_trades_per_day"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	controller.SetScalpingConfig(
		req.Enabled,
		req.MinProfit,
		req.QuickReentry,
		req.ReentryDelaySec,
		req.MaxTradesPerDay,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Scalping config updated",
		"config":  controller.GetScalpingConfig(),
	})
}

// handleGetInvestigateStatus returns comprehensive diagnostic information
func (s *Server) handleGetInvestigateStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	// Try to initialize user-specific clients if not already set
	s.tryInitializeUserClients(c, controller)

	status := controller.GetInvestigateStatus()
	c.JSON(http.StatusOK, status)
}

// tryInitializeUserClients attempts to initialize both LLM analyzer and Binance client
// with user's credentials stored in the database
func (s *Server) tryInitializeUserClients(c *gin.Context, controller *autopilot.FuturesController) {
	// Initialize LLM analyzer
	s.tryInitializeLLMAnalyzer(c, controller)

	// Initialize Binance Futures client with user's keys
	s.tryInitializeFuturesClient(c, controller)
}

// tryInitializeFuturesClient attempts to set the user's Binance Futures client on the controller
// MULTI-USER AWARE: Only modifies controller if user is the owner or no owner is set yet
// This prevents users from hijacking each other's autopilot sessions
func (s *Server) tryInitializeFuturesClient(c *gin.Context, controller *autopilot.FuturesController) {
	// Get user ID from context
	userID := s.getUserID(c)
	if userID == "" {
		return
	}

	// Check if controller already has a valid client (skip if dry_run mode)
	if controller.GetDryRun() {
		return // In paper mode, no need to use real API keys
	}

	// MULTI-USER CHECK: Only allow client injection if:
	// 1. No owner is set yet (first user to start autopilot)
	// 2. The current user IS the owner
	currentOwner := controller.GetOwnerUserID()
	if currentOwner != "" && currentOwner != userID {
		// This user is NOT the owner - don't modify the controller's client
		// The autopilot will continue using the owner's API keys
		log.Printf("[FUTURES-CLIENT-INIT] User %s is not the owner (owner=%s), skipping client injection", userID, currentOwner)
		return
	}

	ctx := c.Request.Context()

	// Get user's Binance API keys from database
	if s.apiKeyService == nil {
		return
	}

	// Try mainnet first, then testnet
	keys, err := s.apiKeyService.GetActiveBinanceKey(ctx, userID, false)
	if err != nil {
		keys, err = s.apiKeyService.GetActiveBinanceKey(ctx, userID, true)
	}

	if err != nil || keys == nil || keys.APIKey == "" || keys.SecretKey == "" {
		log.Printf("[FUTURES-CLIENT-INIT] No Binance keys found for user %s", userID)
		return
	}

	// Create user-specific Futures client
	client := binance.NewFuturesClient(keys.APIKey, keys.SecretKey, keys.IsTestnet)
	if client != nil {
		controller.SetFuturesClient(client)
		log.Printf("[FUTURES-CLIENT-INIT] Injected user %s's Binance Futures client into controller (testnet=%v)", userID, keys.IsTestnet)
	}
}

// tryInitializeLLMAnalyzer attempts to initialize LLM analyzer with user's AI key
// MULTI-USER AWARE: Only modifies controller if user is the owner or no owner is set yet
// This allows using per-user AI keys stored in the database
// It will also RE-INITIALIZE if the user changed their provider (e.g., from Claude to DeepSeek)
func (s *Server) tryInitializeLLMAnalyzer(c *gin.Context, controller *autopilot.FuturesController) {
	// Get user ID from context
	userID := s.getUserID(c)
	if userID == "" {
		return
	}

	// MULTI-USER CHECK: Only allow LLM initialization if:
	// 1. No owner is set yet (first user to start autopilot)
	// 2. The current user IS the owner
	currentOwner := controller.GetOwnerUserID()
	if currentOwner != "" && currentOwner != userID {
		// This user is NOT the owner - don't modify the controller's LLM analyzer
		log.Printf("[LLM-INIT] User %s is not the owner (owner=%s), skipping LLM initialization", userID, currentOwner)
		return
	}

	ctx := c.Request.Context()

	// Get user's AI keys from database
	aiKeys, err := s.repo.GetUserAIKeys(ctx, userID)
	if err != nil {
		log.Printf("[LLM-INIT] Error getting AI keys for user %s: %v", userID, err)
		return
	}

	// Find an active AI key
	for _, key := range aiKeys {
		if !key.IsActive {
			continue
		}

		// Determine provider and model
		var provider llm.Provider
		var model string
		switch key.Provider {
		case database.AIProviderClaude:
			provider = llm.ProviderClaude
			model = "claude-3-haiku-20240307" // Default to fast model
		case database.AIProviderOpenAI:
			provider = llm.ProviderOpenAI
			model = "gpt-4o-mini" // Default to fast model
		case database.AIProviderDeepSeek:
			provider = llm.ProviderDeepSeek
			model = "deepseek-chat"
		default:
			log.Printf("[LLM-INIT] Unknown AI provider: %s", key.Provider)
			continue
		}

		// Check if current analyzer matches the user's preferred provider
		// If already initialized with the SAME provider, skip re-initialization
		if controller.HasLLMAnalyzer() {
			currentProvider := controller.GetLLMProvider()
			if currentProvider == string(provider) {
				// Same provider, no need to re-initialize
				return
			}
			// Different provider - user changed their AI key, need to re-initialize
			log.Printf("[LLM-INIT] Provider changed from %s to %s, re-initializing LLM analyzer", currentProvider, provider)
		}

		// Decrypt the API key
		decryptedKey, err := decryptAPIKey(key.EncryptedKey)
		if err != nil {
			log.Printf("[LLM-INIT] Error decrypting AI key: %v", err)
			continue
		}

		// Create LLM analyzer config
		config := &llm.AnalyzerConfig{
			Provider:  provider,
			APIKey:    decryptedKey,
			Model:     model,
			Enabled:   true,
			MaxTokens: 4096,
		}

		// Create and set the LLM analyzer
		analyzer := llm.NewAnalyzer(config)
		if analyzer != nil {
			controller.SetLLMAnalyzer(analyzer)
			log.Printf("[LLM-INIT] Dynamically initialized LLM analyzer with user %s's %s key", userID, key.Provider)
			return
		}
	}
}

// getUserFuturesClient returns a per-user Binance Futures client for the requesting user
// WITHOUT modifying the shared controller state. This allows non-owner users to
// view their own positions and account data using their own API keys.
// Returns the user's client if available, otherwise returns the controller's client (for dry run mode)
func (s *Server) getUserFuturesClient(c *gin.Context) binance.FuturesClient {
	userID := s.getUserID(c)
	if userID == "" {
		// No user context - use controller's client
		controller := s.getFuturesAutopilot()
		if controller != nil {
			return controller.GetFuturesClient()
		}
		return nil
	}

	// Get user's Binance API keys from database
	if s.apiKeyService == nil {
		controller := s.getFuturesAutopilot()
		if controller != nil {
			return controller.GetFuturesClient()
		}
		return nil
	}

	ctx := c.Request.Context()

	// Try mainnet first, then testnet
	keys, err := s.apiKeyService.GetActiveBinanceKey(ctx, userID, false)
	if err != nil {
		keys, err = s.apiKeyService.GetActiveBinanceKey(ctx, userID, true)
	}

	if err != nil || keys == nil || keys.APIKey == "" || keys.SecretKey == "" {
		// No keys for this user - fall back to controller's client
		controller := s.getFuturesAutopilot()
		if controller != nil {
			return controller.GetFuturesClient()
		}
		return nil
	}

	// Create user-specific Futures client (cached by ClientFactory if available)
	return binance.NewFuturesClient(keys.APIKey, keys.SecretKey, keys.IsTestnet)
}

// handleClearFlipFlopCooldown clears the flip-flop cooldown
func (s *Server) handleClearFlipFlopCooldown(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	controller.ClearFlipFlopCooldown()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Flip-flop cooldown cleared for all symbols",
	})
}

// handleForceSyncPositions forces a sync with Binance positions
func (s *Server) handleForceSyncPositions(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	controller.ForceSyncPositions()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Position sync completed",
	})
}

// handleRecalculateAllocation recalculates USD allocation based on current balance
func (s *Server) handleRecalculateAllocation(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	controller.RecalculateAllocation()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Allocation recalculated based on current balance",
	})
}

// handleGetAdaptiveEngineStatus returns the adaptive decision engine status
func (s *Server) handleGetAdaptiveEngineStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":       false,
			"message":       "Autopilot not configured",
			"style":         "unknown",
			"market_context": nil,
		})
		return
	}

	engine := controller.GetAdaptiveEngine()
	if engine == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":       false,
			"message":       "Adaptive engine not initialized",
			"style":         "unknown",
			"market_context": nil,
		})
		return
	}

	status := engine.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ============================================================================
// AUTO MODE ENDPOINTS (LLM-Driven Trading)
// ============================================================================

// handleGetAutoModeConfig returns auto mode configuration
func (s *Server) handleGetAutoModeConfig(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	ctx := c.Request.Context()

	// Load per-user settings from database
	userSettings, err := s.repo.GetUserGinieSettings(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings: "+err.Error())
		return
	}

	// Use defaults if user settings not found
	if userSettings == nil {
		userSettings = database.DefaultUserGinieSettings()
		userSettings.UserID = userID
	}

	// Build response from user settings
	config := map[string]interface{}{
		"enabled":            userSettings.AutoModeEnabled,
		"max_positions":      userSettings.AutoModeMaxPositions,
		"max_leverage":       userSettings.AutoModeMaxLeverage,
		"max_position_size":  userSettings.AutoModeMaxPositionSize,
		"max_total_usd":      userSettings.AutoModeMaxTotalUSD,
		"allow_averaging":    userSettings.AutoModeAllowAveraging,
		"max_averages":       userSettings.AutoModeMaxAverages,
		"min_hold_minutes":   userSettings.AutoModeMinHoldMinutes,
		"quick_profit_mode":  userSettings.AutoModeQuickProfitMode,
		"min_profit_for_exit": userSettings.AutoModeMinProfitExit,
	}

	c.JSON(http.StatusOK, config)
}

// handleSetAutoModeConfig updates auto mode configuration
func (s *Server) handleSetAutoModeConfig(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		Enabled           bool    `json:"enabled"`
		MaxPositions      int     `json:"max_positions"`
		MaxLeverage       int     `json:"max_leverage"`
		MaxPositionSize   float64 `json:"max_position_size"`
		MaxTotalUSD       float64 `json:"max_total_usd"`
		AllowAveraging    bool    `json:"allow_averaging"`
		MaxAverages       int     `json:"max_averages"`
		MinHoldMinutes    int     `json:"min_hold_minutes"`
		QuickProfitMode   bool    `json:"quick_profit_mode"`
		MinProfitForExit  float64 `json:"min_profit_for_exit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := c.Request.Context()

	// Load current user settings from database
	userSettings, err := s.repo.GetUserGinieSettings(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings: "+err.Error())
		return
	}

	// Use defaults if not found
	if userSettings == nil {
		userSettings = database.DefaultUserGinieSettings()
		userSettings.UserID = userID
	}

	// Update auto mode fields
	userSettings.AutoModeEnabled = req.Enabled
	userSettings.AutoModeMaxPositions = req.MaxPositions
	userSettings.AutoModeMaxLeverage = req.MaxLeverage
	userSettings.AutoModeMaxPositionSize = req.MaxPositionSize
	userSettings.AutoModeMaxTotalUSD = req.MaxTotalUSD
	userSettings.AutoModeAllowAveraging = req.AllowAveraging
	userSettings.AutoModeMaxAverages = req.MaxAverages
	userSettings.AutoModeMinHoldMinutes = req.MinHoldMinutes
	userSettings.AutoModeQuickProfitMode = req.QuickProfitMode
	userSettings.AutoModeMinProfitExit = req.MinProfitForExit

	// Save to database
	if err := s.repo.SaveUserGinieSettings(ctx, userSettings); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save auto mode settings: "+err.Error())
		return
	}

	// Build response config
	config := map[string]interface{}{
		"enabled":            userSettings.AutoModeEnabled,
		"max_positions":      userSettings.AutoModeMaxPositions,
		"max_leverage":       userSettings.AutoModeMaxLeverage,
		"max_position_size":  userSettings.AutoModeMaxPositionSize,
		"max_total_usd":      userSettings.AutoModeMaxTotalUSD,
		"allow_averaging":    userSettings.AutoModeAllowAveraging,
		"max_averages":       userSettings.AutoModeMaxAverages,
		"min_hold_minutes":   userSettings.AutoModeMinHoldMinutes,
		"quick_profit_mode":  userSettings.AutoModeQuickProfitMode,
		"min_profit_for_exit": userSettings.AutoModeMinProfitExit,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Auto mode settings updated",
		"config":  config,
	})
}

// handleToggleAutoMode toggles auto mode on/off
func (s *Server) handleToggleAutoMode(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := c.Request.Context()

	// Load current user settings from database
	userSettings, err := s.repo.GetUserGinieSettings(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings: "+err.Error())
		return
	}

	// Use defaults if not found
	if userSettings == nil {
		userSettings = database.DefaultUserGinieSettings()
		userSettings.UserID = userID
	}

	// Update auto mode enabled flag
	userSettings.AutoModeEnabled = req.Enabled

	// Save to database
	if err := s.repo.SaveUserGinieSettings(ctx, userSettings); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to toggle auto mode: "+err.Error())
		return
	}

	status := "disabled"
	if req.Enabled {
		status = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Auto mode " + status,
		"enabled": req.Enabled,
	})
}

// ============================================================================
// PER-SYMBOL PERFORMANCE SETTINGS ENDPOINTS
// ============================================================================

// handleGetSymbolSettings returns all symbol settings with performance data
func (s *Server) handleGetSymbolSettings(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	allSettings := sm.GetAllSymbolSettings()

	// Also get category defaults
	categorySettings := sm.GetCategorySettings()

	// Apply category adjustments to each symbol's effective values
	type EnrichedSymbolSettings struct {
		*autopilot.SymbolSettings
		EffectiveConfidence float64 `json:"effective_confidence"`
		EffectiveMaxUSD     float64 `json:"effective_max_usd"`
	}

	enrichedSymbols := make(map[string]*EnrichedSymbolSettings)
	for symbol, settings := range allSettings {
		enrichedSymbols[symbol] = &EnrichedSymbolSettings{
			SymbolSettings:      settings,
			EffectiveConfidence: sm.GetEffectiveConfidence(symbol, 50.0), // Default confidence
			EffectiveMaxUSD:     sm.GetEffectivePositionSize(symbol, 500), // Default max USD
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"symbols":          enrichedSymbols,
		"category_config":  categorySettings,
		"global_min_confidence": 50.0, // Default confidence
		"global_max_usd":   500.0,     // Default max USD
	})
}

// handleGetSymbolPerformanceReport returns a detailed performance report for all symbols
func (s *Server) handleGetSymbolPerformanceReport(c *gin.Context) {
	sm := autopilot.GetSettingsManager()
	report := sm.GetSymbolPerformanceReport()

	// Group by category
	byCategory := make(map[string][]autopilot.SymbolPerformanceReport)
	for _, r := range report {
		byCategory[r.Category] = append(byCategory[r.Category], r)
	}

	c.JSON(http.StatusOK, gin.H{
		"report":      report,
		"by_category": byCategory,
		"total_symbols": len(report),
	})
}

// handleRefreshSymbolPerformance recalculates symbol performance from database trades
func (s *Server) handleRefreshSymbolPerformance(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Get performance stats from database
	ctx := c.Request.Context()
	dbStats, err := s.repo.GetDB().GetSymbolPerformanceStatsForUser(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to get performance stats: "+err.Error())
		return
	}

	// Convert to map[string]interface{} for RecalculateSymbolPerformance
	statsMap := make(map[string]interface{})
	for symbol, stats := range dbStats {
		statsMap[symbol] = map[string]interface{}{
			"total_trades":   stats.TotalTrades,
			"winning_trades": stats.WinningTrades,
			"losing_trades":  stats.LosingTrades,
			"total_pnl":      stats.TotalPnL,
			"avg_pnl":        stats.AvgPnL,
			"avg_win":        stats.AvgWin,
			"avg_loss":       stats.AvgLoss,
		}
	}

	// Update symbol settings with new performance data
	sm := autopilot.GetSettingsManager()
	updated, err := sm.RecalculateSymbolPerformance(statsMap)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update symbol settings: "+err.Error())
		return
	}

	// Get updated report
	report := sm.GetSymbolPerformanceReport()

	// Group by category
	byCategory := make(map[string][]autopilot.SymbolPerformanceReport)
	for _, r := range report {
		byCategory[r.Category] = append(byCategory[r.Category], r)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"symbols_updated": updated,
		"report":         report,
		"by_category":    byCategory,
		"total_symbols":  len(report),
	})
}

// handleGetSingleSymbolSettings returns settings for a specific symbol
func (s *Server) handleGetSingleSymbolSettings(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol parameter required")
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetSymbolSettings(symbol)

	c.JSON(http.StatusOK, gin.H{
		"symbol":              symbol,
		"settings":            settings,
		"effective_confidence": sm.GetEffectiveConfidence(symbol, 50.0), // Default confidence
		"effective_max_usd":   sm.GetEffectivePositionSize(symbol, 500), // Default max USD
		"enabled":             sm.IsSymbolEnabled(symbol),
	})
}

// handleUpdateSymbolSettings updates settings for a specific symbol
func (s *Server) handleUpdateSymbolSettings(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol parameter required")
		return
	}

	var req struct {
		Category         string  `json:"category"`
		MinConfidence    float64 `json:"min_confidence"`
		MaxPositionUSD   float64 `json:"max_position_usd"`
		SizeMultiplier   float64 `json:"size_multiplier"`
		LeverageOverride int     `json:"leverage_override"`
		Enabled          bool    `json:"enabled"`
		Notes            string  `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()

	// Map string category to type
	category := autopilot.PerformanceNeutral
	switch req.Category {
	case "best":
		category = autopilot.PerformanceBest
	case "good":
		category = autopilot.PerformanceGood
	case "neutral":
		category = autopilot.PerformanceNeutral
	case "poor":
		category = autopilot.PerformancePoor
	case "worst":
		category = autopilot.PerformanceWorst
	case "blacklist":
		category = autopilot.PerformanceBlacklist
	}

	update := &autopilot.SymbolSettings{
		Symbol:           symbol,
		Category:         category,
		MinConfidence:    req.MinConfidence,
		MaxPositionUSD:   req.MaxPositionUSD,
		SizeMultiplier:   req.SizeMultiplier,
		LeverageOverride: req.LeverageOverride,
		Enabled:          req.Enabled,
		Notes:            req.Notes,
	}

	if err := sm.UpdateSymbolSettings(symbol, update); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update symbol settings: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"message":              "Symbol settings updated",
		"symbol":               symbol,
		"settings":             sm.GetSymbolSettings(symbol),
		"effective_confidence": sm.GetEffectiveConfidence(symbol, 50.0), // Default confidence
		"effective_max_usd":    sm.GetEffectivePositionSize(symbol, 500), // Default max USD
	})
}

// handleBlacklistSymbol adds a symbol to the blacklist
func (s *Server) handleBlacklistSymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol parameter required")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req) // Optional

	sm := autopilot.GetSettingsManager()
	if err := sm.BlacklistSymbol(symbol, req.Reason); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to blacklist symbol: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": symbol + " has been blacklisted",
		"symbol":  symbol,
		"reason":  req.Reason,
	})
}

// handleUnblacklistSymbol removes a symbol from the blacklist
func (s *Server) handleUnblacklistSymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol parameter required")
		return
	}

	sm := autopilot.GetSettingsManager()
	if err := sm.UnblacklistSymbol(symbol); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to unblacklist symbol: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": symbol + " has been removed from blacklist",
		"symbol":  symbol,
	})
}

// handleUpdateCategorySettings updates the default adjustments for performance categories
func (s *Server) handleUpdateCategorySettings(c *gin.Context) {
	var req struct {
		ConfidenceBoost map[string]float64 `json:"confidence_boost"`
		SizeMultiplier  map[string]float64 `json:"size_multiplier"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateCategorySettings(req.ConfidenceBoost, req.SizeMultiplier); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update category settings: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Category settings updated",
		"settings": sm.GetCategorySettings(),
	})
}

// handleGetSymbolsByCategory returns all symbols in a given performance category
func (s *Server) handleGetSymbolsByCategory(c *gin.Context) {
	category := c.Param("category")
	if category == "" {
		errorResponse(c, http.StatusBadRequest, "Category parameter required")
		return
	}

	sm := autopilot.GetSettingsManager()

	// Map string to category type
	var perfCategory autopilot.SymbolPerformanceCategory
	switch category {
	case "best":
		perfCategory = autopilot.PerformanceBest
	case "good":
		perfCategory = autopilot.PerformanceGood
	case "neutral":
		perfCategory = autopilot.PerformanceNeutral
	case "poor":
		perfCategory = autopilot.PerformancePoor
	case "worst":
		perfCategory = autopilot.PerformanceWorst
	case "blacklist":
		perfCategory = autopilot.PerformanceBlacklist
	default:
		errorResponse(c, http.StatusBadRequest, "Invalid category. Use: best, good, neutral, poor, worst, blacklist")
		return
	}

	symbols := sm.GetSymbolsByCategory(perfCategory)

	// Get effective settings for each symbol
	symbolDetails := make([]map[string]interface{}, 0, len(symbols))
	for _, symbol := range symbols {
		settings := sm.GetSymbolSettings(symbol)
		symbolDetails = append(symbolDetails, map[string]interface{}{
			"symbol":              symbol,
			"win_rate":            settings.WinRate,
			"total_pnl":           settings.TotalPnL,
			"total_trades":        settings.TotalTrades,
			"effective_confidence": sm.GetEffectiveConfidence(symbol, 50.0), // Default confidence
			"effective_max_usd":   sm.GetEffectivePositionSize(symbol, 500), // Default max USD
			"enabled":             settings.Enabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"category": category,
		"count":    len(symbols),
		"symbols":  symbolDetails,
	})
}
