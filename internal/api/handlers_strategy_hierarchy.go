package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/cache"
	"binance-trading-bot/internal/database"

	"github.com/gin-gonic/gin"
)

// ==================== STORY 11.45: Strategy Hierarchy API Endpoints ====================
// API endpoints for strategy hierarchy (Mode -> Strategy Group -> Sub-Strategy)
// Supports the Volume Imbalance strategy and future sub-strategy additions
// Uses cache-first read pattern with write-through to DB
// ======================================================================================

// ==================== VALIDATION HELPERS ====================

// validStrategyGroups maps valid strategy group names
var validStrategyGroups = map[string]bool{
	"breakout":  true,
	"trending":  true,
	"range":     true,
	"volatile":  true,
}

// validSubStrategies maps valid sub-strategy names
var validSubStrategies = map[string]bool{
	"ravindra_volume_imbalance": true,
	"classic_breakout":          true,
}

// validateStrategyGroup checks if a strategy group name is valid
func validateStrategyGroup(group string) bool {
	return validStrategyGroups[group]
}

// validateSubStrategy checks if a sub-strategy name is valid
func validateSubStrategy(subStrategy string) bool {
	return validSubStrategies[subStrategy]
}

// ==================== REQUEST/RESPONSE TYPES ====================

// StrategyGroupResponse is the API response format for a strategy group
type StrategyGroupResponse struct {
	ID                  string  `json:"id"`
	Mode                string  `json:"mode"`
	StrategyGroup       string  `json:"strategy_group"`
	Enabled             bool    `json:"enabled"`
	Timeframe           string  `json:"timeframe"`
	PositionSizePercent float64 `json:"position_size_percent"`
	MaxLeverage         int     `json:"max_leverage"`
	MaxPositions        int     `json:"max_positions"`
	MinVolumeUSDT       float64 `json:"min_volume_usdt"`
}

// SubStrategyResponse is the API response format for a sub-strategy
type SubStrategyResponse struct {
	ID            string          `json:"id"`
	Mode          string          `json:"mode"`
	StrategyGroup string          `json:"strategy_group"`
	SubStrategy   string          `json:"sub_strategy"`
	Enabled       bool            `json:"enabled"`
	Settings      json.RawMessage `json:"settings"`
}

// UpdateStrategyGroupRequest is the request body for updating a strategy group
type UpdateStrategyGroupRequest struct {
	Enabled             *bool    `json:"enabled,omitempty"`
	Timeframe           *string  `json:"timeframe,omitempty"`
	PositionSizePercent *float64 `json:"position_size_percent,omitempty"`
	MaxLeverage         *int     `json:"max_leverage,omitempty"`
	MaxPositions        *int     `json:"max_positions,omitempty"`
	MinVolumeUSDT       *float64 `json:"min_volume_usdt,omitempty"`
}

// UpdateSubStrategyRequest is the request body for updating a sub-strategy
type UpdateSubStrategyRequest struct {
	Enabled  *bool           `json:"enabled,omitempty"`
	Settings json.RawMessage `json:"settings,omitempty"`
}

// StrategyGroupComparisonResponse shows diff between user settings and defaults
type StrategyGroupComparisonResponse struct {
	Mode          string                        `json:"mode"`
	StrategyGroup string                        `json:"strategy_group"`
	AllMatch      bool                          `json:"all_match"`
	Differences   []StrategyGroupFieldComparison `json:"differences"`
}

// StrategyGroupFieldComparison represents a single field comparison
type StrategyGroupFieldComparison struct {
	Field   string      `json:"field"`
	Current interface{} `json:"current"`
	Default interface{} `json:"default"`
}

// SubStrategyComparisonResponse shows diff between user settings and defaults
type SubStrategyComparisonResponse struct {
	Mode          string                       `json:"mode"`
	StrategyGroup string                       `json:"strategy_group"`
	SubStrategy   string                       `json:"sub_strategy"`
	AllMatch      bool                         `json:"all_match"`
	Differences   []SubStrategyFieldComparison `json:"differences"`
}

// SubStrategyFieldComparison represents a single field comparison for sub-strategy
type SubStrategyFieldComparison struct {
	Field   string      `json:"field"`
	Current interface{} `json:"current"`
	Default interface{} `json:"default"`
}

