// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.5: Regime Transition Handler
package decision

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// TransitionConfig holds the configurable parameters for regime transition handling.
type TransitionConfig struct {
	// ConfirmationCandles is the number of consecutive candles required to confirm a regime change.
	// Default: 3
	ConfirmationCandles int

	// MinRegimeDuration is the minimum time a symbol must spend in a regime before
	// a transition is allowed. This prevents rapid flipping between regimes.
	// Default: 15 minutes
	MinRegimeDuration time.Duration

	// WhipsawWindow is the time window used to detect whipsaw patterns.
	// If multiple transitions occur within this window, they may be considered whipsaws.
	// Default: 30 minutes
	WhipsawWindow time.Duration

	// EnableNotifications enables notification to listeners on confirmed transitions.
	// Default: true
	EnableNotifications bool

	// EnableLogging enables logging of regime transitions for analysis.
	// Default: true
	EnableLogging bool
}

// DefaultTransitionConfig returns a TransitionConfig with sensible defaults.
func DefaultTransitionConfig() *TransitionConfig {
	return &TransitionConfig{
		ConfirmationCandles: 3,
		MinRegimeDuration:   15 * time.Minute,
		WhipsawWindow:       30 * time.Minute,
		EnableNotifications: true,
		EnableLogging:       true,
	}
}

// PendingTransition tracks a regime change that is awaiting confirmation.
type PendingTransition struct {
	// From is the original regime before the change was detected.
	From MarketRegime `json:"from"`

	// To is the new regime that was detected.
	To MarketRegime `json:"to"`

	// FirstDetected is the Unix timestamp (milliseconds) when the change was first detected.
	FirstDetected int64 `json:"first_detected"`

	// Candles is the number of consecutive candles confirming the new regime.
	Candles int `json:"candles"`

	// Confirmed indicates whether the transition has been confirmed.
	Confirmed bool `json:"confirmed"`

	// ConfirmedAt is the Unix timestamp (milliseconds) when the transition was confirmed.
	// Zero if not yet confirmed.
	ConfirmedAt int64 `json:"confirmed_at,omitempty"`
}

// TransitionListener is the interface for receiving regime change notifications.
type TransitionListener interface {
	// OnRegimeChange is called when a regime transition is confirmed.
	// Implementations should be thread-safe and non-blocking.
	OnRegimeChange(symbol string, from, to MarketRegime)
}

// TransitionHandler manages regime transitions with confirmation and whipsaw prevention.
type TransitionHandler struct {
	classifier *RegimeClassifier
	config     *TransitionConfig

	// pendingChanges tracks pending (unconfirmed) regime transitions per symbol.
	pendingChanges map[string]*PendingTransition
	pendingMu      sync.RWMutex

	// regimeStartTimes tracks when each symbol entered its current regime.
	// Used for MinRegimeDuration enforcement.
	regimeStartTimes map[string]int64
	startTimesMu     sync.RWMutex

	// listeners are notified of confirmed regime changes.
	listeners  []TransitionListener
	listenerMu sync.RWMutex

	// transitionLog tracks recent transitions for whipsaw detection and analysis.
	transitionLog   map[string][]TransitionLogEntry
	transitionLogMu sync.RWMutex
}

// TransitionLogEntry records a completed regime transition for analysis.
type TransitionLogEntry struct {
	From        MarketRegime `json:"from"`
	To          MarketRegime `json:"to"`
	DetectedAt  int64        `json:"detected_at"`
	ConfirmedAt int64        `json:"confirmed_at"`
	Candles     int          `json:"candles"`
}

// NewTransitionHandler creates a new TransitionHandler with the given configuration.
// If config is nil, default configuration is used.
// The classifier parameter is required and must not be nil.
func NewTransitionHandler(classifier *RegimeClassifier, config *TransitionConfig) *TransitionHandler {
	if config == nil {
		config = DefaultTransitionConfig()
	}
	// Ensure ConfirmationCandles is at least 1
	if config.ConfirmationCandles < 1 {
		config.ConfirmationCandles = 1
	}
	return &TransitionHandler{
		classifier:       classifier,
		config:           config,
		pendingChanges:   make(map[string]*PendingTransition),
		regimeStartTimes: make(map[string]int64),
		listeners:        make([]TransitionListener, 0),
		transitionLog:    make(map[string][]TransitionLogEntry),
	}
}

// GetConfig returns a copy of the current configuration.
func (th *TransitionHandler) GetConfig() TransitionConfig {
	return *th.config
}

// GetClassifier returns the underlying RegimeClassifier.
func (th *TransitionHandler) GetClassifier() *RegimeClassifier {
	return th.classifier
}

