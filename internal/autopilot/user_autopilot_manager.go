package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/apikeys"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/coinprofiler"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/exitdecision"
	"binance-trading-bot/internal/logging"
	"binance-trading-bot/internal/orders"
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
// - ChainEntryRunner (automatic chain-based entries when entry_decision_system="chain")
// - CoinProfiler (real-time WebSocket data collection for Chain Trading System)
// - ExitDecisionService (monitors positions for TP/SL/trailing stop exits)
// - PositionController (Story 10.4: executes exit signals on Binance)
type UserAutopilotInstance struct {
	UserID              string
	FuturesClient       binance.FuturesClient
	LLMAnalyzer         *llm.Analyzer
	Autopilot           *GinieAutopilot
	ChainEntryRunner    *ChainEntryRunner              // Automatic chain entries (independent of Ginie)
	CoinProfiler        *coinprofiler.CoinProfiler     // Epic 14: Real-time data collection
	ExitDecisionService *exitdecision.Service          // Epic 14: Exit signal monitoring
	PositionController  *PositionController            // Story 10.4: Exit signal executor
	CreatedAt           time.Time
	LastActive          time.Time

	mu sync.RWMutex
}

// IsRunning returns whether this user's autopilot is currently running
func (u *UserAutopilotInstance) IsRunning() bool {
	if u.Autopilot == nil {
		return false
	}
	return u.Autopilot.IsRunning()
}

// IsChainEntryRunnerRunning returns whether this user's chain entry runner is active
func (u *UserAutopilotInstance) IsChainEntryRunnerRunning() bool {
	if u.ChainEntryRunner == nil {
		return false
	}
	return u.ChainEntryRunner.IsRunning()
}

// IsCoinProfilerRunning returns whether this user's coin profiler is active
func (u *UserAutopilotInstance) IsCoinProfilerRunning() bool {
	if u.CoinProfiler == nil {
		return false
	}
	return u.CoinProfiler.IsRunning()
}

// IsExitDecisionRunning returns whether this user's exit decision service is active
func (u *UserAutopilotInstance) IsExitDecisionRunning() bool {
	if u.ExitDecisionService == nil {
		return false
	}
	return u.ExitDecisionService.IsRunning()
}

// IsPositionControllerRunning returns whether this user's position controller is active
func (u *UserAutopilotInstance) IsPositionControllerRunning() bool {
	if u.PositionController == nil {
		return false
	}
	return u.PositionController.IsRunning()
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
	settingsCache     SettingsCacheReader                    // Story 6.6: Cache-only settings reads
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

	// Epic 11: StateManager for Entry Decision Engine integration
	stateManager StateManagerInterface

	// Epic 7: Position state integration for trade lifecycle tracking
	positionStateInt *PositionStateIntegration

	// Epic 11: ChainEventWriter for chain-based entry tracking
	chainEventWriter *orders.ChainEventWriter

	// Epic 11: ChainStateProvider for ChainEntryRunner (interface to avoid circular imports)
	chainStateProvider ChainStateProvider

	mu sync.RWMutex
}

