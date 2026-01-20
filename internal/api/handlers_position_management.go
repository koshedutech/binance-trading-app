package api

import (
	"fmt"
	"log"
	"net/http"

	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/database"

	"github.com/gin-gonic/gin"
)

// ==================== POSITION MANAGEMENT SETTINGS HANDLERS ====================
// Story 10.1: Position Decision Configuration
// These handlers manage the position_management settings in default-settings.json

// PositionManagementResponse represents the API response for position management settings
type PositionManagementResponse struct {
	DecisionMode   string                               `json:"decision_mode"`
	Classic        PositionManagementClassicResponse    `json:"classic"`
	NewEngine      PositionManagementNewEngineResponse  `json:"new_engine"`
	EfficiencyExit PositionManagementEfficiencyResponse `json:"efficiency_exit"`
	DynamicSLTP    PositionManagementDynamicSLTPResponse `json:"dynamic_sltp"`
}

// PositionManagementClassicResponse represents classic decision engine settings
type PositionManagementClassicResponse struct {
	ADXReversalThreshold  float64 `json:"adx_reversal_threshold"`
	EMAReversalPeriods    []int   `json:"ema_reversal_periods"`
	RSIOverbought         float64 `json:"rsi_overbought"`
	RSIOversold           float64 `json:"rsi_oversold"`
	ReversalConfirmations int     `json:"reversal_confirmations"`
}

// PositionManagementNewEngineResponse represents new engine settings
type PositionManagementNewEngineResponse struct {
	UseActiveStrategy    bool   `json:"use_active_strategy"`
	StrategyName         string `json:"strategy_name"`
	UseStrategyExitRules bool   `json:"use_strategy_exit_rules"`
	ExitOnRegimeChange   bool   `json:"exit_on_regime_change"`
}

// PositionManagementEfficiencyResponse represents efficiency exit settings
type PositionManagementEfficiencyResponse struct {
	Enabled                    bool `json:"enabled"`
	HistoricalWindowHours      int  `json:"historical_window_hours"`
	MinimumHoldMinutes         int  `json:"minimum_hold_minutes"`
	ConsecutiveSignalsRequired int  `json:"consecutive_signals_required"`
}

// PositionManagementDynamicSLTPResponse represents dynamic SL/TP settings
type PositionManagementDynamicSLTPResponse struct {
	Enabled         bool    `json:"enabled"`
	SLATRMultiplier float64 `json:"sl_atr_multiplier"`
	TPATRMultiplier float64 `json:"tp_atr_multiplier"`
	UpdateOnBinance bool    `json:"update_on_binance"`
}

// handleGetPositionManagement returns current position management settings for the user
// GET /api/settings/position-management
func (s *Server) handleGetPositionManagement(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	// Try cache first, fallback to DB
	var config *database.UserPositionManagement
	var err error

	if s.settingsCacheService != nil {
		config, err = s.settingsCacheService.GetPositionManagement(ctx, userID)
		if err != nil {
			log.Printf("[POSITION-MANAGEMENT] Cache error for user %s: %v, falling back to DB", userID, err)
		}
	}

	// Fallback to DB if cache miss or error
	if config == nil {
		config, err = s.repo.GetUserPositionManagement(ctx, userID)
		if err != nil {
			log.Printf("[POSITION-MANAGEMENT] DB error for user %s: %v", userID, err)
			errorResponse(c, http.StatusInternalServerError, "Failed to load position management settings")
			return
		}
	}

	// If still nil, use defaults
	if config == nil {
		config = database.DefaultUserPositionManagement()
		config.UserID = userID
	}

	// Build response in API-friendly format
	response := PositionManagementResponse{
		DecisionMode: config.DecisionMode,
		Classic: PositionManagementClassicResponse{
			ADXReversalThreshold:  config.ClassicADXReversalThreshold,
			EMAReversalPeriods:    config.ClassicEMAReversalPeriods,
			RSIOverbought:         config.ClassicRSIOverbought,
			RSIOversold:           config.ClassicRSIOversold,
			ReversalConfirmations: config.ClassicReversalConfirmations,
		},
		NewEngine: PositionManagementNewEngineResponse{
			UseActiveStrategy:    config.NewEngineUseActiveStrategy,
			StrategyName:         config.NewEngineStrategyName,
			UseStrategyExitRules: config.NewEngineUseStrategyExitRules,
			ExitOnRegimeChange:   config.NewEngineExitOnRegimeChange,
		},
		EfficiencyExit: PositionManagementEfficiencyResponse{
			Enabled:                    config.EfficiencyExitEnabled,
			HistoricalWindowHours:      config.EfficiencyExitHistoricalWindowHours,
			MinimumHoldMinutes:         config.EfficiencyExitMinimumHoldMinutes,
			ConsecutiveSignalsRequired: config.EfficiencyExitConsecutiveSignalsRequired,
		},
		DynamicSLTP: PositionManagementDynamicSLTPResponse{
			Enabled:         config.DynamicSLTPEnabled,
			SLATRMultiplier: config.DynamicSLTPSLATRMultiplier,
			TPATRMultiplier: config.DynamicSLTPTPATRMultiplier,
			UpdateOnBinance: config.DynamicSLTPUpdateOnBinance,
		},
	}

	c.JSON(http.StatusOK, response)
}

