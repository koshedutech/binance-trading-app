package binance

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// FuturesMockClient implements the FuturesClient interface for dry-run mode
type FuturesMockClient struct {
	mu            sync.RWMutex
	positions     map[string]*FuturesPosition
	orders        map[int64]*FuturesOrder
	trades        []FuturesTrade
	fundingFees   []FundingFeeRecord
	leverage      map[string]int
	marginType    map[string]MarginType
	dualPosition  bool
	balance       float64
	nextOrderId   int64
	nextTradeId   int64
	priceProvider func(symbol string) (float64, error)
}

// NewFuturesMockClient creates a new mock futures client
func NewFuturesMockClient(initialBalance float64, priceProvider func(symbol string) (float64, error)) *FuturesMockClient {
	return &FuturesMockClient{
		positions:     make(map[string]*FuturesPosition),
		orders:        make(map[int64]*FuturesOrder),
		trades:        make([]FuturesTrade, 0),
		fundingFees:   make([]FundingFeeRecord, 0),
		leverage:      make(map[string]int),
		marginType:    make(map[string]MarginType),
		dualPosition:  false,
		balance:       initialBalance,
		nextOrderId:   1000,
		nextTradeId:   1000,
		priceProvider: priceProvider,
	}
}

// ==================== ACCOUNT ====================

func (c *FuturesMockClient) GetFuturesAccountInfo() (*FuturesAccountInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalUnrealizedProfit := 0.0
	for _, pos := range c.positions {
		totalUnrealizedProfit += pos.UnrealizedProfit
	}

	return &FuturesAccountInfo{
		CanTrade:              true,
		CanDeposit:            true,
		CanWithdraw:           true,
		TotalWalletBalance:    c.balance,
		TotalUnrealizedProfit: totalUnrealizedProfit,
		TotalMarginBalance:    c.balance + totalUnrealizedProfit,
		AvailableBalance:      c.balance,
		Assets: []FuturesAsset{
			{
				Asset:            "USDT",
				WalletBalance:    c.balance,
				AvailableBalance: c.balance,
				MarginAvailable:  true,
			},
		},
	}, nil
}

func (c *FuturesMockClient) GetPositions() ([]FuturesPosition, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	positions := make([]FuturesPosition, 0, len(c.positions))
	for _, pos := range c.positions {
		// Update mark price and unrealized PnL
		if c.priceProvider != nil {
			if price, err := c.priceProvider(pos.Symbol); err == nil {
				pos.MarkPrice = price
				if pos.PositionAmt > 0 {
					pos.UnrealizedProfit = (price - pos.EntryPrice) * pos.PositionAmt
				} else {
					pos.UnrealizedProfit = (pos.EntryPrice - price) * (-pos.PositionAmt)
				}
			}
		}
		positions = append(positions, *pos)
	}

	return positions, nil
}

func (c *FuturesMockClient) GetPositionBySymbol(symbol string) (*FuturesPosition, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pos, exists := c.positions[symbol]
	if !exists {
		return &FuturesPosition{
			Symbol:       symbol,
			PositionAmt:  0,
			EntryPrice:   0,
			Leverage:     c.getLeverageLocked(symbol),
			MarginType:   string(c.getMarginTypeLocked(symbol)),
			PositionSide: "BOTH",
		}, nil
	}

	return pos, nil
}

// ==================== LEVERAGE & MARGIN ====================

func (c *FuturesMockClient) SetLeverage(symbol string, leverage int) (*LeverageResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if leverage < 1 || leverage > 125 {
		return nil, fmt.Errorf("invalid leverage: must be between 1 and 125")
	}

	c.leverage[symbol] = leverage

	return &LeverageResponse{
		Leverage:         leverage,
		MaxNotionalValue: 1000000.0 / float64(leverage),
		Symbol:           symbol,
	}, nil
}

func (c *FuturesMockClient) SetMarginType(symbol string, marginType MarginType) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.marginType[symbol] = marginType
	return nil
}

func (c *FuturesMockClient) SetPositionMode(dualSidePosition bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Cannot change position mode while having open positions
	for _, pos := range c.positions {
		if pos.PositionAmt != 0 {
			return fmt.Errorf("cannot change position mode while having open positions")
		}
	}

	c.dualPosition = dualSidePosition
	return nil
}

func (c *FuturesMockClient) GetPositionMode() (*PositionModeResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &PositionModeResponse{
		DualSidePosition: c.dualPosition,
	}, nil
}

