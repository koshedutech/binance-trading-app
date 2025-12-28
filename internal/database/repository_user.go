package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER CRUD OPERATIONS
// =====================================================

// CreateUser creates a new user
func (r *Repository) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (
			email, password_hash, name, subscription_tier, subscription_status,
			api_key_mode, profit_share_pct, referral_code, referred_by, is_admin
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		user.Email,
		user.PasswordHash,
		user.Name,
		user.SubscriptionTier,
		user.SubscriptionStatus,
		user.APIKeyMode,
		user.ProfitSharePct,
		user.ReferralCode,
		user.ReferredBy,
		user.IsAdmin,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByID retrieves a user by ID
func (r *Repository) GetUserByID(ctx context.Context, userID string) (*User, error) {
	query := `
		SELECT id, email, password_hash, COALESCE(name, ''), email_verified, email_verified_at,
			subscription_tier, subscription_status, subscription_expires_at,
			COALESCE(stripe_customer_id, ''), COALESCE(crypto_deposit_address, ''),
			api_key_mode, profit_share_pct,
			COALESCE(referral_code, ''), referred_by, is_admin, last_login_at,
			created_at, updated_at
		FROM users WHERE id = $1
	`

	user := &User{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.EmailVerified, &user.EmailVerifiedAt,
		&user.SubscriptionTier, &user.SubscriptionStatus, &user.SubscriptionExpiresAt,
		&user.StripeCustomerID, &user.CryptoDepositAddress, &user.APIKeyMode, &user.ProfitSharePct,
		&user.ReferralCode, &user.ReferredBy, &user.IsAdmin, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, password_hash, COALESCE(name, ''), email_verified, email_verified_at,
			subscription_tier, subscription_status, subscription_expires_at,
			COALESCE(stripe_customer_id, ''), COALESCE(crypto_deposit_address, ''),
			api_key_mode, profit_share_pct,
			COALESCE(referral_code, ''), referred_by, is_admin, last_login_at,
			created_at, updated_at
		FROM users WHERE email = $1
	`

	log.Printf("GetUserByEmail: Looking up user with email: %s", email)

	user := &User{}
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.EmailVerified, &user.EmailVerifiedAt,
		&user.SubscriptionTier, &user.SubscriptionStatus, &user.SubscriptionExpiresAt,
		&user.StripeCustomerID, &user.CryptoDepositAddress, &user.APIKeyMode, &user.ProfitSharePct,
		&user.ReferralCode, &user.ReferredBy, &user.IsAdmin, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		log.Printf("GetUserByEmail: User not found for email: %s", email)
		return nil, nil
	}
	if err != nil {
		log.Printf("GetUserByEmail: Query failed: %v", err)
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	log.Printf("GetUserByEmail: Found user ID=%s, tier=%s", user.ID, user.SubscriptionTier)
	return user, nil
}

// UpdateUser updates a user's profile
func (r *Repository) UpdateUser(ctx context.Context, user *User) error {
	query := `
		UPDATE users SET
			name = $2,
			email_verified = $3,
			email_verified_at = $4,
			subscription_tier = $5,
			subscription_status = $6,
			subscription_expires_at = $7,
			stripe_customer_id = $8,
			crypto_deposit_address = $9,
			api_key_mode = $10,
			profit_share_pct = $11,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query,
		user.ID,
		user.Name,
		user.EmailVerified,
		user.EmailVerifiedAt,
		user.SubscriptionTier,
		user.SubscriptionStatus,
		user.SubscriptionExpiresAt,
		user.StripeCustomerID,
		user.CryptoDepositAddress,
		user.APIKeyMode,
		user.ProfitSharePct,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// UpdateUserPassword updates a user's password
func (r *Repository) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

// UpdateUserLastLogin updates the last login timestamp
func (r *Repository) UpdateUserLastLogin(ctx context.Context, userID string) error {
	query := `UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

// UpdateUserTier updates a user's subscription tier
func (r *Repository) UpdateUserTier(ctx context.Context, userID string, tier SubscriptionTier, profitSharePct float64) error {
	query := `
		UPDATE users SET
			subscription_tier = $2,
			profit_share_pct = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, userID, tier, profitSharePct)
	if err != nil {
		return fmt.Errorf("failed to update user tier: %w", err)
	}
	return nil
}

// GetAllActiveUsers retrieves all users with active subscriptions
func (r *Repository) GetAllActiveUsers(ctx context.Context) ([]*User, error) {
	query := `
		SELECT id, email, password_hash, name, email_verified, email_verified_at,
			subscription_tier, subscription_status, subscription_expires_at,
			stripe_customer_id, crypto_deposit_address, api_key_mode, profit_share_pct,
			referral_code, referred_by, is_admin, last_login_at, created_at, updated_at
		FROM users
		WHERE subscription_status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		err := rows.Scan(
			&user.ID, &user.Email, &user.PasswordHash, &user.Name,
			&user.EmailVerified, &user.EmailVerifiedAt,
			&user.SubscriptionTier, &user.SubscriptionStatus, &user.SubscriptionExpiresAt,
			&user.StripeCustomerID, &user.CryptoDepositAddress, &user.APIKeyMode, &user.ProfitSharePct,
			&user.ReferralCode, &user.ReferredBy, &user.IsAdmin, &user.LastLoginAt,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// =====================================================
// SESSION CRUD OPERATIONS
// =====================================================

// CreateSession creates a new user session
func (r *Repository) CreateSession(ctx context.Context, session *UserSession) error {
	// Use a simpler INSERT without RETURNING to avoid potential scanning issues
	query := `
		INSERT INTO user_sessions (id, user_id, refresh_token_hash, device_info, ip_address, user_agent, expires_at, created_at, last_used_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4::VARCHAR, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	// Handle empty IP address - convert to NULL
	var ipAddress interface{}
	if session.IPAddress == "" {
		ipAddress = nil
	} else {
		ipAddress = session.IPAddress
	}

	log.Printf("CreateSession: Inserting session for user_id=%s", session.UserID)

	_, err := r.db.Pool.Exec(ctx, query,
		session.UserID,
		session.RefreshTokenHash,
		session.DeviceInfo,
		ipAddress,
		session.UserAgent,
		session.ExpiresAt,
	)

	if err != nil {
		log.Printf("CreateSession: Insert failed: %v", err)
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Set defaults for the session object since we're not using RETURNING
	session.CreatedAt = time.Now()
	session.LastUsedAt = time.Now()

	log.Printf("CreateSession: Session created for user %s", session.UserID)
	return nil
}

// GetSessionByTokenHash retrieves a session by refresh token hash
func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*UserSession, error) {
	query := `
		SELECT id, user_id, refresh_token_hash, device_info, ip_address, user_agent,
			expires_at, revoked_at, created_at, last_used_at
		FROM user_sessions
		WHERE refresh_token_hash = $1 AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP
	`

	session := &UserSession{}
	err := r.db.Pool.QueryRow(ctx, query, tokenHash).Scan(
		&session.ID, &session.UserID, &session.RefreshTokenHash,
		&session.DeviceInfo, &session.IPAddress, &session.UserAgent,
		&session.ExpiresAt, &session.RevokedAt, &session.CreatedAt, &session.LastUsedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// UpdateSessionLastUsed updates the last_used_at timestamp
func (r *Repository) UpdateSessionLastUsed(ctx context.Context, sessionID string) error {
	query := `UPDATE user_sessions SET last_used_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	return nil
}

// RevokeSession revokes a session
func (r *Repository) RevokeSession(ctx context.Context, sessionID string) error {
	query := `UPDATE user_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	return nil
}

// RevokeAllUserSessions revokes all sessions for a user
func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID string) error {
	query := `UPDATE user_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all sessions: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes expired sessions
func (r *Repository) DeleteExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM user_sessions WHERE expires_at < CURRENT_TIMESTAMP`
	_, err := r.db.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}
	return nil
}

// =====================================================
// API KEY CRUD OPERATIONS
// =====================================================

// CreateAPIKey creates a new API key reference
func (r *Repository) CreateAPIKey(ctx context.Context, apiKey *UserAPIKey) error {
	query := `
		INSERT INTO user_api_keys (
			user_id, exchange, vault_secret_path, encrypted_api_key, encrypted_secret_key,
			api_key_last_four, label, is_testnet, is_active, permissions, validation_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		apiKey.UserID,
		apiKey.Exchange,
		apiKey.VaultSecretPath,
		apiKey.EncryptedAPIKey,
		apiKey.EncryptedSecretKey,
		apiKey.APIKeyLastFour,
		apiKey.Label,
		apiKey.IsTestnet,
		apiKey.IsActive,
		apiKey.Permissions,
		apiKey.ValidationStatus,
	).Scan(&apiKey.ID, &apiKey.CreatedAt, &apiKey.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	return nil
}

// GetAPIKeysByUserID retrieves all API keys for a user
func (r *Repository) GetAPIKeysByUserID(ctx context.Context, userID string) ([]*UserAPIKey, error) {
	query := `
		SELECT id, user_id, exchange, vault_secret_path,
			COALESCE(api_key_last_four, ''), COALESCE(label, ''),
			is_testnet, is_active, COALESCE(permissions, '{}')::jsonb,
			last_validated_at, validation_status, COALESCE(validation_error, ''),
			created_at, updated_at
		FROM user_api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	var keys []*UserAPIKey
	for rows.Next() {
		key := &UserAPIKey{}
		err := rows.Scan(
			&key.ID, &key.UserID, &key.Exchange, &key.VaultSecretPath, &key.APIKeyLastFour,
			&key.Label, &key.IsTestnet, &key.IsActive, &key.Permissions,
			&key.LastValidatedAt, &key.ValidationStatus, &key.ValidationError,
			&key.CreatedAt, &key.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// GetActiveAPIKey retrieves the active API key for a user/exchange combination
func (r *Repository) GetActiveAPIKey(ctx context.Context, userID, exchange string, testnet bool) (*UserAPIKey, error) {
	query := `
		SELECT id, user_id, exchange, COALESCE(vault_secret_path, ''),
			COALESCE(encrypted_api_key, ''), COALESCE(encrypted_secret_key, ''),
			COALESCE(api_key_last_four, ''), COALESCE(label, ''),
			is_testnet, is_active, COALESCE(permissions, '{}')::jsonb,
			last_validated_at, validation_status, COALESCE(validation_error, ''),
			created_at, updated_at
		FROM user_api_keys
		WHERE user_id = $1 AND exchange = $2 AND is_testnet = $3 AND is_active = true
		LIMIT 1
	`

	key := &UserAPIKey{}
	err := r.db.Pool.QueryRow(ctx, query, userID, exchange, testnet).Scan(
		&key.ID, &key.UserID, &key.Exchange, &key.VaultSecretPath,
		&key.EncryptedAPIKey, &key.EncryptedSecretKey,
		&key.APIKeyLastFour, &key.Label, &key.IsTestnet, &key.IsActive, &key.Permissions,
		&key.LastValidatedAt, &key.ValidationStatus, &key.ValidationError,
		&key.CreatedAt, &key.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return key, nil
}

// UpdateAPIKeyValidation updates the validation status of an API key
func (r *Repository) UpdateAPIKeyValidation(ctx context.Context, keyID string, status ValidationStatus, errorMsg string) error {
	query := `
		UPDATE user_api_keys SET
			last_validated_at = CURRENT_TIMESTAMP,
			validation_status = $2,
			validation_error = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, keyID, status, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to update API key validation: %w", err)
	}
	return nil
}

// DeleteAPIKey deletes an API key
func (r *Repository) DeleteAPIKey(ctx context.Context, keyID string) error {
	query := `DELETE FROM user_api_keys WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, keyID)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	return nil
}

// =====================================================
// TRADING CONFIG CRUD OPERATIONS
// =====================================================

// GetUserTradingConfig retrieves a user's trading configuration
func (r *Repository) GetUserTradingConfig(ctx context.Context, userID string) (*UserTradingConfig, error) {
	query := `
		SELECT user_id, max_open_positions, max_risk_per_trade, default_stop_loss_percent,
			default_take_profit_percent, enable_spot, enable_futures, futures_default_leverage,
			futures_margin_type, autopilot_enabled, autopilot_risk_level, autopilot_min_confidence,
			autopilot_require_multi_signal, allowed_symbols, blocked_symbols,
			notification_email, notification_push, notification_telegram, telegram_chat_id,
			COALESCE(dry_run_mode, true), created_at, updated_at
		FROM user_trading_configs
		WHERE user_id = $1
	`

	config := &UserTradingConfig{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.UserID, &config.MaxOpenPositions, &config.MaxRiskPerTrade,
		&config.DefaultStopLossPercent, &config.DefaultTakeProfitPercent,
		&config.EnableSpot, &config.EnableFutures, &config.FuturesDefaultLeverage,
		&config.FuturesMarginType, &config.AutopilotEnabled, &config.AutopilotRiskLevel,
		&config.AutopilotMinConfidence, &config.AutopilotRequireMultiSign,
		&config.AllowedSymbols, &config.BlockedSymbols,
		&config.NotificationEmail, &config.NotificationPush, &config.NotificationTelegram,
		&config.TelegramChatID, &config.DryRunMode, &config.CreatedAt, &config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get trading config: %w", err)
	}

	return config, nil
}

// UpsertUserTradingConfig creates or updates a user's trading configuration
func (r *Repository) UpsertUserTradingConfig(ctx context.Context, config *UserTradingConfig) error {
	query := `
		INSERT INTO user_trading_configs (
			user_id, max_open_positions, max_risk_per_trade, default_stop_loss_percent,
			default_take_profit_percent, enable_spot, enable_futures, futures_default_leverage,
			futures_margin_type, autopilot_enabled, autopilot_risk_level, autopilot_min_confidence,
			autopilot_require_multi_signal, allowed_symbols, blocked_symbols,
			notification_email, notification_push, notification_telegram, telegram_chat_id, dry_run_mode
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT (user_id) DO UPDATE SET
			max_open_positions = EXCLUDED.max_open_positions,
			max_risk_per_trade = EXCLUDED.max_risk_per_trade,
			default_stop_loss_percent = EXCLUDED.default_stop_loss_percent,
			default_take_profit_percent = EXCLUDED.default_take_profit_percent,
			enable_spot = EXCLUDED.enable_spot,
			enable_futures = EXCLUDED.enable_futures,
			futures_default_leverage = EXCLUDED.futures_default_leverage,
			futures_margin_type = EXCLUDED.futures_margin_type,
			autopilot_enabled = EXCLUDED.autopilot_enabled,
			autopilot_risk_level = EXCLUDED.autopilot_risk_level,
			autopilot_min_confidence = EXCLUDED.autopilot_min_confidence,
			autopilot_require_multi_signal = EXCLUDED.autopilot_require_multi_signal,
			allowed_symbols = EXCLUDED.allowed_symbols,
			blocked_symbols = EXCLUDED.blocked_symbols,
			notification_email = EXCLUDED.notification_email,
			notification_push = EXCLUDED.notification_push,
			notification_telegram = EXCLUDED.notification_telegram,
			telegram_chat_id = EXCLUDED.telegram_chat_id,
			dry_run_mode = EXCLUDED.dry_run_mode,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		config.UserID, config.MaxOpenPositions, config.MaxRiskPerTrade,
		config.DefaultStopLossPercent, config.DefaultTakeProfitPercent,
		config.EnableSpot, config.EnableFutures, config.FuturesDefaultLeverage,
		config.FuturesMarginType, config.AutopilotEnabled, config.AutopilotRiskLevel,
		config.AutopilotMinConfidence, config.AutopilotRequireMultiSign,
		config.AllowedSymbols, config.BlockedSymbols,
		config.NotificationEmail, config.NotificationPush, config.NotificationTelegram,
		config.TelegramChatID, config.DryRunMode,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert trading config: %w", err)
	}

	return nil
}

// GetUserDryRunMode retrieves just the trading mode for a user (fast lookup)
func (r *Repository) GetUserDryRunMode(ctx context.Context, userID string) (bool, error) {
	query := `SELECT COALESCE(dry_run_mode, true) FROM user_trading_configs WHERE user_id = $1`

	var dryRun bool
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&dryRun)

	if err == pgx.ErrNoRows {
		// No config exists, default to paper trading (safe default)
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("failed to get user dry run mode: %w", err)
	}

	return dryRun, nil
}

// SetUserDryRunMode updates just the trading mode for a user
func (r *Repository) SetUserDryRunMode(ctx context.Context, userID string, dryRun bool) error {
	// Use upsert to handle case where user has no config yet
	query := `
		INSERT INTO user_trading_configs (user_id, dry_run_mode)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET
			dry_run_mode = EXCLUDED.dry_run_mode,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, dryRun)
	if err != nil {
		return fmt.Errorf("failed to set user dry run mode: %w", err)
	}

	return nil
}

// =====================================================
// USER STATS & COUNTS
// =====================================================

// GetUserCount returns the total number of users
func (r *Repository) GetUserCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// GetUserCountByTier returns the number of users per tier
func (r *Repository) GetUserCountByTier(ctx context.Context) (map[SubscriptionTier]int64, error) {
	query := `SELECT subscription_tier, COUNT(*) FROM users GROUP BY subscription_tier`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count users by tier: %w", err)
	}
	defer rows.Close()

	counts := make(map[SubscriptionTier]int64)
	for rows.Next() {
		var tier SubscriptionTier
		var count int64
		if err := rows.Scan(&tier, &count); err != nil {
			return nil, err
		}
		counts[tier] = count
	}

	return counts, nil
}

// GetUserByReferralCode retrieves a user by their referral code
func (r *Repository) GetUserByReferralCode(ctx context.Context, code string) (*User, error) {
	query := `
		SELECT id, email, name, subscription_tier, referral_code
		FROM users WHERE referral_code = $1
	`

	user := &User{}
	err := r.db.Pool.QueryRow(ctx, query, code).Scan(
		&user.ID, &user.Email, &user.Name, &user.SubscriptionTier, &user.ReferralCode,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by referral: %w", err)
	}

	return user, nil
}

// GetReferralCount returns the number of users referred by a user
func (r *Repository) GetReferralCount(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE referred_by = $1", userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count referrals: %w", err)
	}
	return count, nil
}

// =====================================================
// ADMIN OPERATIONS
// =====================================================

// ListUsers returns paginated list of users for admin
func (r *Repository) ListUsers(ctx context.Context, limit, offset int, tier string) ([]*User, int64, error) {
	countQuery := "SELECT COUNT(*) FROM users"
	listQuery := `
		SELECT id, email, name, subscription_tier, subscription_status, profit_share_pct,
			is_admin, last_login_at, created_at
		FROM users
	`

	args := []interface{}{}
	if tier != "" {
		countQuery += " WHERE subscription_tier = $1"
		listQuery += " WHERE subscription_tier = $1"
		args = append(args, tier)
	}

	listQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d OFFSET %d", limit, offset)

	var total int64
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		err := rows.Scan(
			&user.ID, &user.Email, &user.Name, &user.SubscriptionTier,
			&user.SubscriptionStatus, &user.ProfitSharePct, &user.IsAdmin,
			&user.LastLoginAt, &user.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, total, nil
}

// SuspendUser suspends a user account
func (r *Repository) SuspendUser(ctx context.Context, userID string) error {
	query := `UPDATE users SET subscription_status = 'suspended', updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}
	return nil
}

// ReactivateUser reactivates a suspended user
func (r *Repository) ReactivateUser(ctx context.Context, userID string) error {
	query := `UPDATE users SET subscription_status = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to reactivate user: %w", err)
	}
	return nil
}

// UpdateUserProfile updates a user's name and/or email
func (r *Repository) UpdateUserProfile(ctx context.Context, userID, name, email string) error {
	query := `
		UPDATE users SET
			name = COALESCE(NULLIF($2, ''), name),
			email = COALESCE(NULLIF($3, ''), email),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, userID, name, email)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}
	return nil
}

// GetUserAPIKeys retrieves all API keys for a user
func (r *Repository) GetUserAPIKeys(ctx context.Context, userID string) ([]*UserAPIKey, error) {
	return r.GetAPIKeysByUserID(ctx, userID)
}

// CreateUserAPIKey creates a new API key for a user
func (r *Repository) CreateUserAPIKey(ctx context.Context, key *UserAPIKey) error {
	return r.CreateAPIKey(ctx, key)
}

// DeleteUserAPIKey deletes an API key ensuring it belongs to the user
func (r *Repository) DeleteUserAPIKey(ctx context.Context, keyID, userID string) error {
	query := `DELETE FROM user_api_keys WHERE id = $1 AND user_id = $2`
	result, err := r.db.Pool.Exec(ctx, query, keyID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("API key not found or not owned by user")
	}
	return nil
}

// GetUserAPIKeyByID retrieves a specific API key ensuring it belongs to the user
func (r *Repository) GetUserAPIKeyByID(ctx context.Context, keyID, userID string) (*UserAPIKey, error) {
	query := `
		SELECT id, user_id, exchange, vault_secret_path,
			COALESCE(api_key_last_four, ''), COALESCE(label, ''),
			is_testnet, is_active, COALESCE(permissions, '{}')::jsonb,
			last_validated_at, validation_status, COALESCE(validation_error, ''),
			created_at, updated_at
		FROM user_api_keys
		WHERE id = $1 AND user_id = $2
	`

	key := &UserAPIKey{}
	err := r.db.Pool.QueryRow(ctx, query, keyID, userID).Scan(
		&key.ID, &key.UserID, &key.Exchange, &key.VaultSecretPath, &key.APIKeyLastFour,
		&key.Label, &key.IsTestnet, &key.IsActive, &key.Permissions,
		&key.LastValidatedAt, &key.ValidationStatus, &key.ValidationError,
		&key.CreatedAt, &key.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return key, nil
}

// EmailExists checks if an email is already registered
func (r *Repository) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check email: %w", err)
	}
	return exists, nil
}

// SetEmailVerified marks an email as verified
func (r *Repository) SetEmailVerified(ctx context.Context, userID string) error {
	query := `UPDATE users SET email_verified = true, email_verified_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}
	return nil
}

// UpdateStripeCustomerID updates the Stripe customer ID
func (r *Repository) UpdateStripeCustomerID(ctx context.Context, userID, stripeID string) error {
	query := `UPDATE users SET stripe_customer_id = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID, stripeID)
	if err != nil {
		return fmt.Errorf("failed to update stripe customer: %w", err)
	}
	return nil
}

// GenerateReferralCode generates a unique referral code for a user
func (r *Repository) GenerateReferralCode(ctx context.Context, userID string) (string, error) {
	// Generate a random 8-character code
	code := fmt.Sprintf("%s%d", userID[:4], time.Now().UnixNano()%100000)

	query := `UPDATE users SET referral_code = $2 WHERE id = $1 RETURNING referral_code`
	var resultCode string
	err := r.db.Pool.QueryRow(ctx, query, userID, code).Scan(&resultCode)
	if err != nil {
		return "", fmt.Errorf("failed to generate referral code: %w", err)
	}
	return resultCode, nil
}

// =====================================================
// STRIPE & SUBSCRIPTION OPERATIONS
// =====================================================

// UpdateUserStripeCustomerID updates the Stripe customer ID for a user
func (r *Repository) UpdateUserStripeCustomerID(ctx context.Context, userID, stripeCustomerID string) error {
	query := `UPDATE users SET stripe_customer_id = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID, stripeCustomerID)
	if err != nil {
		return fmt.Errorf("failed to update stripe customer ID: %w", err)
	}
	return nil
}

