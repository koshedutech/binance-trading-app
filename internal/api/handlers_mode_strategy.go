package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"binance-trading-bot/internal/cache"
	"binance-trading-bot/internal/database"

	"github.com/gin-gonic/gin"
)

// ==================== STORY 11.31: Mode-Strategy API Endpoints ====================
// API endpoints for Mode+Strategy configuration management
// Uses cache-first read pattern with write-through to DB
// ==================================================================================

// validModes lists the valid trading mode names
var validModes = map[string]bool{
	"ultra_fast": true,
	"scalp":      true,
	"swing":      true,
	"position":   true,
}

// validStrategies lists the valid strategy names
var validStrategies = map[string]bool{
	"trend_following": true,
	"mean_reversion":  true,
	"breakout":        true,
	"range_trading":   true,
}

// ==================== REQUEST/RESPONSE TYPES ====================

// ModeStrategyResponse is the API response format for a mode+strategy config
type ModeStrategyResponse struct {
	Mode             string                 `json:"mode"`
	Strategy         string                 `json:"strategy"`
	Enabled          bool                   `json:"enabled"`
	Priority         int                    `json:"priority"`
	SupportedRegimes []string               `json:"supported_regimes"`
	Settings         map[string]interface{} `json:"settings"`
}

// ModeWithStrategiesResponse is the response for a mode with all its strategies
type ModeWithStrategiesResponse struct {
	Mode       string                         `json:"mode"`
	Strategies map[string]*ModeStrategyResponse `json:"strategies"`
}

// AllModesResponse is the response for listing all modes with their strategies
type AllModesResponse struct {
	Modes map[string]*ModeWithStrategiesResponse `json:"modes"`
}

// UpdateModeStrategyRequest is the request body for updating a mode+strategy
type UpdateModeStrategyRequest struct {
	Enabled          *bool                  `json:"enabled,omitempty"`
	Priority         *int                   `json:"priority,omitempty"`
	SupportedRegimes []string               `json:"supported_regimes,omitempty"`
	Settings         map[string]interface{} `json:"settings,omitempty"`
}

// ==================== VALIDATION HELPERS ====================

// validateMode checks if a mode name is valid
func validateMode(mode string) bool {
	return validModes[mode]
}

// validateStrategy checks if a strategy name is valid
func validateStrategy(strategy string) bool {
	return validStrategies[strategy]
}

// parseUserIDToInt converts a string user ID to integer
// Handles UUID format for default admin user
func parseUserIDToInt(userID string) (int, error) {
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

// configToResponse converts a ModeStrategyConfig to API response format
func configToResponse(mode, strategy string, config *database.ModeStrategyConfig) *ModeStrategyResponse {
	// Build settings map from config fields
	settings := map[string]interface{}{
		"leverage":      config.Leverage,
		"max_positions": config.MaxPositions,
		"base_size_usd": config.BaseSizeUSD,
		"timeframe":     config.Timeframe,
		"sltp":          config.SLTP,
		"confidence":    config.Confidence,
		"exit_conditions": config.ExitConditions,
		"scoring":       config.Scoring,
	}

	// Add entry_conditions if present
	if config.EntryConditions != nil {
		settings["entry_conditions"] = config.EntryConditions
	}

	return &ModeStrategyResponse{
		Mode:             mode,
		Strategy:         strategy,
		Enabled:          config.Enabled,
		Priority:         config.Priority,
		SupportedRegimes: config.SupportedRegimes,
		Settings:         settings,
	}
}

// ==================== GET /api/modes ====================
// List all modes with their strategies

func (s *Server) handleGetAllModes(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	ctx := c.Request.Context()

	// Check if settings cache service is available
	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	response := &AllModesResponse{
		Modes: make(map[string]*ModeWithStrategiesResponse),
	}

	// Get all strategies for each mode
	for mode := range validModes {
		strategies, err := s.settingsCacheService.GetAllStrategiesForMode(ctx, userID, mode)
		if err != nil {
			if err == cache.ErrCacheUnavailable {
				errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
				return
			}
			// Log error but continue with other modes
			continue
		}

		modeResponse := &ModeWithStrategiesResponse{
			Mode:       mode,
			Strategies: make(map[string]*ModeStrategyResponse),
		}

		for strategyName, config := range strategies {
			modeResponse.Strategies[strategyName] = configToResponse(mode, strategyName, config)
		}

		response.Modes[mode] = modeResponse
	}

	c.JSON(http.StatusOK, response)
}

// ==================== GET /api/modes/:mode ====================
// Get mode with all strategy configs

func (s *Server) handleGetMode(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: ultra_fast, scalp, swing, position", mode))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	strategies, err := s.settingsCacheService.GetAllStrategiesForMode(ctx, userID, mode)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get mode strategies: "+err.Error())
		return
	}

	response := &ModeWithStrategiesResponse{
		Mode:       mode,
		Strategies: make(map[string]*ModeStrategyResponse),
	}

	for strategyName, config := range strategies {
		response.Strategies[strategyName] = configToResponse(mode, strategyName, config)
	}

	c.JSON(http.StatusOK, response)
}

