// Package autopilot provides integration tests for the Position Controller.
// Epic 10, Story 10.4: Position Controller - Exit Signal Executor
//
// These tests verify the integration between:
// - Exit Decision Service signal delivery
// - Position Controller signal processing
// - Mock Binance client order execution
// - Retry logic on API failures
package autopilot

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/exitdecision"
	"binance-trading-bot/internal/orders"

	"github.com/rs/zerolog"
)

// ============================================================================
// MOCK BINANCE FUTURES CLIENT FOR INTEGRATION TESTS
// ============================================================================

// MockBinanceFuturesClient is a comprehensive mock for integration testing.
// It implements binance.FuturesClient interface with tracking and failure simulation.
type MockBinanceFuturesClient struct {
	mu sync.RWMutex

	// Position data
	positions map[string]*binance.FuturesPosition

	// Algo order tracking
	algoOrders     map[string][]binance.AlgoOrder
	algoOrderIDGen int64

	// Operation tracking
	cancelledAlgoOrders []int64
	cancelledOrders     []int64
	placedAlgoOrders    []binance.AlgoOrderParams
	getPositionCalls    []string
	getAlgoOrdersCalls  []string

	// Failure simulation
	failCancelAlgo      bool
	failCancelAlgoTimes int // Number of times to fail before succeeding
	failCancelAlgoCount int
	failPlaceAlgo       bool
	failPlaceAlgoTimes  int
	failPlaceAlgoCount  int
	failGetPosition     bool
	failGetAlgoOrders   bool

	// Error types to return
	cancelAlgoErr  error
	placeAlgoErr   error
	getPositionErr error

	// Latency simulation
	operationDelay time.Duration
}

// NewMockBinanceFuturesClient creates a new mock Binance futures client for integration testing.
func NewMockBinanceFuturesClient() *MockBinanceFuturesClient {
	return &MockBinanceFuturesClient{
		positions:           make(map[string]*binance.FuturesPosition),
		algoOrders:          make(map[string][]binance.AlgoOrder),
		algoOrderIDGen:      1000,
		cancelledAlgoOrders: []int64{},
		cancelledOrders:     []int64{},
		placedAlgoOrders:    []binance.AlgoOrderParams{},
		getPositionCalls:    []string{},
		getAlgoOrdersCalls:  []string{},
	}
}

// === Setup Methods ===

// SetPosition sets a mock position for testing.
func (m *MockBinanceFuturesClient) SetPosition(symbol string, pos *binance.FuturesPosition) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.positions[symbol] = pos
}

// SetAlgoOrders sets mock algo orders for a symbol.
func (m *MockBinanceFuturesClient) SetAlgoOrders(symbol string, orders []binance.AlgoOrder) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.algoOrders[symbol] = orders
}

// SetFailCancelAlgo configures cancel failure simulation.
func (m *MockBinanceFuturesClient) SetFailCancelAlgo(fail bool, times int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failCancelAlgo = fail
	m.failCancelAlgoTimes = times
	m.failCancelAlgoCount = 0
	m.cancelAlgoErr = err
}

// SetFailPlaceAlgo configures place failure simulation.
func (m *MockBinanceFuturesClient) SetFailPlaceAlgo(fail bool, times int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failPlaceAlgo = fail
	m.failPlaceAlgoTimes = times
	m.failPlaceAlgoCount = 0
	m.placeAlgoErr = err
}

// SetFailGetPosition configures get position failure.
func (m *MockBinanceFuturesClient) SetFailGetPosition(fail bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failGetPosition = fail
	m.getPositionErr = err
}

// SetOperationDelay configures operation latency.
func (m *MockBinanceFuturesClient) SetOperationDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.operationDelay = delay
}

// === Inspection Methods ===

// GetCancelledAlgoOrders returns the list of cancelled algo order IDs.
func (m *MockBinanceFuturesClient) GetCancelledAlgoOrders() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]int64, len(m.cancelledAlgoOrders))
	copy(result, m.cancelledAlgoOrders)
	return result
}

// GetPlacedAlgoOrders returns the list of placed algo orders.
func (m *MockBinanceFuturesClient) GetPlacedAlgoOrders() []binance.AlgoOrderParams {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]binance.AlgoOrderParams, len(m.placedAlgoOrders))
	copy(result, m.placedAlgoOrders)
	return result
}

// GetAlgoOrdersForSymbol returns the current algo orders for a symbol.
func (m *MockBinanceFuturesClient) GetAlgoOrdersForSymbol(symbol string) []binance.AlgoOrder {
	m.mu.RLock()
	defer m.mu.RUnlock()
	orders := m.algoOrders[symbol]
	result := make([]binance.AlgoOrder, len(orders))
	copy(result, orders)
	return result
}

// Reset clears all tracking data.
func (m *MockBinanceFuturesClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelledAlgoOrders = []int64{}
	m.cancelledOrders = []int64{}
	m.placedAlgoOrders = []binance.AlgoOrderParams{}
	m.getPositionCalls = []string{}
	m.getAlgoOrdersCalls = []string{}
	m.failCancelAlgoCount = 0
	m.failPlaceAlgoCount = 0
}

