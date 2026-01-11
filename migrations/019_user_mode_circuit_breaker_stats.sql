-- Migration: 019_user_mode_circuit_breaker_stats
-- Description: Create per-user per-mode circuit breaker statistics table
-- Purpose: Replace autopilot_settings.json ModeCircuitBreakerStats section with database storage
-- Date: 2026-01-08

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_mode_circuit_breaker_stats
-- ====================================================================================
-- Stores per-user per-mode circuit breaker statistics
-- Trade counters and loss tracking that survives restarts
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_mode_circuit_breaker_stats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode_name VARCHAR(50) NOT NULL,

    -- Trade counters
    trades_this_minute INTEGER NOT NULL DEFAULT 0,
    trades_this_hour INTEGER NOT NULL DEFAULT 0,
    trades_this_day INTEGER NOT NULL DEFAULT 0,
    total_trades INTEGER NOT NULL DEFAULT 0,
    total_wins INTEGER NOT NULL DEFAULT 0,
    consecutive_losses INTEGER NOT NULL DEFAULT 0,

    -- Loss tracking
    current_hour_loss DECIMAL(20,8) NOT NULL DEFAULT 0.00000000,
    current_day_loss DECIMAL(20,8) NOT NULL DEFAULT 0.00000000,

    -- Pause state
    is_paused BOOLEAN NOT NULL DEFAULT false,
    paused_until TIMESTAMP WITH TIME ZONE,
    pause_reason TEXT,

    -- Timestamps for time-based resets
    last_minute_reset TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_hour_reset TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_day_reset DATE DEFAULT CURRENT_DATE,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one stats record per user per mode
    CONSTRAINT unique_user_mode_cb_stats UNIQUE(user_id, mode_name)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_mode_cb_stats_user_id ON user_mode_circuit_breaker_stats(user_id);

-- Fast mode name filtering
CREATE INDEX IF NOT EXISTS idx_user_mode_cb_stats_mode_name ON user_mode_circuit_breaker_stats(mode_name);

-- Optimized composite index for user+mode queries
CREATE INDEX IF NOT EXISTS idx_user_mode_cb_stats_user_mode ON user_mode_circuit_breaker_stats(user_id, mode_name);

-- Optimized partial index for paused modes
CREATE INDEX IF NOT EXISTS idx_user_mode_cb_stats_paused ON user_mode_circuit_breaker_stats(user_id, mode_name) WHERE is_paused = true;

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_mode_cb_stats_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_mode_cb_stats_updated_at ON user_mode_circuit_breaker_stats;

CREATE TRIGGER trigger_user_mode_cb_stats_updated_at
    BEFORE UPDATE ON user_mode_circuit_breaker_stats
    FOR EACH ROW
    EXECUTE FUNCTION update_user_mode_cb_stats_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate mode_name values (5 trading modes)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_mode_cb_stats_mode_name_valid'
    ) THEN
        ALTER TABLE user_mode_circuit_breaker_stats
        ADD CONSTRAINT check_mode_cb_stats_mode_name_valid
        CHECK (mode_name IN ('ultra_fast', 'scalp', 'scalp_reentry', 'swing', 'position'));
    END IF;
END $$;

-- Validate trade counters (non-negative)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_mode_cb_stats_counters_non_negative'
    ) THEN
        ALTER TABLE user_mode_circuit_breaker_stats
        ADD CONSTRAINT check_mode_cb_stats_counters_non_negative
        CHECK (
            trades_this_minute >= 0 AND
            trades_this_hour >= 0 AND
            trades_this_day >= 0 AND
            total_trades >= 0 AND
            total_wins >= 0 AND
            consecutive_losses >= 0
        );
    END IF;
END $$;

-- Validate total_wins <= total_trades
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_mode_cb_stats_wins_not_exceed_trades'
    ) THEN
        ALTER TABLE user_mode_circuit_breaker_stats
        ADD CONSTRAINT check_mode_cb_stats_wins_not_exceed_trades
        CHECK (total_wins <= total_trades);
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_mode_circuit_breaker_stats IS 'Per-user per-mode circuit breaker statistics. Trade counters and loss tracking that survives restarts.';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.user_id IS 'Foreign key to users table. Stats are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.mode_name IS 'Trading mode name: ultra_fast, scalp, scalp_reentry, swing, or position';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.trades_this_minute IS 'Number of trades executed in the current minute (resets every minute)';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.trades_this_hour IS 'Number of trades executed in the current hour (resets every hour)';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.trades_this_day IS 'Number of trades executed today (resets daily at midnight)';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.total_trades IS 'Lifetime trade count for this mode';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.total_wins IS 'Lifetime winning trades count for this mode';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.consecutive_losses IS 'Current consecutive loss streak (resets on win)';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.current_hour_loss IS 'Total loss in USD for the current hour';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.current_day_loss IS 'Total loss in USD for today';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.is_paused IS 'Whether this mode is currently paused due to circuit breaker trip';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.paused_until IS 'Timestamp when pause will be lifted (NULL if not paused)';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.pause_reason IS 'Reason for pause (e.g., "Max consecutive losses reached")';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.last_minute_reset IS 'Last timestamp when minute counters were reset';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.last_hour_reset IS 'Last timestamp when hour counters were reset';
COMMENT ON COLUMN user_mode_circuit_breaker_stats.last_day_reset IS 'Last date when daily counters were reset';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_mode_cb_stats_updated_at ON user_mode_circuit_breaker_stats;
-- DROP FUNCTION IF EXISTS update_user_mode_cb_stats_updated_at();
-- DROP INDEX IF EXISTS idx_user_mode_cb_stats_paused;
-- DROP INDEX IF EXISTS idx_user_mode_cb_stats_user_mode;
-- DROP INDEX IF EXISTS idx_user_mode_cb_stats_mode_name;
-- DROP INDEX IF EXISTS idx_user_mode_cb_stats_user_id;
-- DROP TABLE IF EXISTS user_mode_circuit_breaker_stats CASCADE;