// GetUserByStripeCustomerID retrieves a user by their Stripe customer ID
func (r *Repository) GetUserByStripeCustomerID(ctx context.Context, stripeCustomerID string) (*User, error) {
	query := `
		SELECT id, email, password_hash, name, email_verified, email_verified_at,
			subscription_tier, subscription_status, subscription_expires_at,
			stripe_customer_id, crypto_deposit_address, api_key_mode, profit_share_pct,
			referral_code, referred_by, is_admin, last_login_at, created_at, updated_at
		FROM users WHERE stripe_customer_id = $1
	`

	user := &User{}
	err := r.db.Pool.QueryRow(ctx, query, stripeCustomerID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.EmailVerified, &user.EmailVerifiedAt,
		&user.SubscriptionTier, &user.SubscriptionStatus, &user.SubscriptionExpiresAt,
		&user.StripeCustomerID, &user.CryptoDepositAddress, &user.APIKeyMode, &user.ProfitSharePct,
		&user.ReferralCode, &user.ReferredBy, &user.IsAdmin, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by stripe customer ID: %w", err)
	}

	return user, nil
}

// UpdateUserSubscriptionStatus updates a user's subscription status
func (r *Repository) UpdateUserSubscriptionStatus(ctx context.Context, userID string, status SubscriptionStatus) error {
	query := `UPDATE users SET subscription_status = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID, status)
	if err != nil {
		return fmt.Errorf("failed to update subscription status: %w", err)
	}
	return nil
}

// UpdateUserSubscription updates a user's subscription tier and profit share
func (r *Repository) UpdateUserSubscription(ctx context.Context, userID string, tier SubscriptionTier, profitSharePct float64) error {
	query := `
		UPDATE users SET
			subscription_tier = $2,
			profit_share_pct = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, userID, tier, profitSharePct)
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}
	return nil
}

