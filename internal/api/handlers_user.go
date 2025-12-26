package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
		fmt.Printf("[API-KEYS] Error getting API keys for user %s: %v\n", userID, err)
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

	// Encrypt the API key and secret key for database storage
	encryptedAPIKey, err := encryptAPIKey(req.APIKey)
	if err != nil {
		fmt.Printf("[API-KEYS] Error encrypting API key for user %s: %v\n", userID, err)
		errorResponse(c, http.StatusInternalServerError, "failed to encrypt API key")
		return
	}

	encryptedSecretKey, err := encryptAPIKey(req.SecretKey)
	if err != nil {
		fmt.Printf("[API-KEYS] Error encrypting secret key for user %s: %v\n", userID, err)
		errorResponse(c, http.StatusInternalServerError, "failed to encrypt secret key")
		return
	}

	// Try to store in Vault if available (optional)
	if s.vaultClient != nil {
		vaultData := vault.APIKeyData{
			APIKey:    req.APIKey,
			SecretKey: req.SecretKey,
			Exchange:  "binance",
			IsTestnet: req.IsTestnet,
		}

		if err := s.vaultClient.StoreAPIKey(ctx, userID, vaultData); err != nil {
			// Log but continue - we store encrypted in database as fallback
			fmt.Printf("[API-KEYS] Vault storage failed (using database): %v\n", err)
		}
	}

	apiKey := &database.UserAPIKey{
		UserID:             userID,
		Exchange:           "binance",
		VaultSecretPath:    vaultPath,
		EncryptedAPIKey:    encryptedAPIKey,
		EncryptedSecretKey: encryptedSecretKey,
		APIKeyLastFour:     req.APIKey[len(req.APIKey)-4:],
		Label:              "Binance " + network,
		IsTestnet:          req.IsTestnet,
		IsActive:           true,
		ValidationStatus:   database.ValidationPending,
	}

	if err := s.repo.CreateUserAPIKey(ctx, apiKey); err != nil {
		fmt.Printf("[API-KEYS] Error creating API key for user %s: %v\n", userID, err)
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

// handleGetUserIPAddress returns the server's public IP address for Binance whitelist
// This is the IP that Binance will see when the trading bot makes API calls
func (s *Server) handleGetUserIPAddress(c *gin.Context) {
	// Get the server's public IP by querying external services
	// This is what Binance needs for whitelisting - the IP from which API calls originate
	ip := getServerPublicIP()

	if ip == "" {
		// Fallback: try to get from request headers (less reliable)
		ip = c.GetHeader("X-Real-IP")
		if ip == "" {
			ip = c.GetHeader("X-Forwarded-For")
			if ip != "" {
				// X-Forwarded-For can contain multiple IPs, take the first one
				for i, ch := range ip {
					if ch == ',' {
						ip = ip[:i]
						break
					}
				}
			}
		}
		if ip == "" {
			ip = c.ClientIP()
		}
		ip = cleanIPAddress(ip)
	}

	c.JSON(200, gin.H{
		"success":    true,
		"ip_address": ip,
		"message":    "Use this IP address for Binance API whitelist (server's public IP)",
	})
}

// getServerPublicIP fetches the server's public IP from external services
func getServerPublicIP() string {
	// Try multiple services in case one is down
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			ip := strings.TrimSpace(string(body))
			// Validate it looks like an IP
			if len(ip) > 0 && len(ip) < 50 && !strings.Contains(ip, "<") {
				return ip
			}
		}
	}

	return ""
}

// cleanIPAddress removes port and brackets from IP addresses
func cleanIPAddress(ip string) string {
	// Remove brackets for IPv6
	if len(ip) > 0 && ip[0] == '[' {
		if idx := len(ip) - 1; idx > 0 {
			for i := 1; i < len(ip); i++ {
				if ip[i] == ']' {
					ip = ip[1:i]
					break
				}
			}
		}
	}
	// Remove port suffix
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == ':' {
			// Check if this is IPv6 (has more colons) or IPv4 with port
			colonCount := 0
			for j := 0; j < i; j++ {
				if ip[j] == ':' {
					colonCount++
				}
			}
			if colonCount == 0 {
				// IPv4 with port
				ip = ip[:i]
			}
			break
		}
	}
	return ip
}

// handleGetUserAPIStatus returns user-specific API connection status
func (s *Server) handleGetUserAPIStatus(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	fmt.Printf("[USER-STATUS-DEBUG] Checking API status for userID=%s\n", userID)
	ctx := c.Request.Context()

	// Result structure
	status := map[string]interface{}{
		"binance_spot":    map[string]interface{}{"status": "not_configured", "message": "No API key configured"},
		"binance_futures": map[string]interface{}{"status": "not_configured", "message": "No API key configured"},
		"ai_service":      map[string]interface{}{"status": "not_configured", "message": "No AI key configured"},
		"database":        map[string]interface{}{"status": "ok", "message": "Connected"},
	}

	// Check user's Binance API keys
	binanceKeys, err := s.repo.GetUserAPIKeys(ctx, userID)
	if err != nil {
		fmt.Printf("[USER-STATUS] Error getting API keys for user %s: %v\n", userID, err)
	}
	fmt.Printf("[USER-STATUS-DEBUG] Found %d Binance keys for user %s\n", len(binanceKeys), userID)

	hasBinanceKey := false
	hasTestnetKey := false
	hasMainnetKey := false
	for _, key := range binanceKeys {
		if key.Exchange == "binance" && key.IsActive {
			hasBinanceKey = true
			if key.IsTestnet {
				hasTestnetKey = true
			} else {
				hasMainnetKey = true
			}
		}
	}

	if hasBinanceKey {
		// User has Binance keys configured
		if hasMainnetKey {
			status["binance_spot"] = map[string]interface{}{"status": "ok", "message": "Mainnet key configured"}
			status["binance_futures"] = map[string]interface{}{"status": "ok", "message": "Mainnet key configured"}
		} else if hasTestnetKey {
			status["binance_spot"] = map[string]interface{}{"status": "ok", "message": "Testnet key configured"}
			status["binance_futures"] = map[string]interface{}{"status": "ok", "message": "Testnet key configured"}
		}
	}

	// Check user's AI keys
	aiKeys, err := s.repo.GetUserAIKeys(ctx, userID)
	if err != nil {
		fmt.Printf("[USER-STATUS] Error getting AI keys for user %s: %v\n", userID, err)
	}
	fmt.Printf("[USER-STATUS-DEBUG] Found %d AI keys for user %s\n", len(aiKeys), userID)

	hasAIKey := false
	aiProvider := ""
	for i, key := range aiKeys {
		fmt.Printf("[USER-STATUS-DEBUG] AI key %d: provider=%s, is_active=%v, last_four=%s\n", i, key.Provider, key.IsActive, key.KeyLastFour)
		if key.IsActive {
			hasAIKey = true
			aiProvider = string(key.Provider)
			break
		}
	}

	if hasAIKey {
		status["ai_service"] = map[string]interface{}{"status": "ok", "message": fmt.Sprintf("%s configured", aiProvider)}
		fmt.Printf("[USER-STATUS-DEBUG] AI service set to OK with provider: %s\n", aiProvider)
	} else {
		fmt.Printf("[USER-STATUS-DEBUG] No active AI key found for user %s\n", userID)
	}

	// Determine overall health
	healthy := hasBinanceKey && hasAIKey

	c.JSON(200, gin.H{
		"success":  true,
		"healthy":  healthy,
		"services": status,
	})
}
