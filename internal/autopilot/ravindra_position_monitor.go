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
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/orders"
	"context"
	"fmt"
	"log"
	"math"
	"strings"
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
		DebugMode:           true,
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

	// Precision for Binance API (prevents -1111 errors on SL replacement orders)
	PricePrecision    int `json:"price_precision"`    // Price precision for the symbol
	QuantityPrecision int `json:"quantity_precision"` // Quantity precision for the symbol

	// Modification tracking
	ModificationCount int `json:"modification_count"`

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

	// Deduplication: Check if a position with the same symbol+side already exists under a different chainID.
	// If so, remove the old entry to prevent duplicate monitoring of the same position.
	for existingChainID, existingPos := range m.positions {
		if existingChainID != pos.ChainID && existingPos.Symbol == pos.Symbol && existingPos.Side == pos.Side {
			log.Printf("[RAVINDRA-MONITOR] Removing duplicate position %s for %s/%s (replaced by %s)",
				existingChainID, existingPos.Symbol, existingPos.Side, pos.ChainID)
			delete(m.positions, existingChainID)
		}
	}

	m.positions[pos.ChainID] = pos
	m.stats.ActivePositions = len(m.positions)

	log.Printf("[RAVINDRA-MONITOR] Added position %s for %s: entry=%.4f, SL=%.4f, TP=%.4f",
		pos.ChainID, pos.Symbol, pos.EntryPrice, pos.CurrentSL, pos.CurrentTP)

	return nil
}

