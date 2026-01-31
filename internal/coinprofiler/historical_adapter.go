// Package coinprofiler provides real-time coin data collection via WebSocket
// for the Chain Trading System.
package coinprofiler

// ============================================================================
// HISTORICAL DATA ADAPTER
// Epic 14: Entry Decision Strategy Requirements & Real-Time Monitoring
//
// This adapter wraps a Binance FuturesClient to provide historical candle data
// for pre-populating the CoinProfiler's candle history buffer on startup.
// ============================================================================

// BinanceKline mirrors the binance.Kline struct to avoid import cycles.
// The actual binance.Kline has the same field structure.
type BinanceKline struct {
	OpenTime                 int64
	Open                     float64
	High                     float64
	Low                      float64
	Close                    float64
	Volume                   float64
	CloseTime                int64
	QuoteAssetVolume         float64
	NumberOfTrades           int
	TakerBuyBaseAssetVolume  float64
	TakerBuyQuoteAssetVolume float64
}

// BinanceFuturesKlineProvider defines the interface for fetching klines from Binance.
// This matches the FuturesClient.GetFuturesKlines method signature.
type BinanceFuturesKlineProvider interface {
	GetFuturesKlines(symbol, interval string, limit int) ([]BinanceKline, error)
}

// FuturesClientAdapter wraps a FuturesClient to implement HistoricalDataProvider.
// This enables CoinProfiler to fetch historical candles without direct dependency
// on the binance package.
type FuturesClientAdapter struct {
	// getKlines is the function to call for fetching klines
	// This is set via NewFuturesClientAdapter to avoid import cycles
	getKlines func(symbol, interval string, limit int) ([]HistoricalKline, error)
}

// NewFuturesClientAdapter creates an adapter that converts FuturesClient klines
// to HistoricalKline format. The getKlines function should wrap the actual
// FuturesClient.GetFuturesKlines method.
//
// Example usage in user_autopilot_manager.go:
//
//	adapter := coinprofiler.NewFuturesClientAdapter(func(symbol, interval string, limit int) ([]coinprofiler.HistoricalKline, error) {
//	    klines, err := futuresClient.GetFuturesKlines(symbol, interval, limit)
//	    if err != nil {
//	        return nil, err
//	    }
//	    result := make([]coinprofiler.HistoricalKline, len(klines))
//	    for i, k := range klines {
//	        result[i] = coinprofiler.HistoricalKline{
//	            OpenTime: k.OpenTime,
//	            Open: k.Open,
//	            High: k.High,
//	            Low: k.Low,
//	            Close: k.Close,
//	            Volume: k.Volume,
//	            CloseTime: k.CloseTime,
//	            QuoteAssetVolume: k.QuoteAssetVolume,
//	            NumberOfTrades: k.NumberOfTrades,
//	            TakerBuyBaseAssetVolume: k.TakerBuyBaseAssetVolume,
//	            TakerBuyQuoteAssetVolume: k.TakerBuyQuoteAssetVolume,
//	        }
//	    }
//	    return result, nil
//	})
func NewFuturesClientAdapter(getKlines func(symbol, interval string, limit int) ([]HistoricalKline, error)) *FuturesClientAdapter {
	return &FuturesClientAdapter{
		getKlines: getKlines,
	}
}

// GetFuturesKlines implements HistoricalDataProvider interface.
func (a *FuturesClientAdapter) GetFuturesKlines(symbol, interval string, limit int) ([]HistoricalKline, error) {
	if a.getKlines == nil {
		return nil, nil
	}
	return a.getKlines(symbol, interval, limit)
}
