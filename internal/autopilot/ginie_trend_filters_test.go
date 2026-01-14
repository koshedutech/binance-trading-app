package autopilot

import (
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"binance-trading-bot/internal/binance"
)

// ==================== MOCK FUTURES CLIENT ====================

// mockFuturesClient is a minimal mock for testing trend filter validation
// It implements the binance.FuturesClient interface
type mockFuturesClient struct {
	klines       map[string][]binance.Kline // key: symbol:interval
	klinesErr    error
	callCount    int
	callsMu      sync.Mutex
	callsHistory []string // track which symbol:interval were called
}

func newMockFuturesClient() *mockFuturesClient {
	return &mockFuturesClient{
		klines:       make(map[string][]binance.Kline),
		callsHistory: make([]string, 0),
	}
}

func (m *mockFuturesClient) GetFuturesKlines(symbol, interval string, limit int) ([]binance.Kline, error) {
	m.callsMu.Lock()
	m.callCount++
	m.callsHistory = append(m.callsHistory, symbol+":"+interval)
	m.callsMu.Unlock()

	if m.klinesErr != nil {
		return nil, m.klinesErr
	}

	key := symbol + ":" + interval
	if klines, ok := m.klines[key]; ok {
		return klines, nil
	}

	// Return empty if not configured
	return []binance.Kline{}, nil
}

