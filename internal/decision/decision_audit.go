// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.19: Decision Output Interface
package decision

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// DefaultAuditMaxHistory is the default maximum history entries per symbol
const DefaultAuditMaxHistory = 1000

// DecisionAuditService manages decision audit trail.
// It provides in-memory storage with thread-safe access for tracking all decisions.
type DecisionAuditService struct {
	// Thread-safe storage
	decisions sync.Map // key: decisionID, value: *PositionDecision
	bySymbol  sync.Map // key: symbol, value: []string (decision IDs)
	byUser    sync.Map // key: userID, value: []string (decision IDs)

	// Configuration
	maxHistoryPerSymbol int
	mu                  sync.RWMutex // Protects bySymbol index
	userMu              sync.RWMutex // Protects byUser index (separate to avoid deadlock)
}

// NewDecisionAuditService creates a new DecisionAuditService
func NewDecisionAuditService() *DecisionAuditService {
	return &DecisionAuditService{
		maxHistoryPerSymbol: DefaultAuditMaxHistory,
	}
}

// NewDecisionAuditServiceWithMaxHistory creates a service with custom max history
func NewDecisionAuditServiceWithMaxHistory(maxHistory int) *DecisionAuditService {
	if maxHistory <= 0 {
		maxHistory = DefaultAuditMaxHistory
	}
	return &DecisionAuditService{
		maxHistoryPerSymbol: maxHistory,
	}
}

// RecordDecision stores a decision for audit
func (s *DecisionAuditService) RecordDecision(d *PositionDecision) error {
	if d == nil {
		return fmt.Errorf("decision cannot be nil")
	}
	if d.ID == "" {
		return fmt.Errorf("decision ID cannot be empty")
	}
	if d.Symbol == "" {
		return fmt.Errorf("decision symbol cannot be empty")
	}
	if d.UserID <= 0 {
		return fmt.Errorf("invalid user ID: %d", d.UserID)
	}

	// Store a copy to prevent external mutation
	s.decisions.Store(d.ID, d.Copy())

	// Update by-symbol index
	s.addToSymbolIndex(d.Symbol, d.ID)

	// Update by-user index
	s.addToUserIndex(d.UserID, d.ID)

	return nil
}

// addToSymbolIndex adds a decision ID to the symbol index with limit enforcement
func (s *DecisionAuditService) addToSymbolIndex(symbol, decisionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get existing list or create new
	existing, _ := s.bySymbol.Load(symbol)
	var ids []string
	if existing != nil {
		ids = existing.([]string)
	}

	// Add new ID
	ids = append(ids, decisionID)

	// Trim if over limit
	if len(ids) > s.maxHistoryPerSymbol {
		// Remove oldest decisions from main store and user index
		toRemove := ids[:len(ids)-s.maxHistoryPerSymbol]
		for _, id := range toRemove {
			// Get the decision to find its userID before deleting
			if val, ok := s.decisions.Load(id); ok {
				if d, ok := val.(*PositionDecision); ok {
					s.removeFromUserIndex(d.UserID, id)
				}
			}
			s.decisions.Delete(id)
		}
		ids = ids[len(ids)-s.maxHistoryPerSymbol:]
	}

	s.bySymbol.Store(symbol, ids)
}

// removeFromUserIndex removes a decision ID from the user index
func (s *DecisionAuditService) removeFromUserIndex(userID int, decisionID string) {
	key := fmt.Sprintf("%d", userID)

	s.userMu.Lock()
	defer s.userMu.Unlock()

	existing, ok := s.byUser.Load(key)
	if !ok {
		return
	}

	ids := existing.([]string)
	// Find and remove the decision ID
	for i, id := range ids {
		if id == decisionID {
			ids = append(ids[:i], ids[i+1:]...)
			break
		}
	}

	if len(ids) == 0 {
		s.byUser.Delete(key)
	} else {
		s.byUser.Store(key, ids)
	}
}

// addToUserIndex adds a decision ID to the user index with limit enforcement
func (s *DecisionAuditService) addToUserIndex(userID int, decisionID string) {
	key := fmt.Sprintf("%d", userID)

	// Use separate mutex for user index to avoid deadlock with addToSymbolIndex
	s.userMu.Lock()
	defer s.userMu.Unlock()

	// Get existing list or create new
	existing, _ := s.byUser.Load(key)
	var ids []string
	if existing != nil {
		ids = existing.([]string)
	}

	// Add new ID
	ids = append(ids, decisionID)

	// Trim user index as well
	if len(ids) > s.maxHistoryPerSymbol*10 { // Allow more per user
		ids = ids[len(ids)-s.maxHistoryPerSymbol*10:]
	}

	s.byUser.Store(key, ids)
}