// ProcessCandle processes a new candle and handles regime transition logic.
// Returns a RegimeChange if a transition was confirmed, nil otherwise.
func (th *TransitionHandler) ProcessCandle(ctx context.Context, userID, symbol string, state *CoinState) (*RegimeChange, error) {
	// Check context for cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if state == nil {
		return nil, fmt.Errorf("state cannot be nil")
	}
	if th.classifier == nil {
		return nil, fmt.Errorf("classifier cannot be nil")
	}

	// Get current regime for this symbol
	currentRegime, _ := th.classifier.GetCurrentStreak(symbol)

	// Classify the new state
	newRegime := th.classifier.Classify(state)

	now := time.Now().UnixMilli()

	// Lock for the entire pending transition logic to avoid data races
	th.pendingMu.Lock()
	pending := th.pendingChanges[symbol]

	// If no regime change from current, handle pending transitions
	if newRegime == currentRegime {
		// No change detected - cancel any pending transition
		if pending != nil {
			// Log before deleting (copy values for logging)
			pendingFrom := pending.From
			pendingTo := pending.To
			delete(th.pendingChanges, symbol)
			th.pendingMu.Unlock()

			if th.config.EnableLogging {
				log.Printf("[TransitionHandler] %s: Cancelled pending transition %s -> %s (reverted to %s)",
					symbol, pendingFrom, pendingTo, currentRegime)
			}
			return nil, nil
		}
		th.pendingMu.Unlock()
		return nil, nil
	}

	// A different regime is detected

	// Check minimum regime duration (release lock temporarily for this check)
	th.pendingMu.Unlock()
	if !th.canTransition(symbol, now) {
		if th.config.EnableLogging {
			log.Printf("[TransitionHandler] %s: Transition blocked - minimum regime duration not met", symbol)
		}
		return nil, nil
	}
	th.pendingMu.Lock()
	// Re-fetch pending after re-acquiring lock
	pending = th.pendingChanges[symbol]

	// If we have a pending transition to a different regime than what's now detected,
	// cancel it and start a new one
	if pending != nil && pending.To != newRegime {
		pendingFrom := pending.From
		pendingTo := pending.To
		delete(th.pendingChanges, symbol)
		pending = nil

		if th.config.EnableLogging {
			// Unlock before logging to avoid holding lock during I/O
			th.pendingMu.Unlock()
			log.Printf("[TransitionHandler] %s: Cancelled pending transition %s -> %s (new direction: %s)",
				symbol, pendingFrom, pendingTo, newRegime)
			th.pendingMu.Lock()
		}
	}

	// If no pending transition exists, create one
	if pending == nil {
		pending = &PendingTransition{
			From:          currentRegime,
			To:            newRegime,
			FirstDetected: now,
			Candles:       1,
			Confirmed:     false,
		}
		th.pendingChanges[symbol] = pending

		if th.config.EnableLogging {
			log.Printf("[TransitionHandler] %s: New pending transition %s -> %s (1/%d candles)",
				symbol, pending.From, pending.To, th.config.ConfirmationCandles)
		}

		// Check if single-candle confirmation is configured
		if th.config.ConfirmationCandles <= 1 {
			// Make a copy for confirmTransition to avoid holding lock
			pendingCopy := *pending
			th.pendingMu.Unlock()
			return th.confirmTransition(ctx, userID, symbol, state, &pendingCopy)
		}
		th.pendingMu.Unlock()
		return nil, nil
	}

	// Increment confirmation candle count (now safe - under lock)
	pending.Candles++
	candleCount := pending.Candles

	if th.config.EnableLogging {
		log.Printf("[TransitionHandler] %s: Pending transition %s -> %s (%d/%d candles)",
			symbol, pending.From, pending.To, candleCount, th.config.ConfirmationCandles)
	}

	// Check if confirmed
	if candleCount >= th.config.ConfirmationCandles {
		// Make a copy for confirmTransition
		pendingCopy := *pending
		th.pendingMu.Unlock()
		return th.confirmTransition(ctx, userID, symbol, state, &pendingCopy)
	}

	th.pendingMu.Unlock()
	return nil, nil
}

