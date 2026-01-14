// Package database provides Redis-based order tracking with timeout.
// This tracks ALL open orders and cancels them if not filled within timeout.
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key prefixes for order tracking
const (
	// PendingOrderKeyPrefix is the prefix for pending order tracking
	// Format: ginie:pending_order:{symbol}:{orderID}
	PendingOrderKeyPrefix = "ginie:pending_order"

	// PendingOrderListKey is the key for the set of all pending order keys
	PendingOrderListKey = "ginie:pending_orders:list"

	// DefaultOrderTimeoutSec is the default timeout for orders (3 minutes)
	DefaultOrderTimeoutSec = 180
)

// PendingOrderInfo stores information about a pending order
type PendingOrderInfo struct {
	OrderID     int64     `json:"order_id"`
	Symbol      string    `json:"symbol"`
	Side        string    `json:"side"`        // BUY or SELL
	Type        string    `json:"type"`        // LIMIT, MARKET, etc.
	Price       float64   `json:"price"`       // Order price
	Quantity    float64   `json:"quantity"`    // Order quantity
	Source      string    `json:"source"`      // "scalp", "scalp_reentry", "swing", etc.
	PlacedAt    time.Time `json:"placed_at"`   // When order was placed
	TimeoutSec  int       `json:"timeout_sec"` // Timeout in seconds
	TimeoutAt   time.Time `json:"timeout_at"`  // When order should timeout
	Description string    `json:"description"` // Optional description
}

// OrderCancelFunc is a callback function to cancel an order on Binance
type OrderCancelFunc func(symbol string, orderID int64) error

// RedisOrderTracker tracks pending orders in Redis with timeout
type RedisOrderTracker struct {
	client        *redis.Client
	mu            sync.RWMutex
	cancelFunc    OrderCancelFunc
	timeoutSec    int
	stopChan      chan struct{}
	monitorWG     sync.WaitGroup
	isRunning     bool
	checkInterval time.Duration
}

// NewRedisOrderTracker creates a new RedisOrderTracker
func NewRedisOrderTracker(client *redis.Client, timeoutSec int) *RedisOrderTracker {
	if timeoutSec <= 0 {
		timeoutSec = DefaultOrderTimeoutSec
	}

	return &RedisOrderTracker{
		client:        client,
		timeoutSec:    timeoutSec,
		stopChan:      make(chan struct{}),
		checkInterval: 10 * time.Second, // Check every 10 seconds
	}
}

// SetCancelFunc sets the callback function to cancel orders on Binance
func (t *RedisOrderTracker) SetCancelFunc(fn OrderCancelFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cancelFunc = fn
}

// SetTimeoutSec updates the timeout duration
func (t *RedisOrderTracker) SetTimeoutSec(timeoutSec int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if timeoutSec > 0 {
		t.timeoutSec = timeoutSec
	}
}

// TrackOrder adds an order to the tracking system
func (t *RedisOrderTracker) TrackOrder(ctx context.Context, info PendingOrderInfo) error {
	if t.client == nil {
		log.Printf("[ORDER-TRACKER] Redis client not available, cannot track order %d", info.OrderID)
		return fmt.Errorf("redis client not available")
	}

	t.mu.RLock()
	timeoutSec := t.timeoutSec
	t.mu.RUnlock()

	// Set timeout if not specified
	if info.TimeoutSec <= 0 {
		info.TimeoutSec = timeoutSec
	}
	info.PlacedAt = time.Now()
	info.TimeoutAt = info.PlacedAt.Add(time.Duration(info.TimeoutSec) * time.Second)

	// Create Redis key
	key := fmt.Sprintf("%s:%s:%d", PendingOrderKeyPrefix, info.Symbol, info.OrderID)

	// Serialize order info
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal order info: %w", err)
	}

	// Store in Redis with TTL (timeout + 60 seconds buffer for cleanup)
	ttl := time.Duration(info.TimeoutSec+60) * time.Second
	if err := t.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store order in Redis: %w", err)
	}

	// Add to pending orders list
	if err := t.client.SAdd(ctx, PendingOrderListKey, key).Err(); err != nil {
		log.Printf("[ORDER-TRACKER] Warning: Failed to add order to list: %v", err)
	}

	log.Printf("[ORDER-TRACKER] Tracking order %d for %s, timeout in %ds at %s",
		info.OrderID, info.Symbol, info.TimeoutSec, info.TimeoutAt.Format("15:04:05"))

	return nil
}

// RemoveOrder removes an order from tracking (called when order is filled or cancelled)
func (t *RedisOrderTracker) RemoveOrder(ctx context.Context, symbol string, orderID int64) error {
	if t.client == nil {
		return nil
	}

	key := fmt.Sprintf("%s:%s:%d", PendingOrderKeyPrefix, symbol, orderID)

	// Remove from Redis
	if err := t.client.Del(ctx, key).Err(); err != nil {
		log.Printf("[ORDER-TRACKER] Warning: Failed to remove order %d from Redis: %v", orderID, err)
	}

	// Remove from list
	if err := t.client.SRem(ctx, PendingOrderListKey, key).Err(); err != nil {
		log.Printf("[ORDER-TRACKER] Warning: Failed to remove order from list: %v", err)
	}

	log.Printf("[ORDER-TRACKER] Removed order %d for %s from tracking", orderID, symbol)
	return nil
}

