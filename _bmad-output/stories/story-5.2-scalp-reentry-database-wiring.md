# Story 5.2: Wire Scalp Reentry Config to Database

**Story ID:** SCALP-5.2
**Epic:** Epic 5 - Scalp Reentry Mode Database Integration
**Priority:** P0 (Critical - Foundation for Per-User Customization)
**Estimated Effort:** 8 hours
**Author:** BMAD Agent (Bob - Scrum Master)
**Status:** Ready for Development
**Depends On:** Story 4.1, Story 4.2 (Database-First Mode Configuration System)

---

## Problem Statement

### Current State

- Scalp Reentry configuration loaded from static `autopilot_settings.json` file
- All users share the same configuration (36+ settings)
- No way to customize scalp reentry settings per user
- Configuration stored in `internal/autopilot/settings.go` as file-based
- No API endpoints for scalp_reentry config CRUD operations
- Settings hardcoded in `DefaultScalpReentryConfig()` function

### Expected Behavior

- Scalp Reentry configuration loaded from `user_mode_configs` database table
- Each user can have completely different scalp reentry settings
- Settings persist across sessions in PostgreSQL
- Fallback to system defaults if no user config exists
- Full CRUD API endpoints for scalp_reentry configuration
- Migration path from JSON file to database

---

## User Story

> As a trader using scalp reentry mode,
> I want my scalp reentry settings (TP levels, hedge mode, DCA, etc.) stored in the database,
> So that I can customize all 36+ settings to my trading style and have them persist across sessions.

---

## Technical Architecture

### Configuration Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│  BEFORE (Story 5.1)                                                 │
│  ┌──────────────────────┐                                           │
│  │ autopilot_settings.  │ → SettingsManager.LoadSettings()          │
│  │ json (static file)   │   ↓                                       │
│  └──────────────────────┘   ScalpReentryConfig (in-memory)          │
│                              - All users share same config           │
│                              - No persistence of changes             │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│  AFTER (Story 5.2)                                                  │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │ PostgreSQL: user_mode_configs                                │   │
│  │ ┌────────┬─────────┬─────────────┬─────────┬──────────────┐ │   │
│  │ │ user_id│mode_name│   enabled   │config_  │  updated_at  │ │   │
│  │ ├────────┼─────────┼─────────────┼─────────┼──────────────┤ │   │
│  │ │ user1  │scalp_   │    true     │ {json}  │ 2026-01-06   │ │   │
│  │ │        │reentry  │             │         │              │ │   │
│  │ │ user2  │scalp_   │    false    │ {json}  │ 2026-01-05   │ │   │
│  │ │        │reentry  │             │         │              │ │   │
│  │ └────────┴─────────┴─────────────┴─────────┴──────────────┘ │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                              ↓                                      │
│  SettingsManager.GetScalpReentryConfig(userID)                      │
│  ↓                                                                   │
│  1. Load from database (if exists)                                  │
│  2. Fallback to DefaultScalpReentryConfig() if not found            │
│  3. Return user-specific config                                     │
│                                                                      │
│  User1 config: TP1=0.4%, HedgeMode=true, DCA=true                   │
│  User2 config: TP1=0.3%, HedgeMode=false, DCA=false                 │
└─────────────────────────────────────────────────────────────────────┘
```

### Database Schema

```sql
-- Existing table from Story 4.1 (already created)
CREATE TABLE user_mode_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode_name VARCHAR(50) NOT NULL,  -- 'scalp_reentry' for this story
    enabled BOOLEAN NOT NULL DEFAULT false,
    config_json JSONB NOT NULL,      -- ScalpReentryConfig serialized to JSON
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, mode_name)
);
```

### Configuration JSON Structure

```json
{
  "enabled": true,

  "tp1_percent": 0.4,
  "tp1_sell_percent": 30,
  "tp2_percent": 0.7,
  "tp2_sell_percent": 50,
  "tp3_percent": 1.0,
  "tp3_sell_percent": 80,

  "reentry_percent": 80,
  "reentry_price_buffer": 0.05,
  "max_reentry_attempts": 3,
  "reentry_timeout_sec": 300,

  "final_trailing_percent": 5.0,
  "final_hold_min_percent": 20,

  "dynamic_sl_max_loss_pct": 40,
  "dynamic_sl_protect_pct": 60,
  "dynamic_sl_update_int": 30,

  "use_ai_decisions": true,
  "ai_min_confidence": 0.65,
  "ai_tp_optimization": true,
  "ai_dynamic_sl": true,

  "use_multi_agent": true,
  "enable_sentiment_agent": true,
  "enable_risk_agent": true,
  "enable_tp_agent": true,

  "enable_adaptive_learning": true,
  "adaptive_window_trades": 20,
  "adaptive_min_trades": 10,
  "adaptive_max_reentry_adjust": 20,

  "max_cycles_per_position": 10,
  "max_daily_reentries": 50,
  "min_position_size_usd": 10,
  "stop_loss_percent": 2.0,

  "hedge_mode_enabled": false,
  "trigger_on_profit_tp": true,
  "trigger_on_loss_tp": true,
  "dca_on_loss": true,
  "max_position_multiple": 3.0,

  "combined_roi_exit_pct": 2.0,

  "wide_sl_atr_multiplier": 2.5,
  "disable_ai_sl": true,

  "rally_exit_enabled": true,
  "rally_adx_threshold": 25.0,
  "rally_sustained_move_pct": 3.0,

  "neg_tp1_percent": 0.4,
  "neg_tp1_add_percent": 30,
  "neg_tp2_percent": 0.7,
  "neg_tp2_add_percent": 50,
  "neg_tp3_percent": 1.0,
  "neg_tp3_add_percent": 80,

  "profit_protection_enabled": true,
  "profit_protection_percent": 50,
  "max_loss_of_earned_profit": 50,

  "allow_hedge_chains": false,
  "max_hedge_chain_depth": 2
}
```

---

## Acceptance Criteria

### AC5.2.1: Database Loading with Fallback
- [ ] `SettingsManager.GetScalpReentryConfig(userID)` loads from database
- [ ] If no database config exists, returns `DefaultScalpReentryConfig()`
- [ ] Database query uses `repository.GetUserModeConfig(ctx, userID, "scalp_reentry")`
- [ ] JSON unmarshal handles all 36+ scalp reentry fields correctly
- [ ] Error handling logs warning and falls back to defaults on unmarshal failure

### AC5.2.2: GET Scalp Reentry Config API
- [ ] `GET /api/futures/ginie/scalp-reentry-config` endpoint exists
- [ ] Requires authentication (JWT token)
- [ ] Returns current user's scalp reentry config (from DB or defaults)
- [ ] Response includes all 36+ settings in JSON format
- [ ] Returns 200 with config, even if no DB config exists (returns defaults)

### AC5.2.3: PUT Scalp Reentry Config API
- [ ] `PUT /api/futures/ginie/scalp-reentry-config` endpoint exists
- [ ] Requires authentication
- [ ] Accepts partial or full config updates
- [ ] Validates all numeric fields (percentages, counts, multipliers)
- [ ] Saves to database using `repository.SaveUserModeConfig()`
- [ ] Returns 200 with updated config on success

### AC5.2.4: Per-User Isolation
- [ ] User A changes config → only User A sees changes
- [ ] User B config remains at defaults (or their custom values)
- [ ] Two users can have completely different TP percentages
- [ ] Two users can have different hedge mode settings
- [ ] Database enforces UNIQUE(user_id, mode_name) constraint

### AC5.2.5: Persistence Across Sessions
- [ ] User changes config → logout → login → config persists
- [ ] Container restart → config persists (stored in PostgreSQL)
- [ ] Config changes reflected immediately in autopilot engine
- [ ] No stale config data served after updates

### AC5.2.6: Migration Support
- [ ] Existing `autopilot_settings.json` still works as system defaults
- [ ] `DefaultScalpReentryConfig()` matches production-ready values
- [ ] Admin can bulk-load defaults for all users (future story)
- [ ] No breaking changes to existing autopilot engine logic

---

## Technical Implementation

### Task 1: Update SettingsManager to Load from Database

```go
// internal/autopilot/settings.go

