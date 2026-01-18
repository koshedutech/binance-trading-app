// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.13: Indicator Calculation Engine
package indicators

import (
	"fmt"
	"sync"
	"time"
)

// CalculationEngine orchestrates indicator calculations for symbols.
// It integrates with SegmentCalculator and uses caching for efficiency.
// Thread-safe for concurrent access.
type CalculationEngine struct {
	mu         sync.RWMutex
	registry   *IndicatorRegistry
	cache      *IndicatorCache
	calculator *SegmentCalculator
	settings   *IndicatorSettings

	// Performance tracking
	lastCalcDuration time.Duration
	totalCalcs       int64
	totalDuration    time.Duration
}

// EngineConfig holds configuration for the CalculationEngine.
type EngineConfig struct {
	// Registry is the indicator registry to use. If nil, uses global registry.
	Registry *IndicatorRegistry
	// Cache is the indicator cache to use. If nil, creates a new cache.
	Cache *IndicatorCache
	// Settings is the indicator settings. If nil, uses defaults.
	Settings *IndicatorSettings
	// CacheTTL is the TTL for cached indicator values.
	CacheTTL time.Duration
}

// DefaultEngineConfig returns sensible defaults for the engine.
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		Registry: nil,                 // Use global
		Cache:    nil,                 // Create new
		Settings: nil,                 // Use defaults
		CacheTTL: 5 * time.Second,
	}
}

// NewCalculationEngine creates a new CalculationEngine with the given config.
// If config is nil, uses default configuration.
func NewCalculationEngine(config *EngineConfig) *CalculationEngine {
	if config == nil {
		config = DefaultEngineConfig()
	}

	registry := config.Registry
	if registry == nil {
		registry = GetGlobalIndicatorRegistry()
	}

	cache := config.Cache
	if cache == nil {
		cache = NewIndicatorCache(config.CacheTTL)
	}

	settings := config.Settings
	if settings == nil {
		settings = DefaultIndicatorSettings()
	}

	return &CalculationEngine{
		registry:   registry,
		cache:      cache,
		calculator: NewSegmentCalculator(registry),
		settings:   settings,
	}
}

// NewCalculationEngineSimple creates a CalculationEngine with minimal configuration.
// Uses global registry, default cache, and default settings.
func NewCalculationEngineSimple() *CalculationEngine {
	return NewCalculationEngine(nil)
}

// CalculationResult holds the results of calculating indicators for a symbol.
type CalculationResult struct {
	// Symbol is the trading pair (e.g., "BTCUSDT").
	Symbol string `json:"symbol"`
	// SegmentResults contains the calculated segment scores.
	SegmentResults map[IndicatorSegment]*SegmentResult `json:"segment_results"`
	// AllIndicators contains all calculated indicator values.
	AllIndicators map[string]*IndicatorValue `json:"all_indicators"`
	// CacheHits is the number of values retrieved from cache.
	CacheHits int `json:"cache_hits"`
	// CacheMisses is the number of values calculated fresh.
	CacheMisses int `json:"cache_misses"`
	// Duration is how long the calculation took.
	Duration time.Duration `json:"duration"`
	// CalculatedAt is when the calculation completed (Unix milliseconds).
	CalculatedAt int64 `json:"calculated_at"`
	// Errors lists any errors encountered during calculation.
	Errors []string `json:"errors,omitempty"`
}

// NewCalculationResult creates a new CalculationResult.
func NewCalculationResult(symbol string) *CalculationResult {
	return &CalculationResult{
		Symbol:         symbol,
		SegmentResults: make(map[IndicatorSegment]*SegmentResult),
		AllIndicators:  make(map[string]*IndicatorValue),
		CalculatedAt:   time.Now().UnixMilli(),
	}
}

// AddError records an error that occurred during calculation.
func (cr *CalculationResult) AddError(err string) {
	cr.Errors = append(cr.Errors, err)
}

// HasErrors returns true if any errors occurred.
func (cr *CalculationResult) HasErrors() bool {
	return len(cr.Errors) > 0
}

