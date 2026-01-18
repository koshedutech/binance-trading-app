# Story 11.2: Delta Update Processor

**Epic:** 11 - Position Decision Engine
**Priority:** P0 (Critical)
**Status:** done
**Created:** 2026-01-17

## Goal

Implement an efficient delta processing system that compares new state values against cached values and only updates changed fields in Redis through batched operations, minimizing database operations and achieving sub-millisecond update performance.

## Acceptance Criteria

- [x] Compare new values vs cached values
- [x] Only update changed fields in Redis
- [x] Batch multiple field updates in single HSET
- [x] Track update frequency per field
- [x] Performance target: < 1ms per update (achieved 65µs)

## Implementation Tasks

### Task 1: Create DeltaProcessor Core Structure
- Define DeltaProcessor struct with cache map and metrics
- Implement initialization with symbol-based cache organization
- Add metrics tracking for update frequency per field

### Task 2: Implement Delta Comparison Logic
- Build comparison algorithm for new vs cached state
- Identify changed fields only
- Return list of field names that changed
- Handle nested maps and complex types

### Task 3: Build Batched Redis Updates
- Implement HSET batching for multiple field updates
- Create transaction wrapper for atomic updates
- Optimize for minimal Redis round trips
- Handle Redis connection pooling

### Task 4: Add Update Frequency Tracking
- Track how often each field is updated
- Store metrics for hot/cold fields
- Provide frequency reporting interface
- Use for optimization decisions

### Task 5: Implement Performance Monitoring
- Add benchmarking for delta processing time
- Create performance metrics collection
- Log slow updates (> 1ms threshold)
- Provide performance reporting

### Task 6: Write Comprehensive Tests and Benchmarks
- Unit tests for delta comparison across various data types
- Tests for batched update operations
- Performance benchmarks targeting < 1ms
- Integration tests with Redis

## Technical Design

### DeltaProcessor Architecture

The DeltaProcessor maintains an in-memory cache of the last known state for each symbol and identifies only the fields that have changed between updates.

**Key Components:**
- Cache Map: Symbol -> (Field -> Value) mapping
- Comparison Engine: Detects changed fields
- Batch Operator: Combines updates into single HSET calls
- Metrics Tracker: Records update frequencies and timing

**Delta Processing Flow:**
1. Receive new state for symbol
2. Compare against cached state
3. Identify changed fields
4. Batch changed fields into single Redis HSET operation
5. Update in-memory cache
6. Track metrics
7. Return list of changed fields for logging/debugging

### Performance Optimization

- Keep cache in-memory to avoid cache lookups
- Use string field names as keys for quick comparison
- Batch updates to minimize Redis network overhead
- Pre-allocate maps/slices to reduce GC pressure

## Dependencies

- Story 11.1: Redis State Management (provides CoinState and StateManager)
- `internal/decision/coin_state.go`
- `internal/decision/state_manager.go`
- Redis client library (already in project)

## Test Requirements

1. **Unit tests for delta comparison**
   - Test identical values (no delta)
   - Test changed values (detect delta)
   - Test new fields (detect addition)
   - Test removed fields (detect removal)
   - Test various data types

2. **Tests for batched updates**
   - Verify single HSET call for multiple changes
   - Verify Redis key format consistency
   - Verify transaction handling

3. **Performance benchmarks**
   - Benchmark < 1ms for typical updates
   - Benchmark with various map sizes
   - Measure memory usage
   - Profile hot paths

## Files to Create

1. `internal/decision/delta_processor.go`
2. `internal/decision/delta_processor_test.go`

## Files to Modify

1. `internal/decision/state_manager.go` - Integrate DeltaProcessor into update flow
2. `internal/cache/settings_cache_service.go` - Use delta processor if applicable

## Notes

- This is part of the NEW decision engine (Epic 11)
- Builds on top of Story 11.1's StateManager and CoinState
- Focus on efficiency - minimize Redis operations while maintaining data consistency
- The delta processor should be reusable for other state update scenarios
- Consider using reflection carefully if needed for type-agnostic comparisons
- Ensure thread-safe access to cache map

---

## Change Log

| Date | Status | Notes |
|------|--------|-------|
| 2026-01-17 | ready-for-dev | Story created from epic |
| 2026-01-17 | in-progress | Implementation started |
| 2026-01-17 | review | Code review passed (8 issues, 5 fixed) |
| 2026-01-17 | done | QA trace passed - all 5 ACs verified, 65µs performance |
