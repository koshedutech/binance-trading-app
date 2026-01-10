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
