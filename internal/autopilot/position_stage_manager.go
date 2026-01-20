// Package autopilot provides position stage management for Epic 10 Story 10.1
// This file manages position lifecycle stages from entry through exit
package autopilot

import (
	"log"
	"time"
)

// Stage constants are defined in position_efficiency.go
// Adding StageExiting which is not in position_efficiency.go
const (
	// StageExiting - Position is being closed
	StageExiting = "EXITING"
)

// PositionStageData holds stage-related data for a position
type PositionStageData struct {
	// Current stage
	Stage string `json:"stage"`

	// Breakeven tracking
	BEPrice    float64   `json:"be_price"`    // Breakeven price (entry + fees + buffer)
	BEAchieved bool      `json:"be_achieved"` // Whether breakeven was reached
	BETime     time.Time `json:"be_time"`     // When breakeven was achieved

	// TP1 tracking (if Position Optimization enabled)
	TP1Hit      bool      `json:"tp1_hit"`
	TP1Time     time.Time `json:"tp1_time"`
	TP1Qty      float64   `json:"tp1_qty"`      // Quantity sold at TP1
	TP1Profit   float64   `json:"tp1_profit"`   // Profit booked at TP1
	TP1ExitPct  float64   `json:"tp1_exit_pct"` // Percentage of position sold

	// Efficiency tracking
	EffActive     bool      `json:"eff_active"`     // Efficiency tracking is active
	EffStartTime  time.Time `json:"eff_start_time"` // When efficiency tracking started
	PeakProfit    float64   `json:"peak_profit"`    // Highest profit % achieved
	PeakPrice     float64   `json:"peak_price"`     // Price at peak
	PeakTime      time.Time `json:"peak_time"`      // When peak was achieved
	CurrentProfit float64   `json:"cur_profit"`     // Current profit %
	Efficiency    float64   `json:"efficiency"`     // currentProfit / peakProfit

	// Exit tracking
	ExitDecided   bool   `json:"exit_decided"`
	ExitReason    string `json:"exit_reason"`
	ExitUrgency   string `json:"exit_urgency"` // "immediate" or "normal"
	ExitTriggered bool   `json:"exit_triggered"`

	// Stage transition history
	StageHistory []StageTransition `json:"stage_history,omitempty"`
}

