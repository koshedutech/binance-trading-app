package autopilot

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// ===== TRADE OUTCOME TRACKING =====

// TradeOutcome represents a completed trade for adaptive learning analysis
type TradeOutcome struct {
	TradeID         string                 `json:"trade_id"`
	Symbol          string                 `json:"symbol"`
	Mode            GinieTradingMode       `json:"mode"`
	EntryTime       time.Time              `json:"entry_time"`
	ExitTime        time.Time              `json:"exit_time"`
	Direction       string                 `json:"direction"` // LONG, SHORT
	EntryPrice      float64                `json:"entry_price"`
	ExitPrice       float64                `json:"exit_price"`
	PnLPercent      float64                `json:"pnl_percent"`
	PnLUSD          float64                `json:"pnl_usd"`
	Outcome         string                 `json:"outcome"` // WIN, LOSS, BREAKEVEN
	DecisionContext *DecisionContext       `json:"decision_context,omitempty"`
	MarketSnapshot  map[string]interface{} `json:"market_snapshot,omitempty"`
}

// Note: DecisionContext is defined in ginie_types.go with the following fields:
// - TechnicalConfidence int
// - LLMConfidence int
// - FinalConfidence int
// - TechnicalDirection string
// - LLMDirection string
// - Agreement bool
// - LLMReasoning string
// - LLMKeyFactors []string
// - LLMProvider string
// - LLMModel string
// - LLMLatencyMs int64
// - UsedCache bool
// - SkippedLLM bool
// - SkipReason string

// ===== ADAPTIVE RECOMMENDATION =====

// AdaptiveRecommendation represents a suggestion from the adaptive learning system
type AdaptiveRecommendation struct {
	ID                  string           `json:"id"`
	CreatedAt           time.Time        `json:"created_at"`
	Type                string           `json:"type"` // llm_weight, min_confidence, block_disagreement
	Mode                GinieTradingMode `json:"mode"`
	CurrentValue        interface{}      `json:"current_value"`
	SuggestedValue      interface{}      `json:"suggested_value"`
	Reason              string           `json:"reason"`
	ExpectedImprovement string           `json:"expected_improvement"`
	AppliedAt           *time.Time       `json:"applied_at,omitempty"`
	Dismissed           bool             `json:"dismissed"`
}

// ===== MODE STATISTICS =====

// ModeStatistics contains aggregated statistics for a trading mode
type ModeStatistics struct {
	Mode                GinieTradingMode `json:"mode"`
	TotalTrades         int              `json:"total_trades"`
	Wins                int              `json:"wins"`
	Losses              int              `json:"losses"`
	Breakeven           int              `json:"breakeven"`
	WinRate             float64          `json:"win_rate"`
	AvgWinPercent       float64          `json:"avg_win_percent"`
	AvgLossPercent      float64          `json:"avg_loss_percent"`
	TotalProfit         float64          `json:"total_profit"`
	AgreementWinRate    float64          `json:"agreement_win_rate"`    // Win rate when tech+LLM agree
	DisagreementWinRate float64          `json:"disagreement_win_rate"` // Win rate when they disagree

	// Confidence bracket analysis
	LowConfidenceWinRate  float64 `json:"low_confidence_win_rate"`  // Confidence 50-65
	MedConfidenceWinRate  float64 `json:"med_confidence_win_rate"`  // Confidence 65-80
	HighConfidenceWinRate float64 `json:"high_confidence_win_rate"` // Confidence 80-100
	LowConfidenceTrades   int     `json:"low_confidence_trades"`
	MedConfidenceTrades   int     `json:"med_confidence_trades"`
	HighConfidenceTrades  int     `json:"high_confidence_trades"`
}

// ===== ADAPTIVE AI CONTROLLER =====

// AdaptiveAI is the main controller for adaptive learning from trade outcomes
type AdaptiveAI struct {
	mu              sync.RWMutex
	outcomes        []TradeOutcome
	recommendations []AdaptiveRecommendation
	lastAnalysis    time.Time
	analysisCount   int
	settingsManager *SettingsManager

	// Learning configuration
	learningWindowTrades int // Number of trades before analysis
	learningWindowHours  int // Hours before analysis (whichever comes first)
	maxOutcomes          int // Maximum outcomes to retain
	recommendationIDSeq  int // Sequence for generating IDs
}

