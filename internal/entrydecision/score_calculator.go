// Package entrydecision provides score calculation logic for the Entry Decision System.
// Epic 14: Chain Trading System - Story 14.10: Score Calculator for Trend Following
package entrydecision

import (
	"errors"
	"fmt"
	"time"

	"binance-trading-bot/internal/coinprofiler"
)

// Default configuration values for score calculation.
const (
	// DefaultScoreThreshold is the minimum score required for a coin to be "ready"
	DefaultScoreThreshold = 55

	// Default component weights (must sum to 100)
	DefaultWeightTrendStrength = 30 // EMA alignment
	DefaultWeightMomentum      = 25 // RSI, MACD
	DefaultWeightVolume        = 20 // Volume confirmation
	DefaultWeightPriceAction   = 15 // Higher highs/lows
	DefaultWeightSRProximity   = 10 // Support/Resistance proximity
)

// ScoreComponent represents a named component of the total score.
type ScoreComponent string

const (
	// ScoreComponentTrendStrength measures EMA alignment and trend direction.
	ScoreComponentTrendStrength ScoreComponent = "trend_strength"
	// ScoreComponentMomentum measures RSI and MACD signals.
	ScoreComponentMomentum ScoreComponent = "momentum"
	// ScoreComponentVolume measures volume confirmation.
	ScoreComponentVolume ScoreComponent = "volume"
	// ScoreComponentPriceAction measures higher highs/lows pattern.
	ScoreComponentPriceAction ScoreComponent = "price_action"
	// ScoreComponentSRProximity measures proximity to support/resistance levels.
	ScoreComponentSRProximity ScoreComponent = "sr_proximity"
)

// Direction constants for trade direction.
const (
	DirectionLong    = "long"
	DirectionShort   = "short"
	DirectionNeutral = "neutral"
)

// ScoreResult contains the result of score calculation for a coin.
type ScoreResult struct {
	Symbol     string         `json:"symbol"`      // e.g., "BTCUSDT"
	Score      int            `json:"score"`       // Final weighted score 0-100
	Direction  string         `json:"direction"`   // "long", "short", or "neutral"
	Ready      bool           `json:"ready"`       // Score >= threshold
	Components map[string]int `json:"components"`  // Individual component scores
	Threshold  int            `json:"threshold"`   // Threshold used for ready determination
	UpdatedAt  time.Time      `json:"updated_at"`  // When this result was calculated
}

// ToCoinMatch converts the ScoreResult to a CoinMatch for API responses.
func (sr *ScoreResult) ToCoinMatch() *CoinMatch {
	if sr == nil {
		return nil
	}
	return &CoinMatch{
		Symbol:    sr.Symbol,
		Score:     sr.Score,
		Ready:     sr.Ready,
		Direction: sr.Direction,
		UpdatedAt: sr.UpdatedAt,
	}
}

// ScoreWeights defines the weight for each score component.
// Weights must sum to 100 for proper percentage calculation.
type ScoreWeights struct {
	TrendStrength int `json:"trend_strength"` // EMA alignment weight
	Momentum      int `json:"momentum"`       // RSI, MACD weight
	Volume        int `json:"volume"`         // Volume confirmation weight
	PriceAction   int `json:"price_action"`   // Higher highs/lows weight
	SRProximity   int `json:"sr_proximity"`   // Support/Resistance proximity weight
}

// DefaultScoreWeights returns the default score weights.
func DefaultScoreWeights() *ScoreWeights {
	return &ScoreWeights{
		TrendStrength: DefaultWeightTrendStrength,
		Momentum:      DefaultWeightMomentum,
		Volume:        DefaultWeightVolume,
		PriceAction:   DefaultWeightPriceAction,
		SRProximity:   DefaultWeightSRProximity,
	}
}

// Sum returns the total of all weights.
func (w *ScoreWeights) Sum() int {
	if w == nil {
		return 0
	}
	return w.TrendStrength + w.Momentum + w.Volume + w.PriceAction + w.SRProximity
}

// Validate checks if the weights are valid (all non-negative and sum to 100).
func (w *ScoreWeights) Validate() error {
	if w == nil {
		return errors.New("weights cannot be nil")
	}
	if w.TrendStrength < 0 || w.Momentum < 0 || w.Volume < 0 || w.PriceAction < 0 || w.SRProximity < 0 {
		return errors.New("all weights must be non-negative")
	}
	if w.Sum() != 100 {
		return fmt.Errorf("weights must sum to 100, got %d", w.Sum())
	}
	return nil
}

