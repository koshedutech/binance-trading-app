-- Migration: 018_user_spot_settings
-- Description: Create per-user spot autopilot settings table
-- Purpose: Replace autopilot_settings.json spot autopilot settings with database storage
-- Date: 2026-01-08

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_spot_settings
-- ====================================================================================
-- Stores per-user spot trading autopilot settings
-- Includes circuit breaker settings, coin preferences, and PnL statistics
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_spot_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Core spot settings
    enabled BOOLEAN NOT NULL DEFAULT false,
    dry_run_mode BOOLEAN NOT NULL DEFAULT false,
    risk_level VARCHAR(20) NOT NULL DEFAULT 'moderate',
    max_positions INTEGER NOT NULL DEFAULT 5,
    max_usd_per_position DECIMAL(20,8) NOT NULL DEFAULT 500.00000000,
    take_profit_percent DECIMAL(10,4) NOT NULL DEFAULT 3.0000,
    stop_loss_percent DECIMAL(10,4) NOT NULL DEFAULT 2.0000,
    min_confidence DECIMAL(10,4) NOT NULL DEFAULT 70.0000,

    -- Circuit breaker settings
    circuit_breaker_enabled BOOLEAN NOT NULL DEFAULT true,
    cb_max_loss_per_hour DECIMAL(20,8) NOT NULL DEFAULT 50.00000000,
    cb_max_daily_loss DECIMAL(20,8) NOT NULL DEFAULT 200.00000000,
    cb_max_consecutive_losses INTEGER NOT NULL DEFAULT 5,
    cb_cooldown_minutes INTEGER NOT NULL DEFAULT 30,
    cb_max_trades_per_minute INTEGER NOT NULL DEFAULT 5,
    cb_max_daily_trades INTEGER NOT NULL DEFAULT 50,

    -- Coin preferences (stored as PostgreSQL arrays)
    coin_blacklist TEXT[] DEFAULT ARRAY[]::TEXT[],
    coin_whitelist TEXT[] DEFAULT ARRAY[]::TEXT[],
    use_whitelist BOOLEAN NOT NULL DEFAULT false,

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

    -- Ensure one spot settings config per user
    CONSTRAINT unique_user_spot_settings UNIQUE(user_id)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_spot_settings_user_id ON user_spot_settings(user_id);

-- Optimized partial index for enabled spot trading
CREATE INDEX IF NOT EXISTS idx_user_spot_settings_enabled ON user_spot_settings(user_id) WHERE enabled = true;

-- GIN index for coin blacklist/whitelist array queries
CREATE INDEX IF NOT EXISTS idx_user_spot_settings_coin_blacklist ON user_spot_settings USING GIN (coin_blacklist);
CREATE INDEX IF NOT EXISTS idx_user_spot_settings_coin_whitelist ON user_spot_settings USING GIN (coin_whitelist);

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_spot_settings_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_spot_settings_updated_at ON user_spot_settings;

CREATE TRIGGER trigger_user_spot_settings_updated_at
    BEFORE UPDATE ON user_spot_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_user_spot_settings_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate risk_level values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_risk_level_valid'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_risk_level_valid
        CHECK (risk_level IN ('conservative', 'moderate', 'aggressive'));
    END IF;
END $$;

-- Validate max_positions (1-100)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_max_positions_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_max_positions_range
        CHECK (max_positions >= 1 AND max_positions <= 100);
    END IF;
END $$;

-- Validate max_usd_per_position ($10 - $100,000)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_max_usd_per_position_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_max_usd_per_position_range
        CHECK (max_usd_per_position >= 10.0 AND max_usd_per_position <= 100000.0);
    END IF;
END $$;

-- Validate take_profit_percent (0.1% - 100%)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_take_profit_percent_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_take_profit_percent_range
        CHECK (take_profit_percent >= 0.1 AND take_profit_percent <= 100.0);
    END IF;
END $$;

-- Validate stop_loss_percent (0.1% - 50%)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_stop_loss_percent_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_stop_loss_percent_range
        CHECK (stop_loss_percent >= 0.1 AND stop_loss_percent <= 50.0);
    END IF;
END $$;

