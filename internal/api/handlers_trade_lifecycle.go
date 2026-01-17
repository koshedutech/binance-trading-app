package api

import (
	"log"
	"net/http"
	"strconv"

	"binance-trading-bot/internal/orders"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// ==================== TRADE LIFECYCLE EVENTS ====================

// handleGetTradeLifecycleEvents returns lifecycle events for a specific trade
// GET /api/futures/trades/:tradeId/events
func (s *Server) handleGetTradeLifecycleEvents(c *gin.Context) {
	tradeIDStr := c.Param("tradeId")
	tradeID, err := strconv.ParseInt(tradeIDStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid trade ID")
		return
	}

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	events, err := db.GetTradeLifecycleEvents(c.Request.Context(), tradeID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch trade lifecycle events: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"trade_id": tradeID,
		"events":   events,
		"count":    len(events),
	})
}

// handleGetTradeLifecycleSummary returns an aggregated summary of a trade's lifecycle
// GET /api/futures/trades/:tradeId/lifecycle-summary
func (s *Server) handleGetTradeLifecycleSummary(c *gin.Context) {
	tradeIDStr := c.Param("tradeId")
	tradeID, err := strconv.ParseInt(tradeIDStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid trade ID")
		return
	}

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	summary, err := db.GetTradeLifecycleSummary(c.Request.Context(), tradeID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch trade lifecycle summary: "+err.Error())
		return
	}

	if summary == nil {
		errorResponse(c, http.StatusNotFound, "No lifecycle events found for this trade")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"summary": summary,
	})
}

// handleGetTradeLifecycleEventsByType returns events of a specific type for a trade
// GET /api/futures/trades/:tradeId/events/:eventType
func (s *Server) handleGetTradeLifecycleEventsByType(c *gin.Context) {
	tradeIDStr := c.Param("tradeId")
	tradeID, err := strconv.ParseInt(tradeIDStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid trade ID")
		return
	}

	eventType := c.Param("eventType")
	if eventType == "" {
		errorResponse(c, http.StatusBadRequest, "Event type is required")
		return
	}

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	events, err := db.GetTradeLifecycleEventsByType(c.Request.Context(), tradeID, eventType)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch trade lifecycle events: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"trade_id":   tradeID,
		"event_type": eventType,
		"events":     events,
		"count":      len(events),
	})
}

// handleGetRecentTradeLifecycleEvents returns recent lifecycle events across all trades
// GET /api/futures/trade-events/recent?limit=50
func (s *Server) handleGetRecentTradeLifecycleEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500 // Cap at 500
	}

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	events, err := db.GetRecentTradeLifecycleEvents(c.Request.Context(), limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch recent trade lifecycle events: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"events":  events,
		"count":   len(events),
		"limit":   limit,
	})
}

// handleGetTradeSLRevisionCount returns the number of SL revisions for a trade
// GET /api/futures/trades/:tradeId/sl-revisions
func (s *Server) handleGetTradeSLRevisionCount(c *gin.Context) {
	tradeIDStr := c.Param("tradeId")
	tradeID, err := strconv.ParseInt(tradeIDStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid trade ID")
		return
	}

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	count, err := db.CountSLRevisions(c.Request.Context(), tradeID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to count SL revisions: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"trade_id":     tradeID,
		"sl_revisions": count,
	})
}

// ==================== POSITION STATES (Story 7.11) ====================

// handleGetPositionStateByChainID returns position state for a specific chain
// GET /api/futures/position-states/:chainId
func (s *Server) handleGetPositionStateByChainID(c *gin.Context) {
	chainID := c.Param("chainId")
	if chainID == "" {
		errorResponse(c, http.StatusBadRequest, "Chain ID is required")
		return
	}

	// Get user ID from context
	userIDStr, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	position, err := db.GetPositionByChainID(c.Request.Context(), userID, chainID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch position state: "+err.Error())
		return
	}

	if position == nil {
		errorResponse(c, http.StatusNotFound, "No position state found for chain ID: "+chainID)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"position": position,
	})
}

