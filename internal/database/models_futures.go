package database

import (
	"time"
)

// FuturesTrade represents a futures trading position in the database
type FuturesTrade struct {
	ID                  int64      `json:"id"`
	Symbol              string     `json:"symbol"`
	PositionSide        string     `json:"position_side"` // LONG, SHORT, BOTH
	Side                string     `json:"side"`          // BUY, SELL (entry side)
	EntryPrice          float64    `json:"entry_price"`
	ExitPrice           *float64   `json:"exit_price,omitempty"`
	MarkPrice           *float64   `json:"mark_price,omitempty"`
	Quantity            float64    `json:"quantity"`
	Leverage            int        `json:"leverage"`
	MarginType          string     `json:"margin_type"` // CROSSED, ISOLATED
	IsolatedMargin      *float64   `json:"isolated_margin,omitempty"`
	RealizedPnL         *float64   `json:"realized_pnl,omitempty"`
	UnrealizedPnL       *float64   `json:"unrealized_pnl,omitempty"`
	RealizedPnLPercent  *float64   `json:"realized_pnl_percent,omitempty"`
	LiquidationPrice    *float64   `json:"liquidation_price,omitempty"`
	StopLoss            *float64   `json:"stop_loss,omitempty"`
	TakeProfit          *float64   `json:"take_profit,omitempty"`
	TrailingStop        *float64   `json:"trailing_stop,omitempty"`
	Status              string     `json:"status"` // OPEN, CLOSED, LIQUIDATED
	EntryTime           time.Time  `json:"entry_time"`
	ExitTime            *time.Time `json:"exit_time,omitempty"`
	TradeSource         string     `json:"trade_source"` // manual, strategy, ai
	Notes               *string    `json:"notes,omitempty"`
	AIDecisionID        *int64     `json:"ai_decision_id,omitempty"`
	AIDecision          *AIDecision `json:"ai_decision,omitempty"`
	StrategyID          *int64     `json:"strategy_id,omitempty"`
	StrategyName        *string    `json:"strategy_name,omitempty"`
	TradingMode         *string    `json:"trading_mode,omitempty"` // ultra_fast, scalp, swing, position
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// FuturesOrder represents a futures order in the database
type FuturesOrder struct {
	ID              int64      `json:"id"`
	OrderID         int64      `json:"order_id"` // Binance order ID
	Symbol          string     `json:"symbol"`
	PositionSide    string     `json:"position_side"`
	Side            string     `json:"side"`
	OrderType       string     `json:"order_type"`
	Price           *float64   `json:"price,omitempty"`
	AvgPrice        *float64   `json:"avg_price,omitempty"`
	StopPrice       *float64   `json:"stop_price,omitempty"`
	Quantity        float64    `json:"quantity"`
	ExecutedQty     float64    `json:"executed_qty"`
	TimeInForce     string     `json:"time_in_force"`
	ReduceOnly      bool       `json:"reduce_only"`
	ClosePosition   bool       `json:"close_position"`
	WorkingType     string     `json:"working_type"`
	Status          string     `json:"status"`
	FuturesTradeID  *int64     `json:"futures_trade_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	FilledAt        *time.Time `json:"filled_at,omitempty"`
}

// FundingFee represents a funding fee record in the database
type FundingFee struct {
	ID          int64     `json:"id"`
	Symbol      string    `json:"symbol"`
	FundingRate float64   `json:"funding_rate"`
	FundingFee  float64   `json:"funding_fee"`
	PositionAmt float64   `json:"position_amt"`
	Asset       string    `json:"asset"`
	Timestamp   time.Time `json:"timestamp"`
	CreatedAt   time.Time `json:"created_at"`
}

// FuturesTransaction represents a transaction record for futures
type FuturesTransaction struct {
	ID              int64     `json:"id"`
	TransactionID   int64     `json:"transaction_id"` // Binance transaction ID
	Symbol          string    `json:"symbol"`
	IncomeType      string    `json:"income_type"` // REALIZED_PNL, FUNDING_FEE, COMMISSION, etc.
	Income          float64   `json:"income"`
	Asset           string    `json:"asset"`
	Info            *string   `json:"info,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	FuturesTradeID  *int64    `json:"futures_trade_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// FuturesAccountSettings represents account settings per symbol
type FuturesAccountSettings struct {
	ID           int64     `json:"id"`
	Symbol       string    `json:"symbol"`
	Leverage     int       `json:"leverage"`
	MarginType   string    `json:"margin_type"`
	PositionMode string    `json:"position_mode"` // ONE_WAY, HEDGE
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// FuturesTradingMetrics represents aggregated futures trading metrics
type FuturesTradingMetrics struct {
	TotalTrades       int        `json:"total_trades"`
	WinningTrades     int        `json:"winning_trades"`
	LosingTrades      int        `json:"losing_trades"`
	WinRate           float64    `json:"win_rate"`
	TotalRealizedPnL  float64    `json:"total_realized_pnl"`
	TotalUnrealizedPnL float64   `json:"total_unrealized_pnl"`
	TotalFundingFees  float64    `json:"total_funding_fees"`
	AveragePnL        float64    `json:"average_pnl"`
	AverageWin        float64    `json:"average_win"`
	AverageLoss       float64    `json:"average_loss"`
	LargestWin        float64    `json:"largest_win"`
	LargestLoss       float64    `json:"largest_loss"`
	ProfitFactor      float64    `json:"profit_factor"`
	AverageLeverage   float64    `json:"average_leverage"`
	OpenPositions     int        `json:"open_positions"`
	OpenOrders        int        `json:"open_orders"`
	LastTradeTime     *time.Time `json:"last_trade_time,omitempty"`
}

// ModeSafetyHistory tracks safety control events for modes
type ModeSafetyHistory struct {
	ID               int64      `json:"id"`
	Mode             string     `json:"mode"` // ultra_fast, scalp, swing, position
	EventType        string     `json:"event_type"` // paused, resumed, threshold_triggered
	TriggerReason    *string    `json:"trigger_reason,omitempty"`
	WinRate          *float64   `json:"win_rate,omitempty"`
	ProfitWindowPct  *float64   `json:"profit_window_pct,omitempty"`
	TradesPerMinute  *int       `json:"trades_per_minute,omitempty"`
	PauseDurationMins *int      `json:"pause_duration_mins,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// ModeAllocationHistory tracks capital allocation changes for modes
type ModeAllocationHistory struct {
	ID               int64     `json:"id"`
	Mode             string    `json:"mode"`
	AllocatedPercent float64   `json:"allocated_percent"`
	AllocatedUSD     float64   `json:"allocated_usd"`
	UsedUSD          float64   `json:"used_usd"`
	AvailableUSD     float64   `json:"available_usd"`
	CurrentPositions int       `json:"current_positions"`
	MaxPositions     int       `json:"max_positions"`
	CapacityPercent  float64   `json:"capacity_percent"`
	CreatedAt        time.Time `json:"created_at"`
}

// ModePerformanceStats tracks aggregated performance metrics per trading mode
type ModePerformanceStats struct {
	ID                int64     `json:"id"`
	Mode              string    `json:"mode"`
	TotalTrades       int       `json:"total_trades"`
	WinningTrades     int       `json:"winning_trades"`
	LosingTrades      int       `json:"losing_trades"`
	WinRate           float64   `json:"win_rate"`
	TotalPnLUSD       float64   `json:"total_pnl_usd"`
	TotalPnLPercent   float64   `json:"total_pnl_percent"`
	AvgPnLPerTrade    float64   `json:"avg_pnl_per_trade"`
	MaxDrawdownUSD    float64   `json:"max_drawdown_usd"`
	MaxDrawdownPercent float64  `json:"max_drawdown_percent"`
	AvgHoldSeconds    int       `json:"avg_hold_seconds"`
	LastTradeTime     *time.Time `json:"last_trade_time,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}