// PatternStateResponse represents a volume imbalance pattern state
type PatternStateResponse struct {
	Symbol               string  `json:"symbol"`
	Mode                 string  `json:"mode"`
	State                string  `json:"state"`
	ReferenceHigh        float64 `json:"reference_high,omitempty"`
	ReferenceLow         float64 `json:"reference_low,omitempty"`
	ReferenceVolume      float64 `json:"reference_volume,omitempty"`
	ConsolidationCandles int     `json:"consolidation_candles,omitempty"`
	ConsolidationLow     float64 `json:"consolidation_low,omitempty"`
	ConsolidationHigh    float64 `json:"consolidation_high,omitempty"`
	VolumeTrend          float64 `json:"volume_trend,omitempty"`
	IsValid              bool    `json:"is_valid"`
	InvalidReason        string  `json:"invalid_reason,omitempty"`
	UpdatedAt            int64   `json:"updated_at"`
	ExpiresAt            int64   `json:"expires_at,omitempty"`
}

// EnabledStrategyResponse represents an enabled sub-strategy
type EnabledStrategyResponse struct {
	Mode          string `json:"mode"`
	StrategyGroup string `json:"strategy_group"`
	SubStrategy   string `json:"sub_strategy"`
}

// ==================== STRATEGY GROUP ENDPOINTS ====================

// handleGetStrategyGroupsForMode handles GET /api/futures/strategy-groups/:mode
// Returns all strategy groups for a mode
func (s *Server) handleGetStrategyGroupsForMode(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: scalp, swing, position, ultra_fast", mode))
		return
	}

	ctx := c.Request.Context()

	// Check if strategy hierarchy cache service is available
	if s.strategyHierarchyCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Strategy hierarchy cache service not available")
		return
	}

	groups, err := s.strategyHierarchyCacheService.GetAllStrategyGroupsForMode(ctx, userID, mode)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get strategy groups: "+err.Error())
		return
	}

	// Convert to response format
	responses := make([]StrategyGroupResponse, 0, len(groups))
	for _, g := range groups {
		responses = append(responses, convertStrategyGroupToResponse(g))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"mode":    mode,
		"groups":  responses,
		"count":   len(responses),
	})
}

// handleUpdateStrategyGroup handles PUT /api/futures/strategy-groups/:mode/:group
// Updates a strategy group settings
func (s *Server) handleUpdateStrategyGroup(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	group := c.Param("group")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategyGroup(group) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy group: %s. Valid groups: breakout, trending, range, volatile", group))
		return
	}

	var req UpdateStrategyGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	if s.strategyHierarchyCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Strategy hierarchy cache service not available")
		return
	}

	// Get existing settings or create new
	existing, err := s.strategyHierarchyCacheService.GetStrategyGroup(ctx, userID, mode, group)
	if err != nil && err != cache.ErrCacheUnavailable {
		errorResponse(c, http.StatusInternalServerError, "Failed to get existing settings: "+err.Error())
		return
	}

	// If not found, create with defaults
	if existing == nil {
		existing = getDefaultStrategyGroup(mode, group, userID)
	}

	// Apply updates
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Timeframe != nil {
		existing.Timeframe = *req.Timeframe
	}
	if req.PositionSizePercent != nil {
		existing.PositionSizePercent = *req.PositionSizePercent
	}
	if req.MaxLeverage != nil {
		existing.MaxLeverage = *req.MaxLeverage
	}
	if req.MaxPositions != nil {
		existing.MaxPositions = *req.MaxPositions
	}
	if req.MinVolumeUSDT != nil {
		existing.MinVolumeUSDT = *req.MinVolumeUSDT
	}

	// Save with write-through
	if err := s.strategyHierarchyCacheService.UpdateStrategyGroup(ctx, existing); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update strategy group: "+err.Error())
		return
	}

	// Notify autopilot system that settings have changed and refresh CoinProfiler subscriptions
	var enabledStrategies int
	if s.userAutopilotManager != nil {
		s.userAutopilotManager.NotifySettingsChanged(ctx, userID, "strategy_group", mode, group, "")
		enabledStrategies, _ = s.userAutopilotManager.RefreshCoinProfilerSubscriptions(ctx, userID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"message":            fmt.Sprintf("Strategy group %s/%s updated successfully", mode, group),
		"data":               convertStrategyGroupToResponse(existing),
		"enabled_strategies": enabledStrategies,
	})
}

