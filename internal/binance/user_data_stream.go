package binance

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// UserDataStream handles the Binance Futures User Data WebSocket stream
// This provides real-time updates for account, positions, and orders
// eliminating the need for REST API polling
type UserDataStream struct {
	mu sync.RWMutex

	client    FuturesClient
	listenKey string
	wsConn    *websocket.Conn
	isRunning bool
	stopChan  chan struct{}

	// Callbacks for different event types
	onAccountUpdate  func(*AccountUpdateEvent)
	onOrderUpdate    func(*OrderUpdateEvent)
	onPositionUpdate func(*PositionUpdateEvent)

	// Cached data from stream
	positions      map[string]*StreamPosition
	orders         map[int64]*StreamOrder
	accountBalance float64
	lastUpdateTime time.Time

	// Configuration
	baseURL    string
	isTestnet  bool
	reconnects int
}

// AccountUpdateEvent represents a ACCOUNT_UPDATE event from the stream
type AccountUpdateEvent struct {
	EventType       string                `json:"e"`
	EventTime       int64                 `json:"E"`
	TransactionTime int64                 `json:"T"`
	AccountUpdate   AccountUpdateData     `json:"a"`
}

type AccountUpdateData struct {
	EventReasonType string            `json:"m"` // DEPOSIT, WITHDRAW, ORDER, FUNDING_FEE, etc.
	Balances        []BalanceUpdate   `json:"B"`
	Positions       []PositionUpdate  `json:"P"`
}

type BalanceUpdate struct {
	Asset              string  `json:"a"`
	WalletBalance      float64 `json:"wb,string"`
	CrossWalletBalance float64 `json:"cw,string"`
	BalanceChange      float64 `json:"bc,string"`
}

type PositionUpdate struct {
	Symbol           string  `json:"s"`
	PositionAmount   float64 `json:"pa,string"`
	EntryPrice       float64 `json:"ep,string"`
	AccumulatedPnL   float64 `json:"cr,string"` // (Pre-fee) Accumulated Realized
	UnrealizedPnL    float64 `json:"up,string"`
	MarginType       string  `json:"mt"` // ISOLATED, CROSSED
	IsolatedWallet   float64 `json:"iw,string"`
	PositionSide     string  `json:"ps"` // BOTH, LONG, SHORT
}

// OrderUpdateEvent represents an ORDER_TRADE_UPDATE event from the stream
type OrderUpdateEvent struct {
	EventType       string          `json:"e"`
	EventTime       int64           `json:"E"`
	TransactionTime int64           `json:"T"`
	Order           OrderUpdateData `json:"o"`
}

type OrderUpdateData struct {
	Symbol              string  `json:"s"`
	ClientOrderId       string  `json:"c"`
	Side                string  `json:"S"`      // BUY, SELL
	OrderType           string  `json:"o"`      // MARKET, LIMIT, STOP, etc.
	TimeInForce         string  `json:"f"`
	OriginalQuantity    float64 `json:"q,string"`
	OriginalPrice       float64 `json:"p,string"`
	AveragePrice        float64 `json:"ap,string"`
	StopPrice           float64 `json:"sp,string"`
	ExecutionType       string  `json:"x"`      // NEW, TRADE, CANCELED, etc.
	OrderStatus         string  `json:"X"`      // NEW, FILLED, CANCELED, etc.
	OrderId             int64   `json:"i"`
	LastFilledQty       float64 `json:"l,string"`
	CumulativeFilledQty float64 `json:"z,string"`
	LastFilledPrice     float64 `json:"L,string"`
	CommissionAsset     string  `json:"N"`
	Commission          float64 `json:"n,string"`
	OrderTradeTime      int64   `json:"T"`
	TradeId             int64   `json:"t"`
	BidsNotional        float64 `json:"b,string"`
	AskNotional         float64 `json:"a,string"`
	IsMakerSide         bool    `json:"m"`
	IsReduceOnly        bool    `json:"R"`
	WorkingType         string  `json:"wt"`
	OriginalOrderType   string  `json:"ot"`
	PositionSide        string  `json:"ps"`     // BOTH, LONG, SHORT
	IsClosePosition     bool    `json:"cp"`
	ActivationPrice     float64 `json:"AP,string"`
	CallbackRate        float64 `json:"cr,string"`
	RealizedProfit      float64 `json:"rp,string"`
}

