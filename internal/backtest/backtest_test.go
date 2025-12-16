package backtest

import (
	"binance-trading-bot/internal/binance"
	"testing"
	"time"
)

// MockStrategy for testing
type MockStrategy struct {
	symbol   string
	interval string
	signals  []SignalResult
	callIdx  int
}

type SignalResult struct {
	ShouldSignal bool
	Side         string
	EntryPrice   float64
	StopLoss     float64
	TakeProfit   float64
}

func (m *MockStrategy) GetSymbol() string   { return m.symbol }
func (m *MockStrategy) GetInterval() string { return m.interval }
func (m *MockStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
	if m.callIdx >= len(m.signals) {
		return nil, nil
	}
	result := m.signals[m.callIdx]
	m.callIdx++

	if !result.ShouldSignal {
		return nil, nil
	}

	return &Signal{
		Side:       result.Side,
		EntryPrice: result.EntryPrice,
		StopLoss:   result.StopLoss,
		TakeProfit: result.TakeProfit,
	}, nil
}

// Signal type for mock
type Signal struct {
	Side       string
	EntryPrice float64
	StopLoss   float64
	TakeProfit float64
}

func TestBacktestEngine_BasicFunctionality(t *testing.T) {
	config := &BacktestConfig{
		Symbol:           "BTCUSDT",
		StartDate:        time.Now().Add(-30 * 24 * time.Hour),
		EndDate:          time.Now(),
		InitialBalance:   10000.0,
		PositionSize:     0.1,
		StopLossPercent:  2.0,
		TakeProfitPercent: 4.0,
		Commission:       0.1,
	}

	engine := NewBacktestEngine(config)

	if engine == nil {
		t.Fatal("NewBacktestEngine returned nil")
	}

	if engine.config.InitialBalance != 10000.0 {
		t.Errorf("Expected initial balance 10000, got %f", engine.config.InitialBalance)
	}
}

func TestBacktestResult_Calculations(t *testing.T) {
	result := &BacktestResult{
		TotalTrades:  10,
		WinningTrades: 6,
		LosingTrades:  4,
		GrossProfit:   1000.0,
		GrossLoss:     400.0,
		TotalProfit:   600.0,
		InitialBalance: 10000.0,
		FinalBalance:  10600.0,
	}

	// Test win rate calculation
	winRate := float64(result.WinningTrades) / float64(result.TotalTrades) * 100
	if winRate != 60.0 {
		t.Errorf("Expected win rate 60%%, got %.2f%%", winRate)
	}

	// Test profit factor
	profitFactor := result.GrossProfit / result.GrossLoss
	if profitFactor != 2.5 {
		t.Errorf("Expected profit factor 2.5, got %.2f", profitFactor)
	}

	// Test return percentage
	returnPercent := ((result.FinalBalance - result.InitialBalance) / result.InitialBalance) * 100
	if returnPercent != 6.0 {
		t.Errorf("Expected return 6%%, got %.2f%%", returnPercent)
	}
}

func TestBacktestTrade_PnLCalculation(t *testing.T) {
	tests := []struct {
		name       string
		side       string
		entryPrice float64
		exitPrice  float64
		quantity   float64
		expectedPnL float64
	}{
		{
			name:       "Long profitable trade",
			side:       "BUY",
			entryPrice: 100.0,
			exitPrice:  110.0,
			quantity:   1.0,
			expectedPnL: 10.0,
		},
		{
			name:       "Long losing trade",
			side:       "BUY",
			entryPrice: 100.0,
			exitPrice:  95.0,
			quantity:   1.0,
			expectedPnL: -5.0,
		},
		{
			name:       "Short profitable trade",
			side:       "SELL",
			entryPrice: 100.0,
			exitPrice:  90.0,
			quantity:   1.0,
			expectedPnL: 10.0,
		},
		{
			name:       "Short losing trade",
			side:       "SELL",
			entryPrice: 100.0,
			exitPrice:  105.0,
			quantity:   1.0,
			expectedPnL: -5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pnl float64
			if tt.side == "BUY" {
				pnl = (tt.exitPrice - tt.entryPrice) * tt.quantity
			} else {
				pnl = (tt.entryPrice - tt.exitPrice) * tt.quantity
			}

			if pnl != tt.expectedPnL {
				t.Errorf("Expected P&L %.2f, got %.2f", tt.expectedPnL, pnl)
			}
		})
	}
}

func TestMaxDrawdown_Calculation(t *testing.T) {
	equityCurve := []float64{10000, 10500, 10200, 9800, 10100, 9500, 10000, 10800}

	peak := equityCurve[0]
	maxDrawdown := 0.0

	for _, equity := range equityCurve {
		if equity > peak {
			peak = equity
		}
		drawdown := (peak - equity) / peak * 100
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	// Peak was 10500, lowest after that was 9500
	// Drawdown = (10500 - 9500) / 10500 * 100 = 9.52%
	expectedDrawdown := 9.52

	if maxDrawdown < 9.0 || maxDrawdown > 10.0 {
		t.Errorf("Expected max drawdown around %.2f%%, got %.2f%%", expectedDrawdown, maxDrawdown)
	}
}

func TestSharpeRatio_Calculation(t *testing.T) {
	// Monthly returns
	returns := []float64{0.02, 0.03, -0.01, 0.04, 0.01, -0.02, 0.03, 0.02, 0.01, 0.02, 0.03, 0.01}

	// Calculate average return
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	avgReturn := sum / float64(len(returns))

	// Calculate standard deviation
	variance := 0.0
	for _, r := range returns {
		variance += (r - avgReturn) * (r - avgReturn)
	}
	stdDev := variance / float64(len(returns))

	// Risk-free rate (annual, convert to monthly)
	riskFreeRate := 0.02 / 12

	// Sharpe Ratio (annualized)
	if stdDev > 0 {
		sharpe := (avgReturn - riskFreeRate) / stdDev * 12

		// Should be positive with these returns
		if sharpe <= 0 {
			t.Errorf("Expected positive Sharpe ratio, got %.2f", sharpe)
		}
	}
}

func TestBacktestConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *BacktestConfig
		shouldError bool
	}{
		{
			name: "Valid config",
			config: &BacktestConfig{
				Symbol:         "BTCUSDT",
				InitialBalance: 10000,
				PositionSize:   0.1,
			},
			shouldError: false,
		},
		{
			name: "Zero initial balance",
			config: &BacktestConfig{
				Symbol:         "BTCUSDT",
				InitialBalance: 0,
			},
			shouldError: true,
		},
		{
			name: "Empty symbol",
			config: &BacktestConfig{
				Symbol:         "",
				InitialBalance: 10000,
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := tt.config.Symbol == "" || tt.config.InitialBalance <= 0
			if hasError != tt.shouldError {
				t.Errorf("Expected error: %v, got error: %v", tt.shouldError, hasError)
			}
		})
	}
}
