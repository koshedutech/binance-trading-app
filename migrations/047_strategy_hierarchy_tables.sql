-- Migration 047: Strategy Hierarchy Tables
-- Story 11.44: Volume Imbalance Database Schema & Repository
-- Supports Mode -> Strategy Group -> Sub-Strategy architecture

-- =====================================================
-- TABLE 1: Strategy Group Settings
-- Base settings per mode/group (breakout, trending, range, volatile)
-- =====================================================

CREATE TABLE IF NOT EXISTS user_strategy_group_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(20) NOT NULL,           -- 'scalp', 'swing', 'position', 'ultra_fast'
    strategy_group VARCHAR(20) NOT NULL, -- 'breakout', 'trending', 'range', 'volatile'
    enabled BOOLEAN DEFAULT false,

    -- Base settings (inherited by all sub-strategies)
    timeframe VARCHAR(10) NOT NULL DEFAULT '15m',
    position_size_percent DECIMAL(5,2) DEFAULT 2.0,
    max_leverage INTEGER DEFAULT 10,
    max_positions INTEGER DEFAULT 3,
    min_volume_usdt DECIMAL(15,2) DEFAULT 1000000,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, mode, strategy_group)
);

-- Index for querying all groups for a user's mode
CREATE INDEX IF NOT EXISTS idx_strategy_group_user_mode
ON user_strategy_group_settings(user_id, mode);

-- Partial index for enabled groups only (efficient for finding active strategies)
CREATE INDEX IF NOT EXISTS idx_strategy_group_enabled
ON user_strategy_group_settings(user_id, enabled) WHERE enabled = true;

-- =====================================================
-- TABLE 2: Sub-Strategy Settings
-- Strategy-specific settings (e.g., ravindra_volume_imbalance parameters)
-- =====================================================

CREATE TABLE IF NOT EXISTS user_sub_strategy_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(20) NOT NULL,
    strategy_group VARCHAR(20) NOT NULL,
    sub_strategy VARCHAR(50) NOT NULL,   -- 'ravindra_volume_imbalance', 'classic_breakout', etc.
    enabled BOOLEAN DEFAULT false,

    -- Strategy-specific settings (JSONB for flexibility)
    -- Allows each sub-strategy to have its own unique configuration structure
    settings JSONB DEFAULT '{}',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, mode, strategy_group, sub_strategy)
);

-- Partial index for enabled sub-strategies (fast lookup for active strategies)
CREATE INDEX IF NOT EXISTS idx_sub_strategy_enabled
ON user_sub_strategy_settings(user_id, enabled) WHERE enabled = true;

-- Compound index for lookup by mode and group
CREATE INDEX IF NOT EXISTS idx_sub_strategy_lookup
ON user_sub_strategy_settings(user_id, mode, strategy_group);

-- GIN index for efficient JSONB queries on settings
CREATE INDEX IF NOT EXISTS idx_sub_strategy_settings_gin
ON user_sub_strategy_settings USING GIN (settings);

-- =====================================================
-- COMMENTS
-- =====================================================

COMMENT ON TABLE user_strategy_group_settings IS 'Strategy group settings per mode. Story 11.44: Part of the Mode -> Strategy Group -> Sub-Strategy hierarchy for breakout, trending, range, and volatile strategies.';

COMMENT ON COLUMN user_strategy_group_settings.mode IS 'Trading mode: scalp, swing, position, or ultra_fast';
COMMENT ON COLUMN user_strategy_group_settings.strategy_group IS 'Strategy group: breakout, trending, range, or volatile';
COMMENT ON COLUMN user_strategy_group_settings.enabled IS 'Whether this strategy group is enabled for the mode';
COMMENT ON COLUMN user_strategy_group_settings.timeframe IS 'Base timeframe for strategies in this group (e.g., 15m, 1h)';
COMMENT ON COLUMN user_strategy_group_settings.position_size_percent IS 'Position size as percentage of allocated capital';
COMMENT ON COLUMN user_strategy_group_settings.max_leverage IS 'Maximum leverage allowed for this group';
COMMENT ON COLUMN user_strategy_group_settings.max_positions IS 'Maximum concurrent positions for this group';
COMMENT ON COLUMN user_strategy_group_settings.min_volume_usdt IS 'Minimum 24h volume in USDT for symbol eligibility';

COMMENT ON TABLE user_sub_strategy_settings IS 'Sub-strategy specific settings. Story 11.44: Each sub-strategy (e.g., ravindra_volume_imbalance) has its own JSONB settings allowing flexible configuration per strategy implementation.';

COMMENT ON COLUMN user_sub_strategy_settings.sub_strategy IS 'Sub-strategy identifier: ravindra_volume_imbalance, classic_breakout, etc.';
COMMENT ON COLUMN user_sub_strategy_settings.settings IS 'JSONB containing strategy-specific parameters (pattern detection, trailing stop milestones, etc.)';

-- =====================================================
-- VALIDATION CONSTRAINTS (Story 11.44 Fix)
-- =====================================================

-- Validate mode values for strategy groups
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_strategy_group_mode_valid'
    ) THEN
        ALTER TABLE user_strategy_group_settings
        ADD CONSTRAINT check_strategy_group_mode_valid
        CHECK (mode IN ('scalp', 'swing', 'position', 'ultra_fast'));
    END IF;
END $$;

-- Validate strategy_group values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_strategy_group_valid'
    ) THEN
        ALTER TABLE user_strategy_group_settings
        ADD CONSTRAINT check_strategy_group_valid
        CHECK (strategy_group IN ('breakout', 'trending', 'range', 'volatile'));
    END IF;
END $$;

-- Validate mode values for sub-strategies
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_sub_strategy_mode_valid'
    ) THEN
        ALTER TABLE user_sub_strategy_settings
        ADD CONSTRAINT check_sub_strategy_mode_valid
        CHECK (mode IN ('scalp', 'swing', 'position', 'ultra_fast'));
    END IF;
END $$;

-- Validate strategy_group values for sub-strategies
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_sub_strategy_group_valid'
    ) THEN
        ALTER TABLE user_sub_strategy_settings
        ADD CONSTRAINT check_sub_strategy_group_valid
        CHECK (strategy_group IN ('breakout', 'trending', 'range', 'volatile'));
    END IF;
END $$;

-- =====================================================
-- AUTO-UPDATE TRIGGERS FOR updated_at (Story 11.44 Fix)
-- =====================================================

-- Trigger function for strategy_group_settings
CREATE OR REPLACE FUNCTION update_strategy_group_settings_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_strategy_group_settings_updated_at ON user_strategy_group_settings;

CREATE TRIGGER trigger_strategy_group_settings_updated_at
    BEFORE UPDATE ON user_strategy_group_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_strategy_group_settings_updated_at();

-- Trigger function for sub_strategy_settings
CREATE OR REPLACE FUNCTION update_sub_strategy_settings_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_sub_strategy_settings_updated_at ON user_sub_strategy_settings;

CREATE TRIGGER trigger_sub_strategy_settings_updated_at
    BEFORE UPDATE ON user_sub_strategy_settings
    FOR EACH ROW
    EXECUTE FUNCTION update_sub_strategy_settings_updated_at();

-- =====================================================
-- ADDITIONAL INDEX FOR USER-ONLY QUERIES (Story 11.44 Fix)
-- =====================================================

-- Index for user-only queries (GetFullStrategyHierarchy, GetEnabledStrategies)
CREATE INDEX IF NOT EXISTS idx_sub_strategy_user
ON user_sub_strategy_settings(user_id);
