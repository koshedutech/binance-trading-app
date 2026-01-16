package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"

	"binance-trading-bot/internal/autopilot"

	"github.com/gin-gonic/gin"
)

// ==================== USER SETTINGS HANDLERS ====================
// Story 4.16: Settings Comparison & Risk Display

// SettingsComparisonResponse represents the comparison between user settings and defaults
type SettingsComparisonResponse struct {
	Timestamp      string                      `json:"timestamp"`
	TotalChanges   int                         `json:"total_changes"`
	HighRiskCount  int                         `json:"high_risk_count"`
	MediumRiskCount int                        `json:"medium_risk_count"`
	LowRiskCount   int                         `json:"low_risk_count"`
	AllMatch       bool                        `json:"all_match"`
	Groups         map[string]DifferenceGroup  `json:"groups"`
}

// DifferenceGroup represents a group of setting differences
type DifferenceGroup struct {
	GroupName    string       `json:"group_name"`
	DisplayName  string       `json:"display_name"`
	ChangeCount  int          `json:"change_count"`
	Differences  []Difference `json:"differences"`
}

// Difference represents a single setting difference with risk info
type Difference struct {
	Path           string      `json:"path"`
	Current        interface{} `json:"current"`
	Default        interface{} `json:"default"`
	RiskLevel      string      `json:"risk_level"`      // "high", "medium", "low"
	Impact         string      `json:"impact"`
	Recommendation string      `json:"recommendation"`
}

// ResetSingleSettingRequest for resetting individual settings
type ResetSingleSettingRequest struct {
	Path string `json:"path"` // e.g., "mode_configs.ultra_fast.enabled"
}

// handleGetSettingsComparison compares user settings vs defaults and returns ONLY differences
// GET /api/user/settings/comparison
// Story 6.4: Uses AdminDefaultsCacheService for cache-first default settings loading
func (s *Server) handleGetSettingsComparison(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	// Story 6.4: Load default settings from cache (with file fallback)
	var defaults *autopilot.DefaultSettingsFile
	var err error

	if s.adminDefaultsCacheService != nil && s.adminDefaultsCacheService.IsHealthy() {
		defaults, err = s.adminDefaultsCacheService.GetAllAdminDefaults(ctx)
		if err != nil {
			log.Printf("[SETTINGS-COMPARISON] Cache unavailable, falling back to file: %v", err)
			defaults, err = autopilot.LoadDefaultSettings()
		}
	} else {
		// Fallback to direct file load if cache not available
		defaults, err = autopilot.LoadDefaultSettings()
	}

	if err != nil {
		log.Printf("[SETTINGS-COMPARISON] Failed to load defaults: %v", err)
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings")
		return
	}

	// Load user's current settings from database
	userSettings, err := s.loadUserSettings(ctx, userID)
	if err != nil {
		log.Printf("[SETTINGS-COMPARISON] Failed to load user settings: %v", err)
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings")
		return
	}

	// Compare and build response
	response := SettingsComparisonResponse{
		Timestamp: time.Now().Format(time.RFC3339),
		Groups:    make(map[string]DifferenceGroup),
	}

	// Compare mode configs
	modeConfigDiffs := s.compareModeConfigs(userSettings.ModeConfigs, defaults.ModeConfigs, defaults.SettingsRiskIndex)
	if len(modeConfigDiffs) > 0 {
		response.Groups["mode_configs"] = DifferenceGroup{
			GroupName:   "mode_configs",
			DisplayName: "Mode Configurations",
			ChangeCount: len(modeConfigDiffs),
			Differences: modeConfigDiffs,
		}
	}

	// Compare global trading settings
	globalDiffs := s.compareGlobalTrading(userSettings.GlobalTrading, defaults.GlobalTrading, defaults.SettingsRiskIndex)
	if len(globalDiffs) > 0 {
		response.Groups["global_trading"] = DifferenceGroup{
			GroupName:   "global_trading",
			DisplayName: "Global Trading Settings",
			ChangeCount: len(globalDiffs),
			Differences: globalDiffs,
		}
	}

	// Compare circuit breaker settings
	circuitDiffs := s.compareCircuitBreaker(userSettings.CircuitBreaker, defaults.CircuitBreaker, defaults.SettingsRiskIndex)
	if len(circuitDiffs) > 0 {
		response.Groups["circuit_breaker"] = DifferenceGroup{
			GroupName:   "circuit_breaker",
			DisplayName: "Circuit Breaker Settings",
			ChangeCount: len(circuitDiffs),
			Differences: circuitDiffs,
		}
	}

	// Compare position optimization settings
	posOptDiffs := s.comparePositionOptimization(userSettings.PositionOptimization, defaults.PositionOptimization, defaults.SettingsRiskIndex)
	if len(posOptDiffs) > 0 {
		response.Groups["position_optimization"] = DifferenceGroup{
			GroupName:   "position_optimization",
			DisplayName: "Position Optimization",
			ChangeCount: len(posOptDiffs),
			Differences: posOptDiffs,
		}
	}

	// Compare LLM config settings
	llmDiffs := s.compareLLMConfig(userSettings.LLMConfig, defaults.LLMConfig, defaults.SettingsRiskIndex)
	if len(llmDiffs) > 0 {
		response.Groups["llm_config"] = DifferenceGroup{
			GroupName:   "llm_config",
			DisplayName: "LLM Configuration",
			ChangeCount: len(llmDiffs),
			Differences: llmDiffs,
		}
	}

	// Note: Global early_warning comparison removed - early_warning is now per-mode only

	// Compare capital allocation settings
	capAllocDiffs := s.compareCapitalAllocation(userSettings.CapitalAllocation, defaults.CapitalAllocation, defaults.SettingsRiskIndex)
	if len(capAllocDiffs) > 0 {
		response.Groups["capital_allocation"] = DifferenceGroup{
			GroupName:   "capital_allocation",
			DisplayName: "Capital Allocation",
			ChangeCount: len(capAllocDiffs),
			Differences: capAllocDiffs,
		}
	}

	// Calculate totals
	for _, group := range response.Groups {
		response.TotalChanges += group.ChangeCount
		for _, diff := range group.Differences {
			switch diff.RiskLevel {
			case "high":
				response.HighRiskCount++
			case "medium":
				response.MediumRiskCount++
			case "low":
				response.LowRiskCount++
			}
		}
	}

	response.AllMatch = response.TotalChanges == 0

	c.JSON(http.StatusOK, response)
}

