// Package api provides HTTP handlers for the Decision Engine Settings UI.
// Epic 11: Position Decision Engine - Story 11.24: Decision Engine Settings UI
package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"binance-trading-bot/internal/decision"

	"github.com/gin-gonic/gin"
)

// decisionSettingsService is the singleton instance of the decision settings service.
// This ensures settings are properly persisted and cached across requests.
var (
	decisionSettingsService     *decision.SettingsService
	decisionSettingsServiceOnce sync.Once

	// Story 11.26: Calibration lifecycle service singleton
	calibrationLifecycleService     *decision.CalibrationLifecycleService
	calibrationLifecycleServiceOnce sync.Once
)

// ==================== GET /api/decision-engine/settings ====================
// Get user's complete decision engine settings

func (s *Server) handleGetDecisionEngineSettings(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	ctx := c.Request.Context()
	settings, err := s.getDecisionSettingsService().GetSettings(ctx, userID)
	if err != nil {
		log.Printf("[DECISION-SETTINGS] Error getting settings for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get decision engine settings: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"settings": settings,
	})
}

// ==================== PUT /api/decision-engine/settings ====================
// Update user's complete decision engine settings

type UpdateDecisionSettingsRequest struct {
	ActiveStrategy string                                       `json:"active_strategy,omitempty"`
	Strategies     map[string]*decision.StrategySettings `json:"strategies,omitempty"`
}

func (s *Server) handleUpdateDecisionEngineSettings(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req UpdateDecisionSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// Get current settings
	settings, err := s.getDecisionSettingsService().GetSettings(ctx, userID)
	if err != nil {
		log.Printf("[DECISION-SETTINGS] Error getting settings for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get current settings: "+err.Error())
		return
	}

	// Update active strategy if provided
	if req.ActiveStrategy != "" {
		strategyName := decision.StrategyName(req.ActiveStrategy)
		if !strategyName.IsValid() {
			errorResponse(c, http.StatusBadRequest, "Invalid strategy name: "+req.ActiveStrategy)
			return
		}
		settings.ActiveStrategy = strategyName
	}

	// Update strategies if provided
	if req.Strategies != nil {
		for name, stratSettings := range req.Strategies {
			strategyName := decision.StrategyName(name)
			if !strategyName.IsValid() {
				errorResponse(c, http.StatusBadRequest, "Invalid strategy name: "+name)
				return
			}
			stratSettings.Name = strategyName
			settings.SetStrategy(stratSettings)
		}
	}

	// Save updated settings
	if err := s.getDecisionSettingsService().SaveSettings(ctx, userID, settings); err != nil {
		log.Printf("[DECISION-SETTINGS] Error saving settings for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to save settings: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Decision engine settings updated",
		"settings": settings,
	})
}

// ==================== POST /api/decision-engine/settings/reset ====================
// Reset user's settings to defaults

type ResetDecisionSettingsRequest struct {
	Strategy string `json:"strategy,omitempty"` // Optional: reset only specific strategy
	// Group field is reserved for future use - group-level reset not yet implemented
	// Group    string `json:"group,omitempty"`
}

func (s *Server) handleResetDecisionEngineSettings(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req ResetDecisionSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// No body is fine - reset all
		req = ResetDecisionSettingsRequest{}
	}

	ctx := c.Request.Context()

	if req.Strategy != "" {
		// Reset specific strategy
		strategyName := decision.StrategyName(req.Strategy)
		if !strategyName.IsValid() {
			errorResponse(c, http.StatusBadRequest, "Invalid strategy name: "+req.Strategy)
			return
		}

		if err := s.getDecisionSettingsService().ResetStrategyToDefaults(ctx, userID, strategyName); err != nil {
			log.Printf("[DECISION-SETTINGS] Error resetting strategy %s for user %s: %v", req.Strategy, userID, err)
			errorResponse(c, http.StatusInternalServerError, "Failed to reset strategy: "+err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Strategy " + req.Strategy + " reset to defaults",
		})
		return
	}

	// Reset all settings
	if err := s.getDecisionSettingsService().ResetToDefaults(ctx, userID); err != nil {
		log.Printf("[DECISION-SETTINGS] Error resetting all settings for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to reset settings: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All decision engine settings reset to defaults",
	})
}

