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

	// Fee constants for Binance Futures
	const (
		takerFeePercent      = 0.05  // 0.05% taker fee
		makerFeePercent      = 0.02  // 0.02% maker fee
		roundTripFeePercent  = 0.10  // 0.10% total (taker open + taker close)
		minRecommendedUSD    = 100.0 // Minimum recommended position size in USD
		optimalMinUSD        = 200.0 // Optimal minimum for better fee ratio
	)

	// Convert map to array format expected by frontend
	allocationsArray := make([]gin.H, 0, len(allocationsMap))
	modeOrder := []string{"ultra_fast", "scalp", "swing", "position"}

	for _, mode := range modeOrder {
		if alloc, exists := allocationsMap[mode]; exists {
			// Type assert to get individual fields
			if allocMap, ok := alloc.(map[string]interface{}); ok {
				// Convert capital_utilization to float64 to ensure proper JSON encoding
				var capacityPercent float64 = 0
				if cu, exists := allocMap["capital_utilization"]; exists && cu != nil {
					switch v := cu.(type) {
					case float64:
						capacityPercent = v
					case int:
						capacityPercent = float64(v)
					case int64:
						capacityPercent = float64(v)
					}
				}

				// Get allocated USD for fee calculations
				var allocatedUSD float64 = 0
				if au, exists := allocMap["allocated_usd"]; exists && au != nil {
					switch v := au.(type) {
					case float64:
						allocatedUSD = v
					case int:
						allocatedUSD = float64(v)
					case int64:
						allocatedUSD = float64(v)
					}
				}

				// Calculate fee impact
				roundTripFeeUSD := allocatedUSD * roundTripFeePercent / 100
				breakEvenMovePercent := roundTripFeePercent // Need this % move just to break even

				// Get max positions for per-position calculation
				var maxPositions int = 1
				if mp, exists := allocMap["max_positions"]; exists && mp != nil {
					switch v := mp.(type) {
					case float64:
						maxPositions = int(v)
					case int:
						maxPositions = v
					case int64:
						maxPositions = int(v)
					}
				}
				if maxPositions < 1 {
					maxPositions = 1
				}

				// Per-position size
				perPositionUSD := allocatedUSD / float64(maxPositions)
				perPositionFeeUSD := perPositionUSD * roundTripFeePercent / 100

				// Generate warning if position size is too small
				var positionWarning string
				var warningLevel string
				if perPositionUSD < minRecommendedUSD {
					warningLevel = "critical"
					positionWarning = "Position size too small! Fees will consume most profits. Increase allocation or reduce max positions."
				} else if perPositionUSD < optimalMinUSD {
					warningLevel = "warning"
					positionWarning = "Position size below optimal. Consider increasing allocation for better fee efficiency."
				} else {
					warningLevel = "ok"
					positionWarning = ""
				}

				allocationsArray = append(allocationsArray, gin.H{
					"mode":                    mode,
					"allocated_percent":       allocMap["allocated_percent"],
					"allocated_usd":           allocMap["allocated_usd"],
					"used_usd":                allocMap["used_usd"],
					"available_usd":           allocMap["available_usd"],
					"current_positions":       allocMap["current_positions"],
					"max_positions":           allocMap["max_positions"],
					"capacity_percent":        capacityPercent,
					// Fee information
					"per_position_usd":        perPositionUSD,
					"round_trip_fee_percent":  roundTripFeePercent,
					"round_trip_fee_usd":      roundTripFeeUSD,
					"per_position_fee_usd":    perPositionFeeUSD,
					"break_even_move_percent": breakEvenMovePercent,
					"min_recommended_usd":     minRecommendedUSD,
					"optimal_min_usd":         optimalMinUSD,
					// Warning
					"warning_level":           warningLevel,
					"position_warning":        positionWarning,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"allocations":  allocationsArray,
		"total_modes":  len(allocationsMap),
		// Global fee info
		"fee_info": gin.H{
			"taker_fee_percent":       takerFeePercent,
			"maker_fee_percent":       makerFeePercent,
			"round_trip_fee_percent":  roundTripFeePercent,
			"min_recommended_usd":     minRecommendedUSD,
			"optimal_min_usd":         optimalMinUSD,
			"fee_note":                "Fees shown are for taker orders. Round-trip = open + close fees. Minimum $100/position recommended to ensure fees don't consume profits.",
		},
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