// handleResetSingleSetting resets a single setting to default value
// POST /api/user/settings/reset
// Story 6.4: Uses AdminDefaultsCacheService for cache-first default settings loading
func (s *Server) handleResetSingleSetting(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	var req ResetSingleSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Path == "" {
		errorResponse(c, http.StatusBadRequest, "Path is required")
		return
	}

	ctx := c.Request.Context()

	// Story 6.4: Load defaults from cache (with file fallback)
	var defaults *autopilot.DefaultSettingsFile
	var err error

	if s.adminDefaultsCacheService != nil && s.adminDefaultsCacheService.IsHealthy() {
		defaults, err = s.adminDefaultsCacheService.GetAllAdminDefaults(ctx)
		if err != nil {
			log.Printf("[SETTINGS-RESET] Cache unavailable, falling back to file: %v", err)
			defaults, err = autopilot.LoadDefaultSettings()
		}
	} else {
		defaults, err = autopilot.LoadDefaultSettings()
	}

	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings")
		return
	}

	// Parse path and reset the specific setting
	// Path format: "mode_configs.ultra_fast.enabled" or "circuit_breaker.global.enabled"
	parts := strings.Split(req.Path, ".")
	if len(parts) < 2 {
		errorResponse(c, http.StatusBadRequest, "Invalid path format")
		return
	}

	switch parts[0] {
	case "mode_configs":
		if len(parts) < 3 {
			errorResponse(c, http.StatusBadRequest, "Invalid mode_configs path")
			return
		}
		modeName := parts[1]
		err = s.resetModeConfigSetting(ctx, userID, modeName, parts[2:], defaults)

	case "global_trading":
		err = s.resetGlobalTradingSetting(ctx, userID, parts[1:], defaults)

	case "circuit_breaker":
		err = s.resetCircuitBreakerSetting(ctx, userID, parts[1:], defaults)

	case "position_optimization":
		err = s.resetPositionOptimizationSetting(ctx, userID, parts[1:], defaults)

	case "llm_config":
		err = s.resetLLMConfigSetting(ctx, userID, parts[1:], defaults)

	// Note: Global early_warning reset removed - early_warning is now per-mode only

	case "capital_allocation":
		err = s.resetCapitalAllocationSetting(ctx, userID, parts[1:], defaults)

	default:
		errorResponse(c, http.StatusBadRequest, "Unknown settings group: "+parts[0])
		return
	}

	if err != nil {
		log.Printf("[SETTINGS-RESET] Failed to reset %s: %v", req.Path, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to reset setting: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Setting %s reset to default", req.Path),
		"path":    req.Path,
	})
}

