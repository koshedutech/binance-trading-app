package api

import (
	"log"
	"net/http"

	"binance-trading-bot/internal/database"

	"github.com/gin-gonic/gin"
)

// ==================== REQUEST/RESPONSE TYPES ====================

// SystemControlResponse represents the system control settings response
type SystemControlResponse struct {
	OrderTrackingSystem      string   `json:"order_tracking_system"`
	PositionManagementSystem string   `json:"position_management_system"`
	EntryDecisionSystem      string   `json:"entry_decision_system"`
	ValidOrderTracking       []string `json:"valid_order_tracking_systems"`
	ValidPositionManagement  []string `json:"valid_position_management_systems"`
	ValidEntryDecision       []string `json:"valid_entry_decision_systems"`
}

// UpdateSystemControlRequest for updating system control settings
type UpdateSystemControlRequest struct {
	OrderTrackingSystem      *string `json:"order_tracking_system,omitempty"`
	PositionManagementSystem *string `json:"position_management_system,omitempty"`
	EntryDecisionSystem      *string `json:"entry_decision_system,omitempty"`
}

// ==================== HANDLERS ====================

// handleGetSystemControl returns current system control settings for the authenticated user
// GET /api/settings/system-control
func (s *Server) handleGetSystemControl(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	ctx := c.Request.Context()
	config, err := s.repo.GetUserSystemControlOrDefault(ctx, userID)
	if err != nil {
		log.Printf("[SYSTEM-CONTROL] Error getting system control for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to retrieve system control settings")
		return
	}

	c.JSON(http.StatusOK, SystemControlResponse{
		OrderTrackingSystem:      config.OrderTrackingSystem,
		PositionManagementSystem: config.PositionManagementSystem,
		EntryDecisionSystem:      config.EntryDecisionSystem,
		ValidOrderTracking:       database.ValidOrderTrackingSystems(),
		ValidPositionManagement:  database.ValidPositionManagementSystems(),
		ValidEntryDecision:       database.ValidEntryDecisionSystems(),
	})
}

// handleUpdateSystemControl updates system control settings for the authenticated user
// PUT /api/settings/system-control
func (s *Server) handleUpdateSystemControl(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req UpdateSystemControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate at least one field is provided
	if req.OrderTrackingSystem == nil && req.PositionManagementSystem == nil && req.EntryDecisionSystem == nil {
		errorResponse(c, http.StatusBadRequest, "At least one setting must be provided")
		return
	}

	ctx := c.Request.Context()

	// Get current config or defaults
	config, err := s.repo.GetUserSystemControlOrDefault(ctx, userID)
	if err != nil {
		log.Printf("[SYSTEM-CONTROL] Error getting current config for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to retrieve current settings")
		return
	}

	// Validate and update order tracking system
	if req.OrderTrackingSystem != nil {
		valid := false
		for _, v := range database.ValidOrderTrackingSystems() {
			if v == *req.OrderTrackingSystem {
				valid = true
				break
			}
		}
		if !valid {
			errorResponse(c, http.StatusBadRequest, "Invalid order_tracking_system. Valid values: chain, legacy, both")
			return
		}
		config.OrderTrackingSystem = *req.OrderTrackingSystem
	}

	// Validate and update position management system
	if req.PositionManagementSystem != nil {
		valid := false
		for _, v := range database.ValidPositionManagementSystems() {
			if v == *req.PositionManagementSystem {
				valid = true
				break
			}
		}
		if !valid {
			errorResponse(c, http.StatusBadRequest, "Invalid position_management_system. Valid values: legacy, chain")
			return
		}
		config.PositionManagementSystem = *req.PositionManagementSystem
	}

	// Validate and update entry decision system
	if req.EntryDecisionSystem != nil {
		valid := false
		for _, v := range database.ValidEntryDecisionSystems() {
			if v == *req.EntryDecisionSystem {
				valid = true
				break
			}
		}
		if !valid {
			errorResponse(c, http.StatusBadRequest, "Invalid entry_decision_system. Valid values: legacy, chain")
			return
		}
		config.EntryDecisionSystem = *req.EntryDecisionSystem
	}

	// Save updated config
	config.UserID = userID
	if err := s.repo.SaveUserSystemControl(ctx, config); err != nil {
		log.Printf("[SYSTEM-CONTROL] Error saving config for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to save system control settings")
		return
	}

	log.Printf("[SYSTEM-CONTROL] Updated for user %s: order_tracking=%s, position_management=%s, entry_decision=%s",
		userID, config.OrderTrackingSystem, config.PositionManagementSystem, config.EntryDecisionSystem)

	// CRITICAL: Notify the running autopilot to reload system control settings
	// The autopilot caches these settings, so we must explicitly trigger a reload
	if autopilot := s.getGinieAutopilotForUser(c); autopilot != nil {
		autopilot.ReloadSystemControlSettings()
		log.Printf("[SYSTEM-CONTROL] Reloaded system control settings in autopilot for user %s", userID)
	}

	c.JSON(http.StatusOK, SystemControlResponse{
		OrderTrackingSystem:      config.OrderTrackingSystem,
		PositionManagementSystem: config.PositionManagementSystem,
		EntryDecisionSystem:      config.EntryDecisionSystem,
		ValidOrderTracking:       database.ValidOrderTrackingSystems(),
		ValidPositionManagement:  database.ValidPositionManagementSystems(),
		ValidEntryDecision:       database.ValidEntryDecisionSystems(),
	})
}

