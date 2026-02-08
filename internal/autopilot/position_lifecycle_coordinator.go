// Package autopilot provides the Position Lifecycle Coordinator.
// Story 14.19: Centralized handler for position close events (SL/TP fills).
//
// The PositionLifecycleCoordinator replaces the scattered close logic in
// GinieAutopilot.HandleSLTPOrderFilled with a deterministic, single-responsibility
// handler that:
//  1. Identifies the chain by Binance order ID
//  2. Updates filled/canceled order status in DB
//  3. Cancels remaining algo orders on Binance
//  4. Calculates PnL and closes the chain
//  5. Resets entry decision pattern for the symbol
//  6. Updates CoinProfiler capacity
//  7. Broadcasts a composite CHAIN_LIFECYCLE_UPDATE event
package autopilot

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/orders"

	"github.com/rs/zerolog"
)

// PatternResetter defines the interface for resetting entry decision patterns.
// Implemented by entrydecision.RealtimePatternMatcher.
type PatternResetter interface {
	ResetPatternForSymbol(symbol, mode, timeframe string)
}

// CapacityRebuilder defines the interface for updating CoinProfiler capacity.
// Implemented by coinprofiler.CoinProfiler.
type CapacityRebuilder interface {
	UpdateSymbolToStrategy(symbol string)
	RebuildCapacity(activeChainCount, maxConcurrent int) (int, bool)
}

// FuturesOrderCanceler defines the interface for canceling algo orders on Binance.
// Implemented by binance.CachedFuturesClient.
type FuturesOrderCanceler interface {
	CancelAllAlgoOrders(symbol string) error
}

// PositionLifecycleCoordinator handles position close events in a deterministic order.
type PositionLifecycleCoordinator struct {
	db               *database.DB
	chainEventWriter *orders.ChainEventWriter
	patternResetter  PatternResetter
	capacityRebuilder CapacityRebuilder
	futuresCanceler  FuturesOrderCanceler
	maxConcurrent    int
	logger           zerolog.Logger
}

// NewPositionLifecycleCoordinator creates a new coordinator.
func NewPositionLifecycleCoordinator(
	db *database.DB,
	chainEventWriter *orders.ChainEventWriter,
	logger zerolog.Logger,
) *PositionLifecycleCoordinator {
	return &PositionLifecycleCoordinator{
		db:               db,
		chainEventWriter: chainEventWriter,
		maxConcurrent:    5, // default, overridden by SetMaxConcurrent
		logger:           logger.With().Str("component", "PositionLifecycleCoordinator").Logger(),
	}
}

// SetPatternResetter sets the pattern resetter (RealtimePatternMatcher).
func (plc *PositionLifecycleCoordinator) SetPatternResetter(pr PatternResetter) {
	plc.patternResetter = pr
}

// SetCapacityRebuilder sets the capacity rebuilder (CoinProfiler).
func (plc *PositionLifecycleCoordinator) SetCapacityRebuilder(cr CapacityRebuilder) {
	plc.capacityRebuilder = cr
}

// SetFuturesCanceler sets the futures order canceler.
func (plc *PositionLifecycleCoordinator) SetFuturesCanceler(fc FuturesOrderCanceler) {
	plc.futuresCanceler = fc
}

// SetMaxConcurrent sets the maximum concurrent trades for capacity calculations.
func (plc *PositionLifecycleCoordinator) SetMaxConcurrent(max int) {
	plc.maxConcurrent = max
}

// OrderFillEvent contains data from a Binance WebSocket order fill event.
type OrderFillEvent struct {
	UserID          string
	Symbol          string
	OrderType       string  // "STOP_MARKET", "STOP", "TAKE_PROFIT_MARKET", "TAKE_PROFIT"
	OrderID         int64
	FilledPrice     float64
	RealizedProfit  float64
	Commission      float64
	BinanceTimestamp int64
	PositionSide    string // "LONG", "SHORT", "BOTH"
}

