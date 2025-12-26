package binance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"binance-trading-bot/config"
	"binance-trading-bot/internal/vault"
)

// ClientFactory creates and manages per-user Binance clients
// NOTE: All API keys are per-user, stored in database. No global/master API keys.
type ClientFactory struct {
	vault          *vault.Client
	config         config.BinanceConfig
	futuresConfig  config.FuturesConfig

	// Per-user client caches
	spotClients    sync.Map // userID -> *clientEntry
	futuresClients sync.Map // userID -> *futuresClientEntry

	// Cache settings
	clientTTL     time.Duration
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

type clientEntry struct {
	client    BinanceClient
	createdAt time.Time
	lastUsed  time.Time
	mu        sync.Mutex
}

type futuresClientEntry struct {
	client    FuturesClient
	createdAt time.Time
	lastUsed  time.Time
	mu        sync.Mutex
}

// NewClientFactory creates a new client factory
// NOTE: All API keys are per-user, stored in database. No global/master API keys.
func NewClientFactory(
	vaultClient *vault.Client,
	cfg config.BinanceConfig,
	futuresCfg config.FuturesConfig,
) (*ClientFactory, error) {
	factory := &ClientFactory{
		vault:         vaultClient,
		config:        cfg,
		futuresConfig: futuresCfg,
		clientTTL:     30 * time.Minute,
		stopCleanup:   make(chan struct{}),
	}

	// Start cleanup goroutine
	factory.startCleanup()

	return factory, nil
}

// GetClientForUser returns a spot client for a specific user
func (f *ClientFactory) GetClientForUser(ctx context.Context, userID string) (BinanceClient, error) {
	// Check cache first
	if entry, ok := f.spotClients.Load(userID); ok {
		e := entry.(*clientEntry)
		e.mu.Lock()
		e.lastUsed = time.Now()
		e.mu.Unlock()
		return e.client, nil
	}

	// Get API key from vault
	apiKeyData, err := f.vault.GetAPIKey(ctx, userID, "binance", f.config.TestNet)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key for user %s: %w", userID, err)
	}

	// Create new client
	cfg := config.BinanceConfig{
		APIKey:    apiKeyData.APIKey,
		SecretKey: apiKeyData.SecretKey,
		BaseURL:   f.config.BaseURL,
		TestNet:   apiKeyData.IsTestnet,
		MockMode:  f.config.MockMode,
	}

	client := NewClient(cfg.APIKey, cfg.SecretKey, cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for user %s: %w", userID, err)
	}

	// Store in cache
	entry := &clientEntry{
		client:    client,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}
	f.spotClients.Store(userID, entry)

	return client, nil
}

// GetFuturesClientForUser returns a futures client for a specific user
func (f *ClientFactory) GetFuturesClientForUser(ctx context.Context, userID string) (FuturesClient, error) {
	// Check cache first
	if entry, ok := f.futuresClients.Load(userID); ok {
		e := entry.(*futuresClientEntry)
		e.mu.Lock()
		e.lastUsed = time.Now()
		e.mu.Unlock()
		return e.client, nil
	}

	// Get API key from vault
	apiKeyData, err := f.vault.GetAPIKey(ctx, userID, "binance", f.futuresConfig.TestNet)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key for user %s: %w", userID, err)
	}

	// Create new client
	client := NewFuturesClient(apiKeyData.APIKey, apiKeyData.SecretKey, apiKeyData.IsTestnet)

	// Store in cache
	entry := &futuresClientEntry{
		client:    client,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}
	f.futuresClients.Store(userID, entry)

	return client, nil
}

// GetMasterClient is deprecated - all API keys are per-user
// Returns nil to indicate no global/master client
func (f *ClientFactory) GetMasterClient() BinanceClient {
	return nil
}

// GetMasterFuturesClient is deprecated - all API keys are per-user
// Returns nil to indicate no global/master client
func (f *ClientFactory) GetMasterFuturesClient() FuturesClient {
	return nil
}

// HasMasterClient is deprecated - always returns false
// All API keys are per-user, stored in database
func (f *ClientFactory) HasMasterClient() bool {
	return false
}

// HasMasterFuturesClient is deprecated - always returns false
// All API keys are per-user, stored in database
func (f *ClientFactory) HasMasterFuturesClient() bool {
	return false
}

// InvalidateClient removes a client from the cache
func (f *ClientFactory) InvalidateClient(userID string) {
	f.spotClients.Delete(userID)
	f.futuresClients.Delete(userID)
}

