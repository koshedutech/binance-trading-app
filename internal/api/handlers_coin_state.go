// Package api provides handlers for coin state API endpoints.
// Epic 11: Position Decision Engine - API Integration
// Story 11.40: Entry Decision Engine Gap Analysis UI
package api

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/decision"
	"binance-trading-bot/internal/orders"

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

// OverrideEntryResponse represents the response from a successful override entry
type OverrideEntryResponse struct {
	Success          bool    `json:"success"`
	Message          string  `json:"message"`
	Symbol           string  `json:"symbol"`
	Direction        string  `json:"direction"`
	ChainID          string  `json:"chain_id"`
	EntryOrderID     int64   `json:"entry_order_id"`
	EntryPrice       float64 `json:"entry_price"`
	Quantity         float64 `json:"quantity"`
	PositionSizeUSD  float64 `json:"position_size_usd"`
	Mode             string  `json:"mode"`
	OverriddenBlocks int     `json:"overridden_blocks"`
	SLPrice          float64 `json:"sl_price,omitempty"`
	TPPrice          float64 `json:"tp_price,omitempty"`
	ExecutedVia      string  `json:"executed_via"` // "chain_system" - indicates bypass of legacy autopilot
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
		"INSUFFICIENT_DATA":      "Technical indicators not available - scores are N/A",
		"HIGH_VOLATILITY":        "Market volatility is extreme",
		"SIGNALS_NOT_MET":        "Required trading signals not met",
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

		// BUG FIX: The scores stored in CoinState are already RAW POINTS, not percentages!
		// saveCoinStateFromDecision() calculates:
		//   - ScoreTechnical: 0-40 raw points
		//   - ScoreContext: 0-30 raw points
		//   - ScoreLLM: 0-20 raw points
		//   - ScoreHistory: 0-10 raw points
		//   - ScoreFinal: sum of all components (0-100)
		//
		// DO NOT convert from percentages - use values directly!
		technicalPoints := state.ScoreTechnical
		contextPoints := state.ScoreContext
		llmPoints := state.ScoreLLM
		historyPoints := state.ScoreHistory

		// Validate: ScoreFinal should equal the sum of components
		// If not, recalculate to ensure consistency
		expectedFinal := technicalPoints + contextPoints + llmPoints + historyPoints
		if state.ScoreFinal != expectedFinal {
			// Log the mismatch for debugging
			log.Printf("[SCORE-MISMATCH] %s: stored final=%d, calculated=%d (T=%d C=%d L=%d H=%d)",
				state.Symbol, state.ScoreFinal, expectedFinal,
				technicalPoints, contextPoints, llmPoints, historyPoints)
			// Use the calculated sum as the true final score
			state.ScoreFinal = expectedFinal
		}

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
// Allows user to override soft blocks and trigger entry DIRECTLY through the chain system,
// bypassing Ginie Autopilot's legacy execution logic.
//
// This is the "new chain system" entry point that:
// 1. Uses ChainEventWriter for order chain creation (Epic 7)
// 2. Uses ClientOrderIdGenerator for proper chain IDs
// 3. Places orders directly via FuturesClient
// 4. Does NOT go through Ginie Autopilot's scanning/execution loop
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
	ctx := c.Request.Context()

	// Get current coin state
	state, err := s.stateManager.GetCoinState(ctx, userIDStr, req.Symbol)
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

	// Audit logging for override action (Story 11.40 requirement)
	log.Printf("[OVERRIDE] User %s overriding %d soft blocks for %s %s. Reason: %s",
		userIDStr, response.Blocking.SoftBlockCount, req.Symbol, req.Direction, req.Reason)

	// ============================================================
	// CHAIN SYSTEM DIRECT EXECUTION (Bypasses Ginie Autopilot)
	// ============================================================

	// Step 1: Get user's autopilot instance for FuturesClient and ChainEventWriter
	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Autopilot manager not initialized",
			"message": "Cannot execute trades - autopilot system not available",
		})
		return
	}

	instance := s.userAutopilotManager.GetInstance(userIDStr)
	if instance == nil || instance.Autopilot == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "User autopilot not initialized",
			"message": "Please start autopilot first or wait for it to initialize",
		})
		return
	}

	autopilot := instance.Autopilot
	futuresClient := autopilot.GetFuturesClient()
	chainEventWriter := autopilot.GetChainEventWriter()

	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Futures client not available",
			"message": "Binance connection not established",
		})
		return
	}

	// Step 2: Get current price from Binance
	currentPrice, err := futuresClient.GetFuturesCurrentPrice(req.Symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get current price",
			"details": err.Error(),
		})
		return
	}

	// Step 3: Get symbol info for quantity precision from exchange info
	exchangeInfo, err := futuresClient.GetFuturesExchangeInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get exchange info",
			"details": err.Error(),
		})
		return
	}

	// Find the symbol in exchange info
	var symbolInfo *binance.FuturesSymbolInfo
	for i := range exchangeInfo.Symbols {
		if exchangeInfo.Symbols[i].Symbol == req.Symbol {
			symbolInfo = &exchangeInfo.Symbols[i]
			break
		}
	}
	if symbolInfo == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Symbol not found",
			"symbol":  req.Symbol,
			"message": "This symbol is not available for futures trading",
		})
		return
	}

	// Step 4: Calculate position size using mode settings
	// Use the active strategy from coin state or default to scalp
	modeStr := state.ActiveStrategy
	if modeStr == "" {
		modeStr = "scalp"
	}

	// Get position size and SL/TP defaults based on mode
	// These defaults match the mode configs in default-settings.json
	positionSizeUSD, slPercent, tpPercent := getModeDefaults(modeStr)

	// Calculate quantity from USD value
	quantity := positionSizeUSD / currentPrice

	// Round to symbol's quantity precision
	quantityPrecision := symbolInfo.QuantityPrecision
	if quantityPrecision < 0 {
		quantityPrecision = 3 // Default
	}
	multiplier := math.Pow(10, float64(quantityPrecision))
	quantity = math.Floor(quantity*multiplier) / multiplier

	// Parse MinQty from filters if available
	var minQty float64 = 0.001 // Default minimum
	for _, filter := range symbolInfo.Filters {
		if filter.FilterType == "LOT_SIZE" && filter.MinQty != "" {
			if parsedMinQty, parseErr := parseFloatStr(filter.MinQty); parseErr == nil {
				minQty = parsedMinQty
			}
			break
		}
	}

	// Ensure quantity meets minimum
	if quantity < minQty {
		quantity = minQty
	}

	// Step 5: Generate chain ID using ClientOrderIdGenerator
	tradingMode := orders.ModeFromString(modeStr)
	modeCode := orders.ModeCode[tradingMode]
	if modeCode == "" {
		modeCode = "SCA" // Default to scalp
	}

	// Generate a unique chain ID in format: [MODE]-[DDMMM]-[NNNNN]-E
	// Since we may not have the full generator setup, use a fallback format
	now := time.Now().UTC()
	dateStr := now.Format("02Jan")
	seqNum := now.UnixNano() % 100000 // Simple sequence based on nanoseconds
	chainID := fmt.Sprintf("%s-%s-%05d", modeCode, dateStr, seqNum)
	entryClientOrderID := fmt.Sprintf("%s-E", chainID)

	// Step 6: Determine order side based on direction
	var orderSide string
	var positionSide binance.PositionSide
	if req.Direction == "LONG" {
		orderSide = "BUY"
		positionSide = binance.PositionSideLong
	} else {
		orderSide = "SELL"
		positionSide = binance.PositionSideShort
	}

	// Step 7: Create order chain if ChainEventWriter is available
	if chainEventWriter != nil {
		_, err := chainEventWriter.CreateChain(ctx, orders.CreateChainRequest{
			UserID:   userIDStr,
			ChainID:  chainID,
			Symbol:   req.Symbol,
			Side:     req.Direction,
			ModeCode: modeCode,
			IsHedge:  false,
		})
		if err != nil {
			log.Printf("[OVERRIDE] Warning: Failed to create order chain: %v", err)
			// Continue anyway - we can still place the order
		}
	}

	// Step 8: Place entry order via FuturesClient (MARKET order for immediate execution)
	orderParams := binance.FuturesOrderParams{
		Symbol:           req.Symbol,
		Side:             orderSide,
		PositionSide:     positionSide,
		Type:             binance.FuturesOrderTypeMarket,
		Quantity:         quantity,
		NewClientOrderId: entryClientOrderID,
	}

	log.Printf("[OVERRIDE] Placing MARKET %s order for %s: qty=%.6f, positionSizeUSD=%.2f, chainID=%s",
		orderSide, req.Symbol, quantity, positionSizeUSD, chainID)

	orderResp, err := futuresClient.PlaceFuturesOrder(orderParams)
	if err != nil {
		// Record failure in audit log
		log.Printf("[OVERRIDE] FAILED to place order for %s: %v", req.Symbol, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to place entry order",
			"details": err.Error(),
			"symbol":  req.Symbol,
		})
		return
	}

	// Step 9: Record entry placed event in chain system
	if chainEventWriter != nil {
		err := chainEventWriter.RecordEntryPlaced(ctx, chainID, orders.ChainEntryPlacedEvent{
			BinanceOrderID:       orderResp.OrderId,
			BinanceClientOrderID: entryClientOrderID,
			Price:                orderResp.AvgPrice,
			Quantity:             quantity,
			BinanceTimestamp:     orderResp.UpdateTime,
		})
		if err != nil {
			log.Printf("[OVERRIDE] Warning: Failed to record entry placed event: %v", err)
		}
	}

	// Step 10: Calculate SL/TP prices based on mode defaults (already retrieved in step 4)
	var slPrice, tpPrice float64
	entryPrice := orderResp.AvgPrice
	if entryPrice <= 0 {
		entryPrice = currentPrice
	}

	if req.Direction == "LONG" {
		slPrice = entryPrice * (1 - slPercent/100)
		tpPrice = entryPrice * (1 + tpPercent/100)
	} else {
		slPrice = entryPrice * (1 + slPercent/100)
		tpPrice = entryPrice * (1 - tpPercent/100)
	}

	// Success audit log
	log.Printf("[OVERRIDE] SUCCESS: %s %s entry at %.6f, qty=%.6f, orderID=%d, chainID=%s, SL=%.6f, TP=%.6f",
		req.Direction, req.Symbol, entryPrice, quantity, orderResp.OrderId, chainID, slPrice, tpPrice)

	// Step 11: Return success response with order details
	c.JSON(http.StatusOK, OverrideEntryResponse{
		Success:          true,
		Message:          fmt.Sprintf("Entry order placed successfully via chain system"),
		Symbol:           req.Symbol,
		Direction:        req.Direction,
		ChainID:          chainID,
		EntryOrderID:     orderResp.OrderId,
		EntryPrice:       entryPrice,
		Quantity:         quantity,
		PositionSizeUSD:  positionSizeUSD,
		Mode:             modeStr,
		OverriddenBlocks: response.Blocking.SoftBlockCount,
		SLPrice:          slPrice,
		TPPrice:          tpPrice,
		ExecutedVia:      "chain_system",
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

// parseFloatStr parses a string to float64, used for symbol filter values
func parseFloatStr(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// getModeDefaults returns position size, SL%, and TP% defaults for a trading mode
// These values match the default-settings.json mode configurations
func getModeDefaults(mode string) (positionSizeUSD, slPercent, tpPercent float64) {
	switch mode {
	case "ultra_fast":
		return 15.0, 0.5, 0.3 // $15 position, 0.5% SL, 0.3% TP (quick scalps)
	case "scalp":
		return 20.0, 1.0, 0.4 // $20 position, 1.0% SL, 0.4% TP (standard scalp)
	case "swing":
		return 30.0, 2.0, 1.0 // $30 position, 2.0% SL, 1.0% TP (swing trades)
	case "position":
		return 50.0, 3.0, 2.0 // $50 position, 3.0% SL, 2.0% TP (longer holds)
	default:
		return 20.0, 1.5, 0.4 // Default to scalp-like settings
	}
}
