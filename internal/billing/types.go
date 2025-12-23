package billing

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

// TierLimits defines the limits for each subscription tier
type TierLimits struct {
	MaxPositions      int
	MaxDailyTrades    int
	EnableFutures     bool
	MaxLeverage       int
	PrioritySupport   bool
	DedicatedAgent    bool
}

// GetTierLimits returns the limits for a given tier
func GetTierLimits(tier SubscriptionTier) TierLimits {
	switch tier {
	case TierFree:
		return TierLimits{
			MaxPositions:   3,
			MaxDailyTrades: 10,
			EnableFutures:  false,
			MaxLeverage:    1,
		}
	case TierTrader:
		return TierLimits{
			MaxPositions:   10,
			MaxDailyTrades: 50,
			EnableFutures:  true,
			MaxLeverage:    10,
		}
	case TierPro:
		return TierLimits{
			MaxPositions:     25,
			MaxDailyTrades:   200,
			EnableFutures:    true,
			MaxLeverage:      50,
			PrioritySupport:  true,
		}
	case TierWhale:
		return TierLimits{
			MaxPositions:    -1, // Unlimited
			MaxDailyTrades:  -1, // Unlimited
			EnableFutures:   true,
			MaxLeverage:     125,
			PrioritySupport: true,
			DedicatedAgent:  true,
		}
	default:
		return GetTierLimits(TierFree)
	}
}

// GetProfitShareRate returns the profit share rate for a tier
func GetProfitShareRate(tier SubscriptionTier) float64 {
	switch tier {
	case TierFree:
		return 0.30 // 30%
	case TierTrader:
		return 0.20 // 20%
	case TierPro:
		return 0.12 // 12%
	case TierWhale:
		return 0.05 // 5%
	default:
		return 0.30
	}
}

// GetMonthlyFee returns the monthly subscription fee for a tier
func GetMonthlyFee(tier SubscriptionTier) float64 {
	switch tier {
	case TierFree:
		return 0
	case TierTrader:
		return 49.0
	case TierPro:
		return 149.0
	case TierWhale:
		return 499.0
	default:
		return 0
	}
}

// ProfitPeriod is an alias for database.ProfitPeriod
// Moved to database package to avoid import cycle
// Use database.ProfitPeriod directly

// SettlementStatus represents the status of a profit period settlement
type SettlementStatus string

const (
	StatusPending   SettlementStatus = "pending"
	StatusInvoiced  SettlementStatus = "invoiced"
	StatusPaid      SettlementStatus = "paid"
	StatusFailed    SettlementStatus = "failed"
	StatusWaived    SettlementStatus = "waived"
)

// ProfitReport represents the calculated profit for a period
type ProfitReport struct {
	UserID              string    `json:"user_id"`
	PeriodStart         time.Time `json:"period_start"`
	PeriodEnd           time.Time `json:"period_end"`
	StartingBalance     float64   `json:"starting_balance"`
	EndingBalance       float64   `json:"ending_balance"`
	TotalDeposits       float64   `json:"total_deposits"`
	TotalWithdrawals    float64   `json:"total_withdrawals"`
	GrossProfit         float64   `json:"gross_profit"`
	PreviousLossCarry   float64   `json:"previous_loss_carry"`
	NetProfit           float64   `json:"net_profit"`
	NewHighWaterMark    float64   `json:"new_high_water_mark"`
	ProfitAboveHWM      float64   `json:"profit_above_hwm"` // Profit above high-water mark
	NewLossCarryforward float64   `json:"new_loss_carryforward"`
	ProfitShareRate     float64   `json:"profit_share_rate"`
	ProfitShareDue      float64   `json:"profit_share_due"`
	TotalTrades         int       `json:"total_trades"`
	WinningTrades       int       `json:"winning_trades"`
	LosingTrades        int       `json:"losing_trades"`
	WinRate             float64   `json:"win_rate"`
}

// BalanceSnapshot, Transaction, Invoice types moved to database package
// to avoid import cycle. Use database.BalanceSnapshot, database.Transaction, database.Invoice directly

// BillingConfig holds billing configuration
type BillingConfig struct {
	SettlementDayOfWeek int     // 0 = Sunday
	SettlementHourUTC   int     // Hour to run settlement
	MinimumPayout       float64 // Minimum profit share to invoice
	GracePeriodDays     int     // Days before late fees
	LateFeePercent      float64 // Late fee percentage
}

// DefaultBillingConfig returns default billing configuration
func DefaultBillingConfig() *BillingConfig {
	return &BillingConfig{
		SettlementDayOfWeek: 0,    // Sunday
		SettlementHourUTC:   0,    // Midnight UTC
		MinimumPayout:       10.0, // $10 minimum
		GracePeriodDays:     7,
		LateFeePercent:      5.0,
	}
}
