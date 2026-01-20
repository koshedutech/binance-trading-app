-- Migration: 043_user_ginie_morning_auto_block
-- Description: Add morning auto-block settings to user_ginie_settings table
-- Purpose: Story 9.12 Phase 4 - Migrate morning auto-block scheduler configuration to database
-- Date: 2026-01-20

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- Add morning auto-block columns to user_ginie_settings table
ALTER TABLE user_ginie_settings
ADD COLUMN IF NOT EXISTS morning_auto_block_enabled BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN IF NOT EXISTS morning_auto_block_hour_utc INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS morning_auto_block_min_utc INTEGER NOT NULL DEFAULT 5;

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate morning_auto_block_hour_utc (0-23)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_morning_auto_block_hour_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_morning_auto_block_hour_range
        CHECK (morning_auto_block_hour_utc >= 0 AND morning_auto_block_hour_utc <= 23);
    END IF;
END $$;

-- Validate morning_auto_block_min_utc (0-59)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_morning_auto_block_min_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_morning_auto_block_min_range
        CHECK (morning_auto_block_min_utc >= 0 AND morning_auto_block_min_utc <= 59);
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON COLUMN user_ginie_settings.morning_auto_block_enabled IS 'Enable automatic morning blocking of worst performers (default: false)';
COMMENT ON COLUMN user_ginie_settings.morning_auto_block_hour_utc IS 'Hour in UTC to run morning auto-block (0-23, default: 0)';
COMMENT ON COLUMN user_ginie_settings.morning_auto_block_min_utc IS 'Minute in UTC to run morning auto-block (0-59, default: 5)';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- ALTER TABLE user_ginie_settings DROP CONSTRAINT IF EXISTS check_ginie_morning_auto_block_hour_range;
-- ALTER TABLE user_ginie_settings DROP CONSTRAINT IF EXISTS check_ginie_morning_auto_block_min_range;
-- ALTER TABLE user_ginie_settings DROP COLUMN IF EXISTS morning_auto_block_enabled;
-- ALTER TABLE user_ginie_settings DROP COLUMN IF EXISTS morning_auto_block_hour_utc;
-- ALTER TABLE user_ginie_settings DROP COLUMN IF EXISTS morning_auto_block_min_utc;
