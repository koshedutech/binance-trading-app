package backtest

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/patterns"
	"fmt"
	"time"
)

// BacktestEngine runs historical strategy validation
type BacktestEngine struct {
	startDate  time.Time
	endDate    time.Time
	initialCapital float64
	commission float64 // Trading fees as percentage
}

// BacktestResult contains backtest performance metrics
type BacktestResult struct {
	TotalTrades      int
	WinningTrades    int
	LosingTrades     int
	WinRate          float64
	TotalProfit      float64
	TotalLoss        float64
	NetProfit        float64
	ROI              float64 // Return on Investment %
	MaxDrawdown      float64
	AverageWin       float64
	AverageLoss      float64
	ProfitFactor     float64
	SharpeRatio      float64
	Trades           []Trade
	EquityCurve      []EquityPoint
	PatternStats     map[patterns.PatternType]*PatternPerformance
}

// Trade represents a single backtest trade
type Trade struct {
	EntryTime   time.Time
	ExitTime    time.Time
	EntryPrice  float64
	ExitPrice   float64
	Quantity    float64
	Side        string // "BUY" or "SELL"
	ProfitLoss  float64
	PLPercent   float64
	Pattern     patterns.PatternType
	StopLoss    float64
	TakeProfit  float64
	ExitReason  string // "stop_loss", "take_profit", "signal", "timeout"
}

// EquityPoint represents account balance at a point in time
type EquityPoint struct {
	Timestamp time.Time
	Equity    float64
}

// PatternPerformance tracks performance by pattern type
type PatternPerformance struct {
	PatternType   patterns.PatternType
	TotalTrades   int
	Wins          int
	Losses        int
	WinRate       float64
	AvgProfit     float64
	AvgLoss       float64
	NetProfit     float64
}

// StrategyFunc defines a backtest strategy function
type StrategyFunc func(candles []binance.Kline, currentIndex int) (*Signal, error)

// Signal represents a trading signal
type Signal struct {
	Action     string  // "BUY" or "SELL"
	Price      float64
	StopLoss   float64
	TakeProfit float64
	Pattern    patterns.PatternType
}

// NewBacktestEngine creates a new backtest engine
func NewBacktestEngine(startDate, endDate time.Time, initialCapital, commission float64) *BacktestEngine {
	return &BacktestEngine{
		startDate:      startDate,
		endDate:        endDate,
		initialCapital: initialCapital,
		commission:     commission,
	}
}

