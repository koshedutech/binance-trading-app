// Package research provides data structures and services for research infrastructure.
// Story 15.7: Simple Backtest Engine - Core backtesting implementation
package research

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"binance-trading-bot/internal/research/indicators"
)

// BacktestEngine runs backtests on historical data using cached features.
type BacktestEngine struct {
	featureCache      *FeatureCache
	featureCalculator *FeatureCalculator
	candleRepo        CandleRepository
}

// CandleRepository interface for fetching candles (implemented by database)
type CandleRepository interface {
	GetCandles(ctx context.Context, symbol string, timeframe Timeframe, startTime, endTime time.Time) ([]*ResearchCandle, error)
}

// NewBacktestEngine creates a new BacktestEngine.
func NewBacktestEngine(featureCache *FeatureCache, featureCalculator *FeatureCalculator, candleRepo CandleRepository) *BacktestEngine {
	return &BacktestEngine{
		featureCache:      featureCache,
		featureCalculator: featureCalculator,
		candleRepo:        candleRepo,
	}
}

// HasFeatures checks if features are cached for a specific symbol/timeframe.
func (e *BacktestEngine) HasFeatures(ctx context.Context, symbol string, timeframe Timeframe) bool {
	if e.featureCache == nil {
		return false
	}
	return e.featureCache.HasFeatures(ctx, symbol, timeframe)
}

// CountFeatures returns the number of cached features for a symbol/timeframe.
func (e *BacktestEngine) CountFeatures(ctx context.Context, symbol string, timeframe Timeframe) int64 {
	if e.featureCache == nil {
		return 0
	}
	return e.featureCache.CountFeatures(ctx, symbol, timeframe)
}

// GetAllFeatureCounts returns feature counts for all symbol/timeframe combinations efficiently.
// This is much faster than calling CountFeatures for each combination individually.
func (e *BacktestEngine) GetAllFeatureCounts(ctx context.Context) (map[string]map[string]int64, error) {
	if e.featureCache == nil {
		return make(map[string]map[string]int64), nil
	}
	return e.featureCache.GetAllFeatureCounts(ctx)
}

// RunBacktest executes a backtest based on the request parameters.
func (e *BacktestEngine) RunBacktest(ctx context.Context, req *BacktestRequest) (*BacktestResult, error) {
	startTime := time.Now()

	// Set defaults and validate
	req.SetDefaults()
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Validate feature names in conditions
	for i, cond := range req.EntryConditions {
		if !ValidateFeatureName(cond.Feature) {
			return nil, fmt.Errorf("invalid feature name in condition %d: %s", i, cond.Feature)
		}
	}

	// Run backtest for each coin
	allTrades := []Trade{}
	coinBreakdowns := make([]CoinBreakdown, 0, len(req.Coins))

	for _, symbol := range req.Coins {
		trades, err := e.runCoinBacktest(ctx, symbol, req)
		if err != nil {
			log.Printf("[BACKTEST] Error running backtest for %s: %v", symbol, err)
			// Continue with other coins
			continue
		}

		allTrades = append(allTrades, trades...)

		// Calculate coin-specific stats
		breakdown := e.calculateCoinBreakdown(symbol, trades)
		coinBreakdowns = append(coinBreakdowns, breakdown)
	}

	// Calculate overall statistics
	summary := e.calculateSummary(allTrades, req.Timeframe)

	// Perform statistical validation
	validation := e.calculateValidation(allTrades)

	result := &BacktestResult{
		Summary:         summary,
		ByCoin:          coinBreakdowns,
		Validation:      validation,
		ExecutionTimeMs: time.Since(startTime).Milliseconds(),
		Trades:          allTrades, // Include trades for detailed analysis
		Request:         *req,
	}

	return result, nil
}

