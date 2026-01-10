package api

import (
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/database"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
)

// ====== LOAD DEFAULTS HANDLERS (Story 4.14) ======
// These handlers allow users to:
// 1. Preview differences between their settings and defaults
// 2. Load default settings for specific modes or all modes
// 3. Compare user settings vs defaults with risk impact analysis

// FieldComparison represents a comparison between user and default value for any field
type FieldComparison struct {
	Path      string      `json:"path"`                // e.g., "confidence.min_confidence"
	Current   interface{} `json:"current"`             // User's current value
	Default   interface{} `json:"default"`             // Default value
	Match     bool        `json:"match"`               // true if current == default
	RiskLevel string      `json:"risk_level,omitempty"` // "high", "medium", "low" (only if different)
}

// SettingDifference represents a single difference between user and default settings
type SettingDifference struct {
	Path           string      `json:"path"`            // e.g., "confidence.min_confidence"
	Current        interface{} `json:"current"`         // User's current value
	Default        interface{} `json:"default"`         // Default value
	RiskLevel      string      `json:"risk_level"`      // "high", "medium", "low"
	Impact         string      `json:"impact"`          // Human-readable impact description
	Recommendation string      `json:"recommendation"`  // Suggested action
}

// ModeDiffResponse represents the diff response for a specific mode
type ModeDiffResponse struct {
	Preview       bool                `json:"preview"`        // true if preview mode
	Mode          string              `json:"mode"`           // Mode name
	TotalChanges  int                 `json:"total_changes"`  // Count of differences
	AllMatch      bool                `json:"all_match"`      // true if settings match defaults
	Differences   []SettingDifference `json:"differences"`    // List of differences (only fields that differ)
	AllValues     []FieldComparison   `json:"all_values"`     // ALL fields with current vs default comparison
	AppliedAt     string              `json:"applied_at,omitempty"` // Timestamp if applied
}

// AllModesDiffResponse represents the diff response for all modes
type AllModesDiffResponse struct {
	Preview      bool                        `json:"preview"`       // true if preview mode
	TotalChanges int                         `json:"total_changes"` // Total across all modes
	AllMatch     bool                        `json:"all_match"`     // true if all settings match
	Modes        map[string]*ModeDiffResponse `json:"modes"`        // Per-mode diffs
	AppliedAt    string                      `json:"applied_at,omitempty"` // Timestamp if applied
}

// handleLoadModeDefaults loads default settings for a specific mode
// POST /api/futures/ginie/modes/:mode/load-defaults?preview=true
func (s *Server) handleLoadModeDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	mode := c.Param("mode")
	if mode == "" {
		errorResponse(c, http.StatusBadRequest, "Mode is required")
		return
	}

	// Validate mode name
	validModes := []string{"ultra_fast", "scalp", "scalp_reentry", "swing", "position"}
	isValid := false
	for _, m := range validModes {
		if m == mode {
			isValid = true
			break
		}
	}
	if !isValid {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s. Valid modes: ultra_fast, scalp, scalp_reentry, swing, position", mode))
		return
	}

	preview := c.Query("preview") == "true"

	// Get user's current settings FROM DATABASE (not JSON file!)
	sm := autopilot.GetSettingsManager()
	ctx := c.Request.Context()
	currentMode, err := sm.GetUserModeConfigFromDB(ctx, s.repo, userID, mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings from database: "+err.Error())
		return
	}
	if currentMode == nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Mode config not found in database: %s", mode))
		return
	}

	// Get default mode config from default-settings.json
	defaultMode, err := autopilot.GetDefaultModeFullConfig(mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	// Compare and generate diff
	diff := compareModeConfigs(mode, currentMode, defaultMode)

	// If not preview mode, apply the defaults to user's database
	if !preview {
		// Serialize default config to JSON for database storage
		configJSON, marshalErr := json.Marshal(defaultMode)
		if marshalErr != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to serialize default config: "+marshalErr.Error())
			return
		}

		// Save defaults to user's database record
		if saveErr := s.repo.SaveUserModeConfig(ctx, userID, mode, defaultMode.Enabled, configJSON); saveErr != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to save settings to database: "+saveErr.Error())
			return
		}

		// Trigger immediate config reload in running autopilot
		if s.userAutopilotManager != nil {
			instance := s.userAutopilotManager.GetInstance(userID)
			if instance != nil && instance.Autopilot != nil {
				instance.Autopilot.TriggerConfigReload()
				log.Printf("[DEFAULTS-RESET] Triggered immediate config reload for user %s", userID)
			}
		}

		// Return success response
		c.JSON(http.StatusOK, ConfigResetResult{
			Success:        true,
			ConfigType:     mode,
			ChangesApplied: diff.TotalChanges,
			Message:        fmt.Sprintf("%s mode reset to defaults (%d changes applied)", mode, diff.TotalChanges),
		})
		return
	}

	c.JSON(http.StatusOK, diff)
}

