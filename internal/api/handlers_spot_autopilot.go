package api

import (
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/circuit"
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// SPOT AUTOPILOT HANDLERS
// ============================================================================

// handleGetSpotAutopilotStatus returns spot autopilot status
func (s *Server) handleGetSpotAutopilotStatus(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":     false,
			"running":     false,
			"dry_run":     true,
			"message":     "Spot autopilot not configured",
			"user_id":     userID,
		})
		return
	}

	status := controller.GetStatus()

	// Add multi-user ownership information
	ownerUserID := controller.GetOwnerUserID()
	status["owner_user_id"] = ownerUserID
	status["is_owner"] = (userID != "" && userID == ownerUserID) || ownerUserID == ""
	status["user_id"] = userID

	c.JSON(http.StatusOK, status)
}

// handleToggleSpotAutopilot toggles spot autopilot on/off
func (s *Server) handleToggleSpotAutopilot(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

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

	// Check ownership - only owner or no owner can toggle
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, fmt.Sprintf("Spot autopilot is owned by another user"))
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
				"user_id": userID,
			})
			return
		}

		// Set owner when starting
		controller.SetOwnerUserID(userID)
		log.Printf("[SPOT] User %s starting spot autopilot", userID)

		if err := controller.Start(); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to start spot autopilot: "+err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Spot autopilot started",
			"status":  controller.GetStatus(),
			"user_id": userID,
		})
	} else {
		if !controller.IsRunning() {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Spot autopilot already stopped",
				"status":  controller.GetStatus(),
				"user_id": userID,
			})
			return
		}

		log.Printf("[SPOT] User %s stopping spot autopilot", userID)
		controller.Stop()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Spot autopilot stopped",
			"status":  controller.GetStatus(),
			"user_id": userID,
		})
	}
}

// handleSetSpotAutopilotDryRun sets dry run mode for spot autopilot
func (s *Server) handleSetSpotAutopilotDryRun(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

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

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
		return
	}

	controller.SetDryRun(req.DryRun)
	log.Printf("[SPOT] User %s set dry_run=%v", userID, req.DryRun)

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
		"user_id": userID,
	})
}

// handleSetSpotAutopilotAllocation sets max USD allocation for spot
func (s *Server) handleSetSpotAutopilotAllocation(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

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

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
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
		"user_id":              userID,
	})
}

// handleSetSpotAutopilotMaxPositions sets max number of positions for spot
func (s *Server) handleSetSpotAutopilotMaxPositions(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

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

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
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
		"user_id":       userID,
	})
}

// handleSetSpotAutopilotTPSL sets custom TP/SL percentages for spot
func (s *Server) handleSetSpotAutopilotTPSL(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

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

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
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
		"user_id":             userID,
	})
}

// handleGetSpotAutopilotProfitStats returns profit statistics for spot
func (s *Server) handleGetSpotAutopilotProfitStats(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"total_profit":         0,
			"total_trades":         0,
			"winning_trades":       0,
			"losing_trades":        0,
			"win_rate":             0,
			"max_usd_per_position": 0,
			"daily_pnl":            0,
			"user_id":              userID,
		})
		return
	}

	stats := controller.GetProfitStats()
	stats["user_id"] = userID
	c.JSON(http.StatusOK, stats)
}

// ============================================================================
// SPOT CIRCUIT BREAKER HANDLERS
// ============================================================================

// handleGetSpotCircuitBreakerStatus returns the circuit breaker status for spot
func (s *Server) handleGetSpotCircuitBreakerStatus(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"enabled":   false,
			"message":   "Spot autopilot not configured",
			"user_id":   userID,
		})
		return
	}

	status := controller.GetCircuitBreakerStatus()
	status["user_id"] = userID
	c.JSON(http.StatusOK, status)
}

// handleResetSpotCircuitBreaker resets the spot circuit breaker
func (s *Server) handleResetSpotCircuitBreaker(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
		return
	}

	if err := controller.ResetCircuitBreaker(); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("[SPOT] User %s reset circuit breaker", userID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Spot circuit breaker reset successfully",
		"status":  controller.GetCircuitBreakerStatus(),
		"user_id": userID,
	})
}

// handleUpdateSpotCircuitBreakerConfig updates the spot circuit breaker config
func (s *Server) handleUpdateSpotCircuitBreakerConfig(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
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

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
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
		"user_id": userID,
	})
}

