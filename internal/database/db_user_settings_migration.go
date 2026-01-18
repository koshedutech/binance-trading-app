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

// MigrateEarlyWarningExtendedFields adds new columns to user_early_warning table (Story 9.4 Phase 4)
// These extended fields support more sophisticated early warning behavior
func (db *DB) MigrateEarlyWarningExtendedFields(ctx context.Context) error {
	log.Println("Running user_early_warning extended fields migration (Story 9.4 Phase 4)...")

	migrations := []string{
		// Add 7 new columns to user_early_warning table
		`ALTER TABLE user_early_warning
		ADD COLUMN IF NOT EXISTS tighten_sl_on_warning BOOLEAN DEFAULT true`,

		`ALTER TABLE user_early_warning
		ADD COLUMN IF NOT EXISTS min_confidence NUMERIC(5,4) DEFAULT 0.7`,

		`ALTER TABLE user_early_warning
		ADD COLUMN IF NOT EXISTS max_llm_calls_per_pos INTEGER DEFAULT 3`,

		`ALTER TABLE user_early_warning
		ADD COLUMN IF NOT EXISTS close_min_hold_mins INTEGER DEFAULT 15`,

		`ALTER TABLE user_early_warning
		ADD COLUMN IF NOT EXISTS close_min_confidence NUMERIC(5,4) DEFAULT 0.85`,

		`ALTER TABLE user_early_warning
		ADD COLUMN IF NOT EXISTS close_require_consecutive INTEGER DEFAULT 2`,

		`ALTER TABLE user_early_warning
		ADD COLUMN IF NOT EXISTS close_sl_proximity_pct INTEGER DEFAULT 50`,

		// Add comments for documentation
		`COMMENT ON COLUMN user_early_warning.tighten_sl_on_warning IS 'Tighten SL if warning detected (default: true)'`,
		`COMMENT ON COLUMN user_early_warning.min_confidence IS 'Min LLM confidence to act on warning (0.0-1.0, default: 0.7)'`,
		`COMMENT ON COLUMN user_early_warning.max_llm_calls_per_pos IS 'Max LLM calls per position per cycle (default: 3)'`,
		`COMMENT ON COLUMN user_early_warning.close_min_hold_mins IS 'Min hold time before close_now allowed (default: 15)'`,
		`COMMENT ON COLUMN user_early_warning.close_min_confidence IS 'Higher confidence for close_now action (default: 0.85)'`,
		`COMMENT ON COLUMN user_early_warning.close_require_consecutive IS 'Require X consecutive warnings before close (default: 2)'`,
		`COMMENT ON COLUMN user_early_warning.close_sl_proximity_pct IS 'Only close if within X% of SL distance (default: 50)'`,
	}

	for i, migration := range migrations {
		_, err := db.Pool.Exec(ctx, migration)
		if err != nil {
			log.Printf("[MIGRATION] Warning on step %d: %v (continuing...)", i+1, err)
		}
	}

	log.Println("user_early_warning extended fields migration completed")
	return nil
}

// RunUserSafetySettingsMigration creates the user_safety_settings table (Migration 023)
// Story 9.4: Per-user safety controls per trading mode
func (db *DB) RunUserSafetySettingsMigration(ctx context.Context) error {
	log.Println("Running User Safety Settings migration (023)...")

	// Get the project root directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Try to find migrations directory
	migrationsDir := filepath.Join(currentDir, "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = filepath.Join(currentDir, "..", "migrations")
		if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
			migrationsDir = filepath.Join(currentDir, "..", "..", "migrations")
			if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
				return fmt.Errorf("migrations directory not found: %w", err)
			}
		}
	}

	migrationPath := filepath.Join(migrationsDir, "023_user_safety_settings.sql")

	// Read the SQL file
	sqlContent, err := os.ReadFile(migrationPath)
	if err != nil {
		log.Printf("[MIGRATION] Warning: Failed to read migration file 023_user_safety_settings.sql: %v", err)
		// Try inline migration
		return db.runUserSafetySettingsInlineMigration(ctx)
	}

	// Execute the SQL
	if _, err := db.Pool.Exec(ctx, string(sqlContent)); err != nil {
		log.Printf("[MIGRATION] Migration 023 file failed: %v, trying inline migration", err)
		return db.runUserSafetySettingsInlineMigration(ctx)
	}

	log.Println("User Safety Settings migration (023) completed successfully")
	return nil
}