// handleUpdatePositionManagement updates position management settings
// PUT /api/settings/position-management
// Body can update any combination of fields
func (s *Server) handleUpdatePositionManagement(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := c.Request.Context()

	// Get current settings
	var current *database.UserPositionManagement
	var err error

	if s.settingsCacheService != nil {
		current, err = s.settingsCacheService.GetPositionManagement(ctx, userID)
		if err != nil && !IsCacheUnavailableError(err) {
			log.Printf("[POSITION-MANAGEMENT] Cache error for user %s: %v, falling back to DB", userID, err)
		}
	}

	if current == nil {
		current, err = s.repo.GetUserPositionManagement(ctx, userID)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to get position management settings: %v", err))
			return
		}
	}

	// If no config exists, create default
	if current == nil {
		current = database.DefaultUserPositionManagement()
		current.UserID = userID
	}

	// Update fields from request
	updated := false
	for key, value := range req {
		switch key {
		case "decision_mode":
			if v, ok := value.(string); ok && (v == "classic" || v == "new_engine") {
				current.DecisionMode = v
				updated = true
			}
		case "classic":
			if v, ok := value.(map[string]interface{}); ok {
				updated = s.updateClassicSettings(current, v) || updated
			}
		case "new_engine":
			if v, ok := value.(map[string]interface{}); ok {
				updated = s.updateNewEngineSettings(current, v) || updated
			}
		case "efficiency_exit":
			if v, ok := value.(map[string]interface{}); ok {
				updated = s.updateEfficiencyExitSettings(current, v) || updated
			}
		case "dynamic_sltp":
			if v, ok := value.(map[string]interface{}); ok {
				updated = s.updateDynamicSLTPSettings(current, v) || updated
			}
		}
	}

	if !updated {
		errorResponse(c, http.StatusBadRequest, "No valid fields to update")
		return
	}

	// Save to DB (write-through cache)
	if s.settingsCacheService != nil {
		if err := s.settingsCacheService.UpdatePositionManagement(ctx, userID, current); err != nil {
			log.Printf("[POSITION-MANAGEMENT] Failed to save via cache service for user %s: %v", userID, err)
			errorResponse(c, http.StatusInternalServerError, "Failed to save position management settings")
			return
		}
	} else {
		if err := s.repo.SaveUserPositionManagement(ctx, current); err != nil {
			log.Printf("[POSITION-MANAGEMENT] Failed to save to DB for user %s: %v", userID, err)
			errorResponse(c, http.StatusInternalServerError, "Failed to save position management settings")
			return
		}
	}

	log.Printf("[POSITION-MANAGEMENT] Updated settings for user %s", userID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Position management settings updated",
	})
}

