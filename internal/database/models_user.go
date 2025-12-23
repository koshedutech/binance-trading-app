package database

import (
	"time"
)

// SubscriptionTier represents the user's subscription level
type SubscriptionTier string

const (
	TierFree   SubscriptionTier = "free"
	TierTrader SubscriptionTier = "trader"
	TierPro    SubscriptionTier = "pro"
	TierWhale  SubscriptionTier = "whale"
)

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus string

const (
	StatusActive    SubscriptionStatus = "active"
	StatusPastDue   SubscriptionStatus = "past_due"
	StatusCancelled SubscriptionStatus = "cancelled"
	StatusSuspended SubscriptionStatus = "suspended"
)

// APIKeyMode represents how the user's trades are executed
type APIKeyMode string

const (
	APIKeyModeUserProvided APIKeyMode = "user_provided"
	APIKeyModeMaster       APIKeyMode = "master"
)

// User represents a platform user
type User struct {
	ID                    string             `json:"id"`
	Email                 string             `json:"email"`
	PasswordHash          string             `json:"-"` // Never serialize
	Name                  string             `json:"name,omitempty"`
	EmailVerified         bool               `json:"email_verified"`
	EmailVerifiedAt       *time.Time         `json:"email_verified_at,omitempty"`
	SubscriptionTier      SubscriptionTier   `json:"subscription_tier"`
	SubscriptionStatus    SubscriptionStatus `json:"subscription_status"`
	SubscriptionExpiresAt *time.Time         `json:"subscription_expires_at,omitempty"`
	StripeCustomerID      string             `json:"stripe_customer_id,omitempty"`
	CryptoDepositAddress  string             `json:"crypto_deposit_address,omitempty"`
	APIKeyMode            APIKeyMode         `json:"api_key_mode"`
	ProfitSharePct        float64            `json:"profit_share_pct"`
	ReferralCode          string             `json:"referral_code,omitempty"`
	ReferredBy            *string            `json:"referred_by,omitempty"`
	IsAdmin               bool               `json:"is_admin"`
	LastLoginAt           *time.Time         `json:"last_login_at,omitempty"`
	CreatedAt             time.Time          `json:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at"`
}

// UserSession represents an active user session with refresh token
type UserSession struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	RefreshTokenHash string     `json:"-"` // Never serialize
	DeviceInfo       string     `json:"device_info,omitempty"`
	IPAddress        string     `json:"ip_address,omitempty"`
	UserAgent        string     `json:"user_agent,omitempty"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	LastUsedAt       time.Time  `json:"last_used_at"`
}

// ValidationStatus for API keys
type ValidationStatus string

const (
	ValidationPending ValidationStatus = "pending"
	ValidationValid   ValidationStatus = "valid"
	ValidationInvalid ValidationStatus = "invalid"
	ValidationExpired ValidationStatus = "expired"
)

// UserAPIKey represents a user's exchange API key reference
type UserAPIKey struct {
	ID               string                 `json:"id"`
	UserID           string                 `json:"user_id"`
	Exchange         string                 `json:"exchange"`
	VaultSecretPath  string                 `json:"-"` // Never expose vault path
	APIKeyLastFour   string                 `json:"api_key_last_four,omitempty"`
	Label            string                 `json:"label,omitempty"`
	IsTestnet        bool                   `json:"is_testnet"`
	IsActive         bool                   `json:"is_active"`
	Permissions      map[string]interface{} `json:"permissions,omitempty"`
	LastValidatedAt  *time.Time             `json:"last_validated_at,omitempty"`
	ValidationStatus ValidationStatus       `json:"validation_status"`
	ValidationError  string                 `json:"validation_error,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// UserTradingConfig represents per-user trading configuration
