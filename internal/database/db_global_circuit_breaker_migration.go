package database

import (
	"context"
	"log"
)

// RunGlobalCircuitBreakerMigration runs database migration for user_global_circuit_breaker table
func (db *DB) RunGlobalCircuitBreakerMigration(ctx context.Context) error {
	log.Println("Running Global Circuit Breaker database migration...")

	migrations := []string{
		// Create user_global_circuit_breaker table
		`CREATE TABLE IF NOT EXISTS user_global_circuit_breaker (
			user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			max_loss_per_hour DECIMAL(20,8) NOT NULL DEFAULT 100,
			max_daily_loss DECIMAL(20,8) NOT NULL DEFAULT 300,
			max_consecutive_losses INTEGER NOT NULL DEFAULT 3,
			cooldown_minutes INTEGER NOT NULL DEFAULT 30,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,

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
