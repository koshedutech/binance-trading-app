package database

import (
	"context"
	"log"
)

// RunGlobalCircuitBreakerMigration runs database migration for user_global_circuit_breaker table
func (db *DB) RunGlobalCircuitBreakerMigration(ctx context.Context) error {
	log.Println("Running Global Circuit Breaker database migration...")

	// Check if table exists and has the old schema (user_id as primary key, no id column)
	var hasIDColumn bool
	err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'user_global_circuit_breaker'
			AND column_name = 'id'
		)
	`).Scan(&hasIDColumn)
	if err != nil {
		log.Printf("Failed to check for id column: %v", err)
	}

	var tableExists bool
	err = db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'user_global_circuit_breaker'
		)
	`).Scan(&tableExists)
	if err != nil {
		log.Printf("Failed to check if table exists: %v", err)
	}

	if tableExists && !hasIDColumn {
		log.Println("Found old table schema - migrating to new schema with id column...")
		// Old table exists without id column - need to migrate
		migrationSteps := []string{
			// Drop the old primary key constraint
			`ALTER TABLE user_global_circuit_breaker DROP CONSTRAINT IF EXISTS user_global_circuit_breaker_pkey`,
			// Add the new id column with serial
			`ALTER TABLE user_global_circuit_breaker ADD COLUMN IF NOT EXISTS id SERIAL`,
			// Add id as primary key
			`ALTER TABLE user_global_circuit_breaker ADD PRIMARY KEY (id)`,
			// Add unique constraint on user_id
			`ALTER TABLE user_global_circuit_breaker ADD CONSTRAINT user_global_circuit_breaker_user_id_key UNIQUE (user_id)`,
			// Add missing columns
			`ALTER TABLE user_global_circuit_breaker ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true`,
			`ALTER TABLE user_global_circuit_breaker ADD COLUMN IF NOT EXISTS max_trades_per_minute INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE user_global_circuit_breaker ADD COLUMN IF NOT EXISTS max_daily_trades INTEGER NOT NULL DEFAULT 0`,
		}

		for i, step := range migrationSteps {
			if _, err := db.Pool.Exec(ctx, step); err != nil {
				log.Printf("Schema migration step %d failed: %v", i+1, err)
				// Continue with other steps
			}
		}
	}

	migrations := []string{
		// Create user_global_circuit_breaker table with all columns (for new installations)
		`CREATE TABLE IF NOT EXISTS user_global_circuit_breaker (
			id SERIAL PRIMARY KEY,
			user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			enabled BOOLEAN NOT NULL DEFAULT true,
			max_loss_per_hour DECIMAL(20,8) NOT NULL DEFAULT 100,
			max_daily_loss DECIMAL(20,8) NOT NULL DEFAULT 300,
			max_consecutive_losses INTEGER NOT NULL DEFAULT 3,
			cooldown_minutes INTEGER NOT NULL DEFAULT 30,
			max_trades_per_minute INTEGER NOT NULL DEFAULT 0,
			max_daily_trades INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,

		// Add missing columns for existing tables (in case migration above failed)
		`ALTER TABLE user_global_circuit_breaker ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true`,
		`ALTER TABLE user_global_circuit_breaker ADD COLUMN IF NOT EXISTS max_trades_per_minute INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE user_global_circuit_breaker ADD COLUMN IF NOT EXISTS max_daily_trades INTEGER NOT NULL DEFAULT 0`,

		// Create index for faster lookups
		`CREATE INDEX IF NOT EXISTS idx_global_circuit_breaker_user ON user_global_circuit_breaker(user_id)`,

		// Create trigger for updated_at
		`DROP TRIGGER IF EXISTS update_user_global_circuit_breaker_updated_at ON user_global_circuit_breaker`,
		`CREATE TRIGGER update_user_global_circuit_breaker_updated_at BEFORE UPDATE ON user_global_circuit_breaker
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
	}

	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			log.Printf("Global Circuit Breaker migration %d failed: %v", i+1, err)
			// Continue with other migrations (some may already exist)
			continue
		}
	}

	log.Println("Global Circuit Breaker database migration completed successfully")
	return nil
}
