-- Migration: 041_user_mode_strategy_settings
-- Description: Create mode+strategy settings storage table
-- Purpose: Store per-user mode+strategy configurations with JSONB settings
-- Story: 11.29 - Mode-Strategy Database Schema
-- Date: 2026-01-20

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_mode_strategy_settings
-- ====================================================================================
-- Stores per-user mode+strategy configurations
-- Each user can have multiple mode+strategy combinations, each with its own settings
-- Settings are stored as JSONB for flexibility in future schema changes
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_mode_strategy_settings (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(20) NOT NULL,        -- scalp, swing, position, ultra_fast
    strategy VARCHAR(30) NOT NULL,     -- trend_following, mean_reversion, breakout, range_trading
    enabled BOOLEAN DEFAULT true,
    priority INTEGER DEFAULT 1,
    settings JSONB NOT NULL,           -- Complete settings for this mode+strategy
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, mode, strategy)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_mode_strategy_user ON user_mode_strategy_settings(user_id);

-- Fast mode-specific lookups within a user
CREATE INDEX IF NOT EXISTS idx_user_mode_strategy_mode ON user_mode_strategy_settings(user_id, mode);

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_mode_strategy_settings_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_mode_strategy_settings_updated_at ON user_mode_strategy_settings;

CREATE TRIGGER trigger_user_mode_strategy_settings_updated_at
    BEFORE UPDATE ON user_mode_strategy_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_user_mode_strategy_settings_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate mode values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_mode_strategy_mode_valid'
    ) THEN
        ALTER TABLE user_mode_strategy_settings
        ADD CONSTRAINT check_mode_strategy_mode_valid
        CHECK (mode IN ('scalp', 'swing', 'position', 'ultra_fast'));
    END IF;
END $$;

-- Validate strategy values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_mode_strategy_strategy_valid'
    ) THEN
        ALTER TABLE user_mode_strategy_settings
        ADD CONSTRAINT check_mode_strategy_strategy_valid
        CHECK (strategy IN ('trend_following', 'mean_reversion', 'breakout', 'range_trading'));
    END IF;
END $$;

-- Validate priority range (1-10)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_mode_strategy_priority_range'
    ) THEN
        ALTER TABLE user_mode_strategy_settings
        ADD CONSTRAINT check_mode_strategy_priority_range
        CHECK (priority >= 1 AND priority <= 10);
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_mode_strategy_settings IS 'Per-user mode+strategy configuration. Each row represents a unique mode+strategy combination for a user.';
COMMENT ON COLUMN user_mode_strategy_settings.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_mode_strategy_settings.mode IS 'Trading mode: scalp, swing, position, ultra_fast';
COMMENT ON COLUMN user_mode_strategy_settings.strategy IS 'Trading strategy: trend_following, mean_reversion, breakout, range_trading';
COMMENT ON COLUMN user_mode_strategy_settings.enabled IS 'Whether this mode+strategy combination is active';
COMMENT ON COLUMN user_mode_strategy_settings.priority IS 'Priority for this mode+strategy (1-10, higher = more priority)';
COMMENT ON COLUMN user_mode_strategy_settings.settings IS 'Complete JSONB settings for this mode+strategy combination';
COMMENT ON COLUMN user_mode_strategy_settings.created_at IS 'Timestamp when this configuration was created';
COMMENT ON COLUMN user_mode_strategy_settings.updated_at IS 'Timestamp when this configuration was last updated';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_mode_strategy_settings_updated_at ON user_mode_strategy_settings;
-- DROP FUNCTION IF EXISTS update_user_mode_strategy_settings_updated_at();
-- DROP INDEX IF EXISTS idx_user_mode_strategy_mode;
-- DROP INDEX IF EXISTS idx_user_mode_strategy_user;
-- DROP TABLE IF EXISTS user_mode_strategy_settings CASCADE;
