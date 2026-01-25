# Story 14.2: Coin Profiler - Strategy Requirement Aggregation

## Story

**Epic:** 14 - Chain Trading System
**Priority:** P0
**Status:** review
**Created:** 2026-01-25

Aggregate data requirements from all enabled strategies to inform the Coin Profiler what data to collect.

## Acceptance Criteria

- [x] AC1: Read enabled strategies from database (use existing repository methods in `internal/database/repository_strategy_hierarchy.go`)
- [x] AC2: Extract timeframe requirements per strategy
- [x] AC3: Extract indicator/data requirements per strategy
- [x] AC4: Combine and deduplicate requirements
- [x] AC5: Support mode+strategy combinations (same strategy, different modes)

## Tasks/Subtasks

- [x] Task 1: Create `StrategyRequirements` type with Mode, Strategy, SubStrategy, Timeframes, DataFields, Filters
- [x] Task 2: Create strategy registry with data requirements for known strategies (Volume Imbalance, Classic Breakout, Trend Following, etc.)
- [x] Task 3: Implement `GetStrategyRequirements()` function to extract requirements for a single strategy
- [x] Task 4: Implement `GetRequirementsForStrategies()` to extract requirements for multiple enabled strategies
- [x] Task 5: Implement `AggregateRequirements()` to combine and deduplicate all requirements
- [x] Task 6: Create `RequirementAggregator` with `Aggregate()` method that integrates with database reader
- [x] Task 7: Add utility functions for timeframe sorting and strategy registration
- [x] Task 8: Write comprehensive unit tests covering all acceptance criteria

## Dev Notes

### Architecture

The requirements aggregation follows a strategy-driven approach:
1. Read enabled strategies from database using existing `GetEnabledStrategies(ctx, userID)` repository method
2. Look up each strategy in the registry to get its data requirements
3. Aggregate and deduplicate across all strategies to minimize subscriptions

### Key Types

```go
type StrategyRequirements struct {
    Mode        string   // "scalp", "swing", "position", "ultra_fast"
    Strategy    string   // Strategy group: "breakout", "trending", "range", "volatile"
    SubStrategy string   // e.g., "ravindra_volume_imbalance", "classic_breakout"
    Timeframes  []string // ["5m", "15m", "1h"]
    DataFields  []string // ["volume", "taker_buy_volume", "ohlc"]
    Filters     map[string]interface{} // min_volume, etc.
}

type AggregatedRequirements struct {
    AllTimeframes []string                    // Deduplicated timeframes
    AllDataFields []string                    // Deduplicated data fields
    ByTimeframe   map[string][]StrategyRef   // Which strategies need each timeframe
    ByStrategy    []StrategyRequirements      // Full requirements per strategy
    TotalStrategies int
}
```

### Strategy Registry

The registry maps sub-strategy names to their data requirements:
- `ravindra_volume_imbalance`: Scalp=15m, Swing=1h, Position=4h; needs volume, taker_buy_volume, ohlc
- `classic_breakout`: Scalp=5m+15m, Swing=15m+1h, Position=1h+4h; needs volume, ohlc
- `trend_following`: Scalp=5m+15m, Swing=1h+4h, Position=4h+1d; needs volume, ohlc
- `mean_reversion`: Same timeframes as trend_following
- `range_trading`: Same timeframes as trend_following

### Timeframe Validation

Volume Imbalance strategy uses 15m for scalp mode (validated by backtesting), not 5m. Institutions need 30-60+ minutes to accumulate quietly.

## Dev Agent Record

### Implementation Plan
1. Create `requirements.go` with all types and functions
2. Implement strategy registry with known strategies from Epic 11
3. Add aggregation logic with deduplication
4. Create interface for dependency injection (database reader)
5. Write comprehensive tests

### Debug Log
- 2026-01-25: Story started - in-progress status
- 2026-01-25: Created `internal/coinprofiler/requirements.go` with all types and functions
- 2026-01-25: Created `internal/coinprofiler/requirements_test.go` with 51 test cases
- 2026-01-25: All tests pass, build successful

### Completion Notes
- Implemented complete strategy requirement aggregation system
- Registry includes 5 strategies: ravindra_volume_imbalance, classic_breakout, trend_following, mean_reversion, range_trading
- Supports mode+strategy combinations with different timeframes per mode
- Deduplication works correctly for both timeframes and data fields
- Interface-based design allows easy testing with mock readers
- All 51 tests pass covering all 5 acceptance criteria

## File List

### New Files
- `internal/coinprofiler/requirements.go` - Strategy requirement types, registry, and aggregation logic
- `internal/coinprofiler/requirements_test.go` - Comprehensive test suite (51 tests)

### Modified Files
- None

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-25 | Story created | Dev Agent |
| 2026-01-25 | Implemented strategy requirement aggregation | Dev Agent |
| 2026-01-25 | Added comprehensive tests (51 tests, all pass) | Dev Agent |
| 2026-01-25 | Story marked for review | Dev Agent |
