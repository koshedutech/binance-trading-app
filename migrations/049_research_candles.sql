-- Migration 049: Research Candles Table
-- Epic 15: Research Infrastructure - Data, Features & Backtesting
-- Story 15.1: Database Schema for Historical Candles
-- Purpose: Store historical candle data for backtesting and research

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- Create table for storing historical candle data
-- Includes all Binance kline fields for complete OHLCV data
CREATE TABLE IF NOT EXISTS research_candles (
    id BIGSERIAL PRIMARY KEY,

    -- Candle identification
    symbol VARCHAR(20) NOT NULL,        -- Trading pair (e.g., BTCUSDT)
    timeframe VARCHAR(5) NOT NULL,      -- Candle interval (1m, 5m, 15m, 1h, 4h, 1d)
    open_time TIMESTAMPTZ NOT NULL,      -- Candle open time (UTC)
    close_time TIMESTAMPTZ NOT NULL,     -- Candle close time (UTC)

    -- OHLC price data (20 digits, 8 decimal places for crypto precision)
    open DECIMAL(20,8) NOT NULL,        -- Opening price
    high DECIMAL(20,8) NOT NULL,        -- Highest price
    low DECIMAL(20,8) NOT NULL,         -- Lowest price
    close DECIMAL(20,8) NOT NULL,       -- Closing price

    -- Volume data (30 digits for high-volume pairs)
    volume DECIMAL(30,8) NOT NULL,           -- Base asset volume
    quote_volume DECIMAL(30,8) NOT NULL,     -- Quote asset volume (USDT)

    -- Trade activity
    trade_count INTEGER NOT NULL,            -- Number of trades in candle

    -- Taker buy/sell data (for order flow analysis)
    taker_buy_volume DECIMAL(30,8) NOT NULL,      -- Taker buy base asset volume
    taker_buy_quote DECIMAL(30,8) NOT NULL,       -- Taker buy quote asset volume

    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW(),

    -- Unique constraint: one candle per symbol/timeframe/time
    UNIQUE(symbol, timeframe, open_time)
);

-- ============================================================
-- INDEXES FOR EFFICIENT QUERIES
-- ============================================================

-- Primary composite index for time-series queries
-- Descending time for efficient "most recent first" queries
CREATE INDEX IF NOT EXISTS idx_candles_symbol_tf_time
    ON research_candles(symbol, timeframe, open_time DESC);

-- Index for range queries by open_time alone (cross-symbol analysis)
CREATE INDEX IF NOT EXISTS idx_candles_open_time
    ON research_candles(open_time DESC);

-- Index for symbol-only queries (all timeframes for a symbol)
CREATE INDEX IF NOT EXISTS idx_candles_symbol
    ON research_candles(symbol);

-- Index for timeframe analysis (all symbols at a specific timeframe)
CREATE INDEX IF NOT EXISTS idx_candles_timeframe
    ON research_candles(timeframe);

-- Note: Partial index for recent data removed - PostgreSQL partial indexes with
-- NOW() expressions are evaluated at index creation time, not query time.
-- For recent data queries, use the idx_candles_symbol_tf_time index with a
-- WHERE clause filter, which PostgreSQL will optimize efficiently.

-- ============================================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================================

COMMENT ON TABLE research_candles IS 'Historical candle data for backtesting and research (Epic 15). Contains all Binance kline fields.';

COMMENT ON COLUMN research_candles.symbol IS 'Trading pair symbol (e.g., BTCUSDT, ETHUSDT)';
COMMENT ON COLUMN research_candles.timeframe IS 'Candle interval: 1m, 5m, 15m, 30m, 1h, 4h, 1d';
COMMENT ON COLUMN research_candles.open_time IS 'Candle open timestamp in UTC';
COMMENT ON COLUMN research_candles.close_time IS 'Candle close timestamp in UTC';
COMMENT ON COLUMN research_candles.open IS 'Opening price of the candle';
COMMENT ON COLUMN research_candles.high IS 'Highest price during the candle period';
COMMENT ON COLUMN research_candles.low IS 'Lowest price during the candle period';
COMMENT ON COLUMN research_candles.close IS 'Closing price of the candle';
COMMENT ON COLUMN research_candles.volume IS 'Total base asset volume traded';
COMMENT ON COLUMN research_candles.quote_volume IS 'Total quote asset (USDT) volume traded';
COMMENT ON COLUMN research_candles.trade_count IS 'Number of trades executed during the candle';
COMMENT ON COLUMN research_candles.taker_buy_volume IS 'Base asset volume bought by takers (aggressive buyers)';
COMMENT ON COLUMN research_candles.taker_buy_quote IS 'Quote asset volume of taker buys';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- To rollback this migration, execute the following:
--
-- (idx_candles_recent removed - see note above)
-- DROP INDEX IF EXISTS idx_candles_timeframe;
-- DROP INDEX IF EXISTS idx_candles_symbol;
-- DROP INDEX IF EXISTS idx_candles_open_time;
-- DROP INDEX IF EXISTS idx_candles_symbol_tf_time;
-- DROP TABLE IF EXISTS research_candles CASCADE;
