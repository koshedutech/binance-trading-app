package api

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/strategy"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// PatternScanRequest represents a pattern scan request
type PatternScanRequest struct {
	Symbols   []string `json:"symbols" binding:"required"`
	Intervals []string `json:"intervals" binding:"required"`
}

// TimeframePatternResult represents patterns detected in a specific timeframe
type TimeframePatternResult struct {
	Interval string                         `json:"interval"`
	Patterns []strategy.CandlestickPattern  `json:"patterns"`
}

// SymbolPatternResult represents all patterns detected for a symbol
type SymbolPatternResult struct {
	Symbol     string                   `json:"symbol"`
	Timeframes []TimeframePatternResult `json:"timeframes"`
}

// handleScanPatterns scans multiple symbols across multiple timeframes for candlestick patterns
func (s *Server) handleScanPatterns(c *gin.Context) {
	var req PatternScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get Binance client
	client, ok := s.botAPI.GetClient().(*binance.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Binance client not available"})
		return
	}

	results := []SymbolPatternResult{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Scan each symbol concurrently
	for _, symbol := range req.Symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()

			symbolResult := SymbolPatternResult{
				Symbol:     sym,
				Timeframes: []TimeframePatternResult{},
			}

			// Scan each timeframe for this symbol
			for _, interval := range req.Intervals {
				// Fetch klines for this symbol and interval
				klines, err := client.GetKlines(sym, interval, 100)
				if err != nil {
					continue
				}

				if len(klines) == 0 {
					continue
				}

				// Detect patterns
				patterns := strategy.DetectAllPatterns(klines)

				if len(patterns) > 0 {
					symbolResult.Timeframes = append(symbolResult.Timeframes, TimeframePatternResult{
						Interval: interval,
						Patterns: patterns,
					})
				}
			}

			// Only add if patterns were found
			if len(symbolResult.Timeframes) > 0 {
				mu.Lock()
				results = append(results, symbolResult)
				mu.Unlock()
			}
		}(symbol)
	}

	wg.Wait()

	c.JSON(http.StatusOK, results)
}

// handleGetAllSymbols returns all available USDT symbols from Binance
func (s *Server) handleGetAllSymbols(c *gin.Context) {
	// Get Binance client
	client, ok := s.botAPI.GetClient().(*binance.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Binance client not available"})
		return
	}

	symbols, err := client.GetAllSymbols()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter for USDT pairs only
	usdtSymbols := []string{}
	for _, symbol := range symbols {
		if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
			usdtSymbols = append(usdtSymbols, symbol)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"symbols": usdtSymbols,
		"count":   len(usdtSymbols),
	})
}
