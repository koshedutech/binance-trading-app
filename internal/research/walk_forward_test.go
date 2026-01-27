// Package research provides data structures and services for research infrastructure.
// Story 15.8: Walk-Forward Backtest Engine - Unit tests
package research

import (
	"testing"
	"time"
)

func TestWalkForwardConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  WalkForwardConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: WalkForwardConfig{
				TrainMonths:    6,
				TestMonths:     3,
				StepMonths:     3,
				HoldoutPercent: 20,
			},
			wantErr: false,
		},
		{
			name: "invalid train months",
			config: WalkForwardConfig{
				TrainMonths:    0,
				TestMonths:     3,
				StepMonths:     3,
				HoldoutPercent: 20,
			},
			wantErr: true,
		},
		{
			name: "invalid test months",
			config: WalkForwardConfig{
				TrainMonths:    6,
				TestMonths:     0,
				StepMonths:     3,
				HoldoutPercent: 20,
			},
			wantErr: true,
		},
		{
			name: "invalid step months",
			config: WalkForwardConfig{
				TrainMonths:    6,
				TestMonths:     3,
				StepMonths:     0,
				HoldoutPercent: 20,
			},
			wantErr: true,
		},
		{
			name: "holdout too high",
			config: WalkForwardConfig{
				TrainMonths:    6,
				TestMonths:     3,
				StepMonths:     3,
				HoldoutPercent: 60,
			},
			wantErr: true,
		},
		{
			name: "holdout negative",
			config: WalkForwardConfig{
				TrainMonths:    6,
				TestMonths:     3,
				StepMonths:     3,
				HoldoutPercent: -10,
			},
			wantErr: true,
		},
		{
			name: "zero holdout valid",
			config: WalkForwardConfig{
				TrainMonths:    6,
				TestMonths:     3,
				StepMonths:     3,
				HoldoutPercent: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWalkForwardConfig_SetDefaults(t *testing.T) {
	config := WalkForwardConfig{}
	config.SetDefaults()

	if config.TrainMonths != 6 {
		t.Errorf("TrainMonths = %d, want 6", config.TrainMonths)
	}
	if config.TestMonths != 3 {
		t.Errorf("TestMonths = %d, want 3", config.TestMonths)
	}
	if config.StepMonths != 3 {
		t.Errorf("StepMonths = %d, want 3", config.StepMonths)
	}
	if config.HoldoutPercent != 20 {
		t.Errorf("HoldoutPercent = %d, want 20", config.HoldoutPercent)
	}
}

func TestWalkForwardRequest_Validate(t *testing.T) {
	baseReq := WalkForwardRequest{
		Coins:     []string{"BTCUSDT"},
		Timeframe: Timeframe15m,
		From:      time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		To:        time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC),
		EntryConditions: []EntryCondition{
			{Feature: "rsi_14", Operator: OpLT, Value: 30},
		},
		EntryLogic:      LogicAND,
		SLATRMultiplier: 0.5,
		TPATRMultiplier: 2.0,
		MaxHoldCandles:  100,
		WalkForwardConfig: WalkForwardConfig{
			TrainMonths:    6,
			TestMonths:     3,
			StepMonths:     3,
			HoldoutPercent: 20,
		},
	}

	t.Run("valid request", func(t *testing.T) {
		req := baseReq
		if err := req.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("no coins", func(t *testing.T) {
		req := baseReq
		req.Coins = nil
		if err := req.Validate(); err == nil {
			t.Error("Validate() should fail with no coins")
		}
	})

	t.Run("invalid timeframe", func(t *testing.T) {
		req := baseReq
		req.Timeframe = "invalid"
		if err := req.Validate(); err == nil {
			t.Error("Validate() should fail with invalid timeframe")
		}
	})

	t.Run("date range too short", func(t *testing.T) {
		req := baseReq
		req.To = time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC) // Only 2 months
		if err := req.Validate(); err == nil {
			t.Error("Validate() should fail with date range too short")
		}
	})

	t.Run("to before from", func(t *testing.T) {
		req := baseReq
		req.From = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		req.To = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := req.Validate(); err == nil {
			t.Error("Validate() should fail when to is before from")
		}
	})
}

func TestWalkForwardEngine_GeneratePeriods(t *testing.T) {
	engine := &WalkForwardEngine{}

	tests := []struct {
		name           string
		from           time.Time
		to             time.Time
		config         WalkForwardConfig
		expectedPeriods int
	}{
		{
			name: "standard 18 months with 6 train, 3 test, 3 step",
			from: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), // 18 months
			config: WalkForwardConfig{
				TrainMonths: 6,
				TestMonths:  3,
				StepMonths:  3,
			},
			// Rolling windows starting at Jan, Apr, Jul, Oct each fit within 18 months
			expectedPeriods: 4,
		},
		{
			name: "one year with 6 train, 3 test, 3 step",
			from: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), // 12 months
			config: WalkForwardConfig{
				TrainMonths: 6,
				TestMonths:  3,
				StepMonths:  3,
			},
			// Two windows fit: Jan-Jun train/Jul-Sep test, Apr-Sep train/Oct-Dec test
			expectedPeriods: 2,
		},
		{
			name: "two years with 3 train, 1 test, 1 step",
			from: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), // 24 months
			config: WalkForwardConfig{
				TrainMonths: 3,
				TestMonths:  1,
				StepMonths:  1,
			},
			// 24 - 3 (train) = 21 possible test month starts, each 1 month
			expectedPeriods: 21,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			periods := engine.generatePeriods(tt.from, tt.to, tt.config)
			if len(periods) != tt.expectedPeriods {
				t.Errorf("generatePeriods() got %d periods, want %d", len(periods), tt.expectedPeriods)
				for i, p := range periods {
					t.Logf("Period %d: Train %s to %s, Test %s to %s",
						i+1,
						p.Train.From.Format("2006-01-02"),
						p.Train.To.Format("2006-01-02"),
						p.Test.From.Format("2006-01-02"),
						p.Test.To.Format("2006-01-02"))
				}
			}
		})
	}
}