// InvalidateAllClients clears all cached clients
func (f *ClientFactory) InvalidateAllClients() {
	f.spotClients.Range(func(key, value interface{}) bool {
		f.spotClients.Delete(key)
		return true
	})
	f.futuresClients.Range(func(key, value interface{}) bool {
		f.futuresClients.Delete(key)
		return true
	})
}

// GetOrCreateMockClient returns a mock client for testing/development
func (f *ClientFactory) GetOrCreateMockClient() BinanceClient {
	return &MockClient{}
}

// GetOrCreateMockFuturesClient returns a mock futures client for testing/development
func (f *ClientFactory) GetOrCreateMockFuturesClient() FuturesClient {
	return NewFuturesMockClient(10000.0, func(symbol string) (float64, error) {
    return 0.0, nil // Mock price function
})
}

// SetClientTTL sets the time-to-live for cached clients
func (f *ClientFactory) SetClientTTL(ttl time.Duration) {
	f.clientTTL = ttl
}

// startCleanup starts the periodic cleanup of expired clients
func (f *ClientFactory) startCleanup() {
	f.cleanupTicker = time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-f.cleanupTicker.C:
				f.cleanupExpiredClients()
			case <-f.stopCleanup:
				f.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// cleanupExpiredClients removes clients that haven't been used recently
func (f *ClientFactory) cleanupExpiredClients() {
	now := time.Now()

	// Clean spot clients
	f.spotClients.Range(func(key, value interface{}) bool {
		entry := value.(*clientEntry)
		entry.mu.Lock()
		if now.Sub(entry.lastUsed) > f.clientTTL {
			f.spotClients.Delete(key)
		}
		entry.mu.Unlock()
		return true
	})

	// Clean futures clients
	f.futuresClients.Range(func(key, value interface{}) bool {
		entry := value.(*futuresClientEntry)
		entry.mu.Lock()
		if now.Sub(entry.lastUsed) > f.clientTTL {
			f.futuresClients.Delete(key)
		}
		entry.mu.Unlock()
		return true
	})
}

// Close stops the cleanup goroutine and clears all clients
func (f *ClientFactory) Close() {
	close(f.stopCleanup)
	f.InvalidateAllClients()
}

// Stats returns statistics about the client factory
func (f *ClientFactory) Stats() FactoryStats {
	var spotCount, futuresCount int

	f.spotClients.Range(func(key, value interface{}) bool {
		spotCount++
		return true
	})

	f.futuresClients.Range(func(key, value interface{}) bool {
		futuresCount++
		return true
	})

	return FactoryStats{
		CachedSpotClients:    spotCount,
		CachedFuturesClients: futuresCount,
		VaultEnabled:         f.vault.IsEnabled(),
	}
}

// FactoryStats contains statistics about the client factory
type FactoryStats struct {
	CachedSpotClients    int  `json:"cached_spot_clients"`
	CachedFuturesClients int  `json:"cached_futures_clients"`
	VaultEnabled         bool `json:"vault_enabled"`
}

// UserClientManager provides a simplified interface for getting clients
// with automatic fallback to mock clients in development mode
type UserClientManager struct {
	factory  *ClientFactory
	devMode  bool
}

// NewUserClientManager creates a new user client manager
func NewUserClientManager(factory *ClientFactory, devMode bool) *UserClientManager {
	return &UserClientManager{
		factory: factory,
		devMode: devMode,
	}
}

// GetSpotClient returns a spot client for the user, with fallback to mock in dev mode
func (m *UserClientManager) GetSpotClient(ctx context.Context, userID string) (BinanceClient, error) {
	client, err := m.factory.GetClientForUser(ctx, userID)
	if err != nil {
		if m.devMode {
			return m.factory.GetOrCreateMockClient(), nil
		}
		return nil, err
	}
	return client, nil
}

// GetFuturesClient returns a futures client for the user, with fallback to mock in dev mode
func (m *UserClientManager) GetFuturesClient(ctx context.Context, userID string) (FuturesClient, error) {
	client, err := m.factory.GetFuturesClientForUser(ctx, userID)
	if err != nil {
		if m.devMode {
			return m.factory.GetOrCreateMockFuturesClient(), nil
		}
		return nil, err
	}
	return client, nil
}

// GetMasterSpotClient is deprecated - all API keys are per-user
// Returns mock client in dev mode, nil otherwise
func (m *UserClientManager) GetMasterSpotClient() BinanceClient {
	if m.devMode {
		return m.factory.GetOrCreateMockClient()
	}
	return nil
}

// GetMasterFuturesClient is deprecated - all API keys are per-user
// Returns mock client in dev mode, nil otherwise
func (m *UserClientManager) GetMasterFuturesClient() FuturesClient {
	if m.devMode {
		return m.factory.GetOrCreateMockFuturesClient()
	}
	return nil
}