// NewAdaptiveAI creates a new AdaptiveAI instance
func NewAdaptiveAI(settingsManager *SettingsManager) *AdaptiveAI {
	ai := &AdaptiveAI{
		outcomes:             make([]TradeOutcome, 0, 1000),
		recommendations:      make([]AdaptiveRecommendation, 0, 100),
		settingsManager:      settingsManager,
		learningWindowTrades: 50,   // Analyze after 50 trades
		learningWindowHours:  24,   // Or after 24 hours
		maxOutcomes:          1000, // Keep last 1000 outcomes
		lastAnalysis:         time.Now(),
	}

	log.Println("[ADAPTIVE-AI] AdaptiveAI initialized")
	log.Printf("[ADAPTIVE-AI] Learning window: %d trades or %d hours", ai.learningWindowTrades, ai.learningWindowHours)

	return ai
}

// ===== CORE METHODS =====

// RecordTradeOutcome adds a new trade outcome for learning
func (ai *AdaptiveAI) RecordTradeOutcome(outcome TradeOutcome) {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	// Add the outcome
	ai.outcomes = append(ai.outcomes, outcome)

	// Trim old outcomes if exceeding max
	if len(ai.outcomes) > ai.maxOutcomes {
		// Keep only the most recent outcomes
		trimCount := len(ai.outcomes) - ai.maxOutcomes
		ai.outcomes = ai.outcomes[trimCount:]
		log.Printf("[ADAPTIVE-AI] Trimmed %d old outcomes, now have %d", trimCount, len(ai.outcomes))
	}

	log.Printf("[ADAPTIVE-AI] Recorded trade outcome: %s %s %s, PnL: %.2f%% ($%.2f), Outcome: %s",
		outcome.Symbol, outcome.Mode, outcome.Direction,
		outcome.PnLPercent, outcome.PnLUSD, outcome.Outcome)

	// Check if analysis should run
	if ai.shouldRunAnalysisLocked() {
		log.Println("[ADAPTIVE-AI] Learning window reached, triggering analysis")
		go ai.runAnalysisAsync()
	}
}

// shouldRunAnalysisLocked checks if analysis should run (must hold lock)
func (ai *AdaptiveAI) shouldRunAnalysisLocked() bool {
	// Count trades since last analysis
	tradesSinceAnalysis := 0
	for i := len(ai.outcomes) - 1; i >= 0; i-- {
		if ai.outcomes[i].ExitTime.After(ai.lastAnalysis) {
			tradesSinceAnalysis++
		} else {
			break
		}
	}

	// Check trade count threshold
	if tradesSinceAnalysis >= ai.learningWindowTrades {
		return true
	}

	// Check time threshold
	hoursSinceAnalysis := time.Since(ai.lastAnalysis).Hours()
	if hoursSinceAnalysis >= float64(ai.learningWindowHours) && tradesSinceAnalysis >= 10 {
		// Require at least 10 trades for time-based analysis
		return true
	}

	return false
}

// ShouldRunAnalysis checks if analysis should run (public, acquires lock)
func (ai *AdaptiveAI) ShouldRunAnalysis() bool {
	ai.mu.RLock()
	defer ai.mu.RUnlock()
	return ai.shouldRunAnalysisLocked()
}

// runAnalysisAsync runs the analysis in the background
func (ai *AdaptiveAI) runAnalysisAsync() {
	recommendations, err := ai.RunAdaptiveLearning()
	if err != nil {
		log.Printf("[ADAPTIVE-AI] Analysis error: %v", err)
		return
	}

	if len(recommendations) > 0 {
		log.Printf("[ADAPTIVE-AI] Generated %d new recommendations", len(recommendations))
	} else {
		log.Println("[ADAPTIVE-AI] Analysis complete, no new recommendations")
	}
}

