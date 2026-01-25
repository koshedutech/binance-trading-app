// Package exitdecision provides exit signal monitoring for the Chain Trading System.
// Epic 14, Story 14.16: Exit Decision Integration
//
// This package monitors open positions and generates exit signals when TP/SL/trailing
// stop conditions are met. It continues running even when Trading is OFF to ensure
// positions can be properly closed.
package exitdecision

import (
	"context"
	"time"
)

// LogPrefix is the standard prefix for all Exit Decision log messages.
const LogPrefix = "[EXIT-DECISION]"

// ============================================================================
// CORE TYPES
// ============================================================================

// ExitType indicates the type of exit signal generated.
type ExitType string

const (
	// ExitTypeTP indicates a take profit exit.
	ExitTypeTP ExitType = "tp"
	// ExitTypeSL indicates a stop loss exit.
	ExitTypeSL ExitType = "sl"
	// ExitTypeTrailing indicates a trailing stop exit.
	ExitTypeTrailing ExitType = "trailing"
	// ExitTypeEfficiency indicates an efficiency-based exit.
	ExitTypeEfficiency ExitType = "efficiency"
	// ExitTypeTimeout indicates a max hold time exit.
	ExitTypeTimeout ExitType = "timeout"
)

// Urgency indicates how urgently an exit should be executed.
type Urgency string

const (
	// UrgencyImmediate means the exit should happen immediately.
	UrgencyImmediate Urgency = "immediate"
	// UrgencyMonitor means the condition is being monitored but not yet critical.
	UrgencyMonitor Urgency = "monitor"
	// UrgencyWarning means approaching exit threshold.
	UrgencyWarning Urgency = "warning"
)

// ExitSignal represents an exit signal for a position.
// Generated when TP/SL/trailing stop conditions are met.
type ExitSignal struct {
	// Position identification
	Symbol string `json:"symbol"`
	Mode   string `json:"mode"` // "scalp", "swing", "position", "ultra_fast"
	Side   string `json:"side"` // "long" or "short"

	// Exit details
	ExitType     ExitType `json:"exit_type"`      // "tp", "sl", "trailing", "efficiency", "timeout"
	TPLevel      int      `json:"tp_level"`       // 1-4 for take profit levels
	TriggerPrice float64  `json:"trigger_price"`  // Price that triggered the signal
	CurrentPrice float64  `json:"current_price"`  // Current market price
	EntryPrice   float64  `json:"entry_price"`    // Position entry price
	Urgency      Urgency  `json:"urgency"`        // "immediate", "monitor", "warning"

	// PnL information
	UnrealizedPnLPercent float64 `json:"unrealized_pnl_percent"` // Current unrealized P&L %
	DistanceToExitPct    float64 `json:"distance_to_exit_pct"`   // How far from exit price (%)

	// Metadata
	Reason    string    `json:"reason"`    // Human-readable reason for the signal
	Timestamp time.Time `json:"timestamp"` // When the signal was generated
	UserID    string    `json:"user_id"`   // User ID for multi-user support
}

// ============================================================================
// POSITION INTERFACE
// ============================================================================

// Position defines the interface for position data needed by Exit Decision.
// This interface allows mocking without depending on the full GiniePosition type.
type Position interface {
	GetSymbol() string
	GetMode() string
	GetSide() string
	GetEntryPrice() float64
	GetCurrentPrice() float64
	GetOriginalQty() float64
	GetRemainingQty() float64
	GetEntryTime() time.Time

	// TP/SL methods
	GetTakeProfitLevels() []TakeProfitLevel
	GetCurrentTPLevel() int
	GetStopLossPrice() float64
	HasMovedToBreakeven() bool

	// Trailing stop methods
	IsTrailingActive() bool
	GetTrailingPercent() float64
	GetTrailingActivationPct() float64
	GetHighestPrice() float64
	GetLowestPrice() float64

	// Efficiency tracking methods
	IsEfficiencyActive() bool
	GetPeakProfit() float64
	GetCurrentProfit() float64
	GetEfficiency() float64

	// Safeguard methods (Story 10.1 Phase 2)
	// S3: Breakeven verification
	GetBreakevenPrice() float64

	// S4: Stale data detection
	GetLastPriceUpdate() time.Time
}

// TakeProfitLevel represents a take profit level configuration.
type TakeProfitLevel struct {
	Level   int     `json:"level"`    // 1-4
	Price   float64 `json:"price"`    // Target price
	Percent float64 `json:"percent"`  // Portion of position to close (0-1)
	GainPct float64 `json:"gain_pct"` // Expected gain % at this level
	Status  string  `json:"status"`   // "pending", "hit"
}

// ============================================================================
// PRICE PROVIDER INTERFACE
// ============================================================================