// ==================== TRADING ====================

func (c *FuturesMockClient) PlaceFuturesOrder(params FuturesOrderParams) (*FuturesOrderResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get current price
	var currentPrice float64
	if c.priceProvider != nil {
		price, err := c.priceProvider(params.Symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get current price: %w", err)
		}
		currentPrice = price
	} else {
		currentPrice = 50000.0 // Default mock price
	}

	// Determine execution price
	executionPrice := currentPrice
	if params.Type == FuturesOrderTypeLimit && params.Price > 0 {
		executionPrice = params.Price
	}

	// Create order
	orderId := c.nextOrderId
	c.nextOrderId++

	order := &FuturesOrder{
		OrderId:       orderId,
		Symbol:        params.Symbol,
		Status:        string(FuturesOrderStatusFilled),
		Price:         executionPrice,
		AvgPrice:      executionPrice,
		OrigQty:       params.Quantity,
		ExecutedQty:   params.Quantity,
		CumQuote:      executionPrice * params.Quantity,
		TimeInForce:   string(params.TimeInForce),
		Type:          string(params.Type),
		ReduceOnly:    params.ReduceOnly,
		ClosePosition: params.ClosePosition,
		Side:          params.Side,
		PositionSide:  string(params.PositionSide),
		StopPrice:     params.StopPrice,
		WorkingType:   string(params.WorkingType),
		Time:          time.Now().UnixMilli(),
		UpdateTime:    time.Now().UnixMilli(),
	}

	// Update position
	posKey := params.Symbol
	if c.dualPosition {
		posKey = fmt.Sprintf("%s_%s", params.Symbol, params.PositionSide)
	}

	pos, exists := c.positions[posKey]
	if !exists {
		pos = &FuturesPosition{
			Symbol:       params.Symbol,
			PositionAmt:  0,
			EntryPrice:   0,
			Leverage:     c.getLeverageLocked(params.Symbol),
			MarginType:   string(c.getMarginTypeLocked(params.Symbol)),
			PositionSide: string(params.PositionSide),
		}
		c.positions[posKey] = pos
	}

	// Calculate new position
	var qty float64
	if params.Side == "BUY" {
		qty = params.Quantity
	} else {
		qty = -params.Quantity
	}

	oldAmt := pos.PositionAmt
	newAmt := oldAmt + qty

	if newAmt != 0 {
		if oldAmt == 0 {
			pos.EntryPrice = executionPrice
		} else if (oldAmt > 0 && qty > 0) || (oldAmt < 0 && qty < 0) {
			// Adding to position - average entry price
			totalCost := (pos.EntryPrice * abs(oldAmt)) + (executionPrice * abs(qty))
			pos.EntryPrice = totalCost / abs(newAmt)
		} else {
			// Reducing position - keep original entry price
			// Realized PnL calculated here
		}
		pos.PositionAmt = newAmt
	} else {
		// Position closed
		delete(c.positions, posKey)
	}

	// Record trade
	trade := FuturesTrade{
		ID:           c.nextTradeId,
		Symbol:       params.Symbol,
		OrderId:      orderId,
		Side:         params.Side,
		Price:        executionPrice,
		Qty:          params.Quantity,
		RealizedPnl:  0, // Calculate if closing position
		QuoteQty:     executionPrice * params.Quantity,
		Commission:   executionPrice * params.Quantity * 0.0004, // 0.04% fee
		Time:         time.Now().UnixMilli(),
		PositionSide: string(params.PositionSide),
		Buyer:        params.Side == "BUY",
	}
	c.nextTradeId++
	c.trades = append(c.trades, trade)

	return &FuturesOrderResponse{
		OrderId:       order.OrderId,
		Symbol:        order.Symbol,
		Status:        order.Status,
		Price:         order.Price,
		AvgPrice:      order.AvgPrice,
		OrigQty:       order.OrigQty,
		ExecutedQty:   order.ExecutedQty,
		CumQuote:      order.CumQuote,
		TimeInForce:   order.TimeInForce,
		Type:          order.Type,
		ReduceOnly:    order.ReduceOnly,
		Side:          order.Side,
		PositionSide:  order.PositionSide,
		UpdateTime:    order.UpdateTime,
	}, nil
}