// Add database repository dependency to SettingsManager
type SettingsManager struct {
	mu       sync.RWMutex
	settings *AutopilotSettings

	// Database repository for per-user configs
	repo     *database.Repository

	// Cache of user-specific configs
	userConfigCache map[string]*UserConfigCache
	cacheMu         sync.RWMutex
}

// UserConfigCache caches user-specific settings to avoid DB queries every tick
type UserConfigCache struct {
	ScalpReentryConfig *ScalpReentryConfig
	LoadedAt           time.Time
	TTL                time.Duration // e.g., 5 minutes
}

// NewSettingsManager creates a new settings manager with database support
func NewSettingsManager(repo *database.Repository) *SettingsManager {
	return &SettingsManager{
		settings:        nil, // Loaded from file as system defaults
		repo:            repo,
		userConfigCache: make(map[string]*UserConfigCache),
	}
}

// GetScalpReentryConfig loads scalp reentry config for a specific user
// Priority: User DB config > System defaults
func (m *SettingsManager) GetScalpReentryConfig(ctx context.Context, userID string) (*ScalpReentryConfig, error) {
	// Check cache first
	if cached := m.getCachedConfig(userID); cached != nil {
		return cached.ScalpReentryConfig, nil
	}

	// Load from database
	configJSON, err := m.repo.GetUserModeConfig(ctx, userID, "scalp_reentry")
	if err != nil {
		log.Printf("[SCALP-REENTRY] Database error loading config for user %s: %v", userID, err)
		return m.getDefaultScalpReentryConfig(), nil
	}

	// No user config found - use defaults
	if configJSON == nil {
		log.Printf("[SCALP-REENTRY] No config found for user %s, using defaults", userID)
		defaultConfig := m.getDefaultScalpReentryConfig()
		m.cacheConfig(userID, defaultConfig)
		return defaultConfig, nil
	}

	// Unmarshal user config
	var config ScalpReentryConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		log.Printf("[SCALP-REENTRY] Failed to unmarshal config for user %s: %v, using defaults", userID, err)
		return m.getDefaultScalpReentryConfig(), nil
	}

	// Cache and return
	m.cacheConfig(userID, &config)
	log.Printf("[SCALP-REENTRY] Loaded custom config for user %s from database", userID)
	return &config, nil
}

