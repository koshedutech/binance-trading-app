package database

import (
	"context"
	"fmt"
	"log"
)

// MigrateUserModeConfigs creates the user_mode_configs table for storing per-user mode configurations
func (db *DB) MigrateUserModeConfigs(ctx context.Context) error {
	log.Println("Running user mode configs migration...")

	migrations := []string{
		// Create user_mode_configs table
		`CREATE TABLE IF NOT EXISTS user_mode_configs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			mode_name VARCHAR(50) NOT NULL CHECK (mode_name IN ('ultra_fast', 'scalp', 'swing', 'position', 'scalp_reentry')),
			enabled BOOLEAN NOT NULL DEFAULT true,
			config_json JSONB NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, mode_name)
		)`,

		// Create indexes for efficient lookups
		`CREATE INDEX IF NOT EXISTS idx_user_mode_configs_user_id ON user_mode_configs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_mode_configs_mode_name ON user_mode_configs(mode_name)`,
		`CREATE INDEX IF NOT EXISTS idx_user_mode_configs_enabled ON user_mode_configs(user_id, enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_user_mode_configs_config_json ON user_mode_configs USING GIN (config_json)`,

		// Create trigger to update updated_at timestamp
		`CREATE OR REPLACE FUNCTION update_user_mode_configs_updated_at()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql`,

		`DROP TRIGGER IF EXISTS trigger_user_mode_configs_updated_at ON user_mode_configs`,

		`CREATE TRIGGER trigger_user_mode_configs_updated_at
		BEFORE UPDATE ON user_mode_configs
		FOR EACH ROW
		EXECUTE FUNCTION update_user_mode_configs_updated_at()`,
	}

	for i, migration := range migrations {
		_, err := db.Pool.Exec(ctx, migration)
		if err != nil {
			return fmt.Errorf("failed to run user mode configs migration %d: %w", i+1, err)
		}
	}

	log.Println("User mode configs migration completed successfully")
	return nil
}

// MigrateRemoveScalpReentryMode removes scalp_reentry from valid modes (Story 9.4 Phase 2)
// scalp_reentry is an optimization feature, not a trading mode
func (db *DB) MigrateRemoveScalpReentryMode(ctx context.Context) error {
	log.Println("Running scalp_reentry mode removal migration (Story 9.4)...")

	migrations := []string{
		// Step 1: Mark any existing scalp_reentry mode configs as deprecated
		`UPDATE user_mode_configs
		SET config_json = jsonb_set(config_json, '{deprecated}', 'true')
		WHERE mode_name = 'scalp_reentry'`,

		// Step 2: Drop the old CHECK constraint
		`ALTER TABLE user_mode_configs
		DROP CONSTRAINT IF EXISTS user_mode_configs_mode_name_check`,

		// Step 3: Add new CHECK constraint without scalp_reentry
		// Note: scalp_reentry is NOT a mode - it's an optimization for scalp mode
		`ALTER TABLE user_mode_configs
		ADD CONSTRAINT user_mode_configs_mode_name_check
		CHECK (mode_name IN ('ultra_fast', 'scalp', 'swing', 'position'))`,
	}

	for i, migration := range migrations {
		_, err := db.Pool.Exec(ctx, migration)
		if err != nil {
			// Log but continue if constraint doesn't exist or update fails on empty table
			log.Printf("[MIGRATION] Warning on step %d: %v (continuing...)", i+1, err)
		}
	}

	// Optionally delete deprecated scalp_reentry configs
	// (keeping them with deprecated flag for now - can delete later)
	// _, _ = db.Pool.Exec(ctx, `DELETE FROM user_mode_configs WHERE mode_name = 'scalp_reentry'`)

	log.Println("Scalp_reentry mode removal migration completed")
	return nil
}