// StageTransition records a stage change
type StageTransition struct {
	FromStage string    `json:"from"`
	ToStage   string    `json:"to"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
}

// PositionStageManager manages position stage transitions
type PositionStageManager struct {
	feePercent          float64 // Trading fee percentage (e.g., 0.10 for 0.10%)
	breakevenBuffer     float64 // Buffer above breakeven (e.g., 0.05 for 0.05%)
	posOptEnabled       bool    // Position Optimization enabled
	efficiencyEnabled   bool    // Efficiency exit enabled
	tp1Percent          float64 // TP1 trigger percentage
}

// NewPositionStageManager creates a new stage manager
func NewPositionStageManager(feePercent, breakevenBuffer, tp1Percent float64, posOptEnabled, efficiencyEnabled bool) *PositionStageManager {
	return &PositionStageManager{
		feePercent:        feePercent,
		breakevenBuffer:   breakevenBuffer,
		posOptEnabled:     posOptEnabled,
		efficiencyEnabled: efficiencyEnabled,
		tp1Percent:        tp1Percent,
	}
}

// InitializeStageData creates initial stage data for a new position
func (m *PositionStageManager) InitializeStageData(entryPrice float64, side string) *PositionStageData {
	bePrice := m.CalculateBreakevenPrice(entryPrice, side)

	return &PositionStageData{
		Stage:        StageRiskZone,
		BEPrice:      bePrice,
		BEAchieved:   false,
		EffActive:    false,
		Efficiency:   1.0, // Start at 100%
		StageHistory: []StageTransition{},
	}
}

// CalculateBreakevenPrice calculates the breakeven price including fees and buffer
// For LONG: entry * (1 + totalBuffer%)
// For SHORT: entry * (1 - totalBuffer%)
func (m *PositionStageManager) CalculateBreakevenPrice(entryPrice float64, side string) float64 {
	// Total buffer = entry fee + exit fee + safety buffer
	// Entry fee: feePercent/2, Exit fee: feePercent/2, Buffer: breakevenBuffer
	totalBuffer := m.feePercent + m.breakevenBuffer

	if side == "LONG" {
		return entryPrice * (1 + totalBuffer/100)
	}
	// SHORT
	return entryPrice * (1 - totalBuffer/100)
}

// ProcessPriceTick processes a price update and manages stage transitions
// Returns true if a stage transition occurred
func (m *PositionStageManager) ProcessPriceTick(
	data *PositionStageData,
	entryPrice, currentPrice float64,
	side string,
	posOpt *PositionOptimizationStatus,
) bool {

	transitioned := false
	prevStage := data.Stage

	switch data.Stage {
	case StageRiskZone:
		transitioned = m.processRiskZone(data, entryPrice, currentPrice, side)

	case StageBreakeven:
		transitioned = m.processBreakeven(data, entryPrice, currentPrice, side, posOpt)

	case StageTP1Pending:
		transitioned = m.processTP1Pending(data, entryPrice, currentPrice, side, posOpt)

	case StageEfficiency:
		m.processEfficiencyStage(data, entryPrice, currentPrice, side)
		// No stage transition in efficiency, but we update efficiency values
	}

	if transitioned {
		// Record transition
		data.StageHistory = append(data.StageHistory, StageTransition{
			FromStage: prevStage,
			ToStage:   data.Stage,
			Reason:    "price_update",
			Timestamp: time.Now(),
			Price:     currentPrice,
		})

		log.Printf("[STAGE] Transition: %s -> %s at price %.4f", prevStage, data.Stage, currentPrice)
	}

	return transitioned
}

// processRiskZone handles the RISK_ZONE stage
func (m *PositionStageManager) processRiskZone(
	data *PositionStageData,
	entryPrice, currentPrice float64,
	side string,
) bool {

	// Check if breakeven reached
	if m.hasReachedBreakeven(data.BEPrice, currentPrice, side) {
		data.BEAchieved = true
		data.BETime = time.Now()
		data.Stage = StageBreakeven
		return true
	}

	return false
}

// processBreakeven handles the BREAKEVEN stage
func (m *PositionStageManager) processBreakeven(
	data *PositionStageData,
	entryPrice, currentPrice float64,
	side string,
	posOpt *PositionOptimizationStatus,
) bool {

	// Check where to go next
	if m.posOptEnabled && posOpt != nil && posOpt.Enabled {
		// Position Optimization enabled - wait for TP1
		data.Stage = StageTP1Pending
		return true
	}

	// No Position Optimization - go directly to efficiency tracking
	if m.efficiencyEnabled {
		m.activateEfficiencyTracking(data, entryPrice, currentPrice, side)
		data.Stage = StageEfficiency
		return true
	}

	// Neither enabled - stay in breakeven (trailing SL handles exit)
	return false
}

// processTP1Pending handles the TP1_PENDING stage
func (m *PositionStageManager) processTP1Pending(
	data *PositionStageData,
	entryPrice, currentPrice float64,
	side string,
	posOpt *PositionOptimizationStatus,
) bool {

	// Check if TP1 was hit (this is typically handled by Position Optimization)
	// Here we just check if we should transition to efficiency
	if posOpt != nil && posOpt.TPLevelUnlocked >= 1 {
		data.TP1Hit = true
		data.TP1Time = time.Now()

		if m.efficiencyEnabled {
			m.activateEfficiencyTracking(data, entryPrice, currentPrice, side)
			data.Stage = StageEfficiency
			return true
		}
	}

	return false
}

// processEfficiencyStage updates efficiency tracking data
func (m *PositionStageManager) processEfficiencyStage(
	data *PositionStageData,
	entryPrice, currentPrice float64,
	side string,
) {
	// Update current profit
	data.CurrentProfit = m.calculateProfitPercent(entryPrice, currentPrice, side)

	// Update peak if current is higher
	if data.CurrentProfit > data.PeakProfit {
		data.PeakProfit = data.CurrentProfit
		data.PeakPrice = currentPrice
		data.PeakTime = time.Now()
	}

	// Calculate efficiency
	if data.PeakProfit > 0 {
		data.Efficiency = data.CurrentProfit / data.PeakProfit
	} else {
		data.Efficiency = 1.0
	}
}

// activateEfficiencyTracking initializes efficiency tracking
func (m *PositionStageManager) activateEfficiencyTracking(
	data *PositionStageData,
	entryPrice, currentPrice float64,
	side string,
) {
	data.EffActive = true
	data.EffStartTime = time.Now()

	// Initialize peak profit at current level
	data.CurrentProfit = m.calculateProfitPercent(entryPrice, currentPrice, side)
	data.PeakProfit = data.CurrentProfit
	data.PeakPrice = currentPrice
	data.PeakTime = time.Now()
	data.Efficiency = 1.0 // At peak, efficiency is 100%
}

// hasReachedBreakeven checks if price has reached or exceeded breakeven
func (m *PositionStageManager) hasReachedBreakeven(bePrice, currentPrice float64, side string) bool {
	if side == "LONG" {
		return currentPrice >= bePrice
	}
	return currentPrice <= bePrice
}

// calculateProfitPercent calculates profit percentage
func (m *PositionStageManager) calculateProfitPercent(entryPrice, currentPrice float64, side string) float64 {
	if entryPrice == 0 {
		return 0
	}

	if side == "LONG" {
		return ((currentPrice - entryPrice) / entryPrice) * 100
	}
	// SHORT
	return ((entryPrice - currentPrice) / entryPrice) * 100
}

// ShouldExitOnEfficiency checks if efficiency has dropped below threshold
func (m *PositionStageManager) ShouldExitOnEfficiency(data *PositionStageData, threshold float64) bool {
	if !data.EffActive || data.Stage != StageEfficiency {
		return false
	}

	return data.Efficiency < threshold
}

// MarkExitDecided marks that an exit decision has been made
func (m *PositionStageManager) MarkExitDecided(data *PositionStageData, reason, urgency string) {
	data.ExitDecided = true
	data.ExitReason = reason
	data.ExitUrgency = urgency
	data.Stage = StageExiting

	data.StageHistory = append(data.StageHistory, StageTransition{
		FromStage: data.Stage,
		ToStage:   StageExiting,
		Reason:    reason,
		Timestamp: time.Now(),
		Price:     0, // Price will be set by caller if needed
	})
}

// GetStageDisplayInfo returns display-friendly stage information
func (m *PositionStageManager) GetStageDisplayInfo(data *PositionStageData) map[string]interface{} {
	return map[string]interface{}{
		"stage":           data.Stage,
		"stage_name":      m.getStageName(data.Stage),
		"be_achieved":     data.BEAchieved,
		"tp1_hit":         data.TP1Hit,
		"efficiency_pct":  data.Efficiency * 100,
		"peak_profit_pct": data.PeakProfit,
		"current_profit":  data.CurrentProfit,
	}
}

// getStageName returns human-readable stage name
func (m *PositionStageManager) getStageName(stage string) string {
	switch stage {
	case StageRiskZone:
		return "Risk Zone"
	case StageBreakeven:
		return "Breakeven Achieved"
	case StageTP1Pending:
		return "Waiting for TP1"
	case StageEfficiency:
		return "Efficiency Tracking"
	case StageExiting:
		return "Exiting"
	default:
		return stage
	}
}

// UpdateFromPositionOptimization updates stage data when Position Optimization triggers
func (m *PositionStageManager) UpdateFromPositionOptimization(
	data *PositionStageData,
	tpLevel int,
	soldQty, soldProfit, exitPct float64,
) {
	if tpLevel == 1 && !data.TP1Hit {
		data.TP1Hit = true
		data.TP1Time = time.Now()
		data.TP1Qty = soldQty
		data.TP1Profit = soldProfit
		data.TP1ExitPct = exitPct

		// Record transition
		data.StageHistory = append(data.StageHistory, StageTransition{
			FromStage: data.Stage,
			ToStage:   "TP1_HIT",
			Reason:    "tp1_triggered",
			Timestamp: time.Now(),
		})
	}
}
