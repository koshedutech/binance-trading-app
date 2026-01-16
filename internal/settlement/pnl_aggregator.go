// Package settlement provides the P&L aggregation service for Epic 8 Story 8.2.
// This service aggregates daily trading P&L by mode (scalp, swing, position, ultra_fast).
package settlement

import (
	"context"
	"fmt"
	"log"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/orders"
)

// PnLAggregator handles daily P&L aggregation by trading mode
type PnLAggregator struct {
	clientFactory *binance.ClientFactory
}

// NewPnLAggregator creates a new P&L aggregator service
func NewPnLAggregator(clientFactory *binance.ClientFactory) *PnLAggregator {
	return &PnLAggregator{
		clientFactory: clientFactory,
	}
}

// AggregatePnLByMode aggregates P&L for a user's trading day by mode
// startTime and endTime define the trading day (typically midnight to midnight in user's timezone)
func (a *PnLAggregator) AggregatePnLByMode(ctx context.Context, userID string, startTime, endTime time.Time) (*DailyPnLAggregation, error) {
	start := time.Now()
	result := &DailyPnLAggregation{
		UserID:      userID,
		Date:        startTime,
		ModeResults: make(map[string]*ModePnL),
		Success:     false,
	}

	// Validate inputs
	if userID == "" {
		result.Error = "userID cannot be empty"
		result.Duration = time.Since(start)
		return result, fmt.Errorf("userID cannot be empty")
	}

	// Get Binance client for user
	client, err := a.clientFactory.GetFuturesClientForUser(ctx, userID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get Binance client: %v", err)
		result.Duration = time.Since(start)
		log.Printf("[PNL-AGGREGATOR] Error getting Binance client for user %s: %v", userID, err)
		return result, err
	}

	// Convert times to milliseconds for Binance API
	startMs := startTime.UnixMilli()
	endMs := endTime.UnixMilli()

	// Step 1: Fetch all trades for the date range
	// Note: Binance requires symbol parameter, so we need to fetch trades for each symbol
	// For now, we'll use empty string which works on some endpoints
	trades, err := client.GetTradeHistoryByDateRange("", startMs, endMs, 1000)
	if err != nil {
		// Binance may require symbol - let's try fetching trades for each active symbol
		trades, err = a.fetchTradesAllSymbols(client, startMs, endMs)
		if err != nil {
			result.Error = fmt.Sprintf("failed to fetch trades: %v", err)
			result.Duration = time.Since(start)
			log.Printf("[PNL-AGGREGATOR] Error fetching trades for user %s: %v", userID, err)
			return result, err
		}
	}

	log.Printf("[PNL-AGGREGATOR] Fetched %d trades for user %s", len(trades), userID)

	// Handle zero trades case
	if len(trades) == 0 {
		// Create empty "ALL" mode
		result.ModeResults[ModeAll] = &ModePnL{Mode: ModeAll}
		result.Success = true
		result.Duration = time.Since(start)
		log.Printf("[PNL-AGGREGATOR] No trades found for user %s in date range", userID)
		return result, nil
	}

	// Step 2: Build orderId -> clientOrderId map from orders
	orderClientIDs, err := a.buildOrderClientIDMap(client, trades, startMs, endMs)
	if err != nil {
		log.Printf("[PNL-AGGREGATOR] Warning: failed to build order-clientID map: %v", err)
		// Continue with empty map - trades will be marked as UNKNOWN mode
		orderClientIDs = make(map[int64]string)
	}

	// Step 3: Aggregate trades by mode
	modeMap := make(map[string]*ModePnL)

	for _, trade := range trades {
		// Get clientOrderId for this trade's order
		clientOrderID := orderClientIDs[trade.OrderId]

		// Extract mode from clientOrderId
		mode := extractModeFromClientOrderID(clientOrderID)

		// Get or create mode entry
		modePnL, exists := modeMap[mode]
		if !exists {
			modePnL = &ModePnL{Mode: mode}
			modeMap[mode] = modePnL
		}

		// Aggregate trade data
		modePnL.TradeCount++
		modePnL.RealizedPnL += trade.RealizedPnl
		modePnL.TotalVolume += trade.QuoteQty // USDT value

		// Track wins and losses
		if trade.RealizedPnl > 0 {
			modePnL.WinCount++
			if trade.RealizedPnl > modePnL.LargestWin {
				modePnL.LargestWin = trade.RealizedPnl
			}
		} else if trade.RealizedPnl < 0 {
			modePnL.LossCount++
			if trade.RealizedPnl < modePnL.LargestLoss {
				modePnL.LargestLoss = trade.RealizedPnl
			}
		}
	}

	// Step 4: Calculate derived metrics for each mode
	for _, modePnL := range modeMap {
		calculateDerivedMetrics(modePnL)
	}

	// Step 5: Create "ALL" mode summary
	allMode := createAllModeSummary(modeMap)
	modeMap[ModeAll] = allMode

	result.ModeResults = modeMap
	result.TotalPnL = allMode.RealizedPnL
	result.TotalTrades = allMode.TradeCount
	result.Success = true
	result.Duration = time.Since(start)

	log.Printf("[PNL-AGGREGATOR] Aggregation completed for user %s: %d trades, %d modes, total P&L: %.2f, duration: %v",
		userID, result.TotalTrades, len(modeMap)-1, result.TotalPnL, result.Duration)

	return result, nil
}

