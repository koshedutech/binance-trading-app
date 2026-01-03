package autopilot

import "time"

// ============ RE-ENTRY STATE MACHINE ============

// ReentryState represents the state of a re-entry cycle
type ReentryState string

const (
	ReentryStateNone      ReentryState = "NONE"       // No re-entry pending
	ReentryStateWaiting   ReentryState = "WAITING"    // Waiting for price to return to breakeven
	ReentryStateExecuting ReentryState = "EXECUTING"  // Placing re-entry order
	ReentryStateCompleted ReentryState = "COMPLETED"  // Re-entry filled successfully
	ReentryStateFailed    ReentryState = "FAILED"     // Re-entry failed or timed out
	ReentryStateSkipped   ReentryState = "SKIPPED"    // AI decided to skip re-entry
)

// ============ RE-ENTRY CYCLE TRACKING ============

// ReentryCycle tracks a single sell -> buyback cycle
type ReentryCycle struct {
	CycleNumber int          `json:"cycle_number"` // 1, 2, 3...
	TPLevel     int          `json:"tp_level"`     // 1=0.3%, 2=0.6%, 3=1%
	Mode        string       `json:"mode"`         // scalp_reentry
	Side        string       `json:"side"`         // LONG or SHORT

	// Sell details
	SellPrice    float64   `json:"sell_price"`
	SellQuantity float64   `json:"sell_quantity"`
	SellPnL      float64   `json:"sell_pnl"`       // Realized PnL from this sell
	SellOrderID  int64     `json:"sell_order_id"`
	SellTime     time.Time `json:"sell_time"`

	// Re-entry details
	ReentryTargetPrice float64      `json:"reentry_target_price"` // Breakeven + buffer
	ReentryQuantity    float64      `json:"reentry_quantity"`     // SellQty * ReentryPercent
	ReentryState       ReentryState `json:"reentry_state"`
	ReentryAttempts    int          `json:"reentry_attempts"`
	ReentryOrderID     int64        `json:"reentry_order_id"`
	ReentryFilledPrice float64      `json:"reentry_filled_price"`
	ReentryFilledQty   float64      `json:"reentry_filled_qty"`
	ReentryFillTime    time.Time    `json:"reentry_fill_time"`

	// AI Decision
	AIDecision *ReentryAIDecision `json:"ai_decision,omitempty"`

	// Timing
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"` // Human readable

	// Outcome tracking
	Outcome       string  `json:"outcome"`         // "profit", "loss", "breakeven", "skipped"
	OutcomePnL    float64 `json:"outcome_pnl"`     // Final PnL from this cycle
	OutcomeReason string  `json:"outcome_reason"`  // Why this outcome
}

// ============ SCALP RE-ENTRY STATUS ============

