package api

import (
	"binance-trading-bot/internal/auth"
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/database"
	"context"
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
	Preview       bool                `json:"preview"`                  // true if preview mode
	Mode          string              `json:"mode"`                     // Mode name (legacy)
	ConfigType    string              `json:"config_type"`              // Config type for frontend (scalp, swing, scalp_reentry, etc)
	TotalChanges  int                 `json:"total_changes"`            // Count of differences
	AllMatch      bool                `json:"all_match"`                // true if settings match defaults
	Differences   []SettingDifference `json:"differences"`              // List of differences (only fields that differ)
	AllValues     []FieldComparison   `json:"all_values"`               // ALL fields with current vs default comparison
	AppliedAt     string              `json:"applied_at,omitempty"`     // Timestamp if applied
	IsAdmin       bool                `json:"is_admin"`                 // true if user is admin (Story 9.4)
	DefaultValue  interface{}         `json:"default_value,omitempty"`  // Raw default config object for admin editing (Story 9.4)
}

// AllModesDiffResponse represents the diff response for all modes
type AllModesDiffResponse struct {
	Preview      bool                         `json:"preview"`                 // true if preview mode
	ConfigType   string                       `json:"config_type"`             // Config type for frontend
	TotalChanges int                          `json:"total_changes"`           // Total across all modes
	AllMatch     bool                         `json:"all_match"`               // true if all settings match
	Modes        map[string]*ModeDiffResponse `json:"modes"`                   // Per-mode diffs
	AppliedAt    string                       `json:"applied_at,omitempty"`    // Timestamp if applied
	IsAdmin      bool                         `json:"is_admin"`                // true if user is admin (Story 9.4)
	DefaultValue interface{}                  `json:"default_value,omitempty"` // Raw default config object for admin editing (Story 9.4)
}

// handleLoadModeDefaults loads default settings for a specific mode
// POST /api/futures/ginie/modes/:mode/load-defaults?preview=true
// For ADMIN users in preview mode: Returns default-settings.json values directly (for editing)
// For REGULAR users in preview mode: Compares user's DB settings vs defaults
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
	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// SPECIAL CASE: scalp_reentry is NOT a regular mode - it's stored separately as ScalpReentryConfig
	// It has a completely different structure (~50 fields) and uses different database methods
	if mode == "scalp_reentry" {
		s.handleLoadScalpReentryDefaultsInternal(c, userID, preview, isAdmin)
		return
	}

	// Get default mode config from default-settings.json
	defaultMode, err := autopilot.GetDefaultModeFullConfig(mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	// ADMIN USER PREVIEW: Return default-settings.json values directly for editing
	// Admin doesn't need comparison - they edit the defaults themselves
	if isAdmin && preview {
		// Return defaults directly with all values for admin editing
		response := &ModeDiffResponse{
			Preview:      true,
			Mode:         mode,
			ConfigType:   mode, // Frontend expects config_type for admin editing
			IsAdmin:      true,
			AllMatch:     true,                                          // No comparison needed for admin
			TotalChanges: 0,                                             // No changes to show - just displaying defaults
			Differences:  []SettingDifference{},
			AllValues:    buildAllValuesFromDefaults(mode, defaultMode), // All default values for editing
			DefaultValue: defaultMode,                                   // Raw config object for admin editing
		}
		c.JSON(http.StatusOK, response)
		return
	}

	// REGULAR USER: Get current settings from DB for comparison
	sm := autopilot.GetSettingsManager()
	currentMode, err := sm.GetUserModeConfigFromDB(ctx, s.repo, userID, mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings from database: "+err.Error())
		return
	}
	if currentMode == nil {
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Mode config not found in database: %s", mode))
		return
	}

	// Compare user's settings vs defaults
	diff := compareModeConfigs(mode, currentMode, defaultMode)
	diff.IsAdmin = isAdmin

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

// handleResetModeGroup resets only a specific group within a mode to defaults
// POST /api/futures/ginie/modes/:mode/groups/:group/reset
func (s *Server) handleResetModeGroup(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	mode := c.Param("mode")
	group := c.Param("group")

	if mode == "" || group == "" {
		errorResponse(c, http.StatusBadRequest, "Mode and group are required")
		return
	}

	// Validate mode name
	validModes := []string{"ultra_fast", "scalp", "swing", "position"}
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

	ctx := c.Request.Context()

	// Get default mode config from default-settings.json
	defaultMode, err := autopilot.GetDefaultModeFullConfig(mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	// Get current user's mode config from DB
	sm := autopilot.GetSettingsManager()
	currentMode, err := sm.GetUserModeConfigFromDB(ctx, s.repo, userID, mode)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings: "+err.Error())
		return
	}
	if currentMode == nil {
		errorResponse(c, http.StatusNotFound, fmt.Sprintf("Mode config not found: %s", mode))
		return
	}

	// Reset only the specific group
	changesApplied := resetModeGroupToDefaults(currentMode, defaultMode, group)

	// Save updated config to database
	configJSON, marshalErr := json.Marshal(currentMode)
	if marshalErr != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to serialize config: "+marshalErr.Error())
		return
	}

	if saveErr := s.repo.SaveUserModeConfig(ctx, userID, mode, currentMode.Enabled, configJSON); saveErr != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save settings: "+saveErr.Error())
		return
	}

	// Trigger config reload
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
		}
	}

	c.JSON(http.StatusOK, ConfigResetResult{
		Success:        true,
		ConfigType:     fmt.Sprintf("%s.%s", mode, group),
		ChangesApplied: changesApplied,
		Message:        fmt.Sprintf("%s %s group reset to defaults", mode, group),
	})
}

// resetModeGroupToDefaults resets a specific group within a mode config
func resetModeGroupToDefaults(current, defaults *autopilot.ModeFullConfig, group string) int {
	changesApplied := 0

	switch group {
	case "enabled":
		if current.Enabled != defaults.Enabled {
			current.Enabled = defaults.Enabled
			changesApplied++
		}
	case "confidence":
		if defaults.Confidence != nil {
			current.Confidence = defaults.Confidence
			changesApplied++
		}
	case "size":
		if defaults.Size != nil {
			current.Size = defaults.Size
			changesApplied++
		}
	case "sltp":
		if defaults.SLTP != nil {
			current.SLTP = defaults.SLTP
			changesApplied++
		}
	case "circuit_breaker":
		if defaults.CircuitBreaker != nil {
			current.CircuitBreaker = defaults.CircuitBreaker
			changesApplied++
		}
	case "timeframe":
		if defaults.Timeframe != nil {
			current.Timeframe = defaults.Timeframe
			changesApplied++
		}
	case "entry":
		if defaults.Entry != nil {
			current.Entry = defaults.Entry
			changesApplied++
		}
	case "averaging":
		if defaults.Averaging != nil {
			current.Averaging = defaults.Averaging
			changesApplied++
		}
	case "hedge":
		if defaults.Hedge != nil {
			current.Hedge = defaults.Hedge
			changesApplied++
		}
	case "risk":
		if defaults.Risk != nil {
			current.Risk = defaults.Risk
			changesApplied++
		}
	case "stale_release":
		if defaults.StaleRelease != nil {
			current.StaleRelease = defaults.StaleRelease
			changesApplied++
		}
	case "assignment":
		if defaults.Assignment != nil {
			current.Assignment = defaults.Assignment
			changesApplied++
		}
	case "mtf":
		if defaults.MTF != nil {
			current.MTF = defaults.MTF
			changesApplied++
		}
	case "dynamic_ai_exit":
		if defaults.DynamicAIExit != nil {
			current.DynamicAIExit = defaults.DynamicAIExit
			changesApplied++
		}
	case "reversal":
		if defaults.Reversal != nil {
			current.Reversal = defaults.Reversal
			changesApplied++
		}
	case "funding_rate":
		if defaults.FundingRate != nil {
			current.FundingRate = defaults.FundingRate
			changesApplied++
		}
	case "trend_divergence":
		if defaults.TrendDivergence != nil {
			current.TrendDivergence = defaults.TrendDivergence
			changesApplied++
		}
	case "position_optimization":
		if defaults.PositionOptimization != nil {
			current.PositionOptimization = defaults.PositionOptimization
			changesApplied++
		}
	case "trend_filters":
		if defaults.TrendFilters != nil {
			current.TrendFilters = defaults.TrendFilters
			changesApplied++
		}
	case "early_warning":
		if defaults.EarlyWarning != nil {
			current.EarlyWarning = defaults.EarlyWarning
			changesApplied++
		}
	}

	return changesApplied
}

// buildAllValuesFromDefaults creates AllValues list from default config only (for admin view)
// Shows all default values without comparison - admin sees/edits the defaults directly
func buildAllValuesFromDefaults(mode string, defaultMode *autopilot.ModeFullConfig) []FieldComparison {
	var allValues []FieldComparison

	if defaultMode == nil {
		return allValues
	}

	// Use reflection to extract all fields from default config
	addDefaultFields("", reflect.ValueOf(defaultMode).Elem(), &allValues)

	return allValues
}

// addDefaultFields recursively extracts fields from a struct for admin view
func addDefaultFields(prefix string, v reflect.Value, allValues *[]FieldComparison) {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Get JSON tag name
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		// Remove omitempty suffix
		if idx := len(jsonTag) - 1; idx >= 0 {
			for j := 0; j < len(jsonTag); j++ {
				if jsonTag[j] == ',' {
					jsonTag = jsonTag[:j]
					break
				}
			}
		}

		// Build path
		path := jsonTag
		if prefix != "" {
			path = prefix + "." + jsonTag
		}

		// Handle different types
		switch fieldValue.Kind() {
		case reflect.Struct:
			// Recurse into nested structs
			addDefaultFields(path, fieldValue, allValues)
		case reflect.Ptr:
			if !fieldValue.IsNil() {
				addDefaultFields(path, fieldValue.Elem(), allValues)
			}
		default:
			// Add leaf field - for admin, Current = Default (no comparison)
			*allValues = append(*allValues, FieldComparison{
				Path:    path,
				Current: fieldValue.Interface(), // Show default as current
				Default: fieldValue.Interface(), // Same as default
				Match:   true,                   // Always match for admin view
			})
		}
	}
}

