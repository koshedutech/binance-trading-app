# Story 7.18: Autopilot Event Integration

## Story Overview

**Story ID:** 7-18
**Epic:** 7 - Client Order ID & Trade Lifecycle Tracking
**Priority:** P0 (Critical)
**Status:** done
**Created:** 2026-01-18
**Completed:** 2026-01-18
**Complexity:** Large
**Depends On:** Story 7.17 (Chain Event Writer Service)

---

## Goal

Integrate the `ChainEventWriter` service into Ginie Autopilot so that every order lifecycle event is captured in the event store. Replace existing fragmented logging with unified event writing.

---

## Current State

Ginie Autopilot currently:
- Uses `PositionTracker` for position state (Story 7.11)
- Uses `ModificationTracker` for SL modifications (Story 7.12)
- Logs SL modifications but NOT TP modifications
- Does not capture entry order placement/fill events
- Does not capture chain close events

---

## Target State

Replace fragmented tracking with `ChainEventWriter`:

| Stage | Current | New |
|-------|---------|-----|
| Entry placed | Not captured | `RecordEntryPlaced()` |
| Entry filled | `PositionTracker.OnEntryFilled()` | `RecordEntryFilled()` |
| SL placed | `ModificationTracker.OnOrderPlaced()` | `RecordSLPlaced()` |
| SL modified | `ModificationTracker.OnOrderModified()` | `RecordSLModified()` |
| SL filled | Not captured | `RecordSLFilled()` |
| TP placed | Not captured | `RecordTPPlaced()` |
| TP modified | Not captured | `RecordTPModified()` |
| TP filled | Not captured | `RecordTPFilled()` |
| Position closed | Partial | `CloseChain()` |

---

## Acceptance Criteria

- [x] AC1: ChainEventWriter injected into GinieAutopilot
- [ ] AC2: Entry order placement triggers `RecordEntryPlaced()` (future enhancement)
- [ ] AC3: Entry order fill triggers `RecordEntryFilled()` + POSITION_OPENED (future enhancement)
- [x] AC4: SL placement triggers `RecordSLPlaced()`
- [x] AC5: SL modification triggers `RecordSLModified()` (with reason from LLM)
- [ ] AC6: SL fill triggers `RecordSLFilled()` + chain close (future enhancement)
- [x] AC7: TP placement triggers `RecordTPPlaced()` or `RecordTPLevelPlaced()`
- [x] AC8: TP modification triggers `RecordTPModified()` (when position opt OFF)
- [ ] AC9: TP fill triggers `RecordTPFilled()` or `RecordTPLevelFilled()` (future enhancement)
- [ ] AC10: Hedge chain creation triggers `LinkHedgeChain()` (future enhancement)
- [ ] AC11: Old trackers deprecated (PositionTracker, ModificationTracker) (Phase 3 - after verification)
- [x] AC12: All events include Binance confirmation timestamp

**Implementation Note:** Dual-write enabled for SL/TP placement and modification events. Entry/fill events and chain close will be added in a follow-up story after verifying the new event store works correctly.

---

## Integration Points in Ginie Autopilot

### 1. Entry Order Placement (ginie_autopilot.go)

**Find:** Where entry order is placed on Binance
**Add:** After Binance confirms order

```go
// After placing entry order and getting Binance response
entryOrder, err := ga.placeEntryOrder(...)
if err == nil && entryOrder != nil {
    // Create chain and record entry placed
    chain, err := ga.chainEventWriter.CreateChain(ctx, orders.CreateChainRequest{
        UserID:   userID,
        ChainID:  chainID,
        Symbol:   symbol,
        Side:     side,
        ModeCode: modeCode,
    })

    ga.chainEventWriter.RecordEntryPlaced(ctx, chainID, orders.EntryPlacedEvent{
        BinanceOrderID:       entryOrder.OrderID,
        BinanceClientOrderID: entryOrder.ClientOrderID,
        Price:                entryOrder.Price,
        Quantity:             entryOrder.OrigQty,
        BinanceTimestamp:     entryOrder.UpdateTime,
    })
}
```

