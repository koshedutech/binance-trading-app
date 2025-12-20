package autopilot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// AutopilotSettings holds persistent settings that survive restarts
type AutopilotSettings struct {
	// Dynamic SL/TP settings
	DynamicSLTPEnabled  bool    `json:"dynamic_sltp_enabled"`
	ATRPeriod           int     `json:"atr_period"`
	ATRMultiplierSL     float64 `json:"atr_multiplier_sl"`
	ATRMultiplierTP     float64 `json:"atr_multiplier_tp"`
	LLMSLTPWeight       float64 `json:"llm_sltp_weight"`
	MinSLPercent        float64 `json:"min_sl_percent"`
	MaxSLPercent        float64 `json:"max_sl_percent"`
	MinTPPercent        float64 `json:"min_tp_percent"`
	MaxTPPercent        float64 `json:"max_tp_percent"`

	// Scalping mode settings
	ScalpingModeEnabled     bool    `json:"scalping_mode_enabled"`
	ScalpingMinProfit       float64 `json:"scalping_min_profit"`
	ScalpingQuickReentry    bool    `json:"scalping_quick_reentry"`
	ScalpingReentryDelaySec int     `json:"scalping_reentry_delay_sec"`
	ScalpingMaxTradesPerDay int     `json:"scalping_max_trades_per_day"`

	// Circuit breaker settings
	CircuitBreakerEnabled    bool    `json:"circuit_breaker_enabled"`
	MaxLossPerHour           float64 `json:"max_loss_per_hour"`
	MaxDailyLoss             float64 `json:"max_daily_loss"`
	MaxConsecutiveLosses     int     `json:"max_consecutive_losses"`
	CooldownMinutes          int     `json:"cooldown_minutes"`
	MaxTradesPerMinute       int     `json:"max_trades_per_minute"`
	MaxDailyTrades           int     `json:"max_daily_trades"`
}

// SettingsManager handles persistent settings storage
type SettingsManager struct {
	settingsPath string
	mu           sync.RWMutex
}

var (
	settingsManager *SettingsManager
	managerOnce     sync.Once
)

// GetSettingsManager returns the singleton settings manager
func GetSettingsManager() *SettingsManager {
	managerOnce.Do(func() {
		// Use a settings file in the config directory
		settingsPath := getSettingsFilePath()
		settingsManager = &SettingsManager{
			settingsPath: settingsPath,
		}
	})
	return settingsManager
}

// getSettingsFilePath returns the path to the settings file
func getSettingsFilePath() string {
	// Try current directory first
	settingsPath := "autopilot_settings.json"

	// Check if we're running from a specific directory
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		settingsPath = filepath.Join(execDir, "autopilot_settings.json")
	}

	return settingsPath
}

// DefaultSettings returns the default settings
func DefaultSettings() *AutopilotSettings {
	return &AutopilotSettings{
		// Dynamic SL/TP defaults
		DynamicSLTPEnabled:  false,
		ATRPeriod:           14,
		ATRMultiplierSL:     1.5,
		ATRMultiplierTP:     2.0,
		LLMSLTPWeight:       0.3,
		MinSLPercent:        0.3,
		MaxSLPercent:        3.0,
		MinTPPercent:        0.5,
		MaxTPPercent:        5.0,

		// Scalping defaults
		ScalpingModeEnabled:     false,
		ScalpingMinProfit:       0.2,
		ScalpingQuickReentry:    false,
		ScalpingReentryDelaySec: 5,
		ScalpingMaxTradesPerDay: 0,

		// Circuit breaker defaults
		CircuitBreakerEnabled:    true,
		MaxLossPerHour:           100,
		MaxDailyLoss:             500,
		MaxConsecutiveLosses:     5,
		CooldownMinutes:          30,
		MaxTradesPerMinute:       10,
		MaxDailyTrades:           100,
	}
}

