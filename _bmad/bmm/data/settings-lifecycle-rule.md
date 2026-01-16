# Settings Lifecycle Rule (MANDATORY)

**Project:** Binance Trading Bot
**Rule ID:** SETTINGS-LIFECYCLE-001
**Status:** MANDATORY for all user-configurable settings

---

## Rule Summary

**Any new user-configurable field MUST follow the complete settings lifecycle:**

```
default-settings.json → Database → Redis Cache → API → Frontend
```

**Failure to follow this lifecycle will result in:**
- Missing defaults for new users
- Cache inconsistencies
- Settings comparison UI failures
- Reset-to-defaults not working

---

## Complete Settings Lifecycle

### Step 1: Add to `default-settings.json`

**File:** `/home/administrator/KOSH/binance-trading-app/default-settings.json`

**Structure:**
```json
{
  "mode_configs": {
    "ultra_fast": { /* 20 groups */ },
    "scalp": { /* 20 groups */ },
    "swing": { /* 20 groups */ },
    "position": { /* 20 groups */ }
  },
  "circuit_breaker": { "global": { /* fields */ } },
  "llm_config": { /* fields */ },
  "capital_allocation": { /* fields */ },
  "global_trading": { /* fields */ },
  "safety_settings": {
    "ultra_fast": { /* fields */ },
    "scalp": { /* fields */ },
    "swing": { /* fields */ },
    "position": { /* fields */ }
  }
}
```

**20 Groups per Mode:**
| Group | Key | Description |
|-------|-----|-------------|
| 1 | `enabled` | Mode on/off |
| 2 | `timeframe` | Trend/entry/analysis timeframes |
| 3 | `confidence` | Min/high/ultra confidence thresholds |
| 4 | `size` | Position sizing, leverage |
| 5 | `sltp` | Stop loss, take profit, trailing |
| 6 | `risk` | Risk level, max drawdown |
| 7 | `circuit_breaker` | Mode-specific circuit breaker |
| 8 | `hedge` | Hedge entry settings |
| 9 | `averaging` | Position averaging/DCA |
| 10 | `stale_release` | Stale position management |
| 11 | `assignment` | Mode assignment criteria |
| 12 | `mtf` | Multi-timeframe analysis |
| 13 | `dynamic_ai_exit` | AI-powered exit decisions |
| 14 | `reversal` | Reversal entry criteria |
| 15 | `funding_rate` | Funding rate exit logic |
| 16 | `trend_divergence` | Trend divergence blocking |
| 17 | `position_optimization` | TP levels, reentry, DCA |
| 18 | `trend_filters` | BTC trend, EMA, VWAP |
| 19 | `early_warning` | Position early warning |
| 20 | `entry` | Entry method settings |

---

### Step 2: Add to Go Data Models

**Mode Fields:** `internal/autopilot/settings.go`
- Add field to appropriate struct (`ModeFullConfig`, `ModeConfidenceConfig`, etc.)
- Ensure JSON tags match `default-settings.json`

**Global Fields:** `internal/database/models_user.go`
- Add to appropriate model (`UserGlobalCircuitBreaker`, `UserLLMConfig`, etc.)

**Example:**
```go
type ModeConfidenceConfig struct {
    MinConfidence   float64 `json:"min_confidence"`
    HighConfidence  float64 `json:"high_confidence"`
    UltraConfidence float64 `json:"ultra_confidence"`
    NewField        float64 `json:"new_field"`  // ADD HERE
}
```

---

### Step 3: Database Persistence

**Repository Files:**
- `internal/database/repository_user_mode_config.go` - Mode settings
- `internal/database/repository_user.go` - Global settings

**Migration (if new column):**
Create migration file: `migrations/XXX_add_new_field.sql`

**Key Functions:**
- `SaveUserModeConfig()` - Persists full mode config (JSONB)
- `GetUserModeConfig()` - Retrieves mode config
- `UpdateUserModeConfigGroup()` - Updates single group

---

### Step 4: User Cache Layer

**File:** `internal/cache/settings_cache_service.go`

**Key Functions to Update:**
1. `extractGroupFromConfig()` - Extract new field from DB model
2. `mergeGroupIntoConfig()` - Merge new field back to DB model
3. Group struct in `settings_groups.go` if new group

**Redis Key Patterns:**
```
# Mode settings (per user)
user:{userID}:mode:{mode}:{group}

# Global settings (per user)
user:{userID}:circuit_breaker
user:{userID}:llm_config
user:{userID}:capital_allocation
user:{userID}:global_trading

# Safety settings (per user, per mode)
user:{userID}:safety:{mode}
```

---

### Step 5: Admin Defaults Cache

**File:** `internal/cache/admin_defaults_cache.go`

**Purpose:** Cache defaults from `default-settings.json` for:
- New user initialization
- Settings comparison UI
- Reset-to-defaults functionality

**Key Functions:**
- `LoadAdminDefaults()` - Load all 89 keys on startup
- `GetAdminDefaultGroup()` - Get single group default
- `CopyDefaultsToNewUser()` - Initialize new user cache

---

### Step 6: New User Initialization

**File:** `internal/database/user_initialization.go`