// PriceProvider defines the interface for getting current prices.
// This is satisfied by the Coin Profiler service.
type PriceProvider interface {
	// GetPrice returns the current price for a symbol.
	GetPrice(ctx context.Context, symbol string) (float64, error)

	// GetPrices returns current prices for multiple symbols.
	GetPrices(ctx context.Context, symbols []string) (map[string]float64, error)
}

// ============================================================================
// POSITION PROVIDER INTERFACE
// ============================================================================

// PositionProvider defines the interface for getting open positions.
// This is satisfied by the GinieAutopilot service.
type PositionProvider interface {
	// GetOpenPositions returns all open positions for a user.
	GetOpenPositions(ctx context.Context, userID string) ([]Position, error)

	// GetPosition returns a specific position by symbol.
	GetPosition(ctx context.Context, userID, symbol string) (Position, error)
}

// ============================================================================
// SERVICE INTERFACE
// ============================================================================

// ExitDecisionService defines the interface for the Exit Decision service.
type ExitDecisionService interface {
	// Start starts the exit decision monitoring service.
	Start(ctx context.Context) error

	// Stop stops the exit decision monitoring service.
	Stop() error

	// IsRunning returns whether the service is currently running.
	IsRunning() bool

	// GetExitSignals returns all pending exit signals for a user.
	GetExitSignals(ctx context.Context, userID string) ([]ExitSignal, error)

	// CheckPosition checks a single position for exit conditions.
	CheckPosition(ctx context.Context, position Position) (*ExitSignal, error)

	// GetStatus returns the current service status.
	GetStatus() *ServiceStatus
}

// ServiceStatus represents the current status of the Exit Decision service.
type ServiceStatus struct {
	Running              bool      `json:"running"`
	StartedAt            time.Time `json:"started_at"`
	Uptime               string    `json:"uptime"`
	PositionsMonitored   int       `json:"positions_monitored"`
	SignalsGenerated     int64     `json:"signals_generated"`
	LastCheckTime        time.Time `json:"last_check_time"`
	CheckIntervalMs      int64     `json:"check_interval_ms"`
	LastError            string    `json:"last_error,omitempty"`
	PriceProviderHealthy bool      `json:"price_provider_healthy"`
}

// ============================================================================
// CONFIGURATION
// ============================================================================

// Config holds configuration for the Exit Decision service.
type Config struct {
	// Check interval for monitoring positions
	CheckInterval time.Duration `json:"check_interval"`

	// Trailing stop defaults
	DefaultTrailingPercent       float64 `json:"default_trailing_percent"`
	DefaultTrailingActivationPct float64 `json:"default_trailing_activation_pct"`

	// Efficiency exit defaults
	DefaultEfficiencyThreshold float64 `json:"default_efficiency_threshold"`

	// Buffer percentages for exit triggers
	TPBufferPercent float64 `json:"tp_buffer_percent"` // Trigger TP slightly before target
	SLBufferPercent float64 `json:"sl_buffer_percent"` // Trigger SL slightly before target

	// Feature flags
	EnableTPMonitoring         bool `json:"enable_tp_monitoring"`
	EnableSLMonitoring         bool `json:"enable_sl_monitoring"`
	EnableTrailingMonitoring   bool `json:"enable_trailing_monitoring"`
	EnableEfficiencyMonitoring bool `json:"enable_efficiency_monitoring"`

	// Safeguards configuration (Story 10.1 Phase 2)
	Safeguards *SafeguardsConfig `json:"safeguards"`

	// Debug mode
	DebugMode bool `json:"debug_mode"`
}

// DefaultConfig returns the default configuration for Exit Decision service.
func DefaultConfig() *Config {
	return &Config{
		// Check every 500ms for responsive exit monitoring
		CheckInterval: 500 * time.Millisecond,

		// Trailing stop defaults
		DefaultTrailingPercent:       1.5,
		DefaultTrailingActivationPct: 1.0,

		// Efficiency exit defaults
		DefaultEfficiencyThreshold: 0.5, // Exit when efficiency < 50%

		// Buffer percentages
		TPBufferPercent: 0.05, // 0.05% buffer for TP
		SLBufferPercent: 0.05, // 0.05% buffer for SL

		// Enable all monitoring by default
		EnableTPMonitoring:         true,
		EnableSLMonitoring:         true,
		EnableTrailingMonitoring:   true,
		EnableEfficiencyMonitoring: true,

		// Safeguards enabled by default
		Safeguards: DefaultSafeguardsConfig(),

		DebugMode: false,
	}
}

// ============================================================================
// SAFEGUARDS CONFIGURATION (Story 10.1 Phase 2)
// ============================================================================

