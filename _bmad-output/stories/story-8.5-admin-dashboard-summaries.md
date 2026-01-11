# Story 8.5: Admin Dashboard for Daily Summaries
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 8
**Story Points:** 5
**Priority:** P1

## User Story
As an admin, I want to view all users' daily performance summaries so that I can calculate profit-share for billing and monitor overall system performance.

## Acceptance Criteria
- [ ] Admin-only endpoint: `GET /api/admin/daily-summaries/all`
- [ ] List all users' daily summaries
- [ ] Filters: Date range, user, mode
- [ ] Sortable by P&L, trade count, win rate
- [ ] Export to CSV for billing
- [ ] Aggregate totals per user for profit-share calculation
- [ ] Display user email, trades, P&L, win rate, fees
- [ ] Paginated results (default 50 per page)

## Technical Approach
Create admin dashboard component that:
1. Fetches aggregated daily summaries across all users
2. Provides filtering by date range, user ID/email, mode
3. Sorts by any column (P&L, trade count, win rate)
4. Aggregates by user for billing period totals
5. Exports to CSV with all relevant columns

**API Endpoint:**
```
GET /api/admin/daily-summaries/all
  ?start_date=2026-01-01
  &end_date=2026-01-31
  &user_id=uuid (optional)
  &mode=ULT (optional)
  &sort_by=total_pnl (optional)
  &sort_order=desc (optional)
  &page=1
  &limit=50
```

**Response Structure:**
```json
{
  "summaries": [
    {
      "user_id": "uuid-123",
      "user_email": "user@example.com",
      "date": "2026-01-06",
      "mode": "ULT",
      "trade_count": 15,
      "win_rate": 66.67,
      "realized_pnl": 245.50,
      "unrealized_pnl": 50.00,
      "total_pnl": 295.50,
      "total_fees": 12.25
    }
  ],
  "totals": {
    "total_trades": 901,
    "total_pnl": 4330.00,
    "total_fees": 510.00,
    "avg_win_rate": 59.0
  },
  "pagination": {
    "page": 1,
    "limit": 50,
    "total_pages": 5,
    "total_count": 245
  }
}
```

**CSV Export Format:**
Columns: User Email, Date, Mode, Trades, Wins, Losses, Win Rate, Realized P&L, Unrealized P&L, Total P&L, Fees

**UI Features:**
- Date range picker (default: current month)
- User search/filter dropdown
- Mode filter (ALL, ULT, SCA, SWI, POS)
- Sortable table headers
- Export CSV button
- Click user row to see mode breakdown

## Dependencies
- **Blocked By:**
  - Story 8.3 (Daily Summary Storage - data source)
  - Story 8.4 (Open Position P&L Handling - accurate total P&L)
- **Blocks:** None

## Files to Create/Modify
- `internal/api/handlers_admin.go` - Admin daily summaries endpoint
- `internal/api/middleware.go` - Admin-only authorization middleware
- `internal/database/repository_daily_summaries.go` - Admin query methods
- `web/src/pages/AdminDailySettlements.tsx` - Admin dashboard component
- `web/src/services/adminApi.ts` - Admin API client
- `web/src/components/Analytics/DailySummaryTable.tsx` - Summary table component
- `web/src/utils/csvExport.ts` - CSV export utility
- `internal/api/server.go` - Route registration

## Testing Requirements
- Unit tests:
  - Test query with various filters
  - Test sorting by different columns
  - Test pagination logic
  - Test CSV generation
  - Test authorization (non-admin denied)
- Integration tests:
  - Test full API endpoint with database
  - Test filtering combinations
  - Test aggregation totals accuracy
  - Test CSV export contains all expected data
- UI tests:
  - Test table rendering with data
  - Test sorting functionality
  - Test filtering updates results
  - Test CSV download
  - Test pagination navigation

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Admin endpoint implemented with authorization
- [ ] Dashboard UI functional and responsive
- [ ] Filtering works for all parameters
- [ ] Sorting works for all columns
- [ ] CSV export generates correct data
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] UI tests passing
- [ ] Pagination working correctly
- [ ] Documentation updated (admin guide, API docs)
- [ ] PO acceptance received