// handleLoadAllModeDefaults loads default settings for all modes
// POST /api/futures/ginie/modes/load-defaults?preview=true
// For ADMIN users in preview mode: Returns default-settings.json values directly (for editing)
// For REGULAR users in preview mode: Compares user's DB settings vs defaults
func (s *Server) handleLoadAllModeDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// Get all default mode configs from default-settings.json
	defaultModes, err := autopilot.GetAllDefaultModeFullConfigs()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	// NOTE: scalp_reentry is NOT a trading mode - it's a Position Optimization feature
	// It's handled separately and uses ScalpReentryConfig instead of ModeFullConfig
	modeNames := []string{"ultra_fast", "scalp", "swing", "position"}

	// ADMIN USER PREVIEW: Return default-settings.json values directly for editing
	if isAdmin && preview {
		// Build combined default value map for all modes
		allDefaultsMap := make(map[string]interface{})
		for _, modeName := range modeNames {
			if defaultModes[modeName] != nil {
				allDefaultsMap[modeName] = defaultModes[modeName]
			}
		}

		response := &AllModesDiffResponse{
			Preview:      true,
			ConfigType:   "all_modes", // Frontend expects config_type
			IsAdmin:      true,
			AllMatch:     true, // No comparison needed for admin
			TotalChanges: 0,
			Modes:        make(map[string]*ModeDiffResponse),
			DefaultValue: allDefaultsMap, // Raw config object for admin editing
		}

		for _, modeName := range modeNames {
			defaultMode := defaultModes[modeName]
			if defaultMode != nil {
				response.Modes[modeName] = &ModeDiffResponse{
					Preview:      true,
					Mode:         modeName,
					ConfigType:   modeName, // Frontend expects config_type
					IsAdmin:      true,
					AllMatch:     true,
					TotalChanges: 0,
					Differences:  []SettingDifference{},
					AllValues:    buildAllValuesFromDefaults(modeName, defaultMode),
					DefaultValue: defaultMode, // Raw config object for admin editing
				}
			}
		}

		c.JSON(http.StatusOK, response)
		return
	}

	// REGULAR USER: Compare user's DB settings vs defaults
	sm := autopilot.GetSettingsManager()
	response := &AllModesDiffResponse{
		Preview: preview,
		Modes:   make(map[string]*ModeDiffResponse),
		IsAdmin: isAdmin,
	}

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
			diff.IsAdmin = isAdmin
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

	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// SPECIAL CASE: scalp_reentry is NOT a regular mode - it's stored separately as ScalpReentryConfig
	if mode == "scalp_reentry" {
		s.handleGetScalpReentryDiffInternal(c, userID, isAdmin)
		return
	}

	// Get user's current settings FROM DATABASE (not JSON file!)
	sm := autopilot.GetSettingsManager()
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
	diff.Preview = true   // This is read-only, always preview
	diff.IsAdmin = isAdmin // Set admin flag for frontend (Story 9.4)

	c.JSON(http.StatusOK, diff)
}

// handleGetScalpReentryDiffInternal handles scalp_reentry diff request
func (s *Server) handleGetScalpReentryDiffInternal(c *gin.Context, userID string, isAdmin bool) {
	ctx := c.Request.Context()

	// Get user's CURRENT scalp_reentry config from DATABASE
	currentConfigJSON, err := s.repo.GetUserScalpReentryConfig(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user scalp_reentry config: "+err.Error())
		return
	}

	// Parse current config (or use empty if not found)
	var currentConfig autopilot.PositionOptimizationConfig
	if currentConfigJSON != nil {
		if err := json.Unmarshal(currentConfigJSON, &currentConfig); err != nil {
			log.Printf("[SCALP-REENTRY-DIFF] WARNING: Failed to parse user config, using empty: %v", err)
		}
	}

	// Get default scalp_reentry config from default-settings.json
	defaultConfig, err := autopilot.GetDefaultPositionOptimizationConfig()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default scalp_reentry settings: "+err.Error())
		return
	}

	// Compare and generate diff
	diff := compareScalpReentryConfigs(&currentConfig, defaultConfig)
	diff.Preview = true    // This is read-only, always preview
	diff.IsAdmin = isAdmin // Set admin flag for frontend (Story 9.4)

	c.JSON(http.StatusOK, diff)
}