// confirmTransition confirms a pending transition and notifies listeners.
// Note: This function receives a copy of pending, so modifications are safe.
// The caller is responsible for unlocking pendingMu before calling this.
func (th *TransitionHandler) confirmTransition(ctx context.Context, userID, symbol string, state *CoinState, pending *PendingTransition) (*RegimeChange, error) {
	now := time.Now().UnixMilli()
	pending.Confirmed = true
	pending.ConfirmedAt = now

	// Update the classifier's record
	newRegime, _ := th.classifier.ClassifyAndRecord(symbol, state.ADX, state.ATR, state.ATRAverage)

	// Update state
	state.Regime = newRegime
	state.ActiveStrategy = GetStrategyForRegime(newRegime)

	// Update regime start time
	th.startTimesMu.Lock()
	th.regimeStartTimes[symbol] = now
	th.startTimesMu.Unlock()

	// Create the change record
	change := &RegimeChange{
		From:      pending.From,
		To:        pending.To,
		Timestamp: now,
		Reason:    fmt.Sprintf("Confirmed after %d candles", pending.Candles),
		Indicators: RegimeIndicators{
			ADX:        state.ADX,
			ATR:        state.ATR,
			ATRAverage: state.ATRAverage,
		},
	}

	// Log the transition
	if th.config.EnableLogging {
		log.Printf("[TransitionHandler] %s: CONFIRMED transition %s -> %s after %d candles",
			symbol, pending.From, pending.To, pending.Candles)
	}

	// Add to transition log
	th.addToTransitionLog(symbol, pending)

	// Clean up pending from map
	th.pendingMu.Lock()
	delete(th.pendingChanges, symbol)
	th.pendingMu.Unlock()

	// Notify listeners
	if th.config.EnableNotifications {
		th.notifyListeners(symbol, pending.From, pending.To)
	}

	return change, nil
}

// canTransition checks if a transition is allowed based on MinRegimeDuration.
func (th *TransitionHandler) canTransition(symbol string, now int64) bool {
	th.startTimesMu.RLock()
	startTime, exists := th.regimeStartTimes[symbol]
	th.startTimesMu.RUnlock()

	if !exists {
		// No recorded start time - allow transition
		return true
	}

	elapsed := time.Duration(now-startTime) * time.Millisecond
	return elapsed >= th.config.MinRegimeDuration
}

// maxTransitionLogEntries is the maximum number of entries per symbol in the transition log.
// This prevents unbounded memory growth.
const maxTransitionLogEntries = 100

// addToTransitionLog adds a transition entry to the log for analysis.
func (th *TransitionHandler) addToTransitionLog(symbol string, pending *PendingTransition) {
	entry := TransitionLogEntry{
		From:        pending.From,
		To:          pending.To,
		DetectedAt:  pending.FirstDetected,
		ConfirmedAt: pending.ConfirmedAt,
		Candles:     pending.Candles,
	}

	th.transitionLogMu.Lock()
	defer th.transitionLogMu.Unlock()

	entries := th.transitionLog[symbol]
	entries = append(entries, entry)

	// Keep only entries within WhipsawWindow for analysis
	cutoff := time.Now().Add(-th.config.WhipsawWindow).UnixMilli()
	filtered := make([]TransitionLogEntry, 0, len(entries))
	for _, e := range entries {
		if e.ConfirmedAt >= cutoff {
			filtered = append(filtered, e)
		}
	}

	// Also enforce maximum entries limit to prevent unbounded growth
	if len(filtered) > maxTransitionLogEntries {
		filtered = filtered[len(filtered)-maxTransitionLogEntries:]
	}

	th.transitionLog[symbol] = filtered
}

// GetPendingTransitions returns a copy of all pending transitions.
func (th *TransitionHandler) GetPendingTransitions() map[string]*PendingTransition {
	th.pendingMu.RLock()
	defer th.pendingMu.RUnlock()

	result := make(map[string]*PendingTransition, len(th.pendingChanges))
	for symbol, pending := range th.pendingChanges {
		// Return a copy
		p := *pending
		result[symbol] = &p
	}
	return result
}

// IsPendingTransition returns true if there's a pending transition for the symbol.
func (th *TransitionHandler) IsPendingTransition(symbol string) bool {
	th.pendingMu.RLock()
	defer th.pendingMu.RUnlock()
	_, exists := th.pendingChanges[symbol]
	return exists
}

// GetPendingTransition returns the pending transition for a symbol, or nil if none.
func (th *TransitionHandler) GetPendingTransition(symbol string) *PendingTransition {
	th.pendingMu.RLock()
	defer th.pendingMu.RUnlock()

	pending, exists := th.pendingChanges[symbol]
	if !exists {
		return nil
	}

	// Return a copy
	p := *pending
	return &p
}

// CancelPendingTransition cancels any pending transition for the symbol.
func (th *TransitionHandler) CancelPendingTransition(symbol string) {
	th.pendingMu.Lock()
	defer th.pendingMu.Unlock()
	delete(th.pendingChanges, symbol)
}

