package database

import (
	"context"
	"log"
)

// RunScanSourceMigrations runs scan source settings database migrations
func (db *DB) RunScanSourceMigrations(ctx context.Context) error {
	log.Println("Running Scan Source database migrations...")

	migrations := []string{
		// Create user_scan_source_settings table for per-user coin scan configuration
		`CREATE TABLE IF NOT EXISTS user_scan_source_settings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

			-- Coin limit
			max_coins INT DEFAULT 50,

			-- Source toggles
			use_saved_coins BOOLEAN DEFAULT false,
			saved_coins TEXT[] DEFAULT '{}',
			use_llm_list BOOLEAN DEFAULT true,
			use_market_movers BOOLEAN DEFAULT true,

			-- Market mover filters
			mover_gainers BOOLEAN DEFAULT true,
			mover_losers BOOLEAN DEFAULT true,
			mover_volume BOOLEAN DEFAULT true,
			mover_volatility BOOLEAN DEFAULT true,
			mover_new_listings BOOLEAN DEFAULT false,

			-- Per-filter limits
			gainers_limit INT DEFAULT 10,
			losers_limit INT DEFAULT 10,
			volume_limit INT DEFAULT 15,
			volatility_limit INT DEFAULT 10,
			new_listings_limit INT DEFAULT 5,

			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

			UNIQUE(user_id)
		)`,

		// Create index for efficient user lookups
		`CREATE INDEX IF NOT EXISTS idx_user_scan_source_settings_user ON user_scan_source_settings(user_id)`,

		// Add table comment
		`COMMENT ON TABLE user_scan_source_settings IS 'Per-user coin scan source configuration for Ginie autopilot'`,

		// Create trigger for updated_at
		`DROP TRIGGER IF EXISTS update_user_scan_source_settings_updated_at ON user_scan_source_settings`,
		`CREATE TRIGGER update_user_scan_source_settings_updated_at BEFORE UPDATE ON user_scan_source_settings
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
	}

	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			log.Printf("Scan source migration %d failed: %v", i+1, err)
			// Continue with other migrations (some may already exist)
			continue
		}
		log.Printf("Scan source migration %d completed", i+1)
	}

	log.Println("Scan Source database migrations completed")
	return nil
}