// handleLoadAllDefaults loads all default settings (global + all modes)
// POST /api/user/settings/load-defaults
func (s *Server) handleLoadAllDefaults(c *gin.Context) {
	// FIXED: Get userID for database operations - no fallback allowed
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"

	// Force reload defaults from disk to pick up any changes without container restart
	if reloadErr := autopilot.ReloadDefaultSettings(); reloadErr != nil {
		log.Printf("[SETTINGS-DEFAULTS] Warning: Failed to reload defaults from disk: %v", reloadErr)
	}

	// Load complete default settings file
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	// FIXED: Get user's current settings from database, not GetDefaultSettings()
	sm := autopilot.GetSettingsManager()
	currentSettings, loadErr := sm.LoadSettingsFromDB(c.Request.Context(), s.repo, userID)
	if loadErr != nil || currentSettings == nil {
		log.Printf("[SETTINGS-DEFAULTS] ERROR: Failed to load user settings from database: %v", loadErr)
		errorResponse(c, http.StatusInternalServerError, "Failed to load user settings from database")
		return
	}

	// Compare all settings
	response := &AllModesDiffResponse{
		Preview: preview,
		Modes:   make(map[string]*ModeDiffResponse),
	}

	// Compare mode configs
	// NOTE: scalp_reentry is NOT a trading mode - it's a Position Optimization feature
	// stored separately as ScalpReentryConfig, not in ModeConfigs
	modes := []struct {
		name    string
		current *autopilot.ModeFullConfig
		def     *autopilot.ModeFullConfig
	}{
		{"ultra_fast", currentSettings.ModeConfigs["ultra_fast"], defaults.ModeConfigs["ultra_fast"]},
		{"scalp", currentSettings.ModeConfigs["scalp"], defaults.ModeConfigs["scalp"]},
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
		// Apply mode configs (4 trading modes - scalp_reentry is separate)
		currentSettings.ModeConfigs["ultra_fast"] = defaults.ModeConfigs["ultra_fast"]
		currentSettings.ModeConfigs["scalp"] = defaults.ModeConfigs["scalp"]
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
	compareSections("entry", current.Entry, defaultCfg.Entry, response) // Entry order settings
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

	// Compare position_optimization (Story 9.9 - TP1/TP2/TP3, DCA, Reentry, Hedging, etc.)
	compareSections("position_optimization", current.PositionOptimization, defaultCfg.PositionOptimization, response)

	// Compare early_warning (Mode-specific early warning - Story 9.4 Phase 4)
	compareSections("early_warning", current.EarlyWarning, defaultCfg.EarlyWarning, response)

	// Compare trend_filters sub-sections (Story 9.5)
	// Each sub-filter (btc_trend_check, price_vs_ema, vwap_filter, candlestick_alignment) is compared separately
	// to generate proper nested paths like "trend_filters.candlestick_alignment.enabled"
	compareTrendFilters(current.TrendFilters, defaultCfg.TrendFilters, response)

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

// compareTrendFilters compares trend_filters sub-sections (Story 9.5)
// Handles nested structure: trend_filters.{btc_trend_check|price_vs_ema|vwap_filter|candlestick_alignment}
func compareTrendFilters(current, defaultCfg *autopilot.TrendFiltersConfig, response *ModeDiffResponse) {
	// Handle nil cases
	if current == nil && defaultCfg == nil {
		return
	}

	// If user has no trend_filters but default does, show all defaults as differences
	if current == nil && defaultCfg != nil {
		compareSections("trend_filters.btc_trend_check", nil, defaultCfg.BTCTrendCheck, response)
		compareSections("trend_filters.price_vs_ema", nil, defaultCfg.PriceVsEMA, response)
		compareSections("trend_filters.vwap_filter", nil, defaultCfg.VWAPFilter, response)
		compareSections("trend_filters.candlestick_alignment", nil, defaultCfg.CandlestickAlignment, response)
		return
	}

	// If user has trend_filters but default doesn't, skip (keep user's values)
	if current != nil && defaultCfg == nil {
		return
	}

	// Both exist - compare each sub-section
	compareSections("trend_filters.btc_trend_check", current.BTCTrendCheck, defaultCfg.BTCTrendCheck, response)
	compareSections("trend_filters.price_vs_ema", current.PriceVsEMA, defaultCfg.PriceVsEMA, response)
	compareSections("trend_filters.vwap_filter", current.VWAPFilter, defaultCfg.VWAPFilter, response)
	compareSections("trend_filters.candlestick_alignment", current.CandlestickAlignment, defaultCfg.CandlestickAlignment, response)
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
	Differences  []SettingDifference `json:"differences"`              // Only fields that differ
	AllValues    []FieldComparison   `json:"all_values"`               // ALL fields with current vs default comparison
	IsAdmin      bool                `json:"is_admin"`                 // true if user is admin (Story 9.4)
	DefaultValue interface{}         `json:"default_value,omitempty"`  // Raw default config object for admin editing (Story 9.4)
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
// For ADMIN users in preview mode: Returns default-settings.json values directly (for editing)
// For REGULAR users in preview mode: Compares user's DB settings vs defaults
func (s *Server) handleLoadCircuitBreakerDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// Load defaults from default-settings.json
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load defaults: %v", err)})
		return
	}

	// ADMIN USER PREVIEW: Return default-settings.json values directly for editing
	if isAdmin && preview {
		allValues := []FieldComparison{
			{Path: "circuit_breaker.enabled", Current: defaults.CircuitBreaker.Global.Enabled, Default: defaults.CircuitBreaker.Global.Enabled, Match: true},
			{Path: "circuit_breaker.max_loss_per_hour", Current: defaults.CircuitBreaker.Global.MaxLossPerHour, Default: defaults.CircuitBreaker.Global.MaxLossPerHour, Match: true},
			{Path: "circuit_breaker.max_daily_loss", Current: defaults.CircuitBreaker.Global.MaxDailyLoss, Default: defaults.CircuitBreaker.Global.MaxDailyLoss, Match: true},
			{Path: "circuit_breaker.max_consecutive_losses", Current: defaults.CircuitBreaker.Global.MaxConsecutiveLosses, Default: defaults.CircuitBreaker.Global.MaxConsecutiveLosses, Match: true},
			{Path: "circuit_breaker.cooldown_minutes", Current: defaults.CircuitBreaker.Global.CooldownMinutes, Default: defaults.CircuitBreaker.Global.CooldownMinutes, Match: true},
			{Path: "circuit_breaker.max_trades_per_minute", Current: defaults.CircuitBreaker.Global.MaxTradesPerMinute, Default: defaults.CircuitBreaker.Global.MaxTradesPerMinute, Match: true},
			{Path: "circuit_breaker.max_daily_trades", Current: defaults.CircuitBreaker.Global.MaxDailyTrades, Default: defaults.CircuitBreaker.Global.MaxDailyTrades, Match: true},
		}
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "circuit_breaker",
			AllMatch:     true,
			TotalChanges: 0,
			Differences:  []SettingDifference{},
			AllValues:    allValues,
			IsAdmin:      true,
			DefaultValue: defaults.CircuitBreaker.Global, // Raw config object for admin editing
		})
		return
	}

	// REGULAR USER: Get current settings from DB for comparison
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
			IsAdmin:      isAdmin, // Set admin flag for frontend (Story 9.4)
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
// For ADMIN users in preview mode: Returns default-settings.json values directly (for editing)
// For REGULAR users in preview mode: Compares user's DB settings vs defaults
func (s *Server) handleLoadLLMConfigDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// Load defaults from default-settings.json
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load defaults: %v", err)})
		return
	}

	// ADMIN USER PREVIEW: Return default-settings.json values directly for editing
	if isAdmin && preview {
		allValues := []FieldComparison{
			{Path: "llm_config.enabled", Current: defaults.LLMConfig.Global.Enabled, Default: defaults.LLMConfig.Global.Enabled, Match: true},
			{Path: "llm_config.provider", Current: defaults.LLMConfig.Global.Provider, Default: defaults.LLMConfig.Global.Provider, Match: true},
			{Path: "llm_config.model", Current: defaults.LLMConfig.Global.Model, Default: defaults.LLMConfig.Global.Model, Match: true},
			{Path: "llm_config.timeout_ms", Current: defaults.LLMConfig.Global.TimeoutMS, Default: defaults.LLMConfig.Global.TimeoutMS, Match: true},
			{Path: "llm_config.retry_count", Current: defaults.LLMConfig.Global.RetryCount, Default: defaults.LLMConfig.Global.RetryCount, Match: true},
			{Path: "llm_config.cache_duration_sec", Current: defaults.LLMConfig.Global.CacheDurationSec, Default: defaults.LLMConfig.Global.CacheDurationSec, Match: true},
		}
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "llm_config",
			AllMatch:     true,
			TotalChanges: 0,
			Differences:  []SettingDifference{},
			AllValues:    allValues,
			IsAdmin:      true,
			DefaultValue: defaults.LLMConfig.Global, // Raw config object for admin editing
		})
		return
	}

	// REGULAR USER: Get current settings from DB for comparison
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

	// Compare current vs defaults (defaults already loaded above)
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
			IsAdmin:      isAdmin, // Set admin flag for frontend (Story 9.4)
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
// For ADMIN users in preview mode: Returns default-settings.json values directly (for editing)
// For REGULAR users in preview mode: Compares user's DB settings vs defaults
func (s *Server) handleLoadCapitalAllocationDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// ADMIN USER PREVIEW: Return values from default-settings.json for editing
	if isAdmin && preview {
		// Load from default-settings.json (the source of truth for admin editing)
		defaults, err := autopilot.LoadDefaultSettings()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load default-settings.json: %v", err)})
			return
		}

		// Get capital allocation from the JSON file
		jsonAlloc := defaults.CapitalAllocation

		allValues := []FieldComparison{
			{Path: "ultra_fast_percent", Current: jsonAlloc.UltraFastPercent, Default: jsonAlloc.UltraFastPercent, Match: true},
			{Path: "scalp_percent", Current: jsonAlloc.ScalpPercent, Default: jsonAlloc.ScalpPercent, Match: true},
			{Path: "swing_percent", Current: jsonAlloc.SwingPercent, Default: jsonAlloc.SwingPercent, Match: true},
			{Path: "position_percent", Current: jsonAlloc.PositionPercent, Default: jsonAlloc.PositionPercent, Match: true},
			{Path: "allow_dynamic_rebalance", Current: jsonAlloc.AllowDynamicRebalance, Default: jsonAlloc.AllowDynamicRebalance, Match: true},
			{Path: "rebalance_threshold_pct", Current: jsonAlloc.RebalanceThresholdPct, Default: jsonAlloc.RebalanceThresholdPct, Match: true},
		}
		// Create a simplified default value map for admin editing
		defaultValueMap := map[string]interface{}{
			"ultra_fast_percent":       jsonAlloc.UltraFastPercent,
			"scalp_percent":            jsonAlloc.ScalpPercent,
			"swing_percent":            jsonAlloc.SwingPercent,
			"position_percent":         jsonAlloc.PositionPercent,
			"allow_dynamic_rebalance":  jsonAlloc.AllowDynamicRebalance,
			"rebalance_threshold_pct":  jsonAlloc.RebalanceThresholdPct,
		}
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "capital_allocation",
			AllMatch:     true,
			TotalChanges: 0,
			Differences:  []SettingDifference{},
			AllValues:    allValues,
			IsAdmin:      true,
			DefaultValue: defaultValueMap, // Raw config object for admin editing
		})
		return
	}

	// For regular users, load defaults from default-settings.json (source of truth)
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load default-settings.json: %v", err)})
		return
	}
	jsonAlloc := defaults.CapitalAllocation

	// REGULAR USER: Get current settings (cache-first, fallback to DB)
	// Story 6.5: Cache-first pattern
	var currentAlloc *database.UserCapitalAllocation
	if s.settingsCacheService != nil {
		currentAlloc, err = s.settingsCacheService.GetCapitalAllocation(ctx, userID)
		if err != nil {
			if IsCacheUnavailableError(err) {
				RespondCacheUnavailable(c, "get_capital_allocation")
				return
			}
			// Other cache error - fallback to DB
			log.Printf("[CAPITAL-ALLOCATION] Cache error for user %s: %v, falling back to DB", userID, err)
			currentAlloc = nil
		}
	}

	// Fallback to direct DB if cache miss or cache not available
	if currentAlloc == nil {
		currentAlloc, err = s.repo.GetUserCapitalAllocation(ctx, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load user capital allocation: %v", err)})
			return
		}
	}

	// If no config exists, use database defaults for comparison
	if currentAlloc == nil {
		currentAlloc = database.DefaultUserCapitalAllocation()
		currentAlloc.UserID = userID
	}

	// Compare current vs defaults from default-settings.json
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

	// Capital Allocation Percentages - compare against default-settings.json values
	addField("capital_allocation.ultra_fast_percent",
		currentAlloc.UltraFastPercent, jsonAlloc.UltraFastPercent,
		"medium", fmt.Sprintf("Current: %.1f%%, Default: %.1f%%", currentAlloc.UltraFastPercent, jsonAlloc.UltraFastPercent),
		"Use default ultra fast allocation")

	addField("capital_allocation.scalp_percent",
		currentAlloc.ScalpPercent, jsonAlloc.ScalpPercent,
		"medium", fmt.Sprintf("Current: %.1f%%, Default: %.1f%%", currentAlloc.ScalpPercent, jsonAlloc.ScalpPercent),
		"Use default scalp allocation")

	addField("capital_allocation.swing_percent",
		currentAlloc.SwingPercent, jsonAlloc.SwingPercent,
		"medium", fmt.Sprintf("Current: %.1f%%, Default: %.1f%%", currentAlloc.SwingPercent, jsonAlloc.SwingPercent),
		"Use default swing allocation")

	addField("capital_allocation.position_percent",
		currentAlloc.PositionPercent, jsonAlloc.PositionPercent,
		"medium", fmt.Sprintf("Current: %.1f%%, Default: %.1f%%", currentAlloc.PositionPercent, jsonAlloc.PositionPercent),
		"Use default position allocation")

	addField("capital_allocation.allow_dynamic_rebalance",
		currentAlloc.AllowDynamicRebalance, jsonAlloc.AllowDynamicRebalance,
		"low", fmt.Sprintf("Dynamic rebalance: %v, Default: %v", currentAlloc.AllowDynamicRebalance, jsonAlloc.AllowDynamicRebalance),
		"Enable dynamic rebalancing for adaptive allocation")

	addField("capital_allocation.rebalance_threshold_pct",
		currentAlloc.RebalanceThresholdPct, jsonAlloc.RebalanceThresholdPct,
		"low", fmt.Sprintf("Current threshold: %.1f%%, Default: %.1f%%", currentAlloc.RebalanceThresholdPct, jsonAlloc.RebalanceThresholdPct),
		"Use default rebalance threshold")

	if preview {
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "capital_allocation",
			AllMatch:     len(differences) == 0,
			TotalChanges: len(differences),
			Differences:  differences,
			AllValues:    allValues,
			IsAdmin:      isAdmin, // Set admin flag for frontend (Story 9.4)
		})
		return
	}

	// Apply defaults from default-settings.json to database
	// Update only the fields managed by defaults (preserve other fields like max positions)
	currentAlloc.UltraFastPercent = jsonAlloc.UltraFastPercent
	currentAlloc.ScalpPercent = jsonAlloc.ScalpPercent
	currentAlloc.SwingPercent = jsonAlloc.SwingPercent
	currentAlloc.PositionPercent = jsonAlloc.PositionPercent
	currentAlloc.AllowDynamicRebalance = jsonAlloc.AllowDynamicRebalance
	currentAlloc.RebalanceThresholdPct = jsonAlloc.RebalanceThresholdPct

	// Story 6.5: Use cache service for write-through pattern
	if s.settingsCacheService != nil {
		if err := s.settingsCacheService.UpdateCapitalAllocation(ctx, userID, currentAlloc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save capital allocation: %v", err)})
			return
		}
	} else {
		// Fallback to direct DB write if cache service not available
		if err := s.repo.SaveUserCapitalAllocation(ctx, currentAlloc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save capital allocation: %v", err)})
			return
		}
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
// For ADMIN users in preview mode: Returns default-settings.json values directly (for editing)
// For REGULAR users in preview mode: Compares user's DB settings vs defaults
func (s *Server) handleLoadHedgeDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// Load defaults from default-settings.json
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load defaults: %v", err)})
		return
	}

	modes := []string{"ultra_fast", "scalp", "swing", "position"}

	// ADMIN USER PREVIEW: Return default-settings.json values directly for editing
	if isAdmin && preview {
		var allValues []FieldComparison
		defaultValueMap := make(map[string]interface{})
		for _, mode := range modes {
			defaultMode := defaults.ModeConfigs[mode]
			if defaultMode != nil && defaultMode.Hedge != nil {
				h := defaultMode.Hedge
				allValues = append(allValues,
					FieldComparison{Path: fmt.Sprintf("%s.hedge.allow_hedge", mode), Current: h.AllowHedge, Default: h.AllowHedge, Match: true},
					FieldComparison{Path: fmt.Sprintf("%s.hedge.min_confidence_for_hedge", mode), Current: h.MinConfidenceForHedge, Default: h.MinConfidenceForHedge, Match: true},
					FieldComparison{Path: fmt.Sprintf("%s.hedge.existing_must_be_in_profit", mode), Current: h.ExistingMustBeInProfit, Default: h.ExistingMustBeInProfit, Match: true},
					FieldComparison{Path: fmt.Sprintf("%s.hedge.max_hedge_size_percent", mode), Current: h.MaxHedgeSizePercent, Default: h.MaxHedgeSizePercent, Match: true},
					FieldComparison{Path: fmt.Sprintf("%s.hedge.allow_same_mode_hedge", mode), Current: h.AllowSameModeHedge, Default: h.AllowSameModeHedge, Match: true},
					FieldComparison{Path: fmt.Sprintf("%s.hedge.max_total_exposure_multiplier", mode), Current: h.MaxTotalExposureMultiplier, Default: h.MaxTotalExposureMultiplier, Match: true},
				)
				// Add to default value map for admin editing
				defaultValueMap[mode] = map[string]interface{}{
					"allow_hedge":                   h.AllowHedge,
					"min_confidence_for_hedge":      h.MinConfidenceForHedge,
					"existing_must_be_in_profit":    h.ExistingMustBeInProfit,
					"max_hedge_size_percent":        h.MaxHedgeSizePercent,
					"allow_same_mode_hedge":         h.AllowSameModeHedge,
					"max_total_exposure_multiplier": h.MaxTotalExposureMultiplier,
				}
			}
		}
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "hedge_mode",
			AllMatch:     true,
			TotalChanges: 0,
			Differences:  []SettingDifference{},
			AllValues:    allValues,
			IsAdmin:      true,
			DefaultValue: defaultValueMap, // Raw config object for admin editing
		})
		return
	}

	// REGULAR USER: Compare user's DB settings vs defaults
	sm := autopilot.GetSettingsManager()
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
			IsAdmin:      isAdmin, // Set admin flag for frontend (Story 9.4)
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