// ==================== GET /api/modes/:mode/strategies ====================
// List strategies for a mode

func (s *Server) handleGetModeStrategies(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: ultra_fast, scalp, swing, position", mode))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	strategies, err := s.settingsCacheService.GetAllStrategiesForMode(ctx, userID, mode)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get mode strategies: "+err.Error())
		return
	}

	// Convert to list format
	strategyList := make([]*ModeStrategyResponse, 0, len(strategies))
	for strategyName, config := range strategies {
		strategyList = append(strategyList, configToResponse(mode, strategyName, config))
	}

	c.JSON(http.StatusOK, gin.H{
		"mode":       mode,
		"strategies": strategyList,
	})
}

// ==================== GET /api/modes/:mode/strategies/:strategy ====================
// Get specific mode+strategy config

func (s *Server) handleGetModeStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	strategy := c.Param("strategy")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: ultra_fast, scalp, swing, position", mode))
		return
	}

	if !validateStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy: %s. Valid strategies: trend_following, mean_reversion, breakout, range_trading", strategy))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	config, err := s.settingsCacheService.GetModeStrategyConfig(ctx, userID, mode, strategy)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get mode strategy config: "+err.Error())
		return
	}

	response := configToResponse(mode, strategy, config)
	c.JSON(http.StatusOK, response)
}

// ==================== PUT /api/modes/:mode/strategies/:strategy ====================
// Update mode+strategy config

