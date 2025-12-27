package binance

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ==================== PRIORITY TYPES ====================

// RequestPriority defines priority levels for API requests
// Higher priority requests get more lenient rate limiting thresholds
type RequestPriority int

const (
	// PriorityCritical - Orders, cancellations, position closures
	// Uses up to 95% of weight budget - these MUST go through
	PriorityCritical RequestPriority = iota

	// PriorityHigh - Position checks, SL/TP management, account info
	// Uses up to 80% of weight budget
	PriorityHigh

	// PriorityNormal - Market data, klines for active trading
	// Uses up to 60% of weight budget
	PriorityNormal

	// PriorityLow - Background scans, analytics, non-urgent data
	// Uses up to 40% of weight budget - throttled first
	PriorityLow
)

// String returns a human-readable priority name
func (p RequestPriority) String() string {
	switch p {
	case PriorityCritical:
		return "CRITICAL"
	case PriorityHigh:
		return "HIGH"
	case PriorityNormal:
		return "NORMAL"
	case PriorityLow:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

// AcquireResult represents the result of a non-blocking TryAcquire attempt
type AcquireResult struct {
	Acquired     bool          // Whether the slot was successfully acquired
	WaitTime     time.Duration // Suggested wait time if not acquired
	Reason       string        // Explanation for denial (empty if acquired)
	WeightBudget int           // Remaining weight budget after this request
	CurrentUsage float64       // Current weight usage percentage (0-100)
}

// ==================== RATE LIMITER ====================

// RateLimiter implements proactive rate limiting with circuit breaker
type RateLimiter struct {
	mu sync.RWMutex

	// Circuit breaker state
	circuitOpen   bool
	circuitOpenAt time.Time
	banUntil      time.Time

	// Weight tracking (Binance uses weight-based limits)
	currentWeight int
	weightResetAt time.Time
	maxWeight     int // 2400 per minute for futures

	// Request tracking
	requestCount   int
	requestResetAt time.Time
	maxRequests    int // 1200 per minute

	// Backoff state
	consecutiveErrors int
	lastErrorAt       time.Time
}

// Endpoint weights for Binance Futures API
var endpointWeights = map[string]int{
	// Account endpoints
	"/fapi/v2/account":       5,
	"/fapi/v2/positionRisk":  5,
	"/fapi/v1/positionSide/dual": 30,

	// Order endpoints
	"/fapi/v1/order":         1,
	"/fapi/v1/openOrders":    1, // 1 with symbol, 40 without
	"/fapi/v1/allOpenOrders": 40,
	"/fapi/v1/allOrders":     5,
	"/fapi/v1/userTrades":    5,

	// Algo order endpoints
	"/fapi/v1/algoOrder":      1,
	"/fapi/v1/openAlgoOrders": 1,
	"/fapi/v1/allAlgoOrders":  5,

	// Market data endpoints
	"/fapi/v1/ticker/price":  1,
	"/fapi/v1/ticker/24hr":   1, // 1 with symbol, 40 without
	"/fapi/v1/klines":        5,
	"/fapi/v1/depth":         5, // depends on limit
	"/fapi/v1/premiumIndex":  1,
	"/fapi/v1/fundingRate":   1,
	"/fapi/v1/exchangeInfo":  1,

	// Income endpoints
	"/fapi/v1/income":        30,

	// Listen key
	"/fapi/v1/listenKey":     1,
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		maxWeight:      2400, // Binance Futures limit
		maxRequests:    1200, // Conservative limit
		weightResetAt:  time.Now().Add(time.Minute),
		requestResetAt: time.Now().Add(time.Minute),
	}
}

// Global rate limiter instance
var globalRateLimiter = NewRateLimiter()

// GetRateLimiter returns the global rate limiter
func GetRateLimiter() *RateLimiter {
	return globalRateLimiter
}

// CanMakeRequest checks if a request can be made (proactive check)
// This is a READ-ONLY check - does NOT record weight. Use with RecordRequest after.
// For new code, prefer TryAcquire which atomically checks AND records.
func (r *RateLimiter) CanMakeRequest(endpoint string) bool {
	return r.CanMakeRequestWithPriority(endpoint, PriorityNormal)
}

// CanMakeRequestWithPriority checks if a request can be made with specific priority
// This is a READ-ONLY check - does NOT record weight. Use with RecordRequest after.
func (r *RateLimiter) CanMakeRequestWithPriority(endpoint string, priority RequestPriority) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check circuit breaker first
	if r.circuitOpen {
		if time.Now().Before(r.banUntil) {
			return false
		}
		// Circuit can be closed, but need write lock
	}

	// Check if we need to reset counters
	now := time.Now()
	if now.After(r.weightResetAt) || now.After(r.requestResetAt) {
		return true // Will reset on actual request
	}

	// Calculate dynamic threshold based on priority
	thresholdPercent := r.getThresholdForPriority(priority)
	threshold := int(float64(r.maxWeight) * thresholdPercent)

	// Check weight limit against priority threshold
	weight := getEndpointWeight(endpoint)
	if r.currentWeight+weight > threshold {
		return false
	}

	// Check request count limit
	requestThreshold := int(float64(r.maxRequests) * thresholdPercent)
	if r.requestCount >= requestThreshold {
		return false
	}

	return true
}

