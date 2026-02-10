// Package autopilot implements the Chain Entry Runner for automatic trade execution.
// This component runs independently of Ginie Autopilot when entry_decision_system = "chain".
package autopilot

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/logging"
	"binance-trading-bot/internal/orders"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ========== Chain Entry Types (local to avoid circular import with decision package) ==========

// ChainCoinState mirrors decision.CoinState for use in ChainEntryRunner.
// This avoids circular import with the decision package.
type ChainCoinState struct {
	Symbol          string
	Price           float64
	Regime          string // TRENDING, RANGING, VOLATILE, CONSOLIDATING
	ActiveStrategy  string
	Decision        string // READY, BLOCKED, PENDING
	ADX             float64
	ATR             float64
	RSI             float64
	EMA9            float64
	EMA21           float64
	Trend1H         string // BULLISH, BEARISH, NEUTRAL
	Trend15M        string // BULLISH, BEARISH, NEUTRAL
	ScoreTechnical  int
	ScoreContext    int
	ScoreLLM        int
	ScoreHistory    int
	ScoreFinal      int
	BlockingReasons []string

	// Strategy configuration fields (from sub-strategy settings)
	StrategyGroup    string  // e.g., "breakout"
	SubStrategy      string  // e.g., "ravindra_volume_imbalance"
	Timeframe        string  // e.g., "3m", "15m"
	Direction        string  // "LONG" or "SHORT" - set from pattern detection
	BudgetUSD        float64 // Position size budget in USD from strategy settings
	MaxLeverage      int     // Maximum leverage from strategy settings
	SLPercent        float64 // Stop loss percentage (max_sl_percent) - used as GATE during pattern detection
	TPPercent        float64 // R:R ratio for TP calculation (e.g., 4.0 for 1:4)

	// Incremental equity / position sizing fields
	UseIncrementalEquity bool    // Whether to use current equity for sizing
	MaxConcurrentTrades  int     // Max concurrent trades for this sub-strategy
	TradeCapital1x       float64 // 1x capital allocated for this trade (before leverage)

	// Absolute price levels from pattern detection (preferred over percentages)
	// These are calculated from Support Price (Consolidation Low) at runtime
	EntryPrice float64 // Entry price from pattern (reference high for long)
	SLPrice    float64 // Actual stop loss price (Support Price from pattern)
	TPPrice    float64 // Actual take profit price (Entry + Risk × R:R)
	RiskAmount float64 // Risk in price terms (Entry - SL for long)
}

// ChainStateProvider is an interface for getting coin states from Redis.
// This abstracts the decision.StateManager to avoid circular imports.
type ChainStateProvider interface {
	// GetAllCoinStates returns all coin states for a user
	GetAllChainCoinStates(ctx context.Context, userID string) ([]*ChainCoinState, error)

	// GetCoinStateForEntry returns a coin state for a specific symbol with actual SL/TP prices
	// from pattern detection. This is used by ExecuteImmediateEntry for tick-level breakout orders.
	GetCoinStateForEntry(ctx context.Context, userID, symbol, mode, timeframe string) (*ChainCoinState, error)
}

// ========== Configuration ==========

// ChainEntryRunnerConfig holds configuration for the chain entry runner
type ChainEntryRunnerConfig struct {
	// ScanInterval is how often to scan for entry candidates (default: 30s)
	ScanInterval time.Duration
	// ScoreThreshold is the minimum score to consider for entry (default: 55)
	ScoreThreshold int
	// EntryCooldown is the minimum time between entries for the same symbol (default: 5m)
	EntryCooldown time.Duration
	// MaxEntriesPerCycle is the maximum entries to execute in one scan cycle (default: 3)
	MaxEntriesPerCycle int
}

// DefaultChainEntryRunnerConfig returns the default configuration
func DefaultChainEntryRunnerConfig() *ChainEntryRunnerConfig {
	return &ChainEntryRunnerConfig{
		ScanInterval:       30 * time.Second,
		ScoreThreshold:     55,
		EntryCooldown:      5 * time.Minute,
		MaxEntriesPerCycle: 3,
	}
}

// ========== ChainEntryRunner ==========

// ChainEntryRunner executes automatic entries based on CoinState data from Redis.
// It runs independently of Ginie Autopilot when entry_decision_system = "chain".
type ChainEntryRunner struct {
	// Dependencies
	userID           string
	stateProvider    ChainStateProvider
	futuresClient    binance.FuturesClient
	chainEventWriter *orders.ChainEventWriter
	settingsCache    SettingsCacheReader
	repo             *database.Repository
	logger           *logging.Logger

	// Configuration
	config *ChainEntryRunnerConfig

	// Order ID Generator for sequential numbering (Epic 7)
	orderIdGenerator *orders.ClientOrderIdGenerator

	// Ravindra Position Monitor for trailing stop management
	ravindraMonitor *RavindraPositionMonitor

	// Callback for entry order placement (called after order is placed, before fill confirmation)
	// Parameters: symbol, mode, timeframe
	onEntryPlaced func(symbol, mode, timeframe string)

	// Callback for when order is placed and waiting for fill (Step 3 transition)
	// Parameters: symbol, mode, timeframe, orderPrice, orderQuantityUSD, fillTimeoutSeconds
	onEntryFilling func(symbol, mode, timeframe string, orderPrice, orderQuantityUSD float64, fillTimeoutSecs int)

	// Callback for when entry fails (order rejected, timeout, etc.) - resets pattern to watching
	// Parameters: symbol, mode, timeframe
	onEntryFailed func(symbol, mode, timeframe string)

	// Callback for fill progress updates (countdown timer for UI)
	// Parameters: symbol, mode, timeframe, remainingSeconds
	onFillProgress func(symbol, mode, timeframe string, remainingSecs int)

	// Callback for when entry order is filled successfully - clears pattern so it stays cleared
	// Parameters: symbol, mode, timeframe, filledQty, filledPrice
	onFillCompleted func(symbol, mode, timeframe string, filledQty, filledPrice float64)

	// Runtime state
	running        bool
	stopChan       chan struct{}
	entryCooldowns map[string]time.Time // symbol -> last entry time
	mu             sync.RWMutex

	// Position limit enforcement mutex - prevents race condition where
	// multiple goroutines read activeChains=0 simultaneously and all
	// place orders, exceeding the max_concurrent_trades limit.
	// This mutex must be held during the entire check-and-order sequence.
	limitMu sync.Mutex

	// Pending entries tracking - symbols that are in the process of entry
	// but haven't yet created their order chain in the database.
	// This prevents duplicate entries during the brief window between
	// passing the limit check and creating the chain record.
	pendingEntries map[string]bool
	pendingMu      sync.Mutex

	// Statistics
	stats ChainEntryRunnerStats
}

// ChainEntryRunnerStats holds statistics for the runner
type ChainEntryRunnerStats struct {
	TotalScans        int       `json:"total_scans"`
	TotalEntries      int       `json:"total_entries"`
	SuccessfulEntries int       `json:"successful_entries"`
	FailedEntries     int       `json:"failed_entries"`
	LastScanTime      time.Time `json:"last_scan_time"`
	LastEntryTime     time.Time `json:"last_entry_time"`
}

// NewChainEntryRunner creates a new ChainEntryRunner instance
func NewChainEntryRunner(
	userID string,
	stateProvider ChainStateProvider,
	futuresClient binance.FuturesClient,
	chainEventWriter *orders.ChainEventWriter,
	settingsCache SettingsCacheReader,
	repo *database.Repository,
	logger *logging.Logger,
	config *ChainEntryRunnerConfig,
) *ChainEntryRunner {
	if config == nil {
		config = DefaultChainEntryRunnerConfig()
	}

	// Create order ID generator for sequential numbering (Epic 7)
	// settingsCache implements SequenceProvider interface (IncrementDailySequence + IsHealthy)
	var orderIdGen *orders.ClientOrderIdGenerator
	if settingsCache != nil {
		var err error
		orderIdGen, err = orders.NewClientOrderIdGenerator(settingsCache, userID, nil)
		if err != nil {
			log.Printf("[CHAIN-ENTRY] Warning: Failed to create order ID generator: %v (will use fallback)", err)
		}
	}

	return &ChainEntryRunner{
		userID:           userID,
		stateProvider:    stateProvider,
		futuresClient:    futuresClient,
		chainEventWriter: chainEventWriter,
		settingsCache:    settingsCache,
		repo:             repo,
		logger:           logger,
		config:           config,
		orderIdGenerator: orderIdGen,
		entryCooldowns:   make(map[string]time.Time),
		pendingEntries:   make(map[string]bool),
		stopChan:         make(chan struct{}),
	}
}

// Start begins the chain entry runner background loop
func (r *ChainEntryRunner) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.stopChan = make(chan struct{})
	r.mu.Unlock()

	log.Printf("[CHAIN-ENTRY] ChainEntryRunner started for user %s", r.userID)
	go r.runLoop()
}

// Stop halts the chain entry runner
func (r *ChainEntryRunner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	r.running = false
	close(r.stopChan)
	log.Printf("[CHAIN-ENTRY] ChainEntryRunner stopped for user %s", r.userID)
}

