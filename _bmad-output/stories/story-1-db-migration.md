# Story 1: Database Migration - Add Paper Balance Column

**Story ID:** PAPER-001
**Epic:** Editable Paper Trading Balance
**Priority:** Critical (Blocker for all other stories)
**Estimated Effort:** 2 hours
**Author:** Bob (Scrum Master)
**Status:** Ready for Development

---

## Description

Add `paper_balance_usdt` column to the `user_trading_configs` table to store per-user, per-trading-type paper trading balances. This migration provides the database foundation for the entire feature.

---

## User Story

> As a developer,
> I need a database column to store custom paper balances,
> So that the application can persist user-configured paper trading balances across sessions.

---

## Acceptance Criteria

### AC1.1: Migration Script Created
- [ ] Create migration file: `migrations/YYYYMMDDHHMMSS_add_paper_balance_column.sql`
- [ ] Migration adds column: `paper_balance_usdt DECIMAL(20,8) NOT NULL DEFAULT 10000.0`
- [ ] Column is added to `user_trading_configs` table
- [ ] Default value ensures backward compatibility (current hardcoded value)

### AC1.2: Data Validation Constraints
- [ ] Add CHECK constraint: `paper_balance_usdt >= 10.0 AND paper_balance_usdt <= 1000000.0`
- [ ] Constraint named: `check_paper_balance_range`
- [ ] Constraint prevents invalid balances at database level

### AC1.3: Existing Data Backfill
- [ ] All existing rows in `user_trading_configs` have `paper_balance_usdt = 10000.0` after migration
- [ ] No NULL values exist in `paper_balance_usdt` column
- [ ] Verify backfill with SQL query: `SELECT COUNT(*) FROM user_trading_configs WHERE paper_balance_usdt IS NULL` returns 0

### AC1.4: Rollback Script Tested
- [ ] Migration DOWN script removes `check_paper_balance_range` constraint
- [ ] Migration DOWN script removes `paper_balance_usdt` column
- [ ] Test rollback on development database successfully
- [ ] Verify table returns to original schema after rollback

### AC1.5: Migration Documentation
- [ ] Add comment to migration file explaining purpose
- [ ] Document rollback procedure in migration file header
- [ ] Update database schema documentation (if exists)

---

## Technical Implementation Notes

### Migration File Structure

**File Location:** `migrations/YYYYMMDDHHMMSS_add_paper_balance_column.sql`

```sql
-- Migration: Add paper_balance_usdt column to user_trading_configs
-- Purpose: Enable per-user customizable paper trading balances
-- Author: Developer Team
-- Date: 2026-01-02
--
-- Rollback: Execute the DOWN section to revert changes

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- Add column with default value
ALTER TABLE user_trading_configs
ADD COLUMN paper_balance_usdt DECIMAL(20,8) NOT NULL DEFAULT 10000.0;

-- Backfill existing rows (explicit for clarity, though default handles this)
UPDATE user_trading_configs
SET paper_balance_usdt = 10000.0
WHERE paper_balance_usdt IS NULL;

-- Add validation constraint
ALTER TABLE user_trading_configs
ADD CONSTRAINT check_paper_balance_range
CHECK (paper_balance_usdt >= 10.0 AND paper_balance_usdt <= 1000000.0);

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================

-- Remove constraint first
ALTER TABLE user_trading_configs
DROP CONSTRAINT IF EXISTS check_paper_balance_range;

-- Remove column
ALTER TABLE user_trading_configs
DROP COLUMN IF EXISTS paper_balance_usdt;
```

### Database Type Justification

**DECIMAL(20,8):**
- Matches Binance API precision for USDT balances
- 20 total digits, 8 decimal places
- Prevents floating-point precision errors
- Supports balances from $0.00000001 to $999,999,999,999.99999999

**NOT NULL with DEFAULT:**
- Ensures data integrity (no NULL handling needed)
- Default 10000.0 maintains current behavior
- Simplifies application logic (no NULL checks required)

### Validation Constraint Ranges

