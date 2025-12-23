package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"binance-trading-bot/config"

	"github.com/hashicorp/vault/api"
)

// APIKeyData represents the API key data stored in Vault
type APIKeyData struct {
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
	Exchange  string `json:"exchange"`
	IsTestnet bool   `json:"is_testnet"`
}

// Client wraps the HashiCorp Vault client
type Client struct {
	client     *api.Client
	config     config.VaultConfig
	mu         sync.RWMutex
	cache      map[string]*APIKeyData // userID -> APIKeyData cache
	cacheEnabled bool
}

// NewClient creates a new Vault client
func NewClient(cfg config.VaultConfig) (*Client, error) {
	if !cfg.Enabled {
		return &Client{
			config:       cfg,
			cache:        make(map[string]*APIKeyData),
			cacheEnabled: true,
		}, nil
	}

	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = cfg.Address

	if cfg.TLSEnabled && cfg.CACert != "" {
		tlsConfig := &api.TLSConfig{
			CACert: cfg.CACert,
		}
		if err := vaultConfig.ConfigureTLS(tlsConfig); err != nil {
			return nil, fmt.Errorf("failed to configure TLS: %w", err)
		}
	}

	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	client.SetToken(cfg.Token)

	return &Client{
		client:       client,
		config:       cfg,
		cache:        make(map[string]*APIKeyData),
		cacheEnabled: true,
	}, nil
}

// StoreAPIKey stores an API key for a user in Vault
func (c *Client) StoreAPIKey(ctx context.Context, userID string, data APIKeyData) error {
	if !c.config.Enabled {
		// Store in local cache only (for development/testing)
		c.mu.Lock()
		c.cache[c.cacheKey(userID, data.Exchange, data.IsTestnet)] = &data
		c.mu.Unlock()
		return nil
	}

	path := c.secretPath(userID, data.Exchange, data.IsTestnet)

	secretData := map[string]interface{}{
		"data": map[string]interface{}{
			"api_key":    data.APIKey,
			"secret_key": data.SecretKey,
			"exchange":   data.Exchange,
			"is_testnet": data.IsTestnet,
		},
	}

	_, err := c.client.Logical().WriteWithContext(ctx, path, secretData)
	if err != nil {
		return fmt.Errorf("failed to store API key in vault: %w", err)
	}

	// Update cache
	if c.cacheEnabled {
		c.mu.Lock()
		c.cache[c.cacheKey(userID, data.Exchange, data.IsTestnet)] = &data
		c.mu.Unlock()
	}

	return nil
}

// GetAPIKey retrieves an API key for a user from Vault
func (c *Client) GetAPIKey(ctx context.Context, userID, exchange string, isTestnet bool) (*APIKeyData, error) {
	// Check cache first
	if c.cacheEnabled {
		c.mu.RLock()
		if cached, ok := c.cache[c.cacheKey(userID, exchange, isTestnet)]; ok {
			c.mu.RUnlock()
			return cached, nil
		}
		c.mu.RUnlock()
	}

	if !c.config.Enabled {
		return nil, fmt.Errorf("API key not found and vault is disabled")
	}

	path := c.secretPath(userID, exchange, isTestnet)

	secret, err := c.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read API key from vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("API key not found")
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid secret format")
	}

	apiKeyData := &APIKeyData{
		APIKey:    getString(data, "api_key"),
		SecretKey: getString(data, "secret_key"),
		Exchange:  getString(data, "exchange"),
		IsTestnet: getBool(data, "is_testnet"),
	}

	// Update cache
	if c.cacheEnabled {
		c.mu.Lock()
		c.cache[c.cacheKey(userID, exchange, isTestnet)] = apiKeyData
		c.mu.Unlock()
	}

	return apiKeyData, nil
}

// DeleteAPIKey deletes an API key for a user from Vault
func (c *Client) DeleteAPIKey(ctx context.Context, userID, exchange string, isTestnet bool) error {
	// Remove from cache
	c.mu.Lock()
	delete(c.cache, c.cacheKey(userID, exchange, isTestnet))
	c.mu.Unlock()

	if !c.config.Enabled {
		return nil
	}

	path := c.metadataPath(userID, exchange, isTestnet)

	_, err := c.client.Logical().DeleteWithContext(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete API key from vault: %w", err)
	}

	return nil
}

