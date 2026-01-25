// Package exitdecision provides exit signal monitoring for the Chain Trading System.
package exitdecision

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ============================================================================
// EXIT DECISION SERVICE
// ============================================================================
// The Exit Decision Service monitors open positions from the Coin Profiler and
// generates exit signals when TP/SL/trailing stop conditions are met.
// It continues running even when Trading is OFF to ensure positions can be closed.

// Service implements the ExitDecisionService interface.
type Service struct {
	// Configuration
	config *Config

	// Dependencies
	monitor          *ExitMonitor
	priceProvider    PriceProvider
	positionProvider PositionProvider

	// State
	running    bool
	startedAt  time.Time
	lastError  string
	lastCheck  time.Time

	// Signal tracking
	pendingSignals map[string][]ExitSignal // userID -> signals
	signalsTotal   int64

	// Monitored positions
	monitored map[string]*monitoredPosition // "userID:symbol" -> position state

	// Context for graceful shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc

	// Control channels
	stopChan chan struct{}

	// Synchronization
	mu sync.RWMutex
	wg sync.WaitGroup
}

// NewService creates a new Exit Decision service.
func NewService(config *Config, priceProvider PriceProvider, positionProvider PositionProvider) *Service {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Service{
		config:           config,
		monitor:          NewExitMonitor(config),
		priceProvider:    priceProvider,
		positionProvider: positionProvider,
		pendingSignals:   make(map[string][]ExitSignal),
		monitored:        make(map[string]*monitoredPosition),
		ctx:              ctx,
		cancelFunc:       cancel,
		stopChan:         make(chan struct{}),
	}
}

// Start starts the exit decision monitoring service.
// This service continues running even when Trading is OFF.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("%s already running", LogPrefix)
	}

	s.running = true
	s.startedAt = time.Now()
	s.lastError = ""

	// Reset context for new run
	s.ctx, s.cancelFunc = context.WithCancel(ctx)
	s.stopChan = make(chan struct{})

	s.mu.Unlock()

	s.log("Starting Exit Decision service (monitors positions even when Trading OFF)")

	// Start the main monitoring loop
	s.wg.Add(1)
	go s.monitoringLoop()

	s.log("Exit Decision service started successfully")
	return nil
}

// Stop stops the exit decision monitoring service gracefully.
func (s *Service) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("%s not running", LogPrefix)
	}

	s.running = false
	stopChan := s.stopChan
	s.mu.Unlock()

	s.log("Stopping Exit Decision service")

	// Signal all goroutines to stop
	s.cancelFunc()

	// Safely close stopChan
	select {
	case <-stopChan:
		// Already closed
	default:
		close(stopChan)
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.log("Exit Decision service stopped gracefully")
	case <-time.After(10 * time.Second):
		s.log("Exit Decision service stop timed out after 10 seconds")
	}

	return nil
}

// IsRunning returns whether the service is currently running.
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetExitSignals returns all pending exit signals for a user.
// After retrieval, immediate signals are cleared (they should be acted upon).
func (s *Service) GetExitSignals(ctx context.Context, userID string) ([]ExitSignal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	signals, exists := s.pendingSignals[userID]
	if !exists {
		return []ExitSignal{}, nil
	}

	// Return a copy and filter out monitor/warning signals
	// Only immediate signals should be "consumed"
	result := make([]ExitSignal, 0, len(signals))
	remaining := make([]ExitSignal, 0)

	for _, sig := range signals {
		result = append(result, sig)
		// Keep non-immediate signals for continued monitoring
		if sig.Urgency != UrgencyImmediate {
			remaining = append(remaining, sig)
		}
	}

	// Clear immediate signals, keep warnings
	s.pendingSignals[userID] = remaining

	return result, nil
}

