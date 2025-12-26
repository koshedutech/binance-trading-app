package api

import (
	"net/http"
	"time"

	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/email"

	"github.com/gin-gonic/gin"
)

// ==================== REQUEST/RESPONSE TYPES ====================

// UpdateSystemSettingRequest for updating a single system setting
type UpdateSystemSettingRequest struct {
	Value       string  `json:"value" binding:"required"`
	IsEncrypted bool    `json:"is_encrypted"`
	Description string  `json:"description"`
}

// UpdateSMTPSettingsRequest for updating SMTP configuration
type UpdateSMTPSettingsRequest struct {
	Host     string `json:"smtp_host"`
	Port     string `json:"smtp_port"`
	Username string `json:"smtp_username"`
	Password string `json:"smtp_password"`
	From     string `json:"smtp_from"`
	FromName string `json:"smtp_from_name"`
	UseTLS   string `json:"smtp_use_tls"`
}

// ==================== HANDLERS ====================

// handleAdminGetAllSettings returns all system settings (admin only)
// GET /api/admin/settings
func (s *Server) handleAdminGetAllSettings(c *gin.Context) {
	ctx := c.Request.Context()

	settings, err := s.repo.GetAllSystemSettings(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch system settings: "+err.Error())
		return
	}

	// Mask encrypted values for security
	for i := range settings {
		if settings[i].IsEncrypted && settings[i].Value != "" {
			settings[i].Value = "********"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"settings": settings,
		"count":    len(settings),
	})
}

// handleAdminUpdateSetting updates a single system setting (admin only)
// PUT /api/admin/settings/:key
func (s *Server) handleAdminUpdateSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		errorResponse(c, http.StatusBadRequest, "Setting key is required")
		return
	}

	var req UpdateSystemSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()
	userID := s.getUserID(c)

	// Create or update the setting
	setting := &database.SystemSetting{
		Key:         key,
		Value:       req.Value,
		IsEncrypted: req.IsEncrypted,
		Description: req.Description,
		UpdatedAt:   time.Now(),
		UpdatedBy:   &userID,
	}

	if err := s.repo.UpsertSystemSetting(ctx, setting); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update setting: "+err.Error())
		return
	}

	// Mask encrypted value in response
	responseValue := req.Value
	if req.IsEncrypted && responseValue != "" {
		responseValue = "********"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Setting updated successfully",
		"setting": gin.H{
			"key":          key,
			"value":        responseValue,
			"is_encrypted": req.IsEncrypted,
			"description":  req.Description,
			"updated_at":   setting.UpdatedAt,
			"updated_by":   userID,
		},
	})
}

// handleAdminGetSMTPSettings returns all SMTP settings (admin only)
// GET /api/admin/settings/smtp
func (s *Server) handleAdminGetSMTPSettings(c *gin.Context) {
	ctx := c.Request.Context()

	smtpSettings, err := s.repo.GetSMTPSettings(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch SMTP settings: "+err.Error())
		return
	}

	// Mask password if present
	if password, exists := smtpSettings["smtp_password"]; exists && password != "" {
		smtpSettings["smtp_password"] = "********"
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"settings": smtpSettings,
	})
}

