package autopilot

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"sync"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
)

// SymbolValidator provides validation and normalization for trading symbols
// It syncs requirements from Binance and caches them for fast access
type SymbolValidator struct {
	mu               sync.RWMutex
	cache            map[string]*database.SymbolRequirements
	repo             *database.SymbolRequirementsRepository
	futuresClient    binance.FuturesClient
	lastSync         time.Time
	syncInterval     time.Duration
	initialized      bool
}

// ValidationError represents a validation failure with details
type ValidationError struct {
	Symbol    string
	Field     string
	Value     float64
	Required  string
	Message   string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s validation failed for %s: %s (value: %v, required: %s)",
		e.Field, e.Symbol, e.Message, e.Value, e.Required)
}

// SymbolValidationResult contains the result of symbol-based order validation
// Different from OrderValidationResult in futures_controller.go as this includes
// detailed ValidationError objects instead of plain strings
type SymbolValidationResult struct {
	Valid            bool
	Symbol           string
	OriginalQty      float64
	RoundedQty       float64
	OriginalPrice    float64
	RoundedPrice     float64
	Errors           []*ValidationError
	Warnings         []string
}

var (
	symbolValidator     *SymbolValidator
	symbolValidatorOnce sync.Once
)

// GetSymbolValidator returns the singleton SymbolValidator instance
func GetSymbolValidator() *SymbolValidator {
	symbolValidatorOnce.Do(func() {
		symbolValidator = &SymbolValidator{
			cache:        make(map[string]*database.SymbolRequirements),
			syncInterval: 6 * time.Hour, // Sync every 6 hours
		}
	})
	return symbolValidator
}

// Initialize sets up the validator with database and client connections
func (sv *SymbolValidator) Initialize(repo *database.SymbolRequirementsRepository, client binance.FuturesClient) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	sv.repo = repo
	sv.futuresClient = client

	// Load from database first (fast startup)
	ctx := context.Background()
	if err := sv.loadFromDatabase(ctx); err != nil {
		log.Printf("[SYMBOL-VALIDATOR] Warning: Could not load from database: %v", err)
	}

	// If cache is empty or stale, sync from Binance
	if len(sv.cache) == 0 || time.Since(sv.lastSync) > sv.syncInterval {
		if err := sv.syncFromBinance(ctx); err != nil {
			log.Printf("[SYMBOL-VALIDATOR] Warning: Could not sync from Binance: %v", err)
			// Continue with whatever we have
		}
	}

	sv.initialized = true
	log.Printf("[SYMBOL-VALIDATOR] Initialized with %d symbols", len(sv.cache))
	return nil
}

// loadFromDatabase loads cached requirements from the database
func (sv *SymbolValidator) loadFromDatabase(ctx context.Context) error {
	if sv.repo == nil {
		return fmt.Errorf("repository not set")
	}

	requirements, err := sv.repo.GetAllActive(ctx)
	if err != nil {
		return err
	}

	for _, req := range requirements {
		sv.cache[req.Symbol] = req
	}

	if len(requirements) > 0 {
		lastSync, _ := sv.repo.GetLastSyncTime(ctx)
		if lastSync != nil {
			sv.lastSync = *lastSync
		}
	}

	log.Printf("[SYMBOL-VALIDATOR] Loaded %d symbols from database", len(requirements))
	return nil
}

// syncFromBinance fetches latest requirements from Binance API and stores in DB
func (sv *SymbolValidator) syncFromBinance(ctx context.Context) error {
	if sv.futuresClient == nil {
		return fmt.Errorf("futures client not set")
	}

	log.Println("[SYMBOL-VALIDATOR] Syncing symbol requirements from Binance...")

	info, err := sv.futuresClient.GetFuturesExchangeInfo()
	if err != nil {
		return fmt.Errorf("failed to fetch exchange info: %w", err)
	}

	var requirements []*database.SymbolRequirements
	for _, sym := range info.Symbols {
		if sym.Status != "TRADING" {
			continue
		}

		req := sv.parseSymbolInfo(&sym)
		requirements = append(requirements, req)
		sv.cache[req.Symbol] = req
	}

	// Store in database for persistence
	if sv.repo != nil {
		count, err := sv.repo.BulkUpsert(ctx, requirements)
		if err != nil {
			log.Printf("[SYMBOL-VALIDATOR] Warning: Failed to persist to database: %v", err)
		} else {
			log.Printf("[SYMBOL-VALIDATOR] Persisted %d symbols to database", count)
		}
	}

	sv.lastSync = time.Now()
	log.Printf("[SYMBOL-VALIDATOR] Synced %d symbols from Binance", len(requirements))
	return nil
}

