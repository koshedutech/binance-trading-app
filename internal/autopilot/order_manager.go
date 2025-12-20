package autopilot

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// OrderManagerConfig holds order management configuration
type OrderManagerConfig struct {
	TakeProfitPercent   float64 `json:"take_profit_percent"`   // Default 5%
	StopLossPercent     float64 `json:"stop_loss_percent"`     // Default 2%
	TrailingStopEnabled bool    `json:"trailing_stop_enabled"` // Enable trailing stop
	TrailingStopPercent float64 `json:"trailing_stop_percent"` // Trail by this %
	TrailingActivation  float64 `json:"trailing_activation"`   // Activate trailing after this % profit
	UpdateIntervalSecs  int     `json:"update_interval_secs"`  // How often to check positions
}

// DefaultOrderManagerConfig returns default configuration
func DefaultOrderManagerConfig() *OrderManagerConfig {
	return &OrderManagerConfig{
		TakeProfitPercent:   5.0,  // 5% take profit
		StopLossPercent:     2.0,  // 2% stop loss
		TrailingStopEnabled: true,
		TrailingStopPercent: 1.0,  // Trail by 1%
		TrailingActivation:  2.0,  // Activate trailing after 2% profit
		UpdateIntervalSecs:  5,
	}
}

// OrderManager manages automatic TP/SL orders and trailing stops
type OrderManager struct {
	config     *OrderManagerConfig
	client     binance.BinanceClient
	repository *database.Repository

	// Track positions being managed
	managedPositions map[string]*ManagedPosition
	mu               sync.RWMutex

	stopChan chan struct{}
	running  bool
}

// ManagedPosition tracks a position's order management state
type ManagedPosition struct {
	TradeID         int64
	Symbol          string
	Side            string // BUY or SELL
	EntryPrice      float64
	Quantity        float64
	TakeProfitPrice float64
	StopLossPrice   float64
	HighestPrice    float64
	LowestPrice     float64
	TrailingActive  bool
	TPOrderID       int64
	SLOrderID       int64
	AIDecisionID    *int64
}

// NewOrderManager creates a new order manager
func NewOrderManager(config *OrderManagerConfig, client binance.BinanceClient, repo *database.Repository) *OrderManager {
	if config == nil {
		config = DefaultOrderManagerConfig()
	}

	return &OrderManager{
		config:           config,
		client:           client,
		repository:       repo,
		managedPositions: make(map[string]*ManagedPosition),
		stopChan:         make(chan struct{}),
	}
}

// Start begins the order management loop
func (om *OrderManager) Start() {
	om.mu.Lock()
	if om.running {
		om.mu.Unlock()
		return
	}
	om.running = true
	om.mu.Unlock()

	log.Printf("[OrderManager] Starting with TP: %.1f%%, SL: %.1f%%, Trailing: %v (%.1f%%)",
		om.config.TakeProfitPercent, om.config.StopLossPercent,
		om.config.TrailingStopEnabled, om.config.TrailingStopPercent)

	go om.runLoop()
}

// Stop stops the order manager
func (om *OrderManager) Stop() {
	om.mu.Lock()
	if !om.running {
		om.mu.Unlock()
		return
	}
	om.running = false
	om.mu.Unlock()

	close(om.stopChan)
	log.Printf("[OrderManager] Stopped")
}

// runLoop is the main order management loop
func (om *OrderManager) runLoop() {
	interval := time.Duration(om.config.UpdateIntervalSecs) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			om.updateAllPositions()
		case <-om.stopChan:
			return
		}
	}
}