// NewUserAutopilotManager creates a new multi-user autopilot manager
// positionStateRepo can be nil (position state will fall back to JSON file only)
// settingsCache is required for cache-only settings reads during trading (Story 6.6)
func NewUserAutopilotManager(
	repo *database.Repository,
	ginieAnalyzer *GinieAnalyzer,
	clientFactory *binance.ClientFactory,
	apiKeyService *apikeys.Service,
	llmConfig *llm.AnalyzerConfig,
	logger *logging.Logger,
	positionStateRepo *database.RedisPositionStateRepository,
	settingsCache SettingsCacheReader,
) *UserAutopilotManager {
	mgr := &UserAutopilotManager{
		repo:               repo,
		positionStateRepo:  positionStateRepo,
		settingsCache:      settingsCache,
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

// SetStateManager sets the Decision Engine state manager for saving coin states during scanning (Epic 11)
// This is called from main.go after the StateManager is initialized
func (m *UserAutopilotManager) SetStateManager(sm StateManagerInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateManager = sm
	m.logger.Info("StateManager set on UserAutopilotManager for Entry Decision Engine integration")
}

// SetPositionStateIntegration sets the position state integration for trade lifecycle tracking (Epic 7)
// This enables entry order details to be saved to the database when orders fill
// Called from main.go after the PositionStateIntegration is initialized
func (m *UserAutopilotManager) SetPositionStateIntegration(psi *PositionStateIntegration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.positionStateInt = psi
	m.logger.Info("PositionStateIntegration set on UserAutopilotManager for trade lifecycle tracking")
}

// SetSettingsCache sets the settings cache service for cache-only reads.
// This fixes the startup race condition where Redis may not be ready when the manager is created.
// It updates the manager's cache reference AND propagates to all existing user autopilot instances.
// Called from main.go after the SettingsCacheService is initialized (Story 6.6).
func (m *UserAutopilotManager) SetSettingsCache(cache SettingsCacheReader) {
	m.settingsCache = cache
	m.logger.Info("SettingsCache set on UserAutopilotManager (late initialization)")

	// Propagate to all existing user autopilot instances (using sync.Map Range)
	propagated := 0
	m.instances.Range(func(key, value interface{}) bool {
		userID := key.(string)
		instance := value.(*UserAutopilotInstance)
		if instance.Autopilot != nil {
			instance.Autopilot.SetSettingsCache(cache)
			propagated++
			m.logger.Debug("Propagated SettingsCache to user autopilot", "user_id", userID)
		}
		return true // continue iteration
	})
	if propagated > 0 {
		m.logger.Info("SettingsCache propagated to existing user autopilots", "count", propagated)
	}
}

// SetChainEventWriter sets the chain event writer for recording order chain events (Epic 11)
// This is called from main.go after the ChainEventWriter is initialized
func (m *UserAutopilotManager) SetChainEventWriter(cew *orders.ChainEventWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chainEventWriter = cew
	m.logger.Info("ChainEventWriter set on UserAutopilotManager for chain-based entry tracking")

	// Propagate to existing ChainEntryRunners
	m.instances.Range(func(key, value interface{}) bool {
		instance := value.(*UserAutopilotInstance)
		if instance.ChainEntryRunner != nil {
			instance.ChainEntryRunner.SetChainEventWriter(cew)
		}
		return true
	})
}

// SetChainStateProvider sets the chain state provider for ChainEntryRunner (Epic 11)
// This is called from main.go after the StateManager is initialized
func (m *UserAutopilotManager) SetChainStateProvider(sp ChainStateProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chainStateProvider = sp
	m.logger.Info("ChainStateProvider set on UserAutopilotManager for ChainEntryRunner")

	// Propagate to existing ChainEntryRunners
	m.instances.Range(func(key, value interface{}) bool {
		instance := value.(*UserAutopilotInstance)
		if instance.ChainEntryRunner != nil {
			instance.ChainEntryRunner.SetStateProvider(sp)
		}
		return true
	})
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

	// Epic 7: Create per-user ClientOrderIdGenerator for trade lifecycle tracking
	// settingsCache implements orders.SequenceProvider interface
	var clientOrderIdGen *orders.ClientOrderIdGenerator
	if isSettingsCacheValid(m.settingsCache) {
		// Story 7.6: Load user's timezone preference for clientOrderId date formatting
		var userTimezone *time.Location
		if tzStr, err := m.repo.GetUserTimezone(ctx, userID); err == nil && tzStr != "" {
			if loc, err := time.LoadLocation(tzStr); err == nil {
				userTimezone = loc
				m.logger.Info("Using user timezone for ClientOrderIdGenerator", "user_id", userID, "timezone", tzStr)
			}
		}
		gen, err := orders.NewClientOrderIdGenerator(m.settingsCache, userID, userTimezone)
		if err != nil {
			m.logger.Warn("Failed to create ClientOrderIdGenerator, orders will not have custom clientOrderId",
				"user_id", userID, "error", err)
		} else {
			clientOrderIdGen = gen
			m.logger.Info("Created ClientOrderIdGenerator for user", "user_id", userID)
		}
	}

	// Create per-user GinieAutopilot instance with userID for multi-tenant PnL isolation
	// Pass shared Redis position state repository for cross-instance state sharing
	// Story 6.6: Pass settingsCache for cache-only settings reads during trading
	autopilot := NewGinieAutopilot(
		m.ginieAnalyzer,
		futuresClient,
		m.logger,
		m.repo,
		userID,
		m.positionStateRepo, // Shared Redis position state (may be nil)
		m.settingsCache,     // Story 6.6: Cache-only settings reads
		clientOrderIdGen,    // Epic 7: Client order ID generator (may be nil)
	)

	// Set the LLM analyzer if we have one
	if llmAnalyzer != nil {
		autopilot.SetLLMAnalyzer(llmAnalyzer)
	}

	// Story 9.12: Apply global settings (RiskLevel) from default-settings.json
	// This reads from default-settings.json which is the single source of truth for defaults.
	// Future enhancement: Load user's risk level from database (user_ginie_settings table)
	// and apply it via cache service. For now, using default-settings.json.
	defaults, err := LoadDefaultSettings()
	if err != nil {
		m.logger.Warn("Failed to load default settings for risk level", "error", err, "user_id", userID)
	} else if defaults.GlobalTrading.RiskLevel != "" {
		if err := autopilot.SetRiskLevel(defaults.GlobalTrading.RiskLevel); err != nil {
			m.logger.Warn("Failed to apply risk level to user autopilot", "error", err, "user_id", userID)
		} else {
			m.logger.Info("Applied risk level to user autopilot", "risk_level", defaults.GlobalTrading.RiskLevel, "user_id", userID)
		}
	}

	// Load persisted stats
	autopilot.LoadPnLStats()

	// Epic 11: Set StateManager for Entry Decision Engine integration
	// This allows the autopilot to save coin states during scanning
	if m.stateManager != nil {
		autopilot.SetStateManager(m.stateManager)
		m.logger.Info("StateManager set on user autopilot for Entry Decision Engine", "user_id", userID)
	}

	// Epic 7: Set PositionStateIntegration for trade lifecycle tracking
	// This enables entry order details to be saved when orders fill
	if m.positionStateInt != nil {
		autopilot.SetPositionStateIntegration(m.positionStateInt)
		m.logger.Info("PositionStateIntegration set on user autopilot for trade lifecycle", "user_id", userID)
	}

	// Epic 11: Create ChainEntryRunner for automatic chain-based entries
	// This runs independently of GinieAutopilot when entry_decision_system = "chain"
	var chainEntryRunner *ChainEntryRunner
	if m.chainStateProvider != nil {
		chainEntryRunner = NewChainEntryRunner(
			userID,
			m.chainStateProvider,
			futuresClient,
			m.chainEventWriter,
			m.settingsCache,
			m.repo,
			m.logger,
			nil, // Use default config
		)
		m.logger.Info("ChainEntryRunner created for user", "user_id", userID)
	}

	// Epic 14: Create CoinProfiler for real-time WebSocket data collection
	coinProfiler := coinprofiler.NewCoinProfiler(nil, m.logger) // Use default config
	m.logger.Info("CoinProfiler created for user", "user_id", userID)

	// Epic 14: Create ExitDecisionService for position exit monitoring
	// Uses CoinProfiler for prices (via adapter) and Autopilot for positions (via adapter)
	exitDecisionSvc := exitdecision.NewService(nil, nil, nil) // Providers wired after creation
	m.logger.Info("ExitDecisionService created for user", "user_id", userID)

	// Story 10.4: Create PositionController for executing exit signals on Binance
	// PositionController subscribes to ExitDecisionService signals and executes SL/TP updates
	positionController := NewPositionController(
		futuresClient,
		exitDecisionSvc,
		m.chainEventWriter,
		nil, // Use default config
		userID,
	)
	m.logger.Info("PositionController created for user", "user_id", userID)

	instance := &UserAutopilotInstance{
		UserID:              userID,
		FuturesClient:       futuresClient,
		LLMAnalyzer:         llmAnalyzer,
		Autopilot:           autopilot,
		ChainEntryRunner:    chainEntryRunner,
		CoinProfiler:        coinProfiler,        // Epic 14: Real-time data collection
		ExitDecisionService: exitDecisionSvc,     // Epic 14: Exit signal monitoring
		PositionController:  positionController,  // Story 10.4: Exit signal executor
		CreatedAt:           time.Now(),
		LastActive:          time.Now(),
	}

	// Wire ExitDecisionService providers (after all components are created)
	// This connects CoinProfiler as price provider and Autopilot as position provider
	WireExitDecisionProviders(exitDecisionSvc, coinProfiler, autopilot, userID)
	m.logger.Info("ExitDecisionService providers wired", "user_id", userID)

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
			// Check system control to decide which systems to start
			systemControl, scErr := m.repo.GetUserSystemControlOrDefault(ctx, userID)
			isChainMode := scErr == nil && systemControl != nil && systemControl.IsEntryDecisionChain() && !systemControl.IsEntryDecisionLegacy()

			// Always start Ginie Autopilot for SCANNING
			// When chain mode: Ginie scans but doesn't place orders (blocked at entry time)
			// When legacy mode: Ginie scans AND places orders
			m.logger.Info("Per-user auto-start enabled, starting autopilot for scanning",
				"user_id", userID,
				"autopilot_enabled", tradingConfig.AutopilotEnabled,
				"chain_mode", isChainMode)
			autopilot.Start()

			// When chain mode is active, also start ChainEntryRunner for order execution
			if isChainMode && chainEntryRunner != nil {
				m.logger.Info("Auto-starting ChainEntryRunner for chain-based entries",
					"user_id", userID)
				chainEntryRunner.Start()
			}
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

// StartAutopilot starts the autopilot for a specific user.
// When entry_decision_system = "chain", also starts ChainEntryRunner for order execution.
func (m *UserAutopilotManager) StartAutopilot(ctx context.Context, userID string) error {
	instance, err := m.GetOrCreateInstance(ctx, userID)
	if err != nil {
		return err
	}

	if instance.Autopilot.IsRunning() {
		return nil // Already running
	}

	// Check if chain mode is active
	var isChainMode bool
	if m.repo != nil {
		systemControl, scErr := m.repo.GetUserSystemControlOrDefault(ctx, userID)
		if scErr == nil && systemControl != nil {
			isChainMode = systemControl.IsEntryDecisionChain() && !systemControl.IsEntryDecisionLegacy()
		}
	}

	// Always start Ginie for scanning
	m.logger.Info("Starting autopilot for user",
		"user_id", userID,
		"chain_mode", isChainMode)
	instance.Autopilot.Start()
	instance.TouchLastActive()

	// When chain mode is active, also start ChainEntryRunner for order execution
	if isChainMode && instance.ChainEntryRunner != nil && !instance.ChainEntryRunner.IsRunning() {
		m.logger.Info("Also starting ChainEntryRunner for chain-based entries",
			"user_id", userID)
		instance.ChainEntryRunner.Start()
	}

	return nil
}

// StopAutopilot stops the autopilot for a specific user.
// Also stops ChainEntryRunner if running.
func (m *UserAutopilotManager) StopAutopilot(userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil // Nothing to stop
	}

	// Stop Ginie if running
	if instance.Autopilot.IsRunning() {
		m.logger.Info("Stopping autopilot for user", "user_id", userID)
		instance.Autopilot.Stop()
	}

	// Also stop ChainEntryRunner if running
	if instance.ChainEntryRunner != nil && instance.ChainEntryRunner.IsRunning() {
		m.logger.Info("Also stopping ChainEntryRunner", "user_id", userID)
		instance.ChainEntryRunner.Stop()
	}

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

	// Stop all running autopilots, chain entry runners, coin profilers, position controllers, and exit decision services
	// Shutdown order: PositionController -> ExitDecision -> CoinProfiler -> ChainEntryRunner -> Autopilot (reverse of startup)
	m.instances.Range(func(key, value any) bool {
		userID := key.(string)
		instance := value.(*UserAutopilotInstance)

		// Story 10.4: Stop PositionController first (it consumes exit signals)
		if instance.IsPositionControllerRunning() {
			m.logger.Info("Stopping position controller for user during shutdown", "user_id", userID)
			instance.PositionController.Stop()
		}

		// Epic 14: Stop ExitDecisionService (it depends on CoinProfiler and positions)
		if instance.IsExitDecisionRunning() {
			m.logger.Info("Stopping exit decision service for user during shutdown", "user_id", userID)
			instance.ExitDecisionService.Stop()
		}

		// Epic 14: Stop CoinProfiler during shutdown
		if instance.IsCoinProfilerRunning() {
			m.logger.Info("Stopping coin profiler for user during shutdown", "user_id", userID)
			instance.CoinProfiler.Stop()
		}

		if instance.IsChainEntryRunnerRunning() {
			m.logger.Info("Stopping chain entry runner for user during shutdown", "user_id", userID)
			instance.ChainEntryRunner.Stop()
		}

		if instance.IsRunning() {
			m.logger.Info("Stopping autopilot for user during shutdown", "user_id", userID)
			instance.Autopilot.Stop()
		}
		return true
	})

	m.logger.Info("UserAutopilotManager shutdown complete")
}

// StartChainEntryRunner starts the chain entry runner for a specific user
func (m *UserAutopilotManager) StartChainEntryRunner(ctx context.Context, userID string) error {
	instance, err := m.GetOrCreateInstance(ctx, userID)
	if err != nil {
		return err
	}

	if instance.ChainEntryRunner == nil {
		return fmt.Errorf("chain entry runner not initialized for user %s", userID)
	}

	if instance.ChainEntryRunner.IsRunning() {
		return nil // Already running
	}

	m.logger.Info("Starting chain entry runner for user", "user_id", userID)
	instance.ChainEntryRunner.Start()
	instance.TouchLastActive()

	return nil
}

// StopChainEntryRunner stops the chain entry runner for a specific user
func (m *UserAutopilotManager) StopChainEntryRunner(userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil // Nothing to stop
	}

	if instance.ChainEntryRunner == nil {
		return nil // No chain entry runner
	}

	if !instance.ChainEntryRunner.IsRunning() {
		return nil // Already stopped
	}

	m.logger.Info("Stopping chain entry runner for user", "user_id", userID)
	instance.ChainEntryRunner.Stop()
	instance.TouchLastActive()

	return nil
}

// GetChainEntryRunnerStatus returns the status of the chain entry runner for a user
func (m *UserAutopilotManager) GetChainEntryRunnerStatus(userID string) *ChainEntryRunnerStatus {
	instance := m.GetInstance(userID)
	if instance == nil {
		return &ChainEntryRunnerStatus{
			UserID:  userID,
			Running: false,
			Message: "No autopilot instance",
		}
	}

	if instance.ChainEntryRunner == nil {
		return &ChainEntryRunnerStatus{
			UserID:  userID,
			Running: false,
			Message: "Chain entry runner not initialized",
		}
	}

	stats := instance.ChainEntryRunner.GetStats()

	return &ChainEntryRunnerStatus{
		UserID:            userID,
		Running:           instance.ChainEntryRunner.IsRunning(),
		TotalScans:        stats.TotalScans,
		TotalEntries:      stats.TotalEntries,
		SuccessfulEntries: stats.SuccessfulEntries,
		FailedEntries:     stats.FailedEntries,
		LastScanTime:      stats.LastScanTime,
		LastEntryTime:     stats.LastEntryTime,
	}
}

// ChainEntryRunnerStatus provides status for a user's chain entry runner
type ChainEntryRunnerStatus struct {
	UserID            string    `json:"user_id"`
	Running           bool      `json:"running"`
	TotalScans        int       `json:"total_scans"`
	TotalEntries      int       `json:"total_entries"`
	SuccessfulEntries int       `json:"successful_entries"`
	FailedEntries     int       `json:"failed_entries"`
	LastScanTime      time.Time `json:"last_scan_time,omitempty"`
	LastEntryTime     time.Time `json:"last_entry_time,omitempty"`
	Message           string    `json:"message,omitempty"`
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
	wasAutopilotRunning := instance.IsRunning()
	if wasAutopilotRunning {
		instance.Autopilot.Stop()
	}

	// Stop chain entry runner if running
	wasChainRunnerRunning := instance.IsChainEntryRunnerRunning()
	if wasChainRunnerRunning {
		instance.ChainEntryRunner.Stop()
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
	if instance.ChainEntryRunner != nil {
		instance.ChainEntryRunner.SetFuturesClient(newClient)
	}
	instance.mu.Unlock()

	// Restart if was running
	if wasAutopilotRunning {
		instance.Autopilot.Start()
	}
	if wasChainRunnerRunning && instance.ChainEntryRunner != nil {
		instance.ChainEntryRunner.Start()
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

// AutoStartFromSettings checks the database for users with auto-start enabled and starts their autopilots
// This should be called after server initialization to restore Ginie state from before restart
// NOTE: Only auto-starts if entry_decision_system is "legacy" - chain mode cannot place trades via legacy autopilot
func (m *UserAutopilotManager) AutoStartFromSettings(ctx context.Context) error {
	// Query database for users with auto_start = true
	if m.repo == nil {
		m.logger.Warn("Repository not available for auto-start check")
		return nil
	}

	// Get all users with auto-start enabled from the database
	users, err := m.repo.GetUsersWithAutoStartEnabled(ctx)
	if err != nil {
		m.logger.Warn("Failed to query users with auto-start enabled", "error", err)
		return nil
	}

	if len(users) == 0 {
		m.logger.Info("No users with auto-start enabled, skipping")
		return nil
	}

	// Start autopilot for each user with auto-start enabled
	var startErrors []error
	for _, userID := range users {
		// Check system control settings to determine which systems to start
		systemControl, err := m.repo.GetUserSystemControlOrDefault(ctx, userID)
		if err != nil {
			m.logger.Warn("Failed to get system control settings for auto-start check",
				"user_id", userID,
				"error", err)
			// Continue with auto-start on error (fail-open for backwards compatibility)
		}

		isChainMode := systemControl != nil && systemControl.IsEntryDecisionChain() && !systemControl.IsEntryDecisionLegacy()

		// Always start Ginie Autopilot for SCANNING purposes
		// When entry_decision_system = "chain", Ginie scans but doesn't place orders (blocked at entry time)
		// When entry_decision_system = "legacy" or "both", Ginie scans AND places orders
		m.logger.Info("Auto-starting Ginie from database settings",
			"user_id", userID,
			"auto_start", true,
			"entry_decision_system", systemControl.EntryDecisionSystem,
			"chain_mode", isChainMode)

		if err := m.StartAutopilot(ctx, userID); err != nil {
			m.logger.Error("Failed to auto-start Ginie for user",
				"user_id", userID,
				"error", err)
			startErrors = append(startErrors, fmt.Errorf("user %s: %w", userID, err))
			continue
		}

		m.logger.Info("Ginie auto-started successfully (scanning enabled)",
			"user_id", userID)

		// When chain mode is active, also start ChainEntryRunner for order execution
		if isChainMode {
			if err := m.StartChainEntryRunner(ctx, userID); err != nil {
				m.logger.Warn("Failed to start ChainEntryRunner (scanning will continue via Ginie)",
					"user_id", userID,
					"error", err)
			} else {
				m.logger.Info("ChainEntryRunner auto-started for chain-based entries",
					"user_id", userID)
			}
		}
	}

	if len(startErrors) > 0 {
		return fmt.Errorf("failed to auto-start Ginie for %d user(s): %v", len(startErrors), startErrors[0])
	}

	return nil
}

// ==================== Epic 14: CoinProfiler Management ====================

// StartCoinProfiler starts the coin profiler for a specific user
func (m *UserAutopilotManager) StartCoinProfiler(ctx context.Context, userID string) error {
	instance, err := m.GetOrCreateInstance(ctx, userID)
	if err != nil {
		return err
	}

	if instance.CoinProfiler == nil {
		return fmt.Errorf("coin profiler not initialized for user %s", userID)
	}

	if instance.CoinProfiler.IsRunning() {
		return nil // Already running
	}

	m.logger.Info("Starting coin profiler for user", "user_id", userID)
	if err := instance.CoinProfiler.Start(); err != nil {
		return fmt.Errorf("failed to start coin profiler: %w", err)
	}
	instance.TouchLastActive()

	return nil
}

// StopCoinProfiler stops the coin profiler for a specific user
func (m *UserAutopilotManager) StopCoinProfiler(userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil // Nothing to stop
	}

	if instance.CoinProfiler == nil {
		return nil // No coin profiler
	}

	if !instance.CoinProfiler.IsRunning() {
		return nil // Already stopped
	}

	m.logger.Info("Stopping coin profiler for user", "user_id", userID)
	if err := instance.CoinProfiler.Stop(); err != nil {
		return fmt.Errorf("failed to stop coin profiler: %w", err)
	}
	instance.TouchLastActive()

	return nil
}

// GetCoinProfilerStatus returns the status of the coin profiler for a user
func (m *UserAutopilotManager) GetCoinProfilerStatus(userID string) *coinprofiler.CoinProfilerStatus {
	instance := m.GetInstance(userID)
	if instance == nil {
		return &coinprofiler.CoinProfilerStatus{
			Running: false,
		}
	}

	if instance.CoinProfiler == nil {
		return &coinprofiler.CoinProfilerStatus{
			Running:   false,
			LastError: "Coin profiler not initialized",
		}
	}

	return instance.CoinProfiler.GetStatus()
}

// GetCoinProfiler returns the coin profiler instance for a user (for advanced operations)
func (m *UserAutopilotManager) GetCoinProfiler(userID string) *coinprofiler.CoinProfiler {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil
	}
	return instance.CoinProfiler
}

// ==================== Epic 14: ExitDecisionService Management ====================

// StartExitDecisionService starts the exit decision service for a specific user.
// This service monitors positions and generates exit signals even when Trading is OFF.
func (m *UserAutopilotManager) StartExitDecisionService(ctx context.Context, userID string) error {
	instance, err := m.GetOrCreateInstance(ctx, userID)
	if err != nil {
		return err
	}

	if instance.ExitDecisionService == nil {
		return fmt.Errorf("exit decision service not initialized for user %s", userID)
	}

	if instance.ExitDecisionService.IsRunning() {
		return nil // Already running
	}

	m.logger.Info("Starting exit decision service for user", "user_id", userID)
	if err := instance.ExitDecisionService.Start(ctx); err != nil {
		return fmt.Errorf("failed to start exit decision service: %w", err)
	}
	instance.TouchLastActive()

	return nil
}

// StopExitDecisionService stops the exit decision service for a specific user.
func (m *UserAutopilotManager) StopExitDecisionService(userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil // Nothing to stop
	}

	if instance.ExitDecisionService == nil {
		return nil // No exit decision service
	}

	if !instance.ExitDecisionService.IsRunning() {
		return nil // Already stopped
	}

	m.logger.Info("Stopping exit decision service for user", "user_id", userID)
	if err := instance.ExitDecisionService.Stop(); err != nil {
		return fmt.Errorf("failed to stop exit decision service: %w", err)
	}
	instance.TouchLastActive()

	return nil
}

// GetExitDecisionStatus returns the status of the exit decision service for a user.
func (m *UserAutopilotManager) GetExitDecisionStatus(userID string) *exitdecision.ServiceStatus {
	instance := m.GetInstance(userID)
	if instance == nil {
		return &exitdecision.ServiceStatus{
			Running: false,
		}
	}

	if instance.ExitDecisionService == nil {
		return &exitdecision.ServiceStatus{
			Running:   false,
			LastError: "Exit decision service not initialized",
		}
	}

	return instance.ExitDecisionService.GetStatus()
}

// GetExitDecisionService returns the exit decision service instance for a user.
func (m *UserAutopilotManager) GetExitDecisionService(userID string) *exitdecision.Service {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil
	}
	return instance.ExitDecisionService
}

