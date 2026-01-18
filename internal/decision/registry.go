// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.6: Strategy Registry
package decision

import (
	"fmt"
	"sync"
	"time"
)

// RegistryError represents an error from the strategy registry.
type RegistryError struct {
	Code    string
	Message string
}

func (e *RegistryError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Common registry errors
var (
	ErrStrategyNotFound      = &RegistryError{Code: "NOT_FOUND", Message: "strategy not found in registry"}
	ErrStrategyAlreadyExists = &RegistryError{Code: "ALREADY_EXISTS", Message: "strategy already registered"}
	ErrStrategyNil           = &RegistryError{Code: "NIL_STRATEGY", Message: "strategy cannot be nil"}
	ErrNoStrategiesForRegime = &RegistryError{Code: "NO_STRATEGIES", Message: "no strategies available for regime"}
)

// StrategyRegistry maintains a registry of available trading strategies.
// It provides thread-safe access to strategies and supports regime-based lookup.
type StrategyRegistry struct {
	// strategies maps strategy names to their implementations
	strategies map[string]Strategy
	mu         sync.RWMutex

	// strategyInfo caches metadata about each strategy
	strategyInfo map[string]*StrategyInfo

	// regimeIndex maps each regime to strategies that support it
	regimeIndex map[MarketRegime][]string

	// defaultStrategy is used when no specific strategy is selected
	defaultStrategy string

	// onRegisterCallbacks are called when a strategy is registered
	onRegisterCallbacks []func(Strategy)
	callbackMu          sync.RWMutex
}

// NewStrategyRegistry creates a new empty StrategyRegistry.
func NewStrategyRegistry() *StrategyRegistry {
	return &StrategyRegistry{
		strategies:          make(map[string]Strategy),
		strategyInfo:        make(map[string]*StrategyInfo),
		regimeIndex:         make(map[MarketRegime][]string),
		onRegisterCallbacks: make([]func(Strategy), 0),
	}
}

// RegisterStrategy adds a strategy to the registry.
// Returns an error if the strategy is nil or already registered.
func (r *StrategyRegistry) RegisterStrategy(strategy Strategy) error {
	if strategy == nil {
		return ErrStrategyNil
	}

	name := strategy.Name()
	if name == "" {
		return &RegistryError{Code: "INVALID_NAME", Message: "strategy name cannot be empty"}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate
	if _, exists := r.strategies[name]; exists {
		return ErrStrategyAlreadyExists
	}

	// Register the strategy
	r.strategies[name] = strategy

	// Create and cache strategy info
	info := NewStrategyInfo(strategy)
	r.strategyInfo[name] = info

	// Build regime index
	for _, regime := range strategy.SupportedRegimes() {
		r.regimeIndex[regime] = append(r.regimeIndex[regime], name)
	}

	// Set as default if this is the first strategy
	if r.defaultStrategy == "" {
		r.defaultStrategy = name
	}

	// Call callbacks (outside the lock to prevent deadlocks)
	go r.notifyRegisterCallbacks(strategy)

	return nil
}

// notifyRegisterCallbacks notifies all registered callbacks of a new strategy.
func (r *StrategyRegistry) notifyRegisterCallbacks(strategy Strategy) {
	r.callbackMu.RLock()
	callbacks := make([]func(Strategy), len(r.onRegisterCallbacks))
	copy(callbacks, r.onRegisterCallbacks)
	r.callbackMu.RUnlock()

	for _, callback := range callbacks {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					// Log panic but don't crash on callback panic
					// In production, this should use a proper logger
					fmt.Printf("[WARN] Strategy registration callback panicked: %v\n", rec)
				}
			}()
			callback(strategy)
		}()
	}
}

// UnregisterStrategy removes a strategy from the registry.
// Returns an error if the strategy is not found.
func (r *StrategyRegistry) UnregisterStrategy(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	strategy, exists := r.strategies[name]
	if !exists {
		return ErrStrategyNotFound
	}

	// Remove from main map
	delete(r.strategies, name)
	delete(r.strategyInfo, name)

	// Remove from regime index
	for _, regime := range strategy.SupportedRegimes() {
		r.removeFromRegimeIndex(regime, name)
	}

	// Update default if necessary
	if r.defaultStrategy == name {
		r.defaultStrategy = r.findFirstStrategy()
	}

	return nil
}

// removeFromRegimeIndex removes a strategy from the regime index.
// Must be called with lock held.
func (r *StrategyRegistry) removeFromRegimeIndex(regime MarketRegime, name string) {
	strategies := r.regimeIndex[regime]
	for i, s := range strategies {
		if s == name {
			r.regimeIndex[regime] = append(strategies[:i], strategies[i+1:]...)
			return
		}
	}
}

