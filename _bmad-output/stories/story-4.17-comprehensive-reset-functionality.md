# Story 4.17: Comprehensive Reset Functionality

**Story ID:** SETTINGS-4.17
**Epic:** Epic 4 - Database-First Mode Configuration System
**Priority:** P1 (High - User Experience)
**Estimated Effort:** 12 hours (expanded from 6 hours)
**Author:** BMAD Agent (Bob - Scrum Master)
**Status:** In Development
**Depends On:** Story 4.13, Story 4.14, Story 4.16

**Latest Update:** 2026-01-06 - Expanded to include dedicated Reset Settings Page with centralized reset management, individual group resets, and master "Reset All" functionality.

---

## Story Summary

This story implements comprehensive reset functionality across all configuration types in the trading bot. It includes:

1. **Individual Reset Buttons** - Each config section gets a reset button with preview functionality
2. **Reset Preview System** - Shows exact changes before applying (current vs default)
3. **Dedicated Reset Settings Page** - Centralized hub for managing all resets (**NEW**)
4. **Master Reset All** - One-click reset of all settings with combined preview (**NEW**)
5. **User Menu Integration** - Easy access via user menu dropdown (**NEW**)

The expansion to include a dedicated Reset Settings Page provides users with a comprehensive overview of all their settings and the ability to reset individual groups or everything at once from a single location.

---

## Problem Statement

### Current State

Reset functionality is incomplete and inconsistent:
- Some configs have reset buttons, others don't
- No preview dialog showing what will change
- No feedback when settings are already at defaults
- Mode Capital Allocation not stored per-user
- Missing reset for: Scalp Reentry, Hedge, LLM Config, Capital Allocation

### Expected Behavior

Every configuration section should have a reset button that:
1. Shows a preview dialog with exact changes (current vs default)
2. If already at defaults: "Your settings are already set to default"
3. If different: Shows table of changes with risk levels
4. User confirms before applying changes

---

## User Story

> As a trader,
> When I click a reset button for any configuration,
> I want to see exactly what will change before confirming,
> So that I understand the impact and can make an informed decision.

---

## Scope

### Configuration Types Needing Reset

| Config Type | Location | Reset Button Placement |
|-------------|----------|------------------------|
| Mode Config (scalp) | GiniePanel | Near enable/disable toggle |
| Mode Config (swing) | GiniePanel | Near enable/disable toggle |
| Mode Config (position) | GiniePanel | Near enable/disable toggle |
| Mode Config (ultra_fast) | GiniePanel | Near enable/disable toggle |
| Mode Config (scalp_reentry) | GiniePanel | Near enable/disable toggle |
| Scalp Reentry Config | ScalpReentryMonitor | Settings section |
| Hedge Config | HedgeModeMonitor | Settings section |
| Ginie Circuit Breaker | GiniePanel | Circuit Breaker section |
| LLM Config | GiniePanel | LLM Settings section |
| Mode Capital Allocation | GiniePanel | Capital Allocation section |

---

## Acceptance Criteria

### AC4.17.1: Reset Preview Dialog Component
- [ ] Create reusable `ResetConfirmDialog` component
- [ ] Shows loading state while fetching preview
- [ ] If `all_match: true`: Shows "Settings already at default"
- [ ] If differences exist: Shows table with columns:
  - Setting Name (path)
  - Current Value
  - Default Value
  - Risk Level (color coded: red=high, orange=medium, green=low)
- [ ] Confirm and Cancel buttons
- [ ] Proper error handling

### AC4.17.2: Mode Config Reset (Per Mode)
- [ ] Reset button near each mode's enable/disable toggle
- [ ] Calls `POST /api/futures/ginie/modes/:mode/load-defaults?preview=true`
- [ ] Shows preview dialog before applying
- [ ] On confirm, calls without `?preview=true` to apply

### AC4.17.3: Scalp Reentry Config Reset
- [ ] Add reset button in ScalpReentryMonitor settings section
- [ ] Backend endpoint: `POST /api/futures/ginie/scalp-reentry/load-defaults`
- [ ] Supports `?preview=true` for preview mode
- [ ] Store defaults in `default-settings.json`

### AC4.17.4: Hedge Config Reset
- [ ] Add reset button in HedgeModeMonitor settings section
- [ ] Backend endpoint: `POST /api/futures/ginie/hedge/load-defaults`
- [ ] Supports `?preview=true` for preview mode
- [ ] Store defaults in `default-settings.json`

