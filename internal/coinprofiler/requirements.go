// Package coinprofiler provides real-time coin data collection via WebSocket
// for the Chain Trading System.
package coinprofiler

import (
	"context"
	"sort"
	"sync"
)

// ============================================================================
// STORY 14.2: Strategy Requirement Aggregation
// ============================================================================
// This file aggregates data requirements from all enabled strategies.
// The Coin Profiler uses these requirements to determine what data to collect.
//
// Flow:
// 1. Read enabled strategies from database
// 2. Extract timeframe requirements per strategy
// 3. Extract indicator/data requirements per strategy
// 4. Combine and deduplicate requirements
// 5. Support mode+strategy combinations (same strategy, different modes)
// ============================================================================

// ============================================================================
// TYPES
// ============================================================================

// StrategyRequirements defines the data requirements for a single enabled strategy.
// Each strategy specifies what timeframes and data fields it needs from the Coin Profiler.
type StrategyRequirements struct {
	Mode        string                 `json:"mode"`         // "scalp", "swing", "position", "ultra_fast"
	Strategy    string                 `json:"strategy"`     // Strategy group: "breakout", "trending", "range", "volatile"
	SubStrategy string                 `json:"sub_strategy"` // e.g., "ravindra_volume_imbalance", "classic_breakout"
	Timeframes  []string               `json:"timeframes"`   // ["5m", "15m", "1h"]
	DataFields  []string               `json:"data_fields"`  // ["volume", "taker_buy_volume", "ohlc"]
	Filters     map[string]interface{} `json:"filters"`      // min_volume, etc.
}

// AggregatedRequirements combines all strategy requirements into a deduplicated set.
// This is what the Coin Profiler uses to know what data to subscribe to.
type AggregatedRequirements struct {
	// AllTimeframes is the deduplicated list of all required timeframes across all strategies.
	AllTimeframes []string `json:"all_timeframes"`

	// AllDataFields is the deduplicated list of all required data fields across all strategies.
	AllDataFields []string `json:"all_data_fields"`

	// ByTimeframe maps each timeframe to the list of strategies that need it.
	// Used for efficient lookup when data arrives.
	ByTimeframe map[string][]StrategyRef `json:"by_timeframe"`

	// ByStrategy contains the full requirements for each strategy (for reference).
	ByStrategy []StrategyRequirements `json:"by_strategy"`

	// TotalStrategies is the count of enabled strategies.
	TotalStrategies int `json:"total_strategies"`

	// GlobalFilters contains filters that apply to all subscriptions.
	GlobalFilters map[string]interface{} `json:"global_filters"`
}

// StrategyRef is a compact reference to a strategy, used in ByTimeframe mapping.
type StrategyRef struct {
	Mode        string `json:"mode"`
	Strategy    string `json:"strategy"`
	SubStrategy string `json:"sub_strategy"`
}

// ============================================================================
// STRATEGY DATA REQUIREMENTS REGISTRY
// ============================================================================

// StrategyDataConfig holds the data requirements for a known sub-strategy type.
// This is the source of truth for what each strategy needs.
type StrategyDataConfig struct {
	// Timeframes required by this strategy (varies by mode)
	TimeframesByMode map[string][]string `json:"timeframes_by_mode"`

	// DataFields required (e.g., volume, taker_buy_volume, ohlc)
	DataFields []string `json:"data_fields"`

	// DefaultFilters applied when no user override exists
	DefaultFilters map[string]interface{} `json:"default_filters"`
}

// strategyRegistryMu protects concurrent access to strategyRegistry.
var strategyRegistryMu sync.RWMutex