// =====================================================
// EMAIL VERIFICATION CODE OPERATIONS
// =====================================================

// CreateEmailVerificationCode creates a new verification code
func (r *Repository) CreateEmailVerificationCode(ctx context.Context, code *EmailVerificationCode) error {
	query := `
		INSERT INTO email_verification_codes (user_id, code, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	err := r.db.Pool.QueryRow(ctx, query,
		code.UserID,
		code.Code,
		code.ExpiresAt,
	).Scan(&code.ID, &code.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create verification code: %w", err)
	}
	return nil
}

// GetLatestVerificationCode retrieves the latest unused verification code for a user
func (r *Repository) GetLatestVerificationCode(ctx context.Context, userID string) (*EmailVerificationCode, error) {
	query := `
		SELECT id, user_id, code, expires_at, used_at, created_at
		FROM email_verification_codes
		WHERE user_id = $1 AND used_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`

	code := &EmailVerificationCode{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&code.ID, &code.UserID, &code.Code, &code.ExpiresAt, &code.UsedAt, &code.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get verification code: %w", err)
	}

	return code, nil
}

// VerifyEmailCode verifies and marks a code as used
func (r *Repository) VerifyEmailCode(ctx context.Context, userID, code string) (bool, error) {
	// Find matching code
	query := `
		SELECT id, expires_at, used_at
		FROM email_verification_codes
		WHERE user_id = $1 AND code = $2
		ORDER BY created_at DESC
		LIMIT 1
	`

	var id string
	var expiresAt time.Time
	var usedAt *time.Time

	err := r.db.Pool.QueryRow(ctx, query, userID, code).Scan(&id, &expiresAt, &usedAt)
	if err == pgx.ErrNoRows {
		return false, nil // Code not found
	}
	if err != nil {
		return false, fmt.Errorf("failed to lookup verification code: %w", err)
	}

	// Check if already used
	if usedAt != nil {
		return false, nil
	}

	// Check if expired
	if time.Now().After(expiresAt) {
		return false, nil
	}

	// Mark as used
	updateQuery := `UPDATE email_verification_codes SET used_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err = r.db.Pool.Exec(ctx, updateQuery, id)
	if err != nil {
		return false, fmt.Errorf("failed to mark code as used: %w", err)
	}

	// Mark email as verified
	if err := r.SetEmailVerified(ctx, userID); err != nil {
		return false, fmt.Errorf("failed to verify email: %w", err)
	}

	return true, nil
}

