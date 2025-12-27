package database

import (
	"context"
	"log"
)

// RunFuturesMigrations runs futures-related database migrations
func (db *DB) RunFuturesMigrations(ctx context.Context) error {
	log.Println("Running Futures database migrations...")

	migrations := []string{
		// Create futures_trades table
		`CREATE TABLE IF NOT EXISTS futures_trades (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL,
			position_side VARCHAR(10) NOT NULL DEFAULT 'BOTH',
			side VARCHAR(10) NOT NULL,
			entry_price DECIMAL(20, 8) NOT NULL,
			exit_price DECIMAL(20, 8),
			mark_price DECIMAL(20, 8),
			quantity DECIMAL(20, 8) NOT NULL,
			leverage INTEGER NOT NULL DEFAULT 1,
			margin_type VARCHAR(10) NOT NULL DEFAULT 'CROSSED',
			isolated_margin DECIMAL(20, 8),
			realized_pnl DECIMAL(20, 8),
			unrealized_pnl DECIMAL(20, 8),
			realized_pnl_percent DECIMAL(10, 4),
			liquidation_price DECIMAL(20, 8),
			stop_loss DECIMAL(20, 8),
			take_profit DECIMAL(20, 8),
			trailing_stop DECIMAL(10, 4),
			status VARCHAR(20) NOT NULL DEFAULT 'OPEN',
			entry_time TIMESTAMP NOT NULL,
			exit_time TIMESTAMP,
			trade_source VARCHAR(20) NOT NULL DEFAULT 'manual',
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_trades_symbol ON futures_trades(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_trades_status ON futures_trades(status)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_trades_entry_time ON futures_trades(entry_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_trades_position_side ON futures_trades(position_side)`,

		// Create futures_orders table
		`CREATE TABLE IF NOT EXISTS futures_orders (
			id SERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			position_side VARCHAR(10) NOT NULL DEFAULT 'BOTH',
			side VARCHAR(10) NOT NULL,
			order_type VARCHAR(30) NOT NULL,
			price DECIMAL(20, 8),
			avg_price DECIMAL(20, 8),
			stop_price DECIMAL(20, 8),
			quantity DECIMAL(20, 8) NOT NULL,
			executed_qty DECIMAL(20, 8) DEFAULT 0,
			time_in_force VARCHAR(10) DEFAULT 'GTC',
			reduce_only BOOLEAN DEFAULT FALSE,
			close_position BOOLEAN DEFAULT FALSE,
			working_type VARCHAR(20) DEFAULT 'CONTRACT_PRICE',
			status VARCHAR(20) NOT NULL DEFAULT 'NEW',
			futures_trade_id INTEGER REFERENCES futures_trades(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			filled_at TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_orders_symbol ON futures_orders(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_orders_status ON futures_orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_orders_order_id ON futures_orders(order_id)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_orders_created_at ON futures_orders(created_at DESC)`,

		// Create funding_fees table
		`CREATE TABLE IF NOT EXISTS funding_fees (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL,
			funding_rate DECIMAL(20, 10) NOT NULL,
			funding_fee DECIMAL(20, 8) NOT NULL,
			position_amt DECIMAL(20, 8) NOT NULL,
			asset VARCHAR(10) NOT NULL DEFAULT 'USDT',
			timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_funding_fees_symbol ON funding_fees(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_funding_fees_timestamp ON funding_fees(timestamp DESC)`,

		// Create futures_transactions table
		`CREATE TABLE IF NOT EXISTS futures_transactions (
			id SERIAL PRIMARY KEY,
			transaction_id BIGINT NOT NULL,
			symbol VARCHAR(20),
			income_type VARCHAR(30) NOT NULL,
			income DECIMAL(20, 8) NOT NULL,
			asset VARCHAR(10) NOT NULL DEFAULT 'USDT',
			info TEXT,
			timestamp TIMESTAMP NOT NULL,
			futures_trade_id INTEGER REFERENCES futures_trades(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_transactions_symbol ON futures_transactions(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_transactions_income_type ON futures_transactions(income_type)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_transactions_timestamp ON futures_transactions(timestamp DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_futures_transactions_txid ON futures_transactions(transaction_id)`,

		// Create futures_account_settings table
		`CREATE TABLE IF NOT EXISTS futures_account_settings (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL UNIQUE,
			leverage INTEGER NOT NULL DEFAULT 10,
			margin_type VARCHAR(10) NOT NULL DEFAULT 'CROSSED',
			position_mode VARCHAR(10) NOT NULL DEFAULT 'ONE_WAY',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_account_settings_symbol ON futures_account_settings(symbol)`,

		// Add AI decision tracking to futures trades
		`ALTER TABLE futures_trades ADD COLUMN IF NOT EXISTS ai_decision_id INTEGER`,
		`CREATE INDEX IF NOT EXISTS idx_futures_trades_ai_decision ON futures_trades(ai_decision_id)`,

		// Add strategy tracking to futures trades (for strategy-based trades vs AI trades)
		`ALTER TABLE futures_trades ADD COLUMN IF NOT EXISTS strategy_id INTEGER`,
		`ALTER TABLE futures_trades ADD COLUMN IF NOT EXISTS strategy_name VARCHAR(100)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_trades_strategy_id ON futures_trades(strategy_id)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_trades_trade_source ON futures_trades(trade_source)`,

		// Add trading mode tracking to futures trades
		`ALTER TABLE futures_trades ADD COLUMN IF NOT EXISTS trading_mode VARCHAR(20)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_trades_mode ON futures_trades(trading_mode)`,

		// Create mode_safety_history table for tracking safety control events
		`CREATE TABLE IF NOT EXISTS mode_safety_history (
			id BIGSERIAL PRIMARY KEY,
			mode VARCHAR(20) NOT NULL,
			event_type VARCHAR(50) NOT NULL,
			trigger_reason TEXT,
			win_rate DECIMAL(5,2),
			profit_window_pct DECIMAL(10,4),
			trades_per_minute INT,
			pause_duration_mins INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mode_safety_history_mode ON mode_safety_history(mode)`,
		`CREATE INDEX IF NOT EXISTS idx_mode_safety_history_event_type ON mode_safety_history(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_mode_safety_history_created_at ON mode_safety_history(created_at DESC)`,

		// Create mode_allocation_history table for tracking capital allocation changes
		`CREATE TABLE IF NOT EXISTS mode_allocation_history (
			id BIGSERIAL PRIMARY KEY,
			mode VARCHAR(20) NOT NULL,
			allocated_percent DECIMAL(5,2) NOT NULL,
			allocated_usd DECIMAL(15,2) NOT NULL,
			used_usd DECIMAL(15,2) NOT NULL,
			available_usd DECIMAL(15,2) NOT NULL,
			current_positions INT NOT NULL,
			max_positions INT NOT NULL,
			capacity_percent DECIMAL(5,2) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mode_allocation_history_mode ON mode_allocation_history(mode)`,
		`CREATE INDEX IF NOT EXISTS idx_mode_allocation_history_created_at ON mode_allocation_history(created_at DESC)`,

		// Create mode_performance_stats table for tracking per-mode performance
		`CREATE TABLE IF NOT EXISTS mode_performance_stats (
			id BIGSERIAL PRIMARY KEY,
			mode VARCHAR(20) NOT NULL UNIQUE,
			total_trades INT NOT NULL DEFAULT 0,
			winning_trades INT NOT NULL DEFAULT 0,
			losing_trades INT NOT NULL DEFAULT 0,
			win_rate DECIMAL(5,2) DEFAULT 0,
			total_pnl_usd DECIMAL(15,2) DEFAULT 0,
			total_pnl_percent DECIMAL(10,4) DEFAULT 0,
			avg_pnl_per_trade DECIMAL(10,4) DEFAULT 0,
			max_drawdown_usd DECIMAL(15,2) DEFAULT 0,
			max_drawdown_percent DECIMAL(10,4) DEFAULT 0,
			avg_hold_seconds INT DEFAULT 0,
			last_trade_time TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mode_performance_stats_mode ON mode_performance_stats(mode)`,
		`CREATE INDEX IF NOT EXISTS idx_mode_performance_stats_updated_at ON mode_performance_stats(updated_at DESC)`,
	}

	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			log.Printf("Futures migration %d failed: %v", i+1, err)
			// Continue with other migrations
			continue
		}
		log.Printf("Futures migration %d completed", i+1)
	}

	log.Println("Futures database migrations completed")
	return nil
}