// strategyRegistry maps sub-strategy names to their data requirements.
// This is populated at init time with known strategies.
// All access must be protected by strategyRegistryMu.
var strategyRegistry = map[string]*StrategyDataConfig{
	// Volume Imbalance (Ravindra's 3-Step Model)
	// Reference: internal/autopilot/volume_imbalance_strategy.go
	// BACKTESTED Dec 2025 - Jan 2026: 51 trades, 47.1% WR, +1147% net return
	"ravindra_volume_imbalance": {
		TimeframesByMode: map[string][]string{
			"scalp":      {"3m"},        // BACKTESTED - was "15m", faster timeframe works better
			"swing":      {"1h"},        // From VolumeImbalanceConfig
			"position":   {"4h"},        // From VolumeImbalanceConfig
			"ultra_fast": {"1m", "3m"},  // Shorter for ultra fast
		},
		DataFields: []string{
			"ohlc",             // Open, High, Low, Close for pattern detection
			"volume",           // Total volume for spike detection
			"taker_buy_volume", // Institutional buying detection
		},
		DefaultFilters: map[string]interface{}{
			// BACKTESTED VALUES
			"min_volume_spike_multiplier": 3.0,   // Was 2.0
			"lookback_period":             5,     // Was 20
			"min_consolidation_candles":   1,     // Was 2
			"max_consolidation_candles":   999,   // Was 6
			"breakout_volume_surge":       1.0,   // Was 1.5
			"entry_volume_vs_reference":   1.0,   // New
			"max_sl_percent":              1.5,   // New
		},
	},

	// Classic Breakout Strategy
	"classic_breakout": {
		TimeframesByMode: map[string][]string{
			"scalp":      {"5m", "15m"},
			"swing":      {"15m", "1h"},
			"position":   {"1h", "4h"},
			"ultra_fast": {"1m", "5m"},
		},
		DataFields: []string{
			"ohlc",   // For support/resistance levels
			"volume", // For breakout confirmation
		},
		DefaultFilters: map[string]interface{}{
			"min_consolidation_candles": 5,
		},
	},

	// Trend Following (Score-based)
	"trend_following": {
		TimeframesByMode: map[string][]string{
			"scalp":      {"5m", "15m"},
			"swing":      {"1h", "4h"},
			"position":   {"4h", "1d"},
			"ultra_fast": {"1m", "5m"},
		},
		DataFields: []string{
			"ohlc",   // For trend calculation
			"volume", // For trend strength
		},
		DefaultFilters: map[string]interface{}{
			"min_trend_strength": 0.6,
		},
	},

	// Mean Reversion Strategy
	"mean_reversion": {
		TimeframesByMode: map[string][]string{
			"scalp":      {"5m", "15m"},
			"swing":      {"1h", "4h"},
			"position":   {"4h", "1d"},
			"ultra_fast": {"1m", "5m"},
		},
		DataFields: []string{
			"ohlc",   // For price deviation calculation
			"volume", // For liquidity check
		},
		DefaultFilters: map[string]interface{}{
			"bollinger_period": 20,
			"std_dev":          2.0,
		},
	},

	// Range Trading Strategy
	"range_trading": {
		TimeframesByMode: map[string][]string{
			"scalp":      {"5m", "15m"},
			"swing":      {"1h", "4h"},
			"position":   {"4h", "1d"},
			"ultra_fast": {"1m", "5m"},
		},
		DataFields: []string{
			"ohlc",   // For range detection
			"volume", // For breakout filtering
		},
		DefaultFilters: map[string]interface{}{
			"min_range_width": 0.02,
		},
	},
}

// ============================================================================
// REQUIREMENT EXTRACTION
// ============================================================================

// EnabledStrategyReader is an interface for reading enabled strategies.
// This allows dependency injection for testing.
type EnabledStrategyReader interface {
	// GetEnabledStrategies returns all enabled sub-strategies for a user.
	GetEnabledStrategies(ctx context.Context, userID string) ([]EnabledSubStrategy, error)
}

// EnabledSubStrategy represents a compact enabled strategy reference.
// Matches the database.EnabledSubStrategy type.
type EnabledSubStrategy struct {
	Mode          string `json:"mode"`
	StrategyGroup string `json:"strategy_group"`
	SubStrategy   string `json:"sub_strategy"`
}

