# Story 7.2: Daily Sequence Storage in Redis
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 3
**Priority:** P0

## User Story
As a trading system, I want atomic daily sequence counters in Redis with timezone-aware reset so that each order gets a unique sequence number that resets at midnight.

## Acceptance Criteria
- [ ] Redis key format: `user:{user_id}:sequence:{YYYYMMDD}`
- [ ] Atomic INCR operation (no duplicates under concurrent load)
- [ ] Key TTL: 48 hours (automatic cleanup)
- [ ] Reset to 1 at user's timezone midnight (not UTC)
- [ ] Handle sequence rollover (99999 → 00001 with date change)
- [ ] `IncrementDailySequence(userID, now)` method in CacheService
- [ ] `GetCurrentSequence(userID, now)` method for monitoring
- [ ] Graceful fallback if Redis unavailable (logged warning)

## Technical Approach
Extend the `CacheService` (from Epic 6) with sequence management:

1. **Key Structure**:
   - Format: `user:{user_id}:sequence:{YYYYMMDD}`
   - Example: `user:123:sequence:20260106`
   - Date in YYYYMMDD format for consistent sorting

2. **Atomic Increment**:
   - Use Redis INCR command (atomic operation)
   - Set TTL on first increment (seq == 1)
   - TTL prevents accumulation of old keys

3. **Timezone Awareness**:
   - Accept `time.Time` in user's timezone
   - Extract date from provided time, not `time.Now()`
   - Allows midnight rollover testing with custom times

4. **Sequence Limits**:
   - Maximum: 99999 (5 digits)
   - If exceeded, log warning and continue (6-digit fallback)
   - Or use Redis SET to reset to 1 with new date

5. **Error Handling**:
   - Return error if Redis unavailable
   - Caller (Story 7.1) handles fallback ID generation

## Dependencies
- **Blocked By:**
  - Story 6.1: Redis Container Setup
  - Story 6.2: CacheService Implementation
  - Story 6.3: Redis Integration in main.go
- **Blocks:**
  - Story 7.1: Client Order ID Generation (needs IncrementDailySequence)
  - Story 7.3: Order Chain Tracking
  - Story 7.8: Redis Fallback Testing

## Files to Create/Modify

### Files to Create:
- `internal/cache/sequence.go` - IncrementDailySequence and GetCurrentSequence methods

### Files to Modify:
- `internal/cache/cache_service.go` - Add sequence methods to CacheService interface
- `internal/cache/cache_service_test.go` - Add sequence increment tests

## Testing Requirements

### Unit Tests:
- Test sequence increment returns sequential numbers (1, 2, 3...)
- Test atomic increment under concurrent load (no duplicates)
- Test TTL set to 48 hours on first increment
- Test timezone-aware date key generation
- Test midnight rollover (23:59 → 00:01 different dates)
- Test year boundary (Dec 31 → Jan 1)
- Test GetCurrentSequence returns correct value
- Test error handling when Redis unavailable

### Integration Tests:
- Test Redis container connection
- Test multiple users have independent sequences
- Test sequence persists across service restarts
- Test TTL expiration after 48 hours
- Test concurrent requests from same user (100+ parallel)

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing (concurrent load test)
- [ ] Documentation updated (Redis key schema documented)
- [ ] PO acceptance received
- [ ] Verified no duplicate sequences under load
- [ ] Timezone rollover tested with Asia/Kolkata