// handleLoadPositionManagementDefaults loads default position management settings
// POST /api/settings/position-management/load-defaults?preview=true
func (s *Server) handleLoadPositionManagementDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()

	// Get default settings
	defaults := autopilot.DefaultPositionManagementConfig()

	if preview {
		// Get current settings for comparison
		var current *database.UserPositionManagement
		var err error

		if s.settingsCacheService != nil {
			current, err = s.settingsCacheService.GetPositionManagement(ctx, userID)
		}
		if current == nil {
			current, err = s.repo.GetUserPositionManagement(ctx, userID)
		}
		if err != nil {
			log.Printf("[POSITION-MANAGEMENT] Error getting current settings for user %s: %v", userID, err)
		}
		if current == nil {
			current = database.DefaultUserPositionManagement()
		}

		// Build comparison response
		differences := s.comparePositionManagementSettings(current, defaults)

		c.JSON(http.StatusOK, gin.H{
			"preview":       true,
			"config_type":   "position_management",
			"total_changes": len(differences),
			"all_match":     len(differences) == 0,
			"differences":   differences,
			"default_value": defaults,
		})
		return
	}

	// Apply defaults
	newConfig := &database.UserPositionManagement{
		UserID:                                   userID,
		DecisionMode:                             defaults.DecisionMode,
		ClassicADXReversalThreshold:              defaults.Classic.ADXReversalThreshold,
		ClassicEMAReversalPeriods:                defaults.Classic.EMAReversalPeriods,
		ClassicRSIOverbought:                     defaults.Classic.RSIOverbought,
		ClassicRSIOversold:                       defaults.Classic.RSIOversold,
		ClassicReversalConfirmations:             defaults.Classic.ReversalConfirmations,
		NewEngineUseActiveStrategy:               defaults.NewEngine.UseActiveStrategy,
		NewEngineStrategyName:                    defaults.NewEngine.StrategyName,
		NewEngineUseStrategyExitRules:            defaults.NewEngine.UseStrategyExitRules,
		NewEngineExitOnRegimeChange:              defaults.NewEngine.ExitOnRegimeChange,
		EfficiencyExitEnabled:                    defaults.EfficiencyExit.Enabled,
		EfficiencyExitHistoricalWindowHours:      defaults.EfficiencyExit.HistoricalWindowHours,
		EfficiencyExitMinimumHoldMinutes:         defaults.EfficiencyExit.MinimumHoldMinutes,
		EfficiencyExitConsecutiveSignalsRequired: defaults.EfficiencyExit.ConsecutiveSignalsRequired,
		DynamicSLTPEnabled:                       defaults.DynamicSLTP.Enabled,
		DynamicSLTPSLATRMultiplier:               defaults.DynamicSLTP.SLATRMultiplier,
		DynamicSLTPTPATRMultiplier:               defaults.DynamicSLTP.TPATRMultiplier,
		DynamicSLTPUpdateOnBinance:               defaults.DynamicSLTP.UpdateOnBinance,
	}

	// Save to DB
	if s.settingsCacheService != nil {
		if err := s.settingsCacheService.UpdatePositionManagement(ctx, userID, newConfig); err != nil {
			log.Printf("[POSITION-MANAGEMENT] Failed to save defaults for user %s: %v", userID, err)
			errorResponse(c, http.StatusInternalServerError, "Failed to save position management defaults")
			return
		}
	} else {
		if err := s.repo.SaveUserPositionManagement(ctx, newConfig); err != nil {
			log.Printf("[POSITION-MANAGEMENT] Failed to save defaults to DB for user %s: %v", userID, err)
			errorResponse(c, http.StatusInternalServerError, "Failed to save position management defaults")
			return
		}
	}

	log.Printf("[POSITION-MANAGEMENT] Reset settings to defaults for user %s", userID)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"config_type":     "position_management",
		"changes_applied": "all",
		"message":         "Position management settings reset to defaults",
	})
}

