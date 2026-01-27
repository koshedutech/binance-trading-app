// Package research provides data structures and services for research infrastructure.
// Story 15.8: Walk-Forward Backtest Engine - Rolling window validation for pattern discovery.
package research

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"
)

// WalkForwardConfig configures the walk-forward testing parameters.
type WalkForwardConfig struct {
	// TrainMonths specifies the duration of each training period in months.
	TrainMonths int `json:"train_months"`

	// TestMonths specifies the duration of each test (out-of-sample) period in months.
	TestMonths int `json:"test_months"`

	// StepMonths specifies how far to advance the window for each iteration.
	// If StepMonths equals TestMonths, there is no overlap between test periods.
	StepMonths int `json:"step_months"`

	// HoldoutPercent specifies what percentage of the final data to reserve
	// for hold-out (final validation) testing. Range: 0-50.
	HoldoutPercent int `json:"holdout_percent"`
}

// Validate checks if the WalkForwardConfig is valid.
func (c *WalkForwardConfig) Validate() error {
	if c.TrainMonths < 1 {
		return fmt.Errorf("train_months must be at least 1")
	}
	if c.TestMonths < 1 {
		return fmt.Errorf("test_months must be at least 1")
	}
	if c.StepMonths < 1 {
		return fmt.Errorf("step_months must be at least 1")
	}
	if c.HoldoutPercent < 0 || c.HoldoutPercent > 50 {
		return fmt.Errorf("holdout_percent must be between 0 and 50")
	}
	return nil
}

// SetDefaults fills in default values for WalkForwardConfig.
func (c *WalkForwardConfig) SetDefaults() {
	if c.TrainMonths == 0 {
		c.TrainMonths = 6
	}
	if c.TestMonths == 0 {
		c.TestMonths = 3
	}
	if c.StepMonths == 0 {
		c.StepMonths = 3
	}
	if c.HoldoutPercent == 0 {
		c.HoldoutPercent = 20
	}
}

// WalkForwardRequest represents the input parameters for a walk-forward backtest.
type WalkForwardRequest struct {
	// Base backtest parameters
	Coins           []string         `json:"coins"`
	Timeframe       Timeframe        `json:"timeframe"`
	From            time.Time        `json:"from"`
	To              time.Time        `json:"to"`
	EntryConditions []EntryCondition `json:"entry_conditions"`
	EntryLogic      EntryLogic       `json:"entry_logic"`
	SLATRMultiplier float64          `json:"sl_atr_multiplier"`
	TPATRMultiplier float64          `json:"tp_atr_multiplier"`
	ATRPeriod       int              `json:"atr_period"`
	MaxHoldCandles  int              `json:"max_hold_candles"`
	IncludeFees     bool             `json:"include_fees"`
	FeePercent      float64          `json:"fee_percent"`

	// Walk-forward specific configuration
	WalkForwardConfig WalkForwardConfig `json:"walk_forward_config"`
}

// Validate checks if the WalkForwardRequest is valid.
func (r *WalkForwardRequest) Validate() error {
	if len(r.Coins) == 0 {
		return fmt.Errorf("at least one coin is required")
	}
	if !r.Timeframe.IsValid() {
		return fmt.Errorf("invalid timeframe: %s", r.Timeframe)
	}
	if r.From.IsZero() {
		return fmt.Errorf("from date is required")
	}
	if r.To.IsZero() {
		return fmt.Errorf("to date is required")
	}
	if r.To.Before(r.From) {
		return fmt.Errorf("to date must be after from date")
	}
	if len(r.EntryConditions) == 0 {
		return fmt.Errorf("at least one entry condition is required")
	}
	for i, cond := range r.EntryConditions {
		if err := cond.Validate(); err != nil {
			return fmt.Errorf("invalid condition %d: %w", i, err)
		}
	}
	if !r.EntryLogic.IsValid() {
		return fmt.Errorf("invalid entry logic: %s (must be AND or OR)", r.EntryLogic)
	}
	if r.SLATRMultiplier <= 0 {
		return fmt.Errorf("sl_atr_multiplier must be positive")
	}
	if r.TPATRMultiplier <= 0 {
		return fmt.Errorf("tp_atr_multiplier must be positive")
	}
	if r.MaxHoldCandles <= 0 {
		return fmt.Errorf("max_hold_candles must be positive")
	}

	// Validate walk-forward config
	if err := r.WalkForwardConfig.Validate(); err != nil {
		return fmt.Errorf("invalid walk_forward_config: %w", err)
	}

	// Check if date range is sufficient for walk-forward testing
	minMonths := r.WalkForwardConfig.TrainMonths + r.WalkForwardConfig.TestMonths
	actualMonths := monthsBetween(r.From, r.To)
	if actualMonths < minMonths {
		return fmt.Errorf("date range (%d months) too short for walk-forward testing (need at least %d months)",
			actualMonths, minMonths)
	}

	return nil
}