// IsRunning returns whether the runner is active
// SetOnEntryPlacedCallback sets the callback for when an entry order is placed.
// This is called after the order is successfully placed, allowing cleanup of pattern state.
func (r *ChainEntryRunner) SetOnEntryPlacedCallback(callback func(symbol, mode, timeframe string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onEntryPlaced = callback
}

// SetOnEntryFillingCallback sets the callback for when an entry order is placed and waiting for fill.
// This transitions the pattern to "filling" status so the UI shows Step 3.
func (r *ChainEntryRunner) SetOnEntryFillingCallback(callback func(symbol, mode, timeframe string, orderPrice, orderQuantityUSD float64, fillTimeoutSecs int)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onEntryFilling = callback
}

// SetOnEntryFailedCallback sets the callback for when an entry order fails.
// This resets the pattern to "watching" so new entries can be detected.
func (r *ChainEntryRunner) SetOnEntryFailedCallback(callback func(symbol, mode, timeframe string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onEntryFailed = callback
}

// SetOnFillProgressCallback sets the callback for fill progress countdown updates.
// Called every 2 seconds during waitForLimitFill with remaining seconds until timeout.
func (r *ChainEntryRunner) SetOnFillProgressCallback(callback func(symbol, mode, timeframe string, remainingSecs int)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onFillProgress = callback
}

// SetOnFillCompletedCallback sets the callback for when an entry order fills successfully.
// This clears the pattern so it stays cleared and doesn't get re-created by candle close.
func (r *ChainEntryRunner) SetOnFillCompletedCallback(callback func(symbol, mode, timeframe string, filledQty, filledPrice float64)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onFillCompleted = callback
}

func (r *ChainEntryRunner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// GetStats returns current statistics
func (r *ChainEntryRunner) GetStats() ChainEntryRunnerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// runLoop is the main scanning loop
func (r *ChainEntryRunner) runLoop() {
	ticker := time.NewTicker(r.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.executeScanCycle()
		}
	}
}

// executeScanCycle performs one scan and entry cycle
func (r *ChainEntryRunner) executeScanCycle() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r.mu.Lock()
	r.stats.TotalScans++
	r.stats.LastScanTime = time.Now()
	r.mu.Unlock()

	// Check if chain entry is enabled
	if !r.isChainEntryEnabled(ctx) {
		return
	}

	// Check circuit breaker
	if r.isCircuitBreakerTripped(ctx) {
		log.Printf("[CHAIN-ENTRY] Circuit breaker tripped, skipping scan")
		return
	}

	// Get all coin states from Redis
	if r.stateProvider == nil {
		log.Printf("[CHAIN-ENTRY] StateProvider not available, skipping scan")
		return
	}

	states, err := r.stateProvider.GetAllChainCoinStates(ctx, r.userID)
	if err != nil {
		log.Printf("[CHAIN-ENTRY] Failed to get coin states: %v", err)
		return
	}

	if len(states) == 0 {
		return
	}

	// Filter and rank candidates
	candidates := r.filterAndRankCandidates(ctx, states)
	if len(candidates) == 0 {
		return
	}

	log.Printf("[CHAIN-ENTRY] Found %d candidates with score >= %d", len(candidates), r.config.ScoreThreshold)

	// Execute entries up to max per cycle
	entriesExecuted := 0
	for _, candidate := range candidates {
		if entriesExecuted >= r.config.MaxEntriesPerCycle {
			break
		}

		// Preliminary checks (mode enabled, dependencies available)
		if !r.canEnterPosition(ctx, candidate) {
			continue
		}

		// CRITICAL: Atomically reserve an entry slot to prevent race conditions
		// This ensures that multiple candidates cannot all pass the limit check
		// and place orders simultaneously, exceeding max_concurrent_trades.
		allowed, releaseSlot := r.tryReserveEntrySlot(ctx, candidate)
		if !allowed {
			continue
		}

		// Execute entry with the slot reserved
		if err := r.executeChainEntry(ctx, candidate); err != nil {
			log.Printf("[CHAIN-ENTRY] Failed to execute entry for %s: %v", candidate.Symbol, err)
			r.mu.Lock()
			r.stats.FailedEntries++
			r.mu.Unlock()
			releaseSlot() // Release the reserved slot on failure
		} else {
			entriesExecuted++
			r.mu.Lock()
			r.stats.SuccessfulEntries++
			r.stats.TotalEntries++
			r.stats.LastEntryTime = time.Now()
			r.mu.Unlock()
			releaseSlot() // Release the reserved slot on success (chain is now in DB)
		}
	}
}

// isChainEntryEnabled checks if chain entry system is enabled
func (r *ChainEntryRunner) isChainEntryEnabled(ctx context.Context) bool {
	if r.repo == nil {
		return false
	}

	systemControl, err := r.repo.GetUserSystemControlOrDefault(ctx, r.userID)
	if err != nil {
		log.Printf("[CHAIN-ENTRY] Failed to get system control: %v", err)
		return false
	}

	return systemControl.IsEntryDecisionChain()
}

// isCircuitBreakerTripped checks if the circuit breaker is active
func (r *ChainEntryRunner) isCircuitBreakerTripped(ctx context.Context) bool {
	if r.settingsCache == nil || !r.settingsCache.IsHealthy() {
		return false // Allow if we can't check
	}

	cbConfig, err := r.settingsCache.GetCircuitBreaker(ctx, r.userID)
	if err != nil {
		return false
	}

	if cbConfig == nil {
		return false
	}

	return cbConfig.IsTripped
}

// filterAndRankCandidates filters coin states by score and blocks, then sorts by score descending
func (r *ChainEntryRunner) filterAndRankCandidates(ctx context.Context, states []*ChainCoinState) []*ChainCoinState {
	var candidates []*ChainCoinState

	for _, state := range states {
		// Check score threshold
		if state.ScoreFinal < r.config.ScoreThreshold {
			continue
		}

		// Check for hard blocks
		if r.hasHardBlocks(state) {
			continue
		}

		// Check cooldown
		if r.isOnCooldown(state.Symbol) {
			continue
		}

		// Check if symbol has open position already
		if r.hasOpenPosition(ctx, state.Symbol) {
			continue
		}

		candidates = append(candidates, state)
	}

	// Sort by score descending (best candidates first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ScoreFinal > candidates[j].ScoreFinal
	})

	return candidates
}

// hasHardBlocks checks if the coin state has any hard blocks
func (r *ChainEntryRunner) hasHardBlocks(state *ChainCoinState) bool {
	if len(state.BlockingReasons) == 0 {
		return false
	}

	// Hard block codes that cannot be overridden
	hardBlockCodes := map[string]bool{
		"TREND_DIVERGENCE":       true,
		"ADX_TOO_LOW":            true,
		"CIRCUIT_BREAKER_ACTIVE": true,
		"REGIME_MISMATCH":        true,
		"TIMEFRAME_MISALIGN":     true,
	}

	for _, reason := range state.BlockingReasons {
		if hardBlockCodes[reason] {
			return true
		}
	}

	return false
}

// isOnCooldown checks if the symbol is on entry cooldown
func (r *ChainEntryRunner) isOnCooldown(symbol string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lastEntry, exists := r.entryCooldowns[symbol]
	if !exists {
		return false
	}

	return time.Since(lastEntry) < r.config.EntryCooldown
}

// hasOpenPosition checks if there's already an open position for the symbol
func (r *ChainEntryRunner) hasOpenPosition(ctx context.Context, symbol string) bool {
	if r.futuresClient == nil {
		return false
	}

	positions, err := r.futuresClient.GetPositions()
	if err != nil {
		return false
	}

	for _, pos := range positions {
		if pos.Symbol == symbol && pos.PositionAmt != 0 {
			return true
		}
	}

	return false
}

// canEnterPosition performs final checks before entry (non-locking version for internal checks).
// NOTE: This method does NOT acquire limitMu. For race-safe entry, use tryReserveEntrySlot instead.
func (r *ChainEntryRunner) canEnterPosition(ctx context.Context, state *ChainCoinState) bool {
	// Check if we have all required dependencies
	if r.futuresClient == nil {
		log.Printf("[CHAIN-ENTRY] FuturesClient not available for %s", state.Symbol)
		return false
	}

	// Check mode is enabled and has available slots
	mode := state.ActiveStrategy
	if mode == "" {
		mode = "scalp" // Default
	}

	// Get mode config to check if enabled
	if r.settingsCache != nil && r.settingsCache.IsHealthy() {
		modeConfig, err := r.settingsCache.GetModeConfig(ctx, r.userID, mode)
		if err == nil && modeConfig != nil {
			if !modeConfig.Enabled {
				log.Printf("[CHAIN-ENTRY] Mode %s is disabled for %s", mode, state.Symbol)
				return false
			}
		}
	}

	// Check position limit (without locking - for preliminary checks)
	allowed, _, _ := r.checkPositionLimit(ctx, state, false)
	return allowed
}

