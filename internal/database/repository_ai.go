package database

import (
	"context"
	"encoding/json"
	"time"
)

// SaveAIDecision saves an AI decision to the database
func (r *Repository) SaveAIDecision(ctx context.Context, decision *AIDecision) error {
	signalsJSON, err := json.Marshal(decision.Signals)
	if err != nil {
		signalsJSON = []byte("{}")
	}

	query := `
		INSERT INTO ai_decisions (
			symbol, current_price, action, confidence, reasoning, signals,
			ml_direction, ml_confidence, sentiment_direction, sentiment_confidence,
			llm_direction, llm_confidence, pattern_direction, pattern_confidence,
			bigcandle_direction, bigcandle_confidence, confluence_count, risk_level, executed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING id, created_at`

	return r.db.Pool.QueryRow(ctx, query,
		decision.Symbol,
		decision.CurrentPrice,
		decision.Action,
		decision.Confidence,
		decision.Reasoning,
		signalsJSON,
		decision.MLDirection,
		decision.MLConfidence,
		decision.SentimentDirection,
		decision.SentimentConfidence,
		decision.LLMDirection,
		decision.LLMConfidence,
		decision.PatternDirection,
		decision.PatternConfidence,
		decision.BigCandleDirection,
		decision.BigCandleConfidence,
		decision.ConfluenceCount,
		decision.RiskLevel,
		decision.Executed,
	).Scan(&decision.ID, &decision.CreatedAt)
}