// ==================== GET /api/decision-engine/settings/compare ====================
// Compare user settings vs defaults for the Settings Comparison UI

// DEFieldComparison represents a field comparison for decision engine settings.
// Named with DE prefix to avoid conflict with FieldComparison in handlers_settings_defaults.go.
type DEFieldComparison struct {
	Path    string      `json:"path"`
	Current interface{} `json:"current"`
	Default interface{} `json:"default"`
	Match   bool        `json:"match"`
}

// DEStrategyComparison represents a strategy comparison result.
type DEStrategyComparison struct {
	StrategyName   string                     `json:"strategy_name"`
	Enabled        bool                       `json:"enabled"`
	TotalFields    int                        `json:"total_fields"`
	MatchingFields int                        `json:"matching_fields"`
	Groups         map[string]*DEGroupComparison `json:"groups"`
}

// DEGroupComparison represents a group comparison within a strategy.
type DEGroupComparison struct {
	GroupName      string              `json:"group_name"`
	TotalFields    int                 `json:"total_fields"`
	MatchingFields int                 `json:"matching_fields"`
	AllMatch       bool                `json:"all_match"`
	Fields         []DEFieldComparison `json:"fields"`
}

func (s *Server) handleCompareDecisionEngineSettings(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	ctx := c.Request.Context()

	// Get user's current settings
	userSettings, err := s.getDecisionSettingsService().GetSettings(ctx, userID)
	if err != nil {
		log.Printf("[DECISION-SETTINGS] Error getting settings for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get settings: "+err.Error())
		return
	}

	// Get default settings
	defaultSettings := decision.DefaultDecisionEngineSettings()

	// Compare each strategy
	comparisons := make(map[string]*DEStrategyComparison)

	for stratName := range defaultSettings.Strategies {
		userStrat := userSettings.GetStrategy(stratName)
		defaultStrat := defaultSettings.GetStrategy(stratName)

		if userStrat == nil || defaultStrat == nil {
			continue
		}

		comparison := compareStrategy(userStrat, defaultStrat)
		comparisons[string(stratName)] = comparison
	}

	// Calculate overall statistics
	totalStrategies := len(comparisons)
	totalFields := 0
	matchingFields := 0
	for _, comp := range comparisons {
		totalFields += comp.TotalFields
		matchingFields += comp.MatchingFields
	}

	allMatch := totalFields == matchingFields

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"preview":           true,
		"config_type":       "decision_engine",
		"all_match":         allMatch,
		"total_strategies":  totalStrategies,
		"total_fields":      totalFields,
		"matching_fields":   matchingFields,
		"total_differences": totalFields - matchingFields,
		"strategies":        comparisons,
		"active_strategy": gin.H{
			"current": userSettings.ActiveStrategy,
			"default": defaultSettings.ActiveStrategy,
			"match":   userSettings.ActiveStrategy == defaultSettings.ActiveStrategy,
		},
	})
}