// RunAdaptiveLearning performs the adaptive learning analysis
func (ai *AdaptiveAI) RunAdaptiveLearning() ([]AdaptiveRecommendation, error) {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	log.Println("[ADAPTIVE-AI] Starting adaptive learning analysis...")

	// Update last analysis time
	ai.lastAnalysis = time.Now()
	ai.analysisCount++

	// Get outcomes since last analysis (or all if first analysis)
	recentOutcomes := ai.getRecentOutcomesLocked(ai.learningWindowTrades * 2)
	if len(recentOutcomes) < 10 {
		log.Printf("[ADAPTIVE-AI] Insufficient outcomes for analysis: %d (need at least 10)", len(recentOutcomes))
		return nil, nil
	}

	log.Printf("[ADAPTIVE-AI] Analyzing %d outcomes", len(recentOutcomes))

	// Calculate statistics by mode
	statsByMode := ai.calculateStatsByMode(recentOutcomes)

	// Log statistics
	for mode, stats := range statsByMode {
		log.Printf("[ADAPTIVE-AI] Mode %s stats: Trades=%d, WinRate=%.1f%%, AgreementWR=%.1f%%, DisagreementWR=%.1f%%",
			mode, stats.TotalTrades, stats.WinRate, stats.AgreementWinRate, stats.DisagreementWinRate)
	}

	// Generate recommendations
	newRecommendations := make([]AdaptiveRecommendation, 0)

	for mode, stats := range statsByMode {
		if stats.TotalTrades < 5 {
			continue // Need minimum trades per mode
		}

		// Check for disagreement pattern recommendations
		if rec := ai.generateDisagreementRecommendation(mode, stats); rec != nil {
			newRecommendations = append(newRecommendations, *rec)
			ai.recommendations = append(ai.recommendations, *rec)
		}

		// Check for LLM weight recommendations
		if rec := ai.generateLLMWeightRecommendation(mode, stats); rec != nil {
			newRecommendations = append(newRecommendations, *rec)
			ai.recommendations = append(ai.recommendations, *rec)
		}

		// Check for confidence threshold recommendations
		if rec := ai.generateConfidenceRecommendation(mode, stats); rec != nil {
			newRecommendations = append(newRecommendations, *rec)
			ai.recommendations = append(ai.recommendations, *rec)
		}
	}

	log.Printf("[ADAPTIVE-AI] Analysis #%d complete, generated %d recommendations",
		ai.analysisCount, len(newRecommendations))

	return newRecommendations, nil
}

// calculateStatsByMode aggregates outcomes into statistics per mode
func (ai *AdaptiveAI) calculateStatsByMode(outcomes []TradeOutcome) map[GinieTradingMode]*ModeStatistics {
	stats := make(map[GinieTradingMode]*ModeStatistics)

	// Initialize stats for all modes
	for _, mode := range []GinieTradingMode{GinieModeUltraFast, GinieModeScalp, GinieModeSwing, GinieModePosition} {
		stats[mode] = &ModeStatistics{Mode: mode}
	}

	// Aggregate outcomes
	for _, outcome := range outcomes {
		stat := stats[outcome.Mode]
		if stat == nil {
			stat = &ModeStatistics{Mode: outcome.Mode}
			stats[outcome.Mode] = stat
		}

		stat.TotalTrades++
		stat.TotalProfit += outcome.PnLUSD

		switch outcome.Outcome {
		case "WIN":
			stat.Wins++
			stat.AvgWinPercent += outcome.PnLPercent
		case "LOSS":
			stat.Losses++
			stat.AvgLossPercent += outcome.PnLPercent
		case "BREAKEVEN":
			stat.Breakeven++
		}

		// Analyze agreement patterns using DecisionContext fields
		if outcome.DecisionContext != nil {
			ctx := outcome.DecisionContext
			// Use Agreement field from DecisionContext (ginie_types.go)
			if ctx.Agreement {
				if outcome.Outcome == "WIN" {
					stat.AgreementWinRate++
				}
			} else {
				if outcome.Outcome == "WIN" {
					stat.DisagreementWinRate++
				}
			}

			// Confidence bracket analysis using FinalConfidence (int)
			confidence := ctx.FinalConfidence
			isWin := outcome.Outcome == "WIN"

			if confidence >= 50 && confidence < 65 {
				stat.LowConfidenceTrades++
				if isWin {
					stat.LowConfidenceWinRate++
				}
			} else if confidence >= 65 && confidence < 80 {
				stat.MedConfidenceTrades++
				if isWin {
					stat.MedConfidenceWinRate++
				}
			} else if confidence >= 80 {
				stat.HighConfidenceTrades++
				if isWin {
					stat.HighConfidenceWinRate++
				}
			}
		}
	}

	// Calculate averages and percentages
	for _, stat := range stats {
		if stat.TotalTrades > 0 {
			stat.WinRate = float64(stat.Wins) / float64(stat.TotalTrades) * 100

			if stat.Wins > 0 {
				stat.AvgWinPercent /= float64(stat.Wins)
			}
			if stat.Losses > 0 {
				stat.AvgLossPercent /= float64(stat.Losses)
			}
		}

		// Calculate agreement/disagreement win rates
		agreementTrades := 0
		disagreementTrades := 0
		for _, outcome := range outcomes {
			if outcome.Mode != stat.Mode || outcome.DecisionContext == nil {
				continue
			}
			if outcome.DecisionContext.Agreement {
				agreementTrades++
			} else {
				disagreementTrades++
			}
		}

		if agreementTrades > 0 {
			stat.AgreementWinRate = stat.AgreementWinRate / float64(agreementTrades) * 100
		}
		if disagreementTrades > 0 {
			stat.DisagreementWinRate = stat.DisagreementWinRate / float64(disagreementTrades) * 100
		}

		// Calculate confidence bracket win rates
		if stat.LowConfidenceTrades > 0 {
			stat.LowConfidenceWinRate = stat.LowConfidenceWinRate / float64(stat.LowConfidenceTrades) * 100
		}
		if stat.MedConfidenceTrades > 0 {
			stat.MedConfidenceWinRate = stat.MedConfidenceWinRate / float64(stat.MedConfidenceTrades) * 100
		}
		if stat.HighConfidenceTrades > 0 {
			stat.HighConfidenceWinRate = stat.HighConfidenceWinRate / float64(stat.HighConfidenceTrades) * 100
		}
	}

	return stats
}

