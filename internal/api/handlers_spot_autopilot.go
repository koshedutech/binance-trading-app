package api

import (
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/circuit"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// SPOT AUTOPILOT HANDLERS
// ============================================================================

// handleGetSpotAutopilotStatus returns spot autopilot status
func (s *Server) handleGetSpotAutopilotStatus(c *gin.Context) {
	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":     false,
			"running":     false,
			"dry_run":     true,
			"message":     "Spot autopilot not configured",
		})
		return
	}

	status := controller.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleToggleSpotAutopilot toggles spot autopilot on/off
func (s *Server) handleToggleSpotAutopilot(c *gin.Context) {
	var req struct {
		Enabled bool  `json:"enabled"`
		DryRun  *bool `json:"dry_run,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	// Update dry run mode if specified
	if req.DryRun != nil {
		controller.SetDryRun(*req.DryRun)
	}

	if req.Enabled {
		if controller.IsRunning() {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Spot autopilot already running",
				"status":  controller.GetStatus(),
			})
			return
		}

		if err := controller.Start(); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to start spot autopilot: "+err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Spot autopilot started",
			"status":  controller.GetStatus(),
		})
	} else {
		if !controller.IsRunning() {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Spot autopilot already stopped",
				"status":  controller.GetStatus(),
			})
			return
		}

		controller.Stop()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Spot autopilot stopped",
			"status":  controller.GetStatus(),
		})
	}
}

// handleSetSpotAutopilotDryRun sets dry run mode for spot autopilot
func (s *Server) handleSetSpotAutopilotDryRun(c *gin.Context) {
	var req struct {
		DryRun bool `json:"dry_run"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	controller.SetDryRun(req.DryRun)

	// Persist dry run mode to settings file
	go func() {
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateSpotDryRunMode(req.DryRun); err != nil {
			fmt.Printf("Failed to persist spot dry run mode: %v\n", err)
		}
	}()

	mode := "LIVE"
	if req.DryRun {
		mode = "DRY RUN (Paper Trading)"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Spot autopilot mode updated to " + mode,
		"dry_run": req.DryRun,
		"status":  controller.GetStatus(),
	})
}