// getDefaultScalpReentryConfig returns system default configuration
func (m *SettingsManager) getDefaultScalpReentryConfig() *ScalpReentryConfig {
	defaultConfig := DefaultScalpReentryConfig()
	return &defaultConfig
}

// getCachedConfig retrieves cached config if valid (within TTL)
func (m *SettingsManager) getCachedConfig(userID string) *UserConfigCache {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()

	cached, exists := m.userConfigCache[userID]
	if !exists {
		return nil
	}

	// Check TTL (5 minutes default)
	if time.Since(cached.LoadedAt) > cached.TTL {
		return nil
	}

	return cached
}

// cacheConfig stores user config in cache
func (m *SettingsManager) cacheConfig(userID string, config *ScalpReentryConfig) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	m.userConfigCache[userID] = &UserConfigCache{
		ScalpReentryConfig: config,
		LoadedAt:           time.Now(),
		TTL:                5 * time.Minute,
	}
}

// InvalidateUserCache clears cache for a user (call after updates)
func (m *SettingsManager) InvalidateUserCache(userID string) {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	delete(m.userConfigCache, userID)
	log.Printf("[SCALP-REENTRY] Invalidated cache for user %s", userID)
}
```

### Task 2: Add GET Scalp Reentry Config API Handler

```go
// internal/api/handlers_ginie.go

// handleGetScalpReentryConfig returns the user's scalp reentry configuration
// GET /api/futures/ginie/scalp-reentry-config
func (s *Server) handleGetScalpReentryConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	ctx := context.Background()

	// Load config from SettingsManager (DB → defaults fallback)
	config, err := s.settingsManager.GetScalpReentryConfig(ctx, userID)
	if err != nil {
		log.Printf("[API] Failed to get scalp reentry config for user %s: %v", userID, err)
		c.JSON(500, gin.H{"error": "Failed to load configuration"})
		return
	}

	// Return full config
	c.JSON(200, gin.H{
		"success": true,
		"config":  config,
		"source":  s.getConfigSource(ctx, userID), // "database" or "defaults"
	})
}

// getConfigSource determines if config is from database or defaults
func (s *Server) getConfigSource(ctx context.Context, userID string) string {
	configJSON, err := s.repo.GetUserModeConfig(ctx, userID, "scalp_reentry")
	if err != nil || configJSON == nil {
		return "defaults"
	}
	return "database"
}
```

### Task 3: Add PUT Scalp Reentry Config API Handler

```go
// internal/api/handlers_ginie.go

