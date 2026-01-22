package database

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER SYSTEM CONTROL CRUD OPERATIONS
// Epic 7 Enhancement: Control switches for trading system components
// =====================================================

// GetUserSystemControl retrieves system control settings for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserSystemControl(ctx context.Context, userID string) (*UserSystemControl, error) {
	query := `
		SELECT id, user_id, order_tracking_system, position_management_system, entry_decision_system,
			created_at, updated_at
		FROM user_system_control
		WHERE user_id = $1
	`

	config := &UserSystemControl{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.OrderTrackingSystem,
		&config.PositionManagementSystem,
		&config.EntryDecisionSystem,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user system control: %w", err)
	}

	return config, nil
}

// GetUserSystemControlOrDefault retrieves system control settings for a user
// Returns defaults if not found in database
func (r *Repository) GetUserSystemControlOrDefault(ctx context.Context, userID string) (*UserSystemControl, error) {
	config, err := r.GetUserSystemControl(ctx, userID)
	if err != nil {
		return nil, err
	}
	if config == nil {
		config = DefaultUserSystemControl()
		config.UserID = userID
	}
	return config, nil
}

// SaveUserSystemControl saves or updates system control settings (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserSystemControl(ctx context.Context, config *UserSystemControl) error {
	query := `
		INSERT INTO user_system_control (
			user_id, order_tracking_system, position_management_system, entry_decision_system
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			order_tracking_system = EXCLUDED.order_tracking_system,
			position_management_system = EXCLUDED.position_management_system,
			entry_decision_system = EXCLUDED.entry_decision_system,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		config.UserID,
		config.OrderTrackingSystem,
		config.PositionManagementSystem,
		config.EntryDecisionSystem,
	)
	if err != nil {
		return fmt.Errorf("failed to save user system control: %w", err)
	}

	return nil
}

// UpdateOrderTrackingSystem updates only the order tracking system setting
func (r *Repository) UpdateOrderTrackingSystem(ctx context.Context, userID, system string) error {
	// Validate system value
	valid := false
	for _, v := range ValidOrderTrackingSystems() {
		if v == system {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid order tracking system: %s", system)
	}

	query := `
		INSERT INTO user_system_control (user_id, order_tracking_system, position_management_system, entry_decision_system)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			order_tracking_system = EXCLUDED.order_tracking_system,
			updated_at = CURRENT_TIMESTAMP
	`

	defaults := DefaultUserSystemControl()
	_, err := r.db.Pool.Exec(ctx, query, userID, system, defaults.PositionManagementSystem, defaults.EntryDecisionSystem)
	if err != nil {
		return fmt.Errorf("failed to update order tracking system: %w", err)
	}

	return nil
}

// UpdatePositionManagementSystem updates only the position management system setting
func (r *Repository) UpdatePositionManagementSystem(ctx context.Context, userID, system string) error {
	// Validate system value
	valid := false
	for _, v := range ValidPositionManagementSystems() {
		if v == system {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid position management system: %s", system)
	}

	query := `
		INSERT INTO user_system_control (user_id, order_tracking_system, position_management_system, entry_decision_system)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			position_management_system = EXCLUDED.position_management_system,
			updated_at = CURRENT_TIMESTAMP
	`

	defaults := DefaultUserSystemControl()
	_, err := r.db.Pool.Exec(ctx, query, userID, defaults.OrderTrackingSystem, system, defaults.EntryDecisionSystem)
	if err != nil {
		return fmt.Errorf("failed to update position management system: %w", err)
	}

	return nil
}

// UpdateEntryDecisionSystem updates only the entry decision system setting
func (r *Repository) UpdateEntryDecisionSystem(ctx context.Context, userID, system string) error {
	// Validate system value
	valid := false
	for _, v := range ValidEntryDecisionSystems() {
		if v == system {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid entry decision system: %s", system)
	}

	query := `
		INSERT INTO user_system_control (user_id, order_tracking_system, position_management_system, entry_decision_system)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			entry_decision_system = EXCLUDED.entry_decision_system,
			updated_at = CURRENT_TIMESTAMP
	`

	defaults := DefaultUserSystemControl()
	_, err := r.db.Pool.Exec(ctx, query, userID, defaults.OrderTrackingSystem, defaults.PositionManagementSystem, system)
	if err != nil {
		return fmt.Errorf("failed to update entry decision system: %w", err)
	}

	return nil
}

// DeleteUserSystemControl deletes system control settings for a user
func (r *Repository) DeleteUserSystemControl(ctx context.Context, userID string) error {
	query := `DELETE FROM user_system_control WHERE user_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user system control: %w", err)
	}
	return nil
}

// InitializeUserSystemControl creates default system control settings for a new user
func (r *Repository) InitializeUserSystemControl(ctx context.Context, userID string) error {
	config := DefaultUserSystemControl()
	config.UserID = userID

	if err := r.SaveUserSystemControl(ctx, config); err != nil {
		log.Printf("[USER-SYSTEM-CONTROL] Warning: Failed to initialize system control: %v", err)
		return err
	}

	log.Printf("[USER-SYSTEM-CONTROL] Initialized system control for user %s (order_tracking=%s, position_management=%s, entry_decision=%s)",
		userID, config.OrderTrackingSystem, config.PositionManagementSystem, config.EntryDecisionSystem)
	return nil
}