// CheckPosition checks a single position for exit conditions.
// This can be called directly for on-demand checking.
func (s *Service) CheckPosition(ctx context.Context, position Position) (*ExitSignal, error) {
	if position == nil {
		return nil, fmt.Errorf("position is nil")
	}

	symbol := position.GetSymbol()
	if symbol == "" {
		return nil, fmt.Errorf("position symbol is empty")
	}

	// Get current price
	if s.priceProvider == nil {
		return nil, fmt.Errorf("price provider not configured")
	}

	price, err := s.priceProvider.GetPrice(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get price for %s: %w", symbol, err)
	}

	// Check position with the monitor
	return s.monitor.CheckPosition(ctx, position, price, "")
}

// GetStatus returns the current service status.
func (s *Service) GetStatus() *ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &ServiceStatus{
		Running:             s.running,
		StartedAt:           s.startedAt,
		PositionsMonitored:  len(s.monitored),
		SignalsGenerated:    s.signalsTotal,
		LastCheckTime:       s.lastCheck,
		CheckIntervalMs:     s.config.CheckInterval.Milliseconds(),
		LastError:           s.lastError,
		PriceProviderHealthy: s.priceProvider != nil,
	}

	// Calculate uptime
	if s.running && !s.startedAt.IsZero() {
		uptime := time.Since(s.startedAt)
		status.Uptime = formatUptime(uptime)
	}

	return status
}

// ============================================================================
// MONITORING LOOP
// ============================================================================

// monitoringLoop is the main loop that continuously monitors positions.
func (s *Service) monitoringLoop() {
	defer s.wg.Done()

	s.log("Monitoring loop started (interval: %v)", s.config.CheckInterval)

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.log("Monitoring loop stopped (context cancelled)")
			return
		case <-s.stopChan:
			s.log("Monitoring loop stopped (stop signal)")
			return
		case <-ticker.C:
			s.runMonitoringCycle()
		}
	}
}

// runMonitoringCycle runs a single monitoring cycle for all positions.
func (s *Service) runMonitoringCycle() {
	s.mu.Lock()
	s.lastCheck = time.Now()
	s.mu.Unlock()

	// Skip if no providers configured
	if s.positionProvider == nil {
		s.logDebug("Skipping cycle: no position provider")
		return
	}

	if s.priceProvider == nil {
		s.logDebug("Skipping cycle: no price provider")
		return
	}

	// Get all monitored users (for now, we'll use a placeholder)
	// In production, this would come from a user registry or the position provider
	// For MVP, we monitor all positions returned by the provider
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	// Get positions for all users
	// Note: In multi-user mode, we'd iterate through users
	// For now, we'll check positions with empty userID (global check)
	positions, err := s.positionProvider.GetOpenPositions(ctx, "")
	if err != nil {
		s.setError("Failed to get positions: %v", err)
		return
	}

	if len(positions) == 0 {
		s.logDebug("No positions to monitor")
		return
	}

	// Collect symbols for batch price fetch
	symbols := make([]string, 0, len(positions))
	symbolSet := make(map[string]bool)
	for _, pos := range positions {
		symbol := pos.GetSymbol()
		if !symbolSet[symbol] {
			symbols = append(symbols, symbol)
			symbolSet[symbol] = true
		}
	}

	// Get prices for all symbols
	prices, err := s.priceProvider.GetPrices(ctx, symbols)
	if err != nil {
		s.setError("Failed to get prices: %v", err)
		return
	}

	// Check each position
	for _, pos := range positions {
		symbol := pos.GetSymbol()
		price, exists := prices[symbol]
		if !exists || price <= 0 {
			s.logDebug("No price available for %s", symbol)
			continue
		}

		// Check for exit signals
		signal, err := s.monitor.CheckPosition(ctx, pos, price, "")
		if err != nil {
			s.logDebug("Error checking position %s: %v", symbol, err)
			continue
		}

		if signal != nil {
			s.handleSignal(signal)
		}

		// Update monitored position state
		s.updateMonitored(pos, price)
	}
}

