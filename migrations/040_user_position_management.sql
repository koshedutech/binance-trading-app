-- Migration: 040_user_position_management
-- Description: Create per-user position management configuration table
-- Purpose: Store user-specific position management settings (decision mode, classic/new engine, efficiency exit, dynamic SL/TP)
-- Story: 10.1 - Position Decision Configuration
-- Date: 2026-01-18

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_position_management
-- ====================================================================================
-- Stores per-user position management settings from default-settings.json → position_management
-- Controls how positions are monitored and decisions are made (classic vs new engine)
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_position_management (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Decision Mode: "classic" or "new_engine"
    decision_mode VARCHAR(20) NOT NULL DEFAULT 'classic',

    -- Classic Decision Engine Settings
    classic_adx_reversal_threshold DECIMAL(10,4) NOT NULL DEFAULT 20.0000,
    classic_ema_reversal_periods JSONB NOT NULL DEFAULT '[9, 21]',
    classic_rsi_overbought DECIMAL(10,4) NOT NULL DEFAULT 70.0000,
    classic_rsi_oversold DECIMAL(10,4) NOT NULL DEFAULT 30.0000,
    classic_reversal_confirmations INTEGER NOT NULL DEFAULT 2,

    -- New Engine Settings
    new_engine_use_active_strategy BOOLEAN NOT NULL DEFAULT TRUE,
    new_engine_strategy_name VARCHAR(100) NOT NULL DEFAULT '',
    new_engine_use_strategy_exit_rules BOOLEAN NOT NULL DEFAULT TRUE,
    new_engine_exit_on_regime_change BOOLEAN NOT NULL DEFAULT TRUE,

    -- Efficiency Exit Settings
    efficiency_exit_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    efficiency_exit_historical_window_hours INTEGER NOT NULL DEFAULT 6,
    efficiency_exit_minimum_hold_minutes INTEGER NOT NULL DEFAULT 2,
    efficiency_exit_consecutive_signals_required INTEGER NOT NULL DEFAULT 3,

    -- Dynamic SL/TP Settings
    dynamic_sltp_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    dynamic_sltp_sl_atr_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.5000,
    dynamic_sltp_tp_atr_multiplier DECIMAL(10,4) NOT NULL DEFAULT 3.0000,
    dynamic_sltp_update_on_binance BOOLEAN NOT NULL DEFAULT TRUE,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one position management config per user
    CONSTRAINT unique_user_position_management UNIQUE(user_id)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_position_management_user_id ON user_position_management(user_id);

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_position_management_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_position_management_updated_at ON user_position_management;

CREATE TRIGGER trigger_user_position_management_updated_at
    BEFORE UPDATE ON user_position_management
    FOR EACH ROW
    EXECUTE FUNCTION update_user_position_management_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate decision_mode values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_pm_decision_mode_valid'
    ) THEN
        ALTER TABLE user_position_management
        ADD CONSTRAINT check_pm_decision_mode_valid
        CHECK (decision_mode IN ('classic', 'new_engine'));
    END IF;
END $$;

-- Validate classic_adx_reversal_threshold (0-100)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_pm_adx_threshold_range'
    ) THEN
        ALTER TABLE user_position_management
        ADD CONSTRAINT check_pm_adx_threshold_range
        CHECK (classic_adx_reversal_threshold >= 0.0 AND classic_adx_reversal_threshold <= 100.0);
    END IF;
END $$;

-- Validate classic_rsi_overbought (50-100)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_pm_rsi_overbought_range'
    ) THEN
        ALTER TABLE user_position_management
        ADD CONSTRAINT check_pm_rsi_overbought_range
        CHECK (classic_rsi_overbought >= 50.0 AND classic_rsi_overbought <= 100.0);
    END IF;
END $$;

-- Validate classic_rsi_oversold (0-50)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_pm_rsi_oversold_range'
    ) THEN
        ALTER TABLE user_position_management
        ADD CONSTRAINT check_pm_rsi_oversold_range
        CHECK (classic_rsi_oversold >= 0.0 AND classic_rsi_oversold <= 50.0);
    END IF;
END $$;

-- Validate classic_reversal_confirmations (1-10)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_pm_reversal_confirmations_range'
    ) THEN
        ALTER TABLE user_position_management
        ADD CONSTRAINT check_pm_reversal_confirmations_range
        CHECK (classic_reversal_confirmations >= 1 AND classic_reversal_confirmations <= 10);
    END IF;
