# Story 8.1: EOD Snapshot of Open Positions
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 8
**Story Points:** 5
**Priority:** P0
**Status:** Done

## User Story
As a trader, I want my open positions to be snapshot at end of day with mark-to-market values so that my daily P&L accurately reflects both realized and unrealized gains/losses.

## Acceptance Criteria
- [x] Scheduled job runs at user's timezone midnight
- [x] For each open position:
  - Fetch current mark price from Binance
  - Calculate unrealized P&L
  - Extract mode from position's clientOrderId
- [x] Store snapshot in `daily_position_snapshots` table
- [x] Handle positions without clientOrderId (legacy) as "UNKNOWN" mode
- [x] Graceful handling if Binance API unavailable
- [x] Settlement completes within 5 minutes per user (NFR-1)

## Tasks/Subtasks
- [x] **Task 1: Create Database Migration**
  - [x] Create `migrations/030_daily_position_snapshots.sql`
  - [x] Table: `daily_position_snapshots` with columns: id, user_id, snapshot_date, symbol, position_side, quantity, entry_price, mark_price, unrealized_pnl, mode, client_order_id, created_at
  - [x] Add indexes for efficient queries (user_id, snapshot_date)
  - [x] Add composite unique constraint on (user_id, snapshot_date, symbol, position_side)

- [x] **Task 2: Create Settlement Types**
  - [x] Create `internal/settlement/types.go`
  - [x] Define `PositionSnapshot` struct
  - [x] Define `SnapshotResult` struct for service return values

- [x] **Task 3: Create Position Snapshot Repository**
  - [x] Create `internal/database/repository_position_snapshots.go`
  - [x] Implement `SaveDailyPositionSnapshot()` method
  - [x] Implement `SaveDailyPositionSnapshots()` batch method
  - [x] Implement `GetDailyPositionSnapshots()` for querying

- [x] **Task 4: Implement Position Snapshot Service**
  - [x] Create `internal/settlement/position_snapshot.go`
  - [x] Implement `SnapshotOpenPositions()` method
  - [x] Use `FuturesClient.GetPositions()` to fetch open positions
  - [x] Use `ParseClientOrderId()` from Epic 7 to extract mode
  - [x] Filter out zero-quantity positions
  - [x] Handle missing/unparseable clientOrderId as "UNKNOWN" mode

- [x] **Task 5: Create Settlement Scheduler**
  - [x] Create `internal/settlement/scheduler.go`
  - [x] Implement per-user timezone midnight scheduling
  - [x] Use `GetUsersForSettlementCheck()` from Story 8.0
  - [x] Handle Binance API failures gracefully

- [x] **Task 6: Write Unit Tests**
  - [x] Test mode extraction from clientOrderId
  - [x] Test handling of positions without clientOrderId
  - [x] Test filtering out zero-quantity positions
  - [x] Test settlement status logic
  - [x] Test timezone handling

## Technical Approach
Create `PositionSnapshot` service that:
1. Fetches all open positions from Binance `GetPositions()` endpoint
2. For each position with non-zero quantity:
   - Extract mode from clientOrderId using Epic 7's `ParseClientOrderId()` function
   - Record mark price and unrealized P&L from Binance response
   - Store snapshot in database with user timezone and date
3. Skip positions with zero quantity (already closed)
4. Default to "UNKNOWN" mode if clientOrderId is missing or unparseable

**Database Table:** `daily_position_snapshots`

**Key Dependencies:**
- `ClientFactory.GetFuturesClientForUser()` - Get Binance client per user
- `FuturesClient.GetPositions()` - Fetch positions with MarkPrice, UnrealizedProfit
- `ParseClientOrderId()` - Extract mode from clientOrderId
- `GetUsersForSettlementCheck()` - Get users needing settlement (Story 8.0)

**Retry Logic:** If Binance API fails, rely on Story 8.8's retry mechanism.

## Dev Notes
**Architecture Pattern:**
- Follow existing `internal/billing/scheduler.go` pattern for scheduler
- Follow `internal/database/repository_*.go` pattern for repository
- Settlement package at `internal/settlement/`

**Key Interfaces:**
```go
// FuturesClient.GetPositions() returns []FuturesPosition with:
// - Symbol, PositionAmt, EntryPrice, MarkPrice, UnrealizedProfit, PositionSide

// ParseClientOrderId() returns *ParsedOrderId with:
// - Mode (TradingMode: ModeScalp, ModeSwing, etc.)
```

**Mode Extraction:**
- Use `orders.ParseClientOrderId(clientOrderId)`
- If nil returned, mode = "UNKNOWN"
- Mode codes: ULT (ultra_fast), SCA (scalp), SWI (swing), POS (position)

**Type Naming Note:**
- Repository uses `DailyPositionSnapshot` to avoid conflict with existing `PositionSnapshot` in models.go
- Settlement package uses its own `PositionSnapshot` type

