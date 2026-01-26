package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"binance-trading-bot/internal/coinprofiler"
	"binance-trading-bot/internal/entrydecision"
	"binance-trading-bot/internal/events"

	"github.com/gin-gonic/gin"
)

// ==================== STORY 14.12: Entry Decision API Endpoints ====================
// API endpoints for the Entry Decision System (Epic 14: Chain Trading System)
// Provides a strategy-first view of entry opportunities across all enabled strategies.
// ===================================================================================

// ==================== REQUEST/RESPONSE TYPES ====================

// EntryDecisionStrategiesResponse is the API response for GET /entry-decision/strategies
type EntryDecisionStrategiesResponse struct {
	Strategies         []entrydecision.StrategyMatch  `json:"strategies"`
	ByMode             []entrydecision.ModeStrategies `json:"by_mode,omitempty"`
	TotalStrategies    int                            `json:"total_strategies"`
	EnabledStrategies  int                            `json:"enabled_strategies"`
	TotalCoinsReady    int                            `json:"total_coins_ready"`
	TotalCoinsWatching int                            `json:"total_coins_watching"`
	UpdatedAt          time.Time                      `json:"updated_at"`
}

// PatternProgressResponse is the API response for GET /entry-decision/pattern/:symbol
type PatternProgressResponse struct {
	Symbol      string                     `json:"symbol"`
	Mode        string                     `json:"mode"`
	Strategy    string                     `json:"strategy"`
	SubStrategy string                     `json:"sub_strategy"`
	Timeframe   string                     `json:"timeframe"`
	CurrentStep int                        `json:"current_step"`
	TotalSteps  int                        `json:"total_steps"`
	Status      string                     `json:"status"`
	StepDetails []entrydecision.StepDetail `json:"step_details"`
	Direction   string                     `json:"direction,omitempty"`
	StartedAt   time.Time                  `json:"started_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
	ExpiresAt   time.Time                  `json:"expires_at,omitempty"`
}

// ScoreBreakdownResponse is the API response for GET /entry-decision/score/:symbol
type ScoreBreakdownResponse struct {
	Symbol     string         `json:"symbol"`
	Score      int            `json:"score"`
	Direction  string         `json:"direction"`
	Ready      bool           `json:"ready"`
	Threshold  int            `json:"threshold"`
	Components map[string]int `json:"components"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// ==================== ENTRY DECISION SERVICE CACHE ====================
// Caches strategy matchers and pattern matchers per user for performance

var (
	entryDecisionMatcherCache = make(map[string]*entrydecision.DefaultStrategyMatcher)
	patternMatcherCache       = make(map[string]*entrydecision.VolumeImbalancePatternMatcher)
	scoreCalculatorCache      = make(map[string]*entrydecision.TrendFollowingScoreCalculator)
	entryDecisionCacheMu      sync.RWMutex
)

// ==================== HANDLER FUNCTIONS ====================

// handleGetEntryDecisionStrategies handles GET /api/futures/entry-decision/strategies
// Returns all strategies with matching coins (strategy-first view)
func (s *Server) handleGetEntryDecisionStrategies(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	// Get or create strategy reader using the repository
	strategyReader := s.getOrCreateStrategyReader(userID)
	if strategyReader == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Database repository not initialized",
		})
		return
	}

	// Get enabled strategies from reader
	enabledStrategies, err := strategyReader.GetEnabledStrategies(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "FETCH_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Build response manually (simpler than full matcher since we don't have coin data yet)
	response := entrydecision.NewEntryDecisionResponse()

	for _, strategy := range enabledStrategies {
		strategyType := entrydecision.GetStrategyType(strategy.StrategyGroup, strategy.SubStrategy)

		sm := entrydecision.NewStrategyMatch(
			strategy.Mode,
			strategy.StrategyGroup,
			strategy.SubStrategy,
			strategyType,
			strategy.Timeframe,
		)
		sm.Enabled = true

		// Get pattern matcher to check for tracked coins
		patternMatcher := s.getOrCreatePatternMatcher(userID)
		if patternMatcher != nil && strategyType == entrydecision.StrategyTypePattern {
			// Get all patterns for this mode/timeframe
			allPatterns := patternMatcher.GetAllPatterns()
			for _, progress := range allPatterns {
				if progress.Mode == strategy.Mode && progress.Timeframe == strategy.Timeframe {
					cm := progress.ToCoinMatch()
					sm.AddCoin(cm)
				}
			}
		}

		response.AddStrategy(*sm)
	}

	// Group by mode and calculate summary
	response.GroupByMode()
	response.CalculateSummary()

	// Convert to API response
	apiResponse := EntryDecisionStrategiesResponse{
		Strategies:         response.Strategies,
		ByMode:             response.ByMode,
		TotalStrategies:    response.TotalStrategies,
		EnabledStrategies:  response.EnabledStrategies,
		TotalCoinsReady:    response.TotalCoinsReady,
		TotalCoinsWatching: response.TotalCoinsWatching,
		UpdatedAt:          response.UpdatedAt,
	}

	c.JSON(http.StatusOK, apiResponse)
}

