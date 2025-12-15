package bot

import (
	"binance-trading-bot/config"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/strategy"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// TradingBot manages the trading operations
type TradingBot struct {
	client            *binance.Client
	config            *config.Config
	repo              *database.Repository
	eventBus          *events.EventBus
	strategies        map[string]strategy.Strategy
	enabledStrategies map[string]bool
	positions         map[string]*Position
	orders            map[string]*Order
	mu                sync.RWMutex
	stopChan          chan struct{}
	wg                sync.WaitGroup
}

// Position represents an open position
type Position struct {
	Symbol       string
	EntryPrice   float64
	Quantity     float64
	StopLoss     float64
	TakeProfit   float64
	Side         string
	EntryTime    time.Time
	StopLossID   int64
	TakeProfitID int64
}

// Order represents an order
type Order struct {
	ID            int64
	Symbol        string
	Type          string
	Side          string
	Price         float64
	Quantity      float64
	Status        string
	CreatedAt     time.Time
	ExecutedPrice float64
}

func NewTradingBot(cfg *config.Config, repo *database.Repository, eventBus *events.EventBus) (*TradingBot, error) {
	baseURL := cfg.BinanceConfig.BaseURL
	if cfg.BinanceConfig.TestNet {
		baseURL = "https://testnet.binance.vision"
	}

	client := binance.NewClient(
		cfg.BinanceConfig.APIKey,
		cfg.BinanceConfig.SecretKey,
		baseURL,
	)

	return &TradingBot{
		client:            client,
		config:            cfg,
		repo:              repo,
		eventBus:          eventBus,
		strategies:        make(map[string]strategy.Strategy),
		enabledStrategies: make(map[string]bool),
		positions:         make(map[string]*Position),
		orders:            make(map[string]*Order),
		stopChan:          make(chan struct{}),
	}, nil
}

// GetBinanceClient returns the Binance client
func (b *TradingBot) GetBinanceClient() *binance.Client {
	return b.client
}

// RegisterStrategy registers a new trading strategy
func (b *TradingBot) RegisterStrategy(name string, s strategy.Strategy) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.strategies[name] = s
	b.enabledStrategies[name] = true // Enable by default
	log.Printf("Strategy registered: %s for %s", name, s.GetSymbol())
}

// Start starts the trading bot
func (b *TradingBot) Start() error {
	log.Println("Trading Bot started")
	log.Printf("Dry run mode: %v", b.config.TradingConfig.DryRun)
	log.Printf("Max open positions: %d", b.config.TradingConfig.MaxOpenPositions)

	// Start strategy evaluation loop
	for name, strat := range b.strategies {
		b.wg.Add(1)
		go b.runStrategy(name, strat)
	}

	// Start position monitoring
	b.wg.Add(1)
	go b.monitorPositions()

	// Start virtual position monitoring for paper trading
	if b.config.TradingConfig.DryRun {
		b.wg.Add(1)
		go b.monitorVirtualPositions()
	}

	return nil
}

// runStrategy runs a single strategy
func (b *TradingBot) runStrategy(name string, strat strategy.Strategy) {
	defer b.wg.Done()

	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := b.evaluateStrategy(name, strat); err != nil {
				log.Printf("Error evaluating strategy %s: %v", name, err)
			}
		case <-b.stopChan:
			return
		}
	}
}

// evaluateStrategy evaluates a strategy and places orders if conditions are met
func (b *TradingBot) evaluateStrategy(name string, strat strategy.Strategy) error {
	// Check if strategy is enabled
	b.mu.RLock()
	enabled, exists := b.enabledStrategies[name]
	openPositions := len(b.positions)
	b.mu.RUnlock()

	if !exists || !enabled {
		return nil // Strategy is disabled, skip evaluation
	}

	if openPositions >= b.config.TradingConfig.MaxOpenPositions {
		return nil
	}

	// Check if we already have a position for this symbol
	if b.hasPosition(strat.GetSymbol()) {
		return nil
	}

	// Fetch klines
	klines, err := b.client.GetKlines(strat.GetSymbol(), strat.GetInterval(), 50)
	if err != nil {
		return fmt.Errorf("error fetching klines: %w", err)
	}

	// Get current price
	currentPrice, err := b.client.GetCurrentPrice(strat.GetSymbol())
	if err != nil {
		return fmt.Errorf("error fetching current price: %w", err)
	}

	// Evaluate strategy
	signal, err := strat.Evaluate(klines, currentPrice)
	if err != nil {
		return fmt.Errorf("error evaluating strategy: %w", err)
	}

	// Check if there's a signal
	if signal.Type == strategy.SignalNone {
		return nil
	}

	log.Printf("Signal detected: %s - %s - %s", name, signal.Symbol, signal.Reason)

	// Publish signal event
	if b.eventBus != nil {
		b.eventBus.Publish(events.Event{
			Type: events.EventSignalGenerated,
			Data: map[string]interface{}{
				"strategy":    name,
				"symbol":      signal.Symbol,
				"signal_type": signal.Side,
				"price":       signal.EntryPrice,
				"reason":      signal.Reason,
			},
			Timestamp: time.Now(),
		})
	}

	// Place order
	if err := b.executeSignal(signal); err != nil {
		return fmt.Errorf("error executing signal: %w", err)
	}

	return nil
}

