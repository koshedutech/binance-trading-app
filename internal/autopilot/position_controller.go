// Package autopilot provides the Position Controller for executing exit signals.
// Epic 10, Story 10.4: Position Controller - Exit Signal Executor
//
// The Position Controller is a lightweight component that:
// 1. Subscribes to Exit Decision Service signals
// 2. Executes SL/TP order updates on Binance
// 3. Provides protection heal - ensures SL/TP orders exist
// 4. Uses Chain system settings - NOT old Ginie mode settings
// 5. Enables complete disabling of Ginie for position management
package autopilot

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/exitdecision"
	"binance-trading-bot/internal/orders"
)

// LogPrefixPC is the standard prefix for Position Controller log messages.
const LogPrefixPC = "[POS-CTRL]"

// ============================================================================
// POSITION CONTROLLER TYPES
// ============================================================================

// ExitTypeExtended extends the exitdecision.ExitType with additional types.
type ExitTypeExtended string

const (
	// ExitTypeTrailSL indicates trailing stop loss update.
	ExitTypeTrailSL ExitTypeExtended = "trail_sl"
	// ExitTypeBreakeven indicates move to breakeven.
	ExitTypeBreakeven ExitTypeExtended = "breakeven"
	// ExitTypeUpdateTP indicates take profit update.
	ExitTypeUpdateTP ExitTypeExtended = "update_tp"
	// ExitTypeTrendReversal indicates trend reversal exit (tighten SL).
	ExitTypeTrendReversal ExitTypeExtended = "trend_reversal"
	// ExitTypeEfficiencyExit indicates efficiency-based exit (tighten SL).
	ExitTypeEfficiencyExit ExitTypeExtended = "efficiency_exit"
	// ExitTypeProtectionHeal indicates protection heal action.
	ExitTypeProtectionHeal ExitTypeExtended = "protection_heal"
)

// PositionControllerConfig holds configuration for the Position Controller.
type PositionControllerConfig struct {
	// Execution settings
	MaxRetries   int           `json:"max_retries"`   // Default: 3
	RetryDelay   time.Duration `json:"retry_delay"`   // Default: 1s
	OrderTimeout time.Duration `json:"order_timeout"` // Default: 10s

	// Protection heal settings
	HealCheckInterval    time.Duration `json:"heal_interval"` // Default: 30s
	EnableProtectionHeal bool          `json:"enable_heal"`   // Default: true

	// Signal processing
	SignalBatchSize    int           `json:"batch_size"`       // Default: 10
	ProcessingInterval time.Duration `json:"process_interval"` // Default: 100ms

	// Debug mode
	DebugMode bool `json:"debug_mode"` // Default: false
}

// DefaultPositionControllerConfig returns the default configuration.
func DefaultPositionControllerConfig() *PositionControllerConfig {
	return &PositionControllerConfig{
		MaxRetries:           3,
		RetryDelay:           time.Second,
		OrderTimeout:         10 * time.Second,
		HealCheckInterval:    30 * time.Second,
		EnableProtectionHeal: true,
		SignalBatchSize:      10,
		ProcessingInterval:   100 * time.Millisecond,
		DebugMode:            false,
	}
}

// PositionControllerStatus provides status information for the Position Controller.
type PositionControllerStatus struct {
	Running             bool      `json:"running"`
	StartedAt           time.Time `json:"started_at"`
	Uptime              string    `json:"uptime"`
	SignalsProcessed    int64     `json:"signals_processed"`
	SLUpdatesExecuted   int64     `json:"sl_updates"`
	TPUpdatesExecuted   int64     `json:"tp_updates"`
	HealActionsExecuted int64     `json:"heal_actions"`
	LastSignalTime      time.Time `json:"last_signal_time"`
	LastError           string    `json:"last_error,omitempty"`
}

// PositionInfo holds information about a position for order updates.
type PositionInfo struct {
	Symbol      string
	Side        string // "LONG" or "SHORT"
	EntryPrice  float64
	Quantity    float64
	SLPrice     float64
	TPPrice     float64
	SLOrderID   int64
	TPOrderID   int64
	ChainID     string
	IsAlgoOrder bool // True if using algo orders (Binance 2025-12 change)
}

// ============================================================================
// POSITION CONTROLLER
// ============================================================================

// PositionController executes exit signals from Exit Decision Service on Binance.
type PositionController struct {
	// Dependencies
	futuresClient    binance.FuturesClient
	exitDecisionSvc  *exitdecision.Service
	chainEventWriter *orders.ChainEventWriter

	// Configuration
	config *PositionControllerConfig
	userID string

	// Statistics (atomic for thread-safety)
	signalsProcessed    int64
	slUpdatesExecuted   int64
	tpUpdatesExecuted   int64
	healActionsExecuted int64
	lastSignalTime      time.Time
	lastError           string

	// State
	running   bool
	startedAt time.Time

	// Context for graceful shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc

	// Control
	stopChan chan struct{}

	// Synchronization
	mu sync.RWMutex
	wg sync.WaitGroup
}

