# Position Lifecycle Coordinator - Full Refactor Plan

## Epic: Replace GinieAutopilot with Position Lifecycle Coordinator for Chain-Based Trades

**Date**: 2026-02-08
**Priority**: Critical
**Scope**: Backend (Go) + Frontend (React/TypeScript)
**Estimated Files**: 15-20 files modified, 2-3 new files

---

## Problem Statement

The current position close lifecycle is fragmented across 4+ systems (GinieAutopilot, FuturesController, ChainEventWriter callbacks, PositionController) with:
- 500ms sleep delays as race condition patches
- 10s background goroutine for CoinProfiler re-initialization
- Empty POSITION_UPDATE arrays silently ignored by frontend
- Counterpart SL/TP order status not persisted (shows "pending" after close)
- Duplicate WebSocket broadcasts causing race conditions
- No single coordinator - each system maintains its own position state view

## Solution: Position Lifecycle Coordinator

Create a **single coordinator** that owns the entire chain-based position lifecycle from SL/TP fill detection through frontend broadcast. GinieAutopilot remains ONLY for legacy direct trades.

---

## Architecture: Before vs After

### BEFORE (Current - Fragmented)
```
WebSocket → FuturesController → GinieAutopilot.HandleSLTPOrderFilled()
                               → ChainEventWriter.CloseChain() + 3 callbacks
                               → Callback #2 → background goroutine (10s)
                               → Callback #3 → BroadcastChainClosed (partial data)
                               → broadcastPositionClosure() (empty array bug)
                               → Frontend: 2 events, 500ms refetch, manual refresh needed
```

### AFTER (New - Single Flow)
```
WebSocket → FuturesController → PositionLifecycleCoordinator.HandleOrderFill()
                                  1. Identify chain (DB lookup by Binance order ID)
                                  2. Update filled order status in DB
                                  3. Update counterpart order → CANCELED in DB
                                  4. Close chain in DB (single transaction)
                                  5. Reset pattern (synchronous)
                                  6. Update CoinProfiler capacity (synchronous)
                                  7. Broadcast ONE composite event to frontend
                                  → Frontend: 1 event, ALL components update instantly
```

---

## Implementation Steps

### STEP 1: Create PositionLifecycleCoordinator (Backend - New File)

**New file**: `internal/autopilot/position_lifecycle_coordinator.go`

**Struct**:
```go
type PositionLifecycleCoordinator struct {
    mu              sync.RWMutex
    db              *database.DB
    cache           *cache.Service
    chainWriter     *orders.ChainEventWriter
    patternMatcher  PatternResetInterface
    coinProfiler    CoinProfilerInterface
    broadcaster     BroadcasterInterface
    logger          *log.Logger
}
```

**Core method**: `HandleOrderFill(ctx, userID, event WebSocketOrderEvent) error`

This method replaces `GinieAutopilot.HandleSLTPOrderFilled()` for chain-based positions:

1. **Identify the chain**: Query `order_chains` table by `sl_binance_order_id` or `tp_binance_order_id`
   - If no match → return (let GinieAutopilot handle as legacy)
   - If match → this is a chain-based position, Coordinator owns it

2. **Determine what filled**: SL or TP based on order type/client order ID

3. **Update filled order in DB**:
   - `UpdateOrderChainSLFilled()` or `UpdateOrderChainTPFilled()`
   - Store fill price, fill time, quantity, status = "FILLED"

4. **Update counterpart order in DB** (NEW - currently missing):
   - If SL filled → `UpdateOrderChainTPStatus(chainID, "CANCELED", time.Now())`
   - If TP filled → `UpdateOrderChainSLStatus(chainID, "CANCELED", time.Now())`
   - Cancel counterpart on Binance: `CancelAllAlgoOrders(symbol)`

5. **Calculate realized PnL**:
   - `realizedPnL = (closePrice - entryPrice) * quantity * direction`
   - `totalFees = entryFees + closeFees`
   - `netPnL = realizedPnL - totalFees`

6. **Close chain in DB** (single operation):
   - `CloseOrderChain(ctx, chainID, closeReason, netPnL, totalFees, &closePrice)`
   - Sets: status=CLOSED, closed_at=NOW(), close_price, realized_pnl, total_fees

7. **Reset Entry Decision pattern** (SYNCHRONOUS, no delay):
   - `patternMatcher.ResetPatternForSymbol(symbol, mode, timeframe)`
   - Clears in-memory state, deletes DB row, removes suppression

8. **Update CoinProfiler capacity** (SYNCHRONOUS, no delay):
   - `coinProfiler.UpdateSymbolToStrategy(symbol)`
   - `coinProfiler.RebuildSubscriptions(userID)` ← NEW synchronous method
   - Immediately recalculates capacity: activeChains count vs maxConcurrent

