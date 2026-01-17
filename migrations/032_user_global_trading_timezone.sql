-- Migration: 032_user_global_trading_timezone
-- Description: Add timezone and timezone_offset columns to user_global_trading table
-- Purpose: Allow users to set their preferred timezone for P&L display, countdowns, and date ranges
-- Date: 2026-01-17

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- Add timezone column (IANA timezone name, e.g., 'Asia/Kolkata', 'UTC')
ALTER TABLE user_global_trading
ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'UTC';

-- Add timezone_offset column (UTC offset, e.g., '+05:30', '-05:00')
ALTER TABLE user_global_trading
ADD COLUMN IF NOT EXISTS timezone_offset VARCHAR(10) DEFAULT '+00:00';

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON COLUMN user_global_trading.timezone IS 'IANA timezone name for P&L display and countdowns (e.g., Asia/Kolkata, UTC, America/New_York)';
COMMENT ON COLUMN user_global_trading.timezone_offset IS 'UTC offset in format +HH:MM or -HH:MM (e.g., +05:30, -05:00, +00:00)';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- ALTER TABLE user_global_trading DROP COLUMN IF EXISTS timezone;
-- ALTER TABLE user_global_trading DROP COLUMN IF EXISTS timezone_offset;
