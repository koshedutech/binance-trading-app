// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.18: Blocking Reason Tracker
package decision

import (
	"encoding/json"
	"fmt"
	"time"
)

// BlockingCategory represents the severity category of a blocking reason.
type BlockingCategory string

const (
	// CategoryHardBlock cannot be overridden (e.g., trend divergence, ADX < threshold)
	CategoryHardBlock BlockingCategory = "HARD_BLOCK"
	// CategorySoftBlock can be overridden with confirmation (e.g., score below threshold)
	CategorySoftBlock BlockingCategory = "SOFT_BLOCK"
	// CategoryWarning is informational only (e.g., low volume, wide spread)
	CategoryWarning BlockingCategory = "WARNING"
)

// BlockingReasonCode is a unique identifier for each type of blocking condition.
type BlockingReasonCode string

const (
	// Hard block reasons
	ReasonTrendDivergence      BlockingReasonCode = "TREND_DIVERGENCE"
	ReasonADXTooLow            BlockingReasonCode = "ADX_TOO_LOW"
	ReasonCircuitBreakerActive BlockingReasonCode = "CIRCUIT_BREAKER_ACTIVE"
	ReasonRegimeMismatch       BlockingReasonCode = "REGIME_MISMATCH"
	ReasonTimeframeMisalign    BlockingReasonCode = "TIMEFRAME_MISALIGN"

	// Soft block reasons
	ReasonScoreBelowThreshold BlockingReasonCode = "SCORE_BELOW_THRESHOLD"
	ReasonLowConfidence       BlockingReasonCode = "LOW_CONFIDENCE"
	ReasonWeakMomentum        BlockingReasonCode = "WEAK_MOMENTUM"

	// Warning reasons
	ReasonLowVolume   BlockingReasonCode = "LOW_VOLUME"
	ReasonWideSpread  BlockingReasonCode = "WIDE_SPREAD"
	ReasonHighRSI     BlockingReasonCode = "HIGH_RSI"
	ReasonLowRSI      BlockingReasonCode = "LOW_RSI"
	ReasonLowWinRate  BlockingReasonCode = "LOW_WIN_RATE"
)

// BlockingReason represents a single reason why a signal is blocked.
type BlockingReason struct {
	// Code is the unique identifier for this blocking reason
	Code BlockingReasonCode `json:"code"`
	// Category is the severity level (HARD_BLOCK, SOFT_BLOCK, WARNING)
	Category BlockingCategory `json:"category"`
	// Description is a human-readable explanation
	Description string `json:"description"`
	// Value is the actual value that caused the block
	Value interface{} `json:"value,omitempty"`
	// Threshold is the threshold it failed against
	Threshold interface{} `json:"threshold,omitempty"`
	// Timestamp is when this reason was recorded (Unix milliseconds)
	Timestamp int64 `json:"timestamp"`
	// Overridable indicates if user can override this block
	Overridable bool `json:"overridable"`
}

// NewBlockingReason creates a new BlockingReason with current timestamp.
func NewBlockingReason(code BlockingReasonCode, category BlockingCategory, description string) *BlockingReason {
	return &BlockingReason{
		Code:        code,
		Category:    category,
		Description: description,
		Timestamp:   time.Now().UnixMilli(),
		Overridable: category != CategoryHardBlock,
	}
}

// WithValue sets the actual value that caused the block.
func (br *BlockingReason) WithValue(value interface{}) *BlockingReason {
	br.Value = value
	return br
}

// WithThreshold sets the threshold the value failed against.
func (br *BlockingReason) WithThreshold(threshold interface{}) *BlockingReason {
	br.Threshold = threshold
	return br
}

// String returns a formatted string representation of the blocking reason.
func (br *BlockingReason) String() string {
	if br.Value != nil && br.Threshold != nil {
		return fmt.Sprintf("[%s] %s: %s (value=%v, threshold=%v)",
			br.Category, br.Code, br.Description, br.Value, br.Threshold)
	}
	return fmt.Sprintf("[%s] %s: %s", br.Category, br.Code, br.Description)
}

// ToJSON returns the JSON representation of the blocking reason.
func (br *BlockingReason) ToJSON() string {
	data, _ := json.Marshal(br)
	return string(data)
}