9. **Broadcast composite event** (SINGLE event, ALL data):
   - New event type: `CHAIN_LIFECYCLE_UPDATE`
   - Contains everything the frontend needs in one payload

**Composite event payload**:
```go
compositeEvent := map[string]interface{}{
    "event_type": "CHAIN_LIFECYCLE_UPDATE",
    "chain": map[string]interface{}{
        "chain_id":     chainID,
        "status":       "CLOSED",
        "close_reason": closeReason, // "SL_HIT" or "TP_HIT"
        "realized_pnl": netPnL,
        "total_fees":   totalFees,
        "close_price":  closePrice,
        "closed_at":    time.Now().UnixMilli(),
    },
    "position": map[string]interface{}{
        "symbol":        symbol,
        "status":        "CLOSED",
        "position_side": positionSide,
    },
    "orders": map[string]interface{}{
        "sl_status":     slStatus,     // "FILLED" or "CANCELED"
        "tp_status":     tpStatus,     // "FILLED" or "CANCELED"
        "sl_fill_price": slFillPrice,  // actual fill price or nil
        "tp_fill_price": tpFillPrice,  // actual fill price or nil
        "filled_at":     fillTime,
    },
    "pattern": map[string]interface{}{
        "symbol":    symbol,
        "status":    "watching",
        "step":      1,
        "mode":      mode,
        "timeframe": timeframe,
    },
    "profiler": map[string]interface{}{
        "capacity_used": newCapacityUsed,
        "max_capacity":  maxConcurrent,
        "scanning":      newCapacityUsed < maxConcurrent,
    },
}
events.BroadcastChainLifecycleUpdate(userID, compositeEvent)
```

---

### STEP 2: Add New Broadcast Function (Backend - events/bus.go)

**File**: `internal/events/bus.go`

Add new event type and broadcast function:

```go
const EventChainLifecycleUpdate = "CHAIN_LIFECYCLE_UPDATE"

func BroadcastChainLifecycleUpdate(userID string, data map[string]interface{}) {
    // Single broadcast that contains ALL close data
}
```

**File**: `internal/api/websocket_user.go`

Add WebSocket broadcast handler for the new event type.

---

### STEP 3: Add Counterpart Order Status Update (Backend - DB)

**File**: `internal/database/repository_order_chains.go`

Add two new methods:

```go
func (db *DB) UpdateOrderChainSLCanceled(ctx context.Context, chainID string, canceledAt time.Time) error {
    query := `UPDATE order_chains SET sl_status = 'CANCELED', sl_canceled_at = $2, updated_at = NOW() WHERE chain_id = $1`
    _, err := db.pool.Exec(ctx, query, chainID, canceledAt)
    return err
}

func (db *DB) UpdateOrderChainTPCanceled(ctx context.Context, chainID string, canceledAt time.Time) error {
    query := `UPDATE order_chains SET tp_status = 'CANCELED', tp_canceled_at = $2, updated_at = NOW() WHERE chain_id = $1`
    _, err := db.pool.Exec(ctx, query, chainID, canceledAt)
    return err
}
```

**Migration**: Add `sl_canceled_at` and `tp_canceled_at` columns if not present.

---

### STEP 4: Add Synchronous CoinProfiler Rebuild (Backend)

**File**: `internal/coinprofiler/profiler.go`

Add new method (replaces 10s background goroutine):

```go
func (cp *CoinProfiler) RebuildCapacity(activeChainCount int, maxConcurrent int) {
    cp.mu.Lock()
    defer cp.mu.Unlock()

    cp.capacityUsed = activeChainCount
    cp.maxCapacity = maxConcurrent
    cp.scanningEnabled = activeChainCount < maxConcurrent

    // If capacity freed, re-aggregate strategy requirements immediately
    if cp.scanningEnabled {
        cp.reAggregateStrategyRequirements()
    }
}
```

**File**: `internal/autopilot/user_autopilot_manager.go`

Remove the background goroutine in `onChainClosedSymbol` callback:
```go
// REMOVE THIS:
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    m.initializeCoinProfilerSubscriptions(ctx, userID, inst)
}()

// REPLACED BY: Coordinator calls CoinProfiler.RebuildCapacity() synchronously
```

---

### STEP 5: Rewire FuturesController Event Routing (Backend)

**File**: `internal/autopilot/futures_controller.go`

In `HandleStreamOrderUpdate()`, add routing logic:

```go
// When SL/TP order fills:
if isSLTPOrder(order) && order.Status == "FILLED" {
    // Try Coordinator first (chain-based positions)
    handled, err := fc.coordinator.HandleOrderFill(ctx, fc.ownerUserID, order)
    if err != nil {
        log.Printf("Coordinator error: %v", err)
    }
    if !handled {
        // Fallback to GinieAutopilot (legacy positions)
        fc.ginieAutopilot.HandleSLTPOrderFilled(ctx, order)
    }
}
```

