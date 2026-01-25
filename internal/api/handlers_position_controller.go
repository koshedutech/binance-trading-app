// Package api provides HTTP handlers for Position Controller endpoints.
// Epic 10, Story 10.4: Position Controller - Exit Signal Executor
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// POSITION CONTROLLER API HANDLERS
// ============================================================================

// GetPositionControllerStatus returns the status of the Position Controller.
// GET /api/futures/position-controller/status
func (s *Server) GetPositionControllerStatus(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Autopilot manager not initialized",
			"running": false,
		})
		return
	}

	// Get user instance
	instance := s.userAutopilotManager.GetInstance(userID)
	if instance == nil {
		c.JSON(http.StatusOK, gin.H{
			"running":             false,
			"signals_processed":   0,
			"sl_updates":          0,
			"tp_updates":          0,
			"heal_actions":        0,
			"message":             "No autopilot instance for user",
		})
		return
	}

	// Check if Position Controller exists on the instance
	if instance.PositionController == nil {
		c.JSON(http.StatusOK, gin.H{
			"running":             false,
			"signals_processed":   0,
			"sl_updates":          0,
			"tp_updates":          0,
			"heal_actions":        0,
			"message":             "Position Controller not initialized",
		})
		return
	}

	// Get status from Position Controller
	status := instance.PositionController.GetStatus()

	c.JSON(http.StatusOK, gin.H{
		"running":              status.Running,
		"started_at":           status.StartedAt,
		"uptime":               status.Uptime,
		"signals_processed":    status.SignalsProcessed,
		"sl_updates":           status.SLUpdatesExecuted,
		"tp_updates":           status.TPUpdatesExecuted,
		"heal_actions":         status.HealActionsExecuted,
		"last_signal_time":     status.LastSignalTime,
		"last_error":           status.LastError,
	})
}

// TriggerPositionControllerHeal triggers an immediate protection heal check.
// POST /api/futures/position-controller/heal
func (s *Server) TriggerPositionControllerHeal(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Autopilot manager not initialized"})
		return
	}

	// Get user instance
	instance := s.userAutopilotManager.GetInstance(userID)
	if instance == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No autopilot instance for user"})
		return
	}

	if instance.PositionController == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position Controller not initialized"})
		return
	}

	// Trigger heal
	if err := instance.PositionController.HealNow(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Protection heal check triggered",
	})
}

// StartPositionController starts the Position Controller for the user.
// POST /api/futures/position-controller/start
func (s *Server) StartPositionController(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Autopilot manager not initialized"})
		return
	}

	ctx := c.Request.Context()

	// Get or create user instance
	instance, err := s.userAutopilotManager.GetOrCreateInstance(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if instance.PositionController == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position Controller not available"})
		return
	}

	// Start if not running
	if instance.PositionController.IsRunning() {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Position Controller already running",
			"running": true,
		})
		return
	}

	if err := instance.PositionController.Start(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Position Controller started",
		"running": true,
	})
}

// StopPositionController stops the Position Controller for the user.
// POST /api/futures/position-controller/stop
func (s *Server) StopPositionController(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Autopilot manager not initialized"})
		return
	}

	instance := s.userAutopilotManager.GetInstance(userID)
	if instance == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No autopilot instance for user"})
		return
	}

	if instance.PositionController == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position Controller not available"})
		return
	}

	// Stop if running
	if !instance.PositionController.IsRunning() {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Position Controller already stopped",
			"running": false,
		})
		return
	}

	if err := instance.PositionController.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Position Controller stopped",
		"running": false,
	})
}