// GetPendingOrders returns all pending orders
func (t *RedisOrderTracker) GetPendingOrders(ctx context.Context) ([]PendingOrderInfo, error) {
	if t.client == nil {
		return nil, fmt.Errorf("redis client not available")
	}

	// Get all keys from list
	keys, err := t.client.SMembers(ctx, PendingOrderListKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending order keys: %w", err)
	}

	var orders []PendingOrderInfo
	for _, key := range keys {
		data, err := t.client.Get(ctx, key).Result()
		if err == redis.Nil {
			// Key expired, remove from list
			t.client.SRem(ctx, PendingOrderListKey, key)
			continue
		} else if err != nil {
			log.Printf("[ORDER-TRACKER] Warning: Failed to get order data for %s: %v", key, err)
			continue
		}

		var info PendingOrderInfo
		if err := json.Unmarshal([]byte(data), &info); err != nil {
			log.Printf("[ORDER-TRACKER] Warning: Failed to unmarshal order data: %v", err)
			continue
		}
		orders = append(orders, info)
	}

	return orders, nil
}

// StartMonitor starts the background monitor that cancels timed-out orders
func (t *RedisOrderTracker) StartMonitor() {
	t.mu.Lock()
	if t.isRunning {
		t.mu.Unlock()
		return
	}
	t.isRunning = true
	t.stopChan = make(chan struct{})
	t.mu.Unlock()

	t.monitorWG.Add(1)
	go t.monitorLoop()

	log.Printf("[ORDER-TRACKER] Started order timeout monitor (check every %v)", t.checkInterval)
}

// StopMonitor stops the background monitor
func (t *RedisOrderTracker) StopMonitor() {
	t.mu.Lock()
	if !t.isRunning {
		t.mu.Unlock()
		return
	}
	t.isRunning = false
	close(t.stopChan)
	t.mu.Unlock()

	t.monitorWG.Wait()
	log.Printf("[ORDER-TRACKER] Stopped order timeout monitor")
}

// monitorLoop is the background loop that checks for timed-out orders
func (t *RedisOrderTracker) monitorLoop() {
	defer t.monitorWG.Done()

	ticker := time.NewTicker(t.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			t.checkAndCancelTimedOutOrders()
		}
	}
}

// checkAndCancelTimedOutOrders checks all pending orders and cancels timed-out ones
func (t *RedisOrderTracker) checkAndCancelTimedOutOrders() {
	ctx := context.Background()

	orders, err := t.GetPendingOrders(ctx)
	if err != nil {
		log.Printf("[ORDER-TRACKER] Error getting pending orders: %v", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	now := time.Now()
	t.mu.RLock()
	cancelFunc := t.cancelFunc
	t.mu.RUnlock()

	for _, order := range orders {
		// Check if order has timed out
		if now.After(order.TimeoutAt) {
			age := now.Sub(order.PlacedAt)
			log.Printf("[ORDER-TRACKER] Order %d for %s timed out after %v (placed at %s, timeout at %s)",
				order.OrderID, order.Symbol, age.Round(time.Second),
				order.PlacedAt.Format("15:04:05"), order.TimeoutAt.Format("15:04:05"))

			// Cancel the order on Binance
			if cancelFunc != nil {
				if err := cancelFunc(order.Symbol, order.OrderID); err != nil {
					log.Printf("[ORDER-TRACKER] Failed to cancel order %d for %s: %v",
						order.OrderID, order.Symbol, err)
					// Still remove from tracking to avoid repeated cancel attempts
				} else {
					log.Printf("[ORDER-TRACKER] Successfully cancelled timed-out order %d for %s",
						order.OrderID, order.Symbol)
				}
			} else {
				log.Printf("[ORDER-TRACKER] Warning: No cancel function set, cannot cancel order %d", order.OrderID)
			}

			// Remove from tracking
			t.RemoveOrder(ctx, order.Symbol, order.OrderID)
		}
	}
}

// GetOrderStatus returns the status of a specific order
func (t *RedisOrderTracker) GetOrderStatus(ctx context.Context, symbol string, orderID int64) (*PendingOrderInfo, error) {
	if t.client == nil {
		return nil, fmt.Errorf("redis client not available")
	}

	key := fmt.Sprintf("%s:%s:%d", PendingOrderKeyPrefix, symbol, orderID)

	data, err := t.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Order not found (already filled/cancelled/expired)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	var info PendingOrderInfo
	if err := json.Unmarshal([]byte(data), &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order info: %w", err)
	}

	return &info, nil
}

// GetStats returns statistics about pending orders
func (t *RedisOrderTracker) GetStats(ctx context.Context) map[string]interface{} {
	orders, err := t.GetPendingOrders(ctx)
	if err != nil {
		return map[string]interface{}{
			"error":         err.Error(),
			"pending_count": 0,
		}
	}

	t.mu.RLock()
	timeoutSec := t.timeoutSec
	isRunning := t.isRunning
	t.mu.RUnlock()

	// Group by symbol
	bySymbol := make(map[string]int)
	for _, o := range orders {
		bySymbol[o.Symbol]++
	}

	return map[string]interface{}{
		"pending_count":    len(orders),
		"timeout_sec":      timeoutSec,
		"monitor_running":  isRunning,
		"by_symbol":        bySymbol,
		"check_interval":   t.checkInterval.String(),
	}
}