func (s *Server) handleUpdateModeStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	strategy := c.Param("strategy")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: ultra_fast, scalp, swing, position", mode))
		return
	}

	if !validateStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy: %s. Valid strategies: trend_following, mean_reversion, breakout, range_trading", strategy))
		return
	}

	var req UpdateModeStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	// Get existing config
	existingConfig, err := s.settingsCacheService.GetModeStrategyConfig(ctx, userID, mode, strategy)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get existing config: "+err.Error())
		return
	}

	// Apply updates
	if req.Enabled != nil {
		existingConfig.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		existingConfig.Priority = *req.Priority
	}
	if req.SupportedRegimes != nil {
		existingConfig.SupportedRegimes = req.SupportedRegimes
	}

	// Apply settings updates
	if req.Settings != nil {
		if leverage, ok := req.Settings["leverage"].(float64); ok {
			existingConfig.Leverage = int(leverage)
		}
		if maxPositions, ok := req.Settings["max_positions"].(float64); ok {
			existingConfig.MaxPositions = int(maxPositions)
		}
		if baseSizeUSD, ok := req.Settings["base_size_usd"].(float64); ok {
			existingConfig.BaseSizeUSD = baseSizeUSD
		}

		// Handle nested settings via JSON re-marshaling for type safety
		// Errors are logged but don't fail the request - partial updates are allowed
		if timeframe, ok := req.Settings["timeframe"]; ok {
			tfBytes, err := json.Marshal(timeframe)
			if err == nil {
				if err := json.Unmarshal(tfBytes, &existingConfig.Timeframe); err != nil {
					c.Error(fmt.Errorf("invalid timeframe format: %w", err))
				}
			}
		}
		if sltp, ok := req.Settings["sltp"]; ok {
			sltpBytes, err := json.Marshal(sltp)
			if err == nil {
				if err := json.Unmarshal(sltpBytes, &existingConfig.SLTP); err != nil {
					c.Error(fmt.Errorf("invalid sltp format: %w", err))
				}
			}
		}
		if confidence, ok := req.Settings["confidence"]; ok {
			confBytes, err := json.Marshal(confidence)
			if err == nil {
				if err := json.Unmarshal(confBytes, &existingConfig.Confidence); err != nil {
					c.Error(fmt.Errorf("invalid confidence format: %w", err))
				}
			}
		}
		if exitConditions, ok := req.Settings["exit_conditions"]; ok {
			exitBytes, err := json.Marshal(exitConditions)
			if err == nil {
				if err := json.Unmarshal(exitBytes, &existingConfig.ExitConditions); err != nil {
					c.Error(fmt.Errorf("invalid exit_conditions format: %w", err))
				}
			}
		}
		if scoring, ok := req.Settings["scoring"]; ok {
			scoringBytes, err := json.Marshal(scoring)
			if err == nil {
				if err := json.Unmarshal(scoringBytes, &existingConfig.Scoring); err != nil {
					c.Error(fmt.Errorf("invalid scoring format: %w", err))
				}
			}
		}
		if entryConditions, ok := req.Settings["entry_conditions"].(map[string]interface{}); ok {
			existingConfig.EntryConditions = entryConditions
		}
	}

	// Save with write-through (DB first, then cache)
	if err := s.settingsCacheService.SetModeStrategyConfig(ctx, userID, mode, strategy, existingConfig); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save config: "+err.Error())
		return
	}

	response := configToResponse(mode, strategy, existingConfig)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Mode strategy config updated successfully",
		"data":    response,
	})
}

// ==================== POST /api/modes/:mode/strategies/:strategy/reset ====================
// Reset mode+strategy to defaults

func (s *Server) handleResetModeStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	strategy := c.Param("strategy")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: ultra_fast, scalp, swing, position", mode))
		return
	}

	if !validateStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy: %s. Valid strategies: trend_following, mean_reversion, breakout, range_trading", strategy))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	// Get default config
	defaultConfig := database.DefaultModeStrategyConfig(mode, strategy)

	// Save default config with write-through
	if err := s.settingsCacheService.SetModeStrategyConfig(ctx, userID, mode, strategy, defaultConfig); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to reset config: "+err.Error())
		return
	}

	response := configToResponse(mode, strategy, defaultConfig)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Mode strategy %s/%s reset to defaults", mode, strategy),
		"data":    response,
	})
}

// ==================== POST /api/modes/:mode/reset-all ====================
// Reset all strategies in mode to defaults

func (s *Server) handleResetAllModeStrategies(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: ultra_fast, scalp, swing, position", mode))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	// Reset all strategies in the mode
	resetStrategies := make(map[string]*ModeStrategyResponse)
	var errors []string

	for strategy := range validStrategies {
		defaultConfig := database.DefaultModeStrategyConfig(mode, strategy)
		if err := s.settingsCacheService.SetModeStrategyConfig(ctx, userID, mode, strategy, defaultConfig); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", strategy, err))
			continue
		}
		resetStrategies[strategy] = configToResponse(mode, strategy, defaultConfig)
	}

	if len(errors) > 0 {
		c.JSON(http.StatusPartialContent, gin.H{
			"success":    false,
			"message":    "Some strategies failed to reset",
			"errors":     errors,
			"strategies": resetStrategies,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    fmt.Sprintf("All strategies in mode %s reset to defaults", mode),
		"mode":       mode,
		"strategies": resetStrategies,
	})
}

