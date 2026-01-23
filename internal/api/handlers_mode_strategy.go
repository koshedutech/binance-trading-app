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

// validSectionNames lists all valid section names for mode-strategy config
var validSectionNames = map[string]bool{
	"position_sizing":       true,
	"timeframe":             true,
	"mtf":                   true,
	"sltp":                  true,
	"confidence":            true,
	"entry_conditions":      true,
	"exit_conditions":       true,
	"scoring":               true,
	"circuit_breaker":       true,
	"hedge":                 true,
	"averaging":             true,
	"stale_release":         true,
	"position_optimization": true,
	"funding_rate":          true,
	"risk":                  true,
	"trend_divergence":      true,
	"dynamic_ai_exit":       true,
	"early_warning":         true,
}

// validateSection checks if a section name is valid
func validateSection(section string) bool {
	return validSectionNames[section]
}

// configToResponse converts a ModeStrategyConfig to API response format
// Story 11.41: Expanded to include all 18 sections
func configToResponse(mode, strategy string, config *database.ModeStrategyConfig) *ModeStrategyResponse {
	// Build settings map from config fields - include ALL 18 sections
	settings := map[string]interface{}{
		// 1. Position sizing (legacy fields for backward compatibility)
		"leverage":      config.Leverage,
		"max_positions": config.MaxPositions,
		"base_size_usd": config.BaseSizeUSD,
		// 2. Timeframe
		"timeframe": config.Timeframe,
		// 4. SLTP
		"sltp": config.SLTP,
		// 5. Confidence
		"confidence": config.Confidence,
		// 7. Exit conditions
		"exit_conditions": config.ExitConditions,
		// 8. Scoring
		"scoring": config.Scoring,
	}

	// 1. Position sizing (new structured format)
	if config.PositionSizing != nil {
		settings["position_sizing"] = config.PositionSizing
	} else {
		// Build from legacy fields
		settings["position_sizing"] = &database.StrategyPositionSizing{
			Leverage:     config.Leverage,
			MaxPositions: config.MaxPositions,
			BaseSizeUSD:  config.BaseSizeUSD,
		}
	}

	// 3. MTF
	if config.MTF != nil {
		settings["mtf"] = config.MTF
	}

	// 6. Entry conditions (prefer v2 if available)
	if config.EntryConditionsV2 != nil {
		settings["entry_conditions"] = config.EntryConditionsV2
	} else if config.EntryConditions != nil {
		settings["entry_conditions"] = config.EntryConditions
	}

	// 9. Circuit breaker
	if config.CircuitBreaker != nil {
		settings["circuit_breaker"] = config.CircuitBreaker
	}

	// 10. Hedge
	if config.Hedge != nil {
		settings["hedge"] = config.Hedge
	}

	// 11. Averaging
	if config.Averaging != nil {
		settings["averaging"] = config.Averaging
	}

	// 12. Stale release
	if config.StaleRelease != nil {
		settings["stale_release"] = config.StaleRelease
	}

	// 13. Position optimization
	if config.PositionOptimization != nil {
		settings["position_optimization"] = config.PositionOptimization
	}

	// 14. Funding rate
	if config.FundingRate != nil {
		settings["funding_rate"] = config.FundingRate
	}

	// 15. Risk
	if config.Risk != nil {
		settings["risk"] = config.Risk
	}

	// 16. Trend divergence
	if config.TrendDivergence != nil {
		settings["trend_divergence"] = config.TrendDivergence
	}

	// 17. Dynamic AI exit
	if config.DynamicAIExit != nil {
		settings["dynamic_ai_exit"] = config.DynamicAIExit
	}

	// 18. Early warning
	if config.EarlyWarning != nil {
		settings["early_warning"] = config.EarlyWarning
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

	// Apply settings updates - Story 11.41: Support all 18 sections
	var parseErrors []string
	if req.Settings != nil {
		// Legacy position sizing fields (for backward compatibility) with validation
		if leverage, ok := req.Settings["leverage"].(float64); ok {
			if leverage < 1 || leverage > 125 {
				parseErrors = append(parseErrors, "leverage: must be between 1 and 125")
			} else {
				existingConfig.Leverage = int(leverage)
			}
		}
		if maxPositions, ok := req.Settings["max_positions"].(float64); ok {
			if maxPositions < 1 || maxPositions > 100 {
				parseErrors = append(parseErrors, "max_positions: must be between 1 and 100")
			} else {
				existingConfig.MaxPositions = int(maxPositions)
			}
		}
		if baseSizeUSD, ok := req.Settings["base_size_usd"].(float64); ok {
			if baseSizeUSD < 1 || baseSizeUSD > 1000000 {
				parseErrors = append(parseErrors, "base_size_usd: must be between 1 and 1000000")
			} else {
				existingConfig.BaseSizeUSD = baseSizeUSD
			}
		}

		// Helper to unmarshal section data with error tracking
		unmarshalSection := func(key string, target interface{}) {
			if data, ok := req.Settings[key]; ok {
				bytes, err := json.Marshal(data)
				if err != nil {
					parseErrors = append(parseErrors, fmt.Sprintf("%s: marshal error: %v", key, err))
					return
				}
				if err := json.Unmarshal(bytes, target); err != nil {
					parseErrors = append(parseErrors, fmt.Sprintf("%s: invalid format: %v", key, err))
				}
			}
		}

		// 1. Position sizing (new structured format) with validation
		if ps, ok := req.Settings["position_sizing"]; ok {
			var positionSizing database.StrategyPositionSizing
			bytes, _ := json.Marshal(ps)
			if err := json.Unmarshal(bytes, &positionSizing); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("position_sizing: invalid format: %v", err))
			} else {
				// Validate position sizing fields
				psValid := true
				if positionSizing.Leverage < 1 || positionSizing.Leverage > 125 {
					parseErrors = append(parseErrors, "position_sizing.leverage: must be between 1 and 125")
					psValid = false
				}
				if positionSizing.MaxPositions < 1 || positionSizing.MaxPositions > 100 {
					parseErrors = append(parseErrors, "position_sizing.max_positions: must be between 1 and 100")
					psValid = false
				}
				if positionSizing.BaseSizeUSD < 1 || positionSizing.BaseSizeUSD > 1000000 {
					parseErrors = append(parseErrors, "position_sizing.base_size_usd: must be between 1 and 1000000")
					psValid = false
				}
				if psValid {
					existingConfig.PositionSizing = &positionSizing
					// Also update legacy fields for backward compatibility
					existingConfig.Leverage = positionSizing.Leverage
					existingConfig.MaxPositions = positionSizing.MaxPositions
					existingConfig.BaseSizeUSD = positionSizing.BaseSizeUSD
				}
			}
		}

		// 2. Timeframe
		unmarshalSection("timeframe", &existingConfig.Timeframe)

		// 3. MTF
		if mtf, ok := req.Settings["mtf"]; ok {
			var mtfConfig database.StrategyMTF
			bytes, _ := json.Marshal(mtf)
			if err := json.Unmarshal(bytes, &mtfConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("mtf: invalid format: %v", err))
			} else {
				existingConfig.MTF = &mtfConfig
			}
		}

		// 4. SLTP
		unmarshalSection("sltp", &existingConfig.SLTP)

		// 5. Confidence
		unmarshalSection("confidence", &existingConfig.Confidence)

		// 6. Entry conditions (supports both legacy map and v2 struct)
		if ec, ok := req.Settings["entry_conditions"]; ok {
			// Try v2 struct first
			var ecV2 database.StrategyEntryConditions
			bytes, _ := json.Marshal(ec)
			if err := json.Unmarshal(bytes, &ecV2); err == nil {
				existingConfig.EntryConditionsV2 = &ecV2
			} else if ecMap, ok := ec.(map[string]interface{}); ok {
				// Fall back to legacy map
				existingConfig.EntryConditions = ecMap
			}
		}

		// 7. Exit conditions
		unmarshalSection("exit_conditions", &existingConfig.ExitConditions)

		// 8. Scoring
		unmarshalSection("scoring", &existingConfig.Scoring)

		// 9. Circuit breaker
		if cb, ok := req.Settings["circuit_breaker"]; ok {
			var cbConfig database.StrategyCircuitBreaker
			bytes, _ := json.Marshal(cb)
			if err := json.Unmarshal(bytes, &cbConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("circuit_breaker: invalid format: %v", err))
			} else {
				existingConfig.CircuitBreaker = &cbConfig
			}
		}

		// 10. Hedge
		if hedge, ok := req.Settings["hedge"]; ok {
			var hedgeConfig database.StrategyHedge
			bytes, _ := json.Marshal(hedge)
			if err := json.Unmarshal(bytes, &hedgeConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("hedge: invalid format: %v", err))
			} else {
				existingConfig.Hedge = &hedgeConfig
			}
		}

		// 11. Averaging
		if avg, ok := req.Settings["averaging"]; ok {
			var avgConfig database.StrategyAveraging
			bytes, _ := json.Marshal(avg)
			if err := json.Unmarshal(bytes, &avgConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("averaging: invalid format: %v", err))
			} else {
				existingConfig.Averaging = &avgConfig
			}
		}

		// 12. Stale release
		if sr, ok := req.Settings["stale_release"]; ok {
			var srConfig database.StrategyStaleRelease
			bytes, _ := json.Marshal(sr)
			if err := json.Unmarshal(bytes, &srConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("stale_release: invalid format: %v", err))
			} else {
				existingConfig.StaleRelease = &srConfig
			}
		}

		// 13. Position optimization
		if po, ok := req.Settings["position_optimization"]; ok {
			var poConfig database.StrategyPositionOptimization
			bytes, _ := json.Marshal(po)
			if err := json.Unmarshal(bytes, &poConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("position_optimization: invalid format: %v", err))
			} else {
				existingConfig.PositionOptimization = &poConfig
			}
		}

		// 14. Funding rate
		if fr, ok := req.Settings["funding_rate"]; ok {
			var frConfig database.StrategyFundingRate
			bytes, _ := json.Marshal(fr)
			if err := json.Unmarshal(bytes, &frConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("funding_rate: invalid format: %v", err))
			} else {
				existingConfig.FundingRate = &frConfig
			}
		}

		// 15. Risk
		if risk, ok := req.Settings["risk"]; ok {
			var riskConfig database.StrategyRisk
			bytes, _ := json.Marshal(risk)
			if err := json.Unmarshal(bytes, &riskConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("risk: invalid format: %v", err))
			} else {
				existingConfig.Risk = &riskConfig
			}
		}

		// 16. Trend divergence
		if td, ok := req.Settings["trend_divergence"]; ok {
			var tdConfig database.StrategyTrendDivergence
			bytes, _ := json.Marshal(td)
			if err := json.Unmarshal(bytes, &tdConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("trend_divergence: invalid format: %v", err))
			} else {
				existingConfig.TrendDivergence = &tdConfig
			}
		}

		// 17. Dynamic AI exit
		if dae, ok := req.Settings["dynamic_ai_exit"]; ok {
			var daeConfig database.StrategyDynamicAIExit
			bytes, _ := json.Marshal(dae)
			if err := json.Unmarshal(bytes, &daeConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("dynamic_ai_exit: invalid format: %v", err))
			} else {
				existingConfig.DynamicAIExit = &daeConfig
			}
		}

		// 18. Early warning
		if ew, ok := req.Settings["early_warning"]; ok {
			var ewConfig database.StrategyEarlyWarning
			bytes, _ := json.Marshal(ew)
			if err := json.Unmarshal(bytes, &ewConfig); err != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("early_warning: invalid format: %v", err))
			} else {
				existingConfig.EarlyWarning = &ewConfig
			}
		}
	}

	// Check for parsing errors before saving
	if len(parseErrors) > 0 {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid settings format: %v", parseErrors))
		return
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
	AllValues      []StrategyFieldComparison `json:"all_values"` // All fields including matches
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

	// Compare configs and build differences and all values
	differences := []StrategyFieldComparison{}
	allValues := []StrategyFieldComparison{}
	totalFields := 0
	matchingFields := 0

	// Helper to compare and add field (adds to both allValues and differences if not matching)
	compareField := func(path string, current, defaultVal interface{}) {
		totalFields++
		match := compareValues(current, defaultVal)
		field := StrategyFieldComparison{
			Path:    path,
			Current: current,
			Default: defaultVal,
			Match:   match,
		}
		allValues = append(allValues, field)
		if match {
			matchingFields++
		} else {
			differences = append(differences, field)
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

	// Compare entry conditions (compare each field individually)
	if defaultConfig.EntryConditions != nil {
		for key, defaultVal := range defaultConfig.EntryConditions {
			currentVal := interface{}(nil)
			if currentConfig.EntryConditions != nil {
				currentVal = currentConfig.EntryConditions[key]
			}
			compareField("entry_conditions."+key, currentVal, defaultVal)
		}
	}

	// Story 11.41/11.42: Compare the 12 additional sections with individual fields
	// MTF section
	if currentConfig.MTF != nil && defaultConfig.MTF != nil {
		compareField("mtf.enabled", currentConfig.MTF.Enabled, defaultConfig.MTF.Enabled)
		compareField("mtf.primary_timeframe", currentConfig.MTF.PrimaryTimeframe, defaultConfig.MTF.PrimaryTimeframe)
		compareField("mtf.primary_weight", currentConfig.MTF.PrimaryWeight, defaultConfig.MTF.PrimaryWeight)
		compareField("mtf.secondary_timeframe", currentConfig.MTF.SecondaryTimeframe, defaultConfig.MTF.SecondaryTimeframe)
		compareField("mtf.secondary_weight", currentConfig.MTF.SecondaryWeight, defaultConfig.MTF.SecondaryWeight)
		compareField("mtf.tertiary_timeframe", currentConfig.MTF.TertiaryTimeframe, defaultConfig.MTF.TertiaryTimeframe)
		compareField("mtf.tertiary_weight", currentConfig.MTF.TertiaryWeight, defaultConfig.MTF.TertiaryWeight)
		compareField("mtf.min_consensus", currentConfig.MTF.MinConsensus, defaultConfig.MTF.MinConsensus)
		compareField("mtf.min_weighted_strength", currentConfig.MTF.MinWeightedStrength, defaultConfig.MTF.MinWeightedStrength)
		compareField("mtf.trend_stability_check", currentConfig.MTF.TrendStabilityCheck, defaultConfig.MTF.TrendStabilityCheck)
	}

	// Circuit breaker section
	if currentConfig.CircuitBreaker != nil && defaultConfig.CircuitBreaker != nil {
		compareField("circuit_breaker.max_loss_per_hour_usd", currentConfig.CircuitBreaker.MaxLossPerHourUSD, defaultConfig.CircuitBreaker.MaxLossPerHourUSD)
		compareField("circuit_breaker.max_loss_per_day_usd", currentConfig.CircuitBreaker.MaxLossPerDayUSD, defaultConfig.CircuitBreaker.MaxLossPerDayUSD)
		compareField("circuit_breaker.max_consecutive_losses", currentConfig.CircuitBreaker.MaxConsecutiveLosses, defaultConfig.CircuitBreaker.MaxConsecutiveLosses)
		compareField("circuit_breaker.cooldown_minutes", currentConfig.CircuitBreaker.CooldownMinutes, defaultConfig.CircuitBreaker.CooldownMinutes)
		compareField("circuit_breaker.max_trades_per_hour", currentConfig.CircuitBreaker.MaxTradesPerHour, defaultConfig.CircuitBreaker.MaxTradesPerHour)
		compareField("circuit_breaker.max_trades_per_day", currentConfig.CircuitBreaker.MaxTradesPerDay, defaultConfig.CircuitBreaker.MaxTradesPerDay)
		compareField("circuit_breaker.win_rate_check_after", currentConfig.CircuitBreaker.WinRateCheckAfter, defaultConfig.CircuitBreaker.WinRateCheckAfter)
		compareField("circuit_breaker.min_win_rate_pct", currentConfig.CircuitBreaker.MinWinRatePct, defaultConfig.CircuitBreaker.MinWinRatePct)
	}

	// Hedge section
	if currentConfig.Hedge != nil && defaultConfig.Hedge != nil {
		compareField("hedge.allow_hedge", currentConfig.Hedge.AllowHedge, defaultConfig.Hedge.AllowHedge)
		compareField("hedge.min_confidence_for_hedge", currentConfig.Hedge.MinConfidenceForHedge, defaultConfig.Hedge.MinConfidenceForHedge)
		compareField("hedge.existing_must_be_in_profit_pct", currentConfig.Hedge.ExistingMustBeInProfitPct, defaultConfig.Hedge.ExistingMustBeInProfitPct)
		compareField("hedge.max_hedge_size_percent", currentConfig.Hedge.MaxHedgeSizePercent, defaultConfig.Hedge.MaxHedgeSizePercent)
		compareField("hedge.allow_same_mode_hedge", currentConfig.Hedge.AllowSameModeHedge, defaultConfig.Hedge.AllowSameModeHedge)
		compareField("hedge.max_total_exposure_multiplier", currentConfig.Hedge.MaxTotalExposureMultiplier, defaultConfig.Hedge.MaxTotalExposureMultiplier)
	}

	// Averaging section
	if currentConfig.Averaging != nil && defaultConfig.Averaging != nil {
		compareField("averaging.allow_averaging", currentConfig.Averaging.AllowAveraging, defaultConfig.Averaging.AllowAveraging)
		compareField("averaging.average_up_profit_percent", currentConfig.Averaging.AverageUpProfitPercent, defaultConfig.Averaging.AverageUpProfitPercent)
		compareField("averaging.average_down_loss_percent", currentConfig.Averaging.AverageDownLossPercent, defaultConfig.Averaging.AverageDownLossPercent)
		compareField("averaging.add_size_percent", currentConfig.Averaging.AddSizePercent, defaultConfig.Averaging.AddSizePercent)
		compareField("averaging.max_averages", currentConfig.Averaging.MaxAverages, defaultConfig.Averaging.MaxAverages)
		compareField("averaging.min_confidence_for_average", currentConfig.Averaging.MinConfidenceForAverage, defaultConfig.Averaging.MinConfidenceForAverage)
		compareField("averaging.use_llm_for_averaging", currentConfig.Averaging.UseLLMForAveraging, defaultConfig.Averaging.UseLLMForAveraging)
		compareField("averaging.staged_entry_enabled", currentConfig.Averaging.StagedEntryEnabled, defaultConfig.Averaging.StagedEntryEnabled)
		compareField("averaging.staged_entry_levels", currentConfig.Averaging.StagedEntryLevels, defaultConfig.Averaging.StagedEntryLevels)
	}

	// Stale release section
	if currentConfig.StaleRelease != nil && defaultConfig.StaleRelease != nil {
		compareField("stale_release.enabled", currentConfig.StaleRelease.Enabled, defaultConfig.StaleRelease.Enabled)
		compareField("stale_release.max_hold_duration_minutes", currentConfig.StaleRelease.MaxHoldDurationMinutes, defaultConfig.StaleRelease.MaxHoldDurationMinutes)
		compareField("stale_release.min_profit_to_keep_pct", currentConfig.StaleRelease.MinProfitToKeepPct, defaultConfig.StaleRelease.MinProfitToKeepPct)
		compareField("stale_release.max_loss_to_force_close_pct", currentConfig.StaleRelease.MaxLossToForceClosePct, defaultConfig.StaleRelease.MaxLossToForceClosePct)
		compareField("stale_release.stale_zone_lo_pct", currentConfig.StaleRelease.StaleZoneLoPct, defaultConfig.StaleRelease.StaleZoneLoPct)
		compareField("stale_release.stale_zone_hi_pct", currentConfig.StaleRelease.StaleZoneHiPct, defaultConfig.StaleRelease.StaleZoneHiPct)
		compareField("stale_release.stale_zone_action", currentConfig.StaleRelease.StaleZoneAction, defaultConfig.StaleRelease.StaleZoneAction)
	}

	// Position optimization section
	if currentConfig.PositionOptimization != nil && defaultConfig.PositionOptimization != nil {
		compareField("position_optimization.reentry_enabled", currentConfig.PositionOptimization.ReentryEnabled, defaultConfig.PositionOptimization.ReentryEnabled)
		compareField("position_optimization.reentry_after_tp1", currentConfig.PositionOptimization.ReentryAfterTP1, defaultConfig.PositionOptimization.ReentryAfterTP1)
		compareField("position_optimization.reentry_min_pullback_pct", currentConfig.PositionOptimization.ReentryMinPullbackPct, defaultConfig.PositionOptimization.ReentryMinPullbackPct)
		compareField("position_optimization.max_reentries_per_position", currentConfig.PositionOptimization.MaxReentriesPerPosition, defaultConfig.PositionOptimization.MaxReentriesPerPosition)
		compareField("position_optimization.dynamic_sl_enabled", currentConfig.PositionOptimization.DynamicSLEnabled, defaultConfig.PositionOptimization.DynamicSLEnabled)
		compareField("position_optimization.dynamic_sl_at_breakeven_pct", currentConfig.PositionOptimization.DynamicSLAtBreakevenPct, defaultConfig.PositionOptimization.DynamicSLAtBreakevenPct)
		compareField("position_optimization.profit_protection_enabled", currentConfig.PositionOptimization.ProfitProtectionEnabled, defaultConfig.PositionOptimization.ProfitProtectionEnabled)
		compareField("position_optimization.profit_protection_trigger_pct", currentConfig.PositionOptimization.ProfitProtectionTriggerPct, defaultConfig.PositionOptimization.ProfitProtectionTriggerPct)
		compareField("position_optimization.profit_protection_lock_pct", currentConfig.PositionOptimization.ProfitProtectionLockPct, defaultConfig.PositionOptimization.ProfitProtectionLockPct)
	}

	// Funding rate section
	if currentConfig.FundingRate != nil && defaultConfig.FundingRate != nil {
		compareField("funding_rate.enabled", currentConfig.FundingRate.Enabled, defaultConfig.FundingRate.Enabled)
		compareField("funding_rate.max_funding_rate_pct", currentConfig.FundingRate.MaxFundingRatePct, defaultConfig.FundingRate.MaxFundingRatePct)
		compareField("funding_rate.exit_before_funding_minutes", currentConfig.FundingRate.ExitBeforeFundingMinutes, defaultConfig.FundingRate.ExitBeforeFundingMinutes)
		compareField("funding_rate.block_entry_above_rate_pct", currentConfig.FundingRate.BlockEntryAboveRatePct, defaultConfig.FundingRate.BlockEntryAboveRatePct)
	}

	// Risk section
	if currentConfig.Risk != nil && defaultConfig.Risk != nil {
		compareField("risk.risk_level", currentConfig.Risk.RiskLevel, defaultConfig.Risk.RiskLevel)
		compareField("risk.max_drawdown_percent", currentConfig.Risk.MaxDrawdownPercent, defaultConfig.Risk.MaxDrawdownPercent)
		compareField("risk.max_daily_loss_percent", currentConfig.Risk.MaxDailyLossPercent, defaultConfig.Risk.MaxDailyLossPercent)
		compareField("risk.position_risk_percent", currentConfig.Risk.PositionRiskPercent, defaultConfig.Risk.PositionRiskPercent)
	}

	// Trend divergence section
	if currentConfig.TrendDivergence != nil && defaultConfig.TrendDivergence != nil {
		compareField("trend_divergence.enabled", currentConfig.TrendDivergence.Enabled, defaultConfig.TrendDivergence.Enabled)
		compareField("trend_divergence.min_divergence_percent", currentConfig.TrendDivergence.MinDivergencePercent, defaultConfig.TrendDivergence.MinDivergencePercent)
		compareField("trend_divergence.block_on_divergence", currentConfig.TrendDivergence.BlockOnDivergence, defaultConfig.TrendDivergence.BlockOnDivergence)
		compareField("trend_divergence.divergence_weight", currentConfig.TrendDivergence.DivergenceWeight, defaultConfig.TrendDivergence.DivergenceWeight)
	}

	// Dynamic AI exit section
	if currentConfig.DynamicAIExit != nil && defaultConfig.DynamicAIExit != nil {
		compareField("dynamic_ai_exit.enabled", currentConfig.DynamicAIExit.Enabled, defaultConfig.DynamicAIExit.Enabled)
		compareField("dynamic_ai_exit.min_hold_before_ai_ms", currentConfig.DynamicAIExit.MinHoldBeforeAIMs, defaultConfig.DynamicAIExit.MinHoldBeforeAIMs)
		compareField("dynamic_ai_exit.ai_check_interval_ms", currentConfig.DynamicAIExit.AICheckIntervalMs, defaultConfig.DynamicAIExit.AICheckIntervalMs)
		compareField("dynamic_ai_exit.use_llm_for_loss", currentConfig.DynamicAIExit.UseLLMForLoss, defaultConfig.DynamicAIExit.UseLLMForLoss)
		compareField("dynamic_ai_exit.use_llm_for_profit", currentConfig.DynamicAIExit.UseLLMForProfit, defaultConfig.DynamicAIExit.UseLLMForProfit)
		compareField("dynamic_ai_exit.max_hold_time_ms", currentConfig.DynamicAIExit.MaxHoldTimeMs, defaultConfig.DynamicAIExit.MaxHoldTimeMs)
	}

	// Early warning section
	if currentConfig.EarlyWarning != nil && defaultConfig.EarlyWarning != nil {
		compareField("early_warning.enabled", currentConfig.EarlyWarning.Enabled, defaultConfig.EarlyWarning.Enabled)
		compareField("early_warning.start_after_minutes", currentConfig.EarlyWarning.StartAfterMinutes, defaultConfig.EarlyWarning.StartAfterMinutes)
		compareField("early_warning.min_loss_percent", currentConfig.EarlyWarning.MinLossPercent, defaultConfig.EarlyWarning.MinLossPercent)
		compareField("early_warning.check_interval_secs", currentConfig.EarlyWarning.CheckIntervalSecs, defaultConfig.EarlyWarning.CheckIntervalSecs)
		compareField("early_warning.close_min_hold_mins", currentConfig.EarlyWarning.CloseMinHoldMins, defaultConfig.EarlyWarning.CloseMinHoldMins)
	}

	// Position sizing section (expanded from top-level fields)
	if currentConfig.PositionSizing != nil && defaultConfig.PositionSizing != nil {
		compareField("position_sizing.max_size_usd", currentConfig.PositionSizing.MaxSizeUSD, defaultConfig.PositionSizing.MaxSizeUSD)
		compareField("position_sizing.min_position_size_usd", currentConfig.PositionSizing.MinPositionSizeUSD, defaultConfig.PositionSizing.MinPositionSizeUSD)
		compareField("position_sizing.safety_margin", currentConfig.PositionSizing.SafetyMargin, defaultConfig.PositionSizing.SafetyMargin)
		compareField("position_sizing.auto_size_enabled", currentConfig.PositionSizing.AutoSizeEnabled, defaultConfig.PositionSizing.AutoSizeEnabled)
		compareField("position_sizing.auto_size_min_cover_fee", currentConfig.PositionSizing.AutoSizeMinCoverFee, defaultConfig.PositionSizing.AutoSizeMinCoverFee)
	}

	// Entry conditions V2 (typed struct - Story 11.41)
	if currentConfig.EntryConditionsV2 != nil && defaultConfig.EntryConditionsV2 != nil {
		compareField("entry_conditions.adx_min", currentConfig.EntryConditionsV2.ADXMin, defaultConfig.EntryConditionsV2.ADXMin)
		compareField("entry_conditions.adx_max", currentConfig.EntryConditionsV2.ADXMax, defaultConfig.EntryConditionsV2.ADXMax)
		compareField("entry_conditions.rsi_min", currentConfig.EntryConditionsV2.RSIMin, defaultConfig.EntryConditionsV2.RSIMin)
		compareField("entry_conditions.rsi_max", currentConfig.EntryConditionsV2.RSIMax, defaultConfig.EntryConditionsV2.RSIMax)
		compareField("entry_conditions.require_trend_align", currentConfig.EntryConditionsV2.RequireTrendAlign, defaultConfig.EntryConditionsV2.RequireTrendAlign)
		compareField("entry_conditions.min_volume_multiplier", currentConfig.EntryConditionsV2.MinVolumeMultiplier, defaultConfig.EntryConditionsV2.MinVolumeMultiplier)
		compareField("entry_conditions.use_limit_entry", currentConfig.EntryConditionsV2.UseLimitEntry, defaultConfig.EntryConditionsV2.UseLimitEntry)
		compareField("entry_conditions.limit_order_gap_percent", currentConfig.EntryConditionsV2.LimitOrderGapPercent, defaultConfig.EntryConditionsV2.LimitOrderGapPercent)
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
		AllValues:      allValues,
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

// ==================== STORY 11.41: Section-Level Endpoints ====================
// GET/PUT/POST (reset) for individual sections within a mode+strategy config
// ===============================================================================

// ==================== GET /api/modes/:mode/strategies/:strategy/sections/:section ====================
// Get a specific section from a mode+strategy config

func (s *Server) handleGetModeStrategySection(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	strategy := c.Param("strategy")
	section := c.Param("section")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy: %s", strategy))
		return
	}

	if !validateSection(section) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid section: %s. Valid sections: position_sizing, timeframe, mtf, sltp, confidence, entry_conditions, exit_conditions, scoring, circuit_breaker, hedge, averaging, stale_release, position_optimization, funding_rate, risk, trend_divergence, dynamic_ai_exit, early_warning", section))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	// Get section data from cache layer
	sectionData, err := s.settingsCacheService.GetModeStrategySection(ctx, userID, mode, strategy, section)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get section: "+err.Error())
		return
	}

	// Parse section data back to interface for JSON response
	var sectionValue interface{}
	if err := json.Unmarshal(sectionData, &sectionValue); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to parse section data")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"mode":     mode,
		"strategy": strategy,
		"section":  section,
		"data":     sectionValue,
	})
}

