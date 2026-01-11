# Story 8.10: Data Quality Validation
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 9
**Story Points:** 5
**Priority:** P1

## User Story
As an admin, I want settlement data to be validated for quality and anomalies flagged for review so that billing calculations are accurate and suspicious data is caught early.

## Acceptance Criteria
- [ ] Win rate validation: Must be between 0-100%
- [ ] Total P&L validation: Flag if outside -$10,000 to +$10,000 range (configurable)
- [ ] Trade count validation: Flag if >500 trades/day (suspicious)
- [ ] Unrealized P&L validation: Compare with Binance API snapshot
- [ ] Mark anomalies with `data_quality_flag` in database
- [ ] Admin review queue for flagged settlements
- [ ] Manual approval/rejection workflow
- [ ] Data quality validation catches >95% of anomalies (NFR-7)
- [ ] Win/Loss count consistency check (wins + losses = total trades)

## Technical Approach
Implement validation service that runs before storing settlement data:

**1. Validation Rules:**
```go
type ValidationResult struct {
    IsValid  bool
    Errors   []string    // Hard failures (reject settlement)
    Warnings []string    // Anomalies (flag for review)
}

func (s *SettlementService) ValidateSettlementData(summary *DailyModeSummary) ValidationResult {
    result := ValidationResult{IsValid: true}

    // HARD ERRORS (reject settlement)
    // Win rate validation
    if summary.WinRate < 0 || summary.WinRate > 100 {
        result.Errors = append(result.Errors,
            fmt.Sprintf("Invalid win rate: %.2f%%", summary.WinRate))
        result.IsValid = false
    }

    // Win/loss count consistency
    if summary.WinCount + summary.LossCount != summary.TradeCount {
        result.Errors = append(result.Errors,
            "Win + Loss count doesn't match total trade count")
        result.IsValid = false
    }

    // WARNINGS (flag for admin review)
    // P&L bounds check
    if summary.TotalPnl < -10000 || summary.TotalPnl > 10000 {
        result.Warnings = append(result.Warnings,
            fmt.Sprintf("P&L outside normal range: $%.2f", summary.TotalPnl))
    }

    // Trade count check
    if summary.TradeCount > 500 {
        result.Warnings = append(result.Warnings,
            fmt.Sprintf("High trade count: %d", summary.TradeCount))
    }

    // Unrealized P&L consistency
    if summary.UnrealizedPnl != 0 {
        binanceUnrealized, err := s.binance.GetUnrealizedPnl(summary.UserID)
        if err == nil {
            diff := math.Abs(summary.UnrealizedPnl - binanceUnrealized)
            if diff > 100 { // $100 tolerance
                result.Warnings = append(result.Warnings,
                    fmt.Sprintf("Unrealized P&L mismatch: Stored=%.2f, Binance=%.2f",
                        summary.UnrealizedPnl, binanceUnrealized))
            }
        }
    }

    return result
}
```

**2. Validation Integration:**
```go
func (s *SettlementService) runSettlement(userID string, date time.Time) error {
    // ... settlement logic ...

    // Validate before storing
    validation := s.ValidateSettlementData(summary)
    if !validation.IsValid {
        return fmt.Errorf("validation failed: %v", validation.Errors)
    }

    // Flag if warnings present
    if len(validation.Warnings) > 0 {
        summary.DataQualityFlag = true
        summary.DataQualityNotes = strings.Join(validation.Warnings, "; ")
    }

    // Store in database
    return s.db.SaveDailySummary(summary)
}
```

**3. Admin Review Queue:**
```
GET /api/admin/settlements/review-queue

Returns all settlements where:
- data_quality_flag = true
- reviewed_at IS NULL
```

**4. Admin Approval Workflow:**
```
POST /api/admin/settlements/approve/:id
POST /api/admin/settlements/reject/:id

Approve: Sets reviewed_by and reviewed_at
Reject: Marks settlement for re-run or manual correction
```

**5. Database Schema Updates:**
```sql
ALTER TABLE daily_mode_summaries ADD COLUMN IF NOT EXISTS
    data_quality_flag BOOLEAN DEFAULT false;

ALTER TABLE daily_mode_summaries ADD COLUMN IF NOT EXISTS
    data_quality_notes TEXT;

ALTER TABLE daily_mode_summaries ADD COLUMN IF NOT EXISTS
    reviewed_by UUID REFERENCES users(id);

ALTER TABLE daily_mode_summaries ADD COLUMN IF NOT EXISTS
    reviewed_at TIMESTAMP;
```

**6. Admin Review UI:**
- Table of flagged settlements
- Show data quality notes (warnings)
- Approve/Reject buttons
- Filter by review status
- Display user email, date, mode, P&L, flags

## Dependencies
- **Blocked By:**
  - Story 8.3 (Daily Summary Storage - data_quality_flag column)
  - Story 8.8 (Settlement Failure Recovery - validation before storage)
- **Blocks:** None

## Files to Create/Modify
- `internal/settlement/validation.go` - Validation service and rules
- `internal/settlement/service.go` - Integration with settlement flow
- `internal/api/handlers_admin.go` - Review queue and approval endpoints
- `internal/database/repository_daily_summaries.go` - Review queue queries
- `internal/database/migrations/20260106_add_data_quality_columns.sql` - Schema update
- `web/src/pages/AdminSettlementReviewQueue.tsx` - Review queue UI
- `web/src/services/adminApi.ts` - Admin API client
- `internal/api/server.go` - Route registration

## Testing Requirements
- Unit tests:
  - Test each validation rule independently
  - Test validation with valid data (no errors, no warnings)
  - Test validation with invalid win rate
  - Test validation with P&L out of bounds
  - Test validation with high trade count
  - Test validation with unrealized P&L mismatch
  - Test validation with win/loss count mismatch
  - Test configurable thresholds
- Integration tests:
  - Test validation prevents invalid data from being stored
  - Test flagging workflow (warnings set data_quality_flag)
  - Test review queue endpoint
  - Test approval endpoint (sets reviewed_by, reviewed_at)
  - Test rejection endpoint
- End-to-end tests:
  - Test complete validation flow:
    1. Settlement produces anomalous data
    2. Validation flags as warning
    3. Settlement stored with data_quality_flag = true
    4. Appears in admin review queue
    5. Admin approves
    6. Flag remains but reviewed_at set
- Anomaly detection tests:
  - Test >95% anomaly detection rate with synthetic data

## Definition of Done
- [ ] All acceptance criteria met
- [ ] All validation rules implemented
- [ ] Hard errors reject settlement
- [ ] Warnings flag for admin review
- [ ] Database columns added
- [ ] Review queue functional
- [ ] Approval/rejection workflow working
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] E2E tests passing
- [ ] Anomaly detection >95% accuracy
- [ ] Configurable thresholds (P&L bounds, trade count)
- [ ] UI displays flagged settlements correctly
- [ ] Documentation updated (validation rules, admin guide)
- [ ] PO acceptance received