// runCoinBacktest runs the backtest for a single coin.
func (e *BacktestEngine) runCoinBacktest(ctx context.Context, symbol string, req *BacktestRequest) ([]Trade, error) {
	// Fetch candles for the date range
	// Include lookback buffer for feature calculation
	lookbackBuffer := time.Duration(RecommendedLookback) * req.Timeframe.Duration()
	adjustedFrom := req.From.Add(-lookbackBuffer)

	candles, err := e.candleRepo.GetCandles(ctx, symbol, req.Timeframe, adjustedFrom, req.To)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch candles: %w", err)
	}

	if len(candles) < MinimumLookback {
		return nil, fmt.Errorf("insufficient candles: have %d, need at least %d", len(candles), MinimumLookback)
	}

	log.Printf("[BACKTEST] %s: Loaded %d candles from %s to %s",
		symbol, len(candles), candles[0].OpenTime.Format(time.RFC3339), candles[len(candles)-1].OpenTime.Format(time.RFC3339))

	// Try to get cached features first
	features, err := e.getCachedOrCalculateFeatures(ctx, symbol, req.Timeframe, candles, req.From, req.To)
	if err != nil {
		return nil, fmt.Errorf("failed to get features: %w", err)
	}

	// Create condition evaluator
	evaluator := NewConditionEvaluator(req.EntryConditions, req.EntryLogic)

	// Simulate trades
	trades := e.simulateTrades(candles, features, evaluator, req)

	log.Printf("[BACKTEST] %s: Generated %d trades", symbol, len(trades))

	return trades, nil
}

// getCachedOrCalculateFeatures retrieves features from cache or calculates them.
func (e *BacktestEngine) getCachedOrCalculateFeatures(ctx context.Context, symbol string, timeframe Timeframe, candles []*ResearchCandle, from, to time.Time) ([]*CandleFeatures, error) {
	// Build a map from timestamp to candle index for quick lookup
	candleMap := make(map[int64]int, len(candles))
	for i, c := range candles {
		candleMap[c.OpenTime.Unix()] = i
	}

	// Try to get features from cache
	var cachedFeatures []*CandleFeatures
	var err error

	if e.featureCache != nil {
		cachedFeatures, err = e.featureCache.GetBatch(ctx, symbol, timeframe, from, to)
		if err != nil {
			log.Printf("[BACKTEST] Cache retrieval failed, will calculate: %v", err)
		}
	}

	// Count cache hits
	hitCount := 0
	if cachedFeatures != nil {
		for _, f := range cachedFeatures {
			if f != nil {
				hitCount++
			}
		}
	}

	// Calculate missing features
	// We'll calculate features for all candles in the target range
	targetInterval := timeframe.Duration()
	fromAligned := alignToInterval(from, targetInterval)

	// Prepare result slice
	var resultFeatures []*CandleFeatures
	if cachedFeatures != nil && len(cachedFeatures) > 0 {
		resultFeatures = cachedFeatures
	} else {
		// Calculate number of candles in range
		candleCount := int(to.Sub(fromAligned) / targetInterval) + 1
		resultFeatures = make([]*CandleFeatures, candleCount)
	}

	// Calculate features for candles without cache
	calculatedCount := 0
	for i := range resultFeatures {
		if resultFeatures[i] != nil {
			continue // Already cached
		}

		timestamp := fromAligned.Add(time.Duration(i) * targetInterval)
		candleIdx, ok := candleMap[timestamp.Unix()]
		if !ok {
			continue // No candle for this timestamp
		}

		// Need enough lookback candles
		if candleIdx < MinimumLookback-1 {
			continue
		}

		// Calculate features using lookback window
		window := candles[:candleIdx+1]
		features, err := e.featureCalculator.Calculate(window)
		if err != nil {
			log.Printf("[BACKTEST] Failed to calculate features for %s at %s: %v", symbol, timestamp, err)
			continue
		}

		resultFeatures[i] = features
		calculatedCount++

		// Cache the calculated features
		if e.featureCache != nil {
			if err := e.featureCache.Set(ctx, features); err != nil {
				log.Printf("[BACKTEST] Failed to cache features: %v", err)
			}
		}
	}

	log.Printf("[BACKTEST] Features: %d cached, %d calculated", hitCount, calculatedCount)

	return resultFeatures, nil
}