// GetExitSignals returns pending exit signals for a user.
func (m *UserAutopilotManager) GetExitSignals(ctx context.Context, userID string) ([]exitdecision.ExitSignal, error) {
	instance := m.GetInstance(userID)
	if instance == nil {
		return []exitdecision.ExitSignal{}, nil
	}

	if instance.ExitDecisionService == nil {
		return []exitdecision.ExitSignal{}, nil
	}

	return instance.ExitDecisionService.GetExitSignals(ctx, userID)
}

// ==================== Story 10.4: PositionController Management ====================

// StartPositionController starts the position controller for a specific user.
// The PositionController executes exit signals from ExitDecisionService on Binance.
// It should only be started when entry_decision_system = "chain".
func (m *UserAutopilotManager) StartPositionController(ctx context.Context, userID string) error {
	instance, err := m.GetOrCreateInstance(ctx, userID)
	if err != nil {
		return err
	}

	if instance.PositionController == nil {
		return fmt.Errorf("position controller not initialized for user %s", userID)
	}

	if instance.PositionController.IsRunning() {
		return nil // Already running
	}

	m.logger.Info("Starting position controller for user", "user_id", userID)
	if err := instance.PositionController.Start(ctx); err != nil {
		return fmt.Errorf("failed to start position controller: %w", err)
	}
	instance.TouchLastActive()

	return nil
}