// handleGetEntryDecisionStrategiesForMode handles GET /api/futures/entry-decision/strategies/:mode
// Returns strategies for a specific trading mode
func (s *Server) handleGetEntryDecisionStrategiesForMode(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	mode := strings.ToLower(c.Param("mode"))
	if !entrydecision.IsValidMode(mode) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_MODE",
			"message": "Invalid trading mode. Valid modes: scalp, swing, position, ultra_fast",
		})
		return
	}

	// Get or create strategy reader using the repository
	strategyReader := s.getOrCreateStrategyReader(userID)
	if strategyReader == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Database repository not initialized",
		})
		return
	}

	// Get enabled strategies for this mode
	enabledStrategies, err := strategyReader.GetEnabledStrategiesForMode(c.Request.Context(), userID, mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "FETCH_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Build mode strategies response
	modeStrategies := entrydecision.NewModeStrategies(mode, true)

	patternMatcher := s.getOrCreatePatternMatcher(userID)

	for _, strategy := range enabledStrategies {
		strategyType := entrydecision.GetStrategyType(strategy.StrategyGroup, strategy.SubStrategy)

		sm := entrydecision.NewStrategyMatch(
			strategy.Mode,
			strategy.StrategyGroup,
			strategy.SubStrategy,
			strategyType,
			strategy.Timeframe,
		)
		sm.Enabled = true

		// Get pattern matcher to check for tracked coins
		if patternMatcher != nil && strategyType == entrydecision.StrategyTypePattern {
			allPatterns := patternMatcher.GetAllPatterns()
			for _, progress := range allPatterns {
				if progress.Mode == strategy.Mode && progress.Timeframe == strategy.Timeframe {
					cm := progress.ToCoinMatch()
					sm.AddCoin(cm)
				}
			}
		}

		modeStrategies.AddStrategy(*sm)
	}

	c.JSON(http.StatusOK, modeStrategies)
}

// handleGetEntryCandidates handles GET /api/futures/entry-decision/candidates
// Returns all coins that are ready for entry across all strategies
func (s *Server) handleGetEntryCandidates(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	// Get pattern matcher to find ready patterns
	patternMatcher := s.getOrCreatePatternMatcher(userID)

	candidates := entrydecision.NewEntryCandidatesResponse()

	// Get ready patterns from pattern matcher
	if patternMatcher != nil {
		readyPatterns := patternMatcher.GetReadyPatterns()
		for _, progress := range readyPatterns {
			candidate := entrydecision.EntryCandidate{
				Symbol:      progress.Symbol,
				Mode:        progress.Mode,
				Strategy:    progress.Strategy,
				SubStrategy: progress.SubStrategy,
				Type:        entrydecision.StrategyTypePattern,
				Timeframe:   progress.Timeframe,
				Step:        progress.CurrentStep,
				Status:      progress.Status,
				Details:     formatStepDetails(progress),
				Priority:    calculatePatternPriority(progress),
				UpdatedAt:   progress.UpdatedAt,
			}
			candidates.AddCandidate(candidate)
		}
	}

	c.JSON(http.StatusOK, candidates)
}

