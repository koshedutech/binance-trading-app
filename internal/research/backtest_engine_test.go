// Package research provides data structures and services for research infrastructure.
// Story 15.7: Simple Backtest Engine - Tests
package research

import (
	"context"
	"math"
	"testing"
	"time"

	"binance-trading-bot/internal/research/indicators"
)

// mockCandleRepository is a mock implementation of CandleRepository for testing
type mockCandleRepository struct {
	candles []*ResearchCandle
	err     error
}

func (m *mockCandleRepository) GetCandles(ctx context.Context, symbol string, timeframe Timeframe, startTime, endTime time.Time) ([]*ResearchCandle, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.candles, nil
}

// generateTestCandles generates mock candles for testing
func generateTestCandles(symbol string, timeframe Timeframe, count int, startTime time.Time, basePrice float64) []*ResearchCandle {
	candles := make([]*ResearchCandle, count)
	interval := timeframe.Duration()

	for i := 0; i < count; i++ {
		openTime := startTime.Add(time.Duration(i) * interval)
		// Simple price movement: oscillate around base price
		variation := float64(i%10-5) * 0.001 * basePrice
		open := basePrice + variation
		close := open + float64((i%3-1))*0.005*basePrice
		high := math.Max(open, close) + 0.003*basePrice
		low := math.Min(open, close) - 0.003*basePrice

		candles[i] = &ResearchCandle{
			ID:             int64(i + 1),
			Symbol:         symbol,
			Timeframe:      timeframe,
			OpenTime:       openTime,
			CloseTime:      openTime.Add(interval),
			Open:           open,
			High:           high,
			Low:            low,
			Close:          close,
			Volume:         1000000.0,
			QuoteVolume:    1000000.0 * close,
			TradeCount:     5000,
			TakerBuyVolume: 500000.0,
			TakerBuyQuote:  500000.0 * close,
			CreatedAt:      time.Now(),
		}
	}

	return candles
}

func TestOperator_IsValid(t *testing.T) {
	tests := []struct {
		op    Operator
		valid bool
	}{
		{OpLT, true},
		{OpLTE, true},
		{OpGT, true},
		{OpGTE, true},
		{OpEQ, true},
		{OpNEQ, true},
		{Operator("invalid"), false},
		{Operator(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			if got := tt.op.IsValid(); got != tt.valid {
				t.Errorf("Operator(%s).IsValid() = %v, want %v", tt.op, got, tt.valid)
			}
		})
	}
}

