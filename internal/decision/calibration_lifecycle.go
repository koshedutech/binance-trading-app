// Package decision provides calibration data lifecycle management.
// Epic 11: Position Decision Engine - Story 11.26: Calibration Data Lifecycle
package decision

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ConfidenceLevel represents how reliable the calibration data is
type ConfidenceLevel string

const (
	// ConfidenceInsufficient means not enough trades to be statistically significant
	ConfidenceInsufficient ConfidenceLevel = "insufficient"
	// ConfidenceLow means some trades but not fully reliable (10-29 trades)
	ConfidenceLow ConfidenceLevel = "low"
	// ConfidenceMedium means moderately reliable (30-49 trades)
	ConfidenceMedium ConfidenceLevel = "medium"
	// ConfidenceHigh means highly reliable (50+ trades)
	ConfidenceHigh ConfidenceLevel = "high"

	// MinTradesForLowConfidence is minimum trades for low confidence
	MinTradesForLowConfidence = 10
	// MinTradesForMediumConfidence is minimum trades for medium confidence
	MinTradesForMediumConfidence = 30
	// MinTradesForHighConfidence is minimum trades for high confidence
	MinTradesForHighConfidence = 50
)

// validStrategies lists all valid strategy names for validation.
var validStrategies = map[string]bool{
	"trend_following": true,
	"mean_reversion":  true,
	"breakout":        true,
	"range_trading":   true,
}

// isValidStrategy checks if a strategy name is valid.
func isValidStrategy(strategy string) bool {
	return validStrategies[strategy]
}

// CalibrationConfidence holds confidence information for a calibration.
type CalibrationConfidence struct {
	Level           ConfidenceLevel `json:"level"`
	TotalTrades     int64           `json:"total_trades"`
	TradesNeeded    int64           `json:"trades_needed"`     // Trades needed for next level
	NextLevel       ConfidenceLevel `json:"next_level"`        // Next confidence level
	IsReliable      bool            `json:"is_reliable"`       // True if at least medium confidence
	ReliabilityPct  float64         `json:"reliability_pct"`   // 0-100% progress toward high confidence
	BucketBreakdown map[string]int64 `json:"bucket_breakdown"` // Trades per bucket
}

// TradeCloseEvent represents a trade close that should update calibration.
type TradeCloseEvent struct {
	UserID        int       `json:"user_id"`
	Strategy      string    `json:"strategy"`
	IndicatorHash string    `json:"indicator_hash"`
	Indicators    []string  `json:"indicators"`
	EntryScore    int       `json:"entry_score"`    // Score at time of entry (0-100)
	IsWin         bool      `json:"is_win"`         // True if profitable trade
	PnLPercent    float64   `json:"pnl_percent"`    // Realized P&L percentage
	Symbol        string    `json:"symbol"`
	CloseTime     time.Time `json:"close_time"`
}

// IndicatorChangeEvent represents when a user changes their indicator selection.
type IndicatorChangeEvent struct {
	UserID        int      `json:"user_id"`
	Strategy      string   `json:"strategy"`
	OldIndicators []string `json:"old_indicators"`
	NewIndicators []string `json:"new_indicators"`
	OldHash       string   `json:"old_hash"`
	NewHash       string   `json:"new_hash"`
}

// CalibrationLifecycleService manages the calibration data lifecycle.
// Story 11.26: Handles trade close updates, indicator changes, and manual resets.
type CalibrationLifecycleService struct {
	calibrationSvc *CalibrationService
	mu             sync.RWMutex

	// Callbacks for integration with other services
	onCalibrationUpdate func(userID int, strategy, hash string)
	onIndicatorChange   func(userID int, strategy, oldHash, newHash string)
}

// NewCalibrationLifecycleService creates a new lifecycle service.
func NewCalibrationLifecycleService(calibrationSvc *CalibrationService) *CalibrationLifecycleService {
	if calibrationSvc == nil {
		log.Printf("[CALIBRATION-LIFECYCLE] Warning: CalibrationService is nil")
		return nil
	}
	return &CalibrationLifecycleService{
		calibrationSvc: calibrationSvc,
	}
}

