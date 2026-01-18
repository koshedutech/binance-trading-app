-- Migration 037: Calibration Data Storage
-- Epic 11: Position Decision Engine
-- Story 11-25: Calibration Data Storage
--
-- Purpose: Persistent storage for system-learned calibration data (not user settings).
-- Calibration data tracks historical trade outcomes by score bucket to improve
-- decision confidence over time.
--
-- Architecture:
--   - PostgreSQL table for persistence
--   - Redis cache for fast reads (handled by CalibrationService)
--   - Write-through pattern: Save to DB + Redis on every update
--   - Read pattern: Check Redis first, fall back to DB, populate Redis
--
-- Date: 2026-01-18

-- ============================================================================
-- TABLE: calibration_data
-- ============================================================================
CREATE TABLE IF NOT EXISTS calibration_data (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    strategy VARCHAR(50) NOT NULL,          -- "trend_following", "mean_reversion", etc.
    indicator_hash VARCHAR(16) NOT NULL,    -- SHA256 hash (first 8 chars) of selected indicators
    indicator_list JSONB NOT NULL DEFAULT '[]',  -- ["rsi", "ema_cross", "atr"]

    -- Score bucket data (0-24, 25-49, 50-74, 75-100)
    bucket_0_24 JSONB NOT NULL DEFAULT '{}',
    bucket_25_49 JSONB NOT NULL DEFAULT '{}',
    bucket_50_74 JSONB NOT NULL DEFAULT '{}',
    bucket_75_100 JSONB NOT NULL DEFAULT '{}',

    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}',

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Unique constraint per user/strategy/indicator combination
    CONSTRAINT unique_user_strategy_indicator UNIQUE(user_id, strategy, indicator_hash)
);

-- ============================================================================
-- INDEXES
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_calibration_user_id ON calibration_data(user_id);
CREATE INDEX IF NOT EXISTS idx_calibration_strategy ON calibration_data(strategy);
CREATE INDEX IF NOT EXISTS idx_calibration_updated ON calibration_data(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_calibration_user_strategy ON calibration_data(user_id, strategy);

-- ============================================================================
-- COMMENTS
-- ============================================================================
COMMENT ON TABLE calibration_data IS 'System-learned calibration data for Decision Engine score buckets';

COMMENT ON COLUMN calibration_data.user_id IS 'User who owns this calibration data';
COMMENT ON COLUMN calibration_data.strategy IS 'Trading strategy name (trend_following, mean_reversion, etc.)';
COMMENT ON COLUMN calibration_data.indicator_hash IS 'SHA256 hash (first 8 chars) of sorted indicator list for versioning';
COMMENT ON COLUMN calibration_data.indicator_list IS 'JSON array of indicator names used for this calibration';
COMMENT ON COLUMN calibration_data.bucket_0_24 IS 'Score bucket 0-24: {total_trades, winning_trades, win_rate, avg_pnl_percent, total_pnl_percent}';
COMMENT ON COLUMN calibration_data.bucket_25_49 IS 'Score bucket 25-49: {total_trades, winning_trades, win_rate, avg_pnl_percent, total_pnl_percent}';
COMMENT ON COLUMN calibration_data.bucket_50_74 IS 'Score bucket 50-74: {total_trades, winning_trades, win_rate, avg_pnl_percent, total_pnl_percent}';
COMMENT ON COLUMN calibration_data.bucket_75_100 IS 'Score bucket 75-100: {total_trades, winning_trades, win_rate, avg_pnl_percent, total_pnl_percent}';
COMMENT ON COLUMN calibration_data.metadata IS 'JSON object with first_trade_date, last_updated, total_calibration_trades';

-- ============================================================================
-- TRIGGER: Auto-update updated_at on modification
-- ============================================================================
DROP TRIGGER IF EXISTS update_calibration_data_updated_at ON calibration_data;
CREATE TRIGGER update_calibration_data_updated_at
    BEFORE UPDATE ON calibration_data
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- CALIBRATION HISTORY TABLE (for indicator changes)
-- ============================================================================
-- When user changes indicators, old calibration is archived here for reference
CREATE TABLE IF NOT EXISTS calibration_data_history (
    id SERIAL PRIMARY KEY,
    original_id INTEGER NOT NULL,           -- Original calibration_data.id
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    strategy VARCHAR(50) NOT NULL,
    indicator_hash VARCHAR(16) NOT NULL,
    indicator_list JSONB NOT NULL DEFAULT '[]',
    bucket_0_24 JSONB NOT NULL DEFAULT '{}',
    bucket_25_49 JSONB NOT NULL DEFAULT '{}',
    bucket_50_74 JSONB NOT NULL DEFAULT '{}',
    bucket_75_100 JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    archived_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    archive_reason VARCHAR(50) DEFAULT 'INDICATOR_CHANGE'  -- INDICATOR_CHANGE, USER_RESET, etc.
);

CREATE INDEX IF NOT EXISTS idx_calibration_history_user ON calibration_data_history(user_id);
CREATE INDEX IF NOT EXISTS idx_calibration_history_strategy ON calibration_data_history(user_id, strategy);
CREATE INDEX IF NOT EXISTS idx_calibration_history_archived ON calibration_data_history(archived_at DESC);

COMMENT ON TABLE calibration_data_history IS 'Archived calibration data when indicators change or user resets';
COMMENT ON COLUMN calibration_data_history.archive_reason IS 'Why this calibration was archived: INDICATOR_CHANGE, USER_RESET, etc.';