// compareStrategy compares user strategy settings with defaults
func compareStrategy(user, def *decision.StrategySettings) *DEStrategyComparison {
	comp := &DEStrategyComparison{
		StrategyName: string(user.Name),
		Enabled:      user.Enabled,
		Groups:       make(map[string]*DEGroupComparison),
	}

	// Compare Scoring group
	if user.Scoring != nil && def.Scoring != nil {
		scoringGroup := compareScoring(user.Scoring, def.Scoring)
		comp.Groups["scoring"] = scoringGroup
		comp.TotalFields += scoringGroup.TotalFields
		comp.MatchingFields += scoringGroup.MatchingFields
	}

	// Compare Blocking group
	if user.Blocking != nil && def.Blocking != nil {
		blockingGroup := compareBlocking(user.Blocking, def.Blocking)
		comp.Groups["blocking"] = blockingGroup
		comp.TotalFields += blockingGroup.TotalFields
		comp.MatchingFields += blockingGroup.MatchingFields
	}

	// Compare Entry group
	if user.Entry != nil && def.Entry != nil {
		entryGroup := compareEntry(user.Entry, def.Entry)
		comp.Groups["entry"] = entryGroup
		comp.TotalFields += entryGroup.TotalFields
		comp.MatchingFields += entryGroup.MatchingFields
	}

	// Compare Exit group
	if user.Exit != nil && def.Exit != nil {
		exitGroup := compareExit(user.Exit, def.Exit)
		comp.Groups["exit"] = exitGroup
		comp.TotalFields += exitGroup.TotalFields
		comp.MatchingFields += exitGroup.MatchingFields
	}

	// Compare timeframes
	timeframeGroup := &DEGroupComparison{
		GroupName: "timeframes",
		Fields:    make([]DEFieldComparison, 0),
	}
	timeframeGroup.Fields = append(timeframeGroup.Fields, DEFieldComparison{
		Path:    "primary_timeframe",
		Current: user.PrimaryTimeframe,
		Default: def.PrimaryTimeframe,
		Match:   user.PrimaryTimeframe == def.PrimaryTimeframe,
	})
	timeframeGroup.Fields = append(timeframeGroup.Fields, DEFieldComparison{
		Path:    "secondary_timeframe",
		Current: user.SecondaryTimeframe,
		Default: def.SecondaryTimeframe,
		Match:   user.SecondaryTimeframe == def.SecondaryTimeframe,
	})
	timeframeGroup.TotalFields = len(timeframeGroup.Fields)
	for _, f := range timeframeGroup.Fields {
		if f.Match {
			timeframeGroup.MatchingFields++
		}
	}
	timeframeGroup.AllMatch = timeframeGroup.TotalFields == timeframeGroup.MatchingFields
	comp.Groups["timeframes"] = timeframeGroup
	comp.TotalFields += timeframeGroup.TotalFields
	comp.MatchingFields += timeframeGroup.MatchingFields

	// Compare enabled status
	enabledGroup := &DEGroupComparison{
		GroupName: "general",
		Fields: []DEFieldComparison{
			{
				Path:    "enabled",
				Current: user.Enabled,
				Default: def.Enabled,
				Match:   user.Enabled == def.Enabled,
			},
		},
		TotalFields: 1,
	}
	if user.Enabled == def.Enabled {
		enabledGroup.MatchingFields = 1
	}
	enabledGroup.AllMatch = enabledGroup.TotalFields == enabledGroup.MatchingFields
	comp.Groups["general"] = enabledGroup
	comp.TotalFields += enabledGroup.TotalFields
	comp.MatchingFields += enabledGroup.MatchingFields

	return comp
}

func compareScoring(user, def *decision.ScoringConfig) *DEGroupComparison {
	group := &DEGroupComparison{
		GroupName: "scoring",
		Fields:    make([]DEFieldComparison, 0),
	}

	addField(group, "technical_weight", user.TechnicalWeight, def.TechnicalWeight)
	addField(group, "context_weight", user.ContextWeight, def.ContextWeight)
	addField(group, "llm_weight", user.LLMWeight, def.LLMWeight)
	addField(group, "history_weight", user.HistoryWeight, def.HistoryWeight)
	addField(group, "min_threshold", user.MinThreshold, def.MinThreshold)

	group.TotalFields = len(group.Fields)
	group.AllMatch = group.TotalFields == group.MatchingFields
	return group
}

func compareBlocking(user, def *decision.BlockingConfig) *DEGroupComparison {
	group := &DEGroupComparison{
		GroupName: "blocking",
		Fields:    make([]DEFieldComparison, 0),
	}

	addField(group, "adx_minimum", user.ADXMinimum, def.ADXMinimum)
	addField(group, "score_minimum", user.ScoreMinimum, def.ScoreMinimum)
	addField(group, "rsi_upper_limit", user.RSIUpperLimit, def.RSIUpperLimit)
	addField(group, "rsi_lower_limit", user.RSILowerLimit, def.RSILowerLimit)
	addField(group, "win_rate_minimum", user.WinRateMinimum, def.WinRateMinimum)
	addFieldInt(group, "max_blocking_reasons", user.MaxBlockingReasons, def.MaxBlockingReasons)
	addFieldBool(group, "enable_hard_blocks", user.EnableHardBlocks, def.EnableHardBlocks)
	addFieldBool(group, "enable_soft_blocks", user.EnableSoftBlocks, def.EnableSoftBlocks)
	addFieldBool(group, "enable_warnings", user.EnableWarnings, def.EnableWarnings)

	group.TotalFields = len(group.Fields)
	group.AllMatch = group.TotalFields == group.MatchingFields
	return group
}

