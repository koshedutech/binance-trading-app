// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.20: Actor Tracking System
package decision

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Actor represents who initiated a trade
type Actor string

const (
	ActorUser           Actor = "USER"            // Manual trade via UI
	ActorGinieAuto      Actor = "GINIE_AUTO"      // Automatic trade by autopilot
	ActorGinieSuggested Actor = "GINIE_SUGGESTED" // User accepted Ginie suggestion
	ActorSystem         Actor = "SYSTEM"          // System-initiated (e.g., stop loss, liquidation)
	ActorUnknown        Actor = "UNKNOWN"         // Unknown actor
)

// String returns the string representation of the Actor
func (a Actor) String() string {
	return string(a)
}

// DisplayName returns a human-readable display name for the Actor
func (a Actor) DisplayName() string {
	switch a {
	case ActorUser:
		return "Manual"
	case ActorGinieAuto:
		return "Auto"
	case ActorGinieSuggested:
		return "Suggested"
	case ActorSystem:
		return "System"
	case ActorUnknown:
		return "Unknown"
	default:
		return "Unknown"
	}
}

// Badge returns an emoji badge for UI display
func (a Actor) Badge() string {
	switch a {
	case ActorUser:
		return "U" // User icon representation
	case ActorGinieAuto:
		return "A" // Auto/Robot representation
	case ActorGinieSuggested:
		return "S" // Suggestion/Lightbulb representation
	case ActorSystem:
		return "X" // System/Gear representation
	case ActorUnknown:
		return "?" // Question mark representation
	default:
		return "?"
	}
}

// ParseActor converts a string to an Actor type
func ParseActor(s string) Actor {
	switch s {
	case "USER":
		return ActorUser
	case "GINIE_AUTO":
		return ActorGinieAuto
	case "GINIE_SUGGESTED":
		return ActorGinieSuggested
	case "SYSTEM":
		return ActorSystem
	case "UNKNOWN":
		return ActorUnknown
	default:
		return ActorUnknown
	}
}

// IsValidActor checks if a string is a valid actor type
func IsValidActor(s string) bool {
	switch s {
	case "USER", "GINIE_AUTO", "GINIE_SUGGESTED", "SYSTEM", "UNKNOWN":
		return true
	default:
		return false
	}
}

// AllActors returns all valid actor types
func AllActors() []Actor {
	return []Actor{
		ActorUser,
		ActorGinieAuto,
		ActorGinieSuggested,
		ActorSystem,
		ActorUnknown,
	}
}

// ActorInfo contains metadata about a trade actor
type ActorInfo struct {
	Actor       Actor     `json:"actor"`
	Description string    `json:"description"`
	InitiatedAt time.Time `json:"initiated_at"`

	// Optional context
	SourceIP     string `json:"source_ip,omitempty"`     // For USER trades
	AutoRule     string `json:"auto_rule,omitempty"`     // For GINIE_AUTO (which rule triggered)
	SuggestionID string `json:"suggestion_id,omitempty"` // For GINIE_SUGGESTED
}

// NewActorInfo creates a new ActorInfo with the given actor
func NewActorInfo(actor Actor, description string) *ActorInfo {
	return &ActorInfo{
		Actor:       actor,
		Description: description,
		InitiatedAt: time.Now(),
	}
}

// NewUserActorInfo creates actor info for a manual user trade
func NewUserActorInfo(sourceIP string) *ActorInfo {
	return &ActorInfo{
		Actor:       ActorUser,
		Description: "Manual trade via UI",
		InitiatedAt: time.Now(),
		SourceIP:    sourceIP,
	}
}

// NewAutoActorInfo creates actor info for an automatic trade
func NewAutoActorInfo(autoRule string) *ActorInfo {
	return &ActorInfo{
		Actor:       ActorGinieAuto,
		Description: "Automatic trade by Ginie autopilot",
		InitiatedAt: time.Now(),
		AutoRule:    autoRule,
	}
}

// NewSuggestedActorInfo creates actor info for a user-accepted suggestion
func NewSuggestedActorInfo(suggestionID string) *ActorInfo {
	return &ActorInfo{
		Actor:        ActorGinieSuggested,
		Description:  "User accepted Ginie suggestion",
		InitiatedAt:  time.Now(),
		SuggestionID: suggestionID,
	}
}

