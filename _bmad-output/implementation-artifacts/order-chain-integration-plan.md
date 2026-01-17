# Order Chain Display Integration Plan

## Created: 2026-01-17
## Related Epic: Epic 7 - Client Order ID & Trade Lifecycle Tracking

---

## Problem Statement

The Order Chain display in the Trade Lifecycle tab is missing critical elements:

1. **Entry Order disappears** after it fills and becomes a position
2. **Position state** not displayed (data exists in DB but not shown)
3. **TP/SL shown sequentially** instead of parallel (tree structure)
4. **Modification history** shown as separate section instead of nested inside each order

### Expected vs Current Display

**EXPECTED (Tree Structure):**
```
Chain: ULT-17JAN-00001
├── 📥 Entry Order (E) - FILLED ✅ @ $97,123
│   └── 📈 POSITION (active) - $97,455 (current price)
│       ├── 🎯 TP1 [expandable - 3 modifications]
│       │   └── v1 → v2 → v3 modification history
│       ├── 🎯 TP2
│       └── 🛡️ SL [expandable - 5 modifications]
│           └── v1 → v2 → v3 → v4 → v5 modification history
```

**CURRENT (Linear/Horizontal):**
```
Chain: ULT-17JAN-00001
[TP1] -- [TP2] -- [SL]   ← Entry missing, Position missing
                          ← Modification history in separate expandable section
```

---

## Root Cause Analysis

### Gap 1: Entry Order Missing

**Backend Issue:** `internal/api/handlers_futures.go:673`
```go
regularOrders, err := futuresClient.GetOpenOrders("")  // Only OPEN orders!
```
- Binance API `GetOpenOrders()` only returns orders with status NEW/PARTIALLY_FILLED
- Once Entry order fills (status=FILLED), it disappears from the response
- No historical order data retrieved

**Solution:** Create a new endpoint or modify existing to:
1. Return position_states data alongside orders
2. Position state contains entry order details (stored when entry fills)

### Gap 2: Position State Not Displayed

**Backend:** Position states exist in `position_states` table (Story 7.11)
- `/api/futures/position-states/:chainId` endpoint exists
- Data captured when entry fills via `OnEntryFilled()`

**Frontend:** Data never fetched or displayed
- `OrderChain` type has no `positionState` field
- `ChainCard.tsx` doesn't fetch or render position state

**Solution:**
1. Add `positionState` field to `OrderChain` interface
2. Fetch position states with order chains
3. Display position state under entry order

### Gap 3: Horizontal Linear Layout (Not Tree)

**Current Rendering:** `ChainCard.tsx:225-331`
```tsx
<div className="flex items-center gap-2 overflow-x-auto pb-2">
  {chain.entryOrder && (...)}    // Entry
  {chain.tpOrders.map(...)}      // TPs horizontal
  {chain.slOrder && (...)}       // SL horizontal
</div>
```

**Problem:** All orders rendered horizontally with `flex items-center`
- TP1, TP2, TP3, SL appear as sequential steps
- Reality: TP1, TP2, TP3, SL are all CHILDREN of Position (parallel)

**Solution:** Restructure to tree hierarchy:
1. Entry → Position (parent-child)
2. Position → [TP1, TP2, TP3, SL] (children, displayed vertically)
3. Each child expandable for modification history

### Gap 4: Modification History Placement

**Current:** Separate "Modification History" section at bottom (lines 344-411)
```tsx
{(chain.slOrder || chain.tpOrders.length > 0) && (
  <div className="space-y-3">
    <button onClick={() => setShowModificationHistory(...)}>
      Modification History
    </button>
    ...
  </div>
)}
```

**Expected:** Each TP/SL order should have its own expandable modification tree

**Solution:**
1. Integrate `ModificationTree` directly into each order node
2. Remove separate modification history section
3. Each order shows badge with modification count

### Gap 5: Backend Integration

**Missing:** Combined endpoint that returns:
- Open orders (from Binance)
- Position states (from DB)
- Modification summaries per order type

**Solution:** New or enhanced endpoint:
```go
GET /api/futures/order-chains
Response: {
  chains: [{
    chainId: "ULT-17JAN-00001",
    openOrders: [...],
    positionState: {...},  // From position_states table
    modificationCounts: {SL: 5, TP1: 3}  // Quick summary
  }]
}
```

---

## Implementation Plan

### Phase 1: Backend Enhancement (Story 7.14)

**File:** `internal/api/handlers_futures.go`

1. Create new handler `handleGetOrderChainsWithState`
2. Fetch open orders from Binance
3. Fetch position_states from DB for matching chain IDs
4. Merge data and return combined response

