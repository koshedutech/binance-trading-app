-- Per-user Symbol Settings for custom ROI, position sizing, and trading preferences
-- This allows each user to have their own symbol-specific settings

CREATE TABLE IF NOT EXISTS user_symbol_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,

    -- Trading configuration
    category VARCHAR(20) DEFAULT 'neutral',           -- Performance category (excellent, good, neutral, poor, avoid)
    min_confidence DECIMAL(5,2) DEFAULT 0,            -- Override minimum confidence threshold
    max_position_usd DECIMAL(12,2) DEFAULT 0,         -- Override max position size in USD
    size_multiplier DECIMAL(4,2) DEFAULT 1.0,         -- Multiplier for position size
    leverage_override INT DEFAULT 0,                   -- Override leverage (0 = use default)
    enabled BOOLEAN DEFAULT true,                      -- Whether to trade this symbol

    -- Custom ROI settings (the main fix for multi-user isolation)
    custom_roi_percent DECIMAL(8,4) DEFAULT 0,        -- Custom ROI% for early profit booking

    -- Notes and metadata
    notes TEXT DEFAULT '',

    -- Performance tracking (per-user, per-symbol)
    total_trades INT DEFAULT 0,
    winning_trades INT DEFAULT 0,
    total_pnl DECIMAL(20,8) DEFAULT 0,
    win_rate DECIMAL(5,4) DEFAULT 0,
    avg_pnl DECIMAL(12,4) DEFAULT 0,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Unique constraint: one settings record per user per symbol
    UNIQUE(user_id, symbol)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_user_symbol_settings_user ON user_symbol_settings(user_id);
CREATE INDEX IF NOT EXISTS idx_user_symbol_settings_symbol ON user_symbol_settings(symbol);
CREATE INDEX IF NOT EXISTS idx_user_symbol_settings_enabled ON user_symbol_settings(user_id, enabled);
CREATE INDEX IF NOT EXISTS idx_user_symbol_settings_roi ON user_symbol_settings(user_id, custom_roi_percent) WHERE custom_roi_percent > 0;

-- Add comment explaining the table
COMMENT ON TABLE user_symbol_settings IS 'Per-user symbol-specific trading settings including custom ROI targets';
COMMENT ON COLUMN user_symbol_settings.custom_roi_percent IS 'Custom ROI% for early profit booking. 0 = use mode defaults';