// handleCompareStrategyGroup handles GET /api/futures/strategy-groups/:mode/:group/compare
// Compares user settings with defaults
func (s *Server) handleCompareStrategyGroup(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	group := c.Param("group")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategyGroup(group) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy group: %s", group))
		return
	}

	ctx := c.Request.Context()

	if s.strategyHierarchyCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Strategy hierarchy cache service not available")
		return
	}

	// Get current user settings
	current, err := s.strategyHierarchyCacheService.GetStrategyGroup(ctx, userID, mode, group)
	if err != nil && err != cache.ErrCacheUnavailable {
		errorResponse(c, http.StatusInternalServerError, "Failed to get current settings: "+err.Error())
		return
	}

	// Get defaults
	defaults := getDefaultStrategyGroup(mode, group, userID)

	// If user has no settings, use defaults as current
	if current == nil {
		current = defaults
	}

	// Compare fields
	differences := compareStrategyGroupSettings(current, defaults)

	c.JSON(http.StatusOK, StrategyGroupComparisonResponse{
		Mode:          mode,
		StrategyGroup: group,
		AllMatch:      len(differences) == 0,
		Differences:   differences,
	})
}

// ==================== SUB-STRATEGY ENDPOINTS ====================

// handleGetSubStrategiesForGroup handles GET /api/futures/sub-strategies/:mode/:group
// Returns all sub-strategies for a group
func (s *Server) handleGetSubStrategiesForGroup(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	group := c.Param("group")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategyGroup(group) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy group: %s", group))
		return
	}

	ctx := c.Request.Context()

	if s.strategyHierarchyCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Strategy hierarchy cache service not available")
		return
	}

	// Get the parent strategy group to check if it's enabled
	strategyGroup, err := s.strategyHierarchyCacheService.GetStrategyGroup(ctx, userID, mode, group)
	if err != nil && err != cache.ErrCacheUnavailable {
		// Log but don't fail - we can still return sub-strategies
	}
	parentEnabled := strategyGroup != nil && strategyGroup.Enabled

	subStrategies, err := s.strategyHierarchyCacheService.GetAllSubStrategiesForGroup(ctx, userID, mode, group)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get sub-strategies: "+err.Error())
		return
	}

	// Convert to response format
	responses := make([]SubStrategyResponse, 0, len(subStrategies))
	for _, ss := range subStrategies {
		responses = append(responses, convertSubStrategyToResponse(ss))
	}

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"mode":                 mode,
		"strategy_group":       group,
		"strategy_group_enabled": parentEnabled,
		"sub_strategies":       responses,
		"count":                len(responses),
	})
}

// handleUpdateSubStrategy handles PUT /api/futures/sub-strategies/:mode/:group/:strategy
// Updates a sub-strategy settings
func (s *Server) handleUpdateSubStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	group := c.Param("group")
	strategy := c.Param("strategy")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategyGroup(group) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy group: %s", group))
		return
	}

	if !validateSubStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid sub-strategy: %s", strategy))
		return
	}

	var req UpdateSubStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	if s.strategyHierarchyCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Strategy hierarchy cache service not available")
		return
	}

	// Get existing settings or create new
	existing, err := s.strategyHierarchyCacheService.GetSubStrategy(ctx, userID, mode, group, strategy)
	if err != nil && err != cache.ErrCacheUnavailable {
		errorResponse(c, http.StatusInternalServerError, "Failed to get existing settings: "+err.Error())
		return
	}

	// If not found, create with defaults
	if existing == nil {
		existing = getDefaultSubStrategy(mode, group, strategy, userID)
	}

	// Apply updates
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Settings != nil {
		existing.Settings = req.Settings
	}

	// Save with write-through
	if err := s.strategyHierarchyCacheService.UpdateSubStrategy(ctx, existing); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update sub-strategy: "+err.Error())
		return
	}

	// Notify autopilot system that settings have changed
	// This triggers a reload of pattern matcher config and refreshes CoinProfiler subscriptions
	var enabledStrategies int
	if s.userAutopilotManager != nil {
		s.userAutopilotManager.NotifySettingsChanged(ctx, userID, "sub_strategy", mode, group, strategy)
		enabledStrategies, _ = s.userAutopilotManager.RefreshCoinProfilerSubscriptions(ctx, userID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"message":            fmt.Sprintf("Sub-strategy %s/%s/%s updated successfully", mode, group, strategy),
		"data":               convertSubStrategyToResponse(existing),
		"enabled_strategies": enabledStrategies,
	})
}

