# Story 7.20: Order Chain Redis Cache Layer

## Story Overview

**Story ID:** 7-20
**Epic:** 7 - Client Order ID & Trade Lifecycle Tracking
**Priority:** P1
**Status:** done
**Created:** 2026-01-18
**Complexity:** Medium
**Depends On:** Story 7.16 (Event Store), Story 7.17 (Event Writer)

---

## Goal

Implement Redis caching for active order chains to provide fast reads for the UI and reduce PostgreSQL load during active trading.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         DATA FLOW                                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  WRITE PATH:                                                         │
│  Autopilot → ChainEventWriter → PostgreSQL → Redis Cache             │
│                                     │              │                 │
│                                     ↓              ↓                 │
│                              (Source of Truth)  (Fast Access)        │
│                                                                      │
│  READ PATH (Active Chains):                                          │
│  UI → API → Redis Cache (hit) → Return                               │
│            → Redis Cache (miss) → PostgreSQL → Update Cache → Return │
│                                                                      │
│  READ PATH (Historical/Closed):                                      │
│  UI → API → PostgreSQL (no cache for closed chains)                  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Acceptance Criteria

- [x] AC1: Active chain state cached in Redis as JSON
- [x] AC2: Cache key pattern: `order_chain:{user_id}:{chain_id}`
- [x] AC3: NO TTL for active chains (explicit delete on close)
- [x] AC4: Cache updated after every PostgreSQL write (write-through)
- [x] AC5: Cache miss triggers PostgreSQL read + cache population
- [x] AC6: Chain close event deletes from cache
- [x] AC7: Active chains list cached per user: `order_chains:active:{user_id}`
- [x] AC8: WebSocket can read from cache for real-time updates
- [x] AC9: Startup: Warm cache from PostgreSQL for all active chains
- [x] AC10: Cache invalidation on user logout (optional cleanup)

---

## Redis Key Structure

### Individual Chain Cache

```
Key: order_chain:{user_id}:{chain_id}
Type: STRING (JSON blob)
TTL: None (until chain closes)

Value:
{
    "chainId": "ULT-18JAN-00001",
    "symbol": "BTCUSDT",
    "side": "LONG",
    "modeCode": "ULT",
    "status": "ACTIVE",

    "entry": {
        "price": 97450.00,
        "quantity": 0.01,
        "status": "FILLED",
        "filledAt": "2026-01-18T09:15:32Z",
        "binanceOrderId": 123456789
    },

    "position": {
        "status": "ACTIVE",
        "entryPrice": 97450.00,
        "quantity": 0.01,
        "remainingQuantity": 0.01
    },

    "stopLoss": {
        "currentPrice": 97100.00,
        "binanceOrderId": 123456790,
        "modificationCount": 3,
        "lastModified": "2026-01-18T11:30:00Z"
    },

    "takeProfit": {
        "mode": "NORMAL",
        "currentPrice": 98000.00,
        "binanceOrderId": 123456791,
        "modificationCount": 1
    },

    "hedgeChainId": null,
    "eventCount": 8,
    "lastEventSeq": 8,
    "updatedAt": "2026-01-18T11:30:00Z"
}
```

### Active Chains Index (per user)

```
Key: order_chains:active:{user_id}
Type: SET
TTL: None

Members: ["ULT-18JAN-00001", "SCA-18JAN-00002", ...]
```

### Recent Events (for WebSocket broadcast)

```
Key: order_chain_events:recent:{user_id}
Type: LIST (capped at 100)
TTL: 1 hour

Items: [
    {"chainId": "...", "eventType": "SL_MODIFIED", "timestamp": "..."},
    ...
]
```

---

## Implementation

### Cache Service Extensions

