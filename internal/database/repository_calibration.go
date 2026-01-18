// Package database provides the CalibrationRepository for calibration data persistence.
// Epic 11: Position Decision Engine - Story 11.25: Calibration Data Storage
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CalibrationDataRecord represents a calibration data record in the database.
// This maps to the calibration_data table.
type CalibrationDataRecord struct {
	ID            int64                  `json:"id"`
	UserID        string                 `json:"user_id"`
	Strategy      string                 `json:"strategy"`
	IndicatorHash string                 `json:"indicator_hash"`
	IndicatorList []string               `json:"indicator_list"`
	Bucket0_24    *CalibrationBucketData `json:"bucket_0_24"`
	Bucket25_49   *CalibrationBucketData `json:"bucket_25_49"`
	Bucket50_74   *CalibrationBucketData `json:"bucket_50_74"`
	Bucket75_100  *CalibrationBucketData `json:"bucket_75_100"`
	Metadata      *CalibrationMetadata   `json:"metadata"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// CalibrationBucketData represents the data for a single score bucket.
type CalibrationBucketData struct {
	TotalTrades     int64   `json:"total_trades"`
	WinningTrades   int64   `json:"winning_trades"`
	WinRate         float64 `json:"win_rate"`
	AvgPnLPercent   float64 `json:"avg_pnl_percent"`
	TotalPnLPercent float64 `json:"total_pnl_percent"`
	LastUpdated     string  `json:"last_updated,omitempty"`
}

// CalibrationMetadata contains metadata about the calibration.
type CalibrationMetadata struct {
	LastUpdated            string `json:"last_updated"`
	FirstTradeDate         string `json:"first_trade_date,omitempty"`
	TotalCalibrationTrades int64  `json:"total_calibration_trades"`
}

// CalibrationHistoryRecord represents an archived calibration record.
type CalibrationHistoryRecord struct {
	ID            int64                  `json:"id"`
	OriginalID    int64                  `json:"original_id"`
	UserID        string                 `json:"user_id"`
	Strategy      string                 `json:"strategy"`
	IndicatorHash string                 `json:"indicator_hash"`
	IndicatorList []string               `json:"indicator_list"`
	Bucket0_24    *CalibrationBucketData `json:"bucket_0_24"`
	Bucket25_49   *CalibrationBucketData `json:"bucket_25_49"`
	Bucket50_74   *CalibrationBucketData `json:"bucket_50_74"`
	Bucket75_100  *CalibrationBucketData `json:"bucket_75_100"`
	Metadata      *CalibrationMetadata   `json:"metadata"`
	ArchivedAt    time.Time              `json:"archived_at"`
	ArchiveReason string                 `json:"archive_reason"`
}

// UpsertCalibrationData inserts or updates calibration data.
// Uses PostgreSQL UPSERT (ON CONFLICT) for atomic operation.
func (db *DB) UpsertCalibrationData(ctx context.Context, data *CalibrationDataRecord) error {
	if db.Pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	// Marshal JSONB fields
	indicatorListJSON, err := json.Marshal(data.IndicatorList)
	if err != nil {
		return fmt.Errorf("failed to marshal indicator_list: %w", err)
	}

	bucket024JSON, err := json.Marshal(data.Bucket0_24)
	if err != nil {
		return fmt.Errorf("failed to marshal bucket_0_24: %w", err)
	}

	bucket2549JSON, err := json.Marshal(data.Bucket25_49)
	if err != nil {
		return fmt.Errorf("failed to marshal bucket_25_49: %w", err)
	}

	bucket5074JSON, err := json.Marshal(data.Bucket50_74)
	if err != nil {
		return fmt.Errorf("failed to marshal bucket_50_74: %w", err)
	}

	bucket75100JSON, err := json.Marshal(data.Bucket75_100)
	if err != nil {
		return fmt.Errorf("failed to marshal bucket_75_100: %w", err)
	}

	metadataJSON, err := json.Marshal(data.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO calibration_data (
			user_id, strategy, indicator_hash, indicator_list,
			bucket_0_24, bucket_25_49, bucket_50_74, bucket_75_100,
			metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW()
		)
		ON CONFLICT (user_id, strategy, indicator_hash) DO UPDATE SET
			indicator_list = EXCLUDED.indicator_list,
			bucket_0_24 = EXCLUDED.bucket_0_24,
			bucket_25_49 = EXCLUDED.bucket_25_49,
			bucket_50_74 = EXCLUDED.bucket_50_74,
			bucket_75_100 = EXCLUDED.bucket_75_100,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id, created_at, updated_at`

	err = db.Pool.QueryRow(ctx, query,
		data.UserID,
		data.Strategy,
		data.IndicatorHash,
		indicatorListJSON,
		bucket024JSON,
		bucket2549JSON,
		bucket5074JSON,
		bucket75100JSON,
		metadataJSON,
	).Scan(&data.ID, &data.CreatedAt, &data.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert calibration data: %w", err)
	}

	return nil
}

