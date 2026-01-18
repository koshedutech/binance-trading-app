# Story 11.1: Redis State Management

**Epic:** 11 - Position Decision Engine
**Priority:** P0 (Critical)
**Status:** done
**Created:** 2026-01-17

## Goal

Build Redis-first state architecture for all monitored coins. This provides the foundation for the Position Decision Engine by storing real-time coin state including price, indicators, scores, regime, and blocking reasons.

## Acceptance Criteria

- [x] Redis hash per coin: `decision:user:{userID}:coin:{symbol}`
- [x] Store: price, indicators, scores, regime, blocking reasons
- [x] Delta updates only (update changed fields, not full recalculation)
- [x] TTL management for stale data cleanup
- [x] Atomic operations for consistency
- [x] Multi-user isolation via user_id in Redis keys

## Key Fields Structure

```
decision:user:{userID}:coin:BTCUSDT
├── price: 45000.00
├── regime: TRENDING
├── active_strategy: trend_following
├── adx: 28.5
├── rsi: 62.3
├── ema_9: 44850.00
├── ema_21: 44720.00
├── trend_1h: BULLISH
├── trend_15m: BULLISH
├── score_technical: 72
├── score_context: 65
├── score_llm: 58
├── score_history: 70
├── score_final: 68
├── blocking_reasons: []
├── last_updated: 1705312800000
└── decision: READY
```

## Implementation Tasks

### Task 1: Define CoinState Struct (coin_state.go)
- Create CoinState struct with all required fields
- Implement JSON serialization for complex fields (blocking_reasons)
- Add validation methods
- Define constants for field names and regimes

### Task 2: Implement Redis Hash Operations (coin_state.go)
- HSet for delta updates (only changed fields)
- HGet/HGetAll for reading state
- Atomic MULTI/EXEC operations
- TTL management with EXPIRE

### Task 3: Create StateManager Service (state_manager.go)
- Service struct with CacheService dependency
- GetCoinState - read full state for a symbol
- UpdateCoinState - delta update with changed fields only
- SetCoinDecision - update decision status
- CleanupStaleStates - remove expired data

### Task 4: Write Unit Tests (coin_state_test.go)
- Test Redis hash operations
- Test delta updates (only changed fields written)
- Test TTL expiration
- Test atomic operations
- Test multi-user isolation

### Task 5: Integration Points
- Follow existing CacheService patterns
- Use consistent error handling
- Add proper logging

## Technical Design

### Redis Key Pattern
```
decision:user:{userID}:coin:{symbol}
```

Example: `decision:user:abc123:coin:BTCUSDT`

### TTL Strategy
- Default TTL: 5 minutes for state data
- Refresh TTL on each update
- Separate cleanup routine for orphaned keys

### Delta Update Pattern
```go
// Only send fields that changed
updates := map[string]interface{}{
    "price": newPrice,
    "last_updated": time.Now().UnixMilli(),
}
stateManager.UpdateCoinState(ctx, userID, symbol, updates)
```

## Dependencies

- `internal/cache/cache_service.go` - Redis connection and operations
- go-redis/v9 - Redis client library

## Test Requirements

1. Unit tests for CoinState struct methods
2. Mock Redis tests for hash operations
3. Integration tests for delta updates
4. TTL verification tests
5. Multi-user isolation tests

## Files to Create

1. `internal/decision/coin_state.go` - CoinState struct and Redis operations
2. `internal/decision/state_manager.go` - StateManager service
3. `internal/decision/coin_state_test.go` - Unit tests

## Notes

- This is a NEW system separate from existing Ginie autopilot
- Do NOT modify any existing autopilot code
- Use user_id prefix for multi-user isolation
- Follow existing Redis patterns from cache_service.go

---

## Change Log

| Date | Status | Notes |
|------|--------|-------|
| 2026-01-17 | ready-for-dev | Story created with implementation plan |
| 2026-01-17 | in-progress | Implementation started |
| 2026-01-17 | review | Code review passed (10 issues fixed) |
| 2026-01-17 | done | QA trace passed - all 6 ACs verified |
