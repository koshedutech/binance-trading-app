# Story 8.3: Daily Summary Storage
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 8
**Story Points:** 5
**Priority:** P0

## User Story
As a system, I want daily settlement summaries persisted to the database so that historical data is available beyond Binance's 90-day limit and can be queried efficiently.

## Acceptance Criteria
- [ ] Create `daily_mode_summaries` table with all required columns
- [ ] Create `daily_position_snapshots` table for EOD position data
- [ ] Create `capital_samples` table for intraday capital tracking
- [ ] Settlement service writes one row per mode per day
- [ ] Upsert logic: Update if already exists for same date/mode
- [ ] Store settlement timestamp and user's timezone for reference
- [ ] Indexes created for fast queries by user/date/mode
- [ ] Historical queries return within 2 seconds (NFR-2)
- [ ] Data retained indefinitely (NFR-3)

## Technical Approach
Create database migration `20260106_create_daily_summaries.sql` that defines:
1. **daily_mode_summaries** - Main settlement data with mode breakdown
2. **daily_position_snapshots** - EOD open position snapshots
3. **capital_samples** - Intraday capital utilization samples

**Upsert Strategy:**
Use PostgreSQL `INSERT ... ON CONFLICT ... DO UPDATE` for idempotency:
```sql
INSERT INTO daily_mode_summaries (user_id, summary_date, mode, ...)
VALUES ($1, $2, $3, ...)
ON CONFLICT (user_id, summary_date, mode)
DO UPDATE SET
  realized_pnl = EXCLUDED.realized_pnl,
  ...
```

**API Endpoints:**
- `GET /api/user/daily-summaries` - User's own summaries with filters
- Query parameters: start_date, end_date, mode (optional)
- Response includes summaries array and totals object

**Indexes for Performance:**
- `idx_daily_summaries_user_date` - User + date DESC (most common query)
- `idx_daily_summaries_mode` - Mode + date DESC (mode comparison)
- `idx_daily_summaries_date_range` - Date + user (date range queries)
- `idx_daily_summaries_status` - Settlement status (failure monitoring)

## Dependencies
- **Blocked By:**
  - Story 8.0 (User Timezone Migration)
  - Story 8.2 (Daily P&L Aggregation - data source)
- **Blocks:**
  - Story 8.5 (Admin Dashboard - queries this data)
  - Story 8.6 (Historical Reports - queries this data)

## Files to Create/Modify
- `internal/database/migrations/20260106_create_daily_summaries.sql` - Database schema migration
- `internal/database/repository_daily_summaries.go` - Repository for daily summaries CRUD
- `internal/database/repository_position_snapshots.go` - Repository for position snapshots
- `internal/database/repository_capital_samples.go` - Repository for capital samples
- `internal/settlement/service.go` - Integration with settlement flow
- `internal/api/handlers_settlements.go` - API handlers for user summaries
- `internal/api/server.go` - Route registration
- `web/src/services/analyticsApi.ts` - Frontend API client

## Testing Requirements
- Unit tests:
  - Test repository save/update operations
  - Test upsert logic (insert new, update existing)
  - Test query by date range
  - Test query filtering by mode
- Integration tests:
  - Test full settlement storage flow
  - Test data retrieval via API endpoints
  - Test index usage with EXPLAIN ANALYZE
  - Test concurrent upserts (same user/date/mode)
- Performance tests:
  - Query 1-year data range completes <2 seconds
  - Verify indexes are used (check query plans)
  - Test with 10,000+ summary rows

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Migration script created and tested
- [ ] All three tables created successfully
- [ ] Indexes created and verified
- [ ] Repository functions implemented
- [ ] API endpoints functional
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Performance verified (<2s for 1-year queries)
- [ ] Upsert logic prevents duplicate rows
- [ ] Documentation updated (database schema, API endpoints)
- [ ] PO acceptance received