// ==================== PUT /api/modes/:mode/strategies/:strategy/sections/:section ====================
// Update a specific section within a mode+strategy config

func (s *Server) handleUpdateModeStrategySection(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	strategy := c.Param("strategy")
	section := c.Param("section")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy: %s", strategy))
		return
	}

	if !validateSection(section) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid section: %s", section))
		return
	}

	// Parse request body - expecting the section data directly
	var sectionData interface{}
	if err := c.ShouldBindJSON(&sectionData); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Marshal to JSON bytes for cache layer
	sectionBytes, err := json.Marshal(sectionData)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Failed to process section data")
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	// Update section via cache layer (write-through to DB)
	if err := s.settingsCacheService.SetModeStrategySection(ctx, userID, mode, strategy, section, sectionBytes); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update section: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Section %s updated for %s/%s", section, mode, strategy),
		"mode":    mode,
		"strategy": strategy,
		"section": section,
		"data":    sectionData,
	})
}

// ==================== POST /api/modes/:mode/strategies/:strategy/sections/:section/reset ====================
// Reset a specific section to defaults

func (s *Server) handleResetModeStrategySection(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	strategy := c.Param("strategy")
	section := c.Param("section")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy: %s", strategy))
		return
	}

	if !validateSection(section) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid section: %s", section))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	// Get default config for this mode+strategy
	defaultConfig := database.DefaultModeStrategyConfig(mode, strategy)

	// Extract the default section data
	defaultSectionData := extractSectionFromConfig(defaultConfig, section)
	if defaultSectionData == nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to get default section data")
		return
	}

	// Marshal to JSON bytes
	sectionBytes, err := json.Marshal(defaultSectionData)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to process default section data")
		return
	}

	// Reset section via cache layer (write-through to DB)
	if err := s.settingsCacheService.SetModeStrategySection(ctx, userID, mode, strategy, section, sectionBytes); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to reset section: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Section %s reset to defaults for %s/%s", section, mode, strategy),
		"mode":    mode,
		"strategy": strategy,
		"section": section,
		"data":    defaultSectionData,
	})
}