func compareEntry(user, def *decision.EntryConditions) *DEGroupComparison {
	group := &DEGroupComparison{
		GroupName: "entry",
		Fields:    make([]DEFieldComparison, 0),
	}

	addField(group, "min_score", user.MinScore, def.MinScore)
	addField(group, "min_adx", user.MinADX, def.MinADX)
	addFieldBool(group, "require_trend_align", user.RequireTrendAlign, def.RequireTrendAlign)
	addField(group, "volume_multiplier", user.VolumeMultiplier, def.VolumeMultiplier)
	addField(group, "min_confidence", user.MinConfidence, def.MinConfidence)
	addField(group, "max_spread", user.MaxSpread, def.MaxSpread)

	group.TotalFields = len(group.Fields)
	group.AllMatch = group.TotalFields == group.MatchingFields
	return group
}

func compareExit(user, def *decision.ExitConditions) *DEGroupComparison {
	group := &DEGroupComparison{
		GroupName: "exit",
		Fields:    make([]DEFieldComparison, 0),
	}

	addField(group, "take_profit_percent", user.TakeProfitPercent, def.TakeProfitPercent)
	addField(group, "stop_loss_percent", user.StopLossPercent, def.StopLossPercent)
	addFieldBool(group, "trailing_stop", user.TrailingStop, def.TrailingStop)
	addField(group, "trailing_percent", user.TrailingPercent, def.TrailingPercent)
	addFieldStr(group, "max_hold_time", user.MaxHoldTime, def.MaxHoldTime)
	addFieldBool(group, "exit_on_trend_reversal", user.ExitOnTrendReversal, def.ExitOnTrendReversal)
	addFieldStr(group, "min_hold_time", user.MinHoldTime, def.MinHoldTime)

	group.TotalFields = len(group.Fields)
	group.AllMatch = group.TotalFields == group.MatchingFields
	return group
}

func addField(group *DEGroupComparison, path string, current, def float64) {
	match := current == def
	group.Fields = append(group.Fields, DEFieldComparison{
		Path:    path,
		Current: current,
		Default: def,
		Match:   match,
	})
	if match {
		group.MatchingFields++
	}
}

func addFieldBool(group *DEGroupComparison, path string, current, def bool) {
	match := current == def
	group.Fields = append(group.Fields, DEFieldComparison{
		Path:    path,
		Current: current,
		Default: def,
		Match:   match,
	})
	if match {
		group.MatchingFields++
	}
}

func addFieldStr(group *DEGroupComparison, path string, current, def string) {
	match := current == def
	group.Fields = append(group.Fields, DEFieldComparison{
		Path:    path,
		Current: current,
		Default: def,
		Match:   match,
	})
	if match {
		group.MatchingFields++
	}
}

func addFieldInt(group *DEGroupComparison, path string, current, def int) {
	match := current == def
	group.Fields = append(group.Fields, DEFieldComparison{
		Path:    path,
		Current: current,
		Default: def,
		Match:   match,
	})
	if match {
		group.MatchingFields++
	}
}

// ==================== GET /api/decision-engine/strategies ====================
// List available strategies with their metadata

func (s *Server) handleListDecisionEngineStrategies(c *gin.Context) {
	strategies := decision.AllStrategies()
	defaults := decision.DefaultDecisionEngineSettings()

	strategyList := make([]gin.H, 0, len(strategies))
	for _, name := range strategies {
		stratSettings := defaults.GetStrategy(name)
		if stratSettings != nil {
			strategyList = append(strategyList, gin.H{
				"name":               string(name),
				"display_name":       formatStrategyName(string(name)),
				"description":        stratSettings.Description,
				"primary_timeframe":  stratSettings.PrimaryTimeframe,
				"secondary_timeframe": stratSettings.SecondaryTimeframe,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"strategies": strategyList,
	})
}

// ==================== PUT /api/decision-engine/strategy/:name ====================
// Update a specific strategy

func (s *Server) handleUpdateDecisionEngineStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	strategyNameStr := c.Param("name")
	strategyName := decision.StrategyName(strategyNameStr)
	if !strategyName.IsValid() {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy name: "+strategyNameStr)
		return
	}

	var stratSettings decision.StrategySettings
	if err := c.ShouldBindJSON(&stratSettings); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	stratSettings.Name = strategyName

	ctx := c.Request.Context()
	if err := s.getDecisionSettingsService().UpdateStrategySettings(ctx, userID, strategyName, &stratSettings); err != nil {
		log.Printf("[DECISION-SETTINGS] Error updating strategy %s for user %s: %v", strategyNameStr, userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to update strategy: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Strategy " + strategyNameStr + " updated",
		"strategy": stratSettings,
	})
}

// ==================== POST /api/decision-engine/strategy/:name/reset ====================
// Reset a specific strategy to defaults

func (s *Server) handleResetDecisionEngineStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	strategyNameStr := c.Param("name")
	strategyName := decision.StrategyName(strategyNameStr)
	if !strategyName.IsValid() {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy name: "+strategyNameStr)
		return
	}

	ctx := c.Request.Context()
	if err := s.getDecisionSettingsService().ResetStrategyToDefaults(ctx, userID, strategyName); err != nil {
		log.Printf("[DECISION-SETTINGS] Error resetting strategy %s for user %s: %v", strategyNameStr, userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to reset strategy: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Strategy " + strategyNameStr + " reset to defaults",
	})
}

