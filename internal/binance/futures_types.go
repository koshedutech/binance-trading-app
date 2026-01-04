package binance

import "time"

// ==================== ENUMS ====================

// MarginType represents the margin mode for futures trading
type MarginType string

const (
	MarginTypeCrossed  MarginType = "CROSSED"
	MarginTypeIsolated MarginType = "ISOLATED"
)

// PositionSide represents the position side for futures trading
type PositionSide string

const (
	PositionSideBoth  PositionSide = "BOTH"  // One-way mode
	PositionSideLong  PositionSide = "LONG"  // Hedge mode long
	PositionSideShort PositionSide = "SHORT" // Hedge mode short
)

// FuturesOrderType represents order types for futures
type FuturesOrderType string

const (
	FuturesOrderTypeLimit           FuturesOrderType = "LIMIT"
	FuturesOrderTypeMarket          FuturesOrderType = "MARKET"
	FuturesOrderTypeStop            FuturesOrderType = "STOP"
	FuturesOrderTypeStopMarket      FuturesOrderType = "STOP_MARKET"
	FuturesOrderTypeTakeProfit      FuturesOrderType = "TAKE_PROFIT"
	FuturesOrderTypeTakeProfitMarket FuturesOrderType = "TAKE_PROFIT_MARKET"
	FuturesOrderTypeTrailingStop    FuturesOrderType = "TRAILING_STOP_MARKET"
)

// TimeInForce represents order time-in-force options
type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "GTC" // Good Till Cancel
	TimeInForceIOC TimeInForce = "IOC" // Immediate or Cancel
	TimeInForceFOK TimeInForce = "FOK" // Fill or Kill
	TimeInForceGTX TimeInForce = "GTX" // Good Till Crossing (Post Only)
)

// FuturesOrderStatus represents order status
type FuturesOrderStatus string

const (
	FuturesOrderStatusNew             FuturesOrderStatus = "NEW"
	FuturesOrderStatusPartiallyFilled FuturesOrderStatus = "PARTIALLY_FILLED"
	FuturesOrderStatusFilled          FuturesOrderStatus = "FILLED"
	FuturesOrderStatusCanceled        FuturesOrderStatus = "CANCELED"
	FuturesOrderStatusExpired         FuturesOrderStatus = "EXPIRED"
)

// WorkingType for TP/SL orders
type WorkingType string

const (
	WorkingTypeContractPrice WorkingType = "CONTRACT_PRICE"
	WorkingTypeMarkPrice     WorkingType = "MARK_PRICE"
)

// ==================== ACCOUNT TYPES ====================

// FuturesAccountInfo represents futures account information
type FuturesAccountInfo struct {
	FeeTier                     int                     `json:"feeTier"`
	CanTrade                    bool                    `json:"canTrade"`
	CanDeposit                  bool                    `json:"canDeposit"`
	CanWithdraw                 bool                    `json:"canWithdraw"`
	UpdateTime                  int64                   `json:"updateTime"`
	TotalInitialMargin          float64                 `json:"totalInitialMargin,string"`
	TotalMaintMargin            float64                 `json:"totalMaintMargin,string"`
	TotalWalletBalance          float64                 `json:"totalWalletBalance,string"`
	TotalUnrealizedProfit       float64                 `json:"totalUnrealizedProfit,string"`
	TotalMarginBalance          float64                 `json:"totalMarginBalance,string"`
	TotalPositionInitialMargin  float64                 `json:"totalPositionInitialMargin,string"`
	TotalOpenOrderInitialMargin float64                 `json:"totalOpenOrderInitialMargin,string"`
	TotalCrossWalletBalance     float64                 `json:"totalCrossWalletBalance,string"`
	TotalCrossUnPnl             float64                 `json:"totalCrossUnPnl,string"`
	AvailableBalance            float64                 `json:"availableBalance,string"`
	MaxWithdrawAmount           float64                 `json:"maxWithdrawAmount,string"`
	Assets                      []FuturesAsset          `json:"assets"`
	Positions                   []FuturesAccountPosition `json:"positions"`
}

