# Settings Refactoring Plan

## Objective
Refactor the settings system to be database-first, with optional admin sync to default_settings.json.

## Current Problem
- 38+ Update* methods in settings.go save only to file
- No database-first approach for user settings
- GetDefaultSettings() used for runtime (should be database)

## Solution Architecture

### 1. Admin Sync Function (NEW)
```go
// SyncSettingsToDefaultsFile - Admin-only function to sync database settings to default_settings.json
// This is the ONLY function that should write to default_settings.json after initialization
func (sm *SettingsManager) SyncSettingsToDefaultsFile(ctx context.Context, repo *database.Repository, userID string, isAdmin bool) error
```

**Requirements:**
- Only works if `isAdmin == true`
- Reads all user settings from database for the admin user
- Writes them to default_settings.json
- Returns error if not admin

### 2. Database Tables Already Available
- `user_mode_configs` - per-mode settings (enabled, timeframe, sltp, etc.)
- `user_capital_allocation` - allocation limits per mode
- `user_global_circuit_breaker` - circuit breaker settings
- `user_ginie_settings` - Ginie-specific settings
- `user_llm_config` - LLM provider settings
- `user_early_warning` - early warning thresholds
- `user_spot_settings` - spot trading settings
- `user_symbol_settings` - per-symbol settings (ALREADY EXISTS)

### 3. New Database Methods Needed (in repository_user.go)

Check if these exist, if not create them:
- `GetUserModeConfig(ctx, userID, mode string) (*ModeConfig, error)`
- `UpsertUserModeConfig(ctx, userID string, config *ModeConfig) error`
- `GetUserCapitalAllocation(ctx, userID string) (*ModeAllocationConfig, error)`
- `UpsertUserCapitalAllocation(ctx, userID string, config *ModeAllocationConfig) error`
- `GetUserGinieSettings(ctx, userID string) (*GinieSettings, error)`
- `UpsertUserGinieSettings(ctx, userID string, config *GinieSettings) error`
- etc.

### 4. Refactor Pattern for Update Methods

**BEFORE (file-only):**
```go
func (sm *SettingsManager) UpdateDynamicSLTP(...) error {
    settings := sm.GetDefaultSettings()
    settings.DynamicSLTPEnabled = enabled
    // ... modify fields
    return sm.SaveSettings(settings)
}
```

**AFTER (database-first):**
```go
func (sm *SettingsManager) UpdateDynamicSLTP(ctx context.Context, repo *database.Repository, userID string, isAdmin bool, ...) error {
    // 1. Load current settings from database
    config, err := repo.GetUserModeConfig(ctx, userID, mode)
    if err != nil {
        return err
    }

    // 2. Modify settings
    config.DynamicSLTPEnabled = enabled
    // ... modify fields

    // 3. Save to database
    if err := repo.UpsertUserModeConfig(ctx, userID, config); err != nil {
        return err
    }

    // 4. If admin, also sync to file
    if isAdmin {
        return sm.SyncSettingsToDefaultsFile(ctx, repo, userID, isAdmin)
    }

    return nil
}
```

### 5. Priority Methods to Refactor

Based on user request, prioritize these:
1. **GetModeAllocation()** - line 3763 - should load from `user_capital_allocation` table
2. **UpdateDynamicSLTP()** - should save to `user_mode_configs` in DB
3. **UpdateGinieTrendTimeframes()** - should save to `user_ginie_settings` in DB
4. All mode enable/disable methods - should update `user_mode_configs` in DB

### 6. All Update Methods to Refactor (38 total)

Search for: `^func (sm \*SettingsManager) Update`

1. UpdateDynamicSLTP
2. UpdateScalping
3. UpdateGinieTrendTimeframes
4. UpdateGinieConfidence
5. UpdateGinieMaxPositions
6. UpdateGinieRiskSettings
7. UpdateModeTimeframes
8. UpdateModeConfidence
9. UpdateModeSizing
10. UpdateModeCircuitBreaker
11. UpdateModeSLTP
12. UpdateModeEnable
13. UpdateModeDisable
14. UpdateCapitalAllocation
15. UpdateGlobalCircuitBreaker
16. UpdateLLMConfig
17. UpdateEarlyWarning
18. UpdateSpotSettings
19. ... (find all 38 methods)

### 7. Implementation Steps

1. **Read all Update methods** - Use Grep to find them all
2. **Analyze database schema** - Check what tables exist and what fields they have
3. **Create missing repository methods** - Add to repository_user.go if needed
4. **Implement SyncSettingsToDefaultsFile** - Admin-only sync function
5. **Refactor each Update method** - Database-first, then admin sync
6. **Update GetDefaultSettings()** - Add deprecation warning
7. **Create GetUserSettings()** - New method that loads from database
8. **Test** - Ensure backward compatibility

### 8. Testing Plan

- Test admin sync function
- Test Update methods save to database
- Test admin sync to file after update
- Test non-admin users don't sync to file
- Test loading settings from database
- Test fallback to file defaults for new users

## Files to Modify

1. `/mnt/c/KOSH/binance-trading-bot/internal/autopilot/settings.go`
   - Add SyncSettingsToDefaultsFile()
   - Refactor all Update* methods
   - Add GetUserSettings()

2. `/mnt/c/KOSH/binance-trading-bot/internal/database/repository_user.go`
   - Add missing repository methods if needed
   - Ensure all tables are accessible

3. Create new file if needed: `/mnt/c/KOSH/binance-trading-bot/internal/database/repository_user_settings.go`
   - Dedicated file for user settings CRUD operations