// RunBacktest executes a strategy against historical data
func (be *BacktestEngine) RunBacktest(candles []binance.Kline, strategy StrategyFunc) (*BacktestResult, error) {
	result := &BacktestResult{
		Trades:       make([]Trade, 0),
		EquityCurve:  make([]EquityPoint, 0),
		PatternStats: make(map[patterns.PatternType]*PatternPerformance),
	}

	currentEquity := be.initialCapital
	var openTrade *Trade

	// Iterate through candles
	for i := 50; i < len(candles); i++ { // Start at 50 to have enough history
		currentCandle := candles[i]
		currentPrice := currentCandle.Close

		// Check for exit conditions if we have an open trade
		if openTrade != nil {
			exitReason := ""
			exitPrice := currentPrice

			// Check stop loss
			if currentPrice <= openTrade.StopLoss {
				exitReason = "stop_loss"
				exitPrice = openTrade.StopLoss
			}

			// Check take profit
			if currentPrice >= openTrade.TakeProfit {
				exitReason = "take_profit"
				exitPrice = openTrade.TakeProfit
			}

			// Exit if reason found
			if exitReason != "" {
				openTrade.ExitTime = time.Unix(currentCandle.CloseTime/1000, 0)
				openTrade.ExitPrice = exitPrice
				openTrade.ExitReason = exitReason

				// Calculate P&L
				priceDiff := exitPrice - openTrade.EntryPrice
				grossPL := priceDiff * openTrade.Quantity
				commission := (openTrade.EntryPrice * openTrade.Quantity * be.commission) +
					(exitPrice * openTrade.Quantity * be.commission)
				openTrade.ProfitLoss = grossPL - commission
				openTrade.PLPercent = (priceDiff / openTrade.EntryPrice) * 100

				// Update equity
				currentEquity += openTrade.ProfitLoss

				// Record trade
				result.Trades = append(result.Trades, *openTrade)

				// Update pattern stats
				be.updatePatternStats(result, openTrade)

				// Record equity point
				result.EquityCurve = append(result.EquityCurve, EquityPoint{
					Timestamp: openTrade.ExitTime,
					Equity:    currentEquity,
				})

				// Close position
				openTrade = nil
			}
		}

		// Look for new entry signal if no open trade
		if openTrade == nil {
			signal, err := strategy(candles[:i+1], i)
			if err != nil {
				continue
			}

			if signal != nil && signal.Action == "BUY" {
				// Calculate position size (use 10% of equity per trade)
				positionSize := currentEquity * 0.10
				quantity := positionSize / signal.Price

				openTrade = &Trade{
					EntryTime:  time.Unix(currentCandle.CloseTime/1000, 0),
					EntryPrice: signal.Price,
					Quantity:   quantity,
					Side:       "BUY",
					Pattern:    signal.Pattern,
					StopLoss:   signal.StopLoss,
					TakeProfit: signal.TakeProfit,
				}
			}
		}
	}

	// Close any remaining open trade at end of backtest
	if openTrade != nil {
		lastCandle := candles[len(candles)-1]
		openTrade.ExitTime = time.Unix(lastCandle.CloseTime/1000, 0)
		openTrade.ExitPrice = lastCandle.Close
		openTrade.ExitReason = "backtest_end"

		priceDiff := openTrade.ExitPrice - openTrade.EntryPrice
		grossPL := priceDiff * openTrade.Quantity
		commission := (openTrade.EntryPrice * openTrade.Quantity * be.commission) +
			(openTrade.ExitPrice * openTrade.Quantity * be.commission)
		openTrade.ProfitLoss = grossPL - commission
		openTrade.PLPercent = (priceDiff / openTrade.EntryPrice) * 100

		currentEquity += openTrade.ProfitLoss
		result.Trades = append(result.Trades, *openTrade)
		be.updatePatternStats(result, openTrade)
	}

	// Calculate final metrics
	be.calculateMetrics(result, currentEquity)

	return result, nil
}

// updatePatternStats updates performance stats for a pattern type
func (be *BacktestEngine) updatePatternStats(result *BacktestResult, trade *Trade) {
	stats, exists := result.PatternStats[trade.Pattern]
	if !exists {
		stats = &PatternPerformance{
			PatternType: trade.Pattern,
		}
		result.PatternStats[trade.Pattern] = stats
	}

	stats.TotalTrades++
	if trade.ProfitLoss > 0 {
		stats.Wins++
		stats.AvgProfit = ((stats.AvgProfit * float64(stats.Wins-1)) + trade.ProfitLoss) / float64(stats.Wins)
	} else {
		stats.Losses++
		stats.AvgLoss = ((stats.AvgLoss * float64(stats.Losses-1)) + trade.ProfitLoss) / float64(stats.Losses)
	}
	stats.NetProfit += trade.ProfitLoss
	if stats.TotalTrades > 0 {
		stats.WinRate = (float64(stats.Wins) / float64(stats.TotalTrades)) * 100
	}
}