// RequirementAggregator aggregates strategy requirements from enabled strategies.
type RequirementAggregator struct {
	reader EnabledStrategyReader
}

// NewRequirementAggregator creates a new aggregator with the given strategy reader.
func NewRequirementAggregator(reader EnabledStrategyReader) *RequirementAggregator {
	return &RequirementAggregator{reader: reader}
}

// GetStrategyRequirements extracts data requirements for a single enabled strategy.
// It looks up the strategy in the registry and returns its requirements for the given mode.
// This function is thread-safe and returns copies of internal data to prevent corruption.
func GetStrategyRequirements(mode, strategy, subStrategy string) *StrategyRequirements {
	strategyRegistryMu.RLock()
	config, exists := strategyRegistry[subStrategy]
	strategyRegistryMu.RUnlock()

	if !exists {
		// Unknown strategy - return minimal requirements
		return &StrategyRequirements{
			Mode:        mode,
			Strategy:    strategy,
			SubStrategy: subStrategy,
			Timeframes:  getDefaultTimeframesForMode(mode),
			DataFields:  []string{"ohlc", "volume"},
			Filters:     make(map[string]interface{}),
		}
	}

	// Get timeframes for this mode (need lock again to access config safely)
	strategyRegistryMu.RLock()
	defer strategyRegistryMu.RUnlock()

	timeframes, ok := config.TimeframesByMode[mode]
	if !ok {
		// Fallback to scalp timeframes
		timeframes = config.TimeframesByMode["scalp"]
		if timeframes == nil {
			timeframes = getDefaultTimeframesForMode(mode)
		}
	}

	// Copy timeframes to prevent caller from corrupting registry
	timeframesCopy := make([]string, len(timeframes))
	copy(timeframesCopy, timeframes)

	// Copy data fields to prevent caller from corrupting registry
	dataFieldsCopy := make([]string, len(config.DataFields))
	copy(dataFieldsCopy, config.DataFields)

	// Copy filters
	filters := make(map[string]interface{})
	for k, v := range config.DefaultFilters {
		filters[k] = v
	}

	return &StrategyRequirements{
		Mode:        mode,
		Strategy:    strategy,
		SubStrategy: subStrategy,
		Timeframes:  timeframesCopy,
		DataFields:  dataFieldsCopy,
		Filters:     filters,
	}
}

// getDefaultTimeframesForMode returns default timeframes when a strategy is unknown.
func getDefaultTimeframesForMode(mode string) []string {
	switch mode {
	case "ultra_fast":
		return []string{"1m", "5m"}
	case "scalp":
		return []string{"5m", "15m"}
	case "swing":
		return []string{"1h", "4h"}
	case "position":
		return []string{"4h", "1d"}
	default:
		return []string{"15m", "1h"}
	}
}

// GetRequirementsForStrategies extracts requirements for a list of enabled strategies.
func GetRequirementsForStrategies(strategies []EnabledSubStrategy) []StrategyRequirements {
	requirements := make([]StrategyRequirements, 0, len(strategies))

	for _, s := range strategies {
		req := GetStrategyRequirements(s.Mode, s.StrategyGroup, s.SubStrategy)
		if req != nil {
			requirements = append(requirements, *req)
		}
	}

	return requirements
}

// AggregateRequirements combines multiple strategy requirements into a deduplicated set.
// This is the main entry point for aggregating all enabled strategies' requirements.
func AggregateRequirements(strategies []StrategyRequirements) *AggregatedRequirements {
	agg := &AggregatedRequirements{
		ByTimeframe:     make(map[string][]StrategyRef),
		ByStrategy:      strategies,
		TotalStrategies: len(strategies),
		GlobalFilters:   make(map[string]interface{}),
	}

	// Use maps for deduplication
	timeframeSet := make(map[string]bool)
	dataFieldSet := make(map[string]bool)

	for _, req := range strategies {
		stratRef := StrategyRef{
			Mode:        req.Mode,
			Strategy:    req.Strategy,
			SubStrategy: req.SubStrategy,
		}

		// Collect timeframes
		for _, tf := range req.Timeframes {
			timeframeSet[tf] = true
			agg.ByTimeframe[tf] = append(agg.ByTimeframe[tf], stratRef)
		}

		// Collect data fields
		for _, df := range req.DataFields {
			dataFieldSet[df] = true
		}
	}

	// Convert sets to sorted slices
	agg.AllTimeframes = sortedKeys(timeframeSet)
	agg.AllDataFields = sortedKeys(dataFieldSet)

	return agg
}