// ScoreCalculatorConfig holds configuration for the score calculator.
type ScoreCalculatorConfig struct {
	Threshold int           `json:"threshold"` // Score threshold for ready determination
	Weights   *ScoreWeights `json:"weights"`   // Component weights
}

// DefaultScoreCalculatorConfig returns the default configuration.
func DefaultScoreCalculatorConfig() *ScoreCalculatorConfig {
	return &ScoreCalculatorConfig{
		Threshold: DefaultScoreThreshold,
		Weights:   DefaultScoreWeights(),
	}
}

// Validate validates the configuration.
func (c *ScoreCalculatorConfig) Validate() error {
	if c == nil {
		return errors.New("config cannot be nil")
	}
	if c.Threshold < 0 || c.Threshold > 100 {
		return fmt.Errorf("threshold must be between 0 and 100, got %d", c.Threshold)
	}
	if c.Weights == nil {
		return errors.New("weights cannot be nil")
	}
	return c.Weights.Validate()
}

// ScoreCalculator calculates entry scores for coins based on technical indicators.
type ScoreCalculator interface {
	// CalculateScore calculates the entry score for a single coin.
	CalculateScore(symbol string, data *coinprofiler.CoinData) (*ScoreResult, error)

	// CalculateScores calculates scores for multiple coins.
	CalculateScores(coins map[string]*coinprofiler.CoinData) ([]*ScoreResult, error)

	// GetConfig returns the current configuration.
	GetConfig() *ScoreCalculatorConfig

	// SetConfig updates the configuration.
	SetConfig(config *ScoreCalculatorConfig) error
}

// ComponentCalculator is an interface for calculating individual score components.
// This allows for pluggable calculation logic (stub vs real indicators).
type ComponentCalculator interface {
	// CalculateTrendStrength calculates the trend strength score (0-100).
	// Measures EMA alignment and trend direction.
	CalculateTrendStrength(data *coinprofiler.CoinData) (score int, direction string, err error)

	// CalculateMomentum calculates the momentum score (0-100).
	// Measures RSI and MACD signals.
	CalculateMomentum(data *coinprofiler.CoinData) (score int, direction string, err error)

	// CalculateVolume calculates the volume confirmation score (0-100).
	// Measures volume relative to average and trend alignment.
	CalculateVolume(data *coinprofiler.CoinData) (score int, err error)

	// CalculatePriceAction calculates the price action score (0-100).
	// Measures higher highs/lows pattern.
	CalculatePriceAction(data *coinprofiler.CoinData) (score int, direction string, err error)

	// CalculateSRProximity calculates the support/resistance proximity score (0-100).
	// Measures how close price is to key levels.
	CalculateSRProximity(data *coinprofiler.CoinData) (score int, err error)
}

// TrendFollowingScoreCalculator implements ScoreCalculator for the Trend Following strategy.
type TrendFollowingScoreCalculator struct {
	config     *ScoreCalculatorConfig
	components ComponentCalculator
}

// NewTrendFollowingScoreCalculator creates a new score calculator with default configuration.
func NewTrendFollowingScoreCalculator() *TrendFollowingScoreCalculator {
	return &TrendFollowingScoreCalculator{
		config:     DefaultScoreCalculatorConfig(),
		components: NewStubComponentCalculator(),
	}
}

// NewTrendFollowingScoreCalculatorWithConfig creates a new score calculator with custom configuration.
func NewTrendFollowingScoreCalculatorWithConfig(config *ScoreCalculatorConfig) (*TrendFollowingScoreCalculator, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &TrendFollowingScoreCalculator{
		config:     config,
		components: NewStubComponentCalculator(),
	}, nil
}

// NewTrendFollowingScoreCalculatorWithComponents creates a calculator with custom component calculator.
func NewTrendFollowingScoreCalculatorWithComponents(config *ScoreCalculatorConfig, components ComponentCalculator) (*TrendFollowingScoreCalculator, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if components == nil {
		return nil, errors.New("component calculator cannot be nil")
	}
	return &TrendFollowingScoreCalculator{
		config:     config,
		components: components,
	}, nil
}

