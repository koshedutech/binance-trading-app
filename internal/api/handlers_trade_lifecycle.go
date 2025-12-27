package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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