// handleLoadAllModeDefaults loads default settings for all modes
// POST /api/futures/ginie/modes/load-defaults?preview=true
func (s *Server) handleLoadAllModeDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	sm := autopilot.GetSettingsManager()

	// Get all default mode configs from default-settings.json
	defaultModes, err := autopilot.GetAllDefaultModeFullConfigs()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	// Compare all modes - get current user configs from DATABASE
	response := &AllModesDiffResponse{
		Preview: preview,
		Modes:   make(map[string]*ModeDiffResponse),
	}

	modeNames := []string{"ultra_fast", "scalp", "scalp_reentry", "swing", "position"}
	totalChanges := 0
	allMatch := true

	for _, modeName := range modeNames {
		// Get user's current mode config from DATABASE
		currentMode, dbErr := sm.GetUserModeConfigFromDB(ctx, s.repo, userID, modeName)
		if dbErr != nil {
			// Log but continue - user might not have all modes configured
			continue
		}

		defaultMode := defaultModes[modeName]
		if currentMode != nil && defaultMode != nil {
			diff := compareModeConfigs(modeName, currentMode, defaultMode)
			response.Modes[modeName] = diff
			totalChanges += diff.TotalChanges
			if !diff.AllMatch {
				allMatch = false
			}
		}
	}

	response.TotalChanges = totalChanges
	response.AllMatch = allMatch

	// If not preview mode, apply all defaults to user's database
	if !preview {
		modesApplied := 0
		for _, modeName := range modeNames {
			defaultMode := defaultModes[modeName]
			if defaultMode == nil {
				continue
			}

			// Serialize default config to JSON for database storage
			configJSON, marshalErr := json.Marshal(defaultMode)
			if marshalErr != nil {
				errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to serialize %s config: %v", modeName, marshalErr))
				return
			}

			// Save defaults to user's database record
			if saveErr := s.repo.SaveUserModeConfig(ctx, userID, modeName, defaultMode.Enabled, configJSON); saveErr != nil {
				errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to save %s settings: %v", modeName, saveErr))
				return
			}
			modesApplied++
		}

		// Trigger immediate config reload in running autopilot
		if s.userAutopilotManager != nil {
			instance := s.userAutopilotManager.GetInstance(userID)
			if instance != nil && instance.Autopilot != nil {
				instance.Autopilot.TriggerConfigReload()
				log.Printf("[DEFAULTS-RESET] Triggered immediate config reload for all modes for user %s", userID)
			}
		}

		// Return success response
		c.JSON(http.StatusOK, ConfigResetResult{
			Success:        true,
			ConfigType:     "all_modes",
			ChangesApplied: totalChanges,
			Message:        fmt.Sprintf("All mode settings reset to defaults (%d modes, %d changes applied)", modesApplied, totalChanges),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// handleGetModeDiff returns differences between user settings and defaults for a mode
// GET /api/user/settings/diff/modes/:mode
func (s *Server) handleGetModeDiff(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	mode := c.Param("mode")
	if mode == "" {
		errorResponse(c, http.StatusBadRequest, "Mode is required")
		return
	}

	// Validate mode name
	validModes := []string{"ultra_fast", "scalp", "scalp_reentry", "swing", "position"}
	isValid := false
	for _, m := range validModes {
		if m == mode {
			isValid = true
			break
		}
	}
	if !isValid {
		errorResponse(c, http.StatusBadRequest, fmt.Sprintf("Invalid mode: %s", mode))
		return
	}

	// Get user's current settings FROM DATABASE (not JSON file!)
	sm := autopilot.GetSettingsManager()
	ctx := c.Request.Context()
	currentMode, err := sm.GetUserModeConfigFromDB(ctx, s.repo, userID, mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings from database: "+err.Error())
		return
	}
	if currentMode == nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Mode config not found in database: %s", mode))
		return
	}

	// Get default mode config from default-settings.json
	defaultMode, err := autopilot.GetDefaultModeFullConfig(mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	// Compare and generate diff
	diff := compareModeConfigs(mode, currentMode, defaultMode)
	diff.Preview = true // This is read-only, always preview

	c.JSON(http.StatusOK, diff)
}

// handleLoadAllDefaults loads all default settings (global + all modes)
// POST /api/user/settings/load-defaults
func (s *Server) handleLoadAllDefaults(c *gin.Context) {
	_, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"

	// Load complete default settings file
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	// Get user's current settings
	sm := autopilot.GetSettingsManager()
	currentSettings := sm.GetDefaultSettings()
	if currentSettings == nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings")
		return
	}

	// Compare all settings
	response := &AllModesDiffResponse{
		Preview: preview,
		Modes:   make(map[string]*ModeDiffResponse),
	}

	// Compare mode configs
	modes := []struct {
		name    string
		current *autopilot.ModeFullConfig
		def     *autopilot.ModeFullConfig
	}{
		{"ultra_fast", currentSettings.ModeConfigs["ultra_fast"], defaults.ModeConfigs["ultra_fast"]},
		{"scalp", currentSettings.ModeConfigs["scalp"], defaults.ModeConfigs["scalp"]},
		{"scalp_reentry", currentSettings.ModeConfigs["scalp_reentry"], defaults.ModeConfigs["scalp_reentry"]},
		{"swing", currentSettings.ModeConfigs["swing"], defaults.ModeConfigs["swing"]},
		{"position", currentSettings.ModeConfigs["position"], defaults.ModeConfigs["position"]},
	}

	totalChanges := 0
	allMatch := true

	for _, m := range modes {
		if m.current != nil && m.def != nil {
			diff := compareModeConfigs(m.name, m.current, m.def)
			response.Modes[m.name] = diff
			totalChanges += diff.TotalChanges
			if !diff.AllMatch {
				allMatch = false
			}
		}
	}

	response.TotalChanges = totalChanges
	response.AllMatch = allMatch

	// If not preview mode, apply all defaults
	if !preview {
		// Apply mode configs
		currentSettings.ModeConfigs["ultra_fast"] = defaults.ModeConfigs["ultra_fast"]
		currentSettings.ModeConfigs["scalp"] = defaults.ModeConfigs["scalp"]
		currentSettings.ModeConfigs["scalp_reentry"] = defaults.ModeConfigs["scalp_reentry"]
		currentSettings.ModeConfigs["swing"] = defaults.ModeConfigs["swing"]
		currentSettings.ModeConfigs["position"] = defaults.ModeConfigs["position"]

		// TODO: Also apply global settings, position optimization, circuit breaker, etc.
		// For Story 4.14, we focus on mode configs only

		// Save settings to database
		if err := sm.SaveSettings(currentSettings); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to save settings: "+err.Error())
			return
		}

		// Note: User-specific autopilot instances will reload config on next cycle
		// No need to force reload as settings are saved to database

		response.AppliedAt = currentTime()
	}

	c.JSON(http.StatusOK, response)
}

// ====== HELPER FUNCTIONS ======

// compareModeConfigs compares a user's mode config with the default and returns differences
func compareModeConfigs(modeName string, current, defaultCfg *autopilot.ModeFullConfig) *ModeDiffResponse {
	response := &ModeDiffResponse{
		Mode:        modeName,
		Preview:     true,
		Differences: []SettingDifference{},
		AllValues:   []FieldComparison{},
		AllMatch:    true,
	}

	log.Printf("[DIFF-DEBUG] Comparing mode: %s", modeName)
	log.Printf("[DIFF-DEBUG] Current ptr: %p, Default ptr: %p", current, defaultCfg)
	log.Printf("[DIFF-DEBUG] Current.Enabled: %v, Default.Enabled: %v", current.Enabled, defaultCfg.Enabled)
	log.Printf("[DIFF-DEBUG] Current.ModeName: %s, Default.ModeName: %s", current.ModeName, defaultCfg.ModeName)

	// Compare each section using reflection
	compareSections("confidence", current.Confidence, defaultCfg.Confidence, response)
	compareSections("size", current.Size, defaultCfg.Size, response)
	compareSections("sltp", current.SLTP, defaultCfg.SLTP, response)
	compareSections("circuit_breaker", current.CircuitBreaker, defaultCfg.CircuitBreaker, response)
	compareSections("timeframe", current.Timeframe, defaultCfg.Timeframe, response)
	compareSections("averaging", current.Averaging, defaultCfg.Averaging, response)
	compareSections("hedge", current.Hedge, defaultCfg.Hedge, response)
	compareSections("risk", current.Risk, defaultCfg.Risk, response)
	compareSections("stale_release", current.StaleRelease, defaultCfg.StaleRelease, response)
	compareSections("assignment", current.Assignment, defaultCfg.Assignment, response)
	compareSections("mtf", current.MTF, defaultCfg.MTF, response)
	compareSections("dynamic_ai_exit", current.DynamicAIExit, defaultCfg.DynamicAIExit, response)
	compareSections("reversal", current.Reversal, defaultCfg.Reversal, response)
	compareSections("funding_rate", current.FundingRate, defaultCfg.FundingRate, response)
	compareSections("trend_divergence", current.TrendDivergence, defaultCfg.TrendDivergence, response)

	// Compare enabled flag
	enabledMatch := current.Enabled == defaultCfg.Enabled

	// Add to AllValues
	enabledFieldComp := FieldComparison{
		Path:    "enabled",
		Current: current.Enabled,
		Default: defaultCfg.Enabled,
		Match:   enabledMatch,
	}
	if !enabledMatch {
		enabledFieldComp.RiskLevel = "high"
	}
	response.AllValues = append(response.AllValues, enabledFieldComp)

	// Add to Differences if different
	if !enabledMatch {
		response.Differences = append(response.Differences, SettingDifference{
			Path:           "enabled",
			Current:        current.Enabled,
			Default:        defaultCfg.Enabled,
			RiskLevel:      "high",
			Impact:         "Mode disabled = no trades in this mode",
			Recommendation: "Enable mode to allow trading",
		})
		response.AllMatch = false
	}

	response.TotalChanges = len(response.Differences)
	if response.TotalChanges > 0 {
		response.AllMatch = false
	}

	log.Printf("[DIFF-DEBUG] Mode %s: TotalChanges=%d, AllMatch=%v", modeName, response.TotalChanges, response.AllMatch)
	if response.TotalChanges > 0 {
		log.Printf("[DIFF-DEBUG] First 3 differences:")
		for i := 0; i < response.TotalChanges && i < 3; i++ {
			log.Printf("[DIFF-DEBUG]   %d. %s: current=%v, default=%v", i+1, response.Differences[i].Path, response.Differences[i].Current, response.Differences[i].Default)
		}
	}

	return response
}

// compareSections compares two structs field by field using reflection
func compareSections(sectionName string, current, defaultCfg interface{}, response *ModeDiffResponse) {
	// Use reflection to properly check for nil pointers wrapped in interface{}
	// In Go, a nil pointer passed to interface{} is NOT a nil interface
	currentVal := reflect.ValueOf(current)
	defaultVal := reflect.ValueOf(defaultCfg)

	currentIsNil := current == nil || (currentVal.Kind() == reflect.Ptr && currentVal.IsNil())
	defaultIsNil := defaultCfg == nil || (defaultVal.Kind() == reflect.Ptr && defaultVal.IsNil())

	// If both are nil, no differences
	if currentIsNil && defaultIsNil {
		return
	}

	// CRITICAL FIX: If user's config is nil but default is not, that's a difference!
	// User has empty/missing config, so ALL default fields should be shown
	if currentIsNil && !defaultIsNil {
		// User config is nil/empty - show entire default section as difference
		addEntireSectionAsDifference(sectionName, defaultCfg, response)
		return
	}

	// If default is nil but current is not, skip (keep user's custom values)
	if !currentIsNil && defaultIsNil {
		return
	}

	// Dereference pointers
	if currentVal.Kind() == reflect.Ptr {
		currentVal = currentVal.Elem()
	}
	if defaultVal.Kind() == reflect.Ptr {
		defaultVal = defaultVal.Elem()
	}

	// Only compare structs
	if currentVal.Kind() != reflect.Struct || defaultVal.Kind() != reflect.Struct {
		return
	}

	// Compare each field
	for i := 0; i < currentVal.NumField(); i++ {
		field := currentVal.Type().Field(i)
		currentFieldVal := currentVal.Field(i)
		defaultFieldVal := defaultVal.Field(i)

		// Get JSON tag for field name
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			jsonTag = field.Name
		}

		path := fmt.Sprintf("%s.%s", sectionName, jsonTag)

		// Compare values
		currentInterface := currentFieldVal.Interface()
		defaultInterface := defaultFieldVal.Interface()
		areEqual := reflect.DeepEqual(currentInterface, defaultInterface)

		// DEBUG: Log the comparison for the first field of each section
		if i == 0 {
			log.Printf("[DIFF-DEBUG] Section %s, field %s: equal=%v, current=%v, default=%v",
				sectionName, jsonTag, areEqual, currentInterface, defaultInterface)
		}

		// Always add to AllValues array (shows ALL fields)
		fieldComp := FieldComparison{
			Path:    path,
			Current: currentInterface,
			Default: defaultInterface,
			Match:   areEqual,
		}

		// If values differ, add risk level
		if !areEqual {
			riskLevel := getRiskLevel(sectionName, jsonTag)
			fieldComp.RiskLevel = riskLevel
		}

		response.AllValues = append(response.AllValues, fieldComp)

		// Only add to Differences if values are different
		if !areEqual {
			diff := SettingDifference{
				Path:    path,
				Current: currentInterface,
				Default: defaultInterface,
			}

			// Assign risk level and recommendations based on field
			assignRiskInfo(&diff, sectionName, jsonTag, currentInterface, defaultInterface)

			response.Differences = append(response.Differences, diff)
			response.AllMatch = false
		}
	}
}