// NewSystemActorInfo creates actor info for a system-initiated trade
func NewSystemActorInfo(description string) *ActorInfo {
	return &ActorInfo{
		Actor:       ActorSystem,
		Description: description,
		InitiatedAt: time.Now(),
	}
}

// Copy creates a deep copy of ActorInfo
func (a *ActorInfo) Copy() *ActorInfo {
	if a == nil {
		return nil
	}
	return &ActorInfo{
		Actor:        a.Actor,
		Description:  a.Description,
		InitiatedAt:  a.InitiatedAt,
		SourceIP:     a.SourceIP,
		AutoRule:     a.AutoRule,
		SuggestionID: a.SuggestionID,
	}
}

// ActorMetrics tracks performance by actor type
type ActorMetrics struct {
	Actor         Actor     `json:"actor"`
	TotalTrades   int64     `json:"total_trades"`
	WinningTrades int64     `json:"winning_trades"`
	LosingTrades  int64     `json:"losing_trades"`
	WinRate       float64   `json:"win_rate"`
	TotalPnL      float64   `json:"total_pnl"`
	AvgPnL        float64   `json:"avg_pnl"`
	BestTrade     float64   `json:"best_trade"`
	WorstTrade    float64   `json:"worst_trade"`
	LastTradeAt   time.Time `json:"last_trade_at"`
}

// NewActorMetrics creates a new ActorMetrics for the given actor
func NewActorMetrics(actor Actor) *ActorMetrics {
	return &ActorMetrics{
		Actor: actor,
	}
}

// Update updates metrics with a new trade result
func (m *ActorMetrics) Update(pnl float64, won bool, tradeTime time.Time) {
	m.TotalTrades++
	m.TotalPnL += pnl

	if won {
		m.WinningTrades++
	} else {
		m.LosingTrades++
	}

	// Update win rate
	if m.TotalTrades > 0 {
		m.WinRate = float64(m.WinningTrades) / float64(m.TotalTrades) * 100
		m.AvgPnL = m.TotalPnL / float64(m.TotalTrades)
	}

	// Update best/worst trade
	if pnl > m.BestTrade || m.TotalTrades == 1 {
		m.BestTrade = pnl
	}
	if pnl < m.WorstTrade || m.TotalTrades == 1 {
		m.WorstTrade = pnl
	}

	// Update last trade time
	if tradeTime.After(m.LastTradeAt) {
		m.LastTradeAt = tradeTime
	}
}

// Copy creates a deep copy of ActorMetrics
func (m *ActorMetrics) Copy() *ActorMetrics {
	if m == nil {
		return nil
	}
	return &ActorMetrics{
		Actor:         m.Actor,
		TotalTrades:   m.TotalTrades,
		WinningTrades: m.WinningTrades,
		LosingTrades:  m.LosingTrades,
		WinRate:       m.WinRate,
		TotalPnL:      m.TotalPnL,
		AvgPnL:        m.AvgPnL,
		BestTrade:     m.BestTrade,
		WorstTrade:    m.WorstTrade,
		LastTradeAt:   m.LastTradeAt,
	}
}

// TradeRecord represents a completed trade with actor info
type TradeRecord struct {
	TradeID    string     `json:"trade_id"`
	Symbol     string     `json:"symbol"`
	Actor      Actor      `json:"actor"`
	ActorInfo  *ActorInfo `json:"actor_info"`
	Side       string     `json:"side"` // "LONG" or "SHORT"
	EntryPrice float64    `json:"entry_price"`
	ExitPrice  float64    `json:"exit_price"`
	Quantity   float64    `json:"quantity"`
	PnL        float64    `json:"pnl"`
	PnLPercent float64    `json:"pnl_percent"`
	Won        bool       `json:"won"`
	EnteredAt  time.Time  `json:"entered_at"`
	ExitedAt   time.Time  `json:"exited_at"`
	DecisionID string     `json:"decision_id,omitempty"` // Link to PositionDecision
}

