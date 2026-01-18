// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.23: Decision Engine Settings Structure
package decision

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// validDurationRegex matches duration strings like "5m", "1h", "4h", "1d", "7d"
var validDurationRegex = regexp.MustCompile(`^[1-9][0-9]*(m|h|d)$`)

// validTimeframes are the allowed timeframe values for primary/secondary timeframes
var validTimeframes = map[string]bool{
	"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
	"1h": true, "2h": true, "4h": true, "6h": true, "8h": true, "12h": true,
	"1d": true, "3d": true, "1w": true, "1M": true,
}

// Redis key patterns for decision settings
const (
	// PrefixDecisionSettings is the Redis key pattern for user decision engine settings
	// Format: decision:settings:{userID}
	PrefixDecisionSettings = "decision:settings:%s"
)

// DecisionSettingsKey generates the Redis key for a user's decision engine settings.
func DecisionSettingsKey(userID string) string {
	return fmt.Sprintf(PrefixDecisionSettings, sanitizeKeyComponent(userID))
}

// StrategyName represents the type of trading strategy.
type StrategyName string

const (
	// StrategyTrendFollowing follows established trends with momentum confirmation.
	StrategyTrendFollowing StrategyName = "trend_following"
	// StrategyMeanReversion trades reversals when price deviates from mean.
	StrategyMeanReversion StrategyName = "mean_reversion"
	// StrategyBreakout trades breakouts from consolidation patterns.
	StrategyBreakout StrategyName = "breakout"
	// StrategyRangeTrading trades between support and resistance levels.
	StrategyRangeTrading StrategyName = "range_trading"
)

// AllStrategies returns all available strategy names.
func AllStrategies() []StrategyName {
	return []StrategyName{
		StrategyTrendFollowing,
		StrategyMeanReversion,
		StrategyBreakout,
		StrategyRangeTrading,
	}
}

// IsValid checks if the strategy name is valid.
func (s StrategyName) IsValid() bool {
	switch s {
	case StrategyTrendFollowing, StrategyMeanReversion, StrategyBreakout, StrategyRangeTrading:
		return true
	default:
		return false
	}
}

// String returns the string representation of the strategy name.
func (s StrategyName) String() string {
	return string(s)
}

// DecisionEngineSettings contains the complete settings for the decision engine.
// Each user has their own instance with per-strategy configuration.
type DecisionEngineSettings struct {
	// Strategies maps strategy names to their complete settings.
	Strategies map[StrategyName]*StrategySettings `json:"strategies"`
	// ActiveStrategy is the currently selected strategy for decision making.
	ActiveStrategy StrategyName `json:"active_strategy"`
	// LastUpdated is when these settings were last modified (Unix milliseconds).
	LastUpdated int64 `json:"last_updated"`
	// Version is the settings schema version for migration support.
	Version int `json:"version"`
}

// StrategySettings contains the complete configuration for a single trading strategy.
// Each strategy has independent scoring, blocking, entry, and exit conditions.
type StrategySettings struct {
	// Name is the strategy identifier.
	Name StrategyName `json:"name"`
	// Enabled indicates if this strategy can be used.
	Enabled bool `json:"enabled"`
	// Description is a human-readable explanation of the strategy.
	Description string `json:"description"`

	// Scoring configuration from Story 11.15.
	Scoring *ScoringConfig `json:"scoring"`
	// Blocking configuration from Story 11.18.
	Blocking *BlockingConfig `json:"blocking"`

	// Entry conditions for opening positions.
	Entry *EntryConditions `json:"entry"`
	// Exit conditions for closing positions.
	Exit *ExitConditions `json:"exit"`

	// PrimaryTimeframe is the main analysis timeframe (e.g., "1h", "4h").
	PrimaryTimeframe string `json:"primary_timeframe"`
	// SecondaryTimeframe is the confirmation timeframe (e.g., "15m", "5m").
	SecondaryTimeframe string `json:"secondary_timeframe"`
}

// EntryConditions defines when a position can be opened.
type EntryConditions struct {
	// MinScore is the minimum final score required to enter (0-100).
	MinScore float64 `json:"min_score"`
	// MinADX is the minimum ADX value for trend strength (0-100).
	MinADX float64 `json:"min_adx"`
	// RequireTrendAlign requires primary and secondary timeframes to agree.
	RequireTrendAlign bool `json:"require_trend_align"`
	// VolumeMultiplier requires volume to be this multiple of average.
	VolumeMultiplier float64 `json:"volume_multiplier"`
	// MinConfidence is the minimum LLM confidence score (0-100).
	MinConfidence float64 `json:"min_confidence"`
	// MaxSpread is the maximum bid-ask spread allowed (percentage).
	MaxSpread float64 `json:"max_spread"`
}

