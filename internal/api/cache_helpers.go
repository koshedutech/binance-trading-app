package api

import (
	"errors"
	"net/http"

	"binance-trading-bot/internal/cache"

	"github.com/gin-gonic/gin"
)

// Story 6.5: Cache-First Read Pattern APIs
// Helper functions for cache-related API responses

// RespondCacheUnavailable returns HTTP 503 when cache is unavailable
// This should be returned for settings read endpoints when cache is down
func RespondCacheUnavailable(c *gin.Context, operation string) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":   "cache_unavailable",
		"message": "Settings cache is temporarily unavailable. Please try again.",
		"details": operation,
	})
}

// IsCacheUnavailableError checks if the error is a cache unavailable error
func IsCacheUnavailableError(err error) bool {
	return errors.Is(err, cache.ErrCacheUnavailable)
}

// RespondSettingNotFound returns HTTP 404 when a setting is not found
func RespondSettingNotFound(c *gin.Context, setting string) {
	c.JSON(http.StatusNotFound, gin.H{
		"error":   "setting_not_found",
		"message": "The requested setting was not found",
		"setting": setting,
	})
}

// RespondCacheError handles cache errors with appropriate HTTP status
// Returns true if error was handled, false if no error
func RespondCacheError(c *gin.Context, err error, operation string) bool {
	if err == nil {
		return false
	}

	if IsCacheUnavailableError(err) {
		RespondCacheUnavailable(c, operation)
		return true
	}

	if errors.Is(err, cache.ErrSettingNotFound) {
		RespondSettingNotFound(c, operation)
		return true
	}

	// Generic cache error
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":   "cache_error",
		"message": err.Error(),
		"details": operation,
	})
	return true
}
