# Story 13.1: PNL Summary Caching & Historical Navigation

## Status: done

## Story

- **As a** trader
- **I want** the 7-day PNL calendar to load instantly by caching historical data and only fetching today's data live
- **So that** I can quickly view my trading performance without waiting for 7 Binance API calls every time

## Description

Currently, the PNL Summary card makes 7 Binance API calls (one per day) every time it loads, with a 120-second timeout. This is inefficient because:
1. Historical days (yesterday and before) never change - their PNL is final
2. Only TODAY's PNL is dynamic and needs live fetching

This story implements:
1. **Database caching** of daily PNL summaries
2. **Smart fetching** - only TODAY from Binance, historical from DB
3. **Extended date range navigation** - browse beyond 7 days
4. **Enhanced UI** - date numbers, "TODAY" label, click-to-view details

## Acceptance Criteria

### AC1: Database Storage for Daily PNL Summaries
- [ ] New table `user_daily_pnl_summaries` stores daily PNL data per user
- [ ] Schema includes: user_id, date, pnl, commission, funding, net_pnl, trade_count, fetched_from_binance_at
- [ ] Unique constraint on (user_id, date) - one record per user per day
- [ ] Records are immutable once created (historical data doesn't change)

### AC2: Smart Data Fetching Logic
- [ ] When fetching PNL data:
  - For **TODAY**: Always fetch live from Binance API
  - For **past days**: Check database first, only fetch from Binance if not exists
- [ ] After fetching from Binance, store the result in database
- [ ] API response time reduced from ~7 seconds to <1 second for cached data

### AC3: Backend API Changes
- [ ] Modify `/api/futures/pnl-summary` to use database-first pattern
- [ ] Add new endpoint `/api/futures/pnl-history` for extended date ranges
- [ ] Query parameters: `start_date`, `end_date`, `period` (day/week/month/year)
- [ ] Returns array of daily PNL records within range

### AC4: UI - Date Display on Cards
- [ ] Each day card shows the date number prominently (e.g., "18", "17", "16")
- [ ] Day name shown below date (Mon, Tue, Wed...)
- [ ] Only current day shows "TODAY" label instead of date number
- [ ] Visual indicator distinguishes cached (from DB) vs live data

### AC5: UI - Date Range Filters
- [ ] Add filter selector above PNL cards: Day | Week | Month | Year | Custom Range
- [ ] Default view: Last 7 days (current behavior)
- [ ] Week: Shows 7 days (current week or selected week)
- [ ] Month: Shows all days in selected month
- [ ] Year: Shows monthly summaries for selected year
- [ ] Custom Range: Date picker for start/end dates

### AC6: UI - Navigation Controls
- [ ] Navigation buttons: First (|<), Previous (<), Next (>), Last (>|)
- [ ] Card container size remains fixed
- [ ] Horizontal navigation through date range
- [ ] Page indicator shows current position (e.g., "Jan 12-18 of Jan 2026")

### AC7: UI - Detail View on Click
- [ ] Clicking any day card shows detailed breakdown
- [ ] Detail panel shows: Gross PNL, Commission, Funding, Net PNL, Trade Count
- [ ] Selected card is visually highlighted
- [ ] Detail panel appears on left side of cards

### AC8: Performance Requirements
- [ ] Initial load (7-day cached): < 500ms
- [ ] Live TODAY fetch: < 3 seconds (single API call)
- [ ] Historical range query (30 days): < 1 second
- [ ] No regression in current functionality

## Technical Notes

### Database Migration
```sql
CREATE TABLE user_daily_pnl_summaries (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    date DATE NOT NULL,
    pnl DECIMAL(20,8) NOT NULL DEFAULT 0,
    commission DECIMAL(20,8) NOT NULL DEFAULT 0,
    funding DECIMAL(20,8) NOT NULL DEFAULT 0,
    net_pnl DECIMAL(20,8) NOT NULL DEFAULT 0,
    trade_count INTEGER NOT NULL DEFAULT 0,
    fetched_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, date)
);

CREATE INDEX idx_user_daily_pnl_user_date ON user_daily_pnl_summaries(user_id, date DESC);
```

### Backend Changes

**Repository Layer:**
- `GetUserDailyPnL(userID int, date time.Time) (*UserDailyPnL, error)`
- `GetUserDailyPnLRange(userID int, startDate, endDate time.Time) ([]UserDailyPnL, error)`
- `SaveUserDailyPnL(summary *UserDailyPnL) error`

**Handler Logic:**
```go
func (h *Handler) handleGetPnLSummary(c *gin.Context) {
    userID := getUserID(c)
    timezone := getUserTimezone(c)

    today := time.Now().In(timezone).Truncate(24*time.Hour)
    results := make([]DailyPnL, 7)

    for i := 0; i < 7; i++ {
        date := today.AddDate(0, 0, -i)

        if date.Equal(today) {
            // Always fetch TODAY live from Binance
            pnl := h.fetchFromBinance(userID, date)
            results[i] = pnl
        } else {
            // Check DB first for historical
            cached, err := h.repo.GetUserDailyPnL(userID, date)
            if err == nil && cached != nil {
                results[i] = cached.ToDailyPnL()
            } else {
                // Not cached, fetch and store
                pnl := h.fetchFromBinance(userID, date)
                h.repo.SaveUserDailyPnL(pnl)
                results[i] = pnl
            }
        }
    }

    return results
}
```

### Frontend Changes

**PnLSummaryCard.tsx:**
- Add state for `selectedDay` and `dateRange`
- Add `DateRangeFilter` component
- Add `NavigationControls` component
- Modify day cards to show date numbers
- Add `DayDetailPanel` component for selected day

**API Service:**
```typescript
// New endpoint for extended ranges
getPnLHistory(startDate: string, endDate: string): Promise<DailyPnL[]>
```

### Affected Files

**Backend:**
- `migrations/038_user_daily_pnl_summaries.sql` (NEW)
- `internal/database/repository_daily_pnl.go` (NEW)
- `internal/database/models_daily_pnl.go` (NEW)
- `internal/api/handlers_futures.go` (MODIFY)

**Frontend:**
- `web/src/components/PnLSummaryCard.tsx` (MODIFY)
- `web/src/components/PnLSummary/DateRangeFilter.tsx` (NEW)
- `web/src/components/PnLSummary/NavigationControls.tsx` (NEW)
- `web/src/components/PnLSummary/DayDetailPanel.tsx` (NEW)
- `web/src/services/futuresApi.ts` (MODIFY)

## Tasks

### Task 1: Database Migration & Repository
- [ ] Create migration `038_user_daily_pnl_summaries.sql`
- [ ] Create model `models_daily_pnl.go`
- [ ] Create repository methods in `repository_daily_pnl.go`
- [ ] Unit tests for repository

### Task 2: Backend API - Smart Fetching
- [ ] Modify `handleGetPnLSummary` to use DB-first pattern
- [ ] Implement caching logic (TODAY=live, historical=DB)
- [ ] Add `/api/futures/pnl-history` endpoint
- [ ] Integration tests

### Task 3: Frontend - Day Card UI Enhancements
- [ ] Update PnLSummaryCard to show date numbers
- [ ] Add "TODAY" label for current day
- [ ] Visual indicator for live vs cached data
- [ ] Click handler for day selection

### Task 4: Frontend - Date Range Filter
- [ ] Create DateRangeFilter component
- [ ] Implement Day/Week/Month/Year/Range modes
- [ ] Connect to API with appropriate date parameters

### Task 5: Frontend - Navigation Controls
- [ ] Create NavigationControls component
- [ ] Implement First/Prev/Next/Last navigation
- [ ] Page indicator display
- [ ] Horizontal scrolling/pagination

### Task 6: Frontend - Detail Panel
- [ ] Create DayDetailPanel component
- [ ] Display selected day's full breakdown
- [ ] Highlight selected card
- [ ] Position panel on left side

### Task 7: Testing & Verification
- [ ] Verify performance improvement (< 500ms cached load)
- [ ] Test date range queries
- [ ] Test navigation controls
- [ ] Verify TODAY always fetches live

## Dependencies

- Epic 8 (Daily Settlement) - Uses similar data structure
- Epic 12 (WebSocket) - PNL_UPDATE events for live refresh

## Estimated Effort

- Backend: 3-4 hours
- Frontend: 4-5 hours
- Testing: 2 hours
- **Total: ~10 hours**

## Priority

**High** - Significant UX improvement and API efficiency gain

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-18 | Story created | BMad Master |
| 2026-01-18 | Implementation complete - CODE REVIEW PASSED + QA CONCERNS | BMad Master |