// PositionUpdateEvent is a simplified position update for callbacks
type PositionUpdateEvent struct {
	Symbol        string
	PositionAmt   float64
	EntryPrice    float64
	UnrealizedPnL float64
	PositionSide  string
	MarginType    string
}

// StreamPosition represents cached position data from the stream
type StreamPosition struct {
	Symbol        string
	PositionAmt   float64
	EntryPrice    float64
	UnrealizedPnL float64
	PositionSide  string
	MarginType    string
	LastUpdate    time.Time
}

// StreamOrder represents cached order data from the stream
type StreamOrder struct {
	OrderId     int64
	Symbol      string
	Side        string
	Type        string
	Status      string
	Price       float64
	Quantity    float64
	FilledQty   float64
	AvgPrice    float64
	PositionSide string
	LastUpdate  time.Time
}

// NewUserDataStream creates a new user data stream
func NewUserDataStream(client FuturesClient, isTestnet bool) *UserDataStream {
	baseURL := "wss://fstream.binance.com"
	if isTestnet {
		baseURL = "wss://stream.binancefuture.com"
	}

	return &UserDataStream{
		client:    client,
		baseURL:   baseURL,
		isTestnet: isTestnet,
		positions: make(map[string]*StreamPosition),
		orders:    make(map[int64]*StreamOrder),
		stopChan:  make(chan struct{}),
	}
}

// SetAccountUpdateCallback sets the callback for account updates
func (s *UserDataStream) SetAccountUpdateCallback(cb func(*AccountUpdateEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAccountUpdate = cb
}

// SetOrderUpdateCallback sets the callback for order updates
func (s *UserDataStream) SetOrderUpdateCallback(cb func(*OrderUpdateEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onOrderUpdate = cb
}

// SetPositionUpdateCallback sets the callback for position updates
func (s *UserDataStream) SetPositionUpdateCallback(cb func(*PositionUpdateEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onPositionUpdate = cb
}

// Start begins the user data stream connection
func (s *UserDataStream) Start() error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return nil
	}
	s.isRunning = true
	s.mu.Unlock()

	// Get listen key
	listenKey, err := s.client.GetListenKey()
	if err != nil {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
		return err
	}

	s.mu.Lock()
	s.listenKey = listenKey
	s.mu.Unlock()

	// Start connection
	go s.connect()

	// Start keepalive loop
	go s.keepAliveLoop()

	log.Printf("[USER-DATA-STREAM] Started with listen key: %s...", listenKey[:20])
	return nil
}

// Stop stops the user data stream
func (s *UserDataStream) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	s.isRunning = false
	close(s.stopChan)

	if s.wsConn != nil {
		s.wsConn.Close()
	}

	if s.listenKey != "" {
		// Try to close listen key (ignore errors)
		_ = s.client.CloseListenKey(s.listenKey)
	}

	log.Printf("[USER-DATA-STREAM] Stopped")
}

// IsRunning returns true if the stream is running
func (s *UserDataStream) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetPosition returns cached position for a symbol
func (s *UserDataStream) GetPosition(symbol string) (*StreamPosition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pos, ok := s.positions[symbol]
	return pos, ok
}

// GetAllPositions returns all cached positions
func (s *UserDataStream) GetAllPositions() map[string]*StreamPosition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*StreamPosition)
	for k, v := range s.positions {
		result[k] = v
	}
	return result
}

// GetAccountBalance returns the cached account balance
func (s *UserDataStream) GetAccountBalance() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accountBalance
}

