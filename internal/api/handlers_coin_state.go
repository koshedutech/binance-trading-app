// Package api provides handlers for coin state API endpoints.
// Epic 11: Position Decision Engine - API Integration
package api

import (
	"net/http"

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
