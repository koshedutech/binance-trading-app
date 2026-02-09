// Package coinprofiler provides real-time coin data collection via WebSocket
// for the Chain Trading System. It serves as the central data hub for both
// Entry Decision (strategies) and Exit Decision (positions).
package coinprofiler

import (
	"binance-trading-bot/internal/logging"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// LogPrefix is the standard prefix for all Coin Profiler log messages.
const LogPrefix = "[COIN-PROFILER]"

// CoinProfiler is the central data hub for the Chain Trading System.
// It collects real-time data via WebSocket based on strategy and position requirements.
type CoinProfiler struct {
	// Configuration
	config *CoinProfilerConfig

	// Dependencies
	logger             *logging.Logger
	wsManager          *WebSocketManager        // WebSocket manager for real-time data
	historicalProvider HistoricalDataProvider   // For fetching historical candles on startup

	// State
	running         bool
	connectionState ConnectionState
	startedAt       time.Time
	reconnectCount  int
	lastError       string
	lastUpdateTime  time.Time

	// Subscription management
	subscriptions    map[string]*SubscriptionRequest // symbol -> request
	coinData         map[string]*CoinData            // symbol -> data
	combinedReqs     *CombinedRequirements           // Current combined requirements (for API)

	// Historical candle storage for pattern detection
	// Key: "SYMBOL:TIMEFRAME" (e.g., "BTCUSDT:5m")
	candleHistory   map[string]*CandleHistory
	candleHistoryMu sync.RWMutex

	// Callback for candle close events (used by pattern matcher)
	onCandleClose CandleCloseCallback

	// Callback for real-time price updates (used for tick-level breakout detection)
	onPriceUpdate PriceUpdateCallback

	// Callback for volume progress updates (used for real-time volume ratio display)
	onVolumeProgress VolumeProgressCallback

	// Volume progress configuration (set by pattern matcher)
	volumeProgressConfig struct {
		RequiredRatio   float64 // e.g., 3.0 for 3x volume spike
		LookbackCandles int     // e.g., 5 for 5-candle average
	}

	// Metrics
	updateCount      int64
	updatesPerSecond float64

	// Context for graceful shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc

	// Control channels
	stopChan   chan struct{}
	updateChan chan interface{} // Receives WebSocket messages

	// Synchronization
	mu sync.RWMutex
	wg sync.WaitGroup
}

// CandleCloseCallback is called when a candle closes (is_closed_bar: true).
// This enables real-time pattern evaluation.
type CandleCloseCallback func(symbol, timeframe string, candles []HistoricalCandle)

// PriceUpdateCallback is called on every price update (tick-level).
// This enables real-time breakout detection without waiting for candle close.
// Parameters: symbol, timeframe, current price, current high of forming candle
type PriceUpdateCallback func(symbol, timeframe string, price, currentHigh, currentLow float64)

// VolumeProgressData contains real-time volume ratio information for UI display.
type VolumeProgressData struct {
	Symbol              string  `json:"symbol"`
	Timeframe           string  `json:"timeframe"`
	CurrentVolume       float64 `json:"current_volume"`        // Current forming candle's volume
	AverageVolume       float64 `json:"average_volume"`        // Average of last N closed candles
	CurrentRatio        float64 `json:"current_ratio"`         // CurrentVolume / AverageVolume
	RequiredRatio       float64 `json:"required_ratio"`        // Threshold to trigger (e.g., 3.0)
	ProgressPercent     float64 `json:"progress_percent"`      // (CurrentRatio / RequiredRatio) * 100
	CandleDirection     string  `json:"candle_direction"`      // "bullish" or "bearish"
	IsApproachingSpike  bool    `json:"is_approaching_spike"`  // True if > 50% of threshold
	TimeRemainingMs     int64   `json:"time_remaining_ms"`     // Milliseconds until candle close
	LookbackCandles     int     `json:"lookback_candles"`      // How many candles used for average
	CurrentPrice        float64 `json:"current_price"`         // Real-time current price (from close)
}

// VolumeProgressCallback is called on every kline update with current volume progress.
// This enables real-time volume ratio display in the UI.
type VolumeProgressCallback func(data VolumeProgressData)

// NewCoinProfiler creates a new Coin Profiler instance.
// If config is nil, default configuration will be used.
// If logger is nil, a standard logger will be used.
func NewCoinProfiler(config *CoinProfilerConfig, logger *logging.Logger) *CoinProfiler {
	if config == nil {
		config = DefaultCoinProfilerConfig()
	}

	// Create a context that will be cancelled on stop
	ctx, cancel := context.WithCancel(context.Background())

	cp := &CoinProfiler{
		config:          config,
		logger:          logger,
		connectionState: ConnectionStateDisconnected,
		subscriptions:   make(map[string]*SubscriptionRequest),
		coinData:        make(map[string]*CoinData),
		candleHistory:   make(map[string]*CandleHistory),
		ctx:             ctx,
		cancelFunc:      cancel,
		stopChan:        make(chan struct{}),
		updateChan:      make(chan interface{}, 1000), // Buffered channel for updates
	}

	// Create WebSocket manager (connects to Binance Futures WebSocket)
	cp.wsManager = NewWebSocketManager(config, cp)

	return cp
}

// Start starts the Coin Profiler service.
// It initializes WebSocket connections and begins data collection.
// Returns an error if the profiler is already running.
func (cp *CoinProfiler) Start() error {
	cp.mu.Lock()
	if cp.running {
		cp.mu.Unlock()
		return fmt.Errorf("%s already running", LogPrefix)
	}

	cp.running = true
	cp.startedAt = time.Now()
	cp.connectionState = ConnectionStateConnecting
	cp.reconnectCount = 0
	cp.lastError = ""

	// Reset context for new run
	cp.ctx, cp.cancelFunc = context.WithCancel(context.Background())
	cp.stopChan = make(chan struct{})

	cp.mu.Unlock()

	cp.log("Starting Coin Profiler service")

	// Start the WebSocket manager for real-time data
	if cp.wsManager != nil {
		if err := cp.wsManager.Start(); err != nil {
			cp.mu.Lock()
			cp.running = false
			cp.connectionState = ConnectionStateDisconnected
			cp.mu.Unlock()
			return fmt.Errorf("%s failed to start WebSocket: %w", LogPrefix, err)
		}
		cp.log("WebSocket manager started")
	}

	// Start the main processing loop
	cp.wg.Add(1)
	go cp.runLoop()

	// Start metrics collection
	cp.wg.Add(1)
	go cp.metricsLoop()

	// Start connection state sync loop
	cp.wg.Add(1)
	go cp.syncConnectionState()

	cp.log("Coin Profiler started successfully")
	return nil
}

// Stop stops the Coin Profiler service gracefully.
// It closes WebSocket connections and waits for goroutines to finish.
// Returns an error if the profiler is not running.
// Thread-safe: can be called concurrently without panic.
func (cp *CoinProfiler) Stop() error {
	cp.mu.Lock()
	if !cp.running {
		cp.mu.Unlock()
		return fmt.Errorf("%s not running", LogPrefix)
	}

	cp.running = false
	cp.connectionState = ConnectionStateDisconnected

	// Capture stopChan before unlocking to avoid race with Start()
	stopChan := cp.stopChan
	cp.mu.Unlock()

	cp.log("Stopping Coin Profiler service")

	// Stop WebSocket manager first
	if cp.wsManager != nil {
		if err := cp.wsManager.Stop(); err != nil {
			cp.log("Warning: WebSocket manager stop error: %v", err)
		}
		cp.log("WebSocket manager stopped")
	}

	// Signal all goroutines to stop
	cp.cancelFunc()

	// Use select to safely close - prevents panic if already closed
	select {
	case <-stopChan:
		// Already closed, do nothing
	default:
		close(stopChan)
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		cp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		cp.log("Coin Profiler stopped gracefully")
	case <-time.After(10 * time.Second):
		cp.log("Coin Profiler stop timed out after 10 seconds")
	}

	return nil
}

// IsRunning returns whether the Coin Profiler is currently running.
func (cp *CoinProfiler) IsRunning() bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.running
}