// ScalpReentryConfigUpdateRequest handles partial updates
type ScalpReentryConfigUpdateRequest struct {
	// All fields optional - only update what's provided
	Enabled *bool `json:"enabled,omitempty"`

	// TP Levels
	TP1Percent     *float64 `json:"tp1_percent,omitempty"`
	TP1SellPercent *float64 `json:"tp1_sell_percent,omitempty"`
	TP2Percent     *float64 `json:"tp2_percent,omitempty"`
	TP2SellPercent *float64 `json:"tp2_sell_percent,omitempty"`
	TP3Percent     *float64 `json:"tp3_percent,omitempty"`
	TP3SellPercent *float64 `json:"tp3_sell_percent,omitempty"`

	// Reentry
	ReentryPercent     *float64 `json:"reentry_percent,omitempty"`
	ReentryPriceBuffer *float64 `json:"reentry_price_buffer,omitempty"`
	MaxReentryAttempts *int     `json:"max_reentry_attempts,omitempty"`
	ReentryTimeoutSec  *int     `json:"reentry_timeout_sec,omitempty"`

	// Final portion
	FinalTrailingPercent *float64 `json:"final_trailing_percent,omitempty"`
	FinalHoldMinPercent  *float64 `json:"final_hold_min_percent,omitempty"`

	// Dynamic SL
	DynamicSLMaxLossPct   *float64 `json:"dynamic_sl_max_loss_pct,omitempty"`
	DynamicSLProtectPct   *float64 `json:"dynamic_sl_protect_pct,omitempty"`
	DynamicSLUpdateIntSec *int     `json:"dynamic_sl_update_int,omitempty"`

	// AI
	UseAIDecisions   *bool    `json:"use_ai_decisions,omitempty"`
	AIMinConfidence  *float64 `json:"ai_min_confidence,omitempty"`
	AITPOptimization *bool    `json:"ai_tp_optimization,omitempty"`
	AIDynamicSL      *bool    `json:"ai_dynamic_sl,omitempty"`

	// Multi-agent
	UseMultiAgent        *bool `json:"use_multi_agent,omitempty"`
	EnableSentimentAgent *bool `json:"enable_sentiment_agent,omitempty"`
	EnableRiskAgent      *bool `json:"enable_risk_agent,omitempty"`
	EnableTPAgent        *bool `json:"enable_tp_agent,omitempty"`

	// Adaptive learning
	EnableAdaptiveLearning   *bool    `json:"enable_adaptive_learning,omitempty"`
	AdaptiveWindowTrades     *int     `json:"adaptive_window_trades,omitempty"`
	AdaptiveMinTrades        *int     `json:"adaptive_min_trades,omitempty"`
	AdaptiveMaxReentryPctAdj *float64 `json:"adaptive_max_reentry_adjust,omitempty"`

	// Risk limits
	MaxCyclesPerPosition *int     `json:"max_cycles_per_position,omitempty"`
	MaxDailyReentries    *int     `json:"max_daily_reentries,omitempty"`
	MinPositionSizeUSD   *float64 `json:"min_position_size_usd,omitempty"`
	StopLossPercent      *float64 `json:"stop_loss_percent,omitempty"`

	// Hedge mode
	HedgeModeEnabled    *bool    `json:"hedge_mode_enabled,omitempty"`
	TriggerOnProfitTP   *bool    `json:"trigger_on_profit_tp,omitempty"`
	TriggerOnLossTP     *bool    `json:"trigger_on_loss_tp,omitempty"`
	DCAOnLoss           *bool    `json:"dca_on_loss,omitempty"`
	MaxPositionMultiple *float64 `json:"max_position_multiple,omitempty"`

	CombinedROIExitPct *float64 `json:"combined_roi_exit_pct,omitempty"`

	WideSLATRMultiplier *float64 `json:"wide_sl_atr_multiplier,omitempty"`
	DisableAISL         *bool    `json:"disable_ai_sl,omitempty"`

	RallyExitEnabled      *bool    `json:"rally_exit_enabled,omitempty"`
	RallyADXThreshold     *float64 `json:"rally_adx_threshold,omitempty"`
	RallySustainedMovePct *float64 `json:"rally_sustained_move_pct,omitempty"`

	// Negative TPs
	NegTP1Percent    *float64 `json:"neg_tp1_percent,omitempty"`
	NegTP1AddPercent *float64 `json:"neg_tp1_add_percent,omitempty"`
	NegTP2Percent    *float64 `json:"neg_tp2_percent,omitempty"`
	NegTP2AddPercent *float64 `json:"neg_tp2_add_percent,omitempty"`
	NegTP3Percent    *float64 `json:"neg_tp3_percent,omitempty"`
	NegTP3AddPercent *float64 `json:"neg_tp3_add_percent,omitempty"`

	// Profit protection
	ProfitProtectionEnabled *bool    `json:"profit_protection_enabled,omitempty"`
	ProfitProtectionPercent *float64 `json:"profit_protection_percent,omitempty"`
	MaxLossOfEarnedProfit   *float64 `json:"max_loss_of_earned_profit,omitempty"`

	// Chain control
	AllowHedgeChains   *bool `json:"allow_hedge_chains,omitempty"`
	MaxHedgeChainDepth *int  `json:"max_hedge_chain_depth,omitempty"`
}

// handleUpdateScalpReentryConfig updates user's scalp reentry configuration
// PUT /api/futures/ginie/scalp-reentry-config
func (s *Server) handleUpdateScalpReentryConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var req ScalpReentryConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	ctx := context.Background()

	// Load current config (DB or defaults)
	currentConfig, err := s.settingsManager.GetScalpReentryConfig(ctx, userID)
	if err != nil {
		log.Printf("[API] Failed to get current config for user %s: %v", userID, err)
		c.JSON(500, gin.H{"error": "Failed to load current configuration"})
		return
	}

	// Apply partial updates
	updatedConfig := applyScalpReentryConfigUpdates(currentConfig, &req)

	// Validate updated config
	if err := validateScalpReentryConfig(updatedConfig); err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Invalid configuration: %v", err)})
		return
	}

	// Serialize to JSON
	configJSON, err := json.Marshal(updatedConfig)
	if err != nil {
		log.Printf("[API] Failed to marshal config for user %s: %v", userID, err)
		c.JSON(500, gin.H{"error": "Failed to save configuration"})
		return
	}

	// Save to database
	err = s.repo.SaveUserModeConfig(ctx, userID, "scalp_reentry", updatedConfig.Enabled, configJSON)
	if err != nil {
		log.Printf("[API] Failed to save config for user %s: %v", userID, err)
		c.JSON(500, gin.H{"error": "Failed to save configuration"})
		return
	}

	// Invalidate cache
	s.settingsManager.InvalidateUserCache(userID)

	log.Printf("[API] Updated scalp reentry config for user %s", userID)

	c.JSON(200, gin.H{
		"success": true,
		"message": "Scalp reentry configuration updated",
		"config":  updatedConfig,
	})
}