// Copy creates a deep copy of TradeRecord
func (t *TradeRecord) Copy() *TradeRecord {
	if t == nil {
		return nil
	}
	return &TradeRecord{
		TradeID:    t.TradeID,
		Symbol:     t.Symbol,
		Actor:      t.Actor,
		ActorInfo:  t.ActorInfo.Copy(),
		Side:       t.Side,
		EntryPrice: t.EntryPrice,
		ExitPrice:  t.ExitPrice,
		Quantity:   t.Quantity,
		PnL:        t.PnL,
		PnLPercent: t.PnLPercent,
		Won:        t.Won,
		EnteredAt:  t.EnteredAt,
		ExitedAt:   t.ExitedAt,
		DecisionID: t.DecisionID,
	}
}

// ActorComparison provides side-by-side actor analytics
type ActorComparison struct {
	UserID      int                     `json:"user_id"`
	Period      string                  `json:"period"`       // "day", "week", "month", "all"
	TotalTrades int64                   `json:"total_trades"`
	Actors      map[Actor]*ActorMetrics `json:"actors"`
	BestActor   Actor                   `json:"best_actor"`  // Highest win rate
	MostActive  Actor                   `json:"most_active"` // Most trades
	Analysis    string                  `json:"analysis"`    // Human-readable summary
}

// NewActorComparison creates a new ActorComparison
func NewActorComparison(userID int, period string) *ActorComparison {
	return &ActorComparison{
		UserID:  userID,
		Period:  period,
		Actors:  make(map[Actor]*ActorMetrics),
	}
}

// ActorTrackingService manages actor-based trade tracking
type ActorTrackingService struct {
	mu       sync.RWMutex
	metrics  map[int]map[Actor]*ActorMetrics // userID -> actor -> metrics
	trades   map[int][]TradeRecord           // userID -> trades with actor info
	tradeIDs map[int]map[string]bool         // userID -> tradeID -> exists (for deduplication)

	// Configuration
	maxTradesPerUser int
}

// DefaultMaxTradesPerUser is the default maximum trades stored per user
const DefaultMaxTradesPerUser = 1000

// NewActorTrackingService creates a new ActorTrackingService
func NewActorTrackingService() *ActorTrackingService {
	return &ActorTrackingService{
		metrics:          make(map[int]map[Actor]*ActorMetrics),
		trades:           make(map[int][]TradeRecord),
		tradeIDs:         make(map[int]map[string]bool),
		maxTradesPerUser: DefaultMaxTradesPerUser,
	}
}

// NewActorTrackingServiceWithMaxTrades creates a service with custom max trades
func NewActorTrackingServiceWithMaxTrades(maxTrades int) *ActorTrackingService {
	if maxTrades <= 0 {
		maxTrades = DefaultMaxTradesPerUser
	}
	return &ActorTrackingService{
		metrics:          make(map[int]map[Actor]*ActorMetrics),
		trades:           make(map[int][]TradeRecord),
		tradeIDs:         make(map[int]map[string]bool),
		maxTradesPerUser: maxTrades,
	}
}

// ErrDuplicateTradeID is returned when attempting to record a trade with an ID that already exists
var ErrDuplicateTradeID = fmt.Errorf("duplicate trade ID")

// RecordTrade records a completed trade with actor info
func (s *ActorTrackingService) RecordTrade(userID int, record TradeRecord) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user ID: %d", userID)
	}
	if record.TradeID == "" {
		return fmt.Errorf("trade ID cannot be empty")
	}
	if record.Symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Initialize user maps if needed
	if s.metrics[userID] == nil {
		s.metrics[userID] = make(map[Actor]*ActorMetrics)
	}
	if s.trades[userID] == nil {
		s.trades[userID] = make([]TradeRecord, 0)
	}
	if s.tradeIDs[userID] == nil {
		s.tradeIDs[userID] = make(map[string]bool)
	}

	// Check for duplicate trade ID
	if s.tradeIDs[userID][record.TradeID] {
		return fmt.Errorf("%w: %s", ErrDuplicateTradeID, record.TradeID)
	}

	// Initialize actor metrics if needed
	if s.metrics[userID][record.Actor] == nil {
		s.metrics[userID][record.Actor] = NewActorMetrics(record.Actor)
	}

	// Update metrics
	s.metrics[userID][record.Actor].Update(record.PnL, record.Won, record.ExitedAt)

	// Store trade record (copy to prevent external mutation)
	s.trades[userID] = append(s.trades[userID], *record.Copy())

	// Mark trade ID as recorded
	s.tradeIDs[userID][record.TradeID] = true

	// Trim trades if over limit (keep tradeIDs in sync)
	if len(s.trades[userID]) > s.maxTradesPerUser {
		// Remove oldest trade IDs from the map
		trimCount := len(s.trades[userID]) - s.maxTradesPerUser
		for i := 0; i < trimCount; i++ {
			delete(s.tradeIDs[userID], s.trades[userID][i].TradeID)
		}
		s.trades[userID] = s.trades[userID][trimCount:]
	}

	return nil
}