// SetDefaults fills in default values for WalkForwardRequest.
func (r *WalkForwardRequest) SetDefaults() {
	if r.ATRPeriod <= 0 {
		r.ATRPeriod = 14
	}
	if r.EntryLogic == "" {
		r.EntryLogic = LogicAND
	}
	if r.FeePercent == 0 && r.IncludeFees {
		r.FeePercent = 0.04
	}
	r.WalkForwardConfig.SetDefaults()
}

// ToBacktestRequest converts a WalkForwardRequest to a BacktestRequest for a specific period.
func (r *WalkForwardRequest) ToBacktestRequest(from, to time.Time) *BacktestRequest {
	return &BacktestRequest{
		Coins:           r.Coins,
		Timeframe:       r.Timeframe,
		From:            from,
		To:              to,
		EntryConditions: r.EntryConditions,
		EntryLogic:      r.EntryLogic,
		SLATRMultiplier: r.SLATRMultiplier,
		TPATRMultiplier: r.TPATRMultiplier,
		ATRPeriod:       r.ATRPeriod,
		MaxHoldCandles:  r.MaxHoldCandles,
		IncludeFees:     r.IncludeFees,
		FeePercent:      r.FeePercent,
	}
}

// DatePeriod represents a date range.
type DatePeriod struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// PeriodResult holds the backtest results for a single train/test period.
type PeriodResult struct {
	// Train period dates (for reference, training not actually used in simple backtest)
	Train DatePeriod `json:"train"`

	// Test period dates
	Test DatePeriod `json:"test"`

	// Results from running backtest on test period
	TestResults PeriodMetrics `json:"test_results"`
}

// PeriodMetrics holds key metrics for a single period.
type PeriodMetrics struct {
	Trades         int     `json:"trades"`
	Wins           int     `json:"wins"`
	Losses         int     `json:"losses"`
	Timeouts       int     `json:"timeouts"`
	WinRate        float64 `json:"win_rate"`
	ExpectedATR    float64 `json:"expected_atr"`
	ExpectedPct    float64 `json:"expected_percent"`
	ProfitFactor   float64 `json:"profit_factor"`
	MaxDrawdown    float64 `json:"max_drawdown"`
	TotalNetProfit float64 `json:"total_net_profit"`
}

// AggregateTestResults holds aggregated statistics across all test periods.
type AggregateTestResults struct {
	TotalTrades        int     `json:"total_trades"`
	TotalWins          int     `json:"total_wins"`
	TotalLosses        int     `json:"total_losses"`
	TotalTimeouts      int     `json:"total_timeouts"`
	WinRate            float64 `json:"win_rate"`
	ExpectedATR        float64 `json:"expected_atr"`
	ExpectedPct        float64 `json:"expected_percent"`
	ProfitFactor       float64 `json:"profit_factor"`
	MaxDrawdown        float64 `json:"max_drawdown"`
	TotalNetProfit     float64 `json:"total_net_profit"`

	// Consistency metrics
	Consistency         float64 `json:"consistency"`          // Percentage of periods with positive expected ATR
	WorstPeriodWinRate  float64 `json:"worst_period_win_rate"`
	BestPeriodWinRate   float64 `json:"best_period_win_rate"`
	WorstPeriodExpATR   float64 `json:"worst_period_exp_atr"`
	BestPeriodExpATR    float64 `json:"best_period_exp_atr"`
	WinRateStdDev       float64 `json:"win_rate_std_dev"`
	ExpATRStdDev        float64 `json:"exp_atr_std_dev"`
	PeriodsWithProfit   int     `json:"periods_with_profit"`
	TotalPeriods        int     `json:"total_periods"`
}

