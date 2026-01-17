-- Migration: 033_init_user_global_trading
-- Description: Initialize user_global_trading records for existing users
-- Purpose: Ensure all users have a user_global_trading record with default values
-- Date: 2026-01-17

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- Insert user_global_trading record for each user that doesn't have one
-- Uses default values from default-settings.json
INSERT INTO user_global_trading (
    user_id,
    risk_level,
    max_usd_allocation,
    profit_reinvest_percent,
    profit_reinvest_risk_level,
    timezone,
    timezone_offset
)
SELECT
    u.id,
    'moderate',           -- risk_level from default-settings.json
    2500.00000000,        -- max_usd_allocation from default-settings.json
    50.00,                -- profit_reinvest_percent from default-settings.json
    'aggressive',         -- profit_reinvest_risk_level from default-settings.json
    'Asia/Kolkata',       -- timezone from default-settings.json
    '+05:30'              -- timezone_offset from default-settings.json
FROM users u
WHERE NOT EXISTS (
    SELECT 1 FROM user_global_trading ugt WHERE ugt.user_id = u.id
);

-- Log how many records were created
DO $$
DECLARE
    inserted_count INTEGER;
BEGIN
    GET DIAGNOSTICS inserted_count = ROW_COUNT;
    RAISE NOTICE 'Initialized user_global_trading for % users', inserted_count;
END $$;

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- This migration only inserts missing records, rollback would delete them:
-- DELETE FROM user_global_trading WHERE created_at > '2026-01-17';