### AC4.17.5: Ginie Circuit Breaker Reset
- [ ] Reset button in Circuit Breaker section
- [ ] Backend endpoint: `POST /api/futures/ginie/circuit-breaker/load-defaults`
- [ ] Supports `?preview=true` for preview mode
- [ ] Store defaults in `default-settings.json`

### AC4.17.6: LLM Config Reset
- [ ] Reset button in LLM Settings section
- [ ] Backend endpoint: `POST /api/futures/ginie/llm-config/load-defaults`
- [ ] Supports `?preview=true` for preview mode
- [ ] Store defaults in `default-settings.json`

### AC4.17.7: Mode Capital Allocation Reset
- [ ] Add capital allocation to `default-settings.json`
- [ ] Store per-user in database (`user_mode_configs` or new table)
- [ ] Backend endpoint: `POST /api/futures/ginie/capital-allocation/load-defaults`
- [ ] Supports `?preview=true` for preview mode
- [ ] Admin changes sync to `default-settings.json`

### AC4.17.8: Admin Sync Integration
- [ ] All new config types sync when admin changes them
- [ ] Verify admin sync works for capital allocation
- [ ] Backup created before sync

### AC4.17.9: Dedicated Reset Settings Page
- [ ] Create `/reset-settings` route in App.tsx
- [ ] Create `ResetSettings.tsx` page component
- [ ] Add "Reset Settings" menu item to user menu dropdown in Header.tsx
- [ ] Page shows all setting groups in card layout:
  - Mode Configurations (Ultra-Fast, Scalp, Swing, Position, Scalp Reentry)
  - Circuit Breaker Settings
  - LLM Configuration
  - Capital Allocation
  - Hedge Mode Settings
- [ ] Each card displays current values preview
- [ ] Each card has individual reset button
- [ ] Visual indicators for settings that differ from defaults

### AC4.17.10: Reset All Functionality
- [ ] Master "Reset All" button at top of Reset Settings page
- [ ] Shows combined preview dialog with ALL changes across all groups
- [ ] Requires explicit confirmation
- [ ] Executes all reset operations in sequence
- [ ] Shows progress indicator during reset
- [ ] Success/error feedback for each group
- [ ] Refreshes all previews after completion

### AC4.17.11: User Menu Integration
- [ ] Add "Reset Settings" item to user menu dropdown
- [ ] Icon: settings/reset icon
- [ ] Navigate to `/reset-settings` on click
- [ ] Menu item visible to all authenticated users
- [ ] Proper routing and navigation

---

## Technical Implementation

### Task 1: Update default-settings.json Structure

Add missing sections:
```json
{
  "metadata": { ... },
  "mode_configs": { ... },
  "scalp_reentry": {
    "enabled": false,
    "reentry_enabled": true,
    "max_reentries": 3,
    "reentry_threshold_percent": 0.5,
    "tp_levels": [1.0, 2.0, 3.0, 5.0],
    "tp_percentages": [25, 25, 25, 25],
    "_risk_info": { "risk_level": "medium", ... }
  },
  "hedge_config": {
    "enabled": false,
    "hedge_ratio": 0.5,
    "trigger_loss_percent": 2.0,
    "max_hedge_positions": 3,
    "_risk_info": { "risk_level": "high", ... }
  },
  "circuit_breaker": {
    "enabled": true,
    "max_daily_loss": 500,
    "max_daily_trades": 50,
    "cooldown_minutes": 30,
    "_risk_info": { "risk_level": "high", ... }
  },
  "llm_config": {
    "provider": "anthropic",
    "model": "claude-3-5-sonnet",
    "temperature": 0.3,
    "max_tokens": 1000,
    "kill_switch_threshold": 3,
    "_risk_info": { "risk_level": "medium", ... }
  },
  "capital_allocation": {
    "total_capital_percent": 100,
    "mode_allocations": {
      "scalp": 30,
      "swing": 40,
      "position": 20,
      "ultra_fast": 5,
      "scalp_reentry": 5
    },
    "reserve_percent": 10,
    "_risk_info": { "risk_level": "high", ... }
  }
}
```

### Task 2: Backend Reset APIs