// DeleteExpiredVerificationCodes removes expired verification codes (cleanup)
func (r *Repository) DeleteExpiredVerificationCodes(ctx context.Context) error {
	query := `DELETE FROM email_verification_codes WHERE expires_at < CURRENT_TIMESTAMP`
	_, err := r.db.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete expired codes: %w", err)
	}
	return nil
}

// =====================================================
// AI API KEY CRUD OPERATIONS
// =====================================================

// GetUserAIKeys retrieves all AI API keys for a user
func (r *Repository) GetUserAIKeys(ctx context.Context, userID string) ([]*UserAIKey, error) {
	query := `
		SELECT id, user_id, provider, encrypted_key, key_last_four, is_active, created_at, updated_at
		FROM user_ai_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query AI keys: %w", err)
	}
	defer rows.Close()

	var keys []*UserAIKey
	for rows.Next() {
		key := &UserAIKey{}
		err := rows.Scan(
			&key.ID, &key.UserID, &key.Provider, &key.EncryptedKey,
			&key.KeyLastFour, &key.IsActive, &key.CreatedAt, &key.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan AI key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// GetUserAIKey retrieves a specific AI key for a user and provider
func (r *Repository) GetUserAIKey(ctx context.Context, userID string, provider AIProvider) (*UserAIKey, error) {
	query := `
		SELECT id, user_id, provider, encrypted_key, key_last_four, is_active, created_at, updated_at
		FROM user_ai_keys
		WHERE user_id = $1 AND provider = $2
	`

	key := &UserAIKey{}
	err := r.db.Pool.QueryRow(ctx, query, userID, provider).Scan(
		&key.ID, &key.UserID, &key.Provider, &key.EncryptedKey,
		&key.KeyLastFour, &key.IsActive, &key.CreatedAt, &key.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get AI key: %w", err)
	}

	return key, nil
}

// CreateUserAIKey creates a new AI API key for a user
func (r *Repository) CreateUserAIKey(ctx context.Context, key *UserAIKey) error {
	query := `
		INSERT INTO user_ai_keys (user_id, provider, encrypted_key, key_last_four, is_active)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			encrypted_key = EXCLUDED.encrypted_key,
			key_last_four = EXCLUDED.key_last_four,
			is_active = EXCLUDED.is_active,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		key.UserID,
		key.Provider,
		key.EncryptedKey,
		key.KeyLastFour,
		key.IsActive,
	).Scan(&key.ID, &key.CreatedAt, &key.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create AI key: %w", err)
	}

	return nil
}

