package autopilot

import (
	"binance-trading-bot/internal/database"
	"context"
	"log"
)

// SetRepository sets the database repository for saving decisions
func (c *Controller) SetRepository(repo *database.Repository) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.repository = repo
}

// saveDecisionToDB saves an AI decision to the database and returns the decision ID
func (c *Controller) saveDecisionToDB(symbol string, currentPrice float64, decision *TradingDecision, signals map[string]Signal) *int64 {
	if c.repository == nil {
		return nil
	}

	// Build the AI decision record
	aiDecision := &database.AIDecision{
		Symbol:       symbol,
		CurrentPrice: currentPrice,
		Action:       decision.Action,
		Confidence:   decision.Confidence,
		Reasoning:    decision.Reasoning,
		Signals:      make(map[string]interface{}),
		RiskLevel:    c.config.RiskLevel,
		Executed:     decision.Approved,
	}

	// Count confluence
	confluenceCount := 0
	decisionDir := decision.Direction
	if decisionDir == "" {
		// For hold decisions, determine majority direction
		longCount := 0
		shortCount := 0
		for _, sig := range signals {
			if sig.Direction == "long" || sig.Direction == "up" {
				longCount++
			} else if sig.Direction == "short" || sig.Direction == "down" {
				shortCount++
			}
		}
		if longCount > shortCount {
			decisionDir = "long"
		} else if shortCount > longCount {
			decisionDir = "short"
		}
	}

	// Extract individual signal data
	for source, sig := range signals {
		aiDecision.Signals[source] = map[string]interface{}{
			"direction":  sig.Direction,
			"confidence": sig.Confidence,
			"reason":     sig.Reason,
		}

		dir := sig.Direction
		conf := sig.Confidence

		switch source {
		case "ml":
			aiDecision.MLDirection = &dir
			aiDecision.MLConfidence = &conf
		case "sentiment":
			aiDecision.SentimentDirection = &dir
			aiDecision.SentimentConfidence = &conf
		case "llm":
			aiDecision.LLMDirection = &dir
			aiDecision.LLMConfidence = &conf
		case "big_candle":
			aiDecision.BigCandleDirection = &dir
			aiDecision.BigCandleConfidence = &conf
		case "pattern", "scalping":
			aiDecision.PatternDirection = &dir
			aiDecision.PatternConfidence = &conf
		}

		// Count signals that agree with the decision direction
		normalizedSigDir := sig.Direction
		if normalizedSigDir == "up" {
			normalizedSigDir = "long"
		} else if normalizedSigDir == "down" {
			normalizedSigDir = "short"
		}
		if decisionDir != "" && normalizedSigDir == decisionDir {
			confluenceCount++
		}
	}

	aiDecision.ConfluenceCount = confluenceCount

	// Save to database
	ctx := context.Background()
	if err := c.repository.SaveAIDecision(ctx, aiDecision); err != nil {
		log.Printf("[Autopilot] Failed to save AI decision: %v", err)
		return nil
	}

	return &aiDecision.ID
}