// ==================== PUT /api/decision-engine/active-strategy ====================
// Set the active strategy

type SetActiveStrategyRequest struct {
	Strategy string `json:"strategy" binding:"required"`
}

func (s *Server) handleSetActiveStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req SetActiveStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	strategyName := decision.StrategyName(req.Strategy)
	if !strategyName.IsValid() {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy name: "+req.Strategy)
		return
	}

	ctx := c.Request.Context()
	if err := s.getDecisionSettingsService().SetActiveStrategy(ctx, userID, strategyName); err != nil {
		log.Printf("[DECISION-SETTINGS] Error setting active strategy to %s for user %s: %v", req.Strategy, userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to set active strategy: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "Active strategy changed to " + req.Strategy,
		"active_strategy": req.Strategy,
	})
}

// ==================== POST /api/decision-engine/strategy/:name/enable ====================
// Enable a strategy

func (s *Server) handleEnableStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	strategyNameStr := c.Param("name")
	strategyName := decision.StrategyName(strategyNameStr)
	if !strategyName.IsValid() {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy name: "+strategyNameStr)
		return
	}

	ctx := c.Request.Context()
	if err := s.getDecisionSettingsService().EnableStrategy(ctx, userID, strategyName); err != nil {
		log.Printf("[DECISION-SETTINGS] Error enabling strategy %s for user %s: %v", strategyNameStr, userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to enable strategy: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Strategy " + strategyNameStr + " enabled",
	})
}

// ==================== POST /api/decision-engine/strategy/:name/disable ====================
// Disable a strategy

func (s *Server) handleDisableStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	strategyNameStr := c.Param("name")
	strategyName := decision.StrategyName(strategyNameStr)
	if !strategyName.IsValid() {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy name: "+strategyNameStr)
		return
	}

	ctx := c.Request.Context()
	if err := s.getDecisionSettingsService().DisableStrategy(ctx, userID, strategyName); err != nil {
		log.Printf("[DECISION-SETTINGS] Error disabling strategy %s for user %s: %v", strategyNameStr, userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to disable strategy: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Strategy " + strategyNameStr + " disabled",
	})
}

// Helper to format strategy name for display
func formatStrategyName(name string) string {
	switch name {
	case "trend_following":
		return "Trend Following"
	case "mean_reversion":
		return "Mean Reversion"
	case "breakout":
		return "Breakout"
	case "range_trading":
		return "Range Trading"
	default:
		return name
	}
}

// getDecisionSettingsService returns or creates the singleton decision settings service.
// Uses sync.Once to ensure thread-safe lazy initialization with Redis support.
func (s *Server) getDecisionSettingsService() *decision.SettingsService {
	decisionSettingsServiceOnce.Do(func() {
		// Try to get Redis client from existing services
		var redisClient interface{ GetRedisClient() interface{} }

		// Check if we have a settings cache service with Redis
		if s.settingsCacheService != nil {
			// The SettingsCacheService has Redis, but we need direct access
			// For now, create without Redis - in-memory only
			// TODO: Consider adding Redis client getter to SettingsCacheService
			log.Println("[DECISION-SETTINGS] Initializing settings service (in-memory cache)")
			decisionSettingsService = decision.NewSettingsService(nil)
		} else {
			log.Println("[DECISION-SETTINGS] Initializing settings service (in-memory cache, no cache service)")
			decisionSettingsService = decision.NewSettingsService(nil)
		}

		// Suppress unused variable warning
		_ = redisClient
	})
	return decisionSettingsService
}