// SetCalibrationUpdateCallback sets a callback for when calibration is updated.
func (cls *CalibrationLifecycleService) SetCalibrationUpdateCallback(cb func(userID int, strategy, hash string)) {
	cls.mu.Lock()
	defer cls.mu.Unlock()
	cls.onCalibrationUpdate = cb
}

// SetIndicatorChangeCallback sets a callback for when indicators change.
func (cls *CalibrationLifecycleService) SetIndicatorChangeCallback(cb func(userID int, strategy, oldHash, newHash string)) {
	cls.mu.Lock()
	defer cls.mu.Unlock()
	cls.onIndicatorChange = cb
}

// ============================================================================
// Trade Close Handling
// ============================================================================

// UpdateOnTradeClose updates calibration data when a trade closes.
// This is called after a position is closed to record the outcome.
//
// Parameters:
//   - userID: The user ID
//   - strategy: The trading strategy used (e.g., "trend_following")
//   - indicatorHash: Hash of the indicators used (from CalculateIndicatorHash)
//   - entryScore: The decision score at time of entry (0-100)
//   - isWin: Whether the trade was profitable
//   - pnlPercent: The realized P&L percentage
func (cls *CalibrationLifecycleService) UpdateOnTradeClose(
	ctx context.Context,
	userID int,
	strategy string,
	indicatorHash string,
	entryScore int,
	isWin bool,
	pnlPercent float64,
) error {
	if cls == nil || cls.calibrationSvc == nil {
		return fmt.Errorf("calibration lifecycle service not initialized")
	}

	// Validate inputs
	if userID <= 0 {
		return fmt.Errorf("invalid userID: %d", userID)
	}
	if strategy == "" {
		return fmt.Errorf("strategy is required")
	}
	if !isValidStrategy(strategy) {
		return fmt.Errorf("invalid strategy: %s (valid: trend_following, mean_reversion, breakout, range_trading)", strategy)
	}
	if indicatorHash == "" {
		indicatorHash = "00000000" // Default for no specific indicators
	}

	// Clamp entry score to valid range
	if entryScore < 0 {
		entryScore = 0
	}
	if entryScore > 100 {
		entryScore = 100
	}

	// Record the outcome through CalibrationService
	err := cls.calibrationSvc.RecordTradeOutcome(ctx, userID, strategy, indicatorHash, entryScore, isWin, pnlPercent)
	if err != nil {
		log.Printf("[CALIBRATION-LIFECYCLE] Failed to update on trade close for user %d: %v", userID, err)
		return fmt.Errorf("failed to record trade outcome: %w", err)
	}

	log.Printf("[CALIBRATION-LIFECYCLE] Recorded trade close: user=%d, strategy=%s, hash=%s, score=%d, win=%v, pnl=%.2f%%",
		userID, strategy, indicatorHash, entryScore, isWin, pnlPercent)

	// Invoke callback if set (with panic recovery)
	cls.mu.RLock()
	cb := cls.onCalibrationUpdate
	cls.mu.RUnlock()
	if cb != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[CALIBRATION-LIFECYCLE] Panic in calibration update callback: %v", r)
				}
			}()
			cb(userID, strategy, indicatorHash)
		}()
	}

	return nil
}

// ProcessTradeCloseEvent processes a complete trade close event.
// This is a convenience method that accepts a TradeCloseEvent struct.
func (cls *CalibrationLifecycleService) ProcessTradeCloseEvent(ctx context.Context, event *TradeCloseEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}

	return cls.UpdateOnTradeClose(
		ctx,
		event.UserID,
		event.Strategy,
		event.IndicatorHash,
		event.EntryScore,
		event.IsWin,
		event.PnLPercent,
	)
}

// ============================================================================
// Indicator Change Handling
// ============================================================================

