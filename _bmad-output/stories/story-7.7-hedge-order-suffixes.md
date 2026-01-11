# Story 7.7: Hedge Order Suffixes
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 3
**Priority:** P1

## User Story
As a trader using hedge mode, I want hedge orders to have distinct suffixes (-H, -HSL, -HTP) so that I can distinguish hedge orders from main position orders in the trade lifecycle.

## Acceptance Criteria
- [ ] Suffix -H for hedge entry order
- [ ] Suffix -HSL for hedge stop loss
- [ ] Suffix -HTP for hedge take profit
- [ ] Hedge orders share same chain base as original trade
- [ ] Trade Lifecycle displays hedge as distinct stage with yellow color
- [ ] P&L calculation includes hedge P&L in chain total
- [ ] Parser recognizes hedge suffixes (H, HSL, HTP)
- [ ] Hedge orders visually grouped with parent chain in UI

## Technical Approach

1. **Hedge Order Type Constants**:
   ```go
   const (
       OrderTypeHedge   OrderType = "H"
       OrderTypeHedgeSL OrderType = "HSL"
       OrderTypeHedgeTP OrderType = "HTP"
   )
   ```

2. **Hedge Order Flow**:
   ```
   Original Chain: ULT-06JAN-00001
   ├── ULT-06JAN-00001-E    → LONG Entry @ 97,450
   ├── ULT-06JAN-00001-SL   → Stop Loss @ 96,500
   ├── ULT-06JAN-00001-TP1  → TP1 @ 98,000 (HIT)
   [Price reverses, hedge triggered]
   ├── ULT-06JAN-00001-H    → HEDGE SHORT Entry @ 97,200
   ├── ULT-06JAN-00001-HSL  → Hedge SL @ 97,800
   └── ULT-06JAN-00001-HTP  → Hedge TP @ 96,500 (HIT)
   ```

3. **Hedge Detection Logic**:
   - When hedge activation detected (Epic 4 logic)
   - Reuse chain base from protected position
   - Generate hedge entry ID: `{chainBase}-H`
   - Generate hedge SL ID: `{chainBase}-HSL`
   - Generate hedge TP ID: `{chainBase}-HTP`

4. **P&L Calculation with Hedges**:
   ```
   Timeline:
   09:15 → Entry LONG @ 97,450
   10:30 → TP1 Hit @ 98,000 (+$100)
   11:30 → Hedge SHORT @ 97,200 (price reversing)
   12:45 → Hedge TP Hit @ 96,500 (+$80)
   13:00 → Remaining position closed @ 98,200 (+$65)

   TOTAL P&L: +$245 (main position + hedge combined)
   ```

5. **UI Color Coding**:
   - Entry orders: Blue (#3B82F6)
   - Stop Loss: Red (#EF4444)
   - Take Profit: Green (#10B981)
   - Hedge orders: Yellow/Amber (#F59E0B)
   - Hedge SL/TP: Orange variants (#FB923C, #FCD34D)

6. **Chain Timeline Display**:
   ```
   🔔 Signal      09:15:30  97,450  ✅
   📥 Entry (E)   09:15:32  97,455  ✅ (Blue)
   🛡️ SL         09:15:33  96,500  ✅ (Red)
   🎯 TP1        09:15:33  98,000  ✅ (Green)
   🔄 Hedge (H)  11:30:45  97,200  ✅ (Yellow)
   🛡️ HSL        11:30:46  97,800  ✅ (Orange)
   🎯 HTP        12:45:12  96,500  ✅ (Green-Yellow)
   📤 Exit       13:00:00  98,200  ✅
   ```

## Dependencies
- **Blocked By:**
  - Story 7.1: Client Order ID Generation (adds hedge constants)
  - Story 7.3: Order Chain Tracking (chain reuse logic)
  - Story 7.4: Parse Client Order ID (recognize hedge suffixes)
- **Blocks:**
  - Story 7.5: Trade Lifecycle Tab UI (hedge display)
  - Story 7.9: Backend API (hedge P&L calculation)

## Files to Create/Modify

### Files to Create:
- None (extends existing files)

### Files to Modify:
- `internal/orders/types.go` - Add hedge order type constants (H, HSL, HTP)
- `internal/orders/client_order_id.go` - Support hedge suffix generation
- `internal/orders/client_order_id_parser.go` - Recognize hedge suffixes in parsing
- `internal/autopilot/hedge_controller.go` - Use hedge suffixes when placing hedge orders
- `web/src/components/TradeLifecycle/ChainTimeline.tsx` - Yellow color for hedge stages
- `web/src/components/TradeLifecycle/ChainSummary.tsx` - Include hedge P&L in total
- `web/src/styles/tradeLifecycle.css` - Hedge color definitions

## Testing Requirements

### Unit Tests:
- Test hedge suffix generation (H, HSL, HTP)
- Test hedge orders reuse chain base from parent
- Test parser recognizes all hedge suffixes
- Test hedge P&L calculation included in chain total
- Test color coding for hedge orders
- Test chain timeline displays hedge stage correctly

### Integration Tests:
- Test complete hedge flow: Entry → TP1 Hit → Hedge Trigger → Hedge Orders
- Test hedge orders query by chain base ID
- Test hedge orders display in Trade Lifecycle tab
- Test P&L calculation with mixed orders (TP + Hedge)
- Test hedge orders link to parent position

### UI Tests:
- Test hedge orders appear in chain card with yellow color
- Test hedge timeline stage shows distinct icon
- Test hedge P&L contributes to chain summary
- Test hedge orders collapsible/expandable

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Documentation updated (hedge suffix specification)
- [ ] PO acceptance received
- [ ] All hedge suffixes (H, HSL, HTP) tested
- [ ] Hedge P&L calculation verified
- [ ] UI color coding matches specification (yellow)
- [ ] Hedge orders visually distinct in Trade Lifecycle tab
