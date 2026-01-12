package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER SAFETY SETTINGS CRUD OPERATIONS
// Story 9.4: Per-user safety controls per trading mode
// =====================================================

// UserSafetySettings represents per-user safety controls per mode
type UserSafetySettings struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Mode   string `json:"mode"` // ultra_fast, scalp, swing, position

	// Rate limiting
	MaxTradesPerMinute int `json:"max_trades_per_minute"`
	MaxTradesPerHour   int `json:"max_trades_per_hour"`
	MaxTradesPerDay    int `json:"max_trades_per_day"`

	// Profit monitoring
	EnableProfitMonitor    bool    `json:"enable_profit_monitor"`
	ProfitWindowMinutes    int     `json:"profit_window_minutes"`
	MaxLossPercentInWindow float64 `json:"max_loss_percent_in_window"`
	PauseCooldownMinutes   int     `json:"pause_cooldown_minutes"`

	// Win-rate monitoring
	EnableWinRateMonitor   bool    `json:"enable_win_rate_monitor"`
	WinRateSampleSize      int     `json:"win_rate_sample_size"`
	MinWinRateThreshold    float64 `json:"min_win_rate_threshold"`
	WinRateCooldownMinutes int     `json:"win_rate_cooldown_minutes"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultUserSafetySettings returns default safety settings for a mode
func DefaultUserSafetySettings(mode string) *UserSafetySettings {
	settings := &UserSafetySettings{
		Mode:               mode,
		EnableProfitMonitor:  true,
		EnableWinRateMonitor: true,
	}

	switch mode {
	case "ultra_fast":
		settings.MaxTradesPerMinute = 5
		settings.MaxTradesPerHour = 20
		settings.MaxTradesPerDay = 50
		settings.ProfitWindowMinutes = 10
		settings.MaxLossPercentInWindow = -1.5
		settings.PauseCooldownMinutes = 30
		settings.WinRateSampleSize = 15
		settings.MinWinRateThreshold = 50
		settings.WinRateCooldownMinutes = 60
	case "scalp":
		settings.MaxTradesPerMinute = 8
		settings.MaxTradesPerHour = 30
		settings.MaxTradesPerDay = 100
		settings.ProfitWindowMinutes = 15
		settings.MaxLossPercentInWindow = -2.0
		settings.PauseCooldownMinutes = 30
		settings.WinRateSampleSize = 20
		settings.MinWinRateThreshold = 50
		settings.WinRateCooldownMinutes = 60
	case "swing":
		settings.MaxTradesPerMinute = 10
		settings.MaxTradesPerHour = 30
		settings.MaxTradesPerDay = 80
		settings.ProfitWindowMinutes = 60
		settings.MaxLossPercentInWindow = -3.0
		settings.PauseCooldownMinutes = 60
		settings.WinRateSampleSize = 25
		settings.MinWinRateThreshold = 55
		settings.WinRateCooldownMinutes = 120
	case "position":
		settings.MaxTradesPerMinute = 5
		settings.MaxTradesPerHour = 15
		settings.MaxTradesPerDay = 50
		settings.ProfitWindowMinutes = 120
		settings.MaxLossPercentInWindow = -5.0
		settings.PauseCooldownMinutes = 120
		settings.WinRateSampleSize = 30
		settings.MinWinRateThreshold = 60
		settings.WinRateCooldownMinutes = 180
	default:
		// Default to scalp settings if unknown mode
		settings.MaxTradesPerMinute = 8
		settings.MaxTradesPerHour = 30
		settings.MaxTradesPerDay = 100
		settings.ProfitWindowMinutes = 15
		settings.MaxLossPercentInWindow = -2.0
		settings.PauseCooldownMinutes = 30
		settings.WinRateSampleSize = 20
		settings.MinWinRateThreshold = 50
		settings.WinRateCooldownMinutes = 60
	}

	return settings
}

// GetUserSafetySettings retrieves safety settings for a user and mode
// Returns nil if not found (allows calling code to use defaults)
func (r *Repository) GetUserSafetySettings(ctx context.Context, userID, mode string) (*UserSafetySettings, error) {
	query := `
		SELECT id, user_id, mode,
			max_trades_per_minute, max_trades_per_hour, max_trades_per_day,
			enable_profit_monitor, profit_window_minutes, max_loss_percent_in_window, pause_cooldown_minutes,
			enable_win_rate_monitor, win_rate_sample_size, min_win_rate_threshold, win_rate_cooldown_minutes,
			created_at, updated_at
		FROM user_safety_settings
		WHERE user_id = $1 AND mode = $2
	`

	settings := &UserSafetySettings{}
	err := r.db.Pool.QueryRow(ctx, query, userID, mode).Scan(
		&settings.ID,
		&settings.UserID,
		&settings.Mode,
		&settings.MaxTradesPerMinute,
		&settings.MaxTradesPerHour,
		&settings.MaxTradesPerDay,
		&settings.EnableProfitMonitor,
		&settings.ProfitWindowMinutes,
		&settings.MaxLossPercentInWindow,
		&settings.PauseCooldownMinutes,
		&settings.EnableWinRateMonitor,
		&settings.WinRateSampleSize,
		&settings.MinWinRateThreshold,
		&settings.WinRateCooldownMinutes,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found, return nil without error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user safety settings: %w", err)
	}

	return settings, nil
}

// GetAllUserSafetySettings retrieves all safety settings for a user (all modes)
func (r *Repository) GetAllUserSafetySettings(ctx context.Context, userID string) (map[string]*UserSafetySettings, error) {
	query := `
		SELECT id, user_id, mode,
			max_trades_per_minute, max_trades_per_hour, max_trades_per_day,
			enable_profit_monitor, profit_window_minutes, max_loss_percent_in_window, pause_cooldown_minutes,
			enable_win_rate_monitor, win_rate_sample_size, min_win_rate_threshold, win_rate_cooldown_minutes,
			created_at, updated_at
		FROM user_safety_settings
		WHERE user_id = $1
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user safety settings: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*UserSafetySettings)
	for rows.Next() {
		settings := &UserSafetySettings{}
		err := rows.Scan(
			&settings.ID,
			&settings.UserID,
			&settings.Mode,
			&settings.MaxTradesPerMinute,
			&settings.MaxTradesPerHour,
			&settings.MaxTradesPerDay,
			&settings.EnableProfitMonitor,
			&settings.ProfitWindowMinutes,
			&settings.MaxLossPercentInWindow,
			&settings.PauseCooldownMinutes,
			&settings.EnableWinRateMonitor,
			&settings.WinRateSampleSize,
			&settings.MinWinRateThreshold,
			&settings.WinRateCooldownMinutes,
			&settings.CreatedAt,
			&settings.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user safety settings: %w", err)
		}
		result[settings.Mode] = settings
	}

	return result, nil
}