- **Minimum:** $10.00 (prevents trivial/test balances that aren't realistic)
- **Maximum:** $1,000,000.00 (MVP cap, expandable in future versions)
- **Enforced at:** Database level for bulletproof validation

---

## Testing Requirements

### Test 1: Migration Execution
```bash
# Run migration
psql -U postgres -d binance_bot_dev -f migrations/YYYYMMDDHHMMSS_add_paper_balance_column.sql

# Verify column exists
psql -U postgres -d binance_bot_dev -c "\d user_trading_configs"

# Expected output includes:
# paper_balance_usdt | numeric(20,8) | not null | 10000.0
```

### Test 2: Default Value Application
```sql
-- Insert new row without specifying paper_balance_usdt
INSERT INTO user_trading_configs (user_id, trading_type, dry_run_mode)
VALUES (999, 'spot', true);

-- Verify default applied
SELECT paper_balance_usdt FROM user_trading_configs WHERE user_id = 999;
-- Expected: 10000.00000000
```

### Test 3: Constraint Enforcement
```sql
-- Test minimum boundary (should fail)
UPDATE user_trading_configs
SET paper_balance_usdt = 5.0
WHERE user_id = 1;
-- Expected: ERROR - violates check constraint "check_paper_balance_range"

-- Test maximum boundary (should fail)
UPDATE user_trading_configs
SET paper_balance_usdt = 1000001.0
WHERE user_id = 1;
-- Expected: ERROR - violates check constraint "check_paper_balance_range"

-- Test valid value (should succeed)
UPDATE user_trading_configs
SET paper_balance_usdt = 5000.0
WHERE user_id = 1;
-- Expected: SUCCESS
```

### Test 4: Rollback Verification
```bash
# Execute rollback script
psql -U postgres -d binance_bot_dev -c "
ALTER TABLE user_trading_configs DROP CONSTRAINT IF EXISTS check_paper_balance_range;
ALTER TABLE user_trading_configs DROP COLUMN IF EXISTS paper_balance_usdt;
"

# Verify column removed
psql -U postgres -d binance_bot_dev -c "\d user_trading_configs"
# Expected: paper_balance_usdt column NOT present
```

### Test 5: Existing Data Backfill
```sql
-- After migration, verify all rows have default value
SELECT COUNT(*) as total_rows,
       COUNT(paper_balance_usdt) as non_null_rows,
       COUNT(CASE WHEN paper_balance_usdt = 10000.0 THEN 1 END) as default_value_rows
FROM user_trading_configs;

-- Expected: total_rows = non_null_rows = default_value_rows
```

---

## Dependencies

### Prerequisites
- PostgreSQL database running (development: port 5433)
- Database migration tool configured (e.g., golang-migrate, Flyway, or manual psql)
- Database backup taken before migration

### Blocks
- **Story 2**: Backend API endpoints (cannot implement without database column)
- **Story 3**: Trading logic update (cannot read balance from DB without column)
- **Story 4**: Frontend UI (cannot display balance without backend support)

---

## Deployment Notes

### Development Environment
```bash
# Connect to development database
docker exec -it binance-bot-postgres psql -U postgres -d binance_bot_dev

# Run migration
\i /path/to/migrations/YYYYMMDDHHMMSS_add_paper_balance_column.sql

# Verify
\d user_trading_configs
```

### Production Environment
1. **Pre-Deployment:**
   - Take full database backup
   - Schedule maintenance window (estimated 2 minutes downtime)
   - Notify users of brief service interruption

2. **Deployment:**
   - Stop application containers
   - Run migration script
   - Verify migration success
   - Restart application containers

3. **Post-Deployment Verification:**
   - Query sample rows: `SELECT * FROM user_trading_configs LIMIT 10;`
   - Confirm all rows have `paper_balance_usdt = 10000.0`
   - Monitor application logs for database errors

4. **Rollback Procedure (if needed):**
   - Stop application
   - Execute rollback script
   - Restore from backup if rollback fails
   - Restart application

---

## Definition of Done

- [ ] Migration script created and committed to repository
- [ ] Migration executed successfully on development database
- [ ] All 5 test cases pass
- [ ] Rollback tested successfully on development database
- [ ] Column visible in database schema: `\d user_trading_configs`
- [ ] No NULL values in `paper_balance_usdt` column
- [ ] CHECK constraint enforced (tested with invalid values)
- [ ] Code review approved
- [ ] Migration documented in project changelog

---

## Notes for Developer

- **Migration Tool:** Check if project uses golang-migrate, Flyway, or custom migration system
- **File Naming:** Follow project's migration file naming convention (timestamp format)
- **Decimal Handling:** In Go code (Story 2), use `github.com/shopspring/decimal` library to match PostgreSQL precision
- **Docker:** If running database in Docker, ensure migration file is accessible inside container

---

## Related Stories

- **Story 2:** Backend API endpoints (depends on this migration)
- **Story 3:** Update trading logic (depends on this migration)
- **Story 4:** Frontend UI (indirectly depends via Story 2)

---

## Approval Sign-Off

- **Scrum Master (Bob)**: ✅ Story Ready for Development
- **Developer (Amelia)**: _Pending Assignment_
- **Test Architect (Murat)**: _Pending Test Review_