// GetStatus returns the current status of the Coin Profiler.
func (cp *CoinProfiler) GetStatus() *CoinProfilerStatus {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// Get WebSocket connection status
	connected := cp.connectionState == ConnectionStateConnected
	reconnectCount := cp.reconnectCount
	lastError := cp.lastError

	if cp.wsManager != nil {
		wsStatus := cp.wsManager.GetStatus()
		connected = wsStatus.Connected
		reconnectCount = wsStatus.ReconnectCount
		if wsStatus.LastError != "" {
			lastError = wsStatus.LastError
		}
	}

	status := &CoinProfilerStatus{
		Running:           cp.running,
		Connected:         connected,
		SymbolCount:       len(cp.coinData),
		SubscriptionCount: len(cp.subscriptions),
		UpdatesPerSecond:  cp.updatesPerSecond,
		LastUpdateTime:    cp.lastUpdateTime,
		LastError:         lastError,
		ReconnectCount:    reconnectCount,
		StartedAt:         cp.startedAt,
	}

	// Calculate uptime
	if cp.running && !cp.startedAt.IsZero() {
		uptime := time.Since(cp.startedAt)
		status.Uptime = formatUptime(uptime)
	}

	return status
}

// GetConfig returns the current configuration.
func (cp *CoinProfiler) GetConfig() *CoinProfilerConfig {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// Return a copy to prevent modification
	configCopy := *cp.config
	return &configCopy
}