// BlockingConfig contains configurable thresholds for blocking conditions.
type BlockingConfig struct {
	// ADXMinimum is the minimum ADX value to allow trading (default: 15)
	ADXMinimum float64 `json:"adx_minimum"`
	// ScoreMinimum is the minimum final score to allow trading (default: 55)
	ScoreMinimum float64 `json:"score_minimum"`
	// RSIUpperLimit is the RSI value above which a warning is issued (default: 80)
	RSIUpperLimit float64 `json:"rsi_upper_limit"`
	// RSILowerLimit is the RSI value below which a warning is issued (default: 20)
	RSILowerLimit float64 `json:"rsi_lower_limit"`
	// WinRateMinimum is the minimum win rate to avoid warning (default: 40)
	WinRateMinimum float64 `json:"win_rate_minimum"`
	// MaxBlockingReasons is the maximum number of reasons to track per symbol (default: 10)
	MaxBlockingReasons int `json:"max_blocking_reasons"`
	// EnableHardBlocks controls whether hard blocks are enforced (default: true)
	EnableHardBlocks bool `json:"enable_hard_blocks"`
	// EnableSoftBlocks controls whether soft blocks are enforced (default: true)
	EnableSoftBlocks bool `json:"enable_soft_blocks"`
	// EnableWarnings controls whether warnings are tracked (default: true)
	EnableWarnings bool `json:"enable_warnings"`
}

// DefaultBlockingConfig returns a BlockingConfig with sensible defaults.
func DefaultBlockingConfig() *BlockingConfig {
	return &BlockingConfig{
		ADXMinimum:         15.0,
		ScoreMinimum:       55.0,
		RSIUpperLimit:      80.0,
		RSILowerLimit:      20.0,
		WinRateMinimum:     40.0,
		MaxBlockingReasons: 10,
		EnableHardBlocks:   true,
		EnableSoftBlocks:   true,
		EnableWarnings:     true,
	}
}

// Validate ensures the config has valid values, clamping as needed.
func (bc *BlockingConfig) Validate() {
	if bc.ADXMinimum < 0 {
		bc.ADXMinimum = 0
	}
	if bc.ADXMinimum > 100 {
		bc.ADXMinimum = 100
	}

	if bc.ScoreMinimum < 0 {
		bc.ScoreMinimum = 0
	}
	if bc.ScoreMinimum > 100 {
		bc.ScoreMinimum = 100
	}

	if bc.RSIUpperLimit < 50 {
		bc.RSIUpperLimit = 50
	}
	if bc.RSIUpperLimit > 100 {
		bc.RSIUpperLimit = 100
	}

	if bc.RSILowerLimit < 0 {
		bc.RSILowerLimit = 0
	}
	if bc.RSILowerLimit > 50 {
		bc.RSILowerLimit = 50
	}

	if bc.WinRateMinimum < 0 {
		bc.WinRateMinimum = 0
	}
	if bc.WinRateMinimum > 100 {
		bc.WinRateMinimum = 100
	}

	if bc.MaxBlockingReasons <= 0 {
		bc.MaxBlockingReasons = 10
	}
	if bc.MaxBlockingReasons > 100 {
		bc.MaxBlockingReasons = 100
	}
}

// Clone returns a deep copy of the config.
func (bc *BlockingConfig) Clone() *BlockingConfig {
	return &BlockingConfig{
		ADXMinimum:         bc.ADXMinimum,
		ScoreMinimum:       bc.ScoreMinimum,
		RSIUpperLimit:      bc.RSIUpperLimit,
		RSILowerLimit:      bc.RSILowerLimit,
		WinRateMinimum:     bc.WinRateMinimum,
		MaxBlockingReasons: bc.MaxBlockingReasons,
		EnableHardBlocks:   bc.EnableHardBlocks,
		EnableSoftBlocks:   bc.EnableSoftBlocks,
		EnableWarnings:     bc.EnableWarnings,
	}
}

