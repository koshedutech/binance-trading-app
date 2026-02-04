package api

import (
	"log"
	"net/http"

	"binance-trading-bot/internal/autopilot"

	"github.com/gin-gonic/gin"
)

// ==================== STORY 14.14: Trading State API Endpoints ====================
// API endpoints for Trading Controller - ON/OFF toggle for Chain Trading System
// =================================================================================

// ==================== REQUEST/RESPONSE TYPES ====================

// TradingStateResponse represents the trading state response
type TradingStateResponse struct {
	Enabled     bool   `json:"enabled"`
	EntryActive bool   `json:"entry_active"`
	ExitActive  bool   `json:"exit_active"`
	LastChanged string `json:"last_changed"`
	ChangedBy   string `json:"changed_by"`
	Reason      string `json:"reason,omitempty"`
}

// SetTradingEnabledRequest for enabling/disabling trading
type SetTradingEnabledRequest struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"`
}

// ==================== HANDLERS ====================

// handleGetTradingState returns current trading state for the authenticated user
// GET /api/trading/state
func (s *Server) handleGetTradingState(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	ctx := c.Request.Context()

	// Get trading controller
	controller := autopilot.GetTradingController()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Trading controller not initialized")
		return
	}

	state, err := controller.GetTradingState(ctx, userID)
	if err != nil {
		log.Printf("[TRADING-STATE] Error getting trading state for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to retrieve trading state")
		return
	}

	c.JSON(http.StatusOK, TradingStateResponse{
		Enabled:     state.Enabled,
		EntryActive: state.EntryActive,
		ExitActive:  state.ExitActive,
		LastChanged: state.LastChanged.Format("2006-01-02T15:04:05Z07:00"),
		ChangedBy:   state.ChangedBy,
		Reason:      state.Reason,
	})
}

// handleEnableTrading enables trading for the authenticated user
// POST /api/trading/enable
func (s *Server) handleEnableTrading(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req SetTradingEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to no reason if body is empty or malformed
		req.Reason = ""
	}

	ctx := c.Request.Context()

	// Get trading controller
	controller := autopilot.GetTradingController()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Trading controller not initialized")
		return
	}

	// Enable trading
	if err := controller.SetTradingEnabled(ctx, userID, true, "user", req.Reason); err != nil {
		log.Printf("[TRADING-STATE] Error enabling trading for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to enable trading")
		return
	}

	log.Printf("[TRADING-STATE] Trading enabled for user %s, reason: %s", userID, req.Reason)

	// Refresh coin profiler subscriptions to include strategy requirements
	if s.userAutopilotManager != nil {
		if _, err := s.userAutopilotManager.RefreshCoinProfilerSubscriptions(ctx, userID); err != nil {
			log.Printf("[TRADING-STATE] Warning: Failed to refresh coin profiler subscriptions for user %s: %v", userID, err)
		}
	}

	// Get updated state
	state, err := controller.GetTradingState(ctx, userID)
	if err != nil {
		log.Printf("[TRADING-STATE] Error getting updated state for user %s: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{
			"message": "Trading enabled successfully",
			"enabled": true,
		})
		return
	}

	c.JSON(http.StatusOK, TradingStateResponse{
		Enabled:     state.Enabled,
		EntryActive: state.EntryActive,
		ExitActive:  state.ExitActive,
		LastChanged: state.LastChanged.Format("2006-01-02T15:04:05Z07:00"),
		ChangedBy:   state.ChangedBy,
		Reason:      state.Reason,
	})
}

// handleDisableTrading disables trading for the authenticated user
// POST /api/trading/disable
func (s *Server) handleDisableTrading(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req SetTradingEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to no reason if body is empty or malformed
		req.Reason = ""
	}

	ctx := c.Request.Context()

	// Get trading controller
	controller := autopilot.GetTradingController()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Trading controller not initialized")
		return
	}

	// Disable trading
	if err := controller.SetTradingEnabled(ctx, userID, false, "user", req.Reason); err != nil {
		log.Printf("[TRADING-STATE] Error disabling trading for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to disable trading")
		return
	}

	log.Printf("[TRADING-STATE] Trading disabled for user %s, reason: %s", userID, req.Reason)

	// Refresh coin profiler subscriptions to only include position requirements (no strategies)
	if s.userAutopilotManager != nil {
		if _, err := s.userAutopilotManager.RefreshCoinProfilerSubscriptions(ctx, userID); err != nil {
			log.Printf("[TRADING-STATE] Warning: Failed to refresh coin profiler subscriptions for user %s: %v", userID, err)
		}
	}

	// Get updated state
	state, err := controller.GetTradingState(ctx, userID)
	if err != nil {
		log.Printf("[TRADING-STATE] Error getting updated state for user %s: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{
			"message": "Trading disabled successfully",
			"enabled": false,
		})
		return
	}

	c.JSON(http.StatusOK, TradingStateResponse{
		Enabled:     state.Enabled,
		EntryActive: state.EntryActive,
		ExitActive:  state.ExitActive,
		LastChanged: state.LastChanged.Format("2006-01-02T15:04:05Z07:00"),
		ChangedBy:   state.ChangedBy,
		Reason:      state.Reason,
	})
}

// handleSetTradingState sets the trading state for the authenticated user
// PUT /api/trading/state
func (s *Server) handleSetTradingState(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req SetTradingEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body. Required field: enabled (boolean)")
		return
	}

	ctx := c.Request.Context()

	// Get trading controller
	controller := autopilot.GetTradingController()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Trading controller not initialized")
		return
	}

	// Set trading state
	if err := controller.SetTradingEnabled(ctx, userID, req.Enabled, "user", req.Reason); err != nil {
		log.Printf("[TRADING-STATE] Error setting trading state for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to set trading state")
		return
	}

	action := "enabled"
	if !req.Enabled {
		action = "disabled"
	}
	log.Printf("[TRADING-STATE] Trading %s for user %s, reason: %s", action, userID, req.Reason)

	// Refresh coin profiler subscriptions based on new trading state
	if s.userAutopilotManager != nil {
		if _, err := s.userAutopilotManager.RefreshCoinProfilerSubscriptions(ctx, userID); err != nil {
			log.Printf("[TRADING-STATE] Warning: Failed to refresh coin profiler subscriptions for user %s: %v", userID, err)
		}
	}

	// Get updated state
	state, err := controller.GetTradingState(ctx, userID)
	if err != nil {
		log.Printf("[TRADING-STATE] Error getting updated state for user %s: %v", userID, err)
		c.JSON(http.StatusOK, gin.H{
			"message": "Trading state updated successfully",
			"enabled": req.Enabled,
		})
		return
	}

	c.JSON(http.StatusOK, TradingStateResponse{
		Enabled:     state.Enabled,
		EntryActive: state.EntryActive,
		ExitActive:  state.ExitActive,
		LastChanged: state.LastChanged.Format("2006-01-02T15:04:05Z07:00"),
		ChangedBy:   state.ChangedBy,
		Reason:      state.Reason,
	})
}