// checkPositionLimit checks if we can open a new position given the current limits.
// If includePending is true, pending entries are counted toward the current active count.
// Returns: (allowed bool, currentCount int, maxCount int)
func (r *ChainEntryRunner) checkPositionLimit(ctx context.Context, state *ChainCoinState, includePending bool) (bool, int, int) {
	if r.chainEventWriter == nil {
		return true, 0, 1 // Allow if we can't check
	}

	activeChains, err := r.chainEventWriter.GetActiveChains(ctx, r.userID)
	if err != nil {
		log.Printf("[CHAIN-ENTRY] Warning: Failed to get active chains for %s: %v", state.Symbol, err)
		return true, 0, 1 // Allow if we can't check (fail-open for trading)
	}

	currentActiveCount := len(activeChains)

	// Add pending entries to the count if requested
	if includePending {
		r.pendingMu.Lock()
		currentActiveCount += len(r.pendingEntries)
		r.pendingMu.Unlock()
	}

	// Get max_concurrent_trades from sub-strategy settings
	mode := state.ActiveStrategy
	if mode == "" {
		mode = "scalp"
	}
	maxConcurrentTrades := 1 // Default fallback - conservative
	if r.repo != nil {
		strategyGroup := state.StrategyGroup
		if strategyGroup == "" {
			strategyGroup = "volume_imbalance" // Default
		}
		subStrategy := state.SubStrategy
		if subStrategy == "" {
			subStrategy = "ravindra" // Default
		}

		subSettings, err := r.repo.GetSubStrategySettings(ctx, r.userID, mode, strategyGroup, subStrategy)
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

	log.Printf("[CHAIN-ENTRY] Position limit check for %s: active=%d (pending included=%v), max=%d",
		state.Symbol, currentActiveCount, includePending, maxConcurrentTrades)

	if currentActiveCount >= maxConcurrentTrades {
		log.Printf("[CHAIN-ENTRY] BLOCKED: max_concurrent_trades limit reached (%d/%d), skipping %s",
			currentActiveCount, maxConcurrentTrades, state.Symbol)
		return false, currentActiveCount, maxConcurrentTrades
	}

	return true, currentActiveCount, maxConcurrentTrades
}

// tryReserveEntrySlot atomically checks position limits and reserves a slot for entry.
// This prevents the race condition where multiple goroutines read activeChains=0,
// all pass the check, and all place orders exceeding the limit.
//
// Returns: (allowed bool, releaseFunc func())
// If allowed is true, the caller MUST call releaseFunc() after the entry completes
// (success or failure) to release the reserved slot.
func (r *ChainEntryRunner) tryReserveEntrySlot(ctx context.Context, state *ChainCoinState) (bool, func()) {
	symbol := state.Symbol

	// Acquire the limit mutex to ensure atomic check-and-reserve
	r.limitMu.Lock()

	// Check if this symbol already has a pending entry
	r.pendingMu.Lock()
	if r.pendingEntries[symbol] {
		r.pendingMu.Unlock()
		r.limitMu.Unlock()
		log.Printf("[CHAIN-ENTRY] BLOCKED: %s already has pending entry", symbol)
		return false, nil
	}
	r.pendingMu.Unlock()

	// Check position limit (including pending entries)
	allowed, currentCount, maxCount := r.checkPositionLimit(ctx, state, true)
	if !allowed {
		r.limitMu.Unlock()
		return false, nil
	}

	// Reserve the slot by marking this symbol as pending
	r.pendingMu.Lock()
	r.pendingEntries[symbol] = true
	r.pendingMu.Unlock()

	log.Printf("[CHAIN-ENTRY] Reserved entry slot for %s: active=%d (now +1 pending), max=%d",
		symbol, currentCount, maxCount)

	// Release function to be called after entry completes
	releaseFunc := func() {
		r.pendingMu.Lock()
		delete(r.pendingEntries, symbol)
		r.pendingMu.Unlock()
		r.limitMu.Unlock()
		log.Printf("[CHAIN-ENTRY] Released entry slot for %s", symbol)
	}

	return true, releaseFunc
}

// executeChainEntry places the entry order and records chain events
func (r *ChainEntryRunner) executeChainEntry(ctx context.Context, state *ChainCoinState) error {
	symbol := state.Symbol
	modeStr := state.ActiveStrategy
	if modeStr == "" {
		modeStr = "scalp"
	}

	// Use pattern-detected direction if set, fall back to trend-based
	direction := strings.ToUpper(state.Direction)
	if direction == "" {
		direction = "LONG"
		if state.Trend1H == "BEARISH" || state.Trend15M == "BEARISH" {
			direction = "SHORT"
		}
	}

	log.Printf("[CHAIN-ENTRY] Executing entry for %s, mode=%s, direction=%s, score=%d",
		symbol, modeStr, direction, state.ScoreFinal)

	// Step 1: Get current price from Binance
	currentPrice, err := r.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get current price: %w", err)
	}

	// Step 2: Get symbol info for quantity precision
	exchangeInfo, err := r.futuresClient.GetFuturesExchangeInfo()
	if err != nil {
		return fmt.Errorf("failed to get exchange info: %w", err)
	}

	var symbolInfo *binance.FuturesSymbolInfo
	for i := range exchangeInfo.Symbols {
		if exchangeInfo.Symbols[i].Symbol == symbol {
			symbolInfo = &exchangeInfo.Symbols[i]
			break
		}
	}
	if symbolInfo == nil {
		return fmt.Errorf("symbol %s not found in exchange info", symbol)
	}

	// Step 3: Get position size from strategy settings (if available) or fall back to mode defaults
	// NOTE: We ignore tpPercent from getModeDefaults because we use R:R ratio calculation instead
	// (tpPercent from defaults is a small percentage like 0.4%, not a R:R ratio)
	positionSizeUSD, slPercent, _ := r.getModeDefaults(modeStr)

	// Override with strategy-specific settings if provided
	if state.BudgetUSD > 0 {
		positionSizeUSD = state.BudgetUSD
		log.Printf("[CHAIN-ENTRY] Using strategy budget: %.2f USD (from sub-strategy settings)", positionSizeUSD)
	}

	// === Incremental Equity Position Sizing ===
	effectiveBudget := positionSizeUSD
	if state.UseIncrementalEquity && r.repo != nil {
		equity, err := r.repo.GetSubStrategyEquity(ctx, r.userID, modeStr, state.StrategyGroup, state.SubStrategy)
		if err != nil {
			log.Printf("[CHAIN-ENTRY] Warning: failed to get equity, using assigned budget: %v", err)
		} else {
			effectiveBudget = equity
			log.Printf("[CHAIN-ENTRY] Incremental equity enabled: current=$%.2f (assigned=$%.2f)", equity, positionSizeUSD)
		}
	}

	// Stop trading if equity depleted (threshold: $1)
	if effectiveBudget <= 1.0 {
		return fmt.Errorf("equity depleted: $%.2f <= $1.00 threshold, trading stopped for %s", effectiveBudget, symbol)
	}

	// Calculate per-trade capital based on available budget and concurrent slots
	maxConcurrent := state.MaxConcurrentTrades
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	capitalInUse := 0.0
	activeForStrategy := 0
	if r.repo != nil {
		var err error
		capitalInUse, activeForStrategy, err = r.repo.GetDB().GetCapitalInUseForSubStrategy(ctx, r.userID, modeStr, state.StrategyGroup, state.SubStrategy)
		if err != nil {
			log.Printf("[CHAIN-ENTRY] Warning: failed to get capital in use: %v", err)
		}
	}

	availableCapital := effectiveBudget - capitalInUse
	if availableCapital <= 0 {
		return fmt.Errorf("no available capital: effective=$%.2f, in_use=$%.2f for %s", effectiveBudget, capitalInUse, symbol)
	}

	remainingSlots := maxConcurrent - activeForStrategy
	if remainingSlots <= 0 {
		remainingSlots = 1 // Fallback: shouldn't happen (position limit check should catch this)
	}

	tradeCapital := availableCapital / float64(remainingSlots)
	positionSizeUSD = tradeCapital
	state.TradeCapital1x = tradeCapital // Store for recording in chain

	log.Printf("[CHAIN-ENTRY] Position sizing: effective=$%.2f, in_use=$%.2f, available=$%.2f, slots=%d/%d, per_trade=$%.2f",
		effectiveBudget, capitalInUse, availableCapital, remainingSlots, maxConcurrent, tradeCapital)

	if state.SLPercent > 0 {
		slPercent = state.SLPercent
	}
	// NOTE: state.TPPercent is used as R:R ratio (e.g., 4.0 for 1:4) in the SL/TP calculation below

	// Apply leverage to position size (budget × leverage = effective position)
	if state.MaxLeverage > 0 {
		positionSizeUSD = positionSizeUSD * float64(state.MaxLeverage)
		log.Printf("[CHAIN-ENTRY] Applied leverage %dx: effective position %.2f USD", state.MaxLeverage, positionSizeUSD)
	}

	// Step 4: Calculate quantity
	quantity := positionSizeUSD / currentPrice

	// Round to symbol's quantity precision
	quantityPrecision := symbolInfo.QuantityPrecision
	if quantityPrecision < 0 {
		quantityPrecision = 3
	}
	multiplier := math.Pow(10, float64(quantityPrecision))
	quantity = math.Floor(quantity*multiplier) / multiplier

	// Parse MinQty from filters
	var minQty float64 = 0.001
	for _, filter := range symbolInfo.Filters {
		if filter.FilterType == "LOT_SIZE" && filter.MinQty != "" {
			if parsedMinQty, parseErr := parseFloatStr(filter.MinQty); parseErr == nil {
				minQty = parsedMinQty
			}
			break
		}
	}
	if quantity < minQty {
		quantity = minQty
	}

	// Step 5: Generate chain ID using sequential generator (Epic 7)
	tradingMode := orders.ModeFromString(modeStr)
	modeCode := orders.ModeCode[tradingMode]
	if modeCode == "" {
		modeCode = "SCA"
	}
	var chainID, entryClientOrderID string

	if r.orderIdGenerator != nil {
		// Use proper sequential generator
		fullID, baseID, err := r.orderIdGenerator.Generate(ctx, tradingMode, orders.OrderTypeEntry)
		if err != nil {
			log.Printf("[CHAIN-ENTRY] Warning: Generator error, using fallback: %v", err)
			// Fallback to generator's built-in fallback
			fullID, baseID = r.orderIdGenerator.GenerateFallback(tradingMode, orders.OrderTypeEntry)
		}
		chainID = baseID
		entryClientOrderID = fullID
	} else {
		// Fallback when generator not available
		now := time.Now().UTC()
		dateStr := now.Format("02Jan")
		seqNum := now.UnixNano() % 100000
		chainID = fmt.Sprintf("%s-%s-%05d", modeCode, dateStr, seqNum)
		entryClientOrderID = fmt.Sprintf("%s-E", chainID)
		log.Printf("[CHAIN-ENTRY] Warning: Using fallback order ID generation (no generator available)")
	}

	// Step 6: Determine order side
	var orderSide string
	var positionSide binance.PositionSide
	if direction == "LONG" {
		orderSide = "BUY"
		positionSide = binance.PositionSideLong
	} else {
		orderSide = "SELL"
		positionSide = binance.PositionSideShort
	}

	// Step 7: Build entry context from pattern detection data (for UI display)
	var entryContextJSON json.RawMessage
	entryCtx := map[string]interface{}{
		"direction":   direction,
		"timeframe":   state.Timeframe,
		"entry_price": state.EntryPrice,
		"sl_price":    state.SLPrice,
		"tp_price":    state.TPPrice,
		"risk_amount": state.RiskAmount,
		"breakout_at": time.Now().UTC().Format(time.RFC3339),
	}
	if entryCtxBytes, err := json.Marshal(entryCtx); err == nil {
		entryContextJSON = entryCtxBytes
	}

	// Record budget capital used for incremental equity tracking
	var budgetCapUsed *float64
	if state.TradeCapital1x > 0 {
		cap := state.TradeCapital1x
		budgetCapUsed = &cap
	}

	// Step 8: Create order chain - MANDATORY for chain system integrity
	// If database write fails, abort entry to prevent orphaned positions that Ginie picks up
	if r.chainEventWriter != nil {
		_, err := r.chainEventWriter.CreateChain(ctx, orders.CreateChainRequest{
			UserID:            r.userID,
			ChainID:           chainID,
			Symbol:            symbol,
			Side:              direction,
			ModeCode:          modeCode,
			IsHedge:           false,
			Mode:              state.ActiveStrategy,  // scalp, swing, position, ultra_fast
			StrategyGroup:     state.StrategyGroup,   // e.g., breakout
			SubStrategy:       state.SubStrategy,     // e.g., ravindra_volume_imbalance
			Timeframe:         state.Timeframe,       // e.g., 3m, 5m, 15m from pattern detection
			EntryContext:      entryContextJSON,
			BudgetCapitalUsed: budgetCapUsed,
		})
		if err != nil {
			log.Printf("[CHAIN-ENTRY] ABORTED: Failed to create order chain in database - cannot place order without chain record: %v", err)
			// Release the consumed sequence number so daily counter stays accurate
			if r.orderIdGenerator != nil {
				r.orderIdGenerator.ReleaseSequence(ctx)
			}
			return fmt.Errorf("failed to create order chain (required for chain system): %w", err)
		}
	} else {
		log.Printf("[CHAIN-ENTRY] ABORTED: No chainEventWriter - cannot place order without chain tracking")
		// Release the consumed sequence number so daily counter stays accurate
		if r.orderIdGenerator != nil {
			r.orderIdGenerator.ReleaseSequence(ctx)
		}
		return fmt.Errorf("chainEventWriter not available - cannot place untracked orders")
	}

	// Step 8: Place LIMIT order (maker-friendly entry)
	// Instead of MARKET order (always taker), use LIMIT order:
	// - Controlled entry price (no slippage beyond limit)
	// - Chance to be a maker (lower fees: maker < taker)
	// - Price capped at slightly above/below breakout level
	limitPriceOffset := 0.001 // 0.1% offset to increase fill probability while controlling price
	var limitPrice float64
	if state.EntryPrice > 0 {
		// Use the breakout level from pattern detection (reference candle HIGH for LONG, LOW for SHORT)
		// This ensures the LIMIT order is placed at the correct entry level, not wherever
		// the market price happens to be when the API call is made
		if direction == "LONG" {
			limitPrice = state.EntryPrice * (1 + limitPriceOffset)
		} else {
			limitPrice = state.EntryPrice * (1 - limitPriceOffset)
		}
		log.Printf("[CHAIN-ENTRY] Using pattern entry level %.6f for LIMIT price (current market: %.6f)", state.EntryPrice, currentPrice)
	} else {
		// Fallback: use current market price when pattern entry level not available
		if direction == "LONG" {
			limitPrice = currentPrice * (1 + limitPriceOffset)
		} else {
			limitPrice = currentPrice * (1 - limitPriceOffset)
		}
		log.Printf("[CHAIN-ENTRY] No pattern entry level, using current market price %.6f for LIMIT price", currentPrice)
	}
	limitPrice = roundToTickSizeFromSymbol(limitPrice, symbolInfo)

	orderParams := binance.FuturesOrderParams{
		Symbol:           symbol,
		Side:             orderSide,
		PositionSide:     positionSide,
		Type:             binance.FuturesOrderTypeLimit,
		Quantity:         quantity,
		Price:            limitPrice,
		TimeInForce:      binance.TimeInForceGTC,
		NewClientOrderId: entryClientOrderID,
	}

	log.Printf("[CHAIN-ENTRY] Placing LIMIT %s order for %s: qty=%.6f, limitPrice=%.6f, positionSizeUSD=%.2f, chainID=%s",
		orderSide, symbol, quantity, limitPrice, positionSizeUSD, chainID)

	orderResp, err := r.futuresClient.PlaceFuturesOrder(orderParams)
	if err != nil {
		log.Printf("[CHAIN-ENTRY] FAILED to place LIMIT order for %s: %v - cancelling chain", symbol, err)
		// Cancel the chain that was already created in the database
		if r.chainEventWriter != nil {
			cancelCtx := context.Background()
			_ = r.chainEventWriter.RecordEntryCancelled(cancelCtx, chainID, "order_rejected: "+err.Error())
			if closeErr := r.chainEventWriter.CloseChain(cancelCtx, chainID, "entry_order_rejected", 0, 0, nil); closeErr != nil {
				log.Printf("[CHAIN-ENTRY] Warning: Failed to close rejected chain %s: %v", chainID, closeErr)
			}
		}
		return fmt.Errorf("failed to place entry order: %w", err)
	}

	log.Printf("[CHAIN-ENTRY] LIMIT order placed: orderID=%d, status=%q, limitPrice=%.6f",
		orderResp.OrderId, orderResp.Status, limitPrice)

	// Step 9: Record entry placed event
	if r.chainEventWriter != nil {
		err := r.chainEventWriter.RecordEntryPlaced(ctx, chainID, orders.ChainEntryPlacedEvent{
			BinanceOrderID:       orderResp.OrderId,
			BinanceClientOrderID: entryClientOrderID,
			Price:                limitPrice,
			Quantity:             quantity,
			BinanceTimestamp:     orderResp.UpdateTime,
		})
		if err != nil {
			log.Printf("[CHAIN-ENTRY] Warning: Failed to record entry placed event: %v", err)
		}
	}

	// Calculate fill timeout (candle duration) - needed for both UI notification and actual wait
	fillTimeout := parseTimeframeToDuration(state.Timeframe)
	if fillTimeout < time.Minute {
		fillTimeout = time.Minute // Minimum 1 minute
	}

	// Step 9a: Notify UI that order is placed and waiting for fill (Step 3 transition)
	r.mu.RLock()
	fillingCallback := r.onEntryFilling
	r.mu.RUnlock()
	if fillingCallback != nil {
		fillTimeoutSecs := int(fillTimeout.Seconds())
		fillingCallback(symbol, modeStr, state.Timeframe, limitPrice, positionSizeUSD, fillTimeoutSecs)
	}

	// Step 9b: Wait for LIMIT order fill
	// Timeout matches candle duration so order has one full candle to fill
	var filledPrice float64
	var filledQty float64

	// Check if LIMIT order filled immediately (at or better than limit price)
	if orderResp.Status == "FILLED" && orderResp.AvgPrice > 0 {
		filledPrice = orderResp.AvgPrice
		filledQty = orderResp.ExecutedQty
		if filledQty <= 0 {
			filledQty = quantity
		}
		log.Printf("[CHAIN-ENTRY] LIMIT order filled immediately: price=%.6f, qty=%.6f", filledPrice, filledQty)
	} else {
		// Poll for fill until timeout (candle duration)
		log.Printf("[CHAIN-ENTRY] Waiting for LIMIT order fill: timeout=%v (timeframe=%s)", fillTimeout, state.Timeframe)
		filledOrder, fillErr := r.waitForLimitFill(symbol, modeStr, state.Timeframe, orderResp.OrderId, fillTimeout)
		if fillErr != nil {
			log.Printf("[CHAIN-ENTRY] LIMIT order not filled for %s: %v - cancelling chain", symbol, fillErr)

			// CRITICAL: Cancel the chain in the database to prevent orphaned active chains
			// that would block future entries by counting toward max_concurrent_trades
			cancelCtx := context.Background()
			if r.chainEventWriter != nil {
				// Record entry cancelled event
				if recErr := r.chainEventWriter.RecordEntryCancelled(cancelCtx, chainID, fillErr.Error()); recErr != nil {
					log.Printf("[CHAIN-ENTRY] Warning: Failed to record entry cancelled event for chain %s: %v", chainID, recErr)
				}
				// Close the chain as cancelled (0 PnL, 0 fees since no fill occurred)
				if closeErr := r.chainEventWriter.CloseChain(cancelCtx, chainID, "entry_timeout_"+fillErr.Error(), 0, 0, nil); closeErr != nil {
					log.Printf("[CHAIN-ENTRY] Warning: Failed to close cancelled chain %s: %v", chainID, closeErr)
				} else {
					log.Printf("[CHAIN-ENTRY] Chain %s cancelled after entry timeout for %s", chainID, symbol)
				}
			}

			return fmt.Errorf("limit order not filled: %w", fillErr)
		}
		filledPrice = filledOrder.AvgPrice
		filledQty = filledOrder.ExecutedQty
		if filledQty <= 0 {
			filledQty = quantity
		}
		log.Printf("[CHAIN-ENTRY] LIMIT order filled after waiting: price=%.6f, qty=%.6f", filledPrice, filledQty)
	}

	if filledPrice <= 0 {
		filledPrice = limitPrice
	}

	// Step 9c: Record entry filled event (use background ctx since parent may have expired during fill wait)
	postFillCtx := context.Background()
	if r.chainEventWriter != nil {
		err = r.chainEventWriter.RecordEntryFilled(postFillCtx, chainID, orders.ChainEntryFilledEvent{
			FilledPrice:      filledPrice,
			FilledQuantity:   filledQty,
			Fees:             0, // Fees tracked separately via WebSocket
			BinanceTimestamp: orderResp.UpdateTime,
		})
		if err != nil {
			log.Printf("[CHAIN-ENTRY] Warning: Failed to record entry filled event: %v", err)
		} else {
			log.Printf("[CHAIN-ENTRY] Recorded entry filled: chainID=%s, price=%.6f, qty=%.6f",
				chainID, filledPrice, filledQty)

			// Notify that fill is completed FIRST - this triggers:
			// 1. SetPatternPositionRunning (Step 4 in Entry Decision UI)
			// 2. UpdateSymbolToPosition (coin profiler source update)
			// Must happen BEFORE POSITION_CREATED broadcast because the frontend
			// subscribes to POSITION_CREATED and refetches data. If we broadcast first,
			// the refetched data still shows "strategy" source (race condition).
			if r.onFillCompleted != nil {
				r.onFillCompleted(symbol, modeStr, state.Timeframe, filledQty, filledPrice)
			}

			// THEN broadcast POSITION_CREATED event to UI for instant update
			// At this point, pattern state and coin profiler are already updated,
			// so when frontend refetches it gets the correct data.
			events.BroadcastPositionCreated(r.userID, map[string]interface{}{
				"chain_id":       chainID,
				"symbol":         symbol,
				"side":           direction,
				"entry_price":    filledPrice,
				"quantity":       filledQty,
				"mode":           state.ActiveStrategy,
				"mode_code":      modeCode,
				"strategy_group": state.StrategyGroup,
				"sub_strategy":   state.SubStrategy,
				"timeframe":      state.Timeframe,
				"order_id":       orderResp.OrderId,
				"created_at":     time.Now().UTC().Format(time.RFC3339),
			})
			log.Printf("[CHAIN-ENTRY] Broadcast POSITION_CREATED: chainID=%s, symbol=%s, side=%s",
				chainID, symbol, direction)
		}
	}

	// Step 10: Calculate SL/TP prices using actual fill price
	entryPrice := filledPrice
	filledQuantity := filledQty

	// Calculate SL/TP prices
	// PREFERRED: Use actual prices from pattern detection (Support/Resistance based)
	// FALLBACK: Use percentage-based calculation
	var slPrice, tpPrice float64
	if state.SLPrice > 0 && state.TPPrice > 0 {
		// Use actual prices from pattern detection
		// These are calculated from Support Price (Consolidation Low) at pattern detection time
		slPrice = state.SLPrice
		tpPrice = state.TPPrice
		log.Printf("[CHAIN-ENTRY] Using actual SL/TP from pattern: SL=%.6f, TP=%.6f, Risk=%.6f",
			slPrice, tpPrice, state.RiskAmount)

		// If the actual entry differs from pattern's expected entry, adjust SL/TP proportionally
		// This maintains the same R:R ratio even if market moved
		if state.EntryPrice > 0 && state.RiskAmount > 0 && entryPrice != state.EntryPrice {
			priceDiff := entryPrice - state.EntryPrice
			if direction == "LONG" {
				// For LONG: if entry is higher, SL and TP both move up
				slPrice = state.SLPrice + priceDiff
				tpPrice = state.TPPrice + priceDiff
			} else {
				// For SHORT: if entry is lower, SL and TP both move down
				slPrice = state.SLPrice + priceDiff
				tpPrice = state.TPPrice + priceDiff
			}
			log.Printf("[CHAIN-ENTRY] Adjusted SL/TP for price drift (%.6f → %.6f): SL=%.6f, TP=%.6f",
				state.EntryPrice, entryPrice, slPrice, tpPrice)
		}
	} else {
		// Fallback to percentage-based calculation
		// CRITICAL FIX: Use R:R ratio to calculate TP percentage from SL percentage
		// state.TPPercent stores the R:R ratio (e.g., 4.0 for 1:4), NOT a percentage
		// For 1:4 R:R ratio: if SL=1.5%, then TP should be 1.5% × 4 = 6%
		rrRatio := 4.0 // Default R:R ratio for Ravindra Volume Imbalance strategy
		if state.TPPercent > 1 {
			// TPPercent > 1 means it's an R:R ratio (e.g., 4.0), not a percentage
			rrRatio = state.TPPercent
		}
		// Calculate actual TP percentage using the R:R ratio
		actualTPPercent := slPercent * rrRatio

		if direction == "LONG" {
			slPrice = entryPrice * (1 - slPercent/100)
			tpPrice = entryPrice * (1 + actualTPPercent/100)
		} else {
			slPrice = entryPrice * (1 + slPercent/100)
			tpPrice = entryPrice * (1 - actualTPPercent/100)
		}
		log.Printf("[CHAIN-ENTRY] Using percentage-based SL/TP with R:R ratio %.1f: SL=%.2f%%, TP=%.2f%% (Risk:Reward = 1:%.1f)",
			rrRatio, slPercent, actualTPPercent, rrRatio)
	}

	// Round SL/TP prices to appropriate precision
	slPrice = roundToTickSizeFromSymbol(slPrice, symbolInfo)
	tpPrice = roundToTickSizeFromSymbol(tpPrice, symbolInfo)

	// SAFETY NET: Final max_sl_percent validation before placing SL/TP orders
	// This is the last line of defense - if the calculated SL exceeds the configured max,
	// log a warning. The primary enforcement happens in pattern_matcher.go and realtime_matcher.go.
	if state.SLPercent > 0 && entryPrice > 0 && slPrice > 0 {
		var actualSLPercent float64
		if direction == "LONG" {
			actualSLPercent = ((entryPrice - slPrice) / entryPrice) * 100
		} else {
			actualSLPercent = ((slPrice - entryPrice) / entryPrice) * 100
		}
		if actualSLPercent > state.SLPercent {
			log.Printf("[CHAIN-ENTRY] SL_REJECTED: %s %s actual SL=%.2f%% exceeds max_sl_percent=%.2f%% (entry=%.6f, sl=%.6f) - ABORTING ENTRY",
				symbol, direction, actualSLPercent, state.SLPercent, entryPrice, slPrice)
			// Cancel the chain that was already created
			if r.chainEventWriter != nil {
				cancelCtx := context.Background()
				reason := fmt.Sprintf("sl_too_wide: %.2f%% > max %.2f%%", actualSLPercent, state.SLPercent)
				_ = r.chainEventWriter.RecordEntryCancelled(cancelCtx, chainID, reason)
				if closeErr := r.chainEventWriter.CloseChain(cancelCtx, chainID, "sl_exceeds_max_percent", 0, 0, nil); closeErr != nil {
					log.Printf("[CHAIN-ENTRY] Warning: Failed to close rejected chain %s: %v", chainID, closeErr)
				}
			}
			return fmt.Errorf("SL %.2f%% exceeds max_sl_percent %.2f%%", actualSLPercent, state.SLPercent)
		}
		log.Printf("[CHAIN-ENTRY] SL validation passed: %s %s actual_sl=%.2f%% <= max=%.2f%%",
			symbol, direction, actualSLPercent, state.SLPercent)
	}

	// Step 11: Place SL/TP orders immediately to protect the position
	// CRITICAL: Position must have SL/TP orders placed within seconds of entry
	// For MARKET orders, we place SL/TP immediately after entry - don't wait for status
	// The order was placed successfully (no error), so we protect the position
	shouldPlaceSLTP := filledQuantity > 0
	if !shouldPlaceSLTP {
		log.Printf("[CHAIN-ENTRY] WARNING: filledQuantity is 0, cannot place SL/TP orders")
	} else {
		log.Printf("[CHAIN-ENTRY] Proceeding to place SL/TP: filledQty=%.6f", filledQuantity)
	}
	if shouldPlaceSLTP {
		// Determine close side (opposite of entry side)
		var closeSide string
		if direction == "LONG" {
			closeSide = "SELL"
		} else {
			closeSide = "BUY"
		}

		// Generate client order IDs for SL/TP
		slClientOrderID := fmt.Sprintf("%s-SL", chainID)
		tpClientOrderID := fmt.Sprintf("%s-TP", chainID)

		// Get precision from symbol info to avoid Binance -1111 errors
		pricePrecision, qtyPrecision := getPrecisionFromSymbol(symbolInfo)
		log.Printf("[CHAIN-ENTRY] Using precision: price=%d, qty=%d for %s", pricePrecision, qtyPrecision, symbol)

		// Place Stop Loss order (STOP_MARKET - executes as MARKET order when trigger hits, always fills)
		slParams := binance.AlgoOrderParams{
			Symbol:            symbol,
			Side:              closeSide,
			PositionSide:      positionSide,
			Type:              binance.FuturesOrderTypeStopMarket,
			Quantity:          filledQuantity,
			TriggerPrice:      slPrice,
			ClosePosition:     false,
			WorkingType:       binance.WorkingTypeMarkPrice,
			ClientAlgoId:      slClientOrderID,
			PricePrecision:    pricePrecision,
			QuantityPrecision: qtyPrecision,
		}

		log.Printf("[CHAIN-ENTRY] Placing STOP_MARKET SL order for %s: triggerPrice=%.6f, qty=%.6f", symbol, slPrice, filledQuantity)

		slResp, err := r.futuresClient.PlaceAlgoOrder(slParams)
		if err != nil {
			log.Printf("[CHAIN-ENTRY] WARNING: Failed to place SL order for %s: %v", symbol, err)
		} else {
			log.Printf("[CHAIN-ENTRY] SL order placed: symbol=%s, algoID=%d, price=%.6f", symbol, slResp.AlgoId, slPrice)

			// Record SL placed event
			if r.chainEventWriter != nil {
				err := r.chainEventWriter.RecordSLPlaced(postFillCtx, chainID, orders.ChainSLPlacedEvent{
					BinanceOrderID:       slResp.AlgoId,
					BinanceClientOrderID: slClientOrderID,
					Price:                slPrice,
					BinanceTimestamp:     slResp.UpdateTime,
				})
				if err != nil {
					log.Printf("[CHAIN-ENTRY] Warning: Failed to record SL placed event: %v", err)
				}
				// Persist SL order details to order_chains for closed chain reconstruction
				r.chainEventWriter.PersistSLDetails(postFillCtx, chainID, slResp.AlgoId, slPrice, filledQuantity)
			}
		}

		// Place Take Profit order (TAKE_PROFIT_MARKET - executes as MARKET order when trigger hits, always fills)
		tpParams := binance.AlgoOrderParams{
			Symbol:            symbol,
			Side:              closeSide,
			PositionSide:      positionSide,
			Type:              binance.FuturesOrderTypeTakeProfitMarket,
			Quantity:          filledQuantity,
			TriggerPrice:      tpPrice,
			ClosePosition:     false,
			WorkingType:       binance.WorkingTypeMarkPrice,
			ClientAlgoId:      tpClientOrderID,
			PricePrecision:    pricePrecision,
			QuantityPrecision: qtyPrecision,
		}

		log.Printf("[CHAIN-ENTRY] Placing TAKE_PROFIT_MARKET TP order for %s: triggerPrice=%.6f, qty=%.6f", symbol, tpPrice, filledQuantity)

		tpResp, err := r.futuresClient.PlaceAlgoOrder(tpParams)
		if err != nil {
			log.Printf("[CHAIN-ENTRY] WARNING: Failed to place TP order for %s: %v", symbol, err)
		} else {
			log.Printf("[CHAIN-ENTRY] TP order placed: symbol=%s, algoID=%d, price=%.6f", symbol, tpResp.AlgoId, tpPrice)

			// Record TP placed event
			if r.chainEventWriter != nil {
				err := r.chainEventWriter.RecordTPPlaced(postFillCtx, chainID, orders.ChainTPPlacedEvent{
					BinanceOrderID:       tpResp.AlgoId,
					BinanceClientOrderID: tpClientOrderID,
					Price:                tpPrice,
					BinanceTimestamp:     tpResp.UpdateTime,
				})
				if err != nil {
					log.Printf("[CHAIN-ENTRY] Warning: Failed to record TP placed event: %v", err)
				}
				// Persist TP order details to order_chains for closed chain reconstruction
				r.chainEventWriter.PersistTPDetails(postFillCtx, chainID, tpResp.AlgoId, tpPrice, filledQuantity)
			}
		}

		// Broadcast SL/TP placement to frontend for real-time display
		if slResp != nil || tpResp != nil {
			slTPData := map[string]interface{}{
				"chain_id": chainID,
				"symbol":   symbol,
				"side":     direction,
			}
			if slResp != nil {
				slTPData["sl_order_id"] = slResp.AlgoId
				slTPData["sl_price"] = slPrice
				slTPData["sl_status"] = "NEW"
			}
			if tpResp != nil {
				slTPData["tp_order_id"] = tpResp.AlgoId
				slTPData["tp_price"] = tpPrice
				slTPData["tp_status"] = "NEW"
			}
			events.BroadcastSLTPPlaced(r.userID, slTPData)
			log.Printf("[CHAIN-ENTRY] Broadcast SL_TP_PLACED: chainID=%s, SL=%.6f, TP=%.6f", chainID, slPrice, tpPrice)
		}

		// Step 11b: Register position with Ravindra Position Monitor for trailing stop management
		// This enables automatic SL updates at 1:2 (breakeven) and 1:3 (1:1 profit lock) milestones
		// Placed OUTSIDE the TP success/failure blocks so registration happens regardless of TP placement result
		if r.ravindraMonitor != nil && entryPrice > 0 {
			slOrderID := int64(0)
			if slResp != nil {
				slOrderID = slResp.AlgoId
			}
			tpOrderID := int64(0)
			if tpResp != nil {
				tpOrderID = tpResp.AlgoId
			}

			ravindraPos := CreateRavindraPositionFromChainEntry(
				chainID,
				symbol,
				r.userID,
				direction,
				entryPrice,
				filledQuantity,
				slPrice,
				tpPrice,
				slOrderID,
				tpOrderID,
				nil, // Use default config
				pricePrecision,
				qtyPrecision,
			)
			if err := r.ravindraMonitor.AddPosition(ravindraPos); err != nil {
				log.Printf("[CHAIN-ENTRY] Warning: Failed to register position with Ravindra monitor: %v", err)
			} else {
				log.Printf("[CHAIN-ENTRY] Position registered with Ravindra monitor: chainID=%s, symbol=%s, SL milestones: BE@1:2, 1R@1:3", chainID, symbol)
			}
		}
	}

	// Step 12: Update cooldown
	r.mu.Lock()
	r.entryCooldowns[symbol] = time.Now()
	r.mu.Unlock()

	log.Printf("[CHAIN-ENTRY] SUCCESS: %s %s entry at %.6f, qty=%.6f, orderID=%d, chainID=%s, SL=%.6f, TP=%.6f",
		direction, symbol, entryPrice, filledQuantity, orderResp.OrderId, chainID, slPrice, tpPrice)

	return nil
}

