// Package autopilot provides adapter types for integrating ExitDecisionService
// with CoinProfiler and GinieAutopilot.
package autopilot

import (
	"binance-trading-bot/internal/coinprofiler"
	"binance-trading-bot/internal/exitdecision"
	"context"
	"fmt"
	"time"
)

// ============================================================================
// COIN PROFILER -> PRICE PROVIDER ADAPTER
// ============================================================================

// CoinProfilerPriceAdapter adapts CoinProfiler to implement exitdecision.PriceProvider.
// This allows ExitDecisionService to get prices from CoinProfiler's WebSocket data.
type CoinProfilerPriceAdapter struct {
	profiler *coinprofiler.CoinProfiler
}

// NewCoinProfilerPriceAdapter creates a new adapter.
func NewCoinProfilerPriceAdapter(profiler *coinprofiler.CoinProfiler) *CoinProfilerPriceAdapter {
	return &CoinProfilerPriceAdapter{profiler: profiler}
}

// GetPrice returns the current price for a symbol from CoinProfiler.
func (a *CoinProfilerPriceAdapter) GetPrice(ctx context.Context, symbol string) (float64, error) {
	if a.profiler == nil {
		return 0, fmt.Errorf("coin profiler not available")
	}

	data := a.profiler.GetCoinData(symbol)
	if data == nil {
		return 0, fmt.Errorf("no data for symbol %s", symbol)
	}

	if data.Price <= 0 {
		return 0, fmt.Errorf("invalid price for symbol %s", symbol)
	}

	return data.Price, nil
}

// GetPrices returns current prices for multiple symbols.
func (a *CoinProfilerPriceAdapter) GetPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	if a.profiler == nil {
		return nil, fmt.Errorf("coin profiler not available")
	}

	prices := make(map[string]float64)
	for _, symbol := range symbols {
		data := a.profiler.GetCoinData(symbol)
		if data != nil && data.Price > 0 {
			prices[symbol] = data.Price
		}
	}

	return prices, nil
}

// ============================================================================
// GINIE POSITION -> EXIT DECISION POSITION ADAPTER
// ============================================================================

// GiniePositionAdapter wraps GiniePosition to implement exitdecision.Position interface.
type GiniePositionAdapter struct {
	pos          *GiniePosition
	currentPrice float64 // Injected from price provider
	peakProfit   float64 // Tracked for efficiency calculation
}

// NewGiniePositionAdapter creates a new adapter.
func NewGiniePositionAdapter(pos *GiniePosition) *GiniePositionAdapter {
	adapter := &GiniePositionAdapter{pos: pos}
	// Calculate current price from unrealized PnL if available
	// This is a fallback; price provider should set the actual current price
	if pos.HighestPrice > 0 {
		adapter.currentPrice = pos.HighestPrice
	}
	return adapter
}

// SetCurrentPrice sets the current market price (called by position provider).
func (a *GiniePositionAdapter) SetCurrentPrice(price float64) {
	a.currentPrice = price
}

// GetSymbol returns the position symbol.
func (a *GiniePositionAdapter) GetSymbol() string {
	return a.pos.Symbol
}

// GetMode returns the trading mode.
func (a *GiniePositionAdapter) GetMode() string {
	return string(a.pos.Mode)
}

// GetSide returns "long" or "short".
func (a *GiniePositionAdapter) GetSide() string {
	return a.pos.Side
}

// GetEntryPrice returns the entry price.
func (a *GiniePositionAdapter) GetEntryPrice() float64 {
	return a.pos.EntryPrice
}

// GetCurrentPrice returns the current price.
func (a *GiniePositionAdapter) GetCurrentPrice() float64 {
	return a.currentPrice
}

// GetOriginalQty returns the original quantity.
func (a *GiniePositionAdapter) GetOriginalQty() float64 {
	return a.pos.OriginalQty
}

// GetRemainingQty returns the remaining quantity.
func (a *GiniePositionAdapter) GetRemainingQty() float64 {
	return a.pos.RemainingQty
}

// GetEntryTime returns the entry timestamp.
func (a *GiniePositionAdapter) GetEntryTime() time.Time {
	return a.pos.EntryTime
}

// GetTakeProfitLevels returns the TP level configuration.
func (a *GiniePositionAdapter) GetTakeProfitLevels() []exitdecision.TakeProfitLevel {
	levels := make([]exitdecision.TakeProfitLevel, len(a.pos.TakeProfits))
	for i, tp := range a.pos.TakeProfits {
		levels[i] = exitdecision.TakeProfitLevel{
			Level:   tp.Level,
			Price:   tp.Price,
			Percent: tp.Percent,
			GainPct: tp.GainPct,
			Status:  tp.Status,
		}
	}
	return levels
}

// GetCurrentTPLevel returns the current TP level (0 = none hit yet).
func (a *GiniePositionAdapter) GetCurrentTPLevel() int {
	return a.pos.CurrentTPLevel
}

// GetStopLossPrice returns the stop loss price.
func (a *GiniePositionAdapter) GetStopLossPrice() float64 {
	return a.pos.StopLoss
}