// GetMetricsByActor returns metrics for a specific actor
func (s *ActorTrackingService) GetMetricsByActor(userID int, actor Actor) (*ActorMetrics, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	userMetrics, ok := s.metrics[userID]
	if !ok {
		return NewActorMetrics(actor), nil
	}

	metrics, ok := userMetrics[actor]
	if !ok {
		return NewActorMetrics(actor), nil
	}

	return metrics.Copy(), nil
}

// GetAllMetrics returns metrics for all actors for a user
func (s *ActorTrackingService) GetAllMetrics(userID int) (map[Actor]*ActorMetrics, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[Actor]*ActorMetrics)

	userMetrics, ok := s.metrics[userID]
	if !ok {
		return result, nil
	}

	for actor, metrics := range userMetrics {
		result[actor] = metrics.Copy()
	}

	return result, nil
}

// GetTradesByActor returns trades filtered by actor
func (s *ActorTrackingService) GetTradesByActor(userID int, actor Actor, limit int) ([]TradeRecord, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}
	if limit <= 0 {
		limit = 100
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	userTrades, ok := s.trades[userID]
	if !ok {
		return []TradeRecord{}, nil
	}

	// Filter by actor (iterate from end for most recent first)
	var filtered []TradeRecord
	for i := len(userTrades) - 1; i >= 0 && len(filtered) < limit; i-- {
		if userTrades[i].Actor == actor {
			filtered = append(filtered, *userTrades[i].Copy())
		}
	}

	return filtered, nil
}

// GetTradeHistory returns all trades for a user
func (s *ActorTrackingService) GetTradeHistory(userID int, limit int) ([]TradeRecord, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}
	if limit <= 0 {
		limit = 100
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	userTrades, ok := s.trades[userID]
	if !ok {
		return []TradeRecord{}, nil
	}

	// Get most recent trades
	start := len(userTrades) - limit
	if start < 0 {
		start = 0
	}

	result := make([]TradeRecord, 0, limit)
	for i := len(userTrades) - 1; i >= start; i-- {
		result = append(result, *userTrades[i].Copy())
	}

	return result, nil
}

// GetActorComparison returns side-by-side comparison of actors
func (s *ActorTrackingService) GetActorComparison(userID int) (*ActorComparison, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	comparison := NewActorComparison(userID, "all")

	userMetrics, ok := s.metrics[userID]
	if !ok {
		comparison.Analysis = "No trades recorded yet"
		return comparison, nil
	}

	var bestWinRate float64
	var mostTrades int64

	for actor, metrics := range userMetrics {
		comparison.Actors[actor] = metrics.Copy()
		comparison.TotalTrades += metrics.TotalTrades

		// Track best actor by win rate (with minimum trades threshold)
		if metrics.TotalTrades >= 5 && metrics.WinRate > bestWinRate {
			bestWinRate = metrics.WinRate
			comparison.BestActor = actor
		}

		// Track most active actor
		if metrics.TotalTrades > mostTrades {
			mostTrades = metrics.TotalTrades
			comparison.MostActive = actor
		}
	}

	// Generate analysis
	comparison.Analysis = s.generateAnalysis(comparison)

	return comparison, nil
}