// GetCalibrationData retrieves calibration data for a user/strategy/indicator combination.
func (db *DB) GetCalibrationData(ctx context.Context, userID, strategy, indicatorHash string) (*CalibrationDataRecord, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT id, user_id, strategy, indicator_hash, indicator_list,
			bucket_0_24, bucket_25_49, bucket_50_74, bucket_75_100,
			metadata, created_at, updated_at
		FROM calibration_data
		WHERE user_id = $1 AND strategy = $2 AND indicator_hash = $3`

	var record CalibrationDataRecord
	var indicatorListJSON, bucket024JSON, bucket2549JSON, bucket5074JSON, bucket75100JSON, metadataJSON []byte

	err := db.Pool.QueryRow(ctx, query, userID, strategy, indicatorHash).Scan(
		&record.ID,
		&record.UserID,
		&record.Strategy,
		&record.IndicatorHash,
		&indicatorListJSON,
		&bucket024JSON,
		&bucket2549JSON,
		&bucket5074JSON,
		&bucket75100JSON,
		&metadataJSON,
		&record.CreatedAt,
		&record.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get calibration data: %w", err)
	}

	// Unmarshal JSONB fields
	if err := json.Unmarshal(indicatorListJSON, &record.IndicatorList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal indicator_list: %w", err)
	}

	record.Bucket0_24 = &CalibrationBucketData{}
	if err := json.Unmarshal(bucket024JSON, record.Bucket0_24); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bucket_0_24: %w", err)
	}

	record.Bucket25_49 = &CalibrationBucketData{}
	if err := json.Unmarshal(bucket2549JSON, record.Bucket25_49); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bucket_25_49: %w", err)
	}

	record.Bucket50_74 = &CalibrationBucketData{}
	if err := json.Unmarshal(bucket5074JSON, record.Bucket50_74); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bucket_50_74: %w", err)
	}

	record.Bucket75_100 = &CalibrationBucketData{}
	if err := json.Unmarshal(bucket75100JSON, record.Bucket75_100); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bucket_75_100: %w", err)
	}

	record.Metadata = &CalibrationMetadata{}
	if err := json.Unmarshal(metadataJSON, record.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &record, nil
}

// GetAllCalibrationDataForUser retrieves all calibration data for a user.
func (db *DB) GetAllCalibrationDataForUser(ctx context.Context, userID string) ([]*CalibrationDataRecord, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT id, user_id, strategy, indicator_hash, indicator_list,
			bucket_0_24, bucket_25_49, bucket_50_74, bucket_75_100,
			metadata, created_at, updated_at
		FROM calibration_data
		WHERE user_id = $1
		ORDER BY updated_at DESC`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get calibration data for user: %w", err)
	}
	defer rows.Close()

	var records []*CalibrationDataRecord
	for rows.Next() {
		record, err := scanCalibrationRow(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calibration rows: %w", err)
	}

	return records, nil
}