// handleGetPatternProgress handles GET /api/futures/entry-decision/pattern/:symbol
// Returns pattern progress for a specific coin
func (s *Server) handleGetPatternProgress(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(c.Param("symbol")))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_SYMBOL",
			"message": "Symbol is required",
		})
		return
	}

	// Optional mode and timeframe query parameters
	mode := strings.ToLower(c.Query("mode"))
	if mode == "" {
		mode = "scalp" // Default mode
	}
	if !entrydecision.IsValidMode(mode) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_MODE",
			"message": "Invalid trading mode. Valid modes: scalp, swing, position, ultra_fast",
		})
		return
	}

	timeframe := c.Query("timeframe")
	if timeframe == "" {
		timeframe = getDefaultTimeframeForMode(mode)
	}

	// Get the pattern matcher
	patternMatcher := s.getOrCreatePatternMatcher(userID)
	if patternMatcher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Pattern matcher not initialized",
		})
		return
	}

	// Get pattern progress for the symbol
	progress := patternMatcher.GetPattern(symbol, mode, timeframe)
	if progress == nil {
		// No active pattern for this symbol
		c.JSON(http.StatusOK, gin.H{
			"symbol":       symbol,
			"mode":         mode,
			"timeframe":    timeframe,
			"status":       "not_tracking",
			"message":      "No active pattern being tracked for this symbol",
			"current_step": 0,
			"total_steps":  3,
		})
		return
	}

	// Convert to API response
	response := PatternProgressResponse{
		Symbol:      progress.Symbol,
		Mode:        progress.Mode,
		Strategy:    progress.Strategy,
		SubStrategy: progress.SubStrategy,
		Timeframe:   progress.Timeframe,
		CurrentStep: progress.CurrentStep,
		TotalSteps:  progress.TotalSteps,
		Status:      string(progress.Status),
		StepDetails: progress.StepDetails,
		StartedAt:   progress.StartedAt,
		UpdatedAt:   progress.UpdatedAt,
		ExpiresAt:   progress.ExpiresAt,
	}

	c.JSON(http.StatusOK, response)
}

