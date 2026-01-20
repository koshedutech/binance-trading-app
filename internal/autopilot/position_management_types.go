package autopilot

// PositionManagementConfig holds all position management settings
// This controls how positions are monitored and decisions are made (classic vs new engine)
type PositionManagementConfig struct {
	DecisionMode   string                         `json:"decision_mode"`    // "classic" or "new_engine"
	Classic        ClassicDecisionSettingsBasic   `json:"classic"`          // Classic decision engine settings
	NewEngine      NewEngineDecisionSettingsBasic `json:"new_engine"`       // New strategy-based engine settings
	EfficiencyExit EfficiencyExitSettings         `json:"efficiency_exit"`  // Efficiency-based exit settings
	DynamicSLTP    DynamicSLTPSettings            `json:"dynamic_sltp"`     // Dynamic SL/TP adjustment settings
}

// ClassicDecisionSettingsBasic holds settings for the classic position decision engine
// Uses traditional technical indicators (ADX, EMA, RSI) for reversal detection
// Note: The detailed ClassicDecisionSettings struct is in position_exit_classic.go
type ClassicDecisionSettingsBasic struct {
	ADXReversalThreshold  float64 `json:"adx_reversal_threshold"`  // ADX threshold for trend weakness (default: 20)
	EMAReversalPeriods    []int   `json:"ema_reversal_periods"`    // EMA periods for reversal signals (default: [9, 21])
	RSIOverbought         float64 `json:"rsi_overbought"`          // RSI overbought threshold (default: 70)
	RSIOversold           float64 `json:"rsi_oversold"`            // RSI oversold threshold (default: 30)
	ReversalConfirmations int     `json:"reversal_confirmations"`  // Required confirmations before reversal signal (default: 2)
}

// NewEngineDecisionSettingsBasic holds settings for the new strategy-based position decision engine
// Integrates with the Entry Decision Engine (Story 10.1) for consistent strategy application
// Note: The detailed NewEngineDecisionSettings struct is in position_exit_engine.go
type NewEngineDecisionSettingsBasic struct {
	UseActiveStrategy    bool   `json:"use_active_strategy"`     // Use the currently active strategy for exit decisions
	StrategyName         string `json:"strategy_name"`           // Specific strategy name (empty = use active)
	UseStrategyExitRules bool   `json:"use_strategy_exit_rules"` // Apply strategy's exit rules
	ExitOnRegimeChange   bool   `json:"exit_on_regime_change"`   // Exit when market regime changes
}

// EfficiencyExitSettings holds settings for efficiency-based position exits
// Analyzes historical performance to determine optimal exit timing
type EfficiencyExitSettings struct {
	Enabled                    bool `json:"enabled"`                      // Enable efficiency-based exits
	HistoricalWindowHours      int  `json:"historical_window_hours"`      // Hours of history to analyze (default: 6)
	MinimumHoldMinutes         int  `json:"minimum_hold_minutes"`         // Minimum hold time before efficiency exit (default: 2)
	ConsecutiveSignalsRequired int  `json:"consecutive_signals_required"` // Consecutive exit signals needed (default: 3)
}

// DynamicSLTPSettings holds settings for dynamic stop-loss and take-profit adjustments
// Allows real-time SL/TP updates based on market conditions
type DynamicSLTPSettings struct {
	Enabled         bool    `json:"enabled"`            // Enable dynamic SL/TP adjustments
	SLATRMultiplier float64 `json:"sl_atr_multiplier"`  // ATR multiplier for stop-loss (default: 1.5)
	TPATRMultiplier float64 `json:"tp_atr_multiplier"`  // ATR multiplier for take-profit (default: 3.0)
	UpdateOnBinance bool    `json:"update_on_binance"`  // Push SL/TP updates to Binance orders
}

// DefaultPositionManagementConfig returns the default position management configuration
func DefaultPositionManagementConfig() *PositionManagementConfig {
	return &PositionManagementConfig{
		DecisionMode: "classic",
		Classic: ClassicDecisionSettingsBasic{
			ADXReversalThreshold:  20,
			EMAReversalPeriods:    []int{9, 21},
			RSIOverbought:         70,
			RSIOversold:           30,
			ReversalConfirmations: 2,
		},
		NewEngine: NewEngineDecisionSettingsBasic{
			UseActiveStrategy:    true,
			StrategyName:         "",
			UseStrategyExitRules: true,
			ExitOnRegimeChange:   true,
		},
		EfficiencyExit: EfficiencyExitSettings{
			Enabled:                    true,
			HistoricalWindowHours:      6,
			MinimumHoldMinutes:         2,
			ConsecutiveSignalsRequired: 3,
		},
		DynamicSLTP: DynamicSLTPSettings{
			Enabled:         true,
			SLATRMultiplier: 1.5,
			TPATRMultiplier: 3.0,
			UpdateOnBinance: true,
		},
	}
}