// ScalpReentryStatus tracks all re-entry state for a position
type ScalpReentryStatus struct {
	Enabled bool `json:"enabled"`

	// Cycle tracking
	CurrentCycle      int            `json:"current_cycle"`
	Cycles            []ReentryCycle `json:"cycles"`
	AccumulatedProfit float64        `json:"accumulated_profit"` // Total realized profit across all cycles

	// TP Level gating
	TPLevelUnlocked int  `json:"tp_level_unlocked"` // Highest unlocked TP (0, 1, 2, 3)
	NextTPBlocked   bool `json:"next_tp_blocked"`   // Blocked waiting for re-entry to complete

	// Current position state
	OriginalEntryPrice float64 `json:"original_entry_price"` // Initial entry price
	CurrentBreakeven   float64 `json:"current_breakeven"`    // Current breakeven after partial closes
	RemainingQuantity  float64 `json:"remaining_quantity"`   // Current remaining quantity

	// Dynamic SL (after 1% threshold)
	DynamicSLActive        bool    `json:"dynamic_sl_active"`          // Activated after 1% reached
	DynamicSLPrice         float64 `json:"dynamic_sl_price"`           // Current dynamic SL price
	ProtectedProfit        float64 `json:"protected_profit"`           // 60% of accumulated profit
	MaxAllowableLoss       float64 `json:"max_allowable_loss"`         // 40% of accumulated profit

	// Final portion tracking (20% after 1%)
	FinalPortionActive    bool    `json:"final_portion_active"`    // After 80% sold at 1%
	FinalPortionQty       float64 `json:"final_portion_qty"`       // 20% remaining quantity
	FinalTrailingPeak     float64 `json:"final_trailing_peak"`     // Peak price for 5% trailing
	FinalTrailingPercent  float64 `json:"final_trailing_percent"`  // 5.0% default
	FinalTrailingActive   bool    `json:"final_trailing_active"`   // Whether trailing is engaged

	// Configuration (per position)
	ReentryPercent     float64 `json:"reentry_percent"`      // 0.8 = 80% of sold qty
	ReentryPriceBuffer float64 `json:"reentry_price_buffer"` // 0.05% buffer near breakeven

	// Statistics
	TotalCyclesCompleted int     `json:"total_cycles_completed"`
	TotalReentries       int     `json:"total_reentries"`
	SuccessfulReentries  int     `json:"successful_reentries"`
	SkippedReentries     int     `json:"skipped_reentries"`
	TotalCyclePnL        float64 `json:"total_cycle_pnl"`

	// Timestamps
	StartedAt   time.Time `json:"started_at"`
	LastUpdate  time.Time `json:"last_update"`

	// Debug/Audit
	DebugLog []string `json:"debug_log,omitempty"`

	// Manual Intervention Alert (visible to UI)
	NeedsManualIntervention   bool   `json:"needs_manual_intervention"`   // True when position is stuck
	ManualInterventionReason  string `json:"manual_intervention_reason"`  // Reason for manual intervention
	ManualInterventionAlertAt string `json:"manual_intervention_alert_at"` // Timestamp when alert was triggered
}

// ============ AI DECISION TYPES ============

// ReentryAIDecision holds AI decision for a re-entry opportunity
type ReentryAIDecision struct {
	ShouldReenter     bool    `json:"should_reenter"`
	Confidence        float64 `json:"confidence"`          // 0.0-1.0
	RecommendedQtyPct float64 `json:"recommended_qty_pct"` // 0.5-1.0 of configured reentry%
	Reasoning         string  `json:"reasoning"`
	MarketCondition   string  `json:"market_condition"` // "trending", "ranging", "volatile", "calm"
	TrendAlignment    bool    `json:"trend_aligned"`    // Is re-entry aligned with trend?
	RiskLevel         string  `json:"risk_level"`       // "low", "medium", "high"
	Timestamp         time.Time `json:"timestamp"`

	// Detailed factors
	TrendStrength     float64 `json:"trend_strength"`     // 0-100
	MomentumScore     float64 `json:"momentum_score"`     // 0-100
	VolumeConfirmed   bool    `json:"volume_confirmed"`
	RSIValue          float64 `json:"rsi_value"`
	PriceActionSignal string  `json:"price_action_signal"` // "bullish", "bearish", "neutral"
}

// TPTimingDecision holds AI decision for taking profit
type TPTimingDecision struct {
	ShouldTake       bool    `json:"should_take"`
	Confidence       float64 `json:"confidence"`
	OptimalPercent   float64 `json:"optimal_percent"`  // Suggested % to sell (may differ from config)
	Reasoning        string  `json:"reasoning"`
	MomentumStatus   string  `json:"momentum_status"`  // "accelerating", "stable", "decelerating"
	VolumeStatus     string  `json:"volume_status"`    // "increasing", "stable", "decreasing"
	ResistanceNear   bool    `json:"resistance_near"`  // Near key resistance
	Timestamp        time.Time `json:"timestamp"`
}