```go
// internal/api/handlers_settings_defaults.go

// POST /api/futures/ginie/scalp-reentry/load-defaults
func (s *Server) handleLoadScalpReentryDefaults(c *gin.Context)

// POST /api/futures/ginie/hedge/load-defaults
func (s *Server) handleLoadHedgeDefaults(c *gin.Context)

// POST /api/futures/ginie/circuit-breaker/load-defaults
func (s *Server) handleLoadCircuitBreakerDefaults(c *gin.Context)

// POST /api/futures/ginie/llm-config/load-defaults
func (s *Server) handleLoadLLMConfigDefaults(c *gin.Context)

// POST /api/futures/ginie/capital-allocation/load-defaults
func (s *Server) handleLoadCapitalAllocationDefaults(c *gin.Context)
```

### Task 3: Frontend ResetConfirmDialog Component

```tsx
// web/src/components/ResetConfirmDialog.tsx

interface SettingDiff {
  path: string;
  current: any;
  default: any;
  risk_level: 'high' | 'medium' | 'low';
  impact?: string;
}

interface ResetConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  loading: boolean;
  allMatch: boolean;
  differences: SettingDiff[];
}

export function ResetConfirmDialog(props: ResetConfirmDialogProps) {
  // Show loading spinner while fetching preview
  // If allMatch: "Your settings are already set to default"
  // If differences: Show table with color-coded risk levels
  // Confirm/Cancel buttons
}
```

### Task 4: Add Reset Buttons to Components

**GiniePanel.tsx:**
- Add reset button next to each mode toggle
- Add reset button in Circuit Breaker section
- Add reset button in LLM section
- Add reset button in Capital Allocation section

**ScalpReentryMonitor.tsx:**
- Add reset button in settings section

**HedgeModeMonitor.tsx:**
- Add reset button in settings section

### Task 5: Create Dedicated Reset Settings Page

**File: `/web/src/pages/ResetSettings.tsx`**

```tsx
// Dedicated page for managing all reset operations
// Features:
// - Card layout for each setting group
// - Preview current vs default values
// - Individual reset buttons per group
// - Master "Reset All" button
// - Visual indicators for differences
// - Progress tracking for bulk operations

interface SettingGroup {
  id: string;
  title: string;
  description: string;
  endpoint: string;
  currentValues: any;
  defaultValues: any;
  hasDifferences: boolean;
  riskLevel: 'high' | 'medium' | 'low';
}

export function ResetSettings() {
  const settingGroups: SettingGroup[] = [
    { id: 'ultra_fast', title: 'Ultra-Fast Mode', endpoint: '/api/futures/ginie/modes/ultra_fast/load-defaults', ... },
    { id: 'scalp', title: 'Scalp Mode', endpoint: '/api/futures/ginie/modes/scalp/load-defaults', ... },
    { id: 'swing', title: 'Swing Mode', endpoint: '/api/futures/ginie/modes/swing/load-defaults', ... },
    { id: 'position', title: 'Position Mode', endpoint: '/api/futures/ginie/modes/position/load-defaults', ... },
    { id: 'scalp_reentry', title: 'Scalp Reentry', endpoint: '/api/futures/ginie/scalp-reentry/load-defaults', ... },
    { id: 'circuit_breaker', title: 'Circuit Breaker', endpoint: '/api/futures/ginie/circuit-breaker/load-defaults', ... },
    { id: 'llm_config', title: 'LLM Configuration', endpoint: '/api/futures/ginie/llm-config/load-defaults', ... },
    { id: 'capital_allocation', title: 'Capital Allocation', endpoint: '/api/futures/ginie/capital-allocation/load-defaults', ... },
    { id: 'hedge', title: 'Hedge Mode', endpoint: '/api/futures/ginie/hedge/load-defaults', ... },
  ];

  // Layout: Grid of cards, each showing group info + reset button
  // Master reset button at top
}
```

### Task 6: Update App.tsx Routing

```tsx
// web/src/App.tsx
import ResetSettings from './pages/ResetSettings';

// Add route:
<Route path="/reset-settings" element={<ResetSettings />} />
```

### Task 7: Update Header.tsx User Menu

```tsx
// web/src/components/Header.tsx
// Add menu item in user dropdown:

<MenuItem onClick={() => navigate('/reset-settings')}>
  <ListItemIcon>
    <SettingsBackupRestoreIcon fontSize="small" />
  </ListItemIcon>
  <ListItemText>Reset Settings</ListItemText>
</MenuItem>
```

### Task 8: Reset Settings Page Data Fetching Strategy