// addEntireSectionAsDifference adds all fields from a default section as differences
// when the user's config is nil/empty
func addEntireSectionAsDifference(sectionName string, defaultCfg interface{}, response *ModeDiffResponse) {
	if defaultCfg == nil {
		return
	}

	defaultVal := reflect.ValueOf(defaultCfg)

	// Dereference pointers
	if defaultVal.Kind() == reflect.Ptr {
		defaultVal = defaultVal.Elem()
	}

	// Only process structs
	if defaultVal.Kind() != reflect.Struct {
		return
	}

	// Add each field as a difference AND to AllValues
	for i := 0; i < defaultVal.NumField(); i++ {
		field := defaultVal.Type().Field(i)
		defaultFieldVal := defaultVal.Field(i)

		// Get JSON tag for field name
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			jsonTag = field.Name
		}

		path := fmt.Sprintf("%s.%s", sectionName, jsonTag)
		defaultInterface := defaultFieldVal.Interface()

		// Add to AllValues (shows ALL fields)
		riskLevel := getRiskLevel(sectionName, jsonTag)
		response.AllValues = append(response.AllValues, FieldComparison{
			Path:      path,
			Current:   nil, // User has no value (nil/empty)
			Default:   defaultInterface,
			Match:     false, // nil != default
			RiskLevel: riskLevel,
		})

		// Add to Differences (user has nil vs default)
		diff := SettingDifference{
			Path:    path,
			Current: nil, // User has no value (nil/empty)
			Default: defaultInterface,
		}

		// Assign risk level and recommendations based on field
		assignRiskInfo(&diff, sectionName, jsonTag, nil, defaultInterface)

		response.Differences = append(response.Differences, diff)
	}

	response.AllMatch = false
}