// ==================== HELPER METHODS ====================

// updateClassicSettings updates classic decision engine fields
func (s *Server) updateClassicSettings(config *database.UserPositionManagement, values map[string]interface{}) bool {
	updated := false
	if v, ok := values["adx_reversal_threshold"].(float64); ok {
		config.ClassicADXReversalThreshold = v
		updated = true
	}
	if v, ok := values["ema_reversal_periods"]; ok {
		if arr, ok := v.([]interface{}); ok {
			periods := make([]int, 0, len(arr))
			for _, item := range arr {
				if num, ok := item.(float64); ok {
					periods = append(periods, int(num))
				}
			}
			if len(periods) > 0 {
				config.ClassicEMAReversalPeriods = periods
				updated = true
			}
		}
	}
	if v, ok := values["rsi_overbought"].(float64); ok {
		config.ClassicRSIOverbought = v
		updated = true
	}
	if v, ok := values["rsi_oversold"].(float64); ok {
		config.ClassicRSIOversold = v
		updated = true
	}
	if v, ok := values["reversal_confirmations"].(float64); ok {
		config.ClassicReversalConfirmations = int(v)
		updated = true
	}
	return updated
}

// updateNewEngineSettings updates new engine fields
func (s *Server) updateNewEngineSettings(config *database.UserPositionManagement, values map[string]interface{}) bool {
	updated := false
	if v, ok := values["use_active_strategy"].(bool); ok {
		config.NewEngineUseActiveStrategy = v
		updated = true
	}
	if v, ok := values["strategy_name"].(string); ok {
		config.NewEngineStrategyName = v
		updated = true
	}
	if v, ok := values["use_strategy_exit_rules"].(bool); ok {
		config.NewEngineUseStrategyExitRules = v
		updated = true
	}
	if v, ok := values["exit_on_regime_change"].(bool); ok {
		config.NewEngineExitOnRegimeChange = v
		updated = true
	}
	return updated
}

// updateEfficiencyExitSettings updates efficiency exit fields
func (s *Server) updateEfficiencyExitSettings(config *database.UserPositionManagement, values map[string]interface{}) bool {
	updated := false
	if v, ok := values["enabled"].(bool); ok {
		config.EfficiencyExitEnabled = v
		updated = true
	}
	if v, ok := values["historical_window_hours"].(float64); ok {
		config.EfficiencyExitHistoricalWindowHours = int(v)
		updated = true
	}
	if v, ok := values["minimum_hold_minutes"].(float64); ok {
		config.EfficiencyExitMinimumHoldMinutes = int(v)
		updated = true
	}
	if v, ok := values["consecutive_signals_required"].(float64); ok {
		config.EfficiencyExitConsecutiveSignalsRequired = int(v)
		updated = true
	}
	return updated
}

// updateDynamicSLTPSettings updates dynamic SL/TP fields
func (s *Server) updateDynamicSLTPSettings(config *database.UserPositionManagement, values map[string]interface{}) bool {
	updated := false
	if v, ok := values["enabled"].(bool); ok {
		config.DynamicSLTPEnabled = v
		updated = true
	}
	if v, ok := values["sl_atr_multiplier"].(float64); ok {
		config.DynamicSLTPSLATRMultiplier = v
		updated = true
	}
	if v, ok := values["tp_atr_multiplier"].(float64); ok {
		config.DynamicSLTPTPATRMultiplier = v
		updated = true
	}
	if v, ok := values["update_on_binance"].(bool); ok {
		config.DynamicSLTPUpdateOnBinance = v
		updated = true
	}
	return updated
}

