-- Migration: 017_user_ginie_settings
-- Description: Create per-user Ginie autopilot global settings table
-- Purpose: Replace autopilot_settings.json Ginie global settings with database storage
-- Date: 2026-01-08

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_ginie_settings
-- ====================================================================================
-- Stores per-user Ginie autopilot global settings and PnL statistics
-- Includes dry run mode, auto start, max positions, and performance metrics
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_ginie_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Global settings
    dry_run_mode BOOLEAN NOT NULL DEFAULT false,
    auto_start BOOLEAN NOT NULL DEFAULT false,
    max_positions INTEGER NOT NULL DEFAULT 10,

    -- Auto mode settings (LLM-driven trading)
    auto_mode_enabled BOOLEAN NOT NULL DEFAULT false,
    auto_mode_max_positions INTEGER NOT NULL DEFAULT 5,
    auto_mode_max_leverage INTEGER NOT NULL DEFAULT 10,
    auto_mode_max_position_size DECIMAL(20,8) NOT NULL DEFAULT 1000.00000000,
    auto_mode_max_total_usd DECIMAL(20,8) NOT NULL DEFAULT 5000.00000000,
    auto_mode_allow_averaging BOOLEAN NOT NULL DEFAULT true,
    auto_mode_max_averages INTEGER NOT NULL DEFAULT 3,
    auto_mode_min_hold_minutes INTEGER NOT NULL DEFAULT 5,
    auto_mode_quick_profit_mode BOOLEAN NOT NULL DEFAULT false,
    auto_mode_min_profit_exit DECIMAL(10,4) NOT NULL DEFAULT 1.5000,

    -- PnL statistics (persisted)
    total_pnl DECIMAL(20,8) NOT NULL DEFAULT 0.00000000,
    daily_pnl DECIMAL(20,8) NOT NULL DEFAULT 0.00000000,
    total_trades INTEGER NOT NULL DEFAULT 0,
    winning_trades INTEGER NOT NULL DEFAULT 0,
    daily_trades INTEGER NOT NULL DEFAULT 0,
    pnl_last_update TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one Ginie settings config per user
    CONSTRAINT unique_user_ginie_settings UNIQUE(user_id)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_ginie_settings_user_id ON user_ginie_settings(user_id);

-- Optimized partial index for auto-start users
CREATE INDEX IF NOT EXISTS idx_user_ginie_settings_auto_start ON user_ginie_settings(user_id) WHERE auto_start = true;

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_ginie_settings_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_ginie_settings_updated_at ON user_ginie_settings;

CREATE TRIGGER trigger_user_ginie_settings_updated_at
    BEFORE UPDATE ON user_ginie_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_user_ginie_settings_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate max_positions (1-100)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_max_positions_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_max_positions_range
        CHECK (max_positions >= 1 AND max_positions <= 100);
    END IF;
END $$;

-- Validate auto_mode_max_positions (1-50)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_auto_mode_max_positions_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_auto_mode_max_positions_range
        CHECK (auto_mode_max_positions >= 1 AND auto_mode_max_positions <= 50);
    END IF;
END $$;

-- Validate auto_mode_max_leverage (1-125)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_auto_mode_max_leverage_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_auto_mode_max_leverage_range
        CHECK (auto_mode_max_leverage >= 1 AND auto_mode_max_leverage <= 125);
    END IF;
END $$;

-- Validate auto_mode_max_position_size ($10 - $100,000)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_auto_mode_max_position_size_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_auto_mode_max_position_size_range
        CHECK (auto_mode_max_position_size >= 10.0 AND auto_mode_max_position_size <= 100000.0);
    END IF;
END $$;

-- Validate auto_mode_max_total_usd ($10 - $1,000,000)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_auto_mode_max_total_usd_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_auto_mode_max_total_usd_range
        CHECK (auto_mode_max_total_usd >= 10.0 AND auto_mode_max_total_usd <= 1000000.0);
    END IF;
END $$;

-- Validate auto_mode_max_averages (1-10)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_auto_mode_max_averages_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_auto_mode_max_averages_range
        CHECK (auto_mode_max_averages >= 1 AND auto_mode_max_averages <= 10);
    END IF;
END $$;

-- Validate auto_mode_min_hold_minutes (1-1440 = 1 minute to 24 hours)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_auto_mode_min_hold_minutes_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_auto_mode_min_hold_minutes_range
        CHECK (auto_mode_min_hold_minutes >= 1 AND auto_mode_min_hold_minutes <= 1440);
    END IF;
END $$;

-- Validate auto_mode_min_profit_exit (0.1% - 20%)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ginie_auto_mode_min_profit_exit_range'
    ) THEN
        ALTER TABLE user_ginie_settings
        ADD CONSTRAINT check_ginie_auto_mode_min_profit_exit_range
        CHECK (auto_mode_min_profit_exit >= 0.1 AND auto_mode_min_profit_exit <= 20.0);
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_ginie_settings IS 'Per-user Ginie autopilot global settings and PnL statistics. Includes dry run mode, auto start, and performance metrics.';
COMMENT ON COLUMN user_ginie_settings.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_ginie_settings.dry_run_mode IS 'Paper trading mode - no real trades executed (default: false)';
COMMENT ON COLUMN user_ginie_settings.auto_start IS 'Auto-start Ginie on server restart (default: false)';
COMMENT ON COLUMN user_ginie_settings.max_positions IS 'Max concurrent positions for Ginie (1-100, default: 10)';
COMMENT ON COLUMN user_ginie_settings.auto_mode_enabled IS 'Enable LLM-driven auto trading mode (default: false)';
COMMENT ON COLUMN user_ginie_settings.total_pnl IS 'Lifetime realized PnL in USD';
COMMENT ON COLUMN user_ginie_settings.daily_pnl IS 'Todays realized PnL in USD';
COMMENT ON COLUMN user_ginie_settings.total_trades IS 'Lifetime trade count';
COMMENT ON COLUMN user_ginie_settings.winning_trades IS 'Lifetime winning trades count';
COMMENT ON COLUMN user_ginie_settings.daily_trades IS 'Todays trade count';
COMMENT ON COLUMN user_ginie_settings.pnl_last_update IS 'Last update timestamp for daily reset tracking';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_ginie_settings_updated_at ON user_ginie_settings;
-- DROP FUNCTION IF EXISTS update_user_ginie_settings_updated_at();
-- DROP INDEX IF EXISTS idx_user_ginie_settings_auto_start;
-- DROP INDEX IF EXISTS idx_user_ginie_settings_user_id;
-- DROP TABLE IF EXISTS user_ginie_settings CASCADE;
