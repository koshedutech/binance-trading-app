package apikeys

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"sync"

	"binance-trading-bot/internal/database"
)

// AdminEmail is the default admin email for fallback
const AdminEmail = "admin@binance-bot.local"

// Service provides centralized access to user API keys from the database
// This replaces the environment variable approach with user-specific keys
type Service struct {
	repo           *database.Repository
	encryptionKey  []byte
	mu             sync.RWMutex
	cachedAdminID  string
}

// NewService creates a new API key service
func NewService(repo *database.Repository) *Service {
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

	return &Service{
		repo:          repo,
		encryptionKey: key,
	}
}

// AIKeyResult contains the decrypted AI API key and provider info
type AIKeyResult struct {
	APIKey   string
	Provider database.AIProvider
	Model    string
}

// BinanceKeyResult contains the decrypted Binance API credentials
type BinanceKeyResult struct {
	APIKey    string
	SecretKey string
	IsTestnet bool
}

// GetAdminUserID returns the admin user's ID, caching for performance
func (s *Service) GetAdminUserID(ctx context.Context) (string, error) {
	s.mu.RLock()
	if s.cachedAdminID != "" {
		s.mu.RUnlock()
		return s.cachedAdminID, nil
	}
	s.mu.RUnlock()

	// Fetch admin user
	user, err := s.repo.GetUserByEmail(ctx, AdminEmail)
	if err != nil {
		return "", fmt.Errorf("failed to get admin user: %w", err)
	}
	if user == nil {
		return "", fmt.Errorf("admin user not found")
	}

	s.mu.Lock()
	s.cachedAdminID = user.ID
	s.mu.Unlock()

	return user.ID, nil
}

// GetActiveAIKey returns the active AI API key for a user
// Falls back to admin's key if userID is empty
func (s *Service) GetActiveAIKey(ctx context.Context, userID string) (*AIKeyResult, error) {
	// If no userID provided, use admin's key
	if userID == "" {
		var err error
		userID, err = s.GetAdminUserID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get admin user for AI key: %w", err)
		}
	}

	// Get all AI keys for the user
	keys, err := s.repo.GetUserAIKeys(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI keys: %w", err)
	}

	// Find an active key
	for _, key := range keys {
		if key.IsActive {
			// Decrypt the key
			decrypted, err := s.decryptKey(key.EncryptedKey)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt AI key: %w", err)
			}

			// Determine default model based on provider
			model := getDefaultModel(key.Provider)

			return &AIKeyResult{
				APIKey:   decrypted,
				Provider: key.Provider,
				Model:    model,
			}, nil
		}
	}

	return nil, fmt.Errorf("no active AI key found for user")
}

// GetActiveBinanceKey returns the active Binance API key for a user
// Falls back to admin's key if userID is empty
func (s *Service) GetActiveBinanceKey(ctx context.Context, userID string, isTestnet bool) (*BinanceKeyResult, error) {
	// If no userID provided, use admin's key
	if userID == "" {
		var err error
		userID, err = s.GetAdminUserID(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get admin user for Binance key: %w", err)
		}
	}

	// Get API key from database using repository method
	key, err := s.repo.GetActiveAPIKey(ctx, userID, "binance", isTestnet)
	if err != nil {
		return nil, fmt.Errorf("failed to get Binance API key: %w", err)
	}
	if key == nil {
		return nil, fmt.Errorf("no active Binance API key found for user")
	}

	// Check if we have encrypted keys in the database
	if key.EncryptedAPIKey == "" || key.EncryptedSecretKey == "" {
		return nil, fmt.Errorf("Binance API key not stored in database - please re-add your API key")
	}

	// Decrypt the API key and secret key
	apiKey, err := s.decryptKey(key.EncryptedAPIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt Binance API key: %w", err)
	}

	secretKey, err := s.decryptKey(key.EncryptedSecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt Binance secret key: %w", err)
	}

	return &BinanceKeyResult{
		APIKey:    apiKey,
		SecretKey: secretKey,
		IsTestnet: key.IsTestnet,
	}, nil
}

// decryptKey decrypts an AES-256-GCM encrypted key
func (s *Service) decryptKey(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(s.encryptionKey)
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

// getDefaultModel returns the default model for each AI provider
func getDefaultModel(provider database.AIProvider) string {
	switch provider {
	case database.AIProviderClaude:
		return "claude-sonnet-4-20250514"
	case database.AIProviderOpenAI:
		return "gpt-4-turbo-preview"
	case database.AIProviderDeepSeek:
		return "deepseek-chat"
	default:
		return "deepseek-chat"
	}
}

// HasActiveAIKey checks if a user has an active AI key configured
func (s *Service) HasActiveAIKey(ctx context.Context, userID string) bool {
	if userID == "" {
		var err error
		userID, err = s.GetAdminUserID(ctx)
		if err != nil {
			return false
		}
	}

	keys, err := s.repo.GetUserAIKeys(ctx, userID)
	if err != nil {
		return false
	}

	for _, key := range keys {
		if key.IsActive {
			return true
		}
	}
	return false
}

// HasActiveBinanceKey checks if a user has an active Binance key configured
func (s *Service) HasActiveBinanceKey(ctx context.Context, userID string, isTestnet bool) bool {
	if userID == "" {
		var err error
		userID, err = s.GetAdminUserID(ctx)
		if err != nil {
			return false
		}
	}

	key, err := s.repo.GetActiveAPIKey(ctx, userID, "binance", isTestnet)
	if err != nil || key == nil {
		return false
	}
	return true
}

// ClearCache clears the cached admin user ID
func (s *Service) ClearCache() {
	s.mu.Lock()
	s.cachedAdminID = ""
	s.mu.Unlock()
}
