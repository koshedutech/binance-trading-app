# Story 7.15: Order Chain Tree Structure UI

## Story

**As a** trader viewing order chains in the Trade Lifecycle tab
**I want** to see a hierarchical tree display showing Entry -> Position -> TP/SL with modification history
**So that** I can understand the complete lifecycle of my trades including entry order details even after they fill

## Status

**Status:** done
**Story Points:** 5
**Priority:** High
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Completed:** 2026-01-17

## Acceptance Criteria

- [x] AC1: Entry order visible even after filling (reconstructed from position_state)
- [x] AC2: Position state displayed as child of entry with status indicator
- [x] AC3: TP1, TP2, TP3, SL displayed as children of position (parallel, not sequential)
- [x] AC4: Each TP/SL order expandable to show modification history
- [x] AC5: Modification count badge on each order (e.g., "SL (3)")
- [x] AC6: Tree connectors (|-- or box-drawing chars) for visual hierarchy
- [x] AC7: Collapsible sub-trees for cleaner display
- [x] AC8: Timezone-aware timestamps using user's timezone setting

## Tasks/Subtasks

### Task 1: Add PositionState types to types.ts
- [x] 1.1: Add PositionState interface matching backend response
- [x] 1.2: Add OrderChainWithState interface
- [x] 1.3: Add POSITION type to ORDER_TYPE_CONFIG

### Task 2: Create OrderTreeNode component
- [x] 2.1: Create `web/src/components/TradeLifecycle/OrderTreeNode.tsx`
- [x] 2.2: Implement tree connector rendering (|-- or box chars)
- [x] 2.3: Add modification count badge display
- [x] 2.4: Integrate with ModificationTree for expandable history

### Task 3: Update ChainCard with tree layout
- [x] 3.1: Replace horizontal layout with hierarchical tree structure
- [x] 3.2: Build entry order from positionState when entryOrder is null
- [x] 3.3: Display position state as child of entry
- [x] 3.4: Show TP/SL orders as parallel children of position
- [x] 3.5: Add collapsible sub-tree functionality

### Task 4: Update TradeLifecycleTab to use new API
- [x] 4.1: Switch from `getAllOrders()` to `getOrderChainsWithState()`
- [x] 4.2: Map OrderChainWithState to existing OrderChain interface
- [x] 4.3: Preserve existing filter and stats functionality

### Task 5: Testing and verification
- [x] 5.1: Verify tree layout renders correctly with entry -> position -> TP/SL
- [x] 5.2: Verify entry order shows even after filling
- [x] 5.3: Verify modification badges show correct counts
- [x] 5.4: Verify build succeeds

## Dev Notes

### Dependencies
- Story 7.14 (completed): Backend API `/api/futures/order-chains` returns `OrderChainWithState`
- Story 7.13 (completed): ModificationTree component exists for modification history display
- Story 7.11 (completed): Position state tracking backend

### Backend API (Story 7.14)
```
GET /api/futures/order-chains
Response:
{
  "chains": [
    {
      "chain_id": "ULT-17JAN-00001",
      "mode_code": "ULT",
      "symbol": "BTCUSDT",
      "position_side": "LONG",
      "orders": [...],
      "position_state": {
        "id": 1,
        "chain_id": "ULT-17JAN-00001",
        "entry_price": 97450.00,
        "entry_quantity": 0.01,
        "status": "ACTIVE",
        ...
      },
      "modification_counts": {"SL": 3, "TP1": 2},
      "status": "active"
    }
  ]
}
```

### Key Implementation Notes
1. Entry order may be null when filled - reconstruct from position_state
2. Use CSS flexbox/grid for tree layout with proper indentation
3. Tree connectors use box-drawing characters or CSS borders
4. ModificationTree component from Story 7.13 handles expansion

### Files to Modify
| File | Changes |
|------|---------|
| `web/src/components/TradeLifecycle/types.ts` | Add PositionState, POSITION type |
| `web/src/components/TradeLifecycle/OrderTreeNode.tsx` | New component |
| `web/src/components/TradeLifecycle/ChainCard.tsx` | Tree layout |
| `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx` | Use new API |

---

## Dev Agent Record

### Implementation Plan
1. Add types to types.ts (PositionState, POSITION order type)
2. Create OrderTreeNode for individual nodes in tree
3. Refactor ChainCard to use tree structure instead of horizontal
4. Update TradeLifecycleTab to use getOrderChainsWithState API

### Debug Log
- 2026-01-17: Starting implementation
- 2026-01-17: Completed all tasks, build verified

### Completion Notes
Implementation completed successfully. Key changes:

1. **types.ts**: Added `PositionState` interface and `POSITION` order type to `ORDER_TYPE_CONFIG`
2. **OrderTreeNode.tsx**: New component with:
   - Tree connector rendering using box-drawing characters
   - Modification count badges with edit icon
   - Status indicators for different order states
   - Expandable modification history integration
   - Helper function `buildEntryFromPositionState()` to reconstruct entry from position state
3. **ChainCard.tsx**: Complete refactor to tree layout:
   - Default tree view with switch to legacy list view
   - Entry -> Position -> TP/SL hierarchy
   - DCA, Hedge, Rebuy orders in separate branches
   - Position state indicator in header
   - Total modification count badge
4. **TradeLifecycleTab.tsx**: Updated to use new API:
   - `getOrderChainsWithState()` instead of `getAllOrders()`
   - Mapping functions for API response conversion
   - Fallback to old API if new endpoint fails

---

## File List

### New Files
- `web/src/components/TradeLifecycle/OrderTreeNode.tsx` (271 lines)

### Modified Files
- `web/src/components/TradeLifecycle/types.ts` (added PositionState interface, POSITION type)
- `web/src/components/TradeLifecycle/ChainCard.tsx` (complete refactor with tree and legacy views)
- `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx` (new API integration with mapping)

---

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-17 | Story created | Dev Agent |
| 2026-01-17 | Implementation completed | Dev Agent |