// NewPositionController creates a new Position Controller.
func NewPositionController(
	futuresClient binance.FuturesClient,
	exitDecisionSvc *exitdecision.Service,
	chainEventWriter *orders.ChainEventWriter,
	config *PositionControllerConfig,
	userID string,
) *PositionController {
	if config == nil {
		config = DefaultPositionControllerConfig()
	}

	return &PositionController{
		futuresClient:    futuresClient,
		exitDecisionSvc:  exitDecisionSvc,
		chainEventWriter: chainEventWriter,
		config:           config,
		userID:           userID,
		stopChan:         make(chan struct{}),
	}
}

// ============================================================================
// LIFECYCLE METHODS
// ============================================================================

// Start starts the Position Controller.
func (pc *PositionController) Start(ctx context.Context) error {
	pc.mu.Lock()
	if pc.running {
		pc.mu.Unlock()
		return fmt.Errorf("%s already running", LogPrefixPC)
	}

	pc.running = true
	pc.startedAt = time.Now()
	pc.lastError = ""

	// Reset context for new run
	pc.ctx, pc.cancelFunc = context.WithCancel(ctx)
	pc.stopChan = make(chan struct{})

	pc.mu.Unlock()

	pc.log("Starting Position Controller for user %s", pc.userID)

	// Subscribe to exit decision signals
	if pc.exitDecisionSvc != nil {
		pc.exitDecisionSvc.SubscribeToSignals(pc.handleExitSignal)
		pc.log("Subscribed to Exit Decision signals")
	}

	// Start protection heal loop if enabled
	if pc.config.EnableProtectionHeal {
		pc.wg.Add(1)
		go func() {
			defer pc.wg.Done()
			pc.runProtectionHealLoop()
		}()
		pc.log("Protection heal loop started (interval: %v)", pc.config.HealCheckInterval)
	}

	pc.log("Position Controller started successfully")
	return nil
}

// Stop stops the Position Controller gracefully.
func (pc *PositionController) Stop() error {
	pc.mu.Lock()
	if !pc.running {
		pc.mu.Unlock()
		return fmt.Errorf("%s not running", LogPrefixPC)
	}

	pc.running = false
	stopChan := pc.stopChan
	pc.mu.Unlock()

	pc.log("Stopping Position Controller")

	// Signal all goroutines to stop
	pc.cancelFunc()

	// Safely close stopChan
	select {
	case <-stopChan:
		// Already closed
	default:
		close(stopChan)
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		pc.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		pc.log("Position Controller stopped gracefully")
	case <-time.After(10 * time.Second):
		pc.log("Position Controller stop timed out after 10 seconds")
	}

	return nil
}

// IsRunning returns whether the Position Controller is running.
func (pc *PositionController) IsRunning() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.running
}

// GetStatus returns the current status of the Position Controller.
func (pc *PositionController) GetStatus() *PositionControllerStatus {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	status := &PositionControllerStatus{
		Running:             pc.running,
		StartedAt:           pc.startedAt,
		SignalsProcessed:    atomic.LoadInt64(&pc.signalsProcessed),
		SLUpdatesExecuted:   atomic.LoadInt64(&pc.slUpdatesExecuted),
		TPUpdatesExecuted:   atomic.LoadInt64(&pc.tpUpdatesExecuted),
		HealActionsExecuted: atomic.LoadInt64(&pc.healActionsExecuted),
		LastSignalTime:      pc.lastSignalTime,
		LastError:           pc.lastError,
	}

	// Calculate uptime
	if pc.running && !pc.startedAt.IsZero() {
		uptime := time.Since(pc.startedAt)
		status.Uptime = formatUptimePC(uptime)
	}

	return status
}

// ============================================================================
// SIGNAL HANDLERS
// ============================================================================

// handleExitSignal processes an exit signal from Exit Decision Service.
// This is the dispatcher that routes signals to the appropriate handler.
func (pc *PositionController) handleExitSignal(signal exitdecision.ExitSignal) {
	pc.mu.Lock()
	pc.lastSignalTime = time.Now()
	pc.mu.Unlock()

	atomic.AddInt64(&pc.signalsProcessed, 1)

	pc.log("Received signal: %s %s (urgency=%s, type=%s)",
		signal.Symbol, signal.ExitType, signal.Urgency, signal.ExitType)

	// Map exitdecision.ExitType to our handlers
	switch signal.ExitType {
	case exitdecision.ExitTypeTrailing:
		pc.executeTrailingSL(signal)

	case exitdecision.ExitTypeTP:
		// TP hit - update or close
		pc.executeUpdateTP(signal)

	case exitdecision.ExitTypeSL:
		// SL hit - typically handled by Binance, but we can process for breakeven
		pc.executeMoveToBreakeven(signal)

	case exitdecision.ExitTypeEfficiency:
		pc.executeEfficiencyExit(signal)

	case exitdecision.ExitTypeTimeout:
		// Trend reversal or timeout - tighten SL immediately
		pc.executeTrendReversalExit(signal)

	default:
		pc.logDebug("Unknown signal type: %s", signal.ExitType)
	}
}