// calculateMetrics calculates final backtest metrics
func (be *BacktestEngine) calculateMetrics(result *BacktestResult, finalEquity float64) {
	result.TotalTrades = len(result.Trades)

	for _, trade := range result.Trades {
		if trade.ProfitLoss > 0 {
			result.WinningTrades++
			result.TotalProfit += trade.ProfitLoss
		} else {
			result.LosingTrades++
			result.TotalLoss += abs(trade.ProfitLoss)
		}
	}

	// Win rate
	if result.TotalTrades > 0 {
		result.WinRate = (float64(result.WinningTrades) / float64(result.TotalTrades)) * 100
	}

	// Average win/loss
	if result.WinningTrades > 0 {
		result.AverageWin = result.TotalProfit / float64(result.WinningTrades)
	}
	if result.LosingTrades > 0 {
		result.AverageLoss = result.TotalLoss / float64(result.LosingTrades)
	}

	// Net profit and ROI
	result.NetProfit = finalEquity - be.initialCapital
	result.ROI = (result.NetProfit / be.initialCapital) * 100

	// Profit factor
	if result.TotalLoss > 0 {
		result.ProfitFactor = result.TotalProfit / result.TotalLoss
	}

	// Max drawdown
	result.MaxDrawdown = be.calculateMaxDrawdown(result.EquityCurve)

	// Sharpe ratio (simplified)
	result.SharpeRatio = be.calculateSharpeRatio(result.Trades)
}

// calculateMaxDrawdown calculates maximum equity drawdown
func (be *BacktestEngine) calculateMaxDrawdown(equityCurve []EquityPoint) float64 {
	if len(equityCurve) == 0 {
		return 0
	}

	maxDrawdown := 0.0
	peak := equityCurve[0].Equity

	for _, point := range equityCurve {
		if point.Equity > peak {
			peak = point.Equity
		}
		drawdown := ((peak - point.Equity) / peak) * 100
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return maxDrawdown
}

// calculateSharpeRatio calculates risk-adjusted return
func (be *BacktestEngine) calculateSharpeRatio(trades []Trade) float64 {
	if len(trades) == 0 {
		return 0
	}

	// Calculate average return
	totalReturn := 0.0
	for _, trade := range trades {
		totalReturn += trade.PLPercent
	}
	avgReturn := totalReturn / float64(len(trades))

	// Calculate standard deviation
	variance := 0.0
	for _, trade := range trades {
		diff := trade.PLPercent - avgReturn
		variance += diff * diff
	}
	stdDev := sqrt(variance / float64(len(trades)))

	if stdDev == 0 {
		return 0
	}

	// Sharpe ratio (assuming 0 risk-free rate)
	return avgReturn / stdDev
}

// PrintResults prints backtest results
func (be *BacktestEngine) PrintResults(result *BacktestResult) {
	fmt.Println("\n=== BACKTEST RESULTS ===")
	fmt.Printf("Total Trades: %d\n", result.TotalTrades)
	fmt.Printf("Winning Trades: %d (%.1f%%)\n", result.WinningTrades, result.WinRate)
	fmt.Printf("Losing Trades: %d\n", result.LosingTrades)
	fmt.Printf("Net Profit: $%.2f\n", result.NetProfit)
	fmt.Printf("ROI: %.2f%%\n", result.ROI)
	fmt.Printf("Profit Factor: %.2f\n", result.ProfitFactor)
	fmt.Printf("Max Drawdown: %.2f%%\n", result.MaxDrawdown)
	fmt.Printf("Average Win: $%.2f\n", result.AverageWin)
	fmt.Printf("Average Loss: $%.2f\n", result.AverageLoss)
	fmt.Printf("Sharpe Ratio: %.2f\n", result.SharpeRatio)

	fmt.Println("\n=== PATTERN PERFORMANCE ===")
	for patternType, stats := range result.PatternStats {
		fmt.Printf("%s: %d trades, %.1f%% win rate, Net: $%.2f\n",
			patternType, stats.TotalTrades, stats.WinRate, stats.NetProfit)
	}
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func sqrt(x float64) float64 {
	// Simple Newton's method for square root
	if x == 0 {
		return 0
	}
	guess := x / 2
	for i := 0; i < 10; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}
