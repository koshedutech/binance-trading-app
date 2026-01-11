# Story 4.11: DB-First Mode Enabled/Disabled Status

**Story ID:** MODE-4.11
**Epic:** Epic 4 - Database-First Mode Configuration System
**Priority:** P1 (High - Critical for Multi-User Support)
**Estimated Effort:** 4 hours
**Author:** BMAD Agent
**Status:** Ready for Development

---

## Problem Statement

### Current Behavior (INCORRECT)
The mode orchestration in `ginie_autopilot.go` determines which trading modes are enabled by reading from:
- `ga.config.EnableScalpMode` - Config struct field
- `ga.config.EnableSwingMode` - Config struct field
- `ga.config.EnablePositionMode` - Config struct field
- `currentSettings.UltraFastEnabled` - JSON file setting

### Evidence of Problem
```
Database shows:        4 modes enabled (scalp, scalp_reentry, swing, ultra_fast)
MODE-ORCHESTRATION:    2 modes enabled [SCALP, SWING]
UI displays:           4/4 enabled
```

The trading logic is NOT reading from the database `user_mode_configs` table, causing a disconnect between what users configure in the UI and what the bot actually trades.

### Expected Behavior (CORRECT)
Mode enabled/disabled status should be read from the `user_mode_configs` table in PostgreSQL, ensuring:
- Per-user mode configurations work correctly
- UI changes immediately affect trading behavior
- Single source of truth (database)

---

## User Story

> As a trader,
> When I enable or disable a trading mode in the UI,
> I expect the bot to immediately respect that setting,
> So that I have full control over which modes are actively trading.

---

## Findings

### Location 1: Mode Integration Status Check
**File:** `internal/autopilot/ginie_autopilot.go`
**Lines:** 1796-1815

```go
// Mode Integration Status (Story 2.5 verification)
modesEnabled := 0
var enabledModes []string
if ga.config.EnableScalpMode {           // <-- OLD: reads from config struct
    modesEnabled++
    enabledModes = append(enabledModes, "SCALP")
}
if ga.config.EnableSwingMode {           // <-- OLD: reads from config struct
    modesEnabled++
    enabledModes = append(enabledModes, "SWING")
}
if ga.config.EnablePositionMode {        // <-- OLD: reads from config struct
    modesEnabled++
    enabledModes = append(enabledModes, "POSITION")
}
currentSettings := GetSettingsManager().GetCurrentSettings()
if currentSettings.UltraFastEnabled {    // <-- OLD: reads from JSON file
    modesEnabled++
    enabledModes = append(enabledModes, "ULTRA-FAST")
}
```

**Issue:** This code does NOT check `scalp_reentry` mode at all, and reads from old sources instead of database.

### Location 2: Scan Trigger Checks
**File:** `internal/autopilot/ginie_autopilot.go`
**Lines:** 1835-1860 (approximate)

```go
// Scan based on mode intervals
if ga.config.EnableScalpMode && now.Sub(lastScalpScan) >= time.Duration(ga.config.ScalpScanInterval)*time.Second {
    log.Printf("[MODE-SCAN] Scanning SCALP mode (interval: %ds)", ga.config.ScalpScanInterval)
    ga.scanForMode(GinieModeScalp)
    lastScalpScan = now
    scansPerformed++
}

if ga.config.EnableSwingMode && now.Sub(lastSwingScan) >= time.Duration(ga.config.SwingScanInterval)*time.Second {
    log.Printf("[MODE-SCAN] Scanning SWING mode (interval: %ds)", ga.config.SwingScanInterval)
    ga.scanForMode(GinieModeSwing)
    lastSwingScan = now
    scansPerformed++
}

// Similar for position and ultra_fast modes...
```

**Issue:** The enabled check uses config struct, not database.

### Location 3: Scalp-Reentry Mode Missing
**File:** `internal/autopilot/ginie_autopilot.go`

**Issue:** The mode orchestration does not check if `scalp_reentry` mode is enabled. It's completely missing from the enabled modes list and scan triggers.