// ===== RECOMMENDATION GENERATORS =====

// generateDisagreementRecommendation generates a recommendation based on agreement/disagreement patterns
func (ai *AdaptiveAI) generateDisagreementRecommendation(mode GinieTradingMode, stats *ModeStatistics) *AdaptiveRecommendation {
	// If disagreement win rate < 40%, recommend blocking on disagreement
	if stats.DisagreementWinRate > 0 && stats.DisagreementWinRate < 40 {
		// Count disagreement trades
		disagreementTrades := 0
		for _, outcome := range ai.outcomes {
			if outcome.Mode == mode && outcome.DecisionContext != nil && !outcome.DecisionContext.Agreement {
				disagreementTrades++
			}
		}

		if disagreementTrades < 5 {
			return nil // Not enough data
		}

		ai.recommendationIDSeq++
		rec := &AdaptiveRecommendation{
			ID:             fmt.Sprintf("rec_%d_%d", time.Now().Unix(), ai.recommendationIDSeq),
			CreatedAt:      time.Now(),
			Type:           "block_disagreement",
			Mode:           mode,
			CurrentValue:   false,
			SuggestedValue: true,
			Reason: fmt.Sprintf("Disagreement trades have %.1f%% win rate (below 40%%). "+
				"Based on %d trades where tech and LLM signals disagreed.",
				stats.DisagreementWinRate, disagreementTrades),
			ExpectedImprovement: fmt.Sprintf("Could improve win rate by %.1f%% by avoiding low-confidence trades",
				40-stats.DisagreementWinRate),
		}

		log.Printf("[ADAPTIVE-AI] Generated recommendation: block_disagreement for mode %s (disagreement WR: %.1f%%)",
			mode, stats.DisagreementWinRate)

		return rec
	}

	return nil
}