type UserTradingConfig struct {
	UserID                    string   `json:"user_id"`
	MaxOpenPositions          int      `json:"max_open_positions"`
	MaxRiskPerTrade           float64  `json:"max_risk_per_trade"`
	DefaultStopLossPercent    float64  `json:"default_stop_loss_percent"`
	DefaultTakeProfitPercent  float64  `json:"default_take_profit_percent"`
	EnableSpot                bool     `json:"enable_spot"`
	EnableFutures             bool     `json:"enable_futures"`
	FuturesDefaultLeverage    int      `json:"futures_default_leverage"`
	FuturesMarginType         string   `json:"futures_margin_type"`
	AutopilotEnabled          bool     `json:"autopilot_enabled"`
	AutopilotRiskLevel        string   `json:"autopilot_risk_level"`
	AutopilotMinConfidence    float64  `json:"autopilot_min_confidence"`
	AutopilotRequireMultiSign bool     `json:"autopilot_require_multi_signal"`
	AllowedSymbols            []string `json:"allowed_symbols,omitempty"`
	BlockedSymbols            []string `json:"blocked_symbols,omitempty"`
	NotificationEmail         bool     `json:"notification_email"`
	NotificationPush          bool     `json:"notification_push"`
	NotificationTelegram      bool     `json:"notification_telegram"`
	TelegramChatID            string   `json:"telegram_chat_id,omitempty"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// SettlementStatus for profit tracking
type SettlementStatus string

const (
	SettlementPending  SettlementStatus = "pending"
	SettlementInvoiced SettlementStatus = "invoiced"
	SettlementPaid     SettlementStatus = "paid"
	SettlementFailed   SettlementStatus = "failed"
	SettlementWaived   SettlementStatus = "waived"
)

// UserProfitTracking represents a billing period's profit calculation
type UserProfitTracking struct {
	ID                  string           `json:"id"`
	UserID              string           `json:"user_id"`
	PeriodStart         time.Time        `json:"period_start"`
	PeriodEnd           time.Time        `json:"period_end"`
	StartingBalance     float64          `json:"starting_balance"`
	EndingBalance       float64          `json:"ending_balance"`
	Deposits            float64          `json:"deposits"`
	Withdrawals         float64          `json:"withdrawals"`
	GrossPnL            float64          `json:"gross_pnl"`
	LossCarryforwardIn  float64          `json:"loss_carryforward_in"`
	LossCarryforwardOut float64          `json:"loss_carryforward_out"`
	HighWaterMark       float64          `json:"high_water_mark"`
	NetProfit           float64          `json:"net_profit"`
	ProfitShareRate     float64          `json:"profit_share_rate"`
	ProfitShareDue      float64          `json:"profit_share_due"`
	SettlementStatus    SettlementStatus `json:"settlement_status"`
	SettledAt           *time.Time       `json:"settled_at,omitempty"`
	StripeInvoiceID     string           `json:"stripe_invoice_id,omitempty"`
	CryptoTxHash        string           `json:"crypto_tx_hash,omitempty"`
	Notes               string           `json:"notes,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
}

// SnapshotType for balance snapshots
type SnapshotType string

const (
	SnapshotHourly     SnapshotType = "hourly"
	SnapshotDaily      SnapshotType = "daily"
	SnapshotWeekly     SnapshotType = "weekly"
	SnapshotTrade      SnapshotType = "trade"
	SnapshotDeposit    SnapshotType = "deposit"
	SnapshotWithdrawal SnapshotType = "withdrawal"
	SnapshotManual     SnapshotType = "manual"
)

// UserBalanceSnapshot represents a point-in-time balance snapshot
type UserBalanceSnapshot struct {
	ID               string       `json:"id"`
	UserID           string       `json:"user_id"`
	SnapshotType     SnapshotType `json:"snapshot_type"`
	Exchange         string       `json:"exchange"`
	SpotBalance      float64      `json:"spot_balance"`
	FuturesBalance   float64      `json:"futures_balance"`
	TotalBalance     float64      `json:"total_balance"`
	UnrealizedPnL    float64      `json:"unrealized_pnl"`
	MarginBalance    float64      `json:"margin_balance"`
	AvailableBalance float64      `json:"available_balance"`
	Source           string       `json:"source"`
	CreatedAt        time.Time    `json:"created_at"`
}

// AdjustmentType for balance adjustments
type AdjustmentType string

const (
	AdjustmentDeposit     AdjustmentType = "deposit"
	AdjustmentWithdrawal  AdjustmentType = "withdrawal"
	AdjustmentTransferIn  AdjustmentType = "transfer_in"
	AdjustmentTransferOut AdjustmentType = "transfer_out"
)

// UserBalanceAdjustment represents a deposit or withdrawal
type UserBalanceAdjustment struct {
	ID              string         `json:"id"`
	UserID          string         `json:"user_id"`
	AdjustmentType  AdjustmentType `json:"adjustment_type"`
	Amount          float64        `json:"amount"`
	Asset           string         `json:"asset"`
	TxID            string         `json:"tx_id,omitempty"`
	Source          string         `json:"source,omitempty"`
	DetectedAt      time.Time      `json:"detected_at"`
	ExcludedFromPnL bool           `json:"excluded_from_pnl"`
	CreatedAt       time.Time      `json:"created_at"`
}