// executeTrailingSL updates the stop loss order on Binance.
func (pc *PositionController) executeTrailingSL(signal exitdecision.ExitSignal) {
	ctx, cancel := context.WithTimeout(pc.ctx, pc.config.OrderTimeout)
	defer cancel()

	pc.log("Executing trailing SL for %s: new price %.4f", signal.Symbol, signal.TriggerPrice)

	// Get current position info
	position, err := pc.getPosition(ctx, signal.Symbol)
	if err != nil {
		pc.setError("Failed to get position for %s: %v", signal.Symbol, err)
		return
	}

	// Determine new SL price from signal
	newSLPrice := signal.TriggerPrice
	if newSLPrice <= 0 {
		// Calculate from current price if not provided
		newSLPrice = pc.calculateTrailingSL(position, signal.CurrentPrice)
	}

	// Validate new SL is an improvement
	if !pc.isValidSLUpdate(position, newSLPrice) {
		pc.logDebug("SL update rejected for %s: not an improvement (current=%.4f, new=%.4f)",
			signal.Symbol, position.SLPrice, newSLPrice)
		return
	}

	// Cancel existing SL order
	if position.SLOrderID > 0 {
		if err := pc.cancelOrderWithRetry(ctx, signal.Symbol, position.SLOrderID, position.IsAlgoOrder); err != nil {
			pc.log("Warning: failed to cancel old SL for %s: %v (continuing anyway)", signal.Symbol, err)
		}
	}

	// Place new SL order
	newOrderID, err := pc.placeStopLossOrder(ctx, signal.Symbol, position.Side, position.Quantity, newSLPrice)
	if err != nil {
		pc.setError("Failed to place new SL for %s: %v", signal.Symbol, err)
		return
	}

	atomic.AddInt64(&pc.slUpdatesExecuted, 1)

	// Record event if chain event writer is available
	if pc.chainEventWriter != nil && position.ChainID != "" {
		pc.chainEventWriter.RecordSLModified(ctx, position.ChainID, orders.ChainSLModifiedEvent{
			BinanceOrderID:     newOrderID,
			OldPrice:           position.SLPrice,
			NewPrice:           newSLPrice,
			ModificationSource: orders.ModSourceSystem,
			ModificationReason: "Trailing SL update",
			MarketPrice:        signal.CurrentPrice,
			BinanceTimestamp:   time.Now().UnixMilli(),
		})
	}

	pc.log("SL updated for %s: %.4f -> %.4f (order=%d)", signal.Symbol, position.SLPrice, newSLPrice, newOrderID)
}

// executeMoveToBreakeven moves the stop loss to entry price plus fees.
func (pc *PositionController) executeMoveToBreakeven(signal exitdecision.ExitSignal) {
	ctx, cancel := context.WithTimeout(pc.ctx, pc.config.OrderTimeout)
	defer cancel()

	pc.log("Executing move to breakeven for %s", signal.Symbol)

	position, err := pc.getPosition(ctx, signal.Symbol)
	if err != nil {
		pc.setError("Failed to get position for breakeven %s: %v", signal.Symbol, err)
		return
	}

	// Calculate breakeven price (entry + estimated fees)
	// Assume 0.04% fee each way = 0.08% total
	feePercent := 0.0008
	var breakevenPrice float64
	if position.Side == "LONG" {
		breakevenPrice = position.EntryPrice * (1 + feePercent)
	} else {
		breakevenPrice = position.EntryPrice * (1 - feePercent)
	}

	// Round to appropriate precision
	breakevenPrice = roundToTickSize(breakevenPrice, getTickSizePC(signal.Symbol))

	// Check if current SL is already better than breakeven
	if !pc.isValidSLUpdate(position, breakevenPrice) {
		pc.logDebug("Breakeven rejected for %s: current SL already better", signal.Symbol)
		return
	}

	// Cancel existing SL
	if position.SLOrderID > 0 {
		if err := pc.cancelOrderWithRetry(ctx, signal.Symbol, position.SLOrderID, position.IsAlgoOrder); err != nil {
			pc.log("Warning: failed to cancel old SL for breakeven %s: %v", signal.Symbol, err)
		}
	}

	// Place new SL at breakeven
	newOrderID, err := pc.placeStopLossOrder(ctx, signal.Symbol, position.Side, position.Quantity, breakevenPrice)
	if err != nil {
		pc.setError("Failed to place breakeven SL for %s: %v", signal.Symbol, err)
		return
	}

	atomic.AddInt64(&pc.slUpdatesExecuted, 1)

	// Record event
	if pc.chainEventWriter != nil && position.ChainID != "" {
		pc.chainEventWriter.RecordSLModified(ctx, position.ChainID, orders.ChainSLModifiedEvent{
			BinanceOrderID:     newOrderID,
			OldPrice:           position.SLPrice,
			NewPrice:           breakevenPrice,
			ModificationSource: orders.ModSourceSystem,
			ModificationReason: "Move to breakeven",
			MarketPrice:        signal.CurrentPrice,
			BinanceTimestamp:   time.Now().UnixMilli(),
		})
	}

	pc.log("Moved SL to breakeven for %s: %.4f -> %.4f", signal.Symbol, position.SLPrice, breakevenPrice)
}

