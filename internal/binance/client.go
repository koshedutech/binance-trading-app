package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	apiKey     string
	secretKey  string
	baseURL    string
	httpClient *http.Client
}

func NewClient(apiKey, secretKey, baseURL string) *Client {
	return &Client{
		apiKey:     apiKey,
		secretKey:  secretKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Kline represents a candlestick
type Kline struct {
	OpenTime                 int64   `json:"openTime"`
	Open                     float64 `json:"open,string"`
	High                     float64 `json:"high,string"`
	Low                      float64 `json:"low,string"`
	Close                    float64 `json:"close,string"`
	Volume                   float64 `json:"volume,string"`
	CloseTime                int64   `json:"closeTime"`
	QuoteAssetVolume         float64 `json:"quoteAssetVolume,string"`
	NumberOfTrades           int     `json:"numberOfTrades"`
	TakerBuyBaseAssetVolume  float64 `json:"takerBuyBaseAssetVolume,string"`
	TakerBuyQuoteAssetVolume float64 `json:"takerBuyQuoteAssetVolume,string"`
}

// Ticker24hr represents 24hr ticker price change statistics
type Ticker24hr struct {
	Symbol             string  `json:"symbol"`
	PriceChange        float64 `json:"priceChange,string"`
	PriceChangePercent float64 `json:"priceChangePercent,string"`
	WeightedAvgPrice   float64 `json:"weightedAvgPrice,string"`
	LastPrice          float64 `json:"lastPrice,string"`
	Volume             float64 `json:"volume,string"`
	QuoteVolume        float64 `json:"quoteVolume,string"`
	OpenTime           int64   `json:"openTime"`
	CloseTime          int64   `json:"closeTime"`
	FirstId            int64   `json:"firstId"`
	LastId             int64   `json:"lastId"`
	Count              int64   `json:"count"`
}

// OrderResponse represents a response from placing an order
type OrderResponse struct {
	Symbol              string  `json:"symbol"`
	OrderId             int64   `json:"orderId"`
	ClientOrderId       string  `json:"clientOrderId"`
	TransactTime        int64   `json:"transactTime"`
	Price               float64 `json:"price,string"`
	OrigQty             float64 `json:"origQty,string"`
	ExecutedQty         float64 `json:"executedQty,string"`
	CummulativeQuoteQty float64 `json:"cummulativeQuoteQty,string"`
	Status              string  `json:"status"`
	Type                string  `json:"type"`
	Side                string  `json:"side"`
}

// GetKlines fetches candlestick data
func (c *Client) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", interval)
	params.Set("limit", strconv.Itoa(limit))

	endpoint := fmt.Sprintf("%s/api/v3/klines?%s", c.baseURL, params.Encode())

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("error fetching klines: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var rawKlines [][]interface{}
	if err := json.Unmarshal(body, &rawKlines); err != nil {
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

// Get24hrTickers fetches 24hr ticker data for all symbols
func (c *Client) Get24hrTickers() ([]Ticker24hr, error) {
	endpoint := fmt.Sprintf("%s/api/v3/ticker/24hr", c.baseURL)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("error fetching tickers: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var tickers []Ticker24hr
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, fmt.Errorf("error parsing tickers: %w", err)
	}

	return tickers, nil
}

// PlaceOrder places a new order
func (c *Client) PlaceOrder(params map[string]string) (*OrderResponse, error) {
	params["timestamp"] = strconv.FormatInt(time.Now().UnixMilli(), 10)
	params["signature"] = c.sign(params)

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	endpoint := fmt.Sprintf("%s/api/v3/order", c.baseURL)

	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = values.Encode()
	req.Header.Set("X-MBX-APIKEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error placing order: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("error parsing order response: %w", err)
	}

	return &orderResp, nil
}

// CancelOrder cancels an existing order
func (c *Client) CancelOrder(symbol string, orderId int64) error {
	params := map[string]string{
		"symbol":    symbol,
		"orderId":   strconv.FormatInt(orderId, 10),
		"timestamp": strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	params["signature"] = c.sign(params)

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	endpoint := fmt.Sprintf("%s/api/v3/order", c.baseURL)

	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}

	req.URL.RawQuery = values.Encode()
	req.Header.Set("X-MBX-APIKEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error canceling order: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}

	return nil
}

// GetCurrentPrice fetches the current price for a symbol
func (c *Client) GetCurrentPrice(symbol string) (float64, error) {
	endpoint := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", c.baseURL, symbol)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return 0, fmt.Errorf("error fetching price: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API error: %s", string(body))
	}

	var priceResp struct {
		Symbol string  `json:"symbol"`
		Price  float64 `json:"price,string"`
	}

	if err := json.Unmarshal(body, &priceResp); err != nil {
		return 0, fmt.Errorf("error parsing price: %w", err)
	}

	return priceResp.Price, nil
}

// SymbolInfo represents basic symbol information
type SymbolInfo struct {
	Symbol               string `json:"symbol"`
	Status               string `json:"status"`
	BaseAsset            string `json:"baseAsset"`
	QuoteAsset           string `json:"quoteAsset"`
	IsSpotTradingAllowed bool   `json:"isSpotTradingAllowed"`
}

// ExchangeInfo represents exchange information response
type ExchangeInfo struct {
	Symbols []SymbolInfo `json:"symbols"`
}

// GetExchangeInfo fetches exchange information including all trading symbols
func (c *Client) GetExchangeInfo() (*ExchangeInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v3/exchangeInfo", c.baseURL)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("error fetching exchange info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var exchangeInfo ExchangeInfo
	if err := json.Unmarshal(body, &exchangeInfo); err != nil {
		return nil, fmt.Errorf("error parsing exchange info: %w", err)
	}

	return &exchangeInfo, nil
}

// sign creates a signature for authenticated requests
func (c *Client) sign(params map[string]string) string {
	query := ""
	for k, v := range params {
		if k != "signature" {
			if query != "" {
				query += "&"
			}
			query += k + "=" + v
		}
	}

	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}

func parseFloat(val interface{}) float64 {
	switch v := val.(type) {
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	case float64:
		return v
	default:
		return 0
	}
}
