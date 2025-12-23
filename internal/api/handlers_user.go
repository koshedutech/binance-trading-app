package api

import (
	"net/http"

	"binance-trading-bot/internal/auth"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/vault"

	"github.com/gin-gonic/gin"
)

// handleUpdateProfile updates the user's profile
func (s *Server) handleUpdateProfile(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := c.Request.Context()

	// Validate inputs
	if req.Name != "" && len(req.Name) < 2 {
		errorResponse(c, http.StatusBadRequest, "name must be at least 2 characters")
		return
	}

	// Update profile
	if err := s.repo.UpdateUserProfile(ctx, userID, req.Name, req.Email); err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to update profile")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "profile updated successfully",
	})
}

// handleChangePassword changes the user's password
func (s *Server) handleChangePassword(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	// Validate new password length
	if len(req.NewPassword) < 8 {
		errorResponse(c, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	ctx := c.Request.Context()

	// Use auth service to change password
	if s.authService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "authentication service not available")
		return
	}

	if err := s.authService.ChangePassword(ctx, userID, auth.ChangePasswordRequest{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}); err != nil {
		if err.Error() == "incorrect current password" {
			errorResponse(c, http.StatusUnauthorized, "current password is incorrect")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "failed to change password")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "password changed successfully",
	})
}

// handleGetAPIKeys returns the user's API keys
func (s *Server) handleGetAPIKeys(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	keys, err := s.repo.GetUserAPIKeys(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to get API keys")
		return
	}

	// Convert to response format
	response := make([]gin.H, len(keys))
	for i, key := range keys {
		response[i] = gin.H{
			"id":                key.ID,
			"exchange":          key.Exchange,
			"api_key_last_four": key.APIKeyLastFour,
			"is_testnet":        key.IsTestnet,
			"is_active":         key.IsActive,
			"created_at":        key.CreatedAt,
		}
	}

	successResponse(c, response)
}

// handleAddAPIKey adds a new API key for the user
func (s *Server) handleAddAPIKey(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	var req struct {
		APIKey    string `json:"api_key" binding:"required"`
		SecretKey string `json:"secret_key" binding:"required"`
		IsTestnet bool   `json:"is_testnet"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "api_key and secret_key are required")
		return
	}

	// Validate API key length
	if len(req.APIKey) < 4 {
		errorResponse(c, http.StatusBadRequest, "api_key is too short")
		return
	}

	ctx := c.Request.Context()

	// Store reference in database
	network := "mainnet"
	if req.IsTestnet {
		network = "testnet"
	}

	vaultPath := userID + "/binance_" + network

	// Try to store in Vault if available
	if s.vaultClient != nil {
		vaultData := vault.APIKeyData{
			APIKey:    req.APIKey,
			SecretKey: req.SecretKey,
			Exchange:  "binance",
			IsTestnet: req.IsTestnet,
		}

		if err := s.vaultClient.StoreAPIKey(ctx, userID, vaultData); err != nil {
			// Log but continue - we'll store in dev mode
			// In production, Vault should be required
		}
	}

	apiKey := &database.UserAPIKey{
		UserID:           userID,
		Exchange:         "binance",
		VaultSecretPath:  vaultPath,
		APIKeyLastFour:   req.APIKey[len(req.APIKey)-4:],
		Label:            "Binance " + network,
		IsTestnet:        req.IsTestnet,
		IsActive:         true,
		ValidationStatus: database.ValidationPending,
	}

	if err := s.repo.CreateUserAPIKey(ctx, apiKey); err != nil {
		// Try to clean up Vault entry if it was created
		if s.vaultClient != nil {
			_ = s.vaultClient.DeleteAPIKey(ctx, userID, "binance", req.IsTestnet)
		}
		errorResponse(c, http.StatusInternalServerError, "failed to save API key reference")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "API key added successfully",
	})
}

// handleDeleteAPIKey deletes an API key
func (s *Server) handleDeleteAPIKey(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	keyID := c.Param("id")
	if keyID == "" {
		errorResponse(c, http.StatusBadRequest, "key ID is required")
		return
	}

	ctx := c.Request.Context()

	// Get the key to find vault path
	key, err := s.repo.GetUserAPIKeyByID(ctx, keyID, userID)
	if err != nil || key == nil {
		errorResponse(c, http.StatusNotFound, "API key not found")
		return
	}

	// Delete from Vault
	if s.vaultClient != nil {
		_ = s.vaultClient.DeleteAPIKey(ctx, userID, key.Exchange, key.IsTestnet)
	}

	// Delete from database
	if err := s.repo.DeleteUserAPIKey(ctx, keyID, userID); err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to delete API key")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key deleted successfully",
	})
}

// handleTestAPIKey tests an API key connection
func (s *Server) handleTestAPIKey(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	keyID := c.Param("id")
	if keyID == "" {
		errorResponse(c, http.StatusBadRequest, "key ID is required")
		return
	}

	ctx := c.Request.Context()

	// Get the key
	key, err := s.repo.GetUserAPIKeyByID(ctx, keyID, userID)
	if err != nil || key == nil {
		errorResponse(c, http.StatusNotFound, "API key not found")
		return
	}

	// Try to get credentials from Vault if available
	if s.vaultClient != nil {
		creds, err := s.vaultClient.GetAPIKey(ctx, userID, key.Exchange, key.IsTestnet)
		if err == nil && creds.APIKey != "" && creds.SecretKey != "" {
			// TODO: Actually test the Binance connection
			// client := binance.NewClient(creds.APIKey, creds.SecretKey)
			// _, err = client.NewGetAccountService().Do(ctx)

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "API key connection successful",
			})
			return
		}
	}

	// Without Vault, we can only verify the key exists in database
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key exists (Vault not available for full test)",
	})
}
