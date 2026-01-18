# Traceability Matrix - Story 13.1: PNL Summary Caching & Historical Navigation

## Story Information
- **Story ID**: 13.1
- **Title**: PNL Summary Caching & Historical Navigation
- **Status**: Review
- **Date**: 2026-01-18

---

## Requirements-to-Implementation Traceability

| AC | Requirement | Implementation File(s) | Evidence | Status |
|----|-------------|------------------------|----------|--------|
| **AC1** | Database Storage for Daily PnL Summaries | `migrations/038_user_daily_pnl_summaries.sql` | Table created with columns: id, user_id, date, pnl, commission, funding, net_pnl, trade_count, rebate, other, fetched_at, created_at. UNIQUE constraint on (user_id, date) present at line 42. | PASS |
| **AC1** | Repository methods | `internal/database/repository_daily_pnl.go` | GetUserDailyPnL (line 15), GetUserDailyPnLRange (line 40), SaveUserDailyPnL (line 84), BulkSaveUserDailyPnL (line 117), GetUserDailyPnLMultipleDates (line 165) | PASS |
| **AC1** | Data models | `internal/database/models_daily_pnl.go` | UserDailyPnLSummary struct (line 11), DailyPnLBreakdown struct (line 36), ToDailyBreakdown conversion (line 53) | PASS |
| **AC2** | Smart Data Fetching - TODAY always live | `internal/api/handlers_futures.go` | Lines 3216-3218: `if isToday { datesToFetchFromBinance = append(...) }` ensures TODAY always fetched from Binance | PASS |
| **AC2** | Smart Data Fetching - Historical from DB first | `internal/api/handlers_futures.go` | Lines 3219-3227: Checks cachedMap first for historical days, only adds to Binance fetch list if not cached | PASS |
| **AC2** | Cache after Binance fetch | `internal/api/handlers_futures.go` | Lines 3263-3282: Historical data saved to cache via BulkSaveUserDailyPnL after fetching. TODAY explicitly excluded from caching (line 3263: `if !isToday`) | PASS |
| **AC3** | New endpoint /api/futures/pnl-history | `internal/api/server.go` | Line 518: `futures.GET("/pnl-history", s.handleGetPnLHistory)` | PASS |
| **AC3** | Query parameters start_date, end_date | `internal/api/handlers_futures.go` | Lines 3145-3146: `startDateStr := c.Query("start_date")`, `endDateStr := c.Query("end_date")` | PASS |
| **AC3** | Handler implementation | `internal/api/handlers_futures.go` | handleGetPnLHistory function (lines 3121-3322) with full date range support, validation, and response structure | PASS |
| **AC3** | Frontend API service | `web/src/services/futuresApi.ts` | getPnLHistory function (lines 1254-1300) with start_date/end_date params and 120s timeout | PASS |
| **AC4** | Date numbers on cards | `web/src/components/PnLSummaryCard.tsx` | Lines 489-492: Day number rendered prominently with `{day.day}` in 2xl bold text | PASS |
| **AC4** | Day name below date | `web/src/components/PnLSummaryCard.tsx` | Lines 484-487: Day name rendered with `{day.is_today ? 'TODAY' : day.day_name}` | PASS |
| **AC4** | TODAY label logic | `web/src/components/PnLSummaryCard.tsx` | Line 486: Shows "TODAY" when `day.is_today` is true, otherwise shows day_name | PASS |
| **AC4** | Visual indicator for cached vs live | `web/src/components/PnLSummaryCard.tsx` | Lines 513-518: Database icon shown when `day.is_cached` is true; DayDetailPanel shows "Cached" vs "Live" status (lines 131-140) | PASS |
| **AC5** | DateRangeFilter component | `web/src/components/PnLSummary/DateRangeFilter.tsx` | Full implementation (123 lines) with Day/Week/Month/Year/Custom modes | PASS |
| **AC5** | Filter modes supported | `web/src/components/PnLSummary/DateRangeFilter.tsx` | Lines 23-29: rangeOptions array with 'day', 'week', 'month', 'year', 'custom' | PASS |
| **AC5** | Custom date picker | `web/src/components/PnLSummary/DateRangeFilter.tsx` | Lines 74-106: Date input fields with Apply/Cancel buttons when custom mode selected | PASS |
| **AC5** | Integration in PnLSummaryCard | `web/src/components/PnLSummaryCard.tsx` | Lines 428-433: DateRangeFilter component integrated with onRangeChange handler | PASS |
| **AC6** | NavigationControls component | `web/src/components/PnLSummary/NavigationControls.tsx` | Full implementation (97 lines) | PASS |
| **AC6** | First/Prev/Next/Last buttons | `web/src/components/PnLSummary/NavigationControls.tsx` | Lines 28-94: All four navigation buttons with ChevronsLeft, ChevronLeft, ChevronRight, ChevronsRight icons | PASS |
| **AC6** | Page indicator | `web/src/components/PnLSummary/NavigationControls.tsx` | Lines 59-62: `pageLabel` displayed in center of navigation | PASS |
| **AC6** | Integration with pagination | `web/src/components/PnLSummaryCard.tsx` | Lines 434-444: NavigationControls wired to currentPage state with proper handlers | PASS |
| **AC7** | DayDetailPanel component | `web/src/components/PnLSummary/DayDetailPanel.tsx` | Full implementation (147 lines) | PASS |
| **AC7** | Detailed breakdown display | `web/src/components/PnLSummary/DayDetailPanel.tsx` | Lines 72-122: Shows Gross PnL, Commission, Rebate (conditional), Funding Fee, Other (conditional), Net PnL | PASS |
| **AC7** | Trade count display | `web/src/components/PnLSummary/DayDetailPanel.tsx` | Lines 125-129: Shows `{trade_count} trades` with Activity icon | PASS |
| **AC7** | Selected card highlighting | `web/src/components/PnLSummaryCard.tsx` | Line 475: `${isSelected ? 'ring-2 ring-purple-500' : ''}` | PASS |
| **AC7** | Panel positioning | `web/src/components/PnLSummaryCard.tsx` | Lines 449-457: Detail panel rendered inside flex container, appearing on left of calendar grid | PASS |
| **AC8** | Performance - DB-first pattern | `internal/api/handlers_futures.go` | Lines 3198-3228: Cached data retrieved first, only missing dates fetched from Binance | PARTIAL |
| **AC8** | Performance - timing requirements | N/A | Cannot verify < 500ms / < 3s / < 1s timing in code review; requires runtime testing | DEFER |

