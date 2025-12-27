package api

import (
	"binance-trading-bot/internal/autopilot"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetHedgingStatus returns the current hedging status
func (s *Server) handleGetHedgingStatus(c *gin.Context) {
	userID := s.getUserID(c)
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Log user access for read operation
	log.Printf("User %s accessing hedging status", userID)

	hm := controller.GetHedgingManager()
	if hm == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Hedging manager not initialized")
		return
	}

	status := hm.GetHedgeStatus()
	c.JSON(http.StatusOK, status)
}

// handleGetHedgingConfig returns the hedging configuration
// Note: This uses the GetHedgeStatus which includes config info
func (s *Server) handleGetHedgingConfig(c *gin.Context) {
	userID := s.getUserID(c)
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Log user access for read operation
	log.Printf("User %s accessing hedging config", userID)

	hm := controller.GetHedgingManager()
	if hm == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Hedging manager not initialized")
		return
	}

	// GetHedgeStatus includes all config info
	status := hm.GetHedgeStatus()
	c.JSON(http.StatusOK, status)
}

// UpdateHedgingConfigRequest is the request body for updating hedging config
type UpdateHedgingConfigRequest struct {
	Enabled               *bool     `json:"enabled,omitempty"`
	PriceDropTriggerPct   *float64  `json:"price_drop_trigger_pct,omitempty"`
	UnrealizedLossTrigger *float64  `json:"unrealized_loss_trigger,omitempty"`
	AIEnabled             *bool     `json:"ai_enabled,omitempty"`
	AIConfidenceMin       *float64  `json:"ai_confidence_min,omitempty"`
	DefaultPercent        *float64  `json:"default_percent,omitempty"`
	PartialSteps          []float64 `json:"partial_steps,omitempty"`
	ProfitTakePct         *float64  `json:"profit_take_pct,omitempty"`
	CloseOnRecoveryPct    *float64  `json:"close_on_recovery_pct,omitempty"`
	MaxSimultaneous       *int      `json:"max_simultaneous,omitempty"`
}

// handleUpdateHedgingConfig updates the hedging configuration
func (s *Server) handleUpdateHedgingConfig(c *gin.Context) {
	userID := s.getUserID(c)
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Check ownership for write operation
	ownerID := controller.GetOwnerUserID()
	if ownerID != "" && ownerID != userID {
		errorResponse(c, http.StatusForbidden, "This autopilot is owned by another user")
		return
	}

	hm := controller.GetHedgingManager()
	if hm == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Hedging manager not initialized")
		return
	}

	var req UpdateHedgingConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Update settings via hedging manager
	hm.UpdateSettings(
		req.Enabled,
		req.PriceDropTriggerPct,
		req.UnrealizedLossTrigger,
		req.AIEnabled,
		req.AIConfidenceMin,
		req.DefaultPercent,
		req.PartialSteps,
		req.ProfitTakePct,
		req.CloseOnRecoveryPct,
		req.MaxSimultaneous,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Hedging configuration updated",
	})
}

// ManualHedgeRequest is the request body for manual hedge
type ManualHedgeRequest struct {
	Symbol       string  `json:"symbol"`
	HedgePercent float64 `json:"hedge_percent"`
}

// handleExecuteManualHedge manually executes a hedge on a position
func (s *Server) handleExecuteManualHedge(c *gin.Context) {
	userID := s.getUserID(c)
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Check ownership for write operation
	ownerID := controller.GetOwnerUserID()
	if ownerID != "" && ownerID != userID {
		errorResponse(c, http.StatusForbidden, "This autopilot is owned by another user")
		return
	}

	hm := controller.GetHedgingManager()
	if hm == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Hedging manager not initialized")
		return
	}

	var req ManualHedgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	if req.HedgePercent <= 0 || req.HedgePercent > 100 {
		errorResponse(c, http.StatusBadRequest, "Hedge percent must be between 0 and 100")
		return
	}

	// Get the position for this symbol from controller status
	status := controller.GetStatus()
	positions, ok := status["active_positions"].([]map[string]interface{})
	if !ok {
		errorResponse(c, http.StatusNotFound, "No active positions")
		return
	}

	var targetPos *autopilot.FuturesAutopilotPosition
	for _, p := range positions {
		if p["symbol"].(string) == req.Symbol {
			targetPos = &autopilot.FuturesAutopilotPosition{
				Symbol:     p["symbol"].(string),
				Side:       p["side"].(string),
				EntryPrice: p["entry_price"].(float64),
				Quantity:   p["quantity"].(float64),
				Leverage:   p["leverage"].(int),
			}
			break
		}
	}

	if targetPos == nil {
		errorResponse(c, http.StatusNotFound, "No active position found for symbol")
		return
	}

	// Execute the hedge
	hedgeInfo, err := hm.ExecuteHedge(
		req.Symbol,
		targetPos,
		req.HedgePercent,
		autopilot.HedgeTriggerManual,
		controller.IsDryRun(),
	)

	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to execute hedge: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"hedge":   hedgeInfo,
	})
}

// CloseHedgeRequest is the request body for closing a hedge
type CloseHedgeRequest struct {
	Symbol string `json:"symbol"`
	Reason string `json:"reason,omitempty"`
}

// handleCloseHedge closes a hedge position
func (s *Server) handleCloseHedge(c *gin.Context) {
	userID := s.getUserID(c)
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Check ownership for write operation
	ownerID := controller.GetOwnerUserID()
	if ownerID != "" && ownerID != userID {
		errorResponse(c, http.StatusForbidden, "This autopilot is owned by another user")
		return
	}

	hm := controller.GetHedgingManager()
	if hm == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Hedging manager not initialized")
		return
	}

	var req CloseHedgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	reason := req.Reason
	if reason == "" {
		reason = "manual_close"
	}

	pnl, err := hm.CloseHedge(req.Symbol, reason, controller.IsDryRun())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to close hedge: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"pnl":     pnl,
		"symbol":  req.Symbol,
		"reason":  reason,
	})
}

// handleEnableHedgeMode enables Binance HEDGE position mode
func (s *Server) handleEnableHedgeMode(c *gin.Context) {
	userID := s.getUserID(c)
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Check ownership for write operation
	ownerID := controller.GetOwnerUserID()
	if ownerID != "" && ownerID != userID {
		errorResponse(c, http.StatusForbidden, "This autopilot is owned by another user")
		return
	}

	hm := controller.GetHedgingManager()
	if hm == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Hedging manager not initialized")
		return
	}

	err := hm.EnsureHedgeMode()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to enable hedge mode: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "HEDGE position mode enabled",
	})
}

// handleClearAllHedges closes all active hedges
func (s *Server) handleClearAllHedges(c *gin.Context) {
	userID := s.getUserID(c)
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Check ownership for write operation
	ownerID := controller.GetOwnerUserID()
	if ownerID != "" && ownerID != userID {
		errorResponse(c, http.StatusForbidden, "This autopilot is owned by another user")
		return
	}

	hm := controller.GetHedgingManager()
	if hm == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Hedging manager not initialized")
		return
	}

	err := hm.ClearAllHedges(controller.IsDryRun())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to clear hedges: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All hedges cleared",
	})
}

// handleGetHedgeHistory returns hedge history for a symbol
func (s *Server) handleGetHedgeHistory(c *gin.Context) {
	userID := s.getUserID(c)
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Log user access for read operation
	log.Printf("User %s accessing hedge history", userID)

	hm := controller.GetHedgingManager()
	if hm == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Hedging manager not initialized")
		return
	}

	symbol := c.Query("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	history := hm.GetHedgeHistory(symbol)
	c.JSON(http.StatusOK, history)
}
