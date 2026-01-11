-- Migration: 013_user_llm_config
-- Description: Create per-user LLM provider configuration table
-- Purpose: Replace autopilot_settings.json LLMConfig section with database storage
-- Date: 2026-01-08

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_llm_config
-- ====================================================================================
-- Stores per-user LLM provider configuration settings
-- Replaces autopilot_settings.json LLMConfig section
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_llm_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Core LLM settings
    enabled BOOLEAN NOT NULL DEFAULT true,
    provider VARCHAR(50) NOT NULL DEFAULT 'deepseek',
    model VARCHAR(100) NOT NULL DEFAULT 'deepseek-chat',

    -- Fallback configuration
    fallback_provider VARCHAR(50) DEFAULT 'claude',
    fallback_model VARCHAR(100) DEFAULT 'claude-3-haiku',

    -- Performance tuning
    timeout_ms INTEGER NOT NULL DEFAULT 5000,
    retry_count INTEGER NOT NULL DEFAULT 2,
    cache_duration_sec INTEGER NOT NULL DEFAULT 300,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one config per user
    CONSTRAINT unique_user_llm_config UNIQUE(user_id)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_llm_config_user_id ON user_llm_config(user_id);

-- Lookup by provider (for monitoring/analytics)
CREATE INDEX IF NOT EXISTS idx_user_llm_config_provider ON user_llm_config(provider);

-- Optimized partial index for enabled LLM configs
CREATE INDEX IF NOT EXISTS idx_user_llm_config_enabled ON user_llm_config(user_id) WHERE enabled = true;

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_llm_config_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_llm_config_updated_at ON user_llm_config;

CREATE TRIGGER trigger_user_llm_config_updated_at
    BEFORE UPDATE ON user_llm_config
    FOR EACH ROW
    EXECUTE FUNCTION update_user_llm_config_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate provider values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_llm_provider_valid'
    ) THEN
        ALTER TABLE user_llm_config
        ADD CONSTRAINT check_llm_provider_valid
        CHECK (provider IN ('deepseek', 'claude', 'openai', 'local', 'gemini'));
    END IF;
END $$;

-- Validate timeout range (1 second to 30 seconds)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_llm_timeout_range'
    ) THEN
        ALTER TABLE user_llm_config
        ADD CONSTRAINT check_llm_timeout_range
        CHECK (timeout_ms >= 1000 AND timeout_ms <= 30000);
    END IF;
END $$;

-- Validate retry count (0 to 5)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_llm_retry_range'
    ) THEN
        ALTER TABLE user_llm_config
        ADD CONSTRAINT check_llm_retry_range
        CHECK (retry_count >= 0 AND retry_count <= 5);
    END IF;
END $$;

-- Validate cache duration (0 to 1 hour)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_llm_cache_range'
    ) THEN
        ALTER TABLE user_llm_config
        ADD CONSTRAINT check_llm_cache_range
        CHECK (cache_duration_sec >= 0 AND cache_duration_sec <= 3600);
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_llm_config IS 'Per-user LLM provider configuration. Replaces autopilot_settings.json LLMConfig section.';
COMMENT ON COLUMN user_llm_config.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_llm_config.enabled IS 'Master toggle for LLM features (default: true)';
COMMENT ON COLUMN user_llm_config.provider IS 'Primary LLM provider: deepseek, claude, openai, local, gemini';
COMMENT ON COLUMN user_llm_config.model IS 'Model name for primary provider (e.g., deepseek-chat, claude-3-haiku)';
COMMENT ON COLUMN user_llm_config.fallback_provider IS 'Fallback provider if primary fails';
COMMENT ON COLUMN user_llm_config.fallback_model IS 'Model name for fallback provider';
COMMENT ON COLUMN user_llm_config.timeout_ms IS 'Request timeout in milliseconds (1000-30000, default: 5000)';
COMMENT ON COLUMN user_llm_config.retry_count IS 'Number of retries on failure (0-5, default: 2)';
COMMENT ON COLUMN user_llm_config.cache_duration_sec IS 'Cache duration in seconds (0-3600, default: 300)';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_llm_config_updated_at ON user_llm_config;
-- DROP FUNCTION IF EXISTS update_user_llm_config_updated_at();
-- DROP INDEX IF EXISTS idx_user_llm_config_enabled;
-- DROP INDEX IF EXISTS idx_user_llm_config_provider;
-- DROP INDEX IF EXISTS idx_user_llm_config_user_id;
-- DROP TABLE IF EXISTS user_llm_config CASCADE;
