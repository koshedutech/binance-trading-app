package database

import (
	"context"
	"log"
)

// RunSymbolRequirementsMigration creates the symbol_requirements table
// This table stores Binance exchange requirements for each trading symbol
// to enable pre-order validation and prevent precision errors
func (db *DB) RunSymbolRequirementsMigration(ctx context.Context) error {
	log.Println("Running symbol requirements migration...")

	migrations := []string{
		// Main symbol requirements table
		`CREATE TABLE IF NOT EXISTS symbol_requirements (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) UNIQUE NOT NULL,

			-- Precision settings (derived from filters)
			price_precision INT NOT NULL DEFAULT 4,
			quantity_precision INT NOT NULL DEFAULT 0,

			-- From PRICE_FILTER
			tick_size DECIMAL(20, 10) NOT NULL DEFAULT 0.0001,
			min_price DECIMAL(20, 8) DEFAULT 0,
			max_price DECIMAL(20, 8) DEFAULT 0,

			-- From LOT_SIZE filter
			step_size DECIMAL(20, 10) NOT NULL DEFAULT 1,
			min_qty DECIMAL(20, 8) NOT NULL DEFAULT 1,
			max_qty DECIMAL(20, 8) NOT NULL DEFAULT 10000000,

			-- From MIN_NOTIONAL filter
			min_notional DECIMAL(20, 8) DEFAULT 5,

			-- From MARKET_LOT_SIZE filter (for market orders)
			market_min_qty DECIMAL(20, 8) DEFAULT 0,
			market_max_qty DECIMAL(20, 8) DEFAULT 0,
			market_step_size DECIMAL(20, 10) DEFAULT 0,

			-- Symbol metadata
			base_asset VARCHAR(20),
			quote_asset VARCHAR(20),
			margin_asset VARCHAR(20),
			contract_type VARCHAR(20),
			status VARCHAR(20) NOT NULL DEFAULT 'TRADING',

			-- Tracking
			last_synced_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Indexes for fast lookups
		`CREATE INDEX IF NOT EXISTS idx_symbol_requirements_symbol ON symbol_requirements(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_symbol_requirements_status ON symbol_requirements(status)`,
		`CREATE INDEX IF NOT EXISTS idx_symbol_requirements_last_synced ON symbol_requirements(last_synced_at)`,
	}

	for _, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			log.Printf("Symbol requirements migration error: %v", err)
			return err
		}
	}

	log.Println("Symbol requirements migration completed successfully")
	return nil
}