// waitForLimitFill polls order status until filled, cancelled, or timeout.
// Returns the filled order or error. On timeout, cancels the order automatically.
// mode and timeframe are passed through for fill progress callback broadcasts.
func (r *ChainEntryRunner) waitForLimitFill(symbol, mode, timeframe string, orderID int64, timeout time.Duration) (*binance.FuturesOrder, error) {
	deadline := time.Now().Add(timeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout reached - cancel the unfilled order
			log.Printf("[CHAIN-ENTRY] LIMIT order timeout for %s (orderID=%d) after %v - cancelling", symbol, orderID, timeout)
			if cancelErr := r.futuresClient.CancelFuturesOrder(symbol, orderID); cancelErr != nil {
				log.Printf("[CHAIN-ENTRY] Warning: Failed to cancel LIMIT order %d: %v", orderID, cancelErr)
			} else {
				log.Printf("[CHAIN-ENTRY] LIMIT order cancelled: orderID=%d", orderID)
			}
			return nil, fmt.Errorf("limit order not filled within %v timeout", timeout)
		case <-ticker.C:
			// Broadcast fill progress countdown
			r.mu.RLock()
			progressCb := r.onFillProgress
			r.mu.RUnlock()
			if progressCb != nil {
				remaining := int(time.Until(deadline).Seconds())
				if remaining < 0 {
					remaining = 0
				}
				progressCb(symbol, mode, timeframe, remaining)
			}

			order, err := r.futuresClient.GetOrder(symbol, orderID)
			if err != nil {
				log.Printf("[CHAIN-ENTRY] Warning: Failed to query order status (orderID=%d): %v", orderID, err)
				continue
			}
			switch order.Status {
			case "FILLED":
				return order, nil
			case "CANCELED", "EXPIRED", "REJECTED":
				return nil, fmt.Errorf("order %s (status=%s)", symbol, order.Status)
			default:
				// NEW, PARTIALLY_FILLED - continue waiting
				if order.ExecutedQty > 0 {
					log.Printf("[CHAIN-ENTRY] LIMIT order partially filled: %.6f/%.6f", order.ExecutedQty, order.OrigQty)
				}
			}
		}
	}
}

