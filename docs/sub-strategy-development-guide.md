# Sub-Strategy Development Guide

This document provides a step-by-step guideline for adding new sub-strategies to the Binance Trading Bot. Follow this process whenever implementing a new trading sub-strategy.

**One-Time Setup Principle:** Once the initial coding is complete for a sub-strategy, all settings automatically flow from `default-settings.json` through the entire system to the Futures page UI.

---

## Table of Contents

1. [Overview](#overview)
2. [Complete Data Flow](#complete-data-flow)
3. [Phase 1: Strategy Definition](#phase-1-strategy-definition)
4. [Phase 2: Configuration (default-settings.json)](#phase-2-configuration-default-settingsjson)
5. [Phase 3: Database & Cache Structure](#phase-3-database--cache-structure)
6. [Phase 4: Backend API Implementation](#phase-4-backend-api-implementation)
7. [Phase 5: Frontend Implementation](#phase-5-frontend-implementation)
8. [Phase 6: Build & Verification](#phase-6-build--verification)
9. [Checklist](#checklist)
10. [File Reference](#file-reference)
11. [Troubleshooting](#troubleshooting)

---

## Overview

### Strategy Hierarchy

```
Trading Mode (ultra_fast, scalp, swing, position)
    └── Strategy Group (breakout, momentum, reversal, range)
            └── Sub-Strategy (e.g., ravindra_volume_imbalance)
                    └── Strategy Settings (risk_reward, trailing_stop, pattern_detection, etc.)
```

### One-Time Coding Benefits

Once you complete the implementation for a new sub-strategy:
- Settings automatically load from `default-settings.json`
- User modifications are stored in database and cached in Redis
- Reset Settings page compares user values with defaults
- Futures page displays editable UI for all fields
- All changes persist through the complete pipeline

---

## Complete Data Flow

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                            SUB-STRATEGY DATA FLOW                               │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│  ┌──────────────────────┐                                                       │
│  │  default-settings.json │  ← SOURCE OF TRUTH (Default Values)                 │
│  └──────────┬───────────┘                                                       │
│             │                                                                   │
│             ▼                                                                   │
│  ┌──────────────────────┐     ┌──────────────────────┐                         │
│  │  Database Migration   │ ──► │  User Settings Table  │  ← User Modifications  │
│  │  (sub_strategy_settings)│    │  (PostgreSQL)         │                        │
│  └──────────────────────┘     └──────────┬───────────┘                         │
│                                          │                                      │
│                                          ▼                                      │
│                               ┌──────────────────────┐                         │
│                               │     Redis Cache       │  ← Fast Access          │
│                               │  (Same Hierarchy)     │                         │
│                               └──────────┬───────────┘                         │
│                                          │                                      │
│                                          ▼                                      │
│                               ┌──────────────────────┐                         │
│                               │    REST API Layer     │                         │
│                               │  /api/futures/...     │                         │
│                               └──────────┬───────────┘                         │
│                                          │                                      │
│                    ┌─────────────────────┼─────────────────────┐               │
│                    ▼                     ▼                     ▼               │
│         ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐       │
│         │  Reset Settings   │  │   Futures Page    │  │  Trading Engine   │      │
│         │  (Comparison UI)  │  │  (Edit Settings)  │  │  (Execute Trades) │      │
│         └──────────────────┘  └──────────────────┘  └──────────────────┘       │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Flow Description

| Step | Component | Action |
|------|-----------|--------|
| 1 | `default-settings.json` | Define all default values for sub-strategy |
| 2 | Database Migration | Create/update table structure for storing user settings |
| 3 | User Settings Table | Store user-modified values (PostgreSQL) |
| 4 | Redis Cache | Cache settings following same hierarchy for fast access |
| 5 | REST API | Expose endpoints for reading/writing settings |
| 6 | Reset Settings Page | Compare user DB values with defaults, allow reset |
| 7 | Futures Page | Display editable UI, save changes |
| 8 | Trading Engine | Read settings for trade decisions |

---

## Phase 1: Strategy Definition

Before writing any code, answer these questions:

### 1.1 Select Trading Mode(s)

Which mode(s) will this sub-strategy be available in?

| Mode | Timeframe Range | Hold Duration | Description |
|------|-----------------|---------------|-------------|
| `ultra_fast` | 1m - 5m | Seconds to minutes | High-frequency scalping |
| `scalp` | 3m - 15m | Minutes to hours | Quick trades, small profits |
| `swing` | 1h - 4h | Hours to days | Medium-term positions |
| `position` | 4h - 1d | Days to weeks | Long-term trend following |

**Note:** A sub-strategy can be added to multiple modes with different parameter values.

### 1.2 Select Strategy Group

Which strategy group does this sub-strategy belong to?

| Group | Description | Entry Trigger |
|-------|-------------|---------------|
| `breakout` | Price breaks key levels with volume | Volume spike + price breakout |
| `momentum` | Trend continuation patterns | Strong directional movement |
| `reversal` | Counter-trend opportunities | Exhaustion + reversal signals |
| `range` | Mean reversion in sideways markets | Support/resistance bounces |

### 1.3 Define Sub-Strategy

| Attribute | Description | Example |
|-----------|-------------|---------|
| **ID** | Unique snake_case identifier | `ravindra_volume_imbalance` |
| **Name** | Human-readable display name | `Ravindra Volume Imbalance` |
| **Description** | Brief explanation of the strategy | `2-step pattern: Volume Spike → Breakout` |
| **Pattern Steps** | Sequential conditions for entry | Step 1: Volume Spike, Step 2: Breakout |

### 1.4 Define Required Fields

List all configurable parameters:

| Category | Fields | Description |
|----------|--------|-------------|
| **Basic** | `enabled`, `priority` | On/off and execution order |
| **Risk/Reward** | `risk`, `reward`, `min_ratio` | R:R configuration |
| **Pattern Detection** | Strategy-specific parameters | Volume thresholds, candle counts, etc. |
| **Trailing Stop** | `enabled`, `activation`, `milestones[]` | Dynamic stop-loss management |
| **Budget Allocation** | `assigned_budget_usd`, `max_concurrent_trades`, `position_sizing` | Capital management |
| **LLM Validation** | `llm_validation_enabled`, `llm_provider` | AI confirmation (optional) |

---

## Phase 2: Configuration (default-settings.json)

### 2.1 Add to default-settings.json

Location: `/default-settings.json`

Navigate to the appropriate mode and strategy group, then add the sub-strategy:

```json
{
  "modes": {
    "scalp": {
      "strategy_groups": {
        "breakout": {
          "sub_strategies": {
            "your_new_strategy": {
              "enabled": true,
              "_description": "Brief description of what this strategy does",
              "_pattern": "Step 1 → Step 2 → Step 3 (describe the pattern)",
              "settings": {
                "enabled": true,
                "priority": 1,

                "risk_reward": {
                  "risk": 1,
                  "reward": 4,
                  "min_ratio": 3
                },

                "trailing_stop": {
                  "enabled": true,
                  "activation_profit_pct": 2.0,
                  "initial_trail_pct": 0.0,
                  "milestones": [
                    { "trigger_profit_pct": 2.0, "trail_distance_pct": 0.0, "label": "BE" },
                    { "trigger_profit_pct": 3.0, "trail_distance_pct": 1.0, "label": "+1R" }
                  ]
                },

                "pattern_detection": {
                  "param1": 5,
                  "param2": 3.0,
                  "param3": true
                },

                "budget_allocation": {
                  "assigned_budget_usd": 100,
                  "max_concurrent_trades": 1,
                  "position_sizing": "all_in",
                  "use_incremental_equity": true
                },

                "llm_validation_enabled": false,
                "max_concurrent_patterns": 5
              }
            }
          }
        }
      }
    }
  }
}
```

### 2.2 Repeat for Each Mode

If the sub-strategy applies to multiple modes, add to each with appropriate parameters.

---

## Phase 3: Database & Cache Structure

### 3.1 Database Table Structure

Sub-strategy settings are stored in the `sub_strategy_settings` table:

```sql
CREATE TABLE sub_strategy_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    mode VARCHAR(50) NOT NULL,           -- 'scalp', 'swing', etc.
    strategy_group VARCHAR(50) NOT NULL,  -- 'breakout', 'momentum', etc.
    sub_strategy VARCHAR(100) NOT NULL,   -- 'ravindra_volume_imbalance'
    enabled BOOLEAN DEFAULT true,
    settings JSONB NOT NULL,              -- All strategy settings as JSON
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, mode, strategy_group, sub_strategy)
);
```

### 3.2 Database Migration

When adding a new sub-strategy, create a migration if needed:

Location: `/internal/database/migrations/`

```sql
-- Migration: Add new sub-strategy support
-- Only needed if adding new columns, not for new sub-strategies using existing JSONB

-- Example: If adding a new column for strategy-specific tracking
ALTER TABLE sub_strategy_settings
ADD COLUMN IF NOT EXISTS execution_count INTEGER DEFAULT 0;
```

**Note:** Most sub-strategies don't need migrations - they store all settings in the existing `settings` JSONB column.

### 3.3 Redis Cache Structure

The cache follows the **exact same hierarchy** as the strategy structure:

```
Cache Key Pattern:
user:{userID}:mode:{mode}:strategy_group:{group}:sub_strategy:{strategyName}

Example Keys:
user:123e4567-e89b:mode:scalp:strategy_group:breakout:sub_strategy:ravindra_volume_imbalance
user:123e4567-e89b:mode:swing:strategy_group:breakout:sub_strategy:ravindra_volume_imbalance
```

### 3.4 Cache Data Structure

```json
{
  "id": "817dfba8-9cc1-414b-bf99-60ccaf0ed434",
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "mode": "scalp",
  "strategy_group": "breakout",
  "sub_strategy": "ravindra_volume_imbalance",
  "enabled": true,
  "settings": {
    "risk_reward": { "risk": 1, "reward": 4, "min_ratio": 3 },
    "trailing_stop": { ... },
    "pattern_detection": { ... },
    "budget_allocation": { ... }
  },
  "updated_at": "2026-01-29T10:30:00Z"
}
```

### 3.5 Cache Service Integration

Location: `/internal/cache/settings_cache_service.go`

The cache service automatically handles:
- Loading settings on first access
- Invalidating cache on updates
- Write-through pattern (DB + Cache updated together)

```go
// Get sub-strategy settings (checks cache first, then DB)
func (s *SettingsCacheService) GetSubStrategySettings(
    userID, mode, group, strategy string,
) (*SubStrategySettings, error)

// Update sub-strategy settings (writes to DB and cache)
func (s *SettingsCacheService) UpdateSubStrategySettings(
    userID, mode, group, strategy string,
    settings map[string]interface{},
) error

// Invalidate cache (forces reload from DB)
func (s *SettingsCacheService) InvalidateSubStrategyCache(
    userID, mode, group, strategy string,
) error
```

---

## Phase 4: Backend API Implementation

### 4.1 API Endpoints

The following endpoints support sub-strategy operations:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/futures/sub-strategies/{mode}/{group}` | List all sub-strategies for mode/group |
| GET | `/api/futures/sub-strategy/{mode}/{group}/{strategy}` | Get single sub-strategy settings |
| PUT | `/api/futures/sub-strategy/{mode}/{group}/{strategy}` | Update sub-strategy settings |
| POST | `/api/futures/sub-strategy/{mode}/{group}/{strategy}/compare` | Compare user vs default |
| POST | `/api/futures/sub-strategy/{mode}/{group}/{strategy}/reset` | Reset to defaults |

### 4.2 Comparison API

Location: `/internal/api/handlers_strategy_hierarchy.go`

The comparison API compares **user database values** with **default-settings.json values**:

```go
// CompareSubStrategySettings compares user settings with defaults
// Returns list of fields with current value, default value, and match status
func CompareSubStrategySettings(w http.ResponseWriter, r *http.Request) {
    // 1. Get user settings from database (via cache)
    userSettings := cache.GetSubStrategySettings(userID, mode, group, strategy)

    // 2. Get default settings from default-settings.json
    defaultSettings := autopilot.GetSubStrategyDefaults(mode, group, strategy)

    // 3. Compare each field recursively
    comparisons := compareSettingsRecursive(userSettings, defaultSettings)

    // 4. Return comparison results
    json.NewEncoder(w).Encode(ComparisonResponse{
        Success: true,
        Fields: comparisons,
        MatchCount: countMatching(comparisons),
        TotalCount: len(comparisons),
    })
}
```

### 4.3 Comparison Response Structure

```json
{
  "success": true,
  "mode": "scalp",
  "strategy_group": "breakout",
  "sub_strategy": "ravindra_volume_imbalance",
  "total_fields": 33,
  "matching_fields": 30,
  "different_fields": 3,
  "comparisons": [
    {
      "path": "risk_reward.risk",
      "current_value": 1,
      "default_value": 1,
      "match": true
    },
    {
      "path": "pattern_detection.volume_spike_threshold",
      "current_value": 2.5,
      "default_value": 3.0,
      "match": false
    },
    {
      "path": "trailing_stop.milestones[0].trigger_profit_pct",
      "current_value": 2.0,
      "default_value": 2.0,
      "match": true
    }
  ]
}
```

### 4.4 TypeScript Types

Location: `/web/src/types/strategyHierarchy.ts`

Add interface for your strategy settings:

```typescript
/**
 * Your New Strategy settings
 */
export interface YourNewStrategySettings {
  enabled: boolean;
  risk_reward: RiskRewardConfig;
  llm_validation_enabled: boolean;
  trailing_stop: TrailingStopConfig;
  pattern_detection: YourPatternDetectionConfig;
  max_concurrent_patterns: number;
  priority: number;
  budget_allocation?: BudgetAllocationConfig;
}

export interface YourPatternDetectionConfig {
  param1: number;
  param2: number;
  param3: boolean;
}

// Default settings constant (matches default-settings.json)
export const DEFAULT_YOUR_NEW_STRATEGY_SETTINGS: YourNewStrategySettings = {
  enabled: true,
  risk_reward: { risk: 1, reward: 4, min_ratio: 3 },
  // ... copy from default-settings.json
};
```

### 4.5 Fallback Defaults in API Handler

Location: `/internal/api/handlers_strategy_hierarchy.go`

Update `getDefaultSubStrategy()` for fallback when `default-settings.json` is unavailable:

```go
func getDefaultSubStrategy(mode, group, strategy, userID string) *database.SubStrategySettings {
    // Try to load from default-settings.json first
    if defaults := autopilot.GetSubStrategyDefaults(mode, group, strategy); defaults != nil {
        return defaults
    }

    // Fallback hardcoded defaults
    if strategy == "your_new_strategy" {
        return &database.SubStrategySettings{
            SubStrategy:   strategy,
            Mode:          mode,
            StrategyGroup: group,
            Enabled:       true,
            Settings: map[string]interface{}{
                "enabled": true,
                "priority": 1,
                "risk_reward": map[string]interface{}{
                    "risk": 1, "reward": 4, "min_ratio": 3,
                },
                // ... all default settings
            },
        }
    }
    return nil
}
```

---

## Phase 5: Frontend Implementation

### 5.1 Reset Settings Page - Comparison UI

Location: `/web/src/components/SettingsComparisonView.tsx`

The Reset Settings page displays a **side-by-side comparison** of:
- **Current Value:** User's settings from database (via Redis cache)
- **Default Value:** From default-settings.json

```typescript
// In SubStrategyCollapsibleSection component

// Build comparison list for ALL fields
const fieldComparisons: FieldComparison[] = [];

if (settings) {
  // Basic fields
  fieldComparisons.push({
    path: 'enabled',
    current: subStrategy.enabled,
    default: defaultSettings.enabled,
    match: subStrategy.enabled === defaultSettings.enabled,
    inputType: 'toggle',
  });

  // Risk/Reward fields
  fieldComparisons.push({
    path: 'risk_reward.risk',
    current: settings.risk_reward.risk,
    default: defaultSettings.risk_reward.risk,
    match: settings.risk_reward.risk === defaultSettings.risk_reward.risk,
    inputType: 'number',
  });

  // Pattern Detection fields - ADD ALL YOUR STRATEGY FIELDS
  fieldComparisons.push({
    path: 'pattern_detection.param1',
    current: settings.pattern_detection.param1 ?? 5,
    default: defaultSettings.pattern_detection.param1,
    match: (settings.pattern_detection.param1 ?? 5) === defaultSettings.pattern_detection.param1,
    inputType: 'number',
  });

  // Budget Allocation fields
  fieldComparisons.push({
    path: 'budget_allocation.assigned_budget_usd',
    current: settings.budget_allocation?.assigned_budget_usd ?? 100,
    default: defaultSettings.budget_allocation?.assigned_budget_usd ?? 100,
    match: (settings.budget_allocation?.assigned_budget_usd ?? 100) ===
           (defaultSettings.budget_allocation?.assigned_budget_usd ?? 100),
    inputType: 'number',
  });

  // Trailing Stop Milestones - IMPORTANT: Include array fields
  const currentMilestones = settings.trailing_stop.milestones || [];
  const defaultMilestones = defaultSettings.trailing_stop.milestones || [];
  const maxMilestones = Math.max(currentMilestones.length, defaultMilestones.length);

  for (let i = 0; i < maxMilestones; i++) {
    fieldComparisons.push({
      path: `trailing_stop.milestones[${i}].trigger_profit_pct`,
      current: currentMilestones[i]?.trigger_profit_pct ?? 'N/A',
      default: defaultMilestones[i]?.trigger_profit_pct ?? 'N/A',
      match: currentMilestones[i]?.trigger_profit_pct === defaultMilestones[i]?.trigger_profit_pct,
      inputType: 'number',
    });
    fieldComparisons.push({
      path: `trailing_stop.milestones[${i}].trail_distance_pct`,
      current: currentMilestones[i]?.trail_distance_pct ?? 'N/A',
      default: defaultMilestones[i]?.trail_distance_pct ?? 'N/A',
      match: currentMilestones[i]?.trail_distance_pct === defaultMilestones[i]?.trail_distance_pct,
      inputType: 'number',
    });
    fieldComparisons.push({
      path: `trailing_stop.milestones[${i}].label`,
      current: currentMilestones[i]?.label ?? 'N/A',
      default: defaultMilestones[i]?.label ?? 'N/A',
      match: currentMilestones[i]?.label === defaultMilestones[i]?.label,
      inputType: 'text',
    });
  }
}

// Display comparison count
const matchingFields = fieldComparisons.filter(f => f.match).length;
const totalFields = fieldComparisons.length;
// Shows: "Ravindra Volume Imbalance (30/33)" meaning 30 of 33 fields match defaults
```

### 5.2 Reset Settings UI Display

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ SCALP MODE → BREAKOUT → Ravindra Volume Imbalance          (30/33) [Reset] │
├─────────────────────────────────────────────────────────────────────────────┤
│ Field Name                    │ Current Value │ Default Value │ Status     │
├───────────────────────────────┼───────────────┼───────────────┼────────────┤
│ enabled                       │ true          │ true          │ ✓ Match    │
│ risk_reward.risk              │ 1             │ 1             │ ✓ Match    │
│ risk_reward.reward            │ 4             │ 4             │ ✓ Match    │
│ pattern_detection.volume_spike│ 2.5           │ 3.0           │ ✗ Different│
│ trailing_stop.milestones[0]   │ 2.0           │ 2.0           │ ✓ Match    │
│ budget_allocation.budget_usd  │ 150           │ 100           │ ✗ Different│
│ ...                           │ ...           │ ...           │ ...        │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Futures Page - Settings UI

Location: `/web/src/components/settings/ModeStrategySettings.tsx`

Add UI section for your strategy:

```tsx
{isYourNewStrategy && (
  <div className="space-y-4">
    {/* Strategy Header */}
    <div className="bg-purple-900/20 p-3 rounded-lg">
      <h4 className="text-purple-300 font-medium">Your New Strategy</h4>
      <p className="text-xs text-gray-400">Description of the pattern</p>
    </div>

    {/* Pattern Detection Section */}
    <CollapsibleCard title="Pattern Detection" defaultOpen>
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="text-xs text-gray-400">Parameter 1</label>
          <input
            type="number"
            value={localSettings.pattern_detection.param1}
            onChange={(e) => handleFieldChange('pattern_detection.param1', Number(e.target.value))}
            className="w-full bg-gray-700 rounded px-3 py-2"
          />
        </div>
      </div>
    </CollapsibleCard>

    {/* Risk/Reward Section */}
    <CollapsibleCard title="Risk/Reward">
      {/* R:R fields */}
    </CollapsibleCard>

    {/* Trailing Stop Section */}
    <CollapsibleCard title="Trailing Stop">
      {/* Trailing stop with editable milestones */}
    </CollapsibleCard>

    {/* Budget Allocation Section */}
    <CollapsibleCard title="Budget Allocation">
      {/* Budget fields */}
    </CollapsibleCard>
  </div>
)}
```

---

## Phase 6: Build & Verification

### 6.1 Clear Frontend Cache

**CRITICAL:** Always clear the frontend cache after making changes:

```bash
# Clear cache inside the container
docker exec binance-trading-bot-dev rm -rf /app/web/node_modules/.cache /app/web/dist

# Restart the container to rebuild
./scripts/docker-dev.sh
```

### 6.2 Backend Restart Required

The backend loads `default-settings.json` **once at startup** (singleton pattern).

**Container restart is REQUIRED** for:
- Changes to `default-settings.json`
- Changes to Go files

```bash
# Restart container (rebuilds both backend and frontend)
./scripts/docker-dev.sh
```

### 6.3 Wait for Build

Wait for the container to fully rebuild (approximately 60-90 seconds):

```bash
# Check health
curl http://localhost:8094/health

# Watch logs for build completion
./scripts/docker-dev.sh --logs
```

### 6.4 Browser Cache

Clear browser cache if changes don't appear:
- **Windows:** `Ctrl+Shift+R`
- **Mac:** `Cmd+Shift+R`
- **Or:** DevTools → Network → Check "Disable cache"

### 6.5 Verify Implementation

1. **Reset Settings Page:**
   - Navigate to `/reset-settings`
   - Expand your mode → Strategy Group → Sub-Strategy
   - Verify ALL fields are displayed with correct defaults
   - Verify field count matches expected (e.g., "30/33")
   - Test "Reset to Defaults" button

2. **Futures Page:**
   - Navigate to `/futures`
   - Go to Settings → Modes → Your Mode → Strategy Group
   - Expand the sub-strategy
   - Verify all fields are editable
   - Save changes and verify persistence
   - Refresh page and verify values persisted

3. **API Verification:**
   ```bash
   # Get auth token
   TOKEN=$(curl -s -X POST 'http://localhost:8094/api/auth/login' \
     -H 'Content-Type: application/json' \
     -d '{"email":"admin@binance-bot.local","password":"Weber@#2025"}' \
     | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

   # Check sub-strategy settings
   curl -s "http://localhost:8094/api/futures/sub-strategies/scalp/breakout" \
     -H "Authorization: Bearer $TOKEN"
   ```

---

## Checklist

Use this checklist when adding a new sub-strategy:

### Strategy Definition
- [ ] Mode(s) selected: ____________________
- [ ] Strategy group selected: ____________________
- [ ] Sub-strategy ID defined: ____________________
- [ ] All required fields listed

### Configuration (default-settings.json)
- [ ] Added to `default-settings.json` for each mode
- [ ] All parameters have sensible defaults
- [ ] JSON syntax validated

### Database & Cache
- [ ] Database migration created (if new columns needed)
- [ ] Cache key pattern follows hierarchy
- [ ] Cache service handles new strategy

### Backend API
- [ ] TypeScript types added to `strategyHierarchy.ts`
- [ ] Default settings constant added
- [ ] Fallback defaults added to `handlers_strategy_hierarchy.go`
- [ ] Comparison API returns all fields

### Frontend - Reset Settings
- [ ] All field comparisons added to `SettingsComparisonView.tsx`
- [ ] Nested objects handled (risk_reward, pattern_detection, etc.)
- [ ] Array fields handled (milestones)
- [ ] Field count displays correctly

### Frontend - Futures Page
- [ ] UI controls added to `ModeStrategySettings.tsx`
- [ ] All fields are editable
- [ ] Save functionality works

### Build & Test
- [ ] Frontend cache cleared: `docker exec binance-trading-bot-dev rm -rf /app/web/node_modules/.cache /app/web/dist`
- [ ] Container restarted: `./scripts/docker-dev.sh`
- [ ] Health check passed: `curl http://localhost:8094/health`
- [ ] Reset Settings page verified (all fields visible)
- [ ] Futures page verified (all fields editable)
- [ ] Reset to defaults tested
- [ ] Settings persistence verified (save, refresh, check)
- [ ] Browser cache cleared if needed

### Git
- [ ] Changes committed with descriptive message
- [ ] Pushed to GitHub

---

## File Reference

| File | Purpose |
|------|---------|
| `/default-settings.json` | Source of truth for all default settings |
| `/web/src/types/strategyHierarchy.ts` | TypeScript interfaces and defaults |
| `/web/src/components/SettingsComparisonView.tsx` | Reset Settings comparison UI |
| `/web/src/components/settings/ModeStrategySettings.tsx` | Futures page strategy editor UI |
| `/internal/api/handlers_strategy_hierarchy.go` | API handlers and fallback defaults |
| `/internal/database/models_user_settings.go` | Go structs for database |
| `/internal/database/migrations/` | Database migration files |
| `/internal/cache/settings_cache_service.go` | Redis cache integration |

---

## Troubleshooting

### Changes Not Appearing

**Problem:** Made code changes but UI shows old values.

**Solution - Frontend Cache:**
```bash
docker exec binance-trading-bot-dev rm -rf /app/web/node_modules/.cache /app/web/dist
./scripts/docker-dev.sh
```

**Solution - Backend Singleton:**
- The backend loads `default-settings.json` once at startup
- Container restart required: `./scripts/docker-dev.sh`

**Solution - Browser Cache:**
- Hard refresh: `Ctrl+Shift+R` (Windows) or `Cmd+Shift+R` (Mac)
- Or open DevTools → Network → Check "Disable cache"

### Field Count Mismatch

**Problem:** Reset Settings shows fewer fields than expected (e.g., 22 instead of 33).

**Solution:**
- Check `fieldComparisons.push()` calls in `SettingsComparisonView.tsx`
- Ensure ALL fields are added, including:
  - Basic fields (enabled, priority)
  - Nested objects (risk_reward.*, pattern_detection.*)
  - Array fields (trailing_stop.milestones[*])
- Verify default settings constant in `strategyHierarchy.ts`

### API Returns Old Data

**Problem:** API returns stale settings.

**Solution:**
1. Check Redis cache: `docker exec binance-bot-redis redis-cli KEYS "*sub_strategy*"`
2. Clear specific cache: Delete relevant keys
3. Verify database has correct values via pgAdmin
4. Check API handler is reading from correct source

### Reset to Defaults Not Working

**Problem:** Clicking "Reset to Defaults" doesn't restore values.

**Solution:**
1. Check `handleReset()` function in `SubStrategyCollapsibleSection`
2. Verify default settings match `default-settings.json`
3. Check API endpoint returns success
4. Verify cache is invalidated after reset

---

## Example: Complete Sub-Strategy Addition

Here's a complete example of adding a hypothetical "RSI Divergence" sub-strategy:

### Step 1: Define Strategy
- **Modes:** scalp, swing
- **Group:** reversal
- **ID:** `rsi_divergence`
- **Pattern:** RSI divergence from price action signals reversal

### Step 2: Add to default-settings.json
```json
"reversal": {
  "sub_strategies": {
    "rsi_divergence": {
      "enabled": true,
      "_description": "RSI divergence detection for reversal entries",
      "settings": {
        "enabled": true,
        "priority": 1,
        "rsi_period": 14,
        "divergence_lookback": 5,
        "min_divergence_pct": 2.0,
        "risk_reward": { "risk": 1, "reward": 3, "min_ratio": 2 },
        "trailing_stop": {
          "enabled": true,
          "activation_profit_pct": 1.5,
          "initial_trail_pct": 0.5,
          "milestones": [
            { "trigger_profit_pct": 1.5, "trail_distance_pct": 0.0, "label": "BE" }
          ]
        },
        "budget_allocation": {
          "assigned_budget_usd": 50,
          "max_concurrent_trades": 2,
          "position_sizing": "fixed_percent",
          "use_incremental_equity": false
        }
      }
    }
  }
}
```

### Step 3: Add TypeScript Types
```typescript
// In strategyHierarchy.ts
export interface RSIDivergenceSettings {
  enabled: boolean;
  priority: number;
  rsi_period: number;
  divergence_lookback: number;
  min_divergence_pct: number;
  risk_reward: RiskRewardConfig;
  trailing_stop: TrailingStopConfig;
  budget_allocation: BudgetAllocationConfig;
}

export const DEFAULT_RSI_DIVERGENCE_SETTINGS: RSIDivergenceSettings = {
  enabled: true,
  priority: 1,
  rsi_period: 14,
  divergence_lookback: 5,
  min_divergence_pct: 2.0,
  // ... rest matches default-settings.json
};
```

### Step 4: Add to Reset Settings Comparison
```typescript
// In SettingsComparisonView.tsx
if (isRSIDivergence && settings) {
  fieldComparisons.push({
    path: 'rsi_period',
    current: settings.rsi_period ?? 14,
    default: 14,
    match: (settings.rsi_period ?? 14) === 14,
    inputType: 'number',
  });
  fieldComparisons.push({
    path: 'divergence_lookback',
    current: settings.divergence_lookback ?? 5,
    default: 5,
    match: (settings.divergence_lookback ?? 5) === 5,
    inputType: 'number',
  });
  // ... add ALL fields
}
```

### Step 5: Add Futures Page UI
```tsx
// In ModeStrategySettings.tsx
{isRSIDivergence && (
  <div className="space-y-4">
    <CollapsibleCard title="RSI Settings" defaultOpen>
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label>RSI Period</label>
          <input type="number" value={settings.rsi_period} onChange={...} />
        </div>
        <div>
          <label>Divergence Lookback</label>
          <input type="number" value={settings.divergence_lookback} onChange={...} />
        </div>
      </div>
    </CollapsibleCard>
  </div>
)}
```

### Step 6: Build and Verify
```bash
# Clear cache and rebuild
docker exec binance-trading-bot-dev rm -rf /app/web/node_modules/.cache /app/web/dist
./scripts/docker-dev.sh

# Wait for health
curl http://localhost:8094/health

# Test Reset Settings page - verify all fields appear
# Test Futures page - verify all fields are editable
# Test persistence - save, refresh, verify values persist
```

---

*Document Version: 2.0*
*Last Updated: January 2026*
*Based on Ravindra Volume Imbalance implementation*