// handleSetOrderTrackingSystem updates only the order tracking system
// PUT /api/settings/system-control/order-tracking
func (s *Server) handleSetOrderTrackingSystem(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req struct {
		System string `json:"system" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body. Required field: system")
		return
	}

	ctx := c.Request.Context()
	if err := s.repo.UpdateOrderTrackingSystem(ctx, userID, req.System); err != nil {
		log.Printf("[SYSTEM-CONTROL] Error updating order tracking for user %s: %v", userID, err)
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[SYSTEM-CONTROL] Order tracking updated for user %s: %s", userID, req.System)

	// Reload autopilot settings
	if autopilot := s.getGinieAutopilotForUser(c); autopilot != nil {
		autopilot.ReloadSystemControlSettings()
	}

	c.JSON(http.StatusOK, gin.H{
		"order_tracking_system": req.System,
		"message":               "Order tracking system updated successfully",
	})
}

// handleSetPositionManagementSystem updates only the position management system
// PUT /api/settings/system-control/position-management
func (s *Server) handleSetPositionManagementSystem(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req struct {
		System string `json:"system" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body. Required field: system")
		return
	}

	ctx := c.Request.Context()
	if err := s.repo.UpdatePositionManagementSystem(ctx, userID, req.System); err != nil {
		log.Printf("[SYSTEM-CONTROL] Error updating position management for user %s: %v", userID, err)
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[SYSTEM-CONTROL] Position management updated for user %s: %s", userID, req.System)

	// Reload autopilot settings
	if autopilot := s.getGinieAutopilotForUser(c); autopilot != nil {
		autopilot.ReloadSystemControlSettings()
	}

	c.JSON(http.StatusOK, gin.H{
		"position_management_system": req.System,
		"message":                    "Position management system updated successfully",
	})
}

// handleSetEntryDecisionSystem updates only the entry decision system
// PUT /api/settings/system-control/entry-decision
func (s *Server) handleSetEntryDecisionSystem(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req struct {
		System string `json:"system" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body. Required field: system")
		return
	}

	ctx := c.Request.Context()
	if err := s.repo.UpdateEntryDecisionSystem(ctx, userID, req.System); err != nil {
		log.Printf("[SYSTEM-CONTROL] Error updating entry decision for user %s: %v", userID, err)
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[SYSTEM-CONTROL] Entry decision updated for user %s: %s", userID, req.System)

	// Reload autopilot settings
	if autopilot := s.getGinieAutopilotForUser(c); autopilot != nil {
		autopilot.ReloadSystemControlSettings()
	}

	c.JSON(http.StatusOK, gin.H{
		"entry_decision_system": req.System,
		"message":               "Entry decision system updated successfully",
	})
}
