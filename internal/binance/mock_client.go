package binance

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// MockClient provides simulated market data for development/testing
type MockClient struct {
	baseClient *Client
	prices     map[string]float64
	lastUpdate time.Time
	mu         sync.RWMutex // Protects prices map and lastUpdate
}

// NewMockClient creates a new mock client
func NewMockClient() *MockClient {
	rand.Seed(time.Now().UnixNano())

	mc := &MockClient{
		prices:     make(map[string]float64),
		lastUpdate: time.Now(),
	}

	// Initialize with realistic base prices
	mc.prices = map[string]float64{
		"BTCUSDT":  104500.00,
		"ETHUSDT":  3900.00,
		"BNBUSDT":  710.00,
		"SOLUSDT":  220.00,
		"XRPUSDT":  2.35,
		"ADAUSDT":  1.05,
		"DOGEUSDT": 0.40,
		"AVAXUSDT": 50.00,
		"DOTUSDT":  9.50,
		"MATICUSDT": 0.55,
		"LINKUSDT": 28.00,
		"UNIUSDT":  17.50,
		"ATOMUSDT": 12.00,
		"LTCUSDT":  115.00,
		"ETCUSDT":  32.00,
		"XLMUSDT":  0.45,
		"NEARUSDT": 7.00,
		"APTUSDT":  13.50,
		"ARBUSDT":  1.10,
		"OPUSDT":   2.80,
	}

	return mc
}

// updatePrices adds small random variations to simulate market movement
func (mc *MockClient) updatePrices() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if time.Since(mc.lastUpdate) < time.Second {
		return
	}

	for symbol, price := range mc.prices {
		// Random walk: -0.5% to +0.5% change
		change := (rand.Float64() - 0.5) * 0.01
		mc.prices[symbol] = price * (1 + change)
	}
	mc.lastUpdate = time.Now()
}

// GetKlines returns simulated candlestick data
func (mc *MockClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	mc.updatePrices()

	mc.mu.RLock()
	basePrice, ok := mc.prices[symbol]
	mc.mu.RUnlock()
	if !ok {
		basePrice = 100.0
	}

	// Determine interval duration
	var intervalDuration time.Duration
	switch interval {
	case "1m":
		intervalDuration = time.Minute
	case "5m":
		intervalDuration = 5 * time.Minute
	case "15m":
		intervalDuration = 15 * time.Minute
	case "1h":
		intervalDuration = time.Hour
	case "4h":
		intervalDuration = 4 * time.Hour
	case "1d":
		intervalDuration = 24 * time.Hour
	default:
		intervalDuration = time.Minute
	}

	klines := make([]Kline, limit)
	now := time.Now()

	// Generate historical klines working backwards
	currentPrice := basePrice
	for i := limit - 1; i >= 0; i-- {
		openTime := now.Add(-time.Duration(limit-i) * intervalDuration)
		closeTime := openTime.Add(intervalDuration)

		// Generate OHLCV data with some volatility
		volatility := 0.02 // 2% volatility
		open := currentPrice
		change := (rand.Float64() - 0.5) * volatility * 2
		close := open * (1 + change)

		high := math.Max(open, close) * (1 + rand.Float64()*volatility*0.5)
		low := math.Min(open, close) * (1 - rand.Float64()*volatility*0.5)

		volume := basePrice * (1000 + rand.Float64()*5000)

		klines[i] = Kline{
			OpenTime:                 openTime.UnixMilli(),
			Open:                     open,
			High:                     high,
			Low:                      low,
			Close:                    close,
			Volume:                   volume / basePrice,
			CloseTime:                closeTime.UnixMilli(),
			QuoteAssetVolume:         volume,
			NumberOfTrades:           int(100 + rand.Float64()*1000),
			TakerBuyBaseAssetVolume:  volume / basePrice * 0.5,
			TakerBuyQuoteAssetVolume: volume * 0.5,
		}

		currentPrice = close
	}

	return klines, nil
}