// SaveUserSafetySettings saves or updates safety settings (UPSERT)
func (r *Repository) SaveUserSafetySettings(ctx context.Context, settings *UserSafetySettings) error {
	query := `
		INSERT INTO user_safety_settings (
			user_id, mode,
			max_trades_per_minute, max_trades_per_hour, max_trades_per_day,
			enable_profit_monitor, profit_window_minutes, max_loss_percent_in_window, pause_cooldown_minutes,
			enable_win_rate_monitor, win_rate_sample_size, min_win_rate_threshold, win_rate_cooldown_minutes,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW())
		ON CONFLICT (user_id, mode) DO UPDATE SET
			max_trades_per_minute = EXCLUDED.max_trades_per_minute,
			max_trades_per_hour = EXCLUDED.max_trades_per_hour,
			max_trades_per_day = EXCLUDED.max_trades_per_day,
			enable_profit_monitor = EXCLUDED.enable_profit_monitor,
			profit_window_minutes = EXCLUDED.profit_window_minutes,
			max_loss_percent_in_window = EXCLUDED.max_loss_percent_in_window,
			pause_cooldown_minutes = EXCLUDED.pause_cooldown_minutes,
			enable_win_rate_monitor = EXCLUDED.enable_win_rate_monitor,
			win_rate_sample_size = EXCLUDED.win_rate_sample_size,
			min_win_rate_threshold = EXCLUDED.min_win_rate_threshold,
			win_rate_cooldown_minutes = EXCLUDED.win_rate_cooldown_minutes,
			updated_at = NOW()
	`

	_, err := r.db.Pool.Exec(ctx, query,
		settings.UserID,
		settings.Mode,
		settings.MaxTradesPerMinute,
		settings.MaxTradesPerHour,
		settings.MaxTradesPerDay,
		settings.EnableProfitMonitor,
		settings.ProfitWindowMinutes,
		settings.MaxLossPercentInWindow,
		settings.PauseCooldownMinutes,
		settings.EnableWinRateMonitor,
		settings.WinRateSampleSize,
		settings.MinWinRateThreshold,
		settings.WinRateCooldownMinutes,
	)
	if err != nil {
		return fmt.Errorf("failed to save user safety settings: %w", err)
	}

	return nil
}

// InitializeUserSafetySettings creates default safety settings for all modes for a new user
func (r *Repository) InitializeUserSafetySettings(ctx context.Context, userID string) error {
	modes := []string{"ultra_fast", "scalp", "swing", "position"}

	for _, mode := range modes {
		settings := DefaultUserSafetySettings(mode)
		settings.UserID = userID
		if err := r.SaveUserSafetySettings(ctx, settings); err != nil {
			log.Printf("[USER-SAFETY] Warning: Failed to initialize %s safety settings: %v", mode, err)
		}
	}

	log.Printf("[USER-SAFETY] Initialized safety settings for user %s (all 4 modes)", userID)
	return nil
}

// DeleteUserSafetySettings deletes safety settings for a user (all modes)
func (r *Repository) DeleteUserSafetySettings(ctx context.Context, userID string) error {
	query := `DELETE FROM user_safety_settings WHERE user_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user safety settings: %w", err)
	}
	return nil
}
