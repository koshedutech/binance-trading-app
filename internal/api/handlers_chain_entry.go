package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetChainEntryRunnerStatus returns the status of the chain entry runner for the authenticated user.
// GET /api/futures/chain-entry/status
func (s *Server) handleGetChainEntryRunnerStatus(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	status := s.userAutopilotManager.GetChainEntryRunnerStatus(userID)
	c.JSON(http.StatusOK, status)
}

// handleStartChainEntryRunner starts the chain entry runner for the authenticated user.
// POST /api/futures/chain-entry/start
func (s *Server) handleStartChainEntryRunner(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	if err := s.userAutopilotManager.StartChainEntryRunner(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "START_FAILED",
			"message": err.Error(),
		})
		return
	}

	status := s.userAutopilotManager.GetChainEntryRunnerStatus(userID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Chain entry runner started",
		"status":  status,
	})
}

// handleStopChainEntryRunner stops the chain entry runner for the authenticated user.
// POST /api/futures/chain-entry/stop
func (s *Server) handleStopChainEntryRunner(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	if err := s.userAutopilotManager.StopChainEntryRunner(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "STOP_FAILED",
			"message": err.Error(),
		})
		return
	}

	status := s.userAutopilotManager.GetChainEntryRunnerStatus(userID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Chain entry runner stopped",
		"status":  status,
	})
}
