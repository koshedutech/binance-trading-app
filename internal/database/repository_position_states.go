package database

import (
	"context"
	"fmt"
	"time"

	"binance-trading-bot/internal/orders"

	"github.com/jackc/pgx/v5"
)

// CreatePositionState inserts a new position state record
func (db *DB) CreatePositionState(ctx context.Context, position *orders.PositionState) error {
	if db.Pool == nil {
		return nil // No database configured
	}

	query := `
		INSERT INTO position_states (
			user_id, chain_id, symbol, entry_order_id, entry_client_order_id,
			entry_side, entry_price, entry_quantity, entry_value, entry_fees,
			entry_filled_at, status, remaining_quantity, realized_pnl,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
		ON CONFLICT (user_id, chain_id) DO UPDATE SET
			entry_order_id = EXCLUDED.entry_order_id,
			entry_client_order_id = EXCLUDED.entry_client_order_id,
			entry_price = EXCLUDED.entry_price,
			entry_quantity = EXCLUDED.entry_quantity,
			entry_value = EXCLUDED.entry_value,
			entry_fees = EXCLUDED.entry_fees,
			entry_filled_at = EXCLUDED.entry_filled_at,
			status = EXCLUDED.status,
			remaining_quantity = EXCLUDED.remaining_quantity,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at`

	now := time.Now()
	if position.CreatedAt.IsZero() {
		position.CreatedAt = now
	}
	if position.UpdatedAt.IsZero() {
		position.UpdatedAt = now
	}

	err := db.Pool.QueryRow(ctx, query,
		position.UserID,
		position.ChainID,
		position.Symbol,
		position.EntryOrderID,
		position.EntryClientOrderID,
		position.EntrySide,
		position.EntryPrice,
		position.EntryQuantity,
		position.EntryValue,
		position.EntryFees,
		position.EntryFilledAt,
		position.Status,
		position.RemainingQuantity,
		position.RealizedPnL,
		position.CreatedAt,
		position.UpdatedAt,
	).Scan(&position.ID, &position.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create position state: %w", err)
	}

	return nil
}

// UpdatePositionState updates an existing position state record
func (db *DB) UpdatePositionState(ctx context.Context, position *orders.PositionState) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE position_states SET
			status = $2,
			remaining_quantity = $3,
			realized_pnl = $4,
			updated_at = $5,
			closed_at = $6
		WHERE id = $1`

	position.UpdatedAt = time.Now()

	_, err := db.Pool.Exec(ctx, query,
		position.ID,
		position.Status,
		position.RemainingQuantity,
		position.RealizedPnL,
		position.UpdatedAt,
		position.ClosedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update position state: %w", err)
	}

	return nil
}

// GetPositionByChainID retrieves a position state by chain ID
func (db *DB) GetPositionByChainID(ctx context.Context, userID int64, chainID string) (*orders.PositionState, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, user_id, chain_id, symbol, entry_order_id, entry_client_order_id,
			entry_side, entry_price, entry_quantity, entry_value, entry_fees,
			entry_filled_at, status, remaining_quantity, realized_pnl,
			created_at, updated_at, closed_at
		FROM position_states
		WHERE user_id = $1 AND chain_id = $2`

	position := &orders.PositionState{}
	err := db.Pool.QueryRow(ctx, query, userID, chainID).Scan(
		&position.ID,
		&position.UserID,
		&position.ChainID,
		&position.Symbol,
		&position.EntryOrderID,
		&position.EntryClientOrderID,
		&position.EntrySide,
		&position.EntryPrice,
		&position.EntryQuantity,
		&position.EntryValue,
		&position.EntryFees,
		&position.EntryFilledAt,
		&position.Status,
		&position.RemainingQuantity,
		&position.RealizedPnL,
		&position.CreatedAt,
		&position.UpdatedAt,
		&position.ClosedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get position by chain ID: %w", err)
	}

	return position, nil
}

