-- Migration: 022_remove_scalp_reentry_allocation
-- Description: Remove scalp_reentry allocation columns (scalp_reentry uses scalp mode allocation)
-- Reason: scalp_reentry is position optimization, not a separate trading mode
-- Date: 2026-01-09

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- Drop constraints that reference these columns first
DO $$
BEGIN
    -- Drop the check constraint if it exists
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_capital_allocation_max_positions_range'
    ) THEN
        ALTER TABLE user_capital_allocation DROP CONSTRAINT check_capital_allocation_max_positions_range;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_capital_allocation_max_usd_per_position_range'
    ) THEN
        ALTER TABLE user_capital_allocation DROP CONSTRAINT check_capital_allocation_max_usd_per_position_range;
    END IF;
END $$;

-- Drop the scalp_reentry columns
ALTER TABLE user_capital_allocation
DROP COLUMN IF EXISTS max_scalp_reentry_positions;

ALTER TABLE user_capital_allocation
DROP COLUMN IF EXISTS max_scalp_reentry_usd_per_position;

-- Recreate constraints without scalp_reentry fields
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_capital_allocation_max_positions_range'
    ) THEN
        ALTER TABLE user_capital_allocation
        ADD CONSTRAINT check_capital_allocation_max_positions_range
        CHECK (
            max_ultra_fast_positions >= 1 AND max_ultra_fast_positions <= 100 AND
            max_scalp_positions >= 1 AND max_scalp_positions <= 100 AND
            max_swing_positions >= 1 AND max_swing_positions <= 100 AND
            max_position_positions >= 1 AND max_position_positions <= 100
        );
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_capital_allocation_max_usd_per_position_range'
    ) THEN
        ALTER TABLE user_capital_allocation
        ADD CONSTRAINT check_capital_allocation_max_usd_per_position_range
        CHECK (
            max_ultra_fast_usd_per_position >= 10.0 AND max_ultra_fast_usd_per_position <= 100000.0 AND
            max_scalp_usd_per_position >= 10.0 AND max_scalp_usd_per_position <= 100000.0 AND
            max_swing_usd_per_position >= 10.0 AND max_swing_usd_per_position <= 100000.0 AND
            max_position_usd_per_position >= 10.0 AND max_position_usd_per_position <= 100000.0
        );
    END IF;
END $$;

-- Update comments
COMMENT ON TABLE user_capital_allocation IS 'Per-user capital allocation configuration. scalp_reentry uses scalp mode allocation (not separate).';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- To rollback, uncomment and run:
-- ALTER TABLE user_capital_allocation ADD COLUMN max_scalp_reentry_positions INTEGER NOT NULL DEFAULT 3;
-- ALTER TABLE user_capital_allocation ADD COLUMN max_scalp_reentry_usd_per_position DECIMAL(20,8) NOT NULL DEFAULT 400.00000000;