// applyScalpReentryConfigUpdates applies partial updates to config
func applyScalpReentryConfigUpdates(current *ScalpReentryConfig, req *ScalpReentryConfigUpdateRequest) *ScalpReentryConfig {
	updated := *current // Copy

	// Apply all non-nil updates
	if req.Enabled != nil {
		updated.Enabled = *req.Enabled
	}
	if req.TP1Percent != nil {
		updated.TP1Percent = *req.TP1Percent
	}
	if req.TP1SellPercent != nil {
		updated.TP1SellPercent = *req.TP1SellPercent
	}
	if req.TP2Percent != nil {
		updated.TP2Percent = *req.TP2Percent
	}
	if req.TP2SellPercent != nil {
		updated.TP2SellPercent = *req.TP2SellPercent
	}
	if req.TP3Percent != nil {
		updated.TP3Percent = *req.TP3Percent
	}
	if req.TP3SellPercent != nil {
		updated.TP3SellPercent = *req.TP3SellPercent
	}
	if req.ReentryPercent != nil {
		updated.ReentryPercent = *req.ReentryPercent
	}
	if req.ReentryPriceBuffer != nil {
		updated.ReentryPriceBuffer = *req.ReentryPriceBuffer
	}
	if req.MaxReentryAttempts != nil {
		updated.MaxReentryAttempts = *req.MaxReentryAttempts
	}
	if req.ReentryTimeoutSec != nil {
		updated.ReentryTimeoutSec = *req.ReentryTimeoutSec
	}
	if req.FinalTrailingPercent != nil {
		updated.FinalTrailingPercent = *req.FinalTrailingPercent
	}
	if req.FinalHoldMinPercent != nil {
		updated.FinalHoldMinPercent = *req.FinalHoldMinPercent
	}
	if req.DynamicSLMaxLossPct != nil {
		updated.DynamicSLMaxLossPct = *req.DynamicSLMaxLossPct
	}
	if req.DynamicSLProtectPct != nil {
		updated.DynamicSLProtectPct = *req.DynamicSLProtectPct
	}
	if req.DynamicSLUpdateIntSec != nil {
		updated.DynamicSLUpdateIntSec = *req.DynamicSLUpdateIntSec
	}
	if req.UseAIDecisions != nil {
		updated.UseAIDecisions = *req.UseAIDecisions
	}
	if req.AIMinConfidence != nil {
		updated.AIMinConfidence = *req.AIMinConfidence
	}
	if req.AITPOptimization != nil {
		updated.AITPOptimization = *req.AITPOptimization
	}
	if req.AIDynamicSL != nil {
		updated.AIDynamicSL = *req.AIDynamicSL
	}
	if req.UseMultiAgent != nil {
		updated.UseMultiAgent = *req.UseMultiAgent
	}
	if req.EnableSentimentAgent != nil {
		updated.EnableSentimentAgent = *req.EnableSentimentAgent
	}
	if req.EnableRiskAgent != nil {
		updated.EnableRiskAgent = *req.EnableRiskAgent
	}
	if req.EnableTPAgent != nil {
		updated.EnableTPAgent = *req.EnableTPAgent
	}
	if req.EnableAdaptiveLearning != nil {
		updated.EnableAdaptiveLearning = *req.EnableAdaptiveLearning
	}
	if req.AdaptiveWindowTrades != nil {
		updated.AdaptiveWindowTrades = *req.AdaptiveWindowTrades
	}
	if req.AdaptiveMinTrades != nil {
		updated.AdaptiveMinTrades = *req.AdaptiveMinTrades
	}
	if req.AdaptiveMaxReentryPctAdj != nil {
		updated.AdaptiveMaxReentryPctAdj = *req.AdaptiveMaxReentryPctAdj
	}
	if req.MaxCyclesPerPosition != nil {
		updated.MaxCyclesPerPosition = *req.MaxCyclesPerPosition
	}
	if req.MaxDailyReentries != nil {
		updated.MaxDailyReentries = *req.MaxDailyReentries
	}
	if req.MinPositionSizeUSD != nil {
		updated.MinPositionSizeUSD = *req.MinPositionSizeUSD
	}
	if req.StopLossPercent != nil {
		updated.StopLossPercent = *req.StopLossPercent
	}
	if req.HedgeModeEnabled != nil {
		updated.HedgeModeEnabled = *req.HedgeModeEnabled
	}
	if req.TriggerOnProfitTP != nil {
		updated.TriggerOnProfitTP = *req.TriggerOnProfitTP
	}
	if req.TriggerOnLossTP != nil {
		updated.TriggerOnLossTP = *req.TriggerOnLossTP
	}
	if req.DCAOnLoss != nil {
		updated.DCAOnLoss = *req.DCAOnLoss
	}
	if req.MaxPositionMultiple != nil {
		updated.MaxPositionMultiple = *req.MaxPositionMultiple
	}
	if req.CombinedROIExitPct != nil {
		updated.CombinedROIExitPct = *req.CombinedROIExitPct
	}
	if req.WideSLATRMultiplier != nil {
		updated.WideSLATRMultiplier = *req.WideSLATRMultiplier
	}
	if req.DisableAISL != nil {
		updated.DisableAISL = *req.DisableAISL
	}
	if req.RallyExitEnabled != nil {
		updated.RallyExitEnabled = *req.RallyExitEnabled
	}
	if req.RallyADXThreshold != nil {
		updated.RallyADXThreshold = *req.RallyADXThreshold
	}
	if req.RallySustainedMovePct != nil {
		updated.RallySustainedMovePct = *req.RallySustainedMovePct
	}
	if req.NegTP1Percent != nil {
		updated.NegTP1Percent = *req.NegTP1Percent
	}
	if req.NegTP1AddPercent != nil {
		updated.NegTP1AddPercent = *req.NegTP1AddPercent
	}
	if req.NegTP2Percent != nil {
		updated.NegTP2Percent = *req.NegTP2Percent
	}
	if req.NegTP2AddPercent != nil {
		updated.NegTP2AddPercent = *req.NegTP2AddPercent
	}
	if req.NegTP3Percent != nil {
		updated.NegTP3Percent = *req.NegTP3Percent
	}
	if req.NegTP3AddPercent != nil {
		updated.NegTP3AddPercent = *req.NegTP3AddPercent
	}
	if req.ProfitProtectionEnabled != nil {
		updated.ProfitProtectionEnabled = *req.ProfitProtectionEnabled
	}
	if req.ProfitProtectionPercent != nil {
		updated.ProfitProtectionPercent = *req.ProfitProtectionPercent
	}
	if req.MaxLossOfEarnedProfit != nil {
		updated.MaxLossOfEarnedProfit = *req.MaxLossOfEarnedProfit
	}
	if req.AllowHedgeChains != nil {
		updated.AllowHedgeChains = *req.AllowHedgeChains
	}
	if req.MaxHedgeChainDepth != nil {
		updated.MaxHedgeChainDepth = *req.MaxHedgeChainDepth
	}

	return &updated
}