// handleGetScoreBreakdown handles GET /api/futures/entry-decision/score/:symbol
// Returns score breakdown for a specific coin (for score-based strategies)
func (s *Server) handleGetScoreBreakdown(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(c.Param("symbol")))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_SYMBOL",
			"message": "Symbol is required",
		})
		return
	}

	// Get or create the score calculator
	scoreCalc := s.getOrCreateScoreCalculator(userID)
	if scoreCalc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Score calculator not initialized",
		})
		return
	}

	// Get coin data for the symbol (from coin profiler if available)
	coinData := s.getCoinDataForSymbol(userID, symbol)

	// Calculate score
	result, err := scoreCalc.CalculateScore(symbol, coinData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "SCORE_CALCULATION_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Convert to API response
	response := ScoreBreakdownResponse{
		Symbol:     result.Symbol,
		Score:      result.Score,
		Direction:  result.Direction,
		Ready:      result.Ready,
		Threshold:  result.Threshold,
		Components: result.Components,
		UpdatedAt:  result.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// ==================== HELPER FUNCTIONS ====================

// getOrCreateStrategyReader returns or creates a strategy reader for a user.
func (s *Server) getOrCreateStrategyReader(userID string) entrydecision.StrategyReader {
	if s.repo == nil {
		return nil
	}
	// Create a new reader each time - it's lightweight and uses the DB directly
	return entrydecision.NewDefaultStrategyReader(s.repo)
}

// getOrCreatePatternMatcher returns or creates a pattern matcher for a user.
func (s *Server) getOrCreatePatternMatcher(userID string) *entrydecision.VolumeImbalancePatternMatcher {
	entryDecisionCacheMu.RLock()
	matcher, exists := patternMatcherCache[userID]
	entryDecisionCacheMu.RUnlock()

	if exists {
		return matcher
	}

	// Create a new pattern matcher with default config
	entryDecisionCacheMu.Lock()
	defer entryDecisionCacheMu.Unlock()

	// Double-check after acquiring write lock
	if matcher, exists = patternMatcherCache[userID]; exists {
		return matcher
	}

	matcher = entrydecision.NewVolumeImbalancePatternMatcher(nil)
	patternMatcherCache[userID] = matcher

	return matcher
}

// getOrCreateScoreCalculator returns or creates a score calculator for a user.
func (s *Server) getOrCreateScoreCalculator(userID string) *entrydecision.TrendFollowingScoreCalculator {
	entryDecisionCacheMu.RLock()
	calc, exists := scoreCalculatorCache[userID]
	entryDecisionCacheMu.RUnlock()

	if exists {
		return calc
	}

	// Create a new score calculator with default config
	entryDecisionCacheMu.Lock()
	defer entryDecisionCacheMu.Unlock()

	// Double-check after acquiring write lock
	if calc, exists = scoreCalculatorCache[userID]; exists {
		return calc
	}

	calc = entrydecision.NewTrendFollowingScoreCalculator()
	scoreCalculatorCache[userID] = calc

	return calc
}

// getCoinDataForSymbol returns coin data for a symbol from the coin profiler.
// Returns nil if coin profiler is not available or symbol not found.
func (s *Server) getCoinDataForSymbol(userID string, symbol string) *coinprofiler.CoinData {
	// For now, return nil - the score calculator handles nil gracefully
	// In a future story, this will integrate with the Coin Profiler service
	return nil
}

// getDefaultTimeframeForMode returns the default timeframe for a trading mode.
func getDefaultTimeframeForMode(mode string) string {
	switch mode {
	case "ultra_fast":
		return "1m"
	case "scalp":
		return "5m"
	case "swing":
		return "15m"
	case "position":
		return "1h"
	default:
		return "5m"
	}
}

// formatStepDetails formats step details for display.
func formatStepDetails(progress *entrydecision.PatternProgress) string {
	if progress == nil {
		return ""
	}
	if progress.CurrentStep > 0 && progress.CurrentStep <= len(progress.StepDetails) {
		step := progress.StepDetails[progress.CurrentStep-1]
		if step.Progress != "" {
			return step.Progress
		}
	}
	return ""
}

// calculatePatternPriority calculates priority for a pattern candidate.
func calculatePatternPriority(progress *entrydecision.PatternProgress) int {
	if progress == nil {
		return 0
	}
	priority := 50 // Base priority

	// Ready patterns have highest priority
	if progress.Status == entrydecision.PatternStatusReady {
		priority += 30
	}

	// Add priority based on step progress
	priority += progress.CurrentStep * 5

	// Mode-based adjustments
	switch progress.Mode {
	case "ultra_fast":
		priority += 10
	case "scalp":
		priority += 5
	}

	return priority
}

// ==================== REAL-TIME PATTERN UPDATE BROADCASTING ====================

// PatternUpdateResponse is the WebSocket message format for pattern updates.
type PatternUpdateResponse struct {
	EventType string                       `json:"event_type"`
	Data      entrydecision.PatternUpdate  `json:"data"`
	Timestamp time.Time                    `json:"timestamp"`
}

// EntryLevelsResponse is used for displaying entry levels in the UI.
type EntryLevelsResponse struct {
	Symbol          string  `json:"symbol"`
	EntryPrice      float64 `json:"entry_price"`
	StopLoss        float64 `json:"stop_loss"`
	TakeProfit      float64 `json:"take_profit"`
	RiskPercent     float64 `json:"risk_percent"`
	RewardPercent   float64 `json:"reward_percent"`
	RiskRewardRatio float64 `json:"risk_reward_ratio"`
	CurrentPrice    float64 `json:"current_price,omitempty"`
}

// PatternUpdateBroadcaster provides WebSocket broadcasting for pattern updates.
// This implements the entrydecision.PatternUpdateBroadcaster interface.
type PatternUpdateBroadcaster struct {
	server *Server
}

// NewPatternUpdateBroadcaster creates a new pattern update broadcaster.
func NewPatternUpdateBroadcaster(server *Server) *PatternUpdateBroadcaster {
	return &PatternUpdateBroadcaster{server: server}
}

// BroadcastPatternUpdate broadcasts a pattern update to all connected WebSocket clients.
func (b *PatternUpdateBroadcaster) BroadcastPatternUpdate(update entrydecision.PatternUpdate) error {
	if userWSHub == nil {
		return nil
	}

	// Create event with the pattern update data
	event := events.Event{
		Type:      events.EventEntryDecisionPatternUpdate,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol":       update.Symbol,
			"timeframe":    update.Timeframe,
			"mode":         update.Mode,
			"strategy":     update.Strategy,
			"sub_strategy": update.SubStrategy,
			"current_step": update.CurrentStep,
			"total_steps":  update.TotalSteps,
			"status":       string(update.Status),
			"step_details": update.StepDetails,
			"entry_levels": update.EntryLevels,
			"direction":    update.Direction,
			"updated_at":   update.UpdatedAt,
		},
	}

	// Broadcast to all connected clients
	userWSHub.BroadcastToAll(event)

	return nil
}

// handleGetPatternUpdates handles GET /api/futures/entry-decision/pattern-updates
// Returns all current pattern updates (for initial page load).
func (s *Server) handleGetPatternUpdates(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	// Get realtime matcher
	realtimeMatcher := s.getOrCreateRealtimeMatcher(userID)
	if realtimeMatcher == nil {
		c.JSON(http.StatusOK, gin.H{
			"updates": []interface{}{},
			"count":   0,
		})
		return
	}

	// Get all pattern updates
	updates := realtimeMatcher.GetAllPatternUpdates()

	c.JSON(http.StatusOK, gin.H{
		"updates": updates,
		"count":   len(updates),
	})
}

// getOrCreateRealtimeMatcher returns or creates a realtime pattern matcher for a user.
func (s *Server) getOrCreateRealtimeMatcher(userID string) *entrydecision.RealtimePatternMatcher {
	entryDecisionCacheMu.RLock()
	// Check if we have a cached realtime matcher
	// For now, return nil - will be initialized when CoinProfiler integration is complete
	entryDecisionCacheMu.RUnlock()

	// Get pattern matcher
	patternMatcher := s.getOrCreatePatternMatcher(userID)
	if patternMatcher == nil {
		return nil
	}

	// Create realtime matcher
	realtimeMatcher := entrydecision.NewRealtimePatternMatcher(patternMatcher, nil)

	// Wire up broadcaster
	broadcaster := NewPatternUpdateBroadcaster(s)
	realtimeMatcher.WirePatternBroadcaster(broadcaster)

	return realtimeMatcher
}

// ==================== ENTRY LEVELS ENDPOINT ====================

// handleGetEntryLevels handles GET /api/futures/entry-decision/entry-levels/:symbol
// Returns calculated entry, SL, and TP levels for a symbol.
func (s *Server) handleGetEntryLevels(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(c.Param("symbol")))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_SYMBOL",
			"message": "Symbol is required",
		})
		return
	}

	mode := strings.ToLower(c.Query("mode"))
	if mode == "" {
		mode = "scalp"
	}

	timeframe := c.Query("timeframe")
	if timeframe == "" {
		timeframe = getDefaultTimeframeForMode(mode)
	}

	// Get realtime matcher
	realtimeMatcher := s.getOrCreateRealtimeMatcher(userID)
	if realtimeMatcher == nil {
		c.JSON(http.StatusOK, gin.H{
			"symbol":  symbol,
			"message": "No active pattern for entry level calculation",
		})
		return
	}

	// Get pattern update with entry levels
	update := realtimeMatcher.GetPatternUpdate(symbol, mode, timeframe)
	if update == nil || update.EntryLevels == nil {
		c.JSON(http.StatusOK, gin.H{
			"symbol":  symbol,
			"message": "No entry levels available - pattern not at required stage",
		})
		return
	}

	response := EntryLevelsResponse{
		Symbol:          symbol,
		EntryPrice:      update.EntryLevels.EntryPrice,
		StopLoss:        update.EntryLevels.StopLoss,
		TakeProfit:      update.EntryLevels.TakeProfit,
		RiskPercent:     update.EntryLevels.RiskPercent,
		RewardPercent:   update.EntryLevels.RewardPercent,
		RiskRewardRatio: update.EntryLevels.RiskRewardRatio,
		CurrentPrice:    update.EntryLevels.CurrentPrice,
	}

	c.JSON(http.StatusOK, response)
}