// ==================== POST /api/modes/:mode/strategies/:strategy/enable ====================
// Enable a specific strategy

func (s *Server) handleEnableModeStrategy(c *gin.Context) {
	s.handleToggleModeStrategy(c, true)
}

// ==================== POST /api/modes/:mode/strategies/:strategy/disable ====================
// Disable a specific strategy

func (s *Server) handleDisableModeStrategy(c *gin.Context) {
	s.handleToggleModeStrategy(c, false)
}

// handleToggleModeStrategy is the shared implementation for enable/disable
func (s *Server) handleToggleModeStrategy(c *gin.Context, enabled bool) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	strategy := c.Param("strategy")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: ultra_fast, scalp, swing, position", mode))
		return
	}

	if !validateStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy: %s. Valid strategies: trend_following, mean_reversion, breakout, range_trading", strategy))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	// Get existing config
	existingConfig, err := s.settingsCacheService.GetModeStrategyConfig(ctx, userID, mode, strategy)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get existing config: "+err.Error())
		return
	}

	// Update enabled status
	existingConfig.Enabled = enabled

	// Save with write-through (DB first, then cache)
	if err := s.settingsCacheService.SetModeStrategyConfig(ctx, userID, mode, strategy, existingConfig); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save config: "+err.Error())
		return
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Strategy %s/%s %s", mode, strategy, action),
	})
}

// ==================== GET /api/modes/:mode/strategies/:strategy/compare ====================
// Compare current mode+strategy config with defaults

// StrategyFieldComparison represents a single field comparison
type StrategyFieldComparison struct {
	Path    string      `json:"path"`
	Current interface{} `json:"current"`
	Default interface{} `json:"default"`
	Match   bool        `json:"match"`
}

// StrategyComparisonResponse is the API response for strategy comparison
type StrategyComparisonResponse struct {
	Success        bool                      `json:"success"`
	Mode           string                    `json:"mode"`
	Strategy       string                    `json:"strategy"`
	Enabled        bool                      `json:"enabled"`
	AllMatch       bool                      `json:"all_match"`
	TotalFields    int                       `json:"total_fields"`
	MatchingFields int                       `json:"matching_fields"`
	Differences    []StrategyFieldComparison `json:"differences"`
}