// comparePositionManagementSettings compares current vs default settings
func (s *Server) comparePositionManagementSettings(current *database.UserPositionManagement, defaults *autopilot.PositionManagementConfig) []map[string]interface{} {
	var differences []map[string]interface{}

	// Compare decision_mode
	if current.DecisionMode != defaults.DecisionMode {
		differences = append(differences, map[string]interface{}{
			"path":    "decision_mode",
			"current": current.DecisionMode,
			"default": defaults.DecisionMode,
		})
	}

	// Compare classic settings
	if current.ClassicADXReversalThreshold != defaults.Classic.ADXReversalThreshold {
		differences = append(differences, map[string]interface{}{
			"path":    "classic.adx_reversal_threshold",
			"current": current.ClassicADXReversalThreshold,
			"default": defaults.Classic.ADXReversalThreshold,
		})
	}
	if current.ClassicRSIOverbought != defaults.Classic.RSIOverbought {
		differences = append(differences, map[string]interface{}{
			"path":    "classic.rsi_overbought",
			"current": current.ClassicRSIOverbought,
			"default": defaults.Classic.RSIOverbought,
		})
	}
	if current.ClassicRSIOversold != defaults.Classic.RSIOversold {
		differences = append(differences, map[string]interface{}{
			"path":    "classic.rsi_oversold",
			"current": current.ClassicRSIOversold,
			"default": defaults.Classic.RSIOversold,
		})
	}
	if current.ClassicReversalConfirmations != defaults.Classic.ReversalConfirmations {
		differences = append(differences, map[string]interface{}{
			"path":    "classic.reversal_confirmations",
			"current": current.ClassicReversalConfirmations,
			"default": defaults.Classic.ReversalConfirmations,
		})
	}

	// Compare new_engine settings
	if current.NewEngineUseActiveStrategy != defaults.NewEngine.UseActiveStrategy {
		differences = append(differences, map[string]interface{}{
			"path":    "new_engine.use_active_strategy",
			"current": current.NewEngineUseActiveStrategy,
			"default": defaults.NewEngine.UseActiveStrategy,
		})
	}
	if current.NewEngineUseStrategyExitRules != defaults.NewEngine.UseStrategyExitRules {
		differences = append(differences, map[string]interface{}{
			"path":    "new_engine.use_strategy_exit_rules",
			"current": current.NewEngineUseStrategyExitRules,
			"default": defaults.NewEngine.UseStrategyExitRules,
		})
	}
	if current.NewEngineExitOnRegimeChange != defaults.NewEngine.ExitOnRegimeChange {
		differences = append(differences, map[string]interface{}{
			"path":    "new_engine.exit_on_regime_change",
			"current": current.NewEngineExitOnRegimeChange,
			"default": defaults.NewEngine.ExitOnRegimeChange,
		})
	}

	// Compare efficiency_exit settings
	if current.EfficiencyExitEnabled != defaults.EfficiencyExit.Enabled {
		differences = append(differences, map[string]interface{}{
			"path":    "efficiency_exit.enabled",
			"current": current.EfficiencyExitEnabled,
			"default": defaults.EfficiencyExit.Enabled,
		})
	}
	if current.EfficiencyExitHistoricalWindowHours != defaults.EfficiencyExit.HistoricalWindowHours {
		differences = append(differences, map[string]interface{}{
			"path":    "efficiency_exit.historical_window_hours",
			"current": current.EfficiencyExitHistoricalWindowHours,
			"default": defaults.EfficiencyExit.HistoricalWindowHours,
		})
	}
	if current.EfficiencyExitMinimumHoldMinutes != defaults.EfficiencyExit.MinimumHoldMinutes {
		differences = append(differences, map[string]interface{}{
			"path":    "efficiency_exit.minimum_hold_minutes",
			"current": current.EfficiencyExitMinimumHoldMinutes,
			"default": defaults.EfficiencyExit.MinimumHoldMinutes,
		})
	}
	if current.EfficiencyExitConsecutiveSignalsRequired != defaults.EfficiencyExit.ConsecutiveSignalsRequired {
		differences = append(differences, map[string]interface{}{
			"path":    "efficiency_exit.consecutive_signals_required",
			"current": current.EfficiencyExitConsecutiveSignalsRequired,
			"default": defaults.EfficiencyExit.ConsecutiveSignalsRequired,
		})
	}

	// Compare dynamic_sltp settings
	if current.DynamicSLTPEnabled != defaults.DynamicSLTP.Enabled {
		differences = append(differences, map[string]interface{}{
			"path":    "dynamic_sltp.enabled",
			"current": current.DynamicSLTPEnabled,
			"default": defaults.DynamicSLTP.Enabled,
		})
	}
	if current.DynamicSLTPSLATRMultiplier != defaults.DynamicSLTP.SLATRMultiplier {
		differences = append(differences, map[string]interface{}{
			"path":    "dynamic_sltp.sl_atr_multiplier",
			"current": current.DynamicSLTPSLATRMultiplier,
			"default": defaults.DynamicSLTP.SLATRMultiplier,
		})
	}
	if current.DynamicSLTPTPATRMultiplier != defaults.DynamicSLTP.TPATRMultiplier {
		differences = append(differences, map[string]interface{}{
			"path":    "dynamic_sltp.tp_atr_multiplier",
			"current": current.DynamicSLTPTPATRMultiplier,
			"default": defaults.DynamicSLTP.TPATRMultiplier,
		})
	}
	if current.DynamicSLTPUpdateOnBinance != defaults.DynamicSLTP.UpdateOnBinance {
		differences = append(differences, map[string]interface{}{
			"path":    "dynamic_sltp.update_on_binance",
			"current": current.DynamicSLTPUpdateOnBinance,
			"default": defaults.DynamicSLTP.UpdateOnBinance,
		})
	}

	return differences
}

