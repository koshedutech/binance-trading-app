-- Migration 038: User Daily PnL Summaries Cache
-- Epic 13: P&L Analytics & Reporting
-- Story 13.1: PNL Summary Caching & Historical Navigation
--
-- Purpose: Cache daily PnL summaries per user to avoid repeated Binance API calls.
-- Historical days never change, so we fetch once and store permanently.
-- Only TODAY's data needs live fetching.
--
-- Architecture:
--   - One record per user per day
--   - Immutable once created (historical data doesn't change)
--   - TODAY's record is NOT stored here (always fetched live)
--
-- Date: 2026-01-18

-- ============================================================================
-- TABLE: user_daily_pnl_summaries
-- ============================================================================
CREATE TABLE IF NOT EXISTS user_daily_pnl_summaries (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,

    -- P&L components (from Binance income history)
    pnl DECIMAL(20,8) NOT NULL DEFAULT 0,           -- REALIZED_PNL total
    commission DECIMAL(20,8) NOT NULL DEFAULT 0,     -- Commission fees paid (positive)
    funding DECIMAL(20,8) NOT NULL DEFAULT 0,        -- Net funding fee (positive = paid, negative = received)
    net_pnl DECIMAL(20,8) NOT NULL DEFAULT 0,        -- Final P&L after all fees

    -- Trade metrics
    trade_count INTEGER NOT NULL DEFAULT 0,          -- Count of REALIZED_PNL records

    -- Additional fee details (optional, for breakdown)
    rebate DECIMAL(20,8) NOT NULL DEFAULT 0,         -- Commission rebates
    other DECIMAL(20,8) NOT NULL DEFAULT 0,          -- Other income types

    -- Metadata
    fetched_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),  -- When data was fetched from Binance
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Unique constraint: one record per user per day
    CONSTRAINT unique_user_daily_pnl UNIQUE(user_id, date)
);

-- ============================================================================
-- INDEXES
-- ============================================================================
-- Primary lookup: user + date (descending for recent-first queries)
CREATE INDEX IF NOT EXISTS idx_user_daily_pnl_user_date
    ON user_daily_pnl_summaries(user_id, date DESC);

-- Date range queries
CREATE INDEX IF NOT EXISTS idx_user_daily_pnl_date
    ON user_daily_pnl_summaries(date DESC);

-- User lookup
CREATE INDEX IF NOT EXISTS idx_user_daily_pnl_user
    ON user_daily_pnl_summaries(user_id);

-- ============================================================================
-- COMMENTS
-- ============================================================================
COMMENT ON TABLE user_daily_pnl_summaries IS 'Cached daily P&L summaries per user - historical data never changes';

COMMENT ON COLUMN user_daily_pnl_summaries.user_id IS 'User who owns this P&L record';
COMMENT ON COLUMN user_daily_pnl_summaries.date IS 'UTC date for this P&L summary (Binance uses UTC)';
COMMENT ON COLUMN user_daily_pnl_summaries.pnl IS 'Gross realized P&L from REALIZED_PNL records';
COMMENT ON COLUMN user_daily_pnl_summaries.commission IS 'Total commission fees paid (stored as positive)';
COMMENT ON COLUMN user_daily_pnl_summaries.funding IS 'Net funding fee: positive = paid, negative = received';
COMMENT ON COLUMN user_daily_pnl_summaries.net_pnl IS 'Net P&L after all fees (pnl - commission + rebate - funding + other)';
COMMENT ON COLUMN user_daily_pnl_summaries.trade_count IS 'Number of trades (REALIZED_PNL records) for this day';
COMMENT ON COLUMN user_daily_pnl_summaries.rebate IS 'Commission rebates (BNB discount, referral kickback, etc.)';
COMMENT ON COLUMN user_daily_pnl_summaries.other IS 'Other income types not categorized';
COMMENT ON COLUMN user_daily_pnl_summaries.fetched_at IS 'Timestamp when this data was fetched from Binance API';
