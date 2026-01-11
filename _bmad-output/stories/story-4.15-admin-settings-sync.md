# Story 4.15: Admin Settings Sync

**Story ID:** SETTINGS-4.15
**Epic:** Epic 4 - Database-First Mode Configuration System
**Priority:** P1 (High - Admin Template Management)
**Estimated Effort:** 4 hours
**Author:** BMAD Agent (Bob - Scrum Master)
**Status:** Ready for Development
**Depends On:** Story 4.13, Story 4.14

---

## Problem Statement

### Current State

Admin user (`admin@binance-bot.local`) has no special capabilities:
- Admin changes settings like any other user (saved to database)
- No way for admin to update the default template for new users
- Production defaults require code changes and redeployment

### Expected Behavior

Admin should be a **template manager**:
- Admin's settings work like any other user (database-first)
- When admin changes settings, automatically sync to `default-settings.json`
- New users get admin's curated defaults
- Admin can test settings, then publish to all future users

---

## User Story

> As a system administrator,
> When I change any setting through my admin account,
> I want those changes to automatically update `default-settings.json`,
> So that all new users get my curated default settings.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    ADMIN USER                                │
│  - Makes setting changes via UI                              │
│  - Changes saved to database (like any user)                 │
│  - ADDITIONALLY: Auto-sync to default-settings.json          │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
         ┌─────────────────┴─────────────────┐
         │                                   │
         ▼                                   ▼
┌─────────────────┐                ┌─────────────────┐
│   Database      │                │ default-settings│
│ (admin's copy)  │  ──SYNC──►     │     .json       │
│                 │                │ (template file) │
└─────────────────┘                └─────────────────┘
                                            │
                                            ▼
                                   ┌─────────────────┐
                                   │  NEW USERS      │
                                   │ (get template)  │
                                   └─────────────────┘
```

---

## Acceptance Criteria

### AC4.15.1: Admin Detection
- [ ] System identifies admin user by email: `admin@binance-bot.local`
- [ ] Admin flag stored in user record or checked at runtime
- [ ] Non-admin users do NOT trigger sync to JSON

### AC4.15.2: Auto-Sync on Setting Change
- [ ] When admin changes mode config → sync that mode to JSON
- [ ] When admin changes circuit breaker → sync circuit breaker to JSON
- [ ] When admin changes any setting group → sync that group to JSON
- [ ] Sync happens asynchronously (don't block API response)

### AC4.15.3: JSON File Update
- [ ] `default-settings.json` updated with admin's changes
- [ ] `metadata.last_updated` timestamp updated
- [ ] `metadata.updated_by` set to "admin"
- [ ] File maintains proper JSON formatting
- [ ] Backup created before overwriting: `default-settings.json.backup`

### AC4.15.4: Admin First-Login Initialization
- [ ] If admin has no database settings, copy from `default-settings.json`
- [ ] This ensures admin starts with current defaults
- [ ] Admin can then modify and auto-sync back

### AC4.15.5: Logging & Audit
- [ ] Log: `[ADMIN-SYNC] Admin changed scalp.confidence.min -> syncing to defaults`
- [ ] Log: `[ADMIN-SYNC] Updated default-settings.json (group: mode_configs.scalp)`
- [ ] Audit trail of admin changes (optional: store in database)

### AC4.15.6: Manual Sync Endpoint
- [ ] Admin-only endpoint: `POST /api/admin/sync-defaults`
- [ ] Syncs entire admin config to `default-settings.json`
- [ ] Returns summary of what was synced
- [ ] Useful if auto-sync missed something

---

## Technical Implementation

### Task 1: Admin Detection Service

```go
// internal/auth/admin_check.go

package auth

const AdminEmail = "admin@binance-bot.local"

// IsAdminUser checks if the user is the system admin
func IsAdminUser(email string) bool {
    return email == AdminEmail
}

// IsAdminUserID checks admin by user ID (cached lookup)
func IsAdminUserID(ctx context.Context, db *database.Repository, userID string) bool {
    user, err := db.GetUserByID(ctx, userID)
    if err != nil {
        return false
    }
    return IsAdminUser(user.Email)
}
```

### Task 2: Admin Sync Service

```go
// internal/autopilot/admin_sync.go

package autopilot

import (
    "encoding/json"
    "os"
    "sync"
    "time"
)

var adminSyncMutex sync.Mutex

// SyncAdminSettingToDefaults syncs a specific setting group to default-settings.json
func SyncAdminSettingToDefaults(group string, data interface{}) error {
    adminSyncMutex.Lock()
    defer adminSyncMutex.Unlock()

    // Load current defaults
    defaults, err := loadDefaultSettingsFile()
    if err != nil {
        return fmt.Errorf("failed to load defaults: %w", err)
    }

    // Create backup
    backupPath := "default-settings.json.backup"
    if err := copyFile("default-settings.json", backupPath); err != nil {
        log.Printf("[ADMIN-SYNC] Warning: Failed to create backup: %v", err)
    }

    // Update the specific group
    switch group {
    case "mode_configs":
        if modeConfigs, ok := data.(map[string]*ModeFullConfig); ok {
            defaults.ModeConfigs = modeConfigs
        }
    case "mode_config":
        // Single mode update
        if update, ok := data.(*ModeConfigUpdate); ok {
            defaults.ModeConfigs[update.ModeName] = update.Config
        }
    case "circuit_breaker":
        if cb, ok := data.(*CircuitBreakerDefaults); ok {
            defaults.CircuitBreaker = *cb
        }
    case "llm_config":
        if llm, ok := data.(*LLMConfigDefaults); ok {
            defaults.LLMConfig = *llm
        }
    // ... other groups
    }

    // Update metadata
    defaults.Metadata.LastUpdated = time.Now().UTC().Format(time.RFC3339)
    defaults.Metadata.UpdatedBy = "admin"

    // Write back to file
    data, err := json.MarshalIndent(defaults, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal defaults: %w", err)
    }

    if err := os.WriteFile("default-settings.json", data, 0644); err != nil {
        return fmt.Errorf("failed to write defaults: %w", err)
    }

    log.Printf("[ADMIN-SYNC] Updated default-settings.json (group: %s)", group)
    return nil
}

type ModeConfigUpdate struct {
    ModeName string
    Config   *ModeFullConfig
}

// SyncAdminModeConfig syncs a single mode config
func SyncAdminModeConfig(modeName string, config *ModeFullConfig) error {
    return SyncAdminSettingToDefaults("mode_config", &ModeConfigUpdate{
        ModeName: modeName,
        Config:   config,
    })
}
```

### Task 3: Hook into Setting Save Handlers

```go
// internal/api/handlers_ginie.go

// In handleUpdateModeConfig or similar handlers:

func (s *Server) handleUpdateModeConfig(c *gin.Context) {
    userID := c.GetString("user_id")
    modeName := c.Param("mode")

    // ... existing validation and save logic ...

    // Save to database
    err = s.repo.SaveUserModeConfig(ctx, userID, modeName, config.Enabled, configJSON)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to save config"})
        return
    }

    // ADMIN SYNC: If admin user, sync to defaults file
    if auth.IsAdminUserID(ctx, s.repo, userID) {
        go func() {
            if err := autopilot.SyncAdminModeConfig(modeName, config); err != nil {
                log.Printf("[ADMIN-SYNC] Failed to sync %s: %v", modeName, err)
            }
        }()
    }

    c.JSON(200, gin.H{"success": true})
}
```

### Task 4: Admin-Only Manual Sync Endpoint

```go
// internal/api/handlers_admin.go

