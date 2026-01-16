// Package settlement provides daily settlement and position snapshot services for Epic 8.
// It handles end-of-day position snapshots, P&L aggregation, and mode analytics.
package settlement

import (
	"time"
)

// PositionSnapshot represents a single position snapshot at end of day
type PositionSnapshot struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	SnapshotDate   time.Time `json:"snapshot_date"`   // Date in user's timezone
	Symbol         string    `json:"symbol"`          // Trading pair (e.g., BTCUSDT)
	PositionSide   string    `json:"position_side"`   // LONG, SHORT, or BOTH
	Quantity       float64   `json:"quantity"`        // Position size
	EntryPrice     float64   `json:"entry_price"`     // Average entry price
	MarkPrice      float64   `json:"mark_price"`      // Mark price at snapshot time
	UnrealizedPnL  float64   `json:"unrealized_pnl"`  // Unrealized P&L at snapshot
	Mode           string    `json:"mode"`            // Trading mode (scalp, swing, position, ultra_fast, UNKNOWN)
	ClientOrderID  string    `json:"client_order_id"` // Original clientOrderId if available
	Leverage       int       `json:"leverage"`        // Position leverage
	MarginType     string    `json:"margin_type"`     // CROSSED or ISOLATED
	CreatedAt      time.Time `json:"created_at"`
}

// SnapshotResult represents the result of a position snapshot operation for a user
type SnapshotResult struct {
	UserID         string             `json:"user_id"`
	SnapshotDate   time.Time          `json:"snapshot_date"`
	PositionCount  int                `json:"position_count"`  // Number of open positions snapshot
	Snapshots      []PositionSnapshot `json:"snapshots"`       // Individual position snapshots
	TotalUnrealizedPnL float64        `json:"total_unrealized_pnl"` // Sum of all unrealized P&L
	Success        bool               `json:"success"`
	Error          string             `json:"error,omitempty"` // Error message if failed
	Duration       time.Duration      `json:"duration"`        // Time taken to complete snapshot
}

// SettlementStatus represents the status of a user's daily settlement
type SettlementStatus struct {
	UserID              string     `json:"user_id"`
	Timezone            string     `json:"timezone"`
	LastSettlementDate  *time.Time `json:"last_settlement_date"`
	NextSettlementTime  time.Time  `json:"next_settlement_time"`
	NeedsSettlement     bool       `json:"needs_settlement"`
}

// ModeBreakdown represents P&L breakdown by trading mode
type ModeBreakdown struct {
	Mode           string  `json:"mode"`
	PositionCount  int     `json:"position_count"`
	UnrealizedPnL  float64 `json:"unrealized_pnl"`
}

// SnapshotSummary represents a summary of snapshots for a date
type SnapshotSummary struct {
	UserID             string          `json:"user_id"`
	SnapshotDate       time.Time       `json:"snapshot_date"`
	TotalPositions     int             `json:"total_positions"`
	TotalUnrealizedPnL float64         `json:"total_unrealized_pnl"`
	ModeBreakdowns     []ModeBreakdown `json:"mode_breakdowns"`
}

// Constants for mode values
const (
	ModeUnknown   = "UNKNOWN"
	ModeScalp     = "scalp"
	ModeSwing     = "swing"
	ModePosition  = "position"
	ModeUltraFast = "ultra_fast"
)

// ModeCodeToName maps 3-character mode codes to full mode names
var ModeCodeToName = map[string]string{
	"ULT": ModeUltraFast,
	"SCA": ModeScalp,
	"SWI": ModeSwing,
	"POS": ModePosition,
}

// ModeAll is the special mode name for aggregated totals across all modes
const ModeAll = "ALL"

// ModePnL represents daily P&L aggregation for a specific trading mode
type ModePnL struct {
	Mode         string  `json:"mode"`           // Trading mode (scalp, swing, position, ultra_fast, UNKNOWN, ALL)
	RealizedPnL  float64 `json:"realized_pnl"`   // Total realized P&L for the day
	TradeCount   int     `json:"trade_count"`    // Number of trades
	WinCount     int     `json:"win_count"`      // Number of winning trades (RealizedPnL > 0)
	LossCount    int     `json:"loss_count"`     // Number of losing trades (RealizedPnL < 0)
	WinRate      float64 `json:"win_rate"`       // Win rate as percentage (0-100)
	LargestWin   float64 `json:"largest_win"`    // Largest single winning trade P&L
	LargestLoss  float64 `json:"largest_loss"`   // Largest single losing trade P&L (negative value)
	TotalVolume  float64 `json:"total_volume"`   // Total trading volume in USDT
	AvgTradeSize float64 `json:"avg_trade_size"` // Average trade size (TotalVolume / TradeCount)
}

// DailyPnLAggregation represents the complete P&L aggregation for a user's trading day
type DailyPnLAggregation struct {
	UserID      string              `json:"user_id"`
	Date        time.Time           `json:"date"`          // Date in user's timezone
	ModeResults map[string]*ModePnL `json:"mode_results"`  // P&L breakdown by mode (including "ALL")
	TotalPnL    float64             `json:"total_pnl"`     // Sum of all realized P&L
	TotalTrades int                 `json:"total_trades"`  // Total trade count across all modes
	Duration    time.Duration       `json:"duration"`      // Time taken to compute aggregation
	Success     bool                `json:"success"`
	Error       string              `json:"error,omitempty"`
}