// handleCompareSubStrategy handles GET /api/futures/sub-strategies/:mode/:group/:strategy/compare
// Compares user sub-strategy settings with defaults
func (s *Server) handleCompareSubStrategy(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	group := c.Param("group")
	strategy := c.Param("strategy")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategyGroup(group) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy group: %s", group))
		return
	}

	if !validateSubStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid sub-strategy: %s", strategy))
		return
	}

	ctx := c.Request.Context()

	if s.strategyHierarchyCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Strategy hierarchy cache service not available")
		return
	}

	// Get current user settings
	current, err := s.strategyHierarchyCacheService.GetSubStrategy(ctx, userID, mode, group, strategy)
	if err != nil && err != cache.ErrCacheUnavailable {
		errorResponse(c, http.StatusInternalServerError, "Failed to get current settings: "+err.Error())
		return
	}

	// Get defaults
	defaults := getDefaultSubStrategy(mode, group, strategy, userID)

	// If user has no settings, use defaults as current
	if current == nil {
		current = defaults
	}

	// Compare fields
	differences := compareSubStrategySettings(current, defaults)

	c.JSON(http.StatusOK, SubStrategyComparisonResponse{
		Mode:          mode,
		StrategyGroup: group,
		SubStrategy:   strategy,
		AllMatch:      len(differences) == 0,
		Differences:   differences,
	})
}

// ==================== PATTERN STATE ENDPOINTS ====================

// handleGetVolumeImbalancePatterns handles GET /api/futures/patterns/volume-imbalance
// Returns current pattern states for all symbols
func (s *Server) handleGetVolumeImbalancePatterns(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	// Get user's autopilot instance to access VolumeImbalanceDetector
	if s.userAutopilotManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"patterns": []PatternStateResponse{},
			"count":    0,
			"message":  "Autopilot not initialized - no active patterns",
		})
		return
	}

	instance := s.userAutopilotManager.GetInstance(userID)
	if instance == nil || instance.Autopilot == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"patterns": []PatternStateResponse{},
			"count":    0,
			"message":  "Autopilot not running - no active patterns",
		})
		return
	}

	// Get the VolumeImbalanceDetector from the autopilot
	detector := instance.Autopilot.GetVolumeImbalanceDetector()
	if detector == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"patterns": []PatternStateResponse{},
			"count":    0,
			"message":  "Volume imbalance detector not enabled",
		})
		return
	}

	// Get all patterns
	patterns := detector.GetAllPatterns()
	responses := make([]PatternStateResponse, 0, len(patterns))

	for _, p := range patterns {
		responses = append(responses, convertPatternToResponse(p))
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"patterns": responses,
		"count":    len(responses),
	})
}

// handleGetVolumeImbalancePatternForSymbol handles GET /api/futures/patterns/volume-imbalance/:symbol
// Returns pattern state for specific symbol
func (s *Server) handleGetVolumeImbalancePatternForSymbol(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	// Get user's autopilot instance - return 200 OK with null pattern if not available
	// (consistent with "no patterns detected" behavior)
	if s.userAutopilotManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"symbol":  symbol,
			"pattern": nil,
			"message": "Autopilot manager not available - no active patterns",
		})
		return
	}

	instance := s.userAutopilotManager.GetInstance(userID)
	if instance == nil || instance.Autopilot == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"symbol":  symbol,
			"pattern": nil,
			"message": "Autopilot not running - no active patterns",
		})
		return
	}

	detector := instance.Autopilot.GetVolumeImbalanceDetector()
	if detector == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"symbol":  symbol,
			"pattern": nil,
			"message": "Volume imbalance detector not enabled",
		})
		return
	}

	pattern := detector.GetPattern(symbol)
	if pattern == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"symbol":  symbol,
			"pattern": nil,
			"message": "No active pattern for this symbol",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"symbol":  symbol,
		"pattern": convertPatternToResponse(pattern),
	})
}

// ==================== ENABLED STRATEGIES ENDPOINT ====================