// HandleOrderFillResult contains the result of handling an order fill.
type HandleOrderFillResult struct {
	Handled       bool
	ChainID       string
	Symbol        string
	CloseReason   string
	RealizedPnL   float64
	PnLPercent    float64
	CapacityUsed  int
	ScanEnabled   bool
}

// HandleOrderFill processes a SL/TP order fill event through the full lifecycle.
// Returns (result, error). If the order doesn't match any active chain, result.Handled is false.
func (plc *PositionLifecycleCoordinator) HandleOrderFill(ctx context.Context, event OrderFillEvent) (*HandleOrderFillResult, error) {
	isSLFill := event.OrderType == "STOP_MARKET" || event.OrderType == "STOP"
	isTPFill := event.OrderType == "TAKE_PROFIT_MARKET" || event.OrderType == "TAKE_PROFIT"

	if !isSLFill && !isTPFill {
		return &HandleOrderFillResult{Handled: false}, nil
	}

	plc.logger.Info().
		Str("symbol", event.Symbol).
		Str("order_type", event.OrderType).
		Int64("order_id", event.OrderID).
		Float64("filled_price", event.FilledPrice).
		Float64("realized_profit", event.RealizedProfit).
		Str("position_side", event.PositionSide).
		Msg("HandleOrderFill: processing SL/TP fill")

	// Step 1: Find chain by Binance order ID
	chain, orderIndicator, err := plc.db.FindChainByBinanceOrderID(ctx, event.UserID, event.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to find chain by order ID: %w", err)
	}
	if chain == nil {
		plc.logger.Debug().
			Int64("order_id", event.OrderID).
			Str("symbol", event.Symbol).
			Msg("HandleOrderFill: no matching chain found")
		return &HandleOrderFillResult{Handled: false}, nil
	}

	plc.logger.Info().
		Str("chain_id", chain.ChainID).
		Str("symbol", chain.Symbol).
		Str("order_indicator", orderIndicator).
		Msg("HandleOrderFill: matched chain")

	// Guard against concurrent processing - verify chain is still active
	if chain.Status != "ACTIVE" && chain.Status != "PARTIAL" {
		plc.logger.Info().
			Str("chain_id", chain.ChainID).
			Str("status", string(chain.Status)).
			Msg("HandleOrderFill: chain already processed, skipping (concurrent processing guard)")
		return &HandleOrderFillResult{Handled: true}, nil
	}

	// Validate consistency: SL fill should match SL order, TP fill should match TP order
	if (isSLFill && orderIndicator != "sl") || (isTPFill && orderIndicator != "tp") {
		plc.logger.Warn().
			Str("chain_id", chain.ChainID).
			Bool("is_sl_fill", isSLFill).
			Str("order_indicator", orderIndicator).
			Msg("HandleOrderFill: order type mismatch, proceeding anyway")
	}

	fillTime := time.UnixMilli(event.BinanceTimestamp)

	// Step 2: Update filled order status in DB
	if isSLFill {
		if err := plc.db.UpdateOrderChainSLFilled(ctx, chain.ChainID, event.FilledPrice, fillTime); err != nil {
			plc.logger.Warn().Err(err).Str("chain_id", chain.ChainID).Msg("Failed to update SL filled")
		}
		// Record SL filled event
		if plc.chainEventWriter != nil {
			_ = plc.chainEventWriter.RecordSLFilled(ctx, chain.ChainID, orders.ChainSLFilledEvent{
				FilledPrice:      event.FilledPrice,
				PnL:              event.RealizedProfit,
				Fees:             event.Commission,
				BinanceTimestamp: event.BinanceTimestamp,
			})
		}
	} else {
		if err := plc.db.UpdateOrderChainTPFilled(ctx, chain.ChainID, event.FilledPrice, fillTime); err != nil {
			plc.logger.Warn().Err(err).Str("chain_id", chain.ChainID).Msg("Failed to update TP filled")
		}
		// Record TP filled event
		if plc.chainEventWriter != nil {
			_ = plc.chainEventWriter.RecordTPFilled(ctx, chain.ChainID, orders.ChainTPFilledEvent{
				FilledPrice:      event.FilledPrice,
				PnL:              event.RealizedProfit,
				Fees:             event.Commission,
				BinanceTimestamp: event.BinanceTimestamp,
			})
		}
	}

	// Step 3: Update counterpart order to CANCELED
	if isSLFill {
		if err := plc.db.UpdateOrderChainTPCanceled(ctx, chain.ChainID, fillTime); err != nil {
			plc.logger.Warn().Err(err).Str("chain_id", chain.ChainID).Msg("Failed to mark TP canceled")
		}
	} else {
		if err := plc.db.UpdateOrderChainSLCanceled(ctx, chain.ChainID, fillTime); err != nil {
			plc.logger.Warn().Err(err).Str("chain_id", chain.ChainID).Msg("Failed to mark SL canceled")
		}
	}

	// Step 4: Cancel remaining algo orders on Binance
	if plc.futuresCanceler != nil {
		if err := plc.futuresCanceler.CancelAllAlgoOrders(chain.Symbol); err != nil {
			plc.logger.Warn().Err(err).
				Str("symbol", chain.Symbol).
				Msg("Failed to cancel remaining algo orders (may already be cancelled)")
		} else {
			plc.logger.Info().Str("symbol", chain.Symbol).Msg("Cancelled remaining algo orders")
		}
	}

	// Step 5: Calculate PnL
	var closeReason string
	if isSLFill {
		closeReason = string(orders.CloseReasonSLHit)
	} else {
		closeReason = string(orders.CloseReasonTPHit)
	}

	// Calculate realized PnL - ALGO_UPDATE events have RealizedProfit=0,
	// so we compute from entry vs close price when needed
	realizedPnL := event.RealizedProfit
	if realizedPnL == 0 && chain.EntryPrice != nil && *chain.EntryPrice > 0 &&
		chain.EntryQuantity != nil && *chain.EntryQuantity > 0 && event.FilledPrice > 0 {
		if strings.ToUpper(chain.Side) == "LONG" {
			realizedPnL = (event.FilledPrice - *chain.EntryPrice) * *chain.EntryQuantity
		} else {
			realizedPnL = (*chain.EntryPrice - event.FilledPrice) * *chain.EntryQuantity
		}
		realizedPnL = math.Round(realizedPnL*10000) / 10000
		plc.logger.Info().
			Float64("calculated_pnl", realizedPnL).
			Float64("entry_price", *chain.EntryPrice).
			Float64("close_price", event.FilledPrice).
			Float64("quantity", *chain.EntryQuantity).
			Str("side", chain.Side).
			Msg("PnL calculated from entry/close prices (ALGO_UPDATE lacks RealizedProfit)")
	}

	pnlPercent := 0.0
	if chain.EntryPrice != nil && *chain.EntryPrice > 0 && chain.EntryQuantity != nil && *chain.EntryQuantity > 0 {
		notionalValue := *chain.EntryPrice * *chain.EntryQuantity
		if notionalValue > 0 {
			pnlPercent = (realizedPnL / notionalValue) * 100
			pnlPercent = math.Round(pnlPercent*100) / 100
		}
	}

	// Step 6: Close chain in DB
	closePrice := event.FilledPrice
	if err := plc.db.CloseOrderChain(ctx, chain.ChainID, closeReason, realizedPnL, event.Commission, &closePrice); err != nil {
		plc.logger.Error().Err(err).Str("chain_id", chain.ChainID).Msg("Failed to close order chain")
		// Return Handled=true because we already modified state in steps 2-4 (SL/TP status,
		// algo order cancellation). Returning Handled=false would cause GinieAutopilot to
		// double-process the same fill.
		return &HandleOrderFillResult{
			Handled:     true,
			ChainID:     chain.ChainID,
			Symbol:      chain.Symbol,
			CloseReason: closeReason,
			RealizedPnL: realizedPnL,
		}, fmt.Errorf("failed to close order chain: %w", err)
	}

	plc.logger.Info().
		Str("chain_id", chain.ChainID).
		Str("close_reason", closeReason).
		Float64("pnl", realizedPnL).
		Float64("pnl_percent", pnlPercent).
		Msg("Chain closed")

	// Step 7: Reset pattern via patternMatcher
	if plc.patternResetter != nil && chain.Mode != "" && chain.Timeframe != "" {
		plc.patternResetter.ResetPatternForSymbol(chain.Symbol, chain.Mode, chain.Timeframe)
		plc.logger.Debug().
			Str("symbol", chain.Symbol).
			Str("mode", chain.Mode).
			Str("timeframe", chain.Timeframe).
			Msg("Pattern reset for symbol")
	}

	// Step 8: Update CoinProfiler capacity
	capacityUsed := 0
	scanEnabled := true
	if plc.capacityRebuilder != nil {
		plc.capacityRebuilder.UpdateSymbolToStrategy(chain.Symbol)

		// Count remaining active chains
		activeCount, err := plc.db.CountActiveChains(ctx, event.UserID)
		if err != nil {
			plc.logger.Warn().Err(err).Msg("Failed to count active chains for capacity rebuild")
		} else {
			capacityUsed, scanEnabled = plc.capacityRebuilder.RebuildCapacity(activeCount, plc.maxConcurrent)
		}
	}

	// Step 9: Build and broadcast composite event
	// IMPORTANT: Field names must match what the frontend expects:
	//   - "chain" (not "chain_close") - TradeLifecycleTab accesses data.chain.chain_id
	//   - "pattern" (not "pattern_reset") - CoinStageCard accesses data.pattern.symbol
	//   - "orders" - TradeLifecycleTab accesses data.orders.sl_status / tp_status
	//   - "capacity" - CoinProfilerCard accesses data.capacity
	var slStatus, tpStatus string
	var slFillPrice, tpFillPrice *float64
	if isSLFill {
		slStatus = "FILLED"
		tpStatus = "CANCELED"
		slFillPrice = &event.FilledPrice
	} else {
		slStatus = "CANCELED"
		tpStatus = "FILLED"
		tpFillPrice = &event.FilledPrice
	}

	compositeEvent := map[string]interface{}{
		"chain": map[string]interface{}{
			"chain_id":     chain.ChainID,
			"symbol":       chain.Symbol,
			"side":         chain.Side,
			"close_reason": closeReason,
			"close_price":  event.FilledPrice,
			"realized_pnl": realizedPnL,
			"pnl_percent":  pnlPercent,
			"total_fees":   event.Commission,
			"mode":         chain.Mode,
			"mode_code":    chain.ModeCode,
			"timeframe":    chain.Timeframe,
			"closed_at":    fillTime.Format("2006-01-02T15:04:05Z07:00"),
		},
		"pattern": map[string]interface{}{
			"symbol":    chain.Symbol,
			"mode":      chain.Mode,
			"timeframe": chain.Timeframe,
			"status":    "watching",
		},
		"orders": map[string]interface{}{
			"sl_status":     slStatus,
			"tp_status":     tpStatus,
			"sl_fill_price": slFillPrice,
			"tp_fill_price": tpFillPrice,
		},
		"capacity": map[string]interface{}{
			"capacity_used":    capacityUsed,
			"max_concurrent":   plc.maxConcurrent,
			"scanning_enabled": scanEnabled,
		},
	}

	events.BroadcastChainLifecycleUpdate(event.UserID, compositeEvent)

	// Also broadcast the legacy CHAIN_CLOSED event for backward compatibility
	events.BroadcastChainClosed(event.UserID, map[string]interface{}{
		"chain_id":     chain.ChainID,
		"symbol":       chain.Symbol,
		"close_reason": closeReason,
		"realized_pnl": realizedPnL,
		"mode":         chain.Mode,
		"mode_code":    chain.ModeCode,
		"timeframe":    chain.Timeframe,
	})

	return &HandleOrderFillResult{
		Handled:      true,
		ChainID:      chain.ChainID,
		Symbol:       chain.Symbol,
		CloseReason:  closeReason,
		RealizedPnL:  realizedPnL,
		PnLPercent:   pnlPercent,
		CapacityUsed: capacityUsed,
		ScanEnabled:  scanEnabled,
	}, nil
}
