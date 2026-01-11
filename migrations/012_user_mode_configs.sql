-- Migration: 012_user_mode_configs
-- Description: Create per-user mode configuration storage
-- Story: Epic 4 Story 4.1

-- ====================================================================================
-- TABLE: user_mode_configs
-- ====================================================================================
-- Stores per-user trading mode configurations as JSONB for maximum flexibility
-- Each user can customize 5 modes: ultra_fast, scalp, scalp_reentry, swing, position
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_mode_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode_name VARCHAR(50) NOT NULL CHECK (mode_name IN ('ultra_fast', 'scalp', 'scalp_reentry', 'swing', 'position')),
    enabled BOOLEAN NOT NULL DEFAULT false,
    config_json JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_user_mode UNIQUE(user_id, mode_name)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_user_id ON user_mode_configs(user_id);

-- Fast mode name filtering
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_mode_name ON user_mode_configs(mode_name);

-- Optimized partial index for enabled modes (commonly queried)
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_enabled ON user_mode_configs(user_id, mode_name) WHERE enabled = true;

-- GIN index for JSONB queries (allows querying inside config_json)
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_config_json ON user_mode_configs USING GIN (config_json);

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_mode_configs_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_mode_configs_updated_at ON user_mode_configs;

CREATE TRIGGER trigger_user_mode_configs_updated_at
    BEFORE UPDATE ON user_mode_configs
    FOR EACH ROW
    EXECUTE FUNCTION update_user_mode_configs_updated_at();

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_mode_configs IS 'Per-user trading mode configurations. Each user can customize 5 trading modes with unique settings stored as JSONB.';
COMMENT ON COLUMN user_mode_configs.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_mode_configs.mode_name IS 'Trading mode name: ultra_fast, scalp, scalp_reentry, swing, or position';
COMMENT ON COLUMN user_mode_configs.enabled IS 'Whether this mode is currently enabled for the user (default: false for safety)';
COMMENT ON COLUMN user_mode_configs.config_json IS 'Complete mode configuration as JSONB (autopilot.ModeFullConfig structure). Default: {} requires explicit configuration.';