// HasMovedToBreakeven returns whether SL has been moved to breakeven.
func (a *GiniePositionAdapter) HasMovedToBreakeven() bool {
	return a.pos.MovedToBreakeven
}

// IsTrailingActive returns whether trailing stop is active.
func (a *GiniePositionAdapter) IsTrailingActive() bool {
	return a.pos.TrailingActive
}

// GetTrailingPercent returns the trailing stop percentage.
func (a *GiniePositionAdapter) GetTrailingPercent() float64 {
	return a.pos.TrailingPercent
}

// GetTrailingActivationPct returns when trailing activates.
func (a *GiniePositionAdapter) GetTrailingActivationPct() float64 {
	return a.pos.TrailingActivationPct
}

// GetHighestPrice returns the highest price since entry.
func (a *GiniePositionAdapter) GetHighestPrice() float64 {
	return a.pos.HighestPrice
}

// GetLowestPrice returns the lowest price since entry.
func (a *GiniePositionAdapter) GetLowestPrice() float64 {
	return a.pos.LowestPrice
}

// IsEfficiencyActive returns whether efficiency tracking is active.
// Efficiency tracking is considered active when the position has had positive unrealized PnL.
func (a *GiniePositionAdapter) IsEfficiencyActive() bool {
	return a.peakProfit > 0
}

// GetPeakProfit returns the peak unrealized profit.
func (a *GiniePositionAdapter) GetPeakProfit() float64 {
	return a.peakProfit
}

// GetCurrentProfit returns the current unrealized profit.
func (a *GiniePositionAdapter) GetCurrentProfit() float64 {
	return a.pos.UnrealizedPnL
}

// GetEfficiency returns current efficiency (current/peak profit ratio).
func (a *GiniePositionAdapter) GetEfficiency() float64 {
	if a.peakProfit <= 0 {
		return 1.0 // No peak yet, 100% efficiency
	}
	if a.pos.UnrealizedPnL <= 0 {
		return 0.0 // Currently in loss
	}
	return a.pos.UnrealizedPnL / a.peakProfit
}

// UpdatePeakProfit updates the peak profit if current is higher.
func (a *GiniePositionAdapter) UpdatePeakProfit() {
	if a.pos.UnrealizedPnL > a.peakProfit {
		a.peakProfit = a.pos.UnrealizedPnL
	}
}

// ============================================================================
// GINIE AUTOPILOT -> POSITION PROVIDER ADAPTER
// ============================================================================

// GiniePositionProviderAdapter adapts GinieAutopilot to implement exitdecision.PositionProvider.
type GiniePositionProviderAdapter struct {
	autopilot *GinieAutopilot
	userID    string
}

// NewGiniePositionProviderAdapter creates a new adapter.
func NewGiniePositionProviderAdapter(autopilot *GinieAutopilot, userID string) *GiniePositionProviderAdapter {
	return &GiniePositionProviderAdapter{
		autopilot: autopilot,
		userID:    userID,
	}
}

// GetOpenPositions returns all open positions for a user.
func (a *GiniePositionProviderAdapter) GetOpenPositions(ctx context.Context, userID string) ([]exitdecision.Position, error) {
	if a.autopilot == nil {
		return []exitdecision.Position{}, nil
	}

	// Get positions from GinieAutopilot
	positions := a.autopilot.GetPositions()
	if len(positions) == 0 {
		return []exitdecision.Position{}, nil
	}

	// Convert to exitdecision.Position interface
	result := make([]exitdecision.Position, 0, len(positions))
	for _, pos := range positions {
		if pos != nil {
			result = append(result, NewGiniePositionAdapter(pos))
		}
	}

	return result, nil
}

// GetPosition returns a specific position by symbol.
func (a *GiniePositionProviderAdapter) GetPosition(ctx context.Context, userID, symbol string) (exitdecision.Position, error) {
	if a.autopilot == nil {
		return nil, fmt.Errorf("autopilot not available")
	}

	// Get all positions and find the one with matching symbol
	positions := a.autopilot.GetPositions()
	for _, pos := range positions {
		if pos != nil && pos.Symbol == symbol {
			return NewGiniePositionAdapter(pos), nil
		}
	}

	return nil, fmt.Errorf("position not found for symbol %s", symbol)
}

// ============================================================================
// WIRE PROVIDERS TO EXIT DECISION SERVICE
// ============================================================================

// WireExitDecisionProviders wires the PriceProvider and PositionProvider to ExitDecisionService.
// This should be called after all components are created but before starting the service.
func WireExitDecisionProviders(
	exitSvc *exitdecision.Service,
	profiler *coinprofiler.CoinProfiler,
	autopilot *GinieAutopilot,
	userID string,
) {
	if exitSvc == nil {
		return
	}

	// Wire price provider (from CoinProfiler)
	if profiler != nil {
		priceAdapter := NewCoinProfilerPriceAdapter(profiler)
		exitSvc.SetPriceProvider(priceAdapter)
	}

	// Wire position provider (from GinieAutopilot)
	if autopilot != nil {
		positionAdapter := NewGiniePositionProviderAdapter(autopilot, userID)
		exitSvc.SetPositionProvider(positionAdapter)
	}
}
