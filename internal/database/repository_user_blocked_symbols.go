package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER BLOCKED SYMBOLS CRUD OPERATIONS
// Story 9.11: Migrate symbol blocking from file to database
// =====================================================

// UserBlockedSymbol represents a blocked trading symbol for a user
type UserBlockedSymbol struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Symbol    string     `json:"symbol"`
	Reason    string     `json:"reason"`
	BlockedAt time.Time  `json:"blocked_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // nil = permanent
	CreatedAt time.Time  `json:"created_at"`
}

// BlockSymbol blocks a symbol for a user with optional duration
// If duration is nil, the block is permanent until manually unblocked
func (r *Repository) BlockSymbol(ctx context.Context, userID, symbol, reason string, duration *time.Duration) error {
	var expiresAt *time.Time
	if duration != nil {
		t := time.Now().Add(*duration)
		expiresAt = &t
	}

	query := `
		INSERT INTO user_blocked_symbols (user_id, symbol, reason, blocked_at, expires_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, $4)
		ON CONFLICT (user_id, symbol) DO UPDATE SET
			reason = EXCLUDED.reason,
			blocked_at = CURRENT_TIMESTAMP,
			expires_at = EXCLUDED.expires_at
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, symbol, reason, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to block symbol %s for user %s: %w", symbol, userID, err)
	}

	return nil
}

// BlockSymbolForDay blocks a symbol for 24 hours (convenience method)
func (r *Repository) BlockSymbolForDay(ctx context.Context, userID, symbol, reason string) error {
	duration := 24 * time.Hour
	return r.BlockSymbol(ctx, userID, symbol, reason, &duration)
}

// UnblockSymbol removes a symbol block for a user
func (r *Repository) UnblockSymbol(ctx context.Context, userID, symbol string) error {
	query := `DELETE FROM user_blocked_symbols WHERE user_id = $1 AND symbol = $2`

	_, err := r.db.Pool.Exec(ctx, query, userID, symbol)
	if err != nil {
		return fmt.Errorf("failed to unblock symbol %s for user %s: %w", symbol, userID, err)
	}

	return nil
}

// GetBlockedSymbols returns all currently blocked symbols for a user
// Automatically filters out expired blocks
func (r *Repository) GetBlockedSymbols(ctx context.Context, userID string) ([]UserBlockedSymbol, error) {
	query := `
		SELECT id, user_id, symbol, reason, blocked_at, expires_at, created_at
		FROM user_blocked_symbols
		WHERE user_id = $1
		  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
		ORDER BY blocked_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocked symbols for user %s: %w", userID, err)
	}
	defer rows.Close()

	var blocks []UserBlockedSymbol
	for rows.Next() {
		var b UserBlockedSymbol
		if err := rows.Scan(&b.ID, &b.UserID, &b.Symbol, &b.Reason, &b.BlockedAt, &b.ExpiresAt, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan blocked symbol: %w", err)
		}
		blocks = append(blocks, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blocked symbols: %w", err)
	}

	return blocks, nil
}

// GetBlockedSymbolsList returns just the symbol names that are blocked for a user
func (r *Repository) GetBlockedSymbolsList(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT symbol
		FROM user_blocked_symbols
		WHERE user_id = $1
		  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocked symbols list for user %s: %w", userID, err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan symbol: %w", err)
		}
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// IsSymbolBlocked checks if a symbol is currently blocked for a user
// Returns (blocked bool, reason string, error)
func (r *Repository) IsSymbolBlocked(ctx context.Context, userID, symbol string) (bool, string, error) {
	query := `
		SELECT reason
		FROM user_blocked_symbols
		WHERE user_id = $1 AND symbol = $2
		  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	`

	var reason string
	err := r.db.Pool.QueryRow(ctx, query, userID, symbol).Scan(&reason)
	if err == pgx.ErrNoRows {
		return false, "", nil // Not blocked
	}
	if err != nil {
		return false, "", fmt.Errorf("failed to check if symbol %s is blocked: %w", symbol, err)
	}

	return true, reason, nil
}

// GetBlockedReason returns the reason a symbol is blocked, or empty string if not blocked
func (r *Repository) GetBlockedReason(ctx context.Context, userID, symbol string) (string, error) {
	blocked, reason, err := r.IsSymbolBlocked(ctx, userID, symbol)
	if err != nil {
		return "", err
	}
	if !blocked {
		return "", nil
	}
	return reason, nil
}

// GetBlockedSymbolInfo returns full block info for a symbol, or nil if not blocked
func (r *Repository) GetBlockedSymbolInfo(ctx context.Context, userID, symbol string) (*UserBlockedSymbol, error) {
	query := `
		SELECT id, user_id, symbol, reason, blocked_at, expires_at, created_at
		FROM user_blocked_symbols
		WHERE user_id = $1 AND symbol = $2
		  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	`

	var b UserBlockedSymbol
	err := r.db.Pool.QueryRow(ctx, query, userID, symbol).Scan(
		&b.ID, &b.UserID, &b.Symbol, &b.Reason, &b.BlockedAt, &b.ExpiresAt, &b.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil // Not blocked
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get blocked symbol info for %s: %w", symbol, err)
	}

	return &b, nil
}

// CleanExpiredBlocks removes all expired blocks from the database
// Should be called periodically (e.g., daily) to clean up
func (r *Repository) CleanExpiredBlocks(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM user_blocked_symbols
		WHERE expires_at IS NOT NULL AND expires_at <= CURRENT_TIMESTAMP
	`

	result, err := r.db.Pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to clean expired blocks: %w", err)
	}

	return result.RowsAffected(), nil
}

// BlockMultipleSymbols blocks multiple symbols at once (e.g., for auto-blocking worst performers)
func (r *Repository) BlockMultipleSymbols(ctx context.Context, userID string, symbols []string, reason string, duration *time.Duration) error {
	if len(symbols) == 0 {
		return nil
	}

	var expiresAt *time.Time
	if duration != nil {
		t := time.Now().Add(*duration)
		expiresAt = &t
	}

	// Use batch insert for efficiency
	batch := &pgx.Batch{}
	query := `
		INSERT INTO user_blocked_symbols (user_id, symbol, reason, blocked_at, expires_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, $4)
		ON CONFLICT (user_id, symbol) DO UPDATE SET
			reason = EXCLUDED.reason,
			blocked_at = CURRENT_TIMESTAMP,
			expires_at = EXCLUDED.expires_at
	`

	for _, symbol := range symbols {
		batch.Queue(query, userID, symbol, reason, expiresAt)
	}

	br := r.db.Pool.SendBatch(ctx, batch)
	defer br.Close()

	for range symbols {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to block symbols: %w", err)
		}
	}

	return nil
}

// UnblockAllSymbols removes all blocks for a user
func (r *Repository) UnblockAllSymbols(ctx context.Context, userID string) error {
	query := `DELETE FROM user_blocked_symbols WHERE user_id = $1`

	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to unblock all symbols for user %s: %w", userID, err)
	}

	return nil
}

// GetBlockCount returns the number of blocked symbols for a user
func (r *Repository) GetBlockCount(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM user_blocked_symbols
		WHERE user_id = $1
		  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get block count for user %s: %w", userID, err)
	}

	return count, nil
}
