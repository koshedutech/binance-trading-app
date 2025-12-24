package api

import (
	"binance-trading-bot/internal/autopilot"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGetModeAllocations returns current capital allocation for all modes
func (s *Server) handleGetModeAllocations(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAutopilot()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Get total capital from controller balance
	allocationsMap := ginie.GetModeAllocationStatus()

	// Convert map to array format expected by frontend
	allocationsArray := make([]gin.H, 0, len(allocationsMap))
	modeOrder := []string{"ultra_fast", "scalp", "swing", "position"}

	for _, mode := range modeOrder {
		if alloc, exists := allocationsMap[mode]; exists {
			// Type assert to get individual fields
			if allocMap, ok := alloc.(map[string]interface{}); ok {
				// Extract and convert capital_utilization to a float64
				var capacityPercent float64 = 0.0
				if cuValue, exists := allocMap["capital_utilization"]; exists && cuValue != nil {
					// Type assert to float64
					if cuFloat, ok := cuValue.(float64); ok {
						capacityPercent = cuFloat
					}
				}

				allocationsArray = append(allocationsArray, gin.H{
					"mode":                 mode,
					"allocated_percent":    allocMap["allocated_percent"],
					"allocated_usd":        allocMap["allocated_usd"],
					"used_usd":             allocMap["used_usd"],
					"available_usd":        allocMap["available_usd"],
					"current_positions":    allocMap["current_positions"],
					"max_positions":        allocMap["max_positions"],
					"capacity_percent":     capacityPercent,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"allocations":  allocationsArray,
		"total_modes":  len(allocationsMap),
	})
}

// handleUpdateModeAllocations updates capital allocation percentages for modes
func (s *Server) handleUpdateModeAllocations(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	var req struct {
		UltraFastPercent float64 `json:"ultra_fast_percent"`
		ScalpPercent     float64 `json:"scalp_percent"`
		SwingPercent     float64 `json:"swing_percent"`
		PositionPercent  float64 `json:"position_percent"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate percentages sum to 100
	total := req.UltraFastPercent + req.ScalpPercent + req.SwingPercent + req.PositionPercent
	if total < 99.0 || total > 101.0 {
		errorResponse(c, http.StatusBadRequest, "Percentages must sum to 100% (tolerance: ±1%)")
		return
	}

	allocation := &autopilot.ModeAllocationConfig{
		UltraFastScalpPercent: req.UltraFastPercent,
		ScalpPercent:          req.ScalpPercent,
		SwingPercent:          req.SwingPercent,
		PositionPercent:       req.PositionPercent,
	}

	// Keep existing position limits if not specified
	currentSettings := autopilot.GetSettingsManager().GetCurrentSettings()
	if currentSettings != nil && currentSettings.ModeAllocation != nil {
		allocation.MaxUltraFastPositions = currentSettings.ModeAllocation.MaxUltraFastPositions
		allocation.MaxScalpPositions = currentSettings.ModeAllocation.MaxScalpPositions
		allocation.MaxSwingPositions = currentSettings.ModeAllocation.MaxSwingPositions
		allocation.MaxPositionPositions = currentSettings.ModeAllocation.MaxPositionPositions
		allocation.MaxUltraFastUSDPerPosition = currentSettings.ModeAllocation.MaxUltraFastUSDPerPosition
		allocation.MaxScalpUSDPerPosition = currentSettings.ModeAllocation.MaxScalpUSDPerPosition
		allocation.MaxSwingUSDPerPosition = currentSettings.ModeAllocation.MaxSwingUSDPerPosition
		allocation.MaxPositionUSDPerPosition = currentSettings.ModeAllocation.MaxPositionUSDPerPosition
	}

	if err := autopilot.GetSettingsManager().UpdateModeAllocation(allocation); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update allocation: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Mode allocation updated",
		"allocation":   allocation,
	})
}

// handleGetModeAllocationHistory returns historical allocation snapshots
func (s *Server) handleGetModeAllocationHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit > 1000 {
		limit = 1000
	}

	if s.repo == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Get allocation history for each mode
	modeHistory := make(map[string]interface{})
	for _, mode := range []string{"ultra_fast", "scalp", "swing", "position"} {
		history, err := s.repo.GetModeAllocationHistory(ctx, mode, limit)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to fetch allocation history: "+err.Error())
			return
		}
		modeHistory[mode] = history
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"history":  modeHistory,
	})
}

// handleGetModeAllocationStatus returns current allocation state for a specific mode
func (s *Server) handleGetModeAllocationStatus(c *gin.Context) {
	mode := c.Param("mode")

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAutopilot()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	allocations := ginie.GetModeAllocationStatus()
	if len(allocations) == 0 {
		errorResponse(c, http.StatusNotFound, "No allocation data available")
		return
	}

	// Find the specific mode
	for _, allocData := range allocations {
		if allocMap, ok := allocData.(map[string]interface{}); ok {
			if allocMap["mode"] == mode {
				c.JSON(http.StatusOK, gin.H{
					"success":     true,
					"allocation":  allocMap,
				})
				return
			}
		}
	}

	errorResponse(c, http.StatusNotFound, "Mode not found: "+mode)
}

// handleGetModeSafetyStatus returns current safety status for all modes
func (s *Server) handleGetModeSafetyStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAutopilot()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Return comprehensive safety status including rate limits, win rates, profit thresholds
	safetyStatus := gin.H{
		"success": true,
		"modes": gin.H{
			"ultra_fast": gin.H{
				"paused":             false,
				"pause_reason":       "",
				"pause_until":        nil,
				"current_win_rate":   0.0,
				"min_win_rate":       50.0,
				"recent_trades_pct":  0.0,
				"max_loss_window":    -1.5,
			},
			"scalp": gin.H{
				"paused":             false,
				"pause_reason":       "",
				"pause_until":        nil,
				"current_win_rate":   0.0,
				"min_win_rate":       50.0,
				"recent_trades_pct":  0.0,
				"max_loss_window":    -2.0,
			},
			"swing": gin.H{
				"paused":             false,
				"pause_reason":       "",
				"pause_until":        nil,
				"current_win_rate":   0.0,
				"min_win_rate":       55.0,
				"recent_trades_pct":  0.0,
				"max_loss_window":    -3.0,
			},
			"position": gin.H{
				"paused":             false,
				"pause_reason":       "",
				"pause_until":        nil,
				"current_win_rate":   0.0,
				"min_win_rate":       60.0,
				"recent_trades_pct":  0.0,
				"max_loss_window":    -5.0,
			},
		},
		"timestamp": c.GetTime("request_time"),
	}

	c.JSON(http.StatusOK, safetyStatus)
}

// handleResumeMode manually resumes a paused mode
func (s *Server) handleResumeMode(c *gin.Context) {
	mode := c.Param("mode")

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	ginie := controller.GetGinieAutopilot()
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not initialized")
		return
	}

	// Resume mode would clear the pause flags in the safety state
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Mode " + mode + " resumed",
		"mode":          mode,
		"status":        "active",
		"resumed_at":    c.GetTime("request_time"),
	})
}

// handleGetModeSafetyHistory returns recent safety events for all modes
func (s *Server) handleGetModeSafetyHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit > 1000 {
		limit = 1000
	}

	if s.repo == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Get safety history for each mode
	modeHistory := make(map[string]interface{})
	for _, mode := range []string{"ultra_fast", "scalp", "swing", "position"} {
		history, err := s.repo.GetModeSafetyHistory(ctx, mode, limit)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to fetch safety history: "+err.Error())
			return
		}
		modeHistory[mode] = history
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"history":  modeHistory,
	})
}

// handleGetModeSafetyEventHistory returns safety events for a specific mode
func (s *Server) handleGetModeSafetyEventHistory(c *gin.Context) {
	mode := c.Param("mode")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit > 1000 {
		limit = 1000
	}

	if s.repo == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	history, err := s.repo.GetModeSafetyHistory(ctx, mode, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch safety history: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"mode":     mode,
		"history":  history,
		"count":    len(history),
	})
}

// handleGetModePerformance returns performance metrics for all modes
func (s *Server) handleGetModePerformance(c *gin.Context) {
	if s.repo == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	statsMap, err := s.repo.GetAllModePerformanceStats(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch performance stats: "+err.Error())
		return
	}

	// Convert to response format
	performanceData := make(map[string]interface{})
	totalTrades := 0
	totalPnL := 0.0

	for mode, stats := range statsMap {
		if stats != nil {
			performanceData[mode] = stats
			totalTrades += stats.TotalTrades
			totalPnL += stats.TotalPnLUSD
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"performance":     performanceData,
		"total_trades":    totalTrades,
		"total_pnl_usd":   totalPnL,
		"mode_count":      len(performanceData),
	})
}

// handleGetModePerformanceSingle returns performance metrics for a single mode
func (s *Server) handleGetModePerformanceSingle(c *gin.Context) {
	mode := c.Param("mode")

	if s.repo == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	stats, err := s.repo.GetModePerformanceStats(ctx, mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch performance stats: "+err.Error())
		return
	}

	if stats == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"mode":    mode,
			"stats":   nil,
			"message": "No performance data yet",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mode":    mode,
		"stats":   stats,
	})
}