// fetchTradesAllSymbols fetches trades for all symbols that had activity in the date range.
// It discovers symbols using income history (REALIZED_PNL records) and current positions,
// then fetches trades for each symbol and deduplicates by TradeId.
func (a *PnLAggregator) fetchTradesAllSymbols(client binance.FuturesClient, startMs, endMs int64) ([]binance.FuturesTrade, error) {
	symbols := make(map[string]bool)

	// Step 1: Discover symbols from income history (captures closed positions)
	// This is the most reliable way to find all symbols that had realized P&L
	incomeRecords, err := client.GetIncomeHistory("REALIZED_PNL", startMs, endMs, 1000)
	if err != nil {
		log.Printf("[PNL-AGGREGATOR] Warning: failed to get income history: %v", err)
		// Continue - we'll fall back to positions
	} else {
		for _, record := range incomeRecords {
			if record.Symbol != "" {
				symbols[record.Symbol] = true
			}
		}
		log.Printf("[PNL-AGGREGATOR] Discovered %d symbols from income history", len(symbols))
	}

	// Step 2: Also include symbols with current open positions
	positions, err := client.GetPositions()
	if err != nil {
		log.Printf("[PNL-AGGREGATOR] Warning: failed to get positions: %v", err)
		// Continue with symbols from income history
	} else {
		for _, pos := range positions {
			if pos.PositionAmt != 0 {
				symbols[pos.Symbol] = true
			}
		}
	}

	// If no symbols found, return empty
	if len(symbols) == 0 {
		log.Printf("[PNL-AGGREGATOR] No symbols found for date range")
		return []binance.FuturesTrade{}, nil
	}

	// Step 3: Fetch trades for each symbol
	var allTrades []binance.FuturesTrade
	for symbol := range symbols {
		trades, err := client.GetTradeHistoryByDateRange(symbol, startMs, endMs, 1000)
		if err != nil {
			log.Printf("[PNL-AGGREGATOR] Warning: failed to get trades for %s: %v", symbol, err)
			continue
		}
		allTrades = append(allTrades, trades...)
	}

	// Step 4: Deduplicate trades by TradeId (in case of any overlap)
	allTrades = deduplicateTradesByID(allTrades)

	log.Printf("[PNL-AGGREGATOR] Fetched %d unique trades across %d symbols", len(allTrades), len(symbols))
	return allTrades, nil
}

// deduplicateTradesByID removes duplicate trades by their ID field
func deduplicateTradesByID(trades []binance.FuturesTrade) []binance.FuturesTrade {
	seen := make(map[int64]bool)
	unique := make([]binance.FuturesTrade, 0, len(trades))

	for _, trade := range trades {
		if !seen[trade.ID] {
			seen[trade.ID] = true
			unique = append(unique, trade)
		}
	}

	return unique
}