// validateScalpReentryConfig validates configuration values
func validateScalpReentryConfig(config *ScalpReentryConfig) error {
	// TP percentages must be positive and ascending
	if config.TP1Percent <= 0 || config.TP2Percent <= 0 || config.TP3Percent <= 0 {
		return fmt.Errorf("TP percentages must be positive")
	}
	if config.TP1Percent >= config.TP2Percent || config.TP2Percent >= config.TP3Percent {
		return fmt.Errorf("TP percentages must be ascending (TP1 < TP2 < TP3)")
	}

	// TP sell percentages must be 0-100
	if config.TP1SellPercent < 0 || config.TP1SellPercent > 100 {
		return fmt.Errorf("TP1 sell percent must be 0-100")
	}
	if config.TP2SellPercent < 0 || config.TP2SellPercent > 100 {
		return fmt.Errorf("TP2 sell percent must be 0-100")
	}
	if config.TP3SellPercent < 0 || config.TP3SellPercent > 100 {
		return fmt.Errorf("TP3 sell percent must be 0-100")
	}

	// Reentry percent must be 0-100
	if config.ReentryPercent < 0 || config.ReentryPercent > 100 {
		return fmt.Errorf("reentry percent must be 0-100")
	}

	// AI confidence must be 0-1
	if config.AIMinConfidence < 0 || config.AIMinConfidence > 1 {
		return fmt.Errorf("AI min confidence must be 0.0-1.0")
	}

	// Max position multiple must be >= 1
	if config.MaxPositionMultiple < 1.0 {
		return fmt.Errorf("max position multiple must be >= 1.0")
	}

	// Negative TP percentages must be positive and ascending
	if config.NegTP1Percent <= 0 || config.NegTP2Percent <= 0 || config.NegTP3Percent <= 0 {
		return fmt.Errorf("negative TP percentages must be positive")
	}
	if config.NegTP1Percent >= config.NegTP2Percent || config.NegTP2Percent >= config.NegTP3Percent {
		return fmt.Errorf("negative TP percentages must be ascending")
	}

	// Profit protection percent must be 0-100
	if config.ProfitProtectionPercent < 0 || config.ProfitProtectionPercent > 100 {
		return fmt.Errorf("profit protection percent must be 0-100")
	}

	return nil
}
```

### Task 4: Update Frontend API Service

```typescript
// web/src/services/futuresApi.ts

export interface ScalpReentryConfig {
  enabled: boolean;

  // TP Levels
  tp1_percent: number;
  tp1_sell_percent: number;
  tp2_percent: number;
  tp2_sell_percent: number;
  tp3_percent: number;
  tp3_sell_percent: number;

