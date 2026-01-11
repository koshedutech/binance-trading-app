# Story 8.8: Settlement Failure Recovery
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 9
**Story Points:** 5
**Priority:** P0

## User Story
As a system administrator, I want settlements to automatically retry on failure with exponential backoff so that temporary issues don't result in missing daily data.

## Acceptance Criteria
- [ ] Binance API failures: Retry 3 times with exponential backoff (5s, 15s, 45s)
- [ ] Database failures: Rollback transaction, retry once after 10 seconds
- [ ] Partial data scenarios: Mark settlement as 'failed', store error details
- [ ] Admin endpoint: `POST /api/admin/settlements/retry/:user_id/:date`
- [ ] Settlement status tracked: 'completed', 'failed', 'retrying'
- [ ] Alert admin if settlement fails for >1 hour (Story 8.9 dependency)
- [ ] Failed settlements visible in admin dashboard
- [ ] Settlement resilient to partial failures (NFR-4)
- [ ] Exponential backoff prevents API rate limiting

## Technical Approach
Implement robust error handling with retry logic:

**1. Error Classification:**
```go
func (s *SettlementService) isRetryableError(err error) bool {
    // Binance rate limit, timeout, connection errors
    if strings.Contains(err.Error(), "rate limit") ||
       strings.Contains(err.Error(), "timeout") ||
       strings.Contains(err.Error(), "connection refused") {
        return true
    }

    // Database deadlock, connection errors
    if strings.Contains(err.Error(), "deadlock") ||
       strings.Contains(err.Error(), "connection") {
        return true
    }

    return false
}
```

**2. Retry Strategy:**
- **Binance API errors:** 3 retries with exponential backoff (5s, 15s, 45s)
- **Database errors:** 1 retry after 10 seconds with transaction rollback
- **Non-retryable errors:** Immediate failure, mark as 'failed'

**3. Settlement Status Tracking:**
Update `daily_mode_summaries.settlement_status`:
- `completed` - Settlement succeeded
- `failed` - All retries exhausted or non-retryable error
- `retrying` - Currently in retry loop

**4. Error Logging:**
Store detailed error information:
- Phase where error occurred (snapshot, aggregate, store)
- Attempt number
- Error message
- Timestamp

**5. Admin Retry Endpoint:**
```
POST /api/admin/settlements/retry/:user_id/:date
```
Allows manual retry of failed settlements.

**Implementation Details:**
```go
func (s *SettlementService) RunDailySettlementWithRetry(userID string, date time.Time) error {
    maxRetries := 3
    backoff := []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second}

    for attempt := 0; attempt < maxRetries; attempt++ {
        err := s.runSettlement(userID, date)
        if err == nil {
            return nil
        }

        // Log error
        s.logSettlementError(SettlementError{
            UserID:    userID,
            Date:      date,
            Phase:     s.identifyErrorPhase(err),
            Attempt:   attempt + 1,
            Error:     err,
            Timestamp: time.Now(),
        })

        // Check if retryable
        if !s.isRetryableError(err) {
            s.markSettlementFailed(userID, date, err)
            return err
        }

        // Wait before retry
        if attempt < maxRetries-1 {
            time.Sleep(backoff[attempt])
        }
    }

    // All retries exhausted
    s.markSettlementFailed(userID, date, errors.New("max retries exceeded"))
    s.alertAdmin(userID, date)
    return errors.New("settlement failed after retries")
}
```

## Dependencies
- **Blocked By:**
  - Story 8.3 (Daily Summary Storage - settlement_status column)
- **Blocks:**
  - Story 8.9 (Settlement Monitoring - uses retry endpoint)

## Files to Create/Modify
- `internal/settlement/error_handling.go` - Retry logic and error classification
- `internal/settlement/service.go` - Integration with main settlement flow
- `internal/api/handlers_admin.go` - Admin retry endpoint
- `internal/database/repository_daily_summaries.go` - Mark settlement status
- `internal/api/server.go` - Route registration
- `web/src/services/adminApi.ts` - Admin API client for retry

## Testing Requirements
- Unit tests:
  - Test error classification (retryable vs non-retryable)
  - Test exponential backoff timing
  - Test max retries enforcement
  - Test settlement status updates
  - Test error logging
- Integration tests:
  - Test retry with mock Binance failures
  - Test database transaction rollback
  - Test admin retry endpoint
  - Test concurrent retries (different users)
- End-to-end tests:
  - Test complete failure recovery cycle:
    1. Settlement fails (simulated timeout)
    2. Retry 3 times
    3. Mark as failed
    4. Admin retries manually
    5. Settlement succeeds
- Performance tests:
  - Verify backoff delays accurate
  - Test retry doesn't block other settlements

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Retry logic implemented with exponential backoff
- [ ] Error classification functional
- [ ] Settlement status tracking working
- [ ] Admin retry endpoint functional
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] E2E tests passing
- [ ] Retry prevents API rate limiting
- [ ] Database transactions rolled back on failure
- [ ] Error details logged for debugging
- [ ] Documentation updated (error handling, retry process)
- [ ] PO acceptance received
