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
	logger *logging.Logger

	// State
	running         bool
	connectionState ConnectionState
	startedAt       time.Time
	reconnectCount  int
	lastError       string
	lastUpdateTime  time.Time

	// Subscription management
	subscriptions map[string]*SubscriptionRequest // symbol -> request
	coinData      map[string]*CoinData            // symbol -> data

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

	// Start the main processing loop
	cp.wg.Add(1)
	go cp.runLoop()

	// Start metrics collection
	cp.wg.Add(1)
	go cp.metricsLoop()

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

	status := &CoinProfilerStatus{
		Running:           cp.running,
		Connected:         cp.connectionState == ConnectionStateConnected,
		SymbolCount:       len(cp.coinData),
		SubscriptionCount: len(cp.subscriptions),
		UpdatesPerSecond:  cp.updatesPerSecond,
		LastUpdateTime:    cp.lastUpdateTime,
		LastError:         cp.lastError,
		ReconnectCount:    cp.reconnectCount,
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