// ==================== COMPARISON HELPERS ====================

// compareModeConfigs compares mode configurations
func (s *Server) compareModeConfigs(
	userConfigs map[string]*autopilot.ModeFullConfig,
	defaultConfigs map[string]*autopilot.ModeFullConfig,
	riskIndex autopilot.SettingsRiskIndex,
) []Difference {
	var diffs []Difference

	for modeName, defaultConfig := range defaultConfigs {
		userConfig, exists := userConfigs[modeName]
		if !exists {
			userConfig = defaultConfig // Use defaults if user hasn't configured
		}

		prefix := fmt.Sprintf("mode_configs.%s", modeName)

		// Compare enabled
		if userConfig.Enabled != defaultConfig.Enabled {
			riskLevel, impact, recommendation := s.getRiskInfo(prefix+".enabled", riskIndex, nil)
			diffs = append(diffs, Difference{
				Path:           prefix + ".enabled",
				Current:        userConfig.Enabled,
				Default:        defaultConfig.Enabled,
				RiskLevel:      riskLevel,
				Impact:         impact,
				Recommendation: recommendation,
			})
		}

		// Compare confidence thresholds
		if userConfig.Confidence.MinConfidence != defaultConfig.Confidence.MinConfidence {
			riskLevel, impact, recommendation := s.getRiskInfo(prefix+".confidence.min_confidence", riskIndex, nil)
			diffs = append(diffs, Difference{
				Path:           prefix + ".confidence.min_confidence",
				Current:        userConfig.Confidence.MinConfidence,
				Default:        defaultConfig.Confidence.MinConfidence,
				RiskLevel:      riskLevel,
				Impact:         impact,
				Recommendation: recommendation,
			})
		}

		// Compare leverage
		if userConfig.Size.Leverage != defaultConfig.Size.Leverage {
			riskLevel, impact, recommendation := s.getRiskInfo(prefix+".size.leverage", riskIndex, nil)
			diffs = append(diffs, Difference{
				Path:           prefix + ".size.leverage",
				Current:        userConfig.Size.Leverage,
				Default:        defaultConfig.Size.Leverage,
				RiskLevel:      riskLevel,
				Impact:         impact,
				Recommendation: recommendation,
			})
		}

		// Compare SL/TP percentages
		if userConfig.SLTP.StopLossPercent != defaultConfig.SLTP.StopLossPercent {
			riskLevel, impact, recommendation := s.getRiskInfo(prefix+".sltp.stop_loss_percent", riskIndex, nil)
			diffs = append(diffs, Difference{
				Path:           prefix + ".sltp.stop_loss_percent",
				Current:        userConfig.SLTP.StopLossPercent,
				Default:        defaultConfig.SLTP.StopLossPercent,
				RiskLevel:      riskLevel,
				Impact:         impact,
				Recommendation: recommendation,
			})
		}

		if userConfig.SLTP.TakeProfitPercent != defaultConfig.SLTP.TakeProfitPercent {
			riskLevel, impact, recommendation := s.getRiskInfo(prefix+".sltp.take_profit_percent", riskIndex, nil)
			diffs = append(diffs, Difference{
				Path:           prefix + ".sltp.take_profit_percent",
				Current:        userConfig.SLTP.TakeProfitPercent,
				Default:        defaultConfig.SLTP.TakeProfitPercent,
				RiskLevel:      riskLevel,
				Impact:         impact,
				Recommendation: recommendation,
			})
		}

		// Compare position_optimization settings (Story 9.9)
		diffs = append(diffs, s.compareModePositionOptimization(prefix, userConfig.PositionOptimization, defaultConfig.PositionOptimization, riskIndex)...)
	}

	return diffs
}