// ==================== Account Methods ====================
func (m *mockFuturesClient) GetFuturesAccountInfo() (*binance.FuturesAccountInfo, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetPositions() ([]binance.FuturesPosition, error) { return nil, nil }
func (m *mockFuturesClient) GetPositionBySymbol(symbol string) (*binance.FuturesPosition, error) {
	return nil, nil
}

// ==================== Leverage & Margin ====================
func (m *mockFuturesClient) SetLeverage(symbol string, leverage int) (*binance.LeverageResponse, error) {
	return nil, nil
}
func (m *mockFuturesClient) SetMarginType(symbol string, marginType binance.MarginType) error {
	return nil
}
func (m *mockFuturesClient) SetPositionMode(dualSidePosition bool) error { return nil }
func (m *mockFuturesClient) GetPositionMode() (*binance.PositionModeResponse, error) {
	return nil, nil
}

// ==================== Trading ====================
func (m *mockFuturesClient) PlaceFuturesOrder(params binance.FuturesOrderParams) (*binance.FuturesOrderResponse, error) {
	return nil, nil
}
func (m *mockFuturesClient) CancelFuturesOrder(symbol string, orderId int64) error { return nil }
func (m *mockFuturesClient) CancelAllFuturesOrders(symbol string) error            { return nil }
func (m *mockFuturesClient) GetOpenOrders(symbol string) ([]binance.FuturesOrder, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetOrder(symbol string, orderId int64) (*binance.FuturesOrder, error) {
	return nil, nil
}

// ==================== Algo Orders ====================
func (m *mockFuturesClient) PlaceAlgoOrder(params binance.AlgoOrderParams) (*binance.AlgoOrderResponse, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetOpenAlgoOrders(symbol string) ([]binance.AlgoOrder, error) {
	return nil, nil
}
func (m *mockFuturesClient) CancelAlgoOrder(symbol string, algoId int64) error { return nil }
func (m *mockFuturesClient) CancelAllAlgoOrders(symbol string) error           { return nil }
func (m *mockFuturesClient) GetAllAlgoOrders(symbol string, limit int) ([]binance.AlgoOrder, error) {
	return nil, nil
}

// ==================== Market Data ====================
func (m *mockFuturesClient) GetFundingRate(symbol string) (*binance.FundingRate, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetFundingRateHistory(symbol string, limit int) ([]binance.FundingRate, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetMarkPrice(symbol string) (*binance.MarkPrice, error) { return nil, nil }
func (m *mockFuturesClient) GetAllMarkPrices() ([]binance.MarkPrice, error)         { return nil, nil }
func (m *mockFuturesClient) GetOrderBookDepth(symbol string, limit int) (*binance.OrderBookDepth, error) {
	return nil, nil
}
func (m *mockFuturesClient) Get24hrTicker(symbol string) (*binance.Futures24hrTicker, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetAll24hrTickers() ([]binance.Futures24hrTicker, error) { return nil, nil }
func (m *mockFuturesClient) GetFuturesCurrentPrice(symbol string) (float64, error)   { return 0, nil }

// ==================== Exchange Info ====================
func (m *mockFuturesClient) GetFuturesExchangeInfo() (*binance.FuturesExchangeInfo, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetFuturesSymbols() ([]string, error) { return nil, nil }

// ==================== History ====================
func (m *mockFuturesClient) GetTradeHistory(symbol string, limit int) ([]binance.FuturesTrade, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetFundingFeeHistory(symbol string, limit int) ([]binance.FundingFeeRecord, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetAllOrders(symbol string, limit int) ([]binance.FuturesOrder, error) {
	return nil, nil
}
func (m *mockFuturesClient) GetIncomeHistory(incomeType string, startTime, endTime int64, limit int) ([]binance.IncomeRecord, error) {
	return nil, nil
}

// ==================== WebSocket ====================
func (m *mockFuturesClient) GetListenKey() (string, error)             { return "", nil }
func (m *mockFuturesClient) KeepAliveListenKey(listenKey string) error { return nil }
func (m *mockFuturesClient) CloseListenKey(listenKey string) error     { return nil }

// ==================== Commission ====================
func (m *mockFuturesClient) GetCommissionRate(symbol string) (*binance.CommissionRate, error) {
	return &binance.CommissionRate{
		Symbol:              symbol,
		MakerCommissionRate: 0.0002, // 0.02%
		TakerCommissionRate: 0.0005, // 0.05%
	}, nil
}

// ==================== HELPER FUNCTIONS ====================

// createTestLogger returns a logger that outputs to stdout for testing
func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

// createBullishKlines creates 100 klines with a strong uptrend pattern (EMA20 > EMA50)
// Price increases significantly to ensure EMA20 > EMA50 by more than 0.5%
func createBullishKlines(basePrice float64) []binance.Kline {
	klines := make([]binance.Kline, 100)
	for i := 0; i < 100; i++ {
		// Price increases exponentially to create clear bullish EMA crossover
		// Use basePrice * (1 + 0.5% per candle) to ensure EMA20 >> EMA50
		multiplier := 1.0 + float64(i)*0.005 // 0.5% increase per candle
		price := basePrice * multiplier
		klines[i] = binance.Kline{
			OpenTime:  int64(i * 60000),
			Open:      price - 0.5,
			High:      price + 1.0,
			Low:       price - 1.0,
			Close:     price,
			Volume:    1000.0,
			CloseTime: int64((i + 1) * 60000),
		}
	}
	return klines
}

// createBearishKlines creates 100 klines with a strong downtrend pattern (EMA20 < EMA50)
// Price decreases significantly to ensure EMA20 < EMA50 by more than 0.5%
func createBearishKlines(basePrice float64) []binance.Kline {
	klines := make([]binance.Kline, 100)
	for i := 0; i < 100; i++ {
		// Price decreases to create clear bearish EMA crossover
		// Use basePrice * (1 - 0.5% per candle) to ensure EMA20 << EMA50
		multiplier := 1.0 - float64(i)*0.005 // 0.5% decrease per candle
		if multiplier < 0.5 {
			multiplier = 0.5 // Don't let price go too low
		}
		price := basePrice * multiplier
		klines[i] = binance.Kline{
			OpenTime:  int64(i * 60000),
			Open:      price + 0.5,
			High:      price + 1.0,
			Low:       price - 1.0,
			Close:     price,
			Volume:    1000.0,
			CloseTime: int64((i + 1) * 60000),
		}
	}
	return klines
}

// createSidewaysKlines creates 100 klines with a sideways pattern (EMA20 ~ EMA50)
// Price oscillates within a tight range to keep EMAs within 0.5% of each other
func createSidewaysKlines(basePrice float64) []binance.Kline {
	klines := make([]binance.Kline, 100)
	for i := 0; i < 100; i++ {
		// Price oscillates around basePrice with tiny variance
		offset := float64(i%10-5) * 0.001 * basePrice // Very small oscillation
		price := basePrice + offset
		klines[i] = binance.Kline{
			OpenTime:  int64(i * 60000),
			Open:      price - 0.5,
			High:      price + 1.0,
			Low:       price - 1.0,
			Close:     price,
			Volume:    1000.0,
			CloseTime: int64((i + 1) * 60000),
		}
	}
	return klines
}

// ==================== PRICE VS EMA TESTS ====================

func TestPriceVsEMA_BlocksLongBelowEMA(t *testing.T) {
	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      true,
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// Test: price=100, ema=110, direction=long -> should block
	passed, reason := validator.ValidateAll("ETHUSDT", "long", 100.0, 110.0, 0)

	if passed {
		t.Error("Expected block when price below EMA for LONG")
	}
	if reason == "" {
		t.Error("Expected rejection reason")
	}
	if !containsSubstring(reason, "below EMA") {
		t.Errorf("Expected reason to mention 'below EMA', got: %s", reason)
	}
}

func TestPriceVsEMA_AllowsLongAboveEMA(t *testing.T) {
	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      true,
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// Test: price=120, ema=110, direction=long -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 120.0, 110.0, 0)

	if !passed {
		t.Errorf("Expected pass when price above EMA for LONG, got blocked: %s", reason)
	}
	if reason != "" {
		t.Errorf("Expected no rejection reason, got: %s", reason)
	}
}

func TestPriceVsEMA_BlocksShortAboveEMA(t *testing.T) {
	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      true,
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// Test: price=120, ema=110, direction=short -> should block
	passed, reason := validator.ValidateAll("ETHUSDT", "short", 120.0, 110.0, 0)

	if passed {
		t.Error("Expected block when price above EMA for SHORT")
	}
	if !containsSubstring(reason, "above EMA") {
		t.Errorf("Expected reason to mention 'above EMA', got: %s", reason)
	}
}

func TestPriceVsEMA_AllowsShortBelowEMA(t *testing.T) {
	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      true,
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// Test: price=100, ema=110, direction=short -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "SHORT", 100.0, 110.0, 0)

	if !passed {
		t.Errorf("Expected pass when price below EMA for SHORT, got blocked: %s", reason)
	}
}

func TestPriceVsEMA_SkipsWhenDisabled(t *testing.T) {
	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      false, // Disabled
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// Test: price below EMA but filter disabled -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 100.0, 110.0, 0)

	if !passed {
		t.Errorf("Expected pass when filter disabled, got blocked: %s", reason)
	}
}

func TestPriceVsEMA_SkipsWhenEMAZero(t *testing.T) {
	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      true,
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// Test: ema=0 (not calculated) -> should skip filter
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 100.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass when EMA is 0 (not available), got blocked: %s", reason)
	}
}

// ==================== VWAP TESTS ====================

func TestVWAP_BlocksLongBelowVWAP(t *testing.T) {
	config := &TrendFiltersConfig{
		VWAPFilter: &VWAPFilterConfig{
			Enabled:                       true,
			RequirePriceAboveVWAPForLong:  true,
			RequirePriceBelowVWAPForShort: true,
			NearVWAPTolerancePercent:      0.1,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// VWAP = 100, tolerance = 0.1% = 0.1
	// VWAP lower band = 100 - 0.1 = 99.9
	// Price = 99.0 is below 99.9 -> should block
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 99.0, 0, 100.0)

	if passed {
		t.Error("Expected block when price below VWAP band for LONG")
	}
	if !containsSubstring(reason, "below VWAP") {
		t.Errorf("Expected reason to mention 'below VWAP', got: %s", reason)
	}
}

func TestVWAP_AllowsWithinTolerance(t *testing.T) {
	config := &TrendFiltersConfig{
		VWAPFilter: &VWAPFilterConfig{
			Enabled:                       true,
			RequirePriceAboveVWAPForLong:  true,
			RequirePriceBelowVWAPForShort: true,
			NearVWAPTolerancePercent:      0.5, // 0.5% tolerance
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// VWAP = 100, tolerance = 0.5% = 0.5
	// VWAP lower band = 100 - 0.5 = 99.5
	// Price = 99.6 is within tolerance -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 99.6, 0, 100.0)

	if !passed {
		t.Errorf("Expected pass when price within VWAP tolerance, got blocked: %s", reason)
	}
}

func TestVWAP_SkipsWhenUnavailable(t *testing.T) {
	config := &TrendFiltersConfig{
		VWAPFilter: &VWAPFilterConfig{
			Enabled:                       true,
			RequirePriceAboveVWAPForLong:  true,
			RequirePriceBelowVWAPForShort: true,
			NearVWAPTolerancePercent:      0.1,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// VWAP = 0 (not available) -> should skip filter
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 50.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass when VWAP unavailable, got blocked: %s", reason)
	}
}

func TestVWAP_BlocksShortAboveVWAP(t *testing.T) {
	config := &TrendFiltersConfig{
		VWAPFilter: &VWAPFilterConfig{
			Enabled:                       true,
			RequirePriceAboveVWAPForLong:  true,
			RequirePriceBelowVWAPForShort: true,
			NearVWAPTolerancePercent:      0.1,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// VWAP = 100, tolerance = 0.1% = 0.1
	// VWAP upper band = 100 + 0.1 = 100.1
	// Price = 101.0 is above 100.1 -> should block
	passed, reason := validator.ValidateAll("ETHUSDT", "SHORT", 101.0, 0, 100.0)

	if passed {
		t.Error("Expected block when price above VWAP band for SHORT")
	}
	if !containsSubstring(reason, "above VWAP") {
		t.Errorf("Expected reason to mention 'above VWAP', got: %s", reason)
	}
}

func TestVWAP_AllowsShortWithinTolerance(t *testing.T) {
	config := &TrendFiltersConfig{
		VWAPFilter: &VWAPFilterConfig{
			Enabled:                       true,
			RequirePriceAboveVWAPForLong:  true,
			RequirePriceBelowVWAPForShort: true,
			NearVWAPTolerancePercent:      0.5, // 0.5% tolerance
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// VWAP = 100, tolerance = 0.5% = 0.5
	// VWAP upper band = 100 + 0.5 = 100.5
	// Price = 100.4 is within tolerance -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "SHORT", 100.4, 0, 100.0)

	if !passed {
		t.Errorf("Expected pass when price within VWAP tolerance for SHORT, got blocked: %s", reason)
	}
}

// ==================== BTC TREND TESTS ====================

func TestBTCTrendCheck_BlocksAltLongWhenBTCBearish(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBearishKlines(50000)

	config := &TrendFiltersConfig{
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                     true,
			BTCSymbol:                   "BTCUSDT",
			BlockAltLongWhenBTCBearish:  true,
			BlockAltShortWhenBTCBullish: true,
			BTCTrendTimeframe:           "15m",
		},
	}

	validator := NewTrendFilterValidator(config, nil, mockClient, createTestLogger())

	// Test: Altcoin LONG when BTC bearish -> should block
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)

	if passed {
		t.Error("Expected block when BTC bearish for altcoin LONG")
	}
	if !containsSubstring(reason, "BTC trend bearish") {
		t.Errorf("Expected reason to mention 'BTC trend bearish', got: %s", reason)
	}
}

func TestBTCTrendCheck_AllowsAltLongWhenBTCBullish(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBullishKlines(50000)

	config := &TrendFiltersConfig{
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                     true,
			BTCSymbol:                   "BTCUSDT",
			BlockAltLongWhenBTCBearish:  true,
			BlockAltShortWhenBTCBullish: true,
			BTCTrendTimeframe:           "15m",
		},
	}

	validator := NewTrendFilterValidator(config, nil, mockClient, createTestLogger())

	// Test: Altcoin LONG when BTC bullish -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass when BTC bullish for altcoin LONG, got blocked: %s", reason)
	}
}

func TestBTCTrendCheck_BypassesForBTCUSDT(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBearishKlines(50000)

	config := &TrendFiltersConfig{
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                     true,
			BTCSymbol:                   "BTCUSDT",
			BlockAltLongWhenBTCBearish:  true,
			BlockAltShortWhenBTCBullish: true,
			BTCTrendTimeframe:           "15m",
		},
	}

	validator := NewTrendFilterValidator(config, nil, mockClient, createTestLogger())

	// Test: BTCUSDT should bypass BTC check
	passed, reason := validator.ValidateAll("BTCUSDT", "LONG", 50000.0, 0, 0)

	if !passed {
		t.Errorf("Expected BTCUSDT to bypass BTC check, got blocked: %s", reason)
	}

	// Verify no API call was made for BTC trend
	if mockClient.callCount > 0 {
		t.Error("Expected no API calls for BTCUSDT, but calls were made")
	}
}

func TestBTCTrendCheck_BlocksAltShortWhenBTCBullish(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBullishKlines(50000)

	config := &TrendFiltersConfig{
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                     true,
			BTCSymbol:                   "BTCUSDT",
			BlockAltLongWhenBTCBearish:  true,
			BlockAltShortWhenBTCBullish: true,
			BTCTrendTimeframe:           "15m",
		},
	}

	validator := NewTrendFilterValidator(config, nil, mockClient, createTestLogger())

	// Test: Altcoin SHORT when BTC bullish -> should block
	passed, reason := validator.ValidateAll("ETHUSDT", "SHORT", 2000.0, 0, 0)

	if passed {
		t.Error("Expected block when BTC bullish for altcoin SHORT")
	}
	if !containsSubstring(reason, "BTC trend bullish") {
		t.Errorf("Expected reason to mention 'BTC trend bullish', got: %s", reason)
	}
}

func TestBTCTrendCheck_AllowsAltShortWhenBTCBearish(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBearishKlines(50000)

	config := &TrendFiltersConfig{
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                     true,
			BTCSymbol:                   "BTCUSDT",
			BlockAltLongWhenBTCBearish:  true,
			BlockAltShortWhenBTCBullish: true,
			BTCTrendTimeframe:           "15m",
		},
	}

	validator := NewTrendFilterValidator(config, nil, mockClient, createTestLogger())

	// Test: Altcoin SHORT when BTC bearish -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "SHORT", 2000.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass when BTC bearish for altcoin SHORT, got blocked: %s", reason)
	}
}

func TestBTCTrendCheck_AllowsOnAPIError(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klinesErr = errors.New("API error")

	config := &TrendFiltersConfig{
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                     true,
			BTCSymbol:                   "BTCUSDT",
			BlockAltLongWhenBTCBearish:  true,
			BlockAltShortWhenBTCBullish: true,
			BTCTrendTimeframe:           "15m",
		},
	}

	validator := NewTrendFilterValidator(config, nil, mockClient, createTestLogger())

	// Test: API error -> should pass (don't block on API issues)
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass on API error, got blocked: %s", reason)
	}
}

// ==================== HIGHER TIMEFRAME TESTS ====================

func TestHigherTF_Blocks1hBearishFor15mLong(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["ETHUSDT:1h"] = createBearishKlines(2000)

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// Test: 1h bearish, 15m long -> should block
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)

	if passed {
		t.Error("Expected block when 1h trend bearish for LONG")
	}
	if !containsSubstring(reason, "bearish") {
		t.Errorf("Expected reason to mention 'bearish', got: %s", reason)
	}
}

func TestHigherTF_Allows1hBullishFor15mLong(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["ETHUSDT:1h"] = createBullishKlines(2000)

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// Test: 1h bullish, 15m long -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass when 1h trend bullish for LONG, got blocked: %s", reason)
	}
}

func TestHigherTF_BlocksBullishForShort(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["ETHUSDT:1h"] = createBullishKlines(2000)

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// Test: 1h bullish, direction=SHORT -> should block
	passed, reason := validator.ValidateAll("ETHUSDT", "SHORT", 2000.0, 0, 0)

	if passed {
		t.Error("Expected block when 1h trend bullish for SHORT")
	}
	if !containsSubstring(reason, "bullish") {
		t.Errorf("Expected reason to mention 'bullish', got: %s", reason)
	}
}

func TestHigherTF_AllowsBearishForShort(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["ETHUSDT:1h"] = createBearishKlines(2000)

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// Test: 1h bearish, direction=SHORT -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "SHORT", 2000.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass when 1h trend bearish for SHORT, got blocked: %s", reason)
	}
}

func TestHigherTF_AllowsSidewaysForBothDirections(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["ETHUSDT:1h"] = createSidewaysKlines(2000)

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// Test: Sideways should allow LONG
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)
	if !passed {
		t.Errorf("Expected pass when 1h sideways for LONG, got blocked: %s", reason)
	}

	// Clear cache to test SHORT
	validator.ClearCache()

	// Test: Sideways should allow SHORT
	passed, reason = validator.ValidateAll("ETHUSDT", "SHORT", 2000.0, 0, 0)
	if !passed {
		t.Errorf("Expected pass when 1h sideways for SHORT, got blocked: %s", reason)
	}
}

func TestHigherTF_SkipsWhenDisabled(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["ETHUSDT:1h"] = createBearishKlines(2000)

	mtfConfig := &ModeMTFConfig{
		Enabled:          false, // Disabled
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// Test: MTF disabled -> should pass without API call
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass when MTF disabled, got blocked: %s", reason)
	}

	if mockClient.callCount > 0 {
		t.Error("Expected no API calls when MTF disabled")
	}
}

func TestHigherTF_AllowsOnAPIError(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klinesErr = errors.New("API error")

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// Test: API error -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass on API error, got blocked: %s", reason)
	}
}

func TestHigherTF_SkipsWithInsufficientKlines(t *testing.T) {
	mockClient := newMockFuturesClient()
	// Only 10 klines (less than required 50)
	mockClient.klines["ETHUSDT:1h"] = createBearishKlines(2000)[:10]

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// Test: Insufficient klines -> should pass
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)

	if !passed {
		t.Errorf("Expected pass with insufficient klines, got blocked: %s", reason)
	}
}

