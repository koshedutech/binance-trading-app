# JSON Fallback Removal - Status and Action Plan

## Objective
Remove all JSON file fallbacks from the settings system to ensure database-first operation for all runtime trading decisions.

## What Has Been Completed

### 1. Settings Manager Refactoring (✅ DONE)
**File**: `/mnt/c/KOSH/binance-trading-bot/internal/autopilot/settings.go`

- **Renamed `GetCurrentSettings()` → `GetDefaultSettings()`**
  - Added deprecation warning
  - Logs when called: "WARNING: GetDefaultSettings() called - this should ONLY be used for defaults, not runtime trading"
  - Kept old method name as deprecated alias for backward compatibility

- **Renamed `GetAllModeConfigs()` → `GetDefaultModeConfigs()`**
  - Added deprecation warning
  - Logs when called
  - Kept old method name as deprecated alias

- **Renamed `GetModeConfig()` → `GetDefaultModeConfig()`**
  - Added deprecation warning
  - Logs when called
  - Kept old method name as deprecated alias

**Purpose**: These methods are now ONLY for:
- Admin sync operations
- New user initialization
- Reset-to-defaults functionality

### 2. API Handler Cleanup (✅ DONE)
**File**: `/mnt/c/KOSH/binance-trading-bot/internal/api/handlers_ginie.go`

- **`handleGetModeConfigs()`**: Removed JSON fallback
  - Now returns error if database fails (no silent fallback to JSON)
  - Requires authentication
  - Logs warning if user is missing modes in database (indicates incomplete initialization)

- **`handleGetModeConfig()`**: Removed JSON fallback
  - Now returns error if database fails
  - Only uses default for specific "mode config not found" error (incomplete user initialization)
  - All other errors return HTTP error response

**Result**: API endpoints are now database-first with no silent JSON fallbacks.

## What Remains To Be Done

### 3. Autopilot Runtime Code (⚠️ MAJOR REFACTORING NEEDED)

**Problem**: The autopilot trading code has **40+ calls** to the deprecated `GetCurrentSettings()` method throughout `ginie_autopilot.go` and related files.

**Current Architecture**:
```go
type GinieAutopilot struct {
    repo   *database.Repository  // ✅ Has database access
    userID string                // ✅ Has user context
    // ... other fields
}
```

