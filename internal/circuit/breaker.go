package circuit

import (
	"fmt"
	"math"
	"sync"
	"time"

	"binance-trading-bot/internal/events"
)

// BreakerState represents the circuit breaker state
type BreakerState string

const (
	StateClosed   BreakerState = "closed"    // Normal operation
	StateOpen     BreakerState = "open"      // Trading halted
	StateHalfOpen BreakerState = "half_open" // Testing recovery
)

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled              bool    `json:"enabled"`
	MaxLossPerHour       float64 `json:"max_loss_per_hour"`       // Max loss % per hour
	MaxConsecutiveLosses int     `json:"max_consecutive_losses"`  // Max losing trades in a row
	CooldownMinutes      int     `json:"cooldown_minutes"`        // Cooldown after trip
	MaxTradesPerMinute   int     `json:"max_trades_per_minute"`   // Rate limit
	MaxDailyLoss         float64 `json:"max_daily_loss"`          // Max daily loss %
	MaxDailyTrades       int     `json:"max_daily_trades"`        // Max trades per day
}

// DefaultCircuitBreakerConfig returns safe defaults
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Enabled:              true,
		MaxLossPerHour:       3.0,  // 3% max loss per hour
		MaxConsecutiveLosses: 5,    // 5 consecutive losses
		CooldownMinutes:      30,   // 30 minute cooldown
		MaxTradesPerMinute:   10,   // 10 trades per minute max
		MaxDailyLoss:         5.0,  // 5% max daily loss
		MaxDailyTrades:       100,  // 100 trades per day max
	}
}

// CircuitBreaker implements trading circuit breaker pattern
type CircuitBreaker struct {
	config            *CircuitBreakerConfig
	state             BreakerState
	consecutiveLosses int
	hourlyLoss        float64
	dailyLoss         float64
	tradesLastMinute  int
	dailyTrades       int
	lastTripTime      time.Time
	lastTradeTime     time.Time
	hourlyResetTime   time.Time
	dailyResetTime    time.Time
	minuteResetTime   time.Time
	tripReason        string
	mu                sync.RWMutex
	onTrip            func(reason string)
	onReset           func()
	userID            string // UserID for WebSocket broadcasts
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	now := time.Now()
	return &CircuitBreaker{
		config:          config,
		state:           StateClosed,
		hourlyResetTime: now.Add(time.Hour),
		dailyResetTime:  now.Truncate(24 * time.Hour).Add(24 * time.Hour),
		minuteResetTime: now.Add(time.Minute),
	}
}

// OnTrip sets callback for when breaker trips
func (cb *CircuitBreaker) OnTrip(handler func(reason string)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onTrip = handler
}

// OnReset sets callback for when breaker resets
func (cb *CircuitBreaker) OnReset(handler func()) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onReset = handler
}