// GetDecision retrieves a decision by ID
func (s *DecisionAuditService) GetDecision(id string) (*PositionDecision, error) {
	if id == "" {
		return nil, fmt.Errorf("decision ID cannot be empty")
	}

	value, ok := s.decisions.Load(id)
	if !ok {
		return nil, fmt.Errorf("decision not found: %s", id)
	}

	decision := value.(*PositionDecision)
	return decision.Copy(), nil // Return copy to prevent mutation
}

// GetDecisionHistory retrieves decision history for a symbol
func (s *DecisionAuditService) GetDecisionHistory(symbol string, limit int) ([]*PositionDecision, error) {
	if symbol == "" {
		return nil, fmt.Errorf("symbol cannot be empty")
	}
	if limit <= 0 {
		limit = 100
	}

	// Get decision IDs for symbol
	value, ok := s.bySymbol.Load(symbol)
	if !ok {
		return []*PositionDecision{}, nil
	}

	ids := value.([]string)

	// Get most recent decisions (from end of list)
	start := len(ids) - limit
	if start < 0 {
		start = 0
	}
	recentIDs := ids[start:]

	// Fetch decisions
	decisions := make([]*PositionDecision, 0, len(recentIDs))
	for i := len(recentIDs) - 1; i >= 0; i-- { // Reverse order (most recent first)
		if d, err := s.GetDecision(recentIDs[i]); err == nil {
			decisions = append(decisions, d)
		}
	}

	return decisions, nil
}

// GetUserDecisions retrieves all decisions for a user
func (s *DecisionAuditService) GetUserDecisions(userID int, limit int) ([]*PositionDecision, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}
	if limit <= 0 {
		limit = 100
	}

	key := fmt.Sprintf("%d", userID)
	value, ok := s.byUser.Load(key)
	if !ok {
		return []*PositionDecision{}, nil
	}

	ids := value.([]string)

	// Get most recent decisions
	start := len(ids) - limit
	if start < 0 {
		start = 0
	}
	recentIDs := ids[start:]

	// Fetch decisions
	decisions := make([]*PositionDecision, 0, len(recentIDs))
	for i := len(recentIDs) - 1; i >= 0; i-- {
		if d, err := s.GetDecision(recentIDs[i]); err == nil {
			decisions = append(decisions, d)
		}
	}

	return decisions, nil
}

// MarkExecuted updates decision with execution info
func (s *DecisionAuditService) MarkExecuted(decisionID string, orderID string, executedAt time.Time) error {
	if decisionID == "" {
		return fmt.Errorf("decision ID cannot be empty")
	}
	if orderID == "" {
		return fmt.Errorf("order ID cannot be empty")
	}

	value, ok := s.decisions.Load(decisionID)
	if !ok {
		return fmt.Errorf("decision not found: %s", decisionID)
	}

	decision := value.(*PositionDecision)
	decision.ExecutedAt = &executedAt
	decision.OrderID = orderID

	// Store updated decision
	s.decisions.Store(decisionID, decision)
	return nil
}

// MarkError records an execution error
func (s *DecisionAuditService) MarkError(decisionID string, err error) error {
	if decisionID == "" {
		return fmt.Errorf("decision ID cannot be empty")
	}

	value, ok := s.decisions.Load(decisionID)
	if !ok {
		return fmt.Errorf("decision not found: %s", decisionID)
	}

	decision := value.(*PositionDecision)
	if err != nil {
		decision.ExecutionError = err.Error()
	}

	s.decisions.Store(decisionID, decision)
	return nil
}

// GetRecentDecisions returns most recent N decisions across all symbols
func (s *DecisionAuditService) GetRecentDecisions(limit int) ([]*PositionDecision, error) {
	if limit <= 0 {
		limit = 100
	}

	// Collect all decisions
	var allDecisions []*PositionDecision
	s.decisions.Range(func(key, value interface{}) bool {
		if d, ok := value.(*PositionDecision); ok {
			allDecisions = append(allDecisions, d.Copy())
		}
		return true
	})

	// Sort by timestamp (most recent first)
	sort.Slice(allDecisions, func(i, j int) bool {
		return allDecisions[i].Timestamp.After(allDecisions[j].Timestamp)
	})

	// Return limited results
	if len(allDecisions) > limit {
		allDecisions = allDecisions[:limit]
	}

	return allDecisions, nil
}

