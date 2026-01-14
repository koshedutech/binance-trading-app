package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/apikeys"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/logging"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// UserAutopilotInstance holds a single user's autopilot session
// Each user gets their own isolated instance with their own:
// - GinieAutopilot (positions, trades, daily stats)
// - FuturesClient (user's Binance API keys)
// - LLMAnalyzer (user's AI API key)
type UserAutopilotInstance struct {
	UserID        string
	FuturesClient binance.FuturesClient
	LLMAnalyzer   *llm.Analyzer
	Autopilot     *GinieAutopilot
	CreatedAt     time.Time
	LastActive    time.Time

	mu sync.RWMutex
}

// IsRunning returns whether this user's autopilot is currently running
func (u *UserAutopilotInstance) IsRunning() bool {
	if u.Autopilot == nil {
		return false
	}
	return u.Autopilot.IsRunning()
}

// TouchLastActive updates the last active timestamp
func (u *UserAutopilotInstance) TouchLastActive() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.LastActive = time.Now()
}

// UserAutopilotManager manages multiple concurrent user autopilot instances
// This enables true multi-user simultaneous trading where each user has their own:
// - Independent Ginie autopilot
// - Independent Binance client (using their own API keys)
// - Independent LLM analyzer (using their own AI keys)
// - Independent position tracking
// - Independent daily limits and circuit breakers
type UserAutopilotManager struct {
	// Per-user instances (thread-safe map)
	instances sync.Map // map[userID string] -> *UserAutopilotInstance

	// Shared components (read-only, safe to share)
	repo              *database.Repository
	positionStateRepo *database.RedisPositionStateRepository // Shared Redis position state
	ginieAnalyzer     *GinieAnalyzer
	logger            *logging.Logger

	// Client factory for creating per-user Binance clients
	clientFactory *binance.ClientFactory

	// API key service for retrieving user's API keys
	apiKeyService *apikeys.Service

	// LLM config for creating per-user analyzers
	llmConfig *llm.AnalyzerConfig

	// Cleanup settings
	cleanupInterval    time.Duration // How often to clean up idle sessions
	sessionIdleTimeout time.Duration // Close sessions idle for this long

	// Cleanup goroutine control
	cleanupStop chan struct{}
	cleanupWg   sync.WaitGroup

	mu sync.RWMutex
}

// NewUserAutopilotManager creates a new multi-user autopilot manager
// positionStateRepo can be nil (position state will fall back to JSON file only)
func NewUserAutopilotManager(
	repo *database.Repository,
	ginieAnalyzer *GinieAnalyzer,
	clientFactory *binance.ClientFactory,
	apiKeyService *apikeys.Service,
	llmConfig *llm.AnalyzerConfig,
	logger *logging.Logger,
	positionStateRepo *database.RedisPositionStateRepository,
) *UserAutopilotManager {
	mgr := &UserAutopilotManager{
		repo:               repo,
		positionStateRepo:  positionStateRepo,
		ginieAnalyzer:      ginieAnalyzer,
		clientFactory:      clientFactory,
		apiKeyService:      apiKeyService,
		llmConfig:          llmConfig,
		logger:             logger,
		cleanupInterval:    5 * time.Minute,
		sessionIdleTimeout: 30 * time.Minute,
		cleanupStop:        make(chan struct{}),
	}

	// Start background cleanup goroutine
	mgr.cleanupWg.Add(1)
	go mgr.cleanupLoop()

	return mgr
}

// cleanupLoop periodically removes idle user sessions
func (m *UserAutopilotManager) cleanupLoop() {
	defer m.cleanupWg.Done()

	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupIdleSessions()
		case <-m.cleanupStop:
			return
		}
	}
}

// cleanupIdleSessions removes sessions that have been idle too long
func (m *UserAutopilotManager) cleanupIdleSessions() {
	now := time.Now()
	var toRemove []string

	m.instances.Range(func(key, value any) bool {
		userID := key.(string)
		instance := value.(*UserAutopilotInstance)

		instance.mu.RLock()
		idleDuration := now.Sub(instance.LastActive)
		isRunning := instance.IsRunning()
		instance.mu.RUnlock()

		// Don't remove running instances, only truly idle ones
		if !isRunning && idleDuration > m.sessionIdleTimeout {
			toRemove = append(toRemove, userID)
		}
		return true
	})

	for _, userID := range toRemove {
		log.Printf("[USER-AUTOPILOT] Cleaning up idle session for user %s", userID)
		m.instances.Delete(userID)
	}
}

