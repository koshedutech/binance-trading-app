# Story 8.7: Capital Utilization Tracking
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 9
**Story Points:** 5
**Priority:** P1

## User Story
As a trader, I want to track my maximum and average capital utilization each day so that I can monitor my risk exposure and optimize my capital efficiency.

## Acceptance Criteria
- [ ] Sample capital usage periodically (every 5 minutes)
- [ ] Store max capital used during day
- [ ] Calculate average capital utilization
- [ ] Track max drawdown from daily high
- [ ] Include in daily summary (starting/ending balance, max/avg capital)
- [ ] Used for risk monitoring and billing tier determination
- [ ] Capital samples stored in Redis cache during day
- [ ] EOD aggregation persisted to database

## Technical Approach
Implement capital sampling service that:
1. **Intraday Sampling** (every 5 minutes):
   - Fetch account balance from Binance
   - Calculate used margin (in open positions)
   - Calculate available margin
   - Get unrealized P&L
   - Store in Redis cache with timestamp
   - Key format: `capital_samples:{user_id}:{date}`

2. **EOD Aggregation**:
   - Retrieve all samples for the day from Redis
   - Calculate max capital used (highest used margin)
   - Calculate average capital utilization
   - Determine starting balance (first sample)
   - Determine ending balance (last sample)
   - Calculate max drawdown (largest unrealized loss)
   - Store aggregated metrics in daily_mode_summaries
   - Clear Redis cache for that day

**Data Structures:**
```go
// Intraday sample (stored in Redis)
type CapitalSample struct {
    Timestamp       time.Time
    TotalBalance    float64  // Wallet balance
    UsedMargin      float64  // In positions
    AvailableMargin float64
    UnrealizedPnl   float64
}

// EOD aggregation (stored in daily_mode_summaries)
type CapitalMetrics struct {
    StartingBalance float64  // First sample of day
    EndingBalance   float64  // Last sample of day
    MaxCapitalUsed  float64  // Highest used margin
    AvgCapitalUsed  float64  // Average of samples
    MaxDrawdown     float64  // Largest unrealized loss
    PeakBalance     float64  // Highest balance during day
}
```

**Sampling Strategy:**
- Use Redis sorted set for efficient time-based queries
- Sample every 5 minutes during market hours
- Graceful handling if Binance API unavailable (skip sample, log warning)
- Retry once if sampling fails

**Capital Utilization Calculation:**
```
Capital Utilization % = (Used Margin / Total Balance) * 100
Average Utilization = Sum(all samples' utilization) / Sample Count
Max Capital Used = Max(all samples' used margin)
```

## Dependencies
- **Blocked By:**
  - Epic 6 (Redis - for intraday cache)
  - Story 8.3 (Daily Summary Storage - for EOD persistence)
- **Blocks:** None

## Files to Create/Modify
- `internal/settlement/capital_tracker.go` - Capital sampling service
- `internal/settlement/scheduler.go` - 5-minute sampling ticker
- `internal/cache/capital_samples.go` - Redis operations for samples
- `internal/database/repository_capital_samples.go` - Capital samples repository
- `internal/binance/client.go` - GetAccountBalance() method
- `internal/settlement/service.go` - Integration with settlement flow
- `main.go` - Start capital sampling goroutine

## Testing Requirements
- Unit tests:
  - Test capital metrics calculation from samples
  - Test max capital used tracking
  - Test average utilization calculation
  - Test max drawdown calculation
  - Test edge case: single sample
  - Test edge case: no samples (API unavailable all day)
- Integration tests:
  - Test Redis storage and retrieval
  - Test 5-minute sampling ticker
  - Test EOD aggregation from Redis to database
  - Test Redis cache cleanup after aggregation
- Performance tests:
  - Verify sampling completes <1 second per user
  - Test with 288 samples per day (5-min intervals for 24h)
  - Verify Redis memory usage reasonable

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Capital sampling runs every 5 minutes
- [ ] Samples stored in Redis cache
- [ ] EOD aggregation persisted to database
- [ ] Capital metrics included in daily_mode_summaries
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Sampling resilient to Binance API failures
- [ ] Redis cache cleanup working
- [ ] Performance verified (<1s per sample)
- [ ] Documentation updated (capital metrics, sampling process)
- [ ] PO acceptance received