// CalculateScore calculates the entry score for a single coin.
func (c *TrendFollowingScoreCalculator) CalculateScore(symbol string, data *coinprofiler.CoinData) (*ScoreResult, error) {
	if symbol == "" {
		return nil, errors.New("symbol cannot be empty")
	}

	// Handle nil or empty data - return zero score
	if data == nil {
		return &ScoreResult{
			Symbol:     symbol,
			Score:      0,
			Direction:  DirectionNeutral,
			Ready:      false,
			Components: make(map[string]int),
			Threshold:  c.config.Threshold,
			UpdatedAt:  time.Now(),
		}, nil
	}

	// Calculate individual components
	components := make(map[string]int)
	directions := make(map[string]string)

	// Trend Strength
	trendScore, trendDir, err := c.components.CalculateTrendStrength(data)
	if err != nil {
		return nil, fmt.Errorf("error calculating trend strength: %w", err)
	}
	components[string(ScoreComponentTrendStrength)] = clampScore(trendScore)
	directions[string(ScoreComponentTrendStrength)] = trendDir

	// Momentum
	momentumScore, momentumDir, err := c.components.CalculateMomentum(data)
	if err != nil {
		return nil, fmt.Errorf("error calculating momentum: %w", err)
	}
	components[string(ScoreComponentMomentum)] = clampScore(momentumScore)
	directions[string(ScoreComponentMomentum)] = momentumDir

	// Volume
	volumeScore, err := c.components.CalculateVolume(data)
	if err != nil {
		return nil, fmt.Errorf("error calculating volume: %w", err)
	}
	components[string(ScoreComponentVolume)] = clampScore(volumeScore)

	// Price Action
	priceActionScore, priceActionDir, err := c.components.CalculatePriceAction(data)
	if err != nil {
		return nil, fmt.Errorf("error calculating price action: %w", err)
	}
	components[string(ScoreComponentPriceAction)] = clampScore(priceActionScore)
	directions[string(ScoreComponentPriceAction)] = priceActionDir

	// Support/Resistance Proximity
	srScore, err := c.components.CalculateSRProximity(data)
	if err != nil {
		return nil, fmt.Errorf("error calculating S/R proximity: %w", err)
	}
	components[string(ScoreComponentSRProximity)] = clampScore(srScore)

	// Calculate weighted average
	weights := c.config.Weights
	weightedSum := components[string(ScoreComponentTrendStrength)]*weights.TrendStrength +
		components[string(ScoreComponentMomentum)]*weights.Momentum +
		components[string(ScoreComponentVolume)]*weights.Volume +
		components[string(ScoreComponentPriceAction)]*weights.PriceAction +
		components[string(ScoreComponentSRProximity)]*weights.SRProximity

	// Weights sum to 100, so divide by 100 to get final score
	finalScore := weightedSum / 100

	// Determine direction based on directional components
	direction := c.determineDirection(directions)

	return &ScoreResult{
		Symbol:     symbol,
		Score:      clampScore(finalScore),
		Direction:  direction,
		Ready:      finalScore >= c.config.Threshold,
		Components: components,
		Threshold:  c.config.Threshold,
		UpdatedAt:  time.Now(),
	}, nil
}

// determineDirection determines the overall direction based on component directions.
// Uses weighted voting from directional components (trend, momentum, price action).
func (c *TrendFollowingScoreCalculator) determineDirection(directions map[string]string) string {
	weights := c.config.Weights

	// Only directional components contribute to direction
	longScore := 0
	shortScore := 0

	// Trend Strength has highest weight in direction
	switch directions[string(ScoreComponentTrendStrength)] {
	case DirectionLong:
		longScore += weights.TrendStrength
	case DirectionShort:
		shortScore += weights.TrendStrength
	}

	// Momentum direction
	switch directions[string(ScoreComponentMomentum)] {
	case DirectionLong:
		longScore += weights.Momentum
	case DirectionShort:
		shortScore += weights.Momentum
	}

	// Price Action direction
	switch directions[string(ScoreComponentPriceAction)] {
	case DirectionLong:
		longScore += weights.PriceAction
	case DirectionShort:
		shortScore += weights.PriceAction
	}

	// Determine overall direction
	if longScore > shortScore {
		return DirectionLong
	} else if shortScore > longScore {
		return DirectionShort
	}
	return DirectionNeutral
}

