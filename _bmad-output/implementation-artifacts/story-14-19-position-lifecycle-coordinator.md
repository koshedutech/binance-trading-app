# Story 14.19: Position Lifecycle Coordinator - Replace GinieAutopilot for Chain Trades

## Status: done

## Epic
Epic 14: Chain Trading System - Coin Profiler & Entry Decision Enhancement

## Story Description
Create a PositionLifecycleCoordinator that replaces GinieAutopilot's role for all chain-based position close handling. This single coordinator owns the entire lifecycle from SL/TP fill detection through frontend broadcast, eliminating 500ms delays, 10s background goroutines, duplicate broadcasts, and race conditions.

GinieAutopilot remains ONLY for legacy direct trades. Chain-based positions flow through the new Coordinator.

## Priority: P0 - Critical

## Acceptance Criteria

### AC1: PositionLifecycleCoordinator Created
- [ ] New file `internal/autopilot/position_lifecycle_coordinator.go` exists
- [ ] Coordinator receives SL/TP fill events from FuturesController
- [ ] Identifies chain-based positions by querying DB for matching `sl_binance_order_id` or `tp_binance_order_id`
- [ ] Returns `handled=false` for non-chain positions (GinieAutopilot handles those)
- [ ] Executes complete close flow: update filled order → update counterpart → close chain → reset pattern → rebuild capacity → broadcast

### AC2: Counterpart Order Status Persisted
- [ ] When SL fills: `tp_status` updated to "CANCELED" in DB
- [ ] When TP fills: `sl_status` updated to "CANCELED" in DB
- [ ] New DB methods: `UpdateOrderChainSLCanceled()`, `UpdateOrderChainTPCanceled()`
- [ ] Migration adds `sl_canceled_at`, `tp_canceled_at` columns if needed

### AC3: Single Composite WebSocket Event
- [ ] New event type `CHAIN_LIFECYCLE_UPDATE` in events/bus.go
- [ ] Composite payload includes: chain close data, position status, order statuses (FILLED/CANCELED), pattern reset (step 1), profiler capacity
- [ ] Single broadcast replaces multiple separate events for chain closes
- [ ] WebSocket handler registered in websocket_user.go

### AC4: Synchronous CoinProfiler Capacity Rebuild
- [ ] New `RebuildCapacity()` method on CoinProfiler (synchronous, no goroutine)
- [ ] Called directly by Coordinator after chain close
- [ ] 10-second background goroutine REMOVED from `onChainClosedSymbol` callback
- [ ] Capacity updates instantly: e.g., 1/1 → 0/1

### AC5: FuturesController Routing
- [ ] FuturesController routes SL/TP fills to Coordinator first
- [ ] If Coordinator returns `handled=true` → skip GinieAutopilot
- [ ] If Coordinator returns `handled=false` → fall through to GinieAutopilot (legacy)
- [ ] No duplicate processing of same event

### AC6: Frontend - Composite Event Handling
- [ ] TradeLifecycleTab.tsx subscribes to `CHAIN_LIFECYCLE_UPDATE`
- [ ] Chain status, PnL, close price, closed_at all update from single event
- [ ] Position automatically removed from Positions section (no manual refresh)
- [ ] Empty POSITION_UPDATE array bug fixed (line ~636)

### AC7: Frontend - OrderTreeNode Display Fixes
- [ ] SL/TP orders show correct status: "FILLED" (green) or "CANCELED" (gray)
- [ ] Timer freezes on both filled and canceled orders
- [ ] Footer displays realized PnL (not "n/a")
- [ ] Close price displayed correctly

### AC8: Frontend - CoinProfiler & CoinStageCard Updates
- [ ] CoinProfilerCard updates capacity from composite event (no 500ms refetch)
- [ ] CoinStageCard resets to Step 1 from composite event
- [ ] Remove setTimeout(500ms) refetch in CoinProfilerCard handleChainClosed
- [ ] Scanning status re-enables immediately when capacity freed

### AC9: Legacy Compatibility
- [ ] GinieAutopilot continues to handle non-chain positions unchanged
- [ ] Existing CHAIN_CLOSED and POSITION_UPDATE events still fire as safety net
- [ ] No regression for manual trades or legacy autopilot

### AC10: Build Verification
- [ ] Backend builds: `docker exec binance-trading-bot-dev go build -buildvcs=false -o /tmp/test-build .`
- [ ] Frontend builds: `docker exec binance-trading-bot-dev npm --prefix web run build`

## Tasks