// parseSymbolInfo extracts requirements from Binance symbol info
func (sv *SymbolValidator) parseSymbolInfo(sym *binance.FuturesSymbolInfo) *database.SymbolRequirements {
	req := &database.SymbolRequirements{
		Symbol:            sym.Symbol,
		PricePrecision:    sym.PricePrecision,
		QuantityPrecision: sym.QuantityPrecision,
		BaseAsset:         sym.BaseAsset,
		QuoteAsset:        sym.QuoteAsset,
		MarginAsset:       sym.MarginAsset,
		ContractType:      sym.ContractType,
		Status:            sym.Status,
		LastSyncedAt:      time.Now(),
		// Defaults that will be overwritten by filters
		TickSize:    0.0001,
		StepSize:    1,
		MinQty:      1,
		MaxQty:      10000000,
		MinNotional: 5,
	}

	// Parse filters for accurate values (Binance returns strings)
	for _, filter := range sym.Filters {
		switch filter.FilterType {
		case "PRICE_FILTER":
			if tickSize, err := strconv.ParseFloat(filter.TickSize, 64); err == nil && tickSize > 0 {
				req.TickSize = tickSize
				req.PricePrecision = calculatePrecisionFromStep(tickSize)
			}
			if minPrice, err := strconv.ParseFloat(filter.MinPrice, 64); err == nil {
				req.MinPrice = minPrice
			}
			if maxPrice, err := strconv.ParseFloat(filter.MaxPrice, 64); err == nil {
				req.MaxPrice = maxPrice
			}

		case "LOT_SIZE":
			if stepSize, err := strconv.ParseFloat(filter.StepSize, 64); err == nil && stepSize > 0 {
				req.StepSize = stepSize
				req.QuantityPrecision = calculatePrecisionFromStep(stepSize)
			}
			if minQty, err := strconv.ParseFloat(filter.MinQty, 64); err == nil {
				req.MinQty = minQty
			}
			if maxQty, err := strconv.ParseFloat(filter.MaxQty, 64); err == nil {
				req.MaxQty = maxQty
			}

		case "MARKET_LOT_SIZE":
			if minQty, err := strconv.ParseFloat(filter.MinQty, 64); err == nil {
				req.MarketMinQty = minQty
			}
			if maxQty, err := strconv.ParseFloat(filter.MaxQty, 64); err == nil {
				req.MarketMaxQty = maxQty
			}
			if stepSize, err := strconv.ParseFloat(filter.StepSize, 64); err == nil {
				req.MarketStepSize = stepSize
			}

		case "MIN_NOTIONAL":
			if notional, err := strconv.ParseFloat(filter.Notional, 64); err == nil && notional > 0 {
				req.MinNotional = notional
			}
		}
	}

	return req
}

// calculatePrecisionFromStep calculates decimal precision from step size
func calculatePrecisionFromStep(stepSize float64) int {
	if stepSize >= 1 {
		return 0
	}
	precision := 0
	for stepSize < 1 && precision < 10 {
		stepSize *= 10
		precision++
	}
	return precision
}