// GetOrCreateInstance gets an existing instance or creates a new one for a user
func (m *UserAutopilotManager) GetOrCreateInstance(ctx context.Context, userID string) (*UserAutopilotInstance, error) {
	// Check for existing instance
	if existing, ok := m.instances.Load(userID); ok {
		instance := existing.(*UserAutopilotInstance)
		instance.TouchLastActive()
		return instance, nil
	}

	// Create new instance
	return m.createInstance(ctx, userID)
}

// createInstance creates a new user autopilot instance
func (m *UserAutopilotManager) createInstance(ctx context.Context, userID string) (*UserAutopilotInstance, error) {
	m.logger.Info("Creating new autopilot instance for user", "user_id", userID)

	var futuresClient binance.FuturesClient
	var err error

	// Try ClientFactory first (vault-based), then fall back to apiKeyService (database-based)
	if m.clientFactory != nil {
		futuresClient, err = m.clientFactory.GetFuturesClientForUser(ctx, userID)
		if err != nil {
			m.logger.Warn("ClientFactory failed, falling back to apiKeyService", "user_id", userID, "error", err)
		}
	}

	// Fallback: Use apiKeyService to get keys directly from database
	if futuresClient == nil && m.apiKeyService != nil {
		binanceKey, err := m.apiKeyService.GetActiveBinanceKey(ctx, userID, false) // false = mainnet
		if err != nil {
			return nil, fmt.Errorf("failed to get Binance API key for user %s: %w", userID, err)
		}
		if binanceKey == nil || binanceKey.APIKey == "" {
			return nil, fmt.Errorf("user %s has no Binance API keys configured", userID)
		}

		// Create futures client directly from API keys
		futuresClient = binance.NewFuturesClient(binanceKey.APIKey, binanceKey.SecretKey, binanceKey.IsTestnet)
		m.logger.Info("Created futures client from apiKeyService", "user_id", userID, "testnet", binanceKey.IsTestnet)
	}

	if futuresClient == nil {
		return nil, fmt.Errorf("user %s has no Binance API keys configured", userID)
	}

	// Get user's AI API key and create LLM analyzer
	var llmAnalyzer *llm.Analyzer
	if m.llmConfig != nil {
		// Get user's AI keys from database
		aiKey, err := m.apiKeyService.GetActiveAIKey(ctx, userID)
		if err == nil && aiKey != nil && aiKey.APIKey != "" {
			// Create per-user LLM analyzer with their AI key
			userLLMConfig := *m.llmConfig
			userLLMConfig.APIKey = aiKey.APIKey
			llmAnalyzer = llm.NewAnalyzer(&userLLMConfig)
			if llmAnalyzer != nil {
				m.logger.Info("Created LLM analyzer for user", "user_id", userID, "provider", userLLMConfig.Provider)
			}
		}
	}

	// Create per-user GinieAutopilot instance with userID for multi-tenant PnL isolation
	// Pass shared Redis position state repository for cross-instance state sharing
	autopilot := NewGinieAutopilot(
		m.ginieAnalyzer,
		futuresClient,
		m.logger,
		m.repo,
		userID,
		m.positionStateRepo, // Shared Redis position state (may be nil)
	)

	// Set the LLM analyzer if we have one
	if llmAnalyzer != nil {
		autopilot.SetLLMAnalyzer(llmAnalyzer)
	}

	// Apply global settings (RiskLevel, etc.) from SettingsManager
	settingsManager := GetSettingsManager()
	if settingsManager != nil {
		settings := settingsManager.GetDefaultSettings()
		if settings.RiskLevel != "" {
			if err := autopilot.SetRiskLevel(settings.RiskLevel); err != nil {
				m.logger.Warn("Failed to apply risk level to user autopilot", "error", err, "user_id", userID)
			} else {
				m.logger.Info("Applied risk level to user autopilot", "risk_level", settings.RiskLevel, "user_id", userID)
			}
		}
	}

	// Load persisted stats
	autopilot.LoadPnLStats()

	instance := &UserAutopilotInstance{
		UserID:        userID,
		FuturesClient: futuresClient,
		LLMAnalyzer:   llmAnalyzer,
		Autopilot:     autopilot,
		CreatedAt:     time.Now(),
		LastActive:    time.Now(),
	}

	// Store instance (use LoadOrStore to handle race conditions)
	actual, loaded := m.instances.LoadOrStore(userID, instance)
	if loaded {
		// Another goroutine created it first, use theirs
		return actual.(*UserAutopilotInstance), nil
	}

	m.logger.Info("Created new autopilot instance for user", "user_id", userID)

	// Check per-user auto-start setting from database
	if m.repo != nil {
		tradingConfig, err := m.repo.GetUserTradingConfig(ctx, userID)
		if err == nil && tradingConfig != nil && tradingConfig.AutopilotEnabled {
			m.logger.Info("Per-user auto-start enabled, starting autopilot",
				"user_id", userID,
				"autopilot_enabled", tradingConfig.AutopilotEnabled)
			autopilot.Start()
		}
	}

	return instance, nil
}