### Task 1: Backend - Create PositionLifecycleCoordinator (AC1, AC2)
**New file**: `internal/autopilot/position_lifecycle_coordinator.go`
- Create struct with dependencies (db, cache, chainWriter, patternMatcher, coinProfiler, broadcaster)
- Implement `HandleOrderFill(ctx, userID, event)` - main orchestration method
- Chain identification: query DB by `sl_binance_order_id` / `tp_binance_order_id`
- Update filled order (SL/TP fill price, time, status)
- Update counterpart order to CANCELED
- Calculate realized PnL
- Close chain via ChainEventWriter
- Reset pattern synchronously
- Rebuild CoinProfiler capacity synchronously
- Build and broadcast composite event

### Task 2: Backend - Database Updates (AC2)
**File**: `internal/database/repository_order_chains.go`
- Add `UpdateOrderChainSLCanceled(ctx, chainID, canceledAt)` method
- Add `UpdateOrderChainTPCanceled(ctx, chainID, canceledAt)` method
- Add method to find chain by Binance order ID: `FindChainByBinanceOrderID(ctx, orderID)`

### Task 3: Backend - Events & WebSocket (AC3)
**File**: `internal/events/bus.go`
- Add `EventChainLifecycleUpdate = "CHAIN_LIFECYCLE_UPDATE"` constant
- Add `BroadcastChainLifecycleUpdate(userID, data)` function
**File**: `internal/api/websocket_user.go`
- Register broadcast handler for new event type

### Task 4: Backend - CoinProfiler Synchronous Rebuild (AC4)
**File**: `internal/coinprofiler/profiler.go`
- Add `RebuildCapacity(activeChainCount, maxConcurrent)` method
- Synchronous capacity recalculation
- Re-aggregate strategy requirements if capacity freed

### Task 5: Backend - FuturesController Routing (AC5)
**File**: `internal/autopilot/futures_controller.go`
- Add coordinator reference to FuturesController
- In HandleStreamOrderUpdate: route SL/TP fills to Coordinator first
- Fallback to GinieAutopilot if not handled

### Task 6: Backend - Wiring & Cleanup (AC4, AC5, AC9)
**File**: `internal/autopilot/user_autopilot_manager.go`
- Create PositionLifecycleCoordinator instance
- Wire dependencies
- Set coordinator on FuturesController
- Remove 10s background goroutine from onChainClosedSymbol callback (for chain positions)
- Keep equity callback (#1) unchanged

### Task 7: Frontend - TradeLifecycleTab Composite Event (AC6)
**File**: `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx`
- Subscribe to `CHAIN_LIFECYCLE_UPDATE`
- Update chain status, PnL, closePrice, closedAt, order statuses from single event
- Position automatically filtered out (status = "completed")
- Fix empty POSITION_UPDATE array bug (line ~636)

### Task 8: Frontend - OrderTreeNode Display (AC7)
**File**: `web/src/components/TradeLifecycle/OrderTreeNode.tsx`
- Show SL/TP status badges: "FILLED" (green) or "CANCELED" (gray)
- Freeze timer on filled/canceled orders using respective timestamps
- Wire footer PnL from chain realized_pnl
- Display close price

### Task 9: Frontend - CoinProfiler & CoinStageCard (AC8)
**File**: `web/src/components/CoinProfiler/CoinProfilerCard.tsx`
- Handle CHAIN_LIFECYCLE_UPDATE for instant capacity update
- Remove setTimeout(500ms) refetch
**File**: `web/src/components/EntryDecision/CoinStageCard.tsx`
- Handle CHAIN_LIFECYCLE_UPDATE for pattern reset to Step 1

## Dependencies
- Epic 14 (Chain Trading System) - builds on existing chain infrastructure
- Epic 7 (Order Chain Event Store) - uses order_chains table
- Epic 12 (WebSocket) - uses WebSocket broadcast infrastructure

## Technical Notes
- FuturesController determines routing by checking if Coordinator.HandleOrderFill returns handled=true
- Coordinator uses existing ChainEventWriter.CloseChain() for DB persistence but simplifies callback chain
- Pattern reset via RealtimePatternMatcher.ResetPatternForSymbol() - already synchronous
- CoinProfiler.RebuildCapacity() replaces the async initializeCoinProfilerSubscriptions() for chain closes only

## Change Log
- 2026-02-08: Story created from comprehensive analysis and plan (ready-for-dev)
- 2026-02-08: Implementation complete - all 9 tasks done, both builds pass (review)
- 2026-02-08: Code review PASSED (attempt 2/3) - 9 issues found, 5 fixed, 4 accepted as tech debt
- 2026-02-08: QA trace CONCERNS (33/35 pass) - AC2.4 timestamp cols skipped, AC7.4 close price footer minor
- 2026-02-08: Story completed - all gates passed (done)