func (c *FuturesMockClient) CancelFuturesOrder(symbol string, orderId int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	order, exists := c.orders[orderId]
	if !exists {
		return fmt.Errorf("order not found: %d", orderId)
	}

	if order.Symbol != symbol {
		return fmt.Errorf("symbol mismatch")
	}

	if order.Status != string(FuturesOrderStatusNew) {
		return fmt.Errorf("order cannot be canceled")
	}

	order.Status = string(FuturesOrderStatusCanceled)
	return nil
}

func (c *FuturesMockClient) CancelAllFuturesOrders(symbol string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, order := range c.orders {
		if order.Symbol == symbol && order.Status == string(FuturesOrderStatusNew) {
			order.Status = string(FuturesOrderStatusCanceled)
		}
	}

	return nil
}

func (c *FuturesMockClient) GetOpenOrders(symbol string) ([]FuturesOrder, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	orders := make([]FuturesOrder, 0)
	for _, order := range c.orders {
		if order.Status == string(FuturesOrderStatusNew) {
			if symbol == "" || order.Symbol == symbol {
				orders = append(orders, *order)
			}
		}
	}

	return orders, nil
}

func (c *FuturesMockClient) GetOrder(symbol string, orderId int64) (*FuturesOrder, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	order, exists := c.orders[orderId]
	if !exists {
		return nil, fmt.Errorf("order not found: %d", orderId)
	}

	return order, nil
}

// ==================== ALGO ORDERS (NEW API as of 2025-12-09) ====================

func (c *FuturesMockClient) PlaceAlgoOrder(params AlgoOrderParams) (*AlgoOrderResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	algoId := c.nextOrderId
	c.nextOrderId++

	return &AlgoOrderResponse{
		AlgoId:        algoId,
		AlgoType:      string(AlgoTypeConditional),
		OrderType:     string(params.Type),
		Symbol:        params.Symbol,
		Side:          params.Side,
		PositionSide:  string(params.PositionSide),
		AlgoStatus:    string(AlgoOrderStatusNew),
		TriggerPrice:  params.TriggerPrice,
		Price:         params.Price,
		Quantity:      params.Quantity,
		WorkingType:   string(params.WorkingType),
		ClosePosition: params.ClosePosition,
		ReduceOnly:    params.ReduceOnly,
		CreateTime:    time.Now().UnixMilli(),
		UpdateTime:    time.Now().UnixMilli(),
	}, nil
}

func (c *FuturesMockClient) GetOpenAlgoOrders(symbol string) ([]AlgoOrder, error) {
	// Mock returns empty - in real implementation would track algo orders
	return []AlgoOrder{}, nil
}

func (c *FuturesMockClient) CancelAlgoOrder(symbol string, algoId int64) error {
	// Mock - always succeeds
	return nil
}

func (c *FuturesMockClient) CancelAllAlgoOrders(symbol string) error {
	// Mock - always succeeds
	return nil
}

func (c *FuturesMockClient) GetAllAlgoOrders(symbol string, limit int) ([]AlgoOrder, error) {
	// Mock returns empty - in real implementation would track algo orders
	return []AlgoOrder{}, nil
}

// ==================== MARKET DATA ====================

func (c *FuturesMockClient) GetFundingRate(symbol string) (*FundingRate, error) {
	// Mock funding rate
	return &FundingRate{
		Symbol:          symbol,
		FundingRate:     0.0001,
		FundingTime:     time.Now().UnixMilli(),
		NextFundingTime: time.Now().Add(8 * time.Hour).UnixMilli(),
		MarkPrice:       50000.0,
	}, nil
}

func (c *FuturesMockClient) GetFundingRateHistory(symbol string, limit int) ([]FundingRate, error) {
	rates := make([]FundingRate, 0, limit)
	now := time.Now()

	for i := 0; i < limit; i++ {
		fundingTime := now.Add(-time.Duration(i*8) * time.Hour)
		rates = append(rates, FundingRate{
			Symbol:      symbol,
			FundingRate: 0.0001 + (rand.Float64()-0.5)*0.0002,
			FundingTime: fundingTime.UnixMilli(),
		})
	}

	return rates, nil
}

func (c *FuturesMockClient) GetMarkPrice(symbol string) (*MarkPrice, error) {
	var price float64 = 50000.0
	if c.priceProvider != nil {
		if p, err := c.priceProvider(symbol); err == nil {
			price = p
		}
	}

	return &MarkPrice{
		Symbol:          symbol,
		MarkPrice:       price,
		IndexPrice:      price * 0.9999,
		LastFundingRate: 0.0001,
		NextFundingTime: time.Now().Add(8 * time.Hour).UnixMilli(),
		Time:            time.Now().UnixMilli(),
	}, nil
}