// executeUpdateTP updates the take profit order on Binance.
func (pc *PositionController) executeUpdateTP(signal exitdecision.ExitSignal) {
	ctx, cancel := context.WithTimeout(pc.ctx, pc.config.OrderTimeout)
	defer cancel()

	pc.log("Executing TP update for %s", signal.Symbol)

	position, err := pc.getPosition(ctx, signal.Symbol)
	if err != nil {
		pc.setError("Failed to get position for TP %s: %v", signal.Symbol, err)
		return
	}

	// New TP price from signal
	newTPPrice := signal.TriggerPrice
	if newTPPrice <= 0 {
		pc.logDebug("No TP price in signal for %s", signal.Symbol)
		return
	}

	// Cancel existing TP
	if position.TPOrderID > 0 {
		if err := pc.cancelOrderWithRetry(ctx, signal.Symbol, position.TPOrderID, position.IsAlgoOrder); err != nil {
			pc.log("Warning: failed to cancel old TP for %s: %v", signal.Symbol, err)
		}
	}

	// Place new TP
	newOrderID, err := pc.placeTakeProfitOrder(ctx, signal.Symbol, position.Side, position.Quantity, newTPPrice)
	if err != nil {
		pc.setError("Failed to place new TP for %s: %v", signal.Symbol, err)
		return
	}

	atomic.AddInt64(&pc.tpUpdatesExecuted, 1)

	// Record event
	if pc.chainEventWriter != nil && position.ChainID != "" {
		pc.chainEventWriter.RecordTPModified(ctx, position.ChainID, orders.ChainTPModifiedEvent{
			BinanceOrderID:     newOrderID,
			OldPrice:           position.TPPrice,
			NewPrice:           newTPPrice,
			ModificationSource: orders.ModSourceSystem,
			ModificationReason: "TP update from exit signal",
			MarketPrice:        signal.CurrentPrice,
			BinanceTimestamp:   time.Now().UnixMilli(),
		})
	}

	pc.log("TP updated for %s: %.4f -> %.4f", signal.Symbol, position.TPPrice, newTPPrice)
}

// executeTrendReversalExit tightens SL immediately on trend reversal signal.
func (pc *PositionController) executeTrendReversalExit(signal exitdecision.ExitSignal) {
	ctx, cancel := context.WithTimeout(pc.ctx, pc.config.OrderTimeout)
	defer cancel()

	pc.log("Executing trend reversal exit for %s (tighten SL)", signal.Symbol)

	position, err := pc.getPosition(ctx, signal.Symbol)
	if err != nil {
		pc.setError("Failed to get position for trend reversal %s: %v", signal.Symbol, err)
		return
	}

	// Tighten SL to a closer price (e.g., 0.5% from current price)
	tightenPercent := 0.005
	var newSLPrice float64
	if position.Side == "LONG" {
		newSLPrice = signal.CurrentPrice * (1 - tightenPercent)
	} else {
		newSLPrice = signal.CurrentPrice * (1 + tightenPercent)
	}

	// Round to tick size
	newSLPrice = roundToTickSize(newSLPrice, getTickSizePC(signal.Symbol))

	// Validate the update
	if !pc.isValidSLUpdate(position, newSLPrice) {
		pc.logDebug("Trend reversal SL rejected for %s: not an improvement", signal.Symbol)
		return
	}

	// Cancel and replace SL
	if position.SLOrderID > 0 {
		if err := pc.cancelOrderWithRetry(ctx, signal.Symbol, position.SLOrderID, position.IsAlgoOrder); err != nil {
			pc.log("Warning: failed to cancel old SL for trend reversal %s: %v", signal.Symbol, err)
		}
	}

	newOrderID, err := pc.placeStopLossOrder(ctx, signal.Symbol, position.Side, position.Quantity, newSLPrice)
	if err != nil {
		pc.setError("Failed to place trend reversal SL for %s: %v", signal.Symbol, err)
		return
	}

	atomic.AddInt64(&pc.slUpdatesExecuted, 1)

	// Record event
	if pc.chainEventWriter != nil && position.ChainID != "" {
		pc.chainEventWriter.RecordSLModified(ctx, position.ChainID, orders.ChainSLModifiedEvent{
			BinanceOrderID:     newOrderID,
			OldPrice:           position.SLPrice,
			NewPrice:           newSLPrice,
			ModificationSource: orders.ModSourceSystem,
			ModificationReason: "Trend reversal - tightened SL",
			MarketPrice:        signal.CurrentPrice,
			BinanceTimestamp:   time.Now().UnixMilli(),
		})
	}

	pc.log("Trend reversal SL tightened for %s: %.4f -> %.4f", signal.Symbol, position.SLPrice, newSLPrice)
}

