// Package api provides handlers for coin state API endpoints.
// Epic 11: Position Decision Engine - API Integration
// Story 11.40: Entry Decision Engine Gap Analysis UI
package api

import (
	"log"
	"net/http"
	"sort"

	"binance-trading-bot/internal/decision"

	"github.com/gin-gonic/gin"
)

// CoinStateResponse represents a coin state in API response format
type CoinStateResponse struct {
	Symbol         string   `json:"symbol"`
	Price          float64  `json:"price"`
	Regime         string   `json:"regime"`
	ActiveStrategy string   `json:"active_strategy"`
	Decision       string   `json:"decision"`
	ADX            float64  `json:"adx"`
	ATR            float64  `json:"atr"`
	RSI            float64  `json:"rsi"`
	EMA9           float64  `json:"ema_9"`
	EMA21          float64  `json:"ema_21"`
	Trend1H        string   `json:"trend_1h"`
	Trend15M       string   `json:"trend_15m"`
	Scores         Scores   `json:"scores"`
	Blocking       Blocking `json:"blocking"`
	LastUpdated    int64    `json:"last_updated"`
}

// CoinStateWithGapsResponse extends CoinStateResponse with gap analysis data (Story 11.40)
type CoinStateWithGapsResponse struct {
	CoinStateResponse
	ScoreBreakdown   *decision.ScoreBreakdownDetailed `json:"score_breakdown"`
	BlockingWithGaps []decision.BlockingReasonWithGap `json:"blocking_with_gaps"`
	ScoreHistory     *decision.ScoreHistoryForUI      `json:"score_history"`
	OverallGap       int                              `json:"overall_gap"`       // Gap to entry threshold (positive = below, negative = above)
	ProximityRank    int                              `json:"proximity_rank"`    // 1 = closest to entry
	CanOverride      bool                             `json:"can_override"`      // True if only soft blocks
	StatusLabel      string                           `json:"status_label"`      // "Ready", "Nearly Ready", "Moderate Gap", "Needs Work", "Far"
	StatusColor      string                           `json:"status_color"`      // "green", "light-green", "yellow", "orange", "red"
}

// Scores represents the additive scoring breakdown
type Scores struct {
	Technical int `json:"technical"`
	Context   int `json:"context"`
	LLM       int `json:"llm"`
	History   int `json:"history"`
	Final     int `json:"final"`
}

// Blocking represents the blocking summary
type Blocking struct {
	TotalReasons   int              `json:"total_reasons"`
	HardBlockCount int              `json:"hard_block_count"`
	SoftBlockCount int              `json:"soft_block_count"`
	WarningCount   int              `json:"warning_count"`
	IsBlocked      bool             `json:"is_blocked"`
	CanOverride    bool             `json:"can_override"`
	AllReasons     []BlockingReason `json:"all_reasons"`
}

// BlockingReason represents a single blocking reason
type BlockingReason struct {
	Code        string      `json:"code"`
	Category    string      `json:"category"`
	Description string      `json:"description"`
	Value       interface{} `json:"value,omitempty"`
	Threshold   interface{} `json:"threshold,omitempty"`
	Timestamp   int64       `json:"timestamp"`
	Overridable bool        `json:"overridable"`
}

// OverrideEntryRequest represents the request to override soft blocks
type OverrideEntryRequest struct {
	Symbol    string `json:"symbol" binding:"required"`
	Direction string `json:"direction" binding:"required,oneof=LONG SHORT"`
	Reason    string `json:"reason"` // Optional user's reason for override
}

// getStatusLabelAndColor returns the status label and color based on gap to threshold
func getStatusLabelAndColor(gap int, decision string) (label, color string) {
	if decision == "READY" || gap <= 0 {
		return "Ready for Entry", "green"
	}
	if gap <= 5 {
		return "Nearly Ready", "light-green"
	}
	if gap <= 10 {
		return "Moderate Gap", "yellow"
	}
	if gap <= 20 {
		return "Needs Work", "orange"
	}
	return "Far from Entry", "red"
}