// ==================== EXPANDED POSITION DATA HANDLER ====================
// Story 10.1: Position Card Expanded - Returns full position data for expanded UI display

// ExpandedPositionDataResponse represents the full position data for the expanded card
type ExpandedPositionDataResponse struct {
	// Basic position info
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"` // "LONG" or "SHORT"
	EntryPrice   float64 `json:"entry_price"`
	CurrentPrice float64 `json:"current_price"`
	Quantity     float64 `json:"quantity"`
	Leverage     int     `json:"leverage"`

	// P&L
	UnrealizedPnL        float64 `json:"unrealized_pnl"`
	UnrealizedPnLPercent float64 `json:"unrealized_pnl_percent"`
	ROE                  float64 `json:"roe"`

	// Stage info
	Stage               string `json:"stage"` // RISK_ZONE, BREAKEVEN, TP1, EFFICIENCY
	StageEntryTime      int64  `json:"stage_entry_time"`
	StageProgressPercent float64 `json:"stage_progress_percent"`

	// Price levels
	StopLoss       *float64 `json:"stop_loss,omitempty"`
	TakeProfit     *float64 `json:"take_profit,omitempty"`
	BreakevenPrice *float64 `json:"breakeven_price,omitempty"`
	TP1Price       *float64 `json:"tp1_price,omitempty"`
	TP2Price       *float64 `json:"tp2_price,omitempty"`
	TP3Price       *float64 `json:"tp3_price,omitempty"`

	// Decision info
	DecisionMode string `json:"decision_mode"` // "classic" or "new_engine"

	// Efficiency metrics (when in EFFICIENCY stage)
	Efficiency *EfficiencyMetricsResponse `json:"efficiency,omitempty"`

	// Classic indicator scores
	ClassicScores *ClassicIndicatorScoresResponse `json:"classic_scores,omitempty"`

	// New engine indicator scores
	NewEngineScores *NewEngineIndicatorScoresResponse `json:"new_engine_scores,omitempty"`

	// Timestamps
	EntryTime   int64 `json:"entry_time"`
	LastUpdated int64 `json:"last_updated"`
}

// EfficiencyMetricsResponse represents efficiency metrics for display
type EfficiencyMetricsResponse struct {
	PeakProfit        float64 `json:"peak_profit"`
	CurrentProfit     float64 `json:"current_profit"`
	EfficiencyPercent float64 `json:"efficiency_percent"`
	ThresholdPercent  float64 `json:"threshold_percent"`
	PeakTimestamp     int64   `json:"peak_timestamp"`
}