// parseTimeframeToDuration converts a timeframe string (e.g., "3m", "5m", "1h") to time.Duration.
func parseTimeframeToDuration(timeframe string) time.Duration {
	if len(timeframe) < 2 {
		return time.Minute // default
	}

	numStr := timeframe[:len(timeframe)-1]
	unit := timeframe[len(timeframe)-1:]

	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 {
		return time.Minute
	}

	switch unit {
	case "m":
		return time.Duration(num) * time.Minute
	case "h":
		return time.Duration(num) * time.Hour
	case "d":
		return time.Duration(num) * 24 * time.Hour
	default:
		return time.Minute
	}
}

// roundToTickSizeFromSymbol rounds a price to the symbol's tick size precision.
// This extracts the tick size from the FuturesSymbolInfo filters.
func roundToTickSizeFromSymbol(price float64, symbolInfo *binance.FuturesSymbolInfo) float64 {
	if symbolInfo == nil {
		return price
	}

	// Find tick size from filters
	var tickSize float64 = 0.01 // Default
	for _, filter := range symbolInfo.Filters {
		if filter.FilterType == "PRICE_FILTER" && filter.TickSize != "" {
			if parsed, err := parseFloatStr(filter.TickSize); err == nil && parsed > 0 {
				tickSize = parsed
			}
			break
		}
	}

	// Round to tick size
	if tickSize > 0 {
		return math.Round(price/tickSize) * tickSize
	}
	return price
}