// RegisterPosition registers a new position for order management
func (om *OrderManager) RegisterPosition(tradeID int64, symbol, side string, entryPrice, quantity float64, aiDecisionID *int64) error {
	om.mu.Lock()
	defer om.mu.Unlock()

	// Calculate TP and SL prices
	var tpPrice, slPrice float64
	if side == "BUY" {
		tpPrice = entryPrice * (1 + om.config.TakeProfitPercent/100)
		slPrice = entryPrice * (1 - om.config.StopLossPercent/100)
	} else {
		tpPrice = entryPrice * (1 - om.config.TakeProfitPercent/100)
		slPrice = entryPrice * (1 + om.config.StopLossPercent/100)
	}

	managed := &ManagedPosition{
		TradeID:         tradeID,
		Symbol:          symbol,
		Side:            side,
		EntryPrice:      entryPrice,
		Quantity:        quantity,
		TakeProfitPrice: tpPrice,
		StopLossPrice:   slPrice,
		HighestPrice:    entryPrice,
		LowestPrice:     entryPrice,
		AIDecisionID:    aiDecisionID,
	}

	// Place TP order
	tpOrderID, err := om.placeTakeProfitOrder(managed)
	if err != nil {
		log.Printf("[OrderManager] Failed to place TP order for %s: %v", symbol, err)
	} else {
		managed.TPOrderID = tpOrderID
	}

	// Place SL order
	slOrderID, err := om.placeStopLossOrder(managed)
	if err != nil {
		log.Printf("[OrderManager] Failed to place SL order for %s: %v", symbol, err)
	} else {
		managed.SLOrderID = slOrderID
	}

	om.managedPositions[symbol] = managed

	log.Printf("[OrderManager] Registered %s position: Entry=%.2f, TP=%.2f (%.1f%%), SL=%.2f (%.1f%%)",
		symbol, entryPrice, tpPrice, om.config.TakeProfitPercent, slPrice, om.config.StopLossPercent)

	return nil
}

// UnregisterPosition removes a position from management
func (om *OrderManager) UnregisterPosition(symbol string) {
	om.mu.Lock()
	defer om.mu.Unlock()

	if managed, exists := om.managedPositions[symbol]; exists {
		// Cancel any open orders
		if managed.TPOrderID > 0 {
			om.client.CancelOrder(symbol, managed.TPOrderID)
		}
		if managed.SLOrderID > 0 {
			om.client.CancelOrder(symbol, managed.SLOrderID)
		}
		delete(om.managedPositions, symbol)
		log.Printf("[OrderManager] Unregistered position: %s", symbol)
	}
}

// updateAllPositions updates trailing stops for all managed positions
func (om *OrderManager) updateAllPositions() {
	om.mu.Lock()
	positions := make([]*ManagedPosition, 0, len(om.managedPositions))
	for _, pos := range om.managedPositions {
		positions = append(positions, pos)
	}
	om.mu.Unlock()

	for _, pos := range positions {
		om.updatePosition(pos)
	}
}

// updatePosition updates a single position's trailing stop
func (om *OrderManager) updatePosition(pos *ManagedPosition) {
	// Get current price
	currentPrice, err := om.client.GetCurrentPrice(pos.Symbol)
	if err != nil {
		log.Printf("[OrderManager] Failed to get price for %s: %v", pos.Symbol, err)
		return
	}

	om.mu.Lock()
	defer om.mu.Unlock()

	// Update highest/lowest price
	if currentPrice > pos.HighestPrice {
		pos.HighestPrice = currentPrice
	}
	if currentPrice < pos.LowestPrice || pos.LowestPrice == 0 {
		pos.LowestPrice = currentPrice
	}

	// Check if trailing should be activated
	if om.config.TrailingStopEnabled && !pos.TrailingActive {
		var profitPercent float64
		if pos.Side == "BUY" {
			profitPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
		} else {
			profitPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
		}

		if profitPercent >= om.config.TrailingActivation {
			pos.TrailingActive = true
			log.Printf("[OrderManager] %s: Trailing stop activated at %.1f%% profit", pos.Symbol, profitPercent)
		}
	}

	// Update trailing stop if active
	if pos.TrailingActive {
		var newSLPrice float64
		if pos.Side == "BUY" {
			// For long positions, trail from highest price
			newSLPrice = pos.HighestPrice * (1 - om.config.TrailingStopPercent/100)
			if newSLPrice > pos.StopLossPrice {
				om.updateStopLoss(pos, newSLPrice)
			}
		} else {
			// For short positions, trail from lowest price
			newSLPrice = pos.LowestPrice * (1 + om.config.TrailingStopPercent/100)
			if newSLPrice < pos.StopLossPrice {
				om.updateStopLoss(pos, newSLPrice)
			}
		}
	}

	// Update trade record in database with current high/low
	if om.repository != nil {
		ctx := context.Background()
		om.repository.UpdateTradeTrailingInfo(ctx, pos.TradeID, pos.HighestPrice, pos.LowestPrice, pos.StopLossPrice)
	}
}

