// Package types provides shared type definitions to avoid import cycles.
// Epic 10 / Story 10.1: Position Management Optimization
package types

import "time"

// HistoricalBaseline stores the efficiency baseline for a user/mode combination.
// This is the threshold used for efficiency-based exit decisions.
// Shared between autopilot and cache packages to avoid import cycles.
type HistoricalBaseline struct {
	// Mode is the trading mode (scalp, swing, position, ultra_fast)
	Mode string `json:"mode"`

	// AvgExitEfficiency is the average exit efficiency from recent trades
	// This is THE THRESHOLD - exit when current efficiency drops below this
	AvgExitEfficiency float64 `json:"avg_eff"`

	// TradeCount is the number of trades used to calculate the average
	TradeCount int `json:"count"`

	// WindowHours is the lookback window in hours
	WindowHours int `json:"window"`

	// LastUpdated is the Unix timestamp of when this baseline was last calculated
	LastUpdated int64 `json:"updated"`

	// MinEfficiency is the minimum exit efficiency observed in the window
	MinEfficiency float64 `json:"min_eff,omitempty"`

	// MaxEfficiency is the maximum exit efficiency observed in the window
	MaxEfficiency float64 `json:"max_eff,omitempty"`

	// StdDeviation is the standard deviation of exit efficiencies
	StdDeviation float64 `json:"std_dev,omitempty"`
}

// EfficiencyBaseline constants
const (
	// DefaultEfficiencyThreshold is the default threshold when no history exists
	// 50% means exit when efficiency drops to 50% (gave back half the peak profit)
	DefaultEfficiencyThreshold = 0.50

	// DefaultBaselineWindowHours is the default lookback window for baseline calculation
	DefaultBaselineWindowHours = 6

	// MinTradesForBaseline is minimum trades needed for a valid baseline
	// If fewer trades exist, use the default threshold
	MinTradesForBaseline = 3
)

// NewDefaultBaseline creates a default baseline for a mode.
func NewDefaultBaseline(mode string) *HistoricalBaseline {
	return &HistoricalBaseline{
		Mode:              mode,
		AvgExitEfficiency: DefaultEfficiencyThreshold,
		TradeCount:        0,
		WindowHours:       DefaultBaselineWindowHours,
		LastUpdated:       time.Now().Unix(),
	}
}