// findFirstStrategy returns the name of any registered strategy.
// Must be called with lock held.
func (r *StrategyRegistry) findFirstStrategy() string {
	for name := range r.strategies {
		return name
	}
	return ""
}

// GetStrategy returns a strategy by name.
// Returns an error if the strategy is not found.
func (r *StrategyRegistry) GetStrategy(name string) (Strategy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	strategy, exists := r.strategies[name]
	if !exists {
		return nil, ErrStrategyNotFound
	}

	return strategy, nil
}

// GetStrategyInfo returns metadata about a strategy.
// Returns an error if the strategy is not found.
func (r *StrategyRegistry) GetStrategyInfo(name string) (*StrategyInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.strategyInfo[name]
	if !exists {
		return nil, ErrStrategyNotFound
	}

	// Return a copy to prevent external modification
	infoCopy := *info
	return &infoCopy, nil
}

// GetStrategiesForRegime returns all strategies that support the given regime.
// Returns an empty slice if no strategies support the regime.
func (r *StrategyRegistry) GetStrategiesForRegime(regime MarketRegime) []Strategy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.regimeIndex[regime]
	if len(names) == 0 {
		return []Strategy{}
	}

	strategies := make([]Strategy, 0, len(names))
	for _, name := range names {
		if strategy, exists := r.strategies[name]; exists {
			strategies = append(strategies, strategy)
		}
	}

	return strategies
}

// GetStrategyNamesForRegime returns the names of strategies that support the given regime.
func (r *StrategyRegistry) GetStrategyNamesForRegime(regime MarketRegime) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.regimeIndex[regime]
	if len(names) == 0 {
		return []string{}
	}

	// Return a copy
	result := make([]string, len(names))
	copy(result, names)
	return result
}

// ListStrategies returns information about all registered strategies.
func (r *StrategyRegistry) ListStrategies() []*StrategyInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*StrategyInfo, 0, len(r.strategyInfo))
	for _, info := range r.strategyInfo {
		// Return copies to prevent external modification
		infoCopy := *info
		result = append(result, &infoCopy)
	}

	return result
}

// ListStrategyNames returns the names of all registered strategies.
func (r *StrategyRegistry) ListStrategyNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.strategies))
	for name := range r.strategies {
		names = append(names, name)
	}

	return names
}

// Count returns the number of registered strategies.
func (r *StrategyRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.strategies)
}

// HasStrategy checks if a strategy is registered.
func (r *StrategyRegistry) HasStrategy(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.strategies[name]
	return exists
}

// GetDefaultStrategy returns the default strategy.
// Returns an error if no strategies are registered.
func (r *StrategyRegistry) GetDefaultStrategy() (Strategy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultStrategy == "" {
		return nil, &RegistryError{Code: "NO_DEFAULT", Message: "no default strategy set"}
	}

	strategy, exists := r.strategies[r.defaultStrategy]
	if !exists {
		return nil, ErrStrategyNotFound
	}

	return strategy, nil
}

// SetDefaultStrategy sets the default strategy.
// Returns an error if the strategy is not registered.
func (r *StrategyRegistry) SetDefaultStrategy(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.strategies[name]; !exists {
		return ErrStrategyNotFound
	}

	r.defaultStrategy = name
	return nil
}

// GetDefaultStrategyName returns the name of the default strategy.
func (r *StrategyRegistry) GetDefaultStrategyName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.defaultStrategy
}

// OnRegister adds a callback that is called when a strategy is registered.
func (r *StrategyRegistry) OnRegister(callback func(Strategy)) {
	if callback == nil {
		return
	}

	r.callbackMu.Lock()
	defer r.callbackMu.Unlock()

	r.onRegisterCallbacks = append(r.onRegisterCallbacks, callback)
}

// Clear removes all registered strategies.
func (r *StrategyRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.strategies = make(map[string]Strategy)
	r.strategyInfo = make(map[string]*StrategyInfo)
	r.regimeIndex = make(map[MarketRegime][]string)
	r.defaultStrategy = ""
}

// ErrNilCoinState is returned when a nil CoinState is passed to scoring functions.
var ErrNilCoinState = &RegistryError{Code: "NIL_STATE", Message: "coin state cannot be nil"}