// === FuturesClient Interface Implementation ===

func (m *MockBinanceFuturesClient) GetPositionBySymbol(symbol string) (*binance.FuturesPosition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getPositionCalls = append(m.getPositionCalls, symbol)

	if m.operationDelay > 0 {
		time.Sleep(m.operationDelay)
	}

	if m.failGetPosition {
		if m.getPositionErr != nil {
			return nil, m.getPositionErr
		}
		return nil, errors.New("mock: get position failed")
	}

	pos, ok := m.positions[symbol]
	if !ok {
		return nil, fmt.Errorf("position not found for symbol: %s", symbol)
	}
	return pos, nil
}

func (m *MockBinanceFuturesClient) GetPositions() ([]binance.FuturesPosition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []binance.FuturesPosition
	for _, pos := range m.positions {
		result = append(result, *pos)
	}
	return result, nil
}

func (m *MockBinanceFuturesClient) GetOpenAlgoOrders(symbol string) ([]binance.AlgoOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getAlgoOrdersCalls = append(m.getAlgoOrdersCalls, symbol)

	if m.operationDelay > 0 {
		time.Sleep(m.operationDelay)
	}

	if m.failGetAlgoOrders {
		return nil, errors.New("mock: get algo orders failed")
	}

	orders, ok := m.algoOrders[symbol]
	if !ok {
		return []binance.AlgoOrder{}, nil
	}
	return orders, nil
}

func (m *MockBinanceFuturesClient) CancelAlgoOrder(symbol string, algoId int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.operationDelay > 0 {
		time.Sleep(m.operationDelay)
	}

	if m.failCancelAlgo {
		if m.failCancelAlgoTimes > 0 && m.failCancelAlgoCount >= m.failCancelAlgoTimes {
			// Stop failing after specified times
		} else {
			m.failCancelAlgoCount++
			if m.cancelAlgoErr != nil {
				return m.cancelAlgoErr
			}
			return errors.New("mock: cancel algo order failed")
		}
	}

	m.cancelledAlgoOrders = append(m.cancelledAlgoOrders, algoId)

	// Remove from algo orders
	orders := m.algoOrders[symbol]
	for i, o := range orders {
		if o.AlgoId == algoId {
			m.algoOrders[symbol] = append(orders[:i], orders[i+1:]...)
			break
		}
	}

	return nil
}

func (m *MockBinanceFuturesClient) PlaceAlgoOrder(params binance.AlgoOrderParams) (*binance.AlgoOrderResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.operationDelay > 0 {
		time.Sleep(m.operationDelay)
	}

	if m.failPlaceAlgo {
		if m.failPlaceAlgoTimes > 0 && m.failPlaceAlgoCount >= m.failPlaceAlgoTimes {
			// Stop failing after specified times
		} else {
			m.failPlaceAlgoCount++
			if m.placeAlgoErr != nil {
				return nil, m.placeAlgoErr
			}
			return nil, errors.New("mock: place algo order failed")
		}
	}

	m.placedAlgoOrders = append(m.placedAlgoOrders, params)

	orderID := atomic.AddInt64(&m.algoOrderIDGen, 1)

	// Add to algo orders list
	newOrder := binance.AlgoOrder{
		AlgoId:       orderID,
		Symbol:       params.Symbol,
		Side:         params.Side,
		PositionSide: string(params.PositionSide),
		OrderType:    string(params.Type),
		TriggerPrice: params.TriggerPrice,
		Quantity:     params.Quantity,
		WorkingType:  string(params.WorkingType),
		ReduceOnly:   params.ReduceOnly,
		AlgoStatus:   "NEW",
		CreateTime:   time.Now().UnixMilli(),
	}
	m.algoOrders[params.Symbol] = append(m.algoOrders[params.Symbol], newOrder)

	return &binance.AlgoOrderResponse{
		AlgoId:       orderID,
		Symbol:       params.Symbol,
		Side:         params.Side,
		PositionSide: string(params.PositionSide),
		OrderType:    string(params.Type),
		TriggerPrice: params.TriggerPrice,
		Quantity:     params.Quantity,
		AlgoStatus:   "NEW",
	}, nil
}

func (m *MockBinanceFuturesClient) CancelFuturesOrder(symbol string, orderId int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelledOrders = append(m.cancelledOrders, orderId)
	return nil
}

func (m *MockBinanceFuturesClient) GetOrder(symbol string, orderId int64) (*binance.FuturesOrder, error) {
	return nil, errors.New("not implemented")
}