// DeleteUserAIKey deletes an AI API key ensuring it belongs to the user
func (r *Repository) DeleteUserAIKey(ctx context.Context, keyID, userID string) error {
	query := `DELETE FROM user_ai_keys WHERE id = $1 AND user_id = $2`
	result, err := r.db.Pool.Exec(ctx, query, keyID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete AI key: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("AI key not found or not owned by user")
	}
	return nil
}

// UpdateUserAIKey updates an AI API key
func (r *Repository) UpdateUserAIKey(ctx context.Context, key *UserAIKey) error {
	query := `
		UPDATE user_ai_keys SET
			encrypted_key = $3,
			key_last_four = $4,
			is_active = $5,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND user_id = $2
	`

	result, err := r.db.Pool.Exec(ctx, query,
		key.ID,
		key.UserID,
		key.EncryptedKey,
		key.KeyLastFour,
		key.IsActive,
	)

	if err != nil {
		return fmt.Errorf("failed to update AI key: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("AI key not found or not owned by user")
	}

	return nil
}

// GetUserAIKeyByID retrieves a specific AI key by ID ensuring it belongs to the user
func (r *Repository) GetUserAIKeyByID(ctx context.Context, keyID, userID string) (*UserAIKey, error) {
	query := `
		SELECT id, user_id, provider, encrypted_key, key_last_four, is_active, created_at, updated_at
		FROM user_ai_keys
		WHERE id = $1 AND user_id = $2
	`

	key := &UserAIKey{}
	err := r.db.Pool.QueryRow(ctx, query, keyID, userID).Scan(
		&key.ID, &key.UserID, &key.Provider, &key.EncryptedKey,
		&key.KeyLastFour, &key.IsActive, &key.CreatedAt, &key.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get AI key: %w", err)
	}

	return key, nil
}

// =====================================================
// USER SYMBOL SETTINGS CRUD OPERATIONS
// =====================================================

// UserSymbolSettings represents per-user symbol-specific trading settings
type UserSymbolSettings struct {
	ID               string  `json:"id"`
	UserID           string  `json:"user_id"`
	Symbol           string  `json:"symbol"`
	Category         string  `json:"category"`
	MinConfidence    float64 `json:"min_confidence"`
	MaxPositionUSD   float64 `json:"max_position_usd"`
	SizeMultiplier   float64 `json:"size_multiplier"`
	LeverageOverride int     `json:"leverage_override"`
	Enabled          bool    `json:"enabled"`
	CustomROIPercent float64 `json:"custom_roi_percent"`
	Notes            string  `json:"notes"`
	TotalTrades      int     `json:"total_trades"`
	WinningTrades    int     `json:"winning_trades"`
	TotalPnL         float64 `json:"total_pnl"`
	WinRate          float64 `json:"win_rate"`
	AvgPnL           float64 `json:"avg_pnl"`
	LastUpdated      string  `json:"last_updated"`
}

// GetUserSymbolSettings retrieves symbol settings for a specific user and symbol
func (r *Repository) GetUserSymbolSettings(ctx context.Context, userID, symbol string) (*UserSymbolSettings, error) {
	query := `
		SELECT id, user_id, symbol, COALESCE(category, 'neutral'),
			COALESCE(min_confidence, 0), COALESCE(max_position_usd, 0),
			COALESCE(size_multiplier, 1.0), COALESCE(leverage_override, 0),
			COALESCE(enabled, true), COALESCE(custom_roi_percent, 0),
			COALESCE(notes, ''), COALESCE(total_trades, 0), COALESCE(winning_trades, 0),
			COALESCE(total_pnl, 0), COALESCE(win_rate, 0), COALESCE(avg_pnl, 0),
			COALESCE(to_char(last_updated, 'YYYY-MM-DD HH24:MI:SS'), '')
		FROM user_symbol_settings
		WHERE user_id = $1 AND symbol = $2
	`

	settings := &UserSymbolSettings{}
	err := r.db.Pool.QueryRow(ctx, query, userID, symbol).Scan(
		&settings.ID, &settings.UserID, &settings.Symbol, &settings.Category,
		&settings.MinConfidence, &settings.MaxPositionUSD, &settings.SizeMultiplier,
		&settings.LeverageOverride, &settings.Enabled, &settings.CustomROIPercent,
		&settings.Notes, &settings.TotalTrades, &settings.WinningTrades,
		&settings.TotalPnL, &settings.WinRate, &settings.AvgPnL, &settings.LastUpdated,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user symbol settings: %w", err)
	}

	return settings, nil
}

// GetAllUserSymbolSettings retrieves all symbol settings for a user
func (r *Repository) GetAllUserSymbolSettings(ctx context.Context, userID string) (map[string]*UserSymbolSettings, error) {
	query := `
		SELECT id, user_id, symbol, COALESCE(category, 'neutral'),
			COALESCE(min_confidence, 0), COALESCE(max_position_usd, 0),
			COALESCE(size_multiplier, 1.0), COALESCE(leverage_override, 0),
			COALESCE(enabled, true), COALESCE(custom_roi_percent, 0),
			COALESCE(notes, ''), COALESCE(total_trades, 0), COALESCE(winning_trades, 0),
			COALESCE(total_pnl, 0), COALESCE(win_rate, 0), COALESCE(avg_pnl, 0),
			COALESCE(to_char(last_updated, 'YYYY-MM-DD HH24:MI:SS'), '')
		FROM user_symbol_settings
		WHERE user_id = $1
		ORDER BY symbol
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user symbol settings: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*UserSymbolSettings)
	for rows.Next() {
		settings := &UserSymbolSettings{}
		err := rows.Scan(
			&settings.ID, &settings.UserID, &settings.Symbol, &settings.Category,
			&settings.MinConfidence, &settings.MaxPositionUSD, &settings.SizeMultiplier,
			&settings.LeverageOverride, &settings.Enabled, &settings.CustomROIPercent,
			&settings.Notes, &settings.TotalTrades, &settings.WinningTrades,
			&settings.TotalPnL, &settings.WinRate, &settings.AvgPnL, &settings.LastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user symbol settings: %w", err)
		}
		result[settings.Symbol] = settings
	}

	return result, nil
}

// UpsertUserSymbolSettings creates or updates symbol settings for a user
func (r *Repository) UpsertUserSymbolSettings(ctx context.Context, settings *UserSymbolSettings) error {
	query := `
		INSERT INTO user_symbol_settings (
			user_id, symbol, category, min_confidence, max_position_usd,
			size_multiplier, leverage_override, enabled, custom_roi_percent, notes,
			total_trades, winning_trades, total_pnl, win_rate, avg_pnl, last_updated
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, symbol) DO UPDATE SET
			category = EXCLUDED.category,
			min_confidence = EXCLUDED.min_confidence,
			max_position_usd = EXCLUDED.max_position_usd,
			size_multiplier = EXCLUDED.size_multiplier,
			leverage_override = EXCLUDED.leverage_override,
			enabled = EXCLUDED.enabled,
			custom_roi_percent = EXCLUDED.custom_roi_percent,
			notes = EXCLUDED.notes,
			total_trades = EXCLUDED.total_trades,
			winning_trades = EXCLUDED.winning_trades,
			total_pnl = EXCLUDED.total_pnl,
			win_rate = EXCLUDED.win_rate,
			avg_pnl = EXCLUDED.avg_pnl,
			last_updated = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`

	err := r.db.Pool.QueryRow(ctx, query,
		settings.UserID, settings.Symbol, settings.Category, settings.MinConfidence,
		settings.MaxPositionUSD, settings.SizeMultiplier, settings.LeverageOverride,
		settings.Enabled, settings.CustomROIPercent, settings.Notes,
		settings.TotalTrades, settings.WinningTrades, settings.TotalPnL,
		settings.WinRate, settings.AvgPnL,
	).Scan(&settings.ID)

	if err != nil {
		return fmt.Errorf("failed to upsert user symbol settings: %w", err)
	}

	return nil
}

// SetUserSymbolROI sets just the custom ROI percent for a user's symbol
func (r *Repository) SetUserSymbolROI(ctx context.Context, userID, symbol string, roiPercent float64) error {
	query := `
		INSERT INTO user_symbol_settings (user_id, symbol, custom_roi_percent)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, symbol) DO UPDATE SET
			custom_roi_percent = EXCLUDED.custom_roi_percent,
			last_updated = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, symbol, roiPercent)
	if err != nil {
		return fmt.Errorf("failed to set user symbol ROI: %w", err)
	}

	return nil
}

// GetUserSymbolROI gets just the custom ROI percent for a user's symbol
func (r *Repository) GetUserSymbolROI(ctx context.Context, userID, symbol string) (float64, error) {
	query := `SELECT COALESCE(custom_roi_percent, 0) FROM user_symbol_settings WHERE user_id = $1 AND symbol = $2`

	var roi float64
	err := r.db.Pool.QueryRow(ctx, query, userID, symbol).Scan(&roi)

	if err == pgx.ErrNoRows {
		return 0, nil // No custom ROI set, use defaults
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get user symbol ROI: %w", err)
	}

	return roi, nil
}

// DeleteUserSymbolSettings removes symbol settings for a user
func (r *Repository) DeleteUserSymbolSettings(ctx context.Context, userID, symbol string) error {
	query := `DELETE FROM user_symbol_settings WHERE user_id = $1 AND symbol = $2`
	_, err := r.db.Pool.Exec(ctx, query, userID, symbol)
	if err != nil {
		return fmt.Errorf("failed to delete user symbol settings: %w", err)
	}
	return nil
}