// CalculateScores calculates scores for multiple coins.
func (c *TrendFollowingScoreCalculator) CalculateScores(coins map[string]*coinprofiler.CoinData) ([]*ScoreResult, error) {
	if coins == nil {
		return nil, nil
	}

	results := make([]*ScoreResult, 0, len(coins))
	for symbol, data := range coins {
		result, err := c.CalculateScore(symbol, data)
		if err != nil {
			// Log error but continue with other coins
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GetConfig returns the current configuration.
func (c *TrendFollowingScoreCalculator) GetConfig() *ScoreCalculatorConfig {
	return c.config
}

// SetConfig updates the configuration.
func (c *TrendFollowingScoreCalculator) SetConfig(config *ScoreCalculatorConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	c.config = config
	return nil
}

// SetComponentCalculator sets the component calculator.
// This allows switching between stub and real indicator calculations.
func (c *TrendFollowingScoreCalculator) SetComponentCalculator(components ComponentCalculator) error {
	if components == nil {
		return errors.New("component calculator cannot be nil")
	}
	c.components = components
	return nil
}

// StubComponentCalculator provides mock score calculations for testing.
// This will be replaced with real indicator calculations in Story 14.11.
type StubComponentCalculator struct {
	// Fixed scores for testing (if set, overrides random generation)
	FixedTrendScore       *int
	FixedTrendDirection   *string
	FixedMomentumScore    *int
	FixedMomentumDirection *string
	FixedVolumeScore      *int
	FixedPriceActionScore *int
	FixedPriceActionDir   *string
	FixedSRScore          *int
}

// NewStubComponentCalculator creates a new stub calculator.
func NewStubComponentCalculator() *StubComponentCalculator {
	return &StubComponentCalculator{}
}

// WithFixedScores sets fixed scores for deterministic testing.
func (s *StubComponentCalculator) WithFixedScores(trend, momentum, volume, priceAction, sr int) *StubComponentCalculator {
	s.FixedTrendScore = &trend
	s.FixedMomentumScore = &momentum
	s.FixedVolumeScore = &volume
	s.FixedPriceActionScore = &priceAction
	s.FixedSRScore = &sr
	return s
}

// WithFixedDirections sets fixed directions for deterministic testing.
func (s *StubComponentCalculator) WithFixedDirections(trend, momentum, priceAction string) *StubComponentCalculator {
	s.FixedTrendDirection = &trend
	s.FixedMomentumDirection = &momentum
	s.FixedPriceActionDir = &priceAction
	return s
}

// CalculateTrendStrength returns a mock trend strength score.
func (s *StubComponentCalculator) CalculateTrendStrength(data *coinprofiler.CoinData) (int, string, error) {
	score := 70 // Default stub score
	direction := DirectionLong

	if s.FixedTrendScore != nil {
		score = *s.FixedTrendScore
	}
	if s.FixedTrendDirection != nil {
		direction = *s.FixedTrendDirection
	}

	// If fixed score is set, return without additional modification for deterministic testing
	if s.FixedTrendScore != nil {
		return score, direction, nil
	}

	// If we have real price data, use it to influence the stub
	if data != nil && data.Price > 0 {
		// Simple heuristic: higher volatility = stronger trend potential
		if data.Volatility > 2.0 {
			score = min(100, score+10)
		}
	}

	return score, direction, nil
}

// CalculateMomentum returns a mock momentum score.
func (s *StubComponentCalculator) CalculateMomentum(data *coinprofiler.CoinData) (int, string, error) {
	score := 65 // Default stub score
	direction := DirectionLong

	if s.FixedMomentumScore != nil {
		score = *s.FixedMomentumScore
	}
	if s.FixedMomentumDirection != nil {
		direction = *s.FixedMomentumDirection
	}

	return score, direction, nil
}

// CalculateVolume returns a mock volume score.
func (s *StubComponentCalculator) CalculateVolume(data *coinprofiler.CoinData) (int, error) {
	score := 60 // Default stub score

	if s.FixedVolumeScore != nil {
		// When fixed score is set, return it directly without modification
		return *s.FixedVolumeScore, nil
	}

	// If we have real volume data, use it to influence the stub
	if data != nil && data.Volume24h > 0 {
		// Higher volume = higher score potential
		if data.Volume24h > 100000000 { // >100M
			score = min(100, score+15)
		} else if data.Volume24h > 10000000 { // >10M
			score = min(100, score+5)
		}
	}

	return score, nil
}

// CalculatePriceAction returns a mock price action score.
func (s *StubComponentCalculator) CalculatePriceAction(data *coinprofiler.CoinData) (int, string, error) {
	score := 55 // Default stub score
	direction := DirectionLong

	if s.FixedPriceActionScore != nil {
		score = *s.FixedPriceActionScore
	}
	if s.FixedPriceActionDir != nil {
		direction = *s.FixedPriceActionDir
	}

	return score, direction, nil
}

// CalculateSRProximity returns a mock support/resistance proximity score.
func (s *StubComponentCalculator) CalculateSRProximity(data *coinprofiler.CoinData) (int, error) {
	score := 50 // Default stub score

	if s.FixedSRScore != nil {
		score = *s.FixedSRScore
	}

	return score, nil
}

// clampScore ensures a score is within the valid range [0, 100].
func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