// RollbackDecision marks a decision as rolled back (for audit)
func (s *DecisionAuditService) RollbackDecision(decisionID string, reason string) error {
	if decisionID == "" {
		return fmt.Errorf("decision ID cannot be empty")
	}

	value, ok := s.decisions.Load(decisionID)
	if !ok {
		return fmt.Errorf("decision not found: %s", decisionID)
	}

	decision := value.(*PositionDecision)
	decision.MarkRolledBack(reason)

	s.decisions.Store(decisionID, decision)
	return nil
}

// GetDecisionChainCount returns the current chain count for a symbol
func (s *DecisionAuditService) GetDecisionChainCount(symbol string) int {
	value, ok := s.bySymbol.Load(symbol)
	if !ok {
		return 0
	}
	return len(value.([]string))
}

// GetLastDecisionForSymbol returns the most recent decision for a symbol
func (s *DecisionAuditService) GetLastDecisionForSymbol(symbol string) (*PositionDecision, error) {
	if symbol == "" {
		return nil, fmt.Errorf("symbol cannot be empty")
	}

	value, ok := s.bySymbol.Load(symbol)
	if !ok {
		return nil, nil
	}

	ids := value.([]string)
	if len(ids) == 0 {
		return nil, nil
	}

	// Get the last (most recent) decision
	return s.GetDecision(ids[len(ids)-1])
}

// GetExecutedDecisions returns decisions that were executed
func (s *DecisionAuditService) GetExecutedDecisions(userID int, limit int) ([]*PositionDecision, error) {
	decisions, err := s.GetUserDecisions(userID, limit*2) // Fetch more to filter
	if err != nil {
		return nil, err
	}

	var executed []*PositionDecision
	for _, d := range decisions {
		if d.IsExecuted() {
			executed = append(executed, d)
			if len(executed) >= limit {
				break
			}
		}
	}

	return executed, nil
}

// GetBlockedDecisions returns decisions that were blocked
func (s *DecisionAuditService) GetBlockedDecisions(userID int, limit int) ([]*PositionDecision, error) {
	decisions, err := s.GetUserDecisions(userID, limit*2)
	if err != nil {
		return nil, err
	}

	var blocked []*PositionDecision
	for _, d := range decisions {
		if d.IsBlocked {
			blocked = append(blocked, d)
			if len(blocked) >= limit {
				break
			}
		}
	}

	return blocked, nil
}

// ClearSymbolHistory removes all decision history for a symbol
func (s *DecisionAuditService) ClearSymbolHistory(symbol string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.bySymbol.Load(symbol)
	if !ok {
		return
	}

	ids := value.([]string)
	for _, id := range ids {
		s.decisions.Delete(id)
	}

	s.bySymbol.Delete(symbol)
}

// ClearUserHistory removes all decision history for a user
func (s *DecisionAuditService) ClearUserHistory(userID int) {
	s.userMu.Lock()
	defer s.userMu.Unlock()

	key := fmt.Sprintf("%d", userID)

	value, ok := s.byUser.Load(key)
	if !ok {
		return
	}

	ids := value.([]string)
	for _, id := range ids {
		s.decisions.Delete(id)
	}

	s.byUser.Delete(key)
}

// ClearAll removes all decision history
func (s *DecisionAuditService) ClearAll() {
	s.decisions = sync.Map{}
	s.bySymbol = sync.Map{}
	s.byUser = sync.Map{}
}

// GetStats returns statistics about the audit service
type AuditStats struct {
	TotalDecisions    int `json:"total_decisions"`
	UniqueSymbols     int `json:"unique_symbols"`
	UniqueUsers       int `json:"unique_users"`
	ExecutedDecisions int `json:"executed_decisions"`
	BlockedDecisions  int `json:"blocked_decisions"`
	ErrorDecisions    int `json:"error_decisions"`
}

// GetStats returns current statistics
func (s *DecisionAuditService) GetStats() AuditStats {
	stats := AuditStats{}

	// Count decisions and stats
	s.decisions.Range(func(key, value interface{}) bool {
		stats.TotalDecisions++
		if d, ok := value.(*PositionDecision); ok {
			if d.IsExecuted() {
				stats.ExecutedDecisions++
			}
			if d.IsBlocked {
				stats.BlockedDecisions++
			}
			if d.HasError() {
				stats.ErrorDecisions++
			}
		}
		return true
	})

	// Count unique symbols
	s.bySymbol.Range(func(key, value interface{}) bool {
		stats.UniqueSymbols++
		return true
	})

	// Count unique users
	s.byUser.Range(func(key, value interface{}) bool {
		stats.UniqueUsers++
		return true
	})

	return stats
}