// getModeDefaults returns position size, SL%, and TP% defaults for a trading mode
// These values match the default-settings.json mode configurations
func (r *ChainEntryRunner) getModeDefaults(mode string) (positionSizeUSD, slPercent, tpPercent float64) {
	switch mode {
	case "ultra_fast":
		return 15.0, 0.5, 0.3 // $15 position, 0.5% SL, 0.3% TP
	case "scalp":
		return 20.0, 1.0, 0.4 // $20 position, 1.0% SL, 0.4% TP
	case "swing":
		return 30.0, 2.0, 1.0 // $30 position, 2.0% SL, 1.0% TP
	case "position":
		return 50.0, 3.0, 2.0 // $50 position, 3.0% SL, 2.0% TP
	default:
		return 20.0, 1.5, 0.4 // Default to scalp-like settings
	}
}

// parseFloatStr parses a string to float64
func parseFloatStr(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// getPrecisionFromSymbol extracts price and quantity precision from symbol info
// Returns (pricePrecision, quantityPrecision)
func getPrecisionFromSymbol(symbolInfo *binance.FuturesSymbolInfo) (int, int) {
	pricePrecision := 8  // Default safe precision
	qtyPrecision := 8    // Default safe precision

	if symbolInfo == nil {
		return pricePrecision, qtyPrecision
	}

	// Get price precision from tick size
	for _, filter := range symbolInfo.Filters {
		if filter.FilterType == "PRICE_FILTER" && filter.TickSize != "" {
			if tickSize, err := parseFloatStr(filter.TickSize); err == nil && tickSize > 0 {
				// Calculate decimal places from tick size (e.g., 0.00001 = 5 decimal places)
				precision := 0
				for tickSize < 1 {
					tickSize *= 10
					precision++
				}
				pricePrecision = precision
			}
			break
		}
	}

	// Get quantity precision from step size
	for _, filter := range symbolInfo.Filters {
		if filter.FilterType == "LOT_SIZE" && filter.StepSize != "" {
			if stepSize, err := parseFloatStr(filter.StepSize); err == nil && stepSize > 0 {
				// Calculate decimal places from step size
				precision := 0
				for stepSize < 1 {
					stepSize *= 10
					precision++
				}
				qtyPrecision = precision
			}
			break
		}
	}

	// Also check the symbol's declared precision if available
	if symbolInfo.PricePrecision > 0 && symbolInfo.PricePrecision < pricePrecision {
		pricePrecision = symbolInfo.PricePrecision
	}
	if symbolInfo.QuantityPrecision > 0 && symbolInfo.QuantityPrecision < qtyPrecision {
		qtyPrecision = symbolInfo.QuantityPrecision
	}

	return pricePrecision, qtyPrecision
}

// SetFuturesClient updates the futures client (used when client is refreshed)
func (r *ChainEntryRunner) SetFuturesClient(client binance.FuturesClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.futuresClient = client
}

// SetRavindraMonitor sets the Ravindra position monitor for trailing stop management
func (r *ChainEntryRunner) SetRavindraMonitor(monitor *RavindraPositionMonitor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ravindraMonitor = monitor
	log.Printf("[CHAIN-ENTRY] Ravindra position monitor configured")
}

// GetRavindraMonitor returns the Ravindra position monitor
func (r *ChainEntryRunner) GetRavindraMonitor() *RavindraPositionMonitor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ravindraMonitor
}

