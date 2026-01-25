-- Migration 048: User Trading State
-- Story 14.14: Trading Controller - ON/OFF toggle for Chain Trading System
-- Story 14.18: Database Migrations verification (added DOWN section and updated_at trigger)
-- Controls Entry Decision and Exit Decision independently of Ginie Autopilot

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- Create table for user trading state (one row per user)
CREATE TABLE IF NOT EXISTS user_trading_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Master trading switch
    -- When OFF: Entry Decision paused (no new entries), Exit Decision still active
    -- Coin Profiler continues to run, patterns/scores still calculate
    enabled BOOLEAN NOT NULL DEFAULT true,

    -- Entry Decision active state
    -- Follows master switch: true when enabled=true, false when enabled=false
    entry_active BOOLEAN NOT NULL DEFAULT true,

    -- Exit Decision active state
    -- Always true when positions exist, ensures position management continues
    exit_active BOOLEAN NOT NULL DEFAULT true,

    -- Audit trail
    last_changed TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    changed_by VARCHAR(20) NOT NULL DEFAULT 'system', -- 'user' or 'system'
    reason TEXT, -- Optional reason for state change

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure one record per user
    UNIQUE(user_id)
);

-- Create index for fast lookups by user
CREATE INDEX IF NOT EXISTS idx_user_trading_state_user_id ON user_trading_state(user_id);

-- Create index for finding enabled users
CREATE INDEX IF NOT EXISTS idx_user_trading_state_enabled ON user_trading_state(enabled) WHERE enabled = true;

-- ============================================================
-- AUTO-UPDATE TRIGGER FOR updated_at
-- ============================================================

CREATE OR REPLACE FUNCTION update_user_trading_state_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_trading_state_updated_at ON user_trading_state;

CREATE TRIGGER trigger_user_trading_state_updated_at
    BEFORE UPDATE ON user_trading_state
    FOR EACH ROW
    EXECUTE FUNCTION update_user_trading_state_updated_at();

-- ============================================================
-- VALIDATION CONSTRAINTS
-- ============================================================

-- Validate changed_by values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_trading_state_changed_by_valid'
    ) THEN
        ALTER TABLE user_trading_state
        ADD CONSTRAINT check_trading_state_changed_by_valid
        CHECK (changed_by IN ('user', 'system'));
    END IF;
END $$;

-- ============================================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================================

COMMENT ON TABLE user_trading_state IS 'Per-user trading ON/OFF state for Chain Trading System. Independent of Ginie Autopilot state.';
COMMENT ON COLUMN user_trading_state.enabled IS 'Master trading switch: true=trading active, false=new entries paused';
COMMENT ON COLUMN user_trading_state.entry_active IS 'Entry Decision state: follows master switch';
COMMENT ON COLUMN user_trading_state.exit_active IS 'Exit Decision state: always true when positions exist';
COMMENT ON COLUMN user_trading_state.changed_by IS 'Who changed the state: user or system';
COMMENT ON COLUMN user_trading_state.reason IS 'Optional reason for the state change';
COMMENT ON COLUMN user_trading_state.last_changed IS 'Timestamp of last state change (for audit trail)';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- To rollback this migration, execute the following:
--
-- DROP TRIGGER IF EXISTS trigger_user_trading_state_updated_at ON user_trading_state;
-- DROP FUNCTION IF EXISTS update_user_trading_state_updated_at();
-- DROP INDEX IF EXISTS idx_user_trading_state_enabled;
-- DROP INDEX IF EXISTS idx_user_trading_state_user_id;
-- DROP TABLE IF EXISTS user_trading_state CASCADE;