// RotateAPIKey updates an existing API key
func (c *Client) RotateAPIKey(ctx context.Context, userID string, newData APIKeyData) error {
	return c.StoreAPIKey(ctx, userID, newData)
}

// ListUserKeys lists all API keys for a user
func (c *Client) ListUserKeys(ctx context.Context, userID string) ([]APIKeyData, error) {
	if !c.config.Enabled {
		// Return cached keys for disabled vault
		c.mu.RLock()
		defer c.mu.RUnlock()

		var keys []APIKeyData
		prefix := userID + "/"
		for key, data := range c.cache {
			if len(key) > len(prefix) && key[:len(prefix)] == prefix {
				keys = append(keys, *data)
			}
		}
		return keys, nil
	}

	path := fmt.Sprintf("%s/metadata/%s/%s", c.config.MountPath, c.config.SecretPath, userID)

	secret, err := c.client.Logical().ListWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	keys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return nil, nil
	}

	var result []APIKeyData
	for _, key := range keys {
		keyStr, ok := key.(string)
		if !ok {
			continue
		}
		// Parse the key to extract exchange and testnet info
		// Key format: exchange_testnet or exchange_mainnet
		apiKey, err := c.GetAPIKey(ctx, userID, keyStr, false) // Try mainnet first
		if err != nil {
			apiKey, err = c.GetAPIKey(ctx, userID, keyStr, true) // Try testnet
			if err != nil {
				continue
			}
		}
		result = append(result, *apiKey)
	}

	return result, nil
}

// ClearCache clears the in-memory cache
func (c *Client) ClearCache() {
	c.mu.Lock()
	c.cache = make(map[string]*APIKeyData)
	c.mu.Unlock()
}

// InvalidateCacheForUser removes cached API keys for a specific user
func (c *Client) InvalidateCacheForUser(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := userID + "/"
	for key := range c.cache {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			delete(c.cache, key)
		}
	}
}

// SetCacheEnabled enables or disables caching
func (c *Client) SetCacheEnabled(enabled bool) {
	c.mu.Lock()
	c.cacheEnabled = enabled
	c.mu.Unlock()
}

// IsEnabled returns whether Vault is enabled
func (c *Client) IsEnabled() bool {
	return c.config.Enabled
}

// Health checks the Vault connection
func (c *Client) Health(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	health, err := c.client.Sys().Health()
	if err != nil {
		return fmt.Errorf("vault health check failed: %w", err)
	}

	if health.Sealed {
		return fmt.Errorf("vault is sealed")
	}

	return nil
}

// secretPath returns the path for storing a secret
func (c *Client) secretPath(userID, exchange string, isTestnet bool) string {
	network := "mainnet"
	if isTestnet {
		network = "testnet"
	}
	return fmt.Sprintf("%s/data/%s/%s/%s_%s", c.config.MountPath, c.config.SecretPath, userID, exchange, network)
}

// metadataPath returns the metadata path for a secret
func (c *Client) metadataPath(userID, exchange string, isTestnet bool) string {
	network := "mainnet"
	if isTestnet {
		network = "testnet"
	}
	return fmt.Sprintf("%s/metadata/%s/%s/%s_%s", c.config.MountPath, c.config.SecretPath, userID, exchange, network)
}

// cacheKey returns the cache key for an API key
func (c *Client) cacheKey(userID, exchange string, isTestnet bool) string {
	network := "mainnet"
	if isTestnet {
		network = "testnet"
	}
	return fmt.Sprintf("%s/%s_%s", userID, exchange, network)
}

// Helper functions
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getBool(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v == "true"
		case json.Number:
			n, _ := v.Int64()
			return n != 0
		}
	}
	return false
}

// MockClient creates a mock client for testing
func NewMockClient() *Client {
	return &Client{
		config: config.VaultConfig{
			Enabled: false,
		},
		cache:        make(map[string]*APIKeyData),
		cacheEnabled: true,
	}
}