// GetInstance gets an existing instance for a user (nil if not exists)
func (m *UserAutopilotManager) GetInstance(userID string) *UserAutopilotInstance {
	if existing, ok := m.instances.Load(userID); ok {
		instance := existing.(*UserAutopilotInstance)
		instance.TouchLastActive()
		return instance
	}
	return nil
}

// StartAutopilot starts the autopilot for a specific user
func (m *UserAutopilotManager) StartAutopilot(ctx context.Context, userID string) error {
	instance, err := m.GetOrCreateInstance(ctx, userID)
	if err != nil {
		return err
	}

	if instance.Autopilot.IsRunning() {
		return nil // Already running
	}

	m.logger.Info("Starting autopilot for user", "user_id", userID)
	instance.Autopilot.Start()
	instance.TouchLastActive()

	return nil
}

// StopAutopilot stops the autopilot for a specific user
func (m *UserAutopilotManager) StopAutopilot(userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil // Nothing to stop
	}

	if !instance.Autopilot.IsRunning() {
		return nil // Already stopped
	}

	m.logger.Info("Stopping autopilot for user", "user_id", userID)
	instance.Autopilot.Stop()
	instance.TouchLastActive()

	return nil
}

// GetStatus returns the autopilot status for a specific user
func (m *UserAutopilotManager) GetStatus(userID string) *UserAutopilotStatus {
	instance := m.GetInstance(userID)
	if instance == nil {
		return &UserAutopilotStatus{
			UserID:  userID,
			Running: false,
			Message: "No autopilot instance",
		}
	}

	instance.TouchLastActive()
	stats := instance.Autopilot.GetStats()

	// Extract values from stats map
	running, _ := stats["running"].(bool)
	dryRun, _ := stats["dry_run"].(bool)
	totalTrades, _ := stats["total_trades"].(int)
	winRate, _ := stats["win_rate"].(float64)
	totalPnL, _ := stats["total_pnl"].(float64)
	dailyTrades, _ := stats["daily_trades"].(int)
	dailyPnL, _ := stats["daily_pnl"].(float64)
	activePositions, _ := stats["active_positions"].(int)

	// Get circuit breaker status
	cbStatus := instance.Autopilot.GetCircuitBreakerStatus()
	cbMessage := "unknown"
	if tripped, ok := cbStatus["tripped"].(bool); ok && tripped {
		cbMessage = "tripped"
	} else if enabled, ok := cbStatus["enabled"].(bool); ok && enabled {
		cbMessage = "active"
	} else {
		cbMessage = "disabled"
	}

	return &UserAutopilotStatus{
		UserID:          userID,
		Running:         running,
		DryRun:          dryRun,
		ActivePositions: activePositions,
		TotalTrades:     totalTrades,
		WinRate:         winRate,
		TotalPnL:        totalPnL,
		DailyTrades:     dailyTrades,
		DailyPnL:        dailyPnL,
		CircuitBreaker:  cbMessage,
		CreatedAt:       instance.CreatedAt,
		LastActive:      instance.LastActive,
	}
}

// GetPositions returns the positions for a specific user
func (m *UserAutopilotManager) GetPositions(userID string) []*GiniePosition {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil
	}
	return instance.Autopilot.GetPositions()
}

// GetTradeHistory returns the trade history for a specific user
func (m *UserAutopilotManager) GetTradeHistory(userID string, limit int) []GinieTradeResult {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil
	}
	return instance.Autopilot.GetTradeHistory(limit)
}

// IsRunning checks if a user's autopilot is running
func (m *UserAutopilotManager) IsRunning(userID string) bool {
	instance := m.GetInstance(userID)
	if instance == nil {
		return false
	}
	return instance.Autopilot.IsRunning()
}

// GetAllRunningUsers returns list of user IDs with running autopilots
func (m *UserAutopilotManager) GetAllRunningUsers() []string {
	var runningUsers []string

	m.instances.Range(func(key, value any) bool {
		userID := key.(string)
		instance := value.(*UserAutopilotInstance)

		if instance.IsRunning() {
			runningUsers = append(runningUsers, userID)
		}
		return true
	})

	return runningUsers
}

// GetInstanceCount returns the number of active instances
func (m *UserAutopilotManager) GetInstanceCount() int {
	count := 0
	m.instances.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}

// GetRunningCount returns the number of running autopilots
func (m *UserAutopilotManager) GetRunningCount() int {
	count := 0
	m.instances.Range(func(key, value any) bool {
		instance := value.(*UserAutopilotInstance)
		if instance.IsRunning() {
			count++
		}
		return true
	})
	return count
}