// HoldoutResults holds the results from the hold-out validation period.
type HoldoutResults struct {
	Period         DatePeriod `json:"period"`
	Trades         int        `json:"trades"`
	Wins           int        `json:"wins"`
	Losses         int        `json:"losses"`
	Timeouts       int        `json:"timeouts"`
	WinRate        float64    `json:"win_rate"`
	ExpectedATR    float64    `json:"expected_atr"`
	ExpectedPct    float64    `json:"expected_percent"`
	ProfitFactor   float64    `json:"profit_factor"`
	MaxDrawdown    float64    `json:"max_drawdown"`
	TotalNetProfit float64    `json:"total_net_profit"`
	Status         string     `json:"status"` // "PASS" or "FAIL"
}

// FinalVerdict provides the overall assessment of the walk-forward test.
type FinalVerdict struct {
	Profitable     bool   `json:"profitable"`      // Overall positive expected value
	Consistent     bool   `json:"consistent"`      // Consistency > 70%
	HoldoutPassed  bool   `json:"holdout_passed"`  // Holdout test passed
	Recommendation string `json:"recommendation"`  // Human-readable recommendation
}

// WalkForwardResult represents the complete result of a walk-forward backtest.
type WalkForwardResult struct {
	// Individual period results
	Periods []PeriodResult `json:"periods"`

	// Aggregated test results across all periods
	AggregateTestResults AggregateTestResults `json:"aggregate_test_results"`

	// Hold-out validation results
	HoldoutResults *HoldoutResults `json:"holdout_results,omitempty"`

	// Final assessment
	FinalVerdict FinalVerdict `json:"final_verdict"`

	// Metadata
	ExecutionTimeMs int64              `json:"execution_time_ms"`
	Request         WalkForwardRequest `json:"request,omitempty"`
}

// WalkForwardEngine runs walk-forward backtests using the BacktestEngine.
type WalkForwardEngine struct {
	backtestEngine *BacktestEngine
}

// NewWalkForwardEngine creates a new WalkForwardEngine.
func NewWalkForwardEngine(backtestEngine *BacktestEngine) *WalkForwardEngine {
	return &WalkForwardEngine{
		backtestEngine: backtestEngine,
	}
}

