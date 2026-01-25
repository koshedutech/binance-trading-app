package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER TRADING STATE CRUD OPERATIONS
// Story 14.14: Trading Controller - ON/OFF toggle for Chain Trading System
// =====================================================

// TradingStateData represents the trading state stored in the database
// This is a data struct used for database operations
// The autopilot.TradingState is the primary type used in the application
type TradingStateData struct {
	Enabled     bool      `json:"enabled"`
	EntryActive bool      `json:"entry_active"`
	ExitActive  bool      `json:"exit_active"`
	LastChanged time.Time `json:"last_changed"`
	ChangedBy   string    `json:"changed_by"`
	Reason      string    `json:"reason,omitempty"`
}

// UserTradingState represents the database row for user trading state
type UserTradingState struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Enabled     bool      `json:"enabled"`
	EntryActive bool      `json:"entry_active"`
	ExitActive  bool      `json:"exit_active"`
	LastChanged time.Time `json:"last_changed"`
	ChangedBy   string    `json:"changed_by"`
	Reason      string    `json:"reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetUserTradingState retrieves the trading state for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserTradingState(ctx context.Context, userID string) (*TradingStateData, error) {
	query := `
		SELECT enabled, entry_active, exit_active, last_changed, changed_by, reason
		FROM user_trading_state
		WHERE user_id = $1
	`

	var state TradingStateData
	var reason *string
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&state.Enabled,
		&state.EntryActive,
		&state.ExitActive,
		&state.LastChanged,
		&state.ChangedBy,
		&reason,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user trading state: %w", err)
	}

	if reason != nil {
		state.Reason = *reason
	}

	return &state, nil
}

// SaveUserTradingState saves or updates the trading state for a user (UPSERT)
func (r *Repository) SaveUserTradingState(ctx context.Context, userID string, state *TradingStateData) error {
	query := `
		INSERT INTO user_trading_state (
			user_id, enabled, entry_active, exit_active, last_changed, changed_by, reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			entry_active = EXCLUDED.entry_active,
			exit_active = EXCLUDED.exit_active,
			last_changed = EXCLUDED.last_changed,
			changed_by = EXCLUDED.changed_by,
			reason = EXCLUDED.reason,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		userID,
		state.Enabled,
		state.EntryActive,
		state.ExitActive,
		state.LastChanged,
		state.ChangedBy,
		state.Reason,
	)
	if err != nil {
		return fmt.Errorf("failed to save user trading state: %w", err)
	}

	return nil
}

// UpdateTradingEnabled updates only the enabled state for a user
func (r *Repository) UpdateTradingEnabled(ctx context.Context, userID string, enabled bool, changedBy, reason string) error {
	query := `
		INSERT INTO user_trading_state (
			user_id, enabled, entry_active, exit_active, last_changed, changed_by, reason
		) VALUES ($1, $2, $2, true, CURRENT_TIMESTAMP, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			entry_active = EXCLUDED.entry_active,
			last_changed = EXCLUDED.last_changed,
			changed_by = EXCLUDED.changed_by,
			reason = EXCLUDED.reason,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, enabled, changedBy, reason)
	if err != nil {
		return fmt.Errorf("failed to update trading enabled: %w", err)
	}

	return nil
}

// GetUsersWithTradingEnabled returns a list of user IDs that have trading enabled
// Used for system-wide queries about active traders
func (r *Repository) GetUsersWithTradingEnabled(ctx context.Context) ([]string, error) {
	query := `
		SELECT user_id
		FROM user_trading_state
		WHERE enabled = true
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users with trading enabled: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user IDs: %w", err)
	}

	return userIDs, nil
}

// DeleteUserTradingState deletes the trading state for a user
func (r *Repository) DeleteUserTradingState(ctx context.Context, userID string) error {
	query := `DELETE FROM user_trading_state WHERE user_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user trading state: %w", err)
	}
	return nil
}

// InitializeUserTradingState creates default trading state for a new user
// Trading is enabled by default
func (r *Repository) InitializeUserTradingState(ctx context.Context, userID string) error {
	state := &TradingStateData{
		Enabled:     true,
		EntryActive: true,
		ExitActive:  true,
		LastChanged: time.Now(),
		ChangedBy:   "system",
		Reason:      "user initialization",
	}

	if err := r.SaveUserTradingState(ctx, userID, state); err != nil {
		return fmt.Errorf("failed to initialize user trading state: %w", err)
	}

	return nil
}
