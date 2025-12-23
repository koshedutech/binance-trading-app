package database

import (
	"context"
	"fmt"
	"time"
)

// Billing-related repository methods for profit tracking and settlement

// CreateProfitPeriod creates a new profit period record
func (r *Repository) CreateProfitPeriod(ctx context.Context, period *ProfitPeriod) error {
	query := `
		INSERT INTO user_profit_tracking (
			user_id, period_start, period_end, starting_balance, ending_balance,
			deposits, withdrawals, gross_profit, loss_carryforward, net_profit,
			high_water_mark, profit_share_rate, profit_share_due, settlement_status,
			stripe_invoice_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW())
		RETURNING id`

	return r.db.Pool.QueryRow(ctx, query,
		period.UserID,
		period.PeriodStart,
		period.PeriodEnd,
		period.StartingBalance,
		period.EndingBalance,
		period.Deposits,
		period.Withdrawals,
		period.GrossProfit,
		period.LossCarryforward,
		period.NetProfit,
		period.HighWaterMark,
		period.ProfitShareRate,
		period.ProfitShareDue,
		period.SettlementStatus,
		period.StripeInvoiceID,
	).Scan(&period.ID)
}

// GetUserProfitPeriods retrieves profit periods for a user
func (r *Repository) GetUserProfitPeriods(ctx context.Context, userID string, limit int) ([]ProfitPeriod, error) {
	query := `
		SELECT id, user_id, period_start, period_end, starting_balance,
			COALESCE(ending_balance, 0), COALESCE(deposits, 0), COALESCE(withdrawals, 0),
			COALESCE(gross_pnl, 0), COALESCE(loss_carryforward_in, 0), COALESCE(net_profit, 0),
			COALESCE(high_water_mark, 0), profit_share_rate, COALESCE(profit_share_due, 0),
			settlement_status, stripe_invoice_id, created_at
		FROM user_profit_tracking
		WHERE user_id = $1
		ORDER BY period_start DESC
		LIMIT $2`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var periods []ProfitPeriod
	for rows.Next() {
		var p ProfitPeriod
		err := rows.Scan(
			&p.ID, &p.UserID, &p.PeriodStart, &p.PeriodEnd, &p.StartingBalance,
			&p.EndingBalance, &p.Deposits, &p.Withdrawals, &p.GrossProfit,
			&p.LossCarryforward, &p.NetProfit, &p.HighWaterMark, &p.ProfitShareRate,
			&p.ProfitShareDue, &p.SettlementStatus, &p.StripeInvoiceID, &p.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		periods = append(periods, p)
	}

	return periods, rows.Err()
}

// GetLatestProfitPeriod gets the most recent profit period for a user before a date
func (r *Repository) GetLatestProfitPeriod(ctx context.Context, userID string, beforeDate time.Time) (*ProfitPeriod, error) {
	query := `
		SELECT id, user_id, period_start, period_end, starting_balance, ending_balance,
			deposits, withdrawals, gross_profit, loss_carryforward, net_profit,
			high_water_mark, profit_share_rate, profit_share_due, settlement_status,
			stripe_invoice_id, created_at
		FROM user_profit_tracking
		WHERE user_id = $1 AND period_end < $2
		ORDER BY period_end DESC
		LIMIT 1`

	var p ProfitPeriod
	err := r.db.Pool.QueryRow(ctx, query, userID, beforeDate).Scan(
		&p.ID, &p.UserID, &p.PeriodStart, &p.PeriodEnd, &p.StartingBalance,
		&p.EndingBalance, &p.Deposits, &p.Withdrawals, &p.GrossProfit,
		&p.LossCarryforward, &p.NetProfit, &p.HighWaterMark, &p.ProfitShareRate,
		&p.ProfitShareDue, &p.SettlementStatus, &p.StripeInvoiceID, &p.CreatedAt,
	)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	return &p, nil
}

// UpdateProfitPeriodStatus updates the settlement status of a profit period
func (r *Repository) UpdateProfitPeriodStatus(ctx context.Context, periodID string, status string, stripeInvoiceID *string) error {
	query := `
		UPDATE user_profit_tracking
		SET settlement_status = $2, stripe_invoice_id = $3
		WHERE id = $1`

	_, err := r.db.Pool.Exec(ctx, query, periodID, status, stripeInvoiceID)
	return err
}

// GetPendingProfitPeriods gets all periods awaiting settlement
func (r *Repository) GetPendingProfitPeriods(ctx context.Context) ([]ProfitPeriod, error) {
	query := `
		SELECT id, user_id, period_start, period_end, starting_balance, ending_balance,
			deposits, withdrawals, gross_profit, loss_carryforward, net_profit,
			high_water_mark, profit_share_rate, profit_share_due, settlement_status,
			stripe_invoice_id, created_at
		FROM user_profit_tracking
		WHERE settlement_status = 'pending' AND profit_share_due > 0
		ORDER BY period_end ASC`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var periods []ProfitPeriod
	for rows.Next() {
		var p ProfitPeriod
		err := rows.Scan(
			&p.ID, &p.UserID, &p.PeriodStart, &p.PeriodEnd, &p.StartingBalance,
			&p.EndingBalance, &p.Deposits, &p.Withdrawals, &p.GrossProfit,
			&p.LossCarryforward, &p.NetProfit, &p.HighWaterMark, &p.ProfitShareRate,
			&p.ProfitShareDue, &p.SettlementStatus, &p.StripeInvoiceID, &p.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		periods = append(periods, p)
	}

	return periods, rows.Err()
}

// Balance Snapshot methods

// CreateBalanceSnapshot creates a new balance snapshot
func (r *Repository) CreateBalanceSnapshot(ctx context.Context, snapshot *BalanceSnapshot) error {
	query := `
		INSERT INTO user_balance_snapshots (
			user_id, snapshot_type, total_balance, unrealized_pnl, created_at
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	return r.db.Pool.QueryRow(ctx, query,
		snapshot.UserID,
		snapshot.SnapshotType,
		snapshot.TotalBalance,
		snapshot.UnrealizedPnL,
		snapshot.CreatedAt,
	).Scan(&snapshot.ID)
}

// GetLatestBalanceSnapshot gets the most recent balance snapshot for a user
func (r *Repository) GetLatestBalanceSnapshot(ctx context.Context, userID string) (*BalanceSnapshot, error) {
	query := `
		SELECT id, user_id, snapshot_type, total_balance, unrealized_pnl, created_at
		FROM user_balance_snapshots
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	var s BalanceSnapshot
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&s.ID, &s.UserID, &s.SnapshotType, &s.TotalBalance, &s.UnrealizedPnL, &s.CreatedAt,
	)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	return &s, nil
}

// GetBalanceSnapshotNear gets the balance snapshot closest to a timestamp
func (r *Repository) GetBalanceSnapshotNear(ctx context.Context, userID string, timestamp time.Time) (*BalanceSnapshot, error) {
	// Try to get snapshot just before or at the timestamp
	query := `
		SELECT id, user_id, snapshot_type, total_balance, unrealized_pnl, created_at
		FROM user_balance_snapshots
		WHERE user_id = $1 AND created_at <= $2
		ORDER BY created_at DESC
		LIMIT 1`

	var s BalanceSnapshot
	err := r.db.Pool.QueryRow(ctx, query, userID, timestamp).Scan(
		&s.ID, &s.UserID, &s.SnapshotType, &s.TotalBalance, &s.UnrealizedPnL, &s.CreatedAt,
	)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	return &s, nil
}

// Transaction methods

// CreateTransaction records a deposit or withdrawal
func (r *Repository) CreateTransaction(ctx context.Context, tx *Transaction) error {
	query := `
		INSERT INTO user_transactions (
			user_id, type, amount, currency, tx_hash, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id`

	return r.db.Pool.QueryRow(ctx, query,
		tx.UserID,
		tx.Type,
		tx.Amount,
		tx.Currency,
		tx.TxHash,
		tx.Status,
	).Scan(&tx.ID)
}

// GetUserTransactions gets transactions for a user in a time period
func (r *Repository) GetUserTransactions(ctx context.Context, userID string, start, end time.Time) ([]Transaction, error) {
	query := `
		SELECT id, user_id, type, amount, currency, tx_hash, status, created_at, confirmed_at
		FROM user_transactions
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		ORDER BY created_at ASC`

	rows, err := r.db.Pool.Query(ctx, query, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		err := rows.Scan(
			&t.ID, &t.UserID, &t.Type, &t.Amount, &t.Currency,
			&t.TxHash, &t.Status, &t.CreatedAt, &t.ConfirmedAt,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}

	return transactions, rows.Err()
}

// UpdateTransactionStatus updates the status of a transaction
func (r *Repository) UpdateTransactionStatus(ctx context.Context, txID, status string) error {
	query := `UPDATE user_transactions SET status = $2 WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, txID, status)
	return err
}

// ConfirmTransaction marks a transaction as confirmed
func (r *Repository) ConfirmTransaction(ctx context.Context, txID string) error {
	query := `UPDATE user_transactions SET status = 'confirmed', confirmed_at = NOW() WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, txID)
	return err
}

// Trade period queries

// GetTradesForPeriod gets closed trades for a user in a time period
func (r *Repository) GetTradesForPeriod(ctx context.Context, userID string, start, end time.Time) ([]*Trade, error) {
	query := `
		SELECT id, symbol, side, entry_price, exit_price, quantity, entry_time, exit_time,
			status, pnl, pnl_percent, strategy_name, created_at, updated_at
		FROM trades
		WHERE user_id = $1 AND status = 'CLOSED' AND exit_time >= $2 AND exit_time < $3
		ORDER BY exit_time ASC`

	rows, err := r.db.Pool.Query(ctx, query, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		t := &Trade{}
		err := rows.Scan(
			&t.ID, &t.Symbol, &t.Side, &t.EntryPrice, &t.ExitPrice, &t.Quantity,
			&t.EntryTime, &t.ExitTime, &t.Status, &t.PnL, &t.PnLPercent,
			&t.StrategyName, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}

	return trades, rows.Err()
}

// GetFuturesTradesForPeriod gets closed futures trades for a user in a time period
func (r *Repository) GetFuturesTradesForPeriod(ctx context.Context, userID string, start, end time.Time) ([]FuturesTrade, error) {
	query := `
		SELECT id, symbol, side, position_side, entry_price, exit_price, quantity,
			leverage, margin_type, entry_time, exit_time, status, realized_pnl,
			trade_source, created_at, updated_at
		FROM futures_trades
		WHERE status = 'CLOSED' AND exit_time >= $1 AND exit_time < $2
		ORDER BY exit_time ASC`

	rows, err := r.db.Pool.Query(ctx, query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []FuturesTrade
	for rows.Next() {
		t := FuturesTrade{}
		err := rows.Scan(
			&t.ID, &t.Symbol, &t.Side, &t.PositionSide, &t.EntryPrice, &t.ExitPrice,
			&t.Quantity, &t.Leverage, &t.MarginType, &t.EntryTime, &t.ExitTime,
			&t.Status, &t.RealizedPnL, &t.TradeSource, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}

	return trades, rows.Err()
}

// Invoice methods

// CreateInvoice creates a new invoice record
func (r *Repository) CreateInvoice(ctx context.Context, invoice *Invoice) error {
	query := `
		INSERT INTO invoices (
			user_id, invoice_number, invoice_type, subscription_amount, profit_share_amount,
			total_amount, currency, status, stripe_invoice_id, period_start, period_end,
			due_date, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
		RETURNING id`

	return r.db.Pool.QueryRow(ctx, query,
		invoice.UserID,
		invoice.InvoiceNumber,
		invoice.InvoiceType,
		invoice.SubscriptionAmount,
		invoice.ProfitShareAmount,
		invoice.TotalAmount,
		invoice.Currency,
		invoice.Status,
		invoice.StripeInvoiceID,
		invoice.PeriodStart,
		invoice.PeriodEnd,
		invoice.DueDate,
	).Scan(&invoice.ID)
}

// GetUserInvoices gets invoices for a user
func (r *Repository) GetUserInvoices(ctx context.Context, userID string, limit int) ([]Invoice, error) {
	query := `
		SELECT id, user_id, invoice_number, invoice_type, subscription_amount,
			profit_share_amount, total_amount, currency, status, stripe_invoice_id,
			period_start, period_end, due_date, paid_at, created_at
		FROM invoices
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []Invoice
	for rows.Next() {
		var inv Invoice
		err := rows.Scan(
			&inv.ID, &inv.UserID, &inv.InvoiceNumber, &inv.InvoiceType,
			&inv.SubscriptionAmount, &inv.ProfitShareAmount, &inv.TotalAmount,
			&inv.Currency, &inv.Status, &inv.StripeInvoiceID,
			&inv.PeriodStart, &inv.PeriodEnd, &inv.DueDate, &inv.PaidAt, &inv.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}

	return invoices, rows.Err()
}

// UpdateInvoiceStatus updates the status of an invoice
func (r *Repository) UpdateInvoiceStatus(ctx context.Context, invoiceID, status string, paidAt *time.Time) error {
	query := `UPDATE invoices SET status = $2, paid_at = $3 WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, invoiceID, status, paidAt)
	return err
}

// GetActiveUsers gets all users with active subscriptions
func (r *Repository) GetActiveUsers(ctx context.Context) ([]*User, error) {
	query := `
		SELECT id, email, password_hash, name, subscription_tier, subscription_status,
			stripe_customer_id, api_key_mode, profit_share_pct, referral_code, referred_by,
			is_admin, email_verified, created_at, updated_at, last_login_at
		FROM users
		WHERE subscription_status = 'active'`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		err := rows.Scan(
			&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.SubscriptionTier,
			&u.SubscriptionStatus, &u.StripeCustomerID, &u.APIKeyMode, &u.ProfitSharePct,
			&u.ReferralCode, &u.ReferredBy, &u.IsAdmin, &u.EmailVerified,
			&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

// GetUserTotalProfitShareDue gets the total unpaid profit share for a user
func (r *Repository) GetUserTotalProfitShareDue(ctx context.Context, userID string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(profit_share_due), 0)
		FROM user_profit_tracking
		WHERE user_id = $1 AND settlement_status = 'pending'`

	var total float64
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&total)
	return total, err
}

// GetPlatformTotalProfitShareDue gets the total unpaid profit share across all users
func (r *Repository) GetPlatformTotalProfitShareDue(ctx context.Context) (float64, error) {
	query := `
		SELECT COALESCE(SUM(profit_share_due), 0)
		FROM user_profit_tracking
		WHERE settlement_status = 'pending'`

	var total float64
	err := r.db.Pool.QueryRow(ctx, query).Scan(&total)
	return total, err
}

// GetPlatformProfitStats gets aggregate profit statistics
func (r *Repository) GetPlatformProfitStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(DISTINCT user_id) as total_users,
			COALESCE(SUM(profit_share_due), 0) as total_pending,
			COALESCE(SUM(CASE WHEN settlement_status = 'paid' THEN profit_share_due ELSE 0 END), 0) as total_collected,
			COALESCE(SUM(net_profit), 0) as total_user_profit,
			COUNT(*) as total_periods
		FROM user_profit_tracking`

	var totalUsers int
	var totalPending, totalCollected, totalUserProfit float64
	var totalPeriods int

	err := r.db.Pool.QueryRow(ctx, query).Scan(
		&totalUsers, &totalPending, &totalCollected, &totalUserProfit, &totalPeriods,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get platform profit stats: %w", err)
	}

	return map[string]interface{}{
		"total_users":       totalUsers,
		"total_pending":     totalPending,
		"total_collected":   totalCollected,
		"total_user_profit": totalUserProfit,
		"total_periods":     totalPeriods,
	}, nil
}