func TestWalkForwardEngine_CalculateHoldoutPeriod(t *testing.T) {
	engine := &WalkForwardEngine{}

	tests := []struct {
		name             string
		from             time.Time
		to               time.Time
		holdoutPercent   int
		expectedDuration time.Duration
	}{
		{
			name:             "20% holdout of 1 year",
			from:             time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			to:               time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			holdoutPercent:   20,
			expectedDuration: 365 * 24 * time.Hour * 20 / 100, // ~73 days
		},
		{
			name:             "0% holdout",
			from:             time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			to:               time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			holdoutPercent:   0,
			expectedDuration: 0,
		},
		{
			name:             "50% holdout",
			from:             time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			to:               time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			holdoutPercent:   50,
			expectedDuration: 365 * 24 * time.Hour / 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &WalkForwardRequest{
				From: tt.from,
				To:   tt.to,
				WalkForwardConfig: WalkForwardConfig{
					HoldoutPercent: tt.holdoutPercent,
				},
			}

			holdoutStart, holdoutEnd := engine.calculateHoldoutPeriod(req)

			if tt.holdoutPercent == 0 {
				if holdoutStart != tt.to {
					t.Errorf("Expected holdoutStart to equal to for 0%% holdout")
				}
				return
			}

			actualDuration := holdoutEnd.Sub(holdoutStart)
			// Allow 1 day tolerance for rounding
			tolerance := 24 * time.Hour
			if actualDuration < tt.expectedDuration-tolerance || actualDuration > tt.expectedDuration+tolerance {
				t.Errorf("Holdout duration = %v, expected ~%v", actualDuration, tt.expectedDuration)
			}
		})
	}
}