// generateLLMWeightRecommendation generates a recommendation for LLM weight adjustment
func (ai *AdaptiveAI) generateLLMWeightRecommendation(mode GinieTradingMode, stats *ModeStatistics) *AdaptiveRecommendation {
	// Calculate LLM-only signal win rate (when LLM disagreed with tech and was right)
	llmWins := 0
	llmTrades := 0

	for _, outcome := range ai.outcomes {
		if outcome.Mode != mode || outcome.DecisionContext == nil {
			continue
		}

		ctx := outcome.DecisionContext
		// LLM-led trade: when LLM signal was followed despite tech disagreement
		// Use LLMDirection and TechnicalDirection from DecisionContext
		// Compare outcome.Direction (the executed trade) with LLMDirection
		if !ctx.Agreement && outcome.Direction == ctx.LLMDirection {
			llmTrades++
			if outcome.Outcome == "WIN" {
				llmWins++
			}
		}
	}

	if llmTrades < 5 {
		return nil // Not enough LLM-led trades
	}

	llmWinRate := float64(llmWins) / float64(llmTrades) * 100

	// If LLM-only wins < 45%, recommend reducing LLM weight
	if llmWinRate < 45 {
		// Get current LLM weight (default 0.3)
		currentWeight := 0.3 // Default
		if ai.settingsManager != nil {
			settings := ai.settingsManager.GetCurrentSettings()
			currentWeight = settings.LLMSLTPWeight // Using this as a proxy
		}

		suggestedWeight := currentWeight - 0.05
		if suggestedWeight < 0.1 {
			suggestedWeight = 0.1
		}

		ai.recommendationIDSeq++
		rec := &AdaptiveRecommendation{
			ID:             fmt.Sprintf("rec_%d_%d", time.Now().Unix(), ai.recommendationIDSeq),
			CreatedAt:      time.Now(),
			Type:           "llm_weight",
			Mode:           mode,
			CurrentValue:   currentWeight,
			SuggestedValue: suggestedWeight,
			Reason: fmt.Sprintf("LLM-led trades have %.1f%% win rate (below 45%%). "+
				"Based on %d trades where LLM signal was followed over tech analysis.",
				llmWinRate, llmTrades),
			ExpectedImprovement: "Reducing LLM weight may improve overall decision quality",
		}

		log.Printf("[ADAPTIVE-AI] Generated recommendation: reduce llm_weight for mode %s from %.2f to %.2f (LLM WR: %.1f%%)",
			mode, currentWeight, suggestedWeight, llmWinRate)

		return rec
	}

	// If LLM-only wins > 60%, recommend increasing LLM weight
	if llmWinRate > 60 {
		currentWeight := 0.3
		if ai.settingsManager != nil {
			settings := ai.settingsManager.GetCurrentSettings()
			currentWeight = settings.LLMSLTPWeight
		}

		suggestedWeight := currentWeight + 0.05
		if suggestedWeight > 0.5 {
			suggestedWeight = 0.5
		}

		ai.recommendationIDSeq++
		rec := &AdaptiveRecommendation{
			ID:             fmt.Sprintf("rec_%d_%d", time.Now().Unix(), ai.recommendationIDSeq),
			CreatedAt:      time.Now(),
			Type:           "llm_weight",
			Mode:           mode,
			CurrentValue:   currentWeight,
			SuggestedValue: suggestedWeight,
			Reason: fmt.Sprintf("LLM-led trades have %.1f%% win rate (above 60%%). "+
				"LLM signals are outperforming in %d trades.",
				llmWinRate, llmTrades),
			ExpectedImprovement: "Increasing LLM weight may capture more winning opportunities",
		}

		log.Printf("[ADAPTIVE-AI] Generated recommendation: increase llm_weight for mode %s from %.2f to %.2f (LLM WR: %.1f%%)",
			mode, currentWeight, suggestedWeight, llmWinRate)

		return rec
	}

	return nil
}

// generateConfidenceRecommendation generates a recommendation for confidence threshold adjustment
func (ai *AdaptiveAI) generateConfidenceRecommendation(mode GinieTradingMode, stats *ModeStatistics) *AdaptiveRecommendation {
	// If low confidence trades have < 50% win rate, recommend raising min confidence
	if stats.LowConfidenceTrades >= 5 && stats.LowConfidenceWinRate < 50 {
		// Get current min confidence for mode
		currentMinConf := 60.0 // Default
		if ai.settingsManager != nil {
			modeConfig, err := ai.settingsManager.GetModeConfig(string(mode))
			if err == nil && modeConfig != nil && modeConfig.Confidence != nil {
				currentMinConf = modeConfig.Confidence.MinConfidence
			}
		}

		// If already at 65+, suggest 70; otherwise suggest 65
		suggestedMinConf := currentMinConf + 10
		if suggestedMinConf > 75 {
			suggestedMinConf = 75 // Cap at 75%
		}

		ai.recommendationIDSeq++
		rec := &AdaptiveRecommendation{
			ID:             fmt.Sprintf("rec_%d_%d", time.Now().Unix(), ai.recommendationIDSeq),
			CreatedAt:      time.Now(),
			Type:           "min_confidence",
			Mode:           mode,
			CurrentValue:   currentMinConf,
			SuggestedValue: suggestedMinConf,
			Reason: fmt.Sprintf("Low confidence trades (50-65%%) have %.1f%% win rate. "+
				"Based on %d trades in this bracket.",
				stats.LowConfidenceWinRate, stats.LowConfidenceTrades),
			ExpectedImprovement: fmt.Sprintf("Raising min confidence from %.0f%% to %.0f%% may filter out %.1f%% of losing trades",
				currentMinConf, suggestedMinConf, 50-stats.LowConfidenceWinRate),
		}

		log.Printf("[ADAPTIVE-AI] Generated recommendation: raise min_confidence for mode %s from %.0f to %.0f (low conf WR: %.1f%%)",
			mode, currentMinConf, suggestedMinConf, stats.LowConfidenceWinRate)

		return rec
	}

	return nil
}