```go
// internal/cache/order_chain_cache.go

type OrderChainCache struct {
    cache *CacheService
    log   zerolog.Logger
}

func NewOrderChainCache(cache *CacheService) *OrderChainCache

// --- Chain State ---

// SetChainState caches the current chain state
func (c *OrderChainCache) SetChainState(ctx context.Context, chain *OrderChainState) error {
    key := fmt.Sprintf("order_chain:%s:%s", chain.UserID, chain.ChainID)
    data, err := json.Marshal(chain)
    if err != nil {
        return err
    }
    return c.cache.Set(ctx, key, string(data), 0)  // No TTL
}

// GetChainState retrieves cached chain state
func (c *OrderChainCache) GetChainState(ctx context.Context, userID, chainID string) (*OrderChainState, error) {
    key := fmt.Sprintf("order_chain:%s:%s", userID, chainID)
    data, err := c.cache.Get(ctx, key)
    if err != nil {
        return nil, err  // Cache miss
    }
    var chain OrderChainState
    if err := json.Unmarshal([]byte(data), &chain); err != nil {
        return nil, err
    }
    return &chain, nil
}

// DeleteChainState removes chain from cache (on close)
func (c *OrderChainCache) DeleteChainState(ctx context.Context, userID, chainID string) error {
    key := fmt.Sprintf("order_chain:%s:%s", userID, chainID)
    return c.cache.Delete(ctx, key)
}

// --- Active Chains Index ---

// AddToActiveChains adds chain to user's active set
func (c *OrderChainCache) AddToActiveChains(ctx context.Context, userID, chainID string) error {
    key := fmt.Sprintf("order_chains:active:%s", userID)
    return c.cache.SAdd(ctx, key, chainID)
}

// RemoveFromActiveChains removes chain from user's active set
func (c *OrderChainCache) RemoveFromActiveChains(ctx context.Context, userID, chainID string) error {
    key := fmt.Sprintf("order_chains:active:%s", userID)
    return c.cache.SRem(ctx, key, chainID)
}

// GetActiveChains returns all active chain IDs for user
func (c *OrderChainCache) GetActiveChains(ctx context.Context, userID string) ([]string, error) {
    key := fmt.Sprintf("order_chains:active:%s", userID)
    return c.cache.SMembers(ctx, key)
}

// --- Recent Events (for WebSocket) ---

// PushRecentEvent adds event to recent list (capped)
func (c *OrderChainCache) PushRecentEvent(ctx context.Context, userID string, event *ChainEventSummary) error {
    key := fmt.Sprintf("order_chain_events:recent:%s", userID)
    data, _ := json.Marshal(event)

    // Push to list, trim to 100 items
    pipe := c.cache.Pipeline()
    pipe.LPush(ctx, key, string(data))
    pipe.LTrim(ctx, key, 0, 99)
    pipe.Expire(ctx, key, time.Hour)
    _, err := pipe.Exec(ctx)
    return err
}

// GetRecentEvents returns recent events for WebSocket catch-up
func (c *OrderChainCache) GetRecentEvents(ctx context.Context, userID string, limit int) ([]*ChainEventSummary, error) {
    key := fmt.Sprintf("order_chain_events:recent:%s", userID)
    items, err := c.cache.LRange(ctx, key, 0, int64(limit-1))
    if err != nil {
        return nil, err
    }

    events := make([]*ChainEventSummary, 0, len(items))
    for _, item := range items {
        var event ChainEventSummary
        if err := json.Unmarshal([]byte(item), &event); err == nil {
            events = append(events, &event)
        }
    }
    return events, nil
}

// --- Batch Operations ---

// GetMultipleChains retrieves multiple chains (for list view)
func (c *OrderChainCache) GetMultipleChains(ctx context.Context, userID string, chainIDs []string) ([]*OrderChainState, error) {
    keys := make([]string, len(chainIDs))
    for i, id := range chainIDs {
        keys[i] = fmt.Sprintf("order_chain:%s:%s", userID, id)
    }

    values, err := c.cache.MGet(ctx, keys...)
    if err != nil {
        return nil, err
    }

    chains := make([]*OrderChainState, 0, len(values))
    for _, v := range values {
        if v == nil {
            continue
        }
        var chain OrderChainState
        if err := json.Unmarshal([]byte(v.(string)), &chain); err == nil {
            chains = append(chains, &chain)
        }
    }
    return chains, nil
}

// --- Warm-up ---

// WarmCacheForUser loads all active chains for user into cache
func (c *OrderChainCache) WarmCacheForUser(ctx context.Context, userID string, chains []*OrderChainState) error {
    pipe := c.cache.Pipeline()

    // Clear old active set
    activeKey := fmt.Sprintf("order_chains:active:%s", userID)
    pipe.Del(ctx, activeKey)

    for _, chain := range chains {
        if chain.Status != "ACTIVE" && chain.Status != "PARTIAL" {
            continue
        }

        // Cache chain state
        key := fmt.Sprintf("order_chain:%s:%s", userID, chain.ChainID)
        data, _ := json.Marshal(chain)
        pipe.Set(ctx, key, string(data), 0)

        // Add to active set
        pipe.SAdd(ctx, activeKey, chain.ChainID)
    }

    _, err := pipe.Exec(ctx)
    return err
}
```

---

## Integration with ChainEventWriter