func TestWalkForwardEngine_CalculateStdDev(t *testing.T) {
	engine := &WalkForwardEngine{}

	tests := []struct {
		name     string
		values   []float64
		expected float64
		tolerance float64
	}{
		{
			name:      "simple values",
			values:    []float64{2, 4, 4, 4, 5, 5, 7, 9},
			expected:  2.14, // Sample std dev = sqrt(32/7) = 2.138
			tolerance: 0.15,
		},
		{
			name:      "all same",
			values:    []float64{5, 5, 5, 5},
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "two values",
			values:    []float64{10, 20},
			expected:  7.07, // sqrt((5^2 + 5^2) / 1) = sqrt(50) = 7.07
			tolerance: 0.01,
		},
		{
			name:      "single value",
			values:    []float64{5},
			expected:  0.0,
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.calculateStdDev(tt.values)
			if result < tt.expected-tt.tolerance || result > tt.expected+tt.tolerance {
				t.Errorf("calculateStdDev() = %v, expected ~%v", result, tt.expected)
			}
		})
	}
}

func TestWalkForwardEngine_CalculateMaxDrawdown(t *testing.T) {
	engine := &WalkForwardEngine{}

	tests := []struct {
		name     string
		pnls     []float64 // NetPnLPercent for each trade
		expected float64
	}{
		{
			name:     "simple drawdown",
			pnls:     []float64{10, -5, -5, 10}, // Peak 10, drops to 0, back to 10
			expected: 10.0,
		},
		{
			name:     "no drawdown",
			pnls:     []float64{5, 5, 5, 5},
			expected: 0.0,
		},
		{
			name:     "continuous losses",
			pnls:     []float64{-5, -5, -5},
			expected: 15.0,
		},
		{
			name:     "empty",
			pnls:     []float64{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trades := make([]Trade, len(tt.pnls))
			baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
			for i, pnl := range tt.pnls {
				trades[i] = Trade{
					EntryTime:     baseTime.Add(time.Duration(i) * time.Hour),
					NetPnLPercent: pnl,
				}
			}

			result := engine.calculateMaxDrawdown(trades)
			if result != tt.expected {
				t.Errorf("calculateMaxDrawdown() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWalkForwardEngine_CalculateFinalVerdict(t *testing.T) {
	engine := &WalkForwardEngine{}

	tests := []struct {
		name              string
		agg               AggregateTestResults
		holdout           *HoldoutResults
		wantProfitable    bool
		wantConsistent    bool
		wantHoldoutPassed bool
		wantRecommendation string
	}{
		{
			name: "all pass",
			agg: AggregateTestResults{
				ExpectedATR: 0.05,
				Consistency: 80.0,
			},
			holdout: &HoldoutResults{
				Status: "PASS",
			},
			wantProfitable:     true,
			wantConsistent:     true,
			wantHoldoutPassed:  true,
			wantRecommendation: "VALIDATED - Ready for live trading",
		},
		{
			name: "profitable and consistent but holdout failed",
			agg: AggregateTestResults{
				ExpectedATR: 0.05,
				Consistency: 80.0,
			},
			holdout: &HoldoutResults{
				Status: "FAIL",
			},
			wantProfitable:     true,
			wantConsistent:     true,
			wantHoldoutPassed:  false,
			wantRecommendation: "CAUTION - Profitable and consistent but failed holdout validation; may be overfit",
		},
		{
			name: "profitable but inconsistent",
			agg: AggregateTestResults{
				ExpectedATR: 0.05,
				Consistency: 50.0,
			},
			holdout: &HoldoutResults{
				Status: "PASS",
			},
			wantProfitable:     true,
			wantConsistent:     false,
			wantHoldoutPassed:  true,
			wantRecommendation: "INCONSISTENT - Profitable overall but inconsistent across periods; needs refinement",
		},
		{
			name: "unprofitable but consistent",
			agg: AggregateTestResults{
				ExpectedATR: -0.05,
				Consistency: 80.0,
			},
			holdout: nil,
			wantProfitable:     false,
			wantConsistent:     true,
			wantHoldoutPassed:  true, // No holdout configured
			wantRecommendation: "UNPROFITABLE - Consistently unprofitable; strategy needs redesign",
		},
		{
			name: "neither profitable nor consistent",
			agg: AggregateTestResults{
				ExpectedATR: -0.05,
				Consistency: 40.0,
			},
			holdout: nil,
			wantProfitable:     false,
			wantConsistent:     false,
			wantHoldoutPassed:  true,
			wantRecommendation: "REJECTED - Neither profitable nor consistent; do not use",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdict := engine.calculateFinalVerdict(tt.agg, tt.holdout)

			if verdict.Profitable != tt.wantProfitable {
				t.Errorf("Profitable = %v, want %v", verdict.Profitable, tt.wantProfitable)
			}
			if verdict.Consistent != tt.wantConsistent {
				t.Errorf("Consistent = %v, want %v", verdict.Consistent, tt.wantConsistent)
			}
			if verdict.HoldoutPassed != tt.wantHoldoutPassed {
				t.Errorf("HoldoutPassed = %v, want %v", verdict.HoldoutPassed, tt.wantHoldoutPassed)
			}
			if verdict.Recommendation != tt.wantRecommendation {
				t.Errorf("Recommendation = %q, want %q", verdict.Recommendation, tt.wantRecommendation)
			}
		})
	}
}

func TestAddMonths(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		months   int
		expected time.Time
	}{
		{
			name:     "add 3 months",
			input:    time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			months:   3,
			expected: time.Date(2023, 4, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "add 6 months crossing year",
			input:    time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC),
			months:   6,
			expected: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "month end handling",
			input:    time.Date(2023, 1, 31, 0, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2023, 3, 3, 0, 0, 0, 0, time.UTC), // Feb 31 normalizes to Mar 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addMonths(tt.input, tt.months)
			if !result.Equal(tt.expected) {
				t.Errorf("addMonths(%v, %d) = %v, want %v", tt.input, tt.months, result, tt.expected)
			}
		})
	}
}

func TestMonthsBetween(t *testing.T) {
	tests := []struct {
		name     string
		from     time.Time
		to       time.Time
		expected int
	}{
		{
			name:     "exact 12 months",
			from:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: 12,
		},
		{
			name:     "11 months and some days",
			from:     time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: 11, // Not complete 12 months
		},
		{
			name:     "exact 6 months",
			from:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC),
			expected: 6,
		},
		{
			name:     "same month",
			from:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: 0,
		},
		{
			name:     "reverse order returns 0",
			from:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			to:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := monthsBetween(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("monthsBetween(%v, %v) = %d, want %d", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestWalkForwardEngine_CalculateAggregateResults_ZeroTradePeriods(t *testing.T) {
	engine := &WalkForwardEngine{}

	// Test with mix of periods with and without trades
	periods := []PeriodResult{
		{
			TestResults: PeriodMetrics{
				Trades:      10,
				Wins:        6,
				Losses:      4,
				WinRate:     60.0,
				ExpectedATR: 0.05,
			},
		},
		{
			TestResults: PeriodMetrics{
				Trades:      0, // No trades in this period
				Wins:        0,
				Losses:      0,
				WinRate:     0,
				ExpectedATR: 0,
			},
		},
		{
			TestResults: PeriodMetrics{
				Trades:      8,
				Wins:        5,
				Losses:      3,
				WinRate:     62.5,
				ExpectedATR: 0.03,
			},
		},
	}

	// Create some trades for the aggregate
	trades := []Trade{
		{EntryTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), PnLATR: 0.05, NetPnLPercent: 2.0},
		{EntryTime: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), PnLATR: -0.02, NetPnLPercent: -1.0},
	}

	result := engine.calculateAggregateResults(periods, trades)

	// Total periods should be 3
	if result.TotalPeriods != 3 {
		t.Errorf("TotalPeriods = %d, want 3", result.TotalPeriods)
	}

	// Consistency should be calculated based on periods WITH trades (2 profitable / 2 with trades = 100%)
	// Both periods with trades have positive ExpectedATR
	if result.Consistency != 100.0 {
		t.Errorf("Consistency = %v, want 100.0 (only periods with trades count)", result.Consistency)
	}

	// PeriodsWithProfit should be 2 (the two periods that had trades and positive expected ATR)
	if result.PeriodsWithProfit != 2 {
		t.Errorf("PeriodsWithProfit = %d, want 2", result.PeriodsWithProfit)
	}

	// Worst/Best win rate should not be affected by zero-trade periods
	if result.WorstPeriodWinRate != 60.0 {
		t.Errorf("WorstPeriodWinRate = %v, want 60.0", result.WorstPeriodWinRate)
	}
	if result.BestPeriodWinRate != 62.5 {
		t.Errorf("BestPeriodWinRate = %v, want 62.5", result.BestPeriodWinRate)
	}
}

func TestWalkForwardEngine_CalculateAggregateResults_AllZeroTradePeriods(t *testing.T) {
	engine := &WalkForwardEngine{}

	// All periods have zero trades
	periods := []PeriodResult{
		{TestResults: PeriodMetrics{Trades: 0}},
		{TestResults: PeriodMetrics{Trades: 0}},
	}

	result := engine.calculateAggregateResults(periods, []Trade{})

	// Consistency should be 0 when no periods have trades
	if result.Consistency != 0 {
		t.Errorf("Consistency = %v, want 0 for all zero-trade periods", result.Consistency)
	}

	// Best/Worst should be 0, not uninitialized sentinel values
	if result.WorstPeriodWinRate != 0 {
		t.Errorf("WorstPeriodWinRate = %v, want 0", result.WorstPeriodWinRate)
	}
	if result.BestPeriodWinRate != 0 {
		t.Errorf("BestPeriodWinRate = %v, want 0", result.BestPeriodWinRate)
	}
	if result.WorstPeriodExpATR != 0 {
		t.Errorf("WorstPeriodExpATR = %v, want 0", result.WorstPeriodExpATR)
	}
	if result.BestPeriodExpATR != 0 {
		t.Errorf("BestPeriodExpATR = %v, want 0", result.BestPeriodExpATR)
	}
}

func TestWalkForwardRequest_ToBacktestRequest(t *testing.T) {
	wfReq := WalkForwardRequest{
		Coins:     []string{"BTCUSDT", "ETHUSDT"},
		Timeframe: Timeframe15m,
		EntryConditions: []EntryCondition{
			{Feature: "rsi_14", Operator: OpLT, Value: 30},
		},
		EntryLogic:      LogicAND,
		SLATRMultiplier: 0.5,
		TPATRMultiplier: 2.0,
		ATRPeriod:       14,
		MaxHoldCandles:  100,
		IncludeFees:     true,
		FeePercent:      0.04,
	}

	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 3, 31, 0, 0, 0, 0, time.UTC)

	btReq := wfReq.ToBacktestRequest(from, to)

	if len(btReq.Coins) != 2 {
		t.Errorf("Coins count = %d, want 2", len(btReq.Coins))
	}
	if btReq.Timeframe != Timeframe15m {
		t.Errorf("Timeframe = %s, want 15m", btReq.Timeframe)
	}
	if !btReq.From.Equal(from) {
		t.Errorf("From = %v, want %v", btReq.From, from)
	}
	if !btReq.To.Equal(to) {
		t.Errorf("To = %v, want %v", btReq.To, to)
	}
	if btReq.SLATRMultiplier != 0.5 {
		t.Errorf("SLATRMultiplier = %v, want 0.5", btReq.SLATRMultiplier)
	}
	if btReq.TPATRMultiplier != 2.0 {
		t.Errorf("TPATRMultiplier = %v, want 2.0", btReq.TPATRMultiplier)
	}
	if btReq.MaxHoldCandles != 100 {
		t.Errorf("MaxHoldCandles = %d, want 100", btReq.MaxHoldCandles)
	}
	if !btReq.IncludeFees {
		t.Error("IncludeFees should be true")
	}
}
