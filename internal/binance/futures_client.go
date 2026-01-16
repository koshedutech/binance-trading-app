package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Retry configuration for API calls
const (
	maxRetries     = 3
	baseRetryDelay = 500 * time.Millisecond
	maxRetryDelay  = 5 * time.Second
)

const (
	// FuturesBaseURL is the production Binance Futures API URL
	FuturesBaseURL = "https://fapi.binance.com"
	// FuturesTestnetURL is the testnet Binance Futures API URL
	FuturesTestnetURL = "https://testnet.binancefuture.com"
)

// FuturesClientImpl implements the FuturesClient interface
type FuturesClientImpl struct {
	apiKey     string
	secretKey  string
	baseURL    string
	httpClient *http.Client
}

// NewFuturesClient creates a new FuturesClient instance
func NewFuturesClient(apiKey, secretKey string, testnet bool) *FuturesClientImpl {
	baseURL := FuturesBaseURL
	if testnet {
		baseURL = FuturesTestnetURL
	}

	// Trim any whitespace from keys - critical for signature generation
	return &FuturesClientImpl{
		apiKey:     strings.TrimSpace(apiKey),
		secretKey:  strings.TrimSpace(secretKey),
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// ==================== ACCOUNT ====================

// GetFuturesAccountInfo retrieves futures account information
func (c *FuturesClientImpl) GetFuturesAccountInfo() (*FuturesAccountInfo, error) {
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	resp, err := c.signedGet("/fapi/v2/account", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching account info: %w", err)
	}

	var accountInfo FuturesAccountInfo
	if err := json.Unmarshal(resp, &accountInfo); err != nil {
		return nil, fmt.Errorf("error parsing account info: %w", err)
	}

	return &accountInfo, nil
}

// GetUSDTBalance fetches the USDT balance from futures account
func (c *FuturesClientImpl) GetUSDTBalance() (float64, error) {
	accountInfo, err := c.GetFuturesAccountInfo()
	if err != nil {
		return 0, fmt.Errorf("failed to get account info: %w", err)
	}

	// Find USDT in assets array
	for _, asset := range accountInfo.Assets {
		if asset.Asset == "USDT" {
			return asset.WalletBalance, nil
		}
	}

	// No USDT balance found
	return 0, nil
}

// GetPositions retrieves all futures positions
func (c *FuturesClientImpl) GetPositions() ([]FuturesPosition, error) {
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	resp, err := c.signedGet("/fapi/v2/positionRisk", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching positions: %w", err)
	}

	var positions []FuturesPosition
	if err := json.Unmarshal(resp, &positions); err != nil {
		return nil, fmt.Errorf("error parsing positions: %w", err)
	}

	return positions, nil
}

// GetPositionBySymbol retrieves position for a specific symbol
func (c *FuturesClientImpl) GetPositionBySymbol(symbol string) (*FuturesPosition, error) {
	params := map[string]string{
		"symbol":    symbol,
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	resp, err := c.signedGet("/fapi/v2/positionRisk", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching position: %w", err)
	}

	var positions []FuturesPosition
	if err := json.Unmarshal(resp, &positions); err != nil {
		return nil, fmt.Errorf("error parsing position: %w", err)
	}

	if len(positions) == 0 {
		return nil, fmt.Errorf("position not found for symbol: %s", symbol)
	}

	// In hedge mode, there are two positions (LONG and SHORT)
	// Return the one with non-zero position amount
	for i := range positions {
		if positions[i].PositionAmt != 0 {
			return &positions[i], nil
		}
	}

	// If no position has non-zero amount, return the first one
	return &positions[0], nil
}

// ==================== LEVERAGE & MARGIN ====================

// SetLeverage sets the leverage for a symbol
func (c *FuturesClientImpl) SetLeverage(symbol string, leverage int) (*LeverageResponse, error) {
	params := map[string]string{
		"symbol":    symbol,
		"leverage":  strconv.Itoa(leverage),
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	resp, err := c.signedPost("/fapi/v1/leverage", params)
	if err != nil {
		return nil, fmt.Errorf("error setting leverage: %w", err)
	}

	var leverageResp LeverageResponse
	if err := json.Unmarshal(resp, &leverageResp); err != nil {
		return nil, fmt.Errorf("error parsing leverage response: %w", err)
	}

	return &leverageResp, nil
}

// SetMarginType sets the margin type (ISOLATED or CROSSED)
func (c *FuturesClientImpl) SetMarginType(symbol string, marginType MarginType) error {
	params := map[string]string{
		"symbol":     symbol,
		"marginType": string(marginType),
		"timestamp":  strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	_, err := c.signedPost("/fapi/v1/marginType", params)
	if err != nil {
		// Binance returns error if margin type is already set - ignore this
		return nil
	}

	return nil
}

// SetPositionMode sets the position mode (Hedge or One-way)
func (c *FuturesClientImpl) SetPositionMode(dualSidePosition bool) error {
	params := map[string]string{
		"dualSidePosition": strconv.FormatBool(dualSidePosition),
		"timestamp":        strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	_, err := c.signedPost("/fapi/v1/positionSide/dual", params)
	if err != nil {
		// Binance returns error if position mode is already set - ignore this
		return nil
	}

	return nil
}

// GetPositionMode retrieves the current position mode
func (c *FuturesClientImpl) GetPositionMode() (*PositionModeResponse, error) {
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	resp, err := c.signedGet("/fapi/v1/positionSide/dual", params)
	if err != nil {
		return nil, fmt.Errorf("error getting position mode: %w", err)
	}

	var modeResp PositionModeResponse
	if err := json.Unmarshal(resp, &modeResp); err != nil {
		return nil, fmt.Errorf("error parsing position mode: %w", err)
	}

	return &modeResp, nil
}

// ==================== TRADING ====================

// PlaceFuturesOrder places a new futures order
func (c *FuturesClientImpl) PlaceFuturesOrder(params FuturesOrderParams) (*FuturesOrderResponse, error) {
	reqParams := map[string]string{
		"symbol":    params.Symbol,
		"side":      params.Side,
		"type":      string(params.Type),
		"quantity":  strconv.FormatFloat(params.Quantity, 'f', -1, 64),
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	// Add position side if not empty
	if params.PositionSide != "" {
		reqParams["positionSide"] = string(params.PositionSide)
	}

	// Add price for limit orders
	if params.Price > 0 {
		reqParams["price"] = strconv.FormatFloat(params.Price, 'f', -1, 64)
	}

	// Add stop price for stop orders
	if params.StopPrice > 0 {
		reqParams["stopPrice"] = strconv.FormatFloat(params.StopPrice, 'f', -1, 64)
	}

	// Add time in force
	if params.TimeInForce != "" {
		reqParams["timeInForce"] = string(params.TimeInForce)
	} else if params.Type == FuturesOrderTypeLimit {
		reqParams["timeInForce"] = string(TimeInForceGTC)
	}

	// Add reduce only
	if params.ReduceOnly {
		reqParams["reduceOnly"] = "true"
	}

	// Add close position
	if params.ClosePosition {
		reqParams["closePosition"] = "true"
	}

	// Add working type
	if params.WorkingType != "" {
		reqParams["workingType"] = string(params.WorkingType)
	}

	// Add price protect
	if params.PriceProtect {
		reqParams["priceProtect"] = "true"
	}

	// Add client order id
	if params.NewClientOrderId != "" {
		reqParams["newClientOrderId"] = params.NewClientOrderId
	}

	// Signature is added by signParams() in signed* methods

	resp, err := c.signedPost("/fapi/v1/order", reqParams)
	if err != nil {
		return nil, fmt.Errorf("error placing order: %w", err)
	}

	var orderResp FuturesOrderResponse
	if err := json.Unmarshal(resp, &orderResp); err != nil {
		return nil, fmt.Errorf("error parsing order response: %w", err)
	}

	return &orderResp, nil
}

// CancelFuturesOrder cancels an existing futures order
func (c *FuturesClientImpl) CancelFuturesOrder(symbol string, orderId int64) error {
	params := map[string]string{
		"symbol":    symbol,
		"orderId":   strconv.FormatInt(orderId, 10),
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	_, err := c.signedDelete("/fapi/v1/order", params)
	if err != nil {
		return fmt.Errorf("error canceling order: %w", err)
	}

	return nil
}

// CancelAllFuturesOrders cancels all open orders for a symbol
func (c *FuturesClientImpl) CancelAllFuturesOrders(symbol string) error {
	params := map[string]string{
		"symbol":    symbol,
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	_, err := c.signedDelete("/fapi/v1/allOpenOrders", params)
	if err != nil {
		return fmt.Errorf("error canceling all orders: %w", err)
	}

	return nil
}

// GetOpenOrders retrieves all open orders for a symbol
func (c *FuturesClientImpl) GetOpenOrders(symbol string) ([]FuturesOrder, error) {
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	if symbol != "" {
		params["symbol"] = symbol
	}

	// Signature is added by signParams() in signed* methods

	resp, err := c.signedGet("/fapi/v1/openOrders", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching open orders: %w", err)
	}

	var orders []FuturesOrder
	if err := json.Unmarshal(resp, &orders); err != nil {
		return nil, fmt.Errorf("error parsing open orders: %w", err)
	}

	return orders, nil
}

// GetOrder retrieves a specific order
func (c *FuturesClientImpl) GetOrder(symbol string, orderId int64) (*FuturesOrder, error) {
	params := map[string]string{
		"symbol":    symbol,
		"orderId":   strconv.FormatInt(orderId, 10),
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	resp, err := c.signedGet("/fapi/v1/order", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching order: %w", err)
	}

	var order FuturesOrder
	if err := json.Unmarshal(resp, &order); err != nil {
		return nil, fmt.Errorf("error parsing order: %w", err)
	}

	return &order, nil
}

// ==================== ALGO ORDERS (NEW API as of 2025-12-09) ====================

// PlaceAlgoOrder places a new algo order (conditional order)
// This is required for STOP_MARKET, TAKE_PROFIT_MARKET, STOP, TAKE_PROFIT, TRAILING_STOP_MARKET
// as of Binance API change on 2025-12-09
func (c *FuturesClientImpl) PlaceAlgoOrder(params AlgoOrderParams) (*AlgoOrderResponse, error) {
	reqParams := map[string]string{
		"algoType":  string(AlgoTypeConditional),
		"symbol":    params.Symbol,
		"side":      params.Side,
		"type":      string(params.Type),
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	// Add trigger price (required for conditional orders)
	if params.TriggerPrice > 0 {
		reqParams["triggerPrice"] = strconv.FormatFloat(params.TriggerPrice, 'f', -1, 64)
	}

	// Add position side if specified
	if params.PositionSide != "" {
		reqParams["positionSide"] = string(params.PositionSide)
	}

	// Add quantity (not used with closePosition=true)
	if params.Quantity > 0 && !params.ClosePosition {
		reqParams["quantity"] = strconv.FormatFloat(params.Quantity, 'f', -1, 64)
	}

	// Add price for limit conditional orders (STOP, TAKE_PROFIT)
	if params.Price > 0 {
		reqParams["price"] = strconv.FormatFloat(params.Price, 'f', -1, 64)
	}

	// Add time in force
	if params.TimeInForce != "" {
		reqParams["timeInForce"] = string(params.TimeInForce)
	}

	// Add working type (MARK_PRICE or CONTRACT_PRICE)
	if params.WorkingType != "" {
		reqParams["workingType"] = string(params.WorkingType)
	}

	// Add close position flag
	if params.ClosePosition {
		reqParams["closePosition"] = "true"
	}

	// Add reduce only (cannot be used with closePosition)
	if params.ReduceOnly && !params.ClosePosition {
		reqParams["reduceOnly"] = "true"
	}

	// Add price protect
	if params.PriceProtect {
		reqParams["priceProtect"] = "true"
	}

	// Add client algo id
	if params.ClientAlgoId != "" {
		reqParams["clientAlgoId"] = params.ClientAlgoId
	}

	// Trailing stop specific parameters
	if params.ActivatePrice > 0 {
		reqParams["activatePrice"] = strconv.FormatFloat(params.ActivatePrice, 'f', -1, 64)
	}
	if params.CallbackRate > 0 {
		reqParams["callbackRate"] = strconv.FormatFloat(params.CallbackRate, 'f', -1, 64)
	}

	resp, err := c.signedPost("/fapi/v1/algoOrder", reqParams)
	if err != nil {
		return nil, fmt.Errorf("error placing algo order: %w", err)
	}

	var algoResp AlgoOrderResponse
	if err := json.Unmarshal(resp, &algoResp); err != nil {
		return nil, fmt.Errorf("error parsing algo order response: %w", err)
	}

	return &algoResp, nil
}

// GetOpenAlgoOrders retrieves all open algo orders
func (c *FuturesClientImpl) GetOpenAlgoOrders(symbol string) ([]AlgoOrder, error) {
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	if symbol != "" {
		params["symbol"] = symbol
	}

	resp, err := c.signedGet("/fapi/v1/openAlgoOrders", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching open algo orders: %w", err)
	}

	// The Binance API returns an array directly
	var orders []AlgoOrder
	if err := json.Unmarshal(resp, &orders); err != nil {
		return nil, fmt.Errorf("error parsing open algo orders: %w (response: %s)", err, string(resp))
	}

	return orders, nil
}

// CancelAlgoOrder cancels an algo order
func (c *FuturesClientImpl) CancelAlgoOrder(symbol string, algoId int64) error {
	params := map[string]string{
		"symbol":    symbol,
		"algoId":    strconv.FormatInt(algoId, 10),
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	_, err := c.signedDelete("/fapi/v1/algoOrder", params)
	if err != nil {
		return fmt.Errorf("error canceling algo order: %w", err)
	}

	return nil
}

// CancelAllAlgoOrders cancels all open algo orders for a symbol
func (c *FuturesClientImpl) CancelAllAlgoOrders(symbol string) error {
	params := map[string]string{
		"symbol":    symbol,
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	_, err := c.signedDelete("/fapi/v1/algoOpenOrders", params)
	if err != nil {
		return fmt.Errorf("error canceling all algo orders: %w", err)
	}

	return nil
}

// GetAllAlgoOrders retrieves all algo orders (historical, including filled/cancelled)
func (c *FuturesClientImpl) GetAllAlgoOrders(symbol string, limit int) ([]AlgoOrder, error) {
	params := map[string]string{
		"symbol":    symbol,
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}

	resp, err := c.signedGet("/fapi/v1/allAlgoOrders", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching all algo orders: %w", err)
	}

	var orders []AlgoOrder
	if err := json.Unmarshal(resp, &orders); err != nil {
		return nil, fmt.Errorf("error parsing all algo orders: %w (response: %s)", err, string(resp))
	}

	return orders, nil
}

// ==================== MARKET DATA ====================

// GetFundingRate retrieves the current funding rate for a symbol
func (c *FuturesClientImpl) GetFundingRate(symbol string) (*FundingRate, error) {
	resp, err := c.publicGet("/fapi/v1/premiumIndex", map[string]string{
		"symbol": symbol,
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching funding rate: %w", err)
	}

	var fundingRate FundingRate
	if err := json.Unmarshal(resp, &fundingRate); err != nil {
		return nil, fmt.Errorf("error parsing funding rate: %w", err)
	}

	return &fundingRate, nil
}

// GetFundingRateHistory retrieves funding rate history
func (c *FuturesClientImpl) GetFundingRateHistory(symbol string, limit int) ([]FundingRate, error) {
	params := map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	}

	resp, err := c.publicGet("/fapi/v1/fundingRate", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching funding rate history: %w", err)
	}

	var fundingRates []FundingRate
	if err := json.Unmarshal(resp, &fundingRates); err != nil {
		return nil, fmt.Errorf("error parsing funding rate history: %w", err)
	}

	return fundingRates, nil
}

// GetMarkPrice retrieves the mark price for a symbol
func (c *FuturesClientImpl) GetMarkPrice(symbol string) (*MarkPrice, error) {
	resp, err := c.publicGet("/fapi/v1/premiumIndex", map[string]string{
		"symbol": symbol,
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching mark price: %w", err)
	}

	var markPrice MarkPrice
	if err := json.Unmarshal(resp, &markPrice); err != nil {
		return nil, fmt.Errorf("error parsing mark price: %w", err)
	}

	return &markPrice, nil
}

// GetAllMarkPrices retrieves mark prices for all symbols
func (c *FuturesClientImpl) GetAllMarkPrices() ([]MarkPrice, error) {
	resp, err := c.publicGet("/fapi/v1/premiumIndex", nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching mark prices: %w", err)
	}

	var markPrices []MarkPrice
	if err := json.Unmarshal(resp, &markPrices); err != nil {
		return nil, fmt.Errorf("error parsing mark prices: %w", err)
	}

	return markPrices, nil
}

// GetOrderBookDepth retrieves the order book depth
func (c *FuturesClientImpl) GetOrderBookDepth(symbol string, limit int) (*OrderBookDepth, error) {
	resp, err := c.publicGet("/fapi/v1/depth", map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching order book: %w", err)
	}

	var orderBook OrderBookDepth
	if err := json.Unmarshal(resp, &orderBook); err != nil {
		return nil, fmt.Errorf("error parsing order book: %w", err)
	}

	return &orderBook, nil
}

// GetFuturesKlines retrieves candlestick data for futures
func (c *FuturesClientImpl) GetFuturesKlines(symbol, interval string, limit int) ([]Kline, error) {
	resp, err := c.publicGet("/fapi/v1/klines", map[string]string{
		"symbol":   symbol,
		"interval": interval,
		"limit":    strconv.Itoa(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching klines: %w", err)
	}

	var rawKlines [][]interface{}
	if err := json.Unmarshal(resp, &rawKlines); err != nil {
		return nil, fmt.Errorf("error parsing klines: %w", err)
	}

	klines := make([]Kline, len(rawKlines))
	for i, raw := range rawKlines {
		klines[i] = Kline{
			OpenTime:                 int64(raw[0].(float64)),
			Open:                     parseFloat(raw[1]),
			High:                     parseFloat(raw[2]),
			Low:                      parseFloat(raw[3]),
			Close:                    parseFloat(raw[4]),
			Volume:                   parseFloat(raw[5]),
			CloseTime:                int64(raw[6].(float64)),
			QuoteAssetVolume:         parseFloat(raw[7]),
			NumberOfTrades:           int(raw[8].(float64)),
			TakerBuyBaseAssetVolume:  parseFloat(raw[9]),
			TakerBuyQuoteAssetVolume: parseFloat(raw[10]),
		}
	}

	return klines, nil
}

// Get24hrTicker retrieves 24 hour price change statistics for a symbol
func (c *FuturesClientImpl) Get24hrTicker(symbol string) (*Futures24hrTicker, error) {
	resp, err := c.publicGet("/fapi/v1/ticker/24hr", map[string]string{
		"symbol": symbol,
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching 24hr ticker: %w", err)
	}

	var ticker Futures24hrTicker
	if err := json.Unmarshal(resp, &ticker); err != nil {
		return nil, fmt.Errorf("error parsing 24hr ticker: %w", err)
	}

	return &ticker, nil
}

// GetAll24hrTickers retrieves 24 hour price change statistics for all symbols
func (c *FuturesClientImpl) GetAll24hrTickers() ([]Futures24hrTicker, error) {
	resp, err := c.publicGet("/fapi/v1/ticker/24hr", nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching all 24hr tickers: %w", err)
	}

	var tickers []Futures24hrTicker
	if err := json.Unmarshal(resp, &tickers); err != nil {
		return nil, fmt.Errorf("error parsing 24hr tickers: %w", err)
	}

	return tickers, nil
}

// GetFuturesCurrentPrice retrieves the current price for a symbol
func (c *FuturesClientImpl) GetFuturesCurrentPrice(symbol string) (float64, error) {
	resp, err := c.publicGet("/fapi/v1/ticker/price", map[string]string{
		"symbol": symbol,
	})
	if err != nil {
		return 0, fmt.Errorf("error fetching price: %w", err)
	}

	var priceResp struct {
		Symbol string  `json:"symbol"`
		Price  float64 `json:"price,string"`
	}

	if err := json.Unmarshal(resp, &priceResp); err != nil {
		return 0, fmt.Errorf("error parsing price: %w", err)
	}

	return priceResp.Price, nil
}

// ==================== EXCHANGE INFO ====================

// GetFuturesExchangeInfo retrieves futures exchange information
func (c *FuturesClientImpl) GetFuturesExchangeInfo() (*FuturesExchangeInfo, error) {
	resp, err := c.publicGet("/fapi/v1/exchangeInfo", nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching exchange info: %w", err)
	}

	var exchangeInfo FuturesExchangeInfo
	if err := json.Unmarshal(resp, &exchangeInfo); err != nil {
		return nil, fmt.Errorf("error parsing exchange info: %w", err)
	}

	return &exchangeInfo, nil
}

// GetFuturesSymbols retrieves all available futures trading pairs
func (c *FuturesClientImpl) GetFuturesSymbols() ([]string, error) {
	exchangeInfo, err := c.GetFuturesExchangeInfo()
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, symbol := range exchangeInfo.Symbols {
		// Only include USDT perpetual contracts that are trading
		if symbol.Status == "TRADING" && symbol.QuoteAsset == "USDT" && symbol.ContractType == "PERPETUAL" {
			symbols = append(symbols, symbol.Symbol)
		}
	}

	return symbols, nil
}

// ==================== HISTORY ====================

// GetTradeHistory retrieves trade history for a symbol
func (c *FuturesClientImpl) GetTradeHistory(symbol string, limit int) ([]FuturesTrade, error) {
	params := map[string]string{
		"symbol":    symbol,
		"limit":     strconv.Itoa(limit),
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	resp, err := c.signedGet("/fapi/v1/userTrades", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching trade history: %w", err)
	}

	var trades []FuturesTrade
	if err := json.Unmarshal(resp, &trades); err != nil {
		return nil, fmt.Errorf("error parsing trade history: %w", err)
	}

	return trades, nil
}

// GetTradeHistoryByDateRange retrieves trade history for a date range
// symbol: trading pair (required by Binance API)
// startTime/endTime: Unix milliseconds, 0 to ignore
// limit: Max 1000 records
func (c *FuturesClientImpl) GetTradeHistoryByDateRange(symbol string, startTime, endTime int64, limit int) ([]FuturesTrade, error) {
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	if symbol != "" {
		params["symbol"] = symbol
	}
	if startTime > 0 {
		params["startTime"] = strconv.FormatInt(startTime, 10)
	}
	if endTime > 0 {
		params["endTime"] = strconv.FormatInt(endTime, 10)
	}
	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}

	resp, err := c.signedGet("/fapi/v1/userTrades", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching trade history by date range: %w", err)
	}

	var trades []FuturesTrade
	if err := json.Unmarshal(resp, &trades); err != nil {
		return nil, fmt.Errorf("error parsing trade history: %w", err)
	}

	return trades, nil
}

// GetFundingFeeHistory retrieves funding fee payment history
func (c *FuturesClientImpl) GetFundingFeeHistory(symbol string, limit int) ([]FundingFeeRecord, error) {
	params := map[string]string{
		"incomeType": "FUNDING_FEE",
		"limit":      strconv.Itoa(limit),
		"timestamp":  strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	if symbol != "" {
		params["symbol"] = symbol
	}

	// Signature is added by signParams() in signed* methods

	resp, err := c.signedGet("/fapi/v1/income", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching funding fee history: %w", err)
	}

	var fundingFees []FundingFeeRecord
	if err := json.Unmarshal(resp, &fundingFees); err != nil {
		return nil, fmt.Errorf("error parsing funding fee history: %w", err)
	}

	// Convert timestamps to time.Time
	for i := range fundingFees {
		fundingFees[i].Timestamp = time.UnixMilli(fundingFees[i].Time)
	}

	return fundingFees, nil
}

// GetAllOrders retrieves all orders for a symbol
func (c *FuturesClientImpl) GetAllOrders(symbol string, limit int) ([]FuturesOrder, error) {
	params := map[string]string{
		"symbol":    symbol,
		"limit":     strconv.Itoa(limit),
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	// Signature is added by signParams() in signed* methods

	resp, err := c.signedGet("/fapi/v1/allOrders", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching all orders: %w", err)
	}

	var orders []FuturesOrder
	if err := json.Unmarshal(resp, &orders); err != nil {
		return nil, fmt.Errorf("error parsing all orders: %w", err)
	}

	return orders, nil
}

// GetAllOrdersByDateRange retrieves all orders for a date range
// symbol: trading pair (required by Binance API for allOrders)
// startTime/endTime: Unix milliseconds, 0 to ignore
// limit: Max 1000 records
func (c *FuturesClientImpl) GetAllOrdersByDateRange(symbol string, startTime, endTime int64, limit int) ([]FuturesOrder, error) {
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	if symbol != "" {
		params["symbol"] = symbol
	}
	if startTime > 0 {
		params["startTime"] = strconv.FormatInt(startTime, 10)
	}
	if endTime > 0 {
		params["endTime"] = strconv.FormatInt(endTime, 10)
	}
	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}

	resp, err := c.signedGet("/fapi/v1/allOrders", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching all orders by date range: %w", err)
	}

	var orders []FuturesOrder
	if err := json.Unmarshal(resp, &orders); err != nil {
		return nil, fmt.Errorf("error parsing all orders: %w", err)
	}

	return orders, nil
}

// GetIncomeHistory retrieves income history (realized PnL, funding fees, commissions, etc.)
// incomeType: REALIZED_PNL, FUNDING_FEE, COMMISSION, TRANSFER, etc. Empty string for all types.
// startTime/endTime: Unix milliseconds. Pass 0 to ignore.
// limit: Max 1000 records
func (c *FuturesClientImpl) GetIncomeHistory(incomeType string, startTime, endTime int64, limit int) ([]IncomeRecord, error) {
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	if incomeType != "" {
		params["incomeType"] = incomeType
	}
	if startTime > 0 {
		params["startTime"] = strconv.FormatInt(startTime, 10)
	}
	if endTime > 0 {
		params["endTime"] = strconv.FormatInt(endTime, 10)
	}
	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}

	resp, err := c.signedGet("/fapi/v1/income", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching income history: %w", err)
	}

	var records []IncomeRecord
	if err := json.Unmarshal(resp, &records); err != nil {
		return nil, fmt.Errorf("error parsing income history: %w", err)
	}

	// Convert timestamps to time.Time
	for i := range records {
		records[i].Timestamp = time.UnixMilli(records[i].Time)
	}

	return records, nil
}

// ==================== COMMISSION RATES ====================

// GetCommissionRate fetches user's actual commission rates from Binance
// This returns the real maker/taker fees for the authenticated user's account tier
func (c *FuturesClientImpl) GetCommissionRate(symbol string) (*CommissionRate, error) {
	params := map[string]string{
		"symbol":    symbol,
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}

	resp, err := c.signedGet("/fapi/v1/commissionRate", params)
	if err != nil {
		return nil, fmt.Errorf("error fetching commission rate: %w", err)
	}

	var rate CommissionRate
	if err := json.Unmarshal(resp, &rate); err != nil {
		return nil, fmt.Errorf("error parsing commission rate: %w", err)
	}

	return &rate, nil
}

// ==================== WEBSOCKET ====================

// GetListenKey creates a new user data stream listen key
func (c *FuturesClientImpl) GetListenKey() (string, error) {
	resp, err := c.signedPost("/fapi/v1/listenKey", nil)
	if err != nil {
		return "", fmt.Errorf("error getting listen key: %w", err)
	}

	var listenKeyResp ListenKeyResponse
	if err := json.Unmarshal(resp, &listenKeyResp); err != nil {
		return "", fmt.Errorf("error parsing listen key: %w", err)
	}

	return listenKeyResp.ListenKey, nil
}

// KeepAliveListenKey extends the validity of a listen key
// CRITICAL: This bypasses rate limiter circuit breaker to prevent disconnection
// Listen key keepalive is essential for maintaining WebSocket connection
func (c *FuturesClientImpl) KeepAliveListenKey(listenKey string) error {
	params := map[string]string{
		"listenKey": listenKey,
	}

	// Use critical PUT that bypasses circuit breaker
	_, err := c.criticalPut("/fapi/v1/listenKey", params)
	if err != nil {
		return fmt.Errorf("error keeping listen key alive: %w", err)
	}

	return nil
}

// CloseListenKey closes a user data stream
func (c *FuturesClientImpl) CloseListenKey(listenKey string) error {
	params := map[string]string{
		"listenKey": listenKey,
	}

	_, err := c.signedDelete("/fapi/v1/listenKey", params)
	if err != nil {
		return fmt.Errorf("error closing listen key: %w", err)
	}

	return nil
}

// ==================== HTTP HELPERS ====================

// buildQueryString builds a query string from params (without signature)
func (c *FuturesClientImpl) buildQueryString(params map[string]string) string {
	query := ""
	for k, v := range params {
		if k != "signature" {
			if query != "" {
				query += "&"
			}
			query += k + "=" + url.QueryEscape(v)
		}
	}
	return query
}

// sign creates a signature for the given query string
func (c *FuturesClientImpl) sign(query string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}

// signParams builds query string with signature appended
func (c *FuturesClientImpl) signParams(params map[string]string) string {
	query := c.buildQueryString(params)
	signature := c.sign(query)
	return query + "&signature=" + signature
}

// publicGet performs an unauthenticated GET request with rate limiting and retry
func (c *FuturesClientImpl) publicGet(endpoint string, params map[string]string) ([]byte, error) {
	rateLimiter := GetRateLimiter()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check rate limiter before making request
		if !rateLimiter.WaitForSlot(endpoint, 30*time.Second) {
			return nil, fmt.Errorf("rate limit: circuit breaker open, request blocked")
		}

		values := url.Values{}
		for k, v := range params {
			values.Set(k, v)
		}

		reqURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)
		if len(values) > 0 {
			reqURL = fmt.Sprintf("%s?%s", reqURL, values.Encode())
		}

		resp, err := c.httpClient.Get(reqURL)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] Public GET %s failed (attempt %d/%d): %v, retrying in %v",
					endpoint, attempt+1, maxRetries+1, err, delay)
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		// Update rate limiter from headers
		if usedWeight := resp.Header.Get("X-MBX-USED-WEIGHT-1M"); usedWeight != "" {
			if weight, err := strconv.Atoi(usedWeight); err == nil {
				rateLimiter.UpdateFromHeaders(0, weight)
			}
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error: %s", string(body))

			// Check for rate limit error and trigger circuit breaker
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 418 ||
				strings.Contains(string(body), "-1003") {
				banUntil := ParseBanUntilFromError(string(body))
				rateLimiter.RecordRateLimitError(banUntil)
			}

			if isRetryableError(resp.StatusCode, string(body)) && attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] Public GET %s returned %d (attempt %d/%d): %s, retrying in %v",
					endpoint, resp.StatusCode, attempt+1, maxRetries+1, string(body), delay)
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		}

		// Record successful request
		rateLimiter.RecordRequest(endpoint)
		return body, nil
	}

	return nil, lastErr
}

// isRetryableError checks if an error is transient and should be retried
func isRetryableError(statusCode int, body string) bool {
	// Retry on rate limits (429) and server errors (5xx)
	if statusCode == http.StatusTooManyRequests || statusCode >= 500 {
		return true
	}
	// Retry on specific Binance errors that are transient
	if strings.Contains(body, "-1001") || // DISCONNECTED
		strings.Contains(body, "-1003") || // TOO_MANY_REQUESTS
		strings.Contains(body, "-1015") || // TOO_MANY_ORDERS
		strings.Contains(body, "-1016") { // SERVICE_SHUTTING_DOWN
		return true
	}
	return false
}

// calculateRetryDelay returns delay with exponential backoff and jitter
func calculateRetryDelay(attempt int) time.Duration {
	delay := baseRetryDelay * time.Duration(1<<uint(attempt)) // 2^attempt
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	// Add jitter (±25%)
	jitter := time.Duration(rand.Int63n(int64(delay) / 2))
	return delay + jitter - (delay / 4)
}

// signedGet performs an authenticated GET request with rate limiting and retry logic
func (c *FuturesClientImpl) signedGet(endpoint string, params map[string]string) ([]byte, error) {
	rateLimiter := GetRateLimiter()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check rate limiter before making request
		if !rateLimiter.WaitForSlot(endpoint, 30*time.Second) {
			return nil, fmt.Errorf("rate limit: circuit breaker open, request blocked")
		}

		// Refresh timestamp for each attempt and set recvWindow for clock skew tolerance
		params["timestamp"] = strconv.FormatInt(time.Now().UnixMilli(), 10)
		params["recvWindow"] = "10000" // 10 seconds tolerance for clock skew
		query := c.signParams(params)
		reqURL := fmt.Sprintf("%s%s?%s", c.baseURL, endpoint, query)

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("X-MBX-APIKEY", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] GET %s failed (attempt %d/%d): %v, retrying in %v",
					endpoint, attempt+1, maxRetries+1, err, delay)
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		// Update rate limiter from headers
		if usedWeight := resp.Header.Get("X-MBX-USED-WEIGHT-1M"); usedWeight != "" {
			if weight, err := strconv.Atoi(usedWeight); err == nil {
				rateLimiter.UpdateFromHeaders(0, weight)
			}
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error: %s", string(body))

			// Check for rate limit error and trigger circuit breaker
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 418 ||
				strings.Contains(string(body), "-1003") {
				banUntil := ParseBanUntilFromError(string(body))
				rateLimiter.RecordRateLimitError(banUntil)
			}

			if isRetryableError(resp.StatusCode, string(body)) && attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] GET %s returned %d (attempt %d/%d): %s, retrying in %v",
					endpoint, resp.StatusCode, attempt+1, maxRetries+1, string(body), delay)
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		}

		// Record successful request
		rateLimiter.RecordRequest(endpoint)
		return body, nil
	}

	return nil, lastErr
}

// signedPost performs an authenticated POST request with rate limiting and retry logic
func (c *FuturesClientImpl) signedPost(endpoint string, params map[string]string) ([]byte, error) {
	rateLimiter := GetRateLimiter()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check rate limiter before making request
		if !rateLimiter.WaitForSlot(endpoint, 30*time.Second) {
			return nil, fmt.Errorf("rate limit: circuit breaker open, request blocked")
		}

		// Refresh timestamp for each attempt and set recvWindow for clock skew tolerance
		if params == nil {
			params = make(map[string]string)
		}
		params["timestamp"] = strconv.FormatInt(time.Now().UnixMilli(), 10)
		params["recvWindow"] = "10000" // 10 seconds tolerance for clock skew
		query := c.signParams(params)
		reqURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)

		req, err := http.NewRequest("POST", reqURL, nil)
		if err != nil {
			return nil, err
		}

		req.URL.RawQuery = query
		req.Header.Set("X-MBX-APIKEY", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] POST %s failed (attempt %d/%d): %v, retrying in %v",
					endpoint, attempt+1, maxRetries+1, err, delay)
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		// Update rate limiter from headers
		if usedWeight := resp.Header.Get("X-MBX-USED-WEIGHT-1M"); usedWeight != "" {
			if weight, err := strconv.Atoi(usedWeight); err == nil {
				rateLimiter.UpdateFromHeaders(0, weight)
			}
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error: %s", string(body))

			// Check for rate limit error and trigger circuit breaker
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 418 ||
				strings.Contains(string(body), "-1003") {
				banUntil := ParseBanUntilFromError(string(body))
				rateLimiter.RecordRateLimitError(banUntil)
			}

			if isRetryableError(resp.StatusCode, string(body)) && attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] POST %s returned %d (attempt %d/%d): %s, retrying in %v",
					endpoint, resp.StatusCode, attempt+1, maxRetries+1, string(body), delay)
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		}

		// Record successful request
		rateLimiter.RecordRequest(endpoint)
		return body, nil
	}

	return nil, lastErr
}

// criticalPut performs an authenticated PUT request that BYPASSES the circuit breaker
// This is used for critical operations like listen key keepalive that MUST go through
// even when the circuit breaker is open due to rate limiting
func (c *FuturesClientImpl) criticalPut(endpoint string, params map[string]string) ([]byte, error) {
	rateLimiter := GetRateLimiter()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// BYPASS circuit breaker check - only do weight-based limiting
		// This is intentional for critical operations that must succeed
		result := rateLimiter.TryAcquire(endpoint, PriorityCritical)

		// Even if circuit is open, allow critical requests through with a warning
		if !result.Acquired && result.Reason == "circuit_breaker_open" {
			log.Printf("[BINANCE] CRITICAL PUT %s bypassing circuit breaker (keepalive essential)", endpoint)
			// Don't block, proceed anyway for critical operations
		} else if !result.Acquired {
			// For other reasons (weight limit), wait briefly and retry
			if result.WaitTime > 0 && attempt < maxRetries {
				waitTime := result.WaitTime
				if waitTime > 5*time.Second {
					waitTime = 5 * time.Second
				}
				log.Printf("[BINANCE] CRITICAL PUT %s waiting %v: %s", endpoint, waitTime, result.Reason)
				time.Sleep(waitTime)
				continue
			}
		}

		// Refresh timestamp for each attempt and set recvWindow for clock skew tolerance
		if params == nil {
			params = make(map[string]string)
		}
		params["timestamp"] = strconv.FormatInt(time.Now().UnixMilli(), 10)
		params["recvWindow"] = "10000" // 10 seconds tolerance for clock skew
		query := c.signParams(params)
		reqURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)

		req, err := http.NewRequest("PUT", reqURL, nil)
		if err != nil {
			return nil, err
		}

		req.URL.RawQuery = query
		req.Header.Set("X-MBX-APIKEY", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] CRITICAL PUT %s failed (attempt %d/%d): %v, retrying in %v",
					endpoint, attempt+1, maxRetries+1, err, delay)
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		// Update rate limiter from headers
		if usedWeight := resp.Header.Get("X-MBX-USED-WEIGHT-1M"); usedWeight != "" {
			if weight, err := strconv.Atoi(usedWeight); err == nil {
				rateLimiter.UpdateFromHeaders(0, weight)
			}
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error: %s", string(body))

			// For critical requests, still record rate limit errors but don't block future critical requests
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 418 ||
				strings.Contains(string(body), "-1003") {
				banUntil := ParseBanUntilFromError(string(body))
				rateLimiter.RecordRateLimitError(banUntil)
				log.Printf("[BINANCE] CRITICAL PUT %s got rate limited - will retry after backoff", endpoint)
			}

			if isRetryableError(resp.StatusCode, string(body)) && attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] CRITICAL PUT %s returned %d (attempt %d/%d): %s, retrying in %v",
					endpoint, resp.StatusCode, attempt+1, maxRetries+1, string(body), delay)
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		}

		// Record successful request
		rateLimiter.RecordRequest(endpoint)
		log.Printf("[BINANCE] CRITICAL PUT %s succeeded", endpoint)
		return body, nil
	}

	return nil, lastErr
}