// handleGetEnabledStrategies handles GET /api/futures/enabled-strategies
// Returns all enabled sub-strategies for the user
func (s *Server) handleGetEnabledStrategies(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	ctx := c.Request.Context()

	if s.strategyHierarchyCacheService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Strategy hierarchy cache service not available")
		return
	}

	strategies, err := s.strategyHierarchyCacheService.GetEnabledStrategies(ctx, userID)
	if err != nil {
		if err == cache.ErrCacheUnavailable {
			errorResponse(c, http.StatusServiceUnavailable, "Cache service unavailable")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to get enabled strategies: "+err.Error())
		return
	}

	// Convert to response format
	responses := make([]EnabledStrategyResponse, 0, len(strategies))
	for _, s := range strategies {
		responses = append(responses, EnabledStrategyResponse{
			Mode:          s.Mode,
			StrategyGroup: s.StrategyGroup,
			SubStrategy:   s.SubStrategy,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"strategies": responses,
		"count":      len(responses),
	})
}

// ==================== SUB-STRATEGY EQUITY ENDPOINT ====================

// handleGetSubStrategyEquity handles GET /api/futures/sub-strategies/:mode/:group/:strategy/equity
// Returns the effective budget, capital in use, and available capital for a sub-strategy
func (s *Server) handleGetSubStrategyEquity(c *gin.Context) {
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	mode := c.Param("mode")
	group := c.Param("group")
	strategy := c.Param("strategy")

	if !validateMode(mode) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	if !validateStrategyGroup(group) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid strategy group: %s", group))
		return
	}

	if !validateSubStrategy(strategy) {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid sub-strategy: %s", strategy))
		return
	}

	ctx := c.Request.Context()

	// Get effective budget (current_equity or assigned_budget_usd)
	var effectiveBudget float64
	if s.repo != nil {
		equity, err := s.repo.GetSubStrategyEquity(ctx, userID, mode, group, strategy)
		if err != nil {
			log.Printf("[STRATEGY-EQUITY] Failed to get equity for %s/%s/%s: %v", mode, group, strategy, err)
			errorResponse(c, http.StatusInternalServerError, "Failed to get equity: "+err.Error())
			return
		}
		effectiveBudget = equity
	}

	// Get capital in use from active chains
	var capitalInUse float64
	var activeChains int
	if s.repo != nil {
		var err error
		capitalInUse, activeChains, err = s.repo.GetDB().GetCapitalInUseForSubStrategy(ctx, userID, mode, group, strategy)
		if err != nil {
			log.Printf("[STRATEGY-EQUITY] Failed to get capital in use for %s/%s/%s: %v", mode, group, strategy, err)
			// Non-fatal - continue with 0
		}
	}

	available := effectiveBudget - capitalInUse
	if available < 0 {
		available = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"effective_budget": effectiveBudget,
		"capital_in_use":   capitalInUse,
		"available":        available,
		"active_chains":    activeChains,
	})
}

// ==================== HELPER FUNCTIONS ====================

// convertStrategyGroupToResponse converts database model to API response
func convertStrategyGroupToResponse(g *database.StrategyGroupSettings) StrategyGroupResponse {
	return StrategyGroupResponse{
		ID:                  g.ID,
		Mode:                g.Mode,
		StrategyGroup:       g.StrategyGroup,
		Enabled:             g.Enabled,
		Timeframe:           g.Timeframe,
		PositionSizePercent: g.PositionSizePercent,
		MaxLeverage:         g.MaxLeverage,
		MaxPositions:        g.MaxPositions,
		MinVolumeUSDT:       g.MinVolumeUSDT,
	}
}

// convertSubStrategyToResponse converts database model to API response
func convertSubStrategyToResponse(ss *database.SubStrategySettings) SubStrategyResponse {
	return SubStrategyResponse{
		ID:            ss.ID,
		Mode:          ss.Mode,
		StrategyGroup: ss.StrategyGroup,
		SubStrategy:   ss.SubStrategy,
		Enabled:       ss.Enabled,
		Settings:      ss.Settings,
	}
}

