package database

import (
	"time"
)

// ProfitPeriod represents a billing period for profit calculation
type ProfitPeriod struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
	StartingBalance  float64   `json:"starting_balance"`
	EndingBalance    float64   `json:"ending_balance"`
	Deposits         float64   `json:"deposits"`
	Withdrawals      float64   `json:"withdrawals"`
	GrossProfit      float64   `json:"gross_profit"`
	LossCarryforward float64   `json:"loss_carryforward"`
	NetProfit        float64   `json:"net_profit"`
	HighWaterMark    float64   `json:"high_water_mark"`
	ProfitShareRate  float64   `json:"profit_share_rate"`
	ProfitShareDue   float64   `json:"profit_share_due"`
	SettlementStatus string    `json:"settlement_status"`
	StripeInvoiceID  *string   `json:"stripe_invoice_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// BalanceSnapshot represents a point-in-time balance snapshot
type BalanceSnapshot struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	SnapshotType  string    `json:"snapshot_type"` // "daily", "deposit", "withdrawal", "period_start", "period_end"
	TotalBalance  float64   `json:"total_balance"`
	UnrealizedPnL float64   `json:"unrealized_pnl"`
	CreatedAt     time.Time `json:"created_at"`
}

// Transaction represents a deposit or withdrawal
type Transaction struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Type        string     `json:"type"` // "deposit" or "withdrawal"
	Amount      float64    `json:"amount"`
	Currency    string     `json:"currency"`
	TxHash      *string    `json:"tx_hash,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
}

// Invoice type is defined in models_user.go