// updateStopLoss updates the stop loss order
func (om *OrderManager) updateStopLoss(pos *ManagedPosition, newSLPrice float64) {
	// Cancel old SL order
	if pos.SLOrderID > 0 {
		if err := om.client.CancelOrder(pos.Symbol, pos.SLOrderID); err != nil {
			log.Printf("[OrderManager] Failed to cancel old SL order: %v", err)
		}
	}

	// Update price
	oldSL := pos.StopLossPrice
	pos.StopLossPrice = newSLPrice

	// Place new SL order
	newOrderID, err := om.placeStopLossOrder(pos)
	if err != nil {
		log.Printf("[OrderManager] Failed to place new SL order: %v", err)
	} else {
		pos.SLOrderID = newOrderID
	}

	log.Printf("[OrderManager] %s: Trailing SL updated: %.2f -> %.2f", pos.Symbol, oldSL, newSLPrice)
}

// placeTakeProfitOrder places a take profit limit order
func (om *OrderManager) placeTakeProfitOrder(pos *ManagedPosition) (int64, error) {
	side := "SELL"
	if pos.Side == "SELL" {
		side = "BUY"
	}

	params := map[string]string{
		"symbol":      pos.Symbol,
		"side":        side,
		"type":        "LIMIT",
		"timeInForce": "GTC",
		"price":       fmt.Sprintf("%.8f", pos.TakeProfitPrice),
		"quantity":    fmt.Sprintf("%.8f", pos.Quantity),
	}

	order, err := om.client.PlaceOrder(params)
	if err != nil {
		return 0, err
	}

	log.Printf("[OrderManager] TP order placed: %s %s @ %.2f, ID: %d",
		side, pos.Symbol, pos.TakeProfitPrice, order.OrderId)

	return order.OrderId, nil
}

// placeStopLossOrder places a stop loss order
func (om *OrderManager) placeStopLossOrder(pos *ManagedPosition) (int64, error) {
	side := "SELL"
	if pos.Side == "SELL" {
		side = "BUY"
	}

	params := map[string]string{
		"symbol":      pos.Symbol,
		"side":        side,
		"type":        "STOP_LOSS_LIMIT",
		"timeInForce": "GTC",
		"stopPrice":   fmt.Sprintf("%.8f", pos.StopLossPrice),
		"price":       fmt.Sprintf("%.8f", pos.StopLossPrice*0.999), // Slightly below stop for execution
		"quantity":    fmt.Sprintf("%.8f", pos.Quantity),
	}

	order, err := om.client.PlaceOrder(params)
	if err != nil {
		// Try market stop if limit fails
		params["type"] = "STOP_LOSS"
		delete(params, "price")
		order, err = om.client.PlaceOrder(params)
		if err != nil {
			return 0, err
		}
	}

	log.Printf("[OrderManager] SL order placed: %s %s @ %.2f, ID: %d",
		side, pos.Symbol, pos.StopLossPrice, order.OrderId)

	return order.OrderId, nil
}

// GetManagedPosition returns a managed position's details
func (om *OrderManager) GetManagedPosition(symbol string) *ManagedPosition {
	om.mu.RLock()
	defer om.mu.RUnlock()

	if pos, exists := om.managedPositions[symbol]; exists {
		// Return a copy
		copy := *pos
		return &copy
	}
	return nil
}

// GetAllManagedPositions returns all managed positions
func (om *OrderManager) GetAllManagedPositions() map[string]*ManagedPosition {
	om.mu.RLock()
	defer om.mu.RUnlock()

	result := make(map[string]*ManagedPosition)
	for k, v := range om.managedPositions {
		copy := *v
		result[k] = &copy
	}
	return result
}