// GetRequirements returns requirements for a symbol (from cache or fetches if needed)
func (sv *SymbolValidator) GetRequirements(symbol string) (*database.SymbolRequirements, error) {
	sv.mu.RLock()
	if req, ok := sv.cache[symbol]; ok {
		sv.mu.RUnlock()
		return req, nil
	}
	sv.mu.RUnlock()

	// Not in cache - try to fetch from database
	if sv.repo != nil {
		ctx := context.Background()
		req, err := sv.repo.GetBySymbol(ctx, symbol)
		if err == nil && req != nil {
			sv.mu.Lock()
			sv.cache[symbol] = req
			sv.mu.Unlock()
			return req, nil
		}
	}

	// Not in database - try to fetch from Binance (for new symbols)
	if sv.futuresClient != nil {
		log.Printf("[SYMBOL-VALIDATOR] Symbol %s not cached, fetching from Binance...", symbol)
		ctx := context.Background()
		if err := sv.syncFromBinance(ctx); err == nil {
			sv.mu.RLock()
			if req, ok := sv.cache[symbol]; ok {
				sv.mu.RUnlock()
				return req, nil
			}
			sv.mu.RUnlock()
		}
	}

	return nil, fmt.Errorf("symbol %s not found", symbol)
}

// ValidateOrder performs comprehensive order validation
func (sv *SymbolValidator) ValidateOrder(symbol string, quantity, price float64, isMarketOrder bool) *SymbolValidationResult {
	result := &SymbolValidationResult{
		Symbol:        symbol,
		OriginalQty:   quantity,
		OriginalPrice: price,
		Valid:         true,
	}

	req, err := sv.GetRequirements(symbol)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, &ValidationError{
			Symbol:  symbol,
			Field:   "symbol",
			Message: fmt.Sprintf("Symbol requirements not found: %v", err),
		})
		return result
	}

	// Round quantity to proper precision
	result.RoundedQty = sv.RoundQuantity(symbol, quantity)

	// Round price to proper precision (if not market order)
	if !isMarketOrder && price > 0 {
		result.RoundedPrice = sv.RoundPrice(symbol, price)
	}

	// Validate minimum quantity
	minQty := req.MinQty
	if isMarketOrder && req.MarketMinQty > 0 {
		minQty = req.MarketMinQty
	}
	if result.RoundedQty < minQty {
		result.Valid = false
		result.Errors = append(result.Errors, &ValidationError{
			Symbol:   symbol,
			Field:    "quantity",
			Value:    result.RoundedQty,
			Required: fmt.Sprintf(">= %.8f", minQty),
			Message:  "Quantity below minimum",
		})
	}

	// Validate maximum quantity
	maxQty := req.MaxQty
	if isMarketOrder && req.MarketMaxQty > 0 {
		maxQty = req.MarketMaxQty
	}
	if result.RoundedQty > maxQty {
		result.Valid = false
		result.Errors = append(result.Errors, &ValidationError{
			Symbol:   symbol,
			Field:    "quantity",
			Value:    result.RoundedQty,
			Required: fmt.Sprintf("<= %.8f", maxQty),
			Message:  "Quantity above maximum",
		})
	}

	// Validate step size
	if req.StepSize > 0 {
		remainder := math.Mod(result.RoundedQty, req.StepSize)
		if remainder > 1e-10 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Quantity %.8f not aligned with step size %.8f", result.RoundedQty, req.StepSize))
		}
	}

	// Validate price (for limit orders)
	if !isMarketOrder && price > 0 {
		if req.MinPrice > 0 && result.RoundedPrice < req.MinPrice {
			result.Valid = false
			result.Errors = append(result.Errors, &ValidationError{
				Symbol:   symbol,
				Field:    "price",
				Value:    result.RoundedPrice,
				Required: fmt.Sprintf(">= %.8f", req.MinPrice),
				Message:  "Price below minimum",
			})
		}
		if req.MaxPrice > 0 && result.RoundedPrice > req.MaxPrice {
			result.Valid = false
			result.Errors = append(result.Errors, &ValidationError{
				Symbol:   symbol,
				Field:    "price",
				Value:    result.RoundedPrice,
				Required: fmt.Sprintf("<= %.8f", req.MaxPrice),
				Message:  "Price above maximum",
			})
		}
	}

	// Validate notional value (quantity * price)
	if price > 0 && req.MinNotional > 0 {
		notional := result.RoundedQty * price
		if notional < req.MinNotional {
			result.Valid = false
			result.Errors = append(result.Errors, &ValidationError{
				Symbol:   symbol,
				Field:    "notional",
				Value:    notional,
				Required: fmt.Sprintf(">= %.2f", req.MinNotional),
				Message:  fmt.Sprintf("Order value $%.2f below minimum $%.2f", notional, req.MinNotional),
			})
		}
	}

	return result
}