// ===== STATISTICS RETRIEVAL =====

// GetStatisticsByMode returns statistics for all modes
func (ai *AdaptiveAI) GetStatisticsByMode() map[GinieTradingMode]*ModeStatistics {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	return ai.calculateStatsByMode(ai.outcomes)
}

// GetPendingRecommendations returns recommendations that haven't been applied or dismissed
func (ai *AdaptiveAI) GetPendingRecommendations() []AdaptiveRecommendation {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	pending := make([]AdaptiveRecommendation, 0)
	for _, rec := range ai.recommendations {
		if rec.AppliedAt == nil && !rec.Dismissed {
			pending = append(pending, rec)
		}
	}
	return pending
}

// GetAllRecommendations returns all recommendations
func (ai *AdaptiveAI) GetAllRecommendations() []AdaptiveRecommendation {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	result := make([]AdaptiveRecommendation, len(ai.recommendations))
	copy(result, ai.recommendations)
	return result
}

// ===== RECOMMENDATION ACTIONS =====

// ApplyRecommendation applies a recommendation and updates settings
func (ai *AdaptiveAI) ApplyRecommendation(id string) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	// Find the recommendation
	var rec *AdaptiveRecommendation
	for i := range ai.recommendations {
		if ai.recommendations[i].ID == id {
			rec = &ai.recommendations[i]
			break
		}
	}

	if rec == nil {
		return fmt.Errorf("recommendation not found: %s", id)
	}

	if rec.AppliedAt != nil {
		return fmt.Errorf("recommendation already applied: %s", id)
	}

	if rec.Dismissed {
		return fmt.Errorf("recommendation was dismissed: %s", id)
	}

	log.Printf("[ADAPTIVE-AI] Applying recommendation %s: %s for mode %s", id, rec.Type, rec.Mode)

	// Apply based on type
	var err error
	switch rec.Type {
	case "block_disagreement":
		// Update block_on_divergence in all ModeConfigs
		if ai.settingsManager != nil {
			settings := ai.settingsManager.GetCurrentSettings()
			// Update all mode configurations with block_on_divergence
			for _, mc := range settings.ModeConfigs {
				if mc != nil && mc.TrendDivergence != nil {
					mc.TrendDivergence.BlockOnDivergence = true
				}
			}
			err = ai.settingsManager.SaveSettings(settings)
		}

	case "llm_weight":
		// Update LLM weight
		if suggestedWeight, ok := rec.SuggestedValue.(float64); ok && ai.settingsManager != nil {
			settings := ai.settingsManager.GetCurrentSettings()
			settings.LLMSLTPWeight = suggestedWeight
			err = ai.settingsManager.SaveSettings(settings)
		}

	case "min_confidence":
		// Update min confidence for the specific mode
		if suggestedConf, ok := rec.SuggestedValue.(float64); ok && ai.settingsManager != nil {
			modeConfig, getErr := ai.settingsManager.GetModeConfig(string(rec.Mode))
			if getErr == nil && modeConfig != nil && modeConfig.Confidence != nil {
				modeConfig.Confidence.MinConfidence = suggestedConf
				err = ai.settingsManager.UpdateModeConfig(string(rec.Mode), modeConfig)
			}
		}

	default:
		return fmt.Errorf("unknown recommendation type: %s", rec.Type)
	}

	if err != nil {
		return fmt.Errorf("failed to apply recommendation: %w", err)
	}

	// Mark as applied
	now := time.Now()
	rec.AppliedAt = &now

	log.Printf("[ADAPTIVE-AI] Successfully applied recommendation %s", id)

	return nil
}