// RunWalkForward executes a walk-forward backtest.
func (e *WalkForwardEngine) RunWalkForward(ctx context.Context, req *WalkForwardRequest) (*WalkForwardResult, error) {
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

	// Calculate hold-out period
	holdoutStart, holdoutEnd := e.calculateHoldoutPeriod(req)
	effectiveEnd := req.To
	if holdoutStart.Before(req.To) {
		effectiveEnd = holdoutStart
	}

	// Generate rolling window periods
	periods := e.generatePeriods(req.From, effectiveEnd, req.WalkForwardConfig)

	// Validate that holdout period doesn't overlap with the last test period
	if len(periods) > 0 && req.WalkForwardConfig.HoldoutPercent > 0 {
		lastTestEnd := periods[len(periods)-1].Test.To
		if holdoutStart.Before(lastTestEnd) {
			// Adjust holdout start to prevent overlap
			holdoutStart = lastTestEnd
			log.Printf("[WALKFORWARD] Adjusted holdout start to %s to avoid overlap with test periods",
				holdoutStart.Format("2006-01-02"))
		}
	}
	if len(periods) == 0 {
		return nil, fmt.Errorf("no valid periods generated; date range may be too short")
	}

	log.Printf("[WALKFORWARD] Generated %d periods for walk-forward testing", len(periods))

	// Run backtest for each test period
	periodResults := make([]PeriodResult, 0, len(periods))
	allTestTrades := []Trade{}

	for i, period := range periods {
		log.Printf("[WALKFORWARD] Running period %d/%d: Train %s to %s, Test %s to %s",
			i+1, len(periods),
			period.Train.From.Format("2006-01-02"), period.Train.To.Format("2006-01-02"),
			period.Test.From.Format("2006-01-02"), period.Test.To.Format("2006-01-02"))

		// Create backtest request for this test period
		backtestReq := req.ToBacktestRequest(period.Test.From, period.Test.To)

		// Run backtest
		result, err := e.backtestEngine.RunBacktest(ctx, backtestReq)
		if err != nil {
			log.Printf("[WALKFORWARD] Period %d failed: %v", i+1, err)
			continue
		}

		// Extract metrics
		metrics := e.extractPeriodMetrics(result)
		periodResults = append(periodResults, PeriodResult{
			Train:       period.Train,
			Test:        period.Test,
			TestResults: metrics,
		})

		allTestTrades = append(allTestTrades, result.Trades...)
	}

	if len(periodResults) == 0 {
		return nil, fmt.Errorf("all periods failed; no results to aggregate")
	}

	// Calculate aggregate results
	aggregateResults := e.calculateAggregateResults(periodResults, allTestTrades)

	// Run hold-out validation if configured
	var holdoutResults *HoldoutResults
	if req.WalkForwardConfig.HoldoutPercent > 0 && holdoutStart.Before(req.To) {
		log.Printf("[WALKFORWARD] Running hold-out validation: %s to %s",
			holdoutStart.Format("2006-01-02"), holdoutEnd.Format("2006-01-02"))

		backtestReq := req.ToBacktestRequest(holdoutStart, holdoutEnd)
		holdoutResult, err := e.backtestEngine.RunBacktest(ctx, backtestReq)
		if err != nil {
			log.Printf("[WALKFORWARD] Hold-out validation failed: %v", err)
		} else {
			holdoutResults = e.extractHoldoutResults(holdoutResult, holdoutStart, holdoutEnd, aggregateResults)
		}
	}

	// Calculate final verdict
	finalVerdict := e.calculateFinalVerdict(aggregateResults, holdoutResults)

	result := &WalkForwardResult{
		Periods:              periodResults,
		AggregateTestResults: aggregateResults,
		HoldoutResults:       holdoutResults,
		FinalVerdict:         finalVerdict,
		ExecutionTimeMs:      time.Since(startTime).Milliseconds(),
		Request:              *req,
	}

	return result, nil
}