// GetAIDecisions retrieves AI decisions with optional filters
func (r *Repository) GetAIDecisions(ctx context.Context, limit int, symbol string, action string) ([]AIDecision, error) {
	query := `
		SELECT id, symbol, current_price, action, confidence, reasoning, signals,
			ml_direction, ml_confidence, sentiment_direction, sentiment_confidence,
			llm_direction, llm_confidence, pattern_direction, pattern_confidence,
			bigcandle_direction, bigcandle_confidence, confluence_count, risk_level, executed, created_at
		FROM ai_decisions
		WHERE ($1 = '' OR symbol = $1)
		AND ($2 = '' OR action = $2)
		ORDER BY created_at DESC
		LIMIT $3`

	rows, err := r.db.Pool.Query(ctx, query, symbol, action, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decisions []AIDecision
	for rows.Next() {
		var d AIDecision
		var signalsJSON []byte

		err := rows.Scan(
			&d.ID, &d.Symbol, &d.CurrentPrice, &d.Action, &d.Confidence, &d.Reasoning, &signalsJSON,
			&d.MLDirection, &d.MLConfidence, &d.SentimentDirection, &d.SentimentConfidence,
			&d.LLMDirection, &d.LLMConfidence, &d.PatternDirection, &d.PatternConfidence,
			&d.BigCandleDirection, &d.BigCandleConfidence, &d.ConfluenceCount, &d.RiskLevel, &d.Executed, &d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if len(signalsJSON) > 0 {
			json.Unmarshal(signalsJSON, &d.Signals)
		}

		decisions = append(decisions, d)
	}

	return decisions, nil
}

// GetAIDecisionStats returns statistics about AI decisions
func (r *Repository) GetAIDecisionStats(ctx context.Context, since time.Time) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN action = 'buy' THEN 1 END) as buy_decisions,
			COUNT(CASE WHEN action = 'sell' THEN 1 END) as sell_decisions,
			COUNT(CASE WHEN action = 'hold' THEN 1 END) as hold_decisions,
			COUNT(CASE WHEN executed = true THEN 1 END) as executed,
			AVG(confidence) as avg_confidence,
			AVG(confluence_count) as avg_confluence
		FROM ai_decisions
		WHERE created_at >= $1`

	var total, buyDecisions, sellDecisions, holdDecisions, executed int
	var avgConfidence, avgConfluence *float64

	err := r.db.Pool.QueryRow(ctx, query, since).Scan(
		&total, &buyDecisions, &sellDecisions, &holdDecisions, &executed, &avgConfidence, &avgConfluence,
	)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total":          total,
		"buy_decisions":  buyDecisions,
		"sell_decisions": sellDecisions,
		"hold_decisions": holdDecisions,
		"executed":       executed,
		"avg_confidence": avgConfidence,
		"avg_confluence": avgConfluence,
	}

	return stats, nil
}

// CleanupOldAIDecisions removes AI decisions older than the specified duration
func (r *Repository) CleanupOldAIDecisions(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.Pool.Exec(ctx, "DELETE FROM ai_decisions WHERE created_at < $1", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// UpdateTradeTrailingInfo updates trailing stop info for a trade
func (r *Repository) UpdateTradeTrailingInfo(ctx context.Context, tradeID int64, highestPrice, lowestPrice, stopLoss float64) error {
	query := `UPDATE trades SET highest_price = $1, lowest_price = $2, stop_loss = $3, updated_at = NOW() WHERE id = $4`
	_, err := r.db.Pool.Exec(ctx, query, highestPrice, lowestPrice, stopLoss, tradeID)
	return err
}

// GetTradeWithAIDecision returns a trade with its linked AI decision
func (r *Repository) GetTradeWithAIDecision(ctx context.Context, tradeID int64) (*Trade, error) {
	query := `
		SELECT t.id, t.symbol, t.side, t.entry_price, t.exit_price, t.quantity,
			t.entry_time, t.exit_time, t.stop_loss, t.take_profit, t.pnl, t.pnl_percent,
			t.strategy_name, t.status, t.created_at, t.updated_at,
			t.ai_decision_id, t.trailing_stop_enabled, t.trailing_stop_percent,
			t.highest_price, t.lowest_price, t.take_profit_order_id, t.stop_loss_order_id
		FROM trades t
		WHERE t.id = $1`

	var trade Trade
	err := r.db.Pool.QueryRow(ctx, query, tradeID).Scan(
		&trade.ID, &trade.Symbol, &trade.Side, &trade.EntryPrice, &trade.ExitPrice, &trade.Quantity,
		&trade.EntryTime, &trade.ExitTime, &trade.StopLoss, &trade.TakeProfit, &trade.PnL, &trade.PnLPercent,
		&trade.StrategyName, &trade.Status, &trade.CreatedAt, &trade.UpdatedAt,
		&trade.AIDecisionID, &trade.TrailingStopEnabled, &trade.TrailingStopPercent,
		&trade.HighestPrice, &trade.LowestPrice, &trade.TakeProfitOrderID, &trade.StopLossOrderID,
	)
	if err != nil {
		return nil, err
	}

	// Load AI decision if linked
	if trade.AIDecisionID != nil {
		aiDecision, err := r.GetAIDecisionByID(ctx, *trade.AIDecisionID)
		if err == nil {
			trade.AIDecision = aiDecision
		}
	}

	return &trade, nil
}

// GetAIDecisionByID gets a single AI decision by ID
func (r *Repository) GetAIDecisionByID(ctx context.Context, id int64) (*AIDecision, error) {
	query := `
		SELECT id, symbol, current_price, action, confidence, reasoning, signals,
			ml_direction, ml_confidence, sentiment_direction, sentiment_confidence,
			llm_direction, llm_confidence, pattern_direction, pattern_confidence,
			bigcandle_direction, bigcandle_confidence, confluence_count, risk_level, executed, created_at
		FROM ai_decisions WHERE id = $1`

	var d AIDecision
	var signalsJSON []byte

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&d.ID, &d.Symbol, &d.CurrentPrice, &d.Action, &d.Confidence, &d.Reasoning, &signalsJSON,
		&d.MLDirection, &d.MLConfidence, &d.SentimentDirection, &d.SentimentConfidence,
		&d.LLMDirection, &d.LLMConfidence, &d.PatternDirection, &d.PatternConfidence,
		&d.BigCandleDirection, &d.BigCandleConfidence, &d.ConfluenceCount, &d.RiskLevel, &d.Executed, &d.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(signalsJSON) > 0 {
		json.Unmarshal(signalsJSON, &d.Signals)
	}

	return &d, nil
}

// LinkTradeToAIDecision links a trade to an AI decision
func (r *Repository) LinkTradeToAIDecision(ctx context.Context, tradeID, aiDecisionID int64) error {
	query := `UPDATE trades SET ai_decision_id = $1 WHERE id = $2`
	_, err := r.db.Pool.Exec(ctx, query, aiDecisionID, tradeID)
	return err
}

// CreateFuturesTrade creates a new futures trade - wrapper for DB method
func (r *Repository) CreateFuturesTrade(ctx context.Context, trade *FuturesTrade) error {
	return r.db.CreateFuturesTrade(ctx, trade)
}

// GetOpenTradesWithAIDecisions returns open trades with their AI decisions
func (r *Repository) GetOpenTradesWithAIDecisions(ctx context.Context) ([]Trade, error) {
	query := `
		SELECT t.id, t.symbol, t.side, t.entry_price, t.exit_price, t.quantity,
			t.entry_time, t.exit_time, t.stop_loss, t.take_profit, t.pnl, t.pnl_percent,
			t.strategy_name, t.status, t.created_at, t.updated_at,
			COALESCE(t.ai_decision_id, 0), 
			COALESCE(t.trailing_stop_enabled, false), t.trailing_stop_percent,
			t.highest_price, t.lowest_price, t.take_profit_order_id, t.stop_loss_order_id
		FROM trades t
		WHERE t.status = 'OPEN'
		ORDER BY t.entry_time DESC`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var trade Trade
		var aiDecisionID int64
		err := rows.Scan(
			&trade.ID, &trade.Symbol, &trade.Side, &trade.EntryPrice, &trade.ExitPrice, &trade.Quantity,
			&trade.EntryTime, &trade.ExitTime, &trade.StopLoss, &trade.TakeProfit, &trade.PnL, &trade.PnLPercent,
			&trade.StrategyName, &trade.Status, &trade.CreatedAt, &trade.UpdatedAt,
			&aiDecisionID, &trade.TrailingStopEnabled, &trade.TrailingStopPercent,
			&trade.HighestPrice, &trade.LowestPrice, &trade.TakeProfitOrderID, &trade.StopLossOrderID,
		)
		if err != nil {
			continue
		}

		if aiDecisionID > 0 {
			trade.AIDecisionID = &aiDecisionID
			// Load AI decision
			aiDecision, err := r.GetAIDecisionByID(ctx, aiDecisionID)
			if err == nil {
				trade.AIDecision = aiDecision
			}
		}

		trades = append(trades, trade)
	}

	return trades, nil
}