// SetConfig updates the configuration.
// Note: Some configuration changes may require a restart to take effect.
func (cp *CoinProfiler) SetConfig(config *CoinProfilerConfig) {
	if config == nil {
		return
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.config = config
	cp.log("Configuration updated")
}

// GetCoinData returns data for a specific symbol, or nil if not found.
// Returns a deep copy to prevent caller modifications from affecting internal state.
func (cp *CoinProfiler) GetCoinData(symbol string) *CoinData {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if data, exists := cp.coinData[symbol]; exists {
		// Return a deep copy to prevent modification
		dataCopy := *data
		// Deep copy the Timeframes map
		if data.Timeframes != nil {
			dataCopy.Timeframes = make(map[string]*TimeframeData, len(data.Timeframes))
			for k, v := range data.Timeframes {
				tfCopy := *v
				dataCopy.Timeframes[k] = &tfCopy
			}
		}
		// Deep copy the Strategies slice
		if data.Strategies != nil {
			dataCopy.Strategies = make([]string, len(data.Strategies))
			copy(dataCopy.Strategies, data.Strategies)
		}
		return &dataCopy
	}
	return nil
}

// GetAllCoinData returns all currently tracked coin data.
// Returns deep copies to prevent caller modifications from affecting internal state.
func (cp *CoinProfiler) GetAllCoinData() map[string]*CoinData {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	// Return deep copies to prevent modification
	result := make(map[string]*CoinData, len(cp.coinData))
	for symbol, data := range cp.coinData {
		dataCopy := *data
		// Deep copy the Timeframes map
		if data.Timeframes != nil {
			dataCopy.Timeframes = make(map[string]*TimeframeData, len(data.Timeframes))
			for k, v := range data.Timeframes {
				tfCopy := *v
				dataCopy.Timeframes[k] = &tfCopy
			}
		}
		// Deep copy the Strategies slice
		if data.Strategies != nil {
			dataCopy.Strategies = make([]string, len(data.Strategies))
			copy(dataCopy.Strategies, data.Strategies)
		}
		result[symbol] = &dataCopy
	}
	return result
}

// GetAllCoinDataCtx returns all currently tracked coin data (interface-compliant version).
// This method implements the CoinDataProvider interface.
func (cp *CoinProfiler) GetAllCoinDataCtx(ctx context.Context) (map[string]*CoinData, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return cp.GetAllCoinData(), nil
}

// GetCoinDataCtx returns data for a specific symbol (context-aware).
// This method implements the CoinDataProvider interface.
func (cp *CoinProfiler) GetCoinDataCtx(ctx context.Context, symbol string) (*CoinData, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cp.mu.RLock()
	defer cp.mu.RUnlock()

	data, exists := cp.coinData[symbol]
	if !exists {
		return nil, fmt.Errorf("symbol %s not found", symbol)
	}

	// Return deep copy
	dataCopy := *data
	if data.Timeframes != nil {
		dataCopy.Timeframes = make(map[string]*TimeframeData, len(data.Timeframes))
		for k, v := range data.Timeframes {
			tfCopy := *v
			dataCopy.Timeframes[k] = &tfCopy
		}
	}
	return &dataCopy, nil
}

// AddSubscription adds a new subscription request.
// The subscription will be merged with existing subscriptions for the same symbol.
func (cp *CoinProfiler) AddSubscription(req *SubscriptionRequest) error {
	if req == nil || req.Symbol == "" {
		return fmt.Errorf("%s invalid subscription request", LogPrefix)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	existing, exists := cp.subscriptions[req.Symbol]
	if exists {
		// Merge timeframes
		timeframeSet := make(map[string]bool)
		for _, tf := range existing.Timeframes {
			timeframeSet[tf] = true
		}
		for _, tf := range req.Timeframes {
			timeframeSet[tf] = true
		}
		merged := make([]string, 0, len(timeframeSet))
		for tf := range timeframeSet {
			merged = append(merged, tf)
		}
		existing.Timeframes = merged

		// When sources differ, position takes priority over strategy.
		// This prevents the entry decision engine from scanning for new entries
		// on symbols that already have an active position (prevents duplicate orders).
		if existing.Source != req.Source {
			if existing.Source == DataSourcePosition || req.Source == DataSourcePosition {
				existing.Source = DataSourcePosition
			}
		}

		// Update existing coinData source if it exists
		if coinData, ok := cp.coinData[req.Symbol]; ok {
			coinData.Source = existing.Source
		}

		cp.logDebug("Updated subscription for %s: timeframes=%v, source=%s", req.Symbol, merged, existing.Source)
	} else {
		cp.subscriptions[req.Symbol] = req
		cp.logDebug("Added subscription for %s: timeframes=%v, source=%s", req.Symbol, req.Timeframes, req.Source)
	}

	return nil
}

// RemoveSubscription removes a subscription for a symbol.
func (cp *CoinProfiler) RemoveSubscription(symbol string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, exists := cp.subscriptions[symbol]; exists {
		delete(cp.subscriptions, symbol)
		delete(cp.coinData, symbol)
		cp.logDebug("Removed subscription for %s", symbol)
	}
}

// UpdateSymbolToPosition instantly updates a symbol's source to "position" with the given timeframe.
// This is called when an entry fills - provides instant coin profiler update without heavy re-initialization.
// The timeframe should be the entry strategy's timeframe (e.g., "3m") not generic exit timeframes.
func (cp *CoinProfiler) UpdateSymbolToPosition(symbol, timeframe, mode string) {
	if symbol == "" {
		return
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Update subscription
	if sub, exists := cp.subscriptions[symbol]; exists {
		sub.Source = DataSourcePosition
		if timeframe != "" {
			// Replace timeframes with the entry timeframe (exact match for position monitoring)
			sub.Timeframes = []string{timeframe}
		}
	} else {
		// Create new subscription for this position symbol
		timeframes := []string{timeframe}
		if timeframe == "" {
			timeframes = []string{"5m"} // Fallback
		}
		cp.subscriptions[symbol] = &SubscriptionRequest{
			Symbol:     symbol,
			Timeframes: timeframes,
			Source:     DataSourcePosition,
		}
	}

	// Update coinData source
	if coinData, exists := cp.coinData[symbol]; exists {
		coinData.Source = DataSourcePosition
	}

	// Update combinedReqs if it exists
	if cp.combinedReqs != nil {
		if symReq, exists := cp.combinedReqs.BySymbol[symbol]; exists {
			symReq.Source = DataSourcePosition
			if timeframe != "" {
				symReq.Timeframes = []string{timeframe}
			}
		}
	}

	log.Printf("%s Updated symbol %s to position source (timeframe=%s, mode=%s)", LogPrefix, symbol, timeframe, mode)
}

// UpdateSymbolToStrategy reverts a symbol's source back to "strategy" when a position closes.
// This is called when a chain closes - provides instant coin profiler update.
// If the symbol was only tracked because of a position (no strategy requirements),
// it is removed entirely from subscriptions, coinData, and combinedReqs.
func (cp *CoinProfiler) UpdateSymbolToStrategy(symbol string) {
	if symbol == "" {
		return
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Check if this symbol has strategy-based requirements in combinedReqs.
	// If the symbol was only added because of a position (e.g., auto-detected from Binance
	// without a DB chain), it won't have any strategy references and should be removed entirely.
	hasStrategyBacking := false
	if cp.combinedReqs != nil {
		if symReq, exists := cp.combinedReqs.BySymbol[symbol]; exists {
			hasStrategyBacking = len(symReq.Strategies) > 0
		}
	}

	if hasStrategyBacking {
		// Symbol is part of the strategy watchlist - revert source to "strategy"
		if sub, exists := cp.subscriptions[symbol]; exists {
			sub.Source = DataSourceStrategy
		}
		if coinData, exists := cp.coinData[symbol]; exists {
			coinData.Source = DataSourceStrategy
		}
		if cp.combinedReqs != nil {
			if symReq, exists := cp.combinedReqs.BySymbol[symbol]; exists {
				symReq.Source = DataSourceStrategy
				symReq.Positions = nil // Clear position references
			}
		}
		log.Printf("%s Reverted symbol %s to strategy source (position closed, still in watchlist)", LogPrefix, symbol)
	} else {
		// Symbol was tracked ONLY for a position - remove it entirely
		delete(cp.subscriptions, symbol)
		delete(cp.coinData, symbol)
		if cp.combinedReqs != nil {
			delete(cp.combinedReqs.BySymbol, symbol)
			// Remove from AllSymbols list
			newAllSymbols := make([]string, 0, len(cp.combinedReqs.AllSymbols))
			for _, s := range cp.combinedReqs.AllSymbols {
				if s != symbol {
					newAllSymbols = append(newAllSymbols, s)
				}
			}
			cp.combinedReqs.AllSymbols = newAllSymbols
			if cp.combinedReqs.PositionCount > 0 {
				cp.combinedReqs.PositionCount--
			}
		}
		log.Printf("%s Removed symbol %s entirely (position closed, not in any watchlist)", LogPrefix, symbol)
	}
}

// RebuildCapacity recalculates coin profiler capacity based on active chain count.
// This is a synchronous method called by the PositionLifecycleCoordinator after a chain closes.
// Returns (capacityUsed, scanningEnabled) where:
//   - capacityUsed is the number of active chains (positions consuming capacity)
//   - scanningEnabled is true if activeChainCount < maxConcurrent (room for new entries)
func (cp *CoinProfiler) RebuildCapacity(activeChainCount, maxConcurrent int) (int, bool) {
	scanningEnabled := activeChainCount < maxConcurrent
	log.Printf("%s RebuildCapacity: active=%d, max=%d, scanning=%v",
		LogPrefix, activeChainCount, maxConcurrent, scanningEnabled)
	return activeChainCount, scanningEnabled
}

// GetSubscriptions returns all current subscriptions.
func (cp *CoinProfiler) GetSubscriptions() map[string]*SubscriptionRequest {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make(map[string]*SubscriptionRequest, len(cp.subscriptions))
	for symbol, sub := range cp.subscriptions {
		subCopy := *sub
		result[symbol] = &subCopy
	}
	return result
}

// GetCombinedRequirements returns the current combined requirements.
// This includes detailed information about strategy and position sources.
func (cp *CoinProfiler) GetCombinedRequirements() *CombinedRequirements {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if cp.combinedReqs == nil {
		return nil
	}

	// Return a copy to prevent modification
	return cp.combinedReqs
}

// runLoop is the main processing loop for the Coin Profiler.
func (cp *CoinProfiler) runLoop() {
	defer cp.wg.Done()

	cp.log("Main processing loop started")

	for {
		select {
		case <-cp.ctx.Done():
			cp.log("Main processing loop stopped (context cancelled)")
			return
		case <-cp.stopChan:
			cp.log("Main processing loop stopped (stop signal)")
			return
		case msg := <-cp.updateChan:
			cp.processUpdate(msg)
		}
	}
}

// metricsLoop calculates metrics periodically.
func (cp *CoinProfiler) metricsLoop() {
	defer cp.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var prevUpdateCount int64

	for {
		select {
		case <-cp.ctx.Done():
			return
		case <-cp.stopChan:
			return
		case <-ticker.C:
			cp.mu.Lock()
			currentCount := cp.updateCount
			cp.updatesPerSecond = float64(currentCount - prevUpdateCount)
			prevUpdateCount = currentCount
			cp.mu.Unlock()
		}
	}
}

// syncConnectionState syncs the connection state from the WebSocket manager.
func (cp *CoinProfiler) syncConnectionState() {
	defer cp.wg.Done()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cp.ctx.Done():
			return
		case <-cp.stopChan:
			return
		case <-ticker.C:
			if cp.wsManager != nil {
				wsStatus := cp.wsManager.GetStatus()
				cp.mu.Lock()
				if wsStatus.Connected {
					cp.connectionState = ConnectionStateConnected
				} else if wsStatus.ConnectionState == "connecting" {
					cp.connectionState = ConnectionStateConnecting
				} else {
					cp.connectionState = ConnectionStateDisconnected
				}
				cp.reconnectCount = wsStatus.ReconnectCount
				cp.mu.Unlock()
			}
		}
	}
}

// processUpdate processes an incoming WebSocket message.
func (cp *CoinProfiler) processUpdate(msg interface{}) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.updateCount++
	cp.lastUpdateTime = time.Now()

	// Actual WebSocket message processing will be implemented in Story 14.4
	// For now, this is a placeholder that demonstrates the pattern
	if cp.config.DebugMode {
		cp.logDebug("Received update: %T", msg)
	}
}

// log logs a message with the standard prefix.
func (cp *CoinProfiler) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if cp.logger != nil {
		cp.logger.Info(LogPrefix + " " + msg)
	} else {
		log.Printf("%s %s", LogPrefix, msg)
	}
}

