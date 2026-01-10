package api

import (
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/database"
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGetModeAllocations returns current capital allocation for all modes
func (s *Server) handleGetModeAllocations(c *gin.Context) {
	userID := s.getUserID(c)

	// Use per-user Ginie autopilot for multi-user isolation
	ginie := s.getGinieAutopilotForUser(c)
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Log user access for read operation
	log.Printf("User %s accessing mode allocations", userID)

	// Get user's real futures client to fetch actual balance
	// This ensures allocations are calculated based on real Binance balance, not mock
	futuresClient := s.getFuturesClientForUser(c)
	var allocationsMap map[string]interface{}

	if futuresClient != nil {
		// Try to get real balance from user's Binance account
		accountInfo, err := futuresClient.GetFuturesAccountInfo()
		if err == nil && accountInfo != nil {
			// Use real balance for allocation calculation
			realBalance := accountInfo.AvailableBalance
			log.Printf("User %s: Using real balance $%.2f for mode allocations", userID, realBalance)
			allocationsMap = ginie.GetModeAllocationStatusWithBalance(realBalance)
		} else {
			// Fallback to internal client balance if account fetch fails
			log.Printf("User %s: Failed to get real balance (%v), using internal client", userID, err)
			allocationsMap = ginie.GetModeAllocationStatus()
		}
	} else {
		// No user client available, use internal client (mock in paper mode)
		log.Printf("User %s: No user client available, using internal client for allocations", userID)
		allocationsMap = ginie.GetModeAllocationStatus()
	}

	// Fee constants for Binance Futures
	// NOTE: These are standard Binance Futures fee rates (non-VIP tier)
	// VIP users may have lower fees. Consider making these configurable
	// via settings if users request fee customization.
	// Fee rates: https://www.binance.com/en/fee/futureFee
	const (
		takerFeePercent      = 0.05  // 0.05% taker fee (standard tier)
		makerFeePercent      = 0.02  // 0.02% maker fee (standard tier)
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
	userID := s.getUserID(c)
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	// Add ownership check for write operations
	ownerID := controller.GetOwnerUserID()
	if ownerID != "" && ownerID != userID {
		errorResponse(c, http.StatusForbidden, "This autopilot is owned by another user")
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
	// NOTE: We allow ±1% tolerance to account for:
	// 1. Floating point rounding errors in UI sliders/inputs
	// 2. User convenience when distributing percentages (e.g., 33.3% + 33.3% + 33.4% = 100%)
	// The system normalizes to exactly 100% internally before applying allocations
	total := req.UltraFastPercent + req.ScalpPercent + req.SwingPercent + req.PositionPercent
	if total < 99.0 || total > 101.0 {
		errorResponse(c, http.StatusBadRequest, "Percentages must sum to 100% (tolerance: ±1%)")
		return
	}

	ctx := c.Request.Context()

	// 1. Load current allocation from database (or use defaults)
	currentAllocation, err := s.repo.GetUserCapitalAllocation(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load current allocation: "+err.Error())
		return
	}

	// Use defaults if not found
	if currentAllocation == nil {
		currentAllocation = database.DefaultUserCapitalAllocation()
		currentAllocation.UserID = userID
	}

	// 2. Update only the percentage fields (keep position limits)
	currentAllocation.UltraFastPercent = req.UltraFastPercent
	currentAllocation.ScalpPercent = req.ScalpPercent
	currentAllocation.SwingPercent = req.SwingPercent
	currentAllocation.PositionPercent = req.PositionPercent

	// 3. Save to DATABASE first
	if err := s.repo.SaveUserCapitalAllocation(ctx, currentAllocation); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save allocation: "+err.Error())
		return
	}

	// 4. Update in-memory settings for immediate use (optional - for backward compatibility)
	allocation := &autopilot.ModeAllocationConfig{
		UltraFastScalpPercent:        currentAllocation.UltraFastPercent,
		ScalpPercent:                 currentAllocation.ScalpPercent,
		SwingPercent:                 currentAllocation.SwingPercent,
		PositionPercent:              currentAllocation.PositionPercent,
		MaxUltraFastPositions:        currentAllocation.MaxUltraFastPositions,
		MaxScalpPositions:            currentAllocation.MaxScalpPositions,
		MaxSwingPositions:            currentAllocation.MaxSwingPositions,
		MaxPositionPositions:         currentAllocation.MaxPositionPositions,
		MaxUltraFastUSDPerPosition:   currentAllocation.MaxUltraFastUSDPerPosition,
		MaxScalpUSDPerPosition:       currentAllocation.MaxScalpUSDPerPosition,
		MaxSwingUSDPerPosition:       currentAllocation.MaxSwingUSDPerPosition,
		MaxPositionUSDPerPosition:    currentAllocation.MaxPositionUSDPerPosition,
	}
	if err := autopilot.GetSettingsManager().UpdateModeAllocation(allocation); err != nil {
		// Log but don't fail - database is source of truth
		log.Printf("[MODE-ALLOCATION] Warning: Failed to update in-memory settings: %v", err)
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
	userID := s.getUserID(c)
	mode := c.Param("mode")

	// Use per-user Ginie autopilot for multi-user isolation
	ginie := s.getGinieAutopilotForUser(c)
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Log user access for read operation
	log.Printf("User %s accessing mode allocation status for %s", userID, mode)

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
	userID := s.getUserID(c)

	// Use per-user Ginie autopilot for multi-user isolation
	ginie := s.getGinieAutopilotForUser(c)
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	// Log user access for read operation
	log.Printf("User %s accessing mode safety status", userID)

	// Get REAL circuit breaker status for each mode
	modes := []autopilot.GinieTradingMode{
		autopilot.GinieModeUltraFast,
		autopilot.GinieModeScalp,
		autopilot.GinieModeSwing,
		autopilot.GinieModePosition,
	}

	// Default min win rates per mode (fallback if not in circuit breaker config)
	minWinRates := map[string]float64{
		"ultra_fast": 50.0,
		"scalp":      50.0,
		"swing":      55.0,
		"position":   60.0,
	}

	// Default max loss windows per mode
	maxLossWindows := map[string]float64{
		"ultra_fast": -1.5,
		"scalp":      -2.0,
		"swing":      -3.0,
		"position":   -5.0,
	}

	modesStatus := make(gin.H)
	for _, mode := range modes {
		modeStr := string(mode)

		// Get real circuit breaker status from Ginie
		cbStatus := ginie.GetModeCircuitBreakerStatus(mode)

		// Extract real values from circuit breaker status
		paused := false
		pauseReason := ""
		var pauseUntil interface{} = nil
		currentWinRate := 0.0
		recentTradesPct := 0.0
		cooldownRemaining := ""

		if cbStatus != nil {
			if v, ok := cbStatus["is_paused"].(bool); ok {
				paused = v
			}
			if v, ok := cbStatus["pause_reason"].(string); ok {
				pauseReason = v
			}
			if v, ok := cbStatus["paused_until"]; ok && v != nil {
				pauseUntil = v
			}
			if v, ok := cbStatus["current_win_rate"].(float64); ok {
				currentWinRate = v
			}
			if v, ok := cbStatus["recent_trades_pct"].(float64); ok {
				recentTradesPct = v
			}
			if v, ok := cbStatus["cooldown_remaining"].(string); ok {
				cooldownRemaining = v
			}
			// Use circuit breaker's min_win_rate if available
			if v, ok := cbStatus["min_win_rate"].(float64); ok && v > 0 {
				minWinRates[modeStr] = v
			}
		}

		modesStatus[modeStr] = gin.H{
			"paused":             paused,
			"pause_reason":       pauseReason,
			"pause_until":        pauseUntil,
			"current_win_rate":   currentWinRate,
			"min_win_rate":       minWinRates[modeStr],
			"recent_trades_pct":  recentTradesPct,
			"max_loss_window":    maxLossWindows[modeStr],
			"cooldown_remaining": cooldownRemaining,
		}
	}

	safetyStatus := gin.H{
		"success":   true,
		"modes":     modesStatus,
		"timestamp": c.GetTime("request_time"),
	}

	c.JSON(http.StatusOK, safetyStatus)
}

// handleResumeMode manually resumes a paused mode
func (s *Server) handleResumeMode(c *gin.Context) {
	userID := s.getUserID(c)
	mode := c.Param("mode")

	// Validate mode
	validModes := map[string]autopilot.GinieTradingMode{
		"ultra_fast": autopilot.GinieModeUltraFast,
		"scalp":      autopilot.GinieModeScalp,
		"swing":      autopilot.GinieModeSwing,
		"position":   autopilot.GinieModePosition,
	}

	ginieMode, valid := validModes[mode]
	if !valid {
		errorResponse(c, http.StatusBadRequest, "Invalid mode: "+mode+". Valid modes are: ultra_fast, scalp, swing, position")
		return
	}

	// Use per-user Ginie autopilot for multi-user isolation (includes ownership check)
	ginie := s.getGinieAutopilotForUser(c)
	if ginie == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Ginie autopilot not available for this user")
		return
	}

	log.Printf("User %s resuming mode %s", userID, mode)

	// Actually reset the circuit breaker to clear the pause state
	err := ginie.ResetModeCircuitBreaker(ginieMode)
	if err != nil {
		log.Printf("Failed to reset circuit breaker for mode %s: %v", mode, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to resume mode: "+err.Error())
		return
	}

	log.Printf("Successfully resumed mode %s for user %s", mode, userID)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Mode " + mode + " resumed successfully",
		"mode":       mode,
		"status":     "active",
		"resumed_at": c.GetTime("request_time"),
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