// FuturesAsset represents an asset in futures account
type FuturesAsset struct {
	Asset                  string  `json:"asset"`
	WalletBalance          float64 `json:"walletBalance,string"`
	UnrealizedProfit       float64 `json:"unrealizedProfit,string"`
	MarginBalance          float64 `json:"marginBalance,string"`
	MaintMargin            float64 `json:"maintMargin,string"`
	InitialMargin          float64 `json:"initialMargin,string"`
	PositionInitialMargin  float64 `json:"positionInitialMargin,string"`
	OpenOrderInitialMargin float64 `json:"openOrderInitialMargin,string"`
	CrossWalletBalance     float64 `json:"crossWalletBalance,string"`
	CrossUnPnl             float64 `json:"crossUnPnl,string"`
	AvailableBalance       float64 `json:"availableBalance,string"`
	MaxWithdrawAmount      float64 `json:"maxWithdrawAmount,string"`
	MarginAvailable        bool    `json:"marginAvailable"`
	UpdateTime             int64   `json:"updateTime"`
}

// FuturesAccountPosition represents a position in account info
type FuturesAccountPosition struct {
	Symbol                 string  `json:"symbol"`
	InitialMargin          float64 `json:"initialMargin,string"`
	MaintMargin            float64 `json:"maintMargin,string"`
	UnrealizedProfit       float64 `json:"unrealizedProfit,string"`
	PositionInitialMargin  float64 `json:"positionInitialMargin,string"`
	OpenOrderInitialMargin float64 `json:"openOrderInitialMargin,string"`
	Leverage               int     `json:"leverage,string"`
	Isolated               bool    `json:"isolated"`
	EntryPrice             float64 `json:"entryPrice,string"`
	MaxNotional            float64 `json:"maxNotional,string"`
	PositionSide           string  `json:"positionSide"`
	PositionAmt            float64 `json:"positionAmt,string"`
	Notional               float64 `json:"notional,string"`
	IsolatedWallet         float64 `json:"isolatedWallet,string"`
	UpdateTime             int64   `json:"updateTime"`
}

// ==================== POSITION TYPES ====================

// FuturesPosition represents a futures position from positionRisk endpoint
type FuturesPosition struct {
	Symbol           string  `json:"symbol"`
	PositionAmt      float64 `json:"positionAmt,string"`
	EntryPrice       float64 `json:"entryPrice,string"`
	MarkPrice        float64 `json:"markPrice,string"`
	UnrealizedProfit float64 `json:"unRealizedProfit,string"`
	LiquidationPrice float64 `json:"liquidationPrice,string"`
	Leverage         int     `json:"leverage,string"`
	MaxNotionalValue float64 `json:"maxNotionalValue,string"`
	MarginType       string  `json:"marginType"`
	IsolatedMargin   float64 `json:"isolatedMargin,string"`
	IsAutoAddMargin  bool    `json:"isAutoAddMargin,string"`
	PositionSide     string  `json:"positionSide"`
	Notional         float64 `json:"notional,string"`
	IsolatedWallet   float64 `json:"isolatedWallet,string"`
	UpdateTime       int64   `json:"updateTime"`
}

// ==================== ORDER TYPES ====================

// FuturesOrderParams represents parameters for placing a futures order
type FuturesOrderParams struct {
	Symbol           string           `json:"symbol"`
	Side             string           `json:"side"` // BUY or SELL
	PositionSide     PositionSide     `json:"positionSide"`
	Type             FuturesOrderType `json:"type"`
	Quantity         float64          `json:"quantity"`
	Price            float64          `json:"price,omitempty"`
	StopPrice        float64          `json:"stopPrice,omitempty"`
	TimeInForce      TimeInForce      `json:"timeInForce,omitempty"`
	ReduceOnly       bool             `json:"reduceOnly,omitempty"`
	ClosePosition    bool             `json:"closePosition,omitempty"`
	WorkingType      WorkingType      `json:"workingType,omitempty"`
	PriceProtect     bool             `json:"priceProtect,omitempty"`
	NewClientOrderId string           `json:"newClientOrderId,omitempty"`
}

// FuturesOrder represents a futures order
type FuturesOrder struct {
	OrderId          int64   `json:"orderId"`
	Symbol           string  `json:"symbol"`
	Status           string  `json:"status"`
	ClientOrderId    string  `json:"clientOrderId"`
	Price            float64 `json:"price,string"`
	AvgPrice         float64 `json:"avgPrice,string"`
	OrigQty          float64 `json:"origQty,string"`
	ExecutedQty      float64 `json:"executedQty,string"`
	CumQuote         float64 `json:"cumQuote,string"`
	TimeInForce      string  `json:"timeInForce"`
	Type             string  `json:"type"`
	ReduceOnly       bool    `json:"reduceOnly"`
	ClosePosition    bool    `json:"closePosition"`
	Side             string  `json:"side"`
	PositionSide     string  `json:"positionSide"`
	StopPrice        float64 `json:"stopPrice,string"`
	WorkingType      string  `json:"workingType"`
	PriceProtect     bool    `json:"priceProtect"`
	OrigType         string  `json:"origType"`
	Time             int64   `json:"time"`
	UpdateTime       int64   `json:"updateTime"`
}

