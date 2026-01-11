# Story 8.2: Daily P&L Aggregation by Mode
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 8
**Story Points:** 5
**Priority:** P0

## User Story
As a trader, I want my daily P&L broken down by trading mode (ULT, SCA, SWI, POS) so that I can analyze which strategies perform best.

## Acceptance Criteria
- [ ] Fetch all trades closed during the day from Binance
- [ ] Parse clientOrderId to extract mode
- [ ] Aggregate by mode:
  - Total realized P&L
  - Trade count
  - Win count / Loss count
  - Win rate calculation
  - Largest win / Largest loss
  - Total volume in USDT
  - Average trade size
- [ ] Handle trades without mode as "UNKNOWN"
- [ ] Calculate "ALL" totals across all modes
- [ ] Win rate calculated as (win_count / trade_count) * 100

## Technical Approach
Create `PnLAggregator` service that:
1. Queries Binance `GetUserTrades()` for date range (midnight to midnight in user timezone)
2. Groups trades by mode extracted from clientOrderId using `ParseClientOrderId()`
3. For each mode, calculates:
   - Realized P&L (sum of all trade P&L)
   - Trade metrics (count, wins, losses, win rate)
   - Volume metrics (total volume, average trade size)
   - Extremes (largest win, largest loss)
4. Creates "ALL" mode summary by summing all mode totals
5. Returns `map[string]*ModePnL` for storage

**Key Calculations:**
- Win Rate = (WinCount / TradeCount) * 100
- Average Trade Size = TotalVolume / TradeCount
- Win/Loss determined by trade RealizedPnl sign (positive = win, negative = loss)

## Dependencies
- **Blocked By:**
  - Story 8.0 (User Timezone Migration)
  - Epic 7 (ParseClientOrderId function)
- **Blocks:**
  - Story 8.3 (Daily Summary Storage)
  - Story 8.4 (Handle Open Positions in Daily P&L)

## Files to Create/Modify
- `internal/settlement/pnl_aggregator.go` - P&L aggregation service
- `internal/settlement/types.go` - ModePnL struct definition
- `internal/settlement/service.go` - Integration with main settlement service
- `internal/binance/client.go` - GetUserTrades() method for date range

## Testing Requirements
- Unit tests:
  - Test aggregation with multiple trades per mode
  - Test win rate calculation (various win/loss ratios)
  - Test "ALL" mode total calculation
  - Test handling of trades without clientOrderId (UNKNOWN mode)
  - Test largest win/loss tracking
  - Test average trade size calculation
  - Test edge case: zero trades (no division by zero)
- Integration tests:
  - Test full aggregation with Binance testnet data
  - Test date range filtering (midnight to midnight)
  - Test multiple modes in single day
- Performance tests:
  - Verify aggregation completes <2 minutes for 500 trades

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Aggregation logic verified against manual calculations
- [ ] "ALL" mode totals match sum of individual modes
- [ ] Win rate calculation verified (0-100% range)
- [ ] Documentation updated (aggregation logic, mode types)
- [ ] PO acceptance received