// OnIndicatorChange handles when a user changes their indicator selection.
// This initializes new calibration or retrieves existing calibration for the new indicators.
//
// Parameters:
//   - userID: The user ID
//   - strategy: The trading strategy
//   - oldHash: Hash of the old indicator configuration
//   - newHash: Hash of the new indicator configuration
//   - newIndicators: List of new indicator names
func (cls *CalibrationLifecycleService) OnIndicatorChange(
	ctx context.Context,
	userID int,
	strategy string,
	oldHash string,
	newHash string,
	newIndicators []string,
) error {
	if cls == nil || cls.calibrationSvc == nil {
		return fmt.Errorf("calibration lifecycle service not initialized")
	}

	// Validate inputs
	if userID <= 0 {
		return fmt.Errorf("invalid userID: %d", userID)
	}
	if strategy == "" {
		return fmt.Errorf("strategy is required")
	}
	if !isValidStrategy(strategy) {
		return fmt.Errorf("invalid strategy: %s (valid: trend_following, mean_reversion, breakout, range_trading)", strategy)
	}

	// If hashes are the same, no change needed
	if oldHash == newHash {
		log.Printf("[CALIBRATION-LIFECYCLE] No indicator change detected (same hash: %s)", oldHash)
		return nil
	}

	// Check if calibration exists for new hash
	existingCalibration, err := cls.calibrationSvc.GetCalibrationData(ctx, userID, strategy, newHash)
	if err != nil {
		log.Printf("[CALIBRATION-LIFECYCLE] Error checking existing calibration: %v", err)
	}

	if existingCalibration != nil {
		// Calibration already exists for new indicators - just log and return
		log.Printf("[CALIBRATION-LIFECYCLE] Found existing calibration for new indicators: user=%d, strategy=%s, hash=%s (total trades: %d)",
			userID, strategy, newHash, existingCalibration.TotalTrades)
		return nil
	}

	// Archive old calibration and create new one
	err = cls.calibrationSvc.ArchiveAndReplace(ctx, userID, strategy, oldHash, newHash, newIndicators)
	if err != nil {
		log.Printf("[CALIBRATION-LIFECYCLE] Failed to archive and replace calibration: %v", err)
		return fmt.Errorf("failed to handle indicator change: %w", err)
	}

	log.Printf("[CALIBRATION-LIFECYCLE] Indicator change processed: user=%d, strategy=%s, old_hash=%s, new_hash=%s",
		userID, strategy, oldHash, newHash)

	// Invoke callback if set (with panic recovery)
	cls.mu.RLock()
	cb := cls.onIndicatorChange
	cls.mu.RUnlock()
	if cb != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[CALIBRATION-LIFECYCLE] Panic in indicator change callback: %v", r)
				}
			}()
			cb(userID, strategy, oldHash, newHash)
		}()
	}

	return nil
}

// ProcessIndicatorChangeEvent processes a complete indicator change event.
func (cls *CalibrationLifecycleService) ProcessIndicatorChangeEvent(ctx context.Context, event *IndicatorChangeEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}

	return cls.OnIndicatorChange(
		ctx,
		event.UserID,
		event.Strategy,
		event.OldHash,
		event.NewHash,
		event.NewIndicators,
	)
}

// ============================================================================
// Calibration Reset
// ============================================================================

// ResetCalibration manually resets calibration data.
// The old data is archived to history for analysis.
//
// Parameters:
//   - userID: The user ID
//   - strategy: The trading strategy
//   - indicatorHash: Hash of the indicator configuration to reset
func (cls *CalibrationLifecycleService) ResetCalibration(
	ctx context.Context,
	userID int,
	strategy string,
	indicatorHash string,
) error {
	if cls == nil || cls.calibrationSvc == nil {
		return fmt.Errorf("calibration lifecycle service not initialized")
	}

	// Validate inputs
	if userID <= 0 {
		return fmt.Errorf("invalid userID: %d", userID)
	}
	if strategy == "" {
		return fmt.Errorf("strategy is required")
	}
	if !isValidStrategy(strategy) {
		return fmt.Errorf("invalid strategy: %s (valid: trend_following, mean_reversion, breakout, range_trading)", strategy)
	}
	if indicatorHash == "" {
		indicatorHash = "00000000"
	}

	// Reset through CalibrationService (which archives the old data)
	err := cls.calibrationSvc.ResetCalibration(ctx, userID, strategy, indicatorHash)
	if err != nil {
		log.Printf("[CALIBRATION-LIFECYCLE] Failed to reset calibration for user %d: %v", userID, err)
		return fmt.Errorf("failed to reset calibration: %w", err)
	}

	log.Printf("[CALIBRATION-LIFECYCLE] Calibration reset: user=%d, strategy=%s, hash=%s",
		userID, strategy, indicatorHash)

	return nil
}