// generateAnalysis creates a human-readable summary of actor comparison
func (s *ActorTrackingService) generateAnalysis(c *ActorComparison) string {
	if c.TotalTrades == 0 {
		return "No trades recorded yet"
	}

	if c.TotalTrades < 10 {
		return fmt.Sprintf("Early stage: %d total trades recorded. More data needed for meaningful comparison.", c.TotalTrades)
	}

	if c.BestActor == "" || c.MostActive == "" {
		return fmt.Sprintf("%d trades recorded across different actors.", c.TotalTrades)
	}

	bestMetrics := c.Actors[c.BestActor]
	mostActiveMetrics := c.Actors[c.MostActive]

	if c.BestActor == c.MostActive {
		return fmt.Sprintf("%s trades perform best with %.1f%% win rate across %d trades.",
			c.BestActor.DisplayName(), bestMetrics.WinRate, bestMetrics.TotalTrades)
	}

	return fmt.Sprintf("%s trades have highest win rate (%.1f%%), while %s is most active (%d trades).",
		c.BestActor.DisplayName(), bestMetrics.WinRate,
		c.MostActive.DisplayName(), mostActiveMetrics.TotalTrades)
}

// ValidPeriods defines the allowed period values for filtering
var ValidPeriods = map[string]bool{
	"day":   true,
	"week":  true,
	"month": true,
	"all":   true,
}

// IsValidPeriod checks if a period string is valid
func IsValidPeriod(period string) bool {
	return ValidPeriods[period]
}

// GetActorComparisonForPeriod returns comparison filtered by time period
// Valid periods are: "day", "week", "month", "all"
func (s *ActorTrackingService) GetActorComparisonForPeriod(userID int, period string) (*ActorComparison, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}
	if !IsValidPeriod(period) {
		return nil, fmt.Errorf("invalid period: %s (valid values: day, week, month, all)", period)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	comparison := NewActorComparison(userID, period)

	userTrades, ok := s.trades[userID]
	if !ok {
		comparison.Analysis = "No trades recorded yet"
		return comparison, nil
	}

	// Determine cutoff time
	var cutoff time.Time
	now := time.Now()
	switch period {
	case "day":
		cutoff = now.AddDate(0, 0, -1)
	case "week":
		cutoff = now.AddDate(0, 0, -7)
	case "month":
		cutoff = now.AddDate(0, -1, 0)
	default:
		cutoff = time.Time{} // No cutoff for "all"
	}

	// Calculate metrics from filtered trades
	periodMetrics := make(map[Actor]*ActorMetrics)

	for _, trade := range userTrades {
		if !cutoff.IsZero() && trade.ExitedAt.Before(cutoff) {
			continue
		}

		if periodMetrics[trade.Actor] == nil {
			periodMetrics[trade.Actor] = NewActorMetrics(trade.Actor)
		}
		periodMetrics[trade.Actor].Update(trade.PnL, trade.Won, trade.ExitedAt)
		comparison.TotalTrades++
	}

	comparison.Actors = periodMetrics

	// Determine best and most active
	var bestWinRate float64
	var mostTrades int64

	for actor, metrics := range periodMetrics {
		if metrics.TotalTrades >= 3 && metrics.WinRate > bestWinRate {
			bestWinRate = metrics.WinRate
			comparison.BestActor = actor
		}
		if metrics.TotalTrades > mostTrades {
			mostTrades = metrics.TotalTrades
			comparison.MostActive = actor
		}
	}

	comparison.Analysis = s.generateAnalysis(comparison)
	return comparison, nil
}

// GetTradesByActorForPeriod returns trades filtered by actor and time period
// Valid periods are: "day", "week", "month", "all"
func (s *ActorTrackingService) GetTradesByActorForPeriod(userID int, actor Actor, period string, limit int) ([]TradeRecord, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}
	if !IsValidPeriod(period) {
		return nil, fmt.Errorf("invalid period: %s (valid values: day, week, month, all)", period)
	}
	if limit <= 0 {
		limit = 100
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	userTrades, ok := s.trades[userID]
	if !ok {
		return []TradeRecord{}, nil
	}

	// Determine cutoff time
	var cutoff time.Time
	now := time.Now()
	switch period {
	case "day":
		cutoff = now.AddDate(0, 0, -1)
	case "week":
		cutoff = now.AddDate(0, 0, -7)
	case "month":
		cutoff = now.AddDate(0, -1, 0)
	default:
		cutoff = time.Time{}
	}

	// Filter by actor and period
	var filtered []TradeRecord
	for i := len(userTrades) - 1; i >= 0 && len(filtered) < limit; i-- {
		trade := userTrades[i]
		if trade.Actor != actor {
			continue
		}
		if !cutoff.IsZero() && trade.ExitedAt.Before(cutoff) {
			continue
		}
		filtered = append(filtered, *trade.Copy())
	}

	return filtered, nil
}