// StopPositionController stops the position controller for a specific user.
func (m *UserAutopilotManager) StopPositionController(userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil // Nothing to stop
	}

	if instance.PositionController == nil {
		return nil // No position controller
	}

	if !instance.PositionController.IsRunning() {
		return nil // Already stopped
	}

	m.logger.Info("Stopping position controller for user", "user_id", userID)
	if err := instance.PositionController.Stop(); err != nil {
		return fmt.Errorf("failed to stop position controller: %w", err)
	}
	instance.TouchLastActive()

	return nil
}

// GetPositionControllerStatus returns the status of the position controller for a user.
func (m *UserAutopilotManager) GetPositionControllerStatus(userID string) *PositionControllerStatus {
	instance := m.GetInstance(userID)
	if instance == nil {
		return &PositionControllerStatus{
			Running: false,
		}
	}

	if instance.PositionController == nil {
		return &PositionControllerStatus{
			Running:   false,
			LastError: "Position controller not initialized",
		}
	}

	return instance.PositionController.GetStatus()
}

// GetPositionController returns the position controller instance for a user.
func (m *UserAutopilotManager) GetPositionController(userID string) *PositionController {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil
	}
	return instance.PositionController
}

// TriggerPositionControllerHeal triggers an immediate protection heal check for a user.
func (m *UserAutopilotManager) TriggerPositionControllerHeal(userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return fmt.Errorf("no autopilot instance for user %s", userID)
	}

	if instance.PositionController == nil {
		return fmt.Errorf("position controller not initialized for user %s", userID)
	}

	return instance.PositionController.HealNow()
}