// DismissRecommendation marks a recommendation as dismissed
func (ai *AdaptiveAI) DismissRecommendation(id string) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	for i := range ai.recommendations {
		if ai.recommendations[i].ID == id {
			if ai.recommendations[i].AppliedAt != nil {
				return fmt.Errorf("cannot dismiss applied recommendation: %s", id)
			}
			ai.recommendations[i].Dismissed = true
			log.Printf("[ADAPTIVE-AI] Dismissed recommendation %s", id)
			return nil
		}
	}

	return fmt.Errorf("recommendation not found: %s", id)
}

// ===== OUTCOME RETRIEVAL =====

// getRecentOutcomesLocked returns the most recent outcomes (must hold lock)
func (ai *AdaptiveAI) getRecentOutcomesLocked(limit int) []TradeOutcome {
	if limit <= 0 || limit > len(ai.outcomes) {
		limit = len(ai.outcomes)
	}

	start := len(ai.outcomes) - limit
	if start < 0 {
		start = 0
	}

	result := make([]TradeOutcome, limit)
	copy(result, ai.outcomes[start:])
	return result
}

// GetRecentOutcomes returns the most recent outcomes
func (ai *AdaptiveAI) GetRecentOutcomes(limit int) []TradeOutcome {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	return ai.getRecentOutcomesLocked(limit)
}

// GetOutcomesByMode returns outcomes filtered by mode
func (ai *AdaptiveAI) GetOutcomesByMode(mode GinieTradingMode, limit int) []TradeOutcome {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	filtered := make([]TradeOutcome, 0)
	for i := len(ai.outcomes) - 1; i >= 0 && len(filtered) < limit; i-- {
		if ai.outcomes[i].Mode == mode {
			filtered = append(filtered, ai.outcomes[i])
		}
	}

	// Reverse to get chronological order
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	return filtered
}

// ===== AGREEMENT PATTERN ANALYSIS =====

// AgreementPatternStats contains detailed agreement pattern analysis
type AgreementPatternStats struct {
	BothAgreeLongWinRate   float64 `json:"both_agree_long_win_rate"`
	BothAgreeShortWinRate  float64 `json:"both_agree_short_win_rate"`
	TechWinsLLMHoldWinRate float64 `json:"tech_wins_llm_hold_win_rate"` // Tech says trade, LLM says hold
	LLMWinsTechHoldWinRate float64 `json:"llm_wins_tech_hold_win_rate"` // LLM says trade, Tech says hold
	DisagreementWinRate    float64 `json:"disagreement_win_rate"`
	TotalBothAgreeLong     int     `json:"total_both_agree_long"`
	TotalBothAgreeShort    int     `json:"total_both_agree_short"`
	TotalTechWinsLLMHold   int     `json:"total_tech_wins_llm_hold"`
	TotalLLMWinsTechHold   int     `json:"total_llm_wins_tech_hold"`
	TotalDisagreement      int     `json:"total_disagreement"`
}

// AnalyzeAgreementPatterns performs detailed agreement pattern analysis
func (ai *AdaptiveAI) AnalyzeAgreementPatterns(outcomes []TradeOutcome) *AgreementPatternStats {
	stats := &AgreementPatternStats{}

	bothAgreeLongWins := 0
	bothAgreeShortWins := 0
	techWinsLLMHoldWins := 0
	llmWinsTechHoldWins := 0
	disagreementWins := 0

	for _, outcome := range outcomes {
		if outcome.DecisionContext == nil {
			continue
		}

		ctx := outcome.DecisionContext
		isWin := outcome.Outcome == "WIN"

		// Use TechnicalDirection and LLMDirection from DecisionContext
		techDir := ctx.TechnicalDirection
		llmDir := ctx.LLMDirection

		// Both agree LONG
		if techDir == "LONG" && llmDir == "LONG" {
			stats.TotalBothAgreeLong++
			if isWin {
				bothAgreeLongWins++
			}
		}

		// Both agree SHORT
		if techDir == "SHORT" && llmDir == "SHORT" {
			stats.TotalBothAgreeShort++
			if isWin {
				bothAgreeShortWins++
			}
		}

		// Tech says trade (LONG/SHORT), LLM says NEUTRAL/opposite
		if (techDir == "LONG" || techDir == "SHORT") &&
			(llmDir == "NEUTRAL" || llmDir != techDir) {
			stats.TotalTechWinsLLMHold++
			if isWin {
				techWinsLLMHoldWins++
			}
		}

		// LLM says trade, Tech says NEUTRAL/opposite
		if (llmDir == "LONG" || llmDir == "SHORT") &&
			(techDir == "NEUTRAL" || techDir != llmDir) {
			stats.TotalLLMWinsTechHold++
			if isWin {
				llmWinsTechHoldWins++
			}
		}

		// General disagreement
		if !ctx.Agreement {
			stats.TotalDisagreement++
			if isWin {
				disagreementWins++
			}
		}
	}

	// Calculate percentages
	if stats.TotalBothAgreeLong > 0 {
		stats.BothAgreeLongWinRate = float64(bothAgreeLongWins) / float64(stats.TotalBothAgreeLong) * 100
	}
	if stats.TotalBothAgreeShort > 0 {
		stats.BothAgreeShortWinRate = float64(bothAgreeShortWins) / float64(stats.TotalBothAgreeShort) * 100
	}
	if stats.TotalTechWinsLLMHold > 0 {
		stats.TechWinsLLMHoldWinRate = float64(techWinsLLMHoldWins) / float64(stats.TotalTechWinsLLMHold) * 100
	}
	if stats.TotalLLMWinsTechHold > 0 {
		stats.LLMWinsTechHoldWinRate = float64(llmWinsTechHoldWins) / float64(stats.TotalLLMWinsTechHold) * 100
	}
	if stats.TotalDisagreement > 0 {
		stats.DisagreementWinRate = float64(disagreementWins) / float64(stats.TotalDisagreement) * 100
	}

	return stats
}

