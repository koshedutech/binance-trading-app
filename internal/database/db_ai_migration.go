package database

import (
	"context"
	"log"
)

// RunAIMigrations runs AI-related database migrations
func (db *DB) RunAIMigrations(ctx context.Context) error {
	log.Println("Running AI database migrations...")

	migrations := []string{
		// Create AI decisions table for autopilot signal tracking
		`CREATE TABLE IF NOT EXISTS ai_decisions (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL,
			current_price DECIMAL(20, 8) NOT NULL,
			action VARCHAR(20) NOT NULL,
			confidence DECIMAL(5, 4),
			reasoning TEXT,
			signals JSONB NOT NULL,
			ml_direction VARCHAR(20),
			ml_confidence DECIMAL(5, 4),
			sentiment_direction VARCHAR(20),
			sentiment_confidence DECIMAL(5, 4),
			llm_direction VARCHAR(20),
			llm_confidence DECIMAL(5, 4),
			pattern_direction VARCHAR(20),
			pattern_confidence DECIMAL(5, 4),
			bigcandle_direction VARCHAR(20),
			bigcandle_confidence DECIMAL(5, 4),
			confluence_count INT DEFAULT 0,
			risk_level VARCHAR(20),
			executed BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_decisions_symbol ON ai_decisions(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_decisions_action ON ai_decisions(action)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_decisions_created_at ON ai_decisions(created_at DESC)`,
	}

	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			return err
		}
		log.Printf("AI migration %d completed", i+1)
	}

	log.Println("AI database migrations completed successfully")
	return nil
}

// RunTradeAILinkMigration adds AI decision linking to trades
func (db *DB) RunTradeAILinkMigration(ctx context.Context) error {
	migrations := []string{
		// Add AI decision ID to trades
		`ALTER TABLE trades ADD COLUMN IF NOT EXISTS ai_decision_id INTEGER REFERENCES ai_decisions(id)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_ai_decision ON trades(ai_decision_id)`,
		
		// Add trailing stop fields to trades
		`ALTER TABLE trades ADD COLUMN IF NOT EXISTS trailing_stop_enabled BOOLEAN DEFAULT FALSE`,
		`ALTER TABLE trades ADD COLUMN IF NOT EXISTS trailing_stop_percent DECIMAL(5, 2)`,
		`ALTER TABLE trades ADD COLUMN IF NOT EXISTS highest_price DECIMAL(20, 8)`,
		`ALTER TABLE trades ADD COLUMN IF NOT EXISTS lowest_price DECIMAL(20, 8)`,
		
		// Add order IDs for TP/SL tracking
		`ALTER TABLE trades ADD COLUMN IF NOT EXISTS take_profit_order_id BIGINT`,
		`ALTER TABLE trades ADD COLUMN IF NOT EXISTS stop_loss_order_id BIGINT`,
	}

	for _, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			// Ignore errors for columns that already exist
			continue
		}
	}

	return nil
}
