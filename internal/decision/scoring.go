// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.15: Additive Score Calculator
package decision

import (
	"time"
)

// ScoreDimension represents the scoring dimension categories
type ScoreDimension string

const (
	// DimensionTechnical covers technical indicators (0-40 points)
	DimensionTechnical ScoreDimension = "technical"
	// DimensionContext covers market context (0-30 points)
	DimensionContext ScoreDimension = "context"
	// DimensionLLM covers AI/LLM confidence (0-20 points)
	DimensionLLM ScoreDimension = "llm"
	// DimensionHistory covers historical performance (0-10 points)
	DimensionHistory ScoreDimension = "history"
)

// Maximum points per dimension
const (
	MaxTechnicalScore = 40.0
	MaxContextScore   = 30.0
	MaxLLMScore       = 20.0
	MaxHistoryScore   = 10.0
	MaxTotalScore     = 100.0
)

// Technical sub-component maximum points
const (
	MaxTrendAlignmentScore = 15.0
	MaxMomentumScore       = 10.0
	MaxVolatilityScore     = 10.0
	MaxVolumeScore         = 5.0
)

// Context sub-component maximum points
const (
	MaxRegimeMatchScore     = 10.0
	MaxTimeframeAlignScore  = 10.0
	MaxBTCTrendScore        = 10.0
)

// History sub-component maximum points
const (
	MaxSymbolWinRateScore   = 5.0
	MaxStrategyWinRateScore = 5.0
)

// ComponentScore represents a scored dimension with breakdown details
type ComponentScore struct {
	// Dimension is the scoring category (technical, context, llm, history)
	Dimension ScoreDimension `json:"dimension"`
	// Score is the current value (0 to MaxScore)
	Score float64 `json:"score"`
	// MaxScore is the maximum possible score for this dimension
	MaxScore float64 `json:"max_score"`
	// Weight is the applied weight multiplier (default 1.0)
	Weight float64 `json:"weight"`
	// WeightedScore is Score * Weight
	WeightedScore float64 `json:"weighted_score"`
	// Details contains sub-component breakdown (e.g., trend_alignment: 12.5)
	Details map[string]float64 `json:"details"`
}

// NewComponentScore creates a new ComponentScore with initialized Details map
func NewComponentScore(dimension ScoreDimension, maxScore float64) *ComponentScore {
	return &ComponentScore{
		Dimension: dimension,
		MaxScore:  maxScore,
		Weight:    1.0,
		Details:   make(map[string]float64),
	}
}

// SetScore sets the score and calculates weighted score
func (cs *ComponentScore) SetScore(score float64) {
	// Clamp to valid range
	if score < 0 {
		score = 0
	}
	if score > cs.MaxScore {
		score = cs.MaxScore
	}
	cs.Score = score
	cs.WeightedScore = score * cs.Weight
}

// SetWeight sets the weight and recalculates weighted score
func (cs *ComponentScore) SetWeight(weight float64) {
	if weight < 0 {
		weight = 0
	}
	cs.Weight = weight
	cs.WeightedScore = cs.Score * cs.Weight
}

// Percentage returns the score as a percentage of max score
func (cs *ComponentScore) Percentage() float64 {
	if cs.MaxScore == 0 {
		return 0
	}
	return (cs.Score / cs.MaxScore) * 100
}

// ScoreBreakdown contains the complete scoring breakdown for a symbol
type ScoreBreakdown struct {
	// Symbol is the trading pair (e.g., BTCUSDT)
	Symbol string `json:"symbol"`
	// Components contains scores for each dimension
	Components []ComponentScore `json:"components"`
	// FinalScore is the sum of all weighted component scores (0-100)
	FinalScore float64 `json:"final_score"`
	// NormalizedScore is FinalScore normalized to 0-100 scale after weighting
	NormalizedScore float64 `json:"normalized_score"`
	// Timestamp is when this breakdown was calculated (Unix milliseconds)
	Timestamp int64 `json:"timestamp"`
	// PassesThreshold indicates if the score meets the minimum threshold
	PassesThreshold bool `json:"passes_threshold"`
	// Threshold is the configured minimum threshold value
	Threshold float64 `json:"threshold"`
}

// NewScoreBreakdown creates a new ScoreBreakdown for a symbol
func NewScoreBreakdown(symbol string) *ScoreBreakdown {
	return &ScoreBreakdown{
		Symbol:     symbol,
		Components: make([]ComponentScore, 0, 4),
		Timestamp:  time.Now().UnixMilli(),
	}
}

// AddComponent adds a component score to the breakdown
func (sb *ScoreBreakdown) AddComponent(cs ComponentScore) {
	sb.Components = append(sb.Components, cs)
}

// GetComponent returns a component score by dimension, or nil if not found
func (sb *ScoreBreakdown) GetComponent(dimension ScoreDimension) *ComponentScore {
	for i := range sb.Components {
		if sb.Components[i].Dimension == dimension {
			return &sb.Components[i]
		}
	}
	return nil
}