// ClassicIndicatorScoresResponse represents classic indicator scores
type ClassicIndicatorScoresResponse struct {
	ADX              float64 `json:"adx"`
	ADXThreshold     float64 `json:"adx_threshold"`
	RSI              float64 `json:"rsi"`
	RSIState         string  `json:"rsi_state"` // "oversold", "normal", "overbought"
	ReversalSignals  int     `json:"reversal_signals"`
	ReversalRequired int     `json:"reversal_required"`
}

// NewEngineIndicatorScoresResponse represents new engine indicator scores
type NewEngineIndicatorScoresResponse struct {
	Technical float64 `json:"technical"`
	Context   float64 `json:"context"`
	LLM       float64 `json:"llm"`
	History   float64 `json:"history"`
	Final     float64 `json:"final"`
	Regime    string  `json:"regime"`
	Strategy  string  `json:"strategy"`
}

// handleGetExpandedPositionData returns full position data for the expanded card
// GET /api/futures/positions/:symbol/expanded
func (s *Server) handleGetExpandedPositionData(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	// Get user's autopilot instance
	if s.userAutopilotManager == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot manager not initialized")
		return
	}

	// Get positions from autopilot manager
	positions := s.userAutopilotManager.GetPositions(userID)
	if len(positions) == 0 {
		errorResponse(c, http.StatusNotFound, "No positions found for user")
		return
	}

	// Find the specific position
	var position *autopilot.GiniePosition
	for _, pos := range positions {
		if pos.Symbol == symbol {
			position = pos
			break
		}
	}

	if position == nil {
		errorResponse(c, http.StatusNotFound, fmt.Sprintf("Position not found for symbol: %s", symbol))
		return
	}

	// Get current price from position or market data
	currentPrice := position.HighestPrice // Use tracked price
	if currentPrice == 0 {
		currentPrice = position.EntryPrice
	}

	// Calculate P&L values
	var unrealizedPnL, unrealizedPnLPercent, roe float64
	if position.EntryPrice > 0 && position.RemainingQty > 0 {
		if position.Side == "LONG" {
			unrealizedPnL = (currentPrice - position.EntryPrice) * position.RemainingQty
		} else {
			unrealizedPnL = (position.EntryPrice - currentPrice) * position.RemainingQty
		}
		unrealizedPnLPercent = ((currentPrice - position.EntryPrice) / position.EntryPrice) * 100
		if position.Side == "SHORT" {
			unrealizedPnLPercent = -unrealizedPnLPercent
		}
		// ROE includes leverage
		roe = unrealizedPnLPercent * float64(position.Leverage)
	}

	// Map stage to frontend format
	stage := mapStageToFrontend(position.Stage)

	// Build response
	response := ExpandedPositionDataResponse{
		Symbol:               symbol,
		Side:                 position.Side,
		EntryPrice:           position.EntryPrice,
		CurrentPrice:         currentPrice,
		Quantity:             position.RemainingQty,
		Leverage:             position.Leverage,
		UnrealizedPnL:        unrealizedPnL,
		UnrealizedPnLPercent: unrealizedPnLPercent,
		ROE:                  roe,
		Stage:                stage,
		StageEntryTime:       position.EntryTime.UnixMilli(),
		StageProgressPercent: calculateStageProgress(position),
		DecisionMode:         getDecisionMode(position),
		EntryTime:            position.EntryTime.UnixMilli(),
		LastUpdated:          position.EntryTime.UnixMilli(), // Use entry time as baseline
	}

	// Add price levels
	if position.StopLoss > 0 {
		response.StopLoss = &position.StopLoss
	}
	if position.BEPrice > 0 {
		response.BreakevenPrice = &position.BEPrice
	}

	// Add TP levels if available
	if len(position.TakeProfits) > 0 {
		if position.TakeProfits[0].Price > 0 {
			response.TP1Price = &position.TakeProfits[0].Price
		}
		if len(position.TakeProfits) > 1 && position.TakeProfits[1].Price > 0 {
			response.TP2Price = &position.TakeProfits[1].Price
		}
		if len(position.TakeProfits) > 2 && position.TakeProfits[2].Price > 0 {
			response.TP3Price = &position.TakeProfits[2].Price
		}
		// Main TP is the last one
		if len(position.TakeProfits) > 0 {
			lastTP := position.TakeProfits[len(position.TakeProfits)-1].Price
			if lastTP > 0 {
				response.TakeProfit = &lastTP
			}
		}
	}

	// Add efficiency metrics if in efficiency stage
	if position.EffActive && stage == "EFFICIENCY" {
		// Get efficiency threshold from user settings
		ctx := c.Request.Context()
		var thresholdPercent float64 = 50.0 // Default threshold
		if s.settingsCacheService != nil {
			posManagement, err := s.settingsCacheService.GetPositionManagement(ctx, userID)
			if err == nil && posManagement != nil {
				// Could add efficiency threshold to settings in the future
				thresholdPercent = 50.0 // Use default for now
			}
		}

		response.Efficiency = &EfficiencyMetricsResponse{
			PeakProfit:        position.PeakProfit,
			CurrentProfit:     position.CurrentProfit,
			EfficiencyPercent: position.Efficiency * 100,
			ThresholdPercent:  thresholdPercent,
			PeakTimestamp:     position.PeakTime.UnixMilli(),
		}
	}

	// Add indicator scores based on decision mode
	if response.DecisionMode == "classic" {
		// Get classic indicator values (these would come from the position's stored data or real-time calculation)
		response.ClassicScores = &ClassicIndicatorScoresResponse{
			ADX:              position.TrendStrength * 40, // Approximate from stored trend strength
			ADXThreshold:     20.0,                        // Default threshold
			RSI:              50.0,                        // Would need real-time RSI
			RSIState:         "normal",
			ReversalSignals:  0,
			ReversalRequired: 2,
		}
	} else if response.DecisionMode == "new_engine" {
		// Get new engine indicator values from stored scores
		response.NewEngineScores = &NewEngineIndicatorScoresResponse{
			Technical: position.TrendScore,
			Context:   position.MomentumScore,
			LLM:       0, // Would come from LLM analysis if available
			History:   position.VolumeScore,
			Final:     position.Confidence * 100,
			Regime:    position.CurrentRegime,
			Strategy:  position.EntryStrategy,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"position": response,
	})
}