// ============================================================================
// Story 11.26: Calibration Data Lifecycle API Endpoints
// ============================================================================

// getCalibrationLifecycleService returns or creates the singleton calibration lifecycle service.
func (s *Server) getCalibrationLifecycleService() *decision.CalibrationLifecycleService {
	calibrationLifecycleServiceOnce.Do(func() {
		// Initialize the calibration service first (from Story 11-25)
		calibrationSvc := s.getCalibrationService()
		if calibrationSvc != nil {
			calibrationLifecycleService = decision.NewCalibrationLifecycleService(calibrationSvc)
			log.Println("[CALIBRATION-LIFECYCLE] Initialized calibration lifecycle service")
		} else {
			log.Println("[CALIBRATION-LIFECYCLE] Warning: Could not initialize calibration lifecycle service (no calibration service)")
		}
	})
	return calibrationLifecycleService
}

// calibrationService is the singleton instance of the calibration service.
var (
	calibrationService     *decision.CalibrationService
	calibrationServiceOnce sync.Once
)

// getCalibrationService returns or creates the singleton calibration service (from Story 11-25).
// Uses sync.Once for thread-safe lazy initialization with Redis support.
// Note: DB persistence is handled separately through the CalibrationService.SetDBRepository method
// when the appropriate adapter is available.
func (s *Server) getCalibrationService() *decision.CalibrationService {
	calibrationServiceOnce.Do(func() {
		// Get Redis client from settings cache service if available
		if s.settingsCacheService != nil {
			redisClient := s.settingsCacheService.GetRedisClient()
			if redisClient != nil {
				// Initialize with Redis-only for now
				// DB persistence can be added later with SetDBRepository if an adapter is created
				calibrationService = decision.NewCalibrationService(redisClient)
				log.Println("[CALIBRATION] Initialized calibration service with Redis")
			} else {
				log.Println("[CALIBRATION] Warning: No Redis client available for calibration service")
			}
		} else {
			log.Println("[CALIBRATION] Warning: No settings cache service available for calibration service")
		}
	})
	return calibrationService
}

// parseUserIDInt safely parses a string user ID to an integer.
// Handles both numeric strings and returns an error for invalid formats.
func parseUserIDInt(userID string) (int, error) {
	if userID == "" {
		return 0, fmt.Errorf("user ID is empty")
	}

	// Handle UUID format (00000000-0000-0000-0000-000000000000)
	// Default admin UUID should map to user ID 1
	if userID == "00000000-0000-0000-0000-000000000000" {
		return 1, nil
	}

	// Try parsing as integer
	id, err := strconv.Atoi(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID format: %s", userID)
	}

	if id <= 0 {
		return 0, fmt.Errorf("user ID must be positive: %d", id)
	}

	return id, nil
}

// ==================== POST /api/calibration/reset ====================
// Manually reset calibration data for a user

type ResetCalibrationRequest struct {
	Strategy      string `json:"strategy" binding:"required"`
	IndicatorHash string `json:"indicator_hash,omitempty"` // Optional, reset all if not provided
}

func (s *Server) handleResetCalibration(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	var req ResetCalibrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	lifecycleSvc := s.getCalibrationLifecycleService()
	if lifecycleSvc == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Calibration service not available")
		return
	}

	ctx := c.Request.Context()

	// Parse user ID using safe parser
	userIDInt, parseErr := parseUserIDInt(userID)
	if parseErr != nil {
		log.Printf("[CALIBRATION] Invalid user ID: %v", parseErr)
		errorResponse(c, http.StatusBadRequest, "Invalid user ID: "+parseErr.Error())
		return
	}

	var err error
	if req.IndicatorHash != "" {
		// Reset specific calibration
		err = lifecycleSvc.ResetCalibration(ctx, userIDInt, req.Strategy, req.IndicatorHash)
	} else {
		// Reset all calibrations for this strategy
		err = lifecycleSvc.ResetAllCalibrations(ctx, userIDInt, req.Strategy)
	}

	if err != nil {
		log.Printf("[CALIBRATION] Error resetting calibration for user %d: %v", userIDInt, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to reset calibration: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "Calibration reset successfully",
		"strategy":       req.Strategy,
		"indicator_hash": req.IndicatorHash,
	})
}