func TestParseOperator(t *testing.T) {
	tests := []struct {
		input string
		want  Operator
		err   bool
	}{
		{"<", OpLT, false},
		{"<=", OpLTE, false},
		{">", OpGT, false},
		{">=", OpGTE, false},
		{"==", OpEQ, false},
		{"!=", OpNEQ, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseOperator(tt.input)
			if (err != nil) != tt.err {
				t.Errorf("ParseOperator(%s) error = %v, wantErr %v", tt.input, err, tt.err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseOperator(%s) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestEntryCondition_Evaluate(t *testing.T) {
	tests := []struct {
		name    string
		cond    EntryCondition
		value   float64
		want    bool
	}{
		{"LT true", EntryCondition{Operator: OpLT, Value: 0}, -1.0, true},
		{"LT false", EntryCondition{Operator: OpLT, Value: 0}, 1.0, false},
		{"LTE equal", EntryCondition{Operator: OpLTE, Value: 0}, 0, true},
		{"GT true", EntryCondition{Operator: OpGT, Value: 0}, 1.0, true},
		{"GTE equal", EntryCondition{Operator: OpGTE, Value: 0}, 0, true},
		{"EQ true", EntryCondition{Operator: OpEQ, Value: 5}, 5, true},
		{"EQ false", EntryCondition{Operator: OpEQ, Value: 5}, 4, false},
		{"NEQ true", EntryCondition{Operator: OpNEQ, Value: 5}, 4, true},
		{"NEQ false", EntryCondition{Operator: OpNEQ, Value: 5}, 5, false},
		{"NaN value", EntryCondition{Operator: OpLT, Value: 0}, math.NaN(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cond.Evaluate(tt.value); got != tt.want {
				t.Errorf("EntryCondition.Evaluate(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestBacktestRequest_Validate(t *testing.T) {
	now := time.Now()
	validRequest := BacktestRequest{
		Coins:           []string{"BTCUSDT"},
		Timeframe:       Timeframe15m,
		From:            now.Add(-24 * time.Hour),
		To:              now,
		EntryConditions: []EntryCondition{{Feature: "return_5c", Operator: OpLT, Value: -2.0}},
		EntryLogic:      LogicAND,
		SLATRMultiplier: 0.5,
		TPATRMultiplier: 2.0,
		MaxHoldCandles:  50,
	}

	tests := []struct {
		name    string
		modify  func(r *BacktestRequest)
		wantErr bool
	}{
		{"valid", func(r *BacktestRequest) {}, false},
		{"no coins", func(r *BacktestRequest) { r.Coins = nil }, true},
		{"invalid timeframe", func(r *BacktestRequest) { r.Timeframe = "invalid" }, true},
		{"no from", func(r *BacktestRequest) { r.From = time.Time{} }, true},
		{"no to", func(r *BacktestRequest) { r.To = time.Time{} }, true},
		{"to before from", func(r *BacktestRequest) { r.To = r.From.Add(-time.Hour) }, true},
		{"no conditions", func(r *BacktestRequest) { r.EntryConditions = nil }, true},
		{"invalid logic", func(r *BacktestRequest) { r.EntryLogic = "INVALID" }, true},
		{"zero SL", func(r *BacktestRequest) { r.SLATRMultiplier = 0 }, true},
		{"zero TP", func(r *BacktestRequest) { r.TPATRMultiplier = 0 }, true},
		{"zero max hold", func(r *BacktestRequest) { r.MaxHoldCandles = 0 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validRequest // Copy
			tt.modify(&req)
			err := req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("BacktestRequest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBacktestRequest_SetDefaults(t *testing.T) {
	req := BacktestRequest{
		IncludeFees: true,
	}

	req.SetDefaults()

	if req.ATRPeriod != 14 {
		t.Errorf("ATRPeriod = %d, want 14", req.ATRPeriod)
	}
	if req.EntryLogic != LogicAND {
		t.Errorf("EntryLogic = %s, want AND", req.EntryLogic)
	}
	if req.FeePercent != 0.04 {
		t.Errorf("FeePercent = %f, want 0.04", req.FeePercent)
	}
}

func TestConditionEvaluator_Evaluate(t *testing.T) {
	// Create test features
	features := &CandleFeatures{
		Price: &indicators.PriceFeatures{
			Return5c:        -2.5,
			PositionInRange: 0.25,
		},
		Momentum: &indicators.MomentumFeatures{
			RSI14: 35,
		},
	}

	tests := []struct {
		name       string
		conditions []EntryCondition
		logic      EntryLogic
		want       bool
	}{
		{
			name: "AND all true",
			conditions: []EntryCondition{
				{Feature: "return_5c", Operator: OpLT, Value: -2.0},
				{Feature: "position_in_range", Operator: OpLT, Value: 0.3},
			},
			logic: LogicAND,
			want:  true,
		},
		{
			name: "AND one false",
			conditions: []EntryCondition{
				{Feature: "return_5c", Operator: OpLT, Value: -2.0},
				{Feature: "position_in_range", Operator: OpLT, Value: 0.2}, // 0.25 is not < 0.2
			},
			logic: LogicAND,
			want:  false,
		},
		{
			name: "OR one true",
			conditions: []EntryCondition{
				{Feature: "return_5c", Operator: OpGT, Value: 0},  // false
				{Feature: "rsi_14", Operator: OpLT, Value: 40},     // true
			},
			logic: LogicOR,
			want:  true,
		},
		{
			name: "OR none true",
			conditions: []EntryCondition{
				{Feature: "return_5c", Operator: OpGT, Value: 0},  // false
				{Feature: "rsi_14", Operator: OpGT, Value: 50},     // false
			},
			logic: LogicOR,
			want:  false,
		},
		{
			name: "missing feature",
			conditions: []EntryCondition{
				{Feature: "nonexistent_feature", Operator: OpLT, Value: 0},
			},
			logic: LogicAND,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewConditionEvaluator(tt.conditions, tt.logic)
			if got := evaluator.Evaluate(features); got != tt.want {
				t.Errorf("ConditionEvaluator.Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConditionEvaluator_NilFeatures(t *testing.T) {
	evaluator := NewConditionEvaluator(
		[]EntryCondition{{Feature: "return_5c", Operator: OpLT, Value: -2.0}},
		LogicAND,
	)

	if evaluator.Evaluate(nil) {
		t.Error("Expected false for nil features")
	}
}

func TestGetAvailableFeatures(t *testing.T) {
	features := GetAvailableFeatures()

	// Check that we have a reasonable number of features
	if len(features) < 50 {
		t.Errorf("Expected at least 50 features, got %d", len(features))
	}

	// Check some expected features exist
	expected := []string{"return_5c", "rsi_14", "atr_14", "volume_ratio_5", "sma_20"}
	for _, exp := range expected {
		found := false
		for _, f := range features {
			if f == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected feature %s not found in available features", exp)
		}
	}
}

func TestValidateFeatureName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"return_5c", true},
		{"rsi_14", true},
		{"invalid_feature", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateFeatureName(tt.name); got != tt.valid {
				t.Errorf("ValidateFeatureName(%s) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestTradeOutcome(t *testing.T) {
	// Just verify the constants are as expected
	if OutcomeWin != "win" {
		t.Errorf("OutcomeWin = %s, want win", OutcomeWin)
	}
	if OutcomeLoss != "loss" {
		t.Errorf("OutcomeLoss = %s, want loss", OutcomeLoss)
	}
	if OutcomeTimeout != "timeout" {
		t.Errorf("OutcomeTimeout = %s, want timeout", OutcomeTimeout)
	}
}

func TestAlignToInterval(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		interval time.Duration
		want     time.Time
	}{
		{
			name:     "15m alignment",
			input:    time.Date(2024, 1, 1, 12, 17, 30, 0, time.UTC),
			interval: 15 * time.Minute,
			want:     time.Date(2024, 1, 1, 12, 15, 0, 0, time.UTC),
		},
		{
			name:     "1h alignment",
			input:    time.Date(2024, 1, 1, 12, 45, 30, 0, time.UTC),
			interval: time.Hour,
			want:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "already aligned",
			input:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			interval: time.Hour,
			want:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alignToInterval(tt.input, tt.interval)
			if !got.Equal(tt.want) {
				t.Errorf("alignToInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBacktestSummary_Defaults(t *testing.T) {
	summary := BacktestSummary{}

	// Verify defaults are zero
	if summary.TotalTrades != 0 {
		t.Errorf("Expected TotalTrades=0, got %d", summary.TotalTrades)
	}
	if summary.WinRate != 0 {
		t.Errorf("Expected WinRate=0, got %f", summary.WinRate)
	}
}

func TestCoinBreakdown_Defaults(t *testing.T) {
	breakdown := CoinBreakdown{}

	if breakdown.Symbol != "" {
		t.Errorf("Expected empty Symbol, got %s", breakdown.Symbol)
	}
	if breakdown.TotalTrades != 0 {
		t.Errorf("Expected TotalTrades=0, got %d", breakdown.TotalTrades)
	}
}

func TestValidationResult_Defaults(t *testing.T) {
	validation := ValidationResult{}

	if validation.Significant != false {
		t.Error("Expected Significant=false by default")
	}
	if validation.SampleSizeAdequate != false {
		t.Error("Expected SampleSizeAdequate=false by default")
	}
}

func TestEntryLogic_IsValid(t *testing.T) {
	tests := []struct {
		logic EntryLogic
		valid bool
	}{
		{LogicAND, true},
		{LogicOR, true},
		{EntryLogic("INVALID"), false},
		{EntryLogic(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.logic), func(t *testing.T) {
			if got := tt.logic.IsValid(); got != tt.valid {
				t.Errorf("EntryLogic(%s).IsValid() = %v, want %v", tt.logic, got, tt.valid)
			}
		})
	}
}

func TestBacktestEngine_EmptyTrades(t *testing.T) {
	// Test calculateSummary with empty trades
	engine := &BacktestEngine{}
	summary := engine.calculateSummary([]Trade{}, Timeframe15m)

	if summary.TotalTrades != 0 {
		t.Errorf("Expected TotalTrades=0 for empty trades, got %d", summary.TotalTrades)
	}
	if summary.WinRate != 0 {
		t.Errorf("Expected WinRate=0 for empty trades, got %f", summary.WinRate)
	}
	if summary.SharpeRatio != 0 {
		t.Errorf("Expected SharpeRatio=0 for empty trades, got %f", summary.SharpeRatio)
	}
}

func TestBacktestEngine_MaxDrawdown(t *testing.T) {
	engine := &BacktestEngine{}

	// Test with trades that have a drawdown
	trades := []Trade{
		{NetPnLPercent: 2.0},  // Peak at 2
		{NetPnLPercent: -1.0}, // Cumulative: 1
		{NetPnLPercent: -2.0}, // Cumulative: -1 (drawdown from peak: 3)
		{NetPnLPercent: 1.0},  // Cumulative: 0
	}

	drawdown := engine.calculateMaxDrawdown(trades)
	// Max drawdown should be 3 (from peak of 2 to trough of -1)
	if drawdown != 3.0 {
		t.Errorf("Expected max drawdown of 3.0, got %f", drawdown)
	}
}

func TestBacktestEngine_SharpeRatioTimeframeAdjustment(t *testing.T) {
	engine := &BacktestEngine{}

	// Create trades with known mean and stddev
	trades := []Trade{
		{NetPnLPercent: 1.0},
		{NetPnLPercent: 2.0},
		{NetPnLPercent: 3.0},
	}

	// Test with different timeframes - Sharpe should be different
	summary15m := engine.calculateSummary(trades, Timeframe15m)
	summary1h := engine.calculateSummary(trades, Timeframe1h)

	// 15m has more periods per year than 1h, so Sharpe should be higher
	if summary15m.SharpeRatio <= summary1h.SharpeRatio {
		t.Errorf("Expected 15m Sharpe (%f) > 1h Sharpe (%f)", summary15m.SharpeRatio, summary1h.SharpeRatio)
	}
}

func TestBacktestEngine_ProfitFactorEdgeCases(t *testing.T) {
	engine := &BacktestEngine{}

	// All winners - should get large finite value, not Inf
	allWinners := []Trade{
		{PnLPercent: 1.0, NetPnLPercent: 1.0},
		{PnLPercent: 2.0, NetPnLPercent: 2.0},
	}
	summary := engine.calculateSummary(allWinners, Timeframe15m)
	if math.IsInf(summary.ProfitFactor, 0) {
		t.Error("ProfitFactor should not be Inf for JSON serialization")
	}
	if summary.ProfitFactor < 999999.0 {
		t.Errorf("Expected ProfitFactor >= 999999.0 for all winners, got %f", summary.ProfitFactor)
	}
}
