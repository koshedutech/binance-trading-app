-- Migration 031: Create daily_mode_summaries table (Epic 8 Story 8.3)
-- Purpose: Store aggregated daily P&L summaries by trading mode
-- This is the main settlement data table for billing and analytics

-- Create table for daily mode summaries
CREATE TABLE IF NOT EXISTS daily_mode_summaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    summary_date DATE NOT NULL,              -- Date in user's timezone
    mode VARCHAR(20) NOT NULL,               -- scalp, swing, position, ultra_fast, UNKNOWN, ALL

    -- Trade metrics
    trade_count INT NOT NULL DEFAULT 0,
    win_count INT NOT NULL DEFAULT 0,
    loss_count INT NOT NULL DEFAULT 0,
    win_rate DECIMAL(5, 2) NOT NULL DEFAULT 0,  -- Percentage (0-100)

    -- P&L metrics
    realized_pnl DECIMAL(20, 8) NOT NULL DEFAULT 0,
    unrealized_pnl DECIMAL(20, 8) NOT NULL DEFAULT 0,      -- Today's unrealized at EOD
    unrealized_pnl_change DECIMAL(20, 8) NOT NULL DEFAULT 0, -- Change from yesterday
    total_pnl DECIMAL(20, 8) NOT NULL DEFAULT 0,           -- realized + unrealized_change

    -- Trade details
    largest_win DECIMAL(20, 8) NOT NULL DEFAULT 0,
    largest_loss DECIMAL(20, 8) NOT NULL DEFAULT 0,
    total_volume DECIMAL(20, 8) NOT NULL DEFAULT 0,       -- Total trading volume in USDT
    avg_trade_size DECIMAL(20, 8) NOT NULL DEFAULT 0,

    -- Capital metrics (from Story 8.7)
    starting_balance DECIMAL(20, 8),
    ending_balance DECIMAL(20, 8),
    max_capital_used DECIMAL(20, 8),
    avg_capital_used DECIMAL(20, 8),
    max_drawdown DECIMAL(20, 8),
    peak_balance DECIMAL(20, 8),

    -- Fees
    total_fees DECIMAL(20, 8) NOT NULL DEFAULT 0,

    -- Settlement metadata
    settlement_status VARCHAR(20) NOT NULL DEFAULT 'completed', -- completed, failed, retrying
    settlement_error TEXT,
    settlement_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    user_timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',

    -- Data quality (from Story 8.10)
    data_quality_flag BOOLEAN NOT NULL DEFAULT false,
    data_quality_notes TEXT,
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMP WITH TIME ZONE,
    alerted BOOLEAN NOT NULL DEFAULT false,  -- For failure alerting (Story 8.9)

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Composite unique constraint: one summary per user per date per mode
    CONSTRAINT uq_daily_summary UNIQUE (user_id, summary_date, mode)
);

-- Index for efficient queries by user and date (most common query)
CREATE INDEX IF NOT EXISTS idx_daily_summaries_user_date
    ON daily_mode_summaries(user_id, summary_date DESC);

-- Index for mode comparison queries
CREATE INDEX IF NOT EXISTS idx_daily_summaries_mode
    ON daily_mode_summaries(mode, summary_date DESC);

-- Index for date range queries
CREATE INDEX IF NOT EXISTS idx_daily_summaries_date_range
    ON daily_mode_summaries(summary_date, user_id);

-- Index for settlement status (failure monitoring)
CREATE INDEX IF NOT EXISTS idx_daily_summaries_status
    ON daily_mode_summaries(settlement_status) WHERE settlement_status != 'completed';

-- Index for admin review queue (data quality flagged items)
CREATE INDEX IF NOT EXISTS idx_daily_summaries_review_queue
    ON daily_mode_summaries(data_quality_flag, reviewed_at) WHERE data_quality_flag = true;

-- Index for alert monitoring (unalerted failures)
CREATE INDEX IF NOT EXISTS idx_daily_summaries_unalerted
    ON daily_mode_summaries(alerted, settlement_status, settlement_time)
    WHERE settlement_status = 'failed' AND alerted = false;

-- Add comments for documentation
COMMENT ON TABLE daily_mode_summaries IS 'Daily P&L summaries aggregated by trading mode (Epic 8)';
COMMENT ON COLUMN daily_mode_summaries.summary_date IS 'Date in user timezone when trading occurred';
COMMENT ON COLUMN daily_mode_summaries.mode IS 'Trading mode: scalp, swing, position, ultra_fast, UNKNOWN, or ALL';
COMMENT ON COLUMN daily_mode_summaries.unrealized_pnl_change IS 'Change in unrealized P&L from yesterday (matches Binance daily P&L method)';
COMMENT ON COLUMN daily_mode_summaries.total_pnl IS 'Total daily P&L = realized_pnl + unrealized_pnl_change';
COMMENT ON COLUMN daily_mode_summaries.settlement_status IS 'Settlement status: completed, failed, or retrying';
COMMENT ON COLUMN daily_mode_summaries.data_quality_flag IS 'True if data has anomalies requiring admin review';

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_daily_mode_summaries_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-update updated_at
DROP TRIGGER IF EXISTS trigger_daily_mode_summaries_updated_at ON daily_mode_summaries;
CREATE TRIGGER trigger_daily_mode_summaries_updated_at
    BEFORE UPDATE ON daily_mode_summaries
    FOR EACH ROW
    EXECUTE FUNCTION update_daily_mode_summaries_updated_at();

-- CHECK constraints for data integrity (Issue #5 from code review)
-- Using ALTER TABLE to be idempotent if table already exists

-- Constraint: win_rate must be between 0 and 100
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_win_rate_range'
    ) THEN
        ALTER TABLE daily_mode_summaries
        ADD CONSTRAINT chk_win_rate_range CHECK (win_rate >= 0 AND win_rate <= 100);
    END IF;
END $$;

-- Constraint: mode must be a valid value
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_valid_mode'
    ) THEN
        ALTER TABLE daily_mode_summaries
        ADD CONSTRAINT chk_valid_mode CHECK (mode IN ('scalp', 'swing', 'position', 'ultra_fast', 'UNKNOWN', 'ALL'));
    END IF;
END $$;

-- Constraint: settlement_status must be a valid value
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_valid_settlement_status'
    ) THEN
        ALTER TABLE daily_mode_summaries
        ADD CONSTRAINT chk_valid_settlement_status CHECK (settlement_status IN ('completed', 'failed', 'retrying'));
    END IF;
END $$;

-- Rollback script (keep in comments for reference):
-- DROP TABLE IF EXISTS daily_mode_summaries;