// handleLoadScalpReentryDefaultsInternal handles scalp_reentry config reset
// ScalpReentryConfig is a Position Optimization feature, NOT a regular trading mode
// It has ~50 fields and uses a completely different structure than ModeFullConfig
// For ADMIN users in preview mode: Returns default-settings.json values directly (for editing)
// For REGULAR users in preview mode: Compares user's DB settings vs defaults
func (s *Server) handleLoadScalpReentryDefaultsInternal(c *gin.Context, userID string, preview bool, isAdmin bool) {
	ctx := c.Request.Context()

	// Get default scalp_reentry config from default-settings.json
	defaultConfig, err := autopilot.GetDefaultPositionOptimizationConfig()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default scalp_reentry settings: "+err.Error())
		return
	}

	// ADMIN USER PREVIEW: Return default-settings.json values directly for editing
	if isAdmin && preview {
		// Build all values from defaults only (no comparison)
		var allValues []FieldComparison
		addDefaultFields("scalp_reentry", reflect.ValueOf(defaultConfig).Elem(), &allValues)

		response := &ModeDiffResponse{
			Preview:      true,
			Mode:         "scalp_reentry",
			ConfigType:   "scalp_reentry", // Frontend expects config_type for admin editing
			IsAdmin:      true,
			AllMatch:     true,
			TotalChanges: 0,
			Differences:  []SettingDifference{},
			AllValues:    allValues,
			DefaultValue: defaultConfig, // Raw config object for admin editing
		}
		c.JSON(http.StatusOK, response)
		return
	}

	// REGULAR USER: Get current settings from DB for comparison
	currentConfigJSON, err := s.repo.GetUserScalpReentryConfig(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user scalp_reentry config: "+err.Error())
		return
	}

	// Parse current config (or use empty if not found)
	var currentConfig autopilot.PositionOptimizationConfig
	if currentConfigJSON != nil {
		if err := json.Unmarshal(currentConfigJSON, &currentConfig); err != nil {
			log.Printf("[SCALP-REENTRY-DEFAULTS] WARNING: Failed to parse user config, using empty: %v", err)
		}
	}

	// Compare and generate diff
	diff := compareScalpReentryConfigs(&currentConfig, defaultConfig)
	diff.IsAdmin = isAdmin

	if preview {
		c.JSON(http.StatusOK, diff)
		return
	}

	// Apply defaults to database
	configJSON, marshalErr := json.Marshal(defaultConfig)
	if marshalErr != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to serialize default scalp_reentry config: "+marshalErr.Error())
		return
	}

	if saveErr := s.repo.SaveUserScalpReentryConfig(ctx, userID, configJSON); saveErr != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save scalp_reentry config to database: "+saveErr.Error())
		return
	}

	// Trigger immediate config reload in running autopilot
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
			log.Printf("[DEFAULTS-RESET] Triggered immediate config reload for user %s scalp_reentry", userID)
		}
	}

	c.JSON(http.StatusOK, ConfigResetResult{
		Success:        true,
		ConfigType:     "scalp_reentry",
		ChangesApplied: diff.TotalChanges,
		Message:        fmt.Sprintf("Scalp re-entry optimization reset to defaults (%d changes applied)", diff.TotalChanges),
	})
}

// compareScalpReentryConfigs compares user's scalp_reentry config with defaults
func compareScalpReentryConfigs(current, defaultCfg *autopilot.PositionOptimizationConfig) *ModeDiffResponse {
	response := &ModeDiffResponse{
		Mode:        "scalp_reentry",
		Preview:     true,
		Differences: []SettingDifference{},
		AllValues:   []FieldComparison{},
		AllMatch:    true,
	}

	if current == nil || defaultCfg == nil {
		return response
	}

	// Use reflection to compare all fields
	compareSections("scalp_reentry", current, defaultCfg, response)

	response.TotalChanges = len(response.Differences)
	if response.TotalChanges > 0 {
		response.AllMatch = false
	}

	return response
}

// ====== GET ALL DEFAULT SETTINGS HANDLER (Story 9.4) ======
// This handler returns all default settings from default-settings.json
// Used by ResetSettings UI to display view-only default values

// AllDefaultSettingsResponse represents the response for getting all defaults
type AllDefaultSettingsResponse struct {
	Success  bool        `json:"success"`
	Defaults interface{} `json:"defaults"` // Full default-settings.json content
	IsAdmin  bool        `json:"is_admin"` // true if user is admin
}

// handleGetAllDefaultSettings returns all default settings from default-settings.json
// GET /api/futures/ginie/default-settings
func (s *Server) handleGetAllDefaultSettings(c *gin.Context) {
	// Get user ID to determine admin status
	_, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	isAdmin := auth.IsAdmin(c)

	// Load defaults from default-settings.json
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, AllDefaultSettingsResponse{
		Success:  true,
		Defaults: defaults,
		IsAdmin:  isAdmin,
	})
}

// ====== SAFETY SETTINGS HANDLERS (Story 9.4) ======
// Per-mode safety controls for rate limiting, profit monitoring, and win-rate monitoring