// convertCoinStateToResponse converts internal CoinState to API response format
func convertCoinStateToResponse(state *decision.CoinState) CoinStateResponse {
	// Parse blocking reasons
	blocking := Blocking{
		AllReasons: make([]BlockingReason, 0),
	}

	if len(state.BlockingReasons) > 0 {
		blocking.TotalReasons = len(state.BlockingReasons)
		blocking.IsBlocked = state.Decision == decision.DecisionBlocked

		for _, reason := range state.BlockingReasons {
			// Categorize blocking reasons
			category := "SOFT_BLOCK"
			overridable := true

			// Hard blocks - cannot be overridden
			hardBlocks := map[string]bool{
				"TREND_DIVERGENCE":      true,
				"ADX_TOO_LOW":           true,
				"CIRCUIT_BREAKER_ACTIVE": true,
				"REGIME_MISMATCH":       true,
				"TIMEFRAME_MISALIGN":    true,
			}

			// Warnings - informational only
			warnings := map[string]bool{
				"LOW_VOLUME":   true,
				"WIDE_SPREAD":  true,
				"HIGH_RSI":     true,
				"LOW_RSI":      true,
				"LOW_WIN_RATE": true,
			}

			if hardBlocks[reason] {
				category = "HARD_BLOCK"
				overridable = false
				blocking.HardBlockCount++
			} else if warnings[reason] {
				category = "WARNING"
				blocking.WarningCount++
			} else {
				blocking.SoftBlockCount++
			}

			blocking.AllReasons = append(blocking.AllReasons, BlockingReason{
				Code:        reason,
				Category:    category,
				Description: getBlockingReasonDescription(reason),
				Timestamp:   state.LastUpdated,
				Overridable: overridable,
			})
		}

		blocking.CanOverride = blocking.HardBlockCount == 0
	}

	return CoinStateResponse{
		Symbol:         state.Symbol,
		Price:          state.Price,
		Regime:         string(state.Regime),
		ActiveStrategy: state.ActiveStrategy,
		Decision:       string(state.Decision),
		ADX:            state.ADX,
		ATR:            state.ATR,
		RSI:            state.RSI,
		EMA9:           state.EMA9,
		EMA21:          state.EMA21,
		Trend1H:        string(state.Trend1H),
		Trend15M:       string(state.Trend15M),
		Scores: Scores{
			Technical: state.ScoreTechnical,
			Context:   state.ScoreContext,
			LLM:       state.ScoreLLM,
			History:   state.ScoreHistory,
			Final:     state.ScoreFinal,
		},
		Blocking:    blocking,
		LastUpdated: state.LastUpdated,
	}
}

// getBlockingReasonDescription returns human-readable description for blocking codes
func getBlockingReasonDescription(code string) string {
	descriptions := map[string]string{
		"TREND_DIVERGENCE":       "1H and 15M trends are not aligned",
		"ADX_TOO_LOW":            "ADX is below minimum threshold - no clear trend",
		"CIRCUIT_BREAKER_ACTIVE": "Circuit breaker is currently active",
		"REGIME_MISMATCH":        "Current market regime doesn't match strategy",
		"TIMEFRAME_MISALIGN":     "Timeframe indicators are misaligned",
		"SCORE_BELOW_THRESHOLD":  "Score is below minimum entry threshold",
		"LOW_CONFIDENCE":         "LLM confidence is too low",
		"WEAK_MOMENTUM":          "Momentum indicators show weakness",
		"LOW_VOLUME":             "Volume is below average",
		"WIDE_SPREAD":            "Bid-ask spread is wider than normal",
		"HIGH_RSI":               "RSI indicates overbought conditions",
		"LOW_RSI":                "RSI indicates oversold conditions",
		"LOW_WIN_RATE":           "Historical win rate for this setup is low",
	}

	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return code
}

// handleGetCoinState handles GET /api/futures/decision/coin/:symbol
func (s *Server) handleGetCoinState(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol is required"})
		return
	}

	// Check if state manager is available
	if s.stateManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "State manager not initialized",
			"message": "Decision engine state management is not available",
		})
		return
	}

	state, err := s.stateManager.GetCoinState(c.Request.Context(), userID.(string), symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get coin state", "details": err.Error()})
		return
	}

	if state == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Coin state not found",
			"symbol":  symbol,
			"message": "No state data available for this symbol. The decision engine may not be tracking this coin yet.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    convertCoinStateToResponse(state),
	})
}

// handleGetAllCoinStates handles GET /api/futures/decision/coins
func (s *Server) handleGetAllCoinStates(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check if state manager is available
	if s.stateManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "State manager not initialized",
			"message": "Decision engine state management is not available",
		})
		return
	}

	// Optional: filter by specific symbols
	symbolsParam := c.Query("symbols")
	var states []*decision.CoinState
	var err error

	if symbolsParam != "" {
		// Parse comma-separated symbols
		symbols := parseSymbolsList(symbolsParam)
		states, err = s.stateManager.GetCoinStates(c.Request.Context(), userID.(string), symbols)
	} else {
		states, err = s.stateManager.GetAllCoinStates(c.Request.Context(), userID.(string))
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get coin states", "details": err.Error()})
		return
	}

	// Convert to response format
	responses := make([]CoinStateResponse, 0, len(states))
	for _, state := range states {
		responses = append(responses, convertCoinStateToResponse(state))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    responses,
		"count":   len(responses),
	})
}