// compareModePositionOptimization compares position optimization settings within a mode config
func (s *Server) compareModePositionOptimization(
	prefix string,
	user *autopilot.PositionOptimizationConfig,
	defaults *autopilot.PositionOptimizationConfig,
	riskIndex autopilot.SettingsRiskIndex,
) []Difference {
	var diffs []Difference
	posOptPrefix := prefix + ".position_optimization"

	// Handle nil cases
	if defaults == nil {
		return diffs
	}
	if user == nil {
		// User has no position_optimization, but defaults do - show all defaults as differences
		user = &autopilot.PositionOptimizationConfig{}
	}

	// Compare enabled
	if user.Enabled != defaults.Enabled {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".enabled",
			Current:   user.Enabled,
			Default:   defaults.Enabled,
			RiskLevel: "medium",
		})
	}

	// Compare TP levels (profit taking)
	if user.TP1Percent != defaults.TP1Percent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".tp1_percent",
			Current:   user.TP1Percent,
			Default:   defaults.TP1Percent,
			RiskLevel: "low",
		})
	}
	if user.TP1SellPercent != defaults.TP1SellPercent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".tp1_sell_percent",
			Current:   user.TP1SellPercent,
			Default:   defaults.TP1SellPercent,
			RiskLevel: "low",
		})
	}
	if user.TP2Percent != defaults.TP2Percent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".tp2_percent",
			Current:   user.TP2Percent,
			Default:   defaults.TP2Percent,
			RiskLevel: "low",
		})
	}
	if user.TP2SellPercent != defaults.TP2SellPercent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".tp2_sell_percent",
			Current:   user.TP2SellPercent,
			Default:   defaults.TP2SellPercent,
			RiskLevel: "low",
		})
	}
	if user.TP3Percent != defaults.TP3Percent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".tp3_percent",
			Current:   user.TP3Percent,
			Default:   defaults.TP3Percent,
			RiskLevel: "low",
		})
	}
	if user.TP3SellPercent != defaults.TP3SellPercent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".tp3_sell_percent",
			Current:   user.TP3SellPercent,
			Default:   defaults.TP3SellPercent,
			RiskLevel: "low",
		})
	}

	// Compare negative TP levels (DCA on loss)
	if user.NegTP1Percent != defaults.NegTP1Percent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".neg_tp1_percent",
			Current:   user.NegTP1Percent,
			Default:   defaults.NegTP1Percent,
			RiskLevel: "medium",
		})
	}
	if user.NegTP1AddPercent != defaults.NegTP1AddPercent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".neg_tp1_add_percent",
			Current:   user.NegTP1AddPercent,
			Default:   defaults.NegTP1AddPercent,
			RiskLevel: "medium",
		})
	}
	if user.NegTP2Percent != defaults.NegTP2Percent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".neg_tp2_percent",
			Current:   user.NegTP2Percent,
			Default:   defaults.NegTP2Percent,
			RiskLevel: "medium",
		})
	}
	if user.NegTP2AddPercent != defaults.NegTP2AddPercent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".neg_tp2_add_percent",
			Current:   user.NegTP2AddPercent,
			Default:   defaults.NegTP2AddPercent,
			RiskLevel: "medium",
		})
	}
	if user.NegTP3Percent != defaults.NegTP3Percent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".neg_tp3_percent",
			Current:   user.NegTP3Percent,
			Default:   defaults.NegTP3Percent,
			RiskLevel: "medium",
		})
	}
	if user.NegTP3AddPercent != defaults.NegTP3AddPercent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".neg_tp3_add_percent",
			Current:   user.NegTP3AddPercent,
			Default:   defaults.NegTP3AddPercent,
			RiskLevel: "medium",
		})
	}

	// Compare reentry settings
	if user.ReentryPercent != defaults.ReentryPercent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".reentry_percent",
			Current:   user.ReentryPercent,
			Default:   defaults.ReentryPercent,
			RiskLevel: "low",
		})
	}
	if user.ReentryMinADX != defaults.ReentryMinADX {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".reentry_min_adx",
			Current:   user.ReentryMinADX,
			Default:   defaults.ReentryMinADX,
			RiskLevel: "low",
		})
	}

	// Compare AI settings
	if user.UseAIDecisions != defaults.UseAIDecisions {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".use_ai_decisions",
			Current:   user.UseAIDecisions,
			Default:   defaults.UseAIDecisions,
			RiskLevel: "medium",
		})
	}

	// Compare hedge mode
	if user.HedgeModeEnabled != defaults.HedgeModeEnabled {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".hedge_mode_enabled",
			Current:   user.HedgeModeEnabled,
			Default:   defaults.HedgeModeEnabled,
			RiskLevel: "high",
		})
	}

	// Compare DCA on loss
	if user.DCAOnLoss != defaults.DCAOnLoss {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".dca_on_loss",
			Current:   user.DCAOnLoss,
			Default:   defaults.DCAOnLoss,
			RiskLevel: "high",
		})
	}

	// Compare profit protection
	if user.ProfitProtectionEnabled != defaults.ProfitProtectionEnabled {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".profit_protection_enabled",
			Current:   user.ProfitProtectionEnabled,
			Default:   defaults.ProfitProtectionEnabled,
			RiskLevel: "medium",
		})
	}
	if user.ProfitProtectionPercent != defaults.ProfitProtectionPercent {
		diffs = append(diffs, Difference{
			Path:      posOptPrefix + ".profit_protection_percent",
			Current:   user.ProfitProtectionPercent,
			Default:   defaults.ProfitProtectionPercent,
			RiskLevel: "low",
		})
	}

	return diffs
}