// simulateTrades simulates trades based on entry conditions.
func (e *BacktestEngine) simulateTrades(candles []*ResearchCandle, features []*CandleFeatures, evaluator *ConditionEvaluator, req *BacktestRequest) []Trade {
	trades := []Trade{}

	// Build candle index by timestamp for quick lookup
	candleByTime := make(map[int64]*ResearchCandle, len(candles))
	candleIndex := make(map[int64]int, len(candles))
	for i, c := range candles {
		candleByTime[c.OpenTime.Unix()] = c
		candleIndex[c.OpenTime.Unix()] = i
	}

	// Pre-calculate ATR for all candles
	atrByCandle := e.preCalculateATR(candles, req.ATRPeriod)

	// Track active trade (only one trade at a time)
	var activeTrade *Trade

	// Iterate through features to find entry signals
	for _, feature := range features {
		if feature == nil {
			continue
		}

		// Skip if outside requested range
		if feature.OpenTime.Before(req.From) || feature.OpenTime.After(req.To) {
			continue
		}

		// If we have an active trade, check for exit
		if activeTrade != nil {
			candle := candleByTime[feature.OpenTime.Unix()]
			if candle == nil {
				continue
			}

			// Check exit conditions
			exitResult := e.checkExit(activeTrade, candle, candleIndex[feature.OpenTime.Unix()]-candleIndex[activeTrade.EntryTime.Unix()], req)
			if exitResult != nil {
				trades = append(trades, *exitResult)
				activeTrade = nil
			}
			continue
		}

		// Check entry conditions
		if evaluator.Evaluate(feature) {
			// Entry is triggered at close of signal candle, but actual execution
			// happens at the open of the NEXT candle to avoid look-ahead bias.
			// The features are calculated using data up to and including the close,
			// so we can only act on the next bar's open.
			signalCandleIdx := candleIndex[feature.OpenTime.Unix()]
			nextCandleIdx := signalCandleIdx + 1
			if nextCandleIdx >= len(candles) {
				continue // No next candle to enter on
			}

			nextCandle := candles[nextCandleIdx]
			atr := atrByCandle[signalCandleIdx] // Use ATR from signal candle
			if math.IsNaN(atr) || atr <= 0 {
				continue // Skip if ATR is invalid
			}

			// Create new trade - enter at next candle's open
			entryPrice := nextCandle.Open
			stopLoss := entryPrice - (atr * req.SLATRMultiplier)
			takeProfit := entryPrice + (atr * req.TPATRMultiplier)

			activeTrade = &Trade{
				Symbol:     feature.Symbol,
				EntryTime:  nextCandle.OpenTime,
				EntryPrice: entryPrice,
				EntryATR:   atr,
				StopLoss:   stopLoss,
				TakeProfit: takeProfit,
			}
		}
	}

	// Close any remaining active trade at end of data
	if activeTrade != nil {
		lastCandle := candles[len(candles)-1]
		trade := e.closeTradeAtTimeout(activeTrade, lastCandle, len(candles)-1-candleIndex[activeTrade.EntryTime.Unix()], req)
		trades = append(trades, trade)
	}

	return trades
}

// preCalculateATR calculates ATR for all candles.
func (e *BacktestEngine) preCalculateATR(candles []*ResearchCandle, period int) []float64 {
	n := len(candles)
	result := make([]float64, n)

	// Initialize with NaN
	for i := range result {
		result[i] = math.NaN()
	}

	if n < period+1 {
		return result
	}

	// Extract price arrays
	highs := make([]float64, n)
	lows := make([]float64, n)
	closes := make([]float64, n)
	for i, c := range candles {
		highs[i] = c.High
		lows[i] = c.Low
		closes[i] = c.Close
	}

	// Calculate ATR for each position
	for i := period; i < n; i++ {
		result[i] = indicators.CalculateATR(highs[:i+1], lows[:i+1], closes[:i+1], period)
	}

	return result
}

