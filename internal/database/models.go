package database

import (
	"time"
)

// TradeSource constants
const (
	TradeSourceManual   = "manual"   // User-initiated trade
	TradeSourceStrategy = "strategy" // Automated strategy trade
	TradeSourceAI       = "ai"       // AI autopilot trade
)

// Trade represents a trading position in the database
type Trade struct {
	ID           int64     `json:"id"`
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`
	EntryPrice   float64   `json:"entry_price"`
	ExitPrice    *float64  `json:"exit_price,omitempty"`
	Quantity     float64   `json:"quantity"`
	EntryTime    time.Time `json:"entry_time"`
	ExitTime     *time.Time `json:"exit_time,omitempty"`
	StopLoss     *float64  `json:"stop_loss,omitempty"`
	TakeProfit   *float64  `json:"take_profit,omitempty"`
	PnL          *float64  `json:"pnl,omitempty"`
	PnLPercent   *float64  `json:"pnl_percent,omitempty"`
	StrategyName *string   `json:"strategy_name,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// Trade source: "manual", "strategy", or "ai"
	TradeSource  string    `json:"trade_source"`
	// AI Decision linking
	AIDecisionID        *int64   `json:"ai_decision_id,omitempty"`
	AIDecision          *AIDecision `json:"ai_decision,omitempty"`
	// Trailing stop fields
	TrailingStopEnabled bool     `json:"trailing_stop_enabled"`
	TrailingStopPercent *float64 `json:"trailing_stop_percent,omitempty"`
	HighestPrice        *float64 `json:"highest_price,omitempty"`
	LowestPrice         *float64 `json:"lowest_price,omitempty"`
	// Order IDs for TP/SL
	TakeProfitOrderID   *int64   `json:"take_profit_order_id,omitempty"`
	StopLossOrderID     *int64   `json:"stop_loss_order_id,omitempty"`
}

// Order represents an order in the database
type Order struct {
	ID          int64      `json:"id"`
	Symbol      string     `json:"symbol"`
	OrderType   string     `json:"order_type"`
	Side        string     `json:"side"`
	Price       *float64   `json:"price,omitempty"`
	Quantity    float64    `json:"quantity"`
	ExecutedQty float64    `json:"executed_qty"`
	Status      string     `json:"status"`
	TimeInForce *string    `json:"time_in_force,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	FilledAt    *time.Time `json:"filled_at,omitempty"`
	TradeID     *int64     `json:"trade_id,omitempty"`
}

// Signal represents a trading signal in the database
type Signal struct {
	ID           int64     `json:"id"`
	StrategyName string    `json:"strategy_name"`
	Symbol       string    `json:"symbol"`
	SignalType   string    `json:"signal_type"`
	EntryPrice   float64   `json:"entry_price"`
	StopLoss     *float64  `json:"stop_loss,omitempty"`
	TakeProfit   *float64  `json:"take_profit,omitempty"`
	Quantity     *float64  `json:"quantity,omitempty"`
	Reason       *string   `json:"reason,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	Executed     bool      `json:"executed"`
	CreatedAt    time.Time `json:"created_at"`
}

// PositionSnapshot represents a position snapshot in the database
type PositionSnapshot struct {
	ID           int64     `json:"id"`
	Symbol       string    `json:"symbol"`
	EntryPrice   float64   `json:"entry_price"`
	CurrentPrice float64   `json:"current_price"`
	Quantity     float64   `json:"quantity"`
	PnL          float64   `json:"pnl"`
	PnLPercent   float64   `json:"pnl_percent"`
	Timestamp    time.Time `json:"timestamp"`
	CreatedAt    time.Time `json:"created_at"`
}

// ScreenerResult represents a market screening result in the database
type ScreenerResult struct {
	ID                 int64     `json:"id"`
	Symbol             string    `json:"symbol"`
	LastPrice          float64   `json:"last_price"`
	PriceChangePercent *float64  `json:"price_change_percent,omitempty"`
	Volume             *float64  `json:"volume,omitempty"`
	QuoteVolume        *float64  `json:"quote_volume,omitempty"`
	High24h            *float64  `json:"high_24h,omitempty"`
	Low24h             *float64  `json:"low_24h,omitempty"`
	Signals            []string  `json:"signals"`
	Timestamp          time.Time `json:"timestamp"`
	CreatedAt          time.Time `json:"created_at"`
}

