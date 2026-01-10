package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// MigrateModeConfigsFromJSON migrates mode configurations from autopilot_settings.json to the database
// This migration:
// 1. Reads the autopilot_settings.json file
// 2. Extracts the mode_configs section
// 3. For each user in the users table, inserts the mode configs if they don't already exist
// 4. Is idempotent - can be run multiple times safely (won't overwrite existing user configs)
func (db *DB) MigrateModeConfigsFromJSON(ctx context.Context, settingsFilePath string) error {
	log.Println("[MODE-MIGRATE] Starting mode configs migration from autopilot_settings.json...")

	// Check if the settings file exists
	if _, err := os.Stat(settingsFilePath); os.IsNotExist(err) {
		log.Printf("[MODE-MIGRATE] Settings file not found at %s, skipping migration", settingsFilePath)
		return nil // Not an error - file may not exist in some environments
	}

	// Read the settings file
	fileData, err := os.ReadFile(settingsFilePath)
	if err != nil {
		return fmt.Errorf("[MODE-MIGRATE] failed to read settings file: %w", err)
	}

	// Parse the JSON
	var settings map[string]interface{}
	if err := json.Unmarshal(fileData, &settings); err != nil {
		return fmt.Errorf("[MODE-MIGRATE] failed to parse settings JSON: %w", err)
	}

	// Extract mode_configs section
	modeConfigsRaw, ok := settings["mode_configs"]
	if !ok {
		log.Println("[MODE-MIGRATE] No mode_configs section found in settings file, skipping migration")
		return nil
	}

	modeConfigs, ok := modeConfigsRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("[MODE-MIGRATE] mode_configs is not a valid object")
	}

	log.Printf("[MODE-MIGRATE] Found %d mode configurations in settings file", len(modeConfigs))

	// Get all users from the database
	usersQuery := `SELECT id, email FROM users ORDER BY created_at`
	rows, err := db.Pool.Query(ctx, usersQuery)
	if err != nil {
		return fmt.Errorf("[MODE-MIGRATE] failed to query users: %w", err)
	}
	defer rows.Close()

	var users []struct {
		ID    string
		Email string
	}
	for rows.Next() {
		var user struct {
			ID    string
			Email string
		}
		if err := rows.Scan(&user.ID, &user.Email); err != nil {
			return fmt.Errorf("[MODE-MIGRATE] failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("[MODE-MIGRATE] error iterating users: %w", err)
	}

	log.Printf("[MODE-MIGRATE] Found %d users to migrate configs for", len(users))

	// Migrate mode configs for each user
	migratedCount := 0
	skippedCount := 0

	for _, user := range users {
		for modeName, modeConfigRaw := range modeConfigs {
			// Check if user already has this mode config
			var exists bool
			checkQuery := `SELECT EXISTS(SELECT 1 FROM user_mode_configs WHERE user_id = $1 AND mode_name = $2)`
			if err := db.Pool.QueryRow(ctx, checkQuery, user.ID, modeName).Scan(&exists); err != nil {
				return fmt.Errorf("[MODE-MIGRATE] failed to check existing config for user %s mode %s: %w", user.Email, modeName, err)
			}

			if exists {
				skippedCount++
				log.Printf("[MODE-MIGRATE] Skipping %s for user %s (already exists)", modeName, user.Email)
				continue
			}

			// Convert mode config to JSON bytes
			modeConfigJSON, err := json.Marshal(modeConfigRaw)
			if err != nil {
				return fmt.Errorf("[MODE-MIGRATE] failed to marshal mode config for %s: %w", modeName, err)
			}

			// Get enabled status from the config
		// DEFAULT: ultra_fast and scalp_reentry are disabled by default (high risk)
		// Other modes (scalp, swing, position) are enabled by default
		enabled := false
		if modeName == "scalp" || modeName == "swing" || modeName == "position" {
			enabled = true // Standard trading modes enabled by default
		}
		// Override with actual config value if present
		if modeConfig, ok := modeConfigRaw.(map[string]interface{}); ok {
			if enabledVal, ok := modeConfig["enabled"].(bool); ok {
				enabled = enabledVal
			}
		}

			// Insert the mode config
			insertQuery := `
				INSERT INTO user_mode_configs (user_id, mode_name, enabled, config_json)
				VALUES ($1, $2, $3, $4)
			`
			_, err = db.Pool.Exec(ctx, insertQuery, user.ID, modeName, enabled, modeConfigJSON)
			if err != nil {
				return fmt.Errorf("[MODE-MIGRATE] failed to insert mode config for user %s mode %s: %w", user.Email, modeName, err)
			}

			migratedCount++
			log.Printf("[MODE-MIGRATE] Migrated %s for user %s (enabled: %v)", modeName, user.Email, enabled)
		}
	}

	log.Printf("[MODE-MIGRATE] Migration complete: %d configs migrated, %d configs skipped (already existed)", migratedCount, skippedCount)
	return nil
}