// CancelAllPendingTransitions cancels all pending transitions.
func (th *TransitionHandler) CancelAllPendingTransitions() {
	th.pendingMu.Lock()
	defer th.pendingMu.Unlock()
	th.pendingChanges = make(map[string]*PendingTransition)
}

// AddListener adds a transition listener.
// Listeners are notified when regime transitions are confirmed.
func (th *TransitionHandler) AddListener(listener TransitionListener) {
	if listener == nil {
		return
	}
	th.listenerMu.Lock()
	defer th.listenerMu.Unlock()
	th.listeners = append(th.listeners, listener)
}

// RemoveListener removes a transition listener.
func (th *TransitionHandler) RemoveListener(listener TransitionListener) {
	if listener == nil {
		return
	}
	th.listenerMu.Lock()
	defer th.listenerMu.Unlock()

	for i, l := range th.listeners {
		if l == listener {
			th.listeners = append(th.listeners[:i], th.listeners[i+1:]...)
			return
		}
	}
}

// notifyListeners notifies all registered listeners of a regime change.
// Listeners are notified in goroutines to avoid blocking.
func (th *TransitionHandler) notifyListeners(symbol string, from, to MarketRegime) {
	th.listenerMu.RLock()
	listeners := make([]TransitionListener, len(th.listeners))
	copy(listeners, th.listeners)
	th.listenerMu.RUnlock()

	for _, listener := range listeners {
		// Notify asynchronously to avoid blocking
		go func(l TransitionListener) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[TransitionHandler] Listener panic recovered: %v", r)
				}
			}()
			l.OnRegimeChange(symbol, from, to)
		}(listener)
	}
}

// GetTransitionLog returns the transition log entries for a symbol.
// Returns entries within the WhipsawWindow.
func (th *TransitionHandler) GetTransitionLog(symbol string) []TransitionLogEntry {
	th.transitionLogMu.RLock()
	defer th.transitionLogMu.RUnlock()

	log := th.transitionLog[symbol]
	if log == nil {
		return []TransitionLogEntry{}
	}

	// Return a copy
	result := make([]TransitionLogEntry, len(log))
	copy(result, log)
	return result
}

// GetRecentTransitionCount returns the number of transitions within the WhipsawWindow.
// This can be used to detect whipsaw patterns.
func (th *TransitionHandler) GetRecentTransitionCount(symbol string) int {
	th.transitionLogMu.RLock()
	defer th.transitionLogMu.RUnlock()

	return len(th.transitionLog[symbol])
}

// IsWhipsawDetected returns true if excessive transitions have occurred within the WhipsawWindow.
// The threshold parameter specifies the maximum allowed transitions before whipsaw is detected.
func (th *TransitionHandler) IsWhipsawDetected(symbol string, threshold int) bool {
	return th.GetRecentTransitionCount(symbol) >= threshold
}

// SetRegimeStartTime sets the start time for a symbol's current regime.
// This is useful for initializing the handler with existing regime data.
func (th *TransitionHandler) SetRegimeStartTime(symbol string, timestamp int64) {
	th.startTimesMu.Lock()
	defer th.startTimesMu.Unlock()
	th.regimeStartTimes[symbol] = timestamp
}

// GetRegimeStartTime returns the start time for a symbol's current regime.
// Returns 0 if no start time is recorded.
func (th *TransitionHandler) GetRegimeStartTime(symbol string) int64 {
	th.startTimesMu.RLock()
	defer th.startTimesMu.RUnlock()
	return th.regimeStartTimes[symbol]
}

// ClearSymbol removes all data for a symbol (pending, start time, log).
func (th *TransitionHandler) ClearSymbol(symbol string) {
	th.pendingMu.Lock()
	delete(th.pendingChanges, symbol)
	th.pendingMu.Unlock()

	th.startTimesMu.Lock()
	delete(th.regimeStartTimes, symbol)
	th.startTimesMu.Unlock()

	th.transitionLogMu.Lock()
	delete(th.transitionLog, symbol)
	th.transitionLogMu.Unlock()
}

// ClearAll removes all data for all symbols.
func (th *TransitionHandler) ClearAll() {
	th.pendingMu.Lock()
	th.pendingChanges = make(map[string]*PendingTransition)
	th.pendingMu.Unlock()

	th.startTimesMu.Lock()
	th.regimeStartTimes = make(map[string]int64)
	th.startTimesMu.Unlock()

	th.transitionLogMu.Lock()
	th.transitionLog = make(map[string][]TransitionLogEntry)
	th.transitionLogMu.Unlock()
}
