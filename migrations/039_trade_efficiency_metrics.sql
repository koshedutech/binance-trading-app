-- Migration 039: Trade Efficiency Metrics
-- Epic 10: Exit Optimization Engine
-- Story 10.1: Database Implementation for Trade Efficiency Metrics
--
-- Purpose: Stores exit efficiency data for historical baseline calculation.
-- This table captures detailed metrics about how efficiently trades were exited
-- relative to their peak profit, enabling data-driven exit optimization.
--
-- Date: 2026-01-18

-- ============================================================================
-- TABLE: trade_efficiency_metrics
-- ============================================================================
CREATE TABLE trade_efficiency_metrics (
    id SERIAL PRIMARY KEY,
    futures_trade_id INTEGER REFERENCES futures_trades(id),
    user_id UUID NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    mode VARCHAR(20) NOT NULL,

    -- Entry/Exit Data
    entry_price DECIMAL(20,8) NOT NULL,
    exit_price DECIMAL(20,8) NOT NULL,
    entry_time TIMESTAMP NOT NULL,
    exit_time TIMESTAMP NOT NULL,

    -- Quantity
    original_qty DECIMAL(20,8) NOT NULL,
    exit_qty DECIMAL(20,8) NOT NULL,

    -- Efficiency Data
    peak_profit_pct DECIMAL(10,6) NOT NULL,
    exit_profit_pct DECIMAL(10,6) NOT NULL,
    exit_efficiency DECIMAL(10,6) NOT NULL,

    -- Exit Details
    exit_reason VARCHAR(50) NOT NULL,
    exit_urgency VARCHAR(20),

    -- Stage Data
    breakeven_achieved BOOLEAN DEFAULT FALSE,
    breakeven_time TIMESTAMP,
    tp1_hit BOOLEAN DEFAULT FALSE,
    tp1_time TIMESTAMP,
    tp1_qty DECIMAL(20,8),
    tp1_profit DECIMAL(20,8),

    -- Decision Engine Data
    decision_mode VARCHAR(20) NOT NULL,
    entry_strategy VARCHAR(50),
    entry_regime VARCHAR(20),
    exit_regime VARCHAR(20),

    -- Indicator Scores at Exit
    trend_score DECIMAL(5,2),
    momentum_score DECIMAL(5,2),
    volatility_score DECIMAL(5,2),
    volume_score DECIMAL(5,2),

    -- Classic Indicators at Exit
    adx_at_exit DECIMAL(10,4),
    rsi_at_exit DECIMAL(10,4),
    atr_pct_at_exit DECIMAL(10,4),
    trend_direction VARCHAR(20),
    trend_strength DECIMAL(10,6),

    -- Category
    trade_category INTEGER NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- INDEXES for baseline query performance
-- ============================================================================
-- Primary index for baseline calculations by user/mode/time window
CREATE INDEX idx_eff_user_mode_time ON trade_efficiency_metrics(user_id, mode, created_at);

-- Category-based queries for strategy-specific analysis
CREATE INDEX idx_eff_category ON trade_efficiency_metrics(trade_category);

-- Entry strategy analysis
CREATE INDEX idx_eff_strategy ON trade_efficiency_metrics(entry_strategy);

-- Market regime analysis
CREATE INDEX idx_eff_regime ON trade_efficiency_metrics(entry_regime);

-- Lookup by original trade
CREATE INDEX idx_eff_trade_id ON trade_efficiency_metrics(futures_trade_id);

-- Symbol-specific queries
CREATE INDEX idx_eff_symbol ON trade_efficiency_metrics(symbol, created_at);

-- ============================================================================
-- COMMENTS
-- ============================================================================
COMMENT ON TABLE trade_efficiency_metrics IS 'Exit efficiency data for historical baseline calculation in Exit Optimization Engine';

COMMENT ON COLUMN trade_efficiency_metrics.futures_trade_id IS 'Reference to original futures_trades record';
COMMENT ON COLUMN trade_efficiency_metrics.user_id IS 'User who executed the trade';
COMMENT ON COLUMN trade_efficiency_metrics.symbol IS 'Trading symbol (e.g., BTCUSDT)';
COMMENT ON COLUMN trade_efficiency_metrics.mode IS 'Trading mode: ultra_fast, scalp, swing, position';
COMMENT ON COLUMN trade_efficiency_metrics.entry_price IS 'Position entry price';
COMMENT ON COLUMN trade_efficiency_metrics.exit_price IS 'Position exit price';
COMMENT ON COLUMN trade_efficiency_metrics.entry_time IS 'When position was entered';
COMMENT ON COLUMN trade_efficiency_metrics.exit_time IS 'When position was exited';
COMMENT ON COLUMN trade_efficiency_metrics.original_qty IS 'Original position quantity';
COMMENT ON COLUMN trade_efficiency_metrics.exit_qty IS 'Quantity exited in this trade';
COMMENT ON COLUMN trade_efficiency_metrics.peak_profit_pct IS 'Maximum profit percentage achieved during trade';
COMMENT ON COLUMN trade_efficiency_metrics.exit_profit_pct IS 'Actual profit percentage at exit';
COMMENT ON COLUMN trade_efficiency_metrics.exit_efficiency IS 'exit_profit_pct / peak_profit_pct * 100 (how much of peak was captured)';
COMMENT ON COLUMN trade_efficiency_metrics.exit_reason IS 'Why trade was closed: TP_HIT, SL_HIT, TRAILING_STOP, MANUAL, etc.';
COMMENT ON COLUMN trade_efficiency_metrics.exit_urgency IS 'Exit urgency level: LOW, MEDIUM, HIGH, CRITICAL';
COMMENT ON COLUMN trade_efficiency_metrics.breakeven_achieved IS 'Whether breakeven stage was reached';
COMMENT ON COLUMN trade_efficiency_metrics.breakeven_time IS 'When breakeven was achieved';
COMMENT ON COLUMN trade_efficiency_metrics.tp1_hit IS 'Whether first take-profit target was hit';
COMMENT ON COLUMN trade_efficiency_metrics.tp1_time IS 'When TP1 was hit';
COMMENT ON COLUMN trade_efficiency_metrics.tp1_qty IS 'Quantity sold at TP1';
COMMENT ON COLUMN trade_efficiency_metrics.tp1_profit IS 'Profit captured at TP1';
COMMENT ON COLUMN trade_efficiency_metrics.decision_mode IS 'Decision engine mode: MANUAL, SEMI_AUTO, AUTO';
COMMENT ON COLUMN trade_efficiency_metrics.entry_strategy IS 'Strategy used for entry: trend_following, mean_reversion, etc.';
COMMENT ON COLUMN trade_efficiency_metrics.entry_regime IS 'Market regime at entry: TRENDING, RANGING, VOLATILE';
COMMENT ON COLUMN trade_efficiency_metrics.exit_regime IS 'Market regime at exit';
COMMENT ON COLUMN trade_efficiency_metrics.trend_score IS 'Trend indicator score at exit (0-100)';
COMMENT ON COLUMN trade_efficiency_metrics.momentum_score IS 'Momentum indicator score at exit (0-100)';
COMMENT ON COLUMN trade_efficiency_metrics.volatility_score IS 'Volatility indicator score at exit (0-100)';
COMMENT ON COLUMN trade_efficiency_metrics.volume_score IS 'Volume indicator score at exit (0-100)';
COMMENT ON COLUMN trade_efficiency_metrics.adx_at_exit IS 'ADX value at exit';
COMMENT ON COLUMN trade_efficiency_metrics.rsi_at_exit IS 'RSI value at exit';
COMMENT ON COLUMN trade_efficiency_metrics.atr_pct_at_exit IS 'ATR as percentage of price at exit';
COMMENT ON COLUMN trade_efficiency_metrics.trend_direction IS 'Trend direction at exit: UP, DOWN, NEUTRAL';
COMMENT ON COLUMN trade_efficiency_metrics.trend_strength IS 'Trend strength at exit (0-1)';
COMMENT ON COLUMN trade_efficiency_metrics.trade_category IS 'Trade category for classification';