// buildOrderClientIDMap builds a map of orderId -> clientOrderId from orders
func (a *PnLAggregator) buildOrderClientIDMap(client binance.FuturesClient, trades []binance.FuturesTrade, startMs, endMs int64) (map[int64]string, error) {
	orderClientIDs := make(map[int64]string)

	// Get unique symbols from trades
	symbols := make(map[string]bool)
	for _, trade := range trades {
		symbols[trade.Symbol] = true
	}

	// Fetch orders for each symbol
	for symbol := range symbols {
		orders, err := client.GetAllOrdersByDateRange(symbol, startMs, endMs, 500)
		if err != nil {
			log.Printf("[PNL-AGGREGATOR] Warning: failed to get orders for %s: %v", symbol, err)
			continue
		}

		for _, order := range orders {
			if order.ClientOrderId != "" {
				orderClientIDs[order.OrderId] = order.ClientOrderId
			}
		}
	}

	log.Printf("[PNL-AGGREGATOR] Built orderClientID map with %d entries", len(orderClientIDs))
	return orderClientIDs, nil
}

// calculateDerivedMetrics calculates win rate and average trade size for a mode
func calculateDerivedMetrics(modePnL *ModePnL) {
	// Calculate win rate (avoid division by zero)
	if modePnL.TradeCount > 0 {
		modePnL.WinRate = float64(modePnL.WinCount) / float64(modePnL.TradeCount) * 100
		modePnL.AvgTradeSize = modePnL.TotalVolume / float64(modePnL.TradeCount)
	}
}

// createAllModeSummary creates the "ALL" mode summary from all individual modes
func createAllModeSummary(modeMap map[string]*ModePnL) *ModePnL {
	allMode := &ModePnL{Mode: ModeAll}

	for _, modePnL := range modeMap {
		allMode.TradeCount += modePnL.TradeCount
		allMode.RealizedPnL += modePnL.RealizedPnL
		allMode.WinCount += modePnL.WinCount
		allMode.LossCount += modePnL.LossCount
		allMode.TotalVolume += modePnL.TotalVolume

		// Track largest win/loss across all modes
		if modePnL.LargestWin > allMode.LargestWin {
			allMode.LargestWin = modePnL.LargestWin
		}
		if modePnL.LargestLoss < allMode.LargestLoss {
			allMode.LargestLoss = modePnL.LargestLoss
		}
	}

	// Calculate derived metrics for ALL mode
	calculateDerivedMetrics(allMode)

	return allMode
}

// AggregatePnLForTrades aggregates P&L for a given set of trades and orders
// This is a testable version that doesn't require Binance API access
func AggregatePnLForTrades(trades []binance.FuturesTrade, orderClientIDs map[int64]string) map[string]*ModePnL {
	modeMap := make(map[string]*ModePnL)

	for _, trade := range trades {
		// Get clientOrderId for this trade's order
		clientOrderID := orderClientIDs[trade.OrderId]

		// Extract mode from clientOrderId
		mode := extractModeFromClientOrderID(clientOrderID)

		// Get or create mode entry
		modePnL, exists := modeMap[mode]
		if !exists {
			modePnL = &ModePnL{Mode: mode}
			modeMap[mode] = modePnL
		}

		// Aggregate trade data
		modePnL.TradeCount++
		modePnL.RealizedPnL += trade.RealizedPnl
		modePnL.TotalVolume += trade.QuoteQty

		// Track wins and losses
		if trade.RealizedPnl > 0 {
			modePnL.WinCount++
			if trade.RealizedPnl > modePnL.LargestWin {
				modePnL.LargestWin = trade.RealizedPnl
			}
		} else if trade.RealizedPnl < 0 {
			modePnL.LossCount++
			if trade.RealizedPnl < modePnL.LargestLoss {
				modePnL.LargestLoss = trade.RealizedPnl
			}
		}
	}

	// Calculate derived metrics for each mode
	for _, modePnL := range modeMap {
		calculateDerivedMetrics(modePnL)
	}

	// Create "ALL" mode summary
	allMode := createAllModeSummary(modeMap)
	modeMap[ModeAll] = allMode

	return modeMap
}

// ExtractModeFromParsedOrderID extracts the mode string from a ParsedOrderId
// This is exported for testing
func ExtractModeFromParsedOrderID(parsed *orders.ParsedOrderId) string {
	if parsed == nil {
		return ModeUnknown
	}

	switch parsed.Mode {
	case orders.ModeScalp:
		return ModeScalp
	case orders.ModeSwing:
		return ModeSwing
	case orders.ModePosition:
		return ModePosition
	case orders.ModeUltraFast:
		return ModeUltraFast
	default:
		return ModeUnknown
	}
}
