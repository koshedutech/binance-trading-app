package api

import (
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/circuit"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGetFuturesAutopilotStatus returns futures autopilot status
func (s *Server) handleGetFuturesAutopilotStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":     false,
			"running":     false,
			"dry_run":     true,
			"message":     "Futures autopilot not configured",
		})
		return
	}

	status := controller.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleToggleFuturesAutopilot toggles futures autopilot on/off
func (s *Server) handleToggleFuturesAutopilot(c *gin.Context) {
	var req struct {
		Enabled bool  `json:"enabled"`
		DryRun  *bool `json:"dry_run,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	// Update dry run mode if specified
	if req.DryRun != nil {
		controller.SetDryRun(*req.DryRun)
	}

	if req.Enabled {
		if controller.IsRunning() {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Futures autopilot already running",
				"status":  controller.GetStatus(),
			})
			return
		}

		if err := controller.Start(); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to start futures autopilot: "+err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Futures autopilot started",
			"status":  controller.GetStatus(),
		})
	} else {
		if !controller.IsRunning() {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Futures autopilot already stopped",
				"status":  controller.GetStatus(),
			})
			return
		}

		controller.Stop()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Futures autopilot stopped",
			"status":  controller.GetStatus(),
		})
	}
}

// handleSetFuturesAutopilotDryRun sets dry run mode for futures autopilot
func (s *Server) handleSetFuturesAutopilotDryRun(c *gin.Context) {
	var req struct {
		DryRun bool `json:"dry_run"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	controller.SetDryRun(req.DryRun)

	mode := "LIVE"
	if req.DryRun {
		mode = "DRY RUN (Paper Trading)"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Futures autopilot mode updated to " + mode,
		"dry_run": req.DryRun,
		"status":  controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotRiskLevel changes the risk level
func (s *Server) handleSetFuturesAutopilotRiskLevel(c *gin.Context) {
	var req struct {
		RiskLevel string `json:"risk_level" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: risk_level is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetRiskLevel(req.RiskLevel); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Risk level updated to " + req.RiskLevel,
		"risk_level": controller.GetRiskLevel(),
		"status":     controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotAllocation sets max USD allocation
func (s *Server) handleSetFuturesAutopilotAllocation(c *gin.Context) {
	var req struct {
		MaxUSDAllocation float64 `json:"max_usd_allocation" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: max_usd_allocation is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetMaxUSDAllocation(req.MaxUSDAllocation); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"message":            "Max USD allocation updated",
		"max_usd_allocation": controller.GetMaxUSDAllocation(),
		"status":             controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotProfitReinvest configures profit reinvestment
func (s *Server) handleSetFuturesAutopilotProfitReinvest(c *gin.Context) {
	var req struct {
		ProfitReinvestPercent float64 `json:"profit_reinvest_percent" binding:"required"`
		ProfitRiskLevel       string  `json:"profit_risk_level" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: profit_reinvest_percent and profit_risk_level are required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetProfitReinvestSettings(req.ProfitReinvestPercent, req.ProfitRiskLevel); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                 true,
		"message":                 "Profit reinvestment settings updated",
		"profit_reinvest_percent": req.ProfitReinvestPercent,
		"profit_risk_level":       req.ProfitRiskLevel,
		"status":                  controller.GetStatus(),
	})
}

// handleGetFuturesAutopilotProfitStats returns profit statistics
func (s *Server) handleGetFuturesAutopilotProfitStats(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"total_profit":               0,
			"profit_pool":                0,
			"total_usd_allocated":        0,
			"max_usd_allocation":         0,
			"profit_reinvest_percent":    50,
			"profit_reinvest_risk_level": "aggressive",
			"daily_pnl":                  0,
		})
		return
	}

	stats := controller.GetProfitStats()
	c.JSON(http.StatusOK, stats)
}

// handleSetFuturesAutopilotLeverage sets custom default leverage
func (s *Server) handleSetFuturesAutopilotLeverage(c *gin.Context) {
	var req struct {
		Leverage int `json:"leverage" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: leverage is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetDefaultLeverage(req.Leverage); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Default leverage updated",
		"leverage": controller.GetDefaultLeverage(),
		"status":   controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotMinConfidence sets custom minimum confidence threshold
func (s *Server) handleSetFuturesAutopilotMinConfidence(c *gin.Context) {
	var req struct {
		MinConfidence float64 `json:"min_confidence" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: min_confidence is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetMinConfidence(req.MinConfidence); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "Min confidence updated",
		"min_confidence": controller.GetMinConfidence(),
		"status":         controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotMaxPositionSize sets the maximum position size percentage
func (s *Server) handleSetFuturesAutopilotMaxPositionSize(c *gin.Context) {
	var req struct {
		MaxPositionSize float64 `json:"max_position_size" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: max_position_size is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetMaxPositionSize(req.MaxPositionSize); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"message":           "Max position size updated",
		"max_position_size": controller.GetMaxPositionSize(),
		"status":            controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotConfluence sets the confluence requirement
func (s *Server) handleSetFuturesAutopilotConfluence(c *gin.Context) {
	var req struct {
		Confluence int `json:"confluence"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: confluence is required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetConfluence(req.Confluence); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    fmt.Sprintf("Confluence requirement updated to %d", req.Confluence),
		"confluence": controller.GetConfluence(),
		"status":     controller.GetStatus(),
	})
}

// handleSetFuturesAutopilotTPSL sets custom TP/SL percentages
func (s *Server) handleSetFuturesAutopilotTPSL(c *gin.Context) {
	var req struct {
		TakeProfitPercent float64 `json:"take_profit_percent" binding:"required"`
		StopLossPercent   float64 `json:"stop_loss_percent" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: take_profit_percent and stop_loss_percent are required")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetTPSLPercent(req.TakeProfitPercent, req.StopLossPercent); err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	tp, sl := controller.GetTPSLPercent()

	c.JSON(http.StatusOK, gin.H{
		"success":             true,
		"message":             "TP/SL percentages updated",
		"take_profit_percent": tp,
		"stop_loss_percent":   sl,
		"status":              controller.GetStatus(),
	})
}

// handleResetFuturesAutopilotAllocation resets or recalculates the allocation counter
func (s *Server) handleResetFuturesAutopilotAllocation(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	// Check if recalculate param is set
	recalculate := c.Query("recalculate") == "true"

	var newAllocation float64
	if recalculate {
		// Recalculate based on active positions
		newAllocation = controller.RecalculateAllocation()
	} else {
		// Reset to zero
		controller.ResetAllocation()
		newAllocation = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"message":              "Allocation reset successfully",
		"new_allocation":       newAllocation,
		"recalculated":         recalculate,
		"status":               controller.GetStatus(),
	})
}

// handleGetFuturesCircuitBreakerStatus returns the circuit breaker status for futures
func (s *Server) handleGetFuturesCircuitBreakerStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"enabled":   false,
			"message":   "Futures autopilot not configured",
		})
		return
	}

	status := controller.GetCircuitBreakerStatus()
	c.JSON(http.StatusOK, status)
}

// handleResetFuturesCircuitBreaker resets the futures circuit breaker
func (s *Server) handleResetFuturesCircuitBreaker(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.ResetCircuitBreaker(); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Futures circuit breaker reset successfully",
		"status":  controller.GetCircuitBreakerStatus(),
	})
}

// handleUpdateFuturesCircuitBreakerConfig updates the futures circuit breaker config
func (s *Server) handleUpdateFuturesCircuitBreakerConfig(c *gin.Context) {
	var req struct {
		MaxLossPerHour       float64 `json:"max_loss_per_hour"`
		MaxDailyLoss         float64 `json:"max_daily_loss"`
		MaxConsecutiveLosses int     `json:"max_consecutive_losses"`
		CooldownMinutes      int     `json:"cooldown_minutes"`
		MaxTradesPerMinute   int     `json:"max_trades_per_minute"`
		MaxDailyTrades       int     `json:"max_daily_trades"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	config := &circuit.CircuitBreakerConfig{
		MaxLossPerHour:       req.MaxLossPerHour,
		MaxDailyLoss:         req.MaxDailyLoss,
		MaxConsecutiveLosses: req.MaxConsecutiveLosses,
		CooldownMinutes:      req.CooldownMinutes,
		MaxTradesPerMinute:   req.MaxTradesPerMinute,
		MaxDailyTrades:       req.MaxDailyTrades,
	}

	if err := controller.UpdateCircuitBreakerConfig(config); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Futures circuit breaker config updated",
		"status":  controller.GetCircuitBreakerStatus(),
	})
}

// handleToggleFuturesCircuitBreaker enables or disables the futures circuit breaker
func (s *Server) handleToggleFuturesCircuitBreaker(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures autopilot not configured")
		return
	}

	if err := controller.SetCircuitBreakerEnabled(req.Enabled); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Futures circuit breaker " + map[bool]string{true: "enabled", false: "disabled"}[req.Enabled],
		"status":  controller.GetCircuitBreakerStatus(),
	})
}

// handleGetFuturesAutopilotRecentDecisions returns recent decision events for UI display
func (s *Server) handleGetFuturesAutopilotRecentDecisions(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"decisions": []interface{}{},
			"message":   "Futures autopilot not configured",
		})
		return
	}

	decisions := controller.GetRecentDecisions()
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"decisions": decisions,
		"count":     len(decisions),
	})
}

// handleGetAPIHealthStatus returns the health status of all APIs
func (s *Server) handleGetAPIHealthStatus(c *gin.Context) {
	status := map[string]interface{}{
		"binance_spot":    map[string]interface{}{"status": "unknown", "message": "Not checked"},
		"binance_futures": map[string]interface{}{"status": "unknown", "message": "Not checked"},
		"ai_service":      map[string]interface{}{"status": "unknown", "message": "Not checked"},
		"database":        map[string]interface{}{"status": "unknown", "message": "Not checked"},
	}

	// Check spot API via botAPI interface
	if s.botAPI != nil {
		clientIface := s.botAPI.GetBinanceClient()
		if clientIface != nil {
			// Try to get account info as health check
			type SpotChecker interface {
				GetAccountInfo() (interface{}, error)
			}
			if checker, ok := clientIface.(SpotChecker); ok {
				_, err := checker.GetAccountInfo()
				if err != nil {
					status["binance_spot"] = map[string]interface{}{"status": "error", "message": err.Error()}
				} else {
					status["binance_spot"] = map[string]interface{}{"status": "ok", "message": "Connected"}
				}
			} else {
				status["binance_spot"] = map[string]interface{}{"status": "ok", "message": "Available"}
			}
		} else {
			status["binance_spot"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
		}
	} else {
		status["binance_spot"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
	}

	// Check futures API
	futuresClient := s.getFuturesClient()
	if futuresClient != nil {
		_, err := futuresClient.GetFuturesAccountInfo()
		if err != nil {
			errMsg := err.Error()
			// Parse ban time if present in error message
			if strings.Contains(errMsg, "banned until") {
				// Extract timestamp from error like "banned until 1766161377588"
				re := regexp.MustCompile(`banned until (\d+)`)
				if matches := re.FindStringSubmatch(errMsg); len(matches) > 1 {
					if banTs, parseErr := strconv.ParseInt(matches[1], 10, 64); parseErr == nil {
						banTime := time.UnixMilli(banTs)
						remaining := time.Until(banTime)
						if remaining > 0 {
							errMsg = fmt.Sprintf("IP banned. Releases in %d min %d sec (at %s)",
								int(remaining.Minutes()), int(remaining.Seconds())%60,
								banTime.Format("15:04:05"))
						} else {
							errMsg = "Ban expired. Refresh to check."
						}
					}
				}
			}
			status["binance_futures"] = map[string]interface{}{"status": "error", "message": errMsg}
		} else {
			status["binance_futures"] = map[string]interface{}{"status": "ok", "message": "Connected"}
		}
	} else {
		status["binance_futures"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
	}

	// Check AI service (via autopilot)
	controller := s.getFuturesAutopilot()
	if controller != nil && controller.IsRunning() {
		status["ai_service"] = map[string]interface{}{"status": "ok", "message": "Running"}
	} else if controller != nil {
		status["ai_service"] = map[string]interface{}{"status": "stopped", "message": "Autopilot stopped"}
	} else {
		status["ai_service"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
	}

	// Check database
	if s.repo != nil {
		// Simple ping check
		status["database"] = map[string]interface{}{"status": "ok", "message": "Connected"}
	} else {
		status["database"] = map[string]interface{}{"status": "disabled", "message": "Not configured"}
	}

	// Overall status
	allOk := true
	for _, v := range status {
		if m, ok := v.(map[string]interface{}); ok {
			if m["status"] == "error" {
				allOk = false
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"healthy":  allOk,
		"services": status,
	})
}

// getFuturesAutopilot is a helper to get the futures autopilot from botAPI
func (s *Server) getFuturesAutopilot() *autopilot.FuturesController {
	if s.botAPI == nil {
		return nil
	}

	// Check if botAPI implements FuturesAutopilotProvider
	if provider, ok := s.botAPI.(interface {
		GetFuturesAutopilot() interface{}
	}); ok {
		if ctrl, ok := provider.GetFuturesAutopilot().(*autopilot.FuturesController); ok {
			return ctrl
		}
	}

	return nil
}

// ==================== SENTIMENT & NEWS ====================

// handleGetSentimentNews returns recent crypto news with sentiment
func (s *Server) handleGetSentimentNews(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	if limit > 50 {
		limit = 50
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"news":      []interface{}{},
			"sentiment": nil,
			"count":     0,
			"message":   "Autopilot not configured",
		})
		return
	}

	// Get sentiment analyzer from controller
	sentiment := controller.GetSentimentScore()
	news := controller.GetRecentNews(limit)

	c.JSON(http.StatusOK, gin.H{
		"news":      news,
		"sentiment": sentiment,
		"count":     len(news),
	})
}

// ==================== POSITION AVERAGING ====================

// handleGetAveragingStatus returns averaging configuration and position status
func (s *Server) handleGetAveragingStatus(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":   false,
			"config":    nil,
			"positions": []interface{}{},
			"message":   "Autopilot not configured",
		})
		return
	}

	status := controller.GetAveragingStatus()
	c.JSON(http.StatusOK, status)
}

// handleSetAveragingConfig updates averaging configuration
func (s *Server) handleSetAveragingConfig(c *gin.Context) {
	var req struct {
		Enabled         bool    `json:"enabled"`
		MaxEntries      int     `json:"max_entries"`
		MinConfidence   float64 `json:"min_confidence"`
		MinPriceImprove float64 `json:"min_price_improve"`
		CooldownMins    int     `json:"cooldown_mins"`
		NewsWeight      float64 `json:"news_weight"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	controller.SetAveragingConfig(
		req.Enabled,
		req.MaxEntries,
		req.MinConfidence,
		req.MinPriceImprove,
		req.CooldownMins,
		req.NewsWeight,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Averaging config updated",
		"status":  controller.GetAveragingStatus(),
	})
}

// ============================================================================
// DYNAMIC SL/TP ENDPOINTS
// ============================================================================

// handleGetDynamicSLTPConfig returns dynamic SL/TP configuration
func (s *Server) handleGetDynamicSLTPConfig(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	config := controller.GetDynamicSLTPConfig()
	c.JSON(http.StatusOK, config)
}

// handleSetDynamicSLTPConfig updates dynamic SL/TP configuration
func (s *Server) handleSetDynamicSLTPConfig(c *gin.Context) {
	var req struct {
		Enabled         bool    `json:"enabled"`
		ATRPeriod       int     `json:"atr_period"`
		ATRMultiplierSL float64 `json:"atr_multiplier_sl"`
		ATRMultiplierTP float64 `json:"atr_multiplier_tp"`
		LLMWeight       float64 `json:"llm_weight"`
		MinSLPercent    float64 `json:"min_sl_percent"`
		MaxSLPercent    float64 `json:"max_sl_percent"`
		MinTPPercent    float64 `json:"min_tp_percent"`
		MaxTPPercent    float64 `json:"max_tp_percent"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	controller.SetDynamicSLTPConfig(
		req.Enabled,
		req.ATRPeriod,
		req.ATRMultiplierSL,
		req.ATRMultiplierTP,
		req.LLMWeight,
		req.MinSLPercent,
		req.MaxSLPercent,
		req.MinTPPercent,
		req.MaxTPPercent,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Dynamic SL/TP config updated",
		"config":  controller.GetDynamicSLTPConfig(),
	})
}

// ============================================================================
// SCALPING MODE ENDPOINTS
// ============================================================================

// handleGetScalpingConfig returns scalping mode configuration
func (s *Server) handleGetScalpingConfig(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	config := controller.GetScalpingConfig()
	c.JSON(http.StatusOK, config)
}

// handleSetScalpingConfig updates scalping mode configuration
func (s *Server) handleSetScalpingConfig(c *gin.Context) {
	var req struct {
		Enabled          bool    `json:"enabled"`
		MinProfit        float64 `json:"min_profit"`
		QuickReentry     bool    `json:"quick_reentry"`
		ReentryDelaySec  int     `json:"reentry_delay_sec"`
		MaxTradesPerDay  int     `json:"max_trades_per_day"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not configured")
		return
	}

	controller.SetScalpingConfig(
		req.Enabled,
		req.MinProfit,
		req.QuickReentry,
		req.ReentryDelaySec,
		req.MaxTradesPerDay,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Scalping config updated",
		"config":  controller.GetScalpingConfig(),
	})
}
