// Package entrydecision provides the Strategy Matcher for the Entry Decision System.
// Epic 14: Chain Trading System - Story 14.11: Strategy Reader & Matcher
package entrydecision

import (
	"context"
	"fmt"
	"sync"
	"time"

	"binance-trading-bot/internal/coinprofiler"
)

// ============================================================================
// STRATEGY MATCHER INTERFACE
// ============================================================================

// StrategyMatcher orchestrates pattern and score matching across all enabled strategies.
// It determines which strategies apply to which coins and aggregates the results.
type StrategyMatcher interface {
	// MatchAll matches all enabled strategies against the provided coin data.
	// Returns complete entry decision response with all strategies and their matching coins.
	MatchAll(ctx context.Context, coins map[string]*coinprofiler.CoinData) (*EntryDecisionResponse, error)

	// MatchForMode matches strategies for a specific trading mode.
	// Returns strategies grouped for that mode.
	MatchForMode(ctx context.Context, mode string, coins map[string]*coinprofiler.CoinData) (*ModeStrategies, error)

	// GetReadyCandidates returns all coins that are ready for entry across all strategies.
	// This is the main API for finding actionable entry opportunities.
	GetReadyCandidates(ctx context.Context) (*EntryCandidatesResponse, error)
}

// ============================================================================
// COIN DATA PROVIDER INTERFACE
// ============================================================================

// CoinDataProvider provides coin data for strategy matching.
// This abstracts the coin profiler or any other data source.
type CoinDataProvider interface {
	// GetAllCoinData returns all available coin data.
	GetAllCoinData(ctx context.Context) (map[string]*coinprofiler.CoinData, error)

	// GetCoinData returns data for a specific symbol.
	GetCoinData(ctx context.Context, symbol string) (*coinprofiler.CoinData, error)
}

// ============================================================================
// DEFAULT STRATEGY MATCHER IMPLEMENTATION
// ============================================================================

// DefaultStrategyMatcher implements StrategyMatcher.
type DefaultStrategyMatcher struct {
	userID         string
	strategyReader StrategyReader
	patternMatcher *VolumeImbalancePatternMatcher
	scoreCalc      *TrendFollowingScoreCalculator
	coinProvider   CoinDataProvider

	mu sync.RWMutex

	// Cached enabled strategies (refreshed periodically)
	enabledStrategies []EnabledSubStrategy
	lastRefresh       time.Time
	refreshInterval   time.Duration
}

// StrategyMatcherConfig holds configuration for the strategy matcher.
type StrategyMatcherConfig struct {
	UserID          string
	RefreshInterval time.Duration // How often to refresh enabled strategies list
}

// DefaultStrategyMatcherConfig returns default configuration.
func DefaultStrategyMatcherConfig(userID string) *StrategyMatcherConfig {
	return &StrategyMatcherConfig{
		UserID:          userID,
		RefreshInterval: 5 * time.Minute,
	}
}

// NewDefaultStrategyMatcher creates a new DefaultStrategyMatcher.
func NewDefaultStrategyMatcher(
	config *StrategyMatcherConfig,
	strategyReader StrategyReader,
	patternMatcher *VolumeImbalancePatternMatcher,
	scoreCalc *TrendFollowingScoreCalculator,
	coinProvider CoinDataProvider,
) *DefaultStrategyMatcher {
	if config == nil {
		config = DefaultStrategyMatcherConfig("")
	}

	return &DefaultStrategyMatcher{
		userID:          config.UserID,
		strategyReader:  strategyReader,
		patternMatcher:  patternMatcher,
		scoreCalc:       scoreCalc,
		coinProvider:    coinProvider,
		refreshInterval: config.RefreshInterval,
	}
}