// logDebug logs a debug message (only if debug mode is enabled).
func (cp *CoinProfiler) logDebug(format string, args ...interface{}) {
	if cp.config != nil && cp.config.DebugMode {
		msg := fmt.Sprintf(format, args...)
		if cp.logger != nil {
			cp.logger.Debug(LogPrefix + " " + msg)
		} else {
			log.Printf("%s [DEBUG] %s", LogPrefix, msg)
		}
	}
}

// logError logs an error and stores it in lastError.
// NOTE: This method is safe to call from any context - it uses its own lock.
func (cp *CoinProfiler) logError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Store error in a separate lock scope to avoid deadlock if called from locked context
	cp.mu.Lock()
	cp.lastError = msg
	cp.mu.Unlock()

	// Log after releasing lock to prevent blocking
	if cp.logger != nil {
		cp.logger.Error(LogPrefix + " " + msg)
	} else {
		log.Printf("%s [ERROR] %s", LogPrefix, msg)
	}
}

// formatUptime formats a duration as a human-readable uptime string.
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// Context returns the profiler's context (for integration with other components).
func (cp *CoinProfiler) Context() context.Context {
	return cp.ctx
}

// InitializeSubscriptions sets up WebSocket subscriptions based on enabled strategies and open positions.
// This is the key method that connects strategy requirements to actual WebSocket streams.
//
// Flow:
// 1. Read enabled strategies from the reader (database)
// 2. Aggregate requirements using the RequirementAggregator
// 3. Combine with position requirements (for exit monitoring)
// 4. Subscribe to the required WebSocket streams
//
// Parameters:
//   - ctx: Context for the operation
//   - reader: Interface to read enabled strategies (typically wraps database.Repository)
//   - positions: Open positions that need price monitoring (can be nil)
//
// Returns an error if aggregation or subscription fails.
func (cp *CoinProfiler) InitializeSubscriptions(ctx context.Context, reader EnabledStrategyReader, positions []Position) error {
	if reader == nil {
		return fmt.Errorf("%s reader is nil", LogPrefix)
	}

	cp.mu.RLock()
	running := cp.running
	cp.mu.RUnlock()

	if !running {
		return fmt.Errorf("%s not running", LogPrefix)
	}

	cp.log("Initializing subscriptions from enabled strategies...")

	// Create aggregator and fetch requirements
	aggregator := NewRequirementAggregator(reader)

	// Get user ID from context (if available) or use empty string
	userID := ""
	if uid := ctx.Value("user_id"); uid != nil {
		if s, ok := uid.(string); ok {
			userID = s
		}
	}

	// Aggregate strategy requirements
	strategyReqs, err := aggregator.Aggregate(ctx, userID)
	if err != nil {
		cp.logError("Failed to aggregate strategy requirements: %v", err)
		return fmt.Errorf("failed to aggregate requirements: %w", err)
	}

	cp.log("Aggregated %d strategies: timeframes=%v, fields=%v",
		strategyReqs.TotalStrategies,
		strategyReqs.AllTimeframes,
		strategyReqs.AllDataFields)

	// Get position requirements
	positionReqs := GetPositionRequirements(positions)
	cp.log("Got %d position requirements", len(positionReqs))

	// Combine strategy and position requirements
	combinedReqs := CombineRequirements(strategyReqs, positionReqs)

	cp.log("Combined requirements: %d symbols, %d timeframes",
		len(combinedReqs.AllSymbols),
		len(combinedReqs.AllTimeframes))

	// Update internal subscriptions map
	cp.mu.Lock()
	for symbol, symReq := range combinedReqs.BySymbol {
		cp.subscriptions[symbol] = &SubscriptionRequest{
			Symbol:     symbol,
			Timeframes: symReq.Timeframes,
			Source:     symReq.Source,
		}
	}
	cp.mu.Unlock()

	// Subscribe via WebSocket manager
	if cp.wsManager != nil {
		if err := cp.wsManager.UpdateSubscriptions(combinedReqs); err != nil {
			cp.logError("Failed to update WebSocket subscriptions: %v", err)
			return fmt.Errorf("failed to subscribe: %w", err)
		}
		cp.log("WebSocket subscriptions updated successfully")
	}

	return nil
}

