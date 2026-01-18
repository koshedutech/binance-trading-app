// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.3: WebSocket State Sync
package decision

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// MarketUpdateType defines the type of market update received
type MarketUpdateType string

const (
	// UpdateTypePrice indicates a price tick update
	UpdateTypePrice MarketUpdateType = "PRICE"
	// UpdateTypeKline indicates a candlestick/kline close event
	UpdateTypeKline MarketUpdateType = "KLINE"
	// UpdateTypeOrderBook indicates an order book update
	UpdateTypeOrderBook MarketUpdateType = "ORDERBOOK"
)

// MarketUpdate represents incoming market data from WebSocket streams
type MarketUpdate struct {
	// Symbol is the trading pair (e.g., BTCUSDT)
	Symbol string
	// Type indicates what kind of update this is
	Type MarketUpdateType
	// Price is the current price (for PRICE and KLINE updates)
	Price float64
	// Timestamp is when this update was generated (Unix milliseconds)
	Timestamp int64
	// Data contains additional type-specific fields
	Data map[string]interface{}
}

// DefaultThrottleWindow is the default minimum interval between updates per symbol
const DefaultThrottleWindow = 100 * time.Millisecond

// WSStateSync connects WebSocket market data streams to the Redis state management.
// It processes price updates, kline closes, and order book changes, applying throttling
// to prevent excessive Redis writes while maintaining up-to-date state.
type WSStateSync struct {
	sm             *StateManager
	dp             *DeltaProcessor
	throttleWindow int64 // Stored as nanoseconds for atomic access

	// Per-symbol tracking
	lastUpdate map[string]int64 // symbol -> last update timestamp (Unix nano)
	updateMu   sync.RWMutex

	// Statistics
	updateCounts    map[string]int64 // symbol -> total update count
	throttledCounts map[string]int64 // symbol -> throttled update count
	errorCounts     map[string]int64 // symbol -> error count for graceful degradation
	countsMu        sync.RWMutex

	// Lifecycle control
	isRunning int32 // atomic: 0 = stopped, 1 = running

	// Graceful degradation
	consecutiveErrors int32 // atomic: consecutive error count
	degradedMode      int32 // atomic: 0 = normal, 1 = degraded (skip Redis writes)
}

// MaxConsecutiveErrors is the threshold before entering degraded mode
const MaxConsecutiveErrors = 5

// NewWSStateSync creates a new WebSocket State Synchronizer.
// Returns nil if StateManager or DeltaProcessor is nil.
func NewWSStateSync(sm *StateManager, dp *DeltaProcessor, throttleWindow time.Duration) *WSStateSync {
	if sm == nil || dp == nil {
		log.Printf("[DECISION] Warning: NewWSStateSync called with nil StateManager or DeltaProcessor")
		return nil
	}

	if throttleWindow <= 0 {
		throttleWindow = DefaultThrottleWindow
	}

	return &WSStateSync{
		sm:              sm,
		dp:              dp,
		throttleWindow:  int64(throttleWindow),
		lastUpdate:      make(map[string]int64),
		updateCounts:    make(map[string]int64),
		throttledCounts: make(map[string]int64),
		errorCounts:     make(map[string]int64),
	}
}

// Start marks the synchronizer as running. Call this before processing updates.
func (ws *WSStateSync) Start() {
	atomic.StoreInt32(&ws.isRunning, 1)
	atomic.StoreInt32(&ws.degradedMode, 0)
	atomic.StoreInt32(&ws.consecutiveErrors, 0)
	log.Printf("[DECISION] WSStateSync started with throttle window: %v", time.Duration(atomic.LoadInt64(&ws.throttleWindow)))
}

// Stop marks the synchronizer as stopped. Updates will be rejected after this.
func (ws *WSStateSync) Stop() {
	atomic.StoreInt32(&ws.isRunning, 0)
	log.Printf("[DECISION] WSStateSync stopped")
}

// IsRunning returns true if the synchronizer is currently running.
func (ws *WSStateSync) IsRunning() bool {
	return atomic.LoadInt32(&ws.isRunning) == 1
}

