package database

import (
	"context"
	"log"
)

// RunMultiTenantMigrations runs multi-tenant database migrations
func (db *DB) RunMultiTenantMigrations(ctx context.Context) error {
	log.Println("Running Multi-Tenant database migrations...")

	migrations := []string{
		// =====================================================
		// CORE USER MANAGEMENT TABLES
		// =====================================================

		// Create users table
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			name VARCHAR(255),
			email_verified BOOLEAN DEFAULT FALSE,
			email_verified_at TIMESTAMP,
			subscription_tier VARCHAR(20) DEFAULT 'free',
			subscription_status VARCHAR(20) DEFAULT 'active',
			subscription_expires_at TIMESTAMP,
			stripe_customer_id VARCHAR(100),
			crypto_deposit_address VARCHAR(100),
			api_key_mode VARCHAR(20) DEFAULT 'user_provided',
			profit_share_pct DECIMAL(5,2) DEFAULT 30.00,
			referral_code VARCHAR(20) UNIQUE,
			referred_by UUID REFERENCES users(id),
			is_admin BOOLEAN DEFAULT FALSE,
			last_login_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT valid_tier CHECK (subscription_tier IN ('free', 'trader', 'pro', 'whale')),
			CONSTRAINT valid_status CHECK (subscription_status IN ('active', 'past_due', 'cancelled', 'suspended')),
			CONSTRAINT valid_api_mode CHECK (api_key_mode IN ('user_provided', 'master'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_stripe ON users(stripe_customer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_referral ON users(referral_code)`,
		`CREATE INDEX IF NOT EXISTS idx_users_tier ON users(subscription_tier)`,

		// Create user sessions table for JWT refresh tokens
		`CREATE TABLE IF NOT EXISTS user_sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			refresh_token_hash VARCHAR(255) NOT NULL,
			device_info VARCHAR(500),
			ip_address VARCHAR(45),
			user_agent TEXT,
			expires_at TIMESTAMP NOT NULL,
			revoked_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user ON user_sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_token ON user_sessions(refresh_token_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON user_sessions(expires_at)`,

		// Fix ip_address column type if it was created as INET
		`ALTER TABLE user_sessions ALTER COLUMN ip_address TYPE VARCHAR(45) USING ip_address::VARCHAR`,

		// Create user API keys table (references to HashiCorp Vault)
		`CREATE TABLE IF NOT EXISTS user_api_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			exchange VARCHAR(50) DEFAULT 'binance',
			vault_secret_path VARCHAR(255) NOT NULL,
			api_key_last_four VARCHAR(4),
			label VARCHAR(100),
			is_testnet BOOLEAN DEFAULT TRUE,
			is_active BOOLEAN DEFAULT TRUE,
			permissions JSONB,
			last_validated_at TIMESTAMP,
			validation_status VARCHAR(20) DEFAULT 'pending',
			validation_error TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT valid_validation_status CHECK (validation_status IN ('pending', 'valid', 'invalid', 'expired')),
			UNIQUE(user_id, exchange, is_testnet)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_user ON user_api_keys(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_active ON user_api_keys(is_active)`,

		// Create user trading config table
		`CREATE TABLE IF NOT EXISTS user_trading_configs (
			user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			max_open_positions INT DEFAULT 5,
			max_risk_per_trade DECIMAL(5,2) DEFAULT 2.0,
			default_stop_loss_percent DECIMAL(5,2) DEFAULT 2.0,
			default_take_profit_percent DECIMAL(5,2) DEFAULT 5.0,
			enable_spot BOOLEAN DEFAULT TRUE,
			enable_futures BOOLEAN DEFAULT FALSE,
			futures_default_leverage INT DEFAULT 5,
			futures_margin_type VARCHAR(10) DEFAULT 'CROSSED',
			autopilot_enabled BOOLEAN DEFAULT FALSE,
			autopilot_risk_level VARCHAR(20) DEFAULT 'moderate',
			autopilot_min_confidence DECIMAL(3,2) DEFAULT 0.65,
			autopilot_require_multi_signal BOOLEAN DEFAULT TRUE,
			allowed_symbols TEXT[],
			blocked_symbols TEXT[],
			notification_email BOOLEAN DEFAULT TRUE,
			notification_push BOOLEAN DEFAULT TRUE,
			notification_telegram BOOLEAN DEFAULT FALSE,
			telegram_chat_id VARCHAR(50),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// =====================================================
		// BILLING & PROFIT TRACKING TABLES
		// =====================================================

		// Create profit tracking table for billing
		`CREATE TABLE IF NOT EXISTS user_profit_tracking (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			period_start TIMESTAMP NOT NULL,
			period_end TIMESTAMP NOT NULL,
			starting_balance DECIMAL(20,8) NOT NULL,
			ending_balance DECIMAL(20,8),
			deposits DECIMAL(20,8) DEFAULT 0,
			withdrawals DECIMAL(20,8) DEFAULT 0,
			gross_pnl DECIMAL(20,8) DEFAULT 0,
			loss_carryforward_in DECIMAL(20,8) DEFAULT 0,
			loss_carryforward_out DECIMAL(20,8) DEFAULT 0,
			high_water_mark DECIMAL(20,8) DEFAULT 0,
			net_profit DECIMAL(20,8) DEFAULT 0,
			profit_share_rate DECIMAL(5,4) NOT NULL,
			profit_share_due DECIMAL(20,8) DEFAULT 0,
			settlement_status VARCHAR(20) DEFAULT 'pending',
			settled_at TIMESTAMP,
			stripe_invoice_id VARCHAR(100),
			crypto_tx_hash VARCHAR(100),
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT valid_settlement_status CHECK (settlement_status IN ('pending', 'invoiced', 'paid', 'failed', 'waived')),
			UNIQUE(user_id, period_start)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_profit_tracking_user ON user_profit_tracking(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_profit_tracking_period ON user_profit_tracking(period_start, period_end)`,
		`CREATE INDEX IF NOT EXISTS idx_profit_tracking_status ON user_profit_tracking(settlement_status)`,

		// Create balance snapshots table
		`CREATE TABLE IF NOT EXISTS user_balance_snapshots (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			snapshot_type VARCHAR(20) NOT NULL,
			exchange VARCHAR(50) DEFAULT 'binance',
			spot_balance DECIMAL(20,8) DEFAULT 0,
			futures_balance DECIMAL(20,8) DEFAULT 0,
			total_balance DECIMAL(20,8) NOT NULL,
			unrealized_pnl DECIMAL(20,8) DEFAULT 0,
			margin_balance DECIMAL(20,8) DEFAULT 0,
			available_balance DECIMAL(20,8) DEFAULT 0,
			source VARCHAR(50) DEFAULT 'api',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT valid_snapshot_type CHECK (snapshot_type IN ('hourly', 'daily', 'weekly', 'trade', 'deposit', 'withdrawal', 'manual'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_balance_snapshots_user ON user_balance_snapshots(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_balance_snapshots_type ON user_balance_snapshots(snapshot_type)`,

		// Create balance adjustments table (deposits/withdrawals)
		`CREATE TABLE IF NOT EXISTS user_balance_adjustments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			adjustment_type VARCHAR(20) NOT NULL,
			amount DECIMAL(20,8) NOT NULL,
			asset VARCHAR(20) DEFAULT 'USDT',
			tx_id VARCHAR(255),
			source VARCHAR(50),
			detected_at TIMESTAMP NOT NULL,
			excluded_from_pnl BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT valid_adjustment_type CHECK (adjustment_type IN ('deposit', 'withdrawal', 'transfer_in', 'transfer_out'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_adjustments_user ON user_balance_adjustments(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_adjustments_detected ON user_balance_adjustments(detected_at)`,

		// Create invoices table
		`CREATE TABLE IF NOT EXISTS invoices (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			invoice_number VARCHAR(50) UNIQUE NOT NULL,
			invoice_type VARCHAR(20) NOT NULL,
			subscription_amount DECIMAL(20,8) DEFAULT 0,
			profit_share_amount DECIMAL(20,8) DEFAULT 0,
			total_amount DECIMAL(20,8) NOT NULL,
			currency VARCHAR(10) DEFAULT 'USD',
			status VARCHAR(20) DEFAULT 'draft',
			stripe_invoice_id VARCHAR(100),
			stripe_payment_intent VARCHAR(100),
			crypto_address VARCHAR(100),
			crypto_tx_hash VARCHAR(100),
			period_start TIMESTAMP,
			period_end TIMESTAMP,
			due_date TIMESTAMP,
			paid_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT valid_invoice_type CHECK (invoice_type IN ('subscription', 'profit_share', 'combined')),
			CONSTRAINT valid_invoice_status CHECK (status IN ('draft', 'pending', 'paid', 'failed', 'cancelled', 'refunded'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_user ON invoices(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status)`,
		`CREATE INDEX IF NOT EXISTS idx_invoices_number ON invoices(invoice_number)`,

		// =====================================================
		// RATE LIMITING TABLE
		// =====================================================

		`CREATE TABLE IF NOT EXISTS rate_limit_tracking (
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			endpoint_category VARCHAR(50) NOT NULL,
			window_start TIMESTAMP NOT NULL,
			request_count INT DEFAULT 1,
			PRIMARY KEY (user_id, endpoint_category, window_start)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rate_limit_window ON rate_limit_tracking(window_start)`,

		// =====================================================
		// ADD user_id TO EXISTING TABLES
		// =====================================================

		// Add user_id to trades table
		`ALTER TABLE trades ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_user_id ON trades(user_id)`,

		// Add user_id to orders table
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id)`,

		// Add user_id to signals table
		`ALTER TABLE signals ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_signals_user_id ON signals(user_id)`,

		// Add user_id to pending_signals table
		`ALTER TABLE pending_signals ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_signals_user_id ON pending_signals(user_id)`,

		// Add user_id to strategy_configs table
		`ALTER TABLE strategy_configs ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_strategy_configs_user_id ON strategy_configs(user_id)`,

		// Add user_id to watchlist table
		`ALTER TABLE watchlist ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_watchlist_user_id ON watchlist(user_id)`,
		// Update unique constraint to include user_id
		`ALTER TABLE watchlist DROP CONSTRAINT IF EXISTS watchlist_symbol_key`,

		// Add user_id to position_snapshots table
		`ALTER TABLE position_snapshots ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_position_snapshots_user_id ON position_snapshots(user_id)`,

		// Add user_id to screener_results table
		`ALTER TABLE screener_results ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_screener_results_user_id ON screener_results(user_id)`,

		// Add user_id to scanner_results table
		`ALTER TABLE scanner_results ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_scanner_results_user_id ON scanner_results(user_id)`,

		// Add user_id to system_events table
		`ALTER TABLE system_events ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_system_events_user_id ON system_events(user_id)`,

		// Add user_id to backtest_results table
		`ALTER TABLE backtest_results ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_backtest_results_user_id ON backtest_results(user_id)`,

		// Add user_id to futures_trades table
		`ALTER TABLE futures_trades ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_trades_user_id ON futures_trades(user_id)`,

		// Add user_id to futures_orders table
		`ALTER TABLE futures_orders ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_orders_user_id ON futures_orders(user_id)`,

		// Add user_id to funding_fees table
		`ALTER TABLE funding_fees ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_funding_fees_user_id ON funding_fees(user_id)`,

		// Add user_id to futures_transactions table
		`ALTER TABLE futures_transactions ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_transactions_user_id ON futures_transactions(user_id)`,

		// Add user_id to futures_account_settings table
		`ALTER TABLE futures_account_settings ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_futures_account_settings_user_id ON futures_account_settings(user_id)`,
		// Update unique constraint
		`ALTER TABLE futures_account_settings DROP CONSTRAINT IF EXISTS futures_account_settings_symbol_key`,

		// Add encrypted API key columns for direct database storage (no Vault dependency)
		`ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS encrypted_api_key TEXT`,
		`ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS encrypted_secret_key TEXT`,
		`ALTER TABLE user_api_keys ALTER COLUMN vault_secret_path DROP NOT NULL`,

		// =====================================================
		// TRIGGERS FOR updated_at
		// =====================================================

		`DROP TRIGGER IF EXISTS update_users_updated_at ON users`,
		`CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,

		`DROP TRIGGER IF EXISTS update_user_trading_configs_updated_at ON user_trading_configs`,
		`CREATE TRIGGER update_user_trading_configs_updated_at BEFORE UPDATE ON user_trading_configs
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,

		`DROP TRIGGER IF EXISTS update_user_api_keys_updated_at ON user_api_keys`,
		`CREATE TRIGGER update_user_api_keys_updated_at BEFORE UPDATE ON user_api_keys
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,

		// =====================================================
		// SYSTEM SETTINGS TABLE (for SMTP, encryption keys, etc.)
		// =====================================================

		`CREATE TABLE IF NOT EXISTS system_settings (
			key VARCHAR(100) PRIMARY KEY,
			value TEXT NOT NULL,
			is_encrypted BOOLEAN DEFAULT FALSE,
			description TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_by UUID REFERENCES users(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_system_settings_encrypted ON system_settings(is_encrypted)`,

		// =====================================================
		// EMAIL VERIFICATION CODES TABLE
		// =====================================================

		`CREATE TABLE IF NOT EXISTS email_verification_codes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			code VARCHAR(6) NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			used_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_verification_user ON email_verification_codes(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_verification_code ON email_verification_codes(code)`,
		`CREATE INDEX IF NOT EXISTS idx_verification_expires ON email_verification_codes(expires_at)`,

		// =====================================================
		// CREATE DEFAULT ADMIN USER (for existing data migration)
		// =====================================================

		// Create admin user with email admin@binance-bot.local and password Weber@#2025
		// bcrypt hash generated with cost 12 for: Weber@#2025
		`INSERT INTO users (id, email, password_hash, name, subscription_tier, is_admin, profit_share_pct, email_verified)
		VALUES (
			'00000000-0000-0000-0000-000000000000',
			'admin@binance-bot.local',
			'$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/X4.G1zGrNrQ1L5Ymu',
			'Administrator',
			'whale',
			TRUE,
			5.00,
			TRUE
		) ON CONFLICT (id) DO UPDATE SET
			email = 'admin@binance-bot.local',
			password_hash = '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/X4.G1zGrNrQ1L5Ymu',
			name = 'Administrator',
			email_verified = TRUE`,

		// Assign existing data to admin user
		`UPDATE trades SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE orders SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE signals SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE pending_signals SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE strategy_configs SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE watchlist SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE position_snapshots SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE screener_results SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE system_events SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE backtest_results SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE futures_trades SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE futures_orders SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE funding_fees SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE futures_transactions SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,
		`UPDATE futures_account_settings SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL`,

		// Create default trading config for admin
		`INSERT INTO user_trading_configs (user_id, max_open_positions, enable_futures, autopilot_enabled)
		VALUES ('00000000-0000-0000-0000-000000000000', 100, TRUE, TRUE)
		ON CONFLICT (user_id) DO NOTHING`,

		// =====================================================
		// AI API KEYS TABLE
		// =====================================================

		// Add dry_run_mode column to user_trading_configs (per-user paper/live mode)
		`ALTER TABLE user_trading_configs ADD COLUMN IF NOT EXISTS dry_run_mode BOOLEAN DEFAULT TRUE`,

		// Create user AI keys table for per-user AI provider API keys
		`CREATE TABLE IF NOT EXISTS user_ai_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider VARCHAR(50) NOT NULL,
			encrypted_key TEXT NOT NULL,
			key_last_four VARCHAR(4),
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT valid_ai_provider CHECK (provider IN ('claude', 'openai', 'deepseek')),
			UNIQUE(user_id, provider)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_keys_user ON user_ai_keys(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_keys_provider ON user_ai_keys(provider)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_keys_active ON user_ai_keys(is_active)`,

		// Trigger for updated_at on user_ai_keys
		`DROP TRIGGER IF EXISTS update_user_ai_keys_updated_at ON user_ai_keys`,
		`CREATE TRIGGER update_user_ai_keys_updated_at BEFORE UPDATE ON user_ai_keys
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,

		// =====================================================
		// USER TIMEZONE & SETTLEMENT DATE COLUMNS
		// =====================================================

		// Add timezone column to users table (for daily settlement calculations)
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'Asia/Kolkata'`,

		// Add last_settlement_date column to users table (for daily P&L tracking)
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_settlement_date TIMESTAMP`,

		// =====================================================
		// USER GLOBAL TRADING TIMEZONE COLUMNS
		// =====================================================

		// Add timezone to user_global_trading (IANA timezone name)
		`ALTER TABLE user_global_trading ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'UTC'`,

		// Add timezone_offset to user_global_trading (UTC offset like +05:30)
		`ALTER TABLE user_global_trading ADD COLUMN IF NOT EXISTS timezone_offset VARCHAR(10) DEFAULT '+00:00'`,
	}

	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			log.Printf("Multi-tenant migration %d failed: %v", i+1, err)
			// Continue with other migrations (some may already exist)
			continue
		}
	}

	log.Println("Multi-Tenant database migrations completed successfully")
	return nil
}