// SubscribeToSymbols subscribes to specific symbols with the given timeframes.
// This is a simpler alternative to InitializeSubscriptions when you already know
// which symbols to track (e.g., from a coin scanner or manual selection).
func (cp *CoinProfiler) SubscribeToSymbols(symbols []string, timeframes []string) error {
	if len(symbols) == 0 || len(timeframes) == 0 {
		return nil
	}

	cp.mu.RLock()
	running := cp.running
	cp.mu.RUnlock()

	if !running {
		return fmt.Errorf("%s not running", LogPrefix)
	}

	cp.log("Subscribing to %d symbols with timeframes %v", len(symbols), timeframes)

	// Build combined requirements
	combinedReqs := &CombinedRequirements{
		AllSymbols:    symbols,
		AllTimeframes: timeframes,
		AllDataFields: []string{"ohlc", "volume", "taker_buy_volume"},
		BySymbol:      make(map[string]*SymbolRequirements),
	}

	for _, symbol := range symbols {
		combinedReqs.BySymbol[symbol] = &SymbolRequirements{
			Symbol:     symbol,
			Timeframes: timeframes,
			DataFields: []string{"ohlc", "volume", "taker_buy_volume"},
			Source:     DataSourceStrategy,
		}
	}

	// Update internal subscriptions
	cp.mu.Lock()
	for _, symbol := range symbols {
		cp.subscriptions[symbol] = &SubscriptionRequest{
			Symbol:     symbol,
			Timeframes: timeframes,
			Source:     DataSourceStrategy,
		}
	}
	cp.mu.Unlock()

	// Subscribe via WebSocket
	if cp.wsManager != nil {
		if err := cp.wsManager.UpdateSubscriptions(combinedReqs); err != nil {
			return fmt.Errorf("failed to subscribe: %w", err)
		}
	}

	cp.log("Subscribed to %d symbols", len(symbols))

	// Prefetch historical candles for immediate pattern detection
	// This runs in background to avoid blocking the subscription
	go func() {
		if err := cp.PrefetchHistoricalCandles(symbols, timeframes); err != nil {
			cp.logError("Failed to prefetch historical candles: %v", err)
		}
	}()

	return nil
}