// TryAcquire attempts to acquire a rate limit slot WITHOUT blocking
// Returns immediately with status - this ATOMICALLY checks AND records weight
// This is the preferred method for new non-blocking code
func (r *RateLimiter) TryAcquire(endpoint string, priority RequestPriority) AcquireResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Reset counters if window expired
	if now.After(r.weightResetAt) {
		r.currentWeight = 0
		r.weightResetAt = now.Add(time.Minute)
	}
	if now.After(r.requestResetAt) {
		r.requestCount = 0
		r.requestResetAt = now.Add(time.Minute)
	}

	// Check circuit breaker first
	if r.circuitOpen && now.Before(r.banUntil) {
		waitTime := time.Until(r.banUntil)
		return AcquireResult{
			Acquired:     false,
			WaitTime:     waitTime,
			Reason:       "circuit_breaker_open",
			WeightBudget: 0,
			CurrentUsage: 100.0,
		}
	}

	// Close circuit if ban expired
	if r.circuitOpen && now.After(r.banUntil) {
		r.circuitOpen = false
		log.Printf("[RATE-LIMITER] Circuit breaker auto-closed (ban expired)")
	}

	weight := getEndpointWeight(endpoint)

	// Calculate dynamic threshold based on priority
	thresholdPercent := r.getThresholdForPriority(priority)
	threshold := int(float64(r.maxWeight) * thresholdPercent)

	// Check weight limit against priority threshold
	if r.currentWeight+weight > threshold {
		waitTime := time.Until(r.weightResetAt)
		if waitTime < 0 {
			waitTime = 100 * time.Millisecond
		}
		return AcquireResult{
			Acquired:     false,
			WaitTime:     waitTime,
			Reason:       fmt.Sprintf("weight_limit_exceeded_for_%s_priority", priority.String()),
			WeightBudget: threshold - r.currentWeight,
			CurrentUsage: float64(r.currentWeight) / float64(r.maxWeight) * 100,
		}
	}

	// Check request count limit (use same threshold for consistency)
	requestThreshold := int(float64(r.maxRequests) * thresholdPercent)
	if r.requestCount >= requestThreshold {
		waitTime := time.Until(r.requestResetAt)
		if waitTime < 0 {
			waitTime = 100 * time.Millisecond
		}
		return AcquireResult{
			Acquired:     false,
			WaitTime:     waitTime,
			Reason:       fmt.Sprintf("request_limit_exceeded_for_%s_priority", priority.String()),
			WeightBudget: threshold - r.currentWeight,
			CurrentUsage: float64(r.currentWeight) / float64(r.maxWeight) * 100,
		}
	}

	// Acquire the slot - record the weight atomically
	r.currentWeight += weight
	r.requestCount++

	// Reset consecutive errors on successful acquire
	r.consecutiveErrors = 0

	return AcquireResult{
		Acquired:     true,
		WaitTime:     0,
		Reason:       "",
		WeightBudget: threshold - r.currentWeight,
		CurrentUsage: float64(r.currentWeight) / float64(r.maxWeight) * 100,
	}
}

// TryAcquireNonBlocking is an alias for TryAcquire for explicit non-blocking usage
func (r *RateLimiter) TryAcquireNonBlocking(endpoint string, priority RequestPriority) AcquireResult {
	return r.TryAcquire(endpoint, priority)
}

// getThresholdForPriority returns the weight threshold percentage for a priority level
// Higher priority = higher threshold = more access to the rate limit budget
func (r *RateLimiter) getThresholdForPriority(priority RequestPriority) float64 {
	switch priority {
	case PriorityCritical:
		return 0.95 // Orders can use up to 95% of budget
	case PriorityHigh:
		return 0.80 // Position checks up to 80%
	case PriorityNormal:
		return 0.60 // Market data up to 60%
	case PriorityLow:
		return 0.40 // Background scans up to 40%
	default:
		return 0.50
	}
}