// getRiskLevel returns the risk level for a field based on section and field name
func getRiskLevel(section, field string) string {
	// High-risk settings
	switch section {
	case "confidence":
		return "high"
	case "size":
		if field == "max_size_usd" || field == "base_size_usd" || field == "leverage" {
			return "high"
		}
		return "medium"
	case "sltp":
		return "medium"
	case "circuit_breaker":
		if field == "max_loss_per_hour" || field == "max_loss_per_day" {
			return "high"
		}
		return "medium"
	case "averaging":
		if field == "max_entries" {
			return "medium"
		}
		return "low"
	case "hedge":
		if field == "allow_hedge" || field == "max_hedge_size_percent" || field == "max_total_exposure_multiplier" {
			return "high"
		}
		return "medium"
	default:
		return "low"
	}
}

// assignRiskInfo assigns risk level, impact, and recommendation for a setting difference
func assignRiskInfo(diff *SettingDifference, section, field string, current, defaultVal interface{}) {
	// Default values
	diff.RiskLevel = "medium"
	diff.Impact = "May affect trading performance"
	diff.Recommendation = "Consider using default value"

	// High-risk settings
	switch section {
	case "confidence":
		diff.RiskLevel = "high"
		if field == "min_confidence" {
			diff.Impact = fmt.Sprintf("Lower confidence = more trades. Current: %.0f%%, Default: %.0f%%", current, defaultVal)
			diff.Recommendation = "Keep at default or higher for safer trading"
		}
	case "size":
		diff.RiskLevel = "high"
		if field == "max_size_usd" || field == "base_size_usd" {
			diff.Impact = "Affects capital at risk per position"
			diff.Recommendation = "Use conservative default for safer position sizing"
		}
		if field == "leverage" {
			diff.Impact = "Higher leverage = higher risk"
			diff.Recommendation = "Use default leverage for balanced risk"
		}
	case "sltp":
		diff.RiskLevel = "medium"
		if field == "stop_loss_percent" {
			diff.Impact = "Wider SL = larger potential losses"
			diff.Recommendation = "Use tighter default SL for risk management"
		}
		if field == "take_profit_percent" {
			diff.Impact = "Higher TP = longer hold time, may miss profits"
			diff.Recommendation = "Use default TP for optimal profit capture"
		}
	case "circuit_breaker":
		diff.RiskLevel = "high"
		if field == "max_loss_per_hour" || field == "max_loss_per_day" {
			diff.Impact = "Affects loss limits and safety"
			diff.Recommendation = "Use default for proper risk control"
		}
	case "averaging":
		diff.RiskLevel = "medium"
		if field == "max_entries" {
			diff.Impact = "More entries = more capital at risk"
			diff.Recommendation = "Limit averaging entries for safer DCA"
		}
	default:
		diff.RiskLevel = "low"
	}
}

// currentTime returns current timestamp in RFC3339 format
func currentTime() string {
	return time.Now().Format(time.RFC3339)
}

// ====== STORY 4.17: CONFIG RESET HANDLERS ======
// These handlers allow users to reset specific configuration sections to defaults
// with preview support (preview=true shows differences without applying)

// ConfigResetPreview represents the preview response when resetting config sections
type ConfigResetPreview struct {
	Preview      bool                `json:"preview"`
	ConfigType   string              `json:"config_type"`
	AllMatch     bool                `json:"all_match"`
	TotalChanges int                 `json:"total_changes"`
	Differences  []SettingDifference `json:"differences"` // Only fields that differ
	AllValues    []FieldComparison   `json:"all_values"`  // ALL fields with current vs default comparison
}

// ConfigResetResult represents the response after applying config reset
type ConfigResetResult struct {
	Success        bool   `json:"success"`
	ConfigType     string `json:"config_type"`
	ChangesApplied int    `json:"changes_applied"`
	Message        string `json:"message"`
}

// handleLoadCircuitBreakerDefaults resets circuit breaker to default values
// POST /api/futures/ginie/circuit-breaker/load-defaults?preview=true
func (s *Server) handleLoadCircuitBreakerDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()

	// Get user's CURRENT circuit breaker config from DATABASE
	currentCB, err := s.repo.GetUserGlobalCircuitBreaker(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load user circuit breaker: %v", err)})
		return
	}

	// If no config exists, use database defaults for comparison
	if currentCB == nil {
		currentCB = &database.UserGlobalCircuitBreaker{
			UserID:               userID,
			MaxLossPerHour:       100.0,
			MaxDailyLoss:         300.0,
			MaxConsecutiveLosses: 3,
			CooldownMinutes:      30,
		}
	}

	// Load defaults from default-settings.json
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load defaults: %v", err)})
		return
	}

	// Compare current vs defaults
	var differences []SettingDifference
	var allValues []FieldComparison

	// Helper to add field comparison
	addField := func(path string, current, defaultVal interface{}, riskLevel, impact, recommendation string) {
		match := reflect.DeepEqual(current, defaultVal)

		// Add to AllValues
		fc := FieldComparison{
			Path:    path,
			Current: current,
			Default: defaultVal,
			Match:   match,
		}
		if !match {
			fc.RiskLevel = riskLevel
		}
		allValues = append(allValues, fc)

		// Add to Differences only if different
		if !match {
			differences = append(differences, SettingDifference{
				Path:           path,
				Current:        current,
				Default:        defaultVal,
				RiskLevel:      riskLevel,
				Impact:         impact,
				Recommendation: recommendation,
			})
		}
	}

	addField("circuit_breaker.max_loss_per_hour",
		currentCB.MaxLossPerHour, defaults.CircuitBreaker.Global.MaxLossPerHour,
		"high", fmt.Sprintf("Current hourly loss limit: $%.2f, Default: $%.2f", currentCB.MaxLossPerHour, defaults.CircuitBreaker.Global.MaxLossPerHour),
		"Use default for optimal risk control")

	addField("circuit_breaker.max_daily_loss",
		currentCB.MaxDailyLoss, defaults.CircuitBreaker.Global.MaxDailyLoss,
		"high", fmt.Sprintf("Current daily loss limit: $%.2f, Default: $%.2f", currentCB.MaxDailyLoss, defaults.CircuitBreaker.Global.MaxDailyLoss),
		"Use default for optimal daily risk control")

	addField("circuit_breaker.max_consecutive_losses",
		currentCB.MaxConsecutiveLosses, defaults.CircuitBreaker.Global.MaxConsecutiveLosses,
		"medium", fmt.Sprintf("Current: %d consecutive losses, Default: %d", currentCB.MaxConsecutiveLosses, defaults.CircuitBreaker.Global.MaxConsecutiveLosses),
		"Use default to balance safety and opportunities")

	addField("circuit_breaker.cooldown_minutes",
		currentCB.CooldownMinutes, defaults.CircuitBreaker.Global.CooldownMinutes,
		"medium", fmt.Sprintf("Current cooldown: %d min, Default: %d min", currentCB.CooldownMinutes, defaults.CircuitBreaker.Global.CooldownMinutes),
		"Use default cooldown period")

	if preview {
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "circuit_breaker",
			AllMatch:     len(differences) == 0,
			TotalChanges: len(differences),
			Differences:  differences,
			AllValues:    allValues,
		})
		return
	}

	// Apply defaults to database
	// Note: Runtime state (IsTripped, etc.) is tracked separately in user_mode_circuit_breaker_stats table
	updatedCB := &database.UserGlobalCircuitBreaker{
		UserID:               userID,
		Enabled:              currentCB.Enabled, // Preserve enabled state
		MaxLossPerHour:       defaults.CircuitBreaker.Global.MaxLossPerHour,
		MaxDailyLoss:         defaults.CircuitBreaker.Global.MaxDailyLoss,
		MaxConsecutiveLosses: defaults.CircuitBreaker.Global.MaxConsecutiveLosses,
		CooldownMinutes:      defaults.CircuitBreaker.Global.CooldownMinutes,
		MaxTradesPerMinute:   defaults.CircuitBreaker.Global.MaxTradesPerMinute,
		MaxDailyTrades:       defaults.CircuitBreaker.Global.MaxDailyTrades,
	}

	if err := s.repo.SaveUserGlobalCircuitBreaker(ctx, updatedCB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save circuit breaker: %v", err)})
		return
	}

	// Trigger immediate config reload in running autopilot
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
			log.Printf("[DEFAULTS-RESET] Triggered immediate config reload for user %s circuit breaker", userID)
		}
	}

	c.JSON(http.StatusOK, ConfigResetResult{
		Success:        true,
		ConfigType:     "circuit_breaker",
		ChangesApplied: len(differences),
		Message:        fmt.Sprintf("Circuit breaker settings reset to defaults (%d changes applied)", len(differences)),
	})
}