// LoadSettings loads settings from file, returns defaults if file doesn't exist
func (sm *SettingsManager) LoadSettings() (*AutopilotSettings, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	settings := DefaultSettings()

	data, err := os.ReadFile(sm.settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return defaults
			return settings, nil
		}
		return settings, err
	}

	if err := json.Unmarshal(data, settings); err != nil {
		return DefaultSettings(), err
	}

	return settings, nil
}

// SaveSettings saves settings to file
func (sm *SettingsManager) SaveSettings(settings *AutopilotSettings) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sm.settingsPath, data, 0644)
}

// GetCurrentSettings gets current settings (loading from file if needed)
func (sm *SettingsManager) GetCurrentSettings() *AutopilotSettings {
	settings, _ := sm.LoadSettings()
	return settings
}

// UpdateDynamicSLTP updates dynamic SL/TP settings and saves to file
func (sm *SettingsManager) UpdateDynamicSLTP(
	enabled bool,
	atrPeriod int,
	atrMultiplierSL float64,
	atrMultiplierTP float64,
	llmWeight float64,
	minSL float64,
	maxSL float64,
	minTP float64,
	maxTP float64,
) error {
	settings := sm.GetCurrentSettings()

	settings.DynamicSLTPEnabled = enabled
	if atrPeriod > 0 {
		settings.ATRPeriod = atrPeriod
	}
	if atrMultiplierSL > 0 {
		settings.ATRMultiplierSL = atrMultiplierSL
	}
	if atrMultiplierTP > 0 {
		settings.ATRMultiplierTP = atrMultiplierTP
	}
	if llmWeight >= 0 && llmWeight <= 1 {
		settings.LLMSLTPWeight = llmWeight
	}
	if minSL > 0 {
		settings.MinSLPercent = minSL
	}
	if maxSL > 0 {
		settings.MaxSLPercent = maxSL
	}
	if minTP > 0 {
		settings.MinTPPercent = minTP
	}
	if maxTP > 0 {
		settings.MaxTPPercent = maxTP
	}

	return sm.SaveSettings(settings)
}

// UpdateScalping updates scalping settings and saves to file
func (sm *SettingsManager) UpdateScalping(
	enabled bool,
	minProfit float64,
	quickReentry bool,
	reentryDelaySec int,
	maxTradesPerDay int,
) error {
	settings := sm.GetCurrentSettings()

	settings.ScalpingModeEnabled = enabled
	if minProfit > 0 {
		settings.ScalpingMinProfit = minProfit
	}
	settings.ScalpingQuickReentry = quickReentry
	if reentryDelaySec > 0 {
		settings.ScalpingReentryDelaySec = reentryDelaySec
	}
	if maxTradesPerDay >= 0 {
		settings.ScalpingMaxTradesPerDay = maxTradesPerDay
	}

	return sm.SaveSettings(settings)
}

// UpdateCircuitBreaker updates circuit breaker settings and saves to file
func (sm *SettingsManager) UpdateCircuitBreaker(
	enabled bool,
	maxLossPerHour float64,
	maxDailyLoss float64,
	maxConsecutiveLosses int,
	cooldownMinutes int,
	maxTradesPerMinute int,
	maxDailyTrades int,
) error {
	settings := sm.GetCurrentSettings()

	settings.CircuitBreakerEnabled = enabled
	if maxLossPerHour > 0 {
		settings.MaxLossPerHour = maxLossPerHour
	}
	if maxDailyLoss > 0 {
		settings.MaxDailyLoss = maxDailyLoss
	}
	if maxConsecutiveLosses > 0 {
		settings.MaxConsecutiveLosses = maxConsecutiveLosses
	}
	if cooldownMinutes > 0 {
		settings.CooldownMinutes = cooldownMinutes
	}
	if maxTradesPerMinute > 0 {
		settings.MaxTradesPerMinute = maxTradesPerMinute
	}
	if maxDailyTrades > 0 {
		settings.MaxDailyTrades = maxDailyTrades
	}

	return sm.SaveSettings(settings)
}

// ResetToDefaults resets all settings to defaults and saves to file
func (sm *SettingsManager) ResetToDefaults() error {
	return sm.SaveSettings(DefaultSettings())
}
