package api

import (
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/circuit"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGetFuturesAutopilotStatus returns futures autopilot status
func (s *Server) handleGetFuturesAutopilotStatus(c *gin.Context) {
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

	status := controller.GetStatus()
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

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if req.Enabled {
		if controller.IsRunning() {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Futures autopilot already running",
				"status":  controller.GetStatus(),
			})
			return
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

// handleSetFuturesAutopilotRiskLevel changes the risk level
func (s *Server) handleSetFuturesAutopilotRiskLevel(c *gin.Context) {
	var req struct {
		RiskLevel string `json:"risk_level" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: risk_level is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetRiskLevel(req.RiskLevel); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Persist risk level to settings file
	go func() {
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateRiskLevel(req.RiskLevel); err != nil {
			fmt.Printf("Failed to persist risk level: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Risk level updated to " + req.RiskLevel,
		"risk_level": controller.GetRiskLevel(),
		"status":     controller.GetStatus(),
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

// handleSetFuturesAutopilotMinConfidence sets custom minimum confidence threshold
func (s *Server) handleSetFuturesAutopilotMinConfidence(c *gin.Context) {
	var req struct {
		MinConfidence float64 `json:"min_confidence" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: min_confidence is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetMinConfidence(req.MinConfidence); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "Min confidence updated",
		"min_confidence": controller.GetMinConfidence(),
		"status":         controller.GetStatus(),
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

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	config := &circuit.CircuitBreakerConfig{
		MaxLossPerHour:       req.MaxLossPerHour,
		MaxDailyLoss:         req.MaxDailyLoss,
		MaxConsecutiveLosses: req.MaxConsecutiveLosses,
		CooldownMinutes:      req.CooldownMinutes,
		MaxTradesPerMinute:   req.MaxTradesPerMinute,
		MaxDailyTrades:       req.MaxDailyTrades,
	}

	if err := controller.UpdateCircuitBreakerConfig(config); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Persist circuit breaker settings to file
	go func() {
		sm := autopilot.GetSettingsManager()
		// Get current enabled state from controller status
		status := controller.GetCircuitBreakerStatus()
		enabled, _ := status["enabled"].(bool)
		if err := sm.UpdateCircuitBreaker(
			enabled,
			req.MaxLossPerHour,
			req.MaxDailyLoss,
			req.MaxConsecutiveLosses,
			req.CooldownMinutes,
			req.MaxTradesPerMinute,
			req.MaxDailyTrades,
		); err != nil {
			// Log error but don't fail the request
			fmt.Printf("Failed to persist circuit breaker settings: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Futures circuit breaker config updated",
		"status":  controller.GetCircuitBreakerStatus(),
	})
}

// handleToggleFuturesCircuitBreaker enables or disables the futures circuit breaker
func (s *Server) handleToggleFuturesCircuitBreaker(c *gin.Context) {
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

	// Persist circuit breaker enabled state
	go func() {
		sm := autopilot.GetSettingsManager()
		settings := sm.GetCurrentSettings()
		settings.CircuitBreakerEnabled = req.Enabled
		if err := sm.SaveSettings(settings); err != nil {
			fmt.Printf("Failed to persist circuit breaker enabled state: %v\n", err)
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
	futuresClient := s.getFuturesClient()
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

	// Check AI service (via autopilot)
	controller := s.getFuturesAutopilot()
	if controller != nil && controller.IsRunning() {
		status["ai_service"] = map[string]interface{}{"status": "ok", "message": "Running"}
	} else if controller != nil {
		status["ai_service"] = map[string]interface{}{"status": "stopped", "message": "Autopilot stopped"}
	} else {
		status["ai_service"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
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

	status := controller.GetInvestigateStatus()
	c.JSON(http.StatusOK, status)
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
	sm := autopilot.GetSettingsManager()
	config := sm.GetAutoModeSettings()
	c.JSON(http.StatusOK, config)
}

// handleSetAutoModeConfig updates auto mode configuration
func (s *Server) handleSetAutoModeConfig(c *gin.Context) {
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

	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateAutoModeSettings(
		req.Enabled,
		req.MaxPositions,
		req.MaxLeverage,
		req.MaxPositionSize,
		req.MaxTotalUSD,
		req.AllowAveraging,
		req.MaxAverages,
		req.MinHoldMinutes,
		req.QuickProfitMode,
		req.MinProfitForExit,
	); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save auto mode settings: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Auto mode settings updated",
		"config":  sm.GetAutoModeSettings(),
	})
}

// handleToggleAutoMode toggles auto mode on/off
func (s *Server) handleToggleAutoMode(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	sm := autopilot.GetSettingsManager()
	if err := sm.UpdateAutoModeEnabled(req.Enabled); err != nil {
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
	globalSettings := sm.GetCurrentSettings()

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
			EffectiveConfidence: sm.GetEffectiveConfidence(symbol, globalSettings.GinieMinConfidence),
			EffectiveMaxUSD:     sm.GetEffectivePositionSize(symbol, globalSettings.GinieMaxUSD),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"symbols":          enrichedSymbols,
		"category_config":  categorySettings,
		"global_min_confidence": globalSettings.GinieMinConfidence,
		"global_max_usd":   globalSettings.GinieMaxUSD,
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

// handleGetSingleSymbolSettings returns settings for a specific symbol
func (s *Server) handleGetSingleSymbolSettings(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol parameter required")
		return
	}

	sm := autopilot.GetSettingsManager()
	settings := sm.GetSymbolSettings(symbol)
	globalSettings := sm.GetCurrentSettings()

	c.JSON(http.StatusOK, gin.H{
		"symbol":              symbol,
		"settings":            settings,
		"effective_confidence": sm.GetEffectiveConfidence(symbol, globalSettings.GinieMinConfidence),
		"effective_max_usd":   sm.GetEffectivePositionSize(symbol, globalSettings.GinieMaxUSD),
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

	globalSettings := sm.GetCurrentSettings()
	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"message":              "Symbol settings updated",
		"symbol":               symbol,
		"settings":             sm.GetSymbolSettings(symbol),
		"effective_confidence": sm.GetEffectiveConfidence(symbol, globalSettings.GinieMinConfidence),
		"effective_max_usd":    sm.GetEffectivePositionSize(symbol, globalSettings.GinieMaxUSD),
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
	globalSettings := sm.GetCurrentSettings()

	// Get effective settings for each symbol
	symbolDetails := make([]map[string]interface{}, 0, len(symbols))
	for _, symbol := range symbols {
		settings := sm.GetSymbolSettings(symbol)
		symbolDetails = append(symbolDetails, map[string]interface{}{
			"symbol":              symbol,
			"win_rate":            settings.WinRate,
			"total_pnl":           settings.TotalPnL,
			"total_trades":        settings.TotalTrades,
			"effective_confidence": sm.GetEffectiveConfidence(symbol, globalSettings.GinieMinConfidence),
			"effective_max_usd":   sm.GetEffectivePositionSize(symbol, globalSettings.GinieMaxUSD),
			"enabled":             settings.Enabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"category": category,
		"count":    len(symbols),
		"symbols":  symbolDetails,
	})
}
