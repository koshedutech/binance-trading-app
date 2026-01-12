-- Migration 023: User Safety Settings
-- Story 9.4: Per-user safety controls for rate limiting, profit monitoring, and win-rate monitoring
-- These settings protect against excessive trading and consecutive losses

-- Create table for user safety settings (one row per user per mode)
CREATE TABLE IF NOT EXISTS user_safety_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(20) NOT NULL,  -- ultra_fast, scalp, swing, position

    -- Rate limiting
    max_trades_per_minute INTEGER NOT NULL DEFAULT 5,
    max_trades_per_hour INTEGER NOT NULL DEFAULT 20,
    max_trades_per_day INTEGER NOT NULL DEFAULT 50,

    -- Cumulative profit monitoring
    enable_profit_monitor BOOLEAN NOT NULL DEFAULT true,
    profit_window_minutes INTEGER NOT NULL DEFAULT 10,
    max_loss_percent_in_window DECIMAL(5,2) NOT NULL DEFAULT -1.5,
    pause_cooldown_minutes INTEGER NOT NULL DEFAULT 30,

    -- Win-rate monitoring
    enable_win_rate_monitor BOOLEAN NOT NULL DEFAULT true,
    win_rate_sample_size INTEGER NOT NULL DEFAULT 15,
    min_win_rate_threshold DECIMAL(5,2) NOT NULL DEFAULT 50,
    win_rate_cooldown_minutes INTEGER NOT NULL DEFAULT 60,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure one record per user per mode
    UNIQUE(user_id, mode)
);

-- Create index for fast lookups by user
CREATE INDEX IF NOT EXISTS idx_user_safety_settings_user_id ON user_safety_settings(user_id);

-- Create index for fast lookups by user and mode
CREATE INDEX IF NOT EXISTS idx_user_safety_settings_user_mode ON user_safety_settings(user_id, mode);

-- Add comments for documentation
COMMENT ON TABLE user_safety_settings IS 'Per-user safety controls per trading mode for rate limiting, profit monitoring, and win-rate monitoring';
COMMENT ON COLUMN user_safety_settings.mode IS 'Trading mode: ultra_fast, scalp, swing, position';
COMMENT ON COLUMN user_safety_settings.max_trades_per_minute IS 'Maximum trades allowed per minute';
COMMENT ON COLUMN user_safety_settings.max_trades_per_hour IS 'Maximum trades allowed per hour';
COMMENT ON COLUMN user_safety_settings.max_trades_per_day IS 'Maximum trades allowed per day';
COMMENT ON COLUMN user_safety_settings.enable_profit_monitor IS 'Enable cumulative profit monitoring in time window';
COMMENT ON COLUMN user_safety_settings.profit_window_minutes IS 'Size of profit monitoring window in minutes';
COMMENT ON COLUMN user_safety_settings.max_loss_percent_in_window IS 'Maximum cumulative loss percent allowed in window (negative value)';
COMMENT ON COLUMN user_safety_settings.pause_cooldown_minutes IS 'Minutes to pause trading after hitting loss threshold';
COMMENT ON COLUMN user_safety_settings.enable_win_rate_monitor IS 'Enable win-rate monitoring';
COMMENT ON COLUMN user_safety_settings.win_rate_sample_size IS 'Number of recent trades to calculate win rate';
COMMENT ON COLUMN user_safety_settings.min_win_rate_threshold IS 'Minimum win rate percentage required to continue trading';
COMMENT ON COLUMN user_safety_settings.win_rate_cooldown_minutes IS 'Minutes to pause trading if win rate below threshold';
