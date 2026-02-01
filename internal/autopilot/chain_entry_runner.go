// Package autopilot implements the Chain Entry Runner for automatic trade execution.
// This component runs independently of Ginie Autopilot when entry_decision_system = "chain".
package autopilot

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/logging"
	"binance-trading-bot/internal/orders"
	"context"
	"fmt"
	"log"
	"math"
	"sort"
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
}

// ChainStateProvider is an interface for getting coin states from Redis.
// This abstracts the decision.StateManager to avoid circular imports.
type ChainStateProvider interface {
	// GetAllCoinStates returns all coin states for a user
	GetAllChainCoinStates(ctx context.Context, userID string) ([]*ChainCoinState, error)
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

	// Runtime state
	running        bool
	stopChan       chan struct{}
	entryCooldowns map[string]time.Time // symbol -> last entry time
	mu             sync.RWMutex

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

	return &ChainEntryRunner{
		userID:           userID,
		stateProvider:    stateProvider,
		futuresClient:    futuresClient,
		chainEventWriter: chainEventWriter,
		settingsCache:    settingsCache,
		repo:             repo,
		logger:           logger,
		config:           config,
		entryCooldowns:   make(map[string]time.Time),
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

		if r.canEnterPosition(ctx, candidate) {
			if err := r.executeChainEntry(ctx, candidate); err != nil {
				log.Printf("[CHAIN-ENTRY] Failed to execute entry for %s: %v", candidate.Symbol, err)
				r.mu.Lock()
				r.stats.FailedEntries++
				r.mu.Unlock()
			} else {
				entriesExecuted++
				r.mu.Lock()
				r.stats.SuccessfulEntries++
				r.stats.TotalEntries++
				r.stats.LastEntryTime = time.Now()
				r.mu.Unlock()
			}
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

// canEnterPosition performs final checks before entry
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

	return true
}

// executeChainEntry places the entry order and records chain events
func (r *ChainEntryRunner) executeChainEntry(ctx context.Context, state *ChainCoinState) error {
	symbol := state.Symbol
	modeStr := state.ActiveStrategy
	if modeStr == "" {
		modeStr = "scalp"
	}

	// Determine direction from trend
	direction := "LONG"
	if state.Trend1H == "BEARISH" || state.Trend15M == "BEARISH" {
		direction = "SHORT"
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

	// Step 3: Get mode defaults for position size
	positionSizeUSD, slPercent, tpPercent := r.getModeDefaults(modeStr)

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

	// Step 5: Generate chain ID
	tradingMode := orders.ModeFromString(modeStr)
	modeCode := orders.ModeCode[tradingMode]
	if modeCode == "" {
		modeCode = "SCA"
	}

	now := time.Now().UTC()
	dateStr := now.Format("02Jan")
	seqNum := now.UnixNano() % 100000
	chainID := fmt.Sprintf("%s-%s-%05d", modeCode, dateStr, seqNum)
	entryClientOrderID := fmt.Sprintf("%s-E", chainID)

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

	// Step 7: Create order chain - MANDATORY for chain system integrity
	// If database write fails, abort entry to prevent orphaned positions that Ginie picks up
	if r.chainEventWriter != nil {
		_, err := r.chainEventWriter.CreateChain(ctx, orders.CreateChainRequest{
			UserID:   r.userID,
			ChainID:  chainID,
			Symbol:   symbol,
			Side:     direction,
			ModeCode: modeCode,
			IsHedge:  false,
		})
		if err != nil {
			log.Printf("[CHAIN-ENTRY] ABORTED: Failed to create order chain in database - cannot place order without chain record: %v", err)
			return fmt.Errorf("failed to create order chain (required for chain system): %w", err)
		}
	} else {
		log.Printf("[CHAIN-ENTRY] ABORTED: No chainEventWriter - cannot place order without chain tracking")
		return fmt.Errorf("chainEventWriter not available - cannot place untracked orders")
	}

	// Step 8: Place MARKET order
	orderParams := binance.FuturesOrderParams{
		Symbol:           symbol,
		Side:             orderSide,
		PositionSide:     positionSide,
		Type:             binance.FuturesOrderTypeMarket,
		Quantity:         quantity,
		NewClientOrderId: entryClientOrderID,
	}

	log.Printf("[CHAIN-ENTRY] Placing MARKET %s order for %s: qty=%.6f, positionSizeUSD=%.2f, chainID=%s",
		orderSide, symbol, quantity, positionSizeUSD, chainID)

	orderResp, err := r.futuresClient.PlaceFuturesOrder(orderParams)
	if err != nil {
		log.Printf("[CHAIN-ENTRY] FAILED to place order for %s: %v", symbol, err)
		return fmt.Errorf("failed to place entry order: %w", err)
	}

	// Step 9: Record entry placed event
	if r.chainEventWriter != nil {
		err := r.chainEventWriter.RecordEntryPlaced(ctx, chainID, orders.ChainEntryPlacedEvent{
			BinanceOrderID:       orderResp.OrderId,
			BinanceClientOrderID: entryClientOrderID,
			Price:                orderResp.AvgPrice,
			Quantity:             quantity,
			BinanceTimestamp:     orderResp.UpdateTime,
		})
		if err != nil {
			log.Printf("[CHAIN-ENTRY] Warning: Failed to record entry placed event: %v", err)
		}

		// Step 9b: For MARKET orders, if status is FILLED, record entry filled event immediately
		// This ensures the chain shows ACTIVE status with correct quantity/price
		if orderResp.Status == "FILLED" && orderResp.ExecutedQty > 0 {
			filledPrice := orderResp.AvgPrice
			if filledPrice <= 0 {
				filledPrice = currentPrice
			}
			err := r.chainEventWriter.RecordEntryFilled(ctx, chainID, orders.ChainEntryFilledEvent{
				FilledPrice:      filledPrice,
				FilledQuantity:   orderResp.ExecutedQty,
				Fees:             0, // Fees will be tracked separately via WebSocket
				BinanceTimestamp: orderResp.UpdateTime,
			})
			if err != nil {
				log.Printf("[CHAIN-ENTRY] Warning: Failed to record entry filled event: %v", err)
			} else {
				log.Printf("[CHAIN-ENTRY] Recorded entry filled: chainID=%s, price=%.6f, qty=%.6f",
					chainID, filledPrice, orderResp.ExecutedQty)
			}
		}
	}

	// Step 10: Calculate SL/TP prices
	entryPrice := orderResp.AvgPrice
	if entryPrice <= 0 {
		entryPrice = currentPrice
	}

	filledQuantity := orderResp.ExecutedQty
	if filledQuantity <= 0 {
		filledQuantity = quantity
	}

	var slPrice, tpPrice float64
	if direction == "LONG" {
		slPrice = entryPrice * (1 - slPercent/100)
		tpPrice = entryPrice * (1 + tpPercent/100)
	} else {
		slPrice = entryPrice * (1 + slPercent/100)
		tpPrice = entryPrice * (1 - tpPercent/100)
	}

	// Round SL/TP prices to appropriate precision
	slPrice = roundToTickSizeFromSymbol(slPrice, symbolInfo)
	tpPrice = roundToTickSizeFromSymbol(tpPrice, symbolInfo)

	// Step 11: Place SL/TP orders immediately to protect the position
	// CRITICAL: Position must have SL/TP orders placed within seconds of entry
	if orderResp.Status == "FILLED" && filledQuantity > 0 {
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

		// Place Stop Loss order (STOP_MARKET)
		slParams := binance.AlgoOrderParams{
			Symbol:        symbol,
			Side:          closeSide,
			PositionSide:  positionSide,
			Type:          binance.FuturesOrderTypeStopMarket,
			Quantity:      filledQuantity,
			TriggerPrice:  slPrice,
			ClosePosition: false,
			WorkingType:   binance.WorkingTypeMarkPrice,
			ClientAlgoId:  slClientOrderID,
		}

		log.Printf("[CHAIN-ENTRY] Placing STOP_MARKET SL order for %s: price=%.6f, qty=%.6f", symbol, slPrice, filledQuantity)

		slResp, err := r.futuresClient.PlaceAlgoOrder(slParams)
		if err != nil {
			log.Printf("[CHAIN-ENTRY] WARNING: Failed to place SL order for %s: %v", symbol, err)
		} else {
			log.Printf("[CHAIN-ENTRY] SL order placed: symbol=%s, algoID=%d, price=%.6f", symbol, slResp.AlgoId, slPrice)

			// Record SL placed event
			if r.chainEventWriter != nil {
				err := r.chainEventWriter.RecordSLPlaced(ctx, chainID, orders.ChainSLPlacedEvent{
					BinanceOrderID:       slResp.AlgoId,
					BinanceClientOrderID: slClientOrderID,
					Price:                slPrice,
					BinanceTimestamp:     slResp.UpdateTime,
				})
				if err != nil {
					log.Printf("[CHAIN-ENTRY] Warning: Failed to record SL placed event: %v", err)
				}
			}
		}

		// Place Take Profit order (TAKE_PROFIT_MARKET)
		tpParams := binance.AlgoOrderParams{
			Symbol:        symbol,
			Side:          closeSide,
			PositionSide:  positionSide,
			Type:          binance.FuturesOrderTypeTakeProfitMarket,
			Quantity:      filledQuantity,
			TriggerPrice:  tpPrice,
			ClosePosition: false,
			WorkingType:   binance.WorkingTypeMarkPrice,
			ClientAlgoId:  tpClientOrderID,
		}

		log.Printf("[CHAIN-ENTRY] Placing TAKE_PROFIT_MARKET TP order for %s: price=%.6f, qty=%.6f", symbol, tpPrice, filledQuantity)

		tpResp, err := r.futuresClient.PlaceAlgoOrder(tpParams)
		if err != nil {
			log.Printf("[CHAIN-ENTRY] WARNING: Failed to place TP order for %s: %v", symbol, err)
		} else {
			log.Printf("[CHAIN-ENTRY] TP order placed: symbol=%s, algoID=%d, price=%.6f", symbol, tpResp.AlgoId, tpPrice)

			// Record TP placed event
			if r.chainEventWriter != nil {
				err := r.chainEventWriter.RecordTPPlaced(ctx, chainID, orders.ChainTPPlacedEvent{
					BinanceOrderID:       tpResp.AlgoId,
					BinanceClientOrderID: tpClientOrderID,
					Price:                tpPrice,
					BinanceTimestamp:     tpResp.UpdateTime,
				})
				if err != nil {
					log.Printf("[CHAIN-ENTRY] Warning: Failed to record TP placed event: %v", err)
				}
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

// SetFuturesClient updates the futures client (used when client is refreshed)
func (r *ChainEntryRunner) SetFuturesClient(client binance.FuturesClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.futuresClient = client
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
// - price: Current market price at breakout detection
//
// Returns error if entry cannot be executed.
func (r *ChainEntryRunner) ExecuteImmediateEntry(symbol, direction, mode string, price float64) error {
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

	// Create ChainCoinState for immediate entry
	trend := "BULLISH"
	if direction == "short" {
		trend = "BEARISH"
	}

	state := &ChainCoinState{
		Symbol:         symbol,
		Price:          price,
		ActiveStrategy: mode,
		Decision:       "READY",
		Trend1H:        trend,
		Trend15M:       trend,
		ScoreFinal:     90, // High score for breakout entries
	}

	log.Printf("[CHAIN-ENTRY-IMMEDIATE] Executing breakout entry for %s: direction=%s, mode=%s, price=%.6f",
		symbol, direction, mode, price)

	// Execute the entry
	if err := r.executeChainEntry(ctx, state); err != nil {
		log.Printf("[CHAIN-ENTRY-IMMEDIATE] Failed to execute entry for %s: %v", symbol, err)
		r.mu.Lock()
		r.stats.FailedEntries++
		r.mu.Unlock()
		return err
	}

	r.mu.Lock()
	r.stats.SuccessfulEntries++
	r.stats.TotalEntries++
	r.stats.LastEntryTime = time.Now()
	r.mu.Unlock()

	log.Printf("[CHAIN-ENTRY-IMMEDIATE] SUCCESS: Breakout entry placed for %s", symbol)
	return nil
}