// handleLoadLLMConfigDefaults resets LLM config to default values
// POST /api/futures/ginie/llm-config/load-defaults?preview=true
func (s *Server) handleLoadLLMConfigDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()

	// Get user's CURRENT LLM config from DATABASE
	currentLLM, err := s.repo.GetUserLLMConfig(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load user LLM config: %v", err)})
		return
	}

	// If no config exists, use database defaults for comparison
	if currentLLM == nil {
		currentLLM = database.DefaultUserLLMConfig()
		currentLLM.UserID = userID
	}

	// Load defaults from default-settings.json
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load defaults: %v", err)})
		return
	}

	// Compare current vs defaults
	var differences []SettingDifference
	var allValues []FieldComparison

	// Helper to add field comparison
	addField := func(path string, current, defaultVal interface{}, riskLevel, impact, recommendation string) {
		match := reflect.DeepEqual(current, defaultVal)

		// Add to AllValues
		fc := FieldComparison{
			Path:    path,
			Current: current,
			Default: defaultVal,
			Match:   match,
		}
		if !match {
			fc.RiskLevel = riskLevel
		}
		allValues = append(allValues, fc)

		// Add to Differences only if different
		if !match {
			differences = append(differences, SettingDifference{
				Path:           path,
				Current:        current,
				Default:        defaultVal,
				RiskLevel:      riskLevel,
				Impact:         impact,
				Recommendation: recommendation,
			})
		}
	}

	addField("llm_config.provider",
		currentLLM.Provider, defaults.LLMConfig.Global.Provider,
		"low", fmt.Sprintf("Current provider: %s, Default: %s", currentLLM.Provider, defaults.LLMConfig.Global.Provider),
		"Use default provider for optimal performance")

	addField("llm_config.model",
		currentLLM.Model, defaults.LLMConfig.Global.Model,
		"low", fmt.Sprintf("Current model: %s, Default: %s", currentLLM.Model, defaults.LLMConfig.Global.Model),
		"Use default model for optimal performance")

	addField("llm_config.timeout_ms",
		currentLLM.TimeoutMs, defaults.LLMConfig.Global.TimeoutMS,
		"low", fmt.Sprintf("Current timeout: %d ms, Default: %d ms", currentLLM.TimeoutMs, defaults.LLMConfig.Global.TimeoutMS),
		"Use default timeout for optimal performance")

	addField("llm_config.retry_count",
		currentLLM.RetryCount, defaults.LLMConfig.Global.RetryCount,
		"low", fmt.Sprintf("Current retry count: %d, Default: %d", currentLLM.RetryCount, defaults.LLMConfig.Global.RetryCount),
		"Use default retry count")

	if preview {
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "llm_config",
			AllMatch:     len(differences) == 0,
			TotalChanges: len(differences),
			Differences:  differences,
			AllValues:    allValues,
		})
		return
	}

	// Apply defaults to database
	updatedLLM := &database.UserLLMConfig{
		UserID:           userID,
		Enabled:          currentLLM.Enabled, // Preserve enabled state
		Provider:         defaults.LLMConfig.Global.Provider,
		Model:            defaults.LLMConfig.Global.Model,
		FallbackProvider: currentLLM.FallbackProvider, // Preserve fallback
		FallbackModel:    currentLLM.FallbackModel,    // Preserve fallback
		TimeoutMs:        defaults.LLMConfig.Global.TimeoutMS,
		RetryCount:       defaults.LLMConfig.Global.RetryCount,
		CacheDurationSec: defaults.LLMConfig.Global.CacheDurationSec,
	}

	if err := s.repo.SaveUserLLMConfig(ctx, updatedLLM); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save LLM config: %v", err)})
		return
	}

	// Trigger immediate config reload in running autopilot
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
			log.Printf("[DEFAULTS-RESET] Triggered immediate config reload for user %s LLM config", userID)
		}
	}

	c.JSON(http.StatusOK, ConfigResetResult{
		Success:        true,
		ConfigType:     "llm_config",
		ChangesApplied: len(differences),
		Message:        fmt.Sprintf("LLM config reset to defaults (%d changes applied)", len(differences)),
	})
}