The Coordinator returns `handled=true` if it found a matching chain, `false` if not (legacy position).

---

### STEP 6: Frontend - Handle Composite Event (React/TypeScript)

**File**: `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx`

Add handler for `CHAIN_LIFECYCLE_UPDATE`:

```typescript
const handleChainLifecycleUpdate = useCallback((event: WSEvent) => {
    const data = event.data;
    if (!data) return;

    // 1. Update chain status to CLOSED
    setChains(prev => prev.map(chain => {
        if (chain.chainId !== data.chain?.chain_id) return chain;
        return {
            ...chain,
            status: 'completed',
            realizedPnl: data.chain.realized_pnl,
            closePrice: data.chain.close_price,
            closedAt: data.chain.closed_at,
            closeReason: data.chain.close_reason,
            positionState: {
                ...chain.positionState,
                status: 'CLOSED',
            },
            // Update order statuses
            slStatus: data.orders.sl_status,
            tpStatus: data.orders.tp_status,
            slFillPrice: data.orders.sl_fill_price,
            tpFillPrice: data.orders.tp_fill_price,
        };
    }));

    // 2. Position is automatically removed from positionStats
    //    because status is now 'completed' (filter excludes it)

    // 3. NO 500ms refetch needed - all data is in the event
}, []);

// Subscribe
useEffect(() => {
    const unsub = wsService.subscribe('CHAIN_LIFECYCLE_UPDATE', handleChainLifecycleUpdate);
    return () => unsub();
}, [handleChainLifecycleUpdate]);
```

---

### STEP 7: Frontend - Fix OrderTreeNode Display (React/TypeScript)

**File**: `web/src/components/TradeLifecycle/OrderTreeNode.tsx`

Updates for closed orders:

1. **SL/TP order status badges**: Show "FILLED" (green) or "CANCELED" (gray) based on `slStatus`/`tpStatus` from chain data

2. **Freeze timer on filled order**: When order status = "FILLED", show frozen timestamp (filled_at - placed_at duration)

3. **Freeze timer on canceled order**: When order status = "CANCELED", show frozen timestamp (canceled_at - placed_at)

4. **Footer PnL**: Wire `realized_pnl` from chain data to footer display (currently shows "n/a")

5. **Close price**: Already displayed (from `close_price` field) - verify it renders correctly

---

### STEP 8: Frontend - Fix CoinProfilerCard (React/TypeScript)

**File**: `web/src/components/CoinProfiler/CoinProfilerCard.tsx`

Handle composite event for capacity update:

```typescript
const handleChainLifecycleUpdate = useCallback((event: WSEvent) => {
    const profiler = event.data?.profiler;
    if (profiler) {
        // Instant capacity update - no 500ms refetch needed
        setCapacityUsed(profiler.capacity_used);
        setMaxCapacity(profiler.max_capacity);
        setScanningEnabled(profiler.scanning);
    }

    const position = event.data?.position;
    if (position?.status === 'CLOSED') {
        // Remove this symbol from position coins
        setCoins(prev => prev.map(c =>
            c.symbol === position.symbol
                ? { ...c, source: 'strategy' }
                : c
        ));
    }
}, []);
```

Remove the existing `handleChainClosed` with 500ms setTimeout refetch.

---

### STEP 9: Frontend - Fix CoinStageCard (React/TypeScript)

**File**: `web/src/components/EntryDecision/CoinStageCard.tsx`

Handle pattern reset from composite event:

```typescript
const handleChainLifecycleUpdate = useCallback((event: WSEvent) => {
    const pattern = event.data?.pattern;
    if (pattern && pattern.symbol === symbol) {
        // Instant reset to Step 1 - no separate broadcast needed
        setPatternStatus('watching');
        setCurrentStep(1);
    }
}, [symbol]);
```

---

### STEP 10: Frontend - Fix Empty POSITION_UPDATE Handling

**File**: `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx`

Fix the empty array bug (line ~636):

```typescript
// CHANGE FROM:
if (!Array.isArray(positions) || positions.length === 0) {
    return; // BUG: silently ignores "all positions closed"
}

// CHANGE TO:
if (!Array.isArray(positions)) return;

if (positions.length === 0) {
    // All positions closed - mark all active chains as completed
    setChains(prev => prev.map(chain => {
        if (chain.status === 'active' || chain.status === 'partial') {
            return {
                ...chain,
                status: 'completed',
                positionState: chain.positionState
                    ? { ...chain.positionState, status: 'CLOSED' }
                    : undefined,
            };
        }
        return chain;
    }));
    return;
}
```

---

### STEP 11: Remove Legacy Delays and Duplicate Broadcasts

