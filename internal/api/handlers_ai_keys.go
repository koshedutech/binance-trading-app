package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"

	"binance-trading-bot/internal/database"

	"github.com/gin-gonic/gin"
)

// encryptAPIKey encrypts an API key using AES-256-GCM
func encryptAPIKey(plaintext string) (string, error) {
	// Use encryption key from environment or a default for development
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		// Default key for development - in production this should be set via env var
		encryptionKey = "binance-trading-bot-default-encryption-key-32bytes!"
	}

	// Ensure key is 32 bytes for AES-256
	key := []byte(encryptionKey)
	if len(key) < 32 {
		// Pad the key to 32 bytes
		padding := make([]byte, 32-len(key))
		key = append(key, padding...)
	} else if len(key) > 32 {
		key = key[:32]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptAPIKey decrypts an API key using AES-256-GCM
func decryptAPIKey(ciphertext string) (string, error) {
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		encryptionKey = "binance-trading-bot-default-encryption-key-32bytes!"
	}

	key := []byte(encryptionKey)
	if len(key) < 32 {
		padding := make([]byte, 32-len(key))
		key = append(key, padding...)
	} else if len(key) > 32 {
		key = key[:32]
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// handleGetAIKeys returns the user's AI API keys (masked)
func (s *Server) handleGetAIKeys(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	keys, err := s.repo.GetUserAIKeys(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to get AI keys")
		return
	}

	// Convert to response format (never expose encrypted keys)
	response := make([]gin.H, len(keys))
	for i, key := range keys {
		response[i] = gin.H{
			"id":            key.ID,
			"provider":      key.Provider,
			"key_last_four": key.KeyLastFour,
			"is_active":     key.IsActive,
			"created_at":    key.CreatedAt,
		}
	}

	successResponse(c, response)
}

// handleAddAIKey adds or updates an AI API key for the user
func (s *Server) handleAddAIKey(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	var req struct {
		Provider string `json:"provider" binding:"required"`
		APIKey   string `json:"api_key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "provider and api_key are required")
		return
	}

	// Validate provider
	var provider database.AIProvider
	switch req.Provider {
	case "claude":
		provider = database.AIProviderClaude
	case "openai":
		provider = database.AIProviderOpenAI
	case "deepseek":
		provider = database.AIProviderDeepSeek
	default:
		errorResponse(c, http.StatusBadRequest, "invalid provider. Must be one of: claude, openai, deepseek")
		return
	}

	// Validate API key length
	if len(req.APIKey) < 10 {
		errorResponse(c, http.StatusBadRequest, "api_key is too short")
		return
	}

	ctx := c.Request.Context()

	// Encrypt the API key
	encryptedKey, err := encryptAPIKey(req.APIKey)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to encrypt API key")
		return
	}

	// Get last 4 characters for display
	lastFour := ""
	if len(req.APIKey) >= 4 {
		lastFour = req.APIKey[len(req.APIKey)-4:]
	}

	aiKey := &database.UserAIKey{
		UserID:       userID,
		Provider:     provider,
		EncryptedKey: encryptedKey,
		KeyLastFour:  lastFour,
		IsActive:     true,
	}

	if err := s.repo.CreateUserAIKey(ctx, aiKey); err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to save AI key")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "AI key added successfully",
	})
}

// handleDeleteAIKey deletes an AI API key
func (s *Server) handleDeleteAIKey(c *gin.Context) {
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

	// Delete from database
	if err := s.repo.DeleteUserAIKey(ctx, keyID, userID); err != nil {
		if err.Error() == "AI key not found or not owned by user" {
			errorResponse(c, http.StatusNotFound, "AI key not found")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "failed to delete AI key")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "AI key deleted successfully",
	})
}

// handleTestAIKey tests an AI API key connection
func (s *Server) handleTestAIKey(c *gin.Context) {
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
	key, err := s.repo.GetUserAIKeyByID(ctx, keyID, userID)
	if err != nil || key == nil {
		errorResponse(c, http.StatusNotFound, "AI key not found")
		return
	}

	// Decrypt the key
	decryptedKey, err := decryptAPIKey(key.EncryptedKey)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to decrypt AI key")
		return
	}

	// Basic validation - just check if we can decrypt it
	// In a real implementation, you might want to make a test API call to the provider
	if len(decryptedKey) < 10 {
		errorResponse(c, http.StatusBadRequest, "decrypted key is invalid")
		return
	}

	// TODO: Actually test the AI provider connection
	// For now, we just verify the key can be decrypted
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("AI key for %s is valid (basic validation)", key.Provider),
	})
}
