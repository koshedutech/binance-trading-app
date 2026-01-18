// Package api provides HTTP handlers for the Binance Trading Bot.
// Epic 11: Position Decision Engine - Story 11.14: Indicator Performance Tracker
package api

import (
	"net/http"
	"strconv"

	"binance-trading-bot/internal/decision"

	"github.com/gin-gonic/gin"
)

// handleGetIndicatorPerformanceGin returns performance statistics for a strategy.
// GET /api/futures/indicators/performance/:strategy
func (s *Server) handleGetIndicatorPerformanceGin(c *gin.Context) {
	ctx := c.Request.Context()

	// Extract user ID from context (set by auth middleware)
	userID, err := s.getUserIDFromGinContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Extract strategy from path
	strategy := c.Param("strategy")
	if strategy == "" {
		strategy = "trend_following" // Default strategy
	}

	// Check if indicator performance tracker is available
	if s.indicatorPerfTracker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":    "indicator performance tracking not configured",
			"user_id":  userID,
			"strategy": strategy,
		})
		return
	}

	// Get performance stats
	stats, err := s.indicatorPerfTracker.GetPerformanceStats(ctx, userID, strategy)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error":    err.Error(),
			"user_id":  userID,
			"strategy": strategy,
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleGetIndicatorRecommendationsGin returns recommendations based on performance data.
// GET /api/futures/indicators/recommendations/:strategy
func (s *Server) handleGetIndicatorRecommendationsGin(c *gin.Context) {
	ctx := c.Request.Context()

	// Extract user ID from context
	userID, err := s.getUserIDFromGinContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Extract strategy from path
	strategy := c.Param("strategy")
	if strategy == "" {
		strategy = "trend_following"
	}

	// Check if indicator performance tracker is available
	if s.indicatorPerfTracker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":           "indicator performance tracking not configured",
			"user_id":         userID,
			"strategy":        strategy,
			"recommendations": []decision.Recommendation{},
		})
		return
	}

	// Get recommendations
	recommendations, err := s.indicatorPerfTracker.GetRecommendations(ctx, userID, strategy)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error":           err.Error(),
			"user_id":         userID,
			"strategy":        strategy,
			"recommendations": []decision.Recommendation{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":         userID,
		"strategy":        strategy,
		"recommendations": recommendations,
		"count":           len(recommendations),
	})
}

// handleGetIndicatorCorrelationsGin returns correlations between indicators and trade outcomes.
// GET /api/futures/indicators/correlations/:strategy
func (s *Server) handleGetIndicatorCorrelationsGin(c *gin.Context) {
	ctx := c.Request.Context()

	// Extract user ID from context
	userID, err := s.getUserIDFromGinContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Extract strategy from path
	strategy := c.Param("strategy")
	if strategy == "" {
		strategy = "trend_following"
	}

	// Check if indicator performance tracker is available
	if s.indicatorPerfTracker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":        "indicator performance tracking not configured",
			"user_id":      userID,
			"strategy":     strategy,
			"correlations": map[string]interface{}{},
		})
		return
	}

	// Get correlations
	correlations, err := s.indicatorPerfTracker.GetIndicatorCorrelations(ctx, userID, strategy)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error":        err.Error(),
			"user_id":      userID,
			"strategy":     strategy,
			"correlations": map[string]interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":      userID,
		"strategy":     strategy,
		"correlations": correlations,
		"count":        len(correlations),
	})
}

// handleGetTopIndicatorCombinationsGin returns the top performing indicator combinations.
// GET /api/futures/indicators/top-combinations/:strategy
func (s *Server) handleGetTopIndicatorCombinationsGin(c *gin.Context) {
	ctx := c.Request.Context()

	// Extract user ID from context
	userID, err := s.getUserIDFromGinContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Extract strategy from path
	strategy := c.Param("strategy")
	if strategy == "" {
		strategy = "trend_following"
	}

	// Extract limit from query
	limitStr := c.Query("limit")
	limit := 10
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Check if indicator performance tracker is available
	if s.indicatorPerfTracker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":        "indicator performance tracking not configured",
			"user_id":      userID,
			"strategy":     strategy,
			"combinations": []decision.IndicatorCombinationStats{},
		})
		return
	}

	// Get top combinations
	combinations, err := s.indicatorPerfTracker.GetTopPerformingCombinations(ctx, userID, strategy, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error":        err.Error(),
			"user_id":      userID,
			"strategy":     strategy,
			"combinations": []decision.IndicatorCombinationStats{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":      userID,
		"strategy":     strategy,
		"combinations": combinations,
		"count":        len(combinations),
	})
}

// getUserIDFromGinContext extracts user ID from the Gin context.
func (s *Server) getUserIDFromGinContext(c *gin.Context) (int, error) {
	// Get user from context (set by auth middleware - uses "user_id" key)
	userIDStr := c.GetString("user_id")
	if userIDStr == "" {
		return 0, &errUnauthorizedType{}
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

// errUnauthorizedType is an error type for unauthorized requests.
type errUnauthorizedType struct{}

func (e *errUnauthorizedType) Error() string {
	return "unauthorized"
}