---

## Acceptance Criteria

### AC4.11.1: Mode Status Read from Database
- [ ] Mode enabled/disabled status is read from `user_mode_configs` table
- [ ] Uses `GetUserModeConfigFromDB(ctx, db, userID, modeName)` method
- [ ] All 5 modes checked: scalp, swing, position, ultra_fast, scalp_reentry
- [ ] Log shows: `[MODE-ORCHESTRATION] Multi-mode trading active: X modes enabled [from DB]`

### AC4.11.2: Scan Triggers Use Database Status
- [ ] Scalp scan trigger checks `enabledModesMap["scalp"]` from database
- [ ] Swing scan trigger checks `enabledModesMap["swing"]` from database
- [ ] Position scan trigger checks `enabledModesMap["position"]` from database
- [ ] Ultra-fast scan trigger checks `enabledModesMap["ultra_fast"]` from database
- [ ] Scalp-reentry mode is included in orchestration

### AC4.11.3: UI and Trading Logic Synchronized
- [ ] When user disables a mode in UI, bot stops scanning that mode
- [ ] When user enables a mode in UI, bot starts scanning that mode
- [ ] No restart required for mode changes to take effect
- [ ] Log shows correct enabled count matching database

### AC4.11.4: Logging Shows Database Source
- [ ] Logs include "(from DB)" or "DB-enabled" indicator
- [ ] Each mode scan shows which source determined enabled status
- [ ] Warning logged if database lookup fails

---

## Technical Implementation

### Step 1: Create Enabled Modes Map from Database

Replace the old config-based checks with:

```go
// Mode Integration Status - DB-FIRST approach
modesEnabled := 0
var enabledModes []string
ctx := context.Background()
settingsManager := GetSettingsManager()

// Build enabled modes map from database
enabledModesMap := make(map[string]bool)
modeChecks := []struct {
    modeName    string
    displayName string
}{
    {"scalp", "SCALP"},
    {"swing", "SWING"},
    {"position", "POSITION"},
    {"ultra_fast", "ULTRA-FAST"},
    {"scalp_reentry", "SCALP-REENTRY"},
}

for _, mc := range modeChecks {
    modeConfig, err := settingsManager.GetUserModeConfigFromDB(ctx, ga.db, ga.userID, mc.modeName)
    if err != nil {
        log.Printf("[MODE-ORCHESTRATION] Warning: Failed to get %s config from DB: %v", mc.modeName, err)
        continue
    }
    if modeConfig.Enabled {
        modesEnabled++
        enabledModes = append(enabledModes, mc.displayName)
        enabledModesMap[mc.modeName] = true
    }
}
```

### Step 2: Update Scan Triggers

Replace:
```go
if ga.config.EnableScalpMode && now.Sub(lastScalpScan) >= ...
```

With:
```go
if enabledModesMap["scalp"] && now.Sub(lastScalpScan) >= ...
```

### Step 3: Add Scalp-Reentry to Orchestration

Add scan trigger for scalp_reentry mode if it doesn't exist:
```go
if enabledModesMap["scalp_reentry"] && now.Sub(lastScalpReentryScan) >= ... {
    log.Printf("[MODE-SCAN] Scanning SCALP-REENTRY mode (DB-enabled)")
    ga.scanForMode(GinieModeScalpReentry)
    lastScalpReentryScan = now
    scansPerformed++
}
```

---

## Database Schema Reference

**Table:** `user_mode_configs`

```sql
SELECT mode_name, enabled
FROM user_mode_configs
WHERE user_id = '35d1a6ba-2143-4327-8e28-1b7417281b97';

   mode_name   | enabled
---------------+---------
 position      | f
 scalp         | t
 scalp_reentry | t
 swing         | t
 ultra_fast    | t
```

**Repository Method:**
```go
func (sm *SettingsManager) GetUserModeConfigFromDB(ctx context.Context, db *database.Repository, userID, modeName string) (*ModeFullConfig, error)
```