// extractSectionFromConfig extracts a specific section from ModeStrategyConfig
func extractSectionFromConfig(config *database.ModeStrategyConfig, section string) interface{} {
	switch section {
	case "position_sizing":
		if config.PositionSizing != nil {
			return config.PositionSizing
		}
		return &database.StrategyPositionSizing{
			Leverage:     config.Leverage,
			MaxPositions: config.MaxPositions,
			BaseSizeUSD:  config.BaseSizeUSD,
		}
	case "timeframe":
		return &config.Timeframe
	case "mtf":
		return config.MTF
	case "sltp":
		return &config.SLTP
	case "confidence":
		return &config.Confidence
	case "entry_conditions":
		if config.EntryConditionsV2 != nil {
			return config.EntryConditionsV2
		}
		return config.EntryConditions
	case "exit_conditions":
		return &config.ExitConditions
	case "scoring":
		return &config.Scoring
	case "circuit_breaker":
		return config.CircuitBreaker
	case "hedge":
		return config.Hedge
	case "averaging":
		return config.Averaging
	case "stale_release":
		return config.StaleRelease
	case "position_optimization":
		return config.PositionOptimization
	case "funding_rate":
		return config.FundingRate
	case "risk":
		return config.Risk
	case "trend_divergence":
		return config.TrendDivergence
	case "dynamic_ai_exit":
		return config.DynamicAIExit
	case "early_warning":
		return config.EarlyWarning
	default:
		return nil
	}
}