// convertPatternToResponse converts VolumeImbalancePattern to API response
func convertPatternToResponse(p interface{}) PatternStateResponse {
	// Type assertion to access the pattern fields
	// We use interface{} to avoid circular import with autopilot package
	type patternData struct {
		Symbol               string
		Mode                 string
		State                string
		ReferenceHigh        float64
		ReferenceLow         float64
		ReferenceVolume      float64
		ConsolidationCandles int
		ConsolidationLow     float64
		ConsolidationHigh    float64
		VolumeTrend          float64
		IsValid              bool
		InvalidReason        string
		UpdatedAt            int64
		ExpiresAt            int64
	}

	// Use JSON marshaling/unmarshaling to convert the pattern
	data, err := json.Marshal(p)
	if err != nil {
		return PatternStateResponse{IsValid: false, InvalidReason: "Failed to serialize pattern"}
	}

	var pd struct {
		Symbol           string `json:"symbol"`
		Mode             string `json:"mode"`
		State            string `json:"state"`
		ReferenceCandle  struct {
			High   float64 `json:"high"`
			Low    float64 `json:"low"`
			Volume float64 `json:"volume"`
		} `json:"reference_candle"`
		ConsolidationCandles int     `json:"consolidation_candles"`
		ConsolidationLow     float64 `json:"consolidation_low"`
		ConsolidationHigh    float64 `json:"consolidation_high"`
		VolumeTrend          float64 `json:"volume_trend"`
		IsValid              bool    `json:"is_valid"`
		InvalidReason        string  `json:"invalid_reason"`
		UpdatedAt            string  `json:"updated_at"`
		ExpiresAt            string  `json:"expires_at"`
	}

	if err := json.Unmarshal(data, &pd); err != nil {
		return PatternStateResponse{IsValid: false, InvalidReason: "Failed to parse pattern"}
	}

	// Parse timestamps from string to int64
	var updatedAt, expiresAt int64
	if pd.UpdatedAt != "" {
		t, err := time.Parse(time.RFC3339, pd.UpdatedAt)
		if err == nil {
			updatedAt = t.Unix()
		}
	}
	if pd.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, pd.ExpiresAt)
		if err == nil {
			expiresAt = t.Unix()
		}
	}

	return PatternStateResponse{
		Symbol:               pd.Symbol,
		Mode:                 pd.Mode,
		State:                pd.State,
		ReferenceHigh:        pd.ReferenceCandle.High,
		ReferenceLow:         pd.ReferenceCandle.Low,
		ReferenceVolume:      pd.ReferenceCandle.Volume,
		ConsolidationCandles: pd.ConsolidationCandles,
		ConsolidationLow:     pd.ConsolidationLow,
		ConsolidationHigh:    pd.ConsolidationHigh,
		VolumeTrend:          pd.VolumeTrend,
		IsValid:              pd.IsValid,
		InvalidReason:        pd.InvalidReason,
		UpdatedAt:            updatedAt,
		ExpiresAt:            expiresAt,
	}
}

// getDefaultStrategyGroup returns default settings for a strategy group
// Loads from default-settings.json if available, otherwise uses hardcoded fallbacks
func getDefaultStrategyGroup(mode, group, userID string) *database.StrategyGroupSettings {
	// Try to load from default-settings.json first
	if autopilot.HasStrategyHierarchy() {
		groupDefaults, err := autopilot.GetStrategyGroupDefaults(mode, group)
		if err == nil && groupDefaults != nil {
			return &database.StrategyGroupSettings{
				UserID:              userID,
				Mode:                mode,
				StrategyGroup:       group,
				Enabled:             groupDefaults.Enabled,
				Timeframe:           groupDefaults.BaseSettings.Timeframe,
				PositionSizePercent: groupDefaults.BaseSettings.PositionSizePercent,
				MaxLeverage:         groupDefaults.BaseSettings.MaxLeverage,
				MaxPositions:        groupDefaults.BaseSettings.MaxPositions,
				MinVolumeUSDT:       groupDefaults.BaseSettings.MinVolumeUSDT,
			}
		}
		log.Printf("[STRATEGY-HIERARCHY] Failed to load defaults from file for %s/%s: %v, using fallback", mode, group, err)
	}

	// Fallback: Hardcoded default settings based on mode and group
	defaults := &database.StrategyGroupSettings{
		UserID:              userID,
		Mode:                mode,
		StrategyGroup:       group,
		Enabled:             true,
		PositionSizePercent: 2.0,
		MaxPositions:        3,
		MinVolumeUSDT:       1000000,
	}

	// Mode-specific defaults
	switch mode {
	case "scalp":
		defaults.Timeframe = "15m"
		defaults.MaxLeverage = 10
	case "swing":
		defaults.Timeframe = "1h"
		defaults.MaxLeverage = 5
	case "position":
		defaults.Timeframe = "4h"
		defaults.MaxLeverage = 3
	case "ultra_fast":
		defaults.Timeframe = "5m"
		defaults.MaxLeverage = 20
	}

	// Group-specific adjustments
	switch group {
	case "volatile":
		defaults.PositionSizePercent = 1.5 // Lower size for volatile
		defaults.MaxLeverage = defaults.MaxLeverage - 2
		if defaults.MaxLeverage < 2 {
			defaults.MaxLeverage = 2
		}
	case "breakout":
		defaults.PositionSizePercent = 2.5 // Higher size for breakouts
	}

	return defaults
}