// ==================== COMBINED FILTER TESTS ====================

func TestCombinedFilters_AllMustPass(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBullishKlines(50000)
	mockClient.klines["ETHUSDT:1h"] = createBullishKlines(2000)

	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      true,
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
		VWAPFilter: &VWAPFilterConfig{
			Enabled:                       true,
			RequirePriceAboveVWAPForLong:  true,
			RequirePriceBelowVWAPForShort: true,
			NearVWAPTolerancePercent:      0.1,
		},
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                     true,
			BTCSymbol:                   "BTCUSDT",
			BlockAltLongWhenBTCBearish:  true,
			BlockAltShortWhenBTCBullish: true,
			BTCTrendTimeframe:           "15m",
		},
	}

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(config, mtfConfig, mockClient, createTestLogger())

	// Test: All conditions met for LONG
	// price=2100, ema=2000 (above), vwap=2050 (above-tolerance), BTC bullish, 1h bullish
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 2100.0, 2000.0, 2050.0)

	if !passed {
		t.Errorf("Expected pass when all filters pass, got blocked: %s", reason)
	}
}

func TestCombinedFilters_FirstFailureStopsEvaluation(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBullishKlines(50000)
	mockClient.klines["ETHUSDT:1h"] = createBullishKlines(2000)

	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      true,
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
		VWAPFilter: &VWAPFilterConfig{
			Enabled:                       true,
			RequirePriceAboveVWAPForLong:  true,
			RequirePriceBelowVWAPForShort: true,
			NearVWAPTolerancePercent:      0.1,
		},
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                     true,
			BTCSymbol:                   "BTCUSDT",
			BlockAltLongWhenBTCBearish:  true,
			BlockAltShortWhenBTCBullish: true,
			BTCTrendTimeframe:           "15m",
		},
	}

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(config, mtfConfig, mockClient, createTestLogger())

	// Test: Price/EMA fails first -> no API calls should be made
	// price=1900, ema=2000 (below) -> should block at Price/EMA
	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 1900.0, 2000.0, 2050.0)

	if passed {
		t.Error("Expected block at Price/EMA filter")
	}
	if !containsSubstring(reason, "below EMA") {
		t.Errorf("Expected Price/EMA rejection, got: %s", reason)
	}

	// Verify no API calls were made (early exit)
	if mockClient.callCount > 0 {
		t.Errorf("Expected no API calls on early exit, got %d calls", mockClient.callCount)
	}
}