// SetChainEventWriter updates the chain event writer
func (r *ChainEntryRunner) SetChainEventWriter(writer *orders.ChainEventWriter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chainEventWriter = writer
}

// SetStateProvider updates the state provider
func (r *ChainEntryRunner) SetStateProvider(sp ChainStateProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stateProvider = sp
}

// ClearCooldown clears the cooldown for a specific symbol
func (r *ChainEntryRunner) ClearCooldown(symbol string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entryCooldowns, symbol)
}

// ClearAllCooldowns clears all cooldowns
func (r *ChainEntryRunner) ClearAllCooldowns() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entryCooldowns = make(map[string]time.Time)
}

// ExecuteImmediateEntry executes an entry immediately without waiting for the scan cycle.
// This is called when tick-level breakout is detected for instant order placement.
// It bypasses the scan timer to ensure orders are placed at the moment of breakout.
//
// Parameters:
// - symbol: The trading pair (e.g., "BTCUSDT")
// - direction: "long" or "short"
// - mode: Trading mode (e.g., "scalp", "swing")
// - strategyGroup: Strategy group (e.g., "breakout")
// - subStrategy: Sub-strategy name (e.g., "ravindra_volume_imbalance")
// - timeframe: Candle timeframe (e.g., "3m", "5m", "15m")
// - price: Current market price at breakout detection
//
// Returns error if entry cannot be executed.
func (r *ChainEntryRunner) ExecuteImmediateEntry(symbol, direction, mode, strategyGroup, subStrategy, timeframe string, price float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if chain entry is enabled
	if !r.isChainEntryEnabled(ctx) {
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] Chain entry not enabled, skipping %s", symbol)
		return fmt.Errorf("chain entry system not enabled")
	}

	// Check circuit breaker
	if r.isCircuitBreakerTripped(ctx) {
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] Circuit breaker tripped, skipping %s", symbol)
		return fmt.Errorf("circuit breaker tripped")
	}

	// Check cooldown
	if r.isOnCooldown(symbol) {
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] %s on cooldown, skipping", symbol)
		return fmt.Errorf("symbol on cooldown")
	}

	// Check if symbol has open position already
	if r.hasOpenPosition(ctx, symbol) {
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] %s has open position, skipping", symbol)
		return fmt.Errorf("symbol has open position")
	}

	// Build a preliminary state for the limit check
	// (We need this to check max_concurrent_trades from sub-strategy settings)
	preliminaryState := &ChainCoinState{
		Symbol:         symbol,
		ActiveStrategy: mode,
		StrategyGroup:  strategyGroup,
		SubStrategy:    subStrategy,
	}

	// CRITICAL: Atomically reserve an entry slot to prevent race conditions
	// This replaces the old non-atomic limit check that allowed 4 positions when only 1 was allowed.
	// The limitMu mutex ensures that multiple goroutines cannot simultaneously pass the limit check.
	allowed, releaseSlot := r.tryReserveEntrySlot(ctx, preliminaryState)
	if !allowed {
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] BLOCKED: Could not reserve entry slot for %s (limit reached or pending)", symbol)
		return fmt.Errorf("could not reserve entry slot (max_concurrent_trades limit reached)")
	}

	// IMPORTANT: We now have the slot reserved. We MUST call releaseSlot() when done.
	// Use a deferred call with a flag to track if we've already released.
	slotReleased := false
	defer func() {
		if !slotReleased {
			releaseSlot()
		}
	}()

	// CRITICAL: Try to get actual SL/TP prices from pattern detection
	// This ensures correct R:R ratio for both LONG and SHORT positions
	var state *ChainCoinState

	// Try to get the state from pattern provider using the specific timeframe from breakout
	if r.stateProvider != nil && timeframe != "" {
		patternState, err := r.stateProvider.GetCoinStateForEntry(ctx, r.userID, symbol, mode, timeframe)
		if err == nil && patternState != nil && patternState.SLPrice > 0 && patternState.TPPrice > 0 {
			state = patternState
			log.Printf("[CHAIN-ENTRY-IMMEDIATE] Loaded pattern state with actual SL/TP: symbol=%s, timeframe=%s, SL=%.6f, TP=%.6f, Risk=%.6f",
				symbol, timeframe, state.SLPrice, state.TPPrice, state.RiskAmount)
		}
	}

	// Fallback: try common timeframes if specific timeframe didn't work
	if state == nil && r.stateProvider != nil {
		for _, tf := range []string{"3m", "5m", "15m", "1m"} {
			if tf == timeframe {
				continue // Already tried
			}
			patternState, err := r.stateProvider.GetCoinStateForEntry(ctx, r.userID, symbol, mode, tf)
			if err == nil && patternState != nil && patternState.SLPrice > 0 && patternState.TPPrice > 0 {
				state = patternState
				log.Printf("[CHAIN-ENTRY-IMMEDIATE] Loaded pattern state from fallback timeframe %s: symbol=%s, SL=%.6f, TP=%.6f",
					tf, symbol, state.SLPrice, state.TPPrice)
				break
			}
		}
	}

	// Fallback: Create basic state if pattern not found (uses percentage-based SL/TP)
	if state == nil {
		trend := "BULLISH"
		if direction == "short" {
			trend = "BEARISH"
		}
		state = &ChainCoinState{
			Symbol:         symbol,
			Price:          price,
			ActiveStrategy: mode,
			StrategyGroup:  strategyGroup,
			SubStrategy:    subStrategy,
			Timeframe:      timeframe,
			Direction:      strings.ToUpper(direction),
			Decision:       "READY",
			Trend1H:        trend,
			Trend15M:       trend,
			ScoreFinal:     90, // High score for breakout entries
		}
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] WARNING: No pattern state found, using percentage-based SL/TP (may have inverted R:R for SHORT)")
	} else {
		// Override strategy info in case pattern had different values
		state.StrategyGroup = strategyGroup
		state.SubStrategy = subStrategy
		state.ActiveStrategy = mode
		state.Price = price // Use current breakout price
		// Ensure direction from breakout callback is used (pattern may already have it,
		// but callback direction is authoritative)
		if direction != "" {
			state.Direction = strings.ToUpper(direction)
		}
	}

	// Load budget and leverage from user settings
	if r.repo != nil && r.userID != "" {
		// Get strategy group settings for leverage
		groupSettings, err := r.repo.GetStrategyGroupSettings(ctx, r.userID, mode, strategyGroup)
		if err == nil && groupSettings != nil {
			state.MaxLeverage = groupSettings.MaxLeverage
			log.Printf("[CHAIN-ENTRY-IMMEDIATE] Loaded leverage from strategy group: %dx", state.MaxLeverage)
		}

		// Get sub-strategy settings for budget
		subSettings, err := r.repo.GetSubStrategySettings(ctx, r.userID, mode, strategyGroup, subStrategy)
		if err == nil && subSettings != nil && len(subSettings.Settings) > 0 {
			var settingsMap map[string]interface{}
			if err := json.Unmarshal(subSettings.Settings, &settingsMap); err == nil {
				// Check for budget_allocation.assigned_budget_usd
				if budgetAlloc, ok := settingsMap["budget_allocation"].(map[string]interface{}); ok {
					if budgetUSD, ok := budgetAlloc["assigned_budget_usd"].(float64); ok && budgetUSD > 0 {
						state.BudgetUSD = budgetUSD
						log.Printf("[CHAIN-ENTRY-IMMEDIATE] Loaded budget from sub-strategy: $%.2f", budgetUSD)
					}
					if useIncremental, ok := budgetAlloc["use_incremental_equity"].(bool); ok {
						state.UseIncrementalEquity = useIncremental
						log.Printf("[CHAIN-ENTRY-IMMEDIATE] Incremental equity: %v", useIncremental)
					}
					if maxTrades, ok := budgetAlloc["max_concurrent_trades"].(float64); ok && maxTrades > 0 {
						state.MaxConcurrentTrades = int(maxTrades)
						log.Printf("[CHAIN-ENTRY-IMMEDIATE] Max concurrent trades: %d", state.MaxConcurrentTrades)
					}
				}
				// Check for pattern_detection.max_sl_percent
				if patternDet, ok := settingsMap["pattern_detection"].(map[string]interface{}); ok {
					if maxSL, ok := patternDet["max_sl_percent"].(float64); ok && maxSL > 0 {
						state.SLPercent = maxSL
						log.Printf("[CHAIN-ENTRY-IMMEDIATE] Loaded max SL from sub-strategy: %.2f%%", maxSL)
					}
				}
			}
		}
	}

	log.Printf("[CHAIN-ENTRY-IMMEDIATE] Executing breakout entry for %s: direction=%s, mode=%s, budget=$%.2f, leverage=%dx, price=%.6f",
		symbol, direction, mode, state.BudgetUSD, state.MaxLeverage, price)

	// Execute the entry (slot is reserved, will be released by defer or explicitly)
	if err := r.executeChainEntry(ctx, state); err != nil {
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] Failed to execute entry for %s: %v", symbol, err)
		r.mu.Lock()
		r.stats.FailedEntries++
		// Set cooldown to prevent infinite retry loop from tick-level breakout re-detection
		r.entryCooldowns[symbol] = time.Now().Add(r.config.EntryCooldown)
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] Set cooldown for %s until %v", symbol, r.entryCooldowns[symbol])
		failedCallback := r.onEntryFailed
		r.mu.Unlock()
		// Reset pattern to watching so new entries can be detected
		if failedCallback != nil && state.Timeframe != "" {
			log.Printf("[CHAIN-ENTRY-IMMEDIATE] Calling onEntryFailed callback for %s:%s:%s", symbol, mode, state.Timeframe)
			failedCallback(symbol, mode, state.Timeframe)
		}
		// releaseSlot() will be called by defer
		return err
	}

	r.mu.Lock()
	r.stats.SuccessfulEntries++
	r.stats.TotalEntries++
	r.stats.LastEntryTime = time.Now()
	r.mu.Unlock()

	// Explicitly release the slot now that the chain is in the database
	releaseSlot()
	slotReleased = true

	// Notify that entry order was placed (allows pattern clearing for new entries on this symbol)
	r.mu.RLock()
	callback := r.onEntryPlaced
	r.mu.RUnlock()
	if callback != nil && state.Timeframe != "" {
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] Calling onEntryPlaced callback for %s:%s:%s", symbol, mode, state.Timeframe)
		callback(symbol, mode, state.Timeframe)
	}

	log.Printf("[CHAIN-ENTRY-IMMEDIATE] SUCCESS: Breakout entry placed for %s", symbol)
	return nil
}