// handleSignal processes a generated exit signal.
func (s *Service) handleSignal(signal *ExitSignal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.signalsTotal++

	// Store signal for retrieval
	userID := signal.UserID
	if userID == "" {
		userID = "default"
	}

	// Check for duplicate signals
	existingSignals := s.pendingSignals[userID]
	for i, existing := range existingSignals {
		if existing.Symbol == signal.Symbol && existing.ExitType == signal.ExitType {
			// Update existing signal if urgency increased
			if signal.Urgency == UrgencyImmediate && existing.Urgency != UrgencyImmediate {
				s.pendingSignals[userID][i] = *signal
				s.logSignal(signal, "upgraded")
			}
			return
		}
	}

	// Add new signal
	s.pendingSignals[userID] = append(s.pendingSignals[userID], *signal)
	s.logSignal(signal, "generated")
}

// updateMonitored updates the monitored position state.
func (s *Service) updateMonitored(pos Position, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf(":%s", pos.GetSymbol()) // userID:symbol

	state, exists := s.monitored[key]
	if !exists {
		state = &monitoredPosition{
			Symbol:       pos.GetSymbol(),
			HighestPrice: price,
			LowestPrice:  price,
		}
		s.monitored[key] = state
	}

	state.LastCheck = time.Now()
	state.LastPrice = price
	state.CheckCount++

	// Track high/low for trailing stop calculations
	if price > state.HighestPrice {
		state.HighestPrice = price
	}
	if price < state.LowestPrice || state.LowestPrice == 0 {
		state.LowestPrice = price
	}
}

// ============================================================================
// CONFIGURATION METHODS
// ============================================================================

// SetPriceProvider sets the price provider.
func (s *Service) SetPriceProvider(provider PriceProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.priceProvider = provider
}

// SetPositionProvider sets the position provider.
func (s *Service) SetPositionProvider(provider PositionProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.positionProvider = provider
}

// SetConfig updates the configuration.
func (s *Service) SetConfig(config *Config) {
	if config == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
	s.monitor = NewExitMonitor(config)
}

// ============================================================================
// LOGGING
// ============================================================================

// log logs a message with the standard prefix.
func (s *Service) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s %s", LogPrefix, msg)
}

// logDebug logs a debug message (only if debug mode is enabled).
func (s *Service) logDebug(format string, args ...interface{}) {
	s.mu.RLock()
	debug := s.config != nil && s.config.DebugMode
	s.mu.RUnlock()

	if debug {
		msg := fmt.Sprintf(format, args...)
		log.Printf("%s [DEBUG] %s", LogPrefix, msg)
	}
}

// logSignal logs an exit signal.
func (s *Service) logSignal(signal *ExitSignal, action string) {
	log.Printf("%s [SIGNAL-%s] %s %s: %s (price=%.4f, pnl=%.2f%%, urgency=%s)",
		LogPrefix, action, signal.Symbol, signal.ExitType,
		signal.Reason, signal.CurrentPrice, signal.UnrealizedPnLPercent, signal.Urgency)
}

// setError sets the last error.
func (s *Service) setError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	s.mu.Lock()
	s.lastError = msg
	s.mu.Unlock()
	log.Printf("%s [ERROR] %s", LogPrefix, msg)
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

// ============================================================================
// SIGNAL SUBSCRIPTION (for future use)
// ============================================================================

// SignalHandler is a function that handles exit signals.
type SignalHandler func(signal ExitSignal)

// signalSubscribers holds subscribers for exit signals.
var signalSubscribers []SignalHandler
var subscriberMu sync.RWMutex

// SubscribeToSignals registers a handler to receive exit signals.
func (s *Service) SubscribeToSignals(handler SignalHandler) {
	subscriberMu.Lock()
	defer subscriberMu.Unlock()
	signalSubscribers = append(signalSubscribers, handler)
}

// notifySubscribers notifies all subscribers of a signal.
func notifySubscribers(signal ExitSignal) {
	subscriberMu.RLock()
	defer subscriberMu.RUnlock()
	for _, handler := range signalSubscribers {
		go handler(signal)
	}
}