func (c *FuturesMockClient) GetAllMarkPrices() ([]MarkPrice, error) {
	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT"}
	prices := make([]MarkPrice, 0, len(symbols))

	for _, symbol := range symbols {
		mp, _ := c.GetMarkPrice(symbol)
		prices = append(prices, *mp)
	}

	return prices, nil
}

func (c *FuturesMockClient) GetOrderBookDepth(symbol string, limit int) (*OrderBookDepth, error) {
	var basePrice float64 = 50000.0
	if c.priceProvider != nil {
		if p, err := c.priceProvider(symbol); err == nil {
			basePrice = p
		}
	}

	bids := make([][]string, limit)
	asks := make([][]string, limit)

	for i := 0; i < limit; i++ {
		bidPrice := basePrice * (1 - float64(i)*0.0001)
		askPrice := basePrice * (1 + float64(i)*0.0001)
		qty := 0.5 + rand.Float64()*2.0

		bids[i] = []string{
			fmt.Sprintf("%.2f", bidPrice),
			fmt.Sprintf("%.4f", qty),
		}
		asks[i] = []string{
			fmt.Sprintf("%.2f", askPrice),
			fmt.Sprintf("%.4f", qty),
		}
	}

	return &OrderBookDepth{
		LastUpdateId: time.Now().UnixMilli(),
		Bids:         bids,
		Asks:         asks,
	}, nil
}

func (c *FuturesMockClient) GetFuturesKlines(symbol, interval string, limit int) ([]Kline, error) {
	var basePrice float64 = 50000.0
	if c.priceProvider != nil {
		if p, err := c.priceProvider(symbol); err == nil {
			basePrice = p
		}
	}

	klines := make([]Kline, limit)
	now := time.Now()

	for i := limit - 1; i >= 0; i-- {
		variation := (rand.Float64() - 0.5) * 0.02 * basePrice
		open := basePrice + variation
		close := open + (rand.Float64()-0.5)*0.01*basePrice
		high := max(open, close) + rand.Float64()*0.005*basePrice
		low := min(open, close) - rand.Float64()*0.005*basePrice

		klines[limit-1-i] = Kline{
			OpenTime:  now.Add(-time.Duration(i) * time.Hour).UnixMilli(),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    100 + rand.Float64()*500,
			CloseTime: now.Add(-time.Duration(i-1) * time.Hour).UnixMilli(),
		}
	}

	return klines, nil
}

func (c *FuturesMockClient) GetFuturesCurrentPrice(symbol string) (float64, error) {
	if c.priceProvider != nil {
		return c.priceProvider(symbol)
	}
	return 50000.0, nil
}

func (c *FuturesMockClient) Get24hrTicker(symbol string) (*Futures24hrTicker, error) {
	price := 50000.0
	if c.priceProvider != nil {
		if p, err := c.priceProvider(symbol); err == nil {
			price = p
		}
	}

	// Generate mock 24hr data with realistic variations
	priceChange := price * (rand.Float64()*0.1 - 0.05) // -5% to +5%
	priceChangePercent := (priceChange / price) * 100

	return &Futures24hrTicker{
		Symbol:             symbol,
		PriceChange:        priceChange,
		PriceChangePercent: priceChangePercent,
		WeightedAvgPrice:   price * 0.99,
		LastPrice:          price,
		LastQty:            rand.Float64() * 10,
		OpenPrice:          price - priceChange,
		HighPrice:          price * 1.03,
		LowPrice:           price * 0.97,
		Volume:             rand.Float64() * 10000000,
		QuoteVolume:        rand.Float64() * 500000000,
		OpenTime:           time.Now().Add(-24 * time.Hour).UnixMilli(),
		CloseTime:          time.Now().UnixMilli(),
		Count:              rand.Int63n(1000000),
	}, nil
}

