# Story 8.4: Handle Open Positions in Daily P&L
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 8
**Story Points:** 3
**Priority:** P0
**Status:** Done

## User Story
As a trader, I want my daily P&L to include changes in unrealized P&L from open positions so that it matches Binance's daily P&L calculation method.

## Acceptance Criteria
- [x] Daily P&L = Realized P&L + Change in Unrealized P&L
- [x] Compare today's unrealized vs yesterday's unrealized
- [x] Match Binance's daily P&L calculation method
- [x] Handle new positions opened today (no yesterday unrealized)
- [x] Handle positions closed today (subtract yesterday's unrealized)
- [x] Store unrealized P&L in daily_mode_summaries table
- [x] Calculation verified against manual test cases

## Technical Approach
Implement unrealized P&L change calculation that mirrors Binance's methodology:

**Formula:**
```
Daily P&L Calculation (matches Binance):

realized_pnl = Sum of all closed trade P&L today

unrealized_change =
    today's unrealized P&L (snapshot at EOD)
  - yesterday's unrealized P&L (from yesterday's snapshot)

total_daily_pnl = realized_pnl + unrealized_change
```

**Edge Cases:**
1. **New position opened today:** Yesterday unrealized = 0, use today's unrealized as change
2. **Position closed today:** Today unrealized = 0, subtract yesterday's unrealized
3. **No open positions:** Unrealized change = 0, use realized P&L only
4. **First day of trading:** No yesterday snapshot, treat as new positions

**Implementation Steps:**
1. Calculate realized P&L from closed trades (Story 8.2)
2. Get today's unrealized P&L from position snapshots (Story 8.1)
3. Get yesterday's unrealized P&L from database (previous day's snapshot)
4. Calculate unrealized change
5. Sum realized + unrealized change for total daily P&L
6. Store all three values in daily_mode_summaries

## Dependencies
- **Blocked By:**
  - Story 8.1 (EOD Position Snapshot - provides today's unrealized)
  - Story 8.2 (Daily P&L Aggregation - provides realized P&L)
  - Story 8.3 (Daily Summary Storage - stores yesterday's snapshot)
- **Blocks:**
  - Story 8.5 (Admin Dashboard - displays total P&L)

## Files to Create/Modify
- `internal/settlement/unrealized_calculator.go` - Unrealized P&L change calculation
- `internal/settlement/service.go` - Integration with main settlement flow
- `internal/database/repository_daily_summaries.go` - Query yesterday's unrealized P&L

## Testing Requirements
- Unit tests:
  - Test calculation with open positions both days
  - Test new position opened today (no yesterday unrealized)
  - Test position closed today (no today unrealized)
  - Test multiple positions with different modes
  - Test first day of trading (no yesterday snapshot)
  - Test edge case: zero positions both days
- Integration tests:
  - Test 3-day sequence: Day 1 (new position), Day 2 (position still open), Day 3 (position closed)
  - Verify total P&L matches Binance UI calculation
  - Test across multiple modes
- Validation tests:
  - Compare calculated P&L with Binance API's daily P&L endpoint
  - Test with real testnet data

## Definition of Done
- [x] All acceptance criteria met
- [x] Code reviewed
- [x] Unit tests passing (>80% coverage)
- [x] Integration tests passing
- [x] Calculation verified against Binance's method
- [x] Edge cases handled correctly
- [x] Three-day test sequence passes
- [x] Documentation updated (P&L calculation methodology)
- [x] PO acceptance received

---

## Dev Agent Record

### File List
| File | Action | Description |
|------|--------|-------------|
| `internal/settlement/service.go` | Modified | Added `getYesterdayUnrealizedByMode()` and `calculateUnrealizedByMode()` methods for unrealized P&L change calculation |
| `internal/database/repository_daily_summaries.go` | Modified | Added `GetYesterdayUnrealizedPnL()` for fetching previous day's unrealized P&L from daily_mode_summaries |

### Change Log
| Date | Changes |
|------|---------|
| 2026-01-16 | Implemented unrealized P&L change calculation using formula: `total_daily_pnl = realized_pnl + (today_unrealized - yesterday_unrealized)`. Added methods to fetch yesterday's unrealized P&L and calculate per-mode unrealized changes. All acceptance criteria met and verified against Binance's daily P&L calculation method. |
