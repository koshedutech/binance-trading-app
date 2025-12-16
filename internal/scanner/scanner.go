package scanner

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/strategy"
)

// Scanner orchestrates strategy scanning across multiple symbols
type Scanner struct {
	client     *binance.Client
	repo       *database.Repository
	evaluator  *ProximityEvaluator
	strategies []strategy.Strategy
	config     ScannerConfig
	stopChan   chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
	lastResult *ScanResult
}

// NewScanner creates a new scanner instance
func NewScanner(
	client *binance.Client,
	repo *database.Repository,
	strategies []strategy.Strategy,
	config ScannerConfig,
) *Scanner {
	return &Scanner{
		client:     client,
		repo:       repo,
		evaluator:  NewProximityEvaluator(config.CacheTTL),
		strategies: strategies,
		config:     config,
		stopChan:   make(chan struct{}),
	}
}

// Start begins the background scan loop
func (sc *Scanner) Start() {
	if !sc.config.Enabled {
		log.Println("Strategy scanner is disabled")
		return
	}

	sc.wg.Add(1)
	go sc.runScanLoop()
	log.Println("Strategy scanner started")
}

// runScanLoop executes scans at configured intervals
func (sc *Scanner) runScanLoop() {
	defer sc.wg.Done()

	ticker := time.NewTicker(sc.config.ScanInterval)
	defer ticker.Stop()

	// Run immediately
	sc.scan()

	for {
		select {
		case <-ticker.C:
			sc.scan()
		case <-sc.stopChan:
			log.Println("Strategy scanner stopped")
			return
		}
	}
}

// Scan executes a single scan cycle (public method for manual triggering)
func (sc *Scanner) Scan() {
	sc.scan()
}

// scan executes a single scan cycle
func (sc *Scanner) scan() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	startTime := time.Now()
	scanID := fmt.Sprintf("scan-%d", startTime.Unix())

	log.Printf("[Scanner] Starting scan %s", scanID)

	// Get symbols to scan
	symbols := sc.getSymbolsToScan(ctx)

	// Create result container
	allResults := []ProximityResult{}
	resultChan := make(chan ProximityResult, len(symbols)*len(sc.strategies))

	// Worker pool for concurrent scanning
	symbolChan := make(chan string, len(symbols))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < sc.config.WorkerCount; i++ {
		wg.Add(1)
		go sc.worker(ctx, symbolChan, resultChan, &wg)
	}

	// Feed symbols to workers
	go func() {
		for _, symbol := range symbols {
			select {
			case symbolChan <- symbol:
			case <-ctx.Done():
				break
			}
		}
		close(symbolChan)
	}()

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		if result.ReadinessScore > 0 { // Only include relevant results
			allResults = append(allResults, result)
		}
	}

	// Sort by readiness score (descending)
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].ReadinessScore > allResults[j].ReadinessScore
	})

	// Limit to top results
	if len(allResults) > sc.config.MaxSymbols {
		allResults = allResults[:sc.config.MaxSymbols]
	}

	scanResult := &ScanResult{
		ScanID:         scanID,
		StartTime:      startTime,
		EndTime:        time.Now(),
		Duration:       time.Since(startTime),
		SymbolsScanned: len(symbols),
		Results:        allResults,
	}

	// Update last result
	sc.mu.Lock()
	sc.lastResult = scanResult
	sc.mu.Unlock()

	log.Printf("[Scanner] Scan completed in %v: %d opportunities found (top %d shown)",
		scanResult.Duration, len(allResults), sc.config.MaxSymbols)
}

// worker processes symbols from the channel
func (sc *Scanner) worker(
	ctx context.Context,
	symbolChan <-chan string,
	resultChan chan<- ProximityResult,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for symbol := range symbolChan {
		select {
		case <-ctx.Done():
			return
		default:
			sc.scanSymbol(ctx, symbol, resultChan)
		}
	}
}

// scanSymbol evaluates all strategies against a single symbol
func (sc *Scanner) scanSymbol(
	ctx context.Context,
	symbol string,
	resultChan chan<- ProximityResult,
) {
	// Get current price
	currentPrice, err := sc.client.GetCurrentPrice(symbol)
	if err != nil {
		return
	}

	// Evaluate each strategy
	for _, strat := range sc.strategies {
		// Get klines for this strategy's interval
		klines, err := sc.client.GetKlines(symbol, strat.GetInterval(), 100)
		if err != nil {
			continue
		}

		// Evaluate proximity
		result, err := sc.evaluator.EvaluateProximity(strat, klines, currentPrice)
		if err != nil {
			continue
		}

		resultChan <- *result
	}
}

// getSymbolsToScan returns symbols to evaluate (watchlist + all USDT pairs)
func (sc *Scanner) getSymbolsToScan(ctx context.Context) []string {
	symbols := []string{}

	// Add watchlist symbols first (prioritize these)
	if sc.config.IncludeWatchlist {
		watchlist, err := sc.repo.GetWatchlist(ctx)
		if err == nil {
			for _, item := range watchlist {
				symbols = append(symbols, item.Symbol)
			}
		}
	}

	// Get all USDT pairs from Binance
	exchangeInfo, err := sc.client.GetExchangeInfo()
	if err == nil {
		for _, s := range exchangeInfo.Symbols {
			if s.Status == "TRADING" && s.QuoteAsset == "USDT" {
				// Avoid duplicates from watchlist
				if !contains(symbols, s.Symbol) {
					symbols = append(symbols, s.Symbol)
				}
			}
		}
	}

	return symbols
}

// GetLastResult returns the most recent scan result
func (sc *Scanner) GetLastResult() *ScanResult {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.lastResult
}

// Stop gracefully shuts down the scanner
func (sc *Scanner) Stop() {
	close(sc.stopChan)
	sc.wg.Wait()
}

// Helper function to check if string slice contains a value
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