**Backend removals**:
1. Remove `broadcastPositionClosure()` call from GinieAutopilot for chain-based positions (Coordinator handles)
2. Remove 10s background goroutine in `onChainClosedSymbol` callback
3. Remove duplicate `BroadcastChainClosed` from Callback #3 (Coordinator sends composite event)

**Frontend removals**:
1. Remove `setTimeout(500ms)` refetch in CoinProfilerCard `handleChainClosed`
2. Remove duplicate `handleChainClosed` listener (replaced by `handleChainLifecycleUpdate`)
3. Remove duplicate `handlePositionUpdate` for close scenarios (composite event handles it)

**Keep existing broadcasts as fallbacks** (for legacy GinieAutopilot positions):
- `POSITION_UPDATE` still works for non-chain positions
- `CHAIN_CLOSED` still works as safety net
- The composite `CHAIN_LIFECYCLE_UPDATE` is the primary for chain positions

---

### STEP 12: Wiring in UserAutopilotManager

**File**: `internal/autopilot/user_autopilot_manager.go`

Create and wire the Coordinator:

```go
coordinator := NewPositionLifecycleCoordinator(
    db,
    cacheService,
    chainEventWriter,
    realtimePatternMatcher,
    coinProfiler,
    broadcaster,
)

// Wire to FuturesController
futuresController.SetCoordinator(coordinator)

// Simplify ChainEventWriter callbacks:
// - Callback #1 (equity): KEEP (separate concern)
// - Callback #2 (pattern + profiler): REMOVE (Coordinator does this)
// - Callback #3 (broadcast): REMOVE (Coordinator does this)
```

---

## File Change Summary

| # | File | Action | Description |
|---|------|--------|-------------|
| 1 | `internal/autopilot/position_lifecycle_coordinator.go` | **NEW** | Core coordinator logic |
| 2 | `internal/events/bus.go` | EDIT | Add `CHAIN_LIFECYCLE_UPDATE` event type + broadcast function |
| 3 | `internal/api/websocket_user.go` | EDIT | Add WebSocket handler for composite event |
| 4 | `internal/database/repository_order_chains.go` | EDIT | Add counterpart order cancel methods |
| 5 | `internal/autopilot/futures_controller.go` | EDIT | Route SL/TP fills to Coordinator first |
| 6 | `internal/autopilot/user_autopilot_manager.go` | EDIT | Create + wire Coordinator, remove 10s goroutine |
| 7 | `internal/coinprofiler/profiler.go` | EDIT | Add `RebuildCapacity()` synchronous method |
| 8 | `internal/orders/chain_event_writer.go` | EDIT | Simplify callbacks (remove #2, #3 for chain positions) |
| 9 | `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx` | EDIT | Add composite event handler, fix empty array |
| 10 | `web/src/components/TradeLifecycle/OrderTreeNode.tsx` | EDIT | SL/TP status badges, timer freeze, footer PnL |
| 11 | `web/src/components/CoinProfiler/CoinProfilerCard.tsx` | EDIT | Handle composite event, remove 500ms refetch |
| 12 | `web/src/components/EntryDecision/CoinStageCard.tsx` | EDIT | Handle pattern reset from composite event |
| 13 | `migrations/0XX_counterpart_order_status.sql` | **NEW** | Add sl_canceled_at, tp_canceled_at columns |

---

## Testing Scenarios

1. **SL Hit**: Position closes via stop loss → verify all components update instantly
2. **TP Hit**: Position closes via take profit → verify counterpart shows CANCELED
3. **Legacy Position**: Direct Ginie trade closes → verify GinieAutopilot still handles
4. **Multiple Positions**: Close one of two → verify only correct position removed
5. **Hedge Mode**: LONG position closes → verify SHORT position unaffected
6. **Capacity**: Position closes → verify profiler shows 0/1, scanning resumes
7. **Pattern Reset**: Position closes → verify Step 4 → Step 1 instantly
8. **Order Tree Footer**: Closed chain → verify PnL shows in footer (not "n/a")

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Breaking legacy positions | GinieAutopilot remains untouched for non-chain positions |
| Missing chain lookup | Coordinator returns `handled=false` → falls back to Ginie |
| Frontend backward compat | Keep old event listeners as safety net |
| DB migration | Non-destructive (adds columns, doesn't modify existing) |
| Build failure | Go build check after each major step |

---

## Agent Team Structure

| Agent | Responsibility | Files |
|-------|---------------|-------|
| **Backend-Core** | Steps 1-5: Coordinator, DB, events, FuturesController rewire | Go files |
| **Backend-Wire** | Steps 6, 11-12: UserAutopilotManager wiring, cleanup, CoinProfiler | Go files |
| **Frontend** | Steps 7-10: All React/TypeScript changes | TSX files |
| **Migration** | Step 3 DB part: SQL migration file | SQL file |

All agents can work in parallel since backend and frontend are independent.