// executeSignal executes a trading signal
func (b *TradingBot) executeSignal(signal *strategy.Signal) error {
	if b.config.TradingConfig.DryRun {
		log.Printf("DRY RUN - Would place %s order for %s at %.4f", signal.Side, signal.Symbol, signal.EntryPrice)
		log.Printf("DRY RUN - Stop Loss: %.4f | Take Profit: %.4f", signal.StopLoss, signal.TakeProfit)

		// Create virtual trade in database for paper trading
		return b.createVirtualTrade(signal)
	}

	// Calculate quantity based on risk management
	// This is a simplified version - in production, you'd want more sophisticated position sizing
	quantity := b.calculatePositionSize(signal)

	// Place market order
	params := map[string]string{
		"symbol":   signal.Symbol,
		"side":     signal.Side,
		"type":     "MARKET",
		"quantity": fmt.Sprintf("%.8f", quantity),
	}

	orderResp, err := b.client.PlaceOrder(params)
	if err != nil {
		return fmt.Errorf("error placing order: %w", err)
	}

	log.Printf("Order placed: %s %s %.8f @ %.4f", signal.Side, signal.Symbol, quantity, orderResp.Price)

	// Create position
	position := &Position{
		Symbol:     signal.Symbol,
		EntryPrice: orderResp.Price,
		Quantity:   quantity,
		StopLoss:   signal.StopLoss,
		TakeProfit: signal.TakeProfit,
		Side:       signal.Side,
		EntryTime:  time.Now(),
	}

	// Place stop loss order
	slParams := map[string]string{
		"symbol":    signal.Symbol,
		"side":      getOppositeSide(signal.Side),
		"type":      "STOP_LOSS_LIMIT",
		"quantity":  fmt.Sprintf("%.8f", quantity),
		"price":     fmt.Sprintf("%.4f", signal.StopLoss),
		"stopPrice": fmt.Sprintf("%.4f", signal.StopLoss),
		"timeInForce": "GTC",
	}

	slResp, err := b.client.PlaceOrder(slParams)
	if err != nil {
		log.Printf("Warning: Failed to place stop loss: %v", err)
	} else {
		position.StopLossID = slResp.OrderId
		log.Printf("Stop loss placed at %.4f", signal.StopLoss)
	}

	// Place take profit order
	tpParams := map[string]string{
		"symbol":   signal.Symbol,
		"side":     getOppositeSide(signal.Side),
		"type":     "LIMIT",
		"quantity": fmt.Sprintf("%.8f", quantity),
		"price":    fmt.Sprintf("%.4f", signal.TakeProfit),
		"timeInForce": "GTC",
	}

	tpResp, err := b.client.PlaceOrder(tpParams)
	if err != nil {
		log.Printf("Warning: Failed to place take profit: %v", err)
	} else {
		position.TakeProfitID = tpResp.OrderId
		log.Printf("Take profit placed at %.4f", signal.TakeProfit)
	}

	// Store position
	b.mu.Lock()
	b.positions[signal.Symbol] = position
	b.mu.Unlock()

	return nil
}

// monitorPositions monitors open positions
func (b *TradingBot) monitorPositions() {
	defer b.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.checkPositions()
		case <-b.stopChan:
			return
		}
	}
}

// checkPositions checks the status of all open positions
func (b *TradingBot) checkPositions() {
	b.mu.RLock()
	positions := make([]*Position, 0, len(b.positions))
	for _, pos := range b.positions {
		positions = append(positions, pos)
	}
	b.mu.RUnlock()

	for _, pos := range positions {
		currentPrice, err := b.client.GetCurrentPrice(pos.Symbol)
		if err != nil {
			log.Printf("Error fetching price for %s: %v", pos.Symbol, err)
			continue
		}

		// Calculate P&L
		var pnlPercent float64
		if pos.Side == "BUY" {
			pnlPercent = ((currentPrice - pos.EntryPrice) / pos.EntryPrice) * 100
		} else {
			pnlPercent = ((pos.EntryPrice - currentPrice) / pos.EntryPrice) * 100
		}

		log.Printf("Position %s: Entry %.4f | Current %.4f | P&L: %.2f%%",
			pos.Symbol, pos.EntryPrice, currentPrice, pnlPercent)
	}
}

