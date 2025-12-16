-- Backtest Results Table
CREATE TABLE IF NOT EXISTS backtest_results (
    id BIGSERIAL PRIMARY KEY,
    strategy_config_id BIGINT NOT NULL REFERENCES strategy_configs(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    interval VARCHAR(10) NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,

    -- Performance Metrics
    total_trades INT NOT NULL DEFAULT 0,
    winning_trades INT NOT NULL DEFAULT 0,
    losing_trades INT NOT NULL DEFAULT 0,
    win_rate DECIMAL(5, 2),

    total_pnl DECIMAL(20, 8) NOT NULL DEFAULT 0,
    total_fees DECIMAL(20, 8) NOT NULL DEFAULT 0,
    net_pnl DECIMAL(20, 8) NOT NULL DEFAULT 0,

    average_win DECIMAL(20, 8),
    average_loss DECIMAL(20, 8),
    largest_win DECIMAL(20, 8),
    largest_loss DECIMAL(20, 8),

    profit_factor DECIMAL(10, 4),
    max_drawdown DECIMAL(20, 8),
    max_drawdown_percent DECIMAL(5, 2),

    -- Trade Distribution
    avg_trade_duration_minutes INT,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Backtest Trades Table
CREATE TABLE IF NOT EXISTS backtest_trades (
    id BIGSERIAL PRIMARY KEY,
    backtest_result_id BIGINT NOT NULL REFERENCES backtest_results(id) ON DELETE CASCADE,

    entry_time TIMESTAMP NOT NULL,
    entry_price DECIMAL(20, 8) NOT NULL,
    entry_reason TEXT,

    exit_time TIMESTAMP NOT NULL,
    exit_price DECIMAL(20, 8) NOT NULL,
    exit_reason TEXT,

    quantity DECIMAL(20, 8) NOT NULL,
    side VARCHAR(10) NOT NULL, -- BUY/SELL

    pnl DECIMAL(20, 8) NOT NULL,
    pnl_percent DECIMAL(10, 4) NOT NULL,
    fees DECIMAL(20, 8) NOT NULL DEFAULT 0,

    duration_minutes INT NOT NULL,

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_backtest_results_strategy ON backtest_results(strategy_config_id);
CREATE INDEX idx_backtest_results_symbol ON backtest_results(symbol);
CREATE INDEX idx_backtest_results_dates ON backtest_results(start_date, end_date);
CREATE INDEX idx_backtest_trades_result ON backtest_trades(backtest_result_id);
CREATE INDEX idx_backtest_trades_entry_time ON backtest_trades(entry_time);
