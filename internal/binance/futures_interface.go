package binance

// FuturesClient defines the interface for Binance Futures API operations
type FuturesClient interface {
	// ==================== ACCOUNT ====================

	// GetFuturesAccountInfo retrieves futures account information including balances and positions
	GetFuturesAccountInfo() (*FuturesAccountInfo, error)

	// GetPositions retrieves all futures positions
	GetPositions() ([]FuturesPosition, error)

	// GetPositionBySymbol retrieves position for a specific symbol
	GetPositionBySymbol(symbol string) (*FuturesPosition, error)

	// GetCommissionRate retrieves user's actual maker/taker fee rates from Binance
	GetCommissionRate(symbol string) (*CommissionRate, error)

	// ==================== LEVERAGE & MARGIN ====================

	// SetLeverage sets the leverage for a symbol (1-125x)
	SetLeverage(symbol string, leverage int) (*LeverageResponse, error)

	// SetMarginType sets the margin type (ISOLATED or CROSSED)
	SetMarginType(symbol string, marginType MarginType) error

	// SetPositionMode sets the position mode (true for Hedge mode, false for One-way mode)
	SetPositionMode(dualSidePosition bool) error

	// GetPositionMode retrieves the current position mode
	GetPositionMode() (*PositionModeResponse, error)

	// ==================== TRADING ====================

	// PlaceFuturesOrder places a new futures order
	PlaceFuturesOrder(params FuturesOrderParams) (*FuturesOrderResponse, error)

	// CancelFuturesOrder cancels an existing futures order
	CancelFuturesOrder(symbol string, orderId int64) error

	// CancelAllFuturesOrders cancels all open orders for a symbol
	CancelAllFuturesOrders(symbol string) error

	// GetOpenOrders retrieves all open orders for a symbol (empty string for all symbols)
	GetOpenOrders(symbol string) ([]FuturesOrder, error)

	// GetOrder retrieves a specific order
	GetOrder(symbol string, orderId int64) (*FuturesOrder, error)

	// ==================== ALGO ORDERS (NEW API as of 2025-12-09) ====================

	// PlaceAlgoOrder places a conditional order (STOP_MARKET, TAKE_PROFIT_MARKET, etc.)
	// Required since Binance migrated conditional orders to Algo Service on 2025-12-09
	PlaceAlgoOrder(params AlgoOrderParams) (*AlgoOrderResponse, error)

	// GetOpenAlgoOrders retrieves all open algo orders
	GetOpenAlgoOrders(symbol string) ([]AlgoOrder, error)

	// CancelAlgoOrder cancels an algo order
	CancelAlgoOrder(symbol string, algoId int64) error

	// CancelAllAlgoOrders cancels all open algo orders for a symbol
	CancelAllAlgoOrders(symbol string) error

	// GetAllAlgoOrders retrieves all algo orders (historical, including filled/cancelled)
	GetAllAlgoOrders(symbol string, limit int) ([]AlgoOrder, error)

	// ==================== MARKET DATA ====================

	// GetFundingRate retrieves the current funding rate for a symbol
	GetFundingRate(symbol string) (*FundingRate, error)

	// GetFundingRateHistory retrieves funding rate history
	GetFundingRateHistory(symbol string, limit int) ([]FundingRate, error)

	// GetMarkPrice retrieves the mark price for a symbol
	GetMarkPrice(symbol string) (*MarkPrice, error)

	// GetAllMarkPrices retrieves mark prices for all symbols
	GetAllMarkPrices() ([]MarkPrice, error)

	// GetOrderBookDepth retrieves the order book depth
	GetOrderBookDepth(symbol string, limit int) (*OrderBookDepth, error)

	// GetFuturesKlines retrieves candlestick data for futures
	GetFuturesKlines(symbol, interval string, limit int) ([]Kline, error)

	// Get24hrTicker retrieves 24 hour price change statistics for a symbol
	Get24hrTicker(symbol string) (*Futures24hrTicker, error)

	// GetAll24hrTickers retrieves 24 hour price change statistics for all symbols
	GetAll24hrTickers() ([]Futures24hrTicker, error)

	// GetCurrentPrice retrieves the current price for a symbol
	GetFuturesCurrentPrice(symbol string) (float64, error)

	// ==================== EXCHANGE INFO ====================

	// GetFuturesExchangeInfo retrieves futures exchange information
	GetFuturesExchangeInfo() (*FuturesExchangeInfo, error)

	// GetFuturesSymbols retrieves all available futures trading pairs
	GetFuturesSymbols() ([]string, error)

	// ==================== HISTORY ====================

	// GetTradeHistory retrieves trade history for a symbol
	GetTradeHistory(symbol string, limit int) ([]FuturesTrade, error)

	// GetTradeHistoryByDateRange retrieves trade history for a specific symbol and date range.
	// Note: Binance API requires symbol parameter; behavior with empty string is undefined.
	// startTime/endTime in milliseconds, 0 to ignore
	GetTradeHistoryByDateRange(symbol string, startTime, endTime int64, limit int) ([]FuturesTrade, error)

	// GetFundingFeeHistory retrieves funding fee payment history
	GetFundingFeeHistory(symbol string, limit int) ([]FundingFeeRecord, error)

	// GetAllOrders retrieves all orders (filled, canceled, etc.) for a symbol
	GetAllOrders(symbol string, limit int) ([]FuturesOrder, error)

	// GetAllOrdersByDateRange retrieves all orders for a date range
	// startTime/endTime in milliseconds, 0 to ignore
	GetAllOrdersByDateRange(symbol string, startTime, endTime int64, limit int) ([]FuturesOrder, error)

	// GetIncomeHistory retrieves income history (realized PnL, funding fees, commissions, etc.)
	// incomeType can be: REALIZED_PNL, FUNDING_FEE, COMMISSION, etc. Empty string for all types.
	// startTime/endTime in milliseconds, 0 to ignore
	GetIncomeHistory(incomeType string, startTime, endTime int64, limit int) ([]IncomeRecord, error)

	// ==================== WEBSOCKET ====================

	// GetListenKey creates a new user data stream listen key
	GetListenKey() (string, error)

	// KeepAliveListenKey extends the validity of a listen key
	KeepAliveListenKey(listenKey string) error

	// CloseListenKey closes a user data stream
	CloseListenKey(listenKey string) error
}

// Ensure both FuturesClient implementations will satisfy the interface
// (compile-time checks added when implementations are created)