// FuturesOrderResponse represents response from placing an order
type FuturesOrderResponse struct {
	OrderId          int64   `json:"orderId"`
	Symbol           string  `json:"symbol"`
	Status           string  `json:"status"`
	ClientOrderId    string  `json:"clientOrderId"`
	Price            float64 `json:"price,string"`
	AvgPrice         float64 `json:"avgPrice,string"`
	OrigQty          float64 `json:"origQty,string"`
	ExecutedQty      float64 `json:"executedQty,string"`
	CumQty           float64 `json:"cumQty,string"`
	CumQuote         float64 `json:"cumQuote,string"`
	TimeInForce      string  `json:"timeInForce"`
	Type             string  `json:"type"`
	ReduceOnly       bool    `json:"reduceOnly"`
	ClosePosition    bool    `json:"closePosition"`
	Side             string  `json:"side"`
	PositionSide     string  `json:"positionSide"`
	StopPrice        float64 `json:"stopPrice,string"`
	WorkingType      string  `json:"workingType"`
	PriceProtect     bool    `json:"priceProtect"`
	OrigType         string  `json:"origType"`
	UpdateTime       int64   `json:"updateTime"`
}

// ==================== MARKET DATA TYPES ====================

// FundingRate represents funding rate data
type FundingRate struct {
	Symbol          string  `json:"symbol"`
	FundingRate     float64 `json:"fundingRate,string"`
	FundingTime     int64   `json:"fundingTime"`
	NextFundingTime int64   `json:"nextFundingTime,omitempty"`
	MarkPrice       float64 `json:"markPrice,string"`
}

// MarkPrice represents mark price data
type MarkPrice struct {
	Symbol               string  `json:"symbol"`
	MarkPrice            float64 `json:"markPrice,string"`
	IndexPrice           float64 `json:"indexPrice,string"`
	EstimatedSettlePrice float64 `json:"estimatedSettlePrice,string"`
	LastFundingRate      float64 `json:"lastFundingRate,string"`
	NextFundingTime      int64   `json:"nextFundingTime"`
	InterestRate         float64 `json:"interestRate,string"`
	Time                 int64   `json:"time"`
}

// OrderBookDepth represents order book data
type OrderBookDepth struct {
	LastUpdateId int64           `json:"lastUpdateId"`
	EventTime    int64           `json:"E"`
	TransactTime int64           `json:"T"`
	Bids         [][]string      `json:"bids"` // [price, qty]
	Asks         [][]string      `json:"asks"` // [price, qty]
}

// OrderBookEntry represents a single order book entry
type OrderBookEntry struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

// Futures24hrTicker represents 24 hour price change statistics for a futures symbol
type Futures24hrTicker struct {
	Symbol             string  `json:"symbol"`
	PriceChange        float64 `json:"priceChange,string"`
	PriceChangePercent float64 `json:"priceChangePercent,string"`
	WeightedAvgPrice   float64 `json:"weightedAvgPrice,string"`
	LastPrice          float64 `json:"lastPrice,string"`
	LastQty            float64 `json:"lastQty,string"`
	OpenPrice          float64 `json:"openPrice,string"`
	HighPrice          float64 `json:"highPrice,string"`
	LowPrice           float64 `json:"lowPrice,string"`
	Volume             float64 `json:"volume,string"`
	QuoteVolume        float64 `json:"quoteVolume,string"`
	OpenTime           int64   `json:"openTime"`
	CloseTime          int64   `json:"closeTime"`
	FirstId            int64   `json:"firstId"`
	LastId             int64   `json:"lastId"`
	Count              int64   `json:"count"`
}

// ==================== HISTORY TYPES ====================

// FuturesTrade represents a futures trade from history
type FuturesTrade struct {
	ID              int64   `json:"id"`
	Symbol          string  `json:"symbol"`
	OrderId         int64   `json:"orderId"`
	Side            string  `json:"side"`
	Price           float64 `json:"price,string"`
	Qty             float64 `json:"qty,string"`
	RealizedPnl     float64 `json:"realizedPnl,string"`
	MarginAsset     string  `json:"marginAsset"`
	QuoteQty        float64 `json:"quoteQty,string"`
	Commission      float64 `json:"commission,string"`
	CommissionAsset string  `json:"commissionAsset"`
	Time            int64   `json:"time"`
	PositionSide    string  `json:"positionSide"`
	Buyer           bool    `json:"buyer"`
	Maker           bool    `json:"maker"`
}

