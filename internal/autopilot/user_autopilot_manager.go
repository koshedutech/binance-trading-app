package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/apikeys"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/coinprofiler"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/entrydecision"
	"binance-trading-bot/internal/exitdecision"
	"binance-trading-bot/internal/logging"
	"binance-trading-bot/internal/orders"
	"context"
	"encoding/json"
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
// - RealtimePatternMatcher (evaluates patterns on candle close, triggers Entry Decision updates)
// - ExitDecisionService (monitors positions for TP/SL/trailing stop exits)
// - PositionController (Story 10.4: executes exit signals on Binance)
// - RavindraPositionMonitor (R:R-based trailing stop: 1:2 breakeven, 1:3 profit lock)
type UserAutopilotInstance struct {
	UserID                  string
	FuturesClient           binance.FuturesClient
	LLMAnalyzer             *llm.Analyzer
	Autopilot               *GinieAutopilot
	ChainEntryRunner        *ChainEntryRunner                       // Automatic chain entries (independent of Ginie)
	CoinProfiler            *coinprofiler.CoinProfiler              // Epic 14: Real-time data collection
	RealtimePatternMatcher  *entrydecision.RealtimePatternMatcher   // Epic 14: Pattern evaluation on candle close
	ExitDecisionService     *exitdecision.Service                   // Epic 14: Exit signal monitoring
	PositionController      *PositionController                     // Story 10.4: Exit signal executor
	RavindraPositionMonitor *RavindraPositionMonitor                // R:R-based trailing stop management
	CreatedAt               time.Time
	LastActive              time.Time

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

// IsRavindraMonitorRunning returns whether this user's Ravindra position monitor is active
func (u *UserAutopilotInstance) IsRavindraMonitorRunning() bool {
	if u.RavindraPositionMonitor == nil {
		return false
	}
	return u.RavindraPositionMonitor.IsRunning()
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

	// Epic 14: Callback for real-time coin data updates (set by API layer)
	coinUpdateCallback coinprofiler.CoinUpdateCallback

	// Epic 14: Callback for pattern updates (set by API layer for WebSocket broadcasting)
	patternUpdateCallback entrydecision.PatternUpdateCallback

	// Epic 14: Callback for settings changes (set by API layer for WebSocket broadcasting)
	settingsChangeCallback SettingsChangeCallback

	// Epic 14: Callback to clear entry decision UI when profiler stops
	entryDecisionClearCallback func(string)

	mu sync.RWMutex
}

// SettingsChangeCallback is the callback function type for settings change notifications.
// Used for WebSocket broadcasting when settings are modified.
type SettingsChangeCallback func(userID, changeType, mode, strategyGroup, subStrategy string)

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

// SetCoinUpdateCallback sets the callback for real-time coin data updates (Epic 14)
// This is called from the API server to enable WebSocket broadcasting of coin updates.
func (m *UserAutopilotManager) SetCoinUpdateCallback(callback coinprofiler.CoinUpdateCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.coinUpdateCallback = callback
	m.logger.Info("CoinUpdateCallback set on UserAutopilotManager for real-time coin streaming")

	// Propagate to existing CoinProfilers
	m.instances.Range(func(key, value interface{}) bool {
		instance := value.(*UserAutopilotInstance)
		if instance.CoinProfiler != nil {
			instance.CoinProfiler.SetCoinUpdateCallback(callback)
		}
		return true
	})
}

// SetPatternUpdateCallback sets the callback for pattern updates (Epic 14)
// This is called from the API server to enable WebSocket broadcasting of pattern updates.
// When patterns change state (e.g., volume spike detected → consolidation → breakout),
// the callback is triggered to broadcast the update to connected clients.
func (m *UserAutopilotManager) SetPatternUpdateCallback(callback entrydecision.PatternUpdateCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patternUpdateCallback = callback
	m.logger.Info("PatternUpdateCallback set on UserAutopilotManager for real-time pattern updates")

	// Propagate to existing RealtimePatternMatchers
	m.instances.Range(func(key, value interface{}) bool {
		userID := key.(string)
		instance := value.(*UserAutopilotInstance)
		if instance.RealtimePatternMatcher != nil {
			instance.RealtimePatternMatcher.SetPatternUpdateCallback(callback)
			m.logger.Debug("Propagated PatternUpdateCallback to user", "user_id", userID)
		}
		return true
	})
}

// SetSettingsChangeCallback sets the callback for settings change notifications (Epic 14)
// This is called from the API server to enable WebSocket broadcasting of settings changes.
// When settings are modified (sub-strategy, strategy group, or mode), the callback is
// triggered to broadcast the update to connected clients for real-time UI refresh.
func (m *UserAutopilotManager) SetSettingsChangeCallback(callback SettingsChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settingsChangeCallback = callback
	m.logger.Info("SettingsChangeCallback set on UserAutopilotManager for real-time settings notifications")
}

// SetEntryDecisionClearCallback sets the callback invoked when the coin profiler stops.
// This clears stale entry decision data from caches and notifies the frontend via WebSocket.
func (m *UserAutopilotManager) SetEntryDecisionClearCallback(callback func(string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entryDecisionClearCallback = callback
	m.logger.Info("EntryDecisionClearCallback set on UserAutopilotManager")
}

// NotifySettingsChanged notifies the autopilot system that settings have changed.
// This triggers a reload of relevant components based on what changed.
//
// Parameters:
//   - ctx: Context for database operations
//   - userID: The user whose settings changed
//   - changeType: Type of change - "sub_strategy", "strategy_group", or "mode"
//   - mode: Trading mode (scalp, swing, position) - may be empty for mode-level changes
//   - strategyGroup: Strategy group name - may be empty for mode/strategy_group level changes
//   - subStrategy: Sub-strategy name - may be empty for higher-level changes
//
// Actions:
//   - Reloads pattern matcher config when sub-strategy settings change
//   - Refreshes CoinProfiler subscriptions when strategies are enabled/disabled
func (m *UserAutopilotManager) NotifySettingsChanged(ctx context.Context, userID string, changeType string, mode string, strategyGroup string, subStrategy string) {
	m.logger.Info("Settings change notification received",
		"user_id", userID,
		"change_type", changeType,
		"mode", mode,
		"strategy_group", strategyGroup,
		"sub_strategy", subStrategy)

	// Get the user instance
	instance := m.GetInstance(userID)
	if instance == nil {
		m.logger.Debug("No autopilot instance for user, skipping settings notification", "user_id", userID)
		return
	}

	// Reload pattern matcher config if RealtimePatternMatcher exists
	// This ensures pattern detection uses the latest user settings
	if instance.RealtimePatternMatcher != nil {
		if err := m.ReloadPatternMatcherConfig(ctx, userID); err != nil {
			m.logger.Error("Failed to reload pattern matcher config",
				"user_id", userID,
				"error", err)
		} else {
			m.logger.Info("Pattern matcher config reloaded successfully",
				"user_id", userID,
				"change_type", changeType)
		}
	}

	// Refresh CoinProfiler subscriptions if running
	// This handles strategy enable/disable affecting which symbols to monitor
	if instance.CoinProfiler != nil && instance.CoinProfiler.IsRunning() {
		count, err := m.RefreshCoinProfilerSubscriptions(ctx, userID)
		if err != nil {
			m.logger.Error("Failed to refresh CoinProfiler subscriptions",
				"user_id", userID,
				"error", err)
		} else {
			m.logger.Info("CoinProfiler subscriptions refreshed",
				"user_id", userID,
				"enabled_strategies", count)
		}
	}

	// Broadcast settings change via WebSocket for real-time UI updates
	// This is called AFTER all internal processing is complete
	m.mu.RLock()
	callback := m.settingsChangeCallback
	m.mu.RUnlock()

	if callback != nil {
		callback(userID, changeType, mode, strategyGroup, subStrategy)
		m.logger.Debug("Settings change callback triggered",
			"user_id", userID,
			"change_type", changeType)
	}

	m.logger.Info("Settings change notification processed",
		"user_id", userID,
		"change_type", changeType)
}

// ReloadPatternMatcherConfig reloads the pattern matcher configuration from database settings.
// This should be called when sub-strategy settings change to ensure the pattern matcher
// uses the latest user-configured values (direction, thresholds, etc.)
func (m *UserAutopilotManager) ReloadPatternMatcherConfig(ctx context.Context, userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return fmt.Errorf("no autopilot instance for user %s", userID)
	}

	if instance.RealtimePatternMatcher == nil {
		return fmt.Errorf("realtime pattern matcher not initialized for user %s", userID)
	}

	// Load fresh config from database using existing method
	config := m.loadUserPatternMatcherConfig(ctx, userID)

	// Update the pattern matcher with new config
	instance.RealtimePatternMatcher.ReloadPatternMatcherConfig(config)

	m.logger.Info("Reloaded pattern matcher config",
		"user_id", userID,
		"direction", config.Direction,
		"min_volume_spike_multiplier", config.MinVolumeSpikeMultiplier)

	return nil
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

	// Epic 11/14: Create ChainEntryRunner for automatic chain-based entries
	// This runs independently of GinieAutopilot when entry_decision_system = "chain"
	// NOTE: StateProvider is wired after RealtimePatternMatcher is created (see below)
	chainEntryRunner := NewChainEntryRunner(
		userID,
		nil, // StateProvider wired later via PatternStateProvider
		futuresClient,
		m.chainEventWriter,
		m.settingsCache,
		m.repo,
		m.logger,
		nil, // Use default config
	)
	m.logger.Info("ChainEntryRunner created for user", "user_id", userID)

	// Epic 14: Create CoinProfiler for real-time WebSocket data collection
	coinProfiler := coinprofiler.NewCoinProfiler(nil, m.logger) // Use default config
	// Set real-time update callback if configured (for WebSocket broadcasting)
	if m.coinUpdateCallback != nil {
		coinProfiler.SetCoinUpdateCallback(m.coinUpdateCallback)
	}

	// Epic 14: Set historical data provider for prefetching candles on startup
	// This enables pattern detection to work immediately without waiting for candles to close
	if futuresClient != nil {
		adapter := coinprofiler.NewFuturesClientAdapter(func(symbol, interval string, limit int) ([]coinprofiler.HistoricalKline, error) {
			klines, err := futuresClient.GetFuturesKlines(symbol, interval, limit)
			if err != nil {
				return nil, err
			}
			result := make([]coinprofiler.HistoricalKline, len(klines))
			for i, k := range klines {
				result[i] = coinprofiler.HistoricalKline{
					OpenTime:                 k.OpenTime,
					Open:                     k.Open,
					High:                     k.High,
					Low:                      k.Low,
					Close:                    k.Close,
					Volume:                   k.Volume,
					CloseTime:                k.CloseTime,
					QuoteAssetVolume:         k.QuoteAssetVolume,
					NumberOfTrades:           k.NumberOfTrades,
					TakerBuyBaseAssetVolume:  k.TakerBuyBaseAssetVolume,
					TakerBuyQuoteAssetVolume: k.TakerBuyQuoteAssetVolume,
				}
			}
			return result, nil
		})
		coinProfiler.SetHistoricalDataProvider(adapter)
		m.logger.Info("Historical data provider set on CoinProfiler", "user_id", userID)
	}

	m.logger.Info("CoinProfiler created for user", "user_id", userID)

	// Epic 14: Create RealtimePatternMatcher for Entry Decision pattern evaluation
	// This evaluates patterns on candle close events and triggers Entry Decision updates
	//
	// IMPORTANT: We load user's pattern matcher config from database settings to ensure
	// configured values (direction, volume thresholds, etc.) are respected instead of defaults.
	patternMatcherConfig := m.loadUserPatternMatcherConfig(ctx, userID)
	patternMatcher := entrydecision.NewVolumeImbalancePatternMatcher(patternMatcherConfig)
	realtimeMatcher := entrydecision.NewRealtimePatternMatcher(patternMatcher, nil)
	// Set userID for targeted broadcasts
	realtimeMatcher.SetUserID(userID)
	// Wire pattern state persistence for restart recovery
	if m.repo != nil {
		persister := NewPatternStatePersisterAdapter(m.repo)
		realtimeMatcher.SetPersister(persister)
		// Restore any previously saved pattern states from DB
		realtimeMatcher.RestorePatternStates()
		m.logger.Info("Pattern state persistence wired and restored from DB", "user_id", userID)
	}
	// Register with CoinProfiler to receive candle close events
	realtimeMatcher.RegisterWithCoinProfiler(coinProfiler)
	// Wire pattern update callback for real-time WebSocket broadcasting
	if m.patternUpdateCallback != nil {
		realtimeMatcher.SetPatternUpdateCallback(m.patternUpdateCallback)
		m.logger.Info("PatternUpdateCallback wired to RealtimePatternMatcher", "user_id", userID)
	}
	m.logger.Info("RealtimePatternMatcher created and registered with CoinProfiler", "user_id", userID)

	// Epic 14: Wire PatternStateProvider to ChainEntryRunner
	// This is the critical bridge between pattern detection and order execution.
	// When a pattern becomes "ready", ChainEntryRunner will automatically place orders.
	patternStateProvider := NewPatternStateProvider(realtimeMatcher, userID)
	// Wire repository for strategy settings access (budget, leverage, SL/TP)
	if m.repo != nil {
		patternStateProvider.SetRepository(m.repo)
	}
	chainEntryRunner.SetStateProvider(patternStateProvider)
	m.logger.Info("PatternStateProvider wired to ChainEntryRunner for automatic order execution", "user_id", userID)

	// Epic 14: Wire immediate breakout callback for instant order execution
	// This enables orders to be placed the MOMENT price breaks out, not waiting for scan cycle.
	realtimeMatcher.SetBreakoutCallback(func(symbol, direction, mode, strategyGroup, subStrategy, timeframe string, price float64) {
		if err := chainEntryRunner.ExecuteImmediateEntry(symbol, direction, mode, strategyGroup, subStrategy, timeframe, price); err != nil {
			m.logger.Error("Immediate breakout entry failed", "symbol", symbol, "direction", direction, "timeframe", timeframe, "error", err)
		}
	})
	m.logger.Info("Breakout callback wired for immediate order execution", "user_id", userID)

	// Epic 14: Wire capacity checker for proactive limit enforcement
	// This is called BEFORE triggering breakout to check if new entries are allowed.
	// When max_concurrent_trades is reached, breakouts are detected but orders are NOT placed.
	// Pattern matching continues (for UI display) but entry is blocked at source.
	realtimeMatcher.SetCapacityChecker(func() (bool, int, int) {
		ctx := context.Background()
		activeChains, err := m.chainEventWriter.GetActiveChains(ctx, userID)
		if err != nil {
			m.logger.Error("Capacity check failed - allowing entry", "error", err)
			return true, 0, 1 // Default to allowing if check fails
		}
		currentCount := len(activeChains)

		// Get max_concurrent_trades from sub-strategy settings (breakout/ravindra_volume_imbalance)
		maxConcurrentTrades := 1 // Default
		if m.repo != nil {
			subSettings, err := m.repo.GetSubStrategySettings(ctx, userID, "scalp", "breakout", "ravindra_volume_imbalance")
			if err == nil && subSettings != nil && len(subSettings.Settings) > 0 {
				var settingsMap map[string]interface{}
				if err := json.Unmarshal(subSettings.Settings, &settingsMap); err == nil {
					if budgetAlloc, ok := settingsMap["budget_allocation"].(map[string]interface{}); ok {
						if maxTrades, ok := budgetAlloc["max_concurrent_trades"].(float64); ok && maxTrades > 0 {
							maxConcurrentTrades = int(maxTrades)
						}
					}
				}
			}
		}

		canEnter := currentCount < maxConcurrentTrades
		return canEnter, currentCount, maxConcurrentTrades
	})
	m.logger.Info("Capacity checker wired for proactive limit enforcement", "user_id", userID)

	// Epic 14: Wire entry placed callback for pattern cleanup
	// When an entry order is placed, clear the pattern so coin profiler can look for new entries
	chainEntryRunner.SetOnEntryPlacedCallback(func(symbol, mode, timeframe string) {
		m.logger.Info("Entry placed - clearing pattern for new entries",
			"symbol", symbol, "mode", mode, "timeframe", timeframe, "user_id", userID)
		realtimeMatcher.ClearPatternForSymbol(symbol, mode, timeframe)
	})
	m.logger.Info("Entry placed callback wired for pattern cleanup", "user_id", userID)

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

	// Create RavindraPositionMonitor for R:R-based trailing stop management
	// This implements the Ravindra strategy: move SL to breakeven at 1:2 R:R, lock 1R profit at 1:3 R:R
	// The monitor runs independently and checks positions periodically for milestone triggers
	priceProvider := &futuresClientPriceProvider{client: futuresClient}
	ravindraMonitor := NewRavindraPositionMonitor(
		futuresClient,
		m.chainEventWriter,
		priceProvider,
		DefaultRavindraPositionMonitorConfig(),
	)
	m.logger.Info("RavindraPositionMonitor created for user", "user_id", userID)

	// Wire RavindraPositionMonitor to ChainEntryRunner for automatic position registration
	// When ChainEntryRunner places an entry with SL/TP, it will register the position
	// with the monitor for R:R milestone tracking
	chainEntryRunner.SetRavindraMonitor(ravindraMonitor)
	m.logger.Info("RavindraPositionMonitor wired to ChainEntryRunner", "user_id", userID)

	instance := &UserAutopilotInstance{
		UserID:                  userID,
		FuturesClient:           futuresClient,
		LLMAnalyzer:             llmAnalyzer,
		Autopilot:               autopilot,
		ChainEntryRunner:        chainEntryRunner,
		CoinProfiler:            coinProfiler,        // Epic 14: Real-time data collection
		RealtimePatternMatcher:  realtimeMatcher,     // Epic 14: Pattern evaluation on candle close
		ExitDecisionService:     exitDecisionSvc,     // Epic 14: Exit signal monitoring
		PositionController:      positionController,  // Story 10.4: Exit signal executor
		RavindraPositionMonitor: ravindraMonitor,     // R:R-based trailing stop management
		CreatedAt:               time.Now(),
		LastActive:              time.Now(),
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
	// AC10.4.5: Use StartAutopilot to ensure consistent chain/legacy mode handling
	if m.repo != nil {
		tradingConfig, err := m.repo.GetUserTradingConfig(ctx, userID)
		if err == nil && tradingConfig != nil && tradingConfig.AutopilotEnabled {
			m.logger.Info("Per-user auto-start enabled, delegating to StartAutopilot",
				"user_id", userID,
				"autopilot_enabled", tradingConfig.AutopilotEnabled)

			// Note: StartAutopilot checks if already running, so this is safe
			// It also handles chain mode properly (AC10.4.5)
			if err := m.StartAutopilot(ctx, userID); err != nil {
				m.logger.Error("Failed to auto-start autopilot",
					"user_id", userID,
					"error", err)
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
// AC10.4.5: When entry_decision_system = "chain":
//   - Ginie scans but position monitoring is DISABLED (SkipPositionMonitoring=true)
//   - Position Controller handles all position management (SL/TP updates)
//   - Chain system components are started: CoinProfiler, ExitDecisionService, PositionController, ChainEntryRunner
// When entry_decision_system = "legacy":
//   - Ginie runs normally with full position monitoring
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

	// AC10.4.5: Configure Ginie based on entry decision system
	if isChainMode {
		// Chain mode: Ginie scans only, Position Controller manages positions
		instance.Autopilot.SetSkipPositionMonitoring(true)
		m.logger.Info("╔════════════════════════════════════════════════════════════════╗",
			"user_id", userID)
		m.logger.Info("║  CHAIN MODE ACTIVE - Starting Chain Trading System             ║",
			"user_id", userID)
		m.logger.Info("║  Ginie: Scanning ENABLED, Position Monitoring DISABLED         ║",
			"user_id", userID)
		m.logger.Info("║  Position Controller: Will handle all SL/TP management         ║",
			"user_id", userID)
		m.logger.Info("╚════════════════════════════════════════════════════════════════╝",
			"user_id", userID)
	} else {
		// Legacy mode: Ginie handles everything
		instance.Autopilot.SetSkipPositionMonitoring(false)
		m.logger.Info("Starting autopilot in LEGACY mode (full Ginie position monitoring)",
			"user_id", userID)
	}

	// Start Ginie (scanning always enabled, position monitoring conditional)
	m.logger.Info("Starting Ginie autopilot",
		"user_id", userID,
		"chain_mode", isChainMode,
		"skip_position_monitoring", isChainMode)
	instance.Autopilot.Start()
	instance.TouchLastActive()

	// AC10.4.5: When chain mode is active, start all Chain Trading System components
	if isChainMode {
		// 1. Start CoinProfiler (real-time WebSocket data collection)
		if instance.CoinProfiler != nil && !instance.CoinProfiler.IsRunning() {
			// CRITICAL: Clear all pattern state before starting to prevent "pattern timeout" issues.
			// When the profiler restarts (e.g., after browser refresh), old patterns with stale
			// ExpiresAt timestamps would appear as "expired" when new candle data arrives.
			if instance.RealtimePatternMatcher != nil {
				instance.RealtimePatternMatcher.ClearAllPatterns()
				m.logger.Info("Cleared stale pattern state before CoinProfiler start (chain mode)", "user_id", userID)
			}

			m.logger.Info("Starting CoinProfiler for chain system", "user_id", userID)
			if err := instance.CoinProfiler.Start(); err != nil {
				m.logger.Error("Failed to start CoinProfiler", "user_id", userID, "error", err)
			} else {
				// Initialize WebSocket subscriptions based on enabled strategies
				m.initializeCoinProfilerSubscriptions(ctx, userID, instance)
			}
		}

		// 2. Start ExitDecisionService (monitors positions for exit signals)
		if instance.ExitDecisionService != nil && !instance.ExitDecisionService.IsRunning() {
			m.logger.Info("Starting ExitDecisionService for chain system", "user_id", userID)
			if err := instance.ExitDecisionService.Start(ctx); err != nil {
				m.logger.Error("Failed to start ExitDecisionService", "user_id", userID, "error", err)
			}
		}

		// 3. Start PositionController (executes exit signals on Binance)
		if instance.PositionController != nil && !instance.PositionController.IsRunning() {
			m.logger.Info("Starting PositionController for chain system", "user_id", userID)
			if err := instance.PositionController.Start(ctx); err != nil {
				m.logger.Error("Failed to start PositionController", "user_id", userID, "error", err)
			}
		}

		// 4. Start ChainEntryRunner (automatic chain-based entries)
		if instance.ChainEntryRunner != nil && !instance.ChainEntryRunner.IsRunning() {
			m.logger.Info("Starting ChainEntryRunner for chain system", "user_id", userID)
			instance.ChainEntryRunner.Start()
		}

		// 5. Start RavindraPositionMonitor (R:R-based trailing stop management)
		// This monitors positions for 1:2 and 1:3 R:R milestones and updates SL accordingly
		if instance.RavindraPositionMonitor != nil && !instance.RavindraPositionMonitor.IsRunning() {
			m.logger.Info("Starting RavindraPositionMonitor for chain system", "user_id", userID)
			instance.RavindraPositionMonitor.Start()
		}

		m.logger.Info("Chain Trading System fully started",
			"user_id", userID,
			"coin_profiler", instance.IsCoinProfilerRunning(),
			"exit_decision", instance.IsExitDecisionRunning(),
			"position_controller", instance.IsPositionControllerRunning(),
			"chain_entry_runner", instance.IsChainEntryRunnerRunning(),
			"ravindra_monitor", instance.IsRavindraMonitorRunning())
	}

	return nil
}

// StopAutopilot stops the autopilot for a specific user.
// AC10.4.5: Also stops all Chain Trading System components if running.
// Shutdown order: PositionController -> ExitDecision -> CoinProfiler -> ChainEntryRunner -> Ginie
func (m *UserAutopilotManager) StopAutopilot(userID string) error {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil // Nothing to stop
	}

	// AC10.4.5: Stop Chain Trading System components in reverse order of dependencies
	// 1. Stop PositionController first (it consumes exit signals)
	if instance.PositionController != nil && instance.PositionController.IsRunning() {
		m.logger.Info("Stopping PositionController", "user_id", userID)
		if err := instance.PositionController.Stop(); err != nil {
			m.logger.Error("Failed to stop PositionController", "user_id", userID, "error", err)
		}
	}

	// 2. Stop ExitDecisionService (produces exit signals)
	if instance.ExitDecisionService != nil && instance.ExitDecisionService.IsRunning() {
		m.logger.Info("Stopping ExitDecisionService", "user_id", userID)
		if err := instance.ExitDecisionService.Stop(); err != nil {
			m.logger.Error("Failed to stop ExitDecisionService", "user_id", userID, "error", err)
		}
	}

	// 3. Stop CoinProfiler (provides price data)
	if instance.CoinProfiler != nil && instance.CoinProfiler.IsRunning() {
		m.logger.Info("Stopping CoinProfiler", "user_id", userID)
		if err := instance.CoinProfiler.Stop(); err != nil {
			m.logger.Error("Failed to stop CoinProfiler", "user_id", userID, "error", err)
		}
	}

	// 4. Stop RavindraPositionMonitor (R:R-based trailing stop)
	if instance.RavindraPositionMonitor != nil && instance.RavindraPositionMonitor.IsRunning() {
		m.logger.Info("Stopping RavindraPositionMonitor", "user_id", userID)
		instance.RavindraPositionMonitor.Stop()
	}

	// 5. Stop ChainEntryRunner
	if instance.ChainEntryRunner != nil && instance.ChainEntryRunner.IsRunning() {
		m.logger.Info("Stopping ChainEntryRunner", "user_id", userID)
		instance.ChainEntryRunner.Stop()
	}

	// 6. Stop Ginie if running
	if instance.Autopilot.IsRunning() {
		m.logger.Info("Stopping Ginie autopilot", "user_id", userID)
		instance.Autopilot.Stop()
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

// IsCoinProfilerRunningForUser checks if a specific user's CoinProfiler is running.
// Used by API handlers to guard coin population - when profiler is not running,
// no coins should be shown (they would be stale from previous sessions).
func (m *UserAutopilotManager) IsCoinProfilerRunningForUser(userID string) bool {
	instance := m.GetInstance(userID)
	if instance == nil {
		return false
	}
	return instance.IsCoinProfilerRunning()
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
	// Shutdown order: PositionController -> ExitDecision -> CoinProfiler -> RavindraMonitor -> ChainEntryRunner -> Autopilot (reverse of startup)
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

		// Stop RavindraPositionMonitor (R:R-based trailing stop)
		if instance.IsRavindraMonitorRunning() {
			m.logger.Info("Stopping ravindra position monitor for user during shutdown", "user_id", userID)
			instance.RavindraPositionMonitor.Stop()
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

	// Calculate position limits from sub-strategy max_concurrent_trades
	ctx := context.Background()
	currentPositions, maxPositions := instance.ChainEntryRunner.CalculateMaxPositionsFromSubStrategies(ctx)

	return &ChainEntryRunnerStatus{
		UserID:            userID,
		Running:           instance.ChainEntryRunner.IsRunning(),
		TotalScans:        stats.TotalScans,
		TotalEntries:      stats.TotalEntries,
		SuccessfulEntries: stats.SuccessfulEntries,
		FailedEntries:     stats.FailedEntries,
		LastScanTime:      stats.LastScanTime,
		LastEntryTime:     stats.LastEntryTime,
		CurrentPositions:  currentPositions,
		MaxPositions:      maxPositions,
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
	// Position limits calculated from active sub-strategies' max_concurrent_trades
	CurrentPositions int `json:"current_positions"`
	MaxPositions     int `json:"max_positions"`
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
// AC10.4.5: StartAutopilot handles chain/legacy mode properly:
//   - Chain mode: Ginie scans only + all chain components (CoinProfiler, ExitDecision, PositionController, ChainEntryRunner)
//   - Legacy mode: Ginie handles everything (scanning + position management)
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
	// AC10.4.5: StartAutopilot handles chain/legacy mode automatically
	var startErrors []error
	for _, userID := range users {
		// Log the system control settings for debugging
		systemControl, err := m.repo.GetUserSystemControlOrDefault(ctx, userID)
		if err != nil {
			m.logger.Warn("Failed to get system control settings for auto-start check",
				"user_id", userID,
				"error", err)
		}

		entrySystem := "unknown"
		if systemControl != nil {
			entrySystem = systemControl.EntryDecisionSystem
		}

		m.logger.Info("Auto-starting from database settings",
			"user_id", userID,
			"auto_start", true,
			"entry_decision_system", entrySystem)

		// AC10.4.5: StartAutopilot handles everything:
		// - Sets SkipPositionMonitoring based on chain/legacy mode
		// - Starts Ginie (always for scanning)
		// - Starts chain components if chain mode (CoinProfiler, ExitDecision, PositionController, ChainEntryRunner)
		if err := m.StartAutopilot(ctx, userID); err != nil {
			m.logger.Error("Failed to auto-start for user",
				"user_id", userID,
				"error", err)
			startErrors = append(startErrors, fmt.Errorf("user %s: %w", userID, err))
			continue
		}

		m.logger.Info("Auto-start completed successfully",
			"user_id", userID,
			"entry_decision_system", entrySystem)
	}

	if len(startErrors) > 0 {
		return fmt.Errorf("failed to auto-start for %d user(s): %v", len(startErrors), startErrors[0])
	}

	return nil
}

// ==================== Epic 14: CoinProfiler Management ====================

// GetEnabledStrategiesCount returns the count of enabled strategies for a user.
// This can be used to validate before starting the Coin Profiler.
func (m *UserAutopilotManager) GetEnabledStrategiesCount(ctx context.Context, userID string) (int, error) {
	strategies, err := m.repo.GetEnabledStrategies(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get enabled strategies: %w", err)
	}
	return len(strategies), nil
}

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

	// CRITICAL: Clear all pattern state before starting to prevent "pattern timeout" issues.
	// When the profiler restarts (e.g., after browser refresh), old patterns with stale
	// ExpiresAt timestamps would appear as "expired" when new candle data arrives.
	// Clearing patterns ensures fresh detection with new timestamps.
	if instance.RealtimePatternMatcher != nil {
		instance.RealtimePatternMatcher.ClearAllPatterns()
		m.logger.Info("Cleared stale pattern state before CoinProfiler start", "user_id", userID)
	}

	if err := instance.CoinProfiler.Start(); err != nil {
		return fmt.Errorf("failed to start coin profiler: %w", err)
	}

	// Initialize WebSocket subscriptions based on enabled strategies and positions
	m.initializeCoinProfilerSubscriptions(ctx, userID, instance)

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

	// CRITICAL: Clear all pattern data from Entry Decision strategies
	// When Coin Profiler is stopped, all collected data becomes stale and meaningless.
	// The patterns must be cleared so that when the profiler restarts, fresh data
	// is collected and new patterns are detected from scratch.
	if instance.RealtimePatternMatcher != nil {
		m.logger.Info("Clearing all Entry Decision pattern data (CoinProfiler stopping)", "user_id", userID)
		instance.RealtimePatternMatcher.ClearAllPatterns()
	}

	if err := instance.CoinProfiler.Stop(); err != nil {
		return fmt.Errorf("failed to stop coin profiler: %w", err)
	}
	instance.TouchLastActive()

	// Notify frontend to clear stale entry decision data
	m.mu.RLock()
	clearCb := m.entryDecisionClearCallback
	m.mu.RUnlock()
	if clearCb != nil {
		m.logger.Info("Broadcasting entry decision clear signal", "user_id", userID)
		clearCb(userID)
	}

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

// RefreshCoinProfilerSubscriptions refreshes the Coin Profiler subscriptions for a user.
// This should be called when strategies are enabled/disabled or when trading state changes.
// Returns the count of enabled strategies after refresh.
//
// IMPORTANT: When trading is OFF, this method:
// 1. Clears all strategy-related pattern data from Entry Decision
// 2. Only subscribes to position symbols (for exit monitoring)
// 3. The cleared data must be re-collected when trading is turned back ON
func (m *UserAutopilotManager) RefreshCoinProfilerSubscriptions(ctx context.Context, userID string) (int, error) {
	instance := m.GetInstance(userID)
	if instance == nil {
		return 0, fmt.Errorf("no autopilot instance for user %s", userID)
	}

	if instance.CoinProfiler == nil {
		return 0, fmt.Errorf("coin profiler not initialized for user %s", userID)
	}

	// Check if profiler is running
	if !instance.CoinProfiler.IsRunning() {
		m.logger.Info("Coin profiler not running, skipping refresh", "user_id", userID)
		return 0, nil
	}

	// Check trading state to determine if we should clear strategy patterns
	tradingEnabled := false
	tradingController := GetTradingController()
	if tradingController != nil {
		enabled, err := tradingController.IsTradingEnabled(ctx, userID)
		if err != nil {
			m.logger.Warn("Failed to check trading state during refresh", "user_id", userID, "error", err)
		} else {
			tradingEnabled = enabled
		}
	}

	// CRITICAL: When trading is turned OFF, clear all strategy-related pattern data
	// This ensures stale patterns don't persist and cause incorrect entries when
	// trading is turned back ON. Fresh data collection is required after OFF→ON.
	if !tradingEnabled {
		if instance.RealtimePatternMatcher != nil {
			m.logger.Info("Trade Cycle OFF - clearing all Entry Decision strategy patterns", "user_id", userID)
			instance.RealtimePatternMatcher.ClearAllPatterns()
		}
	}

	m.logger.Info("Refreshing CoinProfiler subscriptions",
		"user_id", userID,
		"trading_enabled", tradingEnabled)

	// Re-initialize subscriptions based on current trading state and enabled strategies
	m.initializeCoinProfilerSubscriptions(ctx, userID, instance)

	// Get the count of enabled strategies (for return value)
	dbStrategies, err := m.repo.GetEnabledStrategies(ctx, userID)
	if err != nil {
		m.logger.Error("Failed to get enabled strategies count", "user_id", userID, "error", err)
		return 0, nil // Don't fail the refresh
	}

	return len(dbStrategies), nil
}

// initializeCoinProfilerSubscriptions aggregates strategy requirements and initializes WebSocket subscriptions.
// This is the key integration point that connects Entry Decision strategies to Coin Profiler data collection.
//
// Flow:
// 1. Get enabled strategies from database using Repository
// 2. Convert database types to coinprofiler types
// 3. Aggregate requirements using coinprofiler.AggregateRequirements
// 4. Get open positions from Ginie for exit monitoring
// 5. Combine strategy and position requirements
// 6. Subscribe to the required WebSocket streams
func (m *UserAutopilotManager) initializeCoinProfilerSubscriptions(ctx context.Context, userID string, instance *UserAutopilotInstance) {
	if instance == nil || instance.CoinProfiler == nil {
		return
	}

	m.logger.Info("Initializing CoinProfiler subscriptions", "user_id", userID)

	// Check if trading is enabled - this determines if we include strategy requirements
	tradingEnabled := false
	tradingController := GetTradingController()
	if tradingController != nil {
		enabled, err := tradingController.IsTradingEnabled(ctx, userID)
		if err != nil {
			m.logger.Warn("Failed to check trading state, assuming disabled", "user_id", userID, "error", err)
		} else {
			tradingEnabled = enabled
		}
	}

	// Initialize aggregated requirements - only include strategies if trading is ON
	var aggregatedReqs *coinprofiler.AggregatedRequirements

	if tradingEnabled {
		// Trading is ON - include strategy requirements for entry scanning
		// Step 1: Get enabled strategies from database
		dbStrategies, err := m.repo.GetEnabledStrategies(ctx, userID)
		if err != nil {
			m.logger.Error("Failed to get enabled strategies", "user_id", userID, "error", err)
			// Continue anyway - we can still subscribe for positions
			dbStrategies = []database.EnabledSubStrategy{}
		}
		m.logger.Info("Found enabled strategies (trading ON)", "user_id", userID, "count", len(dbStrategies))

		// Step 2: Convert database types to coinprofiler types
		cpStrategies := make([]coinprofiler.EnabledSubStrategy, 0, len(dbStrategies))
		for _, s := range dbStrategies {
			cpStrategies = append(cpStrategies, coinprofiler.EnabledSubStrategy{
				Mode:          s.Mode,
				StrategyGroup: s.StrategyGroup,
				SubStrategy:   s.SubStrategy,
			})
		}

		// Step 3: Get requirements for each strategy
		strategyReqs := coinprofiler.GetRequirementsForStrategies(cpStrategies)

		// Step 4: Aggregate all strategy requirements
		aggregatedReqs = coinprofiler.AggregateRequirements(strategyReqs)
		m.logger.Info("Aggregated strategy requirements",
			"user_id", userID,
			"strategies", aggregatedReqs.TotalStrategies,
			"timeframes", aggregatedReqs.AllTimeframes)
	} else {
		// Trading is OFF - skip strategy requirements, only monitor positions
		m.logger.Info("Trading is OFF - skipping strategy requirements, only monitoring positions", "user_id", userID)
		aggregatedReqs = &coinprofiler.AggregatedRequirements{
			AllTimeframes: []string{},
			AllDataFields: []string{},
			ByStrategy:    []coinprofiler.StrategyRequirements{},
		}
	}

	// Step 5: Get open positions for exit monitoring (always included)
	var positions []coinprofiler.Position
	if instance.Autopilot != nil {
		giniePositions := instance.Autopilot.GetPositions()
		for _, gp := range giniePositions {
			// Create an adapter to convert GiniePosition to coinprofiler.Position interface
			positions = append(positions, &giniePositionAdapter{pos: gp})
		}
	}
	m.logger.Info("Found open positions", "user_id", userID, "count", len(positions))

	// Step 6: Combine strategy and position requirements
	positionReqs := coinprofiler.GetPositionRequirements(positions)
	combinedReqs := coinprofiler.CombineRequirements(aggregatedReqs, positionReqs)

	// Step 6b: If trading is ON and we have timeframes but no symbols, add default watchlist
	// Only add default watchlist when trading is enabled (for entry scanning)
	if tradingEnabled && len(aggregatedReqs.AllTimeframes) > 0 && len(combinedReqs.AllSymbols) == 0 {
		defaultSymbols := []string{
			"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
			"ADAUSDT", "DOGEUSDT", "AVAXUSDT", "DOTUSDT", "MATICUSDT",
		}
		m.logger.Info("No symbols from positions, using default watchlist",
			"user_id", userID,
			"default_symbols", len(defaultSymbols))

		// Add default symbols with strategy timeframes
		for _, symbol := range defaultSymbols {
			stratRefs := make([]coinprofiler.StrategyRef, 0)
			for _, req := range aggregatedReqs.ByStrategy {
				stratRefs = append(stratRefs, coinprofiler.StrategyRef{
					Mode:        req.Mode,
					Strategy:    req.Strategy,
					SubStrategy: req.SubStrategy,
				})
			}
			combinedReqs.AddSymbolFromStrategy(symbol, stratRefs, aggregatedReqs.AllTimeframes, aggregatedReqs.AllDataFields)
		}
	}

	m.logger.Info("Combined requirements",
		"user_id", userID,
		"trading_enabled", tradingEnabled,
		"symbols", len(combinedReqs.AllSymbols),
		"timeframes", combinedReqs.AllTimeframes)

	// Step 7: Set subscriptions on CoinProfiler
	if err := instance.CoinProfiler.SetSubscriptionsFromCombined(combinedReqs); err != nil {
		m.logger.Error("Failed to set CoinProfiler subscriptions", "user_id", userID, "error", err)
		return
	}

	m.logger.Info("CoinProfiler subscriptions initialized successfully",
		"user_id", userID,
		"trading_enabled", tradingEnabled,
		"symbols", len(combinedReqs.AllSymbols),
		"strategies", aggregatedReqs.TotalStrategies,
		"positions", len(positions))

	// Step 8: Pattern evaluation is now triggered automatically by CoinProfiler
	// during PrefetchHistoricalCandles - as each symbol's data loads, the candle close
	// callback fires immediately, enabling pattern evaluation without waiting.
	m.logger.Info("CoinProfiler subscriptions complete - pattern evaluation will trigger as historical data loads", "user_id", userID)
}

// giniePositionAdapter adapts GiniePosition to the coinprofiler.Position interface.
type giniePositionAdapter struct {
	pos *GiniePosition
}

// futuresClientPriceProvider adapts FuturesClient to the PriceProvider interface
// required by RavindraPositionMonitor.
type futuresClientPriceProvider struct {
	client binance.FuturesClient
}

// GetMarkPrice implements the PriceProvider interface.
// It returns the mark price for a symbol from the Binance Futures API.
func (p *futuresClientPriceProvider) GetMarkPrice(symbol string) (float64, error) {
	if p.client == nil {
		return 0, fmt.Errorf("futures client is nil")
	}

	markPrice, err := p.client.GetMarkPrice(symbol)
	if err != nil {
		return 0, fmt.Errorf("failed to get mark price for %s: %w", symbol, err)
	}

	return markPrice.MarkPrice, nil
}

func (a *giniePositionAdapter) GetSymbol() string {
	if a.pos == nil {
		return ""
	}
	return a.pos.Symbol
}

func (a *giniePositionAdapter) GetMode() string {
	if a.pos == nil {
		return ""
	}
	// Convert GinieTradingMode to string
	return string(a.pos.Mode)
}

func (a *giniePositionAdapter) GetSide() string {
	if a.pos == nil {
		return ""
	}
	return a.pos.Side
}

func (a *giniePositionAdapter) HasTakeProfit() bool {
	if a.pos == nil {
		return false
	}
	// GiniePosition has TakeProfits slice - check if any levels exist
	return len(a.pos.TakeProfits) > 0
}

func (a *giniePositionAdapter) HasStopLoss() bool {
	if a.pos == nil {
		return false
	}
	// GiniePosition uses StopLoss field
	return a.pos.StopLoss > 0
}

func (a *giniePositionAdapter) IsTrailingActive() bool {
	if a.pos == nil {
		return false
	}
	return a.pos.TrailingActive
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

// GetRealtimePatternMatcher returns the realtime pattern matcher instance for a user.
// Used for accessing pattern updates, countdown timer, and direction info for Entry Decision UI.
func (m *UserAutopilotManager) GetRealtimePatternMatcher(userID string) *entrydecision.RealtimePatternMatcher {
	instance := m.GetInstance(userID)
	if instance == nil {
		return nil
	}
	return instance.RealtimePatternMatcher
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

// loadUserPatternMatcherConfig loads the user's Volume Imbalance pattern_detection settings
// from the database and converts them to PatternMatcherConfig.
// Falls back to DefaultPatternMatcherConfig if user settings cannot be loaded.
// This ensures the pattern matcher respects user-configured values (direction, thresholds, etc.)
// instead of using hard-coded defaults.
func (m *UserAutopilotManager) loadUserPatternMatcherConfig(ctx context.Context, userID string) *entrydecision.PatternMatcherConfig {
	if m.repo == nil {
		m.logger.Warn("Repository not available, using default pattern matcher config", "user_id", userID)
		return entrydecision.DefaultPatternMatcherConfig()
	}

	// Load Volume Imbalance sub-strategy settings for scalp mode (primary mode)
	// The settings are the same structure across modes
	subSettings, err := m.repo.GetSubStrategySettings(ctx, userID, "scalp", "breakout", "ravindra_volume_imbalance")
	if err != nil || subSettings == nil || len(subSettings.Settings) == 0 {
		m.logger.Debug("User settings not found, using default pattern matcher config", "user_id", userID, "error", err)
		return entrydecision.DefaultPatternMatcherConfig()
	}

	// Parse settings JSON and create config
	settings := entrydecision.ParseSubStrategySettingsJSON(subSettings.Settings)
	if settings == nil {
		m.logger.Warn("Failed to parse sub-strategy settings, using default", "user_id", userID)
		return entrydecision.DefaultPatternMatcherConfig()
	}

	config := entrydecision.NewPatternMatcherConfigFromSettings(settings)
	m.logger.Info("Loaded user pattern matcher config",
		"user_id", userID,
		"direction", config.Direction,
		"min_volume_spike_multiplier", config.MinVolumeSpikeMultiplier)

	return config
}