// signedPut performs an authenticated PUT request with rate limiting and retry logic
func (c *FuturesClientImpl) signedPut(endpoint string, params map[string]string) ([]byte, error) {
	rateLimiter := GetRateLimiter()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check rate limiter before making request
		if !rateLimiter.WaitForSlot(endpoint, 30*time.Second) {
			return nil, fmt.Errorf("rate limit: circuit breaker open, request blocked")
		}

		// Refresh timestamp for each attempt and set recvWindow for clock skew tolerance
		params["timestamp"] = strconv.FormatInt(time.Now().UnixMilli(), 10)
		params["recvWindow"] = "10000" // 10 seconds tolerance for clock skew
		query := c.signParams(params)
		reqURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)

		req, err := http.NewRequest("PUT", reqURL, nil)
		if err != nil {
			return nil, err
		}

		req.URL.RawQuery = query
		req.Header.Set("X-MBX-APIKEY", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] PUT %s failed (attempt %d/%d): %v, retrying in %v",
					endpoint, attempt+1, maxRetries+1, err, delay)
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		// Update rate limiter from headers
		if usedWeight := resp.Header.Get("X-MBX-USED-WEIGHT-1M"); usedWeight != "" {
			if weight, err := strconv.Atoi(usedWeight); err == nil {
				rateLimiter.UpdateFromHeaders(0, weight)
			}
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error: %s", string(body))

			// Check for rate limit error and trigger circuit breaker
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 418 ||
				strings.Contains(string(body), "-1003") {
				banUntil := ParseBanUntilFromError(string(body))
				rateLimiter.RecordRateLimitError(banUntil)
			}

			if isRetryableError(resp.StatusCode, string(body)) && attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] PUT %s returned %d (attempt %d/%d): %s, retrying in %v",
					endpoint, resp.StatusCode, attempt+1, maxRetries+1, string(body), delay)
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		}

		// Record successful request
		rateLimiter.RecordRequest(endpoint)
		return body, nil
	}

	return nil, lastErr
}