```go
// internal/orders/chain_event_writer.go

func (w *ChainEventWriter) RecordSLModified(ctx context.Context, chainID string, req SLModifiedEvent) error {
    // 1. Write to PostgreSQL (transaction)
    err := w.db.Transaction(ctx, func(tx *sql.Tx) error {
        // ... insert event, update master table
    })
    if err != nil {
        return err
    }

    // 2. Update Redis cache (best-effort, outside transaction)
    go w.updateCacheAfterSLModified(ctx, chainID, req)

    // 3. Push to recent events for WebSocket
    go w.pushRecentEvent(ctx, chainID, "SL_MODIFIED", req)

    return nil
}

func (w *ChainEventWriter) updateCacheAfterSLModified(ctx context.Context, chainID string, req SLModifiedEvent) {
    // Get fresh chain state from DB
    chain, err := w.db.GetOrderChain(ctx, chainID)
    if err != nil {
        w.log.Warn().Err(err).Msg("Failed to refresh cache")
        return
    }

    // Update cache
    state := w.toOrderChainState(chain)
    w.chainCache.SetChainState(ctx, state)
}
```

---

## Startup Cache Warming

```go
// internal/startup/cache_warmer.go

func WarmOrderChainCache(ctx context.Context, db *database.Repository, cache *OrderChainCache) error {
    // Get all users with active positions
    users, err := db.GetUsersWithActiveChains(ctx)
    if err != nil {
        return err
    }

    for _, userID := range users {
        // Get active chains for user
        chains, err := db.GetActiveOrderChains(ctx, userID)
        if err != nil {
            log.Warn().Str("user_id", userID).Err(err).Msg("Failed to warm cache for user")
            continue
        }

        // Warm cache
        if err := cache.WarmCacheForUser(ctx, userID, chains); err != nil {
            log.Warn().Str("user_id", userID).Err(err).Msg("Failed to warm cache")
        } else {
            log.Info().Str("user_id", userID).Int("chains", len(chains)).Msg("Warmed cache for user")
        }
    }

    return nil
}
```

---

## Files to Create

| File | Description |
|------|-------------|
| `internal/cache/order_chain_cache.go` | Cache operations |
| `internal/cache/order_chain_types.go` | Cached state types |
| `internal/startup/cache_warmer.go` | Startup warming |
| `internal/cache/order_chain_cache_test.go` | Unit tests |

---

## Test Scenarios

1. **Set/Get chain** - Cache round-trip works
2. **Active chains index** - Add/remove/list works
3. **Cache miss** - Returns nil, not error
4. **Cache delete on close** - Chain removed from cache and active set
5. **Batch get** - Multiple chains retrieved efficiently
6. **Warm cache** - All active chains loaded on startup
7. **Recent events** - Capped list works, old events evicted

---

## Definition of Done

- [x] Cache service implemented
- [x] Write-through from ChainEventWriter
- [x] Active chains index maintained
- [x] Startup warming works
- [x] Unit tests passing
- [x] Build passes

---

## Implementation Summary (2026-01-18)

### Files Created

| File | Description |
|------|-------------|
| `internal/cache/order_chain_types.go` | Cached state types (OrderChainState, ChainEventSummary) |
| `internal/cache/order_chain_cache.go` | Cache operations (SetChainState, GetChainState, etc.) |
| `internal/cache/order_chain_cache_adapter.go` | Adapter implementing orders.ChainCacheInterface |
| `internal/cache/order_chain_cache_test.go` | Unit tests for cache operations |
| `internal/startup/cache_warmer.go` | Startup cache warming logic |

### Files Modified

| File | Changes |
|------|---------|
| `internal/orders/chain_event_writer.go` | Added write-through caching, cache interface |
| `internal/api/server.go` | Added OrderChainCache field and setter |
| `internal/api/handlers_futures.go` | Added cache-first API endpoints |

### API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/futures/order-chains/cached` | Get all active chains (cache-first) |
| `GET /api/futures/order-chains/cached/:chainId` | Get single chain (cache-first) |

### Key Features

1. **Write-Through Caching**: ChainEventWriter updates cache after every PostgreSQL write
2. **Active Chains Index**: Redis SET tracks active chain IDs per user
3. **Recent Events List**: Capped LIST (100 items, 1hr TTL) for WebSocket catch-up
4. **Startup Warming**: CacheWarmer pre-populates cache on application start
5. **Graceful Degradation**: All cache operations handle nil/unhealthy cache gracefully
6. **Cache Adapter**: OrderChainCacheAdapter bridges orders and cache packages

---

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-18 | Story created | System |
| 2026-01-18 | Implementation complete - cache service, write-through, API endpoints, tests | Claude |