// compareGlobalTrading compares global trading settings
func (s *Server) compareGlobalTrading(
	user autopilot.GlobalTradingDefaults,
	defaults autopilot.GlobalTradingDefaults,
	riskIndex autopilot.SettingsRiskIndex,
) []Difference {
	var diffs []Difference
	prefix := "global_trading"

	if user.RiskLevel != defaults.RiskLevel {
		riskLevel, impact, recommendation := s.getRiskInfo(prefix+".risk_level", riskIndex, defaults.RiskInfo)
		diffs = append(diffs, Difference{
			Path:           prefix + ".risk_level",
			Current:        user.RiskLevel,
			Default:        defaults.RiskLevel,
			RiskLevel:      riskLevel,
			Impact:         impact,
			Recommendation: recommendation,
		})
	}

	if user.MaxUSDAllocation != defaults.MaxUSDAllocation {
		riskLevel, impact, recommendation := s.getRiskInfo(prefix+".max_usd_allocation", riskIndex, defaults.RiskInfo)
		diffs = append(diffs, Difference{
			Path:           prefix + ".max_usd_allocation",
			Current:        user.MaxUSDAllocation,
			Default:        defaults.MaxUSDAllocation,
			RiskLevel:      riskLevel,
			Impact:         impact,
			Recommendation: recommendation,
		})
	}

	return diffs
}

// compareCircuitBreaker compares circuit breaker settings
func (s *Server) compareCircuitBreaker(
	user autopilot.CircuitBreakerDefaults,
	defaults autopilot.CircuitBreakerDefaults,
	riskIndex autopilot.SettingsRiskIndex,
) []Difference {
	var diffs []Difference
	prefix := "circuit_breaker.global"

	if user.Global.Enabled != defaults.Global.Enabled {
		riskLevel, impact, recommendation := s.getRiskInfo("circuit_breaker.global.enabled", riskIndex, defaults.RiskInfo)
		diffs = append(diffs, Difference{
			Path:           prefix + ".enabled",
			Current:        user.Global.Enabled,
			Default:        defaults.Global.Enabled,
			RiskLevel:      riskLevel,
			Impact:         impact,
			Recommendation: recommendation,
		})
	}

	if user.Global.MaxDailyLoss != defaults.Global.MaxDailyLoss {
		riskLevel, impact, recommendation := s.getRiskInfo(prefix+".max_daily_loss", riskIndex, nil)
		diffs = append(diffs, Difference{
			Path:           prefix + ".max_daily_loss",
			Current:        user.Global.MaxDailyLoss,
			Default:        defaults.Global.MaxDailyLoss,
			RiskLevel:      riskLevel,
			Impact:         impact,
			Recommendation: recommendation,
		})
	}

	return diffs
}

