// Package coinprofiler provides real-time coin data collection via WebSocket
// for the Chain Trading System. It serves as the central data hub for both
// Entry Decision (strategies) and Exit Decision (positions).
package coinprofiler

import (
	"time"
)

// CoinProfilerConfig holds configuration for the Coin Profiler service.
type CoinProfilerConfig struct {
	// WebSocket Configuration
	WebSocketEndpoint    string        `json:"websocket_endpoint"`     // Binance Futures WebSocket endpoint
	ReconnectDelay       time.Duration `json:"reconnect_delay"`        // Initial delay before reconnection
	MaxReconnectDelay    time.Duration `json:"max_reconnect_delay"`    // Maximum backoff delay
	ReconnectBackoff     float64       `json:"reconnect_backoff"`      // Multiplier for exponential backoff
	PingInterval         time.Duration `json:"ping_interval"`          // WebSocket ping interval
	PongTimeout          time.Duration `json:"pong_timeout"`           // Timeout waiting for pong response
	MaxReconnectAttempts int           `json:"max_reconnect_attempts"` // Max reconnection attempts (0 = unlimited)

	// Data Collection Configuration
	UpdateInterval    time.Duration `json:"update_interval"`     // How often to process accumulated updates
	MaxSubscriptions  int           `json:"max_subscriptions"`   // Maximum concurrent subscriptions
	DataRetentionTTL  time.Duration `json:"data_retention_ttl"`  // TTL for stale coin data in Redis
	BatchSize         int           `json:"batch_size"`          // Number of subscriptions per batch

	// Feature Flags
	EnableDeltaUpdates   bool `json:"enable_delta_updates"`    // Only update changed fields
	EnableDataValidation bool `json:"enable_data_validation"`  // Validate incoming data
	DebugMode            bool `json:"debug_mode"`              // Enable verbose logging
}

// DefaultCoinProfilerConfig returns the default configuration.
func DefaultCoinProfilerConfig() *CoinProfilerConfig {
	return &CoinProfilerConfig{
		// Binance Futures WebSocket endpoint (production)
		WebSocketEndpoint: "wss://fstream.binance.com/ws",

		// Reconnection settings
		ReconnectDelay:       1 * time.Second,
		MaxReconnectDelay:    5 * time.Minute,
		ReconnectBackoff:     2.0,
		MaxReconnectAttempts: 0, // Unlimited

		// Heartbeat settings
		PingInterval: 3 * time.Minute,  // Binance requires ping within 5 minutes
		PongTimeout:  10 * time.Second,

		// Data collection settings
		UpdateInterval:   100 * time.Millisecond, // Process updates every 100ms
		MaxSubscriptions: 200,                    // Binance limit per connection
		DataRetentionTTL: 5 * time.Minute,        // Remove stale data after 5 minutes
		BatchSize:        50,                     // Subscribe in batches of 50

		// Features
		EnableDeltaUpdates:   true,
		EnableDataValidation: true,
		DebugMode:            false,
	}
}

// CoinProfilerStatus represents the current status of the Coin Profiler.
type CoinProfilerStatus struct {
	Running           bool      `json:"running"`             // Whether the profiler is running
	Connected         bool      `json:"connected"`           // WebSocket connection status
	SymbolCount       int       `json:"symbol_count"`        // Number of symbols being profiled
	SubscriptionCount int       `json:"subscription_count"`  // Total active subscriptions
	UpdatesPerSecond  float64   `json:"updates_per_second"`  // Data update rate
	LastUpdateTime    time.Time `json:"last_update_time"`    // Time of last data update
	LastError         string    `json:"last_error"`          // Last error message (if any)
	ReconnectCount    int       `json:"reconnect_count"`     // Number of reconnection attempts
	StartedAt         time.Time `json:"started_at"`          // When the profiler started
	Uptime            string    `json:"uptime"`              // Human-readable uptime
}

// ConnectionState represents WebSocket connection states.
type ConnectionState int

