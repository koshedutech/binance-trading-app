# Story 14.3: Coin Profiler - Position Requirement Aggregation

## Story
**As a** trading system
**I want** the Coin Profiler to aggregate data requirements from open positions
**So that** the Exit Decision system can monitor positions for TP/SL/Trailing exits even when Trading is OFF

## Status
review

## Story Points
3

## Priority
P0 (Critical)

## Dependencies
- Story 14.1: Coin Profiler Core (DONE) - Core service structure
- Story 14.2: Strategy Requirements (REVIEW) - Strategy requirement aggregation pattern
- Epic 10: Position Management - Position tracking (GetPositions method)

---

## Acceptance Criteria

### AC1: PositionRequirements Type
- [x] PositionRequirements struct defined with Symbol, Timeframes, ExitMode fields
- [x] ExitMode supports "tp_sl", "trailing", "both" values
- [x] Type is exported and documented

### AC2: Position Reading Interface
- [x] PositionReader interface defined for dependency injection
- [x] Interface matches GinieAutopilot's GetPositions() signature
- [x] Supports mock implementation for testing

### AC3: GetPositionRequirements Function
- [x] Function extracts requirements from a list of positions
- [x] Determines exit monitoring timeframes based on position mode
- [x] Handles nil/empty position list gracefully

### AC4: Timeframe Selection Based on Mode
- [x] Scalp mode positions use 5m, 15m timeframes
- [x] Swing mode positions use 15m, 1h timeframes
- [x] Position mode positions use 1h, 4h timeframes
- [x] Ultra-fast mode positions use 1m, 5m timeframes

### AC5: Exit Mode Detection
- [x] Detects TP/SL monitoring based on position fields (TakeProfits, StopLoss)
- [x] Detects trailing mode based on TrailingActive field
- [x] Returns "both" when position has both TP/SL and trailing active

### AC6: CombineRequirements Function
- [x] Combines strategy requirements with position requirements
- [x] Deduplicates symbols and timeframes
- [x] Updates DataSource to "both" when same symbol needed by both
- [x] Preserves all strategy-specific data

### AC7: Unit Tests
- [x] Tests for PositionRequirements type
- [x] Tests for GetPositionRequirements with various positions
- [x] Tests for CombineRequirements deduplication
- [x] Tests for edge cases (empty lists, nil positions)
- [x] All tests pass

---

## Tasks/Subtasks

### Task 1: Implement Position Requirement Types
- [x] Define PositionRequirements struct
- [x] Define ExitMode constants
- [x] Define PositionReader interface
- [x] Add documentation comments

### Task 2: Implement GetPositionRequirements Function
- [x] Create getExitTimeframesForMode helper function
- [x] Create detectExitMode function
- [x] Implement GetPositionRequirements main function
- [x] Handle edge cases (nil, empty)

### Task 3: Implement CombineRequirements Function
- [x] Merge strategy and position symbols
- [x] Deduplicate timeframes per symbol
- [x] Update DataSource appropriately
- [x] Return combined AggregatedRequirements

### Task 4: Write Unit Tests
- [x] Test GetPositionRequirements with scalp positions
- [x] Test GetPositionRequirements with swing positions
- [x] Test GetPositionRequirements with position mode
- [x] Test GetPositionRequirements with trailing active
- [x] Test GetPositionRequirements with TP/SL only
- [x] Test CombineRequirements deduplication
- [x] Test edge cases (empty, nil)

### Task 5: Integration Verification
- [x] Verify types are compatible with GiniePosition
- [x] Run full test suite
- [x] Update sprint status

---

## Dev Notes

### Architecture Context
The Coin Profiler serves as the central data hub for both:
1. **Entry Decision** (strategies) - What data to collect for new entries
2. **Exit Decision** (positions) - What data to collect for monitoring exits

Even when Trading is OFF, the Coin Profiler must still collect data for open positions
so that TP/SL/Trailing exits can be monitored.

### Position Data Flow
```
Open Positions (GinieAutopilot.GetPositions())
    |
    v
GetPositionRequirements() - Extract symbols, timeframes, exit mode
    |
    v
CombineRequirements() - Merge with strategy requirements
    |
    v
Coin Profiler subscribes to combined symbol+timeframe list
```

### GiniePosition Fields (from ginie_autopilot.go:785)
```go
type GiniePosition struct {
    Symbol       string           // e.g., "BTCUSDT"
    Side         string           // "LONG" or "SHORT"
    Mode         GinieTradingMode // scalp, swing, position, ultra_fast
    EntryPrice   float64
    StopLoss     float64          // Exit monitoring
    TakeProfits  []GinieTakeProfitLevel // Exit monitoring
    TrailingActive bool           // Trailing stop mode
    // ... other fields
}
```

### Timeframe Selection Logic
```go
// Exit monitoring needs faster updates than entry scanning
func getExitTimeframesForMode(mode string) []string {
    switch mode {
    case "ultra_fast":
        return []string{"1m", "5m"}
    case "scalp":
        return []string{"5m", "15m"}
    case "swing":
        return []string{"15m", "1h"}
    case "position":
        return []string{"1h", "4h"}
    default:
        return []string{"5m", "15m"} // Fallback to scalp
    }
}
```

### Implementation Notes
1. Use same file structure as Story 14.2 (requirements.go)
2. Follow existing test patterns from requirements_test.go
3. PositionReader interface allows mocking without autopilot dependency
4. CombineRequirements returns same AggregatedRequirements type for consistency

### Trading ON/OFF Impact
| Button State | Coin Profiler | Entry Decision | Exit Decision |
|--------------|---------------|----------------|---------------|
| **ON** | Runs (strategies + positions) | Active | Active |
| **OFF** | Runs (positions only) | Paused | Active |

---

## Dev Agent Record

### Implementation Plan
1. Add position-specific types to requirements.go
2. Implement GetPositionRequirements function
3. Implement CombineRequirements function
4. Write comprehensive tests
5. Verify integration with existing patterns

### Debug Log
(Will be updated during implementation)

### Completion Notes
(Will be updated on completion)

---

## File List
- `/home/administrator/KOSH/binance-trading-app/internal/coinprofiler/requirements.go` (lines 419-776) - Position requirement types and functions
- `/home/administrator/KOSH/binance-trading-app/internal/coinprofiler/requirements_test.go` (lines 739-1523) - Comprehensive unit tests

---

## Change Log
| Date | Change |
|------|--------|
| 2026-01-25 | Story created from Epic 14.3 requirements |
| 2026-01-25 | Implementation complete - all acceptance criteria met |

