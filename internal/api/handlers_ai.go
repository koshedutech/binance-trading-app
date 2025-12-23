package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGetAIDecisions returns AI autopilot decisions
func (s *Server) handleGetAIDecisions(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	symbol := c.Query("symbol")
	action := c.Query("action")

	ctx := context.Background()
	decisions, err := s.repo.GetAIDecisions(ctx, limit, symbol, action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    decisions,
		"count":   len(decisions),
	})
}

// handleGetAIDecisionByID returns a single AI decision by ID
func (s *Server) handleGetAIDecisionByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid decision ID"})
		return
	}

	ctx := context.Background()
	decision, err := s.repo.GetAIDecisionByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "decision not found"})
		return
	}

	c.JSON(http.StatusOK, decision)
}

// handleGetAIDecisionStats returns AI decision statistics
func (s *Server) handleGetAIDecisionStats(c *gin.Context) {
	// Parse time range (default to last 24 hours)
	hoursStr := c.DefaultQuery("hours", "24")
	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours <= 0 {
		hours = 24
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	ctx := context.Background()
	stats, err := s.repo.GetAIDecisionStats(ctx, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
		"period":  hours,
	})
}