// GetCalibrationHistory retrieves archived calibration data for a user/strategy.
func (db *DB) GetCalibrationHistory(ctx context.Context, userID, strategy string) ([]*CalibrationHistoryRecord, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT id, original_id, user_id, strategy, indicator_hash, indicator_list,
			bucket_0_24, bucket_25_49, bucket_50_74, bucket_75_100,
			metadata, archived_at, archive_reason
		FROM calibration_data_history
		WHERE user_id = $1 AND strategy = $2
		ORDER BY archived_at DESC`

	rows, err := db.Pool.Query(ctx, query, userID, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to get calibration history: %w", err)
	}
	defer rows.Close()

	var records []*CalibrationHistoryRecord
	for rows.Next() {
		var record CalibrationHistoryRecord
		var indicatorListJSON, bucket024JSON, bucket2549JSON, bucket5074JSON, bucket75100JSON, metadataJSON []byte

		err := rows.Scan(
			&record.ID,
			&record.OriginalID,
			&record.UserID,
			&record.Strategy,
			&record.IndicatorHash,
			&indicatorListJSON,
			&bucket024JSON,
			&bucket2549JSON,
			&bucket5074JSON,
			&bucket75100JSON,
			&metadataJSON,
			&record.ArchivedAt,
			&record.ArchiveReason,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan calibration history row: %w", err)
		}

		// Unmarshal JSONB fields
		if err := json.Unmarshal(indicatorListJSON, &record.IndicatorList); err != nil {
			return nil, fmt.Errorf("failed to unmarshal indicator_list: %w", err)
		}

		record.Bucket0_24 = &CalibrationBucketData{}
		if err := json.Unmarshal(bucket024JSON, record.Bucket0_24); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bucket_0_24: %w", err)
		}

		record.Bucket25_49 = &CalibrationBucketData{}
		if err := json.Unmarshal(bucket2549JSON, record.Bucket25_49); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bucket_25_49: %w", err)
		}

		record.Bucket50_74 = &CalibrationBucketData{}
		if err := json.Unmarshal(bucket5074JSON, record.Bucket50_74); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bucket_50_74: %w", err)
		}

		record.Bucket75_100 = &CalibrationBucketData{}
		if err := json.Unmarshal(bucket75100JSON, record.Bucket75_100); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bucket_75_100: %w", err)
		}

		record.Metadata = &CalibrationMetadata{}
		if err := json.Unmarshal(metadataJSON, record.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		records = append(records, &record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calibration history rows: %w", err)
	}

	return records, nil
}

// ArchiveCalibrationData archives calibration data to history table.
// This is called when indicators change or user explicitly resets.
func (db *DB) ArchiveCalibrationData(ctx context.Context, userID, strategy, indicatorHash, reason string) error {
	if db.Pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	// Use a transaction to ensure atomicity
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// First, copy to history table
	copyQuery := `
		INSERT INTO calibration_data_history (
			original_id, user_id, strategy, indicator_hash, indicator_list,
			bucket_0_24, bucket_25_49, bucket_50_74, bucket_75_100,
			metadata, archive_reason
		)
		SELECT id, user_id, strategy, indicator_hash, indicator_list,
			bucket_0_24, bucket_25_49, bucket_50_74, bucket_75_100,
			metadata, $4
		FROM calibration_data
		WHERE user_id = $1 AND strategy = $2 AND indicator_hash = $3`

	result, err := tx.Exec(ctx, copyQuery, userID, strategy, indicatorHash, reason)
	if err != nil {
		return fmt.Errorf("failed to copy calibration to history: %w", err)
	}

	// Check if any row was copied
	if result.RowsAffected() == 0 {
		// No record to archive
		return nil
	}

	// Then delete from main table
	deleteQuery := `
		DELETE FROM calibration_data
		WHERE user_id = $1 AND strategy = $2 AND indicator_hash = $3`

	_, err = tx.Exec(ctx, deleteQuery, userID, strategy, indicatorHash)
	if err != nil {
		return fmt.Errorf("failed to delete archived calibration: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit archive transaction: %w", err)
	}

	return nil
}

// DeleteCalibrationData deletes calibration data without archiving.
func (db *DB) DeleteCalibrationData(ctx context.Context, userID, strategy, indicatorHash string) error {
	if db.Pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	query := `
		DELETE FROM calibration_data
		WHERE user_id = $1 AND strategy = $2 AND indicator_hash = $3`

	_, err := db.Pool.Exec(ctx, query, userID, strategy, indicatorHash)
	if err != nil {
		return fmt.Errorf("failed to delete calibration data: %w", err)
	}

	return nil
}

// GetCalibrationDataByStrategy retrieves all calibration data for a user/strategy combination.
// Returns multiple records if user has used different indicator configurations.
func (db *DB) GetCalibrationDataByStrategy(ctx context.Context, userID, strategy string) ([]*CalibrationDataRecord, error) {
	if db.Pool == nil {
		return nil, fmt.Errorf("database pool is nil")
	}

	query := `
		SELECT id, user_id, strategy, indicator_hash, indicator_list,
			bucket_0_24, bucket_25_49, bucket_50_74, bucket_75_100,
			metadata, created_at, updated_at
		FROM calibration_data
		WHERE user_id = $1 AND strategy = $2
		ORDER BY updated_at DESC`

	rows, err := db.Pool.Query(ctx, query, userID, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to get calibration data by strategy: %w", err)
	}
	defer rows.Close()

	var records []*CalibrationDataRecord
	for rows.Next() {
		record, err := scanCalibrationRow(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating calibration rows: %w", err)
	}

	return records, nil
}

// scanCalibrationRow is a helper to scan a calibration row.
func scanCalibrationRow(rows pgx.Rows) (*CalibrationDataRecord, error) {
	var record CalibrationDataRecord
	var indicatorListJSON, bucket024JSON, bucket2549JSON, bucket5074JSON, bucket75100JSON, metadataJSON []byte

	err := rows.Scan(
		&record.ID,
		&record.UserID,
		&record.Strategy,
		&record.IndicatorHash,
		&indicatorListJSON,
		&bucket024JSON,
		&bucket2549JSON,
		&bucket5074JSON,
		&bucket75100JSON,
		&metadataJSON,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan calibration row: %w", err)
	}

	// Unmarshal JSONB fields
	if err := json.Unmarshal(indicatorListJSON, &record.IndicatorList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal indicator_list: %w", err)
	}

	record.Bucket0_24 = &CalibrationBucketData{}
	if err := json.Unmarshal(bucket024JSON, record.Bucket0_24); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bucket_0_24: %w", err)
	}

	record.Bucket25_49 = &CalibrationBucketData{}
	if err := json.Unmarshal(bucket2549JSON, record.Bucket25_49); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bucket_25_49: %w", err)
	}

	record.Bucket50_74 = &CalibrationBucketData{}
	if err := json.Unmarshal(bucket5074JSON, record.Bucket50_74); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bucket_50_74: %w", err)
	}

	record.Bucket75_100 = &CalibrationBucketData{}
	if err := json.Unmarshal(bucket75100JSON, record.Bucket75_100); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bucket_75_100: %w", err)
	}

	record.Metadata = &CalibrationMetadata{}
	if err := json.Unmarshal(metadataJSON, record.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &record, nil
}

// CalibrationRepository provides an interface for calibration data operations.
// This can be used for dependency injection and testing.
type CalibrationRepository interface {
	UpsertCalibrationData(ctx context.Context, data *CalibrationDataRecord) error
	GetCalibrationData(ctx context.Context, userID, strategy, indicatorHash string) (*CalibrationDataRecord, error)
	GetAllCalibrationDataForUser(ctx context.Context, userID string) ([]*CalibrationDataRecord, error)
	GetCalibrationDataByStrategy(ctx context.Context, userID, strategy string) ([]*CalibrationDataRecord, error)
	GetCalibrationHistory(ctx context.Context, userID, strategy string) ([]*CalibrationHistoryRecord, error)
	ArchiveCalibrationData(ctx context.Context, userID, strategy, indicatorHash, reason string) error
	DeleteCalibrationData(ctx context.Context, userID, strategy, indicatorHash string) error
}

// Ensure DB implements CalibrationRepository
var _ CalibrationRepository = (*DB)(nil)

// RunCalibrationMigration runs the calibration data migration.
func (db *DB) RunCalibrationMigration(ctx context.Context) error {
	if db.Pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	migrations := []string{
		// Create calibration_data table
		`CREATE TABLE IF NOT EXISTS calibration_data (
			id SERIAL PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			strategy VARCHAR(50) NOT NULL,
			indicator_hash VARCHAR(16) NOT NULL,
			indicator_list JSONB NOT NULL DEFAULT '[]',
			bucket_0_24 JSONB NOT NULL DEFAULT '{}',
			bucket_25_49 JSONB NOT NULL DEFAULT '{}',
			bucket_50_74 JSONB NOT NULL DEFAULT '{}',
			bucket_75_100 JSONB NOT NULL DEFAULT '{}',
			metadata JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			CONSTRAINT unique_user_strategy_indicator UNIQUE(user_id, strategy, indicator_hash)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_calibration_user_id ON calibration_data(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calibration_strategy ON calibration_data(strategy)`,
		`CREATE INDEX IF NOT EXISTS idx_calibration_updated ON calibration_data(updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_calibration_user_strategy ON calibration_data(user_id, strategy)`,

		// Create calibration_data_history table
		`CREATE TABLE IF NOT EXISTS calibration_data_history (
			id SERIAL PRIMARY KEY,
			original_id INTEGER NOT NULL,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			strategy VARCHAR(50) NOT NULL,
			indicator_hash VARCHAR(16) NOT NULL,
			indicator_list JSONB NOT NULL DEFAULT '[]',
			bucket_0_24 JSONB NOT NULL DEFAULT '{}',
			bucket_25_49 JSONB NOT NULL DEFAULT '{}',
			bucket_50_74 JSONB NOT NULL DEFAULT '{}',
			bucket_75_100 JSONB NOT NULL DEFAULT '{}',
			metadata JSONB NOT NULL DEFAULT '{}',
			archived_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			archive_reason VARCHAR(50) DEFAULT 'INDICATOR_CHANGE'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_calibration_history_user ON calibration_data_history(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_calibration_history_strategy ON calibration_data_history(user_id, strategy)`,
		`CREATE INDEX IF NOT EXISTS idx_calibration_history_archived ON calibration_data_history(archived_at DESC)`,

		// Create trigger for updated_at
		`DROP TRIGGER IF EXISTS update_calibration_data_updated_at ON calibration_data`,
		`CREATE TRIGGER update_calibration_data_updated_at
			BEFORE UPDATE ON calibration_data
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
	}

	for _, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("calibration migration failed: %w", err)
		}
	}

	return nil
}