// GetSegmentScore returns the score for a segment, or 50 if not found.
func (cr *CalculationResult) GetSegmentScore(segment IndicatorSegment) float64 {
	if result, ok := cr.SegmentResults[segment]; ok {
		return result.Score
	}
	return 50 // Neutral
}

// GetTotalScore returns the sum of all segment scores.
func (cr *CalculationResult) GetTotalScore() float64 {
	var total float64
	for _, result := range cr.SegmentResults {
		total += result.Score
	}
	return total
}

// CalculateForSymbol calculates all indicators for a symbol using the provided data.
// Uses cache when available and caches new calculations.
func (ce *CalculationEngine) CalculateForSymbol(symbol string, data *IndicatorData) (*CalculationResult, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	start := time.Now()
	result := NewCalculationResult(symbol)

	ce.mu.RLock()
	settings := ce.settings.Clone()
	ce.mu.RUnlock()

	// Get all selected indicators across all segments
	allIndicators := settings.AllSelectedIndicators()

	// Check cache for existing values
	cached := ce.cache.GetMultiple(symbol, allIndicators)
	result.CacheHits = len(cached)
	result.CacheMisses = len(allIndicators) - len(cached)

	// Add cached values to data.OtherValues for dependency resolution
	for name, value := range cached {
		data.SetOtherValue(name, value.Value)
		result.AllIndicators[name] = value
	}

	// Calculate missing indicators
	missingIndicators := ce.findMissingIndicators(allIndicators, cached)

	if len(missingIndicators) > 0 {
		// Resolve dependencies and get calculation order
		orderedIndicators, err := ce.registry.ResolveDependencies(missingIndicators)
		if err != nil {
			result.AddError(fmt.Sprintf("dependency resolution failed: %v", err))
		} else {
			// Calculate each missing indicator
			for _, ind := range orderedIndicators {
				// Skip if already cached
				if _, ok := cached[ind.Name()]; ok {
					continue
				}

				value, err := ind.Calculate(data)
				if err != nil {
					result.AddError(fmt.Sprintf("%s: %v", ind.Name(), err))
					continue
				}

				// Add to results
				result.AllIndicators[ind.Name()] = value

				// Add to data for dependent indicators
				data.SetOtherValue(ind.Name(), value.Value)

				// Cache the result
				ce.cache.Set(symbol, ind.Name(), value)
			}
		}
	}

	// Calculate segment scores
	segmentResults, err := ce.calculator.CalculateAllFromSettings(settings, data)
	if err != nil {
		result.AddError(fmt.Sprintf("segment calculation failed: %v", err))
	} else {
		result.SegmentResults = segmentResults
	}

	// Record timing
	result.Duration = time.Since(start)
	result.CalculatedAt = time.Now().UnixMilli()

	// Update performance stats
	ce.mu.Lock()
	ce.lastCalcDuration = result.Duration
	ce.totalCalcs++
	ce.totalDuration += result.Duration
	ce.mu.Unlock()

	return result, nil
}