**Function:** `InitializeUserDefaultSettings(ctx, userID, defaults)`

**What it does:**
1. Creates 4 mode config records (ultra_fast, scalp, swing, position)
2. Creates global settings records
3. Creates safety settings records
4. All values from `default-settings.json`

**Also calls:** `AdminDefaultsCacheService.CopyDefaultsToNewUser()`

---

### Step 7: API Handlers

**Files:**
- `internal/api/handlers_settings.go` - General settings
- `internal/api/handlers_settings_defaults.go` - Reset defaults
- `internal/api/handlers_user_settings.go` - Settings comparison

**Pattern: Write-Through**
```go
// 1. Save to DB first (durability)
err := repo.UpdateUserModeConfigGroup(ctx, userID, mode, group, data)
if err != nil {
    return err
}

// 2. Update cache (best effort)
cache.UpdateModeGroup(ctx, userID, mode, group, data)
```

**Endpoints:**
```
GET  /api/futures/ginie/modes/{mode}/groups/{group}
PUT  /api/futures/ginie/modes/{mode}/groups/{group}
POST /api/futures/ginie/modes/{mode}/load-defaults
GET  /api/futures/user/settings/comparison
```

---

### Step 8: Frontend Components

**Settings Display:**
- `web/src/pages/Settings.tsx` - Main settings page
- `web/src/components/SettingsComparisonView.tsx` - Comparison UI

**Add to `SETTING_GROUPS` constant:**
```typescript
const SETTING_GROUPS = {
  confidence: {
    label: 'Confidence Settings',
    fields: ['min_confidence', 'high_confidence', 'ultra_confidence', 'new_field'],
    risk_levels: { new_field: 'medium' }  // For UI indicators
  }
}
```

---

## Cache Patterns (Reference)

### Cache-First Read
```go
// 1. Try cache first
cached, err := cache.GetModeGroup(ctx, userID, mode, group)
if err == nil {
    return cached
}

// 2. Cache miss - load from DB, populate cache
data, err := repo.GetUserModeConfig(ctx, userID, mode)
cache.SetModeGroup(ctx, userID, mode, group, data)
return data
```

### Write-Through Pattern
```go
// 1. DB first (truth)
err := repo.Save(ctx, userID, data)

// 2. Cache second (best effort)
cache.Set(ctx, key, data)
```

### Cache Invalidation
```go
// After settings update
cache.InvalidateModeGroup(ctx, userID, mode, group)

// After bulk changes
cache.InvalidateAllUserSettings(ctx, userID)
```

---

## Redis Key Count

**Per User: 88 keys**
- 80 mode keys (4 modes x 20 groups)
- 4 global keys
- 4 safety keys

**Admin Defaults: 89 keys**
- 80 mode defaults
- 4 global defaults
- 4 safety defaults
- 1 hash key (change detection)

---

## Checklist for New Settings Fields

When implementing a story that adds new user settings:

- [ ] 1. Added to `default-settings.json` in correct section
- [ ] 2. Added to Go struct with matching JSON tag
- [ ] 3. Database migration created (if new column)
- [ ] 4. Repository methods handle new field
- [ ] 5. Cache `extractGroupFromConfig()` extracts field
- [ ] 6. Cache `mergeGroupIntoConfig()` merges field
- [ ] 7. Admin defaults cache loads new field
- [ ] 8. New user initialization includes field
- [ ] 9. API handler reads/writes new field
- [ ] 10. Frontend component displays/edits field
- [ ] 11. Settings comparison shows field with risk level
- [ ] 12. Reset-to-defaults includes new field

---

## Files Summary

| Layer | File | Purpose |
|-------|------|---------|
| Source | `default-settings.json` | Single source of truth |
| Model | `internal/autopilot/settings.go` | Mode config structs |
| Model | `internal/database/models_user.go` | Global config models |
| DB | `internal/database/repository_user_mode_config.go` | Mode CRUD |
| DB | `internal/database/user_initialization.go` | New user setup |
| Cache | `internal/cache/settings_cache_service.go` | User settings cache |
| Cache | `internal/cache/admin_defaults_cache.go` | Defaults cache |
| Cache | `internal/cache/settings_groups.go` | Group definitions |
| API | `internal/api/handlers_settings.go` | Settings endpoints |
| API | `internal/api/handlers_settings_defaults.go` | Reset endpoints |
| Frontend | `web/src/pages/Settings.tsx` | Settings UI |
| Frontend | `web/src/components/SettingsComparisonView.tsx` | Comparison UI |

---

## Non-Compliance Examples

**Wrong:** Adding field only to API handler
- Result: New users get null values
- Result: Reset-to-defaults breaks

**Wrong:** Adding field only to database
- Result: Cache returns stale data
- Result: Settings comparison fails

**Wrong:** Missing from `default-settings.json`
- Result: New users have no value
- Result: Admin cannot set defaults

---

## Enforcement

This rule applies to ALL stories that:
1. Add new user-configurable settings
2. Add new mode-specific parameters
3. Add new global configuration options
4. Modify existing settings structure

**Code Review Checklist Item:**
> "Does this change follow the Settings Lifecycle Rule (SETTINGS-LIFECYCLE-001)?"