// checkExit checks if the active trade should be closed.
func (e *BacktestEngine) checkExit(trade *Trade, candle *ResearchCandle, holdCandles int, req *BacktestRequest) *Trade {
	slHit := candle.Low <= trade.StopLoss
	tpHit := candle.High >= trade.TakeProfit

	// When both SL and TP could be hit in the same candle, determine order
	// by checking which level the open price is closer to.
	// If open is closer to SL, assume SL hit first (pessimistic assumption for longs).
	if slHit && tpHit {
		distToSL := math.Abs(candle.Open - trade.StopLoss)
		distToTP := math.Abs(candle.Open - trade.TakeProfit)
		if distToSL <= distToTP {
			// SL hit first
			return e.closeTrade(trade, candle, holdCandles, trade.StopLoss, OutcomeLoss, req)
		}
		// TP hit first
		return e.closeTrade(trade, candle, holdCandles, trade.TakeProfit, OutcomeWin, req)
	}

	// Check stop loss (price goes below SL during the candle)
	if slHit {
		return e.closeTrade(trade, candle, holdCandles, trade.StopLoss, OutcomeLoss, req)
	}

	// Check take profit (price goes above TP during the candle)
	if tpHit {
		return e.closeTrade(trade, candle, holdCandles, trade.TakeProfit, OutcomeWin, req)
	}

	// Check timeout
	if holdCandles >= req.MaxHoldCandles {
		closedTrade := e.closeTradeAtTimeout(trade, candle, holdCandles, req)
		return &closedTrade
	}

	return nil // Trade still active
}

// closeTrade closes a trade with the specified outcome.
func (e *BacktestEngine) closeTrade(trade *Trade, exitCandle *ResearchCandle, holdCandles int, exitPrice float64, outcome TradeOutcome, req *BacktestRequest) *Trade {
	pnlPercent := ((exitPrice - trade.EntryPrice) / trade.EntryPrice) * 100
	pnlATR := (exitPrice - trade.EntryPrice) / trade.EntryATR

	// Calculate fees
	feesPercent := 0.0
	if req.IncludeFees {
		feesPercent = req.FeePercent * 2 // Entry + exit
	}

	return &Trade{
		Symbol:        trade.Symbol,
		EntryTime:     trade.EntryTime,
		EntryPrice:    trade.EntryPrice,
		EntryATR:      trade.EntryATR,
		StopLoss:      trade.StopLoss,
		TakeProfit:    trade.TakeProfit,
		ExitTime:      exitCandle.OpenTime,
		ExitPrice:     exitPrice,
		Outcome:       outcome,
		HoldCandles:   holdCandles,
		PnLPercent:    pnlPercent,
		PnLATR:        pnlATR,
		FeesPercent:   feesPercent,
		NetPnLPercent: pnlPercent - feesPercent,
	}
}

// closeTradeAtTimeout closes a trade due to timeout.
func (e *BacktestEngine) closeTradeAtTimeout(trade *Trade, exitCandle *ResearchCandle, holdCandles int, req *BacktestRequest) Trade {
	exitPrice := exitCandle.Close
	pnlPercent := ((exitPrice - trade.EntryPrice) / trade.EntryPrice) * 100
	pnlATR := (exitPrice - trade.EntryPrice) / trade.EntryATR

	feesPercent := 0.0
	if req.IncludeFees {
		feesPercent = req.FeePercent * 2
	}

	return Trade{
		Symbol:        trade.Symbol,
		EntryTime:     trade.EntryTime,
		EntryPrice:    trade.EntryPrice,
		EntryATR:      trade.EntryATR,
		StopLoss:      trade.StopLoss,
		TakeProfit:    trade.TakeProfit,
		ExitTime:      exitCandle.OpenTime,
		ExitPrice:     exitPrice,
		Outcome:       OutcomeTimeout,
		HoldCandles:   holdCandles,
		PnLPercent:    pnlPercent,
		PnLATR:        pnlATR,
		FeesPercent:   feesPercent,
		NetPnLPercent: pnlPercent - feesPercent,
	}
}