// ExitConditions defines when a position should be closed.
type ExitConditions struct {
	// TakeProfitPercent is the profit target as percentage.
	TakeProfitPercent float64 `json:"take_profit_percent"`
	// StopLossPercent is the maximum loss allowed as percentage.
	StopLossPercent float64 `json:"stop_loss_percent"`
	// TrailingStop enables trailing stop loss.
	TrailingStop bool `json:"trailing_stop"`
	// TrailingPercent is the trailing stop distance as percentage.
	TrailingPercent float64 `json:"trailing_percent"`
	// MaxHoldTime is the maximum position duration (e.g., "4h", "1d", "7d").
	MaxHoldTime string `json:"max_hold_time"`
	// ExitOnTrendReversal closes position when trend direction changes.
	ExitOnTrendReversal bool `json:"exit_on_trend_reversal"`
	// MinHoldTime is the minimum time before allowing exit (e.g., "5m", "1h").
	MinHoldTime string `json:"min_hold_time"`
}

// NewDecisionEngineSettings creates a new DecisionEngineSettings with default values.
func NewDecisionEngineSettings() *DecisionEngineSettings {
	return &DecisionEngineSettings{
		Strategies:     make(map[StrategyName]*StrategySettings),
		ActiveStrategy: StrategyTrendFollowing,
		LastUpdated:    time.Now().UnixMilli(),
		Version:        1,
	}
}

// DefaultDecisionEngineSettings returns a fully configured settings instance with defaults.
func DefaultDecisionEngineSettings() *DecisionEngineSettings {
	settings := NewDecisionEngineSettings()

	// Add default settings for all strategies
	settings.Strategies[StrategyTrendFollowing] = DefaultTrendFollowingSettings()
	settings.Strategies[StrategyMeanReversion] = DefaultMeanReversionSettings()
	settings.Strategies[StrategyBreakout] = DefaultBreakoutSettings()
	settings.Strategies[StrategyRangeTrading] = DefaultRangeTradingSettings()

	return settings
}

// DefaultTrendFollowingSettings returns default settings for trend following strategy.
func DefaultTrendFollowingSettings() *StrategySettings {
	return &StrategySettings{
		Name:        StrategyTrendFollowing,
		Enabled:     true,
		Description: "Follows established trends with momentum and volume confirmation. Best for trending markets with ADX > 25.",

		Scoring:  DefaultScoringConfig(),
		Blocking: DefaultBlockingConfig(),

		Entry: &EntryConditions{
			MinScore:          55.0,
			MinADX:            25.0,
			RequireTrendAlign: true,
			VolumeMultiplier:  1.2,
			MinConfidence:     60.0,
			MaxSpread:         0.1,
		},
		Exit: &ExitConditions{
			TakeProfitPercent:   2.0,
			StopLossPercent:     1.0,
			TrailingStop:        true,
			TrailingPercent:     0.5,
			MaxHoldTime:         "4h",
			ExitOnTrendReversal: true,
			MinHoldTime:         "5m",
		},

		PrimaryTimeframe:   "1h",
		SecondaryTimeframe: "15m",
	}
}

// DefaultMeanReversionSettings returns default settings for mean reversion strategy.
func DefaultMeanReversionSettings() *StrategySettings {
	return &StrategySettings{
		Name:        StrategyMeanReversion,
		Enabled:     true,
		Description: "Trades reversals when price deviates significantly from mean. Best for ranging markets with RSI extremes.",

		Scoring: &ScoringConfig{
			TechnicalWeight: 1.2, // Higher weight on technicals for mean reversion
			ContextWeight:   0.8,
			LLMWeight:       1.0,
			HistoryWeight:   1.0,
			MinThreshold:    50.0, // Lower threshold for reversal setups
		},
		Blocking: &BlockingConfig{
			ADXMinimum:         10.0, // Can work with lower trend strength
			ScoreMinimum:       50.0,
			RSIUpperLimit:      85.0, // Look for extremes
			RSILowerLimit:      15.0,
			WinRateMinimum:     35.0,
			MaxBlockingReasons: 10,
			EnableHardBlocks:   true,
			EnableSoftBlocks:   true,
			EnableWarnings:     true,
		},

		Entry: &EntryConditions{
			MinScore:          50.0,
			MinADX:            10.0, // Can enter with low ADX
			RequireTrendAlign: false,
			VolumeMultiplier:  1.0, // Normal volume is fine
			MinConfidence:     50.0,
			MaxSpread:         0.15,
		},
		Exit: &ExitConditions{
			TakeProfitPercent:   1.5,
			StopLossPercent:     1.0,
			TrailingStop:        false, // Fixed targets for mean reversion
			TrailingPercent:     0.0,
			MaxHoldTime:         "2h",
			ExitOnTrendReversal: false, // We're betting on reversal
			MinHoldTime:         "3m",
		},

		PrimaryTimeframe:   "1h",
		SecondaryTimeframe: "5m",
	}
}