// DynamicSLDecision holds AI decision for dynamic stop loss
type DynamicSLDecision struct {
	RecommendedSL     float64 `json:"recommended_sl"`
	ProtectionLevel   float64 `json:"protection_level"`  // % of profit to protect
	Reasoning         string  `json:"reasoning"`
	VolatilityFactor  float64 `json:"volatility_factor"` // Higher = wider SL
	TrendSupport      float64 `json:"trend_support"`     // Nearest support level
	Confidence        float64 `json:"confidence"`
	Timestamp         time.Time `json:"timestamp"`
}

// FinalExitDecision holds AI decision for final 20% exit
type FinalExitDecision struct {
	ShouldExit        bool    `json:"should_exit"`
	ExitReason        string  `json:"exit_reason"` // "trailing_hit", "momentum_loss", "target_reached", "manual"
	OptimalExitPrice  float64 `json:"optimal_exit_price"`
	Confidence        float64 `json:"confidence"`
	Reasoning         string  `json:"reasoning"`
	MomentumRemaining float64 `json:"momentum_remaining"` // 0-100, how much momentum is left
	Timestamp         time.Time `json:"timestamp"`
}

// ============ MARKET DATA FOR AI ANALYSIS ============

// ScalpReentryMarketData holds market data for AI analysis
type ScalpReentryMarketData struct {
	Symbol       string  `json:"symbol"`
	CurrentPrice float64 `json:"current_price"`
	EntryPrice   float64 `json:"entry_price"`
	Breakeven    float64 `json:"breakeven"`
	Side         string  `json:"side"` // LONG or SHORT

	// Price action
	PriceChange1m  float64 `json:"price_change_1m"`
	PriceChange5m  float64 `json:"price_change_5m"`
	PriceChange15m float64 `json:"price_change_15m"`
	DistanceFromBE float64 `json:"distance_from_be"` // % from breakeven

	// Indicators
	RSI14       float64 `json:"rsi_14"`
	MACD        float64 `json:"macd"`
	MACDSignal  float64 `json:"macd_signal"`
	MACDHist    float64 `json:"macd_histogram"`
	EMA20       float64 `json:"ema_20"`
	EMA50       float64 `json:"ema_50"`
	BollingerUpper float64 `json:"bollinger_upper"`
	BollingerLower float64 `json:"bollinger_lower"`
	ATR14       float64 `json:"atr_14"`

	// Volume
	Volume24h     float64 `json:"volume_24h"`
	VolumeRatio   float64 `json:"volume_ratio"` // Current vs average
	VolumeProfile string  `json:"volume_profile"` // "increasing", "decreasing", "stable"

	// Trend
	Trend5m       string  `json:"trend_5m"`  // "bullish", "bearish", "neutral"
	Trend15m      string  `json:"trend_15m"`
	Trend1h       string  `json:"trend_1h"`
	TrendStrength float64 `json:"trend_strength"` // 0-100
	ADX           float64 `json:"adx"`

	// Support/Resistance
	NearestSupport    float64 `json:"nearest_support"`
	NearestResistance float64 `json:"nearest_resistance"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// ============ SENTIMENT TYPES ============

// MarketSentimentResult holds sentiment analysis result
type MarketSentimentResult struct {
	Symbol     string  `json:"symbol"`
	Sentiment  string  `json:"sentiment"` // "bullish", "bearish", "neutral", "mixed"
	Score      float64 `json:"score"`     // -100 to +100
	Confidence float64 `json:"confidence"`

	// Components
	TechnicalSentiment  float64 `json:"technical_sentiment"`  // -100 to +100
	MomentumSentiment   float64 `json:"momentum_sentiment"`   // -100 to +100
	VolumeSentiment     float64 `json:"volume_sentiment"`     // -100 to +100
	TrendSentiment      float64 `json:"trend_sentiment"`      // -100 to +100

	// Context
	MarketPhase    string `json:"market_phase"`    // "accumulation", "markup", "distribution", "markdown"
	VolatilityEnv  string `json:"volatility_env"`  // "low", "normal", "high", "extreme"
	TrendPhase     string `json:"trend_phase"`     // "early", "mature", "late", "reversal"

	Timestamp time.Time `json:"timestamp"`
}

// ============ CONFIGURATION ============

// ScalpReentryConfig holds configuration for scalp re-entry mode
type ScalpReentryConfig struct {
	// Master toggle
	Enabled bool `json:"enabled"`

	// TP Levels configuration
	TP1Percent     float64 `json:"tp1_percent"`      // 0.3 (0.3% profit)
	TP1SellPercent float64 `json:"tp1_sell_percent"` // 30 (sell 30%)
	TP2Percent     float64 `json:"tp2_percent"`      // 0.6 (0.6% profit)
	TP2SellPercent float64 `json:"tp2_sell_percent"` // 50 (sell 50% of remaining)
	TP3Percent     float64 `json:"tp3_percent"`      // 1.0 (1% profit)
	TP3SellPercent float64 `json:"tp3_sell_percent"` // 80 (sell 80%, keep 20%)

	// Re-entry configuration
	ReentryPercent     float64 `json:"reentry_percent"`      // 80 (buy back 80% of sold qty)
	ReentryPriceBuffer float64 `json:"reentry_price_buffer"` // 0.05 (0.05% buffer from breakeven)
	MaxReentryAttempts int     `json:"max_reentry_attempts"` // 3 max attempts before skipping
	ReentryTimeoutSec  int     `json:"reentry_timeout_sec"`  // 300 (5 min timeout)

	// Final portion (20% remaining after 1%)
	FinalTrailingPercent float64 `json:"final_trailing_percent"` // 5.0 (5% trailing from peak)
	FinalHoldMinPercent  float64 `json:"final_hold_min_percent"` // 20 (minimum 20% to hold)

	// Dynamic SL after 1% reached
	DynamicSLMaxLossPct   float64 `json:"dynamic_sl_max_loss_pct"` // 40 (can lose 40% of profit max)
	DynamicSLProtectPct   float64 `json:"dynamic_sl_protect_pct"`  // 60 (protect 60% of profit)
	DynamicSLUpdateIntSec int     `json:"dynamic_sl_update_int"`   // 30 (update every 30s)

	// AI Configuration
	UseAIDecisions   bool    `json:"use_ai_decisions"`    // Enable AI for re-entry decisions
	AIMinConfidence  float64 `json:"ai_min_confidence"`   // 0.65 minimum confidence
	AITPOptimization bool    `json:"ai_tp_optimization"`  // Use AI to optimize TP timing
	AIDynamicSL      bool    `json:"ai_dynamic_sl"`       // Use AI for dynamic SL decisions

	// Multi-agent configuration
	UseMultiAgent        bool `json:"use_multi_agent"`         // Enable multi-agent system
	EnableSentimentAgent bool `json:"enable_sentiment_agent"`  // Enable sentiment analysis
	EnableRiskAgent      bool `json:"enable_risk_agent"`       // Enable risk management agent
	EnableTPAgent        bool `json:"enable_tp_agent"`         // Enable TP timing agent

	// Adaptive learning
	EnableAdaptiveLearning   bool    `json:"enable_adaptive_learning"`
	AdaptiveWindowTrades     int     `json:"adaptive_window_trades"`      // 20 trades window
	AdaptiveMinTrades        int     `json:"adaptive_min_trades"`         // 10 trades before adjusting
	AdaptiveMaxReentryPctAdj float64 `json:"adaptive_max_reentry_adjust"` // Max 20% adjustment

	// Risk limits
	MaxCyclesPerPosition int     `json:"max_cycles_per_position"` // 10 max cycles
	MaxDailyReentries    int     `json:"max_daily_reentries"`     // 50 max per day
	MinPositionSizeUSD   float64 `json:"min_position_size_usd"`   // $10 minimum
}

// DefaultScalpReentryConfig returns default configuration
func DefaultScalpReentryConfig() ScalpReentryConfig {
	return ScalpReentryConfig{
		Enabled: false, // Disabled by default, feature flag

		// TP Levels
		TP1Percent:     0.3,
		TP1SellPercent: 30,
		TP2Percent:     0.6,
		TP2SellPercent: 50,
		TP3Percent:     1.0,
		TP3SellPercent: 80,

		// Re-entry
		ReentryPercent:     80,
		ReentryPriceBuffer: 0.05,
		MaxReentryAttempts: 3,
		ReentryTimeoutSec:  300,

		// Final portion
		FinalTrailingPercent: 5.0,
		FinalHoldMinPercent:  20,

		// Dynamic SL
		DynamicSLMaxLossPct:   40,
		DynamicSLProtectPct:   60,
		DynamicSLUpdateIntSec: 30,

		// AI
		UseAIDecisions:   true,
		AIMinConfidence:  0.65,
		AITPOptimization: true,
		AIDynamicSL:      true,

		// Multi-agent
		UseMultiAgent:        true,
		EnableSentimentAgent: true,
		EnableRiskAgent:      true,
		EnableTPAgent:        true,

		// Adaptive learning
		EnableAdaptiveLearning:   true,
		AdaptiveWindowTrades:     20,
		AdaptiveMinTrades:        10,
		AdaptiveMaxReentryPctAdj: 20,

		// Risk limits
		MaxCyclesPerPosition: 10,
		MaxDailyReentries:    50,
		MinPositionSizeUSD:   10,
	}
}

// ============ HELPER METHODS ============

// NewScalpReentryStatus creates a new scalp reentry status for a position
func NewScalpReentryStatus(entryPrice, quantity float64, config ScalpReentryConfig) *ScalpReentryStatus {
	return &ScalpReentryStatus{
		Enabled:              true,
		CurrentCycle:         0,
		Cycles:               []ReentryCycle{},
		AccumulatedProfit:    0,
		TPLevelUnlocked:      0,
		NextTPBlocked:        false,
		OriginalEntryPrice:   entryPrice,
		CurrentBreakeven:     entryPrice,
		RemainingQuantity:    quantity,
		DynamicSLActive:      false,
		FinalPortionActive:   false,
		ReentryPercent:       config.ReentryPercent / 100.0, // Convert to decimal
		ReentryPriceBuffer:   config.ReentryPriceBuffer / 100.0,
		StartedAt:            time.Now(),
		LastUpdate:           time.Now(),
		DebugLog:             []string{},
	}
}

// AddDebugLog adds a debug log entry
func (s *ScalpReentryStatus) AddDebugLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	s.DebugLog = append(s.DebugLog, timestamp+": "+message)
	// Keep only last 50 entries
	if len(s.DebugLog) > 50 {
		s.DebugLog = s.DebugLog[len(s.DebugLog)-50:]
	}
	s.LastUpdate = time.Now()
}

// GetCurrentCycle returns the current active cycle or nil
func (s *ScalpReentryStatus) GetCurrentCycle() *ReentryCycle {
	if s.CurrentCycle <= 0 || s.CurrentCycle > len(s.Cycles) {
		return nil
	}
	return &s.Cycles[s.CurrentCycle-1]
}

// IsWaitingForReentry returns true if waiting for re-entry
func (s *ScalpReentryStatus) IsWaitingForReentry() bool {
	cycle := s.GetCurrentCycle()
	return cycle != nil && cycle.ReentryState == ReentryStateWaiting
}

// CanProceedToNextTP returns true if next TP level is allowed
func (s *ScalpReentryStatus) CanProceedToNextTP() bool {
	if !s.NextTPBlocked {
		return true
	}
	cycle := s.GetCurrentCycle()
	return cycle != nil && (cycle.ReentryState == ReentryStateCompleted || cycle.ReentryState == ReentryStateSkipped)
}

// GetTPConfig returns TP percent and sell percent for a level
func (c *ScalpReentryConfig) GetTPConfig(level int) (tpPercent, sellPercent float64) {
	switch level {
	case 1:
		return c.TP1Percent, c.TP1SellPercent
	case 2:
		return c.TP2Percent, c.TP2SellPercent
	case 3:
		return c.TP3Percent, c.TP3SellPercent
	default:
		return 0, 0
	}
}
