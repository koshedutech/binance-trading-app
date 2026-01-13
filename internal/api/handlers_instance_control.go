package api

import (
	"log"
	"net/http"
	"time"

	"binance-trading-bot/internal/autopilot"

	"github.com/gin-gonic/gin"
)

// ==================== Instance Control API Types ====================
// These types are used for the Active/Standby container control feature (Story 9.6)

// InstanceStatusResponse contains the current instance control status
type InstanceStatusResponse struct {
	InstanceID      string    `json:"instance_id"`       // "dev" or "prod"
	IsActive        bool      `json:"is_active"`         // This instance's status
	ActiveInstance  string    `json:"active_instance"`   // Which instance is currently active
	OtherAlive      bool      `json:"other_alive"`       // Is other instance running (has heartbeat)
	LastHeartbeat   time.Time `json:"last_heartbeat"`    // Last heartbeat time of this instance
	CanTakeControl  bool      `json:"can_take_control"`  // Can this instance take over
	ActiveByDefault bool      `json:"active_by_default"` // Whether this instance is active by default
}

// TakeControlRequest contains parameters for taking control
type TakeControlRequest struct {
	Force bool `json:"force"` // Force takeover even if other instance not responding
}

// TakeControlResponse contains the result of a take control request
type TakeControlResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	WaitSeconds int    `json:"wait_seconds,omitempty"` // Estimated wait time for graceful handover
}

// ReleaseControlResponse contains the result of a release control request
type ReleaseControlResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ==================== Instance Control Handler ====================

// InstanceControlHandler handles instance control API requests
// This is kept as a separate struct for potential future use, but handlers
// are implemented as Server methods to match existing patterns
type InstanceControlHandler struct {
	instanceControl *autopilot.InstanceControl
}

// NewInstanceControlHandler creates a new instance control handler
func NewInstanceControlHandler(ic *autopilot.InstanceControl) *InstanceControlHandler {
	return &InstanceControlHandler{
		instanceControl: ic,
	}
}

// ==================== Server Instance Control Helpers ====================

// getInstanceControl returns the InstanceControl from the FuturesController.
// Returns nil if instance control is not initialized (e.g., Redis not available).
func (s *Server) getInstanceControl() *autopilot.InstanceControl {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		return nil
	}
	return controller.GetInstanceControl()
}

// ==================== Instance Control Handlers ====================

// handleGetInstanceStatus returns the current instance control status
// GET /api/ginie/instance-status
func (s *Server) handleGetInstanceStatus(c *gin.Context) {
	log.Println("[INSTANCE-CONTROL] Getting instance status")

	ic := s.getInstanceControl()
	if ic == nil {
		// Instance control not available - return a default "standalone" status
		// This allows the system to work without Redis/multi-instance setup
		log.Println("[INSTANCE-CONTROL] Instance control not initialized - running in standalone mode")
		c.JSON(http.StatusOK, InstanceStatusResponse{
			InstanceID:      "standalone",
			IsActive:        true, // Standalone mode is always active
			ActiveInstance:  "standalone",
			OtherAlive:      false,
			LastHeartbeat:   time.Now(),
			CanTakeControl:  false, // No other instance to take from
			ActiveByDefault: true,
		})
		return
	}

	// Get instance status from InstanceControl
	ctx := c.Request.Context()
	status := ic.GetStatus(ctx)

	// Can take control if this instance is not already active
	canTakeControl := !status.IsActive

	response := InstanceStatusResponse{
		InstanceID:      status.InstanceID,
		IsActive:        status.IsActive,
		ActiveInstance:  status.ActiveInstance,
		OtherAlive:      status.OtherAlive,
		LastHeartbeat:   time.Now(), // Current time since we just checked
		CanTakeControl:  canTakeControl,
		ActiveByDefault: status.ActiveByDefault,
	}

	log.Printf("[INSTANCE-CONTROL] Status: instance=%s, active=%v, activeInstance=%s, otherAlive=%v",
		status.InstanceID, status.IsActive, status.ActiveInstance, status.OtherAlive)

	c.JSON(http.StatusOK, response)
}