// connect establishes the WebSocket connection
func (s *UserDataStream) connect() {
	s.mu.RLock()
	listenKey := s.listenKey
	baseURL := s.baseURL
	s.mu.RUnlock()

	wsURL := baseURL + "/ws/" + listenKey

	for {
		s.mu.RLock()
		if !s.isRunning {
			s.mu.RUnlock()
			return
		}
		s.mu.RUnlock()

		log.Printf("[USER-DATA-STREAM] Connecting to %s...", wsURL[:50]+"...")

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Printf("[USER-DATA-STREAM] Connection failed: %v, retrying in 5s", err)
			time.Sleep(5 * time.Second)
			s.mu.Lock()
			s.reconnects++
			s.mu.Unlock()
			continue
		}

		s.mu.Lock()
		s.wsConn = conn
		s.reconnects = 0
		s.mu.Unlock()

		log.Printf("[USER-DATA-STREAM] Connected successfully")

		// Read messages
		s.readLoop(conn)

		// If we get here, connection was lost
		s.mu.RLock()
		isRunning := s.isRunning
		s.mu.RUnlock()

		if !isRunning {
			return
		}

		log.Printf("[USER-DATA-STREAM] Connection lost, reconnecting in 3s")
		time.Sleep(3 * time.Second)
	}
}

// readLoop reads messages from the WebSocket
func (s *UserDataStream) readLoop(conn *websocket.Conn) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("[USER-DATA-STREAM] Connection closed normally")
			} else {
				log.Printf("[USER-DATA-STREAM] Read error: %v", err)
			}
			return
		}

		s.handleMessage(message)
	}
}

// handleMessage processes incoming WebSocket messages
func (s *UserDataStream) handleMessage(message []byte) {
	// Parse event type first
	var baseEvent struct {
		EventType string `json:"e"`
	}
	if err := json.Unmarshal(message, &baseEvent); err != nil {
		log.Printf("[USER-DATA-STREAM] Failed to parse event type: %v", err)
		return
	}

	switch baseEvent.EventType {
	case "ACCOUNT_UPDATE":
		s.handleAccountUpdate(message)

	case "ORDER_TRADE_UPDATE":
		s.handleOrderUpdate(message)

	case "listenKeyExpired":
		log.Printf("[USER-DATA-STREAM] Listen key expired, refreshing...")
		s.refreshListenKey()

	case "MARGIN_CALL":
		log.Printf("[USER-DATA-STREAM] ⚠️ MARGIN CALL received!")

	default:
		log.Printf("[USER-DATA-STREAM] Unknown event type: %s", baseEvent.EventType)
	}
}