// SelectBestStrategy selects the best strategy for the given state and regime.
// It evaluates all strategies for the regime and returns the one with the highest score.
// Returns the strategy name, score, and any error.
func (r *StrategyRegistry) SelectBestStrategy(state *CoinState, regime MarketRegime) (string, *StrategyScore, error) {
	if state == nil {
		return "", nil, ErrNilCoinState
	}

	strategies := r.GetStrategiesForRegime(regime)
	if len(strategies) == 0 {
		// Fall back to default strategy
		defaultStrategy, err := r.GetDefaultStrategy()
		if err != nil {
			return "", nil, ErrNoStrategiesForRegime
		}
		strategies = []Strategy{defaultStrategy}
	}

	var bestName string
	var bestScore *StrategyScore

	for _, strategy := range strategies {
		score := strategy.CalculateScore(state)
		if bestScore == nil || score.FinalScore > bestScore.FinalScore {
			bestName = strategy.Name()
			bestScore = &score
		}
	}

	return bestName, bestScore, nil
}

// EvaluateAllStrategies evaluates all registered strategies against the state.
// Returns a map of strategy names to their scores.
// Returns an empty map if state is nil.
func (r *StrategyRegistry) EvaluateAllStrategies(state *CoinState) map[string]*StrategyScore {
	if state == nil {
		return make(map[string]*StrategyScore)
	}

	r.mu.RLock()
	strategies := make([]Strategy, 0, len(r.strategies))
	for _, strategy := range r.strategies {
		strategies = append(strategies, strategy)
	}
	r.mu.RUnlock()

	results := make(map[string]*StrategyScore, len(strategies))
	for _, strategy := range strategies {
		score := strategy.CalculateScore(state)
		results[strategy.Name()] = &score
	}

	return results
}

// EnableStrategy enables a strategy for use.
func (r *StrategyRegistry) EnableStrategy(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.strategyInfo[name]
	if !exists {
		return ErrStrategyNotFound
	}

	info.Enabled = true
	return nil
}

// DisableStrategy disables a strategy.
func (r *StrategyRegistry) DisableStrategy(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.strategyInfo[name]
	if !exists {
		return ErrStrategyNotFound
	}

	info.Enabled = false
	return nil
}

// IsEnabled checks if a strategy is enabled.
func (r *StrategyRegistry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.strategyInfo[name]
	if !exists {
		return false
	}

	return info.Enabled
}

// GetEnabledStrategies returns only enabled strategies.
func (r *StrategyRegistry) GetEnabledStrategies() []Strategy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Strategy, 0)
	for name, strategy := range r.strategies {
		if info, exists := r.strategyInfo[name]; exists && info.Enabled {
			result = append(result, strategy)
		}
	}

	return result
}

// GetEnabledStrategiesForRegime returns enabled strategies that support the regime.
func (r *StrategyRegistry) GetEnabledStrategiesForRegime(regime MarketRegime) []Strategy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.regimeIndex[regime]
	if len(names) == 0 {
		return []Strategy{}
	}

	strategies := make([]Strategy, 0, len(names))
	for _, name := range names {
		if info, exists := r.strategyInfo[name]; exists && info.Enabled {
			if strategy, exists := r.strategies[name]; exists {
				strategies = append(strategies, strategy)
			}
		}
	}

	return strategies
}

// RegistryStats provides statistics about the registry.
type RegistryStats struct {
	TotalStrategies   int                    `json:"total_strategies"`
	EnabledStrategies int                    `json:"enabled_strategies"`
	StrategiesByRegime map[MarketRegime]int  `json:"strategies_by_regime"`
	DefaultStrategy   string                 `json:"default_strategy"`
	LastUpdated       int64                  `json:"last_updated"`
}

// GetStats returns statistics about the registry.
func (r *StrategyRegistry) GetStats() *RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &RegistryStats{
		TotalStrategies:    len(r.strategies),
		EnabledStrategies:  0,
		StrategiesByRegime: make(map[MarketRegime]int),
		DefaultStrategy:    r.defaultStrategy,
		LastUpdated:        time.Now().UnixMilli(),
	}

	for _, info := range r.strategyInfo {
		if info.Enabled {
			stats.EnabledStrategies++
		}
	}

	for regime, names := range r.regimeIndex {
		stats.StrategiesByRegime[regime] = len(names)
	}

	return stats
}

// globalRegistry is the default global strategy registry.
var globalRegistry = NewStrategyRegistry()

// GetGlobalRegistry returns the global strategy registry.
func GetGlobalRegistry() *StrategyRegistry {
	return globalRegistry
}

// RegisterGlobalStrategy registers a strategy in the global registry.
func RegisterGlobalStrategy(strategy Strategy) error {
	return globalRegistry.RegisterStrategy(strategy)
}

// GetGlobalStrategy gets a strategy from the global registry.
func GetGlobalStrategy(name string) (Strategy, error) {
	return globalRegistry.GetStrategy(name)
}