// handleSetSpotAutopilotRiskLevel changes the risk level for spot
func (s *Server) handleSetSpotAutopilotRiskLevel(c *gin.Context) {
	var req struct {
		RiskLevel string `json:"risk_level" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: risk_level is required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	if err := controller.SetRiskLevel(req.RiskLevel); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Persist risk level to settings file
	go func() {
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateSpotRiskLevel(req.RiskLevel); err != nil {
			fmt.Printf("Failed to persist spot risk level: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Spot risk level updated to " + req.RiskLevel,
		"risk_level": controller.GetRiskLevel(),
		"status":     controller.GetStatus(),
	})
}

// handleSetSpotAutopilotAllocation sets max USD allocation for spot
func (s *Server) handleSetSpotAutopilotAllocation(c *gin.Context) {
	var req struct {
		MaxUSDPerPosition float64 `json:"max_usd_per_position" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: max_usd_per_position is required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	if err := controller.SetMaxUSDPerPosition(req.MaxUSDPerPosition); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Persist to settings
	go func() {
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateSpotMaxAllocation(req.MaxUSDPerPosition); err != nil {
			fmt.Printf("Failed to persist spot max allocation: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"message":              "Max USD per position updated",
		"max_usd_per_position": controller.GetMaxUSDPerPosition(),
		"status":               controller.GetStatus(),
	})
}

// handleSetSpotAutopilotMaxPositions sets max number of positions for spot
func (s *Server) handleSetSpotAutopilotMaxPositions(c *gin.Context) {
	var req struct {
		MaxPositions int `json:"max_positions" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: max_positions is required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	if err := controller.SetMaxPositions(req.MaxPositions); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Max positions updated",
		"max_positions": controller.GetMaxPositions(),
		"status":        controller.GetStatus(),
	})
}

// handleSetSpotAutopilotTPSL sets custom TP/SL percentages for spot
func (s *Server) handleSetSpotAutopilotTPSL(c *gin.Context) {
	var req struct {
		TakeProfitPercent float64 `json:"take_profit_percent" binding:"required"`
		StopLossPercent   float64 `json:"stop_loss_percent" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: take_profit_percent and stop_loss_percent are required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	if err := controller.SetTPSLPercent(req.TakeProfitPercent, req.StopLossPercent); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	tp, sl := controller.GetTPSLPercent()

	c.JSON(http.StatusOK, gin.H{
		"success":             true,
		"message":             "Spot TP/SL percentages updated",
		"take_profit_percent": tp,
		"stop_loss_percent":   sl,
		"status":              controller.GetStatus(),
	})
}

// handleSetSpotAutopilotMinConfidence sets minimum confidence threshold for spot
func (s *Server) handleSetSpotAutopilotMinConfidence(c *gin.Context) {
	var req struct {
		MinConfidence float64 `json:"min_confidence" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: min_confidence is required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	if err := controller.SetMinConfidence(req.MinConfidence); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "Spot min confidence updated",
		"min_confidence": controller.GetMinConfidence(),
		"status":         controller.GetStatus(),
	})
}

// handleGetSpotAutopilotProfitStats returns profit statistics for spot
func (s *Server) handleGetSpotAutopilotProfitStats(c *gin.Context) {
	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"total_profit":        0,
			"total_trades":        0,
			"winning_trades":      0,
			"losing_trades":       0,
			"win_rate":            0,
			"max_usd_per_position": 0,
			"daily_pnl":           0,
		})
		return
	}

	stats := controller.GetProfitStats()
	c.JSON(http.StatusOK, stats)
}

// ============================================================================
// SPOT CIRCUIT BREAKER HANDLERS
// ============================================================================

// handleGetSpotCircuitBreakerStatus returns the circuit breaker status for spot
func (s *Server) handleGetSpotCircuitBreakerStatus(c *gin.Context) {
	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"enabled":   false,
			"message":   "Spot autopilot not configured",
		})
		return
	}

	status := controller.GetCircuitBreakerStatus()
	c.JSON(http.StatusOK, status)
}

// handleResetSpotCircuitBreaker resets the spot circuit breaker
func (s *Server) handleResetSpotCircuitBreaker(c *gin.Context) {
	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	if err := controller.ResetCircuitBreaker(); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Spot circuit breaker reset successfully",
		"status":  controller.GetCircuitBreakerStatus(),
	})
}

// handleUpdateSpotCircuitBreakerConfig updates the spot circuit breaker config
func (s *Server) handleUpdateSpotCircuitBreakerConfig(c *gin.Context) {
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

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
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
		status := controller.GetCircuitBreakerStatus()
		enabled, _ := status["enabled"].(bool)
		if err := sm.UpdateSpotCircuitBreaker(
			enabled,
			req.MaxLossPerHour,
			req.MaxDailyLoss,
			req.MaxConsecutiveLosses,
			req.CooldownMinutes,
			req.MaxTradesPerMinute,
			req.MaxDailyTrades,
		); err != nil {
			fmt.Printf("Failed to persist spot circuit breaker settings: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Spot circuit breaker config updated",
		"status":  controller.GetCircuitBreakerStatus(),
	})
}

// handleToggleSpotCircuitBreaker enables or disables the spot circuit breaker
func (s *Server) handleToggleSpotCircuitBreaker(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
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
		settings.SpotCircuitBreakerEnabled = req.Enabled
		if err := sm.SaveSettings(settings); err != nil {
			fmt.Printf("Failed to persist spot circuit breaker enabled state: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Spot circuit breaker " + map[bool]string{true: "enabled", false: "disabled"}[req.Enabled],
		"status":  controller.GetCircuitBreakerStatus(),
	})
}

// ============================================================================
// SPOT COIN PREFERENCES HANDLERS
// ============================================================================

// handleGetSpotCoinPreferences returns coin preferences for spot trading
func (s *Server) handleGetSpotCoinPreferences(c *gin.Context) {
	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"preferences":     map[string]interface{}{},
			"blacklist":       []string{},
			"whitelist":       []string{},
			"use_whitelist":   false,
			"message":         "Spot autopilot not configured",
		})
		return
	}

	prefs := controller.GetCoinPreferences()
	c.JSON(http.StatusOK, prefs)
}

// handleSetSpotCoinPreferences updates coin preferences for spot trading
func (s *Server) handleSetSpotCoinPreferences(c *gin.Context) {
	var req struct {
		Blacklist    []string `json:"blacklist"`
		Whitelist    []string `json:"whitelist"`
		UseWhitelist bool     `json:"use_whitelist"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	controller.SetCoinPreferences(req.Blacklist, req.Whitelist, req.UseWhitelist)

	// Persist coin preferences
	go func() {
		sm := autopilot.GetSettingsManager()
		if err := sm.UpdateSpotCoinPreferences(req.Blacklist, req.Whitelist, req.UseWhitelist); err != nil {
			fmt.Printf("Failed to persist spot coin preferences: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Spot coin preferences updated",
		"preferences": controller.GetCoinPreferences(),
	})
}

// ============================================================================
// SPOT AI DECISIONS HANDLERS
// ============================================================================

// handleGetSpotAutopilotRecentDecisions returns recent decision events for spot UI display
func (s *Server) handleGetSpotAutopilotRecentDecisions(c *gin.Context) {
	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"decisions": []interface{}{},
			"message":   "Spot autopilot not configured",
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

// handleGetSpotDecisionStats returns decision statistics for spot
func (s *Server) handleGetSpotDecisionStats(c *gin.Context) {
	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"total_decisions":  0,
			"buy_decisions":    0,
			"sell_decisions":   0,
			"hold_decisions":   0,
			"executed_trades":  0,
			"skipped_trades":   0,
			"message":          "Spot autopilot not configured",
		})
		return
	}

	stats := controller.GetDecisionStats()
	c.JSON(http.StatusOK, stats)
}

// ============================================================================
// SPOT POSITIONS HANDLERS
// ============================================================================

// handleGetSpotPositions returns current spot positions managed by autopilot
func (s *Server) handleGetSpotPositions(c *gin.Context) {
	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"positions": []interface{}{},
			"count":     0,
			"message":   "Spot autopilot not configured",
		})
		return
	}

	positions := controller.GetPositions()
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"positions": positions,
		"count":     len(positions),
	})
}

// handleCloseSpotPosition manually closes a spot position
func (s *Server) handleCloseSpotPosition(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	if err := controller.ClosePosition(symbol); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Position closed for " + symbol,
	})
}

// handleCloseAllSpotPositions closes all spot positions (panic button)
func (s *Server) handleCloseAllSpotPositions(c *gin.Context) {
	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	result := controller.CloseAllPositions()
	c.JSON(http.StatusOK, result)
}

// ============================================================================
// HELPER
// ============================================================================

// getSpotAutopilot is a helper to get the spot autopilot from botAPI
func (s *Server) getSpotAutopilot() *autopilot.SpotController {
	if s.botAPI == nil {
		return nil
	}

	// Check if botAPI implements SpotAutopilotProvider
	if provider, ok := s.botAPI.(interface {
		GetSpotAutopilot() interface{}
	}); ok {
		if ctrl, ok := provider.GetSpotAutopilot().(*autopilot.SpotController); ok {
			return ctrl
		}
	}

	return nil
}