// ===== STATE MANAGEMENT =====

// GetAnalysisState returns the current state of the adaptive AI
func (ai *AdaptiveAI) GetAnalysisState() map[string]interface{} {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	pendingCount := 0
	for _, rec := range ai.recommendations {
		if rec.AppliedAt == nil && !rec.Dismissed {
			pendingCount++
		}
	}

	return map[string]interface{}{
		"total_outcomes":          len(ai.outcomes),
		"total_recommendations":   len(ai.recommendations),
		"pending_recommendations": pendingCount,
		"last_analysis":           ai.lastAnalysis,
		"analysis_count":          ai.analysisCount,
		"learning_window_trades":  ai.learningWindowTrades,
		"learning_window_hours":   ai.learningWindowHours,
	}
}

// SetLearningWindow updates the learning window parameters
func (ai *AdaptiveAI) SetLearningWindow(trades int, hours int) {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	if trades > 0 {
		ai.learningWindowTrades = trades
	}
	if hours > 0 {
		ai.learningWindowHours = hours
	}

	log.Printf("[ADAPTIVE-AI] Learning window updated: %d trades or %d hours",
		ai.learningWindowTrades, ai.learningWindowHours)
}

// ClearOldOutcomes removes outcomes older than the specified duration
func (ai *AdaptiveAI) ClearOldOutcomes(maxAge time.Duration) int {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	newOutcomes := make([]TradeOutcome, 0, len(ai.outcomes))

	for _, outcome := range ai.outcomes {
		if outcome.ExitTime.After(cutoff) {
			newOutcomes = append(newOutcomes, outcome)
		}
	}

	removed := len(ai.outcomes) - len(newOutcomes)
	ai.outcomes = newOutcomes

	if removed > 0 {
		log.Printf("[ADAPTIVE-AI] Cleared %d outcomes older than %v", removed, maxAge)
	}

	return removed
}

// ExportOutcomes exports outcomes as JSON
func (ai *AdaptiveAI) ExportOutcomes() ([]byte, error) {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	return json.MarshalIndent(ai.outcomes, "", "  ")
}

// ImportOutcomes imports outcomes from JSON
func (ai *AdaptiveAI) ImportOutcomes(data []byte) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	var outcomes []TradeOutcome
	if err := json.Unmarshal(data, &outcomes); err != nil {
		return fmt.Errorf("failed to unmarshal outcomes: %w", err)
	}

	ai.outcomes = append(ai.outcomes, outcomes...)

	// Trim if needed
	if len(ai.outcomes) > ai.maxOutcomes {
		trimCount := len(ai.outcomes) - ai.maxOutcomes
		ai.outcomes = ai.outcomes[trimCount:]
	}

	log.Printf("[ADAPTIVE-AI] Imported %d outcomes, total: %d", len(outcomes), len(ai.outcomes))

	return nil
}