// handleToggleSpotCircuitBreaker enables or disables the spot circuit breaker
func (s *Server) handleToggleSpotCircuitBreaker(c *gin.Context) {
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

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
		return
	}

	if err := controller.SetCircuitBreakerEnabled(req.Enabled); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	// FIXED: Persist circuit breaker enabled state using database call, not GetDefaultSettings()
	// Capture repo for goroutine (c.Request.Context() not safe after handler returns)
	repo := s.repo
	go func() {
		sm := autopilot.GetSettingsManager()
		ctx := context.Background()
		settings, loadErr := sm.LoadSettingsFromDB(ctx, repo, userID)
		if loadErr != nil || settings == nil {
			fmt.Printf("[SPOT-CB] ERROR: Failed to load settings from database for user %s: %v\n", userID, loadErr)
			return
		}
		settings.SpotCircuitBreakerEnabled = req.Enabled
		if err := sm.SaveSettings(settings); err != nil {
			fmt.Printf("[SPOT-CB] Failed to persist spot circuit breaker enabled state: %v\n", err)
		}
	}()

	log.Printf("[SPOT] User %s toggled circuit breaker to %v", userID, req.Enabled)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Spot circuit breaker " + map[bool]string{true: "enabled", false: "disabled"}[req.Enabled],
		"status":  controller.GetCircuitBreakerStatus(),
		"user_id": userID,
	})
}

// ============================================================================
// SPOT COIN PREFERENCES HANDLERS
// ============================================================================

// handleGetSpotCoinPreferences returns coin preferences for spot trading
func (s *Server) handleGetSpotCoinPreferences(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"preferences":   map[string]interface{}{},
			"blacklist":     []string{},
			"whitelist":     []string{},
			"use_whitelist": false,
			"message":       "Spot autopilot not configured",
			"user_id":       userID,
		})
		return
	}

	prefs := controller.GetCoinPreferences()
	c.JSON(http.StatusOK, prefs)
}

// handleSetSpotCoinPreferences updates coin preferences for spot trading
func (s *Server) handleSetSpotCoinPreferences(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

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

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
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
		"user_id":     userID,
	})
}

// ============================================================================
// SPOT AI DECISIONS HANDLERS
// ============================================================================

// handleGetSpotAutopilotRecentDecisions returns recent decision events for spot UI display
func (s *Server) handleGetSpotAutopilotRecentDecisions(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"decisions": []interface{}{},
			"message":   "Spot autopilot not configured",
			"user_id":   userID,
		})
		return
	}

	decisions := controller.GetRecentDecisions()
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"decisions": decisions,
		"count":     len(decisions),
		"user_id":   userID,
	})
}

// handleGetSpotDecisionStats returns decision statistics for spot
func (s *Server) handleGetSpotDecisionStats(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"total_decisions": 0,
			"buy_decisions":   0,
			"sell_decisions":  0,
			"hold_decisions":  0,
			"executed_trades": 0,
			"skipped_trades":  0,
			"message":         "Spot autopilot not configured",
			"user_id":         userID,
		})
		return
	}

	stats := controller.GetDecisionStats()
	stats["user_id"] = userID
	c.JSON(http.StatusOK, stats)
}

// ============================================================================
// SPOT POSITIONS HANDLERS
// ============================================================================

// handleGetSpotPositions returns current spot positions managed by autopilot
func (s *Server) handleGetSpotPositions(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"positions": []interface{}{},
			"count":     0,
			"message":   "Spot autopilot not configured",
			"user_id":   userID,
		})
		return
	}

	positions := controller.GetPositions()
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"positions": positions,
		"count":     len(positions),
		"user_id":   userID,
	})
}

// handleCloseSpotPosition manually closes a spot position
func (s *Server) handleCloseSpotPosition(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

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

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
		return
	}

	if err := controller.ClosePosition(symbol); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("[SPOT] User %s closed position for %s", userID, symbol)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Position closed for " + symbol,
		"user_id": userID,
	})
}

// handleCloseAllSpotPositions closes all spot positions (panic button)
func (s *Server) handleCloseAllSpotPositions(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		errorResponse(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	controller := s.getSpotAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Spot autopilot not configured")
		return
	}

	// Check ownership
	ownerUserID := controller.GetOwnerUserID()
	if ownerUserID != "" && ownerUserID != userID {
		errorResponse(c, http.StatusForbidden, "Spot autopilot is owned by another user")
		return
	}

	log.Printf("[SPOT] User %s closing all positions (panic button)", userID)
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