// ReRegisterFromChain registers an existing position from an active order chain.
// This is used on startup to re-register positions that were being tracked before restart.
// It reconstructs the RavindraPosition from chain data and current market state.
func (m *RavindraPositionMonitor) ReRegisterFromChain(chainID, symbol, userID, side string,
	entryPrice, quantity, slPrice, tpPrice float64,
	slAlgoID, tpAlgoID int64, timeframe string,
	pricePrecision, quantityPrecision int) error {
	if chainID == "" || symbol == "" {
		return fmt.Errorf("chainID and symbol are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Don't re-register if already tracked with this exact chainID
	if _, exists := m.positions[chainID]; exists {
		return nil
	}

	direction := "LONG"
	if side == "SHORT" || side == "SELL" {
		direction = "SHORT"
	}

	// Deduplication: Remove any existing position with the same symbol+side but different chainID.
	// This prevents double-monitoring if startup paths register the same position twice under different chains.
	for existingChainID, existingPos := range m.positions {
		if existingChainID != chainID && existingPos.Symbol == symbol && existingPos.Side == direction {
			log.Printf("[RAVINDRA-MONITOR] ReRegister: removing stale position %s for %s/%s (replaced by %s)",
				existingChainID, existingPos.Symbol, existingPos.Side, chainID)
			delete(m.positions, existingChainID)
		}
	}

	riskAmount := math.Abs(entryPrice - slPrice)
	if riskAmount <= 0 {
		riskAmount = entryPrice * 0.01 // Fallback: 1% risk
	}

	pos := &RavindraPosition{
		ChainID:           chainID,
		Symbol:            symbol,
		UserID:            userID,
		Side:              direction,
		EntryPrice:        entryPrice,
		Quantity:          quantity,
		CurrentSL:         slPrice,
		InitialSL:         slPrice,
		CurrentTP:         tpPrice,
		InitialTP:         tpPrice,
		SLOrderID:         slAlgoID,
		TPOrderID:         tpAlgoID,
		IsAlgoOrder:       true,
		PricePrecision:    pricePrecision,
		QuantityPrecision: quantityPrecision,
		EnteredAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Create trailing stop manager with EXPLICIT direction.
	// When SL has been tightened past entry (e.g. after breakeven move), the direction
	// cannot be inferred from SL position. We pass the actual chain direction.
	pos.TrailingStop = NewTrailingStopManager(entryPrice, slPrice, tpPrice, nil, direction)

	// Check if breakeven was likely already hit based on current SL position
	// If SL is at or above entry (LONG) or at or below entry (SHORT), mark breakeven as done
	if direction == "LONG" && slPrice >= entryPrice {
		pos.TrailingStop.MovedToBreakeven = true
	} else if direction == "SHORT" && slPrice <= entryPrice {
		pos.TrailingStop.MovedToBreakeven = true
	}

	// Check if 1:1 milestone was also passed (SL is at or above entry + risk for LONG)
	if pos.TrailingStop.MovedToBreakeven {
		oneRLevel := entryPrice + pos.TrailingStop.RiskAmount
		if direction == "SHORT" {
			oneRLevel = entryPrice - pos.TrailingStop.RiskAmount
		}
		if (direction == "LONG" && slPrice >= oneRLevel) || (direction == "SHORT" && slPrice <= oneRLevel) {
			pos.TrailingStop.MovedTo1R = true
		}
	}

	m.positions[chainID] = pos
	m.stats.ActivePositions = len(m.positions)
	log.Printf("[RAVINDRA-MONITOR] Re-registered position from chain: chainID=%s, symbol=%s, direction=%s, entry=%.6f, SL=%.6f, TP=%.6f, pricePrecision=%d, qtyPrecision=%d",
		chainID, symbol, direction, entryPrice, slPrice, tpPrice, pricePrecision, quantityPrecision)

	return nil
}

// RemovePositionBySymbol removes a monitored position by symbol.
// Used when chain closes and we don't have the chainID in the callback context.
func (m *RavindraPositionMonitor) RemovePositionBySymbol(symbol string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for chainID, pos := range m.positions {
		if pos.Symbol == symbol {
			delete(m.positions, chainID)
			m.stats.ActivePositions = len(m.positions)
			log.Printf("[RAVINDRA-MONITOR] Removed position by symbol: chainID=%s, symbol=%s", chainID, symbol)
			return
		}
	}
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

// GetAllPositions returns all monitored positions (thread-safe copy)
func (m *RavindraPositionMonitor) GetAllPositions() map[string]*RavindraPosition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*RavindraPosition, len(m.positions))
	for k, v := range m.positions {
		result[k] = v
	}
	return result
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
			// Include milestone state for diagnosis when R:R is approaching thresholds
			log.Printf("[RAVINDRA-MONITOR] %s: price=%.4f, RR=%.2f, SL=%.4f, entry=%.4f, risk=%.4f, BE=%v, 1R=%v, BElvl=%.1f, 1Rlvl=%.1f - HOLD",
				pos.Symbol, currentPrice, pos.TrailingStop.CurrentRR, pos.CurrentSL,
				pos.TrailingStop.EntryPrice, pos.TrailingStop.RiskAmount,
				pos.TrailingStop.MovedToBreakeven, pos.TrailingStop.MovedTo1R,
				pos.TrailingStop.BreakevenRRLevel, pos.TrailingStop.OneRRLevel)
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

	log.Printf("[RAVINDRA-MONITOR] === SL MODIFICATION START === Symbol=%s ChainID=%s OldSL=%.4f NewSL=%.4f Reason=%s OldOrderID=%d IsAlgo=%v Precision=%d",
		pos.Symbol, pos.ChainID, oldSL, newSLPrice, reason, pos.SLOrderID, pos.IsAlgoOrder, pos.PricePrecision)

	// Resolve precision from exchange info if not set on the position.
	// Positions re-registered on startup have precision=0, which causes PlaceAlgoOrder
	// to default to 8 decimal places. For symbols like BNBUSDT (precision=2), Binance
	// rejects orders with -1111 "Precision is over the maximum".
	if pos.PricePrecision == 0 || pos.QuantityPrecision == 0 {
		m.resolvePrecision(pos)
	}

	// Round to tick size
	tickSize := getTickSizePC(pos.Symbol)
	newSLPrice = roundToTickSize(newSLPrice, tickSize)

	// Cancel existing SL order - ABORT if cancel fails to prevent duplicate orders
	if pos.SLOrderID > 0 {
		if err := m.cancelOrderWithRetry(ctx, pos.Symbol, pos.SLOrderID, pos.IsAlgoOrder); err != nil {
			log.Printf("[RAVINDRA-MONITOR] CRITICAL: Failed to cancel old SL for %s (order=%d): %v - ABORTING modification to prevent duplicates", pos.Symbol, pos.SLOrderID, err)
			m.stats.FailedUpdates++
			return
		}

		// Verification delay: wait 500ms then confirm old order is actually gone
		// Prevents duplicate SL orders when Binance cancel ACKs but order persists briefly
		time.Sleep(500 * time.Millisecond)
		openAlgos, verifyErr := m.futuresClient.GetOpenAlgoOrders(pos.Symbol)
		if verifyErr == nil {
			for _, algo := range openAlgos {
				if algo.AlgoId == pos.SLOrderID {
					log.Printf("[RAVINDRA-MONITOR] WARNING: Old SL order %d still present after cancel for %s - retrying cancel", pos.SLOrderID, pos.Symbol)
					if retryErr := m.cancelOrderWithRetry(ctx, pos.Symbol, pos.SLOrderID, pos.IsAlgoOrder); retryErr != nil {
						log.Printf("[RAVINDRA-MONITOR] CRITICAL: Retry cancel failed for old SL %d on %s: %v - ABORTING to prevent duplicates", pos.SLOrderID, pos.Symbol, retryErr)
						m.stats.FailedUpdates++
						return
					}
					// Brief additional wait after retry cancel
					time.Sleep(300 * time.Millisecond)
					break
				}
			}
		} else {
			log.Printf("[RAVINDRA-MONITOR] Could not verify SL cancellation for %s (API error): %v - proceeding cautiously", pos.Symbol, verifyErr)
		}
	}

	// Place new SL order (passes pos for precision fields)
	newOrderID, err := m.placeStopLossOrder(ctx, pos, newSLPrice)
	if err != nil {
		log.Printf("[RAVINDRA-MONITOR] Failed to place new SL for %s: %v", pos.Symbol, err)
		m.stats.FailedUpdates++
		return
	}

	// Update position state
	pos.SLOrderID = newOrderID
	pos.CurrentSL = newSLPrice
	pos.UpdatedAt = time.Now()
	pos.ModificationCount++

	// Record modification event and update SL binance order ID in DB
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

		// Update sl_binance_order_id in order_chains table so protection watchdog
		// can match this order when it receives ALGO_UPDATE events
		if err := m.chainEventWriter.GetDB().UpdateOrderChainSLDetails(
			ctx, pos.ChainID, newOrderID, newSLPrice, pos.Quantity,
		); err != nil {
			log.Printf("[RAVINDRA-MONITOR] Warning: failed to update SL details in DB for chain %s: %v", pos.ChainID, err)
		}
	}

	// Update statistics
	m.stats.LastUpdateTime = time.Now()
	if pos.TrailingStop.MovedTo1R {
		m.stats.OneRMoves++
	} else if pos.TrailingStop.MovedToBreakeven {
		m.stats.BreakevenMoves++
	}

	log.Printf("[RAVINDRA-MONITOR] === SL MODIFICATION SUCCESS === Symbol=%s NewSL=%.4f NewOrderID=%d ModCount=%d", pos.Symbol, newSLPrice, newOrderID, pos.ModificationCount)

	// Broadcast trailing SL update to frontend
	trailingStatus := pos.TrailingStop.GetStatus()
	events.BroadcastTrailingSLUpdate(pos.UserID, map[string]interface{}{
		"chain_id":              pos.ChainID,
		"symbol":                pos.Symbol,
		"action":                string(source),
		"reason":                reason,
		"old_sl_price":          oldSL,
		"new_sl_price":          newSLPrice,
		"modification_source":   string(source),
		"new_order_id":          newOrderID,
		"modification_count":    pos.ModificationCount,
		"entry_price":           pos.EntryPrice,
		"current_rr":            trailingStatus.CurrentRR,
		"at_breakeven":          trailingStatus.AtBreakeven,
		"at_1r":                 trailingStatus.At1R,
		"trailing_stop_status":  trailingStatus,
		"updated_at":            time.Now().UTC().Format(time.RFC3339),
	})
}

// cancelOrderWithRetry cancels an order with retries.
// If the order was already cancelled externally (not found on Binance), this returns nil
// to treat it as a success -- the order is already gone which is what we wanted.
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

		// Don't retry if order doesn't exist (already cancelled/filled externally)
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "unknown order") ||
			strings.Contains(errMsg, "order not found") ||
			strings.Contains(errMsg, "not found") ||
			strings.Contains(errMsg, "-2011") { // Binance error code for unknown order
			log.Printf("[RAVINDRA-MONITOR] Order %d for %s not found (likely cancelled externally), skipping cancel", orderID, symbol)
			return nil // Treat as success - order is already gone
		}

		log.Printf("[RAVINDRA-MONITOR] Cancel order attempt %d failed for %s (order=%d): %v",
			attempt+1, symbol, orderID, err)
	}

	return fmt.Errorf("failed to cancel order after %d attempts: %v", m.config.MaxRetries, lastErr)
}