// getDefaultSubStrategy returns default settings for a sub-strategy
// Loads from default-settings.json if available, otherwise uses hardcoded fallbacks
func getDefaultSubStrategy(mode, group, strategy, userID string) *database.SubStrategySettings {
	// Try to load from default-settings.json first
	hasHierarchy := autopilot.HasStrategyHierarchy()
	log.Printf("[STRATEGY-HIERARCHY-DEBUG] HasStrategyHierarchy=%v for %s/%s/%s", hasHierarchy, mode, group, strategy)

	if hasHierarchy {
		subDefaults, err := autopilot.GetSubStrategyDefaults(mode, group, strategy)
		if err == nil && subDefaults != nil {
			log.Printf("[STRATEGY-HIERARCHY-DEBUG] Loaded from default-settings.json: enabled=%v, settings_len=%d", subDefaults.Enabled, len(subDefaults.Settings))
			return &database.SubStrategySettings{
				UserID:        userID,
				Mode:          mode,
				StrategyGroup: group,
				SubStrategy:   strategy,
				Enabled:       subDefaults.Enabled,
				Settings:      subDefaults.Settings,
			}
		}
		log.Printf("[STRATEGY-HIERARCHY] Failed to load sub-strategy defaults from file for %s/%s/%s: %v, using fallback", mode, group, strategy, err)
	} else {
		log.Printf("[STRATEGY-HIERARCHY-DEBUG] No strategy_hierarchy section in default-settings.json, using fallback")
	}

	// Fallback: Hardcoded default settings
	defaults := &database.SubStrategySettings{
		UserID:        userID,
		Mode:          mode,
		StrategyGroup: group,
		SubStrategy:   strategy,
		Enabled:       true,
	}

	// Strategy-specific default settings
	// Based on Dec 2025 - Jan 2026 backtest: 51 trades, 47.1% WR, +1147% net return
	switch strategy {
	case "ravindra_volume_imbalance":
		settings := map[string]interface{}{
			"risk_reward": map[string]interface{}{
				"risk":      1,
				"reward":    4,
				"min_ratio": 3,
			},
			"llm_validation_enabled": false,
			"trailing_stop": map[string]interface{}{
				"enabled":               true,
				"activation_profit_pct": 2.0, // Activate at 2:1 R:R
				"initial_trail_pct":     0.0, // Move SL to breakeven
				"milestones": []map[string]interface{}{
					{"trigger_profit_pct": 2.0, "trail_distance_pct": 0.0, "label": "BE"},   // At 2:1 → breakeven
					{"trigger_profit_pct": 3.0, "trail_distance_pct": 1.0, "label": "+1R"},  // At 3:1 → lock 1:1
				},
			},
			"pattern_detection": map[string]interface{}{
				"direction":                     "long",
				"reference_lookback_candles":    5,
				"volume_spike_threshold":        3.0,
				"require_pre_trend_down":        false, // BACKTESTED: false - original strategy did NOT use this filter
				"breakout_volume_surge":         1.0,
				"breakout_confirmation_candles": 1,
				"entry_volume_vs_reference":     1.0,
				"max_sl_percent":                1.5,
				"max_pattern_age_mins":          60,
			},
			"budget_allocation": map[string]interface{}{
				"assigned_budget_usd":     100,
				"max_concurrent_trades":   1,
				"position_sizing":         "all_in",
				"use_incremental_equity":  true,
			},
			"max_concurrent_patterns": 5,
			"priority":                1,
		}
		data, _ := json.Marshal(settings)
		defaults.Settings = data
	case "classic_breakout":
		settings := map[string]interface{}{
			"enabled":               true,
			"breakout_confirmation": "close_above",
			"volume_multiplier":     1.5,
			"lookback_periods":      20,
		}
		data, _ := json.Marshal(settings)
		defaults.Settings = data
	}

	return defaults
}