// ==================== FILTER ORDER TESTS ====================

func TestFilterOrder_FastestFirst(t *testing.T) {
	// This test verifies the order of filter execution:
	// 1. Price/EMA (instant)
	// 2. VWAP (instant)
	// 3. Higher TF (may need API)
	// 4. BTC Trend (may need API)

	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBullishKlines(50000)
	mockClient.klines["ETHUSDT:1h"] = createBullishKlines(2000)

	// Test 1: Price/EMA fails -> no API calls
	config1 := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                     true,
			RequirePriceAboveEMAForLong: true,
			EMAPeriod:                   20,
		},
		VWAPFilter: &VWAPFilterConfig{
			Enabled:                      true,
			RequirePriceAboveVWAPForLong: true,
			NearVWAPTolerancePercent:     0.1,
		},
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                    true,
			BTCSymbol:                  "BTCUSDT",
			BlockAltLongWhenBTCBearish: true,
			BTCTrendTimeframe:          "15m",
		},
	}
	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator1 := NewTrendFilterValidator(config1, mtfConfig, mockClient, createTestLogger())
	passed, _ := validator1.ValidateAll("ETHUSDT", "LONG", 90.0, 100.0, 95.0) // price < ema
	if passed {
		t.Error("Expected block at Price/EMA")
	}
	if mockClient.callCount > 0 {
		t.Error("Price/EMA filter should be checked before API calls")
	}

	// Test 2: Price/EMA passes, VWAP fails -> no API calls
	mockClient2 := newMockFuturesClient()
	mockClient2.klines["BTCUSDT:15m"] = createBullishKlines(50000)
	mockClient2.klines["ETHUSDT:1h"] = createBullishKlines(2000)

	validator2 := NewTrendFilterValidator(config1, mtfConfig, mockClient2, createTestLogger())
	passed, reason := validator2.ValidateAll("ETHUSDT", "LONG", 110.0, 100.0, 200.0) // price > ema, but price << vwap
	if passed {
		t.Error("Expected block at VWAP")
	}
	if !containsSubstring(reason, "VWAP") {
		t.Errorf("Expected VWAP rejection, got: %s", reason)
	}
	if mockClient2.callCount > 0 {
		t.Error("VWAP filter should be checked before API calls")
	}
}