// executeEfficiencyExit tightens SL based on efficiency decline.
func (pc *PositionController) executeEfficiencyExit(signal exitdecision.ExitSignal) {
	ctx, cancel := context.WithTimeout(pc.ctx, pc.config.OrderTimeout)
	defer cancel()

	pc.log("Executing efficiency exit for %s", signal.Symbol)

	position, err := pc.getPosition(ctx, signal.Symbol)
	if err != nil {
		pc.setError("Failed to get position for efficiency exit %s: %v", signal.Symbol, err)
		return
	}

	// Calculate new SL based on efficiency - tighten more aggressively
	// Efficiency exit typically means the trade is losing momentum
	tightenPercent := 0.003 // 0.3% from current price
	var newSLPrice float64
	if position.Side == "LONG" {
		newSLPrice = signal.CurrentPrice * (1 - tightenPercent)
	} else {
		newSLPrice = signal.CurrentPrice * (1 + tightenPercent)
	}

	newSLPrice = roundToTickSize(newSLPrice, getTickSizePC(signal.Symbol))

	// Validate
	if !pc.isValidSLUpdate(position, newSLPrice) {
		pc.logDebug("Efficiency SL rejected for %s: not an improvement", signal.Symbol)
		return
	}

	// Cancel and replace
	if position.SLOrderID > 0 {
		if err := pc.cancelOrderWithRetry(ctx, signal.Symbol, position.SLOrderID, position.IsAlgoOrder); err != nil {
			pc.log("Warning: failed to cancel old SL for efficiency exit %s: %v", signal.Symbol, err)
		}
	}

	newOrderID, err := pc.placeStopLossOrder(ctx, signal.Symbol, position.Side, position.Quantity, newSLPrice)
	if err != nil {
		pc.setError("Failed to place efficiency SL for %s: %v", signal.Symbol, err)
		return
	}

	atomic.AddInt64(&pc.slUpdatesExecuted, 1)

	// Record event
	if pc.chainEventWriter != nil && position.ChainID != "" {
		pc.chainEventWriter.RecordSLModified(ctx, position.ChainID, orders.ChainSLModifiedEvent{
			BinanceOrderID:     newOrderID,
			OldPrice:           position.SLPrice,
			NewPrice:           newSLPrice,
			ModificationSource: orders.ModSourceSystem,
			ModificationReason: fmt.Sprintf("Efficiency exit (%.1f%%)", signal.UnrealizedPnLPercent),
			MarketPrice:        signal.CurrentPrice,
			BinanceTimestamp:   time.Now().UnixMilli(),
		})
	}

	pc.log("Efficiency SL tightened for %s: %.4f -> %.4f", signal.Symbol, position.SLPrice, newSLPrice)
}

// ============================================================================
// PROTECTION HEAL
// ============================================================================

// runProtectionHealLoop ensures SL/TP orders exist for all chain positions.
func (pc *PositionController) runProtectionHealLoop() {
	pc.log("Protection heal loop started")

	ticker := time.NewTicker(pc.config.HealCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pc.ctx.Done():
			pc.log("Protection heal loop stopped (context cancelled)")
			return
		case <-pc.stopChan:
			pc.log("Protection heal loop stopped (stop signal)")
			return
		case <-ticker.C:
			pc.healAllPositions()
		}
	}
}

// healAllPositions checks and fixes missing SL/TP orders for all chain positions.
func (pc *PositionController) healAllPositions() {
	// Skip if no futures client configured
	if pc.futuresClient == nil {
		pc.logDebug("Heal: skipping - no futures client configured")
		return
	}

	ctx, cancel := context.WithTimeout(pc.ctx, 30*time.Second)
	defer cancel()

	positions, err := pc.getChainPositions(ctx)
	if err != nil {
		pc.setError("Heal: failed to get chain positions: %v", err)
		return
	}

	if len(positions) == 0 {
		pc.logDebug("Heal: no chain positions to check")
		return
	}

	pc.logDebug("Heal: checking %d chain positions", len(positions))

	for _, pos := range positions {
		// Check if SL order exists
		slExists := pos.SLOrderID > 0 && pc.orderExists(ctx, pos.Symbol, pos.SLOrderID, pos.IsAlgoOrder)
		if !slExists {
			pc.log("Heal: Missing SL for %s, placing protection SL", pos.Symbol)
			if err := pc.placeProtectionSL(ctx, pos); err != nil {
				pc.log("Heal: Failed to place protection SL for %s: %v", pos.Symbol, err)
			} else {
				atomic.AddInt64(&pc.healActionsExecuted, 1)
			}
		}

		// Check if TP order exists
		tpExists := pos.TPOrderID > 0 && pc.orderExists(ctx, pos.Symbol, pos.TPOrderID, pos.IsAlgoOrder)
		if !tpExists {
			pc.log("Heal: Missing TP for %s, placing protection TP", pos.Symbol)
			if err := pc.placeProtectionTP(ctx, pos); err != nil {
				pc.log("Heal: Failed to place protection TP for %s: %v", pos.Symbol, err)
			} else {
				atomic.AddInt64(&pc.healActionsExecuted, 1)
			}
		}
	}
}