```go
type OrderChainResponse struct {
    ChainID           string            `json:"chain_id"`
    Orders            []interface{}     `json:"orders"`
    PositionState     *PositionState    `json:"position_state"`
    ModificationCount map[string]int    `json:"modification_counts"`
}
```

**Estimated Effort:** 2-3 hours

### Phase 2: Frontend Type Updates (Story 7.14)

**File:** `web/src/components/TradeLifecycle/types.ts`

1. Add `POSITION` to visualization types
2. Add `PositionState` interface
3. Add `positionState` to `OrderChain` interface
4. Add `modificationCounts` to `OrderChain`

```typescript
export interface PositionState {
  id: number;
  chainId: string;
  symbol: string;
  entrySide: 'BUY' | 'SELL';
  entryPrice: number;
  entryQuantity: number;
  entryValue: number;
  entryFilledAt: string;
  status: 'ACTIVE' | 'PARTIAL' | 'CLOSED';
  remainingQuantity: number;
  realizedPnl: number;
}

export interface OrderChain {
  // ... existing fields
  positionState?: PositionState;
  modificationCounts?: Record<string, number>;
}
```

**Estimated Effort:** 1 hour

### Phase 3: API Service Update (Story 7.14)

**File:** `web/src/services/futuresApi.ts`

1. Add `getOrderChainsWithState()` method
2. Update or replace `getAllOrders()` to include position states

**Estimated Effort:** 30 minutes

### Phase 4: ChainCard Tree Restructure (Story 7.15)

**File:** `web/src/components/TradeLifecycle/ChainCard.tsx`

1. Replace horizontal layout with vertical tree structure
2. Create `OrderNode` component for each order type
3. Integrate modification history into each order node
4. Add expand/collapse for individual orders

**New Structure:**
```tsx
<div className="tree-structure">
  <EntryNode order={chain.entryOrder} positionState={chain.positionState}>
    <PositionNode state={chain.positionState}>
      <OrderNode type="TP1" order={...} modifications={...} />
      <OrderNode type="TP2" order={...} modifications={...} />
      <OrderNode type="SL"  order={...} modifications={...} />
    </PositionNode>
  </EntryNode>
</div>
```

**Estimated Effort:** 4-5 hours

### Phase 5: Testing & Validation

1. Verify entry order displays after filling
2. Verify position state shows under entry
3. Verify tree structure renders correctly
4. Verify modification history nested in each order
5. Verify timezone consistency (GMT+7)

---

## Files to Modify

| Layer | File | Changes |
|-------|------|---------|
| Backend | `internal/api/handlers_futures.go` | New `handleGetOrderChainsWithState` handler |
| Backend | `internal/api/server.go` | Register new route |
| Frontend | `web/src/components/TradeLifecycle/types.ts` | Add PositionState, update OrderChain |
| Frontend | `web/src/services/futuresApi.ts` | Add `getOrderChainsWithState()` |
| Frontend | `web/src/components/TradeLifecycle/ChainCard.tsx` | Complete restructure to tree layout |
| Frontend | `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx` | Update to use new API |

---

## New Stories to Add to Epic 7

### Story 7.14: Order Chain Backend Integration

**Goal:** Create backend endpoint that returns orders + position states combined

**Tasks:**
1. Create `handleGetOrderChainsWithState` handler
2. Fetch position states for all chain IDs
3. Include modification counts per order type
4. Update frontend types and API service

**Acceptance Criteria:**
- [ ] New endpoint returns orders with position_state field
- [ ] Position state includes entry order details
- [ ] Modification counts included for each order type
- [ ] Existing order chain functionality unchanged

### Story 7.15: Order Chain Tree Structure UI

**Goal:** Restructure ChainCard from horizontal to tree hierarchy

**Tasks:**
1. Create tree visualization components
2. Show Entry → Position → [TP/SL] hierarchy
3. Integrate modification history into each order node
4. Add expand/collapse per order

**Acceptance Criteria:**
- [ ] Entry order visible even after filling
- [ ] Position state displayed under entry
- [ ] TP and SL shown as parallel children of position
- [ ] Each order expandable to show modification history
- [ ] Modification count badge on each order

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Binance API rate limits | Medium | Cache position states, batch requests |
| Performance with many chains | Medium | Pagination, lazy loading |
| Breaking existing UI | High | Feature flag for new tree view |
| Timezone inconsistency | Low | Use user timezone from settings |

---

## Success Metrics

1. Entry order visible for 100% of active chains
2. Position state displayed when entry fills
3. Tree structure renders correctly
4. Modification history accessible from each order
5. No regression in existing functionality