// FundingFeeRecord represents a funding fee payment
type FundingFeeRecord struct {
	Symbol      string    `json:"symbol"`
	IncomeType  string    `json:"incomeType"`
	Income      float64   `json:"income,string"`
	Asset       string    `json:"asset"`
	Info        string    `json:"info"`
	Time        int64     `json:"time"`
	TranId      int64     `json:"tranId"`
	TradeId     string    `json:"tradeId"`
	// Derived fields
	Timestamp   time.Time `json:"-"`
}

// IncomeRecord represents an income entry from the /fapi/v1/income endpoint
// Used for fetching realized PnL, funding fees, commissions, etc.
type IncomeRecord struct {
	Symbol     string    `json:"symbol"`
	IncomeType string    `json:"incomeType"` // REALIZED_PNL, FUNDING_FEE, COMMISSION, etc.
	Income     float64   `json:"income,string"`
	Asset      string    `json:"asset"`
	Info       string    `json:"info"`
	Time       int64     `json:"time"`
	TranId     int64     `json:"tranId"`
	TradeId    string    `json:"tradeId"`
	// Derived fields
	Timestamp  time.Time `json:"-"`
}

// ==================== LEVERAGE & SETTINGS TYPES ====================

// LeverageResponse represents response from setting leverage
type LeverageResponse struct {
	Leverage         int     `json:"leverage"`
	MaxNotionalValue float64 `json:"maxNotionalValue,string"`
	Symbol           string  `json:"symbol"`
}

// MarginTypeResponse represents response from setting margin type
type MarginTypeResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// PositionModeResponse represents response from getting position mode
type PositionModeResponse struct {
	DualSidePosition bool `json:"dualSidePosition"`
}

// ==================== SYMBOL INFO TYPES ====================

// FuturesSymbolFilter represents a filter from the symbol's filters array
type FuturesSymbolFilter struct {
	FilterType string  `json:"filterType"`
	MinPrice   string  `json:"minPrice,omitempty"`
	MaxPrice   string  `json:"maxPrice,omitempty"`
	TickSize   string  `json:"tickSize,omitempty"`
	MinQty     string  `json:"minQty,omitempty"`
	MaxQty     string  `json:"maxQty,omitempty"`
	StepSize   string  `json:"stepSize,omitempty"`
	Notional   string  `json:"notional,omitempty"`
}

// FuturesSymbolInfo represents futures symbol information
type FuturesSymbolInfo struct {
	Symbol                string                `json:"symbol"`
	Pair                  string                `json:"pair"`
	ContractType          string                `json:"contractType"`
	DeliveryDate          int64                 `json:"deliveryDate"`
	OnboardDate           int64                 `json:"onboardDate"`
	Status                string                `json:"status"`
	MaintMarginPercent    float64               `json:"maintMarginPercent,string"`
	RequiredMarginPercent float64               `json:"requiredMarginPercent,string"`
	BaseAsset             string                `json:"baseAsset"`
	QuoteAsset            string                `json:"quoteAsset"`
	MarginAsset           string                `json:"marginAsset"`
	PricePrecision        int                   `json:"pricePrecision"`
	QuantityPrecision     int                   `json:"quantityPrecision"`
	BaseAssetPrecision    int                   `json:"baseAssetPrecision"`
	QuotePrecision        int                   `json:"quotePrecision"`
	UnderlyingType        string                `json:"underlyingType"`
	UnderlyingSubType     []string              `json:"underlyingSubType"`
	SettlePlan            int                   `json:"settlePlan"`
	TriggerProtect        float64               `json:"triggerProtect,string"`
	OrderTypes            []string              `json:"orderTypes"`
	TimeInForce           []string              `json:"timeInForce"`
	Filters               []FuturesSymbolFilter `json:"filters"`
}

// FuturesExchangeInfo represents futures exchange information
type FuturesExchangeInfo struct {
	ExchangeFilters []interface{}       `json:"exchangeFilters"`
	RateLimits      []interface{}       `json:"rateLimits"`
	ServerTime      int64               `json:"serverTime"`
	Assets          []FuturesAssetInfo  `json:"assets"`
	Symbols         []FuturesSymbolInfo `json:"symbols"`
	Timezone        string              `json:"timezone"`
}

// FuturesAssetInfo represents asset info in exchange info
type FuturesAssetInfo struct {
	Asset             string  `json:"asset"`
	MarginAvailable   bool    `json:"marginAvailable"`
	AutoAssetExchange float64 `json:"autoAssetExchange,string"`
}