// compareStrategyGroupSettings compares two strategy group settings
func compareStrategyGroupSettings(current, defaults *database.StrategyGroupSettings) []StrategyGroupFieldComparison {
	var differences []StrategyGroupFieldComparison

	if current.Enabled != defaults.Enabled {
		differences = append(differences, StrategyGroupFieldComparison{
			Field:   "enabled",
			Current: current.Enabled,
			Default: defaults.Enabled,
		})
	}
	if current.Timeframe != defaults.Timeframe {
		differences = append(differences, StrategyGroupFieldComparison{
			Field:   "timeframe",
			Current: current.Timeframe,
			Default: defaults.Timeframe,
		})
	}
	if current.PositionSizePercent != defaults.PositionSizePercent {
		differences = append(differences, StrategyGroupFieldComparison{
			Field:   "position_size_percent",
			Current: current.PositionSizePercent,
			Default: defaults.PositionSizePercent,
		})
	}
	if current.MaxLeverage != defaults.MaxLeverage {
		differences = append(differences, StrategyGroupFieldComparison{
			Field:   "max_leverage",
			Current: current.MaxLeverage,
			Default: defaults.MaxLeverage,
		})
	}
	if current.MaxPositions != defaults.MaxPositions {
		differences = append(differences, StrategyGroupFieldComparison{
			Field:   "max_positions",
			Current: current.MaxPositions,
			Default: defaults.MaxPositions,
		})
	}
	if current.MinVolumeUSDT != defaults.MinVolumeUSDT {
		differences = append(differences, StrategyGroupFieldComparison{
			Field:   "min_volume_usdt",
			Current: current.MinVolumeUSDT,
			Default: defaults.MinVolumeUSDT,
		})
	}

	return differences
}

// compareSubStrategySettings compares two sub-strategy settings with deep comparison
func compareSubStrategySettings(current, defaults *database.SubStrategySettings) []SubStrategyFieldComparison {
	var differences []SubStrategyFieldComparison

	if current.Enabled != defaults.Enabled {
		differences = append(differences, SubStrategyFieldComparison{
			Field:   "enabled",
			Current: current.Enabled,
			Default: defaults.Enabled,
		})
	}

	// Compare settings JSON with deep comparison
	var currentMap, defaultMap map[string]interface{}
	if err := json.Unmarshal(current.Settings, &currentMap); err != nil {
		currentMap = make(map[string]interface{})
	}
	if err := json.Unmarshal(defaults.Settings, &defaultMap); err != nil {
		defaultMap = make(map[string]interface{})
	}

	// Deep compare and flatten all fields
	flattenAndCompare("settings", currentMap, defaultMap, &differences)

	return differences
}

// flattenAndCompare recursively compares nested maps and adds differences
func flattenAndCompare(prefix string, current, defaults map[string]interface{}, differences *[]SubStrategyFieldComparison) {
	// Track all keys from both maps
	allKeys := make(map[string]bool)
	for k := range defaults {
		allKeys[k] = true
	}
	for k := range current {
		allKeys[k] = true
	}

	for key := range allKeys {
		// Skip underscore-prefixed keys (comments/descriptions)
		if len(key) > 0 && key[0] == '_' {
			continue
		}

		path := prefix + "." + key
		defaultVal, defaultExists := defaults[key]
		currentVal, currentExists := current[key]

		// If default exists but current doesn't
		if defaultExists && !currentExists {
			*differences = append(*differences, SubStrategyFieldComparison{
				Field:   path,
				Current: nil,
				Default: defaultVal,
			})
			continue
		}

		// If current exists but default doesn't
		if currentExists && !defaultExists {
			*differences = append(*differences, SubStrategyFieldComparison{
				Field:   path,
				Current: currentVal,
				Default: nil,
			})
			continue
		}

		// Both exist - compare based on type
		switch dv := defaultVal.(type) {
		case map[string]interface{}:
			// Nested object - recurse
			cv, ok := currentVal.(map[string]interface{})
			if !ok {
				cv = make(map[string]interface{})
			}
			flattenAndCompare(path, cv, dv, differences)
		case []interface{}:
			// Array - compare as JSON strings for simplicity
			defaultJSON, _ := json.Marshal(defaultVal)
			currentJSON, _ := json.Marshal(currentVal)
			if string(defaultJSON) != string(currentJSON) {
				*differences = append(*differences, SubStrategyFieldComparison{
					Field:   path,
					Current: currentVal,
					Default: defaultVal,
				})
			}
		default:
			// Primitive value - direct comparison
			if !subStrategyValuesEqual(currentVal, defaultVal) {
				*differences = append(*differences, SubStrategyFieldComparison{
					Field:   path,
					Current: currentVal,
					Default: defaultVal,
				})
			}
		}
	}
}

// subStrategyValuesEqual compares two interface{} values accounting for numeric type differences
func subStrategyValuesEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert to JSON and compare (handles numeric type differences)
	aJSON, err1 := json.Marshal(a)
	bJSON, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}