// ==================== CACHE TESTS ====================

func TestBTCCache_ReusesWithinTTL(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBullishKlines(50000)

	config := &TrendFiltersConfig{
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                    true,
			BTCSymbol:                  "BTCUSDT",
			BlockAltLongWhenBTCBearish: true,
			BTCTrendTimeframe:          "15m",
		},
	}

	validator := NewTrendFilterValidator(config, nil, mockClient, createTestLogger())

	// First call - should fetch
	validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)
	firstCallCount := mockClient.callCount

	// Second call immediately - should use cache
	validator.ValidateAll("XRPUSDT", "LONG", 1.0, 0, 0)
	secondCallCount := mockClient.callCount

	if secondCallCount != firstCallCount {
		t.Errorf("Expected cache reuse, but got additional API calls: %d -> %d",
			firstCallCount, secondCallCount)
	}
}

func TestBTCCache_RefreshesAfterTTL(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBullishKlines(50000)

	config := &TrendFiltersConfig{
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                    true,
			BTCSymbol:                  "BTCUSDT",
			BlockAltLongWhenBTCBearish: true,
			BTCTrendTimeframe:          "15m",
		},
	}

	validator := NewTrendFilterValidator(config, nil, mockClient, createTestLogger())

	// First call - should fetch
	validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)
	firstCallCount := mockClient.callCount

	// Manually expire the cache
	validator.btcCache.mu.Lock()
	validator.btcCache.timestamp = time.Now().Add(-10 * time.Minute) // Expired
	validator.btcCache.mu.Unlock()

	// Next call should fetch again
	validator.ValidateAll("XRPUSDT", "LONG", 1.0, 0, 0)
	secondCallCount := mockClient.callCount

	if secondCallCount <= firstCallCount {
		t.Error("Expected cache refresh after TTL expiry, but no new API call was made")
	}
}