// DefaultBreakoutSettings returns default settings for breakout strategy.
func DefaultBreakoutSettings() *StrategySettings {
	return &StrategySettings{
		Name:        StrategyBreakout,
		Enabled:     true,
		Description: "Trades breakouts from consolidation patterns with volume confirmation. Best for transitions from ranging to trending.",

		Scoring: &ScoringConfig{
			TechnicalWeight: 1.0,
			ContextWeight:   1.2, // Higher weight on context (consolidation detection)
			LLMWeight:       1.0,
			HistoryWeight:   1.0,
			MinThreshold:    60.0, // Higher threshold for breakouts
		},
		Blocking: &BlockingConfig{
			ADXMinimum:         15.0, // Starting to build momentum
			ScoreMinimum:       60.0,
			RSIUpperLimit:      75.0, // Not too overbought
			RSILowerLimit:      25.0,
			WinRateMinimum:     40.0,
			MaxBlockingReasons: 10,
			EnableHardBlocks:   true,
			EnableSoftBlocks:   true,
			EnableWarnings:     true,
		},

		Entry: &EntryConditions{
			MinScore:          60.0,
			MinADX:            15.0,
			RequireTrendAlign: true, // Wait for confirmation
			VolumeMultiplier:  1.5,  // Require volume spike
			MinConfidence:     65.0,
			MaxSpread:         0.08, // Tighter spread for breakouts
		},
		Exit: &ExitConditions{
			TakeProfitPercent:   3.0, // Larger target for breakout momentum
			StopLossPercent:     0.8,
			TrailingStop:        true,
			TrailingPercent:     0.6,
			MaxHoldTime:         "6h",
			ExitOnTrendReversal: true,
			MinHoldTime:         "10m",
		},

		PrimaryTimeframe:   "4h",
		SecondaryTimeframe: "1h",
	}
}

// DefaultRangeTradingSettings returns default settings for range trading strategy.
func DefaultRangeTradingSettings() *StrategySettings {
	return &StrategySettings{
		Name:        StrategyRangeTrading,
		Enabled:     true,
		Description: "Trades between identified support and resistance levels. Best for sideways markets with clear boundaries.",

		Scoring: &ScoringConfig{
			TechnicalWeight: 1.1, // Slightly higher weight on technicals
			ContextWeight:   1.1,
			LLMWeight:       0.8, // Lower LLM weight - more mechanical
			HistoryWeight:   1.0,
			MinThreshold:    45.0, // Lower threshold for range setups
		},
		Blocking: &BlockingConfig{
			ADXMinimum:         0.0,  // ADX doesn't matter for ranging
			ScoreMinimum:       45.0,
			RSIUpperLimit:      75.0,
			RSILowerLimit:      25.0,
			WinRateMinimum:     45.0, // Higher win rate required
			MaxBlockingReasons: 10,
			EnableHardBlocks:   true,
			EnableSoftBlocks:   true,
			EnableWarnings:     true,
		},

		Entry: &EntryConditions{
			MinScore:          45.0,
			MinADX:            0.0,   // ADX not relevant for ranging
			RequireTrendAlign: false, // We're looking for range-bound
			VolumeMultiplier:  0.8,   // Lower volume is fine
			MinConfidence:     45.0,
			MaxSpread:         0.12,
		},
		Exit: &ExitConditions{
			TakeProfitPercent:   1.0, // Smaller, consistent targets
			StopLossPercent:     0.6,
			TrailingStop:        false, // Fixed targets for range trading
			TrailingPercent:     0.0,
			MaxHoldTime:         "2h",
			ExitOnTrendReversal: false,
			MinHoldTime:         "2m",
		},

		PrimaryTimeframe:   "1h",
		SecondaryTimeframe: "15m",
	}
}

