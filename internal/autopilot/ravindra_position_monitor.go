// Package autopilot implements the Ravindra Position Monitor for managing
// stop-loss updates according to Ravindra's trailing stop strategy.
//
// Milestones (R:R based):
// - At 1:2 R:R → Move SL to entry (breakeven, zero risk)
// - At 1:3 R:R → Move SL to 1:1 level (lock profit)
// - At 1:4 R:R → Take profit (target reached)
package autopilot

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/orders"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// RavindraPositionMonitorConfig holds configuration for the monitor
type RavindraPositionMonitorConfig struct {
	// CheckInterval is how often to check positions for SL updates (default: 5s)
	CheckInterval time.Duration
	// PriceStaleThreshold is how old a price can be before considered stale (default: 30s)
	PriceStaleThreshold time.Duration
	// MaxRetries for Binance API calls (default: 3)
	MaxRetries int
	// RetryDelay between retries (default: 1s)
	RetryDelay time.Duration
	// DebugMode enables verbose logging
	DebugMode bool
}

// DefaultRavindraPositionMonitorConfig returns the default configuration
func DefaultRavindraPositionMonitorConfig() *RavindraPositionMonitorConfig {
	return &RavindraPositionMonitorConfig{
		CheckInterval:       5 * time.Second,
		PriceStaleThreshold: 30 * time.Second,
		MaxRetries:          3,
		RetryDelay:          time.Second,
		DebugMode:           false,
	}
}