// ResetAllCalibrations resets all calibrations for a user/strategy.
// Each calibration is archived individually before reset.
func (cls *CalibrationLifecycleService) ResetAllCalibrations(
	ctx context.Context,
	userID int,
	strategy string,
) error {
	if cls == nil || cls.calibrationSvc == nil {
		return fmt.Errorf("calibration lifecycle service not initialized")
	}

	// Get all calibrations for this user
	userIDStr := fmt.Sprintf("%d", userID)
	if cls.calibrationSvc.dbRepo == nil {
		return fmt.Errorf("database repository not configured")
	}

	records, err := cls.calibrationSvc.dbRepo.GetCalibrationDataByStrategy(ctx, userIDStr, strategy)
	if err != nil {
		return fmt.Errorf("failed to get calibrations: %w", err)
	}

	resetCount := 0
	for _, record := range records {
		if err := cls.ResetCalibration(ctx, userID, strategy, record.IndicatorHash); err != nil {
			log.Printf("[CALIBRATION-LIFECYCLE] Warning: failed to reset calibration %s: %v", record.IndicatorHash, err)
		} else {
			resetCount++
		}
	}

	log.Printf("[CALIBRATION-LIFECYCLE] Reset %d calibrations for user=%d, strategy=%s", resetCount, userID, strategy)
	return nil
}

// ============================================================================
// Calibration Confidence
// ============================================================================

// GetCalibrationConfidence returns the confidence level for a calibration.
// This helps users understand how reliable their calibration data is.
func (cls *CalibrationLifecycleService) GetCalibrationConfidence(
	ctx context.Context,
	userID int,
	strategy string,
	indicatorHash string,
) (*CalibrationConfidence, error) {
	if cls == nil || cls.calibrationSvc == nil {
		return nil, fmt.Errorf("calibration lifecycle service not initialized")
	}

	// Validate inputs
	if userID <= 0 {
		return nil, fmt.Errorf("invalid userID: %d", userID)
	}
	if strategy == "" {
		return nil, fmt.Errorf("strategy is required")
	}
	if indicatorHash == "" {
		indicatorHash = "00000000"
	}

	// Get calibration data
	data, err := cls.calibrationSvc.GetCalibrationData(ctx, userID, strategy, indicatorHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get calibration data: %w", err)
	}

	// Return default confidence for no data
	if data == nil || data.TotalTrades == 0 {
		return &CalibrationConfidence{
			Level:           ConfidenceInsufficient,
			TotalTrades:     0,
			TradesNeeded:    int64(MinTradesForLowConfidence),
			NextLevel:       ConfidenceLow,
			IsReliable:      false,
			ReliabilityPct:  0,
			BucketBreakdown: make(map[string]int64),
		}, nil
	}

	// Calculate confidence level
	confidence := cls.calculateConfidence(data)
	return confidence, nil
}