**On Page Load:**
```tsx
// Fetch preview for ALL groups in parallel
useEffect(() => {
  const fetchAllPreviews = async () => {
    const promises = settingGroups.map(group =>
      fetch(`${group.endpoint}?preview=true`)
    );
    const results = await Promise.all(promises);
    // Update each card with difference count and risk level
  };
  fetchAllPreviews();
}, []);
```

**Individual Reset Flow:**
```tsx
// 1. User clicks reset button on a card
// 2. Show preview dialog (data already fetched on page load)
// 3. User confirms
// 4. POST to endpoint without ?preview=true
// 5. Refresh that group's preview data
// 6. Update card UI
```

**Reset All Flow:**
```tsx
// 1. User clicks "Reset All" button
// 2. Aggregate all previews into combined dialog
// 3. Show risk summary (count high/medium/low changes)
// 4. User confirms
// 5. Execute resets sequentially with progress tracking:
//    - Update progress dialog for each group
//    - Show success/error for each
// 6. Refresh all preview data when complete
```

**Performance Optimization:**
- Cache preview results (5 min TTL)
- Only re-fetch on user action or page refresh
- Parallel fetching for initial load
- Sequential execution for Reset All (safer)

---

## API Reference

### New Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/futures/ginie/scalp-reentry/load-defaults` | Reset scalp reentry config |
| POST | `/api/futures/ginie/hedge/load-defaults` | Reset hedge config |
| POST | `/api/futures/ginie/circuit-breaker/load-defaults` | Reset circuit breaker |
| POST | `/api/futures/ginie/llm-config/load-defaults` | Reset LLM config |
| POST | `/api/futures/ginie/capital-allocation/load-defaults` | Reset capital allocation |

All endpoints support `?preview=true` query parameter.

### Response Format (Preview Mode)

```json
{
  "preview": true,
  "config_type": "scalp_reentry",
  "all_match": false,
  "total_changes": 5,
  "differences": [
    {
      "path": "max_reentries",
      "current": 5,
      "default": 3,
      "risk_level": "medium",
      "impact": "More re-entries = higher risk exposure",
      "recommendation": "Use default for balanced risk"
    }
  ]
}
```

### Response Format (Apply Mode)

```json
{
  "success": true,
  "config_type": "scalp_reentry",
  "changes_applied": 5,
  "message": "Scalp reentry settings reset to defaults"
}
```

---

## UI Design

### Dedicated Reset Settings Page Layout

```
┌─────────────────────────────────────────────────────────────────┐
│  RESET SETTINGS                              [🔄 Reset All]     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────────────────┐  ┌─────────────────────────┐ │
│  │ 🚀 ULTRA-FAST MODE      [🔄] │  │ 📊 SCALP MODE      [🔄] │ │
│  │ ───────────────────────────  │  │ ───────────────────────  │ │
│  │ • Confidence: 45% → 40%      │  │ • Leverage: 15x → 10x   │ │
│  │ • Leverage: 20x → 15x        │  │ • Size: $500 → $200     │ │
│  │ 🟡 2 differences             │  │ 🔴 3 differences        │ │
│  └──────────────────────────────┘  └─────────────────────────┘ │
│                                                                 │
│  ┌──────────────────────────────┐  ┌─────────────────────────┐ │
│  │ 📈 SWING MODE           [🔄] │  │ 💼 POSITION MODE   [🔄] │ │
│  │ ───────────────────────────  │  │ ───────────────────────  │ │
│  │ ✅ Already at defaults       │  │ • Leverage: 5x → 3x     │ │
│  │                              │  │ 🟢 1 difference         │ │
│  └──────────────────────────────┘  └─────────────────────────┘ │
│                                                                 │
│  ┌──────────────────────────────┐  ┌─────────────────────────┐ │
│  │ 🔄 SCALP REENTRY        [🔄] │  │ 🛡️ CIRCUIT BREAKER [🔄] │ │
│  │ ───────────────────────────  │  │ ───────────────────────  │ │
│  │ • Max Reentries: 5 → 3       │  │ • Max Loss: $1000→$500  │ │
│  │ 🟡 2 differences             │  │ 🔴 2 differences        │ │
│  └──────────────────────────────┘  └─────────────────────────┘ │
│                                                                 │
│  ┌──────────────────────────────┐  ┌─────────────────────────┐ │
│  │ 🤖 LLM CONFIG           [🔄] │  │ 💰 CAPITAL ALLOC   [🔄] │ │
│  │ ───────────────────────────  │  │ ───────────────────────  │ │
│  │ • Model: opus → sonnet       │  │ • Scalp: 50% → 30%      │ │
│  │ 🟢 1 difference              │  │ 🔴 4 differences        │ │
│  └──────────────────────────────┘  └─────────────────────────┘ │
│                                                                 │
│  ┌──────────────────────────────┐                              │
│  │ ⚖️ HEDGE MODE           [🔄] │                              │
│  │ ───────────────────────────  │                              │
│  │ ✅ Already at defaults       │                              │
│  └──────────────────────────────┘                              │
└─────────────────────────────────────────────────────────────────┘
```