// GetAdaptiveScanBudget returns how many items can be processed given current weight usage
// This enables adaptive throttling for scan loops that process multiple symbols
//
// Parameters:
//   - weightPerItem: estimated weight consumption per item (e.g., per symbol scan)
//
// Returns:
//   - itemBudget: number of items that can be processed within budget
//   - shouldThrottle: true if current usage is high and caller should reduce scope
//   - waitTime: suggested wait time if budget is exhausted
func (r *RateLimiter) GetAdaptiveScanBudget(weightPerItem int) (itemBudget int, shouldThrottle bool, waitTime time.Duration) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If less than 10 seconds until reset, suggest waiting
	remainingTime := time.Until(r.weightResetAt)
	if remainingTime < 10*time.Second && remainingTime > 0 {
		return 0, true, remainingTime
	}

	// Calculate remaining budget for low-priority scans (40% threshold)
	lowPriorityThreshold := int(float64(r.maxWeight) * 0.40)
	availableBudget := lowPriorityThreshold - r.currentWeight

	// Reserve 20% of max for potential high-priority requests during scan
	reserveForHighPriority := int(float64(r.maxWeight) * 0.20)
	availableBudget -= reserveForHighPriority

	if availableBudget < 0 {
		availableBudget = 0
	}

	// Calculate how many items can be processed
	if weightPerItem > 0 {
		itemBudget = availableBudget / weightPerItem
	} else {
		itemBudget = 0
	}

	// Determine if throttling should occur
	usagePercent := float64(r.currentWeight) / float64(r.maxWeight)
	shouldThrottle = usagePercent > 0.50 // Start throttling at 50% usage

	// If budget exhausted, calculate wait time
	if itemBudget <= 0 {
		waitTime = time.Until(r.weightResetAt)
		if waitTime < 0 {
			waitTime = 100 * time.Millisecond
		}
	}

	return itemBudget, shouldThrottle, waitTime
}

// GetCurrentUsage returns current weight usage statistics without modifying state
func (r *RateLimiter) GetCurrentUsage() (currentWeight, maxWeight int, usagePercent float64, timeUntilReset time.Duration) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	currentWeight = r.currentWeight
	maxWeight = r.maxWeight
	usagePercent = float64(r.currentWeight) / float64(r.maxWeight) * 100
	timeUntilReset = time.Until(r.weightResetAt)
	if timeUntilReset < 0 {
		timeUntilReset = 0
	}
	return
}

// WaitForSlot blocks until a request can be made (with timeout)
func (r *RateLimiter) WaitForSlot(endpoint string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if r.CanMakeRequest(endpoint) {
			return true
		}

		// Check how long to wait
		r.mu.RLock()
		var waitTime time.Duration
		if r.circuitOpen && time.Now().Before(r.banUntil) {
			waitTime = time.Until(r.banUntil)
			log.Printf("[RATE-LIMITER] Circuit open, waiting %v for ban to expire", waitTime)
		} else {
			// Wait until next reset
			waitTime = time.Until(r.weightResetAt)
			if waitTime < 0 {
				waitTime = 100 * time.Millisecond
			}
		}
		r.mu.RUnlock()

		// Cap wait time
		if waitTime > 5*time.Second {
			waitTime = 5 * time.Second
		}

		time.Sleep(waitTime)
	}

	return false
}

// RecordRequest records a successful request
func (r *RateLimiter) RecordRequest(endpoint string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Reset counters if window expired
	if now.After(r.weightResetAt) {
		r.currentWeight = 0
		r.weightResetAt = now.Add(time.Minute)
	}
	if now.After(r.requestResetAt) {
		r.requestCount = 0
		r.requestResetAt = now.Add(time.Minute)
	}

	// Record this request
	weight := getEndpointWeight(endpoint)
	r.currentWeight += weight
	r.requestCount++

	// Reset consecutive errors on success
	r.consecutiveErrors = 0

	// Close circuit if it was open and ban expired
	if r.circuitOpen && now.After(r.banUntil) {
		log.Printf("[RATE-LIMITER] Circuit breaker closed after successful request")
		r.circuitOpen = false
	}
}