// calculateSummary calculates aggregate statistics from all trades.
func (e *BacktestEngine) calculateSummary(trades []Trade, timeframe Timeframe) BacktestSummary {
	if len(trades) == 0 {
		return BacktestSummary{}
	}

	summary := BacktestSummary{
		TotalTrades: len(trades),
	}

	// Basic counts
	var totalPnL, grossProfit, grossLoss, totalFees float64
	var totalPnLATR float64
	var totalHoldCandles int
	var winPercents, lossPercents []float64
	var allPnLs []float64

	// Track consecutive wins/losses
	currentWinStreak, currentLossStreak := 0, 0

	for _, t := range trades {
		switch t.Outcome {
		case OutcomeWin:
			summary.Wins++
			currentWinStreak++
			currentLossStreak = 0
			if currentWinStreak > summary.ConsecutiveWins {
				summary.ConsecutiveWins = currentWinStreak
			}
		case OutcomeLoss:
			summary.Losses++
			currentLossStreak++
			currentWinStreak = 0
			if currentLossStreak > summary.ConsecutiveLosses {
				summary.ConsecutiveLosses = currentLossStreak
			}
		case OutcomeTimeout:
			summary.Timeouts++
			// Reset streaks on timeout
			currentWinStreak = 0
			currentLossStreak = 0
		}

		totalPnL += t.NetPnLPercent
		totalPnLATR += t.PnLATR
		totalFees += t.FeesPercent
		totalHoldCandles += t.HoldCandles
		allPnLs = append(allPnLs, t.NetPnLPercent)

		if t.PnLPercent > 0 {
			grossProfit += t.PnLPercent
			winPercents = append(winPercents, t.PnLPercent)
		} else {
			grossLoss += math.Abs(t.PnLPercent)
			lossPercents = append(lossPercents, t.PnLPercent)
		}

		// Track largest win/loss
		if t.PnLPercent > summary.LargestWinPercent {
			summary.LargestWinPercent = t.PnLPercent
		}
		if t.PnLPercent < summary.LargestLossPercent {
			summary.LargestLossPercent = t.PnLPercent
		}
	}

	// Calculate averages
	n := float64(len(trades))
	summary.WinRate = float64(summary.Wins) / n * 100
	summary.ExpectedPercent = totalPnL / n
	summary.ExpectedATR = totalPnLATR / n
	summary.AvgHoldCandles = float64(totalHoldCandles) / n
	summary.TotalGrossProfit = grossProfit
	summary.TotalGrossLoss = -grossLoss // Store as negative
	summary.TotalNetProfit = totalPnL
	summary.TotalFeesPercent = totalFees

	if len(winPercents) > 0 {
		sum := 0.0
		for _, w := range winPercents {
			sum += w
		}
		summary.AvgWinPercent = sum / float64(len(winPercents))
	}

	if len(lossPercents) > 0 {
		sum := 0.0
		for _, l := range lossPercents {
			sum += l
		}
		summary.AvgLossPercent = sum / float64(len(lossPercents))
	}

	// Profit factor
	if grossLoss > 0 {
		summary.ProfitFactor = grossProfit / grossLoss
	} else if grossProfit > 0 {
		// All winning trades - use large finite value (Inf doesn't serialize to JSON properly)
		summary.ProfitFactor = 999999.0
	}
	// Note: if both grossProfit and grossLoss are 0, ProfitFactor stays 0

	// Sharpe ratio (annualized based on timeframe)
	// Sharpe = (mean return) / (std dev of returns) * sqrt(periods_per_year)
	if len(allPnLs) > 1 {
		mean := totalPnL / n
		variance := 0.0
		for _, pnl := range allPnLs {
			diff := pnl - mean
			variance += diff * diff
		}
		stdDev := math.Sqrt(variance / (n - 1))
		if stdDev > 0 {
			// Calculate annualization factor based on timeframe
			// Assuming 365 trading days * 24 hours for crypto
			periodsPerYear := float64(365 * 24 * 60) / timeframe.Duration().Minutes()
			summary.SharpeRatio = (mean / stdDev) * math.Sqrt(periodsPerYear)
		}
	}

	// Maximum drawdown
	summary.MaxDrawdownPercent = e.calculateMaxDrawdown(trades)

	return summary
}

