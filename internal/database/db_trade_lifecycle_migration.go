package database

import (
	"context"
	"log"
)

// RunTradeLifecycleMigrations runs trade lifecycle event tracking migrations
func (db *DB) RunTradeLifecycleMigrations(ctx context.Context) error {
	log.Println("Running Trade Lifecycle database migrations...")

	migrations := []string{
		// Create trade_lifecycle_events table for comprehensive trade tracking
		`CREATE TABLE IF NOT EXISTS trade_lifecycle_events (
			id BIGSERIAL PRIMARY KEY,
			futures_trade_id INTEGER REFERENCES futures_trades(id) ON DELETE CASCADE,

			-- Event identification
			event_type VARCHAR(50) NOT NULL,
			event_subtype VARCHAR(50),

			-- Timing
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

			-- Price data
			trigger_price DECIMAL(20, 8),
			old_value DECIMAL(20, 8),
			new_value DECIMAL(20, 8),

			-- Context
			mode VARCHAR(20),
			source VARCHAR(20) DEFAULT 'ginie',

			-- For TP events
			tp_level INT,
			quantity_closed DECIMAL(20, 8),
			pnl_realized DECIMAL(20, 8),
			pnl_percent DECIMAL(10, 4),

			-- For SL events
			sl_revision_count INT DEFAULT 0,

			-- Conditions that triggered the event
			conditions_met JSONB,

			-- Additional context
			reason TEXT,
			details JSONB,

			-- Metadata
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Indexes for common queries
		`CREATE INDEX IF NOT EXISTS idx_trade_lifecycle_futures_trade_id
			ON trade_lifecycle_events(futures_trade_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trade_lifecycle_event_type
			ON trade_lifecycle_events(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_trade_lifecycle_timestamp
			ON trade_lifecycle_events(timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_trade_lifecycle_created_at
			ON trade_lifecycle_events(created_at DESC)`,

		// Add user_id for multi-tenant support
		`ALTER TABLE trade_lifecycle_events
			ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id)`,
		`CREATE INDEX IF NOT EXISTS idx_trade_lifecycle_user_id
			ON trade_lifecycle_events(user_id)`,

		// Composite index for efficient trade timeline queries
		`CREATE INDEX IF NOT EXISTS idx_trade_lifecycle_trade_timeline
			ON trade_lifecycle_events(futures_trade_id, timestamp ASC)`,
	}

	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			log.Printf("Trade lifecycle migration %d failed: %v", i+1, err)
			continue
		}
		log.Printf("Trade lifecycle migration %d completed", i+1)
	}

	log.Println("Trade lifecycle database migrations completed")
	return nil
}