// ==================== GET /api/calibration/confidence/:strategy ====================
// Get calibration confidence level for a strategy

func (s *Server) handleGetCalibrationConfidence(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	strategy := c.Param("strategy")
	if strategy == "" {
		errorResponse(c, http.StatusBadRequest, "Strategy parameter required")
		return
	}

	// Optional indicator hash query param
	indicatorHash := c.Query("indicator_hash")
	if indicatorHash == "" {
		indicatorHash = "00000000" // Default hash
	}

	lifecycleSvc := s.getCalibrationLifecycleService()
	if lifecycleSvc == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Calibration service not available")
		return
	}

	ctx := c.Request.Context()

	// Parse user ID using safe parser
	userIDInt, parseErr := parseUserIDInt(userID)
	if parseErr != nil {
		log.Printf("[CALIBRATION] Invalid user ID: %v", parseErr)
		errorResponse(c, http.StatusBadRequest, "Invalid user ID: "+parseErr.Error())
		return
	}

	confidence, err := lifecycleSvc.GetCalibrationConfidence(ctx, userIDInt, strategy, indicatorHash)
	if err != nil {
		log.Printf("[CALIBRATION] Error getting confidence for user %d: %v", userIDInt, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get calibration confidence: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"strategy":       strategy,
		"indicator_hash": indicatorHash,
		"confidence":     confidence,
	})
}

// ==================== GET /api/calibration/history/:strategy ====================
// Get calibration history for a strategy

func (s *Server) handleGetCalibrationHistory(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	strategy := c.Param("strategy")
	if strategy == "" {
		errorResponse(c, http.StatusBadRequest, "Strategy parameter required")
		return
	}

	lifecycleSvc := s.getCalibrationLifecycleService()
	if lifecycleSvc == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Calibration service not available")
		return
	}

	ctx := c.Request.Context()

	// Parse user ID using safe parser
	userIDInt, parseErr := parseUserIDInt(userID)
	if parseErr != nil {
		log.Printf("[CALIBRATION] Invalid user ID: %v", parseErr)
		errorResponse(c, http.StatusBadRequest, "Invalid user ID: "+parseErr.Error())
		return
	}

	history, err := lifecycleSvc.GetCalibrationHistory(ctx, userIDInt, strategy)
	if err != nil {
		log.Printf("[CALIBRATION] Error getting history for user %d: %v", userIDInt, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get calibration history: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"strategy": strategy,
		"history":  history,
	})
}

// ==================== GET /api/calibration/data/:strategy ====================
// Get current calibration data for a strategy

func (s *Server) handleGetCalibrationData(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	strategy := c.Param("strategy")
	if strategy == "" {
		errorResponse(c, http.StatusBadRequest, "Strategy parameter required")
		return
	}

	// Optional indicator hash query param
	indicatorHash := c.Query("indicator_hash")
	if indicatorHash == "" {
		indicatorHash = "00000000"
	}

	lifecycleSvc := s.getCalibrationLifecycleService()
	if lifecycleSvc == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Calibration service not available")
		return
	}

	ctx := c.Request.Context()

	// Parse user ID using safe parser
	userIDInt, parseErr := parseUserIDInt(userID)
	if parseErr != nil {
		log.Printf("[CALIBRATION] Invalid user ID: %v", parseErr)
		errorResponse(c, http.StatusBadRequest, "Invalid user ID: "+parseErr.Error())
		return
	}

	// Get confidence (which includes bucket breakdown)
	confidence, err := lifecycleSvc.GetCalibrationConfidence(ctx, userIDInt, strategy, indicatorHash)
	if err != nil {
		log.Printf("[CALIBRATION] Error getting data for user %d: %v", userIDInt, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to get calibration data: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"strategy":       strategy,
		"indicator_hash": indicatorHash,
		"total_trades":   confidence.TotalTrades,
		"confidence":     confidence,
		"buckets":        confidence.BucketBreakdown,
	})
}
