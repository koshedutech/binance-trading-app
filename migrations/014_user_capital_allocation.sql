-- Migration: 014_user_capital_allocation
-- Description: Create per-user capital allocation configuration table
-- Purpose: Replace autopilot_settings.json ModeAllocationConfig section with database storage
-- Date: 2026-01-08

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_capital_allocation
-- ====================================================================================
-- Stores per-user capital allocation settings across 4 trading modes
-- Note: scalp_reentry is NOT a trading mode, it's a position optimization method
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_capital_allocation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Capital allocation percentages (must sum to 100%)
    ultra_fast_percent DECIMAL(5,2) NOT NULL DEFAULT 20.00,
    scalp_percent DECIMAL(5,2) NOT NULL DEFAULT 30.00,
    swing_percent DECIMAL(5,2) NOT NULL DEFAULT 35.00,
    position_percent DECIMAL(5,2) NOT NULL DEFAULT 15.00,

    -- Max positions per mode
    max_ultra_fast_positions INTEGER NOT NULL DEFAULT 3,
    max_scalp_positions INTEGER NOT NULL DEFAULT 4,
    max_scalp_reentry_positions INTEGER NOT NULL DEFAULT 3,
    max_swing_positions INTEGER NOT NULL DEFAULT 3,
    max_position_positions INTEGER NOT NULL DEFAULT 2,

    -- Max USD per position per mode
    max_ultra_fast_usd_per_position DECIMAL(20,8) NOT NULL DEFAULT 200.00000000,
    max_scalp_usd_per_position DECIMAL(20,8) NOT NULL DEFAULT 300.00000000,
    max_scalp_reentry_usd_per_position DECIMAL(20,8) NOT NULL DEFAULT 400.00000000,
    max_swing_usd_per_position DECIMAL(20,8) NOT NULL DEFAULT 500.00000000,
    max_position_usd_per_position DECIMAL(20,8) NOT NULL DEFAULT 750.00000000,

    -- Dynamic rebalancing (optional)
    allow_dynamic_rebalance BOOLEAN NOT NULL DEFAULT false,
    rebalance_threshold_pct DECIMAL(5,2) NOT NULL DEFAULT 20.00,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one allocation config per user
    CONSTRAINT unique_user_capital_allocation UNIQUE(user_id)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_capital_allocation_user_id ON user_capital_allocation(user_id);

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_capital_allocation_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_capital_allocation_updated_at ON user_capital_allocation;

CREATE TRIGGER trigger_user_capital_allocation_updated_at
    BEFORE UPDATE ON user_capital_allocation
    FOR EACH ROW
    EXECUTE FUNCTION update_user_capital_allocation_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate allocation percentages sum to 100%
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_capital_allocation_sum_100'
    ) THEN
        ALTER TABLE user_capital_allocation
        ADD CONSTRAINT check_capital_allocation_sum_100
        CHECK (
            ABS((ultra_fast_percent + scalp_percent + swing_percent + position_percent) - 100.0) < 0.01
        );
    END IF;
END $$;

-- Validate percentage ranges (0-100%)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_capital_allocation_percent_range'
    ) THEN
        ALTER TABLE user_capital_allocation
        ADD CONSTRAINT check_capital_allocation_percent_range
        CHECK (
            ultra_fast_percent >= 0 AND ultra_fast_percent <= 100 AND
            scalp_percent >= 0 AND scalp_percent <= 100 AND
            swing_percent >= 0 AND swing_percent <= 100 AND
            position_percent >= 0 AND position_percent <= 100
        );
    END IF;
END $$;

-- Validate max positions (1-100)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_capital_max_positions_range'
    ) THEN
        ALTER TABLE user_capital_allocation
        ADD CONSTRAINT check_capital_max_positions_range
        CHECK (
            max_ultra_fast_positions >= 1 AND max_ultra_fast_positions <= 100 AND
            max_scalp_positions >= 1 AND max_scalp_positions <= 100 AND
            max_scalp_reentry_positions >= 1 AND max_scalp_reentry_positions <= 100 AND
            max_swing_positions >= 1 AND max_swing_positions <= 100 AND
            max_position_positions >= 1 AND max_position_positions <= 100
        );
    END IF;
END $$;

-- Validate max USD per position ($10 - $100,000)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_capital_max_usd_range'
    ) THEN
        ALTER TABLE user_capital_allocation
        ADD CONSTRAINT check_capital_max_usd_range
        CHECK (
            max_ultra_fast_usd_per_position >= 10.0 AND max_ultra_fast_usd_per_position <= 100000.0 AND
            max_scalp_usd_per_position >= 10.0 AND max_scalp_usd_per_position <= 100000.0 AND
            max_scalp_reentry_usd_per_position >= 10.0 AND max_scalp_reentry_usd_per_position <= 100000.0 AND
            max_swing_usd_per_position >= 10.0 AND max_swing_usd_per_position <= 100000.0 AND
            max_position_usd_per_position >= 10.0 AND max_position_usd_per_position <= 100000.0
        );
    END IF;
END $$;

-- Validate rebalance threshold (1-100%)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_rebalance_threshold_range'
    ) THEN
        ALTER TABLE user_capital_allocation
        ADD CONSTRAINT check_rebalance_threshold_range
        CHECK (rebalance_threshold_pct >= 1.0 AND rebalance_threshold_pct <= 100.0);
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_capital_allocation IS 'Per-user capital allocation across 4 trading modes. Replaces autopilot_settings.json ModeAllocationConfig section.';
COMMENT ON COLUMN user_capital_allocation.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_capital_allocation.ultra_fast_percent IS 'Capital allocation for ultra-fast scalping mode (%)';
COMMENT ON COLUMN user_capital_allocation.scalp_percent IS 'Capital allocation for scalping mode (%)';
COMMENT ON COLUMN user_capital_allocation.swing_percent IS 'Capital allocation for swing trading mode (%)';
COMMENT ON COLUMN user_capital_allocation.position_percent IS 'Capital allocation for position trading mode (%)';
COMMENT ON COLUMN user_capital_allocation.max_ultra_fast_positions IS 'Max concurrent positions for ultra-fast mode (1-100)';
COMMENT ON COLUMN user_capital_allocation.max_scalp_positions IS 'Max concurrent positions for scalp mode (1-100)';
COMMENT ON COLUMN user_capital_allocation.max_scalp_reentry_positions IS 'Max concurrent positions for scalp reentry optimization (1-100)';
COMMENT ON COLUMN user_capital_allocation.max_swing_positions IS 'Max concurrent positions for swing mode (1-100)';
COMMENT ON COLUMN user_capital_allocation.max_position_positions IS 'Max concurrent positions for position mode (1-100)';
COMMENT ON COLUMN user_capital_allocation.allow_dynamic_rebalance IS 'Allow dynamic capital rebalancing across modes (default: false)';
COMMENT ON COLUMN user_capital_allocation.rebalance_threshold_pct IS 'Drift threshold % before rebalancing triggers (default: 20%)';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_capital_allocation_updated_at ON user_capital_allocation;
-- DROP FUNCTION IF EXISTS update_user_capital_allocation_updated_at();
-- DROP INDEX IF EXISTS idx_user_capital_allocation_user_id;
-- DROP TABLE IF EXISTS user_capital_allocation CASCADE;