func (s *Server) handleCompareModeStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	strategy := c.Param("strategy")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: ultra_fast, scalp, swing, position", mode))
		return
	}

	if !validateStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy: %s. Valid strategies: trend_following, mean_reversion, breakout, range_trading", strategy))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	// Get current user config
	currentConfig, err := s.settingsCacheService.GetModeStrategyConfig(ctx, userID, mode, strategy)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get current config: "+err.Error())
		return
	}

	// Get default config
	defaultConfig := database.DefaultModeStrategyConfig(mode, strategy)

	// Compare configs and build differences
	differences := []StrategyFieldComparison{}
	totalFields := 0
	matchingFields := 0

	// Helper to compare and add field
	compareField := func(path string, current, defaultVal interface{}) {
		totalFields++
		match := compareValues(current, defaultVal)
		if match {
			matchingFields++
		} else {
			differences = append(differences, StrategyFieldComparison{
				Path:    path,
				Current: current,
				Default: defaultVal,
				Match:   false,
			})
		}
	}

	// Compare top-level fields
	compareField("enabled", currentConfig.Enabled, defaultConfig.Enabled)
	compareField("priority", currentConfig.Priority, defaultConfig.Priority)
	compareField("leverage", currentConfig.Leverage, defaultConfig.Leverage)
	compareField("max_positions", currentConfig.MaxPositions, defaultConfig.MaxPositions)
	compareField("base_size_usd", currentConfig.BaseSizeUSD, defaultConfig.BaseSizeUSD)

	// Compare timeframe fields
	compareField("timeframe.trend_timeframe", currentConfig.Timeframe.TrendTimeframe, defaultConfig.Timeframe.TrendTimeframe)
	compareField("timeframe.entry_timeframe", currentConfig.Timeframe.EntryTimeframe, defaultConfig.Timeframe.EntryTimeframe)
	compareField("timeframe.analysis_timeframe", currentConfig.Timeframe.AnalysisTimeframe, defaultConfig.Timeframe.AnalysisTimeframe)

	// Compare SLTP fields
	compareField("sltp.sl_percent", currentConfig.SLTP.SLPercent, defaultConfig.SLTP.SLPercent)
	compareField("sltp.tp1_percent", currentConfig.SLTP.TP1Percent, defaultConfig.SLTP.TP1Percent)
	compareField("sltp.tp2_percent", currentConfig.SLTP.TP2Percent, defaultConfig.SLTP.TP2Percent)
	compareField("sltp.tp3_percent", currentConfig.SLTP.TP3Percent, defaultConfig.SLTP.TP3Percent)
	compareField("sltp.trailing_enabled", currentConfig.SLTP.TrailingEnabled, defaultConfig.SLTP.TrailingEnabled)
	compareField("sltp.trailing_activation_pct", currentConfig.SLTP.TrailingActivationPct, defaultConfig.SLTP.TrailingActivationPct)
	compareField("sltp.trailing_stop_pct", currentConfig.SLTP.TrailingStopPct, defaultConfig.SLTP.TrailingStopPct)

	// Compare confidence fields
	compareField("confidence.min_confidence", currentConfig.Confidence.MinConfidence, defaultConfig.Confidence.MinConfidence)
	compareField("confidence.high_confidence", currentConfig.Confidence.HighConfidence, defaultConfig.Confidence.HighConfidence)
	compareField("confidence.ultra_confidence", currentConfig.Confidence.UltraConfidence, defaultConfig.Confidence.UltraConfidence)

	// Compare exit conditions
	compareField("exit_conditions.max_hold_minutes", currentConfig.ExitConditions.MaxHoldMinutes, defaultConfig.ExitConditions.MaxHoldMinutes)
	compareField("exit_conditions.early_warning_enabled", currentConfig.ExitConditions.EarlyWarningEnabled, defaultConfig.ExitConditions.EarlyWarningEnabled)

	// Compare scoring fields
	compareField("scoring.technical_weight", currentConfig.Scoring.TechnicalWeight, defaultConfig.Scoring.TechnicalWeight)
	compareField("scoring.momentum_weight", currentConfig.Scoring.MomentumWeight, defaultConfig.Scoring.MomentumWeight)
	compareField("scoring.volume_weight", currentConfig.Scoring.VolumeWeight, defaultConfig.Scoring.VolumeWeight)
	compareField("scoring.sentiment_weight", currentConfig.Scoring.SentimentWeight, defaultConfig.Scoring.SentimentWeight)

	// Compare entry conditions (compare as JSON since it's a map)
	if currentConfig.EntryConditions != nil || defaultConfig.EntryConditions != nil {
		currentJSON, _ := json.Marshal(currentConfig.EntryConditions)
		defaultJSON, _ := json.Marshal(defaultConfig.EntryConditions)
		if string(currentJSON) != string(defaultJSON) {
			// Add individual entry condition differences
			for key, defaultVal := range defaultConfig.EntryConditions {
				currentVal := currentConfig.EntryConditions[key]
				compareField("entry_conditions."+key, currentVal, defaultVal)
			}
		} else {
			// Count entry condition fields as matching
			for range defaultConfig.EntryConditions {
				totalFields++
				matchingFields++
			}
		}
	}

	response := StrategyComparisonResponse{
		Success:        true,
		Mode:           mode,
		Strategy:       strategy,
		Enabled:        currentConfig.Enabled,
		AllMatch:       len(differences) == 0,
		TotalFields:    totalFields,
		MatchingFields: matchingFields,
		Differences:    differences,
	}

	c.JSON(http.StatusOK, response)
}

// compareValues compares two values for equality
func compareValues(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert to JSON and compare for deep equality
	aJSON, errA := json.Marshal(a)
	bJSON, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}

	return string(aJSON) == string(bJSON)
}
