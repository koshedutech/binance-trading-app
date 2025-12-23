package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetLicenseInfo returns the current license information
func (s *Server) handleGetLicenseInfo(c *gin.Context) {
	if s.licenseInfo == nil {
		c.JSON(http.StatusOK, gin.H{
			"type":        "trial",
			"is_valid":    true,
			"max_symbols": 3,
			"features":    []string{"spot_trading", "basic_signals"},
			"message":     "Running in trial mode",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"type":         s.licenseInfo.Type,
		"is_valid":     s.licenseInfo.IsValid,
		"valid_until":  s.licenseInfo.ValidUntil,
		"max_symbols":  s.licenseInfo.MaxSymbols,
		"features":     s.licenseInfo.Features,
		"message":      s.licenseInfo.Message,
		"offline_mode": s.licenseInfo.OfflineMode,
	})
}

// handleCheckFeature checks if a feature is available
func (s *Server) handleCheckFeature(c *gin.Context) {
	feature := c.Param("feature")

	if s.licenseInfo == nil {
		c.JSON(http.StatusOK, gin.H{
			"feature":   feature,
			"available": false,
			"message":   "No license configured",
		})
		return
	}

	available := false
	for _, f := range s.licenseInfo.Features {
		if f == feature {
			available = true
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"feature":   feature,
		"available": available,
	})
}