// handleLoadSafetySettingsDefaults loads default safety settings for all modes
// POST /api/futures/ginie/safety-settings/load-defaults?preview=true
func (s *Server) handleLoadSafetySettingsDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// Get user's CURRENT safety settings from DATABASE for all modes
	currentSettings, err := s.repo.GetAllUserSafetySettings(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load user safety settings: "+err.Error())
		return
	}

	// Load defaults from default-settings.json
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load defaults: "+err.Error())
		return
	}

	modes := []string{"ultra_fast", "scalp", "swing", "position"}

	// ADMIN USER PREVIEW: Return default-settings.json values directly for editing
	if isAdmin && preview {
		var allValues []FieldComparison
		defaultValueMap := make(map[string]interface{})

		for _, mode := range modes {
			var defaultSafety *autopilot.SafetySettingsMode
			if defaults.SafetySettings != nil {
				switch mode {
				case "ultra_fast":
					defaultSafety = defaults.SafetySettings.UltraFast
				case "scalp":
					defaultSafety = defaults.SafetySettings.Scalp
				case "swing":
					defaultSafety = defaults.SafetySettings.Swing
				case "position":
					defaultSafety = defaults.SafetySettings.Position
				}
			}
			// If no defaults from JSON, use hardcoded defaults
			if defaultSafety == nil {
				hardcodedDefault := database.DefaultUserSafetySettings(mode)
				defaultSafety = &autopilot.SafetySettingsMode{
					MaxTradesPerMinute:     hardcodedDefault.MaxTradesPerMinute,
					MaxTradesPerHour:       hardcodedDefault.MaxTradesPerHour,
					MaxTradesPerDay:        hardcodedDefault.MaxTradesPerDay,
					EnableProfitMonitor:    hardcodedDefault.EnableProfitMonitor,
					ProfitWindowMinutes:    hardcodedDefault.ProfitWindowMinutes,
					MaxLossPercentInWindow: hardcodedDefault.MaxLossPercentInWindow,
					PauseCooldownMinutes:   hardcodedDefault.PauseCooldownMinutes,
					EnableWinRateMonitor:   hardcodedDefault.EnableWinRateMonitor,
					WinRateSampleSize:      hardcodedDefault.WinRateSampleSize,
					MinWinRateThreshold:    hardcodedDefault.MinWinRateThreshold,
					WinRateCooldownMinutes: hardcodedDefault.WinRateCooldownMinutes,
				}
			}

			prefix := fmt.Sprintf("safety_settings.%s.", mode)
			allValues = append(allValues,
				FieldComparison{Path: prefix + "max_trades_per_minute", Current: defaultSafety.MaxTradesPerMinute, Default: defaultSafety.MaxTradesPerMinute, Match: true},
				FieldComparison{Path: prefix + "max_trades_per_hour", Current: defaultSafety.MaxTradesPerHour, Default: defaultSafety.MaxTradesPerHour, Match: true},
				FieldComparison{Path: prefix + "max_trades_per_day", Current: defaultSafety.MaxTradesPerDay, Default: defaultSafety.MaxTradesPerDay, Match: true},
				FieldComparison{Path: prefix + "enable_profit_monitor", Current: defaultSafety.EnableProfitMonitor, Default: defaultSafety.EnableProfitMonitor, Match: true},
				FieldComparison{Path: prefix + "profit_window_minutes", Current: defaultSafety.ProfitWindowMinutes, Default: defaultSafety.ProfitWindowMinutes, Match: true},
				FieldComparison{Path: prefix + "max_loss_percent_in_window", Current: defaultSafety.MaxLossPercentInWindow, Default: defaultSafety.MaxLossPercentInWindow, Match: true},
				FieldComparison{Path: prefix + "pause_cooldown_minutes", Current: defaultSafety.PauseCooldownMinutes, Default: defaultSafety.PauseCooldownMinutes, Match: true},
				FieldComparison{Path: prefix + "enable_win_rate_monitor", Current: defaultSafety.EnableWinRateMonitor, Default: defaultSafety.EnableWinRateMonitor, Match: true},
				FieldComparison{Path: prefix + "win_rate_sample_size", Current: defaultSafety.WinRateSampleSize, Default: defaultSafety.WinRateSampleSize, Match: true},
				FieldComparison{Path: prefix + "min_win_rate_threshold", Current: defaultSafety.MinWinRateThreshold, Default: defaultSafety.MinWinRateThreshold, Match: true},
				FieldComparison{Path: prefix + "win_rate_cooldown_minutes", Current: defaultSafety.WinRateCooldownMinutes, Default: defaultSafety.WinRateCooldownMinutes, Match: true},
			)
			defaultValueMap[mode] = defaultSafety
		}

		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "safety_settings",
			AllMatch:     true,
			TotalChanges: 0,
			Differences:  []SettingDifference{},
			AllValues:    allValues,
			IsAdmin:      true,
			DefaultValue: defaultValueMap, // Raw config object for admin editing
		})
		return
	}

	// REGULAR USER: Prepare response comparing user's DB settings vs defaults
	var differences []SettingDifference
	var allValues []FieldComparison

	for _, mode := range modes {
		// Get current settings for this mode (or defaults if not found)
		current := currentSettings[mode]
		if current == nil {
			current = database.DefaultUserSafetySettings(mode)
			current.UserID = userID
		}

		// Get defaults for this mode
		var defaultSafety *autopilot.SafetySettingsMode
		if defaults.SafetySettings != nil {
			switch mode {
			case "ultra_fast":
				defaultSafety = defaults.SafetySettings.UltraFast
			case "scalp":
				defaultSafety = defaults.SafetySettings.Scalp
			case "swing":
				defaultSafety = defaults.SafetySettings.Swing
			case "position":
				defaultSafety = defaults.SafetySettings.Position
			}
		}

		// If no defaults from JSON, use hardcoded defaults
		if defaultSafety == nil {
			hardcodedDefault := database.DefaultUserSafetySettings(mode)
			defaultSafety = &autopilot.SafetySettingsMode{
				MaxTradesPerMinute:     hardcodedDefault.MaxTradesPerMinute,
				MaxTradesPerHour:       hardcodedDefault.MaxTradesPerHour,
				MaxTradesPerDay:        hardcodedDefault.MaxTradesPerDay,
				EnableProfitMonitor:    hardcodedDefault.EnableProfitMonitor,
				ProfitWindowMinutes:    hardcodedDefault.ProfitWindowMinutes,
				MaxLossPercentInWindow: hardcodedDefault.MaxLossPercentInWindow,
				PauseCooldownMinutes:   hardcodedDefault.PauseCooldownMinutes,
				EnableWinRateMonitor:   hardcodedDefault.EnableWinRateMonitor,
				WinRateSampleSize:      hardcodedDefault.WinRateSampleSize,
				MinWinRateThreshold:    hardcodedDefault.MinWinRateThreshold,
				WinRateCooldownMinutes: hardcodedDefault.WinRateCooldownMinutes,
			}
		}

		// Compare fields
		prefix := fmt.Sprintf("safety_settings.%s.", mode)

		addSafetyField := func(name string, currentVal, defaultVal interface{}, riskLevel, impact string) {
			match := reflect.DeepEqual(currentVal, defaultVal)
			allValues = append(allValues, FieldComparison{
				Path:      prefix + name,
				Current:   currentVal,
				Default:   defaultVal,
				Match:     match,
				RiskLevel: riskLevel,
			})
			if !match {
				differences = append(differences, SettingDifference{
					Path:           prefix + name,
					Current:        currentVal,
					Default:        defaultVal,
					RiskLevel:      riskLevel,
					Impact:         impact,
					Recommendation: "Use default for balanced safety",
				})
			}
		}

		addSafetyField("max_trades_per_minute", current.MaxTradesPerMinute, defaultSafety.MaxTradesPerMinute, "medium", "Rate limit per minute")
		addSafetyField("max_trades_per_hour", current.MaxTradesPerHour, defaultSafety.MaxTradesPerHour, "medium", "Rate limit per hour")
		addSafetyField("max_trades_per_day", current.MaxTradesPerDay, defaultSafety.MaxTradesPerDay, "medium", "Rate limit per day")
		addSafetyField("enable_profit_monitor", current.EnableProfitMonitor, defaultSafety.EnableProfitMonitor, "high", "Profit loss protection")
		addSafetyField("profit_window_minutes", current.ProfitWindowMinutes, defaultSafety.ProfitWindowMinutes, "low", "Profit monitoring window")
		addSafetyField("max_loss_percent_in_window", current.MaxLossPercentInWindow, defaultSafety.MaxLossPercentInWindow, "high", "Max allowed loss")
		addSafetyField("pause_cooldown_minutes", current.PauseCooldownMinutes, defaultSafety.PauseCooldownMinutes, "medium", "Pause duration after loss")
		addSafetyField("enable_win_rate_monitor", current.EnableWinRateMonitor, defaultSafety.EnableWinRateMonitor, "high", "Win rate protection")
		addSafetyField("win_rate_sample_size", current.WinRateSampleSize, defaultSafety.WinRateSampleSize, "low", "Win rate sample size")
		addSafetyField("min_win_rate_threshold", current.MinWinRateThreshold, defaultSafety.MinWinRateThreshold, "high", "Min required win rate")
		addSafetyField("win_rate_cooldown_minutes", current.WinRateCooldownMinutes, defaultSafety.WinRateCooldownMinutes, "medium", "Cooldown after low win rate")
	}

	if preview {
		c.JSON(http.StatusOK, ConfigResetPreview{
			Preview:      true,
			ConfigType:   "safety_settings",
			AllMatch:     len(differences) == 0,
			TotalChanges: len(differences),
			Differences:  differences,
			AllValues:    allValues,
			IsAdmin:      isAdmin,
		})
		return
	}

	// Apply defaults from default-settings.json to database - reset all modes
	for _, mode := range modes {
		// Get defaults from default-settings.json (same source used for comparison)
		var defaultSafety *autopilot.SafetySettingsMode
		if defaults.SafetySettings != nil {
			switch mode {
			case "ultra_fast":
				defaultSafety = defaults.SafetySettings.UltraFast
			case "scalp":
				defaultSafety = defaults.SafetySettings.Scalp
			case "swing":
				defaultSafety = defaults.SafetySettings.Swing
			case "position":
				defaultSafety = defaults.SafetySettings.Position
			}
		}

		// Create database settings from default-settings.json values
		defaultSettings := &database.UserSafetySettings{
			UserID: userID,
			Mode:   mode,
		}

		if defaultSafety != nil {
			// Use values from default-settings.json
			defaultSettings.MaxTradesPerMinute = defaultSafety.MaxTradesPerMinute
			defaultSettings.MaxTradesPerHour = defaultSafety.MaxTradesPerHour
			defaultSettings.MaxTradesPerDay = defaultSafety.MaxTradesPerDay
			defaultSettings.EnableProfitMonitor = defaultSafety.EnableProfitMonitor
			defaultSettings.ProfitWindowMinutes = defaultSafety.ProfitWindowMinutes
			defaultSettings.MaxLossPercentInWindow = defaultSafety.MaxLossPercentInWindow
			defaultSettings.PauseCooldownMinutes = defaultSafety.PauseCooldownMinutes
			defaultSettings.EnableWinRateMonitor = defaultSafety.EnableWinRateMonitor
			defaultSettings.WinRateSampleSize = defaultSafety.WinRateSampleSize
			defaultSettings.MinWinRateThreshold = defaultSafety.MinWinRateThreshold
			defaultSettings.WinRateCooldownMinutes = defaultSafety.WinRateCooldownMinutes
		} else {
			// Fallback to hardcoded defaults only if not in default-settings.json
			hardcoded := database.DefaultUserSafetySettings(mode)
			defaultSettings.MaxTradesPerMinute = hardcoded.MaxTradesPerMinute
			defaultSettings.MaxTradesPerHour = hardcoded.MaxTradesPerHour
			defaultSettings.MaxTradesPerDay = hardcoded.MaxTradesPerDay
			defaultSettings.EnableProfitMonitor = hardcoded.EnableProfitMonitor
			defaultSettings.ProfitWindowMinutes = hardcoded.ProfitWindowMinutes
			defaultSettings.MaxLossPercentInWindow = hardcoded.MaxLossPercentInWindow
			defaultSettings.PauseCooldownMinutes = hardcoded.PauseCooldownMinutes
			defaultSettings.EnableWinRateMonitor = hardcoded.EnableWinRateMonitor
			defaultSettings.WinRateSampleSize = hardcoded.WinRateSampleSize
			defaultSettings.MinWinRateThreshold = hardcoded.MinWinRateThreshold
			defaultSettings.WinRateCooldownMinutes = hardcoded.WinRateCooldownMinutes
		}

		if err := s.repo.SaveUserSafetySettings(ctx, defaultSettings); err != nil {
			log.Printf("[SAFETY-SETTINGS] Warning: Failed to reset %s safety settings: %v", mode, err)
		}
	}

	// Trigger config reload
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
			log.Printf("[SAFETY-SETTINGS] Triggered config reload for user %s", userID)
		}
	}

	c.JSON(http.StatusOK, ConfigResetResult{
		Success:        true,
		ConfigType:     "safety_settings",
		ChangesApplied: len(differences),
		Message:        fmt.Sprintf("Safety settings reset to defaults for all modes (%d changes applied)", len(differences)),
	})
}