func (c *FuturesMockClient) GetAll24hrTickers() ([]Futures24hrTicker, error) {
	// All 30 symbols that match GetFuturesExchangeInfo()
	symbols := []string{
		// Major cryptocurrencies
		"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
		// Popular altcoins
		"DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT", "MATICUSDT",
		// Additional popular pairs
		"DOTUSDT", "LTCUSDT", "ATOMUSDT", "UNIUSDT", "NEARUSDT",
		// High volume memecoins and trending
		"SHIBUSDT", "PEPEUSDT", "WIFUSDT",
		// Layer 2 and DeFi
		"ARBUSDT", "OPUSDT", "AAVEUSDT", "MKRUSDT",
		// Gaming and metaverse
		"SANDUSDT", "MANAUSDT", "AXSUSDT",
		// Infrastructure
		"FILUSDT", "ICPUSDT", "APTUSDT", "SUIUSDT", "SEIUSDT",
	}

	tickers := make([]Futures24hrTicker, len(symbols))
	for i, sym := range symbols {
		ticker, _ := c.Get24hrTicker(sym)
		tickers[i] = *ticker
	}
	return tickers, nil
}

// ==================== EXCHANGE INFO ====================

func (c *FuturesMockClient) GetFuturesExchangeInfo() (*FuturesExchangeInfo, error) {
	return &FuturesExchangeInfo{
		ServerTime: time.Now().UnixMilli(),
		Timezone:   "UTC",
		Symbols: []FuturesSymbolInfo{
			// Major cryptocurrencies
			{Symbol: "BTCUSDT", Pair: "BTCUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "BTC", QuoteAsset: "USDT", PricePrecision: 2, QuantityPrecision: 3},
			{Symbol: "ETHUSDT", Pair: "ETHUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "ETH", QuoteAsset: "USDT", PricePrecision: 2, QuantityPrecision: 3},
			{Symbol: "BNBUSDT", Pair: "BNBUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "BNB", QuoteAsset: "USDT", PricePrecision: 2, QuantityPrecision: 2},
			{Symbol: "SOLUSDT", Pair: "SOLUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "SOL", QuoteAsset: "USDT", PricePrecision: 2, QuantityPrecision: 0},
			{Symbol: "XRPUSDT", Pair: "XRPUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "XRP", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 1},
			// Popular altcoins
			{Symbol: "DOGEUSDT", Pair: "DOGEUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "DOGE", QuoteAsset: "USDT", PricePrecision: 5, QuantityPrecision: 0},
			{Symbol: "ADAUSDT", Pair: "ADAUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "ADA", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 0},
			{Symbol: "AVAXUSDT", Pair: "AVAXUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "AVAX", QuoteAsset: "USDT", PricePrecision: 3, QuantityPrecision: 0},
			{Symbol: "LINKUSDT", Pair: "LINKUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "LINK", QuoteAsset: "USDT", PricePrecision: 3, QuantityPrecision: 1},
			{Symbol: "MATICUSDT", Pair: "MATICUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "MATIC", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 0},
			// Additional popular pairs
			{Symbol: "DOTUSDT", Pair: "DOTUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "DOT", QuoteAsset: "USDT", PricePrecision: 3, QuantityPrecision: 1},
			{Symbol: "LTCUSDT", Pair: "LTCUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "LTC", QuoteAsset: "USDT", PricePrecision: 2, QuantityPrecision: 3},
			{Symbol: "ATOMUSDT", Pair: "ATOMUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "ATOM", QuoteAsset: "USDT", PricePrecision: 3, QuantityPrecision: 1},
			{Symbol: "UNIUSDT", Pair: "UNIUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "UNI", QuoteAsset: "USDT", PricePrecision: 3, QuantityPrecision: 0},
			{Symbol: "NEARUSDT", Pair: "NEARUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "NEAR", QuoteAsset: "USDT", PricePrecision: 3, QuantityPrecision: 0},
			// High volume memecoins and trending
			{Symbol: "SHIBUSDT", Pair: "SHIBUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "SHIB", QuoteAsset: "USDT", PricePrecision: 8, QuantityPrecision: 0},
			{Symbol: "PEPEUSDT", Pair: "PEPEUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "PEPE", QuoteAsset: "USDT", PricePrecision: 8, QuantityPrecision: 0},
			{Symbol: "WIFUSDT", Pair: "WIFUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "WIF", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 0},
			// Layer 2 and DeFi
			{Symbol: "ARBUSDT", Pair: "ARBUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "ARB", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 0},
			{Symbol: "OPUSDT", Pair: "OPUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "OP", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 0},
			{Symbol: "AAVEUSDT", Pair: "AAVEUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "AAVE", QuoteAsset: "USDT", PricePrecision: 2, QuantityPrecision: 1},
			{Symbol: "MKRUSDT", Pair: "MKRUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "MKR", QuoteAsset: "USDT", PricePrecision: 1, QuantityPrecision: 3},
			// Gaming and metaverse
			{Symbol: "SANDUSDT", Pair: "SANDUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "SAND", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 0},
			{Symbol: "MANAUSDT", Pair: "MANAUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "MANA", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 0},
			{Symbol: "AXSUSDT", Pair: "AXSUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "AXS", QuoteAsset: "USDT", PricePrecision: 2, QuantityPrecision: 1},
			// Infrastructure
			{Symbol: "FILUSDT", Pair: "FILUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "FIL", QuoteAsset: "USDT", PricePrecision: 3, QuantityPrecision: 1},
			{Symbol: "ICPUSDT", Pair: "ICPUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "ICP", QuoteAsset: "USDT", PricePrecision: 2, QuantityPrecision: 1},
			{Symbol: "APTUSDT", Pair: "APTUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "APT", QuoteAsset: "USDT", PricePrecision: 3, QuantityPrecision: 1},
			{Symbol: "SUIUSDT", Pair: "SUIUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "SUI", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 0},
			{Symbol: "SEIUSDT", Pair: "SEIUSDT", ContractType: "PERPETUAL", Status: "TRADING", BaseAsset: "SEI", QuoteAsset: "USDT", PricePrecision: 4, QuantityPrecision: 0},
		},
	}, nil
}

func (c *FuturesMockClient) GetFuturesSymbols() ([]string, error) {
	return []string{
		// Major
		"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
		// Popular altcoins
		"DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT", "MATICUSDT",
		// Additional popular
		"DOTUSDT", "LTCUSDT", "ATOMUSDT", "UNIUSDT", "NEARUSDT",
		// Memecoins
		"SHIBUSDT", "PEPEUSDT", "WIFUSDT",
		// Layer 2 & DeFi
		"ARBUSDT", "OPUSDT", "AAVEUSDT", "MKRUSDT",
		// Gaming
		"SANDUSDT", "MANAUSDT", "AXSUSDT",
		// Infrastructure
		"FILUSDT", "ICPUSDT", "APTUSDT", "SUIUSDT", "SEIUSDT",
	}, nil
}

// ==================== HISTORY ====================

func (c *FuturesMockClient) GetTradeHistory(symbol string, limit int) ([]FuturesTrade, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filtered := make([]FuturesTrade, 0)
	for _, trade := range c.trades {
		if symbol == "" || trade.Symbol == symbol {
			filtered = append(filtered, trade)
		}
	}

	// Return last N trades
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered, nil
}

func (c *FuturesMockClient) GetFundingFeeHistory(symbol string, limit int) ([]FundingFeeRecord, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filtered := make([]FundingFeeRecord, 0)
	for _, fee := range c.fundingFees {
		if symbol == "" || fee.Symbol == symbol {
			filtered = append(filtered, fee)
		}
	}

	// Return last N records
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered, nil
}

func (c *FuturesMockClient) GetAllOrders(symbol string, limit int) ([]FuturesOrder, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	orders := make([]FuturesOrder, 0)
	for _, order := range c.orders {
		if symbol == "" || order.Symbol == symbol {
			orders = append(orders, *order)
		}
	}

	// Return last N orders
	if len(orders) > limit {
		orders = orders[len(orders)-limit:]
	}

	return orders, nil
}

// ==================== WEBSOCKET ====================

func (c *FuturesMockClient) GetListenKey() (string, error) {
	return fmt.Sprintf("mock_listen_key_%d", time.Now().UnixNano()), nil
}

func (c *FuturesMockClient) KeepAliveListenKey(listenKey string) error {
	return nil
}

func (c *FuturesMockClient) CloseListenKey(listenKey string) error {
	return nil
}

// ==================== HELPERS ====================

func (c *FuturesMockClient) getLeverageLocked(symbol string) int {
	if lev, exists := c.leverage[symbol]; exists {
		return lev
	}
	return 10 // Default leverage
}

func (c *FuturesMockClient) getMarginTypeLocked(symbol string) MarginType {
	if mt, exists := c.marginType[symbol]; exists {
		return mt
	}
	return MarginTypeCrossed // Default margin type
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Ensure FuturesMockClient implements FuturesClient
var _ FuturesClient = (*FuturesMockClient)(nil)