// MatchAll matches all enabled strategies against the provided coin data.
func (m *DefaultStrategyMatcher) MatchAll(ctx context.Context, coins map[string]*coinprofiler.CoinData) (*EntryDecisionResponse, error) {
	// Refresh enabled strategies if needed
	if err := m.refreshEnabledStrategies(ctx); err != nil {
		return nil, fmt.Errorf("failed to refresh enabled strategies: %w", err)
	}

	response := NewEntryDecisionResponse()

	// Process each enabled strategy
	for _, strategy := range m.enabledStrategies {
		strategyType := GetStrategyType(strategy.StrategyGroup, strategy.SubStrategy)

		var strategyMatch *StrategyMatch
		var err error

		switch strategyType {
		case StrategyTypePattern:
			strategyMatch, err = m.matchPatternStrategy(ctx, strategy, coins)
		case StrategyTypeScore:
			strategyMatch, err = m.matchScoreStrategy(ctx, strategy, coins)
		default:
			// Skip unknown strategy types
			continue
		}

		if err != nil {
			// Log error but continue with other strategies
			continue
		}

		if strategyMatch != nil {
			response.AddStrategy(*strategyMatch)
		}
	}

	// Group by mode for hierarchical view
	response.GroupByMode()
	response.CalculateSummary()

	return response, nil
}

