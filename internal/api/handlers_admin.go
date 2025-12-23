package api

import (
	"net/http"
	"strconv"
	"time"

	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/license"

	"github.com/gin-gonic/gin"
)

// Admin middleware - requires admin role
func (s *Server) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.isUserAdmin(c) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "FORBIDDEN",
				"message": "Admin access required",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// handleAdminGenerateLicense generates a new license key
func (s *Server) handleAdminGenerateLicense(c *gin.Context) {
	var req struct {
		Type          string  `json:"type" binding:"required"`
		CustomerEmail string  `json:"customer_email"`
		CustomerName  string  `json:"customer_name"`
		ExpiresIn     int     `json:"expires_in_days"` // 0 = no expiry
		Notes         string  `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Validate type
	var licenseType license.LicenseType
	switch req.Type {
	case "personal":
		licenseType = license.LicenseTypePersonal
	case "pro":
		licenseType = license.LicenseTypePro
	case "enterprise":
		licenseType = license.LicenseTypeEnterprise
	default:
		errorResponse(c, http.StatusBadRequest, "Invalid license type. Must be: personal, pro, or enterprise")
		return
	}

	// Generate key
	key := license.GenerateLicenseKey(licenseType)

	// Get features and max symbols for this type
	features, maxSymbols := getLicenseFeatures(licenseType)

	// Create database record
	dbLicense := &database.License{
		Key:           key,
		Type:          string(licenseType),
		CustomerEmail: req.CustomerEmail,
		CustomerName:  req.CustomerName,
		MaxSymbols:    maxSymbols,
		Features:      database.FeaturesToJSON(features),
		IsActive:      true,
		Notes:         req.Notes,
	}

	if req.ExpiresIn > 0 {
		expires := time.Now().AddDate(0, 0, req.ExpiresIn)
		dbLicense.ExpiresAt = &expires
	}

	ctx := c.Request.Context()
	if err := s.repo.CreateLicense(ctx, dbLicense); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save license: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"license": gin.H{
			"id":             dbLicense.ID,
			"key":            key,
			"type":           licenseType,
			"customer_email": req.CustomerEmail,
			"customer_name":  req.CustomerName,
			"max_symbols":    maxSymbols,
			"features":       features,
			"expires_at":     dbLicense.ExpiresAt,
			"created_at":     dbLicense.CreatedAt,
		},
	})
}

// handleAdminListLicenses lists all licenses
func (s *Server) handleAdminListLicenses(c *gin.Context) {
	licenseType := c.Query("type")
	activeOnly := c.Query("active_only") == "true"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	ctx := c.Request.Context()
	licenses, total, err := s.repo.ListLicenses(ctx, licenseType, activeOnly, limit, offset)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch licenses: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"licenses": licenses,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// handleAdminGetLicense gets a single license
func (s *Server) handleAdminGetLicense(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	license, err := s.repo.GetLicenseByID(ctx, id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch license: "+err.Error())
		return
	}

	if license == nil {
		errorResponse(c, http.StatusNotFound, "License not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"license": license,
	})
}

// handleAdminUpdateLicense updates a license
func (s *Server) handleAdminUpdateLicense(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		CustomerEmail string  `json:"customer_email"`
		CustomerName  string  `json:"customer_name"`
		IsActive      *bool   `json:"is_active"`
		ExpiresAt     *string `json:"expires_at"` // ISO format
		Notes         string  `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	ctx := c.Request.Context()
	license, err := s.repo.GetLicenseByID(ctx, id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch license: "+err.Error())
		return
	}

	if license == nil {
		errorResponse(c, http.StatusNotFound, "License not found")
		return
	}

	// Update fields
	if req.CustomerEmail != "" {
		license.CustomerEmail = req.CustomerEmail
	}
	if req.CustomerName != "" {
		license.CustomerName = req.CustomerName
	}
	if req.IsActive != nil {
		license.IsActive = *req.IsActive
	}
	if req.ExpiresAt != nil {
		expires, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err == nil {
			license.ExpiresAt = &expires
		}
	}
	if req.Notes != "" {
		license.Notes = req.Notes
	}

	if err := s.repo.UpdateLicense(ctx, license); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update license: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"license": license,
	})
}

