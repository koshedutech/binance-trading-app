-- Migration: Add paper_balance_usdt column to user_trading_configs
-- Purpose: Enable per-user customizable paper trading balances
-- Story: PAPER-001 - Database Migration
-- Author: Development Team
-- Date: 2026-01-02
--
-- Rollback Instructions:
-- To rollback this migration, execute the SQL in the "DOWN" section at the bottom of this file

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- Add column with default value (maintains current $10k hardcoded behavior)
ALTER TABLE user_trading_configs
ADD COLUMN IF NOT EXISTS paper_balance_usdt DECIMAL(20,8) NOT NULL DEFAULT 10000.00000000;

-- Backfill existing rows (explicit for clarity, though DEFAULT handles this)
UPDATE user_trading_configs
SET paper_balance_usdt = 10000.00000000
WHERE paper_balance_usdt IS NULL OR paper_balance_usdt = 0;

-- Add validation constraint (minimum $10, maximum $1,000,000)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_paper_balance_range'
    ) THEN
        ALTER TABLE user_trading_configs
        ADD CONSTRAINT check_paper_balance_range
        CHECK (paper_balance_usdt >= 10.0 AND paper_balance_usdt <= 1000000.0);
    END IF;
END $$;

-- Add comment to column for documentation
COMMENT ON COLUMN user_trading_configs.paper_balance_usdt IS 'Custom paper trading balance in USDT. Default 10000. Range: $10-$1M.';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- -- Remove constraint first
-- ALTER TABLE user_trading_configs
-- DROP CONSTRAINT IF EXISTS check_paper_balance_range;

-- -- Remove column
-- ALTER TABLE user_trading_configs
-- DROP COLUMN IF EXISTS paper_balance_usdt;