// handleLoadCapitalAllocationDefaults resets capital allocation to default values
// POST /api/futures/ginie/capital-allocation/load-defaults?preview=true
func (s *Server) handleLoadCapitalAllocationDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()

	// Get user's CURRENT capital allocation from DATABASE
	currentAlloc, err := s.repo.GetUserCapitalAllocation(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load user capital allocation: %v", err)})
		return
	}

	// If no config exists, use database defaults for comparison
	if currentAlloc == nil {
		currentAlloc = database.DefaultUserCapitalAllocation()
		currentAlloc.UserID = userID
	}

	// Use database defaults (NOT from default-settings.json, as capital allocation is per-user in database)
	defaultAlloc := database.DefaultUserCapitalAllocation()
	defaultAlloc.UserID = userID

	// Compare current vs defaults
	var differences []SettingDifference
	var allValues []FieldComparison

	// Helper to add field comparison
	addField := func(path string, current, defaultVal interface{}, riskLevel, impact, recommendation string) {
		match := reflect.DeepEqual(current, defaultVal)

		// Add to AllValues
		fc := FieldComparison{
			Path:    path,
			Current: current,
			Default: defaultVal,
			Match:   match,
		}
		if !match {
			fc.RiskLevel = riskLevel
		}
		allValues = append(allValues, fc)

		// Add to Differences only if different
		if !match {
			differences = append(differences, SettingDifference{
				Path:           path,
				Current:        current,
				Default:        defaultVal,
				RiskLevel:      riskLevel,
				Impact:         impact,
				Recommendation: recommendation,
			})
		}
	}

	// Capital Allocation Percentages
	addField("capital_allocation.ultra_fast_percent",
		currentAlloc.UltraFastPercent, defaultAlloc.UltraFastPercent,
		"medium", fmt.Sprintf("Current: %.1f%%, Default: %.1f%%", currentAlloc.UltraFastPercent, defaultAlloc.UltraFastPercent),
		"Use default ultra fast allocation")

	addField("capital_allocation.scalp_percent",
		currentAlloc.ScalpPercent, defaultAlloc.ScalpPercent,
		"medium", fmt.Sprintf("Current: %.1f%%, Default: %.1f%%", currentAlloc.ScalpPercent, defaultAlloc.ScalpPercent),
		"Use default scalp allocation")

	addField("capital_allocation.swing_percent",
		currentAlloc.SwingPercent, defaultAlloc.SwingPercent,
		"medium", fmt.Sprintf("Current: %.1f%%, Default: %.1f%%", currentAlloc.SwingPercent, defaultAlloc.SwingPercent),
		"Use default swing allocation")

	addField("capital_allocation.position_percent",
		currentAlloc.PositionPercent, defaultAlloc.PositionPercent,
		"medium", fmt.Sprintf("Current: %.1f%%, Default: %.1f%%", currentAlloc.PositionPercent, defaultAlloc.PositionPercent),
		"Use default position allocation")

	addField("capital_allocation.allow_dynamic_rebalance",
		currentAlloc.AllowDynamicRebalance, defaultAlloc.AllowDynamicRebalance,
		"low", fmt.Sprintf("Dynamic rebalance: %v, Default: %v", currentAlloc.AllowDynamicRebalance, defaultAlloc.AllowDynamicRebalance),
		"Enable dynamic rebalancing for adaptive allocation")

	addField("capital_allocation.rebalance_threshold_pct",
		currentAlloc.RebalanceThresholdPct, defaultAlloc.RebalanceThresholdPct,
		"low", fmt.Sprintf("Current threshold: %.1f%%, Default: %.1f%%", currentAlloc.RebalanceThresholdPct, defaultAlloc.RebalanceThresholdPct),
		"Use default rebalance threshold")

	if preview {
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "capital_allocation",
			AllMatch:     len(differences) == 0,
			TotalChanges: len(differences),
			Differences:  differences,
			AllValues:    allValues,
		})
		return
	}

	// Apply defaults to database
	if err := s.repo.SaveUserCapitalAllocation(ctx, defaultAlloc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save capital allocation: %v", err)})
		return
	}

	// Trigger immediate config reload in running autopilot
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
			log.Printf("[DEFAULTS-RESET] Triggered immediate config reload for user %s capital allocation", userID)
		}
	}

	c.JSON(http.StatusOK, ConfigResetResult{
		Success:        true,
		ConfigType:     "capital_allocation",
		ChangesApplied: len(differences),
		Message:        fmt.Sprintf("Capital allocation reset to defaults (%d changes applied)", len(differences)),
	})
}

// ====== COMPARISON HELPER FUNCTIONS ======
// NOTE: These functions are temporarily unused until AutopilotSettings is migrated
// to use nested struct types. Prefixed with _ to avoid compilation errors.

// _compareCircuitBreakerSettings compares current vs default circuit breaker settings
// TODO: Re-enable after Story 5.3 migration
func _compareCircuitBreakerSettings(current, defaultCfg *autopilot.CircuitBreakerDefaults) []SettingDifference {
	var differences []SettingDifference

	if current == nil || defaultCfg == nil {
		return differences
	}

	// Compare Global.Enabled
	if current.Global.Enabled != defaultCfg.Global.Enabled {
		differences = append(differences, SettingDifference{
			Path:           "circuit_breaker.global.enabled",
			Current:        current.Global.Enabled,
			Default:        defaultCfg.Global.Enabled,
			RiskLevel:      "high",
			Impact:         "Circuit breaker disabled = no automatic loss protection",
			Recommendation: "Keep circuit breaker enabled for safety",
		})
	}

	// Compare Global.MaxLossPerHour
	if current.Global.MaxLossPerHour != defaultCfg.Global.MaxLossPerHour {
		differences = append(differences, SettingDifference{
			Path:           "circuit_breaker.global.max_loss_per_hour",
			Current:        current.Global.MaxLossPerHour,
			Default:        defaultCfg.Global.MaxLossPerHour,
			RiskLevel:      "high",
			Impact:         fmt.Sprintf("Current hourly loss limit: $%.2f, Default: $%.2f", current.Global.MaxLossPerHour, defaultCfg.Global.MaxLossPerHour),
			Recommendation: "Use default for optimal risk control",
		})
	}

	// Compare Global.MaxDailyLoss
	if current.Global.MaxDailyLoss != defaultCfg.Global.MaxDailyLoss {
		differences = append(differences, SettingDifference{
			Path:           "circuit_breaker.global.max_daily_loss",
			Current:        current.Global.MaxDailyLoss,
			Default:        defaultCfg.Global.MaxDailyLoss,
			RiskLevel:      "high",
			Impact:         fmt.Sprintf("Current daily loss limit: $%.2f, Default: $%.2f", current.Global.MaxDailyLoss, defaultCfg.Global.MaxDailyLoss),
			Recommendation: "Use default for optimal risk control",
		})
	}

	// Compare Global.MaxConsecutiveLosses
	if current.Global.MaxConsecutiveLosses != defaultCfg.Global.MaxConsecutiveLosses {
		differences = append(differences, SettingDifference{
			Path:           "circuit_breaker.global.max_consecutive_losses",
			Current:        current.Global.MaxConsecutiveLosses,
			Default:        defaultCfg.Global.MaxConsecutiveLosses,
			RiskLevel:      "high",
			Impact:         "Affects when trading is paused after consecutive losses",
			Recommendation: "Use default to prevent loss streaks",
		})
	}

	// Compare Global.CooldownMinutes
	if current.Global.CooldownMinutes != defaultCfg.Global.CooldownMinutes {
		differences = append(differences, SettingDifference{
			Path:           "circuit_breaker.global.cooldown_minutes",
			Current:        current.Global.CooldownMinutes,
			Default:        defaultCfg.Global.CooldownMinutes,
			RiskLevel:      "medium",
			Impact:         "Affects how long trading is paused after circuit breaker trips",
			Recommendation: "Use default for balanced recovery time",
		})
	}

	// Compare Global.MaxTradesPerMinute
	if current.Global.MaxTradesPerMinute != defaultCfg.Global.MaxTradesPerMinute {
		differences = append(differences, SettingDifference{
			Path:           "circuit_breaker.global.max_trades_per_minute",
			Current:        current.Global.MaxTradesPerMinute,
			Default:        defaultCfg.Global.MaxTradesPerMinute,
			RiskLevel:      "medium",
			Impact:         "Affects rate limiting to prevent over-trading",
			Recommendation: "Use default to avoid excessive trading",
		})
	}

	// Compare Global.MaxDailyTrades
	if current.Global.MaxDailyTrades != defaultCfg.Global.MaxDailyTrades {
		differences = append(differences, SettingDifference{
			Path:           "circuit_breaker.global.max_daily_trades",
			Current:        current.Global.MaxDailyTrades,
			Default:        defaultCfg.Global.MaxDailyTrades,
			RiskLevel:      "medium",
			Impact:         "Affects maximum number of trades per day",
			Recommendation: "Use default to control daily trading volume",
		})
	}

	return differences
}