// calculatePositionSize calculates the position size based on risk management
func (b *TradingBot) calculatePositionSize(signal *strategy.Signal) float64 {
	// This is a simplified calculation
	// In production, you'd fetch account balance and calculate based on risk per trade
	return 0.001 // Default small position
}

// hasPosition checks if there's an open position for a symbol
func (b *TradingBot) hasPosition(symbol string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, exists := b.positions[symbol]
	return exists
}

// createVirtualTrade creates a virtual trade for paper trading in dry run mode
func (b *TradingBot) createVirtualTrade(signal *strategy.Signal) error {
	if b.repo == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Calculate quantity (0.001 BTC or equivalent for now)
	quantity := 0.001

	// Create trade record
	trade := &database.Trade{
		Symbol:       signal.Symbol,
		Side:         signal.Side,
		EntryPrice:   signal.EntryPrice,
		Quantity:     quantity,
		StopLoss:     &signal.StopLoss,
		TakeProfit:   &signal.TakeProfit,
		Status:       "OPEN",
		EntryTime:    time.Now(),
		StrategyName: &signal.Reason,
	}

	if err := b.repo.CreateTrade(ctx, trade); err != nil {
		log.Printf("Failed to create virtual trade: %v", err)
		return err
	}

	// Store position in memory for tracking
	position := &Position{
		Symbol:     signal.Symbol,
		EntryPrice: signal.EntryPrice,
		Quantity:   quantity,
		StopLoss:   signal.StopLoss,
		TakeProfit: signal.TakeProfit,
		Side:       signal.Side,
		EntryTime:  time.Now(),
	}

	b.mu.Lock()
	b.positions[signal.Symbol] = position
	b.mu.Unlock()

	log.Printf("Virtual trade created: %s %s %.8f @ %.4f (ID: %d)", signal.Side, signal.Symbol, quantity, signal.EntryPrice, trade.ID)

	// Publish trade opened event
	if b.eventBus != nil {
		b.eventBus.Publish(events.Event{
			Type: events.EventTradeOpened,
			Data: map[string]interface{}{
				"symbol":      signal.Symbol,
				"side":        signal.Side,
				"entry_price": signal.EntryPrice,
				"quantity":    quantity,
				"stop_loss":   signal.StopLoss,
				"take_profit": signal.TakeProfit,
			},
			Timestamp: time.Now(),
		})
	}

	return nil
}

// GetOpenVirtualTrades returns all open virtual trades from database
func (b *TradingBot) GetOpenVirtualTrades() []map[string]interface{} {
	if b.repo == nil {
		return []map[string]interface{}{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	trades, err := b.repo.GetOpenTrades(ctx)
	if err != nil {
		log.Printf("Failed to get open trades: %v", err)
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, 0, len(trades))
	for _, trade := range trades {
		// Get current price for P&L calculation
		currentPrice, err := b.client.GetCurrentPrice(trade.Symbol)
		if err != nil {
			currentPrice = trade.EntryPrice
		}

		// Calculate P&L
		var pnl, pnlPercent float64
		if trade.Side == "BUY" {
			pnl = (currentPrice - trade.EntryPrice) * trade.Quantity
			pnlPercent = ((currentPrice - trade.EntryPrice) / trade.EntryPrice) * 100
		} else {
			pnl = (trade.EntryPrice - currentPrice) * trade.Quantity
			pnlPercent = ((trade.EntryPrice - currentPrice) / trade.EntryPrice) * 100
		}

		result = append(result, map[string]interface{}{
			"symbol":        trade.Symbol,
			"side":          trade.Side,
			"entry_price":   trade.EntryPrice,
			"current_price": currentPrice,
			"quantity":      trade.Quantity,
			"pnl":           pnl,
			"pnl_percent":   pnlPercent,
			"entry_time":    trade.EntryTime,
			"stop_loss":     trade.StopLoss,
			"take_profit":   trade.TakeProfit,
		})
	}

	return result
}

// monitorVirtualPositions monitors virtual positions in dry run mode
func (b *TradingBot) monitorVirtualPositions() {
	defer b.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if b.config.TradingConfig.DryRun {
				b.checkVirtualPositions()
			}
		case <-b.stopChan:
			return
		}
	}
}