-- Validate min_confidence (0-100%)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_min_confidence_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_min_confidence_range
        CHECK (min_confidence >= 0.0 AND min_confidence <= 100.0);
    END IF;
END $$;

-- Validate circuit breaker settings (same as global circuit breaker)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_cb_max_loss_per_hour_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_cb_max_loss_per_hour_range
        CHECK (cb_max_loss_per_hour >= 1.0 AND cb_max_loss_per_hour <= 100000.0);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_cb_max_daily_loss_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_cb_max_daily_loss_range
        CHECK (cb_max_daily_loss >= 1.0 AND cb_max_daily_loss <= 1000000.0);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_cb_max_consecutive_losses_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_cb_max_consecutive_losses_range
        CHECK (cb_max_consecutive_losses >= 1 AND cb_max_consecutive_losses <= 100);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_cb_cooldown_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_cb_cooldown_range
        CHECK (cb_cooldown_minutes >= 1 AND cb_cooldown_minutes <= 1440);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_cb_max_trades_per_minute_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_cb_max_trades_per_minute_range
        CHECK (cb_max_trades_per_minute >= 1 AND cb_max_trades_per_minute <= 100);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_spot_cb_max_daily_trades_range'
    ) THEN
        ALTER TABLE user_spot_settings
        ADD CONSTRAINT check_spot_cb_max_daily_trades_range
        CHECK (cb_max_daily_trades >= 1 AND cb_max_daily_trades <= 10000);
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_spot_settings IS 'Per-user spot trading autopilot settings. Includes circuit breaker settings, coin preferences, and PnL statistics.';
COMMENT ON COLUMN user_spot_settings.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_spot_settings.enabled IS 'Master toggle for spot trading autopilot (default: false)';
COMMENT ON COLUMN user_spot_settings.dry_run_mode IS 'Paper trading mode - no real trades executed (default: false)';
COMMENT ON COLUMN user_spot_settings.risk_level IS 'Risk level: conservative, moderate, aggressive (default: moderate)';
COMMENT ON COLUMN user_spot_settings.max_positions IS 'Max concurrent spot positions (1-100, default: 5)';
COMMENT ON COLUMN user_spot_settings.max_usd_per_position IS 'Max USD per spot position ($10-$100k, default: $500)';
COMMENT ON COLUMN user_spot_settings.take_profit_percent IS 'Take profit percentage (0.1-100%, default: 3%)';
COMMENT ON COLUMN user_spot_settings.stop_loss_percent IS 'Stop loss percentage (0.1-50%, default: 2%)';
COMMENT ON COLUMN user_spot_settings.min_confidence IS 'Min AI confidence to enter trade (0-100%, default: 70%)';
COMMENT ON COLUMN user_spot_settings.circuit_breaker_enabled IS 'Enable spot circuit breaker (default: true)';
COMMENT ON COLUMN user_spot_settings.coin_blacklist IS 'Coins to never trade (PostgreSQL array)';
COMMENT ON COLUMN user_spot_settings.coin_whitelist IS 'Coins to exclusively trade (PostgreSQL array)';
COMMENT ON COLUMN user_spot_settings.use_whitelist IS 'Use whitelist instead of blacklist (default: false)';
COMMENT ON COLUMN user_spot_settings.total_pnl IS 'Lifetime spot trading PnL in USD';
COMMENT ON COLUMN user_spot_settings.daily_pnl IS 'Todays spot trading PnL in USD';
COMMENT ON COLUMN user_spot_settings.total_trades IS 'Lifetime spot trade count';
COMMENT ON COLUMN user_spot_settings.winning_trades IS 'Lifetime spot winning trades count';
COMMENT ON COLUMN user_spot_settings.daily_trades IS 'Todays spot trade count';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_spot_settings_updated_at ON user_spot_settings;
-- DROP FUNCTION IF EXISTS update_user_spot_settings_updated_at();
-- DROP INDEX IF EXISTS idx_user_spot_settings_coin_whitelist;
-- DROP INDEX IF EXISTS idx_user_spot_settings_coin_blacklist;
-- DROP INDEX IF EXISTS idx_user_spot_settings_enabled;
-- DROP INDEX IF EXISTS idx_user_spot_settings_user_id;
-- DROP TABLE IF EXISTS user_spot_settings CASCADE;
