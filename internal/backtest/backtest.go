package backtest

import (
	"context"
	"fmt"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/strategy"
)

// Backtest executes a backtest for a visual strategy
type Backtest struct {
	client   *binance.Client
	repo     *database.Repository
	strategy strategy.Strategy
}

// Config holds backtest configuration
type Config struct {
	StrategyConfigID int64
	Symbol           string
	Interval         string
	StartDate        time.Time
	EndDate          time.Time
	InitialBalance   float64
}

// Position represents an open trading position
type Position struct {
	EntryTime   time.Time
	EntryPrice  float64
	EntryReason string
	Side        string
	Quantity    float64
}

// NewBacktest creates a new backtest instance
func NewBacktest(client *binance.Client, repo *database.Repository, strat strategy.Strategy) *Backtest {
	return &Backtest{
		client:   client,
		repo:     repo,
		strategy: strat,
	}
}

// Run executes the backtest
func (b *Backtest) Run(ctx context.Context, config Config) (*database.BacktestResult, []database.BacktestTrade, error) {
	// Fetch historical klines
	klines, err := b.fetchHistoricalKlines(ctx, config.Symbol, config.Interval, config.StartDate, config.EndDate)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch klines: %w", err)
	}

	if len(klines) < 100 {
		return nil, nil, fmt.Errorf("insufficient historical data: got %d klines, need at least 100", len(klines))
	}

	// Initialize result
	result := &database.BacktestResult{
		StrategyConfigID: config.StrategyConfigID,
		Symbol:           config.Symbol,
		Interval:         config.Interval,
		StartDate:        config.StartDate,
		EndDate:          config.EndDate,
	}

	trades := []database.BacktestTrade{}
	balance := config.InitialBalance
	var currentPosition *Position

	// Simulate trading on each candle (start at index 100 to have enough history)
	for i := 100; i < len(klines); i++ {
		windowKlines := klines[i-100 : i+1]
		currentCandle := klines[i]

		// Check exit conditions first if we have a position
		if currentPosition != nil {
			exitTriggered, exitReason := b.checkExitConditions(currentPosition, currentCandle)

			if exitTriggered {
				trade := b.closeTrade(currentPosition, currentCandle, exitReason, balance)
				trades = append(trades, trade)
				balance += trade.PnL - trade.Fees
				currentPosition = nil
			}
		}

		// If no position, check for entry signal
		if currentPosition == nil {
			currentPrice := currentCandle.Close
			signal, err := b.strategy.Evaluate(windowKlines, currentPrice)
			if err != nil {
				continue
			}

			if signal != nil {
				// Open position
				quantity := (balance * 0.95) / signal.EntryPrice // Use 95% of balance
				currentPosition = &Position{
					EntryTime:   time.Unix(0, currentCandle.CloseTime*int64(time.Millisecond)),
					EntryPrice:  signal.EntryPrice,
					EntryReason: signal.Reason,
					Side:        string(signal.Type),
					Quantity:    quantity,
				}
			}
		}
	}

	// Close any remaining open position
	if currentPosition != nil {
		lastCandle := klines[len(klines)-1]
		trade := b.closeTrade(currentPosition, lastCandle, "End of backtest", balance)
		trades = append(trades, trade)
	}

	// Calculate metrics
	b.calculateMetrics(result, trades, config.InitialBalance)

	return result, trades, nil
}

// fetchHistoricalKlines fetches klines for the backtest period
func (b *Backtest) fetchHistoricalKlines(ctx context.Context, symbol, interval string, startDate, endDate time.Time) ([]binance.Kline, error) {
	// For MVP, fetch all klines in one call (max 1000)
	// In production, you'd paginate through larger date ranges
	allKlines := []binance.Kline{}

	// Calculate how many klines we need based on interval
	limit := 1000 // Max allowed by Binance

	klines, err := b.client.GetKlines(symbol, interval, limit)
	if err != nil {
		return nil, err
	}

	// Filter klines by date range
	for _, kline := range klines {
		klTime := time.Unix(0, kline.CloseTime*int64(time.Millisecond))
		if klTime.After(startDate) && klTime.Before(endDate) {
			allKlines = append(allKlines, kline)
		}
	}

	return allKlines, nil
}