// checkVirtualPositions checks virtual positions and closes them if stop loss or take profit is hit
func (b *TradingBot) checkVirtualPositions() {
	if b.repo == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	trades, err := b.repo.GetOpenTrades(ctx)
	if err != nil {
		log.Printf("Error fetching open trades: %v", err)
		return
	}

	for _, trade := range trades {
		currentPrice, err := b.client.GetCurrentPrice(trade.Symbol)
		if err != nil {
			log.Printf("Error fetching price for %s: %v", trade.Symbol, err)
			continue
		}

		// Calculate P&L
		var pnl, pnlPercent float64
		shouldClose := false
		closeReason := ""

		if trade.Side == "BUY" {
			pnl = (currentPrice - trade.EntryPrice) * trade.Quantity
			pnlPercent = ((currentPrice - trade.EntryPrice) / trade.EntryPrice) * 100

			// Check stop loss
			if trade.StopLoss != nil && currentPrice <= *trade.StopLoss {
				shouldClose = true
				closeReason = "STOP_LOSS"
			}
			// Check take profit
			if trade.TakeProfit != nil && currentPrice >= *trade.TakeProfit {
				shouldClose = true
				closeReason = "TAKE_PROFIT"
			}
		} else {
			pnl = (trade.EntryPrice - currentPrice) * trade.Quantity
			pnlPercent = ((trade.EntryPrice - currentPrice) / trade.EntryPrice) * 100

			// Check stop loss
			if trade.StopLoss != nil && currentPrice >= *trade.StopLoss {
				shouldClose = true
				closeReason = "STOP_LOSS"
			}
			// Check take profit
			if trade.TakeProfit != nil && currentPrice <= *trade.TakeProfit {
				shouldClose = true
				closeReason = "TAKE_PROFIT"
			}
		}

		if shouldClose {
			// Close the virtual trade
			trade.ExitPrice = &currentPrice
			trade.ExitTime = timePtr(time.Now())
			trade.PnL = &pnl
			trade.PnLPercent = &pnlPercent
			trade.Status = "CLOSED"

			if err := b.repo.UpdateTrade(ctx, trade); err != nil {
				log.Printf("Failed to close trade for %s: %v", trade.Symbol, err)
				continue
			}

			log.Printf("Virtual trade closed: %s - %s at %.4f (Entry: %.4f, P&L: %.2f%%)",
				trade.Symbol, closeReason, currentPrice, trade.EntryPrice, pnlPercent)

			// Remove from in-memory positions
			b.mu.Lock()
			delete(b.positions, trade.Symbol)
			b.mu.Unlock()

			// Publish trade closed event
			if b.eventBus != nil {
				b.eventBus.Publish(events.Event{
					Type: events.EventTradeClosed,
					Data: map[string]interface{}{
						"symbol":      trade.Symbol,
						"exit_price":  currentPrice,
						"pnl":         pnl,
						"pnl_percent": pnlPercent,
						"reason":      closeReason,
					},
					Timestamp: time.Now(),
				})
			}
		}
	}
}

// EnableStrategy enables a strategy
func (b *TradingBot) EnableStrategy(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.strategies[name]; !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	b.enabledStrategies[name] = true
	log.Printf("Strategy %s enabled", name)
	return nil
}

// DisableStrategy disables a strategy
func (b *TradingBot) DisableStrategy(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.strategies[name]; !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	b.enabledStrategies[name] = false
	log.Printf("Strategy %s disabled", name)
	return nil
}

// GetStrategyInfo returns information about all registered strategies
func (b *TradingBot) GetStrategyInfo() []map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(b.strategies))
	for name, strat := range b.strategies {
		enabled := b.enabledStrategies[name]
		result = append(result, map[string]interface{}{
			"name":     name,
			"symbol":   strat.GetSymbol(),
			"interval": strat.GetInterval(),
			"enabled":  enabled,
		})
	}

	return result
}

// IsStrategyEnabled checks if a strategy is enabled
func (b *TradingBot) IsStrategyEnabled(name string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.enabledStrategies[name]
}

// Stop stops the trading bot
func (b *TradingBot) Stop() {
	close(b.stopChan)
	b.wg.Wait()
	log.Println("Trading Bot stopped")
}

func getOppositeSide(side string) string {
	if side == "BUY" {
		return "SELL"
	}
	return "BUY"
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// ExecutePendingSignal executes a manually confirmed pending signal
func (b *TradingBot) ExecutePendingSignal(pendingSignal *database.PendingSignal) error {
	// Convert PendingSignal to Signal
	signal := &strategy.Signal{
		Type:       strategy.SignalType(pendingSignal.SignalType),
		Symbol:     pendingSignal.Symbol,
		EntryPrice: pendingSignal.EntryPrice,
		Side:       pendingSignal.SignalType,
		Timestamp:  pendingSignal.Timestamp,
	}

	if pendingSignal.StopLoss != nil {
		signal.StopLoss = *pendingSignal.StopLoss
	}
	if pendingSignal.TakeProfit != nil {
		signal.TakeProfit = *pendingSignal.TakeProfit
	}
	if pendingSignal.Reason != nil {
		signal.Reason = *pendingSignal.Reason
	}

	// Execute the signal
	log.Printf("Executing manually confirmed signal: %s %s at %.4f", signal.Side, signal.Symbol, signal.EntryPrice)
	return b.executeSignal(signal)
}
