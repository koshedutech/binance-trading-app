package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateLicenseTable creates the licenses table
func (r *Repository) CreateLicenseTable(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS licenses (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		key VARCHAR(20) UNIQUE NOT NULL,
		type VARCHAR(20) NOT NULL,
		customer_email VARCHAR(255),
		customer_name VARCHAR(255),
		max_symbols INTEGER DEFAULT 10,
		features JSONB DEFAULT '[]',
		is_active BOOLEAN DEFAULT true,
		activated_at TIMESTAMP,
		expires_at TIMESTAMP,
		last_used_at TIMESTAMP,
		last_used_ip VARCHAR(45),
		notes TEXT,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_licenses_key ON licenses(key);
	CREATE INDEX IF NOT EXISTS idx_licenses_email ON licenses(customer_email);
	CREATE INDEX IF NOT EXISTS idx_licenses_type ON licenses(type);
	CREATE INDEX IF NOT EXISTS idx_licenses_active ON licenses(is_active);

	CREATE TABLE IF NOT EXISTS license_usage_logs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		license_id UUID REFERENCES licenses(id) ON DELETE CASCADE,
		ip VARCHAR(45),
		user_agent TEXT,
		success BOOLEAN,
		message TEXT,
		created_at TIMESTAMP DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_license_logs_license ON license_usage_logs(license_id);
	CREATE INDEX IF NOT EXISTS idx_license_logs_created ON license_usage_logs(created_at);
	`

	_, err := r.db.Pool.Exec(ctx, query)
	return err
}

// CreateLicense creates a new license
func (r *Repository) CreateLicense(ctx context.Context, license *License) error {
	if license.ID == "" {
		license.ID = uuid.New().String()
	}
	license.CreatedAt = time.Now()
	license.UpdatedAt = time.Now()

	query := `
	INSERT INTO licenses (id, key, type, customer_email, customer_name, max_symbols, features, is_active, expires_at, notes, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		license.ID,
		license.Key,
		license.Type,
		license.CustomerEmail,
		license.CustomerName,
		license.MaxSymbols,
		license.Features,
		license.IsActive,
		license.ExpiresAt,
		license.Notes,
		license.CreatedAt,
		license.UpdatedAt,
	)

	return err
}

// GetLicenseByKey retrieves a license by its key
func (r *Repository) GetLicenseByKey(ctx context.Context, key string) (*License, error) {
	query := `
	SELECT id, key, type, COALESCE(customer_email, ''), COALESCE(customer_name, ''), max_symbols,
	       COALESCE(features::text, '[]'), is_active, activated_at, expires_at, last_used_at,
	       COALESCE(last_used_ip, ''), COALESCE(notes, ''), created_at, updated_at
	FROM licenses
	WHERE key = $1
	`

	var license License

	err := r.db.Pool.QueryRow(ctx, query, key).Scan(
		&license.ID,
		&license.Key,
		&license.Type,
		&license.CustomerEmail,
		&license.CustomerName,
		&license.MaxSymbols,
		&license.Features,
		&license.IsActive,
		&license.ActivatedAt,
		&license.ExpiresAt,
		&license.LastUsedAt,
		&license.LastUsedIP,
		&license.Notes,
		&license.CreatedAt,
		&license.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get license by key: %w", err)
	}

	return &license, nil
}

// GetLicenseByID retrieves a license by ID
func (r *Repository) GetLicenseByID(ctx context.Context, id string) (*License, error) {
	query := `
	SELECT id, key, type, COALESCE(customer_email, ''), COALESCE(customer_name, ''), max_symbols,
	       COALESCE(features::text, '[]'), is_active, activated_at, expires_at, last_used_at,
	       COALESCE(last_used_ip, ''), COALESCE(notes, ''), created_at, updated_at
	FROM licenses
	WHERE id = $1
	`

	var license License

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&license.ID,
		&license.Key,
		&license.Type,
		&license.CustomerEmail,
		&license.CustomerName,
		&license.MaxSymbols,
		&license.Features,
		&license.IsActive,
		&license.ActivatedAt,
		&license.ExpiresAt,
		&license.LastUsedAt,
		&license.LastUsedIP,
		&license.Notes,
		&license.CreatedAt,
		&license.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get license by id: %w", err)
	}

	return &license, nil
}