// GetPositionsByUserID retrieves all positions for a user, optionally filtered by status
func (db *DB) GetPositionsByUserID(ctx context.Context, userID int64, status string) ([]*orders.PositionState, error) {
	if db.Pool == nil {
		return nil, nil
	}

	var query string
	var args []interface{}

	if status != "" {
		query = `
			SELECT id, user_id, chain_id, symbol, entry_order_id, entry_client_order_id,
				entry_side, entry_price, entry_quantity, entry_value, entry_fees,
				entry_filled_at, status, remaining_quantity, realized_pnl,
				created_at, updated_at, closed_at
			FROM position_states
			WHERE user_id = $1 AND status = $2
			ORDER BY created_at DESC`
		args = []interface{}{userID, status}
	} else {
		query = `
			SELECT id, user_id, chain_id, symbol, entry_order_id, entry_client_order_id,
				entry_side, entry_price, entry_quantity, entry_value, entry_fees,
				entry_filled_at, status, remaining_quantity, realized_pnl,
				created_at, updated_at, closed_at
			FROM position_states
			WHERE user_id = $1
			ORDER BY created_at DESC`
		args = []interface{}{userID}
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions by user ID: %w", err)
	}
	defer rows.Close()

	var positions []*orders.PositionState
	for rows.Next() {
		position := &orders.PositionState{}
		err := rows.Scan(
			&position.ID,
			&position.UserID,
			&position.ChainID,
			&position.Symbol,
			&position.EntryOrderID,
			&position.EntryClientOrderID,
			&position.EntrySide,
			&position.EntryPrice,
			&position.EntryQuantity,
			&position.EntryValue,
			&position.EntryFees,
			&position.EntryFilledAt,
			&position.Status,
			&position.RemainingQuantity,
			&position.RealizedPnL,
			&position.CreatedAt,
			&position.UpdatedAt,
			&position.ClosedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position row: %w", err)
		}
		positions = append(positions, position)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating position rows: %w", err)
	}

	return positions, nil
}

// GetPositionBySymbol retrieves a position by symbol and status
func (db *DB) GetPositionBySymbol(ctx context.Context, userID int64, symbol string, status string) (*orders.PositionState, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, user_id, chain_id, symbol, entry_order_id, entry_client_order_id,
			entry_side, entry_price, entry_quantity, entry_value, entry_fees,
			entry_filled_at, status, remaining_quantity, realized_pnl,
			created_at, updated_at, closed_at
		FROM position_states
		WHERE user_id = $1 AND symbol = $2 AND status = $3
		ORDER BY created_at DESC
		LIMIT 1`

	position := &orders.PositionState{}
	err := db.Pool.QueryRow(ctx, query, userID, symbol, status).Scan(
		&position.ID,
		&position.UserID,
		&position.ChainID,
		&position.Symbol,
		&position.EntryOrderID,
		&position.EntryClientOrderID,
		&position.EntrySide,
		&position.EntryPrice,
		&position.EntryQuantity,
		&position.EntryValue,
		&position.EntryFees,
		&position.EntryFilledAt,
		&position.Status,
		&position.RemainingQuantity,
		&position.RealizedPnL,
		&position.CreatedAt,
		&position.UpdatedAt,
		&position.ClosedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get position by symbol: %w", err)
	}

	return position, nil
}

// GetPositionByEntryOrderID retrieves a position by entry order ID
func (db *DB) GetPositionByEntryOrderID(ctx context.Context, entryOrderID int64) (*orders.PositionState, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, user_id, chain_id, symbol, entry_order_id, entry_client_order_id,
			entry_side, entry_price, entry_quantity, entry_value, entry_fees,
			entry_filled_at, status, remaining_quantity, realized_pnl,
			created_at, updated_at, closed_at
		FROM position_states
		WHERE entry_order_id = $1`

	position := &orders.PositionState{}
	err := db.Pool.QueryRow(ctx, query, entryOrderID).Scan(
		&position.ID,
		&position.UserID,
		&position.ChainID,
		&position.Symbol,
		&position.EntryOrderID,
		&position.EntryClientOrderID,
		&position.EntrySide,
		&position.EntryPrice,
		&position.EntryQuantity,
		&position.EntryValue,
		&position.EntryFees,
		&position.EntryFilledAt,
		&position.Status,
		&position.RemainingQuantity,
		&position.RealizedPnL,
		&position.CreatedAt,
		&position.UpdatedAt,
		&position.ClosedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get position by entry order ID: %w", err)
	}

	return position, nil
}