Returns `ModeFullConfig` which has `Enabled bool` field.

---

## Testing Requirements

### Test 1: Verify Database-Driven Mode Count
```bash
# 1. Check database enabled modes
docker exec binance-bot-postgres psql -U trading_bot -d trading_bot -c \
  "SELECT mode_name, enabled FROM user_mode_configs WHERE user_id = '<user_id>' ORDER BY mode_name;"

# 2. Check logs for matching count
docker logs binance-trading-bot-dev 2>&1 | grep "MODE-ORCHESTRATION"

# Expected: Log shows same count as database enabled modes
```

### Test 2: Disable Mode and Verify
```bash
# 1. Disable scalp mode in database
docker exec binance-bot-postgres psql -U trading_bot -d trading_bot -c \
  "UPDATE user_mode_configs SET enabled = false WHERE user_id = '<user_id>' AND mode_name = 'scalp';"

# 2. Wait for next scan cycle (check logs)
docker logs binance-trading-bot-dev --tail 50 2>&1 | grep "MODE-ORCHESTRATION"

# Expected: SCALP no longer in enabled modes list
# Expected: No "[MODE-SCAN] Scanning SCALP mode" logs
```

### Test 3: Enable Mode and Verify
```bash
# 1. Enable position mode in database
docker exec binance-bot-postgres psql -U trading_bot -d trading_bot -c \
  "UPDATE user_mode_configs SET enabled = true WHERE user_id = '<user_id>' AND mode_name = 'position';"

# 2. Wait for next scan cycle
docker logs binance-trading-bot-dev --tail 50 2>&1 | grep -E "MODE-ORCHESTRATION|MODE-SCAN.*POSITION"

# Expected: POSITION now in enabled modes list
# Expected: "[MODE-SCAN] Scanning POSITION mode" logs appear
```

### Test 4: Verify Scalp-Reentry Included
```bash
# Check that scalp_reentry mode appears in orchestration
docker logs binance-trading-bot-dev 2>&1 | grep "MODE-ORCHESTRATION" | tail -5

# Expected: SCALP-REENTRY appears in enabled modes list (if enabled in DB)
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/autopilot/ginie_autopilot.go` | Update mode orchestration (lines 1796-1860) |

---

## Dependencies

### Prerequisites
- Story 4.9: Data migration complete (mode configs in database)
- `GetUserModeConfigFromDB()` method exists in settings.go
- `ga.db` and `ga.userID` fields exist in GinieAutopilot struct

### Blocks
- None

---

## Definition of Done

- [ ] Mode enabled status read from `user_mode_configs` table
- [ ] All 5 modes included in orchestration check
- [ ] Scan triggers use database-loaded enabled status
- [ ] Logs show "(from DB)" or equivalent indicator
- [ ] Test 1: Database-driven mode count passes
- [ ] Test 2: Disable mode test passes
- [ ] Test 3: Enable mode test passes
- [ ] Test 4: Scalp-reentry included test passes
- [ ] No regression in signal detection
- [ ] Code review approved

---

## Notes

### Why This Matters
Without this fix, users can configure modes in the UI but the bot ignores those settings. This breaks the multi-user architecture where each user should have independent mode configurations.

### Scan Intervals
This story focuses on **enabled/disabled status**. Scan intervals still come from config struct for now. A future story can migrate scan intervals to database if needed.

### Error Handling
If database lookup fails for a mode, that mode should be treated as **disabled** (safe default). Log a warning but don't block other modes from being checked.

---

## Related Stories

- **Story 4.5:** Apply per-mode confidence (prerequisite)
- **Story 4.6:** Remove hardcoded confidence (prerequisite)
- **Story 4.9:** Data migration (prerequisite)
- **Story 4.12:** Scan intervals from database (future)

---

## Approval Sign-Off

- **Scrum Master**: Pending
- **Developer**: Pending
- **Test Architect**: Pending
- **Product Manager**: Pending