// handleGetUserSafetySettings returns user's current safety settings
// GET /api/futures/ginie/safety-settings
func (s *Server) handleGetUserSafetySettings(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	// Get all user safety settings from database
	settings, err := s.repo.GetAllUserSafetySettings(ctx, userID)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load safety settings: "+err.Error())
		return
	}

	// If no settings exist, return defaults
	modes := []string{"ultra_fast", "scalp", "swing", "position"}
	result := make(map[string]*database.UserSafetySettings)

	for _, mode := range modes {
		if existing := settings[mode]; existing != nil {
			result[mode] = existing
		} else {
			result[mode] = database.DefaultUserSafetySettings(mode)
			result[mode].UserID = userID
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"settings": result,
	})
}

// handleUpdateUserSafetySettings updates user's safety settings for a specific mode
// PUT /api/futures/ginie/safety-settings/:mode
func (s *Server) handleUpdateUserSafetySettings(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	mode := c.Param("mode")
	validModes := map[string]bool{"ultra_fast": true, "scalp": true, "swing": true, "position": true}
	if !validModes[mode] {
		errorResponse(c, http.StatusBadRequest, "Invalid mode. Valid modes: ultra_fast, scalp, swing, position")
		return
	}

	var req database.UserSafetySettings
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Validate fields
	if req.MaxTradesPerMinute < 1 || req.MaxTradesPerMinute > 100 {
		errorResponse(c, http.StatusBadRequest, "max_trades_per_minute must be 1-100")
		return
	}
	if req.MaxTradesPerHour < 1 || req.MaxTradesPerHour > 1000 {
		errorResponse(c, http.StatusBadRequest, "max_trades_per_hour must be 1-1000")
		return
	}
	if req.MaxTradesPerDay < 1 || req.MaxTradesPerDay > 10000 {
		errorResponse(c, http.StatusBadRequest, "max_trades_per_day must be 1-10000")
		return
	}
	if req.MaxLossPercentInWindow > 0 || req.MaxLossPercentInWindow < -100 {
		errorResponse(c, http.StatusBadRequest, "max_loss_percent_in_window must be -100 to 0 (negative value)")
		return
	}
	if req.MinWinRateThreshold < 0 || req.MinWinRateThreshold > 100 {
		errorResponse(c, http.StatusBadRequest, "min_win_rate_threshold must be 0-100")
		return
	}

	// Set user ID and mode
	req.UserID = userID
	req.Mode = mode

	ctx := c.Request.Context()
	if err := s.repo.SaveUserSafetySettings(ctx, &req); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to save safety settings: "+err.Error())
		return
	}

	// Trigger config reload
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
		}
	}

	log.Printf("[SAFETY-SETTINGS] Updated %s safety settings for user %s", mode, userID)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"mode":     mode,
		"settings": req,
		"message":  fmt.Sprintf("%s safety settings updated", mode),
	})
}

// ====== POSITION OPTIMIZATION LOAD DEFAULTS HANDLER ======
// Dedicated endpoint for position optimization (scalp_reentry) settings
// POST /api/futures/ginie/position-optimization/load-defaults?preview=true

// handleLoadPositionOptimizationDefaults resets position optimization (scalp_reentry) to default values
// POST /api/futures/ginie/position-optimization/load-defaults?preview=true
// For ADMIN users in preview mode: Returns default-settings.json values directly (for editing)
// For REGULAR users in preview mode: Compares user's DB settings vs defaults
func (s *Server) handleLoadPositionOptimizationDefaults(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	isAdmin := auth.IsAdmin(c)

	// Reuse the internal handler for scalp_reentry
	s.handleLoadScalpReentryDefaultsInternal(c, userID, preview, isAdmin)
}

// ====== BATCH RESET HANDLERS ======
// These handlers allow resetting multiple config sections at once

// BatchResetResult represents the response for batch reset operations
type BatchResetResult struct {
	Success        bool                   `json:"success"`
	ConfigType     string                 `json:"config_type"`
	ChangesApplied int                    `json:"changes_applied"`
	Results        map[string]interface{} `json:"results"`
	Message        string                 `json:"message"`
}

// handleResetAllModes resets all 4 trading modes to defaults
// POST /api/futures/ginie/modes/reset-all?preview=true
func (s *Server) handleResetAllModes(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// Get all default mode configs from default-settings.json
	defaultModes, err := autopilot.GetAllDefaultModeFullConfigs()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load default settings: "+err.Error())
		return
	}

	modeNames := []string{"ultra_fast", "scalp", "swing", "position"}

	// ADMIN USER PREVIEW: Return default-settings.json values directly
	if isAdmin && preview {
		allDefaultsMap := make(map[string]interface{})
		for _, modeName := range modeNames {
			if defaultModes[modeName] != nil {
				allDefaultsMap[modeName] = defaultModes[modeName]
			}
		}

		response := &AllModesDiffResponse{
			Preview:      true,
			ConfigType:   "all_modes",
			IsAdmin:      true,
			AllMatch:     true,
			TotalChanges: 0,
			Modes:        make(map[string]*ModeDiffResponse),
			DefaultValue: allDefaultsMap,
		}

		for _, modeName := range modeNames {
			defaultMode := defaultModes[modeName]
			if defaultMode != nil {
				response.Modes[modeName] = &ModeDiffResponse{
					Preview:      true,
					Mode:         modeName,
					ConfigType:   modeName,
					IsAdmin:      true,
					AllMatch:     true,
					TotalChanges: 0,
					Differences:  []SettingDifference{},
					AllValues:    buildAllValuesFromDefaults(modeName, defaultMode),
					DefaultValue: defaultMode,
				}
			}
		}

		c.JSON(http.StatusOK, response)
		return
	}

	// REGULAR USER: Compare user's DB settings vs defaults
	sm := autopilot.GetSettingsManager()
	response := &AllModesDiffResponse{
		Preview: preview,
		Modes:   make(map[string]*ModeDiffResponse),
		IsAdmin: isAdmin,
	}

	totalChanges := 0
	allMatch := true

	for _, modeName := range modeNames {
		currentMode, dbErr := sm.GetUserModeConfigFromDB(ctx, s.repo, userID, modeName)
		if dbErr != nil {
			continue
		}

		defaultMode := defaultModes[modeName]
		if currentMode != nil && defaultMode != nil {
			diff := compareModeConfigs(modeName, currentMode, defaultMode)
			diff.IsAdmin = isAdmin
			response.Modes[modeName] = diff
			totalChanges += diff.TotalChanges
			if !diff.AllMatch {
				allMatch = false
			}
		}
	}

	response.TotalChanges = totalChanges
	response.AllMatch = allMatch
	response.ConfigType = "all_modes"

	if preview {
		c.JSON(http.StatusOK, response)
		return
	}

	// Apply defaults to all modes
	modesApplied := 0
	for _, modeName := range modeNames {
		defaultMode := defaultModes[modeName]
		if defaultMode == nil {
			continue
		}

		configJSON, marshalErr := json.Marshal(defaultMode)
		if marshalErr != nil {
			errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to serialize %s config: %v", modeName, marshalErr))
			return
		}

		if saveErr := s.repo.SaveUserModeConfig(ctx, userID, modeName, defaultMode.Enabled, configJSON); saveErr != nil {
			errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to save %s settings: %v", modeName, saveErr))
			return
		}
		modesApplied++
	}

	// Trigger immediate config reload
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
			log.Printf("[DEFAULTS-RESET] Triggered config reload for all modes for user %s", userID)
		}
	}

	c.JSON(http.StatusOK, BatchResetResult{
		Success:        true,
		ConfigType:     "all_modes",
		ChangesApplied: totalChanges,
		Results: map[string]interface{}{
			"modes_reset": modesApplied,
		},
		Message: fmt.Sprintf("All %d trading modes reset to defaults (%d changes applied)", modesApplied, totalChanges),
	})
}

