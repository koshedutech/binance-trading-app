-- Migration 045: User System Control Switches
-- Epic 7 Enhancement: Control switches for trading system components
-- Allows switching between legacy and new chain-based order tracking and position management

-- Create table for user system control settings (one row per user)
CREATE TABLE IF NOT EXISTS user_system_control (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Order Tracking System
    -- 'chain' = New chain-based system (order_chains, order_chain_events, position_states)
    -- 'legacy' = Old system (trade_lifecycle_events with source: ginie)
    -- 'both' = Log to both systems (for migration/testing)
    order_tracking_system VARCHAR(20) NOT NULL DEFAULT 'chain',

    -- Position Management System
    -- 'legacy' = Old Ginie position management (current default)
    -- 'chain' = New chain-based position tracking
    position_management_system VARCHAR(20) NOT NULL DEFAULT 'legacy',

    -- Entry Decision System
    -- 'legacy' = Old Ginie confluence system (current default)
    -- 'chain' = New chain-based entry decision system
    entry_decision_system VARCHAR(20) NOT NULL DEFAULT 'legacy',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure one record per user
    UNIQUE(user_id)
);

-- Create index for fast lookups by user
CREATE INDEX IF NOT EXISTS idx_user_system_control_user_id ON user_system_control(user_id);

-- Add comments for documentation
COMMENT ON TABLE user_system_control IS 'Per-user system control switches for selecting between legacy and new trading system components';
COMMENT ON COLUMN user_system_control.order_tracking_system IS 'Order tracking system: chain (new), legacy (old), or both';
COMMENT ON COLUMN user_system_control.position_management_system IS 'Position management system: legacy (Ginie) or chain (new lifecycle)';
COMMENT ON COLUMN user_system_control.entry_decision_system IS 'Entry decision system: legacy (Ginie confluence) or chain (new entry system)';