// generatePeriods creates the rolling window periods for walk-forward testing.
// Ensures test periods do not overlap to maintain out-of-sample integrity.
func (e *WalkForwardEngine) generatePeriods(from, to time.Time, config WalkForwardConfig) []PeriodResult {
	periods := []PeriodResult{}

	// Warn if step months is less than test months (would cause overlap)
	// In this case, we use non-overlapping test periods by using testEnd as next testStart
	useNonOverlappingTests := config.StepMonths < config.TestMonths

	// Start with first train period
	trainStart := from
	trainEnd := addMonths(trainStart, config.TrainMonths)
	testStart := trainEnd
	testEnd := addMonths(testStart, config.TestMonths)

	var lastTestEnd time.Time

	for testEnd.Before(to) || testEnd.Equal(to) {
		// Skip if this test period would overlap with the previous one
		if !lastTestEnd.IsZero() && testStart.Before(lastTestEnd) {
			// Adjust testStart to avoid overlap
			testStart = lastTestEnd
			testEnd = addMonths(testStart, config.TestMonths)
			if testEnd.After(to) {
				break
			}
		}

		periods = append(periods, PeriodResult{
			Train: DatePeriod{From: trainStart, To: trainEnd},
			Test:  DatePeriod{From: testStart, To: testEnd},
		})
		lastTestEnd = testEnd

		// Advance the window
		trainStart = addMonths(trainStart, config.StepMonths)
		trainEnd = addMonths(trainStart, config.TrainMonths)

		if useNonOverlappingTests {
			// Use end of last test period as start of next to prevent overlap
			testStart = lastTestEnd
		} else {
			testStart = trainEnd
		}
		testEnd = addMonths(testStart, config.TestMonths)
	}

	// Handle partial final period if significant data remains
	// Only include if at least half a test period remains
	if testStart.Before(to) && (lastTestEnd.IsZero() || !testStart.Before(lastTestEnd)) {
		// Use calendar-accurate half month calculation
		halfTestEnd := addMonths(testStart, config.TestMonths/2)
		if config.TestMonths%2 != 0 {
			// For odd months, add ~15 days for the half
			halfTestEnd = halfTestEnd.AddDate(0, 0, 15)
		}
		if to.After(halfTestEnd) || to.Equal(halfTestEnd) {
			periods = append(periods, PeriodResult{
				Train: DatePeriod{From: trainStart, To: trainEnd},
				Test:  DatePeriod{From: testStart, To: to},
			})
		}
	}

	return periods
}

// calculateHoldoutPeriod determines the hold-out validation period.
func (e *WalkForwardEngine) calculateHoldoutPeriod(req *WalkForwardRequest) (time.Time, time.Time) {
	if req.WalkForwardConfig.HoldoutPercent == 0 {
		return req.To, req.To // No holdout
	}

	totalDuration := req.To.Sub(req.From)
	holdoutDuration := time.Duration(float64(totalDuration) * float64(req.WalkForwardConfig.HoldoutPercent) / 100)

	holdoutStart := req.To.Add(-holdoutDuration)
	return holdoutStart, req.To
}

// extractPeriodMetrics extracts key metrics from a backtest result.
func (e *WalkForwardEngine) extractPeriodMetrics(result *BacktestResult) PeriodMetrics {
	return PeriodMetrics{
		Trades:         result.Summary.TotalTrades,
		Wins:           result.Summary.Wins,
		Losses:         result.Summary.Losses,
		Timeouts:       result.Summary.Timeouts,
		WinRate:        result.Summary.WinRate,
		ExpectedATR:    result.Summary.ExpectedATR,
		ExpectedPct:    result.Summary.ExpectedPercent,
		ProfitFactor:   result.Summary.ProfitFactor,
		MaxDrawdown:    result.Summary.MaxDrawdownPercent,
		TotalNetProfit: result.Summary.TotalNetProfit,
	}
}