// CalculateFinal computes the final score from all components
func (sb *ScoreBreakdown) CalculateFinal(threshold float64) {
	sb.Threshold = threshold
	sb.FinalScore = 0

	// Sum weighted scores
	totalWeightedMax := 0.0
	for _, cs := range sb.Components {
		sb.FinalScore += cs.WeightedScore
		totalWeightedMax += cs.MaxScore * cs.Weight
	}

	// Calculate normalized score (handles custom weights)
	if totalWeightedMax > 0 {
		sb.NormalizedScore = (sb.FinalScore / totalWeightedMax) * MaxTotalScore
	} else {
		sb.NormalizedScore = 0
	}

	sb.PassesThreshold = sb.NormalizedScore >= threshold
	sb.Timestamp = time.Now().UnixMilli()
}

// ScoringConfig contains configurable weights for each scoring dimension
type ScoringConfig struct {
	// TechnicalWeight is the weight for technical indicators (default 1.0 = 40 points max)
	TechnicalWeight float64 `json:"technical_weight"`
	// ContextWeight is the weight for market context (default 1.0 = 30 points max)
	ContextWeight float64 `json:"context_weight"`
	// LLMWeight is the weight for AI/LLM confidence (default 1.0 = 20 points max)
	LLMWeight float64 `json:"llm_weight"`
	// HistoryWeight is the weight for historical performance (default 1.0 = 10 points max)
	HistoryWeight float64 `json:"history_weight"`
	// MinThreshold is the minimum normalized score to pass (default 55)
	MinThreshold float64 `json:"min_threshold"`
}

// DefaultScoringConfig returns the default scoring configuration
func DefaultScoringConfig() *ScoringConfig {
	return &ScoringConfig{
		TechnicalWeight: 1.0,
		ContextWeight:   1.0,
		LLMWeight:       1.0,
		HistoryWeight:   1.0,
		MinThreshold:    55.0,
	}
}

// Validate checks if the config has valid values
func (sc *ScoringConfig) Validate() error {
	// Weights must be non-negative
	if sc.TechnicalWeight < 0 {
		sc.TechnicalWeight = 0
	}
	if sc.ContextWeight < 0 {
		sc.ContextWeight = 0
	}
	if sc.LLMWeight < 0 {
		sc.LLMWeight = 0
	}
	if sc.HistoryWeight < 0 {
		sc.HistoryWeight = 0
	}

	// Threshold must be in valid range
	if sc.MinThreshold < 0 {
		sc.MinThreshold = 0
	}
	if sc.MinThreshold > MaxTotalScore {
		sc.MinThreshold = MaxTotalScore
	}

	return nil
}

// Clone returns a deep copy of the config
func (sc *ScoringConfig) Clone() *ScoringConfig {
	return &ScoringConfig{
		TechnicalWeight: sc.TechnicalWeight,
		ContextWeight:   sc.ContextWeight,
		LLMWeight:       sc.LLMWeight,
		HistoryWeight:   sc.HistoryWeight,
		MinThreshold:    sc.MinThreshold,
	}
}

// ScoreHistory represents historical score tracking for a symbol
type ScoreHistory struct {
	// Symbol is the trading pair
	Symbol string `json:"symbol"`
	// Scores is a rolling window of recent score breakdowns
	Scores []ScoreBreakdown `json:"scores"`
	// MaxSize is the maximum number of scores to retain
	MaxSize int `json:"max_size"`
}

// NewScoreHistory creates a new ScoreHistory with the given max size
func NewScoreHistory(symbol string, maxSize int) *ScoreHistory {
	if maxSize <= 0 {
		maxSize = 100 // Default to 100 entries
	}
	return &ScoreHistory{
		Symbol:  symbol,
		Scores:  make([]ScoreBreakdown, 0, maxSize),
		MaxSize: maxSize,
	}
}

// Add adds a score breakdown to the history, removing oldest if at capacity
func (sh *ScoreHistory) Add(breakdown ScoreBreakdown) {
	if len(sh.Scores) >= sh.MaxSize {
		// Remove oldest entry
		sh.Scores = sh.Scores[1:]
	}
	sh.Scores = append(sh.Scores, breakdown)
}

// Latest returns the most recent score breakdown, or nil if empty
func (sh *ScoreHistory) Latest() *ScoreBreakdown {
	if len(sh.Scores) == 0 {
		return nil
	}
	return &sh.Scores[len(sh.Scores)-1]
}

// Average returns the average final score over all historical entries
func (sh *ScoreHistory) Average() float64 {
	if len(sh.Scores) == 0 {
		return 0
	}

	total := 0.0
	for _, s := range sh.Scores {
		total += s.NormalizedScore
	}
	return total / float64(len(sh.Scores))
}

// Count returns the number of score entries
func (sh *ScoreHistory) Count() int {
	return len(sh.Scores)
}

// Clear removes all historical scores
func (sh *ScoreHistory) Clear() {
	sh.Scores = make([]ScoreBreakdown, 0, sh.MaxSize)
}
