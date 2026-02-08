package autopilot

import (
	"context"

	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/entrydecision"
)

// PatternStatePersisterAdapter adapts the database Repository to implement
// the entrydecision.PatternStatePersister interface.
// This lives in the autopilot package to avoid circular imports between
// database and entrydecision packages.
type PatternStatePersisterAdapter struct {
	repo *database.Repository
}

// NewPatternStatePersisterAdapter creates a new adapter for pattern state persistence.
func NewPatternStatePersisterAdapter(repo *database.Repository) *PatternStatePersisterAdapter {
	return &PatternStatePersisterAdapter{repo: repo}
}

// SavePatternStateToDB saves a pattern state to the database (upsert).
func (a *PatternStatePersisterAdapter) SavePatternStateToDB(ctx context.Context, state *entrydecision.PersistedPatternState) error {
	record := &database.PatternStateRecord{
		UserID:            state.UserID,
		Symbol:            state.Symbol,
		Mode:              state.Mode,
		Timeframe:         state.Timeframe,
		Strategy:          state.Strategy,
		SubStrategy:       state.SubStrategy,
		Status:            state.Status,
		CurrentStep:       state.CurrentStep,
		Direction:         state.Direction,
		ReferenceCandle:   state.ReferenceCandle,
		ConsolidationData: state.ConsolidationData,
		EntryLevels:       state.EntryLevels,
		PatternData:       state.PatternData,
		StartedAt:         state.StartedAt,
		UpdatedAt:         state.UpdatedAt,
		ExpiresAt:         state.ExpiresAt,
	}
	return a.repo.SavePatternState(ctx, record)
}

// GetPatternStatesFromDB retrieves all active pattern states for a user.
func (a *PatternStatePersisterAdapter) GetPatternStatesFromDB(ctx context.Context, userID string) ([]entrydecision.PersistedPatternState, error) {
	records, err := a.repo.GetPatternStates(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]entrydecision.PersistedPatternState, len(records))
	for i, r := range records {
		result[i] = entrydecision.PersistedPatternState{
			UserID:            r.UserID,
			Symbol:            r.Symbol,
			Mode:              r.Mode,
			Timeframe:         r.Timeframe,
			Strategy:          r.Strategy,
			SubStrategy:       r.SubStrategy,
			Status:            r.Status,
			CurrentStep:       r.CurrentStep,
			Direction:         r.Direction,
			ReferenceCandle:   r.ReferenceCandle,
			ConsolidationData: r.ConsolidationData,
			EntryLevels:       r.EntryLevels,
			PatternData:       r.PatternData,
			StartedAt:         r.StartedAt,
			UpdatedAt:         r.UpdatedAt,
			ExpiresAt:         r.ExpiresAt,
		}
	}
	return result, nil
}

// DeletePatternStateFromDB removes a specific pattern state.
func (a *PatternStatePersisterAdapter) DeletePatternStateFromDB(ctx context.Context, userID, symbol, mode, timeframe string) error {
	return a.repo.DeletePatternState(ctx, userID, symbol, mode, timeframe)
}

// DeleteAllPatternStatesFromDB removes all pattern states for a user.
func (a *PatternStatePersisterAdapter) DeleteAllPatternStatesFromDB(ctx context.Context, userID string) error {
	return a.repo.DeleteAllPatternStates(ctx, userID)
}

// DeleteStalePatternStatesFromDB removes all non-position_running pattern states for a user.
func (a *PatternStatePersisterAdapter) DeleteStalePatternStatesFromDB(ctx context.Context, userID string) error {
	return a.repo.DeleteNonPositionPatternStates(ctx, userID)
}
