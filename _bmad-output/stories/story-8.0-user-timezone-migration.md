# Story 8.0: User Timezone Database Migration
**Epic:** Epic 8: Daily Settlement & Mode Analytics
**Sprint:** Sprint 8
**Story Points:** 2
**Priority:** P0

## User Story
As a system administrator, I want user timezone and settlement tracking columns added to the users table so that the settlement scheduler can run daily settlements at each user's midnight in their local timezone.

## Acceptance Criteria
- [ ] Migration adds `timezone VARCHAR(50) DEFAULT 'Asia/Kolkata'` to users table
- [ ] Migration adds `last_settlement_date DATE` to users table
- [ ] Migration is idempotent (checks if columns exist before adding)
- [ ] Default timezone is 'Asia/Kolkata' for existing users
- [ ] Migration includes rollback script
- [ ] Test migration on development database first
- [ ] Index created for efficient settlement queries

## Technical Approach
Create SQL migration file that uses PostgreSQL's `information_schema` to check for column existence before adding. This ensures the migration can be run multiple times without errors (idempotency).

**Migration File:** `internal/database/migrations/20260106_add_user_timezone_settlement.sql`

**Key Implementation Details:**
1. Use DO blocks with conditional logic to check column existence
2. Add columns with appropriate defaults
3. Create index on `last_settlement_date` for efficient scheduler queries
4. Add comments for documentation
5. Include rollback script in comments

**Verification Function:** Create Go function to verify migration success by querying `information_schema` and ensuring both columns exist.

## Dependencies
- **Blocked By:** None (foundation for Epic 8)
- **Blocks:** Stories 8.1-8.10 (all settlement stories require these columns)

## Files to Create/Modify
- `internal/database/migrations/20260106_add_user_timezone_settlement.sql` - SQL migration script
- `internal/database/migrations/verify_user_timezone.go` - Migration verification function

## Testing Requirements
- Unit tests:
  - Test migration script on fresh database
  - Test migration script on database where columns already exist (idempotency)
  - Test rollback script
- Integration tests:
  - Verify columns exist after migration
  - Verify default values applied to existing users
  - Verify index created successfully
  - Test VerifyUserTimezoneMigration() function returns no error

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Migration script created with idempotency checks
- [ ] Rollback script included in migration comments
- [ ] Verification function implemented
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Migration tested on development database
- [ ] Existing users have default timezone 'Asia/Kolkata'
- [ ] Index created successfully
- [ ] Documentation updated (migration README)
- [ ] PO acceptance received