// RavindraPosition represents a position being monitored
type RavindraPosition struct {
	// Identity
	ChainID string `json:"chain_id"`
	Symbol  string `json:"symbol"`
	UserID  string `json:"user_id"`

	// Position details
	Side         string  `json:"side"` // LONG or SHORT
	EntryPrice   float64 `json:"entry_price"`
	Quantity     float64 `json:"quantity"`
	InitialSL    float64 `json:"initial_sl"`
	InitialTP    float64 `json:"initial_tp"`
	CurrentSL    float64 `json:"current_sl"`
	CurrentTP    float64 `json:"current_tp"`
	SLOrderID    int64   `json:"sl_order_id"`
	TPOrderID    int64   `json:"tp_order_id"`
	IsAlgoOrder  bool    `json:"is_algo_order"`

	// Trailing stop state
	TrailingStop *TrailingStopManager `json:"trailing_stop"`

	// Timestamps
	EnteredAt time.Time `json:"entered_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RavindraPositionMonitor monitors Ravindra strategy positions and updates SL
// according to R:R milestones
type RavindraPositionMonitor struct {
	// Dependencies
	futuresClient    binance.FuturesClient
	chainEventWriter *orders.ChainEventWriter
	priceProvider    PriceProvider

	// Configuration
	config *RavindraPositionMonitorConfig

	// Active positions by chainID
	positions map[string]*RavindraPosition
	mu        sync.RWMutex

	// Runtime state
	running  bool
	stopChan chan struct{}

	// Statistics
	stats RavindraMonitorStats
}

// PriceProvider interface for getting current prices
type PriceProvider interface {
	GetMarkPrice(symbol string) (float64, error)
}

// RavindraMonitorStats holds statistics
type RavindraMonitorStats struct {
	TotalChecks       int64     `json:"total_checks"`
	BreakevenMoves    int64     `json:"breakeven_moves"`
	OneRMoves         int64     `json:"one_r_moves"`
	TakeProfitHits    int64     `json:"take_profit_hits"`
	FailedUpdates     int64     `json:"failed_updates"`
	LastCheckTime     time.Time `json:"last_check_time"`
	LastUpdateTime    time.Time `json:"last_update_time"`
	ActivePositions   int       `json:"active_positions"`
}

// NewRavindraPositionMonitor creates a new monitor instance
func NewRavindraPositionMonitor(
	futuresClient binance.FuturesClient,
	chainEventWriter *orders.ChainEventWriter,
	priceProvider PriceProvider,
	config *RavindraPositionMonitorConfig,
) *RavindraPositionMonitor {
	if config == nil {
		config = DefaultRavindraPositionMonitorConfig()
	}

	return &RavindraPositionMonitor{
		futuresClient:    futuresClient,
		chainEventWriter: chainEventWriter,
		priceProvider:    priceProvider,
		config:           config,
		positions:        make(map[string]*RavindraPosition),
		stopChan:         make(chan struct{}),
	}
}

// Start begins the position monitor background loop
func (m *RavindraPositionMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	log.Printf("[RAVINDRA-MONITOR] Position monitor started")
	go m.runLoop()
}

// Stop halts the position monitor
func (m *RavindraPositionMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)
	log.Printf("[RAVINDRA-MONITOR] Position monitor stopped")
}

// IsRunning returns whether the monitor is active
func (m *RavindraPositionMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// AddPosition adds a new position to monitor
func (m *RavindraPositionMonitor) AddPosition(pos *RavindraPosition) error {
	if pos == nil || pos.ChainID == "" {
		return fmt.Errorf("invalid position: nil or missing chain ID")
	}

	// Initialize trailing stop manager if not set
	if pos.TrailingStop == nil {
		pos.TrailingStop = NewTrailingStopManager(
			pos.EntryPrice,
			pos.InitialSL,
			pos.InitialTP,
			nil, // Use defaults
		)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.positions[pos.ChainID] = pos
	m.stats.ActivePositions = len(m.positions)

	log.Printf("[RAVINDRA-MONITOR] Added position %s for %s: entry=%.4f, SL=%.4f, TP=%.4f",
		pos.ChainID, pos.Symbol, pos.EntryPrice, pos.CurrentSL, pos.CurrentTP)

	return nil
}

// RemovePosition removes a position from monitoring
func (m *RavindraPositionMonitor) RemovePosition(chainID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.positions[chainID]; exists {
		delete(m.positions, chainID)
		m.stats.ActivePositions = len(m.positions)
		log.Printf("[RAVINDRA-MONITOR] Removed position %s from monitoring", chainID)
	}
}

// GetPosition returns a position by chain ID
func (m *RavindraPositionMonitor) GetPosition(chainID string) *RavindraPosition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.positions[chainID]
}

// GetStats returns current statistics
func (m *RavindraPositionMonitor) GetStats() RavindraMonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// runLoop is the main monitoring loop
func (m *RavindraPositionMonitor) runLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkAllPositions()
		}
	}
}

// checkAllPositions checks all monitored positions for SL updates
func (m *RavindraPositionMonitor) checkAllPositions() {
	m.mu.RLock()
	positions := make([]*RavindraPosition, 0, len(m.positions))
	for _, pos := range m.positions {
		positions = append(positions, pos)
	}
	m.mu.RUnlock()

	if len(positions) == 0 {
		return
	}

	m.stats.TotalChecks++
	m.stats.LastCheckTime = time.Now()

	for _, pos := range positions {
		m.checkPosition(pos)
	}
}

// checkPosition checks a single position and updates SL if needed
func (m *RavindraPositionMonitor) checkPosition(pos *RavindraPosition) {
	if pos.TrailingStop == nil {
		return
	}

	// Get current price
	currentPrice, err := m.priceProvider.GetMarkPrice(pos.Symbol)
	if err != nil {
		if m.config.DebugMode {
			log.Printf("[RAVINDRA-MONITOR] Failed to get price for %s: %v", pos.Symbol, err)
		}
		return
	}

	// Check trailing stop milestones
	newSL, action := pos.TrailingStop.Update(currentPrice)

	switch action {
	case "HOLD":
		// No action needed
		if m.config.DebugMode {
			log.Printf("[RAVINDRA-MONITOR] %s: price=%.4f, RR=%.2f, SL=%.4f - HOLD",
				pos.Symbol, currentPrice, pos.TrailingStop.CurrentRR, pos.CurrentSL)
		}

	case "MOVE_TO_BREAKEVEN":
		log.Printf("[RAVINDRA-MONITOR] %s: 1:2 R:R reached! Moving SL to breakeven (%.4f -> %.4f)",
			pos.Symbol, pos.CurrentSL, newSL)
		m.executeSLUpdate(pos, newSL, "Move to breakeven at 1:2 R:R", orders.ModSourceTrailingStop)

	case "MOVE_TO_1R":
		log.Printf("[RAVINDRA-MONITOR] %s: 1:3 R:R reached! Moving SL to 1:1 level (%.4f -> %.4f)",
			pos.Symbol, pos.CurrentSL, newSL)
		m.executeSLUpdate(pos, newSL, "Move to 1:1 level at 1:3 R:R", orders.ModSourceTrailingStop)

	case "TAKE_PROFIT":
		log.Printf("[RAVINDRA-MONITOR] %s: 1:4 R:R target reached! Price=%.4f, TP=%.4f",
			pos.Symbol, currentPrice, pos.CurrentTP)
		m.stats.TakeProfitHits++
		// TP order on Binance will handle the exit automatically
	}
}

// executeSLUpdate updates the stop loss on Binance
func (m *RavindraPositionMonitor) executeSLUpdate(pos *RavindraPosition, newSLPrice float64, reason string, source orders.ModificationSource) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oldSL := pos.CurrentSL

	// Round to tick size
	tickSize := getTickSizePC(pos.Symbol)
	newSLPrice = roundToTickSize(newSLPrice, tickSize)

	// Cancel existing SL order
	if pos.SLOrderID > 0 {
		if err := m.cancelOrderWithRetry(ctx, pos.Symbol, pos.SLOrderID, pos.IsAlgoOrder); err != nil {
			log.Printf("[RAVINDRA-MONITOR] Warning: failed to cancel old SL for %s: %v", pos.Symbol, err)
		}
	}

	// Place new SL order
	newOrderID, err := m.placeStopLossOrder(ctx, pos.Symbol, pos.Side, pos.Quantity, newSLPrice)
	if err != nil {
		log.Printf("[RAVINDRA-MONITOR] Failed to place new SL for %s: %v", pos.Symbol, err)
		m.stats.FailedUpdates++
		return
	}

	// Update position state
	pos.SLOrderID = newOrderID
	pos.CurrentSL = newSLPrice
	pos.UpdatedAt = time.Now()

	// Record modification event
	if m.chainEventWriter != nil && pos.ChainID != "" {
		m.chainEventWriter.RecordSLModified(ctx, pos.ChainID, orders.ChainSLModifiedEvent{
			BinanceOrderID:     newOrderID,
			OldPrice:           oldSL,
			NewPrice:           newSLPrice,
			ModificationSource: source,
			ModificationReason: reason,
			MarketPrice:        pos.TrailingStop.HighestPrice,
			BinanceTimestamp:   time.Now().UnixMilli(),
		})
	}

	// Update statistics
	m.stats.LastUpdateTime = time.Now()
	if pos.TrailingStop.MovedTo1R {
		m.stats.OneRMoves++
	} else if pos.TrailingStop.MovedToBreakeven {
		m.stats.BreakevenMoves++
	}

	log.Printf("[RAVINDRA-MONITOR] %s: SL updated successfully %.4f -> %.4f (order=%d, reason=%s)",
		pos.Symbol, oldSL, newSLPrice, newOrderID, reason)
}

// cancelOrderWithRetry cancels an order with retries
// Note: ctx is kept for future use but current Binance client doesn't use it
func (m *RavindraPositionMonitor) cancelOrderWithRetry(ctx context.Context, symbol string, orderID int64, isAlgoOrder bool) error {
	_ = ctx // Reserved for future use
	var lastErr error

	for attempt := 0; attempt < m.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(m.config.RetryDelay)
		}

		var err error
		if isAlgoOrder {
			err = m.futuresClient.CancelAlgoOrder(symbol, orderID)
		} else {
			err = m.futuresClient.CancelFuturesOrder(symbol, orderID)
		}

		if err == nil {
			return nil
		}

		lastErr = err
		log.Printf("[RAVINDRA-MONITOR] Cancel order attempt %d failed for %s (order=%d): %v",
			attempt+1, symbol, orderID, err)
	}

	return fmt.Errorf("failed to cancel order after %d attempts: %v", m.config.MaxRetries, lastErr)
}

// placeStopLossOrder places a new stop loss order
// Note: ctx is kept for future use but current Binance client doesn't use it
func (m *RavindraPositionMonitor) placeStopLossOrder(ctx context.Context, symbol, side string, quantity, stopPrice float64) (int64, error) {
	_ = ctx // Reserved for future use

	// Determine close side (opposite of position side)
	closeSide := "BUY"
	positionSide := binance.PositionSideShort
	if side == "LONG" {
		closeSide = "SELL"
		positionSide = binance.PositionSideLong
	}

	// Place STOP_MARKET algo order
	params := binance.AlgoOrderParams{
		Symbol:       symbol,
		Side:         closeSide,
		PositionSide: positionSide,
		Type:         binance.FuturesOrderTypeStopMarket,
		Quantity:     quantity,
		TriggerPrice: stopPrice,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	resp, err := m.futuresClient.PlaceAlgoOrder(params)
	if err != nil {
		return 0, fmt.Errorf("failed to place SL order: %w", err)
	}

	return resp.AlgoId, nil
}

// getTickSizeForSymbol returns the tick size for a symbol
// Uses the getTickSizePC function from position_controller.go
func getTickSizeForSymbol(symbol string) float64 {
	return getTickSizePC(symbol)
}

// ========== Integration Helper ==========

// CreateRavindraPositionFromChainEntry creates a RavindraPosition from chain entry data
func CreateRavindraPositionFromChainEntry(
	chainID string,
	symbol string,
	userID string,
	side string,
	entryPrice float64,
	quantity float64,
	slPrice float64,
	tpPrice float64,
	slOrderID int64,
	tpOrderID int64,
	config *VolumeImbalanceConfig,
) *RavindraPosition {
	pos := &RavindraPosition{
		ChainID:     chainID,
		Symbol:      symbol,
		UserID:      userID,
		Side:        side,
		EntryPrice:  entryPrice,
		Quantity:    quantity,
		InitialSL:   slPrice,
		InitialTP:   tpPrice,
		CurrentSL:   slPrice,
		CurrentTP:   tpPrice,
		SLOrderID:   slOrderID,
		TPOrderID:   tpOrderID,
		IsAlgoOrder: true, // Binance Dec 2024 requires algo orders for conditional
		EnteredAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Initialize trailing stop manager
	pos.TrailingStop = NewTrailingStopManager(entryPrice, slPrice, tpPrice, config)

	return pos
}