// handleGetCoinStateCount handles GET /api/futures/decision/coins/count
func (s *Server) handleGetCoinStateCount(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	if s.stateManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "State manager not initialized",
			"message": "Decision engine state management is not available",
		})
		return
	}

	count, err := s.stateManager.CountUserStates(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count coin states", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   count,
	})
}

// parseSymbolsList parses a comma-separated list of symbols
func parseSymbolsList(input string) []string {
	if input == "" {
		return []string{}
	}

	var symbols []string
	current := ""
	for _, ch := range input {
		if ch == ',' {
			if current != "" {
				symbols = append(symbols, current)
				current = ""
			}
		} else if ch != ' ' {
			current += string(ch)
		}
	}
	if current != "" {
		symbols = append(symbols, current)
	}
	return symbols
}

// handleGetAllCoinStatesWithGaps handles GET /api/futures/decision/coins/gaps
// Returns coin states with detailed gap analysis, sorted by proximity to entry threshold
func (s *Server) handleGetAllCoinStatesWithGaps(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check if state manager is available
	if s.stateManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "State manager not initialized",
			"message": "Decision engine state management is not available",
		})
		return
	}

	userIDStr := userID.(string)

	// Fetch all coin states
	states, err := s.stateManager.GetAllCoinStates(c.Request.Context(), userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get coin states", "details": err.Error()})
		return
	}

	// Convert to extended response format with gap analysis
	responses := make([]CoinStateWithGapsResponse, 0, len(states))
	threshold := 55 // Default threshold

	for _, state := range states {
		baseResponse := convertCoinStateToResponse(state)

		// Create score breakdown
		// Convert percentage scores back to actual points and distribute proportionally
		// Technical (40 max): TrendAlignment(15) + Momentum(10) + Volatility(10) + Volume(5)
		// Context (30 max): RegimeMatch(10) + TimeframeAlign(10) + BTCTrend(10)
		// History (10 max): SymbolWinRate(5) + StrategyWinRate(5)
		technicalPoints := state.ScoreTechnical * 40 / 100
		contextPoints := state.ScoreContext * 30 / 100
		llmPoints := state.ScoreLLM * 20 / 100
		historyPoints := state.ScoreHistory * 10 / 100

		// Distribute sub-components proportionally so they SUM to the parent total
		// Technical sub-components (proportional to their max weights: 15, 10, 10, 5 = 40)
		trendAlign := technicalPoints * 15 / 40
		momentum := technicalPoints * 10 / 40
		volatility := technicalPoints * 10 / 40
		volume := technicalPoints - trendAlign - momentum - volatility // Remainder to ensure exact sum

		// Context sub-components (proportional to their max weights: 10, 10, 10 = 30)
		regimeMatch := contextPoints * 10 / 30
		timeframeAlign := contextPoints * 10 / 30
		btcTrend := contextPoints - regimeMatch - timeframeAlign // Remainder to ensure exact sum

		// History sub-components (proportional to their max weights: 5, 5 = 10)
		symbolWinRate := historyPoints * 5 / 10
		strategyWinRate := historyPoints - symbolWinRate // Remainder to ensure exact sum

		scoreBreakdown := &decision.ScoreBreakdownDetailed{
			Technical:          technicalPoints,
			TechnicalMax:       40,
			TrendAlignment:     trendAlign,
			TrendAlignmentMax:  15,
			Momentum:           momentum,
			MomentumMax:        10,
			Volatility:         volatility,
			VolatilityMax:      10,
			Volume:             volume,
			VolumeMax:          5,
			Context:            contextPoints,
			ContextMax:         30,
			RegimeMatch:        regimeMatch,
			RegimeMatchMax:     10,
			TimeframeAlign:     timeframeAlign,
			TimeframeAlignMax:  10,
			BTCTrend:           btcTrend,
			BTCTrendMax:        10,
			LLM:                llmPoints,
			LLMMax:             20,
			History:            historyPoints,
			HistoryMax:         10,
			SymbolWinRate:      symbolWinRate,
			SymbolWinRateMax:   5,
			StrategyWinRate:    strategyWinRate,
			StrategyWinRateMax: 5,
			Final:              state.ScoreFinal,
			Threshold:          threshold,
			GapToThreshold:     threshold - state.ScoreFinal,
		}

		// Convert blocking reasons to gap format
		blockingWithGaps := make([]decision.BlockingReasonWithGap, 0)
		for _, reason := range baseResponse.Blocking.AllReasons {
			// Get target range if applicable
			rangeEnd := decision.GetBlockingReasonTargetRange(decision.BlockingReasonCode(reason.Code))

			br := &decision.BlockingReason{
				Code:        decision.BlockingReasonCode(reason.Code),
				Category:    decision.BlockingCategory(reason.Category),
				Description: reason.Description,
				Value:       reason.Value,
				Threshold:   reason.Threshold,
				Timestamp:   reason.Timestamp,
				Overridable: reason.Overridable,
			}

			withGap := decision.NewBlockingReasonWithGap(br, rangeEnd)
			if withGap != nil {
				blockingWithGaps = append(blockingWithGaps, *withGap)
			}
		}

		// Get score history
		scoreHistory, _ := s.stateManager.GetScoreHistory(c.Request.Context(), userIDStr, state.Symbol)
		if scoreHistory == nil {
			scoreHistory = &decision.ScoreHistoryForUI{
				Timestamps: []int64{},
				Scores:     []int{},
				Trend:      "stable",
				Change8h:   0,
			}
		}

		// Calculate overall gap and status
		overallGap := threshold - state.ScoreFinal
		if overallGap < 0 {
			overallGap = 0 // Score is above threshold
		}

		canOverride := baseResponse.Blocking.HardBlockCount == 0 && baseResponse.Blocking.SoftBlockCount > 0
		statusLabel, statusColor := getStatusLabelAndColor(overallGap, string(state.Decision))

		responses = append(responses, CoinStateWithGapsResponse{
			CoinStateResponse: baseResponse,
			ScoreBreakdown:    scoreBreakdown,
			BlockingWithGaps:  blockingWithGaps,
			ScoreHistory:      scoreHistory,
			OverallGap:        overallGap,
			CanOverride:       canOverride,
			StatusLabel:       statusLabel,
			StatusColor:       statusColor,
		})
	}

	// Sort by overall gap (ascending - closest to entry first)
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].OverallGap < responses[j].OverallGap
	})

	// Set proximity rank
	for i := range responses {
		responses[i].ProximityRank = i + 1
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    responses,
		"count":   len(responses),
	})
}

