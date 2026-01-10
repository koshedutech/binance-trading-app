package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// RunUserSettingsMigrations executes all user settings related migrations (013-019)
// These migrations create tables for user-specific settings including:
// - LLM configuration (013)
// - Capital allocation (014)
// - Global circuit breaker (015)
// - Early warning system (016)
// - Ginie settings (017)
// - Spot settings (018)
// - Mode circuit breaker stats (019)
func (db *DB) RunUserSettingsMigrations(ctx context.Context) error {
	log.Println("Running User Settings database migrations (013-019)...")

	// Define the migrations to run in order
	migrationFiles := []string{
		"013_user_llm_config.sql",
		"014_user_capital_allocation.sql",
		"015_user_global_circuit_breaker.sql",
		"016_user_early_warning.sql",
		"017_user_ginie_settings.sql",
		"018_user_spot_settings.sql",
		"019_user_mode_circuit_breaker_stats.sql",
	}

	// Get the project root directory
	// Assuming we're in /internal/database, need to go up 2 levels to reach project root
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Try to find migrations directory
	migrationsDir := filepath.Join(currentDir, "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		// Try parent directory
		migrationsDir = filepath.Join(currentDir, "..", "migrations")
		if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
			// Try grandparent directory
			migrationsDir = filepath.Join(currentDir, "..", "..", "migrations")
			if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
				return fmt.Errorf("migrations directory not found: %w", err)
			}
		}
	}

	log.Printf("Using migrations directory: %s", migrationsDir)

	// Execute each migration file
	for _, filename := range migrationFiles {
		migrationPath := filepath.Join(migrationsDir, filename)

		log.Printf("Running migration: %s", filename)

		// Read the SQL file
		sqlContent, err := os.ReadFile(migrationPath)
		if err != nil {
			log.Printf("Warning: Failed to read migration file %s: %v", filename, err)
			continue
		}

		// Execute the SQL
		if _, err := db.Pool.Exec(ctx, string(sqlContent)); err != nil {
			log.Printf("Warning: Migration %s failed: %v", filename, err)
			// Continue with other migrations (table may already exist)
			continue
		}

		log.Printf("Successfully executed migration: %s", filename)
	}

	log.Println("User Settings database migrations (013-019) completed")
	return nil
}