END $$;

-- Validate efficiency_exit_historical_window_hours (1-72)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_pm_historical_window_range'
    ) THEN
        ALTER TABLE user_position_management
        ADD CONSTRAINT check_pm_historical_window_range
        CHECK (efficiency_exit_historical_window_hours >= 1 AND efficiency_exit_historical_window_hours <= 72);
    END IF;
END $$;

-- Validate dynamic_sltp_sl_atr_multiplier (0.1-10)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_pm_sl_atr_multiplier_range'
    ) THEN
        ALTER TABLE user_position_management
        ADD CONSTRAINT check_pm_sl_atr_multiplier_range
        CHECK (dynamic_sltp_sl_atr_multiplier >= 0.1 AND dynamic_sltp_sl_atr_multiplier <= 10.0);
    END IF;
END $$;

-- Validate dynamic_sltp_tp_atr_multiplier (0.1-20)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_pm_tp_atr_multiplier_range'
    ) THEN
        ALTER TABLE user_position_management
        ADD CONSTRAINT check_pm_tp_atr_multiplier_range
        CHECK (dynamic_sltp_tp_atr_multiplier >= 0.1 AND dynamic_sltp_tp_atr_multiplier <= 20.0);
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_position_management IS 'Per-user position management configuration. Controls decision engine mode and exit strategies.';
COMMENT ON COLUMN user_position_management.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_position_management.decision_mode IS 'Position decision engine: classic (indicator-based) or new_engine (strategy-based)';
COMMENT ON COLUMN user_position_management.classic_adx_reversal_threshold IS 'ADX threshold for detecting trend weakness in classic mode (0-100, default: 20)';
COMMENT ON COLUMN user_position_management.classic_ema_reversal_periods IS 'EMA periods for reversal detection in classic mode (JSON array, default: [9, 21])';
COMMENT ON COLUMN user_position_management.classic_rsi_overbought IS 'RSI overbought threshold in classic mode (50-100, default: 70)';
COMMENT ON COLUMN user_position_management.classic_rsi_oversold IS 'RSI oversold threshold in classic mode (0-50, default: 30)';
COMMENT ON COLUMN user_position_management.classic_reversal_confirmations IS 'Required confirmations for reversal signal (1-10, default: 2)';
COMMENT ON COLUMN user_position_management.new_engine_use_active_strategy IS 'Use active strategy for exit decisions in new engine mode';
COMMENT ON COLUMN user_position_management.new_engine_strategy_name IS 'Specific strategy name for new engine (empty = use active)';
COMMENT ON COLUMN user_position_management.new_engine_use_strategy_exit_rules IS 'Apply strategy exit rules in new engine mode';
COMMENT ON COLUMN user_position_management.new_engine_exit_on_regime_change IS 'Exit when market regime changes in new engine mode';
COMMENT ON COLUMN user_position_management.efficiency_exit_enabled IS 'Enable efficiency-based position exits';
COMMENT ON COLUMN user_position_management.efficiency_exit_historical_window_hours IS 'Hours of history to analyze for efficiency exit (1-72, default: 6)';
COMMENT ON COLUMN user_position_management.efficiency_exit_minimum_hold_minutes IS 'Minimum hold time before efficiency exit allowed';
COMMENT ON COLUMN user_position_management.efficiency_exit_consecutive_signals_required IS 'Consecutive exit signals needed for efficiency exit';
COMMENT ON COLUMN user_position_management.dynamic_sltp_enabled IS 'Enable dynamic SL/TP adjustments based on market conditions';
COMMENT ON COLUMN user_position_management.dynamic_sltp_sl_atr_multiplier IS 'ATR multiplier for stop-loss calculation (0.1-10, default: 1.5)';
COMMENT ON COLUMN user_position_management.dynamic_sltp_tp_atr_multiplier IS 'ATR multiplier for take-profit calculation (0.1-20, default: 3.0)';
COMMENT ON COLUMN user_position_management.dynamic_sltp_update_on_binance IS 'Push SL/TP updates to Binance exchange orders';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_position_management_updated_at ON user_position_management;
-- DROP FUNCTION IF EXISTS update_user_position_management_updated_at();
-- DROP INDEX IF EXISTS idx_user_position_management_user_id;
-- DROP TABLE IF EXISTS user_position_management CASCADE;
