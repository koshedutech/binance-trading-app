package cache

// Logger interface for dependency injection
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// SettingGroup defines a UI settings card that maps to a Redis key
type SettingGroup struct {
	Key         string   // Redis key suffix (e.g., "confidence")
	Name        string   // UI display name (e.g., "Confidence Settings")
	Prefixes    []string // Field prefixes to extract (e.g., ["confidence."])
	Description string   // What this group contains
}

// SettingGroups matches SETTING_GROUPS from SettingsComparisonView.tsx (20 groups)
var SettingGroups = []SettingGroup{
	{Key: "enabled", Name: "Mode Status", Prefixes: []string{"enabled"}, Description: "Whether mode is enabled"},
	{Key: "timeframe", Name: "Timeframe Settings", Prefixes: []string{"timeframe."}, Description: "Chart timeframes"},
	{Key: "confidence", Name: "Confidence Settings", Prefixes: []string{"confidence."}, Description: "Confidence thresholds"},
	{Key: "size", Name: "Size Settings", Prefixes: []string{"size."}, Description: "Position sizing"},
	{Key: "sltp", Name: "SL/TP Settings", Prefixes: []string{"sltp."}, Description: "Stop loss and take profit"},
	{Key: "risk", Name: "Risk Settings", Prefixes: []string{"risk."}, Description: "Risk parameters"},
	{Key: "circuit_breaker", Name: "Circuit Breaker", Prefixes: []string{"circuit_breaker."}, Description: "Per-mode limits"},
	{Key: "hedge", Name: "Hedge Settings", Prefixes: []string{"hedge."}, Description: "Hedge configuration"},
	{Key: "averaging", Name: "Position Averaging", Prefixes: []string{"averaging."}, Description: "DCA rules"},
	{Key: "stale_release", Name: "Stale Position Release", Prefixes: []string{"stale_release."}, Description: "Stale position handling"},
	{Key: "assignment", Name: "Mode Assignment", Prefixes: []string{"assignment."}, Description: "Mode selection criteria"},
	{Key: "mtf", Name: "Multi-Timeframe (MTF)", Prefixes: []string{"mtf."}, Description: "MTF analysis"},
	{Key: "dynamic_ai_exit", Name: "Dynamic AI Exit", Prefixes: []string{"dynamic_ai_exit."}, Description: "AI exit decisions"},
	{Key: "reversal", Name: "Reversal Entry", Prefixes: []string{"reversal."}, Description: "Reversal patterns"},
	{Key: "funding_rate", Name: "Funding Rate", Prefixes: []string{"funding_rate."}, Description: "Funding rate rules"},
	{Key: "trend_divergence", Name: "Trend Divergence", Prefixes: []string{"trend_divergence."}, Description: "Trend alignment"},
	{Key: "position_optimization", Name: "Position Optimization", Prefixes: []string{"position_optimization."}, Description: "Progressive TP, DCA"},
	{Key: "trend_filters", Name: "Trend Filters", Prefixes: []string{"trend_filters."}, Description: "BTC, EMA, VWAP filters"},
	{Key: "early_warning", Name: "Early Warning", Prefixes: []string{"early_warning."}, Description: "Early exit monitoring"},
	{Key: "entry", Name: "Entry Settings", Prefixes: []string{"entry."}, Description: "Entry configuration"},
}

// TradingModes defines the 4 trading modes
var TradingModes = []string{"ultra_fast", "scalp", "swing", "position"}

// CrossModeSettings defines the 4 cross-mode global setting types
var CrossModeSettings = []string{"circuit_breaker", "llm_config", "capital_allocation", "global_trading"}

// SafetySettingsModes defines the 4 modes that have safety settings (rate limits, profit monitor, win rate monitor)
var SafetySettingsModes = []string{"ultra_fast", "scalp", "swing", "position"}

// GetSettingGroupKeys returns just the keys for iteration
func GetSettingGroupKeys() []string {
	keys := make([]string, len(SettingGroups))
	for i, g := range SettingGroups {
		keys[i] = g.Key
	}
	return keys
}