// handleAdminDeactivateLicense deactivates a license
func (s *Server) handleAdminDeactivateLicense(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	if err := s.repo.DeactivateLicense(ctx, id); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to deactivate license: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "License deactivated",
	})
}

// handleAdminDeleteLicense deletes a license
func (s *Server) handleAdminDeleteLicense(c *gin.Context) {
	id := c.Param("id")

	ctx := c.Request.Context()
	if err := s.repo.DeleteLicense(ctx, id); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to delete license: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "License deleted",
	})
}

// handleAdminValidateLicense validates a license key
func (s *Server) handleAdminValidateLicense(c *gin.Context) {
	var req struct {
		Key string `json:"key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	validator := license.NewValidator("")
	info, err := validator.ValidateLicense(req.Key)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"valid":   false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"valid":   info.IsValid,
		"info":    info,
	})
}

// handleAdminGetLicenseStats gets license statistics
func (s *Server) handleAdminGetLicenseStats(c *gin.Context) {
	ctx := c.Request.Context()
	stats, err := s.repo.GetLicenseStats(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch stats: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"stats":   stats,
	})
}

// handleAdminBulkGenerateLicenses generates multiple licenses
func (s *Server) handleAdminBulkGenerateLicenses(c *gin.Context) {
	var req struct {
		Type      string `json:"type" binding:"required"`
		Count     int    `json:"count" binding:"required,min=1,max=100"`
		ExpiresIn int    `json:"expires_in_days"`
		Notes     string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	var licenseType license.LicenseType
	switch req.Type {
	case "personal":
		licenseType = license.LicenseTypePersonal
	case "pro":
		licenseType = license.LicenseTypePro
	case "enterprise":
		licenseType = license.LicenseTypeEnterprise
	default:
		errorResponse(c, http.StatusBadRequest, "Invalid license type")
		return
	}

	features, maxSymbols := getLicenseFeatures(licenseType)
	ctx := c.Request.Context()

	var licenses []gin.H
	for i := 0; i < req.Count; i++ {
		key := license.GenerateLicenseKey(licenseType)

		dbLicense := &database.License{
			Key:        key,
			Type:       string(licenseType),
			MaxSymbols: maxSymbols,
			Features:   database.FeaturesToJSON(features),
			IsActive:   true,
			Notes:      req.Notes,
		}

		if req.ExpiresIn > 0 {
			expires := time.Now().AddDate(0, 0, req.ExpiresIn)
			dbLicense.ExpiresAt = &expires
		}

		if err := s.repo.CreateLicense(ctx, dbLicense); err != nil {
			// Continue on error, just log it
			continue
		}

		licenses = append(licenses, gin.H{
			"id":         dbLicense.ID,
			"key":        key,
			"type":       licenseType,
			"expires_at": dbLicense.ExpiresAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"count":    len(licenses),
		"licenses": licenses,
	})
}

// Helper function to get features for license type
func getLicenseFeatures(licenseType license.LicenseType) ([]string, int) {
	switch licenseType {
	case license.LicenseTypePersonal:
		return []string{
			"spot_trading",
			"futures_trading",
			"basic_signals",
			"ai_analysis",
		}, 10
	case license.LicenseTypePro:
		return []string{
			"spot_trading",
			"futures_trading",
			"basic_signals",
			"ai_analysis",
			"ginie_autopilot",
			"advanced_signals",
			"custom_strategies",
		}, 50
	case license.LicenseTypeEnterprise:
		return []string{
			"spot_trading",
			"futures_trading",
			"basic_signals",
			"ai_analysis",
			"ginie_autopilot",
			"advanced_signals",
			"custom_strategies",
			"api_access",
			"priority_support",
			"white_label",
		}, 999
	default:
		return []string{"spot_trading", "basic_signals"}, 3
	}
}