// handleGetActivePositionStates returns all active position states for the user
// GET /api/futures/position-states?status=ACTIVE
func (s *Server) handleGetActivePositionStates(c *gin.Context) {
	status := c.DefaultQuery("status", orders.PositionStatusActive)

	// Validate status parameter
	validStatuses := map[string]bool{
		orders.PositionStatusActive:  true,
		orders.PositionStatusPartial: true,
		orders.PositionStatusClosed:  true,
		"":                           true, // Empty status returns all
	}
	if !validStatuses[status] {
		errorResponse(c, http.StatusBadRequest, "Invalid status. Must be ACTIVE, PARTIAL, CLOSED, or empty")
		return
	}

	// Get user ID from context
	userIDStr, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	positions, err := db.GetPositionsByUserID(c.Request.Context(), userID, status)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch position states: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"positions": positions,
		"count":     len(positions),
		"status":    status,
	})
}

// handleGetRecentPositionStates returns recent position states for the user
// GET /api/futures/position-states/recent?limit=20
func (s *Server) handleGetRecentPositionStates(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100 // Cap at 100
	}

	// Get user ID from context
	userIDStr, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	positions, err := db.GetRecentPositionStates(c.Request.Context(), userID, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch recent position states: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"positions": positions,
		"count":     len(positions),
		"limit":     limit,
	})
}

// handleGetPositionStateBySymbol returns the active position state for a symbol
// GET /api/futures/position-states/symbol/:symbol
func (s *Server) handleGetPositionStateBySymbol(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	// Get user ID from context
	userIDStr, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	position, err := db.GetPositionBySymbol(c.Request.Context(), userID, symbol, orders.PositionStatusActive)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch position state: "+err.Error())
		return
	}

	if position == nil {
		// Try partial status if no active position
		position, err = db.GetPositionBySymbol(c.Request.Context(), userID, symbol, orders.PositionStatusPartial)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to fetch position state: "+err.Error())
			return
		}
	}

	if position == nil {
		errorResponse(c, http.StatusNotFound, "No active or partial position state found for symbol: "+symbol)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"position": position,
	})
}

// ==================== ORDER MODIFICATION EVENTS (Story 7.12) ====================

// handleGetOrderModificationHistory returns modification history for an order in a chain
// GET /api/futures/trade-lifecycle/:chainId/modifications?orderType=SL
func (s *Server) handleGetOrderModificationHistory(c *gin.Context) {
	chainID := c.Param("chainId")
	if chainID == "" {
		errorResponse(c, http.StatusBadRequest, "Chain ID is required")
		return
	}

	orderType := c.Query("orderType")
	if orderType == "" {
		errorResponse(c, http.StatusBadRequest, "orderType query parameter is required (e.g., SL, TP1, TP2)")
		return
	}

	// Validate order type
	validOrderTypes := map[string]bool{
		"SL": true, "TP1": true, "TP2": true, "TP3": true, "TP4": true,
		"HSL": true, "HTP": true, // Hedge orders
	}
	if !validOrderTypes[orderType] {
		errorResponse(c, http.StatusBadRequest, "Invalid orderType. Must be SL, TP1, TP2, TP3, TP4, HSL, or HTP")
		return
	}

	// Get user ID from context for authorization
	userIDStr, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	// Verify user owns this chain before returning events
	position, err := db.GetPositionByChainID(c.Request.Context(), userID, chainID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to verify chain ownership: "+err.Error())
		return
	}
	if position == nil {
		errorResponse(c, http.StatusForbidden, "Access denied: chain does not belong to current user")
		return
	}

	events, err := db.GetModificationEvents(c.Request.Context(), chainID, orderType)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch modification events: "+err.Error())
		return
	}

	// Calculate summary statistics
	var summary *orders.ModificationSummary
	if len(events) > 0 {
		// Use a no-op logger since GetModificationSummary doesn't actually log
		noopLogger := zerolog.New(zerolog.ConsoleWriter{Out: log.Writer()}).Level(zerolog.Disabled)
		tracker := orders.NewModificationTracker(nil, noopLogger)
		summary = tracker.GetModificationSummary(events)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"chain_id":   chainID,
		"order_type": orderType,
		"events":     events,
		"count":      len(events),
		"summary":    summary,
	})
}