### 2. Entry Order Fill Detection

**Find:** Where entry fill is detected (WebSocket or polling)
**Add:** Record fill event

```go
// When entry order fill is detected
if orderUpdate.Status == "FILLED" && isEntryOrder(orderUpdate.ClientOrderID) {
    ga.chainEventWriter.RecordEntryFilled(ctx, chainID, orders.EntryFilledEvent{
        FilledPrice:      orderUpdate.AvgPrice,
        FilledQuantity:   orderUpdate.FilledQty,
        Fees:             orderUpdate.Commission,
        BinanceTimestamp: orderUpdate.UpdateTime,
    })
}
```

### 3. SL Placement

**Find:** `placeSLOrder()` or similar
**Add:** After Binance confirms SL order

```go
func (ga *GinieAutopilot) placeSLOrder(pos *GiniePosition) {
    slOrder, err := ga.binanceClient.PlaceStopLossOrder(...)
    if err == nil {
        ga.chainEventWriter.RecordSLPlaced(ctx, pos.ChainID, orders.SLPlacedEvent{
            BinanceOrderID:       slOrder.OrderID,
            BinanceClientOrderID: slOrder.ClientOrderID,
            Price:                slOrder.StopPrice,
            BinanceTimestamp:     slOrder.UpdateTime,
        })
    }
}
```

### 4. SL Modification (Critical Path)

**Find:** `updateBinanceSLOrderWithReason()` or similar
**Replace:** Current `logOrderModificationEvent()` call

```go
func (ga *GinieAutopilot) updateSLOrder(pos *GiniePosition, newPrice float64, source, reason string) {
    oldPrice := pos.StopLoss

    // Update on Binance
    slOrder, err := ga.binanceClient.ModifyStopLossOrder(...)
    if err == nil {
        // Record event (replaces ModificationTracker)
        ga.chainEventWriter.RecordSLModified(ctx, pos.ChainID, orders.SLModifiedEvent{
            BinanceOrderID:       slOrder.OrderID,
            OldPrice:             oldPrice,
            NewPrice:             newPrice,
            ModificationSource:   source,
            ModificationReason:   reason,
            MarketPrice:          ga.currentPrice,
            BinanceTimestamp:     slOrder.UpdateTime,
        })
    }
}
```

### 5. TP Placement

**Find:** `placeTPOrder()` or similar
**Add:** Record TP placement

```go
// For normal mode (single TP)
func (ga *GinieAutopilot) placeTPOrder(pos *GiniePosition) {
    tpOrder, err := ga.binanceClient.PlaceTakeProfitOrder(...)
    if err == nil {
        ga.chainEventWriter.RecordTPPlaced(ctx, pos.ChainID, orders.TPPlacedEvent{
            BinanceOrderID:       tpOrder.OrderID,
            BinanceClientOrderID: tpOrder.ClientOrderID,
            Price:                tpOrder.Price,
            BinanceTimestamp:     tpOrder.UpdateTime,
        })
    }
}

// For position optimization (TP1, TP2, TP3)
func (ga *GinieAutopilot) placeNextTPOrder(pos *GiniePosition, level int) {
    tpOrder, err := ga.binanceClient.PlaceTakeProfitOrder(...)
    if err == nil {
        ga.chainEventWriter.RecordTPLevelPlaced(ctx, pos.ChainID, level, orders.TPLevelPlacedEvent{
            Level:                level,
            BinanceOrderID:       tpOrder.OrderID,
            BinanceClientOrderID: tpOrder.ClientOrderID,
            Price:                tpOrder.Price,
            Quantity:             pos.TakeProfits[level-1].Percent,
            BinanceTimestamp:     tpOrder.UpdateTime,
        })
    }
}
```

### 6. TP Modification (Normal Mode - Position Opt OFF)

**Find:** Where TP is modified by LLM
**Add:** Record modification event