// Get24hrTickers returns simulated 24hr ticker data
func (mc *MockClient) Get24hrTickers() ([]Ticker24hr, error) {
	mc.updatePrices()

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	tickers := make([]Ticker24hr, 0, len(mc.prices))
	now := time.Now()

	for symbol, price := range mc.prices {
		// Generate realistic 24hr stats
		priceChange := (rand.Float64() - 0.5) * price * 0.1 // -5% to +5%
		priceChangePercent := (priceChange / price) * 100

		ticker := Ticker24hr{
			Symbol:             symbol,
			PriceChange:        priceChange,
			PriceChangePercent: priceChangePercent,
			WeightedAvgPrice:   price * (1 + (rand.Float64()-0.5)*0.02),
			LastPrice:          price,
			Volume:             1000000 + rand.Float64()*10000000,
			QuoteVolume:        price * (1000000 + rand.Float64()*10000000),
			OpenTime:           now.Add(-24 * time.Hour).UnixMilli(),
			CloseTime:          now.UnixMilli(),
			FirstId:            1,
			LastId:             100000 + rand.Int63n(100000),
			Count:              100000 + rand.Int63n(100000),
		}
		tickers = append(tickers, ticker)
	}

	return tickers, nil
}

// GetCurrentPrice returns simulated current price
func (mc *MockClient) GetCurrentPrice(symbol string) (float64, error) {
	mc.updatePrices()

	mc.mu.RLock()
	price, ok := mc.prices[symbol]
	mc.mu.RUnlock()

	if ok {
		return price, nil
	}
	return 100.0, nil
}

// GetExchangeInfo returns simulated exchange info
func (mc *MockClient) GetExchangeInfo() (*ExchangeInfo, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	symbols := make([]SymbolInfo, 0, len(mc.prices))

	for symbol := range mc.prices {
		baseAsset := symbol[:len(symbol)-4] // Remove "USDT"
		symbols = append(symbols, SymbolInfo{
			Symbol:               symbol,
			Status:               "TRADING",
			BaseAsset:            baseAsset,
			QuoteAsset:           "USDT",
			IsSpotTradingAllowed: true,
		})
	}

	return &ExchangeInfo{Symbols: symbols}, nil
}

// GetAllSymbols returns all mock trading pairs
func (mc *MockClient) GetAllSymbols() ([]string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	symbols := make([]string, 0, len(mc.prices))
	for symbol := range mc.prices {
		symbols = append(symbols, symbol)
	}
	return symbols, nil
}

// PlaceOrder simulates order placement (always succeeds in mock mode)
func (mc *MockClient) PlaceOrder(params map[string]string) (*OrderResponse, error) {
	symbol := params["symbol"]
	side := params["side"]
	quantity := params["quantity"]

	price, _ := mc.GetCurrentPrice(symbol)
	qty := parseFloat(quantity)

	return &OrderResponse{
		Symbol:              symbol,
		OrderId:             rand.Int63n(1000000),
		ClientOrderId:       "mock_" + time.Now().Format("20060102150405"),
		TransactTime:        time.Now().UnixMilli(),
		Price:               price,
		OrigQty:             qty,
		ExecutedQty:         qty,
		CummulativeQuoteQty: price * qty,
		Status:              "FILLED",
		Type:                params["type"],
		Side:                side,
	}, nil
}

// CancelOrder simulates order cancellation
func (mc *MockClient) CancelOrder(symbol string, orderId int64) error {
	return nil
}

// GetAccountInfo returns simulated account information
func (mc *MockClient) GetAccountInfo() (*AccountInfo, error) {
	return &AccountInfo{
		MakerCommission:  10,
		TakerCommission:  10,
		BuyerCommission:  0,
		SellerCommission: 0,
		CanTrade:         true,
		CanWithdraw:      true,
		CanDeposit:       true,
		UpdateTime:       time.Now().UnixMilli(),
		AccountType:      "SPOT",
		Balances: []AssetBalance{
			{Asset: "USDT", Free: "10000.00000000", Locked: "500.00000000"},
			{Asset: "BTC", Free: "0.10000000", Locked: "0.00000000"},
			{Asset: "ETH", Free: "2.50000000", Locked: "0.00000000"},
			{Asset: "BNB", Free: "5.00000000", Locked: "0.00000000"},
		},
	}, nil
}

// GetUSDTBalance returns simulated USDT balance
func (mc *MockClient) GetUSDTBalance() (float64, error) {
	return 10500.0, nil // 10000 free + 500 locked
}