### Reset All Confirmation Dialog

```
┌─────────────────────────────────────────────────────────────────┐
│  Reset ALL Settings to Defaults?                            ✕  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ⚠️ WARNING: This will reset ALL configuration groups          │
│                                                                 │
│  Changes will be applied to:                                    │
│  ✓ Ultra-Fast Mode (2 changes)                                  │
│  ✓ Scalp Mode (3 changes)                                       │
│  ✓ Position Mode (1 change)                                     │
│  ✓ Scalp Reentry (2 changes)                                    │
│  ✓ Circuit Breaker (2 changes)                                  │
│  ✓ LLM Config (1 change)                                        │
│  ✓ Capital Allocation (4 changes)                               │
│                                                                 │
│  Total: 15 settings will change across 7 groups                 │
│                                                                 │
│  Risk Summary:                                                  │
│  🔴 High Risk Changes: 9                                        │
│  🟡 Medium Risk Changes: 4                                      │
│  🟢 Low Risk Changes: 2                                         │
│                                                                 │
│                      [Cancel]  [View Details]  [Reset All]      │
└─────────────────────────────────────────────────────────────────┘
```

### Reset All Progress Dialog

```
┌─────────────────────────────────────────────────────────────────┐
│  Resetting All Settings...                                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ✅ Ultra-Fast Mode reset (2 changes)                           │
│  ✅ Scalp Mode reset (3 changes)                                │
│  ✅ Swing Mode (no changes needed)                              │
│  ✅ Position Mode reset (1 change)                              │
│  🔄 Scalp Reentry resetting...                                  │
│  ⏳ Circuit Breaker (pending)                                   │
│  ⏳ LLM Config (pending)                                        │
│  ⏳ Capital Allocation (pending)                                │
│  ⏳ Hedge Mode (pending)                                        │
│                                                                 │
│  Progress: 4 / 9 groups                                         │
│  ▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░ 44%                                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Reset Button Placement

```
┌─────────────────────────────────────────────────────┐
│  SCALP MODE                         [🔄 Reset] [✓]  │
│  ─────────────────────────────────────────────────  │
│  Confidence: 45%    Leverage: 10x    Size: $200     │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  CIRCUIT BREAKER                    [🔄 Reset]      │
│  ─────────────────────────────────────────────────  │
│  Max Daily Loss: $500   Cooldown: 30min             │
└─────────────────────────────────────────────────────┘
```

### Reset Confirm Dialog

```
┌─────────────────────────────────────────────────────┐
│  Reset Scalp Mode to Defaults?                   ✕  │
├─────────────────────────────────────────────────────┤
│                                                     │
│  The following settings will change:                │
│                                                     │
│  ┌───────────────┬──────────┬─────────┬──────────┐ │
│  │ Setting       │ Current  │ Default │ Risk     │ │
│  ├───────────────┼──────────┼─────────┼──────────┤ │
│  │ min_confidence│ 45%      │ 40%     │ 🟢 Low   │ │
│  │ leverage      │ 15x      │ 10x     │ 🔴 High  │ │
│  │ base_size_usd │ $500     │ $200    │ 🔴 High  │ │
│  │ stop_loss_%   │ 1.5%     │ 2.0%    │ 🟠 Med   │ │
│  └───────────────┴──────────┴─────────┴──────────┘ │
│                                                     │
│  ⚠️ 2 high-risk settings will change               │
│                                                     │
│              [Cancel]        [Reset to Defaults]    │
└─────────────────────────────────────────────────────┘
```

### Already at Defaults Dialog

```
┌─────────────────────────────────────────────────────┐
│  Reset Scalp Mode to Defaults?                   ✕  │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ✅ Your settings are already set to default        │
│                                                     │
│  No changes needed.                                 │
│                                                     │
│                            [OK]                     │
└─────────────────────────────────────────────────────┘
```

---

## Testing Requirements

### Test 1: Preview Shows Differences
```bash
curl -X POST "http://localhost:8094/api/futures/ginie/modes/scalp/load-defaults?preview=true" \
  -H "Authorization: Bearer $TOKEN"