// CanTrade checks if trading is allowed
func (cb *CircuitBreaker) CanTrade() (bool, string) {
	if !cb.config.Enabled {
		return true, ""
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.resetCountersIfNeeded()

	// Check if breaker is open
	if cb.state == StateOpen {
		elapsed := time.Since(cb.lastTripTime)
		cooldown := time.Duration(cb.config.CooldownMinutes) * time.Minute

		if elapsed < cooldown {
			remaining := cooldown - elapsed
			return false, fmt.Sprintf("circuit breaker open, cooldown remaining: %v (reason: %s)",
				remaining.Round(time.Second), cb.tripReason)
		}

		// Cooldown passed, try half-open
		cb.state = StateHalfOpen
	}

	// Check hourly loss limit
	if cb.hourlyLoss >= cb.config.MaxLossPerHour {
		return false, fmt.Sprintf("hourly loss limit reached: %.2f%% >= %.2f%%",
			cb.hourlyLoss, cb.config.MaxLossPerHour)
	}

	// Check daily loss limit
	if cb.dailyLoss >= cb.config.MaxDailyLoss {
		return false, fmt.Sprintf("daily loss limit reached: %.2f%% >= %.2f%%",
			cb.dailyLoss, cb.config.MaxDailyLoss)
	}

	// Check consecutive losses
	if cb.consecutiveLosses >= cb.config.MaxConsecutiveLosses {
		return false, fmt.Sprintf("max consecutive losses reached: %d",
			cb.consecutiveLosses)
	}

	// Check trades per minute rate limit
	if cb.tradesLastMinute >= cb.config.MaxTradesPerMinute {
		return false, fmt.Sprintf("rate limit reached: %d trades/minute",
			cb.tradesLastMinute)
	}

	// Check daily trade limit
	if cb.dailyTrades >= cb.config.MaxDailyTrades {
		return false, fmt.Sprintf("daily trade limit reached: %d trades",
			cb.dailyTrades)
	}

	return true, ""
}

// RecordTrade records a trade result
func (cb *CircuitBreaker) RecordTrade(pnlPercent float64) {
	if !cb.config.Enabled {
		return
	}

	cb.mu.Lock()

	// Validate PnL value to prevent NaN/Inf from breaking the circuit breaker
	if math.IsNaN(pnlPercent) || math.IsInf(pnlPercent, 0) {
		// Log warning but don't process invalid values
		cb.mu.Unlock()
		return
	}

	cb.resetCountersIfNeeded()

	cb.lastTradeTime = time.Now()
	cb.tradesLastMinute++
	cb.dailyTrades++

	var recoveredFromHalfOpen bool
	if pnlPercent < 0 {
		// Losing trade
		cb.consecutiveLosses++
		cb.hourlyLoss += -pnlPercent
		cb.dailyLoss += -pnlPercent
	} else {
		// Winning trade - reset consecutive losses
		cb.consecutiveLosses = 0

		// If in half-open state and we had a winner, close the breaker
		if cb.state == StateHalfOpen {
			cb.state = StateClosed
			recoveredFromHalfOpen = true
			if cb.onReset != nil {
				go cb.onReset()
			}
		}
	}

	userID := cb.userID
	cb.mu.Unlock()

	// Broadcast circuit breaker recovery to WebSocket clients
	if recoveredFromHalfOpen && userID != "" {
		events.BroadcastCircuitBreaker(userID, map[string]interface{}{
			"state":  string(StateClosed),
			"action": "recovered",
			"reason": "winning_trade_after_cooldown",
		})
	}

	// Check if we need to trip the breaker
	cb.mu.Lock()
	cb.checkAndTrip()
	cb.mu.Unlock()
}

// checkAndTrip checks conditions and trips if needed
func (cb *CircuitBreaker) checkAndTrip() {
	var reason string

	if cb.consecutiveLosses >= cb.config.MaxConsecutiveLosses {
		reason = fmt.Sprintf("consecutive losses: %d", cb.consecutiveLosses)
	} else if cb.hourlyLoss >= cb.config.MaxLossPerHour {
		reason = fmt.Sprintf("hourly loss: %.2f%%", cb.hourlyLoss)
	} else if cb.dailyLoss >= cb.config.MaxDailyLoss {
		reason = fmt.Sprintf("daily loss: %.2f%%", cb.dailyLoss)
	}

	if reason != "" {
		cb.trip(reason)
	}
}

// trip opens the circuit breaker
func (cb *CircuitBreaker) trip(reason string) {
	cb.state = StateOpen
	cb.lastTripTime = time.Now()
	cb.tripReason = reason

	if cb.onTrip != nil {
		go cb.onTrip(reason)
	}

	// Broadcast circuit breaker trip to WebSocket clients
	if cb.userID != "" {
		events.BroadcastCircuitBreaker(cb.userID, map[string]interface{}{
			"state":             string(StateOpen),
			"action":            "tripped",
			"reason":            reason,
			"consecutiveLosses": cb.consecutiveLosses,
			"hourlyLoss":        cb.hourlyLoss,
			"dailyLoss":         cb.dailyLoss,
			"lastTripTime":      cb.lastTripTime,
		})
	}
}

// resetCountersIfNeeded resets time-based counters
func (cb *CircuitBreaker) resetCountersIfNeeded() {
	now := time.Now()

	// Reset minute counter
	if now.After(cb.minuteResetTime) {
		cb.tradesLastMinute = 0
		cb.minuteResetTime = now.Add(time.Minute)
	}

	// Reset hourly counter
	if now.After(cb.hourlyResetTime) {
		cb.hourlyLoss = 0
		cb.hourlyResetTime = now.Add(time.Hour)
	}

	// Reset daily counters
	if now.After(cb.dailyResetTime) {
		cb.dailyLoss = 0
		cb.dailyTrades = 0
		cb.dailyResetTime = now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
}

// ForceReset manually resets the circuit breaker
func (cb *CircuitBreaker) ForceReset() {
	cb.mu.Lock()
	cb.state = StateClosed
	cb.consecutiveLosses = 0
	cb.tripReason = ""
	userID := cb.userID
	cb.mu.Unlock()

	if cb.onReset != nil {
		go cb.onReset()
	}

	// Broadcast circuit breaker reset to WebSocket clients
	if userID != "" {
		events.BroadcastCircuitBreaker(userID, map[string]interface{}{
			"state":             string(StateClosed),
			"action":            "reset",
			"reason":            "manual_reset",
			"consecutiveLosses": 0,
		})
	}
}

// GetState returns current breaker state
func (cb *CircuitBreaker) GetState() BreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns current statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":              string(cb.state),
		"consecutive_losses": cb.consecutiveLosses,
		"hourly_loss":        cb.hourlyLoss,
		"daily_loss":         cb.dailyLoss,
		"trades_last_minute": cb.tradesLastMinute,
		"daily_trades":       cb.dailyTrades,
		"trip_reason":        cb.tripReason,
		"last_trip_time":     cb.lastTripTime,
	}
}

// IsEnabled returns if circuit breaker is enabled
func (cb *CircuitBreaker) IsEnabled() bool {
	return cb.config.Enabled
}

// GetConfig returns a copy of the current configuration
func (cb *CircuitBreaker) GetConfig() CircuitBreakerConfig {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return *cb.config
}

// UpdateConfig updates the circuit breaker configuration
func (cb *CircuitBreaker) UpdateConfig(updates *CircuitBreakerConfig) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if updates.MaxLossPerHour > 0 {
		cb.config.MaxLossPerHour = updates.MaxLossPerHour
	}
	if updates.MaxDailyLoss > 0 {
		cb.config.MaxDailyLoss = updates.MaxDailyLoss
	}
	if updates.MaxConsecutiveLosses > 0 {
		cb.config.MaxConsecutiveLosses = updates.MaxConsecutiveLosses
	}
	if updates.CooldownMinutes > 0 {
		cb.config.CooldownMinutes = updates.CooldownMinutes
	}
	if updates.MaxTradesPerMinute > 0 {
		cb.config.MaxTradesPerMinute = updates.MaxTradesPerMinute
	}
	if updates.MaxDailyTrades > 0 {
		cb.config.MaxDailyTrades = updates.MaxDailyTrades
	}
}

// SetEnabled enables or disables the circuit breaker
func (cb *CircuitBreaker) SetEnabled(enabled bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.config.Enabled = enabled
}

// SetUserID sets the user ID for WebSocket broadcasts
func (cb *CircuitBreaker) SetUserID(userID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.userID = userID
}