// handleAdminSyncDefaults manually syncs all admin settings to defaults
// POST /api/admin/sync-defaults
func (s *Server) handleAdminSyncDefaults(c *gin.Context) {
    userID := c.GetString("user_id")
    ctx := context.Background()

    // Verify admin
    if !auth.IsAdminUserID(ctx, s.repo, userID) {
        c.JSON(403, gin.H{"error": "Admin access required"})
        return
    }

    // Get all admin's mode configs
    modeConfigs := make(map[string]*autopilot.ModeFullConfig)
    modes := []string{"ultra_fast", "scalp", "scalp_reentry", "swing", "position"}

    for _, modeName := range modes {
        config, err := s.repo.GetUserModeConfig(ctx, userID, modeName)
        if err != nil {
            log.Printf("[ADMIN-SYNC] Warning: Failed to get %s: %v", modeName, err)
            continue
        }
        modeConfigs[modeName] = config
    }

    // Sync to defaults file
    if err := autopilot.SyncAdminSettingToDefaults("mode_configs", modeConfigs); err != nil {
        c.JSON(500, gin.H{"error": fmt.Sprintf("Sync failed: %v", err)})
        return
    }

    // Get other settings and sync
    // ... circuit breaker, LLM config, etc.

    log.Printf("[ADMIN-SYNC] Manual sync completed by admin")

    c.JSON(200, gin.H{
        "success":      true,
        "synced_modes": len(modeConfigs),
        "timestamp":    time.Now().UTC().Format(time.RFC3339),
        "message":      "All admin settings synced to default-settings.json",
    })
}
```

### Task 5: Admin First-Login Initialization

```go
// internal/auth/auth_service.go

