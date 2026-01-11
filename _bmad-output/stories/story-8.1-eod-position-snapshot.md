# Story 8.1: EOD Snapshot of Open Positions
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 8
**Story Points:** 5
**Priority:** P0

## User Story
As a trader, I want my open positions to be snapshot at end of day with mark-to-market values so that my daily P&L accurately reflects both realized and unrealized gains/losses.

## Acceptance Criteria
- [ ] Scheduled job runs at user's timezone midnight
- [ ] For each open position:
  - Fetch current mark price from Binance
  - Calculate unrealized P&L
  - Extract mode from position's clientOrderId
- [ ] Store snapshot in `daily_position_snapshots` table
- [ ] Handle positions without clientOrderId (legacy) as "UNKNOWN" mode
- [ ] Graceful handling if Binance API unavailable
- [ ] Settlement completes within 5 minutes per user (NFR-1)

## Technical Approach
Create `PositionSnapshot` service that:
1. Fetches all open positions from Binance `GetPositionRisk()` endpoint
2. For each position with non-zero quantity:
   - Extract mode from clientOrderId using Epic 7's `ParseClientOrderId()` function
   - Record mark price and unrealized P&L from Binance response
   - Store snapshot in database with user timezone and date
3. Skip positions with zero quantity (already closed)
4. Default to "UNKNOWN" mode if clientOrderId is missing or unparseable

**Database Table:** `daily_position_snapshots` (see migration in Story 8.3)

**Retry Logic:** If Binance API fails, rely on Story 8.8's retry mechanism.

## Dependencies
- **Blocked By:**
  - Story 8.0 (User Timezone Migration)
  - Epic 7 (ParseClientOrderId function)
- **Blocks:**
  - Story 8.4 (Handle Open Positions in Daily P&L)

## Files to Create/Modify
- `internal/settlement/position_snapshot.go` - Position snapshot service
- `internal/settlement/types.go` - PositionSnapshot struct definition
- `internal/database/repository_position_snapshots.go` - Database operations for snapshots
- `internal/settlement/service.go` - Main settlement service orchestration
- `internal/settlement/scheduler.go` - Per-user midnight scheduler

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
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Snapshot service integrated with scheduler
- [ ] Binance API failure handled gracefully
- [ ] Performance verified (<5 min per user)
- [ ] Documentation updated (API docs, settlement process)
- [ ] PO acceptance received
