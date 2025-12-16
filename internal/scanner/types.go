package scanner

import "time"

// ProximityResult represents how close a symbol is to triggering a strategy
type ProximityResult struct {
	Symbol           string              `json:"symbol"`
	StrategyName     string              `json:"strategy_name"`
	CurrentPrice     float64             `json:"current_price"`
	TargetPrice      float64             `json:"target_price"`
	DistancePercent  float64             `json:"distance_percent"`
	DistanceAbsolute float64             `json:"distance_absolute"`
	ReadinessScore   float64             `json:"readiness_score"` // 0-100
	TrendDirection   string              `json:"trend_direction"` // BULLISH, BEARISH, NEUTRAL
	Conditions       ConditionsChecklist `json:"conditions"`
	TimePrediction   *TimePrediction     `json:"time_prediction,omitempty"`
	LastEvaluated    time.Time           `json:"last_evaluated"`
	Timestamp        time.Time           `json:"timestamp"`
}

// ConditionsChecklist tracks which conditions are met/failed
type ConditionsChecklist struct {
	TotalConditions  int               `json:"total_conditions"`
	MetConditions    int               `json:"met_conditions"`
	FailedConditions int               `json:"failed_conditions"`
	Details          []ConditionDetail `json:"details"`
}

// ConditionDetail represents a single condition's state
type ConditionDetail struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Met         bool        `json:"met"`
	Value       interface{} `json:"value,omitempty"`
	Target      interface{} `json:"target,omitempty"`
	Distance    interface{} `json:"distance,omitempty"`
}

// TimePrediction estimates when signal might trigger
type TimePrediction struct {
	MinMinutes int     `json:"min_minutes"`
	MaxMinutes int     `json:"max_minutes"`
	Confidence float64 `json:"confidence"` // 0-1
	BasedOn    string  `json:"based_on"`   // "price_velocity", "volume_trend"
}

// ScanResult aggregates all proximity results from a scan
type ScanResult struct {
	ScanID         string             `json:"scan_id"`
	StartTime      time.Time          `json:"start_time"`
	EndTime        time.Time          `json:"end_time"`
	Duration       time.Duration      `json:"duration"`
	SymbolsScanned int                `json:"symbols_scanned"`
	Results        []ProximityResult  `json:"results"`
}

// ScannerConfig holds scanner configuration
type ScannerConfig struct {
	Enabled          bool
	ScanInterval     time.Duration
	MaxSymbols       int
	IncludeWatchlist bool
	CacheTTL         time.Duration
	WorkerCount      int
}

// CachedProximity stores proximity results with TTL
type CachedProximity struct {
	Result    *ProximityResult
	ExpiresAt time.Time
}