// SystemEvent represents a system event in the database
type SystemEvent struct {
	ID        int64                  `json:"id"`
	EventType string                 `json:"event_type"`
	Source    *string                `json:"source,omitempty"`
	Message   *string                `json:"message,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	CreatedAt time.Time              `json:"created_at"`
}

// TradingMetrics represents aggregated trading metrics
type TradingMetrics struct {
	TotalTrades       int     `json:"total_trades"`
	WinningTrades     int     `json:"winning_trades"`
	LosingTrades      int     `json:"losing_trades"`
	WinRate           float64 `json:"win_rate"`
	TotalPnL          float64 `json:"total_pnl"`
	AveragePnL        float64 `json:"average_pnl"`
	AverageWin        float64 `json:"average_win"`
	AverageLoss       float64 `json:"average_loss"`
	LargestWin        float64 `json:"largest_win"`
	LargestLoss       float64 `json:"largest_loss"`
	ProfitFactor      float64 `json:"profit_factor"`
	OpenPositions     int     `json:"open_positions"`
	ActiveOrders      int     `json:"active_orders"`
	TotalSignals      int     `json:"total_signals"`
	ExecutedSignals   int     `json:"executed_signals"`
	LastTradeTime     *time.Time `json:"last_trade_time,omitempty"`
}

// StrategyConfig represents a user-configurable trading strategy
type StrategyConfig struct {
	ID                int64                  `json:"id"`
	Name              string                 `json:"name"`
	Symbol            string                 `json:"symbol"`
	Timeframe         string                 `json:"timeframe"`
	IndicatorType     string                 `json:"indicator_type"`
	Autopilot         bool                   `json:"autopilot"`
	Enabled           bool                   `json:"enabled"`
	PositionSize      float64                `json:"position_size"`
	StopLossPercent   float64                `json:"stop_loss_percent"`
	TakeProfitPercent float64                `json:"take_profit_percent"`
	ConfigParams      map[string]interface{} `json:"config_params,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// PendingSignal represents a trading signal awaiting manual confirmation
type PendingSignal struct {
	ID            int64                  `json:"id"`
	StrategyName  string                 `json:"strategy_name"`
	Symbol        string                 `json:"symbol"`
	SignalType    string                 `json:"signal_type"`
	EntryPrice    float64                `json:"entry_price"`
	CurrentPrice  float64                `json:"current_price"`
	StopLoss      *float64               `json:"stop_loss,omitempty"`
	TakeProfit    *float64               `json:"take_profit,omitempty"`
	Quantity      *float64               `json:"quantity,omitempty"`
	Reason        *string                `json:"reason,omitempty"`
	ConditionsMet map[string]interface{} `json:"conditions_met"`
	Timestamp     time.Time              `json:"timestamp"`
	Status        string                 `json:"status"`
	ConfirmedAt   *time.Time             `json:"confirmed_at,omitempty"`
	RejectedAt    *time.Time             `json:"rejected_at,omitempty"`
	Archived      bool                   `json:"archived"`
	ArchivedAt    *time.Time             `json:"archived_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// SignalCondition represents a condition that was checked for a signal
type SignalCondition struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Met         bool    `json:"met"`
	Value       *string `json:"value,omitempty"`
}

// WatchlistItem represents a user's favorite symbol for monitoring
type WatchlistItem struct {
	ID      int64      `json:"id"`
	Symbol  string     `json:"symbol"`
	Notes   *string    `json:"notes,omitempty"`
	AddedAt time.Time  `json:"added_at"`
	CreatedAt time.Time `json:"created_at"`
}

// AIDecision represents an autopilot AI decision with all signal sources
type AIDecision struct {
	ID                  int64                  `json:"id"`
	Symbol              string                 `json:"symbol"`
	CurrentPrice        float64                `json:"current_price"`
	Action              string                 `json:"action"`
	Confidence          float64                `json:"confidence"`
	Reasoning           string                 `json:"reasoning"`
	Signals             map[string]interface{} `json:"signals"`
	MLDirection         *string                `json:"ml_direction,omitempty"`
	MLConfidence        *float64               `json:"ml_confidence,omitempty"`
	SentimentDirection  *string                `json:"sentiment_direction,omitempty"`
	SentimentConfidence *float64               `json:"sentiment_confidence,omitempty"`
	LLMDirection        *string                `json:"llm_direction,omitempty"`
	LLMConfidence       *float64               `json:"llm_confidence,omitempty"`
	PatternDirection    *string                `json:"pattern_direction,omitempty"`
	PatternConfidence   *float64               `json:"pattern_confidence,omitempty"`
	BigCandleDirection  *string                `json:"bigcandle_direction,omitempty"`
	BigCandleConfidence *float64               `json:"bigcandle_confidence,omitempty"`
	ConfluenceCount     int                    `json:"confluence_count"`
	RiskLevel           string                 `json:"risk_level"`
	Executed            bool                   `json:"executed"`
	CreatedAt           time.Time              `json:"created_at"`
}