// handleOverrideEntry handles POST /api/futures/decision/override
// Allows user to override soft blocks and trigger entry
func (s *Server) handleOverrideEntry(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req OverrideEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Check if state manager is available
	if s.stateManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "State manager not initialized",
			"message": "Decision engine state management is not available",
		})
		return
	}

	userIDStr := userID.(string)

	// Get current coin state
	state, err := s.stateManager.GetCoinState(c.Request.Context(), userIDStr, req.Symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get coin state", "details": err.Error()})
		return
	}

	if state == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Coin state not found",
			"symbol":  req.Symbol,
			"message": "No state data available for this symbol",
		})
		return
	}

	// Check for hard blocks
	response := convertCoinStateToResponse(state)
	if response.Blocking.HardBlockCount > 0 {
		c.JSON(http.StatusForbidden, gin.H{
			"error":            "Cannot override hard blocks",
			"hard_block_count": response.Blocking.HardBlockCount,
			"message":          "This coin has hard blocks that cannot be overridden",
			"hard_blocks":      getHardBlockReasons(response.Blocking.AllReasons),
		})
		return
	}

	// Check if there are any soft blocks to override
	if response.Blocking.SoftBlockCount == 0 && response.Decision == "READY" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "No blocks to override",
			"message": "This coin is already ready for entry - no override needed",
		})
		return
	}

	// Audit logging for override action (Story 11.40 requirement)
	// This is critical for tracking manual overrides and analyzing their outcomes
	log.Printf("[OVERRIDE] User %s overrode %d soft blocks for %s %s. Reason: %s",
		userIDStr, response.Blocking.SoftBlockCount, req.Symbol, req.Direction, req.Reason)

	// TODO: In production, this would also:
	// 1. Store override in database for historical analysis
	// 2. Trigger the actual entry through the autopilot/order system
	// 3. Return the order details

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Override request accepted",
		"symbol":    req.Symbol,
		"direction": req.Direction,
		"reason":    req.Reason,
		"overridden_blocks": response.Blocking.SoftBlockCount,
		"note": "Entry order will be placed by the autopilot system",
	})
}

// getHardBlockReasons extracts hard block reasons from the blocking list
func getHardBlockReasons(reasons []BlockingReason) []string {
	var hardBlocks []string
	for _, r := range reasons {
		if r.Category == "HARD_BLOCK" {
			hardBlocks = append(hardBlocks, r.Code+": "+r.Description)
		}
	}
	return hardBlocks
}