// checkExitConditions checks if position should be closed
func (b *Backtest) checkExitConditions(position *Position, currentCandle binance.Kline) (bool, string) {
	// For MVP, we'll use simple price-based exit
	// In full version, this would evaluate exit nodes from visual flow

	// Example: Exit if price moved 3% in either direction
	priceChange := (currentCandle.Close - position.EntryPrice) / position.EntryPrice * 100

	if priceChange >= 3.0 {
		return true, "Take Profit (3%)"
	}

	if priceChange <= -2.0 {
		return true, "Stop Loss (2%)"
	}

	return false, ""
}

// closeTrade closes a position and creates a trade record
func (b *Backtest) closeTrade(position *Position, exitCandle binance.Kline, exitReason string, balance float64) database.BacktestTrade {
	exitPrice := exitCandle.Close
	exitTime := time.Unix(0, exitCandle.CloseTime*int64(time.Millisecond))

	pnl := (exitPrice - position.EntryPrice) * position.Quantity
	pnlPercent := (exitPrice - position.EntryPrice) / position.EntryPrice * 100

	// Calculate fees (0.1% per trade, so 0.2% total)
	fees := (position.EntryPrice*position.Quantity + exitPrice*position.Quantity) * 0.001

	duration := exitTime.Sub(position.EntryTime)

	return database.BacktestTrade{
		EntryTime:       position.EntryTime,
		EntryPrice:      position.EntryPrice,
		EntryReason:     position.EntryReason,
		ExitTime:        exitTime,
		ExitPrice:       exitPrice,
		ExitReason:      exitReason,
		Quantity:        position.Quantity,
		Side:            position.Side,
		PnL:             pnl,
		PnLPercent:      pnlPercent,
		Fees:            fees,
		DurationMinutes: int(duration.Minutes()),
	}
}

// calculateMetrics calculates backtest performance metrics
func (b *Backtest) calculateMetrics(result *database.BacktestResult, trades []database.BacktestTrade, initialBalance float64) {
	if len(trades) == 0 {
		return
	}

	result.TotalTrades = len(trades)

	var totalWinPnL, totalLossPnL float64
	var totalDuration int

	for _, trade := range trades {
		result.TotalPnL += trade.PnL
		result.TotalFees += trade.Fees

		totalDuration += trade.DurationMinutes

		if trade.PnL > 0 {
			result.WinningTrades++
			totalWinPnL += trade.PnL
			if trade.PnL > result.LargestWin {
				result.LargestWin = trade.PnL
			}
		} else {
			result.LosingTrades++
			totalLossPnL += trade.PnL
			if trade.PnL < result.LargestLoss {
				result.LargestLoss = trade.PnL
			}
		}
	}

	result.NetPnL = result.TotalPnL - result.TotalFees

	if result.TotalTrades > 0 {
		result.WinRate = float64(result.WinningTrades) / float64(result.TotalTrades) * 100
		result.AvgTradeDurationMinutes = totalDuration / result.TotalTrades
	}

	if result.WinningTrades > 0 {
		result.AverageWin = totalWinPnL / float64(result.WinningTrades)
	}

	if result.LosingTrades > 0 {
		result.AverageLoss = totalLossPnL / float64(result.LosingTrades)
	}

	if totalLossPnL != 0 {
		result.ProfitFactor = totalWinPnL / (-totalLossPnL)
	}

	// Calculate max drawdown
	result.MaxDrawdown, result.MaxDrawdownPercent = b.calculateMaxDrawdown(trades, initialBalance)
}

// calculateMaxDrawdown calculates the maximum drawdown
func (b *Backtest) calculateMaxDrawdown(trades []database.BacktestTrade, initialBalance float64) (float64, float64) {
	if len(trades) == 0 {
		return 0, 0
	}

	balance := initialBalance
	peak := initialBalance
	maxDrawdown := 0.0

	for _, trade := range trades {
		balance += trade.PnL - trade.Fees

		if balance > peak {
			peak = balance
		}

		drawdown := peak - balance
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	maxDrawdownPercent := 0.0
	if peak > 0 {
		maxDrawdownPercent = (maxDrawdown / peak) * 100
	}

	return maxDrawdown, maxDrawdownPercent
}
