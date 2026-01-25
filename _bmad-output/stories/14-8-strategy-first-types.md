# Story 14.8: Entry Decision - Strategy-First Data Structure

## Story

**As a** Chain Trading System
**I want** data structures for strategy-first view
**So that** the Entry Decision System can display coins matched to strategies with proper pattern/score tracking

## Status

review

## Acceptance Criteria

- [x] AC1: Define StrategyMatch type (coins matching a strategy)
- [x] AC2: Support pattern-based strategies (step progress, not scores)
- [x] AC3: Support score-based strategies (score values)
- [x] AC4: Handle mode+strategy combinations (same strategy in different modes)

## Tasks/Subtasks

- [x] Task 1: Create `internal/entrydecision/` package
  - [x] Create package directory structure
  - [x] Define package documentation

- [x] Task 2: Define StrategyType enum
  - [x] Define StrategyTypePattern for pattern-based strategies
  - [x] Define StrategyTypeScore for score-based strategies
  - [x] Add IsValid() method for validation
  - [x] Add String() method for display

- [x] Task 3: Define PatternStatus enum
  - [x] Define all pattern statuses: watching, accumulation, consolidating, ready, failed, expired
  - [x] Add IsEntryReady() method
  - [x] Add IsActive() method

- [x] Task 4: Define CoinMatch struct
  - [x] Add common fields: Symbol, UpdatedAt
  - [x] Add pattern-based fields: Step, Status, Details
  - [x] Add score-based fields: Score, Ready
  - [x] Add helper methods: IsReady(), String()
  - [x] Add constructors: NewPatternCoinMatch(), NewScoreCoinMatch()

- [x] Task 5: Define StrategyMatch struct
  - [x] Add identification fields: Mode, Strategy, SubStrategy
  - [x] Add configuration fields: Type, Timeframe, Timeframes, Threshold, Enabled
  - [x] Add Coins slice for matched coins
  - [x] Add Requirements for UI display
  - [x] Add helper methods: AddCoin(), GetReadyCoins(), GetReadyCount(), GetWatchingCount(), UniqueKey()
  - [x] Add chain methods: WithThreshold(), WithTimeframes(), WithRequirements()

- [x] Task 6: Define ModeStrategies struct
  - [x] Group strategies by trading mode
  - [x] Add GetStrategyCount(), GetEnabledCount(), GetTotalReadyCoins() methods

- [x] Task 7: Define EntryDecisionResponse struct
  - [x] Add Strategies slice for flat view
  - [x] Add ByMode slice for hierarchical view
  - [x] Add summary statistics
  - [x] Add GroupByMode() method with consistent mode ordering
  - [x] Add CalculateSummary() method

- [x] Task 8: Define EntryCandidatesResponse and EntryCandidate structs
  - [x] Define EntryCandidate with all required fields
  - [x] Add AddCandidate() method

- [x] Task 9: Define PatternProgress struct
  - [x] Add step tracking: CurrentStep, TotalSteps
  - [x] Add StepDetails slice
  - [x] Add timing fields: StartedAt, UpdatedAt, ExpiresAt
  - [x] Add AdvanceStep(), SetStatus(), IsComplete(), IsExpired() methods
  - [x] Add ToCoinMatch() for API responses

- [x] Task 10: Write comprehensive unit tests
  - [x] Test StrategyType validation
  - [x] Test PatternStatus methods
  - [x] Test CoinMatch constructors and methods
  - [x] Test StrategyMatch methods
  - [x] Test ModeStrategies methods
  - [x] Test EntryDecisionResponse methods
  - [x] Test PatternProgress lifecycle
  - [x] Test same strategy in different modes
  - [x] Test mixed pattern and score strategies

## Dev Notes

### Architecture Context
This story creates the core data structures for the Entry Decision System's strategy-first view. The Entry Decision System is part of Epic 14's Chain Trading System, which operates independently of the legacy Ginie Autopilot.

### Key Design Decisions

1. **Unified CoinMatch struct**: Rather than separate types for pattern and score matches, we use a unified struct with optional fields. This simplifies API responses and allows the same coin to appear in both types.

2. **PatternStatus enum**: Explicit status values for pattern tracking make it clear where a coin is in the pattern lifecycle.

3. **UniqueKey format**: `{mode}:{strategy}:{sub_strategy}:{timeframe}` ensures each strategy configuration is uniquely identifiable.

4. **Mode ordering**: The GroupByMode() method uses a consistent ordering (scalp, swing, position, ultra_fast) for predictable UI display.

5. **PatternProgress internal type**: This is for internal pattern tracking, not API exposure. It converts to CoinMatch for responses.

### Related Files
- `internal/coinprofiler/types.go` - Coin data structures from Story 14.1
- `internal/decision/strategy.go` - Strategy interface definitions from Epic 11
- Future: `internal/entrydecision/pattern_volume_imbalance.go` (Story 14.9)
- Future: `internal/entrydecision/score_calculator.go` (Story 14.10)

### Test Coverage
The unit tests achieve 97.2% coverage, testing:
- All type validation methods
- Constructor functions
- Helper methods and chain methods
- Mode grouping and ordering
- Pattern progress lifecycle
- Same strategy in different modes scenario
- Mixed pattern and score strategies scenario

## Dev Agent Record

### Implementation Plan
1. Create the entrydecision package directory
2. Define all types in types.go with comprehensive documentation
3. Implement all helper methods following Go best practices
4. Write comprehensive unit tests covering all acceptance criteria
5. Verify build and ensure no regressions

### Debug Log
- Initial implementation created with all required types
- Test failure in TestPatternProgress_AdvanceStep: AdvanceStep returned false on final step advancement
- Fixed: Changed condition from `CurrentStep >= TotalSteps` to `CurrentStep > TotalSteps`
- All tests now pass with 97.2% coverage

### Completion Notes
- Created `internal/entrydecision/types.go` with 550+ lines of type definitions
- Created `internal/entrydecision/types_test.go` with 500+ lines of tests
- All 26 test cases pass
- Build verification successful
- No regressions in related packages (coinprofiler, decision)

## File List

**New Files:**
- `internal/entrydecision/types.go` - Core type definitions for strategy-first view
- `internal/entrydecision/types_test.go` - Comprehensive unit tests

## Change Log

| Date | Change |
|------|--------|
| 2026-01-25 | Created entrydecision package with strategy-first data structures (Story 14.8) |
