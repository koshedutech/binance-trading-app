package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the PostgreSQL connection pool
type DB struct {
	Pool *pgxpool.Pool
}

// Config holds database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// NewDB creates a new database connection
func NewDB(cfg Config) (*DB, error) {
	// Build connection string
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	// Parse connection string
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	// Create connection pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Printf("Successfully connected to PostgreSQL database: %s", cfg.Database)

	return &DB{Pool: pool}, nil
}

// Close closes the database connection
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		log.Println("Database connection closed")
	}
}

// RunMigrations executes database migrations
func (db *DB) RunMigrations(ctx context.Context) error {
	log.Println("Running database migrations...")

	migrations := []string{
		// Create trades table
		`CREATE TABLE IF NOT EXISTS trades (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL,
			side VARCHAR(4) NOT NULL,
			entry_price DECIMAL(20, 8) NOT NULL,
			exit_price DECIMAL(20, 8),
			quantity DECIMAL(20, 8) NOT NULL,
			entry_time TIMESTAMP NOT NULL,
			exit_time TIMESTAMP,
			stop_loss DECIMAL(20, 8),
			take_profit DECIMAL(20, 8),
			pnl DECIMAL(20, 8),
			pnl_percent DECIMAL(10, 4),
			strategy_name VARCHAR(100),
			status VARCHAR(20) NOT NULL DEFAULT 'OPEN',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_status ON trades(status)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_entry_time ON trades(entry_time)`,

		// Create orders table
		`CREATE TABLE IF NOT EXISTS orders (
			id BIGINT PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL,
			order_type VARCHAR(20) NOT NULL,
			side VARCHAR(4) NOT NULL,
			price DECIMAL(20, 8),
			quantity DECIMAL(20, 8) NOT NULL,
			executed_qty DECIMAL(20, 8) DEFAULT 0,
			status VARCHAR(20) NOT NULL,
			time_in_force VARCHAR(10),
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			filled_at TIMESTAMP,
			trade_id INTEGER REFERENCES trades(id) ON DELETE SET NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_symbol ON orders(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at)`,

		// Create signals table
		`CREATE TABLE IF NOT EXISTS signals (
			id SERIAL PRIMARY KEY,
			strategy_name VARCHAR(100) NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			signal_type VARCHAR(10) NOT NULL,
			entry_price DECIMAL(20, 8) NOT NULL,
			stop_loss DECIMAL(20, 8),
			take_profit DECIMAL(20, 8),
			quantity DECIMAL(20, 8),
			reason TEXT,
			timestamp TIMESTAMP NOT NULL,
			executed BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_signals_symbol ON signals(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_signals_timestamp ON signals(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_signals_executed ON signals(executed)`,

		// Add conditions_met column to signals table if not exists
		`ALTER TABLE signals ADD COLUMN IF NOT EXISTS conditions_met JSONB`,

		// Create strategy_configs table
		`CREATE TABLE IF NOT EXISTS strategy_configs (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			symbol VARCHAR(20) NOT NULL,
			timeframe VARCHAR(10) NOT NULL,
			indicator_type VARCHAR(50) NOT NULL,
			autopilot BOOLEAN DEFAULT FALSE,
			enabled BOOLEAN DEFAULT TRUE,
			position_size DECIMAL(10, 4) DEFAULT 0.10,
			stop_loss_percent DECIMAL(10, 4) DEFAULT 2.0,
			take_profit_percent DECIMAL(10, 4) DEFAULT 5.0,
			config_params JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_strategy_configs_symbol ON strategy_configs(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_strategy_configs_enabled ON strategy_configs(enabled)`,

		// Create pending_signals table for manual confirmation
		`CREATE TABLE IF NOT EXISTS pending_signals (
			id SERIAL PRIMARY KEY,
			strategy_name VARCHAR(100) NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			signal_type VARCHAR(10) NOT NULL,
			entry_price DECIMAL(20, 8) NOT NULL,
			current_price DECIMAL(20, 8) NOT NULL,
			stop_loss DECIMAL(20, 8),
			take_profit DECIMAL(20, 8),
			quantity DECIMAL(20, 8),
			reason TEXT,
			conditions_met JSONB NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			status VARCHAR(20) DEFAULT 'PENDING',
			confirmed_at TIMESTAMP,
			rejected_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_signals_status ON pending_signals(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_signals_timestamp ON pending_signals(timestamp)`,

		// Add archived columns to pending_signals for soft delete functionality
		`ALTER TABLE pending_signals ADD COLUMN IF NOT EXISTS archived BOOLEAN DEFAULT FALSE`,
		`ALTER TABLE pending_signals ADD COLUMN IF NOT EXISTS archived_at TIMESTAMP`,
		`CREATE INDEX IF NOT EXISTS idx_pending_signals_archived ON pending_signals(archived)`,

		// Create positions table (snapshots for history)
		`CREATE TABLE IF NOT EXISTS position_snapshots (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL,
			entry_price DECIMAL(20, 8) NOT NULL,
			current_price DECIMAL(20, 8) NOT NULL,
			quantity DECIMAL(20, 8) NOT NULL,
			pnl DECIMAL(20, 8) NOT NULL,
			pnl_percent DECIMAL(10, 4) NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_position_snapshots_symbol ON position_snapshots(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_position_snapshots_timestamp ON position_snapshots(timestamp)`,

		// Create screener results table
		`CREATE TABLE IF NOT EXISTS screener_results (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL,
			last_price DECIMAL(20, 8) NOT NULL,
			price_change_percent DECIMAL(10, 4),
			volume DECIMAL(30, 8),
			quote_volume DECIMAL(30, 8),
			high_24h DECIMAL(20, 8),
			low_24h DECIMAL(20, 8),
			signals TEXT[],
			timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_screener_results_timestamp ON screener_results(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_screener_results_symbol ON screener_results(symbol)`,

		// Create system events table
		`CREATE TABLE IF NOT EXISTS system_events (
			id SERIAL PRIMARY KEY,
			event_type VARCHAR(50) NOT NULL,
			source VARCHAR(100),
			message TEXT,
			data JSONB,
			timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_system_events_type ON system_events(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_system_events_timestamp ON system_events(timestamp)`,

		// Create updated_at trigger function
		`CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ language 'plpgsql'`,

		// Create triggers for updated_at
		`DROP TRIGGER IF EXISTS update_trades_updated_at ON trades`,
		`CREATE TRIGGER update_trades_updated_at BEFORE UPDATE ON trades
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,

		`DROP TRIGGER IF EXISTS update_orders_updated_at ON orders`,
		`CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON orders
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,

		`DROP TRIGGER IF EXISTS update_strategy_configs_updated_at ON strategy_configs`,
		`CREATE TRIGGER update_strategy_configs_updated_at BEFORE UPDATE ON strategy_configs
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,

		// Create watchlist table for favorites
		`CREATE TABLE IF NOT EXISTS watchlist (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL UNIQUE,
			notes TEXT,
			added_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_watchlist_symbol ON watchlist(symbol)`,

		// Create scanner results table for historical tracking (optional)
		`CREATE TABLE IF NOT EXISTS scanner_results (
			id SERIAL PRIMARY KEY,
			scan_id VARCHAR(50) NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			strategy_name VARCHAR(100) NOT NULL,
			current_price DECIMAL(20, 8) NOT NULL,
			target_price DECIMAL(20, 8),
			distance_percent DECIMAL(10, 4),
			readiness_score DECIMAL(5, 2),
			trend_direction VARCHAR(20),
			conditions_met INTEGER,
			total_conditions INTEGER,
			conditions_data JSONB,
			time_prediction JSONB,
			timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_scanner_results_scan_id ON scanner_results(scan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_scanner_results_symbol ON scanner_results(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_scanner_results_readiness ON scanner_results(readiness_score DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_scanner_results_timestamp ON scanner_results(timestamp DESC)`,

		// Create backtest_results table
		`CREATE TABLE IF NOT EXISTS backtest_results (
			id BIGSERIAL PRIMARY KEY,
			strategy_config_id BIGINT NOT NULL REFERENCES strategy_configs(id) ON DELETE CASCADE,
			symbol VARCHAR(20) NOT NULL,
			interval VARCHAR(10) NOT NULL,
			start_date TIMESTAMP NOT NULL,
			end_date TIMESTAMP NOT NULL,
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
			avg_trade_duration_minutes INT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_results_strategy ON backtest_results(strategy_config_id)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_results_symbol ON backtest_results(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_results_dates ON backtest_results(start_date, end_date)`,

		// Create backtest_trades table
		`CREATE TABLE IF NOT EXISTS backtest_trades (
			id BIGSERIAL PRIMARY KEY,
			backtest_result_id BIGINT NOT NULL REFERENCES backtest_results(id) ON DELETE CASCADE,
			entry_time TIMESTAMP NOT NULL,
			entry_price DECIMAL(20, 8) NOT NULL,
			entry_reason TEXT,
			exit_time TIMESTAMP NOT NULL,
			exit_price DECIMAL(20, 8) NOT NULL,
			exit_reason TEXT,
			quantity DECIMAL(20, 8) NOT NULL,
			side VARCHAR(10) NOT NULL,
			pnl DECIMAL(20, 8) NOT NULL,
			pnl_percent DECIMAL(10, 4) NOT NULL,
			fees DECIMAL(20, 8) NOT NULL DEFAULT 0,
			duration_minutes INT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_trades_result ON backtest_trades(backtest_result_id)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_trades_entry_time ON backtest_trades(entry_time)`,
	}

	// Execute migrations
	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// HealthCheck performs a database health check
func (db *DB) HealthCheck(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}