// MatchForMode matches strategies for a specific trading mode.
func (m *DefaultStrategyMatcher) MatchForMode(ctx context.Context, mode string, coins map[string]*coinprofiler.CoinData) (*ModeStrategies, error) {
	if !IsValidMode(mode) {
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	// Get enabled strategies for this mode
	strategies, err := m.strategyReader.GetEnabledStrategiesForMode(ctx, m.userID, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled strategies for mode %s: %w", mode, err)
	}

	modeStrategies := NewModeStrategies(mode, true)

	// Process each strategy for this mode
	for _, strategy := range strategies {
		strategyType := GetStrategyType(strategy.StrategyGroup, strategy.SubStrategy)

		var strategyMatch *StrategyMatch
		var matchErr error

		switch strategyType {
		case StrategyTypePattern:
			strategyMatch, matchErr = m.matchPatternStrategy(ctx, strategy, coins)
		case StrategyTypeScore:
			strategyMatch, matchErr = m.matchScoreStrategy(ctx, strategy, coins)
		default:
			continue
		}

		if matchErr != nil {
			continue
		}

		if strategyMatch != nil {
			modeStrategies.AddStrategy(*strategyMatch)
		}
	}

	return modeStrategies, nil
}

// GetReadyCandidates returns all coins that are ready for entry across all strategies.
func (m *DefaultStrategyMatcher) GetReadyCandidates(ctx context.Context) (*EntryCandidatesResponse, error) {
	// Get all coin data from provider
	var coins map[string]*coinprofiler.CoinData
	var err error

	if m.coinProvider != nil {
		coins, err = m.coinProvider.GetAllCoinData(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get coin data: %w", err)
		}
	}

	// Match all strategies
	response, err := m.MatchAll(ctx, coins)
	if err != nil {
		return nil, fmt.Errorf("failed to match strategies: %w", err)
	}

	// Extract ready candidates
	candidates := NewEntryCandidatesResponse()

	for _, strategy := range response.Strategies {
		for _, coin := range strategy.GetReadyCoins() {
			candidate := EntryCandidate{
				Symbol:       coin.Symbol,
				CurrentPrice: coin.CurrentPrice,
				Mode:         strategy.Mode,
				Strategy:     strategy.Strategy,
				SubStrategy:  strategy.SubStrategy,
				Type:         strategy.Type,
				Timeframe:    strategy.Timeframe,
				Direction:    coin.Direction,
				UpdatedAt:    coin.UpdatedAt,
			}

			// Set type-specific fields
			if strategy.Type == StrategyTypePattern {
				candidate.Step = coin.Step
				candidate.Status = coin.Status
				candidate.Details = coin.Details
			} else {
				candidate.Score = coin.Score
				candidate.Threshold = strategy.Threshold
			}

			// Calculate priority (higher = more urgent)
			candidate.Priority = m.calculatePriority(coin, strategy)

			candidates.AddCandidate(candidate)
		}
	}

	return candidates, nil
}

// ============================================================================
// INTERNAL MATCHING METHODS
// ============================================================================

// matchPatternStrategy matches a pattern-based strategy against coin data.
func (m *DefaultStrategyMatcher) matchPatternStrategy(
	ctx context.Context,
	strategy EnabledSubStrategy,
	coins map[string]*coinprofiler.CoinData,
) (*StrategyMatch, error) {
	if m.patternMatcher == nil {
		return nil, fmt.Errorf("pattern matcher not initialized")
	}

	// Create the strategy match structure
	sm := NewStrategyMatch(
		strategy.Mode,
		strategy.StrategyGroup,
		strategy.SubStrategy,
		StrategyTypePattern,
		strategy.Timeframe,
	)

	sm.WithTimeframes([]string{strategy.Timeframe})
	sm.WithRequirements(&StrategyRequirements{
		Timeframes:  []string{strategy.Timeframe},
		DataFields:  []string{"volume", "ohlc"},
		Description: "Pattern-based strategy requiring step completion",
	})

	// Match each coin against the pattern
	for symbol, coinData := range coins {
		if coinData == nil {
			continue
		}

		// Convert coin data to candles for pattern matching
		candles := m.coinDataToCandles(coinData, strategy.Timeframe)
		if len(candles) == 0 {
			continue
		}

		// Match the pattern
		coinMatch := m.patternMatcher.MatchPattern(symbol, strategy.Mode, strategy.Timeframe, candles)
		if coinMatch != nil {
			// Enrich with price data
			coinMatch.CurrentPrice = coinData.Price
			coinMatch.Volume24h = coinData.Volume24h
			sm.AddCoin(*coinMatch)
		}
	}

	return sm, nil
}

// matchScoreStrategy matches a score-based strategy against coin data.
func (m *DefaultStrategyMatcher) matchScoreStrategy(
	ctx context.Context,
	strategy EnabledSubStrategy,
	coins map[string]*coinprofiler.CoinData,
) (*StrategyMatch, error) {
	if m.scoreCalc == nil {
		return nil, fmt.Errorf("score calculator not initialized")
	}

	// Create the strategy match structure
	sm := NewStrategyMatch(
		strategy.Mode,
		strategy.StrategyGroup,
		strategy.SubStrategy,
		StrategyTypeScore,
		strategy.Timeframe,
	)

	config := m.scoreCalc.GetConfig()
	sm.WithThreshold(config.Threshold)
	sm.WithTimeframes([]string{strategy.Timeframe})
	sm.WithRequirements(&StrategyRequirements{
		Timeframes:  []string{strategy.Timeframe},
		DataFields:  []string{"price", "volume", "indicators"},
		Description: "Score-based strategy with weighted indicator scoring",
	})

	// Calculate scores for each coin
	for symbol, coinData := range coins {
		result, err := m.scoreCalc.CalculateScore(symbol, coinData)
		if err != nil {
			// Skip coins with calculation errors
			continue
		}

		if result != nil {
			coinMatch := result.ToCoinMatch()
			if coinMatch != nil {
				// Enrich with additional data
				if coinData != nil {
					coinMatch.CurrentPrice = coinData.Price
					coinMatch.Volume24h = coinData.Volume24h
				}
				sm.AddCoin(*coinMatch)
			}
		}
	}

	return sm, nil
}

// coinDataToCandles converts CoinData timeframe data to Candle slices for pattern matching.
func (m *DefaultStrategyMatcher) coinDataToCandles(coinData *coinprofiler.CoinData, timeframe string) []Candle {
	if coinData == nil || coinData.Timeframes == nil {
		return nil
	}

	tfData, ok := coinData.Timeframes[timeframe]
	if !ok || tfData == nil {
		return nil
	}

	// For now, create a single candle from the current timeframe data
	// In a full implementation, this would retrieve historical candle data
	candle := Candle{
		Time:           tfData.UpdatedAt,
		Open:           tfData.Open,
		High:           tfData.High,
		Low:            tfData.Low,
		Close:          tfData.Close,
		Volume:         tfData.Volume,
		TakerBuyVolume: tfData.TakerBuyVol,
	}

	// Return single candle for now
	// Pattern matcher requires more candles for full pattern detection
	return []Candle{candle}
}

// calculatePriority determines the priority of an entry candidate.
// Higher priority = more urgent opportunity.
func (m *DefaultStrategyMatcher) calculatePriority(coin CoinMatch, strategy StrategyMatch) int {
	priority := 50 // Base priority

	// Pattern-based: higher priority for ready patterns
	if strategy.Type == StrategyTypePattern {
		if coin.Status == PatternStatusReady {
			priority += 30
		}
		// Add priority based on step progress
		priority += coin.Step * 5
	}

	// Score-based: higher priority for higher scores
	if strategy.Type == StrategyTypeScore {
		priority += coin.Score / 2
		if coin.Ready {
			priority += 20
		}
	}

	// Mode-based priority adjustments
	switch strategy.Mode {
	case "ultra_fast":
		priority += 10 // Ultra-fast needs quick action
	case "scalp":
		priority += 5
	}

	return priority
}

// refreshEnabledStrategies refreshes the cached list of enabled strategies.
func (m *DefaultStrategyMatcher) refreshEnabledStrategies(ctx context.Context) error {
	m.mu.RLock()
	needsRefresh := time.Since(m.lastRefresh) > m.refreshInterval
	m.mu.RUnlock()

	if !needsRefresh && len(m.enabledStrategies) > 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(m.lastRefresh) <= m.refreshInterval && len(m.enabledStrategies) > 0 {
		return nil
	}

	strategies, err := m.strategyReader.GetEnabledStrategies(ctx, m.userID)
	if err != nil {
		return fmt.Errorf("failed to get enabled strategies: %w", err)
	}

	m.enabledStrategies = strategies
	m.lastRefresh = time.Now()

	return nil
}

// ============================================================================
// STRATEGY MATCHER BUILDER
// ============================================================================

// StrategyMatcherBuilder provides a fluent API for building a StrategyMatcher.
type StrategyMatcherBuilder struct {
	userID         string
	strategyReader StrategyReader
	patternMatcher *VolumeImbalancePatternMatcher
	scoreCalc      *TrendFollowingScoreCalculator
	coinProvider   CoinDataProvider
}

// NewStrategyMatcherBuilder creates a new StrategyMatcherBuilder.
func NewStrategyMatcherBuilder(userID string) *StrategyMatcherBuilder {
	return &StrategyMatcherBuilder{
		userID: userID,
	}
}

// WithStrategyReader sets the strategy reader.
func (b *StrategyMatcherBuilder) WithStrategyReader(reader StrategyReader) *StrategyMatcherBuilder {
	b.strategyReader = reader
	return b
}

// WithPatternMatcher sets the pattern matcher.
func (b *StrategyMatcherBuilder) WithPatternMatcher(matcher *VolumeImbalancePatternMatcher) *StrategyMatcherBuilder {
	b.patternMatcher = matcher
	return b
}

// WithScoreCalculator sets the score calculator.
func (b *StrategyMatcherBuilder) WithScoreCalculator(calc *TrendFollowingScoreCalculator) *StrategyMatcherBuilder {
	b.scoreCalc = calc
	return b
}

// WithCoinProvider sets the coin data provider.
func (b *StrategyMatcherBuilder) WithCoinProvider(provider CoinDataProvider) *StrategyMatcherBuilder {
	b.coinProvider = provider
	return b
}

// Build creates the StrategyMatcher.
func (b *StrategyMatcherBuilder) Build() (*DefaultStrategyMatcher, error) {
	if b.strategyReader == nil {
		return nil, fmt.Errorf("strategy reader is required")
	}

	config := DefaultStrategyMatcherConfig(b.userID)

	return NewDefaultStrategyMatcher(
		config,
		b.strategyReader,
		b.patternMatcher,
		b.scoreCalc,
		b.coinProvider,
	), nil
}

// ============================================================================
// FILTER OPTIONS
// ============================================================================

// MatchOptions provides filtering options for strategy matching.
type MatchOptions struct {
	// Filter by mode
	Mode string

	// Filter by strategy type
	StrategyType *StrategyType

	// Only return ready candidates
	ReadyOnly bool

	// Minimum score threshold (for score-based strategies)
	MinScore int

	// Filter by symbols
	Symbols []string
}

// NewMatchOptions creates default match options.
func NewMatchOptions() *MatchOptions {
	return &MatchOptions{}
}

// WithMode filters by trading mode.
func (o *MatchOptions) WithMode(mode string) *MatchOptions {
	o.Mode = mode
	return o
}

// WithStrategyType filters by strategy type.
func (o *MatchOptions) WithStrategyType(strategyType StrategyType) *MatchOptions {
	o.StrategyType = &strategyType
	return o
}

// WithReadyOnly only returns ready candidates.
func (o *MatchOptions) WithReadyOnly() *MatchOptions {
	o.ReadyOnly = true
	return o
}

// WithMinScore sets minimum score threshold.
func (o *MatchOptions) WithMinScore(score int) *MatchOptions {
	o.MinScore = score
	return o
}

// WithSymbols filters by specific symbols.
func (o *MatchOptions) WithSymbols(symbols ...string) *MatchOptions {
	o.Symbols = symbols
	return o
}