// placeProtectionSL places a fallback SL order if missing.
func (pc *PositionController) placeProtectionSL(ctx context.Context, pos *PositionInfo) error {
	// Get SL percentage from chain settings (NOT Ginie mode settings)
	slPercent := pc.getChainSLPercent(pos.ChainID)
	if slPercent <= 0 {
		slPercent = 1.5 // Default fallback: 1.5%
	}

	// Calculate SL price
	var slPrice float64
	if pos.Side == "LONG" {
		slPrice = pos.EntryPrice * (1 - slPercent/100)
	} else {
		slPrice = pos.EntryPrice * (1 + slPercent/100)
	}

	slPrice = roundToTickSize(slPrice, getTickSizePC(pos.Symbol))

	// Place the SL order
	orderID, err := pc.placeStopLossOrder(ctx, pos.Symbol, pos.Side, pos.Quantity, slPrice)
	if err != nil {
		return fmt.Errorf("failed to place protection SL: %w", err)
	}

	pc.log("Heal: Placed protection SL for %s at %.4f (order=%d)", pos.Symbol, slPrice, orderID)

	// Record event if chain writer is available
	if pc.chainEventWriter != nil && pos.ChainID != "" {
		pc.chainEventWriter.RecordSLPlaced(ctx, pos.ChainID, orders.ChainSLPlacedEvent{
			BinanceOrderID: orderID,
			Price:          slPrice,
			BinanceTimestamp: time.Now().UnixMilli(),
		})
	}

	return nil
}

// placeProtectionTP places a fallback TP order if missing.
func (pc *PositionController) placeProtectionTP(ctx context.Context, pos *PositionInfo) error {
	// Get TP percentage from chain settings (NOT Ginie mode settings)
	tpPercent := pc.getChainTPPercent(pos.ChainID)
	if tpPercent <= 0 {
		tpPercent = 2.0 // Default fallback: 2%
	}

	// Calculate TP price
	var tpPrice float64
	if pos.Side == "LONG" {
		tpPrice = pos.EntryPrice * (1 + tpPercent/100)
	} else {
		tpPrice = pos.EntryPrice * (1 - tpPercent/100)
	}

	tpPrice = roundToTickSize(tpPrice, getTickSizePC(pos.Symbol))

	// Place the TP order
	orderID, err := pc.placeTakeProfitOrder(ctx, pos.Symbol, pos.Side, pos.Quantity, tpPrice)
	if err != nil {
		return fmt.Errorf("failed to place protection TP: %w", err)
	}

	pc.log("Heal: Placed protection TP for %s at %.4f (order=%d)", pos.Symbol, tpPrice, orderID)

	// Record event if chain writer is available
	if pc.chainEventWriter != nil && pos.ChainID != "" {
		pc.chainEventWriter.RecordTPPlaced(ctx, pos.ChainID, orders.ChainTPPlacedEvent{
			BinanceOrderID: orderID,
			Price:          tpPrice,
			BinanceTimestamp: time.Now().UnixMilli(),
		})
	}

	return nil
}

// HealNow triggers an immediate protection heal check.
func (pc *PositionController) HealNow() error {
	if !pc.IsRunning() {
		return fmt.Errorf("position controller not running")
	}

	pc.log("Manual heal triggered")
	pc.wg.Add(1)
	go func() {
		defer pc.wg.Done()
		pc.healAllPositions()
	}()
	return nil
}

// ============================================================================
// BINANCE ORDER OPERATIONS
// ============================================================================

// cancelOrderWithRetry cancels an order with retry logic.
func (pc *PositionController) cancelOrderWithRetry(ctx context.Context, symbol string, orderID int64, isAlgo bool) error {
	var lastErr error

	for attempt := 0; attempt < pc.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(pc.config.RetryDelay * time.Duration(1<<uint(attempt-1))) // Exponential backoff
		}

		var err error
		if isAlgo {
			err = pc.futuresClient.CancelAlgoOrder(symbol, orderID)
		} else {
			err = pc.futuresClient.CancelFuturesOrder(symbol, orderID)
		}

		if err == nil {
			return nil
		}

		lastErr = err
		pc.logDebug("Cancel order attempt %d failed for %s order %d: %v", attempt+1, symbol, orderID, err)
	}

	return fmt.Errorf("failed after %d attempts: %w", pc.config.MaxRetries, lastErr)
}