func TestHigherTFCache_ReusesWithinTTL(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["ETHUSDT:1h"] = createBullishKlines(2000)

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// First call - should fetch
	validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)
	firstCallCount := mockClient.callCount

	// Second call for same symbol - should use cache
	validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)
	secondCallCount := mockClient.callCount

	if secondCallCount != firstCallCount {
		t.Errorf("Expected Higher TF cache reuse, but got additional API calls: %d -> %d",
			firstCallCount, secondCallCount)
	}
}

func TestHigherTFCache_DifferentSymbolsFetchSeparately(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["ETHUSDT:1h"] = createBullishKlines(2000)
	mockClient.klines["XRPUSDT:1h"] = createBullishKlines(1.0)

	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(nil, mtfConfig, mockClient, createTestLogger())

	// First symbol
	validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)
	firstCallCount := mockClient.callCount

	// Different symbol - should fetch
	validator.ValidateAll("XRPUSDT", "LONG", 1.0, 0, 0)
	secondCallCount := mockClient.callCount

	if secondCallCount <= firstCallCount {
		t.Error("Expected separate fetch for different symbols")
	}
}

// ==================== TABLE-DRIVEN TESTS ====================

func TestPriceVsEMA_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		direction   string
		price       float64
		ema         float64
		expectPass  bool
		expectInMsg string
	}{
		{"LONG price above EMA", "LONG", 110.0, 100.0, true, ""},
		{"LONG price equal EMA", "LONG", 100.0, 100.0, true, ""},
		{"LONG price below EMA", "LONG", 90.0, 100.0, false, "below EMA"},
		{"SHORT price below EMA", "SHORT", 90.0, 100.0, true, ""},
		{"SHORT price equal EMA", "SHORT", 100.0, 100.0, true, ""},
		{"SHORT price above EMA", "SHORT", 110.0, 100.0, false, "above EMA"},
		{"LONG EMA zero (skip)", "LONG", 90.0, 0, true, ""},
		{"SHORT EMA negative (skip)", "SHORT", 110.0, -1.0, true, ""},
	}

	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      true,
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())
			passed, reason := validator.ValidateAll("ETHUSDT", tt.direction, tt.price, tt.ema, 0)

			if passed != tt.expectPass {
				t.Errorf("Expected pass=%v, got pass=%v, reason=%s", tt.expectPass, passed, reason)
			}
			if !tt.expectPass && tt.expectInMsg != "" && !containsSubstring(reason, tt.expectInMsg) {
				t.Errorf("Expected reason to contain '%s', got: %s", tt.expectInMsg, reason)
			}
		})
	}
}