// Aggregate fetches enabled strategies and aggregates their requirements.
// This is the high-level method used by the Coin Profiler.
func (ra *RequirementAggregator) Aggregate(ctx context.Context, userID string) (*AggregatedRequirements, error) {
	// Fetch enabled strategies from database
	enabledStrategies, err := ra.reader.GetEnabledStrategies(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Extract requirements for each strategy
	requirements := GetRequirementsForStrategies(enabledStrategies)

	// Aggregate and deduplicate
	return AggregateRequirements(requirements), nil
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// sortedKeys returns the keys of a map as a sorted slice.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TimeframePriority returns a numeric priority for timeframe sorting.
// Lower values are shorter timeframes.
func TimeframePriority(tf string) int {
	priorities := map[string]int{
		"1m":  1,
		"3m":  2,
		"5m":  3,
		"15m": 4,
		"30m": 5,
		"1h":  6,
		"2h":  7,
		"4h":  8,
		"6h":  9,
		"8h":  10,
		"12h": 11,
		"1d":  12,
		"3d":  13,
		"1w":  14,
		"1M":  15,
	}

	if p, ok := priorities[tf]; ok {
		return p
	}
	return 100 // Unknown timeframes go to the end
}

// SortTimeframes sorts timeframes by their priority (shortest first).
func SortTimeframes(timeframes []string) []string {
	sorted := make([]string, len(timeframes))
	copy(sorted, timeframes)
	sort.Slice(sorted, func(i, j int) bool {
		return TimeframePriority(sorted[i]) < TimeframePriority(sorted[j])
	})
	return sorted
}

// GetRegisteredStrategies returns a list of all registered sub-strategy names.
// Useful for validation and debugging. Thread-safe.
func GetRegisteredStrategies() []string {
	strategyRegistryMu.RLock()
	defer strategyRegistryMu.RUnlock()

	strategies := make([]string, 0, len(strategyRegistry))
	for name := range strategyRegistry {
		strategies = append(strategies, name)
	}
	sort.Strings(strategies)
	return strategies
}

// RegisterStrategy adds or updates a strategy in the registry.
// This allows runtime registration of new strategies. Thread-safe.
func RegisterStrategy(name string, config *StrategyDataConfig) {
	if config != nil {
		strategyRegistryMu.Lock()
		defer strategyRegistryMu.Unlock()
		strategyRegistry[name] = config
	}
}

// IsStrategyRegistered checks if a sub-strategy is registered in the registry. Thread-safe.
func IsStrategyRegistered(subStrategy string) bool {
	strategyRegistryMu.RLock()
	defer strategyRegistryMu.RUnlock()
	_, exists := strategyRegistry[subStrategy]
	return exists
}

// GetStrategyConfig returns a deep copy of the data configuration for a registered strategy.
// Returns nil if not found. Thread-safe.
func GetStrategyConfig(subStrategy string) *StrategyDataConfig {
	strategyRegistryMu.RLock()
	defer strategyRegistryMu.RUnlock()

	config, exists := strategyRegistry[subStrategy]
	if !exists {
		return nil
	}

	// Return a deep copy to prevent corruption
	copiedConfig := &StrategyDataConfig{
		TimeframesByMode: make(map[string][]string),
		DataFields:       make([]string, len(config.DataFields)),
		DefaultFilters:   make(map[string]interface{}),
	}

	for mode, tfs := range config.TimeframesByMode {
		copiedTfs := make([]string, len(tfs))
		copy(copiedTfs, tfs)
		copiedConfig.TimeframesByMode[mode] = copiedTfs
	}

	copy(copiedConfig.DataFields, config.DataFields)

	for k, v := range config.DefaultFilters {
		copiedConfig.DefaultFilters[k] = v
	}

	return copiedConfig
}

// ============================================================================
// STORY 14.3: Position Requirement Aggregation
// ============================================================================
// This section aggregates data requirements from open positions for Exit Decision.
// Even when Trading is OFF, the Coin Profiler still collects data for positions
// so that TP/SL/Trailing exits can be monitored.
//
// Flow:
// 1. Read open positions from GinieAutopilot
// 2. Extract symbols being held
// 3. Determine timeframes needed for exit monitoring (TP/SL/Trailing)
// 4. Combine with strategy requirements
// ============================================================================

// ExitMode indicates what type of exit monitoring a position requires.
type ExitMode string

const (
	// ExitModeTPSL indicates position has take profit and/or stop loss orders.
	ExitModeTPSL ExitMode = "tp_sl"
	// ExitModeTrailing indicates position has trailing stop active.
	ExitModeTrailing ExitMode = "trailing"
	// ExitModeBoth indicates position has both TP/SL and trailing stop.
	ExitModeBoth ExitMode = "both"
)

// PositionRequirements defines the data requirements for monitoring an open position.
// Used by the Exit Decision system to know what data the Coin Profiler should collect.
type PositionRequirements struct {
	// Symbol is the trading pair (e.g., "BTCUSDT")
	Symbol string `json:"symbol"`

	// Timeframes are the candle intervals needed for exit monitoring.
	// Faster timeframes are used for more responsive exit monitoring.
	Timeframes []string `json:"timeframes"`

	// ExitMode indicates what type of exit monitoring is active.
	ExitMode ExitMode `json:"exit_mode"`

	// Mode is the trading mode of the position (scalp, swing, position, ultra_fast).
	Mode string `json:"mode"`

	// Side is the position direction ("LONG" or "SHORT").
	Side string `json:"side"`
}

// Position represents the minimal position data needed for requirement extraction.
// This interface allows mocking without depending on the full GiniePosition type.
type Position interface {
	GetSymbol() string
	GetMode() string
	GetSide() string
	GetTimeframe() string // Entry timeframe from strategy (e.g., "3m", "1h")
	HasTakeProfit() bool
	HasStopLoss() bool
	IsTrailingActive() bool
}

// SimplePosition is a concrete implementation of Position for testing and simple use cases.
type SimplePosition struct {
	Symbol         string `json:"symbol"`
	Mode           string `json:"mode"`
	Side           string `json:"side"`
	Timeframe      string `json:"timeframe"`
	TakeProfit     bool   `json:"take_profit"`
	StopLoss       bool   `json:"stop_loss"`
	TrailingActive bool   `json:"trailing_active"`
}

// GetSymbol returns the position's symbol.
func (p *SimplePosition) GetSymbol() string { return p.Symbol }

// GetMode returns the position's trading mode.
func (p *SimplePosition) GetMode() string { return p.Mode }

// GetSide returns the position's direction.
func (p *SimplePosition) GetSide() string { return p.Side }

// GetTimeframe returns the position's entry timeframe.
func (p *SimplePosition) GetTimeframe() string { return p.Timeframe }

// HasTakeProfit returns whether the position has take profit orders.
func (p *SimplePosition) HasTakeProfit() bool { return p.TakeProfit }

// HasStopLoss returns whether the position has a stop loss.
func (p *SimplePosition) HasStopLoss() bool { return p.StopLoss }

// IsTrailingActive returns whether trailing stop is active.
func (p *SimplePosition) IsTrailingActive() bool { return p.TrailingActive }

// GiniePositionSource is an interface that abstracts access to GiniePosition data.
// This allows the coin profiler to work without directly depending on the autopilot package.
type GiniePositionSource interface {
	GetSymbol() string
	GetMode() string
	GetSide() string
	GetTimeframe() string
	GetTakeProfitPrice() float64
	GetStopLossPrice() float64
	IsTrailingStopActive() bool
}

// GiniePositionAdapter wraps a GiniePositionSource to implement the Position interface.
// This adapter bridges the GiniePosition from autopilot package with the Position interface.
type GiniePositionAdapter struct {
	source GiniePositionSource
}

// NewGiniePositionAdapter creates a new adapter from a GiniePositionSource.
func NewGiniePositionAdapter(source GiniePositionSource) *GiniePositionAdapter {
	return &GiniePositionAdapter{source: source}
}

// GetSymbol returns the position's symbol.
func (a *GiniePositionAdapter) GetSymbol() string {
	if a == nil || a.source == nil {
		return ""
	}
	return a.source.GetSymbol()
}

// GetMode returns the position's trading mode.
func (a *GiniePositionAdapter) GetMode() string {
	if a == nil || a.source == nil {
		return ""
	}
	return a.source.GetMode()
}

// GetSide returns the position's direction.
func (a *GiniePositionAdapter) GetSide() string {
	if a == nil || a.source == nil {
		return ""
	}
	return a.source.GetSide()
}

// GetTimeframe returns the position's entry timeframe.
func (a *GiniePositionAdapter) GetTimeframe() string {
	if a == nil || a.source == nil {
		return ""
	}
	return a.source.GetTimeframe()
}

// HasTakeProfit returns whether the position has a take profit order.
func (a *GiniePositionAdapter) HasTakeProfit() bool {
	if a == nil || a.source == nil {
		return false
	}
	return a.source.GetTakeProfitPrice() > 0
}

// HasStopLoss returns whether the position has a stop loss order.
func (a *GiniePositionAdapter) HasStopLoss() bool {
	if a == nil || a.source == nil {
		return false
	}
	return a.source.GetStopLossPrice() > 0
}

// IsTrailingActive returns whether trailing stop is active.
func (a *GiniePositionAdapter) IsTrailingActive() bool {
	if a == nil || a.source == nil {
		return false
	}
	return a.source.IsTrailingStopActive()
}

// AdaptGiniePositions converts a slice of GiniePositionSource to Position interface slice.
// This is a helper function for integrating with the autopilot package.
func AdaptGiniePositions(sources []GiniePositionSource) []Position {
	positions := make([]Position, 0, len(sources))
	for _, source := range sources {
		if source != nil {
			positions = append(positions, NewGiniePositionAdapter(source))
		}
	}
	return positions
}

// PositionReader is an interface for reading open positions.
// This allows dependency injection for testing without depending on GinieAutopilot.
type PositionReader interface {
	// GetOpenPositions returns all currently open positions.
	GetOpenPositions() []Position
}

// getExitTimeframesForMode returns the timeframes needed for exit monitoring
// based on the position's trading mode. Exit monitoring typically needs faster
// timeframes than entry scanning to react quickly to price movements.
func getExitTimeframesForMode(mode string) []string {
	switch mode {
	case "ultra_fast":
		return []string{"1m", "5m"}
	case "scalp":
		return []string{"5m", "15m"}
	case "swing":
		return []string{"15m", "1h"}
	case "position":
		return []string{"1h", "4h"}
	default:
		// Default to scalp timeframes for unknown modes
		return []string{"5m", "15m"}
	}
}

// detectExitMode determines what type of exit monitoring a position needs
// based on its TP/SL and trailing stop configuration.
func detectExitMode(pos Position) ExitMode {
	hasTPSL := pos.HasTakeProfit() || pos.HasStopLoss()
	hasTrailing := pos.IsTrailingActive()

	if hasTPSL && hasTrailing {
		return ExitModeBoth
	}
	if hasTrailing {
		return ExitModeTrailing
	}
	// Default to TP/SL mode - all positions need at least price monitoring
	return ExitModeTPSL
}

// GetPositionRequirements extracts data requirements from a list of positions.
// This function determines what symbols and timeframes the Coin Profiler needs
// to subscribe to for exit monitoring.
func GetPositionRequirements(positions []Position) []PositionRequirements {
	if len(positions) == 0 {
		return []PositionRequirements{}
	}

	requirements := make([]PositionRequirements, 0, len(positions))

	for _, pos := range positions {
		if pos == nil {
			continue
		}

		symbol := pos.GetSymbol()
		if symbol == "" {
			continue
		}

		mode := pos.GetMode()
		if mode == "" {
			mode = "scalp" // Default to scalp for unknown modes
		}

		// Use entry timeframe from strategy if available (exact match for position monitoring)
		// Fall back to mode-based exit timeframes only when entry timeframe is not known
		entryTimeframe := pos.GetTimeframe()
		var timeframes []string
		if entryTimeframe != "" {
			timeframes = []string{entryTimeframe}
		} else {
			timeframes = getExitTimeframesForMode(mode)
		}

		req := PositionRequirements{
			Symbol:     symbol,
			Timeframes: timeframes,
			ExitMode:   detectExitMode(pos),
			Mode:       mode,
			Side:       pos.GetSide(),
		}

		requirements = append(requirements, req)
	}

	return requirements
}

// CombinedRequirements represents the merged requirements from both
// strategies (for entry) and positions (for exit).
type CombinedRequirements struct {
	// AllSymbols is the deduplicated list of all symbols to track.
	AllSymbols []string `json:"all_symbols"`

	// AllTimeframes is the deduplicated list of all required timeframes.
	AllTimeframes []string `json:"all_timeframes"`

	// AllDataFields is the deduplicated list of all required data fields.
	AllDataFields []string `json:"all_data_fields"`

	// BySymbol maps each symbol to its combined requirements.
	BySymbol map[string]*SymbolRequirements `json:"by_symbol"`

	// StrategyCount is the number of enabled strategies.
	StrategyCount int `json:"strategy_count"`

	// PositionCount is the number of open positions.
	PositionCount int `json:"position_count"`
}

// SymbolRequirements holds the combined requirements for a single symbol.
type SymbolRequirements struct {
	// Symbol is the trading pair.
	Symbol string `json:"symbol"`

	// Timeframes is the deduplicated list of timeframes needed for this symbol.
	Timeframes []string `json:"timeframes"`

	// DataFields is the deduplicated list of data fields needed.
	DataFields []string `json:"data_fields"`

	// Source indicates why this symbol is being tracked.
	Source DataSource `json:"source"`

	// Strategies lists which strategies need this symbol (for entry).
	Strategies []StrategyRef `json:"strategies,omitempty"`

	// Positions lists position details (for exit).
	Positions []PositionRequirements `json:"positions,omitempty"`
}

// CombineRequirements merges strategy requirements with position requirements
// into a unified set for the Coin Profiler to subscribe to.
func CombineRequirements(strategyReqs *AggregatedRequirements, positionReqs []PositionRequirements) *CombinedRequirements {
	combined := &CombinedRequirements{
		BySymbol: make(map[string]*SymbolRequirements),
	}

	// Track all unique values
	symbolSet := make(map[string]bool)
	timeframeSet := make(map[string]bool)
	dataFieldSet := make(map[string]bool)

	// Process strategy requirements (these don't have symbols yet - strategies apply to ALL coins)
	// Note: In practice, the Coin Profiler will get the symbol list from coin scanning
	// For now, we just aggregate the timeframes and data fields from strategies
	if strategyReqs != nil {
		combined.StrategyCount = strategyReqs.TotalStrategies

		for _, tf := range strategyReqs.AllTimeframes {
			timeframeSet[tf] = true
		}
		for _, df := range strategyReqs.AllDataFields {
			dataFieldSet[df] = true
		}
	}

	// Process position requirements - these have specific symbols
	for _, posReq := range positionReqs {
		combined.PositionCount++
		symbolSet[posReq.Symbol] = true

		// Get or create symbol requirements
		symReq, exists := combined.BySymbol[posReq.Symbol]
		if !exists {
			symReq = &SymbolRequirements{
				Symbol:     posReq.Symbol,
				Timeframes: []string{},
				DataFields: []string{"ohlc", "volume"}, // Default data fields for exit monitoring
				Source:     DataSourcePosition,
				Positions:  []PositionRequirements{},
			}
			combined.BySymbol[posReq.Symbol] = symReq
		} else {
			// Symbol already exists (from strategy scanning) - position takes priority.
			// Override to position source so entry decision engine stops scanning for
			// new entries on this symbol (prevents duplicate orders).
			if symReq.Source == DataSourceStrategy {
				symReq.Source = DataSourcePosition
			}
		}

		// Add position to the list
		symReq.Positions = append(symReq.Positions, posReq)

		// Merge timeframes
		tfSet := make(map[string]bool)
		for _, tf := range symReq.Timeframes {
			tfSet[tf] = true
		}
		for _, tf := range posReq.Timeframes {
			tfSet[tf] = true
			timeframeSet[tf] = true
		}
		symReq.Timeframes = sortedKeys(tfSet)
	}

	// Add default data fields for exit monitoring
	dataFieldSet["ohlc"] = true
	dataFieldSet["volume"] = true

	// Convert sets to sorted slices
	combined.AllSymbols = sortedKeys(symbolSet)
	combined.AllTimeframes = sortedKeys(timeframeSet)
	combined.AllDataFields = sortedKeys(dataFieldSet)

	return combined
}

// AddSymbolFromStrategy adds a symbol that was found through strategy scanning
// to the combined requirements. This is called when the entry system finds
// potential entry candidates.
func (cr *CombinedRequirements) AddSymbolFromStrategy(symbol string, strategies []StrategyRef, timeframes []string, dataFields []string) {
	if symbol == "" {
		return
	}

	symReq, exists := cr.BySymbol[symbol]
	if !exists {
		symReq = &SymbolRequirements{
			Symbol:     symbol,
			Timeframes: []string{},
			DataFields: []string{},
			Source:     DataSourceStrategy,
			Strategies: []StrategyRef{},
		}
		cr.BySymbol[symbol] = symReq

		// Add to AllSymbols if not present
		found := false
		for _, s := range cr.AllSymbols {
			if s == symbol {
				found = true
				break
			}
		}
		if !found {
			cr.AllSymbols = append(cr.AllSymbols, symbol)
			sort.Strings(cr.AllSymbols)
		}
	} else {
		// Symbol already exists from position - keep as position source only.
		// This prevents the entry decision engine from trying to place a second entry
		// on a symbol that already has an active position.
		if symReq.Source == DataSourcePosition {
			return // Skip - position symbols should not also scan for new entries
		}
	}

	// Merge strategies
	symReq.Strategies = append(symReq.Strategies, strategies...)

	// Merge timeframes
	tfSet := make(map[string]bool)
	for _, tf := range symReq.Timeframes {
		tfSet[tf] = true
	}
	for _, tf := range timeframes {
		tfSet[tf] = true
	}
	symReq.Timeframes = sortedKeys(tfSet)

	// Merge data fields
	dfSet := make(map[string]bool)
	for _, df := range symReq.DataFields {
		dfSet[df] = true
	}
	for _, df := range dataFields {
		dfSet[df] = true
	}
	symReq.DataFields = sortedKeys(dfSet)
}

// GetSymbolsForSource returns all symbols that match the given data source.
func (cr *CombinedRequirements) GetSymbolsForSource(source DataSource) []string {
	symbols := []string{}
	for symbol, req := range cr.BySymbol {
		if req.Source == source {
			symbols = append(symbols, symbol)
		}
	}
	sort.Strings(symbols)
	return symbols
}