// ProcessUpdate handles an incoming market update.
// It applies throttling and routes to the appropriate handler based on update type.
// Returns nil if the update was throttled or skipped.
func (ws *WSStateSync) ProcessUpdate(ctx context.Context, userID string, update *MarketUpdate) error {
	if !ws.IsRunning() {
		return fmt.Errorf("WSStateSync is not running")
	}

	if update == nil {
		return fmt.Errorf("update cannot be nil")
	}

	if userID == "" || update.Symbol == "" {
		return fmt.Errorf("userID and symbol are required")
	}

	// Check throttling
	if ws.shouldThrottle(update.Symbol) {
		ws.recordThrottled(update.Symbol)
		return nil // Throttled, not an error
	}

	// Record this update
	ws.recordUpdate(update.Symbol)

	// Route to appropriate handler
	switch update.Type {
	case UpdateTypePrice:
		return ws.processPrice(ctx, userID, update.Symbol, update.Price, update.Timestamp)
	case UpdateTypeKline:
		return ws.processKline(ctx, userID, update.Symbol, update.Data, update.Timestamp)
	case UpdateTypeOrderBook:
		return ws.processOrderBook(ctx, userID, update.Symbol, update.Data, update.Timestamp)
	default:
		return fmt.Errorf("unknown update type: %s", update.Type)
	}
}

// shouldThrottle checks if the update for this symbol should be throttled.
// Returns true if the last update was within the throttle window.
func (ws *WSStateSync) shouldThrottle(symbol string) bool {
	now := time.Now().UnixNano()
	throttleWindowNs := atomic.LoadInt64(&ws.throttleWindow)

	ws.updateMu.RLock()
	lastUpdate, exists := ws.lastUpdate[symbol]
	ws.updateMu.RUnlock()

	if !exists {
		return false // First update for this symbol
	}

	elapsed := now - lastUpdate
	return elapsed < throttleWindowNs
}

// recordUpdate updates the last update time and increment count for a symbol.
func (ws *WSStateSync) recordUpdate(symbol string) {
	now := time.Now().UnixNano()

	ws.updateMu.Lock()
	ws.lastUpdate[symbol] = now
	ws.updateMu.Unlock()

	ws.countsMu.Lock()
	ws.updateCounts[symbol]++
	ws.countsMu.Unlock()
}

// recordThrottled increments the throttled count for a symbol.
func (ws *WSStateSync) recordThrottled(symbol string) {
	ws.countsMu.Lock()
	ws.throttledCounts[symbol]++
	ws.countsMu.Unlock()
}

// processPrice handles price tick updates.
// Updates the price field in coin state.
// Implements graceful degradation: in degraded mode, updates cache only (skips Redis).
func (ws *WSStateSync) processPrice(ctx context.Context, userID, symbol string, price float64, timestamp int64) error {
	if price <= 0 {
		return fmt.Errorf("invalid price: %f", price)
	}

	// Use DeltaProcessor to only update if price changed
	state := map[string]interface{}{
		"price":        price,
		"last_updated": fmt.Sprintf("%d", timestamp),
	}

	// In degraded mode, skip Redis writes to prevent cascading failures
	if ws.IsDegraded() {
		_, err := ws.dp.ProcessWithoutRedis(userID, symbol, state)
		if err != nil {
			return fmt.Errorf("failed to process price update (degraded): %w", err)
		}
		return nil
	}

	_, err := ws.dp.Process(ctx, userID, symbol, state)
	if err != nil {
		ws.recordError(symbol)
		return fmt.Errorf("failed to process price update: %w", err)
	}

	ws.recordSuccess()
	return nil
}

// processKline handles candlestick/kline close events.
// Updates indicators (ADX, RSI, EMAs) and potentially triggers regime analysis.
// Implements graceful degradation: in degraded mode, updates cache only (skips Redis).
func (ws *WSStateSync) processKline(ctx context.Context, userID, symbol string, data map[string]interface{}, timestamp int64) error {
	if data == nil {
		return fmt.Errorf("kline data cannot be nil")
	}

	// Build state update from kline data
	state := make(map[string]interface{})
	state["last_updated"] = fmt.Sprintf("%d", timestamp)

	// Extract and validate optional indicator fields
	if close, ok := extractFloat(data, "close"); ok {
		state["price"] = close
	}
	if adx, ok := extractFloat(data, "adx"); ok {
		state["adx"] = adx
	}
	if rsi, ok := extractFloat(data, "rsi"); ok {
		state["rsi"] = rsi
	}
	if ema9, ok := extractFloat(data, "ema_9"); ok {
		state["ema_9"] = ema9
	}
	if ema21, ok := extractFloat(data, "ema_21"); ok {
		state["ema_21"] = ema21
	}

	// Extract trend directions if provided
	if trend1h, ok := data["trend_1h"].(string); ok && trend1h != "" {
		state["trend_1h"] = trend1h
	}
	if trend15m, ok := data["trend_15m"].(string); ok && trend15m != "" {
		state["trend_15m"] = trend15m
	}

	// Extract regime if provided
	if regime, ok := data["regime"].(string); ok && regime != "" {
		state["regime"] = regime
	}
	if strategy, ok := data["active_strategy"].(string); ok && strategy != "" {
		state["active_strategy"] = strategy
	}

	// Only process if we have data beyond just timestamp
	if len(state) <= 1 {
		return nil // No meaningful data to update
	}

	// In degraded mode, skip Redis writes to prevent cascading failures
	if ws.IsDegraded() {
		_, err := ws.dp.ProcessWithoutRedis(userID, symbol, state)
		if err != nil {
			return fmt.Errorf("failed to process kline update (degraded): %w", err)
		}
		return nil
	}

	_, err := ws.dp.Process(ctx, userID, symbol, state)
	if err != nil {
		ws.recordError(symbol)
		return fmt.Errorf("failed to process kline update: %w", err)
	}

	ws.recordSuccess()
	return nil
}