func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
    // ... existing login logic ...

    // ADMIN CHECK: Initialize admin settings if needed
    if auth.IsAdminUser(email) {
        hasSettings, err := s.repo.UserHasModeConfigs(ctx, user.ID.String())
        if err == nil && !hasSettings {
            log.Printf("[AUTH] Admin user has no settings, initializing from defaults")
            if err := InitializeUserSettings(ctx, s.repo, user.ID.String()); err != nil {
                log.Printf("[AUTH] Warning: Failed to initialize admin settings: %v", err)
            }
        }
    }

    return &LoginResponse{Token: token, User: user}, nil
}
```

### Task 6: Register Admin Routes

```go
// internal/api/server.go

// Admin-only routes
admin := r.Group("/api/admin")
admin.Use(s.authMiddleware(), s.adminMiddleware())
{
    admin.POST("/sync-defaults", s.handleAdminSyncDefaults)
    admin.GET("/sync-status", s.handleAdminSyncStatus)
}

// Admin middleware
func (s *Server) adminMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        ctx := context.Background()

        if !auth.IsAdminUserID(ctx, s.repo, userID) {
            c.JSON(403, gin.H{"error": "Admin access required"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

---

## API Reference

### Admin Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/admin/sync-defaults` | Manually sync all admin settings to JSON |
| GET | `/api/admin/sync-status` | Get last sync timestamp and status |

### Response Format

```json
{
  "success": true,
  "synced_modes": 5,
  "synced_groups": ["mode_configs", "circuit_breaker", "llm_config"],
  "timestamp": "2026-01-05T12:00:00Z",
  "message": "All admin settings synced to default-settings.json",
  "backup_created": "default-settings.json.backup"
}
```

---

## Security Considerations

1. **Admin-Only Access**: Only `admin@binance-bot.local` can trigger sync
2. **Backup Before Write**: Always create backup before modifying JSON
3. **Async Sync**: Don't block API response on file write
4. **File Permissions**: Ensure JSON file is writable by app user
5. **Audit Trail**: Log all admin sync operations

---

## Testing Requirements

### Test 1: Admin Change Triggers Sync
```bash
# Login as admin
TOKEN=$(curl -s -X POST http://localhost:8094/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@binance-bot.local","password":"Weber@#2025"}' | jq -r '.token')

# Change scalp min_confidence
curl -X PUT http://localhost:8094/api/futures/ginie/modes/scalp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"confidence":{"min_confidence":45}}'

# Verify JSON updated
cat default-settings.json | jq '.mode_configs.scalp.confidence.min_confidence'
# Expected: 45

# Check metadata updated
cat default-settings.json | jq '.metadata.updated_by'
# Expected: "admin"
```

### Test 2: Non-Admin Does NOT Sync
```bash
# Login as regular user
USER_TOKEN=$(curl -s -X POST http://localhost:8094/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"testuser@example.com","password":"password123"}' | jq -r '.token')

# Save original value
ORIGINAL=$(cat default-settings.json | jq '.mode_configs.scalp.confidence.min_confidence')

# Change scalp min_confidence as regular user
curl -X PUT http://localhost:8094/api/futures/ginie/modes/scalp \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"confidence":{"min_confidence":99}}'

# Verify JSON NOT changed
cat default-settings.json | jq '.mode_configs.scalp.confidence.min_confidence'
# Expected: Same as ORIGINAL (not 99)
```

### Test 3: Manual Sync Endpoint
```bash
# Admin manual sync
curl -X POST http://localhost:8094/api/admin/sync-defaults \
  -H "Authorization: Bearer $TOKEN" | jq '.'
# Expected: success=true, synced_modes=5

# Non-admin should get 403
curl -X POST http://localhost:8094/api/admin/sync-defaults \
  -H "Authorization: Bearer $USER_TOKEN"
# Expected: {"error":"Admin access required"}
```

### Test 4: Backup Created
```bash
# After admin change, backup should exist
ls -la default-settings.json.backup
# Expected: File exists with recent timestamp
```

---

## Definition of Done

- [ ] Admin detection works by email
- [ ] Admin setting changes auto-sync to JSON
- [ ] Sync happens asynchronously
- [ ] Backup created before each write
- [ ] Metadata updated (timestamp, updated_by)
- [ ] Manual sync endpoint works
- [ ] Non-admin users don't trigger sync
- [ ] Admin first-login initializes from defaults
- [ ] All tests pass
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

- **Story 4.13:** Default Settings JSON Foundation (prerequisite)
- **Story 4.14:** New User & Load Defaults (prerequisite)
- **Story 4.16:** Settings Comparison & Risk Display (can use sync timestamp)