// Stub implementations for remaining interface methods
func (m *MockBinanceFuturesClient) GetFuturesAccountInfo() (*binance.FuturesAccountInfo, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetCommissionRate(symbol string) (*binance.CommissionRate, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) SetLeverage(symbol string, leverage int) (*binance.LeverageResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) SetMarginType(symbol string, marginType binance.MarginType) error {
	return errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) SetPositionMode(dualSidePosition bool) error {
	return errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetPositionMode() (*binance.PositionModeResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) PlaceFuturesOrder(params binance.FuturesOrderParams) (*binance.FuturesOrderResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) CancelAllFuturesOrders(symbol string) error {
	return errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetOpenOrders(symbol string) ([]binance.FuturesOrder, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) CancelAllAlgoOrders(symbol string) error {
	return errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetAllAlgoOrders(symbol string, limit int) ([]binance.AlgoOrder, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetFundingRate(symbol string) (*binance.FundingRate, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetFundingRateHistory(symbol string, limit int) ([]binance.FundingRate, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetMarkPrice(symbol string) (*binance.MarkPrice, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetAllMarkPrices() ([]binance.MarkPrice, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetOrderBookDepth(symbol string, limit int) (*binance.OrderBookDepth, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetFuturesKlines(symbol, interval string, limit int) ([]binance.Kline, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) Get24hrTicker(symbol string) (*binance.Futures24hrTicker, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetAll24hrTickers() ([]binance.Futures24hrTicker, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetFuturesCurrentPrice(symbol string) (float64, error) {
	return 0, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetFuturesExchangeInfo() (*binance.FuturesExchangeInfo, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetFuturesSymbols() ([]string, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetTradeHistory(symbol string, limit int) ([]binance.FuturesTrade, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetTradeHistoryByDateRange(symbol string, startTime, endTime int64, limit int) ([]binance.FuturesTrade, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetFundingFeeHistory(symbol string, limit int) ([]binance.FundingFeeRecord, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetAllOrders(symbol string, limit int) ([]binance.FuturesOrder, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetAllOrdersByDateRange(symbol string, startTime, endTime int64, limit int) ([]binance.FuturesOrder, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetIncomeHistory(incomeType string, startTime, endTime int64, limit int) ([]binance.IncomeRecord, error) {
	return nil, errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) GetListenKey() (string, error) {
	return "", errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) KeepAliveListenKey(listenKey string) error {
	return errors.New("not implemented")
}
func (m *MockBinanceFuturesClient) CloseListenKey(listenKey string) error {
	return errors.New("not implemented")
}

// Verify interface compliance
var _ binance.FuturesClient = (*MockBinanceFuturesClient)(nil)

// ============================================================================
// MOCK EXIT DECISION SERVICE
// ============================================================================

// MockExitDecisionService is a mock for the Exit Decision Service.
type MockExitDecisionService struct {
	mu          sync.RWMutex
	subscribers []exitdecision.SignalHandler
	running     bool
}

// NewMockExitDecisionService creates a new mock exit decision service.
func NewMockExitDecisionService() *MockExitDecisionService {
	return &MockExitDecisionService{
		subscribers: []exitdecision.SignalHandler{},
	}
}

// SubscribeToSignals registers a signal handler (mimics real service).
func (m *MockExitDecisionService) SubscribeToSignals(handler exitdecision.SignalHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers = append(m.subscribers, handler)
}

// EmitSignal sends a signal to all subscribers.
func (m *MockExitDecisionService) EmitSignal(signal exitdecision.ExitSignal) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, handler := range m.subscribers {
		handler(signal)
	}
}

// GetSubscriberCount returns the number of registered subscribers.
func (m *MockExitDecisionService) GetSubscriberCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.subscribers)
}

// ============================================================================
// MOCK CHAIN EVENT WRITER
// ============================================================================

// MockChainEventWriterDB implements orders.ChainEventWriterDB for testing.
type MockChainEventWriterDB struct {
	mu      sync.RWMutex
	chains  map[string]*orders.OrderChain
	events  map[string][]*orders.ChainEvent
	seqNums map[string]int
}

// NewMockChainEventWriterDB creates a new mock chain event writer DB.
func NewMockChainEventWriterDB() *MockChainEventWriterDB {
	return &MockChainEventWriterDB{
		chains:  make(map[string]*orders.OrderChain),
		events:  make(map[string][]*orders.ChainEvent),
		seqNums: make(map[string]int),
	}
}

func (m *MockChainEventWriterDB) CreateOrderChain(ctx context.Context, chain *orders.OrderChain) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chains[chain.ChainID] = chain
	m.seqNums[chain.ChainID] = 0
	return nil
}

func (m *MockChainEventWriterDB) UpdateOrderChain(ctx context.Context, chain *orders.OrderChain) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chains[chain.ChainID] = chain
	return nil
}

func (m *MockChainEventWriterDB) GetOrderChainByID(ctx context.Context, userID, chainID string) (*orders.OrderChain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chain, ok := m.chains[chainID]
	if !ok {
		return nil, fmt.Errorf("chain not found: %s", chainID)
	}
	return chain, nil
}

func (m *MockChainEventWriterDB) GetOrderChainByChainIDOnly(ctx context.Context, chainID string) (*orders.OrderChain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chain, ok := m.chains[chainID]
	if !ok {
		return nil, nil
	}
	return chain, nil
}

func (m *MockChainEventWriterDB) GetActiveOrderChains(ctx context.Context, userID string) ([]*orders.OrderChain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*orders.OrderChain
	for _, chain := range m.chains {
		if chain.UserID == userID && chain.Status == "active" {
			result = append(result, chain)
		}
	}
	return result, nil
}

func (m *MockChainEventWriterDB) CloseOrderChain(ctx context.Context, chainID string, closeReason string, realizedPnL, totalFees float64, closePrice *float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if chain, ok := m.chains[chainID]; ok {
		chain.Status = "closed"
		chain.CloseReason = &closeReason
	}
	return nil
}

func (m *MockChainEventWriterDB) UpdateOrderChainSLPrice(ctx context.Context, chainID string, newPrice float64) error {
	return nil
}

func (m *MockChainEventWriterDB) UpdateOrderChainTPPrice(ctx context.Context, chainID string, newPrice float64) error {
	return nil
}

func (m *MockChainEventWriterDB) LinkHedgeChain(ctx context.Context, primaryChainID, hedgeChainID string) error {
	return nil
}

func (m *MockChainEventWriterDB) UpdateOrderChainSLDetails(ctx context.Context, chainID string, binanceOrderID int64, limitPrice float64, quantity float64) error {
	return nil
}

func (m *MockChainEventWriterDB) UpdateOrderChainTPDetails(ctx context.Context, chainID string, binanceOrderID int64, limitPrice float64, quantity float64) error {
	return nil
}

func (m *MockChainEventWriterDB) UpdateOrderChainSLFilled(ctx context.Context, chainID string, fillPrice float64, fillTime time.Time) error {
	return nil
}

func (m *MockChainEventWriterDB) UpdateOrderChainTPFilled(ctx context.Context, chainID string, fillPrice float64, fillTime time.Time) error {
	return nil
}

func (m *MockChainEventWriterDB) IncrementOrderChainEventCount(ctx context.Context, chainID string, newSeq int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seqNums[chainID] = newSeq
	return nil
}

func (m *MockChainEventWriterDB) InsertChainEvent(ctx context.Context, event *orders.ChainEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[event.ChainID] = append(m.events[event.ChainID], event)
	return nil
}

func (m *MockChainEventWriterDB) GetChainEvents(ctx context.Context, chainID string) ([]*orders.ChainEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.events[chainID], nil
}

func (m *MockChainEventWriterDB) GetNextEventSequence(ctx context.Context, chainID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.seqNums[chainID] + 1, nil
}

// AddChain adds a chain for testing.
func (m *MockChainEventWriterDB) AddChain(chain *orders.OrderChain) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chains[chain.ChainID] = chain
	m.seqNums[chain.ChainID] = 0
}

// GetEventsForChain returns recorded events for a chain.
func (m *MockChainEventWriterDB) GetEventsForChain(chainID string) []*orders.ChainEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.events[chainID]
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

// TestPositionController_SignalToExecution verifies that a signal from ExitDecision
// results in a Binance order update.
func TestPositionController_SignalToExecution(t *testing.T) {
	// Create mock Binance client
	mockClient := NewMockBinanceFuturesClient()

	// Setup a LONG position with existing SL order
	mockClient.SetPosition("BTCUSDT", &binance.FuturesPosition{
		Symbol:       "BTCUSDT",
		PositionAmt:  0.01,
		EntryPrice:   50000.0,
		MarkPrice:    51000.0,
		PositionSide: "LONG",
	})

	// Existing SL order
	mockClient.SetAlgoOrders("BTCUSDT", []binance.AlgoOrder{
		{
			AlgoId:       1001,
			Symbol:       "BTCUSDT",
			Side:         "SELL",
			PositionSide: "LONG",
			OrderType:    string(binance.FuturesOrderTypeStopMarket),
			TriggerPrice: 49000.0,
			Quantity:     0.01,
			AlgoStatus:   "NEW",
		},
	})

	// Create mock chain event writer
	mockDB := NewMockChainEventWriterDB()
	mockDB.AddChain(&orders.OrderChain{
		ChainID: "chain-001",
		UserID:  "test-user",
		Symbol:  "BTCUSDT",
		Status:  "active",
		Side:    "LONG",
	})

	logger := zerolog.Nop()
	chainWriter := orders.NewChainEventWriter(mockDB, logger)

	// Create Position Controller with fast retry config
	config := &PositionControllerConfig{
		MaxRetries:           3,
		RetryDelay:           10 * time.Millisecond,
		OrderTimeout:         5 * time.Second,
		HealCheckInterval:    1 * time.Hour, // Disable heal loop
		EnableProtectionHeal: false,
		DebugMode:            true,
	}

	pc := NewPositionController(mockClient, nil, chainWriter, config, "test-user")

	// Start the controller
	ctx := context.Background()
	err := pc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start Position Controller: %v", err)
	}
	defer pc.Stop()

	// Directly call the signal handler to simulate Exit Decision signal
	signal := exitdecision.ExitSignal{
		Symbol:       "BTCUSDT",
		Mode:         "scalp",
		Side:         "long",
		ExitType:     exitdecision.ExitTypeTrailing,
		TriggerPrice: 50500.0, // New SL price (higher than current 49000 for LONG)
		CurrentPrice: 51000.0,
		EntryPrice:   50000.0,
		Urgency:      exitdecision.UrgencyImmediate,
		Reason:       "Trailing SL update",
		Timestamp:    time.Now(),
		UserID:       "test-user",
	}

	// Handle the signal
	pc.handleExitSignal(signal)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify old SL order was cancelled
	cancelledOrders := mockClient.GetCancelledAlgoOrders()
	if len(cancelledOrders) == 0 {
		t.Error("Expected old SL order to be cancelled")
	} else if cancelledOrders[0] != 1001 {
		t.Errorf("Expected order 1001 to be cancelled, got %d", cancelledOrders[0])
	}

	// Verify new SL order was placed
	placedOrders := mockClient.GetPlacedAlgoOrders()
	if len(placedOrders) == 0 {
		t.Fatal("Expected new SL order to be placed")
	}

	newOrder := placedOrders[0]
	if newOrder.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", newOrder.Symbol)
	}
	if newOrder.Type != binance.FuturesOrderTypeStopMarket {
		t.Errorf("Expected STOP_MARKET type, got %s", newOrder.Type)
	}
	if newOrder.TriggerPrice != 50500.0 {
		t.Errorf("Expected trigger price 50500.0, got %f", newOrder.TriggerPrice)
	}
	if newOrder.Side != "SELL" {
		t.Errorf("Expected SELL side for LONG SL, got %s", newOrder.Side)
	}

	// Verify statistics were updated
	status := pc.GetStatus()
	if status.SignalsProcessed < 1 {
		t.Errorf("Expected signals processed >= 1, got %d", status.SignalsProcessed)
	}
	if status.SLUpdatesExecuted < 1 {
		t.Errorf("Expected SL updates >= 1, got %d", status.SLUpdatesExecuted)
	}
}

// TestPositionController_ProtectionHeal verifies that missing SL/TP orders are detected and placed.
func TestPositionController_ProtectionHeal(t *testing.T) {
	// Create mock Binance client with position but no SL/TP orders
	mockClient := NewMockBinanceFuturesClient()

	mockClient.SetPosition("ETHUSDT", &binance.FuturesPosition{
		Symbol:       "ETHUSDT",
		PositionAmt:  0.5, // LONG position
		EntryPrice:   3000.0,
		MarkPrice:    3100.0,
		PositionSide: "LONG",
	})

	// No algo orders set - this simulates missing protection orders

	// Create mock chain event writer with an active chain
	mockDB := NewMockChainEventWriterDB()
	mockDB.AddChain(&orders.OrderChain{
		ChainID: "chain-eth-001",
		UserID:  "test-user",
		Symbol:  "ETHUSDT",
		Status:  "active",
		Side:    "LONG",
	})

	logger := zerolog.Nop()
	chainWriter := orders.NewChainEventWriter(mockDB, logger)

	// Create Position Controller with protection heal enabled
	config := &PositionControllerConfig{
		MaxRetries:           3,
		RetryDelay:           10 * time.Millisecond,
		OrderTimeout:         5 * time.Second,
		HealCheckInterval:    50 * time.Millisecond, // Fast interval for testing
		EnableProtectionHeal: true,
		DebugMode:            true,
	}

	pc := NewPositionController(mockClient, nil, chainWriter, config, "test-user")

	ctx := context.Background()
	err := pc.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start Position Controller: %v", err)
	}

	// Wait for heal loop to run
	time.Sleep(200 * time.Millisecond)

	pc.Stop()

	// Verify protection orders were placed
	placedOrders := mockClient.GetPlacedAlgoOrders()

	// Should have placed both SL and TP
	slPlaced := false
	tpPlaced := false

	for _, order := range placedOrders {
		if order.Symbol != "ETHUSDT" {
			continue
		}
		if order.Type == binance.FuturesOrderTypeStopMarket {
			slPlaced = true
			// For LONG position, SL should be SELL
			if order.Side != "SELL" {
				t.Errorf("Expected SL side SELL for LONG, got %s", order.Side)
			}
			// SL price should be below entry (1.5% default)
			expectedSL := 3000.0 * (1 - 0.015)
			tolerance := 1.0 // Allow small rounding difference
			if order.TriggerPrice < expectedSL-tolerance || order.TriggerPrice > expectedSL+tolerance {
				t.Errorf("Expected SL price around %.2f, got %.2f", expectedSL, order.TriggerPrice)
			}
		}
		if order.Type == binance.FuturesOrderTypeTakeProfitMarket {
			tpPlaced = true
			// For LONG position, TP should be SELL
			if order.Side != "SELL" {
				t.Errorf("Expected TP side SELL for LONG, got %s", order.Side)
			}
			// TP price should be above entry (2% default)
			expectedTP := 3000.0 * (1 + 0.02)
			tolerance := 1.0
			if order.TriggerPrice < expectedTP-tolerance || order.TriggerPrice > expectedTP+tolerance {
				t.Errorf("Expected TP price around %.2f, got %.2f", expectedTP, order.TriggerPrice)
			}
		}
	}

	if !slPlaced {
		t.Error("Expected protection SL order to be placed")
	}
	if !tpPlaced {
		t.Error("Expected protection TP order to be placed")
	}

	// Verify heal actions were recorded
	status := pc.GetStatus()
	if status.HealActionsExecuted < 2 {
		t.Errorf("Expected heal actions >= 2, got %d", status.HealActionsExecuted)
	}
}

// TestPositionController_SLOnlyImproves verifies SL only moves in favorable direction.
func TestPositionController_SLOnlyImproves(t *testing.T) {
	testCases := []struct {
		name         string
		side         string
		positionAmt  float64
		currentSL    float64
		newSLPrice   float64
		shouldUpdate bool
		description  string
	}{
		{
			name:         "LONG - new SL higher (valid)",
			side:         "LONG",
			positionAmt:  0.01,
			currentSL:    49000.0,
			newSLPrice:   49500.0, // Higher is better for LONG
			shouldUpdate: true,
			description:  "SL moved up toward entry price",
		},
		{
			name:         "LONG - new SL lower (invalid)",
			side:         "LONG",
			positionAmt:  0.01,
			currentSL:    49000.0,
			newSLPrice:   48500.0, // Lower is worse for LONG
			shouldUpdate: false,
			description:  "SL moved down away from entry price",
		},
		{
			name:         "SHORT - new SL lower (valid)",
			side:         "SHORT",
			positionAmt:  -0.01,
			currentSL:    51000.0,
			newSLPrice:   50500.0, // Lower is better for SHORT
			shouldUpdate: true,
			description:  "SL moved down toward entry price",
		},
		{
			name:         "SHORT - new SL higher (invalid)",
			side:         "SHORT",
			positionAmt:  -0.01,
			currentSL:    51000.0,
			newSLPrice:   51500.0, // Higher is worse for SHORT
			shouldUpdate: false,
			description:  "SL moved up away from entry price",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := NewMockBinanceFuturesClient()

			positionSide := "LONG"
			if tc.positionAmt < 0 {
				positionSide = "SHORT"
			}

			mockClient.SetPosition("BTCUSDT", &binance.FuturesPosition{
				Symbol:       "BTCUSDT",
				PositionAmt:  tc.positionAmt,
				EntryPrice:   50000.0,
				MarkPrice:    50500.0,
				PositionSide: positionSide,
			})

			// Set existing SL order
			mockClient.SetAlgoOrders("BTCUSDT", []binance.AlgoOrder{
				{
					AlgoId:       2001,
					Symbol:       "BTCUSDT",
					Side:         "SELL",
					PositionSide: positionSide,
					OrderType:    string(binance.FuturesOrderTypeStopMarket),
					TriggerPrice: tc.currentSL,
					Quantity:     0.01,
					AlgoStatus:   "NEW",
				},
			})

			config := &PositionControllerConfig{
				MaxRetries:           3,
				RetryDelay:           10 * time.Millisecond,
				OrderTimeout:         5 * time.Second,
				EnableProtectionHeal: false,
				DebugMode:            true,
			}

			pc := NewPositionController(mockClient, nil, nil, config, "test-user")

			ctx := context.Background()
			pc.Start(ctx)
			defer pc.Stop()

			// Send trailing SL signal with the new price
			signal := exitdecision.ExitSignal{
				Symbol:       "BTCUSDT",
				ExitType:     exitdecision.ExitTypeTrailing,
				TriggerPrice: tc.newSLPrice,
				CurrentPrice: 50500.0,
				EntryPrice:   50000.0,
				Urgency:      exitdecision.UrgencyImmediate,
			}

			pc.handleExitSignal(signal)

			// Wait for processing
			time.Sleep(100 * time.Millisecond)

			placedOrders := mockClient.GetPlacedAlgoOrders()

			if tc.shouldUpdate {
				if len(placedOrders) == 0 {
					t.Errorf("%s: Expected SL order to be placed", tc.description)
				} else {
					placed := placedOrders[0]
					if placed.TriggerPrice != tc.newSLPrice {
						t.Errorf("%s: Expected SL price %.2f, got %.2f", tc.description, tc.newSLPrice, placed.TriggerPrice)
					}
				}
			} else {
				// For invalid updates, we should still see processing but no new orders
				// The isValidSLUpdate check happens after getting position info
				if len(placedOrders) > 0 {
					t.Errorf("%s: Expected no SL order to be placed (unfavorable direction)", tc.description)
				}
			}
		})
	}
}

// TestPositionController_RetryOnFailure verifies retries work on Binance API failure.
func TestPositionController_RetryOnFailure(t *testing.T) {
	t.Run("cancel retry succeeds after 2 failures", func(t *testing.T) {
		mockClient := NewMockBinanceFuturesClient()

		mockClient.SetPosition("BTCUSDT", &binance.FuturesPosition{
			Symbol:       "BTCUSDT",
			PositionAmt:  0.01,
			EntryPrice:   50000.0,
			PositionSide: "LONG",
		})

		mockClient.SetAlgoOrders("BTCUSDT", []binance.AlgoOrder{
			{
				AlgoId:       3001,
				Symbol:       "BTCUSDT",
				OrderType:    string(binance.FuturesOrderTypeStopMarket),
				TriggerPrice: 49000.0,
				Quantity:     0.01,
			},
		})

		// Fail cancel twice, then succeed
		mockClient.SetFailCancelAlgo(true, 2, errors.New("temporary network error"))

		config := &PositionControllerConfig{
			MaxRetries:           3,
			RetryDelay:           10 * time.Millisecond,
			OrderTimeout:         5 * time.Second,
			EnableProtectionHeal: false,
			DebugMode:            true,
		}

		pc := NewPositionController(mockClient, nil, nil, config, "test-user")

		ctx := context.Background()
		pc.Start(ctx)
		defer pc.Stop()

		signal := exitdecision.ExitSignal{
			Symbol:       "BTCUSDT",
			ExitType:     exitdecision.ExitTypeTrailing,
			TriggerPrice: 49500.0,
			CurrentPrice: 51000.0,
			Urgency:      exitdecision.UrgencyImmediate,
		}

		pc.handleExitSignal(signal)

		// Wait for retries
		time.Sleep(300 * time.Millisecond)

		// Should eventually succeed
		cancelledOrders := mockClient.GetCancelledAlgoOrders()
		if len(cancelledOrders) == 0 {
			t.Error("Expected order to be cancelled after retries")
		}

		placedOrders := mockClient.GetPlacedAlgoOrders()
		if len(placedOrders) == 0 {
			t.Error("Expected new order to be placed after successful cancel")
		}
	})

	t.Run("place order retry succeeds after 1 failure", func(t *testing.T) {
		mockClient := NewMockBinanceFuturesClient()

		mockClient.SetPosition("ETHUSDT", &binance.FuturesPosition{
			Symbol:       "ETHUSDT",
			PositionAmt:  0.5,
			EntryPrice:   3000.0,
			PositionSide: "LONG",
		})

		// No existing orders - will place new SL
		mockClient.SetAlgoOrders("ETHUSDT", []binance.AlgoOrder{})

		// Fail place once, then succeed
		mockClient.SetFailPlaceAlgo(true, 1, errors.New("rate limit exceeded"))

		config := &PositionControllerConfig{
			MaxRetries:           3,
			RetryDelay:           10 * time.Millisecond,
			OrderTimeout:         5 * time.Second,
			EnableProtectionHeal: false,
			DebugMode:            true,
		}

		pc := NewPositionController(mockClient, nil, nil, config, "test-user")

		ctx := context.Background()
		pc.Start(ctx)
		defer pc.Stop()

		signal := exitdecision.ExitSignal{
			Symbol:       "ETHUSDT",
			ExitType:     exitdecision.ExitTypeTrailing,
			TriggerPrice: 2950.0,
			CurrentPrice: 3100.0,
			Urgency:      exitdecision.UrgencyImmediate,
		}

		pc.handleExitSignal(signal)

		// Wait for retries
		time.Sleep(200 * time.Millisecond)

		// Should eventually succeed
		placedOrders := mockClient.GetPlacedAlgoOrders()
		if len(placedOrders) == 0 {
			t.Error("Expected order to be placed after retries")
		}
	})

	t.Run("fails after max retries exceeded", func(t *testing.T) {
		mockClient := NewMockBinanceFuturesClient()

		mockClient.SetPosition("BTCUSDT", &binance.FuturesPosition{
			Symbol:       "BTCUSDT",
			PositionAmt:  0.01,
			EntryPrice:   50000.0,
			PositionSide: "LONG",
		})

		mockClient.SetAlgoOrders("BTCUSDT", []binance.AlgoOrder{})

		// Always fail
		mockClient.SetFailPlaceAlgo(true, 100, errors.New("persistent error"))

		config := &PositionControllerConfig{
			MaxRetries:           3,
			RetryDelay:           10 * time.Millisecond,
			OrderTimeout:         5 * time.Second,
			EnableProtectionHeal: false,
			DebugMode:            true,
		}

		pc := NewPositionController(mockClient, nil, nil, config, "test-user")

		ctx := context.Background()
		pc.Start(ctx)
		defer pc.Stop()

		signal := exitdecision.ExitSignal{
			Symbol:       "BTCUSDT",
			ExitType:     exitdecision.ExitTypeTrailing,
			TriggerPrice: 49500.0,
			CurrentPrice: 51000.0,
			Urgency:      exitdecision.UrgencyImmediate,
		}

		pc.handleExitSignal(signal)

		// Wait for all retries to complete
		time.Sleep(300 * time.Millisecond)

		// Should have recorded error
		status := pc.GetStatus()
		if status.LastError == "" {
			t.Error("Expected error to be recorded after max retries")
		}

		// No orders should be successfully placed
		placedOrders := mockClient.GetPlacedAlgoOrders()
		if len(placedOrders) > 0 {
			t.Error("Expected no orders to be placed after persistent failure")
		}
	})
}

// TestPositionController_ConcurrentSignals verifies handling of multiple signals.
func TestPositionController_ConcurrentSignals(t *testing.T) {
	mockClient := NewMockBinanceFuturesClient()

	// Setup multiple positions
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	for _, symbol := range symbols {
		mockClient.SetPosition(symbol, &binance.FuturesPosition{
			Symbol:       symbol,
			PositionAmt:  0.01,
			EntryPrice:   1000.0,
			PositionSide: "LONG",
		})
		mockClient.SetAlgoOrders(symbol, []binance.AlgoOrder{})
	}

	config := &PositionControllerConfig{
		MaxRetries:           3,
		RetryDelay:           10 * time.Millisecond,
		OrderTimeout:         5 * time.Second,
		EnableProtectionHeal: false,
		DebugMode:            false, // Less logging for concurrent test
	}

	pc := NewPositionController(mockClient, nil, nil, config, "test-user")

	ctx := context.Background()
	pc.Start(ctx)
	defer pc.Stop()

	// Send signals concurrently
	var wg sync.WaitGroup
	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			signal := exitdecision.ExitSignal{
				Symbol:       sym,
				ExitType:     exitdecision.ExitTypeTrailing,
				TriggerPrice: 990.0,
				CurrentPrice: 1050.0,
				Urgency:      exitdecision.UrgencyImmediate,
			}
			pc.handleExitSignal(signal)
		}(symbol)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	// Verify all orders were placed
	placedOrders := mockClient.GetPlacedAlgoOrders()
	if len(placedOrders) != len(symbols) {
		t.Errorf("Expected %d orders placed, got %d", len(symbols), len(placedOrders))
	}

	// Verify statistics
	status := pc.GetStatus()
	if status.SignalsProcessed < int64(len(symbols)) {
		t.Errorf("Expected signals processed >= %d, got %d", len(symbols), status.SignalsProcessed)
	}
}

// TestPositionController_ShortPositionHandling verifies SHORT position handling.
func TestPositionController_ShortPositionHandling(t *testing.T) {
	mockClient := NewMockBinanceFuturesClient()

	// Setup SHORT position
	mockClient.SetPosition("BTCUSDT", &binance.FuturesPosition{
		Symbol:       "BTCUSDT",
		PositionAmt:  -0.01, // Negative = SHORT
		EntryPrice:   50000.0,
		MarkPrice:    49000.0,
		PositionSide: "SHORT",
	})

	mockClient.SetAlgoOrders("BTCUSDT", []binance.AlgoOrder{
		{
			AlgoId:       4001,
			Symbol:       "BTCUSDT",
			Side:         "BUY", // BUY to close SHORT
			PositionSide: "SHORT",
			OrderType:    string(binance.FuturesOrderTypeStopMarket),
			TriggerPrice: 51000.0, // SL above entry for SHORT
			Quantity:     0.01,
			AlgoStatus:   "NEW",
		},
	})

	config := &PositionControllerConfig{
		MaxRetries:           3,
		RetryDelay:           10 * time.Millisecond,
		OrderTimeout:         5 * time.Second,
		EnableProtectionHeal: false,
		DebugMode:            true,
	}

	pc := NewPositionController(mockClient, nil, nil, config, "test-user")

	ctx := context.Background()
	pc.Start(ctx)
	defer pc.Stop()

	// Trail SL down (favorable for SHORT)
	signal := exitdecision.ExitSignal{
		Symbol:       "BTCUSDT",
		ExitType:     exitdecision.ExitTypeTrailing,
		TriggerPrice: 50500.0, // Lower SL is better for SHORT
		CurrentPrice: 49000.0,
		EntryPrice:   50000.0,
		Urgency:      exitdecision.UrgencyImmediate,
	}

	pc.handleExitSignal(signal)
	time.Sleep(100 * time.Millisecond)

	// Verify order update
	placedOrders := mockClient.GetPlacedAlgoOrders()
	if len(placedOrders) == 0 {
		t.Fatal("Expected SL order to be placed")
	}

	order := placedOrders[0]
	if order.Side != "BUY" {
		t.Errorf("Expected BUY side for SHORT SL, got %s", order.Side)
	}
	if order.PositionSide != binance.PositionSideShort {
		t.Errorf("Expected SHORT position side, got %s", order.PositionSide)
	}
	if order.TriggerPrice != 50500.0 {
		t.Errorf("Expected trigger price 50500.0, got %f", order.TriggerPrice)
	}
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

func BenchmarkPositionController_SignalProcessing(b *testing.B) {
	mockClient := NewMockBinanceFuturesClient()

	mockClient.SetPosition("BTCUSDT", &binance.FuturesPosition{
		Symbol:       "BTCUSDT",
		PositionAmt:  0.01,
		EntryPrice:   50000.0,
		PositionSide: "LONG",
	})
	mockClient.SetAlgoOrders("BTCUSDT", []binance.AlgoOrder{})

	config := &PositionControllerConfig{
		MaxRetries:           1, // Reduce retries for benchmark
		RetryDelay:           time.Millisecond,
		OrderTimeout:         time.Second,
		EnableProtectionHeal: false,
		DebugMode:            false,
	}

	pc := NewPositionController(mockClient, nil, nil, config, "bench-user")

	ctx := context.Background()
	pc.Start(ctx)
	defer pc.Stop()

	signal := exitdecision.ExitSignal{
		Symbol:       "BTCUSDT",
		ExitType:     exitdecision.ExitTypeTrailing,
		TriggerPrice: 49500.0,
		CurrentPrice: 51000.0,
		Urgency:      exitdecision.UrgencyImmediate,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		signal.TriggerPrice = 49500.0 + float64(i%100) // Vary price to force updates
		pc.handleExitSignal(signal)
	}
}
