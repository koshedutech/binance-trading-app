-- Migration 029: Add last_settlement_date to users (Epic 8 Story 8.0)
-- Tracks the last date for which daily settlement was completed for each user
-- Used by the settlement scheduler to avoid duplicate settlements

-- Add last_settlement_date column to users table
-- NULL means settlement has never run for this user
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_settlement_date DATE;

-- Create index for efficient settlement scheduler queries
-- The scheduler needs to find all users needing settlement (last_settlement_date < today in user timezone)
CREATE INDEX IF NOT EXISTS idx_users_last_settlement ON users(last_settlement_date);

-- Add comment for documentation
COMMENT ON COLUMN users.last_settlement_date IS 'Last date for which daily settlement was completed (in user timezone). NULL means never settled.';

-- Rollback script (keep in comments for reference):
-- ALTER TABLE users DROP COLUMN IF EXISTS last_settlement_date;
-- DROP INDEX IF EXISTS idx_users_last_settlement;