// handleTakeControl initiates a control takeover from another instance
// POST /api/ginie/take-control
func (s *Server) handleTakeControl(c *gin.Context) {
	log.Println("[INSTANCE-CONTROL] Take control request received")

	ic := s.getInstanceControl()
	if ic == nil {
		errorResponse(c, http.StatusServiceUnavailable,
			"Instance control not available - system is running in standalone mode")
		return
	}

	// Parse request body
	var req TakeControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// If no body provided, that's okay - force defaults to false
		req.Force = false
	}

	ctx := c.Request.Context()
	instanceID := ic.GetInstanceID()

	// Check if already active
	if ic.IsActive() {
		log.Printf("[INSTANCE-CONTROL] Instance %s is already active", instanceID)
		c.JSON(http.StatusOK, TakeControlResponse{
			Success: true,
			Message: "This instance is already active",
		})
		return
	}

	// Check if other instance is alive for logging purposes
	otherAlive := ic.IsOtherInstanceAlive(ctx)
	if !otherAlive {
		log.Printf("[INSTANCE-CONTROL] Other instance not responding, proceeding with immediate takeover")
	}

	// Attempt to take control
	// TakeControl handles both graceful and force scenarios internally:
	// - If other instance is alive: sends DEACTIVATE and waits for READY
	// - If other instance is dead: takes over immediately
	var takeoverType string
	if req.Force || !otherAlive {
		takeoverType = "force"
		log.Printf("[INSTANCE-CONTROL] Force/immediate takeover requested by instance %s", instanceID)
	} else {
		takeoverType = "graceful"
		log.Printf("[INSTANCE-CONTROL] Graceful takeover requested by instance %s", instanceID)
	}

	err := ic.TakeControl(ctx)
	if err != nil {
		log.Printf("[INSTANCE-CONTROL] Failed to take control: %v", err)
		errorResponse(c, http.StatusInternalServerError,
			"Failed to take control: "+err.Error())
		return
	}

	// Verify we now have control
	if !ic.IsActive() {
		log.Printf("[INSTANCE-CONTROL] Take control completed but instance is not active")
		c.JSON(http.StatusOK, TakeControlResponse{
			Success:     false,
			Message:     "Takeover initiated but not yet complete - waiting for other instance",
			WaitSeconds: 5, // Estimated max wait for graceful handover
		})
		return
	}

	message := "Successfully took control"
	if takeoverType == "force" {
		message = "Successfully took control (immediate takeover - other instance not responding)"
	}

	log.Printf("[INSTANCE-CONTROL] Instance %s successfully took control", instanceID)
	c.JSON(http.StatusOK, TakeControlResponse{
		Success:     true,
		Message:     message,
		WaitSeconds: 0,
	})
}

// handleReleaseControl voluntarily releases control to another instance
// POST /api/ginie/release-control
func (s *Server) handleReleaseControl(c *gin.Context) {
	log.Println("[INSTANCE-CONTROL] Release control request received")

	ic := s.getInstanceControl()
	if ic == nil {
		errorResponse(c, http.StatusServiceUnavailable,
			"Instance control not available - system is running in standalone mode")
		return
	}

	ctx := c.Request.Context()
	instanceID := ic.GetInstanceID()

	// Check if we are active
	if !ic.IsActive() {
		log.Printf("[INSTANCE-CONTROL] Instance %s is not active - cannot release control", instanceID)
		c.JSON(http.StatusOK, ReleaseControlResponse{
			Success: false,
			Message: "This instance is not currently active - nothing to release",
		})
		return
	}

	// Release control
	err := ic.ReleaseControl(ctx)
	if err != nil {
		log.Printf("[INSTANCE-CONTROL] Failed to release control: %v", err)
		errorResponse(c, http.StatusInternalServerError,
			"Failed to release control: "+err.Error())
		return
	}

	log.Printf("[INSTANCE-CONTROL] Instance %s successfully released control", instanceID)
	c.JSON(http.StatusOK, ReleaseControlResponse{
		Success: true,
		Message: "Successfully released control - default instance will become active",
	})
}