  // Reentry
  reentry_percent: number;
  reentry_price_buffer: number;
  max_reentry_attempts: number;
  reentry_timeout_sec: number;

  // Final portion
  final_trailing_percent: number;
  final_hold_min_percent: number;

  // Dynamic SL
  dynamic_sl_max_loss_pct: number;
  dynamic_sl_protect_pct: number;
  dynamic_sl_update_int: number;

  // AI
  use_ai_decisions: boolean;
  ai_min_confidence: number;
  ai_tp_optimization: boolean;
  ai_dynamic_sl: boolean;

  // Multi-agent
  use_multi_agent: boolean;
  enable_sentiment_agent: boolean;
  enable_risk_agent: boolean;
  enable_tp_agent: boolean;

  // Adaptive learning
  enable_adaptive_learning: boolean;
  adaptive_window_trades: number;
  adaptive_min_trades: number;
  adaptive_max_reentry_adjust: number;

  // Risk limits
  max_cycles_per_position: number;
  max_daily_reentries: number;
  min_position_size_usd: number;
  stop_loss_percent: number;

  // Hedge mode
  hedge_mode_enabled: boolean;
  trigger_on_profit_tp: boolean;
  trigger_on_loss_tp: boolean;
  dca_on_loss: boolean;
  max_position_multiple: number;

  combined_roi_exit_pct: number;

  wide_sl_atr_multiplier: number;
  disable_ai_sl: boolean;

  rally_exit_enabled: boolean;
  rally_adx_threshold: number;
  rally_sustained_move_pct: number;

  // Negative TPs
  neg_tp1_percent: number;
  neg_tp1_add_percent: number;
  neg_tp2_percent: number;
  neg_tp2_add_percent: number;
  neg_tp3_percent: number;
  neg_tp3_add_percent: number;

  // Profit protection
  profit_protection_enabled: boolean;
  profit_protection_percent: number;
  max_loss_of_earned_profit: number;

  // Chain control
  allow_hedge_chains: boolean;
  max_hedge_chain_depth: number;
}

export interface ScalpReentryConfigResponse {
  success: boolean;
  config: ScalpReentryConfig;
  source: 'database' | 'defaults';
}

// GET scalp reentry config
export const getScalpReentryConfig = async (): Promise<ScalpReentryConfigResponse> => {
  const response = await fetch('/api/futures/ginie/scalp-reentry-config', {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${localStorage.getItem('token')}`,
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error('Failed to fetch scalp reentry config');
  }

  return response.json();
};

// PUT scalp reentry config (partial or full update)
export const updateScalpReentryConfig = async (
  config: Partial<ScalpReentryConfig>
): Promise<ScalpReentryConfigResponse> => {
  const response = await fetch('/api/futures/ginie/scalp-reentry-config', {
    method: 'PUT',
    headers: {
      'Authorization': `Bearer ${localStorage.getItem('token')}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(config),
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error || 'Failed to update scalp reentry config');
  }

  return response.json();
};
```

### Task 5: Register API Routes

```go
// internal/api/server.go

func (s *Server) setupRoutes() {
	// ... existing routes ...

	// Scalp Reentry Configuration
	futuresGinie := s.router.Group("/api/futures/ginie")
	futuresGinie.Use(s.authMiddleware())
	{
		// ... existing routes ...

		futuresGinie.GET("/scalp-reentry-config", s.handleGetScalpReentryConfig)
		futuresGinie.PUT("/scalp-reentry-config", s.handleUpdateScalpReentryConfig)
	}
}
```

---

## API Reference

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/futures/ginie/scalp-reentry-config` | Get user's scalp reentry config |
| PUT | `/api/futures/ginie/scalp-reentry-config` | Update user's scalp reentry config |

### Response: GET Scalp Reentry Config

