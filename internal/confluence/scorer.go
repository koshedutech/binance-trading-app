package confluence

import (
	"fmt"
	"binance-trading-bot/internal/analysis"
	"binance-trading-bot/internal/patterns"
)

// SignalConfluence represents the strength of multiple aligned signals
type SignalConfluence struct {
	// Individual scores (0.0 to 1.0)
	TrendAlignment     float64
	PatternStrength    float64
	VolumeConfirmation float64
	FVGProximity       float64
	IndicatorAlignment float64

	// Composite score
	TotalScore float64
	Grade      string // "A+", "A", "B", "C", "D", "F"

	// Supporting data
	Direction   string // "bullish" or "bearish"
	Reasoning   []string
	Confidence  string // "Very High", "High", "Medium", "Low"
}

// ConfluenceScorer calculates signal confluence
type ConfluenceScorer struct {
	// Weights for different factors (should sum to 1.0)
	trendWeight     float64
	patternWeight   float64
	volumeWeight    float64
	fvgWeight       float64
	indicatorWeight float64

	minScore float64 // Minimum score to generate signal
}

// NewConfluenceScorer creates a new confluence scorer with default weights
func NewConfluenceScorer() *ConfluenceScorer {
	return &ConfluenceScorer{
		trendWeight:     0.30, // 30% - Most important
		patternWeight:   0.25, // 25%
		volumeWeight:    0.20, // 20%
		fvgWeight:       0.15, // 15%
		indicatorWeight: 0.10, // 10%
		minScore:        0.70, // 70% minimum
	}
}

// CalculateConfluence calculates overall signal strength
func (cs *ConfluenceScorer) CalculateConfluence(
	trendScore float64,
	pattern *patterns.DetectedPattern,
	volumeProfile *analysis.VolumeProfile,
	fvgPresent bool,
	fvgDistance float64,
	indicatorScore float64,
) *SignalConfluence {

	confluence := &SignalConfluence{
		Reasoning: make([]string, 0),
	}

	// 1. Trend Alignment Score
	confluence.TrendAlignment = trendScore
	if trendScore > 0.8 {
		confluence.Reasoning = append(confluence.Reasoning, "Strong trend alignment")
	} else if trendScore > 0.6 {
		confluence.Reasoning = append(confluence.Reasoning, "Moderate trend alignment")
	}

	// 2. Pattern Strength Score
	if pattern != nil {
		confluence.PatternStrength = pattern.Confidence
		confluence.Direction = pattern.Direction
		confluence.Reasoning = append(confluence.Reasoning,
			"Pattern detected: "+string(pattern.Type))
	} else {
		confluence.PatternStrength = 0.0
	}

	// 3. Volume Confirmation Score
	if volumeProfile != nil {
		volumeScore := cs.calculateVolumeScore(volumeProfile)
		confluence.VolumeConfirmation = volumeScore
		if volumeProfile.IsHighVolume {
			confluence.Reasoning = append(confluence.Reasoning,
				"High volume confirmation ("+fmt.Sprintf("%.1fx", volumeProfile.VolumeRatio)+" average)")
		}
	} else {
		confluence.VolumeConfirmation = 0.5 // Neutral if no data
	}

	// 4. FVG Proximity Score
	if fvgPresent {
		// Closer to FVG = higher score
		fvgScore := 1.0 - (fvgDistance / 100.0) // Distance as percentage
		if fvgScore < 0 {
			fvgScore = 0
		}
		if fvgScore > 1 {
			fvgScore = 1
		}
		confluence.FVGProximity = fvgScore
		if fvgDistance < 1.0 {
			confluence.Reasoning = append(confluence.Reasoning, "Price at FVG zone")
		} else if fvgDistance < 3.0 {
			confluence.Reasoning = append(confluence.Reasoning, "Price near FVG zone")
		}
	} else {
		confluence.FVGProximity = 0.0
	}

	// 5. Indicator Alignment Score
	confluence.IndicatorAlignment = indicatorScore

	// Calculate weighted total score
	confluence.TotalScore =
		confluence.TrendAlignment * cs.trendWeight +
		confluence.PatternStrength * cs.patternWeight +
		confluence.VolumeConfirmation * cs.volumeWeight +
		confluence.FVGProximity * cs.fvgWeight +
		confluence.IndicatorAlignment * cs.indicatorWeight

	// Assign grade
	confluence.Grade = cs.scoreToGrade(confluence.TotalScore)

	// Assign confidence level
	confluence.Confidence = cs.scoreToConfidence(confluence.TotalScore)

	return confluence
}

// calculateVolumeScore converts volume profile to score
func (cs *ConfluenceScorer) calculateVolumeScore(vp *analysis.VolumeProfile) float64 {
	score := 0.5 // Base score

	if vp.IsClimaxVolume {
		score = 1.0 // Maximum score for climax volume
	} else if vp.IsHighVolume {
		score = 0.85 // High volume
	} else if vp.VolumeRatio > 1.2 {
		score = 0.7 // Above average volume
	} else if vp.VolumeRatio < 0.8 {
		score = 0.3 // Below average (weak)
	}

	// Adjust based on volume type
	if vp.VolumeType == "buying" && vp.VolumeRatio > 1.5 {
		score += 0.1 // Bonus for strong buying volume
	} else if vp.VolumeType == "selling" && vp.VolumeRatio > 1.5 {
		score += 0.1 // Bonus for strong selling volume
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// scoreToGrade converts numerical score to letter grade
func (cs *ConfluenceScorer) scoreToGrade(score float64) string {
	if score >= 0.90 {
		return "A+"
	} else if score >= 0.85 {
		return "A"
	} else if score >= 0.75 {
		return "B+"
	} else if score >= 0.70 {
		return "B"
	} else if score >= 0.60 {
		return "C"
	} else if score >= 0.50 {
		return "D"
	}
	return "F"
}

// scoreToConfidence converts score to confidence level
func (cs *ConfluenceScorer) scoreToConfidence(score float64) string {
	if score >= 0.85 {
		return "Very High"
	} else if score >= 0.75 {
		return "High"
	} else if score >= 0.60 {
		return "Medium"
	} else if score >= 0.45 {
		return "Low"
	}
	return "Very Low"
}

// ShouldTrade determines if signal is strong enough to trade
func (cs *ConfluenceScorer) ShouldTrade(confluence *SignalConfluence) bool {
	return confluence.TotalScore >= cs.minScore
}

// SetMinimumScore adjusts the minimum required score
func (cs *ConfluenceScorer) SetMinimumScore(minScore float64) {
	cs.minScore = minScore
}

// SetWeights allows custom weight configuration
func (cs *ConfluenceScorer) SetWeights(trend, pattern, volume, fvg, indicator float64) error {
	// Validate weights sum to 1.0
	total := trend + pattern + volume + fvg + indicator
	if total < 0.99 || total > 1.01 {
		return fmt.Errorf("weights must sum to 1.0, got %.2f", total)
	}

	cs.trendWeight = trend
	cs.patternWeight = pattern
	cs.volumeWeight = volume
	cs.fvgWeight = fvg
	cs.indicatorWeight = indicator

	return nil
}