// placeStopLossOrder places a new stop loss order with proper precision
// Note: ctx is kept for future use but current Binance client doesn't use it
func (m *RavindraPositionMonitor) placeStopLossOrder(ctx context.Context, pos *RavindraPosition, stopPrice float64) (int64, error) {
	_ = ctx // Reserved for future use

	// Determine close side (opposite of position side)
	closeSide := "BUY"
	positionSide := binance.PositionSideShort
	if pos.Side == "LONG" {
		closeSide = "SELL"
		positionSide = binance.PositionSideLong
	}

	// Build ClientAlgoId for chain-linked identification
	// Format: CHAINID-SL-M<timestamp> to avoid Binance rejecting duplicate clientAlgoId
	clientAlgoId := ""
	if pos.ChainID != "" {
		clientAlgoId = fmt.Sprintf("%s-SL-M%d", pos.ChainID, time.Now().UnixMilli())
	}

	// Calculate limit price with 0.2% buffer beyond trigger price to ensure fill.
	// LONG SL: selling below trigger → limit price slightly below trigger
	// SHORT SL: buying above trigger → limit price slightly above trigger
	limitPrice := stopPrice
	if pos.Side == "LONG" {
		limitPrice = stopPrice * 0.998 // 0.2% below trigger for LONG SL
	} else {
		limitPrice = stopPrice * 1.002 // 0.2% above trigger for SHORT SL
	}
	// Round limit price to tick size for precision
	slTickSize := getTickSizePC(pos.Symbol)
	limitPrice = roundToTickSize(limitPrice, slTickSize)

	// Place STOP (limit) algo order with precision fields to avoid Binance -1111 errors.
	// Using STOP instead of STOP_MARKET gives a limit fill price, avoiding slippage.
	params := binance.AlgoOrderParams{
		Symbol:            pos.Symbol,
		Side:              closeSide,
		PositionSide:      positionSide,
		Type:              binance.FuturesOrderTypeStop,
		Quantity:          pos.Quantity,
		TriggerPrice:      stopPrice,
		Price:             limitPrice,
		TimeInForce:       binance.TimeInForceGTC,
		WorkingType:       binance.WorkingTypeMarkPrice,
		PricePrecision:    pos.PricePrecision,
		QuantityPrecision: pos.QuantityPrecision,
		ClientAlgoId:      clientAlgoId,
	}

	resp, err := m.futuresClient.PlaceAlgoOrder(params)
	if err != nil {
		return 0, fmt.Errorf("failed to place SL order: %w", err)
	}

	return resp.AlgoId, nil
}