// RecordRateLimitError records a rate limit error and triggers circuit breaker
func (r *RateLimiter) RecordRateLimitError(banUntilMs int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.consecutiveErrors++
	r.lastErrorAt = time.Now()

	// Calculate ban duration
	var banUntil time.Time
	if banUntilMs > 0 {
		banUntil = time.UnixMilli(banUntilMs)
	} else {
		// Default: exponential backoff based on consecutive errors
		backoff := time.Duration(1<<uint(r.consecutiveErrors)) * time.Minute
		if backoff > 30*time.Minute {
			backoff = 30 * time.Minute
		}
		banUntil = time.Now().Add(backoff)
	}

	// Open circuit breaker
	r.circuitOpen = true
	r.circuitOpenAt = time.Now()
	r.banUntil = banUntil

	log.Printf("[RATE-LIMITER] ⚠️ CIRCUIT BREAKER OPEN - IP banned until %v (consecutive errors: %d)",
		banUntil.Format("15:04:05"), r.consecutiveErrors)
}

// IsCircuitOpen returns true if circuit breaker is open
func (r *RateLimiter) IsCircuitOpen() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitOpen {
		return false
	}

	// Check if ban has expired
	if time.Now().After(r.banUntil) {
		return false // Will be closed on next successful request
	}

	return true
}

// GetStatus returns the current rate limiter status with priority budget info
func (r *RateLimiter) GetStatus() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	usagePct := float64(r.currentWeight) / float64(r.maxWeight) * 100
	timeUntilReset := time.Until(r.weightResetAt)
	if timeUntilReset < 0 {
		timeUntilReset = 0
	}

	status := map[string]interface{}{
		"circuit_open":        r.circuitOpen,
		"current_weight":      r.currentWeight,
		"max_weight":          r.maxWeight,
		"weight_usage_pct":    usagePct,
		"request_count":       r.requestCount,
		"max_requests":        r.maxRequests,
		"consecutive_errors":  r.consecutiveErrors,
		"reset_in_seconds":    int(timeUntilReset.Seconds()),
		"reset_at":            r.weightResetAt.Format(time.RFC3339),
	}

	// Add priority budget information
	status["priority_budgets"] = map[string]interface{}{
		"critical": map[string]interface{}{
			"threshold_pct": 95,
			"budget":        int(float64(r.maxWeight)*0.95) - r.currentWeight,
			"can_acquire":   r.currentWeight < int(float64(r.maxWeight)*0.95),
		},
		"high": map[string]interface{}{
			"threshold_pct": 80,
			"budget":        int(float64(r.maxWeight)*0.80) - r.currentWeight,
			"can_acquire":   r.currentWeight < int(float64(r.maxWeight)*0.80),
		},
		"normal": map[string]interface{}{
			"threshold_pct": 60,
			"budget":        int(float64(r.maxWeight)*0.60) - r.currentWeight,
			"can_acquire":   r.currentWeight < int(float64(r.maxWeight)*0.60),
		},
		"low": map[string]interface{}{
			"threshold_pct": 40,
			"budget":        int(float64(r.maxWeight)*0.40) - r.currentWeight,
			"can_acquire":   r.currentWeight < int(float64(r.maxWeight)*0.40),
		},
	}

	// Add throttle status
	status["should_throttle"] = usagePct > 50

	if r.circuitOpen {
		status["ban_until"] = r.banUntil.Format(time.RFC3339)
		status["ban_remaining_sec"] = int(time.Until(r.banUntil).Seconds())
	}

	return status
}

// UpdateFromHeaders updates weight from Binance response headers
func (r *RateLimiter) UpdateFromHeaders(usedWeight int, usedWeight1m int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Use the higher of our tracked weight or reported weight
	if usedWeight1m > r.currentWeight {
		r.currentWeight = usedWeight1m
	}

	// Log if approaching limit
	usagePct := float64(r.currentWeight) / float64(r.maxWeight) * 100
	if usagePct > 60 {
		log.Printf("[RATE-LIMITER] Weight usage: %d/%d (%.1f%%)",
			r.currentWeight, r.maxWeight, usagePct)
	}
}

// getEndpointWeight returns the weight for an endpoint
func getEndpointWeight(endpoint string) int {
	if weight, ok := endpointWeights[endpoint]; ok {
		return weight
	}
	return 1 // Default weight
}

// ParseBanUntilFromError extracts ban timestamp from Binance error message
func ParseBanUntilFromError(errMsg string) int64 {
	// Error format: "banned until 1766824120342"
	var banUntil int64
	_, err := fmt.Sscanf(errMsg, "%*[^0-9]%d", &banUntil)
	if err != nil {
		return 0
	}

	// Sanity check - should be a millisecond timestamp in the future
	if banUntil > time.Now().UnixMilli() && banUntil < time.Now().Add(24*time.Hour).UnixMilli() {
		return banUntil
	}
	return 0
}