// GetWebSocketManager returns the WebSocket manager (for advanced operations).
func (cp *CoinProfiler) GetWebSocketManager() *WebSocketManager {
	return cp.wsManager
}

// SetCoinUpdateCallback sets a callback to be called when coin data is updated in real-time.
// This is used to broadcast updates to WebSocket clients for live UI updates.
func (cp *CoinProfiler) SetCoinUpdateCallback(callback CoinUpdateCallback) {
	if cp.wsManager != nil {
		cp.wsManager.SetCoinUpdateCallback(callback)
	}
}

// SetSubscriptionsFromCombined updates subscriptions from pre-computed combined requirements.
// This is useful when the caller has already aggregated strategy and position requirements.
// The combined requirements should contain the symbols and timeframes to subscribe to.
func (cp *CoinProfiler) SetSubscriptionsFromCombined(combined *CombinedRequirements) error {
	if combined == nil {
		return nil
	}

	cp.mu.RLock()
	running := cp.running
	cp.mu.RUnlock()

	if !running {
		return fmt.Errorf("%s not running", LogPrefix)
	}

	cp.log("Setting subscriptions: %d symbols, %d timeframes",
		len(combined.AllSymbols), len(combined.AllTimeframes))

	// Update internal subscriptions map and store combined requirements
	// CRITICAL: Remove stale subscriptions that are no longer in the new combined set.
	// Without this cleanup, symbols that were tracked only for positions would remain
	// with source "position" even after the position closes.
	cp.mu.Lock()
	cp.combinedReqs = combined // Store for API access

	// Remove subscriptions that are no longer in the new combined requirements
	for symbol := range cp.subscriptions {
		if _, exists := combined.BySymbol[symbol]; !exists {
			delete(cp.subscriptions, symbol)
			delete(cp.coinData, symbol)
			cp.logDebug("Removed stale subscription for %s (not in new requirements)", symbol)
		}
	}

	// Add/update subscriptions from new combined requirements
	for symbol, symReq := range combined.BySymbol {
		cp.subscriptions[symbol] = &SubscriptionRequest{
			Symbol:     symbol,
			Timeframes: symReq.Timeframes,
			Source:     symReq.Source,
		}
		// Also update existing coinData source if it exists
		if coinData, exists := cp.coinData[symbol]; exists {
			coinData.Source = symReq.Source
		}
	}
	cp.mu.Unlock()

	// Subscribe via WebSocket manager
	if cp.wsManager != nil {
		if err := cp.wsManager.UpdateSubscriptions(combined); err != nil {
			cp.logError("Failed to update WebSocket subscriptions: %v", err)
			return fmt.Errorf("failed to subscribe: %w", err)
		}
		cp.log("WebSocket subscriptions updated: %d streams", len(combined.BySymbol))
	}

	// Prefetch historical candles for immediate pattern detection
	// This runs in background to avoid blocking the subscription
	if len(combined.AllSymbols) > 0 && len(combined.AllTimeframes) > 0 {
		go func() {
			if err := cp.PrefetchHistoricalCandles(combined.AllSymbols, combined.AllTimeframes); err != nil {
				cp.logError("Failed to prefetch historical candles: %v", err)
			}
		}()
	}

	return nil
}

// ============================================================================
// CANDLE HISTORY MANAGEMENT
// Epic 14: Entry Decision Strategy Requirements & Real-Time Monitoring
// ============================================================================

// AddClosedCandle stores a closed candle in the history buffer for pattern detection.
// This should be called when a candle closes (is_closed_bar: true).
func (cp *CoinProfiler) AddClosedCandle(symbol, timeframe string, candle HistoricalCandle) {
	key := CandleHistoryKey(symbol, timeframe)

	cp.candleHistoryMu.Lock()
	defer cp.candleHistoryMu.Unlock()

	// Create history buffer if not exists
	if cp.candleHistory[key] == nil {
		cp.candleHistory[key] = NewCandleHistory(DefaultCandleHistorySize)
	}

	// Add the candle
	cp.candleHistory[key].Add(candle)

	// Trigger callback if set
	if cp.onCandleClose != nil {
		candles := cp.candleHistory[key].GetAll()
		// Call callback in goroutine to avoid blocking
		go cp.onCandleClose(symbol, timeframe, candles)
	}
}

// GetCandleHistory returns historical candles for a symbol+timeframe combination.
// Returns nil if no history exists.
func (cp *CoinProfiler) GetCandleHistory(symbol, timeframe string) []HistoricalCandle {
	key := CandleHistoryKey(symbol, timeframe)

	cp.candleHistoryMu.RLock()
	defer cp.candleHistoryMu.RUnlock()

	history := cp.candleHistory[key]
	if history == nil {
		return nil
	}

	return history.GetAll()
}

// GetRecentCandles returns the most recent n candles for a symbol+timeframe.
// Returns nil if no history exists.
func (cp *CoinProfiler) GetRecentCandles(symbol, timeframe string, count int) []HistoricalCandle {
	key := CandleHistoryKey(symbol, timeframe)

	cp.candleHistoryMu.RLock()
	defer cp.candleHistoryMu.RUnlock()

	history := cp.candleHistory[key]
	if history == nil {
		return nil
	}

	return history.GetRecent(count)
}

// GetCandleHistoryCount returns the number of historical candles stored for a symbol+timeframe.
func (cp *CoinProfiler) GetCandleHistoryCount(symbol, timeframe string) int {
	key := CandleHistoryKey(symbol, timeframe)

	cp.candleHistoryMu.RLock()
	defer cp.candleHistoryMu.RUnlock()

	history := cp.candleHistory[key]
	if history == nil {
		return 0
	}

	return history.Count()
}