// SafeguardsConfig holds configuration for exit decision safeguards.
// These safeguards prevent common failure scenarios in exit decisions.
type SafeguardsConfig struct {
	// S1: Minimum Hold Time Before Efficiency Exit
	// Prevents exiting within seconds of entry due to early price fluctuations.
	EnableMinHoldTime            bool              `json:"enable_min_hold_time"`
	MinHoldBeforeEfficiencyMins  map[string]int    `json:"min_hold_before_efficiency_mins"` // mode -> minutes

	// S2: Consecutive Signal Requirement (Whipsaw Prevention)
	// Prevents false exits during normal price oscillation by requiring multiple signals.
	EnableWhipsawPrevention      bool `json:"enable_whipsaw_prevention"`
	ConsecutiveSignalsRequired   int  `json:"consecutive_signals_required"`    // Default: 3
	ConsecutiveSignalWindowSecs  int  `json:"consecutive_signal_window_secs"`  // Time window for consecutive signals

	// S3: Breakeven Verification Before Efficiency Exit
	// Prevents efficiency exit when price is below breakeven (invalid efficiency calculation).
	EnableBreakevenVerification  bool `json:"enable_breakeven_verification"`

	// S4: Stale Data Detection
	// Prevents decisions based on outdated price or trend data.
	EnableStaleDataDetection     bool `json:"enable_stale_data_detection"`
	MaxPriceDataAgeSecs          int  `json:"max_price_data_age_secs"`    // Default: 5
	MaxTrendDataAgeSecs          int  `json:"max_trend_data_age_secs"`    // Default: 30

	// S5: Epic 11 Integration Safeguards
	// Ensures graceful fallback to Classic mode when New Engine components fail.
	EnableNewEngineFallback      bool `json:"enable_new_engine_fallback"`
	FallbackToClassicOnError     bool `json:"fallback_to_classic_on_error"`
	MinIndicatorsRequired        int  `json:"min_indicators_required"`     // Default: 1
	RegimeChangeConfirmationSecs int  `json:"regime_change_confirmation_secs"` // Default: 60

	// S6: Decision Mode Consistency
	// Prevents mode changes while positions are open to avoid inconsistent behavior.
	BlockModeChangeWithOpenPositions bool `json:"block_mode_change_with_open_positions"`
}

// DefaultSafeguardsConfig returns sensible defaults for all safeguards.
func DefaultSafeguardsConfig() *SafeguardsConfig {
	return &SafeguardsConfig{
		// S1: Min Hold Time
		EnableMinHoldTime: true,
		MinHoldBeforeEfficiencyMins: map[string]int{
			"ultra_fast": 1,
			"scalp":      2,
			"swing":      5,
			"position":   15,
		},

		// S2: Whipsaw Prevention
		EnableWhipsawPrevention:     true,
		ConsecutiveSignalsRequired:  3,
		ConsecutiveSignalWindowSecs: 10,

		// S3: Breakeven Verification
		EnableBreakevenVerification: true,

		// S4: Stale Data Detection
		EnableStaleDataDetection: true,
		MaxPriceDataAgeSecs:      5,
		MaxTrendDataAgeSecs:      30,

		// S5: New Engine Fallback
		EnableNewEngineFallback:      true,
		FallbackToClassicOnError:     true,
		MinIndicatorsRequired:        1,
		RegimeChangeConfirmationSecs: 60,

		// S6: Mode Consistency
		BlockModeChangeWithOpenPositions: true,
	}
}

// SafeguardResult represents the result of a safeguard check.
type SafeguardResult struct {
	Safeguard     string `json:"safeguard"`      // S1, S2, S3, S4, S5, S6
	Blocked       bool   `json:"blocked"`        // Whether the exit was blocked
	Reason        string `json:"reason"`         // Human-readable reason
	FallbackUsed  bool   `json:"fallback_used"`  // Whether fallback mode was used (S5)
}

// ConsecutiveSignalTracker tracks consecutive signals for whipsaw prevention (S2).
type ConsecutiveSignalTracker struct {
	Symbol                    string    `json:"symbol"`
	ConsecutiveBelowThreshold int       `json:"consec_below"`
	LastBelowThresholdTime    time.Time `json:"below_ts"`
	LastResetTime             time.Time `json:"reset_ts"`
}

// ============================================================================
// HELPER TYPES FOR INTERNAL USE
// ============================================================================

// positionCheck holds the result of checking a single position.
type positionCheck struct {
	Position Position
	Signal   *ExitSignal
	Error    error
}

// monitoredPosition tracks a position being monitored.
type monitoredPosition struct {
	UserID       string
	Symbol       string
	LastCheck    time.Time
	LastPrice    float64
	CheckCount   int64
	SignalCount  int64
	HighestPrice float64
	LowestPrice  float64

	// Safeguard tracking (S2)
	ConsecutiveBelowThreshold int
	LastBelowThresholdTime    time.Time
}