# Expected: List of differences with risk levels
```

### Test 2: Already at Defaults
```bash
# First reset, then try preview again
curl -X POST "http://localhost:8094/api/futures/ginie/modes/scalp/load-defaults" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "http://localhost:8094/api/futures/ginie/modes/scalp/load-defaults?preview=true" \
  -H "Authorization: Bearer $TOKEN"
# Expected: { "all_match": true, "differences": [] }
```

### Test 3: Capital Allocation Per-User
```bash
# User 1 changes allocation
curl -X PUT "http://localhost:8094/api/futures/ginie/capital-allocation" \
  -H "Authorization: Bearer $USER1_TOKEN" \
  -d '{"scalp": 50, "swing": 30, "position": 20}'

# User 2 should still have defaults
curl -X GET "http://localhost:8094/api/futures/ginie/capital-allocation" \
  -H "Authorization: Bearer $USER2_TOKEN"
# Expected: Default allocation
```

### Test 4: Admin Sync
```bash
# Admin changes capital allocation
curl -X PUT "http://localhost:8094/api/futures/ginie/capital-allocation" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"scalp": 40, "swing": 35, "position": 25}'

# Verify default-settings.json updated
cat default-settings.json | jq '.capital_allocation'
# Expected: Updated values
```

### Test 5: Reset Settings Page Navigation
```bash
# Access via user menu
1. Login to app
2. Click user menu in header
3. Click "Reset Settings" menu item
4. Verify navigation to /reset-settings
5. Verify all setting group cards displayed
```

### Test 6: Individual Group Reset from Page
```bash
# Test individual reset button on Reset Settings page
1. Navigate to /reset-settings
2. Modify scalp mode settings
3. Refresh page - see "2 differences" badge on Scalp card
4. Click reset button on Scalp card
5. Verify preview dialog shows
6. Confirm reset
7. Verify card shows "Already at defaults"
```

### Test 7: Reset All Functionality
```bash
# Test master reset all button
1. Modify multiple setting groups:
   - Change scalp mode leverage
   - Change circuit breaker max loss
   - Change capital allocation
2. Navigate to /reset-settings
3. Click "Reset All" button at top
4. Verify combined preview shows all 3 groups
5. Verify total change count
6. Verify risk summary
7. Confirm reset
8. Verify progress dialog shows each group
9. Verify all cards show "Already at defaults"
```

### Test 8: Visual Indicators on Reset Page
```bash
# Test difference indicators
1. Modify only high-risk settings (leverage, size)
2. Navigate to /reset-settings
3. Verify cards show:
   - Red indicator for high-risk differences
   - Orange for medium-risk
   - Green checkmark for "already at defaults"
4. Verify difference count badges
```

---

## Definition of Done

### Core Reset Functionality (AC 4.17.1-4.17.8)
- [ ] All config types have reset buttons in their respective sections
- [ ] ResetConfirmDialog component created and reusable
- [ ] Preview shows exact changes with risk levels
- [ ] "Already at defaults" message when no changes
- [ ] Capital allocation stored per-user
- [ ] Admin sync works for all config types
- [ ] Backend endpoints support `?preview=true` parameter

### Dedicated Reset Settings Page (AC 4.17.9-4.17.11)
- [ ] `/reset-settings` route created in App.tsx
- [ ] ResetSettings.tsx page component implemented
- [ ] "Reset Settings" menu item added to Header user menu
- [ ] Page displays all 9 setting groups in card layout
- [ ] Each card shows current vs default preview
- [ ] Visual indicators for differences (red/orange/green)
- [ ] Individual reset buttons work on each card
- [ ] Master "Reset All" button functional
- [ ] Combined preview dialog for Reset All
- [ ] Progress tracking during bulk reset operations
- [ ] Success/error feedback for all operations

### Testing & Quality
- [ ] All unit tests pass
- [ ] Integration tests for all reset endpoints
- [ ] E2E tests for Reset Settings page
- [ ] Visual regression tests for dialogs
- [ ] Accessibility review completed
- [ ] Code review approved

---

## Related Stories

- **Story 4.13:** Default Settings JSON Foundation
- **Story 4.14:** New User & Load Defaults
- **Story 4.15:** Admin Settings Sync
- **Story 4.16:** Settings Comparison & Risk Display
