# Story 7.5: Trade Lifecycle Tab UI
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 8
**Priority:** P1

## User Story
As a trader, I want a dedicated Trade Lifecycle tab that displays complete trade journeys with chain visualization so that I can see all stages of a trade (signal, entry, SL, TP, hedge, exit) in one place.

## Acceptance Criteria
- [ ] New tab: "Trade Lifecycle" alongside Orders, Order History, Trade Log, Positions
- [ ] Group orders by chain ID (chain base extraction)
- [ ] Collapsible chain cards showing all stages of trade
- [ ] Timeline visualization within each chain
- [ ] Filters: Mode, Date range, Symbol, Status
- [ ] Search functionality by chain ID
- [ ] Summary per chain: Total P&L, Duration, Fees
- [ ] Color coding: Entry (blue), SL (red), TP (green), Hedge (yellow)
- [ ] Sort chains by most recent activity (default)
- [ ] Display fallback chains with warning icon

## Technical Approach

1. **Component Hierarchy**:
   ```
   TradeLifecycleTab (main container)
   ├── ChainFilters (mode, date, symbol, status filters)
   ├── ChainSearch (search by chain ID)
   └── ChainCard[] (list of chains)
       ├── ChainTimeline (visual timeline)
       └── ChainSummary (P&L, duration, fees)
   ```

2. **Data Flow**:
   - Fetch orders from Backend API (Story 7.9)
   - Parse clientOrderId for each order (Story 7.4)
   - Group orders by chainId (base extraction)
   - Sort chains by most recent activity
   - Render collapsible chain cards

3. **Chain Card Design**:
   ```
   ┌─────────────────────────────────────────────────┐
   │ Chain: ULT-06JAN-00001        [Expand/Collapse] │
   │ Mode: ULTRA | Symbol: BTCUSDT | Direction: LONG │
   │ Start: 06-Jan 09:15 | End: 06-Jan 14:22 | 5h   │
   ├─────────────────────────────────────────────────┤
   │ 🔔 Signal      09:15:30  97,450  ✅  Conf: 85% │
   │ 📥 Entry (E)   09:15:32  97,455  ✅  Slip: +5  │
   │ 🛡️ SL         09:15:33  96,500  ✅  Active     │
   │ 🎯 TP1        09:15:33  98,000  ✅  Hit        │
   │ 🔄 Hedge (H)  11:30:45  97,200  ✅  Filled     │
   │ 📤 Exit       14:22:18  98,200  ✅  Closed     │
   ├─────────────────────────────────────────────────┤
   │ SUMMARY: P&L: +$245.00 (+2.1%) | Fees: $12.50  │
   └─────────────────────────────────────────────────┘
   ```

4. **Timeline Visualization**:
   - Horizontal timeline with stages as nodes
   - Color coding by stage type
   - Time labels on each stage
   - Active stages highlighted
   - Failed stages grayed out

5. **Filter Implementation**:
   - Mode dropdown: All, ULT, SCA, SCR, SWI, POS
   - Date range picker: Start date, End date
   - Symbol dropdown: All, BTCUSDT, ETHUSDT, etc.
   - Status filter: All, Active, Closed, Partial
   - Search input: Filter by chain ID substring

6. **P&L Calculation**:
   - Sum all fills in chain (entry, TP hits, final exit)
   - Calculate entry vs exit price differential
   - Include hedge P&L in total
   - Display percentage gain/loss
   - Sum commission fees across all orders

## Dependencies
- **Blocked By:**
  - Story 7.1: Client Order ID Generation
  - Story 7.3: Order Chain Tracking
  - Story 7.4: Parse Client Order ID
  - Story 7.9: Backend API for Trade Lifecycle (provides data)
- **Blocks:**
  - None (UI endpoint, no blocking dependencies)

## Files to Create/Modify

### Files to Create:
- `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx` - Main tab component
- `web/src/components/TradeLifecycle/ChainCard.tsx` - Individual chain display card
- `web/src/components/TradeLifecycle/ChainTimeline.tsx` - Timeline visualization component
- `web/src/components/TradeLifecycle/ChainFilters.tsx` - Filter controls component
- `web/src/components/TradeLifecycle/ChainSearch.tsx` - Search input component
- `web/src/components/TradeLifecycle/ChainSummary.tsx` - P&L summary display
- `web/src/services/tradeLifecycleApi.ts` - API client for lifecycle endpoints
- `web/src/types/tradeLifecycle.ts` - TypeScript types for chain data

### Files to Modify:
- `web/src/pages/TradingDashboard.tsx` - Add Trade Lifecycle tab to tab list
- `web/src/components/TabNavigation.tsx` - Add tab navigation item
- `web/src/styles/tradeLifecycle.css` - Styling for chain cards and timeline

## Testing Requirements

### Unit Tests:
- Test chain grouping logic (orders with same base grouped)
- Test P&L calculation (entry + TP + hedge + exit)
- Test filter logic (mode, date, symbol, status)
- Test search functionality (chain ID substring match)
- Test chain card expand/collapse
- Test timeline rendering with all stage types

### Integration Tests:
- Test API data fetching and display
- Test filter updates trigger data refresh
- Test real Binance order data rendering
- Test fallback chain display with warning

### UI/UX Tests:
- Test tab loads within 2 seconds
- Test collapsible cards smooth animation
- Test color coding matches specification
- Test responsive design (mobile, tablet, desktop)
- Test empty state (no chains found)
- Test error state (API failure)

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Documentation updated (component usage guide)
- [ ] PO acceptance received
- [ ] UI/UX review passed
- [ ] Responsive design verified
- [ ] Performance: Tab loads within 2 seconds
- [ ] All stage types displayed correctly (E, SL, TP, H)