func TestVWAP_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		direction   string
		price       float64
		vwap        float64
		tolerance   float64
		expectPass  bool
		expectInMsg string
	}{
		{"LONG well above VWAP", "LONG", 105.0, 100.0, 0.1, true, ""},
		{"LONG within tolerance", "LONG", 99.95, 100.0, 0.1, true, ""},
		{"LONG below tolerance", "LONG", 99.8, 100.0, 0.1, false, "below VWAP"},
		{"SHORT well below VWAP", "SHORT", 95.0, 100.0, 0.1, true, ""},
		{"SHORT within tolerance", "SHORT", 100.05, 100.0, 0.1, true, ""},
		{"SHORT above tolerance", "SHORT", 100.2, 100.0, 0.1, false, "above VWAP"},
		{"LONG VWAP zero (skip)", "LONG", 50.0, 0, 0.1, true, ""},
		{"SHORT VWAP negative (skip)", "SHORT", 150.0, -1.0, 0.1, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &TrendFiltersConfig{
				VWAPFilter: &VWAPFilterConfig{
					Enabled:                       true,
					RequirePriceAboveVWAPForLong:  true,
					RequirePriceBelowVWAPForShort: true,
					NearVWAPTolerancePercent:      tt.tolerance,
				},
			}
			validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())
			passed, reason := validator.ValidateAll("ETHUSDT", tt.direction, tt.price, 0, tt.vwap)

			if passed != tt.expectPass {
				t.Errorf("Expected pass=%v, got pass=%v, reason=%s", tt.expectPass, passed, reason)
			}
			if !tt.expectPass && tt.expectInMsg != "" && !containsSubstring(reason, tt.expectInMsg) {
				t.Errorf("Expected reason to contain '%s', got: %s", tt.expectInMsg, reason)
			}
		})
	}
}

