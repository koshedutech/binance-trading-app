-- Migration: 016_user_early_warning
-- Description: Create per-user early warning system configuration table
-- Purpose: Replace autopilot_settings.json early warning settings with database storage
-- Date: 2026-01-08

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_early_warning
-- ====================================================================================
-- Stores per-user early warning system settings
-- Multi-timeframe position health monitor with AI-driven loss detection
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_early_warning (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Master toggle
    enabled BOOLEAN NOT NULL DEFAULT true,

    -- Monitoring timing
    start_after_minutes INTEGER NOT NULL DEFAULT 1,
    check_interval_secs INTEGER NOT NULL DEFAULT 30,

    -- Trigger conditions
    only_underwater BOOLEAN NOT NULL DEFAULT true,
    min_loss_percent DECIMAL(10,4) NOT NULL DEFAULT 0.3000,

    -- Action settings
    close_on_reversal BOOLEAN NOT NULL DEFAULT true,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one early warning config per user
    CONSTRAINT unique_user_early_warning UNIQUE(user_id)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_early_warning_user_id ON user_early_warning(user_id);

-- Optimized partial index for enabled early warning
CREATE INDEX IF NOT EXISTS idx_user_early_warning_enabled ON user_early_warning(user_id) WHERE enabled = true;

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_early_warning_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_early_warning_updated_at ON user_early_warning;

CREATE TRIGGER trigger_user_early_warning_updated_at
    BEFORE UPDATE ON user_early_warning
    FOR EACH ROW
    EXECUTE FUNCTION update_user_early_warning_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate start_after_minutes (0-60 minutes)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ew_start_after_range'
    ) THEN
        ALTER TABLE user_early_warning
        ADD CONSTRAINT check_ew_start_after_range
        CHECK (start_after_minutes >= 0 AND start_after_minutes <= 60);
    END IF;
END $$;

-- Validate check_interval_secs (10-600 seconds = 10s to 10 minutes)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ew_check_interval_range'
    ) THEN
        ALTER TABLE user_early_warning
        ADD CONSTRAINT check_ew_check_interval_range
        CHECK (check_interval_secs >= 10 AND check_interval_secs <= 600);
    END IF;
END $$;

-- Validate min_loss_percent (0.1% - 10%)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_ew_min_loss_percent_range'
    ) THEN
        ALTER TABLE user_early_warning
        ADD CONSTRAINT check_ew_min_loss_percent_range
        CHECK (min_loss_percent >= 0.1 AND min_loss_percent <= 10.0);
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_early_warning IS 'Per-user early warning system configuration. Multi-timeframe position health monitor with AI-driven loss detection.';
COMMENT ON COLUMN user_early_warning.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_early_warning.enabled IS 'Master toggle for early warning system (default: true)';
COMMENT ON COLUMN user_early_warning.start_after_minutes IS 'Start monitoring after X minutes from position entry (0-60, default: 1)';
COMMENT ON COLUMN user_early_warning.check_interval_secs IS 'Check interval in seconds (10-600, default: 30)';
COMMENT ON COLUMN user_early_warning.only_underwater IS 'Only monitor positions with negative PnL (default: true)';
COMMENT ON COLUMN user_early_warning.min_loss_percent IS 'Min loss % to activate monitoring (0.1-10%, default: 0.3%)';
COMMENT ON COLUMN user_early_warning.close_on_reversal IS 'Auto-close position if trend reversal detected (default: true)';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_early_warning_updated_at ON user_early_warning;
-- DROP FUNCTION IF EXISTS update_user_early_warning_updated_at();
-- DROP INDEX IF EXISTS idx_user_early_warning_enabled;
-- DROP INDEX IF EXISTS idx_user_early_warning_user_id;
-- DROP TABLE IF EXISTS user_early_warning CASCADE;