**Why This Is Complex**:
Each call to `GetCurrentSettings()` needs to be replaced with database queries that:
1. Use `ga.userID` for user context
2. Call database methods (which don't exist yet for many settings)
3. Handle errors properly
4. Pass context for cancellation/timeout

**Examples of What Needs To Change**:

#### Current Code (BAD):
```go
// From line 1011 - ginie_autopilot.go
settings := settingsManager.GetCurrentSettings()
// Uses JSON file, not user-specific database settings

// From line 2118 - ginie_autopilot.go
currentSettings := settingsManager.GetCurrentSettings()
if currentSettings.UltraFastCircuitBreakerTripped {
    // This reads from JSON, not user's database settings!
}
```

#### What It Should Be (GOOD):
```go
// Need new method: GetUserSettings(ctx, repo, userID)
ctx := context.Background()
settings, err := settingsManager.GetUserSettings(ctx, ga.repo, ga.userID)
if err != nil {
    ga.logger.Error("Failed to load user settings", "error", err)
    return err
}

// For mode configs - already exists!
config, err := sm.GetUserModeConfigFromDB(ctx, ga.repo, ga.userID, "ultra_fast")
if err != nil {
    // Handle error
}
```

### Required Database Methods (To Be Created)

The following methods need to be added to `settings.go` for database-first operation:

```go
// Get complete user settings from database (not JSON)
func (sm *SettingsManager) GetUserSettings(ctx context.Context, db *database.Repository, userID string) (*AutopilotSettings, error)

// Get specific settings fields
func (sm *SettingsManager) GetUserUltraFastSettings(ctx context.Context, db *database.Repository, userID string) (*UltraFastSettings, error)
func (sm *SettingsManager) GetUserScalpReentrySettings(ctx context.Context, db *database.Repository, userID string) (*ScalpReentryConfig, error)
func (sm *SettingsManager) GetUserEarlyWarningSettings(ctx context.Context, db *database.Repository, userID string) (*EarlyWarningConfig, error)

// Update user settings in database
func (sm *SettingsManager) UpdateUserSettings(ctx context.Context, db *database.Repository, userID string, settings *AutopilotSettings) error
```

### Database Schema Changes Required

Need to add tables for:
- `user_autopilot_settings` - Main settings table (dynamic SL/TP, circuit breaker, etc.)
- `user_ultrafast_settings` - Ultra-fast mode settings
- `user_scalp_reentry_settings` - Scalp re-entry settings
- `user_early_warning_settings` - Early warning monitor settings

**OR** use JSONB columns in existing `user_mode_configs` table.

### Files That Need Major Refactoring

| File | Calls to Fix | Complexity |
|------|--------------|------------|
| `ginie_autopilot.go` | 40+ | HIGH |
| `ginie_analyzer.go` | 5+ | MEDIUM |
| `scalp_reentry_logic.go` | 3+ | LOW |
| `futures_controller.go` | 2+ | LOW |

## Current State Summary

### ✅ What Works (Database-First)
- API endpoints for mode configs (GET/POST/PUT)
- Admin settings sync
- New user initialization
- User mode config storage and retrieval

### ⚠️ What Still Uses JSON Fallback
- **ALL runtime autopilot trading decisions**
- Circuit breaker status
- Ultra-fast settings (scan interval, max positions, etc.)
- Scalp re-entry configuration
- Early warning monitor settings
- Morning auto-block settings
- Dynamic SL/TP configuration

### Why The Deprecated Methods Still Exist

The deprecated methods (`GetCurrentSettings()`, etc.) are kept with deprecation warnings because:
1. **Backward Compatibility**: Prevents immediate breaking changes
2. **Visibility**: Logs every call so we can identify all usage points
3. **Gradual Migration**: Allows us to refactor incrementally

## Recommended Action Plan

### Phase 1: Database Infrastructure (NEXT STEP)
1. Design database schema for user-specific autopilot settings
2. Create migration scripts
3. Implement `GetUserSettings()` and related database methods
4. Add error handling and logging

### Phase 2: Autopilot Refactoring (MAJOR EFFORT)
1. Refactor `ginie_autopilot.go` to use database methods
2. Update all calls to pass `ctx`, `ga.repo`, and `ga.userID`
3. Add proper error handling for database failures
4. Test each change thoroughly

### Phase 3: Cleanup (FINAL)
1. Remove deprecated methods entirely
2. Remove JSON file loading for runtime settings
3. Update documentation
4. Add monitoring for database performance

## Testing Strategy

For each refactored section:
1. **Unit Tests**: Mock database responses
2. **Integration Tests**: Test with real database
3. **Load Tests**: Ensure database queries don't slow down trading
4. **Regression Tests**: Verify existing functionality works

## Estimated Effort

- **Phase 1**: 1-2 days (database setup)
- **Phase 2**: 3-5 days (autopilot refactoring)
- **Phase 3**: 1 day (cleanup)

**Total**: ~1 week of development + testing

## Risks

1. **Performance**: Database queries in trading loops could add latency
   - **Mitigation**: Cache settings in memory, refresh periodically

2. **Error Handling**: Database failures during trading could cause losses
   - **Mitigation**: Graceful degradation, stop trading on DB errors

3. **Data Migration**: Existing users need settings migrated from JSON to DB
   - **Mitigation**: Migration script + admin sync endpoint

## Current Logging

All deprecated method calls now log warnings:
- `[SETTINGS] WARNING: GetDefaultSettings() called`
- `[SETTINGS] DEPRECATED: GetCurrentSettings() called`
- `[SETTINGS] WARNING: GetDefaultModeConfigs() called`
- `[SETTINGS] DEPRECATED: GetModeConfig(mode) called`

Monitor logs to identify remaining usage points.

## Conclusion

The foundation is in place (API endpoints, database methods for mode configs), but the core autopilot trading logic still relies on JSON files. The next step is to create the database infrastructure for user-specific settings, then gradually refactor the autopilot code to use it.

**Status**: 40% Complete (API layer done, autopilot runtime needs work)