// ListLicenses retrieves all licenses with optional filtering
func (r *Repository) ListLicenses(ctx context.Context, licenseType string, activeOnly bool, limit, offset int) ([]License, int, error) {
	// Build query with filters
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argNum := 1

	if licenseType != "" {
		whereClause += fmt.Sprintf(" AND type = $%d", argNum)
		args = append(args, licenseType)
		argNum++
	}

	if activeOnly {
		whereClause += " AND is_active = true"
	}

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM licenses %s", whereClause)
	var total int
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count licenses: %w", err)
	}

	// Data query
	query := fmt.Sprintf(`
	SELECT id, key, type, COALESCE(customer_email, ''), COALESCE(customer_name, ''), max_symbols,
	       COALESCE(features::text, '[]'), is_active, activated_at, expires_at, last_used_at,
	       COALESCE(last_used_ip, ''), COALESCE(notes, ''), created_at, updated_at
	FROM licenses
	%s
	ORDER BY created_at DESC
	LIMIT $%d OFFSET $%d
	`, whereClause, argNum, argNum+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list licenses: %w", err)
	}
	defer rows.Close()

	var licenses []License
	for rows.Next() {
		var license License

		err := rows.Scan(
			&license.ID,
			&license.Key,
			&license.Type,
			&license.CustomerEmail,
			&license.CustomerName,
			&license.MaxSymbols,
			&license.Features,
			&license.IsActive,
			&license.ActivatedAt,
			&license.ExpiresAt,
			&license.LastUsedAt,
			&license.LastUsedIP,
			&license.Notes,
			&license.CreatedAt,
			&license.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan license: %w", err)
		}

		licenses = append(licenses, license)
	}

	return licenses, total, nil
}

// UpdateLicense updates a license
func (r *Repository) UpdateLicense(ctx context.Context, license *License) error {
	license.UpdatedAt = time.Now()

	query := `
	UPDATE licenses
	SET type = $2, customer_email = $3, customer_name = $4, max_symbols = $5,
	    features = $6, is_active = $7, expires_at = $8, notes = $9, updated_at = $10
	WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query,
		license.ID,
		license.Type,
		license.CustomerEmail,
		license.CustomerName,
		license.MaxSymbols,
		license.Features,
		license.IsActive,
		license.ExpiresAt,
		license.Notes,
		license.UpdatedAt,
	)

	return err
}

// ActivateLicense marks a license as activated
func (r *Repository) ActivateLicense(ctx context.Context, id string, ip string) error {
	now := time.Now()
	query := `
	UPDATE licenses
	SET activated_at = $2, last_used_at = $2, last_used_ip = $3, updated_at = $2
	WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, id, now, ip)
	return err
}

// UpdateLicenseUsage updates the last used timestamp and IP
func (r *Repository) UpdateLicenseUsage(ctx context.Context, id string, ip string) error {
	now := time.Now()
	query := `
	UPDATE licenses
	SET last_used_at = $2, last_used_ip = $3, updated_at = $2
	WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, id, now, ip)
	return err
}

// DeactivateLicense deactivates a license
func (r *Repository) DeactivateLicense(ctx context.Context, id string) error {
	query := `UPDATE licenses SET is_active = false, updated_at = NOW() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	return err
}

// DeleteLicense deletes a license
func (r *Repository) DeleteLicense(ctx context.Context, id string) error {
	query := `DELETE FROM licenses WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	return err
}

// LogLicenseUsage logs a license validation attempt
func (r *Repository) LogLicenseUsage(ctx context.Context, log *LicenseUsageLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	log.CreatedAt = time.Now()

	query := `
	INSERT INTO license_usage_logs (id, license_id, ip, user_agent, success, message, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		log.ID,
		log.LicenseID,
		log.IP,
		log.UserAgent,
		log.Success,
		log.Message,
		log.CreatedAt,
	)

	return err
}

// GetLicenseStats returns license statistics
func (r *Repository) GetLicenseStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total counts by type
	query := `
	SELECT type, COUNT(*) as count,
	       SUM(CASE WHEN is_active THEN 1 ELSE 0 END) as active_count
	FROM licenses
	GROUP BY type
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get license stats by type: %w", err)
	}
	defer rows.Close()

	byType := make(map[string]map[string]int)
	for rows.Next() {
		var licenseType string
		var count, activeCount int
		if err := rows.Scan(&licenseType, &count, &activeCount); err != nil {
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}
		byType[licenseType] = map[string]int{
			"total":  count,
			"active": activeCount,
		}
	}
	stats["by_type"] = byType

	// Total active licenses
	var totalActive int
	err = r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM licenses WHERE is_active = true").Scan(&totalActive)
	if err != nil {
		return nil, fmt.Errorf("failed to get total active: %w", err)
	}
	stats["total_active"] = totalActive

	// Total licenses
	var total int
	err = r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM licenses").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to get total: %w", err)
	}
	stats["total"] = total

	// Recent activations (last 7 days)
	var recentActivations int
	err = r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM licenses WHERE activated_at > NOW() - INTERVAL '7 days'",
	).Scan(&recentActivations)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activations: %w", err)
	}
	stats["recent_activations"] = recentActivations

	return stats, nil
}

// Helper function to convert features slice to JSON string
func FeaturesToJSON(features []string) string {
	data, _ := json.Marshal(features)
	return string(data)
}

// Helper function to convert JSON string to features slice
func JSONToFeatures(jsonStr string) []string {
	var features []string
	json.Unmarshal([]byte(jsonStr), &features)
	return features
}
