-- Migration: 015_user_global_circuit_breaker
-- Description: Create per-user global circuit breaker configuration table
-- Purpose: Replace autopilot_settings.json circuit breaker settings with database storage
-- Date: 2026-01-08

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_global_circuit_breaker
-- ====================================================================================
-- Stores per-user global circuit breaker settings
-- Prevents excessive losses by pausing trading when limits are hit
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_global_circuit_breaker (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Master toggle
    enabled BOOLEAN NOT NULL DEFAULT true,

    -- Loss limits
    max_loss_per_hour DECIMAL(20,8) NOT NULL DEFAULT 100.00000000,
    max_daily_loss DECIMAL(20,8) NOT NULL DEFAULT 500.00000000,
    max_consecutive_losses INTEGER NOT NULL DEFAULT 5,

    -- Rate limiting
    cooldown_minutes INTEGER NOT NULL DEFAULT 30,
    max_trades_per_minute INTEGER NOT NULL DEFAULT 10,
    max_daily_trades INTEGER NOT NULL DEFAULT 100,

    -- Risk management
    risk_level VARCHAR(20) NOT NULL DEFAULT 'moderate',
    dry_run_mode BOOLEAN NOT NULL DEFAULT false,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one circuit breaker config per user
    CONSTRAINT unique_user_global_circuit_breaker UNIQUE(user_id)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_global_circuit_breaker_user_id ON user_global_circuit_breaker(user_id);

-- Optimized partial index for enabled circuit breakers
CREATE INDEX IF NOT EXISTS idx_user_global_circuit_breaker_enabled ON user_global_circuit_breaker(user_id) WHERE enabled = true;

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_global_circuit_breaker_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_global_circuit_breaker_updated_at ON user_global_circuit_breaker;

CREATE TRIGGER trigger_user_global_circuit_breaker_updated_at
    BEFORE UPDATE ON user_global_circuit_breaker
    FOR EACH ROW
    EXECUTE FUNCTION update_user_global_circuit_breaker_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate max_loss_per_hour ($1 - $100,000)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_cb_max_loss_per_hour_range'
    ) THEN
        ALTER TABLE user_global_circuit_breaker
        ADD CONSTRAINT check_cb_max_loss_per_hour_range
        CHECK (max_loss_per_hour >= 1.0 AND max_loss_per_hour <= 100000.0);
    END IF;
END $$;

-- Validate max_daily_loss ($1 - $1,000,000)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_cb_max_daily_loss_range'
    ) THEN
        ALTER TABLE user_global_circuit_breaker
        ADD CONSTRAINT check_cb_max_daily_loss_range
        CHECK (max_daily_loss >= 1.0 AND max_daily_loss <= 1000000.0);
    END IF;
END $$;

-- Validate max_consecutive_losses (1-100)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_cb_max_consecutive_losses_range'
    ) THEN
        ALTER TABLE user_global_circuit_breaker
        ADD CONSTRAINT check_cb_max_consecutive_losses_range
        CHECK (max_consecutive_losses >= 1 AND max_consecutive_losses <= 100);
    END IF;
END $$;

-- Validate cooldown_minutes (1-1440 = 1 minute to 24 hours)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_cb_cooldown_range'
    ) THEN
        ALTER TABLE user_global_circuit_breaker
        ADD CONSTRAINT check_cb_cooldown_range
        CHECK (cooldown_minutes >= 1 AND cooldown_minutes <= 1440);
    END IF;
END $$;

-- Validate max_trades_per_minute (1-100)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_cb_max_trades_per_minute_range'
    ) THEN
        ALTER TABLE user_global_circuit_breaker
        ADD CONSTRAINT check_cb_max_trades_per_minute_range
        CHECK (max_trades_per_minute >= 1 AND max_trades_per_minute <= 100);
    END IF;
END $$;

-- Validate max_daily_trades (1-10000)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_cb_max_daily_trades_range'
    ) THEN
        ALTER TABLE user_global_circuit_breaker
        ADD CONSTRAINT check_cb_max_daily_trades_range
        CHECK (max_daily_trades >= 1 AND max_daily_trades <= 10000);
    END IF;
END $$;

-- Validate risk_level values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_cb_risk_level_valid'
    ) THEN
        ALTER TABLE user_global_circuit_breaker
        ADD CONSTRAINT check_cb_risk_level_valid
        CHECK (risk_level IN ('conservative', 'moderate', 'aggressive'));
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_global_circuit_breaker IS 'Per-user global circuit breaker configuration. Prevents excessive losses by pausing trading when limits are hit.';
COMMENT ON COLUMN user_global_circuit_breaker.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_global_circuit_breaker.enabled IS 'Master toggle for circuit breaker protection (default: true)';
COMMENT ON COLUMN user_global_circuit_breaker.max_loss_per_hour IS 'Max loss per hour in USD before circuit trips ($1-$100k, default: $100)';
COMMENT ON COLUMN user_global_circuit_breaker.max_daily_loss IS 'Max daily loss in USD before circuit trips ($1-$1M, default: $500)';
COMMENT ON COLUMN user_global_circuit_breaker.max_consecutive_losses IS 'Max consecutive losses before circuit trips (1-100, default: 5)';
COMMENT ON COLUMN user_global_circuit_breaker.cooldown_minutes IS 'Cooldown period in minutes after circuit trips (1-1440, default: 30)';
COMMENT ON COLUMN user_global_circuit_breaker.max_trades_per_minute IS 'Max trades per minute before rate limit (1-100, default: 10)';
COMMENT ON COLUMN user_global_circuit_breaker.max_daily_trades IS 'Max daily trades before rate limit (1-10000, default: 100)';
COMMENT ON COLUMN user_global_circuit_breaker.risk_level IS 'Risk level: conservative, moderate, aggressive (default: moderate)';
COMMENT ON COLUMN user_global_circuit_breaker.dry_run_mode IS 'Paper trading mode - no real trades executed (default: false)';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_global_circuit_breaker_updated_at ON user_global_circuit_breaker;
-- DROP FUNCTION IF EXISTS update_user_global_circuit_breaker_updated_at();
-- DROP INDEX IF EXISTS idx_user_global_circuit_breaker_enabled;
-- DROP INDEX IF EXISTS idx_user_global_circuit_breaker_user_id;
-- DROP TABLE IF EXISTS user_global_circuit_breaker CASCADE;