// _compareLLMConfigSettings compares current vs default LLM config settings
// TODO: Re-enable after AutopilotSettings.LLMConfig type migration
func _compareLLMConfigSettings(current, defaultCfg *autopilot.LLMConfigDefaults) []SettingDifference {
	var differences []SettingDifference

	if current == nil || defaultCfg == nil {
		return differences
	}

	// Compare Global.Enabled
	if current.Global.Enabled != defaultCfg.Global.Enabled {
		differences = append(differences, SettingDifference{
			Path:           "llm_config.global.enabled",
			Current:        current.Global.Enabled,
			Default:        defaultCfg.Global.Enabled,
			RiskLevel:      "high",
			Impact:         "LLM disabled = no AI-driven decision making",
			Recommendation: "Keep LLM enabled for intelligent trading",
		})
	}

	// Compare Global.Provider
	if current.Global.Provider != defaultCfg.Global.Provider {
		differences = append(differences, SettingDifference{
			Path:           "llm_config.global.provider",
			Current:        current.Global.Provider,
			Default:        defaultCfg.Global.Provider,
			RiskLevel:      "medium",
			Impact:         fmt.Sprintf("Current provider: %s, Default: %s", current.Global.Provider, defaultCfg.Global.Provider),
			Recommendation: "Use default provider for optimal performance",
		})
	}

	// Compare Global.Model
	if current.Global.Model != defaultCfg.Global.Model {
		differences = append(differences, SettingDifference{
			Path:           "llm_config.global.model",
			Current:        current.Global.Model,
			Default:        defaultCfg.Global.Model,
			RiskLevel:      "medium",
			Impact:         fmt.Sprintf("Current model: %s, Default: %s", current.Global.Model, defaultCfg.Global.Model),
			Recommendation: "Use default model for tested performance",
		})
	}

	// Compare Global.TimeoutMS
	if current.Global.TimeoutMS != defaultCfg.Global.TimeoutMS {
		differences = append(differences, SettingDifference{
			Path:           "llm_config.global.timeout_ms",
			Current:        current.Global.TimeoutMS,
			Default:        defaultCfg.Global.TimeoutMS,
			RiskLevel:      "low",
			Impact:         "Affects LLM call timeout duration",
			Recommendation: "Use default for balanced performance",
		})
	}

	// Compare Global.RetryCount
	if current.Global.RetryCount != defaultCfg.Global.RetryCount {
		differences = append(differences, SettingDifference{
			Path:           "llm_config.global.retry_count",
			Current:        current.Global.RetryCount,
			Default:        defaultCfg.Global.RetryCount,
			RiskLevel:      "low",
			Impact:         "Affects number of retry attempts for LLM calls",
			Recommendation: "Use default for reliable operation",
		})
	}

	// Compare Global.CacheDurationSec
	if current.Global.CacheDurationSec != defaultCfg.Global.CacheDurationSec {
		differences = append(differences, SettingDifference{
			Path:           "llm_config.global.cache_duration_sec",
			Current:        current.Global.CacheDurationSec,
			Default:        defaultCfg.Global.CacheDurationSec,
			RiskLevel:      "low",
			Impact:         "Affects LLM response caching duration",
			Recommendation: "Use default for optimal cache performance",
		})
	}

	return differences
}

// _compareCapitalAllocationSettings compares current vs default capital allocation settings
// TODO: Re-enable after CapitalAllocation field is added to AutopilotSettings
func _compareCapitalAllocationSettings(current, defaultCfg *autopilot.CapitalAllocationDefaults) []SettingDifference {
	var differences []SettingDifference

	if current == nil || defaultCfg == nil {
		return differences
	}

	// Compare UltraFastPercent
	if current.UltraFastPercent != defaultCfg.UltraFastPercent {
		differences = append(differences, SettingDifference{
			Path:           "capital_allocation.ultra_fast_percent",
			Current:        current.UltraFastPercent,
			Default:        defaultCfg.UltraFastPercent,
			RiskLevel:      "high",
			Impact:         fmt.Sprintf("Ultra-fast mode allocation: %.1f%%, Default: %.1f%%", current.UltraFastPercent, defaultCfg.UltraFastPercent),
			Recommendation: "Use default for balanced mode allocation",
		})
	}

	// Compare ScalpPercent
	if current.ScalpPercent != defaultCfg.ScalpPercent {
		differences = append(differences, SettingDifference{
			Path:           "capital_allocation.scalp_percent",
			Current:        current.ScalpPercent,
			Default:        defaultCfg.ScalpPercent,
			RiskLevel:      "high",
			Impact:         fmt.Sprintf("Scalp mode allocation: %.1f%%, Default: %.1f%%", current.ScalpPercent, defaultCfg.ScalpPercent),
			Recommendation: "Use default for balanced mode allocation",
		})
	}


	// Compare SwingPercent
	if current.SwingPercent != defaultCfg.SwingPercent {
		differences = append(differences, SettingDifference{
			Path:           "capital_allocation.swing_percent",
			Current:        current.SwingPercent,
			Default:        defaultCfg.SwingPercent,
			RiskLevel:      "high",
			Impact:         fmt.Sprintf("Swing mode allocation: %.1f%%, Default: %.1f%%", current.SwingPercent, defaultCfg.SwingPercent),
			Recommendation: "Use default for balanced mode allocation",
		})
	}

	// Compare PositionPercent
	if current.PositionPercent != defaultCfg.PositionPercent {
		differences = append(differences, SettingDifference{
			Path:           "capital_allocation.position_percent",
			Current:        current.PositionPercent,
			Default:        defaultCfg.PositionPercent,
			RiskLevel:      "high",
			Impact:         fmt.Sprintf("Position mode allocation: %.1f%%, Default: %.1f%%", current.PositionPercent, defaultCfg.PositionPercent),
			Recommendation: "Use default for balanced mode allocation",
		})
	}

	return differences
}

