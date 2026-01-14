-- Migration 025: User Fee Rates
-- Stores actual Binance commission rates per user (fetched from API)
-- UP

-- Create user_fee_rates table for per-user fee rate caching
CREATE TABLE IF NOT EXISTS user_fee_rates (
    id SERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE UNIQUE,

    -- Rates from Binance API (as decimals, e.g., 0.0005 = 0.05%)
    maker_rate DECIMAL(10,8) NOT NULL DEFAULT 0.0002,  -- 0.02% default
    taker_rate DECIMAL(10,8) NOT NULL DEFAULT 0.0005,  -- 0.05% default

    -- Fetch metadata
    fetched_at TIMESTAMP,                    -- When rates were last fetched from Binance
    symbol VARCHAR(20) DEFAULT 'BTCUSDT',    -- Symbol used for rate fetch (rates are same for all)

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast user lookups
CREATE INDEX IF NOT EXISTS idx_user_fee_rates_user ON user_fee_rates(user_id);

-- Comment on table
COMMENT ON TABLE user_fee_rates IS 'Stores per-user Binance commission rates from API';
COMMENT ON COLUMN user_fee_rates.maker_rate IS 'Maker fee rate as decimal (0.0002 = 0.02%)';
COMMENT ON COLUMN user_fee_rates.taker_rate IS 'Taker fee rate as decimal (0.0005 = 0.05%)';
COMMENT ON COLUMN user_fee_rates.fetched_at IS 'Timestamp when rates were last fetched from Binance API';

-- DOWN
-- DROP TABLE IF EXISTS user_fee_rates;
