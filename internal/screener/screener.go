package screener

import (
	"binance-trading-bot/config"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Screener scans all crypto pairs for opportunities
type Screener struct {
	client   *binance.Client
	config   config.ScreenerConfig
	repo     *database.Repository
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.RWMutex
	results  []ScreenResult
}

// ScreenResult represents a screening result for a symbol
type ScreenResult struct {
	Symbol             string
	LastPrice          float64
	PriceChangePercent float64
	Volume             float64
	QuoteVolume        float64
	HighLow24h         struct {
		High float64
		Low  float64
	}
	Timestamp time.Time
	Signals   []string
}

func NewScreener(client *binance.Client, config config.ScreenerConfig, repo *database.Repository) *Screener {
	return &Screener{
		client:   client,
		config:   config,
		repo:     repo,
		stopChan: make(chan struct{}),
		results:  make([]ScreenResult, 0),
	}
}

// StartScreening begins the screening process
func (s *Screener) StartScreening() {
	if !s.config.Enabled {
		log.Println("Screener is disabled in config")
		return
	}

	s.wg.Add(1)
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.config.ScreeningInterval) * time.Second)
	defer ticker.Stop()

	log.Println("Screener started")

	// Run immediately on start
	s.scan()

	for {
		select {
		case <-ticker.C:
			s.scan()
		case <-s.stopChan:
			log.Println("Screener stopped")
			return
		}
	}
}

// scan performs a full market scan
func (s *Screener) scan() {
	log.Println("Starting market scan...")
	startTime := time.Now()

	tickers, err := s.client.Get24hrTickers()
	if err != nil {
		log.Printf("Error fetching tickers: %v", err)
		return
	}

	results := make([]ScreenResult, 0)
	filtered := 0

	for _, ticker := range tickers {
		// Filter by quote currency
		if !strings.HasSuffix(ticker.Symbol, s.config.QuoteCurrency) {
			continue
		}

		// Skip excluded symbols
		if s.isExcluded(ticker.Symbol) {
			continue
		}

		// Filter by minimum volume
		if ticker.QuoteVolume < s.config.MinVolume {
			filtered++
			continue
		}

		// Filter by minimum price change
		if ticker.PriceChangePercent < s.config.MinPriceChange {
			filtered++
			continue
		}

		result := ScreenResult{
			Symbol:             ticker.Symbol,
			LastPrice:          ticker.LastPrice,
			PriceChangePercent: ticker.PriceChangePercent,
			Volume:             ticker.Volume,
			QuoteVolume:        ticker.QuoteVolume,
			Timestamp:          time.Now(),
			Signals:            make([]string, 0),
		}

		// Analyze the symbol
		s.analyzeSymbol(&result)

		results = append(results, result)

		// Limit the number of symbols
		if len(results) >= s.config.MaxSymbols {
			break
		}
	}

	// Update results
	s.mu.Lock()
	s.results = results
	s.mu.Unlock()

	duration := time.Since(startTime)
	log.Printf("Market scan completed in %v. Found %d opportunities (filtered %d)", duration, len(results), filtered)

	// Save results to database
	if s.repo != nil {
		s.saveResultsToDatabase(results)
	}

	// Print top opportunities
	s.printTopOpportunities(5)
}

// analyzeSymbol performs technical analysis on a symbol
func (s *Screener) analyzeSymbol(result *ScreenResult) {
	// Fetch recent klines for analysis
	klines, err := s.client.GetKlines(result.Symbol, s.config.Interval, 10)
	if err != nil {
		return
	}

	if len(klines) < 2 {
		return
	}

	lastCandle := klines[len(klines)-2]
	currentCandle := klines[len(klines)-1]

	// Store high/low
	result.HighLow24h.High = lastCandle.High
	result.HighLow24h.Low = lastCandle.Low

	// Check for breakout above previous high
	if currentCandle.Close > lastCandle.High {
		result.Signals = append(result.Signals, fmt.Sprintf("BREAKOUT: Broke above %.2f", lastCandle.High))
	}

	// Check for price near previous low (support test)
	touchDistance := 0.005 // 0.5%
	if currentCandle.Close <= lastCandle.Low*(1+touchDistance) && currentCandle.Close >= lastCandle.Low {
		result.Signals = append(result.Signals, fmt.Sprintf("SUPPORT: Near low %.2f", lastCandle.Low))
	}

	// Check for high volume
	avgVolume := calculateAverageVolume(klines)
	if lastCandle.Volume > avgVolume*1.5 {
		result.Signals = append(result.Signals, "HIGH_VOLUME")
	}

	// Check for strong momentum
	if result.PriceChangePercent > 5 {
		result.Signals = append(result.Signals, fmt.Sprintf("STRONG_MOMENTUM: +%.2f%%", result.PriceChangePercent))
	}
}

// GetResults returns the current screening results
func (s *Screener) GetResults() []ScreenResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return a copy
	results := make([]ScreenResult, len(s.results))
	copy(results, s.results)
	return results
}

// printTopOpportunities prints the top screening results
func (s *Screener) printTopOpportunities(count int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.results) == 0 {
		return
	}

	log.Println("\n=== TOP OPPORTUNITIES ===")
	displayCount := count
	if len(s.results) < count {
		displayCount = len(s.results)
	}

	for i := 0; i < displayCount; i++ {
		r := s.results[i]
		log.Printf("%d. %s - Price: %.4f | Change: +%.2f%% | Volume: $%.0f | Signals: %v",
			i+1, r.Symbol, r.LastPrice, r.PriceChangePercent, r.QuoteVolume, r.Signals)
	}
	log.Println("========================\n")
}

// isExcluded checks if a symbol is in the exclusion list
func (s *Screener) isExcluded(symbol string) bool {
	for _, excluded := range s.config.ExcludeSymbols {
		if symbol == excluded {
			return true
		}
	}
	return false
}

// saveResultsToDatabase saves screening results to the database
func (s *Screener) saveResultsToDatabase(results []ScreenResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, result := range results {
		dbResult := &database.ScreenerResult{
			Symbol:             result.Symbol,
			LastPrice:          result.LastPrice,
			PriceChangePercent: &result.PriceChangePercent,
			Volume:             &result.Volume,
			QuoteVolume:        &result.QuoteVolume,
			Signals:            result.Signals,
			Timestamp:          result.Timestamp,
		}

		if err := s.repo.CreateScreenerResult(ctx, dbResult); err != nil {
			log.Printf("Failed to save screener result for %s: %v", result.Symbol, err)
		}
	}
}

// Stop stops the screener
func (s *Screener) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func calculateAverageVolume(klines []binance.Kline) float64 {
	if len(klines) == 0 {
		return 0
	}

	var sum float64
	for _, k := range klines {
		sum += k.Volume
	}
	return sum / float64(len(klines))
}