// handleLoadHedgeDefaults resets hedge mode settings to default values
// POST /api/futures/ginie/hedge-mode/load-defaults?preview=true
func (s *Server) handleLoadHedgeDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	sm := autopilot.GetSettingsManager()

	// Load defaults from default-settings.json
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load defaults: %v", err)})
		return
	}

	// Hedge settings are per-mode, so we need to compare all modes
	var differences []SettingDifference
	var allValues []FieldComparison
	modes := []string{"ultra_fast", "scalp", "scalp_reentry", "swing", "position"}

	// Helper to add field comparison
	addField := func(path string, current, defaultVal interface{}, riskLevel, impact, recommendation string) {
		match := reflect.DeepEqual(current, defaultVal)

		// Add to AllValues
		fc := FieldComparison{
			Path:    path,
			Current: current,
			Default: defaultVal,
			Match:   match,
		}
		if !match {
			fc.RiskLevel = riskLevel
		}
		allValues = append(allValues, fc)

		// Add to Differences only if different
		if !match {
			differences = append(differences, SettingDifference{
				Path:           path,
				Current:        current,
				Default:        defaultVal,
				RiskLevel:      riskLevel,
				Impact:         impact,
				Recommendation: recommendation,
			})
		}
	}

	for _, mode := range modes {
		// Get user's CURRENT mode config from DATABASE
		currentMode, dbErr := sm.GetUserModeConfigFromDB(ctx, s.repo, userID, mode)
		if dbErr != nil || currentMode == nil || currentMode.Hedge == nil {
			continue
		}

		defaultMode, defExists := defaults.ModeConfigs[mode]
		if !defExists || defaultMode == nil || defaultMode.Hedge == nil {
			continue
		}

		currentHedge := currentMode.Hedge
		defaultHedge := defaultMode.Hedge

		// Compare AllowHedge
		addField(fmt.Sprintf("%s.hedge.allow_hedge", mode),
			currentHedge.AllowHedge, defaultHedge.AllowHedge,
			"high", fmt.Sprintf("Hedging %s for %s mode", map[bool]string{true: "enabled", false: "disabled"}[defaultHedge.AllowHedge], mode),
			"Use default hedge setting for optimal risk management")

		// Compare MinConfidenceForHedge
		addField(fmt.Sprintf("%s.hedge.min_confidence_for_hedge", mode),
			currentHedge.MinConfidenceForHedge, defaultHedge.MinConfidenceForHedge,
			"medium", fmt.Sprintf("%s: Current min confidence: %.0f%%, Default: %.0f%%", mode, currentHedge.MinConfidenceForHedge, defaultHedge.MinConfidenceForHedge),
			"Higher confidence = safer hedge entries")

		// Compare ExistingMustBeInProfit
		addField(fmt.Sprintf("%s.hedge.existing_must_be_in_profit", mode),
			currentHedge.ExistingMustBeInProfit, defaultHedge.ExistingMustBeInProfit,
			"medium", fmt.Sprintf("%s: Current profit threshold: %.2f%%, Default: %.2f%%", mode, currentHedge.ExistingMustBeInProfit, defaultHedge.ExistingMustBeInProfit),
			"Profit threshold protects existing positions")

		// Compare MaxHedgeSizePercent
		addField(fmt.Sprintf("%s.hedge.max_hedge_size_percent", mode),
			currentHedge.MaxHedgeSizePercent, defaultHedge.MaxHedgeSizePercent,
			"high", fmt.Sprintf("%s: Current max hedge size: %.0f%%, Default: %.0f%%", mode, currentHedge.MaxHedgeSizePercent, defaultHedge.MaxHedgeSizePercent),
			"Lower hedge size = lower total exposure risk")

		// Compare AllowSameModeHedge
		addField(fmt.Sprintf("%s.hedge.allow_same_mode_hedge", mode),
			currentHedge.AllowSameModeHedge, defaultHedge.AllowSameModeHedge,
			"medium", fmt.Sprintf("%s: Same-mode hedging %s", mode, map[bool]string{true: "allowed", false: "blocked"}[defaultHedge.AllowSameModeHedge]),
			"Blocking same-mode hedges reduces over-exposure")

		// Compare MaxTotalExposureMultiplier
		addField(fmt.Sprintf("%s.hedge.max_total_exposure_multiplier", mode),
			currentHedge.MaxTotalExposureMultiplier, defaultHedge.MaxTotalExposureMultiplier,
			"high", fmt.Sprintf("%s: Current exposure cap: %.1fx, Default: %.1fx", mode, currentHedge.MaxTotalExposureMultiplier, defaultHedge.MaxTotalExposureMultiplier),
			"Lower multiplier = tighter exposure control")
	}

	if preview {
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "hedge_mode",
			AllMatch:     len(differences) == 0,
			TotalChanges: len(differences),
			Differences:  differences,
			AllValues:    allValues,
		})
		return
	}

	// Apply defaults to all modes in DATABASE
	changesApplied := 0
	for _, mode := range modes {
		defaultMode, defExists := defaults.ModeConfigs[mode]
		if !defExists || defaultMode == nil {
			continue
		}

		// Serialize default config to JSON for database storage
		configJSON, marshalErr := json.Marshal(defaultMode)
		if marshalErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to serialize %s config: %v", mode, marshalErr)})
			return
		}

		// Save defaults to user's database record
		if saveErr := s.repo.SaveUserModeConfig(ctx, userID, mode, defaultMode.Enabled, configJSON); saveErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save %s settings: %v", mode, saveErr)})
			return
		}
		changesApplied++
	}

	// Trigger immediate config reload in running autopilot
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
			log.Printf("[DEFAULTS-RESET] Triggered immediate config reload for user %s hedge mode", userID)
		}
	}

	c.JSON(http.StatusOK, ConfigResetResult{
		Success:        true,
		ConfigType:     "hedge_mode",
		ChangesApplied: len(differences),
		Message:        fmt.Sprintf("Hedge mode settings reset to defaults for %d modes (%d changes applied)", changesApplied, len(differences)),
	})
}