// ClearCandleHistory clears the candle history for a symbol+timeframe.
func (cp *CoinProfiler) ClearCandleHistory(symbol, timeframe string) {
	key := CandleHistoryKey(symbol, timeframe)

	cp.candleHistoryMu.Lock()
	defer cp.candleHistoryMu.Unlock()

	if history := cp.candleHistory[key]; history != nil {
		history.Clear()
	}
}

// ClearAllCandleHistory clears all candle history.
func (cp *CoinProfiler) ClearAllCandleHistory() {
	cp.candleHistoryMu.Lock()
	defer cp.candleHistoryMu.Unlock()

	cp.candleHistory = make(map[string]*CandleHistory)
}

// SetCandleCloseCallback sets a callback function that is called when a candle closes.
// This enables real-time pattern evaluation in the Entry Decision system.
func (cp *CoinProfiler) SetCandleCloseCallback(callback CandleCloseCallback) {
	cp.candleHistoryMu.Lock()
	defer cp.candleHistoryMu.Unlock()
	cp.onCandleClose = callback
}

// SetPriceUpdateCallback sets a callback function that is called on every price update.
// This enables tick-level breakout detection for immediate entry signals.
func (cp *CoinProfiler) SetPriceUpdateCallback(callback PriceUpdateCallback) {
	cp.candleHistoryMu.Lock()
	defer cp.candleHistoryMu.Unlock()
	cp.onPriceUpdate = callback
}

// GetPriceUpdateCallback returns the current price update callback (for WebSocket manager).
func (cp *CoinProfiler) GetPriceUpdateCallback() PriceUpdateCallback {
	cp.candleHistoryMu.RLock()
	defer cp.candleHistoryMu.RUnlock()
	return cp.onPriceUpdate
}

// SetVolumeProgressCallback sets a callback for real-time volume progress updates.
// This is called on every kline update with calculated volume ratio.
func (cp *CoinProfiler) SetVolumeProgressCallback(callback VolumeProgressCallback) {
	cp.candleHistoryMu.Lock()
	defer cp.candleHistoryMu.Unlock()
	cp.onVolumeProgress = callback
}

// GetVolumeProgressCallback returns the current volume progress callback.
func (cp *CoinProfiler) GetVolumeProgressCallback() VolumeProgressCallback {
	cp.candleHistoryMu.RLock()
	defer cp.candleHistoryMu.RUnlock()
	return cp.onVolumeProgress
}

// SetVolumeProgressConfig configures volume progress calculation parameters.
func (cp *CoinProfiler) SetVolumeProgressConfig(requiredRatio float64, lookbackCandles int) {
	cp.candleHistoryMu.Lock()
	defer cp.candleHistoryMu.Unlock()
	cp.volumeProgressConfig.RequiredRatio = requiredRatio
	cp.volumeProgressConfig.LookbackCandles = lookbackCandles
}

// GetVolumeProgressConfig returns volume progress configuration.
func (cp *CoinProfiler) GetVolumeProgressConfig() (requiredRatio float64, lookbackCandles int) {
	cp.candleHistoryMu.RLock()
	defer cp.candleHistoryMu.RUnlock()
	return cp.volumeProgressConfig.RequiredRatio, cp.volumeProgressConfig.LookbackCandles
}

// CalculateVolumeProgress calculates current volume progress for a symbol/timeframe.
// Returns VolumeProgressData with current ratio vs threshold.
func (cp *CoinProfiler) CalculateVolumeProgress(symbol, timeframe string, currentVolume, open, close float64, closeTime time.Time) *VolumeProgressData {
	cp.candleHistoryMu.RLock()
	requiredRatio := cp.volumeProgressConfig.RequiredRatio
	lookback := cp.volumeProgressConfig.LookbackCandles
	cp.candleHistoryMu.RUnlock()

	// Default values if not configured
	if requiredRatio <= 0 {
		requiredRatio = 3.0
	}
	if lookback <= 0 {
		lookback = 5
	}

	// Get historical candles for average calculation
	history := cp.GetCandleHistory(symbol, timeframe)
	if len(history) < lookback {
		return nil // Not enough data
	}

	// Calculate average volume of last N closed candles
	var totalVolume float64
	for i := len(history) - lookback; i < len(history); i++ {
		totalVolume += history[i].Volume
	}
	avgVolume := totalVolume / float64(lookback)

	if avgVolume <= 0 {
		return nil
	}

	// Calculate current ratio
	currentRatio := currentVolume / avgVolume
	progressPercent := (currentRatio / requiredRatio) * 100
	if progressPercent > 100 {
		progressPercent = 100
	}

	// Determine candle direction
	direction := "bearish"
	if close > open {
		direction = "bullish"
	} else if close == open {
		direction = "neutral"
	}

	// Calculate time remaining until candle close
	timeRemaining := closeTime.Sub(time.Now())
	if timeRemaining < 0 {
		timeRemaining = 0
	}

	return &VolumeProgressData{
		Symbol:             symbol,
		Timeframe:          timeframe,
		CurrentVolume:      currentVolume,
		AverageVolume:      avgVolume,
		CurrentRatio:       currentRatio,
		RequiredRatio:      requiredRatio,
		ProgressPercent:    progressPercent,
		CandleDirection:    direction,
		IsApproachingSpike: currentRatio >= requiredRatio*0.5, // > 50% of threshold
		TimeRemainingMs:    timeRemaining.Milliseconds(),
		LookbackCandles:    lookback,
		CurrentPrice:       close, // Real-time price from candle close
	}
}