// placeStopLossOrder places a stop loss order using algo order API.
func (pc *PositionController) placeStopLossOrder(ctx context.Context, symbol, side string, quantity, price float64) (int64, error) {
	// Determine order side (opposite of position side for SL)
	orderSide := "SELL"
	positionSide := binance.PositionSideLong
	if side == "SHORT" {
		orderSide = "BUY"
		positionSide = binance.PositionSideShort
	}

	// Use algo order API (Binance change as of 2025-12-09)
	params := binance.AlgoOrderParams{
		Symbol:       symbol,
		Side:         orderSide,
		PositionSide: positionSide,
		Type:         binance.FuturesOrderTypeStopMarket,
		TriggerPrice: price,
		Quantity:     quantity,
		ReduceOnly:   true,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	var lastErr error
	for attempt := 0; attempt < pc.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(pc.config.RetryDelay * time.Duration(1<<uint(attempt-1)))
		}

		resp, err := pc.futuresClient.PlaceAlgoOrder(params)
		if err == nil && resp != nil {
			return resp.AlgoId, nil
		}

		lastErr = err
		pc.logDebug("Place SL attempt %d failed for %s: %v", attempt+1, symbol, err)
	}

	return 0, fmt.Errorf("failed to place SL after %d attempts: %w", pc.config.MaxRetries, lastErr)
}

// placeTakeProfitOrder places a take profit order using algo order API.
func (pc *PositionController) placeTakeProfitOrder(ctx context.Context, symbol, side string, quantity, price float64) (int64, error) {
	// Determine order side (opposite of position side for TP)
	orderSide := "SELL"
	positionSide := binance.PositionSideLong
	if side == "SHORT" {
		orderSide = "BUY"
		positionSide = binance.PositionSideShort
	}

	// Use algo order API
	params := binance.AlgoOrderParams{
		Symbol:       symbol,
		Side:         orderSide,
		PositionSide: positionSide,
		Type:         binance.FuturesOrderTypeTakeProfitMarket,
		TriggerPrice: price,
		Quantity:     quantity,
		ReduceOnly:   true,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	var lastErr error
	for attempt := 0; attempt < pc.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(pc.config.RetryDelay * time.Duration(1<<uint(attempt-1)))
		}

		resp, err := pc.futuresClient.PlaceAlgoOrder(params)
		if err == nil && resp != nil {
			return resp.AlgoId, nil
		}

		lastErr = err
		pc.logDebug("Place TP attempt %d failed for %s: %v", attempt+1, symbol, err)
	}

	return 0, fmt.Errorf("failed to place TP after %d attempts: %w", pc.config.MaxRetries, lastErr)
}

// orderExists checks if an order still exists on Binance.
func (pc *PositionController) orderExists(ctx context.Context, symbol string, orderID int64, isAlgo bool) bool {
	if isAlgo {
		// Check algo orders
		algoOrders, err := pc.futuresClient.GetOpenAlgoOrders(symbol)
		if err != nil {
			return false
		}
		for _, order := range algoOrders {
			if order.AlgoId == orderID {
				return true
			}
		}
		return false
	}

	// Check regular orders
	_, err := pc.futuresClient.GetOrder(symbol, orderID)
	return err == nil
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// getPosition retrieves position information for a symbol.
func (pc *PositionController) getPosition(ctx context.Context, symbol string) (*PositionInfo, error) {
	// Get position from Binance
	binancePos, err := pc.futuresClient.GetPositionBySymbol(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get Binance position: %w", err)
	}

	if binancePos == nil || binancePos.PositionAmt == 0 {
		return nil, fmt.Errorf("no open position for %s", symbol)
	}

	// Determine side
	side := "LONG"
	if binancePos.PositionAmt < 0 {
		side = "SHORT"
	}

	// Get open orders for SL/TP detection
	var slOrderID, tpOrderID int64
	var isAlgoOrder bool

	// Check algo orders first (new Binance API)
	algoOrders, err := pc.futuresClient.GetOpenAlgoOrders(symbol)
	if err == nil {
		for _, order := range algoOrders {
			if order.OrderType == string(binance.FuturesOrderTypeStopMarket) {
				slOrderID = order.AlgoId
				isAlgoOrder = true
			} else if order.OrderType == string(binance.FuturesOrderTypeTakeProfitMarket) {
				tpOrderID = order.AlgoId
				isAlgoOrder = true
			}
		}
	}

	// Get chain ID if available (from chain event writer)
	chainID := ""
	if pc.chainEventWriter != nil {
		chains, err := pc.chainEventWriter.GetActiveChains(ctx, pc.userID)
		if err == nil {
			for _, chain := range chains {
				if chain.Symbol == symbol {
					chainID = chain.ChainID
					break
				}
			}
		}
	}

	return &PositionInfo{
		Symbol:      symbol,
		Side:        side,
		EntryPrice:  binancePos.EntryPrice,
		Quantity:    math.Abs(binancePos.PositionAmt),
		SLPrice:     0, // Would need to track or get from chain
		TPPrice:     0,
		SLOrderID:   slOrderID,
		TPOrderID:   tpOrderID,
		ChainID:     chainID,
		IsAlgoOrder: isAlgoOrder,
	}, nil
}

// getChainPositions retrieves all chain positions for the user.
func (pc *PositionController) getChainPositions(ctx context.Context) ([]*PositionInfo, error) {
	// Get Binance positions first
	binancePositions, err := pc.futuresClient.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get Binance positions: %w", err)
	}

	// Filter to only active positions and get chain IDs
	var positions []*PositionInfo
	for _, bp := range binancePositions {
		if bp.PositionAmt == 0 {
			continue
		}

		posInfo, err := pc.getPosition(ctx, bp.Symbol)
		if err != nil {
			pc.logDebug("Skipping position %s: %v", bp.Symbol, err)
			continue
		}

		// Only include positions with chain IDs (chain trading system positions)
		if posInfo.ChainID != "" {
			positions = append(positions, posInfo)
		}
	}

	return positions, nil
}