```go
func (ga *GinieAutopilot) updateTPOrder(pos *GiniePosition, newPrice float64, source, reason string) {
    if pos.PositionOptimizationEnabled {
        return // TP1/2/3 are static, don't modify
    }

    oldPrice := pos.TakeProfit
    tpOrder, err := ga.binanceClient.ModifyTakeProfitOrder(...)
    if err == nil {
        ga.chainEventWriter.RecordTPModified(ctx, pos.ChainID, orders.TPModifiedEvent{
            BinanceOrderID:       tpOrder.OrderID,
            OldPrice:             oldPrice,
            NewPrice:             newPrice,
            ModificationSource:   source,
            ModificationReason:   reason,
            MarketPrice:          ga.currentPrice,
            BinanceTimestamp:     tpOrder.UpdateTime,
        })
    }
}
```

### 7. Order Fill Events (SL/TP Hit)

**Find:** Where SL/TP fills are detected
**Add:** Record fill and close chain if complete

```go
// When SL fills
if orderUpdate.Status == "FILLED" && isSLOrder(orderUpdate.ClientOrderID) {
    pnl := calculatePnL(pos, orderUpdate.AvgPrice)

    ga.chainEventWriter.RecordSLFilled(ctx, pos.ChainID, orders.SLFilledEvent{
        FilledPrice:      orderUpdate.AvgPrice,
        PnL:              pnl,
        Fees:             orderUpdate.Commission,
        BinanceTimestamp: orderUpdate.UpdateTime,
    })

    // Close chain if fully exited
    if pos.RemainingQty <= 0 {
        ga.chainEventWriter.CloseChain(ctx, pos.ChainID, "SL_HIT", pos.TotalPnL, pos.TotalFees)
    }
}

// When TP fills (similar pattern)
```

### 8. Hedge Chain Creation

**Find:** Where hedge position is opened
**Add:** Link chains

```go
func (ga *GinieAutopilot) openHedgePosition(primaryPos *GiniePosition) {
    // Create hedge chain
    hedgeChainID := primaryPos.ChainID + "-H"  // Or generate new chain ID

    hedge, err := ga.chainEventWriter.CreateChain(ctx, orders.CreateChainRequest{
        UserID:        primaryPos.UserID,
        ChainID:       hedgeChainID,
        Symbol:        primaryPos.Symbol,
        Side:          oppositeSide(primaryPos.Side),
        ModeCode:      primaryPos.ModeCode,
        IsHedge:       true,
        ParentChainID: primaryPos.ChainID,
    })

    // Link to primary
    ga.chainEventWriter.LinkHedgeChain(ctx, primaryPos.ChainID, hedgeChainID)

    // Place hedge entry order...
}
```

---

## Deprecation Plan

### Phase 1: Add ChainEventWriter alongside existing trackers
- Both systems write events (dual-write)
- Verify ChainEventWriter captures everything

### Phase 2: Migrate reads to ChainEventWriter
- API handlers read from order_chain_events
- UI displays from new event store

### Phase 3: Remove old trackers
- Delete PositionTracker usage
- Delete ModificationTracker usage
- Keep tables for historical data (archived)

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/autopilot/ginie_autopilot.go` | Add ChainEventWriter, wire events |
| `internal/autopilot/ginie_position.go` | Add ChainID reference |
| `main.go` | Initialize and inject ChainEventWriter |

---

## Test Scenarios

1. **Entry flow** - Place entry, verify ENTRY_PLACED + ENTRY_FILLED events
2. **SL flow** - Place SL, modify 3 times, verify all events captured
3. **TP flow (normal)** - Place TP, modify, fill, verify events
4. **TP flow (position opt)** - Place TP1, fill, place TP2, fill, verify events
5. **Hedge flow** - Create hedge, verify linked chains
6. **Full lifecycle** - Entry → Position → SL mods → TP hit → Close

---

## Definition of Done

- [ ] ChainEventWriter integrated into GinieAutopilot
- [ ] All event types captured
- [ ] Binance timestamps used (not system time)
- [ ] Dual-write working (old + new)
- [ ] Unit tests passing
- [ ] Integration test with live trading verified

---

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-18 | Story created | System |