// SetHistoricalDataProvider sets the provider for fetching historical candles.
// This should be called before starting the CoinProfiler to enable historical prefetch.
func (cp *CoinProfiler) SetHistoricalDataProvider(provider HistoricalDataProvider) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.historicalProvider = provider
	cp.log("Historical data provider set")
}

// PrefetchHistoricalCandles fetches historical candles from Binance REST API
// and pre-populates the candle history buffer. This enables pattern detection
// to work immediately without waiting for candles to close in real-time.
//
// Call this after subscribing to symbols to bootstrap the candle history.
// Uses sequential processing with delays to avoid rate limits.
func (cp *CoinProfiler) PrefetchHistoricalCandles(symbols []string, timeframes []string) error {
	cp.mu.RLock()
	provider := cp.historicalProvider
	cp.mu.RUnlock()

	if provider == nil {
		cp.log("No historical data provider set, skipping prefetch")
		return nil
	}

	if len(symbols) == 0 || len(timeframes) == 0 {
		return nil
	}

	totalRequests := len(symbols) * len(timeframes)
	cp.log("Prefetching historical candles for %d symbols × %d timeframes (%d total requests, sequential)",
		len(symbols), len(timeframes), totalRequests)

	// Process SEQUENTIALLY with delay to avoid rate limits
	// Binance rate limit: 2400 requests/minute = 40/second
	// We use 50ms delay = ~20 requests/second (still safe, faster startup)
	const requestDelay = 50 * time.Millisecond
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	errorCount := 0
	successCount := 0

	for i, symbol := range symbols {
		for j, timeframe := range timeframes {
			requestNum := i*len(timeframes) + j + 1

			// Fetch with retry logic
			var klines []HistoricalKline
			var err error

			for retry := 0; retry < maxRetries; retry++ {
				klines, err = provider.GetFuturesKlines(symbol, timeframe, DefaultCandleHistorySize)
				if err == nil {
					break
				}

				// Check if it's a rate limit error
				if retry < maxRetries-1 {
					cp.log("Retry %d/%d for %s:%s after error: %v", retry+1, maxRetries, symbol, timeframe, err)
					time.Sleep(retryDelay * time.Duration(retry+1)) // Exponential backoff
				}
			}

			if err != nil {
				cp.logError("Failed to fetch historical candles for %s:%s after %d retries: %v",
					symbol, timeframe, maxRetries, err)
				errorCount++
				// Continue to next symbol instead of failing completely
				time.Sleep(requestDelay)
				continue
			}

			if len(klines) == 0 {
				cp.logDebug("No historical candles returned for %s:%s", symbol, timeframe)
				time.Sleep(requestDelay)
				continue
			}

			// Pre-populate candle history
			cp.candleHistoryMu.Lock()
			key := CandleHistoryKey(symbol, timeframe)
			if cp.candleHistory[key] == nil {
				cp.candleHistory[key] = NewCandleHistory(DefaultCandleHistorySize)
			}

			// Add candles in chronological order (oldest first)
			for _, kline := range klines {
				candle := KlineToHistoricalCandle(kline)
				cp.candleHistory[key].Add(candle)
			}

			// Get all candles for callback
			candlesForCallback := cp.candleHistory[key].GetAll()
			cp.candleHistoryMu.Unlock()

			successCount++
			cp.logDebug("Prefetched %d candles for %s:%s (%d/%d)", len(klines), symbol, timeframe, requestNum, totalRequests)

			// CRITICAL: Trigger candle close callback IMMEDIATELY after prefetch
			// This enables pattern evaluation without waiting for the next candle close
			if cp.onCandleClose != nil && len(candlesForCallback) >= 25 {
				cp.log("Triggering immediate pattern evaluation for %s:%s (prefetch complete)", symbol, timeframe)
				go cp.onCandleClose(symbol, timeframe, candlesForCallback)
			}

			// Delay before next request (except for the last one)
			if requestNum < totalRequests {
				time.Sleep(requestDelay)
			}
		}
	}

	if errorCount > 0 {
		cp.log("Prefetch completed: %d success, %d errors", successCount, errorCount)
	} else {
		cp.log("Prefetch completed successfully: %d symbols", successCount)
	}

	// Note: Pattern evaluation is already triggered during the prefetch loop above
	// (see line "go cp.onCandleClose(symbol, timeframe, candlesForCallback)")
	// No need for additional triggerInitialPatternEvaluation call

	return nil
}

// triggerInitialPatternEvaluation triggers the candle close callback for all
// prefetched symbols to enable immediate pattern detection.
func (cp *CoinProfiler) triggerInitialPatternEvaluation(symbols []string, timeframes []string) {
	cp.candleHistoryMu.RLock()
	callback := cp.onCandleClose
	cp.candleHistoryMu.RUnlock()

	if callback == nil {
		return
	}

	cp.log("Triggering initial pattern evaluation for prefetched candles")

	for _, symbol := range symbols {
		for _, timeframe := range timeframes {
			key := CandleHistoryKey(symbol, timeframe)

			cp.candleHistoryMu.RLock()
			history := cp.candleHistory[key]
			var candles []HistoricalCandle
			if history != nil {
				candles = history.GetAll()
			}
			cp.candleHistoryMu.RUnlock()

			if len(candles) >= 25 { // Minimum for pattern detection
				// Call in goroutine to avoid blocking
				go callback(symbol, timeframe, candles)
			}
		}
	}
}

// GetCandleHistoryStats returns statistics about the candle history storage.
func (cp *CoinProfiler) GetCandleHistoryStats() map[string]int {
	cp.candleHistoryMu.RLock()
	defer cp.candleHistoryMu.RUnlock()

	stats := make(map[string]int)
	for key, history := range cp.candleHistory {
		if history != nil {
			stats[key] = history.Count()
		}
	}
	return stats
}