// Validate checks if the DecisionEngineSettings has valid values.
func (des *DecisionEngineSettings) Validate() error {
	if des.Strategies == nil {
		return fmt.Errorf("strategies map cannot be nil")
	}

	// Validate active strategy exists
	if des.ActiveStrategy != "" {
		if !des.ActiveStrategy.IsValid() {
			return fmt.Errorf("invalid active strategy: %s", des.ActiveStrategy)
		}
		if _, exists := des.Strategies[des.ActiveStrategy]; !exists {
			return fmt.Errorf("active strategy %s not found in strategies", des.ActiveStrategy)
		}
	}

	// Validate each strategy
	for name, strategy := range des.Strategies {
		if strategy == nil {
			return fmt.Errorf("strategy %s has nil settings", name)
		}
		if err := strategy.Validate(); err != nil {
			return fmt.Errorf("strategy %s validation failed: %w", name, err)
		}
	}

	return nil
}

// Validate checks if the StrategySettings has valid values.
func (ss *StrategySettings) Validate() error {
	if !ss.Name.IsValid() {
		return fmt.Errorf("invalid strategy name: %s", ss.Name)
	}

	// Validate embedded configs if present
	if ss.Scoring != nil {
		if err := ss.Scoring.Validate(); err != nil {
			return fmt.Errorf("scoring config: %w", err)
		}
	}

	if ss.Blocking != nil {
		ss.Blocking.Validate() // This method clamps values, doesn't return error
	}

	if ss.Entry != nil {
		if err := ss.Entry.Validate(); err != nil {
			return fmt.Errorf("entry conditions: %w", err)
		}
	}

	if ss.Exit != nil {
		if err := ss.Exit.Validate(); err != nil {
			return fmt.Errorf("exit conditions: %w", err)
		}
	}

	// Validate timeframes if set
	if ss.PrimaryTimeframe != "" && !validTimeframes[ss.PrimaryTimeframe] {
		return fmt.Errorf("invalid primary_timeframe: %s", ss.PrimaryTimeframe)
	}
	if ss.SecondaryTimeframe != "" && !validTimeframes[ss.SecondaryTimeframe] {
		return fmt.Errorf("invalid secondary_timeframe: %s", ss.SecondaryTimeframe)
	}

	return nil
}

// Validate checks if the EntryConditions has valid values.
func (ec *EntryConditions) Validate() error {
	if ec.MinScore < 0 || ec.MinScore > 100 {
		return fmt.Errorf("min_score must be between 0 and 100, got %.1f", ec.MinScore)
	}
	if ec.MinADX < 0 || ec.MinADX > 100 {
		return fmt.Errorf("min_adx must be between 0 and 100, got %.1f", ec.MinADX)
	}
	if ec.VolumeMultiplier < 0 {
		return fmt.Errorf("volume_multiplier must be non-negative, got %.2f", ec.VolumeMultiplier)
	}
	if ec.MinConfidence < 0 || ec.MinConfidence > 100 {
		return fmt.Errorf("min_confidence must be between 0 and 100, got %.1f", ec.MinConfidence)
	}
	if ec.MaxSpread < 0 || ec.MaxSpread > 10 {
		return fmt.Errorf("max_spread must be between 0 and 10, got %.2f", ec.MaxSpread)
	}
	return nil
}

// Validate checks if the ExitConditions has valid values.
func (ex *ExitConditions) Validate() error {
	if ex.TakeProfitPercent < 0 || ex.TakeProfitPercent > 100 {
		return fmt.Errorf("take_profit_percent must be between 0 and 100, got %.1f", ex.TakeProfitPercent)
	}
	if ex.StopLossPercent < 0 || ex.StopLossPercent > 100 {
		return fmt.Errorf("stop_loss_percent must be between 0 and 100, got %.1f", ex.StopLossPercent)
	}
	if ex.TrailingStop && ex.TrailingPercent <= 0 {
		return fmt.Errorf("trailing_percent must be positive when trailing_stop is enabled")
	}
	if ex.TrailingPercent < 0 || ex.TrailingPercent > 100 {
		return fmt.Errorf("trailing_percent must be between 0 and 100, got %.1f", ex.TrailingPercent)
	}
	// Validate duration strings if set
	if ex.MaxHoldTime != "" && !validDurationRegex.MatchString(ex.MaxHoldTime) {
		return fmt.Errorf("invalid max_hold_time format: %s (expected format like 4h, 1d, 7d)", ex.MaxHoldTime)
	}
	if ex.MinHoldTime != "" && !validDurationRegex.MatchString(ex.MinHoldTime) {
		return fmt.Errorf("invalid min_hold_time format: %s (expected format like 5m, 1h)", ex.MinHoldTime)
	}
	return nil
}

