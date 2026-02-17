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

	"binance-trading-bot/internal/binance"
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
	RemovePositionSymbol(symbol string)
	RebuildCapacity(activeChainCount, maxConcurrent int) (int, bool)
}

// FuturesOrderCanceler defines the interface for canceling and verifying orders on Binance.
// Implemented by binance.CachedFuturesClient.
type FuturesOrderCanceler interface {
	CancelAllAlgoOrders(symbol string) error
	CancelAllFuturesOrders(symbol string) error
	GetOpenAlgoOrders(symbol string) ([]binance.AlgoOrder, error)
}

// FuturesTradesFetcher defines the interface for fetching trade history from Binance.
// Used to retrieve actual commission/fees after position close.
// Implemented by binance.CachedFuturesClient.
type FuturesTradesFetcher interface {
	GetTradeHistory(symbol string, limit int) ([]binance.FuturesTrade, error)
}

// RavindraMonitorRemover defines the interface for removing positions from Ravindra monitor.
// Implemented by RavindraPositionMonitor.
type RavindraMonitorRemover interface {
	RemovePositionBySymbol(symbol string)
}

// PositionLifecycleCoordinator handles position close events in a deterministic order.
type PositionLifecycleCoordinator struct {
	db               *database.DB
	chainEventWriter *orders.ChainEventWriter
	patternResetter  PatternResetter
	capacityRebuilder CapacityRebuilder
	futuresCanceler  FuturesOrderCanceler
	tradesFetcher    FuturesTradesFetcher
	ravindraRemover  RavindraMonitorRemover
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

// SetTradesFetcher sets the Binance trades fetcher for commission lookup after chain close.
func (plc *PositionLifecycleCoordinator) SetTradesFetcher(tf FuturesTradesFetcher) {
	plc.tradesFetcher = tf
}

// SetRavindraRemover sets the Ravindra monitor remover for explicit position cleanup on chain close.
func (plc *PositionLifecycleCoordinator) SetRavindraRemover(rr RavindraMonitorRemover) {
	plc.ravindraRemover = rr
}

// SetMaxConcurrent sets the maximum concurrent trades for capacity calculations.
func (plc *PositionLifecycleCoordinator) SetMaxConcurrent(max int) {
	plc.maxConcurrent = max
}

// GetMaxConcurrent returns the maximum concurrent trades for capacity calculations.
func (plc *PositionLifecycleCoordinator) GetMaxConcurrent() int {
	return plc.maxConcurrent
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

	// Step 3.5: Remove position from Ravindra monitor BEFORE cancelling orders.
	// This prevents a race where the monitor's 5s check loop tries to modify the SL
	// (cancel old + place new) concurrently with our CancelAllAlgoOrders call below.
	// Without this, the sequence can be:
	//   1. Coordinator calls CancelAllAlgoOrders → cancels SL
	//   2. Ravindra monitor's checkPosition fires, sees stale SL order ID
	//   3. Monitor calls cancelOrderWithRetry → "order not found" (OK)
	//   4. Monitor places NEW SL for a position that no longer exists
	if plc.ravindraRemover != nil {
		plc.ravindraRemover.RemovePositionBySymbol(chain.Symbol)
		plc.logger.Info().Str("symbol", chain.Symbol).Msg("Removed position from Ravindra monitor before order cancellation")
	}

	// Step 4: Cancel remaining algo orders on Binance
	if plc.futuresCanceler != nil {
		// 4a: Cancel all algo orders (SL/TP placed via /fapi/v1/algoOrder)
		if err := plc.futuresCanceler.CancelAllAlgoOrders(chain.Symbol); err != nil {
			plc.logger.Warn().Err(err).
				Str("symbol", chain.Symbol).
				Msg("Failed to cancel remaining algo orders (may already be cancelled)")
		} else {
			plc.logger.Info().Str("symbol", chain.Symbol).Msg("Cancelled remaining algo orders")
		}

		// 4b: Also cancel any regular (non-algo) orders for this symbol.
		// In some code paths, SL/TP could be placed as regular STOP_MARKET/TAKE_PROFIT_MARKET
		// orders (e.g., legacy GinieAutopilot paths). Belt-and-suspenders approach.
		if err := plc.futuresCanceler.CancelAllFuturesOrders(chain.Symbol); err != nil {
			plc.logger.Debug().Err(err).
				Str("symbol", chain.Symbol).
				Msg("Failed to cancel regular orders (may not exist)")
		}

		// 4c: Verify cancellation - brief delay then check if any algo orders remain.
		// Addresses observed issue where CancelAllAlgoOrders succeeds but orders persist
		// on Binance (e.g., SOL/USDT SL remained after TP fill).
		time.Sleep(500 * time.Millisecond)
		openAlgos, verifyErr := plc.futuresCanceler.GetOpenAlgoOrders(chain.Symbol)
		if verifyErr == nil && len(openAlgos) > 0 {
			plc.logger.Warn().
				Int("remaining_count", len(openAlgos)).
				Str("symbol", chain.Symbol).
				Msg("Algo orders still open after cancel - retrying cancellation")
			// Retry cancellation
			if retryErr := plc.futuresCanceler.CancelAllAlgoOrders(chain.Symbol); retryErr != nil {
				plc.logger.Error().Err(retryErr).
					Str("symbol", chain.Symbol).
					Msg("Retry cancel also failed - orders may be orphaned (proactive protection will clean up)")
			} else {
				plc.logger.Info().Str("symbol", chain.Symbol).Msg("Retry cancel succeeded")
			}
		} else if verifyErr == nil {
			plc.logger.Info().Str("symbol", chain.Symbol).Msg("Verified: no algo orders remaining after cancel")
		} else {
			plc.logger.Debug().Err(verifyErr).
				Str("symbol", chain.Symbol).
				Msg("Could not verify algo order cancellation (API error)")
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
	if err := plc.chainEventWriter.CloseChain(ctx, chain.ChainID, closeReason, realizedPnL, event.Commission, &closePrice); err != nil {
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

	// NOTE: Equity update is handled by the ChainEventWriter.onChainClosed callback
	// (set in main.go) which fires inside CloseChain() above. Do NOT call
	// updateStrategyEquity here to avoid double-counting.

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
		plc.capacityRebuilder.RemovePositionSymbol(chain.Symbol)

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

	// Get updated equity for broadcast (if equity was updated)
	var newEquity *float64
	if chain.Mode != "" && chain.StrategyGroup != "" && chain.SubStrategy != "" {
		repo := database.NewRepository(plc.db)
		if eq, err := repo.GetSubStrategyEquity(ctx, event.UserID, chain.Mode, chain.StrategyGroup, chain.SubStrategy); err == nil {
			newEquity = &eq
		}
	}

	compositeEvent := map[string]interface{}{
		"chain": map[string]interface{}{
			"chain_id":       chain.ChainID,
			"symbol":         chain.Symbol,
			"side":           chain.Side,
			"close_reason":   closeReason,
			"close_price":    event.FilledPrice,
			"realized_pnl":   realizedPnL,
			"pnl_percent":    pnlPercent,
			"total_fees":     event.Commission,
			"mode":           chain.Mode,
			"mode_code":      chain.ModeCode,
			"timeframe":      chain.Timeframe,
			"closed_at":      fillTime.Format("2006-01-02T15:04:05Z07:00"),
			"new_equity":     newEquity,
			"strategy_group": chain.StrategyGroup,
			"sub_strategy":   chain.SubStrategy,
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

	// Also broadcast PNL_UPDATE so PnL summary/calendar components refresh
	events.BroadcastPnL(event.UserID, map[string]interface{}{
		"chain_id":     chain.ChainID,
		"symbol":       chain.Symbol,
		"realized_pnl": realizedPnL,
		"source":       "chain_close",
	})

	// Broadcast EQUITY_UPDATE so strategy settings refresh
	events.BroadcastEquityUpdate(event.UserID, map[string]interface{}{
		"source": "chain_close",
	})

	// Step 10: Fetch real fees from Binance in background.
	// ALGO_UPDATE events have Commission=0, and ORDER_TRADE_UPDATE PnL correction
	// may not always arrive. This ensures fees are always captured from actual trades.
	if event.Commission == 0 && plc.tradesFetcher != nil {
		go plc.fetchAndUpdateFees(chain.ChainID, chain.Symbol, event.UserID, fillTime)
	}

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

// fetchAndUpdateFees retrieves actual trade commission from Binance userTrades API
// and updates the chain's total_fees in DB. ALGO_UPDATE events always have Commission=0,
// so this is the reliable way to get real fees after position close.
func (plc *PositionLifecycleCoordinator) fetchAndUpdateFees(chainID, symbol, userID string, fillTime time.Time) {
	// Wait for Binance to finalize the trade records
	time.Sleep(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch recent trades for this symbol (last 20 should be plenty)
	trades, err := plc.tradesFetcher.GetTradeHistory(symbol, 20)
	if err != nil {
		plc.logger.Warn().Err(err).
			Str("chain_id", chainID).
			Str("symbol", symbol).
			Msg("fetchAndUpdateFees: failed to fetch trade history from Binance")
		return
	}

	if len(trades) == 0 {
		plc.logger.Debug().
			Str("chain_id", chainID).
			Str("symbol", symbol).
			Msg("fetchAndUpdateFees: no trades found")
		return
	}

	// Find trades within a 5-second window around the fill time.
	// SL/TP fills are typically single trades, but partials can create multiple.
	fillTimeMs := fillTime.UnixMilli()
	windowMs := int64(5000)
	startMs := fillTimeMs - windowMs
	endMs := fillTimeMs + windowMs

	var totalFees float64
	var totalPnL float64
	var matchedCount int
	for _, trade := range trades {
		if trade.Time >= startMs && trade.Time <= endMs {
			totalFees += math.Abs(trade.Commission)
			totalPnL += trade.RealizedPnl
			matchedCount++
		}
	}

	if matchedCount == 0 {
		plc.logger.Debug().
			Str("chain_id", chainID).
			Str("symbol", symbol).
			Int64("fill_time_ms", fillTimeMs).
			Int("total_trades", len(trades)).
			Msg("fetchAndUpdateFees: no trades matched fill time window")
		return
	}

	plc.logger.Info().
		Str("chain_id", chainID).
		Str("symbol", symbol).
		Float64("total_fees", totalFees).
		Float64("total_pnl", totalPnL).
		Int("matched_trades", matchedCount).
		Msg("fetchAndUpdateFees: found real fees from Binance trades")

	// Use CorrectChainPnL which also fires the equity callback for PnL delta
	if plc.chainEventWriter != nil && totalFees > 0 {
		// Get current chain to check if PnL also needs correction
		chain, err := plc.db.GetOrderChainByChainIDOnly(ctx, chainID)
		if err != nil || chain == nil {
			plc.logger.Warn().Err(err).Str("chain_id", chainID).Msg("fetchAndUpdateFees: failed to get chain for PnL check")
			// Fallback: just update fees directly
			if updateErr := plc.db.UpdateClosedChainPnL(ctx, chainID, 0, totalFees); updateErr != nil {
				plc.logger.Error().Err(updateErr).Str("chain_id", chainID).Msg("fetchAndUpdateFees: failed to update fees in DB")
			}
			return
		}

		// Determine the best PnL value: use real trade PnL if available, else keep existing
		bestPnL := totalPnL
		if bestPnL == 0 && chain.RealizedPnL != nil {
			bestPnL = *chain.RealizedPnL
		}

		// Use CorrectChainPnL to update both PnL and fees with equity callback
		if err := plc.chainEventWriter.CorrectChainPnL(ctx, chainID, bestPnL, totalFees); err != nil {
			plc.logger.Error().Err(err).Str("chain_id", chainID).Msg("fetchAndUpdateFees: failed to correct chain PnL/fees")
			return
		}

		// Broadcast PNL_CORRECTED to frontend
		events.BroadcastPnLCorrected(userID, map[string]interface{}{
			"chain_id":   chainID,
			"symbol":     symbol,
			"total_fees": totalFees,
			"realized_pnl": bestPnL,
		})

		plc.logger.Info().
			Str("chain_id", chainID).
			Float64("fees", totalFees).
			Float64("pnl", bestPnL).
			Msg("fetchAndUpdateFees: corrected chain with real fees from Binance")
	}
}

