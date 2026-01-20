-- Migration: 044_user_coin_confluence_config.sql
-- Story 9.12 Phase 3: Migrate per-coin confluence configuration to database
-- This table stores per-coin entry confluence settings (ADX, VWAP, Volume, Pivots, EMA thresholds)

CREATE TABLE IF NOT EXISTS user_coin_confluence_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(36) NOT NULL,
    symbol VARCHAR(20) NOT NULL,

    -- Tier classification
    tier VARCHAR(20) DEFAULT 'mid_cap',
    enabled BOOLEAN DEFAULT true,

    -- ADX settings
    adx_multiplier DECIMAL(5,2) DEFAULT 1.0,
    scalp_adx DECIMAL(5,2) DEFAULT 0,
    swing_adx DECIMAL(5,2) DEFAULT 0,
    position_adx DECIMAL(5,2) DEFAULT 0,

    -- Volume settings
    volume_multiplier DECIMAL(5,2) DEFAULT 1.0,
    volume_period INT DEFAULT 20,

    -- VWAP settings
    vwap_period INT DEFAULT 20,
    vwap_enabled BOOLEAN DEFAULT true,

    -- Pivot settings
    pivot_proximity DECIMAL(5,2) DEFAULT 0.5,
    pivot_enabled BOOLEAN DEFAULT true,

    -- EMA settings
    ema_fast_period INT DEFAULT 20,
    ema_slow_period INT DEFAULT 50,
    ema_enabled BOOLEAN DEFAULT true,

    -- Confluence requirements
    min_confluence INT DEFAULT 2,
    scalp_min_confluence INT DEFAULT 2,

    -- Custom weights (for future extensibility)
    custom_weights JSONB,

    -- Notes
    notes TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Unique constraint: one config per user/symbol pair
    UNIQUE(user_id, symbol)
);

-- Index for fast lookups by user_id
CREATE INDEX IF NOT EXISTS idx_coin_confluence_user_id ON user_coin_confluence_config(user_id);

-- Comment on table
COMMENT ON TABLE user_coin_confluence_config IS 'Per-coin entry confluence configuration for trading decisions (Story 9.12 Phase 3)';