// mapStageToFrontend converts internal stage names to frontend format
func mapStageToFrontend(stage string) string {
	switch stage {
	case autopilot.StageRiskZone:
		return "RISK_ZONE"
	case autopilot.StageBreakeven:
		return "BREAKEVEN"
	case autopilot.StageTP1Pending:
		return "TP1"
	case autopilot.StageEfficiency:
		return "EFFICIENCY"
	case "":
		// Default to RISK_ZONE if stage is not set
		return "RISK_ZONE"
	default:
		return stage
	}
}

// calculateStageProgress calculates the progress within the current stage
func calculateStageProgress(position *autopilot.GiniePosition) float64 {
	// Simple progress calculation based on profit vs targets
	if position.EntryPrice == 0 {
		return 0
	}

	var profitPct float64
	if position.Side == "LONG" {
		profitPct = ((position.HighestPrice - position.EntryPrice) / position.EntryPrice) * 100
	} else {
		profitPct = ((position.EntryPrice - position.LowestPrice) / position.EntryPrice) * 100
	}

	// Cap at 100%
	if profitPct > 100 {
		return 100
	}
	if profitPct < 0 {
		return 0
	}
	return profitPct
}

// getDecisionMode returns the decision mode used for this position
func getDecisionMode(position *autopilot.GiniePosition) string {
	// Check if position has stored decision mode
	if position.DecisionMode != "" {
		return position.DecisionMode
	}
	// Default to new_engine for new positions
	return "new_engine"
}