// ==================== GET /api/modes/:mode/strategies/:strategy/sections ====================
// List all available sections with their current values

func (s *Server) handleListModeStrategySections(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	strategy := c.Param("strategy")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy: %s", strategy))
		return
	}

	ctx := c.Request.Context()

	if s.settingsCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings cache service not available")
		return
	}

	// Get full config
	config, err := s.settingsCacheService.GetModeStrategyConfig(ctx, userID, mode, strategy)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get config: "+err.Error())
		return
	}

	// Get default config for comparison
	defaultConfig := database.DefaultModeStrategyConfig(mode, strategy)

	// Build sections list with is_default flag
	sections := make(map[string]interface{})
	sectionMeta := make(map[string]map[string]interface{})

	for sectionName := range validSectionNames {
		currentData := extractSectionFromConfig(config, sectionName)
		defaultData := extractSectionFromConfig(defaultConfig, sectionName)

		// Check if current matches default
		isDefault := compareValues(currentData, defaultData)

		sections[sectionName] = currentData
		sectionMeta[sectionName] = map[string]interface{}{
			"is_default":    isDefault,
			"has_data":      currentData != nil,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"mode":          mode,
		"strategy":      strategy,
		"sections":      sections,
		"section_meta":  sectionMeta,
		"total_sections": len(validSectionNames),
	})
}