// RoundQuantity rounds a quantity to the symbol's required precision
func (sv *SymbolValidator) RoundQuantity(symbol string, qty float64) float64 {
	req, err := sv.GetRequirements(symbol)
	if err != nil {
		// Fallback to floor with 0 decimals (safest)
		return math.Floor(qty)
	}

	// Use step size for precise rounding
	if req.StepSize > 0 {
		return math.Floor(qty/req.StepSize) * req.StepSize
	}

	// Fallback to precision-based rounding
	multiplier := math.Pow(10, float64(req.QuantityPrecision))
	return math.Floor(qty*multiplier) / multiplier
}

// RoundPrice rounds a price to the symbol's required precision
func (sv *SymbolValidator) RoundPrice(symbol string, price float64) float64 {
	req, err := sv.GetRequirements(symbol)
	if err != nil {
		// Fallback to 4 decimal places
		return math.Round(price*10000) / 10000
	}

	// Use tick size for precise rounding
	if req.TickSize > 0 {
		return math.Round(price/req.TickSize) * req.TickSize
	}

	// Fallback to precision-based rounding
	multiplier := math.Pow(10, float64(req.PricePrecision))
	return math.Round(price*multiplier) / multiplier
}

// GetQuantityPrecision returns the quantity precision for a symbol
func (sv *SymbolValidator) GetQuantityPrecision(symbol string) int {
	req, err := sv.GetRequirements(symbol)
	if err != nil {
		return 0 // Safest default
	}
	return req.QuantityPrecision
}

// GetPricePrecision returns the price precision for a symbol
func (sv *SymbolValidator) GetPricePrecision(symbol string) int {
	req, err := sv.GetRequirements(symbol)
	if err != nil {
		return 4 // Safe default
	}
	return req.PricePrecision
}

// GetMinQuantity returns the minimum quantity for a symbol
func (sv *SymbolValidator) GetMinQuantity(symbol string) float64 {
	req, err := sv.GetRequirements(symbol)
	if err != nil {
		return 1 // Safe default
	}
	return req.MinQty
}

// GetMinNotional returns the minimum notional value for a symbol
func (sv *SymbolValidator) GetMinNotional(symbol string) float64 {
	req, err := sv.GetRequirements(symbol)
	if err != nil {
		return 5 // Safe default
	}
	return req.MinNotional
}

// IsInitialized returns true if the validator has been initialized
func (sv *SymbolValidator) IsInitialized() bool {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.initialized
}

// GetCacheSize returns the number of symbols in cache
func (sv *SymbolValidator) GetCacheSize() int {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return len(sv.cache)
}

// RefreshCache forces a refresh of the symbol cache
func (sv *SymbolValidator) RefreshCache() error {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	ctx := context.Background()
	return sv.syncFromBinance(ctx)
}

// StartPeriodicSync starts a background goroutine to sync periodically
func (sv *SymbolValidator) StartPeriodicSync() {
	go func() {
		ticker := time.NewTicker(sv.syncInterval)
		defer ticker.Stop()

		for range ticker.C {
			sv.mu.Lock()
			ctx := context.Background()
			if err := sv.syncFromBinance(ctx); err != nil {
				log.Printf("[SYMBOL-VALIDATOR] Periodic sync failed: %v", err)
			}
			sv.mu.Unlock()
		}
	}()
	log.Printf("[SYMBOL-VALIDATOR] Started periodic sync (every %v)", sv.syncInterval)
}