// calculateConfidence calculates the confidence level from calibration data.
func (cls *CalibrationLifecycleService) calculateConfidence(data *CalibrationData) *CalibrationConfidence {
	totalTrades := data.TotalTrades

	confidence := &CalibrationConfidence{
		TotalTrades:     totalTrades,
		BucketBreakdown: make(map[string]int64),
	}

	// Populate bucket breakdown
	for name, bucket := range data.Buckets {
		confidence.BucketBreakdown[name] = bucket.TotalTrades
	}

	// Determine confidence level and next target
	switch {
	case totalTrades >= int64(MinTradesForHighConfidence):
		confidence.Level = ConfidenceHigh
		confidence.TradesNeeded = 0
		confidence.NextLevel = ConfidenceHigh // Already at max
		confidence.IsReliable = true
		confidence.ReliabilityPct = 100.0

	case totalTrades >= int64(MinTradesForMediumConfidence):
		confidence.Level = ConfidenceMedium
		confidence.TradesNeeded = int64(MinTradesForHighConfidence) - totalTrades
		confidence.NextLevel = ConfidenceHigh
		confidence.IsReliable = true
		// Progress from medium to high
		progress := float64(totalTrades-int64(MinTradesForMediumConfidence)) / float64(MinTradesForHighConfidence-MinTradesForMediumConfidence)
		confidence.ReliabilityPct = 50.0 + (progress * 50.0)

	case totalTrades >= int64(MinTradesForLowConfidence):
		confidence.Level = ConfidenceLow
		confidence.TradesNeeded = int64(MinTradesForMediumConfidence) - totalTrades
		confidence.NextLevel = ConfidenceMedium
		confidence.IsReliable = false
		// Progress from low to medium
		progress := float64(totalTrades-int64(MinTradesForLowConfidence)) / float64(MinTradesForMediumConfidence-MinTradesForLowConfidence)
		confidence.ReliabilityPct = progress * 50.0

	default:
		confidence.Level = ConfidenceInsufficient
		confidence.TradesNeeded = int64(MinTradesForLowConfidence) - totalTrades
		confidence.NextLevel = ConfidenceLow
		confidence.IsReliable = false
		// Progress from 0 to low
		progress := float64(totalTrades) / float64(MinTradesForLowConfidence)
		confidence.ReliabilityPct = progress * 25.0 // Max 25% before low confidence
	}

	return confidence
}

// GetActiveCalibrationConfidence gets confidence for the user's currently active indicators.
// This is a convenience method that calculates the indicator hash automatically.
func (cls *CalibrationLifecycleService) GetActiveCalibrationConfidence(
	ctx context.Context,
	userID int,
	strategy string,
	activeIndicators []string,
) (*CalibrationConfidence, error) {
	if len(activeIndicators) == 0 {
		return cls.GetCalibrationConfidence(ctx, userID, strategy, "00000000")
	}

	hash := CalculateIndicatorHash(activeIndicators)
	return cls.GetCalibrationConfidence(ctx, userID, strategy, hash)
}

// ============================================================================
// History Retrieval
// ============================================================================

// GetCalibrationHistory retrieves archived calibration history for a user/strategy.
// This allows users to view their historical calibration data.
func (cls *CalibrationLifecycleService) GetCalibrationHistory(
	ctx context.Context,
	userID int,
	strategy string,
) ([]*CalibrationHistoryRecord, error) {
	if cls == nil || cls.calibrationSvc == nil || cls.calibrationSvc.dbRepo == nil {
		return nil, fmt.Errorf("calibration lifecycle service not properly initialized")
	}

	userIDStr := fmt.Sprintf("%d", userID)
	return cls.calibrationSvc.dbRepo.GetCalibrationHistory(ctx, userIDStr, strategy)
}

// GetAllActiveCalibrations retrieves all active calibrations for a user.
func (cls *CalibrationLifecycleService) GetAllActiveCalibrations(
	ctx context.Context,
	userID int,
) ([]*CalibrationData, error) {
	if cls == nil || cls.calibrationSvc == nil {
		return nil, fmt.Errorf("calibration lifecycle service not initialized")
	}

	return cls.calibrationSvc.GetAllUserCalibrations(userID), nil
}

// ============================================================================
// Utility Methods
// ============================================================================

// GetIndicatorHash calculates a hash for a set of indicators.
// This is a convenience wrapper around CalculateIndicatorHash.
func (cls *CalibrationLifecycleService) GetIndicatorHash(indicators []string) string {
	return CalculateIndicatorHash(indicators)
}

// SyncFromDatabase syncs calibration data from database to cache for a user.
func (cls *CalibrationLifecycleService) SyncFromDatabase(ctx context.Context, userID int) error {
	if cls == nil || cls.calibrationSvc == nil {
		return fmt.Errorf("calibration lifecycle service not initialized")
	}

	return cls.calibrationSvc.SyncFromDatabase(ctx, userID)
}

// GetStats returns statistics about the calibration service.
func (cls *CalibrationLifecycleService) GetStats() CalibrationStats {
	if cls == nil || cls.calibrationSvc == nil {
		return CalibrationStats{}
	}
	return cls.calibrationSvc.GetStats()
}
