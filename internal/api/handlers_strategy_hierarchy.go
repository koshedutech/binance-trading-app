package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Strategy group %s/%s updated successfully", mode, group),
		"data":    convertStrategyGroupToResponse(existing),
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
		"success":        true,
		"mode":           mode,
		"strategy_group": group,
		"sub_strategies": responses,
		"count":          len(responses),
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Sub-strategy %s/%s/%s updated successfully", mode, group, strategy),
		"data":    convertSubStrategyToResponse(existing),
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
func getDefaultStrategyGroup(mode, group, userID string) *database.StrategyGroupSettings {
	// Default settings based on mode and group
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
func getDefaultSubStrategy(mode, group, strategy, userID string) *database.SubStrategySettings {
	defaults := &database.SubStrategySettings{
		UserID:        userID,
		Mode:          mode,
		StrategyGroup: group,
		SubStrategy:   strategy,
		Enabled:       true,
	}

	// Strategy-specific default settings
	switch strategy {
	case "ravindra_volume_imbalance":
		settings := map[string]interface{}{
			"enabled":                        true,
			"min_volume_spike_multiplier":    2.0,
			"lookback_period":                20,
			"min_consolidation_candles":      2,
			"max_consolidation_candles":      6,
			"consolidation_range_tolerance":  0.01,
			"breakout_volume_surge":          1.5,
			"default_risk_reward_ratio":      4.0,
			"stop_loss_buffer":               0.001,
			"breakeven_rr_level":             2.0,
			"one_rr_level":                   3.0,
			"pattern_expiration_minutes":     60,
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

// compareSubStrategySettings compares two sub-strategy settings
func compareSubStrategySettings(current, defaults *database.SubStrategySettings) []SubStrategyFieldComparison {
	var differences []SubStrategyFieldComparison

	if current.Enabled != defaults.Enabled {
		differences = append(differences, SubStrategyFieldComparison{
			Field:   "enabled",
			Current: current.Enabled,
			Default: defaults.Enabled,
		})
	}

	// Compare settings JSON
	if string(current.Settings) != string(defaults.Settings) {
		var currentMap, defaultMap map[string]interface{}
		if err := json.Unmarshal(current.Settings, &currentMap); err != nil {
			// Return empty differences if parsing fails
			return nil
		}
		if err := json.Unmarshal(defaults.Settings, &defaultMap); err != nil {
			return nil
		}

		// Compare each field in settings
		for key, defaultVal := range defaultMap {
			currentVal, exists := currentMap[key]
			if !exists || currentVal != defaultVal {
				differences = append(differences, SubStrategyFieldComparison{
					Field:   "settings." + key,
					Current: currentVal,
					Default: defaultVal,
				})
			}
		}
	}

	return differences
}