// isValidSLUpdate checks if the new SL price is an improvement.
func (pc *PositionController) isValidSLUpdate(pos *PositionInfo, newSL float64) bool {
	if pos.SLPrice <= 0 {
		return true // No current SL, any price is valid
	}

	if pos.Side == "LONG" {
		// For LONG, new SL must be higher (trailing up)
		return newSL > pos.SLPrice
	}
	// For SHORT, new SL must be lower (trailing down)
	return newSL < pos.SLPrice
}

// calculateTrailingSL calculates a trailing SL price based on current price.
func (pc *PositionController) calculateTrailingSL(pos *PositionInfo, currentPrice float64) float64 {
	// Default trailing distance: 1%
	trailPercent := 0.01

	var newSL float64
	if pos.Side == "LONG" {
		newSL = currentPrice * (1 - trailPercent)
	} else {
		newSL = currentPrice * (1 + trailPercent)
	}

	return roundToTickSize(newSL, getTickSizePC(pos.Symbol))
}

// getChainSLPercent retrieves SL percentage from chain settings (NOT Ginie mode settings).
func (pc *PositionController) getChainSLPercent(chainID string) float64 {
	if chainID == "" || pc.chainEventWriter == nil {
		return 1.5 // Default fallback
	}

	chain, err := pc.chainEventWriter.GetChain(pc.ctx, pc.userID, chainID)
	if err != nil || chain == nil {
		return 1.5
	}

	// The chain should have SL% stored from entry time
	// For now, return default - actual implementation would read from chain settings
	// TODO: Add SL/TP percent fields to OrderChain when chain settings are fully implemented
	return 1.5
}

// getChainTPPercent retrieves TP percentage from chain settings (NOT Ginie mode settings).
func (pc *PositionController) getChainTPPercent(chainID string) float64 {
	if chainID == "" || pc.chainEventWriter == nil {
		return 2.0 // Default fallback
	}

	chain, err := pc.chainEventWriter.GetChain(pc.ctx, pc.userID, chainID)
	if err != nil || chain == nil {
		return 2.0
	}

	// TODO: Add SL/TP percent fields to OrderChain when chain settings are fully implemented
	return 2.0
}

// ============================================================================
// LOGGING
// ============================================================================

// log logs a message with the standard prefix.
func (pc *PositionController) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s %s", LogPrefixPC, msg)
}

// logDebug logs a debug message (only if debug mode is enabled).
func (pc *PositionController) logDebug(format string, args ...interface{}) {
	if pc.config != nil && pc.config.DebugMode {
		msg := fmt.Sprintf(format, args...)
		log.Printf("%s [DEBUG] %s", LogPrefixPC, msg)
	}
}

// setError sets the last error and logs it.
func (pc *PositionController) setError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	pc.mu.Lock()
	pc.lastError = msg
	pc.mu.Unlock()
	log.Printf("%s [ERROR] %s", LogPrefixPC, msg)
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// formatUptimePC formats a duration as a human-readable uptime string.
func formatUptimePC(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// roundToTickSize rounds a price to the appropriate tick size.
func roundToTickSize(price float64, tickSize float64) float64 {
	if tickSize <= 0 {
		tickSize = 0.01 // Default tick size
	}
	return math.Round(price/tickSize) * tickSize
}

// getTickSizePC returns the tick size for a symbol.
// In production, this should come from exchange info cache.
func getTickSizePC(symbol string) float64 {
	// Default tick sizes for common pairs
	// This is a simplification - real implementation would use exchange info
	// Check length >= 5 to safely access last 4 characters (e.g., "AUSDT" is minimum)
	if len(symbol) >= 5 && symbol[len(symbol)-4:] == "USDT" {
		baseSymbol := symbol[:len(symbol)-4]
		switch baseSymbol {
		case "BTC":
			return 0.1
		case "ETH":
			return 0.01
		default:
			return 0.0001
		}
	}
	return 0.0001
}