// calculateMaxDrawdown calculates the maximum drawdown percentage.
func (e *BacktestEngine) calculateMaxDrawdown(trades []Trade) float64 {
	if len(trades) == 0 {
		return 0
	}

	// Track cumulative P&L
	cumulative := 0.0
	peak := 0.0
	maxDrawdown := 0.0

	for _, t := range trades {
		cumulative += t.NetPnLPercent
		if cumulative > peak {
			peak = cumulative
		}
		drawdown := peak - cumulative
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return maxDrawdown // Return as positive percentage (conventional drawdown representation)
}

// calculateCoinBreakdown calculates statistics for a single coin.
func (e *BacktestEngine) calculateCoinBreakdown(symbol string, trades []Trade) CoinBreakdown {
	breakdown := CoinBreakdown{
		Symbol: symbol,
	}

	if len(trades) == 0 {
		return breakdown
	}

	breakdown.TotalTrades = len(trades)

	var totalPnLATR float64
	var totalHoldCandles int
	var netProfit float64

	for _, t := range trades {
		switch t.Outcome {
		case OutcomeWin:
			breakdown.Wins++
		case OutcomeLoss:
			breakdown.Losses++
		case OutcomeTimeout:
			breakdown.Timeouts++
		}

		totalPnLATR += t.PnLATR
		totalHoldCandles += t.HoldCandles
		netProfit += t.NetPnLPercent
	}

	n := float64(len(trades))
	breakdown.WinRate = float64(breakdown.Wins) / n * 100
	breakdown.ExpectedATR = totalPnLATR / n
	breakdown.AvgHoldCandles = float64(totalHoldCandles) / n
	breakdown.NetProfit = netProfit

	return breakdown
}

// calculateValidation performs statistical validation on the results.
func (e *BacktestEngine) calculateValidation(trades []Trade) ValidationResult {
	const minSampleSize = 30

	validation := ValidationResult{
		MinSampleSize:      minSampleSize,
		SampleSizeAdequate: len(trades) >= minSampleSize,
	}

	if len(trades) < 2 {
		validation.PValue = 1.0
		validation.Significant = false
		return validation
	}

	// Perform one-sample t-test against zero mean
	// H0: mean return = 0
	// H1: mean return != 0

	// Calculate mean
	mean := 0.0
	for _, t := range trades {
		mean += t.NetPnLPercent
	}
	mean /= float64(len(trades))

	// Calculate standard deviation
	variance := 0.0
	for _, t := range trades {
		diff := t.NetPnLPercent - mean
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(trades)-1))

	// Calculate t-statistic
	if stdDev > 0 {
		standardError := stdDev / math.Sqrt(float64(len(trades)))
		tStat := mean / standardError

		// Approximate p-value using normal distribution for large samples
		// For more accurate p-value, would need t-distribution
		validation.PValue = e.approximatePValue(tStat, len(trades)-1)
		validation.Significant = validation.PValue < 0.05
	} else {
		validation.PValue = 1.0
		validation.Significant = false
	}

	return validation
}

// approximatePValue approximates the two-tailed p-value for t-test.
// Uses normal approximation for simplicity (accurate for df > 30).
func (e *BacktestEngine) approximatePValue(tStat float64, df int) float64 {
	// Use normal approximation for large df
	absT := math.Abs(tStat)

	// Approximate CDF of standard normal
	// Using error function: P(Z <= z) = 0.5 * (1 + erf(z / sqrt(2)))
	cdf := 0.5 * (1 + math.Erf(absT/math.Sqrt(2)))

	// Two-tailed p-value
	pValue := 2 * (1 - cdf)

	return pValue
}

// alignToInterval aligns a timestamp to the nearest interval boundary.
func alignToInterval(t time.Time, interval time.Duration) time.Time {
	unix := t.Unix()
	intervalSecs := int64(interval.Seconds())
	aligned := unix - (unix % intervalSecs)
	return time.Unix(aligned, 0).UTC()
}

// RunBacktestWithOptions provides additional options for backtest execution.
type BacktestOptions struct {
	IncludeTrades bool // Include individual trades in result
	MaxTrades     int  // Maximum trades to return (0 = all)
}

// RunBacktestWithOptions runs a backtest with additional options.
func (e *BacktestEngine) RunBacktestWithOptions(ctx context.Context, req *BacktestRequest, opts BacktestOptions) (*BacktestResult, error) {
	result, err := e.RunBacktest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Apply options
	if !opts.IncludeTrades {
		result.Trades = nil
	} else if opts.MaxTrades > 0 && len(result.Trades) > opts.MaxTrades {
		// Sort by time and take most recent
		sort.Slice(result.Trades, func(i, j int) bool {
			return result.Trades[i].EntryTime.After(result.Trades[j].EntryTime)
		})
		result.Trades = result.Trades[:opts.MaxTrades]
	}

	return result, nil
}