// ClearUserData removes all data for a user
func (s *ActorTrackingService) ClearUserData(userID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.metrics, userID)
	delete(s.trades, userID)
	delete(s.tradeIDs, userID)
}

// GetStats returns statistics about the tracking service
type ActorTrackingStats struct {
	TotalUsers       int            `json:"total_users"`
	TotalTrades      int64          `json:"total_trades"`
	TradesByActor    map[Actor]int64 `json:"trades_by_actor"`
	TopPerformingActor Actor        `json:"top_performing_actor"`
}

// GetStats returns current statistics
func (s *ActorTrackingService) GetStats() ActorTrackingStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ActorTrackingStats{
		TradesByActor: make(map[Actor]int64),
	}

	stats.TotalUsers = len(s.metrics)

	var bestWinRate float64
	var bestTrades int64

	for _, userMetrics := range s.metrics {
		for actor, metrics := range userMetrics {
			stats.TotalTrades += metrics.TotalTrades
			stats.TradesByActor[actor] += metrics.TotalTrades

			// Track best performer (min 10 trades across all users for this actor)
			if stats.TradesByActor[actor] >= 10 && metrics.WinRate > bestWinRate {
				bestWinRate = metrics.WinRate
				stats.TopPerformingActor = actor
				bestTrades = metrics.TotalTrades
			} else if metrics.WinRate == bestWinRate && metrics.TotalTrades > bestTrades {
				stats.TopPerformingActor = actor
				bestTrades = metrics.TotalTrades
			}
		}
	}

	return stats
}

// ActorResponse for API responses
type ActorResponse struct {
	Actor       string `json:"actor"`
	DisplayName string `json:"display_name"`
	Badge       string `json:"badge"`
	Description string `json:"description,omitempty"`
}

// ToResponse converts Actor to ActorResponse
func (a Actor) ToResponse() ActorResponse {
	return ActorResponse{
		Actor:       string(a),
		DisplayName: a.DisplayName(),
		Badge:       a.Badge(),
	}
}

// ToResponseWithDescription converts Actor to ActorResponse with description
func (a Actor) ToResponseWithDescription(description string) ActorResponse {
	return ActorResponse{
		Actor:       string(a),
		DisplayName: a.DisplayName(),
		Badge:       a.Badge(),
		Description: description,
	}
}

// ActorInfoResponse is the API response format for ActorInfo
type ActorInfoResponse struct {
	Actor       ActorResponse `json:"actor"`
	InitiatedAt time.Time     `json:"initiated_at"`
	SourceIP    string        `json:"source_ip,omitempty"`
	AutoRule    string        `json:"auto_rule,omitempty"`
	SuggestionID string       `json:"suggestion_id,omitempty"`
}

// ToResponse converts ActorInfo to ActorInfoResponse
func (a *ActorInfo) ToResponse() *ActorInfoResponse {
	if a == nil {
		return nil
	}
	return &ActorInfoResponse{
		Actor:        a.Actor.ToResponseWithDescription(a.Description),
		InitiatedAt:  a.InitiatedAt,
		SourceIP:     a.SourceIP,
		AutoRule:     a.AutoRule,
		SuggestionID: a.SuggestionID,
	}
}

// GetLeaderboard returns actors sorted by win rate
func (s *ActorTrackingService) GetLeaderboard(userID int) ([]*ActorMetrics, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user ID: %d", userID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	userMetrics, ok := s.metrics[userID]
	if !ok {
		return []*ActorMetrics{}, nil
	}

	// Collect all metrics - initialize to empty slice (not nil) for consistent return type
	leaderboard := make([]*ActorMetrics, 0, len(userMetrics))
	for _, metrics := range userMetrics {
		if metrics.TotalTrades > 0 {
			leaderboard = append(leaderboard, metrics.Copy())
		}
	}

	// Sort by win rate (descending), then by total trades (descending)
	sort.Slice(leaderboard, func(i, j int) bool {
		if leaderboard[i].WinRate != leaderboard[j].WinRate {
			return leaderboard[i].WinRate > leaderboard[j].WinRate
		}
		return leaderboard[i].TotalTrades > leaderboard[j].TotalTrades
	})

	return leaderboard, nil
}
