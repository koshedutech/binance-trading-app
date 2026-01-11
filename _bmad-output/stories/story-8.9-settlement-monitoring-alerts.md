# Story 8.9: Settlement Monitoring & Alerts
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 9
**Story Points:** 5
**Priority:** P1

## User Story
As an admin, I want to be alerted when settlements fail and have a dashboard to monitor settlement health so that I can quickly identify and resolve issues before they impact billing.

## Acceptance Criteria
- [ ] Admin endpoint: `GET /api/admin/settlements/status`
- [ ] Returns list of all settlements with status breakdown
- [ ] Filter by status: all, failed, completed, retrying
- [ ] Show error details for failed settlements
- [ ] Manual retry button per failed settlement
- [ ] Email alert when settlement fails for >1 hour
- [ ] Metrics: success rate, average duration, failure count
- [ ] Admin alerts sent within 1 hour of persistent failure (NFR-6)
- [ ] Dashboard auto-refreshes every 30 seconds

## Technical Approach
Implement settlement monitoring with alerting:

**1. Monitoring Service:**
Background goroutine that checks for stalled settlements every 15 minutes:
```go
func (s *SettlementService) CheckForStalledSettlements() {
    ticker := time.NewTicker(15 * time.Minute)
    for range ticker.C {
        var failures []SettlementFailure
        s.db.Query(`
            SELECT user_id, summary_date, settlement_error, settlement_time
            FROM daily_mode_summaries
            WHERE settlement_status = 'failed'
            AND settlement_time < NOW() - INTERVAL '1 hour'
            AND alerted = false
        `).Scan(&failures)

        for _, failure := range failures {
            s.sendAdminAlert(failure)
            s.db.Exec(`
                UPDATE daily_mode_summaries
                SET alerted = true
                WHERE user_id = $1 AND summary_date = $2
            `, failure.UserID, failure.Date)
        }
    }
}
```

**2. Email Alerting:**
Send email to admin when settlement has been failed for >1 hour:
```
Subject: Settlement Failed: user@example.com - 2026-01-05

Settlement failed and needs manual intervention:

User: user@example.com (uuid-123)
Date: 2026-01-05
Error: Binance API timeout after 3 retries
Failed Since: 2026-01-06 01:30:00 (2.5 hours ago)

Retry: POST /api/admin/settlements/retry/uuid-123/2026-01-05
```

**3. Status API Response:**
```json
GET /api/admin/settlements/status?status=failed

{
  "settlements": [
    {
      "user_id": "uuid-123",
      "user_email": "user@example.com",
      "date": "2026-01-05",
      "status": "failed",
      "error": "Binance API timeout after 3 retries",
      "last_attempt": "2026-01-06T00:15:00Z",
      "failed_since_hours": 2.5
    }
  ],
  "summary": {
    "total_settlements": 150,
    "completed": 145,
    "failed": 3,
    "retrying": 2,
    "success_rate": 96.67
  }
}
```

**4. Admin Dashboard UI:**
- Failed settlements table with error details
- Retry button per failed settlement
- Success rate gauge
- Average settlement duration chart
- Filter by status dropdown
- Auto-refresh every 30 seconds

**5. Metrics Tracking:**
- Total settlements today
- Success rate (%)
- Failed count
- Retrying count
- Average settlement duration

## Dependencies
- **Blocked By:**
  - Story 8.8 (Settlement Failure Recovery - provides retry endpoint)
- **Blocks:**
  - Story 8.10 (Data Quality Validation - uses similar alerting)

## Files to Create/Modify
- `internal/settlement/monitoring.go` - Monitoring service and alerting logic
- `internal/api/handlers_admin.go` - Settlement status endpoint
- `internal/email/service.go` - Email alert service
- `web/src/pages/AdminSettlementStatus.tsx` - Admin dashboard component
- `web/src/services/adminApi.ts` - Admin API client
- `internal/api/server.go` - Route registration
- `main.go` - Start monitoring goroutine

## Testing Requirements
- Unit tests:
  - Test stalled settlement detection (>1 hour)
  - Test alert email generation
  - Test status aggregation calculations
  - Test filtering logic
  - Test success rate calculation
- Integration tests:
  - Test monitoring goroutine with database
  - Test email sending (mock SMTP)
  - Test status endpoint with various filters
  - Test retry button triggers settlement
- End-to-end tests:
  - Test complete alert flow:
    1. Settlement fails
    2. 1 hour passes
    3. Monitoring detects failure
    4. Email sent to admin
    5. Admin sees in dashboard
    6. Admin retries successfully
- UI tests:
  - Test dashboard rendering
  - Test auto-refresh (30s interval)
  - Test retry button functionality
  - Test status filtering

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Monitoring service running in background
- [ ] Email alerts sent for persistent failures
- [ ] Admin dashboard functional
- [ ] Status endpoint returns accurate data
- [ ] Retry button triggers settlement
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] E2E tests passing
- [ ] UI tests passing
- [ ] Auto-refresh working
- [ ] Email alerting tested (mock SMTP)
- [ ] Metrics accurate (success rate, avg duration)
- [ ] Documentation updated (admin guide, alerting process)
- [ ] PO acceptance received
