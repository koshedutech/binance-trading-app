# Story 8.6: Historical Reports with Date Range Queries
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 9
**Story Points:** 5
**Priority:** P1

## User Story
As a trader, I want to query my historical performance over any date range with weekly, monthly, and yearly rollups so that I can analyze long-term trends and strategy effectiveness.

## Acceptance Criteria
- [ ] Query by date range (up to 1 year)
- [ ] Aggregate multiple days into period summary
- [ ] Weekly, monthly, yearly rollups
- [ ] Mode comparison over time
- [ ] Performance graphs data (P&L trend, win rate trend)
- [ ] Efficient queries with proper indexes (<2 seconds for 1-year range)
- [ ] User can only access their own data (admin can access all)

## Technical Approach
Implement three API endpoints for different reporting needs:

**1. Period Summary Endpoint:**
```
GET /api/user/performance/summary
  ?period=weekly|monthly|yearly
  &start_date=2026-01-01
  &end_date=2026-12-31
```
Returns aggregated data grouped by period with totals.

**2. Mode Comparison Endpoint:**
```
GET /api/user/performance/by-mode
  ?start_date=2026-01-01
  &end_date=2026-01-31
```
Returns side-by-side mode performance for the date range.

**3. Trend Analysis Endpoint:**
```
GET /api/user/performance/trend
  ?metric=pnl|win_rate|trade_count
  &granularity=daily|weekly|monthly
  &start_date=2026-01-01
  &end_date=2026-01-31
```
Returns time-series data for charting.

**Database Aggregation Strategy:**
Use PostgreSQL's `DATE_TRUNC` for efficient period grouping:
```sql
-- Weekly rollup example
SELECT
  DATE_TRUNC('week', summary_date) as week_start,
  SUM(trade_count) as total_trades,
  SUM(realized_pnl) as total_pnl,
  AVG(win_rate) as avg_win_rate
FROM daily_mode_summaries
WHERE user_id = $1
  AND summary_date BETWEEN $2 AND $3
  AND mode = 'ALL'
GROUP BY week_start
ORDER BY week_start DESC
```

**Frontend Components:**
- Performance summary cards (period totals)
- Mode comparison bar chart
- P&L trend line chart
- Win rate trend line chart
- Trade volume chart

## Dependencies
- **Blocked By:**
  - Story 8.3 (Daily Summary Storage - data source)
  - Story 8.4 (Open Position P&L Handling - accurate totals)
- **Blocks:** None

## Files to Create/Modify
- `internal/api/handlers_analytics.go` - Analytics endpoints
- `internal/database/repository_daily_summaries.go` - Aggregation query methods
- `web/src/pages/PerformanceReports.tsx` - Reports page component
- `web/src/services/analyticsApi.ts` - Analytics API client
- `web/src/components/Analytics/PnlTrendChart.tsx` - P&L trend chart
- `web/src/components/Analytics/ModePerformanceChart.tsx` - Mode comparison chart
- `web/src/components/Analytics/PerformanceSummaryCards.tsx` - Summary cards
- `internal/api/server.go` - Route registration

## Testing Requirements
- Unit tests:
  - Test period rollup calculations (weekly, monthly, yearly)
  - Test mode comparison aggregation
  - Test trend data generation
  - Test date range validation
  - Test authorization (user can only see their data)
- Integration tests:
  - Test all three endpoints with real data
  - Test 1-year date range completes <2 seconds
  - Test aggregation accuracy against manual calculations
  - Test edge cases (single day, leap year, DST transitions)
- Performance tests:
  - Query 365 daily summaries, aggregate to monthly (should use index)
  - Verify EXPLAIN ANALYZE shows index usage
  - Test with 10,000+ daily summary rows
- UI tests:
  - Test chart rendering with trend data
  - Test period selector changes aggregation
  - Test mode filter updates comparison

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Three API endpoints implemented
- [ ] Aggregation queries optimized with indexes
- [ ] Frontend charts render correctly
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Performance verified (<2s for 1-year queries)
- [ ] UI tests passing
- [ ] Period rollups accurate (weekly, monthly, yearly)
- [ ] Mode comparison functional
- [ ] Trend charts display correctly
- [ ] Documentation updated (API docs, user guide)
- [ ] PO acceptance received