// Clone returns a deep copy of the DecisionEngineSettings.
func (des *DecisionEngineSettings) Clone() *DecisionEngineSettings {
	if des == nil {
		return nil
	}

	clone := &DecisionEngineSettings{
		ActiveStrategy: des.ActiveStrategy,
		LastUpdated:    des.LastUpdated,
		Version:        des.Version,
		Strategies:     make(map[StrategyName]*StrategySettings, len(des.Strategies)),
	}

	for name, strategy := range des.Strategies {
		clone.Strategies[name] = strategy.Clone()
	}

	return clone
}

// Clone returns a deep copy of the StrategySettings.
func (ss *StrategySettings) Clone() *StrategySettings {
	if ss == nil {
		return nil
	}

	clone := &StrategySettings{
		Name:               ss.Name,
		Enabled:            ss.Enabled,
		Description:        ss.Description,
		PrimaryTimeframe:   ss.PrimaryTimeframe,
		SecondaryTimeframe: ss.SecondaryTimeframe,
	}

	if ss.Scoring != nil {
		clone.Scoring = ss.Scoring.Clone()
	}
	if ss.Blocking != nil {
		clone.Blocking = ss.Blocking.Clone()
	}
	if ss.Entry != nil {
		clone.Entry = ss.Entry.Clone()
	}
	if ss.Exit != nil {
		clone.Exit = ss.Exit.Clone()
	}

	return clone
}

// Clone returns a deep copy of the EntryConditions.
func (ec *EntryConditions) Clone() *EntryConditions {
	if ec == nil {
		return nil
	}
	return &EntryConditions{
		MinScore:          ec.MinScore,
		MinADX:            ec.MinADX,
		RequireTrendAlign: ec.RequireTrendAlign,
		VolumeMultiplier:  ec.VolumeMultiplier,
		MinConfidence:     ec.MinConfidence,
		MaxSpread:         ec.MaxSpread,
	}
}

// Clone returns a deep copy of the ExitConditions.
func (ex *ExitConditions) Clone() *ExitConditions {
	if ex == nil {
		return nil
	}
	return &ExitConditions{
		TakeProfitPercent:   ex.TakeProfitPercent,
		StopLossPercent:     ex.StopLossPercent,
		TrailingStop:        ex.TrailingStop,
		TrailingPercent:     ex.TrailingPercent,
		MaxHoldTime:         ex.MaxHoldTime,
		ExitOnTrendReversal: ex.ExitOnTrendReversal,
		MinHoldTime:         ex.MinHoldTime,
	}
}

// ToJSON serializes the DecisionEngineSettings to JSON.
func (des *DecisionEngineSettings) ToJSON() ([]byte, error) {
	return json.Marshal(des)
}

// FromJSON deserializes JSON into DecisionEngineSettings.
func FromJSONSettings(data []byte) (*DecisionEngineSettings, error) {
	var settings DecisionEngineSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}
	return &settings, nil
}

// GetStrategy returns the settings for a specific strategy, or nil if not found.
func (des *DecisionEngineSettings) GetStrategy(name StrategyName) *StrategySettings {
	if des.Strategies == nil {
		return nil
	}
	return des.Strategies[name]
}

// GetActiveStrategySettings returns the settings for the currently active strategy.
func (des *DecisionEngineSettings) GetActiveStrategySettings() *StrategySettings {
	return des.GetStrategy(des.ActiveStrategy)
}

// SetStrategy sets or updates a strategy's settings.
func (des *DecisionEngineSettings) SetStrategy(settings *StrategySettings) {
	if des.Strategies == nil {
		des.Strategies = make(map[StrategyName]*StrategySettings)
	}
	if settings != nil {
		des.Strategies[settings.Name] = settings
		des.LastUpdated = time.Now().UnixMilli()
	}
}

// UpdateTimestamp updates the LastUpdated field to current time.
func (des *DecisionEngineSettings) UpdateTimestamp() {
	des.LastUpdated = time.Now().UnixMilli()
}