// handleResetAllOtherSettings resets all "other settings" (non-mode settings) to defaults
// POST /api/futures/ginie/other-settings/reset-all?preview=true
// Resets: circuit_breaker, llm_config, capital_allocation, position_optimization
func (s *Server) handleResetAllOtherSettings(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	preview := c.Query("preview") == "true"
	ctx := c.Request.Context()
	isAdmin := auth.IsAdmin(c)

	// Load defaults from default-settings.json
	defaults, err := autopilot.LoadDefaultSettings()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to load defaults: "+err.Error())
		return
	}

	// Response structure for preview
	type OtherSettingsPreview struct {
		Preview      bool                              `json:"preview"`
		ConfigType   string                            `json:"config_type"`
		TotalChanges int                               `json:"total_changes"`
		AllMatch     bool                              `json:"all_match"`
		Sections     map[string]*ConfigResetPreview    `json:"sections"`
		IsAdmin      bool                              `json:"is_admin"`
		DefaultValue map[string]interface{}            `json:"default_value,omitempty"`
	}

	response := &OtherSettingsPreview{
		Preview:    preview,
		ConfigType: "other_settings",
		Sections:   make(map[string]*ConfigResetPreview),
		IsAdmin:    isAdmin,
	}

	totalChanges := 0
	allMatch := true

	// ===== 1. Circuit Breaker =====
	cbPreview := s.buildCircuitBreakerPreview(ctx, userID, defaults, isAdmin)
	response.Sections["circuit_breaker"] = cbPreview
	totalChanges += cbPreview.TotalChanges
	if !cbPreview.AllMatch {
		allMatch = false
	}

	// ===== 2. LLM Config =====
	llmPreview := s.buildLLMConfigPreview(ctx, userID, defaults, isAdmin)
	response.Sections["llm_config"] = llmPreview
	totalChanges += llmPreview.TotalChanges
	if !llmPreview.AllMatch {
		allMatch = false
	}

	// ===== 3. Capital Allocation =====
	caPreview := s.buildCapitalAllocationPreview(ctx, userID, defaults, isAdmin)
	response.Sections["capital_allocation"] = caPreview
	totalChanges += caPreview.TotalChanges
	if !caPreview.AllMatch {
		allMatch = false
	}

	// ===== 4. Position Optimization (scalp_reentry) =====
	poPreview := s.buildPositionOptimizationPreview(ctx, userID, isAdmin)
	response.Sections["position_optimization"] = poPreview
	totalChanges += poPreview.TotalChanges
	if !poPreview.AllMatch {
		allMatch = false
	}

	response.TotalChanges = totalChanges
	response.AllMatch = allMatch

	// ADMIN preview - add default values for editing
	if isAdmin && preview {
		defaultConfig, _ := autopilot.GetDefaultPositionOptimizationConfig()
		response.DefaultValue = map[string]interface{}{
			"circuit_breaker":       defaults.CircuitBreaker.Global,
			"llm_config":            defaults.LLMConfig.Global,
			"capital_allocation":    defaults.CapitalAllocation,
			"position_optimization": defaultConfig,
		}
	}

	if preview {
		c.JSON(http.StatusOK, response)
		return
	}

	// ===== Apply all defaults =====
	results := make(map[string]interface{})

	// 1. Reset Circuit Breaker
	if err := s.applyCircuitBreakerDefaults(ctx, userID, defaults); err != nil {
		results["circuit_breaker"] = map[string]interface{}{"success": false, "error": err.Error()}
	} else {
		results["circuit_breaker"] = map[string]interface{}{"success": true, "changes": cbPreview.TotalChanges}
	}

	// 2. Reset LLM Config
	if err := s.applyLLMConfigDefaults(ctx, userID, defaults); err != nil {
		results["llm_config"] = map[string]interface{}{"success": false, "error": err.Error()}
	} else {
		results["llm_config"] = map[string]interface{}{"success": true, "changes": llmPreview.TotalChanges}
	}

	// 3. Reset Capital Allocation
	if err := s.applyCapitalAllocationDefaults(ctx, userID, defaults); err != nil {
		results["capital_allocation"] = map[string]interface{}{"success": false, "error": err.Error()}
	} else {
		results["capital_allocation"] = map[string]interface{}{"success": true, "changes": caPreview.TotalChanges}
	}

	// 4. Reset Position Optimization
	if err := s.applyPositionOptimizationDefaults(ctx, userID); err != nil {
		results["position_optimization"] = map[string]interface{}{"success": false, "error": err.Error()}
	} else {
		results["position_optimization"] = map[string]interface{}{"success": true, "changes": poPreview.TotalChanges}
	}

	// Trigger immediate config reload
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			instance.Autopilot.TriggerConfigReload()
			log.Printf("[DEFAULTS-RESET] Triggered config reload for all other settings for user %s", userID)
		}
	}

	c.JSON(http.StatusOK, BatchResetResult{
		Success:        true,
		ConfigType:     "other_settings",
		ChangesApplied: totalChanges,
		Results:        results,
		Message:        fmt.Sprintf("All other settings reset to defaults (%d changes applied)", totalChanges),
	})
}

// ===== Helper functions for building previews =====

func (s *Server) buildCircuitBreakerPreview(ctx context.Context, userID string, defaults *autopilot.DefaultSettingsFile, isAdmin bool) *ConfigResetPreview {
	preview := &ConfigResetPreview{
		Preview:     true,
		ConfigType:  "circuit_breaker",
		IsAdmin:     isAdmin,
		Differences: []SettingDifference{},
		AllValues:   []FieldComparison{},
	}

	if isAdmin {
		preview.AllValues = []FieldComparison{
			{Path: "circuit_breaker.enabled", Current: defaults.CircuitBreaker.Global.Enabled, Default: defaults.CircuitBreaker.Global.Enabled, Match: true},
			{Path: "circuit_breaker.max_loss_per_hour", Current: defaults.CircuitBreaker.Global.MaxLossPerHour, Default: defaults.CircuitBreaker.Global.MaxLossPerHour, Match: true},
			{Path: "circuit_breaker.max_daily_loss", Current: defaults.CircuitBreaker.Global.MaxDailyLoss, Default: defaults.CircuitBreaker.Global.MaxDailyLoss, Match: true},
			{Path: "circuit_breaker.max_consecutive_losses", Current: defaults.CircuitBreaker.Global.MaxConsecutiveLosses, Default: defaults.CircuitBreaker.Global.MaxConsecutiveLosses, Match: true},
			{Path: "circuit_breaker.cooldown_minutes", Current: defaults.CircuitBreaker.Global.CooldownMinutes, Default: defaults.CircuitBreaker.Global.CooldownMinutes, Match: true},
		}
		preview.AllMatch = true
		preview.DefaultValue = defaults.CircuitBreaker.Global
		return preview
	}

	currentCB, _ := s.repo.GetUserGlobalCircuitBreaker(ctx, userID)
	if currentCB == nil {
		currentCB = &database.UserGlobalCircuitBreaker{
			UserID:               userID,
			MaxLossPerHour:       100.0,
			MaxDailyLoss:         300.0,
			MaxConsecutiveLosses: 3,
			CooldownMinutes:      30,
		}
	}

	addField := func(path string, current, defaultVal interface{}) {
		match := reflect.DeepEqual(current, defaultVal)
		preview.AllValues = append(preview.AllValues, FieldComparison{
			Path:    path,
			Current: current,
			Default: defaultVal,
			Match:   match,
		})
		if !match {
			preview.Differences = append(preview.Differences, SettingDifference{
				Path:      path,
				Current:   current,
				Default:   defaultVal,
				RiskLevel: "high",
			})
		}
	}

	addField("circuit_breaker.max_loss_per_hour", currentCB.MaxLossPerHour, defaults.CircuitBreaker.Global.MaxLossPerHour)
	addField("circuit_breaker.max_daily_loss", currentCB.MaxDailyLoss, defaults.CircuitBreaker.Global.MaxDailyLoss)
	addField("circuit_breaker.max_consecutive_losses", currentCB.MaxConsecutiveLosses, defaults.CircuitBreaker.Global.MaxConsecutiveLosses)
	addField("circuit_breaker.cooldown_minutes", currentCB.CooldownMinutes, defaults.CircuitBreaker.Global.CooldownMinutes)

	preview.TotalChanges = len(preview.Differences)
	preview.AllMatch = len(preview.Differences) == 0
	return preview
}

