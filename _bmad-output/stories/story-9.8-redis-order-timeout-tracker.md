# Story 9.8: Redis-Based Order Timeout Tracker

## Status: Done

## Parent Epic
- Epic 9: Production Stability & Risk Management

## Story Description
Implement a Redis-based order tracking system that monitors ALL pending LIMIT orders and automatically cancels them if they don't fill within a configurable timeout period. This prevents margin from being blocked by stale, unfilled orders.

## Problem Statement
LIMIT orders placed for entry or exit may not fill if the market moves away from the limit price. These unfilled orders:
1. Block margin indefinitely
2. Are not visible across dev/prod container instances
3. Can accumulate and reduce available trading capital
4. Previously only had in-memory tracking (lost on restart)

## Solution
A Redis-based order tracker that:
1. Tracks ALL LIMIT orders with placement timestamp
2. Monitors orders via background goroutine (every 10 seconds)
3. Automatically cancels orders that exceed timeout (default: 180 seconds / 3 minutes)
4. Shares order state across dev/prod instances via Redis
5. Uses TTL on Redis keys for automatic cleanup

## Technical Implementation

### New Files Created
- `internal/database/redis_order_tracker.go` - Redis-based order tracking with timeout

### Key Components

#### RedisOrderTracker
```go
type RedisOrderTracker struct {
    client        *redis.Client
    timeoutSec    int              // Default: 180 seconds
    checkInterval time.Duration    // Default: 10 seconds
    cancelFunc    OrderCancelFunc  // Callback to cancel on Binance
}
```

#### PendingOrderInfo
```go
type PendingOrderInfo struct {
    OrderID     int64     `json:"order_id"`
    Symbol      string    `json:"symbol"`
    Side        string    `json:"side"`        // BUY or SELL
    Type        string    `json:"type"`        // LIMIT
    Price       float64   `json:"price"`
    Quantity    float64   `json:"quantity"`
    Source      string    `json:"source"`      // Entry type (reversal, scalp, etc.)
    PlacedAt    time.Time `json:"placed_at"`
    TimeoutAt   time.Time `json:"timeout_at"`
}
```

### Integration Points

1. **GinieAutopilot struct** - Added `orderTracker *database.RedisOrderTracker`

2. **NewGinieAutopilot** - Initializes order tracker with Redis client:
```go
if positionStateRepo != nil && positionStateRepo.GetClient() != nil {
    ga.orderTracker = database.NewRedisOrderTracker(positionStateRepo.GetClient(), 180)
    ga.orderTracker.SetCancelFunc(ga.cancelOrderOnBinance)
}
```

3. **Start()** - Starts the monitor:
```go
if ga.orderTracker != nil {
    ga.orderTracker.StartMonitor()
}
```

4. **Stop()** - Stops the monitor:
```go
if ga.orderTracker != nil {
    ga.orderTracker.StopMonitor()
}
```

5. **Order Placement** - Tracks LIMIT orders when placed:
```go
// After placing LIMIT order
ga.TrackPendingOrder(symbol, orderID, side, "LIMIT", price, quantity, source)
```

### Redis Key Structure
- Individual order: `ginie:pending_order:{symbol}:{orderID}`
- Order list: `ginie:pending_orders:list`
- TTL: `timeoutSec + 60` seconds (auto-cleanup buffer)

### Helper Methods Added to GinieAutopilot
- `TrackPendingOrder()` - Add order to Redis tracker
- `RemoveTrackedOrder()` - Remove order when filled/cancelled
- `GetOrderTrackerStats()` - Get tracker statistics
- `cancelOrderOnBinance()` - Cancel order via Binance API

## Configuration
- Default timeout: 180 seconds (3 minutes)
- Check interval: 10 seconds
- Configurable via `SetTimeoutSec()`

## Logging
```
[ORDER-TRACKER] Tracking order 123456 for BTCUSDT, timeout in 180s at 12:34:56
[ORDER-TRACKER] Order 123456 for BTCUSDT timed out after 3m0s
[ORDER-CANCEL] Canceling order 123456 for BTCUSDT via Binance API
[ORDER-TRACKER] Successfully cancelled timed-out order 123456 for BTCUSDT
[ORDER-TRACKER] Removed order 123456 for BTCUSDT from tracking
```

## Acceptance Criteria
- [x] Redis-based order tracker created
- [x] Background monitor runs every 10 seconds
- [x] Orders auto-cancel after 180 second timeout
- [x] Tracker integrated into GinieAutopilot lifecycle
- [x] LIMIT orders tracked when placed (reversal, prev_candle_entry)
- [x] Orders removed from tracking when filled/cancelled
- [x] Cross-instance visibility via Redis
- [x] Automatic cleanup via Redis TTL

## Testing
1. Build dev container: `./scripts/docker-dev.sh`
2. Check logs for: `[ORDER-TRACKER] Started order timeout monitor`
3. Place a LIMIT order and verify it appears in logs
4. Wait 3 minutes and verify order is cancelled if not filled

## Dependencies
- Redis (shared between dev/prod)
- RedisPositionStateRepository (provides Redis client)

## Related Issues Fixed
- Orders staying open indefinitely (e.g., FILUSDT open for 7+ minutes)
- Margin blocked by unfilled LIMIT orders
- Order state lost on container restart