// GetRecentPositionStates retrieves recent position states for a user
func (db *DB) GetRecentPositionStates(ctx context.Context, userID int64, limit int) ([]*orders.PositionState, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, user_id, chain_id, symbol, entry_order_id, entry_client_order_id,
			entry_side, entry_price, entry_quantity, entry_value, entry_fees,
			entry_filled_at, status, remaining_quantity, realized_pnl,
			created_at, updated_at, closed_at
		FROM position_states
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent position states: %w", err)
	}
	defer rows.Close()

	var positions []*orders.PositionState
	for rows.Next() {
		position := &orders.PositionState{}
		err := rows.Scan(
			&position.ID,
			&position.UserID,
			&position.ChainID,
			&position.Symbol,
			&position.EntryOrderID,
			&position.EntryClientOrderID,
			&position.EntrySide,
			&position.EntryPrice,
			&position.EntryQuantity,
			&position.EntryValue,
			&position.EntryFees,
			&position.EntryFilledAt,
			&position.Status,
			&position.RemainingQuantity,
			&position.RealizedPnL,
			&position.CreatedAt,
			&position.UpdatedAt,
			&position.ClosedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position row: %w", err)
		}
		positions = append(positions, position)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating position rows: %w", err)
	}

	return positions, nil
}

// DeletePositionState deletes a position state (for testing/cleanup)
func (db *DB) DeletePositionState(ctx context.Context, id int64) error {
	if db.Pool == nil {
		return nil
	}

	query := `DELETE FROM position_states WHERE id = $1`
	_, err := db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete position state: %w", err)
	}

	return nil
}

// GetPositionStatesByChainIDs retrieves position states for multiple chain IDs (batch query)
// Story 7.14: Order Chain Backend Integration
func (db *DB) GetPositionStatesByChainIDs(ctx context.Context, userID int64, chainIDs []string) (map[string]*orders.PositionState, error) {
	result := make(map[string]*orders.PositionState)
	if db.Pool == nil || len(chainIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT id, user_id, chain_id, symbol, entry_order_id, entry_client_order_id,
			entry_side, entry_price, entry_quantity, entry_value, entry_fees,
			entry_filled_at, status, remaining_quantity, realized_pnl,
			created_at, updated_at, closed_at
		FROM position_states
		WHERE user_id = $1 AND chain_id = ANY($2)`

	rows, err := db.Pool.Query(ctx, query, userID, chainIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get position states by chain IDs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		position := &orders.PositionState{}
		err := rows.Scan(
			&position.ID,
			&position.UserID,
			&position.ChainID,
			&position.Symbol,
			&position.EntryOrderID,
			&position.EntryClientOrderID,
			&position.EntrySide,
			&position.EntryPrice,
			&position.EntryQuantity,
			&position.EntryValue,
			&position.EntryFees,
			&position.EntryFilledAt,
			&position.Status,
			&position.RemainingQuantity,
			&position.RealizedPnL,
			&position.CreatedAt,
			&position.UpdatedAt,
			&position.ClosedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position state row: %w", err)
		}
		result[position.ChainID] = position
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating position state rows: %w", err)
	}

	return result, nil
}

// PositionStateDBAdapter adapts the DB type to implement PositionStateRepository interface
type PositionStateDBAdapter struct {
	db *DB
}

// NewPositionStateDBAdapter creates a new adapter
func NewPositionStateDBAdapter(db *DB) *PositionStateDBAdapter {
	return &PositionStateDBAdapter{db: db}
}

// CreatePositionState implements PositionStateRepository
func (a *PositionStateDBAdapter) CreatePositionState(ctx context.Context, position *orders.PositionState) error {
	return a.db.CreatePositionState(ctx, position)
}

// UpdatePositionState implements PositionStateRepository
func (a *PositionStateDBAdapter) UpdatePositionState(ctx context.Context, position *orders.PositionState) error {
	return a.db.UpdatePositionState(ctx, position)
}

// GetPositionByChainID implements PositionStateRepository
func (a *PositionStateDBAdapter) GetPositionByChainID(ctx context.Context, userID int64, chainID string) (*orders.PositionState, error) {
	return a.db.GetPositionByChainID(ctx, userID, chainID)
}

// GetPositionsByUserID implements PositionStateRepository
func (a *PositionStateDBAdapter) GetPositionsByUserID(ctx context.Context, userID int64, status string) ([]*orders.PositionState, error) {
	return a.db.GetPositionsByUserID(ctx, userID, status)
}

// GetPositionBySymbol implements PositionStateRepository
func (a *PositionStateDBAdapter) GetPositionBySymbol(ctx context.Context, userID int64, symbol string, status string) (*orders.PositionState, error) {
	return a.db.GetPositionBySymbol(ctx, userID, symbol, status)
}

// GetPositionByEntryOrderID implements PositionStateRepository
func (a *PositionStateDBAdapter) GetPositionByEntryOrderID(ctx context.Context, entryOrderID int64) (*orders.PositionState, error) {
	return a.db.GetPositionByEntryOrderID(ctx, entryOrderID)
}