// processOrderBook handles order book updates.
// Extracts spread and liquidity metrics from order book data.
// Implements graceful degradation: in degraded mode, updates cache only (skips Redis).
func (ws *WSStateSync) processOrderBook(ctx context.Context, userID, symbol string, data map[string]interface{}, timestamp int64) error {
	if data == nil {
		return fmt.Errorf("order book data cannot be nil")
	}

	// Build state update from order book data
	state := make(map[string]interface{})
	state["last_updated"] = fmt.Sprintf("%d", timestamp)

	// Extract spread if provided directly
	if spread, ok := extractFloat(data, "spread"); ok {
		state["spread"] = spread
	} else {
		// Calculate spread from best bid/ask if available
		bestBid, hasBid := extractFloat(data, "best_bid")
		bestAsk, hasAsk := extractFloat(data, "best_ask")
		if hasBid && hasAsk && bestBid > 0 && bestAsk > 0 {
			spread := (bestAsk - bestBid) / bestBid * 100 // Spread as percentage
			state["spread"] = spread
		}
	}

	// Extract liquidity score if provided
	if liquidity, ok := extractFloat(data, "liquidity_score"); ok {
		state["liquidity_score"] = liquidity
	}

	// Extract bid/ask depth if provided
	if bidDepth, ok := extractFloat(data, "bid_depth"); ok {
		state["bid_depth"] = bidDepth
	}
	if askDepth, ok := extractFloat(data, "ask_depth"); ok {
		state["ask_depth"] = askDepth
	}

	// Only process if we have data beyond just timestamp
	if len(state) <= 1 {
		return nil // No meaningful data to update
	}

	// In degraded mode, skip Redis writes to prevent cascading failures
	if ws.IsDegraded() {
		_, err := ws.dp.ProcessWithoutRedis(userID, symbol, state)
		if err != nil {
			return fmt.Errorf("failed to process order book update (degraded): %w", err)
		}
		return nil
	}

	_, err := ws.dp.Process(ctx, userID, symbol, state)
	if err != nil {
		ws.recordError(symbol)
		return fmt.Errorf("failed to process order book update: %w", err)
	}

	ws.recordSuccess()
	return nil
}

// extractFloat attempts to extract a float64 from a map value.
// Handles float64, int, int64, and string types.
func extractFloat(data map[string]interface{}, key string) (float64, bool) {
	val, exists := data[key]
	if !exists {
		return 0, false
	}

	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f := parseFloat(v)
		return f, f != 0 || v == "0"
	default:
		return 0, false
	}
}

// GetUpdateStats returns a copy of the update counts per symbol.
func (ws *WSStateSync) GetUpdateStats() map[string]int64 {
	ws.countsMu.RLock()
	defer ws.countsMu.RUnlock()

	result := make(map[string]int64, len(ws.updateCounts))
	for k, v := range ws.updateCounts {
		result[k] = v
	}
	return result
}

// GetThrottledStats returns a copy of the throttled counts per symbol.
func (ws *WSStateSync) GetThrottledStats() map[string]int64 {
	ws.countsMu.RLock()
	defer ws.countsMu.RUnlock()

	result := make(map[string]int64, len(ws.throttledCounts))
	for k, v := range ws.throttledCounts {
		result[k] = v
	}
	return result
}

// GetThrottledCount returns the total number of throttled updates across all symbols.
func (ws *WSStateSync) GetThrottledCount() int64 {
	ws.countsMu.RLock()
	defer ws.countsMu.RUnlock()

	var total int64
	for _, count := range ws.throttledCounts {
		total += count
	}
	return total
}

// GetTotalUpdateCount returns the total number of processed updates across all symbols.
func (ws *WSStateSync) GetTotalUpdateCount() int64 {
	ws.countsMu.RLock()
	defer ws.countsMu.RUnlock()

	var total int64
	for _, count := range ws.updateCounts {
		total += count
	}
	return total
}