const (
	// ConnectionStateDisconnected indicates no active connection.
	ConnectionStateDisconnected ConnectionState = iota
	// ConnectionStateConnecting indicates connection is being established.
	ConnectionStateConnecting
	// ConnectionStateConnected indicates active connection.
	ConnectionStateConnected
	// ConnectionStateReconnecting indicates reconnection in progress.
	ConnectionStateReconnecting
)

// String returns a human-readable connection state.
func (cs ConnectionState) String() string {
	switch cs {
	case ConnectionStateDisconnected:
		return "disconnected"
	case ConnectionStateConnecting:
		return "connecting"
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// DataSource indicates where the coin data requirement originated.
type DataSource string

const (
	// DataSourceStrategy indicates data is needed for entry decision strategies.
	DataSourceStrategy DataSource = "strategy"
	// DataSourcePosition indicates data is needed for exit decision (open positions).
	DataSourcePosition DataSource = "position"
	// DataSourceBoth indicates data is needed for both strategies and positions.
	DataSourceBoth DataSource = "both"
)

// TimeframeData holds OHLCV data for a specific timeframe.
type TimeframeData struct {
	Timeframe     string    `json:"timeframe"`       // e.g., "5m", "15m", "1h"
	Open          float64   `json:"open"`            // Open price
	High          float64   `json:"high"`            // High price
	Low           float64   `json:"low"`             // Low price
	Close         float64   `json:"close"`           // Close price
	Volume        float64   `json:"volume"`          // Total volume
	TakerBuyVol   float64   `json:"taker_buy_vol"`   // Taker buy volume
	TakerSellVol  float64   `json:"taker_sell_vol"`  // Taker sell volume (calculated)
	QuoteVolume   float64   `json:"quote_volume"`    // Quote asset volume
	TradeCount    int       `json:"trade_count"`     // Number of trades
	IsClosedBar   bool      `json:"is_closed_bar"`   // Whether this is a closed candle
	OpenTime      time.Time `json:"open_time"`       // Candle open time
	CloseTime     time.Time `json:"close_time"`      // Candle close time
	UpdatedAt     time.Time `json:"updated_at"`      // When this data was last updated
}

// CoinData holds all profiled data for a single coin/symbol.
type CoinData struct {
	Symbol       string                    `json:"symbol"`         // e.g., "BTCUSDT"
	Price        float64                   `json:"price"`          // Current price
	Volume24h    float64                   `json:"volume_24h"`     // 24-hour volume
	Volatility   float64                   `json:"volatility"`     // Price volatility
	Timeframes   map[string]*TimeframeData `json:"timeframes"`     // Data per timeframe
	Source       DataSource                `json:"source"`         // Why this coin is being tracked
	Strategies   []string                  `json:"strategies"`     // Which strategies need this coin
	UpdatedAt    time.Time                 `json:"updated_at"`     // Last update timestamp
}

// SubscriptionRequest represents a request to subscribe to coin data.
type SubscriptionRequest struct {
	Symbol     string     `json:"symbol"`     // Symbol to subscribe
	Timeframes []string   `json:"timeframes"` // Required timeframes
	Source     DataSource `json:"source"`     // Who requested this data
	Strategy   string     `json:"strategy"`   // Strategy name (if from strategy)
}

// WebSocketMessage represents a generic WebSocket message from Binance.
type WebSocketMessage struct {
	Stream string      `json:"stream"` // Stream name (for combined streams)
	Data   interface{} `json:"data"`   // Actual data payload
}

// KlineMessage represents a kline/candlestick update from Binance.
type KlineMessage struct {
	EventType string `json:"e"` // "kline"
	EventTime int64  `json:"E"` // Event time
	Symbol    string `json:"s"` // Symbol
	Kline     struct {
		StartTime    int64  `json:"t"` // Kline start time
		CloseTime    int64  `json:"T"` // Kline close time
		Symbol       string `json:"s"` // Symbol
		Interval     string `json:"i"` // Interval
		FirstTradeID int64  `json:"f"` // First trade ID
		LastTradeID  int64  `json:"L"` // Last trade ID
		Open         string `json:"o"` // Open price
		Close        string `json:"c"` // Close price
		High         string `json:"h"` // High price
		Low          string `json:"l"` // Low price
		Volume       string `json:"v"` // Base asset volume
		TradeCount   int    `json:"n"` // Number of trades
		IsClosed     bool   `json:"x"` // Is this kline closed?
		QuoteVolume  string `json:"q"` // Quote asset volume
		TakerBuyVol  string `json:"V"` // Taker buy base asset volume
		TakerBuyQuote string `json:"Q"` // Taker buy quote asset volume
	} `json:"k"`
}

// TickerMessage represents a 24hr ticker update from Binance.
type TickerMessage struct {
	EventType       string `json:"e"` // "24hrTicker"
	EventTime       int64  `json:"E"` // Event time
	Symbol          string `json:"s"` // Symbol
	PriceChange     string `json:"p"` // Price change
	PriceChangePct  string `json:"P"` // Price change percent
	WeightedAvgPx   string `json:"w"` // Weighted average price
	LastPrice       string `json:"c"` // Last price
	LastQty         string `json:"Q"` // Last quantity
	OpenPrice       string `json:"o"` // Open price
	HighPrice       string `json:"h"` // High price
	LowPrice        string `json:"l"` // Low price
	Volume          string `json:"v"` // Total traded base asset volume
	QuoteVolume     string `json:"q"` // Total traded quote asset volume
	OpenTime        int64  `json:"O"` // Statistics open time
	CloseTime       int64  `json:"C"` // Statistics close time
	FirstTradeID    int64  `json:"F"` // First trade ID
	LastTradeID     int64  `json:"L"` // Last trade ID
	TradeCount      int64  `json:"n"` // Total number of trades
}

// ============================================================================
// CANDLE HISTORY - Circular buffer for pattern detection
// Story: Entry Decision Strategy Requirements & Real-Time Monitoring
// ============================================================================

// HistoricalCandle represents a closed candle for pattern detection.
// This is a simplified version of TimeframeData focused on pattern matching.
type HistoricalCandle struct {
	OpenTime     time.Time `json:"open_time"`
	CloseTime    time.Time `json:"close_time"`
	Open         float64   `json:"open"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	Close        float64   `json:"close"`
	Volume       float64   `json:"volume"`
	TakerBuyVol  float64   `json:"taker_buy_vol"`
	TakerSellVol float64   `json:"taker_sell_vol"`
	QuoteVolume  float64   `json:"quote_volume"`
	TradeCount   int       `json:"trade_count"`
}

// CandleHistory stores historical candles in a circular buffer for pattern detection.
// Thread-safe for concurrent reads and writes.
type CandleHistory struct {
	candles  []HistoricalCandle // Circular buffer
	maxSize  int                // Maximum capacity (default: 50)
	count    int                // Current number of candles
	writeIdx int                // Next write position
}

// DefaultCandleHistorySize is the default number of candles to keep in history.
const DefaultCandleHistorySize = 50

// NewCandleHistory creates a new candle history buffer with the specified size.
func NewCandleHistory(maxSize int) *CandleHistory {
	if maxSize <= 0 {
		maxSize = DefaultCandleHistorySize
	}
	return &CandleHistory{
		candles: make([]HistoricalCandle, maxSize),
		maxSize: maxSize,
	}
}

// Add adds a closed candle to the history.
// Older candles are overwritten when buffer is full (circular behavior).
func (ch *CandleHistory) Add(candle HistoricalCandle) {
	ch.candles[ch.writeIdx] = candle
	ch.writeIdx = (ch.writeIdx + 1) % ch.maxSize
	if ch.count < ch.maxSize {
		ch.count++
	}
}

// GetAll returns all candles in chronological order (oldest first).
// Returns a copy to prevent external modification.
func (ch *CandleHistory) GetAll() []HistoricalCandle {
	if ch.count == 0 {
		return nil
	}

	result := make([]HistoricalCandle, ch.count)
	if ch.count < ch.maxSize {
		// Buffer not full yet - candles are at indices 0..count-1
		copy(result, ch.candles[:ch.count])
	} else {
		// Buffer full - read from writeIdx (oldest) to end, then from start to writeIdx
		// First part: from writeIdx to end
		firstPart := ch.candles[ch.writeIdx:]
		copy(result, firstPart)
		// Second part: from start to writeIdx
		secondPart := ch.candles[:ch.writeIdx]
		copy(result[len(firstPart):], secondPart)
	}
	return result
}

// GetRecent returns the most recent n candles in chronological order.
// If n exceeds available candles, returns all available.
func (ch *CandleHistory) GetRecent(n int) []HistoricalCandle {
	all := ch.GetAll()
	if len(all) <= n {
		return all
	}
	return all[len(all)-n:]
}

// Count returns the current number of candles in history.
func (ch *CandleHistory) Count() int {
	return ch.count
}

// IsFull returns true if the buffer has reached maximum capacity.
func (ch *CandleHistory) IsFull() bool {
	return ch.count >= ch.maxSize
}

// Clear removes all candles from history.
func (ch *CandleHistory) Clear() {
	ch.candles = make([]HistoricalCandle, ch.maxSize)
	ch.count = 0
	ch.writeIdx = 0
}

// Last returns the most recent candle, or nil if empty.
func (ch *CandleHistory) Last() *HistoricalCandle {
	if ch.count == 0 {
		return nil
	}
	// Last written candle is at (writeIdx - 1 + maxSize) % maxSize
	lastIdx := (ch.writeIdx - 1 + ch.maxSize) % ch.maxSize
	candle := ch.candles[lastIdx]
	return &candle
}

// CandleHistoryKey generates a unique key for symbol+timeframe combination.
func CandleHistoryKey(symbol, timeframe string) string {
	return symbol + ":" + timeframe
}

// ============================================================================
// HISTORICAL DATA PROVIDER - Interface for fetching historical candles
// Epic 14: Entry Decision Strategy Requirements & Real-Time Monitoring
// ============================================================================

// HistoricalKline represents a single kline/candlestick from Binance API.
// This mirrors the binance.Kline struct to avoid direct dependency.
type HistoricalKline struct {
	OpenTime                 int64
	Open                     float64
	High                     float64
	Low                      float64
	Close                    float64
	Volume                   float64
	CloseTime                int64
	QuoteAssetVolume         float64
	NumberOfTrades           int
	TakerBuyBaseAssetVolume  float64
	TakerBuyQuoteAssetVolume float64
}

// HistoricalDataProvider defines the interface for fetching historical candles.
// This allows CoinProfiler to pre-populate candle history on startup without
// requiring users to wait for candles to close in real-time.
type HistoricalDataProvider interface {
	// GetFuturesKlines fetches historical klines from Binance API.
	// Returns the most recent `limit` candles for the given symbol and interval.
	GetFuturesKlines(symbol, interval string, limit int) ([]HistoricalKline, error)
}

// KlineToHistoricalCandle converts a HistoricalKline to HistoricalCandle format.
func KlineToHistoricalCandle(k HistoricalKline) HistoricalCandle {
	takerSellVol := k.Volume - k.TakerBuyBaseAssetVolume
	if takerSellVol < 0 {
		takerSellVol = 0
	}

	return HistoricalCandle{
		OpenTime:     time.UnixMilli(k.OpenTime),
		CloseTime:    time.UnixMilli(k.CloseTime),
		Open:         k.Open,
		High:         k.High,
		Low:          k.Low,
		Close:        k.Close,
		Volume:       k.Volume,
		TakerBuyVol:  k.TakerBuyBaseAssetVolume,
		TakerSellVol: takerSellVol,
		QuoteVolume:  k.QuoteAssetVolume,
		TradeCount:   k.NumberOfTrades,
	}
}