// BlockingSummary provides an aggregate view of blocking reasons.
type BlockingSummary struct {
	// TotalReasons is the total number of blocking reasons
	TotalReasons int `json:"total_reasons"`
	// HardBlockCount is the number of hard blocks
	HardBlockCount int `json:"hard_block_count"`
	// SoftBlockCount is the number of soft blocks
	SoftBlockCount int `json:"soft_block_count"`
	// WarningCount is the number of warnings
	WarningCount int `json:"warning_count"`
	// IsBlocked indicates if any blocking condition prevents trading
	IsBlocked bool `json:"is_blocked"`
	// CanOverride indicates if all blocks can be overridden
	CanOverride bool `json:"can_override"`
	// PrimaryReason is the most severe blocking reason (if any)
	PrimaryReason *BlockingReason `json:"primary_reason,omitempty"`
	// AllReasons contains all blocking reasons
	AllReasons []BlockingReason `json:"all_reasons"`
}

// NewBlockingSummary creates an empty BlockingSummary.
func NewBlockingSummary() *BlockingSummary {
	return &BlockingSummary{
		AllReasons:  []BlockingReason{},
		CanOverride: true,
	}
}

// BlockingStats provides historical statistics about blocking reasons.
type BlockingStats struct {
	// UserID is the user this stats belongs to
	UserID string `json:"user_id"`
	// TotalBlocks is the total number of blocks recorded
	TotalBlocks int `json:"total_blocks"`
	// BlocksByCode maps reason codes to their occurrence count
	BlocksByCode map[BlockingReasonCode]int `json:"blocks_by_code"`
	// BlocksByCategory maps categories to their occurrence count
	BlocksByCategory map[BlockingCategory]int `json:"blocks_by_category"`
	// MostFrequentBlock is the most common blocking reason
	MostFrequentBlock BlockingReasonCode `json:"most_frequent_block"`
	// SymbolsAffected is the number of unique symbols affected
	SymbolsAffected int `json:"symbols_affected"`
	// LastUpdated is when these stats were last calculated
	LastUpdated int64 `json:"last_updated"`
}

// NewBlockingStats creates a new BlockingStats for a user.
func NewBlockingStats(userID string) *BlockingStats {
	return &BlockingStats{
		UserID:           userID,
		BlocksByCode:     make(map[BlockingReasonCode]int),
		BlocksByCategory: make(map[BlockingCategory]int),
		LastUpdated:      time.Now().UnixMilli(),
	}
}

// BlockingHistoryEntry represents a single blocking event in history.
type BlockingHistoryEntry struct {
	// Symbol is the trading pair
	Symbol string `json:"symbol"`
	// Reasons are the blocking reasons at this point in time
	Reasons []BlockingReason `json:"reasons"`
	// Timestamp is when this entry was recorded
	Timestamp int64 `json:"timestamp"`
	// WasOverridden indicates if the user overrode these blocks
	WasOverridden bool `json:"was_overridden"`
}

// NewBlockingHistoryEntry creates a new history entry.
func NewBlockingHistoryEntry(symbol string, reasons []BlockingReason) *BlockingHistoryEntry {
	return &BlockingHistoryEntry{
		Symbol:    symbol,
		Reasons:   reasons,
		Timestamp: time.Now().UnixMilli(),
	}
}

// GapDirection represents the direction of the gap to target
type GapDirection string

const (
	// GapDirectionUp means value needs to increase to reach target
	GapDirectionUp GapDirection = "up"
	// GapDirectionDown means value needs to decrease to reach target
	GapDirectionDown GapDirection = "down"
	// GapDirectionInRange means value is within target range
	GapDirectionInRange GapDirection = "in_range"
)