// resolvePrecision looks up price and quantity precision from exchange info.
// This is called lazily (only when needed for order placement) to avoid
// unnecessary API calls during position registration.
func (m *RavindraPositionMonitor) resolvePrecision(pos *RavindraPosition) {
	if m.futuresClient == nil {
		return
	}

	info, err := m.futuresClient.GetFuturesExchangeInfo()
	if err != nil {
		log.Printf("[RAVINDRA-MONITOR] Warning: failed to get exchange info for precision lookup: %v", err)
		return
	}

	for _, sym := range info.Symbols {
		if sym.Symbol == pos.Symbol {
			if sym.PricePrecision > 0 {
				pos.PricePrecision = sym.PricePrecision
			}
			if sym.QuantityPrecision > 0 {
				pos.QuantityPrecision = sym.QuantityPrecision
			}
			log.Printf("[RAVINDRA-MONITOR] Resolved precision for %s: price=%d, qty=%d",
				pos.Symbol, pos.PricePrecision, pos.QuantityPrecision)
			return
		}
	}

	// If not found, use safe defaults based on common pairs
	log.Printf("[RAVINDRA-MONITOR] Symbol %s not found in exchange info, using default precision=2", pos.Symbol)
	pos.PricePrecision = 2
	pos.QuantityPrecision = 3
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
	pricePrecision int,
	quantityPrecision int,
) *RavindraPosition {
	pos := &RavindraPosition{
		ChainID:           chainID,
		Symbol:            symbol,
		UserID:            userID,
		Side:              side,
		EntryPrice:        entryPrice,
		Quantity:          quantity,
		InitialSL:         slPrice,
		InitialTP:         tpPrice,
		CurrentSL:         slPrice,
		CurrentTP:         tpPrice,
		SLOrderID:         slOrderID,
		TPOrderID:         tpOrderID,
		IsAlgoOrder:       true, // Binance Dec 2024 requires algo orders for conditional
		PricePrecision:    pricePrecision,
		QuantityPrecision: quantityPrecision,
		EnteredAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Initialize trailing stop manager with explicit side
	pos.TrailingStop = NewTrailingStopManager(entryPrice, slPrice, tpPrice, config, side)

	return pos
}
