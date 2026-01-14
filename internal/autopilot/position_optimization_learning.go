package autopilot

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ============ ADAPTIVE LEARNING ENGINE ============

// PositionOptimizationLearningRecord represents a single learning record
type PositionOptimizationLearningRecord struct {
	ID                string    `json:"id"`
	Symbol            string    `json:"symbol"`
	Side              string    `json:"side"`
	TPLevel           int       `json:"tp_level"`
	ReentryDecision   bool      `json:"reentry_decision"`
	ReentryConfidence float64   `json:"reentry_confidence"`
	Outcome           string    `json:"outcome"`           // "profit", "loss", "breakeven", "skipped"
	PnL               float64   `json:"pnl"`
	PnLPercent        float64   `json:"pnl_percent"`
	MarketCondition   string    `json:"market_condition"`
	TrendAlignment    bool      `json:"trend_aligned"`
	VolumeRatio       float64   `json:"volume_ratio"`
	RSI               float64   `json:"rsi"`
	HoldDuration      string    `json:"hold_duration"`
	Timestamp         time.Time `json:"timestamp"`
}

// TPLevelStats holds statistics for a specific TP level
type TPLevelStats struct {
	TotalTrades   int     `json:"total_trades"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	Skipped       int     `json:"skipped"`
	TotalPnL      float64 `json:"total_pnl"`
	AvgPnL        float64 `json:"avg_pnl"`
	WinRate       float64 `json:"win_rate"`
	AvgConfidence float64 `json:"avg_confidence"`
}

// ConditionStats holds statistics for a market condition
type ConditionStats struct {
	Condition    string  `json:"condition"`
	TotalTrades  int     `json:"total_trades"`
	Wins         int     `json:"wins"`
	WinRate      float64 `json:"win_rate"`
	AvgPnL       float64 `json:"avg_pnl"`
	ShouldReenter bool   `json:"should_reenter"` // Learned recommendation
}

// PositionOptimizationStats holds aggregate statistics
type PositionOptimizationStats struct {
	TotalReentries      int     `json:"total_reentries"`
	SuccessfulReentries int     `json:"successful_reentries"`
	FailedReentries     int     `json:"failed_reentries"`
	SkippedReentries    int     `json:"skipped_reentries"`
	TotalPnLFromReentry float64 `json:"total_pnl_from_reentry"`
	AvgReentryPnL       float64 `json:"avg_reentry_pnl"`
	OverallWinRate      float64 `json:"overall_win_rate"`

	// Per TP level stats
	ByTPLevel map[int]*TPLevelStats `json:"by_tp_level"`

	// Per market condition stats
	ByMarketCondition map[string]*ConditionStats `json:"by_market_condition"`

	// Per symbol stats
	BySymbol map[string]*SymbolReentryStats `json:"by_symbol"`

	LastUpdated time.Time `json:"last_updated"`
}

// SymbolReentryStats holds per-symbol statistics
type SymbolReentryStats struct {
	Symbol    string  `json:"symbol"`
	Trades    int     `json:"trades"`
	Wins      int     `json:"wins"`
	TotalPnL  float64 `json:"total_pnl"`
	WinRate   float64 `json:"win_rate"`
	AvgPnL    float64 `json:"avg_pnl"`
}

// PositionOptimizationAdaptiveEngine manages adaptive learning for position optimization
type PositionOptimizationAdaptiveEngine struct {
	mu               sync.RWMutex
	records          []PositionOptimizationLearningRecord
	stats            *PositionOptimizationStats
	config           *PositionOptimizationConfig

	// Dynamically adjusted parameters
	currentReentryPct    float64 // Adjusted re-entry percentage (starts at config value)
	currentConfidenceMin float64 // Adjusted min confidence (starts at config value)

	// Window sizes
	shortTermWindow int // Last 20 trades for quick adjustments
	longTermWindow  int // Last 100 trades for trend analysis

	// File persistence
	dataFile string
}

// NewPositionOptimizationAdaptiveEngine creates a new adaptive learning engine
func NewPositionOptimizationAdaptiveEngine(config *PositionOptimizationConfig, dataFile string) *PositionOptimizationAdaptiveEngine {
	engine := &PositionOptimizationAdaptiveEngine{
		records:              []PositionOptimizationLearningRecord{},
		config:               config,
		currentReentryPct:    config.ReentryPercent,
		currentConfidenceMin: config.AIMinConfidence,
		shortTermWindow:      config.AdaptiveWindowTrades,
		longTermWindow:       100,
		dataFile:             dataFile,
	}

	engine.stats = &PositionOptimizationStats{
		ByTPLevel:         make(map[int]*TPLevelStats),
		ByMarketCondition: make(map[string]*ConditionStats),
		BySymbol:          make(map[string]*SymbolReentryStats),
	}

	// Initialize TP level stats
	for i := 1; i <= 3; i++ {
		engine.stats.ByTPLevel[i] = &TPLevelStats{}
	}

	// Load existing data if available
	engine.loadFromFile()

	return engine
}

// RecordOutcome records the outcome of a re-entry decision
func (e *PositionOptimizationAdaptiveEngine) RecordOutcome(record *PositionOptimizationLearningRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Generate ID if not set
	if record.ID == "" {
		record.ID = fmt.Sprintf("po_%d_%s", time.Now().UnixNano(), record.Symbol)
	}
	record.Timestamp = time.Now()

	// Add record
	e.records = append(e.records, *record)

	// Update stats
	e.updateStats(record)

	// Check if we should adjust parameters
	if len(e.records) >= e.config.AdaptiveMinTrades {
		e.adjustParameters()
	}

	// Keep records under limit
	if len(e.records) > e.longTermWindow*2 {
		e.records = e.records[len(e.records)-e.longTermWindow*2:]
	}

	// Save to file periodically
	if len(e.records)%10 == 0 {
		go e.saveToFile()
	}
}

// updateStats updates aggregate statistics
func (e *PositionOptimizationAdaptiveEngine) updateStats(record *PositionOptimizationLearningRecord) {
	s := e.stats

	// Overall stats
	if record.ReentryDecision {
		s.TotalReentries++
		s.TotalPnLFromReentry += record.PnL

		if record.Outcome == "profit" {
			s.SuccessfulReentries++
		} else if record.Outcome == "loss" {
			s.FailedReentries++
		}
	} else {
		s.SkippedReentries++
	}

	if s.TotalReentries > 0 {
		s.AvgReentryPnL = s.TotalPnLFromReentry / float64(s.TotalReentries)
		s.OverallWinRate = float64(s.SuccessfulReentries) / float64(s.TotalReentries) * 100
	}

	// TP level stats
	if tpStats, ok := s.ByTPLevel[record.TPLevel]; ok {
		tpStats.TotalTrades++
		tpStats.TotalPnL += record.PnL

		if record.Outcome == "profit" {
			tpStats.Wins++
		} else if record.Outcome == "loss" {
			tpStats.Losses++
		} else if record.Outcome == "skipped" {
			tpStats.Skipped++
		}

		if tpStats.TotalTrades > 0 {
			tpStats.AvgPnL = tpStats.TotalPnL / float64(tpStats.TotalTrades)
			tpStats.WinRate = float64(tpStats.Wins) / float64(tpStats.TotalTrades-tpStats.Skipped) * 100
		}
		tpStats.AvgConfidence = (tpStats.AvgConfidence*float64(tpStats.TotalTrades-1) + record.ReentryConfidence) / float64(tpStats.TotalTrades)
	}

	// Market condition stats
	if record.MarketCondition != "" {
		if _, ok := s.ByMarketCondition[record.MarketCondition]; !ok {
			s.ByMarketCondition[record.MarketCondition] = &ConditionStats{
				Condition: record.MarketCondition,
			}
		}
		condStats := s.ByMarketCondition[record.MarketCondition]
		condStats.TotalTrades++
		if record.Outcome == "profit" {
			condStats.Wins++
		}
		if condStats.TotalTrades > 0 {
			condStats.WinRate = float64(condStats.Wins) / float64(condStats.TotalTrades) * 100
			condStats.AvgPnL = (condStats.AvgPnL*float64(condStats.TotalTrades-1) + record.PnL) / float64(condStats.TotalTrades)
		}
		// Learn if we should reenter for this condition
		condStats.ShouldReenter = condStats.WinRate >= 50 && condStats.AvgPnL > 0
	}

	// Symbol stats
	if _, ok := s.BySymbol[record.Symbol]; !ok {
		s.BySymbol[record.Symbol] = &SymbolReentryStats{Symbol: record.Symbol}
	}
	symStats := s.BySymbol[record.Symbol]
	symStats.Trades++
	symStats.TotalPnL += record.PnL
	if record.Outcome == "profit" {
		symStats.Wins++
	}
	if symStats.Trades > 0 {
		symStats.WinRate = float64(symStats.Wins) / float64(symStats.Trades) * 100
		symStats.AvgPnL = symStats.TotalPnL / float64(symStats.Trades)
	}

	s.LastUpdated = time.Now()
}

// adjustParameters adjusts re-entry parameters based on performance
func (e *PositionOptimizationAdaptiveEngine) adjustParameters() {
	if len(e.records) < e.config.AdaptiveMinTrades {
		return
	}

	// Get short-term performance
	shortTermRecords := e.getRecentRecords(e.shortTermWindow)
	shortTermWinRate := e.calculateWinRate(shortTermRecords)
	shortTermAvgPnL := e.calculateAvgPnL(shortTermRecords)

	// Adjust re-entry percentage based on win rate
	// Higher win rate = increase re-entry amount
	// Lower win rate = decrease re-entry amount
	maxAdjust := e.config.AdaptiveMaxReentryPctAdj

	if shortTermWinRate > 70 {
		// Increase re-entry percentage (up to max adjustment)
		adjustment := (shortTermWinRate - 70) / 30 * maxAdjust
		e.currentReentryPct = minFloat64(e.config.ReentryPercent+adjustment, 100)
	} else if shortTermWinRate < 50 {
		// Decrease re-entry percentage
		adjustment := (50 - shortTermWinRate) / 50 * maxAdjust
		e.currentReentryPct = maxFloat64(e.config.ReentryPercent-adjustment, 50)
	} else {
		// Gradually return to default
		e.currentReentryPct = e.currentReentryPct*0.9 + e.config.ReentryPercent*0.1
	}

	// Adjust confidence threshold based on false positive rate
	falsePositives := e.countFalsePositives(shortTermRecords)
	if len(shortTermRecords) > 0 {
		fpRate := float64(falsePositives) / float64(len(shortTermRecords))
		if fpRate > 0.3 {
			// Too many false positives, increase confidence threshold
			e.currentConfidenceMin = minFloat64(e.currentConfidenceMin+0.05, 0.9)
		} else if fpRate < 0.1 && shortTermAvgPnL > 0 {
			// Few false positives and profitable, can lower threshold
			e.currentConfidenceMin = maxFloat64(e.currentConfidenceMin-0.02, 0.5)
		}
	}
}

// getRecentRecords returns the most recent N records
func (e *PositionOptimizationAdaptiveEngine) getRecentRecords(n int) []PositionOptimizationLearningRecord {
	if len(e.records) <= n {
		return e.records
	}
	return e.records[len(e.records)-n:]
}

// calculateWinRate calculates win rate for a set of records
func (e *PositionOptimizationAdaptiveEngine) calculateWinRate(records []PositionOptimizationLearningRecord) float64 {
	if len(records) == 0 {
		return 0
	}
	wins := 0
	total := 0
	for _, r := range records {
		if r.ReentryDecision {
			total++
			if r.Outcome == "profit" {
				wins++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(wins) / float64(total) * 100
}

// calculateAvgPnL calculates average PnL for a set of records
func (e *PositionOptimizationAdaptiveEngine) calculateAvgPnL(records []PositionOptimizationLearningRecord) float64 {
	if len(records) == 0 {
		return 0
	}
	total := 0.0
	for _, r := range records {
		total += r.PnL
	}
	return total / float64(len(records))
}

// countFalsePositives counts high-confidence decisions that resulted in losses
func (e *PositionOptimizationAdaptiveEngine) countFalsePositives(records []PositionOptimizationLearningRecord) int {
	count := 0
	for _, r := range records {
		if r.ReentryDecision && r.ReentryConfidence > e.currentConfidenceMin && r.Outcome == "loss" {
			count++
		}
	}
	return count
}

// GetRecommendedReentryPercent returns the current recommended re-entry percentage
func (e *PositionOptimizationAdaptiveEngine) GetRecommendedReentryPercent() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentReentryPct
}

// GetConfidenceThreshold returns the current confidence threshold
func (e *PositionOptimizationAdaptiveEngine) GetConfidenceThreshold() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentConfidenceMin
}

// ShouldSkipReentry returns true if learning suggests skipping re-entry for given conditions
func (e *PositionOptimizationAdaptiveEngine) ShouldSkipReentry(tpLevel int, condition string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Check TP level performance
	if tpStats, ok := e.stats.ByTPLevel[tpLevel]; ok {
		if tpStats.TotalTrades >= 10 && tpStats.WinRate < 40 {
			return true
		}
	}

	// Check market condition performance
	if condStats, ok := e.stats.ByMarketCondition[condition]; ok {
		if condStats.TotalTrades >= 10 && !condStats.ShouldReenter {
			return true
		}
	}

	return false
}

// GetStats returns current statistics
func (e *PositionOptimizationAdaptiveEngine) GetStats() *PositionOptimizationStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// GetSymbolRecommendation returns whether to trade a specific symbol based on history
func (e *PositionOptimizationAdaptiveEngine) GetSymbolRecommendation(symbol string) (shouldTrade bool, confidence float64, reason string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	symStats, ok := e.stats.BySymbol[symbol]
	if !ok || symStats.Trades < 5 {
		return true, 0.5, "insufficient data"
	}

	if symStats.WinRate >= 60 && symStats.AvgPnL > 0 {
		return true, 0.8, fmt.Sprintf("strong performer (%.0f%% win rate, $%.2f avg)", symStats.WinRate, symStats.AvgPnL)
	} else if symStats.WinRate >= 50 {
		return true, 0.6, fmt.Sprintf("average performer (%.0f%% win rate)", symStats.WinRate)
	} else if symStats.WinRate >= 40 {
		return true, 0.4, fmt.Sprintf("below average (%.0f%% win rate)", symStats.WinRate)
	}

	return false, 0.2, fmt.Sprintf("poor performer (%.0f%% win rate, $%.2f avg)", symStats.WinRate, symStats.AvgPnL)
}

// saveToFile persists learning data to file
func (e *PositionOptimizationAdaptiveEngine) saveToFile() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.dataFile == "" {
		return nil
	}

	data := struct {
		Records          []PositionOptimizationLearningRecord `json:"records"`
		Stats            *PositionOptimizationStats           `json:"stats"`
		CurrentReentryPct float64                     `json:"current_reentry_pct"`
		CurrentConfMin   float64                      `json:"current_confidence_min"`
		SavedAt          time.Time                    `json:"saved_at"`
	}{
		Records:          e.records,
		Stats:            e.stats,
		CurrentReentryPct: e.currentReentryPct,
		CurrentConfMin:   e.currentConfidenceMin,
		SavedAt:          time.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal learning data: %w", err)
	}

	return os.WriteFile(e.dataFile, jsonData, 0644)
}

// loadFromFile loads learning data from file
func (e *PositionOptimizationAdaptiveEngine) loadFromFile() error {
	if e.dataFile == "" {
		return nil
	}

	data, err := os.ReadFile(e.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet
		}
		return fmt.Errorf("failed to read learning data: %w", err)
	}

	var loaded struct {
		Records          []PositionOptimizationLearningRecord `json:"records"`
		Stats            *PositionOptimizationStats           `json:"stats"`
		CurrentReentryPct float64                     `json:"current_reentry_pct"`
		CurrentConfMin   float64                      `json:"current_confidence_min"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("failed to unmarshal learning data: %w", err)
	}

	e.records = loaded.Records
	if loaded.Stats != nil {
		e.stats = loaded.Stats
	}
	if loaded.CurrentReentryPct > 0 {
		e.currentReentryPct = loaded.CurrentReentryPct
	}
	if loaded.CurrentConfMin > 0 {
		e.currentConfidenceMin = loaded.CurrentConfMin
	}

	return nil
}

// minFloat64 returns the smaller of two float64 values
// Note: Named to avoid shadowing Go 1.21+ builtin min()
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// maxFloat64 returns the larger of two float64 values
// Note: Named to avoid shadowing Go 1.21+ builtin max()
func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