// BlockingReasonWithGap extends BlockingReason with directional gap information for UI display
// This is used by the Gap Analysis UI (Story 11.40)
type BlockingReasonWithGap struct {
	// Code is the unique identifier for this blocking reason
	Code string `json:"code"`
	// Category is the severity level (HARD_BLOCK, SOFT_BLOCK, WARNING)
	Category string `json:"category"`
	// Description is a human-readable explanation
	Description string `json:"description"`
	// CurrentValue is the actual value that caused the block
	CurrentValue float64 `json:"current_value"`
	// TargetValue is the threshold it needs to reach (single target or range start)
	TargetValue float64 `json:"target_value"`
	// TargetRangeEnd is the end of target range for range-based conditions (e.g., RSI 40-60)
	TargetRangeEnd float64 `json:"target_range_end,omitempty"`
	// Gap is the absolute gap value to target
	Gap float64 `json:"gap"`
	// GapDirection indicates if value needs to go up, down, or is in range
	GapDirection GapDirection `json:"gap_direction"`
	// GapDisplay is a formatted string for UI display (e.g., "↑3.2", "↓8", "✓ In Range")
	GapDisplay string `json:"gap_display"`
	// Overridable indicates if user can override this block
	Overridable bool `json:"overridable"`
	// Timestamp is when this reason was recorded (Unix milliseconds)
	Timestamp int64 `json:"timestamp"`
}

// CalculateGap computes the gap and direction for a blocking reason
// For single targets (e.g., ADX >= 25): gap = target - current
// For range targets (e.g., RSI 40-60): gap = distance to nearest boundary
func CalculateGap(currentValue, targetValue, targetRangeEnd float64) (gap float64, direction GapDirection, display string) {
	// Range target (e.g., RSI 40-60)
	if targetRangeEnd > 0 && targetRangeEnd > targetValue {
		if currentValue < targetValue {
			// Below range - needs to go up
			gap = targetValue - currentValue
			direction = GapDirectionUp
			display = fmt.Sprintf("↑%.1f", gap)
		} else if currentValue > targetRangeEnd {
			// Above range - needs to go down
			gap = currentValue - targetRangeEnd
			direction = GapDirectionDown
			display = fmt.Sprintf("↓%.1f", gap)
		} else {
			// In range
			gap = 0
			direction = GapDirectionInRange
			display = "✓ In Range"
		}
		return
	}

	// Single target (e.g., ADX >= 25 or Score >= 55)
	if currentValue < targetValue {
		gap = targetValue - currentValue
		direction = GapDirectionUp
		display = fmt.Sprintf("↑%.1f", gap)
	} else if currentValue > targetValue {
		gap = currentValue - targetValue
		direction = GapDirectionDown
		display = fmt.Sprintf("↓%.1f", gap)
	} else {
		gap = 0
		direction = GapDirectionInRange
		display = "✓ Met"
	}
	return
}

// NewBlockingReasonWithGap creates a BlockingReasonWithGap from a BlockingReason with gap calculations
func NewBlockingReasonWithGap(br *BlockingReason, targetRangeEnd float64) *BlockingReasonWithGap {
	if br == nil {
		return nil
	}

	result := &BlockingReasonWithGap{
		Code:        string(br.Code),
		Category:    string(br.Category),
		Description: br.Description,
		Overridable: br.Overridable,
		Timestamp:   br.Timestamp,
	}

	// Extract current value
	if br.Value != nil {
		switch v := br.Value.(type) {
		case float64:
			result.CurrentValue = v
		case int:
			result.CurrentValue = float64(v)
		case int64:
			result.CurrentValue = float64(v)
		}
	}

	// Extract target value
	if br.Threshold != nil {
		switch v := br.Threshold.(type) {
		case float64:
			result.TargetValue = v
		case int:
			result.TargetValue = float64(v)
		case int64:
			result.TargetValue = float64(v)
		}
	}

	result.TargetRangeEnd = targetRangeEnd

	// Calculate gap
	result.Gap, result.GapDirection, result.GapDisplay = CalculateGap(result.CurrentValue, result.TargetValue, result.TargetRangeEnd)

	return result
}

// GetBlockingReasonTargetRange returns the target range end for specific blocking codes
// This is used to determine if a blocking condition has a range target vs single target
func GetBlockingReasonTargetRange(code BlockingReasonCode) float64 {
	// RSI-based conditions have range targets
	switch code {
	case ReasonHighRSI:
		// RSI should be <= 70 (target range 0-70)
		return 0 // No range end, it's a max threshold
	case ReasonLowRSI:
		// RSI should be >= 30 (target is 30, no range)
		return 0
	case ReasonWeakMomentum:
		// RSI optimal is 40-60
		return 60 // Target range 40-60
	default:
		return 0 // Single target
	}
}