// handleAccountUpdate processes ACCOUNT_UPDATE events
func (s *UserDataStream) handleAccountUpdate(message []byte) {
	var event AccountUpdateEvent
	if err := json.Unmarshal(message, &event); err != nil {
		log.Printf("[USER-DATA-STREAM] Failed to parse ACCOUNT_UPDATE: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastUpdateTime = time.Now()

	// Update balances
	for _, balance := range event.AccountUpdate.Balances {
		if balance.Asset == "USDT" {
			s.accountBalance = balance.WalletBalance
			log.Printf("[USER-DATA-STREAM] Balance updated: %.4f USDT (change: %.4f)",
				balance.WalletBalance, balance.BalanceChange)
		}
	}

	// Update positions
	for _, pos := range event.AccountUpdate.Positions {
		streamPos := &StreamPosition{
			Symbol:        pos.Symbol,
			PositionAmt:   pos.PositionAmount,
			EntryPrice:    pos.EntryPrice,
			UnrealizedPnL: pos.UnrealizedPnL,
			PositionSide:  pos.PositionSide,
			MarginType:    pos.MarginType,
			LastUpdate:    time.Now(),
		}

		s.positions[pos.Symbol+pos.PositionSide] = streamPos

		log.Printf("[USER-DATA-STREAM] Position updated: %s %s amt=%.4f entry=%.4f pnl=%.4f",
			pos.Symbol, pos.PositionSide, pos.PositionAmount, pos.EntryPrice, pos.UnrealizedPnL)

		// Call position callback
		if s.onPositionUpdate != nil {
			go s.onPositionUpdate(&PositionUpdateEvent{
				Symbol:        pos.Symbol,
				PositionAmt:   pos.PositionAmount,
				EntryPrice:    pos.EntryPrice,
				UnrealizedPnL: pos.UnrealizedPnL,
				PositionSide:  pos.PositionSide,
				MarginType:    pos.MarginType,
			})
		}
	}

	// Call account callback
	if s.onAccountUpdate != nil {
		go s.onAccountUpdate(&event)
	}
}

// handleOrderUpdate processes ORDER_TRADE_UPDATE events
func (s *UserDataStream) handleOrderUpdate(message []byte) {
	var event OrderUpdateEvent
	if err := json.Unmarshal(message, &event); err != nil {
		log.Printf("[USER-DATA-STREAM] Failed to parse ORDER_TRADE_UPDATE: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastUpdateTime = time.Now()

	order := &StreamOrder{
		OrderId:      event.Order.OrderId,
		Symbol:       event.Order.Symbol,
		Side:         event.Order.Side,
		Type:         event.Order.OrderType,
		Status:       event.Order.OrderStatus,
		Price:        event.Order.OriginalPrice,
		Quantity:     event.Order.OriginalQuantity,
		FilledQty:    event.Order.CumulativeFilledQty,
		AvgPrice:     event.Order.AveragePrice,
		PositionSide: event.Order.PositionSide,
		LastUpdate:   time.Now(),
	}

	// Update or remove from cache based on status
	if order.Status == "FILLED" || order.Status == "CANCELED" || order.Status == "EXPIRED" {
		delete(s.orders, order.OrderId)
	} else {
		s.orders[order.OrderId] = order
	}

	log.Printf("[USER-DATA-STREAM] Order %s: %s %s %s qty=%.4f @ %.4f status=%s",
		event.Order.ExecutionType, event.Order.Symbol, event.Order.Side,
		event.Order.OrderType, event.Order.OriginalQuantity,
		event.Order.OriginalPrice, event.Order.OrderStatus)

	// Call order callback
	if s.onOrderUpdate != nil {
		go s.onOrderUpdate(&event)
	}
}

// keepAliveLoop sends keepalive requests every 15 minutes
// Binance listen keys expire after 60 minutes, so 15 min interval provides safety margin
func (s *UserDataStream) keepAliveLoop() {
	// Use 15 minute interval for safer margin (keys expire at 60 min)
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	consecutiveFailures := 0
	const maxConsecutiveFailures = 3

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.mu.RLock()
			listenKey := s.listenKey
			isRunning := s.isRunning
			s.mu.RUnlock()

			if !isRunning {
				return
			}

			// Try keepalive with retries - this is CRITICAL for maintaining connection
			var lastErr error
			success := false

			for attempt := 1; attempt <= 3; attempt++ {
				if err := s.client.KeepAliveListenKey(listenKey); err != nil {
					lastErr = err
					log.Printf("[USER-DATA-STREAM] Keepalive attempt %d/3 failed: %v", attempt, err)
					if attempt < 3 {
						// Wait 5 seconds between retries
						time.Sleep(5 * time.Second)
					}
				} else {
					success = true
					break
				}
			}

			if success {
				consecutiveFailures = 0
				log.Printf("[USER-DATA-STREAM] Listen key kept alive successfully")
			} else {
				consecutiveFailures++
				log.Printf("[USER-DATA-STREAM] CRITICAL: All keepalive attempts failed: %v (consecutive failures: %d)",
					lastErr, consecutiveFailures)

				// If we've failed multiple times, try to get a completely new listen key
				if consecutiveFailures >= maxConsecutiveFailures {
					log.Printf("[USER-DATA-STREAM] Max consecutive failures reached, forcing listen key refresh")
					s.refreshListenKey()
					consecutiveFailures = 0
				}
			}
		}
	}
}

// refreshListenKey gets a new listen key and reconnects
func (s *UserDataStream) refreshListenKey() {
	listenKey, err := s.client.GetListenKey()
	if err != nil {
		log.Printf("[USER-DATA-STREAM] Failed to refresh listen key: %v", err)
		return
	}

	s.mu.Lock()
	s.listenKey = listenKey
	if s.wsConn != nil {
		s.wsConn.Close() // This will trigger reconnect
	}
	s.mu.Unlock()

	log.Printf("[USER-DATA-STREAM] Listen key refreshed: %s...", listenKey[:20])
}

// GetStats returns stream statistics
func (s *UserDataStream) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running":         s.isRunning,
		"reconnects":      s.reconnects,
		"positions_count": len(s.positions),
		"orders_count":    len(s.orders),
		"account_balance": s.accountBalance,
		"last_update":     s.lastUpdateTime.Format(time.RFC3339),
	}
}