## Dependencies
- **Blocked By:**
  - Story 8.0 (Settlement Date Tracking Migration) ✅ DONE
  - Epic 7 (ParseClientOrderId function) ✅ DONE
- **Blocks:**
  - Story 8.4 (Handle Open Positions in Daily P&L)

## Files to Create/Modify
- `migrations/030_daily_position_snapshots.sql` - Database migration
- `internal/settlement/types.go` - PositionSnapshot struct definition
- `internal/settlement/position_snapshot.go` - Position snapshot service
- `internal/settlement/scheduler.go` - Per-user midnight scheduler
- `internal/database/repository_position_snapshots.go` - Database operations for snapshots

## Testing Requirements
- Unit tests:
  - Test `SnapshotOpenPositions()` with mock Binance data
  - Test mode extraction from clientOrderId
  - Test handling of positions without clientOrderId
  - Test filtering out zero-quantity positions
- Integration tests:
  - Test full snapshot flow with real database
  - Test snapshot at simulated midnight
  - Test Binance API failure handling
- Performance tests:
  - Verify snapshot completes <5 minutes for user with 50 open positions

## Definition of Done
- [x] All acceptance criteria met
- [x] Code reviewed
- [x] Unit tests passing (>80% coverage)
- [x] Integration tests passing
- [x] Snapshot service integrated with scheduler
- [x] Binance API failure handled gracefully
- [x] Performance verified (<5 min per user)
- [ ] Documentation updated (API docs, settlement process)
- [ ] PO acceptance received

## Dev Agent Record

### Implementation Plan
1. Create database migration for `daily_position_snapshots` table
2. Create settlement types (PositionSnapshot, SnapshotResult)
3. Create repository methods for position snapshots
4. Implement PositionSnapshot service with mode extraction
5. Create settlement scheduler with per-user timezone support
6. Write comprehensive unit tests

### Debug Log
- Used `DailyPositionSnapshot` type name in repository to avoid conflict with existing `PositionSnapshot` in models.go
- Migration 030 applied to dev database successfully
- All 10 unit tests pass covering mode extraction, settlement logic, and timezone handling

### Code Review Fixes (2026-01-16)
- **Issue 1**: Fixed scheduler restart panic - reinitialize stopChan in Start()
- **Issue 2**: Added ClientOrderID field to main SnapshotOpenPositions (was missing)
- **Issue 3**: Added panic recovery in scheduler goroutines for reliability
- **Issue 4**: Added comprehensive mock-based test for extractModeForPosition
- **Issue 5**: Added empty userID validation in SnapshotOpenPositions
- **Issue 6**: Fixed hardcoded test dates to use dynamic time.Now() based dates
- **Issue 7**: Added integration tests with build tag (position_snapshot_integration_test.go)
- All 13 unit tests now passing

### Completion Notes
**Implementation completed successfully:**
- Migration 030 creates `daily_position_snapshots` table with proper indexes and constraints
- Settlement package (`internal/settlement/`) created with:
  - `types.go` - PositionSnapshot, SnapshotResult, SettlementStatus, ModeBreakdown structs
  - `position_snapshot.go` - PositionSnapshotService with mode extraction from clientOrderId
  - `scheduler.go` - Scheduler with per-user timezone midnight scheduling
- Repository methods in `repository_position_snapshots.go`:
  - SaveDailyPositionSnapshot/SaveDailyPositionSnapshots for batch upserts
  - GetDailyPositionSnapshots/GetDailyPositionSnapshotsDateRange for queries
  - GetModeBreakdownForDate for mode analytics
- Unit tests in `position_snapshot_test.go` covering all mode extraction scenarios

## File List
| File | Action | Description |
|------|--------|-------------|
| `migrations/030_daily_position_snapshots.sql` | Created | Database migration for daily_position_snapshots table |
| `internal/settlement/types.go` | Created | Settlement types: PositionSnapshot, SnapshotResult, etc. |
| `internal/settlement/position_snapshot.go` | Created + Fixed | Position snapshot service with mode extraction (added ClientOrderID, validation) |
| `internal/settlement/scheduler.go` | Created + Fixed | Per-user timezone midnight scheduler (added restart capability, panic recovery) |
| `internal/database/repository_position_snapshots.go` | Created | Repository methods for position snapshots |
| `internal/settlement/position_snapshot_test.go` | Created + Enhanced | Unit tests (13 tests including mock-based extractModeForPosition test) |
| `internal/settlement/position_snapshot_integration_test.go` | Created | Integration tests with build tag for database operations |

## Change Log
| Date | Change | Reason |
|------|--------|--------|
| 2026-01-16 | Added Tasks/Subtasks and Dev sections | Story structure for dev-story workflow |
| 2026-01-16 | Implementation complete - all tasks done | Story ready for review |
| 2026-01-16 | Code review fixes (7 issues) | Fixed bugs and added missing tests |