// Shutdown stops all running autopilots and cleans up
func (m *UserAutopilotManager) Shutdown() {
	m.logger.Info("Shutting down UserAutopilotManager")

	// Stop cleanup goroutine
	close(m.cleanupStop)
	m.cleanupWg.Wait()

	// Stop all running autopilots
	m.instances.Range(func(key, value any) bool {
		userID := key.(string)
		instance := value.(*UserAutopilotInstance)

		if instance.IsRunning() {
			m.logger.Info("Stopping autopilot for user during shutdown", "user_id", userID)
			instance.Autopilot.Stop()
		}
		return true
	})

	m.logger.Info("UserAutopilotManager shutdown complete")
}

// UpdateUserDryRun updates the dry run mode for a specific user
func (m *UserAutopilotManager) UpdateUserDryRun(userID string, dryRun bool) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil // Nothing to update
	}

	// Update the autopilot's config
	config := instance.Autopilot.GetConfig()
	if config != nil {
		config.DryRun = dryRun
		instance.Autopilot.SetConfig(config)
	}

	m.logger.Info("Updated dry run mode for user", "user_id", userID, "dry_run", dryRun)
	return nil
}

// RefreshUserClient refreshes the Binance client for a user (e.g., after API key update)
func (m *UserAutopilotManager) RefreshUserClient(ctx context.Context, userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil // Nothing to refresh
	}

	// Stop autopilot if running
	wasRunning := instance.IsRunning()
	if wasRunning {
		instance.Autopilot.Stop()
	}

	// Get new client
	newClient, err := m.clientFactory.GetFuturesClientForUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to refresh client for user %s: %w", userID, err)
	}

	// Update instance
	instance.mu.Lock()
	instance.FuturesClient = newClient
	instance.Autopilot.SetFuturesClient(newClient)
	instance.mu.Unlock()

	// Restart if was running
	if wasRunning {
		instance.Autopilot.Start()
	}

	m.logger.Info("Refreshed client for user", "user_id", userID)
	return nil
}

// UserAutopilotStatus provides autopilot status for a specific user
type UserAutopilotStatus struct {
	UserID          string    `json:"user_id"`
	Running         bool      `json:"running"`
	DryRun          bool      `json:"dry_run"`
	ActivePositions int       `json:"active_positions"`
	TotalTrades     int       `json:"total_trades"`
	WinRate         float64   `json:"win_rate"`
	TotalPnL        float64   `json:"total_pnl"`
	DailyTrades     int       `json:"daily_trades"`
	DailyPnL        float64   `json:"daily_pnl"`
	CircuitBreaker  string    `json:"circuit_breaker"`
	Message         string    `json:"message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	LastActive      time.Time `json:"last_active"`
}

// ManagerStatus provides an overview of all user autopilots
type ManagerStatus struct {
	TotalInstances    int      `json:"total_instances"`
	RunningInstances  int      `json:"running_instances"`
	RunningUserIDs    []string `json:"running_user_ids"`
}

// GetManagerStatus returns the overall manager status
func (m *UserAutopilotManager) GetManagerStatus() *ManagerStatus {
	return &ManagerStatus{
		TotalInstances:   m.GetInstanceCount(),
		RunningInstances: m.GetRunningCount(),
		RunningUserIDs:   m.GetAllRunningUsers(),
	}
}

// AutoStartFromSettings checks if auto-start is enabled and starts Ginie for the saved user
// This should be called after server initialization to restore Ginie state from before restart
func (m *UserAutopilotManager) AutoStartFromSettings(ctx context.Context) error {
	sm := GetSettingsManager()
	if sm == nil {
		m.logger.Warn("SettingsManager not available for auto-start check")
		return nil
	}

	if !sm.GetGinieAutoStart() {
		m.logger.Info("Ginie auto-start is disabled, skipping")
		return nil
	}

	userID := sm.GetGinieAutoStartUserID()
	if userID == "" {
		m.logger.Warn("Ginie auto-start enabled but no user ID saved, skipping")
		return nil
	}

	m.logger.Info("Auto-starting Ginie from saved settings",
		"user_id", userID,
		"auto_start", true)

	// Start autopilot for the saved user
	if err := m.StartAutopilot(ctx, userID); err != nil {
		m.logger.Error("Failed to auto-start Ginie for user",
			"user_id", userID,
			"error", err)
		return fmt.Errorf("failed to auto-start Ginie for user %s: %w", userID, err)
	}

	m.logger.Info("Ginie auto-started successfully",
		"user_id", userID)

	return nil
}
