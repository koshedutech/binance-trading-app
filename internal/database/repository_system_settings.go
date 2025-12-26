package database

import (
	"context"
	"time"
)

// GetSystemSetting retrieves a single system setting by key
func (db *DB) GetSystemSetting(ctx context.Context, key string) (*SystemSetting, error) {
	var setting SystemSetting
	err := db.Pool.QueryRow(ctx,
		`SELECT key, value, is_encrypted, description, updated_at, updated_by
		 FROM system_settings WHERE key = $1`, key).Scan(
		&setting.Key, &setting.Value, &setting.IsEncrypted,
		&setting.Description, &setting.UpdatedAt, &setting.UpdatedBy)
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// GetAllSystemSettings retrieves all system settings
func (db *DB) GetAllSystemSettings(ctx context.Context) ([]SystemSetting, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT key, value, is_encrypted, description, updated_at, updated_by
		 FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []SystemSetting
	for rows.Next() {
		var s SystemSetting
		if err := rows.Scan(&s.Key, &s.Value, &s.IsEncrypted, &s.Description, &s.UpdatedAt, &s.UpdatedBy); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, nil
}

// UpsertSystemSetting creates or updates a system setting
func (db *DB) UpsertSystemSetting(ctx context.Context, setting *SystemSetting) error {
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO system_settings (key, value, is_encrypted, description, updated_at, updated_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			is_encrypted = EXCLUDED.is_encrypted,
			description = EXCLUDED.description,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by`,
		setting.Key, setting.Value, setting.IsEncrypted, setting.Description, time.Now(), setting.UpdatedBy)
	return err
}

// DeleteSystemSetting deletes a system setting by key
func (db *DB) DeleteSystemSetting(ctx context.Context, key string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM system_settings WHERE key = $1`, key)
	return err
}

// GetSMTPSettings retrieves all SMTP-related settings
func (db *DB) GetSMTPSettings(ctx context.Context) (map[string]string, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT key, value FROM system_settings WHERE key LIKE 'smtp_%' ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}
	return settings, nil
}