// findMissingIndicators returns indicators not in the cached set.
func (ce *CalculationEngine) findMissingIndicators(all []string, cached map[string]*IndicatorValue) []string {
	missing := make([]string, 0, len(all))
	for _, name := range all {
		if _, ok := cached[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

// CalculateWithIncrementalUpdate calculates indicators with incremental update logic.
// Uses updateType to determine what needs recalculation.
func (ce *CalculationEngine) CalculateWithIncrementalUpdate(symbol string, data *IndicatorData, updateType UpdateType) (*CalculationResult, error) {
	// Invalidate appropriate cache entries based on update type
	ce.cache.InvalidateForUpdate(symbol, updateType)

	// Now calculate with potentially partial cache
	return ce.CalculateForSymbol(symbol, data)
}

// OnPriceTick handles a price tick update for a symbol.
// Only recalculates price-sensitive indicators.
func (ce *CalculationEngine) OnPriceTick(symbol string, data *IndicatorData) (*CalculationResult, error) {
	return ce.CalculateWithIncrementalUpdate(symbol, data, UpdatePriceTick)
}

// OnCandleClose handles a candle close update for a symbol.
// Triggers full recalculation of all indicators.
func (ce *CalculationEngine) OnCandleClose(symbol string, data *IndicatorData) (*CalculationResult, error) {
	return ce.CalculateWithIncrementalUpdate(symbol, data, UpdateCandleClose)
}

// CalculateSegment calculates a single segment's indicators.
// Useful when you only need one segment's score.
func (ce *CalculationEngine) CalculateSegment(symbol string, segment IndicatorSegment, data *IndicatorData) (*SegmentResult, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	ce.mu.RLock()
	settings := ce.settings.Clone()
	ce.mu.RUnlock()

	config := settings.GetSegmentConfig(segment)
	if config == nil {
		return nil, fmt.Errorf("segment config not found: %s", segment)
	}

	// Get cached values for this segment
	cached := ce.cache.GetMultiple(symbol, config.SelectedIndicators)

	// Add to data.OtherValues
	for name, value := range cached {
		data.SetOtherValue(name, value.Value)
	}

	// Calculate segment
	result, err := ce.calculator.Calculate(config, data)
	if err != nil {
		return nil, err
	}

	// Cache any new values from the calculation
	for name, score := range result.IndividualScores {
		if _, ok := cached[name]; !ok {
			// Create IndicatorValue from result
			raw := 0.0
			if v, ok := result.RawValues[name]; ok {
				raw = v
			}
			value := NewIndicatorValue(name, raw, score)
			ce.cache.Set(symbol, name, value)
		}
	}

	return result, nil
}

// CalculateIndicators calculates specific indicators only.
// Useful for selective calculation without segment scoring.
func (ce *CalculationEngine) CalculateIndicators(symbol string, indicatorNames []string, data *IndicatorData) (map[string]*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	results := make(map[string]*IndicatorValue)

	// Check cache first
	cached := ce.cache.GetMultiple(symbol, indicatorNames)
	for name, value := range cached {
		results[name] = value
		data.SetOtherValue(name, value.Value)
	}

	// Find missing
	missing := ce.findMissingIndicators(indicatorNames, cached)
	if len(missing) == 0 {
		return results, nil
	}

	// Resolve dependencies and calculate
	ordered, err := ce.registry.ResolveDependencies(missing)
	if err != nil {
		return results, err
	}

	for _, ind := range ordered {
		if _, ok := cached[ind.Name()]; ok {
			continue
		}

		value, err := ind.Calculate(data)
		if err != nil {
			continue
		}

		results[ind.Name()] = value
		data.SetOtherValue(ind.Name(), value.Value)
		ce.cache.Set(symbol, ind.Name(), value)
	}

	return results, nil
}

// GetSettings returns the current settings.
func (ce *CalculationEngine) GetSettings() *IndicatorSettings {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.settings.Clone()
}

// SetSettings updates the indicator settings.
func (ce *CalculationEngine) SetSettings(settings *IndicatorSettings) {
	if settings == nil {
		return
	}

	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.settings = settings.Clone()
}

// GetCache returns the underlying cache for direct access.
func (ce *CalculationEngine) GetCache() *IndicatorCache {
	return ce.cache
}

// GetRegistry returns the underlying registry.
func (ce *CalculationEngine) GetRegistry() *IndicatorRegistry {
	return ce.registry
}

// InvalidateCache invalidates all cached values for a symbol.
func (ce *CalculationEngine) InvalidateCache(symbol string) {
	ce.cache.InvalidateBySymbol(symbol)
}

// InvalidateCacheAll invalidates all cached values.
func (ce *CalculationEngine) InvalidateCacheAll() {
	ce.cache.InvalidateAll()
}

// GetCacheStats returns cache performance statistics.
func (ce *CalculationEngine) GetCacheStats() CacheStats {
	return ce.cache.GetStats()
}

// EnginePerformanceStats holds engine performance metrics.
type EnginePerformanceStats struct {
	TotalCalculations int64         `json:"total_calculations"`
	AverageDuration   time.Duration `json:"average_duration_ns"`
	LastDuration      time.Duration `json:"last_duration_ns"`
	CacheStats        CacheStats    `json:"cache_stats"`
}

// GetPerformanceStats returns engine performance statistics.
func (ce *CalculationEngine) GetPerformanceStats() EnginePerformanceStats {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	var avgDuration time.Duration
	if ce.totalCalcs > 0 {
		avgDuration = ce.totalDuration / time.Duration(ce.totalCalcs)
	}

	return EnginePerformanceStats{
		TotalCalculations: ce.totalCalcs,
		AverageDuration:   avgDuration,
		LastDuration:      ce.lastCalcDuration,
		CacheStats:        ce.cache.GetStats(),
	}
}

// ResetStats resets all performance statistics.
func (ce *CalculationEngine) ResetStats() {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	ce.totalCalcs = 0
	ce.totalDuration = 0
	ce.lastCalcDuration = 0
	ce.cache.ResetStats()
}

// WarmupCache pre-calculates indicators for a symbol.
// Useful to ensure cache is populated before trading begins.
func (ce *CalculationEngine) WarmupCache(symbol string, data *IndicatorData) error {
	_, err := ce.CalculateForSymbol(symbol, data)
	return err
}

// WarmupCacheMultiple warms the cache for multiple symbols concurrently.
func (ce *CalculationEngine) WarmupCacheMultiple(symbolsData map[string]*IndicatorData) map[string]error {
	var wg sync.WaitGroup
	mu := sync.Mutex{}
	errors := make(map[string]error)

	for symbol, data := range symbolsData {
		wg.Add(1)
		go func(s string, d *IndicatorData) {
			defer wg.Done()
			if err := ce.WarmupCache(s, d); err != nil {
				mu.Lock()
				errors[s] = err
				mu.Unlock()
			}
		}(symbol, data)
	}

	wg.Wait()
	return errors
}

// BatchCalculation holds multiple calculation requests.
type BatchCalculation struct {
	Symbol string
	Data   *IndicatorData
}

// CalculateBatch calculates indicators for multiple symbols.
// Returns a map of symbol to result.
func (ce *CalculationEngine) CalculateBatch(requests []BatchCalculation) map[string]*CalculationResult {
	results := make(map[string]*CalculationResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, req := range requests {
		wg.Add(1)
		go func(r BatchCalculation) {
			defer wg.Done()
			result, err := ce.CalculateForSymbol(r.Symbol, r.Data)
			if err != nil {
				result = NewCalculationResult(r.Symbol)
				result.AddError(err.Error())
			}
			mu.Lock()
			results[r.Symbol] = result
			mu.Unlock()
		}(req)
	}

	wg.Wait()
	return results
}

// MeetsPerformanceTarget checks if the engine meets the < 5ms target.
func (ce *CalculationEngine) MeetsPerformanceTarget() bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if ce.totalCalcs == 0 {
		return true // No data yet
	}

	avgDuration := ce.totalDuration / time.Duration(ce.totalCalcs)
	return avgDuration < 5*time.Millisecond
}

// CheckHealth performs a health check on the engine.
func (ce *CalculationEngine) CheckHealth() map[string]interface{} {
	stats := ce.GetPerformanceStats()
	cacheStats := ce.GetCacheStats()

	health := map[string]interface{}{
		"status":                 "healthy",
		"total_calculations":     stats.TotalCalculations,
		"average_duration_ms":    float64(stats.AverageDuration) / float64(time.Millisecond),
		"last_duration_ms":       float64(stats.LastDuration) / float64(time.Millisecond),
		"meets_5ms_target":       ce.MeetsPerformanceTarget(),
		"cache_size":             cacheStats.Size,
		"cache_hit_rate":         ce.cache.HitRate(),
		"registered_indicators":  ce.registry.Count(),
	}

	if !ce.MeetsPerformanceTarget() {
		health["status"] = "degraded"
		health["warning"] = "Average calculation time exceeds 5ms target"
	}

	return health
}