// comparePositionOptimization compares position optimization settings
func (s *Server) comparePositionOptimization(
	user autopilot.PositionOptimizationDefaults,
	defaults autopilot.PositionOptimizationDefaults,
	riskIndex autopilot.SettingsRiskIndex,
) []Difference {
	var diffs []Difference

	// Compare averaging settings
	if user.Averaging.Enabled != defaults.Averaging.Enabled {
		riskLevel, impact, recommendation := s.getRiskInfo("position_optimization.averaging.enabled", riskIndex, defaults.RiskInfo)
		diffs = append(diffs, Difference{
			Path:           "position_optimization.averaging.enabled",
			Current:        user.Averaging.Enabled,
			Default:        defaults.Averaging.Enabled,
			RiskLevel:      riskLevel,
			Impact:         impact,
			Recommendation: recommendation,
		})
	}

	// Compare hedging settings
	if user.Hedging.Enabled != defaults.Hedging.Enabled {
		riskLevel, impact, recommendation := s.getRiskInfo("position_optimization.hedging.enabled", riskIndex, defaults.RiskInfo)
		diffs = append(diffs, Difference{
			Path:           "position_optimization.hedging.enabled",
			Current:        user.Hedging.Enabled,
			Default:        defaults.Hedging.Enabled,
			RiskLevel:      riskLevel,
			Impact:         impact,
			Recommendation: recommendation,
		})
	}

	return diffs
}

// compareLLMConfig compares LLM configuration settings
func (s *Server) compareLLMConfig(
	user autopilot.LLMConfigDefaults,
	defaults autopilot.LLMConfigDefaults,
	riskIndex autopilot.SettingsRiskIndex,
) []Difference {
	var diffs []Difference
	prefix := "llm_config.global"

	if user.Global.Enabled != defaults.Global.Enabled {
		riskLevel, impact, recommendation := s.getRiskInfo(prefix+".enabled", riskIndex, defaults.RiskInfo)
		diffs = append(diffs, Difference{
			Path:           prefix + ".enabled",
			Current:        user.Global.Enabled,
			Default:        defaults.Global.Enabled,
			RiskLevel:      riskLevel,
			Impact:         impact,
			Recommendation: recommendation,
		})
	}

	return diffs
}

// Note: compareEarlyWarning removed - early_warning is now per-mode only
// Per-mode early_warning comparison happens within compareModePositionOptimization

// compareCapitalAllocation compares capital allocation settings
func (s *Server) compareCapitalAllocation(
	user autopilot.CapitalAllocationDefaults,
	defaults autopilot.CapitalAllocationDefaults,
	riskIndex autopilot.SettingsRiskIndex,
) []Difference {
	var diffs []Difference
	prefix := "capital_allocation"

	if user.UltraFastPercent != defaults.UltraFastPercent {
		riskLevel, impact, recommendation := s.getRiskInfo(prefix+".ultra_fast_percent", riskIndex, nil)
		diffs = append(diffs, Difference{
			Path:           prefix + ".ultra_fast_percent",
			Current:        user.UltraFastPercent,
			Default:        defaults.UltraFastPercent,
			RiskLevel:      riskLevel,
			Impact:         impact,
			Recommendation: recommendation,
		})
	}

	if user.ScalpPercent != defaults.ScalpPercent {
		riskLevel, impact, recommendation := s.getRiskInfo(prefix+".scalp_percent", riskIndex, nil)
		diffs = append(diffs, Difference{
			Path:           prefix + ".scalp_percent",
			Current:        user.ScalpPercent,
			Default:        defaults.ScalpPercent,
			RiskLevel:      riskLevel,
			Impact:         impact,
			Recommendation: recommendation,
		})
	}

	return diffs
}

// getRiskInfo determines risk level and info for a setting
func (s *Server) getRiskInfo(
	path string,
	riskIndex autopilot.SettingsRiskIndex,
	riskInfoMap map[string]autopilot.RiskInfo,
) (riskLevel, impact, recommendation string) {
	// Default to low risk
	riskLevel = "low"
	impact = "Minimal impact on trading performance"
	recommendation = "Customize as needed"

	// Check if path is in high risk settings
	for _, highRiskPath := range riskIndex.HighRiskSettings {
		if strings.Contains(path, highRiskPath) || matchesRiskPattern(path, highRiskPath) {
			riskLevel = "high"
			break
		}
	}

	// Check if path is in medium risk settings
	if riskLevel == "low" {
		for _, mediumRiskPath := range riskIndex.MediumRiskSettings {
			if strings.Contains(path, mediumRiskPath) || matchesRiskPattern(path, mediumRiskPath) {
				riskLevel = "medium"
				break
			}
		}
	}

	// Get specific risk info from the risk info map if available
	if riskInfoMap != nil {
		// Extract the key from path (e.g., "enabled" from "mode_configs.ultra_fast.enabled")
		parts := strings.Split(path, ".")
		if len(parts) > 0 {
			key := parts[len(parts)-1]
			if info, exists := riskInfoMap[key]; exists {
				impact = info.Impact
				recommendation = info.Recommendation
			}
		}
	}

	// Set default messages based on risk level
	if impact == "" {
		switch riskLevel {
		case "high":
			impact = "High impact on risk and potential losses"
			recommendation = "Review carefully before changing"
		case "medium":
			impact = "Moderate impact on trading behavior"
			recommendation = "Test in paper trading first"
		case "low":
			impact = "Minimal impact on trading performance"
			recommendation = "Customize as needed"
		}
	}

	return riskLevel, impact, recommendation
}