// signedDelete performs an authenticated DELETE request with rate limiting and retry logic
func (c *FuturesClientImpl) signedDelete(endpoint string, params map[string]string) ([]byte, error) {
	rateLimiter := GetRateLimiter()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check rate limiter before making request
		if !rateLimiter.WaitForSlot(endpoint, 30*time.Second) {
			return nil, fmt.Errorf("rate limit: circuit breaker open, request blocked")
		}

		// Refresh timestamp for each attempt and set recvWindow for clock skew tolerance
		params["timestamp"] = strconv.FormatInt(time.Now().UnixMilli(), 10)
		params["recvWindow"] = "10000" // 10 seconds tolerance for clock skew
		query := c.signParams(params)
		reqURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)

		req, err := http.NewRequest("DELETE", reqURL, nil)
		if err != nil {
			return nil, err
		}

		req.URL.RawQuery = query
		req.Header.Set("X-MBX-APIKEY", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] DELETE %s failed (attempt %d/%d): %v, retrying in %v",
					endpoint, attempt+1, maxRetries+1, err, delay)
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		// Update rate limiter from headers
		if usedWeight := resp.Header.Get("X-MBX-USED-WEIGHT-1M"); usedWeight != "" {
			if weight, err := strconv.Atoi(usedWeight); err == nil {
				rateLimiter.UpdateFromHeaders(0, weight)
			}
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API error: %s", string(body))

			// Check for rate limit error and trigger circuit breaker
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 418 ||
				strings.Contains(string(body), "-1003") {
				banUntil := ParseBanUntilFromError(string(body))
				rateLimiter.RecordRateLimitError(banUntil)
			}

			if isRetryableError(resp.StatusCode, string(body)) && attempt < maxRetries {
				delay := calculateRetryDelay(attempt)
				log.Printf("[BINANCE] DELETE %s returned %d (attempt %d/%d): %s, retrying in %v",
					endpoint, resp.StatusCode, attempt+1, maxRetries+1, string(body), delay)
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		}

		// Record successful request
		rateLimiter.RecordRequest(endpoint)
		return body, nil
	}

	return nil, lastErr
}

// Ensure FuturesClientImpl implements FuturesClient
var _ FuturesClient = (*FuturesClientImpl)(nil)