func (s *Server) buildLLMConfigPreview(ctx context.Context, userID string, defaults *autopilot.DefaultSettingsFile, isAdmin bool) *ConfigResetPreview {
	preview := &ConfigResetPreview{
		Preview:     true,
		ConfigType:  "llm_config",
		IsAdmin:     isAdmin,
		Differences: []SettingDifference{},
		AllValues:   []FieldComparison{},
	}

	if isAdmin {
		preview.AllValues = []FieldComparison{
			{Path: "llm_config.provider", Current: defaults.LLMConfig.Global.Provider, Default: defaults.LLMConfig.Global.Provider, Match: true},
			{Path: "llm_config.model", Current: defaults.LLMConfig.Global.Model, Default: defaults.LLMConfig.Global.Model, Match: true},
			{Path: "llm_config.timeout_ms", Current: defaults.LLMConfig.Global.TimeoutMS, Default: defaults.LLMConfig.Global.TimeoutMS, Match: true},
			{Path: "llm_config.retry_count", Current: defaults.LLMConfig.Global.RetryCount, Default: defaults.LLMConfig.Global.RetryCount, Match: true},
		}
		preview.AllMatch = true
		preview.DefaultValue = defaults.LLMConfig.Global
		return preview
	}

	currentLLM, _ := s.repo.GetUserLLMConfig(ctx, userID)
	if currentLLM == nil {
		currentLLM = database.DefaultUserLLMConfig()
		currentLLM.UserID = userID
	}

	addField := func(path string, current, defaultVal interface{}) {
		match := reflect.DeepEqual(current, defaultVal)
		preview.AllValues = append(preview.AllValues, FieldComparison{
			Path:    path,
			Current: current,
			Default: defaultVal,
			Match:   match,
		})
		if !match {
			preview.Differences = append(preview.Differences, SettingDifference{
				Path:      path,
				Current:   current,
				Default:   defaultVal,
				RiskLevel: "low",
			})
		}
	}

	addField("llm_config.provider", currentLLM.Provider, defaults.LLMConfig.Global.Provider)
	addField("llm_config.model", currentLLM.Model, defaults.LLMConfig.Global.Model)
	addField("llm_config.timeout_ms", currentLLM.TimeoutMs, defaults.LLMConfig.Global.TimeoutMS)
	addField("llm_config.retry_count", currentLLM.RetryCount, defaults.LLMConfig.Global.RetryCount)

	preview.TotalChanges = len(preview.Differences)
	preview.AllMatch = len(preview.Differences) == 0
	return preview
}

func (s *Server) buildCapitalAllocationPreview(ctx context.Context, userID string, defaults *autopilot.DefaultSettingsFile, isAdmin bool) *ConfigResetPreview {
	preview := &ConfigResetPreview{
		Preview:     true,
		ConfigType:  "capital_allocation",
		IsAdmin:     isAdmin,
		Differences: []SettingDifference{},
		AllValues:   []FieldComparison{},
	}

	jsonAlloc := defaults.CapitalAllocation

	if isAdmin {
		preview.AllValues = []FieldComparison{
			{Path: "ultra_fast_percent", Current: jsonAlloc.UltraFastPercent, Default: jsonAlloc.UltraFastPercent, Match: true},
			{Path: "scalp_percent", Current: jsonAlloc.ScalpPercent, Default: jsonAlloc.ScalpPercent, Match: true},
			{Path: "swing_percent", Current: jsonAlloc.SwingPercent, Default: jsonAlloc.SwingPercent, Match: true},
			{Path: "position_percent", Current: jsonAlloc.PositionPercent, Default: jsonAlloc.PositionPercent, Match: true},
			{Path: "allow_dynamic_rebalance", Current: jsonAlloc.AllowDynamicRebalance, Default: jsonAlloc.AllowDynamicRebalance, Match: true},
			{Path: "rebalance_threshold_pct", Current: jsonAlloc.RebalanceThresholdPct, Default: jsonAlloc.RebalanceThresholdPct, Match: true},
		}
		preview.AllMatch = true
		preview.DefaultValue = map[string]interface{}{
			"ultra_fast_percent":      jsonAlloc.UltraFastPercent,
			"scalp_percent":           jsonAlloc.ScalpPercent,
			"swing_percent":           jsonAlloc.SwingPercent,
			"position_percent":        jsonAlloc.PositionPercent,
			"allow_dynamic_rebalance": jsonAlloc.AllowDynamicRebalance,
			"rebalance_threshold_pct": jsonAlloc.RebalanceThresholdPct,
		}
		return preview
	}

	// Story 6.5: Cache-first pattern (with silent fallback to DB for preview)
	var currentAlloc *database.UserCapitalAllocation
	if s.settingsCacheService != nil {
		currentAlloc, _ = s.settingsCacheService.GetCapitalAllocation(ctx, userID)
	}
	if currentAlloc == nil {
		currentAlloc, _ = s.repo.GetUserCapitalAllocation(ctx, userID)
	}
	if currentAlloc == nil {
		currentAlloc = database.DefaultUserCapitalAllocation()
		currentAlloc.UserID = userID
	}

	addField := func(path string, current, defaultVal interface{}) {
		match := reflect.DeepEqual(current, defaultVal)
		preview.AllValues = append(preview.AllValues, FieldComparison{
			Path:    path,
			Current: current,
			Default: defaultVal,
			Match:   match,
		})
		if !match {
			preview.Differences = append(preview.Differences, SettingDifference{
				Path:      path,
				Current:   current,
				Default:   defaultVal,
				RiskLevel: "medium",
			})
		}
	}

	addField("capital_allocation.ultra_fast_percent", currentAlloc.UltraFastPercent, jsonAlloc.UltraFastPercent)
	addField("capital_allocation.scalp_percent", currentAlloc.ScalpPercent, jsonAlloc.ScalpPercent)
	addField("capital_allocation.swing_percent", currentAlloc.SwingPercent, jsonAlloc.SwingPercent)
	addField("capital_allocation.position_percent", currentAlloc.PositionPercent, jsonAlloc.PositionPercent)
	addField("capital_allocation.allow_dynamic_rebalance", currentAlloc.AllowDynamicRebalance, jsonAlloc.AllowDynamicRebalance)
	addField("capital_allocation.rebalance_threshold_pct", currentAlloc.RebalanceThresholdPct, jsonAlloc.RebalanceThresholdPct)

	preview.TotalChanges = len(preview.Differences)
	preview.AllMatch = len(preview.Differences) == 0
	return preview
}

func (s *Server) buildPositionOptimizationPreview(ctx context.Context, userID string, isAdmin bool) *ConfigResetPreview {
	preview := &ConfigResetPreview{
		Preview:     true,
		ConfigType:  "position_optimization",
		IsAdmin:     isAdmin,
		Differences: []SettingDifference{},
		AllValues:   []FieldComparison{},
	}

	defaultConfig, err := autopilot.GetDefaultPositionOptimizationConfig()
	if err != nil {
		return preview
	}

	if isAdmin {
		addDefaultFields("position_optimization", reflect.ValueOf(defaultConfig).Elem(), &preview.AllValues)
		preview.AllMatch = true
		preview.DefaultValue = defaultConfig
		return preview
	}

	currentConfigJSON, _ := s.repo.GetUserScalpReentryConfig(ctx, userID)
	var currentConfig autopilot.PositionOptimizationConfig
	if currentConfigJSON != nil {
		json.Unmarshal(currentConfigJSON, &currentConfig)
	}

	diff := compareScalpReentryConfigs(&currentConfig, defaultConfig)
	preview.Differences = diff.Differences
	preview.AllValues = diff.AllValues
	preview.TotalChanges = diff.TotalChanges
	preview.AllMatch = diff.AllMatch
	return preview
}

// ===== Helper functions for applying defaults =====

func (s *Server) applyCircuitBreakerDefaults(ctx context.Context, userID string, defaults *autopilot.DefaultSettingsFile) error {
	currentCB, _ := s.repo.GetUserGlobalCircuitBreaker(ctx, userID)
	if currentCB == nil {
		currentCB = &database.UserGlobalCircuitBreaker{UserID: userID}
	}

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

	return s.repo.SaveUserGlobalCircuitBreaker(ctx, updatedCB)
}

func (s *Server) applyLLMConfigDefaults(ctx context.Context, userID string, defaults *autopilot.DefaultSettingsFile) error {
	currentLLM, _ := s.repo.GetUserLLMConfig(ctx, userID)
	if currentLLM == nil {
		currentLLM = &database.UserLLMConfig{UserID: userID}
	}

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

	return s.repo.SaveUserLLMConfig(ctx, updatedLLM)
}

func (s *Server) applyCapitalAllocationDefaults(ctx context.Context, userID string, defaults *autopilot.DefaultSettingsFile) error {
	// Story 6.5: Cache-first pattern (with silent fallback to DB)
	var currentAlloc *database.UserCapitalAllocation
	if s.settingsCacheService != nil {
		currentAlloc, _ = s.settingsCacheService.GetCapitalAllocation(ctx, userID)
	}
	if currentAlloc == nil {
		currentAlloc, _ = s.repo.GetUserCapitalAllocation(ctx, userID)
	}
	if currentAlloc == nil {
		currentAlloc = database.DefaultUserCapitalAllocation()
		currentAlloc.UserID = userID
	}

	jsonAlloc := defaults.CapitalAllocation
	currentAlloc.UltraFastPercent = jsonAlloc.UltraFastPercent
	currentAlloc.ScalpPercent = jsonAlloc.ScalpPercent
	currentAlloc.SwingPercent = jsonAlloc.SwingPercent
	currentAlloc.PositionPercent = jsonAlloc.PositionPercent
	currentAlloc.AllowDynamicRebalance = jsonAlloc.AllowDynamicRebalance
	currentAlloc.RebalanceThresholdPct = jsonAlloc.RebalanceThresholdPct

	// Story 6.5: Use cache service for write-through pattern
	if s.settingsCacheService != nil {
		return s.settingsCacheService.UpdateCapitalAllocation(ctx, userID, currentAlloc)
	}
	return s.repo.SaveUserCapitalAllocation(ctx, currentAlloc)
}

func (s *Server) applyPositionOptimizationDefaults(ctx context.Context, userID string) error {
	defaultConfig, err := autopilot.GetDefaultPositionOptimizationConfig()
	if err != nil {
		return err
	}

	configJSON, err := json.Marshal(defaultConfig)
	if err != nil {
		return err
	}

	return s.repo.SaveUserScalpReentryConfig(ctx, userID, configJSON)
}