// calculateAggregateResults aggregates statistics across all test periods.
func (e *WalkForwardEngine) calculateAggregateResults(periods []PeriodResult, allTrades []Trade) AggregateTestResults {
	if len(periods) == 0 {
		return AggregateTestResults{}
	}

	agg := AggregateTestResults{
		TotalPeriods:       len(periods),
		WorstPeriodWinRate: -1.0, // Sentinel value to detect if never set
		BestPeriodWinRate:  -1.0,
		WorstPeriodExpATR:  math.MaxFloat64,
		BestPeriodExpATR:   -math.MaxFloat64,
	}

	// Collect data for statistics
	winRates := make([]float64, 0, len(periods))
	expATRs := make([]float64, 0, len(periods))
	var totalGrossProfit, totalGrossLoss float64
	periodsWithTrades := 0

	for _, period := range periods {
		metrics := period.TestResults

		agg.TotalTrades += metrics.Trades
		agg.TotalWins += metrics.Wins
		agg.TotalLosses += metrics.Losses
		agg.TotalTimeouts += metrics.Timeouts
		agg.TotalNetProfit += metrics.TotalNetProfit

		// Track best/worst - only consider periods with trades
		if metrics.Trades > 0 {
			periodsWithTrades++
			winRates = append(winRates, metrics.WinRate)
			expATRs = append(expATRs, metrics.ExpectedATR)

			if agg.WorstPeriodWinRate < 0 || metrics.WinRate < agg.WorstPeriodWinRate {
				agg.WorstPeriodWinRate = metrics.WinRate
			}
			if agg.BestPeriodWinRate < 0 || metrics.WinRate > agg.BestPeriodWinRate {
				agg.BestPeriodWinRate = metrics.WinRate
			}
			if metrics.ExpectedATR < agg.WorstPeriodExpATR {
				agg.WorstPeriodExpATR = metrics.ExpectedATR
			}
			if metrics.ExpectedATR > agg.BestPeriodExpATR {
				agg.BestPeriodExpATR = metrics.ExpectedATR
			}

			// Count profitable periods (only among periods that had trades)
			if metrics.ExpectedATR > 0 {
				agg.PeriodsWithProfit++
			}
		}
	}

	// Calculate overall metrics from all trades
	if agg.TotalTrades > 0 {
		agg.WinRate = float64(agg.TotalWins) / float64(agg.TotalTrades) * 100

		// Calculate expected values from all trades
		var totalPnLATR float64
		var totalPnLPct float64
		for _, trade := range allTrades {
			totalPnLATR += trade.PnLATR
			totalPnLPct += trade.NetPnLPercent

			if trade.NetPnLPercent > 0 {
				totalGrossProfit += trade.NetPnLPercent
			} else {
				totalGrossLoss += math.Abs(trade.NetPnLPercent)
			}
		}

		agg.ExpectedATR = totalPnLATR / float64(agg.TotalTrades)
		agg.ExpectedPct = totalPnLPct / float64(agg.TotalTrades)

		// Profit factor
		if totalGrossLoss > 0 {
			agg.ProfitFactor = totalGrossProfit / totalGrossLoss
		} else if totalGrossProfit > 0 {
			agg.ProfitFactor = 999999.0
		}

		// Max drawdown across all trades
		agg.MaxDrawdown = e.calculateMaxDrawdown(allTrades)
	}

	// Consistency metric: percentage of periods with positive expected ATR
	// Only consider periods that had trades (not periods with no signals)
	if periodsWithTrades > 0 {
		agg.Consistency = float64(agg.PeriodsWithProfit) / float64(periodsWithTrades) * 100
	}

	// Calculate standard deviations
	if len(winRates) > 1 {
		agg.WinRateStdDev = e.calculateStdDev(winRates)
	}
	if len(expATRs) > 1 {
		agg.ExpATRStdDev = e.calculateStdDev(expATRs)
	}

	// Fix uninitialized values for edge cases (no periods had trades)
	if agg.WorstPeriodWinRate < 0 {
		agg.WorstPeriodWinRate = 0
	}
	if agg.BestPeriodWinRate < 0 {
		agg.BestPeriodWinRate = 0
	}
	if agg.WorstPeriodExpATR == math.MaxFloat64 {
		agg.WorstPeriodExpATR = 0
	}
	if agg.BestPeriodExpATR == -math.MaxFloat64 {
		agg.BestPeriodExpATR = 0
	}

	return agg
}

// calculateMaxDrawdown calculates maximum drawdown from a series of trades.
func (e *WalkForwardEngine) calculateMaxDrawdown(trades []Trade) float64 {
	if len(trades) == 0 {
		return 0
	}

	// Sort trades by entry time
	sortedTrades := make([]Trade, len(trades))
	copy(sortedTrades, trades)
	sort.Slice(sortedTrades, func(i, j int) bool {
		return sortedTrades[i].EntryTime.Before(sortedTrades[j].EntryTime)
	})

	cumulative := 0.0
	peak := 0.0
	maxDrawdown := 0.0

	for _, t := range sortedTrades {
		cumulative += t.NetPnLPercent
		if cumulative > peak {
			peak = cumulative
		}
		drawdown := peak - cumulative
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return maxDrawdown
}

// calculateStdDev calculates the standard deviation of a slice of float64 values.
func (e *WalkForwardEngine) calculateStdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values) - 1)

	return math.Sqrt(variance)
}