// runUserSafetySettingsInlineMigration runs the migration using inline SQL
func (db *DB) runUserSafetySettingsInlineMigration(ctx context.Context) error {
	log.Println("Running inline User Safety Settings migration...")

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS user_safety_settings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			mode VARCHAR(20) NOT NULL,
			max_trades_per_minute INTEGER NOT NULL DEFAULT 5,
			max_trades_per_hour INTEGER NOT NULL DEFAULT 20,
			max_trades_per_day INTEGER NOT NULL DEFAULT 50,
			enable_profit_monitor BOOLEAN NOT NULL DEFAULT true,
			profit_window_minutes INTEGER NOT NULL DEFAULT 10,
			max_loss_percent_in_window DECIMAL(5,2) NOT NULL DEFAULT -1.5,
			pause_cooldown_minutes INTEGER NOT NULL DEFAULT 30,
			enable_win_rate_monitor BOOLEAN NOT NULL DEFAULT true,
			win_rate_sample_size INTEGER NOT NULL DEFAULT 15,
			min_win_rate_threshold DECIMAL(5,2) NOT NULL DEFAULT 50,
			win_rate_cooldown_minutes INTEGER NOT NULL DEFAULT 60,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, mode)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_safety_settings_user_id ON user_safety_settings(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_safety_settings_user_mode ON user_safety_settings(user_id, mode)`,
	}

	for i, migration := range migrations {
		_, err := db.Pool.Exec(ctx, migration)
		if err != nil {
			log.Printf("[MIGRATION] Warning on step %d: %v (continuing...)", i+1, err)
		}
	}

	log.Println("Inline User Safety Settings migration completed")
	return nil
}

// RunUserGlobalTradingMigrations executes all user global trading related migrations (026, 032, 033)
// These migrations create and populate the user_global_trading table including:
// - Main table with risk level, allocation, profit reinvestment (026)
// - Timezone and timezone_offset columns (032)
// - Initialize records for existing users (033)
func (db *DB) RunUserGlobalTradingMigrations(ctx context.Context) error {
	log.Println("Running User Global Trading database migrations (026, 032, 033)...")

	// Define the migrations to run in order
	migrationFiles := []string{
		"026_user_global_trading.sql",
		"032_user_global_trading_timezone.sql",
		"033_init_user_global_trading.sql",
	}

	// Get the project root directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Try to find migrations directory
	migrationsDir := filepath.Join(currentDir, "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = filepath.Join(currentDir, "..", "migrations")
		if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
			migrationsDir = filepath.Join(currentDir, "..", "..", "migrations")
			if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
				log.Println("[MIGRATION] migrations directory not found, running inline migration")
				return db.runUserGlobalTradingInlineMigration(ctx)
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

	log.Println("User Global Trading database migrations (026, 032, 033) completed")
	return nil
}

// runUserGlobalTradingInlineMigration runs the migration using inline SQL
func (db *DB) runUserGlobalTradingInlineMigration(ctx context.Context) error {
	log.Println("Running inline User Global Trading migration...")

	migrations := []string{
		// Create main table (026)
		`CREATE TABLE IF NOT EXISTS user_global_trading (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			risk_level VARCHAR(20) NOT NULL DEFAULT 'moderate',
			max_usd_allocation DECIMAL(20,8) NOT NULL DEFAULT 2500.00000000,
			profit_reinvest_percent DECIMAL(5,2) NOT NULL DEFAULT 50.00,
			profit_reinvest_risk_level VARCHAR(20) NOT NULL DEFAULT 'aggressive',
			timezone VARCHAR(50) DEFAULT 'Asia/Phnom_Penh',
			timezone_offset VARCHAR(10) DEFAULT '+07:00',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT unique_user_global_trading UNIQUE(user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_global_trading_user_id ON user_global_trading(user_id)`,

		// Add timezone columns if they don't exist (032)
		`ALTER TABLE user_global_trading ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'Asia/Phnom_Penh'`,
		`ALTER TABLE user_global_trading ADD COLUMN IF NOT EXISTS timezone_offset VARCHAR(10) DEFAULT '+07:00'`,

		// Initialize records for existing users (033)
		`INSERT INTO user_global_trading (user_id, risk_level, max_usd_allocation, profit_reinvest_percent, profit_reinvest_risk_level, timezone, timezone_offset)
		SELECT u.id, 'moderate', 2500.00000000, 50.00, 'aggressive', 'Asia/Phnom_Penh', '+07:00'
		FROM users u
		WHERE NOT EXISTS (SELECT 1 FROM user_global_trading ugt WHERE ugt.user_id = u.id)`,
	}

	for i, migration := range migrations {
		_, err := db.Pool.Exec(ctx, migration)
		if err != nil {
			log.Printf("[MIGRATION] Warning on step %d: %v (continuing...)", i+1, err)
		}
	}

	log.Println("Inline User Global Trading migration completed")
	return nil
}

// RunOrderChainMigrations executes order chain tracking migrations (034-036)
// These migrations create tables for:
// - Position states (034) - temporary position tracking
// - Order modification events (035) - track SL/TP modifications
// - Order chain events (036) - master order chain table and event store
func (db *DB) RunOrderChainMigrations(ctx context.Context) error {
	log.Println("Running Order Chain database migrations (034-036)...")

	// Define the migrations to run in order
	migrationFiles := []string{
		"034_position_states.sql",
		"035_order_modification_events.sql",
		"036_order_chain_events.sql",
	}

	// Get the project root directory
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

	log.Println("Order Chain database migrations (034-036) completed")
	return nil
}
