package database

import (
	"context"
	"log"
)

// RunPatternStateMigration creates the pattern_states table for persisting
// entry decision pattern detection state across server restarts.
// Epic 14: Pattern State Persistence
func (db *DB) RunPatternStateMigration(ctx context.Context) error {
	log.Println("Running Pattern State database migration...")

	migrations := []string{
		// Create pattern_states table for persisting pattern detection state
		`CREATE TABLE IF NOT EXISTS pattern_states (
			id SERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			mode TEXT NOT NULL,
			timeframe TEXT NOT NULL,
			strategy TEXT NOT NULL DEFAULT '',
			sub_strategy TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			current_step INTEGER NOT NULL DEFAULT 1,
			direction TEXT NOT NULL DEFAULT 'long',
			reference_candle JSONB,
			consolidation_data JSONB,
			entry_levels JSONB,
			pattern_data JSONB,
			started_at TIMESTAMP WITH TIME ZONE,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			expires_at TIMESTAMP WITH TIME ZONE,
			UNIQUE(user_id, symbol, mode, timeframe)
		)`,

		// Indexes for common queries
		`CREATE INDEX IF NOT EXISTS idx_pattern_states_user_id
			ON pattern_states(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pattern_states_user_symbol
			ON pattern_states(user_id, symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_pattern_states_status
			ON pattern_states(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pattern_states_updated_at
			ON pattern_states(updated_at DESC)`,
	}

	for i, migration := range migrations {
		if _, err := db.Pool.Exec(ctx, migration); err != nil {
			log.Printf("Pattern State migration %d failed: %v", i+1, err)
			continue
		}
	}

	log.Println("Pattern State database migration completed")
	return nil
}