// CalculateMaxPositionsFromSubStrategies calculates the total max positions by summing
// max_concurrent_trades from all enabled sub-strategies. This implements the correct
// position limit calculation per Krishna's requirement.
//
// The calculation follows this hierarchy:
// - For each ENABLED mode
// - For each ENABLED strategy group in that mode
// - For each ENABLED sub-strategy in that group
// - SUM the max_concurrent_trades values
// - If sub-strategy doesn't define it, use strategy group's max_positions
//
// Returns: (currentPositions, maxPositions)
func (r *ChainEntryRunner) CalculateMaxPositionsFromSubStrategies(ctx context.Context) (current int, max int) {
	if r.repo == nil {
		log.Printf("[CHAIN-ENTRY] Warning: Repository not available for position limit calculation")
		return 0, 1 // Default to 1 max position
	}

	modes := []string{"ultra_fast", "scalp", "swing", "position"}
	strategyGroups := []string{"breakout", "trending", "range", "volatile"}
	totalMaxPositions := 0

	for _, mode := range modes {
		// Check if mode is enabled (via settings cache or similar)
		// For now, iterate through all and check at strategy group level

		for _, group := range strategyGroups {
			// Get strategy group settings
			groupSettings, err := r.repo.GetStrategyGroupSettings(ctx, r.userID, mode, group)
			if err != nil || groupSettings == nil || !groupSettings.Enabled {
				continue // Skip disabled or non-existent groups
			}

			// Get all sub-strategies for this group
			subStrategies, err := r.repo.GetAllSubStrategies(ctx, r.userID, mode, group)
			if err != nil {
				// If no sub-strategies, use group's max_positions
				if groupSettings.MaxPositions > 0 {
					totalMaxPositions += groupSettings.MaxPositions
				}
				continue
			}

			groupHasEnabledSubStrategy := false
			for _, subStrat := range subStrategies {
				if !subStrat.Enabled {
					continue
				}
				groupHasEnabledSubStrategy = true

				// Parse settings JSON to get max_concurrent_trades
				var settingsMap map[string]interface{}
				if err := json.Unmarshal(subStrat.Settings, &settingsMap); err != nil {
					continue
				}

				// Check for budget_allocation.max_concurrent_trades
				if budgetAlloc, ok := settingsMap["budget_allocation"].(map[string]interface{}); ok {
					if maxTrades, ok := budgetAlloc["max_concurrent_trades"].(float64); ok && maxTrades > 0 {
						totalMaxPositions += int(maxTrades)
						log.Printf("[CHAIN-ENTRY] Sub-strategy %s/%s/%s max_concurrent_trades: %d",
							mode, group, subStrat.SubStrategy, int(maxTrades))
					}
				}
			}

			// If no sub-strategies define max_concurrent_trades, use group's max_positions
			if !groupHasEnabledSubStrategy && groupSettings.MaxPositions > 0 {
				totalMaxPositions += groupSettings.MaxPositions
			}
		}
	}

	// Fallback to 1 if no positions configured
	if totalMaxPositions == 0 {
		totalMaxPositions = 1
	}

	// Get current active positions from order chains (ACTIVE/PARTIAL only - actual open positions)
	currentPositions := 0
	activeCount, err := r.repo.GetDB().CountActiveChains(ctx, r.userID)
	if err != nil {
		log.Printf("[CHAIN-ENTRY] Warning: Failed to count active chains: %v", err)
	} else {
		currentPositions = activeCount
	}

	return currentPositions, totalMaxPositions
}

// GetPositionLimitInfo returns current and max positions for API/UI display
func (r *ChainEntryRunner) GetPositionLimitInfo(ctx context.Context) map[string]interface{} {
	current, max := r.CalculateMaxPositionsFromSubStrategies(ctx)
	return map[string]interface{}{
		"current_positions": current,
		"max_positions":     max,
		"source":            "sub_strategy_max_concurrent_trades",
	}
}