// matchesRiskPattern checks if a path matches a risk pattern
// Handles wildcards like "mode_configs.*.leverage"
func matchesRiskPattern(path, pattern string) bool {
	// Handle "enabled=false" pattern
	if strings.Contains(pattern, "=") {
		return false // Skip these for now
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		patternParts := strings.Split(pattern, ".")
		pathParts := strings.Split(path, ".")

		if len(patternParts) != len(pathParts) {
			return false
		}

		for i, part := range patternParts {
			if part != "*" && part != pathParts[i] {
				return false
			}
		}
		return true
	}

	return strings.Contains(path, pattern)
}

// ==================== RESET HELPERS ====================

// resetModeConfigSetting resets a single mode config setting to default
func (s *Server) resetModeConfigSetting(
	ctx context.Context,
	userID, modeName string,
	subPath []string,
	defaults *autopilot.DefaultSettingsFile,
) error {
	// Get default mode config
	defaultMode, exists := defaults.ModeConfigs[modeName]
	if !exists {
		return fmt.Errorf("mode %s not found in defaults", modeName)
	}

	// Get user's current config
	userConfigJSON, err := s.repo.GetUserModeConfig(ctx, userID, modeName)
	if err != nil {
		return err
	}

	var userConfig autopilot.ModeFullConfig
	if userConfigJSON != nil {
		if err := json.Unmarshal(userConfigJSON, &userConfig); err != nil {
			return err
		}
	} else {
		// No user config exists, copy from defaults
		userConfig = *defaultMode
	}

	// Reset the specific field based on subPath
	if err := s.resetModeConfigField(&userConfig, defaultMode, subPath); err != nil {
		return err
	}

	// Save updated config
	updatedJSON, err := json.Marshal(userConfig)
	if err != nil {
		return err
	}

	return s.repo.SaveUserModeConfig(ctx, userID, modeName, userConfig.Enabled, updatedJSON)
}

// resetModeConfigField resets a specific field in mode config
func (s *Server) resetModeConfigField(
	userConfig, defaultConfig *autopilot.ModeFullConfig,
	subPath []string,
) error {
	if len(subPath) == 0 {
		return fmt.Errorf("empty subPath")
	}

	field := subPath[0]
	switch field {
	case "enabled":
		userConfig.Enabled = defaultConfig.Enabled
	case "confidence":
		if len(subPath) > 1 {
			switch subPath[1] {
			case "min_confidence":
				userConfig.Confidence.MinConfidence = defaultConfig.Confidence.MinConfidence
			case "high_confidence":
				userConfig.Confidence.HighConfidence = defaultConfig.Confidence.HighConfidence
			}
		} else {
			userConfig.Confidence = defaultConfig.Confidence
		}
	case "size":
		if len(subPath) > 1 {
			switch subPath[1] {
			case "leverage":
				userConfig.Size.Leverage = defaultConfig.Size.Leverage
			default:
				// Reset entire size config
				userConfig.Size = defaultConfig.Size
			}
		} else {
			userConfig.Size = defaultConfig.Size
		}
	case "sltp":
		if len(subPath) > 1 {
			switch subPath[1] {
			case "stop_loss_percent":
				userConfig.SLTP.StopLossPercent = defaultConfig.SLTP.StopLossPercent
			case "take_profit_percent":
				userConfig.SLTP.TakeProfitPercent = defaultConfig.SLTP.TakeProfitPercent
			default:
				userConfig.SLTP = defaultConfig.SLTP
			}
		} else {
			userConfig.SLTP = defaultConfig.SLTP
		}
	default:
		return fmt.Errorf("unknown mode config field: %s", field)
	}

	return nil
}