// GetSymbolCount returns the number of unique symbols being tracked.
func (ws *WSStateSync) GetSymbolCount() int {
	ws.updateMu.RLock()
	defer ws.updateMu.RUnlock()

	return len(ws.lastUpdate)
}

// ResetStats clears all update and throttle counters.
func (ws *WSStateSync) ResetStats() {
	ws.countsMu.Lock()
	ws.updateCounts = make(map[string]int64)
	ws.throttledCounts = make(map[string]int64)
	ws.errorCounts = make(map[string]int64)
	ws.countsMu.Unlock()
	atomic.StoreInt32(&ws.consecutiveErrors, 0)
}

// GetTotalErrorCount returns the total number of errors across all symbols.
func (ws *WSStateSync) GetTotalErrorCount() int64 {
	ws.countsMu.RLock()
	defer ws.countsMu.RUnlock()

	var total int64
	for _, count := range ws.errorCounts {
		total += count
	}
	return total
}

// IsDegraded returns true if the synchronizer is in degraded mode.
func (ws *WSStateSync) IsDegraded() bool {
	return atomic.LoadInt32(&ws.degradedMode) == 1
}

// recordError records an error for a symbol and manages degraded mode.
func (ws *WSStateSync) recordError(symbol string) {
	ws.countsMu.Lock()
	ws.errorCounts[symbol]++
	ws.countsMu.Unlock()

	// Increment consecutive errors
	errors := atomic.AddInt32(&ws.consecutiveErrors, 1)
	if errors >= MaxConsecutiveErrors && atomic.LoadInt32(&ws.degradedMode) == 0 {
		atomic.StoreInt32(&ws.degradedMode, 1)
		log.Printf("[DECISION] WSStateSync entering degraded mode after %d consecutive errors", errors)
	}
}

// recordSuccess resets consecutive error count on successful operation.
func (ws *WSStateSync) recordSuccess() {
	if atomic.LoadInt32(&ws.consecutiveErrors) > 0 {
		atomic.StoreInt32(&ws.consecutiveErrors, 0)
	}
	// Exit degraded mode on success
	if atomic.LoadInt32(&ws.degradedMode) == 1 {
		atomic.StoreInt32(&ws.degradedMode, 0)
		log.Printf("[DECISION] WSStateSync exiting degraded mode - connection restored")
	}
}

// ResetDegradedMode manually exits degraded mode.
func (ws *WSStateSync) ResetDegradedMode() {
	atomic.StoreInt32(&ws.degradedMode, 0)
	atomic.StoreInt32(&ws.consecutiveErrors, 0)
	log.Printf("[DECISION] WSStateSync degraded mode manually reset")
}

// ClearSymbol removes all tracking data for a specific symbol.
func (ws *WSStateSync) ClearSymbol(symbol string) {
	ws.updateMu.Lock()
	delete(ws.lastUpdate, symbol)
	ws.updateMu.Unlock()

	ws.countsMu.Lock()
	delete(ws.updateCounts, symbol)
	delete(ws.throttledCounts, symbol)
	delete(ws.errorCounts, symbol)
	ws.countsMu.Unlock()
}

// SetThrottleWindow updates the throttle window. Changes take effect immediately.
// Thread-safe via atomic operations.
func (ws *WSStateSync) SetThrottleWindow(window time.Duration) {
	if window <= 0 {
		window = DefaultThrottleWindow
	}
	atomic.StoreInt64(&ws.throttleWindow, int64(window))
	log.Printf("[DECISION] WSStateSync throttle window changed to: %v", window)
}

// GetThrottleWindow returns the current throttle window duration.
// Thread-safe via atomic operations.
func (ws *WSStateSync) GetThrottleWindow() time.Duration {
	return time.Duration(atomic.LoadInt64(&ws.throttleWindow))
}

// WSSyncStats provides a snapshot of synchronizer statistics.
type WSSyncStats struct {
	IsRunning       bool
	ThrottleWindow  time.Duration
	SymbolCount     int
	TotalUpdates    int64
	TotalThrottled  int64
	TotalErrors     int64
	InDegradedMode  bool
	UpdatesBySymbol map[string]int64
}

// GetStats returns a comprehensive snapshot of synchronizer state.
func (ws *WSStateSync) GetStats() WSSyncStats {
	return WSSyncStats{
		IsRunning:       ws.IsRunning(),
		ThrottleWindow:  ws.GetThrottleWindow(),
		SymbolCount:     ws.GetSymbolCount(),
		TotalUpdates:    ws.GetTotalUpdateCount(),
		TotalThrottled:  ws.GetThrottledCount(),
		TotalErrors:     ws.GetTotalErrorCount(),
		InDegradedMode:  ws.IsDegraded(),
		UpdatesBySymbol: ws.GetUpdateStats(),
	}
}