---

## Implementation-to-Tests Traceability

| Implementation | Test File | Test Coverage | Status |
|----------------|-----------|---------------|--------|
| `internal/database/repository_daily_pnl.go` | None found | No unit tests for repository methods | MISSING |
| `internal/api/handlers_futures.go` (handleGetPnLHistory) | None found | No integration tests for pnl-history endpoint | MISSING |
| `web/src/components/PnLSummary/DateRangeFilter.tsx` | None found | No component tests | MISSING |
| `web/src/components/PnLSummary/NavigationControls.tsx` | None found | No component tests | MISSING |
| `web/src/components/PnLSummary/DayDetailPanel.tsx` | None found | No component tests | MISSING |
| `web/src/components/PnLSummaryCard.tsx` | None found | No component tests | MISSING |

---

## Files Created/Modified

### Backend (Go)
| File | Action | Lines |
|------|--------|-------|
| `migrations/038_user_daily_pnl_summaries.sql` | NEW | 75 |
| `internal/database/models_daily_pnl.go` | NEW | 86 |
| `internal/database/repository_daily_pnl.go` | NEW | 268 |
| `internal/api/handlers_futures.go` | MODIFIED | Added ~200 lines (handleGetPnLHistory, fetchDayPnLFromBinance) |
| `internal/api/server.go` | MODIFIED | Added route registration (line 518) |

### Frontend (TypeScript/React)
| File | Action | Lines |
|------|--------|-------|
| `web/src/components/PnLSummary/DateRangeFilter.tsx` | NEW | 123 |
| `web/src/components/PnLSummary/NavigationControls.tsx` | NEW | 97 |
| `web/src/components/PnLSummary/DayDetailPanel.tsx` | NEW | 147 |
| `web/src/components/PnLSummary/index.ts` | NEW | 5 |
| `web/src/components/PnLSummaryCard.tsx` | MODIFIED | Significant changes (~100 lines for navigation/selection state) |
| `web/src/services/futuresApi.ts` | MODIFIED | Added getPnLHistory function (~50 lines) |

---

## Quality Gate Decision

### Decision: **CONCERNS**

### Rationale

**Implementation Completeness: PASS**
All 8 Acceptance Criteria have corresponding implementation code:
- AC1: Database schema and repository fully implemented
- AC2: Smart fetching logic correctly differentiates TODAY (live) vs historical (DB-first)
- AC3: New `/api/futures/pnl-history` endpoint registered and implemented
- AC4: Day cards display date numbers prominently with "TODAY" label
- AC5: DateRangeFilter component supports Day/Week/Month/Year/Custom modes
- AC6: NavigationControls with First/Prev/Next/Last buttons implemented
- AC7: DayDetailPanel shows complete breakdown with card highlighting
- AC8: DB-first pattern implemented (timing cannot be verified in code review)

**Test Coverage: MISSING**
- No unit tests for repository methods (GetUserDailyPnL, SaveUserDailyPnL, etc.)
- No integration tests for handleGetPnLHistory endpoint
- No component tests for new React components
- This represents a significant gap in quality assurance

**Deferred Items:**
- AC8 performance requirements (< 500ms, < 3s, < 1s) cannot be verified through code review and require runtime load testing

### Recommendations

1. **Add Repository Unit Tests**: Create `internal/database/repository_daily_pnl_test.go` with tests for:
   - `TestGetUserDailyPnL_Found`
   - `TestGetUserDailyPnL_NotFound`
   - `TestGetUserDailyPnLRange`
   - `TestSaveUserDailyPnL_Insert`
   - `TestSaveUserDailyPnL_Upsert`
   - `TestBulkSaveUserDailyPnL`

2. **Add API Integration Tests**: Test the `/api/futures/pnl-history` endpoint with:
   - Valid date range
   - Missing parameters (defaults)
   - Invalid date format
   - Date range exceeding 365 days

3. **Add Frontend Component Tests**: Create tests in `web/src/components/__tests__/`:
   - `DateRangeFilter.test.tsx`
   - `NavigationControls.test.tsx`
   - `DayDetailPanel.test.tsx`

4. **Runtime Performance Testing**: Verify AC8 timing requirements:
   - Initial load with 7-day cached data < 500ms
   - Live TODAY fetch < 3 seconds
   - 30-day historical query < 1 second

---

## Summary

| Category | Status |
|----------|--------|
| AC1: Database Storage | PASS |
| AC2: Smart Fetching | PASS |
| AC3: API Changes | PASS |
| AC4: Date Display UI | PASS |
| AC5: Date Range Filters | PASS |
| AC6: Navigation Controls | PASS |
| AC7: Detail View | PASS |
| AC8: Performance | PARTIAL (code review only) |
| Unit Tests | MISSING |
| Integration Tests | MISSING |
| Component Tests | MISSING |

**Overall Gate Decision: CONCERNS**

The implementation is complete and functional, but the lack of automated test coverage represents a quality risk. The story should proceed to manual QA testing, but automated tests should be added before final acceptance.