```json
{
  "success": true,
  "source": "database",
  "config": {
    "enabled": true,
    "tp1_percent": 0.4,
    "tp1_sell_percent": 30,
    "tp2_percent": 0.7,
    "tp2_sell_percent": 50,
    "tp3_percent": 1.0,
    "tp3_sell_percent": 80,
    "reentry_percent": 80,
    "reentry_price_buffer": 0.05,
    "max_reentry_attempts": 3,
    "reentry_timeout_sec": 300,
    "final_trailing_percent": 5.0,
    "final_hold_min_percent": 20,
    "dynamic_sl_max_loss_pct": 40,
    "dynamic_sl_protect_pct": 60,
    "dynamic_sl_update_int": 30,
    "use_ai_decisions": true,
    "ai_min_confidence": 0.65,
    "ai_tp_optimization": true,
    "ai_dynamic_sl": true,
    "use_multi_agent": true,
    "enable_sentiment_agent": true,
    "enable_risk_agent": true,
    "enable_tp_agent": true,
    "enable_adaptive_learning": true,
    "adaptive_window_trades": 20,
    "adaptive_min_trades": 10,
    "adaptive_max_reentry_adjust": 20,
    "max_cycles_per_position": 10,
    "max_daily_reentries": 50,
    "min_position_size_usd": 10,
    "stop_loss_percent": 2.0,
    "hedge_mode_enabled": false,
    "trigger_on_profit_tp": true,
    "trigger_on_loss_tp": true,
    "dca_on_loss": true,
    "max_position_multiple": 3.0,
    "combined_roi_exit_pct": 2.0,
    "wide_sl_atr_multiplier": 2.5,
    "disable_ai_sl": true,
    "rally_exit_enabled": true,
    "rally_adx_threshold": 25.0,
    "rally_sustained_move_pct": 3.0,
    "neg_tp1_percent": 0.4,
    "neg_tp1_add_percent": 30,
    "neg_tp2_percent": 0.7,
    "neg_tp2_add_percent": 50,
    "neg_tp3_percent": 1.0,
    "neg_tp3_add_percent": 80,
    "profit_protection_enabled": true,
    "profit_protection_percent": 50,
    "max_loss_of_earned_profit": 50,
    "allow_hedge_chains": false,
    "max_hedge_chain_depth": 2
  }
}
```

### Request: PUT Scalp Reentry Config (Partial Update)

```json
{
  "tp1_percent": 0.5,
  "hedge_mode_enabled": true,
  "dca_on_loss": false
}
```

### Response: PUT Scalp Reentry Config

```json
{
  "success": true,
  "message": "Scalp reentry configuration updated",
  "config": {
    "enabled": true,
    "tp1_percent": 0.5,
    "hedge_mode_enabled": true,
    "dca_on_loss": false,
    "... all other fields ...": "..."
  }
}
```

---

## Testing Requirements

### Test 1: Database Load with Fallback
```bash
# User with no config → should get defaults
curl http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $TOKEN" | jq '.source'
# Expected: "defaults"

curl http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $TOKEN" | jq '.config.tp1_percent'
# Expected: 0.4 (default value)
```

### Test 2: Save Custom Config
```bash
# Update TP1 to 0.5%
curl -X PUT http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tp1_percent": 0.5, "hedge_mode_enabled": true}'

# Verify saved
curl http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $TOKEN" | jq '.source'
# Expected: "database"

curl http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $TOKEN" | jq '.config.tp1_percent'
# Expected: 0.5
```

### Test 3: Per-User Isolation
```bash
# User 1 changes config
curl -X PUT http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $USER1_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tp1_percent": 0.6}'

# User 2 still has defaults (or their own config)
curl http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $USER2_TOKEN" | jq '.config.tp1_percent'
# Expected: 0.4 (default) or User 2's custom value (not 0.6)
```

### Test 4: Persistence Across Sessions
```bash
# User changes config
curl -X PUT http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"hedge_mode_enabled": true}'

# Restart container
./scripts/docker-dev.sh

# Verify config persists
curl http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $TOKEN" | jq '.config.hedge_mode_enabled'
# Expected: true
```

### Test 5: Validation
```bash
# Try to set TP1 > TP2 (invalid)
curl -X PUT http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tp1_percent": 1.5, "tp2_percent": 0.7}'
# Expected: 400 Bad Request
# Error: "TP percentages must be ascending (TP1 < TP2 < TP3)"

# Try to set invalid AI confidence
curl -X PUT http://localhost:8094/api/futures/ginie/scalp-reentry-config \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"ai_min_confidence": 1.5}'
# Expected: 400 Bad Request
# Error: "AI min confidence must be 0.0-1.0"
```

---

## Definition of Done

- [ ] SettingsManager loads scalp_reentry config from database per-user
- [ ] Fallback to DefaultScalpReentryConfig() if no DB config exists
- [ ] GET /api/futures/ginie/scalp-reentry-config endpoint implemented
- [ ] PUT /api/futures/ginie/scalp-reentry-config endpoint implemented
- [ ] Partial updates supported (only update provided fields)
- [ ] Config validation implemented (ascending TPs, valid ranges, etc.)
- [ ] Per-user config isolation verified (2 users, different configs)
- [ ] Config persists across container restarts
- [ ] Cache invalidation on updates works correctly
- [ ] TypeScript types added to futuresApi.ts
- [ ] All 5 tests pass
- [ ] No breaking changes to existing autopilot engine
- [ ] Code review approved

---

## Approval Sign-Off

- **Scrum Master (Bob)**: Pending
- **Developer (Amelia)**: Pending
- **Test Architect (Murat)**: Pending
- **Architect (Winston)**: Pending
- **Product Manager (John)**: Pending

---

## Related Stories

- **Story 4.1:** Database-First Mode Configuration Foundation (prerequisite)
- **Story 4.2:** User Mode Config Repository (prerequisite)
- **Story 5.3:** Scalp Reentry Frontend UI (next - depends on this)
- **Story 5.4:** Scalp Reentry Mode Toggle & Status (next - depends on this)