// ==================== LISTEN KEY ====================

// ListenKeyResponse represents response from listen key endpoints
type ListenKeyResponse struct {
	ListenKey string `json:"listenKey"`
}

// ==================== ALGO ORDER TYPES (NEW API as of 2025-12-09) ====================

// AlgoType for algo orders
type AlgoType string

const (
	AlgoTypeConditional AlgoType = "CONDITIONAL"
)

// AlgoOrderStatus represents algo order status
type AlgoOrderStatus string

const (
	AlgoOrderStatusNew       AlgoOrderStatus = "NEW"
	AlgoOrderStatusTriggered AlgoOrderStatus = "TRIGGERED"
	AlgoOrderStatusCancelled AlgoOrderStatus = "CANCELLED"
	AlgoOrderStatusExpired   AlgoOrderStatus = "EXPIRED"
)

// AlgoOrderParams represents parameters for placing an algo order
// Used for conditional orders: STOP_MARKET, TAKE_PROFIT_MARKET, STOP, TAKE_PROFIT, TRAILING_STOP_MARKET
type AlgoOrderParams struct {
	Symbol        string           `json:"symbol"`
	Side          string           `json:"side"` // BUY or SELL
	PositionSide  PositionSide     `json:"positionSide,omitempty"`
	Type          FuturesOrderType `json:"type"` // STOP_MARKET, TAKE_PROFIT_MARKET, etc.
	Quantity      float64          `json:"quantity,omitempty"`
	Price         float64          `json:"price,omitempty"`         // For STOP and TAKE_PROFIT limit orders
	TriggerPrice  float64          `json:"triggerPrice"`            // Required - the activation price
	TimeInForce   TimeInForce      `json:"timeInForce,omitempty"`
	WorkingType   WorkingType      `json:"workingType,omitempty"`   // MARK_PRICE or CONTRACT_PRICE
	ClosePosition bool             `json:"closePosition,omitempty"` // Close all position
	ReduceOnly    bool             `json:"reduceOnly,omitempty"`
	PriceProtect  bool             `json:"priceProtect,omitempty"`
	ClientAlgoId  string           `json:"clientAlgoId,omitempty"`
	// Trailing stop specific
	ActivatePrice float64 `json:"activatePrice,omitempty"` // For TRAILING_STOP_MARKET
	CallbackRate  float64 `json:"callbackRate,omitempty"`  // 0.1-10% for trailing stop
}

// AlgoOrderResponse represents response from placing an algo order
type AlgoOrderResponse struct {
	AlgoId        int64   `json:"algoId"`
	ClientAlgoId  string  `json:"clientAlgoId"`
	AlgoType      string  `json:"algoType"`
	OrderType     string  `json:"orderType"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	PositionSide  string  `json:"positionSide"`
	AlgoStatus    string  `json:"algoStatus"`
	TriggerPrice  float64 `json:"triggerPrice,string"`
	Price         float64 `json:"price,string"`
	Quantity      float64 `json:"quantity,string"`
	WorkingType   string  `json:"workingType"`
	ClosePosition bool    `json:"closePosition"`
	ReduceOnly    bool    `json:"reduceOnly"`
	PriceProtect  bool    `json:"priceProtect"`
	ActivatePrice float64 `json:"activatePrice,string"`
	CallbackRate  float64 `json:"callbackRate,string"`
	CreateTime    int64   `json:"createTime"`
	UpdateTime    int64   `json:"updateTime"`
}

// AlgoOrder represents an open or historical algo order
type AlgoOrder struct {
	AlgoId        int64   `json:"algoId"`
	ClientAlgoId  string  `json:"clientAlgoId"`
	AlgoType      string  `json:"algoType"`
	OrderType     string  `json:"orderType"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	PositionSide  string  `json:"positionSide"`
	AlgoStatus    string  `json:"algoStatus"`
	TriggerPrice  float64 `json:"triggerPrice,string"`
	Price         float64 `json:"price,string"`
	Quantity      float64 `json:"quantity,string"`
	ExecutedQty   float64 `json:"executedQty,string"`
	WorkingType   string  `json:"workingType"`
	ClosePosition bool    `json:"closePosition"`
	ReduceOnly    bool    `json:"reduceOnly"`
	PriceProtect  bool    `json:"priceProtect"`
	ActivatePrice float64 `json:"activatePrice,string"`
	CallbackRate  float64 `json:"callbackRate,string"`
	CreateTime    int64   `json:"createTime"`
	UpdateTime    int64   `json:"updateTime"`
	TriggerTime   int64   `json:"triggerTime"`
}