// Placeholder reset functions for other setting groups
func (s *Server) resetGlobalTradingSetting(ctx context.Context, userID string, subPath []string, defaults *autopilot.DefaultSettingsFile) error {
	// TODO: Implement when global trading settings are stored per-user
	return fmt.Errorf("global trading settings reset not yet implemented")
}

func (s *Server) resetCircuitBreakerSetting(ctx context.Context, userID string, subPath []string, defaults *autopilot.DefaultSettingsFile) error {
	// TODO: Implement when circuit breaker settings are stored per-user
	return fmt.Errorf("circuit breaker settings reset not yet implemented")
}

func (s *Server) resetPositionOptimizationSetting(ctx context.Context, userID string, subPath []string, defaults *autopilot.DefaultSettingsFile) error {
	// TODO: Implement when position optimization settings are stored per-user
	return fmt.Errorf("position optimization settings reset not yet implemented")
}

func (s *Server) resetLLMConfigSetting(ctx context.Context, userID string, subPath []string, defaults *autopilot.DefaultSettingsFile) error {
	// TODO: Implement when LLM config settings are stored per-user
	return fmt.Errorf("LLM config settings reset not yet implemented")
}

// Note: resetEarlyWarningSetting removed - early_warning is now per-mode only

func (s *Server) resetCapitalAllocationSetting(ctx context.Context, userID string, subPath []string, defaults *autopilot.DefaultSettingsFile) error {
	// TODO: Implement when capital allocation settings are stored per-user
	return fmt.Errorf("capital allocation settings reset not yet implemented")
}

// ==================== USER SETTINGS LOADER ====================

// loadUserSettings loads all settings for a user from database
// Falls back to defaults if user has not customized a setting
// Story 6.4: Accepts pre-loaded defaults to avoid redundant cache/file reads
func (s *Server) loadUserSettings(ctx context.Context, userID string) (*autopilot.DefaultSettingsFile, error) {
	// Load defaults from cache or file as baseline
	var defaults *autopilot.DefaultSettingsFile
	var err error

	if s.adminDefaultsCacheService != nil && s.adminDefaultsCacheService.IsHealthy() {
		defaults, err = s.adminDefaultsCacheService.GetAllAdminDefaults(ctx)
		if err != nil {
			log.Printf("[SETTINGS-LOADER] Cache unavailable, falling back to file: %v", err)
			defaults, err = autopilot.LoadDefaultSettings()
		}
	} else {
		defaults, err = autopilot.LoadDefaultSettings()
	}

	if err != nil {
		return nil, err
	}

	// Create a copy to populate with user settings
	// NOTE: Global EarlyWarning removed - early warning is now per-mode only (in mode configs)
	userSettings := &autopilot.DefaultSettingsFile{
		Metadata:             defaults.Metadata,
		GlobalTrading:        defaults.GlobalTrading,
		ModeConfigs:          make(map[string]*autopilot.ModeFullConfig),
		PositionOptimization: defaults.PositionOptimization,
		CircuitBreaker:       defaults.CircuitBreaker,
		LLMConfig:            defaults.LLMConfig,
		CapitalAllocation:    defaults.CapitalAllocation,
		SettingsRiskIndex:    defaults.SettingsRiskIndex,
	}

	// Load user's mode configs from database
	userModeConfigs, err := s.repo.GetAllUserModeConfigs(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Populate mode configs (use defaults if user hasn't customized)
	for modeName, defaultConfig := range defaults.ModeConfigs {
		if userConfigJSON, exists := userModeConfigs[modeName]; exists {
			var userConfig autopilot.ModeFullConfig
			if err := json.Unmarshal(userConfigJSON, &userConfig); err != nil {
				log.Printf("[SETTINGS-LOADER] Failed to unmarshal user config for mode %s: %v", modeName, err)
				userSettings.ModeConfigs[modeName] = defaultConfig
			} else {
				userSettings.ModeConfigs[modeName] = &userConfig
			}
		} else {
			// User hasn't customized this mode, use defaults
			userSettings.ModeConfigs[modeName] = defaultConfig
		}
	}

	// TODO: Load other per-user settings when implemented
	// For now, global trading, circuit breaker, etc. use defaults

	return userSettings, nil
}

// Helper to check if two values are equal (handles floats, ints, bools, strings)
func valuesEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}