// handleAdminUpdateSMTPSettings updates SMTP configuration (admin only)
// PUT /api/admin/settings/smtp
func (s *Server) handleAdminUpdateSMTPSettings(c *gin.Context) {
	var req UpdateSMTPSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()
	userID := s.getUserID(c)

	// Update each SMTP setting that was provided
	settingsToUpdate := map[string]struct {
		value       string
		isEncrypted bool
		description string
	}{
		"smtp_host":      {req.Host, false, "SMTP server hostname"},
		"smtp_port":      {req.Port, false, "SMTP server port"},
		"smtp_username":  {req.Username, false, "SMTP authentication username"},
		"smtp_password":  {req.Password, true, "SMTP authentication password"},
		"smtp_from":      {req.From, false, "SMTP sender email address"},
		"smtp_from_name": {req.FromName, false, "SMTP sender display name"},
		"smtp_use_tls":   {req.UseTLS, false, "Enable TLS for SMTP connection"},
	}

	updatedSettings := make(map[string]string)
	for key, config := range settingsToUpdate {
		// Skip empty values (don't update if not provided)
		if config.value == "" {
			continue
		}

		setting := &database.SystemSetting{
			Key:         key,
			Value:       config.value,
			IsEncrypted: config.isEncrypted,
			Description: config.description,
			UpdatedAt:   time.Now(),
			UpdatedBy:   &userID,
		}

		if err := s.repo.UpsertSystemSetting(ctx, setting); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to update "+key+": "+err.Error())
			return
		}

		// Mask encrypted values in response
		if config.isEncrypted && config.value != "" {
			updatedSettings[key] = "********"
		} else {
			updatedSettings[key] = config.value
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "SMTP settings updated successfully",
		"settings": updatedSettings,
	})
}

// handleAdminDeleteSetting deletes a system setting (admin only)
// DELETE /api/admin/settings/:key
func (s *Server) handleAdminDeleteSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		errorResponse(c, http.StatusBadRequest, "Setting key is required")
		return
	}

	ctx := c.Request.Context()

	// Check if setting exists first
	_, err := s.repo.GetSystemSetting(ctx, key)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "Setting not found")
		return
	}

	// Delete the setting
	if err := s.repo.DeleteSystemSetting(ctx, key); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to delete setting: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Setting deleted successfully",
		"key":     key,
	})
}

// handleAdminGetSetting returns a single system setting (admin only)
// GET /api/admin/settings/:key
func (s *Server) handleAdminGetSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		errorResponse(c, http.StatusBadRequest, "Setting key is required")
		return
	}

	ctx := c.Request.Context()

	setting, err := s.repo.GetSystemSetting(ctx, key)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "Setting not found")
		return
	}

	// Mask encrypted value for security
	if setting.IsEncrypted && setting.Value != "" {
		setting.Value = "********"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"setting": setting,
	})
}

// TestSMTPRequest for testing SMTP configuration
type TestSMTPRequest struct {
	TestEmail string `json:"test_email" binding:"required,email"`
}

// handleAdminTestSMTP tests the SMTP configuration by sending a test email
// POST /api/admin/settings/smtp/test
func (s *Server) handleAdminTestSMTP(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// Get SMTP settings from database
	settings, err := s.repo.GetSMTPSettings(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to get SMTP settings: "+err.Error())
		return
	}

	// Check if required settings are present
	required := []string{"smtp_host", "smtp_port", "smtp_username", "smtp_password", "smtp_from"}
	for _, key := range required {
		if settings[key] == "" {
			errorResponse(c, http.StatusBadRequest, "SMTP not configured: missing "+key)
			return
		}
	}

	// Create email service and send test email
	emailSvc := email.NewService(s.repo)
	testSubject := "SMTP Test Email - Binance Trading Bot"
	testBody := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #10B981; color: white; padding: 20px; text-align: center; border-radius: 5px 5px 0 0; }
        .content { background-color: #f9fafb; padding: 30px; border-radius: 0 0 5px 5px; }
        .success { color: #10B981; font-weight: bold; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>SMTP Test Successful!</h1>
        </div>
        <div class="content">
            <p class="success">Congratulations! Your SMTP configuration is working correctly.</p>
            <p>This test email was sent from your Binance Trading Bot to verify the email settings.</p>
            <p><strong>SMTP Server:</strong> ` + settings["smtp_host"] + `</p>
            <p><strong>Port:</strong> ` + settings["smtp_port"] + `</p>
            <p><strong>From:</strong> ` + settings["smtp_from"] + `</p>
            <p>If you received this email, your email verification and notification features should work properly.</p>
        </div>
    </div>
</body>
</html>`

	err = emailSvc.SendEmail(ctx, req.TestEmail, testSubject, testBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to send test email",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Test email sent successfully to " + req.TestEmail,
	})
}
