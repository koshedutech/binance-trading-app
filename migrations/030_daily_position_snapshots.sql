-- Migration 030: Create daily_position_snapshots table (Epic 8 Story 8.1)
-- Purpose: Store end-of-day position snapshots for daily P&L tracking
-- Each snapshot captures mark-to-market values at user's timezone midnight

-- Create table for daily position snapshots
CREATE TABLE IF NOT EXISTS daily_position_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    snapshot_date DATE NOT NULL,  -- Date in user's timezone
    symbol VARCHAR(20) NOT NULL,  -- Trading pair (e.g., BTCUSDT)
    position_side VARCHAR(10) NOT NULL,  -- LONG, SHORT, or BOTH
    quantity DECIMAL(20, 8) NOT NULL,  -- Position size
    entry_price DECIMAL(20, 8) NOT NULL,  -- Average entry price
    mark_price DECIMAL(20, 8) NOT NULL,  -- Mark price at snapshot time
    unrealized_pnl DECIMAL(20, 8) NOT NULL,  -- Unrealized P&L at snapshot
    mode VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN',  -- Trading mode (scalp, swing, position, ultra_fast, UNKNOWN)
    client_order_id VARCHAR(50),  -- Original clientOrderId if available
    leverage INT NOT NULL DEFAULT 1,  -- Position leverage
    margin_type VARCHAR(10) DEFAULT 'CROSSED',  -- CROSSED or ISOLATED
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Composite unique constraint: one snapshot per position per day
    CONSTRAINT uq_position_snapshot_daily UNIQUE (user_id, snapshot_date, symbol, position_side)
);

-- Index for efficient queries by user and date
CREATE INDEX IF NOT EXISTS idx_position_snapshots_user_date
    ON daily_position_snapshots(user_id, snapshot_date);

-- Index for querying by mode (for mode analytics)
CREATE INDEX IF NOT EXISTS idx_position_snapshots_mode
    ON daily_position_snapshots(mode);

-- Index for date range queries
CREATE INDEX IF NOT EXISTS idx_position_snapshots_date
    ON daily_position_snapshots(snapshot_date);

-- Add comments for documentation
COMMENT ON TABLE daily_position_snapshots IS 'End-of-day position snapshots for daily P&L tracking (Epic 8)';
COMMENT ON COLUMN daily_position_snapshots.snapshot_date IS 'Date in user timezone when snapshot was taken';
COMMENT ON COLUMN daily_position_snapshots.mode IS 'Trading mode extracted from clientOrderId: scalp, swing, position, ultra_fast, or UNKNOWN';
COMMENT ON COLUMN daily_position_snapshots.unrealized_pnl IS 'Unrealized P&L calculated from mark price at snapshot time';

-- Rollback script:
-- DROP TABLE IF EXISTS daily_position_snapshots;
