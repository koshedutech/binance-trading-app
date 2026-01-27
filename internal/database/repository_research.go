// Package database provides database operations for the trading bot.
// Epic 15: Research Infrastructure - Data, Features & Backtesting
// Story 15.1: Database Schema for Historical Candles
package database

import (
	"context"
	"fmt"
	"log"
)

// RunResearchCandlesMigration creates the research_candles table and its indexes.
// This migration supports the research infrastructure for backtesting and pattern discovery.
func (db *DB) RunResearchCandlesMigration(ctx context.Context) error {
	if db.Pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	log.Println("[RESEARCH] Running research candles migration...")

	migrations := []string{
		// Create research_candles table for historical candle data
		`CREATE TABLE IF NOT EXISTS research_candles (
			id BIGSERIAL PRIMARY KEY,

			-- Candle identification
			symbol VARCHAR(20) NOT NULL,        -- Trading pair (e.g., BTCUSDT)
			timeframe VARCHAR(5) NOT NULL,      -- Candle interval (1m, 5m, 15m, 1h, 4h, 1d)
			open_time TIMESTAMPTZ NOT NULL,     -- Candle open time (UTC)
			close_time TIMESTAMPTZ NOT NULL,    -- Candle close time (UTC)

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
		)`,

		// Primary composite index for time-series queries (descending for most recent first)
		`CREATE INDEX IF NOT EXISTS idx_candles_symbol_tf_time
			ON research_candles(symbol, timeframe, open_time DESC)`,

		// Index for range queries by open_time alone (cross-symbol analysis)
		`CREATE INDEX IF NOT EXISTS idx_candles_open_time
			ON research_candles(open_time DESC)`,

		// Index for symbol-only queries (all timeframes for a symbol)
		`CREATE INDEX IF NOT EXISTS idx_candles_symbol
			ON research_candles(symbol)`,

		// Index for timeframe analysis (all symbols at a specific timeframe)
		`CREATE INDEX IF NOT EXISTS idx_candles_timeframe
			ON research_candles(timeframe)`,
	}

	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("research candles migration %d failed: %w", i, err)
		}
	}

	log.Println("[RESEARCH] Research candles migration completed successfully")
	return nil
}