// extractHoldoutResults creates holdout results from a backtest result.
func (e *WalkForwardEngine) extractHoldoutResults(result *BacktestResult, start, end time.Time, agg AggregateTestResults) *HoldoutResults {
	holdout := &HoldoutResults{
		Period:         DatePeriod{From: start, To: end},
		Trades:         result.Summary.TotalTrades,
		Wins:           result.Summary.Wins,
		Losses:         result.Summary.Losses,
		Timeouts:       result.Summary.Timeouts,
		WinRate:        result.Summary.WinRate,
		ExpectedATR:    result.Summary.ExpectedATR,
		ExpectedPct:    result.Summary.ExpectedPercent,
		ProfitFactor:   result.Summary.ProfitFactor,
		MaxDrawdown:    result.Summary.MaxDrawdownPercent,
		TotalNetProfit: result.Summary.TotalNetProfit,
	}

	// Determine pass/fail status based on comparison with aggregate results
	// Pass if:
	// 1. Expected ATR is positive, AND
	// 2. Win rate is within 1 standard deviation of aggregate (or better), AND
	// 3. Expected ATR is at least 50% of the aggregate expected ATR
	holdout.Status = "FAIL"

	if holdout.ExpectedATR > 0 {
		// Check win rate consistency
		winRateLowerBound := agg.WinRate - 2*agg.WinRateStdDev
		if holdout.WinRate >= winRateLowerBound {
			// Check expected ATR consistency (at least 50% of aggregate)
			if agg.ExpectedATR <= 0 || holdout.ExpectedATR >= agg.ExpectedATR*0.5 {
				holdout.Status = "PASS"
			}
		}
	}

	return holdout
}

// calculateFinalVerdict generates the final assessment.
func (e *WalkForwardEngine) calculateFinalVerdict(agg AggregateTestResults, holdout *HoldoutResults) FinalVerdict {
	verdict := FinalVerdict{
		Profitable: agg.ExpectedATR > 0,
		Consistent: agg.Consistency >= 70.0,
	}

	if holdout != nil {
		verdict.HoldoutPassed = holdout.Status == "PASS"
	} else {
		// If no holdout configured, consider it passed by default
		verdict.HoldoutPassed = true
	}

	// Generate recommendation
	if verdict.Profitable && verdict.Consistent && verdict.HoldoutPassed {
		verdict.Recommendation = "VALIDATED - Ready for live trading"
	} else if verdict.Profitable && verdict.Consistent && !verdict.HoldoutPassed {
		verdict.Recommendation = "CAUTION - Profitable and consistent but failed holdout validation; may be overfit"
	} else if verdict.Profitable && !verdict.Consistent {
		verdict.Recommendation = "INCONSISTENT - Profitable overall but inconsistent across periods; needs refinement"
	} else if !verdict.Profitable && verdict.Consistent {
		verdict.Recommendation = "UNPROFITABLE - Consistently unprofitable; strategy needs redesign"
	} else {
		verdict.Recommendation = "REJECTED - Neither profitable nor consistent; do not use"
	}

	return verdict
}

// addMonths adds the specified number of months to a time.
// This handles month-end edge cases properly.
func addMonths(t time.Time, months int) time.Time {
	return t.AddDate(0, months, 0)
}

// monthsBetween calculates the number of complete months between two dates.
// This is more accurate than duration-based calculation.
func monthsBetween(from, to time.Time) int {
	years := to.Year() - from.Year()
	months := int(to.Month()) - int(from.Month())
	days := to.Day() - from.Day()

	totalMonths := years*12 + months
	// If the day of 'to' is before the day of 'from', we haven't completed the last month
	if days < 0 {
		totalMonths--
	}
	if totalMonths < 0 {
		return 0
	}
	return totalMonths
}
