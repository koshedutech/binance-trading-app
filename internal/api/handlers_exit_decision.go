// Package api provides HTTP API handlers for the trading bot.
// Story 10.3: Exit Decision Monitoring API
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ExitDecisionResponse wraps the exit decision state for API response.
type ExitDecisionResponse struct {
	Success      bool        `json:"success"`
	ExitDecision interface{} `json:"exit_decision,omitempty"`
	Error        string      `json:"error,omitempty"`
}

// handleGetExitDecisionState handles GET /api/futures/positions/:symbol/exit-decision
// Returns the current exit decision state for a position including:
// - Hold safeguards (min hold time, breakeven, consecutive signals)
// - Exit checks (trend reversal, efficiency exit, trailing SL, dynamic TP)
// - Overall decision (HOLD/EXIT) with reason
func (s *Server) handleGetExitDecisionState(c *gin.Context) {
	// Get symbol from path parameter
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, ExitDecisionResponse{
			Success: false,
			Error:   "Symbol is required",
		})
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, ExitDecisionResponse{
			Success: false,
			Error:   "User not authenticated",
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, ExitDecisionResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}

	// Get the user's autopilot instance
	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, ExitDecisionResponse{
			Success: false,
			Error:   "Autopilot manager not initialized",
		})
		return
	}

	// Use GetInstance to get existing instance (similar to other handlers)
	instance := s.userAutopilotManager.GetInstance(userIDStr)
	if instance == nil || instance.Autopilot == nil {
		c.JSON(http.StatusServiceUnavailable, ExitDecisionResponse{
			Success: false,
			Error:   "Autopilot instance not available for user",
		})
		return
	}

	// Get exit decision state for the symbol
	exitState := instance.Autopilot.GetExitDecisionState(symbol)
	if exitState == nil {
		c.JSON(http.StatusNotFound, ExitDecisionResponse{
			Success: false,
			Error:   "No exit decision state found for symbol: " + symbol,
		})
		return
	}

	c.JSON(http.StatusOK, ExitDecisionResponse{
		Success:      true,
		ExitDecision: exitState,
	})
}
