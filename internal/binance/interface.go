package binance

// BinanceClient defines the interface for Binance API operations
type BinanceClient interface {
	GetKlines(symbol, interval string, limit int) ([]Kline, error)
	Get24hrTickers() ([]Ticker24hr, error)
	GetCurrentPrice(symbol string) (float64, error)
	GetExchangeInfo() (*ExchangeInfo, error)
	GetAllSymbols() ([]string, error)
	PlaceOrder(params map[string]string) (*OrderResponse, error)
	CancelOrder(symbol string, orderId int64) error
	GetAccountInfo() (*AccountInfo, error)
}

// AccountInfo represents spot account information
type AccountInfo struct {
	MakerCommission  int             `json:"makerCommission"`
	TakerCommission  int             `json:"takerCommission"`
	BuyerCommission  int             `json:"buyerCommission"`
	SellerCommission int             `json:"sellerCommission"`
	CanTrade         bool            `json:"canTrade"`
	CanWithdraw      bool            `json:"canWithdraw"`
	CanDeposit       bool            `json:"canDeposit"`
	UpdateTime       int64           `json:"updateTime"`
	AccountType      string          `json:"accountType"`
	Balances         []AssetBalance  `json:"balances"`
}

// AssetBalance represents a single asset balance
type AssetBalance struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

// Ensure both Client and MockClient implement BinanceClient
var _ BinanceClient = (*Client)(nil)
var _ BinanceClient = (*MockClient)(nil)