// InvoiceType for invoices
type InvoiceType string

const (
	InvoiceSubscription InvoiceType = "subscription"
	InvoiceProfitShare  InvoiceType = "profit_share"
	InvoiceCombined     InvoiceType = "combined"
)

// InvoiceStatus for invoices
type InvoiceStatus string

const (
	InvoiceDraft     InvoiceStatus = "draft"
	InvoicePending   InvoiceStatus = "pending"
	InvoicePaid      InvoiceStatus = "paid"
	InvoiceFailed    InvoiceStatus = "failed"
	InvoiceCancelled InvoiceStatus = "cancelled"
	InvoiceRefunded  InvoiceStatus = "refunded"
)

// Invoice represents a billing invoice
type Invoice struct {
	ID                  string        `json:"id"`
	UserID              string        `json:"user_id"`
	InvoiceNumber       string        `json:"invoice_number"`
	InvoiceType         InvoiceType   `json:"invoice_type"`
	SubscriptionAmount  float64       `json:"subscription_amount"`
	ProfitShareAmount   float64       `json:"profit_share_amount"`
	TotalAmount         float64       `json:"total_amount"`
	Currency            string        `json:"currency"`
	Status              InvoiceStatus `json:"status"`
	StripeInvoiceID     string        `json:"stripe_invoice_id,omitempty"`
	StripePaymentIntent string        `json:"stripe_payment_intent,omitempty"`
	CryptoAddress       string        `json:"crypto_address,omitempty"`
	CryptoTxHash        string        `json:"crypto_tx_hash,omitempty"`
	PeriodStart         *time.Time    `json:"period_start,omitempty"`
	PeriodEnd           *time.Time    `json:"period_end,omitempty"`
	DueDate             *time.Time    `json:"due_date,omitempty"`
	PaidAt              *time.Time    `json:"paid_at,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
}

// TierConfig defines the limits and rates for each subscription tier
type TierConfig struct {
	Name            string  `json:"name"`
	MonthlyFeeCents int64   `json:"monthly_fee_cents"`
	ProfitShareRate float64 `json:"profit_share_rate"`
	MaxPositions    int     `json:"max_positions"`
	RateLimitPerMin int     `json:"rate_limit_per_min"`
	EnableFutures   bool    `json:"enable_futures"`
	EnableAutopilot bool    `json:"enable_autopilot"`
	MaxLeverage     int     `json:"max_leverage"`
	Priority        int     `json:"priority"` // Higher = faster execution
}

// TierConfigs defines all subscription tiers
var TierConfigs = map[SubscriptionTier]TierConfig{
	TierFree: {
		Name:            "Free",
		MonthlyFeeCents: 0,
		ProfitShareRate: 0.30,
		MaxPositions:    3,
		RateLimitPerMin: 10,
		EnableFutures:   false,
		EnableAutopilot: true,
		MaxLeverage:     1,
		Priority:        1,
	},
	TierTrader: {
		Name:            "Trader",
		MonthlyFeeCents: 4900, // $49
		ProfitShareRate: 0.20,
		MaxPositions:    10,
		RateLimitPerMin: 30,
		EnableFutures:   true,
		EnableAutopilot: true,
		MaxLeverage:     10,
		Priority:        2,
	},
	TierPro: {
		Name:            "Pro",
		MonthlyFeeCents: 14900, // $149
		ProfitShareRate: 0.12,
		MaxPositions:    25,
		RateLimitPerMin: 60,
		EnableFutures:   true,
		EnableAutopilot: true,
		MaxLeverage:     20,
		Priority:        3,
	},
	TierWhale: {
		Name:            "Whale",
		MonthlyFeeCents: 49900, // $499
		ProfitShareRate: 0.05,
		MaxPositions:    1000, // Effectively unlimited
		RateLimitPerMin: 120,
		EnableFutures:   true,
		EnableAutopilot: true,
		MaxLeverage:     50,
		Priority:        4,
	},
}

// GetTierConfig returns the configuration for a given tier
func GetTierConfig(tier SubscriptionTier) TierConfig {
	if config, ok := TierConfigs[tier]; ok {
		return config
	}
	return TierConfigs[TierFree]
}