// handleGetChainModificationSummary returns modification summaries for all orders in a chain
// GET /api/futures/trade-lifecycle/:chainId/modifications/summary
func (s *Server) handleGetChainModificationSummary(c *gin.Context) {
	chainID := c.Param("chainId")
	if chainID == "" {
		errorResponse(c, http.StatusBadRequest, "Chain ID is required")
		return
	}

	// Get user ID from context for authorization
	userIDStr, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	// Verify user owns this chain before returning summaries
	position, err := db.GetPositionByChainID(c.Request.Context(), userID, chainID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to verify chain ownership: "+err.Error())
		return
	}
	if position == nil {
		errorResponse(c, http.StatusForbidden, "Access denied: chain does not belong to current user")
		return
	}

	summaries, err := db.GetModificationSummaryByChain(c.Request.Context(), chainID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch modification summaries: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"chain_id":  chainID,
		"summaries": summaries,
	})
}

// handleGetAllChainModifications returns all modification events for a chain (all order types)
// GET /api/futures/trade-lifecycle/:chainId/modifications/all
func (s *Server) handleGetAllChainModifications(c *gin.Context) {
	chainID := c.Param("chainId")
	if chainID == "" {
		errorResponse(c, http.StatusBadRequest, "Chain ID is required")
		return
	}

	// Get user ID from context for authorization
	userIDStr, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	// Verify user owns this chain before returning events
	position, err := db.GetPositionByChainID(c.Request.Context(), userID, chainID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to verify chain ownership: "+err.Error())
		return
	}
	if position == nil {
		errorResponse(c, http.StatusForbidden, "Access denied: chain does not belong to current user")
		return
	}

	events, err := db.GetModificationEventsByChain(c.Request.Context(), chainID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch modification events: "+err.Error())
		return
	}

	// Group events by order type for easier frontend consumption
	groupedEvents := make(map[string][]*orders.OrderModificationEvent)
	for _, event := range events {
		groupedEvents[event.OrderType] = append(groupedEvents[event.OrderType], event)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"chain_id":       chainID,
		"events":         events,
		"grouped_events": groupedEvents,
		"count":          len(events),
	})
}

// handleGetRecentModificationEvents returns recent modification events for the user
// GET /api/futures/modification-events/recent?limit=50&source=LLM_AUTO
func (s *Server) handleGetRecentModificationEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200 // Cap at 200
	}

	source := c.Query("source") // Optional filter: LLM_AUTO, USER_MANUAL, TRAILING_STOP

	// Get user ID from context
	userIDStr, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}
	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	db := s.repo.GetDB()
	if db == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	var events []*orders.OrderModificationEvent
	if source != "" {
		// Validate source
		validSources := map[string]bool{
			orders.ModificationSourceLLMAuto:      true,
			orders.ModificationSourceUserManual:   true,
			orders.ModificationSourceTrailingStop: true,
		}
		if !validSources[source] {
			errorResponse(c, http.StatusBadRequest, "Invalid source. Must be LLM_AUTO, USER_MANUAL, or TRAILING_STOP")
			return
		}
		events, err = db.GetModificationEventsBySource(c.Request.Context(), userID, source, limit)
	} else {
		events, err = db.GetModificationEventsByUser(c.Request.Context(), userID, limit)
	}

	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch modification events: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"events":  events,
		"count":   len(events),
		"limit":   limit,
		"source":  source,
	})
}
