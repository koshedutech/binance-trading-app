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
	logger    *logging.Logger
	wsManager *WebSocketManager // WebSocket manager for real-time data

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

		// Update source to "both" if needed
		if existing.Source != req.Source {
			existing.Source = DataSourceBoth
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
	cp.mu.Lock()
	cp.combinedReqs = combined // Store for API access
	for symbol, symReq := range combined.BySymbol {
		cp.subscriptions[symbol] = &SubscriptionRequest{
			Symbol:     symbol,
			Timeframes: symReq.Timeframes,
			Source:     symReq.Source,
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

	return nil
}