func TestBTCTrendCheck_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		symbol     string
		direction  string
		btcTrend   string // "bullish" or "bearish"
		expectPass bool
	}{
		{"Alt LONG, BTC bullish", "ETHUSDT", "LONG", "bullish", true},
		{"Alt LONG, BTC bearish", "ETHUSDT", "LONG", "bearish", false},
		{"Alt SHORT, BTC bearish", "ETHUSDT", "SHORT", "bearish", true},
		{"Alt SHORT, BTC bullish", "ETHUSDT", "SHORT", "bullish", false},
		{"BTC LONG, BTC bearish (bypass)", "BTCUSDT", "LONG", "bearish", true},
		{"BTC SHORT, BTC bullish (bypass)", "BTCUSDT", "SHORT", "bullish", true},
		{"BTCDOM LONG, BTC bearish (bypass)", "BTCDOMUSDT", "LONG", "bearish", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockFuturesClient()
			if tt.btcTrend == "bullish" {
				mockClient.klines["BTCUSDT:15m"] = createBullishKlines(50000)
			} else {
				mockClient.klines["BTCUSDT:15m"] = createBearishKlines(50000)
			}

			config := &TrendFiltersConfig{
				BTCTrendCheck: &BTCTrendCheckConfig{
					Enabled:                     true,
					BTCSymbol:                   "BTCUSDT",
					BlockAltLongWhenBTCBearish:  true,
					BlockAltShortWhenBTCBullish: true,
					BTCTrendTimeframe:           "15m",
				},
			}

			validator := NewTrendFilterValidator(config, nil, mockClient, createTestLogger())
			passed, _ := validator.ValidateAll(tt.symbol, tt.direction, 1000.0, 0, 0)

			if passed != tt.expectPass {
				t.Errorf("Expected pass=%v, got pass=%v", tt.expectPass, passed)
			}
		})
	}
}

// ==================== EDGE CASES ====================

func TestNilConfig_PassesAll(t *testing.T) {
	validator := NewTrendFilterValidator(nil, nil, nil, createTestLogger())

	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 100.0, 110.0, 50.0)

	if !passed {
		t.Errorf("Expected pass with nil config, got blocked: %s", reason)
	}
}

func TestEmptyConfig_PassesAll(t *testing.T) {
	config := &TrendFiltersConfig{}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	passed, reason := validator.ValidateAll("ETHUSDT", "LONG", 100.0, 110.0, 50.0)

	if !passed {
		t.Errorf("Expected pass with empty config, got blocked: %s", reason)
	}
}

func TestDirectionCaseInsensitive(t *testing.T) {
	config := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                      true,
			RequirePriceAboveEMAForLong:  true,
			RequirePriceBelowEMAForShort: true,
			EMAPeriod:                    20,
		},
	}

	validator := NewTrendFilterValidator(config, nil, nil, createTestLogger())

	// Test lowercase
	passed1, _ := validator.ValidateAll("ETHUSDT", "long", 110.0, 100.0, 0)
	// Test uppercase
	passed2, _ := validator.ValidateAll("ETHUSDT", "LONG", 110.0, 100.0, 0)
	// Test mixed case
	passed3, _ := validator.ValidateAll("ETHUSDT", "Long", 110.0, 100.0, 0)

	if !passed1 || !passed2 || !passed3 {
		t.Error("Expected direction to be case-insensitive")
	}
}

func TestClearCache(t *testing.T) {
	mockClient := newMockFuturesClient()
	mockClient.klines["BTCUSDT:15m"] = createBullishKlines(50000)
	mockClient.klines["ETHUSDT:1h"] = createBullishKlines(2000)

	config := &TrendFiltersConfig{
		BTCTrendCheck: &BTCTrendCheckConfig{
			Enabled:                    true,
			BTCSymbol:                  "BTCUSDT",
			BlockAltLongWhenBTCBearish: true,
			BTCTrendTimeframe:          "15m",
		},
	}
	mtfConfig := &ModeMTFConfig{
		Enabled:          true,
		PrimaryTimeframe: "1h",
	}

	validator := NewTrendFilterValidator(config, mtfConfig, mockClient, createTestLogger())

	// Populate caches
	validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)
	firstCallCount := mockClient.callCount

	// Clear cache
	validator.ClearCache()

	// Should fetch again
	validator.ValidateAll("ETHUSDT", "LONG", 2000.0, 0, 0)
	secondCallCount := mockClient.callCount

	if secondCallCount <= firstCallCount {
		t.Error("Expected fresh fetch after cache clear")
	}
}

func TestUpdateConfig(t *testing.T) {
	config1 := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled:                     true,
			RequirePriceAboveEMAForLong: true,
			EMAPeriod:                   20,
		},
	}

	validator := NewTrendFilterValidator(config1, nil, nil, createTestLogger())

	// Should block with config1
	passed1, _ := validator.ValidateAll("ETHUSDT", "LONG", 90.0, 100.0, 0)
	if passed1 {
		t.Error("Expected block with original config")
	}

	// Update to disabled config
	config2 := &TrendFiltersConfig{
		PriceVsEMA: &PriceVsEMAConfig{
			Enabled: false,
		},
	}
	validator.UpdateConfig(config2, nil)

	// Should pass with updated config
	passed2, _ := validator.ValidateAll("ETHUSDT", "LONG", 90.0, 100.0, 0)
	if !passed2 {
		t.Error("Expected pass after config update to disabled")
	}
}

// ==================== HELPER FUNCTIONS ====================

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
